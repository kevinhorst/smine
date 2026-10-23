package skillmodel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// queryModel is a model-tier skill with one invariant, an exclusivity pair
// and an int atom — the fixture every RunQuery case asks against.
func queryModel() *Model {
	vocab := &Vocabulary{
		Exclusive: [][]string{{"approved", "rejected"}},
		Sorts: map[string][]Atom{
			"count": {{Id: "budget", Kind: skillgraph.KindAtomInt}},
			"state": {{Id: "approved", Source: "arg"}, {Id: "done", Source: "arg"}, {Id: "rejected", Source: "arg"}},
		},
	}
	model := &Model{
		Invariants: []Invariant{{Id: "SKILL-DEMO-EXEC-001", Expr: "done => approved", Section: "run", Text: "t"}},
		IsStrict:   true,
		Sections:   []Section{{Id: "run", Kind: skillgraph.SectionKindPhase, Title: "Run"}},
		Skill:      "demo",
		Vocabulary: vocab,
	}
	return model
}

func TestRunQuery(t *testing.T) {
	t.Run("annotation-tier", func(t *testing.T) {
		model := queryModel()
		model.IsStrict = false
		_, err := RunQuery(context.Background(), model, "done")
		require.ErrorContains(t, err, "RunQuery: demo is not tier strict")
		assert.ErrorIs(t, err, ErrAnnotationTier)
	})

	t.Run("parse-error", func(t *testing.T) {
		_, err := RunQuery(context.Background(), queryModel(), "done and")
		assert.ErrorContains(t, err, "RunQuery:")
	})

	t.Run("unresolved-atom", func(t *testing.T) {
		_, err := RunQuery(context.Background(), queryModel(), "warp_drive")
		assert.ErrorContains(t, err, `query: unresolved atom "warp_drive"`)
	})

	t.Run("holds", func(t *testing.T) {
		skipWithoutZ3(t)
		result, err := RunQuery(context.Background(), queryModel(), "done => not rejected")
		require.NoError(t, err)
		assert.Equal(t, VerdictUnsat, result.Verdict)
		assert.Equal(t, []string{"SKILL-DEMO-EXEC-001"}, result.Core)
	})

	t.Run("counterexample", func(t *testing.T) {
		skipWithoutZ3(t)
		result, err := RunQuery(context.Background(), queryModel(), "budget > 3")
		require.NoError(t, err)
		assert.Equal(t, VerdictSat, result.Verdict)
		assert.Contains(t, result.Scenario, "budget=0")
	})

	t.Run("soft-left-out", func(t *testing.T) {
		skipWithoutZ3(t)
		model := queryModel()
		model.Soft = []Invariant{{Id: "SKILL-DEMO-QUAL-001", Expr: "budget > 5", Section: "run", Text: "t"}}
		result, err := RunQuery(context.Background(), model, "budget > 3")
		require.NoError(t, err)
		assert.Equal(t, VerdictSat, result.Verdict)
	})
}
