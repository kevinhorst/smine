package contextdocs

import (
	"strings"

	"github.com/kevinhorst/smine/internal/reach"
)

// reachBulletPrefix identifies the Reach bullet inside an entry block.
const reachBulletPrefix = "* Reach: "

// Non-entry content — headings, prose, fences, the Tombstones table — passes through verbatim.
func FilterRulesFile(content, target string, aspects []RuleAspect, langs []string) string {
	selected := map[string]bool{}
	for _, lang := range langs {
		selected[lang] = true
	}
	scopeLang := map[string]string{}
	for _, aspect := range aspects {
		if aspect.Class == AspectClassScope && aspect.Lang != "" {
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

		blockEnd := entryContentEnd(lines, index)
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

// No named repo admits every entry: deployed trees and unstamped indexes are reach-filtered at sync.
func IsReachable(reachValue string, self string) bool {
	if self == "" {
		return true
	}

	value := strings.Join(strings.Fields(reachValue), "")
	return value == "" || reach.DeploysTo(value, self)
}

// An absent Reach bullet is the prose default, global.
func entryBlockReach(block []string) string {
	for _, line := range block {
		if strings.HasPrefix(line, reachBulletPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, reachBulletPrefix))
		}
	}
	return reach.Global
}
