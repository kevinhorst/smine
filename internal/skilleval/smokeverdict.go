package skilleval

import (
	"fmt"
	"time"

	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// openState is heartbeat's state of a run that has not ended.
const openState = "open"

// coverageFailures are the coverage minimums the run missed, anchors first.
func coverageFailures(coverage SmokeCoverage, expect *skillgraph.SmokeExpect) []string {
	var failures []string
	if coverageRatio(coverage.AnchorsHit, coverage.AnchorsDeclared) < *expect.MinAnchors {
		failures = append(failures, fmt.Sprintf("anchor coverage %d/%d below the minimum %g", coverage.AnchorsHit, coverage.AnchorsDeclared, *expect.MinAnchors))
	}
	if coverageRatio(coverage.ChecksPassed, coverage.ChecksDeclared) < *expect.MinChecks {
		failures = append(failures, fmt.Sprintf("check coverage %d/%d below the minimum %g", coverage.ChecksPassed, coverage.ChecksDeclared, *expect.MinChecks))
	}
	return failures
}

// coverageRatio is hit over declared; nothing declared counts as covered.
func coverageRatio(hit, declared int) float64 {
	if declared == 0 {
		return 1
	}
	return float64(hit) / float64(declared)
}

// hasAnchorExpectation selects the heartbeat verdict.
func hasAnchorExpectation(document *skillgraph.SmokeDocument) bool {
	if document.Expect == nil {
		return false
	}
	return len(document.Expect.Anchors) > 0
}

func judgeRun(document *skillgraph.SmokeDocument, enforce *skillgraph.Enforce, result *ArmResult, runtime *ArmRuntime) *SmokeResult {
	causes := limitFailures(document.Limits, result)
	causes = append(causes, smokeFailures(result, document.Skill, nil)...)
	if !hasAnchorExpectation(document) {
		return verdictOf(causes, SmokeCoverage{})
	}

	run, missing := skillRun(runtime, document.Skill)
	if run == nil {
		causes = append(causes, missing)
		return verdictOf(causes, smokeCoverage(enforce, document.Expect.Anchors, &heartbeat.Run{}))
	}

	rowCauses, coverage := rowRules(enforce, document.Expect, run)
	return verdictOf(append(causes, rowCauses...), coverage)
}

func judgeSession(document *skillgraph.SmokeDocument, enforce *skillgraph.Enforce, missing string, run *heartbeat.Run) *SmokeResult {
	if run == nil {
		return verdictOf([]string{missing}, smokeCoverage(enforce, document.Expect.Anchors, &heartbeat.Run{}))
	}

	var causes []string
	if run.State == openState {
		causes = append(causes, fmt.Sprintf("run %d still open — the session has not ended", run.Id))
	}
	rowCauses, coverage := rowRules(enforce, document.Expect, run)
	return verdictOf(append(causes, rowCauses...), coverage)
}

// limitFailures treats wall time at the minutes limit as the cell's timeout.
func limitFailures(limits *skillgraph.SmokeLimits, result *ArmResult) []string {
	var failures []string
	limit := time.Duration(limits.Minutes) * time.Minute
	if result.EndedAt.Sub(result.StartedAt) >= limit {
		failures = append(failures, fmt.Sprintf("ran into the %d-minute limit", limits.Minutes))
	}
	hasReachedBudget := result.CostUsd != nil && *result.CostUsd >= limits.Usd
	if hasReachedBudget {
		failures = append(failures, fmt.Sprintf("cost %.2f USD reached the %g USD limit", *result.CostUsd, limits.Usd))
	}
	return failures
}

func rowRules(enforce *skillgraph.Enforce, expect *skillgraph.SmokeExpect, run *heartbeat.Run) ([]string, SmokeCoverage) {
	causes := runFailures(expect.Anchors, run)
	coverage := smokeCoverage(enforce, expect.Anchors, run)
	return append(causes, coverageFailures(coverage, expect)...), coverage
}

func runFailures(expected []string, run *heartbeat.Run) []string {
	var failures []string
	if run.State == killedState {
		failures = append(failures, fmt.Sprintf("run %d killed: %s", run.Id, run.Reason))
	}
	for _, failure := range run.Failures {
		failures = append(failures, fmt.Sprintf("check %s failed, count %d", failure.Check, failure.Count))
	}

	isHit := make(map[string]bool, len(run.Anchors))
	for _, anchor := range run.Anchors {
		isHit[anchor.Id] = true
	}
	for _, anchor := range expected {
		if !isHit[anchor] {
			failures = append(failures, "anchor "+anchor+" never hit")
		}
	}
	return failures
}

// skillRun takes the last row: a nested Skill call or a new root prompt opens another.
func skillRun(runtime *ArmRuntime, skill string) (*heartbeat.Run, string) {
	if runtime.Note != "" {
		return nil, "no heartbeat run row — " + runtime.Note
	}

	var last *heartbeat.Run
	for index := range runtime.Runs {
		run := &runtime.Runs[index]
		if run.Skill != skill {
			continue
		}
		if last == nil || run.Id > last.Id {
			last = run
		}
	}
	if last == nil {
		return nil, "no heartbeat run row — no enforced run of " + skill + " in the session"
	}
	return last, ""
}

// smokeCoverage never counts ids the declaration lacks; the expected ids and their hits travel with the record.
func smokeCoverage(enforce *skillgraph.Enforce, expected []string, run *heartbeat.Run) SmokeCoverage {
	coverage := SmokeCoverage{AnchorsDeclared: len(enforce.Anchors), AnchorsExpected: expected, ChecksDeclared: len(enforce.Checks)}
	isHit := make(map[string]bool, len(run.Anchors))
	for _, anchor := range run.Anchors {
		_, isDeclared := enforce.Anchors[anchor.Id]
		if isDeclared {
			isHit[anchor.Id] = true
		}
	}
	coverage.AnchorsHit = len(isHit)
	for _, anchor := range expected {
		if isHit[anchor] {
			coverage.AnchorsExpectedHit++
		}
	}
	for _, pass := range run.Passes {
		_, isDeclared := enforce.Checks[pass.Check]
		isPassed := isDeclared && pass.Count > 0
		if isPassed {
			coverage.ChecksPassed++
		}
	}
	return coverage
}

// verdictOf passes without causes and fails on the first one otherwise.
func verdictOf(causes []string, coverage SmokeCoverage) *SmokeResult {
	result := &SmokeResult{Coverage: coverage, Verdict: statusPass}
	if len(causes) > 0 {
		result.Cause = causes[0]
		result.Verdict = statusFail
	}
	return result
}
