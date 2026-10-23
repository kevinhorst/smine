package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
)

// saveModel imports a demo skill whose one step reads statement into
// <tmp>/demo/, saves its model file and removes the markdown source, so only
// the model remains; returns the model path.
func saveModel(t *testing.T, statement string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: demo\ndescription: demo fixture. Trigger on nothing.\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n---\n# Demo\n\n## Run\n\n**SKILL-DEMO-RUN-001** `[step]` **Act**\n\n" + statement + "\n"
	changelog := `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillgraph.FileManifest), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(changelog), 0o644))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	require.NoError(t, os.Remove(filepath.Join(dir, skillgraph.FileManifest)))
	require.NoError(t, os.Remove(filepath.Join(dir, "changelog.json")))
	return skillgraph.ModelPath(dir)
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id           string
		_expected     string
		_expectedExit int

		file string
	}

	tests := make([]*testCase, 0)

	// model-path-rendered
	tests = append(tests, &testCase{
		_id:           "model-path-rendered",
		_expected:     "",
		_expectedExit: 0,

		file: saveModel(t, "ACT now."),
	})

	// finding-names-rendered-line
	forbidden := saveModel(t, "ACT now, then WAIT FOR THE WAVE.")
	model, err := os.ReadFile(forbidden)
	require.NoError(t, err)
	require.NotContains(t, string(model), "\nACT now, then WAIT FOR THE WAVE.\n")
	tests = append(tests, &testCase{
		_id:           "finding-names-rendered-line",
		_expected:     filepath.Join(filepath.Dir(forbidden), skillgraph.FileManifest) + ":15: forbidden phrase matches \"WAIT FOR THE WAVE\"\n",
		_expectedExit: 1,

		file: forbidden,
	})

	// markdown-fixture-path
	fixture := filepath.Join(t.TempDir(), "worker.js")
	require.NoError(t, os.WriteFile(fixture, []byte("x\n// WAIT FOR THE WAVE\n"), 0o600))
	tests = append(tests, &testCase{
		_id:           "markdown-fixture-path",
		_expected:     fixture + ":2: forbidden phrase matches \"WAIT FOR THE WAVE\"\n",
		_expectedExit: 1,

		file: fixture,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			list := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(list, []byte(test.file+"\n"), 0o600))
			var out bytes.Buffer

			exit := run([]string{list, "pattern=WAIT FOR THE WAVE"}, &out)

			assert.Equal(t, test._expectedExit, exit, out.String())
			assert.Equal(t, test._expected, out.String())
		})
	}
}
