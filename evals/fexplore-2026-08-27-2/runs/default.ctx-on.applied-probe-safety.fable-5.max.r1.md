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

# Applied-probe safety for agent-worktree branches — Exploration

Concept: [plans/archived/applied_probe_safety/concept/concept.md](../../archived/applied_probe_safety/concept/concept.md) · mode: familiar · unattended run (assumptions under Open questions)

## Context

- **Open question:** which per-commit mechanism makes UNPICKED/IN/Safe reflect actually-transferred work — including conflict-resolved cherry-picks — and where does the single safety predicate live so the overview, the unpicked drill-down, and `remove_agent_worktrees.sh` can never disagree.
- **Driver — the incident:** agent commit 8875782 landed on main as 9a2098e (2026-07-24, verified in this clone) via a conflict-resolved pick; the resolution changed context lines, changed the patch-id, and `git cherry` pinned the branch unsafe forever. The agent branch is deleted — 8875782 no longer resolves here, so ground-truth fixtures must be synthetic.
- **Driver — predicate drift at baseline** (`c4b5561^`, the state the concept describes): three copies — the status script's cherry-set logic (display-only probe in detail mode, old:222–241), the remove script's own first-match `contained_in()` (old:56–70) plus a 3-exclude `count_untracked()` (old:84–87), and the Safe restatement in internal/repos/status.go.
- **Timeline disclosure:** the concept's MVP landed 2026-07-26 (c4b5561) and evolved since — candidate-wide twin sweep (707428a), claude-routines namespace (3b54be8), SHA-keyed row cache (59bd133). This exploration re-surveys the space under the concept's constraints and evaluates the landed design as the incumbent candidate, not as a presumed answer.
- **Space-shapers:** a rejected first mechanism (patch-id only), a rejected disambiguator (`-X theirs`), an implementation-refuted disambiguator (raw range-diff pairing, stop condition S7), and a hard fail-closed requirement on a deletion gate.

## Constraints

| ID | Constraint | Source |
|----|-----------|--------|
| C1 | The deletion gate fails closed: misclassifying a genuinely-unapplied conflicting change as safe corrupts removal — the worst outcome, worse than any false-unsafe | concept.md:46; S7 revision concept.md:127 |
| C2 | Scripts run under macOS bash 3.2 (hooks, launchd): no associative arrays, no mapfile — parallel indexed arrays only | ACDSL-SHELL-002 headers; cmd/worktrees/_lib/verdict.sh:2-3,16 |
| C3 | Deployment topology is fixed: scripts run standalone (sourced-lib precedent routines/_lib/worktree.sh; the remove script ships to other repos via agent-toolset — 9a2098e subject line), and Go shells out to the scripts (internal/repos/status.go:101-109), never the reverse | cmd/worktrees/_lib/; internal/repos/ops.go:81 |
| C4 | FROM may be a remote-tracking ref (Desktop records `Created from refs/remotes/origin/main`) or absent — FROM is resolved from the branch reflog, never guessed | verdict.sh:31-49; print_agent_worktrees_status.sh:16-18 |
| C5 | The candidates rule is fixed: local non-agent branches only (claude/*, claude-routines/* excluded); IN's per-candidate meaning stays untouched | verdict.sh:18-28; concept.md:122 |
| C6 | Output contract: fixed-width table; the Go parser hard-errors on any row under 14 fields (stop condition S8) and shell tests lock row shapes — every new column is a coordinated breaking change across printf template, parser, and all fixtures | status.go:164-170; cmd/tests/test_print_agent_worktrees_status.sh |
| C7 | Probe git output must be fully silenced — stray `Auto-merging`/`CONFLICT` lines on stdout parse as commit rows in the drill-down | verdict.sh:193-195; parseCommits status.go:115-133 |
| C8 | Cost posture: probes run only for rows whose FROM cherry-`+` set is non-empty — a fully harvested branch pays nothing; probe cost is measurable before any caching | concept.md:16; verdict.sh evaluate_branch:406-417; trace print script:262-264 |
| C9 | Auditability: heuristic upgrades are visibly distinct from exact matches, never silently folded in | concept.md:18; VERDICTS column, IN stars, UI badges _worktree_status.html:99-103 |
| C10 | macOS has no flock(1): any concurrent cache write must be lock-free | concept.md:108 |
| C11 | Verdict state travels via globals, not `$()` — a subshell discards the verdict memo and trace counters and leaks the probe worktree past cleanup | verdict.sh:191-196; remove_agent_worktrees.sh:92-94 |
| C12 | Patch-id equality is irrecoverably lossy under manual conflict resolution — changed context lines change the patch-id; no tuning of `git cherry` recovers containment | incident (8875782→9a2098e); git patch-id semantics |

## Options

### Family A — per-commit applied verdict (the layered probe)

**A0 — patch-id only (baseline status quo).** `git cherry` `+`/`-` sets against every candidate; UNPICKED = intersection of `+` sets. Cheap plumbing, zero probe cost. Killed by C12: a single resolved pick pins the branch unsafe permanently (the incident). Baseline anchor: `git show c4b5561^:cmd/worktrees/print_agent_worktrees_status.sh` old:21-22, 283-286.

**A1 — re-pick probe on FROM.** For each commit in FROM's cherry-`+` set: `cherry-pick --no-commit` onto FROM in a lazy detached temp worktree; empty staged diff ⇒ `applied` (change already present), non-empty ⇒ genuinely absent. Pre-existed as the display-only detail-mode probe (old:222-241); incumbent layer 2 (verdict.sh:196-229). Resolves the clean already-applied case only; a conflicting re-pick stays ambiguous (an applied change self-conflicts, but so does a competing change — C1 forbids guessing). Binds C4 (FROM-only target), C7, C8, C11.

**A2 — conflict disambiguation for the re-pick (layer 3).** Sub-family, all assuming A1 fired and conflicted:
- **A2a — `-X theirs` re-pick + empty-diff test.** Auto-resolves in FROM's favor; empty result ⇒ applied. Killed by C1: it silently discards genuinely-unapplied hunks, producing false `applied` verdicts on the deletion gate (concept.md:124).
- **A2b — raw `range-diff` `=`/`!` pairing.** `git range-diff <merge-base>..FROM <hash>^..<hash>`; any pairing ⇒ applied-resolved. Implementation-refuted: at default `--creation-factor` it false-passes a competing change to the same region (S7) — a false-safe, killed by C1 (concept.md:127).
- **A2c — range-diff pairing gated on an addition-payload-identical interdiff (incumbent).** range-diff finds the pairing partner; accept `applied-resolved` only when the interdiff shows no added-line difference between the two patches — removed/context drift is inherent to a legitimate resolution and does not block. Fail-closed guards: no pairing ⇒ unpicked; pure-deletion commit with differing removed lines ⇒ unpicked; any binary hunk ⇒ unpicked (verdict.sh:174-189, 196-229). A genuinely-unapplied change has no pairing partner (an empty `merge-base..FROM` range short-circuits to unpicked, verdict.sh:216-224). Satisfies C1 with measured separation across the three ground-truth cases (incident accept, competing-edit resolved-pick accept, competing change reject).

**A3 — candidate-wide subject-twin sweep (incumbent extension, 707428a).** For a commit even the FROM probe could not place: on each candidate, exact patch presence (cherry `-`) ⇒ picked; else a commit with the identical subject line since the merge-base that range-diff pairs with the agent commit ⇒ `picked-resolved`. Splits the residue into `unpicked` (a twin exists but its content differs) and `unpicked-notwin` (no twin anywhere — reworded/squashed transfers are invisible, said honestly in the drill-down, print script:193-196). Extends coverage beyond FROM to any local candidate (verdict.sh:281-332); binds C5 (reads candidates, never changes the rule), C9 (distinct verdict + IN stars).

**A4 — merge-tree simulation instead of a probe worktree.** `git merge-tree --write-tree` (git ≥ 2.38) computes the pick as an in-memory three-way merge — no checkout I/O, no temp worktree, trivially parallel. Verdict mapping: result tree equal to FROM's tree ⇒ applied; conflict ⇒ same A2c disambiguation. Changes probe *mechanics* only, not verdict semantics — swappable inside `probe_verdict()` behind the same globals (C11). No in-repo precedent; the git-version floor is unverified in this environment (see Open questions). Live as a cost contingency, not a competitor on correctness.

**A5 — transfer-time provenance (`-x` trailers or git notes).** Stamp picks at harvest so containment becomes a log lookup: `cherry-pick -x` writes `(cherry picked from commit <sha>)` on the candidate side. Reality: cherry_pick_worktree.sh:30 stamps nothing today; the incident predates any stamp (retroactively blind); Desktop merges, squashes, and manual applies bypass the tool entirely. Owner of the change: this repo (transfer script). Survives only as an optional forward-looking complement — never the gate's basis (coverage gaps violate the spirit of C1's fail-closed reasoning in reverse: unstampable transfers would read unsafe forever, recreating the incident class).

**A6 — content-presence search (pickaxe/blame).** `git log -S` per added line, or blame-based presence of the commit's payload on FROM. Per-line cost, no pairing semantics, and partial-presence false positives under C1. No verdict vocabulary maps onto it cleanly. Killed.

### Family B — the unified verdict source (packaging)

**B1 — sourced shell lib, both scripts consume it (incumbent, [USER] concept.md:126).** `cmd/worktrees/_lib/verdict.sh` owns candidates, FROM resolution, cherry sets, the layered probe, the twin sweep, the untracked filter, and `evaluate_branch`; the status script (print:60) and the remove script (remove:38, :95, :127) source it; the remove script's own `contained_in()`/`count_untracked()` are deleted, not guarded around. Go stays a parser of the script's output. Follows the routines/_lib/worktree.sh precedent; satisfies C2, C3, C11.

**B2 — Go verdict oracle, scripts exec it.** Implement verdicts in internal/repos; scripts call the binary. Killed by C3: it inverts the fixed dependency direction — the remove script ships standalone via agent-toolset to repos with no Go build, and hooks/launchd contexts run scripts without the module built.

**B3 — remove script parses the status script's table.** One computation, but couples removal to the display contract (C6's most fragile surface), doubles process cost per branch, and loses per-branch reasons. Killed.

**B4 — machine-readable verdict records as the shared surface.** A porcelain mode or sidecar records consumed by both scripts and Go. The landed row cache (59bd133: 5-line records under `<git-common-dir>/agent-status-cache/`, keyed `tip|from|from_tip|candidates-digest`, lock-free per C10) is a de-facto partial B4 — but as an optimization behind `evaluate_branch`, not as the contract. Formalizing it into the contract is unneeded while B1 holds; revisit only if a consumer appears that cannot source bash (see per-scenario notes).

### Family C — surfacing Safe without touching the candidates rule

**C-a — annotated FROM entry in IN plus per-candidate stars (incumbent).** `len(In)>0` keeps carrying Safe; a starred entry (`origin/main*`, `main*`) marks containment-via-verdicts; VERDICTS column carries the per-row breakdown (`applied:n,resolved:n,picked-resolved:n`). Binds C5, C6 (one coordinated break, done), C9. verdict.sh:443-473; status.go:226-272.
- Residual: status.go restates the Safe arithmetic from parsed columns (status.go:259-267). This is a *derivation from the documented header contract* (print script:48-50 names both consumers; status_test.go:48-93 asserts it), not a fourth independent predicate — the divergence surface is one boolean combination, pinned by cross-referenced comments and tests.
- **C-b — add a remote-tracking FROM to the candidate set.** Killed by C5: changes IN's per-candidate meaning everywhere and multiplies cherry cost (concept.md:52).
- **C-c — script-emitted SAFE column so Go stops restating.** Live but not worth it now: a second C6 breaking change buys removal of a restatement whose drift risk is already pinned; becomes attractive only bundled with another contract change or a third Safe consumer.

## Evaluation

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|--------------|--------------|--------|---------------|---------|
| A0 patch-id only | max (was the code) | none | 0 | — | **Killed** — C12, the incident |
| A1+A2c layered probe | high — factors out the pre-existing detail probe; shipped with 633-line shell tests + replaced Go fixture | medium — output contract break (C6), both scripts | ~2.5–3.5d (concept MVP) | medium — verdict semantics are load-bearing once UI/removal trust them | **Winner** (incumbent, validated) |
| A2a `-X theirs` | low | small | ~0.5d | high | **Killed** — false-safe (C1), concept.md:124 |
| A2b raw pairing | low | small | ~0.5d | high | **Killed** — S7 false-safe (C1), concept.md:127 |
| A3 twin sweep | high — same range-diff machinery, shipped 707428a | small — additive verdicts + IN stars | ~1d | high — drop the sweep, verdicts degrade to unpicked | **Winner as extension** |
| A4 merge-tree probe | low — no in-repo precedent, git floor unverified | small — internal to probe_verdict() | ~1d + verification | high — same globals contract | **Contingency** — only on measured probe cost |
| A5 `-x` provenance | low — transfer tool stamps nothing today | small (tool change) | ~0.5d | high | **Complement only** — retroactively blind, coverage gaps |
| A6 content search | none | small | ~1–2d | high | **Killed** — cost + false presence (C1) |
| B1 sourced lib | high — routines/_lib precedent; shipped | medium — both scripts restructured | ~1–1.5d | medium | **Winner** ([USER]) |
| B2 Go oracle | low | large — dependency inversion | ~2d+ | low | **Killed** — C3 |
| B3 parse the table | low | medium | ~0.5d | medium | **Killed** — C6 fragility |
| B4 record contract | medium — row cache is a partial de-facto form | medium | ~1d | high | **Not now** — backlog trigger below |
| C-a annotated IN + VERDICTS | high — shipped, tested end to end | medium — the one C6 break | ~1–1.5d | medium | **Winner** ([USER]) |
| C-b FROM into candidates | low | large — IN semantics everywhere | ~0.5d | low | **Killed** — C5 |
| C-c SAFE column | medium | medium — another C6 break | ~0.5d | medium | **Deferred** — bundle with the next contract change |

Per-scenario notes:

- **Probe cost:** the recurring cost is active branches with genuinely unpicked commits re-probing per overview load (C8's only leak). The measurement path ran its course in reality: trace hook shipped in the MVP, and the SHA-keyed row cache landed 2026-08-26 (59bd133) with a warm-run byte-identical test (test file :495-498). If cost resurfaces despite the cache (e.g. candidate-digest churn invalidating all rows), A4 is the next lever — mechanics-only, C1-neutral.
- **Server-side removal assertion (concept backlog):** a Go-side gate for the config-server Remove button would today re-trust the parsed columns. That is the trigger that makes B4 (formal record contract) or C-c (SAFE column) worth their contract break — one of them, not both.
- **Reworded/squashed transfers:** invisible to every admissible family — A3 names the residue `unpicked-notwin` and the drill-down says exactly what to do (verify via `git log --all --grep`, print script:193-196). Accepted residual; A6 would trade it for C1 violations.
- **Trailer hygiene:** A5 is compatible with everything above and costs one flag in cherry_pick_worktree.sh — worth doing opportunistically, but it must never upgrade a verdict on its own (an unstamped transfer is not evidence of absence).

## Recommendation

- **Confirm the incumbent stack:** layered per-commit verdict, strictest first, short-circuiting — patch-id (`picked`) → FROM re-pick probe (`applied`) → range-diff pairing gated on an addition-payload-identical interdiff (`applied-resolved`) — extended by the candidate-wide subject-twin sweep (`picked-resolved` / `unpicked` / `unpicked-notwin`); packaged as the sourced shell lib `cmd/worktrees/_lib/verdict.sh` (B1) consumed by both scripts; surfaced via annotated IN entries plus the VERDICTS column (C-a). This re-survey found no admissible family the constraints prefer.
- **What fdesign imports:** constraints C1–C12; the fail-closed guard set (no pairing / pure-deletion mismatch / binary ⇒ unpicked); the three ground-truth separation cases (incident accept, competing-edit resolved-pick accept, competing same-region change reject); the measurement-before-cache posture (already vindicated: trace → row cache 59bd133); the globals-not-subshell calling convention (C11).
- **Standing contingencies (recorded to prevent re-exploration):** A4 merge-tree probe mechanics iff trace shows probe cost dominating despite the row cache AND git ≥ 2.38 is verified; B4-or-C-c (exactly one) iff a Go-side removal assertion lands; A5 `-x` stamping as an opportunistic complement that never upgrades verdicts.

## Open questions

Unattended-run assumptions, recorded per the brief:

- **Re-survey framing assumed.** The brief describes the pre-MVP state ("three drifting copies"), but the tree at HEAD (523b699) contains the shipped implementation plus follow-ups. Assumed the exercise wants a fresh survey of the space under the concept's constraints with the landed design evaluated as the incumbent candidate — the only framing that keeps every anchor honest.
- **git version unverified.** `git version` / `git merge-tree -h` are approval-gated in this unattended cell; A4's ≥ 2.38 floor is reasoned, not measured. Verify before pursuing A4.
- **Prohibition scope.** Read nothing under plans/archived/applied_probe_safety/design/; verdict.sh:173's in-code citation of that path was noted but not followed.
- **Incident fixtures.** 8875782 no longer resolves in this clone (agent branch deleted) — reproduction relies on the synthetic shell fixtures, consistent with the concept's decision to replace the impossible Go fixture with captured script output.

## Rejected

- **A0 patch-id only** — a single conflict-resolved pick pins the branch unsafe forever (C12; the 8875782→9a2098e incident).
- **A2a `-X theirs` empty-diff** — silently discards genuinely-unapplied hunks: false `applied` on a deletion gate (C1; concept.md:124).
- **A2b raw range-diff pairing** — implementation-proved false-pass on a competing change to the same region (S7, C1; concept.md:127).
- **A6 content-presence search** — per-line cost, no pairing semantics, partial-presence false positives (C1).
- **B2 Go verdict oracle** — inverts the fixed dependency direction; the remove script ships standalone via agent-toolset and must run without a Go build (C3).
- **B3 remove-parses-the-table** — couples the deletion gate to the most fragile display contract (C6) for no semantic gain.
- **C-b FROM into the candidate set** — changes IN's per-candidate meaning everywhere; the annotated-entry mechanism achieves the upgrade without touching the rule (C5; concept.md:52).
- **A5 as the gate's basis** — retroactively blind (predates the incident), blind to Desktop merges/squashes/manual applies; demoted to optional complement.
- **C-c SAFE column now** — a second output-contract break to remove a restatement whose drift risk is already pinned by the documented header contract and tests; deferred to the next contract change.


