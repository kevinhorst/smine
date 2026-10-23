package plans

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	changesTitle     = "Changes"
	checkRejected    = "rejected"
	driverCheckCell  = "Check"
	driverImpactCell = "Impact"
	driversPrefix    = "drivers:"
	driversTitle     = "Drivers"
	entryPrefix      = "### "
	hotPrefix        = "hot:"
	impactNoChange   = "no-change"
	locationPrefix   = "location:"
	maxPhaseEntries  = 4
	minTableCells    = 2
	phaseEntryCell   = "Entries"
	phaseHeadCell    = "Phase"
	phasePrefix      = "phase:"
	runsAlone        = "alone"
	runsParallel     = "parallel"
	sectionPrefix    = "## "
	tableFileCell    = "File"
	tableKindCell    = "Kind"
)

const (
	tableBefore = iota
	tableInside
	tableDone
	tablePhase
)

// Attention classes of a Changes entry, in the document order the plan rule demands.
const (
	classHot = iota
	classCode
	classProse
)

// Gates a plan under review is presented at.
const (
	GateCode   = "code"
	GateDesign = "design"
)

var (
	codeSpan    = regexp.MustCompile("`([^`]+)`")
	driverId    = regexp.MustCompile(`\bR\d+\b`)
	driverRowId = regexp.MustCompile(`^R\d+$`)
	lineSuffix  = regexp.MustCompile(`:\d+(-\d+)?$`)
	phaseLink   = regexp.MustCompile(`\[([^\]]+)\]\(#`)
	phaseNumber = regexp.MustCompile(`Phase: (\d+)`)
	slugDropped = regexp.MustCompile(`[^a-z0-9 -]`)
)

type Driver struct {
	Id         string
	IsNoChange bool
	IsRejected bool
	Line       int
}

// NeedsEntry reports whether a Changes entry has to name the row.
func (d *Driver) NeedsEntry() bool {
	return !d.IsNoChange && !d.IsRejected
}

type Entry struct {
	BadSpans    []string
	Drivers     []string
	Files       []string
	HasDrivers  bool
	HasLocation bool
	Heading     string
	IsAgentOnly bool
	IsHot       bool
	Line        int
	Phase       int
	PhaseText   string
}

type PhaseRow struct {
	Entries []string
	Line    int
	Phase   int
	Runs    string
}

type Unit struct {
	Entries    []*Entry
	Line       int
	PhaseLine  int
	PhaseRows  []*PhaseRow
	TableFiles []string
	TableLine  int
}

type Violation struct {
	Line    int
	Message string
}

type driverTable struct {
	columns   map[string]int
	drivers   []*Driver
	isSection bool
}

func (t *driverTable) cell(cells []string, name string) string {
	index, isKnown := t.columns[name]
	if !isKnown || index >= len(cells) {
		return ""
	}
	return cells[index]
}

func (t *driverTable) feed(number int, line string) {
	title, isHeading := strings.CutPrefix(line, sectionPrefix)
	if isHeading {
		t.isSection = strings.TrimSpace(title) == driversTitle
		return
	}

	trimmed := strings.TrimSpace(line)
	isRow := t.isSection && strings.HasPrefix(trimmed, "|") && !isSeparatorRow(trimmed)
	if !isRow {
		return
	}

	cells := tableCells(trimmed)
	if t.columns == nil {
		t.columns = map[string]int{}
		for index, cell := range cells {
			t.columns[cell] = index
		}
		return
	}

	if !driverRowId.MatchString(cells[0]) {
		return
	}

	driver := &Driver{
		Id:         cells[0],
		IsNoChange: t.cell(cells, driverImpactCell) == impactNoChange,
		IsRejected: strings.HasPrefix(t.cell(cells, driverCheckCell), checkRejected),
		Line:       number,
	}
	t.drivers = append(t.drivers, driver)
}

// unitParser folds a plan's unfenced lines into its Changes unit; a second ## Changes heading opens nothing.
type unitParser struct {
	changes    *Unit
	entry      *Entry
	isOpen     bool
	tableState int
}

func (p *unitParser) feed(number int, line string) {
	switch {
	case strings.HasPrefix(line, sectionPrefix):
		p.openSection(number, strings.TrimSpace(strings.TrimPrefix(line, sectionPrefix)))
	case !p.isOpen:
		return
	case p.isEntryHeading(line):
		p.openEntry(number, line)
	default:
		p.feedBody(number, strings.TrimSpace(line))
	}
}

func (p *unitParser) feedBody(number int, trimmed string) {
	if p.entry == nil {
		p.feedTable(number, trimmed)
		return
	}
	p.feedEntryLine(trimmed)
}

func (p *unitParser) feedEntryLine(trimmed string) {
	if strings.HasPrefix(trimmed, hotPrefix) {
		p.entry.IsHot = true
		return
	}

	location, isLocation := strings.CutPrefix(trimmed, locationPrefix)
	if isLocation && !p.entry.HasLocation {
		p.entry.HasLocation = true
		p.entry.Files, p.entry.BadSpans = locationFiles(location)
		return
	}

	drivers, isDrivers := strings.CutPrefix(trimmed, driversPrefix)
	if isDrivers && !p.entry.HasDrivers {
		p.entry.HasDrivers = true
		p.entry.Drivers = driverId.FindAllString(drivers, -1)
		return
	}

	value, isPhase := strings.CutPrefix(trimmed, phasePrefix)
	if !isPhase || p.entry.PhaseText != "" {
		return
	}
	p.entry.PhaseText = strings.TrimSpace(value)
	phase, err := strconv.Atoi(p.entry.PhaseText)
	if err == nil {
		p.entry.Phase = phase
	}
}

func (p *unitParser) feedPhaseRow(number int, trimmed string, isRow bool) {
	if !isRow {
		p.tableState = tableDone
		return
	}
	if isSeparatorRow(trimmed) {
		return
	}

	cells := tableCells(trimmed)
	row := &PhaseRow{Line: number}
	if match := phaseNumber.FindStringSubmatch(cells[0]); match != nil {
		row.Phase, _ = strconv.Atoi(match[1])
	}
	if len(cells) >= minTableCells {
		for _, match := range phaseLink.FindAllStringSubmatch(cells[1], -1) {
			row.Entries = append(row.Entries, match[1])
		}
	}
	if len(cells) > minTableCells {
		row.Runs = cells[2]
	}
	p.changes.PhaseRows = append(p.changes.PhaseRows, row)
}

func (p *unitParser) feedTable(number int, trimmed string) {
	isRow := strings.HasPrefix(trimmed, "|")
	switch {
	case p.tableState == tableBefore && isRow:
		p.openTable(number, tableCells(trimmed))
	case p.tableState == tableDone && isRow:
		p.openPhaseTable(number, tableCells(trimmed))
	case p.tableState == tablePhase:
		p.feedPhaseRow(number, trimmed, isRow)
	case p.tableState != tableInside:
		return
	case !isRow:
		p.tableState = tableDone
	case !isSeparatorRow(trimmed):
		cells := tableCells(trimmed)
		p.changes.TableFiles = append(p.changes.TableFiles, spanPaths(cells[0])...)
	}
}

func (p *unitParser) isEntryHeading(line string) bool {
	return p.tableState != tableBefore && strings.HasPrefix(line, entryPrefix)
}

func (p *unitParser) openEntry(number int, line string) {
	p.tableState = tableDone
	heading := strings.TrimSpace(strings.TrimPrefix(line, entryPrefix))
	_, isAgentOnly := SplitHeading(heading)
	p.entry = &Entry{Heading: heading, IsAgentOnly: isAgentOnly, Line: number}
	p.changes.Entries = append(p.changes.Entries, p.entry)
}

func (p *unitParser) openPhaseTable(number int, cells []string) {
	isPhaseTable := p.changes.PhaseLine == 0 && len(cells) >= minTableCells && cells[0] == phaseHeadCell && cells[1] == phaseEntryCell
	if isPhaseTable {
		p.tableState = tablePhase
		p.changes.PhaseLine = number
	}
}

func (p *unitParser) openSection(number int, title string) {
	p.entry = nil
	p.isOpen = title == changesTitle && p.changes == nil
	if p.isOpen {
		p.changes = &Unit{Line: number}
		p.tableState = tableBefore
	}
}

func (p *unitParser) openTable(number int, cells []string) {
	if len(cells) < minTableCells {
		return
	}

	isPlannedFiles := cells[0] == tableFileCell && cells[1] == tableKindCell
	if isPlannedFiles {
		p.tableState = tableInside
		p.changes.TableLine = number
	}
}

// claimFile records entry as the owner of file in its phase; a second entry of the same phase is a violation.
func claimFile(entry *Entry, file string, owners map[string]*Entry) []Violation {
	key := fmt.Sprintf("%d %s", entry.Phase, file)
	owner, isOwned := owners[key]
	if !isOwned {
		owners[key] = entry
		return nil
	}

	message := fmt.Sprintf("%s: phase %d: %s is touched by %q and %q — entries of one phase touch disjoint files", changesTitle, entry.Phase, file, owner.Heading, entry.Heading)
	return []Violation{{Line: entry.Line, Message: message}}
}

func driverStatus(driver *Driver, entries int) string {
	switch {
	case driver.IsRejected:
		return checkRejected
	case driver.IsNoChange:
		return impactNoChange
	case entries == 0:
		return "no entry"
	}
	return fmt.Sprintf("covered (%d)", entries)
}

func entryClass(entry *Entry) int {
	switch {
	case entry.IsAgentOnly:
		return classProse
	case entry.IsHot:
		return classHot
	}
	return classCode
}

func entryDriverProblems(entry *Entry, known map[string]bool) []Violation {
	var messages []string
	switch {
	case !entry.HasDrivers:
		messages = append(messages, "has no drivers: line")
	case len(entry.Drivers) == 0:
		messages = append(messages, "names no R<n> row on its drivers: line")
	}
	for _, id := range entry.Drivers {
		if !known[id] {
			messages = append(messages, fmt.Sprintf("names driver %s, which the Drivers table lacks", id))
		}
	}

	violations := make([]Violation, 0, len(messages))
	for _, message := range messages {
		violation := Violation{Line: entry.Line, Message: fmt.Sprintf("%s: entry %q %s", changesTitle, entry.Heading, message)}
		violations = append(violations, violation)
	}
	return violations
}

func entryProblems(entry *Entry) []Violation {
	var messages []string
	if !entry.HasLocation {
		messages = append(messages, "has no location: line")
	}
	for _, span := range entry.BadSpans {
		messages = append(messages, fmt.Sprintf("location span %q is not one repo-relative file path", span))
	}
	switch {
	case entry.PhaseText == "":
		messages = append(messages, "has no phase: line")
	case entry.Phase < 1:
		messages = append(messages, fmt.Sprintf("phase: must be a positive integer, got %q", entry.PhaseText))
	}

	violations := make([]Violation, 0, len(messages))
	for _, message := range messages {
		violation := Violation{Line: entry.Line, Message: fmt.Sprintf("%s: entry %q %s", changesTitle, entry.Heading, message)}
		violations = append(violations, violation)
	}
	return violations
}

func entryViolations(unit *Unit) []Violation {
	if len(unit.Entries) == 0 {
		return []Violation{{Line: unit.TableLine, Message: changesTitle + ": no Changes entry after the planned-files table"}}
	}

	var violations []Violation
	for _, entry := range unit.Entries {
		violations = append(violations, entryProblems(entry)...)
	}
	return violations
}

func expectedPhaseRows(unit *Unit) []*PhaseRow {
	byPhase := map[int]*PhaseRow{}
	for _, entry := range unit.Entries {
		if entry.Phase < 1 {
			continue
		}

		row, isKnown := byPhase[entry.Phase]
		if !isKnown {
			row = &PhaseRow{Phase: entry.Phase, Runs: runsAlone}
			byPhase[entry.Phase] = row
		}
		name, _ := SplitHeading(entry.Heading)
		row.Entries = append(row.Entries, name)
		if len(row.Entries) > 1 {
			row.Runs = runsParallel
		}
	}

	rows := make([]*PhaseRow, 0, len(byPhase))
	for _, phase := range slices.Sorted(maps.Keys(byPhase)) {
		rows = append(rows, byPhase[phase])
	}
	return rows
}

func isFilePath(path string) bool {
	if strings.ContainsAny(path, " {}*?[]") {
		return false
	}

	isAnchored := strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~")
	isDirectory := path == "" || strings.HasSuffix(path, "/")
	return !isAnchored && !isDirectory
}

func isSamePhaseRow(expected *PhaseRow, actual *PhaseRow) bool {
	isSameHead := expected.Phase == actual.Phase && expected.Runs == actual.Runs
	return isSameHead && slices.Equal(slices.Sorted(slices.Values(expected.Entries)), slices.Sorted(slices.Values(actual.Entries)))
}

func isSeparatorRow(row string) bool {
	return strings.Trim(row, "|:- ") == ""
}

func locationFiles(text string) ([]string, []string) {
	var files, bad []string
	for _, match := range codeSpan.FindAllStringSubmatch(text, -1) {
		path := lineSuffix.ReplaceAllString(match[1], "")
		if isFilePath(path) {
			files = append(files, path)
			continue
		}
		bad = append(bad, match[1])
	}
	return files, bad
}

func missingFrom(source map[string]bool, target map[string]bool) []string {
	var missing []string
	for _, file := range slices.Sorted(maps.Keys(source)) {
		if !target[file] {
			missing = append(missing, file)
		}
	}
	return missing
}

func overlapViolations(unit *Unit) []Violation {
	owners := map[string]*Entry{}
	var violations []Violation
	for _, entry := range unit.Entries {
		if entry.Phase < 1 {
			continue
		}

		files := slices.Clone(entry.Files)
		slices.Sort(files)
		for _, file := range slices.Compact(files) {
			violations = append(violations, claimFile(entry, file, owners)...)
		}
	}
	return violations
}

// plainLines calls visit for every line of markdown outside a fenced block, with its 1-based number.
func plainLines(markdown string, visit func(number int, line string)) {
	fence := ""
	for index, line := range strings.Split(markdown, "\n") {
		wasFenced := fence != ""
		fence = NextFence(fence, line)
		isFenced := wasFenced || fence != ""
		if !isFenced {
			visit(index+1, line)
		}
	}
}

func servedDrivers(unit *Unit) map[string]int {
	served := map[string]int{}
	for _, entry := range unit.Entries {
		for _, id := range slices.Compact(slices.Sorted(slices.Values(entry.Drivers))) {
			served[id]++
		}
	}
	return served
}

func setOf(files []string) map[string]bool {
	set := make(map[string]bool, len(files))
	for _, file := range files {
		set[file] = true
	}
	return set
}

func spanPaths(text string) []string {
	var paths []string
	for _, match := range codeSpan.FindAllStringSubmatch(text, -1) {
		paths = append(paths, lineSuffix.ReplaceAllString(match[1], ""))
	}
	return paths
}

// tableCells splits a markdown row on unescaped pipes; an escaped `\|` stays in its cell as `|`.
func tableCells(row string) []string {
	var cells []string
	var cell strings.Builder
	inner := strings.TrimSpace(row)
	inner = strings.TrimPrefix(inner, "|")
	if strings.HasSuffix(inner, "|") && !strings.HasSuffix(inner, `\|`) {
		inner = strings.TrimSuffix(inner, "|")
	}
	for index := 0; index < len(inner); index++ {
		isEscapedPipe := inner[index] == '\\' && index+1 < len(inner) && inner[index+1] == '|'
		switch {
		case isEscapedPipe:
			cell.WriteByte('|')
			index++
		case inner[index] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(inner[index])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

func tableViolations(unit *Unit) []Violation {
	listed := setOf(unit.TableFiles)
	located := map[string]bool{}
	for _, entry := range unit.Entries {
		maps.Copy(located, setOf(entry.Files))
	}

	var violations []Violation
	for _, file := range missingFrom(listed, located) {
		violation := Violation{Line: unit.TableLine, Message: fmt.Sprintf("%s: %s is in the planned-files table but in no entry's location: line", changesTitle, file)}
		violations = append(violations, violation)
	}
	for _, file := range missingFrom(located, listed) {
		violation := Violation{Line: unit.TableLine, Message: fmt.Sprintf("%s: %s is in a location: line but not in the planned-files table", changesTitle, file)}
		violations = append(violations, violation)
	}
	return violations
}

// CheckDrivers checks the Changes entries against the Drivers rows.
func CheckDrivers(drivers []*Driver, unit *Unit) []Violation {
	known := map[string]bool{}
	for _, driver := range drivers {
		known[driver.Id] = true
	}

	var violations []Violation
	for _, entry := range unit.Entries {
		violations = append(violations, entryDriverProblems(entry, known)...)
	}

	served := servedDrivers(unit)
	for _, driver := range drivers {
		isOpen := served[driver.Id] == 0 && driver.NeedsEntry()
		if isOpen {
			violation := Violation{Line: driver.Line, Message: fmt.Sprintf("%s: driver %s has no Changes entry — name it on an entry's drivers: line", driversTitle, driver.Id)}
			violations = append(violations, violation)
		}
	}
	return violations
}

// CheckOrder checks the document order of a unit's entries.
func CheckOrder(unit *Unit) []Violation {
	var violations []Violation
	last := classHot
	for _, entry := range unit.Entries {
		isHotProse := entry.IsAgentOnly && entry.IsHot
		if isHotProse {
			message := fmt.Sprintf("%s: entry %q carries a hot: line and %s — a hot entry is never agent-only", changesTitle, entry.Heading, AgentOnlyMarker)
			violation := Violation{Line: entry.Line, Message: message}
			violations = append(violations, violation)
		}

		class := entryClass(entry)
		if class < last {
			message := fmt.Sprintf("%s: entry %q stands behind an entry of a later group — order is hot entries, other code, then %s entries", changesTitle, entry.Heading, AgentOnlyMarker)
			violation := Violation{Line: entry.Line, Message: message}
			violations = append(violations, violation)
			continue
		}
		last = class
	}
	return violations
}

// CheckPhases checks the phase cap and the phase table against the phase: lines.
func CheckPhases(unit *Unit) []Violation {
	expected := expectedPhaseRows(unit)
	var violations []Violation
	for _, row := range expected {
		if len(row.Entries) > maxPhaseEntries {
			message := fmt.Sprintf("%s: phase %d holds %d entries — at most %d, an entry is one agent's work package", changesTitle, row.Phase, len(row.Entries), maxPhaseEntries)
			violations = append(violations, Violation{Line: unit.Line, Message: message})
		}
	}

	if unit.PhaseLine == 0 {
		message := changesTitle + ": phase table (Phase | Entries | Runs | Needs) missing under the planned-files table"
		return append(violations, Violation{Line: unit.Line, Message: message})
	}

	isSame := slices.EqualFunc(expected, unit.PhaseRows, isSamePhaseRow)
	if !isSame {
		message := changesTitle + ": phase table does not match the phase: lines of the entries"
		violations = append(violations, Violation{Line: unit.PhaseLine, Message: message})
	}
	return violations
}

// CheckPlan checks a plan's Changes, strictly once they hold entries; a plan without a Changes section passes.
func CheckPlan(drivers []*Driver, unit *Unit) []Violation {
	if unit == nil {
		return nil
	}

	hasEntries := len(unit.Entries) > 0
	violations := CheckUnit(unit, hasEntries)
	if !hasEntries {
		return violations
	}
	return append(violations, CheckDrivers(drivers, unit)...)
}

// CheckUnit checks one unit only when isStrict is set or any entry declares a phase.
func CheckUnit(unit *Unit, isStrict bool) []Violation {
	isDeclared := slices.ContainsFunc(unit.Entries, func(entry *Entry) bool { return entry.PhaseText != "" })
	if !isStrict && !isDeclared {
		return nil
	}
	if unit.TableLine == 0 {
		return []Violation{{Line: unit.Line, Message: changesTitle + ": planned-files table (File | Kind | Entry) missing — entries are the headings after it"}}
	}

	violations := entryViolations(unit)
	violations = append(violations, overlapViolations(unit)...)
	return append(violations, tableViolations(unit)...)
}

func DriverLines(drivers []*Driver, unit *Unit) []string {
	served := servedDrivers(unit)
	lines := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		lines = append(lines, driver.Id+" "+driverStatus(driver, served[driver.Id]))
	}
	return lines
}

// ExpectedPhaseTable renders the phase table the phase: lines imply, its Needs cells left to the author.
func ExpectedPhaseTable(unit *Unit) string {
	lines := []string{"| Phase | Entries | Runs | Needs |", "| :-- | :-- | :-- | :-- |"}
	for _, row := range expectedPhaseRows(unit) {
		links := make([]string, 0, len(row.Entries))
		for _, name := range row.Entries {
			slug := strings.ReplaceAll(slugDropped.ReplaceAllString(strings.ToLower(name), ""), " ", "-")
			links = append(links, fmt.Sprintf("[%s](#%s)", name, slug))
		}
		lines = append(lines, fmt.Sprintf("| **Phase: %d** | %s | %s | |", row.Phase, strings.Join(links, clauseBreak), row.Runs))
	}
	return strings.Join(lines, "\n")
}

// Gate is design while the Changes hold no entry, code once they do.
func Gate(markdown string) string {
	unit := ParseChanges(markdown)
	hasEntries := unit != nil && len(unit.Entries) > 0
	if hasEntries {
		return GateCode
	}
	return GateDesign
}

// ParseChanges returns a plan's ## Changes section, nil when the plan has none.
func ParseChanges(markdown string) *Unit {
	parser := &unitParser{}
	plainLines(markdown, parser.feed)
	return parser.changes
}

func ParseDrivers(markdown string) []*Driver {
	table := &driverTable{}
	plainLines(markdown, table.feed)
	return table.drivers
}
