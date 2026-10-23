package main

import "regexp"

// The gate's protocol facts: Claude Code's event, tool and decision names, the skill-text markers, the shell words it parses, the process-env name.
const (
	catCommand      = "cat"
	contextIndexKey = "SKILL_CONTEXT_INDEX"
	decisionDeny    = "deny"
	denyTextCount   = 2
	entrySuffix     = ".md"
	eventPreToolUse = "PreToolUse"
	shellWordBreaks = " \t\n"
	toolBash        = "Bash"
	toolRead        = "Read"
	toolSkill       = "Skill"
	usageExitCode   = 2
)

var (
	// dotSegments are the path parts that make a path non-literal.
	dotSegments     = []string{"/./", "/../"}
	manifestPattern = regexp.MustCompile(`(?m)^.*LANE-MANIFEST:(/[^ \t\n\v\f\r]*)`)
	requiredPattern = regexp.MustCompile(`(?m)^.*REQUIRE-SKILL:([a-z0-9-]*)`)
)
