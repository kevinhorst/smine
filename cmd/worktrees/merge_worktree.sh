#!/usr/bin/env bash
# Merge one claude/* worktree branch into the current (feature) branch.
#
# Usage:
#   merge_worktree.sh <claude/branch>
#
# Refuses on a dirty checkout. A conflict aborts the merge and restores the
# pre-merge state (CONFLICT line, exit 1) — same semantics as sync_worktrees.sh.

set -uo pipefail

branch="${1:-}"
if [ -z "$branch" ]; then
  echo "usage: $(basename "$0") <claude/branch>"
  exit 1
fi

if ! git show-ref --verify --quiet "refs/heads/$branch"; then
  echo "error: unknown branch '$branch'"
  exit 1
fi

dirty=$(git status --porcelain | grep -cv '^??' || true)
if [ "$dirty" -ne 0 ]; then
  echo "error: checkout is dirty ($dirty uncommitted change(s)) — commit or stash first"
  exit 1
fi

target=$(git rev-parse --abbrev-ref HEAD)
if git merge --no-edit "$branch"; then
  echo "merged $branch into $target"
else
  git merge --abort 2>/dev/null || true
  echo "CONFLICT $branch (merge aborted — $target restored)"
  exit 1
fi
