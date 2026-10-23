package routinerun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/routines"
)

func TestLoadRunSummary(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedAccepted bool
		_expectedChanges  []routines.RunReportChange
		_expectedReason   string
		_expectedStages   int

		body string
	}

	tests := make([]*testCase, 0)

	// valid-full
	tests = append(tests, &testCase{
		_id:               "valid-full",
		_expectedAccepted: true,
		_expectedChanges:  []routines.RunReportChange{{Id: "context/fact-x", Reason: "gates green", Sha: "abc1234", Verdict: "ready"}},
		_expectedReason:   "audit green",
		_expectedStages:   10,

		body: acceptedSummary,
	})

	// missing-verdict-defaults-rejected
	tests = append(tests, &testCase{
		_id:               "missing-verdict-defaults-rejected",
		_expectedAccepted: false,
		_expectedChanges:  []routines.RunReportChange{},
		_expectedReason:   "judge returned no verdict",
		_expectedStages:   1,

		body: `{"stages":[{"stage":"batch","status":"skipped","summary":"nothing pending"}]}`,
	})

	// old-summary-with-carried-key-loads
	tests = append(tests, &testCase{
		_id:               "old-summary-with-carried-key-loads",
		_expectedAccepted: true,
		_expectedChanges:  []routines.RunReportChange{},
		_expectedReason:   "audit green",
		_expectedStages:   0,

		body: `{"stages":[],"carried":[` +
			`{"sha":"abc1234","outcome":"carried","reason":""},` +
			`{"sha":"def5678","outcome":"maybe","reason":"changelog head unresolvable"}],` +
			`"verdict":{"accepted":true,"reason":"audit green"}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), runSummaryFileName)
			require.NoError(t, os.WriteFile(path, []byte(test.body), 0o644))

			summary, err := loadRunSummary(path)

			require.NoError(t, err)
			assert.Equal(t, test._expectedAccepted, summary.Verdict.Accepted)
			assert.Equal(t, test._expectedReason, summary.Verdict.Reason)
			assert.Len(t, summary.Stages, test._expectedStages)
			assert.Equal(t, test._expectedChanges, summary.reportChanges())
		})
	}
}

func TestLoadRunSummaryErrors(t *testing.T) {
	type testCase struct {
		_id string

		body      string
		isWritten bool
	}

	tests := make([]*testCase, 0)

	// unknown-status
	tests = append(tests, &testCase{
		_id: "unknown-status",

		body:      `{"stages":[{"stage":"batch","status":"weird","summary":""}]}`,
		isWritten: true,
	})

	// unknown-change-verdict
	tests = append(tests, &testCase{
		_id: "unknown-change-verdict",

		body:      `{"stages":[],"changes":[{"id":"context/x","sha":"abc","verdict":"maybe","reason":""}]}`,
		isWritten: true,
	})

	// invalid-json
	tests = append(tests, &testCase{
		_id: "invalid-json",

		body:      `{"stages":`,
		isWritten: true,
	})

	// missing-file
	tests = append(tests, &testCase{
		_id: "missing-file",

		body:      "",
		isWritten: false,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), runSummaryFileName)
			writeSummaryFixture(t, path, test.body, test.isWritten)

			summary, err := loadRunSummary(path)

			require.Error(t, err)
			assert.Nil(t, summary)
		})
	}
}

func TestRunSummaryStageExitStatus(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		status runStageStatus
	}

	tests := make([]*testCase, 0)

	// ok-zero
	tests = append(tests, &testCase{
		_id:       "ok-zero",
		_expected: exitOk,

		status: stageOk,
	})

	// skipped-zero
	tests = append(tests, &testCase{
		_id:       "skipped-zero",
		_expected: exitOk,

		status: stageSkipped,
	})

	// failed-software
	tests = append(tests, &testCase{
		_id:       "failed-software",
		_expected: exitSoftware,

		status: stageFailed,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stage := &runSummaryStage{Stage: "batch", Status: test.status}

			assert.Equal(t, test._expected, stage.exitStatus())
		})
	}
}

func writeSummaryFixture(t *testing.T, path, body string, isWritten bool) {
	t.Helper()
	if !isWritten {
		return
	}
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
