<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# Canon rules — authoring spec

Action and fact entries of the context layer — the prose beside the ACDSL gates.
Every statement in this directory is one typed, ID'd entry; the registry tool (`cmd/rules`)
parses these files into `context/context.json` and validates them in `make audit` (in this repo,
together with `../facts/` — the source tree is smine's own context).

Files are **activity chapters** — named for the activity the entries govern: `concepting.md`
(hot-class gates, binding at plan time), `implementing.md` (scoping, stop conditions, data
integrity), `reviewing.md` (the Definition of Done — one entry per checklist item, each
marked PASS/FAIL/N-A at review time), `navigating.md`. The file says when an
entry binds; the entry's scope segment says the same thing in its ID. Moving an entry between
chapters never changes its ID.

Consumers: agents cite entry IDs when orienting, planning, and flagging violations.
Multi-paragraph specifications with examples (language style, commit style, plan format) are
NOT full entries — they are the `RULE-*` guides in `../rules/`, whose headlines share this
entry grammar while their bodies stay free-form specification prose.

## Terms

- **entry** — one `ACTION-*` / `RULE-*` / `FACT-*` statement in the markdown; the source of truth.
- **context.json** — the generated machine-readable form of all entries plus the aspect
  taxonomy (`context/context.json`; deployed targets carry their own merged copy).
- **registry** — only `acdsl/registry.json`, the verifier registry.
- **deploy section** — the per-target sync settings (`role`, `contextDir`, `langs`,
  `symlink`) stored under the `deploy` key of a deployed target's `context.json`.
- **target** — a repository the context is synced into via `cmd/sync/sync_context.sh`.

## Rule identity — the KIND-SCOPE[-TOPIC]-NNN grammar

Every rule-ish artifact across the context system carries one ID grammar:
`KIND-SCOPE[-TOPIC]-NNN` — three or four name segments plus a 3-digit number, never more.
Enforced by `cmd/rules validate` for these entries and by the `id-grammar` verifier
(ACDSL-SMINE-003) for acdsl declarations.

- **KIND** — what it is / who enforces it:
  - `ACDSL` — gate-backed, a verifier proves it.
  - `RULE` — prose artifact-shape rule (the style guides); in principle checkable. When its
    gate lands it *migrates* to ACDSL — kinds never coexist for one rule.
  - `FACT` — information about the world.
  - `ACTION` — process/behavior directive (stop conditions, navigation habits, hot-class
    gates, definition-of-done items); only observable in how the agent works, never in an
    artifact at rest. (Named ACTION, not ACT, for token distance from FACT.) Polarity —
    the old ALWAYS/NEVER — lives in the statement, never in the kind.
- **SCOPE** — the applies-to segment, loud on purpose: languages and artifact classes for
  ACDSL/RULE (`GOLANG`, `PYTHON`, `SQL`, `SHELL`, `JSON`, `SKILL`, `PLAN`, `COMMIT`,
  `CANON`); the *activity* for ACTION (`CONCEPT`, `IMPL`, `REVIEW`, `NAV`); the domain for
  FACT (`REPO`). Scope is the human shadow of an acdsl anchor.
- **TOPIC** — optional; only when scope alone is ambiguous. `RULE-COMMIT-001` needs none;
  `RULE-GOLANG-NAME-001` does.
- **NNN** — 3-digit, sequenced per (kind, scope, topic): uniqueness, citation stability
  while statements are edited, tombstone identity. A retired number is never reused.

The vocabulary lives in the `aspects` array of `context.json`: each member's `class` is
`scope` or `topic`. New scopes and topics are registered there first — press rules into
existing members before adding new ones (the smine convention). Extend it via the config
server's Context page, never as an ad-hoc string in a rules file. In deployed context
trees the taxonomy is baseline-owned and regenerated on every sync.

## Entry grammar

One entry per statement:

```markdown
**ACTION-IMPL-INTEG-002** `[review]` — No auto-complete or auto-rollback timers on stateful flows.

* Why: state transitions fire on explicit events; clock-driven transitions mask lost events.
* Applies: diff touches a state machine, scheduled job, or timer wired to persisted state.
```

```markdown
**FACT-REPO-STACK-001** — Backend is Go services; no Node runtime.

* Location: go.mod, requirements.txt
```

- ID: per the identity grammar above; entry kinds here are `ACTION` and `FACT`.
- Enforcement tag (backticked, after the ID): required on ACTION, absent on FACT.
  `[hook]` | `[lint]` | `[gate]` | `[review]` | `[manual]` — the cheapest sufficient mechanism.
- Marker tag (backticked, optional, after the enforcement tag; never on FACT): flags the entry
  as a member of a named cross-file set. Closed set: `[DoD]` — the entry is a Definition-of-Done
  criterion (walked PASS/FAIL/N/A by railroad-review and dod-report). Query the set with
  `rules entries --marker DoD` or jq over `context.json` (`.entries[].markers`).
- Statement: one sentence after the em dash, polarity included ("Never …", "Always …"). An
  entry needing paragraphs is a spec, not an entry.
- Bullets under the entry:
  - `Why:` — rationale (recommended on ACTION).
  - `Applies:` — applicability trigger (required on ACTION); doubles as the manifest and
    adherence-scoring trigger.
  - `Evidence:` — what proves adherence (optional).
  - `Location:` — evidence pointer (required on FACT; keeps staleness mechanically checkable).
  - `Reach:` — deployment reach (optional): `global` (default — ships to every target)
    or comma-separated repo names, e.g. `Reach: work, peek-mcp` (ships exactly to those
    targets; names = target dir basename). This repo is named `smine` — `Reach: smine`
    means "here only, never deploys"; there is no self-keyword. Sync-time filtering drops
    non-covered entries from deployed copies (`cmd/rules filter`).
- Generated, gitignored surfaces get a `FACT-REPO-GEN-*` overlay entry in the consuming repo's
  `facts/`: the artifact path, the source it is generated from, and the regenerate command —
  the entry is what grounds agents on files git cannot show (ACTION-NAV-006). Example:

```markdown
**FACT-REPO-GEN-001** — Models under `gen/` are generated from `api/openapi.yaml` (gitignored, absent until built); regenerate with `make models`.

* Location: gen/, Makefile
* Reach: <roster-name>
```

## Numbering

- ACTION baseline entries (files in this directory, synced to every deployed context): `NNN` 001–099
  per (kind, scope, topic).
- ACTION repo overlay entries (files a target repo adds to its own `rules/` dir): `NNN`
  100+ per (kind, scope, topic). The validator enforces the split, so overlays can never
  collide with a baseline update. (reviewing.md's checklist ids use 100+ section numbering —
  unparsed, so the range rule does not bind them.)
- FACT entries never live in this directory and number from 001 — the range split does not apply to
  them. They are either repo-owned (files a deployed context adds to its own `facts/` dir) or
  authored centrally in `../facts/` with an explicit `Reach:`; central facts files deploy through
  the same sync filter as actions and guides, and a file ships to a target only when at least one
  of its entries reaches it (`Reach: smine` files never deploy).
- Retiring or renaming an entry adds a row to the chapter's `## Tombstones` table
  (`| Retired | Replacement | Date |`); tombstoned numbers are never reused.
