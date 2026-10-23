package skillgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkill_Entries(t *testing.T) {
	skill := provideCompleteSkill()
	flowSection(skill).Blocks = append(flowSection(skill).Blocks, Block{Heading: "Later"}, Block{Entry: &Entry{Id: "SKILL-DEMO-FLOW-004", Role: RoleStep, Title: "Later"}})

	refs := skill.Entries()
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.Entry.Id)
	}
	assert.Equal(t, []string{"SKILL-DEMO-FLOW-001", "SKILL-DEMO-FLOW-002", "SKILL-DEMO-FLOW-003", "SKILL-DEMO-TPL-001", "SKILL-DEMO-FLOW-004", "SKILL-DEMO-NIGHT-001"}, ids)
	assert.Equal(t, "Later", refs[4].Subsection)
	assert.Equal(t, "Flow", refs[4].Section.Title)
	assert.Empty(t, refs[5].Subsection)

	ref, found := skill.Entry("SKILL-DEMO-FLOW-003")
	assert.True(t, found)
	assert.Equal(t, "Second step", ref.Entry.Title)

	_, found = skill.Entry("SKILL-DEMO-FLOW-099")
	assert.False(t, found)
}

func TestEntry_BulletAndParams(t *testing.T) {
	entry := &Entry{Items: []Item{
		{Label: LabelParam, Marker: "*", Text: "top"},
		{Depth: 1, Label: LabelParam, Marker: "-", Text: "nested"},
		{Fence: &Fence{Marker: "```"}},
		{Label: LabelApplies, Marker: "*", Text: "applies text"},
		{Depth: 1, Label: LabelApplies, Marker: "*", Text: "nested applies"},
	}}
	assert.Equal(t, []string{"top"}, entry.Params())
	assert.Equal(t, "applies text", entry.Bullet(LabelApplies))
	assert.Empty(t, entry.Bullet(LabelGate))
}

func TestEntry_Refs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		entry *Entry
	}

	tests := make([]*testCase, 0)

	// markdown-link
	tests = append(tests, &testCase{
		_id:       "markdown-link",
		_expected: []string{"SKILL-XY-PROC-001"},

		entry: &Entry{Statement: "RUN [Proc](#skill-xy-proc-001) first."},
	})

	// short-token
	tests = append(tests, &testCase{
		_id:       "short-token",
		_expected: []string{"SKILL-XY-MODES-014b"},

		entry: &Entry{Statement: "RUN [[MODES-014b]] first; [[not a token]] stays text."},
	})

	// full-token
	tests = append(tests, &testCase{
		_id:       "full-token",
		_expected: []string{"SKILL-OTHER-PROC-002"},

		entry: &Entry{Items: []Item{{Marker: "*", Text: "see [[SKILL-OTHER-PROC-002]]"}}},
	})

	// ref-field
	tests = append(tests, &testCase{
		_id:       "ref-field",
		_expected: []string{"SKILL-XY-PROC-003", "SKILL-OTHER-PROC-004"},

		entry: &Entry{Fields: &Fields{Ref: []string{"PROC-003", "SKILL-OTHER-PROC-004"}}},
	})

	// dedupe-keeps-order
	tests = append(tests, &testCase{
		_id:       "dedupe-keeps-order",
		_expected: []string{"SKILL-XY-PROC-001", "SKILL-XY-MODES-014b"},

		entry: &Entry{
			Fields:    &Fields{Ref: []string{"MODES-014b", "PROC-001"}},
			Items:     []Item{{Marker: "*", Text: "see [Other](#skill-xy-modes-014b) and [[PROC-001]]"}},
			Statement: "RUN [Proc](#skill-xy-proc-001) first.",
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.entry.Refs("xy"))
		})
	}
}

func TestEntry_Pointers(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedSchema string
		_expectedScript string

		entry *Entry
	}

	labelItems := []Item{
		{Label: LabelSchema, Marker: "*", Text: "schemas/label.json"},
		{Label: LabelScript, Marker: "*", Text: "workflows/label.js"},
	}
	tests := make([]*testCase, 0)

	// field-wins
	tests = append(tests, &testCase{
		_id:             "field-wins",
		_expectedSchema: "schemas/field.json",
		_expectedScript: "workflows/field.js",

		entry: &Entry{Fields: &Fields{Schema: "schemas/field.json", Script: "workflows/field.js"}, Items: labelItems},
	})

	// label-below-format-3
	tests = append(tests, &testCase{
		_id:             "label-below-format-3",
		_expectedSchema: "schemas/label.json",
		_expectedScript: "workflows/label.js",

		entry: &Entry{Items: labelItems},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedSchema, test.entry.SchemaPointer())
			assert.Equal(t, test._expectedScript, test.entry.ScriptPointer())
		})
	}
}

func TestEntry_EffectiveChannel(t *testing.T) {
	assert.Equal(t, "prose", (&Entry{Role: RoleStep}).EffectiveChannel())
	assert.Equal(t, "gate", (&Entry{Channel: "gate", Role: RoleStep}).EffectiveChannel())
	assert.Empty(t, (&Entry{Role: RolePayload}).EffectiveChannel())
}

func TestSkill_AllAtoms(t *testing.T) {
	skill := provideCompleteSkill()
	skill.Vocabulary = &Vocabulary{Atoms: map[string]*Atom{"mode": {Sort: "mode"}, "panic": {Sort: "shared"}}}

	atoms := skill.AllAtoms()
	assert.Len(t, atoms, 3)
	assert.Equal(t, "state", atoms["panic"].Sort, "skill-local overlays shared")

	atom, found := skill.Atom("mode")
	assert.True(t, found)
	assert.Equal(t, "mode", atom.Sort)
	assert.Equal(t, KindAtomBool, atom.EffectiveKind())
	assert.Equal(t, "demo.", skill.Routing().Purpose)
	assert.Equal(t, "frontier", skill.Model().Tier)
	assert.Equal(t, "mode", skill.Args()[0].Name)
	assert.Equal(t, "demo", skill.Name())
	assert.Equal(t, "1.0", skill.Version())
}

func TestSkill_Aliases(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// absent
	tests = append(tests, &testCase{
		_id:       "absent",
		_expected: nil,

		skill: provideCompleteSkill(),
	})

	// empty-value
	skill := provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " "})
	tests = append(tests, &testCase{
		_id:       "empty-value",
		_expected: nil,

		skill: skill,
	})

	// single
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " pk"})
	tests = append(tests, &testCase{
		_id:       "single",
		_expected: []string{"pk"},

		skill: skill,
	})

	// several-trimmed
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " p ,pk"})
	tests = append(tests, &testCase{
		_id:       "several-trimmed",
		_expected: []string{"p", "pk"},

		skill: skill,
	})

	// quoted
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: ` "pk"`})
	tests = append(tests, &testCase{
		_id:       "quoted",
		_expected: []string{"pk"},

		skill: skill,
	})

	// empty-element-kept
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " pk,"})
	tests = append(tests, &testCase{
		_id:       "empty-element-kept",
		_expected: []string{"pk", ""},

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.skill.Aliases())
		})
	}
}

func TestCoverableRole(t *testing.T) {
	for _, role := range []string{RoleQuality, RoleRule, RoleSpawn, RoleStep, RoleStop, RoleSubstep, RoleVerify} {
		assert.True(t, CoverableRole(role), role)
	}
	for _, role := range []string{RoleDef, RolePayload, "unknown"} {
		assert.False(t, CoverableRole(role), role)
	}
}
