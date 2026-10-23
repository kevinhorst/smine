package gitroot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckoutRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		cwd string
	}

	main, worktree, outside := provideCheckoutFixture(t)

	tests := make([]*testCase, 0)

	// main-checkout
	tests = append(tests, &testCase{
		_id:       "main-checkout",
		_expected: main,

		cwd: main,
	})

	// linked-worktree
	tests = append(tests, &testCase{
		_id:       "linked-worktree",
		_expected: worktree,

		cwd: worktree,
	})

	// nested-cwd-in-worktree
	tests = append(tests, &testCase{
		_id:       "nested-cwd-in-worktree",
		_expected: worktree,

		cwd: filepath.Join(worktree, "cmd", "x"),
	})

	// outside-git
	tests = append(tests, &testCase{
		_id:       "outside-git",
		_expected: outside,

		cwd: outside,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, CheckoutRoot(test.cwd))
		})
	}
}

func TestMainRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		cwd string
	}

	base := t.TempDir()
	main := filepath.Join(base, "main")
	worktree := filepath.Join(main, ".claude", "worktrees", "work")
	submodule := filepath.Join(base, "super", "sub")
	outside := filepath.Join(base, "plain", "dir")
	for _, dir := range []string{filepath.Join(main, ".git", "worktrees", "work"), filepath.Join(worktree, "cmd", "x"), submodule, outside} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	pointer := "gitdir: " + filepath.Join(main, ".git", "worktrees", "work") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o644))
	submodulePointer := "gitdir: " + filepath.Join(base, "super", ".git", "modules", "sub") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(submodule, ".git"), []byte(submodulePointer), 0o644))

	tests := make([]*testCase, 0)

	// main-checkout-dir
	tests = append(tests, &testCase{
		_id:       "main-checkout-dir",
		_expected: main,

		cwd: main,
	})

	// linked-worktree-file
	tests = append(tests, &testCase{
		_id:       "linked-worktree-file",
		_expected: main,

		cwd: worktree,
	})

	// nested-cwd-in-worktree
	tests = append(tests, &testCase{
		_id:       "nested-cwd-in-worktree",
		_expected: main,

		cwd: filepath.Join(worktree, "cmd", "x"),
	})

	// pointer-without-worktrees
	tests = append(tests, &testCase{
		_id:       "pointer-without-worktrees",
		_expected: submodule,

		cwd: submodule,
	})

	// outside-git
	tests = append(tests, &testCase{
		_id:       "outside-git",
		_expected: outside,

		cwd: outside,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, MainRoot(test.cwd))
		})
	}
}

// provideCheckoutFixture builds a main checkout with a linked worktree under
// .claude/worktrees/work (with a nested cmd/x dir) and a dir outside git.
func provideCheckoutFixture(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	main := filepath.Join(base, "main")
	worktree := filepath.Join(main, ".claude", "worktrees", "work")
	outside := filepath.Join(base, "plain", "dir")
	for _, dir := range []string{filepath.Join(main, ".git", "worktrees", "work"), filepath.Join(worktree, "cmd", "x"), outside} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	pointer := "gitdir: " + filepath.Join(main, ".git", "worktrees", "work") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o644))
	return main, worktree, outside
}
