package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLinkedWorktree(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		cwd string
	}

	main, worktree, outside := provideCheckoutFixture(t)

	tests := make([]*testCase, 0)

	// main-checkout
	tests = append(tests, &testCase{
		_id:       "main-checkout",
		_expected: false,

		cwd: main,
	})

	// linked-worktree
	tests = append(tests, &testCase{
		_id:       "linked-worktree",
		_expected: true,

		cwd: worktree,
	})

	// nested-cwd-in-worktree
	tests = append(tests, &testCase{
		_id:       "nested-cwd-in-worktree",
		_expected: true,

		cwd: filepath.Join(worktree, "cmd", "x"),
	})

	// outside-git
	tests = append(tests, &testCase{
		_id:       "outside-git",
		_expected: false,

		cwd: outside,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, isLinkedWorktree(test.cwd))
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
