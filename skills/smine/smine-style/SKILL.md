---
name: smine-style
description: Extract code-style rules and doc drift from smine-batch reports into sessions/proposals/style.json. Trigger on /smine-style or "extract style rules or doc fixes from batches". Args — batch file: one batch; absent means all batches with ledger-missing sessions; production cap: honored when the invocation states one.
author: Kevin Horst
version: 1.16
allowed-tools: Read, Write, Edit, mcp__Peek_MCP__session_plan, mcp__Peek_MCP__session_diff, ToolSearch
---

# smine: style proposals

Turn repo-surface rules and doc-drift findings from batch reports into per-surface edit lists. The ad-hoc precedent is TODO.md (memory-batch evaluation: 5 of 24 clusters were "right content, wrong surface").

## When to use

**Use when:** extracting code-style rules and doc drift from batch reports — style rules, review checklist items, process rules, and contradictions against checked-in docs. Invoked via /smine-style or as part of the /smine fan-out.
**Don't use when:** the finding is a process rule belonging inside a specific skill — reroute to /smine-skills. Editing context docs or AGENTS.md directly — this skill only proposes.
**Preconditions:** one or more completed batch reports under `sessions/`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-style** (see `docs/skill-map.md`, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- production cap: when the invocation states "Production cap: add at most N new proposals", write at most N new proposals this run — keep the best-ranked candidates, list every dropped candidate in the run report (never silent). Evidence appends to existing proposals do not count against the cap.

## 0. Setup

- Input: batch reports `sessions/{personal,work}/*batch-*.md` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`).
- Ledger: `sessions/<scope>/analyzed-rules.txt` (historical filename, predates the smine-style rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `sessions/proposals/style.json` — the single authoritative artifact: cumulative, cross-scope, grouped by surface, updated in place, conforming to `sessions/proposals/schema.json`. There is no md form.
- Old batches predate dedicated sections — mine the prose: corrections phrased as conventions ("never X", "always Y", test-struct rules) are rule material wherever they appear.

## 1. Extract

- **Rules**: corrections that belong in enforcement surfaces, not memory — style/design conventions, review checklist items, process rules.
- **Doc drift**: session evidence contradicting checked-in docs — quote the contradiction and name the doc.

## 2. Group by target surface

- `context/rules/*` — FACT/NEVER/ALWAYS activity chapters (implementing, navigating) + reviewing.md (Definition of Done).
- `context/style/*` — artifact style guides and RULE-* candidates (plan.md, commits.md, per-language files incl. go.md's TESTS/GOROUTINES sections).
- `context/AGENTS.md` — template-level conventions.
- Skills — process rules belonging in a specific skill go to /smine-skills's report card instead; note the reroute.
- External repos (their AGENTS.md/CLAUDE.md/docs) — flagged with the exact target path, never edited from here.

## 3. Archive suppression

- Before writing any proposal, read `sessions/proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and is eligible again from day 15 on.
- Drop every candidate matching the suppression set and list each drop (`candidate → matching archived entry`) in the run report — suppression is auditable, never silent.

## 4. Diff & write

- Read the current target surface before proposing: drop rules already covered; near-misses become amendments referencing the existing rule ID. Amendments beat new rules: a **new-rule** proposal carries a one-line justification of why no existing rule or context doc absorbs it.
- `style.json` groups proposals by target surface (`groups[]`), one proposal object per item, evidence (session IDs + quotes) attached. Each proposal's `status` (`proposed | applied | rejected | postponed`) is the user's; re-runs add evidence, never change a user-set status or delete a proposal.
- **One proposal = one change = one vote.** Every votable proposal carries exactly one `change` field — the imperative edit. A candidate demanding N distinct edits becomes N sibling proposals with ids `<slug>--1` … `<slug>--N`, assigned once and never renumbered on re-runs (votes bind to the id). New proposals carry a `proposed` field (`<YYYY-MM-DD>`, the analyze-run date) — stamped once at first write, never rewritten.

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
- Proposals only — this skill never edits a context doc, AGENTS.md, or external repo itself.
- Single-session evidence is marked as such — one occurrence proposes tentatively, recurrence hardens.
- Archived items (`archive/done.md`, `archive/rejected.md`, `archive/postponed.md`) are anti-re-proposal memory — the mechanical suppression in step 3 is the enforcement.

## Model

- Suggested: mid-tier / medium
- Reason: diff rule candidates against current docs
- Tested unviable: — (none yet)

## Changelog

- v1.16 (2026-07-31): session-scope rename to personal/work (input globs)
- v1.15 (2026-07-30): renamed smine-rules → smine-style (kind rename rules → style; output style.json); per-run production cap arg
- v1.14 (2026-07-31): navigating.md rename; stale context/go surface folded into context/style
- v1.13 (2026-07-31): activity-scoped context — target surfaces are rules/ activity chapters and style/ guides
- v1.12 (2026-07-30): context redesign — target surfaces: context/rules entries + relocated skill assets
- v1.11 (2026-07-30): allowed-tools permission manifest declared
- v1.10 (2026-07-27): renamed analyze-rules → smine-rules; part of the /smine fan-out; sibling reroute → /smine-skills
- v1.9 (2026-07-26): JSON-only — `rules.json` is the sole authoritative artifact; the md form and the regenerate-from-md step are gone, evidence/format sections re-anchored to schema fields
- v1.8 (2026-07-26): proposed-date stamp — new items carry `proposed: <date>` (schema.json bump)
- v1.7 (2026-07-24): one proposal = one change = one vote — proposal-level change field, evidence provenance-only, `--<n>` split ids (schema.json bump)
- v1.6 (2026-07-24): mechanical Archive-suppression step (step 3) replaces the advisory archive line; Diff & write gains the Amendments-beat-New justification requirement
- v1.5 (2026-07-22): archive doctrine incl. postponed 14-day suppression; status vocab completed
- v1.4 (2026-07-19): ledger carries a Last-analyzed-batch first line, updated on every append
- v1.3 (2026-07-19): per-session note bullets replace the evidence text blob; dimension must match batch findings
- v1.2 (2026-07-19): structured evidence contract (title + 1–3 ranked session ids + optional change/snippets)
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-11): initial version
