---
name: merge-risk
description: Report file overlap between the current plan and other recent sessions on the same repo — a pre-implementation risk report, never a fix. Trigger on /merge-risk [plan file] or "will this collide with other sessions". Args — plan file: plan whose Changes paths seed the check (default: this session's plan or uncommitted diff); window: recency cutoff in days (default 7); codex: also scan Codex sessions.
author: Kevin Horst
version: 1.3
argument-hint: "[plan file] [window] [codex]"
allowed-tools: mcp__Peek_MCP__session_list, mcp__Peek_MCP__session_diff, mcp__Peek_MCP__session_uncommitted_diff, mcp__Peek_MCP__session_plan, ToolSearch
---

# Merge Risk

Before implementing a plan, check whether any other recent session on the same repo is touching (or planning to touch) the same files. Output is a risk report; this skill never merges, never fixes, never blocks — the user decides what to do with an overlap.

## When to use

**Use when:** about to start implementation (typically between plan approval and /fimplement) and other sessions may be active on the same repo; or the user asks "will this collide".
**Don't use when:** the branches already conflict — /merge-resolve. Reviewing what another session did — /peek. Mining sessions — /smine.
**Preconditions:** peek-mcp available; a file set to check (an approved plan's Changes paths, or the current worktree's diff).
**Workflow position:** standalone, optional pre-implementation check: fdesign (any route) → (merge-risk) → fimplement (see README.md § Skill map, smine repo).

## Args

- plan file: positional — the plan whose Changes `location:` paths seed the check; absent → this session's plan, else the current worktree's `git diff --name-only` vs the base branch.
- `window`: recency cutoff in days for candidate sessions (default 7).
- `codex`: also scan Codex sessions (default: Claude only).

## Method

1. **Own file set.** Collect repo-relative paths from the plan's Changes `location:` lines (or the diff fallback). Empty set → report "nothing to check" and stop.
2. **Candidate sessions.** `session_list` (agent `claude`; plus `codex` with the arg). Keep items where `meta.cwd`, after stripping a trailing `/.claude/worktrees/<name>`, equals this repo's root (same normalization applied to our own cwd); drop items without `meta`; drop our own session (cwd equals this worktree path); drop `last_active` older than the window.
3. **Their file sets.** Per candidate: `session_diff` and `session_uncommitted_diff` → paths from `+++ b/<path>` lines (authoritative); `session_plan` → repo-relative path tokens in prose (weak signal, labeled *planned*). An empty diff vs the target branch means the session's work is merged or untouched — skip it unless the plan signal hits.
4. **Report.** Intersect per candidate and emit one table: Session (title or id) | Branch (`meta.git_branch`) | Last active | Overlapping files | Signal (`committed` / `uncommitted` / `planned`). Close with a one-line verdict: `clear — no overlap` or `overlap — N sessions touch M shared files`. Overlaps in the *planned*-only signal are flagged as tentative.

## Rules

- Read-only: no git mutations, no messages to other sessions, no fixes.
- Never assert absence stronger than the data: sessions without `meta.cwd` are listed as "unattributable" when any exist, not silently dropped.
- Path normalization is exact-match on the repo root — never substring matching (nested checkouts would false-positive).

## Model

- Suggested: any / low
- Reason: mechanical list-filter-intersect over structured tool output
- Tested unviable: — (none yet)
