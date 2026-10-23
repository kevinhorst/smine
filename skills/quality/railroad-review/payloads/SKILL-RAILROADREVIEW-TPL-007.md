$SKILL_RAILROADREVIEW_CLEANUP cleanup IS the sequence per worktree: reverse overlay, archive commits since head (commits.diff),
archive the tracked delta (tracked.diff), move untracked and ignored bytes to salvage, restore, git worktree remove
without --force; then each run branch's diff is saved before git branch -D. One JSON report; a failed salvage,
"removed": false and a skipped branch are skipped rows — reported defects, never retried, never a normal outcome;
the other worktrees go on. $SKILL_RAILROADREVIEW_RESET_ROUND reset-round runs the same sequence for one round.
