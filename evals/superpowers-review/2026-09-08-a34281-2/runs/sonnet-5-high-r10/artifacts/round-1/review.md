# Review: smine v1.1.4 → v1.2.1

Range reviewed: `b4981b88d5..a342811b4c` (178 files, +7314/-749), excluding `sessions/`, `evals/`,
`proposals/`, `plans/`. Plans checked against: `plans/v1-2-1/*` (9 plan dirs).

**Methodology note:** this is a large multi-feature release (9 independent plans: ACDSL
self-management policy, peek multi-profile ports, repo registry read-through cache,
repos/routines UI, plan-format viewer fixes, session context report, headless bootstrap
rewrite, presentation/language profile, railroad-review observability). I reviewed every plan's
TLDR/Decisions/Changes against its corresponding diff, read the full source of the
security/correctness-critical new subsystems in full (ACDSL policy engine, repo registry,
peek identity/install flow, bootstrap orchestrator, profile settings handler,
skill-invoke-gate hook), and spot-checked the remaining UI/template/skill-doc changes rather
than diffing every one of the 178 files line-by-line. I built, vetted and ran the full Go test
suite, and wrote an isolated reproduction test for the one finding I could not verify by
inspection alone.

**Important environment note:** the `arm-wt` checkout handed to me has an uncommitted, dirty
working tree relative to `a342811b` (`acdsl/registry.json`, `acdsl/policy.json`,
`acdsl/rules.acdsl`, several `context/` docs, `AGENTS.md` are all locally modified, apparently
by a self-dist/sync operation run against this checkout). Running the test suite directly in
`arm-wt` produces one failure (`TestDistShipsReachCoveredSliceWithBinaries`) that does **not**
reproduce on a clean worktree of `a342811b` — it's an artifact of that local dirt, not a defect
in the reviewed diff. All test/build verdicts below are from a clean worktree of the head
commit, not from `arm-wt`'s working tree.

## Strengths

- **`internal/repos/registry.go`** (read-through registry): clean fix for the documented
  last-writer-wins bug (FACT-REPO-ARCH-016). `Add`/`Remove` re-read the file before applying
  their delta, `refreshIfChanged` uses mtime+size instead of a TTL, writes are atomic
  (tmp + rename). Well-scoped, well-tested (`registry_test.go`), no over-engineering.
- **`internal/acdsl/policy.go`** improves on its own plan's pseudocode: the plan's `CheckPolicy`
  checked `policy.Mode == PolicyModeFree` on the *working-tree* policy before ever computing the
  base or reading the base-ref's committed policy — meaning a working-copy edit to
  `acdsl/policy.json` setting `mode: "free"` would short-circuit cleanly. The shipped
  implementation adds `basePolicy()`, fetches the base-ref's committed policy first, and treats
  *that* as authoritative before checking free-mode — closing an obvious self-weakening path in
  the plan's own design. Good catch by the implementer (see Critical #1 for the residual gap in
  the same area).
- **peek multi-profile identity check** (`cmd/configserver/main.go` `startPeek`/`peekIdentity`,
  `install.sh`): matches its plan closely; the degrade-visibly-over-serve-wrong-data design (D2)
  is implemented faithfully — a home mismatch on `/healthz` disables the peek endpoint rather
  than silently reusing another profile's peek.
- **`bootstrapEnv`** (`internal/server/bootstrap.go`) rejects quotes/newlines in the free-text
  "extra prompt" field before it's concatenated into a shell-script string that's later passed
  as a single argv element to `claude -p` — correctly scoped defense even though the downstream
  plumbing (`routine_run_claude "$@"`) never re-parses the string through a shell.
  `handleProfileSave` similarly validates audience/language against a fixed enum and forces
  `dev_mode` off server-side for the casual audience, not just in the UI.
- Build, `go vet`, and the full `go test ./...` suite are green on a clean checkout of the head
  commit; `cmd/tests/test_skill_invoke_gate.sh` passes.

## Issues

### Critical (Must Fix)

**1. ACDSL self-management policy is fully bypassable from an agent's own working copy —
defeats the feature's entire purpose.**

- File: `internal/acdsl/policy.go:329-345` (`CheckPolicy`)
- What's wrong: `CheckPolicy` resolves `base, isBoundary, baseErr := policyBase(ctx, root, policy)`
  using the **untrusted working-tree** `policy` (whatever `acdsl/policy.json` says on disk right
  now, attacker-controlled on an agent branch). If `policyBase` fails to resolve a base (e.g.
  `base_ref` is set to a nonexistent ref), the very next check is
  `if policy.Mode == PolicyModeFree { return nil, nil }` — again reading the **same untrusted**
  working-tree `policy`, not the base-ref's committed (authoritative) policy. `basePolicy()` —
  the function that fetches the committed, authoritative policy and is supposed to be
  consulted before any free-mode decision — is never reached on this path.
  Result: an agent on a `claude/*` branch in a **gated or strict** target repo can write
  `acdsl/policy.json` as:
  ```json
  {"mode": "free", "base_ref": "nosuchref"}
  ```
  and `acdsl check` (the binary described in the plan as "not bypassable via data files",
  D2/D4) returns **zero diagnostics and no error** — even though the committed base policy is
  gated/strict, the agent added a rule in a protected scope, and dropped an unsanctioned
  `acdsl/registry.local.json` overlay pointing a verifier at `true`. I verified this with an
  isolated reproduction against a clean worktree of the head commit:
  ```go
  // committed base: mode=gated, editable_scopes=[GOLANG]
  // agent's working copy: {"mode":"free","base_ref":"nosuchref"}
  //   + adds //acdsl:ACDSL-PLAN-999 (protected scope)
  //   + adds acdsl/registry.local.json repointing "gofmt" to "true"
  diagnostics, err := acdsl.CheckPolicy(ctx, root, loaded)
  // diagnostics == [], err == nil  <- BYPASS
  ```
  This directly contradicts the code's own documented invariant ("The base-ref policy is
  authoritative — a branch cannot weaken the mode by editing or deleting the working
  policy.json") and the plan's D10 ("an unresolvable base is a hard tool error, never a silent
  pass"). The existing test `TestCheckPolicyUnresolvableBase` only exercises `base_ref` bogus
  **with `Mode` left at the caller-supplied gated value** — it never combines the bogus
  `base_ref` with an attacker-set `mode: "free"`, so the gap has no test coverage.
  The fail-open `cmd/hooks/acdsl-policy-guard.sh` PreToolUse hook (the advisory layer) does deny
  `Write`/`Edit` to `acdsl/policy.json` when the mode is currently gated/strict, but per the
  plan's own D7 that hook is explicitly bypassable via `Bash` (`echo ... > acdsl/policy.json`,
  `sed -i`, etc.) — which is exactly why the plan calls the binary check "authoritative." That
  binary check is what this bug defeats.
  Since smine's own policy stays `free` by design (D8), this cannot manifest in smine's own
  `make audit`; it only fires in a *target* repo distributed with `dist_mode: gated` — i.e.
  exactly the repos this feature exists to protect. It is untestable by any CI in this repo
  because smine never runs `acdsl check` against its own gated policy.
- Why it matters: this is the load-bearing security boundary of a feature whose stated goal is
  "the loosy-goosy-rules failure mode is structurally closed." An agent (malicious, confused, or
  just running an exploratory edit loop) can trivially disable the entire self-management gate —
  surface freeze, scope restriction, verifier allowlist, and overlay-bypass check all together —
  with a single-line edit that requires no special git topology.
- How to fix: in `CheckPolicy`, the `baseErr != nil` branch must decide free-vs-hard-error using
  a value that cannot be forged by the same edit that's under test. Two options: (a) always
  attempt `basePolicy` reasoning by trying to load the base-ref's policy via a resolvable
  fallback path (e.g. treat "no base at all" — no `origin/HEAD`/`origin/main`/`main`/`master`
  resolves *and* no committed `acdsl/policy.json` exists anywhere reachable — as the only case
  that's legitimately "free"), or (b) simplest: when `base_ref` is explicitly set in the
  *working* policy but fails to resolve, always hard-error regardless of the working-tree mode —
  only fall back to "free" when `base_ref` is unset and *none* of the default candidates exist
  at all (a plausible bootstrap/shallow-clone case), which is a materially narrower and harder
  to forge condition. Add a test combining the two: attacker-set `mode: "free"` + unresolvable
  `base_ref`, against a committed gated base — expect a hard error, not a silent pass.

Related, lower-severity variant in the same function worth closing at the same time:
`isBoundary` is computed by comparing `GitBranch()`'s name string to the resolved base branch
name (`policy.go:195-219`). Two worktrees cannot normally have the same branch checked out, but
in a bare clone or an operator error this reduces the privilege boundary to "is my branch
literally named `main`" rather than any ancestry/identity check — worth a comment/test noting
the assumption, even if not worth hardening in this pass.

### Important (Should Fix)

**2. `cmd/hooks/skill-invoke-gate.sh` (new, 85 lines) ships with no governing plan.**

- Files: `cmd/hooks/skill-invoke-gate.sh`, `settings/claude_code/settings.json:179`,
  `cmd/tests/test_skill_invoke_gate.sh`
- What's wrong: this is new PreToolUse enforcement — it can deny *any* tool call in a session
  whose first user message carries a `REQUIRE-SKILL:<name>` marker until the named skill is
  invoked, and it denies raw `Read`/`cat` of context-dir files after invocation. It's registered
  in `settings.json` and shipped in the same commit as the ACDSL policy guard
  (`c1b52fb: cmd: bootstrap runner, skill-invoke gate, acdsl policy guard, configserver
  install-env, sync and worktree updates`). None of the 9 plans under `plans/v1-2-1/` describe
  it in a Changes section — the only mention anywhere in the plan set is a passing reference in
  `railroad-review-observability`'s `runbooks/trace.md`, and that plan's own Scope explicitly
  says **"Not changed: skill-context.sh / read-gate.sh — the hooks stay as-is"** and its Hot
  Items section says **"N/A — no hot-class code... the only code edits are string-level changes
  in railroad-review.js."** Per this repo's own doctrine (ACTION-REVIEW-SPEC-005 / concept
  audit, ACTION-CONCEPT-HOT-005 — new guard/validation logic needs an approved example in the
  plan first), a new hook that can deny tool calls session-wide is exactly the kind of addition
  that should trace to a reviewed plan entry, not land as a rider on an unrelated commit.
- Why it matters: it's shipped, tested, and wired up, so it's not dead code — but nobody
  reviewed its design against the doctrine that supposedly gates this exact kind of change. Its
  own coverage is also weaker than the ACDSL guard's: the Bash-detection only catches the exact
  two-word shape `cat <path>` (`set -- $command; [ "$#" -eq 2 ] && [ "$1" = "cat" ]`) — `head`,
  `less`, `python -c`, quoted paths, or `cat` with flags all fall through undetected. That's
  probably fine for what looks like an efficiency guard rather than a hard security boundary,
  but that judgment call was never written down anywhere for someone to confirm.
- How to fix: confirm with the implementer whether this was scoped under a plan not included in
  `plans/v1-2-1/`, or file the missing plan/Changes entry retroactively so the design intent
  (which sessions it applies to, what it's meant to catch vs. not) is recorded somewhere other
  than the hook's own comments.

**3. `install.sh` peek-port stale-kill loop can silently no-op on multiple listeners.**

- File: `install.sh` (new block after "A running peek survives configserver restarts...")
- What's wrong: `pid="$(lsof -tiTCP:"$peek_port" -sTCP:LISTEN || true)"` can in principle return
  more than one PID on one line (newline-separated); the subsequent `kill "$pid"` and
  `while kill -0 "$pid"` pass that multi-line string as a single non-numeric argument, which
  `kill` rejects — silently swallowed by the surrounding `|| true` / harmless loop exit — leaving
  the stale process running and the reinstall's "fresh peek respawns with current flags"
  guarantee (D6/DR3) unmet for that run.
- Why it matters: low likelihood (a single LISTEN socket on a port normally has exactly one
  owning PID), but if it ever does happen the failure mode is silent — no error is surfaced, and
  the operator has no signal that the update vector (install = peek update) didn't work this
  time.
- How to fix: iterate `for p in $pid; do kill "$p"; done`, or use `xargs kill` and drop the
  `|| true` swallow (or `echo`-log when the previous listener is still up after the wait).

### Minor (Nice to Have)

**4. `cmd/bootstrap/run.sh` unconditionally overwrites `.claude/settings.local.json` in the
main checkout.**

- File: `cmd/bootstrap/run.sh` ("Casual installs hide skills via user-settings skillOverrides…")
- What's wrong: `cp "$skill_overlay" "$repo_root/.claude/settings.local.json"` runs
  unconditionally whenever `$HOME/.config/claude-routine/skill-overrides.json` exists, with no
  backup of a pre-existing `settings.local.json`. The comment notes it's "gitignored so the
  orchestrate commit never picks it up," which suggests the file is treated as disposable, but
  that assumption isn't verified — a developer who hand-edits local settings on the main
  checkout (not a worktree) for unrelated reasons would lose them silently on the next bootstrap
  run.
- How to fix: back up an existing file before overwrite, or merge the skill-overlay keys into
  the existing JSON instead of a blind `cp`.

**5. Deviation not flagged as a decision:** `startPeek`/`peekIdentity`
(`cmd/configserver/main.go`) added a `backLink` parameter and `--back-link` peek argument not
present in the peek-multiple-profiles plan's pseudocode. Looks like a reasonable small addition
(peek dashboard linking back to the config server), but plans in this repo are supposed to be
binding contracts (`context/rules/plan.md` RULE-PLAN-077) — worth a one-line Changelog entry on
that plan rather than a silent signature addition.

## Recommendations

- Add the missing test for finding #1 (`mode:"free"` + unresolvable `base_ref` against a
  committed gated base) before this ships to any target repo running `dist_mode: gated`; it's
  the one path that actually exercises the feature's threat model.
- Since smine's own `acdsl/policy.json` stays `free`, this repo's own CI can never catch a
  regression in the gated/strict code paths. Consider adding a target-simulation test to
  `make audit` (or a dedicated `TestCheckPolicyGatedRegression`-style suite already exists —
  just needs the missing case) rather than relying solely on `internal/acdsl/policy_test.go`
  being complete by inspection.
- The `arm-wt` checkout's dirty working tree (self-shipped `acdsl/registry.json`,
  `acdsl/policy.json`, `context/` docs) should be reconciled or reset before further review
  rounds against this checkout — it caused one false-positive test failure in this pass and
  will keep doing so for anyone running `go test` directly in `arm-wt` instead of a clean
  worktree of the commit.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The release is broad but mostly solid — the registry read-through fix, peek
multi-profile identity check, and profile-settings validation are well-designed and correctly
implemented. But the ACDSL self-management policy engine — a security-shaped feature whose
entire purpose is to stop an agent from weakening its own governance — has a verified,
one-line-edit bypass that defeats it completely in exactly the gated/strict target repos it's
meant to protect, with zero test coverage of the failing case. That must be fixed (and covered
by a test) before this ships to any repo that will actually run in gated or strict mode. The
undocumented `skill-invoke-gate.sh` hook should also get a plan entry or explicit sign-off
before merge, though it's not blocking on its own.
