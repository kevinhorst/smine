package skilleval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/trash"
)

const (
	cleanupArmBranch    = "claude-review/run-2026-09-27-abc123-a"
	cleanupRunName      = "2026-09-27-abc123"
	cleanupWorkerBranch = "claude-review/rr-2026-09-27-abc123-a-lane1"
)

// newCleanupRun writes a one-arm run dir named cleanupRunName over a fixture
// repo and creates the arm worktree on its branch; it returns the run dir and
// the arm worktree.
func newCleanupRun(t *testing.T) (string, string) {
	t.Helper()
	repo := newFixtureRepo(t)
	armWorktree := filepath.Join(newTempDir(t), "run-"+cleanupRunName+"-a")
	gitOutput(t, repo, "worktree", "add", "-q", "-b", cleanupArmBranch, armWorktree)
	runDir := filepath.Join(newTempDir(t), cleanupRunName)
	manifest := &RunManifest{
		Arms: []ManifestArm{{Id: "opus-high-r1", Key: "opus-high", Label: "A", Model: "opus", Replica: 1, Worktree: armWorktree}},
		At:   gitOutput(t, repo, "rev-parse", "HEAD"),
		Repo: repo,
	}
	writeRunDir(t, runDir, manifest)
	return runDir, armWorktree
}

// removedFlags lists the removed flag of each worktree row in report order.
func removedFlags(report *CleanupReport) []bool {
	flags := make([]bool, 0, len(report.Worktrees))
	for _, worktree := range report.Worktrees {
		flags = append(flags, worktree.Removed)
	}
	return flags
}

func TestCleanupArm_WorkflowWorktrees(t *testing.T) {
	runDir, armWorktree := newCleanupRun(t)
	manifest, err := loadManifest(runDir)
	require.NoError(t, err)
	harnessRoot := filepath.Join(manifest.Repo, ".claude", "worktrees")
	ownHook := filepath.Join(harnessRoot, "wf_afb757e5-ac8-1")
	ownHarness := filepath.Join(harnessRoot, "wf_afb757e5-ac8-2")
	ownForeignBranch := filepath.Join(harnessRoot, "wf_afb757e5-ac8-3")
	nearMiss := filepath.Join(harnessRoot, "wf_afb757e5-ac82-1")
	otherRun := filepath.Join(harnessRoot, "wf_0f7f2d73-f7b-1")
	nested := filepath.Join(harnessRoot, "sub", "wf_afb757e5-ac8-4")
	gitOutput(t, manifest.Repo, "worktree", "add", "-q", "-b", "claude/wf_afb757e5-ac8-1", ownHook)
	gitOutput(t, manifest.Repo, "worktree", "add", "-q", "-b", "worktree-wf_afb757e5-ac8-2", ownHarness)
	gitOutput(t, manifest.Repo, "worktree", "add", "-q", "-b", "claude/session-x", ownForeignBranch)
	gitOutput(t, manifest.Repo, "worktree", "add", "-q", "-b", "claude/wf_afb757e5-ac82-1", nearMiss)
	gitOutput(t, manifest.Repo, "worktree", "add", "-q", "-b", "claude/wf_0f7f2d73-f7b-1", otherRun)
	gitOutput(t, manifest.Repo, "worktree", "add", "-q", "-b", "claude/nested", nested)
	writeTestFile(t, filepath.Join(ownHook, "cell.txt"), "cell\n")
	stream := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"w1","content":"Workflow launched"}]},"tool_use_result":{"status":"async_launched","runId":"wf_afb757e5-ac8"}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"b1","content":"ok"}]},"tool_use_result":"plain text result"}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"w2","content":"x"}]},"tool_use_result":{"runId":"../escape"}}` + "\n"
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", streamFile), stream)

	report, err := CleanupArm(context.Background(), "A", runDir, trash.New(newTempDir(t), 0))

	require.NoError(t, err)
	assert.True(t, report.IsAllRemoved())
	deleted := []string{cleanupArmBranch, "claude/wf_afb757e5-ac8-1", "worktree-wf_afb757e5-ac8-2"}
	assert.Equal(t, deleted, report.Branches.Deleted)
	for _, removed := range []string{armWorktree, ownHook, ownHarness, ownForeignBranch} {
		assert.False(t, pathExists(removed), removed)
	}
	for _, kept := range []string{nearMiss, otherRun, nested} {
		assert.True(t, pathExists(kept), kept)
	}
	kept := []string{"claude/nested", "claude/session-x", "claude/wf_0f7f2d73-f7b-1", "claude/wf_afb757e5-ac82-1", "main"}
	assert.Equal(t, kept, localBranches(t, manifest.Repo))
	assert.False(t, pathExists(armWorktree+outDirSuffix))
	assert.False(t, pathExists(filepath.Join(runDir, runsDir, "A", salvageDir)))
}

func TestCleanupArm(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedAllRemoved bool
		_expectedDeleted    []string
		_expectedOutExists  bool
		_expectedRemoved    []bool
		_expectedSalvage    []string
		_expectedStashed    []string

		label    string
		outDir   string
		runDir   string
		stateDir string
	}

	tests := make([]*testCase, 0)

	// arm-and-worker-salvaged-out-dir-removed
	salvagedRun, salvagedArm := newCleanupRun(t)
	salvagedWorker := filepath.Join(salvagedArm+outDirSuffix, "wt", "lane1")
	gitOutput(t, salvagedArm, "worktree", "add", "-q", "-b", cleanupWorkerBranch, salvagedWorker)
	writeTestFile(t, filepath.Join(salvagedArm, "new.txt"), "new\n")
	writeTestFile(t, filepath.Join(salvagedWorker, "file.txt"), "one\nedit\n")
	writeTestFile(t, filepath.Join(salvagedArm+outDirSuffix, "prebuild.log"), "built\n")
	tests = append(tests, &testCase{
		_id:                 "arm-and-worker-salvaged-out-dir-removed",
		_expectedAllRemoved: true,
		_expectedDeleted:    []string{cleanupArmBranch, cleanupWorkerBranch},
		_expectedOutExists:  false,
		_expectedRemoved:    []bool{true, true},
		_expectedSalvage:    []string{},
		_expectedStashed: []string{
			"prebuild.log",
			filepath.Join(salvageDir, "arm-wt", "new.txt"),
			filepath.Join(salvageDir, "wt", "lane1", "tracked.diff"),
		},

		label:    "A",
		outDir:   salvagedArm + outDirSuffix,
		runDir:   salvagedRun,
		stateDir: newTempDir(t),
	})

	// branches-deleted-with-diffs
	diffRun, diffArm := newCleanupRun(t)
	diffWorker := filepath.Join(diffArm+outDirSuffix, "wt", "lane1")
	gitOutput(t, diffArm, "worktree", "add", "-q", "-b", cleanupWorkerBranch, diffWorker)
	writeTestFile(t, filepath.Join(diffWorker, "committed.txt"), "change\n")
	commitAll(t, diffWorker, "change")
	tests = append(tests, &testCase{
		_id:                 "branches-deleted-with-diffs",
		_expectedAllRemoved: true,
		_expectedDeleted:    []string{cleanupArmBranch, cleanupWorkerBranch},
		_expectedOutExists:  false,
		_expectedRemoved:    []bool{true, true},
		_expectedSalvage:    []string{},
		_expectedStashed: []string{
			filepath.Join(salvageDir, "branches", "claude-review_rr-2026-09-27-abc123-a-lane1.diff"),
			filepath.Join(salvageDir, "wt", "lane1", "commits.diff"),
		},

		label:    "A",
		outDir:   diffArm + outDirSuffix,
		runDir:   diffRun,
		stateDir: newTempDir(t),
	})

	// empty-out-dir-removed
	emptyRun, emptyArm := newCleanupRun(t)
	require.NoError(t, os.MkdirAll(filepath.Join(emptyArm+outDirSuffix, "wt"), 0o755))
	tests = append(tests, &testCase{
		_id:                 "empty-out-dir-removed",
		_expectedAllRemoved: true,
		_expectedDeleted:    []string{cleanupArmBranch},
		_expectedOutExists:  false,
		_expectedRemoved:    []bool{true},
		_expectedSalvage:    []string{},
		_expectedStashed:    []string{},

		label:    "A",
		outDir:   emptyArm + outDirSuffix,
		runDir:   emptyRun,
		stateDir: newTempDir(t),
	})

	// refused-removal-keeps-out-dir
	lockedRun, lockedArm := newCleanupRun(t)
	lockedWorker := filepath.Join(lockedArm+outDirSuffix, "wt", "lane1")
	gitOutput(t, lockedArm, "worktree", "add", "-q", "-b", cleanupWorkerBranch, lockedWorker)
	writeTestFile(t, filepath.Join(lockedWorker, "file.txt"), "one\nedit\n")
	gitOutput(t, lockedArm, "worktree", "lock", lockedWorker)
	tests = append(tests, &testCase{
		_id:                 "refused-removal-keeps-out-dir",
		_expectedAllRemoved: false,
		_expectedDeleted:    []string{cleanupArmBranch},
		_expectedOutExists:  true,
		_expectedRemoved:    []bool{false, true},
		_expectedSalvage:    []string{filepath.Join("wt", "lane1", "tracked.diff")},
		_expectedStashed:    []string{},

		label:    "A",
		outDir:   lockedArm + outDirSuffix,
		runDir:   lockedRun,
		stateDir: newTempDir(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := CleanupArm(context.Background(), test.label, test.runDir, trash.New(test.stateDir, 0))
			require.NoError(t, err)
			assert.Equal(t, test._expectedOutExists, report.Stash == "")
			assert.Equal(t, test._expectedStashed, listFiles(t, filepath.Join(test.stateDir, trash.TrashDir, report.Stash)))
			assert.Equal(t, test._expectedAllRemoved, report.IsAllRemoved())
			assert.Equal(t, test._expectedDeleted, report.Branches.Deleted)
			assert.Equal(t, test._expectedRemoved, removedFlags(report))
			assert.Equal(t, test._expectedSalvage, listFiles(t, filepath.Join(test.outDir, salvageDir)))
			assert.Equal(t, test._expectedOutExists, pathExists(test.outDir))
			assert.False(t, pathExists(filepath.Join(test.runDir, runsDir, test.label, salvageDir)))
		})
	}
}
