---
name: smine-skills
description: Extract and rank skill proposals from smine-batch reports into sessions/proposals/skills.json. Trigger on /smine-skills or "extract or rank skill proposals from batches". Args — batch file: one batch; absent means all batches with ledger-missing sessions; production cap: honored when the invocation states one.
author: Kevin Horst
version: 1.14
allowed-tools: Read, Write, Edit, mcp__Peek_MCP__session_plan, mcp__Peek_MCP__session_diff, ToolSearch
---

# smine: skill proposals

Turn skill candidates and existing-skill report cards from batch reports into one ranked proposals doc.

## When to use

**Use when:** extracting and ranking skill proposals from batch reports — new skill candidates and existing-skill report cards. Invoked via /smine-skills or as part of the /smine fan-out.
**Don't use when:** creating or editing an actual skill — /skillroutine-create. Extracting memory, workflows, routines, or rules — the other dimension skills. Writing a batch report — /smine-batch.
**Preconditions:** one or more completed batch reports under `sessions/`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-skills** (see `docs/skill-map.md`, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- production cap: when the invocation states "Production cap: add at most N new proposals", write at most N new proposals this run — keep the best-ranked candidates, list every dropped candidate in the run report (never silent). Evidence appends to existing proposals do not count against the cap.

## 0. Setup

- Input: batch reports `sessions/{personal,work}/*batch-*.md` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`).
- Ledger: `sessions/<scope>/analyzed-skill.txt` (historical filename, predates the smine-skills rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `sessions/proposals/skills.json` — the single authoritative artifact: cumulative, cross-scope, ranked, updated in place, conforming to `sessions/proposals/schema.json`. There is no md form.
- Old batches predate dedicated sections — mine the prose for candidates, not only headings.

## 1. Extract & merge

- Collect two kinds per batch: **skill candidates** (new procedures, incl. validated dispatch/prompt templates) and **existing-skill report-card entries** (defects/gaps in a named skill's invocation).
- Merge duplicates across batches into one entry each; keep every batch's evidence (session IDs, quotes). Candidates recur — batch-01's `fimplement` is the precedent.

## 2. Inventory check

- Read the live `skills/` inventory (this repo). A candidate an existing skill already covers becomes an **edit** to that skill or is marked covered — never a duplicate proposal.
- Edits beat new skills: a **new-skill** proposal requires an explicit one-line justification of why no existing skill can absorb it as an edit; absent that justification, default to an edit proposal against the closest existing skill.

## 3. Archive suppression

- Before writing any proposal, read `sessions/proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and is eligible again from day 15 on.
- Drop every candidate matching the suppression set and list each drop (`candidate → matching archived entry`) in the run report — suppression is auditable, never silent.

## 4. Rank & write

- Rank by impact, evidence-based ordering (no numeric formula): recurrence (# distinct sessions), frustration weight (attached quotes), cost of absence (burned cycles/sessions), breadth (repos/scopes affected).
- `skills.json` has two groups (`groups[]`): **New skills** and **Edits to existing skills**. Per proposal: title, one-line purpose, rank evidence, status.
- **One proposal = one change = one vote.** Every votable proposal carries exactly one `change` field — the imperative edit, quotable into the skill file. A candidate demanding N distinct changes becomes N sibling proposals with ids `<slug>--1` … `<slug>--N`, assigned once and never renumbered on re-runs (votes bind to the id). Edit proposals name their `target`; a proposal without a concrete change is not actionable and must not be written. New proposals carry a `proposed` field (`<YYYY-MM-DD>`, the analyze-run date) — stamped once at first write, never rewritten.
- Status is the user's column (`proposed | accepted | building | done | rejected | postponed`): new entries get `proposed`; re-runs may add evidence to any entry but never change a user-set status or delete an entry.

## 5. Finish

- Append the processed session IDs to the ledger and set its `Last analyzed batch:` first line (insert if absent), then STOP for user review.

## Rules

- Evidence format — one `evidence[]` object per evidence point (schema fields):
  - `title` — the generalized pattern/rule name, one line, no prose blob.
  - `sessions[]` (1–3, ranked strongest occurrence first): each `{id, note}` — the full session id and one skimmable clause naming what that session evidences; the linked session carries the detail. At least one session is mandatory: a point with no attributable session id is not admissible evidence — recover the id from the batch report or fold the point into the proposal.
  - evidence is provenance only — the proposal's single `change` field lives at proposal level, never on an evidence object.
  - optional `dimension` — the deep link filters the batch page by dimension, so the effective dimension (kind default or this override) MUST match a finding dimension the cited sessions actually have in their batch JSON; verify and override, or the link lands on unrelated findings.
  - optional `snippets[]` annotated `kind` `(violation)` / `(fix)` / `(context)`, verbatim from the batch report or peek-mcp `session_plan`/`session_diff` — best effort, never reconstructed.
- Full session IDs, never truncated; evidence quotes verbatim.
- Proposals only — this skill never creates or edits a skill itself.
- Archived entries (`archive/done.md`, `archive/rejected.md`, `archive/postponed.md`) are anti-re-proposal memory — the mechanical suppression in step 3 is the enforcement.

## Model

- Suggested: mid-tier / medium
- Reason: merge/rank proposals against skill inventory
- Tested unviable: — (none yet)

## Changelog

- v1.14 (2026-07-31): session-scope rename to personal/work (input globs)
- v1.13 (2026-07-30): per-run production cap arg (smine pipeline --max-proposals-per-dimension)
- v1.12 (2026-07-30): allowed-tools permission manifest declared
- v1.11 (2026-07-27): renamed analyze-skill → smine-skills; part of the /smine fan-out; skill authoring handoff → /skillroutine-create
- v1.10 (2026-07-26): JSON-only — `skills.json` is the sole authoritative artifact; the md form and the regenerate-from-md step are gone, sections/evidence re-anchored to schema fields
- v1.9 (2026-07-26): proposed-date stamp — new entries carry `proposed: <date>` (schema.json bump)
- v1.8 (2026-07-24): one proposal = one change = one vote — proposal-level change field, evidence provenance-only, `--<n>` split ids (schema.json bump)
- v1.7 (2026-07-24): mechanical Archive-suppression step (step 3) replaces the advisory archive line — build the suppression set from done/rejected/postponed and drop matching candidates, listing drops; Inventory step gains the Edits-beat-New justification requirement
- v1.6 (2026-07-22): archive doctrine incl. postponed 14-day suppression; status vocab completed
- v1.5 (2026-07-19): ledger carries a Last-analyzed-batch first line, updated on every append
- v1.4 (2026-07-19): per-session note bullets replace the evidence text blob; dimension must match batch findings
- v1.3 (2026-07-19): structured evidence contract (title + 1–3 ranked session ids + snippets); target + change mandatory for edit proposals
- v1.2 (2026-07-16): skill authoring delegates to /couchskill-create
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-11): initial version
