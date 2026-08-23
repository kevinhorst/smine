#!/usr/bin/env bash
# Regression coverage for read-gate.sh: language guides and required skill
# files are enforced by Read-range coverage; every no-op path stays silent.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/read-gate.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == read-gate.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

HOOK="$REPO_DIR/cmd/hooks/read-gate.sh"

home="$TMP/home"
repo="$TMP/repo"
sid="sess-1"
mkdir -p "$home/.claude/hooks" "$repo/ctx/rules" "$TMP/norepo"
printf 'AGENT_CONTEXT_DIR_DEFAULT=ctx\n' > "$home/.claude/hooks/global-context.env"
seq 1 300 | sed 's/^/go line /' > "$repo/ctx/rules/go.md"        # 300 lines: one default Read covers it
seq 1 2500 | sed 's/^/py line /' > "$repo/ctx/rules/python.md"   # 2500 lines: needs two Reads
# The index names the guides and their globs — the hook knows no language itself.
# sql declares a guide whose file is missing on disk (silent path).
jq -n '{entries: [], tombstones: [], aspects: [], guides: [
  {name: "go",     path: "rules/go.md",     files: ["*.go"],           source: "rules/go.md"},
  {name: "python", path: "rules/python.md", files: ["*.py", "*.pyi"],  source: "rules/python.md"},
  {name: "sql",    path: "rules/sql.md",    files: ["*.sql"],          source: "rules/sql.md"}]}' > "$repo/ctx/context.json"

fresh="$TMP/fresh.jsonl"; : > "$fresh"

read_row() { # read_row <file> [offset] [limit]
  jq -nc --arg f "$1" --arg o "${2:-}" --arg l "${3:-}" \
    '{type:"assistant", message:{content:[{type:"tool_use", id:"t", name:"Read",
      input: ({file_path:$f} + (if $o != "" then {offset: ($o|tonumber)} else {} end) + (if $l != "" then {limit: ($l|tonumber)} else {} end))}]}}'
}
go_read="$TMP/go_read.jsonl";  read_row "$repo/ctx/rules/go.md" > "$go_read"
py_part="$TMP/py_part.jsonl";  read_row "$repo/ctx/rules/python.md" > "$py_part"
py_full="$TMP/py_full.jsonl";  { read_row "$repo/ctx/rules/python.md"; read_row "$repo/ctx/rules/python.md" 2001 500; } > "$py_full"
prose="$TMP/prose.jsonl"
jq -nc --arg t "read $repo/ctx/rules/go.md first" '{type:"user", message:{content:[{type:"tool_result", content:$t}]}}' > "$prose"

run_hook() { rc=0; out=$(HOME="$home" bash "$HOOK" <<<"$1" 2>&1) || rc=$?; }
pre_json() { # pre_json <tool> <file> <cwd> <transcript> [agent_id]
  jq -n --arg t "$1" --arg f "$2" --arg cwd "$3" --arg tr "$4" --arg sid "$sid" --arg aid "${5:-}" \
    '{hook_event_name:"PreToolUse", tool_name:$t, tool_input:{file_path:$f}, cwd:$cwd, transcript_path:$tr, session_id:$sid}
     + (if $aid != "" then {agent_id:$aid} else {} end)'
}
bash_json() { # bash_json <command> <cwd> <transcript> [agent_id]
  jq -n --arg c "$1" --arg cwd "$2" --arg tr "$3" --arg sid "$sid" --arg aid "${4:-}" \
    '{hook_event_name:"PreToolUse", tool_name:"Bash", tool_input:{command:$c}, cwd:$cwd, transcript_path:$tr, session_id:$sid}
     + (if $aid != "" then {agent_id:$aid} else {} end)'
}
decision() { jq -r '.hookSpecificOutput.permissionDecision // empty' <<<"$out"; }
reason()   { jq -r '.hookSpecificOutput.permissionDecisionReason // empty' <<<"$out"; }
silent()   { [ "$rc" -eq 0 ] && [ -z "$out" ]; }

# The sidecar persists coverage across hook invocations by design; tests that
# assume a pristine session clear it first.
state_dir="$home/.claude/read-gate-state"
clear_state() { rm -rf "$state_dir"; }
seed_state() { # seed_state <session__agent> <file> <start> <end>
  mkdir -p "$state_dir"
  jq -nc --arg f "$2" --argjson s "$3" --argjson e "$4" '{file:$f,s:$s,e:$e}' >> "$state_dir/$1.jsonl"
}

test_first_touch_denied_with_range() {
  clear_state
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail "first Read not denied: $out"
  case "$(reason)" in *"Read $repo/ctx/rules/go.md offset 1 limit 300"*) : ;; *) fail "range missing: $out" ;; esac
  run_hook "$(pre_json Edit "$repo/a.go" "$repo" "$fresh")";  [ "$(decision)" = "deny" ] || fail "Edit not denied"
  run_hook "$(pre_json Write "$repo/b.go" "$repo" "$fresh")"; [ "$(decision)" = "deny" ] || fail "Write not denied"
  [ ! -d "$state_dir" ] || fail "denied calls wrote sidecar state"
}

test_read_of_guide_itself_allowed() {
  clear_state
  run_hook "$(pre_json Read "$repo/ctx/rules/go.md" "$repo" "$fresh")"
  silent || fail "Read of the guide was denied: $out"
  grep -q "\"file\":\"$repo/ctx/rules/go.md\"" "$state_dir/${sid}__main.jsonl" || fail "guide read not recorded in sidecar"
  # The recorded coverage now stands in for the transcript: a fresh transcript
  # no longer denies the governed read.
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  silent || fail "sidecar coverage not honored: $out"
}

test_full_read_passes() {
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$go_read")"
  silent || fail "covered guide still denied: rc=$rc out=$out"
}

test_second_glob_of_a_guide() {
  clear_state
  run_hook "$(pre_json Read "$repo/t.pyi" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail ".pyi (second glob) not denied: $out"
  case "$(reason)" in *"/ctx/rules/python.md"*) : ;; *) fail "wrong guide for .pyi: $(reason)" ;; esac
}

test_partial_read_lists_remaining() {
  clear_state
  run_hook "$(pre_json Read "$repo/t.py" "$repo" "$py_part")"
  [ "$(decision)" = "deny" ] || fail "partial read not denied: $out"
  case "$(reason)" in *"offset 2001 limit 500"*) : ;; *) fail "remaining range wrong: $(reason)" ;; esac
  case "$(reason)" in *"offset 1 limit"*) fail "already-read range listed: $(reason)" ;; esac
  run_hook "$(pre_json Read "$repo/t.py" "$repo" "$py_full")"
  silent || fail "two-chunk coverage still denied: $out"
}

test_prose_mention_is_not_a_read() {
  clear_state
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$prose")"
  [ "$(decision)" = "deny" ] || fail "prose mention counted as read: $out"
}

test_required_skill_file() {
  clear_state
  mkdir -p "$home/.claude/context-required/$sid"
  seq 1 50 > "$home/.claude/context-required/$sid/fdesign.md"
  json=$(jq -n --arg cwd "$repo" --arg tr "$fresh" --arg sid "$sid" \
    '{hook_event_name:"PreToolUse", tool_name:"Bash", tool_input:{command:"git status"}, cwd:$cwd, transcript_path:$tr, session_id:$sid}')
  run_hook "$json"
  [ "$(decision)" = "deny" ] || fail "Bash before required skill file not denied: $out"
  case "$(reason)" in *"/$sid/fdesign.md offset 1 limit 50"*) : ;; *) fail "skill file range missing: $(reason)" ;; esac
  done_t="$TMP/skill_read.jsonl"; read_row "$home/.claude/context-required/$sid/fdesign.md" > "$done_t"
  run_hook "$(jq -n --arg cwd "$repo" --arg tr "$done_t" --arg sid "$sid" '{hook_event_name:"PreToolUse", tool_name:"Bash", tool_input:{command:"git status"}, cwd:$cwd, transcript_path:$tr, session_id:$sid}')"
  silent || fail "read skill file still denied: $out"
  # Subagent: skill file not required, guide still is.
  run_hook "$(pre_json Bash "" "$repo" "$fresh" agent-9)"
  silent || fail "subagent required skill file: $out"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh" agent-9)"
  [ "$(decision)" = "deny" ] || fail "subagent language guide not required: $out"
  rm -rf "$home/.claude/context-required"
}

test_no_ops() {
  clear_state
  run_hook "$(pre_json Read "$repo/README.md" "$repo" "$fresh")";        silent || fail "unmatched file not silent: $out"
  run_hook "$(pre_json Read "$repo/q.sql" "$repo" "$fresh")";            silent || fail "declared guide missing on disk not silent: $out"
  run_hook "$(pre_json Read "$TMP/norepo/a.go" "$TMP/norepo" "$fresh")"; silent || fail "no index in ctx dir not silent: $out"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$TMP/absent.jsonl")";  silent || fail "absent transcript not silent: $out"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "")";                   silent || fail "empty transcript_path not silent: $out"
  rc=0; out=$(HOME="$home" READ_GATE_ENABLED=0 bash "$HOOK" <<<"$(pre_json Read "$repo/a.go" "$repo" "$fresh")" 2>&1) || rc=$?
  silent || fail "kill switch not silent: $out"
}

test_subagent_sidecar_covers_stale_transcript() {
  # The subagent repro: the transcript never shows the agent's own Reads;
  # its sidecar coverage must stand in.
  clear_state
  run_hook "$(pre_json Read "$repo/ctx/rules/go.md" "$repo" "$fresh" agent-9)"
  silent || fail "subagent guide read denied: $out"
  [ -f "$state_dir/${sid}__agent-9.jsonl" ] || fail "subagent sidecar not written"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh" agent-9)"
  silent || fail "subagent still denied after recorded guide read: $out"
  # Coverage is per-agent: the main conversation earned nothing from it.
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail "agent sidecar leaked into main conversation: $out"
}

test_union_transcript_plus_sidecar() {
  clear_state
  seed_state "${sid}__main" "$repo/ctx/rules/python.md" 2001 2500
  run_hook "$(pre_json Read "$repo/t.py" "$repo" "$py_part")"
  silent || fail "transcript+sidecar union not honored: $out"
  clear_state
  seed_state "${sid}__main" "$repo/ctx/rules/python.md" 1 1000
  run_hook "$(pre_json Read "$repo/t.py" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail "partial sidecar coverage allowed: $out"
  case "$(reason)" in *"offset 1001 limit 1500"*) : ;; *) fail "remaining range wrong: $(reason)" ;; esac
}

test_bash_exact_cat_counts() {
  clear_state
  run_hook "$(bash_json "cat $repo/ctx/rules/go.md" "$repo" "$fresh")"
  silent || fail "exact cat of guide denied: $out"
  grep -q "\"file\":\"$repo/ctx/rules/go.md\"" "$state_dir/${sid}__main.jsonl" || fail "exact cat not recorded"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  silent || fail "cat coverage not honored on later Read: $out"
}

test_bash_cat_variants_do_not_count() {
  # A cat variant touches no governed file, so the call itself is not gated —
  # but unlike the exact form it earns no coverage.
  clear_state
  for variant in "cat -n $repo/ctx/rules/go.md" "cat $repo/ctx/rules/go.md | head" "sed -n 1p $repo/ctx/rules/go.md"; do
    run_hook "$(bash_json "$variant" "$repo" "$fresh")"
    silent || fail "cat variant unexpectedly gated: $variant: $out"
    [ ! -d "$state_dir" ] || fail "cat variant recorded coverage: $variant"
  done
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail "coverage granted without exact cat: $out"
}

test_bash_touching_governed_file_requires_guide() {
  clear_state
  run_hook "$(bash_json "grep -n foo $repo/a.go" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail "Bash touching governed file not denied: $out"
  case "$(reason)" in *"/ctx/rules/go.md"*) : ;; *) fail "guide not named for Bash: $(reason)" ;; esac
  case "$(reason)" in *"use Read"*) : ;; *) fail "Bash addendum missing: $(reason)" ;; esac
  run_hook "$(bash_json "go test ./internal/foo" "$repo" "$fresh")"
  silent || fail "governed-free Bash denied: $out"
  # Subagent: guide applies to Bash too.
  run_hook "$(bash_json "grep -n foo $repo/a.go" "$repo" "$fresh" agent-9)"
  [ "$(decision)" = "deny" ] || fail "subagent Bash bypassed guide: $out"
}

test_sidecar_robustness() {
  # Duplicate and malformed lines are tolerated; malformed-only fails open
  # toward deny (no coverage), never crashes.
  clear_state
  seed_state "${sid}__main" "$repo/ctx/rules/go.md" 1 300
  seed_state "${sid}__main" "$repo/ctx/rules/go.md" 1 300
  echo "not json" >> "$state_dir/${sid}__main.jsonl"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  silent || fail "duplicate/malformed sidecar lines broke coverage: $out"
  clear_state
  mkdir -p "$state_dir"
  echo "not json" > "$state_dir/${sid}__main.jsonl"
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$fresh")"
  [ "$(decision)" = "deny" ] || fail "malformed-only sidecar granted coverage: $out"
}

test_sidecar_prune() {
  clear_state
  mkdir -p "$state_dir"
  : > "$state_dir/old__main.jsonl"
  touch -t "$(date -v-8d +%Y%m%d%H%M 2>/dev/null || date -d '8 days ago' +%Y%m%d%H%M)" "$state_dir/old__main.jsonl"
  run_hook "$(pre_json Read "$repo/ctx/rules/go.md" "$repo" "$fresh")"
  silent || fail "guide read denied during prune test: $out"
  [ ! -f "$state_dir/old__main.jsonl" ] || fail "stale state file not pruned"
}

test_sidecar_without_transcript_keeps_gate_active() {
  clear_state
  seed_state "${sid}__main" "$repo/ctx/rules/python.md" 1 1000
  run_hook "$(pre_json Read "$repo/t.py" "$repo" "$TMP/absent.jsonl")"
  [ "$(decision)" = "deny" ] || fail "absent transcript with sidecar not gated: $out"
  clear_state
  seed_state "${sid}__main" "$repo/ctx/rules/go.md" 1 300
  run_hook "$(pre_json Read "$repo/a.go" "$repo" "$TMP/absent.jsonl")"
  silent || fail "sidecar-only coverage denied: $out"
}

test_first_touch_denied_with_range
test_read_of_guide_itself_allowed
test_full_read_passes
test_second_glob_of_a_guide
test_partial_read_lists_remaining
test_prose_mention_is_not_a_read
test_required_skill_file
test_no_ops
test_subagent_sidecar_covers_stale_transcript
test_union_transcript_plus_sidecar
test_bash_exact_cat_counts
test_bash_cat_variants_do_not_count
test_bash_touching_governed_file_requires_guide
test_sidecar_robustness
test_sidecar_prune
test_sidecar_without_transcript_keeps_gate_active

echo "PASS: test_read_gate.sh"
