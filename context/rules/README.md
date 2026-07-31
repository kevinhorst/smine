# Rules — authoring spec

Doctrine and fact entries for the context layer. Every statement in this directory is one typed,
ID'd entry; the registry tool (`cmd/rules`) parses these files into `context/rules.json` and
validates them in `make audit` (in this repo, together with `../facts/` — the source tree is
smine' own pack).

Files are **activity chapters** — named for the activity the entries govern: `concepting.md`
(hot-class gates, binding at plan time), `implementing.md` (scoping, stop conditions, data
integrity), `reviewing.md` (the Definition of Done — checklist content, not yet entries; the
parser ignores non-entry lines), `navigating.md`. An entry's aspect stays its topic tag; the file
says when it binds. Moving an entry between chapters never changes its ID.

Consumers: agents cite entry IDs when orienting, planning, and flagging violations. Multi-paragraph
specifications with examples (language style, commit style, plan format) are NOT entries — they are
the `RULE-*` guides in `../style/`.

## Entry grammar

One entry per statement:

```markdown
**NEVER-INTEG-002** `[review]` — No auto-complete or auto-rollback timers on stateful flows.

* Why: state transitions fire on explicit events; clock-driven transitions mask lost events.
* Applies: diff touches a state machine, scheduled job, or timer wired to persisted state.
```

```markdown
**FACT-STACK-001** — Backend is Go services; no Node runtime.

* Location: go.mod, requirements.txt
```

- ID: `<CLASS>-<ASPECT>-<NNN>` — `CLASS ∈ {FACT, NEVER, ALWAYS}`, `ASPECT` from the vocabulary
  below, `NNN` zero-padded to three digits.
- Enforcement tag (backticked, after the ID): required on NEVER/ALWAYS, absent on FACT.
  `[hook]` | `[lint]` | `[gate]` | `[review]` | `[manual]` — the cheapest sufficient mechanism.
- Statement: one sentence after the em dash. An entry needing paragraphs is a spec, not an entry.
- Bullets under the entry:
  - `Why:` — rationale (recommended on NEVER/ALWAYS).
  - `Applies:` — applicability trigger (required on NEVER/ALWAYS); doubles as the manifest and
    adherence-scoring trigger.
  - `Evidence:` — what proves adherence (optional).
  - `Location:` — evidence pointer (required on FACT; keeps staleness mechanically checkable).

## Aspect vocabulary

Closed set, enforced by the validator. The source of truth is `aspects.json` next to this file —
extend it via the config server's Context page (or edit the JSON), never as an ad-hoc string in a
rules file. In deployed packs `rules/aspects.json` is baseline-owned and overwritten on every sync.

## Numbering

- NEVER/ALWAYS baseline entries (files in this directory, synced to every pack): `NNN` 001–099
  per (class, aspect).
- NEVER/ALWAYS repo overlay entries (files a target repo adds to its own `rules/` dir): `NNN`
  100+ per (class, aspect). The validator enforces the split, so overlays can never collide with
  a baseline update.
- FACT entries are repo-owned (they live in a pack's `facts/` dir or overlay files, never in this
  directory) and number from 001 — the range split does not apply to them.

