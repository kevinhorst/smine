#!/usr/bin/env bash
# Regression coverage for sync_context.sh flag-driven (non-interactive) mode
# and the context builder behavior (ownership split, deploy settings, validation).
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)

# The sync pipeline reads the generated context file for its aspect taxonomy;
# a tree without it (a public clone, private context artifacts excluded)
# cannot exercise the pipeline — same skip as the acdsl needs= gate.
if [ ! -f "$REPO_DIR/context/context.json" ]; then
  echo "skip: context/context.json absent — context pipeline not materialized"
  exit 0
fi

TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/sync-context.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == sync-context.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_exists() {
  [ -e "$1" ] || fail "expected $1 to exist"
}

assert_missing() {
  [ ! -e "$1" ] || fail "expected $1 to be absent"
}

assert_contains() {
  grep -qF "$2" "$1" || fail "expected $1 to contain: $2"
}

assert_not_contains() {
  ! grep -qF "$2" "$1" || fail "expected $1 to not contain: $2"
}

# Fixture: script copy + controlled context source (REPO_DIR derives from the
# script location, so the copy reads $FIXTURE/context). The fixture has no Go
# module — filter/generate run against the real module via the override, so
# the mandatory reach/lang filtering is exercised, never skipped.
export SYNC_CONTEXT_RULES_REPO="$REPO_DIR"
FIXTURE=$TMP/fixture
SCRIPT=$FIXTURE/cmd/sync/sync_context.sh
mkdir -p "$FIXTURE/cmd/sync" "$FIXTURE/context/actions" "$FIXTURE/context/rules"
cp "$REPO_DIR/cmd/sync/sync_context.sh" "$REPO_DIR/cmd/sync/smine_tool.sh" "$FIXTURE/cmd/sync/"
cat > "$FIXTURE/context/AGENTS.md" <<'EOF'
<!-- template marker comment -->

# Agents

{{ROLE}}

- Use {{CONTEXT_DIR}}/ for orientation
{{LANG:go}}
- Go style: {{CONTEXT_DIR}}/go/go.md
{{/LANG:go}}
{{LANG:python}}
- Python style: {{CONTEXT_DIR}}/python/python.md
{{/LANG:python}}
EOF
printf '%s\n' '**ACTION-NAV-001** `[review]` — Never scan.' '' '* Applies: everywhere.' \
  > "$FIXTURE/context/actions/navigation.md"
printf '%s\n' '# Plan Format' > "$FIXTURE/context/rules/plan.md"
printf '%s\n' '# Commits' > "$FIXTURE/context/rules/commits.md"
printf '%s\n' '# Go' > "$FIXTURE/context/rules/go.md"
printf '%s\n' '# Python' > "$FIXTURE/context/rules/python.md"

new_target() {
  local target
  target=$(mktemp -d "$TMP/target.XXXXXX")
  git -C "$target" init -q
  echo "$target"
}

test_flags_full_sync() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs go --role "You are a test agent." --symlink "$target" >/dev/null

  assert_exists "$target/AGENTS.md"
  assert_contains "$target/AGENTS.md" "You are a test agent."
  assert_contains "$target/AGENTS.md" "docs/ for orientation"
  assert_contains "$target/AGENTS.md" "Go style: docs/go/go.md"
  assert_not_contains "$target/AGENTS.md" "{{LANG:go}}"
  assert_not_contains "$target/AGENTS.md" "{{ROLE}}"
  assert_not_contains "$target/AGENTS.md" "Python style"
  assert_not_contains "$target/AGENTS.md" "template marker comment"
  assert_exists "$target/docs/actions/navigation.md"
  assert_exists "$target/docs/facts"
  assert_exists "$target/docs/rules/plan.md"
  assert_exists "$target/docs/rules/commits.md"
  assert_exists "$target/docs/rules/go.md"
  assert_missing "$target/docs/rules/python.md"
  assert_missing "$target/docs/general"
  [ -L "$target/CLAUDE.md" ] || fail "expected CLAUDE.md symlink"
  [ "$(readlink "$target/CLAUDE.md")" = "AGENTS.md" ] || fail "CLAUDE.md must point at AGENTS.md"
}

test_no_symlink() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs go --role "R." --no-symlink "$target" >/dev/null
  assert_missing "$target/CLAUDE.md"
}

test_empty_langs_baseline_only() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  assert_exists "$target/AGENTS.md"
  assert_not_contains "$target/AGENTS.md" "Go style"
  assert_missing "$target/docs/rules/go.md"
  assert_exists "$target/docs/rules/plan.md"
  assert_exists "$target/docs/rules/commits.md"
  assert_exists "$target/docs/actions/navigation.md"
}

test_general_lang_is_noop() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs general,go --role "R." --no-symlink "$target" >/dev/null
  assert_exists "$target/docs/rules/go.md"
  assert_missing "$target/docs/general"
}

test_unknown_lang_rejected() {
  local target
  target=$(new_target)
  if bash "$SCRIPT" --context-dir docs --langs rust --role "R." --no-symlink "$target" >/dev/null 2>&1; then
    fail "unknown --langs value must be rejected"
  fi
  assert_missing "$target/AGENTS.md"
}

test_non_git_target_rejected() {
  local target=$TMP/not-a-repo
  mkdir -p "$target"
  if bash "$SCRIPT" --context-dir docs --langs go --role "R." --no-symlink "$target" >/dev/null 2>&1; then
    fail "non-git target must be rejected"
  fi
}

test_interactive_prompts_still_work() {
  local target
  target=$(new_target)
  # answers: context dir (default), go y, python n, role (default), symlink n
  printf '\ny\nn\n\nn\n' | bash "$SCRIPT" "$target" >/dev/null
  assert_exists "$target/docs/rules/go.md"
  assert_missing "$target/docs/rules/python.md"
  assert_contains "$target/AGENTS.md" "You are a development agent for this repository."
  assert_missing "$target/CLAUDE.md"
}

test_deploy_settings_written_and_reused() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs go --role "You are a test agent." --no-symlink "$target" >/dev/null
  assert_exists "$target/docs/context.json"
  assert_contains "$target/docs/context.json" "You are a test agent."
  # second run: no flags beyond target — the deploy settings drive everything
  bash "$SCRIPT" "$target" >/dev/null
  assert_contains "$target/AGENTS.md" "You are a test agent."
  assert_exists "$target/docs/rules/go.md"
  assert_missing "$target/CLAUDE.md"
}

test_overlay_and_facts_preserved() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  printf '%s\n' '**ACTION-NAV-100** `[review]` — Repo rule.' '' '* Applies: here.' > "$target/docs/actions/local.md"
  printf '%s\n' '**FACT-REPO-STACK-001** — Fixture fact.' '' '* Location: go.mod' '* Reach: smine' > "$target/docs/facts/repo.md"
  cp "$target/docs/actions/local.md" "$TMP/local.before"
  # rules guides are baseline-owned: local target edits must be overwritten
  printf '%s\n' '# repo-edited plan guide' > "$target/docs/rules/plan.md"
  bash "$SCRIPT" "$target" >/dev/null
  cmp -s "$target/docs/actions/local.md" "$TMP/local.before" || fail "overlay file must survive re-sync byte-identical"
  assert_exists "$target/docs/facts/repo.md"
  assert_not_contains "$target/docs/rules/plan.md" "repo-edited"
}

test_baseline_header_and_collision() {
  local target
  target=$(new_target)
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  assert_contains "$target/docs/actions/navigation.md" "synced from smine"
  # unheaded file colliding with a baseline name must abort
  printf '%s\n' '# repo-owned' > "$target/docs/actions/navigation.md"
  if bash "$SCRIPT" "$target" >/dev/null 2>&1; then
    fail "collision with repo-owned file must be rejected"
  fi
}

test_seeded_violation_fails_validation() {
  # real repo script + real baseline: overlay entry numbered <100 must fail
  # the go-run context validation
  local target
  target=$(new_target)
  bash "$REPO_DIR/cmd/sync/sync_context.sh" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  printf '%s\n' '**ACTION-NAV-099** `[review]` — Overlay in baseline range.' '' '* Applies: here.' > "$target/docs/actions/zz-local.md"
  if bash "$REPO_DIR/cmd/sync/sync_context.sh" "$target" >/dev/null 2>&1; then
    fail "seeded overlay violation must fail context validation"
  fi
}

test_flags_full_sync
test_no_symlink
test_empty_langs_baseline_only
test_general_lang_is_noop
test_unknown_lang_rejected
test_non_git_target_rejected
test_interactive_prompts_still_work
test_deploy_settings_written_and_reused
test_overlay_and_facts_preserved
test_baseline_header_and_collision
test_seeded_violation_fails_validation

echo "test_sync_context: OK"
