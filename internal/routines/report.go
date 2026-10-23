package routines

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ChangeTasksDir is where the miners write task files inside the night worktree, never committed.
const ChangeTasksDir = ".smine-run/changes"

// ChangeEvidence is one verbatim quote of a session the task rests on; Finding is the record's finding index it was taken from.
type ChangeEvidence struct {
	Finding int    `json:"finding"`
	Quote   string `json:"quote"`
	Session string `json:"session"`
}

// ChangeTask is one miner's task file, kept in the run report so the Proposals page can show the problem after the night worktree is gone.
type ChangeTask struct {
	Id       string           `json:"id"`
	Backlog  string           `json:"backlog,omitempty"`
	Change   string           `json:"change"`
	Evidence []ChangeEvidence `json:"evidence"`
	Fix      string           `json:"fix"`
	Kind     string           `json:"kind"`
	Notes    string           `json:"notes"`
	Problem  string           `json:"problem"`
	Sessions []string         `json:"sessions"`
	Target   string           `json:"target"`
	Title    string           `json:"title"`
}

type RunReport struct {
	BatchBranch string                `json:"batch_branch"`
	Branch      string                `json:"branch"`
	Changes     []RunReportChange     `json:"changes"`
	Config      RunReportConfig       `json:"config"`
	Finished    string                `json:"finished"`
	Mode        string                `json:"mode"`
	Orchestrate *RunReportOrchestrate `json:"orchestrate"`
	Publish     *RunReportPublish     `json:"publish"`
	Routine     string                `json:"routine"`
	RunId       string                `json:"run_id"`
	Stages      []RunReportStage      `json:"stages"`
	Started     string                `json:"started"`
	Tasks       []ChangeTask          `json:"tasks"`
	Totals      RunReportTotals       `json:"totals"`
}

// RunReportChange.Id is the commit's Smine-Change trailer value.
type RunReportChange struct {
	Id      string `json:"id"`
	Reason  string `json:"reason"`
	Sha     string `json:"sha"`
	Verdict string `json:"verdict"`
}

type RunReportConfig struct {
	Agents  string          `json:"agents"`
	DevMode bool            `json:"dev_mode"`
	Model   string          `json:"model"`
	Repos   []RunReportRepo `json:"repos"`
}

type RunReportFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type RunReportOrchestrate struct {
	MainSha string `json:"main_sha"`
	Merged  bool   `json:"merged"`
	Report  string `json:"report"`
}

type RunReportPublish struct {
	BatchLanding string          `json:"batch_landing"`
	Commit       string          `json:"commit"`
	CommitBody   string          `json:"commit_body"`
	Files        []RunReportFile `json:"files"`
	Status       int             `json:"status"`
}

type RunReportRepo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type RunReportStage struct {
	ExitStatus   int     `json:"exit_status"`
	IsError      bool    `json:"is_error"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	SessionId    string  `json:"session_id"`
	Stage        string  `json:"stage"`
	Subtype      string  `json:"subtype"`
	TotalCostUsd float64 `json:"total_cost_usd"`
}

type RunReportTotals struct {
	CostUsd    float64 `json:"cost_usd"`
	ExitStatus int     `json:"exit_status"`
}

// LoadReport reads one run report; the caller owns not-found handling.
func LoadReport(path string) (*RunReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadReport: Failed to read %s: %w", path, err)
	}

	var report RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("LoadReport: Failed to parse %s: %w", path, err)
	}
	return &report, nil
}

// FindTask returns the newest report's record for a change id; nil when no report holds it.
func FindTask(reports []RunReport, id string) *ChangeTask {
	for index := range reports {
		position := slices.IndexFunc(reports[index].Tasks, func(task ChangeTask) bool { return task.Id == id })
		if position >= 0 {
			return &reports[index].Tasks[position]
		}
	}
	return nil
}

// ReadChangeTaskFile reads one task file and refuses it when it fails the task schema.
func ReadChangeTaskFile(path string) (*ChangeTask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ReadChangeTaskFile: %w", err)
	}

	schema, err := ChangeTaskSchema()
	if err != nil {
		return nil, fmt.Errorf("ReadChangeTaskFile: %w", err)
	}
	if violations := schema.ViolationsIn(path, data); len(violations) > 0 {
		return nil, fmt.Errorf("ReadChangeTaskFile: %s", strings.Join(violations, "; "))
	}

	var task ChangeTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("ReadChangeTaskFile: %s: %w", path, err)
	}
	return &task, nil
}

// ReadChangeTasks loads the task files of a night worktree in file-name order; a file that does not parse or fails the task schema is reported and skipped.
func ReadChangeTasks(worktree string) ([]ChangeTask, error) {
	paths, err := filepath.Glob(filepath.Join(worktree, ChangeTasksDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("ReadChangeTasks: %w", err)
	}

	tasks := make([]ChangeTask, 0, len(paths))
	failures := make([]error, 0)
	for _, path := range paths {
		task, err := ReadChangeTaskFile(path)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		tasks = append(tasks, *task)
	}
	return tasks, errors.Join(failures...)
}

// Reports skips malformed files so a broken report never hides the readable rest.
func Reports(routineDir string, limit int) ([]RunReport, error) {
	entries, err := os.ReadDir(filepath.Join(routineDir, "runs"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Reports: Failed to read %s: %w", routineDir, err)
	}

	var reports []RunReport
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		report, err := LoadReport(filepath.Join(routineDir, "runs", entry.Name()))
		if err != nil {
			continue
		}
		reports = append(reports, *report)
	}

	// run_id is a UTC stamp — lexicographic order is chronological.
	sort.Slice(reports, func(i, j int) bool { return reports[i].RunId > reports[j].RunId })
	if len(reports) > limit {
		reports = reports[:limit]
	}
	return reports, nil
}
