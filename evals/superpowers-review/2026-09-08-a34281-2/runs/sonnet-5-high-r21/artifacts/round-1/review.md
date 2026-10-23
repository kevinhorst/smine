# Review: smine 1.1.4 → 1.2.1 (b4981b88 → a342811b)

Scope note: 178 files, +7314/-749 across the excluded-path filter. Reviewed in
two passes — a broad pass over all 9 plan docs under `plans/v1-2-1/` (TLDR /
Drivers / Scope / Decisions) plus the full diff stat, followed by a deep pass
on the higher-risk surfaces: the repo registry read-through and worktree
status cache invalidation, the new ACDSL self-management policy engine and
its guard hook, the squash-detection git logic, the headless
bootstrap/orchestrate wrapper, and the sessions archive/delete feature.
Lower-risk, more mechanical changes (template-only edits, i18n catalog
entries, most SKILL.md prose/version bumps, routine plist/README docs) were
sampled rather than diffed line-by-line — flagged here so it isn't mistaken
for full coverage.

Verification performed: `go build ./...`, `go vet ./...`,
`go test -buildvcs -vet=off ./...`, and the full `cmd/tests/test_*.sh` shell
suite, run against a **clean temporary worktree** checked out at the reviewed
head (`a342811`) — all green. The supplied `arm-wt` checkout itself carries
unrelated uncommitted local drift (`acdsl/policy.json`, `acdsl/registry.json`,
several `context/*` files modified outside the reviewed range — visible via
`git status`), which reproduces a spurious `TestDistShipsReachCoveredSliceWithBinaries`
failure if the suite is run there directly. That failure is **not** a defect
in the reviewed diff; it disappears on a clean checkout of the same commit.

## Strengths

- **`internal/repos/registry.go` read-through rewrite** is a clean, correct
  fix for the documented last-writer-wins bug (`FACT-REPO-ARCH-016`): `Add`/
  `Remove` now re-read the file before mutating (`readRegistrySnapshot`), and
  reads (`Find`/`Repos`) auto-reload on a stat-detected mtime/size change.
  Well tested, including the exact "two instances, one deletes, one adds"
  regression (`TestRegistryDeleteStaysDeletedAcrossInstances`) and external
  edit/delete visibility.
- **`internal/server/statuscache.go` fingerprint invalidation** is a solid,
  minimal design: `DropBranches` deliberately keeps the pre-removal
  fingerprint stale so the *next* plain load re-scans instead of re-serving
  hidden rows — documented and exactly matches the bug it's fixing.
- **ACDSL policy engine (`internal/acdsl/policy.go`, `git.go`)** implements
  the three-mode self-management contract cleanly: base-ref resolution,
  strict/gated/free semantics, task-lifetime exemption, and the local-overlay
  allowlist all match `acdsl/README.md`'s "Modes" section and the plan's
  D1/D7/D8/D11. Good test breadth (13+ cases: strict/gated scopes, changed
  marker lines, verifier allowlist, overlay violations, task-contract
  exemption including the "base disables the exemption" case, unresolvable
  base, bootstrap-branch-introduces-policy).
- **Squash-detection layer 4 (`cmd/worktrees/_lib/verdict.sh`)** — the
  `git merge-tree --write-tree` containment check is well-isolated
  (`squash_contained`), degrades gracefully on git < 2.38 (nonzero exit →
  `return 1`, falls back to prior "unpicked" behavior rather than crashing),
  and the shell test suite includes both the happy path
  (`test_squash_transfer_reads_contained`) and the important negative case
  (`test_edited_squash_stays_unpicked`, where main further edits the squashed
  content and containment correctly stays broken).
- **Sessions archive/delete (`internal/sessions/sessions.go`)** —
  `validateScopeName` rejects empty/`.`/`..`/embedded separators/anything
  that isn't its own `filepath.Base`, and `DeleteScope` additionally requires
  the target to already exist under `sessions/archived/` before
  `RemoveAll`. No path-traversal or accidental live-folder-deletion vector
  found.
- Build, `go vet`, the full Go test suite, and the shell test suite are all
  green on the actual reviewed commit.
- Plan-alignment spot checks (routine-detail Status/configure/run-history
  ordering, week-band ISO-Monday grouping algorithm, gated policy defaults
  `mode: free` / `dist_mode: gated` in the committed `acdsl/policy.json`) all
  matched their plans' decisions.

## Issues

### Critical (Must Fix)

None found.

### Important (Should Fix)

1. **Fast policy hook is stricter than the authoritative check, making the
   sanctioned local-override feature unusable via normal tool calls.**
   `cmd/hooks/acdsl-policy-guard.sh:43-44` unconditionally denies any
   Write/Edit to `acdsl/registry.local.json` whenever the repo is in
   `gated`/`strict` mode. But `internal/acdsl/policy.go`'s `overlayViolations`
   (lines 139-173) explicitly *permits* overlay entries listed in
   `policy.LocalOverrides` — that's the entire point of the
   `local_overrides` field per `acdsl/README.md`'s Modes section ("the
   overlay is policy-gated: only entries listed in the policy's
   `local_overrides` are accepted"). Since the fast hook fires before any
   authoritative check runs and blocks the write outright, an agent can
   never actually author a sanctioned local override through the Write/Edit
   tools in a gated/strict repo — it would have to bypass the hook somehow.
   The design doc for this feature (D7) frames the hook as pure UX
   ("advisory... the authoritative layer is the binary check — one
   enforcement truth, one convenience layer"), but here the two layers
   disagree on outcome, not just on timing. Fix: teach the hook to read
   `policy.local_overrides` and only deny entries not yet present there (or
   drop `registry.local.json` from the hook's frozen-surface list and let it
   fall through to `acdsl check`, since that's where the real judgment
   already lives).

2. **Casual skill-edit lockout is enforced only in prose, with no mechanical
   backstop.** DR3 ("casual installs must never receive skill-edit proposals
   against shipped skills") is implemented purely as instructions inside
   `skills/smine/smine-apply/SKILL.md:31`, `skills/smine/smine-skills/SKILL.md:45`,
   and `skills/smine/smine-orchestrate/SKILL.md:41` — each is framed as a
   "hard gate" / "hard invariant breach", but nothing code-level checks it.
   Contrast this with the ACDSL policy feature landing in the very same
   release, which pairs a fail-open `PreToolUse` hook with an authoritative
   Go check compiled into the binary ("so no data file can neuter it," per
   `acdsl/README.md`). The casual lockout has no equivalent: it depends
   entirely on the mining agent, the apply agent, and the orchestrate judge
   agent all correctly reading the profile and frontmatter on every run. A
   single missed instruction (model drift, a future skill revision that
   forgets the check) ships a shipped-skill edit to a casual user with
   nothing mechanical in the loop to catch it. Recommend at minimum a
   deterministic check in the orchestrate stage (or the wrapper itself) that
   diffs the touched `skills/*/SKILL.md` frontmatter against `origin: user`
   when the profile is casual, rather than relying purely on agent judgment
   for a change explicitly called a hard invariant.

3. **`cmd/bootstrap/run.sh` results log grows unbounded across runs.**
   `results="${TMPDIR:-/tmp}/smine-bootstrap-results.jsonl"` (line 23) is a
   fixed, non-run-scoped path, and every stage appends to it (`>> "$results"`,
   line 146) with no truncation at the start of a run. Every bootstrap
   invocation's stage results accumulate in the same file forever. Low
   practical impact today (the file is a debugging aid only — the UI status
   comes from the peek session index, not this file — see
   `internal/server/bootstrap.go`'s `resolveBootstrapSession`), but it will
   grow without bound on a machine that reruns bootstrap repeatedly, and
   makes "the log for this run" ambiguous for anyone reading it directly.
   Fix: timestamp the filename, or truncate it at the top of the script.

### Minor (Nice to Have)

1. `routines/smine-nightly/run.sh:300-306` — the `additionalDirectories`
   reconciliation pipeline (`jq ... "$settings_file" > "$settings_file.tmp" &&
   mv ...`) leaves a stray `.tmp` file behind if `jq` fails (e.g. a malformed
   `settings.json`), since the redirection creates the temp file before jq's
   exit status is checked and the `&&` then skips the `mv`. `settings.json`
   itself is never corrupted, but each such failure leaves debris in
   `~/.claude/`.
2. `internal/acdsl/policy.go`'s `ruleDelta` (lines 259-260) silently discards
   `ParseRules` errors on both the working and base sides
   (`workRules, _ := ParseRules(workLines)`). A declaration file that fails
   to parse on either side would silently under- or over-report
   added/removed rules rather than surfacing the parse error. No test
   exercises this path.
3. `internal/repos/registry.go`'s `Registry` struct field order (`path`,
   `mu`, `loadedModTime`, `loadedSize`, `repos`) isn't alphabetical per
   RULE-GOLANG-STRUCT-002 (the new `loadedModTime`/`loadedSize` fields are
   grouped with the mutex they're guarded alongside via a comment instead).
   Readability tradeoff, not worth churning on its own.

## Recommendations

- The policy-hook/authoritative-check mismatch (Important #1) and the
  casual-lockout enforcement gap (Important #2) share a root cause: this
  release establishes a good two-layer pattern (fast advisory hook +
  compiled authoritative check) for one governance feature (ACDSL rules) but
  doesn't reuse it for a comparably-important governance feature shipped in
  the same release (casual skill-edit lockout). Worth deciding once whether
  "hard invariant" features in this codebase always get a code-level check,
  and applying that consistently rather than per-feature.
- Given the size of this change (9 stacked plans in one review range), a
  follow-up spot-audit of the lower-risk surfaces sampled here (i18n catalog
  completeness against the "clean sweep" plan's DR1-DR4, the presentation.go
  audience/dev-mode gating logic, and the full set of new/changed SKILL.md
  prose contracts) would be worth a dedicated pass if not already covered by
  a separate review.

## Assessment

**Ready to merge?** With fixes

**Reasoning:** The code that was reviewed in depth is correct, well-tested,
and matches its plans — build/vet/tests are all green on a clean checkout of
the reviewed commit. The two Important findings are real design gaps (one
makes a shipped feature unusable as specified, the other leaves a
documented "hard invariant" unenforced by anything but agent prose) rather
than active bugs corrupting data today, so they don't block a merge outright,
but both should be resolved or explicitly accepted before this ships to a
casual, non-developer install — which is exactly the audience these two
gaps affect.
