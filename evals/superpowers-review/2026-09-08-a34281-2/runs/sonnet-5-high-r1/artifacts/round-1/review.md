# Review: smine v1.1.4 → v1.2.1

Range reviewed: `b4981b88d5..a342811b4c` (excluding `sessions/`, `evals/`, `proposals/`, `plans/`).
178 files changed, +7314/−749.

## Method

Given the size (178 files, ~7.3k added lines) this was reviewed in risk-tiered passes rather than
line-by-line: (1) core Go packages most likely to hold the release's real bug fixes
(`internal/repos`, `internal/peek`, `internal/routines`, `internal/acdsl`, `internal/server` —
presentation/profile/bootstrap/statuscache/repos/sessions), cross-checked against the
`context/facts/*` claims in the context pack (FACT-REPO-ARCH-016/017 etc.); (2) the new/changed
shell surfaces with real blast radius (`install.sh`, `cmd/sync/sync_skills.sh`,
`cmd/hooks/acdsl-policy-guard.sh`, `cmd/hooks/skill-invoke-gate.sh`,
`routines/smine-nightly/run.sh`, `cmd/bootstrap/run.sh`, `cmd/worktrees/_lib/verdict.sh` +
`print_agent_worktrees_status.sh`); (3) plan-alignment spot checks against
`plans/v1-2-1/2026-08-27-smine-profile-settings-wip-ca34568/` and
`plans/v1-2-1/2026-08-24-language-style-setting-wip-ef09567/`; (4) a full build/vet/test run.
Skill/routine markdown (`skills/**/SKILL.md`, `changelog.json`) and HTML templates were read where
they intersect the Go handlers above but not exhaustively line-by-line.

Build/test verification: `go build ./...` and `go vet ./...` are clean, and `go test ./...` passes
in full **on a fresh, detached worktree checked out at the exact head commit**. Running the suite
directly inside the provided `arm-wt` checkout produces one failure
(`TestDistShipsReachCoveredSliceWithBinaries`, stat `bin/verifiers/gofmt: no such file`) — this is
caused by pre-existing local drift in that checkout: `acdsl/registry.json`, `acdsl/policy.json`,
`acdsl/rules.acdsl` and several `context/*.md` files are locally modified/untracked relative to
HEAD (confirmed via `git status`/`git diff HEAD`), leaving a "shipped" form of `registry.json` in
place of the source form the test expects. It reproduces green in a clean worktree of the same
commit, so this is environmental, not a defect in the reviewed diff.

## Strengths

- **`internal/repos/registry.go`** — `Add`/`Remove` now read-modify-write against a fresh
  `readRegistrySnapshot` of the file instead of the in-memory slice, closing the documented
  last-writer-wins bug (FACT-REPO-ARCH-016) where a second long-lived configserver could resurrect
  a deleted repo. `refreshIfChanged`'s mtime+size fingerprinting is a sensible cheap staleness
  check, and `Registry.save` still does the atomic tmp+rename. Well covered by
  `registry_test.go`.
- **`internal/peek/peek.go`** — `SessionIndex` (replacing `SessionsByCwd`) exactly implements the
  documented fix (FACT-REPO-ARCH-017): branch-identity attribution (`ByBranch`) alongside the old
  cwd map, `keepNewest` shared correctly between both, `"HEAD"`/empty branch excluded from
  `ByBranch`. All three call sites (`repos.go`, `routines.go`, `bootstrap.go`) were updated
  consistently — no dangling caller of the old method.
- **`internal/routines`** — the `default-disabled` marker + explicit "enabled" override
  (`overrideLabels`, `SyncAll`), the `Unscheduled` vs `ScheduleSupported` distinction in
  `parseSchedule`, and the "started" row superseding logic in `History` are all precise, each has
  a matching new test, and darwin/windows stay behaviorally aligned (`sched_windows.go`'s
  `NotFound && DefaultDisabled` skip mirrors the darwin gate).
- **`internal/acdsl/policy.go`** — the free/gated/strict self-management gate is cleanly isolated,
  resolves its policy from the *base-ref* copy (a branch can't loosen its own leash by editing
  `policy.json`), and `CheckPolicy` degrades to free-mode-friendly behavior when no base resolves.
  `cmd/hooks/acdsl-policy-guard.sh` is a correct, fast, fail-open front door for the same rule.
- **`cmd/worktrees/_lib/verdict.sh` / `print_agent_worktrees_status.sh`** — the new
  squash-containment layer (`squash_contained`, plumbing-only `git merge-tree --write-tree`) and
  the per-branch verdict cache are careful pieces of work: the cache key salts in a `"v2"` marker
  specifically so old cached rows (written before the squash layer existed) miss instead of
  serving stale semantics, and cache reads/writes are atomic (`mktemp` + `mv -f`).
- **Audience rename migration** — `audienceNonDeveloper`→`audienceCasual` (see Issues) ships with
  an actual one-time migration in `sync_skills.sh` and dedicated test coverage
  (`test_legacy_non_developer_migrated_to_casual` in `cmd/tests/test_sync_skills_overrides.sh`).
- Full build/vet/test suite is green (see Method).

## Issues

### Critical (Must Fix)

None found.

### Important (Should Fix)

1. **`internal/shell/shell.go:26-67` — `Run` and `RunSync` are near-total code duplication.**
   The two functions are byte-for-byte identical (context setup, `platformArgv`, `HideWindow`,
   `WaitDelay`, logging, error wrapping) except for the timeout constant and the error-message
   prefix string. This is exactly the "parallel variant instead of one source of truth" shape
   RULE-GOLANG-FUNC-003 and ACTION-REVIEW-QUALITY-005 (no unnecessary duplication) warn about: any
   future fix to the run mechanics (e.g. the `WaitDelay` comment's grandchild-process fix) has to
   be remembered and applied twice.
   Fix: factor a shared unexported `run(ctx context.Context, timeout time.Duration, label, dir,
   name string, args ...string) (string, error)` and make `Run`/`RunSync` thin callers passing
   their own timeout/label.

2. **Plan deviation: the persisted audience value was renamed from `"non-developer"` to
   `"casual"`, contradicting the approved plan's explicit decision.**
   `plans/v1-2-1/2026-08-27-smine-profile-settings-wip-ca34568/design/raw.md` D5 says: "Selector:
   `developer` (default), `non-developer` ("casual")... **only the two existing values are ever
   written**" — i.e. the plan explicitly keeps `"non-developer"` as the wire/persisted value and
   uses "casual" only as the UI label. The implementation instead renamed the constant itself
   (`internal/server/presentation.go:14`, `audienceNonDeveloper` → `audienceCasual = "casual"`)
   and writes `"casual"` to `presentation-profile.md`.
   This is a real behavior change from what was approved, but it is not a silent one: the
   implementer added a one-time migration to `cmd/sync/sync_skills.sh` (rewrites an existing
   `audience: non-developer` line to `audience: casual` in place) which runs on every
   `install.sh` invocation — the standard upgrade path — before the new binary starts, and it has
   dedicated test coverage. The `"non-developer"` audience feature did ship in the v1.1.4 base
   commit (`9e64323`, 8 minutes before the `v1.1.4` release tag), so this is a genuine, live
   upgrade path, not a hypothetical one.
   Flag this for explicit confirmation rather than treat it as settled: the plan's Decisions table
   was not amended and no Changelog row was added (RULE-PLAN-049 requires one on any post-approval
   change). If the rename was deliberate (e.g. "casual" reads better as an internal identifier
   too), update D5/D6 and add the Changelog row; if it wasn't deliberate, the safer, plan-conformant
   fix is trivial (keep `"non-developer"` as the constant value, "Casual" stays only the UI label).
   Secondary note: the migration logic re-implements the frontmatter parsing shape (`sed
   's/^audience:.../'`) in a second place (shell) independent of the Go source of truth in
   `presentation.go`'s `loadPresentationProfile` — functionally fine today, but a small
   maintenance coupling if the frontmatter format changes again.
   Files: `internal/server/presentation.go:14,26-27`, `cmd/sync/sync_skills.sh:15-26`,
   `plans/v1-2-1/2026-08-27-smine-profile-settings-wip-ca34568/design/raw.md` (D5/D6).

### Minor (Nice to Have)

1. **`Makefile:81-85`** — the new `build-install` target's help comment is a copy-paste of
   `build`'s (`## build: build all binaries and install` instead of `## build-install: ...`).
   `make help` parses `## <text>` lines verbatim, so this now shows a bogus/duplicate `build` row
   instead of documenting `build-install`.

2. **`internal/repos/registry.go` `Add`/`Remove`** — the read-modify-write against a fresh
   `readRegistrySnapshot` narrows, but does not fully close, the multi-process race it targets:
   two processes racing `Add`/`Remove` within the same read→write window can still silently drop
   one write, since there's no OS-level file lock, only in-process `mu.Lock()`. Low risk for a
   single-user local tool, but worth a one-line comment acknowledging the residual window (or an
   flock-style guard) if concurrent configservers become a real scenario again — this is the exact
   failure mode FACT-REPO-ARCH-016 documents, just with a much smaller window now.

3. **`cmd/hooks/skill-invoke-gate.sh:64-72`** — the "was a context file read via Bash" detector
   only recognizes the literal two-word form `cat <path>` and bails out on any shell metacharacter
   in the command; any other reader (`less`, `python3 -c ...`, `\cat`, a heredoc) evades detection
   entirely. This matches the header's documented "fail-open by design" stance, so it's not a
   defect, just worth knowing it only catches the naive case, not a determined one.

4. **`cmd/worktrees/_lib/verdict.sh` `squash_contained()`** — the comment says "no side effects",
   but `git merge-tree --write-tree` does write a (garbage-collectible, harmless) tree object into
   the ODB. Comment is slightly over-stated; no functional issue.

5. **`routines/smine-nightly/run.sh:136`** — `[[ "$exit_status" -eq 0 ]] && exit_status=$orchestrate_status`
   is a tautology at that point: the enclosing `if` (line 94) already requires `exit_status -eq 0`
   to enter the block, and nothing inside mutates `exit_status` before this line. Harmless, but
   could just be `exit_status=$orchestrate_status` unconditionally — reads as if it's guarding
   against something it isn't.

## Recommendations

- **Fully automated headless merges to `main`** — `routines/smine-nightly/run.sh`'s new Stage 3
  (`orchestrate`) and `cmd/bootstrap/run.sh` both let an LLM-judged run merge its own branch into
  `main` with no human in the loop, gated only by `ROUTINE_ORCHESTRATE`/casual-audience defaults.
  This is a deliberate, plan-approved design point for the "no developer available to review a PR"
  audience (change-nondev-automation C7/C10/C13), and it's backed by mechanically-verified
  postconditions (clean tree + actual `merge-base --is-ancestor` check, never trusting the agent's
  own exit code) — a sound mitigation. Worth keeping visible as a standing risk in ops/monitoring
  regardless, since a bad LLM judgment call here lands directly on `main` for exactly the users
  least equipped to notice.
- Confirm `install.sh` (not some other auto-update path, e.g. a Windows installer flow via
  `installer/windows/smine.iss`) is genuinely the only supported 1.1.4→1.2.1 upgrade route on real
  machines — both the audience migration and the `peek-mcp` v1.2.0→v1.2.2 bump depend on it
  actually running.

## Assessment

**Ready to merge?** Yes.

**Reasoning:** No correctness, security, or data-loss defects found; build/vet/full test suite are
green on a clean checkout. The one real design deviation from the approved plan (audience
value rename) is already backward-compatible via a tested migration wired into the standard
upgrade path — it needs a plan Changelog entry and a decision on whether to keep it, not a code
fix. The `shell.go` duplication is worth cleaning up before it drifts further, but is not
blocking.
