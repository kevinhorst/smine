// Command skillmeta is an ACDSL verifier: a skill model or SKILL.md fixture passes the metadata check its mode= names.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const modeRouting, modeRuntimeMeta, modeVersionSync = "routing", "runtime-meta", "version-sync"

// modelModes take model file paths straight to a check over the loaded model.
var modelModes = []string{modeRouting, modeRuntimeMeta, modeVersionSync}

// sortRequired arms the allowed-tools normalization check (sort=required).
var sortRequired bool

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillmeta: usage: <files-list> mode=<mode>")
		return 2
	}
	mode := ""
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "mode="); ok {
			mode = value
		}
		if value, ok := strings.CutPrefix(arg, "sort="); ok {
			sortRequired = value == "required"
		}
	}
	check, ok := map[string]func(string, []string, io.Writer) int{
		modeVersionSync: checkModelLoads,
		modeRuntimeMeta: checkModelLoads,
		"frontmatter":   checkFrontmatter,
		"allowed-tools": checkAllowedTools,
		"argument-hint": checkArgumentHint,
		"args-table":    checkArgsTable,
		modeRouting:     checkRouting,
		"context-links": checkContextLinks,
	}[mode]
	if !ok {
		fmt.Fprintf(os.Stderr, "skillmeta: unknown mode %q\n", mode)
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmeta:", err)
		return 2
	}
	isModelMode := slices.Contains(modelModes, mode)
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		base := filepath.Base(file)
		if isModelMode && skillgraph.IsModelFile(file) {
			violations += check(file, nil, out)
			continue
		}

		isManifestSource := skillgraph.IsModelFile(file) || base == skillgraph.FileManifest
		if isModelMode || !isManifestSource {
			continue
		}

		raw, err := skillgraph.ReadManifest(file)
		if err != nil {
			fmt.Fprintf(out, "%s:1: SKILL.md text unavailable — the model does not load or render (violations: ACDSL-SKILL-005)\n", file)
			violations++
			continue
		}
		violations += check(skillgraph.ManifestLabel(file), strings.Split(string(raw), "\n"), out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// Only same-line values are captured: a block-scalar value maps to "".
func frontmatter(lines []string) (map[string]string, map[string]int) {
	values, lineNos := map[string]string{}, map[string]int{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return values, lineNos
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		key, value, ok := strings.Cut(lines[i], ":")
		if !ok || strings.Contains(key, " ") {
			continue
		}
		values[key] = strings.TrimSpace(value)
		lineNos[key] = i + 1
	}
	return values, lineNos
}

// checkModelLoads: the model check holds the version agreement and the runtime placement rules.
func checkModelLoads(file string, _ []string, out io.Writer) int {
	if _, err := skillgraph.Load(filepath.Dir(file)); err != nil {
		fmt.Fprintf(out, "%s:1: model does not load — %v\n", file, err)
		return 1
	}
	return 0
}

func checkFrontmatter(file string, lines []string, out io.Writer) int {
	values, lineNos := frontmatter(lines)
	violations := 0
	for _, key := range []string{"name", "description", "author", "version"} {
		if values[key] == "" {
			fmt.Fprintf(out, "%s:1: frontmatter %s missing or not single-line\n", file, key)
			violations++
		}
	}
	description := values["description"]
	if description != "" && !strings.Contains(description, "Trigger on") {
		fmt.Fprintf(out, "%s:%d: description lacks the \"Trigger on\" clause\n", file, lineNos["description"])
		violations++
	}
	hasArgs := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## Args") {
			hasArgs = true
			break
		}
	}
	if hasArgs && description != "" && !strings.Contains(description, "Args —") {
		fmt.Fprintf(out, "%s:%d: skill has ## Args but the description lacks an \"Args —\" summary\n", file, lineNos["description"])
		violations++
	}
	return violations
}

// Hyphens occur in MCP tool names whenever the server name carries one (mcp__peek-mcp__session_list).
var permissionRuleRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*(\(.+\))?$`)

func checkAllowedTools(file string, lines []string, out io.Writer) int {
	values, lineNos := frontmatter(lines)
	value, present := values["allowed-tools"]
	if !present || value == "" {
		return 0
	}
	violations := 0
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if !permissionRuleRe.MatchString(item) {
			fmt.Fprintf(out, "%s:%d: allowed-tools entry %q is not a permission rule\n", file, lineNos["allowed-tools"], item)
			violations++
		}
		if strings.HasPrefix(item, "Bash(") && strings.Contains(item, ":") {
			fmt.Fprintf(out, "%s:%d: Bash entry %q uses a colon form — the repo dialect is space wildcards\n", file, lineNos["allowed-tools"], item)
			violations++
		}
		if !skills.IsGrantablePathRule(item) {
			fmt.Fprintf(out, "%s:%d: path rule %q grants nothing — the spec must be ~/<dir>/**, //<abs dir>/** or @repos/**\n", file, lineNos["allowed-tools"], item)
			violations++
		}
	}
	if sortRequired && !skills.ToolsSorted(value) {
		fmt.Fprintf(out, "%s:%d: allowed-tools is not normalized — plain tools first (alphabetical), then Bash/MCP entries grouped and sorted\n", file, lineNos["allowed-tools"])
		violations++
	}
	return violations
}

func checkArgsTable(file string, lines []string, out io.Writer) int {
	return printFormatViolations(file, out, skills.CheckArgsTable(skills.ParseArgsSection(lines)))
}

func checkArgumentHint(file string, lines []string, out io.Writer) int {
	values, lineNos := frontmatter(lines)
	hint := strings.Trim(values["argument-hint"], `"'`)
	leaf := filepath.Base(filepath.Dir(file))
	return printFormatViolations(file, out, skills.CheckArgumentHint(leaf, hint, lineNos["argument-hint"], skills.ParseArgsSection(lines)))
}

func checkContextLinks(file string, lines []string, out io.Writer) int {
	return printFormatViolations(file, out, skills.CheckContextIds(lines))
}

func checkRouting(file string, _ []string, out io.Writer) int {
	skill, err := skillgraph.Load(filepath.Dir(file))
	if err != nil {
		fmt.Fprintf(out, "%s:1: model does not load — routing unverified (violations: ACDSL-SKILL-005)\n", file)
		return 1
	}

	if skill.Routing() == nil {
		fmt.Fprintf(out, "%s:1: no routing block — the Routing section carries purpose, routes, preconditions and pipeline as a typed block\n", file)
		return 1
	}
	return 0
}

func printFormatViolations(file string, out io.Writer, violations []skills.FormatViolation) int {
	for _, violation := range violations {
		fmt.Fprintf(out, "%s:%d: %s\n", file, violation.Line, violation.Message)
	}
	return len(violations)
}
