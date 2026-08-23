package contextdocs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/reach"

	"github.com/kevinhorst/smine/internal/fsx"
)

// Entry kinds — the identity grammar's first segment (polarity lives in the
// statement, never in the kind).
const (
	RuleKindAction = "action"
	RuleKindFact   = "fact"
	RuleKindRule   = "rule"
)

// Entry origins.
const (
	RuleOriginBaseline = "baseline"
	RuleOriginOverlay  = "overlay"
)

// BaselineHeaderMarker identifies a synced baseline file inside a deployed
// target's context folder; an entry file without it is a repo-owned overlay.
const BaselineHeaderMarker = "synced from smine"

// ContextFileName is the generated machine-readable file at the context root.
const ContextFileName = "context.json"

// RuleAspect is one member of the closed taxonomy vocabulary. Class places
// it on the identity grammar's axes: "scope" (the applies-to segment) or
// "topic" (the optional third segment).
type RuleAspect struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Class string `json:"class,omitempty"`
	Lang  string `json:"lang,omitempty"` // rules/<lang>.md guide basename for language scopes; empty = not language-bound
}

// LoadAspects reads the closed aspect vocabulary from the aspects array of
// dir/context.json — the taxonomy has no file of its own.
func LoadAspects(dir string) ([]RuleAspect, error) {
	data, err := os.ReadFile(filepath.Join(dir, ContextFileName))
	if err != nil {
		return nil, fmt.Errorf("LoadAspects: %w", err)
	}
	var parsed struct {
		Aspects []RuleAspect `json:"aspects"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("LoadAspects: %w", err)
	}
	return parsed.Aspects, nil
}

// WriteContextFile regenerates dir/context.json from the markdown entries
// with the given aspects — the aspect editor's write path, byte-identical to
// cmd/rules generate. Temp file + rename so a crashed write never truncates.
func WriteContextFile(dir string, aspects []RuleAspect) error {
	set, err := ParseContext(dir, false)
	if err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	data, err := RenderContextJson(set, aspects)
	if err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	tmpPath := filepath.Join(dir, ContextFileName+".tmp")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	if err := fsx.ReplaceFile(tmpPath, filepath.Join(dir, ContextFileName)); err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	return nil
}

// ruleEnforcements are the valid enforcement tags on ACTION entries.
var ruleEnforcements = map[string]bool{
	"gate":   true,
	"hook":   true,
	"lint":   true,
	"manual": true,
	"review": true,
}

// RuleExample is one fenced example block inside an entry — Lang is the fence
// info string ("go", "sql", "" for a bare fence), Code the verbatim body.
type RuleExample struct {
	Code string `json:"code"`
	Lang string `json:"lang,omitempty"`
}

// RuleContent is the authored body of an entry as typed fields — the delivery
// form hooks render, so the .md is authoring only. Tagged bullets fill the
// named fields; untagged bullets keep authoring order in Bullets (nested
// continuation lines joined); prose lines land in Notes; fences in Examples.
type RuleContent struct {
	Applies   string        `json:"applies,omitempty"`
	Bullets   []string      `json:"bullets,omitempty"`
	Evidence  string        `json:"evidence,omitempty"`
	Examples  []RuleExample `json:"examples,omitempty"`
	Location  string        `json:"location,omitempty"`
	Notes     []string      `json:"notes,omitempty"`
	Statement string        `json:"statement"`
	Why       string        `json:"why,omitempty"`
}

// RuleEntry is one ACTION/FACT entry parsed from a rules markdown file —
// id grammar KIND-SCOPE[-TOPIC]-NNN.
type RuleEntry struct {
	Id          string      `json:"id"`
	Kind        string      `json:"kind"`
	Scope       string      `json:"scope"`
	Topic       string      `json:"topic,omitempty"`
	Number      int         `json:"-"`
	Enforcement string      `json:"enforcement,omitempty"`
	Content     RuleContent `json:"content"`
	Reach       string      `json:"reach"`
	Version     string      `json:"version"`
	Source      string      `json:"source"`
	Origin      string      `json:"origin"`
}

// RuleTombstone is one retired-entry row from a "## Tombstones" table.
type RuleTombstone struct {
	Retired     string `json:"retired"`
	Replacement string `json:"replacement"`
	Date        string `json:"date"`
	Source      string `json:"source"`
}

// RuleGuide is one language guide: a rules file that declares the files it
// governs via a "**Files:** `glob`, `glob`" line. Name is the file name without
// .md, Path is relative to the context dir (rules/go.md), Files are the globs.
// The read-gate hook matches touched paths against Files to require the guide.
type RuleGuide struct {
	Files  []string `json:"files"`
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Source string   `json:"source"`
}

// RuleSet holds every entry, tombstone, and guide parsed from one rules directory.
type RuleSet struct {
	Entries    []RuleEntry     `json:"entries"`
	Tombstones []RuleTombstone `json:"tombstones"`
	Guides     []RuleGuide     `json:"guides"`
}

var (
	ruleEntryPattern = regexp.MustCompile(
		`^\*\*(FACT|ACTION|RULE)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-([0-9]{3})\*\*(?: \x60\[([a-z]+)\]\x60)? — (.+)$`)
	ruleEntryPrefixPattern = regexp.MustCompile(`^\*\*(FACT|ACTION|RULE)-`)
	ruleBulletPattern      = regexp.MustCompile(`^\* (Why|Applies|Evidence|Location|Version|Reach): (.+)$`)
	ruleFilesPattern       = regexp.MustCompile(`^\*\*Files:\*\* (.+)$`)
	ruleFilesGlobPattern   = regexp.MustCompile("`([^`]+)`")
)

// ParseRulesDir parses every .md file in dir into a RuleSet. With
// detectOrigin, a file whose first line carries BaselineHeaderMarker is
// baseline and any other file is a repo-owned overlay (deployed-pack mode);
// without it every file is baseline (source-repo mode). Fenced code blocks
// are skipped, so authoring docs can hold example entries.
func ParseRulesDir(dir string, detectOrigin bool) (RuleSet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return RuleSet{}, fmt.Errorf("ParseRulesDir: %w", err)
	}

	var set RuleSet
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		source := filepath.ToSlash(filepath.Join(dir, entry.Name()))
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return RuleSet{}, fmt.Errorf("ParseRulesDir: %w", err)
		}

		origin := RuleOriginBaseline
		headline, _, _ := strings.Cut(string(content), "\n")
		if detectOrigin && !strings.Contains(headline, BaselineHeaderMarker) {
			origin = RuleOriginOverlay
		}

		if err := parseRulesFile(string(content), source, origin, &set); err != nil {
			return RuleSet{}, fmt.Errorf("ParseRulesDir: %w", err)
		}
	}
	return set, nil
}

// ParseContext parses a context tree: actions/ and rules/ as entry files plus
// facts/ as repo-owned overlay files. Any directory may be absent; an entirely
// missing context dir is an error. detectOrigin is true for deployed targets
// (baseline-header detection) and false for the source repo (all baseline).
func ParseContext(contextDir string, detectOrigin bool) (RuleSet, error) {
	if _, err := os.Stat(contextDir); err != nil {
		return RuleSet{}, fmt.Errorf("ParseContext: %w", err)
	}

	var merged RuleSet
	for _, dir := range []string{"actions", "rules"} {
		entryDir := filepath.Join(contextDir, dir)
		if _, statErr := os.Stat(entryDir); statErr != nil {
			continue
		}
		entrySet, err := ParseRulesDir(entryDir, detectOrigin)
		if err != nil {
			return RuleSet{}, fmt.Errorf("ParseContext: %w", err)
		}
		merged.Entries = append(merged.Entries, entrySet.Entries...)
		merged.Tombstones = append(merged.Tombstones, entrySet.Tombstones...)
		for _, guide := range entrySet.Guides {
			relPath, err := filepath.Rel(contextDir, guide.Path)
			if err != nil {
				return RuleSet{}, fmt.Errorf("ParseContext: %w", err)
			}
			guide.Path = filepath.ToSlash(relPath)
			merged.Guides = append(merged.Guides, guide)
		}
	}

	factsDir := filepath.Join(contextDir, "facts")
	if _, statErr := os.Stat(factsDir); statErr == nil {
		factsSet, err := ParseFactsDir(factsDir)
		if err != nil {
			return RuleSet{}, fmt.Errorf("ParseContext: %w", err)
		}
		merged.Entries = append(merged.Entries, factsSet.Entries...)
		merged.Tombstones = append(merged.Tombstones, factsSet.Tombstones...)
	}
	return merged, nil
}

// ParseFactsDir parses every .md file in a repo-owned facts directory with
// overlay origin — facts never ship in the synced baseline.
func ParseFactsDir(dir string) (RuleSet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return RuleSet{}, fmt.Errorf("ParseFactsDir: %w", err)
	}

	var set RuleSet
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		source := filepath.ToSlash(filepath.Join(dir, entry.Name()))
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return RuleSet{}, fmt.Errorf("ParseFactsDir: %w", err)
		}
		if err := parseRulesFile(string(content), source, RuleOriginOverlay, &set); err != nil {
			return RuleSet{}, fmt.Errorf("ParseFactsDir: %w", err)
		}
	}
	return set, nil
}

// parseRulesFile appends the file's entries and tombstones to set. It walks
// the file line-wise: entry headlines open an entry, known bullets fill it,
// a "## Tombstones" heading switches to table parsing.
func parseRulesFile(content, source, origin string, set *RuleSet) error {
	var currentEntry *RuleEntry
	isInFence := false
	isInTombstones := false

	lines := strings.Split(content, "\n")
	for lineNumber, line := range lines {
		if strings.HasPrefix(line, "```") {
			isInFence = !isInFence
			currentEntry = nil
			continue
		}
		if isInFence {
			continue
		}

		if strings.HasPrefix(line, "## ") {
			isInTombstones = strings.TrimSpace(line) == "## Tombstones"
			currentEntry = nil
			continue
		}

		if isInTombstones {
			tombstone, isRow := parseTombstoneRow(line, source)
			if isRow {
				set.Tombstones = append(set.Tombstones, tombstone)
			}
			continue
		}

		if match := ruleFilesPattern.FindStringSubmatch(line); match != nil {
			for _, guide := range set.Guides {
				if guide.Source == source {
					return fmt.Errorf("parseRulesFile: %s:%d: second Files line", source, lineNumber+1)
				}
			}
			var files []string
			for _, glob := range ruleFilesGlobPattern.FindAllStringSubmatch(match[1], -1) {
				files = append(files, glob[1])
			}
			if len(files) == 0 {
				return fmt.Errorf("parseRulesFile: %s:%d: Files line names no backticked glob", source, lineNumber+1)
			}
			set.Guides = append(set.Guides, RuleGuide{
				Name:   strings.TrimSuffix(filepath.Base(source), ".md"),
				Path:   source,
				Files:  files,
				Source: source,
			})
			continue
		}

		if match := ruleEntryPattern.FindStringSubmatch(line); match != nil {
			number, err := strconv.Atoi(match[4])
			if err != nil {
				return fmt.Errorf("parseRulesFile: %s:%d: %w", source, lineNumber+1, err)
			}
			id := fmt.Sprintf("%s-%s-%s", match[1], match[2], match[4])
			if match[3] != "" {
				id = fmt.Sprintf("%s-%s-%s-%s", match[1], match[2], match[3], match[4])
			}
			set.Entries = append(set.Entries, RuleEntry{
				Id:          id,
				Kind:        strings.ToLower(match[1]),
				Scope:       match[2],
				Topic:       match[3],
				Number:      number,
				Enforcement: match[5],
				Content:     parseEntryContent(strings.TrimSpace(match[6]), lines[lineNumber+1:entryContentEnd(lines, lineNumber)]),
				Reach:       reach.Global,
				Version:     "1.0",
				Source:      source,
				Origin:      origin,
			})
			currentEntry = &set.Entries[len(set.Entries)-1]
			continue
		}
		if ruleEntryPrefixPattern.MatchString(line) {
			return fmt.Errorf("parseRulesFile: %s:%d: malformed entry line: %s", source, lineNumber+1, line)
		}

		if match := ruleBulletPattern.FindStringSubmatch(line); match != nil && currentEntry != nil {
			value := strings.TrimSpace(match[2])
			switch match[1] {
			case "Version":
				currentEntry.Version = value
			case "Reach":
				currentEntry.Reach = value
			}
			continue
		}

		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "* ") {
			currentEntry = nil
		}
	}
	return nil
}

// entryContentEnd returns the exclusive end index of the full authored block
// opening at start: everything up to the next entry headline, "## " heading,
// or "---" rule — fenced examples included (unlike entryBlockEnd, which
// stops at the bullets for reach filtering). Fence interiors never terminate
// the block, so a "---" or "## " inside an example stays part of it.
func entryContentEnd(lines []string, start int) int {
	isInFence := false
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "```") {
			isInFence = !isInFence
			continue
		}
		if isInFence {
			continue
		}
		isBoundary := ruleEntryPrefixPattern.MatchString(line) || strings.HasPrefix(line, "## ") || strings.TrimSpace(line) == "---"
		if isBoundary {
			return index
		}
	}
	return len(lines)
}

// parseEntryContent types the body lines of one entry: tagged bullets fill the
// named fields (Version/Reach are entry metadata and are skipped here),
// untagged top-level bullets become Bullets with their indented continuation
// lines joined, fenced blocks become Examples, and remaining prose lines land
// in Notes. Blank lines separate, nothing else is dropped.
func parseEntryContent(statement string, body []string) RuleContent {
	content := RuleContent{Statement: statement}
	var fence *RuleExample
	for _, line := range body {
		if strings.HasPrefix(line, "```") {
			if fence == nil {
				fence = &RuleExample{Lang: strings.TrimSpace(strings.TrimPrefix(line, "```"))}
			} else {
				fence.Code = strings.TrimSuffix(fence.Code, "\n")
				content.Examples = append(content.Examples, *fence)
				fence = nil
			}
			continue
		}
		if fence != nil {
			fence.Code += line + "\n"
			continue
		}
		if match := ruleBulletPattern.FindStringSubmatch(line); match != nil {
			value := strings.TrimSpace(match[2])
			switch match[1] {
			case "Why":
				content.Why = value
			case "Applies":
				content.Applies = value
			case "Evidence":
				content.Evidence = value
			case "Location":
				content.Location = value
			}
			continue
		}
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case strings.HasPrefix(line, "* "):
			content.Bullets = append(content.Bullets, strings.TrimPrefix(line, "* "))
		case strings.HasPrefix(line, " ") && len(content.Bullets) > 0:
			content.Bullets[len(content.Bullets)-1] += "\n" + line
		default:
			content.Notes = append(content.Notes, line)
		}
	}
	return content
}

// parseTombstoneRow parses one markdown table row into a tombstone. Header
// and separator rows report isRow false.
func parseTombstoneRow(line, source string) (tombstone RuleTombstone, isRow bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return RuleTombstone{}, false
	}

	var cells []string
	for _, cell := range strings.Split(strings.Trim(trimmed, "|"), "|") {
		cells = append(cells, strings.TrimSpace(cell))
	}
	if len(cells) < 3 || cells[0] == "Retired" || strings.HasPrefix(cells[0], ":-") || strings.HasPrefix(cells[0], "-") {
		return RuleTombstone{}, false
	}
	return RuleTombstone{
		Retired:     cells[0],
		Replacement: cells[1],
		Date:        cells[2],
		Source:      source,
	}, true
}

// ValidateRules checks a parsed set against the registry contract and the
// given aspect vocabulary, returning one message per violation.
func ValidateRules(set RuleSet, aspects []RuleAspect) []string {
	var violations []string

	scopeNames, topicNames := map[string]bool{}, map[string]bool{}
	for _, aspect := range aspects {
		switch aspect.Class {
		case "scope":
			scopeNames[aspect.Name] = true
		case "topic":
			topicNames[aspect.Name] = true
		}
	}

	seenIds := map[string]string{}
	retiredIds := map[string]string{}
	for _, tombstone := range set.Tombstones {
		retiredIds[tombstone.Retired] = tombstone.Source
	}

	for _, entry := range set.Entries {
		if !reach.Valid(entry.Reach) {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: Reach must be global, none, or a comma-separated repo-name list", entry.Source, entry.Id))
		}
		if entry.Scope == "SMINE" && entry.Reach != reach.ThisRepo {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: scope SMINE entries govern this repo's own pipeline — Reach: smine required", entry.Source, entry.Id))
		}
		if entry.Scope == "REPO" && entry.Reach == reach.Global {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: scope REPO facts describe one repo — Reach: that repo's roster name, never global", entry.Source, entry.Id))
		}
		if previousSource, isSeen := seenIds[entry.Id]; isSeen {
			violations = append(violations, fmt.Sprintf(
				"%s: duplicate id %s (already defined in %s)", entry.Source, entry.Id, previousSource))
		}
		seenIds[entry.Id] = entry.Source

		if tombstoneSource, isRetired := retiredIds[entry.Id]; isRetired {
			violations = append(violations, fmt.Sprintf(
				"%s: id %s is tombstoned in %s — numbers are never reused", entry.Source, entry.Id, tombstoneSource))
		}

		if !scopeNames[entry.Scope] {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: scope %s is not a registered class-scope taxonomy entry", entry.Source, entry.Id, entry.Scope))
		}
		if entry.Topic != "" && !topicNames[entry.Topic] {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: topic %s is not a registered class-topic taxonomy entry", entry.Source, entry.Id, entry.Topic))
		}

		switch entry.Kind {
		case RuleKindFact:
			if entry.Enforcement != "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: FACT entries carry no enforcement tag", entry.Source, entry.Id))
			}
			if entry.Content.Location == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: FACT entries require a Location bullet", entry.Source, entry.Id))
			}
			if entry.Origin == RuleOriginBaseline {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: FACT entries are repo-owned — facts never ship in the synced baseline", entry.Source, entry.Id))
			}
		case RuleKindAction:
			if entry.Enforcement == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: ACTION entries require an enforcement tag", entry.Source, entry.Id))
			} else if !ruleEnforcements[entry.Enforcement] {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: unknown enforcement tag [%s]", entry.Source, entry.Id, entry.Enforcement))
			}
			if entry.Content.Applies == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: ACTION entries require an Applies bullet", entry.Source, entry.Id))
			}
			if entry.Origin == RuleOriginBaseline && entry.Number > 99 {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: baseline entries use numbers 001-099", entry.Source, entry.Id))
			}
			if entry.Origin == RuleOriginOverlay && entry.Number < 100 {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: overlay entries use numbers 100+", entry.Source, entry.Id))
			}
		case RuleKindRule:
			if entry.Enforcement == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: RULE entries require an enforcement tag", entry.Source, entry.Id))
			} else if !ruleEnforcements[entry.Enforcement] {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: unknown enforcement tag [%s]", entry.Source, entry.Id, entry.Enforcement))
			}
		}
	}

	sort.Strings(violations)
	return violations
}

// ContextFile is the generated context.json content — every entry, tombstone,
// and the aspect taxonomy in one machine-readable file. A deployed target's
// copy additionally carries a "deploy" section with its sync settings; the
// generator never writes one, and readers here never need it.
type ContextFile struct {
	Entries    []RuleEntry     `json:"entries"`
	Tombstones []RuleTombstone `json:"tombstones"`
	Guides     []RuleGuide     `json:"guides"`
	Aspects    []RuleAspect    `json:"aspects"`
}

// RenderContextJson renders the committed context.json file content; aspects
// are name-sorted so every writer produces identical bytes.
func RenderContextJson(set RuleSet, aspects []RuleAspect) ([]byte, error) {
	sort.Slice(aspects, func(left, right int) bool {
		return aspects[left].Name < aspects[right].Name
	})
	sort.Slice(set.Guides, func(left, right int) bool {
		return set.Guides[left].Name < set.Guides[right].Name
	})
	contextFile := ContextFile{
		Entries:    set.Entries,
		Tombstones: set.Tombstones,
		Guides:     set.Guides,
		Aspects:    aspects,
	}
	data, err := json.MarshalIndent(contextFile, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("RenderContextJson: %w", err)
	}
	return append(data, '\n'), nil
}
