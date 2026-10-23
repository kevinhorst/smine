package repos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOwnerOf(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		path   string
		roster []Repo
	}

	root := t.TempDir()
	mainCheckout := filepath.Join(root, "src", "configs")
	nested := filepath.Join(root, "src", "configs", "tools")
	poolWorktree := filepath.Join(root, "pool", "wt")
	removedWorktree := filepath.Join(mainCheckout, ".claude", "worktrees", "gone")
	require.NoError(t, os.MkdirAll(filepath.Join(mainCheckout, ".git", "worktrees", "wt"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainCheckout, ".claude", "worktrees", "inner"), 0o755))
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.MkdirAll(poolWorktree, 0o755))
	pointer := "gitdir: " + filepath.Join(mainCheckout, ".git", "worktrees", "wt") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(poolWorktree, ".git"), []byte(pointer), 0o644))
	roster := []Repo{
		{Name: "configs", Path: mainCheckout},
		{Name: "tools", Path: nested},
	}

	tests := make([]*testCase, 0)

	// under-registered-repo
	tests = append(tests, &testCase{
		_id:       "under-registered-repo",
		_expected: "configs",

		path:   filepath.Join(mainCheckout, "cmd"),
		roster: roster,
	})

	// worktree-under-repo
	tests = append(tests, &testCase{
		_id:       "worktree-under-repo",
		_expected: "configs",

		path:   filepath.Join(mainCheckout, ".claude", "worktrees", "inner"),
		roster: roster,
	})

	// worktree-in-external-pool
	tests = append(tests, &testCase{
		_id:       "worktree-in-external-pool",
		_expected: "configs",

		path:   poolWorktree,
		roster: roster,
	})

	// removed-worktree-falls-to-prefix
	tests = append(tests, &testCase{
		_id:       "removed-worktree-falls-to-prefix",
		_expected: "configs",

		path:   removedWorktree,
		roster: roster,
	})

	// longest-prefix-wins
	tests = append(tests, &testCase{
		_id:       "longest-prefix-wins",
		_expected: "tools",

		path:   filepath.Join(nested, "cmd"),
		roster: roster,
	})

	// unregistered
	tests = append(tests, &testCase{
		_id:       "unregistered",
		_expected: "",

		path:   filepath.Join(root, "elsewhere"),
		roster: roster,
	})

	// empty-path
	tests = append(tests, &testCase{
		_id:       "empty-path",
		_expected: "",

		path:   "",
		roster: roster,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, OwnerOf(test.path, test.roster))
		})
	}
}

func TestIsUnder(t *testing.T) {
	assert.True(t, IsUnder("/src/go", "/src/go"))
	assert.True(t, IsUnder("/src/go/tools", "/src/go/"))
	assert.False(t, IsUnder("/src/gopher", "/src/go"))
	assert.False(t, IsUnder("", "/src/go"))
	assert.False(t, IsUnder("/src/go", ""))
}
