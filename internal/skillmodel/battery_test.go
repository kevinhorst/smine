package skillmodel

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// batteryVocabulary declares the atoms the battery fixtures read: sourced
// inputs, granted events, an int budget and the sourced mode enum.
func batteryVocabulary() *Vocabulary {
	vocab := &Vocabulary{Sorts: map[string][]Atom{
		"count": {{Id: "budget", Kind: skillgraph.KindAtomInt}},
		"event": {{Id: "done"}, {Id: "nightly_done"}, {Id: "started"}},
		"input": {{Id: "approved", Source: "arg"}, {Id: "armed", Source: "arg"}, {Id: "never", Source: "arg"}},
		"mode":  {{Id: "mode", Kind: skillgraph.KindAtomEnum, Members: []string{"interactive", "nightly"}, Source: "session"}},
	}}
	return vocab
}

// batteryModel is a one-phase model-tier fixture over batteryVocabulary.
func batteryModel(invariants []Invariant, transitions []Transition) *Model {
	model := &Model{
		Invariants:  invariants,
		IsStrict:    true,
		Sections:    []Section{{Id: "run", Kind: skillgraph.SectionKindPhase, Title: "Run"}},
		Skill:       "demo",
		Transitions: transitions,
		Vocabulary:  batteryVocabulary(),
	}
	return model
}

// step builds one fixture step in the run section.
func step(id, pred string, post ...string) Transition {
	return Transition{Id: id, Kind: KindStep, Post: post, Pred: pred, Section: "run", Text: "t"}
}

// rule builds one fixture invariant in the run section.
func rule(id, expr string) Invariant {
	return Invariant{Id: id, Expr: expr, Section: "run", Text: "t"}
}

// runBattery runs the battery with the CLI's timeout, failing on an error.
func runBattery(t *testing.T, model *Model) []Finding {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	findings, err := RunBattery(ctx, model)
	require.NoError(t, err)
	return findings
}

// findingsOf filters findings by check and first entry id; an empty id
// matches every finding of the check.
func findingsOf(check, entryId string, findings []Finding) []Finding {
	var matched []Finding
	for _, finding := range findings {
		isEntry := entryId == "" || slices.Contains(finding.EntryIds, entryId)
		if finding.Check == check && isEntry {
			matched = append(matched, finding)
		}
	}
	return matched
}

func TestRunBattery_WithoutZ3(t *testing.T) {
	t.Setenv("PATH", "")

	t.Run("annotation-tier", func(t *testing.T) {
		model := batteryModel(nil, []Transition{step("SKILL-DEMO-PROC-001", "")})
		model.IsStrict = false
		findings, err := RunBattery(context.Background(), model)
		require.NoError(t, err)
		assert.Nil(t, findings)
	})

	t.Run("validate-cascade", func(t *testing.T) {
		transition := step("SKILL-DEMO-PROC-001", "")
		transition.Section = "warp"
		findings, err := RunBattery(context.Background(), batteryModel(nil, []Transition{transition}))
		require.NoError(t, err)
		require.Len(t, findings, 1)
		assert.Equal(t, CheckValidate, findings[0].Check)
		assert.Equal(t, SeverityError, findings[0].Severity)
		assert.Equal(t, []string{"SKILL-DEMO-PROC-001"}, findings[0].EntryIds)
	})

	t.Run("strict-no-z3", func(t *testing.T) {
		_, err := RunBattery(context.Background(), batteryModel(nil, []Transition{step("SKILL-DEMO-PROC-001", "")}))
		assert.ErrorContains(t, err, "z3 binary not found")
	})
}

func TestRunBattery(t *testing.T) {
	skipWithoutZ3(t)

	t.Run("inconsistent", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "budget >= 0"), rule("SKILL-DEMO-EXEC-002", "budget < 0")},
			[]Transition{step("SKILL-DEMO-PROC-001", "")},
		)
		findings := runBattery(t, model)
		require.Len(t, findings, 1)
		assert.Equal(t, CheckConsistency, findings[0].Check)
		assert.Equal(t, SeverityError, findings[0].Severity)
		assert.Equal(t, []string{"SKILL-DEMO-EXEC-001", "SKILL-DEMO-EXEC-002"}, findings[0].EntryIds)
	})

	t.Run("guard-dead", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "not never"), rule("SKILL-DEMO-EXEC-002", "done => approved")},
			[]Transition{step("SKILL-DEMO-PROC-001", "never", "done")},
		)
		findings := runBattery(t, model)
		guard := findingsOf(CheckGuard, "SKILL-DEMO-PROC-001", findings)
		require.Len(t, guard, 1)
		assert.Equal(t, "entry SKILL-DEMO-PROC-001: guard can never hold — the entry is dead", guard[0].Message)
		for _, check := range []string{CheckRedundantGuard, CheckPreservation, CheckReach} {
			assert.Empty(t, findingsOf(check, "SKILL-DEMO-PROC-001", findings), check)
		}
	})

	t.Run("redundant-conjunct", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "budget >= 0")},
			[]Transition{step("SKILL-DEMO-PROC-001", "armed and budget >= 0")},
		)
		redundant := findingsOf(CheckRedundantGuard, "SKILL-DEMO-PROC-001", runBattery(t, model))
		require.Len(t, redundant, 1)
		assert.Contains(t, redundant[0].Message, `guard conjunct "budget >= 0" is redundant`)
	})

	t.Run("guard-always-holds", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "budget >= 0")},
			[]Transition{step("SKILL-DEMO-PROC-001", "budget >= 0")},
		)
		redundant := findingsOf(CheckRedundantGuard, "SKILL-DEMO-PROC-001", runBattery(t, model))
		require.Len(t, redundant, 1)
		assert.Equal(t, "entry SKILL-DEMO-PROC-001: guard always holds — the invariants imply it", redundant[0].Message)
	})

	t.Run("halt", func(t *testing.T) {
		stop := Transition{Id: "SKILL-DEMO-PROC-002", Kind: KindStop, Pred: "budget >= 0", Section: "run", Text: "t"}
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "budget >= 0")},
			[]Transition{step("SKILL-DEMO-PROC-001", ""), stop},
		)
		findings := runBattery(t, model)
		halt := findingsOf(CheckHalt, "SKILL-DEMO-PROC-002", findings)
		require.Len(t, halt, 1)
		assert.Equal(t, "stop SKILL-DEMO-PROC-002 halts every run: its guard always holds", halt[0].Message)
		assert.Empty(t, findingsOf(CheckRedundantGuard, "SKILL-DEMO-PROC-002", findings))
	})

	t.Run("preservation", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "done => approved")},
			[]Transition{step("SKILL-DEMO-PROC-001", "", "done")},
		)
		preservation := findingsOf(CheckPreservation, "", runBattery(t, model))
		require.Len(t, preservation, 1)
		assert.Equal(t, []string{"SKILL-DEMO-PROC-001", "SKILL-DEMO-EXEC-001"}, preservation[0].EntryIds)
		assert.Equal(t, "firing SKILL-DEMO-PROC-001 can break invariant SKILL-DEMO-EXEC-001", preservation[0].Message)
	})

	t.Run("reach-unreachable", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "not never")},
			[]Transition{step("SKILL-DEMO-PROC-001", "never", "started"), step("SKILL-DEMO-PROC-002", "started")},
		)
		reach := findingsOf(CheckReach, "SKILL-DEMO-PROC-002", runBattery(t, model))
		require.Len(t, reach, 1)
		assert.Equal(t, "entry SKILL-DEMO-PROC-002 is unreachable within the complete bound 2", reach[0].Message)
	})

	t.Run("conditioned-reach", func(t *testing.T) {
		model := batteryModel(nil, []Transition{
			step("SKILL-DEMO-PROC-001", "mode = nightly", "nightly_done"),
			step("SKILL-DEMO-PROC-002", "nightly_done"),
		})
		findings := runBattery(t, model)
		finish := findingsOf(CheckReach, "SKILL-DEMO-PROC-002", findings)
		require.Len(t, finish, 1)
		assert.Equal(t, "entry SKILL-DEMO-PROC-002 is unreachable under mode = interactive", finish[0].Message)
		assert.Empty(t, findingsOf(CheckReach, "SKILL-DEMO-PROC-001", findings))
	})

	t.Run("completion", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "not never")},
			[]Transition{step("SKILL-DEMO-PROC-001", ""), step("SKILL-DEMO-PROC-002", "never")},
		)
		model.Sections = append(model.Sections, Section{Id: "wrap", After: []string{"run"}, Kind: skillgraph.SectionKindPhase, Title: "Wrap up"})
		model.Transitions[1].Section = "wrap"
		completion := findingsOf(CheckCompletion, "", runBattery(t, model))
		require.Len(t, completion, 1)
		assert.Equal(t, "no run reaches the final phase Wrap up", completion[0].Message)
		assert.Empty(t, completion[0].EntryIds)
	})

	t.Run("route-replaced", func(t *testing.T) {
		model := batteryModel(nil, []Transition{
			step("SKILL-DEMO-PROC-001", ""),
			{Id: "SKILL-DEMO-NIGHT-001", Kind: KindStep, Pred: "mode = nightly", Section: "nightly-route", Text: "t"},
		})
		route := Section{Id: "nightly-route", Kind: skillgraph.SectionKindRoute, Pred: "mode = nightly", Replaces: []string{"SKILL-DEMO-PROC-001"}, Title: "Nightly route"}
		model.Sections = append(model.Sections, route)
		for _, finding := range runBattery(t, model) {
			isReplaced := slices.Contains(finding.EntryIds, "SKILL-DEMO-PROC-001")
			assert.False(t, isReplaced && strings.Contains(finding.Message, "under mode = nightly"), finding.Message)
		}
	})

	t.Run("finding-order-and-entry-ids", func(t *testing.T) {
		model := batteryModel(
			[]Invariant{rule("SKILL-DEMO-EXEC-001", "not never"), rule("SKILL-DEMO-EXEC-002", "done => approved")},
			[]Transition{
				step("SKILL-DEMO-PROC-001", "", "done"),
				step("SKILL-DEMO-PROC-002", "never", "started"),
				step("SKILL-DEMO-PROC-003", "started"),
			},
		)
		findings := runBattery(t, model)
		require.NotEmpty(t, findings)
		positions := entryPositions(model)
		lastCheck := 0
		for _, finding := range findings {
			checkIndex := slices.Index(checkRunOrder, finding.Check)
			assert.GreaterOrEqual(t, checkIndex, lastCheck, "finding-order")
			lastCheck = checkIndex
			for _, id := range finding.EntryIds {
				_, isEntry := positions[id]
				assert.True(t, isEntry, "entry-ids-are-model-ids: %s", id)
			}
		}
	})
}

func TestRunBattery_RailroadReview(t *testing.T) {
	skipWithoutZ3(t)
	if testing.Short() {
		t.Skip("the railroad-review battery runs for seconds")
	}

	shared, err := skillgraph.LoadVocabulary("../../skills/vocabulary.cue")
	require.NoError(t, err)
	model, _ := deriveSkill(t, "../../skills/quality/railroad-review", shared)
	started := time.Now()
	findings := runBattery(t, model)
	elapsed := time.Since(started)

	for _, finding := range findings {
		assert.NotEqual(t, SeverityError, finding.Severity, finding.Message)
		assert.NotContains(t, finding.Message, "undecided")
	}
	assert.Less(t, elapsed, 30*time.Second)
}

func TestObligation_Finding(t *testing.T) {
	type testCase struct {
		_expected *Finding
		_id       string

		obligation *obligation
	}

	satisfied := &obligation{verdict: VerdictSat}
	failed := &obligation{verdict: VerdictUnsat}
	undecided := &obligation{verdict: VerdictUnknown}
	base := func(verdict string, requires ...*obligation) *obligation {
		return &obligation{
			check:          CheckReach,
			entryIds:       []string{"SKILL-A-PROC-001"},
			findingVerdict: VerdictUnsat,
			message:        "entry SKILL-A-PROC-001 is unreachable",
			requires:       requires,
			subject:        "entry SKILL-A-PROC-001",
			verdict:        verdict,
		}
	}
	tests := make([]*testCase, 0)

	// defect-verdict
	tests = append(tests, &testCase{
		_expected:  &Finding{Check: CheckReach, EntryIds: []string{"SKILL-A-PROC-001"}, Message: "entry SKILL-A-PROC-001 is unreachable", Severity: SeverityWarning},
		_id:        "defect-verdict",
		obligation: base(VerdictUnsat, satisfied),
	})

	// other-verdict
	tests = append(tests, &testCase{_expected: nil, _id: "other-verdict", obligation: base(VerdictSat)})

	// unknown
	tests = append(tests, &testCase{
		_expected:  &Finding{Check: CheckReach, EntryIds: []string{"SKILL-A-PROC-001"}, Message: "reach undecided within the 10s check timeout: entry SKILL-A-PROC-001", Severity: SeverityWarning},
		_id:        "unknown",
		obligation: base(VerdictUnknown),
	})

	// requirement-unsat
	tests = append(tests, &testCase{_expected: nil, _id: "requirement-unsat", obligation: base(VerdictUnsat, failed)})

	// requirement-unknown
	tests = append(tests, &testCase{_expected: nil, _id: "requirement-unknown", obligation: base(VerdictUnsat, undecided)})

	// filter-only
	tests = append(tests, &testCase{_expected: nil, _id: "filter-only", obligation: &obligation{verdict: VerdictUnsat}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.obligation.finding())
		})
	}
}

func TestFinding_Json(t *testing.T) {
	finding := Finding{Check: CheckGuard, EntryIds: []string{"SKILL-A-PROC-001"}, Message: "m", Severity: SeverityWarning}
	data, err := json.Marshal(finding)
	require.NoError(t, err)
	assert.JSONEq(t, `{"check": "guard", "entryIds": ["SKILL-A-PROC-001"], "message": "m", "severity": "warning"}`, string(data))
}

func TestExprText(t *testing.T) {
	type testCase struct {
		_expected string
		_id       string

		input string
	}

	tests := make([]*testCase, 0)

	// nested-and-or
	tests = append(tests, &testCase{_expected: "a and (b or c)", _id: "nested-and-or", input: "a and (b or c)"})

	// not-over-implies
	tests = append(tests, &testCase{_expected: "not (a => b)", _id: "not-over-implies", input: "not (a => b)"})

	// right-assoc-implies
	tests = append(tests, &testCase{_expected: "a => (b => c)", _id: "right-assoc-implies", input: "a => b => c"})

	// arithmetic-cmp
	tests = append(tests, &testCase{_expected: "lanes_spawned - lanes_returned > 0", _id: "arithmetic-cmp", input: "lanes_spawned - lanes_returned > 0"})

	// enum-condition
	tests = append(tests, &testCase{_expected: "mode = nightly", _id: "enum-condition", input: "mode = nightly"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			text := exprText(mustParse(t, test.input))
			assert.Equal(t, test._expected, text)
			assert.Equal(t, text, exprText(mustParse(t, text)), "re-parses to the same text")
		})
	}
}
