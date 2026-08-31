#!/usr/bin/env bash
# PreToolUse read-gate: deny tool calls until every required file has been
# read in full. Required = the language guide governing the touched file
# (context.json .guides[]) + files under ~/.claude/context-required/<session>/.
# Read coverage is the union of the transcript's Read tool calls and the
# coverage sidecar under ~/.claude/read-gate-state/ — the sidecar is written
# by this hook itself when it allows a Read of a required file, so coverage
# survives subagent sessions whose transcript view is stale or separate.
# Bash commands are gated too: tokens matching a guide glob require that
# guide, and exactly `cat <required-file>` counts as a full read. The
# tokenizer is deliberately conservative — quoted paths with spaces, $VAR
# expansions and heredocs are not seen and fail open.
set -euo pipefail

if [ -z "${READ_GATE_ENABLED:-}" ] && [ -f "$HOME/.claude/hooks/read-gate.env" ]; then
	# shellcheck disable=SC1091
	. "$HOME/.claude/hooks/read-gate.env"
fi
[ "${READ_GATE_ENABLED:-1}" = "0" ] && exit 0
command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
field() { jq -r ".$1 // empty" <<<"$input"; }
tool=$(field tool_name)
file_path=$(field tool_input.file_path)
command=$(field tool_input.command)
cwd=$(field cwd)
transcript=$(field transcript_path)
session_id=$(field session_id)
agent_id=$(field agent_id)

state_dir="$HOME/.claude/read-gate-state"
state_file=""
[ -n "$session_id" ] && state_file="$state_dir/${session_id}__${agent_id:-main}.jsonl"

# Fail-open: proceed when either coverage source is usable, silent otherwise.
[ -n "$transcript" ] && [ -r "$transcript" ] || transcript=""
{ [ -n "$transcript" ] || { [ -n "$state_file" ] && [ -f "$state_file" ]; }; } || exit 0

# shellcheck disable=SC1091
[ -f "$HOME/.claude/hooks/global-context.env" ] && . "$HOME/.claude/hooks/global-context.env"
ctx_dir="$cwd/${AGENT_CONTEXT_DIR_DEFAULT:-docs}"

# guide_for_path <path>: prints the guide governing the path, per the index's globs.
guide_for_path() {
	[ -f "$ctx_dir/context.json" ] || return 0
	set -f
	jq -r '.guides[]? | "\(.path)\t\(.files | join(" "))"' "$ctx_dir/context.json" |
	while IFS=$'\t' read -r path globs; do
		for glob in $globs; do
			# shellcheck disable=SC2254
			case "$1" in $glob) [ -f "$ctx_dir/$path" ] && echo "$ctx_dir/$path"; return 0 ;; esac
		done
	done
	set +f
}

# bash_guide_candidates: conservative token split of the Bash command —
# metacharacters become separators, surrounding quotes are stripped, flags
# and extension-less words are dropped. Misses fail open by design.
bash_guide_candidates() {
	printf '%s\n' "$command" |
	tr '&;|()<>' '       ' |
	tr ' \t' '\n' |
	sed -e "s/^['\"]//" -e "s/['\"]\$//" |
	grep -v '^-' | grep '\.' | sort -u || true
}

# required_files: one path per line — the touched language guide(s), then the
# session's skill-context files (main conversation only).
required_files() {
	case "$tool" in
	Read|Edit|Write) [ -n "$file_path" ] && guide_for_path "$file_path" ;;
	Bash)
		[ -n "$command" ] && bash_guide_candidates | while IFS= read -r token; do
			guide_for_path "$token"
		done ;;
	esac
	[ -n "$session_id" ] || return 0
	if [ -z "$agent_id" ]; then
		for f in "$HOME/.claude/context-required/$session_id"/*.md; do
			[ -f "$f" ] && echo "$f"
		done
	else
		# Subagents owe only the spills of skills they invoked themselves.
		[ -n "$transcript" ] || return 0
		jq -rs '[ .[] | select(.type == "assistant") | .message.content[]?
			| select(.type == "tool_use" and .name == "Skill") | .input.skill // empty ] | unique | .[]' \
			"$transcript" 2>/dev/null | while IFS= read -r s; do
			[ -n "$s" ] && [ -f "$HOME/.claude/context-required/$session_id/$s.md" ] \
				&& echo "$HOME/.claude/context-required/$session_id/$s.md"
		done
	fi
}

# exact_cat_target: prints the target path iff the Bash command is exactly
# `cat <path>` — two words, no flags, no shell metacharacters or quoting.
exact_cat_target() {
	case "$command" in
	*";"*|*"|"*|*"&"*|*"<"*|*">"*|*'$'*|*'`'*|*"'"*|*'"'*|*"("*|*")"*) return 0 ;;
	esac
	# shellcheck disable=SC2086 # deliberate word split of a metachar-free command
	set -- $command
	[ "$#" -eq 2 ] && [ "$1" = "cat" ] && printf '%s\n' "$2"
	return 0
}

# trackable <path>: 0 iff the path is a coverage-tracked file — a language
# guide from the index or a session skill-context file. Only these earn
# sidecar records; reading them is what satisfies the gate.
trackable() {
	case "$1" in
	"$HOME/.claude/context-required/"*) return 0 ;;
	esac
	[ -f "$ctx_dir/context.json" ] || return 1
	local guide
	while IFS= read -r guide; do
		[ -n "$guide" ] || continue
		if [ "$ctx_dir/$guide" = "$1" ]; then
			return 0
		fi
	done <<<"$(jq -r '.guides[]?.path // empty' "$ctx_dir/context.json" 2>/dev/null || true)"
	return 1
}

# transcript_pairs <path>: "start end" per transcript Read of the path
# (suffix match on the last two components — tolerant of worktree/symlink
# prefix differences).
transcript_pairs() {
	[ -n "$transcript" ] || return 0
	local suffix
	suffix="/$(basename "$(dirname "$1")")/$(basename "$1")"
	jq -r --arg suffix "$suffix" '
		select(.type == "assistant") | .message.content[]?
		| select(.type == "tool_use" and .name == "Read" and ((.input.file_path // "") | endswith($suffix)))
		| "\(.input.offset // 1) \((.input.offset // 1) + (.input.limit // 2000) - 1)"
	' "$transcript" 2>/dev/null || true
}

# sidecar_pairs <path>: "start end" per coverage record for the path;
# malformed lines and a missing file yield nothing.
sidecar_pairs() {
	[ -n "$state_file" ] && [ -f "$state_file" ] || return 0
	jq -Rr --arg f "$1" 'fromjson? | select(.file == $f) | "\(.s) \(.e)"' "$state_file" 2>/dev/null || true
}

# missing_ranges <path>: "start end" per unread line range over the union of
# transcript and sidecar coverage, empty when fully read.
missing_ranges() {
	local path="$1" total
	total=$(wc -l <"$path" | tr -d ' ')
	{ transcript_pairs "$path"; sidecar_pairs "$path"; } | jq -Rrn --argjson total "$total" '
		[inputs | select(test("^[0-9]+ [0-9]+$")) | split(" ") | { s: (.[0]|tonumber), e: (.[1]|tonumber) }]
		| sort_by(.s)
		| reduce .[] as $r ({ next: 1, gaps: [] };
		    (if $r.s > .next then .gaps + [[.next, $r.s - 1]] else .gaps end) as $gaps
		    | { next: ([$r.e + 1, .next] | max), gaps: $gaps })
		| (if .next <= $total then .gaps + [[.next, $total]] else .gaps end)
		| .[] | select(.[0] <= $total) | "\(.[0]) \([.[1], $total] | min)"
	' 2>/dev/null || true
}

# record <path> <start> <end>: queue a coverage record; flushed to the
# sidecar only when the final decision is allow. Optimistic by design — a
# later permission denial of the allowed Read over-records once, which fails
# open, consistent with the gate's posture.
pending=""
record() {
	local line
	line=$(jq -nc --arg f "$1" --arg s "$2" --arg e "$3" '{file: $f, s: ($s|tonumber), e: ($e|tonumber)}' 2>/dev/null) || return 0
	pending="$pending$line
"
}

# Queue a coverage record when this call reads a tracked file — the
# remediation Read of a guide carries no requirement of its own, so recording
# keys on the target, not on this call's requirement set.
cat_target=""
if [ "$tool" = "Read" ] && [ -n "$file_path" ] && [ -f "$file_path" ] && trackable "$file_path"; then
	offset=$(field tool_input.offset)
	limit=$(field tool_input.limit)
	case "$offset" in ''|*[!0-9]*) offset=1 ;; esac
	case "$limit" in ''|*[!0-9]*) limit=2000 ;; esac
	record "$file_path" "$offset" $((offset + limit - 1))
fi
if [ "$tool" = "Bash" ] && [ -n "$command" ]; then
	cat_target=$(exact_cat_target)
	if [ -n "$cat_target" ] && [ -f "$cat_target" ] && trackable "$cat_target"; then
		record "$cat_target" 1 "$(wc -l <"$cat_target" | tr -d ' ')"
	else
		cat_target=""
	fi
fi

missing=""
while IFS= read -r path; do
	[ -n "$path" ] || continue
	if [ "$tool" = "Read" ] && [ "$file_path" = "$path" ]; then
		continue # reading a required file is always allowed
	fi
	if [ "$tool" = "Bash" ] && [ "$cat_target" = "$path" ]; then
		continue # a plain cat of a required file is a full read
	fi
	ranges=$(missing_ranges "$path")
	[ -n "$ranges" ] || continue
	missing="$missing
- $path — unread:"
	while read -r s e; do
		missing="$missing
    Read $path offset $s limit $((e - s + 1))"
	done <<<"$ranges"
done <<<"$(required_files | sort -u)"

if [ -z "$missing" ]; then
	if [ -n "$pending" ] && [ -n "$state_file" ]; then
		mkdir -p "$state_dir"
		find "$state_dir" -type f -mtime +7 -delete 2>/dev/null || true
		printf '%s' "$pending" >> "$state_file"
	fi
	exit 0
fi

reason="Required reading first (read-gate). Read the files below in full with the offset/limit pairs listed, then retry this $tool.$missing"
if [ "$tool" = "Bash" ]; then
	reason="$reason
Bash reads do not count as coverage — use Read (a plain cat of the required file itself is the only exception)."
fi
jq -n --arg reason "$reason" '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $reason}}'
