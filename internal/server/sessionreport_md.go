package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

// Column titles shared by the section tables, the page's headers and their translations.
const (
	columnAgents     = "Agents"
	columnChars      = "Chars"
	columnInjections = "Injections"
	columnReads      = "Reads"
	columnTokens     = "Tokens"
)

// markdownCell escapes the table separator inside one cell.
func markdownCell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}

// markdownRowCells follows the page's column order.
func markdownRowCells(view sectionCellsView) []string {
	row := view.Row
	var subject string
	switch view.Kind {
	case sectionHooks:
		subject = "[" + row.Tag.Text + "]"
	case sectionSkills:
		subject = "/" + row.Subject.Rel
		if row.Version != "" {
			subject += " v" + row.Version
		}
	case sectionPrompts:
		subject = markdownCell(row.Subject.Rel)
	case sectionFiles, sectionGlobal, sectionProjections:
		subject = "`" + row.Subject.Rel + "`"
	}

	cells := []string{subject}
	if view.Kind == sectionHooks {
		cells = append(cells, markdownCell(row.Subject.Rel))
	}
	if view.Kind == sectionPrompts {
		cells = append(cells, row.Agents)
	}
	cells = append(cells, view.Cells...)
	if view.Kind == sectionFiles {
		cells = append(cells, orDash(row.Coverage.Text))
	}
	if view.Kind != sectionPrompts {
		cells = append(cells, row.Agents, strconv.Itoa(row.Count))
	}
	return append(cells, row.Chars, row.Tokens.Text)
}

// orDash renders an absent value as a dash.
func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

// renderSessionReportMarkdown applies the page's filter projection and agent selection.
func renderSessionReportMarkdown(filter contextFilter, id string, page *sessionReportPage) string {
	var out strings.Builder
	writeReportHeader(&out, filter, id, page)
	writeReportContext(&out, page)
	writeReportSkills(&out, page)
	writeReportFiles(&out, page)
	return out.String()
}

// sectionHeaders are one section table's column titles, in the page's order.
func sectionHeaders(kind string) []string {
	switch kind {
	case sectionFiles:
		return []string{"Path", "Lines", "Coverage", columnAgents, columnReads, columnChars, columnTokens}
	case sectionGlobal:
		return []string{"Source", columnAgents, columnInjections, columnChars, columnTokens}
	case sectionHooks:
		return []string{"Hook", "Subject", columnAgents, columnInjections, columnChars, columnTokens}
	case sectionProjections:
		return []string{"File", "Rules", columnAgents, columnReads, columnChars, columnTokens}
	case sectionPrompts:
		return []string{"Prompt", "Spawned by", "Paths", "Ids", columnChars, columnTokens}
	case sectionSkills:
		return []string{"Skill", "Entries", columnAgents, columnInjections, columnChars, columnTokens}
	}
	return nil
}

func writeIdChip(out *strings.Builder, chip idChip) {
	if chip.Title == "" {
		fmt.Fprintf(out, "  - %s\n", chip.Id)
		return
	}
	fmt.Fprintf(out, "  - %s — %s\n", chip.Id, chip.Title)
}

func writeIdFamilies(out *strings.Builder, families []idFamily) {
	if len(families) == 0 {
		out.WriteString("—\n\n")
		return
	}
	for _, family := range families {
		fmt.Fprintf(out, "- %s (%d)\n", family.Name, len(family.Chips))
		for _, chip := range family.Chips {
			writeIdChip(out, chip)
		}
	}
	out.WriteString("\n")
}

// writeReportAgents mirrors the page's per-agent table.
func writeReportAgents(out *strings.Builder, page *sessionReportPage) {
	out.WriteString("### Injected context by agent\n\n")
	if page.TokenNotice != "" {
		fmt.Fprintf(out, "%s\n\n", page.TokenNotice)
	}
	if page.PendingNotice != "" {
		fmt.Fprintf(out, "%s\n\n", page.PendingNotice)
	}
	out.WriteString("| Agent | Description | Model | Injections | Chars | Tokens | Pending |\n|---|---|---|---|---|---|---|\n")
	for _, row := range page.Agents {
		fmt.Fprintf(out, "| %s %s | %s | %s | %s | %s | %s | %s |\n", row.Kind, row.Name, row.Description, row.Model, row.Items, row.Chars, row.Tokens, row.Pending)
	}
	fmt.Fprintf(out, "| **Total** | | | %s | %s | %s | %s |\n\n", page.Totals.Items, page.Totals.Chars, page.Totals.Tokens, page.Totals.Pending)
}

func writeReportContext(out *strings.Builder, page *sessionReportPage) {
	out.WriteString("## Ingested context\n\n")
	if page.ContextErr != "" {
		fmt.Fprintf(out, "Context record unavailable: %s\n\n", page.ContextErr)
	}
	if page.Record == nil {
		return
	}
	if page.PathBase != "" {
		fmt.Fprintf(out, "Paths relative to `%s`\n\n", page.PathBase)
	}
	writeReportAgents(out, page)

	writeSectionHeading(out, "SessionStart [global-context]", page.KindSizes[string(contextrecord.KindGlobal)])
	writeSectionTable(out, sectionGlobal, page.GlobalRows)

	writeSectionHeading(out, "Skill context [skill-context]", page.KindSizes[string(contextrecord.KindSkill)])
	writeSectionTable(out, sectionSkills, page.SkillRows)

	writeSectionHeading(out, "ACDSL projections [acdsl]", page.KindSizes[string(contextrecord.KindAcdslProjection)])
	writeSectionTable(out, sectionProjections, page.ProjectionRows)

	writeSectionHeading(out, "Plan-time rules [acdsl project -plan]", page.KindSizes[string(contextrecord.KindPlanRules)])
	writeIdFamilies(out, page.PlanFamilies)

	writeSectionHeading(out, "Context files read [Read] [read-gate]", page.KindSizes[sizeKeyFileReads])
	writeSectionTable(out, sectionFiles, page.FileReadRows)

	writeSectionHeading(out, "Subagent prompts", page.KindSizes[string(contextrecord.KindSubagentPrompt)])
	writeSectionTable(out, sectionPrompts, page.PromptRows)

	writeSectionHeading(out, "Hook injections", page.KindSizes[sizeKeyHooks])
	writeSectionTable(out, sectionHooks, page.HookRows)
}

func writeReportFiles(out *strings.Builder, page *sessionReportPage) {
	out.WriteString("## Touched files\n\n")
	if len(page.FileGroups) == 0 {
		out.WriteString("—\n")
		return
	}
	for _, group := range page.FileGroups {
		fmt.Fprintf(out, "### %s (%d files)\n\n", group.Name, len(group.Files))
		for _, file := range group.Files {
			fmt.Fprintf(out, "- `%s` — %s · read %d · write %d\n", file.Path.Rel, file.Agents, file.Reads, file.Writes)
		}
		out.WriteString("\n")
	}
}

func writeReportHeader(out *strings.Builder, filter contextFilter, id string, page *sessionReportPage) {
	title := page.Session.Title
	if title == "" {
		title = id
	}
	fmt.Fprintf(out, "# Session context report — %s\n\n", title)
	fmt.Fprintf(out, "- Session: `%s`\n", id)
	if filter.Active() {
		fmt.Fprintf(out, "- Filter: `%s`\n", strings.TrimPrefix(filter.URL(""), "?"))
	}
	if !page.Scope.IsAll() {
		fmt.Fprintf(out, "- Agents: `%s`\n", page.Scope.Param())
	}
	if page.EventsErr != "" {
		fmt.Fprintf(out, "- Events unavailable: %s\n", page.EventsErr)
	}
	if page.Events != nil {
		stats := page.Stats
		fmt.Fprintf(out, "- Started: %s\n", page.Events.StartedAt.Format(time.DateTime))
		fmt.Fprintf(out, "- Last active: %s\n", page.Events.LastActive.Format(time.DateTime))
		fmt.Fprintf(out, "- Runtime: %s (active %s, idle %s)\n", stats.Runtime, stats.ActiveTime, stats.IdleTime)
		fmt.Fprintf(out, "- Cost: %s\n", stats.Cost)
		if stats.PlanPercent != "" {
			fmt.Fprintf(out, "- Plan: %s of a %s 5-hour window\n", stats.PlanPercent, page.Plan)
		}
		fmt.Fprintf(out, "- Tokens: %s — in %s · out %s · cache read %s · cache write %s\n",
			stats.TotalTokens, stats.InputTokens, stats.OutputTokens, stats.CacheRead, stats.CacheWrite)
		if stats.CacheHit != "" {
			fmt.Fprintf(out, "- Cache hit: %s\n", stats.CacheHit)
		}
	}
	if page.Session.Cwd != "" {
		fmt.Fprintf(out, "- Cwd: `%s`\n", page.Session.Cwd)
	}
	if page.Branch != "" {
		fmt.Fprintf(out, "- Branch: `%s`\n", page.Branch)
	}
	out.WriteString("\n")
}

func writeReportSkills(out *strings.Builder, page *sessionReportPage) {
	out.WriteString("## Invoked skills\n\n")
	if len(page.InvokedSkills) == 0 {
		out.WriteString("—\n\n")
		return
	}
	out.WriteString("| Skill | Version | Skill tokens | Source | Started | Ended |\n|---|---|---|---|---|---|\n")
	for _, row := range page.InvokedSkills {
		version := row.Version
		if row.IsVersionAssumed {
			version = "≈" + version
		}
		fmt.Fprintf(out, "| /%s | %s | %s | %s | %s | %s |\n", row.Skill, version, row.SkillTokens, row.Source, row.Started, row.Ended)
	}
	out.WriteString("\n")
}

func writeSectionHeading(out *strings.Builder, title string, size sectionSize) {
	text := size.String()
	if text == "" {
		fmt.Fprintf(out, "### %s\n\n", title)
		return
	}
	fmt.Fprintf(out, "### %s — %s\n\n", title, text)
}

// writeSectionTable writes one section as one table, "—" without rows.
func writeSectionTable(out *strings.Builder, kind string, rows []sectionRow) {
	if len(rows) == 0 {
		out.WriteString("—\n\n")
		return
	}

	headers := sectionHeaders(kind)
	fmt.Fprintf(out, "| %s |\n|%s\n", strings.Join(headers, " | "), strings.Repeat("---|", len(headers)))
	for _, row := range rows {
		cells := markdownRowCells(sectionCells(kind, row))
		fmt.Fprintf(out, "| %s |\n", strings.Join(cells, " | "))
	}
	out.WriteString("\n")
}
