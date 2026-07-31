#!/usr/bin/env bash
# Regression coverage for the pool sweep (worktree-sessionstart.sh --sweep):
# hollow pool dirs are deleted, everything that could hold work is spared.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/wt-sweep.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == wt-sweep.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT="$REPO_DIR/cmd/hooks/worktree-sessionstart.sh"

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

test_sweep() {
  local repo=$TMP/repo pool base victim
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  pool="$repo/.claude/worktrees"
  mkdir -p "$pool"

  # husk: only infrastructure entries -> must be deleted
  mkdir -p "$pool/husk/.idea" "$pool/husk/.claude"
  touch "$pool/husk/.DS_Store" "$pool/husk/.claude-worktree"

  # not a worktree, but holds a real file -> must be kept
  mkdir -p "$pool/notes/.idea"
  printf '%s\n' important > "$pool/notes/todo.txt"

  # real registered worktree -> must be kept
  git -C "$repo" worktree add -q -b claude/real "$pool/real" "$base"

  # dead gitdir pointer -> must be kept (may hold work)
  mkdir -p "$pool/broken"
  printf 'gitdir: %s\n' "$repo/.git/worktrees/gone" > "$pool/broken/.git"
  printf '%s\n' work > "$pool/broken/wip.txt"

  # symlink in the pool -> must not be followed or deleted through
  victim=$TMP/victim
  mkdir -p "$victim"
  printf '%s\n' sentinel > "$victim/sentinel"
  ln -s "$victim" "$pool/linked"

  bash "$SCRIPT" --sweep "$repo" 2>/dev/null || fail "sweep exited nonzero"

  [ ! -e "$pool/husk" ] || fail "husk survived the sweep"
  [ -f "$pool/notes/todo.txt" ] || fail "dir with real files was deleted"
  [ -f "$pool/real/base" ] || fail "real worktree was deleted"
  git -C "$repo" worktree list --porcelain | grep -qF "worktree $pool/real" \
    || fail "real worktree lost its registration"
  [ -f "$pool/broken/wip.txt" ] || fail "dead-gitdir dir was deleted"
  [ -f "$victim/sentinel" ] || fail "sweep deleted through a symlink"
  [ -f "$pool/.git" ] || fail "sweep removed the decoy"
  [ "$(cat "$pool/.git")" = "gitdir: /claude-pool-guard-see-smine-cmd-worktrees" ] \
    || fail "decoy content wrong after sweep (drift between scripts?)"
}

test_sweep_rejects_non_repo() {
  local rc=0
  mkdir -p "$TMP/plain"
  bash "$SCRIPT" --sweep "$TMP/plain" 2>/dev/null || rc=$?
  [ "$rc" -ne 0 ] || fail "sweep accepted a non-repo directory"
}

test_sweep
test_sweep_rejects_non_repo

echo "PASS: worktree-sessionstart.sh --sweep"
