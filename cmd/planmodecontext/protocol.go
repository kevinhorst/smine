// cmd/planmodecontext/protocol.go — the protocol facts of this hook; the one file the literal gate exempts.

package main

const (
	eventPostToolUse   = "PostToolUse"
	failureExitCode    = 1
	permissionModePlan = "plan"
	toolEnterPlanMode  = "EnterPlanMode"
)

// fallbackContextDir is the repo layout a context index names when the configured dir holds none.
const fallbackContextDir = "context"

// noContextDirFormat names the setting a fresh install has to set; nothing is injected.
const noContextDirFormat = "smine: no context dir configured — set context.context_dir (Hooks tab or `smine-rules settings set context.context_dir <dir>`)"
