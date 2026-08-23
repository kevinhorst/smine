package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeEvalDir(t *testing.T, dir, name, date string) string {
	t.Helper()
	evalDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(evalDir, 0o755))
	content := `{"eval": {"skill": "demo", "date": "` + date + `"}, "runs": [], "totals": []}`
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "eval.json"), []byte(content), 0o644))
	return evalDir
}

func TestLoadForSkillSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")
	writeEvalDir(t, dir, "demo-2026-07-10", "2026-07-10")

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 2)
	assert.Equal(t, "2026-07-10", dirs[0].Eval.Eval.Date)
	assert.Equal(t, "2026-07-01", dirs[1].Eval.Eval.Date)
	assert.NotEmpty(t, dirs[0].Eval.SourcePath())
}

func TestLoadForSkillDirNaming(t *testing.T) {
	dir := t.TempDir()
	writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")
	writeEvalDir(t, dir, "demo-2026-07-01-2", "2026-07-01")
	writeEvalDir(t, dir, "demo-abc123", "2026-07-02")
	writeEvalDir(t, dir, "demo-batch-2026-07-01", "2026-07-01")
	writeEvalDir(t, dir, "demography-2026-07-01", "2026-07-01")

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 3)
	assert.Equal(t, "demo-abc123", dirs[0].Dir)
	assert.Equal(t, "demo-2026-07-01-2", dirs[1].Dir)
	assert.Equal(t, "demo-2026-07-01", dirs[2].Dir)
}

func TestLoadForSkillDeltasAndFiles(t *testing.T) {
	dir := t.TempDir()
	evalDir := writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")
	deltas := `[{"axis": "self", "dimension": "context", "arm": "off", "delta_pct": 1.4, "n": 4}]`
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "deltas.json"), []byte(deltas), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(evalDir, "runs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "runs", "r1.md"), []byte("out"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(evalDir, "other"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "other", "x.md"), []byte("x"), 0o644))

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 1)
	require.Len(t, dirs[0].Deltas, 1)
	assert.Equal(t, "self", dirs[0].Deltas[0].Axis)
	assert.InDelta(t, 1.4, dirs[0].Deltas[0].DeltaPct, 0.001)
	assert.Equal(t, []string{"deltas.json", "eval.json", filepath.Join("runs", "r1.md")}, dirs[0].Files)
}

func TestLoadForSkillMissingDeltasIsFine(t *testing.T) {
	dir := t.TempDir()
	writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 1)
	assert.Empty(t, dirs[0].Deltas)
}

func TestLoadForSkillMalformedFilesInErrors(t *testing.T) {
	dir := t.TempDir()
	evalDir := writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "deltas.json"), []byte("{nope"), 0o644))
	brokenDir := filepath.Join(dir, "demo-2026-07-02")
	require.NoError(t, os.MkdirAll(brokenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(brokenDir, "eval.json"), []byte("{nope"), 0o644))
	emptyDir := filepath.Join(dir, "demo-2026-07-03")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Len(t, dirs, 1)
	require.Len(t, loadErrors, 3)
	assert.Contains(t, loadErrors[0], "deltas.json")
	assert.Contains(t, loadErrors[1], "demo-2026-07-02/eval.json")
	assert.Contains(t, loadErrors[2], "demo-2026-07-03")
}

func TestLoadForSkillMissingRootIsEmpty(t *testing.T) {
	dirs, loadErrors := LoadForSkill(filepath.Join(t.TempDir(), "absent"), "demo")
	assert.Empty(t, dirs)
	assert.Empty(t, loadErrors)
}

func TestLoadForSkillLegacyAndV2(t *testing.T) {
	dir := t.TempDir()
	writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")
	v2 := `{"schemaVersion": "2.0", "eval": {"skill": "demo", "date": "2026-08-18"},
	  "runs": [{"id": "r1", "model": {"id": "fable"}, "variant": {"name": "no-tpl", "disable": ["SKILL-DEMO-TPL-*"]}}],
	  "metrics": [{"id": "diff_lines", "label": "Diff lines", "unit": "lines", "direction": "lower", "source": "probe"}],
	  "metricValues": [{"metricId": "diff_lines", "runId": "r1", "value": 42}],
	  "rubric": [{"id": "SKILL-DEMO-A-001", "axis": "self", "phase": "step", "rule": "Do the thing.", "source": "skills/demo/SKILL.md:3"}],
	  "scores": [{"ruleId": "SKILL-DEMO-A-001", "runId": "r1", "score": 0, "source": "agent", "justification": "not done", "evidence": ["line 4"]}],
	  "probes": [{"name": "context-injection", "ruleIds": ["SKILL-DEMO-A-001"], "result": "all runs identical"}],
	  "totals": [{"runId": "r1", "axis": "self", "raw": 3, "max": 4, "pct": 75}],
	  "sharedTotals": [{"runId": "r1", "axis": "self", "raw": 2, "max": 3, "pct": 66.7}]}`
	v2Dir := filepath.Join(dir, "demo-2026-08-18")
	require.NoError(t, os.MkdirAll(v2Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(v2Dir, "eval.json"), []byte(v2), 0o644))

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 2)
	assert.False(t, dirs[0].Eval.IsLegacy())
	assert.True(t, dirs[1].Eval.IsLegacy())
	assert.Equal(t, "no-tpl", dirs[0].Eval.Runs[0].Variant.Name)
	assert.Equal(t, "diff_lines", dirs[0].Eval.Metrics[0].Id)
	assert.Equal(t, float64(42), dirs[0].Eval.MetricValues[0].Value)
	assert.Equal(t, "self", dirs[0].Eval.SharedTotals[0].Axis)
	assert.Equal(t, "SKILL-DEMO-A-001", dirs[0].Eval.Rubric[0].Id)
	assert.Equal(t, "not done", dirs[0].Eval.Scores[0].Justification)
	assert.Equal(t, []string{"line 4"}, dirs[0].Eval.Scores[0].Evidence)
	assert.Equal(t, "context-injection", dirs[0].Eval.Probes[0].Name)
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
	assert.Equal(t, filepath.Join("evals", "demo-<date>", "eval.json"), manifest["output"])
}
