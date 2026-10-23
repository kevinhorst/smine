package skillgraph

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Chain-end tokens a between clause may name instead of an entry id.
const (
	AnchorEnd   = "end"
	AnchorStart = "start"
)

const (
	anchorItemSeparator  = " + "
	anchorRangeSeparator = ".."
	betweenAnchorCount   = 2
	entryIdPrefix        = "SKILL-"
)

var (
	// anchorIdPattern is one short entry id — the skill prefix implied.
	anchorIdPattern = regexp.MustCompile(`^[A-Z0-9]{2,12}-[0-9]{3}[a-z]?$`)
	// anchorRangePattern is an A..B pair.
	anchorRangePattern = regexp.MustCompile(`^([A-Za-z0-9-]+)\.\.([A-Za-z0-9-]+)$`)
)

func isReplacesItem(item string) bool {
	if anchorIdPattern.MatchString(item) {
		return true
	}

	match := anchorRangePattern.FindStringSubmatch(item)
	if match == nil {
		return false
	}
	return anchorIdPattern.MatchString(match[1]) && anchorIdPattern.MatchString(match[2])
}

func parseBetween(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	match := anchorRangePattern.FindStringSubmatch(text)
	if match == nil {
		return nil, fmt.Errorf("parseBetween: Malformed between %q — <after>..<before>, start and end allowed", text)
	}

	isAfter := match[1] == AnchorStart || anchorIdPattern.MatchString(match[1])
	isBefore := match[2] == AnchorEnd || anchorIdPattern.MatchString(match[2])
	if !isAfter || !isBefore {
		return nil, fmt.Errorf("parseBetween: Malformed between %q — <after>..<before>, start and end allowed", text)
	}

	pair := []string{match[1], match[2]}
	return pair, nil
}

// parseReplaces reports every malformed item, not just the first.
func parseReplaces(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	var items []string
	var problems []error
	for item := range strings.SplitSeq(text, anchorItemSeparator) {
		if !isReplacesItem(item) {
			problems = append(problems, fmt.Errorf("parseReplaces: Malformed replaces item %q — short id or A..B range", item))
			continue
		}

		items = append(items, item)
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return items, nil
}

// resolveAnchorId prefixes one short id; the chain-end tokens pass through.
func resolveAnchorId(id, prefix string) string {
	if id == AnchorStart || id == AnchorEnd {
		return id
	}
	return prefix + id
}

func resolveAnchors(items []string, prefix string) []string {
	if len(items) == 0 {
		return nil
	}

	resolved := make([]string, 0, len(items))
	for _, item := range items {
		first, last, isRange := strings.Cut(item, anchorRangeSeparator)
		full := resolveAnchorId(first, prefix)
		if isRange {
			full += anchorRangeSeparator + resolveAnchorId(last, prefix)
		}
		resolved = append(resolved, full)
	}
	return resolved
}

func SkillIdName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", ""))
}
