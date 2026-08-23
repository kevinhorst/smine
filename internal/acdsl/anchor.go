package acdsl

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrAnchorEmpty marks an anchor that resolved to nothing. For doctrine this
// stays a hard authoring error; for task entries Check converts it into a
// diagnostic — the planned artifact not existing IS the contract violation.
var ErrAnchorEmpty = errors.New("anchor matched no files")

// ResolveAnchor filters tracked repo-relative paths by the rule's anchor
// regexps. Resolving to nothing is an authoring error per the concept:
// an anchor must return something.
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
