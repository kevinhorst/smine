// cmd/skillcontext/protocol.go — the protocol facts of this hook; the one file the literal gate exempts.

package main

const (
	eventPostToolUse  = "PostToolUse"
	eventPromptSubmit = "UserPromptSubmit"
	toolSkill         = "Skill"
	usageExitCode     = 2
)

// Process-env overrides of the context index and the lane map: the eval arm points a session at its own.
const (
	indexKey = "SKILL_CONTEXT_INDEX"
	mapKey   = "SKILL_CONTEXT_MAP"
)

// The deployed skill's files and the SKILL.md frontmatter keys the hook parses.
const (
	declarationKey   = "acdsl-context:"
	frontmatterFence = "---"
	mapFileName      = "context-map.json"
	missingVersion   = "?"
	skillFileName    = "SKILL.md"
	versionKey       = "version:"
)

// noContextDirFormat names the setting a fresh install has to set; nothing is injected.
const noContextDirFormat = "smine: no context dir configured — set context.context_dir (Hooks tab or `smine-rules settings set context.context_dir <dir>`)"
