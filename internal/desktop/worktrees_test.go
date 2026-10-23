package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadWorktree(t *testing.T) {
	type testCase struct {
		_id           string
		_expected     *Worktree
		_isNoWorktree bool
		_shouldPass   bool

		name      string
		storePath string
	}

	dir := t.TempDir()
	writeStore := func(fileName string, content string) string {
		path := filepath.Join(dir, fileName)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	fullStore := writeStore("full.json", `{
  "schemaVersion": 3,
  "worktrees": {
    "feature-a1b2c3": {
      "name": "feature-a1b2c3",
      "path": "/repo/.claude/worktrees/feature-a1b2c3",
      "leasedBy": "local_0001",
      "baseRepo": "/repo",
      "branch": "feature-a1b2c3",
      "sourceBranch": "main",
      "createdAt": 1790980578061,
      "hookBased": true
    },
    "pooled-d4e5f6": {
      "name": "pooled-d4e5f6",
      "path": "/repo/.claude/worktrees/pooled-d4e5f6",
      "leasedBy": null,
      "branch": "claude/pooled-d4e5f6"
    }
  }
}`)
	badStore := writeStore("bad.json", "{")

	tests := make([]*testCase, 0)

	// pass-entry
	expectedEntry := &Worktree{
		HookBased:    true,
		LeasedBy:     "local_0001",
		Name:         "feature-a1b2c3",
		Path:         "/repo/.claude/worktrees/feature-a1b2c3",
		SourceBranch: "main",
	}
	tests = append(tests, &testCase{
		_id:           "pass-entry",
		_expected:     expectedEntry,
		_isNoWorktree: false,
		_shouldPass:   true,

		name:      "feature-a1b2c3",
		storePath: fullStore,
	})

	// pass-no-source-branch
	expectedPooled := &Worktree{
		HookBased:    false,
		LeasedBy:     "",
		Name:         "pooled-d4e5f6",
		Path:         "/repo/.claude/worktrees/pooled-d4e5f6",
		SourceBranch: "",
	}
	tests = append(tests, &testCase{
		_id:           "pass-no-source-branch",
		_expected:     expectedPooled,
		_isNoWorktree: false,
		_shouldPass:   true,

		name:      "pooled-d4e5f6",
		storePath: fullStore,
	})

	// fail-unknown-name
	tests = append(tests, &testCase{
		_id:           "fail-unknown-name",
		_expected:     nil,
		_isNoWorktree: true,
		_shouldPass:   false,

		name:      "nope",
		storePath: fullStore,
	})

	// fail-missing-file
	tests = append(tests, &testCase{
		_id:           "fail-missing-file",
		_expected:     nil,
		_isNoWorktree: false,
		_shouldPass:   false,

		name:      "feature-a1b2c3",
		storePath: filepath.Join(dir, "absent.json"),
	})

	// fail-bad-json
	tests = append(tests, &testCase{
		_id:           "fail-bad-json",
		_expected:     nil,
		_isNoWorktree: false,
		_shouldPass:   false,

		name:      "feature-a1b2c3",
		storePath: badStore,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			worktree, err := ReadWorktree(test.storePath, test.name)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, worktree)
			assert.Equal(t, test._isNoWorktree, errors.Is(err, ErrNoWorktree))
		})
	}
}

func TestWorktreeStorePath(t *testing.T) {
	assert.Equal(t, filepath.Join("/cfg", "Claude", "git-worktrees.json"), WorktreeStorePath("/cfg"))
}
