package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/routines"
)

func provideTask() *routines.ChangeTask {
	evidence := []routines.ChangeEvidence{{Finding: 1, Quote: "the check failed on the first run", Session: "s1"}}
	task := &routines.ChangeTask{
		Id:       "skills/smoke-before-deploy",
		Change:   "Add a verify step.",
		Evidence: evidence,
		Fix:      "A smoke run catches it at edit time.",
		Kind:     "skills",
		Notes:    "",
		Problem:  "Skill edits were deployed without a run.",
		Sessions: []string{"s1"},
		Target:   "skills/implement/fimplement/fimplement.cue",
		Title:    "Run the smoke before deploy",
	}
	return task
}

func provideTaskFile(t *testing.T, dir, name string, task *routines.ChangeTask) string {
	t.Helper()
	encoded, err := json.Marshal(task)
	require.NoError(t, err)
	return provideTaskText(t, dir, name, string(encoded))
}

func provideTaskText(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRunTaskCheck(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		args []string
	}

	dir := t.TempDir()
	validFile := provideTaskFile(t, dir, "skills-a.json", provideTask())
	secondTask := provideTask()
	secondTask.Id = "context/b"
	secondTask.Kind = "context"
	secondFile := provideTaskFile(t, dir, "context-b.json", secondTask)
	tests := make([]*testCase, 0)

	// valid-file
	tests = append(tests, &testCase{
		_id:       "valid-file",
		_expected: exitClean,

		args: []string{validFile},
	})

	// two-valid-files
	tests = append(tests, &testCase{
		_id:       "two-valid-files",
		_expected: exitClean,

		args: []string{validFile, secondFile},
	})

	// over-cap-problem
	longProblem := provideTask()
	longProblem.Problem = strings.Repeat("x", 321)
	tests = append(tests, &testCase{
		_id:       "over-cap-problem",
		_expected: exitViolations,

		args: []string{provideTaskFile(t, dir, "skills-long-problem.json", longProblem)},
	})

	// over-cap-quote
	longQuote := provideTask()
	longQuote.Evidence = []routines.ChangeEvidence{{Finding: 1, Quote: strings.Repeat("x", 161), Session: "s1"}}
	tests = append(tests, &testCase{
		_id:       "over-cap-quote",
		_expected: exitViolations,

		args: []string{provideTaskFile(t, dir, "skills-long-quote.json", longQuote)},
	})

	// problem-two-sentences
	twoSentences := provideTask()
	twoSentences.Problem = "The probe was denied. The stage said ok."
	tests = append(tests, &testCase{
		_id:       "problem-two-sentences",
		_expected: exitViolations,

		args: []string{provideTaskFile(t, dir, "skills-two-sentences.json", twoSentences)},
	})

	// problem-hex-id
	hexId := provideTask()
	hexId.Problem = "Session 9c44f8b2 found nothing to commit."
	tests = append(tests, &testCase{
		_id:       "problem-hex-id",
		_expected: exitViolations,

		args: []string{provideTaskFile(t, dir, "skills-hex-id.json", hexId)},
	})

	// backlog-bad-id
	badBacklog := provideTask()
	badBacklog.Backlog = "22"
	tests = append(tests, &testCase{
		_id:       "backlog-bad-id",
		_expected: exitViolations,

		args: []string{provideTaskFile(t, dir, "skills-bad-backlog.json", badBacklog)},
	})

	// evidence-missing-finding
	noFinding := `{"id": "skills/no-finding", "kind": "skills", "title": "T", "problem": "It hurt.", "evidence": [{"session": "s", "quote": "q"}], "fix": "f", "sessions": ["s"], "target": "x.md", "change": "Do it.", "notes": ""}`
	tests = append(tests, &testCase{
		_id:       "evidence-missing-finding",
		_expected: exitViolations,

		args: []string{provideTaskText(t, dir, "skills-no-finding.json", noFinding)},
	})

	// missing-field
	tests = append(tests, &testCase{
		_id:       "missing-field",
		_expected: exitViolations,

		args: []string{provideTaskText(t, dir, "skills-missing.json", `{"id": "skills/missing"}`)},
	})

	// not-json
	tests = append(tests, &testCase{
		_id:       "not-json",
		_expected: exitViolations,

		args: []string{provideTaskText(t, dir, "skills-broken.json", "{not json")},
	})

	// missing-file
	tests = append(tests, &testCase{
		_id:       "missing-file",
		_expected: exitError,

		args: []string{filepath.Join(dir, "absent.json")},
	})

	// no-args
	tests = append(tests, &testCase{
		_id:       "no-args",
		_expected: exitError,

		args: []string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runTaskCheck(test.args))
		})
	}
}
