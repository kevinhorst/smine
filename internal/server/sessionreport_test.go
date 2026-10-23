package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesFilter(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		filter contextFilter
		meta   idMeta
	}

	tests := make([]*testCase, 0)

	// no-filter-matches-all
	tests = append(tests, &testCase{
		_id:       "no-filter-matches-all",
		_expected: true,

		filter: contextFilter{},
		meta:   idMeta{},
	})

	// layer-acdsl-matches-rule
	tests = append(tests, &testCase{
		_id:       "layer-acdsl-matches-rule",
		_expected: true,

		filter: contextFilter{Layer: "acdsl"},
		meta:   idMeta{Kind: "acdsl"},
	})

	// layer-acdsl-drops-prose
	tests = append(tests, &testCase{
		_id:       "layer-acdsl-drops-prose",
		_expected: false,

		filter: contextFilter{Layer: "acdsl"},
		meta:   idMeta{Kind: "rule"},
	})

	// aspect-match
	tests = append(tests, &testCase{
		_id:       "aspect-match",
		_expected: true,

		filter: contextFilter{Aspect: "PLAN"},
		meta:   idMeta{Kind: "rule", Scope: "PLAN"},
	})

	// aspect-mismatch
	tests = append(tests, &testCase{
		_id:       "aspect-mismatch",
		_expected: false,

		filter: contextFilter{Aspect: "PLAN"},
		meta:   idMeta{Kind: "rule", Scope: "IMPL"},
	})

	// topic-mismatch
	tests = append(tests, &testCase{
		_id:       "topic-mismatch",
		_expected: false,

		filter: contextFilter{Aspect: "GOLANG", Topic: "ERR"},
		meta:   idMeta{Kind: "acdsl", Scope: "GOLANG", Topic: "ENUM"},
	})

	// unknown-reach-only-all
	tests = append(tests, &testCase{
		_id:       "unknown-reach-only-all",
		_expected: false,

		filter: contextFilter{Reach: "smine"},
		meta:   idMeta{Kind: "rule"},
	})

	// lifetime-prose-passes
	tests = append(tests, &testCase{
		_id:       "lifetime-prose-passes",
		_expected: true,

		filter: contextFilter{Lifetime: "task"},
		meta:   idMeta{Kind: "rule"},
	})

	// lifetime-mismatch-drops-acdsl
	tests = append(tests, &testCase{
		_id:       "lifetime-mismatch-drops-acdsl",
		_expected: false,

		filter: contextFilter{Lifetime: "task"},
		meta:   idMeta{Kind: "acdsl", Lifetime: "doctrine"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, matchesFilter(test.filter, test.meta))
		})
	}
}

func TestIdMetaFor(t *testing.T) {
	index := map[string]idMeta{
		"RULE-PLAN-001": {Href: "/context/entry/RULE-PLAN-001", Kind: "rule", Reach: "global", Scope: "REGISTRY"},
	}

	// registry hit wins over segment derivation
	hit := idMetaFor("RULE-PLAN-001", index)
	assert.Equal(t, "REGISTRY", hit.Scope)
	assert.Equal(t, "/context/entry/RULE-PLAN-001", hit.Href)

	// ACDSL fallback: kind acdsl, scope/topic from segments, no link
	acdslMeta := idMetaFor("ACDSL-GOLANG-ENUM-001", index)
	assert.Equal(t, "acdsl", acdslMeta.Kind)
	assert.Equal(t, "GOLANG", acdslMeta.Scope)
	assert.Equal(t, "ENUM", acdslMeta.Topic)
	assert.Empty(t, acdslMeta.Href)

	// prose fallback: kind from prefix, scope segment, no topic
	proseMeta := idMetaFor("RULE-IMPL-002", index)
	assert.Equal(t, "rule", proseMeta.Kind)
	assert.Equal(t, "IMPL", proseMeta.Scope)
	assert.Empty(t, proseMeta.Topic)

	// unknown prefix: empty kind
	assert.Empty(t, idMetaFor("XYZ-FOO-001", index).Kind)
}

func TestFilterIds(t *testing.T) {
	index := map[string]idMeta{
		"ACTION-IMPL-001": {Href: "/context/entry/ACTION-IMPL-001", Kind: "action", Scope: "IMPL"},
	}
	filter := contextFilter{Aspect: "IMPL"}
	ids := []string{"ACTION-IMPL-001", "ACTION-IMPL-999", "RULE-PLAN-001"}

	chips := filterIds(filter, ids, index)
	require.Len(t, chips, 2, "the PLAN id drops under aspect IMPL")
	assert.Equal(t, "/context/entry/ACTION-IMPL-001", chips[0].Href)
	assert.Empty(t, chips[1].Href, "unresolved id stays unlinked")
}

func TestAgentRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []agentRow

		filter contextFilter
		record *contextrecord.Record
		scope  agentScope
	}

	pendingItems := []contextrecord.Item{
		{Chars: 10, Kind: contextrecord.KindHookContext, Uncounted: contextrecord.UncountedRate},
		{Chars: 10, Kind: contextrecord.KindHookContext, Uncounted: contextrecord.UncountedCap},
		{Chars: 10, Kind: contextrecord.KindHookContext, Uncounted: contextrecord.UncountedSize},
	}
	rootAndExplore := &contextrecord.Record{Agents: []contextrecord.Agent{
		{Id: "main", Kind: contextrecord.AgentRoot},
		{Id: "a1", Kind: contextrecord.AgentSubagent, Type: "Explore"},
	}}
	tests := make([]*testCase, 0)

	// root-main
	tests = append(tests, &testCase{
		_id: "root-main",
		_expected: []agentRow{
			{Id: "main", Chars: "15 284", Items: "3", Kind: "Root", Model: "claude-opus-5-5", Name: "main", Tokens: "4 100", Url: "/r?agent=id%3Amain"},
		},

		filter: contextFilter{},
		record: &contextrecord.Record{Agents: []contextrecord.Agent{{
			Id:     "main",
			Kind:   contextrecord.AgentRoot,
			Model:  "claude-opus-5-5",
			Totals: contextrecord.Totals{Chars: 15284, Items: 3, Tokens: 4100},
		}}},
		scope: agentScope{},
	})

	// dimmed-outside-scope-url-keeps-filter
	tests = append(tests, &testCase{
		_id: "dimmed-outside-scope-url-keeps-filter",
		_expected: []agentRow{
			{Id: "main", Chars: "0", IsDimmed: true, Items: "0", Kind: "Root", Name: "main", Tokens: "—", Url: "/r?layer=rule&agent=id%3Amain"},
			{Id: "a1", Chars: "0", Items: "0", Kind: "Subagent", Name: "Explore", Tokens: "—", Type: "Explore", Url: "/r?layer=rule&agent=id%3Aa1"},
		},

		filter: contextFilter{Layer: "rule"},
		record: rootAndExplore,
		scope:  agentScope{agentType: "Explore"},
	})

	// subagent-general-purpose-with-description
	tests = append(tests, &testCase{
		_id: "subagent-general-purpose-with-description",
		_expected: []agentRow{{
			Id:          "a1",
			Chars:       "40",
			Description: "Ground the context report",
			Items:       "2",
			Kind:        "Subagent",
			Model:       "claude-sonnet-5",
			Name:        "general-purpose",
			Tokens:      "10",
			Type:        "general-purpose",
			Url:         "/r?agent=id%3Aa1",
		}},

		filter: contextFilter{},
		record: &contextrecord.Record{Agents: []contextrecord.Agent{{
			Id:          "a1",
			Description: "Ground the context report",
			Kind:        contextrecord.AgentSubagent,
			Model:       "claude-sonnet-5",
			Totals:      contextrecord.Totals{Chars: 40, Items: 2, Tokens: 10},
			Type:        "general-purpose",
		}}},
		scope: agentScope{},
	})

	// pending-count-and-title
	tests = append(tests, &testCase{
		_id: "pending-count-and-title",
		_expected: []agentRow{{
			Id:           "a2",
			Chars:        "30",
			Items:        "3",
			Kind:         "Subagent",
			Name:         "Explore",
			Pending:      "2",
			PendingTitle: "not reached by the counter yet — reload to continue",
			Tokens:       "—",
			Type:         "Explore",
			Url:          "/r?agent=id%3Aa2",
		}},

		filter: contextFilter{},
		record: &contextrecord.Record{Agents: []contextrecord.Agent{{
			Id:     "a2",
			Items:  pendingItems,
			Kind:   contextrecord.AgentSubagent,
			Totals: contextrecord.Totals{Chars: 30, Items: 3, Uncounted: 3},
			Type:   "Explore",
		}}},
		scope: agentScope{},
	})

	// workflow-kind
	tests = append(tests, &testCase{
		_id: "workflow-kind",
		_expected: []agentRow{
			{Id: "w1", Chars: "0", Items: "0", Kind: "Workflow", Name: "w1", Tokens: "—", Url: "/r?agent=id%3Aw1"},
		},

		filter: contextFilter{},
		record: &contextrecord.Record{Agents: []contextrecord.Agent{{Id: "w1", Kind: contextrecord.AgentWorkflow}}},
		scope:  agentScope{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, agentRows("/r", test.filter, test.record, test.scope))
		})
	}
}

func TestAgentTypes(t *testing.T) {
	record := &contextrecord.Record{
		Agents: []contextrecord.Agent{
			{Id: "main", Kind: contextrecord.AgentRoot},
			{Id: "a1", Kind: contextrecord.AgentSubagent, Type: "general-purpose"},
			{Id: "a2", Kind: contextrecord.AgentSubagent, Type: "Explore"},
			{Id: "a3", Kind: contextrecord.AgentSubagent, Type: "general-purpose"},
		},
	}

	// sorted unique, the root without a type skipped
	assert.Equal(t, []string{"Explore", "general-purpose"}, agentTypes(record))
	assert.Empty(t, agentTypes(&contextrecord.Record{}))
}

// sizeTexts renders size figures in their one-line markdown form.
func sizeTexts(sizes map[string]sectionSize) map[string]string {
	texts := make(map[string]string, len(sizes))
	for key, size := range sizes {
		texts[key] = size.String()
	}
	return texts
}

func TestKindSizes(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]string

		record *contextrecord.Record
	}

	global := contextrecord.NewItem(contextrecord.KindGlobal, "global.md", contextrecord.SourceToolResult, strings.Repeat("x", 1200))
	global.SetTokens(contextrecord.CounterAnthropic, 300, false)
	secondGlobal := contextrecord.NewItem(contextrecord.KindGlobal, "pointer", contextrecord.SourceHookSuccess, "0123456789")
	secondGlobal.SetTokens(contextrecord.CounterAnthropic, 4, false)
	hook := contextrecord.NewItem(contextrecord.KindHookContext, "sessiontitle", contextrecord.SourceHookContext, "hello")
	hook.Uncounted = contextrecord.UncountedRate
	skipped := contextrecord.NewItem(contextrecord.KindHookContext, "worktree", contextrecord.SourceHookContext, "abc")
	skipped.Uncounted = contextrecord.UncountedSize
	packRead := contextrecord.NewItem(contextrecord.KindPackRead, "/repo/docs/a.md", contextrecord.SourceToolResult, "0123456789")
	packRead.SetTokens(contextrecord.CounterAnthropic, 3, false)
	guideRead := contextrecord.NewItem(contextrecord.KindLang, "/repo/docs/rules/go.md", contextrecord.SourceToolResult, "01234")
	guideRead.Uncounted = contextrecord.UncountedCap
	tests := make([]*testCase, 0)

	// file-reads-combine-pack-and-guide
	tests = append(tests, &testCase{
		_id: "file-reads-combine-pack-and-guide",
		_expected: map[string]string{
			"file_reads": "15 chars · 3 tokens so far · 1 pending",
			"lang":       "5 chars · 0 tokens so far · 1 pending",
			"pack_read":  "10 chars · 3 tokens",
		},

		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{
				{Id: "main", Items: []contextrecord.Item{packRead}},
				{Id: "a1", Items: []contextrecord.Item{guideRead}},
			},
		},
	})

	// no-pending
	tests = append(tests, &testCase{
		_id:       "no-pending",
		_expected: map[string]string{"global": "1 210 chars · 304 tokens"},

		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{
				{Id: "main", Items: []contextrecord.Item{global}},
				{Id: "a1", Items: []contextrecord.Item{secondGlobal}},
			},
		},
	})

	// pending-per-kind
	tests = append(tests, &testCase{
		_id: "pending-per-kind",
		_expected: map[string]string{
			"global":       "1 200 chars · 300 tokens",
			"hook_context": "8 chars · 0 tokens so far · 1 pending",
			"hooks":        "8 chars · 0 tokens so far · 1 pending",
		},

		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{
				{Id: "main", Items: []contextrecord.Item{global, hook}},
				{Id: "a1", Items: []contextrecord.Item{skipped}},
			},
		},
	})

	// empty-record
	tests = append(tests, &testCase{
		_id:       "empty-record",
		_expected: map[string]string{},

		record: &contextrecord.Record{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sizeTexts(kindSizes(test.record)))
		})
	}
}

func TestPendingNotice(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		isCounting bool
		record     *contextrecord.Record
	}

	queued := make([]contextrecord.Item, 7)
	for index := range queued {
		queued[index] = contextrecord.Item{Kind: contextrecord.KindLang, Uncounted: contextrecord.UncountedQueued}
	}
	ratePending := make([]contextrecord.Item, 40)
	for index := range ratePending {
		ratePending[index] = contextrecord.Item{Kind: contextrecord.KindHookContext, Uncounted: contextrecord.UncountedRate}
	}
	capPending := make([]contextrecord.Item, 3)
	for index := range capPending {
		capPending[index] = contextrecord.Item{Kind: contextrecord.KindPackRead, Uncounted: contextrecord.UncountedCap}
	}
	counted := contextrecord.Item{Counter: contextrecord.CounterAnthropic, Kind: contextrecord.KindGlobal, Tokens: 12}
	skipped := contextrecord.Item{Kind: contextrecord.KindGlobal, Uncounted: contextrecord.UncountedSize}
	tests := make([]*testCase, 0)

	// none
	tests = append(tests, &testCase{
		_id:       "none",
		_expected: "",

		isCounting: false,
		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{{Id: "main", Items: []contextrecord.Item{counted, skipped}}},
		},
	})

	// none-while-counting
	tests = append(tests, &testCase{
		_id:       "none-while-counting",
		_expected: "",

		isCounting: true,
		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{{Id: "main", Items: []contextrecord.Item{counted, skipped}}},
		},
	})

	// forty-three-pending
	tests = append(tests, &testCase{
		_id:       "forty-three-pending",
		_expected: "Token counting is throttled — 30 API calls per minute, 200 injections per page load. 43 injections pending: reload to continue.",

		isCounting: false,
		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{
				{Id: "main", Items: ratePending},
				{Id: "a1", Items: capPending},
			},
		},
	})

	// seven-queued-run-in-flight
	tests = append(tests, &testCase{
		_id:       "seven-queued-run-in-flight",
		_expected: "Counting tokens in the background (pending: 7) — 30 API calls per minute, 200 injections per run. This page reloads once when the run finishes.",

		isCounting: true,
		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{{Id: "main", Items: append([]contextrecord.Item{counted}, queued...)}},
		},
	})

	// seven-queued-no-run
	tests = append(tests, &testCase{
		_id:       "seven-queued-no-run",
		_expected: "Token counting is throttled — 30 API calls per minute, 200 injections per page load. 7 injections pending: reload to continue.",

		isCounting: false,
		record: &contextrecord.Record{
			Agents: []contextrecord.Agent{{Id: "main", Items: queued}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, pendingNotice(test.record, test.isCounting))
		})
	}
}

func TestSessionBranch(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		record  *contextrecord.Record
		session peek.Session
	}

	tests := make([]*testCase, 0)

	// record-branch-wins
	tests = append(tests, &testCase{
		_id:       "record-branch-wins",
		_expected: "claude/report",

		record:  &contextrecord.Record{GitBranch: "claude/report"},
		session: peek.Session{GitBranch: "HEAD"},
	})

	// peek-fallback
	tests = append(tests, &testCase{
		_id:       "peek-fallback",
		_expected: "main",

		record:  &contextrecord.Record{},
		session: peek.Session{GitBranch: "main"},
	})

	// head-hidden
	tests = append(tests, &testCase{
		_id:       "head-hidden",
		_expected: "",

		record:  &contextrecord.Record{},
		session: peek.Session{GitBranch: "HEAD"},
	})

	// nil-record
	tests = append(tests, &testCase{
		_id:       "nil-record",
		_expected: "main",

		record:  nil,
		session: peek.Session{GitBranch: "main"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sessionBranch(test.record, test.session))
		})
	}
}

func TestSkillRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []skillRow

		events *peek.SessionEvents
		record *contextrecord.Record
		server *Server
	}

	started := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	skillsRepo := t.TempDir()
	skillsHome := t.TempDir()
	writeSkillModel(
		t,
		filepath.Join(skillsRepo, "fdesign"),
		"---\nname: fdesign\ndescription: d\nversion: 6.3.0\n---\n\n# Fdesign\n",
		`[{"version": "6.3.0", "date": "2026-09-30", "text": "counted", "tokens": 3400}, {"version": "6.2.0", "date": "2026-09-01", "text": "older"}]`,
	)
	homeOnlyDir := filepath.Join(skillsHome, "concept")
	require.NoError(t, os.MkdirAll(homeOnlyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(homeOnlyDir, "SKILL.md"), []byte("---\nname: concept\ndescription: d\nversion: 1.0.0\n---\n"), 0o644))
	deployedDir := filepath.Join(skillsHome, "fdesign")
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "SKILL.md"), []byte("---\nname: fdesign\ndescription: d\nversion: 6.3.0\n---\n"), 0o644))
	writeAliasedRepoSkill(
		t,
		skillsRepo,
		"2.0.0",
		`[{"version": "2.0.0", "date": "2026-10-05", "text": "aliased", "tokens": 1200}, {"version": "1.0.0", "date": "2026-09-01", "text": "first", "tokens": 900}]`,
	)
	writeAliasedHomeSkill(t, skillsHome, "2.0.0")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})
	tests := make([]*testCase, 0)

	// version-joined-by-order
	tests = append(tests, &testCase{
		_id: "version-joined-by-order",
		_expected: []skillRow{
			{Ended: "—", Skill: "fdesign", SkillTokens: "—", Source: "slash", Started: "2000-01-01 00:00:00", Version: "6.2.0"},
			{Ended: "—", Skill: "fdesign", SkillTokens: "3 400", Source: "slash", Started: "2000-01-01 01:00:00", Version: "6.3.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{
			{Skill: "fdesign", Source: "slash", Timestamp: started},
			{Skill: "fdesign", Source: "slash", Timestamp: started.Add(time.Hour)},
		}},
		record: &contextrecord.Record{Injected: contextrecord.InjectedContext{Skill: []contextrecord.SkillContext{
			{Skill: "fdesign", Version: "6.2.0"},
			{Skill: "fdesign", Version: "6.3.0"},
		}}},
		server: server,
	})

	// home-only-copy
	tests = append(tests, &testCase{
		_id: "home-only-copy",
		_expected: []skillRow{
			{Ended: "—", Skill: "concept", SkillTokens: "—", Source: "skill", Started: "2000-01-01 00:00:00", Version: "1.0.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "concept", Source: "skill", Timestamp: started}}},
		record: &contextrecord.Record{Injected: contextrecord.InjectedContext{Skill: []contextrecord.SkillContext{
			{Skill: "concept", Version: "1.0.0"},
		}}},
		server: server,
	})

	// no-recorded-version-takes-deployed
	tests = append(tests, &testCase{
		_id: "no-recorded-version-takes-deployed",
		_expected: []skillRow{
			{Ended: "—", IsVersionAssumed: true, Skill: "fdesign", SkillTokens: "3 400", Source: "slash", Started: "2000-01-01 00:00:00", Version: "6.3.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "fdesign", Source: "slash", Timestamp: started}}},
		record: &contextrecord.Record{},
		server: server,
	})

	// empty-recorded-version-takes-deployed
	tests = append(tests, &testCase{
		_id: "empty-recorded-version-takes-deployed",
		_expected: []skillRow{
			{Ended: "—", IsVersionAssumed: true, Skill: "fdesign", SkillTokens: "3 400", Source: "slash", Started: "2000-01-01 00:00:00", Version: "6.3.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "fdesign", Source: "slash", Timestamp: started}}},
		record: &contextrecord.Record{Injected: contextrecord.InjectedContext{Skill: []contextrecord.SkillContext{
			{Skill: "fdesign", Version: ""},
		}}},
		server: server,
	})

	// home-only-copy-without-context
	tests = append(tests, &testCase{
		_id: "home-only-copy-without-context",
		_expected: []skillRow{
			{Ended: "—", IsVersionAssumed: true, Skill: "concept", SkillTokens: "—", Source: "skill", Started: "2000-01-01 00:00:00", Version: "1.0.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "concept", Source: "skill", Timestamp: started}}},
		record: &contextrecord.Record{},
		server: server,
	})

	// unknown-skill-no-version
	tests = append(tests, &testCase{
		_id: "unknown-skill-no-version",
		_expected: []skillRow{
			{Ended: "—", Skill: "compact", SkillTokens: "—", Source: "slash", Started: "2000-01-01 00:00:00", Version: "—"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "compact", Source: "slash", Timestamp: started}}},
		record: &contextrecord.Record{},
		server: server,
	})

	// alias-row-uses-skill-version
	tests = append(tests, &testCase{
		_id: "alias-row-uses-skill-version",
		_expected: []skillRow{
			{Ended: "—", IsVersionAssumed: true, Skill: "pk", SkillTokens: "1 200", Source: "slash", Started: "2000-01-01 00:00:00", Version: "2.0.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "pk", Source: "slash", Timestamp: started}}},
		record: &contextrecord.Record{},
		server: server,
	})

	// alias-row-uses-skill-tokens
	tests = append(tests, &testCase{
		_id: "alias-row-uses-skill-tokens",
		_expected: []skillRow{
			{Ended: "—", Skill: "pk", SkillTokens: "900", Source: "slash", Started: "2000-01-01 00:00:00", Version: "1.0.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "pk", Source: "slash", Timestamp: started}}},
		record: &contextrecord.Record{Injected: contextrecord.InjectedContext{Skill: []contextrecord.SkillContext{
			{Skill: "pk", Version: "1.0.0"},
		}}},
		server: server,
	})

	// alias-label-as-typed
	tests = append(tests, &testCase{
		_id: "alias-label-as-typed",
		_expected: []skillRow{
			{Ended: "—", IsVersionAssumed: true, Skill: "peek", SkillTokens: "1 200", Source: "slash", Started: "2000-01-01 00:00:00", Version: "2.0.0"},
			{Ended: "—", IsVersionAssumed: true, Skill: "pk", SkillTokens: "1 200", Source: "skill", Started: "2000-01-01 01:00:00", Version: "2.0.0"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{
			{Skill: "peek", Source: "slash", Timestamp: started},
			{Skill: "pk", Source: "skill", Timestamp: started.Add(time.Hour)},
		}},
		record: &contextrecord.Record{},
		server: server,
	})

	// unknown-name-stays-dash
	tests = append(tests, &testCase{
		_id: "unknown-name-stays-dash",
		_expected: []skillRow{
			{Ended: "—", Skill: "pek", SkillTokens: "—", Source: "slash", Started: "2000-01-01 00:00:00", Version: "—"},
		},

		events: &peek.SessionEvents{InvokedSkills: []peek.SkillInvocation{{Skill: "pek", Source: "slash", Timestamp: started}}},
		record: &contextrecord.Record{},
		server: server,
	})

	// runs-joined-by-order
	tests = append(tests, &testCase{
		_id: "runs-joined-by-order",
		_expected: []skillRow{
			{Ended: "2000-01-01 00:02:14", Skill: "fdesign", SkillTokens: "—", Source: "slash", Started: "2000-01-01 00:00:01", Version: "6.2.0"},
			{Ended: "—", Skill: "fdesign", SkillTokens: "3 400", Source: "slash", Started: "2000-01-01 01:00:01", Version: "6.3.0"},
		},

		events: &peek.SessionEvents{
			InvokedSkills: []peek.SkillInvocation{
				{Skill: "fdesign", Source: "slash", Timestamp: started},
				{Skill: "fdesign", Source: "slash", Timestamp: started.Add(time.Hour)},
			},
			SkillRuns: []peek.SkillRun{
				{EndedAt: started.Add(134 * time.Second), Seconds: 134, Skill: "fdesign", StartedAt: started.Add(time.Second)},
				{Skill: "fdesign", StartedAt: started.Add(time.Hour + time.Second)},
			},
		},
		record: &contextrecord.Record{Injected: contextrecord.InjectedContext{Skill: []contextrecord.SkillContext{
			{Skill: "fdesign", Version: "6.2.0"},
			{Skill: "fdesign", Version: "6.3.0"},
		}}},
		server: server,
	})

	// run-of-other-skill-ignored
	tests = append(tests, &testCase{
		_id: "run-of-other-skill-ignored",
		_expected: []skillRow{
			{Ended: "—", Skill: "compact", SkillTokens: "—", Source: "slash", Started: "2000-01-01 00:00:00", Version: "—"},
		},

		events: &peek.SessionEvents{
			InvokedSkills: []peek.SkillInvocation{{Skill: "compact", Source: "slash", Timestamp: started}},
			SkillRuns:     []peek.SkillRun{{EndedAt: started.Add(time.Minute), Seconds: 60, Skill: "fdesign", StartedAt: started}},
		},
		record: &contextrecord.Record{},
		server: server,
	})

	// nil-events
	tests = append(tests, &testCase{
		_id:       "nil-events",
		_expected: nil,

		events: nil,
		record: &contextrecord.Record{},
		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.server.skillRows(test.events, test.record))
		})
	}
}

func TestLoadRecord_UnknownAgent(t *testing.T) {
	server := newTestServer(t, &Options{})

	record, isStored, err := server.loadRecord(context.Background(), nil, peek.Session{Agent: "gemini", Id: "sid"})

	assert.Nil(t, record)
	assert.False(t, isStored)
	assert.EqualError(t, err, `Server.loadRecord: ExtractRecord: Unknown session agent "gemini"`)
}

// storedSession writes a usage record and a breakdown for id, both at
// lastActive; a nil record stores a failed extraction.
func storedSession(t *testing.T, server *Server, id string, lastActive time.Time, record *contextrecord.Record) {
	t.Helper()
	usage := &budget.Record{
		Id:     id,
		Agent:  contextrecord.SessionClaude,
		Cwd:    "/stored",
		Peek:   &budget.PeekSide{LastActive: lastActive},
		Schema: budget.RecordSchema,
		Title:  "Stored session",
	}
	require.NoError(t, server.usage.SaveRecord(usage))
	breakdown := &budget.Breakdown{Id: id, Error: "extraction failed", LastActivity: lastActive, Schema: budget.RecordSchema}
	if record != nil {
		breakdown = budget.NewBreakdown(lastActive, id, lastActive, record)
	}
	require.NoError(t, server.usage.SaveBreakdown(breakdown))
}

// storedRecord is a one-item record marked by its cwd and hook text.
func storedRecord() *contextrecord.Record {
	item := contextrecord.NewItem(contextrecord.KindHookContext, "sessiontitle", contextrecord.SourceHookContext, "stored hook text")
	record := &contextrecord.Record{
		Agent:  contextrecord.SessionClaude,
		Agents: []contextrecord.Agent{{Id: "main", Items: []contextrecord.Item{item}, Kind: contextrecord.AgentRoot}},
		Cwd:    "/stored",
	}
	record.Recompute()
	return record
}

func TestServer_LoadRecord(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCwd      string
		_expectedIsStored bool

		server  *Server
		session peek.Session
	}

	lastActive := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	session := peek.Session{Id: "sess-1", Agent: contextrecord.SessionClaude, LastActive: lastActive}
	newServer := func() *Server {
		claudeHome := t.TempDir()
		copyTranscriptFixture(t, claudeHome, "sess-1")
		return newTestServer(t, &Options{ClaudeHome: claudeHome, ContextDir: filepath.Join(t.TempDir(), "docs")})
	}
	tests := make([]*testCase, 0)

	// stored-current-used
	current := newServer()
	storedSession(t, current, "sess-1", lastActive, storedRecord())
	tests = append(tests, &testCase{
		_id:               "stored-current-used",
		_expectedCwd:      "/stored",
		_expectedIsStored: true,

		server:  current,
		session: session,
	})

	// stored-older-recomputed
	older := newServer()
	storedSession(t, older, "sess-1", lastActive.Add(-time.Hour), storedRecord())
	tests = append(tests, &testCase{
		_id:               "stored-older-recomputed",
		_expectedCwd:      "/repo/x",
		_expectedIsStored: false,

		server:  older,
		session: session,
	})

	// stored-error-recomputed
	failed := newServer()
	storedSession(t, failed, "sess-1", lastActive, nil)
	tests = append(tests, &testCase{
		_id:               "stored-error-recomputed",
		_expectedCwd:      "/repo/x",
		_expectedIsStored: false,

		server:  failed,
		session: session,
	})

	// none-stored-extracted
	tests = append(tests, &testCase{
		_id:               "none-stored-extracted",
		_expectedCwd:      "/repo/x",
		_expectedIsStored: false,

		server:  newServer(),
		session: session,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, isStored, err := test.server.loadRecord(context.Background(), nil, test.session)
			require.NoError(t, err)
			assert.Equal(t, test._expectedCwd, record.Cwd)
			assert.Equal(t, test._expectedIsStored, isStored)
		})
	}
}

func TestServer_LoadSessionReport_NoneStoredExtractedAndStored(t *testing.T) {
	lastActive := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	sessionsByAgent := map[string][]map[string]any{
		"claude": {{"id": "sess-1", "last_active": lastActive.Format(time.RFC3339), "meta": map[string]any{"cwd": "/repo/x"}}},
	}
	stub := newAgentStubPeek(t, nil, sessionsByAgent, nil)
	claudeHome := t.TempDir()
	copyTranscriptFixture(t, claudeHome, "sess-1")
	server := newTestServer(t, &Options{ClaudeHome: claudeHome, ContextDir: filepath.Join(t.TempDir(), "docs"), PeekEndpoint: stub.URL})

	page, ok := server.loadSessionReport(context.Background(), contextFilter{}, agentScope{}, "sess-1", true)
	breakdown, err := server.usage.Breakdown("sess-1")

	require.True(t, ok)
	require.NoError(t, err)
	require.NotNil(t, breakdown)
	assert.Empty(t, page.ContextErr)
	assert.True(t, lastActive.Equal(breakdown.LastActivity))
	require.NotNil(t, breakdown.Record)
	assert.Equal(t, "/repo/x", breakdown.Record.Cwd)
}

func TestServer_LoadStoredReport(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedContains []string

		path   string
		server *Server
	}

	lastActive := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	sessionsByAgent := map[string][]map[string]any{
		"claude": {{"id": "live-1", "last_active": lastActive.Format(time.RFC3339), "meta": map[string]any{"cwd": "/repo/x"}}},
	}
	stub := newAgentStubPeek(t, nil, sessionsByAgent, nil)
	storedNotice := "peek does not serve this session — the context below is the stored breakdown"
	tests := make([]*testCase, 0)

	// peek-unknown-id-with-store
	withStore := newTestServer(t, &Options{ClaudeHome: t.TempDir(), PeekEndpoint: stub.URL})
	storedSession(t, withStore, "gone-1", lastActive, storedRecord())
	tests = append(tests, &testCase{
		_id:               "peek-unknown-id-with-store",
		_expectedCode:     http.StatusOK,
		_expectedContains: []string{storedNotice, "Stored session", "stored hook text"},

		path:   "/sessions-report/gone-1",
		server: withStore,
	})

	// peek-unknown-id-without-store-404
	tests = append(tests, &testCase{
		_id:               "peek-unknown-id-without-store-404",
		_expectedCode:     http.StatusNotFound,
		_expectedContains: []string{},

		path:   "/sessions-report/gone-1",
		server: newTestServer(t, &Options{ClaudeHome: t.TempDir(), PeekEndpoint: stub.URL}),
	})

	// peek-down-with-store
	peekDown := newTestServer(t, &Options{ClaudeHome: t.TempDir(), PeekEndpoint: "http://127.0.0.1:1/mcp"})
	storedSession(t, peekDown, "live-1", lastActive, storedRecord())
	tests = append(tests, &testCase{
		_id:               "peek-down-with-store",
		_expectedCode:     http.StatusOK,
		_expectedContains: []string{storedNotice, "Stored session", "stored hook text"},

		path:   "/sessions-report/live-1",
		server: peekDown,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			assert.Equal(t, test._expectedCode, response.Code)
			for _, expected := range test._expectedContains {
				assert.Contains(t, response.Body.String(), expected)
			}
		})
	}
}

func TestEntryTitle(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		statement string
	}

	tests := make([]*testCase, 0)

	// bold-with-colon
	tests = append(tests, &testCase{
		_id:       "bold-with-colon",
		_expected: "Pivotal marker",

		statement: "**Pivotal marker**: a fact ID gets `!` when it decides.",
	})

	// bold-with-period
	tests = append(tests, &testCase{
		_id:       "bold-with-period",
		_expected: "Code is mode-invariant",

		statement: "**Code is mode-invariant.** Every added function appears as its full code block.",
	})

	// backticks-stripped
	tests = append(tests, &testCase{
		_id:       "backticks-stripped",
		_expected: "Modified existing code = diff fenced block",

		statement: "**Modified existing code = `diff` fenced block**: `+`/`-` lines.",
	})

	// no-bold
	tests = append(tests, &testCase{
		_id:       "no-bold",
		_expected: "",

		statement: "Pivotal rows sort first within their target group.",
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: "",

		statement: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, entryTitle(test.statement))
		})
	}
}

func TestPlainText(t *testing.T) {
	assert.Equal(t, "Pivotal marker: a fact ID gets ! when it decides.", plainText("**Pivotal marker**: a fact ID gets `!` when it decides."))
	assert.Equal(t, "", plainText(""))
}

func TestIdFamilyName(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		id string
	}

	tests := make([]*testCase, 0)

	// numbered
	tests = append(tests, &testCase{
		_id:       "numbered",
		_expected: "RULE-PLAN",

		id: "RULE-PLAN-041",
	})

	// three-segment-numbered
	tests = append(tests, &testCase{
		_id:       "three-segment-numbered",
		_expected: "ACTION-CONCEPT-HOT",

		id: "ACTION-CONCEPT-HOT-007",
	})

	// no-number
	tests = append(tests, &testCase{
		_id:       "no-number",
		_expected: "ACDSL-PROJECTION",

		id: "ACDSL-PROJECTION",
	})

	// no-dash
	tests = append(tests, &testCase{
		_id:       "no-dash",
		_expected: "PLAIN",

		id: "PLAIN",
	})

	// trailing-dash
	tests = append(tests, &testCase{
		_id:       "trailing-dash",
		_expected: "RULE-PLAN-",

		id: "RULE-PLAN-",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, idFamilyName(test.id))
		})
	}
}

func TestGroupIdFamilies(t *testing.T) {
	chips := []idChip{
		{Id: "RULE-PLAN-002"},
		{Id: "ACTION-IMPL-001", Title: "Stop"},
		{Id: "RULE-PLAN-001"},
	}

	families := groupIdFamilies(chips, true)

	require.Len(t, families, 2)
	assert.Equal(t, "RULE-PLAN", families[0].Name)
	assert.True(t, families[0].Open)
	assert.Equal(t, []idChip{{Id: "RULE-PLAN-002"}, {Id: "RULE-PLAN-001"}}, families[0].Chips)
	assert.Equal(t, "ACTION-IMPL", families[1].Name)
	assert.Equal(t, []idChip{{Id: "ACTION-IMPL-001", Title: "Stop"}}, families[1].Chips)
	assert.Nil(t, groupIdFamilies(nil, false))
}

func TestGroupTouchedFiles(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []touchedFileGroup

		claudeHome string
		record     *contextrecord.Record
	}

	claudeHome := filepath.FromSlash("/home/k/.claude")
	repo := filepath.FromSlash("/repo")
	goTouch := contextrecord.FileTouch{Path: filepath.FromSlash("/repo/internal/server/a.go"), Reads: 2}
	secondGoTouch := contextrecord.FileTouch{Path: filepath.FromSlash("/repo/cmd/b.go"), Writes: 1}
	markdownTouch := contextrecord.FileTouch{Path: filepath.FromSlash("/repo/README.md"), Reads: 1}
	configTouch := contextrecord.FileTouch{Path: filepath.FromSlash("/home/k/.claude/skills/fdesign/SKILL.md"), Reads: 1}
	outsideTouch := contextrecord.FileTouch{Path: filepath.FromSlash("/elsewhere/c.json"), Writes: 2}
	sharedGoTouch := contextrecord.FileTouch{Path: filepath.FromSlash("/repo/internal/server/a.go"), Reads: 1, Writes: 3}
	goFile := touchedFile{Agents: "main", Path: pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: filepath.FromSlash("internal/server/a.go")}, Reads: 2}
	secondGoFile := touchedFile{Agents: "Explore", Path: pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: filepath.FromSlash("cmd/b.go")}, Writes: 1}
	markdownFile := touchedFile{Agents: "main", Path: pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: "README.md"}, Reads: 1}
	configFile := touchedFile{Agents: "Explore", Path: pathCell{Prefix: filepath.FromSlash("/home/k/.claude/"), Rel: filepath.FromSlash("skills/fdesign/SKILL.md")}, Reads: 1}
	outsideFile := touchedFile{Agents: "main", Path: pathCell{Rel: filepath.FromSlash("/elsewhere/c.json")}, Writes: 2}
	summedGoFile := touchedFile{Agents: "2 agents", Path: pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: filepath.FromSlash("internal/server/a.go")}, Reads: 3, Writes: 3}
	tests := make([]*testCase, 0)

	// one-path-over-two-agents-summed
	tests = append(tests, &testCase{
		_id: "one-path-over-two-agents-summed",
		_expected: []touchedFileGroup{
			{Files: []touchedFile{summedGoFile}, Name: ".go"},
		},

		claudeHome: claudeHome,
		record: &contextrecord.Record{
			Cwd: repo,
			Agents: []contextrecord.Agent{
				{Id: "main", Files: []contextrecord.FileTouch{goTouch}, Kind: contextrecord.AgentRoot},
				{Id: "a1", Files: []contextrecord.FileTouch{sharedGoTouch}, Type: "Explore"},
			},
		},
	})

	// two-agents-extension-buckets
	tests = append(tests, &testCase{
		_id: "two-agents-extension-buckets",
		_expected: []touchedFileGroup{
			{Files: []touchedFile{secondGoFile, goFile}, Name: ".go"},
			{Files: []touchedFile{markdownFile}, Name: ".md"},
		},

		claudeHome: claudeHome,
		record: &contextrecord.Record{
			Cwd: repo,
			Agents: []contextrecord.Agent{
				{Id: "main", Files: []contextrecord.FileTouch{markdownTouch, goTouch}, Kind: contextrecord.AgentRoot},
				{Id: "a1", Files: []contextrecord.FileTouch{secondGoTouch}, Type: "Explore"},
			},
		},
	})

	// claude-bucket-trailing
	tests = append(tests, &testCase{
		_id: "claude-bucket-trailing",
		_expected: []touchedFileGroup{
			{Files: []touchedFile{markdownFile}, Name: ".md"},
			{Files: []touchedFile{configFile}, Name: claudeConfigGroup},
		},

		claudeHome: claudeHome,
		record: &contextrecord.Record{
			Cwd: repo,
			Agents: []contextrecord.Agent{
				{Id: "main", Files: []contextrecord.FileTouch{markdownTouch}, Kind: contextrecord.AgentRoot},
				{Id: "a1", Files: []contextrecord.FileTouch{configTouch}, Type: "Explore"},
			},
		},
	})

	// paths-relative-to-cwd
	tests = append(tests, &testCase{
		_id: "paths-relative-to-cwd",
		_expected: []touchedFileGroup{
			{Files: []touchedFile{goFile}, Name: ".go"},
			{Files: []touchedFile{outsideFile}, Name: ".json"},
		},

		claudeHome: claudeHome,
		record: &contextrecord.Record{
			Cwd:    repo,
			Agents: []contextrecord.Agent{{Id: "main", Files: []contextrecord.FileTouch{outsideTouch, goTouch}, Kind: contextrecord.AgentRoot}},
		},
	})

	// no-files
	tests = append(tests, &testCase{
		_id:       "no-files",
		_expected: nil,

		claudeHome: claudeHome,
		record:     &contextrecord.Record{Agents: []contextrecord.Agent{{Id: "main"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, groupTouchedFiles(test.record, test.claudeHome))
		})
	}
}

func TestIsClaudeConfigPath(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		path string
	}

	tests := make([]*testCase, 0)

	// config-file
	tests = append(tests, &testCase{
		_id:       "config-file",
		_expected: true,

		path: "/home/k/.claude/settings.json",
	})

	// worktree-file
	tests = append(tests, &testCase{
		_id:       "worktree-file",
		_expected: false,

		path: "/repo/.claude/worktrees/x/main.go",
	})

	// plain-repo-file
	tests = append(tests, &testCase{
		_id:       "plain-repo-file",
		_expected: false,

		path: "/repo/internal/server/a.go",
	})

	// windows-separators
	tests = append(tests, &testCase{
		_id:       "windows-separators",
		_expected: true,

		path: `C:\Users\k\.claude\settings.json`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, isClaudeConfigPath(test.path))
		})
	}
}

// The cost tile opens on plan % with the plan select when a percent exists, on dollars with the plan chip disabled otherwise.
func TestSessionReportCostTile(t *testing.T) {
	server := newTestServer(t, &Options{})
	page := &sessionReportPage{
		Events:  &peek.SessionEvents{},
		Page:    pageSessions,
		Plan:    "Max 20x",
		Plans:   planOptions(budget.PlanMax20x),
		Session: peek.Session{Id: "sid"},
		Stats:   reportStats{Cost: "$1.50", PlanPercent: "12%"},
		Title:   "Session — sid",
	}

	response := httptest.NewRecorder()
	server.renderFragment(response, tmplSessionReport, page)
	body := response.Body.String()
	assert.Contains(t, body, `class="tile tile-cost" data-cost-unit="plan"`)
	assert.Contains(t, body, `<span class="tile-value u-plan">12%</span>`)
	assert.Contains(t, body, `<span class="tile-value u-usd">$1.50</span>`)
	assert.Contains(t, body, `<option value="max-20x" selected>Max 20x</option>`)
	assert.Contains(t, body, `action="/usage/plan"`)
	assert.Contains(t, body, `12% of a Max 20x 5-hour window`)

	page.Stats.PlanPercent = ""
	response = httptest.NewRecorder()
	server.renderFragment(response, tmplSessionReport, page)
	body = response.Body.String()
	assert.Contains(t, body, `data-cost-unit="usd"`)
	assert.Contains(t, body, `data-set-unit="plan" disabled`)
	assert.NotContains(t, body, `action="/usage/plan"`)
}

func TestNewReportStats(t *testing.T) {
	type testCase struct {
		_id       string
		_expected reportStats

		events *peek.SessionEvents
	}

	usage := peek.TokenUsage{
		CacheCreationInputTokens: 150,
		CacheReadInputTokens:     750,
		InputTokens:              100,
		OutputTokens:             2500,
	}
	tests := make([]*testCase, 0)

	// with-telemetry
	tests = append(tests, &testCase{
		_id: "with-telemetry",
		_expected: reportStats{
			ActiveTime:   "1h",
			CacheHit:     "75%",
			CacheRead:    "750",
			CacheWrite:   "150",
			Cost:         "$0.75",
			IdleTime:     "30m",
			InputTokens:  "100",
			OutputTokens: "2 500",
			Runtime:      "1h30m",
			TotalTokens:  "3 500",
		},

		events: &peek.SessionEvents{
			ActiveSeconds: 3600,
			CostUsd:       0.75,
			HasTelemetry:  true,
			IdleSeconds:   1800,
			Usage:         usage,
			WallSeconds:   5400,
		},
	})

	// without-telemetry
	tests = append(tests, &testCase{
		_id: "without-telemetry",
		_expected: reportStats{
			ActiveTime:   "45s",
			CacheHit:     "75%",
			CacheRead:    "750",
			CacheWrite:   "150",
			Cost:         "—",
			IdleTime:     "0s",
			InputTokens:  "100",
			OutputTokens: "2 500",
			Runtime:      "45s",
			TotalTokens:  "3 500",
		},

		events: &peek.SessionEvents{
			ActiveSeconds: 45,
			CostUsd:       0,
			HasTelemetry:  false,
			IdleSeconds:   0,
			Usage:         usage,
			WallSeconds:   45,
		},
	})

	// no-input-tokens
	tests = append(tests, &testCase{
		_id: "no-input-tokens",
		_expected: reportStats{
			ActiveTime:   "0s",
			CacheHit:     "",
			CacheRead:    "0",
			CacheWrite:   "0",
			Cost:         "—",
			IdleTime:     "0s",
			InputTokens:  "0",
			OutputTokens: "0",
			Runtime:      "0s",
			TotalTokens:  "0",
		},

		events: &peek.SessionEvents{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, newReportStats(test.events))
		})
	}
}

func TestRenderSessionReportMarkdown(t *testing.T) {
	started := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	events := &peek.SessionEvents{
		InvokedSkills: []peek.SkillInvocation{{Skill: "fdesign", Source: "slash", Timestamp: started}},
		LastActive:    started.Add(time.Hour),
		StartedAt:     started,
		WallSeconds:   3600,
	}
	chip := idChip{Id: "RULE-GOLANG-NAME-001", Title: "Exported names"}
	families := []idFamily{{Chips: []idChip{chip}, Name: "RULE-GOLANG-NAME"}}
	record := &contextrecord.Record{Agent: contextrecord.SessionClaude}
	agents := []agentRow{{
		Chars:       "15",
		Description: "",
		Id:          "main",
		Items:       "2",
		Kind:        "Root",
		Model:       "claude-opus-5-5",
		Name:        "main",
		Pending:     "1",
		Tokens:      "12",
	}}
	fileReadRows := []sectionRow{{
		Agents:   "2 agents",
		Chars:    "4 200",
		Count:    3,
		Coverage: coverageBadge{Class: "badge-ok", Text: "read in full"},
		Families: families,
		Lines:    "lines 1–1327",
		Subject:  pathCell{Prefix: "/repo/", Rel: "context/rules/go.md"},
		Tokens:   tokenCell{Text: "1 050"},
	}}
	globalRows := []sectionRow{{
		Agents:  "main",
		Chars:   "10",
		Count:   1,
		Subject: pathCell{Prefix: "/home/.claude/", Rel: "context-required/s/global.md"},
		Tokens:  tokenCell{Text: "12"},
	}}
	fileGroups := []touchedFileGroup{{
		Files: []touchedFile{{Agents: "main", Path: pathCell{Prefix: "/repo/", Rel: "a.go"}, Reads: 2, Writes: 1}},
		Name:  ".go",
	}}
	hookRows := []sectionRow{{
		Agents:  "20 agents",
		Chars:   "5",
		Count:   20,
		Subject: pathCell{Rel: "Session title: call | set"},
		Tag:     mechanismTag{Class: tagKindHook, Text: "sessiontitle"},
		Tokens:  tokenCell{Class: "badge badge-dim", Text: "pending"},
	}}
	promptRows := []sectionRow{{
		Agents:  "main",
		Chars:   "900",
		Count:   1,
		Entries: 2,
		Paths:   []pathCell{{Prefix: "/repo/", Rel: "context/facts/repo.md"}},
		Subject: pathCell{Rel: "Ground the report"},
		Tokens:  tokenCell{Text: "220"},
	}}
	skillRows := []sectionRow{{
		Agents:   "main",
		Chars:    "300",
		Count:    2,
		Entries:  1,
		Families: families,
		Subject:  pathCell{Rel: "fdesign"},
		Tokens:   tokenCell{Text: "80"},
		Version:  "6.3.0",
	}}
	invokedSkills := []skillRow{{Ended: "—", Skill: "fdesign", SkillTokens: "3 400", Source: "slash", Started: "2000-01-01 00:00:00", Version: "6.3.0"}}
	kindSizes := map[string]sectionSize{
		string(contextrecord.KindGlobal): {Chars: "10 chars", Tokens: "12 tokens"},
		sizeKeyHooks:                     {Chars: "5 chars", Pending: "20 pending", Tokens: "0 tokens"},
	}
	page := &sessionReportPage{
		Agents:        agents,
		Branch:        "claude/report",
		Events:        events,
		FileGroups:    fileGroups,
		InvokedSkills: invokedSkills,
		KindSizes:     kindSizes,
		PathBase:      "/repo",
		PendingNotice: "1 injections pending: reload to continue.",
		Record:        record,
		Session:       peek.Session{Cwd: "/repo", Id: "abc123", Title: "CC,FD - Report - [WIP]"},
		Stats:         newReportStats(events),
		TokenNotice:   "Claude token counting is off",
		Totals:        agentRow{Chars: "15", Items: "2", Pending: "1", Tokens: "12"},
	}
	page.FileReadRows = fileReadRows
	page.GlobalRows = globalRows
	page.HookRows = hookRows
	page.PromptRows = promptRows
	page.SkillRows = skillRows
	filter := contextFilter{Aspect: "golang"}

	markdown := renderSessionReportMarkdown(filter, "abc123", page)

	// unscoped tables
	assert.Contains(t, markdown, "# Session context report — CC,FD - Report - [WIP]\n")
	assert.Contains(t, markdown, "- Filter: `aspect=golang`\n")
	assert.NotContains(t, markdown, "- Agents:")
	assert.Contains(t, markdown, "- Runtime: 1h (active 0s, idle 0s)\n")
	assert.Contains(t, markdown, "- Branch: `claude/report`\n")
	assert.Contains(t, markdown, "## Ingested context\n\nPaths relative to `/repo`\n\n### Injected context by agent\n\n")
	assert.Contains(t, markdown, "Claude token counting is off\n\n1 injections pending: reload to continue.\n\n")
	assert.Contains(t, markdown, "| Root main |  | claude-opus-5-5 | 2 | 15 | 12 | 1 |\n")
	assert.Contains(t, markdown, "| **Total** | | | 2 | 15 | 12 | 1 |\n")
	assert.Contains(t, markdown, "### SessionStart [global-context] — 10 chars · 12 tokens\n\n"+
		"| Source | Agents | Injections | Chars | Tokens |\n|---|---|---|---|---|\n"+
		"| `context-required/s/global.md` | main | 1 | 10 | 12 |\n")
	assert.Contains(t, markdown, "| Skill | Entries | Agents | Injections | Chars | Tokens |\n|---|---|---|---|---|---|\n"+
		"| /fdesign v6.3.0 | 1 | main | 2 | 300 | 80 |\n")
	assert.Contains(t, markdown, "### ACDSL projections [acdsl]\n\n—\n")
	assert.Contains(t, markdown, "| Path | Lines | Coverage | Agents | Reads | Chars | Tokens |\n|---|---|---|---|---|---|---|\n"+
		"| `context/rules/go.md` | lines 1–1327 | read in full | 2 agents | 3 | 4 200 | 1 050 |\n")
	assert.Contains(t, markdown, "| Prompt | Spawned by | Paths | Ids | Chars | Tokens |\n|---|---|---|---|---|---|\n"+
		"| Ground the report | main | 1 | 2 | 900 | 220 |\n")
	assert.Contains(t, markdown, "### Hook injections — 5 chars · 0 tokens so far · 20 pending\n\n"+
		"| Hook | Subject | Agents | Injections | Chars | Tokens |\n|---|---|---|---|---|---|\n"+
		"| [sessiontitle] | Session title: call \\| set | 20 agents | 20 | 5 | pending |\n")
	assert.Contains(t, markdown, "| /fdesign | 6.3.0 | 3 400 | slash | 2000-01-01 00:00:00 | — |\n")
	assert.Contains(t, markdown, "### .go (1 files)\n\n- `a.go` — main · read 2 · write 1\n")

	// scoped header and figures
	page.Scope = agentScope{agentType: "Explore"}
	page.Totals = agentRow{Chars: "4", Items: "1", Tokens: "1"}

	scoped := renderSessionReportMarkdown(contextFilter{}, "abc123", page)

	assert.Contains(t, scoped, "- Session: `abc123`\n- Agents: `type:Explore`\n")
	assert.Contains(t, scoped, "| **Total** | | | 1 | 4 | 1 |  |\n")
}
