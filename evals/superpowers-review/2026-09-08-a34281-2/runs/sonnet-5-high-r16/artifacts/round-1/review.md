# Review: smine v1.1.4 → v1.2.1

Range `b4981b88..a342811b` (178 files, +7314/-749, excluding sessions/evals/proposals/plans). Reviewed in three passes: (1) automated verification — build, vet, full `go test`, `make audit`, and the full `cmd/tests/test_*.sh` shell suite, run against a clean `git worktree` checkout of the exact head commit (the provided `arm-wt` checkout had uncommitted local mutations from unrelated session tooling — e.g. a rewritten `acdsl/registry.json` — that made testing in-place misleading; a side-by-side worktree of `b4981b88` was also built to confirm one apparent regression was environmental, not real); (2) deep manual read of the highest-risk/most novel subsystems: the new ACDSL self-management policy engine, the repo-registry concurrency fix, launchd/routine scheduling changes, config-server bootstrap/status-cache, and the two new PreToolUse hooks; (3) structural spot-checks of the remaining lower-risk template/doc/skill-changelog churn (a large fraction of the 178 files are `SKILL.md`/`changelog.json` version bumps consistent with a release).

Plan docs consulted: all nine `plans/v1-2-1/*/design/*.md` docs plus `concept/`.

## Strengths

- **`internal/repos/registry.go`** — the read-modify-write fix for FACT-REPO-ARCH-016 (a second process's stale in-memory slice resurrecting a deleted entry) is correctly implemented: `Add`/`Remove` now re-read the file fresh under the write lock before mutating, and reads (`Find`/`Repos`) transparently pick up external edits via an mtime+size fingerprint rather than a TTL. `TestRegistryDeleteStaysDeletedAcrossInstances` pins the exact documented failure scenario. This is a clean, well-tested fix of a real, previously-documented bug.
- **`internal/server/statuscache.go`** + `repos.Fingerprint`/`WorktreeCounts`** — the worktree-status cache invalidates on a data-derived git fingerprint (branch tips + worktree set), not a timer, matching the repo's own stated doctrine (ACTION-IMPL-INTEG-008/010 spirit) and FACT-REPO-ARCH-016's cache-invalidation note. `DropBranches` correctly keeps the stale fingerprint on optimistic removal so the next plain load re-scans instead of silently trusting a hand-edited cache.
- **`internal/routines/routines.go` `History`** — the new "started" marker-row handling (a live run's start row is kept only while it is the newest line in the file) is applied to the *pre-truncation, pre-reversal* list, so `limit` and the "newest first" reversal can't interact badly with it. Correct on inspection and matches its own doc comment.
- Full release gate passes clean on the actual reviewed diff: `go build ./...`, `go vet ./...`, `go test ./...` (all packages), `make audit` (acdsl `check`/`fixtures`, 24 rules OK), and every `cmd/tests/test_*.sh` shell test — all green on a clean worktree of `a342811b`.
- Template helpers that build `template.HTML` from user-controllable strings (`nameWrap`, `pathWrap`) correctly run `template.HTMLEscapeString` per-segment before injecting `<wbr>` — no XSS despite the new user-editable `Repo.Label` field.
- Removal of the single-branch `handleRepoRemove` route was done cleanly — no dangling route registration, no orphaned template reference to the old endpoint.

## Issues

### Critical (Must Fix)

- **`internal/acdsl/policy.go:329-346` (`CheckPolicy`) — the self-management gate can be silently disabled from a working tree it is supposed to be defending against.** The whole feature's purpose (plan `D2`, `D4`, `D10`) is that an agent branch cannot escalate its own editing rights by touching `acdsl/policy.json`; the implementation correctly re-derives the *authoritative* mode from the base ref (`basePolicy`) in the normal path — this is a good hardening beyond the plan's own pseudocode, which naively trusted the local file's `Mode` first. But the `baseErr` branch reintroduces exactly that trust:
  ```go
  base, isBoundary, baseErr := policyBase(ctx, root, policy)
  if baseErr != nil {
      if policy.Mode == PolicyModeFree {
          return nil, nil // no base to gate against, nothing declared — free
      }
      return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
  }
  ```
  `policy` here is still the untrusted **working-copy** value passed in by the caller (`cmd/acdsl/main.go`'s `acdsl.LoadPolicy(*root)`, reading the mutable file on disk). An agent on a `claude/*` branch in a repo whose *committed* policy is `gated`/`strict` can defeat the entire mechanism with a two-field edit to the working `acdsl/policy.json`:
  ```json
  {"mode": "free", "base_ref": "nosuchref"}
  ```
  `policyBase` then fails to resolve any candidate (only one candidate is tried once `base_ref` is set), `CheckPolicy` sees the *local* `policy.Mode == PolicyModeFree` and returns `nil, nil` — no diagnostics, no error, and `D10` ("unresolvable base is a hard tool error, never a silent pass") is violated for the one case that actually matters. Reproduced directly against `CheckPolicy`: a repo with a committed `gated` base policy, a snuck-in `PLAN`-scope rule, and the tampered working policy above yields **zero diagnostics and no error** — the exact loosy-goosy-rules failure mode this feature exists to close. (Verified with a standalone repro test against the built package; not left in the tree.)
  - **Fix:** the `baseErr` early-out must not consult the untrusted working `policy.Mode`. Either (a) treat every unresolvable base as a hard error unconditionally (matching `D10`'s literal wording — `smine` itself, which ships `mode: "free"`, would need `base_ref` handling or a documented exemption for repos with no fetched base), or (b) resolve *some* trustworthy signal that the repo is free-mode before honoring `baseErr` as benign — e.g. only skip the hard error when `acdsl/policy.json` is *absent* from the working tree entirely (can't be tampered to claim a mode it doesn't have), not merely claims free.
  - This is also an **undocumented deviation** from the plan's `Changes §1` pseudocode (which never called `basePolicy` before the free-mode check) — the base-ref-authoritative restructuring is a real improvement, but it wasn't captured in the plan's Changelog, and the specific gap it left open wasn't caught by the plan's own `TestCheckPolicyUnresolvableBase` case (which only exercises an *already-gated* working policy, never a working policy claiming `free` with a bad `base_ref`). Flagging for confirmation this was seen and is intentional to leave for a follow-up, vs. an oversight.

### Important (Should Fix)

- **`cmd/hooks/skill-invoke-gate.sh:62-73`** — the post-invocation "context files stay denied" enforcement for `Bash` only recognizes a literal two-word `cat <path>` command (and bails out of even trying whenever the command contains any of `; | & < > $ backtick ' "` — i.e. almost anything else). `head -c 999 <path>`, `less <path>`, `sed -n p <path>`, `python3 -c "print(open('x').read())"`, or even `cat  <path>` piped through anything, all skip the check entirely and read the supposedly-injected-only context file straight through Bash. Given the stated purpose — proving a review arm actually went through the forced `Skill()` call rather than reading context files directly (per the recent "forced-Skill-call context delivery" commits) — this heuristic is trivially sidestepped by an arm that isn't actively hostile, just using a slightly different command. If this hook is load-bearing for eval integrity (arms being scored on whether they followed the injected-context contract), it should deny on anything that isn't `Read`/`Grep`/`Glob` touching the context dir, or match on the resolved absolute path being under `ctx_dir` for *any* tool, rather than trying to parse Bash commands.
- **`internal/acdsl/git.go` — `GitRevParse`** is declared, documented, and unit-tested (`git_test.go`) but has zero production call sites (`grep` finds only its own test). Per `ACTION-IMPL-ARCH-001`/`RULE-GOLANG-CONST-001`'s spirit ("every new concept traces back to the request"), this is dead exported surface from the plan's `Changes §2` helper list that the final implementation didn't end up needing. Low severity, but worth either removing or documenting why it's kept for a near-term consumer.

### Minor (Nice to Have)

- **`install.sh`** — `PEEK_HOME_ARGS` is built by concatenating raw shell-variable content directly into plist XML (`<string>-claude-home</string><string>$PEEK_CLAUDE_HOME</string>`) and injected via `sed`'s replacement string. Neither the path value's XML special characters (`&`, `<`, `>`) nor sed's own replacement-string metacharacters (`&`, backslash) are escaped. A `PEEK_CLAUDE_HOME`/`PEEK_CODEX_HOME` containing one of these (unusual but possible, e.g. an `&`-named folder) would silently corrupt the generated LaunchAgent plist. Low real-world likelihood; worth a one-line `sed` escape if it's cheap.
- Several skills' `changelog.json` bumps and `SKILL.md` wording tweaks were spot-checked structurally (valid JSON, version monotonic) rather than read line-by-line given the volume (~40 files of this shape) — flagging so the scope of this review is explicit, not implying anything was found wrong there.

## Recommendations

- Add the missing `TestCheckPolicyUnresolvableBase`-style case with a **free-claiming working policy + unresolvable base** against a **gated/strict committed base** to `policy_test.go` — this is exactly the class of case the plan's own test table (`fail-unresolvable-base-hard-error`) was meant to cover, but the case as written only exercises an honest gated working copy.
- Once the `CheckPolicy` fix lands, add a Changelog row to `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/` documenting both the base-authoritative restructuring and the fix — the plan dir is marked `done` and archived-in-place in this range (see `34315e8`/`a342811`), so this is the last chance to record the deviation before it leaves the code repo per `RULE-PLAN-073`.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of this range — the registry concurrency fix, cache invalidation, routine scheduling, and the large volume of skill/doc churn — is solid, well-tested, and matches its plans. But the flagship new feature of this release, ACDSL self-management policy modes, has a reproducible bypass that defeats its entire purpose (an agent branch can silently force a gated/strict repo back to unenforced by editing the one file the feature exists to freeze, combined with an unresolvable `base_ref`). That is a correctness/design bug in code shipped as `done` in this exact release, not a hypothetical — it should be fixed (or explicitly scoped out with a documented follow-up) before this is treated as complete.
