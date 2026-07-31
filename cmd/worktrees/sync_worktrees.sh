#!/usr/bin/env bash
# Merge the current (feature) branch into claude/* agent worktrees.
#
# Usage:
#   sync_worktrees.sh                 merge into every claude/* worktree
#   sync_worktrees.sh <n> [<n> ...]   merge only into the numbered worktrees
#   sync_worktrees.sh <claude/branch> merge only into the named branch's worktree
#
# Numbers match the rows printed by print_agent_worktrees_status.sh.
#
# Workflow: when one reviewer's fix has been cherry-picked onto the feature
# branch, the other worktrees are stale. This merges the feature branch into
# each of them so they pick up the fix.
#
# The feature branch is the currently checked-out branch. Each worktree is
# merged in place (git merge --no-edit) on its own branch. A failure on one
# worktree never aborts the rest.
#
# A worktree is SKIPPED when:
#   - it has no worktree checked out
#   - it is the feature branch itself
#   - it has uncommitted changes (dirty) — merging could clobber work
#   - it is already up to date with the feature branch
#   - the merge hits conflicts — the merge is aborted and left for you to
#     resolve manually (the worktree is restored to its pre-merge state)

set -uo pipefail

target=$(git rev-parse --abbrev-ref HEAD)

branches=()
while IFS= read -r b; do
  branches+=("$b")
done < <(git for-each-ref --format='%(refname:short)' refs/heads/claude/)

if [ ${#branches[@]} -eq 0 ]; then
  echo "No claude/* branches found."
  exit 0
fi

# --- Resolve which branches to sync (all, or the numbered selection) ---
selected=()
if [ "$#" -eq 0 ]; then
  selected=("${branches[@]}")
else
  for num in "$@"; do
    if [[ "$num" == claude/* ]]; then
      found=0
      for b in "${branches[@]}"; do
        if [ "$b" = "$num" ]; then
          selected+=("$b")
          found=1
          break
        fi
      done
      if [ "$found" -eq 0 ]; then
        echo "error: unknown branch '$num'"
        exit 1
      fi
      continue
    fi
    if ! [[ "$num" =~ ^[0-9]+$ ]]; then
      echo "error: '$num' is not a number or claude/* branch"
      exit 1
    fi
    idx=$((num - 1))
    if [ "$idx" -lt 0 ] || [ "$idx" -ge "${#branches[@]}" ]; then
      echo "error: #$num is out of range (1-${#branches[@]})"
      exit 1
    fi
    selected+=("${branches[$idx]}")
  done
fi

echo "feature branch: $target"
echo ""

for branch in "${selected[@]}"; do
  worktree=$(git worktree list --porcelain | awk -v b="refs/heads/$branch" '/^worktree /{p=$2} $0=="branch "b{print p}')

  if [ -z "$worktree" ]; then
    echo "skip  $branch (no worktree)"
    continue
  fi

  if [ "$branch" = "$target" ]; then
    echo "skip  $branch (is the feature branch)"
    continue
  fi

  dirty=$(git -C "$worktree" status --porcelain | grep -cv '^??' || true)
  if [ "$dirty" -ne 0 ]; then
    echo "skip  $branch (dirty: $dirty uncommitted change(s))"
    continue
  fi

  behind=$(git rev-list --count "$branch..$target" 2>/dev/null || echo 0)
  if [ "$behind" -eq 0 ]; then
    echo "ok    $branch (already up to date)"
    continue
  fi

  if git -C "$worktree" merge --no-edit "$target" >/dev/null 2>&1; then
    echo "merge $branch (+$behind commit(s) from $target)"
  else
    git -C "$worktree" merge --abort 2>/dev/null || true
    echo "CONFLICT $branch (merge aborted — resolve manually in $worktree)"
  fi
done

echo ""
echo "done. Re-run print_agent_worktrees_status.sh to verify."
