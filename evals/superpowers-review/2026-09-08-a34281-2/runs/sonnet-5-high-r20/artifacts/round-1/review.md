# Review: smine v1.1.4 → v1.2.1 (b4981b88..a342811b)

Scope note: 178 files, +7314/-749 across the excluded-path filter (`sessions/`, `evals/`,
`proposals/`, `plans/` excluded). This review was done in passes: full read of all nine
plan documents under `plans/v1-2-1/`, then deep line-level review of the highest-risk
surfaces (ACDSL policy engine, repo registry concurrency, status-cache invalidation,
launchd routine gating, new PreToolUse hooks), then a build/vet/test pass, then a spot
check of the remaining UI/skill/doc changes against their plans. Low-risk areas (docs,
skill changelog bumps, template-only reorders already called out in a plan) were read but
not line-annotated.

Verification performed: `go build ./...` (clean), `go vet ./...` (clean), `go test ./...`
(see Issues — one flaky/environmental failure, details below; repeated clean runs pass).

## Strengths

- **`internal/repos/registry.go` read-modify-write fix** is the standout piece of this
  release. It directly targets the documented resurrection bug (FACT-REPO-ARCH-016): `Add`
  and `Remove` now re-read the file fresh under the write lock instead of trusting the
  in-memory slice, and `save` records the post-write `(mtime, size)` so a self-write never
  triggers a spurious reload. `TestRegistryDeleteStaysDeletedAcrossInstances` is exactly the
  right regression test — it instantiates a second `Registry` against the same path to
  reproduce the original two-instance bug and asserts the delete survives it.
- **`internal/server/statuscache.go`** (new) is a clean, minimal cache: `sync.RWMutex`
  guarding a typed map (never `sync.Map`, per house rule), immutable stored slices,
  fingerprint-based invalidation computed from two cheap git reads
  (`internal/repos/registry.go:Fingerprint`). The design matches the repo-cache-reload
  plan's D3/D4 almost verbatim, including the deliberate choice to leave `DropBranches`'
  pre-removal fingerprint stale so the next plain load re-converges to scan truth.
- **ACDSL policy engine (`internal/acdsl/policy.go`)** is well-reasoned. The threat model in
  the plan (self-defeating registry verifier, `registry.local.json` bypass, base-branch
  privilege boundary) is fully implemented in code, and the implementation actually
  **improves on the plan's own literal code** in one specific spot: the plan's `CheckPolicy`
  sketch reads the *working-copy* `policy.json` to decide the mode, which means an agent
  branch could simply edit `acdsl/policy.json` to `mode: "free"` and short-circuit the gate
  before it ever runs `surfaceDelta`. The shipped code adds `basePolicy()`, which fetches the
  base-ref's `policy.json` via `git show` and treats it as authoritative once a boundary is
  established — an agent branch cannot weaken the mode by editing its own working copy. This
  closes a real bypass that the plan's Changes section did not anticipate. (See Issues for
  the corresponding process gap — this deviation isn't recorded anywhere.)
- **`internal/routines` launchd fixes** (`overrideLabels`, `DefaultDisabled`,
  `Unscheduled`/`otherTriggerKeys`) are precise, well-tested, and match FACT-REPO-ARCH-012's
  loaded-vs-running distinction and the `default-disabled` marker file convention cleanly.
  `History`'s handling of a "started" marker row (superseded unless it's the newest line) is
  correct and its edge cases (single-line file, crash-then-restart) check out by hand-trace.
- Build and `go vet` are clean across the whole module; the bulk of the new/changed Go code
  (`registry.go`, `statuscache.go`, `policy.go`, `git.go`, `plist.go`, `routines.go`)
  follows the repo's own style guide (file/type ordering, error-wrap prefixes, receiver
  naming, table-test shape) closely — noticeably more so than average for a diff this size.
- `acdsl/policy.json` itself models the recommended v1 defaults from the plan's `[USER]`
  decision exactly (`mode: free`, `dist_mode: gated`, language scopes editable, PLAN/SKILL
  protected).

## Issues

### Critical (Must Fix)

None. I did not find a data-loss, security, or broken-functionality issue that should block
merge outright. The items below are all either confirmed-but-narrow correctness gaps or
process/documentation gaps.

### Important (Should Fix)

1. **`cmd/hooks/skill-invoke-gate.sh` Bash-command detection is defeated by ordinary
   quoting.** (`cmd/hooks/skill-invoke-gate.sh`, the `case "$command" in` block reading
   `*'"'*|*"'"*|...) ;;`)
   The hook is meant to deny a direct `cat` of a protected context file once a required
   skill has been invoked. Its `Bash` handling only recognizes the target when the raw
   command splits into exactly two words (`cat`, `path`) — any command containing a quote
   character (single or double) is treated as "too complex to parse" and silently exits the
   `case` with `target` left empty, i.e. **no enforcement at all**. `cat "docs/rules/x.md"`
   or `cat 'docs/rules/x.md'` — completely ordinary shell style — both fall into this
   fallthrough and are never checked. Since this is the mechanism a review/eval harness
   relies on to prove a skill's context was actually consulted rather than read around, a
   trivially available quoting habit removes the enforcement it exists to provide. Given
   the file already blocklists shell metacharacters for safety, it should instead do a
   minimal quote-aware tokenization (or use `jq`'s `@sh`/word-splitting helpers already
   available in the script) so a quoted single-argument `cat` is still recognized, and treat
   genuinely un-parseable commands as denyable-if-suspicious rather than pass-through.

2. **Several concrete changes in this diff have no corresponding entry in any
   `plans/v1-2-1/*` plan**, and one of them contradicts an existing plan's explicit scope
   statement. Please confirm these were intentional and simply undocumented, versus
   accidental scope creep:
   - `cmd/hooks/read-gate.sh` was extended to require subagents to read the context spills
     of skills *they themselves* invoked (new `else` branch parsing the transcript for
     `Skill` tool_use entries). The only plan that discusses this exact mechanism
     (`plans/v1-2-1/2026-08-26-railroad-review-observability-wip-5158764/design/raw.md`,
     line 38) states explicitly: *"skill-context.sh / read-gate.sh: the hooks stay as-is;
     the fix rides the existing opt-out token."* That plan's own F4 baseline fact records
     the subagent exemption as a known, accepted gap feeding a different decision (D4,
     entry eviction) — it does not propose fixing it. The shipped diff fixes it anyway,
     with no Decisions/Changes entry anywhere justifying the change or the risk (a subagent
     now fails a PreToolUse deny if it hasn't itself invoked the skill whose spill exists,
     which changes headless subagent behavior for every skill-context consumer, not just
     the one plan under discussion).
   - `cmd/hooks/skill-invoke-gate.sh` (new, 85 lines) has no design-plan entry at all —
     grep across `plans/v1-2-1/` finds it referenced only from a runbook
     (`railroad-review-observability/runbooks/trace.md`), never from a plan's Changes/Hot
     items/Decisions sections. Per this repo's own doctrine (ACTION-CONCEPT-HOT-005 / the
     plan-format rules), new guard/validation logic like this is exactly what an approved
     example in a plan is supposed to precede.
   - The worktree-status page's client-side filter rewrite and the new "Reload" button
     (`internal/server/templates/repo_detail.html`, `_worktree_status.html` — replacing
     server-rendered `?base=&kind=` filter links with a `data-*`/JS filter engine, adding a
     `reload-form` block with a `status-reload-indicator`) is not in any plan either. It's
     adjacent to, but goes beyond, what `2026-08-30-repo-cache-reload-wip-46c8d9e` describes
     — that plan's own Hot Items section states as a design constraint: *"UI
     (ACTION-CONCEPT-HOT-007): no template or JS changes at all in the final scope — the UI
     is untouched."* The shipped commit clearly does touch and meaningfully restructure that
     template's JS. This is also the kind of user-facing UI change RULE-PLAN-069 asks to be
     backed by a screenshot in the plan — none exists for this specific mechanism.

   None of these three are bad changes on their own merits (the read-gate and status UI
   changes both look correct and are covered by passing tests/inline comments), which is
   why I'm not marking them Critical. But given how much weight this repo's own process
   puts on "every new concept traces to a requirement" (ACTION-IMPL-ARCH-001 /
   ACTION-REVIEW-SPEC-005) and on plans being the binding contract, three separate
   untracked changes — one of which contradicts an explicit "stays as-is" — is a pattern
   worth surfacing rather than three isolated nitpicks.

3. **`internal/acdsl/git.go:GitChangedFiles` splits git output on whitespace, not
   newlines**, which mis-parses any path containing a space:
   ```go
   for _, path := range strings.Fields(diffed + "\n" + untracked) {
   ```
   `strings.Fields` splits on *any* run of whitespace, so a changed file named e.g. `acdsl/my rule.acdsl` would be split into two bogus path fragments, neither of which resolves to the real file — and `ruleDelta`/`surfaceDelta` (which consume this list) would silently miss the real change. This is exactly the kind of gap `ACTION-IMPL-INTEG-003`-adjacent doctrine cares about for a self-management *security* gate: a rule file with a space in its name would dodge the delta scan entirely in gated/strict mode. The fix is a straight `strings.Split(diffed, "\n")` + same for `untracked`, trimming empties, rather than `Fields` over the concatenation.

### Minor (Nice to Have)

4. **`internal/shell.RunSync` is a near-verbatim copy-paste of `internal/shell.Run`** —
   identical body except the timeout constant and the error-message prefix. This is the kind
   of duplication the repo's own Go guide calls out as review-flaggable
   (`RULE-GOLANG-TYPE-001`'s "reimplementing existing infrastructure" language, and
   `ACTION-REVIEW-QUALITY-005`). A small `runWithTimeout(ctx, dir, timeout, errPrefix, name string, args ...string)` shared by both would remove ~20 duplicated lines without changing either call site's signature.

5. **`internal/acdsl/dist_test.go:TestDistShipsReachCoveredSliceWithBinaries` reads the live
   repo tree via a relative path (`repoRoot = "../.."`) rather than a synthetic fixture.**
   During this review, a full `go test ./...` run intermittently produced a real, on-disk
   rewrite of this checkout's own `acdsl/policy.json`, `acdsl/registry.json`, and
   `acdsl/rules.acdsl` into their *distributed* (dist-target) form, and the same run failed
   this test with `stat .../bin/verifiers/gofmt: no such file or directory`. I could **not**
   reproduce this in six subsequent isolated runs (individual packages, package groups,
   cleared test cache, full `go test ./...` again) — build, vet, and the full suite were all
   green every other time. Given this host runs many parallel worktrees of this same shared
   git repository for this eval batch (confirmed via `git worktree list` — 30+ sibling
   worktrees at the same commit, plus a live locked worktree on `main`), the far more likely
   explanation is an external process on the shared host (a live config-server/routine scan
   against a registered copy of this repo) racing against the test run, not a bug in the
   reviewed code — I want to be explicit that I cannot pin this on a specific line in this
   diff. Flagging it only because a dist test that reads its fixtures straight from the
   live checkout is inherently exposed to exactly this class of interference in a
   multi-worktree environment; pointing it at a synthetic fixture repo (as
   `TestDistNoCoverageWritesNothing` already does two tests below it) would remove the
   exposure regardless of the actual cause.

6. **`internal/repos/registry.go:Repo.Label`** — the length cap (`labelMaxLength = 80`) and
   reserved-name check for `Name` are both good, narrowly-scoped additions exactly matching
   the plan's C16/C19 decisions. No issue; noting only that this is one of the few new
   fields in the diff with a `json:"...,omitempty"` tag while its sibling fields (`Name`,
   `Path`) don't use `omitempty` — consistent with the plan's intent (`Label` optional,
   `Name`/`Path` load-bearing) so not a defect, just worth a second pair of eyes at review
   time since it's an asymmetry in an otherwise-uniform struct.

## Recommendations

- Add a Changelog row to `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/` (it's
  already marked `done`) documenting the `basePolicy()` deviation from the plan's literal
  `CheckPolicy` sketch — it's a security-relevant hardening that a future reader of the plan
  (which is the design record for this feature) would otherwise never learn happened.
- Resolve the three plan-alignment gaps in Important #2 with the author before merge — even
  if the answer is "yes, intentional, the plan was written before we realized X", a one-line
  Changelog entry in the nearest plan (or a short new plan section) closes the loop per this
  repo's own RULE-PLAN-049.
- Consider whether `skill-invoke-gate.sh`'s enforcement gap (Important #1) is acceptable
  given its stated "fast feedback, fail-open" design philosophy, or whether — given what it
  gates (review/eval integrity) — it needs to be closer to authoritative. The ACDSL policy
  hook explicitly designs around this exact question (D2 in that plan: "a registry verifier
  is self-defeating... reliable only in the binary") and lands on a different, more
  conservative answer for a comparably important surface; worth applying the same standard
  here or documenting why the lower bar is fine for this hook.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The core, highest-risk work (ACDSL policy engine, registry
read-modify-write, status-cache fingerprinting, launchd routine gating) is solid,
well-tested, and in the policy-engine case actively improves on its own plan. Nothing found
rises to a release-blocking bug. But the `skill-invoke-gate.sh` quoting bypass (Important
#1) and the `GitChangedFiles` whitespace-splitting gap (Important #3) both sit in
enforcement/security-adjacent code and are cheap, mechanical fixes — I'd want those in
before shipping. The plan-alignment gaps (Important #2) are lower urgency technically but
should get a yes/no from the author given how much this repo's own process leans on plans
being the binding, traceable record.
