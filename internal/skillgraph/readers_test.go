package skillgraph

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// provideReaderSkill is a minimal skill named name with one section whose
// pred is sectionPred, one entry of role per entry pred, and the given local atoms.
func provideReaderSkill(name, role, sectionPred string, atoms []string, preds ...string) *Skill {
	skill := &Skill{
		Atoms:       map[string]*Atom{},
		Frontmatter: []Field{{Key: "name", Value: " " + name}},
		Sections:    []*Section{{Title: "Run", Pred: sectionPred}},
	}
	for _, id := range atoms {
		skill.Atoms[id] = &Atom{Sort: "state"}
	}
	for index, pred := range preds {
		entry := &Entry{Id: fmt.Sprintf("SKILL-X-RUN-%03d", index+1), Pred: pred, Role: role}
		skill.Sections[0].Blocks = append(skill.Sections[0].Blocks, Block{Entry: entry})
	}
	return skill
}

// withFormatFields opts a reader skill into format 3.
func withFormatFields(skill *Skill) *Skill {
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "format", Value: " 3"})
	return skill
}

func TestUnreadAtoms(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skills []*Skill
	}

	vocabulary := &Vocabulary{Atoms: map[string]*Atom{
		"mode":   {Kind: KindAtomEnum, Members: []string{"interactive", "nightly"}, Sort: "mode"},
		"shared": {Sort: "event"},
	}}
	withVocabulary := func(skills ...*Skill) []*Skill {
		for _, skill := range skills {
			skill.Vocabulary = vocabulary
		}
		return skills
	}

	tests := make([]*testCase, 0)

	// pass-all-read
	tests = append(tests, &testCase{
		_id:       "pass-all-read",
		_expected: nil,

		skills: withVocabulary(provideReaderSkill("skill-a", RoleRule, "", []string{"x"}, "x and shared and mode = nightly")),
	})

	// fail-local-unread
	tests = append(tests, &testCase{
		_id:       "fail-local-unread",
		_expected: []string{"skill-a: atom y: no pred reads it — delete it and the posts granting it"},

		skills: withVocabulary(provideReaderSkill("skill-a", RoleRule, "", []string{"x", "y"}, "x and shared and mode = nightly")),
	})

	// pass-local-read-by-section-pred
	tests = append(tests, &testCase{
		_id:       "pass-local-read-by-section-pred",
		_expected: nil,

		skills: withVocabulary(provideReaderSkill("skill-a", RoleRule, "x", []string{"x"}, "shared and mode = interactive")),
	})

	// pass-enum-read-by-comparison
	tests = append(tests, &testCase{
		_id:       "pass-enum-read-by-comparison",
		_expected: nil,

		skills: withVocabulary(provideReaderSkill("skill-a", RoleStop, "", nil, "shared or mode != nightly")),
	})

	// pass-shared-read-by-another-skill
	tests = append(tests, &testCase{
		_id:       "pass-shared-read-by-another-skill",
		_expected: nil,

		skills: withVocabulary(provideReaderSkill("skill-a", RoleRule, "", nil, "mode = nightly"), provideReaderSkill("skill-b", RoleRule, "", nil, "shared")),
	})

	// fail-shared-unread
	tests = append(tests, &testCase{
		_id:       "fail-shared-unread",
		_expected: []string{"vocabulary.cue: atom shared: no skill's stop, rule, quality, route or query reads it — delete it and the clauses naming it"},

		skills: withVocabulary(provideReaderSkill("skill-a", RoleRule, "", nil, "mode = nightly")),
	})

	// pass-no-skills
	tests = append(tests, &testCase{
		_id:       "pass-no-skills",
		_expected: nil,

		skills: nil,
	})

	// fail-format-3-step-only-reader
	tests = append(tests, &testCase{
		_id:       "fail-format-3-step-only-reader",
		_expected: []string{"skill-a: atom x: no stop, rule, quality, route or query reads it — delete it and the clauses naming it"},

		skills: []*Skill{withFormatFields(provideReaderSkill("skill-a", RoleStep, "", []string{"x"}, "x"))},
	})

	// pass-format-3-stop-reader
	tests = append(tests, &testCase{
		_id:       "pass-format-3-stop-reader",
		_expected: nil,

		skills: []*Skill{withFormatFields(provideReaderSkill("skill-a", RoleStop, "", []string{"x"}, "x"))},
	})

	// pass-format-3-query-reader
	queried := withFormatFields(provideReaderSkill("skill-a", RoleStep, "", []string{"x"}, "x"))
	queried.Queries = []Query{{Assume: []string{"x"}, Expect: "sat", Name: "q"}}
	tests = append(tests, &testCase{
		_id:       "pass-format-3-query-reader",
		_expected: nil,

		skills: []*Skill{queried},
	})

	// pass-shared-read-by-rule-elsewhere
	tests = append(tests, &testCase{
		_id:       "pass-shared-read-by-rule-elsewhere",
		_expected: nil,

		skills: withVocabulary(provideReaderSkill("skill-a", RoleStep, "", nil, "shared"), provideReaderSkill("skill-b", RoleRule, "", nil, "shared or mode = nightly")),
	})

	// fail-shared-read-by-step-only
	tests = append(tests, &testCase{
		_id:       "fail-shared-read-by-step-only",
		_expected: []string{"vocabulary.cue: atom shared: no skill's stop, rule, quality, route or query reads it — delete it and the clauses naming it"},

		skills: withVocabulary(provideReaderSkill("skill-a", RoleStep, "", nil, "shared"), provideReaderSkill("skill-b", RoleRule, "", nil, "mode = nightly")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, UnreadAtoms(test.skills))
		})
	}
}

func TestAtomReaders(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Reader

		atom  string
		skill *Skill
	}

	skill := provideReaderSkill("skill-a", RoleStop, "routed", []string{"routed", "x"}, "x and not routed")
	skill.Queries = []Query{{Assume: []string{"x"}, Check: "routed or only", Expect: "unsat", Name: "q"}}

	tests := make([]*testCase, 0)

	// entry-pred
	tests = append(tests, &testCase{
		_id: "entry-pred",
		_expected: []Reader{
			{Entry: "SKILL-X-RUN-001", Role: RoleStop, Section: "Run"},
			{Query: "q", Role: ReaderQuery},
		},

		atom:  "x",
		skill: skill,
	})

	// section-pred
	tests = append(tests, &testCase{
		_id: "section-pred",
		_expected: []Reader{
			{Role: ReaderSection, Section: "Run"},
			{Entry: "SKILL-X-RUN-001", Role: RoleStop, Section: "Run"},
			{Query: "q", Role: ReaderQuery},
		},

		atom:  "routed",
		skill: skill,
	})

	// query-assume-and-check
	tests = append(tests, &testCase{
		_id:       "query-assume-and-check",
		_expected: []Reader{{Query: "q", Role: ReaderQuery}},

		atom:  "only",
		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, AtomReaders(test.skill)[test.atom])
		})
	}
}

func TestCheckClauseAtoms(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	withMode := func(skill *Skill) *Skill {
		skill.Atoms["mode"] = &Atom{Kind: KindAtomEnum, Members: []string{"interactive", "nightly"}, Sort: "mode"}
		return skill
	}

	tests := make([]*testCase, 0)

	// fail-unknown-identifier
	tests = append(tests, &testCase{
		_id:       "fail-unknown-identifier",
		_expected: []string{"SKILL-X-RUN-001: pred: ghost is no atom and no enum member"},

		skill: withFormatFields(provideReaderSkill("skill-a", RoleStop, "", []string{"x"}, "x and ghost")),
	})

	// pass-enum-member
	tests = append(tests, &testCase{
		_id:       "pass-enum-member",
		_expected: nil,

		skill: withMode(withFormatFields(provideReaderSkill("skill-a", RoleStop, "", nil, "mode = nightly"))),
	})

	// pass-keyword-ignored
	tests = append(tests, &testCase{
		_id:       "pass-keyword-ignored",
		_expected: nil,

		skill: withFormatFields(provideReaderSkill("skill-a", RoleStop, "", []string{"x", "y"}, "not x and (y or x)")),
	})

	// pass-format-2-ignored
	tests = append(tests, &testCase{
		_id:       "pass-format-2-ignored",
		_expected: nil,

		skill: provideReaderSkill("skill-a", RoleStop, "", nil, "ghost"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkClauseAtoms(test.skill))
		})
	}
}

func TestUnreadAtoms_RepoSkills(t *testing.T) {
	dirs, err := Discover("../../skills")
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	skills := make([]*Skill, 0, len(dirs))
	for _, dir := range dirs {
		skill, err := Load(dir)
		require.NoError(t, err, dir)
		skills = append(skills, skill)
	}
	assert.Empty(t, UnreadAtoms(skills))
}
