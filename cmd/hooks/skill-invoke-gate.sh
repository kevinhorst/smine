#!/usr/bin/env bash
# PreToolUse skill-invoke-gate: a session whose FIRST user message carries a
# "REQUIRE-SKILL:<name>" marker (railroad lanes) must invoke Skill(<name>)
# before any other tool call; a prompt-start "/<name>" first message counts
# as invoked (headless lanes). After invocation, direct reads of files under
# the repo context dir stay denied — the context arrives injected; language
# guides (context.json .guides[]) are exempt, the read-gate requires those.
# Fail-open on missing jq, unreadable transcript, or absent marker.
set -euo pipefail

if [ -z "${SKILL_INVOKE_GATE_ENABLED:-}" ] && [ -f "$HOME/.claude/hooks/skill-invoke-gate.env" ]; then
	# shellcheck disable=SC1091
	. "$HOME/.claude/hooks/skill-invoke-gate.env"
fi
[ "${SKILL_INVOKE_GATE_ENABLED:-1}" = "0" ] && exit 0
command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
field() { jq -r ".$1 // empty" <<<"$input"; }
tool=$(field tool_name)
file_path=$(field tool_input.file_path)
command=$(field tool_input.command)
cwd=$(field cwd)
transcript=$(field transcript_path)
[ -n "$transcript" ] && [ -r "$transcript" ] || exit 0

first_user=$(jq -rs '
	[ .[] | select(.type == "user") ][0].message.content
	| if type == "array" then map(.text // "") | join("\n") else (. // "") end
' "$transcript" 2>/dev/null || true)
required=$(printf '%s\n' "$first_user" | sed -n 's/.*REQUIRE-SKILL:\([a-z0-9-]*\).*/\1/p' | head -1)
[ -n "$required" ] || exit 0

deny() {
	jq -n --arg reason "$1" \
		'{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $reason}}'
	exit 0
}

# Invoked = a Skill tool_use for the required skill, or a prompt-start /<skill>
# first message (headless: the skill loads at prompt submit, no Skill call).
case "$first_user" in "/$required"*) invoked=1 ;; *) invoked="" ;; esac
if [ -z "$invoked" ]; then
	n=$(jq -rs --arg s "$required" '
		[ .[] | select(.type == "assistant") | .message.content[]?
		  | select(.type == "tool_use" and .name == "Skill" and .input.skill == $s) ] | length
	' "$transcript" 2>/dev/null || echo 0)
	[ "${n:-0}" -gt 0 ] && invoked=1
fi
if [ -z "$invoked" ]; then
	[ "$tool" = "Skill" ] && exit 0
	deny "skill-invoke-gate: invoke the Skill tool with skill=\"$required\" (bare, no args) before any other tool call — your review context arrives via that invocation."
fi

# Post-invocation: context-entry files are injected — direct reads are denied.
# shellcheck disable=SC1091
[ -f "$HOME/.claude/hooks/global-context.env" ] && . "$HOME/.claude/hooks/global-context.env"
ctx_dir="$cwd/${AGENT_CONTEXT_DIR_DEFAULT:-docs}"
[ -d "$ctx_dir" ] || exit 0

target=""
case "$tool" in
Read) target="$file_path" ;;
Bash)
	case "$command" in
	*";"*|*"|"*|*"&"*|*"<"*|*">"*|*'$'*|*'`'*|*"'"*|*'"'*|*"("*|*")"*) ;;
	*)
		# shellcheck disable=SC2086
		set -- $command
		[ "$#" -eq 2 ] && [ "$1" = "cat" ] && target="$2"
		;;
	esac
	;;
esac
case "$target" in
"$ctx_dir"/*)
	guide=""
	while IFS= read -r g; do
		[ -n "$g" ] && [ "$ctx_dir/$g" = "$target" ] && guide=1
	done <<<"$(jq -r '.guides[]?.path // empty' "$ctx_dir/context.json" 2>/dev/null || true)"
	[ -n "$guide" ] && exit 0
	deny "skill-invoke-gate: context entries are injected via your skill invocation — do not read $target directly (language guides are the only exception)."
	;;
esac
exit 0
