#!/usr/bin/env bash
# Shared skill-manifest helpers for routine wrappers. Sourced, never executed.

# routine_allowed_tools <skill-name> — prints the single-line allowed-tools
# frontmatter value of the installed skill (~/.claude/skills — the copy the
# claude run loads); a missing file or field prints nothing.
routine_allowed_tools() {
  local manifest="$HOME/.claude/skills/$1/SKILL.md"
  [ -f "$manifest" ] || return 0
  awk '/^---$/{fence++; next} fence==1 && sub(/^allowed-tools:[ \t]*/, "") {print; exit}' "$manifest"
}
