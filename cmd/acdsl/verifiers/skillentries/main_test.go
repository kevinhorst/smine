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

// saveModel writes a demo skill body under a frontmatter named demo into
// <tmp>/<leaf>/, imports it and saves it as the dir's model file; returns the model path.
func saveModel(t *testing.T, body, leaf string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), leaf)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: demo\ndescription: demo fixture. Trigger on nothing.\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n---\n# Demo\n\n" + body
	changelog := `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(changelog), 0o644))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	modelPath := skillgraph.ModelPath(dir)
	savedPath := filepath.Join(dir, skillgraph.ModelFile("demo"))
	if savedPath != modelPath {
		require.NoError(t, os.Rename(savedPath, modelPath))
	}
	return modelPath
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		file string
	}

	cleanBody := "## Run\n\n**SKILL-DEMO-RUN-001** `[step]` **Act**\n\nACT now.\n"
	tests := make([]*testCase, 0)

	// pass-clean-model
	tests = append(tests, &testCase{
		_id:         "pass-clean-model",
		_expected:   "",
		_shouldPass: true,

		file: saveModel(t, cleanBody, "demo"),
	})

	// fail-load-error
	broken := saveModel(t, cleanBody, "demo")
	model, err := os.ReadFile(broken)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(broken, append([]byte("// note\n"), model...), 0o644))
	tests = append(tests, &testCase{
		_id:         "fail-load-error",
		_expected:   "demo.cue:1: ",
		_shouldPass: false,

		file: broken,
	})

	// fail-name-mismatch
	tests = append(tests, &testCase{
		_id:         "fail-name-mismatch",
		_expected:   `Frontmatter name "demo" differs from the skill dir`,
		_shouldPass: false,

		file: saveModel(t, cleanBody, "other"),
	})

	// fail-claimless-hook
	tests = append(tests, &testCase{
		_id:         "fail-claimless-hook",
		_expected:   "SKILL-DEMO-RUN-001: [hook] requires a Hook: bullet",
		_shouldPass: false,

		file: saveModel(t, "## Run\n\n**SKILL-DEMO-RUN-001** `[step][hook]` **Act**\n\nACT now.\n", "demo"),
	})

	// fail-hook-outside-cmd
	tests = append(tests, &testCase{
		_id:         "fail-hook-outside-cmd",
		_expected:   "SKILL-DEMO-RUN-001: Hook: internal/x.go is not under cmd/",
		_shouldPass: false,

		file: saveModel(t, "## Run\n\n**SKILL-DEMO-RUN-001** `[step][hook]` **Act**\n\nACT now.\n\n* Hook: internal/x.go\n", "demo"),
	})

	// fail-undeclared-gate
	tests = append(tests, &testCase{
		_id:         "fail-undeclared-gate",
		_expected:   "SKILL-DEMO-RUN-001: Gate: ACDSL-NOPE-999 not declared in acdsl/*.acdsl",
		_shouldPass: false,

		file: saveModel(t, "## Run\n\n**SKILL-DEMO-RUN-001** `[step][gate]` **Act**\n\nACT now.\n\n* Gate: ACDSL-NOPE-999\n", "demo"),
	})

	// fail-enumeration
	tests = append(tests, &testCase{
		_id:         "fail-enumeration",
		_expected:   "SKILL.md:",
		_shouldPass: false,

		file: saveModel(t, "## Run\n\n**SKILL-DEMO-RUN-001** `[step]` **Pick one**\n\nPICK one of `a, b, c, d, e, f, g`.\n", "demo"),
	})

	// pass-format-3-schema-field
	fieldsModel := saveModel(t, cleanBody, "demo")
	fieldsDir := filepath.Dir(fieldsModel)
	fieldsSkill, err := skillgraph.Load(fieldsDir)
	require.NoError(t, err)
	for index := range fieldsSkill.Frontmatter {
		if fieldsSkill.Frontmatter[index].Key == "format" {
			fieldsSkill.Frontmatter[index].Value = " 3"
		}
	}
	fieldsEntry := fieldsSkill.Entries()[0].Entry
	fieldsEntry.Channel = "schema"
	fieldsEntry.Fields = &skillgraph.Fields{Schema: "schemas/x.json"}
	require.NoError(t, os.MkdirAll(filepath.Join(fieldsDir, "schemas"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "schemas", "x.json"), []byte(`{"type": "object", "properties": {"lane": {"type": "string"}}}`), 0o644))
	require.NoError(t, skillgraph.Save(fieldsDir, fieldsSkill))
	tests = append(tests, &testCase{
		_id:         "pass-format-3-schema-field",
		_expected:   "",
		_shouldPass: true,

		file: fieldsModel,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			listPath := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(listPath, []byte(test.file+"\n"), 0o644))

			var out bytes.Buffer
			rc := run([]string{listPath}, &out)

			assert.Equalf(t, test._shouldPass, rc == exitClean, "out = %s", out.String())
			assert.Contains(t, out.String(), test._expected)
		})
	}
}
