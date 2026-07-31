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
)

// Entry classes.
const (
	RuleClassAlways = "always"
	RuleClassFact   = "fact"
	RuleClassNever  = "never"
)

// Entry origins.
const (
	RuleOriginBaseline = "baseline"
	RuleOriginOverlay  = "overlay"
)

// BaselineHeaderMarker identifies a synced baseline file inside a deployed
// pack; a rules file without it is a repo-owned overlay.
const BaselineHeaderMarker = "synced from smine"

// AspectsFileName is the vocabulary file inside a rules directory.
const AspectsFileName = "aspects.json"

// RuleAspect is one member of the closed aspect vocabulary.
type RuleAspect struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

// LoadAspects reads the closed aspect vocabulary from dir/aspects.json.
func LoadAspects(dir string) ([]RuleAspect, error) {
	data, err := os.ReadFile(filepath.Join(dir, AspectsFileName))
	if err != nil {
		return nil, fmt.Errorf("LoadAspects: %w", err)
	}
	var aspects []RuleAspect
	if err := json.Unmarshal(data, &aspects); err != nil {
		return nil, fmt.Errorf("LoadAspects: %w", err)
	}
	return aspects, nil
}

// SaveAspects writes the vocabulary name-sorted and pretty-printed, via a
// temp file + rename so a crashed write never truncates the vocabulary.
func SaveAspects(dir string, aspects []RuleAspect) error {
	sort.Slice(aspects, func(left, right int) bool {
		return aspects[left].Name < aspects[right].Name
	})
	data, err := json.MarshalIndent(aspects, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveAspects: %w", err)
	}
	data = append(data, '\n')

	tmpPath := filepath.Join(dir, AspectsFileName+".tmp")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("SaveAspects: %w", err)
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, AspectsFileName)); err != nil {
		return fmt.Errorf("SaveAspects: %w", err)
	}
	return nil
}

// ruleEnforcements are the valid enforcement tags on NEVER/ALWAYS entries.
var ruleEnforcements = map[string]bool{
	"gate":   true,
	"hook":   true,
	"lint":   true,
	"manual": true,
	"review": true,
}

// RuleEntry is one FACT/NEVER/ALWAYS entry parsed from a rules markdown file.
type RuleEntry struct {
	Id          string `json:"id"`
	Class       string `json:"class"`
	Aspect      string `json:"aspect"`
	Number      int    `json:"-"`
	Statement   string `json:"statement"`
	Enforcement string `json:"enforcement,omitempty"`
	Applies     string `json:"applies,omitempty"`
	Location    string `json:"location,omitempty"`
	Source      string `json:"source"`
	Origin      string `json:"origin"`
}

// RuleTombstone is one retired-entry row from a "## Tombstones" table.
type RuleTombstone struct {
	Retired     string `json:"retired"`
	Replacement string `json:"replacement"`
	Date        string `json:"date"`
	Source      string `json:"source"`
}

// RuleSet holds every entry and tombstone parsed from one rules directory.
type RuleSet struct {
	Entries    []RuleEntry     `json:"entries"`
	Tombstones []RuleTombstone `json:"tombstones"`
}

var (
	ruleEntryPattern = regexp.MustCompile(
		`^\*\*(FACT|NEVER|ALWAYS)-([A-Z]+)-([0-9]{3})\*\*(?: \x60\[([a-z]+)\]\x60)? — (.+)$`)
	ruleEntryPrefixPattern = regexp.MustCompile(`^\*\*(FACT|NEVER|ALWAYS)-`)
	ruleBulletPattern      = regexp.MustCompile(`^\* (Why|Applies|Evidence|Location): (.+)$`)
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

// ParsePack parses a deployed pack: rules/ with origin detection plus facts/
// as repo-owned overlay files. Either directory may be absent; an entirely
// missing pack dir is an error.
func ParsePack(packDir string) (RuleSet, error) {
	if _, err := os.Stat(packDir); err != nil {
		return RuleSet{}, fmt.Errorf("ParsePack: %w", err)
	}

	var merged RuleSet
	rulesDir := filepath.Join(packDir, "rules")
	if _, statErr := os.Stat(rulesDir); statErr == nil {
		rulesSet, err := ParseRulesDir(rulesDir, true)
		if err != nil {
			return RuleSet{}, fmt.Errorf("ParsePack: %w", err)
		}
		merged.Entries = append(merged.Entries, rulesSet.Entries...)
		merged.Tombstones = append(merged.Tombstones, rulesSet.Tombstones...)
	}

	factsDir := filepath.Join(packDir, "facts")
	if _, statErr := os.Stat(factsDir); statErr == nil {
		factsSet, err := ParseFactsDir(factsDir)
		if err != nil {
			return RuleSet{}, fmt.Errorf("ParsePack: %w", err)
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

	for lineNumber, line := range strings.Split(content, "\n") {
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

		if match := ruleEntryPattern.FindStringSubmatch(line); match != nil {
			number, err := strconv.Atoi(match[3])
			if err != nil {
				return fmt.Errorf("parseRulesFile: %s:%d: %w", source, lineNumber+1, err)
			}
			set.Entries = append(set.Entries, RuleEntry{
				Id:          fmt.Sprintf("%s-%s-%s", match[1], match[2], match[3]),
				Class:       strings.ToLower(match[1]),
				Aspect:      match[2],
				Number:      number,
				Statement:   strings.TrimSpace(match[5]),
				Enforcement: match[4],
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
			case "Applies":
				currentEntry.Applies = value
			case "Location":
				currentEntry.Location = value
			}
			continue
		}

		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "* ") {
			currentEntry = nil
		}
	}
	return nil
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

	aspectNames := map[string]bool{}
	for _, aspect := range aspects {
		aspectNames[aspect.Name] = true
	}

	seenIds := map[string]string{}
	retiredIds := map[string]string{}
	for _, tombstone := range set.Tombstones {
		retiredIds[tombstone.Retired] = tombstone.Source
	}

	for _, entry := range set.Entries {
		if previousSource, isSeen := seenIds[entry.Id]; isSeen {
			violations = append(violations, fmt.Sprintf(
				"%s: duplicate id %s (already defined in %s)", entry.Source, entry.Id, previousSource))
		}
		seenIds[entry.Id] = entry.Source

		if tombstoneSource, isRetired := retiredIds[entry.Id]; isRetired {
			violations = append(violations, fmt.Sprintf(
				"%s: id %s is tombstoned in %s — numbers are never reused", entry.Source, entry.Id, tombstoneSource))
		}

		if !aspectNames[entry.Aspect] {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: unknown aspect %s", entry.Source, entry.Id, entry.Aspect))
		}

		switch entry.Class {
		case RuleClassFact:
			if entry.Enforcement != "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: FACT entries carry no enforcement tag", entry.Source, entry.Id))
			}
			if entry.Location == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: FACT entries require a Location bullet", entry.Source, entry.Id))
			}
			if entry.Origin == RuleOriginBaseline {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: FACT entries are repo-owned — facts never ship in the synced baseline", entry.Source, entry.Id))
			}
		case RuleClassNever, RuleClassAlways:
			if entry.Enforcement == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: NEVER/ALWAYS entries require an enforcement tag", entry.Source, entry.Id))
			} else if !ruleEnforcements[entry.Enforcement] {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: unknown enforcement tag [%s]", entry.Source, entry.Id, entry.Enforcement))
			}
			if entry.Applies == "" {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: NEVER/ALWAYS entries require an Applies bullet", entry.Source, entry.Id))
			}
			if entry.Origin == RuleOriginBaseline && entry.Number > 99 {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: baseline entries use numbers 001-099", entry.Source, entry.Id))
			}
			if entry.Origin == RuleOriginOverlay && entry.Number < 100 {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: overlay entries use numbers 100+", entry.Source, entry.Id))
			}
		}
	}

	sort.Strings(violations)
	return violations
}

// RenderRulesJson renders the set as the committed registry file content.
func RenderRulesJson(set RuleSet) ([]byte, error) {
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("RenderRulesJson: %w", err)
	}
	return append(data, '\n'), nil
}
