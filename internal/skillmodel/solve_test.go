package skillmodel

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// deriveSkill loads a skill dir, attaches the shared vocabulary, derives its
// model and flattens its merged atoms into the solver vocabulary.
func deriveSkill(t *testing.T, dir string, shared *skillgraph.Vocabulary) (*Model, *Vocabulary) {
	t.Helper()
	skill, err := skillgraph.Load(dir)
	require.NoError(t, err)
	skill.Vocabulary = shared
	return deriveFixture(t, skill), VocabularyOf(skill)
}

// deriveInline imports a fixture body as the named skill under the shared
// vocabulary, declares its skill-local atoms and derives its model.
func deriveInline(t *testing.T, atoms map[string]*skillgraph.Atom, body, name string, shared *skillgraph.Vocabulary) *Model {
	t.Helper()
	skill := importFixture(t, body, name)
	skill.Atoms = atoms
	skill.Vocabulary = shared
	return deriveFixture(t, skill)
}

// solved runs an emitted document through Solve, failing the test on an
// error.
func solved(t *testing.T, doc SmtDoc, emitErr error, model *Model) *QueryResult {
	t.Helper()
	require.NoError(t, emitErr)
	result, err := Solve(context.Background(), doc, model)
	require.NoError(t, err)
	return result
}

// TestSolveGoldenQueries is the pilot's acceptance harness: the seeded
// contradiction is detected with its exact core, the seeded dead transition
// is unreachable, and the live railroad-review queries hold. Skips when z3 is
// not installed (brew install z3).
func TestSolveGoldenQueries(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not installed — brew install z3")
	}
	shared, err := skillgraph.LoadVocabulary("../../skills/vocabulary.cue")
	require.NoError(t, err)
	globalVocab := VocabularyOf(&skillgraph.Skill{Vocabulary: shared})

	t.Run("seeded-contradiction-core", func(t *testing.T) {
		model, vocab := deriveSkill(t, "testdata/seeded", shared)
		require.Empty(t, validateWith(true, model, vocab))
		// The only asserted fact beyond the invariants is interactive mode
		// itself, so an unsat core must pull in BOTH seeded rules — the
		// exact-pair check.
		query := Query{Name: "seeded", Assume: nil, Check: "mode = interactive", Expect: "unsat"}
		doc, err := EmitQuery(model, query)
		result := solved(t, doc, err, model)
		assert.Equal(t, VerdictUnsat, result.Verdict)
		assert.Contains(t, result.Core, "SKILL-SEEDED-RULES-001")
		assert.Contains(t, result.Core, "SKILL-SEEDED-RULES-002")
	})

	t.Run("seeded-dead-transition", func(t *testing.T) {
		model, _ := deriveSkill(t, "testdata/seeded", shared)
		doc, err := EmitReachability(model, "SKILL-SEEDED-PROC-001", 3, nil)
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)
	})

	t.Run("causal-dead-step", func(t *testing.T) {
		model, vocab := deriveSkill(t, "testdata/causal", shared)
		require.Empty(t, validateWith(true, model, vocab))
		doc, err := EmitReachability(model, "SKILL-CAUSAL-PROC-001", 4, nil)
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)

		// The sibling without a pred stays reachable — the deadness is causal,
		// not structural.
		doc, err = EmitReachability(model, "SKILL-CAUSAL-PROC-002", 4, nil)
		assert.Equal(t, VerdictSat, solved(t, doc, err, model).Verdict)
	})

	t.Run("negative-assume-blocks", func(t *testing.T) {
		// armed is granted by no post, so it is rigid: assuming it false
		// kills the guarded step for the whole run; assuming it true frees it.
		body := "## Run\n\n" +
			"`{pred: armed}`\\\n" +
			"**SKILL-GUARDED-PROC-001** `[step]` **Guarded step**\n\n" +
			"RUN the guarded step.\n"
		model := deriveInline(t, map[string]*skillgraph.Atom{"armed": {Sort: "input", Source: "environment"}}, body, "guarded", shared)

		doc, err := EmitReachability(model, "SKILL-GUARDED-PROC-001", 3, []string{"not armed"})
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)

		doc, err = EmitReachability(model, "SKILL-GUARDED-PROC-001", 3, []string{"armed"})
		assert.Equal(t, VerdictSat, solved(t, doc, err, model).Verdict)
	})

	t.Run("negated-stop-pred-blocks", func(t *testing.T) {
		// ready is non-granted, so `not ready` holds rigidly under the
		// assumption and the mandatory stop blocks every later firing.
		body := "## Run\n\n" +
			"`{pred: not ready}`\\\n" +
			"**SKILL-GATED-PROC-001** `[stop]` **Not ready**\n\n" +
			"If not ready ⇒ STOP: report.\n\n" +
			"**SKILL-GATED-PROC-002** `[step]` **Do work**\n\n" +
			"RUN the work.\n"
		model := deriveInline(t, map[string]*skillgraph.Atom{"ready": {Sort: "input", Source: "environment"}}, body, "gated", shared)

		doc, err := EmitReachability(model, "SKILL-GATED-PROC-002", 3, []string{"not ready"})
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)

		doc, err = EmitReachability(model, "SKILL-GATED-PROC-002", 3, []string{"ready"})
		assert.Equal(t, VerdictSat, solved(t, doc, err, model).Verdict)
	})

	t.Run("boundary-post-honored", func(t *testing.T) {
		// The step's post violates the invariant, so it may not fire at any
		// step — the boundary firing included, whose post lands at bound+1.
		body := "## Run\n\n" +
			"`{pred: not done}`\\\n" +
			"**SKILL-BOUNDED-EXEC-001** `[rule]` **Never done**\n\n" +
			"Run MUST NOT reach done.\n\n" +
			"`{post: done}`\\\n" +
			"**SKILL-BOUNDED-PROC-001** `[step]` **Finish**\n\n" +
			"FINISH the run.\n"
		model := deriveInline(t, map[string]*skillgraph.Atom{"done": {Sort: "event"}}, body, "bounded", shared)

		doc, err := EmitReachability(model, "SKILL-BOUNDED-PROC-001", 1, nil)
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)
	})

	t.Run("route-pred-gates-transitions", func(t *testing.T) {
		body := "## Dispatch\n\n" +
			"**SKILL-ROUTED-PROC-001** `[step]` **Resolve base**\n\n" +
			"RESOLVE the base.\n\n" +
			"## Nightly route\n" +
			"`{section: route, pred: mode = nightly, between: PROC-001..end}`\n\n" +
			"**SKILL-ROUTED-NIGHT-001** `[step]` **Route it**\n\n" +
			"ROUTE the run.\n"
		model := deriveInline(t, nil, body, "routed", shared)
		require.Empty(t, validateWith(true, model, globalVocab))

		// Under interactive mode the route transition is dead; the plain
		// phase step stays reachable.
		doc, err := EmitReachability(model, "SKILL-ROUTED-NIGHT-001", 2, []string{"mode = interactive"})
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)

		doc, err = EmitReachability(model, "SKILL-ROUTED-PROC-001", 2, []string{"mode = interactive"})
		assert.Equal(t, VerdictSat, solved(t, doc, err, model).Verdict)
	})

	t.Run("railroad-red-base-never-approve", func(t *testing.T) {
		model, vocab := deriveSkill(t, "../../skills/quality/railroad-review", shared)
		require.Empty(t, validateWith(true, model, vocab))
		query := Query{Name: "red-base-never-approve", Assume: []string{"base_build_red"}, Check: "recommendation = APPROVE", Expect: "unsat"}
		doc, err := EmitQuery(model, query)
		result := solved(t, doc, err, model)
		assert.Equal(t, VerdictUnsat, result.Verdict)
		assert.Contains(t, result.Core, "SKILL-RAILROADREVIEW-OUT-009")
	})

	t.Run("railroad-nightly-skips-human-gate", func(t *testing.T) {
		model, _ := deriveSkill(t, "../../skills/quality/railroad-review", shared)
		assume := []string{"mode = nightly"}
		bound, err := CompleteBound([]*Expr{mustParse(t, assume[0])}, model)
		require.NoError(t, err)

		doc, err := EmitReachability(model, "SKILL-RAILROADREVIEW-MODES-001", bound, assume)
		assert.Equal(t, VerdictUnsat, solved(t, doc, err, model).Verdict)
	})

	t.Run("railroad-second-void-round-stops", func(t *testing.T) {
		model, _ := deriveSkill(t, "../../skills/quality/railroad-review", shared)
		assume := []string{"void_rounds >= 2"}
		bound, err := CompleteBound([]*Expr{mustParse(t, assume[0])}, model)
		require.NoError(t, err)

		doc, err := EmitReachability(model, "SKILL-RAILROADREVIEW-STATION-002b", bound, assume)
		assert.Equal(t, VerdictSat, solved(t, doc, err, model).Verdict)
	})
}
