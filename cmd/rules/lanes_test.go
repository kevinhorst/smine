package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextdocs"
)

const (
	checklistHead      = "1111111111111111111111111111111111111111"
	checklistMovedHead = "2222222222222222222222222222222222222222"
)

// errorText returns err's message, empty for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// provideChecklistFile writes a checklist for head over one group and returns its path.
func provideChecklistFile(t *testing.T, head string) string {
	t.Helper()
	document := &checklistDocument{
		Groups: []contextdocs.WalkGroup{{Id: "group", Items: []string{"contracts-1: signature holds"}}},
		Head:   head,
		Shape:  "group-first",
	}
	data, err := json.Marshal(document)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "checklist.json")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

// provideChecklistWalk is the walk provideChecklistFile's checklist decodes to.
func provideChecklistWalk() *contextdocs.Walk {
	walk := &contextdocs.Walk{
		Groups: []contextdocs.WalkGroup{{Id: "group", Items: []string{"contracts-1: signature holds"}}},
		Shape:  "group-first",
	}
	return walk
}

func TestReadChecklist(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      *contextdocs.Walk
		_expectedError string
		_shouldPass    bool

		direction string
		path      string
	}

	tests := make([]*testCase, 0)

	// contracts-same-head
	tests = append(tests, &testCase{
		_id:            "contracts-same-head",
		_expected:      provideChecklistWalk(),
		_expectedError: "",
		_shouldPass:    true,

		direction: "contracts",
		path:      provideChecklistFile(t, checklistHead),
	})

	// contracts-head-moved
	tests = append(tests, &testCase{
		_id:            "contracts-head-moved",
		_expected:      nil,
		_expectedError: "the head moved",
		_shouldPass:    false,

		direction: "contracts",
		path:      provideChecklistFile(t, checklistMovedHead),
	})

	// contracts-without-head
	tests = append(tests, &testCase{
		_id:            "contracts-without-head",
		_expected:      nil,
		_expectedError: "the head moved",
		_shouldPass:    false,

		direction: "contracts",
		path:      provideChecklistFile(t, ""),
	})

	// correctness-head-ignored
	tests = append(tests, &testCase{
		_id:            "correctness-head-ignored",
		_expected:      provideChecklistWalk(),
		_expectedError: "",
		_shouldPass:    true,

		direction: "correctness",
		path:      provideChecklistFile(t, checklistMovedHead),
	})

	// file-missing
	tests = append(tests, &testCase{
		_id:            "file-missing",
		_expected:      nil,
		_expectedError: "readJsonFile",
		_shouldPass:    false,

		direction: "contracts",
		path:      filepath.Join(t.TempDir(), "absent.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			walk, err := readChecklist(test.direction, checklistHead, test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, walk)
			assert.Contains(t, errorText(err), test._expectedError)
		})
	}
}
