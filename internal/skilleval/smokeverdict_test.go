package skilleval

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// provideAnchorHits is a run's anchor rows hitting ids in order.
func provideAnchorHits(ids ...string) []heartbeat.AnchorHit {
	hits := make([]heartbeat.AnchorHit, 0, len(ids))
	for index, id := range ids {
		hits = append(hits, heartbeat.AnchorHit{Id: id, Seq: uint64(index + 1)})
	}
	return hits
}

// provideSmokeArmResult is a clean headless cell's result.json — session
// s-1, exit 0, completed, success — with the given check exit, cost and wall
// time.
func provideSmokeArmResult(checkExit *int, costUsd float64, wall time.Duration) *ArmResult {
	startedAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	result := &ArmResult{
		CheckExit:      checkExit,
		CostUsd:        &costUsd,
		EndedAt:        startedAt.Add(wall),
		SessionId:      "s-1",
		StartedAt:      startedAt,
		Subtype:        "success",
		TerminalReason: "completed",
	}
	return result
}

// provideSmokeDocument is heartbeat-demo's resolved nightly smoke document with
// the default limits; anchors select the heartbeat verdict (with both
// minimums 0, as the render resolves them), check the terminal one.
func provideSmokeDocument(anchors, check []string) *skillgraph.SmokeDocument {
	invoke := "pass-clean-run"
	expect := &skillgraph.SmokeExpect{Anchors: anchors, Check: check}
	if len(anchors) > 0 {
		minAnchors, minChecks := 0.0, 0.0
		expect.MinAnchors = &minAnchors
		expect.MinChecks = &minChecks
	}
	smoke := &skillgraph.Smoke{
		Effort: "low",
		Expect: expect,
		Invoke: &invoke,
		Limits: &skillgraph.SmokeLimits{Minutes: 15, Usd: 3},
		Mode:   skillgraph.SmokeModeNightly,
		Model:  "sonnet",
	}
	document := &skillgraph.SmokeDocument{Smoke: smoke, Schema: 1, Skill: "heartbeat-demo", Version: "2.1.2"}
	return document
}

// provideSmokeEnforce is heartbeat-demo's deployed declaration: four
// anchors, four checks.
func provideSmokeEnforce() *skillgraph.Enforce {
	enforce := &skillgraph.Enforce{
		Anchors: map[string]*skillgraph.Anchor{
			"closed":   {Tool: "Bash"},
			"edit":     {Tool: "Edit"},
			"produced": {Tool: "Write"},
			"stage":    {Tool: "Workflow"},
		},
		Checks: map[string]*skillgraph.EnforceCheck{
			"close_before_stop":   {Kind: skillgraph.CheckStop},
			"no_edit_after_stage": {Kind: skillgraph.CheckPhase},
			"producer_first":      {Kind: skillgraph.CheckRequires},
			"value_provenance":    {Kind: skillgraph.CheckProvenance},
		},
	}
	return enforce
}

// provideSmokeRun is heartbeat-demo's ended run 7 of session s-1: every
// anchor hit (closed twice), two checks passed, one entry with count 0.
func provideSmokeRun() heartbeat.Run {
	run := heartbeat.Run{
		Id: 7,
		Anchors: []heartbeat.AnchorHit{
			{Id: "produced", Seq: 1},
			{Id: "stage", Seq: 2},
			{Id: "edit", Seq: 3},
			{Id: "closed", Seq: 4},
			{Id: "closed", Seq: 5},
		},
		Failures: []heartbeat.CheckFailure{},
		Passes: []heartbeat.CheckPass{
			{Check: "close_before_stop", Count: 1},
			{Check: "producer_first", Count: 2},
			{Check: "value_provenance", Count: 0},
		},
		Session: "s-1",
		Skill:   "heartbeat-demo",
		State:   "ended",
		Version: "2.1.2",
	}
	return run
}

// provideWideEnforce is heartbeat-demo's four checks with anchorCount anchors anchor-01, anchor-02, ….
func provideWideEnforce(anchorCount int) *skillgraph.Enforce {
	enforce := provideSmokeEnforce()
	enforce.Anchors = make(map[string]*skillgraph.Anchor, anchorCount)
	for index := range anchorCount {
		enforce.Anchors[fmt.Sprintf("anchor-%02d", index+1)] = &skillgraph.Anchor{Tool: "Bash"}
	}
	return enforce
}

func TestCoverageRatio(t *testing.T) {
	assert.InDelta(t, 1.0, coverageRatio(0, 0), 1e-9)
	assert.InDelta(t, 0.25, coverageRatio(1, 4), 1e-9)
}

func TestJudgeRun(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *SmokeResult

		document *skillgraph.SmokeDocument
		result   *ArmResult
		runtime  *ArmRuntime
	}

	expectedAnchors := []string{"produced", "closed"}
	anchored := provideSmokeDocument(expectedAnchors, nil)
	terminal := provideSmokeDocument(nil, nil)
	failedCheckExit := 1
	fullCoverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: expectedAnchors, AnchorsExpectedHit: 2, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 2}
	noCoverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: expectedAnchors, ChecksDeclared: 4}
	partialCoverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: expectedAnchors, AnchorsExpectedHit: 1, AnchorsHit: 1, ChecksDeclared: 4, ChecksPassed: 2}
	demandingCoverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: []string{"produced"}, AnchorsExpectedHit: 1, AnchorsHit: 1, ChecksDeclared: 4, ChecksPassed: 2}
	silent := &ArmRuntime{Note: "heartbeat did not answer: connection refused"}
	tests := make([]*testCase, 0)

	// pass-terminal-without-check
	tests = append(tests, &testCase{
		_id:       "pass-terminal-without-check",
		_expected: &SmokeResult{Verdict: statusPass},

		document: terminal,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{},
	})

	// fail-check-exit
	tests = append(tests, &testCase{
		_id:       "fail-check-exit",
		_expected: &SmokeResult{Cause: "check exited 1", Verdict: statusFail},

		document: provideSmokeDocument(nil, []string{"go", "test", "./..."}),
		result:   provideSmokeArmResult(&failedCheckExit, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{},
	})

	// fail-budget-named-before-subtype
	overBudget := provideSmokeArmResult(nil, 3, 2*time.Minute)
	overBudget.Subtype = "error_max_budget_usd"
	tests = append(tests, &testCase{
		_id:       "fail-budget-named-before-subtype",
		_expected: &SmokeResult{Cause: "cost 3.00 USD reached the 3 USD limit", Verdict: statusFail},

		document: terminal,
		result:   overBudget,
		runtime:  &ArmRuntime{},
	})

	// fail-timeout-named-before-exit-status
	timedOut := provideSmokeArmResult(nil, 0.5, 15*time.Minute)
	timedOut.ExitStatus = 124
	tests = append(tests, &testCase{
		_id:       "fail-timeout-named-before-exit-status",
		_expected: &SmokeResult{Cause: "ran into the 15-minute limit", Verdict: statusFail},

		document: terminal,
		result:   timedOut,
		runtime:  &ArmRuntime{},
	})

	// pass-heartbeat-with-coverage
	tests = append(tests, &testCase{
		_id:       "pass-heartbeat-with-coverage",
		_expected: &SmokeResult{Coverage: fullCoverage, Verdict: statusPass},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{provideSmokeRun()}},
	})

	// fail-heartbeat-silent
	tests = append(tests, &testCase{
		_id:       "fail-heartbeat-silent",
		_expected: &SmokeResult{Cause: "no heartbeat run row — heartbeat did not answer: connection refused", Coverage: noCoverage, Verdict: statusFail},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  silent,
	})

	// fail-no-enforced-run
	otherSkill := provideSmokeRun()
	otherSkill.Skill = "fdesign"
	tests = append(tests, &testCase{
		_id:       "fail-no-enforced-run",
		_expected: &SmokeResult{Cause: "no heartbeat run row — no enforced run of heartbeat-demo in the session", Coverage: noCoverage, Verdict: statusFail},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{otherSkill}},
	})

	// fail-session-rule-before-row
	crashed := provideSmokeArmResult(nil, 0.5, 2*time.Minute)
	crashed.Subtype = "error_during_execution"
	tests = append(tests, &testCase{
		_id:       "fail-session-rule-before-row",
		_expected: &SmokeResult{Cause: `result subtype "error_during_execution"`, Coverage: noCoverage, Verdict: statusFail},

		document: anchored,
		result:   crashed,
		runtime:  silent,
	})

	// fail-killed-keeps-passes
	killed := provideSmokeRun()
	killed.State = "killed"
	killed.Reason = "producer_first 2"
	tests = append(tests, &testCase{
		_id:       "fail-killed-keeps-passes",
		_expected: &SmokeResult{Cause: "run 7 killed: producer_first 2", Coverage: fullCoverage, Verdict: statusFail},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{killed}},
	})

	// fail-failed-check
	failing := provideSmokeRun()
	failing.Failures = []heartbeat.CheckFailure{{Check: "no_edit_after_stage", Count: 1}}
	tests = append(tests, &testCase{
		_id:       "fail-failed-check",
		_expected: &SmokeResult{Cause: "check no_edit_after_stage failed, count 1", Coverage: fullCoverage, Verdict: statusFail},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{failing}},
	})

	// fail-anchor-never-hit
	partial := provideSmokeRun()
	partial.Anchors = []heartbeat.AnchorHit{{Id: "produced", Seq: 1}}
	tests = append(tests, &testCase{
		_id:       "fail-anchor-never-hit",
		_expected: &SmokeResult{Cause: "anchor closed never hit", Coverage: partialCoverage, Verdict: statusFail},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{partial}},
	})

	// fail-anchor-coverage-minimum
	demanding := provideSmokeDocument([]string{"produced"}, nil)
	*demanding.Expect.MinAnchors = 0.5
	tests = append(tests, &testCase{
		_id:       "fail-anchor-coverage-minimum",
		_expected: &SmokeResult{Cause: "anchor coverage 1/4 below the minimum 0.5", Coverage: demandingCoverage, Verdict: statusFail},

		document: demanding,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{partial}},
	})

	// pass-same-session-latest-row-of-skill-wins
	earlier := provideSmokeRun()
	earlier.Id = 3
	earlier.State = "killed"
	nested := provideSmokeRun()
	nested.Id = 9
	nested.Skill = "fdesign"
	nested.State = "killed"
	tests = append(tests, &testCase{
		_id:       "pass-same-session-latest-row-of-skill-wins",
		_expected: &SmokeResult{Coverage: fullCoverage, Verdict: statusPass},

		document: anchored,
		result:   provideSmokeArmResult(nil, 0.5, 2*time.Minute),
		runtime:  &ArmRuntime{Runs: []heartbeat.Run{earlier, provideSmokeRun(), nested}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, judgeRun(test.document, provideSmokeEnforce(), test.result, test.runtime))
		})
	}
}

func TestJudgeSession(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *SmokeResult

		document *skillgraph.SmokeDocument
		missing  string
		run      *heartbeat.Run
	}

	expectedAnchors := []string{"produced", "closed"}
	anchored := provideSmokeDocument(expectedAnchors, nil)
	fullCoverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: expectedAnchors, AnchorsExpectedHit: 2, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 2}
	noCoverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: expectedAnchors, ChecksDeclared: 4}
	tests := make([]*testCase, 0)

	// pass-ended
	ended := provideSmokeRun()
	tests = append(tests, &testCase{
		_id:       "pass-ended",
		_expected: &SmokeResult{Coverage: fullCoverage, Verdict: statusPass},

		document: anchored,
		missing:  "",
		run:      &ended,
	})

	// fail-still-open
	open := provideSmokeRun()
	open.State = "open"
	tests = append(tests, &testCase{
		_id:       "fail-still-open",
		_expected: &SmokeResult{Cause: "run 7 still open — the session has not ended", Coverage: fullCoverage, Verdict: statusFail},

		document: anchored,
		missing:  "",
		run:      &open,
	})

	// fail-killed
	killed := provideSmokeRun()
	killed.State = "killed"
	killed.Reason = "producer_first 2"
	tests = append(tests, &testCase{
		_id:       "fail-killed",
		_expected: &SmokeResult{Cause: "run 7 killed: producer_first 2", Coverage: fullCoverage, Verdict: statusFail},

		document: anchored,
		missing:  "",
		run:      &killed,
	})

	// fail-missing-row
	tests = append(tests, &testCase{
		_id:       "fail-missing-row",
		_expected: &SmokeResult{Cause: "no heartbeat run row — heartbeat did not answer: connection refused", Coverage: noCoverage, Verdict: statusFail},

		document: anchored,
		missing:  "no heartbeat run row — heartbeat did not answer: connection refused",
		run:      nil,
	})

	// fail-check-coverage-minimum
	demanding := provideSmokeDocument([]string{"produced", "closed"}, nil)
	*demanding.Expect.MinChecks = 0.75
	minimal := provideSmokeRun()
	tests = append(tests, &testCase{
		_id:       "fail-check-coverage-minimum",
		_expected: &SmokeResult{Cause: "check coverage 2/4 below the minimum 0.75", Coverage: fullCoverage, Verdict: statusFail},

		document: demanding,
		missing:  "",
		run:      &minimal,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, judgeSession(test.document, provideSmokeEnforce(), test.missing, test.run))
		})
	}
}

func TestSmokeCoverage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected SmokeCoverage

		enforce  *skillgraph.Enforce
		expected []string
		run      *heartbeat.Run
	}

	tests := make([]*testCase, 0)

	// all-expected-hit
	sixExpected := []string{"anchor-01", "anchor-02", "anchor-03", "anchor-04", "anchor-05", "anchor-06"}
	sixHit := provideSmokeRun()
	sixHit.Anchors = provideAnchorHits(sixExpected...)
	tests = append(tests, &testCase{
		_id:       "all-expected-hit",
		_expected: SmokeCoverage{AnchorsDeclared: 12, AnchorsExpected: sixExpected, AnchorsExpectedHit: 6, AnchorsHit: 6, ChecksDeclared: 4, ChecksPassed: 2},

		enforce:  provideWideEnforce(12),
		expected: sixExpected,
		run:      &sixHit,
	})

	// expected-partly-hit
	threeExpected := []string{"produced", "stage", "edit"}
	twoHit := provideSmokeRun()
	twoHit.Anchors = provideAnchorHits("produced", "stage")
	tests = append(tests, &testCase{
		_id:       "expected-partly-hit",
		_expected: SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: threeExpected, AnchorsExpectedHit: 2, AnchorsHit: 2, ChecksDeclared: 4, ChecksPassed: 2},

		enforce:  provideSmokeEnforce(),
		expected: threeExpected,
		run:      &twoHit,
	})

	// no-expectation
	undeclaredIds := provideSmokeRun()
	undeclaredIds.Anchors = append(undeclaredIds.Anchors, heartbeat.AnchorHit{Id: "ghost", Seq: 9})
	undeclaredIds.Passes = append(undeclaredIds.Passes, heartbeat.CheckPass{Check: "retired_check", Count: 3})
	tests = append(tests, &testCase{
		_id:       "no-expectation",
		_expected: SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: nil, AnchorsExpectedHit: 0, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 2},

		enforce:  provideSmokeEnforce(),
		expected: nil,
		run:      &undeclaredIds,
	})

	// expected-id-undeclared
	ghostExpected := []string{"produced", "ghost"}
	ghostHit := provideSmokeRun()
	ghostHit.Anchors = append(ghostHit.Anchors, heartbeat.AnchorHit{Id: "ghost", Seq: 9})
	tests = append(tests, &testCase{
		_id:       "expected-id-undeclared",
		_expected: SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: ghostExpected, AnchorsExpectedHit: 1, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 2},

		enforce:  provideSmokeEnforce(),
		expected: ghostExpected,
		run:      &ghostHit,
	})

	// older-daemon-without-passes
	olderDaemon := provideSmokeRun()
	olderDaemon.Passes = nil
	tests = append(tests, &testCase{
		_id:       "older-daemon-without-passes",
		_expected: SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: nil, AnchorsExpectedHit: 0, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 0},

		enforce:  provideSmokeEnforce(),
		expected: nil,
		run:      &olderDaemon,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, smokeCoverage(test.enforce, test.expected, test.run))
		})
	}
}
