package contextdocs

import (
	"slices"
	"strings"
)

// bodyFence opens and closes an example block in a rendered body.
const bodyFence = "```"

// appendTagged adds one "* Name: value" line when the value is set.
func appendTagged(lines []string, name string, value string) []string {
	if value == "" {
		return lines
	}
	return append(lines, "* "+name+": "+value)
}

// matchesAnyGlob reports whether the id matches one of the globs.
func matchesAnyGlob(globs []string, id string) bool {
	for _, glob := range globs {
		if TokenMatches(id, glob) {
			return true
		}
	}
	return false
}

// renderBody emits headline, blank, tagged fields, bullets, notes, fenced examples and a closing blank.
func renderBody(entry *RuleEntry) []string {
	content := entry.Content
	headline := "**" + entry.Id + "**"
	if entry.Enforcement != "" {
		headline += " `[" + entry.Enforcement + "]`"
	}
	headline += " — " + content.Statement

	lines := []string{headline, ""}
	lines = appendTagged(lines, "Why", content.Why)
	lines = appendTagged(lines, "Applies", content.Applies)
	lines = appendTagged(lines, "Evidence", content.Evidence)
	lines = appendTagged(lines, "Location", content.Location)
	for _, bullet := range content.Bullets {
		lines = append(lines, "* "+bullet)
	}
	if len(content.Notes) > 0 {
		lines = append(lines, "")
		lines = append(lines, content.Notes...)
	}
	for _, example := range content.Examples {
		lines = append(lines, "", bodyFence+example.Lang, example.Code, bodyFence)
	}
	return append(lines, "")
}

// RenderBodies renders entries as the context docs author them, blank-line separated with no trailing newline — the text the skill-context hook injects.
func RenderBodies(entries []RuleEntry) string {
	parts := make([]string, 0, len(entries))
	for index := range entries {
		parts = append(parts, strings.Join(renderBody(&entries[index]), "\n"))
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n")
}

// ResolveGlobs returns the selected entries unique by id (first occurrence decides), sorted by id and reach-filtered when file is a stamped source index.
func ResolveGlobs(file *ContextFile, globs []string) []RuleEntry {
	selected := make([]RuleEntry, 0)
	seen := map[string]bool{}
	for _, entry := range file.Entries {
		if seen[entry.Id] || !matchesAnyGlob(globs, entry.Id) {
			continue
		}

		seen[entry.Id] = true
		if !IsReachable(entry.Reach, file.Self) {
			continue
		}

		selected = append(selected, entry)
	}

	slices.SortFunc(selected, func(left, right RuleEntry) int { return strings.Compare(left.Id, right.Id) })
	return selected
}
