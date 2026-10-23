// cmd/readgate/protocol.go — the protocol facts of this hook; the one file the literal gate exempts.

package main

// Claude Code's event, tool and decision names.
const (
	decisionDeny      = "deny"
	eventPostToolUse  = "PostToolUse"
	eventPreToolUse   = "PreToolUse"
	eventPromptSubmit = "UserPromptSubmit"
	eventSessionStart = "SessionStart"
	toolBash          = "Bash"
	toolEdit          = "Edit"
	toolRead          = "Read"
	toolSkill         = "Skill"
	toolWrite         = "Write"
	usageExitCode     = 2
)

// bashMetacharacters split a shell command into words.
const bashMetacharacters = "&;|()<>"

// The <skill>--<slice>--<agent>.md injection file name cmd/skillcontext writes.
const (
	injectionExtension = ".md"
	injectionSeparator = "--"
)

// The lane coverage file and the walk marks of a findings artifact.
const (
	coverageSuffix = ".coverage.json"
	walkMarkSpaced = `"walk_complete": true`
	walkMarkTight  = `"walk_complete":true`
	walkNoFile     = "—"
)

// noContextDirFormat names the setting a fresh install has to set; the gate allows every call.
const noContextDirFormat = "smine: no context dir configured — set context.context_dir (Hooks tab or `smine-rules settings set context.context_dir <dir>`)"
