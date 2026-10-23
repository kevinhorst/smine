package components

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// constOrder is the const block's id order, which the display order mirrors.
var constOrder = []ID{
	ConfigServer,
	Skills,
	SessionHooks,
	SkillEnvironment,
	Context,
	Gate,
	Worktrees,
	SessionHelpers,
	GlobalInstructions,
	Agentwt,
	Peek,
	Ccusage,
	Heartbeat,
	Routines,
	Token,
	Mining,
}

// catalogIds lists the catalog ids in display order.
func catalogIds() []ID {
	ids := make([]ID, 0, len(catalog))
	for _, component := range Catalog() {
		ids = append(ids, component.ID)
	}
	return ids
}

// hasRequiresCycle reports whether id reaches itself through Requires.
func hasRequiresCycle(id ID) bool {
	component, _ := Lookup(id)
	for _, required := range component.Requires {
		if slices.Contains(Close([]ID{required}), id) {
			return true
		}
	}
	return false
}

// withoutLines returns the Without lines of the catalog row id.
func withoutLines(id ID) []string {
	component, _ := Lookup(id)
	return component.Without
}

func TestCatalog(t *testing.T) {
	ids := catalogIds()

	// ids-unique
	unique := slices.Clone(ids)
	slices.Sort(unique)
	assert.Len(t, slices.Compact(unique), len(ids))

	// display-order-equals-const-order
	assert.Equal(t, constOrder, ids)

	// only-config-server-fixed
	var fixed []ID
	for _, component := range Catalog() {
		if component.Fixed {
			fixed = append(fixed, component.ID)
		}
	}
	assert.Equal(t, []ID{ConfigServer}, fixed)

	// every-requires-id-in-catalog
	for _, component := range Catalog() {
		for _, required := range component.Requires {
			assert.Containsf(t, ids, required, "%s requires %s", component.ID, required)
		}
	}

	// no-requires-cycle
	for _, id := range ids {
		assert.Falsef(t, hasRequiresCycle(id), "%s requires itself", id)
	}

	// settings-schema-valid
	assert.NoError(t, CheckSettings())
}

func TestClose(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []ID

		ids []ID
	}

	tests := make([]*testCase, 0)

	// empty-input
	tests = append(tests, &testCase{
		_id:       "empty-input",
		_expected: []ID{},

		ids: nil,
	})

	// no-requirements
	tests = append(tests, &testCase{
		_id:       "no-requirements",
		_expected: []ID{Skills, Peek},

		ids: []ID{Peek, Skills},
	})

	// context-adds-session-hooks-and-gate
	tests = append(tests, &testCase{
		_id:       "context-adds-session-hooks-and-gate",
		_expected: []ID{SessionHooks, Context, Gate},

		ids: []ID{Context},
	})

	// mining-adds-peek-context-gate-session-hooks
	tests = append(tests, &testCase{
		_id:       "mining-adds-peek-context-gate-session-hooks",
		_expected: []ID{SessionHooks, Context, Gate, Peek, Mining},

		ids: []ID{Mining},
	})

	// duplicate-input
	tests = append(tests, &testCase{
		_id:       "duplicate-input",
		_expected: []ID{Peek},

		ids: []ID{Peek, Peek},
	})

	// unknown-id-dropped
	tests = append(tests, &testCase{
		_id:       "unknown-id-dropped",
		_expected: []ID{Agentwt},

		ids: []ID{"nope", Agentwt},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Close(test.ids))
		})
	}
}

func TestDependents(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []ID

		id       ID
		selected []ID
	}

	tests := make([]*testCase, 0)

	// session-hooks-in-full-set
	tests = append(tests, &testCase{
		_id:       "session-hooks-in-full-set",
		_expected: []ID{SkillEnvironment, Context, Gate, Worktrees, SessionHelpers, Heartbeat, Mining},

		id:       SessionHooks,
		selected: Full(goosDarwin),
	})

	// peek-with-mining
	tests = append(tests, &testCase{
		_id:       "peek-with-mining",
		_expected: []ID{Mining},

		id:       Peek,
		selected: []ID{Peek, Mining, Skills},
	})

	// id-not-selected-still-checked
	tests = append(tests, &testCase{
		_id:       "id-not-selected-still-checked",
		_expected: []ID{Context},

		id:       SessionHooks,
		selected: []ID{Context},
	})

	// leaf-without-dependents
	tests = append(tests, &testCase{
		_id:       "leaf-without-dependents",
		_expected: []ID{},

		id:       Mining,
		selected: Full(goosDarwin),
	})

	// routines-need-the-token
	tests = append(tests, &testCase{
		_id:       "routines-need-the-token",
		_expected: []ID{Routines},

		id:       Token,
		selected: []ID{ConfigServer, Routines, Token},
	})

	// self-excluded
	tests = append(tests, &testCase{
		_id:       "self-excluded",
		_expected: []ID{},

		id:       Peek,
		selected: []ID{Peek},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Dependents(test.id, test.selected))
		})
	}
}

func TestAvailable(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		goos string
		id   ID
	}

	tests := make([]*testCase, 0)

	// heartbeat-darwin
	tests = append(tests, &testCase{
		_id:       "heartbeat-darwin",
		_expected: true,

		goos: goosDarwin,
		id:   Heartbeat,
	})

	// heartbeat-linux
	tests = append(tests, &testCase{
		_id:       "heartbeat-linux",
		_expected: false,

		goos: goosLinux,
		id:   Heartbeat,
	})

	// heartbeat-windows
	tests = append(tests, &testCase{
		_id:       "heartbeat-windows",
		_expected: true,

		goos: goosWindows,
		id:   Heartbeat,
	})

	// routines-windows
	tests = append(tests, &testCase{
		_id:       "routines-windows",
		_expected: true,

		goos: goosWindows,
		id:   Routines,
	})

	// mining-linux
	tests = append(tests, &testCase{
		_id:       "mining-linux",
		_expected: false,

		goos: goosLinux,
		id:   Mining,
	})

	// unrestricted-any-goos
	tests = append(tests, &testCase{
		_id:       "unrestricted-any-goos",
		_expected: true,

		goos: "plan9",
		id:   Peek,
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:       "unknown-id",
		_expected: false,

		goos: goosDarwin,
		id:   "nope",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Available(test.id, test.goos))
		})
	}
}

func TestFull(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []ID

		goos string
	}

	tests := make([]*testCase, 0)

	// darwin
	tests = append(tests, &testCase{
		_id:       "darwin",
		_expected: constOrder,

		goos: goosDarwin,
	})

	// linux
	tests = append(tests, &testCase{
		_id: "linux",
		_expected: []ID{
			ConfigServer,
			Skills,
			SessionHooks,
			SkillEnvironment,
			Context,
			Gate,
			Worktrees,
			SessionHelpers,
			GlobalInstructions,
			Agentwt,
			Peek,
			Ccusage,
			Token,
		},

		goos: goosLinux,
	})

	// windows
	tests = append(tests, &testCase{
		_id: "windows",
		_expected: []ID{
			ConfigServer,
			Skills,
			SessionHooks,
			SkillEnvironment,
			Context,
			Gate,
			Worktrees,
			SessionHelpers,
			GlobalInstructions,
			Agentwt,
			Peek,
			Ccusage,
			Heartbeat,
			Routines,
			Token,
			Mining,
		},

		goos: goosWindows,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Full(test.goos))
		})
	}
}

func TestLookup(t *testing.T) {
	// known
	component, ok := Lookup(Gate)
	assert.True(t, ok)
	assert.Equal(t, "Permission gate", component.Title)

	// unknown
	component, ok = Lookup("nope")
	assert.False(t, ok)
	assert.Equal(t, Component{}, component)
}

func TestWithout(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedLine string

		id ID
	}

	tests := make([]*testCase, 0)

	// agentwt-names-close-done-fleet-restack-smine
	tests = append(tests, &testCase{
		_id:           "agentwt-names-close-done-fleet-restack-smine",
		_expectedLine: "the close, done, fleet, restack, smine and smine-consolidate skills are not deployed — other skills drop their smine-agentwt steps",

		id: Agentwt,
	})

	// heartbeat-names-heartbeat-demo
	tests = append(tests, &testCase{
		_id:           "heartbeat-names-heartbeat-demo",
		_expectedLine: "the join, done, runtime, fleet and heartbeat-demo skills are not deployed — other skills drop their smine-heartbeat steps",

		id: Heartbeat,
	})

	// routines-names-plan-archive-fleet-smine-style
	tests = append(tests, &testCase{
		_id:           "routines-names-plan-archive-fleet-smine-style",
		_expectedLine: "the plan-archive, fleet and smine-style skills are not deployed — other skills drop their smine-routine steps",

		id: Routines,
	})

	// gate-names-heartbeat-demo-smine-help
	tests = append(tests, &testCase{
		_id:           "gate-names-heartbeat-demo-smine-help",
		_expectedLine: "the heartbeat-demo and smine-help skills are not deployed — other skills drop their toolpolicy steps",

		id: Gate,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Contains(t, withoutLines(test.id), test._expectedLine)
		})
	}
}
