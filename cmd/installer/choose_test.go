package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/skills"
)

func provideComponent(t *testing.T, id components.ID) components.Component {
	t.Helper()
	component, isFound := components.Lookup(id)
	require.True(t, isFound, "component %s", id)
	return component
}

func TestAskSkills(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *choice
		_shouldPass bool

		ids        []components.ID
		isFound    bool
		loadSkills func() ([]skills.Skill, error)
		prompter   *prompter
		stored     install.Record
	}

	loadSkills := func() ([]skills.Skill, error) {
		list := []skills.Skill{
			{Name: "concept"},
			{Name: "fdesign"},
			{Name: "peek", Requires: []components.ID{components.Peek}},
		}
		return list, nil
	}
	withPeek := []components.ID{components.ConfigServer, components.Skills, components.Peek}
	storedChosen := install.Record{Skills: install.Selection{Chosen: []string{"fdesign", "peek", "gone"}, Mode: skills.SelectionModeChosen}}

	tests := make([]*testCase, 0)

	// all-unconfirmed
	tests = append(tests, &testCase{
		_id:         "all-unconfirmed",
		_expected:   &choice{isSkillsAsked: true, isSkillsConfirmed: false, skillsArg: skills.SelectionModeAll},
		_shouldPass: true,

		isFound:    false,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("1\nn\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// none-confirmed
	tests = append(tests, &testCase{
		_id:         "none-confirmed",
		_expected:   &choice{isSkillsAsked: true, isSkillsConfirmed: true, skillsArg: skills.SelectionModeNone},
		_shouldPass: true,

		isFound:    false,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("3\ny\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// chosen-toggled-by-numbers-repo-order
	tests = append(tests, &testCase{
		_id:         "chosen-toggled-by-numbers-repo-order",
		_expected:   &choice{ids: withPeek, isSkillsAsked: true, isSkillsConfirmed: true, skillsArg: skills.SelectionModeChosen + ":concept,peek"},
		_shouldPass: true,

		ids:        withPeek,
		isFound:    false,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("2\n3 1\n\ny\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// chosen-missing-component-refused
	tests = append(tests, &testCase{
		_id:         "chosen-missing-component-refused",
		_expected:   &choice{ids: components.Minimal(), isSkillsAsked: true, isSkillsConfirmed: false, skillsArg: skills.SelectionModeChosen + ":concept"},
		_shouldPass: true,

		ids:        components.Minimal(),
		isFound:    false,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("2\n3 1\n\nn\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// chosen-stored-preselected-unknown-and-unavailable-dropped
	tests = append(tests, &testCase{
		_id:         "chosen-stored-preselected-unknown-and-unavailable-dropped",
		_expected:   &choice{ids: components.Minimal(), isSkillsAsked: true, isSkillsConfirmed: false, skillsArg: skills.SelectionModeChosen + ":fdesign"},
		_shouldPass: true,

		ids:        components.Minimal(),
		isFound:    true,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("\n\nn\n"), &bytes.Buffer{}),
		stored:     storedChosen,
	})

	// chosen-invalid-then-valid
	tests = append(tests, &testCase{
		_id:         "chosen-invalid-then-valid",
		_expected:   &choice{isSkillsAsked: true, isSkillsConfirmed: false, skillsArg: skills.SelectionModeChosen + ":concept"},
		_shouldPass: true,

		isFound:    false,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("2\n9\nx\n1\n\n\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// chosen-empty-all
	tests = append(tests, &testCase{
		_id:         "chosen-empty-all",
		_expected:   &choice{isSkillsAsked: true, isSkillsConfirmed: false, skillsArg: skills.SelectionModeAll},
		_shouldPass: true,

		isFound:    false,
		loadSkills: loadSkills,
		prompter:   newPrompter(nil, strings.NewReader("2\n\n\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// leaves-unreadable
	tests = append(tests, &testCase{
		_id:         "leaves-unreadable",
		_expected:   &choice{isSkillsAsked: true, skillsArg: skills.SelectionModeChosen},
		_shouldPass: false,

		isFound:    false,
		loadSkills: func() ([]skills.Skill, error) { return nil, errors.New("boom") },
		prompter:   newPrompter(nil, strings.NewReader("2\n"), &bytes.Buffer{}),
		stored:     install.Record{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result := &choice{ids: test.ids}
			err := askSkills(test.prompter, result, test.stored, test.isFound, test.loadSkills)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, result)
		})
	}
}

func TestChoose(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *choice
		_shouldPass bool

		goos     string
		isFound  bool
		opts     *options
		prompter *prompter
		repoDir  string
		stored   install.Record
	}

	storedFull := install.Record{
		Components: components.Full("darwin"),
		Preset:     install.PresetFull,
		Skills:     install.Selection{Mode: skills.SelectionModeAll},
	}

	tests := make([]*testCase, 0)

	// components-wins
	tests = append(tests, &testCase{
		_id: "components-wins",
		_expected: &choice{
			componentsArg: "token",
			ids:           []components.ID{components.ConfigServer, components.Token},
			skillsArg:     skills.SelectionModeAll,
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: true,
		opts: &options{
			componentsArg: "token",
			explicit:      map[string]bool{},
			isYes:         true,
			skillsArg:     skills.SelectionModeAll,
		},
		prompter: newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   storedFull,
	})

	// yes-with-record-empty-components-arg
	tests = append(tests, &testCase{
		_id: "yes-with-record-empty-components-arg",
		_expected: &choice{
			componentsArg: "",
			ids:           storedFull.Components,
			skillsArg:     "",
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: true,
		opts: &options{
			explicit: map[string]bool{},
			isYes:    true,
		},
		prompter: newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   storedFull,
	})

	// yes-without-record-minimal
	tests = append(tests, &testCase{
		_id: "yes-without-record-minimal",
		_expected: &choice{
			componentsArg: install.PresetMinimal,
			ids:           components.Minimal(),
			skillsArg:     "",
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: false,
		opts: &options{
			explicit: map[string]bool{},
			isYes:    true,
		},
		prompter: newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   install.Record{},
	})

	// skills-set-skips-the-menu
	tests = append(tests, &testCase{
		_id: "skills-set-skips-the-menu",
		_expected: &choice{
			componentsArg: install.PresetMinimal,
			ids:           components.Minimal(),
			skillsArg:     skills.SelectionModeNone,
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: false,
		opts: &options{
			explicit:  map[string]bool{},
			skillsArg: skills.SelectionModeNone,
		},
		prompter: newPrompter(nil, strings.NewReader("\n"), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   install.Record{},
	})

	// confirmed-selection-skips-the-menu
	storedConfirmed := storedFull
	storedConfirmed.Skills = install.Selection{IsConfirmed: true, Mode: skills.SelectionModeAll}
	tests = append(tests, &testCase{
		_id: "confirmed-selection-skips-the-menu",
		_expected: &choice{
			componentsArg: install.PresetFull,
			ids:           components.Full("darwin"),
			skillsArg:     "",
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: true,
		opts: &options{
			explicit: map[string]bool{},
		},
		prompter: newPrompter(nil, strings.NewReader("2\n"), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   storedConfirmed,
	})

	// reset-asks-again
	tests = append(tests, &testCase{
		_id: "reset-asks-again",
		_expected: &choice{
			componentsArg:     install.PresetFull,
			ids:               components.Full("darwin"),
			isSkillsAsked:     true,
			isSkillsConfirmed: false,
			skillsArg:         skills.SelectionModeNone,
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: true,
		opts: &options{
			explicit:      map[string]bool{},
			isResetSkills: true,
		},
		prompter: newPrompter(nil, strings.NewReader("2\n3\nn\n"), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   storedConfirmed,
	})

	// reset-with-yes-only-clears
	tests = append(tests, &testCase{
		_id: "reset-with-yes-only-clears",
		_expected: &choice{
			componentsArg:     "",
			ids:               storedFull.Components,
			isSkillsAsked:     true,
			isSkillsConfirmed: false,
			skillsArg:         "",
		},
		_shouldPass: true,

		goos:    "darwin",
		isFound: true,
		opts: &options{
			explicit:      map[string]bool{},
			isResetSkills: true,
			isYes:         true,
		},
		prompter: newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		repoDir:  t.TempDir(),
		stored:   storedConfirmed,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := choose(test.prompter, test.opts, test.stored, test.isFound, test.goos, test.repoDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, result)
		})
	}
}

func TestInstallArgs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		opts   *options
		result *choice
	}

	tests := make([]*testCase, 0)

	// only-explicit-port-flags-forwarded
	tests = append(tests, &testCase{
		_id:       "only-explicit-port-flags-forwarded",
		_expected: []string{"-install", "-components", install.PresetMinimal, "-skills", skills.SelectionModeAll, "-peek-port", "5000", "-heartbeat-port", "6102"},

		opts: &options{
			addr:            "127.0.0.1:7001",
			explicit:        map[string]bool{"heartbeat-port": true, "peek-port": true},
			heartbeatPort:   6102,
			initWelcome:     true,
			peekControlPort: 50000,
			peekPort:        5000,
		},
		result: &choice{
			componentsArg: install.PresetMinimal,
			ids:           components.Minimal(),
			skillsArg:     skills.SelectionModeAll,
		},
	})

	// presentation-profile-forwarded
	tests = append(tests, &testCase{
		_id:       "presentation-profile-forwarded",
		_expected: []string{"-install", "-presentation-profile", "dev"},

		opts: &options{
			explicit:            map[string]bool{},
			presentationProfile: "dev",
		},
		result: &choice{},
	})

	// asked-skills-carry-the-confirmation
	tests = append(tests, &testCase{
		_id:       "asked-skills-carry-the-confirmation",
		_expected: []string{"-install", "-skills", skills.SelectionModeChosen + ":concept", "-skills-confirmed=true"},

		opts: &options{
			explicit: map[string]bool{},
		},
		result: &choice{
			isSkillsAsked:     true,
			isSkillsConfirmed: true,
			skillsArg:         skills.SelectionModeChosen + ":concept",
		},
	})

	// reset-without-answer-clears-the-confirmation
	tests = append(tests, &testCase{
		_id:       "reset-without-answer-clears-the-confirmation",
		_expected: []string{"-install", "-skills-confirmed=false"},

		opts: &options{
			explicit: map[string]bool{},
		},
		result: &choice{
			isSkillsAsked: true,
		},
	})

	// empty-selection-parts-omitted
	tests = append(tests, &testCase{
		_id:       "empty-selection-parts-omitted",
		_expected: []string{"-install"},

		opts: &options{
			explicit: map[string]bool{},
		},
		result: &choice{
			componentsArg: "",
			ids:           components.Minimal(),
			skillsArg:     "",
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, installArgs(test.opts, test.result))
		})
	}
}

func TestPrompterChoose(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		choices  []string
		fallback int
		prompter *prompter
	}

	choices := []string{"first", "second", "third"}

	tests := make([]*testCase, 0)

	// enter-fallback
	tests = append(tests, &testCase{
		_id:       "enter-fallback",
		_expected: 1,

		choices:  choices,
		fallback: 1,
		prompter: newPrompter(nil, strings.NewReader("\n"), &bytes.Buffer{}),
	})

	// invalid-then-valid
	tests = append(tests, &testCase{
		_id:       "invalid-then-valid",
		_expected: 1,

		choices:  choices,
		fallback: 0,
		prompter: newPrompter(nil, strings.NewReader("9\nx\n2\n"), &bytes.Buffer{}),
	})

	// end-of-input-fallback
	tests = append(tests, &testCase{
		_id:       "end-of-input-fallback",
		_expected: 2,

		choices:  choices,
		fallback: 2,
		prompter: newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.prompter.choose("Pick", test.choices, test.fallback))
		})
	}
}

func TestParseNumbers(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedIsOk   bool
		_expectedResult []int

		answer string
		count  int
	}

	tests := make([]*testCase, 0)

	// spaces-and-commas
	tests = append(tests, &testCase{
		_id:             "spaces-and-commas",
		_expectedIsOk:   true,
		_expectedResult: []int{3, 7, 12},

		answer: " 3, 7\t12 ",
		count:  12,
	})

	// out-of-range-invalid
	tests = append(tests, &testCase{
		_id:             "out-of-range-invalid",
		_expectedIsOk:   false,
		_expectedResult: nil,

		answer: "1 13",
		count:  12,
	})

	// word-invalid
	tests = append(tests, &testCase{
		_id:             "word-invalid",
		_expectedIsOk:   false,
		_expectedResult: nil,

		answer: "concept",
		count:  12,
	})

	// separators-only-invalid
	tests = append(tests, &testCase{
		_id:             "separators-only-invalid",
		_expectedIsOk:   false,
		_expectedResult: nil,

		answer: " , ",
		count:  12,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			numbers, isOk := parseNumbers(test.answer, test.count)
			assert.Equal(t, test._expectedIsOk, isOk)
			assert.Equal(t, test._expectedResult, numbers)
		})
	}
}

func TestToggleLeaf(t *testing.T) {
	rows := []skillRow{{name: "concept"}, {name: "fdesign"}, {name: "peek"}}

	assert.Equal(t, []string{"concept", "peek"}, toggleLeaf([]string{"peek"}, "concept", rows))
	assert.Equal(t, []string{"peek"}, toggleLeaf([]string{"concept", "peek"}, "concept", rows))
}

func TestToggle(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []components.ID

		component components.Component
		goos      string
		prompter  *prompter
		selected  []components.ID
	}

	withDependents := []components.ID{
		components.ConfigServer,
		components.Skills,
		components.SessionHooks,
		components.Context,
		components.Gate,
	}

	tests := make([]*testCase, 0)

	// fixed-refused
	tests = append(tests, &testCase{
		_id:       "fixed-refused",
		_expected: components.Minimal(),

		component: provideComponent(t, components.ConfigServer),
		goos:      "darwin",
		prompter:  newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		selected:  components.Minimal(),
	})

	// unavailable-refused
	tests = append(tests, &testCase{
		_id:       "unavailable-refused",
		_expected: components.Minimal(),

		component: provideComponent(t, components.Heartbeat),
		goos:      "linux",
		prompter:  newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		selected:  components.Minimal(),
	})

	// tick-closes-requirements
	tests = append(tests, &testCase{
		_id:       "tick-closes-requirements",
		_expected: []components.ID{components.ConfigServer, components.Skills, components.SessionHooks, components.Context, components.Gate},

		component: provideComponent(t, components.Context),
		goos:      "darwin",
		prompter:  newPrompter(nil, strings.NewReader(""), &bytes.Buffer{}),
		selected:  components.Minimal(),
	})

	// untick-with-dependents-answer-y
	tests = append(tests, &testCase{
		_id:       "untick-with-dependents-answer-y",
		_expected: []components.ID{components.ConfigServer, components.Skills},

		component: provideComponent(t, components.SessionHooks),
		goos:      "darwin",
		prompter:  newPrompter(nil, strings.NewReader("y\n"), &bytes.Buffer{}),
		selected:  withDependents,
	})

	// untick-with-dependents-answer-n
	tests = append(tests, &testCase{
		_id:       "untick-with-dependents-answer-n",
		_expected: withDependents,

		component: provideComponent(t, components.SessionHooks),
		goos:      "darwin",
		prompter:  newPrompter(nil, strings.NewReader("n\n"), &bytes.Buffer{}),
		selected:  withDependents,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, toggle(test.prompter, test.selected, test.component, test.goos))
		})
	}
}
