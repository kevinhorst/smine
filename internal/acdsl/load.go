package acdsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// declarationPathspecs lists the files scanned for rule markers: standalone
// .acdsl files (doctrine centrally in acdsl/rules.acdsl, task contracts
// anywhere; renaming a file away from .acdsl retires its entries) plus every
// file class with a registered comment syntax, each scanned for its own
// marker form (declarationMarker).
func declarationPathspecs() []string {
	specs := []string{"*.acdsl", "Makefile"}
	for ext := range projectionSyntaxes {
		specs = append(specs, "*"+ext)
	}
	sort.Strings(specs)
	return specs
}

// Discovery is one rule-discovery pass: the declared rules, the full file
// universe anchors resolve against, and the authoring violations found in
// the markers.
type Discovery struct {
	Rules      []Rule
	Universe   []string
	Violations []string
}

// DiscoverRules loads every rule declaration and the full file universe
// anchors resolve against — all git files, no glob; each rule's anchor
// regexp alone decides which files it governs.
func DiscoverRules(ctx context.Context, root string) (*Discovery, error) {
	universe, err := FileUniverse(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("DiscoverRules: %w", err)
	}
	declarations, err := FileUniverse(ctx, root, declarationPathspecs()...)
	if err != nil {
		return nil, fmt.Errorf("DiscoverRules: %w", err)
	}
	rules, violations, err := LoadRules(root, declarations)
	if err != nil {
		return nil, err
	}
	discovery := &Discovery{
		Rules:      rules,
		Universe:   universe,
		Violations: violations,
	}
	return discovery, nil
}

// LoadRules reads the given declaration files and parses markers. Violations
// are authoring errors reported by the CLI; an unreadable file is a tool error.
func LoadRules(root string, tracked []string) ([]Rule, []string, error) {
	fileLines := make(map[string][]string, len(tracked))
	for _, path := range tracked {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, nil, fmt.Errorf("LoadRules: %w", err)
		}
		fileLines[path] = strings.Split(string(raw), "\n")
	}
	rules, violations := ParseRules(fileLines)
	return rules, violations, nil
}
