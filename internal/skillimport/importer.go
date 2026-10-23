// Package skillimport converts a markdown skill into the typed model and proves the conversion changed whitespace only.
package skillimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

// File names and fixed lines of the markdown skill format.
const (
	argsSectionTitle       = "Args"
	atomsFile              = "atoms.json"
	canonicalArgsAlignment = "| :-- | :--- | :------ | :----- |"
	changelogFile          = "changelog.json"
	entryHeadlinePrefix    = "**SKILL-"
	frontmatterFence       = "---"
	legacyArgsAlignment    = "| :--- | :--- | :--- | :--- |"
	sectionClausePrefix    = "`{section:"
)

// Whitespace-stripped table rows the importer types, and the table shapes.
const (
	argsAlignmentStripped = "|:--|:---|:------|:-----|"
	argsHeaderStripped    = "|Arg|Form|Default|Effect|"
	argsTableCells        = 4
)

// bulletIndentStep is one nesting level; anchorReplaces and anchorBetween come from the parser.
const bulletIndentStep = 2

var (
	// argBacktickedPattern is a canonical Args bullet: - `<name>`: <effect>.
	argBacktickedPattern = regexp.MustCompile("^- `([^`]+)`: (.*)$")
	// argBarePattern is the bare-name Args bullet D7 b) rewrites (delegate, smine-routines, smine-memory).
	argBarePattern        = regexp.MustCompile(`^- ([A-Za-z0-9][A-Za-z0-9|<> _-]*?): (.*)$`)
	frontmatterKeyPattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_-]*):(.*)$`)
	// itemLabelPattern is the text before the first ": " of a labeled bullet.
	itemLabelPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z -]{0,24}$`)
)

type document struct {
	lines []string
	spans map[int]Entry
}

func newDocument(data []byte) *document {
	lines := strings.Split(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return &document{lines: lines}
}

func (d *document) appendProse(blocks []skillgraph.Block, end, start int) []skillgraph.Block {
	prose := d.prose(end, start)
	if prose == "" {
		return blocks
	}

	block := skillgraph.Block{Prose: prose}
	return append(blocks, block)
}

// argsBullet takes a bare name without backticks (D7 b).
func (d *document) argsBullet(index int) (*skillgraph.Arg, error) {
	line := d.lines[index]
	match := argBacktickedPattern.FindStringSubmatch(line)
	if match == nil {
		match = argBarePattern.FindStringSubmatch(line)
	}

	if match == nil {
		return nil, d.lineError("document.argsBullet", index, "Args bullet is not - `<name>`: <effect>")
	}

	arg := &skillgraph.Arg{Effect: match[2], Name: match[1]}
	return arg, nil
}

// argsBullets keeps a blank line inside the list only when another bullet follows.
func (d *document) argsBullets(end, index int) (*skillgraph.Block, int, error) {
	args := &skillgraph.ArgsBlock{Bullets: true}
	next := index
	for ; next < end && d.continuesArgsBullets(end, next); next++ {
		line := d.lines[next]
		switch {
		case strings.HasPrefix(line, "- "):
			arg, err := d.argsBullet(next)
			if err != nil {
				return nil, 0, err
			}

			args.Rows = append(args.Rows, *arg)
		case isIndentedText(line):
			last := &args.Rows[len(args.Rows)-1]
			last.Effect += " " + strings.TrimSpace(line)
		}
	}
	for position := range args.Rows {
		args.Rows[position].Effect = normalizeSpace(args.Rows[position].Effect)
	}

	block := &skillgraph.Block{Args: args}
	return block, next, nil
}

// argsRow reads one Args table row: four cells, the Arg cell backticked.
func (d *document) argsRow(index int) (*skillgraph.Arg, error) {
	cells := skills.SplitTableRow(d.lines[index])
	if len(cells) != argsTableCells {
		return nil, d.lineError("document.argsRow", index, fmt.Sprintf("Args row has %d cells, the index has %d", len(cells), argsTableCells))
	}

	name, isBackticked := strings.CutPrefix(cells[0], "`")
	name, hasClosing := strings.CutSuffix(name, "`")
	if !isBackticked || !hasClosing {
		return nil, d.lineError("document.argsRow", index, "Args row name cell is not backticked")
	}

	arg := &skillgraph.Arg{
		Default: cells[2],
		Effect:  cells[3],
		Form:    cells[1],
		Name:    name,
	}
	return arg, nil
}

// argsTable accepts the canonical or the D7 a) alignment row.
func (d *document) argsTable(end, index int) (*skillgraph.Block, int, error) {
	alignment := index + 1
	if alignment >= end || !isArgsAlignment(d.lines[alignment]) {
		return nil, 0, d.lineError("document.argsTable", alignment, "Args alignment row is neither the canonical nor the D7 a) form")
	}

	args := &skillgraph.ArgsBlock{}
	next := alignment + 1
	for ; next < end && isTableRow(d.lines[next]); next++ {
		arg, err := d.argsRow(next)
		if err != nil {
			return nil, 0, err
		}

		args.Rows = append(args.Rows, *arg)
	}
	if len(args.Rows) == 0 {
		return nil, 0, d.lineError("document.argsTable", index, "Args table has no rows")
	}

	block := &skillgraph.Block{Args: args}
	return block, next, nil
}

func (d *document) block(end, index int, title string) (*skillgraph.Block, int, error) {
	if parsed, isEntry := d.spans[index]; isEntry {
		return d.entryBlock(end, &parsed)
	}

	line := d.lines[index]
	heading, isHeading := strings.CutPrefix(line, "### ")
	stripped, _ := strip(line)
	isArgs := title == argsSectionTitle
	switch {
	case isHeading && strings.TrimSpace(heading) != "":
		block := &skillgraph.Block{Heading: strings.TrimSpace(heading)}
		return block, index + 1, nil
	case isArgs && stripped == argsHeaderStripped:
		return d.argsTable(end, index)
	case isArgs && strings.HasPrefix(line, "- "):
		return d.argsBullets(end, index)
	}
	return nil, index, nil
}

// blocks never inspects fenced prose for structure.
func (d *document) blocks(end, start int, title string) ([]skillgraph.Block, error) {
	var blocks []skillgraph.Block
	proseStart := start
	openFence := ""
	for index := start; index < end; {
		line := d.lines[index]
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			index++
			continue
		}

		if openFence != "" {
			index++
			continue
		}

		block, next, err := d.block(end, index, title)
		if err != nil {
			return nil, err
		}

		if block == nil {
			index++
			continue
		}

		blocks = d.appendProse(blocks, index, proseStart)
		blocks = append(blocks, *block)
		index, proseStart = next, next
	}
	blocks = d.appendProse(blocks, end, proseStart)
	return blocks, nil
}

// bulletLine rejects any other line as content the model cannot hold.
func (d *document) bulletLine(entry *skillgraph.Entry, index int) error {
	line := d.lines[index]
	text := strings.TrimLeft(line, " ")
	indent := len(line) - len(text)
	isBullet := strings.HasPrefix(text, "* ") || strings.HasPrefix(text, "- ")
	isOddIndent := indent%bulletIndentStep != 0
	hasOpenBullet := len(entry.Items) > 0 && entry.Items[len(entry.Items)-1].Fence == nil
	switch {
	case isBullet && isOddIndent:
		return d.lineError("document.bulletLine", index, fmt.Sprintf("Bullet at odd indent %d", indent))
	case isBullet:
		item := skillgraph.Item{
			Depth:  indent / bulletIndentStep,
			Marker: text[:1],
			Text:   text[2:],
		}
		entry.Items = append(entry.Items, item)
	case isIndentedText(line) && hasOpenBullet:
		last := &entry.Items[len(entry.Items)-1]
		last.Text += " " + strings.TrimSpace(line)
	default:
		return d.lineError("document.bulletLine", index, "Entry line is no bullet, continuation or fence")
	}
	return nil
}

// checkEntryCoverage keeps an entry outside every block from being silently dropped.
func (d *document) checkEntryCoverage(entries []Entry, skill *skillgraph.Skill) error {
	imported := skill.Entries()
	for position, parsed := range entries {
		isImported := position < len(imported) && imported[position].Entry.Id == parsed.Id
		if !isImported {
			return d.lineError("document.checkEntryCoverage", parsed.Line, fmt.Sprintf("Entry %s sits outside every block the model holds", parsed.Id))
		}
	}
	return nil
}

func (d *document) continuesArgsBullets(end, index int) bool {
	line := d.lines[index]
	if strings.HasPrefix(line, "- ") || isIndentedText(line) {
		return true
	}

	isBlank := strings.TrimSpace(line) == ""
	hasNextBullet := index+1 < end && strings.HasPrefix(d.lines[index+1], "- ")
	return isBlank && hasNextBullet
}

func (d *document) entryBlock(end int, parsed *Entry) (*skillgraph.Block, int, error) {
	if parsed.End > end {
		return nil, 0, d.lineError("document.entryBlock", parsed.Line, fmt.Sprintf("Entry %s runs past its section", parsed.Id))
	}

	headline := parsed.Line
	for headline < parsed.End && !strings.HasPrefix(d.lines[headline], entryHeadlinePrefix) {
		headline++
	}
	if headline == parsed.End {
		return nil, 0, d.lineError("document.entryBlock", parsed.Line, fmt.Sprintf("Entry %s span holds no headline", parsed.Id))
	}

	match := EntryPattern.FindStringSubmatch(d.lines[headline])
	if match == nil || strings.TrimSpace(match[6]) != "" {
		return nil, 0, d.lineError("document.entryBlock", headline, "Headline carries text after the title")
	}

	if headline-parsed.Line != clauseCount(parsed) {
		return nil, 0, d.lineError("document.entryBlock", parsed.Line, "Clause stack above the headline the model cannot hold")
	}

	role, channel, err := d.headlineTags(headline, parsed)
	if err != nil {
		return nil, 0, err
	}

	entry := &skillgraph.Entry{
		Id:        parsed.Id,
		Channel:   channel,
		Custom:    parsed.Custom,
		Post:      splitPost(parsed.Post),
		Pred:      parsed.Pred,
		Role:      role,
		Statement: normalizeSpace(parsed.Statement),
		Title:     parsed.Title,
	}
	_, statementEnd, _ := statementSpan(d.lines, headline)
	if err := d.entryItems(parsed.End, entry, statementEnd); err != nil {
		return nil, 0, err
	}

	if err := d.payload(entry, headline); err != nil {
		return nil, 0, err
	}

	block := &skillgraph.Block{Entry: entry}
	return block, parsed.End, nil
}

func (d *document) entryItems(end int, entry *skillgraph.Entry, start int) error {
	for index := start; index < end; index++ {
		line := d.lines[index]
		marker := fenceRun(line)
		switch {
		case strings.TrimSpace(line) == "":
		case marker == "":
			if err := d.bulletLine(entry, index); err != nil {
				return err
			}
		default:
			fence, after, err := d.fence(end, index, marker)
			if err != nil {
				return err
			}

			item := skillgraph.Item{Fence: fence}
			entry.Items = append(entry.Items, item)
			index = after - 1
		}
	}
	for position := range entry.Items {
		labelItem(&entry.Items[position])
	}
	return nil
}

// fence closes on the first line equal to its backtick run, the parser's payload-fence rule.
func (d *document) fence(end, index int, marker string) (*skillgraph.Fence, int, error) {
	after, isClosed := payloadFenceEnd(d.lines, index)
	if !isClosed || after > end {
		return nil, 0, d.lineError("document.fence", index, "Fence is not closed inside its entry")
	}

	opening := d.lines[index]
	fence := &skillgraph.Fence{
		Body:   fenceBody(d.lines[index+1 : after-1]),
		Info:   opening[len(marker):],
		Marker: marker,
	}
	return fence, after, nil
}

// frontmatter appends every continuation line, indented or empty, to Value after a newline.
func (d *document) frontmatter() ([]skillgraph.Field, int, error) {
	if len(d.lines) == 0 || strings.TrimSpace(d.lines[0]) != frontmatterFence {
		return nil, 0, d.lineError("document.frontmatter", 0, "No frontmatter")
	}

	var fields []skillgraph.Field
	for index := 1; index < len(d.lines); index++ {
		line := d.lines[index]
		match := frontmatterKeyPattern.FindStringSubmatch(line)
		switch {
		case strings.TrimSpace(line) == frontmatterFence:
			return fields, index + 1, nil
		case match != nil:
			field := skillgraph.Field{Key: match[1], Value: match[2]}
			fields = append(fields, field)
		case len(fields) > 0 && isFrontmatterContinuation(line):
			fields[len(fields)-1].Value += "\n" + line
		default:
			return nil, 0, d.lineError("document.frontmatter", index, "Frontmatter line is neither a key nor a continuation")
		}
	}
	return nil, 0, d.lineError("document.frontmatter", 0, "Frontmatter is never closed")
}

// headlineTags accepts only role, channel, custom — the only order the renderer writes.
func (d *document) headlineTags(index int, parsed *Entry) (string, string, error) {
	role, channel := "", ""
	for tag := range strings.SplitSeq(parsed.Class, "+") {
		switch {
		case EntryRoles[tag] && role == "":
			role = tag
		case EntryChannels[tag] && channel == "":
			channel = tag
		}
	}

	canonical := []string{role}
	if channel != "" {
		canonical = append(canonical, channel)
	}

	canonical = append(canonical, parsed.Custom...)
	if role == "" || strings.Join(canonical, "+") != parsed.Class {
		return "", "", d.lineError("document.headlineTags", index, fmt.Sprintf("Tag slot %q is not [role][channel][custom…]", parsed.Class))
	}
	return role, channel, nil
}

// lineError names the 1-based file line a failure sits on.
func (d *document) lineError(component string, index int, message string) error {
	return fmt.Errorf("%s: %s:%d: %s", component, skillgraph.FileManifest, index+1, message)
}

// payload requires exactly one fence, as the entry's last item.
func (d *document) payload(entry *skillgraph.Entry, headline int) error {
	if entry.Role != skillgraph.RolePayload {
		return nil
	}

	fences := 0
	for _, item := range entry.Items {
		if item.Fence != nil {
			fences++
		}
	}
	last := len(entry.Items) - 1
	if fences != 1 || entry.Items[last].Fence == nil {
		return d.lineError("document.payload", headline, fmt.Sprintf("Payload entry %s holds %d fences, its last item must be the only one", entry.Id, fences))
	}

	fence := entry.Items[last].Fence
	entry.Payload = &skillgraph.Payload{
		Body:   fence.Body,
		Info:   fence.Info,
		Marker: fence.Marker,
	}
	entry.Items = entry.Items[:last]
	if last == 0 {
		entry.Items = nil
	}
	return nil
}

// prose keeps every non-blank line verbatim, trailing spaces included.
func (d *document) prose(end, start int) string {
	var kept []string
	openFence := ""
	hasPendingBlank := false
	for _, line := range d.lines[start:end] {
		isBlankOutsideFence := openFence == "" && strings.TrimSpace(line) == ""
		if isBlankOutsideFence {
			hasPendingBlank = len(kept) > 0
			continue
		}

		if hasPendingBlank {
			kept = append(kept, "")
			hasPendingBlank = false
		}

		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func (d *document) section(end, heading int) (*skillgraph.Section, error) {
	title := strings.TrimSpace(strings.TrimPrefix(d.lines[heading], "## "))
	section := &skillgraph.Section{Title: title}
	start := heading + 1
	if start < end && strings.HasPrefix(d.lines[start], sectionClausePrefix) {
		if err := d.sectionClause(start, section); err != nil {
			return nil, err
		}

		start++
	}

	blocks, err := d.blocks(end, start, title)
	if err != nil {
		return nil, err
	}

	section.Blocks = blocks
	return section, nil
}

func (d *document) sectionClause(index int, section *skillgraph.Section) error {
	match := sectionClausePattern.FindStringSubmatch(d.lines[index])
	if match == nil || match[5] != "" {
		return d.lineError("document.sectionClause", index, "Section clause line the model cannot hold")
	}

	switch match[1] {
	case skillgraph.SectionKindContext, skillgraph.SectionKindPhase, skillgraph.SectionKindRoute:
	default:
		return d.lineError("document.sectionClause", index, fmt.Sprintf("Unknown section kind %q", match[1]))
	}

	section.Kind = match[1]
	section.Pred = strings.TrimSpace(match[2])
	switch match[3] {
	case anchorReplaces:
		section.Replaces = strings.TrimSpace(match[4])
	case anchorBetween:
		section.Between = strings.TrimSpace(match[4])
	}
	return nil
}

// sectionHeadings skips fenced lines like the parser, so a payload template's "## " never splits a section.
func (d *document) sectionHeadings(start int) []int {
	var headings []int
	openFence := ""
	for index := start; index < len(d.lines); index++ {
		line := d.lines[index]
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(openFence, run, line)
			continue
		}

		if openFence == "" && strings.HasPrefix(line, "## ") {
			headings = append(headings, index)
		}
	}
	return headings
}

// clauseCount returns how many formal clause lines stack above the headline.
func clauseCount(parsed *Entry) int {
	count := 0
	if parsed.Pred != "" {
		count++
	}

	if parsed.Post != "" {
		count++
	}
	return count
}

// fenceBody produces the payloads/<entry-id>.md byte form.
func fenceBody(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// hasFile lets any error other than not-exist surface when the file is read.
func hasFile(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func isArgsAlignment(line string) bool {
	stripped, _ := strip(line)
	return stripped == argsAlignmentStripped || line == legacyArgsAlignment
}

func isFrontmatterContinuation(line string) bool {
	return line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func isIndentedText(line string) bool {
	isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
	return isIndented && strings.TrimSpace(line) != ""
}

func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

func labelItem(item *skillgraph.Item) {
	if item.Fence != nil {
		return
	}

	text := normalizeSpace(item.Text)
	label, rest, hasLabel := strings.Cut(text, ": ")
	if hasLabel && itemLabelPattern.MatchString(label) {
		item.Label, text = label, rest
	}
	item.Text = text
}

func normalizeSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func parseSkill(data []byte) (*skillgraph.Skill, error) {
	doc := newDocument(data)
	frontmatter, bodyStart, err := doc.frontmatter()
	if err != nil {
		return nil, err
	}

	entries, _ := ParseEntries(doc.lines)
	doc.spans = make(map[int]Entry, len(entries))
	for _, entry := range entries {
		doc.spans[entry.Line] = entry
	}

	headings := doc.sectionHeadings(bodyStart)
	preambleEnd := len(doc.lines)
	if len(headings) > 0 {
		preambleEnd = headings[0]
	}

	preamble := doc.prose(preambleEnd, bodyStart)
	skill := &skillgraph.Skill{Preamble: preamble}
	skill.Frontmatter = takeRuntimeFields(frontmatter, skill)
	for position, heading := range headings {
		sectionEnd := len(doc.lines)
		if position+1 < len(headings) {
			sectionEnd = headings[position+1]
		}

		section, err := doc.section(sectionEnd, heading)
		if err != nil {
			return nil, err
		}

		skill.Sections = append(skill.Sections, section)
	}
	if err := doc.checkEntryCoverage(entries, skill); err != nil {
		return nil, err
	}
	return skill, nil
}

// readChangelog rejects an unknown key so no row field is dropped.
func readChangelog(path string) ([]skillgraph.ChangelogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readChangelog: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var changelog []skillgraph.ChangelogEntry
	if err := decoder.Decode(&changelog); err != nil {
		return nil, fmt.Errorf("readChangelog: %s: %w", path, err)
	}
	return changelog, nil
}

// splitPost splits a post clause into its granted atoms on " and ".
func splitPost(clause string) []string {
	var atoms []string
	for part := range strings.SplitSeq(clause, " and ") {
		if atom := strings.TrimSpace(part); atom != "" {
			atoms = append(atoms, atom)
		}
	}
	return atoms
}

// takeRuntimeFields moves the rendered placement lines into the skill's typed fields and returns the authored rest.
func takeRuntimeFields(fields []skillgraph.Field, skill *skillgraph.Skill) []skillgraph.Field {
	authored := make([]skillgraph.Field, 0, len(fields))
	for _, field := range fields {
		value := strings.TrimSpace(field.Value)
		switch field.Key {
		case skillgraph.FieldAnsweredBy:
			skill.AnsweredBy = value
		case skillgraph.FieldDocker:
			skill.Docker = value
		case skillgraph.FieldEnableDelegation:
			skill.EnableDelegation = value
		case skillgraph.FieldEnforcePermissions:
			skill.EnforcePermissions = strings.Fields(strings.ReplaceAll(value, ",", " "))
		default:
			authored = append(authored, field)
		}
	}
	return authored
}

// Import strips the working-copy projection block; Vocabulary stays nil for Load to merge.
func Import(dir string) (*skillgraph.Skill, error) {
	path := filepath.Join(dir, skillgraph.FileManifest)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Import: %w", err)
	}

	skill, err := parseSkill(acdsl.StripProjection(path, data))
	if err != nil {
		return nil, fmt.Errorf("Import: %s: %w", dir, err)
	}

	atomsPath := filepath.Join(dir, atomsFile)
	if hasFile(atomsPath) {
		local, err := ImportVocabulary(atomsPath)
		if err != nil {
			return nil, fmt.Errorf("Import: %s: %w", dir, err)
		}

		skill.Atoms = local.Atoms
		skill.Exclusive = local.Exclusive
	}

	changelogPath := filepath.Join(dir, changelogFile)
	if hasFile(changelogPath) {
		changelog, err := readChangelog(changelogPath)
		if err != nil {
			return nil, fmt.Errorf("Import: %s: %w", dir, err)
		}

		skill.Changelog = changelog
	}
	return skill, nil
}
