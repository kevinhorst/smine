#!/usr/bin/env bash
# Regression coverage for worktree-sessionstart.sh hook mode: hardening of
# valid pool worktrees (vcs.xml, sentinel, lock), pool protection (decoy +
# sweep), silence on stdout, and integration with remove_agent_worktrees.sh
# (unlock + infrastructure files tolerated).
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/wt-session.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == wt-session.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT="$REPO_DIR/cmd/hooks/worktree-sessionstart.sh"
REMOVE="$REPO_DIR/cmd/worktrees/remove_agent_worktrees.sh"

# run_hook <dir> -> sets rc, out (stdout), err (stderr)
run_hook() {
  local outfile=$TMP/stdout errfile=$TMP/stderr
  rc=0
  (cd "$1" && bash "$SCRIPT" </dev/null >"$outfile" 2>"$errfile") || rc=$?
  out=$(cat "$outfile")
  err=$(cat "$errfile")
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

make_husk() {
  mkdir -p "$1/.idea"
  touch "$1/.DS_Store"
}

test_valid_worktree_hardened() {
  local repo=$TMP/valid base wt pool
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  pool="$repo/.claude/worktrees"
  git -C "$repo" worktree add -q -b claude/work "$pool/work" "$base"
  wt="$pool/work"
  make_husk "$pool/husk"

  run_hook "$wt"
  [ "$rc" -eq 0 ] || fail "hook exited $rc in valid worktree: $err"
  [ -z "$out" ] || fail "hook wrote to stdout (context pollution): $out"

  grep -q 'mapping directory="\$PROJECT_DIR\$" vcs="Git"' "$wt/.idea/vcs.xml" \
    || fail "vcs.xml missing or wrong"
  grep -q '^branch: claude/work$' "$wt/.claude-worktree" \
    || fail "sentinel missing or wrong branch"
  git -C "$repo" worktree list --porcelain | grep -q '^locked' \
    || fail "worktree not locked"
  [ -f "$pool/.git" ] || fail "decoy not installed"
  [ ! -e "$pool/husk" ] || fail "sibling husk not swept"

  # idempotence: second run changes nothing and stays silent
  local sentinel_before
  sentinel_before=$(cat "$wt/.claude-worktree")
  run_hook "$wt"
  [ "$rc" -eq 0 ] || fail "second run exited $rc"
  [ -z "$out" ] || fail "second run wrote to stdout: $out"
  [ "$(cat "$wt/.claude-worktree")" = "$sentinel_before" ] \
    || fail "sentinel rewritten on second run"
}

test_hollow_cwd_untouched() {
  local repo=$TMP/hollow pool
  init_repo "$repo"
  pool="$repo/.claude/worktrees"
  make_husk "$pool/mine"
  make_husk "$pool/other"

  run_hook "$pool/mine"
  [ "$rc" -eq 0 ] || fail "hook must exit 0 in a hollow dir (SessionStart cannot block), got $rc"
  [ -z "$out" ] || fail "hook wrote to stdout: $out"
  [ -d "$pool/mine" ] || fail "hook deleted the session's own cwd"
  [ ! -e "$pool/other" ] || fail "sibling husk not swept"
  [ -f "$pool/.git" ] || fail "decoy not installed"
  [[ "$err" == *hollow* ]] || fail "no hollow warning on stderr: $err"
}

test_main_checkout_protected() {
  local repo=$TMP/main pool
  init_repo "$repo"
  pool="$repo/.claude/worktrees"
  make_husk "$pool/husk"

  run_hook "$repo"
  [ "$rc" -eq 0 ] || fail "hook exited $rc in main checkout: $err"
  [ -z "$out" ] || fail "hook wrote to stdout: $out"
  [ -f "$pool/.git" ] || fail "decoy not installed from main checkout"
  [ ! -e "$pool/husk" ] || fail "husk not swept from main checkout"

  run_hook "$TMP"
  [ "$rc" -eq 0 ] || fail "hook exited $rc outside any repo"
  [ -z "$out" ] || fail "hook wrote to stdout outside any repo: $out"
}

test_remove_script_handles_hardened_worktree() {
  local repo=$TMP/remove base pool wt
  init_repo "$repo"
  base=$(git -C "$repo" symbolic-ref --short HEAD)
  pool="$repo/.claude/worktrees"
  git -C "$repo" worktree add -q -b claude/done "$pool/done" "$base"
  wt="$pool/done"

  run_hook "$wt"
  git -C "$repo" worktree list --porcelain | grep -q '^locked' || fail "precondition: not locked"

  (cd "$repo" && bash "$REMOVE" >/dev/null) || fail "remove script failed"
  [ ! -d "$wt" ] || fail "hardened worktree not removed (lock or sentinel in the way)"
  git -C "$repo" worktree list --porcelain | grep -qF "worktree $wt" \
    && fail "worktree still registered after removal"
  return 0
}

test_valid_worktree_hardened
test_hollow_cwd_untouched
test_main_checkout_protected
test_remove_script_handles_hardened_worktree

echo "PASS: worktree-sessionstart.sh"
