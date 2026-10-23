package reviewscope

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	buildTestBranch = "claude-review/test-buildtest"
	makeBuild       = "make build"
	makeTest        = "make test"
)

// branchesOf cuts each skipped row to its branch name, dropping git's
// refusal text.
func branchesOf(rows []string) []string {
	branches := make([]string, 0, len(rows))
	for _, row := range rows {
		branch, _, _ := strings.Cut(row, ": ")
		branches = append(branches, branch)
	}
	return branches
}

// commitFixtureFile commits one file with content on main and returns the tip.
func commitFixtureFile(fixture *gitFixture, content, name string) string {
	fixture.t.Helper()
	require.NoError(fixture.t, os.WriteFile(filepath.Join(fixture.repo, name), []byte(content), 0o644))
	fixture.run("add", name)
	fixture.run("commit", "-qm", name)
	return fixture.run("rev-parse", "HEAD")
}

// provideTree is a directory holding the named files with their contents.
func provideTree(t *testing.T, files map[string]string) string {
	t.Helper()
	tree := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tree, name), []byte(content), 0o644))
	}
	return tree
}

// pathExists reports whether path exists on disk.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestRunBuildTest(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedBuildPrefix string
		_expectedBuildText   string
		_expectedSalvaged    []string
		_expectedTestsPrefix string
		_expectedTestsText   string

		fixture *gitFixture
		request *BuildTestRequest
	}

	tests := make([]*testCase, 0)

	// green-flag-commands
	green := newGitFixture(t)
	greenHead := commitFixtureFile(green, "build:\n\ttrue\n\ntest:\n\ttrue\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                  "green-flag-commands",
		_expectedBuildPrefix: "ok",
		_expectedBuildText:   "ok",
		_expectedSalvaged:    []string{},
		_expectedTestsPrefix: "ok",
		_expectedTestsText:   "ok",

		fixture: green,
		request: &BuildTestRequest{Branch: buildTestBranch, Build: makeBuild, Head: greenHead, Overlay: "", Repo: green.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// two-build-lines-run-in-order
	ordered := newGitFixture(t)
	orderedHead := commitFixtureFile(ordered, "first:\n\ttouch out.bin\n\nsecond:\n\ttest -f out.bin\n\ttouch second.bin\n\ntest:\n\ttrue\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                  "two-build-lines-run-in-order",
		_expectedBuildPrefix: "ok",
		_expectedBuildText:   "ok",
		_expectedSalvaged:    []string{"out.bin", "second.bin"},
		_expectedTestsPrefix: "ok",
		_expectedTestsText:   "ok",

		fixture: ordered,
		request: &BuildTestRequest{Branch: buildTestBranch, Build: "make first\nmake second", Head: orderedHead, Overlay: "", Repo: ordered.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// second-line-red-stops
	stopped := newGitFixture(t)
	stoppedHead := commitFixtureFile(stopped, "build:\n\ttrue\n\nbroken:\n\t@echo second line broke\n\t@exit 1\n\nnever:\n\ttouch out.bin\n\ntest:\n\ttrue\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                  "second-line-red-stops",
		_expectedBuildPrefix: "failed: ",
		_expectedBuildText:   "second line broke",
		_expectedSalvaged:    []string{},
		_expectedTestsPrefix: "ok",
		_expectedTestsText:   "ok",

		fixture: stopped,
		request: &BuildTestRequest{Branch: buildTestBranch, Build: "make build\nmake broken\nmake never", Head: stoppedHead, Overlay: "", Repo: stopped.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// red-test-decisive-lines
	red := newGitFixture(t)
	redHead := commitFixtureFile(red, "build:\n\ttrue\n\ntest:\n\t@echo broken assertion\n\t@exit 1\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                  "red-test-decisive-lines",
		_expectedBuildPrefix: "ok",
		_expectedBuildText:   "ok",
		_expectedSalvaged:    []string{},
		_expectedTestsPrefix: "failed: ",
		_expectedTestsText:   "broken assertion",

		fixture: red,
		request: &BuildTestRequest{Branch: buildTestBranch, Build: makeBuild, Head: redHead, Overlay: "", Repo: red.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// worktree-salvaged-and-branch-deleted
	salvaged := newGitFixture(t)
	commitFixtureFile(salvaged, "out.bin\n", ".gitignore")
	salvagedHead := commitFixtureFile(salvaged, "build:\n\ttouch out.bin\n\ntest:\n\ttrue\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                  "worktree-salvaged-and-branch-deleted",
		_expectedBuildPrefix: "ok",
		_expectedBuildText:   "ok",
		_expectedSalvaged:    []string{"out.bin"},
		_expectedTestsPrefix: "ok",
		_expectedTestsText:   "ok",

		fixture: salvaged,
		request: &BuildTestRequest{Branch: buildTestBranch, Build: makeBuild, Head: salvagedHead, Overlay: "", Repo: salvaged.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := RunBuildTest(context.Background(), test.request)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(result.Build, test._expectedBuildPrefix), result.Build)
			assert.Contains(t, result.Build, test._expectedBuildText)
			assert.True(t, strings.HasPrefix(result.Tests, test._expectedTestsPrefix), result.Tests)
			assert.Contains(t, result.Tests, test._expectedTestsText)
			assert.True(t, result.Salvage.Removed, result.Salvage.Reason)
			assert.Len(t, test._expectedSalvaged, result.Salvage.Moved)
			assert.False(t, pathExists(result.Salvage.Path))
			assert.Equal(t, len(test._expectedSalvaged) > 0, pathExists(filepath.Join(test.request.Salvage, filepath.Base(result.Salvage.Path), "out.bin")))
			assert.Empty(t, test.fixture.run("branch", "--list", buildTestBranch))
		})
	}
}

func TestRunBuildTest_Unconfigured(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedErrorText string

		request *BuildTestRequest
	}

	absentRepo := filepath.Join(t.TempDir(), "absent-repo")
	tests := make([]*testCase, 0)

	// missing-build-exits-with-fix
	tests = append(tests, &testCase{
		_id:                "missing-build-exits-with-fix",
		_expectedErrorText: "RunBuildTest: commandLines: No build command — the repo's build and test commands are set on the skill page",

		request: &BuildTestRequest{Branch: buildTestBranch, Build: "", Head: "deadbeef", Overlay: "", Repo: absentRepo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// missing-test-exits-with-fix
	tests = append(tests, &testCase{
		_id:                "missing-test-exits-with-fix",
		_expectedErrorText: "RunBuildTest: commandLines: No test command — the repo's build and test commands are set on the skill page",

		request: &BuildTestRequest{Branch: buildTestBranch, Build: makeBuild, Head: "deadbeef", Overlay: "", Repo: absentRepo, Root: t.TempDir(), Salvage: t.TempDir(), Test: ""},
	})

	// unparsable-build-line
	tests = append(tests, &testCase{
		_id:                "unparsable-build-line",
		_expectedErrorText: `RunBuildTest: commandLines: build: cmdline.SplitLines: cmdline.Split: Shell operator "&&" in "make build && rm -rf x"`,

		request: &BuildTestRequest{Branch: buildTestBranch, Build: "make build && rm -rf x", Head: "deadbeef", Overlay: "", Repo: absentRepo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := RunBuildTest(context.Background(), test.request)
			assert.Nil(t, result)
			assert.EqualError(t, err, test._expectedErrorText)
			assert.Empty(t, readTree(t, test.request.Root), "a request without commands provisions no worktree")
		})
	}
}

func TestRunBuildTest_Waived(t *testing.T) {
	red := newGitFixture(t)
	head := commitFixtureFile(red, "build:\n\t@exit 1\n\ntest:\n\t@exit 1\n", "Makefile")
	root := t.TempDir()
	request := &BuildTestRequest{Branch: buildTestBranch, Build: "", Head: head, IsWaived: true, Overlay: "", Repo: red.repo, Root: root, Salvage: t.TempDir(), Test: ""}

	result, err := RunBuildTest(context.Background(), request)

	require.NoError(t, err, "a waived run answers without build and test commands")
	assert.Equal(t, &BuildTestResult{Build: stepWaived, Tests: stepWaived}, result)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	assert.Empty(t, entries, "a waived run provisions no worktree")
	assert.Empty(t, red.run("branch", "--list", buildTestBranch))
}

func TestRunBuildTest_BranchRemoval(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedDeleted []string
		_expectedKept    []string
		_expectedRemoved bool
		_expectedSkipped []string

		fixture *gitFixture
		request *BuildTestRequest
	}

	roundBranch := buildTestBranch + "-r1"
	tests := make([]*testCase, 0)

	// sibling-branch-kept
	sibling := newGitFixture(t)
	siblingHead := commitFixtureFile(sibling, "build:\n\ttrue\n\ntest:\n\ttrue\n", "Makefile")
	sibling.branch(buildTestBranch+"-r10", siblingHead)
	tests = append(tests, &testCase{
		_id:              "sibling-branch-kept",
		_expectedDeleted: []string{roundBranch},
		_expectedKept:    []string{buildTestBranch + "-r10"},
		_expectedRemoved: true,
		_expectedSkipped: []string{},

		fixture: sibling,
		request: &BuildTestRequest{Branch: roundBranch, Build: makeBuild, Head: siblingHead, Overlay: "", Repo: sibling.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// refused-removal-reported
	locked := newGitFixture(t)
	lockedHead := commitFixtureFile(locked, "build:\n\tgit worktree lock $(CURDIR)\n\ntest:\n\ttrue\n", "Makefile")
	tests = append(tests, &testCase{
		_id:              "refused-removal-reported",
		_expectedDeleted: []string{},
		_expectedKept:    []string{roundBranch},
		_expectedRemoved: false,
		_expectedSkipped: []string{roundBranch},

		fixture: locked,
		request: &BuildTestRequest{Branch: roundBranch, Build: makeBuild, Head: lockedHead, Overlay: "", Repo: locked.repo, Root: t.TempDir(), Salvage: t.TempDir(), Test: makeTest},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := RunBuildTest(context.Background(), test.request)
			require.NoError(t, err)
			assert.Equal(t, test._expectedRemoved, result.Salvage.Removed)
			assert.Equal(t, test._expectedDeleted, result.Branches.Deleted)
			assert.Equal(t, test._expectedSkipped, branchesOf(result.Branches.Skipped))
			assert.Equal(t, test._expectedKept, strings.Fields(test.fixture.run("for-each-ref", "--format=%(refname:short)", "refs/heads/"+buildTestBranch+"*")))
		})
	}
}
