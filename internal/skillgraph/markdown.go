package skillgraph

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	argsTableAlignment     = "| :-- | :--- | :------ | :----- |"
	argsTableHeader        = "| Arg | Form | Default | Effect |"
	entryClauseForbidden   = "}`"
	frontmatterFence       = "---"
	lineWidth              = 128
	nestIndent             = "  "
	routesTableAlignment   = "| :--- | :--- |"
	routesTableHeader      = "| For | Use |"
	sectionClauseForbidden = ",}`"
)

var (
	fenceMarkerPattern = regexp.MustCompile("^`{3,}$")
	// preambleBreakerPattern and sectionProseBreakerPattern match prose lines a block owns.
	preambleBreakerPattern     = regexp.MustCompile(`^(## |\*\*SKILL-)`)
	sectionProseBreakerPattern = regexp.MustCompile(`^(## |### |\*\*SKILL-)`)
	// statementBreakerPattern matches the line starts at which the entry parser ends a statement.
	statementBreakerPattern = regexp.MustCompile("^(\\* |- |#|```|\\*\\*SKILL-)")
	tagPattern              = regexp.MustCompile(`^[a-z][a-z0-9:-]*$`)
	unescapedPipePattern    = regexp.MustCompile(`(^|[^\\])\|`)
)

func argLines(arg *Arg, isBullet bool, skill *Skill) ([]string, error) {
	if err := checkInline("`", arg.Name); err != nil {
		return nil, fmt.Errorf("argLines: Name: %w", err)
	}

	name := "`" + arg.Name + "`"
	effect := tokenText(skill, arg.Effect)
	if !isBullet {
		cells := []string{name, arg.Form, tokenText(skill, arg.Default), effect}
		if err := checkCells(cells); err != nil {
			return nil, fmt.Errorf("argLines: %s: %w", arg.Name, err)
		}

		return []string{tableRow(cells)}, nil
	}

	hasTableCells := arg.Form != "" || arg.Default != ""
	if hasTableCells {
		return nil, fmt.Errorf("argLines: %s: The bullet fallback carries no form or default", arg.Name)
	}

	if err := checkNormalized(effect); err != nil {
		return nil, fmt.Errorf("argLines: %s: %w", arg.Name, err)
	}

	return wrapText(nestIndent, "", "- "+name+": "+effect), nil
}

func blockKindCount(block *Block) int {
	kinds := []bool{block.Args != nil, block.Entry != nil, block.Heading != "", block.Model != nil, block.Prose != "", block.Routing != nil}
	count := 0
	for _, isSet := range kinds {
		if isSet {
			count++
		}
	}
	return count
}

// bulletLines rejects a wrap that starts a line like a bullet.
func bulletLines(text string) ([]string, error) {
	if err := checkNormalized(text); err != nil {
		return nil, fmt.Errorf("bulletLines: %w", err)
	}

	lines := wrapText(nestIndent, "", "- "+text)
	for _, line := range lines[1:] {
		continuation := strings.TrimLeft(line, " ")
		if strings.HasPrefix(continuation, "* ") || strings.HasPrefix(continuation, "- ") {
			return nil, fmt.Errorf("bulletLines: Wrapped line %q starts like a bullet", line)
		}
	}
	return lines, nil
}

func checkCells(cells []string) error {
	for _, cell := range cells {
		isPadded := cell != strings.TrimSpace(cell)
		isSplit := strings.Contains(cell, "\n") || unescapedPipePattern.MatchString(cell)
		if isPadded || isSplit {
			return fmt.Errorf("checkCells: Cell %q carries a line break, an unescaped pipe or surrounding whitespace", cell)
		}
	}
	return nil
}

// checkInline rejects surrounding whitespace because the parser trims it.
func checkInline(forbidden, value string) error {
	isPadded := value != strings.TrimSpace(value)
	if value == "" || isPadded {
		return fmt.Errorf("checkInline: Value %q is empty or padded", value)
	}

	if strings.ContainsAny(value, "\n"+forbidden) {
		return fmt.Errorf("checkInline: Value %q carries a line break or one of %q", value, forbidden)
	}
	return nil
}

// checkNormalized rejects text the greedy wrap cannot reproduce.
func checkNormalized(text string) error {
	if text == "" || text != strings.Join(strings.Fields(text), " ") {
		return fmt.Errorf("checkNormalized: Text %q is empty or not single-space normalized", text)
	}
	return nil
}

// checkVerbatim rejects prose the layout cannot place verbatim between blank-line separators.
func checkVerbatim(breakers *regexp.Regexp, text string) error {
	lines := strings.Split(text, "\n")
	isEdgeBlank := strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[len(lines)-1]) == ""
	if isEdgeBlank {
		return errors.New("checkVerbatim: Prose starts or ends with a blank line")
	}

	openFence := ""
	isPreviousBlank := false
	for _, line := range lines {
		if run := fenceRun(line); run != "" {
			openFence = toggleFence(line, openFence, run)
			isPreviousBlank = false
			continue
		}
		if openFence != "" {
			continue
		}

		isBlank := strings.TrimSpace(line) == ""
		if isBlank && isPreviousBlank {
			return errors.New("checkVerbatim: Prose holds a run of blank lines")
		}

		if breakers.MatchString(line) {
			return fmt.Errorf("checkVerbatim: Prose line %q is a heading or entry headline", line)
		}

		isPreviousBlank = isBlank
	}
	if openFence != "" {
		return fmt.Errorf("checkVerbatim: Prose leaves the %s fence open", openFence)
	}
	return nil
}

func clausePart(name, value string) (string, error) {
	if value == "" {
		return "", nil
	}

	if err := checkInline(sectionClauseForbidden, value); err != nil {
		return "", fmt.Errorf("clausePart: %s: %w", name, err)
	}
	return ", " + name + ": " + value, nil
}

// disableMatches returns the ids of the entries a disable token names.
func disableMatches(entries []EntryRef, token string) []string {
	var ids []string
	for _, ref := range entries {
		if disableTokenMatches(ref.Entry.Id, token) {
			ids = append(ids, ref.Entry.Id)
		}
	}
	return ids
}

func disableTokenMatches(id, token string) bool {
	if id == token {
		return true
	}
	if !strings.HasSuffix(token, "*") {
		return false
	}
	return strings.HasPrefix(id, strings.TrimSuffix(token, "*"))
}

func edgeList(edges []Edge, kind string) string {
	var items []string
	for _, edge := range edges {
		if edge.Kind != kind {
			continue
		}

		item := "/" + edge.Skill
		if edge.Route != "" {
			item += " " + edge.Route
		}
		if edge.Optional {
			item += " *(optional)*"
		}
		items = append(items, item)
	}
	return strings.Join(items, ", ")
}

// entryBody renders a field bullet or item reading a deferred name as its own placeholder block, spaced like a fence; nil deferred renders every one as text.
func entryBody(deferred *deferral, entry *Entry, skill *Skill) ([]string, error) {
	isPayloadRole := entry.Role == RolePayload
	if isPayloadRole != (entry.Payload != nil) {
		return nil, errors.New("entryBody: A payload fence belongs to exactly the payload role")
	}

	var lines []string
	needsBlank := true
	if entry.Statement != "" {
		statement, err := statementLines(tokenText(skill, entry.Statement))
		if err != nil {
			return nil, err
		}

		lines = append(lines, "")
		lines = append(lines, statement...)
		needsBlank = false
	}
	fieldBullets, err := renderFields(skill, entry)
	if err != nil {
		return nil, fmt.Errorf("entryBody: %w", err)
	}

	for _, fieldLines := range fieldBullets {
		isBlock := deferred != nil && len(deferred.readNames(fieldLines)) > 0
		if isBlock {
			fieldLines = deferred.block(fieldLines)
		}
		if needsBlank || isBlock {
			lines = append(lines, "")
		}
		lines = append(lines, fieldLines...)
		needsBlank = isBlock
	}
	for index := range entry.Items {
		item := &entry.Items[index]
		isFence := item.Fence != nil
		if isFence && isPayloadRole {
			return nil, errors.New("entryBody: A payload entry weaves no fence before its payload")
		}

		itemLines, err := renderItem(skill, item)
		if err != nil {
			return nil, fmt.Errorf("entryBody: Item %d: %w", index+1, err)
		}

		isBlock := deferred != nil && len(deferred.readNames(itemLines)) > 0
		if isBlock {
			itemLines = deferred.block(itemLines)
		}
		isFenced := isFence || isBlock
		if needsBlank || isFenced {
			lines = append(lines, "")
		}
		lines = append(lines, itemLines...)
		needsBlank = isFenced
	}
	if entry.Payload == nil {
		return lines, nil
	}

	fence, err := renderFence(entry.Payload.Body, entry.Payload.Info, entry.Payload.Marker)
	if err != nil {
		return nil, fmt.Errorf("entryBody: Payload: %w", err)
	}

	lines = append(lines, "")
	return append(lines, fence...), nil
}

// entryClauseLines never wraps clause lines; each ends in the markdown hard break.
func entryClauseLines(entry *Entry) ([]string, error) {
	var lines []string
	if entry.Pred != "" {
		if err := checkInline(entryClauseForbidden, entry.Pred); err != nil {
			return nil, fmt.Errorf("entryClauseLines: Pred: %w", err)
		}

		lines = append(lines, "`{pred: "+entry.Pred+"}`\\")
	}
	if len(entry.Post) == 0 {
		return lines, nil
	}

	for _, atom := range entry.Post {
		if err := checkInline(entryClauseForbidden+" ", atom); err != nil {
			return nil, fmt.Errorf("entryClauseLines: Post: %w", err)
		}
	}
	return append(lines, "`{post: "+strings.Join(entry.Post, " and ")+"}`\\"), nil
}

func entryHeadline(entry *Entry) (string, error) {
	if err := checkInline("*` ", entry.Id); err != nil {
		return "", fmt.Errorf("entryHeadline: Id: %w", err)
	}

	if err := checkInline("*", entry.Title); err != nil {
		return "", fmt.Errorf("entryHeadline: Title: %w", err)
	}

	tags := []string{entry.Role}
	if entry.Channel != "" {
		tags = append(tags, entry.Channel)
	}
	tags = append(tags, entry.Custom...)
	var slot strings.Builder
	for _, tag := range tags {
		if !tagPattern.MatchString(tag) {
			return "", fmt.Errorf("entryHeadline: Tag %q is not [a-z][a-z0-9:-]*", tag)
		}
		slot.WriteString("[" + tag + "]")
	}
	return "**" + entry.Id + "** `" + slot.String() + "` **" + entry.Title + "**", nil
}

func fenceBodyLines(body string) []string {
	if body == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
}

// fenceRun returns the leading backtick run of a fence line, or "".
func fenceRun(line string) string {
	if !strings.HasPrefix(line, "```") {
		return ""
	}
	return line[:len(line)-len(strings.TrimLeft(line, "`"))]
}

// isFrontmatterFence reports whether a line would close the frontmatter.
func isFrontmatterFence(line string) bool {
	return strings.TrimSpace(line) == frontmatterFence
}

func isScopeEmptied(blocks []Block, heading int, isRemoved []bool) bool {
	hasLost := false
	for index := heading + 1; index < len(blocks) && blocks[index].Heading == ""; index++ {
		if !isRemoved[index] {
			return false
		}
		hasLost = true
	}
	return hasLost
}

// keptBlocks also drops a "### " heading whose scope the filter emptied.
func keptBlocks(blocks []Block, disabled map[string]bool) []int {
	isRemoved := make([]bool, len(blocks))
	for index := range blocks {
		entry := blocks[index].Entry
		isRemoved[index] = entry != nil && disabled[entry.Id]
	}
	for index := range blocks {
		isEmptiedHeading := blocks[index].Heading != "" && isScopeEmptied(blocks, index, isRemoved)
		isRemoved[index] = isRemoved[index] || isEmptiedHeading
	}

	var kept []int
	for index := range blocks {
		if !isRemoved[index] {
			kept = append(kept, index)
		}
	}
	return kept
}

func pipelineText(pipeline *Pipeline, skillName string) (string, error) {
	var parts []string
	if pipeline.Standalone {
		parts = append(parts, "standalone")
	}

	fromList, toList := edgeList(pipeline.Edges, EdgeKindFrom), edgeList(pipeline.Edges, EdgeKindTo)
	if fromList != "" || toList != "" {
		chain := "**" + skillName + "**"
		if fromList != "" {
			chain = fromList + " → " + chain
		}
		if toList != "" {
			chain += " → " + toList
		}
		parts = append(parts, chain)
	}
	if spawnedBy := edgeList(pipeline.Edges, EdgeKindSpawnedBy); spawnedBy != "" {
		parts = append(parts, "spawned by "+spawnedBy)
	}
	if spawns := edgeList(pipeline.Edges, EdgeKindSpawns); spawns != "" {
		parts = append(parts, "spawns "+spawns)
	}
	if escalates := edgeList(pipeline.Edges, EdgeKindEscalates); escalates != "" {
		parts = append(parts, "escalates to "+escalates)
	}

	text := strings.Join(parts, "; ")
	if pipeline.Note == "" {
		return text, nil
	}

	if err := checkNormalized(pipeline.Note); err != nil {
		return "", fmt.Errorf("pipelineText: Note: %w", err)
	}

	if text == "" {
		return pipeline.Note, nil
	}
	return text + " — " + pipeline.Note, nil
}

func renderArgs(args *ArgsBlock, skill *Skill) ([]string, error) {
	var lines []string
	if !args.Bullets {
		lines = append(lines, argsTableHeader, argsTableAlignment)
	}
	for index := range args.Rows {
		rowLines, err := argLines(&args.Rows[index], args.Bullets, skill)
		if err != nil {
			return nil, fmt.Errorf("renderArgs: Row %d: %w", index+1, err)
		}

		lines = append(lines, rowLines...)
	}
	return lines, nil
}

// renderBlock renders one block by its single set kind.
func renderBlock(deferred *deferral, block *Block, deployed bool, skill *Skill) ([]string, error) {
	if blockKindCount(block) != 1 {
		return nil, errors.New("renderBlock: A block sets exactly one of args, entry, heading, model, prose and routing")
	}

	switch {
	case block.Args != nil:
		return renderArgs(block.Args, skill)
	case block.Entry != nil:
		return renderEntry(deferred, deployed, block.Entry, skill)
	case block.Model != nil:
		return renderModel(block.Model)
	case block.Routing != nil:
		return renderRouting(skill, block.Routing)
	case block.Heading != "":
		if err := checkInline("", block.Heading); err != nil {
			return nil, fmt.Errorf("renderBlock: Heading: %w", err)
		}

		return []string{"### " + block.Heading}, nil
	}
	if err := checkVerbatim(sectionProseBreakerPattern, block.Prose); err != nil {
		return nil, fmt.Errorf("renderBlock: Prose: %w", err)
	}

	return strings.Split(block.Prose, "\n"), nil
}

// renderEntry validates in both modes; clause lines render in canonical mode only; a statement or payload reading a deferred name makes the whole entry one placeholder block.
func renderEntry(deferred *deferral, deployed bool, entry *Entry, skill *Skill) ([]string, error) {
	clauses, err := entryClauseLines(entry)
	if err != nil {
		return nil, fmt.Errorf("renderEntry: %s: %w", entry.Id, err)
	}

	headline, err := entryHeadline(entry)
	if err != nil {
		return nil, fmt.Errorf("renderEntry: %s: %w", entry.Id, err)
	}

	statementTexts := []string{entry.Statement}
	if entry.Payload != nil {
		statementTexts = append(statementTexts, entry.Payload.Body)
	}
	isEntryBlock := deferred != nil && len(deferred.readNames(statementTexts)) > 0
	itemDeferral := deferred
	if isEntryBlock {
		itemDeferral = nil
	}
	body, err := entryBody(itemDeferral, entry, skill)
	if err != nil {
		return nil, fmt.Errorf("renderEntry: %s: %w", entry.Id, err)
	}

	var lines []string
	if !deployed {
		lines = append(lines, clauses...)
	}
	lines = append(lines, headline)
	lines = append(lines, body...)
	if !isEntryBlock {
		return lines, nil
	}
	return deferred.block(lines), nil
}

// renderFence rejects a body line the entry parser or CommonMark reads as the closing fence.
func renderFence(body, info, marker string) ([]string, error) {
	if !fenceMarkerPattern.MatchString(marker) {
		return nil, fmt.Errorf("renderFence: Marker %q is not a run of 3 or more backticks", marker)
	}

	if strings.ContainsAny(info, "`\n") {
		return nil, fmt.Errorf("renderFence: Info %q carries a backtick or line break", info)
	}

	isUnterminated := body != "" && !strings.HasSuffix(body, "\n")
	if isUnterminated {
		return nil, errors.New("renderFence: Body does not end in a newline")
	}

	bodyLines := fenceBodyLines(body)
	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		isClosing := fenceMarkerPattern.MatchString(trimmed) && len(trimmed) >= len(marker)
		if isClosing {
			return nil, fmt.Errorf("renderFence: Body line %q closes the %s fence early", line, marker)
		}
	}
	lines := []string{marker + info}
	lines = append(lines, bodyLines...)
	return append(lines, marker), nil
}

// renderFields renders a format-3 entry's fields as labelled bullets in key order, one wrapped bullet per element; nil fields render nothing.
func renderFields(skill *Skill, entry *Entry) ([][]string, error) {
	texts := fieldTexts(entry, func(id string) string { return RenderRef(skill, id) })
	var wrapped [][]string
	for _, text := range texts {
		wrapped = append(wrapped, wrapText(nestIndent, "", "* "+text))
	}
	return wrapped, nil
}

// renderFrontmatter: Value carries everything after the colon; the runtime placement lines follow the authored fields.
func renderFrontmatter(skill *Skill) ([]string, error) {
	lines := []string{frontmatterFence}
	for index := range skill.Frontmatter {
		field := &skill.Frontmatter[index]
		if err := checkInline(": ", field.Key); err != nil {
			return nil, fmt.Errorf("renderFrontmatter: Key: %w", err)
		}

		group := strings.Split(field.Key+":"+field.Value, "\n")
		if slices.ContainsFunc(group, isFrontmatterFence) {
			return nil, fmt.Errorf("renderFrontmatter: Value of %s holds a %s line", field.Key, frontmatterFence)
		}

		lines = append(lines, group...)
	}
	lines = append(lines, runtimeLines(skill)...)
	return append(lines, frontmatterFence), nil
}

func renderItem(skill *Skill, item *Item) ([]string, error) {
	if item.Fence != nil {
		hasBulletField := item.Text != "" || item.Label != "" || item.Marker != "" || item.Depth != 0
		if hasBulletField {
			return nil, errors.New("renderItem: An item is either a fence or a bullet, never both")
		}

		return renderFence(item.Fence.Body, item.Fence.Info, item.Fence.Marker)
	}

	isKnownMarker := item.Marker == "*" || item.Marker == "-"
	if !isKnownMarker || item.Depth < 0 {
		return nil, fmt.Errorf("renderItem: Marker %q at depth %d is no bullet", item.Marker, item.Depth)
	}

	text := tokenText(skill, item.Text)
	if err := checkNormalized(text); err != nil {
		return nil, fmt.Errorf("renderItem: %w", err)
	}

	head := item.Marker + " "
	if item.Label != "" {
		isBadLabel := strings.Contains(item.Label, ":") || checkNormalized(item.Label) != nil
		if isBadLabel {
			return nil, fmt.Errorf("renderItem: Label %q holds a colon or irregular whitespace", item.Label)
		}

		head += item.Label + ": "
	}
	indent := strings.Repeat(nestIndent, item.Depth)
	lines := wrapText(indent+nestIndent, indent, head+text)
	for _, line := range lines[1:] {
		continuation := strings.TrimLeft(line, " ")
		if strings.HasPrefix(continuation, "* ") || strings.HasPrefix(continuation, "- ") {
			return nil, fmt.Errorf("renderItem: Wrapped line %q starts like a bullet", line)
		}
	}
	return lines, nil
}

// renderMarkdown renders every section in both modes, so a model the layout cannot express fails either way.
func renderMarkdown(deferred *deferral, deployed bool, disabled map[string]bool, skill *Skill) ([]byte, error) {
	lines, err := renderFrontmatter(skill)
	if err != nil {
		return nil, fmt.Errorf("renderMarkdown: %w", err)
	}

	var parts [][]string
	if skill.Preamble != "" {
		if err := checkVerbatim(preambleBreakerPattern, skill.Preamble); err != nil {
			return nil, fmt.Errorf("renderMarkdown: Preamble: %w", err)
		}

		parts = append(parts, strings.Split(skill.Preamble, "\n"))
	}
	for _, section := range skill.Sections {
		sectionLines, err := renderSection(deferred, deployed, disabled, section, skill)
		if err != nil {
			return nil, fmt.Errorf("renderMarkdown: Section %q: %w", section.Title, err)
		}

		if sectionLines == nil {
			continue
		}
		parts = append(parts, sectionLines)
	}

	for _, part := range parts {
		lines = append(lines, "")
		lines = append(lines, part...)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// renderModel renders the FORMAT-006 field bullets.
func renderModel(model *Model) ([]string, error) {
	suggested := []string{model.Tier + " / " + model.Effort}
	for _, route := range model.Routes {
		suggested = append(suggested, route.Route+" route: "+route.Tier+" / "+route.Effort)
	}

	fields := []string{"Suggested: " + strings.Join(suggested, "; "), "Reason: " + model.Reason}
	switch len(model.Unviable) {
	case 0:
		fields = append(fields, "Tested unviable: — (none yet)")
	case 1:
		fields = append(fields, "Tested unviable: "+model.Unviable[0])
	}

	var lines []string
	for _, field := range fields {
		fieldLines, err := bulletLines(field)
		if err != nil {
			return nil, fmt.Errorf("renderModel: %w", err)
		}

		lines = append(lines, fieldLines...)
	}
	if len(model.Unviable) > 1 {
		lines = append(lines, "- Tested unviable:")
		for _, unviable := range model.Unviable {
			if err := checkNormalized(unviable); err != nil {
				return nil, fmt.Errorf("renderModel: Unviable: %w", err)
			}

			lines = append(lines, wrapText(nestIndent+nestIndent, nestIndent, "- "+unviable)...)
		}
	}
	for _, note := range model.Notes {
		noteLines, err := bulletLines(note)
		if err != nil {
			return nil, fmt.Errorf("renderModel: Note: %w", err)
		}

		lines = append(lines, noteLines...)
	}
	return lines, nil
}

// renderRouting follows the layout the skillhelp hook parses.
func renderRouting(skill *Skill, routing *Routing) ([]string, error) {
	lines, err := statementLines("**Purpose:** " + tokenText(skill, routing.Purpose))
	if err != nil {
		return nil, fmt.Errorf("renderRouting: Purpose: %w", err)
	}

	table, err := renderRoutes(routing.Routes, skill)
	if err != nil {
		return nil, fmt.Errorf("renderRouting: %w", err)
	}

	lines = append(lines, "", "**Disambiguation:**", "")
	lines = append(lines, table...)
	lines = append(lines, "", "**Preconditions:**")
	for index, precondition := range routing.Preconditions {
		preconditionLines, err := bulletLines(tokenText(skill, precondition))
		if err != nil {
			return nil, fmt.Errorf("renderRouting: Precondition %d: %w", index+1, err)
		}

		lines = append(lines, preconditionLines...)
	}

	text, err := pipelineText(&routing.Pipeline, skill.Name())
	if err != nil {
		return nil, fmt.Errorf("renderRouting: %w", err)
	}

	pipeline, err := statementLines("**Pipeline:** " + text)
	if err != nil {
		return nil, fmt.Errorf("renderRouting: Pipeline: %w", err)
	}

	lines = append(lines, "")
	return append(lines, pipeline...), nil
}

// renderRoutes renders the Routing Disambiguation table.
func renderRoutes(routes []Route, skill *Skill) ([]string, error) {
	lines := []string{routesTableHeader, routesTableAlignment}
	for index := range routes {
		cells := []string{routes[index].For, tokenText(skill, routes[index].Use)}
		if err := checkCells(cells); err != nil {
			return nil, fmt.Errorf("renderRoutes: Row %d: %w", index+1, err)
		}

		lines = append(lines, tableRow(cells))
	}
	return lines, nil
}

// renderSection validates every block even when the deployed filter drops it.
func renderSection(deferred *deferral, deployed bool, disabled map[string]bool, section *Section, skill *Skill) ([]string, error) {
	if err := checkInline("", section.Title); err != nil {
		return nil, fmt.Errorf("renderSection: Title: %w", err)
	}

	clause, err := sectionClauseLine(section)
	if err != nil {
		return nil, err
	}

	rendered := make([][]string, len(section.Blocks))
	for index := range section.Blocks {
		blockLines, err := renderBlock(deferred, &section.Blocks[index], deployed, skill)
		if err != nil {
			return nil, fmt.Errorf("renderSection: Block %d: %w", index+1, err)
		}

		rendered[index] = blockLines
	}

	filter := disabled
	if !deployed {
		filter = nil
	}
	kept := keptBlocks(section.Blocks, filter)
	isEmptied := len(kept) == 0 && len(section.Blocks) > 0
	if isEmptied {
		return nil, nil
	}

	lines := []string{"## " + section.Title}
	isClauseKept := !deployed || section.Kind == SectionKindRoute
	if clause != "" && isClauseKept {
		lines = append(lines, clause)
	}
	for _, index := range kept {
		lines = append(lines, "")
		lines = append(lines, rendered[index]...)
	}
	return lines, nil
}

// resolveDisable fails on a token matching nothing; a silent miss would ship the full skill under a variant name.
func resolveDisable(skill *Skill, tokens []string) (map[string]bool, error) {
	entries := skill.Entries()
	disabled := map[string]bool{}
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		ids := disableMatches(entries, token)
		if len(ids) == 0 {
			return nil, fmt.Errorf("resolveDisable: Token %q matches no entry", token)
		}

		for _, id := range ids {
			disabled[id] = true
		}
	}
	return disabled, nil
}

// runtimeLines are the frontmatter lines derived from the runtime placement fields, one per set field.
func runtimeLines(skill *Skill) []string {
	var lines []string
	if skill.AnsweredBy != "" {
		lines = append(lines, FieldAnsweredBy+": "+skill.AnsweredBy)
	}
	if skill.Docker != "" {
		lines = append(lines, FieldDocker+": "+skill.Docker)
	}
	if skill.EnableDelegation != "" {
		lines = append(lines, FieldEnableDelegation+": "+skill.EnableDelegation)
	}
	if len(skill.EnforcePermissions) > 0 {
		lines = append(lines, FieldEnforcePermissions+": "+strings.Join(skill.EnforcePermissions, ", "))
	}
	return lines
}

// sectionClauseLine: a section without a kind carries no pred and no anchor.
func sectionClauseLine(section *Section) (string, error) {
	hasClauseData := section.Pred != "" || section.Replaces != "" || section.Between != ""
	if section.Kind == "" {
		if hasClauseData {
			return "", errors.New("sectionClauseLine: Pred, replaces and between need a section kind")
		}
		return "", nil
	}

	hasBothAnchors := section.Replaces != "" && section.Between != ""
	if hasBothAnchors {
		return "", errors.New("sectionClauseLine: A section anchors by replaces or between, not both")
	}

	if err := checkInline(sectionClauseForbidden+" ", section.Kind); err != nil {
		return "", fmt.Errorf("sectionClauseLine: Kind: %w", err)
	}

	pred, err := clausePart("pred", section.Pred)
	if err != nil {
		return "", err
	}

	replaces, err := clausePart("replaces", section.Replaces)
	if err != nil {
		return "", err
	}

	between, err := clausePart("between", section.Between)
	if err != nil {
		return "", err
	}
	return "`{section: " + section.Kind + pred + replaces + between + "}`", nil
}

// statementLines rejects a wrap the entry parser would read as the statement's end.
func statementLines(statement string) ([]string, error) {
	if err := checkNormalized(statement); err != nil {
		return nil, fmt.Errorf("statementLines: %w", err)
	}

	lines := wrapText("", "", statement)
	for _, line := range lines {
		if statementBreakerPattern.MatchString(line) {
			return nil, fmt.Errorf("statementLines: Wrapped line %q starts like a bullet, heading, fence or headline", line)
		}
	}
	return lines, nil
}

// tableRow renders one table row, cells separated by " | ".
func tableRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

// tokenText expands a format-3 skill's link tokens; below format 3 the text passes unchanged.
func tokenText(skill *Skill, text string) string {
	if entryFormat(skill) < entryFormatFields {
		return text
	}
	return ExpandTokens(skill, text)
}

// toggleFence closes only on the run that opened the fence.
func toggleFence(line, openFence, run string) string {
	if openFence == "" {
		return run
	}
	if strings.TrimSpace(line) == openFence {
		return ""
	}
	return openFence
}

// wrapText is ported from cmd/skillfmt wrapFirst; a word wider than lineWidth sits on its own line.
func wrapText(continuationPrefix, firstPrefix, text string) []string {
	var lines []string
	current := firstPrefix
	isStarted := false
	for word := range strings.FieldsSeq(text) {
		candidate := current
		if isStarted {
			candidate += " "
		}
		candidate += word
		if isStarted && utf8.RuneCountInString(candidate) > lineWidth {
			lines = append(lines, current)
			current = continuationPrefix + word
			continue
		}

		current = candidate
		isStarted = true
	}
	if isStarted {
		lines = append(lines, current)
	}
	return lines
}
