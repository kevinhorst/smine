package skillgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture pins the declaration shape, never the skill's version: the version field is dropped before comparing.
func TestHeartbeatEnforceFixtures(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		rendered string
	}

	repoRoot := filepath.Join("..", "..")
	fixtures, err := filepath.Glob(filepath.Join(repoRoot, "cmd", "heartbeat", "tests", "fixtures", "enforce", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)
	tests := make([]*testCase, 0)

	for _, fixture := range fixtures {
		leaf := strings.TrimSuffix(filepath.Base(fixture), ".json")
		skillDirs, err := filepath.Glob(filepath.Join(repoRoot, "skills", "*", leaf))
		require.NoError(t, err)
		require.Len(t, skillDirs, 1, "skill dir of fixture %s", fixture)
		skill, err := Load(skillDirs[0])
		require.NoError(t, err)
		rendered := deployedEnforce(t, skill)
		checkedIn, err := os.ReadFile(fixture)
		require.NoError(t, err)
		tests = append(tests, &testCase{_id: leaf, _expected: withoutVersion(t, rendered), rendered: withoutVersion(t, checkedIn)})
	}

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.JSONEq(t, test._expected, test.rendered, "the heartbeat fixture differs from the skill's render — copy the deployed enforce.json over it, version excluded")
		})
	}
}

// deployedEnforce is the enforce.json a full install deploys: every variable filled with its provider's catalog program.
func deployedEnforce(t *testing.T, skill *Skill) []byte {
	t.Helper()
	values := map[string]string{}
	for name, variable := range skill.VariableNames() {
		provider := skill.Catalog.ProviderFor(variable.Interface)
		if provider != nil && provider.Program != "" {
			values[name] = provider.Program
		}
	}

	options := RenderOptions{Values: values}
	files, err := RenderDeployed(options, skill)
	require.NoError(t, err)
	for _, file := range files {
		if file.Path == FileEnforce {
			return file.Content
		}
	}
	require.FailNow(t, "no enforce.json rendered", skill.Name())
	return nil
}

func withoutVersion(t *testing.T, document []byte) string {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(document, &fields))
	delete(fields, "version")
	stripped, err := json.Marshal(fields)
	require.NoError(t, err)
	return string(stripped)
}
