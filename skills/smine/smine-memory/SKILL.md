---
name: smine-memory
description: Route memory candidates from smine-batch reports into auto-memory, then consolidate. Trigger on /smine-memory or "mine batch reports for memory updates". Args — batch file: one batch; absent means all batches with ledger-missing sessions.
author: Kevin Horst
version: 1.9
allowed-tools: Read, Write, Edit, Skill(consolidate-memory)
---

# smine: memory

Route memory candidates out of smine-batch reports. The load-bearing step is evaluation: the ad-hoc run reduced 323 raw candidates to 10 applied.

## When to use

**Use when:** mining batch reports for memory updates — extracting, evaluating, and applying memory candidates. Invoked via /smine-memory or as part of the /smine fan-out.
**Don't use when:** adding a single known fact to memory — write it directly per auto-memory conventions. Extracting rules, skills, workflows, or routines — the other dimension skills handle those. Writing a batch report — /smine-batch.
**Preconditions:** one or more completed batch reports under `sessions/`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-memory** (see `docs/skill-map.md`, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.

## 0. Setup

- Input: batch reports `sessions/{personal,work}/*batch-*.md` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`).
- Ledger: `sessions/<scope>/analyzed-memory.txt` (historical filename, predates the smine rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Old batches predate dedicated sections — mine the prose for memory candidates, not only "Memory candidates" headings.

## 1. Extract & cluster

- Collect memory candidates from the in-scope batches, with their evidence (session IDs, quotes).
- Cluster duplicates across batches — the same fact recurs; recurrence count is evidence, keep it.

## 2. Evaluate (the filter)

Drop clusters that are:
- Already covered by existing memory (read MEMORY.md index + the fact files) or global CLAUDE.md.
- Derivable from the repo itself (code, git history, CLAUDE.md/AGENTS.md of the target project).
- Wrong surface: enforcement-doc rules (RULE-*, FACT/NEVER/ALWAYS entries, the reviewing.md DoD, style/plan.md, AGENTS.md material) are not memory — skip with a note; /smine-style extracts them from the same batches itself, no handoff.

## 3. Route & apply

- Scope each survivor per the memory conventions: user-level facts → smine (global CLAUDE.md / its memory), project facts → that project's auto-memory dir under `~/.claude/projects/<munged-path>/memory/`. Never stash cross-project facts in whichever project is open.
- If a target project's memory dir doesn't exist, report it instead of creating it blindly.
- Write each fact per convention: one fact per file, `type_kebab-slug.md` (type ∈ user | feedback | project | reference), frontmatter, one index line in that project's MEMORY.md.
- Run the built-in consolidate-memory skill (Skill tool) over the current project's memory to merge/prune.

## 4. Report

- Applied (per target), dropped-covered, dropped-derivable, skipped-wrong-surface — counts plus the survivor list.
- Append the processed session IDs to the ledger and set its `Last analyzed batch:` first line (insert if absent), then STOP for user review.

## Rules

- Full session IDs, never truncated; evidence quotes verbatim.
- Better to drop than to bloat: an unclear candidate is a drop, not a hedge-write.
- Never edit external repos' checked-in docs from here — memory dirs and this repo only.

## Model

- Suggested: mid-tier / medium
- Reason: clustering candidates against existing memory
- Tested unviable: — (none yet)

## Changelog

- v1.9 (2026-07-31): session-scope rename to personal/work (input globs)
- v1.8 (2026-07-30): reference rename — smine-rules → smine-style
- v1.7 (2026-07-31): activity-scoped context — wrong-surface list names reviewing.md DoD and style/plan.md
- v1.6 (2026-07-30): context redesign — wrong-surface list includes FACT/NEVER/ALWAYS entries
- v1.5 (2026-07-30): allowed-tools permission manifest declared
- v1.4 (2026-07-27): renamed analyze-memory → smine-memory; part of the /smine fan-out; sibling handoff → /smine-rules
- v1.3 (2026-07-26): Args section
- v1.2 (2026-07-19): ledger carries a Last-analyzed-batch first line, updated on every append
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-11): initial version
