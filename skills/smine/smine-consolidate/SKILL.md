---
name: smine-consolidate
description: Consolidate the pending proposal store — dedup, move misplaced proposals to their right dimension, tighten presentation, validate against schema and audit. Trigger on /smine-consolidate [proposals], normally as the smine-nightly consolidate stage. Args — mode: proposals (default; skills|context reserved); caveman: compress wording, requires caveman skill; language <lang>: full-store sweep — all mutable user-visible prose in <lang>.
author: Kevin Horst
version: 1.4
argument-hint: "[proposals] [caveman] [language <lang>]"
allowed-tools: Read, Write, Edit, Bash(jq *), Bash(go run ./cmd/acdsl *), Bash(make audit *), ToolSearch
---

# smine: consolidate proposals

Batch cleanup pass over `proposals/*.json` after the dimension fan-out: merge duplicates, re-home misplaced proposals, tighten presentation, and leave the store schema- and audit-green.

## When to use

**Use when:** the proposal store needs a cleanup pass — after a /smine run, or as the smine-nightly consolidate stage. Invoked via /smine-consolidate.
**Don't use when:** producing proposals — the dimension skills. Consuming votes — /smine-apply. Editing skills or context docs — mode reserved (see Args).
**Preconditions:** `proposals/*.json` present; no dimension skill currently writing.
**Workflow position:** smine pipeline: fan-out → **smine-consolidate** → user votes → smine-apply.

## Args

- mode: positional, default `proposals`. `skills` and `context` (consolidating the actual repo surfaces) are reserved — STOP with "mode reserved, not implemented".
- `caveman`: compress reworded prose in the caveman style; requires `~/.claude/skills/caveman`, else STOP.
- `language <lang>`: all user-visible prose of the mutable set ends up in `<lang>` — reworded entries are written in it, and untouched entries whose prose is not in `<lang>` are translated in place (the title's change-name part, `change`, `fields[].label/text`, `evidence[].title`, `sessions[].note`, free-form group titles). Never translated: ids, targets, file paths, code, dates, tags, status values, schema keys, and the fixed structural group titles (machine-read keys — e.g. the three skills.json groups and "Considered, not proposed"). Default: keep the store's language.

## Hard invariants

- **Mutable set:** only entries with `status: "proposed"` and no vote in `proposals/votes.jsonl` (key `<kind>/<id>`). Everything else — any user-set status, any voted entry — is immutable, including its formatting.
- Never renumber or reuse ids; a merge keeps the strongest entry's id and archives nothing (dropped duplicates are deleted from JSON and listed in the run report — they were never voted).
- Grouping stays the authored two-level shape (`groups[].title`); this skill never invents new group titles beyond the target kind's existing conventions. With `language <lang>`, free-form group titles are translated in place; fixed structural titles stay verbatim.

## 1. Validate first

- Run `go run ./cmd/acdsl check`. Fix every proposals/*.json conformance violation mechanically (structure only, content preserved — e.g. mis-nested fields moved to their schema location) before any other pass.

## 2. Re-home misplaced proposals

- Routing tests: skill-procedure change → skills.json; knowledge/rules/facts (incl. entries a skill needs via its `acdsl-context:` declaration) → context.json; multi-step automation → skills.json (Workflows group); scheduled job → routines.json.
- Move = rewrite in the target kind's group and schema shape (fresh `<slug>--<n>` id under the target's conventions, `proposed` date carried over), delete from the source, report `moved <kind>/<id> → <kind>/<id>`. Mutable set only.
- A rule duplicated across ≥2 target skills becomes one context.json proposal whose change adds the entry and each skill's `context:` declaration; the sibling skill edits are the dropped duplicates.

## 3. Dedup

- Across kinds and targets: near-identical change texts merge into the strongest-evidenced entry; union the evidence (sessions dedup by id). Mutable set only — a voted sibling blocks the merge (report `kept: voted sibling <id>`).
- Consult `proposals/archive/{done,rejected,postponed}.md`: a pending entry matching `done`/`rejected` is dropped (rejected = permanent anti-re-proposal memory); `postponed` follows the 14-day rule.

## 4. Presentation

- Enforce the title contract (`<target> — <distinct change name>`, no shared candidate headings), one-line evidence notes, and verbosity reduction on `change`/`fields` prose. Apply `caveman` when given. With `language <lang>`, run the full-store language sweep from the Args section: every mutable entry whose user-visible prose is not in `<lang>` is translated — not only entries this pass rewords. Mutable set only.
- **Casual jargon sweep.** When the presentation profile's `audience:` is `casual`: mutable entries whose user-visible prose (title's change-name, `change`, `fields[].label/text`, `evidence[].title`, `sessions[].note`) still carries file paths, rule/FACT/ACDSL IDs, or schema/taxonomy jargon are reworded — the technical content moves into `target`/`anchor`/`code`/snippet fields, never deleted.

## 5. Finish

- Run `make audit`; it must pass — a failure this skill cannot fix by conformance edits is reported and left red, never masked.
- Run report: per pass — fixed / moved / merged / dropped / reworded, with ids; then STOP.

## Model

- Suggested: mid-tier / medium
- Reason: mechanical store hygiene against fixed invariants
- Tested unviable: — (none yet)
