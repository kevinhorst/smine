# Review: smine v1.1.4 → v1.2.1

Range: `b4981b88d5..a342811b4c` (excluding `sessions/`, `evals/`, `proposals/`, `plans/`).
178 files changed, ~7300 insertions / ~750 deletions, spanning ~10 change plans under `plans/v1-2-1/`
(repo cache/registry rework, ACDSL self-management policy, repos/routines UI, multi-profile peek,
presentation/language settings, plan-format viewer fixes, bootstrap, session-context reporting, plus
the smine skill set).

**Method:** This is too large for a single uniform pass. I read every plan doc under `plans/v1-2-1/`
that has code behind it, then reviewed the Go source against each plan's Decisions/Changes sections at
file-diff level, tier-prioritized per ACTION-REVIEW-RISK (registry/cache/policy/scheduling first). I
built and ran `go build ./...`, `go vet ./...`, and `go test ./...` against the actual commits (via a
disposable `git worktree add` off `a342811b4c`, never touching the assigned checkout's HEAD/index). I
spot-checked but did not line-by-line review every skill Markdown/template file, JS snippet, or shell
script in the diff — those were sampled where a plan's Changes section specified an exact block to
verify against.

## Strengths

- **Registry/cache rework (`internal/repos/registry.go`, `internal/server/statuscache.go`,
  `internal/server/repos.go`) matches its plan (`plans/v1-2-1/2026-08-30-repo-cache-reload-wip-46c8d9e`)
  almost line-for-line** — `readRegistrySnapshot`/`apply`/`refreshIfChanged` stat-then-reload, read-modify-write
  `Add`/`Remove`, and the fingerprint-gated status cache are exactly the designed mechanism. The plan's own
  documented deferred finding (grant-revocation asymmetry on repo delete) is correctly left alone, not
  silently fixed or silently forgotten.
- **ACDSL self-management policy (`internal/acdsl/policy.go`) is a deliberate, tested improvement over its
  own plan.** The plan's `CheckPolicy` design (`plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf`)
  short-circuits on the *working-tree* policy's `mode` field before ever computing the base — meaning an
  agent branch could set `mode: "free"` in its own edit to `acdsl/policy.json` and disable the entire gate
  before `surfaceDelta` ever runs. The shipped code closes this: `CheckPolicy` resolves the merge-base first,
  then loads the **base-ref's** committed `acdsl/policy.json` via `basePolicy()` and treats *that* as
  authoritative for every mode decision — a branch can't weaken its own gate by editing the surface it's
  supposed to be blocked from editing. This is explicitly covered by
  `TestCheckPolicyBaseModeSurvivesWorkingWeakening`. Good catch, correctly load-bearing test name.
- **Multi-profile peek identity (`cmd/configserver/main.go`)** replaces a bare TCP-listen check with a
  `/healthz` identity comparison (`claudeHome`/`codexHome`), refusing to silently share another macOS
  profile's peek-mcp instance on the same loopback port — directly closes the port-collision failure mode
  documented in this repo's own facts (`FACT-REPO-ARCH-018`-adjacent).
- **Session attribution (`internal/peek/peek.go`, `internal/server/repos.go`)** — `SessionIndex` now indexes
  both by cwd and by git branch, and `worktreeSessions` prefers branch identity with cwd as the
  detached-HEAD fallback, fixing the documented pool-worktree-recycling misattribution
  (`FACT-REPO-ARCH-017`).
- Cross-cutting consistency is good: the plan-format reorder (`RULE-PLAN-002` "Open questions" moved
  after "Decisions") is reflected identically in `cmd/acdsl/verifiers/planformat/main.go`'s `canon` slice,
  in `context/rules/plan.md`, and in the context-pack's rendered entries — rule, verifier and doc moved
  together as the plan required.
- Test coverage against plan-specified cases is thorough and present, not just claimed: I checked
  `internal/repos/registry_test.go`, `internal/server/statuscache_test.go`, and
  `internal/acdsl/policy_test.go` function-by-function against each plan's Tests table — every named case
  (`TestRegistryRefreshOnExternalEdit`, `TestRegistryAddPreservesExternalEdit`,
  `TestRefreshKeepsLastGoodStateOnParseError`, `TestFingerprint`, the full `TestCheckPolicy*` family) exists.
- `go build ./...`, `go vet ./...` are clean on the actual head commit, and `go test ./...` passes in full
  on a clean worktree of `a342811b4c` (see Testing note below on a false alarm from the assigned checkout's
  local state).

## Issues

### Critical (Must Fix)

None found in the code reviewed.

### Important (Should Fix)

None found. (Two items below were deviations worth a confirming glance, but both are documented,
justified, and — in the policy.go case — a net improvement; I list them here only because the rules ask
me to flag any deviation, not because they need rework.)

- **Deviation to confirm — `CheckPolicy` bases mode decisions on the base-ref policy, not the working-tree
  one that `LoadPolicy`/`main.go` pass in** (`internal/acdsl/policy.go:329-345` vs.
  `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/design/raw.md` §Changes-1, D2). Functionally
  this is strictly safer than the plan (see Strengths above) and is tested, so I'm not asking for a
  change — just flagging that the shipped contract differs from the written plan's pseudocode, in case
  the difference wasn't a deliberate implementation-time call.

### Minor (Nice to Have)

- `cmd/configserver/main.go:216-221` — `type peekID struct` field order is `Version, ClaudeHome,
  CodexHome, ControlPort`, not alphabetical (`RULE-GOLANG-STRUCT-002`). Should be `ClaudeHome, CodexHome,
  ControlPort, Version`.
- `internal/acdsl/git.go:78` — `GitRevParse` is exported and covered by `git_test.go` but has zero
  production callers (only `GitMergeBase`, `GitShowFile`, `GitChangedFiles`, `GitBranch` are used from
  `policy.go`). It was written out in the plan's Changes section 2 alongside the others, so it's not
  unrequested scope, but it's currently dead code in the shipped binary — either use it or drop it
  (`ACTION-REVIEW-QUALITY-004`).
- `internal/config/installenv.go:22` — `strings.Fields(diffed + "\n" + untracked)` (used identically in
  `internal/acdsl/git.go:57` for `GitChangedFiles`) splits on all whitespace, which would silently merge or
  mis-split a path containing a space. This is a pre-existing pattern elsewhere in the repo, not introduced
  here, and repo/file paths with spaces are unlikely in this codebase's own trees — flagging only because
  the exact same shape appears fresh in this diff's `GitChangedFiles`.
- `routines/README.md` documents the new `default-disabled` marker file and per-routine `Unscheduled`
  state (`internal/routines/routines.go`, `bootstrap_darwin.go`, `sched_windows.go`), but neither concept
  has a corresponding plan doc under `plans/v1-2-1/`. The code and doc are consistent and tested
  (`routines_test.go`), so this is a paperwork gap, not a functional one — worth confirming it was
  intentionally out-of-band from the plan set under review.

## Plan-alignment notes

- **Repo cache & deletion** (`2026-08-30-repo-cache-reload`): fully implemented as designed, including the
  Phase 3 dead-endpoint removal (`handleRepoRemove` and its route are gone; `grep` confirms no remaining
  references outside `RemoveSelected`).
- **Repos/routines UI improvements** (`2026-08-27-repos-routines-improvements`): the squash-containment
  layer, `WorktreeCounts` per-kind column, and the `statusCache.DropBranches`/`Delete` cache-drop wiring
  all match; I did not re-derive the bash `merge-tree` logic in `verdict.sh` by hand (shell-script
  correctness there rests on the plan's own shell test suite, `cmd/tests/test_print_agent_worktrees_status.sh`,
  which I did not execute in this pass — flagging as an explicit gap in my coverage, not a finding).
- **ACDSL policy modes**: implemented plus a justified strengthening (see Strengths/Important above).
- **Plan-format viewer fixes**: rule text, verifier, and generated context all move together correctly.
- Several other plans in the directory (`bootstrap-profile-improvements`, `smine-profile-settings`,
  `peek-multiple-profiles`, `language-style-setting`, `session-context-report`) have corresponding code
  (`internal/server/bootstrap.go`, `profilesettings.go`, `presentation.go`, `internal/config/installenv.go`)
  that I read and found internally consistent and well-scoped (atomic file writes via `fsx.ReplaceFile`,
  RWMutex-guarded `presentationStore` with pointer-swap reload, no partial-write races), but I did not
  independently re-derive every UI/template change (`internal/server/templates/*.html`) against their
  request screenshots — that verification is inherently visual and better done by re-running the app.

## Testing

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` on the assigned checkout initially reported one failure:
  `TestDistShipsReachCoveredSliceWithBinaries` (`internal/acdsl`), missing `bin/verifiers/gofmt` after
  `Dist()`. I traced this to the assigned checkout's own **uncommitted local working-tree state**
  (`git status` shows `acdsl/registry.json`, `acdsl/policy.json`, `acdsl/rules.acdsl` and several
  `context/*` files already modified before I touched anything — this repo's own tooling documents this as
  the ACDSL context-injection "projection" overlay sitting on top of governed files, per this repo's
  `AGENTS.md`: *"the projection blocks atop governed files are working-copy views... run `./bin/acdsl
  project -strip` before staging or `make audit`"*). I did not strip it (that would mutate the assigned
  read-only checkout); instead I added a disposable `git worktree add --detach <tmp> a342811b4c` from a
  clean state and reran the identical test there — it **passes** (`ok  internal/acdsl 2.8s`). This
  confirms the failure is an artifact of my review environment's pre-existing local overlay, not a defect
  in the commits under review. The temp worktree was removed afterward; the assigned checkout was never
  written to.

## Recommendations

- Confirm the `CheckPolicy` base-authoritative-mode deviation from the plan's written pseudocode was a
  deliberate implementation-time strengthening (it reads that way from the code and its test name, but the
  plan itself should probably be updated to match what shipped, per this repo's own Changelog discipline).
- Consider filing the `GitRevParse` dead-export as a quick follow-up removal, or wire it into whichever
  future use was anticipated.
- Since `routines/README.md`'s `default-disabled`/`Unscheduled` additions have no plan doc in this set,
  a one-line note in a plan Changelog (or confirmation that this shipped under a different, already-archived
  plan) would close the paperwork loop per `ACTION-REVIEW-SPEC-005`.

## Assessment

**Ready to merge?** Yes.

**Reasoning:** The highest-risk surfaces in this release — the repo registry/status-cache invalidation
rework and the new ACDSL self-management policy gate — are implemented to spec (and in the policy case,
implemented *more safely* than spec, with a test proving it), build and test cleanly against the actual
commits, and show no critical or important defects. The only items raised are a one-line struct-field-order
nit, a dead exported helper, and two paperwork/traceability notes — none block merging.
