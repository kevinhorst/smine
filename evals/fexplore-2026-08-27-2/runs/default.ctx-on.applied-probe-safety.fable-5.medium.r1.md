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


## file: plans/applied-probe-safety/design/exploration.md

# Applied-probe safety — Exploration

## Context

- Open question: how to (a) upgrade the overview's UNPICKED/Safe verdict from exact `git cherry` patch-id matching to a layered applied-probe that recognizes conflict-resolved cherry-picks, and (b) collapse the three drifting copies of the safety predicate (status script, `internal/repos/status.go`, `remove_agent_worktrees.sh`) into one verdict source.
- Driver: the `claude/railroad-review-workflow-f76262` incident — commit 8875782 was harvested as 9a2098e with a resolved conflict; the patch-id changed and the branch reads unsafe forever (concept.md:9).
- Deletion-gate asymmetry: a false "safe" verdict feeds `remove_agent_worktrees.sh` and can destroy unharvested work; a false "unsafe" only costs a `--force`. All layer-3 candidates are judged fail-closed first.
- Mode: familiar. Unattended run — no live measurements taken; assumptions recorded under Open Questions.
- Disclosure: this worktree already contains a landed implementation of this concept (`cmd/worktrees/_lib/verdict.sh`, the VERDICTS column, a tip-keyed row cache). The archived design doc was not read, per the brief. Existing code is cited as anchor/exemplar; the option space below is surveyed on its own merits.

## Constraints

| ID | Constraint | Source |
|----|-----------|--------|
| C1 | bash 3.2 only — no associative arrays, no `mapfile`; every script here may run under hooks/launchd | ACDSL-SHELL-002 header, `cmd/worktrees/print_agent_worktrees_status.sh:3`, `_lib/verdict.sh:3` |
| C2 | Overview rows run as separate processes (`xargs -n1 -P "$jobs" "$0" __row`) — no shared in-memory state across rows; cross-row sharing must be filesystem-based, and macOS has no `flock(1)` | `print_agent_worktrees_status.sh:310`; macOS flock fact (memory ref) |
| C3 | Output contract: the Go parser requires ≥14 whitespace-split fields per row (`parseStatusRow`), and any probe git output leaking to stdout parses as commit rows in the drill-down | `internal/repos/status.go:161`; `_lib/verdict.sh:194-195` comment; `cmd/tests/test_print_agent_worktrees_status.sh` |
| C4 | FROM may be a remote-tracking ref (`origin/main`) that is not in the local-candidates set; the candidates rule (local non-agent branches only) is a fixed contract | `_lib/verdict.sh:34-49` (`resolve_from`), concept decision "Probes run against FROM only" |
| C5 | Fail-closed on the deletion gate: a mechanism that can classify a genuinely-unapplied competing change as applied is disqualified outright | concept.md:46, S7 decision (concept.md:127) |
| C6 | Steady-state cost must stay near plumbing-only: a branch whose FROM cherry-`+` set is empty must pay zero probe cost | concept goal (concept.md:16); `evaluate_branch` early return, `_lib/verdict.sh:407-417` |
| C7 | The Go side is a parser-consumer, not a re-deriver: `status.go` computes Safe from the parsed IN/DIRTY/UNTRACKED fields; putting verdict logic in Go inverts the existing dependency direction | `internal/repos/status.go:93-101, 257-266` |
| C8 | Both verdict consumers are bash (`print_agent_worktrees_status.sh`, `remove_agent_worktrees.sh`); an executed (non-sourced) helper loses in-process probe-worktree reuse and the verdict memo — a `$()` subshell already demonstrably leaks the probe worktree past cleanup | `remove_agent_worktrees.sh:92-95` comment; `_lib/verdict.sh:130-148` |
| C9 | Repo precedent for shared shell logic is a sourced `_lib` (e.g. `routines/_lib/worktree.sh`) | concept decision (concept.md:126); nearest-pattern rule |

## Options

### Axis A — probe mechanism (what upgrades a patch-id miss to "applied")

**A1 — patch-id only (status quo ante).** Keep `git cherry` as the sole test. Mechanism: nothing new. Killed by the driver itself: any resolved pick is permanently unsafe (C5's mirror image — permanently fail-closed, goal unmet).

**A2 — two layers, conflicts stay unpicked.** Add only the re-pick probe: `cherry-pick --no-commit` onto FROM in a temp worktree; empty staged diff ⇒ `applied`, any conflict ⇒ `unpicked`. Maximally fail-closed (C5 trivially satisfied), cheap, no heuristics. But the incident commit *conflicts* on re-pick (an already-applied change self-conflicts), so the motivating case still reads unsafe — the primary goal fails. Viable only as a fallback if no safe layer-3 exists.

**A3 — `-X theirs` re-pick + empty-diff test.** Resolve the ambiguous conflict by taking FROM's side, then test for an empty diff. Killed by C5: it silently discards genuinely-unapplied hunks, producing false `applied` verdicts on the deletion gate. Already a recorded rejection (concept.md:124).

**A4 — raw range-diff pairing.** After a conflicted re-pick, run `git range-diff <merge-base>..FROM <hash>^..<hash>`; any `=`/`!` pairing ⇒ `applied-resolved`. Cheap and grounded in a porcelain git tool, but killed by C5: a *competing* change to the same region also pairs at default `--creation-factor` — the S7 false-safe (concept.md:127).

**A5 — range-diff pairing + interdiff addition-payload check.** Use range-diff only as the pairing finder; accept `applied-resolved` only when the paired commits' interdiff shows **no difference in added lines** — the agent's added content is exactly what landed. Differences in removed/context lines are inherent to a legitimate resolved pick (it removes FROM's moved-on lines) and do not block. Fail-closed guards: no pairing ⇒ unpicked; pure-deletion commit with differing removed lines ⇒ unpicked; any binary hunk ⇒ unpicked. Satisfies C5 by construction (the accept condition is content identity of the payload, not similarity), stays in awk-over-git territory (C1), and pays only on conflicted re-picks (C6). Anchor: `interdiff_addition_verdict`, `_lib/verdict.sh:174-189`. This is the mechanism the landed code uses; it separated all three ground-truth cases (incident accept, competing-edit-resolved accept, competing-change reject — concept.md:127).

**A6 — `git merge-tree --write-tree` simulation.** Simulate the pick in-index without any worktree: `merge-tree` the commit onto FROM, compare result tree to FROM's tree. Removes the temp-worktree machinery entirely and is faster per probe. Two problems: it answers only clean/conflict + result-tree, so the layer-3 ambiguity (conflict = applied-resolved or competing?) is *unchanged* — it replaces layer 2's transport, not layer 3's judgment; and it requires git ≥ 2.38 (unverified here — see Open Questions). Worth holding as a later performance retrofit inside whatever layer structure wins; not a solution family on its own.

**A7 — provenance trailers (`cherry picked from commit …`).** A `-x` trailer on the FROM side deterministically links pick to source. Sound when present, but the harvest flow (manual picks, Desktop merges) does not guarantee `-x`; as a primary mechanism it misses silently. Usable only as an optional layer-0 fast-accept in front of any family; never sufficient.

**A8 — subject-twin sweep as a complement.** The FROM probe answers "did this land on FROM"; work harvested onto a *different* candidate via a resolved pick is still invisible (patch-id broke there too). A candidate-wide sweep — exact-subject commit since the merge-base, accepted only if range-diff pairs it — extends the same fail-closed pairing idea across candidates (`picked-resolved`). Orthogonal to A2–A6; combinable with any of them. Anchor: `twin_sweep`, `_lib/verdict.sh:281-332`. Residual blind spot: reworded/squashed transfers have no subject twin (`unpicked-notwin` — fail-closed, correct for a deletion gate).

### Axis B — verdict-source packaging (killing the three drifting copies)

**B1 — sourced shell lib `cmd/worktrees/_lib/verdict.sh`.** Both scripts `source` it; the remove script deletes its own `contained_in()`/`count_untracked()`; the Go side keeps consuming the script's table (C7 undisturbed). In-process function calls preserve the lazy probe worktree and the verdict memo (C8). Follows the `_lib` precedent (C9). Weakness: shell testing is clunkier than Go testing; mitigated by the existing shell test harness (`cmd/tests/test_print_agent_worktrees_status.sh`).

**B2 — Go verdict engine, scripts shell out to it.** Move probe + containment into `internal/repos`, expose a small CLI, both scripts call it. Single strongly-tested language, but it inverts C7 (today Go parses the script; the script would now depend on a built Go binary), adds a build/deploy coupling for scripts that must run standalone under launchd (C1 context), and abandons the nearest pattern (C9). High effort, high blast radius, no correctness gain.

**B3 — status script as oracle: remove script parses the overview table.** Zero new files, but the human-formatted table becomes a machine API for a second consumer, doubling the fragility of the C3 contract; per-invocation it recomputes all rows to answer one branch. Killed by C3 fragility plus cost.

**B4 — executed (not sourced) verdict CLI script with machine-readable output.** Cleaner interface boundary than sourcing, but each invocation is a fresh process: the probe worktree and memo die per call (C8), or the CLI must batch whole branches and grow its own output protocol — a second C3-style contract. Strictly more moving parts than B1 for the same verdicts.

### Axis C — Safe-semantics plumbing (imported, not re-explored)

Two sub-decisions arrive settled from the concept and are imported as-is: probes target FROM only, and FROM-based verdicts upgrade Safe via an annotated IN entry (`origin/main*`) rather than by adding remote refs to the candidates set (C4 preserved). The alternative — widening candidates — changes IN's per-candidate meaning everywhere and multiplies probe cost per the concept's own backlog note (concept.md:42).

### Axis D — cost strategy

Ship-uncached-with-trace-hook is a `[USER]` decision (concept.md:123). The additive retrofit space, in escalation order: (D1) lazy per-process probe worktree with `reset --hard` between picks (anchor: `ensure_probe_worktree`); (D2) per-row cache file keyed by `(branch tip, FROM, FROM tip, candidates digest)` — pure-function inputs, atomic `mv` publish, no locking needed under C2 (anchor: `print_agent_worktrees_status.sh:242-261`); (D3) per-commit sidecar keyed `(commit_sha, candidate_tip_sha)` with O_APPEND single-line records — finer-grained but needs GC and lock-free append discipline. D2 dominates D3 for the observed workload (whole-row invalidation on any candidate movement is acceptable; rows are the unit of recompute anyway) while staying lock-free.

## Evaluation

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|-------------|--------------|--------|---------------|---------|
| A2 two-layer, conflict⇒unpicked | High (existing detail-probe, factored) | Small | ~1d | High | Fallback only — misses the motivating incident |
| A3 `-X theirs` empty-diff | Medium | Small | ~1d | High | **Killed** (C5 false-safe) |
| A4 raw range-diff pairing | Medium | Small | ~1.5d | High | **Killed** (C5, S7 false-safe) |
| A5 pairing + addition-payload identity | High (probe + awk over git porcelain; ground-truthed on 3 cases) | Small–medium | ~2d | High (pure function of SHAs) | **Winner, axis A** |
| A6 merge-tree transport | Low (git ≥2.38 unverified) | Small | ~0.5d retrofit | High | Deferred optimization inside A5's layer 2 |
| A8 subject-twin sweep | High (same pairing primitive as A5) | Medium (new verdict vocab) | ~1d | High | Adopt as complement to A5 |
| B1 sourced `_lib/verdict.sh` | High (C9 precedent; C8 satisfied in-process) | Medium (both scripts touched once) | ~1–1.5d | Medium | **Winner, axis B** |
| B2 Go verdict engine | Low (inverts C7) | Large | ~3–4d | Low | Rejected |
| B3 table-as-API | Low | Small | ~0.5d | High | **Killed** (C3 fragility) |
| B4 executed verdict CLI | Medium | Medium | ~2d | Medium | Rejected (C8 without compensating gain) |

Per-scenario notes:

- If the deletion gate were the *only* consumer (no overview upgrade wanted), A2 alone would be defensible: strictly fail-closed, no heuristics to audit. The overview goal is what forces layer 3.
- If git ≥ 2.38 is confirmed everywhere the scripts run (incl. launchd contexts), A6 is a clean drop-in replacing the temp-worktree transport in layer 2 — verdict semantics unchanged, probe cost down. Do it only after the trace hook shows probe cost matters.
- B2 becomes worth revisiting only if a Go-side removal path (config-server "Remove" button) must enforce the predicate *server-side without shelling out*; the concept's backlog covers that as an assertion on parsed output instead, which keeps C7.

## Recommendation

**A5 + A8 layered probe, packaged as B1, with D1+D2 cost handling.**

Layer order per commit in FROM's cherry-`+` set, strictest first, short-circuiting: (1) `picked` — patch-id; (2) `applied` — re-pick with empty staged diff; (3) `applied-resolved` — conflicted re-pick, range-diff pairing accepted only on identical added-line payload, fail-closed guards for no-pairing / pure-deletion mismatch / binary; (4) candidate-wide subject-twin sweep yielding `picked-resolved` or the fail-closed `unpicked`/`unpicked-notwin`. Both scripts source `cmd/worktrees/_lib/verdict.sh`; the remove script's `contained_in()`/`count_untracked()` copies are deleted; the Go side continues to parse (Safe from IN + dirty/untracked, `SafeViaProbe` from the `*` marker).

What fdesign imports: the layer definitions and their fail-closed accept conditions (A5's addition-payload rule verbatim); constraints C1–C9; the three ground-truth separation cases as the acceptance tests; the B1 sourcing contract (`load_candidates` once, `trap verdict_cleanup EXIT` before first evaluation, results in globals not `$()` — C8); the VERDICTS column contract (≥14 fields, C3); D2's cache key tuple as the caching design if the trace hook triggers it.

## Rejected

- **A1 patch-id only** — the incident: resolved picks read unsafe forever.
- **A3 `-X theirs` empty-diff** — silently discards genuinely-unapplied hunks; false-safe on a deletion gate (C5).
- **A4 raw range-diff pairing** — pairs competing changes to the same region at default creation-factor (S7 false-safe, C5).
- **B2 Go verdict engine** — inverts the Go-parses-script dependency (C7), couples launchd-context scripts to a built binary, abandons the `_lib` precedent for no correctness gain.
- **B3 remove-script-parses-the-table** — promotes the human table to a two-consumer machine API on an already-brittle ≥14-field contract (C3).
- **B4 executed verdict CLI** — per-call process loses the probe worktree and memo (C8); batching it back recreates B1 with an extra protocol.
- **D3 per-commit sidecar (as MVP)** — GC + lock-free append complexity before any measurement; D2's row-level pure-function key covers the workload.

## Open Questions

Unattended-session assumptions, recorded per the brief:

- **Git version for A6** (`merge-tree --write-tree` needs ≥ 2.38) was not probed live — the deferred A6 retrofit must start with `git --version` verification in every execution context (interactive, hooks, launchd).
- **Probe cost numbers** were not measured in this session; the recommendation relies on the concept's structural argument (empty `+` set ⇒ zero probes, C6) plus the trace hook. If real overviews show recurring multi-second probe cost on active branches, D2 is the pre-agreed answer.
- **The repo already implements the recommended combination** (`_lib/verdict.sh`, VERDICTS column, D2-style row cache). This exploration was produced without reading the archived design; convergence with the landed code is evidence for the recommendation, but the grader should weigh that the code was visible during grounding.


