#!/usr/bin/env bash
# Regression coverage for remove_agent_worktrees.sh targeting: a targeted run
# removes exactly the target branch's own worktree — even when the pool has
# recycled the directory so its name matches a DIFFERENT branch — and a run
# that matches nothing says so instead of ending silently.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/wt-remove.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == wt-remove.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT="$REPO_DIR/cmd/worktrees/remove_agent_worktrees.sh"

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

test_targeted_remove_follows_branch_not_dir_name() {
  local repo=$TMP/repo pool out
  init_repo "$repo"
  pool="$repo/.claude/worktrees"

  # claude/alpha lives in a dir named after beta (pool-recycled naming);
  # claude/beta lives elsewhere. Removing alpha must delete alpha's dir and
  # never touch beta's, regardless of the misleading names.
  git -C "$repo" branch claude/alpha
  git -C "$repo" branch claude/beta
  git -C "$repo" worktree add -q "$pool/beta-a613d6" claude/alpha
  git -C "$repo" worktree add -q "$pool/gallant-raman" claude/beta

  out=$(cd "$repo" && bash "$SCRIPT" claude/alpha)

  echo "$out" | grep -qF "removed: $pool/beta-a613d6" \
    || fail "output does not name alpha's real dir: $out"
  [ ! -d "$pool/beta-a613d6" ] || fail "alpha's worktree dir survived"
  [ -f "$pool/gallant-raman/base" ] || fail "beta's worktree was deleted"
  git -C "$repo" worktree list --porcelain | grep -qF "worktree $pool/gallant-raman" \
    || fail "beta's worktree lost its registration"
  git -C "$repo" show-ref -q --verify refs/heads/claude/alpha \
    || fail "branch was deleted (worktrees only, never branches)"
}

test_targeted_no_worktree_is_reported() {
  local repo=$TMP/repo out
  git -C "$repo" branch claude/ghost
  out=$(cd "$repo" && bash "$SCRIPT" claude/ghost)
  echo "$out" | grep -qF "no worktree checked out for claude/ghost" \
    || fail "targeted no-op ended silently: $out"
}

test_delete_branch_requires_target() {
  local repo=$TMP/repo-db-usage out
  init_repo "$repo"
  if out=$(cd "$repo" && bash "$SCRIPT" --delete-branch 2>&1); then
    fail "--delete-branch without target succeeded"
  fi
  echo "$out" | grep -q "requires a branch target" \
    || fail "missing usage error for target-less --delete-branch: $out"
}

test_delete_branch_removes_contained_branch() {
  local repo=$TMP/repo-db-safe out
  init_repo "$repo"
  # branch tip == main tip → contained, clean worktree → safe
  git -C "$repo" branch claude/done
  git -C "$repo" worktree add -q "$repo/.claude/worktrees/done" claude/done

  out=$(cd "$repo" && bash "$SCRIPT" --delete-branch claude/done)

  echo "$out" | grep -qF "deleted branch: claude/done" || fail "branch delete not reported: $out"
  if git -C "$repo" show-ref -q --verify refs/heads/claude/done; then
    fail "branch survived --delete-branch"
  fi
  [ ! -d "$repo/.claude/worktrees/done" ] || fail "worktree survived"
}

test_delete_branch_skips_uncontained_without_force() {
  local repo=$TMP/repo-db-unsafe out
  init_repo "$repo"
  git -C "$repo" branch claude/wip
  git -C "$repo" worktree add -q "$repo/.claude/worktrees/wip" claude/wip
  # a commit that lives on no non-claude branch → unsafe
  printf '%s\n' wip > "$repo/.claude/worktrees/wip/wip"
  git -C "$repo/.claude/worktrees/wip" add wip
  git -C "$repo/.claude/worktrees/wip" commit -qm wip

  out=$(cd "$repo" && bash "$SCRIPT" --delete-branch claude/wip)

  echo "$out" | grep -q "skipped branch: claude/wip" || fail "unsafe branch delete not skipped: $out"
  git -C "$repo" show-ref -q --verify refs/heads/claude/wip \
    || fail "unsafe branch was deleted without --force"

  out=$(cd "$repo" && bash "$SCRIPT" --delete-branch --force claude/wip)

  echo "$out" | grep -qF "deleted branch: claude/wip" || fail "forced branch delete not reported: $out"
  if git -C "$repo" show-ref -q --verify refs/heads/claude/wip; then
    fail "branch survived --delete-branch --force"
  fi
  [ ! -d "$repo/.claude/worktrees/wip" ] || fail "worktree survived --force"
}

# A branch whose only work landed on FROM as a conflict-resolved pick is safe
# via the shared applied-probe — removed without --force (proves the remove
# script and the status overview share one verdict source).
build_resolved_branch() {
  local repo=$1
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  for i in $(seq 1 20); do echo "line-$i shared content block number $i here"; done > "$repo/doc.md"
  git -C "$repo" add doc.md
  git -C "$repo" commit -qm base

  git -C "$repo" branch claude/resolved main
  git -C "$repo" checkout -q claude/resolved
  { for i in $(seq 1 9); do echo "line-$i shared content block number $i here"; done
    echo "AGENT-ADDITION-10 with extra explanation text appended by agent"
    echo "AGENT-ADDITION-11 with extra explanation text appended by agent"
    for i in $(seq 12 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "agent change lines 10-11"
  git -C "$repo" checkout -q main
  { for i in $(seq 1 8); do echo "line-$i shared content block number $i here"; done
    echo "line-9 REWORDED-ON-MAIN independently before the pick"
    for i in $(seq 10 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "main rewords line 9"
  { for i in $(seq 1 8); do echo "line-$i shared content block number $i here"; done
    echo "line-9 REWORDED-ON-MAIN independently before the pick"
    echo "AGENT-ADDITION-10 with extra explanation text appended by agent"
    echo "AGENT-ADDITION-11 with extra explanation text appended by agent"
    for i in $(seq 12 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "main lands agent change (resolved pick)"
  git -C "$repo" checkout -q main
  git -C "$repo" worktree add -q "$repo/.claude/worktrees/resolved" claude/resolved
}

test_probe_upgraded_branch_removed_without_force() {
  local repo=$TMP/repo-probe out
  build_resolved_branch "$repo"
  out=$(cd "$repo" && bash "$SCRIPT" claude/resolved)
  echo "$out" | grep -qF "removed: $repo/.claude/worktrees/resolved" \
    || fail "probe-upgraded branch not removed without --force: $out"
  [ ! -d "$repo/.claude/worktrees/resolved" ] || fail "worktree survived"
}

# A competing change to the same region that never landed on FROM stays unsafe
# — the probe must not false-pass it, so removal is skipped without --force.
test_competing_change_branch_skipped() {
  local repo=$TMP/repo-compete out
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  for i in $(seq 1 20); do echo "line-$i shared content block number $i here"; done > "$repo/doc.md"
  git -C "$repo" add doc.md
  git -C "$repo" commit -qm base

  git -C "$repo" branch claude/negative main
  git -C "$repo" checkout -q claude/negative
  { for i in $(seq 1 9); do echo "line-$i shared content block number $i here"; done
    echo "AGENT-VERSION-10 with extra explanation text appended here now"
    for i in $(seq 11 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "agent competing change line 10"
  git -C "$repo" checkout -q main
  { for i in $(seq 1 9); do echo "line-$i shared content block number $i here"; done
    echo "MAIN-VERSION-10 with extra explanation text appended here now"
    for i in $(seq 11 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "main competing change line 10"
  git -C "$repo" checkout -q main
  git -C "$repo" worktree add -q "$repo/.claude/worktrees/negative" claude/negative

  out=$(cd "$repo" && bash "$SCRIPT" claude/negative)
  echo "$out" | grep -q "skipped:.*commits exist on no non-claude branch" \
    || fail "competing change wrongly deemed safe: $out"
  [ -d "$repo/.claude/worktrees/negative" ] || fail "unsafe worktree was removed"
}

# claude-routines/* lineages are valid targets: a contained routine branch's
# worktree is removed and the branch deleted; the retired routine/* prefix is
# rejected with the usage error.
test_routine_namespace_target() {
  local repo=$TMP/repo-routines out
  init_repo "$repo"

  git -C "$repo" branch claude-routines/nightly-2026-08-12
  git -C "$repo" worktree add -q "$repo/.routine-wt" claude-routines/nightly-2026-08-12

  out=$(cd "$repo" && bash "$SCRIPT" --delete-branch claude-routines/nightly-2026-08-12)
  echo "$out" | grep -qF "removed: $repo/.routine-wt" \
    || fail "routine worktree not removed: $out"
  echo "$out" | grep -qF "deleted branch: claude-routines/nightly-2026-08-12" \
    || fail "routine branch not deleted: $out"

  if out=$(cd "$repo" && bash "$SCRIPT" routine/legacy 2>&1); then
    fail "retired routine/* prefix accepted: $out"
  fi
  echo "$out" | grep -q '^usage:' || fail "wrong error for routine/* target: $out"
}

test_routine_namespace_target
test_targeted_remove_follows_branch_not_dir_name
test_targeted_no_worktree_is_reported
test_delete_branch_requires_target
test_delete_branch_removes_contained_branch
test_delete_branch_skips_uncontained_without_force
test_probe_upgraded_branch_removed_without_force
test_competing_change_branch_skipped

echo "PASS: remove_agent_worktrees.sh targeting"
