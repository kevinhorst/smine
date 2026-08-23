#!/usr/bin/env bash
# Regression coverage for cmd/context/context_record.sh: hook-injected
# skill/global context, ACDSL projection blocks, plan rules, pack reads and
# subagent IDs are extracted from a fixture transcript; usage errors exit 2.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/ctx-record.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == ctx-record.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT="$REPO_DIR/cmd/context/context_record.sh"
fixture="$TMP/session.jsonl"

cat > "$fixture" <<'EOF'
{"type":"user","cwd":"/repo/x","sessionId":"sess-1","message":{"role":"user","content":"/concept do it"}}
{"type":"attachment","attachment":{"type":"hook_success","hookName":"UserPromptSubmit","hookEvent":"UserPromptSubmit","content":"<persisted-output>stub</persisted-output>","stdout":"===== SKILL CONTEXT /concept (injected by hook) =====\n- **ACTION-CONCEPT-HOT-001** [gate] — x\n- **FACT-REPO-ARCH-001** [fact] — y\n"}}
{"type":"attachment","attachment":{"type":"hook_success","hookName":"SessionStart:startup","hookEvent":"SessionStart","content":"","stdout":"===== GLOBAL CONTEXT (injected by hook) =====\n\n===== docs/global/company.md =====\nfacts\n"}}
{"type":"attachment","attachment":{"type":"hook_success","hookName":"UserPromptSubmit","hookEvent":"UserPromptSubmit","content":"","stdout":"{\"hookSpecificOutput\":{\"additionalContext\":\"===== SKILL CONTEXT /fdesign (injected by hook) =====\\n- **ACTION-PLAN-FMT-001** [review] — z\\n\"}}"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Read","input":{"file_path":"/repo/x/internal/a.go"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","content":"     1\t// [ACDSL-PROJECTION] 2 rule(s) govern this file — working-copy view, stripped before commit\n     2\t// - [ACDSL-GOLANG-FMT-001] gofmt\n     3\t// - [ACDSL-GOLANG-FUNC-001] sigs\n     4\tpackage a\n"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu2","name":"Read","input":{"file_path":"/repo/x/docs/rules/go.md"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu2","content":"rules"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu3","name":"Bash","input":{"command":"go run ./cmd/acdsl project -plan internal/b.go"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu3","content":"- [ACDSL-GOLANG-ENUM-001] enums\n"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu4","name":"Agent","input":{"prompt":"Read docs/actions/implementing.md first; honor ACTION-IMPL-001 and RULE-GOLANG-TEST-002."}}]}}
EOF

out=$("$SCRIPT" "$fixture" docs) || fail "script exited non-zero"
echo "$out" | jq -e '.session_id == "sess-1"' >/dev/null || fail "session id"
echo "$out" | jq -e '.injected.skill[0].skill == "concept" and (.injected.skill[0].ids | index("ACTION-CONCEPT-HOT-001")) and (.injected.skill[0].ids | index("FACT-REPO-ARCH-001"))' >/dev/null || fail "skill context from hook stdout"
echo "$out" | jq -e '.injected.skill[1].skill == "fdesign" and .injected.skill[1].ids == ["ACTION-PLAN-FMT-001"]' >/dev/null || fail "skill context from JSON-form hook stdout (hookSpecificOutput.additionalContext)"
echo "$out" | jq -e '.injected.global == ["docs/global/company.md"]' >/dev/null || fail "global context files"
echo "$out" | jq -e '.injected.lang == [{"lang":"go","guide":"/repo/x/docs/rules/go.md","ids":[],"coverage":"unknown"}]' >/dev/null || fail "lang row from a guide Read (missing on disk → unknown)"
echo "$out" | jq -e '.acdsl_rules == [{"file":"/repo/x/internal/a.go","ids":["ACDSL-GOLANG-FMT-001","ACDSL-GOLANG-FUNC-001"]}]' >/dev/null || fail "acdsl projection"
echo "$out" | jq -e '.plan_rules == ["ACDSL-GOLANG-ENUM-001"]' >/dev/null || fail "plan rules"
echo "$out" | jq -e '.pack_reads == ["/repo/x/docs/rules/go.md"]' >/dev/null || fail "pack reads under ctx dir only"
echo "$out" | jq -e '.subagent_context[0].ids == ["ACTION-IMPL-001","RULE-GOLANG-TEST-002"] and .subagent_context[0].paths == ["docs/actions/implementing.md"]' >/dev/null || fail "subagent ids and paths"

# Language row against guides that exist on disk: coverage complete / partial,
# ids unioned across chunked Reads, non-guide Reads not counted.
mkdir -p "$TMP/ctx/rules"
echo '**RULE-GOLANG-NAME-001** line 1' > "$TMP/ctx/rules/go.md"
seq 2 300 | sed 's/^/go line /' >> "$TMP/ctx/rules/go.md"
seq 1 2500 | sed 's/^/py line /' > "$TMP/ctx/rules/python.md"
lang_fixture="$TMP/lang.jsonl"
jq -nc --arg g "$TMP/ctx/rules/go.md" '{type:"assistant", message:{content:[{type:"tool_use", id:"g1", name:"Read", input:{file_path:$g}}]}}' > "$lang_fixture"
jq -nc '{type:"user", message:{content:[{type:"tool_result", tool_use_id:"g1", content:"     1\t**RULE-GOLANG-NAME-001** `[review]` — a\n"}]}}' >> "$lang_fixture"
jq -nc --arg p "$TMP/ctx/rules/python.md" '{type:"assistant", message:{content:[{type:"tool_use", id:"p1", name:"Read", input:{file_path:$p}}]}}' >> "$lang_fixture"
jq -nc '{type:"user", message:{content:[{type:"tool_result", tool_use_id:"p1", content:[{type:"text", text:"  1\t**RULE-PYTHON-NAME-001** x\n"}]}]}}' >> "$lang_fixture"
jq -nc --arg p "$TMP/ctx/rules/python.md" '{type:"assistant", message:{content:[{type:"tool_use", id:"p2", name:"Read", input:{file_path:$p, offset:2001, limit:100}}]}}' >> "$lang_fixture"
jq -nc '{type:"user", message:{content:[{type:"tool_result", tool_use_id:"p2", content:"  2001\t**RULE-PYTHON-TEST-001** y\n"}]}}' >> "$lang_fixture"
jq -nc '{type:"assistant", message:{content:[{type:"tool_use", id:"o1", name:"Read", input:{file_path:"/repo/x/internal/foo.go"}}]}}' >> "$lang_fixture"
jq -nc '{type:"user", message:{content:[{type:"tool_result", tool_use_id:"o1", content:"package foo // **RULE-GOLANG-NAME-001** in code"}]}}' >> "$lang_fixture"
lang_out=$("$SCRIPT" "$lang_fixture" ctx) || fail "lang fixture exited non-zero"
echo "$lang_out" | jq -e '.injected.lang | length == 2' >/dev/null || fail "two guides expected, non-guide Read excluded: $lang_out"
echo "$lang_out" | jq -e '.injected.lang[] | select(.lang == "go") | .coverage == "complete" and .ids == ["RULE-GOLANG-NAME-001"]' >/dev/null || fail "go: full default Read of a 300-line guide is complete: $lang_out"
echo "$lang_out" | jq -e '.injected.lang[] | select(.lang == "python") | .coverage == "partial" and .ids == ["RULE-PYTHON-NAME-001","RULE-PYTHON-TEST-001"]' >/dev/null || fail "python: 1-2000 + 2001-2100 of 2500 is partial, ids unioned: $lang_out"

# usage error: missing file → exit 2, nothing on stdout
rc=0
"$SCRIPT" "$TMP/missing.jsonl" >"$TMP/out" 2>/dev/null || rc=$?
[ "$rc" -eq 2 ] || fail "missing transcript must exit 2, got $rc"
[ ! -s "$TMP/out" ] || fail "usage error printed output"

echo "test_context_record: ok"
