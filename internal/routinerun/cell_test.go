package routinerun

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/routines"
)

func TestRunCell(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedOutput string
		_expectedStatus int

		cancelAfter time.Duration
		mode        string
		timeout     time.Duration
	}

	tests := make([]*testCase, 0)

	// exit-code-passthrough
	tests = append(tests, &testCase{
		_id:             "exit-code-passthrough",
		_expectedOutput: "child output\n",
		_expectedStatus: 3,

		cancelAfter: 0,
		mode:        "exit:3",
		timeout:     10 * time.Second,
	})

	// deadline-124
	tests = append(tests, &testCase{
		_id:             "deadline-124",
		_expectedOutput: "",
		_expectedStatus: exitDeadline,

		cancelAfter: 0,
		mode:        "sleep",
		timeout:     200 * time.Millisecond,
	})

	// cancelled-143
	tests = append(tests, &testCase{
		_id:             "cancelled-143",
		_expectedOutput: "",
		_expectedStatus: exitCancelled,

		cancelAfter: 200 * time.Millisecond,
		mode:        "sleep",
		timeout:     10 * time.Second,
	})

	// headless-env
	tests = append(tests, &testCase{
		_id:             "headless-env",
		_expectedOutput: "1\n",
		_expectedStatus: exitOk,

		cancelAfter: 0,
		mode:        "env:SMINE_HEADLESS",
		timeout:     10 * time.Second,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(fakeChildEnv, test.mode)
			executable, err := os.Executable()
			require.NoError(t, err)
			ctx := context.Background()
			if test.cancelAfter > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.cancelAfter)
				defer cancel()
			}
			var stdout bytes.Buffer
			cellPids := filepath.Join(t.TempDir(), routines.CellPidsFileName)
			spec := &CellSpec{Args: []string{executable}, CellPids: cellPids, Stdout: &stdout, Timeout: test.timeout}

			status, err := RunCell(ctx, spec)

			require.NoError(t, err)
			assert.Equal(t, test._expectedStatus, status)
			assert.Equal(t, test._expectedOutput, stdout.String())
			assert.Len(t, routines.ReadCellPids(cellPids), 1)
		})
	}
}

func TestRunCellStartFailure(t *testing.T) {
	spec := &CellSpec{Args: []string{filepath.Join(t.TempDir(), "missing")}, Timeout: time.Second}

	_, err := RunCell(context.Background(), spec)

	assert.Error(t, err)
}
