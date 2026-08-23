package skills

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EntryClasses is the closed class-tag set: the context enforcement tags plus
// step (sequential instruction) and payload (template/example, never scored).
var EntryClasses = map[string]bool{
	"gate":    true,
	"hook":    true,
	"lint":    true,
	"manual":  true,
	"payload": true,
	"review":  true,
	"step":    true,
}

var (
	// EntryPattern is the skill entry headline: **SKILL-<NAME>-<TOPIC>-NNN** `[class]` — statement.
	EntryPattern       = regexp.MustCompile(`^\*\*SKILL-([A-Z0-9]{2,24})-([A-Z0-9]{2,12})-([0-9]{3})\*\* \x60\[([a-z]+)\]\x60 — (.+)$`)
	entryBulletPattern = regexp.MustCompile(`^\* (Why|Applies): (.+)$`)
	entryPrefixPattern = regexp.MustCompile(`^\*\*SKILL-`)
)

// Entry is one addressable instruction of a skill body — the SKILL.md is the
// only source; nothing is stored. Line/End are 0-based [Line, End) over the
// file's lines and bound the block a renderer strips when the entry is
// disabled: the entry line, its indented bullets, and — for a payload — the
// fenced block that follows.
type Entry struct {
	Id        string `json:"id"`
	Applies   string `json:"applies,omitempty"`
	Class     string `json:"class"`
	End       int    `json:"-"`
	Line      int    `json:"line"`
	Number    int    `json:"number"`
	Payload   bool   `json:"payload"`
	Skill     string `json:"skill"`
	Statement string `json:"statement"`
	Topic     string `json:"topic"`
	Why       string `json:"why,omitempty"`
}

// ParseEntries walks the body of a SKILL.md (frontmatter and fenced content
// skipped; a fence closes only on its own backtick run, so a four-backtick
// payload may carry three-backtick fences inside) and returns its entries in
// file order plus one violation per malformed headline, unknown class, or
// payload without a following fence.
func ParseEntries(lines []string) ([]Entry, []string) {
	var entries []Entry
	var violations []string
	openFence := ""
	inFrontmatter := isFrontmatterStart(lines)
	for index, line := range lines {
		if inFrontmatter {
			inFrontmatter = !isFrontmatterEnd(index, line)
			continue
		}
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			continue
		}
		if openFence != "" {
			continue
		}
		entry, entryViolations, isEntry := parseEntryAt(lines, index)
		violations = append(violations, entryViolations...)
		if isEntry {
			entries = append(entries, entry)
		}
	}
	return entries, violations
}

// ResolveDisable maps disable tokens (exact IDs or trailing-* globs, the
// context: grammar) to the matched entry IDs; a token matching nothing is an
// error — a silent miss would ship the full skill under a variant name.
func ResolveDisable(entries []Entry, tokens []string) (map[string]bool, error) {
	disabled := map[string]bool{}
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		matched := false
		for _, entry := range entries {
			if !disableTokenMatches(entry.Id, token) {
				continue
			}
			disabled[entry.Id] = true
			matched = true
		}
		if !matched {
			return nil, fmt.Errorf("ResolveDisable: %q matches no entry", token)
		}
	}
	return disabled, nil
}

// SkillIdName is the NAME segment for a leaf dir: upper-cased, hyphens removed.
func SkillIdName(leaf string) string {
	return strings.ToUpper(strings.ReplaceAll(leaf, "-", ""))
}

// disableTokenMatches reports whether an entry id is named by a disable token —
// an exact id or a trailing-* prefix glob.
func disableTokenMatches(id, token string) bool {
	if id == token {
		return true
	}
	if !strings.HasSuffix(token, "*") {
		return false
	}
	return strings.HasPrefix(id, strings.TrimSuffix(token, "*"))
}

// entryBlockEnd returns the exclusive end of the entry block opening at start:
// the headline plus its bullet lines and interior blanks, ending before the
// next non-bullet content line, heading, or fence — the contextdocs rule.
func entryBlockEnd(lines []string, start int) int {
	for index := start + 1; index < len(lines); index++ {
		if isBulletContinuation(lines, index) {
			continue
		}
		return index
	}
	return len(lines)
}

// entryBullets fills Why/Applies from the tagged bullets inside the block.
func entryBullets(entry *Entry, lines []string) {
	for _, bullet := range lines[entry.Line+1 : entry.End] {
		match := entryBulletPattern.FindStringSubmatch(bullet)
		if match == nil {
			continue
		}
		switch match[1] {
		case "Why":
			entry.Why = strings.TrimSpace(match[2])
		case "Applies":
			entry.Applies = strings.TrimSpace(match[2])
		}
	}
}

// fenceRun returns the leading backtick run of a fence line ("```", "````"),
// or "" when the line does not open or close a fence.
func fenceRun(line string) string {
	if !strings.HasPrefix(line, "```") {
		return ""
	}
	end := 0
	for end < len(line) && line[end] == '`' {
		end++
	}
	return line[:end]
}

// isBulletContinuation reports whether the line at index still belongs to an
// entry block: a top-level bullet, or a blank line directly followed by one.
func isBulletContinuation(lines []string, index int) bool {
	line := lines[index]
	if strings.HasPrefix(line, "* ") {
		return true
	}
	if strings.TrimSpace(line) != "" {
		return false
	}
	hasNext := index+1 < len(lines)
	return hasNext && strings.HasPrefix(lines[index+1], "* ")
}

func isFrontmatterEnd(index int, line string) bool {
	return index > 0 && strings.TrimSpace(line) == "---"
}

func isFrontmatterStart(lines []string) bool {
	return len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
}

// parseEntryAt parses the headline at index into an Entry with its block
// extent and bullets; isEntry is false for non-headline lines. Violations
// cover a malformed **SKILL- line, an unknown class, or a payload without a
// following fence.
func parseEntryAt(lines []string, index int) (Entry, []string, bool) {
	line := lines[index]
	match := EntryPattern.FindStringSubmatch(line)
	if match == nil {
		if entryPrefixPattern.MatchString(line) {
			return Entry{}, []string{fmt.Sprintf("%d: malformed entry line: %s", index+1, line)}, false
		}
		return Entry{}, nil, false
	}
	number, _ := strconv.Atoi(match[3])
	entry := Entry{
		Id:        fmt.Sprintf("SKILL-%s-%s-%s", match[1], match[2], match[3]),
		Class:     match[4],
		End:       entryBlockEnd(lines, index),
		Line:      index,
		Number:    number,
		Payload:   match[4] == "payload",
		Skill:     match[1],
		Statement: strings.TrimSpace(match[5]),
		Topic:     match[2],
	}
	var violations []string
	if !EntryClasses[entry.Class] {
		violations = append(violations, fmt.Sprintf("%d: unknown class %q", index+1, entry.Class))
	}
	entryBullets(&entry, lines)
	if entry.Payload {
		fenceEnd, hasFence := payloadFenceEnd(lines, entry.End)
		if hasFence {
			entry.End = fenceEnd
		} else {
			violations = append(violations, fmt.Sprintf("%d: payload entry %s is not followed by a fenced block", index+1, entry.Id))
		}
	}
	return entry, violations, true
}

// payloadFenceEnd finds the fenced block that belongs to a payload entry: it
// must open at from or after at most one blank line; the returned end is the
// index after the closing fence — the line equal to the opening backtick run.
func payloadFenceEnd(lines []string, from int) (int, bool) {
	open := from
	if open < len(lines) && strings.TrimSpace(lines[open]) == "" {
		open++
	}
	if open >= len(lines) {
		return 0, false
	}
	marker := fenceRun(lines[open])
	if marker == "" {
		return 0, false
	}
	for index := open + 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == marker {
			return index + 1, true
		}
	}
	return 0, false
}

// toggleFence returns the fence state after a fence line: opens when none is
// open, closes only on the run that opened it, else stays open (an inner
// three-backtick fence inside a four-backtick block is content).
func toggleFence(openFence, run, line string) string {
	if openFence == "" {
		return run
	}
	if strings.TrimSpace(line) == openFence {
		return ""
	}
	return openFence
}
