package plans

import (
	"regexp"
	"strings"
)

const (
	assumptionsTitle = "Assumptions"
	cellAssumption   = "Assumption"
	cellLocation     = "Location " + AgentOnlyMarker
	cellReality      = "Reality " + AgentOnlyMarker
)

var assumptionRowId = regexp.MustCompile(`^A\d+$`)

// Assumption is one row of the Assumptions table: the premise the request states, the verified reality and its evidence.
type Assumption struct {
	Id         string
	Assumption string
	Line       int
	Location   string
	Reality    string
}

type assumptionTable struct {
	assumptionIndex int
	assumptions     []*Assumption
	hasHeader       bool
	isSection       bool
	locationIndex   int
	realityIndex    int
}

func (t *assumptionTable) feed(number int, line string) {
	title, isHeading := strings.CutPrefix(line, sectionPrefix)
	if isHeading {
		t.isSection = strings.TrimSpace(title) == assumptionsTitle
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
		t.assumptionIndex = indexOf(cells, cellAssumption)
		t.locationIndex = indexOf(cells, cellLocation)
		t.realityIndex = indexOf(cells, cellReality)
		return
	}
	if !assumptionRowId.MatchString(cells[0]) {
		return
	}

	assumption := &Assumption{
		Id:         cells[0],
		Assumption: cellAt(cells, t.assumptionIndex),
		Line:       number,
		Location:   cellAt(cells, t.locationIndex),
		Reality:    cellAt(cells, t.realityIndex),
	}
	t.assumptions = append(t.assumptions, assumption)
}

// cellAt is the cell's text, empty for a column the header lacks or a row too short.
func cellAt(cells []string, index int) string {
	if index < 0 || index >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[index])
}

func ParseAssumptions(markdown string) []*Assumption {
	table := &assumptionTable{}
	plainLines(markdown, table.feed)
	return table.assumptions
}
