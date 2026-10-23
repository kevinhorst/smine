package budget

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// measuredWindowEquivalents is the measured Team Premium window weighted at
// Opus 5.5 ratios, spelled out from docs/token-usage.html.
const measuredWindowEquivalents = 17216269*0.4 + 390238659*0.01 + 4834*0.2 + 2137757

// max20xSessionEquivalents is the session that read 28% of a Max 20x window,
// weighted at Opus 5.5 ratios, spelled out from docs/token-usage.html.
const max20xSessionEquivalents = 15156419*0.4 + 353337991*0.01 + 4308*0.2 + 1876747

func TestNewPlan(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   Plan
		_expectedOk bool

		value string
	}

	tests := make([]*testCase, 0)

	// team-premium
	tests = append(tests, &testCase{
		_id:         "team-premium",
		_expected:   PlanTeamPremium,
		_expectedOk: true,

		value: "team-premium",
	})

	// max-20x
	tests = append(tests, &testCase{
		_id:         "max-20x",
		_expected:   PlanMax20x,
		_expectedOk: true,

		value: "max-20x",
	})

	// empty
	tests = append(tests, &testCase{
		_id:         "empty",
		_expected:   Plan(""),
		_expectedOk: false,

		value: "",
	})

	// dropped-plan-unknown
	tests = append(tests, &testCase{
		_id:         "dropped-plan-unknown",
		_expected:   Plan("pro"),
		_expectedOk: false,

		value: "pro",
	})

	// label-is-not-a-name
	tests = append(tests, &testCase{
		_id:         "label-is-not-a-name",
		_expected:   Plan("Team Premium"),
		_expectedOk: false,

		value: "Team Premium",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			plan, ok := NewPlan(test.value)
			assert.Equal(t, test._expectedOk, ok)
			assert.Equal(t, test._expected, plan)
		})
	}
}

func TestPlans(t *testing.T) {
	assert.Equal(t, []Plan{PlanTeamPremium, PlanMax20x}, Plans)
}

func TestPlan_Label(t *testing.T) {
	assert.Equal(t, "Team Premium", PlanTeamPremium.Label())
	assert.Equal(t, "Max 20x", PlanMax20x.Label())
	assert.Empty(t, Plan("enterprise").Label())
}

func TestPlan_WeeklyShare(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   float64
		_expectedOk bool

		plan Plan
	}

	tests := make([]*testCase, 0)

	// team-premium-16-points
	tests = append(tests, &testCase{
		_id:         "team-premium-16-points",
		_expected:   0.16,
		_expectedOk: true,

		plan: PlanTeamPremium,
	})

	// max-20x-10-of-28-points
	tests = append(tests, &testCase{
		_id:         "max-20x-10-of-28-points",
		_expected:   0.10 / 0.28,
		_expectedOk: true,

		plan: PlanMax20x,
	})

	// unknown-plan
	tests = append(tests, &testCase{
		_id:         "unknown-plan",
		_expected:   0,
		_expectedOk: false,

		plan: Plan("enterprise"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			share, ok := test.plan.WeeklyShare()
			assert.Equal(t, test._expectedOk, ok)
			assert.InDelta(t, test._expected, share, 1e-12)
		})
	}
}

func TestPlan_WindowCapacity(t *testing.T) {
	type testCase struct {
		_id       string
		_expected float64

		plan Plan
	}

	tests := make([]*testCase, 0)

	// team-premium-is-the-measured-window
	tests = append(tests, &testCase{
		_id:       "team-premium-is-the-measured-window",
		_expected: measuredWindowEquivalents,

		plan: PlanTeamPremium,
	})

	// max-20x-measured-session-over-its-share
	tests = append(tests, &testCase{
		_id:       "max-20x-measured-session-over-its-share",
		_expected: max20xSessionEquivalents / 0.28,

		plan: PlanMax20x,
	})

	// unknown-plan-zero
	tests = append(tests, &testCase{
		_id:       "unknown-plan-zero",
		_expected: 0,

		plan: Plan("enterprise"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.InDelta(t, test._expected, test.plan.WindowCapacity(), 1e-6)
		})
	}
}

func TestPlanUsage_Scale(t *testing.T) {
	usage := PlanUsage{All: 10, Fable: 4}

	assert.Equal(t, PlanUsage{All: 2.5, Fable: 1}, usage.Scale(0.25))
	assert.Equal(t, PlanUsage{}, usage.Scale(0))
	assert.Equal(t, PlanUsage{All: 10, Fable: 4}, usage)
}

func TestPlanUsage_Add(t *testing.T) {
	usage := PlanUsage{All: 10, Fable: 4}
	other := PlanUsage{All: 2.5, Fable: 0.5}

	sum := usage.Add(other)

	assert.Equal(t, PlanUsage{All: 12.5, Fable: 4.5}, sum)
	assert.Equal(t, PlanUsage{All: 10, Fable: 4}, usage)
}

func TestWeightsFor(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   classWeights
		_expectedOk bool

		model string
	}

	tests := make([]*testCase, 0)

	// opus-exact
	tests = append(tests, &testCase{
		_id:         "opus-exact",
		_expected:   classWeights{cacheCreation: 0.4, cacheRead: 0.01, input: 0.2, output: 1},
		_expectedOk: true,

		model: "claude-opus-5-5",
	})

	// fable-exact
	tests = append(tests, &testCase{
		_id:         "fable-exact",
		_expected:   classWeights{cacheCreation: 1, cacheRead: 0.0125, input: 0.5, output: 2.5},
		_expectedOk: true,

		model: "claude-fable-5-1",
	})

	// sonnet-exact
	tests = append(tests, &testCase{
		_id:         "sonnet-exact",
		_expected:   classWeights{cacheCreation: 0.2, cacheRead: 0.01, input: 0.1, output: 0.5},
		_expectedOk: true,

		model: "claude-sonnet-5-5",
	})

	// haiku-dated
	tests = append(tests, &testCase{
		_id:         "haiku-dated",
		_expected:   classWeights{cacheCreation: 0.1, cacheRead: 0.005, input: 0.05, output: 0.25},
		_expectedOk: true,

		model: "claude-haiku-4-5-20251001",
	})

	// unknown-model
	tests = append(tests, &testCase{
		_id:         "unknown-model",
		_expected:   classWeights{},
		_expectedOk: false,

		model: "gpt-5",
	})

	// empty-model
	tests = append(tests, &testCase{
		_id:         "empty-model",
		_expected:   classWeights{},
		_expectedOk: false,

		model: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			weights, ok := weightsFor(test.model)
			assert.Equal(t, test._expectedOk, ok)
			assert.Equal(t, test._expected, weights)
		})
	}
}
