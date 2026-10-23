package skilleval

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
)

const (
	abTestDate    = "2026-10-02"
	abTestSkill   = "demo"
	abTestToken   = "eval-token"
	abTestVersion = "14.2.0"
)

// recordFixture is one arm of a fixture run dir: its run.json key and label,
// its result.json identity and its eval.json cost (nil writes null).
type recordFixture struct {
	costUsd      any
	key          string
	label        string
	layer        string
	skill        string
	skillVersion string
}

func (f *recordFixture) armId() string {
	return f.key + "-" + f.label
}

// abDefinition is an eval definition with an overlay and a $10 arm budget
// under the default $150 window.
func abDefinition() *evals.Definition {
	definition := &evals.Definition{
		Budgets:       evals.DefinitionBudgets{ArmTimeoutS: 600, ArmUsd: 10},
		Environment:   evals.DefinitionEnvironment{Overlay: "eval/overlay.patch", Range: evals.DefinitionRange{HeadSha: "a36dd5b4c0ffee"}},
		Invocation:    evals.DefinitionInvocation{Text: "run"},
		SchemaVersion: definitionSchema,
		Skill:         abTestSkill,
	}
	return definition
}

// abRecord is the record AbCollect reads from a fixture arm's result.json
// under runDir.
func abRecord(cost float64, label, runDir, skillVersion string) evals.AbRecord {
	record := evals.AbRecord{
		Measures:     map[string]float64{metricCostUsd: cost},
		Path:         filepath.Join(runDir, runsDir, label, resultFile),
		SkillVersion: skillVersion,
	}
	return record
}

// abSideKeys reads ab.json's baseline and variant keys; nil without a dir.
func abSideKeys(t *testing.T, abDir string) []string {
	t.Helper()
	if abDir == "" {
		return nil
	}

	file, err := loadAbFile(abDir)
	require.NoError(t, err)
	return []string{file.Baseline.Key, file.Variant.Key}
}

// armSummaries lists run.json's arms as "<key>" or "<key> <overlay file>",
// sorted; nil without run.json.
func armSummaries(t *testing.T, abDir string) []string {
	t.Helper()
	manifest, err := loadManifest(abDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	require.NoError(t, err)
	summaries := make([]string, 0, len(manifest.Arms))
	for _, arm := range manifest.Arms {
		summary := arm.Key
		if arm.OverlayPatch != "" {
			summary += " " + filepath.Base(arm.OverlayPatch)
		}
		summaries = append(summaries, summary)
	}
	slices.Sort(summaries)
	return summaries
}

// dirNames lists the entry names of dir; empty when dir is absent.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}
	}

	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// fileText is path's content; empty when absent.
func fileText(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

// readAbJson reads the A/B dir's ab.json.
func readAbJson(t *testing.T, abDir string) *evals.AbFile {
	t.Helper()
	file, err := loadAbFile(abDir)
	require.NoError(t, err)
	return file
}

// writeAbJson writes an ab.json skeleton with both side keys set.
func writeAbJson(t *testing.T, abDir, baselineKey, descriptor, layer, skill, variantKey string) {
	t.Helper()
	file := &evals.AbFile{
		Baseline:      evals.AbSide{Key: baselineKey, Records: []evals.AbRecord{}},
		Date:          abTestDate,
		Descriptor:    descriptor,
		Differences:   []evals.AbDifference{},
		Layer:         layer,
		Notes:         []string{},
		SchemaVersion: abSchemaVersion,
		Skill:         skill,
		Variant:       evals.AbSide{Key: variantKey, Records: []evals.AbRecord{}},
	}
	require.NoError(t, os.MkdirAll(abDir, 0o755))
	require.NoError(t, writeAbFile(abDir, file))
}

// writeRecordRunDir writes a run dir: run.json with one arm per fixture, each
// arm's result.json and, when folded, an eval.json carrying each arm's cost
// and a null step_miss_pct.
func writeRecordRunDir(t *testing.T, isFolded bool, records []recordFixture, runDir string) {
	t.Helper()
	arms := make([]ManifestArm, 0, len(records))
	values := make([]evals.MetricValue, 0, 2*len(records))
	for index := range records {
		record := &records[index]
		arm := ManifestArm{Id: record.armId(), Key: record.key, Label: record.label}
		arms = append(arms, arm)
		result := &ArmResult{Layer: record.layer, Skill: record.skill, SkillVersion: record.skillVersion}
		writeSmokeRecord(t, filepath.Join(runDir, runsDir, record.label, resultFile), result)
		cost := evals.MetricValue{MetricId: metricCostUsd, RunId: record.armId(), Value: record.costUsd}
		stepMiss := evals.MetricValue{MetricId: metricStepMissPct, Note: "no adherence.json", RunId: record.armId(), Value: nil}
		values = append(values, cost, stepMiss)
	}

	manifest := &RunManifest{Arms: arms, Date: abTestDate, Layer: LayerEval, Skill: abTestSkill}
	writeRunDir(t, runDir, manifest)
	if !isFolded {
		return
	}

	evalFile := &evals.EvalFile{MetricValues: values}
	require.NoError(t, writeJsonFile(filepath.Join(runDir, evalFileName), evalFile))
}

func TestInitAb(t *testing.T) {
	type testCase struct {
		_id                     string
		_expectedArms           []string
		_expectedDir            string
		_expectedEntries        []string
		_expectedErr            error
		_expectedKeys           []string
		_expectedVariantOverlay string

		evalsRoot string
		request   *AbInitRequest
	}

	home := newTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv(windowBudgetEnv, "")
	workDir := newTempDir(t)
	t.Chdir(workDir)
	writeTestFile(t, filepath.Join(workDir, "x", "variant.patch"), "variant\n")
	writeTestFile(t, filepath.Join(home, ".config", "claude-routine", "tokens", abTestToken), "secret\n")
	writeTestFile(t, filepath.Join(home, ".claude", "skills", abTestSkill, skillMdFile), "---\nname: demo\nversion: 1.1.0\n---\n")
	skillsRoot := filepath.Join(newTempDir(t), "skills")
	writeDefinitionFile(t, filepath.Join(skillsRoot, abTestSkill), abDefinition())
	writeTestFile(t, filepath.Join(skillsRoot, abTestSkill, "eval", "overlay.patch"), "definition\n")
	tests := make([]*testCase, 0)

	// model-variant-on-eval
	modelRoot := newTempDir(t)
	modelDir := "ab-" + abTestDate + "-" + descriptorHash("model:sonnet")
	tests = append(tests, &testCase{
		_id:                     "model-variant-on-eval",
		_expectedArms:           []string{"opus-high", "opus-high", "sonnet-high", "sonnet-high"},
		_expectedDir:            filepath.Join(modelRoot, abTestSkill, modelDir),
		_expectedEntries:        []string{modelDir},
		_expectedErr:            nil,
		_expectedKeys:           []string{"opus-high", "sonnet-high"},
		_expectedVariantOverlay: "",

		evalsRoot: modelRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  modelRoot,
			Layer:      LayerEval,
			Replicas:   2,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      abTestToken,
			Variant:    "model:sonnet",
		},
	})

	// version-variant-on-eval
	versionRoot := newTempDir(t)
	versionDir := "ab-" + abTestDate + "-" + descriptorHash("version:1.0.0..1.1.0")
	tests = append(tests, &testCase{
		_id:                     "version-variant-on-eval",
		_expectedArms:           nil,
		_expectedDir:            filepath.Join(versionRoot, abTestSkill, versionDir),
		_expectedEntries:        []string{versionDir},
		_expectedErr:            nil,
		_expectedKeys:           []string{"1.0.0", "1.1.0"},
		_expectedVariantOverlay: "",

		evalsRoot: versionRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  versionRoot,
			Layer:      LayerEval,
			Replicas:   0,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      "",
			Variant:    "version:1.0.0..1.1.0",
		},
	})

	// version-variant-on-smoke
	smokeVersionRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                     "version-variant-on-smoke",
		_expectedArms:           nil,
		_expectedDir:            filepath.Join(smokeVersionRoot, abTestSkill, versionDir),
		_expectedEntries:        []string{versionDir},
		_expectedErr:            nil,
		_expectedKeys:           []string{"1.0.0", "1.1.0"},
		_expectedVariantOverlay: "",

		evalsRoot: smokeVersionRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  smokeVersionRoot,
			Layer:      LayerSmoke,
			Replicas:   0,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      "",
			Variant:    "version:1.0.0..1.1.0",
		},
	})

	// model-variant-on-smoke
	smokeModelRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                     "model-variant-on-smoke",
		_expectedArms:           nil,
		_expectedDir:            "",
		_expectedEntries:        []string{},
		_expectedErr:            ErrNotRunnable,
		_expectedKeys:           nil,
		_expectedVariantOverlay: "",

		evalsRoot: smokeModelRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  smokeModelRoot,
			Layer:      LayerSmoke,
			Replicas:   0,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      abTestToken,
			Variant:    "model:sonnet",
		},
	})

	// bad-descriptor
	badRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                     "bad-descriptor",
		_expectedArms:           nil,
		_expectedDir:            "",
		_expectedEntries:        []string{},
		_expectedErr:            ErrBadVariant,
		_expectedKeys:           nil,
		_expectedVariantOverlay: "",

		evalsRoot: badRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  badRoot,
			Layer:      LayerEval,
			Replicas:   0,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      abTestToken,
			Variant:    "model:sonnet+effort:low",
		},
	})

	// overlay-variant
	overlayRoot := newTempDir(t)
	overlayDir := "ab-" + abTestDate + "-" + descriptorHash("overlay:x/variant.patch")
	tests = append(tests, &testCase{
		_id:                     "overlay-variant",
		_expectedArms:           []string{"opus-high", "opus-high-overlay " + overlayVariantFile},
		_expectedDir:            filepath.Join(overlayRoot, abTestSkill, overlayDir),
		_expectedEntries:        []string{overlayDir},
		_expectedErr:            nil,
		_expectedKeys:           []string{"opus-high", "opus-high-overlay"},
		_expectedVariantOverlay: "definition\nvariant\n",

		evalsRoot: overlayRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  overlayRoot,
			Layer:      LayerEval,
			Replicas:   1,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      abTestToken,
			Variant:    "overlay:x/variant.patch",
		},
	})

	// overlay-file-missing
	missingRoot := newTempDir(t)
	missingDir := "ab-" + abTestDate + "-" + descriptorHash("overlay:x/missing.patch")
	tests = append(tests, &testCase{
		_id:                     "overlay-file-missing",
		_expectedArms:           nil,
		_expectedDir:            "",
		_expectedEntries:        []string{missingDir},
		_expectedErr:            fs.ErrNotExist,
		_expectedKeys:           nil,
		_expectedVariantOverlay: "",

		evalsRoot: missingRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  missingRoot,
			Layer:      LayerEval,
			Replicas:   1,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      abTestToken,
			Variant:    "overlay:x/missing.patch",
		},
	})

	// window-counts-both-sides
	windowRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                     "window-counts-both-sides",
		_expectedArms:           nil,
		_expectedDir:            "",
		_expectedEntries:        []string{},
		_expectedErr:            ErrOverWindow,
		_expectedKeys:           nil,
		_expectedVariantOverlay: "",

		evalsRoot: windowRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  windowRoot,
			Layer:      LayerEval,
			Replicas:   8,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      abTestToken,
			Variant:    "model:sonnet",
		},
	})

	// unknown-token
	tokenRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                     "unknown-token",
		_expectedArms:           nil,
		_expectedDir:            "",
		_expectedEntries:        []string{},
		_expectedErr:            ErrUnknownToken,
		_expectedKeys:           nil,
		_expectedVariantOverlay: "",

		evalsRoot: tokenRoot,
		request: &AbInitRequest{
			Date:       abTestDate,
			EvalsRoot:  tokenRoot,
			Layer:      LayerEval,
			Replicas:   1,
			Skill:      abTestSkill,
			SkillsRoot: skillsRoot,
			Token:      "absent",
			Variant:    "model:sonnet",
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			abDir, err := InitAb(test.request)
			require.ErrorIs(t, err, test._expectedErr)
			assert.Equal(t, test._expectedDir, abDir)
			assert.Equal(t, test._expectedEntries, dirNames(t, filepath.Join(test.evalsRoot, abTestSkill)))
			assert.Equal(t, test._expectedKeys, abSideKeys(t, abDir))
			assert.Equal(t, test._expectedArms, armSummaries(t, abDir))
			assert.Equal(t, test._expectedVariantOverlay, fileText(filepath.Join(abDir, overlayVariantFile)))
		})
	}

	assert.Regexp(t, `^[0-9a-f]{6}$`, descriptorHash("model:sonnet"))
}

func TestAbCollect(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedBaseline []evals.AbRecord
		_expectedErrPart  string
		_expectedNotes    []string
		_expectedVariant  []evals.AbRecord
		_shouldPass       bool

		abDir string
	}

	home := newTempDir(t)
	t.Setenv("HOME", home)
	smokeRoot := filepath.Join(home, smokeRootDir)
	tests := make([]*testCase, 0)

	// run-backed-split-by-key
	runBackedDir := filepath.Join(newTempDir(t), abTestSkill, "ab-2026-10-02-aaaaaa")
	writeAbJson(t, runBackedDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	runBackedRecords := []recordFixture{
		{costUsd: 5.3, key: "sonnet-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: abTestVersion},
		{costUsd: 11.4, key: "opus-high", label: "B", layer: LayerEval, skill: abTestSkill, skillVersion: abTestVersion},
	}
	writeRecordRunDir(t, true, runBackedRecords, runBackedDir)
	tests = append(tests, &testCase{
		_id:               "run-backed-split-by-key",
		_expectedBaseline: []evals.AbRecord{abRecord(11.4, "B", runBackedDir, abTestVersion)},
		_expectedErrPart:  "",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{abRecord(5.3, "A", runBackedDir, abTestVersion)},
		_shouldPass:       true,

		abDir: runBackedDir,
	})

	// run-backed-key-on-neither-side
	neitherDir := filepath.Join(newTempDir(t), abTestSkill, "ab-2026-10-02-bbbbbb")
	writeAbJson(t, neitherDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	neitherRecords := []recordFixture{
		{costUsd: 5.3, key: "haiku-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: abTestVersion},
	}
	writeRecordRunDir(t, true, neitherRecords, neitherDir)
	tests = append(tests, &testCase{
		_id:               "run-backed-key-on-neither-side",
		_expectedBaseline: []evals.AbRecord{},
		_expectedErrPart:  "key haiku-high is neither side",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{},
		_shouldPass:       false,

		abDir: neitherDir,
	})

	// version-eval-across-run-dirs
	acrossRoot := filepath.Join(newTempDir(t), abTestSkill)
	acrossDir := filepath.Join(acrossRoot, "ab-2026-10-02-cccccc")
	writeAbJson(t, acrossDir, "1.0.0", "version:1.0.0..1.1.0", LayerEval, abTestSkill, "1.1.0")
	acrossFirst := filepath.Join(acrossRoot, "2026-10-01-abc123")
	acrossFirstRecords := []recordFixture{
		{costUsd: 1.0, key: "opus-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: "1.0.0"},
		{costUsd: 2.0, key: "opus-high", label: "B", layer: LayerEval, skill: abTestSkill, skillVersion: "1.1.0"},
	}
	writeRecordRunDir(t, true, acrossFirstRecords, acrossFirst)
	acrossSecond := filepath.Join(acrossRoot, "2026-10-02-abc123")
	acrossSecondRecords := []recordFixture{
		{costUsd: 3.0, key: "opus-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: "1.1.0"},
		{costUsd: 4.0, key: "opus-high", label: "B", layer: LayerEval, skill: abTestSkill, skillVersion: "1.2.0"},
	}
	writeRecordRunDir(t, true, acrossSecondRecords, acrossSecond)
	tests = append(tests, &testCase{
		_id:               "version-eval-across-run-dirs",
		_expectedBaseline: []evals.AbRecord{abRecord(1.0, "A", acrossFirst, "1.0.0")},
		_expectedErrPart:  "",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{abRecord(2.0, "B", acrossFirst, "1.1.0"), abRecord(3.0, "A", acrossSecond, "1.1.0")},
		_shouldPass:       true,

		abDir: acrossDir,
	})

	// version-eval-skips-foreign-records
	skipsRoot := filepath.Join(newTempDir(t), abTestSkill)
	skipsDir := filepath.Join(skipsRoot, "ab-2026-10-02-dddddd")
	writeAbJson(t, skipsDir, "1.0.0", "version:1.0.0..1.1.0", LayerEval, abTestSkill, "1.1.0")
	otherAbRecords := []recordFixture{
		{costUsd: 9.0, key: "opus-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: "1.0.0"},
	}
	writeRecordRunDir(t, true, otherAbRecords, filepath.Join(skipsRoot, "ab-2026-10-01-eeeeee"))
	skipsRunDir := filepath.Join(skipsRoot, "2026-10-01-abc123")
	skipsRecords := []recordFixture{
		{costUsd: 7.0, key: "opus-high", label: "A", layer: "", skill: abTestSkill, skillVersion: "1.0.0"},
		{costUsd: 8.0, key: "opus-high", label: "B", layer: LayerEval, skill: "other", skillVersion: "1.1.0"},
		{costUsd: 1.0, key: "opus-high", label: "C", layer: LayerEval, skill: abTestSkill, skillVersion: "1.0.0"},
		{costUsd: 2.0, key: "opus-high", label: "D", layer: LayerEval, skill: abTestSkill, skillVersion: "1.1.0"},
	}
	writeRecordRunDir(t, true, skipsRecords, skipsRunDir)
	tests = append(tests, &testCase{
		_id:               "version-eval-skips-foreign-records",
		_expectedBaseline: []evals.AbRecord{abRecord(1.0, "C", skipsRunDir, "1.0.0")},
		_expectedErrPart:  "",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{abRecord(2.0, "D", skipsRunDir, "1.1.0")},
		_shouldPass:       true,

		abDir: skipsDir,
	})

	// version-eval-unfolded-noted
	unfoldedRoot := filepath.Join(newTempDir(t), abTestSkill)
	unfoldedDir := filepath.Join(unfoldedRoot, "ab-2026-10-02-ffffff")
	writeAbJson(t, unfoldedDir, "1.0.0", "version:1.0.0..1.1.0", LayerEval, abTestSkill, "1.1.0")
	foldedRunDir := filepath.Join(unfoldedRoot, "2026-10-01-abc123")
	foldedRecords := []recordFixture{
		{costUsd: 1.0, key: "opus-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: "1.0.0"},
		{costUsd: 2.0, key: "opus-high", label: "B", layer: LayerEval, skill: abTestSkill, skillVersion: "1.1.0"},
	}
	writeRecordRunDir(t, true, foldedRecords, foldedRunDir)
	unfoldedRunDir := filepath.Join(unfoldedRoot, "2026-10-02-abc123")
	unfoldedRecords := []recordFixture{
		{costUsd: 3.0, key: "opus-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: "1.1.0"},
	}
	writeRecordRunDir(t, false, unfoldedRecords, unfoldedRunDir)
	tests = append(tests, &testCase{
		_id:               "version-eval-unfolded-noted",
		_expectedBaseline: []evals.AbRecord{abRecord(1.0, "A", foldedRunDir, "1.0.0")},
		_expectedErrPart:  "",
		_expectedNotes:    []string{"skipped " + unfoldedRunDir + ": no eval.json, not folded"},
		_expectedVariant:  []evals.AbRecord{abRecord(2.0, "B", foldedRunDir, "1.1.0")},
		_shouldPass:       true,

		abDir: unfoldedDir,
	})

	// version-smoke-one-record-per-version
	smokeDir := filepath.Join(newTempDir(t), "smoke-one", "ab-2026-10-02-111111")
	writeAbJson(t, smokeDir, "1.0.0", "version:1.0.0..1.1.0", LayerSmoke, "smoke-one", "1.1.0")
	smokeBaselinePath := filepath.Join(smokeRoot, "smoke-one", "1.0.0", runsDir, smokeArmLabel, resultFile)
	smokeVariantPath := filepath.Join(smokeRoot, "smoke-one", "1.1.0", runsDir, smokeArmLabel, resultFile)
	baselineCost := 0.5
	variantCost := 0.7
	smokePass := &SmokeResult{Verdict: statusPass}
	smokeFail := &SmokeResult{Cause: "anchor missed", Verdict: statusFail}
	writeSmokeRecord(t, smokeBaselinePath, &ArmResult{CostUsd: &baselineCost, Layer: LayerSmoke, Skill: "smoke-one", SkillVersion: "1.0.0", Smoke: smokePass})
	writeSmokeRecord(t, smokeVariantPath, &ArmResult{CostUsd: &variantCost, Layer: LayerSmoke, Skill: "smoke-one", SkillVersion: "1.1.0", Smoke: smokeFail})
	smokeBaselineRecord := evals.AbRecord{
		Measures:     map[string]float64{metricCostUsd: 0.5, metricSmokePass: 1},
		Path:         smokeBaselinePath,
		SkillVersion: "1.0.0",
	}
	smokeVariantRecord := evals.AbRecord{
		Measures:     map[string]float64{metricCostUsd: 0.7, metricSmokePass: 0},
		Path:         smokeVariantPath,
		SkillVersion: "1.1.0",
	}
	tests = append(tests, &testCase{
		_id:               "version-smoke-one-record-per-version",
		_expectedBaseline: []evals.AbRecord{smokeBaselineRecord},
		_expectedErrPart:  "",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{smokeVariantRecord},
		_shouldPass:       true,

		abDir: smokeDir,
	})

	// version-smoke-staging-not-read
	stagedDir := filepath.Join(newTempDir(t), "smoke-staged", "ab-2026-10-02-222222")
	writeAbJson(t, stagedDir, "1.0.0", "version:1.0.0..1.1.0", LayerSmoke, "smoke-staged", "1.1.0")
	stagedBaselinePath := filepath.Join(smokeRoot, "smoke-staged", "1.0.0", runsDir, smokeArmLabel, resultFile)
	writeSmokeRecord(t, stagedBaselinePath, &ArmResult{Layer: LayerSmoke, Skill: "smoke-staged", SkillVersion: "1.0.0", Smoke: smokePass})
	stagedOnlyPath := filepath.Join(smokeRoot, smokeStagingDir, "2026-10-02-120000-smoke-staged", runsDir, smokeArmLabel, resultFile)
	writeSmokeRecord(t, stagedOnlyPath, &ArmResult{Layer: LayerSmoke, Skill: "smoke-staged", SkillVersion: "1.1.0", Smoke: smokePass})
	tests = append(tests, &testCase{
		_id:               "version-smoke-staging-not-read",
		_expectedBaseline: []evals.AbRecord{},
		_expectedErrPart:  "no smoke record of version 1.1.0 at " + filepath.Join(smokeRoot, "smoke-staged", "1.1.0"),
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{},
		_shouldPass:       false,

		abDir: stagedDir,
	})

	// missing-smoke-version
	missingDir := filepath.Join(newTempDir(t), "smoke-missing", "ab-2026-10-02-333333")
	writeAbJson(t, missingDir, "1.0.0", "version:1.0.0..1.1.0", LayerSmoke, "smoke-missing", "1.1.0")
	missingVariantPath := filepath.Join(smokeRoot, "smoke-missing", "1.1.0", runsDir, smokeArmLabel, resultFile)
	writeSmokeRecord(t, missingVariantPath, &ArmResult{Layer: LayerSmoke, Skill: "smoke-missing", SkillVersion: "1.1.0", Smoke: smokePass})
	tests = append(tests, &testCase{
		_id:               "missing-smoke-version",
		_expectedBaseline: []evals.AbRecord{},
		_expectedErrPart:  "no smoke record of version 1.0.0",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{},
		_shouldPass:       false,

		abDir: missingDir,
	})

	// empty-side
	emptyRoot := filepath.Join(newTempDir(t), abTestSkill)
	emptyDir := filepath.Join(emptyRoot, "ab-2026-10-02-444444")
	writeAbJson(t, emptyDir, "1.0.0", "version:1.0.0..1.1.0", LayerEval, abTestSkill, "1.1.0")
	emptyRecords := []recordFixture{
		{costUsd: 1.0, key: "opus-high", label: "A", layer: LayerEval, skill: abTestSkill, skillVersion: "1.0.0"},
	}
	writeRecordRunDir(t, true, emptyRecords, filepath.Join(emptyRoot, "2026-10-01-abc123"))
	tests = append(tests, &testCase{
		_id:               "empty-side",
		_expectedBaseline: []evals.AbRecord{},
		_expectedErrPart:  "baseline 1 records, variant 0 — nothing to compare",
		_expectedNotes:    []string{},
		_expectedVariant:  []evals.AbRecord{},
		_shouldPass:       false,

		abDir: emptyDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := AbCollect(test.abDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._expectedErrPart)
			file := readAbJson(t, test.abDir)
			assert.Equal(t, test._expectedBaseline, file.Baseline.Records)
			assert.Equal(t, test._expectedVariant, file.Variant.Records)
			assert.Equal(t, test._expectedNotes, file.Notes)
		})
	}
}

func TestSmokeMeasures(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]float64

		result *ArmResult
	}

	cost := 1.25
	tokens := 4000
	denials := 2
	startedAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(90 * time.Second)
	tests := make([]*testCase, 0)

	// pass
	tests = append(tests, &testCase{
		_id: "pass",
		_expected: map[string]float64{
			metricCostUsd:           1.25,
			metricOutputTokens:      4000,
			metricPermissionDenials: 2,
			metricSmokePass:         1,
			metricWallSeconds:       90,
		},

		result: &ArmResult{
			CostUsd:           &cost,
			EndedAt:           endedAt,
			OutputTokens:      &tokens,
			PermissionDenials: &denials,
			Smoke:             &SmokeResult{Verdict: statusPass},
			StartedAt:         startedAt,
		},
	})

	// fail
	tests = append(tests, &testCase{
		_id:       "fail",
		_expected: map[string]float64{metricSmokePass: 0},

		result: &ArmResult{Smoke: &SmokeResult{Cause: "anchor missed", Verdict: statusFail}},
	})

	// no-smoke
	tests = append(tests, &testCase{
		_id:       "no-smoke",
		_expected: map[string]float64{metricCostUsd: 1.25},

		result: &ArmResult{CostUsd: &cost},
	})

	// nil-telemetry-skipped
	tests = append(tests, &testCase{
		_id:       "nil-telemetry-skipped",
		_expected: map[string]float64{},

		result: &ArmResult{},
	})

	// zero-started-at
	tests = append(tests, &testCase{
		_id:       "zero-started-at",
		_expected: map[string]float64{},

		result: &ArmResult{EndedAt: endedAt},
	})

	// ratios-when-declared
	tests = append(tests, &testCase{
		_id: "ratios-when-declared",
		_expected: map[string]float64{
			measureAnchorsRatio: 0.75,
			measureChecksRatio:  0.5,
			metricSmokePass:     1,
		},

		result: &ArmResult{
			Smoke: &SmokeResult{
				Coverage: SmokeCoverage{AnchorsDeclared: 4, AnchorsHit: 3, ChecksDeclared: 4, ChecksPassed: 2},
				Verdict:  statusPass,
			},
		},
	})

	// no-ratio-when-undeclared
	tests = append(tests, &testCase{
		_id:       "no-ratio-when-undeclared",
		_expected: map[string]float64{measureChecksRatio: 1, metricSmokePass: 1},

		result: &ArmResult{
			Smoke: &SmokeResult{
				Coverage: SmokeCoverage{AnchorsDeclared: 0, AnchorsHit: 0, ChecksDeclared: 2, ChecksPassed: 2},
				Verdict:  statusPass,
			},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, smokeMeasures(test.result))
		})
	}
}

func TestAbReport(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedDifferences []evals.AbDifference
		_expectedReport      string
		_shouldPass          bool

		abDir string
	}

	tests := make([]*testCase, 0)
	baselineGroup := evals.StatsGroup{Arm: "opus-high", Mean: 11.4, MetricId: metricCostUsd, N: 1, StdDev: 0}
	variantGroup := evals.StatsGroup{Arm: "sonnet-high", Mean: 5.3, MetricId: metricCostUsd, N: 1, StdDev: 0}
	costComparison := evals.StatsComparison{
		Arm:       "sonnet-high",
		DeltaMean: -6.1,
		MetricId:  metricCostUsd,
		Note:      "insufficient replicas for significance (need >=2 per arm)",
	}
	pairedStats := &evals.Stats{
		Baseline:    "opus-high",
		Comparisons: []evals.StatsComparison{costComparison},
		Groups:      []evals.StatsGroup{baselineGroup, variantGroup},
	}
	costDifference := evals.AbDifference{
		BaselineMean:   11.4,
		BaselineN:      1,
		BaselineStdDev: 0,
		DeltaMean:      -6.1,
		Df:             0,
		Measure:        metricCostUsd,
		Note:           "insufficient replicas for significance (need >=2 per arm)",
		Significant:    false,
		VariantMean:    5.3,
		VariantN:       1,
		VariantStdDev:  0,
		WelchT:         0,
	}
	reportHeader := "# demo — A/B 2026-10-02\n\n" +
		"- layer `eval`, variant `model:sonnet`\n" +
		"- baseline `opus-high`: 0 records; variant `sonnet-high`: 0 records\n\n" +
		"| Measure | Baseline | Variant | Δ mean | Welch t | df | Significant | Note |\n" +
		"|---|---|---|---|---|---|---|---|\n"
	costRow := "| cost_usd | $11.40 ± $0.00 (1) | $5.30 ± $0.00 (1) | -6.10 | 0.00 | 0.0 | no | insufficient replicas for significance (need >=2 per arm) |\n"

	// paired-without-compare
	pairedDir := newTempDir(t)
	writeAbJson(t, pairedDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	require.NoError(t, writeJsonFile(filepath.Join(pairedDir, statsFile), pairedStats))
	tests = append(tests, &testCase{
		_id:                  "paired-without-compare",
		_expectedDifferences: []evals.AbDifference{costDifference},
		_expectedReport:      reportHeader + costRow,
		_shouldPass:          true,

		abDir: pairedDir,
	})

	// variant-only-measure
	variantOnlyDir := newTempDir(t)
	writeAbJson(t, variantOnlyDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	stepGroup := evals.StatsGroup{Arm: "sonnet-high", Mean: 12.5, MetricId: metricStepMissPct, N: 1}
	variantOnlyStats := &evals.Stats{Baseline: "opus-high", Comparisons: []evals.StatsComparison{}, Groups: []evals.StatsGroup{stepGroup}}
	require.NoError(t, writeJsonFile(filepath.Join(variantOnlyDir, statsFile), variantOnlyStats))
	tests = append(tests, &testCase{
		_id:                  "variant-only-measure",
		_expectedDifferences: []evals.AbDifference{},
		_expectedReport:      reportHeader,
		_shouldPass:          true,

		abDir: variantOnlyDir,
	})

	// compare-present
	compareDir := newTempDir(t)
	writeAbJson(t, compareDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	require.NoError(t, writeJsonFile(filepath.Join(compareDir, statsFile), pairedStats))
	compareArms := []ManifestArm{{Id: "sonnet-high-r1", Key: "sonnet-high", Label: "A"}, {Id: "opus-high-r1", Key: "opus-high", Label: "B"}}
	writeRunDir(t, compareDir, &RunManifest{Arms: compareArms})
	compareDefects := []CompareDefect{{Claim: "nil map write", File: "x.go", Severity: severityMajor}}
	compare := &CompareReport{
		Arms:    []CompareArm{{Defects: compareDefects, Label: "A", Summary: "tidy"}, {Defects: []CompareDefect{}, Label: "B", Summary: "fine"}},
		Ranking: []string{"B", "A"},
	}
	require.NoError(t, writeJsonFile(filepath.Join(compareDir, compareFile), compare))
	compareSection := "\n## Compare judge\n\n" +
		"Ranking, best first: B, A\n\n" +
		"- `A` (sonnet-high): tidy\n" +
		"  - MAJOR `x.go`: nil map write\n" +
		"- `B` (opus-high): fine\n"
	tests = append(tests, &testCase{
		_id:                  "compare-present",
		_expectedDifferences: []evals.AbDifference{costDifference},
		_expectedReport:      reportHeader + costRow + compareSection,
		_shouldPass:          true,

		abDir: compareDir,
	})

	// missing-stats
	missingStatsDir := newTempDir(t)
	writeAbJson(t, missingStatsDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	tests = append(tests, &testCase{
		_id:                  "missing-stats",
		_expectedDifferences: []evals.AbDifference{},
		_expectedReport:      "",
		_shouldPass:          false,

		abDir: missingStatsDir,
	})

	// notes-rendered
	notesDir := newTempDir(t)
	writeAbJson(t, notesDir, "opus-high", "model:sonnet", LayerEval, abTestSkill, "sonnet-high")
	notesFile := readAbJson(t, notesDir)
	notesFile.Notes = []string{"skipped /evals/demo/2026-10-01-abc123: no eval.json, not folded"}
	require.NoError(t, writeAbFile(notesDir, notesFile))
	require.NoError(t, writeJsonFile(filepath.Join(notesDir, statsFile), pairedStats))
	notesSection := "\n## Notes\n\n- skipped /evals/demo/2026-10-01-abc123: no eval.json, not folded\n"
	tests = append(tests, &testCase{
		_id:                  "notes-rendered",
		_expectedDifferences: []evals.AbDifference{costDifference},
		_expectedReport:      reportHeader + costRow + notesSection,
		_shouldPass:          true,

		abDir: notesDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := AbReport(test.abDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDifferences, readAbJson(t, test.abDir).Differences)
			assert.Equal(t, test._expectedReport, fileText(filepath.Join(test.abDir, reportFile)))
		})
	}
}
