# Review: smine v1.1.4 → v1.2.1

Range reviewed: `b4981b88d52deaa69d6ed1d60eb51b57ead47ba0..a342811b4cf7b8bb2b54359ced6c786d3b5f349b`, excluding `sessions/`, `evals/`, `proposals/`, `plans/`. 178 files, +7314/-749.

This is far too large for one pass to read line-by-line. I read the ~15 change plans under `plans/v1-2-1/` in full and used them to target the diff: the two repos/routines plans, the ACDSL policy-modes plan, and the bootstrap-wrapper plan in depth (code read against plan pseudocode, line by line); the presentation/language, peek-multi-profile, session-context, and smine-pipeline plans at a shallower pass (structural diff read, spot checks). I built and tested the code (both in this checkout and in two disposable worktrees at the base and head commits) rather than trusting the diff alone.

## Strengths

- **`statusCache`/`Registry` fingerprint work is correct and matches both governing plans exactly** (`internal/server/statuscache.go`, `internal/repos/registry.go`). The read-modify-write registry (`readRegistrySnapshot` before every `Add`/`Remove`), the stat-identity `refreshIfChanged`, and the `Fingerprint`-gated cache all match the `repo-cache-reload` plan's pseudocode close to verbatim, including the subtle bit that `DropBranches` deliberately does *not* touch `Fingerprint` so a later plain load re-scans and converges to truth.
- **`peek.SessionIndex`/`keepNewest`** (`internal/peek/peek.go`) is a clean fix for the documented pool-recycled-worktree misattribution (FACT-REPO-ARCH-017): one small map-update helper reused for both the by-branch and by-cwd projections, and `worktreeSessions` in `repos.go` correctly prefers branch identity over cwd.
- **`cmd/bootstrap/run.sh`** is a faithful, careful implementation of the wrapper-orchestration plan: bash 3.2/`set -u` discipline is respected throughout (`${flags[@]+"${flags[@]}"}`, `if`-guarded expected failures rather than `cmd && fail`, no heredocs inside `$()`), and the default-deny dev-mode derivation matches D3/D4 exactly.
- **`internal/acdsl/policy.go`'s `basePolicy` design is actually *better* than the plan's own pseudocode** — see Critical below for why this matters, but the instinct to re-derive the policy from the base ref rather than trusting the caller-supplied value is the right one; it just isn't applied consistently.
- The squash-containment layer (`cmd/worktrees/_lib/verdict.sh`) is careful: it only runs on candidates the existing three layers left partially unexplained, never runs on the fast path, and the containment check (`merge-tree --write-tree` equality) is a legitimate proof of content containment, not a heuristic.
- Go build, `go vet`, and the full test suite are green (verified in a clean disposable worktree at head — see Testing note below).

## Issues

### Critical (Must Fix)

**1. The ACDSL self-management policy gate can be fully disabled by editing the attacker-controlled working-tree file it is supposed to constrain.**
`internal/acdsl/policy.go:329-336`, reached from `cmd/acdsl/main.go:122-127`.

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)
	if baseErr != nil {
		if policy.Mode == PolicyModeFree {
			return nil, nil // no base to gate against, nothing declared — free
		}
		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
	}
	policy, err := basePolicy(ctx, root, base, policy)   // <-- authoritative re-read happens HERE
	...
```

`policy` in the `baseErr != nil` branch is still the value `LoadPolicy(root)` read from the **working tree's own `acdsl/policy.json`** — the exact file the whole feature exists to protect against edits from an agent branch. `policyBase` resolves candidates from `policy.BaseRef` when it is non-empty, trying *only* that one candidate. An agent on a gated or strict repo can edit its local `acdsl/policy.json` to:

```json
{"mode": "free", "base_ref": "no-such-ref-xyz"}
```

`policyBase` then fails to resolve any candidate → returns an error → `CheckPolicy` sees the (tampered) `policy.Mode == PolicyModeFree` and returns `nil, nil` — no diagnostics, no error, `acdsl check` exits 0. The base-ref-authoritative `basePolicy()` call that the rest of the function correctly relies on (and which *does* close the naive "just edit mode to free" bypass) is never reached, because the short-circuit sits before it. No commit is required — `acdsl check` reads the working tree directly (confirmed: no staging requirement blocks this), and the fail-open `cmd/hooks/acdsl-policy-guard.sh` is explicitly documented as advisory only ("the authoritative layer is the binary check"), so this is a full bypass of the sole authoritative layer, not a redundant one.

This is exactly the attack the plan's own hot-item called out as the reason `CheckPolicy` must live in the binary rather than a data file (D2: "not bypassable via data files"). It also directly contradicts D10 ("an unresolvable base is a hard tool error, never a silent pass") — that guarantee only holds when the *untampered* mode reaches this branch.

Confirmed no test exercises this combination: `TestCheckPolicyUnresolvableBase` (`internal/acdsl/policy_test.go:280`) sets `BaseRef` to `"nosuchref"` but leaves `Mode` at `gated` (via `gatedPolicy()`), so it only proves the honest-gated path errors correctly — it never combines the bad ref with a tampered `mode: free`.

**Fix:** move the `policy.Mode == PolicyModeFree` fast path to *after* the `basePolicy` call succeeds (there already is one, at line 341 — it's just also duplicated, unsafely, before base resolution). When `policyBase` errors, gated/strict must never fall back to "free" based on the untrusted working copy; if the intent is "a repo that has never had a policy is implicitly free", check that by asking whether the base has no policy file at all (which `basePolicy` already distinguishes via `exists`), not by trusting the local `Mode` field.

### Important (Should Fix)

**2. Undocumented new subsystem: a persistent, cross-process row cache for `print_agent_worktrees_status.sh` — confirm this was intentional and get it plan-reviewed.**
`cmd/worktrees/print_agent_worktrees_status.sh:230-334`.

The `repos-routines-improvements` plan's Current-state fact C3 describes an *existing* per-branch row cache under `<git-common-dir>/agent-status-cache/` and scopes its own change to appending a `|v2` version salt to the cache key (D3). I checked: **this cache does not exist anywhere in the base commit** (`git grep agent-status-cache b4981b88` returns nothing) — the entire mechanism (cache directory creation, per-branch cache files, the 5-line serialized format, the stale-branch pruning loop, the `WTS_CACHE_DIR`/`WTS_CANDIDATES_DIGEST` plumbing through the parallel `xargs` row workers) is new in this diff, not merely versioned.

This isn't a request to revert it — I read it carefully and it looks correct: keys are content-derived (branch tip, FROM tip, and a digest of every candidate ref, so any relevant git-state change invalidates the row), writes are atomic (`mktemp` + `mv -f` per branch file, so parallel workers never race on the same file), and reads validate both the key line and the line count before trusting a cached row. It's a reasonable answer to the fact that layer 4's `git merge-tree` calls make every row noticeably more expensive to compute from scratch. But per ACTION-REVIEW-SPEC-005 / ACTION-IMPL-004, a new persistent on-disk cache for output that feeds directly into the worktree-removal safety gate is exactly the kind of addition that should have gone through the plan's Decisions table (there is a `D3` about versioning an existing cache, but no decision about *introducing* one). Please confirm this was a deliberate, reviewed choice rather than a discovery that should have paused for the "materially exceeds scope" check.

**3. Minor plan/reality gap worth flagging, not fixing:** the `repos-routines-improvements` plan's baseline facts (C1-C3, C11) describe the pre-squash-layer `evaluate_branch`/cache state as already present. Given point 2 above, at least C3 was not accurate against the actual base commit at plan-writing time (or the plan was written against a different, not-yet-merged intermediate state). Not a code defect, but worth noting so future plans in this repo don't inherit a stale "current state" section as fact.

### Minor (Nice to Have)

**4. `internal/repos/registry.go:54-65`** — `Registry`'s field order (`path`, `mu`, `loadedModTime`, `loadedSize`, `repos`) isn't alphabetical (RULE-GOLANG-STRUCT-002: alphabetical within group). The struct was touched in this diff (two fields added), so it's in scope; pre-existing (the blank line after `path` predates this change) but worth fixing while the struct is already being edited: `loadedModTime, loadedSize, mu, path, repos`.

**5. `internal/repos/registry.go:360-380`** — `WorktreeKindCount` (a type) is declared in the middle of the free-function section, between `Fingerprint` and `WorktreeCounts`, rather than with the other type blocks near the top of the file (RULE-GOLANG-FILE-001: type blocks before free functions). Cosmetic; the file already isn't strictly organized this way for its older content, but a new type is a good place to start conforming.

**6. `cmd/worktrees/print_agent_worktrees_status.sh`** — the header comment says `WORKTREE_STATUS_NO_CACHE=1 bypasses the cache (tests, diagnosis)`, but the implementation only bypasses the *read* path; a `NO_CACHE=1` run still recomputes-and-writes the row into the persistent cache (`print_row`'s `if`/`else` always ends in the same `mv -f` write). This is arguably fine behavior (a "force refresh" rather than "no cache at all" semantics), but the doc comment overstates what the flag does — worth a one-line clarification so a future reader doesn't assume diagnostic runs leave no trace.

## Testing note

`go build ./...` and `go vet ./...` are clean on this checkout. A `go test ./...` run in this exact working copy showed several failures (`TestDistShipsReachCoveredSliceWithBinaries`, `TestCheckParamsPassedSorted`, `TestOverview`, `TestReposChooseFolder`) that I chased down and can rule out as code defects:

- `TestDistShipsReachCoveredSliceWithBinaries` failed because this working tree's `acdsl/registry.json`/`rules.acdsl`/`policy.json` (and several `context/*` files, `AGENTS.md`) carry **uncommitted local modifications already present before I ran anything** (same mtime across all of them) that rewrite the registry into its *dist-shipped* form (verifier `argv` pointing at `bin/verifiers/<name>` instead of the source `go run ./cmd/acdsl/verifiers/<name>`). `git diff` of the reviewed commit range shows **no changes** to these files — this is sandbox/session contamination local to my checkout, not part of the diff under review.
- The other three failures did not reproduce when run individually or in an independent worktree checked out fresh at the head commit; they only appeared under full-suite `go test ./...` on a machine that (per `git worktree list`) is concurrently running dozens of other review sessions against this same commit. I'm treating these as resource-contention flakes, not defects — `go test ./internal/acdsl/... -run TestCheckParamsPassedSorted` and `go test ./internal/server/... -run 'TestOverview$|TestReposChooseFolder'` both pass cleanly in isolation on a clean worktree at head.

I could not exercise the live headless bootstrap wrapper, the Windows-specific `install.ps1`/`launchctl` paths, or the config-server UI end-to-end (no running server, no macOS launchd cycle in this sandbox) — those are marked `not tested` in the plans themselves and I have no way to add coverage here; treat this review's testing claim as "build/vet/unit-tests verified", not "full runbook verified".

## Recommendations

- Add a test case combining a tampered `mode: free` with an unresolvable `base_ref` in the *working-tree* policy to `TestCheckPolicy` (finding 1) — this is the one scenario the current suite structurally cannot catch because every existing case constructs the passed-in `Policy` directly rather than round-tripping it through `LoadPolicy` on a tampered file.
- If the row cache (finding 2) is kept, it deserves its own short addendum to the plan (or a follow-up plan) describing the invalidation contract explicitly, since it's now load-bearing for the removal safety gate's performance and correctness under concurrent scans.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The breadth of this release is well-executed — registry/cache correctness, the peek session-attribution fix, and the bootstrap wrapper rewrite are all solid, plan-conformant work, and the build/vet/test baseline is clean once environmental noise is excluded. But the ACDSL policy-modes feature — the one piece of this release whose entire purpose is a security boundary — has a straightforward bypass of its authoritative enforcement layer via the exact untrusted input it exists to constrain. That must be fixed (and covered by a test) before this ships; everything else here can land as-is or as fast follow-ups.
