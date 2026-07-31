#!/usr/bin/env bash
# Coverage for cherry_pick_worktree.sh: happy pick, dirty refusal, conflict
# abort+restore, unknown sha.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
SCRIPT="$REPO_DIR/cmd/worktrees/cherry_pick_worktree.sh"
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/cherry-pick-worktree.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == cherry-pick-worktree.* ]]; then
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

test_happy_pick() {
  local repo=$TMP/happy base sha
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  git -C "$repo" checkout -qb claude/feature
  printf '%s\n' agent > "$repo/agent"
  git -C "$repo" add agent
  git -C "$repo" commit -qm agent
  sha=$(git -C "$repo" rev-parse HEAD)
  git -C "$repo" checkout -q "$base"

  out=$(cd "$repo" && bash "$SCRIPT" "$sha") || fail "happy pick exited non-zero"
  echo "$out" | grep -q "picked $sha onto $base" || fail "missing picked line: $out"
  [ -f "$repo/agent" ] || fail "picked file missing"
}

test_dirty_refusal() {
  local repo=$TMP/dirty sha
  init_repo "$repo"
  sha=$(git -C "$repo" rev-parse HEAD)
  printf '%s\n' edit >> "$repo/base"

  if out=$(cd "$repo" && bash "$SCRIPT" "$sha"); then
    fail "dirty checkout accepted"
  fi
  echo "$out" | grep -q "dirty" || fail "missing dirty message: $out"
}

test_conflict_abort() {
  local repo=$TMP/conflict base head sha
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  git -C "$repo" checkout -qb claude/feature
  printf '%s\n' agent > "$repo/base"
  git -C "$repo" commit -qam agent
  sha=$(git -C "$repo" rev-parse HEAD)
  git -C "$repo" checkout -q "$base"
  printf '%s\n' local > "$repo/base"
  git -C "$repo" commit -qam local
  head=$(git -C "$repo" rev-parse HEAD)

  if out=$(cd "$repo" && bash "$SCRIPT" "$sha"); then
    fail "conflict pick exited zero"
  fi
  echo "$out" | grep -q "^CONFLICT $sha" || fail "missing CONFLICT line: $out"
  [ "$(git -C "$repo" rev-parse HEAD)" = "$head" ] || fail "HEAD moved after aborted pick"
  [ -z "$(git -C "$repo" status --porcelain)" ] || fail "checkout dirty after aborted pick"
}

test_unknown_sha() {
  local repo=$TMP/unknown
  init_repo "$repo"
  if out=$(cd "$repo" && bash "$SCRIPT" 0123456789abcdef0123456789abcdef01234567); then
    fail "unknown sha accepted"
  fi
  echo "$out" | grep -q "unknown commit" || fail "missing unknown-commit message: $out"
}

test_happy_pick
test_dirty_refusal
test_conflict_abort
test_unknown_sha
echo "cherry_pick_worktree tests: PASS"
