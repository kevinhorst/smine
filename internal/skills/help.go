package skills

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/sessiontitle"
)

const (
	helpColumnGap    = 2
	helpDefaultTopic = "default"
	helpIndent       = "  "
)

// Routing fields parseRouting tracks.
const (
	routingFieldDisambiguation = "disambiguation"
	routingFieldPipeline       = "pipeline"
	routingFieldPreconditions  = "preconditions"
	routingFieldPurpose        = "purpose"
)

// helpLinkSuffixPattern: an on-page anchor means nothing in a hook reason panel.
var helpLinkSuffixPattern = regexp.MustCompile(`\s*—\s*\[[^\]]+\]\([^)]+\)\s*$`)

type Help struct {
	ArgumentHint   string
	Args           []HelpArg
	Disambiguation [][]string // For, Use
	Frontmatter    []string   // raw lines between the fences
	Pipeline       string
	Preconditions  []string
	Purpose        string
	Summary        string
	Tests          string           // help-test.md body, empty when the deploy wrote none
	TitleStates    []HelpTitleState // from title.json, empty without a declaration
	Topics         []HelpTopic      // from context-map.json, empty without a map
}

// HelpArg is one declared arg with its display cells.
type HelpArg struct {
	Cells []string // Form, Default, Effect — as far as the source declares them
	Name  string
}

type HelpTitleState struct {
	Entry   string
	Meaning string
	State   string
}

type HelpTopic struct {
	Globs []string
	Name  string
}

// helpContextMap reads only the lanes of context-map.json.
type helpContextMap struct {
	Lanes map[string][]string `json:"lanes"`
}

// ParseHelp takes each sibling empty when the skill carries none.
func ParseHelp(contextMap string, helpTest string, skillMd string, titleDocument string) (*Help, error) {
	lines := strings.Split(skillMd, "\n")
	fm := parseFrontmatter(skillMd)
	summary, descriptionArgs := splitDescription(fm.Description)
	help := &Help{
		ArgumentHint: fm.ArgumentHint,
		Frontmatter:  frontmatterLines(lines),
		Summary:      summary,
		Tests:        strings.TrimSpace(helpTest),
	}
	help.Args = helpArgs(ParseArgsSection(lines), descriptionArgs)
	parseRouting(routingSection(lines), help)

	if contextMap != "" {
		topics, err := parseHelpTopics(contextMap)
		if err != nil {
			return nil, err
		}
		help.Topics = topics
	}
	if titleDocument != "" {
		states, err := parseHelpTitleStates(titleDocument)
		if err != nil {
			return nil, err
		}
		help.TitleStates = states
	}
	return help, nil
}

// RenderHelp emits no markdown tables: the hook reason panel shows plain text.
func RenderHelp(name string, help *Help) string {
	var out strings.Builder
	fmt.Fprintf(&out, "/%s — %s\n", name, help.Summary)

	out.WriteString("\nFrontmatter\n")
	for _, line := range help.Frontmatter {
		out.WriteString(helpIndent + line + "\n")
	}

	out.WriteString("\nRouting\n")
	if help.Purpose != "" {
		out.WriteString(helpIndent + "Purpose: " + help.Purpose + "\n")
	}
	if len(help.Disambiguation) > 0 {
		out.WriteString(helpIndent + "Use instead:\n")
		writeColumns(&out, helpIndent+helpIndent, help.Disambiguation)
	}
	if len(help.Preconditions) > 0 {
		out.WriteString(helpIndent + "Preconditions:\n")
		for _, precondition := range help.Preconditions {
			out.WriteString(helpIndent + helpIndent + "- " + precondition + "\n")
		}
	}
	if help.Pipeline != "" {
		out.WriteString(helpIndent + "Pipeline: " + help.Pipeline + "\n")
	}

	out.WriteString("\nArgs\n")
	if len(help.Args) == 0 {
		out.WriteString(helpIndent + "none\n")
	}
	rows := make([][]string, 0, len(help.Args))
	for _, arg := range help.Args {
		rows = append(rows, append([]string{arg.Name}, arg.Cells...))
	}
	writeColumns(&out, helpIndent, rows)
	if help.ArgumentHint != "" {
		fmt.Fprintf(&out, "%sExample: /%s %s\n", helpIndent, name, help.ArgumentHint)
	}

	if len(help.TitleStates) > 0 {
		out.WriteString("\nTitle states (title.json)\n")
		stateRows := make([][]string, 0, len(help.TitleStates))
		for _, state := range help.TitleStates {
			row := []string{state.State, state.Meaning, state.Entry}
			stateRows = append(stateRows, row)
		}
		writeColumns(&out, helpIndent, stateRows)
	}

	if help.Tests != "" {
		out.WriteString("\nTests\n")
		for line := range strings.SplitSeq(help.Tests, "\n") {
			out.WriteString(helpIndent + line + "\n")
		}
	}

	if len(help.Topics) > 0 {
		out.WriteString("\nTopics (context-map.json)\n")
		topicRows := make([][]string, 0, len(help.Topics))
		for _, topic := range help.Topics {
			topicRows = append(topicRows, []string{topic.Name, strings.Join(topic.Globs, ", ")})
		}
		writeColumns(&out, helpIndent, topicRows)
	}
	return out.String()
}

// frontmatterLines returns the raw lines between the frontmatter fences.
func frontmatterLines(lines []string) []string {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontmatterFence {
		return nil
	}
	var block []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == frontmatterFence {
			break
		}
		block = append(block, line)
	}
	return block
}

// helpArgs: bullet rows borrow the description's Args doc.
func helpArgs(spec ArgsSpec, descriptionArgs []SkillArg) []HelpArg {
	docs := make(map[string]string, len(descriptionArgs))
	for _, arg := range descriptionArgs {
		docs[arg.Name] = arg.Doc
	}
	if !spec.Exists {
		args := make([]HelpArg, 0, len(descriptionArgs))
		for _, arg := range descriptionArgs {
			args = append(args, HelpArg{Cells: []string{arg.Doc}, Name: arg.Name})
		}
		return args
	}
	args := make([]HelpArg, 0, len(spec.Rows))
	for _, row := range spec.Rows {
		arg := HelpArg{Name: row.Name}
		if len(row.Cells) > 1 {
			for _, cell := range row.Cells[1:] {
				plain := strings.ReplaceAll(helpLinkSuffixPattern.ReplaceAllString(cell, ""), "`", "")
				arg.Cells = append(arg.Cells, plain)
			}
		} else {
			arg.Cells = []string{docs[row.Name]}
		}
		args = append(args, arg)
	}
	return args
}

func parseHelpTopics(contextMap string) ([]HelpTopic, error) {
	var parsed helpContextMap
	if err := json.Unmarshal([]byte(contextMap), &parsed); err != nil {
		return nil, fmt.Errorf("parseHelpTopics: Invalid context map: %w", err)
	}
	names := make([]string, 0, len(parsed.Lanes))
	for name := range parsed.Lanes {
		if name != helpDefaultTopic {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if _, ok := parsed.Lanes[helpDefaultTopic]; ok {
		names = append([]string{helpDefaultTopic}, names...)
	}
	topics := make([]HelpTopic, 0, len(names))
	for _, name := range names {
		topics = append(topics, HelpTopic{Globs: parsed.Lanes[name], Name: name})
	}
	return topics, nil
}

// parseHelpTitleStates reads the states of a title.json in token order.
func parseHelpTitleStates(titleDocument string) ([]HelpTitleState, error) {
	document, err := sessiontitle.ParseDocument([]byte(titleDocument))
	if err != nil {
		return nil, fmt.Errorf("parseHelpTitleStates: %w", err)
	}

	states := make([]HelpTitleState, 0, len(document.States))
	for _, token := range document.Tokens() {
		declared := document.States[string(token)]
		state := HelpTitleState{Entry: declared.Entry, Meaning: declared.Meaning, State: string(token)}
		states = append(states, state)
	}
	return states, nil
}

// parseRouting joins wrapped lines; the blank line after the Pipeline paragraph ends the fields.
func parseRouting(section []string, help *Help) {
	field := ""
	for _, line := range section {
		trimmed := strings.TrimSpace(line)
		switch {
		case field == routingFieldPipeline && trimmed == "":
			return
		case strings.HasPrefix(trimmed, "**Purpose:**"):
			help.Purpose = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Purpose:**"))
			field = routingFieldPurpose
		case strings.HasPrefix(trimmed, "**Disambiguation:**"):
			field = routingFieldDisambiguation
		case strings.HasPrefix(trimmed, "**Preconditions:**"):
			field = routingFieldPreconditions
		case strings.HasPrefix(trimmed, "**Pipeline:**"):
			help.Pipeline = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Pipeline:**"))
			field = routingFieldPipeline
		case trimmed == "":
		case field == routingFieldDisambiguation && strings.HasPrefix(trimmed, "|"):
			cells := SplitTableRow(trimmed)
			if len(cells) >= 2 && cells[0] != "For" && !strings.HasPrefix(cells[0], ":-") {
				help.Disambiguation = append(help.Disambiguation, cells[:2])
			}
		case field == routingFieldPreconditions && strings.HasPrefix(line, "- "):
			help.Preconditions = append(help.Preconditions, strings.TrimPrefix(line, "- "))
		case field == routingFieldPreconditions && len(help.Preconditions) > 0:
			last := len(help.Preconditions) - 1
			help.Preconditions[last] += " " + trimmed
		case field == routingFieldPurpose:
			help.Purpose += " " + trimmed
		case field == routingFieldPipeline:
			help.Pipeline += " " + trimmed
		}
	}
}

func routingSection(lines []string) []string {
	start := -1
	for index, line := range lines {
		if strings.HasPrefix(line, "## Routing") {
			start = index
			break
		}
	}
	if start < 0 {
		return nil
	}
	section := lines[start+1:]
	for index, line := range section {
		if strings.HasPrefix(line, "## ") {
			return section[:index]
		}
	}
	return section
}

// writeColumns prints rows with every column padded to its widest cell.
func writeColumns(out *strings.Builder, indent string, rows [][]string) {
	widths := make([]int, 0)
	for _, row := range rows {
		for index, cell := range row {
			if index >= len(widths) {
				widths = append(widths, 0)
			}
			if width := len([]rune(cell)); width > widths[index] {
				widths[index] = width
			}
		}
	}
	for _, row := range rows {
		out.WriteString(indent)
		for index, cell := range row {
			if index == len(row)-1 {
				out.WriteString(cell)
				break
			}
			out.WriteString(cell + strings.Repeat(" ", widths[index]-len([]rune(cell))+helpColumnGap))
		}
		out.WriteString("\n")
	}
}
