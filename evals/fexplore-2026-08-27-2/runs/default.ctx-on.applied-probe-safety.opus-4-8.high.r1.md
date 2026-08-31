## file: cmd/tests/test_print_agent_worktrees_status.sh

#!/usr/bin/env bash
# [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit
# - [ACDSL-SHELL-002] Scripts launched by hooks or launchd run under macOS bash 3.2 — no declare -A, mapfile/readarray, case expansion, |&, or parameter transformation (the launchd lesson)

# Regression coverage for print_agent_worktrees_status.sh FROM resolution: a
# branch created from a remote-tracking ref (Desktop records the full refname
# refs/remotes/origin/<branch>) must report that origin, and a branch whose
# reflog records no resolvable origin must report "unknown" with "-" counts —
# FROM is never guessed.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/wt-status.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == wt-status.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT="$REPO_DIR/cmd/worktrees/print_agent_worktrees_status.sh"

test_from_resolves_remote_tracking_origin() {
  local repo=$TMP/repo row
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base

  # remote-tracking ref at the base commit; the feature branch moves ahead so
  # a wrong FROM guess would show it with a nonzero BEHIND
  git -C "$repo" update-ref refs/remotes/origin/main main
  git -C "$repo" checkout -qb feature/extension
  printf '%s\n' work > "$repo/work"
  git -C "$repo" add work
  git -C "$repo" commit -qm work

  # reflog records "branch: Created from refs/remotes/origin/main"
  git -C "$repo" branch claude/from-origin refs/remotes/origin/main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/from-origin')
  echo "$row" | awk '{exit $3 == "origin/main" ? 0 : 1}' \
    || fail "FROM is not origin/main: $row"
  echo "$row" | awk '{exit $7 == "0" ? 0 : 1}' \
    || fail "BEHIND vs origin/main should be 0: $row"
}

test_from_without_recorded_origin_is_unknown() {
  local repo=$TMP/repo row
  # created from a raw sha: the reflog records the sha, not a ref — no guess,
  # FROM must be unknown and the counts "-"
  git -C "$repo" branch claude/mystery "$(git -C "$repo" rev-parse main)"

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/mystery')
  echo "$row" | awk '{exit $3 == "unknown" ? 0 : 1}' \
    || fail "FROM is not unknown: $row"
  echo "$row" | awk '{exit ($6 == "-" && $7 == "-") ? 0 : 1}' \
    || fail "AHEAD/BEHIND should be '-' when FROM is unknown: $row"
}

test_unpicked_list_has_no_probe_noise() {
  local repo=$TMP/repo2 row out
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/shared"
  git -C "$repo" add shared
  git -C "$repo" commit -qm base

  # claude branch (reflog origin: main): one commit that applies cleanly onto
  # main, one that conflicts (main rewrites the same line afterwards)
  git -C "$repo" branch claude/work main
  git -C "$repo" checkout -q claude/work
  printf '%s\n' clean > "$repo/new-file"
  git -C "$repo" add new-file
  git -C "$repo" commit -qm clean-commit
  printf '%s\n' agent-version > "$repo/shared"
  git -C "$repo" commit -qam conflict-commit

  git -C "$repo" checkout -q main
  printf '%s\n' main-version > "$repo/shared"
  git -C "$repo" commit -qam main-diverges

  out=$(cd "$repo" && bash "$SCRIPT" 1 unpicked)
  if echo "$out" | grep -qE 'CONFLICT \(|Auto-merging'; then
    fail "probe output leaked into the commit list: $out"
  fi
  # every line after the heading is a sha-prefixed commit line
  if echo "$out" | tail -n +2 | grep -qvE '^[0-9a-f]{7,40}  '; then
    fail "non-commit line in the list: $out"
  fi
  # both commits are genuinely unpicked on main: clean-commit applies with a
  # non-empty diff, conflict-commit is a competing change to the same line.
  # Neither carries an "applied" suffix — the vocabulary is applied-only now.
  if echo "$out" | grep -qE '\(applied on|conflict resolved\)'; then
    fail "genuinely unpicked commit wrongly annotated as applied: $out"
  fi
}

# UNTRACKED counts non-infrastructure untracked files in the worktree —
# .idea/, .claude/, .claude-worktree, .serena/ and .DS_Store never count,
# everything else does.
test_untracked_column_flags_non_infra_files() {
  local repo=$TMP/repo3 wt row
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base

  git -C "$repo" branch claude/wt main
  wt=$repo/.claude/worktrees/wt
  git -C "$repo" worktree add -q "$wt" claude/wt
  mkdir -p "$wt/.idea"
  printf '%s\n' x > "$wt/.idea/workspace.xml"
  printf '%s\n' x > "$wt/.claude-worktree"
  mkdir -p "$wt/.serena"
  printf '%s\n' x > "$wt/.serena/project.yml"
  printf '%s\n' x > "$wt/.DS_Store"
  printf '%s\n' x > "$wt/stray-artifact"

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/wt')
  echo "$row" | awk '{exit $4 == "0" ? 0 : 1}' \
    || fail "DIRTY should be 0 (only untracked files): $row"
  echo "$row" | awk '{exit $5 == "1" ? 0 : 1}' \
    || fail "UNTRACKED should be 1 (infra excluded): $row"
}

# MERGED reports an actual merge commit target only; IN lists every containing
# branch with FROM first, and a cherry-pick transfer leaves MERGED at "-".
test_merged_and_contained_in() {
  local repo=$TMP/repo4 row sha
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base

  # merged: claude/done is truly merged into develop; aaa-wrapper (sorts
  # before develop) contains the work only by merging develop afterwards —
  # the merge commit is reachable but not on its first-parent line
  git -C "$repo" branch claude/done main
  git -C "$repo" checkout -q claude/done
  printf '%s\n' work > "$repo/work"
  git -C "$repo" add work
  git -C "$repo" commit -qm work
  git -C "$repo" checkout -qb develop main
  git -C "$repo" merge -q --no-ff --no-edit claude/done
  git -C "$repo" checkout -qb aaa-wrapper main
  git -C "$repo" merge -q --no-ff --no-edit develop
  git -C "$repo" checkout -q main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/done')
  echo "$row" | awk '{exit $10 == "develop" ? 0 : 1}' \
    || fail "MERGED should be develop (actual merge target, not a downstream branch): $row"
  # IN: FROM (main) does not contain the work — containing set in ref order
  echo "$row" | awk '{exit $11 == "aaa-wrapper,develop" ? 0 : 1}' \
    || fail "IN should list all containing branches: $row"

  # cherry-pick transfer: same patch on a branch, but no merge commit
  git -C "$repo" branch claude/picked main
  git -C "$repo" checkout -q claude/picked
  printf '%s\n' picked > "$repo/picked"
  git -C "$repo" add picked
  git -C "$repo" commit -qm picked
  sha=$(git -C "$repo" rev-parse HEAD)
  git -C "$repo" checkout -qb feature/target main
  git -C "$repo" cherry-pick -x "$sha" >/dev/null
  git -C "$repo" checkout -q main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/picked')
  echo "$row" | awk '{exit $10 == "-" ? 0 : 1}' \
    || fail "MERGED must stay '-' for a cherry-pick transfer: $row"
  echo "$row" | awk '{exit $11 == "feature/target" ? 0 : 1}' \
    || fail "IN should show the cherry-pick target: $row"
}

# IN lists the FROM branch first even when other containing branches sort
# before it alphabetically.
test_contained_in_lists_from_first() {
  local repo=$TMP/repo5 row
  mkdir -p "$repo"
  git -C "$repo" init -q -b zz-base
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base

  # tip == zz-base == aaa-copy: both contain the (empty) work; FROM is zz-base
  git -C "$repo" branch aaa-copy zz-base
  git -C "$repo" branch claude/idle zz-base

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/idle')
  echo "$row" | awk '{exit $11 == "zz-base,aaa-copy" ? 0 : 1}' \
    || fail "IN must list FROM first: $row"
}

# The parallel overview (default jobs) must be byte-identical to a serial run
# (WORKTREE_STATUS_JOBS=1) — pins the order-preserving row reassembly.
test_parallel_output_matches_serial() {
  local repo=$TMP/repo serial parallel
  serial=$(cd "$repo" && WORKTREE_STATUS_JOBS=1 bash "$SCRIPT")
  parallel=$(cd "$repo" && bash "$SCRIPT")
  [ "$serial" = "$parallel" ] \
    || fail "parallel overview differs from serial run"
}

# Detail mode addressed by branch name equals the numbered form; an unknown
# branch fails with the script's error contract.
test_branch_name_selector() {
  local repo=$TMP/repo by_num by_name out
  by_num=$(cd "$repo" && bash "$SCRIPT" 1 ahead)
  by_name=$(cd "$repo" && bash "$SCRIPT" claude/from-origin ahead)
  [ "$by_num" = "$by_name" ] \
    || fail "branch-name selector output differs from numbered form"

  if out=$(cd "$repo" && bash "$SCRIPT" claude/nope ahead 2>&1); then
    fail "unknown branch did not fail: $out"
  fi
  echo "$out" | grep -q 'unknown branch claude/nope' \
    || fail "wrong unknown-branch error: $out"
}

# A context-shifted clean pick (patch-id differs because a nearby context line
# moved on main, but the re-pick applies with an empty diff) reads applied:1
# and adds the FROM* IN entry — layer 2.
test_applied_verdict_upgrades_row() {
  local repo=$TMP/repo-applied row
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf 'l1\nl2\nl3\nl4\nl5\n' > "$repo/f.txt"
  git -C "$repo" add f.txt
  git -C "$repo" commit -qm base

  # agent inserts a line after l3 — its patch context includes l5
  git -C "$repo" branch claude/applied main
  git -C "$repo" checkout -q claude/applied
  printf 'l1\nl2\nl3\nINSERTED-X\nl4\nl5\n' > "$repo/f.txt"
  git -C "$repo" commit -qam "agent insert X"
  # main changes l5 (trailing context of the insertion → patch-id differs)
  # then lands the identical insertion, so the re-pick applies with empty diff
  git -C "$repo" checkout -q main
  printf 'l1\nl2\nl3\nl4\nl5-CHANGED-ON-MAIN\n' > "$repo/f.txt"
  git -C "$repo" commit -qam "main change l5"
  printf 'l1\nl2\nl3\nINSERTED-X\nl4\nl5-CHANGED-ON-MAIN\n' > "$repo/f.txt"
  git -C "$repo" commit -qam "main insert same X"
  git -C "$repo" checkout -q main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/applied')
  echo "$row" | awk '{exit $8 == "0" ? 0 : 1}' \
    || fail "context-shifted clean pick should read UNPICKED 0: $row"
  echo "$row" | grep -q 'applied:1' \
    || fail "VERDICTS should be applied:1: $row"
  echo "$row" | grep -q 'main\*' \
    || fail "IN should carry the starred FROM entry: $row"
}

# The incident shape: agent commit conflicts on re-pick but its added content
# landed on main as a resolved pick -> resolved:1, Safe via probe. A competing
# change to the same region (different added content) stays unpicked.
test_resolved_and_negative_verdicts() {
  local repo=$TMP/repo-resolved rrow nrow detail
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  for i in $(seq 1 20); do echo "line-$i shared content block number $i here"; done > "$repo/doc.md"
  git -C "$repo" add doc.md
  git -C "$repo" commit -qm base

  # resolved pick: agent rewrites lines 10-11; main reworded neighbor line 9
  # first, then lands the agent additions (conflict resolved)
  git -C "$repo" branch claude/resolved main
  git -C "$repo" checkout -q claude/resolved
  { for i in $(seq 1 9); do echo "line-$i shared content block number $i here"; done
    echo "AGENT-ADDITION-10 with extra explanation text appended by agent"
    echo "AGENT-ADDITION-11 with extra explanation text appended by agent"
    for i in $(seq 12 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "agent change lines 10-11"
  git -C "$repo" checkout -q main
  { for i in $(seq 1 8); do echo "line-$i shared content block number $i here"; done
    echo "line-9 REWORDED-ON-MAIN independently before the pick"
    for i in $(seq 10 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "main rewords line 9"
  { for i in $(seq 1 8); do echo "line-$i shared content block number $i here"; done
    echo "line-9 REWORDED-ON-MAIN independently before the pick"
    echo "AGENT-ADDITION-10 with extra explanation text appended by agent"
    echo "AGENT-ADDITION-11 with extra explanation text appended by agent"
    for i in $(seq 12 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "main lands agent change (resolved pick)"
  git -C "$repo" checkout -q main

  # negative: competing change to line 10, never landed on main
  git -C "$repo" branch claude/negative main
  git -C "$repo" checkout -q claude/negative
  { for i in $(seq 1 9); do echo "line-$i shared content block number $i here"; done
    echo "AGENT-VERSION-10 with extra explanation text appended here now"
    for i in $(seq 11 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "agent competing change line 10"
  git -C "$repo" checkout -q main
  { for i in $(seq 1 9); do echo "line-$i shared content block number $i here"; done
    echo "MAIN-VERSION-10 with extra explanation text appended here now"
    for i in $(seq 11 20); do echo "line-$i shared content block number $i here"; done; } > "$repo/doc.md"
  git -C "$repo" commit -qam "main competing change line 10"
  git -C "$repo" checkout -q main

  rrow=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/resolved')
  echo "$rrow" | awk '{exit $8 == "0" ? 0 : 1}' \
    || fail "resolved pick should read UNPICKED 0: $rrow"
  echo "$rrow" | grep -q 'resolved:1' \
    || fail "VERDICTS should be resolved:1: $rrow"
  echo "$rrow" | grep -q 'main\*' \
    || fail "resolved pick IN should carry the starred FROM entry: $rrow"
  detail=$(cd "$repo" && bash "$SCRIPT" claude/resolved unpicked)
  echo "$detail" | grep -q '(applied on main, conflict resolved)' \
    || fail "resolved pick drill-down missing suffix: $detail"

  nrow=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/negative')
  echo "$nrow" | awk '{exit $8 == "1" ? 0 : 1}' \
    || fail "competing change should stay UNPICKED 1: $nrow"
  echo "$nrow" | awk '{exit $11 == "-" ? 0 : 1}' \
    || fail "competing change IN must be empty (unsafe): $nrow"
}

# The incident shape (2026-08-11): commits 0..n reach main via a true merge,
# the rest via a cherry-pick whose conflict was resolved manually — the
# resolution drifts the patch-id, so git cherry flags it forever. The twin
# sweep must place the drifted pick on main via its same-subject twin:
# UNPICKED 0, picked-resolved:1, a starred main entry, and the drill-down
# names the twin as not auto-reconcilable.
test_merge_plus_resolved_pick_combination() {
  local repo=$TMP/repo-incident row detail
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf 'doc base line\n' > "$repo/doc.md"
  git -C "$repo" add doc.md
  git -C "$repo" commit -qm base
  git -C "$repo" branch feature/base main

  git -C "$repo" branch claude/incident feature/base
  git -C "$repo" checkout -q claude/incident
  printf 'part A work\n' > "$repo/work-a"
  git -C "$repo" add work-a
  git -C "$repo" commit -qm "incident work part A"
  { cat "$repo/doc.md"
    echo "SECTION: incident feature description"
    echo "line one of the incident feature text"
    echo "line two of the incident feature text"
    echo "version: 1.10"; } > "$repo/doc.tmp" && mv "$repo/doc.tmp" "$repo/doc.md"
  git -C "$repo" commit -qam "incident tweak doc"

  # merge the prefix (part A) into main via an integration branch
  git -C "$repo" branch feature/finish claude/incident^
  git -C "$repo" checkout -q main
  git -C "$repo" merge -q --no-ff --no-edit feature/finish
  # resolved pick of the doc commit: same subject, drifted content (the
  # manual resolution renumbered the version line)
  { cat "$repo/doc.md"
    echo "SECTION: incident feature description"
    echo "line one of the incident feature text"
    echo "line two of the incident feature text"
    echo "version: 1.11"; } > "$repo/doc.tmp" && mv "$repo/doc.tmp" "$repo/doc.md"
  git -C "$repo" commit -qam "incident tweak doc"
  git -C "$repo" checkout -q main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/incident')
  echo "$row" | awk '{exit $8 == "0" ? 0 : 1}' \
    || fail "resolved pick combination should read UNPICKED 0: $row"
  echo "$row" | grep -q 'picked-resolved:1' \
    || fail "VERDICTS should carry picked-resolved:1: $row"
  echo "$row" | grep -q 'main\*' \
    || fail "IN should carry the starred main entry: $row"
  detail=$(cd "$repo" && bash "$SCRIPT" claude/incident unpicked)
  echo "$detail" | grep -q 'picked on main as .*conflict resolved manually — not auto-reconcilable' \
    || fail "drill-down missing the picked-resolved twin annotation: $detail"
}

# Same combination with a verbatim cherry-pick instead of a resolved one:
# patch-ids match, no probing needed — UNPICKED 0 and plain main containment.
test_merge_plus_clean_pick_combination() {
  local repo=$TMP/repo-cleanpick row sha
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf 'base\n' > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base
  git -C "$repo" branch feature/base main

  git -C "$repo" branch claude/cleanpick feature/base
  git -C "$repo" checkout -q claude/cleanpick
  printf 'a\n' > "$repo/work-a"
  git -C "$repo" add work-a
  git -C "$repo" commit -qm "clean work part A"
  printf 'b\n' > "$repo/work-b"
  git -C "$repo" add work-b
  git -C "$repo" commit -qm "clean work part B"
  sha=$(git -C "$repo" rev-parse HEAD)

  git -C "$repo" branch feature/finish claude/cleanpick^
  git -C "$repo" checkout -q main
  git -C "$repo" merge -q --no-ff --no-edit feature/finish
  git -C "$repo" cherry-pick "$sha" >/dev/null
  git -C "$repo" checkout -q main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/cleanpick')
  echo "$row" | awk '{exit $8 == "0" ? 0 : 1}' \
    || fail "merge + clean pick should read UNPICKED 0: $row"
  echo "$row" | grep -q 'main' \
    || fail "IN should contain main: $row"
}

# The twin sweep's declared blind spot: a transfer whose subject was reworded
# (and whose content drifted) has no subject twin — the commit stays UNPICKED
# with the exhausted-flavor annotation instead of a silent plain listing.
test_reworded_twin_stays_unpicked() {
  local repo=$TMP/repo-reworded row detail
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf 'doc base line\n' > "$repo/doc.md"
  git -C "$repo" add doc.md
  git -C "$repo" commit -qm base

  git -C "$repo" branch claude/reworded main
  git -C "$repo" checkout -q claude/reworded
  { cat "$repo/doc.md"
    echo "reworded feature block line one"
    echo "reworded feature block version 1.10"; } > "$repo/doc.tmp" && mv "$repo/doc.tmp" "$repo/doc.md"
  git -C "$repo" commit -qam "reworded source subject"
  git -C "$repo" checkout -q main
  { cat "$repo/doc.md"
    echo "reworded feature block line one"
    echo "reworded feature block version 1.11"; } > "$repo/doc.tmp" && mv "$repo/doc.tmp" "$repo/doc.md"
  git -C "$repo" commit -qam "landed under a completely different subject"
  git -C "$repo" checkout -q main

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/reworded')
  echo "$row" | awk '{exit $8 == "1" ? 0 : 1}' \
    || fail "reworded transfer must stay UNPICKED 1 (twin search exhausted): $row"
  detail=$(cd "$repo" && bash "$SCRIPT" claude/reworded unpicked)
  echo "$detail" | grep -q 'no twin on any local branch' \
    || fail "drill-down missing the twin-search-exhausted annotation: $detail"
}

# A fully harvested branch (empty FROM '+' set) runs no probes: WORKTREE_STATUS_TRACE=1
# emits no trace line for it, and the trace flag never changes stdout.
test_steady_state_runs_no_probes() {
  local repo=$TMP/repo-steady plain traced trace_err
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/f"
  git -C "$repo" add f
  git -C "$repo" commit -qm base
  # exact pick: agent commit is cherry-picked verbatim onto main (patch-present)
  git -C "$repo" branch claude/harvested main
  git -C "$repo" checkout -q claude/harvested
  printf '%s\n' work > "$repo/g"
  git -C "$repo" add g
  git -C "$repo" commit -qm "agent work"
  local sha
  sha=$(git -C "$repo" rev-parse HEAD)
  git -C "$repo" checkout -q main
  git -C "$repo" cherry-pick -x "$sha" >/dev/null
  git -C "$repo" checkout -q main

  plain=$(cd "$repo" && bash "$SCRIPT" 2>/dev/null)
  traced=$(cd "$repo" && WORKTREE_STATUS_TRACE=1 bash "$SCRIPT" 2>/dev/null)
  [ "$plain" = "$traced" ] \
    || fail "trace flag changed stdout"
  trace_err=$(cd "$repo" && WORKTREE_STATUS_TRACE=1 bash "$SCRIPT" 2>&1 >/dev/null | grep 'claude/harvested' || true)
  [ -z "$trace_err" ] \
    || fail "harvested branch should run no probes (no trace line): $trace_err"
}

# The verdict cache: a warm run is byte-identical and probe-free, the bypass
# env recomputes, a truncated file is a miss, candidate movement invalidates,
# and rows of deleted branches are pruned.
test_verdict_cache_hit_and_invalidation() {
  local repo=$TMP/repo-cache cache_dir cache_file first second traced
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/shared"
  git -C "$repo" add shared
  git -C "$repo" commit -qm base

  # conflicting agent commit so a cold scan provably probes (trace line)
  git -C "$repo" branch claude/work main
  git -C "$repo" checkout -q claude/work
  printf '%s\n' agent-version > "$repo/shared"
  git -C "$repo" commit -qam conflict-commit
  git -C "$repo" checkout -q main
  printf '%s\n' main-version > "$repo/shared"
  git -C "$repo" commit -qam main-diverges

  cache_dir=$repo/.git/agent-status-cache
  cache_file=$cache_dir/row-claude~work

  first=$(cd "$repo" && bash "$SCRIPT")
  [ -f "$cache_file" ] || fail "cache file not written: $cache_file"

  # warm run: byte-identical and probe-free
  traced=$(cd "$repo" && WORKTREE_STATUS_TRACE=1 bash "$SCRIPT" 2>&1 >/dev/null | grep 'trace:' || true)
  [ -z "$traced" ] || fail "warm run still probed: $traced"
  second=$(cd "$repo" && bash "$SCRIPT")
  [ "$first" = "$second" ] || fail "warm run differs from cold run"

  # bypass recomputes despite a valid cache file, with identical output
  traced=$(cd "$repo" && WORKTREE_STATUS_NO_CACHE=1 WORKTREE_STATUS_TRACE=1 bash "$SCRIPT" 2>&1 >/dev/null | grep 'trace:' || true)
  [ -n "$traced" ] || fail "WORKTREE_STATUS_NO_CACHE=1 did not recompute"
  second=$(cd "$repo" && WORKTREE_STATUS_NO_CACHE=1 bash "$SCRIPT")
  [ "$first" = "$second" ] || fail "bypass output differs from cached output"

  # truncated cache file is a miss: recomputed and rewritten complete
  printf 'garbage\n' > "$cache_file"
  second=$(cd "$repo" && bash "$SCRIPT")
  [ "$first" = "$second" ] || fail "truncated cache changed the output"
  [ "$(wc -l < "$cache_file")" -eq 5 ] || fail "cache file not rewritten complete"

  # candidate movement (main advances) invalidates: probes run again
  printf '%s\n' more > "$repo/more"
  git -C "$repo" add more
  git -C "$repo" commit -qm main-moves
  traced=$(cd "$repo" && WORKTREE_STATUS_TRACE=1 bash "$SCRIPT" 2>&1 >/dev/null | grep 'trace:' || true)
  [ -n "$traced" ] || fail "candidate movement did not invalidate the cache"

  # a deleted branch's cache row is pruned on the next run
  git -C "$repo" branch claude/tmp main
  (cd "$repo" && bash "$SCRIPT" >/dev/null)
  [ -f "$cache_dir/row-claude~tmp" ] || fail "no cache row written for claude/tmp"
  git -C "$repo" branch -q -D claude/tmp
  (cd "$repo" && bash "$SCRIPT" >/dev/null)
  if [ -e "$cache_dir/row-claude~tmp" ]; then
    fail "stale cache row for the deleted branch was not pruned"
  fi
}

# A clean checked-out worktree must count DIRTY=0 UNTRACKED=0 — pins the
# empty-porcelain edge of the single captured status run.
test_clean_worktree_counts_zero() {
  local repo=$TMP/repo3 wt row
  git -C "$repo" branch claude/clean main
  wt=$repo/.claude/worktrees/clean
  git -C "$repo" worktree add -q "$wt" claude/clean

  row=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/clean')
  echo "$row" | awk '{exit ($4 == "0" && $5 == "0") ? 0 : 1}' \
    || fail "clean worktree must show DIRTY=0 UNTRACKED=0: $row"
}

# claude-routines/* lineages are agent branches: they get their own row, and
# they are excluded from the containment candidates — work living only on a
# routine branch counts as contained nowhere.
test_routine_namespace_rows_and_candidate_exclusion() {
  local repo=$TMP/repo-routines rrow crow
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  printf '%s\n' base > "$repo/base"
  git -C "$repo" add base
  git -C "$repo" commit -qm base

  # routine lineage branch with its own commit
  git -C "$repo" branch claude-routines/nightly-2026-08-12 main
  git -C "$repo" checkout -q claude-routines/nightly-2026-08-12
  printf '%s\n' routine > "$repo/routine-out"
  git -C "$repo" add routine-out
  git -C "$repo" commit -qm "routine output"
  git -C "$repo" checkout -q main

  # claude branch whose work was merged ONLY into the routine branch
  git -C "$repo" branch claude/only-in-routine main
  git -C "$repo" checkout -q claude/only-in-routine
  printf '%s\n' work > "$repo/claude-work"
  git -C "$repo" add claude-work
  git -C "$repo" commit -qm "claude work"
  git -C "$repo" checkout -q claude-routines/nightly-2026-08-12
  git -C "$repo" merge -q --no-ff --no-edit claude/only-in-routine
  git -C "$repo" checkout -q main

  rrow=$(cd "$repo" && bash "$SCRIPT" | grep 'claude-routines/nightly-2026-08-12')
  [ -n "$rrow" ] || fail "routine branch missing from the overview"
  echo "$rrow" | awk '{exit $3 == "main" ? 0 : 1}' \
    || fail "routine row FROM should be main: $rrow"

  crow=$(cd "$repo" && bash "$SCRIPT" | grep 'claude/only-in-routine')
  echo "$crow" | awk '{exit $10 == "-" ? 0 : 1}' \
    || fail "MERGED must ignore routine branches (not a candidate): $crow"
  echo "$crow" | awk '{exit $11 == "-" ? 0 : 1}' \
    || fail "IN must ignore routine branches (not a candidate): $crow"
}

test_routine_namespace_rows_and_candidate_exclusion
test_from_resolves_remote_tracking_origin
test_from_without_recorded_origin_is_unknown
test_unpicked_list_has_no_probe_noise
test_applied_verdict_upgrades_row
test_resolved_and_negative_verdicts
test_merge_plus_resolved_pick_combination
test_merge_plus_clean_pick_combination
test_reworded_twin_stays_unpicked
test_steady_state_runs_no_probes
test_untracked_column_flags_non_infra_files
test_merged_and_contained_in
test_contained_in_lists_from_first
test_parallel_output_matches_serial
test_branch_name_selector
test_verdict_cache_hit_and_invalidation
test_clean_worktree_counts_zero

echo "PASS: print_agent_worktrees_status.sh"


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


## file: internal/repos/status_test.go

// [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit
// - [ACDSL-GOLANG-FMT-001] Every Go file is gofmt-formatted — run gofmt -w before committing; this gate replaces the raw Makefile gofmt line

package repos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusFixture mirrors the script's 14-column printf contract, captured from
// a real run (D10 — hand-written rows that contradict the script's arithmetic
// are banned): a probe-upgraded row (VERDICTS resolved:1, starred FROM entry
// in IN, UNPICKED 0), the "unknown" FROM row with all "-"/"–" sentinels, an
// untracked-blocked row, a worktree-less row contained via resolved picks
// (twin sweep), and a worktree-less row with genuinely unpicked work.
const statusFixture = `#    BRANCH                               FROM             DIRTY  UNTRACKED AHEAD  BEHIND  UNPICKED  VERDICTS             MERGED           IN                       LAST-COMMIT      WORKTREE
1    claude/feature-a                     origin/main      0      0         1      2       0         resolved:1           -                origin/main*             2026-07-10 12:30 /tmp/repo/.claude/worktrees/feature-a
2    claude/feature-b                     unknown          -      -         -      -       0         -                    -                -                        2026-07-09 08:15 –
3    claude/feature-c                     main             0      3         0      0       0         -                    -                main                     2026-07-08 09:00 /tmp/repo/.claude/worktrees/feature-c
4    claude/feature-d                     main             -      -         2      0       0         picked-resolved:2    -                main*                    2026-07-11 10:00 –
5    claude/feature-e                     main             -      -         1      0       1         -                    -                -                        2026-07-11 10:05 –
`

func TestParseStatusFixture(t *testing.T) {
	statuses, err := parseStatus(statusFixture)
	require.NoError(t, err)
	require.Len(t, statuses, 5)

	// Probe-upgraded row: the conflicted pick resolved on origin/main, so
	// VERDICTS carries resolved:1, IN keeps the starred FROM entry, and the
	// branch is safe via the applied-probe.
	first := statuses[0]
	assert.Equal(t, "claude/feature-a", first.Branch)
	assert.Equal(t, "origin/main", first.From)
	assert.Equal(t, 0, first.Dirty)
	assert.Equal(t, 0, first.Untracked)
	assert.Equal(t, 1, first.Ahead)
	assert.Equal(t, 2, first.Behind)
	assert.Equal(t, 0, first.Unpicked)
	assert.Equal(t, "resolved:1", first.Verdicts)
	assert.Empty(t, first.MergedInto)
	assert.Equal(t, []string{"origin/main*"}, first.In)
	assert.Equal(t, "2026-07-10 12:30", first.LastCommit)
	assert.Equal(t, "/tmp/repo/.claude/worktrees/feature-a", first.Worktree)
	assert.True(t, first.SafeToRemove)
	assert.True(t, first.SafeViaProbe)

	second := statuses[1]
	assert.Equal(t, "unknown", second.From)
	assert.Equal(t, noWorktreeDirty, second.Dirty)
	assert.Equal(t, noWorktreeDirty, second.Untracked)
	assert.Equal(t, unknownCount, second.Ahead)
	assert.Equal(t, unknownCount, second.Behind)
	assert.Empty(t, second.Verdicts)
	assert.Empty(t, second.MergedInto)
	assert.Empty(t, second.In)
	assert.Empty(t, second.Worktree)
	assert.False(t, second.SafeToRemove)
	assert.False(t, second.SafeViaProbe)
	assert.Equal(t, []string{"work contained nowhere", "no worktree"}, second.UnsafeReasons)

	// Untracked files alone must break the safe contract — the remove script
	// blocks on them even when the work is contained elsewhere. Plain "main"
	// IN entry (no star) means exact containment, not a probe upgrade.
	third := statuses[2]
	assert.Equal(t, 0, third.Dirty)
	assert.Equal(t, 3, third.Untracked)
	assert.Equal(t, []string{"main"}, third.In)
	assert.False(t, third.SafeToRemove)
	assert.False(t, third.SafeViaProbe)
	assert.Equal(t, []string{"untracked(3)"}, third.UnsafeReasons)

	// No worktree left, work transferred as manually resolved picks: safety
	// is containment alone — safe, with the resolved-pick count surfaced so
	// the UI can flag that reconciliation is no longer automatic.
	fourth := statuses[3]
	assert.Equal(t, noWorktreeDirty, fourth.Dirty)
	assert.Equal(t, "picked-resolved:2", fourth.Verdicts)
	assert.Equal(t, 2, fourth.ResolvedPicks)
	assert.Equal(t, []string{"main*"}, fourth.In)
	assert.True(t, fourth.SafeToRemove)
	assert.True(t, fourth.SafeViaProbe)
	assert.Empty(t, fourth.UnsafeReasons)

	// No worktree but genuinely unpicked work: unsafe, and the reasons name
	// the unpicked count instead of a generic ✗.
	fifth := statuses[4]
	assert.Equal(t, noWorktreeDirty, fifth.Dirty)
	assert.Equal(t, 1, fifth.Unpicked)
	assert.False(t, fifth.SafeToRemove)
	assert.Equal(t, []string{"unpicked(1)", "no worktree"}, fifth.UnsafeReasons)
}

func TestParseStatusNoBranches(t *testing.T) {
	statuses, err := parseStatus("No agent branches (claude/*, claude-routines/*) found.\n")
	require.NoError(t, err)
	assert.Nil(t, statuses)
}

func TestParseStatusShortRowFails(t *testing.T) {
	// Too few fields, and a legacy 13-column row (pre-VERDICTS) — both fail.
	for _, row := range []string{
		"1 claude/x main 0 1\n",
		"1 claude/x main 0 0 0 0 0 develop main 2026-07-10 12:30 /tmp/wt\n",
	} {
		_, err := parseStatus(row)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Unparseable row")
	}
}

func TestParseStatusNonNumericCountFails(t *testing.T) {
	row := "1    claude/x    main    0    0    x    0    0    -    develop    main    2026-07-10 12:30 /tmp/wt\n"
	_, err := parseStatus(row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AHEAD")
}

func TestParseCommits(t *testing.T) {
	assert.Nil(t, parseCommits("Commits ahead:\n"))
	assert.Nil(t, parseCommits("Commits ahead:\n(none)\n"))

	// Verdict suffixes from the detail-mode probe pass through verbatim in Line.
	commits := parseCommits("Commits unpicked for claude/x:\nabc1234 fix the thing  (applied on origin/main, conflict resolved)\ndef5678 another\n")
	require.Len(t, commits, 2)
	assert.Equal(t, "abc1234", commits[0].Sha)
	assert.Equal(t, "abc1234 fix the thing  (applied on origin/main, conflict resolved)", commits[0].Line)

	commits = parseCommits("Commits unpicked for claude/x:\nabc1234 genuinely unpicked\n")
	require.Len(t, commits, 1)
	assert.Equal(t, "abc1234 genuinely unpicked", commits[0].Line)
}

// writeStatusStub creates a stub status script: no args → the overview
// fixture, known branch → a commit list echoing its argv, unknown branch →
// the script's error contract (exit 1).
func writeStatusStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
if [ $# -eq 0 ]; then
	printf '%s' "` + statusFixture + `"
elif [ "$1" = "claude/missing" ]; then
	echo "error: unknown branch $1"
	exit 1
else
	echo "Commits $2 for $1:"
	echo "abc1234 stub commit"
fi
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, statusScript), []byte(script), 0o755))
	return dir
}

func TestCommitsRunsDetailModeByBranch(t *testing.T) {
	scriptsDir := writeStatusStub(t)
	commits, err := Commits(context.Background(), "claude/feature-b", "ahead", t.TempDir(), scriptsDir)
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, "abc1234", commits[0].Sha)
}

func TestCommitsUnknownBranchFails(t *testing.T) {
	scriptsDir := writeStatusStub(t)
	_, err := Commits(context.Background(), "claude/missing", "ahead", t.TempDir(), scriptsDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown branch")
}

func TestStatusRunsScriptInRepoPath(t *testing.T) {
	scriptsDir := writeStatusStub(t)
	statuses, err := Status(context.Background(), t.TempDir(), scriptsDir)
	require.NoError(t, err)
	assert.Len(t, statuses, 5)
}


## file: plans/applied-probe-safety/design/exploration.md

# Applied-Probe Safety for Agent-Worktree Branches — Exploration

## Context

- **Open question.** Two spaces are open: (1) *how* to decide a commit's change is genuinely present on FROM when `git cherry`'s patch-id says "unpicked" (the layered applied-probe), and (2) *where* the safety predicate lives so the Overview, the drill-down and `remove_agent_worktrees.sh` cannot disagree (the unified verdict source).
- **Driver.** `git cherry` matches by patch-id; a conflict-resolved cherry-pick rewrites context lines, changes the patch-id, and pins the branch UNSAFE forever even though every commit was harvested — the `claude/railroad-review-workflow-f76262` incident (8875782 landed as 9a2098e with a resolved `docs/workflows.md` conflict).
- **Second driver.** The safe predicate exists in three drifting copies (status script, `internal/repos/status.go`, `remove_agent_worktrees.sh`); the UI can read Safe while removal refuses.
- **Deletion gate.** The verdict feeds a *removal* decision. A false "applied" deletes unharvested work — the space must be evaluated fail-closed, never fail-open.
- **Mode.** `familiar` (default) — mechanisms named, git primitives assumed known.

This exploration was written without consulting the prior exploration under `plans/archived/applied_probe_safety/design/`.

## Constraints

| ID | Constraint | Source (anchor / measurement) |
|----|-----------|-------------------------------|
| C1 | `git cherry` decides membership by **patch-id**; conflict resolution changes context lines → new patch-id → false "unpicked". | Concept ¶9; incident 8875782→9a2098e |
| C2 | The verdict is a **deletion gate**. A false-positive "applied/safe" destroys unharvested commits; a false-negative only costs a `--force`. Asymmetric → fail-closed. | Concept Challenge "misclassifying … corrupts a deletion gate" |
| C3 | FROM may be a **remote-tracking ref** (`origin/main`), resolved from the branch reflog only, **never guessed**; "unknown" when unresolvable. FROM is not necessarily in the `git cherry` candidate set. | `print_agent_worktrees_status.sh:16-18`; `verdict.sh resolve_from` |
| C4 | **Output contract is load-bearing.** The printf row template (`print_agent_worktrees_status.sh:267`), the Go parser's `≥14`-field check (`status.go parseStatusRow:167`), and every fixture (Go `status_test.go`, shell `test_print_agent_worktrees_status.sh`) must move together. | Explore anchors; concept ¶48 |
| C5 | Drill-down probe lines must stay **parseable as commit rows** (`internal/repos parseCommits`; `test_print_agent_worktrees_status.sh:66`). Suffix vocabulary may change; the log-line shape may not. | Concept Challenge ¶69 |
| C6 | Scripts run under **macOS bash 3.2** (launchd/hooks): no `declare -A`, `mapfile`, case-expansion, parameter transformation. | `print_agent_worktrees_status.sh:3` (ACDSL-SHELL-002) |
| C7 | **macOS has no `flock(1)`.** Any concurrent-write coordination (cache) must be lock-free or bring its own lock. | Memory `macos-no-flock`; concept ¶108 |
| C8 | **Cost budget:** Overview must stay near plumbing-only speed. Probes may run **only** for rows whose FROM cherry-`+` set is non-empty; a fully-harvested branch pays nothing extra. Cost must be measurable before any cache. | Concept Goal 2 |
| C9 | A sourced-shell-lib precedent already exists (`routines/_lib/worktree.sh`) — a shell function library sourced by multiple CLIs. | `routines/_lib/worktree.sh` |

## Options

### Space A — the per-commit applied-probe

The probe answers one question for a commit `H` that is patch-id-unpicked against FROM: *is H's change actually present on FROM (or a candidate)?* Families are ordered strictest→loosest; the design layers the survivors and short-circuits.

- **A0 — patch-id only (`git cherry`).** Mechanism: exact patch-id membership. Binding: C1 kills it — this *is* the bug. Kept as **layer 1** (`picked`) because it is free and correct for clean transfers; it is only insufficient, not wrong.
- **A1 — clean re-pick empty-diff.** Mechanism: `cherry-pick --no-commit H` onto FROM in a temp worktree; empty staged diff ⇒ `applied`. Catches context-shifted-but-otherwise-clean picks. Binds C8 (temp worktree, gated on non-empty `+` set), C6 (plain bash). Ownership: none — this is the existing detail-mode probe factored out. Fails to speak when the re-pick *conflicts* (an already-applied change self-conflicts).
- **A2 — conflicted re-pick with `-X theirs`, empty-diff.** Mechanism: on conflict, re-pick with `-X theirs` and test for empty diff. **Killed by C2:** `-X theirs` silently discards genuinely-unapplied conflicting hunks, so a competing change reports "applied" — a false-safe on the deletion gate.
- **A3 — range-diff pairing, loose acceptance.** Mechanism: on conflict, `git range-diff <merge-base>..FROM H^..H`; any `=`/`!` pairing with a FROM commit ⇒ `applied-resolved`. **Killed (measured):** at default `--creation-factor` a *competing edit to the same region* pairs and false-passes (stop condition S7) — again a false-safe on a deletion gate.
- **A4 — range-diff pairing, added-line-identity acceptance.** Mechanism: keep range-diff as the *pairing finder*, but accept `applied-resolved` **only when the two paired commits add identical lines** (the interdiff carries no addition-payload difference). Removed/context-line differences are inherent to a legitimate resolved pick and do not block. Fail-closed guards (C2): no pairing → `unpicked`; a pure-deletion commit whose removed lines differ → `unpicked`; any binary hunk → `unpicked`. Ownership: none. Grounded: rides `range-diff` plumbing; measured separation across three ground-truth cases (real incident accept, competing-edit-resolved accept, competing-change reject).
- **A5 — full 3-way merge / `merge-tree` simulation.** Mechanism: simulate the merge and inspect the resulting tree. Heavier, more moving parts, and range-diff already yields a decisive addition-payload signal — subsumed by A4.
- **A6 — candidate-wide twin sweep.** Mechanism: for a commit still `unpicked` after A0–A4, search *every* candidate (FROM first) for an exact-subject twin (`git log --fixed-strings --grep`), pair by range-diff; a twin that landed with manual resolution ⇒ `picked-resolved` (counts as picked — the change is in *some* candidate). Catches transfers that landed off-FROM. Fail-closed: a reworded/squashed transfer has **no** subject twin → stays `unpicked` (invisible transfer, correctly refused). Binds C8 (only runs on the residual unpicked set), C6.
- **A7 — resulting-blob comparison.** Mechanism: compare the file's final blob on branch vs FROM. **Killed by C2:** other commits touch the same files, so blob equality neither implies nor excludes *this* commit's transfer — coarse and false in both directions.

**FROM→Safe upgrade (sub-decision within A).** FROM may sit outside the candidate set (C3), so a FROM-only verdict needs a channel into the Safe rule:
- **A-up1 — add remote refs to the candidate set.** Changes the per-candidate meaning of IN and the candidates rule for everything downstream. Rejected: broad blast radius for one need.
- **A-up2 — annotated FROM entry in IN.** When all FROM cherry-`+` commits end picked/applied/applied-resolved, emit an annotated FROM entry (`origin/main*`) so `len(In)>0` holds. The candidates rule and IN's per-candidate meaning stay untouched. Chosen.

**Output surface (sub-decision within A).** How the row exposes non-`picked` verdicts:
- **O1 — drill-down only.** Verdicts visible only on opening a row. Rejected: Safe changes silently at row level with no row-level evidence.
- **O2 — overload UNPICKED.** Fold applied/resolved counts into UNPICKED. Rejected: destroys UNPICKED's meaning (genuinely-untransferred count) — the number that gates Safe.
- **O3 — new VERDICTS column** directly after UNPICKED, whitespace-free comma summary (`applied:1,resolved:1`, `-` when none); UNPICKED becomes the `unpicked`-only count. Breaking change to template + parser + fixtures (C4), accepted for row-level verdict visibility. Chosen.

### Space B — the unified verdict source

One predicate, three consumers (status script, `status.go`, remove script).

- **B0 — keep three copies, add drift-detecting golden tests.** Mechanism: a shared fixture both scripts must reproduce. Does not remove the drift — only alarms after it happens; the predicate logic is still authored three times. Rejected.
- **B1 — sourced shell lib `cmd/worktrees/_lib/verdict.sh`.** Mechanism: cherry-set computation + probe escalation + containment live in one sourced file; the status script and `remove_agent_worktrees.sh` both `source` it; `status.go` keeps calling the status script and parsing its output. Grounded: rides the `routines/_lib/worktree.sh` precedent (C9); both CLIs stay bash-native (C6); deletes the remove script's own `contained_in()` two-pass and copy-pasted `count_untracked()`. Single source of truth by *deletion*, not by guard. Chosen.
- **B2 — Go as single source, shell shells out.** Mechanism: predicate moves into `internal/repos`, exposed as a subcommand; both scripts invoke the Go binary. Rejected: injects a compiled-binary build dependency into a bash-3.2 hook path, and forces both CLIs to change their shape — high blast radius for a predicate that is fundamentally git-plumbing orchestration.
- **B3 — codegen the three from one spec.** Rejected: introduces a generator (a whole new concept) to solve a three-consumer duplication — overkill.
- **B4 — single Go binary replaces the status script; scripts and `status.go` all call it.** Rejected: a rewrite of the entire status surface, out of scope; discards the bash-native, dependency-free hook property.

## Evaluation

Groundedness = how much existing code/pattern the option rides (the deciding criterion).

### Space A — probe layers

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|-------------|--------------|--------|---------------|---------|
| A0 patch-id | High (pure plumbing) | None | 0 | — | **Layer 1 (`picked`)** — necessary, insufficient (C1) |
| A1 clean re-pick empty-diff | High (existing detail probe) | Low (temp worktree) | ~1d | High | **Layer 2 (`applied`)** |
| A2 `-X theirs` empty-diff | Med | Low | ~0.5d | High | **Killed — C2** (discards unapplied hunks) |
| A3 range-diff loose | Med (range-diff) | Low | ~1d | High | **Killed — S7 measured false-pass** |
| A4 range-diff + added-identity | Med (range-diff) | Low | ~1.5d | High | **Layer 3 (`applied-resolved`)** |
| A5 merge-tree sim | Low | Med | High | High | Rejected — subsumed by A4 |
| A6 candidate-wide twin sweep | Med (log/range-diff) | Med | ~1d | High | **Layer 4 (`picked-resolved`)** |
| A7 blob compare | Low | Low | ~0.5d | High | **Killed — C2** (coarse, false both ways) |

Per-scenario note: the layers are complementary, not competing. A1 handles the clean-context case, A4 the same-file conflict-resolution case *on FROM*, A6 the transferred-off-FROM (subject-twin) case. A reworded/squashed transfer is caught by none and correctly stays `unpicked` (C2 fail-closed) — the one accepted residual false-negative.

### Space B — verdict source

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|-------------|--------------|--------|---------------|---------|
| B0 3 copies + golden tests | High | None | ~0.5d | High | Rejected — alarms drift, doesn't remove it |
| B1 sourced shell lib | High (`worktree.sh` precedent) | Med (both scripts) | ~1–2d | High | **Chosen** |
| B2 Go single source + shell-out | Low | High | ~2–3d | Med | Rejected — build dep in hook path |
| B3 codegen from spec | Low | High | High | Low | Rejected — new concept |
| B4 full Go rewrite | Low | High | High | Low | Rejected — out of scope |

### Sub-decisions

| Sub-decision | Chosen | Beaten alternatives |
|--------------|--------|---------------------|
| FROM→Safe channel | A-up2 annotated IN entry (`origin/main*`) | A-up1 add remotes to candidate set (broad blast radius) |
| Output surface | O3 new VERDICTS column | O1 drill-down only (silent Safe), O2 overload UNPICKED (destroys the gate count) |

## Recommendation

Adopt the **layered short-circuiting probe**, gated on a non-empty FROM cherry-`+` set (C8):

1. `picked` — `git cherry` patch-id match (A0, layer 1).
2. `applied` — clean `cherry-pick --no-commit` empty-diff on FROM (A1, layer 2).
3. `applied-resolved` — range-diff pairing with **added-line identity** on FROM (A4, layer 3), fail-closed on no-pairing / differing-deletion / binary.
4. `picked-resolved` — candidate-wide subject-twin sweep for transfers that landed off-FROM (A6, layer 4).
5. `unpicked` — none of the above; the only verdict that keeps the branch unsafe.

Package the predicate as a **sourced shell lib** `cmd/worktrees/_lib/verdict.sh` (B1), sourced by both the status script and `remove_agent_worktrees.sh`, deleting the remove script's duplicated `contained_in()`/`count_untracked()`; `status.go` keeps parsing the script's output. Feed FROM verdicts into Safe via an **annotated IN entry** (A-up2), and expose the per-row breakdown through a **new VERDICTS column** (O3).

Every recommendation here coincides with a locked concept decision — the exploration confirms these as constraint-forced, not arbitrary. The binding facts fdesign imports:

- **What to elaborate:** the four probe layers (A0/A1/A4/A6), the sourced-lib packaging (B1), the annotated-IN channel (A-up2), and the VERDICTS output contract (O3/C4).
- **Binding constraints:** C2 (fail-closed on the deletion gate — the added-line-identity rule and its guards are non-negotiable), C4 (template + parser + fixtures move together), C8 (probe only on non-empty `+` sets), C3 (FROM from reflog, may be remote-tracking), C6 (bash 3.2).
- **Measurements to carry forward:** the three-case ground-truth separation for A4 (incident accept / competing-edit-resolved accept / competing-change reject) is the acceptance test for layer 3; a trace hook (`WORKTREE_STATUS_TRACE=1`: per-row probe count + duration) measures C8 before any cache is designed.

**No genuinely-open finalist remains** — the concept drained its questions and each space has a single constraint-surviving winner. The one judgment call worth flagging to fdesign is A6's scope: the *re-pick* probe (A1/A4) is FROM-only per decision, while the *twin sweep* (A6) is candidate-wide. That is consistent — `picked-resolved` means the change is in *some* candidate, which the existing candidate set already legitimizes — but fdesign should confirm the twin sweep's cost stays inside C8 (it runs only on the post-layer-3 residual, so it should).

## Rejected

- **A0 alone (patch-id only)** — the reported bug: patch-id breaks on any conflict-resolved pick (C1).
- **A2 (`-X theirs` empty-diff)** — silently discards genuinely-unapplied conflicting hunks → false-safe on a deletion gate (C2).
- **A3 (loose range-diff pairing)** — measured false-pass on a competing edit to the same region at default creation-factor (S7).
- **A5 (merge-tree simulation)** — heavier, subsumed by A4's decisive addition-payload signal.
- **A7 (resulting-blob comparison)** — coarse; unrelated commits touch the same files, false in both directions (C2).
- **A-up1 (remotes into candidate set)** — changes IN's per-candidate meaning repo-wide for one need.
- **O1 (drill-down only)** — Safe changes silently with no row-level evidence.
- **O2 (overload UNPICKED)** — destroys the genuinely-untransferred count that gates Safe.
- **B0 (three copies + golden tests)** — detects drift after the fact; the predicate is still authored three times.
- **B2 (Go single source, shell shells out)** — injects a compiled-binary dependency into a bash-3.2 hook path and reshapes both CLIs.
- **B3 (codegen)** — a generator is a new concept to solve three-consumer duplication.
- **B4 (full Go rewrite of the status surface)** — out of scope; discards the dependency-free bash-native property.

## Open Questions / Assumptions (unattended session)

- **Assumption:** the concept's `[USER]`/AskUserQuestion decisions (2026-07-26) are binding inputs to this exploration, so options that contradict them are listed in the Rejected register rather than re-opened. Recorded because no user was present to reaffirm.
- **Assumption:** mode defaults to `familiar` (the brief named none).
- **Assumption:** the persistent probe cache stays **backlogged** per the `[USER]` no-cache MVP decision; this exploration surveys the *probe* and *source* spaces only, not caching. (This checkout already carries a cache implementation, which post-dates and supersedes the MVP scope — noted, not evaluated here.)
- **Observation, not a question:** the code in this worktree already implements the recommended design (sourced `verdict.sh`, VERDICTS column, layered probe, twin sweep). This exploration was produced as the constraint-first survey that *precedes* that lock, to record why the surviving options win and why the killed ones must not be re-explored.


## file: routines/_lib/worktree.sh

#!/usr/bin/env bash
# [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit
# - [ACDSL-SHELL-002] Scripts launched by hooks or launchd run under macOS bash 3.2 — no declare -A, mapfile/readarray, case expansion, |&, or parameter transformation (the launchd lesson)

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
  prompt="/merge-resolve theirs main. Unattended routine run: never ask a question; decide every judgement call yourself. Merge directly on the current branch — do not create a work branch and skip the cleanup step."
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
  # The merge must land on the dated branch itself: run.sh captures the run
  # branch from HEAD, and publish/prune glob on the group prefix — a HEAD left
  # on a stray merge/* work branch would mis-target the whole run.
  if [[ "$base" != "main" ]] && { ! routine_chain_sync_main "$wt" \
      || [[ "$(git -C "$wt" symbolic-ref --short HEAD 2>/dev/null)" != "$new" ]]; }; then
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


