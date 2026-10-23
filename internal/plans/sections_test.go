package plans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAgentTier(t *testing.T) {
	for _, name := range []string{"Current state", "Target state", "Baseline (verified)", "Exemplar & reuse", "Test runbook"} {
		assert.Truef(t, IsAgentTier(name), "%s is agent tier", name)
	}
	assert.False(t, IsAgentTier("Scope"))
	assert.False(t, IsAgentTier("Changelog"))
}

func TestIsOrderChecked(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		path string
	}

	tests := make([]*testCase, 0)

	// plan-dir-on-cutoff
	tests = append(tests, &testCase{
		_id:       "plan-dir-on-cutoff",
		_expected: true,

		path: "plans/2026-10-04-x-abc1234/2026-10-04-x-wip-abc1234/design/x.md",
	})

	// plan-dir-after-cutoff
	tests = append(tests, &testCase{
		_id:       "plan-dir-after-cutoff",
		_expected: true,

		path: "plans/2026-10-05-x-abc1234/2026-10-05-x-wip-abc1234/design/x.md",
	})

	// plan-dir-before-cutoff
	tests = append(tests, &testCase{
		_id:       "plan-dir-before-cutoff",
		_expected: false,

		path: "plans/2026-10-03-x-abc1234/2026-10-03-x-wip-abc1234/design/x.md",
	})

	// feature-folder-older-than-plan-dir
	tests = append(tests, &testCase{
		_id:       "feature-folder-older-than-plan-dir",
		_expected: true,

		path: "plans/2026-09-01-x-abc1234/2026-10-04-x-wip-abc1234/design/x.md",
	})

	// old-depth-plan-dir-before-cutoff
	tests = append(tests, &testCase{
		_id:       "old-depth-plan-dir-before-cutoff",
		_expected: false,

		path: "plans/2026-09-01-old-wip-abc1234/design/old.md",
	})

	// done-plan-dir-before-cutoff
	tests = append(tests, &testCase{
		_id:       "done-plan-dir-before-cutoff",
		_expected: false,

		path: "plans/2026-09-01-x-abc1234/2026-09-01-x-done-def5678/design/x.md",
	})

	// absolute-path-before-cutoff
	tests = append(tests, &testCase{
		_id:       "absolute-path-before-cutoff",
		_expected: false,

		path: "/Users/someone/GolandProjects/agent-plans/claude-configs/plans/2026-10-03-x-abc1234/2026-10-03-x-wip-abc1234/design/x.md",
	})

	// session-draft
	tests = append(tests, &testCase{
		_id:       "session-draft",
		_expected: true,

		path: "/Users/someone/.claude/plans/draft-name.md",
	})

	// fixture-path-outside-plans
	tests = append(tests, &testCase{
		_id:       "fixture-path-outside-plans",
		_expected: true,

		path: "acdsl/testdata/ACDSL-PLAN-001/pass/plan.md",
	})

	// empty-path
	tests = append(tests, &testCase{
		_id:       "empty-path",
		_expected: true,

		path: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsOrderChecked(test.path))
		})
	}
}

func TestSplitHeading(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedName   string
		_expectedMarked bool

		text string
	}

	tests := make([]*testCase, 0)

	// marked
	tests = append(tests, &testCase{
		_id:             "marked",
		_expectedName:   "Current state",
		_expectedMarked: true,

		text: "Current state [AGENT-ONLY]",
	})

	// bare
	tests = append(tests, &testCase{
		_id:             "bare",
		_expectedName:   "Scope",
		_expectedMarked: false,

		text: "Scope",
	})

	// no-space
	tests = append(tests, &testCase{
		_id:             "no-space",
		_expectedName:   "Scope[AGENT-ONLY]",
		_expectedMarked: false,

		text: "Scope[AGENT-ONLY]",
	})

	// trailing-space
	tests = append(tests, &testCase{
		_id:             "trailing-space",
		_expectedName:   "Test runbook",
		_expectedMarked: true,

		text: "Test runbook [AGENT-ONLY] ",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			name, marked := SplitHeading(test.text)
			assert.Equal(t, test._expectedName, name)
			assert.Equal(t, test._expectedMarked, marked)
		})
	}
}
