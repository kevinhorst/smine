package sessiontitle

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestParseTitle(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Title

		text       string
		vocabulary *Vocabulary
	}

	deployed := provideVocabulary(t, map[string]string{"fdesign": fdesignDocument, "railroad-review": railroadReviewDocument})
	fdesignOnly := provideVocabulary(t, map[string]string{"fdesign": fdesignDocument})

	tests := make([]*testCase, 0)

	// full-with-fleet
	tests = append(tests, &testCase{
		_id:       "full-with-fleet",
		_expected: &Title{Ai: "Quick Question Skill Creation", Fleet: "T1-MA", Repo: "CC", Skill: "DD", State: StateWip},

		text:       "CC,DD - Quick Question Skill Creation - T1-MA[WIP]",
		vocabulary: deployed,
	})

	// no-fleet
	tests = append(tests, &testCase{
		_id:       "no-fleet",
		_expected: &Title{Ai: "Worktree script diagnosis", Fleet: "", Repo: "CC", Skill: "FD", State: StateReady},

		text:       "CC,FD - Worktree script diagnosis - [READY]",
		vocabulary: deployed,
	})

	// no-skill
	tests = append(tests, &testCase{
		_id:       "no-skill",
		_expected: &Title{Ai: "Railroad Review V11 Bump", Fleet: "", Repo: "CC", Skill: "", State: StateDone},

		text:       "CC - Railroad Review V11 Bump - [DONE]",
		vocabulary: deployed,
	})

	// closed
	tests = append(tests, &testCase{
		_id:       "closed",
		_expected: &Title{Ai: "Railroad Review V11 Bump", Fleet: "T1-MB", Repo: "CC", Skill: "", State: StateClosed},

		text:       "CC - Railroad Review V11 Bump - T1-MB[CLOSED]",
		vocabulary: deployed,
	})

	// comma-space-skill
	tests = append(tests, &testCase{
		_id:       "comma-space-skill",
		_expected: &Title{Ai: "CLAUDE.md deprecation", Fleet: "", Repo: "CC", Skill: "FD", State: StateWip},

		text:       "CC, FD - CLAUDE.md deprecation - [WIP]",
		vocabulary: deployed,
	})

	// ai-containing-dash
	tests = append(tests, &testCase{
		_id:       "ai-containing-dash",
		_expected: &Title{Ai: "Smine skill pipeline debugging - Continuation", Fleet: "", Repo: "CC", Skill: "DD", State: StateWip},

		text:       "CC, DD - Smine skill pipeline debugging - Continuation - [WIP]",
		vocabulary: deployed,
	})

	// legacy-token-active
	tests = append(tests, &testCase{
		_id:       "legacy-token-active",
		_expected: &Title{Ai: "Checklist", Fleet: "", Repo: "CC", Skill: "FD", State: StateNone},

		text:       "CC,FD - Checklist - [ACTIVE]",
		vocabulary: deployed,
	})

	// plain-auto-title
	tests = append(tests, &testCase{
		_id:       "plain-auto-title",
		_expected: &Title{Ai: "Session title state structure", Fleet: "", Repo: "", Skill: "", State: StateNone},

		text:       " Session title state structure ",
		vocabulary: deployed,
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: &Title{Ai: "", Fleet: "", Repo: "", Skill: "", State: StateNone},

		text:       "",
		vocabulary: deployed,
	})

	// declared-state
	tests = append(tests, &testCase{
		_id:       "declared-state",
		_expected: &Title{Ai: "Session title gate states", Fleet: "", Repo: "CC", Skill: "FD", State: "DESIGN-READY"},

		text:       "CC,FD - Session title gate states - [DESIGN-READY]",
		vocabulary: deployed,
	})

	// declared-state-with-fleet
	tests = append(tests, &testCase{
		_id:       "declared-state-with-fleet",
		_expected: &Title{Ai: "Review round two", Fleet: "T1-MA", Repo: "CC", Skill: "RR", State: "HUMAN-GATE"},

		text:       "CC,RR - Review round two - T1-MA[HUMAN-GATE]",
		vocabulary: deployed,
	})

	// undeclared-hyphen-token
	tests = append(tests, &testCase{
		_id:       "undeclared-hyphen-token",
		_expected: &Title{Ai: "Session title gate states", Fleet: "", Repo: "CC", Skill: "FD", State: StateNone},

		text:       "CC,FD - Session title gate states - [PLAN-READY]",
		vocabulary: deployed,
	})

	// declared-token-of-undeployed-skill
	tests = append(tests, &testCase{
		_id:       "declared-token-of-undeployed-skill",
		_expected: &Title{Ai: "Review round two", Fleet: "", Repo: "CC", Skill: "RR", State: StateNone},

		text:       "CC,RR - Review round two - [HUMAN-GATE]",
		vocabulary: fdesignOnly,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ParseTitle(test.text, test.vocabulary))
		})
	}
}

func TestState_IsBuiltin(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		state State
	}

	tests := make([]*testCase, 0)

	// none
	tests = append(tests, &testCase{_id: "none", _expected: true, state: StateNone})

	// closed
	tests = append(tests, &testCase{_id: "closed", _expected: true, state: StateClosed})

	// done
	tests = append(tests, &testCase{_id: "done", _expected: true, state: StateDone})

	// ready
	tests = append(tests, &testCase{_id: "ready", _expected: true, state: StateReady})

	// testing
	tests = append(tests, &testCase{_id: "testing", _expected: true, state: StateTesting})

	// wip
	tests = append(tests, &testCase{_id: "wip", _expected: true, state: StateWip})

	// declared-state
	tests = append(tests, &testCase{_id: "declared-state", _expected: false, state: "DESIGN-READY"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.state.IsBuiltin())
		})
	}
}

func TestState_Transitions(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedClear State
		_expectedClose State
		_expectedDone  State
		_expectedJoin  State
		_expectedSet   State

		state State
	}

	// The set target: one of fdesign's states; clear is a set of WIP.
	target := State("CODE-REVIEWABLE")

	tests := make([]*testCase, 0)

	// none
	tests = append(tests, &testCase{_id: "none", _expectedClear: StateWip, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: StateWip, _expectedSet: target, state: StateNone})

	// wip
	tests = append(tests, &testCase{_id: "wip", _expectedClear: StateWip, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: StateWip, _expectedSet: target, state: StateWip})

	// ready
	tests = append(tests, &testCase{_id: "ready", _expectedClear: StateWip, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: StateReady, _expectedSet: target, state: StateReady})

	// testing
	tests = append(tests, &testCase{_id: "testing", _expectedClear: StateTesting, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: StateTesting, _expectedSet: StateTesting, state: StateTesting})

	// done
	tests = append(tests, &testCase{_id: "done", _expectedClear: StateDone, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: StateDone, _expectedSet: StateDone, state: StateDone})

	// closed
	tests = append(tests, &testCase{_id: "closed", _expectedClear: StateClosed, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: StateClosed, _expectedSet: StateClosed, state: StateClosed})

	// declared
	tests = append(tests, &testCase{_id: "declared", _expectedClear: StateWip, _expectedClose: StateClosed, _expectedDone: StateDone, _expectedJoin: "DESIGN-READY", _expectedSet: target, state: "DESIGN-READY"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedClear, test.state.AfterSet(StateWip))
			assert.Equal(t, test._expectedClose, test.state.AfterClose())
			assert.Equal(t, test._expectedDone, test.state.AfterDone())
			assert.Equal(t, test._expectedJoin, test.state.AfterJoin())
			assert.Equal(t, test._expectedSet, test.state.AfterSet(target))
		})
	}
}

func TestTitle_Label(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		title *Title
	}

	tests := make([]*testCase, 0)

	// with-fleet
	tests = append(tests, &testCase{
		_id:       "with-fleet",
		_expected: "CC,DD - Quick Question Skill Creation - T1-MA",

		title: &Title{Ai: "Quick Question Skill Creation", Fleet: "T1-MA", Repo: "CC", Skill: "DD", State: StateWip},
	})

	// no-fleet
	tests = append(tests, &testCase{
		_id:       "no-fleet",
		_expected: "CC,FD - Session title gate states",

		title: &Title{Ai: "Session title gate states", Fleet: "", Repo: "CC", Skill: "FD", State: "DESIGN-READY"},
	})

	// no-skill
	tests = append(tests, &testCase{
		_id:       "no-skill",
		_expected: "CC - Railroad Review V11 Bump",

		title: &Title{Ai: "Railroad Review V11 Bump", Fleet: "", Repo: "CC", Skill: "", State: StateDone},
	})

	// unformatted
	tests = append(tests, &testCase{
		_id:       "unformatted",
		_expected: "Session title state structure",

		title: &Title{Ai: "Session title state structure", Fleet: "", Repo: "", Skill: "", State: StateNone},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.title.Label())
		})
	}
}

func TestTitle_Render(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		title *Title
	}

	deployed := provideVocabulary(t, map[string]string{"fdesign": fdesignDocument, "railroad-review": railroadReviewDocument})

	tests := make([]*testCase, 0)

	// request-example
	tests = append(tests, &testCase{
		_id:       "request-example",
		_expected: "CC,DD - Quick Question Skill Creation - T1-MA[WIP]",

		title: &Title{Ai: "Quick Question Skill Creation", Fleet: "T1-MA", Repo: "CC", Skill: "DD", State: StateWip},
	})

	// no-fleet
	tests = append(tests, &testCase{
		_id:       "no-fleet",
		_expected: "CC,FD - Session title state structure - [READY]",

		title: &Title{Ai: "Session title state structure", Fleet: "", Repo: "CC", Skill: "FD", State: StateReady},
	})

	// no-skill
	tests = append(tests, &testCase{
		_id:       "no-skill",
		_expected: "CC - Session title state structure - [WIP]",

		title: &Title{Ai: "Session title state structure", Fleet: "", Repo: "CC", Skill: "", State: StateWip},
	})

	// exactly-62
	tests = append(tests, &testCase{
		_id:       "exactly-62",
		_expected: "CC,DD - Smine skill pipeline debugging - Continuation - [WIP]",

		title: &Title{Ai: "Smine skill pipeline debugging - Continuation", Fleet: "", Repo: "CC", Skill: "DD", State: StateWip},
	})

	// over-length-word-cut
	tests = append(tests, &testCase{
		_id:       "over-length-word-cut",
		_expected: "CC,FImpl - Railroad Review adjustments for… - T1-MA[READY]",

		title: &Title{Ai: "Railroad Review adjustments for the lane context budget", Fleet: "T1-MA", Repo: "CC", Skill: "FImpl", State: StateReady},
	})

	// no-space-in-budget
	tests = append(tests, &testCase{
		_id:       "no-space-in-budget",
		_expected: "CC,FD - Supercalifragilisticexpialidociousxxxxxxxxxxx… - [WIP]",

		title: &Title{Ai: "Supercalifragilisticexpialidociousxxxxxxxxxxxxxxxxxxxxxxxxxxx", Fleet: "", Repo: "CC", Skill: "FD", State: StateWip},
	})

	// declared-token-shortens-ai
	tests = append(tests, &testCase{
		_id:       "declared-token-shortens-ai",
		_expected: "CC,FD - Railroad Review adjustments… - T1-MA[CODE-REVIEWABLE]",

		title: &Title{Ai: "Railroad Review adjustments for the lane context budget", Fleet: "T1-MA", Repo: "CC", Skill: "FD", State: "CODE-REVIEWABLE"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered := test.title.Render(testLimits)
			assert.Equal(t, test._expected, rendered)
			assert.LessOrEqual(t, utf8.RuneCountInString(rendered), testLimits.MaxTitleRunes)
			assert.Equal(t, rendered, ParseTitle(rendered, deployed).Render(testLimits))
		})
	}
}
