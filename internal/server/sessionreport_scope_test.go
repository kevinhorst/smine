package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

func TestParseAgentScope(t *testing.T) {
	type testCase struct {
		_id       string
		_expected agentScope

		target string
	}

	tests := make([]*testCase, 0)

	// absent
	tests = append(tests, &testCase{
		_id:       "absent",
		_expected: agentScope{},

		target: "/sessions-report/sid",
	})

	// root
	tests = append(tests, &testCase{
		_id:       "root",
		_expected: agentScope{isRoot: true},

		target: "/sessions-report/sid?agent=root",
	})

	// type
	tests = append(tests, &testCase{
		_id:       "type",
		_expected: agentScope{agentType: "Explore"},

		target: "/sessions-report/sid?agent=type%3AExplore",
	})

	// id
	tests = append(tests, &testCase{
		_id:       "id",
		_expected: agentScope{id: "a1"},

		target: "/sessions-report/sid?agent=id:a1",
	})

	// unknown-value
	tests = append(tests, &testCase{
		_id:       "unknown-value",
		_expected: agentScope{},

		target: "/sessions-report/sid?agent=everyone",
	})

	// empty-type
	tests = append(tests, &testCase{
		_id:       "empty-type",
		_expected: agentScope{},

		target: "/sessions-report/sid?agent=type:",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			scope := parseAgentScope(httptest.NewRequest(http.MethodGet, test.target, nil))

			assert.Equal(t, test._expected, scope)
		})
	}
}

func TestAgentScope_Matches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []bool

		scope agentScope
	}

	agents := []contextrecord.Agent{
		{Id: "main", Kind: contextrecord.AgentRoot},
		{Id: "a1", Kind: contextrecord.AgentSubagent, Type: "Explore"},
		{Id: "a2", Kind: contextrecord.AgentSubagent, Type: "general-purpose"},
	}
	tests := make([]*testCase, 0)

	// all
	tests = append(tests, &testCase{
		_id:       "all",
		_expected: []bool{true, true, true},

		scope: agentScope{},
	})

	// root
	tests = append(tests, &testCase{
		_id:       "root",
		_expected: []bool{true, false, false},

		scope: agentScope{isRoot: true},
	})

	// type
	tests = append(tests, &testCase{
		_id:       "type",
		_expected: []bool{false, true, false},

		scope: agentScope{agentType: "Explore"},
	})

	// id
	tests = append(tests, &testCase{
		_id:       "id",
		_expected: []bool{false, false, true},

		scope: agentScope{id: "a2"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			matches := []bool{test.scope.matches(&agents[0]), test.scope.matches(&agents[1]), test.scope.matches(&agents[2])}

			assert.Equal(t, test._expected, matches)
		})
	}
}

func TestScopeURL(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		filter contextFilter
		scope  agentScope
	}

	base := "/sessions-report/sid"
	tests := make([]*testCase, 0)

	// neither
	tests = append(tests, &testCase{
		_id:       "neither",
		_expected: base,

		filter: contextFilter{},
		scope:  agentScope{},
	})

	// filter-only
	tests = append(tests, &testCase{
		_id:       "filter-only",
		_expected: base + "?layer=rule",

		filter: contextFilter{Layer: "rule"},
		scope:  agentScope{},
	})

	// scope-only
	tests = append(tests, &testCase{
		_id:       "scope-only",
		_expected: base + "?agent=root",

		filter: contextFilter{},
		scope:  agentScope{isRoot: true},
	})

	// both
	tests = append(tests, &testCase{
		_id:       "both",
		_expected: base + "?layer=rule&agent=id%3Aa1",

		filter: contextFilter{Layer: "rule"},
		scope:  agentScope{id: "a1"},
	})

	// escaped-type
	tests = append(tests, &testCase{
		_id:       "escaped-type",
		_expected: base + "?agent=type%3Arailroad+lane%2Fx",

		filter: contextFilter{},
		scope:  agentScope{agentType: "railroad lane/x"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, scopeURL(base, test.filter, test.scope))
		})
	}
}

func TestScopedRecord(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedAgents []string
		_expectedChars  int
		_expectedViews  bool

		scope agentScope
	}

	item := func(text string) contextrecord.Item {
		return contextrecord.NewItem(contextrecord.KindHookContext, "hook", contextrecord.SourceHookContext, text)
	}
	record := &contextrecord.Record{
		Agent: contextrecord.SessionClaude,
		Agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{item("aaaa")}, Kind: contextrecord.AgentRoot},
			{Id: "a1", Items: []contextrecord.Item{item("bb")}, Kind: contextrecord.AgentSubagent, Type: "general-purpose"},
			{Id: "a2", Items: []contextrecord.Item{item("c")}, Kind: contextrecord.AgentSubagent, Type: "general-purpose"},
			{Id: "a3", Items: []contextrecord.Item{item("ddddddd")}, Kind: contextrecord.AgentSubagent, Type: "Explore"},
		},
		Cwd:       "/repo",
		PackReads: []string{"/repo/context/a.md"},
		SessionId: "sid",
	}
	tests := make([]*testCase, 0)

	// all
	tests = append(tests, &testCase{
		_id:             "all",
		_expectedAgents: []string{"main", "a1", "a2", "a3"},
		_expectedChars:  14,
		_expectedViews:  true,

		scope: agentScope{},
	})

	// root-keeps-views
	tests = append(tests, &testCase{
		_id:             "root-keeps-views",
		_expectedAgents: []string{"main"},
		_expectedChars:  4,
		_expectedViews:  true,

		scope: agentScope{isRoot: true},
	})

	// type-sums-two-agents
	tests = append(tests, &testCase{
		_id:             "type-sums-two-agents",
		_expectedAgents: []string{"a1", "a2"},
		_expectedChars:  3,
		_expectedViews:  false,

		scope: agentScope{agentType: "general-purpose"},
	})

	// unknown-id-empties
	tests = append(tests, &testCase{
		_id:             "unknown-id-empties",
		_expectedAgents: []string{},
		_expectedChars:  0,
		_expectedViews:  false,

		scope: agentScope{id: "nope"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			scoped := scopedRecord(record, test.scope)

			ids := make([]string, 0, len(scoped.Agents))
			for _, agent := range scoped.Agents {
				ids = append(ids, agent.Id)
			}
			assert.Equal(t, test._expectedAgents, ids)
			assert.Equal(t, test._expectedChars, scoped.Totals.Chars)
			assert.Equal(t, test._expectedViews, len(scoped.PackReads) > 0)
			assert.Equal(t, "/repo", scoped.Cwd)
		})
	}
}
