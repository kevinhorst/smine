package sessiontitle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fdesignDocument is fdesign's rendered title.json.
const fdesignDocument = `{
  "schema": 2,
  "skill": "fdesign",
  "states": {
    "CODE-REVIEWABLE": {
      "entry": "SKILL-FDESIGN-CODE-005",
      "meaning": "code gate presented — approve the Changes or request a change"
    },
    "DESIGN-READY": {
      "entry": "SKILL-FDESIGN-GATE-001",
      "meaning": "design gate presented — approve or revise the design"
    }
  },
  "version": "9.4.0"
}`

// testLimits are the session-helpers catalog defaults.
var testLimits = &Limits{MaxStates: 8, MaxTitleRunes: 62, MaxTokenRunes: 16}

// railroadReviewDocument is railroad-review's rendered title.json.
const railroadReviewDocument = `{
  "schema": 2,
  "skill": "railroad-review",
  "states": {
    "HUMAN-GATE": {
      "entry": "SKILL-RAILROADREVIEW-MODES-001",
      "meaning": "round complete — say go for the next round or accept the review"
    }
  },
  "version": "11.2.0"
}`

func provideDocument(t *testing.T, content string) *Document {
	t.Helper()

	document, err := ParseDocument([]byte(content))
	require.NoError(t, err)
	return document
}

// provideGateDeclaration declares one state presented by one entry.
func provideGateDeclaration(token string) *Declaration {
	state := &DeclaredState{Entry: "SKILL-TEST-GATE-001", Meaning: "gate presented"}
	declaration := &Declaration{States: map[string]*DeclaredState{token: state}}
	return declaration
}

func TestDeclaration_Violations(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		declaration *Declaration
	}

	tests := make([]*testCase, 0)

	// pass-valid
	tests = append(tests, &testCase{
		_id:       "pass-valid",
		_expected: nil,

		declaration: provideDocument(t, fdesignDocument).Declaration,
	})

	// fail-lowercase-token
	tests = append(tests, &testCase{
		_id:       "fail-lowercase-token",
		_expected: []string{"title: state design-ready: token is not uppercase words joined by hyphens"},

		declaration: provideGateDeclaration("design-ready"),
	})

	// fail-token-over-cap
	tests = append(tests, &testCase{
		_id:       "fail-token-over-cap",
		_expected: []string{"title: state VERY-LONG-GATE-STATE: token exceeds 16 runes"},

		declaration: provideGateDeclaration("VERY-LONG-GATE-STATE"),
	})

	// fail-builtin-token
	tests = append(tests, &testCase{
		_id:       "fail-builtin-token",
		_expected: []string{"title: state READY: token is a built-in state"},

		declaration: provideGateDeclaration("READY"),
	})

	// fail-nine-states
	nineStates := &Declaration{States: map[string]*DeclaredState{}}
	for _, token := range []string{"ONE", "TWO", "THREE", "FOUR", "FIVE", "SIX", "SEVEN", "EIGHT", "NINE"} {
		nineStates.States[token] = &DeclaredState{Entry: "SKILL-TEST-GATE-001", Meaning: "gate presented"}
	}
	tests = append(tests, &testCase{
		_id:       "fail-nine-states",
		_expected: []string{"title: 9 states, at most 8"},

		declaration: nineStates,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.declaration.Violations(testLimits))
		})
	}
}

func TestParseDocument(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Document
		_shouldPass bool

		content string
	}

	tests := make([]*testCase, 0)

	// pass-valid
	railroadReview := &Document{
		Declaration: &Declaration{
			States: map[string]*DeclaredState{
				"HUMAN-GATE": {
					Entry:   "SKILL-RAILROADREVIEW-MODES-001",
					Meaning: "round complete — say go for the next round or accept the review",
				},
			},
		},
		Schema:  2,
		Skill:   "railroad-review",
		Version: "11.2.0",
	}
	tests = append(tests, &testCase{
		_id:         "pass-valid",
		_expected:   railroadReview,
		_shouldPass: true,

		content: railroadReviewDocument,
	})

	// fail-signal-schema
	tests = append(tests, &testCase{
		_id:         "fail-signal-schema",
		_expected:   nil,
		_shouldPass: false,

		content: `{"schema": 1, "skill": "railroad-review", "states": {}, "version": "11.2.0"}`,
	})

	// fail-invalid-json
	tests = append(tests, &testCase{
		_id:         "fail-invalid-json",
		_expected:   nil,
		_shouldPass: false,

		content: `{"schema": 2, "skill": `,
	})

	// fail-no-states
	tests = append(tests, &testCase{
		_id:         "fail-no-states",
		_expected:   nil,
		_shouldPass: false,

		content: `{"schema": 2, "skill": "railroad-review", "version": "11.2.0"}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			document, err := ParseDocument([]byte(test.content))
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, document)
		})
	}
}

func TestSetCommand(t *testing.T) {
	assert.Equal(t, "smine-rules title set DESIGN-READY", SetCommand("DESIGN-READY"))
}
