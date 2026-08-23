// Command linttags is a registered ACDSL verifier: it fails (exit 1) when a
// style guide's linter-coverage claims disagree with the repo's golangci
// config. A "* Covered by `<linter>`" bullet claims CI proves that slice, and
// a `[lint]` headline tag tells reviewers to skip the rule — so a claim
// naming a linter that is not enabled means the rule is checked by nobody.
//
// Contract: args = <files-list path> [config=<filename>]; one violation per
// stdout line as file:line: message; exit 0 pass, 1 violations, 2 error.
// The config (default .golangci.yml) is resolved per guide file by walking up
// from the guide's directory; a guide with no config above it passes — the
// claims are only checkable where the named tooling runs.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	headlineRe = regexp.MustCompile("^\\*\\*(RULE-[A-Z0-9-]+-[0-9]{3})\\*\\* `\\[([a-z]+)\\]`")
	coveredRe  = regexp.MustCompile(`^\* Covered by (.+)$`)
	tokenRe    = regexp.MustCompile("`([^`]+)`")
	checkRe    = regexp.MustCompile(`^(ST|QF)[0-9]+$`)
)

// lintConfig is the slice of a golangci config the coverage claims are
// checked against, extracted line-based to mirror the retired check-tags.sh.
type lintConfig struct {
	disabledChecks map[string][]string
	enabled        map[string]bool
	reviveRules    map[string]bool
	staticExcluded map[string]bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

// claimSegment cuts a coverage bullet to its claim sentence: everything up to
// the first ". " or " — ", so trailing prose may name non-linter backticks.
func claimSegment(text string) string {
	segment := text
	if idx := strings.Index(segment, ". "); idx >= 0 {
		segment = segment[:idx]
	}
	if idx := strings.Index(segment, " — "); idx >= 0 {
		segment = segment[:idx]
	}
	return segment
}

// checkToken validates one backticked coverage token against the config.
func checkToken(token string, config *lintConfig) string {
	linter := token
	subcheck := ""
	if idx := strings.Index(token, "/"); idx >= 0 {
		linter = token[:idx]
		subcheck = token[idx+1:]
	} else if fields := strings.Fields(token); len(fields) == 2 && checkRe.MatchString(fields[1]) {
		linter = fields[0]
		if !config.enabled[linter] {
			return fmt.Sprintf("%s is not enabled but a coverage claim names it", linter)
		}
		if config.staticExcluded[fields[1]] {
			return fmt.Sprintf("%s is excluded by the %s checks pin but a coverage claim names it", fields[1], linter)
		}
		return ""
	}
	if !config.enabled[linter] {
		return fmt.Sprintf("%s is neither an enabled linter nor a formatter", linter)
	}
	if subcheck == "" {
		return ""
	}
	if linter == "revive" && !config.reviveRules[subcheck] {
		return fmt.Sprintf("revive/%s is not in the revive rule list", subcheck)
	}
	for _, disabled := range config.disabledChecks[linter] {
		if disabled == subcheck {
			return fmt.Sprintf("%s/%s is disabled in the config but a coverage claim names it", linter, subcheck)
		}
	}
	return ""
}

// findConfig walks up from the guide file's directory looking for name.
func findConfig(guidePath, name string) string {
	dir := filepath.Dir(guidePath)
	for {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func loadConfig(path string) (*lintConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := &lintConfig{
		disabledChecks: map[string][]string{},
		enabled:        map[string]bool{},
		reviveRules:    map[string]bool{},
		staticExcluded: map[string]bool{},
	}
	section := ""
	settingsLinter := ""
	isInEnable := false
	isInDisable := false
	for _, line := range strings.Split(string(raw), "\n") {
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			section = strings.TrimSuffix(strings.TrimSpace(line), ":")
			settingsLinter = ""
			isInEnable = false
			continue
		}
		trimmed := strings.TrimSpace(line)
		if section != "linters" && section != "formatters" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(trimmed, ":"):
			isInEnable = trimmed == "enable:"
			settingsLinter = ""
		case strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") && strings.HasSuffix(trimmed, ":"):
			settingsLinter = strings.TrimSuffix(trimmed, ":")
			isInDisable = false
		case isInEnable && strings.HasPrefix(trimmed, "- "):
			config.enabled[strings.TrimSpace(trimmed[2:])] = true
		case settingsLinter == "revive" && strings.HasPrefix(trimmed, "- name: "):
			config.reviveRules[strings.TrimSpace(trimmed[len("- name: "):])] = true
		case settingsLinter == "staticcheck" && strings.HasPrefix(trimmed, "- -"):
			config.staticExcluded[strings.TrimSpace(trimmed[3:])] = true
		case settingsLinter != "" && strings.HasSuffix(trimmed, ":"):
			isInDisable = trimmed == "disable:"
		case settingsLinter != "" && isInDisable && strings.HasPrefix(trimmed, "- "):
			config.disabledChecks[settingsLinter] = append(config.disabledChecks[settingsLinter], strings.TrimSpace(trimmed[2:]))
		}
	}
	return config, nil
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "linttags: usage: <files-list> [config=<filename>]")
		return 2
	}
	configName := ".golangci.yml"
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "config="); ok {
			configName = value
		}
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "linttags:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		configPath := findConfig(file, configName)
		if configPath == "" {
			continue
		}
		config, err := loadConfig(configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "linttags:", err)
			return 2
		}
		if len(config.enabled) == 0 {
			fmt.Fprintf(out, "%s:1: could not read an enable list from %s — the claim checks below would be vacuous\n", file, configPath)
			violations++
			continue
		}
		violations += checkGuide(file, config, out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkGuide(file string, config *lintConfig, out io.Writer) int {
	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "linttags:", err)
		os.Exit(2)
	}
	violations := 0
	isInFence := false
	lintHeadline := ""
	lintHeadlineLine := 0
	hasCoverage := false
	flushHeadline := func() {
		if lintHeadline != "" && !hasCoverage {
			fmt.Fprintf(out, "%s:%d: %s is tagged [lint] but no \"Covered by\" bullet names a linter\n", file, lintHeadlineLine, lintHeadline)
			violations++
		}
		lintHeadline = ""
		hasCoverage = false
	}
	for lineIdx, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			isInFence = !isInFence
			continue
		}
		if isInFence {
			continue
		}
		if match := headlineRe.FindStringSubmatch(line); match != nil {
			flushHeadline()
			if match[2] == "lint" {
				lintHeadline = match[1]
				lintHeadlineLine = lineIdx + 1
			}
			continue
		}
		match := coveredRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		hasCoverage = true
		for _, token := range tokenRe.FindAllStringSubmatch(claimSegment(match[1]), -1) {
			if problem := checkToken(token[1], config); problem != "" {
				fmt.Fprintf(out, "%s:%d: %s\n", file, lineIdx+1, problem)
				violations++
			}
		}
	}
	flushHeadline()
	return violations
}
