package skilleval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/peek"
)

const (
	foldSkill      = "railroad-review"
	realSchemaPath = "../../skills/skillroutine/skillroutine-eval/reference/schema.json"
	sessionlessArm = `{"exitStatus":0,"sessionId":"","subtype":"success","terminalReason":"completed"}`
)

// definedMetricIds are the candidate metric ids eval.json defines.
func definedMetricIds(metrics []evals.Metric, candidateIds []string) []string {
	defined := []string{}
	for _, metric := range metrics {
		if slices.Contains(candidateIds, metric.Id) {
			defined = append(defined, metric.Id)
		}
	}
	return defined
}

func foldRanks(folds []*armFold) []int {
	ranks := make([]int, 0, len(folds))
	for _, fold := range folds {
		ranks = append(ranks, fold.rank)
	}
	return ranks
}

func foldSummaries(folds []*armFold) []string {
	summaries := make([]string, 0, len(folds))
	for _, fold := range folds {
		summary := ""
		if fold.compare != nil {
			summary = fold.compare.Summary
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func metricLabel(metrics []evals.Metric, metricId string) string {
	for _, metric := range metrics {
		if metric.Id == metricId {
			return metric.Label
		}
	}
	return ""
}

func metricValueOf(values []evals.MetricValue, metricId, runId string) evals.MetricValue {
	for _, value := range values {
		if value.MetricId == metricId && value.RunId == runId {
			return value
		}
	}
	return evals.MetricValue{}
}

func metricOf(metrics []evals.Metric, metricId string) evals.Metric {
	for _, metric := range metrics {
		if metric.Id == metricId {
			return metric
		}
	}
	return evals.Metric{}
}

// oneKeyManifest is run.json of one configuration in two replicas.
func oneKeyManifest(scorer []string) *RunManifest {
	manifest := &RunManifest{
		Arms: []ManifestArm{
			{Id: "opus-high-r1", DisabledServers: []string{}, Effort: "high", Key: "opus-high", Label: "A", Model: "opus", Replica: 1, Worktree: "/wt/skilleval-run-A"},
			{Id: "opus-high-r2", DisabledServers: []string{}, Effort: "high", Key: "opus-high", Label: "B", Model: "opus", Replica: 2, Worktree: "/wt/skilleval-run-B"},
		},
		At:             "a36dd5b46f542542cb58401ef7cf2d7452eb28f9",
		BudgetUsd:      30,
		Check:          []string{},
		CheckTimeoutS:  0,
		Date:           "2026-09-26",
		Judge:          "opus",
		PresetSkillDir: "",
		Prompt:         "/railroad-review",
		Reference:      "",
		Repo:           "/repo",
		Scorer:         scorer,
		Skill:          foldSkill,
		SkillMd:        "evals/railroad-review/2026-09-26-a36dd5/SKILL.md",
		TimeoutS:       7200,
	}
	return manifest
}

// probeResults are the results of every probe of one name on one run.
func probeResults(probes []evals.Probe, name, runId string) []string {
	results := []string{}
	for _, probe := range probes {
		isMatch := probe.Name == name && probe.RunId == runId
		if isMatch {
			results = append(results, probe.Result)
		}
	}
	return results
}

func provideArmResult() *ArmResult {
	return &ArmResult{
		ExitStatus:     0,
		SessionId:      "session-1",
		Subtype:        subtypeSuccess,
		TerminalReason: terminalCompleted,
	}
}

func provideFolds() []*armFold {
	return []*armFold{
		{arm: &ManifestArm{Id: "opus-high-r1", Label: "A"}},
		{arm: &ManifestArm{Id: "sonnet-high-r1", Label: "B"}},
	}
}

func readEvalFile(t *testing.T, runDir string) *evals.EvalFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, evalFileName))
	require.NoError(t, err)
	var file evals.EvalFile
	require.NoError(t, json.Unmarshal(data, &file))
	return &file
}

// twoArmManifest is run.json of two configurations, one replica each.
func twoArmManifest() *RunManifest {
	manifest := &RunManifest{
		Arms: []ManifestArm{
			{Id: "opus-high-r1", DisabledServers: []string{}, Effort: "high", Key: "opus-high", Label: "A", Model: "opus", Replica: 1, Worktree: "/wt/skilleval-run-A"},
			{Id: "sonnet-high-r1", DisabledServers: []string{}, Effort: "high", Key: "sonnet-high", Label: "B", Model: "sonnet", Replica: 1, Worktree: "/wt/skilleval-run-B"},
		},
		At:             "a36dd5b46f542542cb58401ef7cf2d7452eb28f9",
		BudgetUsd:      30,
		Check:          []string{},
		CheckTimeoutS:  0,
		Date:           "2026-09-26",
		Judge:          "opus",
		PresetSkillDir: "",
		Prompt:         "/railroad-review",
		Reference:      "",
		Repo:           "/repo",
		Skill:          foldSkill,
		SkillMd:        "evals/railroad-review/2026-09-26-a36dd5/SKILL.md",
		TimeoutS:       7200,
	}
	return manifest
}

// valuedMetricIds are the candidate metric ids eval.json gives a value for
// one run.
func valuedMetricIds(values []evals.MetricValue, candidateIds []string, runId string) []string {
	valued := []string{}
	for _, value := range values {
		isMatch := value.RunId == runId && slices.Contains(candidateIds, value.MetricId)
		if isMatch {
			valued = append(valued, value.MetricId)
		}
	}
	return valued
}

// writeArmRunDir writes a run dir of run.json and the A and B arm results.
func writeArmRunDir(t *testing.T, manifest *RunManifest, resultA, resultB string) string {
	t.Helper()
	runDir := t.TempDir()
	writeRunDir(t, runDir, manifest)
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", resultFile), resultA)
	writeTestFile(t, filepath.Join(runDir, runsDir, "B", resultFile), resultB)
	return runDir
}

func TestAdherenceScore(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedApplicable bool
		_expectedScore      int
		_shouldPass         bool

		verdict string
	}

	tests := make([]*testCase, 0)

	// followed
	tests = append(tests, &testCase{
		_id:                 "followed",
		_expectedApplicable: true,
		_expectedScore:      1,
		_shouldPass:         true,

		verdict: verdictFollowed,
	})

	// partial
	tests = append(tests, &testCase{
		_id:                 "partial",
		_expectedApplicable: true,
		_expectedScore:      0,
		_shouldPass:         true,

		verdict: verdictPartial,
	})

	// skipped
	tests = append(tests, &testCase{
		_id:                 "skipped",
		_expectedApplicable: true,
		_expectedScore:      -1,
		_shouldPass:         true,

		verdict: verdictSkipped,
	})

	// violated
	tests = append(tests, &testCase{
		_id:                 "violated",
		_expectedApplicable: true,
		_expectedScore:      -1,
		_shouldPass:         true,

		verdict: verdictViolated,
	})

	// not-applicable
	tests = append(tests, &testCase{
		_id:                 "not-applicable",
		_expectedApplicable: false,
		_expectedScore:      0,
		_shouldPass:         true,

		verdict: verdictNotApplicable,
	})

	// not-observable
	tests = append(tests, &testCase{
		_id:                 "not-observable",
		_expectedApplicable: false,
		_expectedScore:      0,
		_shouldPass:         true,

		verdict: verdictNotObservable,
	})

	// unknown
	tests = append(tests, &testCase{
		_id:                 "unknown",
		_expectedApplicable: false,
		_expectedScore:      0,
		_shouldPass:         false,

		verdict: "mostly-followed",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			score, isApplicable, err := adherenceScore(test.verdict)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedScore, score)
			assert.Equal(t, test._expectedApplicable, isApplicable)
		})
	}
}

func TestApplyCompare(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedRanks     []int
		_expectedSummaries []string
		_shouldPass        bool

		folds  []*armFold
		runDir string
	}

	tests := make([]*testCase, 0)

	// labels-mapped-and-ranked
	rankedDir := t.TempDir()
	writeTestFile(t, filepath.Join(rankedDir, compareFile), `{
		"arms": [
			{"label": "B", "summary": "tight diff", "defects": []},
			{"label": "A", "summary": "missed a stop", "defects": [{"severity": "MAJOR", "file": "a.go", "claim": "ignored the gate"}]}
		],
		"ranking": ["B", "A"]
	}`)
	tests = append(tests, &testCase{
		_id:                "labels-mapped-and-ranked",
		_expectedRanks:     []int{2, 1},
		_expectedSummaries: []string{"missed a stop", "tight diff"},
		_shouldPass:        true,

		folds:  provideFolds(),
		runDir: rankedDir,
	})

	// unknown-label-errors
	unknownDir := t.TempDir()
	writeTestFile(t, filepath.Join(unknownDir, compareFile), `{"arms": [{"label": "C", "summary": "ghost"}], "ranking": ["C"]}`)
	tests = append(tests, &testCase{
		_id:                "unknown-label-errors",
		_expectedRanks:     []int{0, 0},
		_expectedSummaries: []string{"", ""},
		_shouldPass:        false,

		folds:  provideFolds(),
		runDir: unknownDir,
	})

	// missing-file-no-op
	tests = append(tests, &testCase{
		_id:                "missing-file-no-op",
		_expectedRanks:     []int{0, 0},
		_expectedSummaries: []string{"", ""},
		_shouldPass:        true,

		folds:  provideFolds(),
		runDir: t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := applyCompare(test.folds, test.runDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedRanks, foldRanks(test.folds))
			assert.Equal(t, test._expectedSummaries, foldSummaries(test.folds))
		})
	}
}

func TestArmFold_failedMetrics(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		goldenValues []evals.MetricValue
	}

	thresholds := map[string]float64{metricReviewScore: 0.5, metricFalsePositiveMinutes: 60}
	tests := make([]*testCase, 0)

	// null-score-skipped-false-positive-bound-fails
	tests = append(tests, &testCase{
		_id:       "null-score-skipped-false-positive-bound-fails",
		_expected: []string{metricFalsePositiveMinutes},

		goldenValues: []evals.MetricValue{
			nullValue(metricReviewScore, "run-1", noteZeroDenominator),
			numberValue(metricFalsePositiveMinutes, "run-1", 270),
		},
	})

	// both-bounds-pass
	tests = append(tests, &testCase{
		_id:       "both-bounds-pass",
		_expected: []string{},

		goldenValues: []evals.MetricValue{
			numberValue(metricReviewScore, "run-1", 0.9),
			numberValue(metricFalsePositiveMinutes, "run-1", 20),
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, (&armFold{goldenValues: test.goldenValues, result: provideArmResult()}).failedMetrics(thresholds))
		})
	}
}

func TestArmFold_envelopeValues(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []evals.MetricValue

		fold *armFold
	}

	arm := &ManifestArm{Id: "opus-high-r1", Label: "A"}
	tests := make([]*testCase, 0)

	// envelope-metrics-from-result
	envelopeResult := provideArmResult()
	envelopeCost := 12.5
	envelopeDenials := 3
	envelopeTokens := 480_000
	envelopeResult.CostUsd = &envelopeCost
	envelopeResult.OutputTokens = &envelopeTokens
	envelopeResult.PermissionDenials = &envelopeDenials
	tests = append(tests, &testCase{
		_id: "envelope-metrics-from-result",
		_expected: []evals.MetricValue{
			numberValue(metricOutputTokens, "opus-high-r1", 480_000),
			numberValue(metricCostUsd, "opus-high-r1", 12.5),
			numberValue(metricPermissionDenials, "opus-high-r1", 3),
		},

		fold: &armFold{arm: arm, result: envelopeResult, telemetry: &peek.SessionEvents{CostUsd: 99, HasTelemetry: true}},
	})

	// no-envelope-null-notes
	tests = append(tests, &testCase{
		_id: "no-envelope-null-notes",
		_expected: []evals.MetricValue{
			nullValue(metricOutputTokens, "opus-high-r1", noEnvelopeNote),
			nullValue(metricCostUsd, "opus-high-r1", noEnvelopeNote),
			nullValue(metricPermissionDenials, "opus-high-r1", noEnvelopeNote),
		},

		fold: &armFold{arm: arm, result: provideArmResult(), telemetry: nil},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.fold.envelopeValues())
		})
	}
}

func TestSmokeFailures(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		result    *ArmResult
		telemetry *peek.SessionEvents
	}

	invokedTelemetry := &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: foldSkill}}}
	tests := make([]*testCase, 0)

	// clean-pass
	tests = append(tests, &testCase{
		_id:       "clean-pass",
		_expected: nil,

		result:    provideArmResult(),
		telemetry: invokedTelemetry,
	})

	// exit-nonzero
	exitResult := provideArmResult()
	exitResult.ExitStatus = 1
	tests = append(tests, &testCase{
		_id:       "exit-nonzero",
		_expected: []string{"exit status 1"},

		result:    exitResult,
		telemetry: invokedTelemetry,
	})

	// terminal-not-completed
	terminalResult := provideArmResult()
	terminalResult.TerminalReason = "api_error"
	tests = append(tests, &testCase{
		_id:       "terminal-not-completed",
		_expected: []string{`terminal reason "api_error"`},

		result:    terminalResult,
		telemetry: invokedTelemetry,
	})

	// check-failed
	checkResult := provideArmResult()
	checkExit := 2
	checkResult.CheckExit = &checkExit
	tests = append(tests, &testCase{
		_id:       "check-failed",
		_expected: []string{"check exited 2"},

		result:    checkResult,
		telemetry: invokedTelemetry,
	})

	// skill-not-invoked
	tests = append(tests, &testCase{
		_id:       "skill-not-invoked",
		_expected: []string{"skill railroad-review never invoked"},

		result:    provideArmResult(),
		telemetry: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "fimplement"}}},
	})

	// peek-unavailable-no-failure
	tests = append(tests, &testCase{
		_id:       "peek-unavailable-no-failure",
		_expected: nil,

		result:    provideArmResult(),
		telemetry: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, smokeFailures(test.result, foldSkill, test.telemetry))
		})
	}
}

func TestConfigNote(t *testing.T) {
	skillDir := t.TempDir()
	definition := &evals.Definition{
		Invocation: evals.DefinitionInvocation{Open: []string{"lanes", "station", "rounds"}},
		Skill:      foldSkill,
	}
	writeDefinitionFile(t, skillDir, definition)

	manifest := oneKeyManifest(nil)
	manifest.PresetSkillDir = skillDir
	manifest.Prompt = "/railroad-review directions: code-style lanes: 1[sonnet:medium] station: opus:medium nightly"
	assert.Equal(t, "lanes 1[sonnet:medium] · station opus:medium", configNote(manifest), "open args the prompt sets, in definition order; rounds absent")

	manifest.PresetSkillDir = ""
	assert.Equal(t, "arms: opus-high", configNote(manifest), "no definition: the arms")

	manifest.PresetSkillDir = filepath.Join(skillDir, "missing")
	assert.Equal(t, "arms: opus-high", configNote(manifest), "unreadable definition: the arms")

	manifest.PresetSkillDir = skillDir
	manifest.Prompt = "/railroad-review lanes:"
	assert.Equal(t, "arms: opus-high", configNote(manifest), "an arg name without a value sets nothing")

	notes := evalHeader(oneKeyManifest(nil)).NotesBullets
	require.Len(t, notes, 2)
	assert.Equal(t, "start commit a36dd5b4", notes[1])
	assert.NotContains(t, strings.Join(notes, " "), "judge", "the judge model is not run configuration")
	assert.NotContains(t, strings.Join(notes, " "), "auth", "the token label is not run configuration")
}

func TestFold_noPreset(t *testing.T) {
	runDir := writeArmRunDir(t, oneKeyManifest([]string{}), sessionlessArm, sessionlessArm)
	compareIds := []string{metricDefectsMajor, metricQualityRank}
	armRuntime := `{"findings": [], "runs": [{"id": 1, "anchors": [], "failures": [{"check": "value_provenance", "count": 1}], "findings": 1, "reason": "", "session": "s-1", "skill": "heartbeat-demo", "state": "closed", "title": "", "version": "2.1.2"}]}`
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", runtimeFile), armRuntime)

	err := Fold(context.Background(), peek.NewClient(""), runDir, realSchemaPath)
	require.NoError(t, err)

	file := readEvalFile(t, runDir)
	report, err := os.ReadFile(filepath.Join(runDir, reportFile))
	require.NoError(t, err)
	require.Len(t, file.Runs, 2)
	assert.Equal(t, "2.13", file.SchemaVersion)
	assert.Empty(t, file.Runs[0].ClaudeCodeVersion, "an arm without a stream records no CLI version")
	assert.Empty(t, definedMetricIds(file.Metrics, compareIds))
	assert.Empty(t, valuedMetricIds(file.MetricValues, compareIds, "opus-high-r1"))
	assert.Equal(t, "Step misses", metricLabel(file.Metrics, metricStepMissPct))
	assert.Equal(t, "no adherence.json", metricValueOf(file.MetricValues, metricStepMissPct, "opus-high-r1").Note)
	assert.InDelta(t, 1.0, metricValueOf(file.MetricValues, metricCheckFailures, "opus-high-r1").Value, 1e-9)
	assert.InDelta(t, 0.0, metricValueOf(file.MetricValues, metricRunsKilled, "opus-high-r1").Value, 1e-9)
	assert.Nil(t, metricValueOf(file.MetricValues, metricCheckFailures, "opus-high-r2").Value)
	assert.Equal(t, "no runtime.json", metricValueOf(file.MetricValues, metricCheckFailures, "opus-high-r2").Note)
	assert.Equal(t, "Enforce check failures", metricLabel(file.Metrics, metricCheckFailures))
	assert.Equal(t, "opus-high", file.Runs[0].Variant.Name)
	assert.Equal(t, "opus-high", file.Runs[1].Variant.Name)
	assert.Nil(t, metricValueOf(file.MetricValues, metricWallSeconds, "opus-high-r1").Value)
	assert.Equal(t, noStreamWallNote, metricValueOf(file.MetricValues, metricWallSeconds, "opus-high-r1").Note, "wall comes from the arm stream, not peek")
	assert.Nil(t, metricValueOf(file.MetricValues, metricActiveSeconds, "opus-high-r1").Value)
	assert.Equal(t, "no session id", metricValueOf(file.MetricValues, metricActiveSeconds, "opus-high-r1").Note)
	assert.Nil(t, metricValueOf(file.MetricValues, metricPermissionDenials, "opus-high-r1").Value)
	assert.Equal(t, noEnvelopeNote, metricValueOf(file.MetricValues, metricPermissionDenials, "opus-high-r1").Note)
	assert.Equal(t, noEnvelopeNote, metricValueOf(file.MetricValues, metricOutputTokens, "opus-high-r1").Note)
	assert.Equal(t, "Main-loop active", metricLabel(file.Metrics, metricActiveSeconds))
	assert.Equal(t, "Output tokens (all agents)", metricLabel(file.Metrics, metricOutputTokens))
	assert.Equal(t, "Permission denials (all agents)", metricLabel(file.Metrics, metricPermissionDenials))
	assert.Nil(t, file.Golden)
	assert.Contains(t, string(report), "| Wall | Main-loop active | Output tokens (all agents) | Cost | Permission denials (all agents) |")
	assert.Contains(t, string(report), "opus-high-r1")
	assert.Contains(t, string(report), "opus-high-r2")
	assert.NoFileExists(t, filepath.Join(runDir, rejectedFileName))
}

func TestFold_scorer(t *testing.T) {
	scoredArm := `{"exitStatus":0,"sessionId":"","subtype":"success","terminalReason":"completed","scores":{"tests_passed":3}}`
	runDir := writeArmRunDir(t, oneKeyManifest([]string{"sh", "-c", "true"}), scoredArm, sessionlessArm)
	scoreId := scorePrefix + "tests_passed"
	expectedDef := evals.Metric{Id: scoreId, Direction: "", Label: "tests_passed (scorer)", Source: sourceScorer, Unit: "score"}

	err := Fold(context.Background(), peek.NewClient(""), runDir, realSchemaPath)
	require.NoError(t, err)

	file := readEvalFile(t, runDir)
	report, err := os.ReadFile(filepath.Join(runDir, reportFile))
	require.NoError(t, err)
	assert.Equal(t, expectedDef, metricOf(file.Metrics, scoreId))
	assert.InDelta(t, 3.0, metricValueOf(file.MetricValues, scoreId, "opus-high-r1").Value, 1e-9)
	assert.Nil(t, metricValueOf(file.MetricValues, scoreId, "opus-high-r2").Value)
	assert.Equal(t, "no score — runs/B/scorer.log", metricValueOf(file.MetricValues, scoreId, "opus-high-r2").Note)
	assert.Empty(t, probeResults(file.Probes, "scorer", "opus-high-r1"))
	assert.Equal(t, []string{"no scores — see runs/B/scorer.log"}, probeResults(file.Probes, "scorer", "opus-high-r2"))
	assert.Contains(t, string(report), "| Rank | tests_passed |")
	assert.Contains(t, string(report), "| 3 |")
	assert.NoFileExists(t, filepath.Join(runDir, rejectedFileName))
}

func TestFold_notObservable(t *testing.T) {
	runDir := writeArmRunDir(t, oneKeyManifest([]string{}), sessionlessArm, sessionlessArm)
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", adherenceFile), `{"entries": [
		{"id": "SKILL-X-RUN-001", "title": "Plan binds", "verdict": "followed", "evidence": "trace.md:3", "note": ""},
		{"id": "SKILL-X-RUN-002", "title": "Stop on red", "verdict": "violated", "evidence": "trace.md:9", "note": "kept going"},
		{"id": "SKILL-X-RUN-003", "title": "Lane reads", "verdict": "not-observable", "evidence": "", "note": "workflow agent"},
		{"id": "SKILL-X-RUN-004", "title": "Golden mode", "verdict": "not-applicable", "evidence": "", "note": ""}
	]}`)

	err := Fold(context.Background(), peek.NewClient(""), runDir, realSchemaPath)
	require.NoError(t, err)

	file := readEvalFile(t, runDir)
	report, err := os.ReadFile(filepath.Join(runDir, reportFile))
	require.NoError(t, err)
	assert.InDelta(t, 50.0, metricValueOf(file.MetricValues, metricStepMissPct, "opus-high-r1").Value, 1e-9)
	assert.InDelta(t, 1.0, metricValueOf(file.MetricValues, metricStepUnobserved, "opus-high-r1").Value, 1e-9)
	assert.Nil(t, metricValueOf(file.MetricValues, metricStepUnobserved, "opus-high-r2").Value)
	assert.Equal(t, "no adherence.json", metricValueOf(file.MetricValues, metricStepUnobserved, "opus-high-r2").Note)
	assert.Equal(t, "Unobservable entries", metricLabel(file.Metrics, metricStepUnobserved))
	assert.Len(t, file.Scores, 2)
	assert.Contains(t, string(report), "## Unobservable entries\n\n- `opus-high-r1` SKILL-X-RUN-003 — Lane reads: workflow agent\n")
}

func TestFold_unknownVerdict(t *testing.T) {
	runDir := writeArmRunDir(t, oneKeyManifest([]string{}), sessionlessArm, sessionlessArm)
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", adherenceFile), `{"entries": [
		{"id": "SKILL-X-RUN-001", "title": "Plan binds", "verdict": "mostly-followed", "evidence": "", "note": ""}
	]}`)

	err := Fold(context.Background(), peek.NewClient(""), runDir, realSchemaPath)

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(runDir, evalFileName))
}

func TestFold_compareOnlyWhenCompared(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedDefinedIds []string
		_expectedValuedIds  []string

		runDir string
	}

	compareIds := []string{metricDefectsMajor, metricQualityRank}
	compareJson := `{
		"arms": [
			{"label": "A", "summary": "tight diff", "defects": [{"severity": "MAJOR", "file": "a.go", "claim": "ignored the gate"}]},
			{"label": "B", "summary": "missed a stop", "defects": []}
		],
		"ranking": ["A", "B"]
	}`
	tests := make([]*testCase, 0)

	// two-keys-compared
	comparedDir := writeArmRunDir(t, twoArmManifest(), sessionlessArm, sessionlessArm)
	writeTestFile(t, filepath.Join(comparedDir, compareFile), compareJson)
	tests = append(tests, &testCase{
		_id:                 "two-keys-compared",
		_expectedDefinedIds: compareIds,
		_expectedValuedIds:  compareIds,

		runDir: comparedDir,
	})

	// one-key-stray-compare-ignored
	strayDir := writeArmRunDir(t, oneKeyManifest([]string{}), sessionlessArm, sessionlessArm)
	writeTestFile(t, filepath.Join(strayDir, compareFile), compareJson)
	tests = append(tests, &testCase{
		_id:                 "one-key-stray-compare-ignored",
		_expectedDefinedIds: []string{},
		_expectedValuedIds:  []string{},

		runDir: strayDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := Fold(context.Background(), peek.NewClient(""), test.runDir, realSchemaPath)
			require.NoError(t, err)
			file := readEvalFile(t, test.runDir)
			assert.Equal(t, test._expectedDefinedIds, definedMetricIds(file.Metrics, compareIds))
			assert.Equal(t, test._expectedValuedIds, valuedMetricIds(file.MetricValues, compareIds, "opus-high-r1"))
		})
	}
}

func TestFold_goldenPreset(t *testing.T) {
	runDir := goldenPresetRunDir(t)
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", candidatesFile), `[
		{"finding_id": "F1", "severity": "MAJOR", "route": "human"}
	]`)

	err := Fold(context.Background(), peek.NewClient(""), runDir, realSchemaPath)
	require.NoError(t, err)

	file := readEvalFile(t, runDir)
	require.NotNil(t, file.Golden)
	require.NotNil(t, file.Verdict)
	assert.Equal(t, "g-1", file.Golden.GoldenId)
	assert.Equal(t, 2, file.Golden.Total)
	assert.InDelta(t, 100, file.Golden.OracleMinutes, 1e-9, "90 + 10")
	assert.InDelta(t, 280, file.Golden.NullMinutes, 1e-9, "90 × 3 + 10 × 1")
	assert.Len(t, file.Golden.PerRun, 1)
	assert.InDelta(t, 1.0, metricValueOf(file.MetricValues, metricReviewScore, "opus-high-r1").Value, 1e-9, "a missed NIT escapes at ×1")
	assert.InDelta(t, 0.0, metricValueOf(file.MetricValues, metricLostMinutes, "opus-high-r1").Value, 1e-9)
	assert.Contains(t, []string{statusPass, statusMixed, statusFail}, file.Verdict.Status)
	assert.Equal(t, statusFail, file.Verdict.Status, "a sessionless arm fails smoke")
}

func TestFold_goldenMissingCandidates(t *testing.T) {
	runDir := goldenPresetRunDir(t)

	err := Fold(context.Background(), peek.NewClient(""), runDir, realSchemaPath)
	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(runDir, "eval.json"))
}

// goldenPresetRunDir writes a one-arm golden test run dir — run.json with its
// golden block, the golden (MAJOR + NIT) and minutes scoring snapshots, a
// judge that matched G1 with F1 — everything but the arm's candidates.json.
func goldenPresetRunDir(t *testing.T) string {
	t.Helper()
	runDir := t.TempDir()
	writeTestFile(t, filepath.Join(runDir, scoringSnapshotFile), `{
		"judgeGuide": "same defect",
		"minutes": {"BLOCKER": 120, "MAJOR": 90, "MINOR": 30, "NIT": 10, "INFO": 2},
		"escapeMultiplier": {"BLOCKER": 5, "MAJOR": 3, "MINOR": 2, "NIT": 1, "INFO": 1},
		"triageLine": "MINOR",
		"fixFactor": {},
		"thresholds": {"review_score": 0.5, "false_positive_minutes": 60}
	}`)
	writeTestFile(t, filepath.Join(runDir, goldenSnapshotFile), `{
		"goldenVersion": 1,
		"findings": [
			{"golden_id": "G1", "file": "a.go", "severity": "MAJOR"},
			{"golden_id": "G2", "file": "b.go", "severity": "NIT"}
		]
	}`)

	manifest := &RunManifest{
		Arms:           []ManifestArm{{Id: "opus-high-r1", DisabledServers: []string{}, Effort: "high", Key: "opus-high", Label: "A", Model: "opus", Replica: 1, Worktree: "/wt/skilleval-run-A"}},
		At:             "a36dd5b46f542542cb58401ef7cf2d7452eb28f9",
		BudgetUsd:      45,
		Date:           "2026-09-26",
		Findings:       evals.DefinitionFindings{ArrayKey: "ranked_plan", ClaimKeys: []string{"claim", "finding"}, Glob: "round-*/review.json"},
		Golden:         &ManifestGolden{Id: "g-1", Kind: GoldenTest, Range: evals.DefinitionRange{BaseSha: "", HeadSha: ""}, Scope: "full", SeedFile: ""},
		Judge:          "opus",
		PresetSkillDir: "",
		Prompt:         "/railroad-review",
		Repo:           "/repo",
		Skill:          foldSkill,
		SkillMd:        "evals/railroad-review/2026-09-26-a36dd5/SKILL.md",
		TimeoutS:       7200,
	}
	writeRunDir(t, runDir, manifest)
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", resultFile), sessionlessArm)
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", judgeFile), `{
		"matched": [{"findingId": "F1", "goldenId": "G1", "note": "same nil write"}],
		"falsePositives": [],
		"misclassified": [],
		"uncataloged": []
	}`)
	return runDir
}

func TestFold_schemaViolation(t *testing.T) {
	runDir := t.TempDir()
	writeRunDir(t, runDir, twoArmManifest())
	writeTestFile(t, filepath.Join(runDir, runsDir, "A", resultFile), sessionlessArm)
	writeTestFile(t, filepath.Join(runDir, runsDir, "B", resultFile), sessionlessArm)
	schemaPath := filepath.Join(t.TempDir(), "reject-all.json")
	writeTestFile(t, schemaPath, `{"type": "string"}`)

	err := Fold(context.Background(), peek.NewClient(""), runDir, schemaPath)

	require.Error(t, err)
	assert.FileExists(t, filepath.Join(runDir, rejectedFileName))
	assert.NoFileExists(t, filepath.Join(runDir, evalFileName))
	assert.NoFileExists(t, filepath.Join(runDir, reportFile))
}
