# Review: smine 1.1.4 → 1.2.1 (config server, routines, railroad-review, ACDSL)

Range reviewed: `b4981b88..a342811b` (178 files, +7314/-749), excluding `sessions/`, `evals/`, `proposals/`, `plans/`. Plans: `plans/v1-2-1/`.

**Method note:** this diff is too large for a single close pass. I reviewed every file at the diff-stat level, read every plan under `plans/v1-2-1/`, and did close code-level review (with live verification, not just reading) of the highest-risk surfaces per the risk tiering in the review doctrine: the new ACDSL self-management policy engine (security-relevant, new), the repo registry read-modify-write rework (persisted-data correctness), the worktree status cache fingerprinting (correctness), the new skill-invoke-gate hook (eval/review-harness integrity — the same mechanism powering this eval), and the settings.json/hook wiring around the policy feature. Lower-risk surfaces (templates, i18n strings, changelog.json bumps, most SKILL.md prose, the smine-orchestrate/smine-bootstrap skill prose) were read but not exhaustively traced.

I built and ran the full test suite against a **clean worktree at the head commit** (not this eval's `arm-wt`, which carries pre-existing uncommitted local modifications from the harness setup — see Testing below) and confirmed one finding by writing and running a throwaway Go test in an isolated `git worktree`, and by piping synthetic hook input directly into the shipped hook script. Nothing was written to the reviewed checkout; temporary worktrees used for verification were removed afterward.

## Strengths

- **Registry read-modify-write rework** (`internal/repos/registry.go`) is a clean, correct fix for the stale-in-memory-slice resurrection bug (FACT-REPO-ARCH-016): `Add`/`Remove` now re-read the file under the write lock before applying their delta, `refreshIfChanged` stat-gates reloads on plain reads, and `save` records the post-write file identity so a self-write never triggers a spurious reload. Test coverage (`TestRegistryAddPreservesExternalEdit`, `TestRegistryRemoveAfterExternalEdit`, `TestRegistryDeleteStaysDeletedAcrossInstances`) directly targets the regression this was built to fix.
- **Status-cache fingerprinting** (`repos.Fingerprint`, `statusCache.Fingerprint`) is a well-reasoned, cheap (~20ms) invalidation signal that correctly composes with the kept optimistic-delete UX (`DropBranches` deliberately keeps the pre-removal fingerprint so the next plain load re-scans and converges to truth) — the plan's DR1–DR4 chain is implemented faithfully.
- **Session-attribution fix** (`worktreeSessions` in `internal/server/repos.go`) correctly implements FACT-REPO-ARCH-017: branch-identity match first, cwd fallback second, closing the pool-dir-recycling misattribution bug.
- **`smine-orchestrate` skill** documents a real, load-bearing constraint discovered mid-implementation — headless agents cannot write `~/.claude/settings.json` (the CLI's sensitive-file guard blocks it) — and correctly moves `additionalDirectories` reconciliation to the deterministic wrapper (`run.sh`) instead of trusting an LLM to edit the settings file, which is a safety improvement over the plan's original C12/C17 design (see Deviations below).
- Build, vet, and the full non-race test suite are green on a clean checkout of the head commit (see Testing).
- The `casual`/`non-developer` audience rename is applied consistently across Go (`presentation.go`, `profilesettings.go`), shell (`run.sh`, `sync_skills.sh`), and the installed profile doc, with a one-time migration (`sed` rewrite) for machines that already have the old value on disk — no half-renamed surface found.

## Issues

### Critical (Must Fix)

#### 1. `CheckPolicy` self-defeat: a working-copy edit fully disables the ACDSL gated/strict enforcement it exists to guard

- **File:** `internal/acdsl/policy.go:329-345` (`CheckPolicy`), `internal/acdsl/policy.go:195-219` (`policyBase`)
- **What's wrong:** The entire point of this feature (D2, D4 in `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/design/raw.md`) is that the base-ref policy is authoritative, so a branch cannot weaken enforcement by editing its own working-copy `acdsl/policy.json`. That correctly holds on the common path: `CheckPolicy` resolves `base` first, then overwrites the passed-in `policy` with `basePolicy()` read from the base ref via `git show`, so a working-copy `mode` edit alone is ignored. But `policyBase` is resolved using the **working-copy** policy's `BaseRef` field when it is non-empty (`candidates := []string{policy.BaseRef}`), and if that candidate fails to resolve (`GitMergeBase` errors), `CheckPolicy` falls back to the **working-copy** `policy.Mode`:
  ```go
  base, isBoundary, baseErr := policyBase(ctx, root, policy)
  if baseErr != nil {
      if policy.Mode == PolicyModeFree {
          return nil, nil // no base to gate against, nothing declared — free
      }
      return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
  }
  ```
  So a single working-copy edit — `{"mode": "free", "base_ref": "does-not-exist"}` — makes `policyBase` fail before `basePolicy()` (the authoritative read) is ever called, and `CheckPolicy` returns `nil, nil`: zero diagnostics, zero error. This defeats strict/gated mode entirely, on a repo whose *committed* base policy is gated or strict.
- **Why it matters:** This is exactly the self-escalation vector D2/D4 were written to close ("a registry.local.json entry ... neuters it", "in gated mode the rule line binding it could be deleted by the very edit it guards"). The `acdsl-policy-guard.sh` hook (the only other layer) is fail-open by design and reads the same working-copy file, so it does not catch this either — it's advisory, and the plan explicitly says the binary check is the sole authoritative layer. With this hole, that authoritative layer has none.
- **Verified, not assumed:** I wrote a throwaway test in an isolated `git worktree` at the head commit (removed afterward), built from the exact `newPolicyRepo`/`gatedPolicy` test fixtures already in `internal/acdsl/policy_test.go`, reproducing: base committed with `{"mode": "gated", "editable_scopes": ["GOLANG"]}`, a rogue `ACDSL-PLAN-999` rule added on the branch, and `CheckPolicy` called with `Policy{Mode: PolicyModeFree, BaseRef: "does-not-exist-ref"}` (what `LoadPolicy` would return from a tampered working file). Result: `err=<nil> diagnostics=[]` — a full, silent bypass.
- **Test gap confirming this wasn't caught:** the existing `TestCheckPolicyUnresolvableBase` only exercises the unresolvable-base-ref case with `gatedPolicy()` (working-copy mode = gated), which correctly hard-errors — matching D10. There is no test where the *working-copy* mode is `free` while the *base* mode is gated/strict combined with an unresolvable base ref. That is precisely the gap.
- **How to fix:** Don't let `policyBase`'s candidate selection depend on an untrusted working-copy field when the fallback path also trusts that same file's `Mode`. Two independent fixes, either sufices: (a) when `policy.BaseRef` fails to resolve, fall back to the default candidate chain (`origin/HEAD`, `origin/main`, `main`, `master`) instead of only the single override, so an invalid `base_ref` can't blind the resolver; and/or (b) resolve the base **once** using only trustworthy inputs (default candidates, or a `base_ref` that is validated against — or ignored in favor of — the *committed* base's own `policy.json`), and never let `baseErr`'s fallback branch consult the working copy's `Mode` at all — an unresolvable base in the presence of *any* `acdsl/policy.json` on disk (working or base) should hard-error, full stop, matching D10's literal text ("never a silent pass").

#### 2. `acdsl/policy.json`'s own commit removed the pre-existing `ask` permission guard on `acdsl/**` — no replacement covers smine itself

- **File:** `settings/claude_code/settings.json` (commit `5c11486`)
- **What's wrong:** The same commit that registers the new `acdsl-policy-guard.sh` hook also deletes four pre-existing `permissions.ask` entries — `Edit(**/*.acdsl)`, `Edit(acdsl/**)`, `Write(**/*.acdsl)`, `Write(acdsl/**)` — with no plan documenting this removal as a decision (I grepped every plan under `plans/v1-2-1/` for `Edit(acdsl` / `Write(acdsl` / `ask.*acdsl`; the only hits are the *baseline fact* describing the pre-existing `ask` entries, and the README's proposed *new* copy-paste `permissions.deny` snippet for **target** repos — neither says to drop smine's own `ask` entries).
- **Why it matters:** `acdsl/policy.json` ships with `mode: "free"` for smine itself (D8/D11 — "smine must stay free (generation flow commits registry entries on branches)"). `acdsl-policy-guard.sh` only does anything when the local `acdsl/policy.json`'s mode is `strict` or `gated` (`[ "$mode" = "strict" ] || [ "$mode" = "gated" ] || exit 0`). So in the smine repo itself — under `defaultMode: acceptEdits` — an agent can now edit `acdsl/rules.acdsl`, `acdsl/registry.json`, or `acdsl/evalgen.json` (the actual rule and verifier definitions that get frozen and dist-shipped to every target) with **zero** confirmation prompt, where before this change the same edit required an explicit "ask" approval. This is a real, live reduction in guardrails on the repo that authors the whole rule system, landed in the same commit as a feature whose stated purpose is tightening self-management, apparently on the (undocumented, and per Critical #1 also incorrect) assumption that the new hook supersedes it.
- **How to fix:** Either restore the `ask` entries for smine's own settings (defense-in-depth even in free mode, since the hook/binary gate only fire in strict/gated), or make the removal an explicit, plan-recorded decision if it was intentional — as written it reads as an unreviewed side effect of the same commit that should have added, not subtracted, protection.

### Important (Should Fix)

#### 3. `skill-invoke-gate.sh`: unanchored prefix match lets any skill whose name is a prefix-extension of the required one satisfy the gate

- **File:** `cmd/hooks/skill-invoke-gate.sh:38` — `case "$first_user" in "/$required"*) invoked=1 ;; esac`
- **What's wrong:** The gate is meant to force a session whose first message carries `REQUIRE-SKILL:<name>` to invoke `Skill(<name>)` (or start with `/<name>`) before any other tool call. The prefix-match pattern `"/$required"*` matches any first message that merely *starts with* `/$required`, with no word-boundary check. This repo's own skill roster has ten names sharing the `smine` prefix (`smine-apply`, `smine-batch`, `smine-consolidate`, `smine-context`, `smine-memory`, `smine-orchestrate`, `smine-permissions`, `smine-routines`, `smine-skills`, `smine-style`).
- **Verified:** I piped a synthetic PreToolUse `Read` event into the shipped hook script with a transcript whose first user message was `/smine-batch --nightly ... REQUIRE-SKILL:smine` — i.e., the required skill is `smine` but the session actually started `/smine-batch`, a different skill. The hook exited 0 with empty stdout (no deny), i.e., it silently treated the mismatched invocation as satisfying the `smine` requirement.
- **Why it matters:** This is the mechanism a railroad/headless lane depends on to guarantee its review/task context actually loaded before any other action — including, per the hook's own comment, for exactly this kind of eval harness. A false "invoked" bypass means a lane can run its entire task against a *different* skill's context (or none) while the gate stays silent, with no signal that anything went wrong. `cmd/tests/test_skill_invoke_gate.sh` only exercises `railroad-review` (a name with no prefix collision in the roster), so this gap has no regression coverage.
- **How to fix:** Require a word boundary after the required name — e.g. match only `"/$required"` (exact) or `"/$required "*`/`"/$required"$'\n'*` (followed by whitespace or newline), never a bare prefix.

#### 4. Worktree status page shows a stale "scanned at" timestamp next to a fresh-scan error

- **File:** `internal/server/repos.go:345-367` (`handleRepoWorktreeStatus`), rendered at `internal/server/templates/_worktree_status.html:34`
- **What's wrong:** On a cache-miss (fingerprint changed or `?refresh=1`) whose fresh `repos.Status()` call errors, the code does:
  ```go
  entry, cached := s.statusCache.Get(repo.Name)
  ...
  } else {
      statuses, err = repos.Status(r.Context(), repo.Path, s.worktreeScripts)
      if err == nil {
          entry = s.statusCache.Store(repo.Name, fingerprint, statuses)
      }
  }
  ...
  data.ScannedAt = entry.ScannedAt
  ```
  When `err != nil`, `entry` is never reassigned, so it stays whatever `Get` returned — the **previous, now-stale** cache entry (if one existed) — and `data.ScannedAt` is set unconditionally from it. The template renders `as of {{.ScannedAt.Format ...}}` (line 34) with no `StatusErr` guard, so the fragment shows both the error banner *and* a confident "as of HH:MM:SS" from the last successful scan, which for this render never actually ran.
- **Why it matters:** This is a new field (`ScannedAt`/caching didn't exist pre-diff, confirmed against the base commit — there was no timestamp to be stale before), and it directly contradicts the plan's own stated principle for this feature ("the cache ... can never re-serve rows that no longer match reality" / "degrade toward correctness, never toward stale rows", D6). The rows correctly stay empty on error, but the metadata around them lies about freshness. No test in `repos_test.go` touches `ScannedAt`, so this shipped without coverage.
- **How to fix:** Only set `data.ScannedAt` when `data.Cached` is true or `err == nil`; leave it zero (template already handles `.ScannedAt.IsZero`) otherwise.

### Minor (Nice to Have)

- `internal/acdsl/policy.go` / `git.go`: `GitShowFile`'s "absent path means ok=false, never an error" contract relies on the underlying git failure being an `*exec.ExitError` specifically; that's correct for the current `shell.Run` implementation but is a slightly implicit coupling worth a one-line comment at the call site if `shell.Run`'s error wrapping ever changes.
- `cmd/hooks/skill-invoke-gate.sh`'s Bash-command target extraction (the `cat <file>` heuristic) only catches the single-word `cat file` shape; `head`, `sed -n`, `less`, or a quoted `cat "$file"` all bypass the post-invocation context-read deterrent. Given the hook is explicitly fail-open/best-effort, this is consistent with its stated posture, not a regression — noted for completeness rather than as an ask.
- `internal/acdsl/policy.go:143` (`overlayViolations`) and the `Registry`/`Policy` struct field orderings mirror the plan's own approved code verbatim, including field groupings that don't sort perfectly alphabetically within their group (`RULE-GOLANG-STRUCT-002`); since this is byte-for-byte what the approved plan specified, I'm not flagging it as an implementation defect, but worth knowing the plan itself introduced the deviation if a style sweep ever touches these files.

## Deviations from plan (flagging for confirmation, not blocking)

- **`smine-orchestrate` no longer writes `additionalDirectories` itself** (plan C12/C17: "the manifest grants Edit/Write ... settings mutations target `~/.claude/settings.json` directly"). The shipped skill instead only *verifies* the reconciliation and reports gaps; `routines/smine-nightly/run.sh` (stage 3, lines ~298-305) does the actual jq-based append. The skill file states the reason plainly ("headless agents cannot write `~/.claude/settings.json` — the CLI's sensitive-file guard blocks it"), which is a real platform constraint discovered during implementation, not an unexplained departure — and moving a mechanical, security-sensitive file write from an LLM to deterministic shell code is a defensible, arguably better design than the plan's original. Flagging only because it's a discovered-mid-implementation contract change (ACTION-IMPL-001 territory) that the plan document itself was never updated to reflect (no Changelog row for it in the acdsl-policy-modes or nondev-automation plans I read).
- **`audience: non-developer` renamed to `audience: casual`** everywhere, with a migration path for already-installed profile files. This is not documented anywhere in the two plans I read under `plans/v1-2-1/language-style-setting-wip*` (both still say "non-developer" throughout); it appears to be a later, unrecorded rename. The implementation itself is consistent (see Strengths), so this is a documentation-lag flag, not a code defect.

## Testing

- `go build ./...` and `go vet ./...`: clean, on both the eval's `arm-wt` checkout and a separately-checked-out clean worktree at the head commit.
- `go test ./...` on `arm-wt` as handed to me: **one failure**, `internal/acdsl.TestDistShipsReachCoveredSliceWithBinaries` (a `bin/verifiers/gofmt: no such file or directory` stat failure). I traced this to the `arm-wt` checkout carrying pre-existing, uncommitted local modifications to `acdsl/registry.json` (and several other files) that were already present in `git status` before I ran anything — this specific test reads the repo's *actual* `acdsl/registry.json` off disk (`const repoRoot = "../.."`) rather than a fixture, so a dirty working tree with a rewritten (dist-projected-looking) registry breaks its "go run" argv assumptions. I confirmed this is **not a regression in the reviewed code**: a fresh `git worktree add --detach` at the exact head commit `a342811b` runs `go test ./...` fully green, including that test. I'm reporting this so the harness owner can decide whether `arm-wt`'s pre-existing dirty state is expected for this eval setup, but it is not a defect in the diff under review.
- On the clean head-commit worktree: full `go build`, `go vet`, and `go test ./...` (all packages) pass, no failures, no skips beyond the normal `[no test files]` for a couple of leaf packages.
- I did not run `make audit-full` (race tests + the `cmd/tests` shell suite) given time budget; the shell-suite additions (`test_skill_invoke_gate.sh`, `test_sync_skills_overrides.sh`, `test_bootstrap_wrapper.sh`, `test_read_gate.sh`, extensions to `test_print_agent_worktrees_status.sh` and `test_remove_agent_worktrees.sh`) were read but not executed.

## Recommendations

- Add the missing negative test for Critical #1 (`gatedPolicy()`'s base-ref override failing while the *base* commit's policy is gated/strict) to `policy_test.go` alongside the fix — this is exactly the shape of test the existing suite is missing.
- Add a `test_skill_invoke_gate.sh` case using two skills where one name is a prefix of the other (the repo has ten ready-made examples in the `smine*` family) to lock in the fix for Important #3.
- Given `acdsl-policy-guard.sh` and `CheckPolicy` are now the two enforcement layers for a security-shaped feature (self-management gating), it would be worth a short "adversarial" pass specifically hunting for other `policy.json`-field-driven bypasses of the same shape as Critical #1 — the `local_overrides`/`verifier_allowlist` fields are read from the *base* policy correctly (via `basePolicy()`), so they don't share this hole, but the pattern (working-copy value trusted before the base-ref override attempt) is worth a deliberate sweep now that one instance is confirmed.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of the release (registry read-modify-write, status-cache fingerprinting, session attribution, the audience/casual rename, the bootstrap/orchestrate wiring) is solid, well-tested, and matches its plans. But the ACDSL self-management feature — the one piece of this release explicitly built to close a security gap — ships with a confirmed, reproducible bypass of its sole authoritative check (Critical #1), compounded by an apparently-unintentional removal of the pre-existing manual safety net for the one repo (smine itself) that runs in the mode the new checks don't cover (Critical #2). Both are narrowly scoped and each has a small, well-understood fix; neither requires redesign. The two Important items are real but lower-stakes (eval-harness integrity, a cosmetic timestamp) and shouldn't block a merge once the Criticals are addressed.
