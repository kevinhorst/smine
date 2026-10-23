package skillgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSection_Anchor(t *testing.T) {
	type testCase struct {
		_id         string
		_between    []string
		_replaces   []string
		_shouldPass bool

		section *Section
	}

	tests := make([]*testCase, 0)

	// short-ids-resolved
	tests = append(tests, &testCase{_id: "short-ids-resolved", _replaces: []string{"SKILL-DEMO-FLOW-001", "SKILL-DEMO-FLOW-003"}, _shouldPass: true, section: &Section{Replaces: "FLOW-001 + FLOW-003"}})

	// range-kept
	tests = append(tests, &testCase{_id: "range-kept", _replaces: []string{"SKILL-DEMO-FLOW-001..SKILL-DEMO-FLOW-003"}, _shouldPass: true, section: &Section{Replaces: "FLOW-001..FLOW-003"}})

	// start-end-kept
	tests = append(tests, &testCase{_id: "start-end-kept", _between: []string{"start", "end"}, _shouldPass: true, section: &Section{Between: "start..end"}})

	// letter-suffix
	tests = append(tests, &testCase{_id: "letter-suffix", _between: []string{"SKILL-DEMO-MODES-014b", "end"}, _shouldPass: true, section: &Section{Between: "MODES-014b..end"}})

	// no-clause-nils
	tests = append(tests, &testCase{_id: "no-clause-nils", _shouldPass: true, section: &Section{}})

	// fail-malformed-replaces-item
	tests = append(tests, &testCase{_id: "fail-malformed-replaces-item", _shouldPass: false, section: &Section{Replaces: "FLOW-001 + nope"}})

	// fail-malformed-between
	tests = append(tests, &testCase{_id: "fail-malformed-between", _shouldPass: false, section: &Section{Between: "end..start"}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			replaces, between, err := test.section.Anchor("demo")
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._replaces, replaces)
			assert.Equal(t, test._between, between)
		})
	}
}

func TestSkillIdName(t *testing.T) {
	assert.Equal(t, "RAILROADREVIEW", SkillIdName("railroad-review"))
	assert.Equal(t, "QQ", SkillIdName("qq"))
}
