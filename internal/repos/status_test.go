package repos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusFixture is a three-row document captured from a real `agentwt status
// --format json` run (behind lists cut to two entries): a sha-base row with a
// clean, locked worktree, a live Claude session and one unpicked commit; a
// ref-base row without worktree and three unpicked commits; a detached Codex
// row with its rollout session (kept from the previous capture — the rollout
// has aged out of the live repo since).
func statusFixture(t *testing.T) string {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("testdata", "status_document.json"))
	require.NoError(t, err)
	return string(fixture)
}

func TestStatusDecodesDocument(t *testing.T) {
	scan, err := decodeStatus(statusFixture(t))
	require.NoError(t, err)
	statuses := scan.Rows
	require.Len(t, statuses, 3)

	first := statuses[0]
	assert.Equal(t, "claude/worktree-pool-diagnosis-f85b2a", first.Branch)
	assert.Equal(t, "claude", first.Kind)
	assert.False(t, first.Detached)
	assert.Equal(t, "main", first.From)
	assert.Equal(t, "sha", first.FromResolution)
	assert.Equal(t, 0, first.Dirty)
	assert.Equal(t, 0, first.Untracked)
	assert.Equal(t, 1, first.Ahead)
	assert.Equal(t, 1, first.Behind)
	// The own commit is on no non-agent branch: no containment, one unpicked commit.
	assert.Equal(t, 1, first.Unpicked)
	assert.Nil(t, first.Contained)
	require.Len(t, first.UnpickedCommits, 1)
	assert.Equal(t, "23c2b82f732cfd3230f5c946009f84f3b6e1582b", first.UnpickedCommits[0].Sha)
	assert.Equal(t, "2026-09-25 20:11", first.LastCommit)
	assert.Equal(t, "/Users/devuser/GolandProjects/claude-configs/.claude/worktrees/worktree-deletion-bug-996e7b", first.Worktree)
	assert.False(t, first.SafeToRemove)
	assert.Equal(t, []string{"unpicked(1)"}, first.UnsafeReasons)
	require.NotNil(t, first.Order)
	assert.Equal(t, "clean", first.Order.State)
	assert.Equal(t, 2, first.Order.TrainPosition)
	require.NotNil(t, first.Session)
	assert.Equal(t, "claude", first.Session.Agent)
	assert.Equal(t, "3d7f877a-10dd-4ae7-9fba-5c4edca1c765", first.Session.Id)
	require.NotNil(t, first.Session.Alive)
	assert.True(t, *first.Session.Alive)
	assert.Equal(t, "idle", first.Session.Status)
	assert.Equal(t, "CC, FD - Worktree pool branch mismatch diagnosis", first.Session.Name)

	second := statuses[1]
	assert.Equal(t, "claude/skillroutine-eval-railroad-review-09145b", second.Branch)
	assert.Equal(t, "claude", second.Kind)
	assert.Equal(t, "main", second.From)
	assert.Equal(t, "ref", second.FromResolution)
	assert.Equal(t, noWorktreeDirty, second.Dirty)
	assert.Equal(t, noWorktreeDirty, second.Untracked)
	assert.Empty(t, second.Worktree)
	assert.Equal(t, 3, second.Ahead)
	assert.Equal(t, 315, second.Behind)
	assert.Equal(t, 3, second.Unpicked)
	assert.Nil(t, second.Contained)
	assert.False(t, second.SafeToRemove)
	assert.Equal(t, []string{"unpicked(3)", "no worktree"}, second.UnsafeReasons)
	assert.Nil(t, second.Session)
	require.NotNil(t, second.Order)
	assert.Equal(t, "clean", second.Order.State)
	assert.Equal(t, 1, second.Order.TrainPosition)
	assert.Len(t, second.AheadCommits, 3)
	assert.Len(t, second.BehindCommits, 2)
	assert.True(t, second.BehindTruncated)
	// Unpicked = the list the binary sends, in ahead order.
	require.Len(t, second.UnpickedCommits, 3)
	assert.Equal(t, "a043df217ac0f18962014f9ef060835f6a128ea1", second.UnpickedCommits[0].Sha)
	assert.Equal(t, "a043df21  2026-09-17 11:18  evals.railroad-review: Added the 2026-09-16-a34281 bootstrap eval run", second.UnpickedCommits[0].Line)

	third := statuses[2]
	assert.Empty(t, third.Branch)
	assert.Equal(t, "0d079e76", third.Head)
	assert.True(t, third.Detached)
	assert.Equal(t, "codex", third.Kind)
	assert.Equal(t, unknownBase, third.From)
	assert.Equal(t, "unknown", third.FromResolution)
	assert.Equal(t, unknownCount, third.Ahead)
	assert.Equal(t, unknownCount, third.Behind)
	assert.Equal(t, 0, third.Dirty)
	assert.Equal(t, "/Users/devuser/.codex/worktrees/ffc8/claude-configs", third.Worktree)
	assert.True(t, third.SafeToRemove)
	assert.Empty(t, third.UnsafeReasons)
	assert.Equal(t, &Containment{Branch: "main", Reason: "ancestor", Rank: "default"}, third.Contained)
	require.NotNil(t, third.Session)
	assert.Equal(t, "codex", third.Session.Agent)
	assert.Nil(t, third.Session.Alive)
}

func TestStatusRejectsUnknownSchema(t *testing.T) {
	_, err := decodeStatus(`{"schema":2,"repo":{"default_branch":"main"},"rows":[]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unsupported schema 2")
}

func TestStatusRejectsMalformedDocument(t *testing.T) {
	_, err := decodeStatus("error: not json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Decode")
}

func TestStatusEmptyRows(t *testing.T) {
	scan, err := decodeStatus(`{"schema":1,"repo":{"default_branch":"main"},"rows":[]}`)
	require.NoError(t, err)
	assert.Empty(t, scan.Rows)
	assert.Equal(t, StatusTimings{}, scan.Timings)
}

func TestStatusDecodesTimings(t *testing.T) {
	document := `{"schema":1,"repo":{"default_branch":"main"},"rows":[],` +
		`"timings":{"analysis_ms":540,"worktrees_ms":213,"order_ms":77,"gitstate_ms":541}}`
	scan, err := decodeStatus(document)
	require.NoError(t, err)
	expected := StatusTimings{AnalysisMs: 540, OrderMs: 77, WorktreesMs: 213}
	assert.Equal(t, expected, scan.Timings)
}

func TestStatusDecodesKinds(t *testing.T) {
	withKinds := `{"schema":1,"repo":{"default_branch":"main"},"rows":[],"kinds":["claude","codex","claude-routines"]}`
	scan, err := decodeStatus(withKinds)
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "codex", "claude-routines"}, scan.Kinds)

	withoutKinds := `{"schema":1,"repo":{"default_branch":"main"},"rows":[]}`
	scan, err = decodeStatus(withoutKinds)
	require.NoError(t, err)
	assert.Nil(t, scan.Kinds)
}

func TestCommitsByClass(t *testing.T) {
	status := &WorktreeStatus{
		AheadCommits:    []Commit{{Sha: "a"}},
		BehindCommits:   []Commit{{Sha: "b"}, {Sha: "c"}},
		UnpickedCommits: []Commit{{Sha: "u"}},
	}
	cases := []struct {
		class string
		want  []string
	}{
		{class: "ahead", want: []string{"a"}},
		{class: "behind", want: []string{"b", "c"}},
		{class: "unpicked", want: []string{"u"}},
		{class: "bogus", want: nil},
	}
	for _, c := range cases {
		var shas []string
		for _, commit := range Commits(c.class, status) {
			shas = append(shas, commit.Sha)
		}
		assert.Equalf(t, c.want, shas, "class=%s", c.class)
	}
}

func TestUnpickedLinesFollowsList(t *testing.T) {
	ahead := []statusCommitEntry{
		{Sha: "n", ShortSha: "n1234567", Subject: "newest", CommittedAt: "2026-07-15T10:02:00Z"},
		{Sha: "m", ShortSha: "m1234567", Subject: "merge", CommittedAt: "2026-07-15T10:01:00Z", IsMerge: true},
		{Sha: "p", ShortSha: "p1234567", Subject: "picked", CommittedAt: "2026-07-15T10:00:30Z"},
		{Sha: "o", ShortSha: "o1234567", Subject: "own", CommittedAt: "2026-07-15T10:00:00Z"},
	}
	lines := unpickedLines(ahead, []string{"o", "n"})
	require.Len(t, lines, 2)
	assert.Equal(t, "n", lines[0].Sha)
	assert.Equal(t, "o", lines[1].Sha)
	assert.Equal(t, "o1234567  2026-07-15 10:00  own", lines[1].Line)
}

// writeStatusStub creates a stub agentwt binary: `status` prints the fixture
// document, anything else the binary's error contract (exit 1).
func writeStatusStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "document.json")
	require.NoError(t, os.WriteFile(fixturePath, []byte(statusFixture(t)), 0o644))
	script := `#!/bin/sh
for arg in "$@"; do
	if [ "$arg" = "status" ]; then
		cat "` + fixturePath + `"
		exit 0
	fi
done
echo "agentwt: unknown command $*" >&2
exit 1
`
	stub := filepath.Join(dir, "agentwt")
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))
	return stub
}

func TestStatusRunsBinaryInRepoPath(t *testing.T) {
	stub := writeStatusStub(t)
	scan, err := Status(context.Background(), stub, "", t.TempDir())
	require.NoError(t, err)
	assert.Len(t, scan.Rows, 3)
}

func TestStatusBinaryFailureKeepsOutput(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "agentwt")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\necho 'agentwt: no default branch could be determined' >&2\nexit 1\n"), 0o755))
	_, err := Status(context.Background(), stub, "", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no default branch could be determined")
}

// writeArgvStub creates a stub agentwt binary that records its argv and
// prints an empty status document holding one kind.
func writeArgvStub(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	argvPath := filepath.Join(dir, "argv")
	script := `#!/bin/sh
echo "$@" > "` + argvPath + `"
echo '{"schema":1,"repo":{"default_branch":"main"},"rows":[],"kinds":["claude"]}'
`
	stub := filepath.Join(dir, "agentwt")
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))
	return stub, argvPath
}

func TestStatusKindArgument(t *testing.T) {
	repoPath := t.TempDir()

	stub, argvPath := writeArgvStub(t)
	scan, err := Status(context.Background(), stub, "", repoPath)
	require.NoError(t, err)
	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	assert.Equal(t, "--format json -C "+repoPath+" status\n", string(argv))
	assert.Equal(t, []string{"claude"}, scan.Kinds)

	stub, argvPath = writeArgvStub(t)
	scan, err = Status(context.Background(), stub, "claude-routines", repoPath)
	require.NoError(t, err)
	argv, err = os.ReadFile(argvPath)
	require.NoError(t, err)
	assert.Equal(t, "--format json -C "+repoPath+" status --kind=claude-routines\n", string(argv))
	assert.Equal(t, []string{"claude"}, scan.Kinds)
}
