# Review: smine v1.1.4 → v1.2.1

Range `b4981b88..a342811b`, excluding `sessions/`, `evals/`, `proposals/`, `plans/`. 178 files, +7314/-749.

Reviewed in passes given size: (1) full plan-vs-diff mapping across the 12 plan dirs under `plans/v1-2-1/`, (2) close read of the highest-risk Go surfaces — `internal/acdsl/policy.go` (new self-management gate), `internal/repos/registry.go` + `internal/server/statuscache.go` (cache invalidation rewrite), `internal/peek/peek.go` (session attribution), `internal/acdsl/dist.go`, (3) build/vet/full `go test ./...` plus the new/changed shell suites, run against a **clean worktree checkout of the exact reviewed head commit** (the assigned `arm-wt` checkout itself has drifted from HEAD — see note under Testing), (4) targeted diff reads for the remaining plans (repos/routines UI, plan-viewer format, railroad-review observability). I did not line-by-line review every skill `SKILL.md`/`changelog.json` touch-up or every doc file; those are Low tier (RULE-... prose, no executable surface) and were spot-checked only.

## Strengths

- `internal/repos/registry.go` + `internal/server/statuscache.go`: the read-through registry (stat-guarded reload, read-modify-write `Add`/`Remove`) and the git-fingerprint status-cache invalidation are exactly what `plans/v1-2-1/2026-08-30-repo-cache-reload-wip-46c8d9e` specced, including the subtle bit — `save` recording the post-write file identity so a self-write never triggers a spurious reload, and `DropBranches` deliberately *keeping* the pre-removal fingerprint so the next plain load re-scans. Tests cover the resurrection regression (`TestRegistryAddPreservesExternalEdit`) and the cross-instance case.
- `internal/peek/peek.go` `SessionIndex` (`ByBranch`/`ByCwd`): correctly implements the FACT-REPO-ARCH-017 fix — `internal/server/repos.go:worktreeSessions` prefers branch identity and falls back to cwd, closing the recycled-worktree-dir misattribution.
- `internal/repos/registry.go` `WorktreeCounts`/`WorktreeKindCount` and the `repos_index.html` per-kind cell: matches `2026-08-27-repos-routines-improvements` D7 verbatim, and the dead `handleRepoRemove`/`branches/{branch}/remove` route was actually deleted (cross-checked both companion plans, which independently converge on "delete" — consistent).
- `cmd/acdsl/verifiers/planformat/main.go` `canon` reorder (`Open questions` after `Decisions`) and `context/rules/plan.md` RULE-PLAN-002/018/069/070: matches `2026-08-27-plan-format-viewer-fixes` exactly, fixture pinned, verified against the live file.
- Squash-containment layer (`cmd/worktrees/_lib/verdict.sh:squash_contained`, `evaluate_branch` layer-4 sweep, cache-key `v2` salt): implemented as specced; `cmd/tests/test_print_agent_worktrees_status.sh` and `cmd/tests/test_remove_agent_worktrees.sh` both pass on a clean checkout, including the new squash fixture cases.
- Clean `go build ./...`, `go vet ./...`, and `go test ./...` (all packages) on a fresh checkout of the reviewed head; the four new/touched shell suites (`test_bootstrap_wrapper.sh`, `test_read_gate.sh`, `test_skill_invoke_gate.sh`, `test_sync_skills_overrides.sh`) all pass.

## Issues

### Critical (Must Fix)

#### 1. ACDSL self-management policy has a working-tree-tamperable bypass that disables the entire gate

`internal/acdsl/policy.go`, `CheckPolicy` (~line 329) + `policyBase` (~line 296).

The gate's whole premise (`plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf`, D4) is that an agent branch cannot self-escalate because the *base-ref* policy is authoritative. The implementation mostly gets this right — `basePolicy()` re-reads `acdsl/policy.json` from the resolved base via `git show` and uses *that* for the `Mode == Free` early-out (this is actually a correctness improvement over the plan's own pseudocode in §c1, which checked the working-tree `Policy.Mode` first). But there's a hole in exactly the path the plan's D10 stop-gap was written for:

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)
	if baseErr != nil {
		if policy.Mode == PolicyModeFree {     // <-- policy is the WORKING-TREE copy here
			return nil, nil
		}
		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
	}
```

`policyBase` resolves the merge-base using `policy.BaseRef` when it's non-empty — again, the *working-tree* value, since `LoadPolicy` in `cmd/acdsl/main.go` reads straight off disk with no git-show. An agent in a `gated`/`strict` repo can therefore neutralize the whole feature with a single working-tree edit to `acdsl/policy.json`:

```json
{"mode": "free", "base_ref": "definitely-does-not-exist"}
```

`policyBase` tries only that one candidate, fails to resolve it, returns an error; `CheckPolicy` sees `policy.Mode == PolicyModeFree` (the tampered value, because the base was never reached to fetch the authoritative one) and returns `nil, nil` — clean. `surfaceDelta` (which would have flagged `acdsl/policy.json` itself differing from base) is never reached, because the function returns before getting there. This is precisely the scenario D10 says must be "a hard tool error (exit 2), never a silent pass," and precisely the escalation the feature exists to prevent ("an agent branch cannot self-escalate").

I confirmed this is not a hypothetical by writing and running a reproduction against a clean checkout of the reviewed head commit (`internal/acdsl/policy_poc_test.go`, not part of this diff — written for verification only):

```go
root := newPolicyRepo(t, gatedBasePolicy) // base committed as gated, GOLANG-only
writePolicyFile(t, root, "acdsl/rules.acdsl", ...+rogueRule) // ACDSL-PLAN-901, out of scope
writePolicyFile(t, root, "acdsl/policy.json", `{"mode": "free", "base_ref": "definitely-does-not-exist"}`+"\n")

tampered := Policy{Mode: PolicyModeFree, BaseRef: "definitely-does-not-exist"}
diagnostics, err := CheckPolicy(context.Background(), root, tampered)
// err == nil, diagnostics == [] — the rogue PLAN-scope rule AND the tampering
// of acdsl/policy.json itself both pass silently.
```

Output: `err=<nil> diagnostics=[]`.

The existing test `TestCheckPolicyUnresolvableBase` doesn't catch this because it only exercises an already-gated policy hitting an unresolvable base (`gatedPolicy()` with `Mode: PolicyModeGated`) — it never exercises the case where the working copy itself claims to be free while the base is unreachable. The listed test case "fail-unresolvable-base-hard-error" from the plan's own Tests table was implemented for the honest case only.

**Fix:** the "unresolvable base" hard-error decision (D10) must not depend on the working-tree `Mode`/`BaseRef` at all — those are exactly the two policySurface-adjacent fields an attacker controls. Either (a) always hard-error on an unresolvable base regardless of the working policy's claimed mode (free-mode repos with a genuinely absent base already return early via a different path in normal operation — check what that path actually is before hard-coding this), or (b) resolve the base using a *fixed, non-configurable* candidate list first, and only consult `BaseRef` after confirming (via a resolvable default candidate) that the working tree hasn't been used to redirect resolution.

### Important (Should Fix)

#### 2. railroad-review lane-context fix ships the opposite of the approved plan's D1, undocumented

`skills/quality/railroad-review/workflows/railroad-review.js` (`lanePrompt`), vs. `plans/v1-2-1/2026-08-26-railroad-review-observability-wip-5158764/design/raw.md`.

The plan's D1 (reaffirmed in its own Changelog on 2026-08-26: "`--no-context` fix promoted from experiment arm to direct spec restoration") is unambiguous and gives the exact diff:

```diff
-    `Invoke the Skill tool with skill="railroad-review", then follow the "${direction}" direction definition in its ` +
+    `Invoke the Skill tool with skill="railroad-review" and args="--no-context" (lanes get no doctrine injection — ` +
+    `CTX-002), then follow the "${direction}" direction definition in its ` +
```

The shipped code does the opposite — it *requires* full doctrine injection and adds a new enforcement mechanism (`REQUIRE-SKILL:railroad-review` + `cmd/hooks/skill-invoke-gate.sh`) to guarantee the lane reads it:

```js
`REQUIRE-SKILL:railroad-review\n` +
...
`Invoke the Skill tool with skill="railroad-review" — bare, no args; the hook injects your full review context and ` +
`a gate blocks every other tool call until you have invoked it and read the injected spill in full. Then follow the ` +
`"${direction}" direction definition in its SKILL.md exactly — your context is the injection; do not read files from ` +
`the repo's context directory (denied; language guides excepted), walk only that direction's checklist. `
```

This is a complete reversal of the plan's core, twice-affirmed decision (the "Fix, not experiment" framing, D1, and the 2026-08-26 changelog entry all commit to suppressing injection to save tokens). It may well be the right call — CTX-002's premise ("lanes never invoke a skill") was already shown false in the plan's own Baseline, and it's plausible that later, unrecorded experience showed lanes genuinely need the doctrine — but nothing in the plan documents this reversal, no new Decision row or Changelog entry records it, and the branch-namespace rename (D6, `claude-review/*`) *was* implemented as specced in the same file, so this isn't a stale/superseded plan — it's a live one with one decision silently inverted. Per the house rule ("approved plans... are binding... never silently deviate... if exploration shows a decision is wrong, stop and ask"), this needs the implementer to confirm intent and, if intentional, update the plan (new Decision + Changelog row) rather than leave D1 standing contradicted by the code.

#### 3. `plans/v1-2-1/session-context-report/` plan is entirely unimplemented, with fabricated-looking verification screenshots

`plans/v1-2-1/session-context-report/design/{raw.md,change-context-filters.md}` + 3 PNGs, added within this diff range (not excluded from my requirements review even though `plans/` is excluded from the code diff).

This plan (context-filter chips on the session report, `sessionreport.go`, `session_report.html`, a refactor of `contextdocs.go`'s `contextFilter.URL()`) has zero corresponding code anywhere in the diff, and — more tellingly — `internal/server/sessionreport.go`, `internal/server/templates/session_report.html`, and `internal/server/templates/_session_filter_bar.html` **do not exist anywhere in this repository's history** (`git log --all` for these paths only turns up "temporarily add archived plans" commits). There is no `/sessions/live/{id}` route in `internal/server/server.go`. Yet the plan's `design/ui/` carries three PNGs, including `session_report_filtered.png`, which per the plan's own Verification checklist is only supposed to be captured *after* the filter feature is built and run against a live server.

This is either: a plan (with what look like real screenshots of a real feature) that was written and verified against a build that was never merged into this line of history, or plan/screenshot content that doesn't belong in this repo's `plans/v1-2-1/` at all. Either way, treating `plans/v1-2-1/` as this release's requirements (per the review brief), "all planned functionality present" fails outright for this feature, and the plan directory itself violates its own naming rule (RULE-PLAN-071: `plans/<date>-<slug>-<state>-<hash>/` — this dir is just `session-context-report`, no date/state/hash). Flag for the implementer: confirm whether this plan is in scope for v1.2.1 at all; if not, it should not live under `plans/v1-2-1/`.

#### 4. `skill-invoke-gate.sh`'s Bash-read detection is trivially evaded

`cmd/hooks/skill-invoke-gate.sh` lines 62–74.

The post-invocation "don't read context files directly" enforcement only recognizes `Read` and a two-word `cat <path>` Bash command:

```sh
Bash)
	case "$command" in
	*";"*|*"|"*|*"&"*|... ) ;;
	*)
		set -- $command
		[ "$#" -eq 2 ] && [ "$1" = "cat" ] && target="$2"
		;;
	esac
	;;
```

`head -50 <ctx_dir>/foo.md`, `sed -n 1,80p <ctx_dir>/foo.md`, `grep -A100 x <ctx_dir>/foo.md`, or even `cat <ctx_dir>/a.md <ctx_dir>/foo.md` (three words) all bypass this and read the "injection-only" file straight from disk. Given the hook's own header says it's fail-open by design (missing jq, unreadable transcript, absent marker), this may be an accepted "advisory nudge, not a hard boundary" — but that's worth being explicit about, since the enforcement text told to the lane ("denied ... do not read $target directly") reads as a hard guarantee it doesn't provide. If this is meant to be load-bearing (it gates the entire "no raw context reads" cost-saving premise of item #2 above), tighten the Bash-command detection or state plainly that it only catches the common case.

### Minor (Nice to Have)

- `internal/acdsl/policy.go` `classViolations`: strict-mode branch (`if policy.Mode == PolicyModeStrict { ...; continue }`) runs inside a loop that also computes `editable`/`scope` for every rule even when mode is strict for every rule — harmless (cheap), but the `editable` map build at the top of the function is wasted work on every strict-mode call. Not worth a diff on its own; mention only because it's adjacent to the Critical finding above and will be touched anyway.
- `internal/config/installenv.go` `installEnvValues` reads `install.env` via a bare relative path (`installEnvPath = "install.env"`), i.e. depends on the caller's CWD being the repo root. Every current caller happens to satisfy this, but nothing enforces it — a future caller from a different CWD gets silent defaults (`DefaultPeekControlPort`) instead of an error. Low risk given the narrow, install-time-only usage.
- `plans/v1-2-1/2026-08-26-railroad-review-observability-wip-5158764/design/raw.md` §"Deferred findings" flags `README.md:362` linking a nonexistent `docs/telemetry.md`, and a missing `sessions/parallelize/verdicts.md` directory — both explicitly deferred by the plan, not introduced by this diff, but neither shows up as a tracked open item outside the plan prose (ACTION-REVIEW-DOCS-004 wants known-broken/missing things recorded as tracked items, not just plan-buried).

## Testing

`go build ./...`, `go vet ./...`, and `go test ./...` (full suite, all packages) are green when run against a **clean worktree checkout of `a342811b`**. I flag "clean" deliberately: the assigned `arm-wt` checkout has itself drifted from HEAD (`git status` shows `acdsl/registry.json`, `acdsl/policy.json`, `acdsl/rules.acdsl`, `context/*`, `AGENTS.md`, and several new untracked files locally modified relative to the committed tree — apparently from this session's own tooling/context sync running inside the checkout). Running the Go test suite *in that dirtied checkout* produces a spurious `TestDistShipsReachCoveredSliceWithBinaries` failure, because `internal/acdsl` `Dist` tests read the repo's own `acdsl/registry.json` off disk and the working copy had been overwritten with post-dist (already-rewritten `bin/verifiers/...`) content instead of the committed pre-dist (`go run ./cmd/acdsl/verifiers/...`) form. I verified this is purely a local-checkout artifact, not a code defect: `git show <base>:acdsl/registry.json` and `git show <head>:acdsl/registry.json` are byte-identical (proper source form on both), and the same test passes cleanly on fresh `git worktree add` checkouts of both the base and head commits. No action needed on the reviewed commit for this; noting it because whatever synced the review environment into `arm-wt` is worth checking separately if other review passes hit the same false failure.

Shell suites run directly (not just via `make audit`, which I didn't invoke to avoid mutating the checkout further): `cmd/tests/test_print_agent_worktrees_status.sh`, `cmd/tests/test_remove_agent_worktrees.sh`, `cmd/tests/test_skill_invoke_gate.sh`, `cmd/tests/test_bootstrap_wrapper.sh`, `cmd/tests/test_read_gate.sh`, `cmd/tests/test_sync_skills_overrides.sh` — all pass.

## Recommendations

- Add a policy_test.go case that pins the exact PoC in Critical #1 (tampered working `mode`+`base_ref` in a repo committed as `gated`) — the existing `TestCheckPolicyUnresolvableBase` name suggests it was meant to cover this and doesn't.
- Given #1 and #2 both concern security/behavior surfaces that are supposed to prevent an agent from silently loosening its own constraints, it's worth a second look specifically for "does the check trust anything read from the working tree that the mechanism is designed to make untrustworthy" — the same class of bug could recur elsewhere in the policy engine (e.g. `overlayViolations` reads `LocalOverrides` off the *base-authoritative* policy correctly, so this looks like it was fixed everywhere except the one early-return path).
- Reconcile plan state before merge: confirm #2's reversal (update the plan or the code) and confirm #3's plan is either scoped out of v1.2.1 or genuinely missing an implementation.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of the diff (registry/statuscache rewrite, peek session attribution, repos/routines UI, plan-viewer format, squash-containment) is solid, plan-conformant, and green end-to-end on a clean checkout. But the new ACDSL self-management policy engine — the release's main security-relevant addition — has a confirmed, reproducible bypass that defeats its own threat model (an agent branch escalating past `gated`/`strict`), and one skill change directly contradicts its approved plan without documentation. Both need resolution (or explicit plan sign-off, for #2) before this ships as the mechanism people will rely on to keep agent-editable repos from self-loosening their rules.
