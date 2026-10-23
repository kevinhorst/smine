package skillimport

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// EntryRoles is the closed effect-role set: what provably changes when the entry is removed.
var EntryRoles = map[string]bool{
	skillgraph.RoleDef:     true,
	skillgraph.RolePayload: true,
	skillgraph.RoleQuality: true,
	skillgraph.RoleRule:    true,
	skillgraph.RoleSpawn:   true,
	skillgraph.RoleStep:    true,
	skillgraph.RoleStop:    true,
	skillgraph.RoleSubstep: true,
	skillgraph.RoleVerify:  true,
}

// EntryChannels is the closed enforcement-channel set: what proves adherence; payload takes none.
var EntryChannels = map[string]bool{
	"gate":   true,
	"hook":   true,
	"prose":  true,
	"schema": true,
	"script": true,
}

// Entry caps: format 1 keeps 320-rune statements and uncapped bullets, format 2 tightens to 200/160.
const (
	EntryFormatStrict        = 2
	EntryStatementMaxRunes   = 320
	EntryStatementMaxRunesV2 = 200
	EntryBulletMaxRunesV2    = 160
	EntryLineMaxChars        = 128
	EntryTitleMaxWords       = 3
)

var (
	// EntryPattern's group 6 captures text after the title: empty in grammar v2, a legacy inline statement otherwise.
	EntryPattern       = regexp.MustCompile(`^\*\*SKILL-([A-Z0-9]{2,24})-([A-Z0-9]{2,12})-([0-9]{3}[a-z]?)\*\* \x60((?:\[[a-z][a-z0-9:-]*\])+)\x60 \*\*([^*]+)\*\*[ \t]*(.*)$`)
	entryBulletPattern = regexp.MustCompile(`^\* (Why|Applies|Hook|Gate|Schema|Script): (.+)$`)
	// clause lines end in a backslash, the markdown hard break that keeps pred, post and headline apart
	entryPredPattern = regexp.MustCompile("^\x60\\{pred: ([^}]+)\\}\x60\\\\?$")
	entryPostPattern = regexp.MustCompile("^\x60\\{post: ([^}]+)\\}\x60\\\\?$")
	// legacyCombinedPattern is the retired one-line `{pred: …}{post: …}` form.
	legacyCombinedPattern = regexp.MustCompile("^\x60\\{pred: [^}]+\\}\\{post: [^}]+\\}\x60\\\\?$")
	legacyClausePattern   = regexp.MustCompile(`^\* (pred|post): `)
	// sectionClausePattern's second anchor group exists so a line carrying both anchors is reported, not dropped.
	sectionClausePattern = regexp.MustCompile("^\x60\\{section: ([a-z]+)(?:, pred: ([^,}]+))?(?:, (replaces|between): ([^,}]+))?(?:, (replaces|between): ([^,}]+))?\\}\x60$")
	// anchorIdPattern is one short entry id, the skill prefix implied.
	anchorIdPattern    = regexp.MustCompile(`^[A-Z0-9]{2,12}-[0-9]{3}[a-z]?$`)
	anchorRangePattern = regexp.MustCompile(`^([A-Za-z0-9-]+)\.\.([A-Za-z0-9-]+)$`)
	entryLinkPattern   = regexp.MustCompile(`\]\(#(skill-[a-z0-9-]+-[0-9]{3}[a-z]?)\)`)
	entryPrefixPattern = regexp.MustCompile(`^\*\*SKILL-`)
	entryTagPattern    = regexp.MustCompile(`\[([a-z][a-z0-9:-]*)\]`)
	formatKeyPattern   = regexp.MustCompile(`^format:\s*([0-9]+)\s*$`)
	// markdownLinkPattern strips link targets when measuring display length.
	markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

// splitSkillTags is best effort; validateSkillTagSlot reports the precise violation.
func splitSkillTags(tags []string) (role, channel string, custom []string) {
	for _, tag := range tags {
		switch {
		case strings.Contains(tag, ":"):
			custom = append(custom, tag)
		case EntryRoles[tag] && role == "":
			role = tag
		case EntryChannels[tag] && channel == "":
			channel = tag
		}
	}
	if role != "" && role != "payload" && channel == "" {
		channel = "prose"
	}
	return role, channel, custom
}

// Entry's Line/End are 0-based [Line, End) and bound the block a renderer strips when the entry is disabled.
type Entry struct {
	Id      string   `json:"id"`
	Applies string   `json:"applies,omitempty"`
	Channel string   `json:"channel,omitempty"`
	Class   string   `json:"class"`
	Custom  []string `json:"custom,omitempty"`
	End     int      `json:"-"`
	Gate    string   `json:"gate,omitempty"`
	Hook    string   `json:"hook,omitempty"`
	Line    int      `json:"line"`
	Number  int      `json:"number"`
	Params  []string `json:"params,omitempty"`
	Payload bool     `json:"payload"`
	Post    string   `json:"post,omitempty"`
	Pred    string   `json:"pred,omitempty"`
	Refs    []string `json:"refs,omitempty"`
	Role    string   `json:"role,omitempty"`
	Schema  string   `json:"schema,omitempty"`
	Script  string   `json:"script,omitempty"`
	Section string   `json:"section,omitempty"`
	// SectionBetween and SectionReplaces hold anchors resolved to full ids; a range reads FIRST..LAST.
	SectionBetween  []string `json:"sectionBetween,omitempty"`
	SectionKind     string   `json:"sectionKind,omitempty"`
	SectionPred     string   `json:"sectionPred,omitempty"`
	SectionReplaces []string `json:"sectionReplaces,omitempty"`
	Skill           string   `json:"skill"`
	Subsection      string   `json:"subsection,omitempty"`
	Statement       string   `json:"statement"`
	Suffix          string   `json:"suffix,omitempty"`
	Title           string   `json:"title"`
	Topic           string   `json:"topic"`
	Why             string   `json:"why,omitempty"`
}

// ParseEntries closes a fence only on its own backtick run, so a four-backtick payload may carry three-backtick fences.
func ParseEntries(lines []string) ([]Entry, []string) {
	var entries []Entry
	var violations []string
	format := 1
	openFence := ""
	section := ""
	subsection := ""
	clause := sectionClause{}
	inFrontmatter := isFrontmatterStart(lines)
	for index, line := range lines {
		if inFrontmatter {
			if match := formatKeyPattern.FindStringSubmatch(line); match != nil {
				parsed, _ := strconv.Atoi(match[1])
				format = parsed
			}
			inFrontmatter = !isFrontmatterEnd(index, line)
			continue
		}
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			continue
		}
		if openFence != "" {
			continue
		}
		if heading, isHeading := strings.CutPrefix(line, "## "); isHeading {
			section = strings.TrimSpace(heading)
			subsection = ""
			clause = sectionClause{}
			if index+1 < len(lines) {
				parsed, kindViolations := parseSectionClause(lines[index+1], index+1)
				violations = append(violations, kindViolations...)
				clause = parsed
			}
		}
		if heading, isHeading := strings.CutPrefix(line, "### "); isHeading {
			subsection = strings.TrimSpace(heading)
		}
		if violation := checkLineWidth(line, index); violation != "" {
			violations = append(violations, violation)
		}
		violations = append(violations, checkClauseLinePlacement(lines, index)...)
		entry, entryViolations, isEntry := parseEntryAt(lines, index, format)
		violations = append(violations, entryViolations...)
		if isEntry {
			entry.Section = section
			entry.SectionKind = clause.kind
			entry.SectionPred = clause.pred
			entry.SectionBetween = resolveAnchors(clause.between, entry.Skill)
			entry.SectionReplaces = resolveAnchors(clause.replaces, entry.Skill)
			entry.Subsection = subsection
			entries = append(entries, entry)
		}
	}
	violations = append(violations, checkSectionOrder(lines, entries, format)...)
	violations = append(violations, checkRouteAnchors(entries)...)
	return entries, violations
}

func checkClauseLinePlacement(lines []string, index int) []string {
	line := lines[index]
	if legacyCombinedPattern.MatchString(line) {
		return []string{fmt.Sprintf("%d: combined {pred}{post} line — pred and post each live on their own line above the headline", index+1)}
	}
	next := ""
	if index+1 < len(lines) {
		next = lines[index+1]
	}
	if entryPostPattern.MatchString(line) && !entryPrefixPattern.MatchString(next) {
		return []string{fmt.Sprintf("%d: post clause line is not directly above an entry headline", index+1)}
	}
	if entryPredPattern.MatchString(line) {
		isAttached := entryPrefixPattern.MatchString(next) ||
			(entryPostPattern.MatchString(next) && index+2 < len(lines) && entryPrefixPattern.MatchString(lines[index+2]))
		if !isAttached {
			return []string{fmt.Sprintf("%d: pred clause line is not directly above an entry headline", index+1)}
		}
	}
	if (entryPredPattern.MatchString(line) || entryPostPattern.MatchString(line)) && !strings.HasSuffix(line, "\\") {
		return []string{fmt.Sprintf("%d: clause line misses the trailing backslash hard break — without it the title renders on the same line", index+1)}
	}
	if sectionClausePattern.MatchString(line) {
		isUnderHeading := index > 0 && strings.HasPrefix(lines[index-1], "## ")
		if !isUnderHeading {
			return []string{fmt.Sprintf("%d: section clause line is not directly under a section heading", index+1)}
		}
	}
	return nil
}

// checkSectionOrder skips gapped scopes: they are legacy state migrated deliberately.
func checkSectionOrder(lines []string, entries []Entry, format int) []string {
	if format < EntryFormatStrict {
		return nil
	}
	gapped := gappedScopes(lines, entries)
	var violations []string
	scope := ""
	previousRank := 0
	previousId := ""
	for _, entry := range entries {
		entryScope := entry.Section + "\x00" + entry.Subsection
		if entryScope != scope {
			scope = entryScope
			previousRank = 0
			previousId = ""
		}
		if gapped[scope] {
			continue
		}
		if !EntryRoles[entry.Role] {
			continue
		}
		rank := skillgraph.RoleOrderRank(entry.Role)
		if rank < previousRank {
			violations = append(violations, fmt.Sprintf("%s: role %q after %s breaks the section order def, rule, quality, flow, payload", entry.Id, entry.Role, previousId))
		}
		if rank >= previousRank {
			previousRank, previousId = rank, entry.Id
		}
	}
	return violations
}

func gappedScopes(lines []string, entries []Entry) map[string]bool {
	gapped := map[string]bool{}
	for index := 1; index < len(entries); index++ {
		previous, current := entries[index-1], entries[index]
		scope := current.Section + "\x00" + current.Subsection
		if previous.Section != current.Section || previous.Subsection != current.Subsection {
			continue
		}
		for gap := previous.End; gap < current.Line && gap < len(lines); gap++ {
			if strings.TrimSpace(lines[gap]) != "" {
				gapped[scope] = true
				break
			}
		}
	}
	return gapped
}

const BetweenAnchorCount = 2

// Anchor clause names and the chain-end tokens a between clause may use.
const (
	anchorBetween  = "between"
	anchorEnd      = "end"
	anchorReplaces = "replaces"
	anchorStart    = "start"
)

// sectionClause holds anchors as written, short ids.
type sectionClause struct {
	between  []string
	kind     string
	pred     string
	replaces []string
}

func anchorCountViolations(anchors, index int, kind string) []string {
	isRoute := kind == skillgraph.SectionKindRoute
	switch {
	case isRoute && anchors == 0:
		return []string{fmt.Sprintf("%d: route section without anchor — declare replaces: <items> or between: <after>..<before>", index+1)}
	case isRoute && anchors > 1:
		return []string{fmt.Sprintf("%d: route section with both replaces and between — exactly one anchor", index+1)}
	case !isRoute && anchors > 0:
		return []string{fmt.Sprintf("%d: anchor clause on kind %q — only route sections carry one", index+1, kind)}
	}
	return nil
}

func checkAnchorEntry(byId map[string]Entry, id, route string) []string {
	anchor, ok := byId[id]
	switch {
	case !ok:
		return []string{fmt.Sprintf("section %q: anchor %s — no such entry", route, id)}
	case anchor.Role == skillgraph.RoleDef || anchor.Role == skillgraph.RolePayload:
		return []string{fmt.Sprintf("section %q: anchor %s — a %s is never an anchor", route, id, anchor.Role)}
	case anchor.Section == route:
		return []string{fmt.Sprintf("section %q: anchor %s — inside the route itself", route, id)}
	}
	return nil
}

// checkBetween: routes never loop back, so the reentry comes after the split.
func checkBetween(byId map[string]Entry, pair []string, route string) []string {
	var violations []string
	for _, id := range pair {
		if id != anchorStart && id != anchorEnd {
			violations = append(violations, checkAnchorEntry(byId, id, route)...)
		}
	}
	after, before := byId[pair[0]], byId[pair[1]]
	isBounded := pair[0] == anchorStart || pair[1] == anchorEnd
	if len(violations) == 0 && !isBounded && before.Line <= after.Line {
		violations = append(violations, fmt.Sprintf("section %q: between %s..%s — reentry at or before the split", route, pair[0], pair[1]))
	}
	return violations
}

// checkReplacesItem lets a range span sections (an early exit) but requires document order.
func checkReplacesItem(byId map[string]Entry, item, route string) []string {
	firstId, lastId, isRange := strings.Cut(item, "..")
	if !isRange {
		lastId = firstId
	}
	violations := checkAnchorEntry(byId, firstId, route)
	if isRange {
		violations = append(violations, checkAnchorEntry(byId, lastId, route)...)
	}
	first, last := byId[firstId], byId[lastId]
	if len(violations) == 0 && first.Line > last.Line {
		violations = append(violations, fmt.Sprintf("section %q: replaces %s — range out of order", route, item))
	}
	return violations
}

func checkRouteAnchors(entries []Entry) []string {
	byId := map[string]Entry{}
	for _, entry := range entries {
		byId[entry.Id] = entry
	}
	seen := map[string]bool{}
	var violations []string
	for _, entry := range entries {
		if seen[entry.Section] || entry.SectionKind != skillgraph.SectionKindRoute {
			continue
		}
		seen[entry.Section] = true
		for _, item := range entry.SectionReplaces {
			violations = append(violations, checkReplacesItem(byId, item, entry.Section)...)
		}
		if len(entry.SectionBetween) == BetweenAnchorCount {
			violations = append(violations, checkBetween(byId, entry.SectionBetween, entry.Section)...)
		}
	}
	return violations
}

func isReplacesItem(item string) bool {
	if anchorIdPattern.MatchString(item) {
		return true
	}
	match := anchorRangePattern.FindStringSubmatch(item)
	return match != nil && anchorIdPattern.MatchString(match[1]) && anchorIdPattern.MatchString(match[2])
}

func parseAnchorItems(index int, name, text string) ([]string, []string) {
	if name == anchorBetween {
		return parseBetween(index, text)
	}
	var items []string
	var violations []string
	for item := range strings.SplitSeq(text, " + ") {
		if !isReplacesItem(item) {
			violations = append(violations, fmt.Sprintf("%d: malformed replaces item %q — short id or A..B range", index+1, item))
			continue
		}
		items = append(items, item)
	}
	return items, violations
}

// parseBetween reads the after..before pair of a between clause.
func parseBetween(index int, text string) ([]string, []string) {
	match := anchorRangePattern.FindStringSubmatch(text)
	if match == nil {
		return nil, []string{fmt.Sprintf("%d: malformed between %q — <after>..<before>, start and end allowed", index+1, text)}
	}
	isAfter := match[1] == anchorStart || anchorIdPattern.MatchString(match[1])
	isBefore := match[2] == anchorEnd || anchorIdPattern.MatchString(match[2])
	if !isAfter || !isBefore {
		return nil, []string{fmt.Sprintf("%d: malformed between %q — <after>..<before>, start and end allowed", index+1, text)}
	}
	pair := []string{match[1], match[2]}
	return pair, nil
}

// parseSectionClause treats an absent line as the default phase kind.
func parseSectionClause(line string, index int) (sectionClause, []string) {
	match := sectionClausePattern.FindStringSubmatch(line)
	if match == nil {
		if strings.HasPrefix(line, "`{section:") {
			return sectionClause{}, []string{fmt.Sprintf("%d: malformed section clause line", index+1)}
		}
		return sectionClause{}, nil
	}
	clause := sectionClause{kind: match[1], pred: strings.TrimSpace(match[2])}
	switch clause.kind {
	case skillgraph.SectionKindContext, skillgraph.SectionKindPhase, skillgraph.SectionKindRoute:
	default:
		return sectionClause{}, []string{fmt.Sprintf("%d: unknown section kind %q", index+1, clause.kind)}
	}
	var violations []string
	if clause.pred != "" && clause.kind != skillgraph.SectionKindRoute {
		violations = append(violations, fmt.Sprintf("%d: section pred on kind %q — only route sections carry a pred", index+1, clause.kind))
	}
	anchors := 0
	for _, group := range [][]string{match[3:5], match[5:7]} {
		if group[0] == "" {
			continue
		}
		anchors++
		items, itemViolations := parseAnchorItems(index, group[0], strings.TrimSpace(group[1]))
		violations = append(violations, itemViolations...)
		if group[0] == anchorReplaces {
			clause.replaces = items
			continue
		}
		clause.between = items
	}
	violations = append(violations, anchorCountViolations(anchors, index, clause.kind)...)
	return clause, violations
}

func resolveAnchors(items []string, skill string) []string {
	if len(items) == 0 {
		return nil
	}
	prefix := "SKILL-" + skill + "-"
	resolved := make([]string, 0, len(items))
	for _, item := range items {
		first, last, isRange := strings.Cut(item, "..")
		full := resolveAnchorId(first, prefix)
		if isRange {
			full += ".." + resolveAnchorId(last, prefix)
		}
		resolved = append(resolved, full)
	}
	return resolved
}

// resolveAnchorId prefixes one short id; the chain-end tokens pass through.
func resolveAnchorId(id, prefix string) string {
	if id == anchorStart || id == anchorEnd {
		return id
	}
	return prefix + id
}

// checkLineWidth exempts table rows: markdown tables cannot wrap.
func checkLineWidth(line string, index int) string {
	if strings.HasPrefix(strings.TrimSpace(line), "|") {
		return ""
	}
	if len([]rune(line)) <= EntryLineMaxChars {
		return ""
	}
	return fmt.Sprintf("%d: line exceeds %d chars — wrap it", index+1, EntryLineMaxChars)
}

func entryBlockEnd(lines []string, from int) int {
	for index := from; index < len(lines); index++ {
		if isBulletContinuation(lines, index) {
			continue
		}
		return index
	}
	return len(lines)
}

func isStatementLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(line, "#") || fenceRun(line) != "" || entryPrefixPattern.MatchString(line) {
		return false
	}
	return true
}

func statementSpan(lines []string, index int) (start, end int, blankSeen bool) {
	start = index + 1
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
		blankSeen = true
	}
	end = start
	for end < len(lines) && isStatementLine(lines[end]) {
		end++
	}
	return start, end, blankSeen
}

// logicalBullet is one bullet joined over its indented continuation lines.
type logicalBullet struct {
	line int
	text string
}

// joinBullets treats fenced spans in the range (woven example blocks) as content, never bullets.
func joinBullets(lines []string, from, end int) []logicalBullet {
	var bullets []logicalBullet
	openFence := ""
	for index := from; index < end; index++ {
		line := lines[index]
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			continue
		}
		if openFence != "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "* "):
			bullet := logicalBullet{line: index, text: line}
			bullets = append(bullets, bullet)
		case strings.TrimSpace(line) != "" && len(bullets) > 0 && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")):
			bullets[len(bullets)-1].text += " " + strings.TrimSpace(line)
		}
	}
	return bullets
}

func entryBullets(entry *Entry, bullets []logicalBullet) {
	for _, bullet := range bullets {
		if text, ok := strings.CutPrefix(bullet.text, "* param: "); ok {
			entry.Params = append(entry.Params, strings.TrimSpace(text))
			continue
		}
		match := entryBulletPattern.FindStringSubmatch(bullet.text)
		if match == nil {
			continue
		}
		switch match[1] {
		case "Why":
			entry.Why = strings.TrimSpace(match[2])
		case "Applies":
			entry.Applies = strings.TrimSpace(match[2])
		case "Hook":
			entry.Hook = strings.TrimSpace(match[2])
		case "Gate":
			entry.Gate = strings.TrimSpace(match[2])
		case "Schema":
			entry.Schema = strings.TrimSpace(match[2])
		case "Script":
			entry.Script = strings.TrimSpace(match[2])
		}
	}
}

func validateSkillTagSlot(tags []string) string {
	seenRole, seenChannel, seenCustom, role := false, false, false, ""
	for _, tag := range tags {
		switch {
		case strings.Contains(tag, ":"):
			seenCustom = true
			if strings.HasSuffix(tag, ":") {
				return fmt.Sprintf("custom tag [%s] has no value after the colon", tag)
			}
		case EntryRoles[tag]:
			if seenCustom {
				return fmt.Sprintf("role tag [%s] must come before any custom tag", tag)
			}
			if seenRole {
				return fmt.Sprintf("two role tags in [%s]", strings.Join(tags, "]["))
			}
			if seenChannel {
				return fmt.Sprintf("role tag [%s] must come before the channel tag", tag)
			}
			seenRole, role = true, tag
		case EntryChannels[tag]:
			if seenCustom {
				return fmt.Sprintf("channel tag [%s] must come before any custom tag", tag)
			}
			if seenChannel {
				return fmt.Sprintf("two channel tags in [%s]", strings.Join(tags, "]["))
			}
			seenChannel = true
		default:
			return fmt.Sprintf("unknown tag %q", tag)
		}
	}
	if role == "payload" && seenChannel {
		return "payload takes no channel tag"
	}
	if !seenRole && !seenChannel {
		return "tag slot requires at least one role or channel tag"
	}
	return ""
}

// formalClauses expects the post line directly above the headline, the pred line above it.
func formalClauses(lines []string, index int) (pred, post string, clauseLines int) {
	scan := index
	if scan > 0 {
		if match := entryPostPattern.FindStringSubmatch(lines[scan-1]); match != nil {
			post = strings.TrimSpace(match[1])
			scan--
		}
	}
	if scan > 0 {
		if match := entryPredPattern.FindStringSubmatch(lines[scan-1]); match != nil {
			pred = strings.TrimSpace(match[1])
			scan--
		}
	}
	return pred, post, index - scan
}

func fenceRun(line string) string {
	if !strings.HasPrefix(line, "```") {
		return ""
	}
	end := 0
	for end < len(line) && line[end] == '`' {
		end++
	}
	return line[:end]
}

func isBulletContinuation(lines []string, index int) bool {
	line := lines[index]
	if strings.HasPrefix(line, "* ") {
		return true
	}
	if strings.TrimSpace(line) != "" {
		return strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")
	}
	hasNext := index+1 < len(lines)
	return hasNext && strings.HasPrefix(lines[index+1], "* ")
}

func isFrontmatterEnd(index int, line string) bool {
	return index > 0 && strings.TrimSpace(line) == "---"
}

func isFrontmatterStart(lines []string) bool {
	return len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
}

func parseEntryAt(lines []string, index, format int) (Entry, []string, bool) {
	line := lines[index]
	match := EntryPattern.FindStringSubmatch(line)
	if match == nil {
		if entryPrefixPattern.MatchString(line) {
			return Entry{}, []string{fmt.Sprintf("%d: malformed entry line: %s", index+1, line)}, false
		}
		return Entry{}, nil, false
	}
	digits := strings.TrimRight(match[3], "abcdefghijklmnopqrstuvwxyz")
	number, _ := strconv.Atoi(digits)
	var rawTags []string
	for _, tag := range entryTagPattern.FindAllStringSubmatch(match[4], -1) {
		rawTags = append(rawTags, tag[1])
	}
	role, channel, custom := splitSkillTags(rawTags)
	pred, post, clauseLines := formalClauses(lines, index)
	startLine := index - clauseLines
	entry := Entry{
		Id:      fmt.Sprintf("SKILL-%s-%s-%s", match[1], match[2], match[3]),
		Channel: channel,
		Class:   strings.Join(rawTags, "+"),
		Custom:  custom,
		Line:    startLine,
		Number:  number,
		Payload: role == "payload",
		Post:    post,
		Pred:    pred,
		Role:    role,
		Skill:   match[1],
		Suffix:  match[3][len(digits):],
		Title:   strings.TrimSpace(match[5]),
		Topic:   match[2],
	}
	var violations []string
	trailing := strings.TrimSpace(match[6])
	start, end, blankSeen := statementSpan(lines, index)
	if trailing != "" {
		// a legacy dash or inline statement is the statement when nothing follows on later lines
		violations = append(violations, fmt.Sprintf("%d: %s: text after the title on the headline line — grammar v2 ends the headline at the title, statement on the next line after a blank", index+1, entry.Id))
		inline := strings.TrimSpace(strings.TrimPrefix(trailing, "—"))
		if inline != "" && start == end {
			entry.Statement = inline
		}
	}
	if entry.Statement == "" {
		var parts []string
		for _, statementLine := range lines[start:end] {
			parts = append(parts, strings.TrimSpace(statementLine))
		}
		entry.Statement = strings.Join(parts, " ")
		if trailing == "" && end > start && !blankSeen {
			violations = append(violations, fmt.Sprintf("%d: %s: a blank line separates the headline from the statement — grammar v2", index+1, entry.Id))
		}
	}
	// a non-payload entry also owns fences woven between its statement and bullets
	if entry.Payload {
		entry.End = entryBlockEnd(lines, end)
	} else {
		cursor := end
		for {
			next := entryBlockEnd(lines, cursor)
			if fenceEnd, hasFence := payloadFenceEnd(lines, next); hasFence {
				cursor = fenceEnd
				continue
			}
			cursor = next
			break
		}
		entry.End = cursor
	}
	if violation := validateSkillTagSlot(rawTags); violation != "" {
		violations = append(violations, fmt.Sprintf("%d: %s", index+1, violation))
	}
	if len(strings.Fields(entry.Title)) > EntryTitleMaxWords {
		violations = append(violations, fmt.Sprintf("%d: title %q of %s exceeds %d words", index+1, entry.Title, entry.Id, EntryTitleMaxWords))
	}
	statementCap := EntryStatementMaxRunes
	if format >= EntryFormatStrict {
		statementCap = EntryStatementMaxRunesV2
	}
	// link targets carry no reading cost, so they never push a statement over the cap
	if displayRunes(entry.Statement) > statementCap {
		violations = append(violations, fmt.Sprintf("%d: statement of %s exceeds %d runes — split into %sa/%sb", index+1, entry.Id, statementCap, entry.Id, entry.Id))
	}
	bullets := joinBullets(lines, end, entry.End)
	entryBullets(&entry, bullets)
	for _, bullet := range bullets {
		if legacyClausePattern.MatchString(bullet.text) {
			violations = append(violations, fmt.Sprintf("%d: %s: clause bullet — formal clauses live on their own lines directly above the headline", bullet.line+1, entry.Id))
		}
	}
	if (entry.Pred != "" || entry.Post != "") && !skillgraph.CoverableRole(entry.Role) {
		violations = append(violations, fmt.Sprintf("%d: %s: formal clause on role %q — clauses belong to rule/quality/step/spawn/substep/verify/stop entries", index+1, entry.Id, entry.Role))
	}
	if entry.Post != "" && (entry.Role == skillgraph.RoleRule || entry.Role == skillgraph.RoleQuality) {
		violations = append(violations, fmt.Sprintf("%d: %s: post clause on role %q — rule and quality entries carry pred only", index+1, entry.Id, entry.Role))
	}
	if format >= EntryFormatStrict {
		for _, bullet := range bullets {
			if strings.HasPrefix(bullet.text, "* Why:") {
				violations = append(violations, fmt.Sprintf("%d: Why bullet on %s — rationale moves to the skill's README.md design notes", bullet.line+1, entry.Id))
			}
			if displayRunes(strings.TrimPrefix(bullet.text, "* ")) > EntryBulletMaxRunesV2 {
				violations = append(violations, fmt.Sprintf("%d: bullet of %s exceeds %d runes — one claim per bullet; sequences go to a fenced procedure block", bullet.line+1, entry.Id, EntryBulletMaxRunesV2))
			}
		}
	}
	if entry.Payload {
		fenceEnd, hasFence := payloadFenceEnd(lines, entry.End)
		if hasFence {
			entry.End = fenceEnd
		} else {
			violations = append(violations, fmt.Sprintf("%d: payload entry %s is not followed by a fenced block", index+1, entry.Id))
		}
	}
	entry.Refs = entryRefs(lines, index, entry.End)
	return entry, violations, true
}

func entryRefs(lines []string, from, to int) []string {
	var refs []string
	seen := map[string]bool{}
	openFence := ""
	for index := from; index < to && index < len(lines); index++ {
		line := lines[index]
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			continue
		}
		if openFence != "" {
			continue
		}
		for _, match := range entryLinkPattern.FindAllStringSubmatch(line, -1) {
			id := anchorToEntryId(match[1])
			if seen[id] {
				continue
			}
			seen[id] = true
			refs = append(refs, id)
		}
	}
	return refs
}

// anchorToEntryId keeps a trailing letter suffix lower-case.
func anchorToEntryId(anchor string) string {
	id := strings.ToUpper(anchor)
	last := anchor[len(anchor)-1]
	if last >= 'a' && last <= 'z' {
		id = id[:len(id)-1] + string(last)
	}
	return id
}

func displayRunes(text string) int {
	return len([]rune(markdownLinkPattern.ReplaceAllString(text, "$1")))
}

// payloadFenceEnd allows at most one blank line before the payload's fence.
func payloadFenceEnd(lines []string, from int) (int, bool) {
	open := from
	if open < len(lines) && strings.TrimSpace(lines[open]) == "" {
		open++
	}
	if open >= len(lines) {
		return 0, false
	}
	marker := fenceRun(lines[open])
	if marker == "" {
		return 0, false
	}
	for index := open + 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == marker {
			return index + 1, true
		}
	}
	return 0, false
}

// toggleFence keeps an inner three-backtick fence inside a four-backtick block as content.
func toggleFence(openFence, run, line string) string {
	if openFence == "" {
		return run
	}
	if strings.TrimSpace(line) == openFence {
		return ""
	}
	return openFence
}
