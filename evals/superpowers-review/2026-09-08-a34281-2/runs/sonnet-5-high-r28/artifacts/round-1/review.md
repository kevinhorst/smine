# Review: smine 1.1.4 → 1.2.1 (b4981b88..a342811b)

Scope: 178 files, ~7,300 insertions / 750 deletions, across config server, routines,
railroad-review, and ACDSL. Reviewed in passes against the nine plan documents under
`plans/v1-2-1/`, prioritized by risk (ACTION-REVIEW-RISK-001): the ACDSL self-management
gate, the repo registry/status-cache rewrite, the bootstrap orchestration wrapper, and
sessions-folder file operations got full read-throughs with code traced against every
plan Decision; template/CSS/i18n-catalog/SKILL.md-prose changes were spot-checked rather
than read exhaustively given the diff's size. Read-only — no files were changed.

## Strengths

- The ACDSL policy engine (`internal/acdsl/policy.go`) is well-structured: free-mode
  short-circuits cleanly, git helpers are properly wrapped through `internal/shell.Run`
  (no raw `exec.Command`), and the rule-delta computation correctly treats an
  unparseable base or unresolvable ref as a hard error rather than silently degrading
  to permissive behavior. Test coverage (`policy_test.go`) matches the plan's table
  almost case-for-case.
- The repo registry rewrite (`internal/repos/registry.go`) is a clean read-through
  design: `readRegistrySnapshot` stats before reading (so a race is caught on the next
  poll, not missed), `Add`/`Remove` re-read the file under the lock before mutating so a
  stale in-memory copy can never resurrect a deleted entry, and `refreshIfChanged`'s
  benign double-reload race is explicitly reasoned about. This is exactly the
  read-modify-write pattern the plan called for.
- `statusCache`'s fingerprint invalidation (`Fingerprint` + `DropBranches` keeping the
  *pre-removal* fingerprint deliberately) is subtle and correctly reasoned — the code
  comments explain why keeping the stale fingerprint is the fix, not a bug.
- `internal/sessions/sessions.go`'s archive/unarchive/delete trio has solid path-safety:
  `validateScopeName` rejects empty, `.`, `..`, containers, and any name that survives a
  `filepath.Base` round-trip differently — reserved-name checks are duplicated
  consistently between the registry (`Repo.Validate`) and the sessions store.
- The squash-containment layer in `cmd/worktrees/_lib/verdict.sh` (a data-loss-adjacent
  change — it feeds the worktree removal safety gate) ships with both a positive test
  (`test_squash_transfer_reads_contained`) and a negative test
  (`test_edited_squash_stays_unpicked`) plus a removal-gate integration test
  (`test_squash_contained_branch_removed_without_force`). The containment check itself
  (conflict-free `git merge-tree` whose result tree equals the candidate's own tree) is
  a sound, conservative primitive: it can only fail toward "still unpicked," never
  toward a false "safe."
- The bootstrap wrapper rewrite (`cmd/bootstrap/run.sh` + `internal/server/bootstrap.go`)
  correctly implements the plan's late pivot away from a single headless-orchestrated
  skill (which had failed in production per the plan's own root-cause section) to a
  wrapper-orchestrated per-stage design mirroring `smine-nightly/run.sh`. The
  `additionalDirectories` reconciliation was correctly moved from the orchestrator
  skill (originally planned) into the shell wrapper once discovered that headless
  agents can't write `~/.claude/settings.json` — a good example of catching a planned
  design's infeasibility during implementation and adapting cleanly.
- `config.LoadFragment` centralizing `ExpandInstallMarkers` means the drift-badge
  comparison path (`sectionOverridden`) gets marker expansion "for free" without a
  separate call — a good single-point fix.

## Issues

### Critical (Must Fix)

#### 1. The ACDSL self-management gate can be trivially bypassed via `base_ref`

**File:** `internal/acdsl/policy.go:195-219` (`policyBase`), `internal/acdsl/policy.go:63-76`
(`basePolicy`), `internal/acdsl/policy.go:329-345` (`CheckPolicy`)

The whole point of `gated`/`strict` mode is that "no data file can neuter it" —
that's the literal claim in `acdsl/README.md:47` ("enforced by acdsl check … so no data
file can neuter it"). But `CheckPolicy` resolves which commit is authoritative using the
**working copy's own, agent-editable** `acdsl/policy.json`:

```go
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)   // policy = the untrusted working-copy struct
	...
	policy, err := basePolicy(ctx, root, base, policy)           // re-reads at `base`, falling back to `policy` (working copy!) if absent there
	...
	if policy.Mode == PolicyModeFree {
		return nil, nil                                          // short-circuits BEFORE surfaceDelta ever runs
	}
```

`policyBase` honors `policy.BaseRef` when set (`policy.go:197-199`), and `policy` here is
whatever `acdsl/policy.json` says on the agent's own branch — a file the agent can edit
with `Bash(sed …)` or a heredoc (the `acdsl-policy-guard.sh` hook only intercepts the
`Write`/`Edit`/`NotebookEdit` tools, never `Bash`, and is explicitly documented as
"fail-open… fast feedback," not the authoritative layer).

An agent on a `gated` (or `strict`) repo can escape entirely with:

1. Find any commit that predates the introduction of `acdsl/policy.json` in this repo's
   history — the very first commit always qualifies, and is trivially discoverable
   (`git log --reverse | head -1`).
2. Set `"base_ref": "<that sha>"` in the branch's own `acdsl/policy.json`.
3. `basePolicy` calls `GitShowFile(ctx, root, base, "acdsl/policy.json")` at that commit
   → `exists == false` (the file didn't exist yet) → `basePolicy` returns the **working**
   policy verbatim (`policy.go:68-70`).
4. Back in `CheckPolicy`, `policy.Mode` is now whatever the attacker set (e.g. `"free"`)
   → immediate `return nil, nil`, **before `surfaceDelta` ever runs** — so the very
   surface-diff check that would have flagged the edited `base_ref` never executes.

The result: `acdsl check` (and therefore `make audit`) reports zero policy violations
regardless of what rules, verifiers, or registry entries the branch adds — completely
defeating `gated`/`strict` mode. This isn't a theoretical edge case; it requires no
special git knowledge beyond `git log`, and it's reachable through the agent's normal
Bash access (no tool restriction needed, since the guard hook doesn't cover Bash writes
anyway).

**Why it matters:** this feature's entire purpose (per its own plan, "acdsl-policy-modes,"
marked done) is to structurally prevent an agent from loosening its own rule system.
The bypass defeats that purpose completely, and — worse — it's live in this repo right
now: `acdsl/policy.json` here is `mode: "gated"` (see Important #1 below), so the gate
this repo relies on for its own GOLANG/PYTHON/SHELL/SQL/JSON rule discipline is
currently bypassable by any agent working on a `claude/*` branch.

**Fix direction:** the base commit used for `surfaceDelta`/`ruleDelta` must never be
chosen from data that is itself part of the diffed surface. Resolve the base using only
the fixed, non-agent-controlled candidate list (`origin/HEAD`, `origin/main`, `main`,
`master`) first; if `base_ref` is to remain an override at all, it must come from
outside the working tree (e.g. an environment variable set by the invoking harness, not
a JSON field the branch under test controls), or the resolved `base` commit must itself
be independently validated to be an ancestor reachable only through one of the fixed
candidate refs. At minimum, `basePolicy`'s "no policy at base → trust the working copy"
fallback must never apply when `base_ref` was taken from the working copy rather than a
harness-supplied value.

### Important (Should Fix)

#### 2. Shipped `acdsl/policy.json` deviates from the approved plan (`mode: gated`, no `dist_mode`) — confirm intentional

**File:** `acdsl/policy.json`

Plan `2026-08-26-acdsl-policy-modes-done-a7cd7bf/design/raw.md`, Decision D11 and D8,
explicitly decided: `mode: "free"` for this repo (smine) + `dist_mode: "gated"` for
targets, specifically *because* "smine must stay free (generation flow commits registry
entries on branches) while targets default gated" — the plan calls out that a gated
smine would red-flag its own sanctioned generation flow (F14!).

The committed file is:

```json
{
  "allow_task_contracts": true,
  "editable_scopes": ["GOLANG", "PYTHON", "SHELL", "SQL", "JSON"],
  "mode": "gated"
}
```

There is no `dist_mode` field at all, and `mode` is `gated` directly — meaning this
repo now gates itself (modulo the bypass in #1), and `writePolicy` (`dist.go:243-245`,
`if policy.DistMode != "" { policy.Mode = policy.DistMode }`) ships the *same* `gated`
mode to every target, since there's nothing in `DistMode` to rewrite from. This is the
opposite of the recorded decision and risks exactly the failure mode D11 was written to
avoid: smine-apply's own routine-branch commits that add new registry/verifier entries
would be flagged by `gated` mode's verifier-allowlist and surface-freeze checks. Please
confirm whether this was a deliberate late decision (and if so, update the plan's
Changelog and D11/D8 to reflect it) or whether it's a rollout gap — the plan's own
`writePolicy` doc comment ("mode rewritten from dist_mode … falling back to the source
mode") suggests the `free`/`dist_mode` split was still assumed to be the shipped state.

#### 3. "Apply overrides" can bake a literal port into the tracked settings fragment, destroying the peek marker

**File:** `internal/server/config_sync.go:37-82` (`handleConfigSyncOp`), `:116-142`
(`copyFileAtomic`)

`settings/claude_code/settings.json:6` carries `"OTEL_EXPORTER_OTLP_ENDPOINT":
"http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"` — a template marker meant to be expanded
per-profile at sync time (peek-multiple-profiles plan, D3). The revert direction
(fragment → live) correctly expands the marker before writing
(`expandMarkers := revert && target != catalog.TargetCodex`, line 50). But the forward
direction — "Apply overrides," live → fragment, an existing button unrelated to this
release but now interacting with the new marker — copies the live file's raw bytes
straight into the **repo-tracked** fragment with no re-templating:

```go
src, dst := live, fragment           // apply-overrides direction (revert==false)
...
output, err := copyFileAtomic(src, dst, expandMarkers)   // expandMarkers is false here
```

Since the live `~/.claude/settings.json` was populated by `sync_settings.sh` with the
marker already expanded to a real port (e.g. `42442`), clicking "Apply overrides" writes
that literal, machine-specific port directly into
`settings/claude_code/settings.json` — permanently replacing `{{PEEK_CONTROL_PORT}}`
in the repo's committed template with one profile's own port. The next `git commit`
from that flow (or the next person reading the fragment as a template) inherits a
hardcoded value instead of the marker, silently breaking multi-profile port isolation
for everyone who syncs from that fragment afterward.

**Fix direction:** either re-collapse the live port back to the marker before writing
into the fragment (the inverse of `ExpandInstallMarkers`), or block/warn when "Apply
overrides" would write a marker-bearing section, since that direction is fundamentally
lossy for any config key that contains an install-specific marker.

### Minor (Nice to Have)

#### 4. `skill-invoke-gate.sh`'s Bash-read detection is trivially bypassed by quoting

**File:** `cmd/hooks/skill-invoke-gate.sh:64-73`

The context-file-read guard only recognizes an unquoted, two-token `cat <path>` Bash
invocation; any command containing a `"`, `'`, `(`, `` ` ``, etc. is excluded from
classification entirely (falls through with `target=""`, i.e., not denied). A perfectly
natural `cat "docs/rules/go.md"` (quoted, as most agents write it) bypasses the
"don't re-read injected context" nudge. Low severity — this hook is documented as a
cost/duplication guard, not a security control, and the fail-open posture is
intentional — but worth tightening the pattern match (e.g. `case "$command" in "cat "*)`
after stripping matched quotes) if the intent is for it to actually catch the common
case.

#### 5. `session-context-report` plan present but unimplemented in this range

**Location:** `plans/v1-2-1/session-context-report/` vs. `internal/server/sessionreport.go`
(does not exist at either endpoint of the diff)

The plan documents a full context-filter system for a session report page
(`contextRecord`, `sessionreport.go`, `session_report.html`), but none of those files
exist in this repo at `b4981b8` or `a342811`, and none of the referenced source files
(`contextdocs.go`, `acdsl.go` `.URL()` signature change) appear in the diff. This looks
like a plan that predates or is independent of this release's scope rather than a
regression — flagging only so it's confirmed as deliberately out of scope for 1.2.1
rather than a dropped deliverable.

## Recommendations

- Treat finding #1 as release-blocking for any repo currently running `gated`/`strict`
  mode (including this one, per finding #2) — the feature is currently providing a
  false sense of security rather than an actual boundary.
- Add a regression test for #1 directly: a fixture repo where the agent branch edits
  `base_ref` to point at a pre-policy commit and asserts `CheckPolicy` still reports the
  surface violation. The existing `policy_test.go` table (`TestCheckPolicy`) is close in
  shape to this but doesn't include a case exercising an attacker-supplied `base_ref`.
- Given the size of this diff (178 files across nine independent plans), a follow-up
  pass focused solely on the SKILL.md prose changes (casual lockout, self-mining
  exclusion, jargon rules) would be worthwhile — those are LLM-behavior contracts with
  no executable test harness, and this review only spot-checked them for internal
  consistency (flag names, wording) rather than verifying an agent actually follows them
  under the described conditions, which the plans themselves acknowledge is untestable
  in this repo's CI.

## Assessment

**Ready to merge?** No — not until Critical #1 is fixed (or the repo's own
`acdsl/policy.json` mode is reverted to `free` per the approved plan, which would at
least remove the live exposure while the enforcement design is hardened).

**Reasoning:** the bulk of this release (registry/cache concurrency, bootstrap
orchestration, sessions archival, casual-mode rename sweep) is carefully executed and
matches its plans closely, with good test coverage on the highest-risk mechanical
pieces (squash detection, registry read-through). But the ACDSL self-management gate —
a security/data-integrity boundary this repo now depends on for its own rule
discipline — is bypassable by design, and that bypass is exploitable through an agent's
ordinary Bash access rather than requiring any unusual privilege.
