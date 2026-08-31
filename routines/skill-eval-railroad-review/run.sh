#!/usr/bin/env bash

# Railroad-review observability routine (operations + claude -p mechanics:
# routines/README.md). Headless claude -p has NO Workflow tool and NO
# mid-session Skill tool, so this wrapper orchestrates the fan-out itself:
# one claude -p cell per lane, per refuter and for the station, all strictly
# sequential IN THE ROUTINE GROUP WORKTREE — the exact cwd the config server
# matches a routine's session on, so the status page shows the currently
# running cell live. Each cell invokes the skill as a prompt-START slash
# command (bare — lanes receive the full doctrine injection, CTX-002); every
# envelope is captured, so tokens/time/cost are measured per stage (lanes,
# refute, station). The wrapper then computes the lane trace (injection
# channels, spill reads, per-cell cost) mechanically and — once >=3
# runs share an arm label — the eviction screen. Artifacts land under
# evals/railroad-review/<date>-<hash6>/ (hash = head commit) on the routine
# branch; the arm label is the deployed railroad-review skill version.
# Deviation from the interactive pipeline, stated: no grouper agent
# (within-direction dedup is a mechanical exact file+line collapse here).
# Requires flock + coreutils timeout (brew install flock coreutils).

set -uo pipefail

[ -d /opt/homebrew/bin ] && export PATH="/opt/homebrew/bin:$PATH"
export DISABLE_AUTOUPDATER=1
export DISABLE_TELEMETRY=1

routine_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$routine_dir/../.." && pwd)"
results="$routine_dir/results.jsonl"
review_base="${ROUTINE_REVIEW_BASE:-HEAD~2}"
review_lanes="${ROUTINE_REVIEW_LANES:-2}"
review_directions="$(printf '%s' "${ROUTINE_REVIEW_DIRECTIONS:-code-style correctness critical}" | tr ',' ' ')"
refute_level="${ROUTINE_REFUTE_LEVEL:-major}"
refute_max="${ROUTINE_REFUTE_MAX:-6}"
cell_budget="${ROUTINE_CELL_BUDGET_USD:-4}"

case "$refute_level" in
  blocker) refute_rank=5 ;;
  major)   refute_rank=4 ;;
  minor)   refute_rank=3 ;;
  nit)     refute_rank=2 ;;
  info)    refute_rank=1 ;;
  none)    refute_rank=99 ;;
  *) echo "invalid ROUTINE_REFUTE_LEVEL: $refute_level" >&2; refute_rank=4 ;;
esac

# Context-requirement glob prefixes per direction — mirrors the SKILL.md
# Directions table (DIR-003); update both together.
dir_globs() {
  case "$1" in
    code-style)     echo "ACTION-REVIEW-QUALITY- RULE-GOLANG- RULE-PYTHON-" ;;
    correctness)    echo "ACTION-REVIEW-SPEC-" ;;
    critical)       echo "ACTION-CONCEPT-HOT-" ;;
    data-integrity) echo "ACTION-IMPL-INTEG-" ;;
    tests)          echo "ACTION-REVIEW-TEST-" ;;
    contracts)      echo "ACTION-REVIEW-DEPLOY-" ;;
    *)              echo "" ;;
  esac
}

# stdin: claude JSON envelope (may be empty); $1: exit status
append_result() {
  jq -cn \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson exit "$1" \
    'input? // {} | {timestamp: $ts, exit_status: $exit, session_id: .session_id, num_turns: .num_turns, total_cost_usd: .total_cost_usd, subtype: .subtype, is_error: .is_error, result: ((.result // "") | tostring | .[0:300])}' \
    >> "$results"
}

if [[ -n "${ROUTINE_TOKEN:-}" ]]; then
  token_file="$HOME/.config/claude-routine/tokens/$ROUTINE_TOKEN"
else
  token_file="$HOME/.config/claude-routine/token"
fi
if [[ ! -s "$token_file" ]]; then
  echo "token file missing or empty: $token_file" >&2
  append_result 78 </dev/null
  exit 78
fi
export CLAUDE_CODE_OAUTH_TOKEN="$(cat "$token_file")"

source "$repo_root/routines/_lib/platform.sh"
routine_self_lock || { echo "already running"; exit 0; }

source "$repo_root/routines/_lib/worktree.sh"
source "$repo_root/routines/_lib/skill.sh"
routine_group_lock || {
  echo "group lock timeout: $ROUTINE_GROUP" >&2
  append_result 75 </dev/null
  exit 75
}
wt="$(routine_worktree_create)"; create_rc=$?
if [[ "$create_rc" -eq 3 ]]; then
  echo "open-instance limit reached for $ROUTINE_GROUP (ROUTINE_MAX_OPEN_BRANCHES=$ROUTINE_MAX_OPEN_BRANCHES); merge a $ROUTINE_BRANCH_PREFIX-* branch before the next run" >&2
  exit 0
elif [[ "$create_rc" -ne 0 || -z "$wt" ]]; then
  echo "worktree create failed" >&2
  append_result 70 </dev/null
  exit 70
fi
cd "$wt" || exit 70

baseCommit="$(git rev-parse --verify "$review_base" 2>/dev/null)" || {
  echo "unresolvable ROUTINE_REVIEW_BASE: $review_base" >&2
  append_result 65 </dev/null
  exit 65
}
headCommit="$(git rev-parse HEAD)"
if [[ "$(git rev-list --count "$baseCommit..$headCommit")" -eq 0 ]]; then
  echo "empty range $review_base..HEAD — nothing to review"
  append_result 0 </dev/null
  exit 0
fi
diff_files="$(git diff --name-only "$baseCommit" "$headCommit")"

arm="$(awk '/^---$/{f++; next} f==1 && sub(/^version:[ \t]*/, "") {print; exit}' "$HOME/.claude/skills/railroad-review/SKILL.md" 2>/dev/null)"
[[ -n "$arm" ]] || arm="unknown"

# Dir name follows the config-server eval contract (internal/evals):
# evals/railroad-review/<date>-<hash6> (hash = head commit — collision-free
# per commit; same-commit rerun gets a numeric suffix). Cell artifacts go
# into runs/ — the one subdir the server's artifact listing serves.
run_dir="evals/railroad-review/$(date -u +%Y-%m-%d)-$(git rev-parse --short=6 "$headCommit")"
[[ -e "$wt/$run_dir" ]] && run_dir="$run_dir-$(date -u +%H%M)"
mkdir -p "$wt/$run_dir/runs"
cells_file="$wt/$run_dir/cells.jsonl"
: > "$cells_file"

# Run-history start row — the run is visible in the UI while it executes.
jq -cn \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg summary "run started — arm $arm, directions: $review_directions, lanes/direction: $review_lanes, artifacts: $run_dir" \
  '{timestamp: $ts, exit_status: 0, subtype: "started", is_error: false, result: $summary}' >> "$results"

tools="$(routine_allowed_tools railroad-review)"
if [[ -n "$tools" ]]; then
  tools="$tools, Glob, Grep, Bash(ls *), Bash(date *)"
else
  echo "no allowed-tools manifest for railroad-review; using a minimal set" >&2
  tools="Read, Write, Glob, Grep, Bash(git diff*), Bash(git log*), Bash(git rev-parse*), Bash(jq *), Bash(ls *)"
fi

# Operator extension (configure widget: Extra prompt) plus the one-shot
# Run-Now extension written by the config server (consumed and deleted here);
# appended to the lane and station prompts.
extra_prompt="${ROUTINE_EXTRA_PROMPT:-}"
if [ -s "$routine_dir/.run-now-prompt" ]; then
  extra_prompt="${extra_prompt:+$extra_prompt }$(cat "$routine_dir/.run-now-prompt")"
  rm -f "$routine_dir/.run-now-prompt"
fi

overall=0

# One claude -p cell, run sequentially in the routine group worktree — its
# cwd is what the config server matches the routine's live session on. The
# cell writes its artifacts directly into $run_dir. $1 id, $2 stage,
# $3 prompt, $4 timeout seconds. Appends one cells.jsonl row.
run_cell() {
  local id="$1" stage="$2" prompt="$3" tmo="$4"
  local status=0 output sid transcript env_error env_subtype
  echo "cell $id ($stage)"
  output="$(cd "$wt" && routine_run_claude "$tmo" claude -p "$prompt" \
    --allowedTools "$tools" \
    --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
    --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
    --max-budget-usd "$cell_budget" \
    --output-format json)" || status=$?
  sid="$(printf '%s' "$output" | jq -r '.session_id // empty' 2>/dev/null || true)"
  transcript=""
  [[ -n "$sid" ]] && transcript="$(find "$HOME/.claude/projects" -name "$sid.jsonl" -print 2>/dev/null | head -n1)"
  env_error="$(printf '%s' "$output" | jq -r '.is_error // false' 2>/dev/null || echo false)"
  env_subtype="$(printf '%s' "$output" | jq -r '.subtype // ""' 2>/dev/null || echo "")"
  if [[ "$env_error" == "true" || ( -n "$output" && -n "$env_subtype" && "$env_subtype" != "success" ) ]]; then
    echo "cell $id: failure envelope (is_error=$env_error subtype=$env_subtype)" >&2
    [[ "$status" -eq 0 ]] && status=70
  fi
  printf '%s' "$output" | jq -cn \
    --arg id "$id" --arg stage "$stage" --arg sid "$sid" --arg transcript "$transcript" \
    --argjson exit "$status" \
    'input? // {} | {id: $id, stage: $stage, session_id: $sid, transcript: $transcript, exit_status: $exit,
      ok: ($exit == 0), is_error: (.is_error // false), subtype: (.subtype // ""), cost_usd: .total_cost_usd,
      num_turns: .num_turns, duration_ms: .duration_ms, output_tokens: (.usage.output_tokens // null)}' \
    >> "$cells_file"
  [[ "$status" -ne 0 ]] && { [[ "$overall" -eq 0 ]] && overall="$status"; return 1; }
  return 0
}

# ---- Stage 1: lanes ----
for d in $review_directions; do
  i=1
  while [[ "$i" -le "$review_lanes" ]]; do
    # Plain concatenation, never a heredoc inside $() — bash 3.2 mis-parses
    # parens in such heredoc bodies and executes the prompt as commands.
    lane_prompt="/railroad-review"
    lane_prompt+=$'\n'"REQUIRE-SKILL:railroad-review"
    lane_prompt+=$'\n'"You are lane $i of $review_lanes on the \"$d\" direction of a headless railroad-review measurement round on $baseCommit..$headCommit. The Workflow tool does not exist in this session — you ARE the lane; never attempt to fan out, spawn agents or invoke further skills."
    lane_prompt+=$'\n'"Follow the \"$d\" direction definition in the loaded railroad-review skill exactly: your review context arrives via the skill invocation's injection — do not read context-entry files from the repo's context directory (the gate denies it; the change's language guide is the only exception), walk only that direction's checklist. Diff with: git diff $baseCommit HEAD. Read each changed file in full, not just the hunks. The changed files are:"
    lane_prompt+=$'\n'"$diff_files"
    lane_prompt+=$'\n'"FALSIFIABILITY (relaxed for code-style, whose ground truth is the style guide): every claim names the concrete input, state or sequence that produces the wrong behaviour; naming opinions, architecture preference and unquantified robustness are rejected at the source."
    lane_prompt+=$'\n'"CONFIRMATION PASS: before finishing, re-read each claim's cited code in full context; drop any claim that does not survive; record what you re-read in the claim's evidence field."
    lane_prompt+=$'\n'"Review ONLY — no edits to any repo file. Your only writes are your two artifacts: $run_dir/runs/lane-$d-$i.json with exactly {\"direction\",\"lane\",\"aborted\",\"files_reviewed\":[...],\"findings\":[{\"finding_id\":\"$d-<SEV>-<n>\",\"file\",\"line\",\"severity\":\"BLOCKER|MAJOR|MINOR|NIT|INFO\",\"claim\",\"fix\",\"evidence\"}]} (severity by consequence; empty findings with aborted=false is a valid clean review) and $run_dir/runs/lane-$d-$i.md (findings table). Never prompt or pause; write both artifacts, then terminate."
    [[ -n "$extra_prompt" ]] && lane_prompt+=$'\n'"$extra_prompt"
    run_cell "lane-$d-$i" lanes "$lane_prompt" 2400 || true
    i=$((i + 1))
  done
done

# ---- Stage 2: refutation (mechanical within-round dedup, then one refuter
# cell per candidate at/above the threshold, capped) ----
shopt -s nullglob
lane_jsons=("$wt/$run_dir/runs/"lane-*.json)
shopt -u nullglob
candidates="[]"
if [[ "${#lane_jsons[@]}" -gt 0 && "$refute_rank" -lt 99 ]]; then
  candidates="$(jq -s --argjson thr "$refute_rank" --argjson max "$refute_max" \
    '[.[] | .findings[]?]
     | map(select(({"BLOCKER":5,"MAJOR":4,"MINOR":3,"NIT":2,"INFO":1}[.severity] // 0) >= $thr))
     | sort_by(-({"BLOCKER":5,"MAJOR":4,"MINOR":3,"NIT":2,"INFO":1}[.severity] // 0))
     | unique_by([.file, .line]) | .[0:$max]' "${lane_jsons[@]}" 2>/dev/null || echo '[]')"
fi
n_candidates="$(jq 'length' <<<"$candidates")"
total_major=0
if [[ "${#lane_jsons[@]}" -gt 0 ]]; then
  total_major="$(jq -s --argjson thr "$refute_rank" \
    '[.[] | .findings[]? | select(({"BLOCKER":5,"MAJOR":4,"MINOR":3,"NIT":2,"INFO":1}[.severity] // 0) >= $thr)] | length' \
    "${lane_jsons[@]}" 2>/dev/null || echo 0)"
fi
echo "refute: $n_candidates candidate(s) at threshold $refute_level (of $total_major pre-dedup; cap $refute_max)"
k=0
while IFS= read -r finding; do
  [[ -n "$finding" ]] || continue
  k=$((k + 1))
  fid="$(jq -r '.finding_id // "cand"' <<<"$finding" | tr -c 'a-zA-Z0-9' '-' | sed 's/-*$//')"
  refute_prompt="/railroad-review"
  refute_prompt+=$'\n'"You are a fresh refuter in a headless railroad-review measurement round on $baseCommit..$headCommit — you did NOT produce the claim below; your brief is to REFUTE it (its second confirmation). The Workflow tool does not exist in this session; work alone."
  refute_prompt+=$'\n'"The claim, from a review lane:"
  refute_prompt+=$'\n'"$finding"
  refute_prompt+=$'\n'"Re-read the cited code in full context (git diff $baseCommit HEAD for the change; the file itself for surroundings). Actively look for why the claim is wrong: a guard the lane missed, an unreachable path, a misread contract. Verdict: \"confirmed\" (survives your attack), \"debunked\" (you found the counter-evidence — name it) or \"unverified\" (cannot be decided from the code)."
  refute_prompt+=$'\n'"Your only writes: $run_dir/runs/refute-$k-$fid.json with exactly {\"finding_id\",\"verdict\":\"confirmed|debunked|unverified\",\"reasoning\",\"evidence\"}. Never prompt or pause; write it, then terminate."
  run_cell "refute-$k" refute "$refute_prompt" 1800 || true
done < <(jq -c '.[]' <<<"$candidates")

# ---- Stage 3: station ----
station_prompt="/railroad-review"
station_prompt+=$'\n'"You are the station-merge of a headless railroad-review measurement round on $baseCommit..$headCommit. The Workflow tool does not exist in this session; you are the only consolidator. The lane findings are in $run_dir/runs/lane-*.json and the refuter verdicts in $run_dir/runs/refute-*.json — read them all."
station_prompt+=$'\n'"Duties: union all claims across lanes and directions; dedup same-defect claims (within and across directions) to the best-argued survivor at max group severity, recording absorbed ids as merged_from; apply refuter verdicts — a debunked verdict kills its claim unless it is plainly inconsistent with the code you re-read; re-verify every surviving claim against the tree (this checkout IS the reviewed tree); reconcile severity by consequence."
station_prompt+=$'\n'"Your only writes: $run_dir/review.json with exactly {\"findings\":[{\"finding_id\",\"file\",\"line\",\"severity\",\"claim\",\"fix\",\"evidence\",\"merged_from\":[...],\"refuter_verdict\":null|\"confirmed|debunked|unverified\"}],\"funnel\":{\"claims_produced\",\"after_dedup\",\"debunked\",\"survivors\"}} and $run_dir/review.md (findings table + funnel line). Never prompt or pause; write both, then terminate."
[[ -n "$extra_prompt" ]] && station_prompt+=$'\n'"$extra_prompt"
run_cell station station "$station_prompt" 3600 || true

# ---- Stage 4: mechanical trace (no model calls) ----
trace_tmp="$(mktemp)"
while IFS= read -r row; do
  stage="$(jq -r '.stage' <<<"$row")"
  [[ "$stage" == "lanes" ]] || continue
  id="$(jq -r '.id' <<<"$row")"
  transcript="$(jq -r '.transcript // ""' <<<"$row")"
  d="${id#lane-}"; d="${d%-*}"
  reads="[]"; injected=0; spill_reads=0; ctx_end=null
  if [[ -n "$transcript" && -f "$transcript" ]]; then
    reads="$(jq -c '[select(.type=="assistant") | .message.content[]? | select(.type=="tool_use" and .name=="Read") | .input.file_path | select(test("/context/"))] ' "$transcript" 2>/dev/null | jq -sc 'add // []' || echo '[]')"
    injected="$(grep -c "SKILL CONTEXT /railroad-review" "$transcript" 2>/dev/null || true)"
    spill_reads="$(jq -r '[select(.type=="assistant") | .message.content[]? | select(.type=="tool_use" and .name=="Read") | .input.file_path | select(test("context-required/"))] | length' "$transcript" 2>/dev/null | jq -s 'add // 0' || echo 0)"
    ctx_end="$(jq -s '[.[] | select(.type=="assistant") | .message.usage | select(. != null)] | if length > 0 then (last | (.cache_read_input_tokens + .cache_creation_input_tokens + .input_tokens)) else null end' "$transcript" 2>/dev/null || echo null)"
  fi
  jq -cn --arg id "$id" --arg direction "$d" \
    --argjson reads "$reads" \
    --argjson injected "${injected:-0}" --argjson spill "${spill_reads:-0}" --argjson ctx "$ctx_end" \
    --argjson row "$row" \
    '{id: $id, direction: $direction, read_ctx: $reads,
      injected: $injected, spill_reads: $spill, ctx_at_end: $ctx,
      output_tokens: $row.output_tokens, cost_usd: $row.cost_usd, duration_ms: $row.duration_ms}' \
    >> "$trace_tmp"
done < <(cat "$cells_file")

stage_rollup="$(jq -sc 'group_by(.stage) | map({stage: .[0].stage, cells: length,
  cost_usd: (map(.cost_usd // 0) | add), output_tokens: (map(.output_tokens // 0) | add),
  wall_ms: (map(.duration_ms // 0) | add), failed: (map(select(.ok | not)) | length)})' "$cells_file")"
jq -sc --arg arm "$arm" --arg base "$baseCommit" --arg head "$headCommit" \
  --arg date "$(date -u +%Y-%m-%d)" --argjson stages "$stage_rollup" \
  '{arm: $arm, date: $date, base: $base, head: $head, stages: $stages, lanes: .}' \
  "$trace_tmp" > "$wt/$run_dir/trace.json"
rm -f "$trace_tmp"

# eval.json per the skillroutine-eval v2 schema (internal/evals) — mechanical
# metrics only, derived from cells.jsonl + trace.json; no rubric scoring.
jq -s --arg arm "$arm" --arg date "$(date -u +%Y-%m-%d)" \
  --arg model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}" \
  --arg notes "headless observability routine — arm $arm, range $baseCommit..$headCommit; mechanical metrics only (no rubric scoring); source: cells.jsonl + trace.json" \
  --slurpfile tr "$wt/$run_dir/trace.json" '
  def lane($id): ($tr[0].lanes // []) | map(select(.id == $id)) | first;
  {schemaVersion: "2.0",
   eval: {skill: "railroad-review", date: $date, notes: $notes},
   rubric: [], probes: [], scores: [], totals: [], sharedTotals: [],
   runs: [.[] | {id: .id, model: {id: $model, effort: "", mode: "headless"}, variant: {name: ("arm-" + $arm), disable: []}}],
   metrics: [
     {id: "cost_usd",      label: "Cost USD",                 unit: "USD",    direction: "lower", source: "probe"},
     {id: "output_tokens", label: "Output tokens",            unit: "tokens", direction: "lower", source: "probe"},
     {id: "duration_ms",   label: "Wall time",                unit: "ms",     direction: "lower", source: "probe"},
     {id: "ctx_at_end",    label: "Context at end",           unit: "tokens", direction: "lower",  source: "probe"},
     {id: "injected",      label: "Doctrine injections",      unit: "count",  direction: "higher", source: "probe"}
   ],
   metricValues: [.[] | . as $c |
     [{metricId: "cost_usd", v: $c.cost_usd}, {metricId: "output_tokens", v: $c.output_tokens},
      {metricId: "duration_ms", v: $c.duration_ms},
      {metricId: "ctx_at_end", v: (lane($c.id) | .ctx_at_end)},
      {metricId: "injected",   v: (lane($c.id) | .injected)}]
     | map({metricId: .metricId, runId: $c.id, value: .v, note: (if .v == null then "not applicable / cell failed" else "" end)})
   ] | flatten}' "$cells_file" > "$wt/$run_dir/eval.json"

{
  echo "# Railroad-review measurement — $(date -u +%Y-%m-%d), arm $arm"
  echo
  echo "Range: $baseCommit..$headCommit | directions: $review_directions | lanes/direction: $review_lanes | refute: $refute_level (cap $refute_max, $n_candidates spawned)"
  echo
  echo "| Stage | Cells | Failed | Cost USD | Output tokens | Wall ms |"
  echo "|---|---|---|---|---|---|"
  jq -r '.[] | "| \(.stage) | \(.cells) | \(.failed) | \(.cost_usd) | \(.output_tokens) | \(.wall_ms) |"' <<<"$stage_rollup"
  echo
  echo "| Lane | Injected | Spill reads | Ctx reads | Ctx at end | Output tokens | Wall ms |"
  echo "|---|---|---|---|---|---|---|"
  jq -r '.lanes[] | "| \(.id) | \(.injected) | \(.spill_reads) | \(.read_ctx | length) | \(.ctx_at_end) | \(.output_tokens) | \(.duration_ms) |"' "$wt/$run_dir/trace.json"
  echo
  echo "Funnel: $(jq -c '.funnel // "station produced no review.json"' "$wt/$run_dir/review.json" 2>/dev/null || echo '"station produced no review.json"')"
  [[ "$overall" -ne 0 ]] && echo "" && echo "RUN FAILURES occurred — see cells.jsonl (exit_status != 0 rows)."
} > "$wt/$run_dir/report.md"

# ---- Stage 5: eviction screen (>=3 same-arm runs) ----
arm_traces="$(find "$wt/evals" -path "*railroad-review*" -name trace.json -exec grep -l "\"arm\":\"$arm\"" {} \; 2>/dev/null)"
n_traces="$(printf '%s\n' "$arm_traces" | grep -c . || true)"
if [[ "${n_traces:-0}" -ge 3 ]]; then
  all_reads="$(printf '%s\n' "$arm_traces" | while IFS= read -r t; do jq -r '.lanes[].read_ctx[]' "$t" 2>/dev/null; done | sort -u)"
  all_cited="$(find "$wt/evals" -path "*railroad-review*" -name review.json -exec cat {} \; 2>/dev/null | grep -ohE "(ACTION|RULE|FACT)-[A-Z-]+-[0-9]{3}" | sort -u)"
  {
    echo "# Eviction candidates — arm $arm, $n_traces runs (observational; rule per plans/railroad-review-observability/runbooks/eviction-screen.md)"
    echo
    echo "| Entry | Source | Read? | Cited? |"
    echo "|---|---|---|---|"
    for d in $review_directions; do
      globs="$(dir_globs "$d")"
      [[ -n "$globs" ]] || continue
      jq -r --arg globs "$globs" \
        '.entries[] | select(.enforcement != "gate" and .enforcement != "lint")
         | select(.id as $i | ($globs | split(" ") | map(select(length > 0)) | any(. as $g | $i | startswith($g))))
         | [.id, .source] | @tsv' "$wt/context/context.json" 2>/dev/null
    done | sort -u | while IFS=$'\t' read -r eid esrc; do
      r=no; c=no
      printf '%s\n' "$all_reads" | grep -q "$(basename "$esrc")" 2>/dev/null && r=yes
      printf '%s\n' "$all_cited" | grep -qx "$eid" 2>/dev/null && c=yes
      [[ "$r" == "no" && "$c" == "no" ]] && echo "| $eid | $esrc | $r | $c |"
    done
  } > "$wt/$run_dir/eviction-candidates.md"
  echo "eviction screen written ($n_traces same-arm runs)"
else
  echo "eviction screen skipped (${n_traces:-0} same-arm runs, need 3)"
fi

# Unified run-history row: station session, summed cost/turns, ok/failed summary.
station_sid="$(jq -r 'select(.stage == "station") | .session_id // empty' "$cells_file" | tail -n1)"
summary="$(jq -s --arg run_dir "$run_dir" \
  '"cells " + (map(select(.ok)) | length | tostring) + "/" + (length | tostring) + " ok" +
   (if any(.[]; .ok | not) then " (failed: " + ([.[] | select(.ok | not) | .id] | join(", ")) + ")" else "" end) +
   "; cost $" + ((map(.cost_usd // 0) | add * 100 | round / 100) | tostring) + "; artifacts " + $run_dir' "$cells_file")"
jq -cn \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --argjson exit "$overall" --arg sid "$station_sid" \
  --argjson cost "$(jq -s 'map(.cost_usd // 0) | add' "$cells_file")" \
  --argjson turns "$(jq -s 'map(.num_turns // 0) | add' "$cells_file")" \
  --argjson summary "$summary" \
  '{timestamp: $ts, exit_status: $exit, session_id: (if $sid == "" then null else $sid end),
    num_turns: $turns, total_cost_usd: $cost, subtype: "summary", is_error: ($exit != 0), result: $summary}' >> "$results"

cd "$repo_root" || exit 70
routine_worktree_publish "$overall" || {
  echo "publish failed; worktree kept: $wt" >&2
}
exit "$overall"
