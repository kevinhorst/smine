package contextrecord

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewItem(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Item

		text string
	}

	tests := make([]*testCase, 0)

	// empty
	tests = append(tests, &testCase{
		_id: "empty",
		_expected: Item{
			Chars:  0,
			Kind:   KindGlobal,
			Label:  "label",
			Lines:  0,
			Source: SourceHookSuccess,
			text:   "",
		},

		text: "",
	})

	// one-line-no-newline
	tests = append(tests, &testCase{
		_id: "one-line-no-newline",
		_expected: Item{
			Chars:  5,
			Kind:   KindGlobal,
			Label:  "label",
			Lines:  1,
			Source: SourceHookSuccess,
			text:   "hello",
		},

		text: "hello",
	})

	// trailing-newline
	tests = append(tests, &testCase{
		_id: "trailing-newline",
		_expected: Item{
			Chars:  12,
			Kind:   KindGlobal,
			Label:  "label",
			Lines:  2,
			Source: SourceHookSuccess,
			text:   "hello\nworld\n",
		},

		text: "hello\nworld\n",
	})

	// multi-byte-runes
	tests = append(tests, &testCase{
		_id: "multi-byte-runes",
		_expected: Item{
			Chars:  11,
			Kind:   KindGlobal,
			Label:  "label",
			Lines:  2,
			Source: SourceHookSuccess,
			text:   "héllo\nwörld",
		},

		text: "héllo\nwörld",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			item := NewItem(KindGlobal, "label", SourceHookSuccess, test.text)
			assert.Equal(t, test._expected, item)
			assert.Equal(t, test.text, item.Text())
		})
	}
}

func TestItem_Capped(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedRest int
		_expectedText string

		item Item
	}

	tests := make([]*testCase, 0)

	// short-text
	tests = append(tests, &testCase{
		_id:           "short-text",
		_expectedRest: 0,
		_expectedText: "hello",

		item: NewItem(KindGlobal, "label", SourceHookSuccess, "hello"),
	})

	// exact-limit
	exactText := strings.Repeat("é", DetailMaxRunes)
	tests = append(tests, &testCase{
		_id:           "exact-limit",
		_expectedRest: 0,
		_expectedText: exactText,

		item: NewItem(KindGlobal, "label", SourceHookSuccess, exactText),
	})

	// over-limit-rest-counted
	tests = append(tests, &testCase{
		_id:           "over-limit-rest-counted",
		_expectedRest: 3,
		_expectedText: strings.Repeat("é", DetailMaxRunes),

		item: NewItem(KindGlobal, "label", SourceHookSuccess, strings.Repeat("é", DetailMaxRunes+3)),
	})

	// stored-detail-without-text
	tests = append(tests, &testCase{
		_id:           "stored-detail-without-text",
		_expectedRest: 12,
		_expectedText: "kept",

		item: Item{Detail: "kept", DetailRest: 12, Kind: KindGlobal},
	})

	// empty
	tests = append(tests, &testCase{
		_id:           "empty",
		_expectedRest: 0,
		_expectedText: "",

		item: NewItem(KindGlobal, "label", SourceHookSuccess, ""),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			text, rest := test.item.Capped()
			assert.Equal(t, test._expectedText, text)
			assert.Equal(t, test._expectedRest, rest)
		})
	}
}

func TestItem_MarshalJSON(t *testing.T) {
	item := NewItem(KindHookContext, "Session title", SourceHookContext, "secret text")

	encoded, err := json.Marshal(&item)
	require.NoError(t, err)

	expected := `{"chars":11,"kind":"hook_context","label":"Session title","lines":1,"source":"hook_additional_context"}`
	assert.JSONEq(t, expected, string(encoded))
	assert.NotContains(t, string(encoded), "secret")
	assert.NotContains(t, string(encoded), `"text"`)
}

func TestItem_SetTokens(t *testing.T) {
	item := NewItem(KindGlobal, "SessionStart", SourceHookSuccess, "hello")
	item.Uncounted = UncountedCap

	item.SetTokens(CounterO200k, 7, true)

	assert.True(t, item.Approx)
	assert.Equal(t, CounterO200k, item.Counter)
	assert.Equal(t, 7, item.Tokens)
	assert.Empty(t, item.Uncounted)
}

func TestRecord_AllIds(t *testing.T) {
	encoded := `{"session_id":"sid","agent":"claude","injected":{"global":["/home/.claude/context/global/style-profile.md"],"skill":[{"skill":"fdesign","ids":["RULE-PLAN-001","ACTION-IMPL-001"],"via":"marker"}],"lang":[{"lang":"go","guide":"/repo/context/rules/go.md","ids":["RULE-GOLANG-NAME-002"],"coverage":"complete"}]},"acdsl_rules":[{"file":"/repo/internal/peek/events.go","ids":["ACDSL-GOLANG-FUNC-001"]}],"plan_rules":["ACDSL-GOLANG-FMT-001","ACDSL-GOLANG-ENUM-001"],"pack_reads":["/repo/context/rules/plan.md"],"subagent_context":[{"ids":["FACT-REPO-ARCH-002"],"paths":["context/actions/implementing.md"]}],"agents":[{"agent_id":"main","kind":"root","items":[{"kind":"global","label":"SessionStart","source":"hook_success","chars":5,"lines":1}],"totals":{"chars":5,"items":1,"lines":1,"tokens":0,"uncounted":1}}],"totals":{"chars":5,"items":1,"lines":1,"tokens":0,"uncounted":1}}`

	var record Record
	require.NoError(t, json.Unmarshal([]byte(encoded), &record))

	assert.Equal(t, "sid", record.SessionId)
	assert.Equal(t, SessionClaude, record.Agent)

	require.Len(t, record.Injected.Global, 1)
	assert.Equal(t, "/home/.claude/context/global/style-profile.md", record.Injected.Global[0])

	require.Len(t, record.Injected.Skill, 1)
	assert.Equal(t, "fdesign", record.Injected.Skill[0].Skill)
	assert.Equal(t, []string{"RULE-PLAN-001", "ACTION-IMPL-001"}, record.Injected.Skill[0].Ids)
	assert.Equal(t, "marker", record.Injected.Skill[0].Via)

	require.Len(t, record.Injected.Lang, 1)
	assert.Equal(t, "go", record.Injected.Lang[0].Lang)
	assert.Equal(t, "/repo/context/rules/go.md", record.Injected.Lang[0].Guide)
	assert.Equal(t, "complete", record.Injected.Lang[0].Coverage)

	require.Len(t, record.AcdslRules, 1)
	assert.Equal(t, "/repo/internal/peek/events.go", record.AcdslRules[0].File)
	assert.Equal(t, []string{"ACDSL-GOLANG-FUNC-001"}, record.AcdslRules[0].Ids)

	assert.Equal(t, []string{"ACDSL-GOLANG-FMT-001", "ACDSL-GOLANG-ENUM-001"}, record.PlanRules)
	assert.Equal(t, []string{"/repo/context/rules/plan.md"}, record.PackReads)

	require.Len(t, record.SubagentContext, 1)
	assert.Equal(t, []string{"FACT-REPO-ARCH-002"}, record.SubagentContext[0].Ids)
	assert.Equal(t, []string{"context/actions/implementing.md"}, record.SubagentContext[0].Paths)

	expectedTotals := Totals{
		Chars:     5,
		Items:     1,
		Lines:     1,
		Tokens:    0,
		Uncounted: 1,
	}
	expectedAgents := []Agent{
		{
			Id: "main",
			Items: []Item{
				{
					Chars:  5,
					Kind:   KindGlobal,
					Label:  "SessionStart",
					Lines:  1,
					Source: SourceHookSuccess,
				},
			},
			Kind:   AgentRoot,
			Totals: expectedTotals,
		},
	}
	assert.Equal(t, expectedAgents, record.Agents)
	assert.Equal(t, expectedTotals, record.Totals)

	expectedIds := []string{
		"RULE-PLAN-001",
		"ACTION-IMPL-001",
		"RULE-GOLANG-NAME-002",
		"ACDSL-GOLANG-FUNC-001",
		"ACDSL-GOLANG-FMT-001",
		"ACDSL-GOLANG-ENUM-001",
		"FACT-REPO-ARCH-002",
	}
	assert.Equal(t, expectedIds, record.AllIds())

	reencoded, err := json.Marshal(&record)
	require.NoError(t, err)
	assert.JSONEq(t, encoded, string(reencoded))
}

func TestRecord_JsonAgentContext(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		agent Agent
	}

	tests := make([]*testCase, 0)

	// context-files-range-preview-version
	fullContext := &AgentContext{
		AcdslRules: []FileRules{
			{File: "/repo/internal/peek/events.go", Ids: []string{"ACDSL-GOLANG-FUNC-001"}},
		},
		Injected: InjectedContext{
			Lang: []LangContext{
				{Coverage: "complete", Guide: "/repo/context/rules/go.md", Ids: []string{"RULE-GOLANG-NAME-002"}, Lang: "go", TotalLines: 105},
			},
			Skill: []SkillContext{
				{Ids: []string{"RULE-PLAN-001"}, Skill: "fdesign", Version: "6.3.0", Via: "marker"},
			},
		},
		PackReads: []string{"/repo/context/rules/plan.md"},
		PlanRules: []string{"ACDSL-GOLANG-FMT-001"},
		SubagentContext: []SubagentContext{
			{Ids: []string{"FACT-REPO-ARCH-002"}, Paths: []string{"context/actions/implementing.md"}},
		},
	}
	fullFiles := []FileTouch{
		{Path: "/repo/a.go", Reads: 2, Writes: 1},
		{Path: "/repo/b.go", Reads: 1},
	}
	fullItems := []Item{
		{
			Chars:     5,
			EndLine:   105,
			Kind:      KindLang,
			Label:     "go",
			Lines:     105,
			Preview:   "# GO CODE STYLE",
			Range:     "1–105",
			Source:    SourceToolResult,
			StartLine: 1,
		},
	}
	tests = append(tests, &testCase{
		_id:       "context-files-range-preview-version",
		_expected: `{"agent_id":"main","context":{"acdsl_rules":[{"file":"/repo/internal/peek/events.go","ids":["ACDSL-GOLANG-FUNC-001"]}],"injected":{"lang":[{"coverage":"complete","guide":"/repo/context/rules/go.md","ids":["RULE-GOLANG-NAME-002"],"lang":"go","total_lines":105}],"skill":[{"ids":["RULE-PLAN-001"],"skill":"fdesign","version":"6.3.0","via":"marker"}]},"pack_reads":["/repo/context/rules/plan.md"],"plan_rules":["ACDSL-GOLANG-FMT-001"],"subagent_context":[{"ids":["FACT-REPO-ARCH-002"],"paths":["context/actions/implementing.md"]}]},"files":[{"path":"/repo/a.go","reads":2,"writes":1},{"path":"/repo/b.go","reads":1}],"items":[{"chars":5,"end_line":105,"kind":"lang","label":"go","lines":105,"preview":"# GO CODE STYLE","range":"1–105","source":"tool_result","start_line":1}],"kind":"root","totals":{"chars":0,"items":0,"lines":0,"tokens":0,"uncounted":0}}`,

		agent: Agent{
			Id:      "main",
			Context: fullContext,
			Files:   fullFiles,
			Items:   fullItems,
			Kind:    AgentRoot,
		},
	})

	// empty-context-omits-every-list
	tests = append(tests, &testCase{
		_id:       "empty-context-omits-every-list",
		_expected: `{"agent_id":"a1","context":{"injected":{}},"items":[],"kind":"subagent","totals":{"chars":0,"items":0,"lines":0,"tokens":0,"uncounted":0}}`,

		agent: Agent{
			Id:      "a1",
			Context: &AgentContext{},
			Items:   []Item{},
			Kind:    AgentSubagent,
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			encoded, err := json.Marshal(&test.agent)
			require.NoError(t, err)
			assert.JSONEq(t, test._expected, string(encoded))

			var decoded Agent
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			assert.Equal(t, test.agent, decoded)
		})
	}
}

func TestRecord_ByKind(t *testing.T) {
	counted := NewItem(KindGlobal, "SessionStart", SourceHookSuccess, "hello")
	counted.SetTokens(CounterAnthropic, 3, false)

	rootItems := []Item{
		counted,
		NewItem(KindLang, "go", SourceToolResult, "one\ntwo\n"),
	}
	subagentItems := []Item{
		NewItem(KindGlobal, "SubagentStart", SourceHookContext, "abc"),
		NewItem(KindSubagentPrompt, "Explore", SourceToolUse, "prompt"),
	}
	record := Record{
		Agents: []Agent{
			{Id: "main", Items: rootItems, Kind: AgentRoot},
			{Id: "a1", Items: subagentItems, Kind: AgentSubagent},
		},
	}

	expected := map[Kind]Totals{
		KindGlobal:         {Chars: 8, Items: 2, Lines: 2, Tokens: 3, Uncounted: 1},
		KindLang:           {Chars: 8, Items: 1, Lines: 2, Tokens: 0, Uncounted: 1},
		KindSubagentPrompt: {Chars: 6, Items: 1, Lines: 1, Tokens: 0, Uncounted: 1},
	}
	assert.Equal(t, expected, record.ByKind())
}

func TestRecord_KeepDetails(t *testing.T) {
	shortItem := NewItem(KindGlobal, "SessionStart", SourceHookSuccess, "hello")
	longItem := NewItem(KindSubagentPrompt, "Explore", SourceToolUse, strings.Repeat("x", DetailMaxRunes+5))
	record := Record{
		Agent: SessionClaude,
		Agents: []Agent{
			{Id: "main", Items: []Item{shortItem}, Kind: AgentRoot},
			{Id: "sub", Items: []Item{longItem}, Kind: AgentSubagent},
		},
	}

	record.KeepDetails()
	encoded, err := json.Marshal(&record)
	require.NoError(t, err)

	var loaded Record
	require.NoError(t, json.Unmarshal(encoded, &loaded))

	for agentIndex := range record.Agents {
		for itemIndex := range record.Agents[agentIndex].Items {
			expectedText, expectedRest := record.Agents[agentIndex].Items[itemIndex].Capped()
			loadedText, loadedRest := loaded.Agents[agentIndex].Items[itemIndex].Capped()

			assert.Equal(t, expectedText, loadedText)
			assert.Equal(t, expectedRest, loadedRest)
		}
	}
	assert.Equal(t, 5, loaded.Agents[1].Items[0].DetailRest)
	assert.Empty(t, loaded.Agents[0].Items[0].Text())
}

func TestRecord_Recompute(t *testing.T) {
	rootCounted := NewItem(KindGlobal, "SessionStart", SourceHookSuccess, "hello")
	rootCounted.SetTokens(CounterAnthropic, 3, false)

	subagentCounted := NewItem(KindSubagentPrompt, "Explore", SourceToolUse, "prompt\ntext")
	subagentCounted.SetTokens(CounterAnthropic, 4, false)

	rootItems := []Item{
		rootCounted,
		NewItem(KindLang, "go", SourceToolResult, "one\ntwo\n"),
	}
	subagentItems := []Item{
		subagentCounted,
	}
	record := Record{
		Agents: []Agent{
			{Id: "main", Items: rootItems, Kind: AgentRoot},
			{Id: "a1", Items: subagentItems, Kind: AgentSubagent},
		},
		Totals: Totals{Chars: 999},
	}

	record.Recompute()

	expectedRoot := Totals{Chars: 13, Items: 2, Lines: 3, Tokens: 3, Uncounted: 1}
	expectedSubagent := Totals{Chars: 11, Items: 1, Lines: 2, Tokens: 4, Uncounted: 0}
	expectedRecord := Totals{Chars: 24, Items: 3, Lines: 5, Tokens: 7, Uncounted: 1}
	assert.Equal(t, expectedRoot, record.Agents[0].Totals)
	assert.Equal(t, expectedSubagent, record.Agents[1].Totals)
	assert.Equal(t, expectedRecord, record.Totals)
}
