package evals

import (
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

func TestLoadForSkillSetupRunAndInterpretation(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "demo-2026-07-01")
	require.NoError(t, os.MkdirAll(evalDir, 0o755))
	content := `{"eval": {"skill": "demo", "date": "2026-07-01"}, "runs": [],
		"setup": {"runtimeS": 60, "run": {"desktop": {"wraps": "workflows/demo.js"}}}}`
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "eval.json"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "interpretation.md"), []byte("## Verdict\n\ndepth gap"), 0o644))

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 1)
	setup := dirs[0].Eval.Setup
	require.NotNil(t, setup)
	require.NotNil(t, setup.Run)
	require.NotNil(t, setup.Run.Desktop)
	assert.Equal(t, "workflows/demo.js", setup.Run.Desktop.Wraps)
	assert.Equal(t, "## Verdict\n\ndepth gap", dirs[0].Interpretation)

	// setup without run block, no interpretation file
	writeEvalDir(t, dir, "demo-2026-07-02", "2026-07-02")
	dirs, loadErrors = LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 2)
	assert.Nil(t, dirs[0].Eval.Setup)
	assert.Empty(t, dirs[0].Interpretation)
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

func TestLoadForSkillV23RunFields(t *testing.T) {
	dir := t.TempDir()
	v23 := `{"schemaVersion": "2.3", "eval": {"skill": "demo", "date": "2026-09-06"},
	  "runs": [
	    {"id": "r1", "model": {"id": "sonnet"}, "variant": {"name": "sonnet-high"}, "transcript": "sess-abc", "invalid": false},
	    {"id": "r2", "model": {"id": "sonnet"}, "variant": {"name": "sonnet-low"}, "invalid": true, "invalidReason": "rounds-incomplete"}
	  ],
	  "metrics": [], "metricValues": [], "rubric": [], "scores": [], "probes": [], "totals": []}`
	v23Dir := filepath.Join(dir, "demo-2026-09-06")
	require.NoError(t, os.MkdirAll(v23Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(v23Dir, "eval.json"), []byte(v23), 0o644))

	// A pre-2.3 dir lacks the new keys and unmarshals to zero values.
	writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 2)

	assert.Equal(t, "sess-abc", dirs[0].Eval.Runs[0].Transcript)
	assert.False(t, dirs[0].Eval.Runs[0].Invalid)
	assert.True(t, dirs[0].Eval.Runs[1].Invalid)
	assert.Equal(t, "rounds-incomplete", dirs[0].Eval.Runs[1].InvalidReason)

	assert.Empty(t, dirs[1].Eval.Runs, "pre-2.3 fixture has no runs")
}

func TestLoadForSkillV24Fields(t *testing.T) {
	dir := t.TempDir()
	v24 := `{"schemaVersion": "2.4",
	  "eval": {"skill": "demo", "date": "2026-09-07",
	    "notes": "long prose",
	    "notesBullets": ["interrupted by the session limit", "cost covers the headless portion only"]},
	  "runs": [{"id": "r1", "model": {"id": "opus"}, "variant": {"name": "opus-high"}}],
	  "golden": {"goldenId": "g1", "goldenFile": "eval/golden/g1.json", "goldenVersion": 5, "total": 245,
	    "perRun": [{"runId": "r1", "matched": [], "missed": [], "falsePositives": [], "misclassified": []}]},
	  "metrics": [], "metricValues": [], "rubric": [], "scores": [], "probes": [], "totals": []}`
	v24Dir := filepath.Join(dir, "demo-2026-09-07")
	require.NoError(t, os.MkdirAll(v24Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(v24Dir, "eval.json"), []byte(v24), 0o644))

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 1)

	loaded := dirs[0].Eval
	assert.Equal(t, []string{"interrupted by the session limit", "cost covers the headless portion only"}, loaded.Eval.NotesBullets)
	require.NotNil(t, loaded.Golden)
	assert.Equal(t, 5, loaded.Golden.GoldenVersion)
	require.Len(t, loaded.Golden.PerRun, 1)
}

func writeDefinition(t *testing.T, skillDir, defJson, goldenRel, goldenJson string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "eval"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "eval", "definition.json"), []byte(defJson), 0o644))
	if goldenRel != "" {
		goldenPath := filepath.Join(skillDir, goldenRel)
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(goldenJson), 0o644))
	}
}

func TestLoadDefinitionMissingIsNil(t *testing.T) {
	def, err := LoadDefinition(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, def)
}

func TestLoadDefinitionMalformedIsError(t *testing.T) {
	skillDir := t.TempDir()
	writeDefinition(t, skillDir, "{ not json", "", "")

	def, err := LoadDefinition(skillDir)
	require.Error(t, err)
	assert.Nil(t, def)
}

func writeScoring(t *testing.T, skillDir, scoringJson string) {
	t.Helper()
	scoringPath := filepath.Join(skillDir, "eval", "golden", "scoring.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(scoringPath), 0o755))
	require.NoError(t, os.WriteFile(scoringPath, []byte(scoringJson), 0o644))
}

func TestLoadDefinitionDecodesAndSummarizesGolden(t *testing.T) {
	skillDir := t.TempDir()
	def := `{"schemaVersion": "4.0", "skill": "demo",
		"scoring": "eval/golden/scoring.json",
		"budgets": {"armUsd": 30, "armTimeoutS": 10800, "windowUsd": 150},
		"findings": {"glob": "round-*/review.json", "arrayKey": "ranked_plan", "claimKeys": ["claim", "finding"]},
		"invocation": {"args": [{"name": "scope", "value": "branch[{base}]"}]},
		"environment": {"targetRepo": "smine", "overlay": "eval/golden/g1.overlay.patch", "contextIndex": "context/review-index.json",
			"prebuild": {"command": ["make", "build-audit"], "outputs": ["bin/acdsl"]},
			"range": {"baseSha": "b4981b88", "headSha": "a342811b", "baseLabel": "1.1.4", "headLabel": "1.2.1", "excludePaths": ["plans/"]}},
		"goldens": [{"id": "g1", "file": "eval/golden/g1.json", "input": "demo range", "scope": "full", "produce": {"seedFile": "eval/golden/g1.seed.json"}}]}`
	golden := `{"range": {"baseLabel": "1.1.4", "headLabel": "1.2.1"},
		"findings": [{"severity": "BLOCKER"}, {"severity": "MAJOR"}, {"severity": "NIT"}]}`
	writeDefinition(t, skillDir, def, filepath.Join("eval", "golden", "g1.json"), golden)
	writeScoring(t, skillDir, `{"judgeGuide": "same defect", "minutes": {"BLOCKER": 120, "MAJOR": 90, "NIT": 10}, "escapeMultiplier": {"BLOCKER": 5, "MAJOR": 3, "NIT": 1}, "triageLine": "MINOR", "fixFactor": {}, "thresholds": {"review_score": 0.6}}`)

	loaded, err := LoadDefinition(skillDir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "4.0", loaded.SchemaVersion)
	expectedArgs := []DefinitionArg{{Name: "scope", Value: "branch[{base}]"}}
	assert.Equal(t, expectedArgs, loaded.Invocation.Args)
	expectedRange := DefinitionRange{
		BaseLabel:    "1.1.4",
		BaseSha:      "b4981b88",
		ExcludePaths: []string{"plans/"},
		HeadLabel:    "1.2.1",
		HeadSha:      "a342811b",
	}
	assert.Equal(t, expectedRange, loaded.Environment.Range)
	assert.Equal(t, "smine", loaded.Environment.TargetRepo)
	assert.Equal(t, "eval/golden/g1.overlay.patch", loaded.Environment.Overlay)
	assert.Equal(t, "context/review-index.json", loaded.Environment.ContextIndex)
	expectedPrebuild := &DefinitionPrebuild{Command: []string{"make", "build-audit"}, Outputs: []string{"bin/acdsl"}}
	assert.Equal(t, expectedPrebuild, loaded.Environment.Prebuild)
	assert.InDelta(t, 30, loaded.Budgets.ArmUsd, 1e-9)
	assert.Equal(t, 10800, loaded.Budgets.ArmTimeoutS)
	assert.Equal(t, "ranked_plan", loaded.Findings.ArrayKey)
	assert.Equal(t, []string{"claim", "finding"}, loaded.Findings.ClaimKeys)
	require.NotNil(t, loaded.Scoring)
	assert.Equal(t, "same defect", loaded.Scoring.JudgeGuide)
	assert.InDelta(t, 0.6, loaded.Scoring.Thresholds["review_score"], 1e-9)
	assert.Equal(t, "eval/golden/g1.seed.json", loaded.Goldens[0].Produce.SeedFile)
	assert.Equal(t, "full", loaded.Goldens[0].Scope)

	require.Len(t, loaded.Goldens, 1)
	summary := loaded.Goldens[0]
	assert.Equal(t, 3, summary.FindingCount)
	assert.InDelta(t, 220, summary.OracleMinutes, 1e-9, "120 + 90 + 10")
	assert.Equal(t, "1.1.4", summary.BaseLabel)
	assert.Equal(t, "1.2.1", summary.HeadLabel)
	assert.Equal(t, 1, summary.SeverityCounts["MAJOR"])
}

func TestLoadDefinitionDecodesSchema31(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Definition

		skillDir string
	}

	tests := make([]*testCase, 0)

	// schema-31-keys-decoded
	fullDir := t.TempDir()
	writeDefinition(t, fullDir, `{"schemaVersion": "4.0", "skill": "demo",
		"invocation": {"text": "plan.md"},
		"environment": {"targetRepo": "claude-configs", "checkoutRef": "a36dd5b4", "overlay": "eval/plan.overlay.patch"},
		"budgets": {"armUsd": 30, "armTimeoutS": 7200, "windowUsd": 150},
		"prerequisites": ["eval/plan.overlay.patch"],
		"check": ["make", "audit"],
		"reference": "9e49be5e",
		"scorer": {"command": ["go", "run", "./cmd/score"]},
		"model": "opus",
		"effort": "high",
		"replicas": 3}`, "", "")
	tests = append(tests, &testCase{
		_id: "schema-31-keys-decoded",
		_expected: &Definition{
			Budgets:       DefinitionBudgets{ArmTimeoutS: 7200, ArmUsd: 30, WindowUsd: 150},
			Check:         []string{"make", "audit"},
			Effort:        "high",
			Environment:   DefinitionEnvironment{CheckoutRef: "a36dd5b4", Overlay: "eval/plan.overlay.patch", TargetRepo: "claude-configs"},
			Invocation:    DefinitionInvocation{Text: "plan.md"},
			Model:         "opus",
			Prerequisites: []string{"eval/plan.overlay.patch"},
			Reference:     "9e49be5e",
			Replicas:      3,
			SchemaVersion: "4.0",
			Scorer:        DefinitionScorer{Command: []string{"go", "run", "./cmd/score"}},
			Skill:         "demo",
		},

		skillDir: fullDir,
	})

	// absent-31-keys-zero
	minimalDir := t.TempDir()
	writeDefinition(t, minimalDir, `{"schemaVersion": "4.0", "skill": "demo",
		"invocation": {"text": "plan.md"},
		"budgets": {"armUsd": 2, "armTimeoutS": 900, "windowUsd": 150}}`, "", "")
	tests = append(tests, &testCase{
		_id: "absent-31-keys-zero",
		_expected: &Definition{
			Budgets:       DefinitionBudgets{ArmTimeoutS: 900, ArmUsd: 2, WindowUsd: 150},
			Invocation:    DefinitionInvocation{Text: "plan.md"},
			SchemaVersion: "4.0",
			Skill:         "demo",
		},

		skillDir: minimalDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			loaded, err := LoadDefinition(test.skillDir)
			require.NoError(t, err)
			assert.Equal(t, test._expected, loaded)
		})
	}
}

// TestLoadDefinitionCommittedDefinitions proves every committed eval definition loads; its content is the skill's own.
func TestLoadDefinitionCommittedDefinitions(t *testing.T) {
	definitions, err := filepath.Glob(filepath.Join("..", "..", "skills", "*", "*", "eval", "definition.json"))
	require.NoError(t, err)
	require.NotEmpty(t, definitions)

	for _, definition := range definitions {
		skillDir := filepath.Dir(filepath.Dir(definition))
		t.Run(filepath.Base(skillDir), func(t *testing.T) {
			loaded, err := LoadDefinition(skillDir)
			require.NoError(t, err)
			require.NotNil(t, loaded, "definition missing")
			assert.Equal(t, filepath.Base(skillDir), loaded.Skill)
			assert.NotEmpty(t, loaded.SchemaVersion)
		})
	}
}

func TestLoadDefinitionParsesGoldenProduction(t *testing.T) {
	skillDir := t.TempDir()
	def := `{"schemaVersion": "4.0", "skill": "demo",
		"goldens": [{"id": "g1", "file": "eval/golden/g1.json", "input": "x"}]}`
	golden := `{"goldenVersion": 2, "findings": [{"severity": "BLOCKER"}],
		"production": {
		  "contributions": [
		    {"version": 1, "findings": 1, "newFindings": 1, "source": "workflow session", "costUsd": null, "wallS": null, "outputTokens": null, "subagentTokens": 59303366},
		    {"version": 2, "findings": 1, "newFindings": 0, "source": "evals/demo/2026-09-05", "costUsd": 324.03, "wallS": 13601, "outputTokens": 415507, "subagentTokens": null}
		  ],
		  "totals": {"costUsd": 324.03, "wallS": 13601, "outputTokens": 415507, "subagentTokens": 59303366}}}`
	writeDefinition(t, skillDir, def, filepath.Join("eval", "golden", "g1.json"), golden)

	loaded, err := LoadDefinition(skillDir)
	require.NoError(t, err)
	require.Len(t, loaded.Goldens, 1)
	production := loaded.Goldens[0].Production
	require.NotNil(t, production)
	require.Len(t, production.Contributions, 2)
	assert.Nil(t, production.Contributions[0].CostUsd, "v1 session cost unknown stays nil")
	require.NotNil(t, production.Contributions[0].SubagentTokens)
	assert.Equal(t, int64(59303366), *production.Contributions[0].SubagentTokens)
	require.NotNil(t, production.Contributions[1].CostUsd)
	assert.InDelta(t, 324.03, *production.Contributions[1].CostUsd, 1e-9)
	require.NotNil(t, production.Totals.WallS)
	assert.Equal(t, int64(13601), *production.Totals.WallS)

	// a golden without the block leaves Production nil
	writeDefinition(t, skillDir, def, filepath.Join("eval", "golden", "g1.json"), `{"goldenVersion": 2, "findings": [{"severity": "BLOCKER"}]}`)
	loaded, err = LoadDefinition(skillDir)
	require.NoError(t, err)
	assert.Nil(t, loaded.Goldens[0].Production)
}

func TestLoadDefinitionUnpricedSeverityAddsNoMinutes(t *testing.T) {
	skillDir := t.TempDir()
	def := `{"schemaVersion": "4.0", "skill": "demo", "scoring": "eval/golden/scoring.json",
		"goldens": [{"id": "g1", "file": "eval/golden/g1.json", "input": "x"}]}`
	golden := `{"findings": [{"severity": "BLOCKER"}, {"severity": "MINOR"}]}`
	writeDefinition(t, skillDir, def, filepath.Join("eval", "golden", "g1.json"), golden)
	writeScoring(t, skillDir, `{"judgeGuide": "x", "minutes": {"BLOCKER": 120}, "escapeMultiplier": {"BLOCKER": 5}, "triageLine": "MINOR", "fixFactor": {}, "thresholds": {}}`)

	loaded, err := LoadDefinition(skillDir)
	require.NoError(t, err)
	require.Len(t, loaded.Goldens, 1)
	assert.InDelta(t, 120, loaded.Goldens[0].OracleMinutes, 1e-9, "unpriced MINOR adds 0")
	assert.Equal(t, 2, loaded.Goldens[0].FindingCount)
}

func TestScoring_IsHumanSide(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		severity string
	}

	scoring := &Scoring{TriageLine: "MINOR"}
	tests := make([]*testCase, 0)

	// at-line-human
	tests = append(tests, &testCase{
		_id:       "at-line-human",
		_expected: true,

		severity: "MINOR",
	})

	// above-line-human
	tests = append(tests, &testCase{
		_id:       "above-line-human",
		_expected: true,

		severity: "BLOCKER",
	})

	// below-line-not
	tests = append(tests, &testCase{
		_id:       "below-line-not",
		_expected: false,

		severity: "NIT",
	})

	// unknown-severity-not
	tests = append(tests, &testCase{
		_id:       "unknown-severity-not",
		_expected: false,

		severity: "blocker",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, scoring.IsHumanSide(test.severity))
		})
	}
}

func TestScoring_SeverityCost(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedMinutes    float64
		_expectedMultiplier float64
		_shouldPass         bool

		scoring  *Scoring
		severity string
	}

	priced := &Scoring{
		EscapeMultiplier: map[string]float64{"BLOCKER": 5, "NIT": 1},
		Minutes:          map[string]float64{"BLOCKER": 120, "MAJOR": 90},
	}
	tests := make([]*testCase, 0)

	// priced
	tests = append(tests, &testCase{
		_id:                 "priced",
		_expectedMinutes:    120,
		_expectedMultiplier: 5,
		_shouldPass:         true,

		scoring:  priced,
		severity: "BLOCKER",
	})

	// missing-minutes
	tests = append(tests, &testCase{
		_id:                 "missing-minutes",
		_expectedMinutes:    0,
		_expectedMultiplier: 0,
		_shouldPass:         false,

		scoring:  priced,
		severity: "NIT",
	})

	// missing-multiplier
	tests = append(tests, &testCase{
		_id:                 "missing-multiplier",
		_expectedMinutes:    0,
		_expectedMultiplier: 0,
		_shouldPass:         false,

		scoring:  priced,
		severity: "MAJOR",
	})

	// nil-scoring
	tests = append(tests, &testCase{
		_id:                 "nil-scoring",
		_expectedMinutes:    0,
		_expectedMultiplier: 0,
		_shouldPass:         false,

		scoring:  nil,
		severity: "BLOCKER",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			minutes, multiplier, err := test.scoring.SeverityCost(test.severity)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.InDelta(t, test._expectedMinutes, minutes, 1e-9)
			assert.InDelta(t, test._expectedMultiplier, multiplier, 1e-9)
		})
	}
}

func TestLoadDefinitionGoldenWithoutShapeIsCountOnly(t *testing.T) {
	skillDir := t.TempDir()
	def := `{"schemaVersion": "4.0", "skill": "demo",
		"goldens": [{"id": "g1", "file": "eval/golden/missing.json", "input": "x"}]}`
	writeDefinition(t, skillDir, def, "", "")

	loaded, err := LoadDefinition(skillDir)
	require.NoError(t, err)
	require.Len(t, loaded.Goldens, 1)
	assert.Equal(t, 0, loaded.Goldens[0].FindingCount)
	assert.Equal(t, "", loaded.Goldens[0].BaseLabel)
	assert.Nil(t, loaded.Scoring)
}

func TestLoadDefinitionMissingScoringIsError(t *testing.T) {
	skillDir := t.TempDir()
	def := `{"schemaVersion": "4.0", "skill": "demo", "scoring": "eval/golden/scoring.json",
		"goldens": [{"id": "g1", "file": "eval/golden/g1.json", "input": "x"}]}`
	writeDefinition(t, skillDir, def, "", "")

	loaded, err := LoadDefinition(skillDir)
	require.Error(t, err)
	assert.Nil(t, loaded)
}

func TestLoadScoring(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedFixFactor map[string]float64
		_shouldPass        bool

		path string
	}

	tests := make([]*testCase, 0)

	// flat-fix-factor
	flatDir := t.TempDir()
	writeScoring(t, flatDir, `{"judgeGuide": "x", "minutes": {}, "escapeMultiplier": {}, "triageLine": "MINOR", "thresholds": {},
		"fixFactor": {"BLOCKER": 0.5, "MAJOR": 0.4, "MINOR": 0.25, "NIT": 0.1, "INFO": 0.1}}`)
	tests = append(tests, &testCase{
		_id:                "flat-fix-factor",
		_expectedFixFactor: map[string]float64{"BLOCKER": 0.5, "MAJOR": 0.4, "MINOR": 0.25, "NIT": 0.1, "INFO": 0.1},
		_shouldPass:        true,

		path: filepath.Join(flatDir, "eval", "golden", "scoring.json"),
	})

	// route-keyed-refused
	routeDir := t.TempDir()
	writeScoring(t, routeDir, `{"judgeGuide": "x", "minutes": {}, "escapeMultiplier": {}, "triageLine": "MINOR", "thresholds": {},
		"fixFactor": {"auto-fix": {"BLOCKER": 0.5}, "runner-fix": {"BLOCKER": 0.5}}}`)
	tests = append(tests, &testCase{
		_id:                "route-keyed-refused",
		_expectedFixFactor: nil,
		_shouldPass:        false,

		path: filepath.Join(routeDir, "eval", "golden", "scoring.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			scoring, err := LoadScoring(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedFixFactor, fixFactorOf(scoring))
		})
	}
}

// fixFactorOf is the fix factor of a loaded scoring, nil when none loaded.
func fixFactorOf(scoring *Scoring) map[string]float64 {
	if scoring == nil {
		return nil
	}
	return scoring.FixFactor
}

func TestLoadForSkillNestedRuns(t *testing.T) {
	dir := t.TempDir()
	writeEvalDir(t, dir, "demo-2026-07-01", "2026-07-01")
	writeEvalDir(t, dir, filepath.Join("demo", "2026-07-10-abc123"), "2026-07-10")
	writeEvalDir(t, dir, filepath.Join("demo", "2026-07-10-abc123-2"), "2026-07-10")
	writeEvalDir(t, dir, filepath.Join("demo", "nonsense"), "2026-07-11")
	writeEvalDir(t, dir, filepath.Join("demo", "2026-07-12"), "2026-07-12")
	writeEvalDir(t, dir, filepath.Join("demo", "ab-2026-10-02-abc123"), "2026-10-02")

	dirs, loadErrors := LoadForSkill(dir, "demo")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 3)
	assert.Equal(t, filepath.Join("demo", "2026-07-10-abc123-2"), dirs[0].Dir)
	assert.Equal(t, filepath.Join("demo", "2026-07-10-abc123"), dirs[1].Dir)
	assert.Equal(t, "demo-2026-07-01", dirs[2].Dir)
}

func writeV21EvalDir(t *testing.T, dir, name, statsContent string) {
	t.Helper()
	evalDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(evalDir, 0o755))
	eval := `{"schemaVersion": "2.1", "eval": {"skill": "railroad-review", "date": "2026-09-01"},
		"runs": [{"id": "opus-high-r1", "model": {"id": "opus", "effort": "high"}, "variant": {"name": "opus-high", "disable": []}}],
		"totals": [],
		"golden": {"goldenId": "g1", "goldenFile": "eval/golden/g1.json", "total": 4,
			"perRun": [{"runId": "opus-high-r1", "matched": [{"goldenId": "G1", "findingId": "critical-BLK-1"}],
				"missed": ["G2"], "falsePositives": [], "misclassified": [],
				"uncataloged": [{"findingId": "extra-1", "reason": "valid, not in golden"}]}]},
		"verdict": {"status": "pass", "reason": "opus-high best by recall", "bestArm": "opus-high",
			"perRun": [{"runId": "opus-high-r1", "pass": true, "failedThresholds": []}]}}`
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "eval.json"), []byte(eval), 0o644))
	if statsContent != "" {
		require.NoError(t, os.WriteFile(filepath.Join(evalDir, "stats.json"), []byte(statsContent), 0o644))
	}
}

func TestLoadForSkillV21(t *testing.T) {
	dir := t.TempDir()
	stats := `{"schemaVersion": "1.0", "baseline": "opus-high", "notes": "no p-values",
		"groups": [{"metricId": "recall", "arm": "opus-high", "n": 1, "mean": 0.75, "stddev": 0, "variance": 0}],
		"comparisons": []}`
	writeV21EvalDir(t, dir, filepath.Join("railroad-review", "2026-09-01-abc123"), stats)

	dirs, loadErrors := LoadForSkill(dir, "railroad-review")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 1)

	loaded := dirs[0]
	require.NotNil(t, loaded.Eval.Golden)
	assert.Equal(t, 4, loaded.Eval.Golden.Total)
	require.Len(t, loaded.Eval.Golden.PerRun, 1)
	assert.Equal(t, []string{"G2"}, loaded.Eval.Golden.PerRun[0].Missed)
	expectedUncataloged := []GoldenFalsePositive{{FindingId: "extra-1", Reason: "valid, not in golden"}}
	assert.Equal(t, expectedUncataloged, loaded.Eval.Golden.PerRun[0].Uncataloged)
	require.NotNil(t, loaded.Eval.Verdict)
	assert.Equal(t, "pass", loaded.Eval.Verdict.Status)
	assert.Equal(t, "opus-high", loaded.Eval.Verdict.BestArm)
	require.NotNil(t, loaded.Stats)
	assert.Equal(t, "opus-high", loaded.Stats.Baseline)
	require.Len(t, loaded.Stats.Groups, 1)
	assert.InDelta(t, 0.75, loaded.Stats.Groups[0].Mean, 1e-9)
}

func TestLoadForSkillV21AbsentSidecars(t *testing.T) {
	dir := t.TempDir()
	writeV21EvalDir(t, dir, filepath.Join("railroad-review", "2026-09-02-def456"), "")

	dirs, loadErrors := LoadForSkill(dir, "railroad-review")
	require.Empty(t, loadErrors)
	require.Len(t, dirs, 1)
	assert.Nil(t, dirs[0].Stats, "absent stats.json leaves Stats nil")
	assert.NotNil(t, dirs[0].Eval.Golden, "golden still parsed from eval.json")
}

func TestLoadForSkillV21MalformedStats(t *testing.T) {
	dir := t.TempDir()
	writeV21EvalDir(t, dir, filepath.Join("railroad-review", "2026-09-03-aaa111"), "{ not json")

	dirs, loadErrors := LoadForSkill(dir, "railroad-review")
	require.Len(t, dirs, 1, "malformed stats.json never drops the page")
	assert.Nil(t, dirs[0].Stats)
	require.Len(t, loadErrors, 1)
	assert.Contains(t, loadErrors[0], "stats.json")
}

func TestMetricValue_Number(t *testing.T) {
	type testCase struct {
		_id       string
		_expected float64
		_ok       bool

		value any
	}

	tests := make([]*testCase, 0)

	// float
	tests = append(tests, &testCase{
		_id:       "float",
		_expected: 1.5,
		_ok:       true,

		value: 1.5,
	})

	// bool-true
	tests = append(tests, &testCase{
		_id:       "bool-true",
		_expected: 1,
		_ok:       true,

		value: true,
	})

	// bool-false
	tests = append(tests, &testCase{
		_id:       "bool-false",
		_expected: 0,
		_ok:       true,

		value: false,
	})

	// nil
	tests = append(tests, &testCase{
		_id:       "nil",
		_expected: 0,
		_ok:       false,

		value: nil,
	})

	// string
	tests = append(tests, &testCase{
		_id:       "string",
		_expected: 0,
		_ok:       false,

		value: "n/a",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			number, ok := MetricValue{Value: test.value}.Number()
			assert.Equal(t, test._expected, number)
			assert.Equal(t, test._ok, ok)
		})
	}
}

func TestLoadArmRuns(t *testing.T) {
	assert.Nil(t, loadArmRuns(filepath.Join(t.TempDir(), "armruns.jsonl")), "absent file")

	path := filepath.Join(t.TempDir(), "armruns.jsonl")
	rows := `{"id":"r1","arm":"sonnet-5-high","agent_tokens":427765,"touched_files":["a.go","b.go"],"judge":{"matched":[{"goldenId":"GV6-252"}]}}
not json
{"id":"r2","arm":"sonnet-5-high","output_tokens":189603,"judge":{"matched":[]}}
`
	require.NoError(t, os.WriteFile(path, []byte(rows), 0o644))

	runs := loadArmRuns(path)
	require.Len(t, runs, 2, "malformed line skipped")
	assert.Equal(t, "r1", runs[0].Id)
	require.NotNil(t, runs[0].AgentTokens)
	assert.InDelta(t, 427765.0, *runs[0].AgentTokens, 0.01)
	assert.Equal(t, "GV6-252", runs[0].Judge.Matched[0].GoldenId)
	assert.Equal(t, []string{"a.go", "b.go"}, runs[0].TouchedFiles, "touched_files parsed")
	assert.Nil(t, runs[1].AgentTokens)
	require.NotNil(t, runs[1].OutputTokens)
	assert.InDelta(t, 189603.0, *runs[1].OutputTokens, 0.01)
	assert.Nil(t, runs[1].TouchedFiles, "absent field stays nil")
}
