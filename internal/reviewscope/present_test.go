package reviewscope

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresent(t *testing.T) {
	type testCase struct {
		_id                string
		_contains          []string
		_excludes          []string
		_expectedErrorText string
		_shouldPass        bool

		heading     string
		ignoreLevel string
		reviewPath  string
	}

	tests := make([]*testCase, 0)

	// gate
	tests = append(tests, &testCase{
		_id: "gate",
		_contains: []string{
			"\nEscalations:\n\n- f1: touches persisted data\n",
			"| ID | Severity | State | Route | Location | Finding | Proposed fix |",
			"| f1 | BLOCKER | new | auto-fix — mechanical rename | [a.go:7](a.go:7) | nil map \\| write | guard it |",
			"\nRefused this round:\n\n| ID | Disposition | Reason |\n| --- | --- | --- |\n| f3 | debunked | guarded upstream |\n| f4 | rejected-with-reason | intended \\| documented |\n",
			"\nUnverified:\n\n| ID | Missing precondition |\n| --- | --- |\n| f6 | needs a live database |\n",
			"\nProbes:\n\n| Probe | Finding | Verdict |\n| --- | --- | --- |\n| p1 | f1 | reproduced |\n",
			"| tests green | PASS |  |",
			"\nBase health: build ok, tests ok\nHead health: build ok, tests ok\nUncovered: b.go, c.go\nFunnel: 5 produced, 1 confirmed, 0 unverified, 2 duplicates, 0 rejected, 1 debunked\n",
			"Recommendation: CONDITIONAL",
		},
		_excludes:          []string{"## Review", "tail", "| f5 |", "Ignored"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "",
		reviewPath:  writePresentReview(t, providePresentDocument()),
	})

	// section-blocks
	tests = append(tests, &testCase{
		_id: "section-blocks",
		_contains: []string{
			"## Review — 2026-09-29 — main...abc — correctness, round 1\n\n### Escalations\n\n- f1: touches persisted data\n",
			"| f1 | BLOCKER | new | auto-fix — mechanical rename | [a.go:7](a.go:7) |",
			"\n### Refused this round\n\n",
			"\n### Unverified\n\n",
			"\n### Probes\n\n",
			"\n### Definition of Done\n\n",
			"\nBase health: build ok, tests ok\nHead health: build ok, tests ok\nUncovered: b.go, c.go\n\n### Recommendation\n\nCONDITIONAL\n",
		},
		_excludes:          []string{"Funnel", "Escalations:"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "2026-09-29 — main...abc — correctness, round 1",
		ignoreLevel: "",
		reviewPath:  writePresentReview(t, providePresentDocument()),
	})

	// ignored-line
	ignoredLine := providePresentDocument()
	nitEntry := rankedEntry{Claim: "odd name", File: "b.go", FindingId: "f2", Fix: "rename it", Line: 3, Route: "auto-fix", Severity: "NIT"}
	ignoredLine.RankedPlan = append(ignoredLine.RankedPlan, nitEntry)
	tests = append(tests, &testCase{
		_id:                "ignored-line",
		_contains:          []string{"| f1 | BLOCKER | new |", "\nIgnored (at or below nit): f2\n"},
		_excludes:          []string{"| f2 |"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "nit",
		reviewPath:  writePresentReview(t, ignoredLine),
	})

	// carried-state
	carriedState := providePresentDocument()
	carriedState.RankedPlan = append(carriedState.RankedPlan, provideCarriedEntry())
	carriedStatePath := writePresentReview(t, carriedState)
	carriedLedger := []KnownEntry{{FindingId: "f9", FixState: FixStateApplied, Round: 1, Severity: "MAJOR"}}
	require.NoError(t, writeJson(filepath.Join(filepath.Dir(filepath.Dir(carriedStatePath)), knownFile), carriedLedger))
	tests = append(tests, &testCase{
		_id:                "carried-state",
		_contains:          []string{"| f7 | MAJOR | carried: applied | human — needs a product call | [c.go:2](c.go:2) |"},
		_excludes:          []string{},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "",
		reviewPath:  carriedStatePath,
	})

	// carried-without-ledger
	carriedWithoutLedger := providePresentDocument()
	carriedWithoutLedger.RankedPlan = append(carriedWithoutLedger.RankedPlan, provideCarriedEntry())
	tests = append(tests, &testCase{
		_id:                "carried-without-ledger",
		_contains:          []string{"| f7 | MAJOR | carried | human — needs a product call |"},
		_excludes:          []string{"carried:"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "",
		reviewPath:  writePresentReview(t, carriedWithoutLedger),
	})

	// unknown-level
	tests = append(tests, &testCase{
		_id:                "unknown-level",
		_contains:          []string{},
		_excludes:          []string{},
		_expectedErrorText: "Present: Unknown ignore level loud",
		_shouldPass:        false,

		heading:     "",
		ignoreLevel: "loud",
		reviewPath:  writePresentReview(t, providePresentDocument()),
	})

	// red-base
	redBase := providePresentDocument()
	redBase.BaseHealth = healthRow{Build: "ok", Tests: "failed: --- FAIL: TestX"}
	redBase.BuildTest = healthRow{Build: "failed: undefined: x", Tests: "ok"}
	tests = append(tests, &testCase{
		_id:                "red-base",
		_contains:          []string{"Base health: build ok, tests failed\nHead health: build failed, tests ok\nUncovered: b.go, c.go\nFunnel:"},
		_excludes:          []string{"TestX", "undefined"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "",
		reviewPath:  writePresentReview(t, redBase),
	})

	// skipped
	skipped := providePresentDocument()
	skipped.BaseHealth = healthRow{Build: "skipped: waived by the invocation", Tests: "skipped: waived by the invocation"}
	tests = append(tests, &testCase{
		_id:                "skipped",
		_contains:          []string{"Base health: build skipped, tests skipped\nHead health: build ok, tests ok\nUncovered: b.go, c.go\nFunnel:"},
		_excludes:          []string{"waived"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "",
		reviewPath:  writePresentReview(t, skipped),
	})

	// missing-base-health
	missingPath := filepath.Join(t.TempDir(), reviewFile)
	missingJson := `{"build_test": {"build": "ok", "tests": "ok"}, "definition_of_done": [], "escalations": [], "funnel": {}, "ranked_plan": [], "recommendation": "APPROVE", "dispositions": [], "unverified": [], "probes": [], "coverage_gaps": []}`
	require.NoError(t, os.WriteFile(missingPath, []byte(missingJson), 0o644))
	tests = append(tests, &testCase{
		_id:                "missing-base-health",
		_contains:          []string{"No confirmed findings.\n", "\nRefused this round:\n\nnone\n", "Base health: build unknown, tests unknown\nHead health: build ok, tests ok\nFunnel:"},
		_excludes:          []string{"Unverified:", "Probes:", "Uncovered:", "Escalations:"},
		_expectedErrorText: "",
		_shouldPass:        true,

		heading:     "",
		ignoreLevel: "",
		reviewPath:  missingPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			text, err := Present(PresentRequest{Heading: test.heading, IgnoreLevel: test.ignoreLevel, ReviewPath: test.reviewPath})

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._expectedErrorText)
			for _, want := range test._contains {
				assert.Contains(t, text, want)
			}
			for _, unwanted := range test._excludes {
				assert.NotContains(t, text, unwanted)
			}
		})
	}
}

// provideCarriedEntry is a ranked entry the ledger knew before this round, ledgered as f9.
func provideCarriedEntry() rankedEntry {
	return rankedEntry{
		Claim:       "unbounded retry",
		File:        "c.go",
		FindingId:   "f7",
		Fix:         "cap the retries",
		Known:       true,
		KnownId:     "f9",
		Line:        2,
		Route:       "human",
		RouteReason: "needs a product call",
		Severity:    "MAJOR",
	}
}

func providePresentDocument() *presentDocument {
	return &presentDocument{
		BaseHealth:       healthRow{Build: "ok", Tests: "ok"},
		BuildTest:        healthRow{Build: "ok", Tests: "ok"},
		CoverageGaps:     []string{"b.go", "c.go"},
		DefinitionOfDone: []doneRow{{Criterion: "tests green", Status: "PASS"}},
		Dispositions: []dispositionRow{
			{Disposition: "debunked", FindingId: "f3", Reason: "guarded upstream"},
			{Disposition: "rejected-with-reason", FindingId: "f4", Reason: "intended | documented"},
			{Disposition: "confirmed", FindingId: "f5", Reason: "reproduced"},
		},
		Escalations: []escalationRow{{FindingId: "f1", Why: "touches persisted data"}},
		Funnel:      funnelRow{Confirmed: 1, Debunked: 1, Duplicates: 2, Produced: 5},
		Probes:      []probeRow{{FindingId: "f1", Id: "p1", Verdict: "reproduced"}},
		RankedPlan: []rankedEntry{{
			Claim:       "nil map | write",
			File:        "a.go",
			FindingId:   "f1",
			Fix:         "guard it",
			Line:        7,
			Route:       "auto-fix",
			RouteReason: "mechanical rename",
			Severity:    "BLOCKER",
		}},
		Recommendation: "CONDITIONAL",
		Unverified:     []reasonRow{{FindingId: "f6", Reason: "needs a live database"}},
	}
}

// writePresentReview writes document as <tmp>/round-1/review.json and returns its path.
func writePresentReview(t *testing.T, document *presentDocument) string {
	t.Helper()
	roundDir := filepath.Join(t.TempDir(), "round-1")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	reviewPath := filepath.Join(roundDir, reviewFile)
	require.NoError(t, writeJson(reviewPath, document))
	return reviewPath
}
