# Review: smine 1.1.4 → 1.2.1 (b4981b88..a342811b)

Scope: `git diff b4981b88..a342811b -- :(exclude)sessions/ :(exclude)evals/ :(exclude)proposals/ :(exclude)plans/` — 178 files, +7314/-749. Reviewed in this checkout: `.../runs/sonnet-5-high-r7/artifacts/arm-wt`, read-only (no commits, no worktree/index mutation; scratch worktrees I added at `/tmp/*` to test a clean HEAD were removed after use).

**Method:** the diff is too large for one uniform pass. I read the full unified diff (12k lines) file-by-file, then went deep (full function bodies, tests, and the corresponding plan) on the highest-risk units per the repo's own risk tiering (ACTION-REVIEW-RISK-001/002): the ACDSL self-management policy engine, the repo registry read-modify-write fix, the new config-server features (bootstrap, profile settings, status cache), and the new/changed shell hooks. I verified build/vet/tests myself (not just read them) on a clean checkout of the head commit (the review checkout itself has unrelated local working-tree drift in `acdsl/*.json`, unrelated to this diff — see note under Testing). Lower-risk files (SKILL.md prose, changelog.json, HTML templates, docs) got a lighter pass for scope creep and internal consistency only, per ACTION-REVIEW-RISK-003.

Plans consulted: `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/`, `2026-08-30-repo-cache-reload-wip-46c8d9e/`, `2026-08-27-repos-routines-improvements-wip-4afbe98/`, `2026-08-27-bootstrap-profile-improvements-wip-397f906/`, `2026-08-27-smine-profile-settings-wip-ca34568/`, `2026-08-30-peek-multiple-profiles-wip-78bfb3e/`, `2026-08-24-language-style-setting-wip-ef09567/`, `session-context-report/`.

## Strengths

- **`internal/repos/registry.go` read-modify-write fix** is exactly right: `Add`/`Remove` now re-read the file under the lock (`readRegistrySnapshot`) instead of mutating a possibly-stale in-memory slice, closing the last-writer-wins bug the fact note (FACT-REPO-ARCH-016) described. `refreshIfChanged` correctly stats before reading so a write racing a read is caught on the *next* stat rather than silently missed. Matches the repo-cache-reload plan.
- **`internal/server/statuscache.go`**: fingerprint-gated cache with a well-reasoned exception — `DropBranches` deliberately keeps the *pre-removal* fingerprint so the next plain load always mismatches and re-scans after an optimistic-UI removal, rather than trusting a hand-maintained cache patch. Good instinct, and it's explained in the doc comment, not just done.
- **`policyBase`/`basePolicy` design intent** (before the flaw below): making the *base-ref* policy authoritative rather than the working copy is the right instinct — `TestCheckPolicyBaseModeSurvivesWorkingWeakening` proves a plain "delete/downgrade your local policy.json" attempt is caught. That specific attack is closed.
- **Command construction hygiene**: `profilesettings.go`'s style-test call and `bootstrap.go`'s wrapper spawn pass the token path, prompt, and model as separate `argv` elements into `bash -c '...' bash $1 $2 $3`, never string-interpolated into the script — no shell-injection surface from user-controlled prompt text, and the OAuth token value itself never appears in argv or is echoed (`tr -d` reads it from a file only inside the child's environment setup).
- **Concurrency shape in `handleRepoWorktreeStatus`**: checkout, peek session index, and the (possibly cached) status scan run in parallel with per-goroutine locals and independent error surfaces, each degrading to its own banner instead of failing the whole fragment — consistent with the existing D3/D7 pattern in that file.
- Build, `go vet`, and `gofmt -l` are clean on a fresh checkout of the head commit; the full `go test ./...` is green (see Testing).

## Issues

### Critical (Must Fix)

#### 1. ACDSL policy self-management gate is bypassable in one commit via `base_ref` — the feature's entire threat model is defeated

- **File:** `internal/acdsl/policy.go:3068-3092` (`policyBase`), used from `CheckPolicy` at `internal/acdsl/policy.go:3202-3220`, invoked from `cmd/acdsl/main.go:340-345`.
- **What's wrong:** `CheckPolicy` is handed the *working tree's own* `acdsl/policy.json` (read by `LoadPolicy(*root)` in `main.go`, no validation against anything trusted yet). It immediately calls `policyBase(ctx, root, policy)`, which — when `policy.BaseRef` is non-empty — uses that value verbatim as the *only* candidate ref to diff against (`internal/acdsl/policy.go:3069-3072`: `if policy.BaseRef != "" { candidates = []string{policy.BaseRef} }`). Only *after* the base is resolved does the code fetch the "authoritative" base policy via `basePolicy(ctx, root, base, ...)` (`GitShowFile(ctx, root, base, "acdsl/policy.json")`) and run the surface freeze (`surfaceDelta`) against that same `base`.

  Since `base_ref` is a field inside `acdsl/policy.json` — the very file `policySurface` is supposed to freeze — and that file is fully attacker-editable on the agent's own branch (that is the actor this feature exists to constrain), an agent can:
  1. Edit `acdsl/policy.json` to `{"mode": "free", "base_ref": "HEAD"}` (or any ref they control — their own branch name, a tag they create).
  2. Commit it on their own `claude/*` branch (a private worktree commit is exactly the standing capability the design assumes agents have — D4 says the boundary is "committing to the base branch"; this doesn't require that).
  3. Run `acdsl check`.

  `GitMergeBase(ctx, root, "HEAD")` runs `git merge-base HEAD HEAD`, returning the tip of the agent's own branch as `base`. `basePolicy` then reads `acdsl/policy.json` *at that same commit* — i.e. the attacker's own malicious content — and treats it as ground truth. `policy.Mode == PolicyModeFree` short-circuits `CheckPolicy` to `return nil, nil` before `surfaceDelta`, `overlayViolations`, or `ruleViolations` ever run. Even without flipping to `free`, the same trick makes `surfaceDelta` compare the working tree against itself (zero diff, by construction) and lets an attacker widen `editable_scopes`/`verifier_allowlist` freely in `gated` mode.

  This directly contradicts D4's stated guarantee ("agent worktrees on `claude/*` branches can never self-escalate") and the plan's headline claim ("the loosy-goosy-rules failure mode is structurally closed").

- **Verified, not asserted:** I wrote a standalone test against a clean checkout of the head commit (`/tmp/head-clean`, since discarded) reusing the shipped test fixtures (`newPolicyRepo`, `gatedBasePolicy`):
  ```go
  func TestExploit_BaseRefSelfReference(t *testing.T) {
      root := newPolicyRepo(t, gatedBasePolicy) // main: gated, GOLANG editable only
      malicious := `{"mode": "free", "base_ref": "HEAD"}`
      writePolicyFile(t, root, "acdsl/policy.json", malicious)
      writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n"+
          `//acdsl:ACDSL-PLAN-999 gofmt anchor="^plans/" why="agent self-escalation"`+"\n")
      policyGit(t, root, "add", ".")
      policyGit(t, root, "commit", "-q", "-m", "attacker commit")

      workingPolicy, _ := LoadPolicy(root)
      diagnostics, err := CheckPolicy(context.Background(), root, workingPolicy)

      require.NoError(t, err)
      assert.Empty(t, diagnostics) // PASSES — zero diagnostics for a PLAN-scope addition under "gated"
  }
  ```
  This passes: `CheckPolicy` returns **zero diagnostics** for an addition to a protected (`PLAN`) scope under a `gated` base policy that only allows `GOLANG`. A control test with the identical malicious edit but *no* `base_ref` override (`{"mode": "free"}` only) correctly produces diagnostics, confirming the base-ref field specifically is the hole, not the free-mode-downgrade path (which the shipped tests do cover).
- **Why it matters:** this is a security feature whose sole purpose is to stop exactly this actor (a gated/strict target's agent branch) from exactly this outcome (unrestricted rule/verifier changes). As shipped, one commit — no tooling bypass, no privilege escalation beyond what every agent branch already has — fully disables it, in both `gated` and `strict` mode, and leaves `acdsl check` reporting clean. Anyone relying on this feature (the plan's stated purpose for shipping it to target repos via `dist_mode: gated`) has no real protection.
- **How to fix:** don't let the file under test choose which ref is authoritative for itself. Concretely: resolve the base using only hardcoded upstream candidates (`origin/HEAD`, `origin/main`, `main`, `master`) plus, if an override is needed for shallow clones, source `base_ref` from somewhere outside `policySurface`'s attacker-reachable scope (e.g. a CLI flag or a file explicitly excluded from what an agent branch can commit and validated to already exist on `origin/HEAD` before use — i.e. verify the *candidate ref itself*, not just its content, is reachable from an upstream tracking ref before trusting anything read from it). At minimum, `policyBase` must never accept a `BaseRef` value that cannot be shown to already exist on a remote-tracking ref (`git merge-base --is-ancestor` against `origin/*`, or requiring `base_ref` to itself be an `origin/*` ref). Whatever the fix, add the missing test case (`fail-base-ref-self-reference` or similar) alongside the existing `TestCheckPolicyUnresolvableBase`.

### Important (Should Fix)

#### 2. `handleRepoWorktreeStatus` can show a stale `ScannedAt` alongside a fresh-scan error

- **File:** `internal/server/repos.go:346-364`.
- **What's wrong:** `entry, cached := s.statusCache.Get(repo.Name)` is read once, up front. When the fingerprint doesn't match (`data.Cached == false`) and the subsequent `repos.Status(...)` call fails (`err != nil`), `entry` is never reassigned — it keeps whatever was returned by the initial `Get` (a previous, now-stale scan, or the zero value if nothing was cached yet). `data.ScannedAt = entry.ScannedAt` (line 364) then renders either a stale timestamp next to `data.StatusErr`, or a zero `time.Time` if this is the first-ever scan and it fails. Neither is what the reader wants to see next to an error banner.
- **Fix:** only set `data.ScannedAt` from `entry.ScannedAt` when `data.Cached` is true or the fresh scan succeeded; otherwise leave it zero (and have the template treat zero as "no successful scan yet", which it likely already needs to for the first-ever-load case).

#### 3. `smine-nightly/run.sh` mutates the operator's live `~/.claude/settings.json` from an unattended nightly job with no backup

- **File:** `routines/smine-nightly/run.sh:10036-10046`.
- **What's wrong:** stage 3 (orchestrate) does an unattended `jq ... "$settings_file" > "$settings_file.tmp" && mv "$settings_file.tmp" "$settings_file"` against the user's real global Claude Code settings — the file every session on the machine reads permissions from. The jq expression itself is correct (append-only dedup: `+ ([...] - existing)`), and it's atomic (`tmp` + `mv`), but there is no pre-write validation (`jq empty "$settings_file.tmp"` before the `mv`) and no backup of the previous `settings.json`. If `repos.json` were ever malformed in a way jq tolerates (e.g. `repos: null`) the resulting `additionalDirectories` could silently become something unexpected, and it's applied with no operator confirmation in an unattended run.
- **Why it matters:** this is exactly the kind of standing-config mutation ACTION-REVIEW-SPEC-004 (no behavior regressions) and general config-write hygiene ask for extra care on, because a bad write degrades every future session's permissions silently until noticed. The atomic tmp+rename means a *crash* mid-write is safe; it does nothing for a *logically* bad write.
- **Fix:** validate the tmp file parses and that `.permissions.additionalDirectories` is still an array of strings before the `mv`; keep one rotated backup (`settings.json.bak`) the way the repo already does for other user-owned config elsewhere (worth checking `sync_settings.sh` for the existing pattern before inventing a new one).

#### 4. `writePolicy` fixture and `LoadPolicy` default disagree slightly on `dist_mode` visibility, worth confirming against D8

- **File:** `internal/acdsl/dist.go:2678-2704` (`writePolicy`), `acdsl/policy.json` (shipped source values).
- **What's wrong:** this is a minor spec-fidelity note rather than a bug: `writePolicy` strips `DistMode` before marshaling the shipped copy (correct per D8 — "a target never re-distributes"), and `TestDistShipsPolicyModeRewritten` confirms `shipped.DistMode` is empty. Good. But the *committed source* `acdsl/policy.json` sets `mode: "free"` for smine itself — meaning `acdsl check` in the smine repo's own CI never exercises `CheckPolicy` beyond the immediate `PolicyModeFree` early return once `policyBase` succeeds. Combined with finding #1, the *only* place this feature is meaningfully exercised in this repo's own automated gates is the unit test file — `make audit`'s `./bin/acdsl check` step never runs the gated/strict code paths for real. Not a defect in the diff, but worth flagging per ACTION-REVIEW-DOCS-004: the plan's own Verification checklist item ("Create a scratch git repo... expect `check` policy-clean") is a manual step, not wired into CI, so a regression here (like #1) would ship silently in a future change too.
- **Fix (or explicit acceptance):** either add a dedicated CI step that runs `acdsl dist` into a scratch repo and asserts a known-bad gated/strict scenario is caught (turning the plan's manual verification scenarios into an automated regression test), or record this as a tracked gap per ACTION-REVIEW-DOCS-004 rather than leaving it implicit.

### Minor (Nice to Have)

#### 5. `cmd/hooks/skill-invoke-gate.sh` context-read denial is trivially bypassable via Bash, same as `acdsl-policy-guard.sh`

- **File:** `cmd/hooks/skill-invoke-gate.sh:1044-1056`.
- **What's wrong:** the `Bash` branch only recognizes a bare `cat <path>` two-token command (`set -- $command; [ "$#" -eq 2 ] && [ "$1" = "cat" ]`) and explicitly bails on any command containing shell metacharacters. `head "$ctx_dir/foo"`, `less`, `$(cat ...)`, or `cat "$ctx_dir"/../$(basename "$ctx_dir")/foo` all sail through unexamined. This is consistent with the hook's own documented stance ("Fail-open... advisory") and mirrors the same accepted trade-off in `acdsl-policy-guard.sh`, so it's not a new problem — just noting it's easy to defeat if anyone ever treats it as a real boundary rather than a nudge.
- **Fix:** none required; consider tightening the comment at the top of the file to say explicitly that this is a UX nudge only, not a boundary (the ACDSL policy guard's doc comment already does this well — worth matching the wording).

#### 6. `styleTestPrompt`/`bootstrapEnv` — extra-prompt validation blocks quotes/newlines but not all shell-meaningful characters

- **File:** `internal/server/bootstrap.go:3629-3675` region is fine (env-only, no shell reinterpretation), but flagging for completeness: `bootstrapEnv`'s validation (`strings.ContainsAny(extraPrompt, "\"'\n")`) is a defense against the string being mis-quoted if some future code path interpolates it into a script rather than passing it as an env var / argv element, as it does today. Today's usage is safe (env var, then passed to `claude -p "$mine_prompt"` as one bash variable expansion, never `eval`'d). No fix needed now; just don't let a future refactor move this into an `eval`-based path without re-deriving the validation.

## Recommendations

- Given finding #1, treat this as blocking regardless of the "done" state marked on `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/`: the plan's own Decisions table (D4, D10) and Hot Items section state the exact guarantee that's broken, so this reads as an implementation gap against an approved design, not a design change that needs re-approval.
- Once #1 is fixed, add it as a permanent regression test in `policy_test.go` (`fail-base-ref-points-at-own-branch` or similar) — the existing test suite is otherwise thorough (16+ named cases covering exactly the taxonomy in the plan's Tests table) and this is the one class of attack it doesn't cover.
- The registry.go / statuscache.go pairing (finding-free) is a good template for other "cache a subprocess scan, invalidate on a cheap fingerprint" spots in this codebase if more appear later — no action needed, just noting it as the pattern to reuse.

## Testing

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `gofmt -l .` — clean except `acdsl/testdata/ACDSL-GOLANG-FMT-001/fail/unformatted.go`, which is a deliberately-unformatted fixture for the gofmt-wrap verifier's fail case (expected).
- `go test ./...` on a **clean worktree checkout of the head commit** (`a342811b4cf7b8bb2b54359ced6c786d3b5f349b`, no local modifications) — all packages pass.
- Note: the review checkout provided to me (`.../runs/sonnet-5-high-r7/artifacts/arm-wt`) has pre-existing, uncommitted local drift in `acdsl/policy.json`, `acdsl/registry.json`, and several `context/*` files (visible via `git status --short` there) that is **not part of this diff** — it looks like local tooling (context-doc generation / a prior `acdsl dist` or `project -strip` run) touched those files in that specific worktree. Running the test suite directly in that dirty worktree produces one failure (`TestDistShipsReachCoveredSliceWithBinaries`, because the polluted `acdsl/registry.json` no longer has the `go run ./cmd/acdsl/verifiers/gofmt`-shaped argv the test's own build step expects). I confirmed this is not a regression from the reviewed diff by running the identical test against both the pre-change base commit and a freshly-checked-out copy of the head commit in separate temporary worktrees — both pass. This is an artifact of the review checkout's local state, not a code defect; flagging it so it isn't mistaken for a real test failure by anyone re-running `go test` directly inside that specific `arm-wt` directory.

## Assessment

**Ready to merge?** No — blocked on Critical finding #1.

**Reasoning:** the release is broad but mostly solid engineering (registry race fix, cache fingerprinting, config-server additions) with clean build/vet/tests. However, the headline new feature of this release — ACDSL self-management policy modes, whose entire purpose is to stop an agent branch from weakening its own governance — is defeated by that same actor in a single commit via the `base_ref` field, contradicting the plan's explicit security guarantee (D4) and its own "structurally closed" claim. That's a shipped security control that doesn't hold under the exact threat model it was built for, verified with a reproducible test against the actual code. Everything else here (Important #2-4, Minor #5-6) is fix-alongside-or-soon, not merge-blocking on its own.
