# Review: smine v1.1.4 → v1.2.1

Range: `b4981b88..a342811b` (excluding `sessions/`, `evals/`, `proposals/`, `plans/`) — 178 files,
+7314/-749. Plans reviewed: the nine dirs under `plans/v1-2-1/`.

**Method note:** this diff bundles nine independent features. I could not verify every file
line-by-line at equal depth in one pass. I went deep (full plan-vs-diff trace, ran the actual code)
on the two highest-risk areas — ACDSL self-management (`internal/acdsl/policy.go` + wiring) and the
repo registry/status-cache concurrency rework (`internal/repos/registry.go`,
`internal/server/statuscache.go`, `internal/server/repos.go`) — because both are new
guard/concurrency logic per `ACTION-REVIEW-RISK-001`. I did a second pass, still with diff-vs-plan
verification but less exhaustively, on the presentation-profile/i18n feature and the routine
loader/schedule changes. Everything else (config-server profile/bootstrap UI, sessions/peek
attribution, docs, the smine-* skill/workflow edits, installer changes) got a lighter read for
obvious defects and plan alignment, not full trace-through. `go build ./...` is clean; `go test`
across `internal/acdsl`, `internal/repos`, `internal/server`, `internal/routines` passes except one
`internal/acdsl` test that fails only because this checkout's `acdsl/*` files are locally modified
by the harness that set up this worktree (confirmed by reproducing clean on a fresh `git worktree
add` of the exact head commit, where it passes) — not a defect in the reviewed range.

## Strengths

- **Repo-cache-reload plan (`internal/repos/registry.go`, `internal/server/statuscache.go`,
  `internal/server/repos.go`) is implemented essentially verbatim against its plan**, including the
  exact diffs. The fix for the documented "second configserver instance resurrects a deleted repo"
  bug (`FACT-REPO-ARCH-016`) is correct: `Add`/`Remove` now re-read the registry file fresh under
  the write lock before applying their delta, so a stale in-memory slice can never be written back.
  The residual multi-process TOCTOU (two processes racing `save` on the same delta base) is
  explicitly called out as accepted/untested in the plan rather than silently left — good practice.
- **Status-cache fingerprinting** (`Fingerprint`, `WorktreeCounts` split into `WorktreeKindCount`)
  replaces a never-expiring cache with a deterministic, git-derived invalidation signal. Error paths
  degrade toward a fresh re-scan, never toward stale rows (`D6`) — the right default for a UI cache.
- **`presentationStore`** (`internal/server/presentation.go`) is a clean read-through/write-through
  design: atomic tmp+rename writes, reload-after-write so the in-memory state never drifts from
  disk, sensible fail-open defaults on a missing/malformed profile. Input validation on the profile
  save handler (`internal/server/profilesettings.go:72-86`) whitelists `audience`/`language` before
  ever touching the filesystem, and the casual-audience dev-mode force-off is enforced server-side,
  not just in the UI.
- **`bootstrapEnv`** (`internal/server/bootstrap.go:169`) rejects quotes/newlines in `extra-prompt`
  before it becomes an env var consumed by a shell script — the right place to close that class of
  injection, and it's actually done here.
- Test coverage for the two areas I traced in depth matches the plans' test tables almost
  1:1 (`TestRegistryRefreshOnExternalEdit`, `TestRegistryAddPreservesExternalEdit`, `TestFingerprint`,
  `TestLoadPresentationProfile`, `TestPresentationStore_SaveProfileSelection`, etc. all present and
  exercising the documented cases).
- Consistent, disciplined commenting style throughout the new Go code — comments explain *why*
  (a decision, a non-obvious invariant), not *what*, matching `RULE-GOLANG-COMMENT-001`.

## Issues

### Critical (Must Fix)

#### 1. The ACDSL self-management gate (`acdsl/policy.json` mode) can be bypassed by an agent operating inside the very repo it's supposed to constrain

- **Files:** `internal/acdsl/policy.go:329-345` (`CheckPolicy`), `:195-217` (`policyBase`), `:63-76`
  (`basePolicy`); reachable via `cmd/acdsl/main.go:122-133` (`runCheck` calls
  `acdsl.LoadPolicy(*root)` then `acdsl.CheckPolicy(ctx, *root, policy)` with that same untrusted,
  working-tree-loaded value).
- **What's wrong:** `CheckPolicy` takes the *working-tree* `Policy` (freshly loaded from
  `acdsl/policy.json` on disk — a file an agent in a `gated`/`strict` repo can edit like any other)
  and uses two of its fields, `BaseRef` and `Mode`, before ever consulting the authoritative,
  base-branch copy of the policy:
  ```go
  func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
  	base, isBoundary, baseErr := policyBase(ctx, root, policy)   // policy.BaseRef is untrusted here
  	if baseErr != nil {
  		if policy.Mode == PolicyModeFree {                       // policy.Mode is untrusted here
  			return nil, nil // no base to gate against, nothing declared — free
  		}
  		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
  	}
  	policy, err := basePolicy(ctx, root, base, policy)            // *now* becomes authoritative
  	...
  ```
  Two exploitable paths follow directly from this ordering:
  1. **Unresolvable-base free-mode escape.** An agent sets `mode: "free"` and `base_ref` to any
     nonexistent ref string in the working `acdsl/policy.json`. `policyBase` fails to resolve
     (`baseErr != nil`); the check then reads the *tampered* `policy.Mode`, sees `"free"`, and
     returns `nil, nil` — no error, no diagnostics, `bin/acdsl check` exits 0. The self-management
     surface (including `policy.json` itself) is never diffed against anything.
  2. **Pre-policy base escape.** An agent points `base_ref` at any commit predating
     `acdsl/policy.json`'s existence in that repo's history (trivial: `git log`/`git tag` locally,
     no push needed). `basePolicy` finds `!exists` at that base and falls back to `return working,
     nil` — i.e. the tampered working policy becomes "authoritative", including a self-declared
     `mode: "free"`.
  Both require the agent to control only files already inside the working tree — no privilege
  beyond what `gated`/`strict` mode is explicitly designed to withstand (per the plan's own stated
  constraint: *"the enforcement must not be neutered by the thing it guards … no policy file that
  can sanction its own edit"*, `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/design/raw.md:16`).
  This directly contradicts the shipped docs: `acdsl/README.md` (new "Modes" section) states
  enforcement is "compiled into `bin/acdsl`, so no data file can neuter it" — but the data file
  *can*, via `base_ref`.
  - Note precisely what is **not** broken: a naive `mode: "free"` edit alone, with `base_ref` left
    unset (so it resolves to a real `main`/`origin/main` that already carries the shipped gated
    policy), *is* caught — `basePolicy` correctly overrides the tampered mode with the base's real
    `gated` mode, and `surfaceDelta` flags `policy.json` itself as changed. The bypass needs the
    `base_ref` field specifically; that field has no format/reachability constraint in
    `acdsl/policy.schema.json` (`type: string`, anything goes) and is read from the untrusted file
    before authority is established.
- **Why it matters:** this is the entire premise of the feature ("an agent … cannot touch PLAN
  rules, add validators, or weaken the policy — the loosy-goosy-rules failure mode is structurally
  closed", plan TLDR). As shipped, an agent in a `gated`/`strict` target repo can silence the
  authoritative check with a two-line edit to a file it already has write access to, and the
  PreToolUse hook (`cmd/hooks/acdsl-policy-guard.sh`) is explicitly documented as fail-open/advisory
  only, not a backstop for this.
- **How to fix:** resolve `base` using a *fixed* candidate order that does not read from the working
  policy at all (or only reads `base_ref` *after* it has been validated against the base-ref's own
  copy of the policy, e.g. require `base_ref` itself to be declared in the base's `policy.json` and
  never in the working one). Simplest correct shape: compute `base` from the fixed default chain
  (`origin/HEAD`→`origin/main`→`main`→`master`) unconditionally; only honor a working-tree
  `base_ref` override once you've independently confirmed (from the resolved default base) that
  overriding is legitimate — or drop the working-tree-sourced `base_ref` override entirely for
  gated/strict repos and make it base-only. Separately, `basePolicy`'s "`!exists` → fall back to
  working policy" branch should fall back to a **hardcoded free-mode `Policy{}`**, not to the
  caller-supplied `working` value, since `working` is exactly the value under adversarial control.
- **Process note:** the approved plan's own pseudocode for `CheckPolicy`
  (`plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/design/raw.md:189-220`) checked
  `policy.Mode == PolicyModeFree` as the *very first* line, with no base-policy resolution at all —
  an even simpler bypass than what shipped. The implementer clearly noticed this during
  implementation and added `basePolicy` as a fix (good instinct), but that's a real architectural
  deviation from the approved design that was never recorded — the plan's Changelog (which this
  plan, marked `done`, otherwise uses correctly for its one Q&A) has no entry for it. Per
  `RULE-PLAN-049` this should have been a Changelog row; as it stands, the approved design on file
  doesn't match what shipped, undetectable by future readers of the plan file. Please confirm this
  was intentional and that I haven't misread the flow.

### Important (Should Fix)

#### 2. `GitRevParse` is dead code

- **File:** `internal/acdsl/git.go:77-84`.
- New exported function, never called outside its own test (`git_test.go:13`). The plan's Changes
  §2 listed it as one of four new git helpers but nothing in `policy.go` ends up using it (`policyBase`
  uses `GitMergeBase` + `shellGitAbbrevRef` instead). Per `ACTION-REVIEW-QUALITY-004`
  (no unnecessary complexity/dead code) this should either be wired in where intended or removed.

#### 3. Plan-specified regression test not implemented (equivalent coverage exists, but not where documented)

- **File:** `internal/server/repos_test.go:689` (`TestReposDelete`).
- The repo-cache-reload plan's test table calls for `TestReposDelete` to gain a case proving "repo
  stays deleted after a second registry instance (fresh `NewRegistry` on same path) writes — cross-
  instance regression" (plan Tests table, row for `repos_test.go TestReposDelete`). That case isn't
  present; the closest coverage is `TestRegistryAddPreservesExternalEdit` at the registry-unit level
  (`internal/repos/registry_test.go:184`), which does prove the same mechanism but not through the
  HTTP handler path the plan named. Low risk since the underlying fix is verified, but worth a
  one-line confirmation that this was a deliberate substitution rather than an oversight.

### Minor (Nice to Have)

- `internal/server/presentation.go`: `saveProfileSelection` locates the German seed template via a
  cwd-relative path (`filepath.Join("settings", "claude_code", "presentation-profile.de.md")`,
  `presentation.go:107`). This only works because both the launchd plist and the Windows scheduled
  task pin `WorkingDirectory`/`WorkingDirectory` to the repo root — verified true for both current
  deployment paths — so this isn't a bug today, but it's an undocumented assumption shared with the
  pre-existing `claudeFragmentPath` default (`server.go:228`). A future deployment path that doesn't
  set cwd to the repo root will silently degrade (empty seed body, not a crash) rather than fail
  loudly.
- `internal/acdsl/policy.go`: `ruleMarkers`/`ruleDelta` silently ignore `ParseRules` errors (`workRules,
  _ := ParseRules(workLines)`), which is consistent with the plan's stated "base-side authoring
  violations are ignored" but also swallows a working-side parse error the same way — a rule file
  broken syntactically enough to fail `ParseRules` outright would produce zero delta entries for it
  rather than a diagnostic. Given `Check`'s own pipeline already runs `DiscoverRules`/parsing
  separately and would presumably catch a truly malformed file, this is likely fine, but wasn't
  explicitly reasoned about in the plan.

## Recommendations

- Add a `policy_test.go` case that constructs exactly the two bypass scenarios in Critical #1 (garbage
  `base_ref` + `mode: free`; `base_ref` pointing pre-policy) and asserts `CheckPolicy` still enforces
  the base's real mode — this is the kind of case a "finite validator set / can't sanction its own
  edit" feature should have from day one given how central the property is to the feature's purpose.
- Given the size of this release (9 independent plans in one diff), a per-plan review pass (this
  review did two in depth) would catch more than a single flat pass across all 178 files — worth
  keeping that as the review shape for future releases this size.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of the release (repo-cache invalidation, presentation-profile/i18n, routine
loader changes) is solid, closely tracks its plans, and is well-tested where I checked. The ACDSL
policy-modes feature — the release's other headline addition — ships with a working bypass of its
core enforcement guarantee that also contradicts its own documentation; that's a self-management/
security feature failing on its central property, not a peripheral bug, and should block merge of
that component until fixed (the rest of the release does not depend on it and could ship
independently).
