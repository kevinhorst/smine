package skillmodel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// errorText returns an error's message, or "" for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// typedVocabulary extends the fixture vocabulary with an int pair and an
// ordered enum for the typed-emission tests.
func typedVocabulary() *Vocabulary {
	vocab := fixtureVocabulary()
	vocab.Sorts["count"] = []Atom{
		{Id: "lanes_returned", Kind: skillgraph.KindAtomInt},
		{Id: "lanes_spawned", Kind: skillgraph.KindAtomInt},
	}
	vocab.Sorts["level"] = []Atom{
		{Id: "apply_level", Kind: skillgraph.KindAtomEnum, Members: []string{"low", "medium", "high"}},
	}
	return vocab
}

// vocabFixtureModel is the demo fixture on the model tier over the fixture
// vocabulary.
func vocabFixtureModel(t *testing.T) *Model {
	t.Helper()
	model := fixtureModel(t)
	model.IsStrict = true
	model.Vocabulary = fixtureVocabulary()
	return model
}

// linearModel is the smallest model exercising every unrolling family: two
// phases, a guarded stop, a route replacing a phase step, an invariant and
// an atom outside every cone.
func linearModel() *Model {
	modeAtom := Atom{
		Id:      "mode",
		Kind:    skillgraph.KindAtomEnum,
		Members: []string{"interactive", "nightly"},
		Source:  "session",
	}
	vocab := &Vocabulary{
		Sorts: map[string][]Atom{
			"event": {{Id: "logged"}, {Id: "prompted"}, {Id: "ready"}},
			"input": {{Id: "cancelled", Source: "arg"}},
			"mode":  {modeAtom},
		},
	}
	nightlyRoute := Section{
		Id:       "nightly-route",
		Kind:     skillgraph.SectionKindRoute,
		Pred:     "mode = nightly",
		Replaces: []string{"SKILL-LINEAR-PROC-002"},
		Title:    "Nightly route",
	}
	model := &Model{
		Coverable:  5,
		Formalized: 5,
		Invariants: []Invariant{
			{Id: "SKILL-LINEAR-EXEC-001", Expr: "mode = nightly => not prompted", Section: "intake", Text: "Nightly runs NEVER prompt."},
		},
		IsStrict: true,
		Sections: []Section{
			{Id: "intake", Kind: skillgraph.SectionKindPhase, Title: "Intake"},
			{Id: "run", After: []string{"intake"}, Kind: skillgraph.SectionKindPhase, Title: "Run"},
			nightlyRoute,
		},
		Skill: "linear",
		Transitions: []Transition{
			{Id: "SKILL-LINEAR-PROC-001", Kind: KindStep, Post: []string{"ready"}, Section: "intake", Text: "PREPARE the input."},
			{Id: "SKILL-LINEAR-PROC-002", Kind: KindStep, Post: []string{"prompted"}, Pred: "ready", Section: "run", Text: "ASK the user to confirm."},
			{Id: "SKILL-LINEAR-PROC-003", Kind: KindStop, Pred: "cancelled", Section: "run", Text: "If cancelled ⇒ STOP: report."},
			{Id: "SKILL-LINEAR-NIGHT-001", Kind: KindStep, Post: []string{"logged"}, Pred: "mode = nightly", Section: "nightly-route", Text: "APPLY the defaults and log them."},
		},
		Vocabulary: vocab,
	}
	return model
}

// linearReachDoc is EmitReachability(linearModel(), "SKILL-LINEAR-PROC-002",
// 3, nil) — the approved example document of the linear encoding.
const linearReachDoc = `(set-option :produce-unsat-cores true)
(set-option :timeout 10000)
(declare-const cancelled__s0 Bool)
(declare-const cancelled__s1 Bool)
(declare-const cancelled__s2 Bool)
(declare-const cancelled__s3 Bool)
(declare-const cancelled__s4 Bool)
(declare-datatypes ((mode_sort 0)) (((interactive) (nightly))))
(declare-const mode mode_sort)
(define-fun mode_rank ((x mode_sort)) Int (ite ((_ is interactive) x) 0 1))
(declare-const prompted__s0 Bool)
(declare-const prompted__s1 Bool)
(declare-const prompted__s2 Bool)
(declare-const prompted__s3 Bool)
(declare-const prompted__s4 Bool)
(declare-const ready__s0 Bool)
(declare-const ready__s1 Bool)
(declare-const ready__s2 Bool)
(declare-const ready__s3 Bool)
(declare-const ready__s4 Bool)
(declare-const t0_s0 Bool)
(declare-const t1_s0 Bool)
(declare-const t2_s0 Bool)
(declare-const t3_s0 Bool)
(declare-const sel_s0 Int)
(declare-const ph_s0 Int)
(declare-const t0_s1 Bool)
(declare-const t1_s1 Bool)
(declare-const t2_s1 Bool)
(declare-const t3_s1 Bool)
(declare-const sel_s1 Int)
(declare-const ph_s1 Int)
(declare-const t0_s2 Bool)
(declare-const t1_s2 Bool)
(declare-const t2_s2 Bool)
(declare-const t3_s2 Bool)
(declare-const sel_s2 Int)
(declare-const ph_s2 Int)
(declare-const t0_s3 Bool)
(declare-const t1_s3 Bool)
(declare-const t2_s3 Bool)
(declare-const t3_s3 Bool)
(declare-const sel_s3 Int)
(declare-const ph_s3 Int)
(declare-const halted_s0 Bool)
(declare-const halted_s1 Bool)
(declare-const halted_s2 Bool)
(declare-const halted_s3 Bool)
(declare-const halted_s4 Bool)
(assert (! (=> (= mode nightly) (not prompted__s0)) :named SKILL_LINEAR_EXEC_001.s0))
(assert (! (=> (= mode nightly) (not prompted__s1)) :named SKILL_LINEAR_EXEC_001.s1))
(assert (! (=> (= mode nightly) (not prompted__s2)) :named SKILL_LINEAR_EXEC_001.s2))
(assert (! (=> (= mode nightly) (not prompted__s3)) :named SKILL_LINEAR_EXEC_001.s3))
(assert (! (=> (= mode nightly) (not prompted__s4)) :named SKILL_LINEAR_EXEC_001.s4))
(assert (! (= cancelled__s1 cancelled__s0) :named rigid.cancelled.s1))
(assert (! (= cancelled__s2 cancelled__s0) :named rigid.cancelled.s2))
(assert (! (= cancelled__s3 cancelled__s0) :named rigid.cancelled.s3))
(assert (! (= cancelled__s4 cancelled__s0) :named rigid.cancelled.s4))
(assert (! (not prompted__s0) :named init.prompted))
(assert (! (=> prompted__s0 prompted__s1) :named frame.prompted.s0))
(assert (! (=> prompted__s1 (or prompted__s0 t1_s0)) :named cause.prompted.s0))
(assert (! (=> prompted__s1 prompted__s2) :named frame.prompted.s1))
(assert (! (=> prompted__s2 (or prompted__s1 t1_s1)) :named cause.prompted.s1))
(assert (! (=> prompted__s2 prompted__s3) :named frame.prompted.s2))
(assert (! (=> prompted__s3 (or prompted__s2 t1_s2)) :named cause.prompted.s2))
(assert (! (=> prompted__s3 prompted__s4) :named frame.prompted.s3))
(assert (! (=> prompted__s4 (or prompted__s3 t1_s3)) :named cause.prompted.s3))
(assert (! (not ready__s0) :named init.ready))
(assert (! (=> ready__s0 ready__s1) :named frame.ready.s0))
(assert (! (=> ready__s1 (or ready__s0 t0_s0)) :named cause.ready.s0))
(assert (! (=> ready__s1 ready__s2) :named frame.ready.s1))
(assert (! (=> ready__s2 (or ready__s1 t0_s1)) :named cause.ready.s1))
(assert (! (=> ready__s2 ready__s3) :named frame.ready.s2))
(assert (! (=> ready__s3 (or ready__s2 t0_s2)) :named cause.ready.s2))
(assert (! (=> ready__s3 ready__s4) :named frame.ready.s3))
(assert (! (=> ready__s4 (or ready__s3 t0_s3)) :named cause.ready.s3))
(assert (! (=> t0_s0 ready__s1) :named post.SKILL_LINEAR_PROC_001.s0))
(assert (=> t0_s0 (= sel_s0 0)))
(assert (=> t0_s0 (not halted_s0)))
(assert (=> t0_s0 (and (<= 0 ph_s0) (<= ph_s0 0))))
(assert (! (=> t0_s1 ready__s2) :named post.SKILL_LINEAR_PROC_001.s1))
(assert (=> t0_s1 (= sel_s1 0)))
(assert (=> t0_s1 (not halted_s1)))
(assert (=> t0_s1 (and (<= 0 ph_s1) (<= ph_s1 0))))
(assert (! (=> t0_s2 ready__s3) :named post.SKILL_LINEAR_PROC_001.s2))
(assert (=> t0_s2 (= sel_s2 0)))
(assert (=> t0_s2 (not halted_s2)))
(assert (=> t0_s2 (and (<= 0 ph_s2) (<= ph_s2 0))))
(assert (! (=> t0_s3 ready__s4) :named post.SKILL_LINEAR_PROC_001.s3))
(assert (=> t0_s3 (= sel_s3 0)))
(assert (=> t0_s3 (not halted_s3)))
(assert (=> t0_s3 (and (<= 0 ph_s3) (<= ph_s3 0))))
(assert (=> t1_s0 (and ready__s0 (not (= mode nightly)))))
(assert (! (=> t1_s0 prompted__s1) :named post.SKILL_LINEAR_PROC_002.s0))
(assert (=> t1_s0 (= sel_s0 1)))
(assert (=> t1_s0 (not halted_s0)))
(assert (=> t1_s0 (and (<= 1 ph_s0) (<= ph_s0 1))))
(assert (=> t1_s1 (and ready__s1 (not (= mode nightly)))))
(assert (! (=> t1_s1 prompted__s2) :named post.SKILL_LINEAR_PROC_002.s1))
(assert (=> t1_s1 (= sel_s1 1)))
(assert (=> t1_s1 (not halted_s1)))
(assert (=> t1_s1 (and (<= 1 ph_s1) (<= ph_s1 1))))
(assert (=> t1_s2 (and ready__s2 (not (= mode nightly)))))
(assert (! (=> t1_s2 prompted__s3) :named post.SKILL_LINEAR_PROC_002.s2))
(assert (=> t1_s2 (= sel_s2 1)))
(assert (=> t1_s2 (not halted_s2)))
(assert (=> t1_s2 (and (<= 1 ph_s2) (<= ph_s2 1))))
(assert (=> t1_s3 (and ready__s3 (not (= mode nightly)))))
(assert (! (=> t1_s3 prompted__s4) :named post.SKILL_LINEAR_PROC_002.s3))
(assert (=> t1_s3 (= sel_s3 1)))
(assert (=> t1_s3 (not halted_s3)))
(assert (=> t1_s3 (and (<= 1 ph_s3) (<= ph_s3 1))))
(assert (=> t2_s0 cancelled__s0))
(assert (=> t2_s0 (= sel_s0 2)))
(assert (=> t2_s0 (not halted_s0)))
(assert (=> t2_s0 (and (<= 1 ph_s0) (<= ph_s0 1))))
(assert (=> t2_s1 cancelled__s1))
(assert (=> t2_s1 (= sel_s1 2)))
(assert (=> t2_s1 (not halted_s1)))
(assert (=> t2_s1 (and (<= 1 ph_s1) (<= ph_s1 1))))
(assert (=> t2_s2 cancelled__s2))
(assert (=> t2_s2 (= sel_s2 2)))
(assert (=> t2_s2 (not halted_s2)))
(assert (=> t2_s2 (and (<= 1 ph_s2) (<= ph_s2 1))))
(assert (=> t2_s3 cancelled__s3))
(assert (=> t2_s3 (= sel_s3 2)))
(assert (=> t2_s3 (not halted_s3)))
(assert (=> t2_s3 (and (<= 1 ph_s3) (<= ph_s3 1))))
(assert (=> t3_s0 (= mode nightly)))
(assert (=> t3_s0 (= sel_s0 3)))
(assert (=> t3_s0 (not halted_s0)))
(assert (=> t3_s0 (and (<= 1 ph_s0) (<= ph_s0 1))))
(assert (=> t3_s1 (= mode nightly)))
(assert (=> t3_s1 (= sel_s1 3)))
(assert (=> t3_s1 (not halted_s1)))
(assert (=> t3_s1 (and (<= 1 ph_s1) (<= ph_s1 1))))
(assert (=> t3_s2 (= mode nightly)))
(assert (=> t3_s2 (= sel_s2 3)))
(assert (=> t3_s2 (not halted_s2)))
(assert (=> t3_s2 (and (<= 1 ph_s2) (<= ph_s2 1))))
(assert (=> t3_s3 (= mode nightly)))
(assert (=> t3_s3 (= sel_s3 3)))
(assert (=> t3_s3 (not halted_s3)))
(assert (=> t3_s3 (and (<= 1 ph_s3) (<= ph_s3 1))))
(assert (<= ph_s0 ph_s1))
(assert (<= ph_s1 ph_s2))
(assert (<= ph_s2 ph_s3))
(assert (not halted_s0))
(assert (! (= halted_s1 (or halted_s0 t2_s0)) :named halt.s0))
(assert (! (= halted_s2 (or halted_s1 t2_s1)) :named halt.s1))
(assert (! (= halted_s3 (or halted_s2 t2_s2)) :named halt.s2))
(assert (! (= halted_s4 (or halted_s3 t2_s3)) :named halt.s3))
(assert (! (=> cancelled__s0 (and (not t0_s0) (not t1_s0) (not t3_s0))) :named stopblock.SKILL_LINEAR_PROC_003.s0))
(assert (! (=> cancelled__s1 (and (not t0_s1) (not t1_s1) (not t3_s1))) :named stopblock.SKILL_LINEAR_PROC_003.s1))
(assert (! (=> cancelled__s2 (and (not t0_s2) (not t1_s2) (not t3_s2))) :named stopblock.SKILL_LINEAR_PROC_003.s2))
(assert (! (=> cancelled__s3 (and (not t0_s3) (not t1_s3) (not t3_s3))) :named stopblock.SKILL_LINEAR_PROC_003.s3))
(assert (! (or t1_s0 t1_s1 t1_s2 t1_s3) :named reach.SKILL_LINEAR_PROC_002))
(check-sat)
(get-unsat-core)
`

func TestEmitConsistency(t *testing.T) {
	doc, err := EmitConsistency(vocabFixtureModel(t))
	require.NoError(t, err)

	assert.Contains(t, doc.Text, "(set-option :produce-unsat-cores true)\n(set-option :timeout 10000)\n", "options")
	assert.Equal(t, 1, doc.Checks, "checks-is-one")
	assert.Contains(t, doc.Text, "(declare-const interactive Bool)")
	assert.Contains(t, doc.Text, "(assert (! (not (and interactive nightly)) :named vocab.exclusive0.interactive.nightly))", "exclusivity-name")
	assert.Contains(t, doc.Text, ":named SKILL_DEMO_EXEC_001", "invariant-name")
	assert.Contains(t, doc.Text, "(check-sat)\n(get-unsat-core)\n")
	assert.NotContains(t, doc.Text, "(get-model)")
	assert.NotContains(t, doc.Text, "assert-soft")

	again, err := EmitConsistency(vocabFixtureModel(t))
	require.NoError(t, err)
	assert.Equal(t, doc.Text, again.Text, "deterministic")
}

func TestEmitDeclarationsTyped(t *testing.T) {
	builder := &strings.Builder{}
	emitDeclarations(builder, typedVocabulary())
	text := builder.String()

	assert.Contains(t, text, "(declare-const lanes_returned Int)")
	assert.Contains(t, text, "(declare-datatypes ((apply_level_sort 0)) (((low) (medium) (high))))")
	assert.Contains(t, text, "(declare-const apply_level apply_level_sort)")
	assert.Contains(t, text, "(define-fun apply_level_rank ((x apply_level_sort)) Int (ite ((_ is low) x) 0 (ite ((_ is medium) x) 1 2)))")
}

func TestEmitCmpForms(t *testing.T) {
	render := newEmitter(typedVocabulary())

	intCmp, err := ParseExpr("lanes_returned = lanes_spawned")
	require.NoError(t, err)
	assert.Equal(t, "(= lanes_returned lanes_spawned)", render.emitExpr(intCmp))

	arith, err := ParseExpr("lanes_spawned - lanes_returned > 0")
	require.NoError(t, err)
	assert.Equal(t, "(> (- lanes_spawned lanes_returned) 0)", render.emitExpr(arith))

	enumEq, err := ParseExpr("apply_level = medium")
	require.NoError(t, err)
	assert.Equal(t, "(= apply_level medium)", render.emitExpr(enumEq))

	enumNe, err := ParseExpr("apply_level != low")
	require.NoError(t, err)
	assert.Equal(t, "(not (= apply_level low))", render.emitExpr(enumNe))

	enumOrder, err := ParseExpr("apply_level >= medium")
	require.NoError(t, err)
	assert.Equal(t, "(>= (apply_level_rank apply_level) (apply_level_rank medium))", render.emitExpr(enumOrder))
}

func TestEmitQuery(t *testing.T) {
	type testCase struct {
		_id         string
		_contains   []string
		_shouldPass bool

		model *Model
		query Query
	}

	softModel := vocabFixtureModel(t)
	softModel.Soft = []Invariant{{Id: "SKILL-DEMO-QUAL-001", Expr: "nightly", Section: "dispatch", Text: "Runs SHOULD be nightly."}}
	tests := make([]*testCase, 0)

	// named-assume-and-check
	tests = append(tests, &testCase{
		_id:         "named-assume-and-check",
		_contains:   []string{":named query.demo_query.assume0", ":named query.demo_query.check", "(check-sat)\n(get-unsat-core)\n(get-model)\n"},
		_shouldPass: true,

		model: vocabFixtureModel(t),
		query: Query{Name: "demo-query", Assume: []string{"nightly"}, Check: "interactive", Expect: "unsat"},
	})

	// soft
	tests = append(tests, &testCase{
		_id:         "soft",
		_contains:   []string{"(assert-soft nightly)"},
		_shouldPass: true,

		model: softModel,
		query: Query{Name: "soft-query", Check: "interactive", Expect: "sat"},
	})

	// bad-assume
	tests = append(tests, &testCase{
		_id:         "bad-assume",
		_contains:   []string{"EmitQuery: assume[0]"},
		_shouldPass: false,

		model: vocabFixtureModel(t),
		query: Query{Name: "bad", Assume: []string{"nightly and"}, Check: "interactive"},
	})

	// bad-check
	tests = append(tests, &testCase{
		_id:         "bad-check",
		_contains:   []string{"EmitQuery: check"},
		_shouldPass: false,

		model: vocabFixtureModel(t),
		query: Query{Name: "bad", Check: "interactive or"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			doc, err := EmitQuery(test.model, test.query)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			for _, fragment := range test._contains {
				assert.Contains(t, doc.Text+errorText(err), fragment)
			}
			if test._shouldPass {
				assert.Equal(t, 1, doc.Checks)
			}
		})
	}
}

func TestEmitReachability(t *testing.T) {
	type testCase struct {
		_id         string
		_contains   []string
		_expected   string
		_shouldPass bool

		assume       []string
		bound        int
		transitionId string
	}

	tests := make([]*testCase, 0)

	// golden-linear-model
	tests = append(tests, &testCase{
		_id:         "golden-linear-model",
		_expected:   linearReachDoc,
		_shouldPass: true,

		bound:        3,
		transitionId: "SKILL-LINEAR-PROC-002",
	})

	// assume-over-state-zero
	tests = append(tests, &testCase{
		_id:         "assume-over-state-zero",
		_contains:   []string{"(assert (! (= mode nightly) :named reach.assume0))"},
		_shouldPass: true,

		assume:       []string{"mode = nightly"},
		bound:        3,
		transitionId: "SKILL-LINEAR-PROC-002",
	})

	// bound-zero
	tests = append(tests, &testCase{
		_id:         "bound-zero",
		_contains:   []string{"(declare-const ready__s1 Bool)", "(assert (! (or t1_s0) :named reach.SKILL_LINEAR_PROC_002))"},
		_shouldPass: true,

		bound:        0,
		transitionId: "SKILL-LINEAR-PROC-002",
	})

	// unknown-transition
	tests = append(tests, &testCase{
		_id:         "unknown-transition",
		_contains:   []string{"unknown transition SKILL-LINEAR-NOPE-001"},
		_shouldPass: false,

		bound:        3,
		transitionId: "SKILL-LINEAR-NOPE-001",
	})

	// bad-assume
	tests = append(tests, &testCase{
		_id:         "bad-assume",
		_contains:   []string{"EmitReachability: assume[0]"},
		_shouldPass: false,

		assume:       []string{"ready and"},
		bound:        3,
		transitionId: "SKILL-LINEAR-PROC-002",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			doc, err := EmitReachability(linearModel(), test.transitionId, test.bound, test.assume)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			if test._expected != "" {
				assert.Equal(t, test._expected, doc.Text)
			}
			for _, fragment := range test._contains {
				assert.Contains(t, doc.Text+errorText(err), fragment)
			}
			if test._shouldPass {
				assert.Equal(t, 1, doc.Checks, "checks-is-one")
			}
		})
	}

	boundZero, err := EmitReachability(linearModel(), "SKILL-LINEAR-PROC-002", 0, nil)
	require.NoError(t, err)
	assert.NotContains(t, boundZero.Text, "(assert (<= ph_", "bound-zero-no-monotone-line")
	assert.NotContains(t, boundZero.Text, "ready__s2", "bound-zero-states-0-1")
}

func TestEmitReachabilityAnchors(t *testing.T) {
	type testCase struct {
		_id          string
		_contains    []string
		_notContains []string

		model *Model
	}

	contextModel := linearModel()
	contextModel.Sections = append(contextModel.Sections, Section{Id: "reference", Kind: skillgraph.SectionKindContext, Title: "Reference"})
	contextModel.Transitions = append(contextModel.Transitions, Transition{Id: "SKILL-LINEAR-REF-001", Kind: KindStep, Section: "reference", Text: "RUN it."})
	replacedInvariant := linearModel()
	replacedInvariant.Sections[2].Replaces = []string{"SKILL-LINEAR-PROC-002", "SKILL-LINEAR-EXEC-001"}
	tests := make([]*testCase, 0)

	// replaced-guard
	tests = append(tests, &testCase{
		_id:       "replaced-guard",
		_contains: []string{"(assert (=> t1_s0 (and ready__s0 (not (= mode nightly)))))"},

		model: linearModel(),
	})

	// route-window
	tests = append(tests, &testCase{
		_id:       "route-window",
		_contains: []string{"(assert (=> t3_s0 (and (<= 1 ph_s0) (<= ph_s0 1))))"},

		model: linearModel(),
	})

	// context-transition-unranked
	tests = append(tests, &testCase{
		_id:          "context-transition-unranked",
		_contains:    []string{"(assert (=> t4_s0 (= sel_s0 4)))", "(assert (=> t4_s0 (not halted_s0)))"},
		_notContains: []string{"(assert (=> t4_s0 (and (<= "},

		model: contextModel,
	})

	// replaced-invariant
	tests = append(tests, &testCase{
		_id:       "replaced-invariant",
		_contains: []string{"(assert (! (=> (not (= mode nightly)) (=> (= mode nightly) (not prompted__s0))) :named SKILL_LINEAR_EXEC_001.s0))"},

		model: replacedInvariant,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			doc, err := EmitReachability(test.model, "SKILL-LINEAR-PROC-002", 3, nil)
			require.NoError(t, err)
			for _, fragment := range test._contains {
				assert.Contains(t, doc.Text, fragment)
			}
			for _, fragment := range test._notContains {
				assert.NotContains(t, doc.Text, fragment)
			}
		})
	}
}

func TestEmitReachabilityCone(t *testing.T) {
	type testCase struct {
		_id          string
		_contains    []string
		_notContains []string

		assume []string
		model  *Model
	}

	grouped := linearModel()
	grouped.Vocabulary.Exclusive = [][]string{{"ready", "cancelled"}}
	tests := make([]*testCase, 0)

	// non-cone-atom
	tests = append(tests, &testCase{
		_id:          "non-cone-atom",
		_notContains: []string{"logged__s0", "post.SKILL_LINEAR_NIGHT_001"},

		model: linearModel(),
	})

	// assume-widens-cone
	tests = append(tests, &testCase{
		_id:       "assume-widens-cone",
		_contains: []string{"(declare-const logged__s0 Bool)", ":named post.SKILL_LINEAR_NIGHT_001.s0"},

		assume: []string{"logged"},
		model:  linearModel(),
	})

	// exclusivity-in-cone
	tests = append(tests, &testCase{
		_id:       "exclusivity-in-cone",
		_contains: []string{"(assert (! (not (and ready__s0 cancelled__s0)) :named vocab.exclusive0.ready.cancelled.s0))"},

		model: grouped,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			doc, err := EmitReachability(test.model, "SKILL-LINEAR-PROC-002", 3, test.assume)
			require.NoError(t, err)
			for _, fragment := range test._contains {
				assert.Contains(t, doc.Text, fragment)
			}
			for _, fragment := range test._notContains {
				assert.NotContains(t, doc.Text, fragment)
			}
		})
	}

	t.Run("exclusivity-outside-cone-direct", func(t *testing.T) {
		model := linearModel()
		model.Vocabulary.Exclusive = [][]string{{"logged", "prompted"}}
		encoding, err := newEncoding(model)
		require.NoError(t, err)

		builder := &strings.Builder{}
		cone := map[string]bool{"cancelled": true, "prompted": true, "ready": true}
		emitUnrolling(3, builder, cone, encoding, model)
		assert.NotContains(t, builder.String(), "vocab.exclusive")
	})
}
