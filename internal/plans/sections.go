// Package plans holds the plan section catalog, the machine copy of the plan section rule.
package plans

import (
	"path/filepath"
	"regexp"
	"strings"
)

// AgentOnlyMarker is the heading suffix of an agent-tier section.
const AgentOnlyMarker = "[AGENT-ONLY]"

// A plan created earlier keeps the order it was written in.
const OrderCutoff = "2026-10-04"

// Sections is the plan section catalog in canonical order.
var Sections = []string{
	"TLDR",
	"Scope",
	"Decisions",
	"Assumptions",
	"Drivers",
	"Open questions",
	"Changes",
	"Contracts",
	"Tests",
	"Verification",
	"Current state",
	"Target state",
	"Baseline (verified)",
	"Exemplar & reuse",
	"Test runbook",
	"Changelog",
}

// agentTier holds the catalog sections whose headings carry AgentOnlyMarker.
var agentTier = map[string]bool{
	"Baseline (verified)": true,
	"Current state":       true,
	"Exemplar & reuse":    true,
	"Target state":        true,
	"Test runbook":        true,
}

// The plan dir name carries its creation date.
var planDirDate = regexp.MustCompile(`(?:^|/)(\d{4}-\d{2}-\d{2})-[^/]+-(?:wip|done)-[^/]+/design/[^/]+\.md$`)

// IsAgentTier reports whether the catalog section is agent-tier.
func IsAgentTier(name string) bool {
	return agentTier[name]
}

// A path in no plan dir is a session draft and is checked.
func IsOrderChecked(path string) bool {
	match := planDirDate.FindStringSubmatch(filepath.ToSlash(path))
	if match == nil {
		return true
	}
	return match[1] >= OrderCutoff
}

// SectionIndex maps each catalog section to its canonical position.
func SectionIndex() map[string]int {
	index := make(map[string]int, len(Sections))
	for position, name := range Sections {
		index[name] = position
	}
	return index
}

// A marker without the separating space is part of the name.
func SplitHeading(text string) (string, bool) {
	name, marked := strings.CutSuffix(strings.TrimSpace(text), " "+AgentOnlyMarker)
	return strings.TrimSpace(name), marked
}
