package contextdocs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/reach"

	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/fsx"
)

// Polarity lives in the statement, never in the kind.
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

// An entry file without it is a repo-owned overlay.
const BaselineHeaderMarker = "synced from smine"

// ContextFileName is the generated, untracked machine-readable file at the context root.
const ContextFileName = "context.json"

// AspectsFileName is the tracked taxonomy source; its presence marks a context root that generates ContextFileName.
const AspectsFileName = "aspects.json"

// SourceContextDir is the context root of a repo that authors the context system.
const SourceContextDir = "context"

// ContextDirDefaultKey names the deployed context dir in the hook environment; docs when unset.
const ContextDirDefaultKey = "AGENT_CONTEXT_DIR_DEFAULT"

const deployedDirName = "docs"

// Aspect classes: the applies-to segment and the optional third segment of an entry id.
const (
	AspectClassScope = "scope"
	AspectClassTopic = "topic"
)

// Class is AspectClassScope or AspectClassTopic.
type RuleAspect struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Class string `json:"class,omitempty"`
	Lang  string `json:"lang,omitempty"` // rules/<lang>.md guide basename for language scopes; empty = not language-bound
}

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

// Name-sorted, so two branches adding different aspects touch different lines.
func RenderAspects(aspects []RuleAspect) ([]byte, error) {
	sorted := slices.Clone(aspects)
	slices.SortFunc(sorted, func(a, b RuleAspect) int { return strings.Compare(a.Name, b.Name) })

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(sorted); err != nil {
		return nil, fmt.Errorf("RenderAspects: %w", err)
	}
	return buffer.Bytes(), nil
}

func SaveAspects(dir string, aspects []RuleAspect) error {
	data, err := RenderAspects(aspects)
	if err != nil {
		return fmt.Errorf("SaveAspects: %w", err)
	}
	if err := fsx.WriteFileAtomic(data, filepath.Join(dir, AspectsFileName), 0o644); err != nil {
		return fmt.Errorf("SaveAspects: %w", err)
	}
	return nil
}

// Byte-identical to cmd/rules generate; temp file + rename so a crashed write never truncates.
func WriteContextFile(dir string) error {
	set, err := ParseContext(dir, false)
	if err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	aspects, err := LoadAspects(dir)
	if err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	data, err := RenderContextJson(set, aspects, reach.ThisRepo)
	if err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	if err := fsx.WriteFileAtomic(data, filepath.Join(dir, ContextFileName), 0o644); err != nil {
		return fmt.Errorf("WriteContextFile: %w", err)
	}
	return nil
}

// Regenerate writes dir's context file; a dir without the aspects source, or holding a deployed index, is left alone.
func Regenerate(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, AspectsFileName)); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if IsDeployedIndex(filepath.Join(dir, ContextFileName)) {
		return nil
	}
	if err := WriteContextFile(dir); err != nil {
		return fmt.Errorf("Regenerate: %w", err)
	}
	return nil
}

// Channel and role tag literals — named once, the taxonomy's own vocabulary.
const (
	channelGate   = "gate"
	channelHook   = "hook"
	channelLint   = "lint"
	channelProse  = "prose"
	channelSchema = "schema"
	roleStop      = "stop"
)

// At most one role and one channel tag.
const maxTagsPerSlot = 2

// ruleChannels are the valid enforcement-channel tags: what proves adherence.
var ruleChannels = map[string]bool{
	channelGate:   true,
	channelHook:   true,
	channelLint:   true,
	channelProse:  true,
	channelSchema: true,
}

// What provably changes when the entry is removed; only ACTION entries carry a role today.
var ruleRoles = map[string]bool{
	roleStop: true,
}

// Channels whose Hook:/Gate:/Schema: bullet is checked against reality.
var pointerChannels = map[string]bool{
	channelHook:   true,
	channelGate:   true,
	channelSchema: true,
}

// An optional second backticked tag flagging an entry as a member of a named set.
var ruleMarkers = map[string]bool{
	"DoD": true,
}

// Lang is the fence info string, "" for a bare fence.
type RuleExample struct {
	Code string `json:"code"`
	Lang string `json:"lang,omitempty"`
}

// The delivery form hooks render, so the .md is authoring only.
type RuleContent struct {
	Applies   string        `json:"applies,omitempty"`
	Bullets   []string      `json:"bullets,omitempty"`
	Evidence  string        `json:"evidence,omitempty"`
	Examples  []RuleExample `json:"examples,omitempty"`
	Gate      string        `json:"gate,omitempty"`
	Hook      string        `json:"hook,omitempty"`
	Location  string        `json:"location,omitempty"`
	Notes     []string      `json:"notes,omitempty"`
	Schema    string        `json:"schema,omitempty"`
	Statement string        `json:"statement"`
	Why       string        `json:"why,omitempty"`
}

// Id grammar KIND-SCOPE[-TOPIC]-NNN.
type RuleEntry struct {
	Id          string      `json:"id"`
	Content     RuleContent `json:"content"`
	Enforcement string      `json:"enforcement,omitempty"`
	Kind        string      `json:"kind"`
	Markers     []string    `json:"markers,omitempty"`
	Number      int         `json:"-"`
	Origin      string      `json:"origin"`
	Reach       string      `json:"reach"`
	Role        string      `json:"role,omitempty"`
	Scope       string      `json:"scope"`
	Source      string      `json:"source"`
	Topic       string      `json:"topic,omitempty"`
	Version     string      `json:"version"`
}

// RuleTombstone is one retired-entry row from a "## Tombstones" table.
type RuleTombstone struct {
	Retired     string `json:"retired"`
	Replacement string `json:"replacement"`
	Date        string `json:"date"`
	Source      string `json:"source"`
}

// The read-gate hook matches touched paths against Files to require the guide.
type RuleGuide struct {
	Files  []string `json:"files"`
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Source string   `json:"source"`
}

// The reach repo is this repo for a source tree, none for a deployed tree (reach-filtered at sync).
type RuleSet struct {
	Entries    []RuleEntry     `json:"entries"`
	Tombstones []RuleTombstone `json:"tombstones"`
	Guides     []RuleGuide     `json:"guides"`
	Self       string          `json:"-"`
}

var (
	// Group 5 captures one or two adjacent bracketed tags, role first; splitContextTags splits them.
	ruleEntryPattern = regexp.MustCompile(
		`^\*\*(FACT|ACTION|RULE)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-([0-9]{3})\*\*(?: \x60((?:\[[a-z]+\])+)\x60)?(?: \x60\[([A-Za-z]+)\]\x60)? — (.+)$`)
	ruleEntryPrefixPattern = regexp.MustCompile(`^\*\*(FACT|ACTION|RULE)-`)
	ruleBulletPattern      = regexp.MustCompile(`^\* (Why|Applies|Evidence|Location|Version|Reach|Hook|Gate|Schema): (.+)$`)
	ruleFilesPattern       = regexp.MustCompile(`^\*\*Files:\*\* (.+)$`)
	ruleFilesGlobPattern   = regexp.MustCompile("`([^`]+)`")
	ruleTagPattern         = regexp.MustCompile(`\[([a-z]+)\]`)
	statementListPattern   = regexp.MustCompile(`^\d+\. `)
)

// A role implies channel "prose"; assumes checkTagSlotShape already passed.
func splitContextTags(tags []string) (role, channel string) {
	for _, tag := range tags {
		switch {
		case ruleRoles[tag] && role == "":
			role = tag
		case ruleChannels[tag] && channel == "":
			channel = tag
		}
	}
	if role != "" && channel == "" {
		channel = channelProse
	}
	return role, channel
}

func checkTagSlotShape(tags []string) string {
	if len(tags) > maxTagsPerSlot {
		return fmt.Sprintf("tag slot [%s] holds more than 2 tags", strings.Join(tags, "]["))
	}
	seenRole, seenChannel := false, false
	for index, tag := range tags {
		switch {
		case ruleRoles[tag]:
			if seenRole {
				return fmt.Sprintf("two role tags in [%s]", strings.Join(tags, "]["))
			}
			if index != 0 {
				return fmt.Sprintf("role tag [%s] must come first", tag)
			}
			seenRole = true
		case ruleChannels[tag]:
			if seenChannel {
				return fmt.Sprintf("two channel tags in [%s]", strings.Join(tags, "]["))
			}
			seenChannel = true
		default:
			return fmt.Sprintf("unknown tag [%s]", tag)
		}
	}
	return ""
}

// Fenced code blocks are skipped, so authoring docs can hold example entries.
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

// Any subdirectory may be absent; an entirely missing context dir is an error.
func ParseContext(contextDir string, detectOrigin bool) (RuleSet, error) {
	if _, err := os.Stat(contextDir); err != nil {
		return RuleSet{}, fmt.Errorf("ParseContext: %w", err)
	}

	var merged RuleSet
	if !detectOrigin {
		merged.Self = reach.ThisRepo
	}

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

// ResolveContextDir: explicit wins; else context/ holding context.json; else the deployed dir holding it; else an error naming both.
func ResolveContextDir(explicit, deployedName string) (string, bool, error) {
	if explicit != "" {
		return explicit, false, nil
	}
	if deployedName == "" {
		deployedName = deployedDirName
	}

	if _, err := os.Stat(filepath.Join(SourceContextDir, ContextFileName)); err == nil {
		return SourceContextDir, false, nil
	}
	if _, err := os.Stat(filepath.Join(deployedName, ContextFileName)); err == nil {
		return deployedName, true, nil
	}
	return "", false, fmt.Errorf("ResolveContextDir: No context dir — neither %s/%s nor %s/%s exists", SourceContextDir, ContextFileName, deployedName, ContextFileName)
}

// Facts never ship in the synced baseline, so they parse with overlay origin.
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
			var markers []string
			if match[6] != "" {
				markers = []string{match[6]}
			}
			var rawTags []string
			for _, tag := range ruleTagPattern.FindAllStringSubmatch(match[5], -1) {
				rawTags = append(rawTags, tag[1])
			}
			if shapeErr := checkTagSlotShape(rawTags); shapeErr != "" {
				return fmt.Errorf("parseRulesFile: %s:%d: %s", source, lineNumber+1, shapeErr)
			}
			role, channel := splitContextTags(rawTags)
			set.Entries = append(set.Entries, RuleEntry{
				Id:          id,
				Kind:        strings.ToLower(match[1]),
				Scope:       match[2],
				Topic:       match[3],
				Number:      number,
				Enforcement: channel,
				Role:        role,
				Markers:     markers,
				Content:     parseEntryContent(strings.TrimSpace(match[7]), lines[lineNumber+1:entryContentEnd(lines, lineNumber)]),
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

// The block runs to the next entry, heading or rule; fence interiors never end it.
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

// Version and Reach are entry metadata and skipped here; nothing but blank lines is dropped.
func parseEntryContent(statement string, body []string) RuleContent {
	headline := []string{statement}
	for len(body) > 0 && isStatementContinuation(body[0]) {
		headline = append(headline, strings.TrimSpace(body[0]))
		body = body[1:]
	}
	content := RuleContent{Statement: strings.Join(headline, " ")}
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
			case "Hook":
				content.Hook = value
			case "Gate":
				content.Gate = value
			case "Schema":
				content.Schema = value
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

func isStatementContinuation(line string) bool {
	if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") {
		return false
	}
	for _, prefix := range []string{"* ", "- ", "|", "#", "```", ">"} {
		if strings.HasPrefix(line, prefix) {
			return false
		}
	}
	return !statementListPattern.MatchString(line)
}

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

// Works whether the entry came from parsing or direct construction (tests).
func validateTags(entry RuleEntry) string {
	if entry.Role != "" && !ruleRoles[entry.Role] {
		return fmt.Sprintf("%s: %s: unknown role tag [%s]", entry.Source, entry.Id, entry.Role)
	}
	if entry.Enforcement != "" && !ruleChannels[entry.Enforcement] {
		return fmt.Sprintf("%s: %s: unknown channel tag [%s]", entry.Source, entry.Id, entry.Enforcement)
	}
	return ""
}

func validateChannelClaim(entry RuleEntry, repoRoot string, acdslIds map[string]bool) string {
	if !pointerChannels[entry.Enforcement] {
		return ""
	}
	switch entry.Enforcement {
	case channelHook:
		if entry.Content.Hook == "" {
			return fmt.Sprintf("%s: %s: [hook] requires a Hook: bullet naming the enforcing script", entry.Source, entry.Id)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, "cmd/hooks", entry.Content.Hook)); err != nil {
			return fmt.Sprintf("%s: %s: Hook: %s not found under cmd/hooks/", entry.Source, entry.Id, entry.Content.Hook)
		}
	case channelGate:
		if entry.Content.Gate == "" {
			return fmt.Sprintf("%s: %s: [gate] requires a Gate: bullet naming the ACDSL rule id", entry.Source, entry.Id)
		}
		if !acdslIds[entry.Content.Gate] {
			return fmt.Sprintf("%s: %s: Gate: %s not declared in acdsl/rules.acdsl", entry.Source, entry.Id, entry.Content.Gate)
		}
	case channelSchema:
		if entry.Content.Schema == "" {
			return fmt.Sprintf("%s: %s: [schema] requires a Schema: bullet naming the schema file", entry.Source, entry.Id)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, entry.Content.Schema)); err != nil {
			return fmt.Sprintf("%s: %s: Schema: %s not found", entry.Source, entry.Id, entry.Content.Schema)
		}
	}
	return ""
}

// acdslIdPattern matches a declaration id in acdsl/*.acdsl files.
var acdslIdPattern = regexp.MustCompile(`//acdsl:(\S+)`)

// A missing file yields an empty set, so the Gate: claim fails loudly rather than passing silently.
func loadAcdslIds(repoRoot string) map[string]bool {
	ids := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(repoRoot, "acdsl"))
	if err != nil {
		return ids
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".acdsl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot, "acdsl", entry.Name()))
		if err != nil {
			continue
		}
		for _, match := range acdslIdPattern.FindAllStringSubmatch(string(data), -1) {
			ids[match[1]] = true
		}
	}
	return ids
}

func validateFactEntry(entry RuleEntry) []string {
	var violations []string
	if entry.Enforcement != "" || entry.Role != "" {
		violations = append(violations, fmt.Sprintf(
			"%s: %s: FACT entries carry no role or channel tag", entry.Source, entry.Id))
	}
	if len(entry.Markers) > 0 {
		violations = append(violations, fmt.Sprintf(
			"%s: %s: FACT entries carry no marker tag", entry.Source, entry.Id))
	}
	if entry.Content.Location == "" {
		violations = append(violations, fmt.Sprintf(
			"%s: %s: FACT entries require a Location bullet", entry.Source, entry.Id))
	}
	if entry.Origin == RuleOriginBaseline {
		violations = append(violations, fmt.Sprintf(
			"%s: %s: FACT entries are repo-owned — facts never ship in the synced baseline", entry.Source, entry.Id))
	}
	return violations
}

func validateActionEntry(entry RuleEntry, repoRoot string, acdslIds map[string]bool) []string {
	var violations []string
	if tagViolation := validateTags(entry); tagViolation != "" {
		violations = append(violations, tagViolation)
	} else if entry.Enforcement == "" {
		violations = append(violations, fmt.Sprintf(
			"%s: %s: ACTION entries require a channel tag", entry.Source, entry.Id))
	} else if claimViolation := validateChannelClaim(entry, repoRoot, acdslIds); claimViolation != "" {
		violations = append(violations, claimViolation)
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
	return violations
}

func validateRuleEntry(entry RuleEntry, repoRoot string, acdslIds map[string]bool) []string {
	if entry.Role != "" {
		return []string{fmt.Sprintf(
			"%s: %s: RULE entries carry no role tag — role applies to ACTION only", entry.Source, entry.Id)}
	}
	if tagViolation := validateTags(entry); tagViolation != "" {
		return []string{tagViolation}
	}
	if entry.Enforcement == "" {
		return []string{fmt.Sprintf(
			"%s: %s: RULE entries require a channel tag", entry.Source, entry.Id)}
	}
	if claimViolation := validateChannelClaim(entry, repoRoot, acdslIds); claimViolation != "" {
		return []string{claimViolation}
	}
	return nil
}

func ValidateRules(set RuleSet, aspects []RuleAspect, repoRoot string) []string {
	var violations []string
	acdslIds := loadAcdslIds(repoRoot)

	scopeNames, topicNames := map[string]bool{}, map[string]bool{}
	for _, aspect := range aspects {
		switch aspect.Class {
		case AspectClassScope:
			scopeNames[aspect.Name] = true
		case AspectClassTopic:
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
				"%s: %s: Reach must be global, always, none, or a comma-separated repo-name list", entry.Source, entry.Id))
		}
		if entry.Scope == "SMINE" && entry.Reach != reach.ThisRepo {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: scope SMINE entries govern this repo's own pipeline — Reach: smine required", entry.Source, entry.Id))
		}
		if entry.Scope == "REPO" && entry.Reach == reach.Global {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: scope REPO facts describe one repo — Reach: that repo's roster name, never global", entry.Source, entry.Id))
		}
		if entry.Scope == "REPO" && entry.Reach == reach.Always {
			violations = append(violations, fmt.Sprintf(
				"%s: %s: scope REPO facts describe one repo — never always-injected", entry.Source, entry.Id))
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
		for _, marker := range entry.Markers {
			if !ruleMarkers[marker] {
				violations = append(violations, fmt.Sprintf(
					"%s: %s: unknown marker tag [%s]", entry.Source, entry.Id, marker))
			}
		}

		switch entry.Kind {
		case RuleKindFact:
			violations = append(violations, validateFactEntry(entry)...)
		case RuleKindAction:
			violations = append(violations, validateActionEntry(entry, repoRoot, acdslIds)...)
		case RuleKindRule:
			violations = append(violations, validateRuleEntry(entry, repoRoot, acdslIds)...)
		}
	}

	sort.Strings(violations)
	return violations
}

// ValidateBudgets reports every language guide and the rendered always pack over the required-reading budget.
func ValidateBudgets(contextDir string, set RuleSet) []string {
	var violations []string
	for _, guide := range set.Guides {
		info, err := os.Stat(guide.Source)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", guide.Path, err))
			continue
		}

		if info.Size() > contextrequired.ReadBudgetBytes {
			violations = append(violations, fmt.Sprintf(
				"%s: guide is %d bytes, over the required-reading budget of %d", guide.Source, info.Size(), contextrequired.ReadBudgetBytes))
		}
	}

	packed, err := PackAlwaysDir(contextDir)
	if err != nil {
		return append(violations, fmt.Sprintf("always pack: %v", err))
	}

	if len(packed) > contextrequired.ReadBudgetBytes {
		violations = append(violations, fmt.Sprintf(
			"always pack: %d bytes, over the required-reading budget of %d", len(packed), contextrequired.ReadBudgetBytes))
	}
	return violations
}

// DeployRecordDir is the state-dir folder holding one build per registry repo.
const DeployRecordDir = "context"

// ContextDeploy records how a deployed index was built; its presence marks the index as deployed.
type ContextDeploy struct {
	Acdsl      bool     `json:"acdsl"`
	ContextDir string   `json:"contextDir"`
	Langs      []string `json:"langs"`
	Markdown   bool     `json:"markdown"`
	Role       string   `json:"role"`
	Task       bool     `json:"task"`
	WriteRepo  bool     `json:"writeRepo"`
}

// A deployed copy carries the deploy record first; a source index has none.
type ContextFile struct {
	Deploy     *ContextDeploy  `json:"deploy,omitempty"`
	Self       string          `json:"self,omitempty"`
	Entries    []RuleEntry     `json:"entries"`
	Tombstones []RuleTombstone `json:"tombstones"`
	Guides     []RuleGuide     `json:"guides"`
	Aspects    []RuleAspect    `json:"aspects"`
}

// RuleSet is the index as the walk and lane builders consume it; a deployed repo carries the index and no sources.
func (f *ContextFile) RuleSet() RuleSet {
	set := RuleSet{Entries: f.Entries, Guides: f.Guides, Self: f.Self, Tombstones: f.Tombstones}
	return set
}

// Render sorts aspects and guides by name and prints the index as cmd/rules generate does.
func (f *ContextFile) Render() ([]byte, error) {
	sort.Slice(f.Aspects, func(left, right int) bool {
		return f.Aspects[left].Name < f.Aspects[right].Name
	})
	sort.Slice(f.Guides, func(left, right int) bool {
		return f.Guides[left].Name < f.Guides[right].Name
	})
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("ContextFile.Render: %w", err)
	}
	return append(data, '\n'), nil
}

func DeployRecordPath(stateDir string, name string) string {
	return filepath.Join(stateDir, DeployRecordDir, name, ContextFileName)
}

// A missing or unreadable file is a source index: regeneration is the repair.
func IsDeployedIndex(path string) bool {
	index, err := LoadContextFile(path)
	if err != nil {
		return false
	}
	return index.Deploy != nil
}

// LoadContextFile reads one rendered context index.
func LoadContextFile(path string) (*ContextFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadContextFile: %w", err)
	}

	index := &ContextFile{}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, fmt.Errorf("LoadContextFile: Failed to decode %s: %w", path, err)
	}
	return index, nil
}

// Deployed indexes pass self "": they are already reach-filtered and carry no stamp.
func RenderContextJson(set RuleSet, aspects []RuleAspect, self string) ([]byte, error) {
	contextFile := ContextFile{
		Self:       self,
		Entries:    set.Entries,
		Tombstones: set.Tombstones,
		Guides:     set.Guides,
		Aspects:    aspects,
	}
	data, err := contextFile.Render()
	if err != nil {
		return nil, fmt.Errorf("RenderContextJson: %w", err)
	}
	return data, nil
}
