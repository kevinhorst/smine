package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeEval(t *testing.T, dir, name, date string) {
	t.Helper()
	content := `{"eval": {"skill": "demo", "date": "` + date + `"}, "runs": [], "totals": []}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestLoadForSkillSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	writeEval(t, skillDir, "a.json", "2026-07-01")
	writeEval(t, skillDir, "b.json", "2026-07-10")

	files, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, files, 2)
	assert.Equal(t, "2026-07-10", files[0].Eval.Date)
	assert.Equal(t, "2026-07-01", files[1].Eval.Date)
	assert.NotEmpty(t, files[0].SourcePath())
}

func TestLoadForSkillMalformedFileInErrors(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	writeEval(t, skillDir, "good.json", "2026-07-01")
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "bad.json"), []byte("{nope"), 0o644))

	files, loadErrors := LoadForSkill(dir, "demo")
	require.Len(t, files, 1)
	require.Len(t, loadErrors, 1)
	assert.Contains(t, loadErrors[0], "bad.json")
}

func TestLoadForSkillMissingDirIsEmpty(t *testing.T) {
	files, loadErrors := LoadForSkill(t.TempDir(), "demo")
	assert.Empty(t, files)
	assert.Empty(t, loadErrors)
}

func TestManifestStub(t *testing.T) {
	stub, err := ManifestStub("evals", []string{"examples/demo/input.md"}, "skills/demo/SKILL.md", "demo")
	require.NoError(t, err)

	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(stub), &manifest))
	assert.Equal(t, []any{"examples/demo/input.md"}, manifest["inputs"])
	for _, key := range []string{"skill", "skillMd", "inputs", "runs", "output"} {
		assert.Contains(t, manifest, key)
	}
	assert.Equal(t, "demo", manifest["skill"])
}
