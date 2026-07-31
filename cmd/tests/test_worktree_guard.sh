#!/usr/bin/env bash
# Regression coverage for worktree-guard.sh: prompts pass in valid locations
# and are blocked (exit 2) in hollow or broken pool worktree dirs.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/wt-guard.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == wt-guard.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

GUARD="$REPO_DIR/cmd/hooks/worktree-guard.sh"

# run_guard <dir> -> sets rc and err
run_guard() {
  rc=0
  err=$( (cd "$1" && bash "$GUARD" </dev/null) 2>&1 ) || rc=$?
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

repo=$TMP/repo
init_repo "$repo"
base=$(git -C "$repo" symbolic-ref --short HEAD)
pool="$repo/.claude/worktrees"
git -C "$repo" worktree add -q -b claude/real "$pool/real" "$base"
mkdir -p "$pool/real/sub" "$pool/hollow" "$pool/broken"
printf 'gitdir: %s\n' "$repo/.git/worktrees/gone" > "$pool/broken/.git"

test_outside_pool_passes() {
  run_guard "$repo"
  [ "$rc" -eq 0 ] || fail "guard blocked the main checkout: $err"
  run_guard "$TMP"
  [ "$rc" -eq 0 ] || fail "guard blocked a non-repo dir: $err"
}

test_valid_worktree_passes() {
  run_guard "$pool/real"
  [ "$rc" -eq 0 ] || fail "guard blocked a valid worktree: $err"
  run_guard "$pool/real/sub"
  [ "$rc" -eq 0 ] || fail "guard blocked a valid worktree subdir: $err"
}

test_hollow_dir_blocks() {
  run_guard "$pool/hollow"
  [ "$rc" -eq 2 ] || fail "expected exit 2 in hollow dir, got $rc"
  [[ "$err" == *"$pool/hollow"* ]] || fail "block message does not name the dir: $err"
  [[ "$err" == *BLOCKED* ]] || fail "block message missing BLOCKED marker: $err"
}

test_dead_gitdir_blocks() {
  run_guard "$pool/broken"
  [ "$rc" -eq 2 ] || fail "expected exit 2 on dead gitdir, got $rc"
  [[ "$err" == *"missing gitdir"* ]] || fail "unexpected message: $err"
}

test_outside_pool_passes
test_valid_worktree_passes
test_hollow_dir_blocks
test_dead_gitdir_blocks

echo "PASS: worktree-guard.sh"
