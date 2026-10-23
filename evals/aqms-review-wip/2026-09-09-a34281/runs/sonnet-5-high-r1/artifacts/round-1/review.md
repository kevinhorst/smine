/Users/devuser/GolandProjects/claude-configs/.claude/worktrees/skillroutine-eval-aqms-review-3f52bb/evals/aqms-review-wip/2026-09-09-a34281/runs/sonnet-5-high-r1/artifacts/aqms-run

## Setup

- Reviewed range: `b4981b88` (v1.1.4) → `a342811b` (v1.2.1), 12 commits, as instructed —
  `git diff b4981b88...a342811b -- :(exclude)sessions/ :(exclude)evals/ :(exclude)proposals/ :(exclude)plans/`.
- Snapshot: `$RUN/diff.patch` — **178 files, +7314/-749**. This is a full-release diff spanning
  unrelated concerns (config-server UI, ACDSL policy engine, routine wrappers, hook scripts,
  ~25 skill doc/changelog bumps). Per the skill's own guidance this is too large to review as
  one unit at full depth; I did not split it (out of scope for this invocation) but I am
  stating the size now and describing exactly what got full-depth attention vs. a skim below,
  rather than silently under-covering it.
- The working tree carries harness-provided uncommitted changes (golden-input plans, a frozen
  context index, `acdsl/*.json`, `CLAUDE.md`, `context/*`) per the dispatch note — treated as
  review context, not part of the change. This mattered concretely (see "Ruled out" below).
- Specification: `plans/v1-2-1/*` (9 plan dirs). Used as design intent for the config-server /
  routine / ACDSL changes; skill-doc-only commits (smine-* SKILL.md/changelog bumps) have no
  corresponding plan and were read only for scope creep.
- Base for `git diff`: the dispatch-given `b4981b88d52deaa69d6ed1d60eb51b57ead47ba0`, explicitly.
- `git status` before and after: identical (confirmed via `diff status-before.txt
  status-after-check2.txt`). Two throwaway git worktrees were added/removed during verification
  (`$RUN/base-check`, `$RUN/head-check`) and two temporary `_test.go` files were added, run, and
  deleted from the reviewed checkout during verification (evidence below) — the checkout was
  restored in place before finishing.

## Risk map

| Tier | Files (representative) |
|---|---|
| **High** | `internal/acdsl/policy.go` (new, self-management enforcement), `cmd/hooks/acdsl-policy-guard.sh` (new), `cmd/hooks/skill-invoke-gate.sh` (new), `internal/repos/registry.go` (persisted registry, concurrency), `internal/sessions/sessions.go` (new delete/archive of persisted analysis data), `internal/server/bootstrap.go` (new, spawns a detached multi-hour subprocess from an HTTP handler), `internal/server/profilesettings.go`, `internal/routines/{routines,plist,bootstrap_darwin}.go`, `routines/smine-nightly/run.sh` (orchestrate stage: merges/prunes branches) |
| **Medium** | `internal/server/{repos,statuscache,presentation,i18n,overview,server,welcome}.go`, `cmd/worktrees/_lib/verdict.sh` (safe-to-remove verdict algorithm, +90 lines), `cmd/worktrees/print_agent_worktrees_status.sh`, `cmd/bootstrap/run.sh` (new), `cmd/sync/sync_skills.sh` (new), `install.sh`, `internal/config/installenv.go` (new) |
| **Low** | `internal/server/templates/*.html` (skimmed for scope creep only), ~25 `skills/*/SKILL.md` + `changelog.json` doc/version bumps, `docs/*.md`, `README.md`, `*_test.go` files |

Full-depth review: everything in **High**, plus the specific **Medium** items named above.
Everything else in **Medium** and all of **Low** got a stat/skim pass for scope creep, not a
line-by-line read — see the Funnel note at the end.

## Findings

### MAJ-1 · major · security · rank 1 · route: human
`cmd/hooks/skill-invoke-gate.sh:62-74` — the post-invocation "no direct reads of injected
context files" block is trivially bypassed for the `Bash` tool by either (a) quoting the path,
or (b) using a relative path instead of an absolute one. The block only recognizes the exact
shape `cat <absolute-unquoted-path>`: any command containing `"`, `'`, `` ` ``, `$`, `(`, `)`,
`;`, `|`, `&`, `<`, `>` is skipped entirely (line 66's pattern list is a no-op arm), and even a
clean `cat <relative-path>` never matches `"$ctx_dir"/*` (line 76) because `$ctx_dir` is always
absolute. Both `cat "docs/foo.md"` (quoted) and plain `cat docs/foo.md` (relative — arguably the
*more* natural way to invoke `cat` when `cwd` is already the repo root) pass the gate silently.
The `Read` tool path (line 63) is unaffected — this is specific to the `Bash` escape hatch this
hook itself introduces.

- verified (repro, both directions) — ran the real hook script against three crafted
  `PreToolUse` payloads (a session with `REQUIRE-SKILL:some-skill` in the first user message,
  already "invoked" via a `Skill` tool_use in the transcript):
  - unquoted absolute `cat $ctx/docs/secret.md` → denied (`$RUN/MAJ-1-repro`, "Test A")
  - quoted absolute `cat "$ctx/docs/secret.md"` → **no output = allowed** (`$RUN/MAJ-1-repro`, "Test B")
  - relative `cat docs/secret.md` → **no output = allowed** (`$RUN/MAJ-1-repro`, "Test C")
- artifact: `$RUN/MAJ-1-repro/` (fixtures + transcript used for all three runs; rerun with
  `bash cmd/hooks/skill-invoke-gate.sh < <(jq ...)` against the fixtures there to reproduce)
- fix: for the `Bash` case, don't special-case `cat`'s exact argv shape — either resolve the
  command's arguments against `$ctx_dir` after normalizing quotes/relative paths (e.g. `eval
  set -- "$command"` inside a subshell with `cd "$cwd"`, or shell out to `python3 -c` to argv-parse
  safely), or drop the metacharacter allowlist and instead grep the raw command string for
  `$ctx_dir` (a path substring match) as a conservative superset — false positives are cheap
  here since the hook is explicitly documented as advisory ("fail-open by design").

### MAJ-2 · major · spec-compliance · rank 2 · route: human
`cmd/hooks/acdsl-policy-guard.sh:43` — the PreToolUse guard denies **every** write to
`acdsl/registry.local.json` whenever policy mode is `gated` or `strict`, unconditionally. This
contradicts both the shipped documentation and the authoritative Go check the hook is supposed
to be fast feedback for:
- `acdsl/README.md` ("Modes"): gated mode's frozen self-management surface is explicitly listed
  as `policy.json`, `policy.schema.json`, `registry.json`, `evalgen.json` — `registry.local.json`
  is *not* in that list, and the same doc says overlay entries in it are "policy-gated: only
  entries listed in the policy's `local_overrides` are accepted" (i.e. sanctioned entries are a
  supported, editable workflow, in both gated and strict mode).
- `internal/acdsl/policy.go`'s `policySurface` var (the Go source of truth for "the frozen
  surface") likewise omits `registry.local.json`; a *separate* function, `overlayViolations`,
  is what judges it — by checking each overlay entry name against `policy.LocalOverrides`, not
  by freezing the file outright.
- This diff also **removed** the client-side `"ask"` permission entries for
  `Edit(acdsl/**)`/`Write(acdsl/**)` from `settings/claude_code/settings.json` (see diff of that
  file), i.e. the hook is now the *only* gate standing between an agent and an edit to this
  file — there is no human-confirmation fallback left for a legitimate sanctioned-override edit.
  Net effect: adding a sanctioned local verifier override via Claude Code's Write/Edit tool is
  now impossible in any gated/strict repo, even though the feature exists specifically to
  support that workflow.

- verified (test, both directions checked) — added a temporary test directly against the
  authoritative Go function, ran it, then deleted it (confirmed `git status` unchanged):
  `overlayViolations(root, Policy{Mode: PolicyModeGated, LocalOverrides: []string{"my-verifier"}})`
  on a `registry.local.json` containing exactly that sanctioned entry returns **zero**
  diagnostics — i.e. the authoritative check accepts the edit.
  Meanwhile the actual shell hook, run against the identical scenario (`mode: gated`,
  `local_overrides: ["my-verifier"]`, writing `acdsl/registry.local.json`), denies it outright.
- artifact: `$RUN/MAJ-2-repro/` (hook repro: fixtures + command, denies) and
  `$RUN/MAJ-2-authoritative-check-output.txt` (Go check on the same scenario: 0 diagnostics)
- fix: drop `acdsl/registry.local.json` from the hook's frozen-surface `case` arm (line 43);
  either let it fall through to `acdsl check`'s authoritative judgment (per the hook's own
  stated design for gated rule edits), or replicate `overlayViolations`' sanctioned-name check
  inline in the hook (it only needs `jq` against `policy.json`'s `local_overrides` array and the
  new content, both readable at hook time).

### MAJ-3 · major · general · rank 3 · route: human
`internal/server/sessions.go:182,191,201` — `handleSessionsScopeArchive`,
`handleSessionsScopeDelete`, and `handleSessionsScopeUnarchive` all set the `HX-Redirect`
response header **unconditionally**, before checking whether the underlying operation
(`ArchiveScope`/`DeleteScope`/`UnarchiveScope`) returned an error. htmx treats `HX-Redirect` as
an instruction to do a full client-side navigation to that URL and ignores the response
body/target entirely — so the `_op_result.html` error card these handlers render server-side
(`{{if .Error}}...Failed...{{end}}`, wired via `hx-target="#op-result"` in
`sessions_archive.html`) is never shown to a real htmx client on the failure path for any of the
three operations, including **Delete**, the irreversible one. A user who tries to delete a
folder that (for whatever reason) fails validation is silently redirected back to
`/sessions/archive` with no indication anything went wrong — this looks identical to a
successful delete from the UI. This is a genuine coverage gap in the existing test, not just the
handler: the "archive-unarchive-delete-flow" subtest in `sessions_test.go` asserts
`response.Body.String()` contains "is not archived" on the failure case, which is true and gives
false confidence — it never asserts on the `HX-Redirect` header, so it can't see that a real
browser would never render that body.

- verified (test, both directions checked) — added a temporary test asserting `HX-Redirect` is
  empty on a failing delete (`DeleteScope` on a name that was never archived), ran it (fails on
  current tree with the exact header value `/sessions/archive`), then applied the one-line fix
  (`else { w.Header().Set("HX-Redirect", ...) }`), reran both the new test and the full
  pre-existing `TestSessionsPages` suite (all pass, including the pre-existing
  archive/unarchive/delete subtest), then reverted the source edit and deleted the temporary
  test (confirmed `git status` unchanged afterward).
- artifact: `$RUN/MAJ-4-repro-fails-on-current-tree.txt` (fails: `Should be empty, but was
  /sessions/archive`) and `$RUN/MAJ-4-repro-passes-with-fix.txt` (passes with the fix, plus the
  full `TestSessionsPages` suite green)
- fix: in all three handlers, set `HX-Redirect` only in the success branch (mirrors how
  `internal/server/repos.go:509` conditionally sets `HX-Trigger` only `if opErr != nil ||
  removalNeedsRefresh(...)` — this codebase already has the right pattern elsewhere, these three
  handlers just don't follow it).

### Ruled out (not a finding — noted for transparency)
`go test ./...` run inside the actual review checkout (`arm-wt`) initially showed
`FAIL github.com/kevinhorst/smine/internal/acdsl` — `TestDistShipsReachCoveredSliceWithBinaries`
failed with `stat .../bin/verifiers/gofmt: no such file or directory`. I did not report this as
a code defect: the test reads real on-disk `acdsl/registry.json`/`acdsl/rules.acdsl` via
`repoRoot = "../.."`, and those exact files are among the harness's uncommitted working-tree
overlay (`git status` shows them modified) that the dispatch note says is review context, not
part of the change. I confirmed this by adding two disposable git worktrees: one detached at the
base commit (test passes), one detached at the exact reviewed HEAD `a342811b` with a **clean**
tree, no overlay (test passes, and the full suite is green — `$RUN/gotest-clean-head.txt`,
exit 0). The failure only reproduces with the harness's uncommitted files layered on top, so it
is an artifact of the review environment, not of the diff. Both throwaway worktrees were removed
afterward.

## Build & tests

- `go build ./...` — clean, no output.
- `go vet ./...` — clean (`$RUN/govet.txt`).
- `go test ./...` on a **clean** worktree checked out at the exact reviewed commit (`a342811b`,
  no working-tree overlay) — **all green**, exit 0 (`$RUN/gotest-clean-head.txt`).
- `go test ./...` inside the actual `arm-wt` checkout (i.e. with the harness's uncommitted
  overlay applied) — one failure, explained above under "Ruled out" (`$RUN/gotest.txt`).
- Did not run `make audit`/`make lint` — per `AGENTS.md` in this checkout, `acdsl project
  -strip` must run first and it rewrites governed files' projection blocks; running it would
  have altered working-tree state beyond what I could cleanly revert within this pass, and it
  wasn't necessary to answer "does the reviewed diff build and pass its own test suite."

## Areas not run down further (recorded, not "unverified" line items)

- `cmd/worktrees/_lib/verdict.sh` (+90 lines: the new `squash_contained`/layer-4 detection for
  the branch-removal safety verdict) implements a real git-history-transfer detection algorithm
  (3-way merge-tree containment check, cross-referenced against per-commit patch-id layers). I
  read it for obvious defects (none found — the merge-tree exit-code handling, the `&&`/`set -e`
  interaction in the neighboring `run.sh`, and the `hash_explained_anywhere` array scan all check
  out) but did not construct an exhaustive git-history fixture to exercise every branch of the
  4-layer verdict logic; that would need its own dedicated pass with real multi-branch git
  fixtures (squash-merge, reworded cherry-pick, partial transfer) rather than code reading.
- `routines/smine-nightly/run.sh`'s new Stage 3 orchestrate block (merge/prune of the run
  branch into `main`, +53 lines) — read for `set -e` correctness (the `[[ cond ]] &&
  exit_status=...` idiom at the end is safe; verified empirically that `false && x; echo after`
  does not abort a `set -e` script) and for the postcondition checks (dirty-tree / ancestor
  check). No defect found, but this stage's failure modes only manifest against a live git
  remote/branch topology and weren't exercised end-to-end.
- The ~25 `skills/*/SKILL.md` + `changelog.json` version-bump commits and template-only HTML
  diffs were skimmed for scope creep, not read line-by-line against `context/rules/plan.md`'s
  formatting rules.

## Funnel

Candidates examined (general + spec-compliance + security lanes; security lane triggered per
step 1 by the two new PreToolUse hook scripts) / verified / partial / unverified / refuted:
**~8 / 3 / 0 / 0 / ~5** (the ~5 not carried forward were things that looked suspicious on first
read — the registry read-modify-write pattern, the `ScannedAt`-on-error display, the bootstrap
run's in-memory-only `running` flag, the `git init` side effect in `handleReposAdd`, the
`acdsl-policy-guard.sh` fail path under `jq` failure — but resolved as correct or immaterial on
closer reading/testing and are not listed as findings).

## What blocks pushing

Nothing here rises to **blocker** by the stated bar (production data loss, an irreversible
migration defect, or a security hole that grants unintended access) — build and the full test
suite are green on the actual committed diff. MAJ-1 and MAJ-2 are real gaps in this release's new
governance/enforcement tooling, though: MAJ-1 means the "context arrives only via skill
injection" guarantee this eval infrastructure depends on doesn't currently hold for realistic
Bash usage, and MAJ-2 means gated/strict ACDSL repos currently have no working path to add a
sanctioned local verifier override through Claude Code at all. I'd fix both before relying on
either mechanism for anything that matters, but neither one touches user data or an already-
released migration.

## What needs a recorded review once a PR exists

- MAJ-1 and MAJ-2 fixed and re-verified with the same repro fixtures (`$RUN/MAJ-1-repro`,
  `$RUN/MAJ-2-repro`).
- MAJ-3's fix, extended to `handleSessionsScopeArchive`/`handleSessionsScopeUnarchive` too (only
  `Delete` was repro'd end-to-end above, but `Archive`/`Unarchive` share the identical
  unconditional-header pattern — same fix, same file).
- Recurring class worth a permanent check: this is the second time in this diff that a
  fast-feedback enforcement layer (a shell hook) disagrees with its own stated authoritative
  counterpart (MAJ-1 vs. the "context is injected, not read directly" contract; MAJ-2 vs.
  `overlayViolations`/README). A cheap guard for the class: a fixture-driven test that runs both
  layers (the hook script, and the Go `CheckPolicy`/equivalent) against the same handful of
  scenarios and asserts they agree — this would have caught MAJ-2 immediately, and a Bash-tool
  variant of the same fixture set (plain, quoted, relative, absolute) would have caught MAJ-1.
- The `verdict.sh` layer-4 squash-containment logic and the `smine-orchestrate` Stage 3 flow
  (noted above) are unverified-by-me, not unverified-in-principle — they'd benefit from a
  dedicated git-fixture-based test pass before being trusted on a real repo's branch cleanup.
