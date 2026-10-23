// Command planformat is an ACDSL verifier: a persisted plan meets the lintable subset of rules/plan.md.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/plans"
)

var permissionsHeading = regexp.MustCompile(`^#{2,6} +Permissions\s*$`)

var oldDepthPlan = regexp.MustCompile(`(^|/)plans/[^/]+-wip-[^/]+/design/[^/]+\.md$`)

type heading struct {
	text string
	line int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "planformat: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "planformat:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "planformat:", err)
			return 2
		}
		var headings []heading
		var permissions []heading
		fence := ""
		for i, line := range strings.Split(string(raw), "\n") {
			fence = plans.NextFence(fence, line)
			if fence != "" {
				continue
			}
			if text, ok := strings.CutPrefix(line, "## "); ok {
				headings = append(headings, heading{text: strings.TrimSpace(text), line: i + 1})
			}
			if permissionsHeading.MatchString(line) {
				permissions = append(permissions, heading{text: strings.TrimSpace(line), line: i + 1})
			}
		}
		violations += checkFile(file, headings, permissions, plans.ParseChanges(string(raw)), out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkFile(file string, headings []heading, permissions []heading, unit *plans.Unit, out io.Writer) int {
	violations := 0
	isOrderChecked := plans.IsOrderChecked(file)
	if oldDepthPlan.MatchString(file) {
		fmt.Fprintf(out, "%s:1: plan dir sits directly under plans/ — move it into its feature folder plans/<started_at>-<feature>-<id>/ (rules/plan.md RULE-PLAN-071)\n", file)
		violations++
	}
	if len(headings) == 0 || headings[0].text != plans.Sections[0] {
		fmt.Fprintf(out, "%s:1: first section must be ## %s (rules/plan.md RULE-PLAN-002)\n", file, plans.Sections[0])
		violations++
	}
	hasChangelog := false
	for _, entry := range headings {
		if entry.text == "Changelog" {
			hasChangelog = true
		}
	}
	if !hasChangelog {
		fmt.Fprintf(out, "%s:1: ## Changelog section missing\n", file)
		violations++
	}
	for _, problem := range markerProblems(headings) {
		fmt.Fprintf(out, "%s:%d: %s\n", file, problem.line, problem.text)
		violations++
	}
	if isOrderChecked {
		for _, problem := range orderProblems(headings, plans.Sections) {
			fmt.Fprintf(out, "%s:%d: section %q out of canonical order (rules/plan.md)\n", file, problem.line, problem.text)
			violations++
		}
	}
	for _, entry := range permissions {
		fmt.Fprintf(out, "%s:%d: plan carries a Permissions section — its rules live in the plan dir's permissions.json (rules/plan.md RULE-PLAN-082)\n", file, entry.line)
		violations++
	}
	if unit == nil {
		return violations
	}
	for _, violation := range plans.CheckUnit(unit, false) {
		fmt.Fprintf(out, "%s:%d: %s (rules/plan.md RULE-PLAN-088)\n", file, violation.Line, violation.Message)
		violations++
	}
	if !isOrderChecked {
		return violations
	}
	for _, violation := range plans.CheckOrder(unit) {
		fmt.Fprintf(out, "%s:%d: %s (rules/plan.md RULE-PLAN-098)\n", file, violation.Line, violation.Message)
		violations++
	}
	return violations
}

func markerProblems(headings []heading) []heading {
	index := plans.SectionIndex()
	var problems []heading
	for _, entry := range headings {
		name, marked := plans.SplitHeading(entry.text)
		_, isCatalog := index[name]
		if !isCatalog {
			continue
		}

		wantMarker := plans.IsAgentTier(name)
		if marked == wantMarker {
			continue
		}
		message := fmt.Sprintf("section %q must carry %s (rules/plan.md RULE-PLAN-090)", name, plans.AgentOnlyMarker)
		if marked {
			message = fmt.Sprintf("section %q must not carry %s (rules/plan.md RULE-PLAN-090)", name, plans.AgentOnlyMarker)
		}
		problems = append(problems, heading{text: message, line: entry.line})
	}
	return problems
}

func orderProblems(headings []heading, canon []string) []heading {
	index := map[string]int{}
	for i, name := range canon {
		index[name] = i
	}
	var problems []heading
	last := -1
	for _, entry := range headings {
		name, _ := plans.SplitHeading(entry.text)
		position, known := index[name]
		if !known {
			continue
		}
		if position < last {
			problems = append(problems, entry)
			continue
		}
		last = position
	}
	return problems
}
