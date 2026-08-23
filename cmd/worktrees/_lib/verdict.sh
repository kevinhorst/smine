#!/usr/bin/env bash
# Shared verdict logic for agent-branch safety: candidate discovery, FROM
# resolution, cherry sets, the layered applied-probe, the candidate-wide
# twin sweep (picked | applied | applied-resolved | picked-resolved |
# unpicked | unpicked-notwin), and the containment list both
# print_agent_worktrees_status.sh and remove_agent_worktrees.sh derive
# Safe from.
#
# Sourced, not executed. Callers run under `set -euo pipefail`, call
# load_candidates once, and register `trap verdict_cleanup EXIT` before the
# first evaluate_branch call (the probe worktree is created lazily).
#
# bash 3.2: parallel indexed arrays throughout, no associative arrays.

# All local non-agent branches (not claude/*, not claude-routines/*) — where
# harvested work could live.
candidates=()
load_candidates() {
  local candidate_branch
  candidates=()
  while IFS= read -r candidate_branch; do
    case "$candidate_branch" in claude/* | claude-routines/*) continue ;; esac
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
    '' | HEAD | claude/* | claude-routines/*) return 0 ;;
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
  # An empty merge_base..from range (FROM never moved past the merge-base)
  # means nothing on FROM could pair with the commit — the conflict is
  # genuine, and range-diff would die with a usage error on the empty range.
  merge_base=$(git merge-base "$hash" "$from")
  if [ "$merge_base" = "$(git rev-parse "$from")" ]; then
    verdict=unpicked
    probe_ms=$((probe_ms + $(now_ms) - started))
    return 0
  fi
  agent_adds=$(git show --numstat --format= "$hash" | awk '{ added += $1 } END { print added + 0 }')
  verdict=$(git range-diff "$merge_base..$from" "$hash^..$hash" 2>/dev/null |
    interdiff_addition_verdict "$agent_adds")
  probe_ms=$((probe_ms + $(now_ms) - started))
}

# Index of a branch in candidates[], -1 when it is not a local candidate
# (e.g. a remote-tracking FROM).
candidate_index() {
  local branch=$1 index
  for index in "${!candidates[@]}"; do
    if [ "${candidates[$index]}" = "$branch" ]; then
      echo "$index"
      return
    fi
  done
  echo -1
}

# hash↔candidate explanations ("<idx> <hash>" pairs) recorded by verdict_for
# and twin_sweep: the commit's change is present on that candidate — exactly
# (patch-id), by FROM probe, or as a range-diff-paired subject twin. Consumed
# by the per-candidate IN stars in evaluate_branch. Hash↔candidate facts are
# branch-independent, so the memo survives multi-branch callers.
explained_pairs=()
mark_explained() {
  explained_pairs+=("$1 $2")
}
is_explained() {
  local pair="$1 $2" p
  for p in ${explained_pairs[@]+"${explained_pairs[@]}"}; do
    if [ "$p" = "$pair" ]; then return 0; fi
  done
  return 1
}

# Candidate-wide explanation sweep for one commit the FROM probe could not
# place (verdict was unpicked). Patch-id equivalence breaks on any manual
# conflict resolution, so a conflict-resolved pick to main reads unpicked
# forever under git cherry alone — this sweep looks for the landed twin:
# for each candidate (FROM first), exact patch presence (cherry '-') counts
# as picked; otherwise a subject twin — a commit with the IDENTICAL subject
# line since the merge-base — that git range-diff pairs (=/!) with the agent
# commit counts as picked-resolved (drift in the added lines is exactly what
# a manual resolution produces; the pairing itself is the equivalence check,
# per the diagnosis decisions — no added-line thresholds).
# Results in the globals:
#   verdict            picked | picked-resolved | unpicked | unpicked-notwin
#   verdict_candidate  branch carrying the twin (picked-resolved), or the
#                      branch whose twin was rejected (unpicked), else ''
#   verdict_twin       the twin sha, else ''
# unpicked-notwin is the exhausted flavor: no subject twin exists on ANY
# candidate — a transfer under a reworded subject, a squash, or heavy
# modification is invisible to this detector and must be verified manually.
verdict_candidate=''
verdict_twin=''
twin_sweep() {
  local hash=$1 from=$2 index candidate_branch subject mb twin
  local picked_found=0 resolved_found=0 twin_seen=0
  local rejected_candidate='' rejected_twin=''
  verdict_candidate=''
  verdict_twin=''
  subject=$(git log -1 --format=%s "$hash")
  while IFS= read -r index; do
    candidate_branch=${candidates[$index]}
    if ! printf '%s\n' "${cherry_plus[$index]}" | grep -qx "$hash"; then
      # Patch-present on this candidate (git cherry '-'): exact pick.
      mark_explained "$index" "$hash"
      picked_found=1
      continue
    fi
    [ -z "$subject" ] && continue
    mb=$(git merge-base "$hash" "$candidate_branch" 2>/dev/null) || continue
    while IFS= read -r twin; do
      [ -z "$twin" ] && continue
      [ "$twin" = "$hash" ] && continue
      # --grep is a substring match even with -F: require the exact subject.
      if [ "$(git log -1 --format=%s "$twin")" != "$subject" ]; then continue; fi
      twin_seen=1
      if git range-diff "$twin^..$twin" "$hash^..$hash" 2>/dev/null |
          grep -qE '^ *[0-9]+: +[0-9a-f]+ +[=!] +[0-9]+: +[0-9a-f]+'; then
        mark_explained "$index" "$hash"
        if [ "$resolved_found" -eq 0 ]; then
          resolved_found=1
          verdict_candidate=$candidate_branch
          verdict_twin=$twin
        fi
        break
      fi
      if [ -z "$rejected_twin" ]; then
        rejected_candidate=$candidate_branch
        rejected_twin=$twin
      fi
    done < <(git log --format=%H --fixed-strings --grep="$subject" "$mb..$candidate_branch" 2>/dev/null)
  done < <(ordered_indices "$from")

  if [ "$picked_found" -eq 1 ]; then
    verdict=picked
  elif [ "$resolved_found" -eq 1 ]; then
    verdict=picked-resolved
  elif [ "$twin_seen" -eq 1 ]; then
    verdict=unpicked
    verdict_candidate=$rejected_candidate
    verdict_twin=$rejected_twin
  else
    verdict=unpicked-notwin
  fi
}

# Memoized probe + twin sweep: hashes are globally unique, so the memo needs
# no FROM key. A FROM-probe explanation (applied/applied-resolved) marks the
# local FROM candidate explained so the IN stars see it.
verdict_hashes=()
verdict_values=()
verdict_candidates=()
verdict_twins=()
verdict_for() {
  local hash=$1 from=$2 index from_index
  for index in "${!verdict_hashes[@]}"; do
    if [ "${verdict_hashes[$index]}" = "$hash" ]; then
      verdict=${verdict_values[$index]}
      verdict_candidate=${verdict_candidates[$index]}
      verdict_twin=${verdict_twins[$index]}
      return 0
    fi
  done
  probe_verdict "$hash" "$from"
  verdict_candidate=''
  verdict_twin=''
  case "$verdict" in
    applied | applied-resolved)
      from_index=$(candidate_index "$from")
      if [ "$from_index" -ge 0 ]; then mark_explained "$from_index" "$hash"; fi
      ;;
    unpicked)
      twin_sweep "$hash" "$from"
      ;;
  esac
  verdict_hashes+=("$hash")
  verdict_values+=("$verdict")
  verdict_candidates+=("$verdict_candidate")
  verdict_twins+=("$verdict_twin")
}

# True when every '+' commit of candidate index $1 is explained on it —
# the candidate contains all of the branch's work, partly via probe or twin
# verdicts. Only probed hashes carry explanations, so this stays conservative.
candidate_fully_explained() {
  local index=$1 hash
  [ -z "${cherry_plus[$index]}" ] && return 1
  while IFS= read -r hash; do
    [ -z "$hash" ] && continue
    if ! is_explained "$index" "$hash"; then return 1; fi
  done <<< "${cherry_plus[$index]}"
  return 0
}

# Full safety evaluation for one branch. Requires load_candidates. Results:
#   eval_unpicked  UNPICKED count (intersection commits still unpicked or
#                  unpicked-notwin after the probe and the twin sweep)
#   eval_verdicts  VERDICTS summary (applied:n,resolved:n,picked-resolved:n)
#                  or '-'
#   eval_in        IN list: plain entries for exact containment, starred
#                  entries (X*) where containment came from probe/twin
#                  verdicts, FROM first, or ''
# FROM's '+' set is the probe target set; the intersection (patch-present in
# no local candidate) stays the UNPICKED base so a commit picked into any
# local branch keeps UNPICKED at 0 exactly as before.
eval_unpicked=0
eval_verdicts=-
eval_in=''
evaluate_branch() {
  local branch=$1 from=$2 hash from_plus candidate_branch index
  local applied_n=0 resolved_n=0 picked_resolved_n=0 unpicked_n=0
  local is_candidate=0 all_transferred=1
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
      picked-resolved) picked_resolved_n=$((picked_resolved_n + 1)); all_transferred=0 ;;
      picked | unpicked | unpicked-notwin) all_transferred=0 ;;
    esac
  done <<< "$from_plus"

  # UNPICKED: intersection commits neither the probe nor the twin sweep
  # explained. Intersection hashes absent from FROM's '+' set are
  # patch-present on FROM => picked.
  while IFS= read -r hash; do
    [ -z "$hash" ] && continue
    if printf '%s\n' "$from_plus" | grep -qx "$hash"; then
      verdict_for "$hash" "$from"
      case "$verdict" in
        unpicked | unpicked-notwin) unpicked_n=$((unpicked_n + 1)) ;;
      esac
    fi
  done < <(unpicked_anywhere)
  eval_unpicked=$unpicked_n

  local summary=''
  [ "$applied_n" -gt 0 ] && summary="applied:$applied_n"
  [ "$resolved_n" -gt 0 ] && summary="${summary:+$summary,}resolved:$resolved_n"
  [ "$picked_resolved_n" -gt 0 ] && summary="${summary:+$summary,}picked-resolved:$picked_resolved_n"
  eval_verdicts=${summary:--}

  # Rebuild IN with per-candidate stars: a candidate whose every '+' commit
  # is explained (FROM probe or twin sweep) contains all the work even though
  # the patch-ids drifted — that is exactly the conflict-resolved-pick case.
  local in_out=''
  while IFS= read -r index; do
    candidate_branch=${candidates[$index]}
    if git merge-base --is-ancestor "$branch" "$candidate_branch" ||
        [ -z "${cherry_plus[$index]}" ]; then
      in_out+="${in_out:+,}$candidate_branch"
    elif candidate_fully_explained "$index"; then
      in_out+="${in_out:+,}$candidate_branch*"
    fi
  done < <(ordered_indices "$from")
  eval_in=$in_out

  # A remote-tracking FROM is not a candidate: its probe upgrade is prepended
  # here. A local FROM already earned its star via candidate_fully_explained.
  if [ "$all_transferred" -eq 1 ]; then
    for candidate_branch in "${candidates[@]}"; do
      if [ "$candidate_branch" = "$from" ]; then is_candidate=1; fi
    done
    if [ "$is_candidate" -eq 0 ]; then
      eval_in="$from*${eval_in:+,$eval_in}"
    fi
  fi
}
