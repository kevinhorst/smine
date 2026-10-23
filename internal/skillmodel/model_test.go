package skillmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
)

// fixtureChangelog is the one-row changelog every imported fixture carries —
// Check ties the frontmatter version to its newest row.
const fixtureChangelog = `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`

const modelFixtureBody = "## Dispatch\n\n" +
	"`{pred: interactive => not nightly}`\\\n" +
	"**SKILL-DEMO-EXEC-001** `[rule]` **Never prompt**\n\n" +
	"Review NEVER prompts.\n\n" +
	"`{post: resolve_base}`\\\n" +
	"**SKILL-DEMO-PROC-001** `[step]` **Resolve base**\n\n" +
	"RESOLVE the base.\n\n" +
	"`{pred: nightly}`\\\n" +
	"**SKILL-DEMO-PROC-002** `[stop]` **Base stop**\n\n" +
	"If the base will not resolve ⇒ STOP: report.\n\n" +
	"**SKILL-DEMO-PROC-003** `[step]` **Unformalized step**\n\n" +
	"REPORT the result.\n"

// importFixture writes a fixture body under a frontmatter naming the skill
// into <tmp>/<name>/SKILL.md with its changelog, and imports it into the
// model the way skillmodel import does.
func importFixture(t *testing.T, body, name string) *skillgraph.Skill {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	frontmatter := "---\nname: " + name + "\ndescription: " + name + " fixture. Trigger on nothing.\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n---\n"
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(frontmatter+body), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(fixtureChangelog), 0o600))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	return skill
}

// deriveFixture derives a fixture skill's model, failing the test on a Check
// violation.
func deriveFixture(t *testing.T, skill *skillgraph.Skill) *Model {
	t.Helper()
	model, err := Derive(skill)
	require.NoError(t, err)
	return model
}

func fixtureModel(t *testing.T) *Model {
	t.Helper()
	return deriveFixture(t, importFixture(t, modelFixtureBody, "demo"))
}

func fixtureVocabulary() *Vocabulary {
	vocab := &Vocabulary{
		Exclusive: [][]string{{"interactive", "nightly"}},
		Sorts: map[string][]Atom{
			"event": {{Id: "resolve_base"}, {Id: "halt"}},
			"mode":  {{Id: "interactive", Source: "session"}, {Id: "nightly", Source: "session"}},
		},
	}
	return vocab
}

// validateWith sets a model's tier and vocabulary, then validates it.
func validateWith(isStrict bool, model *Model, vocab *Vocabulary) []string {
	model.IsStrict = isStrict
	model.Vocabulary = vocab
	return model.Validate()
}

func TestValidate(t *testing.T) {
	vocab := fixtureVocabulary()

	t.Run("clean-model", func(t *testing.T) {
		assert.Empty(t, validateWith(true, fixtureModel(t), vocab))
	})

	t.Run("producer-or-input", func(t *testing.T) {
		// Sourceless bool guarding flow: violation; a grant or a source heals
		// it; invariant-only atoms stay exempt.
		bare := fixtureVocabulary()
		bare.Sorts["mode"] = []Atom{{Id: "interactive"}, {Id: "nightly"}}
		found := strings.Join(validateWith(true, fixtureModel(t), bare), "\n")
		assert.Contains(t, found, `bool atom "nightly" guards flow but nothing grants it and it declares no source`)
		assert.NotContains(t, found, `"interactive" guards flow`)

		granted := fixtureModel(t)
		granted.Transitions[0].Post = append(granted.Transitions[0].Post, "nightly")
		assert.Empty(t, validateWith(true, granted, bare))
	})

	t.Run("unresolved-pred-atom", func(t *testing.T) {
		model := fixtureModel(t)
		model.Transitions[1].Pred = "warp_drive"
		found := validateWith(true, model, vocab)
		assert.Contains(t, strings.Join(found, "\n"), `unresolved atom "warp_drive"`)
	})

	t.Run("unresolved-post-atom", func(t *testing.T) {
		model := fixtureModel(t)
		model.Transitions[0].Post = []string{"warp_drive"}
		found := validateWith(true, model, vocab)
		assert.Contains(t, strings.Join(found, "\n"), `post SKILL-DEMO-PROC-001: unresolved atom "warp_drive"`)
	})

	t.Run("bad-pred-expr", func(t *testing.T) {
		model := fixtureModel(t)
		model.Invariants[0].Expr = "interactive and"
		found := validateWith(true, model, vocab)
		assert.Contains(t, strings.Join(found, "\n"), "invariant SKILL-DEMO-EXEC-001")
	})

	t.Run("unknown-section", func(t *testing.T) {
		model := fixtureModel(t)
		model.Transitions[0].Section = "warp"
		found := validateWith(true, model, vocab)
		assert.Contains(t, strings.Join(found, "\n"), `unknown section "warp"`)
	})

	t.Run("pass-exclusive-bool-members", func(t *testing.T) {
		assert.Empty(t, validateWith(true, fixtureModel(t), vocab))
	})

	t.Run("fail-exclusive-member-undeclared", func(t *testing.T) {
		ghost := fixtureVocabulary()
		ghost.Exclusive = [][]string{{"interactive", "ghost"}}
		found := validateWith(true, fixtureModel(t), ghost)
		assert.Contains(t, strings.Join(found, "\n"), `exclusive group 0: member "ghost" is not a declared bool atom`)
	})

	t.Run("fail-exclusive-member-int", func(t *testing.T) {
		counted := fixtureVocabulary()
		counted.Sorts["count"] = []Atom{{Id: "lanes", Kind: skillgraph.KindAtomInt}}
		counted.Exclusive = [][]string{{"interactive", "lanes"}}
		found := validateWith(true, fixtureModel(t), counted)
		assert.Contains(t, strings.Join(found, "\n"), `exclusive group 0: member "lanes" is not a declared bool atom`)
	})

	t.Run("fail-exclusive-annotation-tier", func(t *testing.T) {
		ghost := fixtureVocabulary()
		ghost.Exclusive = [][]string{{"interactive", "ghost"}}
		found := validateWith(false, fixtureModel(t), ghost)
		assert.Contains(t, strings.Join(found, "\n"), `member "ghost" is not a declared bool atom`)
	})

	t.Run("pass-nil-vocabulary", func(t *testing.T) {
		model := &Model{Sections: []Section{{Id: "dispatch", Title: "Dispatch"}}}
		assert.Empty(t, model.Validate())
	})
}

func TestValidateAnnotationTier(t *testing.T) {
	vocab := fixtureVocabulary()

	invariant := func(expr string) *Model {
		model := &Model{
			Invariants: []Invariant{{Id: "SKILL-DEMO-EXEC-001", Expr: expr, Text: "t"}},
			Sections:   []Section{{Id: "dispatch", Title: "Dispatch"}},
		}
		return model
	}

	t.Run("undeclared-bool-atom-passes", func(t *testing.T) {
		assert.Empty(t, validateWith(false, invariant("warp_drive => not warp_core_breach"), vocab))
	})

	t.Run("undeclared-typed-comparisons-pass", func(t *testing.T) {
		assert.Empty(t, validateWith(false, invariant("run_mode = batch"), vocab))
		assert.Empty(t, validateWith(false, invariant("rejection_count >= 2"), vocab))
	})

	t.Run("undeclared-post-atom-passes", func(t *testing.T) {
		model := invariant("interactive")
		transition := Transition{Id: "SKILL-DEMO-PROC-001", Kind: KindStep, Post: []string{"warp_drive"}, Section: "dispatch", Text: "t"}
		model.Transitions = append(model.Transitions, transition)
		assert.Empty(t, validateWith(false, model, vocab))
	})

	t.Run("malformed-pred-fails-both-tiers", func(t *testing.T) {
		assert.NotEmpty(t, validateWith(false, invariant("interactive and"), vocab))
		assert.NotEmpty(t, validateWith(true, invariant("interactive and"), vocab))
	})

	t.Run("same-clauses-fail-strict", func(t *testing.T) {
		assert.NotEmpty(t, validateWith(true, invariant("run_mode = batch"), vocab))
	})
}

func TestValidateKinds(t *testing.T) {
	vocab := typedVocabulary()

	invariant := func(expr string) *Model {
		model := &Model{
			Invariants: []Invariant{{Id: "SKILL-DEMO-EXEC-001", Expr: expr, Text: "t"}},
			Sections:   []Section{{Id: "dispatch", Title: "Dispatch"}},
		}
		return model
	}

	t.Run("clean-typed-clauses", func(t *testing.T) {
		assert.Empty(t, validateWith(true, invariant("lanes_returned = lanes_spawned"), vocab))
		assert.Empty(t, validateWith(true, invariant("apply_level >= medium"), vocab))
		assert.Empty(t, validateWith(true, invariant("interactive => lanes_spawned > 0"), vocab))
	})

	t.Run("bool-atom-in-comparison", func(t *testing.T) {
		found := validateWith(true, invariant("interactive < 2"), vocab)
		assert.Contains(t, strings.Join(found, "\n"), `atom "interactive" used in a comparison but has kind bool`)
	})

	t.Run("int-atom-as-boolean", func(t *testing.T) {
		found := validateWith(true, invariant("lanes_spawned"), vocab)
		assert.Contains(t, strings.Join(found, "\n"), `atom "lanes_spawned" used as boolean but has kind int`)
	})

	t.Run("int-enum-mix", func(t *testing.T) {
		found := validateWith(true, invariant("lanes_spawned = medium"), vocab)
		assert.Contains(t, strings.Join(found, "\n"), "comparison mixes int and enum sides")
	})

	t.Run("enum-in-arithmetic", func(t *testing.T) {
		found := validateWith(true, invariant("apply_level + 1 > 0"), vocab)
		assert.Contains(t, strings.Join(found, "\n"), `atom "apply_level" used in arithmetic but has kind enum`)
	})

	t.Run("post-non-bool", func(t *testing.T) {
		model := invariant("interactive")
		transition := Transition{Id: "SKILL-DEMO-PROC-001", Kind: KindStep, Post: []string{"lanes_spawned"}, Section: "dispatch", Text: "t"}
		model.Transitions = append(model.Transitions, transition)
		found := validateWith(true, model, vocab)
		assert.Contains(t, strings.Join(found, "\n"), "post grants bool atoms only")
	})

	t.Run("member-collision", func(t *testing.T) {
		collided := typedVocabulary()
		collided.Sorts["state"] = []Atom{{Id: "medium"}}
		found := validateWith(true, invariant("interactive"), collided)
		assert.Contains(t, strings.Join(found, "\n"), `member "medium" collides with an atom id`)
	})

	t.Run("unknown-kind", func(t *testing.T) {
		broken := typedVocabulary()
		broken.Sorts["state"] = []Atom{{Id: "odd", Kind: "float"}}
		found := validateWith(true, invariant("interactive"), broken)
		assert.Contains(t, strings.Join(found, "\n"), `atom odd: unknown kind "float"`)
	})
}

func TestStats(t *testing.T) {
	formalized, coverable := fixtureModel(t).Stats()

	assert.Equal(t, 3, formalized)
	assert.Equal(t, 4, coverable)
}

func TestVocabularyOf(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Vocabulary

		skill *skillgraph.Skill
	}

	shared := &skillgraph.Vocabulary{
		Atoms: map[string]*skillgraph.Atom{
			"mode":    {Kind: skillgraph.KindAtomEnum, Members: []string{"interactive", "nightly"}, Sort: "mode", Source: "session"},
			"tools_x": {Sort: "event"},
		},
		Exclusive: [][]string{{"a", "b"}},
	}
	tests := make([]*testCase, 0)

	// shared-only
	tests = append(tests, &testCase{
		_id: "shared-only",
		_expected: &Vocabulary{
			Exclusive: [][]string{{"a", "b"}},
			Sorts: map[string][]Atom{
				"event": {{Id: "tools_x"}},
				"mode":  {{Id: "mode", Kind: skillgraph.KindAtomEnum, Members: []string{"interactive", "nightly"}, Source: "session"}},
			},
		},
		skill: &skillgraph.Skill{Vocabulary: shared},
	})

	// local-overlays-shared
	tests = append(tests, &testCase{
		_id: "local-overlays-shared",
		_expected: &Vocabulary{
			Exclusive: [][]string{{"a", "b"}},
			Sorts: map[string][]Atom{
				"mode":  {{Id: "mode", Kind: skillgraph.KindAtomEnum, Members: []string{"interactive", "nightly"}, Source: "session"}},
				"state": {{Id: "tools_x", Source: "arg"}},
			},
		},
		skill: &skillgraph.Skill{Atoms: map[string]*skillgraph.Atom{"tools_x": {Sort: "state", Source: "arg"}}, Vocabulary: shared},
	})

	// sorts-grouped-sorted-by-id
	tests = append(tests, &testCase{
		_id: "sorts-grouped-sorted-by-id",
		_expected: &Vocabulary{
			Sorts: map[string][]Atom{
				"count": {{Id: "lanes", Kind: skillgraph.KindAtomInt}},
				"state": {{Id: "alpha"}, {Id: "beta"}, {Id: "gamma"}},
			},
		},
		skill: &skillgraph.Skill{Atoms: map[string]*skillgraph.Atom{
			"alpha": {Sort: "state"},
			"beta":  {Sort: "state"},
			"gamma": {Sort: "state"},
			"lanes": {Kind: skillgraph.KindAtomInt, Sort: "count"},
		}},
	})

	// exclusive-vocabulary-then-skill
	tests = append(tests, &testCase{
		_id: "exclusive-vocabulary-then-skill",
		_expected: &Vocabulary{
			Exclusive: [][]string{{"a", "b"}, {"c", "d"}},
			Sorts: map[string][]Atom{
				"event": {{Id: "tools_x"}},
				"mode":  {{Id: "mode", Kind: skillgraph.KindAtomEnum, Members: []string{"interactive", "nightly"}, Source: "session"}},
			},
		},
		skill: &skillgraph.Skill{Exclusive: [][]string{{"c", "d"}}, Vocabulary: shared},
	})

	// nil-vocabulary
	tests = append(tests, &testCase{
		_id: "nil-vocabulary",
		_expected: &Vocabulary{
			Sorts: map[string][]Atom{"state": {{Id: "alpha"}}},
		},
		skill: &skillgraph.Skill{Atoms: map[string]*skillgraph.Atom{"alpha": {Sort: "state"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, VocabularyOf(test.skill))
		})
	}
}
