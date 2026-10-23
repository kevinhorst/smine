package routinerun

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/routines"
)

type runStageStatus string

const (
	stageFailed  runStageStatus = "failed"
	stageOk      runStageStatus = "ok"
	stageSkipped runStageStatus = "skipped"
)

const (
	judgeStageName = "judge"
	styleStageName = "style"
)

// runChangeVerdict is the judge's call on one change commit.
type runChangeVerdict string

const (
	changeReady    runChangeVerdict = "ready"
	changeRejected runChangeVerdict = "rejected"
)

type runSummary struct {
	Changes []runSummaryChange `json:"changes"`
	Stages  []runSummaryStage  `json:"stages"`
	Verdict *runVerdict        `json:"verdict"`
}

func newRejectedSummary(reason string) *runSummary {
	verdict := &runVerdict{Accepted: false, Reason: reason}
	summary := &runSummary{Stages: []runSummaryStage{}, Verdict: verdict}
	return summary
}

// failedStages names the stages the summary reports failed, in run order.
func (s *runSummary) failedStages() []string {
	names := []string{}
	for _, stage := range s.Stages {
		if stage.Status == stageFailed {
			names = append(names, stage.Stage)
		}
	}
	return names
}

func (s *runSummary) isStageOk(name string) bool {
	for _, stage := range s.Stages {
		if stage.Stage == name && stage.Status == stageOk {
			return true
		}
	}
	return false
}

// reportChanges renders the judged changes as run-report rows.
func (s *runSummary) reportChanges() []routines.RunReportChange {
	changes := make([]routines.RunReportChange, 0, len(s.Changes))
	for _, change := range s.Changes {
		row := routines.RunReportChange{
			Id:      change.Id,
			Reason:  change.Reason,
			Sha:     change.Sha,
			Verdict: string(change.Verdict),
		}
		changes = append(changes, row)
	}
	return changes
}

// runSummaryChange.Id is the commit's Smine-Change trailer value.
type runSummaryChange struct {
	Id      string           `json:"id"`
	Reason  string           `json:"reason"`
	Sha     string           `json:"sha"`
	Verdict runChangeVerdict `json:"verdict"`
}

type runSummaryStage struct {
	Stage   string         `json:"stage"`
	Status  runStageStatus `json:"status"`
	Summary string         `json:"summary"`
}

func (s *runSummaryStage) exitStatus() int {
	switch s.Status {
	case stageOk, stageSkipped:
		return exitOk
	case stageFailed:
		return exitSoftware
	}
	return exitSoftware
}

type runVerdict struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason"`
}

// loadRunSummary rejects a summary without a verdict, so landing never runs on silence.
func loadRunSummary(path string) (*runSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadRunSummary: %w", err)
	}

	summary := &runSummary{}
	if err := json.Unmarshal(data, summary); err != nil {
		return nil, fmt.Errorf("loadRunSummary: %s: %w", path, err)
	}

	for _, stage := range summary.Stages {
		switch stage.Status {
		case stageOk, stageFailed, stageSkipped:
			continue
		}
		return nil, fmt.Errorf("loadRunSummary: Stage %s: Unknown status %q", stage.Stage, stage.Status)
	}

	for _, change := range summary.Changes {
		switch change.Verdict {
		case changeReady, changeRejected:
			continue
		}
		return nil, fmt.Errorf("loadRunSummary: Change %s: Unknown verdict %q", change.Id, change.Verdict)
	}

	if summary.Verdict == nil {
		summary.Verdict = &runVerdict{Accepted: false, Reason: "judge returned no verdict"}
	}
	return summary, nil
}
