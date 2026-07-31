#!/usr/bin/env bash
# Routine wrapper template — copy to routines/<name>/run.sh and edit PROMPT.
# Requires flock (brew install flock): launchd never overlaps two runs.

set -uo pipefail

routine_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$routine_dir/../.." && pwd)"
results="$routine_dir/results.jsonl"

PROMPT='<the claude -p prompt>'

exec 9>"$routine_dir/.lock"
flock -n 9 || { echo "already running"; exit 0; }

source "$repo_root/routines/_lib/cadence.sh"
routine_cadence_gate || exit 0

# Chain members set ROUTINE_GROUP=<group> here (shared worktree + branch,
# serialized by the group lock); independent routines keep the default
# (their own name) and run concurrently with every other group.
source "$repo_root/routines/_lib/worktree.sh"
source "$repo_root/routines/_lib/skill.sh"
routine_group_lock || { echo "group lock timeout: $ROUTINE_GROUP" >&2; exit 75; }
wt="$(routine_worktree_create)" || { echo "worktree create failed" >&2; exit 70; }
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
output=$(claude -p "$PROMPT" \
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
