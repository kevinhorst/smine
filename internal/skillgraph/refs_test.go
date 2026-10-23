package skillgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// provideOtherSkill is a second skill under the root with one entry, SKILL-OTHER-RUN-001.
func provideOtherSkill() *Skill {
	entry := &Entry{Id: "SKILL-OTHER-RUN-001", Role: RoleStep, Statement: "RUN it.", Title: "Run it"}
	skill := &Skill{
		Frontmatter: []Field{{Key: "name", Value: " other"}},
		Sections:    []*Section{{Blocks: []Block{{Entry: entry}}, Title: "Run"}},
	}
	return skill
}

// withStatement sets one entry's statement.
func withStatement(skill *Skill, id, statement string) *Skill {
	return withEntry(skill, id, func(entry *Entry) { entry.Statement = statement })
}

func TestRenderRef(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		token string
	}

	tests := make([]*testCase, 0)

	// local-known
	tests = append(tests, &testCase{
		_id:       "local-known",
		_expected: "[First step · FLOW-002](#skill-demo-flow-002)",

		token: "FLOW-002",
	})

	// local-unknown-keeps-id
	tests = append(tests, &testCase{
		_id:       "local-unknown-keeps-id",
		_expected: "[FLOW-099 · FLOW-099](#skill-demo-flow-099)",

		token: "FLOW-099",
	})

	// foreign-full-id
	tests = append(tests, &testCase{
		_id:       "foreign-full-id",
		_expected: "[SKILL-OTHER-RUN-001 · SKILL-OTHER-RUN-001](#skill-other-run-001)",

		token: "SKILL-OTHER-RUN-001",
	})

	skill := provideFieldsSkill()

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, RenderRef(skill, test.token))
		})
	}
}

func TestCheckRefs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// pass-def-link-across-sections
	tests = append(tests, &testCase{
		_id:       "pass-def-link-across-sections",
		_expected: nil,

		skill: withStatement(provideFieldsSkill(), "SKILL-DEMO-NIGHT-001", "RUN at night in [[FLOW-010]]."),
	})

	// pass-flow-link-same-section
	tests = append(tests, &testCase{
		_id:       "pass-flow-link-same-section",
		_expected: nil,

		skill: withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN after [[FLOW-002]]."),
	})

	// pass-route-links-anchor
	tests = append(tests, &testCase{
		_id:       "pass-route-links-anchor",
		_expected: nil,

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-NIGHT-001", func(entry *Entry) {
			entry.Fields = &Fields{Ref: []string{"FLOW-002"}}
		}),
	})

	// fail-dangling
	tests = append(tests, &testCase{
		_id:       "fail-dangling",
		_expected: []string{"SKILL-DEMO-FLOW-003: link FLOW-099: no such entry"},

		skill: withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN after [[FLOW-099]]."),
	})

	// fail-flow-link-across-sections
	tests = append(tests, &testCase{
		_id:       "fail-flow-link-across-sections",
		_expected: []string{`SKILL-DEMO-FLOW-003: link NIGHT-001: a step in section "Nightly" — flow links stay inside the section`},

		skill: withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN after [[NIGHT-001]]."),
	})

	// fail-def-links
	tests = append(tests, &testCase{
		_id:       "fail-def-links",
		_expected: []string{"SKILL-DEMO-FLOW-010: a def links nothing — TPL-001"},

		skill: withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-010", "The `repo` path, shaped as [[TPL-001]]."),
	})

	// fail-args-dangling
	dangling := provideFieldsSkill()
	dangling.Sections[1].Blocks[0].Args.Rows[0].Effect = "picks a mode — [[FLOW-099]]"
	tests = append(tests, &testCase{
		_id:       "fail-args-dangling",
		_expected: []string{"args or routing: link FLOW-099: no such entry"},

		skill: dangling,
	})

	// pass-format-2-ignored
	tests = append(tests, &testCase{
		_id:       "pass-format-2-ignored",
		_expected: nil,

		skill: withStatement(provideCompleteSkill(), "SKILL-DEMO-FLOW-003", "RUN after [[FLOW-099]]."),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkRefs(test.skill))
		})
	}
}

func TestCheckTitles(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// fail-duplicate-case-insensitive
	tests = append(tests, &testCase{
		_id:       "fail-duplicate-case-insensitive",
		_expected: []string{`SKILL-DEMO-FLOW-003: title "first STEP" is also SKILL-DEMO-FLOW-002's`},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", func(entry *Entry) { entry.Title = "first STEP" }),
	})

	// pass-distinct
	tests = append(tests, &testCase{
		_id:       "pass-distinct",
		_expected: nil,

		skill: provideFieldsSkill(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkTitles(test.skill))
		})
	}
}

func TestDisplayText(t *testing.T) {
	text := displayText(provideFieldsSkill(), "after [[FLOW-002]] and [[FLOW-099]]")

	assert.Equal(t, "after First step and FLOW-099", text)
}

func TestUnresolvedCrossRefs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skills []*Skill
	}

	tests := make([]*testCase, 0)

	// pass-known-entry
	tests = append(tests, &testCase{
		_id:       "pass-known-entry",
		_expected: nil,

		skills: []*Skill{withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN [[SKILL-OTHER-RUN-001]]."), provideOtherSkill()},
	})

	// fail-unknown-skill
	tests = append(tests, &testCase{
		_id:       "fail-unknown-skill",
		_expected: []string{"demo: SKILL-DEMO-FLOW-003: link SKILL-GHOST-RUN-001: no skill GHOST under the root"},

		skills: []*Skill{withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN [[SKILL-GHOST-RUN-001]]."), provideOtherSkill()},
	})

	// fail-unknown-entry
	tests = append(tests, &testCase{
		_id:       "fail-unknown-entry",
		_expected: []string{"demo: SKILL-DEMO-FLOW-003: link SKILL-OTHER-RUN-002: no such entry in other"},

		skills: []*Skill{withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN [[SKILL-OTHER-RUN-002]]."), provideOtherSkill()},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, UnresolvedCrossRefs(test.skills))
		})
	}
}

func TestTextRefs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		text string
	}

	tests := make([]*testCase, 0)

	// markdown-link
	tests = append(tests, &testCase{
		_id:       "markdown-link",
		_expected: []string{"SKILL-DEMO-FLOW-002b"},

		text: "see [Spawn bound](#skill-demo-flow-002b)",
	})

	// short-and-full-tokens
	tests = append(tests, &testCase{
		_id:       "short-and-full-tokens",
		_expected: []string{"SKILL-DEMO-MODES-004", "SKILL-OTHER-RUN-001"},

		text: "binding override — [[MODES-004]], then [[SKILL-OTHER-RUN-001]]",
	})

	// link-and-token-to-one-entry-once
	tests = append(tests, &testCase{
		_id:       "link-and-token-to-one-entry-once",
		_expected: []string{"SKILL-DEMO-FLOW-001"},

		text: "[Start](#skill-demo-flow-001) or [[FLOW-001]]",
	})

	// plain-text
	tests = append(tests, &testCase{
		_id:       "plain-text",
		_expected: nil,

		text: "headless mode, [not a link] and [[lowercase-001]]",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, TextRefs("demo", test.text))
		})
	}
}
