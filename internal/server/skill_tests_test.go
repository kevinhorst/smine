package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
)

func TestBuildEvalView(t *testing.T) {
	rubric := []evals.RubricRule{
		{Id: "SKILL-A-001", Axis: "self", Phase: "step", Rule: "Rule one."},
		{Id: "SKILL-A-002", Axis: "self", Phase: "gate", Rule: "Rule two."},
		{Id: "CTX-001", Axis: "context", Phase: "step", Rule: "Context rule."},
	}
	scores := []evals.ScoreCell{
		{RuleId: "SKILL-A-001", RunId: "r1", Score: 1, Source: "agent"},
		{RuleId: "SKILL-A-001", RunId: "r2", Score: -1, Source: "probe", Justification: "violated", Evidence: []string{"a", "b"}},
		{RuleId: "SKILL-A-002", RunId: "r1", Score: 0, Source: "agent", Justification: "not shown"},
		{RuleId: "SKILL-A-002", RunId: "r2", Score: 1, Source: "agent"},
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

	// score cells map to class/text; absent cell renders as dash
	selfRows := view.Axes[0].Rows
	require.Len(t, selfRows, 2)
	assert.Equal(t, evalScoreCell{Class: "score-pos", Text: "+1"}, selfRows[0].Cells[0])
	assert.Equal(t, "score-neg", selfRows[0].Cells[1].Class)
	assert.Equal(t, "−1", selfRows[0].Cells[1].Text)
	assert.Equal(t, "violated", selfRows[0].Cells[1].Title)
	contextRow := view.Axes[1].Rows[0]
	assert.Equal(t, "score-absent", contextRow.Cells[1].Class)
	assert.Equal(t, "—", contextRow.Cells[1].Text)

	// non-+1 cells collected as justifications with joined evidence
	require.Len(t, view.Axes[0].Justifications, 2)
	assert.Equal(t, "a; b", view.Axes[0].Justifications[0].Evidence)
	assert.Equal(t, "R2", view.Axes[0].Justifications[0].RunLabel)
	assert.Equal(t, -1, view.Axes[0].Justifications[0].Score)
	assert.Empty(t, view.Axes[1].Justifications)

	// rankings sorted axis asc, pct desc
	require.Len(t, view.Rankings, 3)
	assert.Equal(t, "context", view.Rankings[0].Axis)
	assert.Equal(t, "R1", view.Rankings[1].RunLabel)
	assert.Equal(t, "R2", view.Rankings[2].RunLabel)
}
