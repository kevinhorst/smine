package main

import "regexp"

// Events the hook answers: the first two export the session's values, the last two check an invoked skill.
const (
	eventSessionStart        = "SessionStart"
	eventCwdChanged          = "CwdChanged"
	eventUserPromptExpansion = "UserPromptExpansion"
	eventPreToolUse          = "PreToolUse"
)

const (
	decisionBlock         = "block"
	decisionDeny          = "deny"
	envFileMode           = 0o600
	envFileVariable       = "CLAUDE_ENV_FILE"
	expansionSlashCommand = "slash_command"
	repoQuery             = "?repo="
	skillsDirName         = "skills"
)

// The refusal's and the context's texts, their slots in reading order.
const (
	contextUnsetFormat    = "skillenv: %s not set — optional, its steps are left out"
	contextValueFormat    = "skillenv: %s = %s (%s)"
	exportHintFormat      = "Export it before starting Claude Code: export %s=…"
	linkHintFormat        = "Set it here, then start a new session: %s"
	noteFormat            = "Note: %s"
	registeredWhereFormat = "for %s"
	shellHintFormat       = "Exported in your shell already? Send #disable inject:env, then invoke /%s again."
	skillPageFormat       = "http://127.0.0.1:%d/scripts/skills/repo/%s"
	unregisteredWhere     = "(this repo is not registered)"
	unsetFormat           = "%s is not set %s — %s, e.g. %s"
)

// leafPattern is a plain skill name; a plugin's colon or a path never names a deployed skill dir.
var leafPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
