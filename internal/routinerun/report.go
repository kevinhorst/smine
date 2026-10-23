package routinerun

import (
	"encoding/json"
	"log"
	"path/filepath"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/routines"
)

// reportWriter never fails the run on a failed write.
type reportWriter struct {
	path   string
	report *routines.RunReport
}

func newNightlyReport(runner *Runner, options *nightlyOptions, roster []routines.RunReportRepo) *reportWriter {
	now := time.Now()
	reportConfig := routines.RunReportConfig{
		Agents:  options.agents,
		DevMode: options.isDevMode,
		Model:   runner.config.Model,
		Repos:   roster,
	}
	report := &routines.RunReport{
		Changes: []routines.RunReportChange{},
		Config:  reportConfig,
		Mode:    "full",
		Routine: runner.name,
		RunId:   compactStamp(now),
		Stages:  []routines.RunReportStage{},
		Started: utcStamp(now),
		Tasks:   []routines.ChangeTask{},
	}
	writer := &reportWriter{
		path:   filepath.Join(runner.dir, "runs", report.RunId+".json"),
		report: report,
	}
	writer.save()
	return writer
}

func (w *reportWriter) save() {
	data, err := json.MarshalIndent(w.report, "", "  ")
	if err != nil {
		log.Printf("reportWriter.save: %v", err)
		return
	}
	if err := fsx.WriteFileAtomic(append(data, '\n'), w.path, 0o644); err != nil {
		log.Printf("reportWriter.save: %s: %v", w.path, err)
	}
}

func (w *reportWriter) addStage(envelope *Envelope, stage string, status int) {
	entry := routines.RunReportStage{
		ExitStatus:   status,
		IsError:      envelope.IsError,
		NumTurns:     envelope.NumTurns,
		Result:       envelope.Result,
		SessionId:    envelope.SessionId,
		Stage:        stage,
		Subtype:      envelope.Subtype,
		TotalCostUsd: envelope.TotalCostUsd,
	}
	w.report.Stages = append(w.report.Stages, entry)
	w.save()
}

// addSummaryStages leaves cost and turns on the dispatcher's own row.
func (w *reportWriter) addSummaryStages(summary *runSummary, sessionId string) {
	for _, stage := range summary.Stages {
		entry := routines.RunReportStage{
			ExitStatus: stage.exitStatus(),
			Result:     stage.Summary,
			SessionId:  sessionId,
			Stage:      stage.Stage,
		}
		w.report.Stages = append(w.report.Stages, entry)
	}
	w.save()
}

func (w *reportWriter) finalize(status int) {
	var cost float64
	for _, stage := range w.report.Stages {
		cost += stage.TotalCostUsd
	}
	w.report.Finished = utcStamp(time.Now())
	w.report.Totals = routines.RunReportTotals{CostUsd: cost, ExitStatus: status}
	w.save()
}

func (w *reportWriter) setChanges(changes []routines.RunReportChange) {
	w.report.Changes = changes
	w.save()
}

func (w *reportWriter) setOrchestrate(orchestrate *routines.RunReportOrchestrate) {
	w.report.Orchestrate = orchestrate
	w.save()
}

func (w *reportWriter) setPublish(branch, batchBranch string, publish *routines.RunReportPublish) {
	w.report.Branch = branch
	w.report.BatchBranch = batchBranch
	w.report.Publish = publish
	w.save()
}

func (w *reportWriter) setTasks(tasks []routines.ChangeTask) {
	w.report.Tasks = tasks
	w.save()
}
