#!/usr/bin/env bash
# One-shot smine bootstrap orchestrator — Welcome button (internal/server/
# bootstrap.go) or an operator's `bash cmd/bootstrap/run.sh`.
# Headless claude -p has no Skill/Workflow tool and ignores frontmatter
# allowed-tools, so this wrapper owns sequencing, permissions (installed
# skill manifests as --allowedTools), and the terminal state; the stage
# agents own judgment. Runs on the main checkout, main branch, clean tree;
# /smine-orchestrate bootstrap commits the result on main.
# Env: BOOTSTRAP_TOKEN_FILE (required), BOOTSTRAP_SINCE, BOOTSTRAP_N
# (default 30, ignored with SINCE), BOOTSTRAP_EXTRA_PROMPT,
# BOOTSTRAP_DRY_RUN=1 (print stage commands, run nothing).

set -uo pipefail
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL='*'
[ -d /opt/homebrew/bin ] && export PATH="/opt/homebrew/bin:$PATH"
export DISABLE_AUTOUPDATER=1
export DISABLE_TELEMETRY=1

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"
cd "$repo_root" || exit 70
results="${TMPDIR:-/tmp}/smine-bootstrap-results.jsonl"
dry_run="${BOOTSTRAP_DRY_RUN:-0}"

# shellcheck source=routines/_lib/platform.sh
source "$repo_root/routines/_lib/platform.sh"
# shellcheck source=routines/_lib/skill.sh
source "$repo_root/routines/_lib/skill.sh"

# ---- Preflight (the retired skill's clean-tree gate, now deterministic) ----
branch="$(git symbolic-ref --short HEAD 2>/dev/null || echo '')"
if [ "$branch" != "main" ]; then
  echo "bootstrap requires the main checkout on main (found: ${branch:-detached})" >&2
  exit 64
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "bootstrap requires a clean tree — commit or clean first (orchestrate commits the whole tree)" >&2
  exit 64
fi
for dep in jq shellcheck; do
  command -v "$dep" >/dev/null 2>&1 || echo "missing dependency: $dep (acdsl gates will report it; install it before the next run)" >&2
done

if [ "$dry_run" != "1" ]; then
  if [ ! -s "${BOOTSTRAP_TOKEN_FILE:-}" ]; then
    echo "token file missing or empty: ${BOOTSTRAP_TOKEN_FILE:-<unset>}" >&2
    exit 78
  fi
  CLAUDE_CODE_OAUTH_TOKEN="$(tr -d '[:space:]' < "$BOOTSTRAP_TOKEN_FILE")"
  export CLAUDE_CODE_OAUTH_TOKEN
fi

# ---- Presentation profile → stage parameters (nightly pattern) ----
presentation_profile="$HOME/.claude/context/global/presentation-profile.md"
profile_language=""
profile_audience=""
profile_dev_mode=""
if [ -f "$presentation_profile" ]; then
  profile_language=$(sed -n 's/^language:[[:space:]]*//p' "$presentation_profile" | head -1)
  profile_audience=$(sed -n 's/^audience:[[:space:]]*//p' "$presentation_profile" | head -1)
  profile_dev_mode=$(sed -n 's/^dev-mode:[[:space:]]*//p' "$presentation_profile" | head -1)
fi
dev_mode="${SMINE_DEV_MODE:-}"
[ -z "$dev_mode" ] && [ "$profile_dev_mode" = "true" ] && dev_mode=1
[ "$profile_audience" = "casual" ] && dev_mode=""

# Casual installs hide skills via user-settings skillOverrides "off"; the
# project-scope overlay re-enables them for this run (nightly precedent,
# gitignored so the orchestrate commit never picks it up).
skill_overlay="$HOME/.config/claude-routine/skill-overrides.json"
if [ -f "$skill_overlay" ] && [ "$dry_run" != "1" ]; then
  mkdir -p "$repo_root/.claude"
  cp "$skill_overlay" "$repo_root/.claude/settings.local.json"
fi

# Working-repo roster from the deployed permission config (nightly pattern).
repos_arg=""
settings_file="$HOME/.claude/settings.json"
if [ -s "$settings_file" ]; then
  while IFS= read -r dir; do
    [ -d "$dir/.git" ] || [ -f "$dir/.git" ] || continue
    repos_arg="${repos_arg:+$repos_arg,}$(basename "$dir")=$dir"
  done < <(jq -r '.permissions.additionalDirectories // [] | .[]' "$settings_file")
fi

# ---- Stage prompts ----
n="${BOOTSTRAP_N:-30}"
since="${BOOTSTRAP_SINCE:-}"

mine_prompt="/smine --nightly"
if [ -n "$since" ]; then
  mine_prompt="$mine_prompt --since $since"
else
  mine_prompt="$mine_prompt --last $n"
fi
[ -n "$dev_mode" ] && mine_prompt="$mine_prompt --dev"
[ -n "$repos_arg" ] && mine_prompt="$mine_prompt --repos $repos_arg"
mine_prompt="$mine_prompt --agents ${SMINE_AGENTS:-claude,codex}"
[ -n "${BOOTSTRAP_EXTRA_PROMPT:-}" ] && mine_prompt="$mine_prompt ${BOOTSTRAP_EXTRA_PROMPT}"

consolidate_prompt="/smine-consolidate proposals"
if [ -n "$profile_language" ] && [ "$profile_language" != "en" ]; then
  consolidate_prompt="/smine-consolidate proposals language $profile_language"
fi

apply_name="votes-processing-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
apply_prompt="/smine-apply proposals/$apply_name (implementation cap: ${SMINE_APPLY_CAP:-3}) (auto-apply: decide; rules-file: skills/smine/smine-apply/assets/auto-apply-rules.md)"

orchestrate_prompt="/smine-orchestrate bootstrap"
[ -n "$since" ] && orchestrate_prompt="$orchestrate_prompt (since: $since)"

# ---- Stage runner ----
# run_stage <skill-name> <timeout_s> <effort> <prompt>
# Logs the claude JSON envelope + exit status per stage; never aborts the
# run — the orchestrate judge is the terminal arbiter (plan D7).
overall_status=0
run_stage() {
  local skill_name="$1" timeout_s="$2" effort="$3" prompt="$4"
  local tools status output
  local flags=()
  tools="$(routine_allowed_tools "$skill_name")"
  if [ -n "$tools" ]; then
    flags=(--allowedTools "$tools")
  else
    echo "no allowed-tools manifest for $skill_name; running without --allowedTools" >&2
  fi
  if [ "$dry_run" = "1" ]; then
    echo "DRY: claude -p \"$prompt\" [skill=$skill_name timeout=${timeout_s}s effort=$effort tools=${tools:-none}]"
    return 0
  fi
  status=0
  output=$(routine_run_claude "$timeout_s" claude -p "$prompt" \
    ${flags[@]+"${flags[@]}"} \
    --add-dir "$HOME/.claude/context/global" \
    --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
    --effort "$effort" \
    --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
    --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}" \
    --output-format json) || status=$?
  printf '%s' "$output" | jq -cn \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson exit "$status" \
    --arg stage "$skill_name" \
    'input? // {} | {timestamp: $ts, stage: $stage, exit_status: $exit, session_id: .session_id, is_error: .is_error, subtype: .subtype, result: ((.result // "") | tostring | .[0:300])}' \
    >> "$results"
  if [ "$status" -ne 0 ]; then
    echo "stage $skill_name exited $status (continuing — orchestrate judges the result)" >&2
  fi
  [ "$overall_status" -eq 0 ] && overall_status=$status
  return 0
}

run_stage smine-style 3600 medium "/smine-style $n"
run_stage smine 10800 medium "$mine_prompt"
run_stage smine-consolidate 3600 low "$consolidate_prompt"
if [ "$dry_run" != "1" ]; then
  : > "$repo_root/proposals/$apply_name"
fi
run_stage smine-apply 3600 medium "$apply_prompt"
run_stage smine-orchestrate 3600 medium "$orchestrate_prompt"

# ---- Postconditions (wrapper-owned, nightly stage-3 pattern) ----
if [ "$dry_run" = "1" ]; then
  exit 0
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "bootstrap ended with a dirty tree — orchestrate did not reach a terminal commit; see $results and .orchestrate-report" >&2
  [ -f "$repo_root/.orchestrate-report" ] && cat "$repo_root/.orchestrate-report" >&2
  exit 70
fi
rm -f "$repo_root/.orchestrate-report"
echo "bootstrap complete (stages logged to $results)"
exit "$overall_status"
