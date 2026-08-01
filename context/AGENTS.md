<!-- Source of truth: smine repo. {{ROLE}}, {{CONTEXT_DIR}} and {{LANG:x}}...{{/LANG:x}}
     are template markers expanded by cmd/sync/sync_context.sh on deployment. -->

# Agents

{{ROLE}}

## Context pack

- `{{CONTEXT_DIR}}/rules/` — doctrine as typed entries (`FACT-*`, `NEVER-*`, `ALWAYS-*`) in activity chapters: `concepting.md` (hot-class gates), `implementing.md` (scoping, stop conditions, data integrity), `reviewing.md` (Definition of Done — every item PASS or N/A with a stated reason), `navigating.md`; orient here first and cite entry IDs when flagging violations
- `{{CONTEXT_DIR}}/facts/` — facts about this repo (stack, architecture, environment)
- `{{CONTEXT_DIR}}/style/` — artifact style guides: `plan.md` (plans)
- Keep `AGENTS.md` and `{{CONTEXT_DIR}}/` up to date as part of the change when instructions, workflows, or expectations change

## Workflow routing

- Non-trivial features start from an approved plan (fdesign skill); changes to existing features — adjustments, contract changes, restructuring — start from a change plan (fchange skill)

## Subagents

- Subagents do not inherit this file. Any subagent prompt MUST instruct the subagent to read `AGENTS.md` and `{{CONTEXT_DIR}}/` first, or inline the constraints that matter for its task
