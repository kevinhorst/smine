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

const testIndex = `{"entries": [{"id": "ACTION-REVIEW-TEST-001"}]}`

// saveModel imports a demo skill declaring the given acdsl-context into
// <tmp>/demo/, saves its model file and removes the markdown source, so only
// the model remains; returns the model path.
func saveModel(t *testing.T, declaration string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: demo\ndescription: demo fixture. Trigger on nothing.\nauthor: Kevin Horst\nversion: 1.0\nacdsl-context: " + declaration + "\nformat: 2\n---\n# Demo\n\n## Run\n\n**SKILL-DEMO-RUN-001** `[step]` **Act**\n\nACT now.\n"
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
		_id         string
		_expected   string
		_shouldPass bool

		file string
	}

	tests := make([]*testCase, 0)

	// pass-model-path-rendered
	tests = append(tests, &testCase{
		_id:         "pass-model-path-rendered",
		_expected:   "",
		_shouldPass: true,

		file: saveModel(t, "ACTION-REVIEW-TEST-001"),
	})

	// fail-finding-names-rendered-line
	stale := saveModel(t, "ACTION-REVIEW-TEST-002")
	tests = append(tests, &testCase{
		_id:         "fail-finding-names-rendered-line",
		_expected:   filepath.Join(filepath.Dir(stale), skillgraph.FileManifest) + ":6: declared entry \"ACTION-REVIEW-TEST-002\" not in the context index\n",
		_shouldPass: false,

		file: stale,
	})

	// fail-markdown-fixture-path
	fixtureDir := filepath.Join(t.TempDir(), "fixture")
	require.NoError(t, os.MkdirAll(fixtureDir, 0o755))
	fixture := filepath.Join(fixtureDir, skillgraph.FileManifest)
	require.NoError(t, os.WriteFile(fixture, []byte("---\nname: fixture\nacdsl-context: ACTION-REVIEW-TEST-002\n---\n"), 0o644))
	tests = append(tests, &testCase{
		_id:         "fail-markdown-fixture-path",
		_expected:   fixture + ":3: declared entry \"ACTION-REVIEW-TEST-002\" not in the context index\n",
		_shouldPass: false,

		file: fixture,
	})

	// fail-model-not-loading-is-one-violation
	broken := saveModel(t, "ACTION-REVIEW-TEST-001")
	model, err := os.ReadFile(broken)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(broken, append([]byte("// note\n"), model...), 0o644))
	tests = append(tests, &testCase{
		_id:         "fail-model-not-loading-is-one-violation",
		_expected:   broken + ":1: SKILL.md text unavailable — the model does not load or render (violations: ACDSL-SKILL-005)\n",
		_shouldPass: false,

		file: broken,
	})

	indexPath := filepath.Join(t.TempDir(), "context.json")
	require.NoError(t, os.WriteFile(indexPath, []byte(testIndex), 0o644))

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			listPath := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(listPath, []byte(test.file+"\n"), 0o644))

			var out bytes.Buffer
			rc := run([]string{listPath, "index=" + indexPath}, &out)

			assert.Equalf(t, test._shouldPass, rc == 0, "out = %s", out.String())
			assert.Equal(t, test._expected, out.String())
		})
	}
}
