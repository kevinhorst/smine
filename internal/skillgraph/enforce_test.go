package skillgraph

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// demoEnforce is a valid declaration: every anchor referenced, the
// provenance capture declared.
func demoEnforce() *Enforce {
	return &Enforce{
		Anchors: map[string]*Anchor{
			"produced": {Capture: []Capture{{Name: "value", Pointer: "/value"}}, Input: []InputMatch{{Field: "command", Pattern: "^demo-value "}}, Tool: "^Bash$"},
			"stage":    {Tool: "^Workflow$"},
		},
		Checks: map[string]*EnforceCheck{
			"value_provenance": {Arg: "/args/value", Capture: "value", From: "produced", Kind: CheckProvenance, On: "stage", Reason: "r", Recovery: "x"},
			"producer_first":   {After: "produced", Kind: CheckRequires, On: "stage", Reason: "r", Recovery: "x"},
		},
	}
}

func TestCheckEnforce(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// valid
	tests = append(tests, &testCase{
		_id:       "valid",
		_expected: nil,

		skill: &Skill{Enforce: demoEnforce()},
	})

	// unknown-anchor
	enforce := demoEnforce()
	enforce.Checks["producer_first"].After = "missing"
	tests = append(tests, &testCase{
		_id:       "unknown-anchor",
		_expected: []string{"enforce: check producer_first: unknown anchor missing"},

		skill: &Skill{Enforce: enforce},
	})

	// unreferenced-anchor
	enforce = demoEnforce()
	enforce.Anchors["idle"] = &Anchor{Tool: "^Read$"}
	tests = append(tests, &testCase{
		_id:       "unreferenced-anchor",
		_expected: []string{"enforce: anchor idle: referenced by no check or judgment"},

		skill: &Skill{Enforce: enforce},
	})

	// capture-missing
	enforce = demoEnforce()
	enforce.Checks["value_provenance"].Capture = "other"
	tests = append(tests, &testCase{
		_id: "capture-missing",
		_expected: []string{
			"enforce: check value_provenance: anchor produced captures no other",
			"enforce: anchor produced capture value: read by no check, judgment or atom",
		},

		skill: &Skill{Enforce: enforce},
	})

	// bad-regex
	enforce = demoEnforce()
	enforce.Anchors["stage"].Tool = "("
	tests = append(tests, &testCase{
		_id:       "bad-regex",
		_expected: []string{"enforce: anchor stage: tool \"(\" does not compile: error parsing regexp: missing closing ): `(`"},

		skill: &Skill{Enforce: enforce},
	})

	// too-many-anchors
	enforce = demoEnforce()
	for index := range 15 {
		id := fmt.Sprintf("extra_%d", index)
		enforce.Anchors[id] = &Anchor{Tool: "^Read$"}
		enforce.Judgment = map[string]*Judgment{}
		enforce.Checks["phase_"+id] = &EnforceCheck{Deny: id, From: "stage", Kind: CheckPhase, Reason: "r", Recovery: "x", Until: "stage"}
	}
	tests = append(tests, &testCase{
		_id:       "too-many-anchors",
		_expected: []string{"enforce: 17 anchors, at most 16"},

		skill: &Skill{Enforce: enforce},
	})

	// fail-capture-unread
	enforce = demoEnforce()
	enforce.Anchors["stage"].Capture = []Capture{{Name: "idle", Pointer: "/idle"}}
	tests = append(tests, &testCase{
		_id:       "fail-capture-unread",
		_expected: []string{"enforce: anchor stage capture idle: read by no check, judgment or atom"},

		skill: &Skill{Enforce: enforce},
	})

	// pass-capture-read-by-atom
	enforce = demoEnforce()
	enforce.Anchors["stage"].Capture = []Capture{{Name: "count", Pointer: "/count"}}
	tests = append(tests, &testCase{
		_id:       "pass-capture-read-by-atom",
		_expected: nil,

		skill: &Skill{Atoms: map[string]*Atom{"rounds": {Kind: KindAtomInt, Sort: "count", Source: "runtime: stage.count"}}, Enforce: enforce},
	})

	// pass-capture-read-by-judgment
	enforce = demoEnforce()
	enforce.Anchors["stage"].Capture = []Capture{{Name: "verdict", Pointer: "/verdict"}}
	enforce.Judgment = map[string]*Judgment{"fits": {Anchor: "stage", Evidence: []string{"verdict", "arg:/command"}, Question: "q", Send: "s"}}
	tests = append(tests, &testCase{
		_id:       "pass-capture-read-by-judgment",
		_expected: nil,

		skill: &Skill{Enforce: enforce},
	})

	// pass-capture-read-by-provenance
	tests = append(tests, &testCase{
		_id:       "pass-capture-read-by-provenance",
		_expected: nil,

		skill: &Skill{Enforce: demoEnforce()},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := checkEnforce(test.skill)

			assert.Equal(t, test._expected, violations)
		})
	}
}

func TestEnforceCheck_AnchorRefs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []AnchorRef

		check *EnforceCheck
	}

	tests := make([]*testCase, 0)

	// phase
	tests = append(tests, &testCase{
		_id:       "phase",
		_expected: []AnchorRef{{Anchor: "hunk", Role: anchorRoleDeny}, {Anchor: "start", Role: anchorRoleFrom}, {Anchor: "end", Role: anchorRoleUntil}},

		check: &EnforceCheck{Deny: "hunk", From: "start", Kind: CheckPhase, Until: "end"},
	})

	// provenance
	tests = append(tests, &testCase{
		_id:       "provenance",
		_expected: []AnchorRef{{Anchor: "stage", Role: anchorRoleOn}, {Anchor: "produced", Role: anchorRoleFrom}},

		check: &EnforceCheck{From: "produced", Kind: CheckProvenance, On: "stage"},
	})

	// requires-without-since
	tests = append(tests, &testCase{
		_id:       "requires-without-since",
		_expected: []AnchorRef{{Anchor: "stage", Role: anchorRoleOn}, {Anchor: "produced", Role: anchorRoleAfter}},

		check: &EnforceCheck{After: "produced", Kind: CheckRequires, On: "stage"},
	})

	// requires-with-since
	tests = append(tests, &testCase{
		_id:       "requires-with-since",
		_expected: []AnchorRef{{Anchor: "stage", Role: anchorRoleOn}, {Anchor: "produced", Role: anchorRoleAfter}, {Anchor: "round", Role: anchorRoleSince}},

		check: &EnforceCheck{After: "produced", Kind: CheckRequires, On: "stage", Since: "round"},
	})

	// stop
	tests = append(tests, &testCase{
		_id:       "stop",
		_expected: []AnchorRef{{Anchor: "closed", Role: anchorRoleRequire}, {Anchor: "stage", Role: anchorRoleAfter}},

		check: &EnforceCheck{After: "stage", Kind: CheckStop, Require: "closed"},
	})

	// unknown-kind
	tests = append(tests, &testCase{
		_id:       "unknown-kind",
		_expected: nil,

		check: &EnforceCheck{After: "stage", Kind: "nonsense", On: "stage"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			refs := test.check.AnchorRefs()

			assert.Equal(t, test._expected, refs)
		})
	}
}

func TestRender_EnforceJson(t *testing.T) {
	skill := &Skill{Enforce: demoEnforce(), Frontmatter: []Field{{Key: "name", Value: "demo"}, {Key: "version", Value: "1.0.0"}}}

	rendered, err := renderEnforce(skill)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(rendered), "{\n  \"anchors\""))
	assert.Contains(t, string(rendered), "\"schema\": 1")
	assert.Contains(t, string(rendered), "\"skill\": \"demo\"")
	assert.Contains(t, string(rendered), "\"version\": \"1.0.0\"")
}
