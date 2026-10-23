package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/skilleval"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/stretchr/testify/assert"
)

func TestNewSmokeBadge(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *smokeBadge

		currentVersion string
		record         *skilleval.ArmResult
	}

	endedAt := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	expectedCoverage := skilleval.SmokeCoverage{
		AnchorsDeclared:    4,
		AnchorsExpected:    []string{"base", "lanes", "round", "cleanup"},
		AnchorsExpectedHit: 4,
		AnchorsHit:         4,
		ChecksDeclared:     4,
		ChecksPassed:       2,
	}
	tests := make([]*testCase, 0)

	// nil-record
	tests = append(tests, &testCase{
		_id:       "nil-record",
		_expected: nil,

		currentVersion: "1.2",
		record:         nil,
	})

	// eval-record-without-smoke
	tests = append(tests, &testCase{
		_id:       "eval-record-without-smoke",
		_expected: nil,

		currentVersion: "1.2",
		record:         &skilleval.ArmResult{EndedAt: endedAt, Layer: "eval", SkillVersion: "1.2"},
	})

	// current-version
	tests = append(tests, &testCase{
		_id: "current-version",
		_expected: &smokeBadge{
			Cause:    "",
			Coverage: "anchors 4/4 (4), checks 2/4",
			Current:  "1.2",
			Date:     "2026-10-02",
			IsPass:   true,
			IsStale:  false,
			Verdict:  statusPass,
			Version:  "1.2",
		},

		currentVersion: "1.2",
		record: &skilleval.ArmResult{
			EndedAt:      endedAt,
			SkillVersion: "1.2",
			Smoke:        &skilleval.SmokeResult{Coverage: expectedCoverage, Verdict: statusPass},
		},
	})

	// stale-version
	tests = append(tests, &testCase{
		_id: "stale-version",
		_expected: &smokeBadge{
			Cause:    "",
			Coverage: "anchors 4/4 (4), checks 2/4",
			Current:  "1.2",
			Date:     "2026-10-02",
			IsPass:   true,
			IsStale:  true,
			Verdict:  statusPass,
			Version:  "1.1",
		},

		currentVersion: "1.2",
		record: &skilleval.ArmResult{
			EndedAt:      endedAt,
			SkillVersion: "1.1",
			Smoke:        &skilleval.SmokeResult{Coverage: expectedCoverage, Verdict: statusPass},
		},
	})

	// record-without-expectation
	legacyCoverage := skilleval.SmokeCoverage{AnchorsDeclared: 4, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 2}
	tests = append(tests, &testCase{
		_id: "record-without-expectation",
		_expected: &smokeBadge{
			Cause:    "",
			Coverage: "anchors 4 (4), checks 2/4",
			Current:  "1.2",
			Date:     "2026-10-02",
			IsPass:   true,
			IsStale:  false,
			Verdict:  statusPass,
			Version:  "1.2",
		},

		currentVersion: "1.2",
		record: &skilleval.ArmResult{
			EndedAt:      endedAt,
			SkillVersion: "1.2",
			Smoke:        &skilleval.SmokeResult{Coverage: legacyCoverage, Verdict: statusPass},
		},
	})

	// terminal-fail-with-cause
	tests = append(tests, &testCase{
		_id: "terminal-fail-with-cause",
		_expected: &smokeBadge{
			Cause:    "check exited 1",
			Coverage: "",
			Current:  "1.2",
			Date:     "2026-10-02",
			IsPass:   false,
			IsStale:  false,
			Verdict:  statusFail,
			Version:  "1.2",
		},

		currentVersion: "1.2",
		record: &skilleval.ArmResult{
			EndedAt:      endedAt,
			SkillVersion: "1.2",
			Smoke:        &skilleval.SmokeResult{Cause: "check exited 1", Verdict: statusFail},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			badge := newSmokeBadge(test.currentVersion, test.record)
			assert.Equal(t, test._expected, badge)
		})
	}
}

func TestSmokeCard(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *smokeCardView

		server *Server
		skill  *skills.Skill
	}

	smokeDir := t.TempDir()
	server := newTestServer(t, &Options{SkillsHome: t.TempDir(), SkillsRepo: t.TempDir(), SmokeDir: smokeDir})
	endedAt := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	coverage := skilleval.SmokeCoverage{
		AnchorsDeclared:    3,
		AnchorsExpected:    []string{"base", "round"},
		AnchorsExpectedHit: 1,
		AnchorsHit:         2,
		ChecksDeclared:     4,
		ChecksPassed:       3,
	}
	captured := &skilleval.ArmRuntime{Runs: []heartbeat.Run{{Anchors: []heartbeat.AnchorHit{{Id: "base"}, {Id: "lanes"}}}}}
	tests := make([]*testCase, 0)

	// anchors-marked
	costUsd := 1.5
	outputTokens := 1200
	marked := &skilleval.SmokeRecord{
		Manifest: &skilleval.RunManifest{TokenLabel: "label"},
		Result: &skilleval.ArmResult{
			CostUsd:      &costUsd,
			EndedAt:      endedAt,
			OutputTokens: &outputTokens,
			SessionId:    "session-1",
			SkillVersion: "1.2",
			Smoke:        &skilleval.SmokeResult{Cause: "anchor round never hit", Coverage: coverage, Verdict: statusFail},
		},
		Runtime: captured,
	}
	writeSmokeRecord(t, smokeDir, "marked", "1.2", marked)
	tests = append(tests, &testCase{
		_id: "anchors-marked",
		_expected: &smokeCardView{
			Anchors: []smokeAnchorView{{Id: "base", IsHit: true}, {Id: "round", IsHit: false}},
			Badge: &smokeBadge{
				Cause:    "anchor round never hit",
				Coverage: "anchors 1/2 (3), checks 3/4",
				Current:  "1.2",
				Date:     "2026-10-02",
				IsPass:   false,
				IsStale:  false,
				Verdict:  statusFail,
				Version:  "1.2",
			},
			Checks:  "3/4",
			Command: "make skill-smoke SKILL=marked TOKEN=label",
			Cost:    "1.50 USD",
			Dir:     filepath.Join(smokeDir, "marked", "1.2"),
			Session: "session-1",
			Tokens:  "1200",
		},

		server: server,
		skill:  &skills.Skill{Name: "marked", Version: "1.2"},
	})

	// no-runtime
	unwatched := &skilleval.SmokeRecord{
		Manifest: &skilleval.RunManifest{TokenLabel: "label"},
		Result:   &skilleval.ArmResult{EndedAt: endedAt, SkillVersion: "1.0", Smoke: &skilleval.SmokeResult{Coverage: coverage, Verdict: statusFail}},
		Runtime:  nil,
	}
	writeSmokeRecord(t, smokeDir, "unwatched", "1.0", unwatched)
	tests = append(tests, &testCase{
		_id: "no-runtime",
		_expected: &smokeCardView{
			Anchors: []smokeAnchorView{{Id: "base", IsHit: false}, {Id: "round", IsHit: false}},
			Badge: &smokeBadge{
				Cause:    "",
				Coverage: "anchors 1/2 (3), checks 3/4",
				Current:  "1.0",
				Date:     "2026-10-02",
				IsPass:   false,
				IsStale:  false,
				Verdict:  statusFail,
				Version:  "1.0",
			},
			Checks:  "3/4",
			Command: "make skill-smoke SKILL=unwatched TOKEN=label",
			Cost:    "",
			Dir:     filepath.Join(smokeDir, "unwatched", "1.0"),
			Session: "",
			Tokens:  "",
		},

		server: server,
		skill:  &skills.Skill{Name: "unwatched", Version: "1.0"},
	})

	// command-carries-token
	legacyCoverage := skilleval.SmokeCoverage{AnchorsDeclared: 3, AnchorsHit: 2, ChecksDeclared: 4, ChecksPassed: 3}
	legacy := &skilleval.SmokeRecord{
		Manifest: &skilleval.RunManifest{TokenLabel: "Max-Private"},
		Result:   &skilleval.ArmResult{EndedAt: endedAt, SkillVersion: "1.0", Smoke: &skilleval.SmokeResult{Coverage: legacyCoverage, Verdict: statusPass}},
		Runtime:  captured,
	}
	writeSmokeRecord(t, smokeDir, "legacy", "1.0", legacy)
	tests = append(tests, &testCase{
		_id: "command-carries-token",
		_expected: &smokeCardView{
			Anchors: nil,
			Badge: &smokeBadge{
				Cause:    "",
				Coverage: "anchors 2 (3), checks 3/4",
				Current:  "1.1",
				Date:     "2026-10-02",
				IsPass:   true,
				IsStale:  true,
				Verdict:  statusPass,
				Version:  "1.0",
			},
			Checks:  "3/4",
			Command: "make skill-smoke SKILL=legacy TOKEN=Max-Private",
			Cost:    "",
			Dir:     filepath.Join(smokeDir, "legacy", "1.0"),
			Session: "",
			Tokens:  "",
		},

		server: server,
		skill:  &skills.Skill{Name: "legacy", Version: "1.1"},
	})

	// unreadable-record
	unreadable := &skilleval.SmokeRecord{
		Manifest: nil,
		Result:   &skilleval.ArmResult{EndedAt: endedAt, SkillVersion: "1.0", Smoke: &skilleval.SmokeResult{Coverage: coverage, Verdict: statusPass}},
		Runtime:  nil,
	}
	writeSmokeRecord(t, smokeDir, "unreadable", "1.0", unreadable)
	tests = append(tests, &testCase{
		_id:       "unreadable-record",
		_expected: nil,

		server: server,
		skill:  &skills.Skill{Name: "unreadable", Version: "1.0"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			card := test.server.smokeCard(test.skill)
			assert.Equal(t, test._expected, card)
		})
	}
}
