package main

import "regexp"

// The protocol facts of this hook: Claude Code's event, verdict and tool names, its env names, the rename-instruction grammar; the one file the literal gate exempts.
const (
	attendedValue           = "1"
	decisionBlock           = "block"
	entryPointDesktop       = "claude-desktop"
	envEntryPoint           = "CLAUDE_CODE_ENTRYPOINT"
	envHostSessionId        = "CLAUDE_CODE_HOST_SESSION_ID"
	envSessionAttended      = "CLAUDE_CODE_SESSION_ATTENDED"
	eventPostToolUse        = "PostToolUse"
	eventPreToolUse         = "PreToolUse"
	eventSessionStart       = "SessionStart"
	eventStop               = "Stop"
	permissionAllow         = "allow"
	renameInstructionFormat = "Session title: call mcp__ccd_session_mgmt__set_session_title with session_id \"self\" and title %s."
	renameSelf              = "self"
	sourceCompact           = "compact"
	toolBash                = "Bash"
)

// titleSetTokenGroup is the capture group of the title-set trigger pattern that holds the token.
const titleSetTokenGroup = 3

var sessionIdPattern = regexp.MustCompile(`^[0-9a-f-]+$`)
