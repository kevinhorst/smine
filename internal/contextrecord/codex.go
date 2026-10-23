package contextrecord

import "strings"

const (
	codexInstructionsHeading = "# AGENTS.md instructions for"
	labelAgentsInstructions  = "AGENTS.md instructions"
	labelEnvironment         = "environment_context"
	labelPlugins             = "recommended_plugins"
	tagEnvironmentClose      = "</environment_context>"
	tagEnvironmentOpen       = "<environment_context>"
	tagInstructionsClose     = "</INSTRUCTIONS>"
	tagInstructionsOpen      = "<INSTRUCTIONS>"
	tagPluginsClose          = "</recommended_plugins>"
	tagPluginsOpen           = "<recommended_plugins>"
)

// AGENTS.md instructions, environment context and plugin list are injected; the rest is user text.
func ExtractCodex(sessionId, model string, userTurns []string, files []FileTouch) *Record {
	agent := Agent{Id: rootAgentId, Context: &AgentContext{}, Files: files, Items: []Item{}, Kind: AgentRoot, Model: model}
	for _, turn := range userTurns {
		for _, item := range codexItems(turn) {
			agent.add(item)
		}
	}
	record := &Record{Agent: SessionCodex, Agents: []Agent{agent}, SessionId: sessionId}
	record.Recompute()
	return record
}

// codexItems cuts the injected blocks out of one user turn.
func codexItems(turn string) []Item {
	var items []Item
	if block, ok := taggedBlock(turn, tagPluginsOpen, tagPluginsClose); ok {
		items = append(items, NewItem(KindCodexEnvironment, labelPlugins, SourceTurn, block))
	}
	if block, ok := instructionsBlock(turn); ok {
		items = append(items, NewItem(KindCodexInstructions, labelAgentsInstructions, SourceTurn, block))
	}
	if block, ok := taggedBlock(turn, tagEnvironmentOpen, tagEnvironmentClose); ok {
		items = append(items, NewItem(KindCodexEnvironment, labelEnvironment, SourceTurn, block))
	}
	return items
}

// An unclosed block runs to the end of the turn.
func instructionsBlock(turn string) (string, bool) {
	start := strings.Index(turn, codexInstructionsHeading)
	if start < 0 {
		start = strings.Index(turn, tagInstructionsOpen)
	}
	if start < 0 {
		return "", false
	}
	end := strings.Index(turn[start:], tagInstructionsClose)
	if end < 0 {
		return turn[start:], true
	}
	return turn[start : start+end+len(tagInstructionsClose)], true
}

// taggedBlock returns the first open…close span of text, tags included.
func taggedBlock(text, open, close string) (string, bool) {
	start := strings.Index(text, open)
	if start < 0 {
		return "", false
	}
	end := strings.Index(text[start:], close)
	if end < 0 {
		return "", false
	}
	return text[start : start+end+len(close)], true
}
