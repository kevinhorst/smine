#!/usr/bin/env bash
# Nightly /smine routine (operations + claude -p mechanics: routines/README.md).
# Two stages on one worktree/branch: /smine --nightly (mine + route), then — when
# votes are pending — /smine-apply on the drained votes file. One publish commits both.
# Requires flock + coreutils timeout (brew install flock coreutils; macOS
# only — Windows runs under routinewrap.exe, which holds the locks).

set -uo pipefail

[ -d /opt/homebrew/bin ] && export PATH="/opt/homebrew/bin:$PATH"
export DISABLE_AUTOUPDATER=1
export DISABLE_TELEMETRY=1

routine_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$routine_dir/../.." && pwd)"
results="$routine_dir/results.jsonl"
votes_file="$repo_root/proposals/votes.jsonl"
live_archive="$repo_root/proposals/votes-archive.jsonl"
cap="${SMINE_APPLY_CAP:-3}"
auto_apply="${SMINE_AUTO_APPLY:-never}"
auto_dimensions="${SMINE_AUTO_APPLY_DIMENSIONS:-}"
mined_total="${SMINE_MAX_PROPOSALS_MINED:-}"
mined_per_dimension="${SMINE_MAX_PROPOSALS_PER_DIMENSION:-}"
agents="${SMINE_AGENTS:-claude,codex}"

# Working-repo roster = git-repo entries of the deployed permission config —
# the same list that grants the run directory access (single source of truth).
repos_arg=""
settings_file="$HOME/.claude/settings.json"
if [[ -s "$settings_file" ]]; then
  while IFS= read -r dir; do
    [[ -d "$dir/.git" || -f "$dir/.git" ]] || continue
    repos_arg="${repos_arg:+$repos_arg,}$(basename "$dir")=$dir"
  done < <(jq -r '.permissions.additionalDirectories // [] | .[]' "$settings_file")
fi

PROMPT='/smine --nightly'
[[ -n "$mined_per_dimension" ]] && PROMPT+=" --max-proposals-per-dimension $mined_per_dimension"
[[ -n "$mined_total" ]] && PROMPT+=" --max-proposals-mined $mined_total"
[[ -n "$repos_arg" ]] && PROMPT+=" --repos $repos_arg"
PROMPT+=" --agents $agents"
# Operator extension (configure widget: Extra prompt) appended to the stage-1 prompt only.
[ -n "${ROUTINE_EXTRA_PROMPT:-}" ] && PROMPT="$PROMPT $ROUTINE_EXTRA_PROMPT"
# One-shot Run-Now extension written by the config server; consumed and deleted here.
if [ -s "$routine_dir/.run-now-prompt" ]; then
  PROMPT="$PROMPT $(cat "$routine_dir/.run-now-prompt")"
  rm -f "$routine_dir/.run-now-prompt"
fi

# stdin: claude JSON envelope (may be empty, e.g. after timeout kill); $1: exit status; $2: stage (smine|apply)
append_result() {
  jq -cn \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson exit "$1" \
    --arg stage "$2" \
    'input? // {} | {timestamp: $ts, stage: $stage, exit_status: $exit, session_id: .session_id, num_turns: .num_turns, total_cost_usd: .total_cost_usd, subtype: .subtype, is_error: .is_error, result: ((.result // "") | tostring | .[0:300])}' \
    >> "$results"
}

# Drain terminally-processed votes. $1 is the harvested worktree disposition
# ledger (votes-archive.jsonl, one processed vote + disposition per line).
# Append it to the live archive, then drop every archived (kind,id) from the
# live sidecar. Over-cap votes were never archived and stay live for a future
# run; server appends during the run survive unless they collide with an
# archived key. This is the whole restore story — a partial ledger from a
# half-failed night drains exactly the votes that got done, so progress is
# monotonic and nothing is dropped un-recorded.
drain_votes() {
  local wt_archive="$1"
  [[ -s "$wt_archive" ]] || return 0
  cat "$wt_archive" >> "$live_archive"
  [[ -s "$votes_file" ]] || return 0
  local drained
  drained="$(jq -sc '[.[] | {kind, id}]' "$wt_archive")"
  jq -c --argjson d "$drained" \
    '. as $v | select(($d | any(.kind == $v.kind and .id == $v.id)) | not)' \
    "$votes_file" > "$votes_file.tmp" \
    && mv "$votes_file.tmp" "$votes_file"
}

if [[ -n "${ROUTINE_TOKEN:-}" ]]; then
  token_file="$HOME/.config/claude-routine/tokens/$ROUTINE_TOKEN"
else
  token_file="$HOME/.config/claude-routine/token"
fi
if [[ ! -s "$token_file" ]]; then
  echo "token file missing or empty: $token_file" >&2
  append_result 78 smine </dev/null
  exit 78
fi
export CLAUDE_CODE_OAUTH_TOKEN="$(cat "$token_file")"

source "$repo_root/routines/_lib/platform.sh"
routine_self_lock || { echo "already running"; exit 0; }

source "$repo_root/routines/_lib/worktree.sh"
source "$repo_root/routines/_lib/skill.sh"
routine_group_lock || {
  echo "group lock timeout: $ROUTINE_GROUP" >&2
  append_result 75 smine </dev/null
  exit 75
}
wt="$(routine_worktree_create)"; create_rc=$?
if [[ "$create_rc" -eq 3 ]]; then
  echo "open-instance limit reached for $ROUTINE_GROUP (ROUTINE_MAX_OPEN_BRANCHES=$ROUTINE_MAX_OPEN_BRANCHES); merge a $ROUTINE_BRANCH_PREFIX-* branch before the next run" >&2
  exit 0
elif [[ "$create_rc" -ne 0 || -z "$wt" ]]; then
  echo "worktree create failed" >&2
  append_result 70 smine </dev/null
  exit 70
fi
cd "$wt"

smine_tools="$(routine_allowed_tools smine)"
smine_flags=()
if [[ -n "$smine_tools" ]]; then
  smine_flags=(--allowedTools "$smine_tools")
else
  echo "no allowed-tools manifest for smine; running without --allowedTools"
fi

# ---- Stage 1: mine + route ----
exit_status=0
# ${arr[@]+...} guards empty-array expansion under set -u on bash 3.2 (launchd PATH).
output=$(routine_run_claude 7200 claude -p "$PROMPT" \
  ${smine_flags[@]+"${smine_flags[@]}"} \
  --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
  --effort low \
  --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
  --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}" \
  --output-format json) || exit_status=$?

printf '%s' "$output" | append_result "$exit_status" smine

# ---- Stage 1.5: consolidate proposals (dedup, re-home, schema/audit gate) ----
consolidate_status=0
# Presentation profile: thread the install's language into the consolidate
# rewording pass (the style-correction gate for the proposal store).
consolidate_prompt="/smine-consolidate proposals"
presentation_profile="$HOME/.claude/context/global/presentation-profile.md"
if [[ -f "$presentation_profile" ]]; then
  profile_language=$(sed -n 's/^language:[[:space:]]*//p' "$presentation_profile" | head -1)
  if [[ -n "$profile_language" && "$profile_language" != "en" ]]; then
    consolidate_prompt="/smine-consolidate proposals language $profile_language"
  fi
fi
consolidate_tools="$(routine_allowed_tools smine-consolidate)"
consolidate_flags=()
if [[ -n "$consolidate_tools" ]]; then
  consolidate_flags=(--allowedTools "$consolidate_tools")
else
  echo "no allowed-tools manifest for smine-consolidate; running without --allowedTools"
fi
consolidate_output=$(routine_run_claude 3600 claude -p "$consolidate_prompt" \
  ${consolidate_flags[@]+"${consolidate_flags[@]}"} \
  --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
  --effort low \
  --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
  --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}" \
  --output-format json) || consolidate_status=$?
printf '%s' "$consolidate_output" | append_result "$consolidate_status" smine-consolidate

# ---- Stage 2: apply pending proposal votes (skipped when none) ----
# The votes are COPIED into the worktree so the agent never reads or writes
# outside its cwd; the live sidecar is never emptied by the wrapper — over-cap
# votes simply stay in it, and the post-run drain removes only the ones the
# agent archived. The committed .gitignore covers both sidecar patterns and the
# archive, keeping them out of the publish commit.
apply_status=0
wt_archive_tmp=""
# Auto modes run the stage even without votes: the processing file is then
# created empty so the smine-apply invocation contract holds (D9).
if [[ ( -s "$votes_file" || "$auto_apply" != "never" ) && -d "$wt" ]]; then
  processing_name="votes-processing-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
  copy_ok=1
  if [[ -s "$votes_file" ]]; then
    cp "$votes_file" "$wt/proposals/$processing_name" || copy_ok=0
  else
    : > "$wt/proposals/$processing_name" || copy_ok=0
  fi

  apply_prompt="/smine-apply proposals/$processing_name (implementation cap: $cap)"
  if [[ "$auto_apply" == "decide" ]]; then
    apply_prompt+=" (auto-apply: decide; rules-file: skills/smine/smine-apply/assets/auto-apply-rules.md)"
  elif [[ "$auto_apply" == "always" ]]; then
    apply_prompt+=" (auto-apply: always${auto_dimensions:+; dimensions: $auto_dimensions})"
  fi

  if [[ "$copy_ok" -eq 1 ]]; then
    apply_tools="$(routine_allowed_tools smine-apply)"
    apply_flags=()
    if [[ -n "$apply_tools" ]]; then
      apply_flags=(--allowedTools "$apply_tools")
    else
      echo "no allowed-tools manifest for smine-apply; running without --allowedTools"
    fi
    apply_output=$(routine_run_claude 3600 claude -p "$apply_prompt" \
      ${apply_flags[@]+"${apply_flags[@]}"} \
      --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
      --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
      --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}" \
      --output-format json) || apply_status=$?

    printf '%s' "$apply_output" | append_result "$apply_status" apply

    # The claude exit code alone misses an agent that failed while the CLI exited 0
    # (observed 2026-07-24). Treat an error envelope as failure too.
    if [[ -n "$apply_output" ]]; then
      env_error="$(printf '%s' "$apply_output" | jq -r '.is_error // false' 2>/dev/null || echo false)"
      env_subtype="$(printf '%s' "$apply_output" | jq -r '.subtype // ""' 2>/dev/null || echo "")"
      if [[ "$env_error" == "true" || ( -n "$env_subtype" && "$env_subtype" != "success" ) ]]; then
        echo "claude reported failure in envelope (is_error=$env_error subtype=$env_subtype)" >&2
        [[ "$apply_status" -eq 0 ]] && apply_status=70
      fi
    fi

    # A vanished worktree is a failure regardless of claude's exit status — the
    # agent can do no work without its cwd, yet may still report success.
    if [[ ! -d "$wt" ]]; then
      echo "worktree vanished mid-run: $wt" >&2
      [[ "$apply_status" -eq 0 ]] && apply_status=70
    fi

    # Harvest the disposition ledger before publish — a successful publish removes
    # the worktree, a failed run's worktree is swept by the next create.
    wt_archive_tmp="$(mktemp)"
    cp "$wt/proposals/votes-archive.jsonl" "$wt_archive_tmp" 2>/dev/null || true
  else
    echo "votes copy failed" >&2
    apply_status=70
    append_result 70 apply </dev/null
  fi
else
  echo "no pending votes and auto-apply off; apply stage skipped"
fi

# Combined status: first non-zero of the two stages.
[[ "$exit_status" -eq 0 ]] && exit_status=$apply_status

cd "$repo_root"
publish_status=0
routine_worktree_publish "$exit_status" || {
  publish_status=1
  echo "publish failed; worktree kept: $wt" >&2
}

# Drain only when the branch actually captured the work. A committed publish
# (even after a claude failure — publish commits the partial work) means the
# archived votes' json edits are on the branch, so draining them is safe and
# keeps progress monotonic. A failed publish leaves the live sidecar fully
# intact for a retry.
if [[ -n "$wt_archive_tmp" ]]; then
  if [[ "$publish_status" -eq 0 ]]; then
    drain_votes "$wt_archive_tmp"
  else
    echo "publish failed; live sidecar left intact for retry" >&2
  fi
  rm -f "$wt_archive_tmp"
fi
exit "$exit_status"
