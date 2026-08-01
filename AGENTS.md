# Agent guidance

## Skill authoring

- Every skill keeps three version surfaces in sync: the SKILL.md frontmatter `version`, the SKILL.md `## Changelog` section (human/agent-facing), and `changelog.json` next to SKILL.md (server-facing, JSON array of `{version, date, text}`, newest first). A version bump touches all three.
- SKILL.md frontmatter carries `name`, `description`, `author`, `version`, and optionally `allowed-tools` — single-line values preferred for own skills; the config-server parser reads line-wise between the fences but tolerates `>`/`|` block-scalar descriptions from third-party skills.
- `allowed-tools` is the skill's permission manifest: one line, comma-separated Claude Code permission rules in the same dialect as the settings/claude_code/settings.json allow list, pre-approved while the skill is active — never a restriction. Required on bounded-surface skills; implementation skills declare mechanics only; fixed-callee orchestrators declare the union over their callees. Replaces the retired `Command surface:` Model line.
- A skill with invocation-time args declares them verbosely in a `## Args` section (`- <name>: <doc>` bullets) and compressed in the description. Description shape, one line: short sentence what the skill does, `Trigger on /<name> or "<phrase>"`, then `Args — <name>: <doc>; …` for arg-bearing skills — the harness listing shows it verbatim.

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
