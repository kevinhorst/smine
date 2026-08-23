package acdsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/reach"
)

func TestRulesForFile(t *testing.T) {
	universe := []string{"cmd/a/main.go", "internal/b/b.go", "internal/b/b_test.go"}
	rules := []Rule{
		{Id: "R-INT", Anchor: `^internal/`, AnchorNot: `_test\.go$`, Why: "internal rule"},
		{Id: "R-CMD", Anchor: `^cmd/`, Why: "cmd rule"},
	}

	tests := []struct {
		name string
		path string
		want []string
	}{
		{"file inside anchor", "internal/b/b.go", []string{"R-INT"}},
		{"file excluded by anchor-not", "internal/b/b_test.go", nil},
		{"file outside all anchors", "docs/readme.go", nil},
		{"file matching the other rule", "cmd/a/main.go", []string{"R-CMD"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			governing, err := RulesForFile(rules, universe, test.path)
			require.NoError(t, err)
			var ids []string
			for _, rule := range governing {
				ids = append(ids, rule.Id)
			}
			assert.Equal(t, test.want, ids)
		})
	}
}

func TestRulesForFileAnchorErrorPropagates(t *testing.T) {
	_, err := RulesForFile([]Rule{{Id: "R", Anchor: `(`}}, []string{"a.go"}, "a.go")
	require.Error(t, err)
}

func TestRulesForFileSkipsTaskRules(t *testing.T) {
	universe := []string{"internal/b/b.go"}
	rules := []Rule{
		{Id: "TASK-X-001", Anchor: `^internal/`, Why: "task rule", Lifetime: LifetimeTask},
		{Id: "R-INT", Anchor: `^internal/`, Why: "doctrine rule", Lifetime: LifetimeDoctrine},
	}
	governing, err := RulesForFile(rules, universe, "internal/b/b.go")
	require.NoError(t, err)
	require.Len(t, governing, 1)
	assert.Equal(t, "R-INT", governing[0].Id)
}

func TestRulesForFileSkipsDisabledRules(t *testing.T) {
	universe := []string{"internal/b/b.go"}
	rules := []Rule{
		{Id: "R-OFF", Anchor: `^internal/`, Why: "disabled rule", Reach: reach.None},
		{Id: "R-INT", Anchor: `^internal/`, Why: "doctrine rule"},
	}
	governing, err := RulesForFile(rules, universe, "internal/b/b.go")
	require.NoError(t, err)
	require.Len(t, governing, 1)
	assert.Equal(t, "R-INT", governing[0].Id)
}

func TestRulesForPlannedPathSkipsDisabledRules(t *testing.T) {
	rules := []Rule{{Id: "R-OFF", Anchor: `^internal/`, Why: "disabled rule", Reach: reach.None}}
	governing, err := RulesForPlannedPath(rules, "internal/b/b.go")
	require.NoError(t, err)
	assert.Empty(t, governing)
}

func TestRulesForPlannedPath(t *testing.T) {
	rules := []Rule{
		{Id: "R-INT", Anchor: `^internal/`, AnchorNot: `_test\.go$`, Why: "internal rule"},
		{Id: "R-CMD", Anchor: `^cmd/`, Why: "cmd rule"},
	}

	governing, err := RulesForPlannedPath(rules, "internal/brandnew/controller.go")
	require.NoError(t, err)
	require.Len(t, governing, 1)
	assert.Equal(t, "R-INT", governing[0].Id)

	governing, err = RulesForPlannedPath(rules, "internal/brandnew/controller_test.go")
	require.NoError(t, err)
	assert.Empty(t, governing)

	governing, err = RulesForPlannedPath(rules, "docs/readme.md")
	require.NoError(t, err)
	assert.Empty(t, governing)
}

func TestRulesForPlannedPathAnchorErrorPropagates(t *testing.T) {
	_, err := RulesForPlannedPath([]Rule{{Id: "R", Anchor: `(`}}, "a.go")
	require.Error(t, err)
}

func TestDeliveredFiltersGateOnlyRules(t *testing.T) {
	rules := []Rule{
		{Id: "R-ON", Projected: true},
		{Id: "R-OFF", Projected: false},
		{Id: "R-ON-2", Projected: true},
	}
	delivered := Delivered(rules)
	var ids []string
	for _, rule := range delivered {
		ids = append(ids, rule.Id)
	}
	assert.Equal(t, []string{"R-ON", "R-ON-2"}, ids)
}

func TestDeliveredAllGateOnlyIsEmpty(t *testing.T) {
	delivered := Delivered([]Rule{{Id: "R-OFF", Projected: false}})
	assert.Empty(t, delivered)
}
