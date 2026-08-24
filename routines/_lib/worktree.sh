#!/usr/bin/env bash
# Worktree/branch plumbing shared by routine wrappers. Sourced by
# routines/<name>/run.sh after the token gate and self-flock; requires git on
# PATH and the caller's $repo_root and $routine_dir. Each run's state lives on
# a fresh dated branch claude-routines/$ROUTINE_GROUP-<UTC-date> — the
# checkout's working tree is never touched.
#
# Groups: routines that form one chain share a group (one worktree path, one
# branch lineage) and serialize on the group lock; independent routines keep the
# default group (their own name) and run concurrently with every other group.
# Nothing ever touches a sibling group's worktree or branches.
#
# Local contract: routine output is committed on the run's dated branch locally
# — no fetch, no push, no PR, no gh. Reviewing and locally merging the newest
# branch closes a run and prunes its merged ancestors; discarding a run is
# deleting its branch.

ROUTINE_GROUP="${ROUTINE_GROUP:-$(basename "$routine_dir")}"
ROUTINE_BRANCH_PREFIX="${ROUTINE_BRANCH_PREFIX:-claude-routines/$ROUTINE_GROUP}"
ROUTINE_WT_ROOT="${ROUTINE_WT_ROOT:-$HOME/.cache/claude-routine/worktrees}"
# Cap on un-merged dated branches for the group; empty = unlimited. At the cap
# routine_worktree_create returns 3 and mints nothing (caller skips the run).
ROUTINE_MAX_OPEN_BRANCHES="${ROUTINE_MAX_OPEN_BRANCHES:-}"

# Serializes members of one group (shared branch + worktree) for the whole
# run; the lock is held on fd 7 until the wrapper process exits. Different
# groups never contend. Returns 1 on timeout (2h).
routine_group_lock() {
  [ "${ROUTINE_LOCKS_HELD:-0}" = "1" ] && return 0
  mkdir -p "$ROUTINE_WT_ROOT"
  exec 7>"$ROUTINE_WT_ROOT/.$ROUTINE_GROUP.lock"
  flock -w 7200 7
}

# True when $1 holds ≥1 commit main lacks and every one is a failure commit
# (subject carries the [failed] marker from routine_worktree_publish). Such a
# chain has no reviewable output, so create discards it and bases the next run
# on main rather than stacking on dead failed runs. A single real (or manual
# merge) commit among them makes this false — the chain is kept.
routine_branch_only_failures() {
  local subjects
  subjects="$(git -C "$repo_root" log --format=%s "main..$1" 2>/dev/null)" || return 1
  [[ -n "$subjects" ]] || return 1
  ! grep -qv '\[failed\]' <<<"$subjects"
}

# Merges current local main into the run's worktree so the run's result
# descends from main-as-of-run-start — the later local merge into main is then
# conflict-free unless main moved after the run. Clean merges cost nothing; a
# conflicted merge is aborted and handed to an unattended /merge-resolve
# claude run in the worktree, then gated: the tree must be clean and
# `git merge-tree --write-tree main HEAD` conflict-free. Returns 1 when the
# sync cannot produce a mergeable state. routine_run_claude /
# routine_allowed_tools are used when the caller sourced platform.sh/skill.sh
# (every run.sh does); a bare `claude` keeps the lib sourceable standalone.
routine_chain_sync_main() {
  local wt="$1" tools prompt
  if git -C "$wt" merge --no-edit --quiet main >/dev/null 2>&1; then
    return 0
  fi
  git -C "$wt" merge --abort >/dev/null 2>&1 || true

  echo "chain diverged from main with conflicts; running unattended /merge-resolve" >&2
  prompt="/merge-resolve theirs main. Unattended routine run: never ask a question; decide every judgement call yourself."
  tools=""
  [ "$(type -t routine_allowed_tools)" = "function" ] && tools="$(routine_allowed_tools merge-resolve)"
  local claude_cmd=(claude -p "$prompt")
  [[ -n "$tools" ]] && claude_cmd+=(--allowedTools "$tools")
  claude_cmd+=(
    --model "${ROUTINE_MODEL:-claude-opus-4-8[1m]}"
    --effort low
    --permission-mode "${ROUTINE_PERMISSION_MODE:-acceptEdits}"
    --max-budget-usd "${ROUTINE_MAX_BUDGET_USD:-15}"
    --output-format json
  )
  if [ "$(type -t routine_run_claude)" = "function" ]; then
    ( cd "$wt" && routine_run_claude 3600 "${claude_cmd[@]}" ) >/dev/null || true
  else
    ( cd "$wt" && "${claude_cmd[@]}" ) >/dev/null 2>&1 || true
  fi

  if [[ -n "$(git -C "$wt" status --porcelain)" ]]; then
    echo "merge-resolve left the worktree dirty; sync failed" >&2
    return 1
  fi
  if ! git -C "$wt" merge-tree --write-tree main HEAD >/dev/null 2>&1; then
    echo "chain still conflicts with main after merge-resolve; sync failed" >&2
    return 1
  fi
  return 0
}

# Dated group branches, newest commit first (the chain tip is line 1). The
# dated suffix carries no slash, so the refs/heads glob matches cleanly.
routine_group_branches() {
  git -C "$repo_root" for-each-ref --sort=-committerdate \
    --format='%(refname:short)' "refs/heads/$ROUTINE_BRANCH_PREFIX-*"
}

# Delete every dated group branch already merged into main (an accepted run).
# Never touches a checked-out branch — create removed the group's own worktree
# first, and sibling groups carry a different prefix.
routine_prune_merged() {
  local branch
  while IFS= read -r branch; do
    [[ -n "$branch" ]] || continue
    if git -C "$repo_root" merge-base --is-ancestor "$branch" main; then
      git -C "$repo_root" branch -D "$branch" >/dev/null 2>&1 || true
    fi
  done < <(routine_group_branches)
}

# Mints a fresh dated branch for this run and checks it out into the group
# worktree. Never reuses a branch: the run stacks on the newest un-merged dated
# branch (chain tip) so votes, proposal edits, and archives accumulate linearly
# — or on main when none survives. A chain-based run then syncs with main
# (routine_chain_sync_main) so every run ends mergeable into main as of run
# start. Prunes merged branches first; an all-[failed]
# chain is discarded and the run starts from main. Honors
# ROUTINE_MAX_OPEN_BRANCHES: at the cap it mints nothing and returns 3. Removes
# only the group's own stale worktree — never a sibling group's. Echoes the
# worktree path; returns 1 on failure, 3 on limit.
routine_worktree_create() {
  local wt base tip day new count branch n
  wt="$ROUTINE_WT_ROOT/$ROUTINE_GROUP"

  if [[ -e "$wt" ]]; then
    git -C "$repo_root" worktree remove --force "$wt" >/dev/null 2>&1 || rm -rf "$wt"
  fi
  git -C "$repo_root" worktree prune

  routine_prune_merged

  base="main"
  tip="$(routine_group_branches | head -n1)"
  if [[ -n "$tip" ]]; then
    if routine_branch_only_failures "$tip"; then
      while IFS= read -r branch; do
        [[ -n "$branch" ]] && git -C "$repo_root" branch -D "$branch" >/dev/null 2>&1 || true
      done < <(routine_group_branches)
    else
      base="$tip"
    fi
  fi

  if [[ -n "$ROUTINE_MAX_OPEN_BRANCHES" ]]; then
    count="$(routine_group_branches | grep -c . || true)"
    if [[ "$count" -ge "$ROUTINE_MAX_OPEN_BRANCHES" ]]; then
      return 3
    fi
  fi

  day="$(date -u +%F)"
  new="$ROUTINE_BRANCH_PREFIX-$day"
  n=2
  while git -C "$repo_root" rev-parse --quiet --verify "refs/heads/$new" >/dev/null; do
    new="$ROUTINE_BRANCH_PREFIX-$day-$n"
    n=$((n + 1))
  done

  mkdir -p "$ROUTINE_WT_ROOT"
  git -C "$repo_root" worktree add --quiet -b "$new" "$wt" "$base" || return 1

  # A chain-based run syncs with main before anything else touches the tree,
  # so the run's result stays mergeable into main-as-of-run-start. On sync
  # failure the fresh branch and worktree are discarded — the chain is left
  # exactly as found and the run is recorded as failed by the caller.
  if [[ "$base" != "main" ]] && ! routine_chain_sync_main "$wt"; then
    git -C "$repo_root" worktree remove --force "$wt" >/dev/null 2>&1 || rm -rf "$wt"
    git -C "$repo_root" branch -D "$new" >/dev/null 2>&1 || true
    return 1
  fi

  printf '%s\n' "$wt"
}

# Commits whatever the claude run left behind on the run's dated branch, then
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
  # create can discard a chain that holds only failed runs (see
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
