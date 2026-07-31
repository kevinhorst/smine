#!/usr/bin/env bash
# Worktree/branch plumbing shared by routine wrappers. Sourced by
# routines/<name>/run.sh after the token gate and self-flock; requires git on
# PATH and the caller's $repo_root and $routine_dir. All state lives on the
# group branch routine/$ROUTINE_GROUP — the checkout's working tree is never
# touched.
#
# Groups: routines that form one chain share a group (one worktree, one
# branch) and serialize on the group lock; independent routines keep the
# default group (their own name) and run concurrently with every other group.
# Nothing ever touches a sibling group's worktree.
#
# Local contract: routine output is committed on the group branch locally — no
# fetch, no push, no PR, no gh. Reviewing and locally merging the branch closes
# a run; resetting it to main discards one.

ROUTINE_GROUP="${ROUTINE_GROUP:-$(basename "$routine_dir")}"
ROUTINE_BRANCH="${ROUTINE_BRANCH:-routine/$ROUTINE_GROUP}"
ROUTINE_WT_ROOT="${ROUTINE_WT_ROOT:-$HOME/.cache/claude-routine/worktrees}"

# Serializes members of one group (shared branch + worktree) for the whole
# run; the lock is held on fd 7 until the wrapper process exits. Different
# groups never contend. Returns 1 on timeout (2h).
routine_group_lock() {
  mkdir -p "$ROUTINE_WT_ROOT"
  exec 7>"$ROUTINE_WT_ROOT/.$ROUTINE_GROUP.lock"
  flock -w 7200 7
}

# True when the branch holds ≥1 commit main lacks and every one is a failure
# commit (subject carries the [failed] marker from routine_worktree_publish).
# Such a branch has no reviewable output, so create resets it to main rather
# than stacking the next run on top of dead failed runs. A single real (or
# manual merge) commit among them makes this false — the branch is kept.
routine_branch_only_failures() {
  local subjects
  subjects="$(git -C "$repo_root" log --format=%s "main..$ROUTINE_BRANCH" 2>/dev/null)" || return 1
  [[ -n "$subjects" ]] || return 1
  ! grep -qv '\[failed\]' <<<"$subjects"
}

# Resets or reuses the group branch and checks it out into a fresh worktree.
# Reuse iff the branch holds a reviewable commit main lacks; merged (= ancestor
# of main), missing, or holding only failed runs -> reset to main. Removes only
# the group's own stale worktree (leftover of a failed prior run) — never a
# sibling group's. Echoes the worktree path; returns 1 on failure.
routine_worktree_create() {
  local wt
  wt="$ROUTINE_WT_ROOT/$ROUTINE_GROUP"

  if [[ -e "$wt" ]]; then
    git -C "$repo_root" worktree remove --force "$wt" >/dev/null 2>&1 || rm -rf "$wt"
  fi
  git -C "$repo_root" worktree prune

  if ! git -C "$repo_root" rev-parse --quiet --verify "$ROUTINE_BRANCH" >/dev/null \
      || git -C "$repo_root" merge-base --is-ancestor "$ROUTINE_BRANCH" main \
      || routine_branch_only_failures; then
    git -C "$repo_root" branch -f --no-track "$ROUTINE_BRANCH" main || return 1
  fi

  mkdir -p "$ROUTINE_WT_ROOT"
  git -C "$repo_root" worktree add "$wt" "$ROUTINE_BRANCH" >/dev/null || return 1
  printf '%s\n' "$wt"
}

# Commits whatever the claude run left behind on the group branch, then
# removes the worktree. $1 = claude exit status. A .routine-commit-body file
# at the worktree root becomes the commit message body and never enters the
# commit. No push, no PR — local merge is the acceptance act. On failure the
# worktree is kept for debugging (the group's next create sweeps it).
routine_worktree_publish() {
  local name wt claude_exit="$1" rc=0 body_file body=""
  name="$(basename "$routine_dir")"
  wt="$ROUTINE_WT_ROOT/$ROUTINE_GROUP"

  body_file="$wt/.routine-commit-body"
  if [[ -f "$body_file" ]]; then
    body="$(cat "$body_file")"
    rm -f "$body_file"
  fi

  # A non-zero claude exit marks the commit subject with [failed] so the next
  # create can reset a branch that holds only failed runs (see
  # routine_branch_only_failures) instead of letting them block review forever.
  local marker=""
  [[ "$claude_exit" -ne 0 ]] && marker="[failed] "
  local subject="sessions: ${marker}Recorded nightly $name output ($(date -u +%F), exit $claude_exit)"

  if [[ -n "$(git -C "$wt" status --porcelain)" ]]; then
    git -C "$wt" add -A
    if [[ -n "$body" ]]; then
      git -C "$wt" commit --quiet -m "$subject" -m "$body" || rc=1
    else
      git -C "$wt" commit --quiet -m "$subject" || rc=1
    fi
  fi

  if [[ "$rc" -eq 0 ]]; then
    git -C "$repo_root" worktree remove --force "$wt" >/dev/null 2>&1 || true
  fi
  return "$rc"
}
