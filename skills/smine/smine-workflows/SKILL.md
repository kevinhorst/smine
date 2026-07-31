---
name: smine-workflows
description: Extract and rank workflow candidates from smine-batch reports into sessions/proposals/workflows.json. Trigger on /smine-workflows or "extract workflow candidates from batches". Args — batch file: one batch; absent means all batches with ledger-missing sessions; production cap: honored when the invocation states one.
author: Kevin Horst
version: 1.14
allowed-tools: Read, Write, Edit, mcp__Peek_MCP__session_plan, mcp__Peek_MCP__session_diff, ToolSearch
---

# smine: workflow proposals

Turn workflow candidates from batch reports into ranked, spec-normalized proposals ready to author as skill-bundled workflow scripts (`skills/<skill>/workflows/<name>.js`, see `docs/workflows.md`).

## When to use

**Use when:** extracting and ranking workflow candidates from batch reports — deterministic multi-agent orchestration patterns. Invoked via /smine-workflows or as part of the /smine fan-out.
**Don't use when:** the candidate is judgment/adaptive/user-in-loop — reroute to /smine-skills. Time-driven maintenance — reroute to /smine-routines. Writing the workflow script itself — author it directly under `skills/<skill>/workflows/`.
**Preconditions:** one or more completed batch reports under `sessions/`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-workflows** (see `docs/skill-map.md`, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- production cap: when the invocation states "Production cap: add at most N new proposals", write at most N new proposals this run — keep the best-ranked candidates, list every dropped candidate in the run report (never silent). Evidence appends to existing proposals do not count against the cap.

## 0. Setup

- Input: batch reports `sessions/{personal,work}/*batch-*.md` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`).
- Ledger: `sessions/<scope>/analyzed-workflow.txt` (historical filename, predates the smine-workflows rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `sessions/proposals/workflows.json` — the single authoritative artifact: cumulative, cross-scope, ranked, updated in place, conforming to `sessions/proposals/schema.json`. There is no md form.
- Old batches predate dedicated sections — mine the prose: manual multi-session fan-outs the user drove by hand (parallel worktree reviews, bake-offs, consolidation passes) are workflow evidence even when no batch calls them that.

## 1. Extract & test

- Collect workflow candidates: recurring orchestration whose value is deterministic control flow — fan-out over an enumerable item list, staged find→verify→consolidate pipelines, loop-until-dry — runnable unattended.
- Apply the decision test (smine-batch, "Skill vs Workflow"): deterministic/unattended/N agents → workflow; judgment/adaptive/user-in-loop → reroute as a note for /smine-skills; time-driven maintenance → reroute as a note for /smine-routines.
- Edits beat new workflows: prefer extending an existing workflow or its owning skill; a **new-workflow** proposal carries a one-line justification of why no existing workflow or skill absorbs it.

## 2. Normalize to spec

Each candidate gets exactly the spec fields (never a full script):

- name + one-line purpose
- trigger (when it would be invoked)
- item source (files, sessions, findings, PRs)
- stage graph: phases, pipeline vs barrier, where verification/consolidation sits
- per-stage structured output (field names, not JSON Schema)
- evidence: session IDs where the pattern ran manually + what it cost by hand

## 3. Archive suppression

- Before writing any proposal, read `sessions/proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and is eligible again from day 15 on.
- Drop every candidate matching the suppression set and list each drop (`candidate → matching archived entry`) in the run report — suppression is auditable, never silent.

## 4. Rank & write

- Merge duplicates across batches; rank by recurrence, frustration weight, manual cost, breadth.
- Entry fields: spec (above) + status (`proposed | accepted | building | done | rejected | postponed`). Re-runs add evidence, never change a user-set status or delete an entry.
- **One proposal = one change = one vote.** Every votable proposal carries exactly one `change` field — the imperative edit. A candidate demanding N distinct changes becomes N sibling proposals with ids `<slug>--1` … `<slug>--N`, assigned once and never renumbered on re-runs (votes bind to the id). New proposals carry a `proposed` field (`<YYYY-MM-DD>`, the analyze-run date) — stamped once at first write, never rewritten.

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
- Proposals only — this skill never writes a workflow script itself.
- Spec fields must stay identical to smine-batch's definition; if they drift, fix the skills, not the doc.
- Archived entries (`archive/done.md`, `archive/rejected.md`, `archive/postponed.md`) are anti-re-proposal memory — the mechanical suppression in step 3 is the enforcement.

## Model

- Suggested: mid-tier / medium
- Reason: normalize to spec + decision test
- Tested unviable: — (none yet)

## Changelog

- v1.14 (2026-07-31): session-scope rename to personal/work (input globs)
- v1.13 (2026-07-30): per-run production cap arg (smine pipeline --max-proposals-per-dimension)
- v1.12 (2026-07-30): allowed-tools permission manifest declared
- v1.11 (2026-07-27): renamed analyze-workflow → smine-workflows; part of the /smine fan-out; sibling reroutes → /smine-skills, /smine-routines
- v1.10 (2026-07-26): JSON-only — `workflows.json` is the sole authoritative artifact; the md form and the regenerate-from-md step are gone, evidence re-anchored to schema fields
- v1.9 (2026-07-26): proposed-date stamp — new entries carry `proposed: <date>` (schema.json bump)
- v1.8 (2026-07-24): one proposal = one change = one vote — proposal-level change field, evidence provenance-only, `--<n>` split ids (schema.json bump)
- v1.7 (2026-07-24): mechanical Archive-suppression step (step 3) replaces the advisory archive line; Extract & test gains the Edits-beat-New justification requirement
- v1.6 (2026-07-22): archive doctrine incl. postponed 14-day suppression; status vocab completed
- v1.5 (2026-07-19): ledger carries a Last-analyzed-batch first line, updated on every append
- v1.4 (2026-07-19): per-session note bullets replace the evidence text blob; dimension must match batch findings
- v1.3 (2026-07-19): workflow script location updated to skill-bundled convention (skills/<skill>/workflows/), .claude/workflows/ references removed
- v1.2 (2026-07-19): structured evidence contract (title + 1–3 ranked session ids + optional change/snippets)
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-11): initial version
