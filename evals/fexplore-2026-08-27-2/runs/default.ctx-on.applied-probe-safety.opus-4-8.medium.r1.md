## file: internal/repos/status.go

// [ACDSL-PROJECTION] 5 rule(s) govern this file — working-copy view, stripped before commit
// - [ACDSL-GOLANG-ENUM-001] Every switch over a domain enum names all members (ACTION-IMPL-INTEG-005) — no default that silently swallows new ones
// - [ACDSL-GOLANG-EXEC-001] Child processes run under a context deadline via internal/shell.Run — a raw exec.Command has no timeout and can wedge the caller forever; routinewrap runs multi-hour children under its own backstop-deadline context (plans/windows_support raw.md D14)
// - [ACDSL-GOLANG-FMT-001] Every Go file is gofmt-formatted — run gofmt -w before committing; this gate replaces the raw Makefile gofmt line
// - [ACDSL-GOLANG-FUNC-001] Signatures follow RULE-GOLANG-FUNC-001 — ctx context.Context is the first parameter and is named ctx, error is the last return value, at most 3 return values
// - [ACDSL-GOLANG-STATE-001] context/context.json has exactly one producer — only cmd/rules may write it via the internal/contextdocs renderer; internal/server reads it for the coverage metric and internal/repos probes it for repo context detection; any other writer desyncs the generated context file

package repos

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	removeScript = "remove_agent_worktrees.sh"
	statusScript = "print_agent_worktrees_status.sh"
	syncScript   = "sync_worktrees.sh"
)

// noWorktreeDirty marks a branch without a checked-out worktree ("-" in the
// script's DIRTY column).
const noWorktreeDirty = -1

// unknownCount marks AHEAD/BEHIND when the branch's origin is unrecorded
// ("-" in the script; FROM is never guessed).
const unknownCount = -1

type Commit struct {
	Line string
	Sha  string
}

type WorktreeStatus struct {
	Ahead         int // unknownCount when From is "unknown"
	Behind        int // unknownCount when From is "unknown"
	Branch        string
	Dirty         int      // noWorktreeDirty when the branch has no worktree
	From          string   // "unknown" when the branch reflog records no origin
	In            []string // FROM first, probe/twin-upgraded entries keep their "*"; empty when the work exists nowhere else
	LastCommit    string
	MergedInto    string // actual merge commit target; empty when never merged
	ResolvedPicks int    // picked-resolved verdicts: transferred via manually conflict-resolved picks, not auto-reconcilable
	SafeToRemove  bool
	SafeViaProbe  bool     // an IN entry carries the "*" probe/twin marker
	UnsafeReasons []string // the conditions that made SafeToRemove false; empty when safe
	Unpicked      int
	Untracked     int    // noWorktreeDirty when the branch has no worktree
	Verdicts      string // probe summary (applied:n,resolved:n,picked-resolved:n); empty when "-"
	Worktree      string // empty when none
}

type CheckoutStatus struct {
	Branch string
	Dirty  int
}

// Checkout reports the main checkout's branch and uncommitted-change count
// (untracked excluded — the exact predicate merge_worktree.sh and
// cherry_pick_worktree.sh refuse on), so the UI flags a dirty checkout before
// an op runs instead of failing at execution.
func Checkout(ctx context.Context, repoPath string) (*CheckoutStatus, error) {
	branch, err := shell.Run(ctx, repoPath, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("Checkout: %s: %w", strings.TrimSpace(branch), err)
	}

	output, err := shell.Run(ctx, repoPath, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("Checkout: %s: %w", strings.TrimSpace(output), err)
	}

	dirty := 0
	for line := range strings.Lines(output) {
		if line != "\n" && line != "" && !strings.HasPrefix(line, "??") {
			dirty++
		}
	}
	return &CheckoutStatus{Branch: strings.TrimSpace(branch), Dirty: dirty}, nil
}

// Commits lists one class (ahead|behind|unpicked) of commits for a branch via
// the status script's detail mode, addressed by branch name — one script run
// per drill-down (supersedes feature_extension_v2 D17).
func Commits(ctx context.Context, branch, class, repoPath, scriptsDir string) ([]Commit, error) {
	script := filepath.Join(scriptsDir, statusScript)
	output, err := shell.Run(ctx, repoPath, script, branch, class)
	if err != nil {
		return nil, fmt.Errorf("Commits: %s: %w", strings.TrimSpace(output), err)
	}

	return parseCommits(output), nil
}

// Status runs the overview table with cwd = repo path and parses it (D18).
func Status(ctx context.Context, repoPath, scriptsDir string) ([]WorktreeStatus, error) {
	script := filepath.Join(scriptsDir, statusScript)
	output, err := shell.Run(ctx, repoPath, script)
	if err != nil {
		return nil, fmt.Errorf("Status: %s: %w", strings.TrimSpace(output), err)
	}

	return parseStatus(output)
}

func isStatusHeader(fields []string) bool {
	return len(fields) == 0 || fields[0] == "#"
}

func parseCommits(output string) []Commit {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= 1 {
		return nil
	}

	var commits []Commit
	// First line is the "Commits …:" heading.
	for _, line := range lines[1:] {
		isEmpty := line == "" || line == "(none)"
		if isEmpty {
			continue
		}
		fields := strings.Fields(line)
		commit := Commit{Line: line, Sha: fields[0]}
		commits = append(commits, commit)
	}
	return commits
}

func parseCount(field, column string) (int, error) {
	count, err := strconv.Atoi(field)
	if err != nil {
		return 0, fmt.Errorf("parseCount: Invalid %s value %q: %w", column, field, err)
	}
	return count, nil
}

func parseStatus(output string) ([]WorktreeStatus, error) {
	if strings.HasPrefix(output, "No agent branches") {
		return nil, nil
	}

	var result []WorktreeStatus
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		fields := strings.Fields(line)
		if isStatusHeader(fields) {
			continue
		}

		status, err := parseStatusRow(fields, line)
		if err != nil {
			return nil, err
		}
		result = append(result, *status)
	}
	return result, nil
}

// parseStatusRow maps one table row: 11 fixed fields, LAST-COMMIT as two
// tokens (date + time), the worktree path as the remainder. Any other shape
// is a hard error — never guess a field mapping (stop condition S8).
func parseStatusRow(fields []string, line string) (*WorktreeStatus, error) {
	if len(fields) < 14 {
		return nil, fmt.Errorf("parseStatusRow: Unparseable row: %q", line)
	}

	status := &WorktreeStatus{
		Branch:     fields[1],
		From:       fields[2],
		LastCommit: fields[11] + " " + fields[12],
	}

	// DIRTY/UNTRACKED: "-" = branch has no worktree checked out
	if fields[3] == "-" {
		status.Dirty = noWorktreeDirty
	} else {
		dirty, err := parseCount(fields[3], "DIRTY")
		if err != nil {
			return nil, err
		}
		status.Dirty = dirty
	}

	if fields[4] == "-" {
		status.Untracked = noWorktreeDirty
	} else {
		untracked, err := parseCount(fields[4], "UNTRACKED")
		if err != nil {
			return nil, err
		}
		status.Untracked = untracked
	}

	// AHEAD/BEHIND: "-" = FROM unknown, counts not computable
	if fields[5] == "-" {
		status.Ahead = unknownCount
	} else {
		ahead, err := parseCount(fields[5], "AHEAD")
		if err != nil {
			return nil, err
		}
		status.Ahead = ahead
	}

	if fields[6] == "-" {
		status.Behind = unknownCount
	} else {
		behind, err := parseCount(fields[6], "BEHIND")
		if err != nil {
			return nil, err
		}
		status.Behind = behind
	}

	unpicked, err := parseCount(fields[7], "UNPICKED")
	if err != nil {
		return nil, err
	}
	status.Unpicked = unpicked

	// VERDICTS: "-" = nothing needed probing
	if fields[8] != "-" {
		status.Verdicts = fields[8]
		for _, token := range strings.Split(status.Verdicts, ",") {
			count, found := strings.CutPrefix(token, "picked-resolved:")
			if !found {
				continue
			}
			resolved, err := parseCount(count, "VERDICTS")
			if err != nil {
				return nil, err
			}
			status.ResolvedPicks = resolved
		}
	}

	// MERGED: "-" = the tip was never target of an actual merge commit
	if fields[9] != "-" {
		status.MergedInto = fields[9]
	}

	// IN: comma-separated, FROM first; "-" = the work exists nowhere else
	if fields[10] != "-" {
		status.In = strings.Split(fields[10], ",")
	}

	// WORKTREE: en-dash = branch has no worktree (script prints '–')
	worktree := strings.Join(fields[13:], " ")
	if worktree != "–" {
		status.Worktree = worktree
	}

	// Script header contract: IN != "-" plus, when a worktree is checked out,
	// DIRTY=0 and UNTRACKED=0 => safe to remove. A branch without a worktree
	// (DIRTY "-") has no files to lose — containment alone decides. A "*" IN
	// entry means containment came from the probe/twin verdicts.
	contained := len(status.In) > 0
	if status.Dirty == noWorktreeDirty {
		status.SafeToRemove = contained
	} else {
		status.SafeToRemove = status.Dirty == 0 && status.Untracked == 0 && contained
	}
	for _, containing := range status.In {
		if strings.HasSuffix(containing, "*") {
			status.SafeViaProbe = true
		}
	}
	if !status.SafeToRemove {
		status.UnsafeReasons = unsafeReasons(status, contained)
	}
	return status, nil
}

// unsafeReasons names each condition that blocks removal, in the order the
// safety predicate checks them — the UI shows these instead of a generic ✗.
func unsafeReasons(status *WorktreeStatus, contained bool) []string {
	var reasons []string
	if status.Dirty > 0 {
		reasons = append(reasons, fmt.Sprintf("dirty(%d)", status.Dirty))
	}
	if status.Untracked > 0 {
		reasons = append(reasons, fmt.Sprintf("untracked(%d)", status.Untracked))
	}
	if !contained {
		if status.Unpicked > 0 {
			reasons = append(reasons, fmt.Sprintf("unpicked(%d)", status.Unpicked))
		} else {
			reasons = append(reasons, "work contained nowhere")
		}
		if status.Dirty == noWorktreeDirty {
			reasons = append(reasons, "no worktree")
		}
	}
	return reasons
}


## file: plans/applied-probe-safety/design/exploration.md

# Applied-Probe Safety for Agent-Worktree Branches — Exploration

## Context

- **Open question, two parts.** (a) How does the Overview classify an ahead-commit whose patch-id is *absent* from FROM so that a conflict-resolved cherry-pick reads Safe instead of pinned-unsafe forever? (b) Where does the safety predicate live so the Overview, the drill-down, and `remove_agent_worktrees.sh` never disagree — today it is three drifting copies.
- **Driver.** The `claude/railroad-review-workflow-f76262` incident: commit `8875782` landed as `9a2098e` with a resolved `docs/workflows.md` conflict; the resolution changed the hunk context, changed the patch-id, and `git cherry` pins the branch unsafe even though every commit was harvested (concept.md:9).
- **Hard boundary.** The predicate gates an irreversible worktree/branch deletion. A false "applied" verdict destroys unharvested work; the whole space is judged fail-closed.
- **Mode:** familiar — explanation depth assumes the reader knows `git cherry`, patch-ids, and the repo's worktree tooling.
- **Scope note.** The concept has drained its open questions and records `[USER]` decisions that cite *this* exploration (concept.md:127 → "option C"). This document is the upstream survey those decisions rest on; it evaluates the space, then lands on the options the concept ratified — with the reasoning that makes them fail-closed rather than merely workable.

## Constraints

| ID | Constraint | Source (anchor / measurement) |
|----|-----------|-------------------------------|
| C1 | `git cherry` patch-id is exact but brittle: any conflict resolution rewrites context lines → new patch-id → false UNPICKED | concept.md:9; `verdict.sh:68` `compute_cherry_sets` |
| C2 | The verdict gates an irreversible deletion — every layer must **fail closed** (ambiguous ⇒ `unpicked`) | concept.md:46; `remove_agent_worktrees.sh:98` (empty IN ⇒ skip) |
| C3 | bash 3.2 / macOS: no associative arrays, no `flock`, no `date %N` (perl for ms) | `verdict.sh:13,121`; MEMORY `macos-no-flock` |
| C4 | Probe git output must be fully silenced — `Auto-merging`/`CONFLICT` on stdout parse as commit rows in the UI | `verdict.sh:192`; `parseCommits` `status.go:115` |
| C5 | FROM may be a remote-tracking ref (`origin/main`) that is **not** in the candidate set | concept.md:122; `verdict.sh:31-46` `resolve_from` |
| C6 | Overview cost stays near plumbing-only: probe only rows whose FROM cherry-`+` set is non-empty; a fully-harvested branch pays nothing | concept.md:16; `verdict.sh:403-404` (probe loop guarded on `from_plus`) |
| C7 | Output contract is load-bearing: the Go parser demands ≥14 whitespace-split fields; template, parser, and every fixture move together | `status.go:168`; concept.md:128 |
| C8 | The probe worktree is created lazily and reused per process; a multi-branch caller (remove script) re-detaches it across differing FROMs | `verdict.sh:129-145` `ensure_probe_worktree` |
| C9 | Go's role is a **parser** of the shell overview, not an independent computer of the verdict logic | `status.go:101-109,143-277` |

## Options

Two orthogonal sub-problems. **A** = per-commit classification mechanism. **B** = where the shared predicate lives. A solution is one A-family × one B-family.

### A — Per-commit "did this change land?" classifier

The families compose as escalating layers; the design choice is *which layers* and *how strict layer 3 accepts*.

- **A1 — patch-id only (layers = {1}).** `git cherry` `+`/`-`. Cheap, exact, and the exact thing the incident defeats (C1). Mechanism kept as layer 1 everywhere; useless alone.
- **A2 — clean re-pick probe (layers = {1,2}).** `cherry-pick --no-commit "$hash"` onto a detached FROM worktree; empty staged diff ⇒ `applied` (`verdict.sh:199-204`). Catches an already-applied change that merged cleanly. A *conflicted* re-pick aborts — ambiguous, so A2 alone still pins the incident unsafe. Binds C4 (silence), C6 (lazy worktree), C8.
- **A3 — forced-resolution empty-diff (`-X theirs`).** Re-pick with `-X theirs`, test the resulting diff for emptiness. **Killed by C2:** `-X theirs` silently discards a genuinely-unapplied conflicting hunk, manufacturing a false "applied" on the deletion gate (concept.md:124). Listed to keep it refuted.
- **A4 — range-diff disambiguation of the conflicted re-pick (layers = {1,2,3}).** When layer 2 conflicts, `git range-diff <merge-base(hash,FROM)>..FROM  hash^..hash` pairs the agent commit with a similar commit on FROM's side; a `=`/`!` pairing is the pairing signal (`verdict.sh:223`). Sub-choice on *acceptance strictness*:
  - **A4-loose** — any `=`/`!` pairing ⇒ `applied-resolved`. **Killed by C2:** a competing edit to the same region also pairs, producing a false-safe (the S7 stop condition the concept cites).
  - **A4-creation-factor** — tune `range-diff --creation-factor` so only "similar enough" pairs accept. A similarity *percentage* is a heuristic knob, not a safety proof; the competing-edit false-pass returns at any threshold that also accepts a legitimate resolution. Arbitrary, and un-anchored to what the gate actually cares about.
  - **A4-added-content** (the option the concept ratifies as "option C") — accept only when the interdiff shows **no added-line difference** between the two commits (`interdiff_addition_verdict`, `verdict.sh:171-186`). Removed/context-line drift is inherent to a legitimate resolved pick (it removes FROM's moved-on lines, not the base's) and does not block; any *added-content* difference means the agent's payload is not what landed ⇒ `unpicked`. Fail-closed guards: no pairing ⇒ `unpicked`; a pure-deletion commit whose removed lines differ ⇒ `unpicked`; any binary hunk ⇒ `unpicked`. Anchors the accept/reject decision to the one question the gate asks — "is the agent's *added content* present on FROM?" — instead of a global similarity score.
- **A5 — candidate-wide subject-twin sweep (layers = {1,2,3,4}).** A4 probes **FROM only**. The incident's resolved pick landed on a *candidate* branch (`main`), which A4 never inspects (C5 — FROM may even be the remote `origin/main`, a different ref than where the pick landed). A5 adds: for the commits A4 left `unpicked`, sweep every candidate (FROM first) for a commit with the **identical subject line** since the merge-base and `range-diff`-pair it; a pairing ⇒ `picked-resolved` (transferred, but a later re-pick/merge will re-conflict), a rejected twin ⇒ `unpicked`, no twin anywhere ⇒ `unpicked-notwin` (a reworded/squashed transfer is invisible and must be verified by hand). `verdict.sh:278-329`. This is the only family that actually clears the driving incident. Cost is bounded by C6 (only reached for still-`unpicked` commits) and by anchoring the search on an exact subject match rather than probing every candidate blindly (the concept's backlogged "probe every candidate" item, cheaper).

**Layer composition (A5 superset) — how a single commit resolves, strictest first, short-circuiting:**
1. patch-id present on FROM ⇒ `picked` (no probe).
2. clean re-pick, empty diff ⇒ `applied`.
3. conflicted re-pick, added-content-clean range-diff pairing on FROM ⇒ `applied-resolved`.
4. else candidate-wide: exact patch on a candidate ⇒ `picked`; subject-twin paired ⇒ `picked-resolved`; twin rejected ⇒ `unpicked`; no twin ⇒ `unpicked-notwin`.

Only `unpicked`/`unpicked-notwin` keep the branch unsafe.

### B — Where the unified predicate lives

- **B1 — sourced shell lib, Go stays a parser.** Extract candidate discovery, FROM resolution, cherry sets, the layered probe, the twin sweep, containment, and the untracked filter into `cmd/worktrees/_lib/verdict.sh`; both `print_agent_worktrees_status.sh` and `remove_agent_worktrees.sh` `source` it (`verdict.sh:1-11`, `remove_agent_worktrees.sh:35`). Go consumes the overview's `IN`/`DIRTY`/`UNTRACKED` columns and applies a two-line safe predicate (`status.go:262-267`). One home for the expensive, drift-prone logic; follows the `routines/_lib/worktree.sh` precedent; both CLIs stay standalone (C9). Sub-choice on the *safe predicate* itself:
  - **B1-derive** (current) — Go re-derives `safe = IN≠"-" ∧ (no worktree ∨ dirty=0 ∧ untracked=0)`. The predicate is a trivial boolean stated in the shell header contract and re-encoded in Go — cheap, but stated twice.
  - **B1-emit** — the lib computes a `SAFE` verdict and the script emits it as a column; Go reads it verbatim. True single source of the predicate, at the cost of one more column and its fixture churn (C7). Worth it only if the safe predicate grows past two clauses.
- **B2 — Go binary as the single source, shell scripts shell out to it.** Reimplement the verdict logic in Go (`smine worktree-verdict <branch>`); both shell scripts call it. Typed and unit-testable, but it inverts the plumbing: a shell CLI calling a Go binary that itself shells out to `git` — two process layers, a build dependency added to standalone shell tools, and a from-scratch rewrite of working code. Fails groundedness and "keep both CLIs unchanged."
- **B3 — collapse into the Go server, delete the scripts.** Everything served from the config server / a Go CLI. Largest blast radius; kills the scripts' standalone (out-of-server) use. Rejected on scope.
- **B4 — keep three copies, add a golden cross-check test.** A test runs all three and asserts agreement. Guards drift instead of deleting it — three sources survive, against single-source-of-truth. The concept's own framing: "deleting the duplicate, not guarding around it" (concept.md:92).

## Evaluation

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|-------------|-------------|--------|--------------|---------|
| A1 patch-id only | rides existing `git cherry` | none | 0 | n/a | **Reject** — is the bug (C1) |
| A2 clean re-pick | reuses the detail-mode probe | probe worktree lifecycle | ~1d | easy | **Partial** — necessary layer, insufficient alone |
| A3 `-X theirs` | small | deletion gate | low | easy | **Reject** — false-safe (C2) |
| A4-loose range-diff | rides range-diff | deletion gate | low | easy | **Reject** — competing-edit false-safe (S7, C2) |
| A4-creation-factor | rides range-diff | deletion gate | low | easy | **Reject** — arbitrary threshold, same false-safe |
| A4-added-content | rides range-diff, targeted awk | contained, fail-closed | ~1d | easy | **Accept** — layer 3; anchored to added payload |
| A5 twin sweep | rides `git log --grep` + range-diff | contained, fail-closed | ~1.5d | easy | **Accept** — only family that clears the incident |
| B1-derive lib + parser | rides shell plumbing + `_lib` precedent | two scripts + Go parser | ~2–3d | easy | **Accept (recommended)** |
| B1-emit SAFE column | same, +1 column | +fixture churn (C7) | +0.5d | easy | Hold — adopt if predicate grows |
| B2 Go binary source | rewrites working plumbing | both CLIs + build dep | high | hard | **Reject** — low groundedness |
| B3 collapse to server | discards scripts | very high | high | hard | **Reject** — kills standalone use |
| B4 three copies + test | leaves drift live | three sources | med | easy | **Reject** — guards drift, not single-source |

**Per-scenario notes.**
- *Clean already-applied merge* (no conflict on re-pick): A2 alone suffices; layers 3–4 never fire. The cheapest happy path.
- *The driving incident* (resolved pick landed on a candidate under the same subject): needs **A5** — A4's FROM-only probe cannot see a pick that landed on `main` while FROM is `origin/main` (C5).
- *Resolved pick that landed on FROM itself*: A4-added-content resolves it as `applied-resolved` without reaching the sweep.
- *Reworded/squashed transfer*: no family can prove it safe; A5 fails closed to `unpicked-notwin` and tells the user to verify by hand — the honest floor.

## Recommendation

**A5 (full layered probe, layer 3 = A4-added-content) × B1-derive.**

- **A5** because it is the only classifier that clears the driving incident, and every layer fails closed (C2): ambiguity resolves to `unpicked`, never to a false Safe. Layer 3's acceptance is anchored to the *added-content* question the deletion gate actually asks (A4-added-content), which is why A4-loose/A4-creation-factor are out — a similarity score is not a safety proof.
- **B1-derive** because the expensive, drift-prone logic (cherry sets + probe escalation + twin sweep + containment + untracked filter) lands in one sourced `_lib/verdict.sh`, deleting the remove script's copy-pasted `contained_in`/`count_untracked`; Go legitimately stays a parser (C9) applying a two-line safe boolean. B2/B3 buy testability at the cost of rewriting working plumbing and adding a build dependency to standalone tools — groundedness, the criterion that decides ties here, kills them.

**What fdesign imports:**
- The four-layer classifier and its short-circuit order (A5 list above); `applied-resolved` and `picked-resolved` both count as Safe but stay *visibly distinct* from exact picks (VERDICTS column + starred IN entries + drill-down suffixes).
- Layer 3 acceptance = interdiff **added-line** equality, with the three fail-closed guards (no pairing / pure-deletion removed-line drift / binary ⇒ `unpicked`).
- Packaging = `cmd/worktrees/_lib/verdict.sh` sourced by both shell scripts; Go parses the overview.
- Binding constraints: C2 (fail-closed) governs every acceptance rule; C4 (silence probe git output) and C7 (≥14-field output contract, fixtures move together) govern the wiring; C5 (remote-tracking FROM) forces the candidate-wide sweep and the annotated FROM entry in IN; C6 (probe only non-empty `+` rows) governs cost.
- Measurements to carry: the `8875782`→`9a2098e` incident as the accept ground-truth; a competing-edit-to-the-same-region case as the reject ground-truth; a legitimate resolved pick as the second accept. Layer-3 must separate all three (concept.md:127).

## Rejected

- **A1 patch-id only** — is the reported bug; brittle to any conflict resolution (C1).
- **A3 `-X theirs` empty-diff** — discards genuinely-unapplied conflicting hunks ⇒ false Safe on a deletion gate (C2).
- **A4-loose range-diff pairing** — a competing edit to the same region also pairs ⇒ false Safe (S7, C2).
- **A4-creation-factor tuning** — a similarity threshold is not a safety proof; the competing-edit false-pass survives any threshold that still accepts legitimate resolutions.
- **B2 Go-binary source** — rewrites working shell plumbing, adds a build dependency to standalone CLIs, two process layers; low groundedness.
- **B3 collapse into the server** — discards the scripts' standalone use; largest blast radius.
- **B4 three copies + golden cross-check** — guards drift instead of deleting it; three sources survive against single-source-of-truth (concept.md:92).

## Open Questions

- **Safe-predicate home (B1-derive vs B1-emit).** Assumed B1-derive: Go re-derives the two-line safe boolean from IN/DIRTY/UNTRACKED. If the safe predicate ever gains a third clause, switch to B1-emit (shell emits a `SAFE` column, Go reads it verbatim) so the predicate has one home. Recorded as an assumption, not a blocker. *(Unattended session — no user to confirm; defaulting to the lower-churn B1-derive, which the shipped code already reflects.)*
- **Probe-cost cache.** The concept backlogs a persistent verdict cache behind measurement (concept.md:123); this exploration assumes the MVP ships uncached with a trace hook, consistent with that decision. Not re-opened here.


