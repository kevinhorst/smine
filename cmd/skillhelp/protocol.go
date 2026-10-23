package main

import (
	"regexp"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// The protocol facts of this hook: the deployed frontmatter line, Claude Code's verdict word, the flag package's exit code; the one file the literal gate exempts.
const (
	answeredByLine = skillgraph.FieldAnsweredBy + ": " + skillgraph.AnsweredByHook
	decisionBlock  = "block"
	usageExitCode  = 2
)

// invokePattern is the `/<leaf>` prompt grammar.
var invokePattern = regexp.MustCompile(`^\s*/([A-Za-z0-9_-]+)(?:\s|$)`)
