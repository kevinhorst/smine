#!/usr/bin/env bash
# Regression coverage for install_pool_guard.sh: the decoy contains git
# commands in hollow pool dirs (no fall-through to the main checkout) while
# real worktrees and worktree creation stay untouched.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/pool-guard.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == pool-guard.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

INSTALL="$REPO_DIR/cmd/worktrees/install_pool_guard.sh"
DECOY_POINTER="gitdir: /claude-pool-guard-see-smine-cmd-worktrees"

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

test_husk_falls_through_without_decoy() {
  local repo=$TMP/fallthrough
  init_repo "$repo"
  mkdir -p "$repo/.claude/worktrees/husk"
  local top
  top=$(git -C "$repo/.claude/worktrees/husk" rev-parse --show-toplevel)
  [ "$top" = "$repo" ] || fail "expected husk to fall through to $repo, got $top"
}

test_decoy_contains_husk() {
  local repo=$TMP/contain husk rc out head_before head_after ref_before ref_after
  init_repo "$repo"
  husk="$repo/.claude/worktrees/husk"
  mkdir -p "$husk"

  bash "$INSTALL" "$repo" >/dev/null || fail "installer failed"
  [ "$(cat "$repo/.claude/worktrees/.git")" = "$DECOY_POINTER" ] \
    || fail "decoy content wrong"

  head_before=$(git -C "$repo" rev-parse HEAD)
  ref_before=$(git -C "$repo" symbolic-ref HEAD)

  rc=0
  out=$(git -C "$husk" rev-parse --show-toplevel 2>&1) || rc=$?
  [ "$rc" -eq 128 ] || fail "expected exit 128 in husk, got $rc ($out)"

  rc=0
  git -C "$husk" checkout -b claude/hijack origin/main 2>/dev/null || rc=$?
  [ "$rc" -ne 0 ] || fail "checkout in husk unexpectedly succeeded"

  head_after=$(git -C "$repo" rev-parse HEAD)
  ref_after=$(git -C "$repo" symbolic-ref HEAD)
  [ "$head_before" = "$head_after" ] || fail "main HEAD sha moved"
  [ "$ref_before" = "$ref_after" ] || fail "main HEAD ref moved"
  git -C "$repo" rev-parse --verify -q refs/heads/claude/hijack >/dev/null \
    && fail "hijack branch was created in the main repo"
  return 0
}

test_real_worktrees_unaffected() {
  local repo=$TMP/realwt wt top base
  init_repo "$repo"
  bash "$INSTALL" "$repo" >/dev/null
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  wt="$repo/.claude/worktrees/real"

  git -C "$repo" worktree add -q -b claude/real "$wt" "$base" \
    || fail "worktree add failed with decoy present"

  top=$(git -C "$wt" rev-parse --show-toplevel)
  [ "$top" = "$wt" ] || fail "worktree resolves to $top, not itself"
  [ "$(git -C "$wt" branch --show-current)" = "claude/real" ] \
    || fail "worktree not on its branch"

  mkdir -p "$wt/sub"
  top=$(git -C "$wt/sub" rev-parse --show-toplevel)
  [ "$top" = "$wt" ] || fail "worktree subdir resolves to $top"

  [ "$(git -C "$repo" symbolic-ref --short HEAD)" = "$base" ] \
    || fail "main checkout branch changed by worktree add"
}

test_idempotent_install() {
  local repo=$TMP/idem out
  init_repo "$repo"
  bash "$INSTALL" "$repo" >/dev/null
  out=$(bash "$INSTALL" "$repo") || fail "second install failed"
  [[ "$out" == *"already installed"* ]] || fail "second install did not report ok: $out"
  [ "$(cat "$repo/.claude/worktrees/.git")" = "$DECOY_POINTER" ] \
    || fail "decoy content changed on reinstall"
}

test_foreign_git_entry_not_replaced() {
  local repo=$TMP/foreign rc
  init_repo "$repo"
  mkdir -p "$repo/.claude/worktrees/.git/objects"
  rc=0
  bash "$INSTALL" "$repo" >/dev/null 2>&1 || rc=$?
  [ "$rc" -ne 0 ] || fail "installer accepted a foreign .git entry"
  [ -d "$repo/.claude/worktrees/.git/objects" ] || fail "foreign .git entry was replaced"
}

test_no_arg_derives_repo() {
  local repo=$TMP/noarg wt base
  init_repo "$repo"
  (cd "$repo" && bash "$INSTALL" >/dev/null) || fail "no-arg install from main failed"
  [ -f "$repo/.claude/worktrees/.git" ] || fail "no decoy after no-arg install"

  base=$(git -C "$repo" symbolic-ref --short HEAD)
  wt="$repo/.claude/worktrees/real"
  git -C "$repo" worktree add -q -b claude/real "$wt" "$base"
  (cd "$wt" && bash "$INSTALL" >/dev/null) \
    || fail "no-arg install from inside a worktree failed"
}

test_husk_falls_through_without_decoy
test_decoy_contains_husk
test_real_worktrees_unaffected
test_idempotent_install
test_foreign_git_entry_not_replaced
test_no_arg_derives_repo

echo "PASS: install_pool_guard.sh"
