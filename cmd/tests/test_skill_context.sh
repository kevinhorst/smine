#!/usr/bin/env bash
# Regression coverage for skill-context.sh: declared context entries resolve
# and inject on skill invocation; every no-op path stays silent with exit 0.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/skill-ctx.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == skill-ctx.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

HOOK="$REPO_DIR/cmd/hooks/skill-context.sh"

home="$TMP/home"
repo="$TMP/repo"
mkdir -p "$home/.claude/hooks" "$home/.claude/skills/testskill" \
  "$home/.claude/skills/bare" "$repo/ctx" "$TMP/norepo"

printf 'AGENT_CONTEXT_DIR_DEFAULT=ctx\n' > "$home/.claude/hooks/global-context.env"

cat > "$home/.claude/skills/testskill/SKILL.md" <<'EOF'
---
name: testskill
version: 1.0
acdsl-context: ACTION-TEST-HOT-001, RULE-TEST-*
---
# Test skill
EOF

cat > "$home/.claude/skills/bare/SKILL.md" <<'EOF'
---
name: bare
version: 1.0
---
# Bare skill
EOF

cat > "$repo/ctx/context.json" <<'EOF'
{
  "entries": [
    {
      "id": "ACTION-TEST-HOT-001",
      "kind": "action",
      "enforcement": "gate",
      "content": {
        "statement": "First statement.",
        "why": "because.",
        "applies": "always.",
        "bullets": ["An untagged bullet."],
        "examples": [{"lang": "go", "code": "// GOOD\nvar hookGroup int"}]
      }
    },
    {
      "id": "RULE-TEST-NAME-001",
      "kind": "rule",
      "enforcement": "review",
      "content": {"statement": "Second statement."}
    },
    {
      "id": "RULE-OTHER-001",
      "kind": "rule",
      "enforcement": "review",
      "content": {"statement": "Unselected statement."}
    }
  ]
}
EOF

# A big index + skill for the spill path: 40 entries, ~400 chars of bullets each.
mkdir -p "$repo-big/ctx" "$home/.claude/skills/bigskill"
cat > "$home/.claude/skills/bigskill/SKILL.md" <<'EOF'
---
name: bigskill
version: 1.0
acdsl-context: RULE-BIG-*
---
# Big skill
EOF
jq -n '{entries: [range(1; 41) | {id: ("RULE-BIG-" + (. | tostring | ("000" + .)[-3:])), kind: "rule", enforcement: "review",
  content: {statement: ("Big statement " + (. | tostring) + "."), bullets: [range(4) | ("bullet " + ("x" * 90))]}}]}' > "$repo-big/ctx/context.json"

# run_hook <json> -> sets rc and out
run_hook() {
  rc=0
  out=$(HOME="$home" bash "$HOOK" <<<"$1" 2>&1) || rc=$?
}

ups_json() {
  jq -n --arg p "$1" --arg cwd "$2" \
    '{hook_event_name: "UserPromptSubmit", prompt: $p, cwd: $cwd}'
}

test_slash_invocation_injects() {
  run_hook "$(ups_json "/testskill some args" "$repo")"
  [ "$rc" -eq 0 ] || fail "hook errored: $out"
  case "$out" in *ACTION-TEST-HOT-001*) : ;; *) fail "exact ID not injected: $out" ;; esac
  case "$out" in *RULE-TEST-NAME-001*) : ;; *) fail "glob match not injected: $out" ;; esac
  case "$out" in *RULE-OTHER-001*) fail "unselected entry injected: $out" ;; esac
  case "$out" in *"First statement."*) : ;; *) fail "statement text missing: $out" ;; esac
  case "$out" in *"Applies: always."*) : ;; *) fail "applies line missing: $out" ;; esac
}

test_plain_prompt_silent() {
  run_hook "$(ups_json "just a question" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "non-slash prompt not silent: rc=$rc out=$out"
}

test_undeclared_skill_silent() {
  run_hook "$(ups_json "/bare go" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "undeclared skill not silent: rc=$rc out=$out"
}

test_unknown_skill_silent() {
  run_hook "$(ups_json "/nosuchskill" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "unknown skill not silent: rc=$rc out=$out"
}

test_missing_index_silent() {
  run_hook "$(ups_json "/testskill" "$TMP/norepo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "missing index not silent: rc=$rc out=$out"
}

test_posttooluse_wraps_json() {
  json=$(jq -n --arg cwd "$repo" \
    '{hook_event_name: "PostToolUse", tool_name: "Skill", tool_input: {skill: "testskill"}, cwd: $cwd}')
  run_hook "$json"
  [ "$rc" -eq 0 ] || fail "posttooluse errored: $out"
  ctx=$(jq -r '.hookSpecificOutput.additionalContext // empty' <<<"$out") || \
    fail "posttooluse output not JSON: $out"
  case "$ctx" in *ACTION-TEST-HOT-001*) : ;; *) fail "additionalContext missing entry: $out" ;; esac
}

test_other_tool_silent() {
  json=$(jq -n --arg cwd "$repo" \
    '{hook_event_name: "PostToolUse", tool_name: "Read", tool_input: {file_path: "/x"}, cwd: $cwd}')
  run_hook "$json"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "non-Skill tool not silent: rc=$rc out=$out"
}

test_kill_switch() {
  rc=0
  out=$(HOME="$home" SKILL_CONTEXT_ENABLED=0 bash "$HOOK" \
    <<<"$(ups_json "/testskill" "$repo")" 2>&1) || rc=$?
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "kill switch not silent: rc=$rc out=$out"
}

test_no_context_flag_silent() {
  run_hook "$(ups_json "/testskill do the thing --no-context" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "--no-context not silent: rc=$rc out=$out"
}

test_no_context_emdash_silent() {
  run_hook "$(ups_json "/testskill —no-context do the thing" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "em-dash flag not silent: rc=$rc out=$out"
}

test_no_context_single_dash_silent() {
  run_hook "$(ups_json "/testskill -no-context do the thing" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "single-dash flag not silent: rc=$rc out=$out"
}

test_no_context_endash_silent() {
  run_hook "$(ups_json "/testskill –no-context do the thing" "$repo")"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "en-dash flag not silent: rc=$rc out=$out"
}

test_no_context_posttooluse_silent() {
  json=$(jq -n --arg cwd "$repo" \
    '{hook_event_name: "PostToolUse", tool_name: "Skill", tool_input: {skill: "testskill", args: "do the thing --no-context"}, cwd: $cwd}')
  run_hook "$json"
  { [ "$rc" -eq 0 ] && [ -z "$out" ]; } || fail "posttooluse flag not silent: rc=$rc out=$out"
}

test_no_context_mention_still_injects() {
  run_hook "$(ups_json "/testskill add a --no-context, flag to x--no-contexty" "$repo")"
  [ "$rc" -eq 0 ] || fail "mention-case errored: $out"
  case "$out" in *ACTION-TEST-HOT-001*) : ;; *) fail "prose mention suppressed injection: $out" ;; esac
}

test_typed_content_rendered_inline() {
  run_hook "$(ups_json "/testskill" "$repo")"
  [ "$rc" -eq 0 ] || fail "hook errored: $out"
  case "$out" in *'- **ACTION-TEST-HOT-001**'*) : ;; *) fail "ID list line missing: $out" ;; esac
  case "$out" in *'**ACTION-TEST-HOT-001** `[gate]` — First statement.'*) : ;; *) fail "headline missing: $out" ;; esac
  case "$out" in *'* Why: because.'*'* Applies: always.'*'* An untagged bullet.'*'```go'*'var hookGroup int'*'```'*) : ;;
    *) fail "typed fields not rendered in order: $out" ;; esac
  case "$out" in *'**RULE-TEST-NAME-001** `[review]` — Second statement.'*) : ;; *) fail "statement-only entry missing: $out" ;; esac
}

test_spill_over_cap() {
  json=$(jq -n --arg cwd "$repo-big" '{hook_event_name: "UserPromptSubmit", prompt: "/bigskill", cwd: $cwd, session_id: "s1"}')
  run_hook "$json"
  [ "$rc" -eq 0 ] || fail "hook errored: $out"
  [ "${#out}" -lt 10000 ] || fail "spilled output not under the cap: ${#out} chars"
  target="$home/.claude/context-required/s1/bigskill.md"
  case "$out" in *"Written to $target"*) : ;; *) fail "pointer missing: $out" ;; esac
  [ -f "$target" ] || fail "spill file not written"
  [ "$(grep -c '^\*\*RULE-BIG-' "$target")" -eq 40 ] || fail "spill file lacks the 40 headlines"
  [ "$(grep -c '^- \*\*RULE-BIG-' <<<"$out")" -eq 40 ] || fail "ID list incomplete in output"
  case "$out" in *"Big statement 1."*) fail "bodies leaked into the spilled output" ;; esac
}

test_spill_needs_session_id() {
  rm -rf "$home/.claude/context-required"
  json=$(jq -n --arg cwd "$repo-big" '{hook_event_name: "UserPromptSubmit", prompt: "/bigskill", cwd: $cwd}')
  run_hook "$json"
  [ -d "$home/.claude/context-required" ] && fail "file written without session_id"
  case "$out" in *"Big statement 1."*) : ;; *) fail "no session_id should render inline: ${out:0:200}" ;; esac
}

test_prune_old_session_dirs() {
  mkdir -p "$home/.claude/context-required/old" "$home/.claude/context-required/fresh"
  touch -t 202001010000 "$home/.claude/context-required/old"
  json=$(jq -n --arg cwd "$repo-big" '{hook_event_name: "UserPromptSubmit", prompt: "/bigskill", cwd: $cwd, session_id: "s2"}')
  run_hook "$json"
  [ ! -d "$home/.claude/context-required/old" ] || fail "old session dir not pruned"
  [ -d "$home/.claude/context-required/fresh" ] || fail "fresh session dir pruned"
  rm -rf "$home/.claude/context-required"
}

test_prune_without_spill() {
  mkdir -p "$home/.claude/context-required/old" "$home/.claude/context-required/fresh"
  touch -t 202001010000 "$home/.claude/context-required/old"
  run_hook "$(ups_json "/testskill" "$repo")"
  [ "$rc" -eq 0 ] || fail "hook errored: $out"
  [ ! -d "$home/.claude/context-required/old" ] || fail "old session dir not pruned on a no-spill invocation"
  [ -d "$home/.claude/context-required/fresh" ] || fail "fresh session dir pruned"
  [ -z "$(find "$home/.claude/context-required" -name '*.md' 2>/dev/null)" ] || fail "no-spill invocation wrote a context file"
  rm -rf "$home/.claude/context-required"
}

test_slash_invocation_injects
test_typed_content_rendered_inline
test_spill_over_cap
test_spill_needs_session_id
test_prune_old_session_dirs
test_prune_without_spill
test_plain_prompt_silent
test_undeclared_skill_silent
test_unknown_skill_silent
test_missing_index_silent
test_posttooluse_wraps_json
test_other_tool_silent
test_kill_switch
test_no_context_flag_silent
test_no_context_emdash_silent
test_no_context_single_dash_silent
test_no_context_endash_silent
test_no_context_posttooluse_silent
test_no_context_mention_still_injects

echo "PASS: test_skill_context.sh"
