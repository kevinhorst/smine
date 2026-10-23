package routines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reportFixture = `{
  "run_id": "20260903T004501Z",
  "routine": "smine-nightly",
  "mode": "full",
  "started": "2026-09-03T00:45:01Z",
  "config": {
    "model": "claude-opus-5-5[1m]",
    "agents": "claude,codex",
    "dev_mode": false,
    "repos": [{"name": "claude-configs", "path": "/home/u/claude-configs"}]
  },
  "stages": [
    {
      "stage": "smine",
      "exit_status": 0,
      "session_id": "e8ca17f7",
      "num_turns": 288,
      "total_cost_usd": 9.51,
      "subtype": "success",
      "is_error": false,
      "result": "mined 2 batches"
    }
  ],
  "branch": "claude-routines/smine-nightly-2026-09-03",
  "publish": {
    "status": 0,
    "commit": "abc1234",
    "files": [{"status": "A", "path": "sessions/claude-configs/sessions-batch-60.md"}],
    "commit_body": "disposition table"
  },
  "changes": [
    {
      "id": "context/fact-x",
      "sha": "def5678",
      "verdict": "rejected",
      "reason": "audit red"
    }
  ],
  "orchestrate": {"merged": true, "main_sha": "abc1234", "report": ""},
  "finished": "2026-09-03T02:09:38Z",
  "totals": {"cost_usd": 14.06, "exit_status": 0}
}`

// provideTask is a task that passes the schema; a case sets the one field it breaks.
func provideTask(id, kind string) *ChangeTask {
	evidence := []ChangeEvidence{{Finding: 1, Quote: "q", Session: "s1"}}
	task := &ChangeTask{
		Id:       id,
		Change:   "Do it.",
		Evidence: evidence,
		Fix:      "Because.",
		Kind:     kind,
		Notes:    "",
		Problem:  "It hurt.",
		Sessions: []string{"s1"},
		Target:   "x.md",
		Title:    "T",
	}
	return task
}

func taskIds(tasks []ChangeTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.Id)
	}
	return ids
}

func writeReport(t *testing.T, runsDir, runId, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, runId+".json"), []byte(content), 0o644))
}

// writeTask writes one task file; a raw string stands in only where its text is the input under test.
func writeTask(t *testing.T, worktree, name string, task *ChangeTask) {
	t.Helper()
	encoded, err := json.Marshal(task)
	require.NoError(t, err)
	writeTaskText(t, worktree, name, string(encoded))
}

func writeTaskText(t *testing.T, worktree, name, content string) {
	t.Helper()
	dir := filepath.Join(worktree, ChangeTasksDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestFindTask(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *ChangeTask

		id      string
		reports []RunReport
	}

	reports := []RunReport{
		{RunId: "20260902T010000Z", Tasks: []ChangeTask{*provideTask("context/a", "context")}},
		{RunId: "20260901T010000Z", Tasks: []ChangeTask{*provideTask("skills/b", "skills")}},
	}
	tests := make([]*testCase, 0)

	// found-in-newest
	tests = append(tests, &testCase{
		_id:       "found-in-newest",
		_expected: provideTask("skills/b", "skills"),

		id:      "skills/b",
		reports: reports,
	})

	// absent
	tests = append(tests, &testCase{
		_id:       "absent",
		_expected: nil,

		id:      "skills/unknown",
		reports: reports,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, FindTask(test.reports, test.id))
		})
	}
}

func TestLoadReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(path, []byte(reportFixture), 0o644))

	report, err := LoadReport(path)
	require.NoError(t, err)
	assert.Equal(t, "20260903T004501Z", report.RunId)
	assert.Equal(t, "full", report.Mode)
	assert.Equal(t, "claude-routines/smine-nightly-2026-09-03", report.Branch)
	assert.Equal(t, "claude,codex", report.Config.Agents)
	require.Len(t, report.Config.Repos, 1)
	assert.Equal(t, "claude-configs", report.Config.Repos[0].Name)
	require.Len(t, report.Stages, 1)
	assert.Equal(t, "smine", report.Stages[0].Stage)
	assert.Equal(t, 288, report.Stages[0].NumTurns)
	assert.Equal(t, "mined 2 batches", report.Stages[0].Result)
	require.NotNil(t, report.Publish)
	assert.Equal(t, "abc1234", report.Publish.Commit)
	require.Len(t, report.Publish.Files, 1)
	assert.Equal(t, "A", report.Publish.Files[0].Status)
	expectedChanges := []RunReportChange{{Id: "context/fact-x", Reason: "audit red", Sha: "def5678", Verdict: "rejected"}}
	assert.Equal(t, expectedChanges, report.Changes)
	require.NotNil(t, report.Orchestrate)
	assert.True(t, report.Orchestrate.Merged)
	assert.InDelta(t, 14.06, report.Totals.CostUsd, 0.001)
}

func TestLoadReportMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

	_, err := LoadReport(path)
	assert.Error(t, err)
}

func TestLoadReportMissing(t *testing.T) {
	_, err := LoadReport(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)
}

func TestReadChangeTaskFile(t *testing.T) {
	worktree := t.TempDir()
	task := provideTask("skills/backlog", "skills")
	task.Backlog = "B-022"
	writeTask(t, worktree, "skills-backlog.json", task)

	decoded, err := ReadChangeTaskFile(filepath.Join(worktree, ChangeTasksDir, "skills-backlog.json"))
	require.NoError(t, err)
	assert.Equal(t, task, decoded)
}

func TestReadChangeTasks(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedErrorFile string
		_expectedIds       []string
		_shouldPass        bool

		worktree string
	}

	tests := make([]*testCase, 0)

	// two-files-sorted
	sortedWorktree := t.TempDir()
	writeTask(t, sortedWorktree, "skills-b.json", provideTask("skills/b", "skills"))
	writeTask(t, sortedWorktree, "context-a.json", provideTask("context/a", "context"))
	tests = append(tests, &testCase{
		_id:                "two-files-sorted",
		_expectedErrorFile: "",
		_expectedIds:       []string{"context/a", "skills/b"},
		_shouldPass:        true,

		worktree: sortedWorktree,
	})

	// malformed-skipped
	malformedWorktree := t.TempDir()
	writeTask(t, malformedWorktree, "context-a.json", provideTask("context/a", "context"))
	writeTaskText(t, malformedWorktree, "context-broken.json", `{not json`)
	tests = append(tests, &testCase{
		_id:                "malformed-skipped",
		_expectedErrorFile: "context-broken.json",
		_expectedIds:       []string{"context/a"},
		_shouldPass:        false,

		worktree: malformedWorktree,
	})

	// over-cap-skipped
	overCapWorktree := t.TempDir()
	writeTask(t, overCapWorktree, "context-a.json", provideTask("context/a", "context"))
	long := provideTask("skills/long", "skills")
	long.Problem = strings.Repeat("p", 161)
	writeTask(t, overCapWorktree, "skills-long.json", long)
	tests = append(tests, &testCase{
		_id:                "over-cap-skipped",
		_expectedErrorFile: "skills-long.json",
		_expectedIds:       []string{"context/a"},
		_shouldPass:        false,

		worktree: overCapWorktree,
	})

	// missing-field-skipped
	missingWorktree := t.TempDir()
	writeTaskText(t, missingWorktree, "context-a.json", `{"id": "context/a"}`)
	tests = append(tests, &testCase{
		_id:                "missing-field-skipped",
		_expectedErrorFile: "context-a.json",
		_expectedIds:       []string{},
		_shouldPass:        false,

		worktree: missingWorktree,
	})

	// missing-dir
	tests = append(tests, &testCase{
		_id:                "missing-dir",
		_expectedErrorFile: "",
		_expectedIds:       []string{},
		_shouldPass:        true,

		worktree: t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			tasks, err := ReadChangeTasks(test.worktree)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedErrorFile)
			assert.Equal(t, test._expectedIds, taskIds(tasks))
		})
	}
}

func TestReports(t *testing.T) {
	routineDir := t.TempDir()
	runsDir := filepath.Join(routineDir, "runs")
	writeReport(t, runsDir, "20260901T010000Z", `{"run_id": "20260901T010000Z", "stages": []}`)
	writeReport(t, runsDir, "20260903T010000Z", `{"run_id": "20260903T010000Z", "stages": []}`)
	writeReport(t, runsDir, "20260902T010000Z", `{not json`)

	reports, err := Reports(routineDir, 10)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	assert.Equal(t, "20260903T010000Z", reports[0].RunId)
	assert.Equal(t, "20260901T010000Z", reports[1].RunId)
}

func TestReportsLimit(t *testing.T) {
	routineDir := t.TempDir()
	runsDir := filepath.Join(routineDir, "runs")
	writeReport(t, runsDir, "20260901T010000Z", `{"run_id": "20260901T010000Z"}`)
	writeReport(t, runsDir, "20260902T010000Z", `{"run_id": "20260902T010000Z"}`)

	reports, err := Reports(routineDir, 1)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.Equal(t, "20260902T010000Z", reports[0].RunId)
}

func TestReportsMissingDir(t *testing.T) {
	reports, err := Reports(t.TempDir(), 10)
	require.NoError(t, err)
	assert.Nil(t, reports)
}
