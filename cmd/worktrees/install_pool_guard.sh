#!/usr/bin/env bash
# Install the worktree-pool decoy into one or more repositories.
#
# Claude Desktop pools session worktree directories under
# <repo>/.claude/worktrees/ and has handed out "hollow" pool dirs (no .git
# entry) to new sessions. Git commands inside a hollow dir resolve upward to
# the MAIN checkout and hijack its HEAD. The decoy is a .git FILE at the pool
# root with a dead gitdir pointer: git's upward discovery stops there and
# every command in a hollow dir fails loudly (exit 128) instead of reaching
# the main checkout. Real worktrees are unaffected — their own .git file is
# found before the decoy.
#
# Usage:
#   install_pool_guard.sh [<repo> ...]
#
# Without arguments the repository containing the current directory is used
# (resolved via the git common dir, so it works from inside a worktree).
# Idempotent. A foreign .git entry at the pool root is never replaced.

set -uo pipefail

DECOY_POINTER="gitdir: /claude-pool-guard-see-smine-cmd-worktrees"

repos=("$@")
if [ ${#repos[@]} -eq 0 ]; then
  common=$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null) || {
    echo "error: not inside a git repository and no repo argument given" >&2
    exit 1
  }
  repo="${common%/.git}"
  if [ "$repo" = "$common" ]; then
    echo "error: cannot derive main checkout from $common; pass the repo path" >&2
    exit 1
  fi
  repos=("$repo")
fi

failed=0
for repo in "${repos[@]}"; do
  repo=$(cd -- "$repo" 2>/dev/null && pwd -P) || {
    echo "skip  $repo: no such directory" >&2
    failed=1
    continue
  }
  if [ ! -d "$repo/.git" ]; then
    echo "skip  $repo: not a main git checkout (.git dir missing)" >&2
    failed=1
    continue
  fi

  pool="$repo/.claude/worktrees"
  decoy="$pool/.git"
  mkdir -p "$pool"

  if [ -e "$decoy" ] || [ -L "$decoy" ]; then
    if [ -f "$decoy" ] && [ ! -L "$decoy" ] && [ "$(<"$decoy")" = "$DECOY_POINTER" ]; then
      echo "ok    $repo (decoy already installed)"
      continue
    fi
    echo "skip  $repo: unexpected $decoy exists — not replacing" >&2
    failed=1
    continue
  fi

  printf '%s\n' "$DECOY_POINTER" > "$decoy"
  echo "installed  $decoy"
done

exit "$failed"
