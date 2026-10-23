package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var usageTestLocation = time.FixedZone("UTC+2", 2*60*60)

// usageMeasuredWindow is the measured Team Premium 5-hour window's tokens. On
// Opus 5.5 they weigh 0.4 × 17 216 269 + 0.01 × 390 238 659 + 0.2 × 4 834 +
// 1 × 2 137 757 = 12 927 617.99 equivalents — the Team Premium window's whole
// capacity and 31.55% of Max 20x's 40 976 986.1 (11 473 556.11 / 0.28).
var usageMeasuredWindow = budget.TokenClasses{CacheCreation: 17216269, CacheRead: 390238659, Input: 4834, Output: 2137757}

// usageOpusWindow is one full Team Premium window on Opus 5.5: 100%.
var usageOpusWindow = budget.ModelUsage{Model: "claude-opus-5-5", Usage: usageMeasuredWindow}

// usageFableWindow is the measured window's tokens on Fable 5.1: 1 × 17 216 269
// + 0.0125 × 390 238 659 + 0.5 × 4 834 + 2.5 × 2 137 757 = 27 441 061.7375
// equivalents, 212.27% of the Team Premium window.
var usageFableWindow = budget.ModelUsage{Model: "claude-fable-5-1", Usage: usageMeasuredWindow}

// usageFableTenth is 2.5 × 517 105 = 1 292 762.5 equivalents, 10.00% of the
// Team Premium window.
var usageFableTenth = budget.ModelUsage{Model: "claude-fable-5-1", Usage: budget.TokenClasses{Output: 517105}}

// provideUsageRecord is a record whose two sides agree on every metric;
// opts change it per case.
func provideUsageRecord(id string, lastActive time.Time, opts ...func(*budget.Record)) *budget.Record {
	tokens := budget.TokenClasses{CacheCreation: 100, CacheRead: 2000, Input: 30, Output: 400}
	record := &budget.Record{
		Id:    id,
		Agent: contextrecord.SessionClaude,
		Ccusage: &budget.CcusageSide{
			CostUsd:       0.5,
			FirstActivity: lastActive.Add(-10 * time.Minute),
			LastActivity:  lastActive,
			Usage:         tokens,
		},
		Peek: &budget.PeekSide{
			ActiveSeconds: 300,
			CostUsd:       0.5,
			HasCost:       true,
			IdleSeconds:   300,
			LastActive:    lastActive,
			Root:          tokens,
			WallSeconds:   600,
		},
		Repo:   "smine",
		Schema: budget.RecordSchema,
		Title:  "Session " + id,
	}
	for _, opt := range opts {
		opt(record)
	}
	return record
}

// pinRoutinesHost runs the server as a darwin host: the Usage page needs Routines, which Linux cannot run.
func pinRoutinesHost(server *Server) {
	server.machine.GOOS = "darwin"
}

func newUsageTestServer(t *testing.T, records []*budget.Record) *Server {
	t.Helper()
	stateDir := t.TempDir()
	server := newTestServer(t, &Options{StateDir: stateDir})
	pinRoutinesHost(server)
	store := budget.NewStore(stateDir)
	for _, record := range records {
		require.NoError(t, store.SaveRecord(record))
	}
	return server
}

func sessionIds(view *usageView) []string {
	ids := make([]string, 0, len(view.sessions))
	for _, session := range view.sessions {
		ids = append(ids, session.record.Id)
	}
	return ids
}

// containedStrings lists the needles body contains.
func containedStrings(body string, needles []string) []string {
	var found []string
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			found = append(found, needle)
		}
	}
	return found
}

// emptyChartDays is one chart column per day of period without any value,
// every labelStep-th column labelled; zero is the formatted zero of the unit.
func emptyChartDays(labelStep int, period budget.Period, zero string) []usageDay {
	days := make([]usageDay, 0)
	for index, day := range period.Days() {
		column := usageDay{Height: "0.0", Title: day.Format(time.DateOnly) + " — " + zero}
		if index%labelStep == 0 {
			column.Label = day.Format("01-02")
		}
		days = append(days, column)
	}
	return days
}

// newQueriedUsageView loads records into a view of query over its preset
// days ending with now's day.
func newQueriedUsageView(now time.Time, query usageQuery, records []*budget.Record) *usageView {
	view := &usageView{
		location: usageTestLocation,
		period:   budget.NewPresetPeriod(query.days, now),
		plan:     budget.DefaultPlan,
		query:    query,
	}
	view.load(records)
	return view
}

func withRecordCodex() func(*budget.Record) {
	return func(record *budget.Record) {
		record.Agent = contextrecord.SessionCodex
	}
}

func withRecordCost(ccusage float64, peek float64) func(*budget.Record) {
	return func(record *budget.Record) {
		record.Ccusage.CostUsd = ccusage
		record.Peek.CostUsd = peek
	}
}

// withRecordModels gives the ccusage side a per-model split; Usage stays the
// fixture's, so only plan percent follows the split.
func withRecordModels(models ...budget.ModelUsage) func(*budget.Record) {
	return func(record *budget.Record) {
		record.Ccusage.Models = models
	}
}

// withRecordSkillRun adds one run of skill holding a tenth of the fixture's root tokens.
func withRecordSkillRun(skill string, seconds int) func(*budget.Record) {
	return func(record *budget.Record) {
		usage := budget.TokenClasses{CacheCreation: 10, CacheRead: 200, Input: 3, Output: 40}
		run := budget.SkillRun{Seconds: seconds, Skill: skill, Usage: usage}
		record.Peek.SkillRuns = append(record.Peek.SkillRuns, run)
	}
}

func withRecordRepo(repo string) func(*budget.Record) {
	return func(record *budget.Record) {
		record.Repo = repo
	}
}

func TestNewUsageSource(t *testing.T) {
	type testCase struct {
		_id       string
		_expected usageSource

		value string
	}

	tests := make([]*testCase, 0)

	// empty-is-peek
	tests = append(tests, &testCase{
		_id:       "empty-is-peek",
		_expected: usageSourcePeek,

		value: "",
	})

	// ccusage-kept
	tests = append(tests, &testCase{
		_id:       "ccusage-kept",
		_expected: usageSourceCcusage,

		value: "ccusage",
	})

	// peek-kept
	tests = append(tests, &testCase{
		_id:       "peek-kept",
		_expected: usageSourcePeek,

		value: "peek",
	})

	// both-kept
	tests = append(tests, &testCase{
		_id:       "both-kept",
		_expected: usageSourceBoth,

		value: "both",
	})

	// unknown-is-peek
	tests = append(tests, &testCase{
		_id:       "unknown-is-peek",
		_expected: usageSourcePeek,

		value: "codex",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, newUsageSource(test.value))
		})
	}
}

func TestNewUsageUnit(t *testing.T) {
	type testCase struct {
		_id       string
		_expected usageUnit

		value string
	}

	tests := make([]*testCase, 0)

	// empty-is-usd
	tests = append(tests, &testCase{
		_id:       "empty-is-usd",
		_expected: usageUnitUsd,

		value: "",
	})

	// plan-kept
	tests = append(tests, &testCase{
		_id:       "plan-kept",
		_expected: usageUnitPlan,

		value: "plan",
	})

	// unknown-is-usd
	tests = append(tests, &testCase{
		_id:       "unknown-is-usd",
		_expected: usageUnitUsd,

		value: "eur",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, newUsageUnit(test.value))
		})
	}
}

func TestUsageQuery_Period(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedFrom time.Time
		_expectedTo   time.Time
		_shouldPass   bool

		query usageQuery
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	tests := make([]*testCase, 0)

	// default
	tests = append(tests, &testCase{
		_id:           "default",
		_expectedFrom: time.Date(2026, 9, 27, 0, 0, 0, 0, usageTestLocation),
		_expectedTo:   time.Date(2026, 10, 4, 0, 0, 0, 0, usageTestLocation),
		_shouldPass:   true,

		query: usageQuery{days: budget.DefaultDays},
	})

	// preset
	tests = append(tests, &testCase{
		_id:           "preset",
		_expectedFrom: time.Date(2026, 10, 3, 0, 0, 0, 0, usageTestLocation),
		_expectedTo:   time.Date(2026, 10, 4, 0, 0, 0, 0, usageTestLocation),
		_shouldPass:   true,

		query: usageQuery{days: 1},
	})

	// range
	tests = append(tests, &testCase{
		_id:           "range",
		_expectedFrom: time.Date(2026, 9, 1, 0, 0, 0, 0, usageTestLocation),
		_expectedTo:   time.Date(2026, 9, 11, 0, 0, 0, 0, usageTestLocation),
		_shouldPass:   true,

		query: usageQuery{days: budget.DefaultDays, from: "2026-09-01", to: "2026-09-10"},
	})

	// half-range-error
	tests = append(tests, &testCase{
		_id:           "half-range-error",
		_expectedFrom: time.Time{},
		_expectedTo:   time.Time{},
		_shouldPass:   false,

		query: usageQuery{days: budget.DefaultDays, from: "2026-09-01"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			period, err := test.query.period(now)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.True(t, test._expectedFrom.Equal(period.From), "from = %v", period.From)
			assert.True(t, test._expectedTo.Equal(period.To), "to = %v", period.To)
		})
	}
}

func TestUsageQuery_Url(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		path  string
		query usageQuery
	}

	tests := make([]*testCase, 0)

	// default
	tests = append(tests, &testCase{
		_id:       "default",
		_expected: "/usage",

		path:  "/usage",
		query: usageQuery{days: budget.DefaultDays},
	})

	// preset
	tests = append(tests, &testCase{
		_id:       "preset",
		_expected: "/usage?days=30",

		path:  "/usage",
		query: usageQuery{days: 30},
	})

	// range
	tests = append(tests, &testCase{
		_id:       "range",
		_expected: "/usage?from=2026-09-01&to=2026-09-10",

		path:  "/usage",
		query: usageQuery{days: 30, from: "2026-09-01", to: "2026-09-10"},
	})

	// filters-encoded
	tests = append(tests, &testCase{
		_id:       "filters-encoded",
		_expected: "/usage.csv?agent=codex&only=differ&repo=a+b%26c&skill=fdesign%2Fx&source=both",

		path: "/usage.csv",
		query: usageQuery{
			agent:  "codex",
			days:   budget.DefaultDays,
			only:   usageOnlyDiffer,
			repo:   "a b&c",
			skill:  "fdesign/x",
			source: usageSourceBoth,
		},
	})

	// default-source-and-unit-omitted
	tests = append(tests, &testCase{
		_id:       "default-source-and-unit-omitted",
		_expected: "/usage",

		path:  "/usage",
		query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitUsd},
	})

	// source-ccusage-written
	tests = append(tests, &testCase{
		_id:       "source-ccusage-written",
		_expected: "/usage?source=ccusage",

		path:  "/usage",
		query: usageQuery{days: budget.DefaultDays, source: usageSourceCcusage, unit: usageUnitUsd},
	})

	// source-both-with-only-written
	tests = append(tests, &testCase{
		_id:       "source-both-with-only-written",
		_expected: "/usage?only=differ&source=both",

		path:  "/usage",
		query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, source: usageSourceBoth, unit: usageUnitUsd},
	})

	// unit-plan-written
	tests = append(tests, &testCase{
		_id:       "unit-plan-written",
		_expected: "/usage?unit=plan",

		path:  "/usage",
		query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitPlan},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.query.url(test.path))
		})
	}
}

func TestUsageQuery_ForPage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected usageQuery

		query usageQuery
	}

	tests := make([]*testCase, 0)

	// only-dropped-under-ccusage
	tests = append(tests, &testCase{
		_id:       "only-dropped-under-ccusage",
		_expected: usageQuery{days: budget.DefaultDays, repo: "smine", source: usageSourceCcusage, unit: usageUnitUsd},

		query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, repo: "smine", source: usageSourceCcusage, unit: usageUnitUsd},
	})

	// only-dropped-under-peek
	tests = append(tests, &testCase{
		_id:       "only-dropped-under-peek",
		_expected: usageQuery{days: budget.DefaultDays, repo: "smine", source: usageSourcePeek, unit: usageUnitUsd},

		query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, repo: "smine", source: usageSourcePeek, unit: usageUnitUsd},
	})

	// only-kept-under-both
	tests = append(tests, &testCase{
		_id:       "only-kept-under-both",
		_expected: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, repo: "smine", source: usageSourceBoth, unit: usageUnitUsd},

		query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, repo: "smine", source: usageSourceBoth, unit: usageUnitUsd},
	})

	// skill-pins-peek-drops-only
	tests = append(tests, &testCase{
		_id:       "skill-pins-peek-drops-only",
		_expected: usageQuery{days: budget.DefaultDays, skill: "fdesign", source: usageSourcePeek, unit: usageUnitUsd},

		query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, skill: "fdesign", source: usageSourceBoth, unit: usageUnitUsd},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.query.forPage())
		})
	}
}

func TestUsageQuery_ForCsv(t *testing.T) {
	query := usageQuery{
		agent:  "codex",
		days:   30,
		from:   "2026-09-01",
		only:   usageOnlyDiffer,
		repo:   "smine",
		skill:  "fdesign",
		source: usageSourcePeek,
		to:     "2026-09-10",
		unit:   usageUnitPlan,
	}
	expected := usageQuery{
		agent:  "codex",
		days:   30,
		from:   "2026-09-01",
		only:   usageOnlyDiffer,
		repo:   "smine",
		skill:  "fdesign",
		source: usageSourceBoth,
		to:     "2026-09-10",
		unit:   usageUnitUsd,
	}

	assert.Equal(t, expected, query.forCsv())
}

func TestUsageView_Load(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		query  usageQuery
		server *Server
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	recent := now.Add(-time.Hour)
	older := now.Add(-26 * time.Hour)
	tests := make([]*testCase, 0)

	// outside-period-dropped
	tests = append(tests, &testCase{
		_id:       "outside-period-dropped",
		_expected: []string{"inside"},

		query: usageQuery{days: 1},
		server: newUsageTestServer(t, []*budget.Record{
			provideUsageRecord("inside", recent),
			provideUsageRecord("outside", older),
		}),
	})

	// agent-filter
	tests = append(tests, &testCase{
		_id:       "agent-filter",
		_expected: []string{"codex-session"},

		query: usageQuery{agent: contextrecord.SessionCodex, days: budget.DefaultDays},
		server: newUsageTestServer(t, []*budget.Record{
			provideUsageRecord("claude-session", recent),
			provideUsageRecord("codex-session", recent, func(record *budget.Record) {
				record.Agent = contextrecord.SessionCodex
			}),
		}),
	})

	// repo-filter
	tests = append(tests, &testCase{
		_id:       "repo-filter",
		_expected: []string{"no-repo"},

		query: usageQuery{days: budget.DefaultDays, repo: usageUnknownRepo},
		server: newUsageTestServer(t, []*budget.Record{
			provideUsageRecord("in-repo", recent),
			provideUsageRecord("no-repo", recent, func(record *budget.Record) {
				record.Repo = ""
			}),
		}),
	})

	// skill-filter
	tests = append(tests, &testCase{
		_id:       "skill-filter",
		_expected: []string{"with-skill"},

		query: usageQuery{days: budget.DefaultDays, skill: "fdesign"},
		server: newUsageTestServer(t, []*budget.Record{
			provideUsageRecord("without-skill", recent),
			provideUsageRecord("with-skill", recent, func(record *budget.Record) {
				record.Peek.SkillRuns = []budget.SkillRun{{Seconds: 60, Skill: "fdesign"}}
			}),
		}),
	})

	// only-differ
	tests = append(tests, &testCase{
		_id:       "only-differ",
		_expected: []string{"differs"},

		query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer},
		server: newUsageTestServer(t, []*budget.Record{
			provideUsageRecord("agrees", recent),
			provideUsageRecord("differs", recent, func(record *budget.Record) {
				record.Ccusage.CostUsd = 2
			}),
		}),
	})

	// sorted-newest-first
	tests = append(tests, &testCase{
		_id:       "sorted-newest-first",
		_expected: []string{"newest", "middle", "oldest"},

		query: usageQuery{days: budget.DefaultDays},
		server: newUsageTestServer(t, []*budget.Record{
			provideUsageRecord("middle", now.Add(-2*time.Hour)),
			provideUsageRecord("oldest", now.Add(-3*time.Hour)),
			provideUsageRecord("newest", recent),
		}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			view, err := test.server.loadUsage(now, test.query)
			require.NoError(t, err)
			assert.Equal(t, test._expected, sessionIds(view))
		})
	}
}

func TestNewSkillSums(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedActive      int
		_expectedBoth        int
		_expectedCacheRead   float64
		_expectedCost        float64
		_expectedHasPlan     bool
		_expectedOutput      float64
		_expectedPlanPercent string
		_expectedWall        float64

		record *budget.Record
		skill  string
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	teamPremium := &usageView{plan: budget.PlanTeamPremium}
	tests := make([]*testCase, 0)

	// one-run-count-share
	// no model split: the run is 253 of 2 530 tokens, a tenth of the $0.50
	tests = append(tests, &testCase{
		_id:                  "one-run-count-share",
		_expectedActive:      60,
		_expectedBoth:        0,
		_expectedCacheRead:   200,
		_expectedCost:        0.05,
		_expectedHasPlan:     false,
		_expectedOutput:      40,
		_expectedPlanPercent: "",
		_expectedWall:        60,

		record: provideUsageRecord("one", now, withRecordSkillRun("fdesign", 60)),
		skill:  "fdesign",
	})

	// two-runs-summed-other-skill-ignored
	tests = append(tests, &testCase{
		_id:                  "two-runs-summed-other-skill-ignored",
		_expectedActive:      90,
		_expectedBoth:        0,
		_expectedCacheRead:   400,
		_expectedCost:        0.1,
		_expectedHasPlan:     false,
		_expectedOutput:      80,
		_expectedPlanPercent: "",
		_expectedWall:        90,

		record: provideUsageRecord("two", now, withRecordSkillRun("fdesign", 60), withRecordSkillRun("close", 5), withRecordSkillRun("fdesign", 30)),
		skill:  "fdesign",
	})

	// opus-split-price-weighted-share
	// 4 + 2 + 0.6 + 40 = 46.6 of 40 + 20 + 6 + 400 = 466: a tenth of the window
	tests = append(tests, &testCase{
		_id:                  "opus-split-price-weighted-share",
		_expectedActive:      60,
		_expectedBoth:        0,
		_expectedCacheRead:   200,
		_expectedCost:        0.05,
		_expectedHasPlan:     true,
		_expectedOutput:      40,
		_expectedPlanPercent: "10%",
		_expectedWall:        60,

		record: provideUsageRecord("split", now, withRecordModels(usageOpusWindow), withRecordSkillRun("fdesign", 60)),
		skill:  "fdesign",
	})

	// no-run-of-the-skill-zero
	tests = append(tests, &testCase{
		_id:                  "no-run-of-the-skill-zero",
		_expectedActive:      0,
		_expectedBoth:        0,
		_expectedCacheRead:   0,
		_expectedCost:        0,
		_expectedHasPlan:     false,
		_expectedOutput:      0,
		_expectedPlanPercent: "",
		_expectedWall:        0,

		record: provideUsageRecord("none", now, withRecordSkillRun("close", 5)),
		skill:  "fdesign",
	})

	// no-peek-side-zero
	tests = append(tests, &testCase{
		_id:                  "no-peek-side-zero",
		_expectedActive:      0,
		_expectedBoth:        0,
		_expectedCacheRead:   0,
		_expectedCost:        0,
		_expectedHasPlan:     false,
		_expectedOutput:      0,
		_expectedPlanPercent: "",
		_expectedWall:        0,

		record: provideUsageRecord("ccusage-only", now, func(record *budget.Record) {
			record.Peek = nil
		}),
		skill: "fdesign",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			sums := newSkillSums(test.record, test.skill)
			assert.Equal(t, 1, sums.sessions)
			assert.Equal(t, test._expectedActive, sums.active)
			assert.Equal(t, 0, sums.idle)
			assert.Equal(t, test._expectedBoth, sums.cost.Both)
			assert.Equal(t, test._expectedBoth, sums.cacheRead.Both)
			assert.InDelta(t, test._expectedCacheRead, sums.cacheRead.Peek, 0)
			assert.InDelta(t, test._expectedOutput, sums.output.Peek, 0)
			assert.InDelta(t, test._expectedCost, sums.cost.Peek, 1e-9)
			assert.InDelta(t, test._expectedWall, sums.wall.Peek, 0)
			assert.Equal(t, test._expectedHasPlan, sums.hasPlan)
			assert.Equal(t, test._expectedPlanPercent, teamPremium.planPercent(&sums))
			assert.Equal(t, 0, sums.contextSessions)
		})
	}
}

func TestUsageView_Cell(t *testing.T) {
	type testCase struct {
		_id       string
		_expected usageCell

		pair *budget.Pair
		view *usageView
	}

	agreeing := &budget.Pair{Both: 1, Ccusage: 0.5, PairedCcusage: 0.5, PairedPeek: 0.5, Peek: 0.5}
	differing := &budget.Pair{Both: 1, Ccusage: 0.5, PairedCcusage: 0.5, PairedPeek: 2, Peek: 2}
	ccusageOnly := &budget.Pair{Ccusage: 0.5, Single: 1}
	tests := make([]*testCase, 0)

	// ccusage-one-value-no-badge
	tests = append(tests, &testCase{
		_id:       "ccusage-one-value-no-badge",
		_expected: usageCell{Values: []string{"$0.50"}},

		pair: differing,
		view: &usageView{query: usageQuery{source: usageSourceCcusage}},
	})

	// peek-one-value-no-badge
	tests = append(tests, &testCase{
		_id:       "peek-one-value-no-badge",
		_expected: usageCell{Values: []string{"$2.00"}},

		pair: differing,
		view: &usageView{query: usageQuery{source: usageSourcePeek}},
	})

	// both-agreeing-ok-badge
	tests = append(tests, &testCase{
		_id:       "both-agreeing-ok-badge",
		_expected: usageCell{Badge: badgeOk, Values: []string{"$0.50", "$0.50"}, Verdict: budget.AgreementAgree},

		pair: agreeing,
		view: &usageView{query: usageQuery{source: usageSourceBoth}},
	})

	// both-differing-error-badge
	tests = append(tests, &testCase{
		_id:       "both-differing-error-badge",
		_expected: usageCell{Badge: badgeError, Values: []string{"$2.00", "$0.50"}, Verdict: budget.AgreementDiffer},

		pair: differing,
		view: &usageView{query: usageQuery{source: usageSourceBoth}},
	})

	// chosen-side-missing-dash
	tests = append(tests, &testCase{
		_id:       "chosen-side-missing-dash",
		_expected: usageCell{Values: []string{usageDash}},

		pair: ccusageOnly,
		view: &usageView{query: usageQuery{source: usageSourcePeek}},
	})

	// both-one-side-missing-single-verdict
	tests = append(tests, &testCase{
		_id:       "both-one-side-missing-single-verdict",
		_expected: usageCell{Badge: badgeDim, Values: []string{usageDash, "$0.50"}, Verdict: budget.AgreementSingle},

		pair: ccusageOnly,
		view: &usageView{query: usageQuery{source: usageSourceBoth}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.cell(pairCost, test.pair, budget.CostTolerance))
		})
	}
}

func TestServer_HandleUsageRecalculateMissingBinary(t *testing.T) {
	stateDir := t.TempDir()
	options := &Options{CcusagePath: filepath.Join(t.TempDir(), "no-ccusage"), StateDir: stateDir}
	server := newTestServer(t, options)
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/usage/recalculate", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestServer_CcusagePath(t *testing.T) {
	configured := newTestServer(t, &Options{CcusagePath: "/somewhere/ccusage", StateDir: t.TempDir()})

	assert.Equal(t, "/somewhere/ccusage", configured.ccusagePath())
}

func TestUsageReturnUrl(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		referer string
	}

	tests := make([]*testCase, 0)

	// usage-view-query-kept
	tests = append(tests, &testCase{
		_id:       "usage-view-query-kept",
		_expected: "/usage?days=30&source=both",

		referer: "http://127.0.0.1:6100/usage?days=30&source=both",
	})

	// usage-view-without-query
	tests = append(tests, &testCase{
		_id:       "usage-view-without-query",
		_expected: "/usage",

		referer: "http://127.0.0.1:6100/usage",
	})

	// overview-query-kept
	tests = append(tests, &testCase{
		_id:       "overview-query-kept",
		_expected: "/?usage=30&window=last",

		referer: "http://127.0.0.1:6100/?usage=30&window=last",
	})

	// overview-without-query
	tests = append(tests, &testCase{
		_id:       "overview-without-query",
		_expected: "/",

		referer: "http://127.0.0.1:6100/",
	})

	// session-report-query-kept
	tests = append(tests, &testCase{
		_id:       "session-report-query-kept",
		_expected: "/sessions-report/abc?agents=root",

		referer: "http://127.0.0.1:6100/sessions-report/abc?agents=root",
	})

	// other-page-default-view
	tests = append(tests, &testCase{
		_id:       "other-page-default-view",
		_expected: "/usage",

		referer: "http://127.0.0.1:6100/profile?x=1",
	})

	// no-referer-default-view
	tests = append(tests, &testCase{
		_id:       "no-referer-default-view",
		_expected: "/usage",

		referer: "",
	})

	// unparsable-referer-default-view
	tests = append(tests, &testCase{
		_id:       "unparsable-referer-default-view",
		_expected: "/usage",

		referer: "http://[::1",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, usageReturnUrl(test.referer))
		})
	}
}

func TestPlanOptions(t *testing.T) {
	expected := []usagePlanOption{
		{IsSelected: false, Label: "Team Premium", Value: "team-premium"},
		{IsSelected: true, Label: "Max 20x", Value: "max-20x"},
	}

	assert.Equal(t, expected, planOptions(budget.PlanMax20x))
}

func TestUsageView_SessionRowsSkills(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedMore      string
		_expectedMoreCount int
		_expectedSkills    string

		skills []string
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	tests := make([]*testCase, 0)

	// no-skills
	tests = append(tests, &testCase{
		_id:                "no-skills",
		_expectedMore:      "",
		_expectedMoreCount: 0,
		_expectedSkills:    "",

		skills: nil,
	})

	// three-skills-all-shown
	tests = append(tests, &testCase{
		_id:                "three-skills-all-shown",
		_expectedMore:      "",
		_expectedMoreCount: 0,
		_expectedSkills:    "close, fdesign, peek",

		skills: []string{"peek", "close", "fdesign"},
	})

	// five-skills-two-folded
	tests = append(tests, &testCase{
		_id:                "five-skills-two-folded",
		_expectedMore:      "peek, qq",
		_expectedMoreCount: 2,
		_expectedSkills:    "close, fdesign, fimplement",

		skills: []string{"qq", "peek", "close", "fimplement", "fdesign"},
	})

	// repeated-skill-counted-once
	tests = append(tests, &testCase{
		_id:                "repeated-skill-counted-once",
		_expectedMore:      "",
		_expectedMoreCount: 0,
		_expectedSkills:    "close, peek",

		skills: []string{"peek", "close", "peek", "close"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record := provideUsageRecord("skills", now, func(record *budget.Record) {
				for _, skill := range test.skills {
					run := budget.SkillRun{Seconds: 1, Skill: skill}
					record.Peek.SkillRuns = append(record.Peek.SkillRuns, run)
				}
			})
			row := newLoadedUsageView(now, []*budget.Record{record}).sessionRows()[0]
			assert.Equal(t, test._expectedSkills, row.Skills)
			assert.Equal(t, test._expectedMore, row.MoreSkills)
			assert.Equal(t, test._expectedMoreCount, row.MoreSkillsCount)
		})
	}
}

func TestUsageView_EmptyChart(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *usageChart

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	tests := make([]*testCase, 0)

	// usd-without-cost-nil
	tests = append(tests, &testCase{
		_id:       "usd-without-cost-nil",
		_expected: nil,

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourceCcusage, unit: usageUnitUsd}, nil),
	})

	// plan-without-split-keeps-unit-chips
	tests = append(tests, &testCase{
		_id: "plan-without-split-keeps-unit-chips",
		_expected: &usageChart{
			Plan: "Team Premium",
			Units: []usageChip{
				{Active: false, Label: "$", Url: "/usage?days=1"},
				{Active: true, Label: "plan %", Url: "/usage?days=1&unit=plan"},
			},
		},

		view: newQueriedUsageView(
			now,
			usageQuery{days: 1, source: usageSourcePeek, unit: usageUnitPlan},
			[]*budget.Record{provideUsageRecord("no-split", now)},
		),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.chart())
		})
	}
}

func TestUsageView_Chart(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *usageChart

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	week := budget.NewPresetPeriod(7, now)
	weekUsd := usageQuery{days: 7, source: usageSourceBoth, unit: usageUnitUsd}
	weekPlan := usageQuery{days: 7, source: usageSourceBoth, unit: usageUnitPlan}
	usdChips := []usageChip{
		{Active: true, Label: "$", Url: "/usage?source=both"},
		{Active: false, Label: "plan %", Url: "/usage?source=both&unit=plan"},
	}
	planChips := []usageChip{
		{Active: false, Label: "$", Url: "/usage?source=both"},
		{Active: true, Label: "plan %", Url: "/usage?source=both&unit=plan"},
	}
	tests := make([]*testCase, 0)

	// usd-one-column-per-day
	oneSessionDays := emptyChartDays(1, week, "$0.00")
	oneSessionDays[6] = usageDay{Height: "100.0", Label: "10-03", Title: "2026-10-03 — $0.50", Value: "$0.50"}
	oneSessionRecords := []*budget.Record{provideUsageRecord("today", now)}
	tests = append(tests, &testCase{
		_id: "usd-one-column-per-day",
		_expected: &usageChart{
			Average: "$0.07",
			Days:    oneSessionDays,
			Half:    "$0.25",
			Max:     "$0.50",
			Total:   "$0.50",
			Units:   usdChips,
		},

		view: newQueriedUsageView(now, weekUsd, oneSessionRecords),
	})

	// day-without-sessions-zero-column
	gapDays := emptyChartDays(1, week, "$0.00")
	gapDays[4] = usageDay{Height: "50.0", Label: "10-01", Title: "2026-10-01 — $0.50", Value: "$0.50"}
	gapDays[6] = usageDay{Height: "100.0", Label: "10-03", Title: "2026-10-03 — $1.00", Value: "$1.00"}
	gapRecords := []*budget.Record{
		provideUsageRecord("two-days-ago", now.Add(-48*time.Hour)),
		provideUsageRecord("today-first", now.Add(-time.Hour)),
		provideUsageRecord("today-second", now),
	}
	tests = append(tests, &testCase{
		_id: "day-without-sessions-zero-column",
		_expected: &usageChart{
			Average: "$0.21",
			Days:    gapDays,
			Half:    "$0.50",
			Max:     "$1.00",
			Total:   "$1.50",
			Units:   usdChips,
		},

		view: newQueriedUsageView(now, weekUsd, gapRecords),
	})

	// no-cost-nil
	noCostRecords := []*budget.Record{provideUsageRecord("free", now, withRecordCost(0, 0))}
	tests = append(tests, &testCase{
		_id:       "no-cost-nil",
		_expected: nil,

		view: newQueriedUsageView(now, weekUsd, noCostRecords),
	})

	// plan-unit-from-split-not-cost
	// today: the $5 Opus window is 100%; two days ago (index 4): a Claude
	// session without a split is a 0.0 column; yesterday (index 5): Codex never
	// draws on the plan. Average 100% / 7 days = 14.3%.
	planDays := emptyChartDays(1, week, "0%")
	planDays[6] = usageDay{Height: "100.0", Label: "10-03", Title: "2026-10-03 — 100%", Value: "100%"}
	planRecords := []*budget.Record{
		provideUsageRecord("opus", now, withRecordCost(5, 5), withRecordModels(usageOpusWindow)),
		provideUsageRecord("no-split", now.Add(-48*time.Hour)),
		provideUsageRecord("codex", now.Add(-24*time.Hour), withRecordCost(1, 1), withRecordCodex(), withRecordModels(usageOpusWindow)),
	}
	tests = append(tests, &testCase{
		_id: "plan-unit-from-split-not-cost",
		_expected: &usageChart{
			Average: "14%",
			Days:    planDays,
			Half:    "50%",
			Max:     "100%",
			Plan:    "Team Premium",
			Total:   "100%",
			Units:   planChips,
		},

		view: newQueriedUsageView(now, weekPlan, planRecords),
	})

	// plan-unit-follows-view-plan
	// the Opus window is 31.55% of a Max 20x window; average 31.55% / 7 = 4.5%,
	// half 15.8%
	maxDays := emptyChartDays(1, week, "0%")
	maxDays[6] = usageDay{Height: "100.0", Label: "10-03", Title: "2026-10-03 — 32%", Value: "32%"}
	maxView := newQueriedUsageView(now, weekPlan, planRecords)
	maxView.plan = budget.PlanMax20x
	tests = append(tests, &testCase{
		_id: "plan-unit-follows-view-plan",
		_expected: &usageChart{
			Average: "5%",
			Days:    maxDays,
			Half:    "16%",
			Max:     "32%",
			Plan:    "Max 20x",
			Total:   "32%",
			Units:   planChips,
		},

		view: maxView,
	})

	// usd-unit-no-plan-label
	// the same sessions in dollars: $0.50 + $1.00 Codex + $5.00 = $6.50,
	// average $6.50 / 7 = $0.93
	usdDays := emptyChartDays(1, week, "$0.00")
	usdDays[4] = usageDay{Height: "10.0", Label: "10-01", Title: "2026-10-01 — $0.50", Value: "$0.50"}
	usdDays[5] = usageDay{Height: "20.0", Label: "10-02", Title: "2026-10-02 — $1.00", Value: "$1.00"}
	usdDays[6] = usageDay{Height: "100.0", Label: "10-03", Title: "2026-10-03 — $5.00", Value: "$5.00"}
	tests = append(tests, &testCase{
		_id: "usd-unit-no-plan-label",
		_expected: &usageChart{
			Average: "$0.93",
			Days:    usdDays,
			Half:    "$2.50",
			Max:     "$5.00",
			Plan:    "",
			Total:   "$6.50",
			Units:   usdChips,
		},

		view: newQueriedUsageView(now, weekUsd, planRecords),
	})

	// source-peek-cost
	peekRecords := []*budget.Record{provideUsageRecord("today", now, withRecordCost(2, 0.5))}
	tests = append(tests, &testCase{
		_id: "source-peek-cost",
		_expected: &usageChart{
			Average: "$0.07",
			Days:    oneSessionDays,
			Half:    "$0.25",
			Max:     "$0.50",
			Total:   "$0.50",
			Units: []usageChip{
				{Active: true, Label: "$", Url: "/usage"},
				{Active: false, Label: "plan %", Url: "/usage?unit=plan"},
			},
		},

		view: newQueriedUsageView(now, usageQuery{days: 7, source: usageSourcePeek, unit: usageUnitUsd}, peekRecords),
	})

	// thirty-days-thinned-labels
	monthDays := emptyChartDays(3, budget.NewPresetPeriod(30, now), "$0.00")
	monthDays[29] = usageDay{Height: "100.0", Title: "2026-10-03 — $0.50"}
	tests = append(tests, &testCase{
		_id: "thirty-days-thinned-labels",
		_expected: &usageChart{
			Average: "$0.02",
			Days:    monthDays,
			Half:    "$0.25",
			Max:     "$0.50",
			Total:   "$0.50",
			Units: []usageChip{
				{Active: true, Label: "$", Url: "/usage?days=30&source=both"},
				{Active: false, Label: "plan %", Url: "/usage?days=30&source=both&unit=plan"},
			},
		},

		view: newQueriedUsageView(now, usageQuery{days: 30, source: usageSourceBoth, unit: usageUnitUsd}, oneSessionRecords),
	})

	// total-and-average-over-period
	totalDays := emptyChartDays(1, week, "$0.00")
	totalDays[1] = usageDay{Height: "100.0", Label: "09-28", Title: "2026-09-28 — $2.00", Value: "$2.00"}
	totalDays[6] = usageDay{Height: "75.0", Label: "10-03", Title: "2026-10-03 — $1.50", Value: "$1.50"}
	totalRecords := []*budget.Record{
		provideUsageRecord("five-days-ago", now.AddDate(0, 0, -5), withRecordCost(2, 2)),
		provideUsageRecord("today", now, withRecordCost(1.5, 1.5)),
	}
	tests = append(tests, &testCase{
		_id: "total-and-average-over-period",
		_expected: &usageChart{
			Average: "$0.50",
			Days:    totalDays,
			Half:    "$1.00",
			Max:     "$2.00",
			Total:   "$3.50",
			Units:   usdChips,
		},

		view: newQueriedUsageView(now, weekUsd, totalRecords),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.chart())
		})
	}
}

func TestUsageView_Chips(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []usageChip

		active string
		names  []string
		view   *usageView
	}

	tests := make([]*testCase, 0)

	// no-filter-all-first-active
	tests = append(tests, &testCase{
		_id: "no-filter-all-first-active",
		_expected: []usageChip{
			{Active: true, IsAll: true, Url: "/usage"},
			{Label: "a", Url: "/usage?repo=a"},
			{Label: "b", Url: "/usage?repo=b"},
		},

		active: "",
		names:  []string{"b", "a"},
		view:   &usageView{query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitUsd}},
	})

	// filter-set-named-chip-links-to-itself
	tests = append(tests, &testCase{
		_id: "filter-set-named-chip-links-to-itself",
		_expected: []usageChip{
			{IsAll: true, Url: "/usage"},
			{Active: true, Label: "a", Url: "/usage?repo=a"},
			{Label: "b", Url: "/usage?repo=b"},
		},

		active: "a",
		names:  []string{"a", "b"},
		view:   &usageView{query: usageQuery{days: budget.DefaultDays, repo: "a", source: usageSourcePeek, unit: usageUnitUsd}},
	})

	// filter-set-all-keeps-other-filters
	tests = append(tests, &testCase{
		_id: "filter-set-all-keeps-other-filters",
		_expected: []usageChip{
			{IsAll: true, Url: "/usage?agent=codex&source=both"},
			{Active: true, Label: "a", Url: "/usage?agent=codex&repo=a&source=both"},
			{Label: "b", Url: "/usage?agent=codex&repo=b&source=both"},
		},

		active: "a",
		names:  []string{"a", "b"},
		view:   &usageView{query: usageQuery{agent: "codex", days: budget.DefaultDays, repo: "a", source: usageSourceBoth, unit: usageUnitUsd}},
	})

	// duplicate-names-collapse
	tests = append(tests, &testCase{
		_id: "duplicate-names-collapse",
		_expected: []usageChip{
			{Active: true, IsAll: true, Url: "/usage"},
			{Label: "a", Url: "/usage?repo=a"},
			{Label: "b", Url: "/usage?repo=b"},
		},

		active: "",
		names:  []string{"a", "b", "a", "b"},
		view:   &usageView{query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitUsd}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.chips(test.active, test.names, withUsageRepo))
		})
	}
}

func TestUsageView_ContextKinds(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []usageKindRow

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	tests := make([]*testCase, 0)

	// sorted-by-tokens-descending
	sortedRecords := []*budget.Record{provideUsageRecord("sorted", now, func(record *budget.Record) {
		record.Context = &budget.ContextSummary{
			Kinds: []budget.ContextKind{
				{Kind: contextrecord.KindGlobal, Totals: contextrecord.Totals{Chars: 40, Items: 1, Tokens: 10}},
				{Kind: contextrecord.KindSkill, Totals: contextrecord.Totals{Chars: 1200, Items: 2, Tokens: 300}},
				{Kind: contextrecord.KindLang, Totals: contextrecord.Totals{Chars: 200, Items: 1, Tokens: 50}},
			},
			Totals: contextrecord.Totals{Chars: 1440, Items: 4, Tokens: 360},
		}
	})}
	tests = append(tests, &testCase{
		_id: "sorted-by-tokens-descending",
		_expected: []usageKindRow{
			{tokens: 300, Chars: "1 200", Items: 2, Kind: contextrecord.KindSkill, Tokens: "300"},
			{tokens: 50, Chars: "200", Items: 1, Kind: contextrecord.KindLang, Tokens: "50"},
			{tokens: 10, Chars: "40", Items: 1, Kind: contextrecord.KindGlobal, Tokens: "10"},
		},

		view: newLoadedUsageView(now, sortedRecords),
	})

	// equal-tokens-keep-kind-order
	equalRecords := []*budget.Record{provideUsageRecord("equal", now, func(record *budget.Record) {
		record.Context = &budget.ContextSummary{
			Kinds: []budget.ContextKind{
				{Kind: contextrecord.KindSkill, Totals: contextrecord.Totals{Chars: 400, Items: 1, Tokens: 100}},
				{Kind: contextrecord.KindGlobal, Totals: contextrecord.Totals{Chars: 400, Items: 1, Tokens: 100}},
			},
			Totals: contextrecord.Totals{Chars: 800, Items: 2, Tokens: 200},
		}
	})}
	tests = append(tests, &testCase{
		_id: "equal-tokens-keep-kind-order",
		_expected: []usageKindRow{
			{tokens: 100, Chars: "400", Items: 1, Kind: contextrecord.KindGlobal, Tokens: "100"},
			{tokens: 100, Chars: "400", Items: 1, Kind: contextrecord.KindSkill, Tokens: "100"},
		},

		view: newLoadedUsageView(now, equalRecords),
	})

	// no-context-no-rows
	noContextRecords := []*budget.Record{provideUsageRecord("no-context", now)}
	tests = append(tests, &testCase{
		_id:       "no-context-no-rows",
		_expected: nil,

		view: newLoadedUsageView(now, noContextRecords),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.contextKinds())
		})
	}
}

func TestUsageView_RangeFields(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []usageField

		view *usageView
	}

	tests := make([]*testCase, 0)

	// no-filter-empty
	tests = append(tests, &testCase{
		_id:       "no-filter-empty",
		_expected: []usageField{},

		view: &usageView{query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitUsd}, sources: usageSourceOrder},
	})

	// all-filters-sorted
	allFilters := usageQuery{
		agent:  "codex",
		days:   budget.DefaultDays,
		only:   usageOnlyDiffer,
		repo:   "smine",
		skill:  "fdesign",
		source: usageSourceBoth,
		unit:   usageUnitPlan,
	}
	tests = append(tests, &testCase{
		_id: "all-filters-sorted",
		_expected: []usageField{
			{Name: "agent", Value: "codex"},
			{Name: "only", Value: "differ"},
			{Name: "repo", Value: "smine"},
			{Name: "skill", Value: "fdesign"},
			{Name: "source", Value: "both"},
			{Name: "unit", Value: "plan"},
		},

		view: &usageView{query: allFilters},
	})

	// period-values-never-included
	tests = append(tests, &testCase{
		_id:       "period-values-never-included",
		_expected: []usageField{{Name: "source", Value: "ccusage"}},

		view: &usageView{query: usageQuery{days: 30, from: "2026-09-01", source: usageSourceCcusage, to: "2026-09-10", unit: usageUnitUsd}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.rangeFields())
		})
	}
}

func TestUsageView_SessionRows(t *testing.T) {
	type testCase struct {
		_id                         string
		_expectedBadge              string
		_expectedContextTokensValue int
		_expectedPlanPercent        string
		_expectedTokensValue        int64
		_expectedVerdict            budget.Agreement

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	differs := provideUsageRecord("differs", now, func(record *budget.Record) {
		record.Ccusage.Usage.Input = 1030
	})
	differsRecords := []*budget.Record{differs}
	agreesRecords := []*budget.Record{provideUsageRecord("agrees", now)}
	contextRecords := []*budget.Record{provideUsageRecord("with-context", now, func(record *budget.Record) {
		record.Context = &budget.ContextSummary{Totals: contextrecord.Totals{Chars: 4000, Items: 3, Tokens: 1234}}
	})}
	// the Opus window plus a tenth in Fable: 100% + 10% = 110%, whatever the
	// sides' $2.00 / $0.50 cost
	splitRecords := []*budget.Record{provideUsageRecord("split", now, withRecordCost(2, 0.5), withRecordModels(usageOpusWindow, usageFableTenth))}
	tests := make([]*testCase, 0)

	// tokens-value-follows-ccusage
	tests = append(tests, &testCase{
		_id:                         "tokens-value-follows-ccusage",
		_expectedBadge:              "",
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "",
		_expectedTokensValue:        3530,
		_expectedVerdict:            "",

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourceCcusage, unit: usageUnitUsd}, differsRecords),
	})

	// tokens-value-follows-peek
	tests = append(tests, &testCase{
		_id:                         "tokens-value-follows-peek",
		_expectedBadge:              "",
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "",
		_expectedTokensValue:        2530,
		_expectedVerdict:            "",

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourcePeek, unit: usageUnitUsd}, differsRecords),
	})

	// context-tokens-value-with-context
	tests = append(tests, &testCase{
		_id:                         "context-tokens-value-with-context",
		_expectedBadge:              badgeOk,
		_expectedContextTokensValue: 1234,
		_expectedPlanPercent:        "",
		_expectedTokensValue:        2530,
		_expectedVerdict:            budget.AgreementAgree,

		view: newLoadedUsageView(now, contextRecords),
	})

	// context-tokens-value-zero-without-context
	tests = append(tests, &testCase{
		_id:                         "context-tokens-value-zero-without-context",
		_expectedBadge:              badgeOk,
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "",
		_expectedTokensValue:        2530,
		_expectedVerdict:            budget.AgreementAgree,

		view: newLoadedUsageView(now, agreesRecords),
	})

	// badge-and-verdict-under-both
	tests = append(tests, &testCase{
		_id:                         "badge-and-verdict-under-both",
		_expectedBadge:              badgeError,
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "",
		_expectedTokensValue:        2530,
		_expectedVerdict:            budget.AgreementDiffer,

		view: newLoadedUsageView(now, differsRecords),
	})

	// plan-percent-from-split-under-ccusage
	tests = append(tests, &testCase{
		_id:                         "plan-percent-from-split-under-ccusage",
		_expectedBadge:              "",
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "110%",
		_expectedTokensValue:        2530,
		_expectedVerdict:            "",

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourceCcusage, unit: usageUnitUsd}, splitRecords),
	})

	// plan-percent-from-split-under-peek
	tests = append(tests, &testCase{
		_id:                         "plan-percent-from-split-under-peek",
		_expectedBadge:              "",
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "110%",
		_expectedTokensValue:        2530,
		_expectedVerdict:            "",

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourcePeek, unit: usageUnitUsd}, splitRecords),
	})

	// plan-percent-from-split-under-both
	tests = append(tests, &testCase{
		_id:                         "plan-percent-from-split-under-both",
		_expectedBadge:              badgeError,
		_expectedContextTokensValue: 0,
		_expectedPlanPercent:        "110%",
		_expectedTokensValue:        2530,
		_expectedVerdict:            budget.AgreementDiffer,

		view: newLoadedUsageView(now, splitRecords),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			row := test.view.sessionRows()[0]
			assert.Equal(t, test._expectedBadge, row.Badge)
			assert.Equal(t, test._expectedContextTokensValue, row.ContextTokensValue)
			assert.Equal(t, test._expectedPlanPercent, row.PlanPercent)
			assert.Equal(t, test._expectedTokensValue, row.TokensValue)
			assert.Equal(t, test._expectedVerdict, row.Verdict)
		})
	}
}

func TestUsageView_SkillRows(t *testing.T) {
	type testCase struct {
		_id             string
		_expected       []usageSkillRow
		_expectedTokens int

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	tests := make([]*testCase, 0)

	// tokens-exclude-cache-read
	oneRun := provideUsageRecord("one-run", now, func(record *budget.Record) {
		record.Peek.SkillRuns = []budget.SkillRun{
			{Seconds: 60, Skill: "fdesign", Usage: budget.TokenClasses{CacheCreation: 40, CacheRead: 1500, Input: 10, Output: 100}},
		}
	})
	tests = append(tests, &testCase{
		_id: "tokens-exclude-cache-read",
		_expected: []usageSkillRow{
			{
				seconds:        60,
				CacheRead:      "1 500",
				CacheReadValue: 1500,
				Name:           "fdesign",
				Runs:           1,
				Seconds:        "1m",
				Tokens:         "150",
				TokensValue:    150,
				Url:            "/usage?days=1&skill=fdesign&source=both",
			},
			{CacheRead: "500", CacheReadValue: 500, Name: usageOutsideSkills, Seconds: "0s", Tokens: "380", TokensValue: 380},
			{CacheRead: "0", Name: usageSubagents, Seconds: "0s", Tokens: "0"},
		},
		_expectedTokens: oneRun.Peek.Total().Total(),

		view: newLoadedUsageView(now, []*budget.Record{oneRun}),
	})

	// cache-read-column-per-skill
	twoSkills := provideUsageRecord("two-skills", now, func(record *budget.Record) {
		record.Peek.SkillRuns = []budget.SkillRun{
			{Seconds: 60, Skill: "fdesign", Usage: budget.TokenClasses{CacheRead: 300, Output: 50}},
			{Seconds: 30, Skill: "review", Usage: budget.TokenClasses{CacheRead: 900, Output: 20}},
		}
	})
	tests = append(tests, &testCase{
		_id: "cache-read-column-per-skill",
		_expected: []usageSkillRow{
			{
				seconds:        60,
				CacheRead:      "300",
				CacheReadValue: 300,
				Name:           "fdesign",
				Runs:           1,
				Seconds:        "1m",
				Tokens:         "50",
				TokensValue:    50,
				Url:            "/usage?days=1&skill=fdesign&source=both",
			},
			{
				seconds:        30,
				CacheRead:      "900",
				CacheReadValue: 900,
				Name:           "review",
				Runs:           1,
				Seconds:        "30s",
				Tokens:         "20",
				TokensValue:    20,
				Url:            "/usage?days=1&skill=review&source=both",
			},
			{CacheRead: "800", CacheReadValue: 800, Name: usageOutsideSkills, Seconds: "0s", Tokens: "460", TokensValue: 460},
			{CacheRead: "0", Name: usageSubagents, Seconds: "0s", Tokens: "0"},
		},
		_expectedTokens: twoSkills.Peek.Total().Total(),

		view: newLoadedUsageView(now, []*budget.Record{twoSkills}),
	})

	// rows-add-up-to-peek-total
	withRuns := provideUsageRecord("with-runs", now, func(record *budget.Record) {
		record.Peek.SkillRuns = []budget.SkillRun{
			{Seconds: 60, Skill: "fdesign", Usage: budget.TokenClasses{CacheRead: 500, Input: 10, Output: 100}},
			{Seconds: 30, Skill: "fimplement", Usage: budget.TokenClasses{CacheRead: 700, Output: 50}},
		}
		record.Peek.Subagents = budget.TokenClasses{CacheRead: 9000, Input: 5, Output: 70}
	})
	tests = append(tests, &testCase{
		_id: "rows-add-up-to-peek-total",
		_expected: []usageSkillRow{
			{
				seconds:        60,
				CacheRead:      "500",
				CacheReadValue: 500,
				Name:           "fdesign",
				Runs:           1,
				Seconds:        "1m",
				Tokens:         "110",
				TokensValue:    110,
				Url:            "/usage?days=1&skill=fdesign&source=both",
			},
			{
				seconds:        30,
				CacheRead:      "700",
				CacheReadValue: 700,
				Name:           "fimplement",
				Runs:           1,
				Seconds:        "30s",
				Tokens:         "50",
				TokensValue:    50,
				Url:            "/usage?days=1&skill=fimplement&source=both",
			},
			{CacheRead: "800", CacheReadValue: 800, Name: usageOutsideSkills, Seconds: "0s", Tokens: "370", TokensValue: 370},
			{CacheRead: "9 000", CacheReadValue: 9000, Name: usageSubagents, Seconds: "0s", Tokens: "75", TokensValue: 75},
		},
		_expectedTokens: withRuns.Peek.Total().Total(),

		view: newLoadedUsageView(now, []*budget.Record{withRuns}),
	})

	// sorted-by-tokens-remainder-last
	unsorted := provideUsageRecord("unsorted", now, func(record *budget.Record) {
		record.Peek.SkillRuns = []budget.SkillRun{
			{Seconds: 10, Skill: "small", Usage: budget.TokenClasses{Output: 10}},
			{Seconds: 20, Skill: "big", Usage: budget.TokenClasses{Output: 200}},
			{Seconds: 5, Skill: "big", Usage: budget.TokenClasses{Output: 100}},
		}
		record.Peek.Subagents = budget.TokenClasses{Input: 1000}
	})
	tests = append(tests, &testCase{
		_id: "sorted-by-tokens-remainder-last",
		_expected: []usageSkillRow{
			{
				seconds:     25,
				CacheRead:   "0",
				Name:        "big",
				Runs:        2,
				Seconds:     "25s",
				Tokens:      "300",
				TokensValue: 300,
				Url:         "/usage?days=1&skill=big&source=both",
			},
			{
				seconds:     10,
				CacheRead:   "0",
				Name:        "small",
				Runs:        1,
				Seconds:     "10s",
				Tokens:      "10",
				TokensValue: 10,
				Url:         "/usage?days=1&skill=small&source=both",
			},
			{CacheRead: "2 000", CacheReadValue: 2000, Name: usageOutsideSkills, Seconds: "0s", Tokens: "220", TokensValue: 220},
			{CacheRead: "0", Name: usageSubagents, Seconds: "0s", Tokens: "1 000", TokensValue: 1000},
		},
		_expectedTokens: unsorted.Peek.Total().Total(),

		view: newLoadedUsageView(now, []*budget.Record{unsorted}),
	})

	// no-peek-side-skipped
	noPeek := provideUsageRecord("no-peek", now, func(record *budget.Record) {
		record.Peek = nil
	})
	tests = append(tests, &testCase{
		_id: "no-peek-side-skipped",
		_expected: []usageSkillRow{
			{CacheRead: "0", Name: usageOutsideSkills, Seconds: "0s", Tokens: "0"},
			{CacheRead: "0", Name: usageSubagents, Seconds: "0s", Tokens: "0"},
		},
		_expectedTokens: 0,

		view: newLoadedUsageView(now, []*budget.Record{noPeek}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rows := test.view.skillRows()
			assert.Equal(t, test._expected, rows)
			assert.Equal(t, test._expectedTokens, skillRowTokens(rows))
		})
	}
}

func newLoadedUsageView(now time.Time, records []*budget.Record) *usageView {
	view := &usageView{
		location: usageTestLocation,
		period:   budget.NewPresetPeriod(1, now),
		plan:     budget.DefaultPlan,
		query:    usageQuery{days: 1, source: usageSourceBoth, unit: usageUnitUsd},
	}
	view.load(records)
	return view
}

func skillRowTokens(rows []usageSkillRow) int {
	sum := 0
	for _, row := range rows {
		sum += row.TokensValue + row.CacheReadValue
	}
	return sum
}

func TestUsageView_SourceChips(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []usageChip

		view *usageView
	}

	tests := make([]*testCase, 0)

	// three-chips-in-order
	tests = append(tests, &testCase{
		_id: "three-chips-in-order",
		_expected: []usageChip{
			{Active: true, Label: "peek", Url: "/usage"},
			{Label: "ccusage", Url: "/usage?source=ccusage"},
			{Label: "both", Url: "/usage?source=both"},
		},

		view: &usageView{query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitUsd}, sources: usageSourceOrder},
	})

	// active-follows-query
	tests = append(tests, &testCase{
		_id: "active-follows-query",
		_expected: []usageChip{
			{Label: "peek", Url: "/usage"},
			{Active: true, Label: "ccusage", Url: "/usage?source=ccusage"},
			{Label: "both", Url: "/usage?source=both"},
		},

		view: &usageView{query: usageQuery{days: budget.DefaultDays, source: usageSourceCcusage, unit: usageUnitUsd}, sources: usageSourceOrder},
	})

	// leaving-both-drops-only
	tests = append(tests, &testCase{
		_id: "leaving-both-drops-only",
		_expected: []usageChip{
			{Label: "peek", Url: "/usage"},
			{Label: "ccusage", Url: "/usage?source=ccusage"},
			{Active: true, Label: "both", Url: "/usage?only=differ&source=both"},
		},

		view: &usageView{query: usageQuery{days: budget.DefaultDays, only: usageOnlyDiffer, source: usageSourceBoth, unit: usageUnitUsd}, sources: usageSourceOrder},
	})

	// ccusage-off-one-chip
	tests = append(tests, &testCase{
		_id: "ccusage-off-one-chip",
		_expected: []usageChip{
			{Active: true, Label: "peek", Url: "/usage"},
		},

		view: &usageView{query: usageQuery{days: budget.DefaultDays, source: usageSourcePeek, unit: usageUnitUsd}, sources: []usageSource{usageSourcePeek}},
	})

	// skill-filter-no-chips
	tests = append(tests, &testCase{
		_id:       "skill-filter-no-chips",
		_expected: nil,

		view: &usageView{query: usageQuery{days: budget.DefaultDays, skill: "fdesign", source: usageSourcePeek, unit: usageUnitUsd}, sources: usageSourceOrder},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.sourceChips())
		})
	}
}

func TestUsageView_Tiles(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []usageTile

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	agreesRecords := []*budget.Record{provideUsageRecord("agrees", now)}
	differsRecords := []*budget.Record{provideUsageRecord("differs", now, withRecordCost(2, 0.5), func(record *budget.Record) {
		record.Ccusage.Usage.Input = 1030
	})}
	active := usageTile{Cell: usageCell{Values: []string{"5m"}}, Detail: "idle 5m · peek", Label: "Active time"}
	sessions := usageTile{Cell: usageCell{Values: []string{"1"}}, Label: usageSessionsLabel}
	injected := usageTile{Cell: usageCell{Values: []string{"0 chars"}}, Detail: "0 of 1 sessions · 0 tokens", Label: "Injected context"}
	bothAgree := func(value string) usageCell {
		return usageCell{Badge: badgeOk, Values: []string{value, value}, Verdict: budget.AgreementAgree}
	}
	tests := make([]*testCase, 0)

	// eight-tiles-no-wall-time
	tests = append(tests, &testCase{
		_id: "eight-tiles-no-wall-time",
		_expected: []usageTile{
			{Cell: usageCell{Values: []string{"$0.50"}}, Label: "Cost"},
			{Cell: usageCell{Values: []string{"30"}}, Label: "Input tokens"},
			{Cell: usageCell{Values: []string{"400"}}, Label: "Output tokens"},
			{Cell: usageCell{Values: []string{"100"}}, Label: "Cache write"},
			{Cell: usageCell{Values: []string{"2 000"}}, Label: "Cache read"},
			active,
			sessions,
			injected,
		},

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourcePeek, unit: usageUnitUsd}, agreesRecords),
	})

	// single-source-one-value-no-badge
	tests = append(tests, &testCase{
		_id: "single-source-one-value-no-badge",
		_expected: []usageTile{
			{Cell: usageCell{Values: []string{"$2.00"}}, Label: "Cost"},
			{Cell: usageCell{Values: []string{"1 030"}}, Label: "Input tokens"},
			{Cell: usageCell{Values: []string{"400"}}, Label: "Output tokens"},
			{Cell: usageCell{Values: []string{"100"}}, Label: "Cache write"},
			{Cell: usageCell{Values: []string{"2 000"}}, Label: "Cache read"},
			active,
			sessions,
			injected,
		},

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourceCcusage, unit: usageUnitUsd}, differsRecords),
	})

	// both-two-values-on-paired-tiles
	tests = append(tests, &testCase{
		_id: "both-two-values-on-paired-tiles",
		_expected: []usageTile{
			{Cell: bothAgree("$0.50"), Label: "Cost"},
			{Cell: bothAgree("30"), Label: "Input tokens"},
			{Cell: bothAgree("400"), Label: "Output tokens"},
			{Cell: bothAgree("100"), Label: "Cache write"},
			{Cell: bothAgree("2 000"), Label: "Cache read"},
			active,
			sessions,
			injected,
		},

		view: newLoadedUsageView(now, agreesRecords),
	})

	// cost-detail-is-plan-detail
	// the Opus window is 100% of the Team Premium window, × 0.16 = 16% weekly
	splitRecords := []*budget.Record{provideUsageRecord("split", now, withRecordModels(usageOpusWindow))}
	tests = append(tests, &testCase{
		_id: "cost-detail-is-plan-detail",
		_expected: []usageTile{
			{Cell: bothAgree("$0.50"), Detail: "100% of a Team Premium 5-hour window · 16% of the weekly limit", Label: "Cost"},
			{Cell: bothAgree("30"), Label: "Input tokens"},
			{Cell: bothAgree("400"), Label: "Output tokens"},
			{Cell: bothAgree("100"), Label: "Cache write"},
			{Cell: bothAgree("2 000"), Label: "Cache read"},
			active,
			sessions,
			injected,
		},

		view: newLoadedUsageView(now, splitRecords),
	})

	// skill-filter-seven-tiles-of-the-runs
	// the run is a tenth of the session: $0.05, 10% of the window, 16% × 0.1 weekly
	skillRecords := []*budget.Record{provideUsageRecord("skill", now, withRecordModels(usageOpusWindow), withRecordSkillRun("fdesign", 60))}
	tests = append(tests, &testCase{
		_id: "skill-filter-seven-tiles-of-the-runs",
		_expected: []usageTile{
			{Cell: usageCell{Values: []string{"$0.05"}}, Detail: "10% of a Team Premium 5-hour window · 2% of the weekly limit", Label: "Cost"},
			{Cell: usageCell{Values: []string{"3"}}, Label: "Input tokens"},
			{Cell: usageCell{Values: []string{"40"}}, Label: "Output tokens"},
			{Cell: usageCell{Values: []string{"10"}}, Label: "Cache write"},
			{Cell: usageCell{Values: []string{"200"}}, Label: "Cache read"},
			{Cell: usageCell{Values: []string{"1m"}}, Detail: "wall span of the runs · peek", Label: "Active time"},
			sessions,
		},

		view: newQueriedUsageView(now, usageQuery{days: 1, skill: "fdesign", source: usageSourcePeek, unit: usageUnitUsd}, skillRecords),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.tiles())
		})
	}
}

func TestUsageView_CsvRows(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	record := provideUsageRecord("csv", now, func(record *budget.Record) {
		record.Ccusage.Usage.CacheRead = 1234567
	})
	rows := newLoadedUsageView(now, []*budget.Record{record}).csvRows()

	require.Len(t, rows, 2)
	assert.Len(t, rows[1], len(rows[0]))
	assert.Equal(t, usageCsvHeader, rows[0])
	assert.Equal(t, "1234567", rows[1][9])
	assert.Equal(t, "0.5000", rows[1][10])
	assert.Equal(t, "600", rows[1][16])
	assert.Equal(t, "2026-10-03T12:00:00+02:00", rows[1][5])
	assert.Equal(t, "plan_percent", rows[0][23])
	assert.Empty(t, rows[1][23])
}

func TestUsageView_CsvRowsPlanPercent(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	// the Opus window plus a tenth in Fable: 100% + 10% = 110%
	splitRecords := []*budget.Record{provideUsageRecord("split", now, withRecordCost(2, 0.5), withRecordModels(usageOpusWindow, usageFableTenth))}
	tests := make([]*testCase, 0)

	// split-under-both
	tests = append(tests, &testCase{
		_id:       "split-under-both",
		_expected: "110%",

		view: newLoadedUsageView(now, splitRecords),
	})

	// split-under-peek
	tests = append(tests, &testCase{
		_id:       "split-under-peek",
		_expected: "110%",

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourcePeek, unit: usageUnitUsd}, splitRecords),
	})

	// split-under-ccusage
	tests = append(tests, &testCase{
		_id:       "split-under-ccusage",
		_expected: "110%",

		view: newQueriedUsageView(now, usageQuery{days: 1, source: usageSourceCcusage, unit: usageUnitUsd}, splitRecords),
	})

	// no-split-empty
	tests = append(tests, &testCase{
		_id:       "no-split-empty",
		_expected: "",

		view: newLoadedUsageView(now, []*budget.Record{provideUsageRecord("no-split", now)}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.csvRows()[1][23])
		})
	}
}

func TestUsageView_PlanPercent(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		sums usageSums
		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	teamPremium := &usageView{plan: budget.PlanTeamPremium}
	tests := make([]*testCase, 0)

	// opus-measured-window-full
	tests = append(tests, &testCase{
		_id:       "opus-measured-window-full",
		_expected: "100%",

		sums: newUsageSums(provideUsageRecord("opus", now, withRecordModels(usageOpusWindow))),
		view: teamPremium,
	})

	// fable-weights
	// 27 441 061.7375 / 12 927 617.99 = 212.27%
	tests = append(tests, &testCase{
		_id:       "fable-weights",
		_expected: "212%",

		sums: newUsageSums(provideUsageRecord("fable", now, withRecordModels(usageFableWindow))),
		view: teamPremium,
	})

	// switched-models-summed
	// (12 927 617.99 + 1 292 762.5) / 12 927 617.99 = 110.00%
	tests = append(tests, &testCase{
		_id:       "switched-models-summed",
		_expected: "110%",

		sums: newUsageSums(provideUsageRecord("switched", now, withRecordModels(usageOpusWindow, usageFableTenth))),
		view: teamPremium,
	})

	// codex-empty
	tests = append(tests, &testCase{
		_id:       "codex-empty",
		_expected: "",

		sums: newUsageSums(provideUsageRecord("codex", now, withRecordCodex(), withRecordModels(usageOpusWindow))),
		view: teamPremium,
	})

	// nil-ccusage-empty
	tests = append(tests, &testCase{
		_id:       "nil-ccusage-empty",
		_expected: "",

		sums: newUsageSums(provideUsageRecord("peek-only", now, func(record *budget.Record) {
			record.Ccusage = nil
		})),
		view: teamPremium,
	})

	// no-models-empty
	tests = append(tests, &testCase{
		_id:       "no-models-empty",
		_expected: "",

		sums: newUsageSums(provideUsageRecord("no-split", now)),
		view: teamPremium,
	})

	// unknown-model-empty
	// one model without ratios drops the whole session, Opus part included
	unknownModel := budget.ModelUsage{Model: "gpt-5-codex", Usage: budget.TokenClasses{Output: 100}}
	tests = append(tests, &testCase{
		_id:       "unknown-model-empty",
		_expected: "",

		sums: newUsageSums(provideUsageRecord("unknown", now, withRecordModels(usageOpusWindow, unknownModel))),
		view: teamPremium,
	})

	// opus-under-max-20x
	// a Max 20x window is 3.17 Team Premium windows: 100% / 3.17 = 31.55%
	tests = append(tests, &testCase{
		_id:       "opus-under-max-20x",
		_expected: "32%",

		sums: newUsageSums(provideUsageRecord("opus", now, withRecordModels(usageOpusWindow))),
		view: &usageView{plan: budget.PlanMax20x},
	})

	// skill-share-of-the-window
	// the skill's run is 10% of the session by Opus ratios: 4 + 2 + 0.6 + 40 = 46.6 of 466
	skillSums := newSkillSums(provideUsageRecord("skill", now, withRecordModels(usageOpusWindow), withRecordSkillRun("fdesign", 60)), "fdesign")
	tests = append(tests, &testCase{
		_id:       "skill-share-of-the-window",
		_expected: "10%",

		sums: skillSums,
		view: teamPremium,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.planPercent(&test.sums))
		})
	}
}

func TestUsageView_PlanDetail(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		view *usageView
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, usageTestLocation)
	opusRecord := provideUsageRecord("opus", now, withRecordModels(usageOpusWindow))
	fableRecord := provideUsageRecord("fable", now, withRecordModels(usageFableWindow))
	noSplitRecord := provideUsageRecord("no-split", now.Add(-time.Hour))
	codexSplitRecord := provideUsageRecord("codex-split", now.Add(-time.Hour), withRecordCodex(), withRecordModels(usageOpusWindow))
	codexNoSplitRecord := provideUsageRecord("codex-no-split", now.Add(-2*time.Hour), withRecordCodex())
	tests := make([]*testCase, 0)

	// team-premium-window-and-weekly
	// 100% of the window, × 0.16 = 16% of the weekly limit
	tests = append(tests, &testCase{
		_id:       "team-premium-window-and-weekly",
		_expected: "100% of a Team Premium 5-hour window · 16% of the weekly limit",

		view: newLoadedUsageView(now, []*budget.Record{opusRecord}),
	})

	// max-20x-window-and-weekly
	// a Max 20x window is 3.17 Team Premium windows: 100% / 3.17 = 31.55% of the
	// window; × 10/28 = 11.27% of the weekly limit
	maxView := newLoadedUsageView(now, []*budget.Record{opusRecord})
	maxView.plan = budget.PlanMax20x
	tests = append(tests, &testCase{
		_id:       "max-20x-window-and-weekly",
		_expected: "32% of a Max 20x 5-hour window · 11% of the weekly limit",

		view: maxView,
	})

	// fable-part-of-weekly-half
	// 212.27% of the window; × 0.16 = 33.96% weekly; Fable drew all of it:
	// 33.96% / 0.5 = 67.93% of its weekly half
	tests = append(tests, &testCase{
		_id:       "fable-part-of-weekly-half",
		_expected: "212% of a Team Premium 5-hour window · 34% of the weekly limit · Fable 68% of its weekly half",

		view: newLoadedUsageView(now, []*budget.Record{fableRecord}),
	})

	// fable-under-max-20x
	// 212.27% / 3.17 = 66.97% of the window; × 10/28 = 23.92% weekly; Fable drew
	// all of it: 23.92% / 0.5 = 47.83% of its weekly half
	maxFableView := newLoadedUsageView(now, []*budget.Record{fableRecord})
	maxFableView.plan = budget.PlanMax20x
	tests = append(tests, &testCase{
		_id:       "fable-under-max-20x",
		_expected: "67% of a Max 20x 5-hour window · 24% of the weekly limit · Fable 48% of its weekly half",

		view: maxFableView,
	})

	// unweighed-claude-session-counted
	tests = append(tests, &testCase{
		_id:       "unweighed-claude-session-counted",
		_expected: "100% of a Team Premium 5-hour window · 16% of the weekly limit · 1 of 2 sessions",

		view: newLoadedUsageView(now, []*budget.Record{opusRecord, noSplitRecord}),
	})

	// codex-not-counted
	// neither the Codex split (another 100%) nor the Codex session without one
	// enters the sums or the session counts
	tests = append(tests, &testCase{
		_id:       "codex-not-counted",
		_expected: "100% of a Team Premium 5-hour window · 16% of the weekly limit",

		view: newLoadedUsageView(now, []*budget.Record{opusRecord, codexSplitRecord, codexNoSplitRecord}),
	})

	// no-weighable-session-empty
	tests = append(tests, &testCase{
		_id:       "no-weighable-session-empty",
		_expected: "",

		view: newLoadedUsageView(now, []*budget.Record{noSplitRecord, codexSplitRecord}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.view.planDetail())
		})
	}
}

func TestServer_HandleUsage(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedAbsent   []string
		_expectedCode     int
		_expectedContains string

		server *Server
		target string
	}

	tests := make([]*testCase, 0)

	// empty-store-200
	tests = append(tests, &testCase{
		_id:               "empty-store-200",
		_expectedCode:     http.StatusOK,
		_expectedContains: "No sessions in this period.",

		server: newUsageTestServer(t, nil),
		target: "/usage",
	})

	// records-rendered
	tests = append(tests, &testCase{
		_id:               "records-rendered",
		_expectedCode:     http.StatusOK,
		_expectedContains: "Session rendered-session",

		server: newUsageTestServer(t, []*budget.Record{provideUsageRecord("rendered-session", time.Now())}),
		target: "/usage",
	})

	// bad-range-400
	tests = append(tests, &testCase{
		_id:               "bad-range-400",
		_expectedCode:     http.StatusBadRequest,
		_expectedContains: "needs both from and to",

		server: newUsageTestServer(t, nil),
		target: "/usage?from=2026-09-01",
	})

	// recalculate-button-rendered
	tests = append(tests, &testCase{
		_id:               "recalculate-button-rendered",
		_expectedCode:     http.StatusOK,
		_expectedContains: `<form method="post" action="/usage/recalculate"`,

		server: newUsageTestServer(t, nil),
		target: "/usage",
	})

	// store-error-card
	brokenStateDir := t.TempDir()
	brokenRecords := filepath.Join(brokenStateDir, "usage", "records")
	require.NoError(t, os.MkdirAll(brokenRecords, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(brokenRecords, "broken.json"), []byte("{not json"), 0o600))
	brokenServer := newTestServer(t, &Options{StateDir: brokenStateDir})
	pinRoutinesHost(brokenServer)
	tests = append(tests, &testCase{
		_id:               "store-error-card",
		_expectedCode:     http.StatusOK,
		_expectedContains: `<span class="badge badge-error">usage store</span>`,

		server: brokenServer,
		target: "/usage",
	})

	// default-source-no-pair-header-no-only-chip
	singleSessionRecords := []*budget.Record{provideUsageRecord("single-source", time.Now())}
	tests = append(tests, &testCase{
		_id:               "default-source-no-pair-header-no-only-chip",
		_expectedAbsent:   []string{"peek / ccusage", "only disagreements"},
		_expectedCode:     http.StatusOK,
		_expectedContains: "Session single-source",

		server: newUsageTestServer(t, singleSessionRecords),
		target: "/usage",
	})

	// source-both-pair-header
	tests = append(tests, &testCase{
		_id:               "source-both-pair-header",
		_expectedAbsent:   nil,
		_expectedCode:     http.StatusOK,
		_expectedContains: "peek / ccusage",

		server: newUsageTestServer(t, singleSessionRecords),
		target: "/usage?source=both",
	})

	// source-both-only-chip
	tests = append(tests, &testCase{
		_id:               "source-both-only-chip",
		_expectedAbsent:   nil,
		_expectedCode:     http.StatusOK,
		_expectedContains: "only disagreements",

		server: newUsageTestServer(t, singleSessionRecords),
		target: "/usage?source=both",
	})

	// neither-source-off-card
	offServer := newUsageTestServer(t, singleSessionRecords)
	saveWelcomeRecord(t, offServer, components.ConfigServer, components.Skills)
	tests = append(tests, &testCase{
		_id:               "neither-source-off-card",
		_expectedAbsent:   []string{"Session single-source", `href="/usage?source=ccusage"`},
		_expectedCode:     http.StatusOK,
		_expectedContains: "This page needs Routines, peek, ccusage",

		server: offServer,
		target: "/usage",
	})

	// ccusage-only-default-source-no-peek-chip
	ccusageServer := newUsageTestServer(t, singleSessionRecords)
	saveWelcomeRecord(t, ccusageServer, components.ConfigServer, components.Routines, components.Ccusage)
	tests = append(tests, &testCase{
		_id:               "ccusage-only-default-source-no-peek-chip",
		_expectedAbsent:   []string{`href="/usage?source=both"`, `badge-dim" href="/usage">peek`, "peek / ccusage"},
		_expectedCode:     http.StatusOK,
		_expectedContains: `class="badge badge-ok" href="/usage?source=ccusage">ccusage</a>`,

		server: ccusageServer,
		target: "/usage",
	})

	// peek-only-no-recalculate-form
	peekServer := newUsageTestServer(t, singleSessionRecords)
	saveWelcomeRecord(t, peekServer, components.ConfigServer, components.Routines, components.Peek)
	tests = append(tests, &testCase{
		_id:               "peek-only-no-recalculate-form",
		_expectedAbsent:   []string{`action="/usage/recalculate"`, `href="/usage?source=ccusage"`},
		_expectedCode:     http.StatusOK,
		_expectedContains: "Session single-source",

		server: peekServer,
		target: "/usage",
	})

	// skill-select-all-skills
	skillRecords := []*budget.Record{provideUsageRecord("with-skill", time.Now(), func(record *budget.Record) {
		record.Peek.SkillRuns = []budget.SkillRun{{Seconds: 60, Skill: "fdesign"}}
	})}
	tests = append(tests, &testCase{
		_id:               "skill-select-all-skills",
		_expectedAbsent:   nil,
		_expectedCode:     http.StatusOK,
		_expectedContains: "all skills",

		server: newUsageTestServer(t, skillRecords),
		target: "/usage",
	})

	// skill-table-sorts-by-tokens-and-cache-read
	sortedSkillRecords := []*budget.Record{provideUsageRecord("sortable", time.Now(), withRecordSkillRun("fdesign", 60))}
	tests = append(tests, &testCase{
		_id:               "skill-table-sorts-by-tokens-and-cache-read",
		_expectedAbsent:   nil,
		_expectedCode:     http.StatusOK,
		_expectedContains: `<td data-value="53">53</td>` + "\n        " + `<td data-value="200">200</td>`,

		server: newUsageTestServer(t, sortedSkillRecords),
		target: "/usage",
	})

	// skill-filter-hides-breakdowns-and-sources
	tests = append(tests, &testCase{
		_id:               "skill-filter-hides-breakdowns-and-sources",
		_expectedAbsent:   []string{"by skill", "sessions with a stored breakdown", "context chars", `href="/usage?source=ccusage`, "peek / ccusage"},
		_expectedCode:     http.StatusOK,
		_expectedContains: "price-weighted share of their session",

		server: newUsageTestServer(t, skillRecords),
		target: "/usage?skill=fdesign&source=both",
	})

	// range-form-carries-source
	tests = append(tests, &testCase{
		_id:               "range-form-carries-source",
		_expectedAbsent:   nil,
		_expectedCode:     http.StatusOK,
		_expectedContains: `<input type="hidden" name="source" value="both">`,

		server: newUsageTestServer(t, nil),
		target: "/usage?source=both",
	})

	// four-repos-show-all
	now := time.Now()
	fourRepoRecords := []*budget.Record{
		provideUsageRecord("repo-a-session", now, withRecordRepo("repo-a")),
		provideUsageRecord("repo-b-session", now, withRecordRepo("repo-b")),
		provideUsageRecord("repo-c-session", now, withRecordRepo("repo-c")),
		provideUsageRecord("repo-d-session", now, withRecordRepo("repo-d")),
	}
	tests = append(tests, &testCase{
		_id:               "four-repos-show-all",
		_expectedAbsent:   nil,
		_expectedCode:     http.StatusOK,
		_expectedContains: "show all",

		server: newUsageTestServer(t, fourRepoRecords),
		target: "/usage",
	})

	// three-repos-no-show-all
	threeRepoRecords := []*budget.Record{
		provideUsageRecord("repo-a-session", now, withRecordRepo("repo-a")),
		provideUsageRecord("repo-b-session", now, withRecordRepo("repo-b")),
		provideUsageRecord("repo-c-session", now, withRecordRepo("repo-c")),
	}
	tests = append(tests, &testCase{
		_id:               "three-repos-no-show-all",
		_expectedAbsent:   []string{"show all"},
		_expectedCode:     http.StatusOK,
		_expectedContains: "repo-c",

		server: newUsageTestServer(t, threeRepoRecords),
		target: "/usage",
	})

	// sessions-header-names-default-plan
	tests = append(tests, &testCase{
		_id:               "sessions-header-names-default-plan",
		_expectedAbsent:   []string{"set the window capacity"},
		_expectedCode:     http.StatusOK,
		_expectedContains: `plan % of a Team Premium 5-hour window`,

		server: newUsageTestServer(t, singleSessionRecords),
		target: "/usage",
	})

	// sessions-header-names-saved-plan
	savedPlanServer := newUsageTestServer(t, singleSessionRecords)
	savedPlan := &budget.Settings{Plan: budget.PlanMax20x}
	require.NoError(t, savedPlanServer.usage.SaveSettings(savedPlan))
	tests = append(tests, &testCase{
		_id:               "sessions-header-names-saved-plan",
		_expectedAbsent:   []string{"set the window capacity", "plan % of a Team Premium"},
		_expectedCode:     http.StatusOK,
		_expectedContains: `<button type="submit" name="plan" value="max-20x" class="badge badge-ok">Max 20x</button>`,

		server: savedPlanServer,
		target: "/usage?unit=plan",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.target, nil))
			assert.Equal(t, test._expectedCode, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedContains)
			assert.Empty(t, containedStrings(recorder.Body.String(), test._expectedAbsent))
		})
	}
}

func TestServer_HandleUsageCsv(t *testing.T) {
	record := provideUsageRecord("csv", time.Now(), withRecordCost(2, 0.5), withRecordModels(usageOpusWindow))
	server := newUsageTestServer(t, []*budget.Record{record})
	settings := &budget.Settings{Plan: budget.PlanMax20x}
	require.NoError(t, server.usage.SaveSettings(settings))

	plainRequest := httptest.NewRequest(http.MethodGet, "/usage.csv", nil)
	plain := httptest.NewRecorder()
	server.Handler().ServeHTTP(plain, plainRequest)
	viewStateRequest := httptest.NewRequest(http.MethodGet, "/usage.csv?source=peek&unit=plan", nil)
	viewState := httptest.NewRecorder()
	server.Handler().ServeHTTP(viewState, viewStateRequest)

	assert.Equal(t, http.StatusOK, viewState.Code)
	assert.Equal(t, plain.Body.String(), viewState.Body.String())
	// the saved plan reaches the export: 100% / 3.17 = 31.55% of a Max 20x window
	assert.Contains(t, plain.Body.String(), ",32%,")
}
