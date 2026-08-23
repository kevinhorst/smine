#!/usr/bin/env bash
# Routine wrapper template — copy to routines/<name>/run.sh and edit PROMPT.
# Requires flock (brew install flock; macOS only — Windows runs under
# routinewrap.exe, which holds the locks): the scheduler never overlaps runs.

set -uo pipefail

routine_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$routine_dir/../.." && pwd)"
results="$routine_dir/results.jsonl"

PROMPT='<the claude -p prompt>'
# Operator extension (configure widget: Extra prompt) appended to the main prompt.
[ -n "${ROUTINE_EXTRA_PROMPT:-}" ] && PROMPT="$PROMPT $ROUTINE_EXTRA_PROMPT"
# One-shot Run-Now extension written by the config server; consumed and deleted here.
if [ -s "$routine_dir/.run-now-prompt" ]; then
  PROMPT="$PROMPT $(cat "$routine_dir/.run-now-prompt")"
  rm -f "$routine_dir/.run-now-prompt"
fi

source "$repo_root/routines/_lib/platform.sh"
routine_self_lock || { echo "already running"; exit 0; }

if [[ -n "${ROUTINE_TOKEN:-}" ]]; then
  token_file="$HOME/.config/claude-routine/tokens/$ROUTINE_TOKEN"
else
  token_file="$HOME/.config/claude-routine/token"
fi
if [[ ! -s "$token_file" ]]; then
  echo "token file missing or empty: $token_file" >&2
  exit 78
fi
export CLAUDE_CODE_OAUTH_TOKEN="$(cat "$token_file")"

source "$repo_root/routines/_lib/cadence.sh"
routine_cadence_gate || exit 0

# Chain members set ROUTINE_GROUP=<group> here (shared worktree + branch,
# serialized by the group lock); independent routines keep the default
# (their own name) and run concurrently with every other group.
source "$repo_root/routines/_lib/worktree.sh"
source "$repo_root/routines/_lib/skill.sh"
routine_group_lock || { echo "group lock timeout: $ROUTINE_GROUP" >&2; exit 75; }
wt="$(routine_worktree_create)"; create_rc=$?
if [[ "$create_rc" -eq 3 ]]; then
  echo "open-instance limit reached for $ROUTINE_GROUP (ROUTINE_MAX_OPEN_BRANCHES=$ROUTINE_MAX_OPEN_BRANCHES); merge a $ROUTINE_BRANCH_PREFIX-* branch before the next run" >&2
  exit 0
elif [[ "$create_rc" -ne 0 || -z "$wt" ]]; then
  echo "worktree create failed" >&2
  exit 70
fi
cd "$wt"

SKILL='<skill name invoked by PROMPT>'
allowed_tools="$(routine_allowed_tools "$SKILL")"
allowed_flags=()
if [[ -n "$allowed_tools" ]]; then
  allowed_flags=(--allowedTools "$allowed_tools")
else
  echo "no allowed-tools manifest for $SKILL; running without --allowedTools"
fi

exit_status=0
# ${arr[@]+...} guards empty-array expansion under set -u on bash 3.2 (launchd PATH).
output=$(routine_run_claude "${ROUTINE_STAGE_TIMEOUT_S:-7200}" claude -p "$PROMPT" \
  ${allowed_flags[@]+"${allowed_flags[@]}"} \
  --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
  --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
  --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}" \
  --output-format json) || exit_status=$?

echo "$output" | jq -c \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson exit "$exit_status" \
  '{timestamp: $ts, exit_status: $exit, session_id: .session_id, num_turns: .num_turns, total_cost_usd: .total_cost_usd}' \
  >> "$results"

cd "$repo_root"
routine_worktree_publish "$exit_status" || echo "publish failed; worktree kept: $wt" >&2
