#!/usr/bin/env bash
# PreToolUse(Write|Edit|NotebookEdit) guard: in a repo with a non-free
# acdsl/policy.json, deny writes to the self-management surface (both
# modes) and to any rule file (strict). Gated rule edits pass here and are
# judged authoritatively by acdsl check (bin/acdsl CheckPolicy) — this hook
# is fast feedback, fail-open by design.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
cwd=$(jq -r '.cwd // empty' <<<"$input")
file_path=$(jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' <<<"$input")
[ -n "$cwd" ] && [ -n "$file_path" ] || exit 0

case "$file_path" in
  /*) abs="$file_path" ;;
  *) abs="$cwd/$file_path" ;;
esac

root="$cwd"
while [ "$root" != "/" ] && [ ! -f "$root/acdsl/policy.json" ]; do
  root=$(dirname "$root")
done
[ -f "$root/acdsl/policy.json" ] || exit 0

mode=$(jq -r '.mode // "free"' "$root/acdsl/policy.json" 2>/dev/null) || exit 0
[ "$mode" = "strict" ] || [ "$mode" = "gated" ] || exit 0

deny() {
  jq -n --arg reason "$1" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $reason}}'
  exit 0
}

case "$abs" in
  "$root"/*) ;;
  *) exit 0 ;; # outside the policy repo — not this guard's business
esac
rel="${abs#"$root"/}"

case "$rel" in
  acdsl/policy.json|acdsl/policy.schema.json|acdsl/registry.json|acdsl/registry.local.json|acdsl/evalgen.json)
    deny "acdsl policy mode is $mode: the self-management surface ($rel) changes only on the base branch (see acdsl/README.md, Modes)" ;;
esac

if [ "$mode" = "strict" ]; then
  case "$rel" in
    *.acdsl|acdsl/*)
      deny "acdsl policy mode is strict: rule files are not editable in this repo ($rel)" ;;
  esac
fi

exit 0
