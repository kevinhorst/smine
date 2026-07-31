---
name: smine-summary
description: Convert a smine-batch report to schema-conformant JSON under sessions/<scope>/json/. Trigger on /smine-summary [batch file] or "turn batch reports into JSON". Args — batch file: one batch report; absent means all batches with ledger-missing sessions.
author: Kevin Horst
version: 1.11
allowed-tools: Bash(jq *), Read, Write
---

# smine-summary

Convert batch report markdown into JSON conforming to `reference/schema.json`. Consumer: the (later) session-overview server — the schema is the contract; change it only deliberately.

## When to use

**Use when:** converting a finished batch report to schema-conformant JSON for machine consumption. Invoked via /smine-summary [batch file] or as part of the /smine fan-out.
**Don't use when:** mining sessions for the batch report itself — /smine-batch. Processing batch content into dimension proposals — the other smine-* dimension skills. Quick-peeking a session — /peek.
**Preconditions:** a completed batch report markdown file under `sessions/` and the reference schema at `reference/schema.json`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-summary** (see `docs/skill-map.md`, smine repo).

## Args

- batch file: positional — one batch report; absent → every batch containing session IDs missing from the ledger.

## 0. Setup

- Input: batch reports `sessions/{personal,work}/*batch-*.md` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`).
- Ledger: `sessions/<scope>/analyzed-summarize.txt` (historical filename, predates the smine-summary rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `sessions/<scope>/json/<batch-stem>.json`, 1:1 with the md file (e.g. `sessions/work/json/sessions-batch-19.json`).

## 1. Parse

- Batch metadata from the header prose: scope, batch number, analyzed date, covered date range, theme, source note.
- `analyzerVersion` ← the header's `Analyzer: <name> v<version>` line (version string only, e.g. `1.11`; historical batches carry names `session-analyze`, `smine`, or `smine-batch`); absent on old batches → field omitted (nothing invented).
- `title` — a 5–9 word human title naming the batch's centerpiece; derive from the report's theme/header, never a fragment copy.
- Every session in the "Sessions covered/analyzed" table gets an entry — including skipped ones (`skipped: true` + reason) and DATA GAP flags.
- Session fields from the table + title tags: `[Repo, Type]` conventions map to `repo`/`type`; signal/verdict column maps to `signal`.
- Findings from the prose: map each to a canonical dimension (`skill-candidate`, `workflow-candidate`, `routine-candidate`, `skill-report-card`, `feature-design`, `memory`, `rule`, `doc-drift`, `harness-friction`, `exemplar`, `other`) with summary + verbatim quotes. Old batches have no dimension headings — the mapping is this skill's judgment; batch-level findings not attributable to one session attach to every session they cite.
- Frustration-index entries → `frustration[]` (quote + trigger); cross-session arcs → top-level `arcs[]`.
- Positive-index entries → `positive[]` (quote + trigger) — same shape as frustration; an absent section → field omitted (nothing invented).
- `invoked_skills` ← the per-session "Skills invoked:" line, split on commas; `none` or an absent line → field omitted (nothing invented).

## 2. Write & validate

- Write the JSON, then validate with jq: parses; `.batch.scope` and `.batch.file` present; every `.sessions[].id` matches the UUID pattern; every finding has `dimension` + `summary`. Fix and re-validate until clean.
- Nothing invented: a field absent from the batch is omitted, never guessed. Only `sessions[].id` is required per session.

## 3. Finish

- Append the processed session IDs to the ledger and set its `Last analyzed batch:` first line (insert if absent), then STOP for user review.

## Rules

- Full session IDs, never truncated; quotes verbatim.
- Carry fenced code blocks from the report into `findings[].snippets` verbatim — `{kind: violation|fix|context, lang, code, source}` from the report's annotations; never invent or trim code.
- Lossy is fine, wrong is not — omit what doesn't map cleanly and count it in the run report.
- Schema changes are a deliberate act with the consumer in mind, never a side effect of one awkward batch.

## Model

- Suggested: small / low
- Reason: mechanical report→JSON schema conversion
- Tested unviable: — (none yet)

## Changelog

- v1.11 (2026-07-31): session-scope rename to personal/work (globs, schema enum, output-path example)
- v1.10 (2026-07-30): allowed-tools permission manifest declared
- v1.9 (2026-07-27): renamed ssummarize → smine-summary; Analyzer line parsed generically (session-analyze/smine/smine-batch); part of the /smine fan-out
- v1.8 (2026-07-26): Args section
- v1.8 (2026-07-24): renamed session-summarize → ssummarize (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.7 (2026-07-24): delegation declaration removed — small skills are never delegatable
- v1.6 (2026-07-22): classification unattended-safe (Delegation + Command surface), effort low
- v1.5 (2026-07-19): batch.analyzerVersion in the schema, extracted from the report's Analyzer header line
- v1.4 (2026-07-19): ledger carries a Last-analyzed-batch first line, updated on every append
- v1.3 (2026-07-19): findings[].snippets — verbatim violation/fix/context code blocks carried from the report
- v1.2 (2026-07-15): positive[] session field (positive-index mapping)
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-11): initial version
