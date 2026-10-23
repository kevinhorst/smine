package skilleval

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/heartbeat"
)

func TestCaptureRuntime(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *ArmRuntime

		read      RuntimeReader
		sessionId string
	}

	document := &heartbeat.RuntimeDocument{
		Findings: []heartbeat.Finding{{Id: "r1-value_provenance-1", Session: "s-1"}, {Id: "r2-value_provenance-1", Session: "other"}},
		Runs:     []heartbeat.Run{{Id: 1, Session: "s-1", State: "open"}, {Id: 2, Session: "other", State: "killed"}},
	}
	answering := func(context.Context) (*heartbeat.RuntimeDocument, error) { return document, nil }
	failing := func(context.Context) (*heartbeat.RuntimeDocument, error) { return nil, errors.New("unreachable") }

	tests := make([]*testCase, 0)

	// keeps-session-runs-and-findings
	tests = append(tests, &testCase{
		_id: "keeps-session-runs-and-findings",
		_expected: &ArmRuntime{
			Findings: []heartbeat.Finding{{Id: "r1-value_provenance-1", Session: "s-1"}},
			Runs:     []heartbeat.Run{{Id: 1, Session: "s-1", State: "open"}},
		},

		read:      answering,
		sessionId: "s-1",
	})

	// reader-error-note
	tests = append(tests, &testCase{
		_id:       "reader-error-note",
		_expected: &ArmRuntime{Findings: []heartbeat.Finding{}, Note: "heartbeat did not answer: unreachable", Runs: []heartbeat.Run{}},

		read:      failing,
		sessionId: "s-1",
	})

	// no-session-id-note
	tests = append(tests, &testCase{
		_id:       "no-session-id-note",
		_expected: &ArmRuntime{Findings: []heartbeat.Finding{}, Note: "no session id in the arm stream", Runs: []heartbeat.Run{}},

		read:      answering,
		sessionId: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, captureRuntime(context.Background(), test.read, test.sessionId))
		})
	}
}

func TestRuntimeValues(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []evals.MetricValue

		runtime *ArmRuntime
	}

	const runId = "sonnet-high-r1"
	killedRuns := []heartbeat.Run{
		{Failures: []heartbeat.CheckFailure{{Check: "value_provenance", Count: 1}}, State: "closed"},
		{Failures: []heartbeat.CheckFailure{{Check: "value_provenance", Count: 2}}, State: killedState},
	}

	tests := make([]*testCase, 0)

	// failures-summed-one-killed
	tests = append(tests, &testCase{
		_id:       "failures-summed-one-killed",
		_expected: []evals.MetricValue{numberValue(metricCheckFailures, runId, 3), numberValue(metricRunsKilled, runId, 1)},

		runtime: &ArmRuntime{Runs: killedRuns},
	})

	// no-runtime-json
	tests = append(tests, &testCase{
		_id:       "no-runtime-json",
		_expected: []evals.MetricValue{nullValue(metricCheckFailures, runId, "no runtime.json"), nullValue(metricRunsKilled, runId, "no runtime.json")},

		runtime: nil,
	})

	// capture-note
	note := "heartbeat did not answer: unreachable"
	tests = append(tests, &testCase{
		_id:       "capture-note",
		_expected: []evals.MetricValue{nullValue(metricCheckFailures, runId, note), nullValue(metricRunsKilled, runId, note)},

		runtime: &ArmRuntime{Note: note},
	})

	// no-enforced-run
	empty := "no enforced run in the session"
	tests = append(tests, &testCase{
		_id:       "no-enforced-run",
		_expected: []evals.MetricValue{nullValue(metricCheckFailures, runId, empty), nullValue(metricRunsKilled, runId, empty)},

		runtime: &ArmRuntime{Runs: []heartbeat.Run{}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			fold := &armFold{arm: &ManifestArm{Id: runId}, runtime: test.runtime}
			assert.Equal(t, test._expected, fold.runtimeValues())
		})
	}
}
