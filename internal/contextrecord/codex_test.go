package contextrecord

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCodex(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Record

		files     []FileTouch
		model     string
		sessionId string
		userTurns []string
	}

	pluginsBlock := "<recommended_plugins>\n- Box\n</recommended_plugins>"
	instructionsText := "# AGENTS.md instructions for /repo\n\n<INSTRUCTIONS>\n- be concise\n</INSTRUCTIONS>"
	environmentBlock := "<environment_context>\n  <cwd>/repo</cwd>\n</environment_context>"
	unclosedInstructions := "# AGENTS.md instructions for /repo\n\n<INSTRUCTIONS>\n- be concise"

	tests := make([]*testCase, 0)

	// live-turn-shape
	liveItems := []Item{
		{
			Chars:  50,
			Kind:   KindCodexEnvironment,
			Label:  "recommended_plugins",
			Lines:  3,
			Source: "turn",
			text:   pluginsBlock,
		},
		{
			Chars:  79,
			Kind:   KindCodexInstructions,
			Label:  "AGENTS.md instructions",
			Lines:  5,
			Source: "turn",
			text:   instructionsText,
		},
		{
			Chars:  63,
			Kind:   KindCodexEnvironment,
			Label:  "environment_context",
			Lines:  3,
			Source: "turn",
			text:   environmentBlock,
		},
	}
	liveTotals := Totals{Chars: 192, Items: 3, Lines: 11, Tokens: 0, Uncounted: 3}
	liveAgents := []Agent{
		{Id: "main", Context: &AgentContext{}, Items: liveItems, Kind: AgentRoot, Model: "gpt-6-astra", Totals: liveTotals},
	}
	tests = append(tests, &testCase{
		_id:       "live-turn-shape",
		_expected: &Record{Agent: "codex", Agents: liveAgents, SessionId: "s1", Totals: liveTotals},

		files:     nil,
		model:     "gpt-6-astra",
		sessionId: "s1",
		userTurns: []string{pluginsBlock + "\n" + instructionsText + "\n" + environmentBlock},
	})

	// plain-turn
	plainAgents := []Agent{
		{Id: "main", Context: &AgentContext{}, Items: []Item{}, Kind: AgentRoot, Model: "gpt-5.5", Totals: Totals{}},
	}
	tests = append(tests, &testCase{
		_id:       "plain-turn",
		_expected: &Record{Agent: "codex", Agents: plainAgents, SessionId: "s2", Totals: Totals{}},

		files:     nil,
		model:     "gpt-5.5",
		sessionId: "s2",
		userTurns: []string{"fix the failing test"},
	})

	// files-stored
	touchedFiles := []FileTouch{
		{Path: "/repo/a.go", Reads: 2, Writes: 1},
		{Path: "/repo/b.md", Reads: 1},
	}
	filesAgents := []Agent{
		{Id: "main", Context: &AgentContext{}, Files: touchedFiles, Items: []Item{}, Kind: AgentRoot, Model: "gpt-5.5", Totals: Totals{}},
	}
	tests = append(tests, &testCase{
		_id:       "files-stored",
		_expected: &Record{Agent: "codex", Agents: filesAgents, SessionId: "s4", Totals: Totals{}},

		files:     touchedFiles,
		model:     "gpt-5.5",
		sessionId: "s4",
		userTurns: []string{"fix the failing test"},
	})

	// unclosed-instructions
	unclosedItems := []Item{
		{
			Chars:  63,
			Kind:   KindCodexInstructions,
			Label:  "AGENTS.md instructions",
			Lines:  4,
			Source: "turn",
			text:   unclosedInstructions,
		},
	}
	unclosedTotals := Totals{Chars: 63, Items: 1, Lines: 4, Tokens: 0, Uncounted: 1}
	unclosedAgents := []Agent{
		{Id: "main", Context: &AgentContext{}, Items: unclosedItems, Kind: AgentRoot, Model: "gpt-6-astra", Totals: unclosedTotals},
	}
	tests = append(tests, &testCase{
		_id:       "unclosed-instructions",
		_expected: &Record{Agent: "codex", Agents: unclosedAgents, SessionId: "s3", Totals: unclosedTotals},

		files:     nil,
		model:     "gpt-6-astra",
		sessionId: "s3",
		userTurns: []string{"fix the failing test", unclosedInstructions},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ExtractCodex(test.sessionId, test.model, test.userTurns, test.files))
		})
	}
}
