#!/usr/bin/env bash
# Regression coverage for prune_jetbrains_recent_projects.sh: worktree entries
# are dropped, non-worktree entries are kept, and a backup is written.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/prune-recent.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == prune-recent.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT="$REPO_DIR/cmd/worktrees/prune_jetbrains_recent_projects.sh"
OPTS="$TMP/home/Library/Application Support/JetBrains/GoLand2026.1/options"
XML="$OPTS/recentProjects.xml"
PYCHARM_OPTS="$TMP/home/Library/Application Support/JetBrains/PyCharm2026.1/options"
PYCHARM_XML="$PYCHARM_OPTS/recentProjects.xml"

write_fixture() {
  mkdir -p "$OPTS" "$PYCHARM_OPTS"
  rm -f "$OPTS"/recentProjects.xml.bak.* "$PYCHARM_OPTS"/recentProjects.xml.bak.*
  cat > "$PYCHARM_XML" <<'EOF'
<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/PycharmProjects/scraper">
          <value><RecentProjectMetaInfo frameTitle="scraper" /></value>
        </entry>
        <entry key="$USER_HOME$/PycharmProjects/scraper/.claude/worktrees/baz-789">
          <value><RecentProjectMetaInfo frameTitle="baz-789" /></value>
        </entry>
      </map>
    </option>
  </component>
</application>
EOF
  cat > "$XML" <<'EOF'
<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/GolandProjects/project-alpha">
          <value>
            <RecentProjectMetaInfo frameTitle="project-alpha">
              <option name="activationTimestamp" value="1" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/project-alpha/.claude/worktrees/foo-123">
          <value>
            <RecentProjectMetaInfo frameTitle="foo-123">
              <option name="activationTimestamp" value="2" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/project-beta/.claude/worktrees/bar-456">
          <value>
            <RecentProjectMetaInfo frameTitle="bar-456">
              <option name="activationTimestamp" value="3" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/other-project">
          <value>
            <RecentProjectMetaInfo frameTitle="other-project">
              <option name="activationTimestamp" value="4" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
      </map>
    </option>
  </component>
</application>
EOF
}

# --force so the test is independent of whether GoLand happens to be running.
run() { HOME="$TMP/home" bash "$SCRIPT" --force "$@"; }

test_dry_run_changes_nothing() {
  write_fixture
  local before after
  before=$(cat "$XML")
  run --dry-run >/dev/null
  after=$(cat "$XML")
  [ "$before" = "$after" ] || fail "dry-run modified the file"
}

test_prune_removes_only_worktrees() {
  write_fixture
  run >/dev/null

  grep -q 'key="\$USER_HOME\$/GolandProjects/project-alpha"' "$XML" \
    || fail "removed the non-worktree project-alpha entry"
  grep -q 'key="\$USER_HOME\$/GolandProjects/other-project"' "$XML" \
    || fail "removed the non-worktree other-project entry"
  ! grep -q 'worktrees' "$XML" || fail "worktree entries survived"

  # both worktree frame titles gone, both plain ones kept
  grep -q 'frameTitle="project-alpha"' "$XML" || fail "lost project-alpha metadata"
  grep -q 'frameTitle="other-project"' "$XML" || fail "lost other-project metadata"
  ! grep -q 'frameTitle="foo-123"' "$XML" || fail "foo-123 worktree block survived"
  ! grep -q 'frameTitle="bar-456"' "$XML" || fail "bar-456 worktree block survived"

  # XML is still balanced: 2 entries remain
  [ "$(grep -c '<entry key=' "$XML")" -eq 2 ] || fail "expected 2 entries after prune"
  [ "$(grep -c '</entry>' "$XML")" -eq 2 ] || fail "unbalanced </entry> tags after prune"

  # the second IDE's file is pruned too
  grep -q 'frameTitle="scraper"' "$PYCHARM_XML" || fail "removed the non-worktree scraper entry"
  ! grep -q 'frameTitle="baz-789"' "$PYCHARM_XML" || fail "baz-789 worktree block survived in PyCharm file"
  [ "$(grep -c '<entry key=' "$PYCHARM_XML")" -eq 1 ] || fail "expected 1 entry in PyCharm file after prune"
}

test_summary_counts_all_ides() {
  write_fixture
  local out
  out=$(run)
  echo "$out" | grep -q 'removed 3 worktree entries across 2 file(s)' \
    || fail "summary did not count both IDE files: $out"
}

test_backup_written() {
  write_fixture
  run >/dev/null
  ls "$OPTS"/recentProjects.xml.bak.* >/dev/null 2>&1 || fail "no backup written"
}

test_no_worktrees_noop() {
  mkdir -p "$OPTS"
  rm -f "$OPTS"/recentProjects.xml.bak.*
  cat > "$XML" <<'EOF'
<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/GolandProjects/clean-project">
          <value><RecentProjectMetaInfo frameTitle="clean-project" /></value>
        </entry>
      </map>
    </option>
  </component>
</application>
EOF
  run >/dev/null
  ls "$OPTS"/recentProjects.xml.bak.* >/dev/null 2>&1 && fail "backup written for a no-op run"
  [ "$(grep -c '<entry key=' "$XML")" -eq 1 ] || fail "clean file was altered"
}

test_dry_run_changes_nothing
test_prune_removes_only_worktrees
test_summary_counts_all_ides
test_backup_written
test_no_worktrees_noop

echo "PASS: prune_jetbrains_recent_projects.sh"
