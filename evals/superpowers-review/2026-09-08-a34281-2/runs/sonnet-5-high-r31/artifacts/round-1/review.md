# Review: smine 1.1.4 → 1.2.1 (b4981b88..a342811b)

Scope: 178 files, +7314/-749, excluding sessions/, evals/, proposals/, plans/. Reviewed in
several passes: (1) full `git diff --stat` triage by risk tier, (2) deep read of the highest-risk
new subsystems against their plan docs under `plans/v1-2-1/` (repo-cache-reload, acdsl-policy-modes,
peek multi-profile, repos/routines UI, bootstrap/profile), (3) `go build`/`go vet`/`go test ./...`
on the actual diff range plus a control run at the base commit to isolate a suspicious test
failure, (4) targeted reads of install.sh, hooks, routines/smine-nightly/run.sh, and the config
sync/i18n paths. I did not do a line-by-line read of every skill Markdown file, every HTML
template, or the Windows-only install code paths (`install_windows.go`, `sched_windows.go`) —
those were sampled, not exhaustively verified.

Build/vet: clean (`go build ./...`, `go vet ./...` both silent). `go test ./...`: one failure,
`internal/acdsl.TestDistShipsReachCoveredSliceWithBinaries`. I chased this down with a control
worktree at both the head and base commit: the failure does **not** reproduce in a clean clone of
the head commit, only in this checkout, whose `acdsl/registry.json`/`acdsl/rules.acdsl` are locally
dirty (already-projected/unstripped working-tree state, unrelated to the reviewed range — visible
via `git status`). Confirmed via `git show <sha>:acdsl/registry.json` that the committed content is
byte-identical at base and head. **Not a real regression** — noted here only so it isn't rediscovered
and misattributed.

## Strengths

- **Repo registry read-through + status-cache fingerprinting** (`internal/repos/registry.go`,
  `internal/server/statuscache.go`, `internal/server/repos.go`) is a clean, precise implementation
  of its plan (`plans/v1-2-1/2026-08-30-repo-cache-reload-wip-46c8d9e`). The stat-before-read
  ordering, read-modify-write on `Add`/`Remove`, and the git-fingerprint cache invalidation all
  match the plan's pseudocode almost verbatim, and the test list (`TestRegistryRefreshOnExternalEdit`,
  `TestRegistryAddPreservesExternalEdit`, `TestFingerprint`, etc.) matches the plan's test table.
  Good use of "degrade to re-scan on error, never to stale" as the uniform failure posture.
- **Peek session attribution by branch** (`internal/peek/peek.go`, `internal/server/repos.go`
  `worktreeSessions`) correctly implements the FACT-REPO-ARCH-017 fix — branch identity wins over
  cwd, with cwd as a detached-HEAD fallback — with good test coverage of the HEAD/branchless
  exclusion cases.
- **launchd default-disabled override** (`internal/routines/bootstrap_darwin.go`) cleanly
  generalizes `disabledLabels`→`overrideLabels` to carry both states, and gates a
  `default-disabled` routine behind an explicit `enabled` override rather than an absence check —
  correct and well tested (`TestOverrideLabels`).
- **Multi-profile peek port collision handling** (`cmd/configserver/main.go` `startPeek`/
  `peekIdentity`) is a real improvement over the old "port is listening, assume it's mine" check —
  it now verifies identity via `/healthz` before reusing a listener, and refuses (rather than
  silently misattributing sessions) on mismatch or an unidentifiable holder.
- The ACDSL policy engine (`internal/acdsl/policy.go`) gets the *big* thing right that its own
  plan pseudocode got wrong: the plan's `CheckPolicy` pseudocode short-circuited on
  `policy.Mode == PolicyModeFree` using the **working-tree** policy before ever resolving the base;
  the shipped code instead resolves the base first and re-derives the authoritative policy from
  `git show <base>:acdsl/policy.json` (`basePolicy`, policy.go:337) before trusting `Mode`. That's a
  correct, non-obvious deviation from the plan that closes the most obvious escape ("just edit
  mode to free"). See Critical #1 for the one path this still leaves open.
- `handleReposAdd`'s git-init-on-add and forced grant for casual audiences match
  `plans/v1-2-1/2026-08-24-language-style-setting-wip-ef09567/design/change-nondev-automation.md`
  decisions C19/C6 precisely (init before `registry.Add`, init failure aborts the add).
- The sessions-store "rescan on every GET, no Reload button" change
  (`internal/server/sessions.go`) is a deliberate, user-approved decision (D17 in
  `plans/v1-2-1/2026-08-27-bootstrap-profile-improvements-wip-397f906/design/change-bootstrap-profile.md`)
  with a stated rationale ("cheap local read"), not an unreviewed regression — see Minor #1 for the
  one thing worth watching as the corpus grows.

## Issues

### Critical (Must Fix)

#### 1. `acdsl check` policy gate is silently bypassable in gated/strict repos via `base_ref` + `mode` tampering

`internal/acdsl/policy.go:329-336` (`CheckPolicy`), in conjunction with `policy.go:195-218`
(`policyBase`).

**What's wrong:** `CheckPolicy` resolves the policy base with `policyBase(ctx, root, policy)`,
where `policy` is the **working-tree** copy loaded by `LoadPolicy` (`cmd/acdsl/main.go:122`) — i.e.
whatever is currently checked out on the agent's own branch, fully attacker-writable. If
`policyBase` fails to resolve a base (which happens whenever `policy.BaseRef` is set to any
non-resolving ref — note `policyBase` uses **only** that single candidate when `BaseRef != ""`,
with no fallback to the default candidate chain), `CheckPolicy` falls back to:

```go
if baseErr != nil {
    if policy.Mode == PolicyModeFree {
        return nil, nil // no base to gate against, nothing declared — free
    }
    return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
}
```

`policy.Mode` here is still the **untrusted working-tree value** — `basePolicy` (which re-derives
the authoritative mode from the base ref) is only called *after* this branch, on the success path.
So an agent in a `gated` or `strict` target repo can neutralize the entire self-management gate
with a two-field edit to `acdsl/policy.json` on its own branch:

```json
{"mode": "free", "base_ref": "does-not-exist"}
```

Then any subsequent `acdsl check`: `policyBase` fails (its one candidate, `"does-not-exist"`,
doesn't resolve) → `baseErr != nil` → `policy.Mode == PolicyModeFree` (attacker-set) → `return nil,
nil`. No diagnostics, no error, exit 0. `surfaceDelta` — which is the one thing that would have
caught this exact tampering of `acdsl/policy.json` itself — never runs, because the function
returns before reaching it. The agent can now add/remove/modify any rule, add validators via
`registry.local.json`, or otherwise fully undo the gate this feature exists to provide.

**Why it matters:** this defeats the plan's explicit, stated purpose (`plans/v1-2-1/2026-08-26-
acdsl-policy-modes-done-a7cd7bf/design/raw.md` D4: "an agent branch can never self-escalate";
D10: "an unresolvable base in strict/gated is a hard tool error, never a silent pass") and the
README's claim ("enforced by `acdsl check`... so no data file can neuter it"). The PreToolUse hook
(`cmd/hooks/acdsl-policy-guard.sh`) is explicitly documented as fail-open/advisory and bypassable
via Bash — the binary check is supposed to be the actual backstop. This bug removes that backstop
for exactly the threat model the feature was built for. This is high-risk-tier code
(ACTION-REVIEW-RISK-001: self-management/auth-adjacent gating logic) with no compensating control.
It is not tested: `TestCheckPolicyUnresolvableBase` (policy_test.go:280) exercises the unresolvable-
base path only with `gatedPolicy()` (mode `gated`), never with mode `free` — the exact combination
that trips the bypass is absent from the 16+ case table the plan otherwise covers thoroughly.

**How to fix:** the free-mode shortcut inside the `baseErr` branch is the bug — drop it and always
propagate the error when the base is unresolvable, exactly as D10 states ("never a silent pass"):

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
    base, isBoundary, baseErr := policyBase(ctx, root, policy)
    if baseErr != nil {
        return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
    }
    ...
```

If the intent behind the shortcut was to let a genuinely-free repo (e.g. smine's own dev clones)
survive a missing remote without a hard failure, that decision needs to be based on the
**base-derived** mode, not the working copy's — which is only knowable once the base resolves, so
the honest fix is simply: no base, no bypass, always error in every mode when a base can't be
found. Separately, consider making `policyBase` fall through to the default candidate chain
(`origin/HEAD`, `origin/main`, `main`, `master`) even when `BaseRef` is set but fails to resolve,
rather than treating an explicit `base_ref` as the sole candidate — that would make a garbage
`base_ref` alone (without needing mode tampering too) fail closed onto a real base instead of
failing open. Add the missing test case: `mode: free` + unresolvable `base_ref` on a repo whose
*base* is actually `gated` — must still error or, if resolved via the base, still enforce.

### Important (Should Fix)

None beyond the above — the rest of the diff I could verify (registry/cache, peek attribution,
routines scheduling, install multi-profile, profile settings, dist.go, git.go) is solid, plan-
conformant, and adequately tested.

### Minor (Nice to Have)

1. **Sessions store now rescans on every GET, with no cache tier.**
   `internal/server/sessions.go` (`reloadSessions`, called from `handleSessionsIndex`,
   `handleSessionsScope`, `handleSessionsBatch`, `handleSessionsArchiveTab`) →
   `internal/sessions/sessions.go:131` `Store.Reload()`, which does a full `os.ReadDir` +
   `os.ReadFile` + `json.Unmarshal` pass over every batch JSON file in every scope, every time. This
   is a deliberate, user-approved decision (D17, see Strengths) that trades a fresh-install bug for
   full rescans, and the plan calls it "a cheap local read" — reasonable today. Contrast this with
   the same release's repo status cache, which got a proper git-fingerprint invalidation scheme
   specifically to avoid full rescans on every load (`internal/repos/registry.go` `Fingerprint`).
   If the batch corpus grows into the hundreds, this will start to show as page-load latency with
   no cache to fall back on, and there's no test asserting a cost bound. Not a fix-now item since
   it's an explicit, already-adjudicated tradeoff — flagging so it's on the radar for the next pass
   if session-report pages start feeling slow.

2. **`peekIdentity`'s error message can misattribute an unrelated process as "another profile's
   peek".** `cmd/configserver/main.go` `startPeek`: if something other than peek-mcp is listening
   on the configured port but happens to serve a `200` on `/healthz` with JSON that doesn't error on
   decode (e.g. an unrelated dev server with a permissive JSON error handler), `peekIdentity`
   returns a zero-value `*peekID` with no error, and the mismatch branch logs "another profile's
   peek holds this port" — which is not necessarily true; it could be any process. Low-severity
   (behavior — refusing to reuse the port — is correct either way, only the log wording could
   mislead an operator debugging a port conflict).

3. **`install.sh`'s peek-port kill loop assumes one PID per `lsof -t` result.**
   `pid="$(lsof -tiTCP:"$peek_port" -sTCP:LISTEN || true)"` followed by `kill "$pid"` — if two
   processes share the listening socket (SO_REUSEPORT, or a lingering fork), `lsof -t` returns
   multiple newline-separated PIDs, and `kill "$pid"` gets one argument containing all of them,
   which fails as a `kill` invocation (not a valid single PID). Very unlikely in practice for a
   single-owner service port; worth a `kill $pid` (unquoted, word-split) or a loop if it's ever
   observed to matter.

## Recommendations

- Add the `mode: free` + unresolvable-`base_ref` test case to `policy_test.go`'s table before
  landing the Critical fix, so the regression can't come back silently.
- Since the ACDSL policy feature's whole value proposition is "the binary check can't be neutered
  by a data file," it would be worth a short fuzzing-style pass over `CheckPolicy`'s error/early-
  return branches specifically looking for other "trust the working copy before the base resolves"
  patterns — the one in Critical #1 was subtle enough to survive a fairly thorough plan and test
  suite.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of this release (repo cache/registry, peek attribution, routine scheduling,
multi-profile install/peek, profile settings, session archiving) is well-executed, plan-conformant,
and well-tested — builds and vets clean, and the one test failure I found is an environmental
artifact, not a regression. But the ACDSL self-management policy feature — a brand-new security-
relevant gate whose sole purpose is preventing agent self-escalation — has a confirmed, reproducible
bypass in `CheckPolicy`'s unresolvable-base fallback that defeats it for exactly the threat model it
was built for. That one fix (Critical #1) should land before this ships to any repo that will
actually run in `gated` or `strict` mode; everything else in the diff can merge as-is.
