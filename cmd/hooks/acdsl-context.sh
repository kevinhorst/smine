#!/usr/bin/env bash
# PostToolUse(Read) hook: deliver the governing ACDSL rules for files that
# cannot carry an on-disk projection block (.json, .acdsl, binaries) as
# additionalContext after the read. Projectable files are the sibling
# acdsl-project.sh hook's business — project -context prints nothing for
# them, so the two channels never overlap.
# Caveat by design: additionalContext lands once per Read and is not re-shown
# in later turns — weaker than an on-disk block, the accepted fallback for
# file classes without comment syntax.
# No-ops silently outside repos with a verifier registry; never blocks.
set -euo pipefail

# An explicit env var wins over the toggle file — tests and one-off runs stay
# deterministic regardless of the machine's current default.
if [ -z "${ACDSL_CONTEXT_ENABLED:-}" ] && [ -f "$HOME/.claude/hooks/acdsl-context.env" ]; then
	# shellcheck disable=SC1091 # machine-local toggle file, not a repo input
	. "$HOME/.claude/hooks/acdsl-context.env"
fi
[ "${ACDSL_CONTEXT_ENABLED:-1}" = "0" ] && exit 0
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
rules=$(cd "$cwd" && $acdsl_cmd project -context "$rel" 2>/dev/null) || exit 0
[ -n "$rules" ] || exit 0

context="ACDSL rules governing $rel (gate-enforced by acdsl check; the file cannot carry a comment block):
$rules"
jq -n --arg ctx "$context" '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $ctx}}'
exit 0
