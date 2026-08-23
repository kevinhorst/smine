#!/usr/bin/env bash
# Coverage for routines/_lib/matrix.sh (headless A/B cell runner): matrix
# expansion and cap; --no-context only on the off arm; variant cells invoke
# <skill>--<name>; cells.jsonl + manifest.json shape (mode, variant,
# transcript, telemetry); an empty-output cell is excluded from the manifest;
# deltas from a canned eval.json; cleanup removes cell worktrees and rendered
# variant dirs. Uses a stub `claude` and a stub `rules`; git + jq only.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/routine-matrix.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == routine-matrix.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

# ---- sandbox repo: routines/_lib + cmd/sync + a demo skill + a brief ----
sandbox="$TMP/repo"
mkdir -p "$sandbox/routines/_lib" "$sandbox/routines/skill-eval/briefs/demo" "$sandbox/cmd/sync" \
  "$sandbox/skills/group/demo" "$sandbox/bin" "$sandbox/cmd/worktrees/_lib"
cp "$REPO_DIR/routines/_lib/matrix.sh" "$REPO_DIR/routines/_lib/platform.sh" "$sandbox/routines/_lib/"
cp "$REPO_DIR/cmd/sync/sync_skills.sh" "$REPO_DIR/cmd/sync/smine_tool.sh" "$sandbox/cmd/sync/"
printf "#!/usr/bin/env bash\n" > "$sandbox/cmd/worktrees/remove_agent_worktrees.sh"
printf "# lib\n" > "$sandbox/cmd/worktrees/_lib/verdict.sh"
cat > "$sandbox/bin/rules" <<EOF
#!/usr/bin/env bash
exec go run -C "$REPO_DIR" ./cmd/rules "\$@"
EOF
chmod +x "$sandbox/bin/rules"
cat > "$sandbox/skills/group/demo/SKILL.md" <<'EOF'
---
name: demo
description: demo. Trigger on /demo.
author: Kevin Horst
version: 1.0
acdsl-context: ACTION-DEMO-*
---
# Demo

## Intake

**SKILL-DEMO-INTAKE-001** `[step]` — Ask for the feature name.

## Closing

**SKILL-DEMO-CLOSING-001** `[step]` — Offer to continue.
EOF
printf 'Feature name: probe.\nProblem: none.\n' > "$sandbox/routines/skill-eval/briefs/demo/probe.md"

git -C "$sandbox" init -q
git -C "$sandbox" config user.email test@example.com
git -C "$sandbox" config user.name test
git -C "$sandbox" checkout -qb main
git -C "$sandbox" add -A
git -C "$sandbox" commit -qm base

# ---- fake HOME: deployed demo skill, hooks env, projects dir ----
HOME_DIR="$TMP/home"
mkdir -p "$HOME_DIR/.claude/skills/demo" "$HOME_DIR/.claude/hooks" "$HOME_DIR/.claude/projects/x"
cp "$sandbox/skills/group/demo/SKILL.md" "$HOME_DIR/.claude/skills/demo/SKILL.md"
printf 'AGENT_CONTEXT_DIR_DEFAULT=ctx\n' > "$HOME_DIR/.claude/hooks/global-context.env"

# ---- stub claude: records the prompt, writes one plans file unless the
# prompt says "empty", writes a fake transcript, prints an envelope ----
mkdir -p "$TMP/stubbin"
cat > "$TMP/stubbin/claude" <<'EOF'
#!/usr/bin/env bash
prompt=""; model=""; effort=""
while [ $# -gt 0 ]; do
  case "$1" in
    -p) prompt="$2"; shift ;;
    --model) model="$2"; shift ;;
    --effort) effort="$2"; shift ;;
    --allowedTools|--permission-mode|--max-budget-usd|--output-format) shift ;;
  esac
  shift
done
printf '%s\n' "$prompt" >> "$STUB_LOG"
sid="$(printf 'sid-%s' "$(wc -l < "$STUB_LOG" | tr -d ' ')")"
case "$prompt" in
  *empty*) ;;
  *) mkdir -p plans/probe/concept; printf '# concept\n%s\n' "${prompt%%$'\n'*}" > plans/probe/concept/concept.md ;;
esac
printf '{"type":"user","cwd":"%s","sessionId":"%s"}\n' "$PWD" "$sid" > "$HOME/.claude/projects/x/$sid.jsonl"
printf '{"session_id":"%s","duration_ms":4200,"total_cost_usd":0.5,"num_turns":3,"subtype":"success","is_error":false,"usage":{"output_tokens":123},"result":"ok"}' "$sid"
EOF
chmod +x "$TMP/stubbin/claude"
# caffeinate is not on every PATH (CI); a no-op stub keeps routine_run_claude honest.
printf '#!/usr/bin/env bash\nshift; exec "$@"\n' > "$TMP/stubbin/caffeinate"
chmod +x "$TMP/stubbin/caffeinate"

# routine worktree = a second worktree of the sandbox at HEAD (what run.sh
# gets from routine_worktree_create)
WT="$TMP/wt"
git -C "$sandbox" worktree add --quiet -b claude-routines/skill-eval-test "$WT" main
BASE="$(git -C "$WT" rev-parse HEAD)"

# Runs $@ inside a subshell with the lib sourced and the sandbox env.
run_lib() {
  ( export HOME="$HOME_DIR" PATH="$TMP/stubbin:$PATH" STUB_LOG="$TMP/stub.log" \
      ROUTINE_WT_ROOT="$TMP/wtroot" ROUTINE_GROUP="skill-eval" \
      ROUTINE_EVAL_SKILL=demo ROUTINE_EVAL_MODELS="claude-test-1[1m]" ROUTINE_EVAL_EFFORTS=low \
      ROUTINE_EVAL_REPLICAS="${REPLICAS:-1}" ROUTINE_EVAL_CONTEXT_ARMS="${ARMS:-on,off}" \
      ROUTINE_EVAL_VARIANTS="${VARIANTS-no-closing=SKILL-DEMO-CLOSING-*}"
    repo_root="$sandbox"; routine_dir="$sandbox/routines/skill-eval"; wt="$WT"
    source "$sandbox/routines/_lib/platform.sh"
    source "$sandbox/routines/_lib/matrix.sh"
    "$@" )
}

# ---- case: plan expansion and cap ----
test_plan() {
  n="$(run_lib routine_matrix_plan "$TMP/plan.tsv")" || fail "plan refused: $n"
  [ "$n" -eq 4 ] || fail "expected 4 cells (2 arms x 2 variants x 1 replica), got $n"
  [ "$(wc -l < "$TMP/plan.tsv" | tr -d ' ')" -eq 4 ] || fail "plan tsv rows != 4"
  grep -q $'^off\tno-closing\t' "$TMP/plan.tsv" || fail "off/no-closing row missing"
  if REPLICAS=5 run_lib routine_matrix_plan "$TMP/plan-cap.tsv" >/dev/null 2>"$TMP/cap.err"; then
    fail "20-cell matrix accepted"
  fi
  grep -q "exceeds the cap" "$TMP/cap.err" || fail "cap refusal message missing: $(cat "$TMP/cap.err")"
  if ARMS=sideways run_lib routine_matrix_plan "$TMP/plan-arm.tsv" >/dev/null 2>&1; then
    fail "unknown arm accepted"
  fi
}

# ---- case: render variants (default copy + rendered variant) ----
test_render() {
  run_lib routine_matrix_render_variants sessions/eval >/dev/null || fail "render failed"
  [ -f "$WT/sessions/eval/skills/default.md" ] || fail "default.md missing"
  [ -f "$WT/sessions/eval/skills/no-closing.md" ] || fail "no-closing.md missing"
  grep -q 'SKILL-DEMO-INTAKE-001' "$WT/sessions/eval/skills/no-closing.md" || fail "kept entry missing in variant"
  ! grep -q 'SKILL-DEMO-CLOSING-001' "$WT/sessions/eval/skills/no-closing.md" || fail "disabled entry present in variant"
  [ -f "$HOME_DIR/.claude/skills/demo--no-closing/SKILL.md" ] || fail "variant not installed in home skills"
}

# ---- case: run cells → cells.jsonl, bundles, prompts ----
test_cells() {
  : > "$TMP/stub.log"
  run_lib routine_matrix_run_cells sessions/eval "$BASE" "$TMP/plan.tsv" >/dev/null || fail "run_cells failed"
  cells="$WT/sessions/eval/cells.jsonl"
  [ "$(wc -l < "$cells" | tr -d ' ')" -eq 4 ] || fail "cells.jsonl rows != 4"
  [ "$(jq -s 'map(select(.ok == true)) | length' "$cells")" -eq 4 ] || fail "not all cells ok: $(cat "$cells")"
  # prompts: default vs variant skill name, --no-context only on off arm
  grep -q '^/demo Feature name' "$TMP/stub.log" || fail "default on-arm prompt missing"
  grep -q '^/demo --no-context Feature name' "$TMP/stub.log" || fail "default off-arm prompt missing"
  grep -q '^/demo--no-closing Feature name' "$TMP/stub.log" || fail "variant on-arm prompt missing"
  grep -q '^/demo--no-closing --no-context Feature name' "$TMP/stub.log" || fail "variant off-arm prompt missing"
  [ "$(grep -c -- '--no-context' "$TMP/stub.log")" -eq 2 ] || fail "--no-context count != 2"
  grep -q 'Unattended session' "$TMP/stub.log" || fail "unattended sentence missing"
  # per-cell fields
  jq -e '.transcript | test("/\\.claude/projects/x/sid-[0-9]+\\.jsonl$")' <(head -n1 "$cells") >/dev/null || fail "transcript not located"
  jq -e '.output_tokens == 123 and .cost_usd == 0.5 and .duration_ms == 4200 and .model == "claude-test-1[1m]" and .effort == "low"' <(head -n1 "$cells") >/dev/null || fail "telemetry fields wrong"
  id="$(jq -r '.id' <(head -n1 "$cells"))"
  [ "$id" = "default.ctx-on.probe.test-1.low.r1" ] || fail "unexpected cell id $id"
  [ -f "$WT/sessions/eval/runs/$id.md" ] || fail "bundle missing for $id"
  grep -q '^## file: plans/probe/concept/concept.md' "$WT/sessions/eval/runs/$id.md" || fail "bundle header missing"
  # cell worktree exists, committed detached
  cwt="$(jq -r '.worktree' <(head -n1 "$cells"))"
  [ -d "$cwt" ] || fail "cell worktree missing"
  [ -n "$(git -C "$cwt" diff --name-only "$BASE" HEAD)" ] || fail "cell output not committed"
}

# ---- case: manifest shape ----
test_manifest() {
  run_lib routine_matrix_write_manifest sessions/eval || fail "manifest write failed"
  m="$WT/sessions/eval/manifest.json"
  jq -e '.schemaVersion == "2.0" and .skill == "demo" and .skillMd == "skills/group/demo/SKILL.md" and (.runs | length) == 4 and .md == true and .output == "sessions/eval/eval.json"' "$m" >/dev/null || fail "manifest top-level wrong: $(cat "$m")"
  jq -e '.inputs == ["routines/skill-eval/briefs/demo/probe.md"]' "$m" >/dev/null || fail "inputs wrong"
  jq -e '[.runs[] | .model.mode] | sort == ["context-off","context-off","context-on","context-on"]' "$m" >/dev/null || fail "modes wrong"
  jq -e '[.runs[] | select(.variant) | .variant] | length == 2 and all(.name == "no-closing" and .disable == ["SKILL-DEMO-CLOSING-*"])' "$m" >/dev/null || fail "variant blocks wrong"
  jq -e '.runs[] | select(.id | startswith("default.")) | has("variant") | not' "$m" >/dev/null || fail "default run carries variant"
  jq -e '.runs[0].skillMd == "sessions/eval/skills/default.md" and (.runs[0].model.telemetry | test("wall_s=4 output_tokens=123 cost_usd=0.5 turns=3"))' "$m" >/dev/null || fail "run skillMd/telemetry wrong: $(jq -c '.runs[0]' "$m")"
  jq -e '.runs[] | .transcript | test("sid-[0-9]+\\.jsonl$")' "$m" >/dev/null || fail "run transcript missing"
  # additionalProperties false in the manifest schema: only known keys
  jq -e '[.runs[] | keys[]] | unique - ["id","output","skillMd","transcript","worktree","variant","model"] | length == 0' "$m" >/dev/null || fail "unknown run keys"
  jq -e '[.runs[] | .model | keys[]] | unique - ["id","effort","mode","telemetry"] | length == 0' "$m" >/dev/null || fail "unknown model keys"
}

# ---- case: a cell with no output is excluded from the manifest ----
test_empty_cell_excluded() {
  printf 'Feature name: empty run.\n' > "$sandbox/routines/skill-eval/briefs/demo/empty.md"
  ARMS=on VARIANTS='' run_lib routine_matrix_plan "$TMP/plan2.tsv" >/dev/null || fail "plan2 refused"
  [ "$(wc -l < "$TMP/plan2.tsv" | tr -d ' ')" -eq 2 ] || fail "plan2 rows != 2"
  ARMS=on VARIANTS='' run_lib routine_matrix_render_variants sessions/eval2 >/dev/null || fail "render2 failed"
  ARMS=on VARIANTS='' run_lib routine_matrix_run_cells sessions/eval2 "$BASE" "$TMP/plan2.tsv" >/dev/null || fail "run_cells2 failed"
  [ "$(jq -s 'map(select(.ok == true)) | length' "$WT/sessions/eval2/cells.jsonl")" -eq 1 ] || fail "expected exactly one ok cell"
  ARMS=on VARIANTS='' run_lib routine_matrix_write_manifest sessions/eval2 || fail "manifest2 failed"
  [ "$(jq '.runs | length' "$WT/sessions/eval2/manifest.json")" -eq 1 ] || fail "empty cell leaked into manifest"
  rm -f "$sandbox/routines/skill-eval/briefs/demo/empty.md"
}

# ---- case: deltas from a canned eval.json ----
test_deltas() {
  cells="$WT/sessions/eval/cells.jsonl"
  jq -sc '{schemaVersion: "2.0", totals: [ .[] | {runId: .id, axis: "self", raw: 1, max: 2,
      pct: ((if .arm == "on" then 80 else 60 end) + (if .variant == "default" then 0 else 5 end))} ]}' "$cells" > "$WT/sessions/eval/eval.json"
  out="$(run_lib routine_matrix_deltas sessions/eval)" || fail "deltas failed"
  [ -f "$WT/sessions/eval/deltas.json" ] || fail "deltas.json missing"
  jq -e 'map(select(.dimension == "context" and .arm == "off"))[0] | .axis == "self" and .delta_pct == -20 and .n == 2' <<<"$out" >/dev/null || fail "context delta wrong: $out"
  jq -e 'map(select(.dimension == "variant" and .arm == "no-closing"))[0] | .delta_pct == 5 and .n == 2' <<<"$out" >/dev/null || fail "variant delta wrong: $out"
  # sharedTotals wins over totals when present
  jq '. + {sharedTotals: (.totals | map(.pct = 50))}' "$WT/sessions/eval/eval.json" > "$WT/sessions/eval/eval2.json" && mv "$WT/sessions/eval/eval2.json" "$WT/sessions/eval/eval.json"
  out="$(run_lib routine_matrix_deltas sessions/eval)" || fail "deltas (shared) failed"
  jq -e 'all(.delta_pct == 0)' <<<"$out" >/dev/null || fail "sharedTotals not preferred: $out"
}

# ---- case: cleanup ----
test_cleanup() {
  run_lib routine_matrix_cleanup || fail "cleanup returned non-zero"
  [ ! -d "$TMP/wtroot/skill-eval-cells" ] || fail "cells root still present: $(ls "$TMP/wtroot/skill-eval-cells")"
  [ ! -d "$HOME_DIR/.claude/skills/demo--no-closing" ] || fail "variant dir still installed"
  [ -f "$HOME_DIR/.claude/skills/demo/SKILL.md" ] || fail "real skill removed by cleanup"
  ! git -C "$sandbox" worktree list | grep -q "skill-eval-cells" || fail "cell worktree still registered"
  [ -d "$WT" ] || fail "routine worktree removed by cleanup"
}

test_plan
test_render
test_cells
test_manifest
test_empty_cell_excluded
test_deltas
test_cleanup
echo "test_routine_matrix: ok"
