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
// in IN, UNPICKED 0), the "unknown" FROM row with all "-"/"–" sentinels, and
// an untracked-blocked row.
const statusFixture = `#    BRANCH                               FROM             DIRTY  UNTRACKED AHEAD  BEHIND  UNPICKED  VERDICTS             MERGED           IN                       LAST-COMMIT      WORKTREE
1    claude/feature-a                     origin/main      0      0         1      2       0         resolved:1           -                origin/main*             2026-07-10 12:30 /tmp/repo/.claude/worktrees/feature-a
2    claude/feature-b                     unknown          -      -         -      -       0         -                    -                -                        2026-07-09 08:15 –
3    claude/feature-c                     main             0      3         0      0       0         -                    -                main                     2026-07-08 09:00 /tmp/repo/.claude/worktrees/feature-c
`

func TestParseStatusFixture(t *testing.T) {
	statuses, err := parseStatus(statusFixture)
	require.NoError(t, err)
	require.Len(t, statuses, 3)

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

	// Untracked files alone must break the safe contract — the remove script
	// blocks on them even when the work is contained elsewhere. Plain "main"
	// IN entry (no star) means exact containment, not a probe upgrade.
	third := statuses[2]
	assert.Equal(t, 0, third.Dirty)
	assert.Equal(t, 3, third.Untracked)
	assert.Equal(t, []string{"main"}, third.In)
	assert.False(t, third.SafeToRemove)
	assert.False(t, third.SafeViaProbe)
}

func TestParseStatusNoBranches(t *testing.T) {
	statuses, err := parseStatus("No claude/* branches found.\n")
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
	assert.Len(t, statuses, 3)
}
