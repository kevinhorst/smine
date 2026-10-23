package acdsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Built by concatenation so the literal never appears in committed source.
const ProjectionMarker = "[ACDSL-" + "PROJECTION]"

// projectionSyntax describes how one file class renders a projection line.
type projectionSyntax struct {
	linePrefix string
	lineSuffix string
}

// Rules over extensions absent here are gate-only on disk and delivered by the acdsl-context hook.
var projectionSyntaxes = map[string]projectionSyntax{
	".go":   {linePrefix: "// "},
	".html": {linePrefix: "<!-- ", lineSuffix: " -->"},
	".md":   {linePrefix: "<!-- ", lineSuffix: " -->"},
	".py":   {linePrefix: "# "},
	".sh":   {linePrefix: "# "},
	".sql":  {linePrefix: "-- "},
	".toml": {linePrefix: "# "},
	".yaml": {linePrefix: "# "},
	".yml":  {linePrefix: "# "},
}

func syntaxForPath(path string) (projectionSyntax, bool) {
	if syntax, ok := projectionSyntaxes[strings.ToLower(filepath.Ext(path))]; ok {
		return syntax, true
	}
	if strings.EqualFold(filepath.Base(path), "Makefile") {
		hashSyntax := projectionSyntax{linePrefix: "# "}
		return hashSyntax, true
	}
	return projectionSyntax{}, false
}

// An HTML comment above a frontmatter fence would break its parsers; an unclosed fence yields 0.
func insertionIndex(lines []string, syntax projectionSyntax) int {
	if len(lines) == 0 {
		return 0
	}
	if syntax.linePrefix == "# " && strings.HasPrefix(lines[0], "#!") {
		return 1
	}
	if syntax.lineSuffix != "" && lines[0] == "---" {
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" || lines[i] == "..." {
				return i + 1
			}
		}
	}
	return 0
}

// Each line uses the file class's own comment syntax so the projected tree stays parseable.
func ProjectionBlock(governing []Rule, syntax projectionSyntax) []string {
	if len(governing) == 0 {
		return nil
	}
	sorted := make([]Rule, len(governing))
	copy(sorted, governing)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Id < sorted[j].Id })

	header := fmt.Sprintf(
		"%s%s %d rule(s) govern this file — working-copy view, stripped before commit%s",
		syntax.linePrefix,
		ProjectionMarker,
		len(sorted),
		syntax.lineSuffix,
	)
	lines := []string{header}
	for _, rule := range sorted {
		lines = append(lines, fmt.Sprintf("%s- [%s] %s%s", syntax.linePrefix, rule.Id, rule.Why, syntax.lineSuffix))
	}
	return append(lines, "")
}

// Blocks only live at the insertion index; stragglers are left to the staged leak guard.
func stripBlock(lines []string, syntax projectionSyntax, at int) ([]string, bool) {
	if at >= len(lines) || !strings.HasPrefix(lines[at], syntax.linePrefix+ProjectionMarker) {
		return lines, false
	}
	i := at + 1
	for i < len(lines) && strings.HasPrefix(lines[i], syntax.linePrefix+"- [") {
		i++
	}
	if i < len(lines) && lines[i] == "" {
		i++
	}
	return append(append([]string{}, lines[:at]...), lines[i:]...), true
}

// Rules over other file types are gate-only on disk; project -plan and -context still print them.
func Projectable(path string) bool {
	_, ok := syntaxForPath(path)
	return ok
}

func ProjectFile(root, path string, governing []Rule) (bool, error) {
	syntax, ok := syntaxForPath(path)
	if !ok {
		return false, nil
	}
	fullPath := filepath.Join(root, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false, fmt.Errorf("ProjectFile: %w", err)
	}
	raw, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("ProjectFile: %w", err)
	}

	// The index is reused after the strip: stripBlock only removes lines from it on.
	lines := strings.Split(string(raw), "\n")
	at := insertionIndex(lines, syntax)
	stripped, _ := stripBlock(lines, syntax, at)
	block := ProjectionBlock(governing, syntax)
	projected := append(append(append([]string{}, stripped[:at]...), block...), stripped[at:]...)

	next := strings.Join(projected, "\n")
	if next == string(raw) {
		return false, nil
	}
	if err := os.WriteFile(fullPath, []byte(next), info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("ProjectFile: %w", err)
	}
	return true, nil
}

func StripAll(root string, universe []string) (int, error) {
	stripped := 0
	for _, path := range universe {
		changed, err := ProjectFile(root, path, nil)
		if err != nil {
			return stripped, fmt.Errorf("StripAll: %w", err)
		}
		if changed {
			stripped++
		}
	}
	return stripped, nil
}

func StripProjection(path string, content []byte) []byte {
	syntax, ok := syntaxForPath(path)
	if !ok {
		return content
	}
	lines := strings.Split(string(content), "\n")
	stripped, isStripped := stripBlock(lines, syntax, insertionIndex(lines, syntax))
	if !isStripped {
		return content
	}
	return []byte(strings.Join(stripped, "\n"))
}

// Guarded apart from the marker: models copied rule lines without it in the A/B probes.
const projectionRuleLineMarker = "- [ACDSL" + "-"

type projectionShape struct {
	linePrefix string
	literal    string
}

func projectionShapes() []projectionShape {
	prefixes := map[string]bool{}
	for _, syntax := range projectionSyntaxes {
		prefixes[syntax.linePrefix] = true
	}
	shapes := make([]projectionShape, 0)
	for prefix := range prefixes {
		header := projectionShape{linePrefix: prefix, literal: prefix + ProjectionMarker}
		ruleLine := projectionShape{linePrefix: prefix, literal: prefix + projectionRuleLineMarker}
		shapes = append(shapes, header, ruleLine)
	}
	sort.Slice(shapes, func(i, j int) bool { return shapes[i].literal < shapes[j].literal })
	return shapes
}

// A file quoting another syntax's shape is a reference, not a leak.
func ValidateStagedClean(ctx context.Context, root string) ([]string, error) {
	var violations []string
	seen := map[string]bool{}
	for _, shape := range projectionShapes() {
		output, err := shellGitGrepCached(ctx, root, shape.literal)
		if err != nil {
			if isExitOne(err) {
				continue // no matches for this pattern
			}
			return nil, fmt.Errorf("ValidateStagedClean: %w", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			if line == "" {
				continue
			}
			path, lineNo, _ := splitGrepLine(line)
			syntax, ok := syntaxForPath(path)
			if !ok || syntax.linePrefix != shape.linePrefix {
				// Scan space = files whose own syntax produces this shape.
				continue
			}
			key := path + ":" + lineNo
			if seen[key] {
				continue
			}
			seen[key] = true
			violations = append(violations, fmt.Sprintf("%s:%s: projection content staged — run: go run ./cmd/acdsl project -strip, then restage", path, lineNo))
		}
	}
	return violations, nil
}

// These trees legitimately quote projection lines; a match there is a reference, never a leak.
var committedScanExcludes = []string{"sessions/", "plans/", "docs/", "examples/", "evals/"}

// Loud on every audit until the offending commit is stripped and amended or reverted.
func ValidateCommittedClean(ctx context.Context, root string) ([]string, error) {
	var violations []string
	seen := map[string]bool{}
	for _, shape := range projectionShapes() {
		output, err := shellGitGrepTree(ctx, root, shape.literal, "HEAD", committedScanExcludes)
		if err != nil {
			if isExitOne(err) {
				continue // no matches for this pattern
			}
			return nil, fmt.Errorf("ValidateCommittedClean: %w", err)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
			if line == "" {
				continue
			}
			path, lineNo, _ := splitGrepLine(strings.TrimPrefix(line, "HEAD:"))
			syntax, ok := syntaxForPath(path)
			if !ok || syntax.linePrefix != shape.linePrefix {
				continue // scan space = files whose own syntax produces this shape
			}
			key := path + ":" + lineNo
			if seen[key] {
				continue
			}
			seen[key] = true
			violations = append(violations, fmt.Sprintf("%s:%s: projection content committed — strip and amend or revert the offending commit (see docs/acdsl.html)", path, lineNo))
		}
	}
	return violations, nil
}

func splitGrepLine(line string) (string, string, string) {
	parts := strings.SplitN(line, ":", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}
