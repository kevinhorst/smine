#!/usr/bin/env bash
# Coverage for routines/_lib/worktree.sh (local-only contract): a fresh dated
# branch per run, never reused; a new run based on the newest un-merged dated
# branch (chain tip) else main; same-day collision counter; merged-branch prune;
# all-[failed] chain discard; own-group stale-worktree sweep; sibling-group
# survival; the open-instance limit; no-change skip; commit-body handoff. Needs
# only git + a temp dir (no gh, no network; the flock-based group lock is
# exercised manually, not here).
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

run_lib_env() {
  # like run_lib, but exports one extra VAR=value into the subshell (the limit)
  local assignment=$1
  shift
  ( HOME="$TMP" && export "$assignment" && source "$LIB" \
      && ROUTINE_WT_ROOT="$ROUTINE_WT_ROOT_OVERRIDE" && "$@" )
}

run_lib_stub() {
  # like run_lib, but prepends a stub-binary dir to PATH (fake claude for the
  # chain-sync conflict paths)
  local stubdir=$1
  shift
  ( HOME="$TMP" && PATH="$stubdir:$PATH" && source "$LIB" \
      && ROUTINE_WT_ROOT="$ROUTINE_WT_ROOT_OVERRIDE" && "$@" )
}

# main gains a commit editing $1 (relative path) with content $2
advance_main() {
  printf '%s\n' "$2" > "$repo_root/$1"
  git -C "$repo_root" add "$1"
  git -C "$repo_root" commit -qm "main edits $1"
}

# branch checked out in a worktree = the dated branch create just minted
head_branch() {
  git -C "$1" rev-parse --abbrev-ref HEAD
}

test_dated_branch_name() {
  setup_case dated
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  [[ -d "$wt" ]] || fail "worktree missing"
  local branch
  branch=$(head_branch "$wt")
  [[ "$branch" =~ ^claude-routines/probe-nightly-[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] \
    || fail "branch name not dated: $branch"
}

test_same_day_collision() {
  setup_case collision
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  local b1 b2
  b1=$(head_branch "$wt")
  printf '%s\n' one > "$wt/n1"
  run_lib routine_worktree_publish 0 || fail "first publish failed"
  # same-day second run: the plain date is taken, so the counter kicks in
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  b2=$(head_branch "$wt")
  [[ "$b2" == "$b1-2" ]] || fail "second same-day branch not -2 suffixed: $b1 -> $b2"
  git -C "$repo_root" rev-parse --quiet --verify "refs/heads/$b1" >/dev/null \
    || fail "first branch vanished (should persist un-merged)"
}

test_base_off_latest() {
  setup_case baselatest
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  local b1 b2
  b1=$(head_branch "$wt")
  printf '%s\n' one > "$wt/night-one"
  run_lib routine_worktree_publish 0 || fail "first publish failed"
  # main never merged the branch -> new run is a distinct ref based on the tip
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  b2=$(head_branch "$wt")
  [[ "$b2" != "$b1" ]] || fail "second run reused the branch ref"
  [[ -f "$wt/night-one" ]] || fail "new branch not based on the un-merged tip"
}

test_reset_after_merge() {
  setup_case reset
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  local b1 b2
  b1=$(head_branch "$wt")
  printf '%s\n' run-one > "$wt/run-one"
  run_lib routine_worktree_publish 0 || fail "first publish failed"
  # accept the run: merge the dated branch into main, then main advances on its
  # own. The merged branch is an ancestor of main -> next create prunes it and
  # bases the new run on the advanced main.
  git -C "$repo_root" merge -q "$b1" || fail "merge failed"
  printf '%s\n' later > "$repo_root/main-later"
  git -C "$repo_root" add main-later
  git -C "$repo_root" commit -qm "main advances"
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  b2=$(head_branch "$wt")
  [[ -f "$wt/main-later" ]] || fail "branch not based on advanced main"
  [[ -z "$(git -C "$repo_root" rev-list main.."$b2")" ]] \
    || fail "new branch carries commits main lacks"
  # b1 was pruned; the new run re-uses the same date name, so assert the count,
  # not the name — a surviving b1 would leave two dated branches.
  local count
  count=$(git -C "$repo_root" for-each-ref --format='%(refname:short)' \
    'refs/heads/claude-routines/probe-nightly-*' | grep -c . || true)
  [[ "$count" -eq 1 ]] || fail "merged dated branch not pruned (expected 1 open, got $count)"
}

test_failed_chain_discard() {
  setup_case failreset
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  local b1 b2
  b1=$(head_branch "$wt")
  printf '%s\n' partial > "$wt/partial"
  # a failed claude run (non-zero exit) still publishes, but marks the subject
  run_lib routine_worktree_publish 1 || fail "publish failed"
  git -C "$repo_root" log -1 --format=%s "$b1" | grep -q '\[failed\]' \
    || fail "failure commit subject missing [failed] marker"
  # next create: the only un-merged chain is a failed run -> discard, base on main
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  b2=$(head_branch "$wt")
  [[ ! -f "$wt/partial" ]] || fail "new branch not based on main despite only failed runs"
  [[ -z "$(git -C "$repo_root" rev-list main.."$b2")" ]] \
    || fail "new branch still carries the failed commit"
  # b1 was discarded; the new run re-uses the same date name, so assert the
  # count — a surviving failed b1 would leave two dated branches.
  local count
  count=$(git -C "$repo_root" for-each-ref --format='%(refname:short)' \
    'refs/heads/claude-routines/probe-nightly-*' | grep -c . || true)
  [[ "$count" -eq 1 ]] || fail "all-[failed] chain not discarded (expected 1 open, got $count)"
}

test_real_commit_survives_among_failures() {
  setup_case failkeep
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  printf '%s\n' good > "$wt/good"
  run_lib routine_worktree_publish 0 || fail "success publish failed"
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  printf '%s\n' bad > "$wt/bad"
  run_lib routine_worktree_publish 1 || fail "failure publish failed"
  # chain now holds one real + one failed commit -> kept, next run based on it
  wt=$(run_lib routine_worktree_create) || fail "third create failed"
  [[ -f "$wt/good" ]] || fail "chain discarded despite a reviewable commit"
}

test_open_limit_skip() {
  setup_case limit
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  printf '%s\n' a > "$wt/a"
  run_lib routine_worktree_publish 0 || fail "first publish failed"
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  printf '%s\n' b > "$wt/b"
  run_lib routine_worktree_publish 0 || fail "second publish failed"
  # two un-merged dated branches now open; a third create at the cap must refuse
  local rc=0
  wt=$(run_lib_env "ROUTINE_MAX_OPEN_BRANCHES=2" routine_worktree_create) || rc=$?
  [[ "$rc" -eq 3 ]] || fail "expected rc 3 at limit, got $rc"
  [[ ! -d "$ROUTINE_WT_ROOT_OVERRIDE/probe-nightly" ]] || fail "worktree created despite limit"
  local count
  count=$(git -C "$repo_root" for-each-ref --format='%(refname:short)' \
    'refs/heads/claude-routines/probe-nightly-*' | grep -c . || true)
  [[ "$count" -eq 2 ]] || fail "expected 2 open branches, got $count"
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
  local ba bb
  ba=$(head_branch "$wt_a")
  printf '%s\n' wip > "$wt_a/wip"
  # a concurrent run of a different group must not touch alpha's live worktree
  wt_b=$(run_lib_group beta routine_worktree_create) || fail "beta create failed"
  bb=$(head_branch "$wt_b")
  [[ -d "$wt_b" ]] || fail "beta worktree missing"
  [[ -f "$wt_a/wip" ]] || fail "sibling group's live worktree was destroyed"
  [[ "$wt_a" != "$wt_b" ]] || fail "groups share a worktree path"
  [[ "$ba" == claude-routines/alpha-* ]] || fail "alpha branch prefix wrong: $ba"
  [[ "$bb" == claude-routines/beta-* ]] || fail "beta branch prefix wrong: $bb"
}

test_no_change_no_commit() {
  setup_case nochange
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  local b1
  b1=$(head_branch "$wt")
  run_lib routine_worktree_publish 0 || fail "publish failed"
  # clean tree -> no commit; branch tip equals main
  [[ -z "$(git -C "$repo_root" rev-list main.."$b1")" ]] \
    || fail "empty run created a commit"
}

test_commit_body_handoff() {
  setup_case body
  wt=$(run_lib routine_worktree_create) || fail "create failed"
  local b1
  b1=$(head_branch "$wt")
  printf '%s\n' change > "$wt/changed"
  printf '%s\n' 'disposition table body' > "$wt/.routine-commit-body"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  git -C "$repo_root" log -1 --format=%b "$b1" | grep -q "disposition table body" \
    || fail "commit body missing the handoff content"
  git -C "$repo_root" show --name-only --format= "$b1" | grep -q '\.routine-commit-body' \
    && fail "handoff file entered the commit"
  git -C "$repo_root" show --name-only --format= "$b1" | grep -q '^changed$' \
    || fail "changed file not committed"
}

test_chain_sync_clean() {
  setup_case syncclean
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  printf '%s\n' night > "$wt/night-one"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  # main advances on a disjoint file -> the next chain run merges it cleanly
  advance_main main-later later
  wt=$(run_lib routine_worktree_create) || fail "second create failed"
  [[ -f "$wt/night-one" ]] || fail "chain content lost by sync"
  [[ -f "$wt/main-later" ]] || fail "main's commit not merged into the chain run"
  git -C "$wt" merge-tree --write-tree main HEAD >/dev/null \
    || fail "synced branch not mergeable into main"
}

test_chain_sync_conflict_resolved() {
  setup_case syncfix
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  printf '%s\n' chain-side > "$wt/base"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  # main edits the same file differently -> the sync merge conflicts and the
  # stub claude stands in for /merge-resolve: it re-merges and concludes the
  # conflicted merge with a resolution
  advance_main base main-side
  local stubdir="$TMP/syncfix-stub"
  mkdir -p "$stubdir"
  cat > "$stubdir/claude" <<'STUB'
#!/usr/bin/env bash
git merge --no-edit --quiet main >/dev/null 2>&1 || {
  printf '%s\n' resolved > base
  git add base
  git commit --no-edit -q
}
STUB
  chmod +x "$stubdir/claude"
  wt=$(run_lib_stub "$stubdir" routine_worktree_create) || fail "conflicted create failed"
  [[ "$(cat "$wt/base")" == "resolved" ]] || fail "stub resolution not in the worktree"
  git -C "$wt" merge-tree --write-tree main HEAD >/dev/null \
    || fail "resolved branch not mergeable into main"
  [[ -z "$(git -C "$wt" status --porcelain)" ]] || fail "worktree left dirty"
}

test_chain_sync_conflict_unresolved() {
  setup_case syncbad
  wt=$(run_lib routine_worktree_create) || fail "first create failed"
  local b1
  b1=$(head_branch "$wt")
  printf '%s\n' chain-side > "$wt/base"
  run_lib routine_worktree_publish 0 || fail "publish failed"
  advance_main base main-side
  # the stub claude does nothing -> the gate must fail the create, leaving the
  # chain untouched and no half-made branch behind
  local stubdir="$TMP/syncbad-stub"
  mkdir -p "$stubdir"
  printf '%s\n' '#!/usr/bin/env bash' 'exit 0' > "$stubdir/claude"
  chmod +x "$stubdir/claude"
  local rc=0
  wt2=$(run_lib_stub "$stubdir" routine_worktree_create) || rc=$?
  [[ "$rc" -eq 1 ]] || fail "expected rc 1 on unresolved sync, got $rc"
  [[ ! -d "$ROUTINE_WT_ROOT_OVERRIDE/probe-nightly" ]] || fail "worktree survived failed sync"
  git -C "$repo_root" rev-parse --quiet --verify "refs/heads/$b1" >/dev/null \
    || fail "chain branch lost on failed sync"
  local count
  count=$(git -C "$repo_root" for-each-ref --format='%(refname:short)' \
    'refs/heads/claude-routines/probe-nightly-*' | grep -c . || true)
  [[ "$count" -eq 1 ]] || fail "failed sync left an extra branch (expected 1, got $count)"
}

test_dated_branch_name
test_same_day_collision
test_base_off_latest
test_reset_after_merge
test_failed_chain_discard
test_real_commit_survives_among_failures
test_open_limit_skip
test_stale_worktree_sweep
test_sibling_group_survival
test_no_change_no_commit
test_commit_body_handoff
test_chain_sync_clean
test_chain_sync_conflict_resolved
test_chain_sync_conflict_unresolved
echo "OK"
