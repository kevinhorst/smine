package reviewscope

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	healthFailed  = "failed"
	healthSkipped = "skipped"
	healthUnknown = "unknown"

	dispositionDebunked = "debunked"
	dispositionRejected = "rejected-with-reason"
	stateCarried        = "carried"
	stateNew            = "new"
)

// dispositionRow is one dispositions[] entry of review.json.
type dispositionRow struct {
	Disposition string `json:"disposition"`
	FindingId   string `json:"finding_id"`
	Reason      string `json:"reason"`
}

// doneRow is one Definition of Done criterion.
type doneRow struct {
	Criterion string `json:"criterion"`
	Notes     string `json:"notes"`
	Status    string `json:"status"`
}

// escalationRow is one suspected defect in a high-stakes class.
type escalationRow struct {
	FindingId string `json:"finding_id"`
	Why       string `json:"why"`
}

// funnelRow is the round's claim funnel.
type funnelRow struct {
	Confirmed  int `json:"confirmed"`
	Debunked   int `json:"debunked"`
	Duplicates int `json:"duplicates"`
	Produced   int `json:"produced"`
	Rejected   int `json:"rejected"`
	Unverified int `json:"unverified"`
}

// healthRow is empty when the review.json predates the field.
type healthRow struct {
	Build string `json:"build"`
	Tests string `json:"tests"`
}

// presentDocument is the slice of review.json the gate presentation reads.
type presentDocument struct {
	BaseHealth       healthRow        `json:"base_health"`
	BuildTest        healthRow        `json:"build_test"`
	CoverageGaps     []string         `json:"coverage_gaps"`
	DefinitionOfDone []doneRow        `json:"definition_of_done"`
	Dispositions     []dispositionRow `json:"dispositions"`
	Escalations      []escalationRow  `json:"escalations"`
	Funnel           funnelRow        `json:"funnel"`
	Probes           []probeRow       `json:"probes"`
	RankedPlan       []rankedEntry    `json:"ranked_plan"`
	Recommendation   string           `json:"recommendation"`
	Unverified       []reasonRow      `json:"unverified"`
}

// presenter holds one render's inputs.
type presenter struct {
	document   *presentDocument
	ignoreRank int
	isSection  bool
	known      map[string]FixState
	level      string
	out        strings.Builder
}

func (p *presenter) heading(title string) {
	if p.isSection {
		fmt.Fprintf(&p.out, "\n### %s\n\n", title)
		return
	}
	fmt.Fprintf(&p.out, "\n%s:\n\n", title)
}

func (p *presenter) stateOf(entry rankedEntry) string {
	if !entry.Known {
		return stateNew
	}

	state, isLedgered := p.known[entry.KnownId]
	if !isLedgered {
		return stateCarried
	}
	return stateCarried + ": " + string(state)
}

func (p *presenter) writeCoverage() {
	if len(p.document.CoverageGaps) == 0 {
		return
	}
	fmt.Fprintf(&p.out, "Uncovered: %s\n", strings.Join(p.document.CoverageGaps, ", "))
}

// writeDone writes the Definition of Done summary table.
func (p *presenter) writeDone() {
	p.heading("Definition of Done")
	p.out.WriteString("| Criterion | Status | Notes |\n| --- | --- | --- |\n")
	for _, row := range p.document.DefinitionOfDone {
		fmt.Fprintf(&p.out, "| %s | %s | %s |\n", cell(row.Criterion), row.Status, cell(row.Notes))
	}
}

func (p *presenter) writeEscalations() {
	if len(p.document.Escalations) == 0 {
		return
	}

	p.heading("Escalations")
	for _, escalation := range p.document.Escalations {
		fmt.Fprintf(&p.out, "- %s: %s\n", escalation.FindingId, escalation.Why)
	}
	p.out.WriteString("\n")
}

// writeFindings writes the per-finding table (TPL-003 columns); rows at or below the ignore level become one line.
func (p *presenter) writeFindings() {
	ignored := make([]string, 0)
	rows := make([]rankedEntry, 0, len(p.document.RankedPlan))
	for _, entry := range p.document.RankedPlan {
		if !entry.Known && isIgnored(entry, p.ignoreRank) {
			ignored = append(ignored, entry.FindingId)
			continue
		}
		rows = append(rows, entry)
	}

	if len(rows) == 0 {
		p.out.WriteString("No confirmed findings.\n")
	} else {
		p.out.WriteString("| ID | Severity | State | Route | Location | Finding | Proposed fix |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	}
	for _, entry := range rows {
		location := fmt.Sprintf("[%s:%d](%s:%d)", entry.File, entry.Line, entry.File, entry.Line)
		route := entry.Route
		if entry.RouteReason != "" {
			route += " — " + cell(entry.RouteReason)
		}
		fmt.Fprintf(&p.out, "| %s | %s | %s | %s | %s | %s | %s |\n",
			entry.FindingId, entry.Severity, p.stateOf(entry), route, location, cell(entry.Claim), cell(entry.Fix))
	}

	if len(ignored) > 0 {
		fmt.Fprintf(&p.out, "\nIgnored (at or below %s): %s\n", p.level, strings.Join(ignored, ", "))
	}
}

func (p *presenter) writeFunnel() {
	funnel := p.document.Funnel
	fmt.Fprintf(&p.out, "Funnel: %d produced, %d confirmed, %d unverified, %d duplicates, %d rejected, %d debunked\n",
		funnel.Produced, funnel.Confirmed, funnel.Unverified, funnel.Duplicates, funnel.Rejected, funnel.Debunked)
}

// writeHealth writes the base and head build/test lines.
func (p *presenter) writeHealth() {
	fmt.Fprintf(&p.out, "Base health: build %s, tests %s\n", healthWord(p.document.BaseHealth.Build), healthWord(p.document.BaseHealth.Tests))
	fmt.Fprintf(&p.out, "Head health: build %s, tests %s\n", healthWord(p.document.BuildTest.Build), healthWord(p.document.BuildTest.Tests))
}

func (p *presenter) writeProbes() {
	if len(p.document.Probes) == 0 {
		return
	}

	p.heading("Probes")
	p.out.WriteString("| Probe | Finding | Verdict |\n| --- | --- | --- |\n")
	for _, probe := range p.document.Probes {
		fmt.Fprintf(&p.out, "| %s | %s | %s |\n", probe.Id, probe.FindingId, cell(probe.Verdict))
	}
}

// writeRefused lists the round's debunked and human-rejected claims with their reasons.
func (p *presenter) writeRefused() {
	p.heading("Refused this round")
	refused := make([]dispositionRow, 0)
	for _, row := range p.document.Dispositions {
		isRefused := row.Disposition == dispositionDebunked || row.Disposition == dispositionRejected
		if isRefused {
			refused = append(refused, row)
		}
	}

	if len(refused) == 0 {
		p.out.WriteString("none\n")
		return
	}
	p.out.WriteString("| ID | Disposition | Reason |\n| --- | --- | --- |\n")
	for _, row := range refused {
		fmt.Fprintf(&p.out, "| %s | %s | %s |\n", row.FindingId, row.Disposition, cell(row.Reason))
	}
}

func (p *presenter) writeUnverified() {
	if len(p.document.Unverified) == 0 {
		return
	}

	p.heading("Unverified")
	p.out.WriteString("| ID | Missing precondition |\n| --- | --- |\n")
	for _, row := range p.document.Unverified {
		fmt.Fprintf(&p.out, "| %s | %s |\n", row.FindingId, cell(row.Reason))
	}
}

// PresentRequest reads the known ledger beside the round dir; an empty IgnoreLevel reads none.
type PresentRequest struct {
	Heading     string
	IgnoreLevel string
	ReviewPath  string
}

// probeRow is one probes[] entry as the presentation shows it.
type probeRow struct {
	FindingId string `json:"finding_id"`
	Id        string `json:"id"`
	Verdict   string `json:"verdict"`
}

// reasonRow is one unverified[] entry.
type reasonRow struct {
	FindingId string `json:"finding_id"`
	Reason    string `json:"reason"`
}

// cell keeps a value inside one markdown table cell.
func cell(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "|", "\\|")
}

func healthWord(result string) string {
	switch {
	case result == "":
		return healthUnknown
	case strings.HasPrefix(result, healthFailed+":"):
		return healthFailed
	case strings.HasPrefix(result, healthSkipped+":"):
		return healthSkipped
	}
	return result
}

// readKnownStates maps every known.json row's finding id to its fix state; an absent file maps nothing.
func readKnownStates(path string) (map[string]FixState, error) {
	states := map[string]FixState{}
	if _, err := os.Stat(path); err != nil {
		return states, nil
	}

	var known []KnownEntry
	if err := readJson(path, &known); err != nil {
		return nil, err
	}

	for _, row := range known {
		states[row.FindingId] = row.FixState
	}
	return states, nil
}

func Present(request PresentRequest) (string, error) {
	ignoreRank, isKnownLevel := levelRankOf(request.IgnoreLevel)
	if !isKnownLevel {
		return "", fmt.Errorf("Present: Unknown ignore level %s", request.IgnoreLevel)
	}

	var document presentDocument
	if err := readJson(request.ReviewPath, &document); err != nil {
		return "", fmt.Errorf("Present: %w", err)
	}

	known, err := readKnownStates(filepath.Join(filepath.Dir(filepath.Dir(request.ReviewPath)), knownFile))
	if err != nil {
		return "", fmt.Errorf("Present: %w", err)
	}

	level := request.IgnoreLevel
	if level == "" {
		level = "none"
	}
	render := &presenter{document: &document, ignoreRank: ignoreRank, isSection: request.Heading != "", known: known, level: level}
	if render.isSection {
		fmt.Fprintf(&render.out, "## Review — %s\n", request.Heading)
	}
	render.writeEscalations()
	if render.isSection {
		render.out.WriteString("\n")
	}
	render.writeFindings()
	render.writeRefused()
	render.writeUnverified()
	render.writeProbes()
	render.writeDone()
	render.out.WriteString("\n")
	render.writeHealth()
	render.writeCoverage()
	if render.isSection {
		fmt.Fprintf(&render.out, "\n### Recommendation\n\n%s\n", document.Recommendation)
		return render.out.String(), nil
	}
	render.writeFunnel()
	fmt.Fprintf(&render.out, "\nRecommendation: %s\n", document.Recommendation)
	return render.out.String(), nil
}
