#!/usr/bin/env bash
# PreToolUse(Read) hook: project the file on disk before the agent reads it —
# the governing ACDSL rules land as a comment block above the content, so the
# Read result, the disk state, and every later Edit anchor agree (view == disk).
# Side benefit: the dirty set doubles as a touched-files trace — a projected
# file is one an agent actually read. Blocks are stripped before commit
# (acdsl project -strip); the check gate refuses staged blocks.
# No-ops silently outside repos with a verifier registry; never blocks a read.
set -euo pipefail

# An explicit env var wins over the toggle file — tests and one-off runs stay
# deterministic regardless of the machine's current default.
if [ -z "${ACDSL_PROJECT_ENABLED:-}" ] && [ -f "$HOME/.claude/hooks/acdsl-project.env" ]; then
	# shellcheck disable=SC1091 # machine-local toggle file, not a repo input
	. "$HOME/.claude/hooks/acdsl-project.env"
fi
[ "${ACDSL_PROJECT_ENABLED:-1}" = "0" ] && exit 0
command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
file_path=$(jq -r '.tool_input.file_path // empty' <<<"$input")
cwd=$(jq -r '.cwd // empty' <<<"$input")
[ -n "$file_path" ] && [ -n "$cwd" ] || exit 0
[ -f "$cwd/acdsl/registry.json" ] || exit 0

case "$file_path" in
  "$cwd"/*) rel="${file_path#"$cwd"/}" ;;
  *) exit 0 ;;
esac

acdsl_cmd="go run ./cmd/acdsl"
for candidate in "$cwd/bin/acdsl" "$cwd/bin/acdsl.exe"; do
  [ -x "$candidate" ] && { acdsl_cmd="$candidate"; break; }
done
(cd "$cwd" && $acdsl_cmd project -file "$rel" >/dev/null 2>&1) || true
exit 0
