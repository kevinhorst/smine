## file: cmd/worktrees/_lib/verdict.sh

#!/usr/bin/env bash
# [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit
# - [ACDSL-SHELL-002] Scripts launched by hooks or launchd run under macOS bash 3.2 — no declare -A, mapfile/readarray, case expansion, |&, or parameter transformation (the launchd lesson)

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

# Untracked entries written by tooling, not by the session's work: worktree
# infrastructure (worktree-sessionstart.sh) plus per-tool droppings (.serena/
# — Serena MCP project cache; .DS_Store — Finder, any depth). Prefix-matched:
# a partially tracked .idea/ lists individual files instead of the collapsed
# directory line, and those must not count either. Reads porcelain on stdin,
# prints the count of remaining untracked entries. The single source for the
# UNTRACKED column and the remove gate.
filter_untracked() {
  grep '^??' |
    grep -cv \
      -e '^?? \.idea/' \
      -e '^?? \.claude/' \
      -e '^?? \.claude-worktree$' \
      -e '^?? \.serena/' \
      -e '^?? \.DS_Store$' \
      -e '/\.DS_Store$' || true
}

count_untracked() {
  git -C "$1" status --porcelain | filter_untracked
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


## file: cmd/worktrees/print_agent_worktrees_status.sh

#!/usr/bin/env bash
# [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit
# - [ACDSL-SHELL-002] Scripts launched by hooks or launchd run under macOS bash 3.2 — no declare -A, mapfile/readarray, case expansion, |&, or parameter transformation (the launchd lesson)

# State of every agent branch (claude/*, claude-routines/*) relative to the
# rest of the repo.
#
# Usage:
#   print_agent_worktrees_status.sh
#   print_agent_worktrees_status.sh <number|branch> <unpicked|ahead|behind>
#
# Without args, prints the overview table (each branch gets a #).
# With a row <number> or an agent branch name and a class, lists the
# individual commits for that branch.
#
# FROM      branch this one was created from, taken from the branch reflog
#           (remote-tracking origins show as e.g. origin/main). "unknown" when
#           the reflog records no resolvable origin — FROM is never guessed.
# DIRTY     modified files in the agent worktree ("-" = branch has no worktree)
# UNTRACKED untracked files in the agent worktree, excluding worktree
#           infrastructure and tool droppings (.idea/, .claude/,
#           .claude-worktree, .serena/, .DS_Store) — the same filter
#           remove_agent_worktrees.sh applies ("-" = no worktree)
# AHEAD     commits on the agent branch missing from FROM ("-" = FROM unknown)
# BEHIND    commits on FROM missing from the agent branch ("-" = FROM unknown)
# UNPICKED  ahead-commits whose patch is in NO non-claude branch AND whose
#           change neither the applied-probe (re-pick + range-diff on FROM)
#           nor the twin sweep (exact-subject commit paired by range-diff on
#           any candidate) could place — genuinely untransferred work
# VERDICTS  probe summary for commits missing from FROM by patch-id:
#           applied:<n> (clean empty re-pick on FROM), resolved:<n>
#           (conflicted re-pick on FROM paired by range-diff), and
#           picked-resolved:<n> (landed on a candidate as a subject twin with
#           manual conflict resolution — counted as picked, but a later merge
#           or re-pick of this branch will conflict against the resolved
#           version); "-" when nothing needed probing
# MERGED    the non-claude branch whose first-parent history contains a merge
#           commit that merged this branch's tip — an actual merge, never
#           containment; "-" when the tip was never merged (fast-forward or
#           cherry-pick transfers stay "-")
# IN        every non-claude branch that already contains ALL of this branch's
#           work (tip is an ancestor, or every ahead-commit is patch-present),
#           comma-separated, FROM first. A starred entry (X*) means
#           containment came from probe/twin verdicts (applied,
#           applied-resolved, picked-resolved) instead of exact patch-ids;
#           "-" = the work exists nowhere else
#
# Safe to remove: IN != "-" plus, when a worktree is checked out, DIRTY=0 and
# UNTRACKED=0. A branch without a worktree is judged on containment alone.
# (cmd/worktrees/remove_agent_worktrees.sh, internal/repos/status.go)
#
# Verdict cache: UNPICKED/VERDICTS/MERGED/IN are pure functions of the branch
# tip, its FROM, and the candidate tips — cached per branch under
# <git-common-dir>/agent-status-cache/, keyed by exactly those SHAs. DIRTY,
# UNTRACKED, AHEAD/BEHIND and LAST-COMMIT are always computed live.
# WORKTREE_STATUS_NO_CACHE=1 bypasses the cache (tests, diagnosis).

set -euo pipefail

source "$(dirname "$0")/_lib/verdict.sh"

# __row <num> is internal: one overview row, dispatched by the parent via
# xargs -P (requires WTS_WORKTREES and WTS_ROWDIR in the environment).
mode=overview
if [ "${1:-}" = "__row" ]; then
  mode=row
  row_num=${2:?__row needs a row number}
elif [ -n "${1:-}" ] && [ -n "${2:-}" ]; then
  mode=detail
  selected=$1
  selected_class=$2
fi

branches=()
while IFS= read -r b; do
  branches+=("$b")
done < <(git for-each-ref --format='%(refname:short)' \
  refs/heads/claude/ refs/heads/claude-routines/)

if [ ${#branches[@]} -eq 0 ]; then
  echo "No agent branches (claude/*, claude-routines/*) found."
  exit 0
fi

# All local branches that are not agent branches — the places where harvested
# work could live.
load_candidates

if [ ${#candidates[@]} -eq 0 ]; then
  echo "error: no non-claude branches found to compare against"
  exit 1
fi

# Prints the candidate branch whose first-parent history contains a merge
# commit with this branch's tip as a merged parent — an actual `git merge`,
# never mere containment (FROM first, then ref order). Fast-forwards and
# cherry-pick transfers leave no merge commit and print nothing.
merged_into() {
  local branch=$1 from=$2 tip i c
  tip=$(git rev-parse "$branch")
  while IFS= read -r i; do
    c=${candidates[$i]}
    if git rev-list --first-parent --merges --parents "$c" --not "$branch" |
        awk -v tip="$tip" 'BEGIN {found = 1}
          {for (i = 3; i <= NF; i++) if ($i == tip) found = 0}
          END {exit found}'; then
      echo "$c"
      return
    fi
  done < <(ordered_indices "$from")
}

if [ "$mode" = detail ]; then
  if [[ $selected == claude/* || $selected == claude-routines/* ]]; then
    branch=''
    for b in "${branches[@]}"; do
      if [ "$b" = "$selected" ]; then branch=$b; fi
    done
    if [ -z "$branch" ]; then
      echo "error: unknown branch $selected"
      exit 1
    fi
  else
    idx=$((selected - 1))
    if [ "$idx" -lt 0 ] || [ "$idx" -ge "${#branches[@]}" ]; then
      echo "error: #$selected is out of range (1-${#branches[@]})"
      exit 1
    fi
    branch=${branches[$idx]}
  fi
  from=$(resolve_from "$branch")

  case "$selected_class" in
    ahead | behind)
      if [ -z "$from" ]; then
        echo "error: origin of $branch is not recorded — FROM unknown, no $selected_class list"
        exit 1
      fi
      ;;
  esac

  case "$selected_class" in
    ahead)
      echo "Commits ahead on $branch vs $from:"
      git log --format='%h  %cd  %s' --date=format:'%Y-%m-%d %H:%M' "$from..$branch"
      ;;
    behind)
      echo "Commits behind on $branch vs $from:"
      git log --format='%h  %cd  %s' --date=format:'%Y-%m-%d %H:%M' "$branch..$from"
      ;;
    unpicked)
      echo "Commits on $branch found in no non-claude branch:"
      compute_cherry_sets "$branch"
      hashes=()
      while IFS= read -r hash; do
        hashes+=("$hash")
      done < <(unpicked_anywhere)

      if [ ${#hashes[@]} -eq 0 ]; then
        echo "(none)"
        exit 0
      fi

      # FROM unknown: no branch to probe cherry-picks against — plain list.
      if [ -z "$from" ]; then
        for hash in "${hashes[@]}"; do
          git log -1 --format='%h  %cd  %s' --date=format:'%Y-%m-%d %H:%M' "$hash"
        done
        exit 0
      fi

      # Verdict per hash from the shared lib; hashes patch-present on FROM
      # (not in its '+' set) are exact picks and need no probe.
      trap verdict_cleanup EXIT
      from_plus=$(git cherry "$from" "$branch" | sed -n 's/^+ //p')
      for hash in "${hashes[@]}"; do
        if ! printf '%s\n' "$from_plus" | grep -qx "$hash"; then
          git log -1 --format='%h  %cd  %s  (applied on '"$from"')' --date=format:'%Y-%m-%d %H:%M' "$hash"
          continue
        fi
        verdict_for "$hash" "$from"
        case "$verdict" in
          applied)
            git log -1 --format='%h  %cd  %s  (applied on '"$from"')' --date=format:'%Y-%m-%d %H:%M' "$hash" ;;
          applied-resolved)
            git log -1 --format='%h  %cd  %s  (applied on '"$from"', conflict resolved)' --date=format:'%Y-%m-%d %H:%M' "$hash" ;;
          picked-resolved)
            git log -1 --format='%h  %cd  %s  (picked on '"$verdict_candidate"' as '"$(git rev-parse --short "$verdict_twin")"', conflict resolved manually — not auto-reconcilable)' --date=format:'%Y-%m-%d %H:%M' "$hash" ;;
          unpicked)
            # A subject twin exists but range-diff refused to pair it: the
            # content genuinely differs from what landed there.
            git log -1 --format='%h  %cd  %s  (twin '"$(git rev-parse --short "$verdict_twin")"' on '"$verdict_candidate"' differs — content not transferred)' --date=format:'%Y-%m-%d %H:%M' "$hash" ;;
          unpicked-notwin)
            # Twin search exhausted: a transfer under a reworded subject, a
            # squash, or heavy modification is invisible to detection.
            git log -1 --format='%h  %cd  %s  (no twin on any local branch — reworded/squashed transfers are invisible; verify via git log --all --grep before treating as lost)' --date=format:'%Y-%m-%d %H:%M' "$hash" ;;
          *)
            git log -1 --format='%h  %cd  %s' --date=format:'%Y-%m-%d %H:%M' "$hash" ;;
        esac
      done
      ;;
    *)
      echo "error: unknown class '$selected_class' (use: unpicked, ahead, behind)"
      exit 1
      ;;
  esac
  exit 0
fi

# One overview row for branches[num-1], written to $WTS_ROWDIR/row-NNN so the
# parent can reassemble rows in branch order regardless of completion order.
print_row() {
  local num=$1 branch worktree dirty untracked porcelain raw_from from
  local ahead behind merged age tip from_tip cache_key cache_file cache_tmp
  branch=${branches[$((num - 1))]}
  worktree=$(awk -v b="refs/heads/$branch" \
    '/^worktree /{p=$2} $0=="branch "b{print p}' "$WTS_WORKTREES")

  dirty=-
  untracked=-
  if [ -n "$worktree" ]; then
    # printf '%s' (not '%s\n'): a clean worktree must feed grep zero lines,
    # not one empty line; grep still counts a final unterminated line.
    porcelain=$(git -C "$worktree" status --porcelain)
    dirty=$(printf '%s' "$porcelain" | grep -cv '^??' || true)
    untracked=$(printf '%s' "$porcelain" | filter_untracked)
  fi

  raw_from=$(resolve_from "$branch")
  if [ -n "$raw_from" ]; then
    from=$raw_from
    ahead=$(git rev-list --count "$from..$branch")
    behind=$(git rev-list --count "$branch..$from")
  else
    from=unknown
    ahead=-
    behind=-
  fi
  # Cache hit: the key line matches and the file is complete (5 lines) —
  # anything else recomputes and rewrites (a truncated file is a miss, and
  # the guarded read chain must never abort the child under set -e).
  tip=$(git rev-parse "$branch")
  from_tip=''
  [ -n "$raw_from" ] && from_tip=$(git rev-parse "$raw_from")
  cache_key="$tip|$raw_from|$from_tip|$WTS_CANDIDATES_DIGEST"
  cache_file="$WTS_CACHE_DIR/row-$(printf '%s' "$branch" | tr '/' '~')"
  if [ "${WORKTREE_STATUS_NO_CACHE:-0}" != 1 ] && [ -f "$cache_file" ] &&
      [ "$(head -1 "$cache_file")" = "$cache_key" ] &&
      [ "$(wc -l < "$cache_file")" -eq 5 ]; then
    { read -r _; read -r eval_unpicked; read -r eval_verdicts
      read -r eval_in; read -r merged; } < "$cache_file"
  else
    trap verdict_cleanup EXIT
    evaluate_branch "$branch" "$raw_from"
    merged=$(merged_into "$branch" "$raw_from")
    cache_tmp=$(mktemp "$WTS_CACHE_DIR/.tmp.XXXXXX")
    printf '%s\n%s\n%s\n%s\n%s\n' \
      "$cache_key" "$eval_unpicked" "$eval_verdicts" "$eval_in" "$merged" \
      > "$cache_tmp"
    mv -f -- "$cache_tmp" "$cache_file"
  fi
  if [ "${WORKTREE_STATUS_TRACE:-0}" = 1 ] && [ "$probe_count" -gt 0 ]; then
    echo "trace: $branch probes=$probe_count probe_ms=$probe_ms" >&2
  fi
  age=$(git log -1 --format='%cd' --date=format:'%Y-%m-%d %H:%M' "$branch")

  printf '%-4s %-36s %-16s %-6s %-9s %-6s %-7s %-9s %-20s %-16s %-24s %-16s %s\n' \
    "$num" "$branch" "$from" "$dirty" "$untracked" "$ahead" "$behind" "$eval_unpicked" \
    "$eval_verdicts" "${merged:--}" "${eval_in:--}" "$age" "${worktree:-–}" \
    > "$WTS_ROWDIR/row-$(printf '%03d' "$num")"
}

if [ "$mode" = row ]; then
  : "${WTS_WORKTREES:?__row is internal — run the script without args}"
  : "${WTS_ROWDIR:?__row is internal — run the script without args}"
  : "${WTS_CACHE_DIR:?__row is internal — run the script without args}"
  : "${WTS_CANDIDATES_DIGEST:?__row is internal — run the script without args}"
  print_row "$row_num"
  exit 0
fi

printf '%-4s %-36s %-16s %-6s %-9s %-6s %-7s %-9s %-20s %-16s %-24s %-16s %s\n' \
  '#' BRANCH FROM DIRTY UNTRACKED AHEAD BEHIND UNPICKED VERDICTS MERGED IN LAST-COMMIT WORKTREE

rowdir=$(mktemp -d)
trap 'rm -rf -- "$rowdir"' EXIT
git worktree list --porcelain > "$rowdir/worktrees"
export WTS_WORKTREES="$rowdir/worktrees"
export WTS_ROWDIR="$rowdir"

# Verdict-cache inputs shared with the row workers (see header). The digest
# covers every candidate (non-agent) tip so any candidate movement
# invalidates all rows.
cache_dir=$(git rev-parse --git-common-dir)/agent-status-cache
mkdir -p "$cache_dir"
export WTS_CACHE_DIR="$cache_dir"
WTS_CANDIDATES_DIGEST=$(git for-each-ref --format='%(refname:short) %(objectname)' refs/heads/ |
  awk '$1 !~ /^claude(-routines)?\//' | shasum | awk '{print $1}')
export WTS_CANDIDATES_DIGEST

# Prune rows of deleted branches ("~" is illegal in refnames, so the file
# name maps back to exactly one branch).
for cache_file in "$cache_dir"/row-*; do
  [ -e "$cache_file" ] || continue
  cached_branch=$(basename "$cache_file" | sed 's/^row-//' | tr '~' '/')
  git show-ref -q --verify "refs/heads/$cached_branch" || rm -f -- "$cache_file"
done

jobs=${WORKTREE_STATUS_JOBS:-$(getconf _NPROCESSORS_ONLN)}
seq 1 ${#branches[@]} | xargs -n1 -P "$jobs" "$0" __row

for f in "$rowdir"/row-*; do
  cat "$f"
done


## file: cmd/worktrees/remove_agent_worktrees.sh

#!/usr/bin/env bash
# [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit
# - [ACDSL-SHELL-002] Scripts launched by hooks or launchd run under macOS bash 3.2 — no declare -A, mapfile/readarray, case expansion, |&, or parameter transformation (the launchd lesson)

# Remove agent worktrees — claude/* and claude-routines/* branches (and stray
# .codex/worktrees checkouts).
#
# Usage:
#   remove_agent_worktrees.sh [--force] [--delete-branch] [claude/<branch> | claude-routines/<branch>]
#
# With a branch target only that branch's worktree is considered;
# the detached .codex/worktrees sweep runs only in the untargeted invocation.
# A target with no attached worktree is reported explicitly, never silently.
#
# Without --force a worktree is only removed when it is safe to lose:
#   - no modified or untracked files in the worktree, and
#   - all of its commits already live on some non-claude branch (tip is an
#     ancestor, every commit is patch-present via git cherry, or every commit
#     the branch's FROM is missing probes as applied/applied-resolved — the
#     same applied-probe the status overview's Safe column uses).
# Unsafe worktrees are skipped with the reason printed.
#
# Infrastructure entries written by worktree-sessionstart.sh (.idea/,
# .claude/, .claude-worktree sentinel) and per-tool droppings (.serena/,
# .DS_Store) never count as untracked work, and locks placed by that hook
# are lifted before removal.
#
# With --force everything is removed regardless of state.
#
# Without --delete-branch, branches are never deleted, only worktrees. With
# --delete-branch (requires a branch target) the branch itself is
# deleted afterwards, under the same safety rules: only when its worktree
# (if any) was removed and the work lives on a non-claude branch — or
# unconditionally with --force.

set -euo pipefail

source "$(dirname "$0")/_lib/verdict.sh"
trap verdict_cleanup EXIT

force=0
delete_branch=0
target=''
for arg in "$@"; do
  case "$arg" in
    --force) force=1 ;;
    --delete-branch) delete_branch=1 ;;
    claude/* | claude-routines/*) target="$arg" ;;
    *) echo "usage: $(basename "$0") [--force] [--delete-branch] [claude/<branch> | claude-routines/<branch>]"; exit 1 ;;
  esac
done

if [ "$delete_branch" -eq 1 ] && [ -z "$target" ]; then
  echo "usage: --delete-branch requires a branch target"
  exit 1
fi

# Normalize cwd to the main checkout: bare git commands below run against the
# cwd, and /close invokes this from inside the worktree being removed.
cd "$(git worktree list --porcelain | awk '/^worktree /{print $2; exit}')"

load_candidates

remove() {
  local path=$1
  git worktree unlock "$path" 2>/dev/null || true
  git worktree remove --force "$path"
  echo "removed: $path"
}

# count_untracked comes from _lib/verdict.sh — the same infrastructure filter
# (.idea/, .claude/, .claude-worktree, .serena/, .DS_Store) the status
# script's UNTRACKED column applies.

skipped=0
matched=0
target_skipped=0

# agent-branch worktrees (claude/*, claude-routines/*)
while IFS=$'\t' read -r path branch; do
  if [ -n "$target" ] && [ "$branch" != "$target" ]; then
    continue
  fi
  matched=1
  if [ "$force" -eq 1 ]; then
    remove "$path"
    continue
  fi

  modified=$(git -C "$path" status --porcelain | grep -cv '^??' || true)
  untracked=$(count_untracked "$path")
  # evaluate_branch, not $(...) — a subshell would leak the probe worktree
  # past verdict_cleanup and discard the verdict memo. eval_in is the same IN
  # list (containment plus applied-probe upgrade) the overview's Safe uses.
  evaluate_branch "$branch" "$(resolve_from "$branch")"
  in=$eval_in

  reasons=''
  [ "$modified" -gt 0 ] && reasons+="$modified modified file(s); "
  [ "$untracked" -gt 0 ] && reasons+="$untracked untracked file(s); "
  [ -z "$in" ] && reasons+='commits exist on no non-claude branch; '

  if [ -n "$reasons" ]; then
    echo "skipped: $path ($branch): ${reasons%; }"
    skipped=$((skipped + 1))
    if [ "$branch" = "$target" ]; then target_skipped=1; fi
  else
    remove "$path"
  fi
done < <(git worktree list --porcelain |
  awk '/^worktree /{path=$2} sub(/^branch refs\/heads\//, "", $0) && $0 ~ /^claude(-routines)?\//{print path "\t" $0}')

# A targeted run must never end silently — detached checkouts and
# already-removed worktrees would otherwise look like a display bug.
if [ -n "$target" ] && [ "$matched" -eq 0 ]; then
  echo "no worktree checked out for $target — nothing removed"
fi

# Branch deletion runs after the worktree pass so the branch is no longer
# checked out anywhere; same safety rules as removal unless forced.
if [ "$delete_branch" -eq 1 ]; then
  if [ "$force" -eq 1 ]; then
    git branch -D "$target"
    echo "deleted branch: $target"
  elif [ "$target_skipped" -eq 1 ]; then
    echo "skipped branch: $target: worktree kept (use --force)"
  elif { evaluate_branch "$target" "$(resolve_from "$target")"; [ -z "$eval_in" ]; }; then
    echo "skipped branch: $target: commits exist on no non-claude branch (use --force)"
  else
    git branch -D "$target"
    echo "deleted branch: $target"
  fi
fi

# detached .codex/worktrees checkouts (no branch to check work against);
# skipped entirely when a single branch is targeted
if [ -z "$target" ]; then
  while IFS= read -r path; do
    if [ "$force" -eq 1 ]; then
      remove "$path"
      continue
    fi
    changes=$(git -C "$path" status --porcelain | wc -l | tr -d ' ')
    if [ "$changes" -gt 0 ]; then
      echo "skipped: $path: $changes changed/untracked file(s) (codex worktree, use --force)"
      skipped=$((skipped + 1))
    else
      remove "$path"
    fi
  done < <(git worktree list --porcelain | awk '/^worktree .*\.codex\/worktrees/{print $2}')
fi

if [ "$skipped" -gt 0 ]; then
  echo
  echo "$skipped worktree(s) kept — rerun with --force to remove anyway."
fi


## file: internal/repos/status.go

// [ACDSL-PROJECTION] 5 rule(s) govern this file — working-copy view, stripped before commit
// - [ACDSL-GOLANG-ENUM-001] Every switch over a domain enum names all members (ACTION-IMPL-INTEG-005) — no default that silently swallows new ones
// - [ACDSL-GOLANG-EXEC-001] Child processes run under a context deadline via internal/shell.Run — a raw exec.Command has no timeout and can wedge the caller forever; routinewrap runs multi-hour children under its own backstop-deadline context (plans/windows_support raw.md D14)
// - [ACDSL-GOLANG-FMT-001] Every Go file is gofmt-formatted — run gofmt -w before committing; this gate replaces the raw Makefile gofmt line
// - [ACDSL-GOLANG-FUNC-001] Signatures follow RULE-GOLANG-FUNC-001 — ctx context.Context is the first parameter and is named ctx, error is the last return value, at most 3 return values
// - [ACDSL-GOLANG-STATE-001] context/context.json has exactly one producer — only cmd/rules may write it via the internal/contextdocs renderer; internal/server reads it for the coverage metric and internal/repos probes it for repo context detection; any other writer desyncs the generated context file

package repos

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	removeScript = "remove_agent_worktrees.sh"
	statusScript = "print_agent_worktrees_status.sh"
	syncScript   = "sync_worktrees.sh"
)

// noWorktreeDirty marks a branch without a checked-out worktree ("-" in the
// script's DIRTY column).
const noWorktreeDirty = -1

// unknownCount marks AHEAD/BEHIND when the branch's origin is unrecorded
// ("-" in the script; FROM is never guessed).
const unknownCount = -1

type Commit struct {
	Line string
	Sha  string
}

type WorktreeStatus struct {
	Ahead         int // unknownCount when From is "unknown"
	Behind        int // unknownCount when From is "unknown"
	Branch        string
	Dirty         int      // noWorktreeDirty when the branch has no worktree
	From          string   // "unknown" when the branch reflog records no origin
	In            []string // FROM first, probe/twin-upgraded entries keep their "*"; empty when the work exists nowhere else
	LastCommit    string
	MergedInto    string // actual merge commit target; empty when never merged
	ResolvedPicks int    // picked-resolved verdicts: transferred via manually conflict-resolved picks, not auto-reconcilable
	SafeToRemove  bool
	SafeViaProbe  bool     // an IN entry carries the "*" probe/twin marker
	UnsafeReasons []string // the conditions that made SafeToRemove false; empty when safe
	Unpicked      int
	Untracked     int    // noWorktreeDirty when the branch has no worktree
	Verdicts      string // probe summary (applied:n,resolved:n,picked-resolved:n); empty when "-"
	Worktree      string // empty when none
}

type CheckoutStatus struct {
	Branch string
	Dirty  int
}

// Checkout reports the main checkout's branch and uncommitted-change count
// (untracked excluded — the exact predicate merge_worktree.sh and
// cherry_pick_worktree.sh refuse on), so the UI flags a dirty checkout before
// an op runs instead of failing at execution.
func Checkout(ctx context.Context, repoPath string) (*CheckoutStatus, error) {
	branch, err := shell.Run(ctx, repoPath, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("Checkout: %s: %w", strings.TrimSpace(branch), err)
	}

	output, err := shell.Run(ctx, repoPath, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("Checkout: %s: %w", strings.TrimSpace(output), err)
	}

	dirty := 0
	for line := range strings.Lines(output) {
		if line != "\n" && line != "" && !strings.HasPrefix(line, "??") {
			dirty++
		}
	}
	return &CheckoutStatus{Branch: strings.TrimSpace(branch), Dirty: dirty}, nil
}

// Commits lists one class (ahead|behind|unpicked) of commits for a branch via
// the status script's detail mode, addressed by branch name — one script run
// per drill-down (supersedes feature_extension_v2 D17).
func Commits(ctx context.Context, branch, class, repoPath, scriptsDir string) ([]Commit, error) {
	script := filepath.Join(scriptsDir, statusScript)
	output, err := shell.Run(ctx, repoPath, script, branch, class)
	if err != nil {
		return nil, fmt.Errorf("Commits: %s: %w", strings.TrimSpace(output), err)
	}

	return parseCommits(output), nil
}

// Status runs the overview table with cwd = repo path and parses it (D18).
func Status(ctx context.Context, repoPath, scriptsDir string) ([]WorktreeStatus, error) {
	script := filepath.Join(scriptsDir, statusScript)
	output, err := shell.Run(ctx, repoPath, script)
	if err != nil {
		return nil, fmt.Errorf("Status: %s: %w", strings.TrimSpace(output), err)
	}

	return parseStatus(output)
}

func isStatusHeader(fields []string) bool {
	return len(fields) == 0 || fields[0] == "#"
}

func parseCommits(output string) []Commit {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= 1 {
		return nil
	}

	var commits []Commit
	// First line is the "Commits …:" heading.
	for _, line := range lines[1:] {
		isEmpty := line == "" || line == "(none)"
		if isEmpty {
			continue
		}
		fields := strings.Fields(line)
		commit := Commit{Line: line, Sha: fields[0]}
		commits = append(commits, commit)
	}
	return commits
}

func parseCount(field, column string) (int, error) {
	count, err := strconv.Atoi(field)
	if err != nil {
		return 0, fmt.Errorf("parseCount: Invalid %s value %q: %w", column, field, err)
	}
	return count, nil
}

func parseStatus(output string) ([]WorktreeStatus, error) {
	if strings.HasPrefix(output, "No agent branches") {
		return nil, nil
	}

	var result []WorktreeStatus
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		fields := strings.Fields(line)
		if isStatusHeader(fields) {
			continue
		}

		status, err := parseStatusRow(fields, line)
		if err != nil {
			return nil, err
		}
		result = append(result, *status)
	}
	return result, nil
}

// parseStatusRow maps one table row: 11 fixed fields, LAST-COMMIT as two
// tokens (date + time), the worktree path as the remainder. Any other shape
// is a hard error — never guess a field mapping (stop condition S8).
func parseStatusRow(fields []string, line string) (*WorktreeStatus, error) {
	if len(fields) < 14 {
		return nil, fmt.Errorf("parseStatusRow: Unparseable row: %q", line)
	}

	status := &WorktreeStatus{
		Branch:     fields[1],
		From:       fields[2],
		LastCommit: fields[11] + " " + fields[12],
	}

	// DIRTY/UNTRACKED: "-" = branch has no worktree checked out
	if fields[3] == "-" {
		status.Dirty = noWorktreeDirty
	} else {
		dirty, err := parseCount(fields[3], "DIRTY")
		if err != nil {
			return nil, err
		}
		status.Dirty = dirty
	}

	if fields[4] == "-" {
		status.Untracked = noWorktreeDirty
	} else {
		untracked, err := parseCount(fields[4], "UNTRACKED")
		if err != nil {
			return nil, err
		}
		status.Untracked = untracked
	}

	// AHEAD/BEHIND: "-" = FROM unknown, counts not computable
	if fields[5] == "-" {
		status.Ahead = unknownCount
	} else {
		ahead, err := parseCount(fields[5], "AHEAD")
		if err != nil {
			return nil, err
		}
		status.Ahead = ahead
	}

	if fields[6] == "-" {
		status.Behind = unknownCount
	} else {
		behind, err := parseCount(fields[6], "BEHIND")
		if err != nil {
			return nil, err
		}
		status.Behind = behind
	}

	unpicked, err := parseCount(fields[7], "UNPICKED")
	if err != nil {
		return nil, err
	}
	status.Unpicked = unpicked

	// VERDICTS: "-" = nothing needed probing
	if fields[8] != "-" {
		status.Verdicts = fields[8]
		for _, token := range strings.Split(status.Verdicts, ",") {
			count, found := strings.CutPrefix(token, "picked-resolved:")
			if !found {
				continue
			}
			resolved, err := parseCount(count, "VERDICTS")
			if err != nil {
				return nil, err
			}
			status.ResolvedPicks = resolved
		}
	}

	// MERGED: "-" = the tip was never target of an actual merge commit
	if fields[9] != "-" {
		status.MergedInto = fields[9]
	}

	// IN: comma-separated, FROM first; "-" = the work exists nowhere else
	if fields[10] != "-" {
		status.In = strings.Split(fields[10], ",")
	}

	// WORKTREE: en-dash = branch has no worktree (script prints '–')
	worktree := strings.Join(fields[13:], " ")
	if worktree != "–" {
		status.Worktree = worktree
	}

	// Script header contract: IN != "-" plus, when a worktree is checked out,
	// DIRTY=0 and UNTRACKED=0 => safe to remove. A branch without a worktree
	// (DIRTY "-") has no files to lose — containment alone decides. A "*" IN
	// entry means containment came from the probe/twin verdicts.
	contained := len(status.In) > 0
	if status.Dirty == noWorktreeDirty {
		status.SafeToRemove = contained
	} else {
		status.SafeToRemove = status.Dirty == 0 && status.Untracked == 0 && contained
	}
	for _, containing := range status.In {
		if strings.HasSuffix(containing, "*") {
			status.SafeViaProbe = true
		}
	}
	if !status.SafeToRemove {
		status.UnsafeReasons = unsafeReasons(status, contained)
	}
	return status, nil
}

// unsafeReasons names each condition that blocks removal, in the order the
// safety predicate checks them — the UI shows these instead of a generic ✗.
func unsafeReasons(status *WorktreeStatus, contained bool) []string {
	var reasons []string
	if status.Dirty > 0 {
		reasons = append(reasons, fmt.Sprintf("dirty(%d)", status.Dirty))
	}
	if status.Untracked > 0 {
		reasons = append(reasons, fmt.Sprintf("untracked(%d)", status.Untracked))
	}
	if !contained {
		if status.Unpicked > 0 {
			reasons = append(reasons, fmt.Sprintf("unpicked(%d)", status.Unpicked))
		} else {
			reasons = append(reasons, "work contained nowhere")
		}
		if status.Dirty == noWorktreeDirty {
			reasons = append(reasons, "no worktree")
		}
	}
	return reasons
}


## file: plans/applied-probe-safety/design/exploration.md

# Applied-Probe Safety for Agent-Worktree Branches — Exploration

## Context

- **Open question, two axes.** (A) How does the Overview decide a commit's work was *actually transferred* when a conflict-resolved cherry-pick has broken the patch-id? (B) Where does the single safety predicate live so the status script, `status.go`, and `remove_agent_worktrees.sh` stop drifting?
- **Driver A — the bug.** UNPICKED/IN/Safe come from `git cherry` exact patch-id (`compute_cherry_sets`, `verdict.sh:71`). A pick that resolved a conflict changes the hunk context, changes the patch-id, and pins the branch unsafe forever — the `claude/railroad-review-workflow-f76262` incident (8875782 landed as 9a2098e with a resolved `docs/workflows.md` conflict).
- **Driver B — three drifting copies.** The safety predicate ("all work lives elsewhere") is re-derived in the status script, in `status.go`, and in the remove script; the UI can say Safe while removal refuses.
- **Hard boundary.** This gate authorises worktree/branch *deletion*. A false "applied" verdict destroys unharvested work. Every ambiguous case must fail closed.
- **Mode:** familiar. Explanation depth normal; the constraints below are the rigor.

## Constraints

| ID  | Constraint | Source (anchor/measurement) |
|-----|------------|------------------------------|
| C1  | `git cherry` patch-id equivalence breaks on *any* manual conflict resolution — the whole defect | `verdict.sh:71`, concept incident 8875782→9a2098e |
| C2  | bash 3.2 only: no assoc arrays, `mapfile`, case expansion, param transforms | `ACDSL-SHELL-002` header, `verdict.sh:16` |
| C3  | Overview must stay near plumbing-only speed: probe only rows with a non-empty FROM cherry-`+` set; a fully harvested branch pays nothing | concept Goal 2; `evaluate_branch` early-return `verdict.sh:407` |
| C4  | **Fail closed.** A *competing* change to the same region must never read "applied" — this gate deletes work (stop condition S7) | concept Decisions; `interdiff_addition_verdict:185` |
| C5  | Probe git output must be fully silenced — `Auto-merging`/`CONFLICT` on stdout would parse as commit rows | `verdict.sh:194`, `probe_verdict` redirections |
| C6  | FROM may be a remote-tracking ref (`origin/main`), not in the local candidate set | `resolve_from:34`; `candidate_index` returns −1 |
| C7  | Output contract is load-bearing: the VERDICTS column, Go parser `len(fields) < 14` (`status.go:168`), and the shell fixtures move together | `status.go:168`; `cmd/tests/test_print_agent_worktrees_status.sh` |
| C8  | Both CLIs are shell and stay interface-unchanged; the verdict lib is *sourced*, per the `routines/_lib/worktree.sh` precedent | concept [USER] packaging decision; `routines/_lib/` exists |
| C9  | The config-server UI reads the verdict from Go (`status.go`), which cannot re-run the probe cheaply per request | `status.go` `WorktreeStatus`, `SafeToRemove:49` |

## Options

### Axis A — the "already applied?" decision (layered probe)

- **A0 · patch-id only (status quo).** `git cherry` alone. Killed by **C1** — this *is* the bug.
- **A1 · re-pick, empty staged diff.** `cherry-pick --no-commit` onto FROM; empty `diff --cached` ⇒ `applied` (`probe_verdict:202`). Rides the existing detail-mode probe. Resolves the *clean* re-apply case only; a conflicting re-pick is ambiguous (an already-applied change self-conflicts, but so does a competing edit) — needs a layer 3.
- **A2 · re-pick with `-X theirs`, empty-diff test.** Force the pick through, test for empty diff. Killed by **C4**: `-X theirs` silently discards genuinely-unapplied conflicting hunks, producing false "applied" on the deletion gate. Explicitly rejected in the concept.
- **A3 · `git merge-tree` / three-way, no worktree.** Compute the merge tree in-object-db. Cheaper (no worktree churn) but carries the *same* ambiguity as A1's conflict case and adds no disambiguation; still needs a layer 3, and loses the lazy-worktree reuse (`ensure_probe_worktree:136`) that amortises C3.
- **A4 · range-diff pairing, raw `=`/`!`.** On conflict, `range-diff merge_base..FROM hash^..hash`; accept if any `=`/`!` pairing exists. Too loose — a competing edit to the same region pairs at default `--creation-factor` and false-passes (S7). Killed by **C4**.
- **A5 · range-diff + added-line interdiff guard (implemented layer 3).** Same pairing finder as A4, but accept `applied-resolved` only when the interdiff shows **no difference in added lines** (`interdiff_addition_verdict:174`): col-5 `+/-` with col-6 `+` ⇒ the agent's *content* isn't what landed ⇒ `unpicked`. Removed/context differences are inherent to a legitimate resolved pick and don't block. Fail-closed guards: no pairing ⇒ unpicked; empty `merge_base..FROM` range ⇒ unpicked (`probe_verdict:220`); pure-deletion with differing removed lines ⇒ unpicked; any binary hunk ⇒ unpicked. Satisfies **C4** with measured separation across the three ground-truth cases.
- **A6 · file/blob comparison.** Diff the final FROM tree against the agent commit for the touched paths. Coarse: a legitimately-resolved pick leaves the file *differing* from the agent's exact bytes (false unpicked), and unrelated later edits to the same file muddy the signal. Weaker than A5 on both error directions.

Scope sub-question (which refs to probe):

- **A-FROM · FROM only** (concept MVP). Cheap; catches the harvest-back-to-FROM path only. Misses a pick that landed with resolution on a *sibling* candidate (e.g. local `main` while FROM was `origin/main`).
- **A-WIDE · candidate-wide twin sweep** (implemented, beyond the concept MVP). For commits the FROM probe left `unpicked`, sweep every candidate (FROM first) for an exact-subject twin that `range-diff` pairs ⇒ `picked-resolved`; exact patch-present ⇒ `picked`; twin rejected ⇒ `unpicked`; no twin anywhere ⇒ `unpicked-notwin` (`twin_sweep:281`). Cost is bounded by C3 (only still-unpicked commits reach it) and adds auditable long-tail verdicts.

### Axis B — the unified verdict source

- **B1 · sourced shell lib, Go as consumer (implemented).** One predicate in `cmd/worktrees/_lib/verdict.sh`, `source`d by both scripts (`print_agent_worktrees_status.sh:60`, `remove_agent_worktrees.sh:38`); `status.go` *parses the rendered table* — `In` from field 10, Safe from `len(status.In) > 0` + dirty/untracked, `SafeViaProbe` from the `*` marker (`status.go:249-274`). Go never re-derives; it reads the shell's verdict. Rides **C8**'s precedent, keeps both CLIs unchanged, deletes the remove script's copy-pasted `contained_in()`/`count_untracked()`.
- **B2 · Go source of truth, shell shells out.** Both scripts call a compiled `worktree-verdict` binary. Inverts the dependency: a build artifact in a bash hook's hot path, cross-compile/deploy burden, and it breaks the plumbing-only common case (C3) and the "CLIs unchanged" contract (C8).
- **B3 · keep three copies, golden-test parity.** Leaves the drift alive and catches divergence only after it ships; violates the single-source-of-truth doctrine. Additive complexity, not a deletion.
- **B4 · generate all three from one spec.** Codegen a shell fn + a Go fn from a DSL. Overkill — a new toolchain concept for a ~200-line predicate.

## Evaluation

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|-------------|--------------|--------|---------------|---------|
| A0 patch-id only | rides all existing plumbing | none | 0 | — | **Reject** (C1 — the bug) |
| A1 re-pick empty-diff | reuses detail-mode probe | probe worktree per row | low | easy | **Keep as layer 2** — clean case only |
| A2 `-X theirs` | small | corrupts gate | low | easy | **Reject** (C4 false-safe) |
| A3 merge-tree | novel path | loses lazy reuse | med | easy | Reject — same ambiguity, worse on C3 |
| A4 range-diff raw | rides range-diff | corrupts gate | low | easy | **Reject** (C4 S7 false-pass) |
| **A5 range-diff + added-line guard** | rides range-diff + awk | contained, fail-closed | med | easy | **Adopt — layer 3** |
| A6 blob compare | novel | two-sided error | med | easy | Reject — worse than A5 both ways |
| A-FROM only | cheapest | — | low | easy | Keep for the common path |
| **A-WIDE twin sweep** | rides cherry + range-diff | bounded by C3 | med | easy | **Adopt for FROM-miss fallback** |
| **B1 sourced lib, Go consumer** | rides `routines/_lib` precedent | deletes 2 dup fns | low–med | easy | **Adopt** |
| B2 Go source of truth | inverts stack | binary in hook path | high | hard | Reject (C3, C8) |
| B3 test-locked triplicate | none | drift survives | med | easy | Reject (doctrine) |
| B4 codegen | none | new toolchain | high | hard | Reject — overkill |

**Per-scenario notes.**
- *Harvest back to the branch's own FROM* (the dominant case, incl. the f76262 incident): **A1 + A5 against A-FROM** fully covers it. The twin sweep never fires.
- *Pick landed with resolution on a sibling candidate* (FROM `origin/main`, work merged via local `main`): only **A-WIDE** recovers it as `picked-resolved`; without it the branch reads unsafe despite the work being safe.
- *Reworded / squashed transfer*: no detector sees it — **A-WIDE** surfaces this honestly as `unpicked-notwin` (verify manually) rather than a false Safe. Correct fail-closed behaviour, not a gap to fix.
- *Competing edit to the same region*: **A5**'s added-line guard is the single thing standing between correct `unpicked` and a destroyed-work false-safe. It is the load-bearing decision of the whole design.

## Recommendation

Adopt the **layered probe A1→A5 with the A-WIDE twin sweep as the FROM-miss fallback**, unified behind **B1 (sourced `_lib/verdict.sh`, Go as parser)**.

What fdesign imports:
- **Layers, strictest first, short-circuiting:** `picked` (C1 patch-id) → `applied` (A1 empty re-pick) → `applied-resolved` (A5 range-diff + added-line interdiff guard) → twin sweep (`picked` / `picked-resolved` / `unpicked` / `unpicked-notwin`).
- **Binding constraints:** C4 (fail-closed on any added-content difference, no pairing, empty range, pure-deletion mismatch, binary) is non-negotiable — it is the gate's correctness. C3 (probe only non-empty FROM `+` sets; lazy reused probe worktree) is the cost budget. C5 (silence probe git output) is a parser-safety invariant. C7 (VERDICTS column + `≥14`-field parser + fixtures move together) is the output contract.
- **Source of truth:** one `verdict.sh`, both scripts source it, Go consumes the rendered table (`status.In`, `SafeViaProbe`). Delete the remove script's `contained_in()`/`count_untracked()` — deletion, not a guard.
- **Measurements to carry:** the added-line separation must hold across the three ground-truth cases (real-incident accept, competing-edit-resolved accept, competing-change reject); the `WORKTREE_STATUS_TRACE=1` per-row probe count/duration hook makes the cache decision measurable before any cache is built.

## Rejected

- **A0 patch-id only** — is the defect (C1).
- **A2 `-X theirs` empty-diff** — silently drops unapplied hunks → false-safe on a deletion gate (C4).
- **A3 merge-tree** — same conflict ambiguity as A1 with no disambiguation; loses lazy worktree reuse (C3).
- **A4 raw range-diff pairing** — false-passes a competing edit at default creation-factor (C4/S7).
- **A6 blob/file comparison** — false-unpicked on legitimate resolutions and false-safe from unrelated edits; worse than A5 on both error directions.
- **B2 Go source of truth** — compiled binary in a bash hook's hot path; breaks plumbing-only speed (C3) and the CLIs-unchanged contract (C8).
- **B3 test-locked triplicate** — leaves the drift alive; catches divergence post-hoc, doesn't prevent it (doctrine).
- **B4 codegen from a spec** — new toolchain for a ~200-line predicate; overkill.

## Open Questions

*(Unattended run — no user present; assumptions recorded, not blocked on.)*

- **A-WIDE scope exceeds the concept's stated MVP.** The concept scopes layer 3 to "FROM only" and backlogs candidate-wide probing. The grounded code (`twin_sweep`, `picked-resolved`, `unpicked-notwin`) already probes candidate-wide for FROM-miss commits. **Assumption:** the twin sweep is in scope because it closes the sibling-candidate false-unsafe (the second per-scenario case) at a cost bounded by C3 — it runs only for commits already unpicked against FROM, and only over their exact-subject twins. If fdesign must honour the MVP boundary literally, drop A-WIDE and ship A-FROM only; the sibling case then reads unsafe (conservative, never false-safe).
- **Cache deferral stands.** No persistent probe cache in the MVP (concept [USER] decision). **Assumption honoured:** ship uncached with the trace hook; the `$GIT_DIR/agent-status-cache` row cache already present keys on tip|FROM|from-tip|candidates-digest and is a pure additive retrofit, consistent with the deferral.


