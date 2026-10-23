package contextrecord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureRoot         = "testdata/root.jsonl"
	fixtureSubagentA1   = "testdata/root/subagents/agent-a1.jsonl"
	fixtureWorkflowW1   = "testdata/root/subagents/workflows/run1/agent-w1.jsonl"
	globalInjectionPath = "/home/u/.claude/context-required/sess-1/global.md"
	sessionTitleLabel   = "sessiontitle"
	subagentStartLabel  = "SubagentStart"
)

// itemSummary is the identity of an extracted item, sizes left out.
type itemSummary struct {
	Kind   Kind
	Label  string
	Source string
}

// guideAgent is an agent that read guide in the given spans, with the lang
// row its walk would have produced.
func guideAgent(guide string, spans ...[2]int) Agent {
	agent := Agent{Context: &AgentContext{}}
	for _, span := range spans {
		item := NewItem(KindLang, guide, SourceToolResult, "")
		item.StartLine, item.EndLine = span[0], span[1]
		agent.add(item)
	}
	row := LangContext{Guide: guide, Ids: []string{}, Lang: "go"}
	agent.Context.Injected.Lang = append(agent.Context.Injected.Lang, row)
	return agent
}

// langRows lists every agent's lang rows in agent order.
func langRows(agents []Agent) [][]LangContext {
	rows := make([][]LangContext, 0, len(agents))
	for index := range agents {
		rows = append(rows, agents[index].Context.Injected.Lang)
	}
	return rows
}

func summarize(items []Item) []itemSummary {
	summaries := make([]itemSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, itemSummary{Kind: item.Kind, Label: item.Label, Source: item.Source})
	}
	return summaries
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestExtract(t *testing.T) {
	record, err := Extract(fixtureRoot, "docs")
	require.NoError(t, err)

	// legacy fields, as test_context_record.sh expected them
	assert.Equal(t, SessionClaude, record.Agent)
	assert.Equal(t, "sess-1", record.SessionId)
	assert.Equal(t, "/repo/x", record.Cwd)
	assert.Equal(t, "claude/x", record.GitBranch)
	expectedSkills := []SkillContext{
		{Ids: []string{"ACTION-CONCEPT-HOT-001", "FACT-REPO-ARCH-001"}, Skill: "concept", Via: "marker"},
		{Ids: []string{"ACTION-PLAN-FMT-001"}, Skill: "fdesign", Version: "6.3.0", Via: "marker"},
	}
	assert.Equal(t, expectedSkills, record.Injected.Skill)
	expectedGlobal := []string{"docs/global/company.md", globalInjectionPath}
	assert.Equal(t, expectedGlobal, record.Injected.Global)
	require.Len(t, record.Injected.Lang, 1)
	assert.Equal(t, "go", record.Injected.Lang[0].Lang)
	assert.Equal(t, "/repo/x/docs/rules/go.md", record.Injected.Lang[0].Guide)
	assert.Empty(t, record.Injected.Lang[0].Ids)
	// the guide path does not exist on disk: the verdict comes from the reads
	assert.NoFileExists(t, record.Injected.Lang[0].Guide)
	assert.Equal(t, coverageComplete, record.Injected.Lang[0].Coverage)
	assert.Equal(t, 1, record.Injected.Lang[0].TotalLines)
	expectedRules := []FileRules{{File: "/repo/x/internal/a.go", Ids: []string{"ACDSL-GOLANG-FMT-001", "ACDSL-GOLANG-FUNC-001"}}}
	assert.Equal(t, expectedRules, record.AcdslRules)
	assert.Equal(t, []string{"ACDSL-GOLANG-ENUM-001"}, record.PlanRules)
	assert.Equal(t, []string{"/repo/x/docs/rules/go.md"}, record.PackReads)
	expectedSubagentContext := []SubagentContext{
		{Ids: []string{"ACTION-IMPL-001", "RULE-GOLANG-TEST-002"}, Paths: []string{"docs/actions/implementing.md"}},
	}
	assert.Equal(t, expectedSubagentContext, record.SubagentContext)

	// agents order root, a1, w1
	require.Len(t, record.Agents, 3)
	root, subagent, workflow := &record.Agents[0], &record.Agents[1], &record.Agents[2]
	assert.Equal(t, rootAgentId, root.Id)
	assert.Equal(t, AgentRoot, root.Kind)
	assert.Equal(t, fixtureRoot, root.Path)
	assert.Equal(t, "claude-fable-5-1", root.Model)

	// root context is the record-level id view
	expectedRootContext := &AgentContext{
		AcdslRules:      record.AcdslRules,
		Injected:        record.Injected,
		PackReads:       record.PackReads,
		PlanRules:       record.PlanRules,
		SubagentContext: record.SubagentContext,
	}
	assert.Equal(t, expectedRootContext, root.Context)

	// root files: Reads and the Edit, sorted by path
	expectedRootFiles := []FileTouch{
		{Path: "/home/u/.claude/context-required/sess-1/fdesign.md", Reads: 1},
		{Path: globalInjectionPath, Reads: 1},
		{Path: "/repo/a.go", Writes: 1},
		{Path: "/repo/x/docs/rules/go.md", Reads: 1},
		{Path: "/repo/x/internal/a.go", Reads: 1},
	}
	assert.Equal(t, expectedRootFiles, root.Files)

	// root items; the SessionStart hook_additional_context twin is deduped and
	// the allow-only PreToolUse decision yields no item
	expectedRootItems := []itemSummary{
		{Kind: KindSkill, Label: "concept", Source: SourceHookSuccess},
		{Kind: KindGlobal, Label: "SessionStart:startup", Source: SourceHookSuccess},
		{Kind: KindSkill, Label: "fdesign", Source: SourceHookSuccess},
		{Kind: KindAcdslProjection, Label: "/repo/x/internal/a.go", Source: SourceToolResult},
		{Kind: KindLang, Label: "/repo/x/docs/rules/go.md", Source: SourceToolResult},
		{Kind: KindPlanRules, Label: "go run ./cmd/acdsl project -plan internal/b.go", Source: SourceToolResult},
		{Kind: KindSubagentPrompt, Label: toolAgent, Source: SourceToolUse},
		{Kind: KindHookContext, Label: sessionTitleLabel, Source: SourceHookContext},
		{Kind: KindAgentsMd, Label: "/repo/x/context/AGENTS.md", Source: SourceHookContext},
		{Kind: KindGlobal, Label: globalInjectionPath, Source: SourceToolResult},
		{Kind: KindSkill, Label: "fdesign", Source: SourceToolResult},
	}
	assert.Equal(t, expectedRootItems, summarize(root.Items))

	// global injection measured on the numbered Read result: runes, not bytes
	injection := root.Items[9]
	assert.Equal(t, 38, injection.Chars)
	assert.Equal(t, 2, injection.Lines)
	assert.Equal(t, "     1\tglobal — rules\n     2\tline two\n", injection.Text())
	assert.Equal(t, "lines 1–2", injection.Range)
	assert.Equal(t, 1, injection.StartLine)
	assert.Equal(t, 2, injection.EndLine)

	// go guide Read result "rules"
	assert.Equal(t, 5, root.Items[4].Chars)
	assert.Equal(t, 1, root.Items[4].Lines)
	assert.Equal(t, "lines 1–1", root.Items[4].Range)
	assert.Equal(t, 1, root.Items[4].StartLine)
	assert.Equal(t, 1, root.Items[4].EndLine)

	// unmarked hook text: named by its hook, previewed by its first line
	assert.Equal(t, "Session title: call mcp__ccd_session_mgm…", root.Items[7].Preview)

	// a1: meta fields, SubagentStart pointer, projection cut at the blank line
	assert.Equal(t, "a1", subagent.Id)
	assert.Equal(t, AgentSubagent, subagent.Kind)
	assert.Equal(t, fixtureSubagentA1, subagent.Path)
	assert.Equal(t, "Explore", subagent.Type)
	assert.Equal(t, "Ground the report", subagent.Description)
	assert.Equal(t, 1, subagent.SpawnDepth)
	assert.Equal(t, "claude-opus-5-5", subagent.Model)
	expectedSubagentItems := []itemSummary{
		{Kind: KindGlobal, Label: subagentStartLabel, Source: SourceHookContext},
		{Kind: KindAcdslProjection, Label: "/repo/x/internal/b.go", Source: SourceToolResult},
		{Kind: KindPackRead, Label: "/repo/x/docs/actions/implementing.md", Source: SourceToolResult},
		{Kind: KindSubagentPrompt, Label: "Summarize", Source: SourceToolUse},
	}
	assert.Equal(t, expectedSubagentItems, summarize(subagent.Items))
	assert.Equal(t, "lines 3–4", subagent.Items[2].Range)
	assert.Equal(t, 3, subagent.Items[2].StartLine)
	assert.Equal(t, 4, subagent.Items[2].EndLine)
	assert.Equal(t, []string{"/repo/x/docs/actions/implementing.md"}, subagent.Context.PackReads)
	expectedSubagentPromptContext := []SubagentContext{{Ids: []string{}, Paths: []string{}}}
	assert.Equal(t, expectedSubagentPromptContext, subagent.Context.SubagentContext)
	projection := subagent.Items[1]
	// Built from pieces: the committed source must never carry the literal
	// projection shape, which the staged-leak guard rejects.
	expectedProjection := "     1\t// " + acdsl.ProjectionMarker + " 1 rule(s) govern this file — working-copy view, stripped before commit\n" +
		"     2\t// - " + "[ACDSL-GOLANG-FMT-001] gofmt"
	assert.Equal(t, expectedProjection, projection.Text())
	assert.Equal(t, 140, projection.Chars)
	assert.Equal(t, 2, projection.Lines)

	// w1: no meta, model from its transcript
	assert.Equal(t, "w1", workflow.Id)
	assert.Equal(t, AgentWorkflow, workflow.Kind)
	assert.Equal(t, fixtureWorkflowW1, workflow.Path)
	assert.Empty(t, workflow.Type)
	assert.Empty(t, workflow.Description)
	assert.Equal(t, 0, workflow.SpawnDepth)
	assert.Equal(t, "claude-sonnet-5", workflow.Model)
	expectedWorkflowItems := []itemSummary{{Kind: KindGlobal, Label: subagentStartLabel, Source: SourceHookContext}}
	assert.Equal(t, expectedWorkflowItems, summarize(workflow.Items))

	// every prompt has a subagent-context entry, paired by position
	for index := range record.Agents {
		agent := &record.Agents[index]
		prompts := 0
		for _, item := range agent.Items {
			if item.Kind == KindSubagentPrompt {
				prompts++
			}
		}
		assert.Lenf(t, agent.Context.SubagentContext, prompts, "agent %s", agent.Id)
	}

	// totals recomputed over every agent, nothing counted yet
	assert.Equal(t, 11, root.Totals.Items)
	assert.Equal(t, 16, record.Totals.Items)
	assert.Equal(t, 16, record.Totals.Uncounted)
	assert.Equal(t, 0, record.Totals.Tokens)
}

func TestExtract_MissingTranscript(t *testing.T) {
	type testCase struct {
		_id string

		path string
	}

	tests := make([]*testCase, 0)

	// root-missing
	tests = append(tests, &testCase{
		_id: "root-missing",

		path: filepath.Join(t.TempDir(), "missing.jsonl"),
	})

	// subagent-unreadable
	unreadableDir := t.TempDir()
	unreadableRoot := filepath.Join(unreadableDir, "root.jsonl")
	writeFile(t, unreadableRoot, `{"type":"user","cwd":"/repo/x","sessionId":"sess-2"}`+"\n")
	require.NoError(t, os.MkdirAll(filepath.Join(unreadableDir, "root", subagentsDir, "agent-x.jsonl"), 0o755))
	tests = append(tests, &testCase{
		_id: "subagent-unreadable",

		path: unreadableRoot,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := Extract(test.path, "docs")
			require.Error(t, err)
			assert.Nil(t, record)
		})
	}
}

func TestExtract_MalformedLine(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedChars int

		path string
	}

	tests := make([]*testCase, 0)
	identityLine := `{"type":"user","cwd":"/repo/x","sessionId":"sess-3"}` + "\n"
	hookLine := `{"type":"attachment","attachment":{"type":"hook_additional_context","hookName":"X","content":["%s"]}}` + "\n"

	// non-json-line-skipped
	malformedPath := filepath.Join(t.TempDir(), "malformed.jsonl")
	writeFile(t, malformedPath, identityLine+"{not json\n"+strings.Replace(hookLine, "%s", "hello", 1))
	tests = append(tests, &testCase{
		_id:            "non-json-line-skipped",
		_expectedChars: 5,

		path: malformedPath,
	})

	// oversized-line-within-buffer
	oversizedPath := filepath.Join(t.TempDir(), "oversized.jsonl")
	writeFile(t, oversizedPath, identityLine+strings.Replace(hookLine, "%s", strings.Repeat("x", 2<<20), 1))
	tests = append(tests, &testCase{
		_id:            "oversized-line-within-buffer",
		_expectedChars: 2 << 20,

		path: oversizedPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := Extract(test.path, "docs")
			require.NoError(t, err)
			assert.Equal(t, "sess-3", record.SessionId)
			require.Len(t, record.Agents, 1)
			require.Len(t, record.Agents[0].Items, 1)
			assert.Equal(t, KindHookContext, record.Agents[0].Items[0].Kind)
			assert.Equal(t, test._expectedChars, record.Agents[0].Items[0].Chars)
		})
	}
}

func TestCoverage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		spans [][2]int
		total int
	}

	tests := make([]*testCase, 0)

	// contiguous-to-total
	tests = append(tests, &testCase{
		_id:       "contiguous-to-total",
		_expected: coverageComplete,

		spans: [][2]int{{1, 500}, {501, 1000}, {1001, 1327}},
		total: 1327,
	})

	// gap
	tests = append(tests, &testCase{
		_id:       "gap",
		_expected: coveragePartial,

		spans: [][2]int{{1, 100}, {200, 300}},
		total: 300,
	})

	// short-of-total
	tests = append(tests, &testCase{
		_id:       "short-of-total",
		_expected: coveragePartial,

		spans: [][2]int{{1, 2000}, {2001, 2100}},
		total: 2500,
	})

	// starts-after-line-1
	tests = append(tests, &testCase{
		_id:       "starts-after-line-1",
		_expected: coveragePartial,

		spans: [][2]int{{2, 300}},
		total: 300,
	})

	// overlapping
	tests = append(tests, &testCase{
		_id:       "overlapping",
		_expected: coverageComplete,

		spans: [][2]int{{100, 300}, {1, 150}},
		total: 300,
	})

	// no-spans
	tests = append(tests, &testCase{
		_id:       "no-spans",
		_expected: coveragePartial,

		spans: nil,
		total: 300,
	})

	// zero-total
	tests = append(tests, &testCase{
		_id:       "zero-total",
		_expected: coveragePartial,

		spans: [][2]int{{0, 0}},
		total: 0,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, coverage(test.spans, test.total))
		})
	}
}

func TestReadSpan(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		endLine   int
		startLine int
	}

	tests := make([]*testCase, 0)

	// empty-result
	tests = append(tests, &testCase{
		_id:       "empty-result",
		_expected: "",

		endLine:   0,
		startLine: 0,
	})

	// lines-1-105
	tests = append(tests, &testCase{
		_id:       "lines-1-105",
		_expected: "lines 1–105",

		endLine:   105,
		startLine: 1,
	})

	// lines-435-614
	tests = append(tests, &testCase{
		_id:       "lines-435-614",
		_expected: "lines 435–614",

		endLine:   614,
		startLine: 435,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, readSpan(test.startLine, test.endLine))
		})
	}
}

func TestReturnedSpan(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedEndLine   int
		_expectedStartLine int

		text string
		use  *transcript.Block
	}

	tests := make([]*testCase, 0)

	// empty-result
	tests = append(tests, &testCase{
		_id:                "empty-result",
		_expectedEndLine:   0,
		_expectedStartLine: 0,

		text: "",
		use:  &transcript.Block{},
	})

	// default-offset
	tests = append(tests, &testCase{
		_id:                "default-offset",
		_expectedEndLine:   105,
		_expectedStartLine: 1,

		text: strings.Repeat("line\n", 105),
		use:  &transcript.Block{},
	})

	// offset-435-180-lines
	tests = append(tests, &testCase{
		_id:                "offset-435-180-lines",
		_expectedEndLine:   614,
		_expectedStartLine: 435,

		text: strings.Repeat("line\n", 180),
		use:  &transcript.Block{Input: transcript.ToolInput{Limit: 180, Offset: 435}},
	})

	// short-final-chunk
	tests = append(tests, &testCase{
		_id:                "short-final-chunk",
		_expectedEndLine:   1327,
		_expectedStartLine: 1001,

		text: strings.Repeat("line\n", 327),
		use:  &transcript.Block{Input: transcript.ToolInput{Limit: 500, Offset: 1001}},
	})

	// numbered-full-read
	tests = append(tests, &testCase{
		_id:                "numbered-full-read",
		_expectedEndLine:   3,
		_expectedStartLine: 1,

		text: "     1\t# Title\n     2\t\n     3\tlast line",
		use:  &transcript.Block{},
	})

	// numbered-past-final-newline
	tests = append(tests, &testCase{
		_id:                "numbered-past-final-newline",
		_expectedEndLine:   1327,
		_expectedStartLine: 900,

		text: "   900\tbody\n  1326\t| row |\n  1327\t| last |\n  1328\t",
		use:  &transcript.Block{Input: transcript.ToolInput{Limit: 450, Offset: 900}},
	})

	// numbered-empty-line-at-limit-is-real
	tests = append(tests, &testCase{
		_id:                "numbered-empty-line-at-limit-is-real",
		_expectedEndLine:   3,
		_expectedStartLine: 1,

		text: "     1\tbody\n     2\tmore\n     3\t",
		use:  &transcript.Block{Input: transcript.ToolInput{Limit: 3}},
	})

	// numbered-numbers-win-over-offset
	tests = append(tests, &testCase{
		_id:                "numbered-numbers-win-over-offset",
		_expectedEndLine:   451,
		_expectedStartLine: 450,

		text: "   450\tline\n   451\tline\n",
		use:  &transcript.Block{Input: transcript.ToolInput{Offset: 451}},
	})

	// numbered-only-empty-line
	tests = append(tests, &testCase{
		_id:                "numbered-only-empty-line",
		_expectedEndLine:   12,
		_expectedStartLine: 12,

		text: "    12\t",
		use:  &transcript.Block{Input: transcript.ToolInput{Offset: 12}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			startLine, endLine := returnedSpan(test.use, test.text)
			assert.Equal(t, test._expectedStartLine, startLine)
			assert.Equal(t, test._expectedEndLine, endLine)
		})
	}
}

func TestSettleCoverage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected [][]LangContext

		agents []Agent
	}

	const guide = "/repo/context/rules/go.md"
	settled := func(verdict string, totalLines int) []LangContext {
		rows := []LangContext{{Coverage: verdict, Guide: guide, Ids: []string{}, Lang: "go", TotalLines: totalLines}}
		return rows
	}
	tests := make([]*testCase, 0)

	// one-full-read
	tests = append(tests, &testCase{
		_id:       "one-full-read",
		_expected: [][]LangContext{settled(coverageComplete, 1327)},

		agents: []Agent{guideAgent(guide, [2]int{1, 1327})},
	})

	// three-chunks-adding-up
	tests = append(tests, &testCase{
		_id:       "three-chunks-adding-up",
		_expected: [][]LangContext{settled(coverageComplete, 1327)},

		agents: []Agent{guideAgent(guide, [2]int{1, 500}, [2]int{501, 1000}, [2]int{1001, 1327})},
	})

	// gap-between-chunks
	tests = append(tests, &testCase{
		_id:       "gap-between-chunks",
		_expected: [][]LangContext{settled(coveragePartial, 1327)},

		agents: []Agent{guideAgent(guide, [2]int{1, 500}, [2]int{601, 1327})},
	})

	// head-only-another-read-to-the-end
	tests = append(tests, &testCase{
		_id:       "head-only-another-read-to-the-end",
		_expected: [][]LangContext{settled(coveragePartial, 1327), settled(coverageComplete, 1327)},

		agents: []Agent{
			guideAgent(guide, [2]int{1, 500}),
			guideAgent(guide, [2]int{1, 1327}),
		},
	})

	// nobody-past-line-450
	tests = append(tests, &testCase{
		_id:       "nobody-past-line-450",
		_expected: [][]LangContext{settled(coverageComplete, 450), settled(coverageComplete, 450)},

		agents: []Agent{
			guideAgent(guide, [2]int{1, 200}, [2]int{201, 450}),
			guideAgent(guide, [2]int{1, 450}),
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			settleCoverage(test.agents)
			assert.Equal(t, test._expected, langRows(test.agents))
		})
	}
}

func TestScanIdentity(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   transcriptIdentity
		_shouldPass bool

		path string
	}

	transcriptDir := t.TempDir()
	tests := make([]*testCase, 0)

	// branch-on-first-line
	firstLinePath := filepath.Join(transcriptDir, "first.jsonl")
	writeFile(t, firstLinePath, `{"type":"user","cwd":"/repo/a","gitBranch":"claude/a","sessionId":"sess-a"}`+"\n")
	tests = append(tests, &testCase{
		_id:         "branch-on-first-line",
		_expected:   transcriptIdentity{cwd: "/repo/a", gitBranch: "claude/a", sessionId: "sess-a"},
		_shouldPass: true,

		path: firstLinePath,
	})

	// branch-on-a-later-line
	laterLinePath := filepath.Join(transcriptDir, "later.jsonl")
	laterLines := `{"type":"attachment","sessionId":"sess-b"}` + "\n" +
		`{"type":"user","cwd":"/repo/b","sessionId":"sess-b"}` + "\n" +
		`{"type":"assistant","gitBranch":"claude/b","sessionId":"sess-b"}` + "\n"
	writeFile(t, laterLinePath, laterLines)
	tests = append(tests, &testCase{
		_id:         "branch-on-a-later-line",
		_expected:   transcriptIdentity{cwd: "/repo/b", gitBranch: "claude/b", sessionId: "sess-b"},
		_shouldPass: true,

		path: laterLinePath,
	})

	// no-branch
	noBranchPath := filepath.Join(transcriptDir, "no-branch.jsonl")
	writeFile(t, noBranchPath, `{"type":"user","cwd":"/repo/c","sessionId":"sess-c"}`+"\n")
	tests = append(tests, &testCase{
		_id:         "no-branch",
		_expected:   transcriptIdentity{cwd: "/repo/c", sessionId: "sess-c"},
		_shouldPass: true,

		path: noBranchPath,
	})

	// missing-file
	tests = append(tests, &testCase{
		_id:         "missing-file",
		_expected:   transcriptIdentity{},
		_shouldPass: false,

		path: filepath.Join(transcriptDir, "missing.jsonl"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			identity, err := scanIdentity(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, identity)
		})
	}
}

func TestSubagentTranscripts(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Agent

		stem string
	}

	tests := make([]*testCase, 0)

	// plain-before-workflow
	fixtureStem := strings.TrimSuffix(fixtureRoot, transcriptSuffix)
	tests = append(tests, &testCase{
		_id: "plain-before-workflow",
		_expected: []Agent{
			{Id: "a1", Description: "Ground the report", Kind: AgentSubagent, Model: "opus", Path: fixtureSubagentA1, SpawnDepth: 1, Type: "Explore"},
			{Id: "w1", Kind: AgentWorkflow, Path: fixtureWorkflowW1},
		},

		stem: fixtureStem,
	})

	// sorted-by-path
	sortedStem := filepath.Join(t.TempDir(), "root")
	for _, relative := range []string{"agent-b.jsonl", "agent-a.jsonl", "workflows/run2/agent-c.jsonl", "workflows/run1/agent-d.jsonl"} {
		writeFile(t, filepath.Join(sortedStem, subagentsDir, relative), "")
	}
	tests = append(tests, &testCase{
		_id: "sorted-by-path",
		_expected: []Agent{
			{Id: "a", Kind: AgentSubagent, Path: filepath.Join(sortedStem, subagentsDir, "agent-a.jsonl")},
			{Id: "b", Kind: AgentSubagent, Path: filepath.Join(sortedStem, subagentsDir, "agent-b.jsonl")},
			{Id: "d", Kind: AgentWorkflow, Path: filepath.Join(sortedStem, subagentsDir, "workflows/run1/agent-d.jsonl")},
			{Id: "c", Kind: AgentWorkflow, Path: filepath.Join(sortedStem, subagentsDir, "workflows/run2/agent-c.jsonl")},
		},

		stem: sortedStem,
	})

	// no-subagents-dir
	tests = append(tests, &testCase{
		_id:       "no-subagents-dir",
		_expected: nil,

		stem: filepath.Join(t.TempDir(), "root"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, subagentTranscripts(test.stem))
		})
	}
}

func TestWalk_AddResult_Projection(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedKinds      []Kind
		_expectedRuleFiles  []string
		_expectedRuleCounts []int

		result *transcript.Block
		use    *transcript.Block
	}

	headBlock := "     1\t// " + acdsl.ProjectionMarker + " 2 rule(s) govern this file\n" +
		"     2\t// - " + "[ACDSL-GOLANG-FMT-001] gofmt\n" +
		"     3\t// - " + "[ACDSL-GOLANG-LINT-001] lint\n" +
		"     4\t\n" +
		"     5\tpackage x\n" +
		"     6\t// - " + "[ACDSL-GOLANG-ENUM-001] quoted in the body\n"
	quoted := "plans/x.md:40:// " + acdsl.ProjectionMarker + " 6 rule(s) govern this file\n" +
		"plans/x.md:41:// - " + "[ACDSL-GOLANG-FMT-001] gofmt\n"
	result := func(id, text string, isError bool) *transcript.Block {
		block := &transcript.Block{Content: transcript.Content{Text: text}, IsError: isError, ToolUseId: id}
		return block
	}
	tests := make([]*testCase, 0)

	// read-at-file-head
	tests = append(tests, &testCase{
		_id:                 "read-at-file-head",
		_expectedKinds:      []Kind{KindAcdslProjection},
		_expectedRuleFiles:  []string{"/repo/x/internal/a.go"},
		_expectedRuleCounts: []int{2},

		result: result("t1", headBlock, false),
		use:    &transcript.Block{Id: "t1", Input: transcript.ToolInput{FilePath: "/repo/x/internal/a.go"}, Name: toolRead},
	})

	// grep-output-quoting-a-block
	tests = append(tests, &testCase{
		_id:                 "grep-output-quoting-a-block",
		_expectedKinds:      []Kind{},
		_expectedRuleFiles:  []string{},
		_expectedRuleCounts: []int{},

		result: result("t2", quoted, false),
		use:    &transcript.Block{Id: "t2", Name: "Grep"},
	})

	// bash-output-quoting-a-block
	tests = append(tests, &testCase{
		_id:                 "bash-output-quoting-a-block",
		_expectedKinds:      []Kind{},
		_expectedRuleFiles:  []string{},
		_expectedRuleCounts: []int{},

		result: result("t3", headBlock, false),
		use:    &transcript.Block{Id: "t3", Input: transcript.ToolInput{Command: "cat internal/a.go"}, Name: toolBash},
	})

	// mcp-output-quoting-a-block
	tests = append(tests, &testCase{
		_id:                 "mcp-output-quoting-a-block",
		_expectedKinds:      []Kind{},
		_expectedRuleFiles:  []string{},
		_expectedRuleCounts: []int{},

		result: result("t4", headBlock, false),
		use:    &transcript.Block{Id: "t4", Name: "mcp__peek-mcp__session_get"},
	})

	// errored-read
	tests = append(tests, &testCase{
		_id:                 "errored-read",
		_expectedKinds:      []Kind{},
		_expectedRuleFiles:  []string{},
		_expectedRuleCounts: []int{},

		result: result("t5", headBlock, true),
		use:    &transcript.Block{Id: "t5", Input: transcript.ToolInput{FilePath: "/repo/x/internal/a.go"}, Name: toolRead},
	})

	// unknown-tool-use
	tests = append(tests, &testCase{
		_id:                 "unknown-tool-use",
		_expectedKinds:      []Kind{},
		_expectedRuleFiles:  []string{},
		_expectedRuleCounts: []int{},

		result: result("t6", headBlock, false),
		use:    &transcript.Block{Id: "other", Name: toolRead},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			agent := &Agent{}
			walker := newWalk(agent, "docs")
			walker.addUse(test.use)
			walker.addResult(test.result)

			kinds := []Kind{}
			for _, item := range agent.Items {
				kinds = append(kinds, item.Kind)
			}
			files := []string{}
			counts := []int{}
			for _, rules := range agent.Context.AcdslRules {
				files = append(files, rules.File)
				counts = append(counts, len(rules.Ids))
			}
			assert.Equal(t, test._expectedKinds, kinds)
			assert.Equal(t, test._expectedRuleFiles, files)
			assert.Equal(t, test._expectedRuleCounts, counts)
		})
	}
}

func TestTouchFile(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]*FileTouch

		blocks []*transcript.Block
		walker *walk
	}

	fileBlock := func(name, path string) *transcript.Block {
		block := &transcript.Block{Input: transcript.ToolInput{FilePath: path}, Name: name}
		return block
	}
	tests := make([]*testCase, 0)

	// read
	tests = append(tests, &testCase{
		_id:       "read",
		_expected: map[string]*FileTouch{"/repo/a.go": {Path: "/repo/a.go", Reads: 1}},

		blocks: []*transcript.Block{fileBlock(toolRead, "/repo/a.go")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// edit
	tests = append(tests, &testCase{
		_id:       "edit",
		_expected: map[string]*FileTouch{"/repo/a.go": {Path: "/repo/a.go", Writes: 1}},

		blocks: []*transcript.Block{fileBlock(toolEdit, "/repo/a.go")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// write
	tests = append(tests, &testCase{
		_id:       "write",
		_expected: map[string]*FileTouch{"/repo/a.go": {Path: "/repo/a.go", Writes: 1}},

		blocks: []*transcript.Block{fileBlock(toolWrite, "/repo/a.go")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// multi-edit
	tests = append(tests, &testCase{
		_id:       "multi-edit",
		_expected: map[string]*FileTouch{"/repo/a.go": {Path: "/repo/a.go", Writes: 1}},

		blocks: []*transcript.Block{fileBlock(toolMultiEdit, "/repo/a.go")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// notebook-edit
	notebookBlock := &transcript.Block{Name: toolNotebookEdit, Input: transcript.ToolInput{NotebookPath: "/repo/a.ipynb"}}
	tests = append(tests, &testCase{
		_id:       "notebook-edit",
		_expected: map[string]*FileTouch{"/repo/a.ipynb": {Path: "/repo/a.ipynb", Writes: 1}},

		blocks: []*transcript.Block{notebookBlock},
		walker: newWalk(&Agent{}, "docs"),
	})

	// notebook-edit-file-path-ignored
	tests = append(tests, &testCase{
		_id:       "notebook-edit-file-path-ignored",
		_expected: map[string]*FileTouch{},

		blocks: []*transcript.Block{fileBlock(toolNotebookEdit, "/repo/a.ipynb")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// bash-ignored
	tests = append(tests, &testCase{
		_id:       "bash-ignored",
		_expected: map[string]*FileTouch{},

		blocks: []*transcript.Block{fileBlock(toolBash, "/repo/a.go")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// empty-path-ignored
	tests = append(tests, &testCase{
		_id:       "empty-path-ignored",
		_expected: map[string]*FileTouch{},

		blocks: []*transcript.Block{fileBlock(toolRead, "")},
		walker: newWalk(&Agent{}, "docs"),
	})

	// same-path-twice-sums
	tests = append(tests, &testCase{
		_id:       "same-path-twice-sums",
		_expected: map[string]*FileTouch{"/repo/a.go": {Path: "/repo/a.go", Reads: 2, Writes: 1}},

		blocks: []*transcript.Block{
			fileBlock(toolRead, "/repo/a.go"),
			fileBlock(toolRead, "/repo/a.go"),
			fileBlock(toolEdit, "/repo/a.go"),
		},
		walker: newWalk(&Agent{}, "docs"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			for _, block := range test.blocks {
				test.walker.touchFile(block)
			}
			assert.Equal(t, test._expected, test.walker.files)
		})
	}
}
