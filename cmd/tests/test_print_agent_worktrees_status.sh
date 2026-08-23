#!/usr/bin/env bash
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
# .idea/, .claude/ and .claude-worktree never count, everything else does.
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
test_clean_worktree_counts_zero

echo "PASS: print_agent_worktrees_status.sh"
