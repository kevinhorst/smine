package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
)

// saveModel imports a demo skill with the given description into
// <tmp>/demo/, saves its model file and removes the markdown source, so only
// the model remains; returns the model path.
func saveModel(t *testing.T, description string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: demo\ndescription: " + description + "\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n---\n# Demo\n\n## Run\n\n**SKILL-DEMO-RUN-001** `[step]` **Act**\n\nACT now.\n"
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
		mode string
	}

	tests := make([]*testCase, 0)

	// pass-model-path-rendered
	tests = append(tests, &testCase{
		_id:         "pass-model-path-rendered",
		_expected:   "",
		_shouldPass: true,

		file: saveModel(t, "demo fixture. Trigger on nothing."),
		mode: "frontmatter",
	})

	// fail-finding-names-rendered-line
	lacking := saveModel(t, "demo fixture without a trigger clause.")
	tests = append(tests, &testCase{
		_id:         "fail-finding-names-rendered-line",
		_expected:   filepath.Join(filepath.Dir(lacking), skillgraph.FileManifest) + ":3: description lacks the \"Trigger on\" clause\n",
		_shouldPass: false,

		file: lacking,
		mode: "frontmatter",
	})

	// fail-markdown-fixture-path
	fixtureDir := filepath.Join(t.TempDir(), "fixture")
	require.NoError(t, os.MkdirAll(fixtureDir, 0o755))
	fixture := filepath.Join(fixtureDir, skillgraph.FileManifest)
	require.NoError(t, os.WriteFile(fixture, []byte("---\nname: fixture\ndescription: no trigger clause\nauthor: Kevin Horst\nversion: 1.0\n---\n"), 0o644))
	tests = append(tests, &testCase{
		_id:         "fail-markdown-fixture-path",
		_expected:   fixture + ":3: description lacks the \"Trigger on\" clause\n",
		_shouldPass: false,

		file: fixture,
		mode: "frontmatter",
	})

	// fail-model-not-loading-is-one-violation
	broken := saveModel(t, "demo fixture. Trigger on nothing.")
	model, err := os.ReadFile(broken)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(broken, append([]byte("// note\n"), model...), 0o644))
	tests = append(tests, &testCase{
		_id:         "fail-model-not-loading-is-one-violation",
		_expected:   broken + ":1: SKILL.md text unavailable — the model does not load or render (violations: ACDSL-SKILL-005)\n",
		_shouldPass: false,

		file: broken,
		mode: "frontmatter",
	})

	// fail-runtime-meta-bypass-without-docker
	bypass := saveModel(t, "demo fixture. Trigger on nothing.")
	bypassModel, err := os.ReadFile(bypass)
	require.NoError(t, err)
	bypassEdited := strings.Replace(string(bypassModel), "\n}]\n", "\n}]\nenforcePermissions: [\"bypass\"]\n", 1)
	require.NotEqual(t, string(bypassModel), bypassEdited)
	require.NoError(t, os.WriteFile(bypass, []byte(bypassEdited), 0o644))
	_, bypassErr := skillgraph.Load(filepath.Dir(bypass))
	require.ErrorContains(t, bypassErr, `bypass requires docker: "always"`)
	tests = append(tests, &testCase{
		_id:         "fail-runtime-meta-bypass-without-docker",
		_expected:   bypass + ":1: model does not load — " + bypassErr.Error() + "\n",
		_shouldPass: false,

		file: bypass,
		mode: "runtime-meta",
	})

	// pass-runtime-meta-agnostic
	tests = append(tests, &testCase{
		_id:         "pass-runtime-meta-agnostic",
		_expected:   "",
		_shouldPass: true,

		file: saveModel(t, "demo fixture. Trigger on nothing."),
		mode: "runtime-meta",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			listPath := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(listPath, []byte(test.file+"\n"), 0o644))

			var out bytes.Buffer
			rc := run([]string{listPath, "mode=" + test.mode}, &out)

			assert.Equalf(t, test._shouldPass, rc == 0, "out = %s", out.String())
			assert.Equal(t, test._expected, out.String())
		})
	}
}
