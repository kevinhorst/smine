#!/usr/bin/env bash
# Nightly skill A/B eval routine (operations + claude -p mechanics:
# routines/README.md). Two stages on one worktree/branch:
# cells — every matrix cell is a real `claude -p` session of the evaluated
# skill (default: concept) in its own detached worktree, arms = context on/off
# × skill variants × replicas (routines/_lib/matrix.sh) — then eval —
# /skillroutine-eval on the generated manifest. Deltas per axis land in
# results.jsonl and <resultsDir>/deltas.json; one publish commits everything.
# Requires flock + coreutils timeout (brew install flock coreutils; macOS
# only — Windows runs under routinewrap.exe, which holds the locks).

set -uo pipefail

[ -d /opt/homebrew/bin ] && export PATH="/opt/homebrew/bin:$PATH"
export DISABLE_AUTOUPDATER=1
export DISABLE_TELEMETRY=1

routine_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$routine_dir/../.." && pwd)"
results="$routine_dir/results.jsonl"

# stdin: claude JSON envelope (may be empty); $1: exit status; $2: stage
# (cells|eval); $3: optional extra JSON object merged into the line.
append_result() {
  jq -cn \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson exit "$1" \
    --arg stage "$2" \
    --argjson extra "${3:-{\}}" \
    'input? // {} | {timestamp: $ts, stage: $stage, exit_status: $exit, session_id: .session_id, num_turns: .num_turns, total_cost_usd: .total_cost_usd, subtype: .subtype, is_error: .is_error, result: ((.result // "") | tostring | .[0:300])} + $extra' \
    >> "$results"
}

if [[ -n "${ROUTINE_TOKEN:-}" ]]; then
  token_file="$HOME/.config/claude-routine/tokens/$ROUTINE_TOKEN"
else
  token_file="$HOME/.config/claude-routine/token"
fi
if [[ ! -s "$token_file" ]]; then
  echo "token file missing or empty: $token_file" >&2
  append_result 78 cells </dev/null
  exit 78
fi
export CLAUDE_CODE_OAUTH_TOKEN="$(cat "$token_file")"

source "$repo_root/routines/_lib/platform.sh"
routine_self_lock || { echo "already running"; exit 0; }

source "$repo_root/routines/_lib/cadence.sh"
routine_cadence_gate || exit 0

source "$repo_root/routines/_lib/worktree.sh"
source "$repo_root/routines/_lib/skill.sh"
routine_group_lock || {
  echo "group lock timeout: $ROUTINE_GROUP" >&2
  append_result 75 cells </dev/null
  exit 75
}
wt="$(routine_worktree_create)"; create_rc=$?
if [[ "$create_rc" -eq 3 ]]; then
  echo "open-instance limit reached for $ROUTINE_GROUP (ROUTINE_MAX_OPEN_BRANCHES=$ROUTINE_MAX_OPEN_BRANCHES); merge a $ROUTINE_BRANCH_PREFIX-* branch before the next run" >&2
  exit 0
elif [[ "$create_rc" -ne 0 || -z "$wt" ]]; then
  echo "worktree create failed" >&2
  append_result 70 cells </dev/null
  exit 70
fi
cd "$wt"

source "$repo_root/routines/_lib/matrix.sh"

# ---- Stage 1: cells ----
day="$(date -u +%F)"
results_dir="evals/$MATRIX_SKILL-$day"
n=2
while [[ -e "$wt/$results_dir" ]]; do
  results_dir="evals/$MATRIX_SKILL-$day-$n"
  n=$((n + 1))
done
mkdir -p "$wt/$results_dir"
base="$(git -C "$wt" rev-parse HEAD)"
plan="$wt/$results_dir/cells.tsv"

exit_status=0
cell_count="$(routine_matrix_plan "$plan")" || {
  echo "matrix plan refused" >&2
  append_result 2 cells </dev/null
  cd "$repo_root"
  routine_worktree_publish 2 || echo "publish failed; worktree kept: $wt" >&2
  exit 2
}
echo "matrix: $cell_count cell(s), base $base, results $results_dir"

if ! routine_matrix_render_variants "$results_dir"; then
  append_result 70 cells </dev/null
  routine_matrix_cleanup
  cd "$repo_root"
  routine_worktree_publish 70 || echo "publish failed; worktree kept: $wt" >&2
  exit 70
fi

routine_matrix_run_cells "$results_dir" "$base" "$plan"
cells_summary="$(jq -sc --arg rd "$results_dir" '{cells_ok: (map(select(.ok == true)) | length), cells_failed: (map(select(.ok != true)) | length), cells_cost_usd: (map(.cost_usd // 0) | add), results_dir: $rd}' "$wt/$results_dir/cells.jsonl" 2>/dev/null)" || cells_summary='{}'

if ! routine_matrix_write_manifest "$results_dir"; then
  echo "no surviving cells or manifest invalid" >&2
  append_result 70 cells "$cells_summary" </dev/null
  routine_matrix_cleanup
  cd "$repo_root"
  routine_worktree_publish 70 || echo "publish failed; worktree kept: $wt" >&2
  exit 70
fi
append_result 0 cells "$cells_summary" </dev/null

# ---- Stage 2: eval ----
eval_prompt="/skillroutine-eval $results_dir/manifest.json — unattended run: where the skill says STOP for user review, write eval.json and eval.md and finish. cwd is claude-configs: use \`go run ./cmd/rules render-skill --list-entries <skillMd>\` for self rows and \`bash cmd/context/context_record.sh <transcript> context\` for context rows (ctx-dir is context). Runs whose id carries ctx-off were invoked with --no-context; runs whose id carries ctx-on received the injected context."
eval_tools="$(routine_allowed_tools skillroutine-eval)"
eval_flags=()
if [[ -n "$eval_tools" ]]; then
  eval_flags=(--allowedTools "$eval_tools")
else
  echo "no allowed-tools manifest for skillroutine-eval; running without --allowedTools"
fi
eval_status=0
# ${arr[@]+...} guards empty-array expansion under set -u on bash 3.2 (launchd PATH).
eval_output=$(routine_run_claude "${ROUTINE_STAGE_TIMEOUT_S:-3600}" claude -p "$eval_prompt" \
  ${eval_flags[@]+"${eval_flags[@]}"} \
  --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
  --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
  --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}" \
  --output-format json) || eval_status=$?

# The claude exit code alone misses an agent that failed while the CLI exited 0.
if [[ -n "$eval_output" ]]; then
  env_error="$(printf '%s' "$eval_output" | jq -r '.is_error // false' 2>/dev/null || echo false)"
  env_subtype="$(printf '%s' "$eval_output" | jq -r '.subtype // ""' 2>/dev/null || echo "")"
  if [[ "$env_error" == "true" || ( -n "$env_subtype" && "$env_subtype" != "success" ) ]]; then
    echo "claude reported failure in envelope (is_error=$env_error subtype=$env_subtype)" >&2
    [[ "$eval_status" -eq 0 ]] && eval_status=70
  fi
fi
if [[ ! -s "$wt/$results_dir/eval.json" ]]; then
  echo "eval stage left no eval.json" >&2
  [[ "$eval_status" -eq 0 ]] && eval_status=70
fi

deltas="$(routine_matrix_deltas "$results_dir" 2>/dev/null)" || deltas='[]'
printf '%s' "$eval_output" | append_result "$eval_status" eval "$(jq -cn --argjson d "$deltas" --arg rd "$results_dir" '{deltas: $d, results_dir: $rd}')"
printf 'skill-eval %s: %s cells, deltas %s\n' "$MATRIX_SKILL" "$cell_count" "$deltas" > "$wt/.routine-commit-body"

# Machine-local absolute paths never reach the committed artifacts; every
# reader of these fields (eval stage, deltas) has already run.
if [[ -s "$wt/$results_dir/manifest.json" ]]; then
  jq 'del(.runs[].transcript, .runs[].worktree)' "$wt/$results_dir/manifest.json" > "$wt/$results_dir/manifest.json.tmp" \
    && mv "$wt/$results_dir/manifest.json.tmp" "$wt/$results_dir/manifest.json"
fi
if [[ -s "$wt/$results_dir/cells.jsonl" ]]; then
  jq -c 'del(.transcript, .worktree) | if .brief then .brief |= sub(".*/routines/"; "routines/") else . end' \
    "$wt/$results_dir/cells.jsonl" > "$wt/$results_dir/cells.jsonl.tmp" \
    && mv "$wt/$results_dir/cells.jsonl.tmp" "$wt/$results_dir/cells.jsonl"
fi
if [[ -s "$wt/$results_dir/cells.tsv" ]]; then
  sed -E 's|/[^[:space:]]*/routines/|routines/|g' "$wt/$results_dir/cells.tsv" > "$wt/$results_dir/cells.tsv.tmp" \
    && mv "$wt/$results_dir/cells.tsv.tmp" "$wt/$results_dir/cells.tsv"
fi

exit_status=$eval_status
routine_matrix_cleanup
cd "$repo_root"
routine_worktree_publish "$exit_status" || echo "publish failed; worktree kept: $wt" >&2
exit "$exit_status"
