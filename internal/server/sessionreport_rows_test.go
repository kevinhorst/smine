package server

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/stretchr/testify/assert"
)

var (
	testGuide = filepath.FromSlash("/repo/context/rules/go.md")
	testHome  = filepath.FromSlash("/home/k/.claude")
	testRepo  = filepath.FromSlash("/repo")
)

// summarizeGroups renders each group as "label: agent×count, …" for
// comparison.
func summarizeGroups(groups []*rowGroup) []string {
	summaries := make([]string, 0, len(groups))
	for _, group := range groups {
		parts := make([]string, 0, len(group.order))
		for _, agentId := range group.order {
			parts = append(parts, fmt.Sprintf("%s×%d", agentId, len(group.byAgent[agentId])))
		}
		summaries = append(summaries, group.first().Label+": "+strings.Join(parts, ", "))
	}
	return summaries
}

// summarizeProjections renders each projection row as "file · reads · rules
// · chars".
func summarizeProjections(rows []sectionRow) []string {
	summaries := make([]string, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, fmt.Sprintf("%s · %d reads · %d rules · %s", row.Subject.Rel, row.Count, row.Entries, row.Chars))
	}
	return summaries
}

// summarizePrompts renders each prompt row as its subject, spawning agent,
// shown and cut text runes, paths and ids.
func summarizePrompts(rows []sectionRow) []string {
	summaries := make([]string, 0, len(rows))
	for _, row := range rows {
		paths := make([]string, 0, len(row.Paths))
		for _, path := range row.Paths {
			paths = append(paths, path.Rel)
		}
		var ids []string
		for _, family := range row.Families {
			for _, chip := range family.Chips {
				ids = append(ids, chip.Id)
			}
		}
		summary := fmt.Sprintf("%s · %s · text %d+%d · paths %v · ids %v", row.Subject.Rel, row.Agents, utf8.RuneCountInString(row.Text), row.TextRest, paths, ids)
		summaries = append(summaries, summary)
	}
	return summaries
}

func guideRead(startLine, endLine int) contextrecord.Item {
	item := contextrecord.Item{Chars: 100, EndLine: endLine, Kind: contextrecord.KindLang, Label: testGuide, StartLine: startLine}
	return item
}

func guideContext(coverage string, totalLines int) *contextrecord.AgentContext {
	lang := contextrecord.LangContext{Coverage: coverage, Guide: testGuide, Ids: []string{"RULE-GOLANG-NAME-001"}, Lang: "go", TotalLines: totalLines}
	view := &contextrecord.AgentContext{Injected: contextrecord.InjectedContext{Lang: []contextrecord.LangContext{lang}}}
	return view
}

func TestGroupItems(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		agents []contextrecord.Agent
	}

	hookA := contextrecord.Item{Kind: contextrecord.KindHookContext, Label: "a"}
	hookB := contextrecord.Item{Kind: contextrecord.KindHookContext, Label: "b"}
	read := contextrecord.Item{Kind: contextrecord.KindPackRead, Label: "a"}
	tests := make([]*testCase, 0)

	// shared-subject-over-two-agents
	tests = append(tests, &testCase{
		_id:       "shared-subject-over-two-agents",
		_expected: []string{"a: main×2, a1×1"},

		agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{hookA, hookA}},
			{Id: "a1", Items: []contextrecord.Item{hookA}},
		},
	})

	// first-seen-order
	tests = append(tests, &testCase{
		_id:       "first-seen-order",
		_expected: []string{"b: main×1", "a: main×1, a1×1"},

		agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{hookB, hookA}},
			{Id: "a1", Items: []contextrecord.Item{hookA}},
		},
	})

	// item-outside-the-kinds
	tests = append(tests, &testCase{
		_id:       "item-outside-the-kinds",
		_expected: []string{"a: a1×1"},

		agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{read}},
			{Id: "a1", Items: []contextrecord.Item{hookA, read}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, summarizeGroups(groupItems(test.agents, ofKind(contextrecord.KindHookContext), itemLabel)))
		})
	}
}

func TestSortRowsByTokens(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		rows []sectionRow
	}

	row := func(subject string, tokens, chars int) sectionRow {
		return sectionRow{Subject: pathCell{Rel: subject}, charsValue: chars, tokensValue: tokens}
	}
	tests := make([]*testCase, 0)

	// tokens-descending
	tests = append(tests, &testCase{
		_id:       "tokens-descending",
		_expected: []string{"b", "a"},

		rows: []sectionRow{row("a", 10, 900), row("b", 20, 100)},
	})

	// chars-break-ties
	tests = append(tests, &testCase{
		_id:       "chars-break-ties",
		_expected: []string{"b", "a"},

		rows: []sectionRow{row("a", 0, 100), row("b", 0, 900)},
	})

	// uncounted-after-counted
	tests = append(tests, &testCase{
		_id:       "uncounted-after-counted",
		_expected: []string{"c", "a", "b"},

		rows: []sectionRow{row("a", 0, 5000), row("b", 0, 10), row("c", 1, 1)},
	})

	// subject-breaks-full-ties
	tests = append(tests, &testCase{
		_id:       "subject-breaks-full-ties",
		_expected: []string{"a", "b"},

		rows: []sectionRow{row("b", 3, 3), row("a", 3, 3)},
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: nil,

		rows: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			var subjects []string
			for _, sorted := range sortRowsByTokens(test.rows) {
				subjects = append(subjects, sorted.Subject.Rel)
			}
			assert.Equal(t, test._expected, subjects)
		})
	}
}

func TestSumTokenCell(t *testing.T) {
	type testCase struct {
		_id       string
		_expected tokenCell

		items []*contextrecord.Item
	}

	counted := &contextrecord.Item{Counter: contextrecord.CounterAnthropic, Tokens: 12000}
	approx := &contextrecord.Item{Approx: true, Counter: contextrecord.CounterO200k, Tokens: 345}
	queued := &contextrecord.Item{Uncounted: contextrecord.UncountedQueued}
	disabled := &contextrecord.Item{Uncounted: contextrecord.UncountedDisabled}
	failed := &contextrecord.Item{Error: "boom"}
	tests := make([]*testCase, 0)

	// all-counted
	tests = append(tests, &testCase{
		_id:       "all-counted",
		_expected: tokenCell{Text: "≈12 345"},

		items: []*contextrecord.Item{counted, approx},
	})

	// some-pending
	tests = append(tests, &testCase{
		_id:       "some-pending",
		_expected: tokenCell{Text: "12 000 · 2 pending", Title: "not every injection is counted yet"},

		items: []*contextrecord.Item{counted, queued, queued},
	})

	// none-counted
	tests = append(tests, &testCase{
		_id:       "none-counted",
		_expected: tokenCell{Class: cellBadgeDim, Text: wordPending, Title: "not counted yet — a background run counts it"},

		items: []*contextrecord.Item{queued, queued},
	})

	// off
	tests = append(tests, &testCase{
		_id:       "off",
		_expected: tokenCell{Class: cellBadgeDim, Text: wordOff, Title: "token counting is off"},

		items: []*contextrecord.Item{disabled},
	})

	// error
	tests = append(tests, &testCase{
		_id:       "error",
		_expected: tokenCell{Class: cellBadgeDim, Text: wordCountFailed, Title: "boom"},

		items: []*contextrecord.Item{failed, queued},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sumTokenCell(test.items))
		})
	}
}

func TestUnionLines(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		items []*contextrecord.Item
	}

	span := func(startLine, endLine int) *contextrecord.Item {
		return &contextrecord.Item{EndLine: endLine, StartLine: startLine}
	}
	tests := make([]*testCase, 0)

	// one-span
	tests = append(tests, &testCase{
		_id:       "one-span",
		_expected: "lines 1–1327",

		items: []*contextrecord.Item{span(1, 1327)},
	})

	// adjacent-merged
	tests = append(tests, &testCase{
		_id:       "adjacent-merged",
		_expected: "lines 1–1327",

		items: []*contextrecord.Item{span(1, 450), span(451, 900), span(880, 1327)},
	})

	// gap
	tests = append(tests, &testCase{
		_id:       "gap",
		_expected: "lines 1–450, 901–1327",

		items: []*contextrecord.Item{span(1, 450), span(901, 1327)},
	})

	// unordered
	tests = append(tests, &testCase{
		_id:       "unordered",
		_expected: "lines 1–900, 1000–1100",

		items: []*contextrecord.Item{span(1000, 1100), span(451, 900), span(1, 450)},
	})

	// none
	tests = append(tests, &testCase{
		_id:       "none",
		_expected: "",

		items: []*contextrecord.Item{{}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, unionLines(test.items))
		})
	}
}

func TestFileReadRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []sectionRow

		record *contextrecord.Record
	}

	dash := tokenCell{Text: "—"}
	guidePath := pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: filepath.FromSlash("context/rules/go.md")}
	families := []idFamily{{Chips: []idChip{{Id: "RULE-GOLANG-NAME-001"}}, Name: "RULE-GOLANG-NAME"}}
	fullCoverage := coverageBadge{Class: badgeOk, Text: "read in full"}
	fullLine := agentLine{Agent: "main", Chars: "100", Count: 1, Lines: "lines 1–1327", Tokens: dash}
	exploreLine := agentLine{Agent: "Explore", Chars: "100", Count: 1, Lines: "lines 1–1327", Tokens: dash}
	packRead := contextrecord.Item{Chars: 40, EndLine: 40, Kind: contextrecord.KindPackRead, Label: filepath.FromSlash("/repo/context/actions/implementing.md"), StartLine: 1}
	tests := make([]*testCase, 0)

	// three-chunks-one-agent
	tests = append(tests, &testCase{
		_id: "three-chunks-one-agent",
		_expected: []sectionRow{{
			charsValue: 300,

			AgentLines: []agentLine{{Agent: "main", Chars: "300", Count: 3, Lines: "lines 1–1327", Tokens: dash}},
			Agents:     "main",
			Chars:      "300",
			Count:      3,
			Coverage:   fullCoverage,
			Entries:    1,
			Families:   families,
			HasDetail:  true,
			Lines:      "lines 1–1327",
			Subject:    guidePath,
			Tokens:     dash,
		}},

		record: &contextrecord.Record{Cwd: testRepo, Agents: []contextrecord.Agent{{
			Id:      "main",
			Context: guideContext(coverageComplete, 1327),
			Items:   []contextrecord.Item{guideRead(1, 450), guideRead(451, 900), guideRead(901, 1327)},
			Kind:    contextrecord.AgentRoot,
		}}},
	})

	// two-agents
	tests = append(tests, &testCase{
		_id: "two-agents",
		_expected: []sectionRow{{
			charsValue: 200,

			AgentLines: []agentLine{fullLine, exploreLine},
			Agents:     "2 agents",
			Chars:      "200",
			Count:      2,
			Coverage:   fullCoverage,
			Entries:    1,
			Families:   families,
			HasDetail:  true,
			Lines:      "lines 1–1327",
			Subject:    guidePath,
			Tokens:     dash,
		}},

		record: &contextrecord.Record{Cwd: testRepo, Agents: []contextrecord.Agent{
			{Id: "main", Context: guideContext(coverageComplete, 1327), Items: []contextrecord.Item{guideRead(1, 1327)}, Kind: contextrecord.AgentRoot},
			{Id: "a1", Context: guideContext(coverageComplete, 1327), Items: []contextrecord.Item{guideRead(1, 1327)}, Kind: contextrecord.AgentSubagent, Type: "Explore"},
		}},
	})

	// pack-read
	tests = append(tests, &testCase{
		_id: "pack-read",
		_expected: []sectionRow{{
			charsValue: 40,

			AgentLines: []agentLine{{Agent: "main", Chars: "40", Count: 1, Lines: "lines 1–40", Tokens: dash}},
			Agents:     "main",
			Chars:      "40",
			Count:      1,
			Lines:      "lines 1–40",
			Subject:    pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: filepath.FromSlash("context/actions/implementing.md")},
			Tokens:     dash,
		}},

		record: &contextrecord.Record{Cwd: testRepo, Agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{packRead}, Kind: contextrecord.AgentRoot},
		}},
	})

	// one-partial-reader
	tests = append(tests, &testCase{
		_id: "one-partial-reader",
		_expected: []sectionRow{{
			charsValue: 100,

			AgentLines: []agentLine{{Agent: "Explore", Chars: "100", Count: 1, Lines: "lines 1–450", Tokens: dash}},
			Agents:     "Explore",
			Chars:      "100",
			Count:      1,
			Coverage:   coverageBadge{Class: badgeAction, Text: "lines 1–450 of 1327"},
			Entries:    1,
			Families:   families,
			HasDetail:  true,
			Lines:      "lines 1–450",
			Subject:    guidePath,
			Tokens:     dash,
		}},

		record: &contextrecord.Record{Cwd: testRepo, Agents: []contextrecord.Agent{
			{Id: "a1", Context: guideContext(coveragePartial, 1327), Items: []contextrecord.Item{guideRead(1, 450)}, Kind: contextrecord.AgentSubagent, Type: "Explore"},
		}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, fileReadRows(testHome, contextFilter{}, map[string]idMeta{}, test.record))
		})
	}
}

func TestProjectionRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		filter contextFilter
	}

	projection := contextrecord.Item{Chars: 120, Kind: contextrecord.KindAcdslProjection, Label: filepath.FromSlash("/repo/internal/a.go")}
	rules := contextrecord.FileRules{File: filepath.FromSlash("/repo/internal/a.go"), Ids: []string{"ACDSL-GOLANG-FMT-001", "ACDSL-GOLANG-FUNC-001"}}
	record := &contextrecord.Record{Cwd: testRepo, Agents: []contextrecord.Agent{{
		Id:      "main",
		Context: &contextrecord.AgentContext{AcdslRules: []contextrecord.FileRules{rules, rules}},
		Items:   []contextrecord.Item{projection, projection},
		Kind:    contextrecord.AgentRoot,
	}}}
	tests := make([]*testCase, 0)

	// file-read-twice
	tests = append(tests, &testCase{
		_id:       "file-read-twice",
		_expected: []string{filepath.FromSlash("internal/a.go") + " · 2 reads · 2 rules · 240"},

		filter: contextFilter{},
	})

	// filter-empties-row
	tests = append(tests, &testCase{
		_id:       "filter-empties-row",
		_expected: []string{},

		filter: contextFilter{Layer: "rule"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, summarizeProjections(projectionRows(testHome, test.filter, map[string]idMeta{}, record)))
		})
	}
}

func TestSkillContextRows(t *testing.T) {
	hook := contextrecord.Item{Chars: 100, Kind: contextrecord.KindSkill, Label: "fdesign"}
	injectionRead := contextrecord.Item{Chars: 50, Kind: contextrecord.KindSkill, Label: "fdesign"}
	skill := contextrecord.SkillContext{Ids: []string{"ACTION-CONCEPT-HOT-006", "FACT-REPO-ARCH-001"}, Skill: "fdesign", Version: "6.3.0"}
	record := &contextrecord.Record{Agents: []contextrecord.Agent{{
		Id:      "main",
		Context: &contextrecord.AgentContext{Injected: contextrecord.InjectedContext{Skill: []contextrecord.SkillContext{skill}}},
		Items:   []contextrecord.Item{hook, injectionRead},
		Kind:    contextrecord.AgentRoot,
	}}}

	rows := skillContextRows(contextFilter{}, map[string]idMeta{}, record)

	// hook plus injection-file read summed, version, entries
	assert.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].Count)
	assert.Equal(t, "150", rows[0].Chars)
	assert.Equal(t, "6.3.0", rows[0].Version)
	assert.Equal(t, 2, rows[0].Entries)
	assert.Equal(t, pathCell{Rel: "fdesign"}, rows[0].Subject)
}

func TestPromptRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		record *contextrecord.Record
	}

	long := contextrecord.NewItem(contextrecord.KindSubagentPrompt, "Ground the report", contextrecord.SourceToolUse, strings.Repeat("x", contextrecord.DetailMaxRunes+5))
	short := contextrecord.NewItem(contextrecord.KindSubagentPrompt, "Review", contextrecord.SourceToolUse, "Read docs first.")
	inlined := []contextrecord.SubagentContext{
		{Ids: []string{"ACTION-IMPL-001"}},
		{Ids: []string{"RULE-GOLANG-TEST-002"}, Paths: []string{filepath.FromSlash("/repo/context/actions/implementing.md")}},
	}
	tests := make([]*testCase, 0)

	// text-capped-paths-and-families-by-position
	tests = append(tests, &testCase{
		_id: "text-capped-paths-and-families-by-position",
		_expected: []string{
			"Ground the report · main · text 4000+5 · paths [] · ids [ACTION-IMPL-001]",
			"Review · main · text 16+0 · paths [" + filepath.FromSlash("context/actions/implementing.md") + "] · ids [RULE-GOLANG-TEST-002]",
		},

		record: &contextrecord.Record{Cwd: testRepo, Agents: []contextrecord.Agent{{
			Id:      "main",
			Context: &contextrecord.AgentContext{SubagentContext: inlined},
			Items:   []contextrecord.Item{long, short},
			Kind:    contextrecord.AgentRoot,
		}}},
	})

	// no-prompts
	tests = append(tests, &testCase{
		_id:       "no-prompts",
		_expected: []string{},

		record: &contextrecord.Record{Agents: []contextrecord.Agent{{Id: "main", Kind: contextrecord.AgentRoot}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, summarizePrompts(promptRows(testHome, contextFilter{}, map[string]idMeta{}, test.record)))
		})
	}
}

func TestHookInjectionRows(t *testing.T) {
	harness := contextrecord.NewItem(contextrecord.KindAgentsMd, filepath.FromSlash("/repo/AGENTS.md"), contextrecord.SourceHookContext, "Contents of /repo/AGENTS.md:\n# Agents")
	agents := make([]contextrecord.Agent, 0, 20)
	for index := range 20 {
		agent := contextrecord.Agent{Id: fmt.Sprintf("a%d", index), Items: []contextrecord.Item{harness}, Kind: contextrecord.AgentSubagent}
		agents = append(agents, agent)
	}
	hookContext := contextrecord.NewItem(contextrecord.KindHookContext, "sessiontitle", contextrecord.SourceHookContext, "Session title: call\nmore")
	hookContext.Preview = "Session title: call"
	acdslContext := contextrecord.NewItem(contextrecord.KindAcdslContext, filepath.FromSlash("/repo/internal/a.go"), contextrecord.SourceHookContext, "acdsl")
	planContext := contextrecord.NewItem(contextrecord.KindPlanContext, "UserPromptSubmit", contextrecord.SourceHookContext, "plan")
	agents[0].Items = append(agents[0].Items, hookContext, acdslContext, planContext)
	record := &contextrecord.Record{Cwd: testRepo, Agents: agents}

	rows := hookInjectionRows(testHome, record)

	// 20 harness loads as one row
	assert.Len(t, rows, 4)
	assert.Equal(t, "20 agents", rows[0].Agents)
	assert.Equal(t, 20, rows[0].Count)
	assert.Len(t, rows[0].AgentLines, 20)
	// subject per kind
	assert.Equal(t, pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: "AGENTS.md"}, rows[0].Subject)
	assert.Equal(t, mechanismTag{Class: tagKindHook, Text: "harness"}, rows[0].Tag)
	assert.Equal(t, pathCell{Rel: "Session title: call"}, rows[1].Subject)
	assert.Equal(t, mechanismTag{Class: tagKindHook, Text: "sessiontitle"}, rows[1].Tag)
	assert.Equal(t, pathCell{Prefix: filepath.FromSlash("/repo/"), Rel: filepath.FromSlash("internal/a.go")}, rows[2].Subject)
	assert.Equal(t, pathCell{Rel: "UserPromptSubmit"}, rows[3].Subject)
	// text present
	assert.Equal(t, "Contents of /repo/AGENTS.md:\n# Agents", rows[0].Text)
	assert.Equal(t, "Session title: call\nmore", rows[1].Text)
	assert.True(t, rows[3].HasDetail)
}

func TestScopeCoverage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected coverageBadge

		langs []contextrecord.LangContext
		lines string
	}

	full := contextrecord.LangContext{Coverage: coverageComplete, TotalLines: 1327}
	partial := contextrecord.LangContext{Coverage: coveragePartial, TotalLines: 1327}
	seventeenOfEighteen := make([]contextrecord.LangContext, 0, 18)
	for range 17 {
		seventeenOfEighteen = append(seventeenOfEighteen, full)
	}
	seventeenOfEighteen = append(seventeenOfEighteen, partial)
	tests := make([]*testCase, 0)

	// all-full
	tests = append(tests, &testCase{
		_id:       "all-full",
		_expected: coverageBadge{Class: badgeOk, Text: "read in full"},

		langs: []contextrecord.LangContext{full, full},
		lines: "lines 1–1327",
	})

	// one-partial-reader
	tests = append(tests, &testCase{
		_id:       "one-partial-reader",
		_expected: coverageBadge{Class: badgeAction, Text: "lines 1–450 of 1327"},

		langs: []contextrecord.LangContext{partial},
		lines: "lines 1–450",
	})

	// seventeen-of-eighteen
	tests = append(tests, &testCase{
		_id:       "seventeen-of-eighteen",
		_expected: coverageBadge{Class: badgeAction, Text: "17 of 18 agents in full"},

		langs: seventeenOfEighteen,
		lines: "lines 1–1327",
	})

	// no-readers
	tests = append(tests, &testCase{
		_id:       "no-readers",
		_expected: coverageBadge{},

		langs: nil,
		lines: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, scopeCoverage(test.langs, test.lines))
		})
	}
}

func TestSectionCells(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCells   []string
		_expectedColumns int

		kind string
	}

	row := sectionRow{Entries: 3, Lines: "lines 1–40", Paths: []pathCell{{Rel: "a.md"}}}
	tests := make([]*testCase, 0)

	// files
	tests = append(tests, &testCase{
		_id:              "files",
		_expectedCells:   []string{"lines 1–40"},
		_expectedColumns: 7,

		kind: sectionFiles,
	})

	// hooks
	tests = append(tests, &testCase{
		_id:              "hooks",
		_expectedCells:   nil,
		_expectedColumns: 6,

		kind: sectionHooks,
	})

	// projections
	tests = append(tests, &testCase{
		_id:              "projections",
		_expectedCells:   []string{"3"},
		_expectedColumns: 6,

		kind: sectionProjections,
	})

	// prompts
	tests = append(tests, &testCase{
		_id:              "prompts",
		_expectedCells:   []string{"1", "3"},
		_expectedColumns: 6,

		kind: sectionPrompts,
	})

	// global
	tests = append(tests, &testCase{
		_id:              "global",
		_expectedCells:   nil,
		_expectedColumns: 5,

		kind: sectionGlobal,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			view := sectionCells(test.kind, row)
			assert.Equal(t, test._expectedCells, view.Cells)
			assert.Equal(t, test._expectedColumns, view.Columns)
		})
	}
}
