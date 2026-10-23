package skillgraph

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/semver"
)

// Entry size discipline, ported from the parser; format 1 is a skill without a "format: 2" key.
const (
	bulletMaxRunesStrict    = 160
	entryFormatFields       = 3
	entryFormatLegacy       = 1
	entryFormatStrict       = 2
	statementMaxRunes       = 320
	statementMaxRunesStrict = 200
)

// Frontmatter keys Check reads.
const (
	fieldAliases = "aliases"
	fieldFormat  = "format"
	fieldName    = "name"
	fieldVersion = "version"
)

// Entry id shape SKILL-<NAME>-<TOPIC>-<NNN>: split into prefix, name, rest.
const (
	entryIdNamePart = 1
	entryIdParts    = 3
)

// Bullet shape the format-2 caps measure.
const (
	bulletLabelWhy   = "Why"
	bulletMarkerStar = "*"
)

// violationHeadRunes is how much of an over-long line a violation quotes.
const violationHeadRunes = 40

var (
	// aliasPattern allows single hyphens only, so no alias reads as a variant dir (<leaf>--<name>).
	aliasPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	// derivedFields are the frontmatter keys the render derives from the runtime fields.
	derivedFields = []string{FieldAnsweredBy, FieldDocker, FieldEnableDelegation, FieldEnforcePermissions}
	// markdownLinkPattern strips link targets, which inflate the rune count but not what the reader sees.
	markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	// requiredFields are the frontmatter keys Check itself depends on.
	requiredFields = []string{fieldName, fieldVersion}
)

// entryPosition locates an entry for the route-anchor checks.
type entryPosition struct {
	index   int
	role    string
	section *Section
}

// orderScope is gapped when a non-entry block sits between two of its entries.
type orderScope struct {
	entries  []*Entry
	isGapped bool
}

// violations skips a gapped scope as legacy layout.
func (s *orderScope) violations() []string {
	if s.isGapped {
		return nil
	}

	var violations []string
	previousRank, previousId := 0, ""
	for _, entry := range s.entries {
		rank := RoleOrderRank(entry.Role)
		switch {
		case rank > RoleOrderRank(RolePayload):
		case rank < previousRank:
			violations = append(violations, fmt.Sprintf("%s: role %q after %s breaks the section order def, rule, quality, flow, payload", entry.Id, entry.Role, previousId))
		default:
			previousRank, previousId = rank, entry.Id
		}
	}
	return violations
}

// answeredByViolations: a hook-answered skill is payloads plus standing text — it has at least one payload and no flow entry.
func answeredByViolations(skill *Skill) []string {
	if skill.AnsweredBy == "" {
		return nil
	}

	var violations []string
	hasPayload := false
	for _, ref := range skill.Entries() {
		if ref.Entry.Role == RolePayload {
			hasPayload = true
		}
		if RoleOrderRank(ref.Entry.Role) == rankFlow {
			violations = append(violations, fmt.Sprintf("answeredBy: entry %s is a flow entry — a hook-answered skill never runs", ref.Entry.Id))
		}
	}
	if !hasPayload {
		violations = append(violations, "answeredBy: no payload entry — the hook answers with the payload bodies")
	}
	return violations
}

// bulletText is an item's text as rendered after the marker.
func bulletText(item *Item) string {
	if item.Label == "" {
		return item.Text
	}
	return item.Label + ": " + item.Text
}

func checkAliases(skill *Skill) []string {
	var violations []string
	seen := map[string]bool{}
	for _, alias := range skill.Aliases() {
		switch {
		case !aliasPattern.MatchString(alias):
			violations = append(violations, fmt.Sprintf("alias %q is not kebab-case", alias))
		case alias == skill.Name():
			violations = append(violations, fmt.Sprintf("alias %q repeats the skill's own name", alias))
		case seen[alias]:
			violations = append(violations, fmt.Sprintf("alias %q appears twice", alias))
		}
		seen[alias] = true
	}
	return violations
}

func checkAnchorEntry(id string, positions map[string]entryPosition, route *Section) []string {
	anchor, ok := positions[id]
	switch {
	case !ok:
		return []string{fmt.Sprintf("section %q: anchor %s — no such entry", route.Title, id)}
	case !CoverableRole(anchor.role):
		return []string{fmt.Sprintf("section %q: anchor %s — a %s is never an anchor", route.Title, id, anchor.role)}
	case anchor.section == route:
		return []string{fmt.Sprintf("section %q: anchor %s — inside the route itself", route.Title, id)}
	}
	return nil
}

// checkAtoms: the merged registry keeps one declaration per id, as the old merge did.
func checkAtoms(skill *Skill) []string {
	if skill.Vocabulary == nil {
		return nil
	}

	var violations []string
	for _, id := range slices.Sorted(maps.Keys(skill.Atoms)) {
		if _, isShared := skill.Vocabulary.Atoms[id]; isShared {
			violations = append(violations, fmt.Sprintf("atom %s: declared in the skill's model file and in the shared vocabulary.cue", id))
		}
	}
	return violations
}

// checkBetween: routes never loop back, so the reentry comes after the split.
func checkBetween(pair []string, positions map[string]entryPosition, route *Section) []string {
	var violations []string
	for _, id := range pair {
		if id != AnchorStart && id != AnchorEnd {
			violations = append(violations, checkAnchorEntry(id, positions, route)...)
		}
	}
	isBounded := pair[0] == AnchorStart || pair[1] == AnchorEnd
	if len(violations) > 0 || isBounded {
		return violations
	}

	if positions[pair[1]].index <= positions[pair[0]].index {
		violations = append(violations, fmt.Sprintf("section %q: between %s..%s — reentry at or before the split", route.Title, pair[0], pair[1]))
	}
	return violations
}

func checkEntries(skill *Skill) []string {
	format := entryFormat(skill)
	wantName := SkillIdName(skill.Name())
	seen := map[string]bool{}
	var violations []string
	for _, ref := range skill.Entries() {
		entry := ref.Entry
		if seen[entry.Id] {
			violations = append(violations, entry.Id+": duplicate entry id")
		}

		seen[entry.Id] = true
		violations = append(violations, entryNameViolations(entry, wantName)...)
		violations = append(violations, entrySizeViolations(skill, entry, format)...)
	}
	return violations
}

// checkFrontmatter rejects a duplicate key because Field returns only the first.
func checkFrontmatter(skill *Skill) []string {
	var violations []string
	seen := map[string]bool{}
	for _, field := range skill.Frontmatter {
		if seen[field.Key] {
			violations = append(violations, fmt.Sprintf("frontmatter key %q appears twice", field.Key))
		}
		seen[field.Key] = true
	}
	for _, key := range requiredFields {
		if skill.Field(key) == "" {
			violations = append(violations, fmt.Sprintf("frontmatter key %q missing or empty", key))
		}
	}
	return violations
}

func checkPipeline(skill *Skill) []string {
	routing := skill.Routing()
	if routing == nil {
		return nil
	}

	pipeline := &routing.Pipeline
	var violations []string
	if !pipeline.Standalone && len(pipeline.Edges) == 0 && pipeline.Note == "" {
		violations = append(violations, "pipeline: empty — declare standalone, an edge or a note")
	}

	seen := map[string]bool{}
	for _, edge := range pipeline.Edges {
		key := edge.Kind + " " + edge.Skill
		if edge.Skill == skill.Name() {
			violations = append(violations, "pipeline: edge "+key+" names the skill itself")
		}
		if seen[key] {
			violations = append(violations, "pipeline: edge "+key+" is declared twice")
		}
		seen[key] = true
	}
	return violations
}

// checkProseWidth covers only verbatim text; the renderer wraps statements and bullets itself.
func checkProseWidth(skill *Skill) []string {
	if entryFormat(skill) < entryFormatStrict {
		return nil
	}

	violations := proseWidthViolations("preamble", skill.Preamble)
	for _, section := range skill.Sections {
		location := fmt.Sprintf("section %q", section.Title)
		for _, block := range section.Blocks {
			violations = append(violations, proseWidthViolations(location, block.Prose)...)
		}
	}
	return violations
}

func checkReplacesItem(item string, positions map[string]entryPosition, route *Section) []string {
	firstId, lastId, isRange := strings.Cut(item, anchorRangeSeparator)
	if !isRange {
		lastId = firstId
	}

	violations := checkAnchorEntry(firstId, positions, route)
	if isRange {
		violations = append(violations, checkAnchorEntry(lastId, positions, route)...)
	}

	isOutOfOrder := positions[firstId].index > positions[lastId].index
	if len(violations) == 0 && isOutOfOrder {
		violations = append(violations, fmt.Sprintf("section %q: replaces %s — range out of order", route.Title, item))
	}
	return violations
}

// checkRoleOrder enforces the format-2 role order in every scope.
func checkRoleOrder(skill *Skill) []string {
	if entryFormat(skill) < entryFormatStrict {
		return nil
	}

	var violations []string
	for _, section := range skill.Sections {
		for _, scope := range orderScopes(section) {
			violations = append(violations, scope.violations()...)
		}
	}
	return violations
}

func checkRouteAnchors(skill *Skill) []string {
	positions := map[string]entryPosition{}
	for index, ref := range skill.Entries() {
		if _, isKnown := positions[ref.Entry.Id]; !isKnown {
			positions[ref.Entry.Id] = entryPosition{index: index, role: ref.Entry.Role, section: ref.Section}
		}
	}
	var violations []string
	for _, section := range skill.Sections {
		if section.EffectiveKind() == SectionKindRoute {
			violations = append(violations, routeViolations(positions, section, skill.Name())...)
		}
	}
	return violations
}

// checkRuntimeMeta holds the cross-field rules of the runtime placement fields.
func checkRuntimeMeta(skill *Skill) []string {
	var violations []string
	for _, field := range skill.Frontmatter {
		if slices.Contains(derivedFields, field.Key) {
			violations = append(violations, fmt.Sprintf("frontmatter key %q is rendered from the model's runtime fields — never authored", field.Key))
		}
	}

	modes := skill.EnforcePermissions
	for index, mode := range modes {
		if slices.Contains(modes[:index], mode) {
			violations = append(violations, fmt.Sprintf("enforcePermissions lists %q twice", mode))
		}
	}

	hasBypass := slices.Contains(modes, PermissionBypass)
	if hasBypass && len(modes) > 1 {
		violations = append(violations, "enforcePermissions: bypass is exclusive — no other mode stands beside it")
	}
	if hasBypass && skill.Docker != DockerAlways {
		violations = append(violations, `enforcePermissions: bypass requires docker: "always"`)
	}

	isRootOnly := skill.EnableDelegation == DelegationNever
	if !isRootOnly && hasScriptPointer(skill) {
		violations = append(violations, `a Script: pointer names a Workflow stage, which only a root session launches — declare enableDelegation: "never"`)
	}
	violations = append(violations, answeredByViolations(skill)...)
	return violations
}

func checkTypedBlocks(skill *Skill) []string {
	var violations []string
	routings, models := 0, 0
	for _, section := range skill.Sections {
		for index, block := range section.Blocks {
			location := fmt.Sprintf("section %q block %d", section.Title, index+1)
			isRoutingPlaced := section.Title == SectionRouting && index == 0
			if block.Routing != nil && !isRoutingPlaced {
				violations = append(violations, location+": a routing block is the first block of the "+SectionRouting+" section")
			}
			isModelPlaced := section.Title == SectionModel && len(section.Blocks) == 1
			if block.Model != nil && !isModelPlaced {
				violations = append(violations, location+": a model block is the only block of the "+SectionModel+" section")
			}
			if block.Routing != nil {
				routings++
			}
			if block.Model != nil {
				models++
			}
		}
	}
	if routings > 1 || models > 1 {
		violations = append(violations, "a skill carries at most one routing block and one model block")
	}
	return violations
}

// checkVersion is the one home of the version agreement (D10).
func checkVersion(skill *Skill) []string {
	if len(skill.Changelog) == 0 {
		return []string{"changelog is empty — its newest entry carries the frontmatter version"}
	}

	newest := skill.Changelog[0].Version
	if skill.Version() != newest {
		return []string{fmt.Sprintf("frontmatter version %q differs from the newest changelog entry %q", skill.Version(), newest)}
	}
	return descendingViolations(skill.Changelog)
}

// descendingViolations names every adjacent pair whose upper version is not above the lower one.
func descendingViolations(rows []ChangelogEntry) []string {
	var violations []string
	for index := range len(rows) - 1 {
		order, err := semver.Compare(rows[index].Version, rows[index+1].Version)
		if err != nil {
			violations = append(violations, err.Error())
			continue
		}

		if order <= 0 {
			violations = append(violations, fmt.Sprintf("changelog versions are not strictly descending: %q above %q", rows[index].Version, rows[index+1].Version))
		}
	}
	return violations
}

func displayRunes(text string) int {
	display := markdownLinkPattern.ReplaceAllString(text, "$1")
	return utf8.RuneCountInString(display)
}

func entryFormat(skill *Skill) int {
	format, err := strconv.Atoi(skill.Field(fieldFormat))
	if err != nil {
		return entryFormatLegacy
	}
	return format
}

// entryNameViolations checks that the id's NAME segment is this skill's.
func entryNameViolations(entry *Entry, wantName string) []string {
	parts := strings.SplitN(entry.Id, "-", entryIdParts)
	isComparable := wantName != "" && len(parts) == entryIdParts
	if !isComparable || parts[entryIdNamePart] == wantName {
		return nil
	}
	return []string{fmt.Sprintf("%s: entry names skill %s, the skill's id name is %s", entry.Id, parts[entryIdNamePart], wantName)}
}

func entrySizeViolations(skill *Skill, entry *Entry, format int) []string {
	statementCap := statementMaxRunes
	if format >= entryFormatStrict {
		statementCap = statementMaxRunesStrict
	}

	var violations []string
	if displayRunes(displayText(skill, entry.Statement)) > statementCap {
		violations = append(violations, fmt.Sprintf("%s: statement exceeds %d runes — split into %sa/%sb", entry.Id, statementCap, entry.Id, entry.Id))
	}

	if format < entryFormatStrict {
		return violations
	}

	for _, bullet := range logicalBullets(entry.Items) {
		if strings.HasPrefix(bullet, bulletLabelWhy+":") {
			violations = append(violations, entry.Id+": Why bullet — rationale moves to the skill's README.md design notes")
		}
		if displayRunes(displayText(skill, bullet)) > bulletMaxRunesStrict {
			violations = append(violations, fmt.Sprintf("%s: bullet exceeds %d runes — one claim per bullet; sequences go to a fenced procedure block", entry.Id, bulletMaxRunesStrict))
		}
	}
	return violations
}

// hasScriptPointer reports an entry naming a stage file, in its script field or a Script: item.
func hasScriptPointer(skill *Skill) bool {
	for _, ref := range skill.Entries() {
		if ref.Entry.ScriptPointer() != "" {
			return true
		}
	}
	return false
}

// isTopStarBullet reports whether an item opens a measured bullet.
func isTopStarBullet(item *Item) bool {
	isTopLevel := item.Fence == nil && item.Depth == 0
	return isTopLevel && item.Marker == bulletMarkerStar
}

// logicalBullets folds items the way the markdown parser measured bullets.
func logicalBullets(items []Item) []string {
	var bullets []string
	isOpen := false
	for index := range items {
		item := &items[index]
		isNested := item.Fence == nil && item.Depth > 0
		switch {
		case isNested && isOpen:
			bullets[len(bullets)-1] += " " + item.Marker + " " + bulletText(item)
		case isNested:
		case isTopStarBullet(item):
			bullets = append(bullets, bulletText(item))
			isOpen = true
		default:
			isOpen = false
		}
	}
	return bullets
}

func orderScopes(section *Section) []orderScope {
	scopes := []orderScope{{}}
	hasGap := false
	for _, block := range section.Blocks {
		last := len(scopes) - 1
		switch {
		case block.Heading != "":
			scopes = append(scopes, orderScope{})
			hasGap = false
		case block.Entry != nil:
			isGapBetween := hasGap && len(scopes[last].entries) > 0
			scopes[last].isGapped = scopes[last].isGapped || isGapBetween
			scopes[last].entries = append(scopes[last].entries, block.Entry)
			hasGap = false
		default:
			hasGap = true
		}
	}
	return scopes
}

func proseWidthViolations(location, text string) []string {
	var violations []string
	openFence := ""
	for line := range strings.SplitSeq(text, "\n") {
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(line, openFence, run)
			continue
		}

		isExempt := openFence != "" || strings.HasPrefix(strings.TrimSpace(line), "|")
		if isExempt || utf8.RuneCountInString(line) <= lineWidth {
			continue
		}

		violations = append(violations, fmt.Sprintf("%s: line exceeds %d runes — wrap it: %.*s…", location, lineWidth, violationHeadRunes, line))
	}
	return violations
}

func routeViolations(positions map[string]entryPosition, route *Section, skillName string) []string {
	hasReplaces := route.Replaces != ""
	hasBetween := route.Between != ""
	switch {
	case !hasReplaces && !hasBetween:
		return []string{fmt.Sprintf("section %q: route section without anchor — declare replaces: <items> or between: <after>..<before>", route.Title)}
	case hasReplaces && hasBetween:
		return []string{fmt.Sprintf("section %q: route section with both replaces and between — exactly one anchor", route.Title)}
	}

	replaces, between, err := route.Anchor(skillName)
	if err != nil {
		return []string{fmt.Sprintf("section %q: %v", route.Title, err)}
	}

	var violations []string
	for _, item := range replaces {
		violations = append(violations, checkReplacesItem(item, positions, route)...)
	}
	if len(between) == betweenAnchorCount {
		violations = append(violations, checkBetween(between, positions, route)...)
	}
	return violations
}

// Check tolerates schema-invalid models because Render runs it before the guard's schema decode.
func Check(skill *Skill) []string {
	var violations []string
	violations = append(violations, checkFrontmatter(skill)...)
	violations = append(violations, checkAliases(skill)...)
	violations = append(violations, checkVersion(skill)...)
	violations = append(violations, checkEntries(skill)...)
	violations = append(violations, checkLabels(skill)...)
	violations = append(violations, checkFields(skill)...)
	violations = append(violations, checkRefs(skill)...)
	violations = append(violations, checkTitles(skill)...)
	violations = append(violations, checkRoleOrder(skill)...)
	violations = append(violations, checkRouteAnchors(skill)...)
	violations = append(violations, checkAtoms(skill)...)
	violations = append(violations, checkSources(skill)...)
	violations = append(violations, checkClauseAtoms(skill)...)
	violations = append(violations, checkEnforce(skill)...)
	violations = append(violations, checkTitle(skill)...)
	violations = append(violations, checkSmoke(skill)...)
	violations = append(violations, checkRequirements(skill)...)
	violations = append(violations, checkRuntimeMeta(skill)...)
	violations = append(violations, checkEnv(skill)...)
	violations = append(violations, checkProseWidth(skill)...)
	violations = append(violations, checkTypedBlocks(skill)...)
	violations = append(violations, checkPipeline(skill)...)
	return violations
}
