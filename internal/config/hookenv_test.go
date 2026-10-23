package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeHomeDir(t *testing.T) {
	// override-wins
	home, err := ClaudeHomeDir("/tmp/claude-home")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/claude-home", home)

	// default-under-user-home
	userHome, err := os.UserHomeDir()
	require.NoError(t, err)

	home, err = ClaudeHomeDir("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(userHome, ".claude"), home)
}

func TestProtectedRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		path           string
		protectedPaths string
	}

	tests := make([]*testCase, 0)

	// under-a-root
	tests = append(tests, &testCase{
		_id:       "under-a-root",
		_expected: "/target",

		path:           "/target/internal/file.go",
		protectedPaths: "/target",
	})

	// the-root-itself
	tests = append(tests, &testCase{
		_id:       "the-root-itself",
		_expected: "/target",

		path:           "/target",
		protectedPaths: "/target",
	})

	// pool-worktree-under-a-root
	tests = append(tests, &testCase{
		_id:       "pool-worktree-under-a-root",
		_expected: "",

		path:           "/target/.claude/worktrees/x/file",
		protectedPaths: "/target",
	})

	// sibling-named-dir
	tests = append(tests, &testCase{
		_id:       "sibling-named-dir",
		_expected: "",

		path:           "/targetfoo/x",
		protectedPaths: "/target",
	})

	// under-the-second-root
	tests = append(tests, &testCase{
		_id:       "under-the-second-root",
		_expected: "/second",

		path:           "/second/file",
		protectedPaths: "/first:/second",
	})

	// empty-list
	tests = append(tests, &testCase{
		_id:       "empty-list",
		_expected: "",

		path:           "/target/file",
		protectedPaths: "",
	})

	// empty-entries
	tests = append(tests, &testCase{
		_id:       "empty-entries",
		_expected: "/target",

		path:           "/target/file",
		protectedPaths: "::/target:",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ProtectedRoot(test.path, test.protectedPaths, "/.claude/worktrees/"))
		})
	}
}
