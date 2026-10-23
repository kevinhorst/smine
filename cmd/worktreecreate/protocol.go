package main

// The protocol facts of this hook: git's own text, the Claude Code event and source names, the repo layout, the flag names its group commands pass; the one file the literal gate exempts.
const (
	acdslSourceDir    = "cmd/acdsl"
	eventSessionStart = "SessionStart"
	flagBase          = "base"
	flagClaudeHome    = "claude-home"
	flagStore         = "store"
	indexLockText     = "index.lock"
	sourceStartup     = "startup"
)
