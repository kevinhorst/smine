package skills

import (
	"regexp"
	"strings"
)

var headingPattern = regexp.MustCompile(`^(#{2,3}) `)

// RenderSkill returns the SKILL.md content with every entry named by the
// disable tokens removed (entry block or payload block, per Entry.End) and any
// ##/### section left without content removed with it. Frontmatter, the H1,
// and every other byte stay identical. An empty token list returns the input
// unchanged; a token matching nothing is an error.
func RenderSkill(content string, disable []string) (string, error) {
	lines := strings.Split(content, "\n")
	entries, _ := ParseEntries(lines)
	disabled, err := ResolveDisable(entries, disable)
	if err != nil {
		return "", err
	}
	if len(disabled) == 0 {
		return content, nil
	}
	drop := make([]bool, len(lines))
	for _, entry := range entries {
		if disabled[entry.Id] {
			dropEntry(lines, drop, entry)
		}
	}
	dropEmptySections(lines, drop)
	kept := make([]string, 0, len(lines))
	for index, line := range lines {
		if !drop[index] {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n"), nil
}

// dropEmptySections marks ## / ### headings whose remaining body (up to the
// next heading of the same or a higher level) is blank after the entry drops.
// ### headings are judged first so a ## whose subsections all emptied drops too.
func dropEmptySections(lines []string, drop []bool) {
	dropEmptySectionsAtLevel(lines, drop, 3)
	dropEmptySectionsAtLevel(lines, drop, 2)
}

func dropEmptySectionsAtLevel(lines []string, drop []bool, wantLevel int) {
	for index := range lines {
		if headingLevel(lines[index]) != wantLevel {
			continue
		}
		end := sectionEnd(lines, index, wantLevel)
		if !sectionIsEmpty(lines, drop, index+1, end) {
			continue
		}
		for body := index; body < end; body++ {
			drop[body] = true
		}
	}
}

// dropEntry marks the entry's block and the blank line that separated it from
// what follows.
func dropEntry(lines []string, drop []bool, entry Entry) {
	for index := entry.Line; index < entry.End; index++ {
		drop[index] = true
	}
	hasTrailingBlank := entry.End < len(lines) && strings.TrimSpace(lines[entry.End]) == ""
	if hasTrailingBlank {
		drop[entry.End] = true
	}
}

// headingLevel returns 2 or 3 for a ##/### heading line, else 0.
func headingLevel(line string) int {
	match := headingPattern.FindStringSubmatch(line)
	if match == nil {
		return 0
	}
	return len(match[1])
}

// sectionEnd returns the index of the next heading of the same or a higher
// level after start, or len(lines).
func sectionEnd(lines []string, start, level int) int {
	for index := start + 1; index < len(lines); index++ {
		nextLevel := headingLevel(lines[index])
		isBoundary := nextLevel > 0 && nextLevel <= level
		if isBoundary {
			return index
		}
	}
	return len(lines)
}

// sectionIsEmpty reports whether every kept line in [from, to) is blank.
func sectionIsEmpty(lines []string, drop []bool, from, to int) bool {
	for index := from; index < to; index++ {
		if drop[index] {
			continue
		}
		if strings.TrimSpace(lines[index]) != "" {
			return false
		}
	}
	return true
}
