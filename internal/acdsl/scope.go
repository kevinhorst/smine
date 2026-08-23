package acdsl

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/kevinhorst/smine/internal/reach"
)

// RulesForPlannedPath returns the rules whose anchors would match a path that
// need not exist yet — plan-time resolution: which doctrine will govern a
// file a task is about to create. Pure regexp match, no universe membership.
func RulesForPlannedPath(rules []Rule, path string) ([]Rule, error) {
	var governing []Rule
	for _, rule := range rules {
		if rule.Reach == reach.None {
			continue
		}
		include, err := regexp.Compile(rule.Anchor)
		if err != nil {
			return nil, fmt.Errorf("RulesForPlannedPath: %s: anchor: %w", rule.Id, err)
		}
		if !include.MatchString(path) {
			continue
		}
		if rule.AnchorNot != "" {
			exclude, err := regexp.Compile(rule.AnchorNot)
			if err != nil {
				return nil, fmt.Errorf("RulesForPlannedPath: %s: anchor-not: %w", rule.Id, err)
			}
			if exclude.MatchString(path) {
				continue
			}
		}
		governing = append(governing, rule)
	}
	return governing, nil
}

// Delivered returns the rules whose prose reaches prompt-side channels —
// projection blocks and plan-time resolution. Gate-only rules
// (projected=false) are filtered here and nowhere else: Check never
// consults the flag, so enforcement is identical either way.
func Delivered(rules []Rule) []Rule {
	var delivered []Rule
	for _, rule := range rules {
		if rule.Projected {
			delivered = append(delivered, rule)
		}
	}
	return delivered
}

// RulesForFile returns the rules whose anchor context contains path — the
// stage-2 selection primitive: which doctrine governs this one file. Task
// entries never project: the contract is the plan's context, not the file's.
func RulesForFile(rules []Rule, universe []string, path string) ([]Rule, error) {
	var governing []Rule
	for _, rule := range rules {
		if rule.Lifetime == LifetimeTask || rule.Reach == reach.None {
			continue
		}
		matched, err := ResolveAnchor(universe, rule)
		if err != nil {
			// empty="ok": a transient artifact class with no current files
			// governs nothing — not tool breakage.
			if rule.AllowEmpty && errors.Is(err, ErrAnchorEmpty) {
				continue
			}
			return nil, err
		}
		for _, candidate := range matched {
			if candidate == path {
				governing = append(governing, rule)
				break
			}
		}
	}
	return governing, nil
}
