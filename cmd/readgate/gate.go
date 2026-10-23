package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/readledger"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/transcript"
)

// contextIndex is the slice of the context index the gate reads.
type contextIndex struct {
	Guides []contextdocs.RuleGuide `json:"guides"`
}

type Gate struct {
	claudeHome string
	hookConfig *hookconfig.Config
	input      *HookInput
	ledger     *readledger.Ledger
	off        []sessionstate.Feature
}

func NewGate(claudeHome string, hookConfig *hookconfig.Config, input *HookInput, ledger *readledger.Ledger, off []sessionstate.Feature) *Gate {
	gate := &Gate{
		claudeHome: claudeHome,
		hookConfig: hookConfig,
		input:      input,
		ledger:     ledger,
		off:        off,
	}
	return gate
}

func (g *Gate) Decide(ctx context.Context) (string, error) {
	if !g.isGated() || g.isSubagentUngated() {
		return "", nil
	}
	if g.input.isManifestWrite() {
		return g.hookConfig.Readgate.ManifestWriteReason, nil
	}

	manifest := loadManifest(g.input)
	if reason := manifest.headMismatch(ctx, g.hookConfig.Readgate.HeadMismatchText); reason != "" {
		return reason, nil
	}

	records, err := g.ledger.Load()
	if err != nil {
		return "", fmt.Errorf("Gate.Decide: %w", err)
	}

	required := g.required(manifest, records)
	if g.input.isReadingAny(required) || g.isReadingGuide() {
		return "", nil
	}

	missing := &strings.Builder{}
	for _, path := range required {
		g.writeUnread(missing, path, records)
	}

	g.writeWalkGaps(manifest.unansweredPairs(g.input), missing)
	if missing.Len() > 0 {
		if err := g.recordPendingGuides(records); err != nil {
			return "", fmt.Errorf("Gate.Decide: %w", err)
		}
		return g.denyReason(missing.String()), nil
	}

	if err := manifest.writeCoverage(g.input.ToolInput.FilePath); err != nil {
		return "", fmt.Errorf("Gate.Decide: %w", err)
	}
	return "", nil
}

func (g *Gate) ancestorBase(path string) string {
	if !filepath.IsAbs(path) {
		return ""
	}

	for dir := filepath.Dir(path); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		base := filepath.Join(dir, g.hookConfig.Context.ContextDir)
		if isFile(filepath.Join(base, contextdocs.ContextFileName)) {
			return base
		}
	}
	return ""
}

func (g *Gate) contextBase(path string) string {
	return cmp.Or(g.ancestorBase(path), filepath.Join(g.input.Cwd, g.hookConfig.Context.ContextDir))
}

func (g *Gate) denyReason(missing string) string {
	section := &g.hookConfig.Readgate
	reason := fmt.Sprintf(section.DenyFormat, g.input.ToolName, missing)
	if g.input.ToolName == toolBash {
		return reason + section.BashAddendum
	}
	return reason
}

func (g *Gate) guidesFor(path string) []string {
	base := g.contextBase(path)
	guides, err := loadGuides(filepath.Join(base, contextdocs.ContextFileName))
	if err != nil {
		return nil
	}

	relative := strings.TrimPrefix(path, filepath.Dir(base)+string(filepath.Separator))
	paths := make([]string, 0)
	for _, guide := range guides {
		if !matchesAny(guide.Files, path, relative) {
			continue
		}

		guidePath := filepath.Join(base, guide.Path)
		if !isFile(guidePath) {
			continue
		}

		paths = append(paths, guidePath)
	}
	return paths
}

// isOwed: the session has not switched the feature off.
func (g *Gate) isOwed(feature sessionstate.Feature) bool {
	return !slices.Contains(g.off, feature)
}

func (g *Gate) isGated() bool {
	switch g.hookConfig.Toolpolicy.ClassOf(g.input.ToolName) {
	case hookconfig.ClassBash, hookconfig.ClassRead, hookconfig.ClassWrite:
		return true
	case hookconfig.ClassRuleBearing, hookconfig.ClassUnknown:
		return false
	}
	return false
}

func (g *Gate) isReadingGuide() bool {
	if g.input.ToolName != toolRead {
		return false
	}

	target := g.input.ToolInput.FilePath
	base := g.contextBase(target)
	guides, err := loadGuides(filepath.Join(base, contextdocs.ContextFileName))
	if err != nil {
		return false
	}

	isTarget := func(guide contextdocs.RuleGuide) bool { return filepath.Join(base, guide.Path) == target }
	return slices.ContainsFunc(guides, isTarget)
}

func (g *Gate) recordPendingGuides(records []readledger.Record) error {
	pending := readledger.PendingGuides(records)
	for _, guide := range g.touchedGuides() {
		isRecorded := slices.Contains(pending, guide)
		if isRecorded || len(readledger.UnreadRanges(guide, records)) == 0 {
			continue
		}

		if err := g.ledger.RecordGuide(guide); err != nil {
			return fmt.Errorf("Gate.recordPendingGuides: %w", err)
		}

		pending = append(pending, guide)
	}
	return nil
}

func (g *Gate) required(manifest *Manifest, records []readledger.Record) []string {
	pending := slices.DeleteFunc(readledger.PendingGuides(records), isMissing)
	paths := slices.Concat(
		g.touchedGuides(),
		slices.DeleteFunc(pending, g.isPendingGuideOff),
		manifest.existingFiles(),
		g.sessionRequired(records),
	)
	slices.Sort(paths)
	return slices.Compact(paths)
}

// sessionRequired: the session's own files, each behind its rgate member.
func (g *Gate) sessionRequired(records []readledger.Record) []string {
	dir := contextrequired.Dir(g.claudeHome, g.input.SessionId)
	agent := cmp.Or(g.input.AgentId, readledger.MainAgent)
	owed := make([]string, 0)
	if g.isOwed(sessionstate.RgateInvoked) {
		owed = append(owed, invokedInjections(agent, g.hookConfig.Readgate.DefaultSliceSuffix, dir, records)...)
	}
	if g.isOwed(sessionstate.RgateGlobal) {
		owed = append(owed, filepath.Join(dir, contextrequired.GlobalFile))
	}
	if g.input.AgentId == "" && g.isOwed(sessionstate.RgatePlan) {
		owed = append(owed, filepath.Join(dir, contextrequired.PlanModeFile))
	}
	return slices.DeleteFunc(owed, isMissing)
}

// isGuideDisabled: the guide's file name is in the config's disabled list.
func (g *Gate) isGuideDisabled(guidePath string) bool {
	return slices.Contains(g.hookConfig.Readgate.DisabledGuides, filepath.Base(guidePath))
}

// isGuideOff: the guide is dropped by the config or by its session name member.
func (g *Gate) isGuideOff(guidePath string) bool {
	return g.isGuideDisabled(guidePath) || g.isGuideNameOff(guidePath)
}

// isPendingGuideOff: a guide an earlier denial recorded is dropped by its name member, or once every extension it covers is off.
func (g *Gate) isPendingGuideOff(guidePath string) bool {
	if g.isGuideOff(guidePath) {
		return true
	}

	base := g.ancestorBase(guidePath)
	guides, err := loadGuides(filepath.Join(base, contextdocs.ContextFileName))
	if err != nil {
		return false
	}

	for _, guide := range guides {
		if filepath.Join(base, guide.Path) != guidePath {
			continue
		}
		extensions := sessionstate.Extensions(guide.Files)
		isEveryOff := len(extensions) > 0 && !slices.ContainsFunc(extensions, g.isWordOwed)
		return isEveryOff
	}
	return false
}

// isGuideNameOff: the guide's own member, its file name without the extension, is switched off.
func (g *Gate) isGuideNameOff(guidePath string) bool {
	return !g.isWordOwed(sessionstate.GuideStem(guidePath))
}

// isSubagentUngated: the config gates the root agent alone and this call is a subagent's.
func (g *Gate) isSubagentUngated() bool {
	return g.input.AgentId != "" && !g.hookConfig.Readgate.GateSubagents
}

// isTouchedOff: the touched file's extension member is switched off, so no guide is owed for it.
func (g *Gate) isTouchedOff(path string) bool {
	return !g.isWordOwed(strings.TrimPrefix(filepath.Ext(path), "."))
}

func (g *Gate) isWordOwed(word string) bool {
	return g.isOwed(sessionstate.Ext(word))
}

func (g *Gate) touchedGuides() []string {
	guides := make([]string, 0)
	for _, touched := range g.input.touchedPaths() {
		if g.isTouchedOff(touched) {
			continue
		}
		guides = append(guides, g.guidesFor(touched)...)
	}
	return slices.DeleteFunc(guides, g.isGuideOff)
}

func (g *Gate) writeUnread(missing *strings.Builder, path string, records []readledger.Record) {
	gaps := readledger.UnreadRanges(path, records)
	if len(gaps) == 0 {
		return
	}

	fmt.Fprintf(missing, "\n- %s — unread:", path)
	for _, gap := range gaps {
		for _, instruction := range contextrequired.ReadInstructions(gap.End, path, gap.Start, g.hookConfig.Globalcontext.ReadBudget) {
			fmt.Fprintf(missing, "\n    %s", instruction)
		}
	}
}

func (g *Gate) writeWalkGaps(gaps []walkEntry, missing *strings.Builder) {
	if len(gaps) == 0 {
		return
	}

	fmt.Fprintf(missing, "\n- %s — walk incomplete, unanswered (file × group; file empty on group-first):", g.input.ToolInput.FilePath)
	for _, gap := range gaps {
		fmt.Fprintf(missing, "\n    %s × %s", gap.displayFile(), gap.Group)
	}
}

func bashTokens(command string) []string {
	words := strings.FieldsFunc(command, isShellSeparator)
	for i, word := range words {
		words[i] = strings.Trim(word, `'"`)
	}
	return slices.DeleteFunc(words, isNotPathLike)
}

func injectionFor(agent string, args string, defaultSliceSuffix string, dir string, skill string) string {
	candidates := []string{
		filepath.Join(dir, skill+injectionSeparator+sliceSuffix(args, defaultSliceSuffix)+injectionSeparator+agent+injectionExtension),
		filepath.Join(dir, skill+injectionSeparator+agent+injectionExtension),
	}
	index := slices.IndexFunc(candidates, isFile)
	if index < 0 {
		return ""
	}
	return candidates[index]
}

func invokedInjections(agent string, defaultSliceSuffix string, dir string, records []readledger.Record) []string {
	injections := make([]string, 0)
	for _, record := range records {
		if record.Kind == readledger.KindSkill {
			injections = append(injections, injectionFor(agent, record.Args, defaultSliceSuffix, dir, record.Skill))
		}
	}
	return injections
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isMissing(path string) bool {
	return !isFile(path)
}

func isNotPathLike(word string) bool {
	return strings.HasPrefix(word, "-") || !strings.Contains(word, ".")
}

func isShellSeparator(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(bashMetacharacters, r)
}

func loadGuides(path string) ([]contextdocs.RuleGuide, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadGuides: Failed to read %s: %w", path, err)
	}

	index := &contextIndex{}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, fmt.Errorf("loadGuides: Failed to decode %s: %w", path, err)
	}
	return index.Guides, nil
}

func matchesAny(globs []string, path string, relative string) bool {
	isMatch := func(glob string) bool {
		return contextdocs.GlobMatches(glob, path) || contextdocs.GlobMatches(glob, relative)
	}
	return slices.ContainsFunc(globs, isMatch)
}

// sliceNames returns the tokens' names, sorted and deduplicated.
func sliceNames(tokens []string) []string {
	names := make([]string, 0, len(tokens))
	for _, token := range tokens {
		_, name, _ := strings.Cut(token, ":")
		names = append(names, name)
	}

	slices.Sort(names)
	return slices.Compact(names)
}

// sliceSuffix mirrors cmd/skillcontext's filePath slice naming — change both together.
func sliceSuffix(args string, defaultSliceSuffix string) string {
	tokens := transcript.SliceTokens(args)
	if len(tokens) == 0 {
		return defaultSliceSuffix
	}

	kind, _, _ := strings.Cut(tokens[0], ":")
	return kind + "-" + strings.Join(sliceNames(tokens), "+")
}
