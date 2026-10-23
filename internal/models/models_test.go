package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var templateAvailable = []string{
	"claude-opus-5-5",
	"claude-opus-5-5[1m]",
	"claude-opus-5",
	"claude-opus-5[1m]",
	"claude-fable-5",
	"claude-fable-5[1m]",
	"claude-sonnet-5",
	"claude-sonnet-5[1m]",
}

func templateVocabulary() *Vocabulary {
	return &Vocabulary{Available: templateAvailable, Forbidden: []string{"*fable*"}}
}

func TestVocabulary_Resolves(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		token      string
		vocabulary *Vocabulary
	}

	tests := make([]*testCase, 0)

	// sonnet
	tests = append(tests, &testCase{_id: "sonnet", _expected: true, token: "sonnet", vocabulary: templateVocabulary()})

	// opus
	tests = append(tests, &testCase{_id: "opus", _expected: true, token: "opus", vocabulary: templateVocabulary()})

	// fable
	tests = append(tests, &testCase{_id: "fable", _expected: true, token: "fable", vocabulary: templateVocabulary()})

	// claude-sonnet-5-5
	tests = append(tests, &testCase{_id: "claude-sonnet-5-5", _expected: true, token: "claude-sonnet-5-5", vocabulary: templateVocabulary()})

	// claude-opus-5-5
	tests = append(tests, &testCase{_id: "claude-opus-5-5", _expected: true, token: "claude-opus-5-5", vocabulary: templateVocabulary()})

	// claude-fable-5-1
	tests = append(tests, &testCase{_id: "claude-fable-5-1", _expected: true, token: "claude-fable-5-1", vocabulary: templateVocabulary()})

	// claude-sonnet-5-context
	tests = append(tests, &testCase{_id: "claude-sonnet-5-context", _expected: true, token: "claude-sonnet-5[1m]", vocabulary: templateVocabulary()})

	// mixed-case-context
	tests = append(tests, &testCase{_id: "mixed-case-context", _expected: true, token: "Claude-Opus-5-5[1m]", vocabulary: templateVocabulary()})

	// haiku
	tests = append(tests, &testCase{_id: "haiku", _expected: false, token: "haiku", vocabulary: templateVocabulary()})

	// sonnet5
	tests = append(tests, &testCase{_id: "sonnet5", _expected: false, token: "sonnet5", vocabulary: templateVocabulary()})

	// sonnet-dash-5
	tests = append(tests, &testCase{_id: "sonnet-dash-5", _expected: false, token: "sonnet-5", vocabulary: templateVocabulary()})

	// opus-dash-5
	tests = append(tests, &testCase{_id: "opus-dash-5", _expected: false, token: "opus-5", vocabulary: templateVocabulary()})

	// claude-haiku-4-5
	tests = append(tests, &testCase{_id: "claude-haiku-4-5", _expected: false, token: "claude-haiku-4-5-20251001", vocabulary: templateVocabulary()})

	// claude-opus-4-8
	tests = append(tests, &testCase{_id: "claude-opus-4-8", _expected: false, token: "claude-opus-4-8[1m]", vocabulary: templateVocabulary()})

	// opus-high
	tests = append(tests, &testCase{_id: "opus-high", _expected: false, token: "opus-high", vocabulary: templateVocabulary()})

	// empty
	tests = append(tests, &testCase{_id: "empty", _expected: false, token: "", vocabulary: templateVocabulary()})

	// inherit
	tests = append(tests, &testCase{_id: "inherit", _expected: false, token: "inherit", vocabulary: templateVocabulary()})

	// empty-available
	tests = append(tests, &testCase{_id: "empty-available", _expected: true, token: "haiku", vocabulary: &Vocabulary{}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.vocabulary.Resolves(test.token))
		})
	}
}

func TestVocabulary_IsSubagent(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		token string
	}

	tests := make([]*testCase, 0)

	// opus
	tests = append(tests, &testCase{_id: "opus", _expected: true, token: "opus"})

	// sonnet
	tests = append(tests, &testCase{_id: "sonnet", _expected: true, token: "sonnet"})

	// claude-sonnet-5-5
	tests = append(tests, &testCase{_id: "claude-sonnet-5-5", _expected: true, token: "claude-sonnet-5-5"})

	// fable
	tests = append(tests, &testCase{_id: "fable", _expected: false, token: "fable"})

	// claude-fable-5-1
	tests = append(tests, &testCase{_id: "claude-fable-5-1", _expected: false, token: "claude-fable-5-1"})

	// upper-case-fable
	tests = append(tests, &testCase{_id: "upper-case-fable", _expected: false, token: "CLAUDE-FABLE-5-1"})

	// haiku
	tests = append(tests, &testCase{_id: "haiku", _expected: false, token: "haiku"})

	// Run tests
	vocabulary := templateVocabulary()
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, vocabulary.IsSubagent(test.token))
		})
	}
}

func TestVocabulary_SubagentAliases(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedAliases []string
		_expectedDefault string

		vocabulary *Vocabulary
	}

	tests := make([]*testCase, 0)

	// template
	tests = append(tests, &testCase{
		_id:              "template",
		_expectedAliases: []string{"opus", "sonnet"},
		_expectedDefault: "opus",

		vocabulary: templateVocabulary(),
	})

	// no-forbidden
	tests = append(tests, &testCase{
		_id:              "no-forbidden",
		_expectedAliases: []string{"fable", "opus", "sonnet"},
		_expectedDefault: "fable",

		vocabulary: &Vocabulary{Available: templateAvailable},
	})

	// only-haiku
	tests = append(tests, &testCase{
		_id:              "only-haiku",
		_expectedAliases: []string{"haiku"},
		_expectedDefault: "haiku",

		vocabulary: &Vocabulary{Available: []string{"claude-haiku-4-5-20251001"}},
	})

	// nothing-resolves
	tests = append(tests, &testCase{
		_id:              "nothing-resolves",
		_expectedAliases: []string{},
		_expectedDefault: "",

		vocabulary: &Vocabulary{Available: []string{"claude-opus-4-8"}, Forbidden: []string{"*opus*"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedAliases, test.vocabulary.SubagentAliases())
			assert.Equal(t, test._expectedDefault, test.vocabulary.DefaultSubagent())
		})
	}
}

func TestVocabulary_WorkflowBlock(t *testing.T) {
	expected := "// Subagent model vocabulary — generated from availableModels and\n" +
		"// forbidden_subagent_models by the workflow generator (internal/models).\n" +
		"const SUBAGENT_MODELS = [\"opus\", \"sonnet\"]\n" +
		"const DEFAULT_SUBAGENT_MODEL = \"opus\"\n" +
		"const assertModel = (name, value) => {\n" +
		"  if (!SUBAGENT_MODELS.includes(value)) throw new Error(`${name} \"${value}\" is not a subagent model (${SUBAGENT_MODELS.join(' | ')}) — unresolved tokens silently run the session model, forbidden ones are stopped by heartbeat`)\n" +
		"  return value\n" +
		"}\n"

	assert.Equal(t, expected, templateVocabulary().WorkflowBlock())
}

func TestLoadTemplates(t *testing.T) {
	vocabulary, err := LoadTemplates(filepath.Join("..", ".."))
	require.NoError(t, err)

	assert.NotEmpty(t, vocabulary.Available)
	assert.Equal(t, []string{"*fable*"}, vocabulary.Forbidden)
	assert.Contains(t, vocabulary.SubagentAliases(), vocabulary.DefaultSubagent())
}

func TestLoad_MissingHeartbeatConfig(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"availableModels": ["claude-opus-5-5"]}`), 0o600))

	vocabulary, err := Load(settingsPath, filepath.Join(dir, "heartbeat", "config.json"))
	require.NoError(t, err)

	assert.Equal(t, []string{"claude-opus-5-5"}, vocabulary.Available)
	assert.Nil(t, vocabulary.Forbidden)
}
