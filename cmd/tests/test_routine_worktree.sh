#!/usr/bin/env bash
# Coverage for routines/_lib/worktree.sh (local-only contract): fresh group
# branch from main + commit, reuse while unmerged, reset once merged, own-group
# stale-worktree sweep, sibling-group survival, no-change skip, commit-body
# handoff. Needs only git + a temp dir (no gh, no network; the flock-based
# group lock is exercised manually, not here).
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
LIB="$REPO_DIR/routines/_lib/worktree.sh"
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/routine-worktree.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == routine-worktree.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# single local repo with a main branch holding one base commit — no origin,
# no push
setup_case() {
  local case=$1
  local repo="$TMP/$case-repo"
  git init -q "$repo"
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  git -C "$repo" checkout -qb main
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base
  repo_root="$repo"
  routine_dir="$repo/routines/probe-nightly"
  mkdir -p "$routine_dir"
  export ROUTINE_WT_ROOT_OVERRIDE="$TMP/$case-wt"
}

run_lib() {
  # subshell so each case gets fresh lib state; HOME redirected under TMP
  ( HOME="$TMP" && source "$LIB" && ROUTINE_WT_ROOT="$ROUTINE_WT_ROOT_OVERRIDE" && "$@" )
}

run_lib_group() {
  # like run_lib, but pins ROUTINE_GROUP (chain-member simulation)
  local group=$1
  shift
  ( HOME="$TMP" && ROUTINE_GROUP="$group" && source "$LIB" \
      && ROUTINE_WT_ROOT="$ROUTINE_WT_ROOT_OVERRIDE" && "$@" )
}

test_fresh_branch_and_commit() {
  setup_case fresh
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  [[ -d "$wt" ]] || fail "worktree missing"
  printf '%s\n' out > "$wt/sessions-out"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  [[ -n "$(git -C "$repo_root" rev-list main..routine/probe-nightly)" ]] \
    || fail "no commit on routine/probe-nightly ahead of main"
  [[ ! -d "$wt" ]] || fail "worktree not removed after publish"
}

test_reuse_with_unmerged_commits() {
  setup_case reuse
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  printf '%s\n' one > "$wt/night-one"
  run_lib routine_worktree_publish 0 || fail "first publish failed"
  # main never merged the branch -> the commit is still unmerged; reuse it
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  [[ -f "$wt/night-one" ]] || fail "branch reset despite unmerged commits"
}

test_reset_after_merge() {
  setup_case reset
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  printf '%s\n' run-one > "$wt/run-one"
  run_lib routine_worktree_publish 0 || fail "first publish failed"
  # accept the run: merge routine/probe-nightly into main, then main advances on its
  # own. A merged branch is an ancestor of main -> next create resets to main
  # (fresh) rather than reusing. (A local merge carries run-one into main, so
  # file-presence cannot discriminate reset from reuse — advancing main can.)
  git -C "$repo_root" merge -q routine/probe-nightly || fail "merge failed"
  printf '%s\n' later > "$repo_root/main-later"
  git -C "$repo_root" add main-later
  git -C "$repo_root" commit -qm "main advances"
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  [[ -f "$wt/main-later" ]] || fail "branch not reset onto advanced main"
  [[ -z "$(git -C "$repo_root" rev-list main..routine/probe-nightly)" ]] \
    || fail "reset branch carries commits main lacks"
}

test_stale_worktree_sweep() {
  setup_case sweep
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  # crashed run: worktree left holding the branch; create must free it
  wt2=$(run_lib routine_worktree_create) || fail "create with stale worktree failed"
  [[ -d "$wt2" ]] || fail "second worktree missing"
}

test_sibling_group_survival() {
  setup_case sibling
  wt_a=$(run_lib_group alpha routine_worktree_create) || fail "alpha create failed"
  printf '%s\n' wip > "$wt_a/wip"
  # a concurrent run of a different group must not touch alpha's live worktree
  wt_b=$(run_lib_group beta routine_worktree_create) || fail "beta create failed"
  [[ -d "$wt_b" ]] || fail "beta worktree missing"
  [[ -f "$wt_a/wip" ]] || fail "sibling group's live worktree was destroyed"
  [[ "$wt_a" != "$wt_b" ]] || fail "groups share a worktree path"
  git -C "$repo_root" rev-parse --quiet --verify routine/alpha >/dev/null \
    && git -C "$repo_root" rev-parse --quiet --verify routine/beta >/dev/null \
    || fail "per-group branches missing"
}

test_no_change_no_commit() {
  setup_case nochange
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  # clean tree -> no commit; branch tip equals main
  [[ -z "$(git -C "$repo_root" rev-list main..routine/probe-nightly)" ]] \
    || fail "empty run created a commit"
}

test_failed_run_marks_and_resets() {
  setup_case failreset
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  printf '%s\n' partial > "$wt/partial"
  # a failed claude run (non-zero exit) still publishes, but marks the subject
  run_lib routine_worktree_publish 1 || fail "publish failed"
  git -C "$repo_root" log -1 --format=%s routine/probe-nightly | grep -q '\[failed\]' \
    || fail "failure commit subject missing [failed] marker"
  # next create: the only unmerged commit is a failed run -> reset to main
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  [[ ! -f "$wt/partial" ]] || fail "branch not reset despite only failed runs"
  [[ -z "$(git -C "$repo_root" rev-list main..routine/probe-nightly)" ]] \
    || fail "reset branch still carries the failed commit"
}

test_real_commit_survives_among_failures() {
  setup_case failkeep
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  printf '%s\n' good > "$wt/good"
  run_lib routine_worktree_publish 0 || fail "success publish failed"
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  printf '%s\n' bad > "$wt/bad"
  run_lib routine_worktree_publish 1 || fail "failure publish failed"
  # branch now holds one real + one failed commit -> must be kept, not reset
  wt=$(run_lib routine_worktree_create) || fail "third create failed"
  [[ -f "$wt/good" ]] || fail "branch reset despite a reviewable commit"
}

test_commit_body_handoff() {
  setup_case body
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  printf '%s\n' change > "$wt/changed"
  printf '%s\n' 'disposition table body' > "$wt/.routine-commit-body"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  git -C "$repo_root" log -1 --format=%b routine/probe-nightly | grep -q "disposition table body" \
    || fail "commit body missing the handoff content"
  git -C "$repo_root" show --name-only --format= routine/probe-nightly | grep -q '\.routine-commit-body' \
    && fail "handoff file entered the commit"
  git -C "$repo_root" show --name-only --format= routine/probe-nightly | grep -q '^changed$' \
    || fail "changed file not committed"
}

test_fresh_branch_and_commit
test_reuse_with_unmerged_commits
test_reset_after_merge
test_stale_worktree_sweep
test_sibling_group_survival
test_no_change_no_commit
test_failed_run_marks_and_resets
test_real_commit_survives_among_failures
test_commit_body_handoff
echo "OK"
