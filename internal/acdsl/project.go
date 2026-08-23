package acdsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectionMarker opens every projection block in a working copy. Built by
// concatenation so the literal never appears in committed source — the
// staged leak guard (ValidateStagedClean) greps the index for its
// syntax-wrapped shapes but reports only matches in files whose own
// projection syntax produces that shape, so a doc quoting another syntax's
// form in prose is not a leak.
const ProjectionMarker = "[ACDSL-" + "PROJECTION]"

// projectionSyntax describes how one file class renders a projection line.
type projectionSyntax struct {
	linePrefix string
	lineSuffix string
}

// projectionSyntaxes maps a lowercased extension to its comment syntax.
// Extensions absent here (.json, .acdsl, binaries) have no comment syntax
// the projector writes — their rules are gate-only on disk and delivered at
// read time by the acdsl-context hook instead.
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

// syntaxForPath resolves a path's projection syntax; ok=false means the file
// class carries no comment syntax the projector writes.
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

// insertionIndex returns the line index where a projection block belongs:
// after a #!-shebang (hash syntaxes), after a closed leading YAML
// frontmatter fence (HTML-wrapped syntaxes — an HTML comment above the
// fence would break frontmatter parsers), else 0. An unclosed fence yields
// 0 — never guess past malformed preamble.
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

// ProjectionBlock renders the working-copy block for the rules governing one
// file: marker line, one comment line per rule, one blank separator line —
// each line wrapped in the file class's own comment syntax, so the projected
// tree stays parseable. Empty input yields nil — an ungoverned file has no
// projection.
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

// stripBlock removes a projection block at line index at, returning the full
// remaining lines and whether a block was present. Blocks only ever live at
// their file's insertion index (ProjectFile puts them there); stragglers
// elsewhere are caught by the staged leak guard, never silently rewritten.
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

// Projectable reports whether a path can carry an on-disk projection block —
// whether its file class has a registered comment syntax. Rules over other
// file types are gate-only on disk; project -plan and project -context still
// resolve and print them.
func Projectable(path string) bool {
	_, ok := syntaxForPath(path)
	return ok
}

// ProjectFile syncs one file's on-disk projection to the current rules:
// inserts, refreshes, or removes the block so the working copy always shows
// the governing doctrine directly above the content. Idempotent — returns
// whether the file changed. Non-projectable files are never touched.
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

	// The insertion index is computed on the raw lines and reused after the
	// strip: stripBlock only removes lines from that index on, so the
	// preamble before it is invariant.
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

// StripAll removes projection blocks from every universe file — the
// pre-commit sweep. Returns how many files were stripped.
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

// projectionRuleLineMarker is the id-opening fragment of a per-rule line
// inside a projection block. Guarded separately from the marker: a model
// copying the rule lines without the marker line (observed in the A/B
// probes) must not slip the index either. Concatenated so committed source
// stays grep-clean.
const projectionRuleLineMarker = "- [ACDSL" + "-"

// projectionShape is one staged-leak grep target: the syntax-wrapped literal
// and the linePrefix identifying which file class legitimately produces it.
type projectionShape struct {
	linePrefix string
	literal    string
}

// projectionShapes builds the header and rule-line literals for every
// registered comment syntax.
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

// ValidateStagedClean reports every staged (index) line carrying projection
// content — a syntax-wrapped marker or rule line whose comment shape matches
// the file's own projection syntax — a working-copy projection about to
// become repo history. A file quoting another syntax's shape (a doc quoting
// the Go form in a fence) is a reference, not a leak. The working tree
// legitimately carries projections; the index never may.
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

func splitGrepLine(line string) (string, string, string) {
	parts := strings.SplitN(line, ":", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}
