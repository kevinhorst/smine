package routines

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCellPids(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []int

		content string
	}

	tests := make([]*testCase, 0)

	// valid-pids
	tests = append(tests, &testCase{
		_id:       "valid-pids",
		_expected: []int{73326, 73412},

		content: "73326\n73412\n",
	})

	// blank-and-garbage-lines-skipped
	tests = append(tests, &testCase{
		_id:       "blank-and-garbage-lines-skipped",
		_expected: []int{100},

		content: "\n100\nnot-a-pid\n-5\n0\n",
	})

	// pid-one-skipped
	tests = append(tests, &testCase{
		_id:       "pid-one-skipped",
		_expected: []int{42},

		content: "1\n42\n",
	})

	// empty-file
	tests = append(tests, &testCase{
		_id:       "empty-file",
		_expected: nil,

		content: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), CellPidsFileName)
			require.NoError(t, os.WriteFile(path, []byte(test.content), 0o644))

			assert.Equal(t, test._expected, ReadCellPids(path))
		})
	}
}

func TestReadCellPidsMissingFile(t *testing.T) {
	assert.Nil(t, ReadCellPids(filepath.Join(t.TempDir(), "absent")))
	assert.Nil(t, ReadCellPids(""))
}

func TestGroupWorktree(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		env map[string]string
	}

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := make([]*testCase, 0)

	// default-root-own-name
	tests = append(tests, &testCase{
		_id:       "default-root-own-name",
		_expected: filepath.Join(home, ".cache", "claude-routine", "worktrees", "demo"),

		env: map[string]string{},
	})

	// wt-root-override
	tests = append(tests, &testCase{
		_id:       "wt-root-override",
		_expected: filepath.Join("/tmp", "wt", "demo"),

		env: map[string]string{"ROUTINE_WT_ROOT": "/tmp/wt/"},
	})

	// group-override
	tests = append(tests, &testCase{
		_id:       "group-override",
		_expected: filepath.Join("/tmp", "wt", "chain"),

		env: map[string]string{"ROUTINE_GROUP": "chain", "ROUTINE_WT_ROOT": "/tmp/wt"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path, err := GroupWorktree(test.env, "demo")

			require.NoError(t, err)
			assert.Equal(t, test._expected, path)
		})
	}
}

func TestSteerDir(t *testing.T) {
	path, err := SteerDir(map[string]string{"ROUTINE_STEER_ROOT": "/tmp/steer"}, "demo")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/tmp", "steer", "demo"), path)
}
