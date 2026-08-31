#!/usr/bin/env bash
# Regression coverage for skill-invoke-gate.sh: a REQUIRE-SKILL:<name> session
# must invoke the skill before any other tool call, then may not read context
# entry files directly (language guides excepted). No marker => always silent.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/skill-invoke-gate.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == skill-invoke-gate.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

HOOK="$REPO_DIR/cmd/hooks/skill-invoke-gate.sh"

home="$TMP/home"
repo="$TMP/repo"
mkdir -p "$home/.claude/hooks" "$repo/ctx/rules"
printf 'AGENT_CONTEXT_DIR_DEFAULT=ctx\n' > "$home/.claude/hooks/global-context.env"
seq 1 20 > "$repo/ctx/rules/go.md"
jq -n '{entries: [], tombstones: [], aspects: [], guides: [
  {name: "go", path: "rules/go.md", files: ["*.go"], source: "rules/go.md"}]}' > "$repo/ctx/context.json"

# Transcript builders. A "user" line carries the first-message text; assistant
# lines carry tool_use blocks.
user_line() { jq -nc --arg t "$1" '{type:"user", message:{content:[{type:"text", text:$t}]}}'; }
skill_use() { jq -nc --arg s "$1" '{type:"assistant", message:{content:[{type:"tool_use", id:"s", name:"Skill", input:{skill:$s}}]}}'; }

marker_txt="REQUIRE-SKILL:railroad-review
You are lane 1 of a review."
plain_txt="just do a normal thing please"

t_marker_only="$TMP/marker_only.jsonl";     user_line "$marker_txt" > "$t_marker_only"
t_no_marker="$TMP/no_marker.jsonl";         user_line "$plain_txt"  > "$t_no_marker"
t_slash="$TMP/slash.jsonl";                 user_line "/railroad-review do it" > "$t_slash"
t_invoked="$TMP/invoked.jsonl";             { user_line "$marker_txt"; skill_use "railroad-review"; } > "$t_invoked"
t_wrong_skill="$TMP/wrong_skill.jsonl";     { user_line "$marker_txt"; skill_use "fdesign"; } > "$t_wrong_skill"

run_hook() { rc=0; out=$(HOME="$home" bash "$HOOK" <<<"$1" 2>&1) || rc=$?; }
read_json() { # read_json <file> <cwd> <transcript>
  jq -n --arg f "$1" --arg cwd "$2" --arg tr "$3" \
    '{hook_event_name:"PreToolUse", tool_name:"Read", tool_input:{file_path:$f}, cwd:$cwd, transcript_path:$tr}'
}
bash_json() { # bash_json <command> <cwd> <transcript>
  jq -n --arg c "$1" --arg cwd "$2" --arg tr "$3" \
    '{hook_event_name:"PreToolUse", tool_name:"Bash", tool_input:{command:$c}, cwd:$cwd, transcript_path:$tr}'
}
skill_json() { # skill_json <skill> <cwd> <transcript>
  jq -n --arg s "$1" --arg cwd "$2" --arg tr "$3" \
    '{hook_event_name:"PreToolUse", tool_name:"Skill", tool_input:{skill:$s}, cwd:$cwd, transcript_path:$tr}'
}
decision() { jq -r '.hookSpecificOutput.permissionDecision // empty' <<<"$out"; }
reason()   { jq -r '.hookSpecificOutput.permissionDecisionReason // empty' <<<"$out"; }
silent()   { [ "$rc" -eq 0 ] && [ -z "$out" ]; }

test_no_marker_is_silent() {
  run_hook "$(bash_json "git status" "$repo" "$t_no_marker")"
  silent || fail "no-marker session not silent: $out"
  run_hook "$(read_json "$repo/ctx/rules/go.md" "$repo" "$t_no_marker")"
  silent || fail "no-marker Read not silent: $out"
}

test_pre_invocation_denied() {
  run_hook "$(bash_json "git rev-parse HEAD" "$repo" "$t_marker_only")"
  [ "$(decision)" = "deny" ] || fail "pre-invocation Bash not denied: $out"
  case "$(reason)" in *"railroad-review"*) : ;; *) fail "deny reason misses skill name: $(reason)" ;; esac
  # The Skill call itself is always allowed through.
  run_hook "$(skill_json "railroad-review" "$repo" "$t_marker_only")"
  silent || fail "Skill invocation denied: $out"
}

test_invocation_satisfies() {
  run_hook "$(bash_json "git status" "$repo" "$t_invoked")"
  silent || fail "post-invocation Bash denied: $out"
}

test_wrong_skill_does_not_satisfy() {
  run_hook "$(bash_json "git status" "$repo" "$t_wrong_skill")"
  [ "$(decision)" = "deny" ] || fail "a different skill wrongly satisfied the gate: $out"
}

test_prompt_start_slash_counts() {
  # Headless lanes start the first message with /railroad-review and carry the
  # marker for the phase-2 read denial; the slash counts as invoked.
  t="$TMP/slash_marker.jsonl"; user_line "/railroad-review
REQUIRE-SKILL:railroad-review" > "$t"
  run_hook "$(bash_json "git status" "$repo" "$t")"
  silent || fail "prompt-start slash not treated as invoked: $out"
}

test_post_invocation_context_read_denied() {
  mkdir -p "$repo/ctx/actions"
  seq 1 10 > "$repo/ctx/actions/reviewing.md"
  run_hook "$(read_json "$repo/ctx/actions/reviewing.md" "$repo" "$t_invoked")"
  [ "$(decision)" = "deny" ] || fail "context-entry Read not denied post-invocation: $out"
  case "$(reason)" in *"reviewing.md"*) : ;; *) fail "deny reason misses the path: $(reason)" ;; esac
  # exact cat of a context-entry file is denied too
  run_hook "$(bash_json "cat $repo/ctx/actions/reviewing.md" "$repo" "$t_invoked")"
  [ "$(decision)" = "deny" ] || fail "exact cat of context entry not denied: $out"
}

test_language_guide_read_allowed() {
  run_hook "$(read_json "$repo/ctx/rules/go.md" "$repo" "$t_invoked")"
  silent || fail "language guide Read denied (read-gate owns it): $out"
}

test_non_context_read_allowed() {
  run_hook "$(read_json "$repo/main.go" "$repo" "$t_invoked")"
  silent || fail "non-context Read denied post-invocation: $out"
}

test_no_ops() {
  run_hook "$(bash_json "git status" "$repo" "$TMP/absent.jsonl")"
  silent || fail "absent transcript not silent: $out"
  run_hook "$(bash_json "git status" "$repo" "")"
  silent || fail "empty transcript_path not silent: $out"
  rc=0; out=$(HOME="$home" SKILL_INVOKE_GATE_ENABLED=0 bash "$HOOK" <<<"$(bash_json "git status" "$repo" "$t_marker_only")" 2>&1) || rc=$?
  silent || fail "kill switch not silent: $out"
}

test_no_marker_is_silent
test_pre_invocation_denied
test_invocation_satisfies
test_wrong_skill_does_not_satisfy
test_prompt_start_slash_counts
test_post_invocation_context_read_denied
test_language_guide_read_allowed
test_non_context_read_allowed
test_no_ops

echo "PASS: test_skill_invoke_gate.sh"
