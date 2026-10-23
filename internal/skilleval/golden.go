package skilleval

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/reviewscope"
)

const (
	judgeFile                  = "judge.json"
	metricEscapedMinutes       = "escaped_minutes"
	metricFalsePositiveMinutes = "false_positive_minutes"
	metricFalsePositives       = "false_positives"
	metricLostMinutes          = "lost_minutes"
	metricMisclassified        = "misclassified"
	metricRecall               = "recall"
	metricReviewScore          = "review_score"
	noteInvalidRun             = "invalid run"
	noteZeroDenominator        = "zero denominator"
	pairSeparator              = "\x00"
)

// goldenScorer: oracle is every golden entry fixed by a human, null every golden entry missed and merged.
type goldenScorer struct {
	golden        *goldenDoc
	id            string
	nullMinutes   float64
	oracleMinutes float64
	path          string
	scope         string
	scoring       *evals.Scoring
}

func newGoldenScorer(path string, scoring *evals.Scoring) (*goldenScorer, error) {
	var golden goldenDoc
	if err := readJsonFile(path, &golden); err != nil {
		return nil, err
	}

	scorer := &goldenScorer{golden: &golden, path: path, scoring: scoring}
	for _, finding := range golden.Findings {
		minutes, multiplier, err := scoring.SeverityCost(finding.Severity)
		if err != nil {
			return nil, fmt.Errorf("newGoldenScorer: %s: %s: %w", path, finding.GoldenId, err)
		}

		scorer.oracleMinutes += minutes
		scorer.nullMinutes += minutes * multiplier
	}
	return scorer, nil
}

// cost adds the review of a machine fix that changed correct code to a false positive's price.
func (s *goldenScorer) cost(goldenRun *evals.GoldenRun, candidates map[string]candidate) (*runCost, error) {
	findingByGolden := make(map[string]string, len(goldenRun.Matched))
	for _, match := range goldenRun.Matched {
		findingByGolden[match.GoldenId] = match.FindingId
	}
	isFixEscaped := make(map[string]bool, len(goldenRun.FixEscapes))
	for _, goldenId := range goldenRun.FixEscapes {
		isFixEscaped[goldenId] = true
	}

	total := &runCost{bySeverity: make(map[string]*evals.GoldenSeverity)}
	for _, finding := range s.golden.Findings {
		row := total.severity(finding.Severity)
		findingId, isMatched := findingByGolden[finding.GoldenId]
		tally, minutes, err := s.price(candidates, finding, findingId, isFixEscaped[finding.GoldenId], isMatched, &row.Outcomes)
		if err != nil {
			return nil, err
		}

		row.Entries++
		tally.Count++
		tally.Minutes += minutes
	}
	for _, row := range total.bySeverity {
		settleOutcomes(row)
		total.escaped += row.EscapedMinutes
		total.handled += row.FoundMinutes
	}
	for _, ruling := range goldenRun.FalsePositives {
		found, err := s.found(candidates, ruling.FindingId)
		if err != nil {
			return nil, err
		}

		claimedSeverity := strings.ToUpper(found.Severity)
		claimed, _, err := s.scoring.SeverityCost(claimedSeverity)
		if err != nil {
			return nil, fmt.Errorf("goldenScorer.cost: %s: %w", ruling.FindingId, err)
		}

		reviewedFactor := 0.0
		if found.isMachineFix() {
			reviewedFactor = s.scoring.FixFactor[claimedSeverity]
		}
		total.falsePositive += claimed + claimed*reviewedFactor
		row := total.severity(claimedSeverity)
		row.FalsePositives++
		row.FalsePositiveMinutes += claimed + claimed*reviewedFactor
	}
	return total, nil
}

// found treats a judge naming a finding the arm never reported as a contract defect.
func (s *goldenScorer) found(candidates map[string]candidate, findingId string) (candidate, error) {
	found, ok := candidates[findingId]
	if !ok {
		return candidate{}, fmt.Errorf("goldenScorer.found: %s: Judged finding %s is not a candidate", s.path, findingId)
	}
	return found, nil
}

// price puts one golden entry in its outcome: a human-side entry reported below the triage line is never looked at and escapes.
func (s *goldenScorer) price(candidates map[string]candidate, finding goldenFinding, findingId string, isFixFailed, isMatched bool, outcomes *evals.SeverityOutcomes) (*evals.OutcomeTally, float64, error) {
	minutes, multiplier, err := s.scoring.SeverityCost(finding.Severity)
	if err != nil {
		return nil, 0, fmt.Errorf("goldenScorer.price: %s: %w", finding.GoldenId, err)
	}

	switch {
	case !isMatched:
		return &outcomes.Missed, minutes * multiplier, nil
	case isFixFailed:
		return &outcomes.FixFailed, minutes * multiplier, nil
	}

	found, err := s.found(candidates, findingId)
	if err != nil {
		return nil, 0, err
	}

	reportedSeverity := strings.ToUpper(found.Severity)
	reported, _, err := s.scoring.SeverityCost(reportedSeverity)
	if err != nil {
		return nil, 0, fmt.Errorf("goldenScorer.price: %s: %w", found.FindingId, err)
	}

	factor := s.scoring.FixFactor[finding.Severity]
	isDismissed := s.scoring.IsHumanSide(finding.Severity) && !s.scoring.IsHumanSide(reportedSeverity)
	switch {
	case found.FixState == reviewscope.FixStateFixed:
		return &outcomes.Fixed, minutes * factor, nil
	case found.FixState == reviewscope.FixStateApplied:
		return &outcomes.FixUnverified, minutes*factor + minutes, nil
	case isDismissed:
		return &outcomes.Dismissed, minutes * multiplier, nil
	case reported != minutes:
		return &outcomes.WrongSeverity, minutes + math.Abs(reported-minutes), nil
	}
	return &outcomes.AtPrice, minutes, nil
}

// metricValues gives an invalid run nulls, never zeros.
func (s *goldenScorer) metricValues(goldenRun *evals.GoldenRun, candidates map[string]candidate, isValid bool) ([]evals.MetricValue, error) {
	runId := goldenRun.RunId
	if !isValid {
		invalid := []evals.MetricValue{
			nullValue(metricRecall, runId, noteInvalidRun),
			nullValue(metricReviewScore, runId, noteInvalidRun),
			nullValue(metricLostMinutes, runId, noteInvalidRun),
			nullValue(metricEscapedMinutes, runId, noteInvalidRun),
			nullValue(metricFalsePositiveMinutes, runId, noteInvalidRun),
			nullValue(metricFalsePositives, runId, noteInvalidRun),
			nullValue(metricMisclassified, runId, noteInvalidRun),
		}
		return invalid, nil
	}

	cost, err := s.cost(goldenRun, candidates)
	if err != nil {
		return nil, err
	}

	goldenRun.Severities = cost.severities(s.scoring)
	lostMinutes := cost.escaped + cost.falsePositive + cost.handled - s.oracleMinutes
	reviewScore := nullValue(metricReviewScore, runId, noteZeroDenominator)
	savableMinutes := s.nullMinutes - s.oracleMinutes
	if savableMinutes > 0 {
		reviewScore = numberValue(metricReviewScore, runId, 1-lostMinutes/savableMinutes)
	}

	values := []evals.MetricValue{
		ratioValue(metricRecall, runId, float64(len(goldenRun.Matched)), float64(len(s.golden.Findings))),
		reviewScore,
		numberValue(metricLostMinutes, runId, lostMinutes),
		numberValue(metricEscapedMinutes, runId, cost.escaped),
		numberValue(metricFalsePositiveMinutes, runId, cost.falsePositive),
		numberValue(metricFalsePositives, runId, float64(len(goldenRun.FalsePositives))),
		numberValue(metricMisclassified, runId, float64(len(goldenRun.Misclassified))),
	}
	return values, nil
}

// run counts only the first match per golden id.
func (s *goldenScorer) run(runId string, verdict *judgeVerdict) *evals.GoldenRun {
	isMatchedGolden := make(map[string]bool)
	isMatchedFinding := make(map[string]bool)
	isMatchedPair := make(map[string]bool)
	matched := []evals.GoldenMatch{}
	for _, match := range verdict.Matched {
		if isMatchedGolden[match.GoldenId] {
			continue
		}

		isMatchedGolden[match.GoldenId] = true
		isMatchedFinding[match.FindingId] = true
		isMatchedPair[match.GoldenId+pairSeparator+match.FindingId] = true
		matched = append(matched, match)
	}

	missed := []string{}
	for _, finding := range s.golden.Findings {
		if !isMatchedGolden[finding.GoldenId] {
			missed = append(missed, finding.GoldenId)
		}
	}

	goldenRun := &evals.GoldenRun{
		FalsePositives: normalizedRulings(isMatchedFinding, verdict.FalsePositives),
		FixEscapes:     matchedFixEscapes(isMatchedGolden, verdict.FixEscapes),
		Matched:        matched,
		Misclassified:  matchedMisclassified(isMatchedPair, verdict.Misclassified),
		Missed:         missed,
		RunId:          runId,
		Uncataloged:    normalizedRulings(isMatchedFinding, verdict.Uncataloged),
	}
	return goldenRun
}

// goldenDoc is the slice of a per-scope golden file scoring reads.
type goldenDoc struct {
	Findings      []goldenFinding `json:"findings"`
	GoldenVersion int             `json:"goldenVersion"`
}

type goldenFinding struct {
	File     string `json:"file"`
	GoldenId string `json:"golden_id"`
	Severity string `json:"severity"`
}

// judgeRuling is a false-positive or uncataloged row of judge.json.
type judgeRuling struct {
	FindingId string `json:"findingId"`
	Note      string `json:"note"`
	Reason    string `json:"reason"`
}

// judgeVerdict is one golden judge's judge.json.
type judgeVerdict struct {
	FalsePositives []judgeRuling               `json:"falsePositives"`
	FixEscapes     []string                    `json:"fixEscapes"`
	Matched        []evals.GoldenMatch         `json:"matched"`
	Misclassified  []evals.GoldenMisclassified `json:"misclassified"`
	Uncataloged    []judgeRuling               `json:"uncataloged"`
}

// runCost sums the three priced parts and keeps the same minutes per severity for the record.
type runCost struct {
	bySeverity    map[string]*evals.GoldenSeverity
	escaped       float64
	falsePositive float64
	handled       float64
}

// severities lists the rows priciest severity first, by the scoring's minutes, then by name.
func (c *runCost) severities(scoring *evals.Scoring) []evals.GoldenSeverity {
	rows := make([]evals.GoldenSeverity, 0, len(c.bySeverity))
	for _, row := range c.bySeverity {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if scoring.Minutes[rows[i].Severity] != scoring.Minutes[rows[j].Severity] {
			return scoring.Minutes[rows[i].Severity] > scoring.Minutes[rows[j].Severity]
		}
		return rows[i].Severity < rows[j].Severity
	})
	return rows
}

func (c *runCost) severity(name string) *evals.GoldenSeverity {
	row, ok := c.bySeverity[name]
	if !ok {
		row = &evals.GoldenSeverity{Severity: name}
		c.bySeverity[name] = row
	}
	return row
}

// settleOutcomes derives the row's found, missed and minute totals from its outcome tallies, the one source.
func settleOutcomes(row *evals.GoldenSeverity) {
	outcomes := &row.Outcomes
	escaped := outcomes.Escaped()
	row.Missed = outcomes.Missed.Count
	row.Found = row.Entries - row.Missed
	row.EscapedMinutes = escaped.Minutes
	row.FoundMinutes = outcomes.AtPrice.Minutes + outcomes.FixUnverified.Minutes + outcomes.Fixed.Minutes + outcomes.WrongSeverity.Minutes
}

func matchedFixEscapes(isMatchedGolden map[string]bool, goldenIds []string) []string {
	kept := []string{}
	isSeen := make(map[string]bool)
	for _, goldenId := range goldenIds {
		isKept := isMatchedGolden[goldenId] && !isSeen[goldenId]
		if !isKept {
			continue
		}

		isSeen[goldenId] = true
		kept = append(kept, goldenId)
	}
	return kept
}

func matchedMisclassified(isMatchedPair map[string]bool, rows []evals.GoldenMisclassified) []evals.GoldenMisclassified {
	kept := []evals.GoldenMisclassified{}
	isSeen := make(map[string]bool)
	for _, row := range rows {
		isKept := isMatchedPair[row.GoldenId+pairSeparator+row.FindingId] && !isSeen[row.GoldenId]
		if !isKept {
			continue
		}

		isSeen[row.GoldenId] = true
		kept = append(kept, row)
	}
	return kept
}

func normalizedRulings(isMatchedFinding map[string]bool, rulings []judgeRuling) []evals.GoldenFalsePositive {
	kept := []evals.GoldenFalsePositive{}
	isSeen := make(map[string]bool)
	for _, ruling := range rulings {
		isKept := !isMatchedFinding[ruling.FindingId] && !isSeen[ruling.FindingId]
		if !isKept {
			continue
		}

		isSeen[ruling.FindingId] = true
		reason := ruling.Reason
		if reason == "" {
			reason = ruling.Note
		}
		row := evals.GoldenFalsePositive{FindingId: ruling.FindingId, Reason: reason}
		kept = append(kept, row)
	}
	return kept
}

func ratioValue(metricId, runId string, numerator, denominator float64) evals.MetricValue {
	if denominator == 0 {
		return nullValue(metricId, runId, noteZeroDenominator)
	}
	return numberValue(metricId, runId, numerator/denominator)
}
