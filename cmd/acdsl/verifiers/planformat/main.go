// Command planformat is a registered ACDSL verifier: the lintable subset
// of rules/plan.md over design plans. Three checks: the first "## " section
// is TLDR; a "## Changelog" section exists; sections named in the canonical
// order (RULE-PLAN-002, one order for every route) appear in that relative
// order. Unknown headings are ignored; no prose heuristics.
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line: message; exit 0 pass, 1 violations, 2 error. No params.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var canon = []string{"TLDR", "Context", "Drivers", "Scope", "Assumptions", "Current state", "Target state", "Behavior contract", "Decisions", "Open questions", "Baseline (verified)", "Exemplar & reuse", "Changes", "Hot items", "Tests", "Test runbook", "Contracts & sweeps", "Verification", "Stop conditions", "Changelog"}

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
		for i, line := range strings.Split(string(raw), "\n") {
			if text, ok := strings.CutPrefix(line, "## "); ok {
				headings = append(headings, heading{text: strings.TrimSpace(text), line: i + 1})
			}
		}
		violations += checkFile(file, headings, out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkFile(file string, headings []heading, out io.Writer) int {
	violations := 0
	if len(headings) == 0 || headings[0].text != "TLDR" {
		fmt.Fprintf(out, "%s:1: first section must be ## TLDR (rules/plan.md RULE-PLAN-002)\n", file)
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
	for _, problem := range orderProblems(headings, canon) {
		fmt.Fprintf(out, "%s:%d: section %q out of canonical order (rules/plan.md)\n", file, problem.line, problem.text)
		violations++
	}
	return violations
}

// orderProblems returns headings whose canon index precedes an already-seen
// canon index — the out-of-order set relative to the given canon.
func orderProblems(headings []heading, canon []string) []heading {
	index := map[string]int{}
	for i, name := range canon {
		index[name] = i
	}
	var problems []heading
	last := -1
	for _, entry := range headings {
		position, known := index[entry.text]
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
