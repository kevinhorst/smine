#!/usr/bin/env bash
# UserPromptSubmit tripwire: block the prompt (exit 2) when the session sits
# in a broken .claude/worktrees/ directory — one with no .git entry, or a
# .git file whose gitdir target is gone. In such a directory git commands
# would either fall through to the MAIN checkout (hijacking its HEAD) or die
# against the pool decoy; either way the session cannot work correctly.
#
# Detect and block only — repair is deliberately out of scope. Creation and
# hygiene live in worktree-sessionstart.sh / install_pool_guard.sh.
#
# Runs on every prompt in every session (UserPromptSubmit ignores matchers),
# so the happy path uses only bash builtins and stat checks — no process
# spawns, no git.

WORKTREE_DIR=""
MAIN_REPO=""
check="$PWD"
while [[ "$check" == /*/* ]]; do
  parent="${check%/*}"
  grandparent="${parent%/*}"
  if [[ "${parent##*/}" == "worktrees" && "${grandparent##*/}" == ".claude" ]]; then
    MAIN_REPO="${grandparent%/*}"
    WORKTREE_DIR="$check"
    break
  fi
  check="$parent"
done

[[ -n "$WORKTREE_DIR" ]] || exit 0

if [[ ! -e "$WORKTREE_DIR/.git" ]]; then
  {
    echo "worktree-guard: BLOCKED — $WORKTREE_DIR is not a git worktree (no .git entry)."
    echo "Git commands here would not operate on this directory's branch."
    echo "Start the session in a valid worktree, or attach this one manually:"
    echo "  git -C $MAIN_REPO worktree add $WORKTREE_DIR <branch>"
  } >&2
  exit 2
fi

if [[ -f "$WORKTREE_DIR/.git" ]]; then
  IFS= read -r line < "$WORKTREE_DIR/.git"
  gitdir="${line#gitdir: }"
  [[ "$gitdir" == /* ]] || gitdir="$WORKTREE_DIR/$gitdir"
  if [[ ! -d "$gitdir" ]]; then
    {
      echo "worktree-guard: BLOCKED — $WORKTREE_DIR/.git points at missing gitdir $gitdir."
      echo "The worktree registration was pruned; the tree may still hold real work."
      echo "Inspect manually (git -C $MAIN_REPO worktree repair?) before continuing."
    } >&2
    exit 2
  fi
fi

exit 0
