#!/usr/bin/env bash
# Headless A/B matrix runner for skill evals. Sourced by routines/<name>/run.sh
# after platform.sh and worktree.sh; requires $repo_root, $routine_dir, $wt
# (the routine worktree, cwd of the eval stage), routine_run_claude, and
# CLAUDE_CODE_OAUTH_TOKEN exported.
#
# Every cell is a real `claude -p` session in its own detached worktree off
# the routine worktree's HEAD — the UserPromptSubmit skill-context hook fires
# exactly as for a human invocation and the session JSONL is recoverable by
# session_id, so the eval's context axis measures what the run really got.
# Two arm dimensions: context arms (on|off — off appends --no-context to the
# invocation) and skill variants (name=disable-list, rendered by
# sync_skills.sh --variant and invoked as /<skill>--<name>). The runner writes
# cells.jsonl and the skillroutine-eval v2 manifest; scoring is the wrapper's
# eval stage; deltas are computed here from eval.json.
#
# Env knobs (all defaulted): ROUTINE_EVAL_SKILL, ROUTINE_EVAL_CONTEXT_ARMS,
# ROUTINE_EVAL_VARIANTS ("name=ID,glob;name2=..."), ROUTINE_EVAL_MODELS,
# ROUTINE_EVAL_EFFORTS, ROUTINE_EVAL_REPLICAS, ROUTINE_EVAL_BRIEFS (comma of
# basenames under briefs/<skill>/; empty = all), ROUTINE_EVAL_CELL_TIMEOUT_S,
# ROUTINE_EVAL_CELL_BUDGET_USD, ROUTINE_EVAL_CELL_TOOLS,
# ROUTINE_EVAL_QUALITY_CONTEXT, ROUTINE_EVAL_CONTEXT_FILES (comma paths).

MATRIX_MAX_CELLS=16
MATRIX_SKILL="${ROUTINE_EVAL_SKILL:-fexplore}"
MATRIX_CONTEXT_ARMS="${ROUTINE_EVAL_CONTEXT_ARMS:-on,off}"
MATRIX_VARIANTS="${ROUTINE_EVAL_VARIANTS:-}"
MATRIX_MODELS="${ROUTINE_EVAL_MODELS:-${ROUTINE_MODEL:-claude-opus-4-8[1m]}}"
MATRIX_EFFORTS="${ROUTINE_EVAL_EFFORTS:-high}"
MATRIX_REPLICAS="${ROUTINE_EVAL_REPLICAS:-2}"
MATRIX_BRIEFS="${ROUTINE_EVAL_BRIEFS:-}"
MATRIX_CELL_TIMEOUT_S="${ROUTINE_EVAL_CELL_TIMEOUT_S:-1200}"
MATRIX_CELL_BUDGET_USD="${ROUTINE_EVAL_CELL_BUDGET_USD:-5}"
MATRIX_CELL_TOOLS="${ROUTINE_EVAL_CELL_TOOLS:-Read,Write,Edit,Glob,Grep,Bash(ls *),Bash(mkdir *)}"
MATRIX_QUALITY_CONTEXT="${ROUTINE_EVAL_QUALITY_CONTEXT:-}"
MATRIX_CONTEXT_FILES="${ROUTINE_EVAL_CONTEXT_FILES:-}"
MATRIX_CELLS_ROOT="${ROUTINE_WT_ROOT:-$HOME/.cache/claude-routine/worktrees}/${ROUTINE_GROUP:-$(basename "$routine_dir")}-cells"
MATRIX_HOME_SKILLS="${MATRIX_HOME_SKILLS:-$HOME/.claude/skills}"
MATRIX_UNATTENDED="Unattended session: no user is present. Never ask questions or wait for input; make reasonable assumptions and record them under Open Questions."

# Sanitize to the run-id alphabet [A-Za-z0-9._-].
matrix_slug() { printf '%s' "$1" | tr -c 'A-Za-z0-9._-' '-' | sed 's/-\{2,\}/-/g; s/^-//; s/-$//'; }

# Model id → short slug: drop the claude- prefix and any [..] suffix.
matrix_model_slug() {
  local m="$1"
  m="${m#claude-}"; m="${m%%\[*}"
  matrix_slug "$m"
}

# Prints "name<TAB>disable" per configured variant (default excluded).
matrix_variants() {
  local spec item
  [[ -n "$MATRIX_VARIANTS" ]] || return 0
  spec="$MATRIX_VARIANTS;"
  while [[ -n "$spec" ]]; do
    item="${spec%%;*}"; spec="${spec#*;}"
    [[ -n "$item" ]] || continue
    printf '%s\t%s\n' "${item%%=*}" "${item#*=}"
  done
}

# Prints one brief path per line from $routine_dir/briefs/<skill>/.
matrix_briefs() {
  local dir="$routine_dir/briefs/$MATRIX_SKILL" list b
  if [[ -n "$MATRIX_BRIEFS" ]]; then
    list="$MATRIX_BRIEFS,"
    while [[ -n "$list" ]]; do
      b="${list%%,*}"; list="${list#*,}"
      [[ -n "$b" ]] || continue
      [[ "$b" == *.md ]] || b="$b.md"
      printf '%s\n' "$dir/$b"
    done
  else
    for b in "$dir"/*.md; do
      [[ -f "$b" ]] && printf '%s\n' "$b"
    done
  fi
}

# Expands the matrix into $1 (TSV: arm variant brief model effort replica).
# Returns 2 when the matrix is empty or over the cap. Prints the cell count.
routine_matrix_plan() {
  local plan="$1" arms models efforts arm model effort variant brief r n=0
  local variants_tsv briefs_list
  variants_tsv="$(printf 'default\t-\n'; matrix_variants)"
  briefs_list="$(matrix_briefs)"
  if [[ -z "$briefs_list" ]]; then
    echo "matrix: no briefs under $routine_dir/briefs/$MATRIX_SKILL" >&2
    return 2
  fi
  : > "$plan"
  arms="$MATRIX_CONTEXT_ARMS,";
  while [[ -n "$arms" ]]; do
    arm="${arms%%,*}"; arms="${arms#*,}"; [[ -n "$arm" ]] || continue
    case "$arm" in on|off) ;; *) echo "matrix: unknown context arm '$arm' (on|off)" >&2; return 2 ;; esac
    while IFS=$'\t' read -r variant _; do
      [[ -n "$variant" ]] || continue
      while IFS= read -r brief; do
        [[ -n "$brief" ]] || continue
        [[ -f "$brief" ]] || { echo "matrix: brief not found: $brief" >&2; return 2; }
        models="$MATRIX_MODELS,"
        while [[ -n "$models" ]]; do
          model="${models%%,*}"; models="${models#*,}"; [[ -n "$model" ]] || continue
          efforts="$MATRIX_EFFORTS,"
          while [[ -n "$efforts" ]]; do
            effort="${efforts%%,*}"; efforts="${efforts#*,}"; [[ -n "$effort" ]] || continue
            r=1
            while [[ "$r" -le "$MATRIX_REPLICAS" ]]; do
              printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$arm" "$variant" "$brief" "$model" "$effort" "$r" >> "$plan"
              n=$((n + 1)); r=$((r + 1))
            done
          done
        done
      done <<< "$briefs_list"
    done <<< "$variants_tsv"
  done
  if [[ "$n" -eq 0 ]]; then echo "matrix: empty" >&2; return 2; fi
  if [[ "$n" -gt "$MATRIX_MAX_CELLS" ]]; then
    echo "matrix: $n cells exceeds the cap of $MATRIX_MAX_CELLS — shrink a dimension" >&2
    return 2
  fi
  echo "$n"
}

# Copies the deployed skill (what a default cell loads) to
# <resultsDir>/skills/default.md and renders every variant into the home
# skills root as <skill>--<name>, copying each rendered SKILL.md next to it.
# $1 = resultsDir (relative to $wt). Non-zero on any render failure.
routine_matrix_render_variants() {
  local results="$1" name disable deployed out
  mkdir -p "$wt/$results/skills"
  deployed="$MATRIX_HOME_SKILLS/$MATRIX_SKILL/SKILL.md"
  if [[ ! -f "$deployed" ]]; then
    echo "matrix: skill $MATRIX_SKILL is not deployed at $deployed" >&2
    return 1
  fi
  cp "$deployed" "$wt/$results/skills/default.md"
  while IFS=$'\t' read -r name disable; do
    [[ -n "$name" ]] || continue
    out="$(bash "$repo_root/cmd/sync/sync_skills.sh" --variant "$MATRIX_SKILL=$name:$disable" --into "$MATRIX_HOME_SKILLS" </dev/null 2>&1)" || {
      echo "matrix: variant render failed for $name: $out" >&2
      return 1
    }
    cp "$MATRIX_HOME_SKILLS/$MATRIX_SKILL--$name/SKILL.md" "$wt/$results/skills/$name.md" || return 1
  done < <(matrix_variants)
}

# Cell id from its plan row.
matrix_cell_id() {
  local arm="$1" variant="$2" brief="$3" model="$4" effort="$5" r="$6"
  printf '%s.ctx-%s.%s.%s.%s.r%s' "$variant" "$arm" "$(matrix_slug "$(basename "${brief%.md}")")" \
    "$(matrix_model_slug "$model")" "$(matrix_slug "$effort")" "$r"
}

# Runs every planned cell sequentially: detached worktree off $2 (base
# commit), claude -p, commit the cell's output, locate the transcript, bundle
# changed files into <resultsDir>/runs/<id>.md, append a cells.jsonl row.
# $1 = resultsDir, $2 = base commit, $3 = plan TSV.
routine_matrix_run_cells() {
  local results="$1" base="$2" plan="$3"
  local arm variant brief model effort r id cellwt skillname prompt brief_text
  local output status sid transcript bundle files f ok
  mkdir -p "$wt/$results/runs" "$MATRIX_CELLS_ROOT"
  : > "$wt/$results/cells.jsonl"
  while IFS=$'\t' read -r arm variant brief model effort r; do
    [[ -n "$arm" ]] || continue
    id="$(matrix_cell_id "$arm" "$variant" "$brief" "$model" "$effort" "$r")"
    cellwt="$MATRIX_CELLS_ROOT/$id"
    [[ -e "$cellwt" ]] && { git -C "$repo_root" worktree remove --force "$cellwt" >/dev/null 2>&1 || rm -rf "$cellwt"; }
    if ! git -C "$repo_root" worktree add --quiet --detach "$cellwt" "$base" >/dev/null 2>&1; then
      echo "matrix: worktree add failed for $id" >&2
      jq -cn --arg id "$id" --arg arm "$arm" --arg variant "$variant" --arg brief "$brief" \
        '{id: $id, arm: $arm, variant: $variant, brief: $brief, ok: false, exit_status: 70, error: "worktree add failed"}' \
        >> "$wt/$results/cells.jsonl"
      continue
    fi
    skillname="$MATRIX_SKILL"
    [[ "$variant" != "default" ]] && skillname="$MATRIX_SKILL--$variant"
    brief_text="$(cat "$brief")"
    prompt="/$skillname"
    [[ "$arm" == "off" ]] && prompt="$prompt --no-context"
    prompt="$prompt $brief_text

$MATRIX_UNATTENDED"
    echo "matrix: cell $id"
    status=0
    output="$(cd "$cellwt" && routine_run_claude "$MATRIX_CELL_TIMEOUT_S" claude -p "$prompt" \
      --allowedTools "$MATRIX_CELL_TOOLS" \
      --model "$model" \
      --effort "$effort" \
      --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}" \
      --max-budget-usd "$MATRIX_CELL_BUDGET_USD" \
      --output-format json)" || status=$?
    git -C "$cellwt" add -A >/dev/null 2>&1
    git -C "$cellwt" -c user.email=routine@smine -c user.name=routine commit --quiet -m "cell $id" >/dev/null 2>&1 || true
    sid="$(printf '%s' "$output" | jq -r '.session_id // empty' 2>/dev/null || true)"
    transcript=""
    if [[ -n "$sid" ]]; then
      transcript="$(find "$HOME/.claude/projects" -name "$sid.jsonl" -print 2>/dev/null | head -n1)"
    fi
    files="$(git -C "$cellwt" diff --name-only "$base" HEAD 2>/dev/null)"
    bundle="$results/runs/$id.md"
    ok=false
    if [[ -n "$files" ]]; then
      ok=true
      : > "$wt/$bundle"
      while IFS= read -r f; do
        [[ -n "$f" ]] || continue
        { printf '## file: %s\n\n' "$f"; cat "$cellwt/$f" 2>/dev/null; printf '\n\n'; } >> "$wt/$bundle"
      done <<< "$files"
    fi
    printf '%s' "$output" | jq -cn \
      --arg id "$id" --arg arm "$arm" --arg variant "$variant" --arg brief "$brief" \
      --arg model "$model" --arg effort "$effort" --argjson r "$r" \
      --arg wt "$cellwt" --arg sid "$sid" --arg transcript "$transcript" \
      --argjson exit "$status" --argjson ok "$ok" --arg bundle "$bundle" \
      'input? // {} | {id: $id, arm: $arm, variant: $variant, brief: $brief, model: $model, effort: $effort, replica: $r,
        worktree: $wt, session_id: $sid, transcript: $transcript, exit_status: $exit, ok: $ok,
        is_error: (.is_error // false), subtype: (.subtype // ""), cost_usd: .total_cost_usd, num_turns: .num_turns,
        duration_ms: .duration_ms, output_tokens: (.usage.output_tokens // null), bundle: $bundle}' \
      >> "$wt/$results/cells.jsonl"
  done < "$plan"
}

# Writes <resultsDir>/manifest.json (skillroutine-eval manifest v2) from the
# surviving cells. $1 = resultsDir. Returns 1 when no cell survived.
routine_matrix_write_manifest() {
  local results="$1" source_md briefs quality ctxfiles n
  source_md=""
  for d in "$repo_root/skills/$MATRIX_SKILL" "$repo_root/skills"/*/"$MATRIX_SKILL"; do
    [[ -f "$d/SKILL.md" ]] && { source_md="${d#"$repo_root"/}/SKILL.md"; break; }
  done
  [[ -n "$source_md" ]] || source_md="$results/skills/default.md"
  briefs="$(jq -sc --arg root "$routine_dir" '[.[] | .brief | sub("^" + $root + "/"; "routines/" + ($root | split("/") | last) + "/")] | unique' "$wt/$results/cells.jsonl")"
  quality="$(jq -nc --arg s "$MATRIX_QUALITY_CONTEXT" '$s | split(",") | map(select(length > 0))')"
  ctxfiles="$(jq -nc --arg s "$MATRIX_CONTEXT_FILES" '$s | split(",") | map(select(length > 0))')"
  n="$(jq -c 'select(.ok == true)' "$wt/$results/cells.jsonl" | grep -c . || true)"
  if [[ "$n" -eq 0 ]]; then
    echo "matrix: no surviving cells — manifest not written" >&2
    return 1
  fi
  jq -sc --arg skill "$MATRIX_SKILL" --arg skillMd "$source_md" --arg results "$results" --arg variants "$MATRIX_VARIANTS" \
    --argjson inputs "$briefs" --argjson quality "$quality" --argjson ctxfiles "$ctxfiles" '
    def disable_of($v): $variants | split(";") | map(select(startswith($v + "="))) | .[0] // "" | sub("^[^=]*="; "") | split(",") | map(select(length > 0));
    {
      schemaVersion: "2.0",
      skill: $skill,
      skillMd: $skillMd,
      contextFiles: $ctxfiles,
      qualityContext: $quality,
      inputs: $inputs,
      runs: [ .[] | select(.ok == true) | {
        id: .id,
        output: .bundle,
        skillMd: ($results + "/skills/" + .variant + ".md"),
        model: ({id: .model, effort: .effort, mode: ("context-" + .arm),
                 telemetry: ("wall_s=" + ((.duration_ms // 0) / 1000 | floor | tostring)
                            + " output_tokens=" + ((.output_tokens // "null") | tostring)
                            + " cost_usd=" + ((.cost_usd // "null") | tostring)
                            + " turns=" + ((.num_turns // "null") | tostring))})
        }
        + (if .variant != "default" then {variant: {name: .variant, disable: disable_of(.variant)}} else {} end)
        + (if (.transcript // "") != "" then {transcript: .transcript} else {} end)
        + {worktree: .worktree}
      ],
      output: ($results + "/eval.json"),
      md: true
    }' "$wt/$results/cells.jsonl" > "$wt/$results/manifest.json"
  # Every referenced file must exist relative to $wt.
  local missing=0 p
  while IFS= read -r p; do
    [[ -f "$wt/$p" ]] || { echo "matrix: manifest references missing file $p" >&2; missing=$((missing + 1)); }
  done < <(jq -r '.runs[] | .output, .skillMd' "$wt/$results/manifest.json")
  [[ "$missing" -eq 0 ]]
}

# Computes per-axis A/B deltas from eval.json + cells.jsonl into
# <resultsDir>/deltas.json and prints them compactly. $1 = resultsDir.
routine_matrix_deltas() {
  local results="$1" eval_json="$wt/$1/eval.json"
  [[ -s "$eval_json" ]] || { echo "[]" > "$wt/$results/deltas.json"; echo "[]"; return 1; }
  jq -c --slurpfile cells <(jq -c '.' "$wt/$results/cells.jsonl") '
    def rows: (if (.sharedTotals // []) | length > 0 then .sharedTotals else (.totals // []) end);
    def mean: if length == 0 then null else (add / length) end;
    ($cells | map({key: .id, value: .}) | from_entries) as $c
    | [ rows[] | . as $t | ($c[$t.runId] // {}) as $cell
        | {axis: $t.axis, pct: $t.pct, arm: ($cell.arm // "on"), variant: ($cell.variant // "default")} ] as $r
    | ([$r[].axis] | unique) as $axes
    | [ $axes[] as $ax
        | ( ([$r[] | select(.axis == $ax and .arm == "on") | .pct] | mean) as $on
          | [$r[] | select(.axis == $ax) | .arm] | unique | .[] | select(. == "off") as $arm
          | {axis: $ax, dimension: "context", arm: $arm,
             delta_pct: (if $on == null then null else (([$r[] | select(.axis == $ax and .arm == $arm) | .pct] | mean) - $on) end),
             n: ([$r[] | select(.axis == $ax and .arm == $arm)] | length)} ),
          ( ([$r[] | select(.axis == $ax and .variant == "default") | .pct] | mean) as $def
          | [$r[] | select(.axis == $ax) | .variant] | unique | .[] | select(. != "default") as $v
          | {axis: $ax, dimension: "variant", arm: $v,
             delta_pct: (if $def == null then null else (([$r[] | select(.axis == $ax and .variant == $v) | .pct] | mean) - $def) end),
             n: ([$r[] | select(.axis == $ax and .variant == $v)] | length)} )
      ]
    | map(.delta_pct |= (if . == null then null else (. * 10 | round / 10) end))
  ' "$eval_json" > "$wt/$results/deltas.json"
  cat "$wt/$results/deltas.json"
}

# Removes every cell worktree and every rendered variant dir. Never fails.
routine_matrix_cleanup() {
  local d name _
  if [[ -d "$MATRIX_CELLS_ROOT" ]]; then
    for d in "$MATRIX_CELLS_ROOT"/*/; do
      [[ -d "$d" ]] || continue
      git -C "$repo_root" worktree remove --force "${d%/}" >/dev/null 2>&1 || rm -rf "${d%/}"
    done
    rmdir "$MATRIX_CELLS_ROOT" 2>/dev/null || true
    git -C "$repo_root" worktree prune >/dev/null 2>&1 || true
  fi
  while IFS=$'\t' read -r name _; do
    [[ -n "$name" ]] || continue
    d="$MATRIX_HOME_SKILLS/$MATRIX_SKILL--$name"
    [[ -d "$d" && ! -L "$d" ]] && rm -rf "$d"
  done < <(matrix_variants)
  return 0
}
