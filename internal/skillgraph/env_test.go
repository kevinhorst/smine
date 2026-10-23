package skillgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// provideEnvSkill is the complete fixture declaring env keys; FLOW-002's statement reads $SKILL_DEMO_PLAN_SECTIONS.
func provideEnvSkill(env map[string]*Variable) *Skill {
	skill := provideCompleteSkill()
	bindEnv(skill, env)
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_PLAN_SECTIONS plan-sections plan.md`."
	return skill
}

func TestCheckEnv(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	sections := map[string]*Variable{"PLAN_SECTIONS": {Interface: "plan-sections@1"}}
	tests := make([]*testCase, 0)

	// declared-and-referenced
	tests = append(tests, &testCase{
		_id:       "declared-and-referenced",
		_expected: nil,

		skill: provideEnvSkill(sections),
	})

	// referenced-undeclared
	tests = append(tests, &testCase{
		_id:       "referenced-undeclared",
		_expected: []string{"env: SKILL_DEMO_PLAN_SECTIONS is referenced but no env entry declares it"},

		skill: provideEnvSkill(nil),
	})

	// declared-unread
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}})
	tests = append(tests, &testCase{
		_id:       "declared-unread",
		_expected: []string{"env: SKILL_DEMO_CHECK: no text reads it — delete the declaration"},

		skill: skill,
	})

	// read-in-allowed-tools
	skill = provideCompleteSkill()
	bindEnv(skill, sections)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "allowed-tools", Value: " Bash($SKILL_DEMO_PLAN_SECTIONS plan-sections *)"})
	tests = append(tests, &testCase{
		_id:       "read-in-allowed-tools",
		_expected: nil,

		skill: skill,
	})

	// read-in-enforce-pattern
	skill = provideCompleteSkill()
	bindEnv(skill, sections)
	skill.Enforce = &Enforce{Anchors: map[string]*Anchor{
		"sections": {Input: []InputMatch{{Field: "command", Pattern: `^\$SKILL_DEMO_PLAN_SECTIONS plan-sections`}}, Tool: "^Bash$"},
	}}
	tests = append(tests, &testCase{
		_id:       "read-in-enforce-pattern",
		_expected: nil,

		skill: skill,
	})

	// optional-read-in-preamble
	skill = provideEnvSkill(sections)
	skill.Preamble = "Runs `$SKILL_DEMO_PLAN_SECTIONS plan-sections` before anything else."
	tests = append(tests, &testCase{
		_id:       "optional-read-in-preamble",
		_expected: []string{"env: SKILL_DEMO_PLAN_SECTIONS is optional but read in the preamble, frontmatter, prose, args, routing or model, where an unset value cannot be dropped — mark it required or move the reference into an entry"},

		skill: skill,
	})

	// required-read-in-preamble
	skill = provideEnvSkill(map[string]*Variable{"PLAN_SECTIONS": {Interface: "plan-sections@1", Required: true}})
	skill.Preamble = "Runs `$SKILL_DEMO_PLAN_SECTIONS plan-sections` before anything else."
	tests = append(tests, &testCase{
		_id:       "required-read-in-preamble",
		_expected: nil,

		skill: skill,
	})

	// optional-read-in-check-reason
	skill = provideEnvSkill(sections)
	skill.Enforce = &Enforce{
		Anchors: map[string]*Anchor{"edit": {Tool: "^Edit$"}},
		Checks:  map[string]*EnforceCheck{"no_edit": {Deny: "edit", Kind: "phase", Reason: "run `$SKILL_DEMO_PLAN_SECTIONS plan-sections` first", Recovery: "run it"}},
	}
	tests = append(tests, &testCase{
		_id:       "optional-read-in-check-reason",
		_expected: nil,

		skill: skill,
	})

	// unknown-interface
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"TEST": {Interface: "test@9"}})
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_TEST`."
	tests = append(tests, &testCase{
		_id:       "unknown-interface",
		_expected: []string{"env: TEST: interface test@9 is not in interfaces.cue"},

		skill: skill,
	})

	// unknown-interface-without-catalog
	skill = provideCompleteSkill()
	skill.Env = map[string]*Variable{"TEST": {Interface: "test@1"}}
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_TEST`."
	tests = append(tests, &testCase{
		_id:       "unknown-interface-without-catalog",
		_expected: []string{"env: TEST: interface test@1 is not in interfaces.cue"},

		skill: skill,
	})

	// wrong-key
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"RULES": {Interface: "plan-sections@1"}})
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_RULES plan-sections plan.md`."
	tests = append(tests, &testCase{
		_id:       "wrong-key",
		_expected: []string{"env: RULES: key must be PLAN_SECTIONS for plan-sections@1"},

		skill: skill,
	})

	// program-followed-by-wrong-head
	skill = provideEnvSkill(sections)
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_PLAN_SECTIONS plan-display plan.md`."
	tests = append(tests, &testCase{
		_id:       "program-followed-by-wrong-head",
		_expected: []string{`env: $SKILL_DEMO_PLAN_SECTIONS: followed by "plan-display" — plan-sections@1 expects "plan-sections"`},

		skill: skill,
	})

	// program-head-closed-by-parenthesis-passes
	skill = provideEnvSkill(sections)
	flowSection(skill).Blocks[1].Entry.Statement = "GRANT `Bash($SKILL_DEMO_PLAN_SECTIONS plan-sections)`."
	tests = append(tests, &testCase{
		_id:       "program-head-closed-by-parenthesis-passes",
		_expected: nil,

		skill: skill,
	})

	// program-passed-as-value-passes
	skill = provideEnvSkill(sections)
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `x --tool '$SKILL_DEMO_PLAN_SECTIONS'`."
	tests = append(tests, &testCase{
		_id:       "program-passed-as-value-passes",
		_expected: nil,

		skill: skill,
	})

	// program-head-prefix-of-a-longer-word
	skill = provideEnvSkill(sections)
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_PLAN_SECTIONS plan-sectionsx plan.md`."
	tests = append(tests, &testCase{
		_id:       "program-head-prefix-of-a-longer-word",
		_expected: []string{`env: $SKILL_DEMO_PLAN_SECTIONS: followed by "plan-sectionsx" — plan-sections@1 expects "plan-sections"`},

		skill: skill,
	})

	// command-followed-by-argument
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"BUILD": {Interface: "build@1"}})
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_BUILD ./cmd`."
	tests = append(tests, &testCase{
		_id:       "command-followed-by-argument",
		_expected: []string{"env: $SKILL_DEMO_BUILD: followed by \"./cmd`.\" — a command value takes no arguments"},

		skill: skill,
	})

	// command-followed-by-redirect-passes
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"BUILD": {Interface: "build@1"}})
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_BUILD > build.log 2>&1`."
	tests = append(tests, &testCase{
		_id:       "command-followed-by-redirect-passes",
		_expected: nil,

		skill: skill,
	})

	// path-followed-by-anything-passes
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"PLANS_ARCHIVE": {Interface: "plans-archive@1"}})
	flowSection(skill).Blocks[1].Entry.Statement = "READ $SKILL_DEMO_PLANS_ARCHIVE/stores then anything."
	tests = append(tests, &testCase{
		_id:       "path-followed-by-anything-passes",
		_expected: nil,

		skill: skill,
	})

	// repo-scoped-in-preamble-refused
	check := map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}}
	repoScopedRefused := "env: SKILL_DEMO_CHECK is repo-scoped and read in the preamble, frontmatter, prose, args, routing, model or enforce block, where no placeholder can stand — move the reference into an entry"
	skill = provideCompleteSkill()
	bindEnv(skill, check)
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_CHECK`."
	skill.Preamble = "Runs `$SKILL_DEMO_CHECK` before anything else."
	tests = append(tests, &testCase{
		_id:       "repo-scoped-in-preamble-refused",
		_expected: []string{repoScopedRefused},

		skill: skill,
	})

	// repo-scoped-in-routing-refused
	skill = provideCompleteSkill()
	bindEnv(skill, check)
	skill.Routing().Preconditions = []string{"`$SKILL_DEMO_CHECK` is green"}
	tests = append(tests, &testCase{
		_id:       "repo-scoped-in-routing-refused",
		_expected: []string{repoScopedRefused},

		skill: skill,
	})

	// repo-scoped-in-enforce-refused
	skill = provideCompleteSkill()
	bindEnv(skill, check)
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_CHECK`."
	skill.Enforce = &Enforce{
		Anchors: map[string]*Anchor{"edit": {Tool: "^Edit$"}},
		Checks:  map[string]*EnforceCheck{"no_edit": {Deny: "edit", Kind: "phase", Reason: "run `$SKILL_DEMO_CHECK` first", Recovery: "run it"}},
	}
	tests = append(tests, &testCase{
		_id:       "repo-scoped-in-enforce-refused",
		_expected: []string{repoScopedRefused},

		skill: skill,
	})

	// repo-scoped-in-entry-passes
	skill = provideCompleteSkill()
	bindEnv(skill, check)
	flowSection(skill).Blocks[1].Entry.Items[0].Text = "check: `$SKILL_DEMO_CHECK`."
	tests = append(tests, &testCase{
		_id:       "repo-scoped-in-entry-passes",
		_expected: nil,

		skill: skill,
	})

	// repo-scoped-in-grant-passes
	skill = provideCompleteSkill()
	bindEnv(skill, check)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_CHECK)"})
	tests = append(tests, &testCase{
		_id:       "repo-scoped-in-grant-passes",
		_expected: nil,

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkEnv(test.skill))
		})
	}
}

func TestGuardRoundTrip_Env(t *testing.T) {
	env := map[string]*Variable{
		"BUILD":         {Interface: "build@1"},
		"PLAN_SECTIONS": {Interface: "plan-sections@1", Required: true},
	}
	skill := provideEnvSkill(env)
	encoded, err := EncodeSkill(skill)
	require.NoError(t, err)

	assert.NoError(t, guardRoundTrip(encoded, skill))
	assert.Contains(t, string(encoded), "env: {")
}

func TestUnconfiguredVariables(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		deferred map[string]bool
		skill    *Skill
		values   map[string]string
	}

	env := map[string]*Variable{
		"BUILD":         {Interface: "build@1"},
		"CHECK":         {Interface: "check@1", Required: true},
		"PLAN_SECTIONS": {Interface: "plan-sections@1", Required: true},
	}
	tests := make([]*testCase, 0)

	// required-unset-listed-sorted
	tests = append(tests, &testCase{
		_id:       "required-unset-listed-sorted",
		_expected: []string{"SKILL_DEMO_CHECK", "SKILL_DEMO_PLAN_SECTIONS"},

		deferred: nil,
		skill:    provideEnvSkill(env),
		values:   nil,
	})

	// one-required-unset-listed
	tests = append(tests, &testCase{
		_id:       "one-required-unset-listed",
		_expected: []string{"SKILL_DEMO_PLAN_SECTIONS"},

		deferred: nil,
		skill:    provideEnvSkill(env),
		values:   map[string]string{"SKILL_DEMO_CHECK": "make audit"},
	})

	// optional-unset-not-listed
	tests = append(tests, &testCase{
		_id:       "optional-unset-not-listed",
		_expected: nil,

		deferred: nil,
		skill:    provideEnvSkill(env),
		values:   map[string]string{"SKILL_DEMO_CHECK": "make audit", "SKILL_DEMO_PLAN_SECTIONS": "smine-rules"},
	})

	// deferred-required-not-listed
	tests = append(tests, &testCase{
		_id:       "deferred-required-not-listed",
		_expected: []string{"SKILL_DEMO_PLAN_SECTIONS"},

		deferred: map[string]bool{"SKILL_DEMO_CHECK": true},
		skill:    provideEnvSkill(env),
		values:   nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, UnconfiguredVariables(test.deferred, test.skill, test.values))
		})
	}
}

func TestVariableKey(t *testing.T) {
	assert.Equal(t, "PLAN_SECTIONS", VariableKey("plan-sections"))
	assert.Equal(t, "TEST", VariableKey("test"))
}

func TestVariableName(t *testing.T) {
	assert.Equal(t, "SKILL_RAILROADREVIEW_LANES", VariableName("railroad-review", "LANES"))
	assert.Equal(t, "SKILL_FDESIGN_PROJECT_PLAN", VariableName("fdesign", "PROJECT_PLAN"))
}

func TestVariableReads(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []VariableRead

		skill *Skill
	}

	lanes := map[string]*Variable{"LANES": {Interface: "lanes@1"}}
	tests := make([]*testCase, 0)

	// entry-and-param
	skill := provideCompleteSkill()
	bindEnv(skill, lanes)
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_LANES lanes`."
	gate := flowSection(skill).Blocks[1].Entry
	gate.Items = append(gate.Items, Item{Label: LabelParam, Marker: "*", Text: "lanes: `$SKILL_DEMO_LANES lanes --round-dir x`."})
	tests = append(tests, &testCase{
		_id: "entry-and-param",
		_expected: []VariableRead{
			{Id: "SKILL-DEMO-FLOW-002", Kind: ReadKindParam},
			{Id: "SKILL-DEMO-FLOW-003", Kind: ReadKindEntry},
		},

		skill: skill,
	})

	// grant
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_LANES lanes *), Read"})
	tests = append(tests, &testCase{
		_id:       "grant",
		_expected: []VariableRead{{Id: "Bash($SKILL_DEMO_LANES lanes *)", Kind: ReadKindGrant}},

		skill: skill,
	})

	// anchor
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	skill.Enforce = &Enforce{Anchors: map[string]*Anchor{
		"lanes": {Input: []InputMatch{{Field: "command", Pattern: "^$SKILL_DEMO_LANES lanes "}}, Tool: "^Bash$"},
		"stage": {Tool: "^Workflow$"},
	}}
	tests = append(tests, &testCase{
		_id:       "anchor",
		_expected: []VariableRead{{Id: "lanes", Kind: ReadKindAnchor}},

		skill: skill,
	})

	// check-reason
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	skill.Enforce = &Enforce{
		Anchors: map[string]*Anchor{"edit": {Tool: "^Edit$"}},
		Checks: map[string]*EnforceCheck{
			"no_edit":  {Deny: "edit", Kind: "phase", Reason: "run `$SKILL_DEMO_LANES lanes` first", Recovery: "run it"},
			"no_write": {Deny: "edit", Kind: "phase", Reason: "write later", Recovery: "wait"},
		},
	}
	tests = append(tests, &testCase{
		_id:       "check-reason",
		_expected: []VariableRead{{Id: "no_edit", Kind: ReadKindCheck}},

		skill: skill,
	})

	// none
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	tests = append(tests, &testCase{
		_id:       "none",
		_expected: nil,

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, VariableReads(test.skill, "SKILL_DEMO_LANES"))
		})
	}
}
