# Review: smine v1.1.4 → v1.2.1

Range `b4981b88..a342811b` (178 files, +7314/-749), excluding `sessions/`, `evals/`, `proposals/`, `plans/`.
Plans: `plans/v1-2-1/*` (9 change plans + a session-context-report plan).

Reviewed in passes: (1) plan TLDRs/decisions for all 10 plan docs, (2) `internal/acdsl/policy.go` +
its git/dist helpers end-to-end (new self-management-mode feature, security-relevant), (3)
`internal/repos/registry.go` + `internal/server/statuscache.go` + the `repos.go` wiring (cache
fix), (4) `cmd/worktrees/_lib/verdict.sh` squash-containment layer + its tests, (5) the two new
PreToolUse hooks (`acdsl-policy-guard.sh`, `skill-invoke-gate.sh`), (6) `install.sh` / peek
multi-profile port logic, (7) `internal/routines/{bootstrap_darwin,routines,plist}.go`, (8)
`routines/smine-nightly/run.sh` (orchestrate stage), (9) `internal/server/profilesettings.go` +
i18n, (10) build/vet/full test suite at HEAD.

`go build ./...` and `go vet ./...` are clean. `go test ./...` is green except one failure
(`TestDistShipsReachCoveredSliceWithBinaries`) that does **not** reproduce on a clean checkout of
`a342811b` (verified in a scratch worktree) — it's this eval harness's worktree carrying a
pre-existing dirty `acdsl/{registry.json,rules.acdsl,policy.json}` from session bootstrap, predating
my session (file mtimes are ~25 minutes older than my first command). Not a code defect; noted only
so it isn't mistaken for one.

## Strengths

- **`internal/repos/registry.go` read-modify-write rework** is exactly right: `Add`/`Remove` now
  re-read `repos.json` from disk under the lock before mutating, closing the "second server instance
  resurrects a deleted repo" bug (FACT-REPO-ARCH-016) without over-engineering into cross-process
  IPC, which the fact explicitly said was out of scope. `refreshIfChanged`'s stat-before-read
  ordering is correct and self-correcting under races (a mtime/size collision from a write-during-stat
  gets picked up on the *next* poll, not lost).
- **`internal/server/statuscache.go`** is small, well-isolated, and its "stored slices are immutable"
  contract is honored everywhere I checked (`DropBranches` rebuilds rather than mutates in place).
- **Squash-containment layer (`verdict.sh`)** — `git merge-tree --write-tree` equality-to-candidate
  is a sound, content-based containment test (can't false-positive into the deletion gate the way a
  subject/patch heuristic could), it's layered *after* the three existing detectors rather than
  replacing them, and it ships with real tests for both the positive case and the conservative
  "main edited the squashed content further → stays UNPICKED" case. The cache-key version bump
  (`|v2`) that the plan's D3 calls for landed correctly, avoiding stale pre-squash-layer verdicts.
- **`FACT-REPO-ARCH-017` fix** (`worktreeSessions` in `repos.go`): branch-identity lookup with cwd as
  the detached-HEAD fallback is exactly the fix the fact prescribes, and it's a small, readable
  function.
- Good self-awareness in the diff of its own blast radius: `WorktreeCounts`'s signature change is a
  real signature change (not a `*V2` sibling), and the routine `overrideLabels` rename/behavior
  change is fully re-tested rather than patched around.
- `install.sh`'s per-profile port persistence (`install.env`) correctly makes "environment variable
  overrides the file" work even though sourcing the file clobbers the same-named shell variables —
  the `ENV_*` snapshot-before-source pattern is a deliberate, correct piece of shell engineering, not
  an accident (verified by tracing both the fresh-install and re-run-with-override cases by hand).
- Test coverage for the riskiest new logic (`policy_test.go`, `verdict.sh`'s new squash tests, the
  status-cache invalidation test) is genuinely behavioral, not mock-shaped.

## Issues

### Critical (Must Fix)

**`internal/acdsl/policy.go:329-336` (`CheckPolicy`) — the gated/strict self-management mode has a
complete, deterministic bypass.**

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)
	if baseErr != nil {
		if policy.Mode == PolicyModeFree {
			return nil, nil // no base to gate against, nothing declared — free
		}
		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
	}
	policy, err := basePolicy(ctx, root, base, policy)   // <- authoritative override happens HERE
	...
```

`policy` on entry is whatever `cmd/acdsl/main.go` got from `acdsl.LoadPolicy(*root)` — i.e. read
straight from the **working tree's** `acdsl/policy.json`, fully attacker-controlled on an agent
branch. `policyBase` resolves the base commit via `git merge-base HEAD <candidate>`, and when
`policy.BaseRef` is set it uses *only* that value as the candidate (skipping the
`origin/HEAD`/`origin/main`/`main`/`master` fallbacks). The authoritative re-read
(`basePolicy`, which fetches the *real* mode from the base commit via `git show`) only happens
*after* `policyBase` succeeds.

So: an agent branch that edits its own `acdsl/policy.json` to
`{"mode": "free", "base_ref": "no-such-ref"}` causes `policyBase` to fail (no candidate resolves),
and the `baseErr != nil` branch then trusts the **same untrusted `policy.Mode`** to decide whether
that's fatal. Since the attacker set it to `free`, `CheckPolicy` returns `(nil, nil)` — clean, no
error, no diagnostics — before `basePolicy` (the actual authority) is ever consulted. This works
regardless of the real base commit's mode (`gated` or `strict`), and regardless of git topology (no
missing remote or unusual clone is needed — setting `base_ref` to a bogus string is sufficient by
itself, in any repo).

This directly contradicts the feature's stated invariant, both in the plan
(`2026-08-26-acdsl-policy-modes-done-a7cd7bf`: *"An agent in a gated target repo can add/tune ...
but cannot touch PLAN rules, add validators, or weaken the policy"*) and in this file's own doc
comment on `CheckPolicy` (*"The base-ref policy is authoritative — a branch cannot weaken the mode
by editing or deleting the working policy.json"*). The `cmd/hooks/acdsl-policy-guard.sh` PreToolUse
hook (which denies `Write`/`Edit` to `acdsl/policy.json` when the *current* mode is strict/gated) is
the only thing standing between an agent and this edit in an interactive session, and it is
explicitly documented as fail-open/non-authoritative, only guards three tool names (not `Bash`), and
`acdsl check` is the thing everything else (CI, `make audit`) actually trusts.

Why exploitability doesn't depend on unusual repo state: `policy.BaseRef` is attacker-supplied data
read directly from the working tree, and setting it to garbage is the *simplest* way to make
`policyBase` fail — no missing remote or detached clone required.

Fix direction: never let the unauthoritated working-tree `policy.Mode` decide whether an
unresolvable base is fatal. Two independent changes close it:
1. In `policyBase`, always try the well-known fallback candidates (`origin/HEAD`, `origin/main`,
   `main`, `master`) in addition to `policy.BaseRef` — treat `BaseRef` as an extra candidate to try
   first, not a replacement for the list, so a bogus `BaseRef` alone can't force `baseErr`.
2. Independently, `CheckPolicy`'s `baseErr` branch should not special-case free at all (or should
   gate that special-case on some fact that isn't itself attacker-writable — e.g. "no
   `acdsl/policy.json` exists in the working tree", which `LoadPolicy` already handles before this
   function is ever called with a *populated* `Mode`). If a base genuinely cannot be resolved in a
   real repo, that should be a loud error in every mode, matching `policyBase`'s own doc comment
   ("none resolving is a hard error, never a silent free").

Test to add: gated/strict base policy, agent branch sets `base_ref` to a nonexistent ref name and
`mode` to `free`, assert `CheckPolicy` returns a non-nil error (or still detects the surface/rule
violations) — the existing `TestCheckPolicyUnresolvableBase` only covers this with `Mode: gated`
already set on the *passed-in* policy, which never exercises the exploitable branch.

### Important (Should Fix)

- **`internal/server/repos.go` — `handleRepoRemove` deleted, deviating from plan D4.** The
  `repos-routines-improvements` plan's decision table (D4) says: *"`handleRepoRemove` drops its
  branch on success"* — i.e. keep the handler, add the same `statusCache` drop `handleRepoRemoveSelected`
  gets. The plan's own "Deferred findings" section is explicit that *whether to delete the route
  outright* is "left to a separate cleanup decision," not this plan's call. The diff instead deletes
  both the handler and its route (`POST /repos/{name}/branches/{branch}/remove` — see
  `internal/server/server.go`, `internal/server/repos.go`) rather than adding the cache drop the
  decision calls for. Functionally this may be fine (the plan itself notes it's unreachable from any
  template), but it's a case where the implementation overrode an explicit `[USER]`-adjacent decision
  and an explicit scope boundary without a Changelog entry recording the deviation. Flag for
  confirmation — if intentional, the plan's D4 and "Deferred findings" bullet are now stale and
  should get a Changelog row.

- **The "casual lockout" ownership gate is prompt-only, not code-enforced.** The
  `bootstrap-profile-improvements` plan's headline safety property — casual installs must never
  propose or apply edits to skills the user doesn't own, "enforced at extraction (smine-skills),
  apply (smine-apply, hard gate), and the orchestrate judge" — is implemented entirely as prose in
  `skills/smine/smine-skills/SKILL.md`, `skills/smine/smine-apply/SKILL.md`, and
  `skills/smine/smine-orchestrate/SKILL.md`. There is no Go-level check anywhere (`internal/evals`,
  `internal/skills`, the config-server) that reads a skill's `origin:` frontmatter and refuses a
  write. That matches this repo's existing architecture (skills, not code, are the orchestration
  layer, and the "hard gate" language elsewhere in this codebase — e.g. ACTION-* entries — is
  routinely prompt-level too), so this isn't a novel pattern break, but it's worth stating plainly:
  a "hard gate" that only exists as instructions to the same LLM whose output it's supposed to
  constrain is a softer guarantee than the plan's language implies, and it's exactly the kind of
  property that degrades silently if a future skill edit drops the paragraph. Worth at minimum a
  one-line acknowledgment in the plan of what "hard gate" means here.

- **`cmd/hooks/skill-invoke-gate.sh:64-73` — the Bash-command context-read guard is trivially
  bypassable.** The `case "$command" in *";"*|*"|"*|... ) ;; *) ... esac` skips the whole
  target-extraction path (leaving `target=""`, i.e. no denial) for *any* command containing a quote,
  parenthesis, or several other common characters — which includes the quoted form of the exact same
  `cat` invocation the guard is trying to catch (`cat "$ctx_dir/foo.md"`), plus `head`, `less`,
  `sed -n`, `awk`, `python -c`, etc. The hook's own docstring calls it "fail-open by design" for
  missing tooling/transcripts, but this specific gap (a same-shaped command in different clothing
  defeats it) reads more like an incomplete allowlist than an intentional degrade. Given the file's
  purpose (stopping context leakage in eval lanes, `ACTION-REVIEW-*` observability work), either
  tighten the command matcher (e.g. tokenize with `jq`'s shell-words or match on the *resolved*
  absolute path via `realpath`/`readlink` regardless of quoting) or drop the Bash-command branch
  entirely and rely on the Read-tool guard, which isn't quote-bypassable.

### Minor (Nice to Have)

- **`internal/server/repos.go:363` (`handleRepoWorktreeStatus`) — stale `ScannedAt` on a failed
  scan.** When the fingerprint doesn't match (or `?refresh=1`) and the fresh `repos.Status` call then
  errors, `entry` is never reassigned from `s.statusCache.Store(...)`, so `data.ScannedAt` renders
  whatever the *previous* successful scan's timestamp was, next to `data.StatusErr` and a
  (presumably empty) `data.Statuses`. Cosmetic — the timestamp will read as if the visible (empty)
  table is fresh. Consider clearing `data.ScannedAt` (zero value) on the error path so the template
  doesn't imply the empty result is current.
- **`install.sh:172-179`** — `pid="$(lsof -tiTCP:"$peek_port" -sTCP:LISTEN || true)"` followed by
  `kill "$pid"` will pass a multi-line string as a single argument to `kill` if `lsof` ever reports
  more than one PID on the port (rare for a single TCP listener, but not impossible under
  `SO_REUSEPORT`). Not observed to matter in practice; `for pid in $pid; do kill "$pid"; done` would
  be more robust for free.
- `internal/acdsl/policy.go`'s `overlayViolations` reads `acdsl/registry.local.json` straight off
  disk (not via the base-commit diff `ruleDelta`/`surfaceDelta` machinery uses), so a pre-existing,
  already-committed, unsanctioned overlay entry from *before* the policy was ever turned on will
  flag on every single `acdsl check` run in that repo, forever — not only on the branch that
  introduced it. That's arguably correct behavior for a frozen surface, but it means adopting `gated`
  mode in a repo with a legacy uncommitted-locally overlay is a hard stop rather than "only new
  drift is punished." Worth a one-line note in `acdsl/README.md`'s Modes section if not already there
  (I didn't check the doc for this specific nuance given the size of the diff).

## Recommendations

- Add the `base_ref`-poisoning regression test described under the Critical finding before closing
  out the acdsl-policy-modes plan (it's marked `done` in `plans/v1-2-1/`, so this should probably be
  a follow-up commit/plan rather than reopening that plan wholesale).
- The plan-deviation findings above (`handleRepoRemove`, the casual-lockout enforcement layer) are
  both cases where a plan's decision table said one thing and the shipped code did something
  adjacent but different; given the volume of plans in this release (9 plans, several marked `wip`
  in-repo rather than archived), it would be worth one pass reconciling each plan's Changelog against
  what actually landed, since the review above only sampled two of the nine plans' full Changes
  sections against the diff in depth.
- Given the size of this range (178 files across 9+ independent plans), a future release this size
  would benefit from being reviewed plan-by-plan against its own diff scope rather than as one
  combined range — several of the plans here (peek multi-profile ports, repo-cache reload, plan
  viewer fixes) received only a spot-check in this pass because the acdsl-policy and repos/verdict.sh
  areas dominated the available review budget by risk tier.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of the release (registry/cache correctness fixes, squash-detection layer,
routine/launchd default-disabled handling, i18n/profile plumbing) is solid, well-tested, and matches
its plans. But the acdsl self-management policy feature — the one piece of this release explicitly
framed as a security boundary ("structurally closed" failure mode) — has a deterministic bypass that
defeats it entirely in both `gated` and `strict` mode, in any repo, with a two-field edit to a file
an agent already has (fail-open, tool-scoped) hook protection against but no authoritative protection
from. That should block merge of the policy feature specifically until fixed; nothing else in this
diff rises to that bar.
