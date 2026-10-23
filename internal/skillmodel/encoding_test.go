package skillmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// mustParse parses an expression the test knows to be valid.
func mustParse(t *testing.T, text string) *Expr {
	t.Helper()
	expr, err := ParseExpr(text)
	require.NoError(t, err)
	return expr
}

func TestNewAnchors(t *testing.T) {
	type testCase struct {
		_contains   string
		_id         string
		_shouldPass bool
		_windows    map[string]RankWindow

		model *Model
	}

	tests := make([]*testCase, 0)

	// phase-window
	tests = append(tests, &testCase{
		_id:         "phase-window",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 1},
		},

		model: linearModel(),
	})

	// untyped-section-is-phase
	untyped := linearModel()
	untyped.Sections[0].Kind = ""
	tests = append(tests, &testCase{
		_id:         "untyped-section-is-phase",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 1},
		},

		model: untyped,
	})

	// between-window
	between := linearModel()
	between.Sections[2].Replaces = nil
	between.Sections[2].Between = []string{"SKILL-LINEAR-PROC-001", "SKILL-LINEAR-PROC-002"}
	tests = append(tests, &testCase{
		_id:         "between-window",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 0},
		},

		model: between,
	})

	// between-start-end
	startEnd := linearModel()
	startEnd.Sections[2].Replaces = nil
	startEnd.Sections[2].Between = []string{"start", "end"}
	tests = append(tests, &testCase{
		_id:         "between-start-end",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 0},
		},

		model: startEnd,
	})

	// replaces-window
	replacesBoth := linearModel()
	replacesBoth.Sections[2].Replaces = []string{"SKILL-LINEAR-PROC-001", "SKILL-LINEAR-PROC-002"}
	tests = append(tests, &testCase{
		_id:         "replaces-window",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 0},
		},

		model: replacesBoth,
	})

	// replaces-no-phase
	replacesNone := linearModel()
	replacesNone.Sections[2].Replaces = []string{"SKILL-LINEAR-GHOST-001"}
	tests = append(tests, &testCase{
		_id:         "replaces-no-phase",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 0},
		},

		model: replacesNone,
	})

	// predless-rule-anchor
	predless := linearModel()
	predless.Sections[2].Replaces = nil
	predless.Sections[2].Between = []string{"SKILL-LINEAR-RULE-009", "SKILL-LINEAR-PROC-001"}
	tests = append(tests, &testCase{
		_id:         "predless-rule-anchor",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 0, Lo: 0},
		},

		model: predless,
	})

	// context-unranked
	withContext := linearModel()
	withContext.Sections = append(withContext.Sections, Section{Id: "reference", Kind: skillgraph.SectionKindContext, Title: "Reference"})
	withContext.Transitions = append(withContext.Transitions, Transition{Id: "SKILL-LINEAR-REF-001", Kind: KindStep, Section: "reference", Text: "RUN it."})
	tests = append(tests, &testCase{
		_id:         "context-unranked",
		_shouldPass: true,
		_windows: map[string]RankWindow{
			"SKILL-LINEAR-PROC-001":  {Hi: 0, Lo: 0},
			"SKILL-LINEAR-PROC-002":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-PROC-003":  {Hi: 1, Lo: 1},
			"SKILL-LINEAR-NIGHT-001": {Hi: 1, Lo: 1},
		},

		model: withContext,
	})

	// fail-route-without-pred
	noPred := linearModel()
	noPred.Sections[2].Pred = ""
	tests = append(tests, &testCase{
		_contains:   "NewAnchors: route nightly-route",
		_id:         "fail-route-without-pred",
		_shouldPass: false,

		model: noPred,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			anchors, err := NewAnchors(test.model)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._contains)
			if test._shouldPass {
				assert.Equal(t, test._windows, anchors.Windows)
			}
		})
	}

	t.Run("replacing-routes", func(t *testing.T) {
		model := linearModel()
		second := Section{Id: "hotfix-route", Kind: skillgraph.SectionKindRoute, Pred: "cancelled", Replaces: []string{"SKILL-LINEAR-PROC-002"}, Title: "Hotfix route"}
		model.Sections = append(model.Sections, second)
		anchors, err := NewAnchors(model)
		require.NoError(t, err)

		expected := []Route{
			{Pred: mustParse(t, "mode = nightly"), SectionId: "nightly-route"},
			{Pred: mustParse(t, "cancelled"), SectionId: "hotfix-route"},
		}
		assert.Equal(t, expected, anchors.Replacing["SKILL-LINEAR-PROC-002"])
	})

	t.Run("routes-keyed-by-section", func(t *testing.T) {
		anchors, err := NewAnchors(linearModel())
		require.NoError(t, err)
		assert.Equal(t, map[string]*Expr{"nightly-route": mustParse(t, "mode = nightly")}, anchors.Routes)
	})
}

func TestAnchors_ReplacedGuard(t *testing.T) {
	type testCase struct {
		_expected *Expr
		_id       string

		pred         *Expr
		transitionId string
	}

	nightly := mustParse(t, "mode = nightly")
	cancelled := mustParse(t, "cancelled")
	ready := mustParse(t, "ready")
	anchors := &Anchors{Replacing: map[string][]Route{
		"SKILL-A-PROC-001": {{Pred: nightly, SectionId: "nightly"}},
		"SKILL-A-PROC-002": {{Pred: nightly, SectionId: "nightly"}, {Pred: cancelled, SectionId: "hotfix"}},
	}}
	tests := make([]*testCase, 0)

	// unreplaced
	tests = append(tests, &testCase{_expected: ready, _id: "unreplaced", pred: ready, transitionId: "SKILL-A-PROC-009"})

	// replaced-guard
	tests = append(tests, &testCase{_expected: andExpr(ready, notExpr(nightly)), _id: "replaced-guard", pred: ready, transitionId: "SKILL-A-PROC-001"})

	// unguarded-replaced
	tests = append(tests, &testCase{_expected: notExpr(nightly), _id: "unguarded-replaced", pred: nil, transitionId: "SKILL-A-PROC-001"})

	// two-routes
	tests = append(tests, &testCase{
		_expected:    andExpr(andExpr(ready, notExpr(nightly)), notExpr(cancelled)),
		_id:          "two-routes",
		pred:         ready,
		transitionId: "SKILL-A-PROC-002",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, anchors.ReplacedGuard(test.pred, test.transitionId))
		})
	}
}

func TestAnchors_ReplacedClause(t *testing.T) {
	type testCase struct {
		_expected *Expr
		_id       string

		entryId string
	}

	nightly := mustParse(t, "mode = nightly")
	cancelled := mustParse(t, "cancelled")
	invariant := mustParse(t, "not prompted")
	anchors := &Anchors{Replacing: map[string][]Route{
		"SKILL-A-EXEC-001": {{Pred: nightly, SectionId: "nightly"}},
		"SKILL-A-EXEC-002": {{Pred: nightly, SectionId: "nightly"}, {Pred: cancelled, SectionId: "hotfix"}},
	}}
	tests := make([]*testCase, 0)

	// unreplaced
	tests = append(tests, &testCase{_expected: invariant, _id: "unreplaced", entryId: "SKILL-A-EXEC-009"})

	// replaced-invariant
	tests = append(tests, &testCase{_expected: impliesExpr(notExpr(nightly), invariant), _id: "replaced-invariant", entryId: "SKILL-A-EXEC-001"})

	// two-routes
	tests = append(tests, &testCase{
		_expected: impliesExpr(notExpr(cancelled), impliesExpr(notExpr(nightly), invariant)),
		_id:       "two-routes",
		entryId:   "SKILL-A-EXEC-002",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, anchors.ReplacedClause(invariant, test.entryId))
		})
	}
}

func TestNewEncoding(t *testing.T) {
	t.Run("guards-through-anchors", func(t *testing.T) {
		model := linearModel()
		encoding, err := newEncoding(model)
		require.NoError(t, err)
		expected := encoding.anchors.ReplacedGuard(mustParse(t, "ready"), "SKILL-LINEAR-PROC-002")
		assert.Equal(t, expected, encoding.guards["SKILL-LINEAR-PROC-002"])
		assert.Nil(t, encoding.guards["SKILL-LINEAR-PROC-001"])
	})

	t.Run("invariants-through-anchors", func(t *testing.T) {
		model := linearModel()
		model.Sections[2].Replaces = []string{"SKILL-LINEAR-EXEC-001"}
		encoding, err := newEncoding(model)
		require.NoError(t, err)
		expected := encoding.anchors.ReplacedClause(mustParse(t, "mode = nightly => not prompted"), "SKILL-LINEAR-EXEC-001")
		assert.Equal(t, expected, encoding.invariants[0].expr)
	})

	t.Run("soft-through-anchors", func(t *testing.T) {
		model := linearModel()
		model.Soft = []Invariant{{Id: "SKILL-LINEAR-QUAL-001", Expr: "ready", Section: "run", Text: "t"}}
		model.Sections[2].Replaces = []string{"SKILL-LINEAR-QUAL-001"}
		encoding, err := newEncoding(model)
		require.NoError(t, err)
		expected := impliesExpr(notExpr(mustParse(t, "mode = nightly")), mustParse(t, "ready"))
		assert.Equal(t, expected, encoding.soft[0].expr)
	})

	t.Run("own-and-route-conjuncts", func(t *testing.T) {
		model := linearModel()
		model.Transitions[3].Pred = "(mode = nightly) and (ready and cancelled)"
		encoding, err := newEncoding(model)
		require.NoError(t, err)
		assert.Equal(t, []*Expr{mustParse(t, "mode = nightly")}, encoding.routeConjuncts["SKILL-LINEAR-NIGHT-001"])
		assert.Equal(t, []*Expr{mustParse(t, "ready"), mustParse(t, "cancelled")}, encoding.ownConjuncts["SKILL-LINEAR-NIGHT-001"])
	})

	t.Run("route-only-entry", func(t *testing.T) {
		encoding, err := newEncoding(linearModel())
		require.NoError(t, err)
		assert.Empty(t, encoding.ownConjuncts["SKILL-LINEAR-NIGHT-001"])
		assert.Equal(t, []*Expr{mustParse(t, "mode = nightly")}, encoding.routeConjuncts["SKILL-LINEAR-NIGHT-001"])
	})

	t.Run("fail-bad-pred", func(t *testing.T) {
		model := linearModel()
		model.Transitions[1].Pred = "ready and"
		_, err := newEncoding(model)
		assert.ErrorContains(t, err, "newEncoding: pred SKILL-LINEAR-PROC-002")
	})

	t.Run("fail-unknown-section", func(t *testing.T) {
		model := linearModel()
		model.Transitions[0].Section = "warp"
		_, err := newEncoding(model)
		assert.ErrorContains(t, err, `Unknown section "warp"`)
	})

	t.Run("fail-route-without-pred", func(t *testing.T) {
		model := linearModel()
		model.Sections[2].Pred = ""
		_, err := newEncoding(model)
		assert.ErrorContains(t, err, "NewAnchors: route nightly-route")
	})
}

func TestConjuncts(t *testing.T) {
	type testCase struct {
		_expected []*Expr
		_id       string

		expr *Expr
	}

	tests := make([]*testCase, 0)

	// nil
	tests = append(tests, &testCase{_expected: nil, _id: "nil", expr: nil})

	// single
	tests = append(tests, &testCase{_expected: []*Expr{mustParse(t, "a")}, _id: "single", expr: mustParse(t, "a")})

	// nested-and-chain
	tests = append(tests, &testCase{
		_expected: []*Expr{mustParse(t, "a"), mustParse(t, "b"), mustParse(t, "c")},
		_id:       "nested-and-chain",
		expr:      mustParse(t, "a and (b and c)"),
	})

	// or-is-one-conjunct
	tests = append(tests, &testCase{_expected: []*Expr{mustParse(t, "a or b")}, _id: "or-is-one-conjunct", expr: mustParse(t, "a or b")})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, conjuncts(test.expr))
		})
	}
}
