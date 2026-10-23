# Review: smine v1.1.4 → v1.2.1

Range: `b4981b88`..`a342811b` (excluding `sessions/`, `evals/`, `proposals/`, `plans/`), against the
plans under `plans/v1-2-1/`.

Scope note: 178 files, +7314/-749 across ~9 independent feature plans (ACDSL self-management
policy, repo registry/status-cache reload, headless bootstrap wrapper, casual/dev-mode presentation
profile + style mining, multi-profile peek ports, repos/routines UI, railroad-review observability,
plan-format viewer fixes, session-report context filters). Reviewed in passes: (1) full diff stat +
plan TLDRs for coverage map, (2) deep read of the highest-risk new subsystems (ACDSL policy engine +
its PreToolUse hook, repo registry read-through, status cache, peek session index, bootstrap
wrapper, railroad-review skill-invoke gate), (3) targeted diff reads of the remaining server/routine
files, (4) full build/vet/test/audit run against the actual head commit in an isolated clean
worktree (see Testing note below — the assigned checkout itself was locally contaminated and is not
representative of the committed state).

## Strengths

- **Registry read-through fix** (`internal/repos/registry.go`) directly resolves the documented
  resurrection bug (FACT-REPO-ARCH-016): `Add`/`Remove` now re-read `repos.json` under the lock
  before mutating, and `refreshIfChanged` invalidates on mtime+size rather than a TTL. Clean
  read-modify-write, well tested against real files on disk (no mocks).
- **Status cache + git fingerprint** (`internal/server/statuscache.go`, `repos.Fingerprint`,
  `repos.go` `handleRepoWorktreeStatus`) implements the three-mechanism cache-consistency fix from
  the plan exactly as designed: a cheap fingerprint (branch tips + worktree set) gates reuse of the
  last scan, `?refresh=1` and repo-ops force a rescan, and `DropBranches`/`Delete` keep the optimistic
  UI and the cache in agreement. Good separation of concerns, easy to reason about.
- **ACDSL policy engine** (`internal/acdsl/policy.go`) is a solid design: the privilege boundary is
  git topology (merge-base against an authoritative base-branch policy, never the working copy), an
  unresolvable base is a hard error in strict/gated rather than a silent free mode, and task-lifetime
  contracts are cleanly exempted. Test coverage (`policy_test.go`) uses real temporary git repos for
  every mode/scope/overlay/task-exemption/unresolvable-base scenario — no mocks, exercises the actual
  git plumbing.
- **peek.SessionIndex** (`internal/peek/peek.go`) cleanly fixes the pool-recycled-worktree
  misattribution bug (FACT-REPO-ARCH-017): branch identity now wins over cwd, with HEAD/branchless
  sessions correctly excluded from `ByBranch`. Table-driven tests cover the new branch dimension,
  the old cwd behavior, and the boundary cases together.
- **railroad-review observability fix** (`skills/quality/railroad-review/workflows/railroad-review.js`,
  `cmd/hooks/skill-invoke-gate.sh`) is a coherent fix for the CTX-002 context-leak finding: the lane
  prompt now carries `REQUIRE-SKILL:railroad-review`, the new gate hook blocks every other tool call
  until the Skill tool is actually invoked, and it separately denies direct reads of injected context
  files post-invocation (language guides excepted). This one has real, thorough test coverage
  (`cmd/tests/test_skill_invoke_gate.sh`) including the prompt-start-slash headless path.
- **Bootstrap wrapper** (`cmd/bootstrap/run.sh`) correctly works around the documented headless
  limitations (FACT-REPO-ARCH-013: no Skill/Workflow tool, ignored frontmatter `allowed-tools`) by
  moving sequencing and permissions into the wrapper, mirroring the existing nightly pattern. Good
  dry-run test coverage (`cmd/tests/test_bootstrap_wrapper.sh`) for `--since`-vs-`--last`, the
  casual-forces-dev-mode-off rule, env-override precedence, and the clean-tree/main preflight gate.
- `internal/server/sessions.go`/`internal/sessions/sessions.go`: the scope-archive rewrite
  (`ArchiveScope`/`UnarchiveScope`/`DeleteScope`) validates every folder name against path traversal
  and the reserved `archived` container before any filesystem operation, consistently across all
  three entry points.

## Issues

### Critical (Must Fix)

**1. The ACDSL fast-feedback hook blocks the exact overlay edits the policy is designed to allow.**
`cmd/hooks/acdsl-policy-guard.sh:43`
```sh
case "$rel" in
  acdsl/policy.json|acdsl/policy.schema.json|acdsl/registry.json|acdsl/registry.local.json|acdsl/evalgen.json)
    deny "acdsl policy mode is $mode: the self-management surface ($rel) changes only on the base branch (see acdsl/README.md, Modes)" ;;
esac
```
This denies **every** write to `acdsl/registry.local.json` whenever the repo's policy mode is
`strict` or `gated` — full stop, unconditionally.

But `acdsl/registry.local.json` is explicitly **not** part of the frozen self-management surface:
- `internal/acdsl/policy.go`'s `policySurface` (lines 31-36) lists only `policy.json`,
  `policy.schema.json`, `registry.json`, `evalgen.json` — `registry.local.json` is deliberately
  absent.
- The authoritative Go check (`overlayViolations`, `policy.go:143-173`) instead reads
  `registry.local.json` and flags only entry *names* missing from `policy.LocalOverrides` — i.e. it
  is designed to be edited locally, gated per-entry, not frozen.
- `acdsl/README.md` ("Modes" section) says so explicitly: *"Targets own their side: rules in any
  `*.acdsl` file, verifier overrides in `acdsl/registry.local.json` ... In a `strict`/`gated` repo
  the overlay is policy-gated: only entries listed in the policy's `local_overrides` are accepted."*

Since every distributed target gets `dist_mode: gated` by default (`acdsl/README.md`, "smine itself
runs `mode: "free"`... targets get the shipped `dist_mode` (default `gated`)"), this means **no
target repo running under Claude Code can ever successfully add a sanctioned local verifier
override** — the fast-feedback hook denies the Write/Edit before the authoritative, more permissive
check ever runs, for every target repo shipped by this release. The hook's own deny message ("the
self-management surface... changes only on the base branch") is simply wrong for this file: the
whole point of `local_overrides` is that it does *not* have to change on the base branch.

There is no test for `acdsl-policy-guard.sh` (contrast with `skill-invoke-gate.sh`, which has full
coverage in `cmd/tests/test_skill_invoke_gate.sh`) — that is almost certainly why this shipped.

Fix: drop `acdsl/registry.local.json` from the hook's frozen-surface case entirely (its correctness
is judged by name against `policy.local_overrides`, which the hook would need to read from
`acdsl/policy.json` to replicate `overlayViolations` — or, simpler, just don't gate this file at the
hook layer at all and let the authoritative `acdsl check` catch unsanctioned entries, same as it
already does for gated rule scopes).

### Important (Should Fix)

**2. `shipBinary`'s Windows fallback can ship a mis-named prebuilt binary.**
`internal/acdsl/dist.go:285-297`
```go
func shipBinary(ctx context.Context, root, srcDir, destDir, name, pkg string) error {
	out := name
	if runtime.GOOS == "windows" {
		out = name + ".exe"
	}
	for _, prebuilt := range []string{filepath.Join(srcDir, name+".exe"), filepath.Join(srcDir, name)} {
		if info, err := os.Stat(prebuilt); err == nil && !info.IsDir() {
			if err := fsx.CopyFile(prebuilt, filepath.Join(destDir, filepath.Base(prebuilt))); err != nil {
```
The prebuilt-copy branch names the destination file after `filepath.Base(prebuilt)`, not `out`. On
Windows, if only a bare `name` (no `.exe`) prebuilt exists — e.g. a stale build artifact left over
from a non-Windows dev machine syncing its `bin/` into a shared location, or a future packaging
change that stops guaranteeing `.exe` — the shipped verifier binary silently loses its `.exe`
extension, which the code's own comment says matters ("Windows outputs always carry `.exe` so
CreateProcess and Git Bash agree the file is executable"). Low likelihood today (the installer
always produces `.exe`), but the invariant isn't enforced by the code that's supposed to guarantee
it. Fix: `fsx.CopyFile(prebuilt, filepath.Join(destDir, out))`.

### Minor (Nice to Have)

**3. `skill-invoke-gate.sh` prompt-start match is a prefix, not an exact/boundary match.**
`cmd/hooks/skill-invoke-gate.sh:42`
```sh
case "$first_user" in "/$required"*) invoked=1 ;; *) invoked="" ;; esac
```
A first message beginning with `/railroad-review-observability ...` would satisfy
`required="railroad-review"` today, wrongly counting as invocation of a different skill. Not
exploitable with the current skill roster (no skill name is a dash-extension of another), and not
covered by a test case, but worth anchoring on a space/newline/end-of-string boundary
(`"/$required" | "/$required "*` or a `[[ $first_user =~ ^/$required($|[[:space:]]) ]]` equivalent)
before a future skill name collides.

**4. Plan-dir hygiene:** all plan directories under `plans/v1-2-1/` are still `-wip-` (only
`2026-08-26-acdsl-policy-modes-done-a7cd7bf` is `-done-`), even though this range is the merged
v1.2.1 release commit. Per RULE-PLAN-073 these should move to the plans-archive repo renamed
`done-<merge-hash>` after merge. Likely a pending housekeeping step rather than something blocking
this review, but flagging since the repo convention says code repos never hold `done` dirs and none
of these are archived yet either.

**5. `writePolicy` (`internal/acdsl/dist.go:235-261`) forwards `BaseRef` verbatim into every
distributed target's `policy.json`.** `Mode`/`DistMode` are explicitly rewritten for distribution,
but a smine-side `base_ref` (if ever set) would leak unchanged into every target, where it likely
doesn't resolve. Currently moot — smine's own `acdsl/policy.json` sets no `base_ref` — but there's no
field or logic analogous to `dist_mode` to override it per-target if that ever changes.

## Plan Alignment

I checked the implementation against each plan's stated Result:
- `2026-08-26-acdsl-policy-modes` (done): implemented as designed, modulo Critical finding #1 above
  — the Go-level enforcement matches the plan precisely; the hook layer diverges from it.
- `2026-08-30-repo-cache-reload`: all three described fixes present and match (registry read-through,
  fingerprint-gated status cache, dead single-branch remove endpoint deleted — confirmed
  `handleRepoRemove` removed from `internal/server/repos.go` and its route from `server.go`).
- `2026-08-27-bootstrap-profile-improvements` (both change docs): bootstrap form/`--since`, Casual
  rename, casual lockout (prompt-level, verified present in `smine-apply/SKILL.md`), scope-concept
  retirement (`sessions.go`/`sessions/sessions.go` archive model), Reload-button removal (confirmed:
  `POST /sessions/reload` route deleted, `reloadSessions()` now runs on every GET) — all present. The
  headless-bootstrap rebuild (wrapper-orchestrated stages, `/smine-bootstrap` skill deletion, dev-mode
  gating, presentation contracts) is present and matches; I did not deep-verify the presentation
  contract enforcement inside the dimension skills' prose (out of Go-code scope).
- `2026-08-30-peek-multiple-profiles`: `internal/config/installenv.go` (`ExpandInstallMarkers`) and
  `peek.SessionIndex`/`git_branch` plumbing match; I did not have a second peek-mcp binary available
  to verify the `GET /healthz` identity check end-to-end (that lives in the peek-mcp repo, out of this
  diff's scope — this repo only pins the version).
- `2026-08-27-repos-routines-improvements`: worktree-column per-kind counts (`WorktreeCounts`),
  optimistic-delete-vs-cache fix (`DropBranches`) both confirmed. I did not trace the `git
  merge-tree --write-tree` squash-detection layer or the run-history weekly-band JS in this pass.
- `2026-08-26-railroad-review-observability`: branch namespace move and the lane-prompt
  Skill-invocation fix are both present and consistent (code review above).
- No deviations found that look like undocumented departures from an approved plan; the one real
  functional gap (#1) is an implementation bug against the plan's own stated behavior, not a
  deliberate deviation.

## Testing

`go build ./...`, `go vet ./...` and `go test ./...` were run directly in the assigned checkout
first. `go test` reported one failure (`TestDistShipsReachCoveredSliceWithBinaries`) and `make audit`
also failed (a missing `bin/verifiers/errorf-prefix`). Investigating: the assigned checkout
(`.../runs/sonnet-5-high-r25/artifacts/arm-wt`) has **local, uncommitted modifications** already
present before I touched it (`git status` shows `acdsl/policy.json`, `acdsl/registry.json`,
`acdsl/rules.acdsl` and several `context/` files locally modified/added/deleted, unrelated to my
session — this checkout is one of ~30 sibling eval-review worktrees sharing the same repo, and
something applied ACDSL's own dist/self-management output on top of it). I confirmed this is
checkout contamination, not a code regression, by checking out the exact head commit
(`a342811b4cf7b8bb2b54359ced6c786d3b5f349b`) into two independent clean `git worktree add --detach`
copies and re-running everything there:

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — all packages pass, including `internal/acdsl`.
- `make audit` (mod verify, vet, acdsl fixtures/check, full test suite) — passes end-to-end.

So: build, vet, tests, and the release audit gate all genuinely pass against the reviewed commit.
The assigned checkout's dirty state should probably be cleaned by whatever harness step
contaminated it, but that is outside this diff.

## Recommendations

- Add a regression test for `acdsl-policy-guard.sh` mirroring `test_skill_invoke_gate.sh`'s style —
  it's the one PreToolUse hook in this release with zero coverage, and it's exactly where the
  Critical finding above hid.
- Once finding #1 is fixed, consider a small `acdsl/testdata`-style fixture exercising the two layers
  together (hook decision + `acdsl check` verdict) for at least one gated-mode `registry.local.json`
  edit, so the two enforcement layers can't silently diverge again.
- Move the shipped `plans/v1-2-1/*-wip-*` directories through `/plan-archive` now that this range is
  merged, per RULE-PLAN-073.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The release is broad but each feature is independently well-scoped, matches its plan,
and is backed by real (non-mocked) tests that I verified actually pass against the true head commit.
The one Critical issue (the ACDSL PreToolUse hook silently blocking the sanctioned-overlay mechanism
that the same release's flagship feature — policy modes — is supposed to provide) is a one-line,
well-understood fix with no architectural implications; it should land before this ships, since it
defeats the one part of the ACDSL policy feature that target repos are expected to use day-to-day.
Everything else found is Important/Minor and does not block merging.
