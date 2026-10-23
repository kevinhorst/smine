// Command skillplantemplate is an ACDSL verifier: a skill's declared plan template meets the plan interface of rules/plan.md.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/plans"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

const (
	overridePrefix = "<override:"
	templateKey    = "plan-template"
)

type heading struct {
	line int
	text string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillplantemplate: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillplantemplate:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		if !skillgraph.IsModelFile(file) {
			continue
		}
		violations += checkTemplate(file, out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkHeadings(file string, headings []heading, lines []string, out io.Writer) int {
	violations := 0
	if len(headings) == 0 || headings[0].text != plans.Sections[0] {
		fmt.Fprintf(out, "%s:%d: plan template must open with ## %s (rules/plan.md)\n", file, firstLine(headings), plans.Sections[0])
		violations++
	}
	index := plans.SectionIndex()
	last := -1
	for _, entry := range headings {
		name, marked := plans.SplitHeading(entry.text)
		position, isCatalog := index[name]
		if !isCatalog {
			if !isOverride(lines, entry.line) {
				fmt.Fprintf(out, "%s:%d: section %q is not a catalog section and carries no %s placeholder (rules/plan.md)\n", file, entry.line, name, overridePrefix)
				violations++
			}
			continue
		}
		if marked != plans.IsAgentTier(name) {
			fmt.Fprintf(out, "%s:%d: section %q carries the wrong %s marker (rules/plan.md RULE-PLAN-090)\n", file, entry.line, name, plans.AgentOnlyMarker)
			violations++
		}
		if position < last {
			fmt.Fprintf(out, "%s:%d: section %q out of catalog order (rules/plan.md)\n", file, entry.line, name)
			violations++
			continue
		}
		last = position
	}
	return violations
}

func checkTemplate(file string, out io.Writer) int {
	dir := filepath.Dir(file)
	skill, err := skillgraph.Load(dir)
	if err != nil {
		fmt.Fprintf(out, "%s:1: model does not load — plan template unverified (violations: ACDSL-SKILL-005)\n", file)
		return 1
	}

	declared := skill.Field(templateKey)
	if declared == "" {
		return 0
	}

	ref, found := skill.Entry(declared)
	if !found {
		fmt.Fprintf(out, "%s:1: %s %s names no entry of this skill\n", file, templateKey, declared)
		return 1
	}

	if ref.Entry.Payload == nil {
		fmt.Fprintf(out, "%s:1: %s %s is not a [payload] entry\n", file, templateKey, declared)
		return 1
	}

	templatePath := filepath.Join(dir, skillgraph.DirPayloads, declared+".md")
	lines := strings.Split(ref.Entry.Payload.Body, "\n")
	return checkHeadings(templatePath, templateHeadings(lines), lines, out)
}

func firstLine(headings []heading) int {
	if len(headings) == 0 {
		return 1
	}
	return headings[0].line
}

func isOverride(lines []string, headingLine int) bool {
	for index := headingLine; index < len(lines); index++ {
		text := strings.TrimSpace(lines[index])
		if text == "" {
			continue
		}
		return strings.HasPrefix(text, overridePrefix)
	}
	return false
}

func templateHeadings(lines []string) []heading {
	var headings []heading
	for index, line := range lines {
		if text, ok := strings.CutPrefix(line, "## "); ok {
			headings = append(headings, heading{line: index + 1, text: strings.TrimSpace(text)})
		}
	}
	return headings
}
