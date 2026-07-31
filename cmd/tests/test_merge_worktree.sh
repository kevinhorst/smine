#!/usr/bin/env bash
# Coverage for merge_worktree.sh: happy merge, dirty refusal, conflict
# abort+restore, unknown branch.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
SCRIPT="$REPO_DIR/cmd/worktrees/merge_worktree.sh"
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/merge-worktree.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == merge-worktree.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
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

test_happy_merge() {
  local repo=$TMP/happy base
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  git -C "$repo" checkout -qb claude/feature
  printf '%s\n' agent > "$repo/agent"
  git -C "$repo" add agent
  git -C "$repo" commit -qm agent
  git -C "$repo" checkout -q "$base"

  out=$(cd "$repo" && bash "$SCRIPT" claude/feature) || fail "happy merge exited non-zero"
  echo "$out" | grep -q "merged claude/feature into $base" || fail "missing merged line: $out"
  [ -f "$repo/agent" ] || fail "merged file missing"
}

test_dirty_refusal() {
  local repo=$TMP/dirty base
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  git -C "$repo" branch claude/feature
  printf '%s\n' edit >> "$repo/base"

  if out=$(cd "$repo" && bash "$SCRIPT" claude/feature); then
    fail "dirty checkout accepted"
  fi
  echo "$out" | grep -q "dirty" || fail "missing dirty message: $out"
}

test_conflict_abort() {
  local repo=$TMP/conflict base head
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  git -C "$repo" checkout -qb claude/feature
  printf '%s\n' agent > "$repo/base"
  git -C "$repo" commit -qam agent
  git -C "$repo" checkout -q "$base"
  printf '%s\n' local > "$repo/base"
  git -C "$repo" commit -qam local
  head=$(git -C "$repo" rev-parse HEAD)

  if out=$(cd "$repo" && bash "$SCRIPT" claude/feature); then
    fail "conflict merge exited zero"
  fi
  echo "$out" | grep -q "^CONFLICT claude/feature" || fail "missing CONFLICT line: $out"
  [ "$(git -C "$repo" rev-parse HEAD)" = "$head" ] || fail "HEAD moved after aborted merge"
  [ -z "$(git -C "$repo" status --porcelain)" ] || fail "checkout dirty after aborted merge"
}

test_unknown_branch() {
  local repo=$TMP/unknown
  init_repo "$repo"
  if out=$(cd "$repo" && bash "$SCRIPT" claude/nope); then
    fail "unknown branch accepted"
  fi
  echo "$out" | grep -q "unknown branch" || fail "missing unknown-branch message: $out"
}

test_happy_merge
test_dirty_refusal
test_conflict_abort
test_unknown_branch
echo "merge_worktree tests: PASS"
