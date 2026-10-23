package acdsl

import (
	"errors"
	"fmt"
	"regexp"
)

// Doctrine keeps an empty anchor a hard error; Check turns it into a diagnostic for task entries.
var ErrAnchorEmpty = errors.New("anchor matched no files")

func ResolveAnchor(tracked []string, rule Rule) ([]string, error) {
	include, err := regexp.Compile(rule.Anchor)
	if err != nil {
		return nil, fmt.Errorf("ResolveAnchor: %s: anchor: %w", rule.Id, err)
	}
	var exclude *regexp.Regexp
	if rule.AnchorNot != "" {
		if exclude, err = regexp.Compile(rule.AnchorNot); err != nil {
			return nil, fmt.Errorf("ResolveAnchor: %s: anchor-not: %w", rule.Id, err)
		}
	}

	var matched []string
	for _, path := range tracked { // tracked is sorted by the caller
		if !include.MatchString(path) {
			continue
		}
		if exclude != nil && exclude.MatchString(path) {
			continue
		}
		matched = append(matched, path)
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("ResolveAnchor: %s: %w", rule.Id, ErrAnchorEmpty)
	}
	return matched, nil
}
