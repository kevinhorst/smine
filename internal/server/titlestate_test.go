package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// designReadyDocument is a title.json declaring DESIGN-READY for the skill
// named by its one verb.
const designReadyDocument = `{
  "schema": 2,
  "skill": "%s",
  "states": {
    "DESIGN-READY": {
      "entry": "SKILL-FDESIGN-GATE-001",
      "meaning": "design gate presented"
    }
  },
  "version": "1.0.0"
}`

// writeTitleDeclaration deploys skill's DESIGN-READY declaration into
// skillsDir.
func writeTitleDeclaration(t *testing.T, skillsDir string, skill string) {
	t.Helper()
	skillDir := filepath.Join(skillsDir, skill)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "title.json"), fmt.Appendf(nil, designReadyDocument, skill), 0o644))
}

func TestTitleStore_View(t *testing.T) {
	type testCase struct {
		_id       string
		_expected titleView

		title string
	}

	skillsDir := t.TempDir()
	writeTitleDeclaration(t, skillsDir, "fdesign")
	store := newTitleStore(skillsDir)

	tests := make([]*testCase, 0)

	// declared-state
	tests = append(tests, &testCase{
		_id:       "declared-state",
		_expected: titleView{Badge: badgeAction, Meaning: "design gate presented", Skill: "fdesign", State: "DESIGN-READY", Text: "CC,FD - Gate states"},

		title: "CC,FD - Gate states - [DESIGN-READY]",
	})

	// wip
	tests = append(tests, &testCase{
		_id:       "wip",
		_expected: titleView{Badge: badgeDim, State: "WIP", Text: "CC,FD - Gate states - T1"},

		title: "CC,FD - Gate states - T1[WIP]",
	})

	// ready
	tests = append(tests, &testCase{
		_id:       "ready",
		_expected: titleView{Badge: badgeAction, State: "READY", Text: "CC,FD - Gate states"},

		title: "CC,FD - Gate states - [READY]",
	})

	// testing
	tests = append(tests, &testCase{
		_id:       "testing",
		_expected: titleView{Badge: badgeAction, State: "TESTING", Text: "CC,FD - Gate states"},

		title: "CC,FD - Gate states - [TESTING]",
	})

	// done
	tests = append(tests, &testCase{
		_id:       "done",
		_expected: titleView{Badge: badgeOk, State: "DONE", Text: "CC,FD - Gate states - T1"},

		title: "CC,FD - Gate states - T1[DONE]",
	})

	// closed
	tests = append(tests, &testCase{
		_id:       "closed",
		_expected: titleView{Badge: badgeDim, State: "CLOSED", Text: "CC - Gate states"},

		title: "CC - Gate states - [CLOSED]",
	})

	// unformatted
	tests = append(tests, &testCase{
		_id:       "unformatted",
		_expected: titleView{Text: "Fix the login bug"},

		title: "Fix the login bug",
	})

	// undeclared-token
	tests = append(tests, &testCase{
		_id:       "undeclared-token",
		_expected: titleView{Text: "CC,FD - Gate states - [HAS-PLAN]"},

		title: "CC,FD - Gate states - [HAS-PLAN]",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, store.view(test.title))
		})
	}
}

func TestTitleStore_Reload(t *testing.T) {
	skillsDir := t.TempDir()
	store := newTitleStore(skillsDir)
	title := "CC,FD - Gate states - [DESIGN-READY]"

	assert.Equal(t, titleView{Text: title}, store.view(title), "no declaration deployed yet")

	writeTitleDeclaration(t, skillsDir, "fdesign")
	store.reload()

	expected := titleView{Badge: badgeAction, Meaning: "design gate presented", Skill: "fdesign", State: "DESIGN-READY", Text: "CC,FD - Gate states"}
	assert.Equal(t, expected, store.view(title))
}
