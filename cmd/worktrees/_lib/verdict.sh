#!/usr/bin/env bash
# Shared verdict logic for agent-branch safety: candidate discovery, FROM
# resolution, cherry sets, the layered applied-probe
# (picked | applied | applied-resolved | unpicked), and the containment list
# both print_agent_worktrees_status.sh and remove_agent_worktrees.sh derive
# Safe from.
#
# Sourced, not executed. Callers run under `set -euo pipefail`, call
# load_candidates once, and register `trap verdict_cleanup EXIT` before the
# first evaluate_branch call (the probe worktree is created lazily).
#
# bash 3.2: parallel indexed arrays throughout, no associative arrays.

# All local non-claude branches — where harvested work could live.
candidates=()
load_candidates() {
  local candidate_branch
  candidates=()
  while IFS= read -r candidate_branch; do
    case "$candidate_branch" in claude/*) continue ;; esac
    candidates+=("$candidate_branch")
  done < <(git for-each-ref --format='%(refname:short)' refs/heads/)
}

# Branch the agent branch was created from, from the branch reflog: a local
# or remote-tracking ref (Desktop records the full refname, e.g.
# refs/remotes/origin/main). Prints nothing when the reflog records no
# resolvable origin — deterministic or unknown, never a guess.
resolve_from() {
  local branch=$1 created short ref
  created=$(git reflog show --format='%gs' "$branch" 2>/dev/null | tail -1 |
    sed -n 's/^branch: Created from \(.*\)$/\1/p')
  short=${created#refs/remotes/}
  short=${short#refs/heads/}
  case "$short" in
    '' | HEAD | claude/*) return 0 ;;
  esac
  for ref in "refs/heads/$short" "refs/remotes/$short"; do
    if git show-ref -q --verify "$ref"; then
      echo "$short"
      return
    fi
  done
}

# Candidate indices with FROM first (when it is one), then ref order — every
# per-branch report lists the recorded origin before alphabetical accidents.
# Indices (not names) so callers can address the cherry_plus set of the same
# candidate.
ordered_indices() {
  local from=$1 index
  if [ -n "$from" ]; then
    for index in "${!candidates[@]}"; do
      if [ "${candidates[$index]}" = "$from" ]; then echo "$index"; fi
    done
  fi
  for index in "${!candidates[@]}"; do
    if [ "${candidates[$index]}" != "$from" ]; then echo "$index"; fi
  done
}

# Per-candidate '+' hash sets of git cherry against the agent branch, computed
# once per branch (indexed parallel to candidates[]) and consumed by both
# unpicked_anywhere and contained_in.
cherry_plus=()
compute_cherry_sets() {
  local branch=$1 candidate_branch
  cherry_plus=()
  for candidate_branch in "${candidates[@]}"; do
    cherry_plus+=("$(git cherry "$candidate_branch" "$branch" | sed -n 's/^+ //p')")
  done
}

# Hashes of ahead-commits whose patch is in no candidate branch: intersection
# of the precomputed cherry '+' sets (compute_cherry_sets ran for this branch).
unpicked_anywhere() {
  printf '%s\n' "${cherry_plus[@]}" | sed '/^$/d' |
    sort | uniq -c | awk -v n="${#candidates[@]}" '$1 == n {print $2}'
}

# Prints every candidate branch that contains all of the agent branch's work
# (tip is an ancestor, or every ahead-commit is patch-present via git cherry),
# comma-separated, FROM first. Prints nothing when the work exists nowhere else.
contained_in() {
  local branch=$1 from=$2 index candidate_branch out=''
  while IFS= read -r index; do
    candidate_branch=${candidates[$index]}
    if git merge-base --is-ancestor "$branch" "$candidate_branch" ||
        [ -z "${cherry_plus[$index]}" ]; then
      out+="${out:+,}$candidate_branch"
    fi
  done < <(ordered_indices "$from")
  echo "$out"
}

# Untracked files excluding worktree infrastructure (.idea/, .claude/,
# .claude-worktree) — those are written by hooks, not by the session's work.
# Prefix-matched: a partially tracked .idea/ lists individual files instead
# of the collapsed directory line, and those must not count either.
count_untracked() {
  git -C "$1" status --porcelain | grep '^??' |
    grep -cv -e '^?? \.idea/' -e '^?? \.claude/' -e '^?? \.claude-worktree$' || true
}

# Milliseconds since the epoch. macOS date has no %N; perl ships with macOS.
now_ms() {
  perl -MTime::HiRes=time -e 'printf "%d", time() * 1000'
}

# Lazy detached probe worktree, one per process. A different FROM re-detaches
# the same worktree (remove_agent_worktrees.sh iterates branches with
# potentially different FROMs in one process; status row workers are one
# branch each and never hit the re-detach path).
probe_wt=''
probe_wt_from=''
probe_count=0
probe_ms=0
ensure_probe_worktree() {
  local from=$1
  if [ -n "$probe_wt" ]; then
    if [ "$probe_wt_from" != "$from" ]; then
      git -C "$probe_wt" checkout -q --detach "$from" >/dev/null 2>&1
      probe_wt_from=$from
    fi
    return 0
  fi
  probe_wt=$(mktemp -d)
  git worktree add --quiet --detach "$probe_wt" "$from" 2>/dev/null
  probe_wt_from=$from
}

verdict_cleanup() {
  if [ -n "$probe_wt" ]; then
    git worktree remove --force "$probe_wt" 2>/dev/null || true
    rmdir -- "$probe_wt" 2>/dev/null || true
  fi
}

# Reads a range-diff interdiff on stdin, prints "applied-resolved" or
# "unpicked". A conflicted re-pick is ambiguous: an already-applied change
# self-conflicts, but so does a competing change to the same region.
# range-diff pairs the two commits and prints their interdiff — a diff of the
# two patches, where each line carries two markers: column 5 (after the
# 4-space range-diff indent) says whether the patch line differs BETWEEN the
# commits (+/-) or is shared (space), column 6 says the line's role INSIDE its
# patch (+ added, - removed, space context). The only question a deletion gate
# cares about is whether the agent's ADDED content is what landed on FROM:
#   - added line differs between the commits (col5 +/-, col6 +) => the agent's
#     content is not on FROM => unpicked (the false-safe S7 caught).
#   - removed/context lines differ => inherent to a resolved pick (it removes
#     FROM's moved-on lines, not the base's) => does not block.
# Requires a real =/! pairing (no pairing => unpicked). $1 is the agent
# commit's added-line count: a pure-deletion commit has no added lines to
# compare, so its removed lines must match too, else unpicked. Any binary hunk
# is unverifiable => unpicked. (plans/applied_probe_safety/design/exploration.md, option C)
interdiff_addition_verdict() {
  awk -v adds="$1" '
    /^ *[0-9]+: +[0-9a-f]+ +[=!] +[0-9]+: +[0-9a-f]+/ { paired = 1 }
    /Binary files/ { binary = 1 }
    /^    / {
      outer = substr($0, 5, 1)
      inner = substr($0, 6, 1)
      if ((outer == "+" || outer == "-") && inner == "+") added_diff++
      if ((outer == "+" || outer == "-") && inner == "-") removed_diff++
    }
    END {
      if (!paired || binary || added_diff > 0) { print "unpicked"; exit }
      if (adds == 0 && removed_diff > 0)       { print "unpicked"; exit }
      print "applied-resolved"
    }'
}

# Layered probe for one commit whose patch is absent from FROM by patch-id
# (layer 1 already failed). Result in the global `verdict` — never echoed:
# callers need the trace counters and the probe worktree mutations, which a
# $() subshell would discard. Probe git output fully silenced: Auto-merging/
# CONFLICT lines on stdout would parse as commit rows in the UI.
verdict=''
probe_verdict() {
  local hash=$1 from=$2 started merge_base agent_adds
  started=$(now_ms)
  probe_count=$((probe_count + 1))
  ensure_probe_worktree "$from"
  if git -C "$probe_wt" cherry-pick --no-commit "$hash" >/dev/null 2>&1; then
    if git -C "$probe_wt" diff --cached --quiet; then
      verdict=applied
    else
      verdict=unpicked
    fi
    git -C "$probe_wt" reset --hard HEAD >/dev/null 2>&1
    probe_ms=$((probe_ms + $(now_ms) - started))
    return 0
  fi

  git -C "$probe_wt" cherry-pick --abort >/dev/null 2>&1 ||
    git -C "$probe_wt" reset --hard HEAD >/dev/null 2>&1
  # Conflict: disambiguate via the range-diff interdiff (see the function above).
  merge_base=$(git merge-base "$hash" "$from")
  agent_adds=$(git show --numstat --format= "$hash" | awk '{ added += $1 } END { print added + 0 }')
  verdict=$(git range-diff "$merge_base..$from" "$hash^..$hash" 2>/dev/null |
    interdiff_addition_verdict "$agent_adds")
  probe_ms=$((probe_ms + $(now_ms) - started))
}

# Memoized probe: hashes are globally unique, so the memo needs no FROM key.
verdict_hashes=()
verdict_values=()
verdict_for() {
  local hash=$1 from=$2 index
  for index in "${!verdict_hashes[@]}"; do
    if [ "${verdict_hashes[$index]}" = "$hash" ]; then
      verdict=${verdict_values[$index]}
      return 0
    fi
  done
  probe_verdict "$hash" "$from"
  verdict_hashes+=("$hash")
  verdict_values+=("$verdict")
}

# Full safety evaluation for one branch. Requires load_candidates. Results:
#   eval_unpicked  UNPICKED count (unpicked-verdict intersection commits)
#   eval_verdicts  VERDICTS summary (applied:n,resolved:n) or '-'
#   eval_in        IN list incl. FROM upgrade entries, or ''
# FROM's '+' set is the probe target set; the intersection (patch-present in
# no local candidate) stays the UNPICKED base so a commit picked into any
# local branch keeps UNPICKED at 0 exactly as before.
eval_unpicked=0
eval_verdicts=-
eval_in=''
evaluate_branch() {
  local branch=$1 from=$2 hash from_plus candidate_branch
  local applied_n=0 resolved_n=0 unpicked_n=0 is_candidate=0 all_transferred=1
  compute_cherry_sets "$branch"
  eval_in=$(contained_in "$branch" "$from")
  eval_unpicked=$(unpicked_anywhere | sed '/^$/d' | wc -l | tr -d ' ')
  eval_verdicts=-
  [ -z "$from" ] && return 0

  from_plus=$(git cherry "$from" "$branch" | sed -n 's/^+ //p')
  if [ -z "$from_plus" ]; then
    # Exact patch-id containment on FROM. Local candidates are already listed
    # by contained_in; a remote-tracking FROM earns its plain entry here.
    for candidate_branch in "${candidates[@]}"; do
      if [ "$candidate_branch" = "$from" ]; then is_candidate=1; fi
    done
    if [ "$is_candidate" -eq 0 ]; then
      eval_in="$from${eval_in:+,$eval_in}"
    fi
    return 0
  fi

  while IFS= read -r hash; do
    verdict_for "$hash" "$from"
    case "$verdict" in
      applied) applied_n=$((applied_n + 1)) ;;
      applied-resolved) resolved_n=$((resolved_n + 1)) ;;
      unpicked) all_transferred=0 ;;
    esac
  done <<< "$from_plus"

  # UNPICKED: intersection commits the probe did not upgrade. Intersection
  # hashes absent from FROM's '+' set are patch-present on FROM => picked.
  while IFS= read -r hash; do
    [ -z "$hash" ] && continue
    if printf '%s\n' "$from_plus" | grep -qx "$hash"; then
      verdict_for "$hash" "$from"
      if [ "$verdict" = unpicked ]; then unpicked_n=$((unpicked_n + 1)); fi
    fi
  done < <(unpicked_anywhere)
  eval_unpicked=$unpicked_n

  local summary=''
  [ "$applied_n" -gt 0 ] && summary="applied:$applied_n"
  [ "$resolved_n" -gt 0 ] && summary="${summary:+$summary,}resolved:$resolved_n"
  eval_verdicts=${summary:--}

  if [ "$all_transferred" -eq 1 ]; then
    eval_in="$from*${eval_in:+,$eval_in}"
  fi
}
