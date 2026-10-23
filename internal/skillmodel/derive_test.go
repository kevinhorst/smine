package skillmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

func TestDerive(t *testing.T) {
	body := "## Dispatch\n\n" +
		"**SKILL-DEMO-TERM-001** `[def]` **Round**\n\n" +
		"A round is one review pass.\n\n" +
		"`{pred: interactive => not nightly}`\\\n" +
		"**SKILL-DEMO-EXEC-001** `[rule]` **Never prompt**\n\n" +
		"Review NEVER prompts.\n\n" +
		"**SKILL-DEMO-EXEC-002** `[rule]` **Unformalized rule**\n\n" +
		"Handoffs MUST travel as files.\n\n" +
		"## Round\n\n" +
		"`{pred: nightly and not interactive}`\\\n" +
		"**SKILL-DEMO-PROC-001** `[stop]` **Base stop**\n\n" +
		"If the base will not resolve ⇒ STOP: report.\n\n" +
		"`{post: tools_loaded and round_ready}`\\\n" +
		"**SKILL-DEMO-PROC-002** `[verify]` **Tools loaded**\n\n" +
		"Before dispatch: VERIFY tools loaded; load them.\n"
	model := deriveFixture(t, importFixture(t, body, "demo"))

	// rule with pred becomes an invariant; without pred it stays uncovered.
	require.Len(t, model.Invariants, 1)
	assert.Equal(t, "SKILL-DEMO-EXEC-001", model.Invariants[0].Id)
	assert.Equal(t, "interactive => not nightly", model.Invariants[0].Expr)

	// def entries carry no model content and open no section.
	require.Len(t, model.Transitions, 2)
	assert.Equal(t, KindStop, model.Transitions[0].Kind)
	assert.Equal(t, "nightly and not interactive", model.Transitions[0].Pred)
	assert.Empty(t, model.Transitions[0].Post)
	assert.Equal(t, KindVerify, model.Transitions[1].Kind)
	assert.Equal(t, []string{"tools_loaded", "round_ready"}, model.Transitions[1].Post)

	// sections chain in document order.
	require.Len(t, model.Sections, 2)
	assert.Equal(t, "dispatch", model.Sections[0].Id)
	assert.Empty(t, model.Sections[0].After)
	assert.Equal(t, "round", model.Sections[1].Id)
	assert.Equal(t, []string{"dispatch"}, model.Sections[1].After)

	formalized, coverable := model.Stats()
	assert.Equal(t, 3, formalized)
	assert.Equal(t, 4, coverable)
}

func TestDeriveCheckViolation(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		skill *skillgraph.Skill
	}

	body := "## Dispatch\n\n" +
		"**SKILL-DEMO-PROC-001** `[step]` **Resolve base**\n\n" +
		"RESOLVE the base.\n"
	tests := make([]*testCase, 0)

	// fail-version-mismatch
	mismatched := importFixture(t, body, "demo")
	mismatched.Changelog[0].Version = "0.9"
	tests = append(tests, &testCase{
		_id:         "fail-version-mismatch",
		_shouldPass: false,

		skill: mismatched,
	})

	// pass-clean
	tests = append(tests, &testCase{
		_id:         "pass-clean",
		_shouldPass: true,

		skill: importFixture(t, body, "demo"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			model, err := Derive(test.skill)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._shouldPass, model != nil)
		})
	}
}

func TestDeriveSectionKinds(t *testing.T) {
	body := "## Reference\n" +
		"`{section: context}`\n\n" +
		"`{pred: interactive => not nightly}`\\\n" +
		"**SKILL-DEMO-EXEC-001** `[rule]` **Never prompt**\n\n" +
		"Review NEVER prompts.\n\n" +
		"**SKILL-DEMO-REF-001** `[step]` **Reference step**\n\n" +
		"RUN the referenced sub-protocol.\n\n" +
		"## Dispatch\n\n" +
		"**SKILL-DEMO-PROC-001** `[step]` **Resolve base**\n\n" +
		"RESOLVE the base.\n\n" +
		"## Round\n\n" +
		"**SKILL-DEMO-PROC-002** `[step]` **Run round**\n\n" +
		"RUN the round.\n\n" +
		"## Nightly route\n" +
		"`{section: route, pred: nightly, between: PROC-002..end}`\n\n" +
		"`{pred: resolve_base}`\\\n" +
		"**SKILL-DEMO-NIGHT-001** `[step]` **Route it**\n\n" +
		"ROUTE the run.\n\n" +
		"**SKILL-DEMO-NIGHT-002** `[step]` **Plain route step**\n\n" +
		"RUN it.\n"
	model := deriveFixture(t, importFixture(t, body, "demo"))

	// Kinds carried; After chains phase sections only.
	byId := map[string]Section{}
	for _, section := range model.Sections {
		byId[section.Id] = section
	}
	assert.Equal(t, skillgraph.SectionKindContext, byId["reference"].Kind)
	assert.Empty(t, byId["reference"].After)
	assert.Equal(t, skillgraph.SectionKindPhase, byId["dispatch"].Kind)
	assert.Empty(t, byId["dispatch"].After)
	assert.Equal(t, []string{"dispatch"}, byId["round"].After)
	assert.Equal(t, skillgraph.SectionKindRoute, byId["nightly-route"].Kind)
	assert.Equal(t, "nightly", byId["nightly-route"].Pred)
	assert.Empty(t, byId["nightly-route"].After)

	// Route pred conjoined into the route's transitions.
	preds := map[string]string{}
	for _, transition := range model.Transitions {
		preds[transition.Id] = transition.Pred
	}
	assert.Equal(t, "(nightly) and (resolve_base)", preds["SKILL-DEMO-NIGHT-001"])
	assert.Equal(t, "nightly", preds["SKILL-DEMO-NIGHT-002"])
	assert.Empty(t, preds["SKILL-DEMO-PROC-001"])

	// Between anchor resolved to full ids, end kept; invariants carry their
	// section.
	assert.Equal(t, []string{"SKILL-DEMO-PROC-002", "end"}, byId["nightly-route"].Between)
	assert.Empty(t, byId["nightly-route"].Replaces)
	assert.Empty(t, byId["round"].Between)
	require.Len(t, model.Invariants, 1)
	assert.Equal(t, "reference", model.Invariants[0].Section)
}

func TestDeriveTierAndReplaces(t *testing.T) {
	body := "## Round\n\n" +
		"`{pred: nightly => not interactive}`\\\n" +
		"**SKILL-DEMO-EXEC-001** `[rule]` **Never prompt**\n\n" +
		"Review NEVER prompts.\n\n" +
		"`{pred: interactive}`\\\n" +
		"**SKILL-DEMO-GOAL-001** `[quality]` **Stay quick**\n\n" +
		"Review MUST stay quick.\n\n" +
		"**SKILL-DEMO-PROC-001** `[step]` **First**\n\n" +
		"RUN the first step.\n\n" +
		"**SKILL-DEMO-PROC-002** `[step]` **Second**\n\n" +
		"RUN the second step.\n\n" +
		"**SKILL-DEMO-PROC-003** `[step]` **Third**\n\n" +
		"RUN the third step.\n\n" +
		"## Nightly route\n" +
		"`{section: route, pred: nightly, replaces: PROC-001..PROC-002 + PROC-002}`\n\n" +
		"**SKILL-DEMO-NIGHT-001** `[step]` **Route it**\n\n" +
		"ROUTE the run.\n"

	t.Run("annotation-tier", func(t *testing.T) {
		skill := importFixture(t, body, "demo")
		model := deriveFixture(t, skill)
		assert.False(t, model.IsStrict)
		assert.Equal(t, VocabularyOf(skill), model.Vocabulary)
	})

	t.Run("model-tier", func(t *testing.T) {
		skill := importFixture(t, body, "demo")
		skill.Atoms = map[string]*skillgraph.Atom{"interactive": {Sort: "mode"}, "nightly": {Sort: "mode"}}
		skill.Tier = skillgraph.TierStrict
		model := deriveFixture(t, skill)
		assert.True(t, model.IsStrict)
		assert.Equal(t, VocabularyOf(skill), model.Vocabulary)
	})

	t.Run("atoms-without-tier", func(t *testing.T) {
		skill := importFixture(t, body, "demo")
		skill.Atoms = map[string]*skillgraph.Atom{"interactive": {Sort: "mode"}, "nightly": {Sort: "mode"}}
		model := deriveFixture(t, skill)
		assert.False(t, model.IsStrict)
	})

	t.Run("replaces-range-expanded-deduped", func(t *testing.T) {
		model := deriveFixture(t, importFixture(t, body, "demo"))
		byId := map[string]Section{}
		for _, section := range model.Sections {
			byId[section.Id] = section
		}
		assert.Equal(t, []string{"SKILL-DEMO-PROC-001", "SKILL-DEMO-PROC-002"}, byId["nightly-route"].Replaces)
		assert.Empty(t, byId["nightly-route"].Between)
	})

	t.Run("quality-section", func(t *testing.T) {
		model := deriveFixture(t, importFixture(t, body, "demo"))
		require.Len(t, model.Soft, 1)
		assert.Equal(t, "round", model.Soft[0].Section)
	})
}

func TestExpandReplaces(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		items []string
	}

	entries := []skillgraph.EntryRef{
		{Entry: &skillgraph.Entry{Id: "SKILL-DEMO-PROC-001", Role: skillgraph.RoleStep}},
		{Entry: &skillgraph.Entry{Id: "SKILL-DEMO-TERM-001", Role: skillgraph.RoleDef}},
		{Entry: &skillgraph.Entry{Id: "SKILL-DEMO-PROC-002", Role: skillgraph.RoleStep}},
		{Entry: &skillgraph.Entry{Id: "SKILL-DEMO-TPL-001", Role: skillgraph.RolePayload}},
		{Entry: &skillgraph.Entry{Id: "SKILL-DEMO-PROC-003", Role: skillgraph.RoleStop}},
	}
	tests := make([]*testCase, 0)

	// no-items
	tests = append(tests, &testCase{_id: "no-items", _expected: nil, items: nil})

	// single-item
	tests = append(tests, &testCase{_id: "single-item", _expected: []string{"SKILL-DEMO-PROC-002"}, items: []string{"SKILL-DEMO-PROC-002"}})

	// range-skips-def-and-payload
	tests = append(tests, &testCase{
		_id:       "range-skips-def-and-payload",
		_expected: []string{"SKILL-DEMO-PROC-001", "SKILL-DEMO-PROC-002", "SKILL-DEMO-PROC-003"},
		items:     []string{"SKILL-DEMO-PROC-001..SKILL-DEMO-PROC-003"},
	})

	// overlapping-items-deduped
	tests = append(tests, &testCase{
		_id:       "overlapping-items-deduped",
		_expected: []string{"SKILL-DEMO-PROC-001", "SKILL-DEMO-PROC-002"},
		items:     []string{"SKILL-DEMO-PROC-001..SKILL-DEMO-PROC-002", "SKILL-DEMO-PROC-002"},
	})

	// document-order-kept
	tests = append(tests, &testCase{
		_id:       "document-order-kept",
		_expected: []string{"SKILL-DEMO-PROC-001", "SKILL-DEMO-PROC-003"},
		items:     []string{"SKILL-DEMO-PROC-003", "SKILL-DEMO-PROC-001"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, expandReplaces(entries, test.items))
		})
	}
}
