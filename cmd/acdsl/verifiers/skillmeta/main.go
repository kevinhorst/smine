// Command skillmeta is a registered ACDSL verifier over SKILL.md files.
// Modes (params mode=):
//
//	version-sync  — frontmatter version == sibling changelog.json[0].version
//	                (bare versions)
//	frontmatter   — name/description/author/version present, single-line,
//	                non-empty; description carries "Trigger on"; a "## Args"
//	                section requires "Args —" in the description
//	allowed-tools — comma-separated permission rules in the settings-allowlist
//	                dialect; Bash entries never use colon wildcards
//
// Contract: args = <files-list path> [key=value...]; one violation per stdout
// line as file:line: message; exit 0 pass, 1 violations, 2 error.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

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
	}
	check, ok := map[string]func(string, []string, io.Writer) int{
		"version-sync":  checkVersionSync,
		"frontmatter":   checkFrontmatter,
		"allowed-tools": checkAllowedTools,
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
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		if filepath.Base(file) != "SKILL.md" {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillmeta:", err)
			return 2
		}
		violations += check(file, strings.Split(string(raw), "\n"), out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// frontmatter returns key → value and key → line number for the block
// between the opening and closing "---" fences. Only same-line values are
// captured: a block-scalar value maps to "".
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

// changelogVersionEntry is one record of the sibling changelog.json; only the
// version field participates in the version-sync check.
type changelogVersionEntry struct {
	Version string `json:"version"`
}

func checkVersionSync(file string, lines []string, out io.Writer) int {
	values, lineNos := frontmatter(lines)
	declared := values["version"]
	if declared == "" {
		fmt.Fprintf(out, "%s:1: frontmatter version missing\n", file)
		return 1
	}
	sidecar := filepath.Join(filepath.Dir(file), "changelog.json")
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		fmt.Fprintf(out, "%s:%d: changelog.json missing next to SKILL.md\n", file, lineNos["version"])
		return 1
	}
	var entries []changelogVersionEntry
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) == 0 {
		fmt.Fprintf(out, "%s:%d: changelog.json unparseable or empty\n", file, lineNos["version"])
		return 1
	}
	if entries[0].Version != declared {
		fmt.Fprintf(out, "%s:%d: changelog.json[0].version %q != frontmatter %q\n", file, lineNos["version"], entries[0].Version, declared)
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

// Hyphens occur in MCP tool names whenever the server name carries one
// (mcp__peek-mcp__session_list).
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
	}
	return violations
}
