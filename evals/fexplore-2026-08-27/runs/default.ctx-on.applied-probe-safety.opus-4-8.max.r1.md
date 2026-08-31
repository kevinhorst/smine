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

# Applied-Probe Safety — Exploration

## Context
- **Open question (two axes):** (A) how to decide a commit's change is *present* on FROM/a candidate when `git cherry` patch-id equality fails (conflict-resolved / rebased picks read `+` forever), and (B) where the single safe-verdict lives so the overview, the drill-down, and `remove_agent_worktrees.sh` can never disagree.
- **Driver:** the `claude/railroad-review-workflow-f76262` incident — commit `8875782` landed as `9a2098e` with a resolved `docs/workflows.md` conflict, so patch-id drifted and the branch pinned unsafe though every commit was harvested (concept.md:9).
- **Hard rule:** the deletion gate must fail *closed* — a false "safe" on genuinely-unpicked work loses commits; a false "unsafe" only costs a `--force`.
- **Current state (groundedness signal, not a reason to skip the survey):** the concept's MVP is already realized and has evolved past it — `cmd/worktrees/_lib/verdict.sh` implements the full ladder, both scripts source it, and a per-branch SHA-keyed verdict cache exists (the concept *backlogged* caching). This exploration surveys the mechanism space the shipped choice drew from, grounds each alternative, confirms the choice beat them, and surfaces the one substrate alternative the shipped code never weighed (`git merge-tree`).
- **Mode:** familiar (explanation depth only; rigor unchanged).

## Constraints

| ID | Constraint | Source (anchor / measurement) |
|----|------------|-------------------------------|
| C1 | Every shell in the verdict path runs under **macOS bash 3.2** (hooks/launchd): no `declare -A`, `mapfile`, case expansion, `\|&`. Memo state is parallel *indexed* arrays. | verdict.sh:3 header `[ACDSL-SHELL-002]`; verdict.sh:16, :249–259, :337–367 |
| C2 | Probe git output must **never reach stdout** — `Auto-merging`/`CONFLICT` lines parse as commit rows in the UI. | verdict.sh:194–195, :202; test_unpicked_list_has_no_probe_noise (test:69–107) |
| C3 | The overview is a fixed **14-token printf row**; the Go parser hard-errors on `<14` fields; VERDICTS sits directly after UNPICKED. Any mechanism that changes per-row output moves the printf, the parser, and every fixture together. | print script:267–270, :282–283; parseStatusRow status.go:167–176; status_test.go:22–28 |
| C4 | **Fail-closed** deletion gate: no pairing → `unpicked`; pure-deletion whose removed lines differ → `unpicked`; any binary hunk → `unpicked`; any added-content difference → `unpicked`. | verdict.sh:174–189, :219–224; negative case in test_resolved_and_negative_verdicts (test:334–338) |
| C5 | FROM may be a **remote-tracking ref** (`origin/main`), not a local candidate — probes and IN upgrades must handle a FROM outside `candidates[]`. | resolve_from verdict.sh:34–49; candidate_index verdict.sh:231–242; test:28–54 |
| C6 | `git cherry` patch-id is the **wrong equivalence** for "is this change anywhere" — conflict-resolved/rebased transfers stay `+`. Any solution built on patch-id alone over-counts transferred work as missing. | memory `patch-id-not-repo-containment`; concept.md:9; demonstrated by test:277–339 |
| C7 | **Cost floor:** a fully-harvested branch (empty FROM `+` set) pays *zero* probe cost — the steady state is probe-free without a cache. | evaluate_branch early return verdict.sh:407–417; test_steady_state_runs_no_probes (test:468–496) |
| C8 | **One verdict source** feeding overview + drill-down + remove; the UI must never say Safe while removal refuses. | concept Goal; status.sh:60 + remove.sh:38 both `source _lib/verdict.sh`; status.go consumes the printed IN (status.go:248–267) |
| C9 | macOS has **no `flock(1)`** — any cross-worker coordination (e.g. a cache) must be lock-free. | memory `macos-no-flock`; concept backlog:106–108 |

## Options

### Axis A — the equivalence mechanism (proving "present" despite patch-id drift)

The ladder is strictest-first and short-circuits; a commit only reaches a lower layer when every stricter layer failed. Families are the candidate mechanisms for the two non-trivial layers (clean-apply detection, and conflict disambiguation) plus the target-scope question.

- **A0 — patch-id only (`git cherry`, status quo).** Mechanism: compare patch-ids, `+` = absent. Killed by **C6**: the entire reason the concept exists. Retained only as layer 1 (cheap, correct when it says "present").
- **A1 — re-pick + range-diff pairing (the ladder).** Layer 2 `applied`: replay the commit onto FROM; a clean replay with an **empty** resulting diff means the change is already there (catches context-shifted picks patch-id misses). Layer 3 `applied-resolved`: a *conflicting* replay is ambiguous (an already-applied change self-conflicts, but so does a competing edit), so pair the commit against FROM's history with `git range-diff` and accept only when the two paired commits **add identical lines** — the interdiff has no added-content difference (option-C acceptance). Binds: C2 (silence output), C4 (the added-lines guard is the fail-closed core), C6 (this is the fix). Ownership: none external — rides the existing detail-mode probe, factored into the shared lib.
  - **A1-substrate-a — temp detached worktree** (`cherry-pick --no-commit` + `diff --cached --quiet` + `reset --hard`, lazy per-process worktree). *Shipped* (verdict.sh:136–229). Binds C1 (indexed globals for the memo), C2. Cost: a `mktemp` worktree lifecycle + a cleanup trap + a re-detach path when FROM varies across a multi-branch caller.
  - **A1-substrate-b — worktreeless `git merge-tree --write-tree --merge-base=<hash>^ <FROM> <hash>`.** Same three-way merge cherry-pick runs, but entirely in the object DB: exit 0 + result-tree OID equal to `FROM^{tree}` ⇒ `applied`; equal-but-different tree ⇒ clean-apply-with-content ⇒ `unpicked`; exit 1 ⇒ conflict ⇒ hand to the *same* range-diff layer 3. Removes the entire worktree lifecycle (`ensure_probe_worktree`, `verdict_cleanup`, the re-detach branch, the pool-interaction risk). Binds C2 (still silence stderr), needs git ≥2.38 (the repo already relies on `range-diff`, ubiquitous in the same era). Not weighed by the shipped code.
- **A2 — target scope: candidate-wide twin sweep vs FROM-only.** When layer 2/3 on FROM still says `unpicked`, sweep *every* candidate (FROM first) for a commit with the **identical subject** since the merge-base and range-diff-pair it: a hit is `picked-resolved` (landed on some branch via a manual resolution — counted present, but flagged not auto-reconcilable); subject-exhausted is `unpicked-notwin` (reworded/squashed transfers are invisible — say so instead of a silent listing). *Shipped* (twin_sweep verdict.sh:281–332). FROM-only was the concept's original [USER] decision (concept.md:122); it leaves a blind spot — a resolved pick that landed on a candidate ≠ FROM reads `unpicked` forever — which the memory flagged and the code closed. Binds C5, C6; ownership none.
- **A3 — `-X theirs` re-pick + empty-diff.** Mechanism: re-pick forcing "theirs" on conflict, then test for empty diff. Killed by **C4**: it silently discards genuinely-unapplied conflicting hunks, yielding false `applied` on a deletion gate. Named-and-rejected in the concept (concept.md:124).
- **A4 — blob/tree containment.** Does FROM's tree contain the commit's post-image blob? Killed: a line-level edit is not a whole-blob identity, and any co-located change on FROM makes the blob differ though the agent's lines are present — false `unpicked`, and worse, no way to distinguish from a competing edit.
- **A5 — reverse-apply (`git diff hash^ hash | git apply -R --check`).** "The patch reverses cleanly against FROM ⇒ already present." A weaker, hand-rolled merge-tree: brittle on context drift (the exact case C6 is about — a moved neighbor line fails the reverse-apply) and awkward across multiple files/renames. Strictly dominated by A1-substrate-b.
- **A6 — `git log --cherry-mark` / `--cherry`.** Same patch-id equivalence underneath ⇒ inherits **C6**. No gain over A0.
- **A7 — homegrown normalized patch-id (strip context, re-hash).** Reinvents range-diff's pairing worse and with no fail-closed added-lines discipline; a new equivalence relation to test and defend. Rejected against A1.

### Axis B — the unified verdict source

- **B1 — sourced shell lib (`cmd/worktrees/_lib/verdict.sh`).** One file computing candidate discovery, cherry sets, the ladder, the twin sweep, and the IN/UNPICKED/VERDICTS results; both CLIs `source` it; Go parses the printed row. *Shipped.* Follows the `routines/_lib/worktree.sh` precedent, keeps both CLIs runnable standalone, and *deletes* the remove script's copy-pasted `contained_in`/`count_untracked`. Binds C1, C3, C8. Ownership: none external.
- **B2 — Go as the single source.** Rewrite the probe in Go; the overview reads it directly, a Go-backed remove path asserts it server-side, the shell scripts retire or become thin shims. Groundedness low: reimplements working, tested shell in a second language; large blast radius (new Go probe, worktree orchestration in Go, C1 no longer relevant but every test rewrites). Reversibility poor. Offered as the "if we ever kill the CLIs" endgame, not now.
- **B3 — shared Go CLI subcommand called by the shell** (`smine worktree-verdict <branch>`), inverting today's "Go parses shell." Removes the printf/parse contract (C3) but adds a Go build dependency to two shell scripts that today run standalone in a bare checkout, and splits the logic across a language boundary for no correctness gain.
- **B4 — status script is the sole producer; remove + Go both parse its stdout.** No lib: the remove gate runs the overview and greps the IN column. Couples removal to the overview's output contract *and* its row-numbering, and re-runs the whole table to gate one branch. Weaker single-source than B1 (the producer is a CLI surface, not a function).
- **B5 — keep three copies, lock them with a shared test corpus.** Detects drift; does not remove it. Rejected on the concept's "delete the duplicate, not guard around it" (concept.md:92) — leaves three mechanisms alive.

## Evaluation

| Option | Groundedness | Blast radius | Effort | Reversibility | Verdict |
|--------|-------------|-------------|--------|--------------|---------|
| A1 ladder (applied + applied-resolved) | High — the existing detail probe, factored + shared; proven test:241–339 | Contained to verdict.sh + the 14-col contract (C3) | Med | High | **Adopt** — the only family satisfying C4+C6 |
| A1-substrate-a worktree | High — shipped | Local to `probe_verdict` | — | High | Adopt (baseline) |
| A1-substrate-b merge-tree | Med — standard git, not yet in-repo | Local to `probe_verdict` only | Low | High | **OPEN** — adopt pending a live measurement (drops the worktree lifecycle) |
| A2 candidate-wide twin sweep | High — shipped + memory-endorsed | verdict.sh + drill-down suffixes | Med | High | **Adopt** — closes the FROM-only blind spot |
| A2 FROM-only (concept original) | Med | Smaller | Low | High | Superseded — blind to resolved picks off-FROM |
| A3 `-X theirs` empty-diff | Low | — | Low | High | **Reject (C4)** — false-safe |
| A4 blob/tree containment | Low | — | Low | High | Reject — co-located edits false-negative |
| A5 reverse-apply | Low | — | Low | High | Reject — dominated by A1-b |
| B1 sourced shell lib | High — shipped; `worktree.sh` precedent | Both scripts + status.go parser | Med | High | **Adopt** |
| B2 Go single source | Low | Repo-wide | High | Low | Reject now — endgame only |
| B3 Go CLI subcommand | Low | Two scripts gain a build dep | High | Med | Reject — language split, no gain |
| B4 script-as-producer | Med | remove coupled to overview format | Med | Med | Reject — weaker single-source than B1 |

**Per-scenario notes (fail-closed is the discriminator):**
- *Context-shifted clean pick* (test:241–272): patch-id differs, but the re-pick's diff is empty ⇒ **applied**. merge-tree yields the identical result (merged tree = FROM tree). Both substrates correct.
- *Conflict-resolved pick — the incident* (test:277–332): re-pick conflicts; range-diff pairs the FROM commit and the added lines match ⇒ **applied-resolved**, Safe via probe. This is the case A0/A4/A5/A6 all get wrong.
- *Competing change never landed* (test:334–338): re-pick conflicts *and* the paired added content differs ⇒ **unpicked**, IN empty, unsafe. A3 would call this `applied` (the false-safe C4 forbids).
- *Resolved pick that landed off-FROM, on a candidate* (test:347–394): FROM probe says unpicked; the twin sweep finds the same-subject twin on `main` ⇒ **picked-resolved**. FROM-only (A2 alt) misses this entirely — the axis that forced candidate-wide.
- *Reworded/squashed transfer* (test:435–464): no subject twin anywhere ⇒ **unpicked-notwin**, surfaced as a named blind spot, not a silent "lost."

## Recommendation

**Axis A:** the layered ladder (A1) with candidate-wide twin sweep (A2), which is what the code realizes. What fdesign imports as binding:
- Ladder, strict→loose, short-circuiting: `picked` (patch-id) → `applied` (empty re-pick on FROM) → `applied-resolved` (conflicted re-pick, range-diff-paired, **added lines identical**) → twin sweep across candidates → `picked-resolved` (subject twin paired) | `unpicked-notwin` (no twin) → `unpicked`.
- The **fail-closed guards are the contract, not polish** (C4): no pairing, differing removed lines on a pure deletion, any binary hunk, or any added-content difference ⇒ `unpicked`.
- **Scope is candidate-wide, FROM first** (supersedes the concept's FROM-only decision — measured blind spot, memory-confirmed).
- **Cost:** probe only rows with a non-empty FROM `+` set (C7); the empty-`+` fast path pays nothing.
- **OPEN (substrate):** adopt `git merge-tree --write-tree` (A1-substrate-b) in place of the temp-worktree probe, *pending one live measurement* that clean-apply/empty-tree and conflict detection match the worktree probe across the three ground-truth cases. Reversibility is high (the swap is local to `probe_verdict`), so this is a safe post-ship optimization, not a launch blocker. It removes the worktree lifecycle, the cleanup trap, and the pool-interaction surface — the largest simplification available on this axis.

**Axis B:** the sourced shell lib (B1) — shipped, single-source, standalone-CLI-preserving. Go stays a pure output *consumer* (`parseStatusRow` derives `SafeToRemove` from the printed IN/DIRTY/UNTRACKED; it does not re-run the verdict — the concept's "third copy" is gone). fdesign imports: verdict.sh owns candidate discovery + the ladder + IN/UNPICKED/VERDICTS; both scripts source it; the 14-token contract (C3) is the Go seam.

**Residual, sub-noise (flag, don't act):** the *safe predicate* itself — "IN non-empty AND (no worktree OR clean)" — is still expressed three times: the script header doc (status.sh:48–50), the remove gate's `reasons` (remove.sh:98–101), and `SafeToRemove` (status.go:262–267). Each consumer genuinely needs it in its own shape (a gate, a bool, a doc), and it is a trivial derivation over the single-sourced IN column — unifying it further buys nothing and would couple the Go UI to a shell exit code. Left as-is.

## Rejected
- **A0 patch-id only** — the bug itself (C6); kept only as the cheap layer-1 positive.
- **A3 `-X theirs` empty-diff** — discards unapplied conflicting hunks ⇒ false-safe on a deletion gate (C4).
- **A4 blob/tree containment** — a co-located edit on FROM makes the blob differ though the agent's lines are present; cannot separate present-with-resolution from competing.
- **A5 reverse-apply `apply -R --check`** — brittle exactly on context drift (the C6 case); strictly dominated by merge-tree.
- **A6 `log --cherry-mark`** — patch-id underneath; inherits C6.
- **A7 normalized homegrown patch-id** — reinvents range-diff worse, no fail-closed discipline.
- **A2 FROM-only scope** — superseded: blind to a resolved pick landing on a candidate ≠ FROM.
- **B2 Go single source / B3 Go CLI subcommand** — reimplement working tested shell across a language boundary; large blast radius, poor reversibility, no correctness gain; endgame only if the CLIs are retired.
- **B4 script-as-sole-producer** — couples removal to the overview's output format and row-numbering; weaker single-source than a function.
- **B5 three copies + test corpus** — detects drift instead of deleting it; leaves parallel mechanisms alive.

## Open Questions
*(unattended session — assumptions recorded, not asked)*
- **merge-tree substrate unmeasured here.** The A1-substrate-b viability rests on git's documented three-way `merge-tree --write-tree` semantics, not a live run — the probe script (`/tmp/probe_applied.sh`) was written but Bash execution is approval-gated with no operator present. Before fdesign commits the substrate swap, run it: assert `applied`/`unpicked`/`CONFLICT→range-diff` match the worktree probe on the three ground-truth cases, and confirm the minimum git version on the launchd/hook path clears 2.38.
- **Scope reconciliation vs the concept.** I treat the shipped **candidate-wide** twin sweep as authoritative over the concept's [USER] "FROM only" decision, because the code + the `patch-id-not-repo-containment` memory both post-date and supersede it. If FROM-only was in fact a still-binding cost decision, the twin sweep's per-candidate `git log --grep` cost needs its own measurement against C7.
- **Verdict cache is beyond this concept's MVP.** The repo ships a per-branch SHA-keyed cache (`<git-common-dir>/agent-status-cache/`, status.sh:52–56, test:501–560) that the concept explicitly *backlogged*. It is out of scope for this exploration except as evidence C7's steady-state-probe-free assumption held well enough to justify caching; its lock-free single-line-record design (C9) matches the concept's backlog sketch.


