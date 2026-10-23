package skillenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func provideEntry(name string, iface string) *Entry {
	return &Entry{
		Example:   "go vet ./...",
		Interface: iface,
		Name:      name,
		Required:  true,
		Scope:     "repo",
		Summary:   "Runs everything required before a commit.",
	}
}

// provideDocument writes the document as skillsDir/<skill>/env.json.
func provideDocument(t *testing.T, document *Document, skillsDir string) {
	t.Helper()
	encoded, err := json.MarshalIndent(document, "", "  ")
	require.NoError(t, err)

	dir := filepath.Join(skillsDir, document.Skill)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), encoded, 0o644))
}

func TestParseDocument(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Document
		_shouldPass bool

		content []byte
	}

	valid := &Document{
		Schema:    DocumentSchema,
		Skill:     "fimplement",
		Variables: []*Entry{provideEntry("SKILL_FIMPLEMENT_CHECK", "check")},
		Version:   "1.0.0",
	}
	validContent, err := json.Marshal(valid)
	require.NoError(t, err)

	wrongSchema := &Document{Schema: 2, Skill: "fimplement", Version: "1.0.0"}
	wrongSchemaContent, err := json.Marshal(wrongSchema)
	require.NoError(t, err)

	tests := make([]*testCase, 0)

	// pass-schema-one
	tests = append(tests, &testCase{
		_id:         "pass-schema-one",
		_expected:   valid,
		_shouldPass: true,

		content: validContent,
	})

	// fail-wrong-schema
	tests = append(tests, &testCase{
		_id:         "fail-wrong-schema",
		_expected:   nil,
		_shouldPass: false,

		content: wrongSchemaContent,
	})

	// fail-invalid-json
	tests = append(tests, &testCase{
		_id:         "fail-invalid-json",
		_expected:   nil,
		_shouldPass: false,

		content: []byte("{"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			document, err := ParseDocument(test.content)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, document)
		})
	}
}

func TestLoadDocuments(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		skillsDir string
	}

	alpha := &Document{Schema: DocumentSchema, Skill: "alpha", Variables: []*Entry{provideEntry("SKILL_ALPHA_CHECK", "check")}, Version: "1.0.0"}
	beta := &Document{Schema: DocumentSchema, Skill: "beta", Variables: []*Entry{provideEntry("SKILL_BETA_TEST", "test")}, Version: "1.0.0"}

	twoSkills := t.TempDir()
	provideDocument(t, beta, twoSkills)
	provideDocument(t, alpha, twoSkills)

	brokenOne := t.TempDir()
	provideDocument(t, alpha, brokenOne)
	brokenDir := filepath.Join(brokenOne, "broken")
	require.NoError(t, os.MkdirAll(brokenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(brokenDir, FileName), []byte("{"), 0o644))

	tests := make([]*testCase, 0)

	// two-skills-sorted
	tests = append(tests, &testCase{
		_id:         "two-skills-sorted",
		_expected:   []string{"alpha", "beta"},
		_shouldPass: true,

		skillsDir: twoSkills,
	})

	// broken-one-skipped-and-joined
	tests = append(tests, &testCase{
		_id:         "broken-one-skipped-and-joined",
		_expected:   []string{"alpha"},
		_shouldPass: false,

		skillsDir: brokenOne,
	})

	// none-empty
	tests = append(tests, &testCase{
		_id:         "none-empty",
		_expected:   []string{},
		_shouldPass: true,

		skillsDir: t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			documents, err := LoadDocuments(test.skillsDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, documentSkills(documents))
		})
	}
}

func documentSkills(documents []*Document) []string {
	skills := make([]string, 0, len(documents))
	for _, document := range documents {
		skills = append(skills, document.Skill)
	}
	return skills
}

func TestBindings(t *testing.T) {
	alpha := &Document{Variables: []*Entry{provideEntry("SKILL_ALPHA_CHECK", "check"), provideEntry("SKILL_ALPHA_TEST", "test")}}
	beta := &Document{Variables: []*Entry{provideEntry("SKILL_BETA_BUILD", "build")}}
	documents := []*Document{alpha, beta}

	expected := map[string]string{"SKILL_ALPHA_CHECK": "check", "SKILL_ALPHA_TEST": "test", "SKILL_BETA_BUILD": "build"}
	assert.Equal(t, expected, Bindings(documents))
}

func TestPhrase(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		summary string
	}

	tests := make([]*testCase, 0)

	// period-cut-first-lowered
	tests = append(tests, &testCase{
		_id:       "period-cut-first-lowered",
		_expected: "runs everything required before a commit",

		summary: "Runs everything required before a commit.",
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: "",

		summary: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Phrase(test.summary))
		})
	}
}
