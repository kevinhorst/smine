#!/usr/bin/env bash
# Cherry-pick one commit onto the current (feature) branch.
#
# Usage:
#   cherry_pick_worktree.sh <sha>
#
# Refuses on a dirty checkout. A conflict aborts the cherry-pick and restores
# the pre-op state (CONFLICT line, exit 1).

set -uo pipefail

sha="${1:-}"
if [ -z "$sha" ]; then
  echo "usage: $(basename "$0") <sha>"
  exit 1
fi

if ! git rev-parse --verify --quiet "$sha^{commit}" >/dev/null; then
  echo "error: unknown commit '$sha'"
  exit 1
fi

dirty=$(git status --porcelain | grep -cv '^??' || true)
if [ "$dirty" -ne 0 ]; then
  echo "error: checkout is dirty ($dirty uncommitted change(s)) — commit or stash first"
  exit 1
fi

target=$(git rev-parse --abbrev-ref HEAD)
if git cherry-pick "$sha"; then
  echo "picked $sha onto $target"
else
  git cherry-pick --abort 2>/dev/null || true
  echo "CONFLICT $sha (cherry-pick aborted — $target restored)"
  exit 1
fi
