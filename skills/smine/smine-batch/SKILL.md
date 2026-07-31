---
name: smine-batch
description: Mine past session transcripts into a batch report — the raw transcript-mining stage of the /smine pipeline. Trigger on /smine-batch or "mine session transcripts into a batch report". Args — --nightly: unattended mode, process all pending batches without review stops.
author: Kevin Horst
version: 1.14
allowed-tools: mcp__Peek_MCP__session_list, mcp__Peek_MCP__session_full, Task, ToolSearch, Read, Write
---

# smine-batch

Sweep stored session transcripts and distill actionable items into a batch report — stage 1 of the /smine pipeline. The consumer of the output is usually another agent (the /smine fan-out) — artifacts must be self-contained and injectable as-is.

## When to use

**Use when:** mining past sessions for actionable items — skill candidates, workflow patterns, memory updates, rule drift, harness friction — into a batch report. Trigger on /smine-batch, "mine session transcripts", "review transcripts", or "extract lessons".
**Don't use when:** running the whole retrospective (mine + route) — /smine. Quick-peeking the latest session — /peek. Routing an existing batch report into dimension proposals — /smine (or a single dimension skill directly). Evaluating a specific skill's runs — /skillroutine-eval.
**Preconditions:** peek-mcp available (session_list, session_full tools).
**Workflow position:** **smine-batch** → batch report → /smine fan-out → dimension skills (see `docs/skill-map.md`, smine repo).

## Args

- `--nightly`: unattended mode (headless `claude -p` routine) — process **all** pending batches sequentially; after each batch write the report and append the ledger as usual, but do not stop for review; end when no unanalyzed sessions remain.

## 0. Setup

- Data source: peek-mcp — `mcp__Peek_MCP__session_list`, `mcp__Peek_MCP__session_full` (paginate via `request_id` until `has_more=false`). Subagents must load these tools via ToolSearch first.
- Output dir: `sessions/` under the cwd.
- Ledger: `sessions/analyzed-sessions.txt`, one full session ID per line. Skip listed sessions on re-runs; append after each batch.

## 1. Select

- `session_list`, Claude sessions only. Exclude the current session.
- Order newest → oldest unless a range is given.
- Batch size: 10 (or user-given).

## 2. Analyze (parallel subagents, 2–3 sessions each)

Each subagent reads the FULL transcript. For oversized sessions, spawn a dedicated subagent. Extract per session:

- **Skill candidates**: recurring procedure whose value is judgment and convention — phases, review lenses, question protocols — one agent conversing with the user. Validated dispatch/prompt templates count (frozen-plan implement prompt, task-chip format with repro + test list, consolidation merge instruction). Check the skill inventory first: an instance of an existing skill is report-card input, not a candidate.
- **Workflow candidates**: recurring orchestration whose value is deterministic control flow — fan-out over an enumerable item list, staged find→verify→consolidate pipelines, loop-until-dry — runnable unattended. Multi-session fan-outs the user drove by hand (parallel worktree reviews, bake-offs, consolidation passes) count as evidence.
- **Existing-skill report card**: when a session invokes a known skill, grade the invocation — stale paths, phase gaps, protocol misses → skill-improvement candidates.
- **Feature-design signal**: what the agent built wrong vs. what the user corrected it to. Reconstruct the before/after concretely enough to generalize.
- **Memory candidates**: things the agent should have known. Generalize; skip repo trivia that CLAUDE.md already covers.
- **Repo-surface rules + doc drift**: corrections that belong in enforcement surfaces (context/style `RULE-*` guides incl. plan.md, context/rules `FACT/NEVER/ALWAYS` activity chapters incl. reviewing.md, AGENTS.md) — name the target surface. Session evidence contradicting checked-in docs — quote the contradiction.
- **Harness/config friction**: recurring permission prompts → allowlist candidates; hook opportunities; worktree/infra defects; MCP capability gaps. Output is settings/hook/tooling edits, not memory.
- **Nightly routine candidates**: recurring maintenance the user triggers manually on a cadence with no decisions in the loop (analysis batches, worktree cleanup, settings hygiene, ledger upkeep) → scheduled local routine. Time-driven, not on-demand — a routine typically wraps a Workflow or skill in a schedule.
- **Exemplar validation**: friction-free sessions that confirm a pattern or skill works — flag as validation, so working patterns don't read as absence of signal.
- **Frustration index**: cursing and corrections are a 100% interest flag. Quote verbatim, name the trigger.
- **Positive index**: explicit praise or expressed satisfaction — quote verbatim, name the trigger. The green counterpart of the frustration index; session-level user sentiment, distinct from exemplar validation (pattern-level).

## Skill vs Workflow

- Deterministic steps over an enumerable item list, unattended, N agents → Workflow (skill-bundled `skills/<skill>/workflows/<name>.js` script, see `docs/workflows.md`).
- Judgment/conventions, adaptive flow, user decisions mid-flow → Skill (`SKILL.md`).
- Both is legal: a Skill fronts the conversation, a Workflow does the mechanical fan-out — this skill is the Skill, its step 2 is Workflow-shaped.

Workflow candidates are reported as a spec, never a full script:

- name + one-line purpose
- trigger (when it would be invoked)
- item source (files, sessions, findings, PRs)
- stage graph: phases, pipeline vs barrier, where verification/consolidation sits
- per-stage structured output (field names, not JSON Schema)
- evidence: session IDs where the pattern ran manually + what it cost by hand

## 3. Report per batch

- One `sessions-batch-NN.md` per batch.
- The header states the producing skill version — a line `Analyzer: smine-batch v<version>` (this skill's frontmatter version) next to the date; smine-summary extracts it into the batch JSON.
- Full queryable session IDs, always.
- Per session, one report line — `Skills invoked: <comma-separated skill names, or "none">` — feeding the smine-summary `invoked_skills` field.
- Curate: drop no-signal sessions (ghosts, trivial); list them as skipped with one word why.
- **Cross-session arcs**: link related sessions into arcs (design→implement→verify→refine); evaluate handoff quality and skill ordering across the arc — batch-level synthesis, not per-session extraction.
- Skill and Workflow candidates get their own headed section; Workflow candidates in the spec format above.
- Generalize lessons; specifics appear only as evidence quotes.
- **Explicit evidence, per point.** Every extracted finding spells out its evidence: 1–3 full session ids ranked strongest occurrence first, plus the verbatim quotes it rests on. Batch-level sections (arcs, candidate sections) attribute per evidence point, never only per section.
- **Verbatim code snippets.** When the transcript contains the offending code and/or the corrected code (pasted code, diffs, plan excerpts), include both verbatim in fenced blocks annotated `(violation)` / `(fix)` / `(context)` with a source note (`transcript` | `plan` | `diff`). Diffs of pruned worktrees are gone — best effort via the plan text; never reconstruct code the transcript does not show.
- Append IDs to the ledger, then STOP for user review / context management (in `--nightly` mode: skip the STOP, continue with the next batch).
- Downstream: the `/smine` pipeline routes a finished batch to the dimension skills (memory, skills, workflows, routines, rules, JSON summary) — not this skill's job.

## Rules

- Full session IDs, never truncated.
- Artifacts are injected elsewhere as-is: self-contained, no session-local shorthand or invented codenames.
- Write outputs to the analysis dir, never only to tmp/scratchpad — tmp gets cleaned.
- Wait for background-subagent completion notifications before treating their output as missing.
- User cursing is signal, not noise — always dig in.

## Model

- Suggested: frontier / high
- Reason: mining long transcripts for non-obvious lessons
- Tested unviable: — (none yet)

## Changelog

- v1.14 (2026-07-31): activity-scoped context — enforcement-surface list names style/ guides and rules/ activity chapters
- v1.13 (2026-07-30): context redesign — enforcement-surface list names context/rules entries and fdesign assets
- v1.12 (2026-07-30): allowed-tools permission manifest declared
- v1.11 (2026-07-27): renamed smine → smine-batch; now stage 1 of the /smine pipeline (fronted by skills/smine/smine); Analyzer header line updated (Analyzer: smine-batch v<version>); enable-model-invocation moved to the front skill; triggers narrowed to avoid colliding with /smine; effort token normalized (large → high)
- v1.10 (2026-07-26): Args section
- v1.9 (2026-07-24): renamed session-analyze → smine (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.8 (2026-07-19): batch header carries the producing skill version (Analyzer: session-analyze v<version>)
- v1.7 (2026-07-19): reference rename: eval-skill → couchskill-eval; moved under skills/session/
- v1.6 (2026-07-19): workflow script location updated to skill-bundled convention (skills/<skill>/workflows/), .claude/workflows/ reference removed
- v1.5 (2026-07-19): per-point evidence attribution (1–3 ranked session ids) + verbatim violation/fix snippets in reports
- v1.4 (2026-07-16): --nightly unattended mode (all pending batches, no review STOPs) + enable-model-invocation for headless `claude -p` expansion
- v1.3 (2026-07-15): Positive index (green counterpart of the frustration index)
- v1.2 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.1 (2026-07-11): explicit skill and workflow candidate extraction
- v1.0 (2026-07-03): initial version
