#!/usr/bin/env bash
# PreToolUse(Write|Edit|NotebookEdit) guard: a session anchored in a pool
# worktree may write only inside that worktree. Denies writes into the main
# checkout or sibling worktrees — the escape path weak subagents take
# (UserPromptSubmit guards never see subagent tool calls; PreToolUse does).
# Writes anywhere else (tmp, ~/.claude plans/memory) stay allowed. Bash is
# deliberately not matched: command strings are unparseable heuristics.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
cwd=$(jq -r '.cwd // empty' <<<"$input")
file_path=$(jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' <<<"$input")
[ -n "$cwd" ] && [ -n "$file_path" ] || exit 0

case "$cwd" in
  */.claude/worktrees/*) ;;
  *) exit 0 ;; # not a pool-worktree session — guard does not apply
esac

main_root="${cwd%%/.claude/worktrees/*}"
rest="${cwd#*/.claude/worktrees/}"
worktree_name="${rest%%/*}"
worktree_root="$main_root/.claude/worktrees/$worktree_name"

case "$file_path" in
  /*) abs="$file_path" ;;
  *) abs="$cwd/$file_path" ;;
esac

deny() {
  jq -n --arg reason "$1" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $reason}}'
  exit 0
}

case "$abs" in
  *"/../"*) deny "path contains ../ segments — write with a normalized absolute path (session worktree: $worktree_root)" ;;
esac

case "$abs" in
  "$worktree_root"/*) exit 0 ;; # inside the session worktree
  "$main_root"/*) deny "session is worktree-anchored: writes go under $worktree_root — refusing write into the main checkout or a sibling worktree: $abs" ;;
  *) exit 0 ;; # outside the repo entirely (tmp, ~/.claude, ...) — allowed
esac
