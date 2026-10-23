# Review: smine v1.1.4 → v1.2.1 (b4981b88 → a342811b)

Scope note: 178 files, ~7,300 insertions across one large commit
(`22f663c`) plus supporting commits (acdsl policy, bootstrap wrapper,
smine pipeline split, docs). This was reviewed in passes, prioritized by
risk tier (ACTION-REVIEW-RISK-001/002/003): (1) the repo registry/status
cache rework and its plan (`plans/v1-2-1/2026-08-30-repo-cache-reload-*`),
(2) the repos/routines UI plan (`2026-08-27-repos-routines-improvements-*`),
(3) the ACDSL self-management policy engine (`2026-08-26-acdsl-policy-modes-*`,
security-relevant), (4) the headless bootstrap wrapper
(`2026-08-27-bootstrap-profile-improvements-*`), (5) routines/launchctl
changes, (6) a build + targeted `go test` pass. The smine-pipeline skill
split (SKILL.md/changelog.json churn across `skills/smine/*`) and the
remaining plan dirs (language-style-setting, plan-format-viewer-fixes,
peek-multiple-profiles, smine-profile-settings, session-context-report)
were read for context but not verified line-by-line against their diffs —
flagging that as a coverage gap rather than asserting they're clean.

`go build ./...` is clean. Targeted `go test ./internal/repos/... ./internal/server/... ./internal/acdsl/... ./internal/routines/...` is green **except** `TestDistShipsReachCoveredSliceWithBinaries`, which I traced to pre-existing, uncommitted corruption of this checkout's own `acdsl/{registry.json,policy.json,rules.acdsl}` (working tree already carries a `Dist`-shaped rewrite of its own source files, dated before this review session started — `git status --porcelain` shows them modified, `git diff` against the same commit shows no such change). I confirmed the test passes cleanly against a pristine worktree of the exact head commit (`a342811b`) and against the base commit. **This is environment/harness contamination in this specific arm checkout, not a code regression** — but it means this checkout cannot be trusted for further local verification without discarding those three files' working-tree state (which I did not do, per the read-only instruction).

## Strengths

- **Registry read-through (`internal/repos/registry.go`)**: cleanly closes the stale-in-memory-slice resurrection bug. Stat-before-read ordering in `readRegistrySnapshot` correctly re-detects a write racing the read on the *next* call rather than missing it. `Add`/`Remove` re-reading the file under the write lock before applying their delta is exactly right for the stated goal (a second process's Add/Remove can never be silently overwritten by this process's stale view).
- **Status cache fingerprinting**: fingerprint-on-every-plain-load (`git for-each-ref` + `git worktree list`, ~20ms) is a good deterministic alternative to a TTL, and `DropBranches` deliberately keeping the *pre-removal* fingerprint (so the next plain load re-scans and converges to truth) is a subtle, well-reasoned piece of the design — the code comment correctly explains why.
- **ACDSL policy engine (`internal/acdsl/policy.go`)**: the shipped implementation is *better* than the plan's own pseudocode. The plan's `CheckPolicy` short-circuits on `policy.Mode == PolicyModeFree` using the passed-in (working-tree, agent-editable) policy — which would make the entire feature trivially bypassable by editing the local `mode` field to `"free"`. The actual implementation resolves `policyBase` first and then loads the **authoritative base-ref policy** via `basePolicy`/`GitShowFile`, using *that* for every subsequent mode decision. This is a real, positive design correction over the plan (see Important finding below for the one remaining gap).
- **Bootstrap wrapper (`cmd/bootstrap/run.sh`)**: a faithful, careful implementation of the plan — preflight (branch/clean-tree/deps), env→flag translation, per-stage `--allowedTools`, dry-run mode, and the "never abort mid-run, orchestrate is the terminal judge" contract are all present and match the plan's example code almost verbatim, with small hardening improvements (`cd ... || exit 70`, properly-scoped `local flags=()`).
- **Squash-containment layer** (`cmd/worktrees/_lib/verdict.sh`, not detailed above but reviewed): the `git merge-tree --write-tree` content-equality approach is a sound, conservative way to detect squash transfers without touching the working tree, and correctly runs only after the existing three layers leave a candidate partially unexplained.
- Build is clean; the touched packages' existing tests pass (registry, statuscache, repos handlers, routines, most of acdsl).

## Issues

### Critical (Must Fix)

None found. I did not find a data-loss, security-bypass-with-no-mitigation, or broken-functionality issue that blocks merge outright — the two acdsl/policy items below are real but each is either mitigated by a second layer or requires a compound, unlikely precondition.

### Important (Should Fix)

1. **Undocumented, unplanned rework of the worktree filter mechanism** — `internal/server/repos.go` (`worktreeFilter`, `handleRepoWorktreeStatus`), `internal/server/templates/_worktree_status.html`, `internal/server/templates/repo_detail.html`.
   - The diff removes server-side, query-param-driven filtering (`?base=&kind=`, `worktreeFilterQuery`) entirely and replaces it with a client-side JS mechanism (`applyFilter()`, `data-kind`/`data-base` attributes, `.wt-filter` chip click handling, `history.replaceState`).
   - This is **not in either plan that touches this file**. The repo-cache-reload plan's own Hot Items section states explicitly: *"no template or JS changes at all in the final scope — the UI is untouched."* The repos-routines-improvements plan's Phase 3/D7 only touches the Worktrees-column cell; its own D6 is about routine-detail status placement, unrelated to this filter rework. Neither plan's Changes/Decisions sections describe this filtering redesign, yet the code comment in `repo_detail.html` cites `(D6)` as if it were an approved decision — that ID belongs to a different plan's different decision.
   - Per ACTION-REVIEW-SPEC-005 / RULE-PLAN-069-070, a change to user-facing UI needs a screenshot of the actual UI and a Changes-entry `ui:` line; this plan dir (`2026-08-30-repo-cache-reload-wip-46c8d9e`) has no `design/ui/` folder at all, consistent with its own (now-inaccurate) claim that the UI wasn't touched.
   - Functionally the new mechanism looks correct on inspection (reads `location.search` on load so deep links still work, reapplies on every `htmx:afterSettle`), but it is a real behavior change — filtering is now JS-required where it previously degraded to plain links — that was never put in front of anyone for approval and isn't screenshotted anywhere in the plan dirs.
   - Fix: either retroactively document this as a decision in the repo-cache-reload plan (Changelog row, screenshot, `ui:` line) with the actual rationale, or confirm with the author whether this was an intentional scope addition.

2. **ACDSL policy: unresolvable-base fallback trusts the attacker-editable working file** — `internal/acdsl/policy.go:329-336` (`CheckPolicy`).
   ```go
   base, isBoundary, baseErr := policyBase(ctx, root, policy)
   if baseErr != nil {
       if policy.Mode == PolicyModeFree {
           return nil, nil // no base to gate against, nothing declared — free
       }
       return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
   }
   policy, err := basePolicy(ctx, root, base, policy)
   ```
   The normal path (a simple edit of the local `acdsl/policy.json`'s `mode` field) is correctly closed — `basePolicy` overrides `policy.Mode` with the base-ref's real value before anything is decided, exactly matching D2/D4's intent that "an agent branch cannot self-escalate." But when `policyBase` itself fails to resolve (no reachable `origin/HEAD`/`origin/main`/`main`/`master`, or a corrupted `base_ref`), the fallback re-checks `policy.Mode` — which at this point is still the **working-tree copy**, i.e., exactly the file the whole feature exists to make untamperable. An agent that (a) edits `acdsl/policy.json`'s `mode` to `"free"` via a route the fail-open PreToolUse hook doesn't intercept (e.g. `Bash` with `sed -i`/heredoc rather than `Write`/`Edit`/`NotebookEdit`) and (b) makes base resolution fail (deleting the local `main` ref, working in a shallow/single-branch clone) gets a silent pass instead of D10's mandated hard error.
   - This requires two compounding, unusual actions rather than a single file edit, and the hook (`cmd/hooks/acdsl-policy-guard.sh`) already denies direct `Write`/`Edit` of `policy.json` once mode is gated/strict, so the practical exposure is narrow — but it directly contradicts the stated design goal ("never a silent pass," D10) and the test suite doesn't cover this specific combination (`TestCheckPolicyUnresolvableBase` only tests with an already-gated `policy.Mode`, never a locally-forged `free`).
   - Fix: when `policyBase` fails, treat *any* declared mode (i.e., a `policy.json` present on disk at all, gated/strict/free) as requiring the hard error unless there truly is no policy file — or resolve the mode from a source that isn't itself the surface being protected (e.g., require `base_ref` to be set for gated/strict repos and fail if it's unset AND unresolvable, rather than trusting the local mode field either way).

3. **`registry.local.json`-style multi-concern single commit** — the entire feature set here (registry rework, status cache, UI filter rework, bootstrap wrapper, ACDSL policy, presentation/profile settings, install-env, i18n) landed in one commit (`22f663c`) touching every layer of the codebase at once. `context/rules/go.md`'s Enforcement section calls out "per-package logical commits" as a concern this repo cares about (`RULE-GOLANG-FILE-002`/commit conventions), and the size makes this specific review materially harder to do at full confidence — several plan dirs (peek-multiple-profiles, smine-profile-settings, plan-format-viewer-fixes, language-style-setting) were not independently verified against the diff within this pass. Not asking for a revert, but flagging that the "one plan → one commit" discipline visible elsewhere in this repo's history (see the five-commit train around it: `acdsl:`, `context:`, `skills:`, `routines:`, `cmd:`) didn't hold for this specific commit.

### Minor (Nice to Have)

1. **Inline composite literal passed as a call argument** — `internal/repos/registry.go:126`:
   ```go
   r.apply(&registrySnapshot{modTime: info.ModTime(), repos: repos, size: info.Size()})
   ```
   `RULE-GOLANG-CALL-005` requires binding a composite literal to a named variable before passing it as a call argument (the sibling construction three lines earlier in `readRegistrySnapshot`, `snapshot := &registrySnapshot{...}; return snapshot, nil`, already follows this correctly — it's just this one call site in `save` that doesn't). Purely stylistic; no functional impact.

2. **Log line comment drift** — the `Fingerprint`/`WorktreeCounts` doc comments in `registry.go` are good, but `handleRepoWorktreeStatus`'s timing log (`internal/server/repos.go`) now folds the fingerprint-check cost into the `status` timing bucket unconditionally (per the plan, intentionally), which means the `status=Nms` value on a cache hit is no longer "the scan took N ms" but "the fingerprint check took N ms" — the plan calls this out explicitly as intentional ("truthful"), so this is not a bug, just worth confirming anyone reading historical timing logs across the release boundary knows the semantics changed.

3. **`writePolicy`'s "no source policy ships nothing" behavior** (`internal/acdsl/dist.go`) — matches the plan, but the doc-comment vs. behavior split (returns `""` silently, caller checks `if distMode != ""`) is a slightly indirect way to signal "nothing shipped"; a named sentinel or a second boolean return would read more clearly per `RULE-GOLANG-FUNC-001`'s "at most 3 returns" ceiling not being tight here (currently 2). Not worth a signature change on its own.

## Recommendations

- Before the next release, reconcile the repo-cache-reload plan's Hot Items/Changes sections with what actually shipped for the worktree filter UI (Important #1) — either as a plan Changelog entry with a screenshot, or by confirming it should be reverted to the query-param design if it wasn't actually wanted.
- Add a test case to `internal/acdsl/policy_test.go` that specifically forges a local `free` mode combined with an unresolvable base, to lock in whatever fix is chosen for Important #2.
- Consider whether the config-server test suite would benefit from a "does this checkout's own `acdsl/` tree match `git show HEAD:acdsl/`" sanity assertion in CI — this review's environment corruption (dist output landing on the source tree) would have been caught immediately by such a check, and if it can happen to a review harness it can happen to a dev machine that ran `Dist` against the wrong `dest`.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The core mechanisms (registry read-through, status-cache fingerprinting, bootstrap wrapper, squash-containment layer) are well-designed, tested, and in several places improve on their own plans' pseudocode. Nothing here is an outright bug that breaks a happy path. But the undocumented worktree-filter UI rework (Important #1) needs a decision from whoever approves plan deviations before this ships as-is, and the ACDSL policy engine's unresolvable-base fallback (Important #2) should be tightened given this feature exists specifically to be tamper-resistant — shipping a security-adjacent feature with a known, uncovered edge case in its own threat model is worth a fix or an explicit accepted-risk note before merge, not a silent pass.
