package main

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRunMergeBranch runs merge-branch on args in repo and returns its
// exit code and everything it printed to stdout.
func captureRunMergeBranch(t *testing.T, repo string, args []string) (int, string) {
	t.Helper()
	t.Chdir(repo)
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runMergeBranch(args)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	printed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(printed)
}

// provideMergeRepo returns a repo on main with one commit.
func provideMergeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q", "-b", "main")
	gitOutput(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-q", "-m", "base")
	return repo
}

func TestRunMergeBranch(t *testing.T) {
	type testCase struct {
		_id               string
		_expected         int
		_expectedBranches []string
		_expectedHead     string
		_expectedStdout   string

		args []string
		repo string
	}

	tests := make([]*testCase, 0)

	// create-prefix
	createRepo := provideMergeRepo(t)
	gitOutput(t, createRepo, "branch", "theirs")
	tests = append(tests, &testCase{
		_id:               "create-prefix",
		_expected:         exitClean,
		_expectedBranches: []string{"claude-merge/theirs-main", "main", "theirs"},
		_expectedHead:     "claude-merge/theirs-main",
		_expectedStdout:   "claude-merge/theirs-main\n",

		args: []string{"create", "main", "theirs"},
		repo: createRepo,
	})

	// create-slug
	slugRepo := provideMergeRepo(t)
	gitOutput(t, slugRepo, "branch", "claude/Feature_X")
	tests = append(tests, &testCase{
		_id:               "create-slug",
		_expected:         exitClean,
		_expectedBranches: []string{"claude-merge/claude-feature-x-main", "claude/Feature_X", "main"},
		_expectedHead:     "claude-merge/claude-feature-x-main",
		_expectedStdout:   "claude-merge/claude-feature-x-main\n",

		args: []string{"create", "main", "claude/Feature_X"},
		repo: slugRepo,
	})

	// cleanup-merged-deleted
	mergedRepo := provideMergeRepo(t)
	gitOutput(t, mergedRepo, "branch", "claude-merge/done")
	tests = append(tests, &testCase{
		_id:               "cleanup-merged-deleted",
		_expected:         exitClean,
		_expectedBranches: []string{"main"},
		_expectedHead:     "main",
		_expectedStdout:   "deleted: claude-merge/done\n",

		args: []string{"cleanup", "main"},
		repo: mergedRepo,
	})

	// cleanup-unmerged-kept
	unmergedRepo := provideMergeRepo(t)
	gitOutput(t, unmergedRepo, "checkout", "-q", "-b", "claude-merge/open")
	gitOutput(t, unmergedRepo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-q", "-m", "extra")
	gitOutput(t, unmergedRepo, "checkout", "-q", "main")
	tests = append(tests, &testCase{
		_id:               "cleanup-unmerged-kept",
		_expected:         exitClean,
		_expectedBranches: []string{"claude-merge/open", "main"},
		_expectedHead:     "main",
		_expectedStdout:   "kept (not merged into main): claude-merge/open\n",

		args: []string{"cleanup", "main"},
		repo: unmergedRepo,
	})

	// cleanup-ignores-legacy
	legacyRepo := provideMergeRepo(t)
	gitOutput(t, legacyRepo, "branch", "merge/legacy")
	tests = append(tests, &testCase{
		_id:               "cleanup-ignores-legacy",
		_expected:         exitClean,
		_expectedBranches: []string{"main", "merge/legacy"},
		_expectedHead:     "main",
		_expectedStdout:   "",

		args: []string{"cleanup", "main"},
		repo: legacyRepo,
	})

	// usage-unknown-command
	unknownRepo := provideMergeRepo(t)
	tests = append(tests, &testCase{
		_id:               "usage-unknown-command",
		_expected:         mergeExitUsage,
		_expectedBranches: []string{"main"},
		_expectedHead:     "main",
		_expectedStdout:   "",

		args: []string{"merge", "main"},
		repo: unknownRepo,
	})

	// usage-create-missing-theirs
	missingRepo := provideMergeRepo(t)
	tests = append(tests, &testCase{
		_id:               "usage-create-missing-theirs",
		_expected:         mergeExitUsage,
		_expectedBranches: []string{"main"},
		_expectedHead:     "main",
		_expectedStdout:   "",

		args: []string{"create", "main"},
		repo: missingRepo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, stdout := captureRunMergeBranch(t, test.repo, test.args)
			assert.Equal(t, test._expected, code)
			assert.Equal(t, test._expectedStdout, stdout)
			assert.Equal(t, test._expectedHead, gitOutput(t, test.repo, "symbolic-ref", "--short", "HEAD"))
			assert.Equal(t, test._expectedBranches, localBranches(t, test.repo))
		})
	}
}

func TestSlug(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		name string
	}

	tests := make([]*testCase, 0)

	// plain
	tests = append(tests, &testCase{
		_id:       "plain",
		_expected: "main",

		name: "main",
	})

	// mixed-case-and-separators
	tests = append(tests, &testCase{
		_id:       "mixed-case-and-separators",
		_expected: "claude-feature-x",

		name: "claude/Feature_X",
	})

	// edge-runs-trimmed
	tests = append(tests, &testCase{
		_id:       "edge-runs-trimmed",
		_expected: "release-1-2",

		name: "--Release/1.2__",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, slug(test.name))
		})
	}
}
