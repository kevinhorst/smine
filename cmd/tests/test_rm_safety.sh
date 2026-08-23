#!/usr/bin/env bash
# Regression coverage for guarded recursive cleanup in shell scripts.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/rm-safety.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == rm-safety.* ]]; then
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

test_sync_skills() {
  local fixture=$TMP/sync home=$TMP/sync-home victim=$TMP/victim
  mkdir -p "$fixture/cmd/sync" "$fixture/cmd/worktrees/_lib" "$fixture/skills/keep" \
    "$home/.claude/hooks" "$home/.claude/skills/stale" "$victim"
  cp "$REPO_DIR/cmd/sync/sync_skills.sh" "$REPO_DIR/cmd/sync/smine_tool.sh" "$fixture/cmd/sync/"
  cp "$REPO_DIR/cmd/worktrees/remove_agent_worktrees.sh" "$fixture/cmd/worktrees/"
  cp "$REPO_DIR/cmd/worktrees/_lib/verdict.sh" "$fixture/cmd/worktrees/_lib/"
  printf '%s\n' 'name: keep' > "$fixture/skills/keep/SKILL.md"
  printf '%s\n' 'AGENT_CONTEXT_DIR_DEFAULT=docs' > "$home/.claude/hooks/global-context.env"
  printf '%s\n' stale > "$home/.claude/skills/stale/file"

  printf 'y\n' | HOME="$home" bash "$fixture/cmd/sync/sync_skills.sh"
  assert_missing "$home/.claude/skills/stale"
  assert_exists "$home/.claude/agents/tools/remove_agent_worktrees.sh"
  assert_exists "$home/.claude/agents/tools/_lib/verdict.sh"

  printf '%s\n' sentinel > "$victim/sentinel"
  ln -s "$victim" "$home/.claude/skills/unsafe-link"
  if printf 'y\n' | HOME="$home" bash "$fixture/cmd/sync/sync_skills.sh"; then
    fail "sync_skills accepted a symlink prune target"
  fi
  assert_exists "$victim/sentinel"
}

init_repo() {
  local repo=$1
  mkdir -p "$repo"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base
}

test_temporary_worktree_cleanup() {
  local repo=$TMP/status worktmp=$TMP/status-tmp base
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  git -C "$repo" checkout -qb claude/test
  printf '%s\n' change > "$repo/change"
  git -C "$repo" add change
  git -C "$repo" commit -qm change
  git -C "$repo" checkout -q "$base"
  mkdir -p "$worktmp"

  (cd "$repo" && TMPDIR="$worktmp" bash "$REPO_DIR/cmd/worktrees/print_agent_worktrees_status.sh" 1 unpicked)
  if git -C "$repo" worktree list --porcelain | grep -Fq "worktree $worktmp/"; then
    fail "temporary worktree was not removed"
  fi
}

test_sync_skills
test_temporary_worktree_cleanup
echo "shell cleanup safety tests: PASS"
