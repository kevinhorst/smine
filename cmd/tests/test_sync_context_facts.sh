#!/usr/bin/env bash
# Regression coverage for central facts deployment in sync_context.sh:
# reach-covered facts files ship baseline-headed, non-covered files never ship,
# repo-owned collisions abort, and re-syncs overwrite the baseline copy.
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
TMP=$(mktemp -d "$TMP_BASE/sync-context-facts.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == sync-context-facts.* ]]; then
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

# Fixture: script copy + controlled context source (REPO_DIR derives from the
# script location, so the copy reads $FIXTURE/context). Filter/generate run
# against the real module via the override.
export SYNC_CONTEXT_RULES_REPO="$REPO_DIR"
FIXTURE=$TMP/fixture
SCRIPT=$FIXTURE/cmd/sync/sync_context.sh
mkdir -p "$FIXTURE/cmd/sync" "$FIXTURE/context/actions" "$FIXTURE/context/rules" "$FIXTURE/context/facts"
cp "$REPO_DIR/cmd/sync/sync_context.sh" "$REPO_DIR/cmd/sync/smine_tool.sh" "$FIXTURE/cmd/sync/"
cat > "$FIXTURE/context/AGENTS.md" <<'EOF'
# Agents

{{ROLE}}
EOF
printf '%s\n' '**ACTION-NAV-001** `[review]` — Never scan.' '' '* Applies: everywhere.' \
  > "$FIXTURE/context/actions/navigation.md"
printf '%s\n' '# Plan Format' > "$FIXTURE/context/rules/plan.md"
printf '%s\n' '# Commits' > "$FIXTURE/context/rules/commits.md"

new_target() {
  local target
  target=$(mktemp -d "$TMP/target.XXXXXX")
  git -C "$target" init -q
  echo "$target"
}

# Facts files are authored per target reach, so they are written once the
# target's basename is known.
write_facts() {
  local target_name=$1
  printf '%s\n' '# Covered — Facts' '' \
    "**FACT-REPO-STACK-001** — Fact reaching the target." '' \
    '* Location: go.mod' \
    "* Reach: $target_name" \
    > "$FIXTURE/context/facts/covered.md"
  printf '%s\n' '# Internal — Facts' '' \
    '**FACT-REPO-STACK-001** — Fact staying home.' '' \
    '* Location: Makefile' \
    '* Reach: smine' \
    > "$FIXTURE/context/facts/internal.md"
}

test_reach_covered_facts_deploy() {
  local target
  target=$(new_target)
  write_facts "$(basename "$target")"
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null

  assert_exists "$target/docs/facts/covered.md"
  assert_contains "$target/docs/facts/covered.md" "synced from smine"
  assert_contains "$target/docs/facts/covered.md" "FACT-REPO-STACK-001"
  assert_missing "$target/docs/facts/internal.md"
}

test_resync_overwrites_baseline_facts() {
  local target
  target=$(new_target)
  write_facts "$(basename "$target")"
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  printf '%s\n' '<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->' \
    '' '# stale-edited copy' > "$target/docs/facts/covered.md"
  bash "$SCRIPT" "$target" >/dev/null
  assert_contains "$target/docs/facts/covered.md" "FACT-REPO-STACK-001"
}

test_repo_owned_collision_aborts() {
  local target
  target=$(new_target)
  write_facts "$(basename "$target")"
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  printf '%s\n' '**FACT-REPO-STACK-002** — Repo-owned fact.' '' '* Location: here' \
    > "$target/docs/facts/covered.md"
  if bash "$SCRIPT" "$target" >/dev/null 2>&1; then
    fail "collision with repo-owned facts file must be rejected"
  fi
}

test_repo_owned_sibling_facts_survive() {
  local target
  target=$(new_target)
  write_facts "$(basename "$target")"
  bash "$SCRIPT" --context-dir docs --langs "" --role "R." --no-symlink "$target" >/dev/null
  printf '%s\n' '**FACT-REPO-STACK-002** — Repo-owned fact.' '' '* Location: here' \
    "* Reach: $(basename "$target")" \
    > "$target/docs/facts/local.md"
  cp "$target/docs/facts/local.md" "$TMP/local.before"
  bash "$SCRIPT" "$target" >/dev/null
  cmp -s "$target/docs/facts/local.md" "$TMP/local.before" || fail "repo-owned facts file must survive re-sync byte-identical"
}

test_reach_covered_facts_deploy
test_resync_overwrites_baseline_facts
test_repo_owned_collision_aborts
test_repo_owned_sibling_facts_survive

echo "test_sync_context_facts: OK"
