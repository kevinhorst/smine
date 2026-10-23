package acdsl

import (
	"fmt"
	"sort"
	"strings"
)

func BuildReport(registry map[string]RegistryEntry, diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}
	byRule := map[string][]Diagnostic{}
	for _, diagnostic := range diagnostics {
		byRule[diagnostic.RuleId] = append(byRule[diagnostic.RuleId], diagnostic)
	}
	ids := make([]string, 0, len(byRule))
	for id := range byRule {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var report strings.Builder
	report.WriteString("## ACDSL violations report\n")
	files := map[string]bool{}
	for _, id := range ids {
		group := byRule[id]
		first := group[0]
		fmt.Fprintf(&report, "\n### %s — %s\n\n%s\n\n", id, first.Verifier, first.Why)
		for _, diagnostic := range group {
			fmt.Fprintf(&report, "- %s\n", diagnostic.Message)
			files[messageFile(diagnostic.Message)] = true
		}
		if entry, ok := registry[first.Verifier]; ok && entry.Description != "" {
			fmt.Fprintf(&report, "\nfix: %s\n", entry.Description)
		}
	}
	fmt.Fprintf(&report, "\n---\n%d violation(s) · %d rule(s) · %d file(s)\n", len(diagnostics), len(ids), len(files))
	return report.String()
}

// A message without a colon counts as its own bucket.
func messageFile(message string) string {
	path, _, _ := strings.Cut(message, ":")
	return path
}
