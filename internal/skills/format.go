package skills

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// EffectCellMaxRunes: deep semantics live in the owning body entry.
const EffectCellMaxRunes = 160

// argsBulletFallbackMax serves selector/flag-only skills.
const argsBulletFallbackMax = 2

// argsTableCells is the column count of the Args syntax-index table.
const argsTableCells = 4

const enumerationMaxItems = 6

// frontmatterFence delimits the YAML frontmatter block.
const frontmatterFence = "---"

var argsBulletPattern = regexp.MustCompile("^- `?([A-Za-z0-9][A-Za-z0-9|<> _-]*?)`?:")

// pathRuleSpecPattern matches the path-rule specs the tool-policy gate turns into skill-grant roots.
var pathRuleSpecPattern = regexp.MustCompile(`^(~/[^()]+|//[^()]+|@repos|\$SKILL_[A-Z0-9]+_[A-Z][A-Z0-9_]*)/\*\*$`)

// pathRuleTools are the tools whose rules are path rules.
var pathRuleTools = []string{"Edit", "Read", "Write"}

// inlineCodeSpanPattern captures single-backtick inline code spans.
var inlineCodeSpanPattern = regexp.MustCompile("`([^`]+)`")

// markdownLinkPattern strips link targets, which inflate the rune count but not what the reader sees.
var markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)

// contextIdPattern leaves ACDSL-* gate names allowed: they are operational identifiers.
var contextIdPattern = regexp.MustCompile(`\b(ACTION|RULE|FACT)-[A-Z]+(-[A-Z0-9]+)?-[0-9]{3}[a-z]?\b`)

// ArgsRow is one declared arg of the ## Args section.
type ArgsRow struct {
	Cells []string // table form only: the 4 cell contents
	Line  int      // 1-based
	Name  string
}

// ArgsSpec is the parsed ## Args section of a SKILL.md body.
type ArgsSpec struct {
	Exists bool
	Line   int // 1-based line of the ## Args heading
	Rows   []ArgsRow
	Table  bool
}

// FormatViolation is one skill-format finding, addressed by 1-based line.
type FormatViolation struct {
	Line    int
	Message string
}

func CheckArgsTable(spec ArgsSpec) []FormatViolation {
	var violations []FormatViolation
	if !spec.Exists {
		return violations
	}
	if !spec.Table {
		if len(spec.Rows) > argsBulletFallbackMax {
			violation := FormatViolation{
				Line:    spec.Line,
				Message: fmt.Sprintf("## Args declares %d args as bullets — over %d args requires the | Arg | Form | Default | Effect | table", len(spec.Rows), argsBulletFallbackMax),
			}
			violations = append(violations, violation)
		}
		return violations
	}

	for _, row := range spec.Rows {
		if len(row.Cells) != argsTableCells {
			violation := FormatViolation{
				Line:    row.Line,
				Message: fmt.Sprintf("Args row %q has %d cells — the table is | Arg | Form | Default | Effect |", row.Name, len(row.Cells)),
			}
			violations = append(violations, violation)
			continue
		}
		if !strings.HasPrefix(row.Cells[0], "`") || !strings.HasSuffix(row.Cells[0], "`") {
			violation := FormatViolation{
				Line:    row.Line,
				Message: fmt.Sprintf("Args row %q: the Arg cell is backticked", row.Name),
			}
			violations = append(violations, violation)
		}
		if effect := len([]rune(markdownLinkPattern.ReplaceAllString(row.Cells[3], "$1"))); effect > EffectCellMaxRunes {
			violation := FormatViolation{
				Line:    row.Line,
				Message: fmt.Sprintf("Args row %q: Effect cell %d runes exceeds %d — semantics live in the owning entry", row.Name, effect, EffectCellMaxRunes),
			}
			violations = append(violations, violation)
		}
	}
	return violations
}

// CheckArgumentHint rejects a leading /<leaf>: the UI already shows the skill name.
func CheckArgumentHint(leaf, hint string, hintLine int, spec ArgsSpec) []FormatViolation {
	var violations []FormatViolation
	if !spec.Exists {
		return violations
	}
	if hint == "" {
		violation := FormatViolation{
			Line:    1,
			Message: "skill has ## Args but no argument-hint example invocation",
		}
		violations = append(violations, violation)
		return violations
	}
	if strings.HasPrefix(hint, "/"+leaf) {
		violation := FormatViolation{
			Line:    hintLine,
			Message: fmt.Sprintf("argument-hint is the args-only example — it must not start with %q, the UI already shows the skill name", "/"+leaf),
		}
		violations = append(violations, violation)
	}
	for _, row := range spec.Rows {
		// a free-text positional (the run title) has no named token in the example
		if len(row.Cells) == 4 && strings.Contains(row.Cells[1], "free text") {
			continue
		}
		if !strings.Contains(hint, row.Name) {
			violation := FormatViolation{
				Line:    hintLine,
				Message: fmt.Sprintf("argument-hint example does not set arg %q", row.Name),
			}
			violations = append(violations, violation)
		}
	}
	return violations
}

// CheckContextIds is the mechanical half of the no-context-ids rule in context/rules/skills.md.
func CheckContextIds(lines []string) []FormatViolation {
	var violations []FormatViolation
	inFrontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == frontmatterFence
	inFence := false
	for index, line := range lines {
		if index == 0 && inFrontmatter {
			continue
		}
		if inFrontmatter {
			if strings.TrimSpace(line) == frontmatterFence {
				inFrontmatter = false
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if id := contextIdPattern.FindString(line); id != "" {
			violation := FormatViolation{
				Line:    index + 1,
				Message: fmt.Sprintf("bare context-entry id %q in body prose — name the rule in plain language as a link", id),
			}
			violations = append(violations, violation)
		}
	}
	return violations
}

// CheckEnumerations enforces the enumeration rule in context/rules/skills.md.
func CheckEnumerations(lines []string) []FormatViolation {
	var violations []FormatViolation
	inFrontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == frontmatterFence
	inFence := false
	for index, line := range lines {
		if index == 0 && inFrontmatter {
			continue
		}
		if inFrontmatter {
			if strings.TrimSpace(line) == frontmatterFence {
				inFrontmatter = false
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, span := range inlineCodeSpanPattern.FindAllStringSubmatch(line, -1) {
			if items := len(strings.Split(span[1], ",")); items > enumerationMaxItems {
				violation := FormatViolation{
					Line:    index + 1,
					Message: fmt.Sprintf("inline code span enumerates %d comma-separated items (max %d) — use a fenced block or a table", items, enumerationMaxItems),
				}
				violations = append(violations, violation)
			}
		}
	}
	return violations
}

func ParseArgsSection(lines []string) ArgsSpec {
	var spec ArgsSpec
	start := -1
	for index, line := range lines {
		if strings.HasPrefix(line, "## Args") {
			start = index
			spec.Exists = true
			spec.Line = index + 1
			break
		}
	}
	if start < 0 {
		return spec
	}

	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "## ") {
			break
		}
		if match := argsBulletPattern.FindStringSubmatch(line); match != nil {
			row := ArgsRow{
				Line: index + 1,
				Name: match[1],
			}
			spec.Rows = append(spec.Rows, row)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := SplitTableRow(line)
		if len(cells) == 0 || cells[0] == "Arg" || strings.HasPrefix(cells[0], ":-") {
			if len(cells) > 0 && cells[0] == "Arg" {
				spec.Table = true
			}
			continue
		}
		row := ArgsRow{
			Cells: cells,
			Line:  index + 1,
			Name:  strings.Trim(cells[0], "`"),
		}
		spec.Rows = append(spec.Rows, row)
	}
	return spec
}

// FormatInvocationLines starts a line at a `name:` token or a bare flag in argNames.
func FormatInvocationLines(hint string, argNames []string) []string {
	fields := strings.Fields(hint)
	if len(fields) == 0 {
		return nil
	}

	flags := make(map[string]bool, len(argNames))
	for _, name := range argNames {
		flags[name] = true
	}
	argToken := regexp.MustCompile(`^[a-z][a-z0-9-]*:$`)
	lines := []string{fields[0]}
	for _, field := range fields[1:] {
		if argToken.MatchString(field) || flags[field] {
			lines = append(lines, field)
			continue
		}
		lines[len(lines)-1] += " " + field
	}
	return lines
}

// IsGrantablePathRule accepts only path-rule shapes the tool-policy gate matches as a skill grant.
func IsGrantablePathRule(item string) bool {
	tool, spec, hasSpec := strings.Cut(strings.TrimSuffix(item, ")"), "(")
	if !hasSpec || !slices.Contains(pathRuleTools, tool) {
		return true
	}
	return pathRuleSpecPattern.MatchString(spec)
}

// ToolsSorted: plain tools first, then Bash/MCP entries by group key then token.
func ToolsSorted(raw string) bool {
	var keys []string
	for token := range strings.SplitSeq(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		keys = append(keys, toolSortKey(token))
	}
	for index := 1; index < len(keys); index++ {
		if keys[index] < keys[index-1] {
			return false
		}
	}
	return true
}

func toolSortKey(token string) string {
	if spec, ok := strings.CutPrefix(token, "Bash("); ok {
		group, _, _ := strings.Cut(strings.TrimSuffix(spec, ")"), " ")
		return "1|" + strings.TrimSuffix(group, "*") + "|" + token
	}
	if rest, ok := strings.CutPrefix(token, "mcp__"); ok {
		server, _, _ := strings.Cut(rest, "__")
		return "1|" + server + "|" + token
	}
	return "0|" + token
}

// SplitTableRow honors escaped \| pipes inside cells.
func SplitTableRow(line string) []string {
	const escapedPipe = "\x00"
	prepared := strings.ReplaceAll(strings.TrimSpace(line), `\|`, escapedPipe)
	prepared = strings.Trim(prepared, "|")
	var cells []string
	for cell := range strings.SplitSeq(prepared, "|") {
		cells = append(cells, strings.TrimSpace(strings.ReplaceAll(cell, escapedPipe, `\|`)))
	}
	return cells
}
