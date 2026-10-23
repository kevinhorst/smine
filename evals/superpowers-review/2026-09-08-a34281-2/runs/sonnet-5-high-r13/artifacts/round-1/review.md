# Review: smine 1.1.4 → 1.2.1 (`b4981b88` → `a342811b`)

Scope: 178 files, +7314/-749, excluding `sessions/`, `evals/`, `proposals/`, `plans/`. This
bundles ~10 plan folders under `plans/v1-2-1/` (acdsl policy modes, repo/routine registry
fixes, bootstrap wrapper, profile/i18n settings, peek multi-profile, repo-cache reload,
session-context report, railroad-review observability). Reviewed in passes: Go core
(registry/peek/routines/acdsl), shell hooks and routine wrappers, server handlers/templates,
skills/docs, in that order. Verified independently (not just read): `go build ./...`,
`go vet ./...`, the full `go test ./...`, and `make audit`, all run against a **clean
worktree checked out at the head commit** (see Critical-adjacent note below on why not the
review checkout itself) — all green.

## Strengths

- **`internal/repos/registry.go`** correctly fixes the resurrection-after-delete bug
  (FACT-REPO-ARCH-016): `Add`/`Remove` now re-read the file fresh under the lock
  (`readRegistrySnapshot`) instead of mutating a cached in-memory slice, so a second writer's
  last-writer-wins can no longer resurrect a deleted entry. `refreshIfChanged` correctly
  distinguishes "never loaded, file absent" from "was loaded, file now gone" via the
  mtime/size pair, and `save()` re-stats after writing so a self-write is never re-detected as
  an external change.
- **`internal/peek/peek.go` / `internal/server/repos.go`** cleanly fixes the cwd-collision
  attribution bug (FACT-REPO-ARCH-017): `SessionIndex` now returns both `ByCwd` and `ByBranch`
  maps, and `worktreeSessions` prefers branch identity with cwd as the detached-HEAD fallback —
  exactly the fix the fact describes, well-isolated and easy to follow.
- **`internal/server/statuscache.go`** implements fingerprint-based (not TTL-based) cache
  invalidation as the architecture doctrine requires, and `DropBranches` deliberately keeps the
  stale pre-removal fingerprint so the next plain load is forced to re-scan — a subtle
  correctness point that's called out and gotten right.
- **`internal/acdsl/policy.go`/`dist.go`**: the base-ref-is-authoritative design (a working-tree
  edit to `acdsl/policy.json` cannot weaken the mode, because `CheckPolicy` re-reads the policy
  from the merge-base) is a sound answer to the obvious "just edit policy.json to disable
  gating" attack. The `dist_mode` vs `mode` split (dev repo free, distributed default gated) is
  a clean, minimal way to have one field serve two audiences without another parallel struct.
- `routines/_lib/worktree.sh`: the added `HEAD` check after `routine_chain_sync_main`
  (verifying the branch didn't drift onto a stray `merge/*` work branch) is a real, specific bug
  fix with a clear failure-mode comment.
- Test coverage is broad and matches the new logic (registry read-modify-write races,
  fingerprint cache, policy modes/overlay/surface diffs, dist rewrite, plist schedule
  classification). Everything passes on a clean checkout.

## Issues

### Critical (Must Fix)

None found in the diff itself. One process note that looks like a Critical at first glance but
isn't a code defect:

- The review checkout (`arm-wt`) currently has **uncommitted local modifications** to
  `acdsl/policy.json`, `acdsl/registry.json`, and `acdsl/rules.acdsl` (not part of this diff —
  `git status` shows them `M` against the head commit). Running the full test suite in that
  checkout fails `TestDistShipsReachCoveredSliceWithBinaries` (`internal/acdsl/dist_test.go`)
  because the mutated `registry.json` no longer has `gofmt` in the `go run …` form the test
  expects to be rewritten. I checked out `a342811b` cleanly into a scratch worktree and
  confirmed `go build`, `go vet`, `go test ./...`, and `make audit` are **all green** there —
  this is checkout contamination, not a bug in the reviewed commit. Flagging it so it isn't
  mistaken for a real regression, and so whoever owns this checkout knows those three files
  need `git checkout -- acdsl/` before the next real test run in it.

### Important (Should Fix)

1. **Global "ask" permission for ACDSL rule-file edits was removed, and nothing fully replaces
   it** — `settings/claude_code/settings.json:105-110`. Before this diff,
   `Edit(**/*.acdsl)` / `Edit(acdsl/**)` / `Write(**/*.acdsl)` / `Write(acdsl/**)` were in the
   global `ask` list — this file deploys to `~/.claude/settings.json`, i.e. it's the interactive
   permission gate for *every* repo on the machine, not just this one. The diff deletes those
   four entries with no replacement in `ask` or `deny`. The new `acdsl-policy-guard.sh`
   (`cmd/hooks/acdsl-policy-guard.sh`) only denies interactively when the *target repo's own*
   `acdsl/policy.json` has `mode: strict` (blocks all `*.acdsl`/`acdsl/*` edits) or `mode: gated`
   (blocks only the five self-management-surface files, e.g. `policy.json`, `registry.json` —
   **not** general rule-file edits). Net effect:
   - A repo with **no `acdsl/policy.json`** (the default before adoption — `LoadPolicy` falls
     back to `mode: free`) now gets **zero** interactive protection on rule-file edits, where it
     previously always got an "ask" prompt.
   - A **gated**-mode repo (the shipped `dist_mode` default for consumer repos) gets no
     interactive protection for ordinary rule-file edits either — only the five surface files are
     hook-denied; everything else is enforced at `acdsl check` / CI time, i.e. *after* the file is
     already written (and possibly committed) rather than *before*.
   - This repo's own `acdsl/policy.json` ships `mode: "free"` (correct for a repo that has to
     iterate on its own rules), so for the repo actually authoring the rule system there is now
     no gate of any kind (no ask, no hook, `CheckPolicy` returns nothing in free mode) on editing
     `*.acdsl` files — the exact surface this PR is otherwise hardening.
   This may be entirely intentional (trade prevent-time friction for review/CI-time detection
   now that `acdsl check` covers it), but it's a real behavior change on a security-relevant
   surface with no explicit sign-off visible in the diff, and it silently drops coverage for the
   pre-adoption/free-mode case. Worth an explicit decision: at minimum, consider keeping a
   baseline `ask` (or `deny`) for repos where `acdsl/policy.json` is absent or `mode: free` is
   the *un-declared* default, so "haven't adopted policy yet" doesn't silently mean "no gate at
   all" (it used to mean the opposite).

2. **`internal/acdsl/git.go:57` `GitChangedFiles` splits git output with `strings.Fields`,
   which breaks on filenames containing spaces.** `git diff --name-only` and
   `git ls-files --others` both emit one path per line; `strings.Fields(diffed + "\n" +
   untracked)` splits on *any* whitespace, so a changed/untracked path with an embedded space
   (a screenshot dropped in with its OS-default name, an asset under `design/ui/`, etc.) is
   silently split into two bogus, non-existent paths. Downstream, `ruleDelta` (`policy.go:237`)
   uses this list to decide which files to diff for rule-marker changes — a rule declaration
   living in a space-containing filename would be silently skipped from `CheckPolicy`'s
   diagnostics (a security-relevant miss, since this is the policy-gate's own file-change
   detection). Fix: split on `"\n"` and trim/filter empty lines instead of `strings.Fields`.

3. **Speculative, unwired "started" run marker** — `internal/routines/routines.go:123-166`. The
   diff adds `RunResult.Subtype`/`RunResult.Result` fields, `RunResult.IsRunning()` (`Subtype ==
   "started"`), and a filter in `History()` that drops every "started" row except the newest
   (to represent a currently-live run). I traced every writer of `results.jsonl` in the diff and
   pre-existing code (`routines/_templates/run.sh`, `routines/skill-eval/run.sh`,
   `routines/smine-nightly/run.sh`, `cmd/bootstrap/run.sh`) — none of them ever emit
   `subtype: "started"`; `subtype` there is always whatever the `claude -p --output-format
   json` envelope reports (`success`, `error_max_turns`, …), which is never `"started"`. The
   only place `"started"` appears anywhere in the diff besides the method itself is the unit
   test's synthetic fixture (`routines_test.go`). This is dead code today — added complexity
   (new struct fields, new method, new filtering pass on every `History()` call) with no
   producer, i.e. a concept that doesn't trace to an exercised requirement. Either wire up an
   actual "started" marker write (e.g. in the wrapper templates, before `claude -p` is invoked)
   or drop `IsRunning()`/the filtering loop until there's a writer — as it stands it can't
   affect behavior either way, but it's a real "reads as if implemented, isn't" trap for the
   next person who touches `History()`.

### Minor (Nice to Have)

1. **Misplaced doc comment** — `internal/server/repos.go:173-199`. The pre-existing comment for
   `rootCause` (`// rootCause unwraps to the innermost error…`) now sits directly above the
   *new* `worktreeSessions` function (which has its own, correct doc comment right below it),
   while `rootCause` itself, a few lines down, is left with no comment at all. Looks like the
   new function was inserted between an existing doc comment and the function it describes.
   Purely a documentation glitch (confirmed via `git diff`), harmless functionally, but worth a
   one-line fix so `rootCause` isn't left undocumented and `worktreeSessions` doesn't carry two
   unrelated doc comments.
2. **`internal/config/installenv.go:11` `installEnvPath = "install.env"` is read via a relative
   path** (`os.ReadFile("install.env")`), so `ExpandInstallMarkers` silently falls back to
   `DefaultPeekControlPort` if the process's cwd isn't the repo root at call time. In the
   shipped deployment (launchd `WorkingDirectory` = repo root) this is fine, but it's a quiet
   failure mode with no logging if someone runs the config-sync path from elsewhere (e.g. a
   future CLI subcommand, a test, a different OS service manager). Not urgent given the current
   single call site is deployment-guaranteed, but a repo-root-relative path resolved once at
   startup (rather than a bare relative literal used ad hoc) would remove the fragility.
3. German i18n catalog (`internal/server/i18n.go`) is a large, purely additive data table —
   spot-checked the `%d`/`%s` format strings for verb-order/type parity between English keys and
   German values (`"%d of %d accepted"`, `"%+d vs batch %d"`, `"Batch %d-%d"`, etc.); all match.
   No functional issue, just noting it was checked rather than skipped as "just strings."

## Plan Alignment

Spot-checked against the facts the plan/context docs assert this release should fix:

- FACT-REPO-ARCH-016 (registry last-writer-wins) — fixed, verified in code (see Strengths).
- FACT-REPO-ARCH-017 (peek cwd/branch attribution) — fixed, verified in code (see Strengths).
- FACT-REPO-ARCH-012 (bootout-kills-live-run guard) — `internal/routines/launchctl_darwin.go`
  itself is untouched in this diff (only its test file changed); this fact's fix appears to
  predate this range, or lives in a call site I didn't chase down given the diff's size. Not a
  regression either way — just noting I didn't independently re-verify it here.
- The acdsl policy-modes plan (`plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/`) is
  implemented with a sound base-ref-authoritative design; the one open point is the
  settings.json permission gate interaction flagged as Important #1 above — worth confirming
  against that plan's own decisions record, since a "sign off on removing a guard" moment isn't
  visible in the diff itself.

No deviations found that look like accidental scope changes; the diff is large but each piece
(registry, peek, statuscache, routines, acdsl policy, bootstrap wrapper, profile/i18n settings)
maps to a distinct plan folder under `plans/v1-2-1/`.

## Recommendations

- Confirm intent on Important #1 (settings.json ask-permission removal) explicitly — it's the
  kind of change that should have a decision row in the acdsl-policy-modes plan, not just fall
  out of "the hook replaces it."
- Fix the `strings.Fields` → line-split bug in `GitChangedFiles` (Important #2); it's a one-line
  change and the current behavior is a silent miss in a security-relevant path.
- Either wire a real "started" writer or delete the dead filtering logic in `History()`
  (Important #3) before it's mistaken for working "live run" support.
- Before the next test run in this specific worktree (`.../runs/sonnet-5-high-r13/artifacts/arm-wt`),
  `git checkout -- acdsl/policy.json acdsl/registry.json acdsl/rules.acdsl` to drop the local
  contamination that currently breaks `TestDistShipsReachCoveredSliceWithBinaries` there.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** No correctness bugs block the release — every core mechanism I traced (registry
race fix, peek branch attribution, fingerprint cache, policy base-ref authority, routine
scheduling/default-disabled, bootstrap wrapper) is sound and independently verified green
(build/vet/tests/audit on a clean checkout of the head commit). The two Important findings that
matter most are the settings.json permission-gate removal (a real, undocumented reduction of a
safety net on the exact surface this release is hardening) and the `GitChangedFiles` whitespace
bug (a genuine, if narrow, miss in the policy gate's own change-detection). Neither is
data-loss-critical, but both sit on the security-relevant path and deserve a decision/fix before
this ships, not after.
