#!/usr/bin/env bash
# Used-context record: what context reached the model in one Claude Code
# session, read passively from its transcript JSONL. One JSON object on
# stdout; consumers: /smine-batch (batch JSON sessions[].context) and
# /skillroutine-eval (context axis). Rows: hook-injected skill/global
# context (attachment.hook_success.stdout — full text, never the capped
# content stub), ACDSL projection blocks in Read results, plan-time rules
# (acdsl project -plan output), pack reads (Read of <cwd>/<ctx-dir>/…),
# subagent-inlined IDs/paths (Agent prompts), and the language row — guide
# Reads (path suffix /rules/<name>.md, forced by the read-gate) with their
# IDs and a coverage verdict against the guide file when it is readable here.
# Harness-loaded prose (CLAUDE.md, memory) is not in the transcript
# and is deliberately not guessed.
# usage: context_record.sh <transcript.jsonl> [ctx-dir]   (ctx-dir default: docs)
set -euo pipefail

transcript="${1:-}"
ctx_dir="${2:-${AGENT_CONTEXT_DIR_DEFAULT:-docs}}"
[ -n "$transcript" ] && [ -f "$transcript" ] || { echo "usage: $0 <transcript.jsonl> [ctx-dir]" >&2; exit 2; }
command -v jq >/dev/null 2>&1 || { echo "jq required" >&2; exit 2; }

# Guide line totals for the language row's coverage — the guide files the
# session read, when they still exist here; missing → coverage "unknown".
totals=$(jq -r -s '[.[] | select(.type == "assistant") | .message.content[]? | select(.type == "tool_use" and .name == "Read")
  | .input.file_path // "" | select(test("/rules/[a-z]+\\.md$"))] | unique | .[]' "$transcript" |
  while IFS= read -r guide; do
    if [ -r "$guide" ]; then printf '%s\t%s\n' "$guide" "$(wc -l <"$guide" | tr -d ' ')"; fi
  done | jq -R -s 'split("\n") | map(select(length > 0) | split("\t") | {key: .[0], value: (.[1] | tonumber)}) | from_entries')
[ -n "$totals" ] || totals='{}'

jq -s --arg ctx "$ctx_dir" --argjson totals "$totals" '
  def ids: [scan("\\b[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)+-[0-9]{3}\\b")] | unique;
  def hook_text: (.stdout // "") as $s | ((try ($s | fromjson | .hookSpecificOutput.additionalContext) catch null) // $s);
  def coverage($ranges; $total):
    if $total == null then "unknown"
    else ($ranges | sort_by(.[0]) | reduce .[] as $r ({next: 1, ok: true};
           (if $r[0] > .next then .ok = false else . end) | .next = ([$r[1] + 1, .next] | max))
          | if .ok and .next > $total then "complete" else "partial" end) end;
  (map(select(.type == "user" and (.cwd // "") != "")) | first | .cwd // "") as $cwd
  | (map(select(.type == "attachment" and .attachment.type == "hook_success") | .attachment | hook_text)) as $hooks
  | (map(select(.type == "assistant") | .message.content[]? | select(.type == "tool_use"))) as $uses
  | ($uses | map(select(.name == "Read") | {key: .id, value: .input.file_path}) | from_entries) as $read_paths
  | ($uses | map(select(.name == "Bash" and (.input.command // "" | test("acdsl project -plan"))) | .id)) as $plan_uses
  | (map(select(.type == "user") | .message.content[]? | select(.type == "tool_result")
      | {id: .tool_use_id, text: (.content | if type == "string" then . else (map(.text // "") | join("\n")) end)})) as $results
  | ($uses | map(select(.name == "Read" and ((.input.file_path // "") | test("/rules/[a-z]+\\.md$"))))
      | map({id: .id, guide: .input.file_path, lang: (.input.file_path | capture("/rules/(?<l>[a-z]+)\\.md$").l),
             range: [(.input.offset // 1), ((.input.offset // 1) + (.input.limit // 2000) - 1)]})) as $guide_reads
  | {
      session_id: (map(select(.sessionId != null)) | first | .sessionId),
      cwd: $cwd,
      injected: {
        global: ($hooks | map(select(startswith("===== GLOBAL CONTEXT")))
                 | map([scan("===== ([^=\\n]+) =====") | .[0]] | map(select(. != "GLOBAL CONTEXT (injected by hook)"))) | add // []),
        skill: ($hooks | map(select(startswith("===== SKILL CONTEXT /")))
                 | map({skill: (capture("SKILL CONTEXT /(?<s>[^ ]+)").s), ids: ids, via: "marker"})),
        lang: ($guide_reads | group_by(.guide) | map({
                 lang: .[0].lang, guide: .[0].guide,
                 ids: ([.[] | .id as $i | ($results | map(select(.id == $i)) | .[0].text // "")] | join("\n") | ids),
                 coverage: coverage(map(.range); $totals[.[0].guide])}))
      },
      acdsl_rules: ($results | map(select(.text | test("\\[ACDSL-PROJECTION\\] [0-9]+ rule")))
                    | map({file: ($read_paths[.id] // "?"), ids: (.text | [scan("- \\[(ACDSL-[A-Z0-9-]+)\\]") | .[0]] | unique)})),
      plan_rules: ($results | map(select(.id as $i | $plan_uses | index($i))) | map(.text | [scan("- \\[([A-Z0-9-]+)\\]") | .[0]]) | add // [] | unique),
      pack_reads: ($read_paths | to_entries | map(.value) | map(select(startswith($cwd + "/" + $ctx + "/"))) | unique),
      subagent_context: ($uses | map(select(.name == "Agent") | (.input.prompt // "")
                          | {ids: ids, paths: ([scan("[^ `\"\\n]*" + $ctx + "/[^ `\"\\n]+")] | unique)})
                          | map(select(.ids != [] or .paths != [])))
    }
' "$transcript"
