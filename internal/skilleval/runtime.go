package skilleval

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/heartbeat"
)

// runtimeFile is captured when the cell ends.
const runtimeFile = "runtime.json"

// killedState is heartbeat's run state after a check's eighth failure.
const killedState = "killed"

// ArmRuntime's Note says why Runs is empty.
type ArmRuntime struct {
	Findings []heartbeat.Finding `json:"findings"`
	Note     string              `json:"note,omitempty"`
	Runs     []heartbeat.Run     `json:"runs"`
}

// RuntimeReader is heartbeat.Cli.Runtime in production.
type RuntimeReader func(ctx context.Context) (*heartbeat.RuntimeDocument, error)

// captureRuntime records an unanswered read as a note, never an arm failure.
func captureRuntime(ctx context.Context, read RuntimeReader, sessionId string) *ArmRuntime {
	captured := &ArmRuntime{Findings: []heartbeat.Finding{}, Runs: []heartbeat.Run{}}
	if sessionId == "" {
		captured.Note = "no session id in the arm stream"
		return captured
	}

	document, err := read(ctx)
	if err != nil {
		captured.Note = "heartbeat did not answer: " + err.Error()
		return captured
	}

	for _, run := range document.Runs {
		if run.Session == sessionId {
			captured.Runs = append(captured.Runs, run)
		}
	}
	for _, finding := range document.Findings {
		if finding.Session == sessionId {
			captured.Findings = append(captured.Findings, finding)
		}
	}
	return captured
}

// loadRuntime returns nil for an arm run before the capture existed.
func loadRuntime(path string) (*ArmRuntime, error) {
	var captured ArmRuntime
	err := readJsonFile(path, &captured)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loadRuntime: %w", err)
	}
	return &captured, nil
}

func (f *armFold) runtimeValues() []evals.MetricValue {
	runId := f.arm.Id
	note := ""
	switch {
	case f.runtime == nil:
		note = "no runtime.json"
	case f.runtime.Note != "":
		note = f.runtime.Note
	case len(f.runtime.Runs) == 0:
		note = "no enforced run in the session"
	}
	if note != "" {
		return []evals.MetricValue{nullValue(metricCheckFailures, runId, note), nullValue(metricRunsKilled, runId, note)}
	}

	failures, killed := 0, 0
	for _, run := range f.runtime.Runs {
		for _, failure := range run.Failures {
			failures += failure.Count
		}
		if run.State == killedState {
			killed++
		}
	}
	return []evals.MetricValue{
		numberValue(metricCheckFailures, runId, float64(failures)),
		numberValue(metricRunsKilled, runId, float64(killed)),
	}
}
