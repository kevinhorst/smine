package skillgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The atom cut leaves preds only on stops, rules and the three flow guards, and no post anywhere.
func TestRailroadReviewPreds(t *testing.T) {
	const dir = "../../skills/quality/railroad-review"
	skill, err := Load(dir)
	require.NoError(t, err)

	want := map[string]string{
		"INTAKE-003":   "bare_invocation",
		"MODES-001":    "mode = interactive and round_gate_pending",
		"MODES-002":    "plan_mode and not mode_switch_tool",
		"MODES-002a":   "plan_mode and mode_switch_tool",
		"MODES-014b":   "budget_low",
		"OUT-009":      "((base_build_red or base_tests_red) => recommendation = REJECT) and ((head_build_red or head_tests_red) => recommendation != APPROVE)",
		"STATION-002b": "void_rounds >= 2",
		"STATION-004a": "review_complete",
		"STATION-005":  "review_complete",
		"STATION-009":  "finding_reconfirmed",
	}
	preds := map[string]string{}
	var posts []string
	for _, ref := range skill.Entries() {
		shortId := strings.TrimPrefix(ref.Entry.Id, "SKILL-RAILROADREVIEW-")
		if ref.Entry.Pred != "" {
			preds[shortId] = ref.Entry.Pred
		}
		if len(ref.Entry.Post) > 0 {
			posts = append(posts, shortId)
		}
	}

	assert.Equal(t, want, preds)
	assert.Empty(t, posts)
}
