#!/usr/bin/env bash
# UserPromptSubmit + PostToolUse(Skill) hook: when a skill is invoked, resolve
# the context-entry set its frontmatter `acdsl-context:` line declares against
# the current repo's deployed context index and inject the rendered entries
# into the conversation. (The key is acdsl-namespaced — bare `context:` is
# Claude Code's own frontmatter field and is never read here.) Read-side only —
# nothing is written to disk, so there is no strip step and no touched-files
# trace (deliberate contrast to acdsl-project.sh, which projects source files
# on disk for view == disk).
# No-ops silently when: disabled, no jq, not a skill invocation, skill not
# installed, no acdsl-context: declaration, no context index in cwd, or the
# invocation carries a bare --no-context token (per-invocation opt-out).
set -euo pipefail

# An explicit env var wins over the toggle file — tests and one-off runs stay
# deterministic regardless of the machine's current default.
if [ -z "${SKILL_CONTEXT_ENABLED:-}" ] && [ -f "$HOME/.claude/hooks/skill-context.env" ]; then
	# shellcheck disable=SC1091 # machine-local toggle file, not a repo input
	. "$HOME/.claude/hooks/skill-context.env"
fi
[ "${SKILL_CONTEXT_ENABLED:-1}" = "0" ] && exit 0
command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
event=$(jq -r '.hook_event_name // empty' <<<"$input")
cwd=$(jq -r '.cwd // empty' <<<"$input")
session_id=$(jq -r '.session_id // empty' <<<"$input")
[ -n "$cwd" ] || exit 0

# UserPromptSubmit ignores matchers, so both events funnel through this one
# script and the event filter lives here.
skill=""
case "$event" in
	UserPromptSubmit)
		prompt=$(jq -r '.prompt // .user_input // empty' <<<"$input")
		case "$prompt" in
			/*) skill="${prompt#/}"; skill="${skill%%[[:space:]]*}" ;;
			*) exit 0 ;;
		esac
		invocation="$prompt"
		;;
	PostToolUse)
		tool=$(jq -r '.tool_name // empty' <<<"$input")
		[ "$tool" = "Skill" ] || exit 0
		skill=$(jq -r '.tool_input.skill // empty' <<<"$input")
		invocation=$(jq -r '.tool_input.args // empty' <<<"$input")
		;;
	*) exit 0 ;;
esac
[ -n "$skill" ] || exit 0

# Per-invocation opt-out: a bare no-context flag token anywhere in the
# invocation suppresses injection for this call only. Accepted dash prefixes:
# -, --, and the macOS smart-dash forms – (en) and — (em). Punctuation-adjacent
# or backtick-quoted mentions deliberately do not match.
if printf '%s' "$invocation" | grep -qE '(^|[[:space:]])(-{1,2}|—|–)no-context([[:space:]]|$)'; then
	exit 0
fi

skill_md="$HOME/.claude/skills/$skill/SKILL.md"
[ -f "$skill_md" ] || exit 0

# First same-line acdsl-context: value inside the frontmatter fences.
declared=$(awk '
	NR == 1 && $0 != "---" { exit }
	NR > 1 && $0 == "---" { exit }
	NR > 1 && sub(/^acdsl-context:[ \t]*/, "") { print; exit }
' "$skill_md")
[ -n "$declared" ] || exit 0

# shellcheck disable=SC1091 # machine-local env, same file the global-context hook reads
[ -f "$HOME/.claude/hooks/global-context.env" ] && . "$HOME/.claude/hooks/global-context.env"
index="$cwd/${AGENT_CONTEXT_DIR_DEFAULT:-docs}/context.json"
[ -f "$index" ] || exit 0

# Declared-but-undeployed IDs are skipped silently: deployed indexes are
# legitimate subsets; typos are caught by the acdsl "skill-context" authoring gate.
# Two renderings from the same selection: the ID list (always printed — the
# used-context record reads it) and the bodies, rendered from the typed
# content object (statement, why, applies, evidence, location, bullets,
# notes, examples) — the .md is never read here.
selected=$(jq -c --arg declared "$declared" '
	($declared | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))) as $tokens
	| [ .entries[]
	    | . as $e
	    | select(any($tokens[];
	        . as $t
	        | if endswith("*") then ($e.id | startswith($t | rtrimstr("*")))
	          else $e.id == $t end))
	  ]
	| unique_by(.id)
' "$index")
[ "$(jq 'length' <<<"$selected")" -gt 0 ] || exit 0
id_list=$(jq -r '.[] | "- **\(.id)**"' <<<"$selected")
bodies=$(jq -r '
	def tagged(name; value): if (value // "") != "" then ["* \(name): \(value)"] else [] end;
	.[] | . as $e | .content
	| [ "**\($e.id)**" + (if ($e.enforcement // "") != "" then " `[\($e.enforcement)]`" else "" end) + " — \(.statement)", "" ]
	  + tagged("Why"; .why) + tagged("Applies"; .applies) + tagged("Evidence"; .evidence) + tagged("Location"; .location)
	  + ((.bullets // []) | map("* " + .))
	  + ((.notes // []) | if length > 0 then [""] + . else [] end)
	  + ((.examples // []) | map("", "```" + (.lang // ""), .code, "```"))
	  + [""]
	| join("\n")
' <<<"$selected")

header="===== SKILL CONTEXT /$skill (injected by hook) =====
$id_list"
body="$header

$bodies"

# Old session dirs are pruned on every skill invocation, not only on
# spill-writes — a spill-free stretch must not let them outlive the window.
if [ -d "$HOME/.claude/context-required" ]; then
	find "$HOME/.claude/context-required" -mindepth 1 -maxdepth 1 -type d -mtime +7 -exec rm -rf {} + 2>/dev/null || true
fi

# Over the harness cap (10,000 chars per hook output) the bodies go to a
# per-session file the read-gate hook enforces; the injection keeps the IDs
# and points at the file.
if [ "${#body}" -gt 9000 ] && [ -n "$session_id" ]; then
	dir="$HOME/.claude/context-required/$session_id"
	mkdir -p "$dir"
	target="$dir/$skill.md"
	printf '%s\n' "$bodies" > "$target"
	lines=$(wc -l <"$target" | tr -d ' ')
	body="$header
Written to $target (${#bodies} chars, $lines lines). Read it in full before any other tool call — the read-gate hook enforces this."
fi

if [ "$event" = "PostToolUse" ]; then
	jq -n --arg ctx "$body" \
		'{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $ctx}}'
else
	printf '%s\n' "$body"
fi
exit 0
