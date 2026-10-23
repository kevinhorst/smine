package skilleval

import (
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/reviewscope"
)

// fourSeverityGolden is one golden entry per human-side severity plus a NIT:
// oracle 250, null 940, savable 690 under testScoring.
const fourSeverityGolden = `{"goldenVersion":1,"findings":[
	{"golden_id":"G1","file":"a.go","severity":"BLOCKER"},
	{"golden_id":"G2","file":"b.go","severity":"MAJOR"},
	{"golden_id":"G3","file":"c.go","severity":"MINOR"},
	{"golden_id":"G4","file":"d.go","severity":"NIT"}
]}`

// nitOnlyGolden has no savable minutes: NIT escapes at ×1.
const nitOnlyGolden = `{"goldenVersion":1,"findings":[
	{"golden_id":"N1","file":"a.go","severity":"NIT"},
	{"golden_id":"N2","file":"b.go","severity":"NIT"}
]}`

// scenarioCounts is the plan's realistic golden (Hot items scenario table):
// 2 BLOCKER, 3 MAJOR, 5 MINOR, 10 NIT, 3 INFO — oracle 766, null 2416.
var scenarioCounts = map[string]int{"BLOCKER": 2, "MAJOR": 3, "MINOR": 5, "NIT": 10, "INFO": 3}

var scenarioSeverities = []string{"BLOCKER", "MAJOR", "MINOR", "NIT", "INFO"}

// goldenMetricSummary is the comparable slice of one scored run, floats
// rounded to six places.
type goldenMetricSummary struct {
	escapedMinutes       float64
	falsePositiveMinutes float64
	hasScore             bool
	isError              bool
	lostMinutes          float64
	reviewScore          float64
}

// scenarioSpec shapes one scenario run over the realistic golden.
type scenarioSpec struct {
	FixState reviewscope.FixState
	Found    map[string]bool   // severities found; nil = all
	Reported map[string]string // true severity → reported severity
	Skipped  string            // one golden id left unmatched
	Unfixed  map[string]bool   // severities whose fix stays applied, the fix check unconfirmed
}

// scorerTotals is the comparable slice of a constructed scorer.
type scorerTotals struct {
	isError       bool
	nullMinutes   float64
	oracleMinutes float64
}

func TestGoldenScorer_run(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *evals.GoldenRun

		verdict *judgeVerdict
	}

	scorer := &goldenScorer{
		golden: &goldenDoc{Findings: []goldenFinding{
			{File: "a.go", GoldenId: "G1", Severity: "MAJOR"},
			{File: "b.go", GoldenId: "G2", Severity: "NIT"},
		}},
	}
	tests := make([]*testCase, 0)

	// duplicate-match-first-wins
	tests = append(tests, &testCase{
		_id: "duplicate-match-first-wins",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{},
			FixEscapes:     []string{},
			Matched:        []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1", Note: "first"}},
			Misclassified:  []evals.GoldenMisclassified{},
			Missed:         []string{"G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{},
		},

		verdict: &judgeVerdict{Matched: []evals.GoldenMatch{
			{FindingId: "F1", GoldenId: "G1", Note: "first"},
			{FindingId: "F2", GoldenId: "G1", Note: "second"},
		}},
	})

	// missed-derived
	tests = append(tests, &testCase{
		_id: "missed-derived",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{},
			FixEscapes:     []string{},
			Matched:        []evals.GoldenMatch{},
			Misclassified:  []evals.GoldenMisclassified{},
			Missed:         []string{"G1", "G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{},
		},

		verdict: &judgeVerdict{},
	})

	// misclassified-unmatched-dropped
	tests = append(tests, &testCase{
		_id: "misclassified-unmatched-dropped",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{},
			FixEscapes:     []string{},
			Matched:        []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
			Misclassified:  []evals.GoldenMisclassified{{Actual: "NIT", Expected: "MAJOR", FindingId: "F1", GoldenId: "G1"}},
			Missed:         []string{"G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{},
		},

		verdict: &judgeVerdict{
			Matched: []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
			Misclassified: []evals.GoldenMisclassified{
				{Actual: "NIT", Expected: "MAJOR", FindingId: "F1", GoldenId: "G1"},
				{Actual: "MAJOR", Expected: "NIT", FindingId: "F9", GoldenId: "G2"},
			},
		},
	})

	// ruling-naming-matched-finding-dropped
	tests = append(tests, &testCase{
		_id: "ruling-naming-matched-finding-dropped",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{{FindingId: "F3", Reason: "claim is factually wrong"}},
			FixEscapes:     []string{},
			Matched:        []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
			Misclassified:  []evals.GoldenMisclassified{},
			Missed:         []string{"G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{},
		},

		verdict: &judgeVerdict{
			FalsePositives: []judgeRuling{
				{FindingId: "F1", Reason: "judge contradicted itself"},
				{FindingId: "F3", Reason: "claim is factually wrong"},
				{FindingId: "F3", Reason: "duplicate ruling"},
			},
			Matched: []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
		},
	})

	// uncataloged-note-becomes-reason
	tests = append(tests, &testCase{
		_id: "uncataloged-note-becomes-reason",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{},
			FixEscapes:     []string{},
			Matched:        []evals.GoldenMatch{},
			Misclassified:  []evals.GoldenMisclassified{},
			Missed:         []string{"G1", "G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{{FindingId: "F4", Reason: "genuine in-range defect"}},
		},

		verdict: &judgeVerdict{Uncataloged: []judgeRuling{{FindingId: "F4", Note: "genuine in-range defect"}}},
	})

	// fix-escape-unmatched-dropped
	tests = append(tests, &testCase{
		_id: "fix-escape-unmatched-dropped",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{},
			FixEscapes:     []string{"G1"},
			Matched:        []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
			Misclassified:  []evals.GoldenMisclassified{},
			Missed:         []string{"G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{},
		},

		verdict: &judgeVerdict{
			FixEscapes: []string{"G1", "G2"},
			Matched:    []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
		},
	})

	// fix-escape-duplicate-kept-once
	tests = append(tests, &testCase{
		_id: "fix-escape-duplicate-kept-once",
		_expected: &evals.GoldenRun{
			FalsePositives: []evals.GoldenFalsePositive{},
			FixEscapes:     []string{"G1"},
			Matched:        []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
			Misclassified:  []evals.GoldenMisclassified{},
			Missed:         []string{"G2"},
			RunId:          "run-1",
			Uncataloged:    []evals.GoldenFalsePositive{},
		},

		verdict: &judgeVerdict{
			FixEscapes: []string{"G1", "G1"},
			Matched:    []evals.GoldenMatch{{FindingId: "F1", GoldenId: "G1"}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, scorer.run("run-1", test.verdict))
		})
	}
}

func TestGoldenScorer_metricValues(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *goldenMetricSummary

		candidates map[string]candidate
		goldenRun  *evals.GoldenRun
		isValid    bool
		scorer     *goldenScorer
	}

	scorer := newTestScorer(t, fourSeverityGolden)
	nitScorer := newTestScorer(t, nitOnlyGolden)
	scenario := newTestScorer(t, scenarioGolden())
	savable := 690.0
	scenarioSavable := 1650.0
	identity := map[string]string{"BLOCKER": "BLOCKER", "MAJOR": "MAJOR", "MINOR": "MINOR", "NIT": "NIT", "INFO": "INFO"}
	overRated := map[string]string{"BLOCKER": "BLOCKER", "MAJOR": "BLOCKER", "MINOR": "MAJOR", "NIT": "MINOR", "INFO": "NIT"}
	underRated := map[string]string{"BLOCKER": "MAJOR", "MAJOR": "MINOR", "MINOR": "NIT", "NIT": "INFO", "INFO": "INFO"}
	tests := make([]*testCase, 0)

	// human-fix-right-severity
	tests = append(tests, &testCase{
		_id:       "human-fix-right-severity",
		_expected: minuteSummary(0, 0, 0, 1),

		candidates: rightCandidates(),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// same-side-over-rated-gap
	tests = append(tests, &testCase{
		_id:       "same-side-over-rated-gap",
		_expected: minuteSummary(0, 0, 30, 1-30/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F2", FixState: reviewscope.FixStateOpen, Severity: "BLOCKER"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// same-side-under-rated-gap
	tests = append(tests, &testCase{
		_id:       "same-side-under-rated-gap",
		_expected: minuteSummary(0, 0, 90, 1-90/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: reviewscope.FixStateOpen, Severity: "MINOR"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// nit-over-rated-across-line-gap
	tests = append(tests, &testCase{
		_id:       "nit-over-rated-across-line-gap",
		_expected: minuteSummary(0, 0, 80, 1-80/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F4", FixState: reviewscope.FixStateOpen, Severity: "MAJOR"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// open-below-triage-escapes
	tests = append(tests, &testCase{
		_id:       "open-below-triage-escapes",
		_expected: minuteSummary(60, 0, 30, 1-30/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F3", FixState: reviewscope.FixStateOpen, Severity: "nit"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// fixed-priced-by-factor
	tests = append(tests, &testCase{
		_id:       "fixed-priced-by-factor",
		_expected: minuteSummary(0, 0, -60, 1+60/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: reviewscope.FixStateFixed, Severity: "BLOCKER"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// applied-priced-factor-plus-minutes
	tests = append(tests, &testCase{
		_id:       "applied-priced-factor-plus-minutes",
		_expected: minuteSummary(0, 0, 60, 1-60/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: reviewscope.FixStateApplied, Severity: "BLOCKER"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// open-human-price
	tests = append(tests, &testCase{
		_id:       "open-human-price",
		_expected: minuteSummary(0, 0, 0, 1),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: reviewscope.FixStateOpen, Severity: "BLOCKER"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// fix-state-absent-reads-open
	tests = append(tests, &testCase{
		_id:       "fix-state-absent-reads-open",
		_expected: minuteSummary(0, 0, 0, 1),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: "", Severity: "BLOCKER"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// fix-escape-prices-escape
	tests = append(tests, &testCase{
		_id:       "fix-escape-prices-escape",
		_expected: minuteSummary(600, 0, 480, 1-480/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: reviewscope.FixStateFixed, Severity: "BLOCKER"}),
		goldenRun:  allMatchedRun([]string{"G1"}),
		isValid:    true,
		scorer:     scorer,
	})

	// false-positive-open
	tests = append(tests, &testCase{
		_id:       "false-positive-open",
		_expected: minuteSummary(0, 90, 90, 1-90/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F9", FixState: reviewscope.FixStateOpen, Severity: "MAJOR"}),
		goldenRun:  allMatchedRun(nil, "F9"),
		isValid:    true,
		scorer:     scorer,
	})

	// false-positive-applied
	tests = append(tests, &testCase{
		_id:       "false-positive-applied",
		_expected: minuteSummary(0, 126, 126, 1-126/savable),

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F9", FixState: reviewscope.FixStateApplied, Severity: "MAJOR"}),
		goldenRun:  allMatchedRun(nil, "F9"),
		isValid:    true,
		scorer:     scorer,
	})

	// silent-run-scores-zero
	tests = append(tests, &testCase{
		_id:       "silent-run-scores-zero",
		_expected: minuteSummary(940, 0, 690, 0),

		candidates: map[string]candidate{},
		goldenRun:  &evals.GoldenRun{RunId: "run-1"},
		isValid:    true,
		scorer:     scorer,
	})

	// zero-savable-null
	tests = append(tests, &testCase{
		_id:       "zero-savable-null",
		_expected: &goldenMetricSummary{escapedMinutes: 20},

		candidates: map[string]candidate{},
		goldenRun:  &evals.GoldenRun{RunId: "run-1"},
		isValid:    true,
		scorer:     nitScorer,
	})

	// invalid-run-nulls
	tests = append(tests, &testCase{
		_id:       "invalid-run-nulls",
		_expected: &goldenMetricSummary{},

		candidates: rightCandidates(),
		goldenRun:  allMatchedRun(nil),
		isValid:    false,
		scorer:     scorer,
	})

	// unpriced-candidate-severity-error
	tests = append(tests, &testCase{
		_id:       "unpriced-candidate-severity-error",
		_expected: &goldenMetricSummary{isError: true},

		candidates: withCandidate(rightCandidates(), candidate{FindingId: "F2", FixState: reviewscope.FixStateOpen, Severity: "SEVERE"}),
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// judged-finding-not-candidate-error
	tests = append(tests, &testCase{
		_id:       "judged-finding-not-candidate-error",
		_expected: &goldenMetricSummary{isError: true},

		candidates: map[string]candidate{},
		goldenRun:  allMatchedRun(nil),
		isValid:    true,
		scorer:     scorer,
	})

	// scenario-a-blockers-and-majors-only
	runA, candidatesA := scenarioRun(scenarioSpec{Found: map[string]bool{"BLOCKER": true, "MAJOR": true}, Reported: identity, FixState: reviewscope.FixStateOpen})
	tests = append(tests, &testCase{
		_id:       "scenario-a-blockers-and-majors-only",
		_expected: minuteSummary(406, 0, 150, 1-150/scenarioSavable),

		candidates: candidatesA,
		goldenRun:  runA,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-b-perfect-human
	runB, candidatesB := scenarioRun(scenarioSpec{Reported: identity, FixState: reviewscope.FixStateOpen})
	tests = append(tests, &testCase{
		_id:       "scenario-b-perfect-human",
		_expected: minuteSummary(0, 0, 0, 1),

		candidates: candidatesB,
		goldenRun:  runB,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-c-noise-flood
	runC, candidatesC := scenarioRun(scenarioSpec{Found: map[string]bool{"BLOCKER": true, "MAJOR": true}, Reported: identity, FixState: reviewscope.FixStateOpen})
	addScenarioFalsePositives(runC, candidatesC, "NIT", 20)
	addScenarioFalsePositives(runC, candidatesC, "INFO", 10)
	tests = append(tests, &testCase{
		_id:       "scenario-c-noise-flood",
		_expected: minuteSummary(406, 220, 370, 1-370/scenarioSavable),

		candidates: candidatesC,
		goldenRun:  runC,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-d-over-rater
	runD, candidatesD := scenarioRun(scenarioSpec{Reported: overRated, FixState: reviewscope.FixStateOpen})
	tests = append(tests, &testCase{
		_id:       "scenario-d-over-rater",
		_expected: minuteSummary(0, 0, 614, 1-614/scenarioSavable),

		candidates: candidatesD,
		goldenRun:  runD,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-e-under-rater
	runE, candidatesE := scenarioRun(scenarioSpec{Reported: underRated, FixState: reviewscope.FixStateOpen})
	tests = append(tests, &testCase{
		_id:       "scenario-e-under-rater",
		_expected: minuteSummary(300, 0, 470, 1-470/scenarioSavable),

		candidates: candidatesE,
		goldenRun:  runE,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-f-one-blocker-missed
	runF, candidatesF := scenarioRun(scenarioSpec{Reported: identity, FixState: reviewscope.FixStateOpen, Skipped: "S-BLOCKER-1"})
	tests = append(tests, &testCase{
		_id:       "scenario-f-one-blocker-missed",
		_expected: minuteSummary(600, 0, 480, 1-480/scenarioSavable),

		candidates: candidatesF,
		goldenRun:  runF,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-g-all-auto-fixed-held
	runG, candidatesG := scenarioRun(scenarioSpec{FixState: reviewscope.FixStateFixed, Reported: identity})
	tests = append(tests, &testCase{
		_id:       "scenario-g-all-auto-fixed-held",
		_expected: minuteSummary(0, 0, -489.9, 1+489.9/scenarioSavable),

		candidates: candidatesG,
		goldenRun:  runG,
		isValid:    true,
		scorer:     scenario,
	})

	// scenario-h-blocker-fixes-caught
	runH, candidatesH := scenarioRun(scenarioSpec{FixState: reviewscope.FixStateFixed, Reported: identity, Unfixed: map[string]bool{"BLOCKER": true}})
	tests = append(tests, &testCase{
		_id:       "scenario-h-blocker-fixes-caught",
		_expected: minuteSummary(0, 0, -249.9, 1+249.9/scenarioSavable),

		candidates: candidatesH,
		goldenRun:  runH,
		isValid:    true,
		scorer:     scenario,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, summarizeGoldenMetrics(test.scorer.metricValues(test.goldenRun, test.candidates, test.isValid)))
		})
	}
}

func TestGoldenScorer_severities(t *testing.T) {
	scorer := newTestScorer(t, fourSeverityGolden)
	candidates := withCandidate(rightCandidates(), candidate{FindingId: "F1", FixState: reviewscope.FixStateFixed, Severity: "BLOCKER"})
	candidates = withCandidate(candidates, candidate{FindingId: "F4", FixState: reviewscope.FixStateOpen, Severity: "MAJOR"})
	candidates = withCandidate(candidates, candidate{FindingId: "F5", FixState: reviewscope.FixStateOpen, Severity: "NIT"})
	goldenRun := &evals.GoldenRun{
		FalsePositives: []evals.GoldenFalsePositive{{FindingId: "F5", Reason: "wrong"}},
		FixEscapes:     []string{"G3"},
		Matched: []evals.GoldenMatch{
			{FindingId: "F1", GoldenId: "G1"},
			{FindingId: "F3", GoldenId: "G3"},
			{FindingId: "F4", GoldenId: "G4"},
		},
		Missed: []string{"G2"},
		RunId:  "run-1",
	}

	_, err := scorer.metricValues(goldenRun, candidates, true)
	require.NoError(t, err)

	expected := []evals.GoldenSeverity{
		{Entries: 1, Found: 1, FoundMinutes: 60, Outcomes: evals.SeverityOutcomes{Fixed: evals.OutcomeTally{Count: 1, Minutes: 60}}, Severity: "BLOCKER"},
		{Entries: 1, EscapedMinutes: 270, Missed: 1, Outcomes: evals.SeverityOutcomes{Missed: evals.OutcomeTally{Count: 1, Minutes: 270}}, Severity: "MAJOR"},
		{Entries: 1, EscapedMinutes: 60, Found: 1, Outcomes: evals.SeverityOutcomes{FixFailed: evals.OutcomeTally{Count: 1, Minutes: 60}}, Severity: "MINOR"},
		{Entries: 1, FalsePositiveMinutes: 10, FalsePositives: 1, Found: 1, FoundMinutes: 90, Outcomes: evals.SeverityOutcomes{WrongSeverity: evals.OutcomeTally{Count: 1, Minutes: 90}}, Severity: "NIT"},
	}
	assert.Equal(t, expected, goldenRun.Severities, "priciest severity first; the fixed BLOCKER at its factor, the missed MAJOR at its multiplier, the MINOR whose fix failed at its multiplier, the NIT reported as MAJOR at price plus distance, the NIT false positive under its claimed severity")

	invalid := &evals.GoldenRun{RunId: "run-2"}
	_, err = scorer.metricValues(invalid, candidates, false)
	require.NoError(t, err)
	assert.Nil(t, invalid.Severities, "an invalid run carries no breakdown")
}

func TestNewGoldenScorer(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *scorerTotals

		goldenJson string
	}

	goldenDir := t.TempDir()
	tests := make([]*testCase, 0)

	// oracle-and-null-summed
	tests = append(tests, &testCase{
		_id:       "oracle-and-null-summed",
		_expected: &scorerTotals{nullMinutes: 940, oracleMinutes: 250},

		goldenJson: fourSeverityGolden,
	})

	// unpriced-golden-severity-error
	tests = append(tests, &testCase{
		_id:       "unpriced-golden-severity-error",
		_expected: &scorerTotals{isError: true},

		goldenJson: `{"goldenVersion":1,"findings":[{"golden_id":"G1","file":"a.go","severity":"UNRANKED"}]}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, totalsOf(newGoldenScorer(writeGolden(t, goldenDir, test._id, test.goldenJson), testScoring())))
		})
	}
}

// addScenarioFalsePositives appends count false positives of one severity,
// left open, to a scenario run and its candidates.
func addScenarioFalsePositives(goldenRun *evals.GoldenRun, candidates map[string]candidate, severity string, count int) {
	for n := 1; n <= count; n++ {
		findingId := fmt.Sprintf("FP-%s-%d", severity, n)
		goldenRun.FalsePositives = append(goldenRun.FalsePositives, evals.GoldenFalsePositive{FindingId: findingId})
		candidates[findingId] = candidate{FindingId: findingId, FixState: reviewscope.FixStateOpen, Severity: severity}
	}
}

// allMatchedRun matches G1..G4 with F1..F4, plus the given fix escapes and
// false-positive finding ids.
func allMatchedRun(fixEscapes []string, falsePositives ...string) *evals.GoldenRun {
	goldenRun := &evals.GoldenRun{
		FixEscapes: fixEscapes,
		Matched: []evals.GoldenMatch{
			{FindingId: "F1", GoldenId: "G1"},
			{FindingId: "F2", GoldenId: "G2"},
			{FindingId: "F3", GoldenId: "G3"},
			{FindingId: "F4", GoldenId: "G4"},
		},
		RunId: "run-1",
	}
	for _, findingId := range falsePositives {
		goldenRun.FalsePositives = append(goldenRun.FalsePositives, evals.GoldenFalsePositive{FindingId: findingId})
	}
	return goldenRun
}

// minuteSummary builds an expected summary of a run with a review score.
func minuteSummary(escapedMinutes, falsePositiveMinutes, lostMinutes, reviewScore float64) *goldenMetricSummary {
	summary := &goldenMetricSummary{
		escapedMinutes:       roundMetric(escapedMinutes),
		falsePositiveMinutes: roundMetric(falsePositiveMinutes),
		hasScore:             true,
		lostMinutes:          roundMetric(lostMinutes),
		reviewScore:          roundMetric(reviewScore),
	}
	return summary
}

func newTestScorer(t *testing.T, goldenJson string) *goldenScorer {
	t.Helper()
	path := writeGolden(t, t.TempDir(), "golden", goldenJson)
	scorer, err := newGoldenScorer(path, testScoring())
	require.NoError(t, err)
	return scorer
}

// rightCandidates are F1..F4 left open at their golden severities.
func rightCandidates() map[string]candidate {
	candidates := map[string]candidate{
		"F1": {FindingId: "F1", FixState: reviewscope.FixStateOpen, Severity: "BLOCKER"},
		"F2": {FindingId: "F2", FixState: reviewscope.FixStateOpen, Severity: "MAJOR"},
		"F3": {FindingId: "F3", FixState: reviewscope.FixStateOpen, Severity: "MINOR"},
		"F4": {FindingId: "F4", FixState: reviewscope.FixStateOpen, Severity: "NIT"},
	}
	return candidates
}

func roundMetric(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

// scenarioEntries lists the realistic golden's entries, ids S-<SEVERITY>-<n>.
func scenarioEntries() []goldenFinding {
	entries := []goldenFinding{}
	for _, severity := range scenarioSeverities {
		for n := 1; n <= scenarioCounts[severity]; n++ {
			entry := goldenFinding{File: "s.go", GoldenId: fmt.Sprintf("S-%s-%d", severity, n), Severity: severity}
			entries = append(entries, entry)
		}
	}
	return entries
}

func scenarioGolden() string {
	findings := []string{}
	for _, entry := range scenarioEntries() {
		findings = append(findings, fmt.Sprintf(`{"golden_id":%q,"file":%q,"severity":%q}`, entry.GoldenId, entry.File, entry.Severity))
	}
	return `{"goldenVersion":1,"findings":[` + strings.Join(findings, ",") + `]}`
}

// scenarioRun matches every realistic-golden entry the spec finds, reported
// and fix-stated as the spec says.
func scenarioRun(spec scenarioSpec) (*evals.GoldenRun, map[string]candidate) {
	goldenRun := &evals.GoldenRun{RunId: "run-1"}
	candidates := make(map[string]candidate)
	for _, entry := range scenarioEntries() {
		isFound := spec.Found == nil || spec.Found[entry.Severity]
		isMatched := isFound && entry.GoldenId != spec.Skipped
		if !isMatched {
			continue
		}

		findingId := "F-" + entry.GoldenId
		goldenRun.Matched = append(goldenRun.Matched, evals.GoldenMatch{FindingId: findingId, GoldenId: entry.GoldenId})
		fixState := spec.FixState
		if spec.Unfixed[entry.Severity] {
			fixState = reviewscope.FixStateApplied
		}

		candidates[findingId] = candidate{
			FindingId: findingId,
			FixState:  fixState,
			Severity:  spec.Reported[entry.Severity],
		}
	}
	return goldenRun, candidates
}

func summarizeGoldenMetrics(values []evals.MetricValue, err error) *goldenMetricSummary {
	if err != nil {
		return &goldenMetricSummary{isError: true}
	}

	reviewScore, hasScore := metricValueOf(values, metricReviewScore, "run-1").Number()
	lostMinutes, _ := metricValueOf(values, metricLostMinutes, "run-1").Number()
	escapedMinutes, _ := metricValueOf(values, metricEscapedMinutes, "run-1").Number()
	falsePositiveMinutes, _ := metricValueOf(values, metricFalsePositiveMinutes, "run-1").Number()
	summary := &goldenMetricSummary{
		escapedMinutes:       roundMetric(escapedMinutes),
		falsePositiveMinutes: roundMetric(falsePositiveMinutes),
		hasScore:             hasScore,
		lostMinutes:          roundMetric(lostMinutes),
		reviewScore:          roundMetric(reviewScore),
	}
	return summary
}

// testScoring is the railroad family's agreed price list (plan D3, D5, D12).
func testScoring() *evals.Scoring {
	scoring := &evals.Scoring{
		EscapeMultiplier: map[string]float64{"BLOCKER": 5, "MAJOR": 3, "MINOR": 2, "NIT": 1, "INFO": 1},
		FixFactor:        map[string]float64{"BLOCKER": 0.5, "MAJOR": 0.4, "MINOR": 0.25, "NIT": 0.1, "INFO": 0.1},
		Minutes:          map[string]float64{"BLOCKER": 120, "MAJOR": 90, "MINOR": 30, "NIT": 10, "INFO": 2},
		TriageLine:       "MINOR",
	}
	return scoring
}

func totalsOf(scorer *goldenScorer, err error) *scorerTotals {
	if err != nil {
		return &scorerTotals{isError: true}
	}

	totals := &scorerTotals{nullMinutes: scorer.nullMinutes, oracleMinutes: scorer.oracleMinutes}
	return totals
}

// withCandidate returns a copy of candidates with one entry set.
func withCandidate(candidates map[string]candidate, found candidate) map[string]candidate {
	copied := make(map[string]candidate, len(candidates)+1)
	maps.Copy(copied, candidates)
	copied[found.FindingId] = found
	return copied
}

func writeGolden(t *testing.T, dir, name, goldenJson string) string {
	t.Helper()
	path := filepath.Join(dir, name+".json")
	writeTestFile(t, path, goldenJson)
	return path
}
