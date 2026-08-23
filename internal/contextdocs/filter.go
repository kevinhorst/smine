package contextdocs

import (
	"strings"

	"github.com/kevinhorst/smine/internal/reach"
)

// reachBulletPrefix identifies the Reach bullet inside an entry block.
const reachBulletPrefix = "* Reach: "

// FilterRulesFile returns content with entries removed that must not deploy
// to target: entries whose Reach does not cover target (reach.DeploysTo —
// global always does, a name list only when it names target; smine never
// matches a sync target), and entries whose scope is bound (via aspects) to
// a language outside langs. Everything that is not an entry block — headings, prose, fenced
// examples, the Tombstones table — passes through verbatim.
func FilterRulesFile(content, target string, aspects []RuleAspect, langs []string) string {
	selected := map[string]bool{}
	for _, lang := range langs {
		selected[lang] = true
	}
	scopeLang := map[string]string{}
	for _, aspect := range aspects {
		if aspect.Class == "scope" && aspect.Lang != "" {
			scopeLang[aspect.Name] = aspect.Lang
		}
	}

	lines := strings.Split(content, "\n")
	var kept []string
	isInFence := false
	isInTombstones := false

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "```") {
			isInFence = !isInFence
			kept = append(kept, line)
			continue
		}
		if isInFence {
			kept = append(kept, line)
			continue
		}
		if strings.HasPrefix(line, "## ") {
			isInTombstones = strings.TrimSpace(line) == "## Tombstones"
			kept = append(kept, line)
			continue
		}
		if isInTombstones {
			kept = append(kept, line)
			continue
		}

		match := ruleEntryPattern.FindStringSubmatch(line)
		if match == nil {
			kept = append(kept, line)
			continue
		}

		blockEnd := entryBlockEnd(lines, index)
		entryReach := entryBlockReach(lines[index:blockEnd])
		drop := !reach.DeploysTo(entryReach, target)
		if lang, bound := scopeLang[match[2]]; bound && !selected[lang] {
			drop = true
		}
		if !drop {
			kept = append(kept, lines[index:blockEnd]...)
		} else if len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" &&
			blockEnd < len(lines) && strings.TrimSpace(lines[blockEnd]) == "" {
			// dropping the block would leave two adjacent blanks — swallow one
			kept = kept[:len(kept)-1]
		}
		index = blockEnd - 1
	}
	return strings.Join(kept, "\n")
}

// entryBlockEnd returns the exclusive end index of the entry block opening
// at start: the headline plus its bullet lines and interior blanks, ending
// before the next non-bullet content line, heading, or fence.
func entryBlockEnd(lines []string, start int) int {
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.TrimSpace(line) == "" {
			// a blank ends the block unless a bullet continues right after
			if index+1 < len(lines) && strings.HasPrefix(lines[index+1], "* ") {
				continue
			}
			return index
		}
		if !strings.HasPrefix(line, "* ") {
			return index
		}
	}
	return len(lines)
}

// entryBlockReach extracts the block's Reach bullet value; absent = the
// prose default, global.
func entryBlockReach(block []string) string {
	for _, line := range block {
		if strings.HasPrefix(line, reachBulletPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, reachBulletPrefix))
		}
	}
	return reach.Global
}
