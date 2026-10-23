package sessiontitle

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// humanGateDocument declares HUMAN-GATE for the skill named by its format verb.
const humanGateDocument = `{
  "schema": 2,
  "skill": "%s",
  "states": {
    "HUMAN-GATE": {
      "entry": "SKILL-TEST-GATE-001",
      "meaning": "%s gate presented"
    }
  },
  "version": "1.0.0"
}`

// provideSkillsDir writes each document as <leaf>/title.json of a temp skills
// dir; an empty document leaves the leaf without a declaration.
func provideSkillsDir(t *testing.T, documents map[string]string) string {
	t.Helper()

	skillsDir := t.TempDir()
	for leaf, document := range documents {
		leafDir := filepath.Join(skillsDir, leaf)
		require.NoError(t, os.MkdirAll(leafDir, 0o755))
		if document == "" {
			continue
		}

		require.NoError(t, os.WriteFile(filepath.Join(leafDir, FileName), []byte(document), 0o644))
	}
	return skillsDir
}

func provideVocabulary(t *testing.T, documents map[string]string) *Vocabulary {
	t.Helper()

	vocabulary, err := LoadVocabulary(provideSkillsDir(t, documents))
	require.NoError(t, err)
	return vocabulary
}

func TestLoadVocabulary(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Vocabulary
		_shouldPass bool

		skillsDir string
	}

	tests := make([]*testCase, 0)

	// two-skills
	twoSkills := map[string]string{"fdesign": fdesignDocument, "railroad-review": railroadReviewDocument}
	tests = append(tests, &testCase{
		_id: "two-skills",
		_expected: &Vocabulary{declared: map[State]*Declared{
			"CODE-REVIEWABLE": {Meaning: "code gate presented — approve the Changes or request a change", Skill: "fdesign"},
			"DESIGN-READY":    {Meaning: "design gate presented — approve or revise the design", Skill: "fdesign"},
			"HUMAN-GATE":      {Meaning: "round complete — say go for the next round or accept the review", Skill: "railroad-review"},
		}},
		_shouldPass: true,

		skillsDir: provideSkillsDir(t, twoSkills),
	})

	// shared-token-first-skill-names-it
	sharedToken := map[string]string{
		"alpha": fmt.Sprintf(humanGateDocument, "alpha", "alpha"),
		"beta":  fmt.Sprintf(humanGateDocument, "beta", "beta"),
	}
	tests = append(tests, &testCase{
		_id: "shared-token-first-skill-names-it",
		_expected: &Vocabulary{declared: map[State]*Declared{
			"HUMAN-GATE": {Meaning: "alpha gate presented", Skill: "alpha"},
		}},
		_shouldPass: true,

		skillsDir: provideSkillsDir(t, sharedToken),
	})

	// no-declarations
	noDeclarations := map[string]string{"close": ""}
	tests = append(tests, &testCase{
		_id:         "no-declarations",
		_expected:   &Vocabulary{declared: map[State]*Declared{}},
		_shouldPass: true,

		skillsDir: provideSkillsDir(t, noDeclarations),
	})

	// skills-dir-missing
	tests = append(tests, &testCase{
		_id:         "skills-dir-missing",
		_expected:   &Vocabulary{declared: map[State]*Declared{}},
		_shouldPass: true,

		skillsDir: filepath.Join(t.TempDir(), "missing"),
	})

	// invalid-document-fails
	invalidDocument := map[string]string{"fdesign": fdesignDocument, "broken": `{"schema": 2, "skill": `}
	tests = append(tests, &testCase{
		_id:         "invalid-document-fails",
		_expected:   nil,
		_shouldPass: false,

		skillsDir: provideSkillsDir(t, invalidDocument),
	})

	// builtin-token-ignored
	builtinToken := map[string]string{"rogue": `{
  "schema": 2,
  "skill": "rogue",
  "states": {
    "HUMAN-GATE": {"entry": "SKILL-ROGUE-GATE-001", "meaning": "rogue gate presented"},
    "READY": {"entry": "SKILL-ROGUE-GATE-002", "meaning": "rogue review"}
  },
  "version": "1.0.0"
}`}
	tests = append(tests, &testCase{
		_id: "builtin-token-ignored",
		_expected: &Vocabulary{declared: map[State]*Declared{
			"HUMAN-GATE": {Meaning: "rogue gate presented", Skill: "rogue"},
		}},
		_shouldPass: true,

		skillsDir: provideSkillsDir(t, builtinToken),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			vocabulary, err := LoadVocabulary(test.skillsDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, vocabulary)
		})
	}
}

func TestVocabulary_IsSettable(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		state State
	}

	vocabulary := provideVocabulary(t, map[string]string{"fdesign": fdesignDocument})

	tests := make([]*testCase, 0)

	// wip
	tests = append(tests, &testCase{_id: "wip", _expected: true, state: StateWip})

	// ready
	tests = append(tests, &testCase{_id: "ready", _expected: true, state: StateReady})

	// declared
	tests = append(tests, &testCase{_id: "declared", _expected: true, state: "DESIGN-READY"})

	// undeclared
	tests = append(tests, &testCase{_id: "undeclared", _expected: false, state: "HUMAN-GATE"})

	// terminal-builtin
	tests = append(tests, &testCase{_id: "terminal-builtin", _expected: false, state: StateDone})

	// none
	tests = append(tests, &testCase{_id: "none", _expected: false, state: StateNone})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, vocabulary.IsSettable(test.state))
		})
	}
}
