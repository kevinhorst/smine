package acdsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Renaming a file away from .acdsl retires its entries.
func declarationPathspecs() []string {
	specs := []string{"*.acdsl", "Makefile"}
	for ext := range projectionSyntaxes {
		specs = append(specs, "*"+ext)
	}
	sort.Strings(specs)
	for _, exclude := range declarationScanExcludes {
		specs = append(specs, ":(exclude)"+exclude)
	}
	return specs
}

// Recorded-artifact trees whose markers are data; narrower than committedScanExcludes on purpose.
var declarationScanExcludes = []string{"sessions/", "evals/", "plans/*/reviews/*"}

// Files inside the recorded-artifact trees that rules gate: curated interpretations and the session records.
var gatedArtifactPathspecs = []string{"evals/*/*/interpretation.md", "sessions/*/*-W*/*.json"}

type Discovery struct {
	Rules      []Rule
	Universe   []string
	Violations []string
}

// No glob: each rule's anchor regexp alone decides which files it governs.
func DiscoverRules(ctx context.Context, root string) (*Discovery, error) {
	// Probe fixtures in recorded-artifact trees deliberately violate the rules anchors resolve onto.
	excludes := make([]string, 0, len(declarationScanExcludes))
	for _, exclude := range declarationScanExcludes {
		excludes = append(excludes, ":(exclude)"+exclude)
	}
	universe, err := FileUniverse(ctx, root, excludes...)
	if err != nil {
		return nil, fmt.Errorf("DiscoverRules: %w", err)
	}
	gatedArtifacts, err := FileUniverse(ctx, root, gatedArtifactPathspecs...)
	if err != nil {
		return nil, fmt.Errorf("DiscoverRules: %w", err)
	}
	universe = append(universe, gatedArtifacts...)
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

// Violations are authoring errors for the CLI; an unreadable file is a tool error.
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
