package skillimport

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

const (
	excerptMarker = "⟦here⟧"
	excerptRunes  = 60
	listSeparator = "\x1f"
)

// provedEntryFields compare whitespace-normalized: the renderer's greedy wrap collapses interior runs.
var provedEntryFields = []entryField{
	{name: "Id", project: func(entry *Entry) string { return entry.Id }},
	{name: "Class", project: func(entry *Entry) string { return entry.Class }},
	{name: "Title", project: func(entry *Entry) string { return normalizeSpace(entry.Title) }},
	{name: "Statement", project: func(entry *Entry) string { return normalizeSpace(entry.Statement) }},
	{name: "Pred", project: func(entry *Entry) string { return normalizeSpace(entry.Pred) }},
	{name: "Post", project: func(entry *Entry) string { return normalizeSpace(entry.Post) }},
	{name: "Why", project: func(entry *Entry) string { return normalizeSpace(entry.Why) }},
	{name: "Applies", project: func(entry *Entry) string { return normalizeSpace(entry.Applies) }},
	{name: "Hook", project: func(entry *Entry) string { return normalizeSpace(entry.Hook) }},
	{name: "Gate", project: func(entry *Entry) string { return normalizeSpace(entry.Gate) }},
	{name: "Schema", project: func(entry *Entry) string { return normalizeSpace(entry.Schema) }},
	{name: "Script", project: func(entry *Entry) string { return normalizeSpace(entry.Script) }},
	{name: "Params", project: func(entry *Entry) string { return joinNormalized(entry.Params) }},
	{name: "Refs", project: func(entry *Entry) string { return strings.Join(entry.Refs, listSeparator) }},
	{name: "Section", project: func(entry *Entry) string { return entry.Section }},
	{name: "SectionKind", project: func(entry *Entry) string { return entry.SectionKind }},
	{name: "SectionPred", project: func(entry *Entry) string { return normalizeSpace(entry.SectionPred) }},
	{name: "SectionReplaces", project: func(entry *Entry) string { return strings.Join(entry.SectionReplaces, listSeparator) }},
	{name: "SectionBetween", project: func(entry *Entry) string { return strings.Join(entry.SectionBetween, listSeparator) }},
}

// entryField is one compared parser field projected to text.
type entryField struct {
	name    string
	project func(entry *Entry) string
}

func compareEntries(newText, oldText string) error {
	newEntries, _ := ParseEntries(newDocument([]byte(newText)).lines)
	oldEntries, _ := ParseEntries(newDocument([]byte(oldText)).lines)
	for position := range min(len(newEntries), len(oldEntries)) {
		newEntry, oldEntry := &newEntries[position], &oldEntries[position]
		name, oldValue, newValue := fieldDifference(newEntry, oldEntry)
		if name == "" {
			continue
		}

		return fmt.Errorf(
			"compareEntries: Entry %s (old line %d, new line %d): %s differs\nold: %q\nnew: %q",
			oldEntry.Id,
			oldEntry.Line+1,
			newEntry.Line+1,
			name,
			oldValue,
			newValue,
		)
	}
	if len(newEntries) != len(oldEntries) {
		return fmt.Errorf("compareEntries: Old text holds %d entries, new text %d", len(oldEntries), len(newEntries))
	}
	return nil
}

func compareStripped(newText, oldText string) error {
	newStripped, newOffsets := strip(newText)
	oldStripped, oldOffsets := strip(oldText)
	if newStripped == oldStripped {
		return nil
	}

	at := firstDifference(newStripped, oldStripped)
	newAt, oldAt := newOffsets[at], oldOffsets[at]
	return fmt.Errorf(
		"compareStripped: Text differs beyond whitespace at old byte %d (line %d), new byte %d (line %d)\nold: %s\nnew: %s",
		oldAt,
		strings.Count(oldText[:oldAt], "\n")+1,
		newAt,
		strings.Count(newText[:newAt], "\n")+1,
		excerpt(oldAt, oldText),
		excerpt(newAt, newText),
	)
}

func excerpt(offset int, text string) string {
	before := []rune(text[:offset])
	after := []rune(text[offset:])
	head := before[max(0, len(before)-excerptRunes):]
	tail := after[:min(len(after), excerptRunes)]
	return fmt.Sprintf("%q", string(head)+excerptMarker+string(tail))
}

func fieldDifference(newEntry, oldEntry *Entry) (string, string, string) {
	for _, field := range provedEntryFields {
		newValue, oldValue := field.project(newEntry), field.project(oldEntry)
		if newValue != oldValue {
			return field.name, oldValue, newValue
		}
	}
	return "", "", ""
}

// firstDifference returns the byte offset of the first differing rune.
func firstDifference(left, right string) int {
	limit := min(len(left), len(right))
	offset := 0
	for offset < limit && left[offset] == right[offset] {
		offset++
	}
	for offset > 0 && !isRuneStartAt(offset, left) {
		offset--
	}
	return offset
}

func isRuneStartAt(at int, text string) bool {
	return at >= len(text) || utf8.RuneStart(text[at])
}

func joinNormalized(values []string) string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, normalizeSpace(value))
	}
	return strings.Join(normalized, listSeparator)
}

// renderedSkillFile renders the model and returns its SKILL.md.
func renderedSkillFile(skill *skillgraph.Skill) (string, error) {
	manifest, err := skillgraph.RenderManifest(skill)
	if err != nil {
		return "", fmt.Errorf("renderedSkillFile: %w", err)
	}
	return string(manifest), nil
}

// rewriteArgs applies only D7's two admitted rewrites, inside the fence-aware "## Args" section.
func rewriteArgs(text string) string {
	lines := strings.Split(text, "\n")
	isArgs := false
	openFence := ""
	for index, line := range lines {
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			continue
		}

		heading, isHeading := strings.CutPrefix(line, "## ")
		switch {
		case openFence != "":
		case isHeading:
			isArgs = strings.TrimSpace(heading) == argsSectionTitle
		case isArgs:
			lines[index] = rewriteArgsLine(line)
		}
	}
	return strings.Join(lines, "\n")
}

func rewriteArgsLine(line string) string {
	if line == legacyArgsAlignment {
		return canonicalArgsAlignment
	}

	match := argBarePattern.FindStringSubmatch(line)
	if match == nil {
		return line
	}
	return "- `" + match[1] + "`: " + match[2]
}

// strip's offsets map each result byte, plus its end, to the source byte offset.
func strip(text string) (string, []int) {
	var builder strings.Builder
	offsets := make([]int, 0, len(text)+1)
	for offset, char := range text {
		if unicode.IsSpace(char) {
			continue
		}

		builder.WriteRune(char)
		for range utf8.RuneLen(char) {
			offsets = append(offsets, offset)
		}
	}
	offsets = append(offsets, len(text))
	return builder.String(), offsets
}

// Prove guards the cutover import of one skill (D4): only whitespace may change.
func Prove(old []byte, skill *skillgraph.Skill) error {
	rendered, err := renderedSkillFile(skill)
	if err != nil {
		return fmt.Errorf("Prove: %w", err)
	}

	rewritten := rewriteArgs(string(old))
	if err := compareStripped(rendered, rewritten); err != nil {
		return fmt.Errorf("Prove: %s: %w", skill.Name(), err)
	}

	if err := compareEntries(rendered, rewritten); err != nil {
		return fmt.Errorf("Prove: %s: %w", skill.Name(), err)
	}
	return nil
}
