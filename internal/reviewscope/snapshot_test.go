package reviewscope

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshot(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedDiffers bool
		_expectedFile    string

		modify func(repo string)
		scope  string
	}

	tests := make([]*testCase, 0)

	// clean-tree
	tests = append(tests, &testCase{
		_id:              "clean-tree",
		_expectedDiffers: false,
		_expectedFile:    "",

		modify: func(string) {},
		scope:  SnapshotScopeWip,
	})

	// wip-tracked-change
	tests = append(tests, &testCase{
		_id:              "wip-tracked-change",
		_expectedDiffers: true,
		_expectedFile:    "base",

		modify: func(repo string) {
			require.NoError(t, os.WriteFile(filepath.Join(repo, "base"), []byte("changed\n"), 0o644))
		},
		scope: SnapshotScopeWip,
	})

	// wip-untracked-file
	tests = append(tests, &testCase{
		_id:              "wip-untracked-file",
		_expectedDiffers: true,
		_expectedFile:    "fresh",

		modify: func(repo string) {
			require.NoError(t, os.WriteFile(filepath.Join(repo, "fresh"), []byte("fresh\n"), 0o644))
		},
		scope: SnapshotScopeWip,
	})

	// all-untracked-file
	tests = append(tests, &testCase{
		_id:              "all-untracked-file",
		_expectedDiffers: true,
		_expectedFile:    "fresh",

		modify: func(repo string) {
			require.NoError(t, os.WriteFile(filepath.Join(repo, "fresh"), []byte("fresh\n"), 0o644))
		},
		scope: SnapshotScopeAll,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			fixture := newGitFixture(t)
			test.modify(fixture.repo)
			statusBefore := fixture.run("status", "--porcelain")

			result, err := Snapshot(context.Background(), fixture.repo, test.scope)

			require.NoError(t, err)
			assert.Equal(t, test._expectedDiffers, result.Differs)
			assert.Equal(t, fixture.run("rev-parse", "HEAD"), fixture.run("rev-parse", result.Snapshot+"^"))
			assert.Equal(t, statusBefore, fixture.run("status", "--porcelain"), "the index and tree are untouched")
			if test._expectedFile != "" {
				assert.Contains(t, fixture.run("diff", "--name-only", "HEAD", result.Snapshot), test._expectedFile)
			}
		})
	}
}
