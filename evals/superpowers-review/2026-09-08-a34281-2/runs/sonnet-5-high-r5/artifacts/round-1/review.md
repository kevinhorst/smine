# Review: smine v1.1.4 → v1.2.1 (b4981b88..a342811b)

Scope note: 179 files, +7314/-749, spanning 9 plan directories under `plans/v1-2-1/`. This is
too large for one uniform-depth pass. I reviewed in passes: full read of the highest-risk plans
(repo cache/registry concurrency, peek multi-profile port/identity, bootstrap headless wrapper,
casual lockout, sessions archive/delete) with matching diff verification and a clean-worktree
build/test run; the remaining plans (plan-format-viewer-fixes, smine-profile-settings,
language-style-setting, railroad-review-observability, session-context-report,
acdsl-policy-modes) got a lighter pass — diff-stat mapping to their Scope/Changes tables and
targeted greps, not line-by-line diff reading. Flagged below wherever that matters.

One process note: the review checkout (`arm-wt`) had uncommitted local modifications (dirty
`acdsl/registry.json`, `acdsl/rules.acdsl`, plus several untracked context files) already present
at session start, unrelated to the reviewed commit range — these came from something other than
this diff (context-pack provisioning, most likely). Running `make audit` directly in that
checkout fails immediately (`bin/verifiers/errorf-prefix: no such file or directory`) because the
dirty `registry.json` points at prebuilt-binary paths that don't match the real build output
names. That failure is **not** a defect in the reviewed diff — the committed `a342811` registry.json
still uses `go run ./cmd/acdsl/verifiers/<pkg>`, and `make audit` passes cleanly (24 rules OK, full
`go test ./...` green) in a fresh `git worktree add` of `a342811`. Worth a note to whoever owns
that arm-wt, but out of scope for this review.

## Strengths

- **Repo registry/cache rework (`internal/repos/registry.go`, `internal/server/statuscache.go`,
  `internal/server/repos.go`) matches its plan almost verbatim** — the stat-guarded read-through
  registry (`refreshIfChanged`, `readRegistrySnapshot`, read-modify-write `Add`/`Remove`) and the
  fingerprint-gated status cache are implemented exactly as designed in
  `plans/v1-2-1/2026-08-30-repo-cache-reload-wip-46c8d9e/design/change-repo-cache.md`, including the
  documented benign race in `refreshIfChanged` and the "keep last good state on parse error"
  degrade path (D2). All the plan's listed new tests exist
  (`TestRegistryRefreshOnExternalEdit`, `TestRegistryAddPreservesExternalEdit`, `TestFingerprint`,
  `TestStatusCacheDropBranches`, `TestWorktreeStatusFragmentServesCache`, etc.) and pass.
- **Peek multi-profile identity check** (`cmd/configserver/main.go` `startPeek`/`peekIdentity`) is a
  clean, minimal implementation of the plan's D2/D7 decision: reuse only on `/healthz` identity
  match, loud error + degraded (not silently-wrong) UI on mismatch or an unidentifiable holder on
  the port. Matches the plan's exemplar closely.
- **`cmd/bootstrap/run.sh`** (new, 174 lines) is solid shell: `set -uo pipefail` (no `-e`, deliberately,
  since it must not abort on a stage failure — the orchestrate judge is the terminal arbiter),
  bash-3.2-safe empty-array expansion (`${flags[@]+"${flags[@]}"}`), BSD sysexits, no `eval`, no
  string-interpolation-into-a-shell risk from user-supplied `BOOTSTRAP_EXTRA_PROMPT` (it travels as
  an env var into `os/exec`, never re-parsed by a shell). It correctly implements a genuine
  architecture pivot: the original plan (`change-bootstrap-profile.md`, Phase A) specified
  args-in-the-prompt-string with no env vars; a second plan
  (`change-headless-bootstrap.md`) explicitly supersedes that constraint ("dissolves here: the
  wrapper translates env→flags exactly like the nightly") and the code matches the *second* plan.
  This is a well-documented deviation via a follow-up plan, not a silent one — flagging only so it's
  confirmed intentional (see Important #2 below for the one loose end in the old plan's Changelog).
- **Casual skill-edit lockout (DR3)** is threaded through all three planned enforcement points —
  extraction (`smine-skills` SKILL.md "Casual lockout" section), apply (`smine-apply`'s hard
  `casual-locked` disposition gate), and the orchestrate judge (`smine-orchestrate`'s "casual
  lockout" invariant check) — consistently worded and keyed on the same `origin: user` frontmatter
  marker across all three.
- **Casual/non-developer rename sweep is complete and correct.** Grepped every surviving
  `non-developer` string outside `plans/`/`sessions/`/`evals/`/`proposals/`: the only hits are the
  migration logic itself (`cmd/sync/sync_skills.sh` detecting the old value to rewrite it) and its
  test fixture (`cmd/tests/test_sync_skills_overrides.sh`, which deliberately writes the old value
  to prove migration) — both correct, expected uses. No leftover `audienceNonDeveloper` Go
  identifier.
- **`internal/sessions.go` archive/unarchive/delete** (`ArchiveScope`/`UnarchiveScope`/`DeleteScope`)
  has a properly defensive `validateScopeName` — rejects empty, `.`, `..`, the `archived` container
  name itself, and any path separator, and compares via `name != filepath.Base(name)` rather than a
  blocklist. `DeleteScope` only ever operates inside `sessions/archived/`, never a live folder —
  matches the plan's D12 "archive-then-delete is a two-step guard; a live folder can never be
  deleted directly."
- Build, vet, and the full `make audit` (acdsl gates + `go test ./...`, no `-race`) are green on a
  clean checkout of the reviewed head commit.

## Issues

### Critical (Must Fix)

None found.

### Important (Should Fix)

1. **`internal/server/sessions.go:177-201` — archive/delete/unarchive handlers discard their own
   error message via an unconditional `HX-Redirect`.**
   `handleSessionsScopeArchive`, `handleSessionsScopeDelete`, and `handleSessionsScopeUnarchive` all
   follow the same shape:
   ```go
   result := opResult{Page: pageSessions, Subject: "archive " + r.PathValue("scope")}
   if err := s.sessions.ArchiveScope(r.PathValue("scope")); err != nil {
       result.Error = err.Error()
   }
   w.Header().Set("HX-Redirect", "/sessions")
   s.renderFragment(w, tmplOpResult, result)
   ```
   `HX-Redirect` is set **regardless of whether `err` is nil**. Per htmx's own contract, that header
   makes the browser do a full `location.href` navigation and *discard the response body* — the
   `hx-target="#op-result"` on the calling forms (`sessions_index.html:20`, `sessions_archive.html:19,24`)
   never gets a chance to render, because HX-Redirect wins unconditionally. So on any of the real
   failure paths these three ops can hit (archiving an already-archived name, deleting a name that
   isn't actually archived — e.g. a second browser tab, or clicking Delete on a since-unarchived
   row — or unarchiving into a name that was re-registered/re-mined live in the meantime), the user
   is silently redirected as if the operation succeeded, with zero indication anything failed and
   the underlying state unchanged. This is provable from the test itself
   (`internal/server/sessions_test.go:222-225`): it asserts the error text `"is not archived"` shows
   up in the raw HTTP response body, but that assertion only holds at the `httptest` layer — in a
   real htmx client the body is never rendered because `HX-Redirect` preempts it. All three
   instances share the same root cause; fix once, not three ways: set `HX-Redirect` only when
   `err == nil` (mirrors the existing `runRepoOp` pattern in `internal/server/repos.go`, which only
   sets its trigger header conditionally and otherwise lets the fragment render the error in place).

2. **Loose end in the superseded plan's own paper trail.** `plans/v1-2-1/2026-08-27-bootstrap-profile-improvements-wip-397f906/design/change-bootstrap-profile.md` Phase A (bootstrap-form/A1-A5) describes a `bootstrapSkillArgs`-based, prompt-string-only mechanism that was never built that way — it was superseded before implementation by
   `change-headless-bootstrap.md`'s wrapper-orchestrated design, which is what actually shipped
   (`internal/server/bootstrap.go`'s `bootstrapEnv` + `cmd/bootstrap/run.sh`). That supersession is
   explicitly stated in the second plan's Context section, so this isn't a silent deviation — but
   the first plan's own Changelog and Changes sections were never updated to record that Phase A was
   dropped/replaced, and its Verification checklist and Contracts & sweeps still describe the old
   mechanism as if it shipped. Per this repo's own `RULE-PLAN-049` ("any post-approval plan edit
   appends a row... history lives only here"), the first plan should carry a Changelog row pointing
   at the second plan and marking Phase A superseded — right now a reader of
   `change-bootstrap-profile.md` alone would believe the shipped mechanism is args-in-prompt-string,
   which is wrong. Low cost to fix (one Changelog line), but worth doing since these plan docs are
   the durable design record for this repo.

### Minor (Nice to Have)

1. **`internal/server/presentation_test.go:222` — one test name missed the casual rename sweep.**
   `TestProposalsGroupTitlesNonDeveloper` is a *newly added* test in this diff (not pre-existing —
   confirmed via `git log -p` on the range) sitting right next to two correctly-renamed siblings in
   the same file/PR (`TestProposalCardCasual`, `TestNavCasual` in `server_test.go`). Its body already
   uses the new `audience: casual` fixture value (`writeGermanProfile`), so behavior is correct —
   only the identifier is stale. The plan's own B5 sweep criterion ("test names `*NonDeveloper*` →
   `*Casual*`") should have caught this since it's new code in the same commit, not legacy debt.
   Rename to `TestProposalsGroupTitlesCasual`.

2. **`internal/repos/registry.go:63-68` — new struct fields break the field-ordering convention
   they're inserted into.** `Registry`'s guarded-state block was `mu, repos` (alphabetical); the
   diff inserts `loadedModTime, loadedSize` between `mu` and `repos`, i.e. `mu, loadedModTime,
   loadedSize, repos` — not alphabetical (`repos/go.md` RULE-GOLANG-STRUCT-002). Since this whole
   block is touched in this diff (not merely adjacent to it), the new fields should sort before
   `mu`: `loadedModTime, loadedSize, mu, repos`. Cosmetic only — no functional impact.

3. **`cmd/configserver/main.go` `startPeek`/plist wiring — an untracked `--back-link` parameter.**
   The shipped `startPeek(ctx, bin string, port, controlPort int, backLink, claudeHome, codexHome
   string) bool` and its `--back-link` flag to the spawned peek-mcp process aren't mentioned
   anywhere in `plans/v1-2-1/2026-08-30-peek-multiple-profiles-wip-78bfb3e/design/change-multi-profile-ports.md`'s
   Phase 2 Changes list (which only names claude-home/codex-home/identity-check). It's presumably a
   legitimate carryover from the peer peek-mcp repo's own (out-of-scope-here) release, but it's an
   undocumented addition against this specific plan — worth a one-line note in the plan's Changelog
   per `ACTION-REVIEW-SPEC-005` (every new concept should trace to a driver) so a future reader
   knows where `--back-link` came from.

## Recommendations

- The dirty-arm-wt / `make audit` false alarm cost real review time to run down; if this checkout
  is reused for future eval rounds, starting each round from a clean `git worktree add` of the head
  SHA (rather than a possibly-stateful working copy) would avoid re-triggering it.
- Given the size of this range (9 plans squashed into one diff), a future review would be faster and
  more thorough per plan if reviews could be requested per-plan-directory rather than for the whole
  release range at once — several of the smaller plans (plan-format-viewer-fixes,
  smine-profile-settings, language-style-setting, railroad-review-observability,
  session-context-report) got only a light pass here purely on time budget, not because they looked
  risk-free.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The core mechanisms (registry/cache concurrency rework, peek profile identity check,
bootstrap wrapper, casual lockout) are well-designed, closely follow their plans, and are covered by
passing tests; nothing here rises to data loss or broken core functionality. The one Important
finding (HX-Redirect swallowing errors on all three new sessions archive/delete/unarchive handlers)
is a real, reproducible defect that should be fixed before merge since it silently hides failure on
every path through three brand-new destructive-ish operations — but it's a small, contained,
one-line-per-call-site fix, not a design problem.
