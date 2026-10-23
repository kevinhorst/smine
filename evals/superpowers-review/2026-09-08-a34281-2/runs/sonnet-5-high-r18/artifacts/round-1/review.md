# Review: smine 1.1.4 → 1.2.1 (b4981b88..a342811b)

Scope reviewed: full diff excluding `sessions/`, `evals/`, `proposals/`, `plans/` — 179 files, ~7300
insertions. This is a large, multi-plan release (10 plan documents under `plans/v1-2-1/`). Given the
size, I reviewed in passes: (1) read all 10 plan documents in full, (2) diffed every Go package touched
against its plan's Changes section, (3) diffed the shell scripts (worktree squash-detection, bootstrap
wrapper, hooks) against their plans, (4) ran `go build ./...` and `go vet ./...` against the head
commit (both clean), (5) spot-checked templates and skill/routine prose against the plans' Decisions
tables. I did not execute `make audit` (acdsl gates, fixtures, full test suite) or the shell test suite —
only static review and compile/vet.

I did not spawn any subagents for this review; it is single-pass by me end to end.

## Strengths

- **Registry read-through (`internal/repos/registry.go`) and status-cache fingerprinting
  (`internal/server/statuscache.go`, `internal/repos.Fingerprint`) are implemented exactly to their
  plans** (`repo-cache-reload`), including the subtle "stat before read" ordering to avoid missing a
  racing write, and the deliberate choice to leave `DropBranches`' fingerprint stale so the next plain
  load re-converges to scan truth. Clean, well-commented, matches the stated invariants precisely.
- **Squash-containment layer in `cmd/worktrees/_lib/verdict.sh`** (`git merge-tree --write-tree`
  content-equality check) is a sound, conservative addition — it can only promote a branch to "safe" on
  proven content containment, never weakens the dirty/untracked checks. The row-level verdict cache
  added in the same area (`agent-status-cache/`, digest-keyed, `v2`-salted) is well-isolated per branch
  (each `xargs -P` worker writes only its own row file, `mktemp`+`mv` avoids partial writes).
- **`cmd/bootstrap/run.sh`** is a faithful, careful implementation of the wrapper-orchestration plan —
  bash 3.2/Git-bash discipline is respected throughout (`${flags[@]+"${flags[@]}"}` guards, no heredocs
  in `$()`, `if`-form expected-failure checks), and it correctly fixes the four observed production
  failures (missing `--allowedTools`, dead Skill tool headless, prose-relayed `--since`, stranded
  finalization).
- **Self-mining exclusion and Dev Mode threading** (`smine-batch` Select rule, `presentation.go`
  `DevMode`, both wrapper scripts) is default-deny by construction exactly as designed — a forgotten
  flag cannot cause self-mining, and the casual-forces-off rule is enforced at the one point that
  matters (the handler, not the store).
- Test coverage on the new Go surfaces is genuinely good, not token: `presentation_test.go` (254
  lines), `profilesettings_test.go` (219), `repos_test.go` (+343), `registry_test.go` (+215),
  `policy_test.go` (327) all exercise the documented edge cases (external-edit races, cross-instance
  resurrection, unresolvable-base hard errors, overlay bypass, task-contract exemption).
- `go build ./...` and `go vet ./...` are clean at head.

## Issues

### Critical (Must Fix)

#### 1. ACDSL policy gate can be silently defeated by an attacker-controlled working copy — `internal/acdsl/policy.go:329-346`

`CheckPolicy` resolves the policy base and, on failure, decides between a hard error and a silent
"nothing to gate" free-mode pass using the **caller-supplied, working-tree-read `policy` parameter** —
not the base-ref-authoritative policy the rest of the function (and its own doc comment) claims to
enforce:

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)
	if baseErr != nil {
		if policy.Mode == PolicyModeFree {
			return nil, nil // no base to gate against, nothing declared — free
		}
		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
	}
	policy, err := basePolicy(ctx, root, base, policy)   // re-derives the AUTHORITATIVE policy — too late
	...
```

`policy` here comes from `cmd/acdsl/main.go`'s `acdsl.LoadPolicy(*root)`, which does a plain
`os.ReadFile` of the **working tree's** `acdsl/policy.json` — unstaged, uncommitted edits included, and
never checked against `HEAD` or the base. `policyBase` also takes its `base_ref` candidate from this
same untrusted `policy.BaseRef`, and when `BaseRef != ""` it becomes the *sole* candidate (no fallback
chain) — see `policyBase` lines 195-219.

Exploit: on a `gated` or `strict` repo, edit the working copy of `acdsl/policy.json` to
`{"mode": "free", "base_ref": "nosuchref"}` (a raw file write via `Bash`, `sed -i`, or any tool other
than `Write`/`Edit`/`NotebookEdit` — the advisory `acdsl-policy-guard.sh` hook only gates those three
tool names, and the plan explicitly documents it as "fail-open... the authoritative layer is the
binary"). `policyBase` then fails to resolve `nosuchref` (guaranteed, since it's the only candidate),
`baseErr != nil`, and because the tampered `policy.Mode == "free"`, `CheckPolicy` returns `nil, nil` —
no diagnostics at all. `surfaceDelta` (which would have flagged the `acdsl/policy.json` self-edit) never
runs, because the function returns before reaching it.

This defeats the exact threat model D2/D4 of the `acdsl-policy-modes` plan describe ("the enforcement
must not be neutered by the thing it guards"; "agent worktrees on `claude/*` branches can never
self-escalate") and directly contradicts `CheckPolicy`'s own doc comment ("a branch cannot weaken the
mode by editing or deleting the working policy.json"). The existing test suite does not catch this:
`TestCheckPolicyUnresolvableBase` (`policy_test.go:280`) only exercises the hard-error branch with
`gatedPolicy()` (mode always `gated`); no test combines a bad `base_ref` with a tampered `mode: free`.

**Fix:** the base-resolution failure path must never trust the untrusted working file's `Mode` to
decide free-vs-error. Either (a) always hard-error when base resolution fails and a policy file is
present at all (only truly-absent `acdsl/policy.json` — i.e. `LoadPolicy`'s own "file not found" case —
should imply free), or (b) resolve `base_ref` only from a source you already trust before you trust
`Mode` (e.g. try the default candidate chain unconditionally first, and use the working `BaseRef` only
as an additional candidate, never the sole one) — the fallback-chain the plan already specifies for the
no-`BaseRef` case is the safe behavior; forcing `BaseRef` to be the sole candidate at all is what enables
the attacker-chosen-failure trick. This needs a decision from the operator on which fix he wants; either way it
needs a regression test that combines a tampered `mode: free` with an unresolvable/attacker-controlled
`base_ref`.

### Important (Should Fix)

#### 2. Two substantial, security/behavior-relevant additions in this diff trace to no plan

Per `ACTION-REVIEW-SPEC-005` / `ACTION-IMPL-ARCH-001` ("every new concept traces back to a request"),
these should be confirmed as intentional and either get a retroactive plan note or be flagged as scope
creep:

- **`cmd/hooks/skill-invoke-gate.sh`** (new, 85 lines) — a `PreToolUse` hook that parses the session
  transcript for a `REQUIRE-SKILL:<name>` marker in the first user message, denies every tool call until
  `Skill(<name>)` (or a prompt-start `/<name>`) has fired, and afterwards denies direct `Read`/`Bash cat`
  of files under the repo's context directory (with a `.guides[]` exemption). This is genuinely
  session-behavior-shaping and worth documenting, but none of the 10 plans under `plans/v1-2-1/` cover
  it — the only hit for `skill-invoke-gate` outside the code itself is an incidental mention in
  `railroad-review-observability`'s `runbooks/trace.md`, and that plan's own Hot Items section states
  "N/A — no hot-class code... no validation/guard changes" and its Scope explicitly excludes hook
  changes ("`skill-context.sh` / `read-gate.sh`: the hooks stay as-is"). `git log` confirms this file was
  introduced within the reviewed range (one squashed commit: `c1b52fb`), so it isn't inherited baseline
  either.
- **The row-level verdict cache in `cmd/worktrees/print_agent_worktrees_status.sh`**
  (`agent-status-cache/`, `WTS_CACHE_DIR`, `WTS_CANDIDATES_DIGEST`, per-branch cache files, stale-row
  pruning) is also new in this exact range — `git show b4981b88:cmd/worktrees/print_agent_worktrees_status.sh
  | grep cache` returns nothing. The `repos-routines-improvements` plan's baseline (C3!) *describes this
  cache as pre-existing* and only patches its key with a `|v2` salt — meaning the cache's actual design
  (concurrency-safety, invalidation shape, pruning) was never itself put through a plan's Decisions
  table. The implementation looks sound (each parallel worker only ever touches its own file; the digest
  correctly covers all candidate tips so any candidate movement invalidates every row), but it is an
  untraced concept per the same doctrine as #1's design gap, and it is exactly the kind of thing
  `ACTION-CONCEPT-HOT-002`/`005` (concurrency, guard logic) is supposed to have gated with an approved
  example *before* it was written.

Neither of these looks like a functional bug in isolation, but both are meaningful, unplanned mechanism
additions bundled into a "grab-bag" commit (`c1b52fb: cmd: bootstrap runner, skill-invoke gate, acdsl
policy guard, configserver install-env, sync and worktree updates`) — worth a direct confirmation from
the operator that both were intended as part of this release and don't need their own plan/review pass.

#### 3. Undocumented identity-field addition on the repo Add form — `internal/server/repos.go:564-568`, `templates/repos_index.html`

The `language-style-setting` plan's C16/G17 are explicit that `Repo.Name` is load-bearing (URL segment,
dedup key, roster identity) and must stay derived from the folder — only `Label` (display-only) is meant
to be user-editable: *"a display field must not touch identity"*. The shipped `repos_index.html` add
form, however, adds a **`name`** input (`<input type="text" name="name" ... title="registry name — ...
edit to resolve a name collision">`) that lets the user directly override `Repo.Name`, and
`handleReposAdd` honors it (`name := strings.TrimSpace(r.FormValue("name")); if name == "" { name =
filepath.Base(path) }`). `Repo.Validate()` still enforces the character pattern and the `default`/
`archived` reservation, so this isn't unsafe, but it's a real deviation from the plan's stated design
intent (identity should be derived, not form-editable) with no corresponding plan changelog row. Please
confirm this was a deliberate scope addition (it does look like a reasonable answer to a real problem —
name collisions on `filepath.Base` — but it should be recorded as a decision, not shipped silently).

#### 4. `handleProfileTest`'s call mechanics diverged from the approved plan text without a changelog entry — `internal/server/profilesettings.go:25-30, 176-219`

The `smine-profile-settings` plan's D8/D9 specify a fixed prompt byte-for-byte mirroring
`handleWelcomeVerifyToken`, with no model override. The shipped code instead (a) rewrites the prompt
substantially to defeat an observed `error_max_turns` failure ("This is a style check, not a task: do
not use any tools...") and (b) adds a `styleTestTarget()` helper that resolves the model from the
nightly routine's `ROUTINE_MODEL` env rather than the CLI default. Both changes read as legitimate,
well-reasoned fixes for real failures (the comments explain the `error_max_turns` bug and the 429 risk
from an uncredited default model) — but `smine-profile-settings`'s own Changelog table (the mechanism
`RULE-PLAN-049` exists for exactly this) has no row recording either change, and the later
`bootstrap-profile-improvements` plan only *references* `styleTestTarget` as if it were already an
established pattern, rather than documenting its introduction. Not a functional problem, but a process
gap worth closing so a future reader of the frozen plan isn't misled about what shipped.

### Minor (Nice to Have)

#### 5. Discarded background-process error — `internal/server/bootstrap.go:114` (`cmd.Wait()`)

```go
go func() {
    cmd.Wait()
    cancel()
    ...
```

`cmd.Wait()`'s return value is silently dropped, so a bootstrap run that exits non-zero (crash, killed by
the 6h deadline) leaves no server-side trace beyond whatever the child wrote to the log file — the
in-memory `bootstrapRun` state just flips `running = false` either way. Logging the exit status (even
just `log.Printf` on non-nil error) would make `handleWelcomeBootstrapStatus`'s "bootstrap finished" vs.
"bootstrap crashed" distinction possible from server state instead of requiring a log-file read.

#### 6. `profilesettings.go`'s `styleTestPrompt` asks the model not to "use any tools or read any files" but the profile-test call is still made with the full default permission surface

Low-risk (single `--max-turns 1` call, and the prompt is fixed/trusted, not user input), but if the
prompt engineering ever needs another iteration, consider `--disallowedTools` as a second layer rather
than relying purely on prompt compliance — the same class of problem (`error_max_turns`) that motivated
the current prompt rewrite could recur under model updates.

## Recommendations

- Add a `policy_test.go` case combining a tampered working-tree `mode: free` with either (a) a
  deliberately unresolvable `base_ref`, or (b) simply `origin/HEAD`/`main` genuinely absent (shallow
  clone) — both should hard-fail in gated/strict, and the fix for #1 should make that provably true.
- Retroactively note `skill-invoke-gate.sh` and the `agent-status-cache/` mechanism in a plan (even a
  short change-plan after the fact) so their design rationale isn't only recoverable from source
  comments — this also gives them their own review pass with the Hot Items / concurrency gates they'd
  have gone through if flagged as hot-class up front.
- Given the release bundles 10 independent plans into one very large diff and at least one squashed
  "grab-bag" commit spanning unrelated concerns (`c1b52fb`), consider whether the next release can keep
  commits scoped closer to one plan each — it would have made this review (and any future `git bisect`)
  meaningfully faster without changing anything about the plans themselves.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of the release (registry/cache rearchitecture, bootstrap wrapper rewrite,
squash-detection, dev-mode/self-mining exclusion, profile settings) is well-planned, faithfully
implemented, and well-tested — I found no correctness bugs in any of it. But the ACDSL self-management
policy gate (Critical #1) has a real, verified bypass that undermines the entire feature's stated threat
model in exactly the gated/strict repos it exists to protect, and two more mechanisms shipped without
plan traceability (#2). The policy bypass should block merge until fixed or explicitly accepted as a
known limitation; the rest are confirm-and-document items that don't need to block the release.
