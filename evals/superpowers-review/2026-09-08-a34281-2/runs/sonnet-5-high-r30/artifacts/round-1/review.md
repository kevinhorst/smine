# Review: smine v1.1.4 → v1.2.1

Range: `b4981b88`..`a342811b` (178 files, +7314/-749), reviewed against the plans under
`plans/v1-2-1/`. This is a large, multi-feature diff (config server, routines, ACDSL
self-management, railroad-review, sessions/repos UI). I reviewed it in passes: (1) full
`git diff --stat` triage by risk tier, (2) plan-by-plan comparison of each `plans/v1-2-1/*`
change doc against its corresponding code, (3) targeted read of every new/rewritten Go file
over ~100 lines, (4) full build/vet/test/`make audit-full` run on a clean worktree checked out
at the head commit (the review checkout itself had uncommitted local drift — see note below),
(5) spot-read of the new shell hooks and scripts.

I did not line-by-line review every template/HTML diff or every skill `SKILL.md`/changelog
edit — those are Low-tier per ACTION-REVIEW-RISK-003 (docs, prose, generated changelog bumps)
and I sampled rather than exhaustively read them.

**Environment note (not a code defect):** the assigned checkout (`arm-wt`) had an uncommitted
local modification to `acdsl/registry.json` (drift from a `./bin/acdsl project` run without
`-strip`, exactly the working-copy artifact `CLAUDE.md` warns about). This made a plain
`go test ./...` fail one test (`TestDistShipsReachCoveredSliceWithBinaries`) for reasons
unrelated to the diff. I confirmed via `git diff` that `acdsl/registry.json` is byte-identical
between base and head, then verified everything in a fresh `git worktree add` at the head
commit: `go build ./...`, `go vet ./...`, `go test ./...`, `make audit`, and the full
`cmd/tests/test_*.sh` suite (the `audit-full` shell tier) are all green there.

## Strengths

- **`repo-cache-reload` plan executed essentially verbatim.** `internal/repos/registry.go`'s
  read-through registry (stat-guarded reload, read-modify-write `Add`/`Remove`) and
  `internal/server/statuscache.go`'s fingerprint-gated cache match the plan's example code
  almost line-for-line, including the subtle bits: `DropBranches` deliberately keeps the
  pre-removal fingerprint so the next plain load re-scans (D4), and the dead
  `handleRepoRemove` route was actually deleted rather than left to rot. Test coverage
  (`TestRegistryRefreshOnExternalEdit`, `TestRegistryAddPreservesExternalEdit`,
  `TestFingerprint`, etc.) matches the plan's test table and exercises the two real
  regressions (stale second-instance resurrection, cache replaying pre-removal rows).
- **ACDSL policy engine's git-boundary logic is *better* than the plan specified**, not just
  compliant with it. The plan's `CheckPolicy` pseudocode checked `policy.Mode == free` before
  ever resolving the base, which would have let an agent commit `{"mode":"free"}` over the
  real policy file and disable enforcement outright. The implementation instead resolves the
  base first and re-reads the **base ref's** `acdsl/policy.json` (`basePolicy`) before deciding
  the mode — a real, unprompted hardening of the security boundary the feature exists to
  create (see Critical finding below for the one path this doesn't close).
- **Sessions archive feature** (`internal/sessions/sessions.go`) has careful path-traversal
  handling: `validateScopeName` rejects empty/`.`/`..`/embedded separators/the `archived`
  container itself, and correctly special-cases `filepath.Base("..") == ".."` by checking `..`
  explicitly rather than relying solely on the `!= filepath.Base(name)` guard. `DeleteScope`
  only ever operates under `sessions/archived/`, never the live tree.
- **Shell hook style discipline**: every new/touched hook (`acdsl-policy-guard.sh`,
  `skill-invoke-gate.sh`, `cmd/bootstrap/run.sh`) follows the repo's own conventions —
  fail-open on missing `jq`, bash-3.2-safe array guards (`${flags[@]+"${flags[@]}"}`), and
  (for `run.sh`, which doesn't use `set -e`) no expected-failure `&&`-chains that would need
  the `if`-guard. `test_skill_invoke_gate.sh` has good behavioral coverage (pre-invocation
  deny, wrong-skill-doesn't-satisfy, slash-prompt-counts, post-invocation context-read deny,
  language-guide exception, kill switch).
- **Full test/gate suite is green** on the actual head commit: `go build`, `go vet`, `go test
  ./...` (all packages), `make audit` (build-audit, mod verify, vet, `acdsl project -strip`,
  `acdsl check`, `acdsl fixtures`, tests), and every `cmd/tests/test_*.sh` shell test.
- Plan-vs-code traceability is generally good: decisions like C6/C19 (forced additional-dir,
  git-init-on-add) in `change-nondev-automation.md` map cleanly onto the corresponding
  `handleReposAdd` diff, and the repos-routines-improvements plan's `WorktreeKindCount`
  bucketed counts match `internal/repos/registry.go`'s `WorktreeCounts` rewrite.

## Issues

### Critical (Must Fix)

- **`internal/acdsl/policy.go:329-336` (`CheckPolicy`) — the gated/strict enforcement can be
  silently disabled with a single working-tree edit to `acdsl/policy.json`, contradicting the
  feature's own documented guarantee.**

  ```go
  func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
      base, isBoundary, baseErr := policyBase(ctx, root, policy)
      if baseErr != nil {
          if policy.Mode == PolicyModeFree {
              return nil, nil // no base to gate against, nothing declared — free
          }
          return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
      }
      ...
  ```

  `policy` here is the **working-tree** copy (`acdsl.LoadPolicy(*root)` in
  `cmd/acdsl/main.go:runCheck`) — exactly the file an agent on a `claude/*` branch controls.
  `policyBase` (`policy.go:195-219`) uses `policy.BaseRef` when non-empty as the *only*
  resolution candidate (no fallback to `origin/HEAD`/`main`/`master`):

  ```go
  candidates := []string{"origin/HEAD", "origin/main", "main", "master"}
  if policy.BaseRef != "" {
      candidates = []string{policy.BaseRef}
  }
  ```

  So an agent can write `acdsl/policy.json` to
  `{"mode": "free", "base_ref": "does-not-exist"}` in one edit. `policyBase` then fails to
  resolve (single bad candidate, no fallback), and because `baseErr != nil` is checked against
  the *tampered working* `policy.Mode` (not the base's real, gated/strict mode), `CheckPolicy`
  returns `nil, nil` — clean, no diagnostics — instead of the hard error the design promises.
  This never reaches `surfaceDelta` (which would have caught the edit to `policy.json` itself),
  because `surfaceDelta` only runs after base resolution succeeds.

  This directly contradicts `acdsl/README.md:66-67`: *"An unresolvable base in strict/gated is
  a hard tool error, never a silent free."* It is also a one-file bypass of the
  `acdsl-policy-guard.sh` PreToolUse hook: that hook only intercepts `Write|Edit|NotebookEdit`
  tool calls, not `Bash` (`echo '...' > acdsl/policy.json`), so a determined agent has a
  trivial path around both enforcement layers described in the plan's D2/D7 ("authoritative
  check inside `acdsl check`" + "fast feedback hook").

  No test exercises this combination — `TestCheckPolicyUnresolvableBase`
  (`policy_test.go:280`) only covers an unresolvable base ref under an otherwise-gated working
  policy, which correctly errors; it never combines a bad `base_ref` with `mode: "free"` in the
  same working file.

  **Fix:** don't let the free/error branch on `baseErr != nil` trust the working policy's
  `Mode`. Two independent hardenings, either alone closes the hole:
  1. In `policyBase`, always include the default candidate list as a fallback even when
     `policy.BaseRef` is set (try the explicit ref first, but don't let a bad override make the
     base fully unresolvable) — only a truly base-less repo (no `origin/HEAD`, `main`, or
     `master` at all) should ever hit the unresolvable-base path.
  2. Distinguish "no `acdsl/policy.json` on disk" (the legitimate bootstrap/free case) from
     "a `policy.json` exists and merely *claims* free" — only the former should short-circuit
     on `baseErr`. A file that exists and can't be checked against a real base should always be
     the hard error the docs promise, regardless of what mode it claims.

### Important (Should Fix)

- **`cmd/hooks/skill-invoke-gate.sh` (new, 85 lines) + `cmd/tests/test_skill_invoke_gate.sh`
  (new, 138 lines) have no corresponding plan entry under `plans/v1-2-1/`.** This is a new
  PreToolUse enforcement mechanism (mandatory `Skill(<name>)` invocation before any other tool
  call, plus post-invocation denial of direct reads under the context directory) — the kind of
  concept ACTION-REVIEW-SPEC-005 asks to trace back to a requirement. The only mention
  anywhere under `plans/v1-2-1/` is a passing reference in
  `2026-08-26-railroad-review-observability-wip-5158764/runbooks/trace.md` ("As of
  railroad-review 8.4 a clean lane reads NO context-entry file directly — the skill-invoke-gate
  hook denies it") — describing it as *already existing* prior behavior, not introducing it,
  and with no Decision/Change entry anywhere explaining why a Bash-blind, Write/Edit-only
  enforcement hook is the right shape, or covering its interaction with the ACDSL policy
  hook's own file-surface denials. The corresponding plan doc (`raw.md`'s own Changelog) stops
  at the `--no-context` restoration + branch-rename decisions (D1/D6) — nothing in it anticipates
  a v8.4 with a forced-invocation gate. Given this is the second security-relevant PreToolUse
  hook shipped in this diff, and the reviewer has no way to check its design against an approved
  decision, please confirm this was deliberately scoped (perhaps under a plan outside
  `plans/v1-2-1/`, e.g. tied to the skillroutine-eval single-pass fanout work referenced in
  recent commits) rather than added ad hoc.

- **`internal/acdsl/dist.go` / repo self-hosting inconsistency risk (adjacent to the Critical
  finding, lower confidence):** `writeDistRegistry` rewrites `go run ./cmd/acdsl/verifiers/X`
  argv to `bin/verifiers/X` only when the shape matches `verifierArgvRe`
  (`^\./cmd/acdsl/verifiers/[a-z0-9]+$`). This is unrelated to the policy bug but worth noting
  while in this file: `buildBinaries` always **rebuilds** verifier binaries via `go build`
  unless a prebuilt exists at `srcDir` — for the smine repo's own `Dist` calls (used by
  `TestDistShipsReachCoveredSliceWithBinaries` and any operator running `acdsl dist` against a
  scratch target) this means every dist invocation pays a full `go build` per verifier with no
  caching; not a correctness bug, just worth flagging if `Dist` ever runs on a hot path (it
  currently doesn't — it's a manual/CI-adjacent operation).

### Minor (Nice to Have)

- **`cmd/hooks/skill-invoke-gate.sh`'s `Bash` command matching is narrow enough that it mostly
  documents intent rather than enforcing it.** The context-dir read denial only fires for an
  *exact* `cat <path>` invocation (`[ "$#" -eq 2 ] && [ "$1" = "cat" ]`); `head`, `less`,
  `sed -n`, `python3 -c`, or `cat` combined with any shell metacharacter all sail through
  untouched (the metacharacter check `case "$command" in *";"*|...` disables the whole `cat`
  check the moment a `$`, backtick, quote, or paren appears anywhere in the command, even in
  an unrelated part of a longer pipeline). Given the hook's own doc comment says "fail-open by
  design" I'm not flagging this as the Critical/Important bypass class, but it's worth being
  explicit in the hook's comment that this is advisory-only against `Bash`, not a real
  boundary, so nobody later treats it as one when reasoning about the policy engine's
  authoritative/advisory split.
- **`internal/server/repos.go:handleReposAdd`** now silently no-ops the `git init` step when
  the path exists but the `os.Stat` for `.git` doesn't exclusively check "not a repo" vs. "some
  other stat error" (`if info, statErr := os.Stat(path); statErr == nil && info.IsDir() { if _,
  gitErr := os.Stat(filepath.Join(path, ".git")); gitErr != nil { ... git init ... } }`) — a
  transient stat error on an existing `.git` (permissions blip, e.g. an anti-virus lock on
  Windows) would trigger a redundant `git init -b main` on an already-initialized repo. `git
  init` on an existing repo is a documented no-op (it doesn't touch history), so this degrades
  gracefully, but it's worth a one-line comment noting the intentional `gitErr != nil` treats
  "any stat failure" as "needs init," not just `os.IsNotExist`.
- Several new Go files (`profilesettings.go`, `bootstrap.go`, `statuscache.go`) are clean and
  idiomatic; I have no findings against RULE-GOLANG-* there beyond what's called out above.

## Recommendations

- Add the missing test case for the Critical finding
  (`TestCheckPolicyFreeWorkingWithUnresolvableBase` or similar: working policy `{"mode":
  "free", "base_ref": "bogus"}` over a base with `mode: "gated"` → expect a hard error, not
  `nil, nil`) as part of the fix, so the regression can't silently return.
- Since `acdsl/policy.json` is itself a "self-management surface" file per `policySurface`,
  consider whether `policyBase`'s bootstrap read of the working copy's `BaseRef` should happen
  at all — an alternative, simpler design is to hardcode the default candidate list
  (`origin/HEAD`, `origin/main`, `main`, `master`) for the *first* resolution pass always, and
  only honor a base-ref override once it's been read from the confirmed base's own
  `policy.json` (i.e., `base_ref` becomes a base-authored setting, never a working-tree one).
  That removes the untrusted-input dependency entirely rather than patching around it.
- File a short design note (even a one-paragraph addendum to the railroad-review-observability
  plan, or a new plan doc) for `skill-invoke-gate.sh` so the mandatory-invocation + read-denial
  mechanism has the same paper trail as the ACDSL policy hook. Given how much this diff invests
  in "one enforcement truth, one convenience layer" language for the ACDSL policy feature, the
  second hook deserves the same rigor documented somewhere reviewable.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of this release (repo cache/registry rework, sessions archive,
profile/bootstrap UI, skill-visibility sync) is well-executed, matches its plans closely, is
fully covered by a green `make audit-full`, and shows genuine care in the hard parts (path
traversal, concurrency, atomic file writes). But the ACDSL self-management policy feature —
the one explicitly built to "structurally close" the loosy-goosy-rules failure mode — has a
real, single-file-edit bypass that defeats both its enforcement layers and contradicts its own
README's stated guarantee. That's the feature's whole reason for existing, so it should not
ship gated/strict-mode-enforcing until the `policyBase`/`CheckPolicy` fallback path is fixed
and covered by a test. Everything else in this diff can merge independently of that fix.
