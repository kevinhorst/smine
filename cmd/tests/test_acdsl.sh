#!/usr/bin/env bash
# Smoke: ACDSL — gate, on-disk projection, strip, staged-leak guard, fixtures,
# hook, untracked-universe. Verbose by design: every asserted output is printed
# to the console first; the projected file is shown straight from the worktree.
set -euo pipefail
cd "$(dirname "$0")/../.."

# Compiled runner + verifier binaries: every go-run spawn pays the toolchain's
# cache check and link, which used to dominate this suite's runtime. The
# registry substitutes bin/verifiers/* automatically when the binaries exist.
ACDSL=./bin/acdsl
go build -o "$ACDSL" ./cmd/acdsl
mkdir -p ./bin/verifiers
go build -o ./bin/verifiers ./cmd/acdsl/verifiers/...

# Smoke runs include deliberate reds — never pollute the real verdict sink.
export ACDSL_VERDICTS_ENABLED=0

MARKER='[ACDSL-''PROJECTION]'
TARGET=internal/routines/launchctl_darwin.go

cleanup() {
	git restore --staged "$TARGET" plans/agentic_context_dsl/reviews/smoke_prose.md plans/agentic_context_dsl/reviews/smoke_htmlblock.md 2>/dev/null || true
	rm -rf internal/xptask xptask_smoke.acdsl xptask_smoke.acdsl.off internal/gateonlysmoke plans/agentic_context_dsl/reviews/smoke_prose.md plans/agentic_context_dsl/reviews/smoke_htmlblock.md
	rm -f proposals/workflows.json
	rmdir proposals 2>/dev/null || true
	"$ACDSL" project -strip > /dev/null 2>&1 || true
}
trap cleanup EXIT

banner() {
	echo
	echo "########## acdsl smoke: $1"
}

# The working tree legitimately carries projections (any agent Read projects
# on disk); normalize first so the strip-count assertion below sees only the
# projection this script creates.
"$ACDSL" project -strip > /dev/null

banner "check on real tree (expect: all doctrine rules OK)"
check_out=$("$ACDSL" check)
echo "$check_out"
doctrine=$(sed -n 's/^acdsl: \([0-9][0-9]*\) rule(s) OK$/\1/p' <<<"$check_out")
[ -n "$doctrine" ]

banner "fixtures prove all rules with examples (expect: every testdata dir checked or needs-skipped)"
fixtures_out=$("$ACDSL" fixtures)
echo "$fixtures_out"
# Rules whose needs= artifacts are absent (a public tree without the private
# context/proposals files) skip their fixtures; checked + skipped must still
# cover every committed example dir.
total_fixture_dirs=$(find acdsl/testdata -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
checked=$(sed -n 's/.*fixtures OK (\([0-9][0-9]*\) rule(s) with examples.*/\1/p' <<<"$fixtures_out")
skipped=$(sed -n 's/.* \([0-9][0-9]*\) skipped (needs).*/\1/p' <<<"$fixtures_out")
skipped=${skipped:-0}
[ "$((checked + skipped))" -eq "$total_fixture_dirs" ]

banner "project $TARGET in place (expect: projected, 5 rules)"
project_out=$("$ACDSL" project -file "$TARGET")
echo "$project_out"
grep -q "projected $TARGET (5 rule(s))" <<<"$project_out"

banner "the projected file on disk, head (view == disk)"
head -6 "$TARGET"
head -1 "$TARGET" | grep -qF "$MARKER"
grep -q "ACDSL-GOLANG-EXEC-001" "$TARGET"

banner "dirty set is the touched-files trace (expect: $TARGET modified)"
git status --short "$TARGET"
git status --short "$TARGET" | grep -q "^ M"

banner "projection is idempotent (expect: already current)"
idem_out=$("$ACDSL" project -file "$TARGET")
echo "$idem_out"
grep -q "already current" <<<"$idem_out"

banner "staged projection is refused by the gate (expect: red)"
git add "$TARGET"
if staged_out=$("$ACDSL" check 2>&1); then
	echo "FAIL: staged projection block not refused"
	exit 1
fi
echo "$staged_out"
grep -q "projection content staged" <<<"$staged_out"
git restore --staged "$TARGET"

banner "staged doc quoting the marker is NOT refused (expect: green)"
PROSE=plans/agentic_context_dsl/reviews/smoke_prose.md
mkdir -p "$(dirname "$PROSE")"
printf '# Smoke\nThe projection marker is %s, quoted in prose.\n' "$MARKER" > "$PROSE"
git add "$PROSE"
"$ACDSL" check
echo "PASS: prose doc quoting the marker did not trip the guard"
git restore --staged "$PROSE"
rm -f "$PROSE"

banner "multi-syntax: shell keeps its shebang on line 1 (expect: block at line 2)"
SH_TARGET=cmd/hooks/acdsl-context.sh
sh_out=$("$ACDSL" project -file "$SH_TARGET")
echo "$sh_out"
grep -q "projected $SH_TARGET" <<<"$sh_out"
head -2 "$SH_TARGET"
head -1 "$SH_TARGET" | grep -q '^#!'
sed -n 2p "$SH_TARGET" | grep -qF "# $MARKER"

banner "multi-syntax: SKILL.md keeps its frontmatter on top (expect: block after the fence)"
SKILL_TARGET=skills/no-human/jq/SKILL.md
skill_out=$("$ACDSL" project -file "$SKILL_TARGET")
echo "$skill_out"
grep -q "projected $SKILL_TARGET" <<<"$skill_out"
head -1 "$SKILL_TARGET" | grep -qx -- '---'
fence_end=$(awk 'NR>1 && $0=="---" {print NR; exit}' "$SKILL_TARGET")
sed -n "$((fence_end + 1))p" "$SKILL_TARGET" | grep -qF "<!-- $MARKER"

banner "staged HTML-shaped block in a .md is refused (expect: red)"
HTMLBLOCK=plans/agentic_context_dsl/reviews/smoke_htmlblock.md
mkdir -p "$(dirname "$HTMLBLOCK")"
printf '<!-- %s 1 rule(s) govern this file — working-copy view, stripped before commit -->\n# Smoke\n' "$MARKER" > "$HTMLBLOCK"
git add "$HTMLBLOCK"
if staged_md=$("$ACDSL" check 2>&1); then
	echo "FAIL: staged .md projection block not refused"
	exit 1
fi
echo "$staged_md"
grep -q "smoke_htmlblock.md" <<<"$staged_md"
git restore --staged "$HTMLBLOCK"
rm -f "$HTMLBLOCK"

banner "context delivery for comment-incapable files (expect: JSON-001; empty for projectable)"
# A scratch file matching JSON-001's anchor: proposals/context.json is a
# private artifact (absent on a public tree), workflows.json exists in
# neither tree — the assertion behaves identically everywhere.
JSON_TARGET=proposals/workflows.json
mkdir -p proposals
# kind "skills": the schema's kind enum has no "workflows" member yet, and an
# invalid scratch file would trip JSON-001 in the private tree's later checks.
printf '{"kind": "skills", "source": "proposals/workflows.json", "updated": "2026-01-01", "groups": []}\n' > "$JSON_TARGET"
ctx_out=$("$ACDSL" project -context "$JSON_TARGET")
echo "$ctx_out"
grep -q "ACDSL-JSON-001" <<<"$ctx_out"
[ -z "$("$ACDSL" project -context internal/acdsl/project.go)" ]
echo "-> projectable file yields no context output: OK"

banner "acdsl-context hook emits additionalContext (expect: rules in hook JSON)"
hook_ctx=$(printf '{"tool_input":{"file_path":"%s/%s"},"cwd":"%s"}' "$PWD" "$JSON_TARGET" "$PWD" | ACDSL_CONTEXT_ENABLED=1 bash cmd/hooks/acdsl-context.sh)
echo "$hook_ctx"
grep -q '"additionalContext"' <<<"$hook_ctx"
grep -q "ACDSL-JSON-001" <<<"$hook_ctx"
hook_none=$(printf '{"tool_input":{"file_path":"%s/README.md"},"cwd":"%s"}' "$PWD" "$PWD" | ACDSL_CONTEXT_ENABLED=1 bash cmd/hooks/acdsl-context.sh)
[ -z "$hook_none" ]
hook_off=$(printf '{"tool_input":{"file_path":"%s/%s"},"cwd":"%s"}' "$PWD" "$JSON_TARGET" "$PWD" | ACDSL_CONTEXT_ENABLED=0 bash cmd/hooks/acdsl-context.sh)
[ -z "$hook_off" ]
echo "-> projectable/kill-switch paths silent: OK"

banner "strip restores original bytes (expect: 3 files, clean diff)"
strip_out=$("$ACDSL" project -strip)
echo "$strip_out"
grep -q "stripped 3 file(s)" <<<"$strip_out"
git diff --quiet "$TARGET" "$SH_TARGET" "$SKILL_TARGET"
echo "-> git diff clean: OK"

banner "plan-time resolution for a future file (expect: EXEC-001 would govern)"
plan_out=$("$ACDSL" project -plan internal/brandnew/controller.go)
echo "$plan_out"
grep -q "ACDSL-GOLANG-EXEC-001" <<<"$plan_out"
plan_none=$("$ACDSL" project -plan docs/note.md)
echo "$plan_none"
grep -q "no rules would govern" <<<"$plan_none"

banner "ungoverned file README.md (expect: no rules, untouched)"
# Content snapshot, not `git diff --quiet`: legit uncommitted README edits in
# the working tree must not fail the untouched assertion.
readme_before=$(cksum README.md)
ungov_out=$("$ACDSL" project -file README.md)
echo "$ungov_out"
grep -q "no rules govern README.md" <<<"$ungov_out"
[ "$(cksum README.md)" = "$readme_before" ]

banner "untracked violation is caught without git add (expect: red, EXEC-001)"
cat > internal/acdsl_smoke_untracked.go <<'GOEOF'
package internal

import "os/exec"

func smokeViolation() {
	_ = exec.Command("ls")
}
GOEOF
check_out=$("$ACDSL" check 2>&1) && {
	rm internal/acdsl_smoke_untracked.go
	echo "FAIL: untracked violation not caught"
	exit 1
}
rm internal/acdsl_smoke_untracked.go
echo "$check_out"
grep -q "ACDSL-GOLANG-EXEC-001" <<<"$check_out"

banner "hook projects on read (expect: block on disk after PreToolUse JSON)"
printf '{"tool_input":{"file_path":"%s/%s"},"cwd":"%s"}' "$PWD" "$TARGET" "$PWD" | ACDSL_PROJECT_ENABLED=1 bash cmd/hooks/acdsl-project.sh
head -1 "$TARGET"
head -1 "$TARGET" | grep -qF "$MARKER"
"$ACDSL" project -strip > /dev/null

banner "hook honors the kill switch (expect: file untouched)"
printf '{"tool_input":{"file_path":"%s/%s"},"cwd":"%s"}' "$PWD" "$TARGET" "$PWD" | ACDSL_PROJECT_ENABLED=0 bash cmd/hooks/acdsl-project.sh
git diff --quiet "$TARGET"
echo "-> untouched: OK"

banner "hook ignores ungoverned files (expect: README.md untouched)"
readme_before=$(cksum README.md)
printf '{"tool_input":{"file_path":"%s/README.md"},"cwd":"%s"}' "$PWD" "$PWD" | ACDSL_PROJECT_ENABLED=1 bash cmd/hooks/acdsl-project.sh
[ "$(cksum README.md)" = "$readme_before" ]
echo "-> untouched: OK"

banner "gate-only rule: gate fires, prompt side silent, verdict logged"
VERDICTS_TMP=$(mktemp)
mkdir -p internal/gateonlysmoke
cat > internal/gateonlysmoke/gateonly.go <<'GOEOF'
package gateonlysmoke

//acdsl:ACDSL-SMOKE-900 forbid-addr-lit anchor="^internal/gateonlysmoke/.*\.go$" projected="false" why="smoke: gate-only delivery flag"

type cfg struct{ n int }

func smokeAddr() *cfg { return &cfg{n: 1} }
GOEOF
# Five baseline rules (EXEC/STATE/FMT/FUNC/ENUM) anchor every internal .go
# file, so purity is asserted relative to the gate-only rule: SMOKE-900
# absent from both channels, projected rules kept.
plan_gateonly=$("$ACDSL" project -plan internal/gateonlysmoke/other.go)
echo "$plan_gateonly"
! grep -q "ACDSL-SMOKE-900" <<<"$plan_gateonly"
file_gateonly=$("$ACDSL" project -file internal/gateonlysmoke/gateonly.go)
echo "$file_gateonly"
grep -q "projected internal/gateonlysmoke/gateonly.go (5 rule(s))" <<<"$file_gateonly"
head -4 internal/gateonlysmoke/gateonly.go
! grep -qF -- "- [ACDSL-SMOKE-900]" internal/gateonlysmoke/gateonly.go
if check_gateonly=$(ACDSL_VERDICTS_ENABLED=1 ACDSL_VERDICTS_PATH="$VERDICTS_TMP" "$ACDSL" check 2>&1); then
	echo "FAIL: gate-only violation not caught"
	exit 1
fi
echo "$check_gateonly"
grep -q "ACDSL-SMOKE-900" <<<"$check_gateonly"
grep -q '"id":"ACDSL-SMOKE-900","projected":false' "$VERDICTS_TMP"
verdicts_out=$(ACDSL_VERDICTS_PATH="$VERDICTS_TMP" "$ACDSL" verdicts)
echo "$verdicts_out"
grep -q "ACDSL-SMOKE-900" <<<"$verdicts_out"
rm -rf internal/gateonlysmoke "$VERDICTS_TMP"

banner "task contract lifecycle: drop an .acdsl file anywhere (expect: default gate unaffected)"
# Live task contracts (an active plan's contract.acdsl) shift the counts;
# assert relative to the pre-drop baseline instead of absolute numbers.
base_task=$("$ACDSL" check -lifetime task 2>/dev/null | grep -o '[0-9]* rule(s) OK' | grep -o '^[0-9]*' || echo 0)
base_task=${base_task:-0}
printf '%s\n' '//acdsl:TASK-XP-001 symbol-exists anchor="^internal/xptask/x\.go$" lifetime="task" symbol="Ping" why="smoke: planned symbol"' > xptask_smoke.acdsl
"$ACDSL" check | grep -q "$doctrine rule(s) OK"
echo "-> default check still $doctrine rules: OK"

banner "task gate red while planned artifact missing (expect: not yet present)"
if task_out=$("$ACDSL" check -lifetime task 2>&1); then
	echo "FAIL: missing planned artifact not flagged"
	exit 1
fi
echo "$task_out"
grep -q "planned artifact not yet present" <<<"$task_out"

banner "task gate green once the planned symbol lands (expect: baseline+1 rules OK)"
mkdir -p internal/xptask
printf 'package xptask\n\nfunc Ping() {}\n' > internal/xptask/x.go
task_green=$("$ACDSL" check -lifetime task)
echo "$task_green"
grep -q "$((base_task + 1)) rule(s) OK" <<<"$task_green"

banner "lifetime all gates doctrine + task together (expect: doctrine+baseline+1 rules OK)"
all_out=$("$ACDSL" check -lifetime all)
echo "$all_out"
grep -q "$((doctrine + base_task + 1)) rule(s) OK" <<<"$all_out"

banner "retire by rename: file no longer ends in .acdsl (expect: baseline task rules)"
mv xptask_smoke.acdsl xptask_smoke.acdsl.off
archived_out=$("$ACDSL" check -lifetime task)
echo "$archived_out"
grep -q "$base_task rule(s) OK" <<<"$archived_out"
rm -rf xptask_smoke.acdsl.off internal/xptask

echo
echo "########## acdsl smoke: OK"
