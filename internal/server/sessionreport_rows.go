package server

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	// fixedColumns: subject, agents, chars and tokens.
	fixedColumns = 4
	// The section kinds select a table's columns in sectionCells, the _section_row partial and the markdown.
	sectionFiles       = "files"
	sectionGlobal      = "global"
	sectionHooks       = "hooks"
	sectionProjections = "projections"
	sectionPrompts     = "prompts"
	sectionSkills      = "skills"
)

// agentLine is one agent's share of a section row.
type agentLine struct {
	Agent  string
	Chars  string
	Count  int
	Lines  string
	Tokens tokenCell
}

// rowGroup collects one subject's items while grouping.
type rowGroup struct {
	byAgent map[string][]*contextrecord.Item
	ids     []string
	names   map[string]string
	order   []string
	row     sectionRow
}

func newRowGroup() *rowGroup {
	group := &rowGroup{byAgent: map[string][]*contextrecord.Item{}, names: map[string]string{}}
	return group
}

// add files one item under its agent, keeping first-seen agent order.
func (g *rowGroup) add(agent *contextrecord.Agent, item *contextrecord.Item) {
	if _, isKnown := g.byAgent[agent.Id]; !isKnown {
		g.names[agent.Id] = agentName(agent)
		g.order = append(g.order, agent.Id)
	}
	g.byAgent[agent.Id] = append(g.byAgent[agent.Id], item)
}

// contexts lists the id views of the group's agents that carry one.
func (g *rowGroup) contexts(agents []contextrecord.Agent) []*contextrecord.AgentContext {
	var views []*contextrecord.AgentContext
	for index := range agents {
		agent := &agents[index]
		_, isMember := g.byAgent[agent.Id]
		if isMember && agent.Context != nil {
			views = append(views, agent.Context)
		}
	}
	return views
}

func (g *rowGroup) finish() {
	for _, agentId := range g.order {
		agentItems := g.byAgent[agentId]
		line := agentLine{
			Agent:  g.names[agentId],
			Chars:  groupThousands(int64(sumChars(agentItems))),
			Count:  len(agentItems),
			Lines:  unionLines(agentItems),
			Tokens: sumTokenCell(agentItems),
		}
		g.row.AgentLines = append(g.row.AgentLines, line)
	}

	items := g.items()
	names := make([]string, 0, len(g.order))
	for _, agentId := range g.order {
		names = append(names, g.names[agentId])
	}
	g.row.Agents = agentsLabel(names)
	g.row.Chars = groupThousands(int64(sumChars(items)))
	g.row.Count = len(items)
	g.row.Tokens = sumTokenCell(items)
	g.row.charsValue = sumChars(items)
	g.row.tokensValue = sumTokens(items)

	hasLists := len(g.row.Families)+len(g.row.Paths) > 0
	hasText := g.row.Text != ""
	g.row.HasDetail = hasLists || hasText || len(g.row.AgentLines) > 1
}

// first is the group's first item in transcript order.
func (g *rowGroup) first() *contextrecord.Item {
	return g.byAgent[g.order[0]][0]
}

// items flattens the group's items in agent order.
func (g *rowGroup) items() []*contextrecord.Item {
	var items []*contextrecord.Item
	for _, agentId := range g.order {
		items = append(items, g.byAgent[agentId]...)
	}
	return items
}

func (g *rowGroup) setFamilies(filter contextFilter, index map[string]idMeta) {
	ids := slices.Compact(slices.Sorted(slices.Values(g.ids)))
	chips := filterIds(filter, ids, index)
	g.row.Entries = len(chips)
	g.row.Families = groupIdFamilies(chips, filter.Active())
}

type sectionCellsView struct {
	Cells   []string
	Columns int
	Kind    string
	Row     sectionRow
}

type sectionRow struct {
	charsValue  int
	tokensValue int

	AgentLines []agentLine
	Agents     string
	Chars      string
	Count      int
	Coverage   coverageBadge
	Entries    int
	Families   []idFamily
	HasDetail  bool
	Lines      string
	Paths      []pathCell
	Subject    pathCell
	Tag        mechanismTag
	Text       string
	TextRest   int
	Tokens     tokenCell
	Version    string
}

// agentPromptRows pairs the n-th prompt with the n-th subagent-context entry of the agent.
func agentPromptRows(claudeHome, cwd string, filter contextFilter, index map[string]idMeta, agent *contextrecord.Agent) []sectionRow {
	var inlined []contextrecord.SubagentContext
	if agent.Context != nil {
		inlined = agent.Context.SubagentContext
	}

	var rows []sectionRow
	for itemIndex := range agent.Items {
		item := &agent.Items[itemIndex]
		if item.Kind != contextrecord.KindSubagentPrompt {
			continue
		}

		group := newRowGroup()
		group.add(agent, item)
		group.row.Subject = pathCell{Rel: item.Label}
		group.row.Text, group.row.TextRest = item.Capped()
		if len(rows) < len(inlined) {
			entry := inlined[len(rows)]
			group.ids = entry.Ids
			for _, path := range entry.Paths {
				group.row.Paths = append(group.row.Paths, relativePath(path, cwd, claudeHome))
			}
			group.setFamilies(filter, index)
		}
		group.finish()
		rows = append(rows, group.row)
	}
	return rows
}

func agentsLabel(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%d agents", len(names))
}

func fileReadRows(claudeHome string, filter contextFilter, index map[string]idMeta, record *contextrecord.Record) []sectionRow {
	groups := groupItems(record.Agents, ofKind(contextrecord.KindLang, contextrecord.KindPackRead), itemLabel)
	rows := make([]sectionRow, 0, len(groups))
	for _, group := range groups {
		first := group.first()
		group.row.Subject = relativePath(first.Label, record.Cwd, claudeHome)
		group.row.Lines = unionLines(group.items())
		if first.Kind == contextrecord.KindLang {
			langs := guideLangs(group.contexts(record.Agents), first.Label)
			for _, lang := range langs {
				group.ids = append(group.ids, lang.Ids...)
			}
			group.row.Coverage = scopeCoverage(langs, group.row.Lines)
			group.setFamilies(filter, index)
		}
		group.finish()
		rows = append(rows, group.row)
	}
	return sortRowsByTokens(rows)
}

func globalRows(claudeHome string, record *contextrecord.Record) []sectionRow {
	groups := groupItems(record.Agents, ofKind(contextrecord.KindGlobal), itemLabel)
	rows := make([]sectionRow, 0, len(groups))
	for _, group := range groups {
		first := group.first()
		group.row.Subject = relativePath(first.Label, record.Cwd, claudeHome)
		group.row.Text, group.row.TextRest = first.Capped()
		group.finish()
		rows = append(rows, group.row)
	}
	return sortRowsByTokens(rows)
}

func groupItems(agents []contextrecord.Agent, isIn func(*contextrecord.Item) bool, keyOf func(*contextrecord.Item) string) []*rowGroup {
	var groups []*rowGroup
	byKey := map[string]*rowGroup{}
	for agentIndex := range agents {
		agent := &agents[agentIndex]
		for itemIndex := range agent.Items {
			item := &agent.Items[itemIndex]
			if !isIn(item) {
				continue
			}

			key := keyOf(item)
			group, isKnown := byKey[key]
			if !isKnown {
				group = newRowGroup()
				byKey[key] = group
				groups = append(groups, group)
			}
			group.add(agent, item)
		}
	}
	return groups
}

// guideLangs collects the lang views of one guide from the agents' contexts.
func guideLangs(contexts []*contextrecord.AgentContext, guide string) []contextrecord.LangContext {
	var langs []contextrecord.LangContext
	for _, view := range contexts {
		for _, lang := range view.Injected.Lang {
			if lang.Guide == guide {
				langs = append(langs, lang)
			}
		}
	}
	return langs
}

func hookInjectionRows(claudeHome string, record *contextrecord.Record) []sectionRow {
	isHook := func(item *contextrecord.Item) bool { return isHookKind(item.Kind) }
	keyOf := func(item *contextrecord.Item) string {
		return kindTag(item.Kind, item.Label).Text + "\x00" + hookSubject(claudeHome, record.Cwd, item).String()
	}
	groups := groupItems(record.Agents, isHook, keyOf)
	rows := make([]sectionRow, 0, len(groups))
	for _, group := range groups {
		first := group.first()
		group.row.Subject = hookSubject(claudeHome, record.Cwd, first)
		group.row.Tag = kindTag(first.Kind, first.Label)
		group.row.Text, group.row.TextRest = first.Capped()
		group.finish()
		rows = append(rows, group.row)
	}
	return sortRowsByTokens(rows)
}

func hookSubject(claudeHome, cwd string, item *contextrecord.Item) pathCell {
	switch item.Kind {
	case contextrecord.KindAcdslContext, contextrecord.KindAgentsMd:
		return relativePath(item.Label, cwd, claudeHome)
	case contextrecord.KindHookContext:
		return pathCell{Rel: item.Preview}
	case contextrecord.KindCodexEnvironment, contextrecord.KindCodexInstructions, contextrecord.KindPlanContext:
		return pathCell{Rel: item.Label}
	case contextrecord.KindAcdslProjection, contextrecord.KindGlobal, contextrecord.KindLang, contextrecord.KindPackRead,
		contextrecord.KindPlanRules, contextrecord.KindSkill, contextrecord.KindSubagentPrompt:
		return pathCell{Rel: item.Label}
	}
	return pathCell{Rel: item.Label}
}

// itemLabel keys an item by its label.
func itemLabel(item *contextrecord.Item) string {
	return item.Label
}

// ofKind keeps the items of the given kinds.
func ofKind(kinds ...contextrecord.Kind) func(*contextrecord.Item) bool {
	return func(item *contextrecord.Item) bool {
		return slices.Contains(kinds, item.Kind)
	}
}

// projectionRows drops a row the active filter empties.
func projectionRows(claudeHome string, filter contextFilter, index map[string]idMeta, record *contextrecord.Record) []sectionRow {
	groups := groupItems(record.Agents, ofKind(contextrecord.KindAcdslProjection), itemLabel)
	rows := make([]sectionRow, 0, len(groups))
	for _, group := range groups {
		file := group.first().Label
		for _, view := range group.contexts(record.Agents) {
			group.ids = append(group.ids, projectionIds(view, file)...)
		}
		group.setFamilies(filter, index)
		if filter.Active() && len(group.row.Families) == 0 {
			continue
		}

		group.row.Subject = relativePath(file, record.Cwd, claudeHome)
		group.finish()
		rows = append(rows, group.row)
	}
	return sortRowsByTokens(rows)
}

func projectionIds(view *contextrecord.AgentContext, file string) []string {
	var ids []string
	for _, rules := range view.AcdslRules {
		if rules.File == file {
			ids = append(ids, rules.Ids...)
		}
	}
	return ids
}

func promptRows(claudeHome string, filter contextFilter, index map[string]idMeta, record *contextrecord.Record) []sectionRow {
	var rows []sectionRow
	for agentIndex := range record.Agents {
		rows = append(rows, agentPromptRows(claudeHome, record.Cwd, filter, index, &record.Agents[agentIndex])...)
	}
	return sortRowsByTokens(rows)
}

func scopeCoverage(langs []contextrecord.LangContext, lines string) coverageBadge {
	full := 0
	for _, lang := range langs {
		if lang.Coverage == coverageComplete {
			full++
		}
	}
	switch {
	case len(langs) == 0:
		return coverageBadge{}
	case full == len(langs):
		return coverageBadge{Class: badgeOk, Text: "read in full"}
	case len(langs) == 1:
		return coverageBadge{Class: badgeAction, Text: fmt.Sprintf("%s of %d", lines, langs[0].TotalLines)}
	}
	return coverageBadge{Class: badgeAction, Text: fmt.Sprintf("%d of %d agents in full", full, len(langs))}
}

func sectionCells(kind string, row sectionRow) sectionCellsView {
	view := sectionCellsView{Kind: kind, Row: row}
	extraColumns := 1
	switch kind {
	case sectionFiles:
		view.Cells = []string{orDash(row.Lines)}
		extraColumns = 2
	case sectionHooks:
		extraColumns = 2
	case sectionProjections, sectionSkills:
		view.Cells = []string{strconv.Itoa(row.Entries)}
	case sectionPrompts:
		view.Cells = []string{strconv.Itoa(len(row.Paths)), strconv.Itoa(row.Entries)}
		extraColumns = 0
	case sectionGlobal:
	}
	view.Columns = fixedColumns + extraColumns + len(view.Cells)
	return view
}

func skillContext(view *contextrecord.AgentContext, skill string) ([]string, string) {
	var ids []string
	version := ""
	for _, injected := range view.Injected.Skill {
		if injected.Skill != skill {
			continue
		}

		ids = append(ids, injected.Ids...)
		version = cmp.Or(version, injected.Version)
	}
	return ids, version
}

// skillContextRows drops a row the active filter empties.
func skillContextRows(filter contextFilter, index map[string]idMeta, record *contextrecord.Record) []sectionRow {
	groups := groupItems(record.Agents, ofKind(contextrecord.KindSkill), itemLabel)
	rows := make([]sectionRow, 0, len(groups))
	for _, group := range groups {
		skill := group.first().Label
		for _, view := range group.contexts(record.Agents) {
			ids, version := skillContext(view, skill)
			group.ids = append(group.ids, ids...)
			group.row.Version = cmp.Or(group.row.Version, version)
		}
		group.setFamilies(filter, index)
		if filter.Active() && len(group.row.Families) == 0 {
			continue
		}

		group.row.Subject = pathCell{Rel: skill}
		group.finish()
		rows = append(rows, group.row)
	}
	return sortRowsByTokens(rows)
}

// sortRowsByTokens orders rows by counted tokens, then chars, then subject; stable within ties.
func sortRowsByTokens(rows []sectionRow) []sectionRow {
	slices.SortStableFunc(rows, func(left, right sectionRow) int {
		if byTokens := cmp.Compare(right.tokensValue, left.tokensValue); byTokens != 0 {
			return byTokens
		}
		if byChars := cmp.Compare(right.charsValue, left.charsValue); byChars != 0 {
			return byChars
		}
		return strings.Compare(left.Subject.String(), right.Subject.String())
	})
	return rows
}

// sumChars adds the items' chars.
func sumChars(items []*contextrecord.Item) int {
	chars := 0
	for _, item := range items {
		chars += item.Chars
	}
	return chars
}

// sumTokenCell falls back to the first item's state when nothing is counted.
func sumTokenCell(items []*contextrecord.Item) tokenCell {
	if len(items) == 0 {
		return tokenCell{Text: "—"}
	}

	counted, pending, tokens := 0, 0, 0
	isApprox := false
	for _, item := range items {
		if isPending(item) {
			pending++
		}
		if item.Counter == "" {
			continue
		}

		counted++
		tokens += item.Tokens
		isApprox = isApprox || item.Approx
	}
	if counted == 0 {
		return itemTokenCell(items[0])
	}

	text := groupThousands(int64(tokens))
	if isApprox {
		text = "≈" + text
	}
	if pending == 0 {
		return tokenCell{Text: text}
	}
	return tokenCell{Text: fmt.Sprintf("%s · %d %s", text, pending, wordPending), Title: "not every injection is counted yet"}
}

// sumTokens adds the counted items' tokens; pending and uncounted items add nothing.
func sumTokens(items []*contextrecord.Item) int {
	tokens := 0
	for _, item := range items {
		if item.Counter != "" {
			tokens += item.Tokens
		}
	}
	return tokens
}

// unionLines renders "lines 1–450, 901–1327"; adjacent spans join.
func unionLines(items []*contextrecord.Item) string {
	var spans [][2]int
	for _, item := range items {
		if item.EndLine > 0 {
			spans = append(spans, [2]int{item.StartLine, item.EndLine})
		}
	}
	if len(spans) == 0 {
		return ""
	}

	slices.SortFunc(spans, func(first, second [2]int) int { return cmp.Compare(first[0], second[0]) })
	merged := [][2]int{spans[0]}
	for _, span := range spans[1:] {
		last := &merged[len(merged)-1]
		if span[0] <= last[1]+1 {
			last[1] = max(last[1], span[1])
			continue
		}
		merged = append(merged, span)
	}

	parts := make([]string, 0, len(merged))
	for _, span := range merged {
		parts = append(parts, fmt.Sprintf("%d–%d", span[0], span[1]))
	}
	return "lines " + strings.Join(parts, ", ")
}
