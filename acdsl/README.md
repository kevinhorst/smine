# ACDSL — the standing ablation loop

Full specification: `docs/acdsl-spec.md` (PDF: `docs/acdsl-spec.pdf`).

Operational contract for rule-level context optimization (roadmap §3b). Rule declaration,
projection, and the gate itself are documented in `cmd/acdsl/main.go`; this file covers only
the loop: delivery flag, verdict log, and the eviction procedure.

## Reach

- `reach=` declares a rule's deployment reach (`internal/reach` grammar): `global` (would
  bind in any repo), `none` (disabled), or a comma-separated repo-name list (deploys
  exactly to those targets; names = target dir basename). There is no self-keyword — this
  repo is named `smine` (the default), an ordinary name that never matches a sync target
  and therefore never deploys. The `reach="global"` set is the manifest for the future
  distribution workstream.
- `reach="none"` disables a rule: `check` skips it, it never projects, plan-time
  resolution ignores it, and it ships nowhere. Fixtures stay runnable so re-enabling is
  cheap; the UI shows a `disabled` badge and an editor (empty input = disabled). Apart
  from the `none` skip, `check`/`fixtures` never consult the field.

## Distribution

- `acdsl dist -target <name> -dest <dir>` ships a target's gate slice: the reach-covered
  doctrine rules (marker lines verbatim under a synced-from-smine header), the registry
  subset those rules name with argv rewritten to `bin/verifiers/<name>`, and prebuilt
  binaries — `bin/acdsl` plus one per rewritten entry. Invoked by `sync_context.sh`
  (`--acdsl`, default on; remembered as `deploy.acdsl`).
- The deployed read hook activates automatically (it probes `acdsl/registry.json` and
  prefers `bin/acdsl`); target check runs log to the shared home verdict sink.
- Targets own their side: rules in any `*.acdsl` file, verifier overrides in
  `acdsl/registry.local.json` (merged over the baseline by name — any executable that
  exits 0/1 qualifies; the registry name is the contract, the binary one implementation).
- The no-match contract stands everywhere: a doctrine anchor matching nothing is tool
  breakage (the typo guard). Dist therefore ships a rule only when the target has files
  its anchor matches — skipped rules are reported and a later re-sync picks them up.
  A rule whose anchor is smine-specific stays a smine entry (reach default) until its
  anchor is made portable.
- Sync never touches a target's `.gitignore` — committing or ignoring `bin/` is the
  target's call.

## Delivery flag

- `projected="false"` on a rule marker makes the rule **gate-only**: its prose leaves every
  prompt-side channel — projection blocks (`project -file`, and therefore the Read hook) and
  plan-time resolution (`project -plan`). Absent field means `projected="true"`.
- `check` and `fixtures` never consult the flag — enforcement is identical either way.
  Enforcement is the constant; delivery is the variable.
- Prompt-side output stays pure: no "also enforced" hints anywhere. A leak-through note would
  itself be a delivery channel and contaminate the ablation arm.
- A flip is one field edit plus a commit. A rule flipped to gate-only self-heals stale
  projections on the next read (empty delivery strips the block).

## Verdict log

- Every `check` run that reaches verifier execution appends one JSONL record.
- Sink: `$ACDSL_VERDICTS_PATH`, else `~/.claude/acdsl/verdicts.jsonl` — home-anchored because
  pool worktrees are destroyed with their sessions; the loop's data must outlive them.
- `ACDSL_VERDICTS_ENABLED=0` disables logging (the smoke suite exports this: its deliberate
  reds would poison the stats).
- Best-effort contract: a logging failure is a stderr warning; the gate's exit code never
  depends on it.
- Record shape (one line per run):

```json
{
  "ts": "2026-08-07T12:00:00Z",
  "root": "/abs/worktree",
  "branch": "claude/slug",
  "session": "optional CLAUDE_SESSION_ID",
  "outcome": "clean",
  "rules": [
    {
      "id": "ACDSL-GOLANG-EXEC-001",
      "projected": true,
      "violations": 0
    }
  ],
  "diagnostics": [
    {
      "id": "ACDSL-GOLANG-EXEC-001",
      "message": "file.go:12: ..."
    }
  ]
}
```

- Zero-violation rule rows are deliberate — they are the rate denominator. The branch is the
  session proxy (pool sessions run on `claude/<slug>` branches).

## Reading the loop

```bash
go run ./cmd/acdsl verdicts -since 720h
```

- Stats are keyed (rule, projected): a rule flipped mid-window shows two rows — that pair is
  the A/B comparison, no extra tooling.
- Retries-to-green is derivable from consecutive same-branch records (red run followed by a
  clean run); not computed in v1.

## Eviction policy

Any red run is proof the rule earns its projection — ten 1% failures are ten fully
failed features, so a low red rate is never an eviction argument. A rule qualifies for
`projected="false"` only when both hold:

1. Verifier + fixtures green — the gate actually covers it.
2. ≥ 300 logged runs on the projected arm with **zero reds** — a long, entirely clean
   window is the only evidence the prose is dead weight.

The bar is an initial calibration, deliberately arbitrary and subject to adjustment as the
loop accumulates data; rule-set churn makes the arms non-stationary, so treat the stats as
rough guidance.

Re-projection criterion: **any red** on the gate-only arm → flip back, one field edit.
Cadence: monthly `verdicts -since 720h` review. The end state: context shrinks toward
judgment rules and facts; everything mechanical lives as gates.

## Rule identity

- IDs follow the shared `KIND-SCOPE[-TOPIC]-NNN` grammar (see `context/actions/README.md`,
  "Rule identity") — gate-enforced here by ACDSL-SMINE-003 (`id-grammar`): the scope and
  topic segments must be registered in the `aspects` array of `context/context.json`. Task-lifetime contract
  entries are exempt (ephemeral, free-form).
- Verdict-log note: the 2026-08-09 migration to scoped IDs (e.g. ACDSL-FMT-001 →
  ACDSL-GOLANG-FMT-001) restarts per-rule stats under the new IDs; pre-migration records
  keep their old IDs and age out of relevance.

## Projection guard

- File projection is syntax-table-driven: `// ` for Go, `# ` for sh/py/yaml/toml/Makefile,
  `<!-- … -->` for md/html, `-- ` for sql (`internal/acdsl/project.go`, `projectionSyntaxes`).
  Insertion keeps a shebang on line 1 and a Markdown frontmatter fence on top; the
  staged-leak guard matches the syntax-wrapped shape against the file's own syntax, so a
  doc quoting another syntax's form in prose is not a leak.
- Files without a comment syntax — json, `.acdsl`, binaries — are never touched on disk:
  their rules are **gate-only on disk** and delivered at read time by the PostToolUse(Read)
  hook `acdsl-context.sh`, which injects `project -context <file>` output as
  additionalContext. `project -plan` still resolves and prints them.
- Rules whose projection would break their own consumers carry `projected="false"`:
  the context-doc gates (ACDSL-SMINE-001/002 — the contextdocs parser reads those files)
  and ACDSL-GOLANG-DOCS-001 (a projected read-gate guide shifts its line count and
  invalidates recorded read coverage mid-session). Enforcement is unchanged.

## Generation

- A voted context proposal whose `gate.verifier` names no registry entry is **generated by
  the smine-apply agent**: verifier main.go + registry entry + rule line + fixtures, then
  self-validated (`acdsl fixtures` + `make audit`). Two attempts, then the proposal falls
  back to the `manual-verifier` disposition.
- Boundaries live in `acdsl/evalgen.json` and are **enforced by ACDSL-GOLANG-GEN-001**
  (evalgen-bounds), uniformly over handwritten and generated scripts:
  - `language` — `"go"` (hard-coded) or `"inferred"` (go.mod present → go); anything else
    refuses generation.
  - `maxLines` — non-empty line bound per verifier main.go.
  - imports — Go stdlib, this module's `internal/...`, or a module already in go.mod's
    require set. **Generation may never add a dependency** — accepting one (Band D) is a
    user call, made by editing go.mod, which updates the allowlist by construction.
  - `execAllowlist` — external binaries a wrapper may run, always via `internal/shell.Run`.
  - `timeoutMaxS` — cap on a generated registry entry's timeout.
  - `bands` — generatable bands (F, A); Band D is refused.
  - `autoApply` — whether unattended (decide/always) apply runs may generate; default
    false, explicit votes always may.
- Fixture seeds come from the proposal's evidence snippets (≥1 violation + ≥1 clean —
  smine-context requires them on sketched-verifier proposals).

## Coverage convention

- A rule's `why` cites the registry or style rule ID it enforces, in parentheses — e.g.
  `(RULE-GOLANG-ERR-001)`, `(ACTION-IMPL-INTEG-005)`. The config-server UI extracts these citations:
  coverage % = distinct cited `context/context.json` entries over all entries; style-guide
  `RULE-*` citations are counted separately (they have no registry denominator). New rules
  follow the convention.

## UI

- The config server's **Context tab** is tabbed: **ACDSL** (the loop) and **Actions** (the
  entries + docs). The ACDSL tab leads with an always-visible control panel (counts, coverage,
  run-fixtures), then the rule cards, then a collapsed recent-gate-runs list (branch is
  the session proxy). Stats cover the full verdict history.
- **No delivery toggle in the UI.** A flip is a rare, evidence-driven act: edit the
  declaration line (`projected="false"` on/off) and commit it. The UI shows delivery
  state as a badge and flags **flip candidates**:
  - `eviction candidate` — fixtures green, projected arm ≥300 runs with **zero reds**.
    Any red keeps the projection: a rare failure is still a fully failed feature.
  - `re-projection candidate` — **any red** on the gate-only arm; any failure while
    unprojected warrants projecting the prose back.
- **Scopes are the shared taxonomy** across both systems: an acdsl rule's scope is its
  ID's second segment (`ACDSL-GOLANG-ERR-001` → `GOLANG`). The bar on both tabs filters
  and manages the vocabulary (`class: scope` aspect entries in `context/context.json`); scopes missing
  from the taxonomy show amber "unregistered", and a taxonomy member cited by registry
  entries **or** acdsl rule IDs cannot be deleted. New rules use a registered scope, or
  register one — press into existing entries first (the smine convention).
- The **Actions tab** lists the registry entries (kind, scope, topic, statement,
  enforcement) with `gated: <rule>` badges from the coverage citations, plus the context
  docs and deploy tooling.
- Verdict source: `~/.claude/acdsl/verdicts.jsonl` (override: `-acdsl-verdicts` flag).
