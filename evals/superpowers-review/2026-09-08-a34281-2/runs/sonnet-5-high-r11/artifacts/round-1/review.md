# Review: smine v1.1.4 → v1.2.1 (b4981b88..a342811b)

Reviewed in passes: (1) `internal/acdsl` policy engine end-to-end with a standalone
exploit test against a clean worktree of the head commit; (2) `internal/repos`,
`internal/routines`, `internal/server` (repos/status-cache/profile/peek) diffs
line-by-line; (3) new hook scripts and `settings.json`; (4) `routines/smine-nightly/run.sh`;
(5) a lighter pass over the skill/doc/changelog churn (`skills/**`, `context/**`).
Build, `go vet`, and `go test ./...` were run against a **clean worktree checked out
directly at the head commit** (see Note on environment below) — all green.

## Strengths

- `internal/repos/registry.go`: `Add`/`Remove` now re-read the registry file fresh
  under the lock before mutating (`readRegistrySnapshot`), closing the documented
  last-writer-wins resurrection bug (FACT-REPO-ARCH-016) without adding IPC —
  exactly the fix the fact prescribes. `refreshIfChanged` correctly stats before
  reading so a write racing a stat is caught on the *next* stat, not missed.
- `internal/peek/peek.go` (`SessionIndex`, `ByBranch`/`ByCwd`) and its consumer
  `worktreeSessions` in `internal/server/repos.go` implement FACT-REPO-ARCH-017
  precisely: branch identity is tried before the cwd fallback, so a pool-recycled
  worktree directory no longer shows its previous occupant's session.
  `internal/routines/bootstrap_darwin.go`'s `overrideLabels` (was `disabledLabels`)
  now distinguishes explicit `enabled` from `disabled`, which is what
  `DefaultDisabled` routines need to require an explicit opt-in — clean, well-tested
  (`launchctl_darwin_test.go` covers the new states, including "unknown state ignored").
- `internal/server/statuscache.go` + the `Fingerprint`/`WorktreeCounts` split in
  `registry.go`: a genuinely new, well-scoped cache keyed on a cheap two-git-call
  fingerprint rather than a TTL, with an explicit "stored slices are immutable"
  contract and a `DropBranches` optimistic-removal path that deliberately keeps the
  stale fingerprint so the next plain load re-scans. Matches FACT-REPO-ARCH-016's
  closing note almost verbatim.
- Struct field ordering (`RULE-GOLANG-STRUCT-002`) is meticulously alphabetical
  across every new/changed struct I checked (`Repo`, `Policy`, `WorktreeKindCount`,
  `statusEntry`, `profilePage`, `profileTestView`, `styleTestTarget`, `Session`) —
  no violations found.
- `internal/acdsl/policy.go` file organization is exemplary `RULE-GOLANG-FILE-001`:
  unexported package functions alphabetical, exported functions (`CheckPolicy`,
  `LoadPolicy`) alphabetical and last.
- `handleReposAdd`'s new inline `git init` validates the repo (`repo.Validate()`)
  *before* the op closure runs, with a comment explicitly stating why ("its git init
  must never fire for a repo the registry would reject anyway") — good defensive
  sequencing.
- Shell-injection hygiene in `profilesettings.go`'s style-test call: the prompt and
  model are passed as `bash -c script bash "$1" "$2" "$3"` positional arguments,
  never string-interpolated into the script — correct even though the values here
  happen to be trusted.

## Issues

### Critical (Must Fix)

- **`internal/acdsl/policy.go:329-336` (`CheckPolicy`) — the self-management gate
  is fully bypassable by an agent with only working-tree write access, defeating
  the feature's entire stated purpose.**
  ```go
  base, isBoundary, baseErr := policyBase(ctx, root, policy)
  if baseErr != nil {
      if policy.Mode == PolicyModeFree {
          return nil, nil // no base to gate against, nothing declared — free
      }
      return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
  }
  policy, err := basePolicy(ctx, root, base, policy)   // base-ref policy becomes authoritative — but only reached if baseErr == nil
  ```
  The whole point of this feature (TLDR: "the loosy-goosy-rules failure mode is
  structurally closed") and D4/D10 in the plan is that the **base-ref** policy is
  authoritative and a working-tree edit cannot weaken it, with an unresolvable base
  in strict/gated mode being "a hard tool error... never a silent pass." But the
  decision of whether an unresolvable base is fatal is made using `policy.Mode`
  **from the untrusted, attacker-editable local file** (the argument passed in,
  which is `LoadPolicy(root)` read straight from the working tree in
  `cmd/acdsl/main.go`). `basePolicy` — the call that would override a forged local
  `mode` with the real base-branch mode — is never reached when base resolution
  fails.

  Concretely: on a `gated` repo, an agent branch can edit `acdsl/policy.json` to
  `{"mode": "free", "base_ref": "no-such-ref"}` and add/remove/change any protected
  rule in the same commit/working-tree state. `policyBase` then fails to resolve
  `base_ref` (it's the only candidate once set), `CheckPolicy` sees `baseErr != nil`
  and `policy.Mode == PolicyModeFree` (from the forged file) and returns `(nil, nil)`
  — **zero diagnostics, no error, clean check** — even though the same policy.json
  edit is itself a `policySurface` violation that would have been caught by
  `surfaceDelta` had `basePolicy` ever run.

  Verified with a standalone test against a **clean worktree of the head commit**
  (not the review checkout, which has unrelated local state — see note below):
  ```go
  func TestCheckPolicyExploit_FreeModeWithBadBaseRefBypassesGate(t *testing.T) {
      root := newPolicyRepo(t, gatedBasePolicy) // base: gated, GOLANG editable
      writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n"+
          `//acdsl:ACDSL-PLAN-999 gofmt anchor="^plans/" why="agent forged addition"`+"\n")
      forged := Policy{AllowTaskContracts: true, BaseRef: "totally-bogus-ref-xyz", Mode: PolicyModeFree}
      diagnostics, err := CheckPolicy(context.Background(), root, forged)
      require.NoError(t, err)
      // diagnostics == [] — the protected-scope PLAN-999 addition passes silently
  }
  ```
  Result: `diagnostics: []`, `err: nil` — exploit confirmed. `policy_test.go`'s
  `TestCheckPolicyUnresolvableBase` only exercises an unresolvable base while
  `policy.Mode` is honestly `gated`, so it never exercises this path; there is no
  test for "unresolvable base + locally-declared free" and the hole is un-caught.

  **Fix:** the free-mode short-circuit on `baseErr != nil` must not trust the local
  file at all. Either (a) always hard-fail when base resolution fails and the
  *default* candidate chain (not a possibly-forged custom `base_ref`) also can't
  resolve, regardless of the local `policy.Mode`, treating "genuinely no base
  exists" as distinct from "a custom base_ref was supplied and failed"; or (b) when
  `policy.BaseRef` is set, first attempt the default candidate chain as a
  cross-check before trusting failure, and only report free/bootstrapping when no
  candidate — default or custom — resolves. Either way, `policy.Mode` read from the
  working tree must never be the sole gate for whether a failed base resolution is
  fatal.

### Important (Should Fix)

- **`settings/claude_code/settings.json` — undocumented removal of the manual `ask`
  gate on ACDSL rule file edits, not called for by the plan.** Commit `5c11486`
  removes `Edit(**/*.acdsl)`, `Edit(acdsl/**)`, `Write(**/*.acdsl)`,
  `Write(acdsl/**)` from `permissions.ask` in the same change that adds the new
  `acdsl-policy-guard.sh` hook. The acdsl-policy-modes plan's Changes §8 only
  specifies *adding* the hook entry; it says nothing about removing the pre-existing
  manual confirmation. `permissions.ask` is a global (cross-repo) Claude Code
  setting, so this removes the "ask before touching ACDSL rule files" friction for
  **every** repo Claude works in, not just `smine` (which is `free` mode and where
  the removal is arguably fine). The new hook only fires on `Write|Edit|NotebookEdit`
  (not `Bash`, by the plan's own D2 design — the binary check is meant to be
  authoritative there), and the binary check itself has the Critical bypass above.
  With the `ask` gate gone, a gated/strict *target* repo now has no human-in-the-loop
  step at all standing between an agent and a rule-file edit — only a fail-open hook
  and a checker that can be routed around. Confirm this removal was intentional
  (and, if so, document it in the plan's Changelog per RULE-PLAN-049 — post-approval
  plan edits are supposed to be recorded there) rather than an unreviewed side effect
  of the settings.json diff.
- **`internal/acdsl/git.go` — `GitRevParse` is dead production code.** It matches the
  plan's git-helpers contract (`Changes §2`) but nothing in `policy.go` calls it —
  `policyBase`/`basePolicy` use `GitMergeBase`/`GitShowFile` directly. It's exercised
  only by its own test (`git_test.go:13`). Per ACTION-IMPL-ARCH-001 ("every new
  concept traces back to the request... otherwise remove it"), either wire it in
  where the plan intended it (there's no obvious call site left) or drop it.
- **`cmd/hooks/skill-invoke-gate.sh:65-73` — the `Bash cat` post-invocation guard is
  trivially defeated by quoting.** The command-shape check excludes any command
  containing a `"` or `'` character from inspection at all (the `case` pattern's
  first branch matches on those chars and does nothing), so `cat "context/rules/go.md"`
  bypasses the "context entries are injected, don't read them directly" guard that
  `cat context/rules/go.md` (unquoted) would trip. Given the hook's own stated
  posture ("fail-open... on absent marker") this is consistent with the house style
  of loose/advisory hooks rather than a security boundary, but it does mean the
  guard's real coverage is narrower than the comment on line 6-7 implies — a reader
  fixing the read-gate policy in `read-gate.sh` (which uses the same `exact_cat_target`-
  style pattern elsewhere) might reasonably assume this one has equivalent coverage.
  Worth a one-line comment noting the quoting gap, at minimum.

### Minor (Nice to Have)

- **`internal/server/statuscache.go`** — type-block order: `statusEntry` is declared
  before `statusCache`, but alphabetically `statusCache` < `statusEntry`
  (RULE-GOLANG-FILE-001 orders type blocks alphabetically by name). Trivial swap.
- **`internal/acdsl/dist.go:127` (`writePolicy`)** — the function reads
  `LoadPolicy(root)` and then separately `os.Stat`s `acdsl/policy.json` to decide
  whether to ship. If `LoadPolicy` returns an error that isn't `os.ErrNotExist`
  (e.g. malformed JSON on a dev machine mid-edit), `writePolicy` returns that error
  and `Dist` aborts entirely rather than degrading — this matches "fail loud on a
  malformed policy" elsewhere in the file, but it's worth confirming `Dist` callers
  (the `smine dist` CLI path) surface this clearly rather than looking like an
  unrelated dist failure.
- `internal/routines/routines.go`'s `RunResult.IsRunning`/History "started"-row
  filtering is correct but only asserted implicitly through downstream template
  behavior in this diff — a direct unit test for `History` with an interleaved
  started/finished/started sequence would pin the "only the newest survives"
  contract explicitly (the current tests I found cover parsing but I did not find
  a dedicated multi-row `started`-supersession case).

## Note on environment (not a code finding)

The assigned review checkout (`arm-wt`) carries local, uncommitted modifications
layered on top of `a342811b` (context docs, `CLAUDE.md`, and notably a *modified*
`acdsl/registry.json` whose `gofmt` entry is already rewritten to
`bin/verifiers/gofmt` instead of the committed `go run ./cmd/acdsl/verifiers/gofmtwrap`
shape) — this is the eval harness's own context-injection scaffolding, not part of
the reviewed commit range. Running `go test ./internal/acdsl/...` **in that checkout**
fails `TestDistShipsReachCoveredSliceWithBinaries` because of this local drift. I
verified via a separate clean `git worktree add` at the exact head commit that the
full test suite (`go test ./...`), `go build ./...`, and `go vet ./...` are green —
this is not a regression in the reviewed diff. Flagging so it isn't mistaken for one
in a later pass.

## Recommendations

- Add the missing `TestCheckPolicyUnresolvableBase`-adjacent case
  (`fail-forged-free-mode-with-unresolvable-base`) once the Critical fix lands, so
  the exploit path stays closed under regression.
- Given `CheckPolicy` is the sole authoritative enforcement layer (by design, per
  D2), it deserves the same fixture-driven adversarial test posture as a security
  boundary elsewhere in the codebase (`ACTION-IMPL-INTEG-011`-style "forge and
  verify against the real consumer" thinking) — the current test suite is
  extensive on the *cooperative* paths (added/changed/removed rule classification,
  overlay, allowlist, task exemption) but thin on paths where the working tree
  itself lies about its own policy.
- The `settings.json` `ask`-removal deserves a one-line Changelog entry in the
  acdsl-policy-modes plan (`plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/`)
  per RULE-PLAN-049, or an explicit call-out that it was folded in from an unrelated
  change (`5c11486` also touches the presentation-profile settings, suggesting the
  `ask` removal may have been an incidental cleanup rather than considered policy).

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of this release (registry race fix, routine default-disabled
gating, session attribution, status caching, profile settings, dist/policy shipping)
is well-executed, well-tested, and matches its plans closely. But the acdsl
self-management policy engine — the feature whose entire purpose is to stop an
agent from weakening its own governance — has a working-tree-only bypass that fully
defeats it with a two-field edit and zero diagnostics, verified with a runnable
exploit test. That must be fixed (and covered by a regression test) before this
ships to any repo that turns on `gated` or `strict` mode; everything else in the
diff can land as-is.
