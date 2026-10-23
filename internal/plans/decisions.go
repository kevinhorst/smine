package plans

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	addsCell       = "Adds"
	addsLead       = "**Adds**"
	addsNothing    = "nothing"
	clauseBreak    = "<br>"
	decisionsTitle = "Decisions"
	factsCell      = "Facts " + AgentOnlyMarker
	reusesLead     = "**Reuses**"
)

// AddsCutoff is the plan dir date from which the Adds column, the phase cap and the phase table are checked.
const AddsCutoff = "2026-10-06"

var (
	addsCount     = regexp.MustCompile(`^(\d+) (.+)$`)
	addsKinds     = []string{"files", "flags", "commands", "exit codes", "fields", "checks", "agent steps"}
	decisionRowId = regexp.MustCompile(`^D\d+$`)
)

type Decision struct {
	Id       string
	Adds     map[string]int
	Facts    string
	Line     int
	Problems []string
}

type decisionTable struct {
	addsIndex  int
	decisions  []*Decision
	factsIndex int
	hasHeader  bool
	isSection  bool
}

func (t *decisionTable) feed(number int, line string) {
	title, isHeading := strings.CutPrefix(line, sectionPrefix)
	if isHeading {
		t.isSection = strings.TrimSpace(title) == decisionsTitle
		return
	}

	trimmed := strings.TrimSpace(line)
	isRow := t.isSection && strings.HasPrefix(trimmed, "|") && !isSeparatorRow(trimmed)
	if !isRow {
		return
	}

	cells := tableCells(trimmed)
	if !t.hasHeader {
		t.hasHeader = true
		t.addsIndex = indexOf(cells, addsCell)
		t.factsIndex = indexOf(cells, factsCell)
		return
	}
	if !decisionRowId.MatchString(cells[0]) {
		return
	}

	decision := &Decision{Id: cells[0], Facts: cellAt(cells, t.factsIndex), Line: number}
	hasCell := t.addsIndex >= 0 && t.addsIndex < len(cells)
	if hasCell {
		decision.Adds, decision.Problems = parseAdds(cells[t.addsIndex])
	} else {
		decision.Problems = []string{"has no Adds cell"}
	}
	t.decisions = append(t.decisions, decision)
}

func addsKind(name string) string {
	for _, kind := range addsKinds {
		isKind := name == kind || name+"s" == kind
		if isKind {
			return kind
		}
	}
	return ""
}

func indexOf(cells []string, name string) int {
	for index, cell := range cells {
		if cell == name {
			return index
		}
	}
	return -1
}

func parseAdds(cell string) (map[string]int, []string) {
	clauses := strings.Split(cell, clauseBreak)
	counted, isAdds := strings.CutPrefix(strings.TrimSpace(clauses[0]), addsLead)
	if !isAdds {
		return nil, []string{"Adds cell does not open with " + addsLead}
	}

	var problems []string
	reuses := ""
	if len(clauses) > 1 {
		reuses = strings.TrimSpace(clauses[1])
	}
	carrier, isReuses := strings.CutPrefix(reuses, reusesLead)
	hasCarrier := strings.TrimSpace(carrier) != ""
	hasReuses := isReuses && hasCarrier
	if !hasReuses {
		problems = append(problems, "Adds cell names no carrier after "+reusesLead)
	}

	adds := map[string]int{}
	counted = strings.TrimSuffix(strings.TrimSpace(counted), ".")
	if counted == addsNothing {
		return adds, problems
	}
	for item := range strings.SplitSeq(counted, ", ") {
		match := addsCount.FindStringSubmatch(item)
		kind := ""
		if match != nil {
			kind = addsKind(match[2])
		}
		if kind == "" {
			problems = append(problems, fmt.Sprintf("counts %q, which is no kind of the vocabulary", item))
			continue
		}
		count, _ := strconv.Atoi(match[1])
		adds[kind] += count
	}
	return adds, problems
}

func AddsLine(decisions []*Decision) string {
	totals := map[string]int{}
	for _, decision := range decisions {
		for kind, count := range decision.Adds {
			totals[kind] += count
		}
	}

	var parts []string
	for _, kind := range addsKinds {
		count := totals[kind]
		switch {
		case count == 1:
			parts = append(parts, "1 "+strings.TrimSuffix(kind, "s"))
		case count > 1:
			parts = append(parts, fmt.Sprintf("%d %s", count, kind))
		}
	}
	if len(parts) == 0 {
		return "adds: " + addsNothing
	}
	return "adds: " + strings.Join(parts, ", ")
}

func CheckDecisions(decisions []*Decision) []Violation {
	var violations []Violation
	for _, decision := range decisions {
		for _, problem := range decision.Problems {
			violation := Violation{Line: decision.Line, Message: fmt.Sprintf("%s: row %s %s", decisionsTitle, decision.Id, problem)}
			violations = append(violations, violation)
		}
	}
	return violations
}

func IsAddsChecked(path string) bool {
	match := planDirDate.FindStringSubmatch(filepath.ToSlash(path))
	if match == nil {
		return true
	}
	return match[1] >= AddsCutoff
}

func ParseDecisions(markdown string) []*Decision {
	table := &decisionTable{}
	plainLines(markdown, table.feed)
	return table.decisions
}
