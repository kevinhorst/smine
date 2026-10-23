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

const testIndex = `{"entries": [
	{"id": "ACTION-REVIEW-TEST-001"},
	{"id": "ACTION-REVIEW-TEST-002"},
	{"id": "ACTION-REVIEW-SPEC-001"},
	{"id": "RULE-COMMIT-001"}
]}`

const testSkillMd = `---
name: mapskill
version: 1.0
acdsl-context: ACTION-REVIEW-TEST-*, ACTION-REVIEW-SPEC-001
---
# Map skill
`

// saveModel imports testSkillMd into <tmp>/mapskill/, saves its model file and
// removes the markdown source, so only the model remains; returns the dir.
func saveModel(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "mapskill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := strings.Replace(testSkillMd, "version: 1.0\n", "description: map fixture. Trigger on nothing.\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n", 1) + "\n## Run\n\n**SKILL-MAPSKILL-RUN-001** `[step]` **Act**\n\nACT now.\n"
	changelog := `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillgraph.FileManifest), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(changelog), 0o644))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	require.NoError(t, os.Remove(filepath.Join(dir, skillgraph.FileManifest)))
	require.NoError(t, os.Remove(filepath.Join(dir, "changelog.json")))
	return dir
}

func TestCheckMap_siblingModel(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		dir string
	}

	tests := make([]*testCase, 0)

	// pass-model-path-rendered
	tests = append(tests, &testCase{
		_id:         "pass-model-path-rendered",
		_expected:   "",
		_shouldPass: true,

		dir: saveModel(t),
	})

	// fail-model-not-loading-is-one-violation
	brokenDir := saveModel(t)
	brokenModel := skillgraph.ModelPath(brokenDir)
	model, err := os.ReadFile(brokenModel)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(brokenModel, append([]byte("// note\n"), model...), 0o644))
	tests = append(tests, &testCase{
		_id:         "fail-model-not-loading-is-one-violation",
		_expected:   filepath.Join(brokenDir, "context-map.json") + ": sibling SKILL.md unreadable: ",
		_shouldPass: false,

		dir: brokenDir,
	})

	indexPath := filepath.Join(t.TempDir(), "context.json")
	require.NoError(t, os.WriteFile(indexPath, []byte(testIndex), 0o644))

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			mapPath := filepath.Join(test.dir, "context-map.json")
			require.NoError(t, os.WriteFile(mapPath, []byte(`{"schemaVersion": "1.0", "skill": "mapskill", "lanes": {"tests": ["ACTION-REVIEW-TEST-*"]}}`), 0o644))
			listPath := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(listPath, []byte(mapPath+"\n"), 0o644))

			var out bytes.Buffer
			rc := run([]string{listPath, "index=" + indexPath}, &out)

			assert.Equalf(t, test._shouldPass, rc == 0, "out = %s", out.String())
			assert.Contains(t, out.String(), test._expected)
			assert.LessOrEqual(t, strings.Count(out.String(), "\n"), 1)
		})
	}
}

func TestCheckMap(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		mapJson string
	}

	tests := make([]*testCase, 0)

	// pass-valid-map
	tests = append(tests, &testCase{
		_id:         "pass-valid-map",
		_expected:   "",
		_shouldPass: true,

		mapJson: `{"schemaVersion": "1.0", "skill": "mapskill", "lanes": {"tests": ["ACTION-REVIEW-TEST-*"], "spec": ["ACTION-REVIEW-SPEC-001"]}}`,
	})

	// fail-invalid-json
	tests = append(tests, &testCase{
		_id:         "fail-invalid-json",
		_expected:   "invalid JSON",
		_shouldPass: false,

		mapJson: `{"schemaVersion": `,
	})

	// fail-missing-schema-version
	tests = append(tests, &testCase{
		_id:         "fail-missing-schema-version",
		_expected:   "schemaVersion missing",
		_shouldPass: false,

		mapJson: `{"skill": "mapskill", "lanes": {}}`,
	})

	// fail-skill-dirname-mismatch
	tests = append(tests, &testCase{
		_id:         "fail-skill-dirname-mismatch",
		_expected:   `skill "other" does not match the directory name "mapskill"`,
		_shouldPass: false,

		mapJson: `{"schemaVersion": "1.0", "skill": "other", "lanes": {}}`,
	})

	// fail-bad-glob-grammar
	tests = append(tests, &testCase{
		_id:         "fail-bad-glob-grammar",
		_expected:   `token "review-test-*" is neither an entry ID nor a trailing-* glob`,
		_shouldPass: false,

		mapJson: `{"schemaVersion": "1.0", "skill": "mapskill", "lanes": {"tests": ["review-test-*"]}}`,
	})

	// fail-unresolvable-glob
	tests = append(tests, &testCase{
		_id:         "fail-unresolvable-glob",
		_expected:   `glob "ACTION-NOSUCH-*" matches no context entry`,
		_shouldPass: false,

		mapJson: `{"schemaVersion": "1.0", "skill": "mapskill", "lanes": {"tests": ["ACTION-NOSUCH-*"]}}`,
	})

	// fail-lane-superset-of-frontmatter
	tests = append(tests, &testCase{
		_id:         "fail-lane-superset-of-frontmatter",
		_expected:   `lane "commits" selects entries outside the SKILL.md declaration: RULE-COMMIT-001`,
		_shouldPass: false,

		mapJson: `{"schemaVersion": "1.0", "skill": "mapskill", "lanes": {"commits": ["RULE-COMMIT-001"]}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "mapskill")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(testSkillMd), 0o644))
			mapPath := filepath.Join(dir, "context-map.json")
			require.NoError(t, os.WriteFile(mapPath, []byte(test.mapJson), 0o644))
			indexPath := filepath.Join(t.TempDir(), "context.json")
			require.NoError(t, os.WriteFile(indexPath, []byte(testIndex), 0o644))
			listPath := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(listPath, []byte(mapPath+"\n"), 0o644))

			var out bytes.Buffer
			rc := run([]string{listPath, "index=" + indexPath}, &out)

			assert.Equalf(t, test._shouldPass, rc == 0, "out = %s", out.String())
			assert.Contains(t, out.String(), test._expected)
		})
	}
}
