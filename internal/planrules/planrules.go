// Package planrules models a plan's frozen permission rules; the tool-policy gate is the ledger's only writer.
package planrules

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	// FileName is a plan dir's rules file, ignored by git.
	FileName = "permissions.json"
	// LedgerDirName is the gate's state directory under the Claude home.
	LedgerDirName = "tool-policy"
	// LedgerName is the frozen-rules ledger inside that directory.
	LedgerName = "frozen.jsonl"

	planFileSuffix = ".md"
)

// The plan dir path, checkout-relative and slash-separated; group 1 is the slug.
var planDirPattern = regexp.MustCompile(`^plans/\d{4}-\d{2}-\d{2}-[^/]+/\d{4}-\d{2}-\d{2}-([^/]+)-wip-[^/]+$`)

// Group 1 is the plan dir, group 2 its slug, group 3 the file's stem.
var planFilePattern = regexp.MustCompile(`^(plans/\d{4}-\d{2}-\d{2}-[^/]+/\d{4}-\d{2}-\d{2}-([^/]+)-wip-[^/]+)/design/([^/]+)\.md$`)

type File struct {
	Checked []string `json:"checked"`
	Rules   []*Rule  `json:"rules"`
}

func (f *File) Equals(other *File) bool {
	hasSameRules := slices.EqualFunc(f.Rules, other.Rules, func(left *Rule, right *Rule) bool {
		return *left == *right
	})
	return hasSameRules && slices.Equal(f.Checked, other.Checked)
}

// RuleStrings are the rule texts in file order.
func (f *File) RuleStrings() []string {
	rules := make([]string, 0, len(f.Rules))
	for _, rule := range f.Rules {
		rules = append(rules, rule.Rule)
	}
	return rules
}

// The latest line per repo and plan is the plan's frozen state.
type Record struct {
	*File
	Plan      string `json:"plan"`
	Repo      string `json:"repo"`
	SessionId string `json:"session_id"`
	Timestamp string `json:"ts"`
}

type Rule struct {
	NeededFor string `json:"needed_for"`
	Rule      string `json:"rule"`
	Target    string `json:"target"`
}

// A dir that does not resolve is compared as cleaned text.
func resolveDir(dir string) string {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	return resolved
}

// viewCell escapes the one character that would end a table cell.
func viewCell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}

// viewRuleLines are the view's rule table, or its one line for an empty set.
func viewRuleLines(rules []*Rule) []string {
	if len(rules) == 0 {
		return []string{"- no rule needed"}
	}

	lines := []string{"| Rule | Command | Needed for |", "|---|---|---|"}
	for _, rule := range rules {
		lines = append(lines, "| `"+viewCell(rule.Rule)+"` | `"+viewCell(rule.Target)+"` | "+viewCell(rule.NeededFor)+" |")
	}
	return lines
}

func IsPlanDir(dir string) bool {
	return planDirPattern.MatchString(dir)
}

// The plan file is the design file named after the dir's slug.
func IsPlanFile(path string) bool {
	return PlanDir(path) != ""
}

// Nil when the plan was never frozen.
func Latest(plan string, records []*Record) *Record {
	for _, record := range slices.Backward(records) {
		if record.Plan == plan {
			return record
		}
	}
	return nil
}

// Strict decoding, so a typo never silently drops a rule.
func LoadFile(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadFile: %w", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()

	file := &File{}
	if err := decoder.Decode(file); err != nil {
		return nil, fmt.Errorf("LoadFile: %s does not decode: %w", path, err)
	}
	if slices.Contains(file.Rules, nil) {
		return nil, fmt.Errorf("LoadFile: %s holds an empty rule", path)
	}
	return file, nil
}

// A missing ledger or an unparseable line contributes nothing.
func LoadRecords(ledger string, repo string) []*Record {
	records := make([]*Record, 0)
	file, err := os.Open(ledger)
	if err != nil {
		return records
	}
	defer file.Close()

	resolvedRepo := resolveDir(repo)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		record := &Record{}
		if err := json.Unmarshal(scanner.Bytes(), record); err != nil {
			continue
		}

		isOfRepo := record.File != nil && resolveDir(record.Repo) == resolvedRepo
		if isOfRepo {
			records = append(records, record)
		}
	}
	return records
}

func PlanDir(path string) string {
	match := planFilePattern.FindStringSubmatch(path)
	isPlanFile := match != nil && match[2] == match[3]
	if !isPlanFile {
		return ""
	}
	return match[1]
}

func PlanFileName(dir string) string {
	match := planDirPattern.FindStringSubmatch(dir)
	if match == nil {
		return ""
	}
	return match[1] + planFileSuffix
}

func ViewLines(file *File) []string {
	checked := make([]string, 0, len(file.Checked))
	for _, target := range file.Checked {
		checked = append(checked, "`"+target+"`")
	}

	lines := []string{"**Plan rules** — from " + FileName + ", shown in this display copy only", ""}
	lines = append(lines, viewRuleLines(file.Rules)...)
	return append(lines, "", "- checked: "+strings.Join(checked, ", "), "")
}
