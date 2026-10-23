# Review: smine v1.1.4 → v1.2.1

Range: `b4981b88` → `a342811b` (excluding `sessions/`, `evals/`, `proposals/`, `plans/`)
178 files changed, ~7314 insertions, ~749 deletions, against 14 plan documents under `plans/v1-2-1/`.

**Method note:** given the size, I reviewed in risk-prioritized passes rather than line-by-line
across all 178 files. Deep verification (plan cross-reference + code reading, and for the
Critical finding below, live reproduction in an isolated `git worktree`) covered: the repo
registry read-through + status-cache fingerprint invalidation (`internal/repos/registry.go`,
`internal/server/statuscache.go`, `internal/server/repos.go`), the ACDSL self-management policy
engine (`internal/acdsl/policy.go`, `dist.go`, the write-guard hook), peek multi-profile identity
and session indexing (`cmd/configserver/main.go`, `internal/peek/peek.go`), sessions archive
(`internal/sessions/sessions.go`, `internal/server/sessions.go`), install/port persistence
(`install.sh`, `internal/config/installenv.go`), routine default-disabled gating
(`internal/routines/*`), and the two new hooks. Templates, most README/doc prose, the smine skill
markdown changes, and most JSON test fixtures were spot-checked rather than exhaustively verified.

`go build ./...`, `go vet ./...`, and `go test ./...` all pass clean on this checkout. (One caveat
for the next reviewer: a session-level context-injection hook in this environment silently
rewrote `AGENTS.md`, `context/**`, and `acdsl/{policy.json,registry.json,rules.acdsl}` in the
working tree partway through this review — unrelated to the diff under review, and reverted with
`git checkout --` before finishing. If you see a modified/`gated`-mode `acdsl/policy.json` on
disk, it did not come from this commit; the committed file is `mode: "free"`.)

## Strengths

- **Repo registry read-through (`internal/repos/registry.go`)** is a clean, well-isolated fix: `readRegistrySnapshot` centralizes stat-then-read-then-validate, `Add`/`Remove` re-read the file before applying their delta (closes the stale-in-memory-slice resurrection bug), and `refreshIfChanged`'s degrade-to-last-good-state-and-log on a bad re-read (rather than 500ing every page) matches the plan's D2 exactly. Matches its plan (`2026-08-30-repo-cache-reload`) almost verbatim, including the documented benign race in concurrent `refreshIfChanged` calls.
- **Status-cache fingerprinting** (`statuscache.go`, `repos.go` `handleRepoWorktreeStatus`) is correctly wired: `DropBranches` deliberately *keeps* the pre-removal fingerprint (documented in both the plan and a code comment) so the next plain load's fingerprint mismatch forces the convergence re-scan — this is the subtle part of the design and it's implemented right, with tests for it (`TestStatusCacheDropBranches` asserts the fingerprint is unchanged after a drop).
- **peek multi-profile identity check** (`cmd/configserver/main.go` `startPeek`/`peekIdentity`): correctly refuses to reuse a same-port peek whose `/healthz` identity (claude-home/codex-home) doesn't match this profile, rather than silently serving another user's session data — a real cross-profile data leak avoided.
- **`internal/sessions` archive/unarchive/delete** (`ArchiveScope`/`UnarchiveScope`/`DeleteScope`): `validateScopeName` correctly rejects path traversal (`filepath.Base` mismatch check) *and* explicitly checks for `/`/`\` for the Windows case beyond what `filepath.Base` alone catches on Unix — good defensive coding for a filesystem-path-from-user-input handler.
- **`install.sh` port/home persistence** (D9's "re-run keeps this profile's ports") is implemented correctly: env vars are captured *before* sourcing `install.env` (so they can win), the file's re-sourced values become the fallback, and `lsof ... || true` correctly guards a possibly-nonzero command under `set -e`.
- The token-file handling in `profilesettings.go` (`handleProfileTest`) correctly keeps the OAuth token out of argv/shell-string interpolation — read via `tr -d` inside the child bash from a file path passed as a positional arg — consistent with the existing token-verify exemplar and the repo's own memory note on this exact hazard.

## Issues

### Critical (Must Fix)

#### 1. ACDSL self-management policy engine can be fully defeated by editing the working-tree `policy.json` — closes none of what it claims to close

**File:** `internal/acdsl/policy.go:329-346` (`CheckPolicy`), interacting with `policyBase` (`policy.go:195-219`)

The whole point of this feature (plan `2026-08-26-acdsl-policy-modes`, D2) is that the gate must not be "neutered by the thing it guards" — specifically, D2 rejects a registry-verifier implementation *because* a target-owned data file could disable it, and the implementation's own doc comment on `CheckPolicy` claims: *"The base-ref policy is authoritative — a branch cannot weaken the mode by editing or deleting the working policy.json."*

That claim is false for one specific combination, and it's the most natural one for someone (or something) trying to escape the gate to try:

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)
	if baseErr != nil {
		if policy.Mode == PolicyModeFree {
			return nil, nil // no base to gate against, nothing declared — free
		}
		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
	}
	policy, err := basePolicy(ctx, root, base, policy)   // only reached if baseErr == nil
	...
```

`policy` here is the **working-tree** copy (`LoadPolicy(root)`, called by `cmd/acdsl/main.go` before `CheckPolicy`) — not yet overridden by the base-ref policy, because that override only happens *after* `policyBase` succeeds. If the working-tree `acdsl/policy.json` is edited to `{"mode": "free", "base_ref": "no-such-ref"}`, `policyBase` fails to resolve the (now single, non-fallback-eligible) candidate, `baseErr != nil`, and the `if policy.Mode == PolicyModeFree` branch fires on the *tampered* value — returning `nil, nil` with zero diagnostics, regardless of what the real (base-branch) policy actually is.

This directly contradicts the plan's own D10 ("an unresolvable base is a hard tool error … never a silent pass") and the `policyBase` function's own doc comment ("none resolving is a hard error, never a silent free") — the caller silently overrides both.

**Verified by reproduction** (in an isolated `git worktree`, not the reviewed checkout): a repo with a `strict` base policy and a rule the base forbids; overwriting the working `acdsl/policy.json` with `{"mode":"free","base_ref":"no-such-ref-xyz"}` and re-running `CheckPolicy` returns `0 diagnostics, nil error` — clean — despite the forbidden rule being present and the base being `strict`. The "honest" (untampered) run on the same tree correctly reports 1 diagnostic. Test file was written and run outside the reviewed checkout, then discarded; not part of any repo.

**Why it matters:** the advisory `cmd/hooks/acdsl-policy-guard.sh` hook only covers `Write|Edit|NotebookEdit`; a `Bash` overwrite of `acdsl/policy.json` (`printf '...' > acdsl/policy.json`, `python3 -c ...`, `sed -i`, etc.) is not intercepted by anything before `acdsl check` runs — and the binary check is supposed to be the *authoritative* layer precisely because the hook is fail-open advisory-only (D2, D7). This bypass means an agent (or user) in a `gated`/`strict` target repo can silently disable the entire self-management gate with one shell redirect, which is exactly the "loosy-goosy-rules failure mode" the feature exists to close structurally.

**Fix:** resolve `base` using a fixed, non-tamperable candidate list (or resolve it *before* trusting anything from the working-tree `policy.json`, including `BaseRef`) — e.g. always try the default candidate chain (`origin/HEAD`, `origin/main`, `main`, `master`) regardless of `policy.BaseRef`, and only use `BaseRef` as an *additional* first-try, never as the *sole* candidate that can make resolution fail outright. Then gate the "unresolvable base → error" decision on whether *any* base resolves at all, never on the mode field of the same file whose authority is in question.

### Important (Should Fix)

None identified beyond the Critical item above — the rest of the reviewed surface held up well against its plans (see Strengths and Minor below).

### Minor (Nice to Have)

1. **`internal/server/profilesettings.go:26-29`** — `styleTestPrompt` deviates from the approved plan text (`2026-08-27-smine-profile-settings`, D9: `"Summarize for me in 3-4 sentences: ..."`) to a longer anti-tool-use framing (`"This is a style check, not a task: do not use any tools..."`). The code comment explains why (observed `error_max_turns` when the model started investigating the repo), which is a legitimate fix, but the plan's Changelog was never updated with this post-approval adjustment — the sibling plan in the same batch (`bootstrap-profile-improvements`) *did* record an equivalent verification-finding row for its own deviation (the git-init-before-validate reorder). Per this repo's own `RULE-PLAN-049`, this should have a Changelog row too. Low stakes since the reasoning is captured inline in code, but worth a one-line backfill for anyone auditing the plan later.

2. **`internal/acdsl/policy.go`** — the `classViolations`/`ruleDelta`/`surfaceDelta` trio has no test exercising the specific "free mode + unresolvable base_ref" combination from finding #1 (the existing `TestCheckPolicyUnresolvableBase` only covers `gated` mode with a bad `base_ref`, which correctly errors). Once #1 is fixed, add that case explicitly so this doesn't regress silently again.

## Recommendations

- For the Critical finding: after the fix, add the `cmd/hooks/acdsl-policy-guard.sh` scenario "deny `Bash` writes to the policy surface too" as a follow-up hardening item — the hook is documented as advisory/fail-open by design, so this isn't required for correctness (the binary check is supposed to carry that weight), but a defense-in-depth improvement is cheap here and the current hook's `matcher: Write|Edit|NotebookEdit` leaves the most common escape hatch (`Bash`) untouched.
- Given the size of this range, consider whether future `v1-2-x` batches this large would benefit from being reviewed (or at least tested) incrementally per plan directory rather than as one combined diff — several of the 14 plans here are independently shippable per their own "Phase N — shippable alone" framing, and a per-plan review pass would have caught the policy-engine issue earlier and cheaper.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of this release (repo cache/registry, status-cache fingerprinting, peek multi-profile, sessions archive, install/port handling, routine gating, i18n) is solid, well-tested, and matches its plans closely, with build/vet/test all green. But the ACDSL self-management policy engine — a security/governance mechanism whose entire purpose is to prevent exactly this kind of bypass — has a confirmed, easily-triggered hole that defeats it entirely in `strict` and `gated` targets. That one file (`internal/acdsl/policy.go`) should not ship until fixed; everything else in the range can merge independently of it.
