# Agents

You are a development agent for this repository.

## Context docs

- `context/actions/` — doctrine as typed entries (`ACTION-*`) in activity chapters: `concepting.md` (hot-class gates), `implementing.md` (scoping, stop conditions, data integrity), `reviewing.md` (Definition of Done — every item PASS or N/A with a stated reason), `navigating.md`; orient here first and cite entry IDs when flagging violations
- `context/facts/` — facts about this repo (stack, architecture, environment) as `FACT-*` entries
- `context/rules/` — artifact and code rules (`RULE-*`): `plan.md` (plans), `commits.md` (commit subjects), one file per language
  - each language guide declares the files it governs (`**Files:** \`*.go\``); the read-gate hook denies the first Read/Edit/Write of a governed file in a session until that guide has been read in full — this list is the index, not the delivery path
- `context/context.json` — every entry plus the aspect taxonomy in one generated machine-readable file
- Repos carrying `acdsl/registry.json`: run `./bin/acdsl project -strip` (fallback: `go run ./cmd/acdsl project -strip`) before staging files or running `make audit` — the projection blocks atop governed files are working-copy views; the check gate goes red on staged blocks
- Go code + test + goroutine style: context/rules/go.md
- Python code style: context/rules/python.md
- SQL code style incl. database MCP workflow: context/rules/sql.md — the JetBrains IDE MCP database tools are the source of truth for live schema state when available
- Keep `AGENTS.md` and `context/` up to date as part of the change when instructions, workflows, or expectations change

## Workflow routing

- Non-trivial features start from an approved plan (fdesign skill); changes to existing features — adjustments, contract changes, restructuring — start from a change plan (fdesign skill, change route)

## Subagents

- Subagents do not inherit this file. Any subagent prompt MUST instruct the subagent to read `AGENTS.md` and `context/` first, or inline the constraints that matter for its task
- Before fanning out subagents, verify the permission mode / allowlist can absorb their tool calls; arbitrary or compound shell commands are unallowlistable by prefix — explore inline with Read/Grep/Glob (which never prompt) instead. Never edit the user's settings files as a permission workaround — that is the user's call, made on explicit instruction only
