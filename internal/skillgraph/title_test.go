package skillgraph

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessiontitle"
)

// demoTitle is a valid declaration: one state presented by the fixture's
// FLOW-002 step.
func demoTitle() *sessiontitle.Declaration {
	return &sessiontitle.Declaration{
		States: map[string]*sessiontitle.DeclaredState{
			"DESIGN-READY": {Entry: "SKILL-DEMO-FLOW-002", Meaning: "the design waits for approval"},
		},
	}
}

// provideTitledSkill is the complete fixture carrying demoTitle, its gate
// step running the set command through the title-set variable from a param.
func provideTitledSkill() *Skill {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"TITLE_SET": {Interface: "title-set@1"}})
	skill.Title = demoTitle()
	gate := flowSection(skill).Blocks[1].Entry
	gate.Items = append(gate.Items, Item{Label: LabelParam, Marker: "*", Text: "RUN `$SKILL_DEMO_TITLE_SET title set DESIGN-READY` before the gate message."})
	return skill
}

func TestCheckTitle(t *testing.T) {
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

		skill: provideTitledSkill(),
	})

	// valid-command-in-statement
	skill := provideTitledSkill()
	gate := flowSection(skill).Blocks[1].Entry
	gate.Items = nil
	gate.Statement = "RUN `$SKILL_DEMO_TITLE_SET title set DESIGN-READY`, then PRESENT the gate."
	tests = append(tests, &testCase{
		_id:       "valid-command-in-statement",
		_expected: nil,

		skill: skill,
	})

	// literal-command-refused
	skill = provideTitledSkill()
	flowSection(skill).Blocks[1].Entry.Items[1].Text = "RUN `smine-rules title set DESIGN-READY` before the gate message."
	tests = append(tests, &testCase{
		_id:       "literal-command-refused",
		_expected: []string{"title: state DESIGN-READY: entry SKILL-DEMO-FLOW-002 never runs `$SKILL_DEMO_TITLE_SET title set DESIGN-READY`"},

		skill: skill,
	})

	// no-title-set-variable
	skill = provideTitledSkill()
	skill.Env = map[string]*Variable{"LANES": {Interface: "lanes@1"}}
	tests = append(tests, &testCase{
		_id:       "no-title-set-variable",
		_expected: []string{"title: state DESIGN-READY: entry SKILL-DEMO-FLOW-002 has no variable to run `title set DESIGN-READY` through — declare an env variable bound to title-set@1"},

		skill: skill,
	})

	// second-title-set-variable-accepted
	skill = provideTitledSkill()
	skill.Env["ALT"] = &Variable{Interface: "title-set@1"}
	tests = append(tests, &testCase{
		_id:       "second-title-set-variable-accepted",
		_expected: nil,

		skill: skill,
	})

	// no-block
	tests = append(tests, &testCase{
		_id:       "no-block",
		_expected: nil,

		skill: provideCompleteSkill(),
	})

	// unknown-entry
	skill = provideTitledSkill()
	skill.Title.States["DESIGN-READY"].Entry = "SKILL-DEMO-FLOW-099"
	tests = append(tests, &testCase{
		_id:       "unknown-entry",
		_expected: []string{"title: state DESIGN-READY: unknown entry SKILL-DEMO-FLOW-099"},

		skill: skill,
	})

	// entry-role-def
	skill = provideTitledSkill()
	flowSection(skill).Blocks[0].Entry.Role = RoleDef
	skill.Title.States["DESIGN-READY"].Entry = "SKILL-DEMO-FLOW-001"
	tests = append(tests, &testCase{
		_id: "entry-role-def",
		_expected: []string{
			"title: state DESIGN-READY: entry SKILL-DEMO-FLOW-001 is a def — a gate is presented by a step or a stop",
			"title: state DESIGN-READY: entry SKILL-DEMO-FLOW-001 never runs `$SKILL_DEMO_TITLE_SET title set DESIGN-READY`",
		},

		skill: skill,
	})

	// entry-without-set-command
	skill = provideTitledSkill()
	flowSection(skill).Blocks[1].Entry.Items = nil
	tests = append(tests, &testCase{
		_id:       "entry-without-set-command",
		_expected: []string{"title: state DESIGN-READY: entry SKILL-DEMO-FLOW-002 never runs `$SKILL_DEMO_TITLE_SET title set DESIGN-READY`"},

		skill: skill,
	})

	// no-state
	skill = provideTitledSkill()
	skill.Title.States = map[string]*sessiontitle.DeclaredState{}
	tests = append(tests, &testCase{
		_id:       "no-state",
		_expected: []string{"title: no state declared"},

		skill: skill,
	})

	// declaration-violation-passes-through
	skill = provideTitledSkill()
	skill.Title.States["ready"] = skill.Title.States["DESIGN-READY"]
	delete(skill.Title.States, "DESIGN-READY")
	tests = append(tests, &testCase{
		_id: "declaration-violation-passes-through",
		_expected: []string{
			"title: state ready: token is not uppercase words joined by hyphens",
			"title: state ready: entry SKILL-DEMO-FLOW-002 never runs `$SKILL_DEMO_TITLE_SET title set ready`",
		},

		skill: skill,
	})

	// fail-format-3-set-in-param
	skill = provideTitledSkill()
	setFrontmatter(skill, "format", " 3")
	tests = append(tests, &testCase{
		_id:       "fail-format-3-set-in-param",
		_expected: []string{"title: state DESIGN-READY: entry SKILL-DEMO-FLOW-002 never runs `$SKILL_DEMO_TITLE_SET title set DESIGN-READY`"},

		skill: skill,
	})

	// pass-format-3-set-in-statement
	skill = provideTitledSkill()
	setFrontmatter(skill, "format", " 3")
	gate = flowSection(skill).Blocks[1].Entry
	gate.Items = nil
	gate.Statement = "RUN `$SKILL_DEMO_TITLE_SET title set DESIGN-READY`, then PRESENT the gate."
	tests = append(tests, &testCase{
		_id:       "pass-format-3-set-in-statement",
		_expected: nil,

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := checkTitle(test.skill)

			assert.Equal(t, test._expected, violations)
		})
	}
}

func TestRenderDeployed_TitleJsonNeedsTitleSet(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedHasTitle bool

		options RenderOptions
	}

	helpers := []components.ID{components.SessionHelpers}
	tests := make([]*testCase, 0)

	// canonical-renders-title-json
	tests = append(tests, &testCase{_id: "canonical-renders-title-json", _expectedHasTitle: true, options: RenderOptions{Components: helpers}})

	// set-title-set-renders-title-json
	set := RenderOptions{Components: helpers, Values: map[string]string{"SKILL_DEMO_TITLE_SET": "smine-rules"}}
	tests = append(tests, &testCase{_id: "set-title-set-renders-title-json", _expectedHasTitle: true, options: set})

	// unset-title-set-drops-title-json
	unset := RenderOptions{Components: helpers, Values: map[string]string{}}
	tests = append(tests, &testCase{_id: "unset-title-set-drops-title-json", _expectedHasTitle: false, options: unset})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			files, err := RenderDeployed(test.options, provideTitledSkill())

			require.NoError(t, err)
			assert.Equal(t, test._expectedHasTitle, slices.Contains(renderedPaths(files), sessiontitle.FileName))
		})
	}
}

func TestRender_TitleJson(t *testing.T) {
	skill := &Skill{Frontmatter: []Field{{Key: "name", Value: "demo"}, {Key: "version", Value: "1.0.0"}}, Title: demoTitle()}

	rendered, err := renderTitle(skill)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(rendered), "{\n  \"states\""))
	assert.True(t, strings.HasSuffix(string(rendered), "}\n"))
	assert.Contains(t, string(rendered), "\"schema\": 2")
	assert.Contains(t, string(rendered), "\"skill\": \"demo\"")
	assert.Contains(t, string(rendered), "\"version\": \"1.0.0\"")
}
