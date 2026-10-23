package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/skilleval"
)

func TestBuildEvalView(t *testing.T) {
	rubric := []evals.RubricRule{
		{Id: "SKILL-DEMO-STEP-001", Axis: "self", Phase: "step", Rule: "Rule one"},
		{Id: "SKILL-DEMO-OUT-001", Axis: "self", Phase: "step", Rule: "Out rule"},
		{Id: "SKILL-DEMO-STEP-002", Axis: "self", Phase: "gate", Rule: "Rule two"},
		{Id: "CTX-001", Axis: "context", Phase: "step", Rule: "Context rule"},
	}
	scores := []evals.ScoreCell{
		{RuleId: "SKILL-DEMO-STEP-001", RunId: "r1", Score: 1, Source: "agent"},
		{RuleId: "SKILL-DEMO-STEP-001", RunId: "r2", Score: -1, Source: "probe", Justification: "violated", Evidence: []string{"a", "b"}},
		{RuleId: "SKILL-DEMO-STEP-002", RunId: "r1", Score: 0, Source: "agent", Justification: "not shown"},
		{RuleId: "SKILL-DEMO-STEP-002", RunId: "r2", Score: 1, Source: "agent"},
		{RuleId: "CTX-001", RunId: "r1", Score: 1, Source: "agent"},
	}
	totals := []evals.Total{
		{RunId: "r1", Axis: "self", Raw: 1, Max: 2, Pct: 50},
		{RunId: "r2", Axis: "self", Raw: 0, Max: 2, Pct: 0},
		{RunId: "r1", Axis: "context", Raw: 1, Max: 1, Pct: 100},
	}
	runs := []evals.Run{
		{Id: "r1", Model: evals.Model{Id: "fable"}},
		{Id: "r2", Model: evals.Model{Id: "fable"}, Variant: evals.Variant{Name: "lean"}},
	}
	file := evals.EvalFile{Rubric: rubric, Runs: runs, Scores: scores, Totals: totals}
	evalDir := evals.EvalDir{Dir: "demo-2026-07-11", Eval: file}

	view := buildEvalView(evalDir)

	// run labels in run order
	require.Len(t, view.Runs, 2)
	assert.Equal(t, "R1", view.Runs[0].Label)
	assert.Equal(t, "R2", view.Runs[1].Label)

	// axis grouping follows rubric order
	require.Len(t, view.Axes, 2)
	assert.Equal(t, "self", view.Axes[0].Axis)
	assert.Equal(t, "context", view.Axes[1].Axis)

	// one group per topic in first-seen order, an interleaved entry merged into its topic; no header for the context id
	require.Len(t, view.Axes[0].Groups, 2)
	assert.Equal(t, "OUT", view.Axes[0].Groups[1].Topic)
	assert.Len(t, view.Axes[0].Groups[1].Rows, 1)
	stepGroup := view.Axes[0].Groups[0]
	assert.Equal(t, "STEP", stepGroup.Topic)
	assert.Equal(t, 2, stepGroup.Pos)
	assert.Equal(t, 1, stepGroup.Zero)
	assert.Equal(t, 1, stepGroup.Neg)
	assert.InDelta(t, 50.0, stepGroup.PosPct, 0.01)
	assert.InDelta(t, 25.0, stepGroup.ZeroPct, 0.01)
	assert.InDelta(t, 25.0, stepGroup.NegPct, 0.01)
	require.Len(t, view.Axes[1].Groups, 1)
	assert.Empty(t, view.Axes[1].Groups[0].Topic)

	// score cells map to class and glyph; absent cell renders as dash
	selfRows := stepGroup.Rows
	require.Len(t, selfRows, 2)
	assert.Equal(t, "Rule one", selfRows[0].Rule.Rule)
	assert.Equal(t, evalScoreCell{Class: "score-pos", Text: "✓"}, selfRows[0].Cells[0])
	assert.Equal(t, "score-neg", selfRows[0].Cells[1].Class)
	assert.Equal(t, "✗", selfRows[0].Cells[1].Text)
	assert.Equal(t, "violated", selfRows[0].Cells[1].Title)
	assert.Equal(t, "◐", selfRows[1].Cells[0].Text)
	contextRow := view.Axes[1].Groups[0].Rows[0]
	assert.Equal(t, "score-absent", contextRow.Cells[1].Class)
	assert.Equal(t, "—", contextRow.Cells[1].Text)

	// non-+1 cells collected as justifications with joined evidence and the entry title
	require.Len(t, view.Axes[0].Justifications, 2)
	assert.Equal(t, "a; b", view.Axes[0].Justifications[0].Evidence)
	assert.Equal(t, "R2", view.Axes[0].Justifications[0].RunLabel)
	assert.Equal(t, -1, view.Axes[0].Justifications[0].Score)
	assert.Equal(t, "Rule one", view.Axes[0].Justifications[0].RuleTitle)
	assert.Empty(t, view.Axes[1].Justifications)

	// rankings sorted axis asc, pct desc
	require.Len(t, view.Rankings, 3)
	assert.Equal(t, "context", view.Rankings[0].Axis)
	assert.Equal(t, "R1", view.Rankings[1].RunLabel)
	assert.Equal(t, "R2", view.Rankings[2].RunLabel)
}

func TestEntryTopic(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		id string
	}

	tests := make([]*testCase, 0)

	// skill-id-topic
	tests = append(tests, &testCase{
		_id:       "skill-id-topic",
		_expected: "PROBE",

		id: "SKILL-RAILROADREVIEW-PROBE-006",
	})

	// letter-suffix-id
	tests = append(tests, &testCase{
		_id:       "letter-suffix-id",
		_expected: "NIGHTLY",

		id: "SKILL-RAILROADREVIEW-NIGHTLY-001a",
	})

	// context-id-empty
	tests = append(tests, &testCase{
		_id:       "context-id-empty",
		_expected: "",

		id: "ACTION-IMPL-EXEC-001",
	})

	// short-id-empty
	tests = append(tests, &testCase{
		_id:       "short-id-empty",
		_expected: "",

		id: "SKILL-A-001",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, entryTopic(test.id))
		})
	}
}

func TestBuildVerdictView(t *testing.T) {
	metrics := []evals.Metric{
		{Id: "recall", Direction: "higher", Label: "Golden recall", Unit: "ratio"},
		{Id: "false_positives", Direction: "lower", Label: "False positives", Unit: "count"},
	}
	labels := map[string]string{"r1": "R1", "r2": "R2"}
	verdict := &evals.Verdict{
		BestArm: "sonnet-5-high",
		PerRun: []evals.RunVerdict{
			{RunId: "r1", Pass: false, FailedThresholds: []string{"recall", "false_positives"}},
			{RunId: "r2", Pass: false, FailedThresholds: []string{"recall"}},
		},
		Reason:     "best arm = highest golden recall, lower cost breaks ties",
		Status:     "fail",
		Thresholds: map[string]float64{"recall": 0.6, "false_positives": 5},
	}

	file := evals.EvalFile{
		Metrics: metrics,
		MetricValues: []evals.MetricValue{
			{MetricId: "recall", RunId: "r1", Value: 0.5},
			{MetricId: "false_positives", RunId: "r1", Value: 4.0},
		},
		Runs:    []evals.Run{{Id: "r1", Variant: evals.Variant{Name: "sonnet-5-high"}}, {Id: "r2", Variant: evals.Variant{Name: "other"}}},
		Verdict: verdict,
	}
	view := buildVerdictView(file, labels)

	require.NotNil(t, view)
	assert.Equal(t, "Fail — no run reached the thresholds", view.Headline)
	assert.Equal(t, "fail", view.Status)
	assert.True(t, view.IsFail)
	assert.Equal(t, []string{"False positives ≤ 5", "Golden recall ≥ 60%"}, view.Thresholds)
	require.Len(t, view.Runs, 2)
	assert.Equal(t, "R1", view.Runs[0].RunLabel)
	assert.Equal(t, "Golden recall, False positives", view.Runs[0].Failed)
	assert.False(t, view.Runs[0].Pass)

	// per-threshold checks for the best arm: value, mark and bound per metric
	assert.Equal(t, []string{"False positives 4 ✓ (False positives ≤ 5)", "Golden recall 50% ✗ (Golden recall ≥ 60%)"}, view.Checks)

	assert.Nil(t, buildVerdictView(evals.EvalFile{Metrics: metrics}, labels))

	mixed := evals.EvalFile{Metrics: metrics, Verdict: &evals.Verdict{Status: "mixed", PerRun: []evals.RunVerdict{{RunId: "r1", Pass: true}, {RunId: "r2"}}}}
	assert.Equal(t, "Mixed — 1 of 2 runs reached the thresholds", buildVerdictView(mixed, labels).Headline)

	weird := evals.EvalFile{Metrics: metrics, Verdict: &evals.Verdict{Status: "weird"}}
	assert.Equal(t, "Verdict: weird", buildVerdictView(weird, labels).Headline)
	assert.Equal(t, "weird", buildVerdictView(weird, labels).Status)
}

func TestRunTokens(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedFound  bool
		_expectedNumber float64
		_expectedSource string

		arm           string
		armRuns       []evals.ArmRun
		runId         string
		schemaVersion string
	}

	agentTokens := 427765.0
	measuredSibling := 141999.0
	armRuns := []evals.ArmRun{
		{Id: "r1", Arm: "sonnet-5-high", AgentTokens: &agentTokens},
		{Id: "r2", Arm: "sonnet-5-high"},
		{Id: "r3", Arm: "sonnet-5-high", OutputTokens: &measuredSibling},
	}
	values := map[string]float64{"output_tokens\x00r0": 189603}

	tests := make([]*testCase, 0)

	// schema-2.9-measured
	tests = append(tests, &testCase{
		_id:             "schema-2.9-measured",
		_expectedFound:  true,
		_expectedNumber: 189603,
		_expectedSource: "",

		arm:           "sonnet-5-high",
		armRuns:       armRuns,
		runId:         "r0",
		schemaVersion: "2.9",
	})

	// schema-2.8-tagged-main
	tests = append(tests, &testCase{
		_id:             "schema-2.8-tagged-main",
		_expectedFound:  true,
		_expectedNumber: 189603,
		_expectedSource: "main",

		arm:           "sonnet-5-high",
		armRuns:       armRuns,
		runId:         "r0",
		schemaVersion: "2.8",
	})

	// legacy-empty-version-tagged-main
	tests = append(tests, &testCase{
		_id:             "legacy-empty-version-tagged-main",
		_expectedFound:  true,
		_expectedNumber: 189603,
		_expectedSource: "main",

		arm:           "sonnet-5-high",
		armRuns:       armRuns,
		runId:         "r0",
		schemaVersion: "",
	})

	// agent-tokens-rung-unchanged
	tests = append(tests, &testCase{
		_id:             "agent-tokens-rung-unchanged",
		_expectedFound:  true,
		_expectedNumber: agentTokens,
		_expectedSource: "≈",

		arm:           "sonnet-5-high",
		armRuns:       armRuns,
		runId:         "r1",
		schemaVersion: "2.8",
	})

	// sibling-mean-rung
	tests = append(tests, &testCase{
		_id:             "sibling-mean-rung",
		_expectedFound:  true,
		_expectedNumber: (agentTokens + measuredSibling) / 2,
		_expectedSource: "est.",

		arm:           "sonnet-5-high",
		armRuns:       armRuns,
		runId:         "r2",
		schemaVersion: "2.9",
	})

	// no-rung
	tests = append(tests, &testCase{
		_id:             "no-rung",
		_expectedFound:  false,
		_expectedNumber: 0,
		_expectedSource: "",

		arm:           "other",
		armRuns:       nil,
		runId:         "r9",
		schemaVersion: "2.9",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			number, source, ok := runTokens(values, test.armRuns, test.runId, test.arm, test.schemaVersion)
			assert.Equal(t, test._expectedFound, ok)
			assert.InDelta(t, test._expectedNumber, number, 0.01)
			assert.Equal(t, test._expectedSource, source)
		})
	}
}

func TestUnionGoldenFound(t *testing.T) {
	armRuns := []evals.ArmRun{
		{Id: "r1", Judge: evals.ArmRunJudge{Matched: []evals.ArmRunJudgeMatch{{GoldenId: "g1"}, {GoldenId: "g2"}}}},
		{Id: "r2", Judge: evals.ArmRunJudge{Matched: []evals.ArmRunJudgeMatch{{GoldenId: "g2"}, {GoldenId: "g3"}}}},
	}

	found, ok := unionGoldenFound(armRuns)
	assert.True(t, ok)
	assert.Equal(t, 3, found, "overlap deduplicated")

	_, ok = unionGoldenFound(nil)
	assert.False(t, ok)
}

func TestWorseTokenSource(t *testing.T) {
	assert.Empty(t, worseTokenSource("", ""))
	assert.Equal(t, "≈", worseTokenSource("", "≈"))
	assert.Equal(t, "est.", worseTokenSource("≈", "est."))
	assert.Equal(t, "est.", worseTokenSource("est.", "≈"))
}

func TestFormatMetricValue(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		unit  string
		value float64
	}

	tests := make([]*testCase, 0)

	// ratio
	tests = append(tests, &testCase{
		_id:       "ratio",
		_expected: "12%",

		unit:  "ratio",
		value: 0.12195,
	})

	// ratio-negative-zero
	tests = append(tests, &testCase{
		_id:       "ratio-negative-zero",
		_expected: "0%",

		unit:  "ratio",
		value: -0.0005,
	})

	// ratio-negative
	tests = append(tests, &testCase{
		_id:       "ratio-negative",
		_expected: "-20%",

		unit:  "ratio",
		value: -0.2,
	})

	// usd
	tests = append(tests, &testCase{
		_id:       "usd",
		_expected: "$8.91",

		unit:  "USD",
		value: 8.9095,
	})

	// ms-seconds
	tests = append(tests, &testCase{
		_id:       "ms-seconds",
		_expected: "59 s",

		unit:  "ms",
		value: 59000,
	})

	// ms-minutes
	tests = append(tests, &testCase{
		_id:       "ms-minutes",
		_expected: "26.5 min",

		unit:  "ms",
		value: 1592468,
	})

	// tokens-grouped
	tests = append(tests, &testCase{
		_id:       "tokens-grouped",
		_expected: "60 390",

		unit:  "tokens",
		value: 60390,
	})

	// count-small
	tests = append(tests, &testCase{
		_id:       "count-small",
		_expected: "2",

		unit:  "count",
		value: 2,
	})

	// seconds
	tests = append(tests, &testCase{
		_id:       "seconds",
		_expected: "376.5 min",

		unit:  "s",
		value: 22590,
	})

	// unknown-unit
	tests = append(tests, &testCase{
		_id:       "unknown-unit",
		_expected: "1.5",

		unit:  "bool",
		value: 1.5,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, formatMetricValue(test.unit, test.value))
		})
	}
}

func makeArmTilesDir(runs []evals.Run, values []evals.MetricValue, golden *evals.Golden) evals.EvalDir {
	return evals.EvalDir{
		Dir: "railroad-review/2026-09-06-a34281",
		Eval: evals.EvalFile{
			Eval:         evals.Eval{Skill: "railroad-review", Date: "2026-09-06"},
			Runs:         runs,
			MetricValues: values,
			Golden:       golden,
		},
	}
}

func TestBuildResultRows(t *testing.T) {
	runs := []evals.Run{
		{Id: "sonnet-5-high-r1", Variant: evals.Variant{Name: "sonnet-5-high"}, Transcript: "sess-a"},
		{Id: "sonnet-5-high-r2", Variant: evals.Variant{Name: "sonnet-5-high"}, Invalid: true, InvalidReason: "rounds-incomplete"},
	}
	values := []evals.MetricValue{
		{MetricId: metricReviewScore, RunId: "sonnet-5-high-r1", Value: 0.38},
		{MetricId: metricLostMinutes, RunId: "sonnet-5-high-r1", Value: 1023.4},
		{MetricId: "cost_usd", RunId: "sonnet-5-high-r1", Value: 222.48},
		{MetricId: metricWallSeconds, RunId: "sonnet-5-high-r1", Value: float64(3992)},
		{MetricId: metricActiveSeconds, RunId: "sonnet-5-high-r1", Value: float64(600)},
		{MetricId: "output_tokens", RunId: "sonnet-5-high-r1", Value: float64(141999)},
	}
	golden := &evals.Golden{
		Total: 100,
		PerRun: []evals.GoldenRun{{
			RunId:          "sonnet-5-high-r1",
			FalsePositives: []evals.GoldenFalsePositive{{FindingId: "f1"}},
			Matched:        make([]evals.GoldenMatch, 40),
			Misclassified:  make([]evals.GoldenMisclassified, 10),
			Missed:         make([]string, 60),
			Uncataloged:    []evals.GoldenFalsePositive{{FindingId: "f2"}},
		}},
	}
	labels := map[string]string{"sonnet-5-high-r1": "R1", "sonnet-5-high-r2": "R2"}

	rows := buildResultRows(makeArmTilesDir(runs, values, golden), map[string]float64{metricReviewScore: 0.5}, labels)
	require.Len(t, rows, 2)
	row := rows[0]
	assert.Equal(t, "sonnet-5-high", row.Arm)
	assert.Equal(t, "R1", row.Label)
	assert.Equal(t, "sess-a", row.SessionId)
	assert.Equal(t, "38%", row.ReviewScore)
	assert.Equal(t, "tile-fail", row.ReviewScoreClass, "below the review_score bound")
	assert.Equal(t, "1 023", row.LostMinutes, "minutes as a bare grouped number; the header carries the unit")
	assert.Equal(t, "$222.48", row.Cost)
	assert.Equal(t, "66.5 min", row.WallTime)
	assert.Equal(t, "10.0 min", row.ActiveTime)
	assert.Equal(t, 40, row.Found)
	assert.Equal(t, 60, row.Missed)
	assert.Equal(t, 1, row.FalsePositives)
	assert.Equal(t, 10, row.Misclassified)
	assert.Equal(t, 1, row.NewFinds)
	assert.InDelta(t, 60.0, row.MissedPct, 0.01)
	assert.Equal(t, "141 999", row.Tokens, "measured tokens rung")
	assert.Equal(t, "main", row.TokensSource, "a schema-less eval.json counted the main loop only")

	assert.True(t, rows[1].Invalid)
	assert.Equal(t, "rounds-incomplete", rows[1].InvalidReason)
	assert.Equal(t, "—", rows[1].WallTime, "missing telemetry renders a dash")
	assert.Equal(t, "—", rows[1].ActiveTime, "missing telemetry renders a dash")
	assert.Equal(t, "—", rows[1].ReviewScore, "no score for this run renders a dash")
	assert.Empty(t, rows[1].ReviewScoreClass)
}

func TestReviewScoreClass(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		runId      string
		thresholds map[string]float64
	}

	values := map[string]float64{"review_score\x00low": 0.4, "review_score\x00at": 0.5}
	bound := map[string]float64{metricReviewScore: 0.5}
	tests := make([]*testCase, 0)

	// no-score-empty
	tests = append(tests, &testCase{
		_id:       "no-score-empty",
		_expected: "",

		runId:      "missing",
		thresholds: bound,
	})

	// below-bound-fail
	tests = append(tests, &testCase{
		_id:       "below-bound-fail",
		_expected: "tile-fail",

		runId:      "low",
		thresholds: bound,
	})

	// at-bound-pass
	tests = append(tests, &testCase{
		_id:       "at-bound-pass",
		_expected: "tile-pass",

		runId:      "at",
		thresholds: bound,
	})

	// no-bound-pass
	tests = append(tests, &testCase{
		_id:       "no-bound-pass",
		_expected: "tile-pass",

		runId:      "low",
		thresholds: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, reviewScoreClass(values, test.runId, test.thresholds))
		})
	}
}

func TestBuildCombinedView(t *testing.T) {
	agentTokens1 := 400000.0
	agentTokens2 := 500000.0
	runs := []evals.Run{
		{Id: "r1", Variant: evals.Variant{Name: "sonnet-5-high"}},
		{Id: "r2", Variant: evals.Variant{Name: "sonnet-5-high"}},
		{Id: "r3", Variant: evals.Variant{Name: "sonnet-5-high"}, Invalid: true, InvalidReason: "lost"},
	}
	values := []evals.MetricValue{
		{MetricId: metricReviewScore, RunId: "r1", Value: 0.02},
		{MetricId: metricReviewScore, RunId: "r2", Value: 0.04},
		{MetricId: metricReviewScore, RunId: "r3", Value: 0.99},
		{MetricId: metricLostMinutes, RunId: "r1", Value: 300.0},
		{MetricId: metricLostMinutes, RunId: "r2", Value: 500.0},
		{MetricId: metricEscapedMinutes, RunId: "r1", Value: 1000.0},
		{MetricId: metricEscapedMinutes, RunId: "r2", Value: 2000.0},
		{MetricId: metricFalsePositiveMinutes, RunId: "r1", Value: 10.0},
		{MetricId: metricFalsePositiveMinutes, RunId: "r2", Value: 30.0},
		{MetricId: metricWallSeconds, RunId: "r1", Value: 60.0},
		{MetricId: metricWallSeconds, RunId: "r2", Value: 120.0},
		{MetricId: metricGoldenCoverage, RunId: "r1", Value: 0.2},
		{MetricId: metricGoldenCoverage, RunId: "r2", Value: 0.4},
	}
	golden := &evals.Golden{
		Total: 100,
		PerRun: []evals.GoldenRun{
			{RunId: "r1", Matched: make([]evals.GoldenMatch, 3), FalsePositives: []evals.GoldenFalsePositive{{FindingId: "f1"}}, Uncataloged: []evals.GoldenFalsePositive{{FindingId: "u1"}}},
			{RunId: "r2", Matched: make([]evals.GoldenMatch, 2)},
		},
	}
	evalDir := makeArmTilesDir(runs, values, golden)
	evalDir.ArmRuns = []evals.ArmRun{
		{Id: "r1", Arm: "sonnet-5-high", AgentTokens: &agentTokens1, Judge: evals.ArmRunJudge{Matched: []evals.ArmRunJudgeMatch{{GoldenId: "g1"}, {GoldenId: "g2"}}}},
		{Id: "r2", Arm: "sonnet-5-high", AgentTokens: &agentTokens2, Judge: evals.ArmRunJudge{Matched: []evals.ArmRunJudgeMatch{{GoldenId: "g2"}, {GoldenId: "g3"}}}},
	}
	verdict := &evalVerdictView{
		IsFail: true,
		Runs:   []evalRunVerdictRow{{RunLabel: "R1"}, {RunLabel: "R2"}, {RunLabel: "R3"}},
		Status: "fail",
	}
	labels := map[string]string{"r1": "R1", "r2": "R2", "r3": "R3"}
	rows := buildResultRows(evalDir, nil, labels)

	combined := buildCombinedView(evalDir, verdict, rows)
	require.NotNil(t, combined)
	assert.Equal(t, "FAIL — 0/3 runs pass", combined.PassLine)
	assert.True(t, combined.IsFail)
	assert.Equal(t, 3, combined.Runs)
	assert.Equal(t, "3%", combined.ReviewScore, "mean over valid runs only")
	assert.Equal(t, "tile-pass", combined.ReviewScoreClass, "no verdict bound")
	assert.Equal(t, "400", combined.LostMinutes, "mean over valid runs only")
	assert.Equal(t, "1 500", combined.EscapedMinutes, "mean over valid runs only")
	assert.Equal(t, "20", combined.FalsePositiveMinutes, "mean over valid runs only")
	assert.Nil(t, combined.MinutesBar, "the bar is the page data's, built from the headline run")
	assert.Equal(t, "mean ", combined.MeanPrefix)
	assert.Equal(t, "2.0 min", combined.RunWall, "no runtimeS — longest arm as lower bound")
	assert.Equal(t, "wall ≥ (longest arm)", combined.RunWallLabel)
	assert.Equal(t, "900 000", combined.TokensTotal)
	assert.Equal(t, "≈ tokens Σ", combined.TokensLabel)
	assert.Equal(t, 1, combined.NewFinds)
	assert.Equal(t, 1, combined.FalsePositives)
	// union found: g1,g2,g3 across armruns → 3 of 100
	assert.Equal(t, "3/100", combined.FoundLabel, "union found tile")
	assert.Equal(t, 3, combined.FoundCount)
	assert.Empty(t, combined.ArmLines, "single variant has no per-arm lines")
	assert.Equal(t, "30%", combined.GoldenCoverage, "mean coverage over valid runs")
	assert.Empty(t, combined.UnionCoverage, "no touched block — union tile gated off")

	// touched block present → union tile
	golden.Touched = &evals.GoldenTouched{GoldenFiles: 10, TouchedGoldenFiles: 6, UnionReachableGoldens: 40}
	touchedDir := makeArmTilesDir(runs, values, golden)
	touchedDir.ArmRuns = evalDir.ArmRuns
	touchedRows := buildResultRows(touchedDir, nil, labels)
	touchedCombined := buildCombinedView(touchedDir, verdict, touchedRows)
	assert.Equal(t, "40%", touchedCombined.UnionCoverage)
	golden.Touched = nil

	// no armruns → best-run fallback found tile
	evalDir.ArmRuns = nil
	rows = buildResultRows(evalDir, nil, labels)
	combined = buildCombinedView(evalDir, verdict, rows)
	assert.Equal(t, "3/100", combined.FoundLabel, "best-run fallback found tile")
	assert.Equal(t, "tokens Σ", combined.TokensLabel, "no token data on any rung")
	assert.Equal(t, "—", combined.TokensTotal)

	assert.Nil(t, buildCombinedView(evalDir, verdict, nil))

	// no coverage metric anywhere → gated off, not zero; a score below the bound fails the tile
	noCoverageValues := []evals.MetricValue{{MetricId: metricReviewScore, RunId: "r1", Value: 0.02}}
	noCoverageDir := makeArmTilesDir(runs, noCoverageValues, golden)
	noCoverageDir.Eval.Verdict = &evals.Verdict{Thresholds: map[string]float64{metricReviewScore: 0.5}}
	noCoverageRows := buildResultRows(noCoverageDir, nil, labels)
	noCoverageCombined := buildCombinedView(noCoverageDir, verdict, noCoverageRows)
	assert.Empty(t, noCoverageCombined.GoldenCoverage, "absent metric gates the tile off")
	assert.Equal(t, "tile-fail", noCoverageCombined.ReviewScoreClass)
	assert.Equal(t, "—", noCoverageCombined.LostMinutes, "no lost-minutes metric")
}

func TestMinutesBarFor(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *minutesBar

		golden    *evals.Golden
		goldenRun *evals.GoldenRun
	}

	golden := &evals.Golden{NullMinutes: 2000, OracleMinutes: 1000}
	insideRun := &evals.GoldenRun{Severities: []evals.GoldenSeverity{
		{Severity: "BLOCKER", EscapedMinutes: 600, FoundMinutes: 200},
		{Severity: "NIT", EscapedMinutes: 300, FalsePositiveMinutes: 100, FoundMinutes: 100},
	}}
	beyondRun := &evals.GoldenRun{Severities: []evals.GoldenSeverity{{Severity: "BLOCKER", EscapedMinutes: 2400, FoundMinutes: 600}}}

	tests := make([]*testCase, 0)

	// run-inside-null
	tests = append(tests, &testCase{
		_id: "run-inside-null",
		_expected: &minutesBar{
			EscapedLabel:       "900",
			EscapedPct:         45,
			FalsePositiveLabel: "100",
			FalsePositivePct:   5,
			FoundLabel:         "300",
			FoundPct:           15,
			NullLabel:          "2 000",
			NullPct:            100,
			OracleLabel:        "1 000",
			OraclePct:          50,
			RunLabel:           "R1",
			TotalLabel:         "1 300",
		},

		golden:    golden,
		goldenRun: insideRun,
	})

	// run-beyond-null-scales-axis
	tests = append(tests, &testCase{
		_id: "run-beyond-null-scales-axis",
		_expected: &minutesBar{
			EscapedLabel:       "2 400",
			EscapedPct:         80,
			FalsePositiveLabel: "0",
			FalsePositivePct:   0,
			FoundLabel:         "600",
			FoundPct:           20,
			NullLabel:          "2 000",
			NullPct:            66.66666666666667,
			OracleLabel:        "1 000",
			OraclePct:          33.333333333333336,
			RunLabel:           "R1",
			TotalLabel:         "3 000",
		},

		golden:    golden,
		goldenRun: beyondRun,
	})

	// nil-golden
	tests = append(tests, &testCase{
		_id:       "nil-golden",
		_expected: nil,

		golden:    nil,
		goldenRun: insideRun,
	})

	// nil-run
	tests = append(tests, &testCase{
		_id:       "nil-run",
		_expected: nil,

		golden:    golden,
		goldenRun: nil,
	})

	// no-breakdown
	tests = append(tests, &testCase{
		_id:       "no-breakdown",
		_expected: nil,

		golden:    golden,
		goldenRun: &evals.GoldenRun{RunId: "old"},
	})

	// no-null-minutes
	tests = append(tests, &testCase{
		_id:       "no-null-minutes",
		_expected: nil,

		golden:    &evals.Golden{OracleMinutes: 1000},
		goldenRun: insideRun,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, minutesBarFor(test.golden, test.goldenRun, "R1"))
		})
	}
}

func TestBuildSeverityBreakdown(t *testing.T) {
	scoring := &evals.Scoring{
		EscapeMultiplier: map[string]float64{"BLOCKER": 5, "NIT": 1, "CUSTOM": 2},
		FixFactor:        map[string]float64{"BLOCKER": 0.5, "NIT": 0.1, "CUSTOM": 0.25},
		Minutes:          map[string]float64{"BLOCKER": 120, "NIT": 10, "CUSTOM": 30},
		TriageLine:       "NIT",
	}
	golden := evals.DefinitionGolden{SeverityCounts: map[string]int{"BLOCKER": 2, "NIT": 3}}
	goldenRun := &evals.GoldenRun{Severities: []evals.GoldenSeverity{
		{Severity: "BLOCKER", Entries: 2, Found: 1, Missed: 1, FoundMinutes: 60, EscapedMinutes: 600, Outcomes: evals.SeverityOutcomes{
			Fixed:  evals.OutcomeTally{Count: 1, Minutes: 60},
			Missed: evals.OutcomeTally{Count: 1, Minutes: 600},
		}},
		{Severity: "NIT", Entries: 3, Found: 3, FalsePositives: 2, FoundMinutes: 100, EscapedMinutes: 10, FalsePositiveMinutes: 20, Outcomes: evals.SeverityOutcomes{
			AtPrice:       evals.OutcomeTally{Count: 1, Minutes: 10},
			FixFailed:     evals.OutcomeTally{Count: 1, Minutes: 10},
			WrongSeverity: evals.OutcomeTally{Count: 1, Minutes: 90},
		}},
	}}

	breakdown := buildSeverityBreakdown(golden, goldenRun, "R2", scoring)
	require.NotNil(t, breakdown)
	assert.True(t, breakdown.HasRun)
	assert.Equal(t, "R2", breakdown.RunLabel)
	require.Len(t, breakdown.Rows, 3)
	blocker := breakdown.Rows[0]
	assert.Equal(t, "BLOCKER", blocker.Severity, "severity order first")
	assert.Equal(t, "120", blocker.PriceFound)
	assert.Equal(t, "600 (×5)", blocker.PriceMissed)
	assert.Equal(t, "60 (×0.5)", blocker.PriceFixed)
	assert.Equal(t, 2, blocker.Entries)
	assert.Equal(t, 1, blocker.Found)
	assert.Equal(t, 1, blocker.Escaped)
	assert.Equal(t, "60", blocker.FoundMinutes)
	assert.Equal(t, "600", blocker.EscapedMinutes)
	assert.Equal(t, "240", blocker.OracleMinutes, "2 × 120")
	assert.Equal(t, "420", blocker.LostMinutes, "60 + 600 − 240")
	assert.False(t, blocker.IsTriageLine)
	nit := breakdown.Rows[1]
	assert.True(t, nit.IsTriageLine)
	assert.Equal(t, 2, nit.Found, "the entry whose fix failed counts as escaped, not found")
	assert.Equal(t, 1, nit.Escaped)
	assert.Equal(t, 2, nit.FalsePositives)
	assert.Equal(t, "20", nit.FalsePositiveMinutes)
	assert.Equal(t, "100", nit.LostMinutes, "100 + 10 + 20 − 30")
	custom := breakdown.Rows[2]
	assert.Equal(t, "CUSTOM", custom.Severity, "unknown severity last")
	assert.Zero(t, custom.Entries, "no entry of that severity in the run")
	assert.Equal(t, "0", custom.LostMinutes)
	assert.Equal(t, 5, breakdown.Totals.Entries)
	assert.Equal(t, 3, breakdown.Totals.Found)
	assert.Equal(t, 2, breakdown.Totals.Escaped)
	assert.Equal(t, "270", breakdown.Totals.OracleMinutes)
	assert.Equal(t, "520", breakdown.Totals.LostMinutes)
	assert.Equal(t, "review score 46% = 1 − lost 520 ÷ savable 960 (null 1 230 − oracle 270)", breakdown.ScoreLine)
	expectedNotes := []string{
		"BLOCKER: 1 found entry machine-fixed at 60 instead of 120 (60 min).",
		"NIT: 1 found entry counts as escaped, the machine fix left the defect in the tree (10 min). 1 found entry reported at another severity, +80 min over the price.",
	}
	assert.Equal(t, expectedNotes, breakdown.Notes, "a plain severity gets no note")

	// a run without a breakdown shows the prices and the golden's entry counts only
	pricesOnly := buildSeverityBreakdown(golden, &evals.GoldenRun{}, "R1", scoring)
	require.NotNil(t, pricesOnly)
	assert.False(t, pricesOnly.HasRun)
	assert.Equal(t, 2, pricesOnly.Rows[0].Entries, "from the golden file")
	assert.Empty(t, pricesOnly.ScoreLine)

	// no savable minutes: NIT escapes at ×1
	nitScoring := &evals.Scoring{Minutes: map[string]float64{"NIT": 10}, EscapeMultiplier: map[string]float64{"NIT": 1}}
	nitRun := &evals.GoldenRun{Severities: []evals.GoldenSeverity{{Severity: "NIT", Entries: 1, Missed: 1, EscapedMinutes: 10}}}
	undefined := buildSeverityBreakdown(golden, nitRun, "R1", nitScoring)
	assert.Equal(t, "review score undefined: null 10 − oracle 10 leaves no savable minutes", undefined.ScoreLine)

	assert.Nil(t, buildSeverityBreakdown(golden, goldenRun, "R1", nil))
	assert.Nil(t, buildSeverityBreakdown(golden, goldenRun, "R1", &evals.Scoring{}), "no price list")
}

func TestFeaturedBreakdown(t *testing.T) {
	definition := &evals.Definition{
		Goldens: []evals.DefinitionGolden{{Id: "g1", SeverityCounts: map[string]int{"NIT": 1}}},
		Scoring: &evals.Scoring{Minutes: map[string]float64{"NIT": 10}, EscapeMultiplier: map[string]float64{"NIT": 1}},
	}

	breakdown := featuredBreakdown(definition, &evals.Golden{GoldenId: "g1"}, nil, "")
	require.NotNil(t, breakdown)
	assert.False(t, breakdown.HasRun)
	assert.Equal(t, "10", breakdown.Totals.OracleMinutes)

	assert.Nil(t, featuredBreakdown(definition, &evals.Golden{GoldenId: "other"}, nil, ""), "unknown golden id")
	assert.Nil(t, featuredBreakdown(definition, nil, nil, ""), "run without a golden")
	assert.Nil(t, featuredBreakdown(nil, &evals.Golden{GoldenId: "g1"}, nil, ""), "no definition")
}

func TestHeadlineGoldenRun(t *testing.T) {
	view := &evalView{
		Dir: evals.EvalDir{Eval: evals.EvalFile{Golden: &evals.Golden{PerRun: []evals.GoldenRun{{RunId: "a"}, {RunId: "b"}}}}},
		Runs: []evalRunLabel{
			{Label: "R1", Run: evals.Run{Id: "a", Variant: evals.Variant{Name: "opus-high"}}},
			{Label: "R2", Run: evals.Run{Id: "b", Variant: evals.Variant{Name: "sonnet-high"}}},
		},
		Verdict: &evalVerdictView{BestArm: "sonnet-high"},
	}

	goldenRun, label := headlineGoldenRun(view)
	require.NotNil(t, goldenRun)
	assert.Equal(t, "b", goldenRun.RunId, "the verdict's best arm")
	assert.Equal(t, "R2", label)

	view.FocusRunId = "a"
	goldenRun, label = headlineGoldenRun(view)
	assert.Equal(t, "a", goldenRun.RunId, "the focused run wins")
	assert.Equal(t, "R1", label)

	view.FocusRunId = "missing"
	goldenRun, label = headlineGoldenRun(view)
	assert.Nil(t, goldenRun, "a focus id without a golden row")
	assert.Empty(t, label)

	goldenRun, _ = headlineGoldenRun(&evalView{})
	assert.Nil(t, goldenRun, "no golden block")
}

func TestBuildConfigView(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *evalConfigView

		installedVersion string
		view             *evalView
	}

	desktopSetup := &evals.Setup{Run: &evals.SetupRun{Desktop: &evals.SetupDesktop{Wraps: "workflows/railroad-review.js"}}}
	notes := []string{"autofix-level minor · lanes 1[sonnet:medium] · rounds 2", "start commit 1f725ced"}
	tests := make([]*testCase, 0)

	// recorded-versions-distinct-in-run-order
	tests = append(tests, &testCase{
		_id: "recorded-versions-distinct-in-run-order",
		_expected: &evalConfigView{
			Environment:  envCli,
			Notes:        []string{"autofix-level minor", "lanes 1[sonnet:medium]", "rounds 2", "start commit 1f725ced"},
			Version:      "2.1.282, 2.1.290",
			VersionTitle: versionTitleKnown,
		},

		installedVersion: "2.1.300",
		view: &evalView{
			Dir: evals.EvalDir{Eval: evals.EvalFile{Runs: []evals.Run{
				{Id: "r1", ClaudeCodeVersion: "2.1.282"},
				{Id: "r2", ClaudeCodeVersion: "2.1.290"},
				{Id: "r3", ClaudeCodeVersion: "2.1.282"},
			}}},
			NotesBullets: notes,
		},
	})

	// installed-version-stands-in
	tests = append(tests, &testCase{
		_id: "installed-version-stands-in",
		_expected: &evalConfigView{
			Environment:        envDesktop,
			Version:            "2.1.300",
			VersionIsInstalled: true,
			VersionTitle:       versionTitleInstall,
		},

		installedVersion: "2.1.300",
		view:             &evalView{Dir: evals.EvalDir{Eval: evals.EvalFile{Runs: []evals.Run{{Id: "r1"}}, Setup: desktopSetup}}},
	})

	// no-version-anywhere
	tests = append(tests, &testCase{
		_id: "no-version-anywhere",
		_expected: &evalConfigView{
			Environment:  envCli,
			Notes:        []string{"arms: opus-high"},
			Version:      versionUnknown,
			VersionTitle: versionTitleUnknown,
		},

		installedVersion: "",
		view:             &evalView{Dir: evals.EvalDir{Eval: evals.EvalFile{Runs: []evals.Run{{Id: "r1"}}}}, NotesBullets: []string{"arms: opus-high"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, buildConfigView(test.view, test.installedVersion))
		})
	}
}

func TestFocusCombinedView(t *testing.T) {
	runtimeS := 3600.0
	runs := []evals.Run{
		{Id: "r1", Variant: evals.Variant{Name: "sonnet-5-high"}},
		{Id: "r2", Variant: evals.Variant{Name: "opus-high"}},
	}
	values := []evals.MetricValue{
		{MetricId: metricReviewScore, RunId: "r1", Value: 0.02},
		{MetricId: metricReviewScore, RunId: "r2", Value: 0.04},
		{MetricId: metricWallSeconds, RunId: "r1", Value: 60.0},
		{MetricId: metricWallSeconds, RunId: "r2", Value: 120.0},
	}
	golden := &evals.Golden{
		Total: 100,
		PerRun: []evals.GoldenRun{
			{RunId: "r1", Matched: make([]evals.GoldenMatch, 3), FalsePositives: []evals.GoldenFalsePositive{{FindingId: "f1"}}},
			{RunId: "r2", Matched: make([]evals.GoldenMatch, 2), Uncataloged: []evals.GoldenFalsePositive{{FindingId: "u1"}}},
		},
	}
	evalDir := makeArmTilesDir(runs, values, golden)
	evalDir.Eval.Setup = &evals.Setup{RuntimeS: &runtimeS}
	evalDir.ArmRuns = []evals.ArmRun{
		{Id: "r1", Arm: "sonnet-5-high", Judge: evals.ArmRunJudge{Matched: []evals.ArmRunJudgeMatch{{GoldenId: "g1"}, {GoldenId: "g2"}}}},
		{Id: "r2", Arm: "opus-high", Judge: evals.ArmRunJudge{Matched: []evals.ArmRunJudgeMatch{{GoldenId: "g3"}}}},
	}
	verdict := &evalVerdictView{
		Runs:   []evalRunVerdictRow{{RunLabel: "R1"}, {RunLabel: "R2", Pass: true}},
		Status: "mixed",
	}
	labels := map[string]string{"r1": "R1", "r2": "R2"}
	rows := buildResultRows(evalDir, nil, labels)
	rows[1].Pass = true

	focused := focusCombinedView(evalDir, verdict, rows, "r2")
	require.NotNil(t, focused)
	assert.Equal(t, "R2 — PASS — arm opus-high", focused.PassLine)
	assert.False(t, focused.IsFail)
	assert.Equal(t, 1, focused.Runs)
	assert.Equal(t, "4%", focused.ReviewScore, "the run's own value, not the mean")
	assert.Equal(t, 1, focused.NewFinds)
	assert.Zero(t, focused.FalsePositives)
	assert.Equal(t, "1/100", focused.FoundLabel, "bar from the run's own armruns rows only")
	assert.Equal(t, "2.0 min", focused.RunWall, "run duration, not the manifest runtimeS")
	assert.Equal(t, "wall ≥ (longest arm)", focused.RunWallLabel)

	assert.Nil(t, focusCombinedView(evalDir, verdict, rows, "unknown"), "unknown focus id leaves the combined view")

	failFocused := focusCombinedView(evalDir, verdict, rows, "r1")
	require.NotNil(t, failFocused)
	assert.Equal(t, "R1 — FAIL — arm sonnet-5-high", failFocused.PassLine)
	assert.True(t, failFocused.IsFail)
}

func TestCombinedResultRow(t *testing.T) {
	combined := &evalCombinedView{
		CostTotal:        "$10.00",
		FalsePositives:   4,
		FoundCount:       28,
		GoldenTotal:      218,
		LostMinutes:      "912 min",
		NewFinds:         7,
		ReviewScore:      "61%",
		ReviewScoreClass: "tile-pass",
		Runs:             32,
		RunWall:          "52.7 min",
		TokensTotal:      "1 767 121",
	}
	rows := []resultRow{{Misclassified: 2}, {Misclassified: 1, Invalid: true}}

	row := combinedResultRow(combined, rows)
	assert.True(t, row.IsCombined)
	assert.Equal(t, "Σ combined", row.Label)
	assert.Equal(t, "32 runs", row.Arm)
	assert.Equal(t, 28, row.Found)
	assert.Equal(t, 218, row.Total)
	assert.Equal(t, 190, row.Missed)
	assert.InDelta(t, 87.2, row.MissedPct, 0.1)
	assert.Equal(t, 4, row.FalsePositives)
	assert.Equal(t, 7, row.NewFinds)
	assert.Equal(t, 2, row.Misclassified, "invalid rows excluded")
	assert.Equal(t, "61%", row.ReviewScore)
	assert.Equal(t, "tile-pass", row.ReviewScoreClass)
	assert.Equal(t, "912 min", row.LostMinutes)
	assert.Equal(t, "52.7 min", row.WallTime)
	assert.Equal(t, "—", row.ActiveTime)
}

func TestInterpretationBullets(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		markdown string
	}

	long := strings.Repeat("y", 170)
	clampedLong := strings.Repeat("y", 159) + "…"

	tests := make([]*testCase, 0)

	// heading-bullets-paragraph
	tests = append(tests, &testCase{
		_id:       "heading-bullets-paragraph",
		_expected: []string{"breadth is fine", "depth is the gap"},

		markdown: "# Interpretation\n\n- breadth is fine\n- depth is the gap\n\nWorker pins: none.\n",
	})

	// star-bullets
	tests = append(tests, &testCase{
		_id:       "star-bullets",
		_expected: []string{"one", "two"},

		markdown: "* one\n* two\n",
	})

	// nine-bullets-eight-kept
	tests = append(tests, &testCase{
		_id:       "nine-bullets-eight-kept",
		_expected: []string{"a", "b", "c", "d", "e", "f", "g", "h"},

		markdown: "- a\n- b\n- c\n- d\n- e\n- f\n- g\n- h\n- i\n",
	})

	// long-bullet-clamped
	tests = append(tests, &testCase{
		_id:       "long-bullet-clamped",
		_expected: []string{clampedLong},

		markdown: "- " + long + "\n",
	})

	// nested-bullet-dropped
	tests = append(tests, &testCase{
		_id:       "nested-bullet-dropped",
		_expected: []string{"top"},

		markdown: "- top\n  - nested\n",
	})

	// empty-file
	tests = append(tests, &testCase{
		_id:       "empty-file",
		_expected: nil,

		markdown: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, interpretationBullets(test.markdown))
		})
	}
}

func TestClampBullets(t *testing.T) {
	long := strings.Repeat("x", 130)
	clamped := clampBullets([]string{"a", "b", "c", "d", "e", "f"}, notesBulletChars, notesBulletLimit)
	assert.Len(t, clamped, 5, "sixth bullet dropped")

	clamped = clampBullets([]string{long}, notesBulletChars, notesBulletLimit)
	assert.Len(t, []rune(clamped[0]), 120)
	assert.True(t, strings.HasSuffix(clamped[0], "…"))

	assert.Nil(t, clampBullets(nil, notesBulletChars, notesBulletLimit))
}

func TestBuildArtifactRows(t *testing.T) {
	rows := buildArtifactRows([]string{"eval.json", "runs/run-1.md"})
	require.Len(t, rows, 2)
	assert.Equal(t, artifactRow{File: "eval.json", What: "the folded, schema-validated eval record the page reads"}, rows[0])
	assert.Equal(t, artifactRow{File: "runs/run-1.md", What: artifactMissing}, rows[1], "a file the catalog lacks names itself")

	assert.Nil(t, buildArtifactRows(nil))
}

func TestRunSelectorOptions(t *testing.T) {
	testView := evalView{Dir: evals.EvalDir{Dir: "demo/2026-10-06-abc123", Eval: evals.EvalFile{Eval: evals.Eval{Date: "2026-10-06"}}}, Verdict: &evalVerdictView{Status: "fail"}}
	produceView := evalView{Dir: evals.EvalDir{Dir: "demo/2026-10-05-abc123", Eval: evals.EvalFile{Eval: evals.Eval{Date: "2026-10-05"}}}, Verdict: &evalVerdictView{Status: "pass"}}
	views := []evalView{testView, produceView}

	groups := runSelectorOptions(views, "demo/2026-10-05-abc123", map[string]bool{"demo/2026-10-05-abc123": true})
	require.Len(t, groups, 2)
	assert.Equal(t, "eval runs", groups[0].Label)
	require.Len(t, groups[0].Options, 1)
	assert.Equal(t, "demo/2026-10-06-abc123", groups[0].Options[0].Dir)
	assert.Equal(t, "golden production", groups[1].Label)
	require.Len(t, groups[1].Options, 1)
	assert.True(t, groups[1].Options[0].Selected, "a featured production run stays selected")

	groups = runSelectorOptions(views, "demo/2026-10-06-abc123", map[string]bool{})
	require.Len(t, groups, 1, "no production run — one group")
	assert.Len(t, groups[0].Options, 2)

	groups = runSelectorOptions([]evalView{produceView}, "", map[string]bool{"demo/2026-10-05-abc123": true})
	require.Len(t, groups, 1, "only production runs — no empty eval group")
	assert.Equal(t, "golden production", groups[0].Label)
}

func TestBuildGoldenCards(t *testing.T) {
	cost := 302.01
	production := &evals.GoldenProduction{
		Contributions: []evals.GoldenContribution{
			{Version: 1, Findings: 151, NewFindings: 151, Dir: "demo/2026-10-04-abc123", Source: "unseeded", CostUsd: &cost},
			{Version: 1, Findings: 210, NewFindings: 59, Dir: "demo/2026-10-05-abc123", Source: "seeded"},
		},
	}
	definition := &evals.Definition{
		Goldens: []evals.DefinitionGolden{
			{Id: "g2", GoldenVersion: 0, FindingCount: 210, OracleMinutes: 8540, Production: production},
			{Id: "g3", GoldenVersion: 4, FindingCount: 3},
		},
	}
	loaded := evalView{
		Dir:     evals.EvalDir{Dir: "demo/2026-10-04-abc123"},
		Runs:    []evalRunLabel{{Label: "R1", Run: evals.Run{Id: "a"}}},
		Verdict: &evalVerdictView{Status: "pass"},
	}

	cards := buildGoldenCards(definition, []evalView{loaded})
	require.Len(t, cards, 2)
	assert.Equal(t, 1, cards[0].Version, "version from the contributions when the definition carries none")
	assert.Equal(t, "g2", cards[0].Def.Id)
	assert.Contains(t, cards[0].ProductionLine, "built from 2 contributions")
	require.Len(t, cards[0].Contributions, 2)
	assert.True(t, cards[0].Contributions[0].IsLoaded)
	assert.Equal(t, "PASS", cards[0].Contributions[0].Status)
	assert.Equal(t, "badge-ok", cards[0].Contributions[0].StatusClass)
	assert.False(t, cards[0].Contributions[1].IsLoaded, "an unloaded dir renders as text")
	assert.Empty(t, cards[0].Contributions[1].Status)
	assert.Equal(t, 4, cards[1].Version, "version from the definition")
	assert.Empty(t, cards[1].Contributions)

	assert.Nil(t, buildGoldenCards(nil, nil))
	assert.Equal(t, map[string]bool{"demo/2026-10-04-abc123": true, "demo/2026-10-05-abc123": true}, productionDirs(definition))
	assert.Empty(t, productionDirs(nil))
}

func TestHistoryGoldenLabel(t *testing.T) {
	assert.Equal(t, "v5 · 245", historyGoldenLabel(&evals.Golden{GoldenVersion: 5, Total: 245}))
	assert.Equal(t, "— · 136", historyGoldenLabel(&evals.Golden{Total: 136}), "legacy file without goldenVersion")
	assert.Equal(t, "—", historyGoldenLabel(nil))
}

func TestBuildGoldenContributionRows(t *testing.T) {
	cost := 324.03
	wall := int64(13601)
	output := int64(415507)
	subagent := int64(59303366)
	production := &evals.GoldenProduction{
		Contributions: []evals.GoldenContribution{
			{Version: 1, Findings: 136, NewFindings: 136, Source: "workflow session", SubagentTokens: &subagent},
			{Version: 2, Findings: 168, NewFindings: 32, Source: "evals/railroad-review/2026-09-05-a34281-094349", CostUsd: &cost, WallS: &wall, OutputTokens: &output},
		},
		Totals: evals.GoldenContribution{CostUsd: &cost, WallS: &wall, OutputTokens: &output, SubagentTokens: &subagent},
	}

	statusByDir := map[string]historyStatusView{"railroad-review/2026-09-05-a34281-094349": {Class: "badge-ok", Status: "PASS"}}
	production.Contributions[1].Dir = "railroad-review/2026-09-05-a34281-094349"

	rows := buildGoldenContributionRows(production, statusByDir)
	require.Len(t, rows, 2)
	assert.Equal(t, "v1 (136)", rows[0].VersionLabel)
	assert.Equal(t, "—", rows[0].Cost, "v1 session cost unknown")
	assert.Equal(t, "59 303 366 (subagent)", rows[0].Tokens)
	assert.False(t, rows[0].IsLoaded, "no dir — no link")
	assert.Equal(t, "$324.03", rows[1].Cost)
	assert.Equal(t, "226.7 min", rows[1].Wall)
	assert.Equal(t, "415 507", rows[1].Tokens)
	assert.True(t, rows[1].IsLoaded)
	assert.Equal(t, "PASS", rows[1].Status)

	line := goldenProductionLine(evals.DefinitionGolden{Production: production})
	assert.Contains(t, line, "built from 2 contributions")
	assert.Contains(t, line, "$324.03 + the v1 workflow session")
	assert.Contains(t, line, "subagent tok (v1)")

	assert.Nil(t, buildGoldenContributionRows(nil, statusByDir))
	assert.Empty(t, goldenProductionLine(evals.DefinitionGolden{}))
}

func TestBuildHistoryRows(t *testing.T) {
	passView := evalView{
		Dir:     evals.EvalDir{Dir: "railroad-review/2026-09-06-pass", Eval: evals.EvalFile{Eval: evals.Eval{Date: "2026-09-06"}}},
		Runs:    []evalRunLabel{{Label: "R1", Run: evals.Run{Id: "a", Variant: evals.Variant{Name: "sonnet-5-high"}}}},
		Verdict: &evalVerdictView{Status: "pass", BestArm: "sonnet-5-high"},
	}
	failView := evalView{
		Dir:     evals.EvalDir{Dir: "railroad-review/2026-09-05-fail", Eval: evals.EvalFile{Eval: evals.Eval{Date: "2026-09-05"}}},
		Runs:    []evalRunLabel{{Label: "R1", Run: evals.Run{Id: "a", Variant: evals.Variant{Name: "sonnet-5-high"}}}},
		Verdict: &evalVerdictView{Status: "fail", IsFail: true},
	}
	invalidView := evalView{
		Dir:     evals.EvalDir{Dir: "railroad-review/2026-09-04-invalid", Eval: evals.EvalFile{Eval: evals.Eval{Date: "2026-09-04"}}},
		Runs:    []evalRunLabel{{Label: "R1", Run: evals.Run{Id: "a", Invalid: true, InvalidReason: "dispatcher-failed"}}},
		Verdict: &evalVerdictView{Status: "fail", IsFail: true},
	}

	produceView := evalView{
		Dir:     evals.EvalDir{Dir: "railroad-review/2026-09-03-produce", Eval: evals.EvalFile{Eval: evals.Eval{Date: "2026-09-03"}}},
		Verdict: &evalVerdictView{Status: "pass"},
	}
	productionDirs := map[string]bool{"railroad-review/2026-09-03-produce": true}

	rows := buildHistoryRows([]evalView{passView, failView, invalidView, produceView}, "railroad-review/2026-09-06-pass", productionDirs)
	require.Len(t, rows, 3, "the production run leaves the history")

	assert.True(t, rows[0].IsFeatured)
	assert.Equal(t, "PASS", rows[0].Status)
	assert.Equal(t, "sonnet-5-high", rows[0].BestArm)
	assert.Equal(t, "badge-ok", rows[0].StatusClass)

	assert.False(t, rows[1].IsFeatured)
	assert.Equal(t, "FAIL", rows[1].Status)
	assert.Equal(t, "badge-error", rows[1].StatusClass)

	assert.Equal(t, "ERROR", rows[2].Status)
	assert.Equal(t, "dispatcher-failed", rows[2].StatusDetail)
	assert.Equal(t, "badge-dim", rows[2].StatusClass)
}

func TestSkillTestsSmokeCard(t *testing.T) {
	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	smokeDir := filepath.Join(dir, "smoke")
	writeRepoSkill(t, skillsRepo, "smoked", "1.0")
	writeRepoSkill(t, skillsRepo, "plain", "1.0")
	coverage := skilleval.SmokeCoverage{
		AnchorsDeclared:    3,
		AnchorsExpected:    []string{"base", "round"},
		AnchorsExpectedHit: 1,
		AnchorsHit:         1,
		ChecksDeclared:     4,
		ChecksPassed:       3,
	}
	record := &skilleval.SmokeRecord{
		Manifest: &skilleval.RunManifest{TokenLabel: "label"},
		Result: &skilleval.ArmResult{
			EndedAt:      time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC),
			SkillVersion: "1.0",
			Smoke:        &skilleval.SmokeResult{Coverage: coverage, Verdict: statusPass},
		},
		Runtime: &skilleval.ArmRuntime{Runs: []heartbeat.Run{{Anchors: []heartbeat.AnchorHit{{Id: "base"}}}}},
	}
	writeSmokeRecord(t, smokeDir, "smoked", "1.0", record)
	server := newTestServer(t, &Options{
		EvalsDir:   filepath.Join(dir, "evals"),
		SkillsHome: filepath.Join(dir, "home"),
		SkillsRepo: skillsRepo,
		SmokeDir:   smokeDir,
	})

	t.Run("card-rendered", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/smoked/tests", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `class="tool-chip smoke-hit" title="hit">base</span>`)
		assert.Contains(t, body, `class="tool-chip smoke-missed" title="never hit">round</span>`)
		assert.Contains(t, body, filepath.Join(smokeDir, "smoked", "1.0"))
		assert.Contains(t, body, "make skill-smoke SKILL=smoked TOKEN=label")
	})

	t.Run("no-record", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/plain/tests", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "smoke-card")
	})
}
