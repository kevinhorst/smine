package skillmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

const scenarioFixtureBody = "## Args\n\n" +
	"| Arg | Form | Default | Effect |\n" +
	"| :-- | :--- | :------ | :----- |\n" +
	"| `handoff` | flag | absent | renders the plan |\n" +
	"| `--nightly` | flag | absent | headless mode |\n" +
	"| `--trust` | flag | absent | skips the code gate |\n" +
	"| `master` | flag | absent | splits the plan |\n" +
	"| `derived` | flag | absent | plans one split row |\n\n" +
	"## Main\n\n" +
	"**SKILL-DEMO-MAIN-001** `[step]` **Run main**\n\n" +
	"RUN the main step.\n"

// evaluateFixture derives a skill whose main phase holds one step guarded by
// stepPred (unguarded when empty) and one step both routes replace, the first
// route guarded by firstPred, the second by secondPred; it returns the model
// and its inputs: session:fast and arg:high.
func evaluateFixture(t *testing.T, firstPred, secondPred, stepPred string) (*Model, []*Input) {
	t.Helper()
	stepClause := ""
	if stepPred != "" {
		stepClause = "`{pred: " + stepPred + "}`\\\n"
	}
	body := "## Args\n\n" +
		"| Arg | Form | Default | Effect |\n" +
		"| :-- | :--- | :------ | :----- |\n" +
		"| `high` | flag | absent | raises the level |\n\n" +
		"## Main\n\n" +
		stepClause +
		"**SKILL-DEMO-MAIN-001** `[step]` **Run main**\n\n" +
		"RUN the main step.\n\n" +
		"`{post: ready}`\\\n" +
		"**SKILL-DEMO-MAIN-002** `[step]` **Replaced step**\n\n" +
		"RUN the replaced step.\n\n" +
		"## First route\n" +
		"`{section: route, pred: " + firstPred + ", replaces: MAIN-002}`\n\n" +
		"`{pred: loose}`\\\n" +
		"**SKILL-DEMO-FIRST-001** `[rule]` **First rule**\n\n" +
		"The first route MUST stay loose.\n\n" +
		"`{pred: level != high}`\\\n" +
		"**SKILL-DEMO-FIRST-002** `[step]` **First step**\n\n" +
		"RUN the first step.\n\n" +
		"## Second route\n" +
		"`{section: route, pred: " + secondPred + ", replaces: MAIN-002}`\n\n" +
		"**SKILL-DEMO-SECOND-001** `[step]` **Second step**\n\n" +
		"RUN the second step.\n"
	skill := importFixture(t, body, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{
		"fast":  {Sort: "mode", Source: "session"},
		"level": {Kind: skillgraph.KindAtomEnum, Members: []string{"low", "high"}, Sort: "mode", Source: "arg: high => high"},
		"loose": {Sort: "state"},
	}
	return deriveFixture(t, skill), Inputs(skill)
}

// flipFixture derives the scenario fixture with the shared mode atom and the
// local plan_mode, role and trust_arg inputs; it returns the model and its
// inputs in atom id order: arg:--nightly, session:plan_mode, arg:master,
// arg:derived, arg:--trust.
func flipFixture(t *testing.T) (*Model, []*Input) {
	t.Helper()
	skill := importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{
		"plan_mode": {Sort: "mode", Source: "session"},
		"role":      {Kind: skillgraph.KindAtomEnum, Members: []string{"single", "master", "derived"}, Sort: "state", Source: "arg: master => master | arg: derived => derived"},
		"trust_arg": {Sort: "mode", Source: "arg: --trust"},
	}
	skill.Vocabulary = sharedModeVocabulary()
	return deriveFixture(t, skill), Inputs(skill)
}

// sharedModeVocabulary is the shared vocabulary's mode atom as
// skills/vocabulary.cue declares it.
func sharedModeVocabulary() *skillgraph.Vocabulary {
	vocab := &skillgraph.Vocabulary{
		Atoms: map[string]*skillgraph.Atom{
			"mode": {Kind: skillgraph.KindAtomEnum, Members: []string{"interactive", "nightly"}, Sort: "mode", Source: "session | arg: nightly => nightly"},
		},
	}
	return vocab
}

func TestEvaluate(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *ScenarioResult

		model    *Model
		scenario *Scenario
	}

	const (
		mainStep     = "SKILL-DEMO-MAIN-001"
		replacedStep = "SKILL-DEMO-MAIN-002"
		firstRule    = "SKILL-DEMO-FIRST-001"
		firstStep    = "SKILL-DEMO-FIRST-002"
		secondStep   = "SKILL-DEMO-SECOND-001"
	)
	tests := make([]*testCase, 0)

	// bool-input-off
	model, inputs := evaluateFixture(t, "not fast", "fast", "")
	tests = append(tests, &testCase{
		_id: "bool-input-off",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// bool-input-on
	model, inputs = evaluateFixture(t, "not fast", "fast", "")
	tests = append(tests, &testCase{
		_id: "bool-input-on",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "true", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusInactive, firstStep: StatusInactive, secondStep: StatusActive},
			On:       []string{"session:fast"},
			Replaced: map[string]string{replacedStep: "second-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusInactive, "second-route": StatusActive},
		},

		model:    model,
		scenario: NewScenario(inputs, []string{"session:fast"}),
	})

	// unsourced-atom-unknown
	model, inputs = evaluateFixture(t, "not fast", "fast", "loose")
	tests = append(tests, &testCase{
		_id: "unsourced-atom-unknown",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusUnknown, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// granted-atom-unknown
	model, inputs = evaluateFixture(t, "not fast", "fast", "ready")
	tests = append(tests, &testCase{
		_id: "granted-atom-unknown",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusUnknown, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// and-with-false-conjunct-inactive
	model, inputs = evaluateFixture(t, "not fast", "fast", "fast and loose")
	tests = append(tests, &testCase{
		_id: "and-with-false-conjunct-inactive",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusInactive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// and-with-unknown-and-true-unknown
	model, inputs = evaluateFixture(t, "not fast", "fast", "not fast and loose")
	tests = append(tests, &testCase{
		_id: "and-with-unknown-and-true-unknown",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusUnknown, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// or-with-true-disjunct-active
	model, inputs = evaluateFixture(t, "not fast", "fast", "loose or not fast")
	tests = append(tests, &testCase{
		_id: "or-with-true-disjunct-active",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// implication-with-false-antecedent-active
	model, inputs = evaluateFixture(t, "not fast", "fast", "fast => loose")
	tests = append(tests, &testCase{
		_id: "implication-with-false-antecedent-active",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// enum-default-member
	model, inputs = evaluateFixture(t, "not fast", "fast", "level = high")
	tests = append(tests, &testCase{
		_id: "enum-default-member",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusInactive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// enum-input-on
	model, inputs = evaluateFixture(t, "not fast", "fast", "level = high")
	tests = append(tests, &testCase{
		_id: "enum-input-on",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "high"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusInactive, secondStep: StatusInactive},
			On:       []string{"arg:high"},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, []string{"arg:high"}),
	})

	// int-comparison-unknown
	model, inputs = evaluateFixture(t, "not fast", "fast", "count > 1")
	tests = append(tests, &testCase{
		_id: "int-comparison-unknown",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusUnknown, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// transition-in-inactive-route
	model, inputs = evaluateFixture(t, "level = high", "fast", "")
	tests = append(tests, &testCase{
		_id: "transition-in-inactive-route",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusActive, firstRule: StatusInactive, firstStep: StatusInactive, secondStep: StatusInactive},
			Replaced: map[string]string{},
			States:   map[string]string{"main": StatusActive, "first-route": StatusInactive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// invariant-in-inactive-route
	model, inputs = evaluateFixture(t, "fast", "not fast", "")
	tests = append(tests, &testCase{
		_id: "invariant-in-inactive-route",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusInactive, firstStep: StatusInactive, secondStep: StatusActive},
			Replaced: map[string]string{replacedStep: "second-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusInactive, "second-route": StatusActive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// replaced-entry-inactive
	model, inputs = evaluateFixture(t, "not fast", "level = high", "")
	tests = append(tests, &testCase{
		_id: "replaced-entry-inactive",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusInactive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// two-active-routes-first-wins
	model, inputs = evaluateFixture(t, "not fast", "level != high", "")
	tests = append(tests, &testCase{
		_id: "two-active-routes-first-wins",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "low"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusActive, secondStep: StatusActive},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusActive},
		},

		model:    model,
		scenario: NewScenario(inputs, nil),
	})

	// empty-pred-active
	model, inputs = evaluateFixture(t, "not fast", "fast", "")
	tests = append(tests, &testCase{
		_id: "empty-pred-active",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "false", "level": "high"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusActive, firstStep: StatusInactive, secondStep: StatusInactive},
			On:       []string{"arg:high"},
			Replaced: map[string]string{replacedStep: "first-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusActive, "second-route": StatusInactive},
		},

		model:    model,
		scenario: NewScenario(inputs, []string{"arg:high"}),
	})

	// atoms-carry-true-and-member
	model, inputs = evaluateFixture(t, "not fast", "fast", "")
	tests = append(tests, &testCase{
		_id: "atoms-carry-true-and-member",
		_expected: &ScenarioResult{
			Atoms:    map[string]string{"fast": "true", "level": "high"},
			Entries:  map[string]string{mainStep: StatusActive, replacedStep: StatusInactive, firstRule: StatusInactive, firstStep: StatusInactive, secondStep: StatusActive},
			On:       []string{"session:fast", "arg:high"},
			Replaced: map[string]string{replacedStep: "second-route"},
			States:   map[string]string{"main": StatusActive, "first-route": StatusInactive, "second-route": StatusActive},
		},

		model:    model,
		scenario: NewScenario(inputs, []string{"arg:high", "session:fast"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Evaluate(test.model, test.scenario))
		})
	}
}

func TestInputs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []*Input

		skill *skillgraph.Skill
	}

	tests := make([]*testCase, 0)

	// arg-exact-name
	skill := importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"handoff_arg": {Sort: "mode", Source: "arg: handoff"}}
	tests = append(tests, &testCase{
		_id:       "arg-exact-name",
		_expected: []*Input{{Id: "arg:handoff", Atom: "handoff_arg", Label: "handoff", Source: InputSourceArg}},

		skill: skill,
	})

	// arg-undashed-matches-dashed-row
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"nightly_arg": {Sort: "mode", Source: "arg: nightly"}}
	tests = append(tests, &testCase{
		_id:       "arg-undashed-matches-dashed-row",
		_expected: []*Input{{Id: "arg:--nightly", Atom: "nightly_arg", Label: "--nightly", Source: InputSourceArg}},

		skill: skill,
	})

	// arg-dashed-matches-dashed-row
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"trust_arg": {Sort: "mode", Source: "arg: --trust"}}
	tests = append(tests, &testCase{
		_id:       "arg-dashed-matches-dashed-row",
		_expected: []*Input{{Id: "arg:--trust", Atom: "trust_arg", Label: "--trust", Source: InputSourceArg}},

		skill: skill,
	})

	// arg-without-row
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"ghost_arg": {Sort: "mode", Source: "arg: ghost"}}
	tests = append(tests, &testCase{
		_id:       "arg-without-row",
		_expected: nil,

		skill: skill,
	})

	// enum-arg-carries-member
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{
		"role": {Kind: skillgraph.KindAtomEnum, Members: []string{"single", "master", "derived"}, Sort: "state", Source: "arg: master => master | arg: derived => derived"},
	}
	tests = append(tests, &testCase{
		_id: "enum-arg-carries-member",
		_expected: []*Input{
			{Id: "arg:master", Atom: "role", Label: "master", Member: "master", Source: InputSourceArg},
			{Id: "arg:derived", Atom: "role", Label: "derived", Member: "derived", Source: InputSourceArg},
		},

		skill: skill,
	})

	// bool-session
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"plan_mode": {Sort: "mode", Source: "session"}}
	tests = append(tests, &testCase{
		_id:       "bool-session",
		_expected: []*Input{{Id: "session:plan_mode", Atom: "plan_mode", Label: "plan_mode", Source: InputSourceSession}},

		skill: skill,
	})

	// enum-session-only
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{
		"tone": {Kind: skillgraph.KindAtomEnum, Members: []string{"calm", "terse"}, Sort: "mode", Source: "session"},
	}
	tests = append(tests, &testCase{
		_id: "enum-session-only",
		_expected: []*Input{
			{Id: "session:tone=calm", Atom: "tone", Label: "tone = calm", Member: "calm", Source: InputSourceSession},
			{Id: "session:tone=terse", Atom: "tone", Label: "tone = terse", Member: "terse", Source: InputSourceSession},
		},

		skill: skill,
	})

	// enum-arg-and-session
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Vocabulary = sharedModeVocabulary()
	tests = append(tests, &testCase{
		_id:       "enum-arg-and-session",
		_expected: []*Input{{Id: "arg:--nightly", Atom: "mode", Label: "--nightly", Member: "nightly", Source: InputSourceArg}},

		skill: skill,
	})

	// atom-without-source
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"loose": {Sort: "state"}}
	tests = append(tests, &testCase{
		_id:       "atom-without-source",
		_expected: nil,

		skill: skill,
	})

	// runtime-part-no-tile
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"review_done": {Sort: "state", Source: "runtime: ledger.void = true"}}
	tests = append(tests, &testCase{
		_id:       "runtime-part-no-tile",
		_expected: nil,

		skill: skill,
	})

	// environment-part-no-tile
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{"in_docker": {Sort: "state", Source: "environment"}}
	tests = append(tests, &testCase{
		_id:       "environment-part-no-tile",
		_expected: nil,

		skill: skill,
	})

	// order-by-atom-id
	skill = importFixture(t, scenarioFixtureBody, "demo")
	skill.Atoms = map[string]*skillgraph.Atom{
		"alpha_arg": {Sort: "mode", Source: "arg: --trust"},
		"zeta_arg":  {Sort: "mode", Source: "arg: handoff"},
	}
	tests = append(tests, &testCase{
		_id: "order-by-atom-id",
		_expected: []*Input{
			{Id: "arg:--trust", Atom: "alpha_arg", Label: "--trust", Source: InputSourceArg},
			{Id: "arg:handoff", Atom: "zeta_arg", Label: "handoff", Source: InputSourceArg},
		},

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Inputs(test.skill))
		})
	}
}

func TestScenario_Flip(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedFlipped bool
		_expectedOn      []string

		model    *Model
		pred     string
		scenario *Scenario
	}

	model, inputs := flipFixture(t)
	tests := make([]*testCase, 0)

	// bool-literal-off-to-on
	tests = append(tests, &testCase{
		_id:              "bool-literal-off-to-on",
		_expectedFlipped: true,
		_expectedOn:      []string{"session:plan_mode"},

		model:    model,
		pred:     "plan_mode",
		scenario: NewScenario(inputs, nil),
	})

	// bool-literal-on-to-off
	tests = append(tests, &testCase{
		_id:              "bool-literal-on-to-off",
		_expectedFlipped: true,
		_expectedOn:      nil,

		model:    model,
		pred:     "plan_mode",
		scenario: NewScenario(inputs, []string{"session:plan_mode"}),
	})

	// negated-literal-holding-sets-atom-on
	tests = append(tests, &testCase{
		_id:              "negated-literal-holding-sets-atom-on",
		_expectedFlipped: true,
		_expectedOn:      []string{"session:plan_mode"},

		model:    model,
		pred:     "not plan_mode",
		scenario: NewScenario(inputs, nil),
	})

	// enum-eq-member-off-to-on
	tests = append(tests, &testCase{
		_id:              "enum-eq-member-off-to-on",
		_expectedFlipped: true,
		_expectedOn:      []string{"arg:master"},

		model:    model,
		pred:     "role = master",
		scenario: NewScenario(inputs, nil),
	})

	// enum-ne-member-on-to-off
	tests = append(tests, &testCase{
		_id:              "enum-ne-member-on-to-off",
		_expectedFlipped: true,
		_expectedOn:      nil,

		model:    model,
		pred:     "role != master",
		scenario: NewScenario(inputs, []string{"arg:master"}),
	})

	// conjunction-not-holding-sets-every-literal
	tests = append(tests, &testCase{
		_id:              "conjunction-not-holding-sets-every-literal",
		_expectedFlipped: true,
		_expectedOn:      []string{"session:plan_mode", "arg:--trust"},

		model:    model,
		pred:     "plan_mode and ready and trust_arg",
		scenario: NewScenario(inputs, nil),
	})

	// conjunction-holding-falsifies-first-literal
	tests = append(tests, &testCase{
		_id:              "conjunction-holding-falsifies-first-literal",
		_expectedFlipped: true,
		_expectedOn:      []string{"arg:--trust"},

		model:    model,
		pred:     "plan_mode and trust_arg",
		scenario: NewScenario(inputs, []string{"session:plan_mode", "arg:--trust"}),
	})

	// trust-and-not-nightly-holding-turns-trust-off
	tests = append(tests, &testCase{
		_id:              "trust-and-not-nightly-holding-turns-trust-off",
		_expectedFlipped: true,
		_expectedOn:      nil,

		model:    model,
		pred:     "trust_arg and mode != nightly",
		scenario: NewScenario(inputs, []string{"arg:--trust"}),
	})

	// disjunction-unchanged
	tests = append(tests, &testCase{
		_id:              "disjunction-unchanged",
		_expectedFlipped: false,
		_expectedOn:      []string{"arg:--trust"},

		model:    model,
		pred:     "plan_mode or trust_arg",
		scenario: NewScenario(inputs, []string{"arg:--trust"}),
	})

	// implication-unchanged
	tests = append(tests, &testCase{
		_id:              "implication-unchanged",
		_expectedFlipped: false,
		_expectedOn:      nil,

		model:    model,
		pred:     "plan_mode => trust_arg",
		scenario: NewScenario(inputs, nil),
	})

	// granted-atom-only-unchanged
	tests = append(tests, &testCase{
		_id:              "granted-atom-only-unchanged",
		_expectedFlipped: false,
		_expectedOn:      nil,

		model:    model,
		pred:     "ready",
		scenario: NewScenario(inputs, nil),
	})

	// unparsable-pred-unchanged
	tests = append(tests, &testCase{
		_id:              "unparsable-pred-unchanged",
		_expectedFlipped: false,
		_expectedOn:      nil,

		model:    model,
		pred:     "plan_mode and",
		scenario: NewScenario(inputs, nil),
	})

	// enum-member-flip-switches-sibling-off
	tests = append(tests, &testCase{
		_id:              "enum-member-flip-switches-sibling-off",
		_expectedFlipped: true,
		_expectedOn:      []string{"arg:derived"},

		model:    model,
		pred:     "role = derived",
		scenario: NewScenario(inputs, []string{"arg:master"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedFlipped, test.scenario.Flip(test.model, test.pred))
			assert.Equal(t, test._expectedOn, test.scenario.OnIds())
		})
	}
}

func TestScenario_Set(t *testing.T) {
	type testCase struct {
		_id         string
		_expectedOn []string

		id       string
		isOn     bool
		scenario *Scenario
	}

	inputs := []*Input{
		{Id: "arg:handoff", Atom: "handoff_arg", Label: "handoff", Source: InputSourceArg},
		{Id: "arg:master", Atom: "role", Label: "master", Member: "master", Source: InputSourceArg},
		{Id: "arg:derived", Atom: "role", Label: "derived", Member: "derived", Source: InputSourceArg},
		{Id: "arg:--trust", Atom: "trust_arg", Label: "--trust", Source: InputSourceArg},
	}
	tests := make([]*testCase, 0)

	// on
	tests = append(tests, &testCase{
		_id:         "on",
		_expectedOn: []string{"arg:--trust"},

		id:       "arg:--trust",
		isOn:     true,
		scenario: NewScenario(inputs, nil),
	})

	// on-then-off
	tests = append(tests, &testCase{
		_id:         "on-then-off",
		_expectedOn: nil,

		id:       "arg:--trust",
		isOn:     false,
		scenario: NewScenario(inputs, []string{"arg:--trust"}),
	})

	// unknown-id-ignored
	tests = append(tests, &testCase{
		_id:         "unknown-id-ignored",
		_expectedOn: []string{"arg:handoff"},

		id:       "arg:ghost",
		isOn:     true,
		scenario: NewScenario(inputs, []string{"arg:handoff", "arg:ghost"}),
	})

	// enum-member-on-switches-sibling-off
	tests = append(tests, &testCase{
		_id:         "enum-member-on-switches-sibling-off",
		_expectedOn: []string{"arg:derived"},

		id:       "arg:derived",
		isOn:     true,
		scenario: NewScenario(inputs, []string{"arg:master"}),
	})

	// on-ids-keep-input-order
	tests = append(tests, &testCase{
		_id:         "on-ids-keep-input-order",
		_expectedOn: []string{"arg:handoff", "arg:master", "arg:--trust"},

		id:       "arg:master",
		isOn:     true,
		scenario: NewScenario(inputs, []string{"arg:--trust", "arg:handoff"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.scenario.Set(test.id, test.isOn)
			assert.Equal(t, test._expectedOn, test.scenario.OnIds())
		})
	}
}
