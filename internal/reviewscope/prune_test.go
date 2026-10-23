package reviewscope

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/trash"
)

// listTree lists every path under root, slash-separated, dirs with a trailing
// slash; empty when root is missing or no directory.
func listTree(t *testing.T, root string) []string {
	t.Helper()
	paths := make([]string, 0)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		require.NoError(t, err)
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			relative += "/"
		}
		paths = append(paths, relative)
		return nil
	})
	return paths
}

// movedFiles lists the artifact-relative paths the review prune journaled into store, sorted.
func movedFiles(t *testing.T, artifactsDir string, store *trash.Store) []string {
	t.Helper()
	journal, err := store.Tail(1000)
	require.NoError(t, err)
	moved := make([]string, 0, len(journal))
	for _, entry := range journal {
		relative, err := filepath.Rel(artifactsDir, entry.Path)
		require.NoError(t, err)
		moved = append(moved, filepath.ToSlash(relative))
	}
	slices.Sort(moved)
	return moved
}

// stashOf is result's stash stamp, "" without a result.
func stashOf(result *PruneResult) string {
	if result == nil {
		return ""
	}
	return result.Stash
}

// stashedCount counts the entries every stamp dir of store holds.
func stashedCount(t *testing.T, store *trash.Store) int {
	t.Helper()
	stashes, err := store.List()
	require.NoError(t, err)
	count := 0
	for _, stash := range stashes {
		count += len(stash.Entries)
	}
	return count
}

// withoutStash copies result with Stash cleared, so the stamp stays out of the struct comparison.
func withoutStash(result *PruneResult) *PruneResult {
	if result == nil {
		return nil
	}

	copied := *result
	copied.Stash = ""
	return &copied
}

// writeTree creates each relative file under root with its parent dirs.
func writeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(file+"\n"), 0o644))
	}
}

func TestPrune(t *testing.T) {
	type testCase struct {
		_id               string
		_expected         *PruneResult
		_expectedHasStash bool
		_expectedMoved    []string
		_expectedTree     []string
		_shouldPass       bool

		artifactsDir string
		isVerbose    bool
		store        *trash.Store
	}

	tests := make([]*testCase, 0)

	// keep-list
	keepList := t.TempDir()
	writeTree(t, keepList,
		"coverage.json",
		"known.json",
		"review-record.md",
		"candidates.json",
		"prebuild.log",
		"round-1/checklist-correctness.json",
		"round-1/chunks.json",
		"round-1/known.json",
		"round-1/review.json",
		"round-1/risk-map.json",
		"round-1/correctness-blk-1-lane1.json",
		"round-1/correctness-blk-1-lane1.json.coverage.json",
		"round-1/correctness-blk-1-lane1.json.lane-manifest.json",
		"round-1/verdicts/correctness-blk-1-r1.json",
		"round-1/probes/p1.json",
		"round-1/spec.md",
		"round-1/sub/review.json",
		"round-x/review.json",
	)
	tests = append(tests, &testCase{
		_id:               "keep-list",
		_expected:         &PruneResult{Kept: 8, Removed: 10},
		_expectedHasStash: true,
		_expectedMoved: []string{
			"candidates.json",
			"prebuild.log",
			"round-1/correctness-blk-1-lane1.json",
			"round-1/correctness-blk-1-lane1.json.coverage.json",
			"round-1/correctness-blk-1-lane1.json.lane-manifest.json",
			"round-1/probes/p1.json",
			"round-1/spec.md",
			"round-1/sub/review.json",
			"round-1/verdicts/correctness-blk-1-r1.json",
			"round-x/review.json",
		},
		_expectedTree: []string{
			"coverage.json",
			"known.json",
			"review-record.md",
			"round-1/",
			"round-1/checklist-correctness.json",
			"round-1/chunks.json",
			"round-1/known.json",
			"round-1/review.json",
			"round-1/risk-map.json",
		},
		_shouldPass: true,

		artifactsDir: keepList,
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// salvage-dir-kept
	salvage := t.TempDir()
	writeTree(t, salvage,
		"review-record.md",
		"salvage/lane/untracked.txt",
		"salvage/lane/tracked.diff",
		"salvage/branches/claude-review_rr-a-1.diff",
	)
	tests = append(tests, &testCase{
		_id:               "salvage-dir-kept",
		_expected:         &PruneResult{Kept: 4, Removed: 0},
		_expectedHasStash: false,
		_expectedMoved:    []string{},
		_expectedTree: []string{
			"review-record.md",
			"salvage/",
			"salvage/branches/",
			"salvage/branches/claude-review_rr-a-1.diff",
			"salvage/lane/",
			"salvage/lane/tracked.diff",
			"salvage/lane/untracked.txt",
		},
		_shouldPass: true,

		artifactsDir: salvage,
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// verbose-keeps-lane-reports-and-verdicts
	verbose := t.TempDir()
	writeTree(t, verbose,
		"round-2/tests-server-lane2.json",
		"round-2/tests-server-lane2.json.coverage.json",
		"round-2/tests-server-lane2.json.lane-manifest.json",
		"round-2/verdicts/tests-blk-3-r2.json",
		"round-2/verdicts/notes.txt",
		"verdicts/top.json",
	)
	tests = append(tests, &testCase{
		_id:               "verbose-keeps-lane-reports-and-verdicts",
		_expected:         &PruneResult{Kept: 2, Removed: 4},
		_expectedHasStash: true,
		_expectedMoved: []string{
			"round-2/tests-server-lane2.json.coverage.json",
			"round-2/tests-server-lane2.json.lane-manifest.json",
			"round-2/verdicts/notes.txt",
			"verdicts/top.json",
		},
		_expectedTree: []string{
			"round-2/",
			"round-2/tests-server-lane2.json",
			"round-2/verdicts/",
			"round-2/verdicts/tests-blk-3-r2.json",
		},
		_shouldPass: true,

		artifactsDir: verbose,
		isVerbose:    true,
		store:        trash.New(t.TempDir(), 0),
	})

	// nested-git-removed
	nestedGit := t.TempDir()
	writeTree(t, nestedGit,
		"round-1/review.json",
		"round-1/scratch/repo/.git/HEAD",
		"round-1/scratch/repo/.git/objects/ab/cdef",
		"round-1/probes/fixtures/f1/.git/config",
	)
	tests = append(tests, &testCase{
		_id:               "nested-git-removed",
		_expected:         &PruneResult{Kept: 1, Removed: 3},
		_expectedHasStash: true,
		_expectedMoved: []string{
			"round-1/probes/fixtures/f1/.git/config",
			"round-1/scratch/repo/.git/HEAD",
			"round-1/scratch/repo/.git/objects/ab/cdef",
		},
		_expectedTree: []string{"round-1/", "round-1/review.json"},
		_shouldPass:   true,

		artifactsDir: nestedGit,
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// empty-dirs-removed
	emptyDirs := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(emptyDirs, "round-1", "probes", "fixtures"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(emptyDirs, "salvage"), 0o755))
	writeTree(t, emptyDirs, "coverage.json")
	tests = append(tests, &testCase{
		_id:               "empty-dirs-removed",
		_expected:         &PruneResult{Kept: 1, Removed: 0},
		_expectedHasStash: false,
		_expectedMoved:    []string{},
		_expectedTree:     []string{"coverage.json"},
		_shouldPass:       true,

		artifactsDir: emptyDirs,
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// symlinks-removed-not-followed
	symlinks := t.TempDir()
	writeTree(t, symlinks, "known.json", "round-2/review.json")
	require.NoError(t, os.MkdirAll(filepath.Join(symlinks, "round-1"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join("..", "known.json"), filepath.Join(symlinks, "round-1", "known.json")))
	require.NoError(t, os.Symlink("round-2", filepath.Join(symlinks, "linked")))
	tests = append(tests, &testCase{
		_id:               "symlinks-removed-not-followed",
		_expected:         &PruneResult{Kept: 2, Removed: 2},
		_expectedHasStash: true,
		_expectedMoved:    []string{"linked", "round-1/known.json"},
		_expectedTree:     []string{"known.json", "round-2/", "round-2/review.json"},
		_shouldPass:       true,

		artifactsDir: symlinks,
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// empty-artifacts-dir
	tests = append(tests, &testCase{
		_id:               "empty-artifacts-dir",
		_expected:         &PruneResult{Kept: 0, Removed: 0},
		_expectedHasStash: false,
		_expectedMoved:    []string{},
		_expectedTree:     []string{},
		_shouldPass:       true,

		artifactsDir: t.TempDir(),
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// missing-artifacts-dir
	tests = append(tests, &testCase{
		_id:               "missing-artifacts-dir",
		_expected:         nil,
		_expectedHasStash: false,
		_expectedMoved:    []string{},
		_expectedTree:     []string{},
		_shouldPass:       false,

		artifactsDir: filepath.Join(t.TempDir(), "absent"),
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// artifacts-dir-not-a-directory
	notDir := t.TempDir()
	writeTree(t, notDir, "artifacts")
	tests = append(tests, &testCase{
		_id:               "artifacts-dir-not-a-directory",
		_expected:         nil,
		_expectedHasStash: false,
		_expectedMoved:    []string{},
		_expectedTree:     []string{},
		_shouldPass:       false,

		artifactsDir: filepath.Join(notDir, "artifacts"),
		isVerbose:    false,
		store:        trash.New(t.TempDir(), 0),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := Prune(&PruneRequest{ArtifactsDir: test.artifactsDir, IsVerbose: test.isVerbose, Trash: test.store})
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, withoutStash(result))
			assert.Equal(t, test._expectedHasStash, stashOf(result) != "")
			assert.ElementsMatch(t, test._expectedTree, listTree(t, test.artifactsDir))
			assert.Equal(t, test._expectedMoved, movedFiles(t, test.artifactsDir, test.store))
			assert.Len(t, test._expectedMoved, stashedCount(t, test.store))
		})
	}
}
