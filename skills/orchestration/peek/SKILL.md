---
name: peek
description: Show the latest Claude Code or Codex session — turns, plan, diff, or list. Trigger on /peek [n] or "what Claude Code or Codex is doing". Args — n: recent-turn count (default 5); list|plan|diff: mode token; id or title: specific session, id wins; claude|codex: agent qualifier (default claude).
author: Kevin Horst
version: 1.7
allowed-tools: mcp__Peek_MCP__session_full, mcp__Peek_MCP__session_list, mcp__Peek_MCP__session_plan, mcp__Peek_MCP__session_diff, ToolSearch
---

## When to use

**Use when:** quick-viewing the latest Claude Code or Codex session — recent turns, plan, diff, or session list. Invoked via /peek [n] or "what is Claude doing".
**Don't use when:** systematically mining sessions for lessons — /smine (the pipeline; raw miner /smine-batch). Reading session transcripts for memory/rule extraction — /smine. Evaluating skill outputs — /skillroutine-eval.
**Preconditions:** peek-mcp available.
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo).

## Args

- n: positional, default 5 — number of recent turns.
- `list | plan | diff`: mode token — session list / current plan only / git diff only.
- id or title: positional — a specific session by UUID-style id or exact case-insensitive title; id wins over title.
- `claude | codex`: agent qualifier, default `claude` — which agent's sessions; combinable with the above.

## Routing

| Input | Tool | Notes |
|-------|------|-------|
| `/peek [n]`, "what is Claude doing", "show session" | `session_full` | n defaults to 5 |
| `/peek list` | `session_list` | shows all sessions with plan/diff flags |
| `/peek plan` | `session_plan` | current plan only |
| `/peek diff` | `session_diff` | git diff only |
| `/peek <id>` or `/peek <id> [n]` | `session_full` with `id` param | specific session by ID |
| `/peek <title>` or `/peek <title> [n]` | `session_full` with `title` param | exact title match (case-insensitive) |

When the input looks like a session ID (UUID-style), pass it as `id`. Otherwise pass it as `title`.
`id` takes precedence over `title` when both are provided.
Title matching is exact (case-insensitive) — substrings will not match.

All tools need a required `agent` param (`"claude"` or `"codex"`). Pass it when the
user qualifies the command, e.g. `/peek codex`. If the user doesn't qualify, default to Claude.

## Pagination

`session_full` responses may be paginated. When the response contains `has_more: true` and a `request_id`, you MUST call `session_full` again with that `request_id` to get the next page. Keep calling until `has_more` is false or `request_id` is absent. Do NOT call `session_diff` or `session_plan` separately — all content (turns, plan, diff) arrives through the paginated `session_full` responses.

## Output format

Do NOT reproduce the tool result. The data is already in context for the LLM — formatting it again wastes time and tokens.

After calling the tool, respond with only a short confirmation line, e.g.:

> Peeked at session **Login simplification** (5 turns, has plan, has diff).

Include: session title or ID, turn count, and which sections are present (plan/diff). Nothing else.

For `/peek list`, show the session table as-is — that is already compact.

## Model

- Suggested: small / low
- Reason: table-driven routing to MCP tools
- Tested unviable: — (none yet)

## Changelog

- v1.7 (2026-07-30): allowed-tools permission manifest declared
- v1.6 (2026-07-30): moved under skills/orchestration/ group; name and behavior unchanged
- v1.5 (2026-07-27): reference renames — /smine is now the pipeline (raw miner /smine-batch), /couchskill-eval → /skillroutine-eval
- v1.4 (2026-07-26): Args section
- v1.3 (2026-07-22): effort token normalized (small / low)
- v1.2 (2026-07-19): reference rename: eval-skill → couchskill-eval; moved under skills/util/
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-06-22): initial version
