package budget

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveSides(t *testing.T) {
	checkedAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir())
	// stale: checked long ago, no model split — the record a capture run
	// never re-reads
	stale := &Record{
		Id:             "stale",
		Agent:          contextrecord.SessionClaude,
		Ccusage:        &CcusageSide{CostUsd: 1, Usage: TokenClasses{Output: 10}},
		CcusageChecked: checkedAt.Add(-time.Hour),
		Schema:         RecordSchema,
	}
	// untouched: ccusage no longer returns it
	untouched := &Record{
		Id:             "untouched",
		Agent:          contextrecord.SessionClaude,
		Ccusage:        &CcusageSide{CostUsd: 3},
		CcusageChecked: checkedAt.Add(-time.Hour),
		Schema:         RecordSchema,
	}
	require.NoError(t, store.SaveRecord(stale))
	require.NoError(t, store.SaveRecord(untouched))
	recalculated := &CcusageSide{
		CostUsd: 2,
		Models:  []ModelUsage{{Model: "claude-opus-5-5", Usage: TokenClasses{Output: 10}}},
		Usage:   TokenClasses{Output: 10},
	}
	claude := map[string]*CcusageSide{"stale": recalculated, "only-ccusage": {CostUsd: 4}}
	codex := map[string]*CcusageSide{"codex-session": {CostUsd: 5}}

	saved, err := saveSides(checkedAt, claude, codex, []*Record{stale, untouched}, store)
	require.NoError(t, err)

	staleStored, err := store.Record("stale")
	require.NoError(t, err)
	untouchedStored, err := store.Record("untouched")
	require.NoError(t, err)
	createdStored, err := store.Record("only-ccusage")
	require.NoError(t, err)
	codexStored, err := store.Record("codex-session")
	require.NoError(t, err)

	assert.Equal(t, 3, saved)
	assert.Equal(t, recalculated, staleStored.Ccusage)
	assert.True(t, checkedAt.Equal(staleStored.CcusageChecked))
	assert.InDelta(t, 3, untouchedStored.Ccusage.CostUsd, 0)
	assert.True(t, checkedAt.Add(-time.Hour).Equal(untouchedStored.CcusageChecked))
	assert.Equal(t, contextrecord.SessionClaude, createdStored.Agent)
	assert.Equal(t, contextrecord.SessionCodex, codexStored.Agent)
}

func TestSaveSides_NothingReturned(t *testing.T) {
	store := NewStore(t.TempDir())
	record := &Record{Id: "kept", Agent: contextrecord.SessionClaude, Ccusage: &CcusageSide{CostUsd: 1}, Schema: RecordSchema}
	require.NoError(t, store.SaveRecord(record))

	saved, err := saveSides(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), nil, nil, []*Record{record}, store)

	require.NoError(t, err)
	assert.Equal(t, 0, saved)
}

func TestCcusage_RecalculateMissingBinary(t *testing.T) {
	store := NewStore(t.TempDir())
	ccusage := &Ccusage{ArchiveDir: store.ArchiveDir(), BinaryPath: filepath.Join(t.TempDir(), "no-ccusage")}

	saved, err := ccusage.Recalculate(context.Background(), store)

	require.Error(t, err)
	assert.Equal(t, 0, saved)
}

func TestDecodeClaudeSessions(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *CcusageSide
		_shouldPass bool
		_sides      int

		id     string
		output []byte
	}

	report, err := os.ReadFile("testdata/ccusage-claude.json")
	require.NoError(t, err)
	parentCost := 63.74485944999999
	firstWorkflowCost := 10.209121200000002
	secondWorkflowCost := 7.4226265999999965
	tests := make([]*testCase, 0)

	// claude-report
	tests = append(tests, &testCase{
		_id: "claude-report",
		_expected: &CcusageSide{
			CostUsd:       691.7426324000008,
			FirstActivity: time.Date(2026, 10, 2, 1, 50, 55, 568000000, time.UTC),
			LastActivity:  time.Date(2026, 10, 3, 0, 16, 51, 186000000, time.UTC),
			Models: []ModelUsage{
				{Model: "claude-fable-5-1", Usage: TokenClasses{CacheCreation: 7006320, CacheRead: 558982396, Input: 3932, Output: 1049724}},
				{Model: "claude-opus-5-5", Usage: TokenClasses{CacheCreation: 17023851, CacheRead: 794543442, Input: 7410, Output: 3028430}},
				{Model: "claude-sonnet-5-5", Usage: TokenClasses{CacheCreation: 629577, CacheRead: 4104425, Input: 112, Output: 30796}},
			},
			Usage: TokenClasses{CacheCreation: 24659748, CacheRead: 1357630263, Input: 11454, Output: 4108950},
		},
		_shouldPass: true,
		_sides:      2,

		id:     "e8d99d86-d43e-4a75-93d8-1f966ee54139",
		output: report,
	})

	// workflow-rows-folded-into-parent
	tests = append(tests, &testCase{
		_id: "workflow-rows-folded-into-parent",
		_expected: &CcusageSide{
			CostUsd:       parentCost + firstWorkflowCost + secondWorkflowCost,
			FirstActivity: time.Date(2026, 10, 3, 12, 0, 20, 652000000, time.UTC),
			LastActivity:  time.Date(2026, 10, 3, 14, 19, 13, 948000000, time.UTC),
			Models: []ModelUsage{
				{Model: "claude-fable-5-1", Usage: TokenClasses{CacheCreation: 767195, CacheRead: 134365673, Input: 471, Output: 282186}},
				{
					Model: "claude-opus-5-5",
					Usage: TokenClasses{
						CacheCreation: 72363 + 565375 + 445248,
						CacheRead:     197476 + 19685326 + 14823573,
						Input:         8 + 294 + 222,
						Output:        3855 + 87394 + 44752,
					},
				},
			},
			Usage: TokenClasses{
				CacheCreation: 839558 + 565375 + 445248,
				CacheRead:     134563149 + 19685326 + 14823573,
				Input:         479 + 294 + 222,
				Output:        286041 + 87394 + 44752,
			},
			WorkflowRows: 2,
		},
		_shouldPass: true,
		_sides:      2,

		id:     "a46baac0-dfee-436f-ba1d-247f5aafe552",
		output: report,
	})

	// workflow-row-without-suffix-kept
	tests = append(tests, &testCase{
		_id: "workflow-row-without-suffix-kept",
		_expected: &CcusageSide{
			CostUsd:       2.5,
			FirstActivity: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
			LastActivity:  time.Date(2026, 10, 3, 13, 0, 0, 0, time.UTC),
			Usage:         TokenClasses{Input: 10, Output: 20},
			WorkflowRows:  1,
		},
		_shouldPass: true,
		_sides:      1,

		id:     "wf_f3260aff-aff",
		output: []byte(`{"sessions":[{"sessionId":"wf_f3260aff-aff","projectPath":"-src-repo/a46baac0","inputTokens":10,"outputTokens":20,"cacheCreationTokens":0,"cacheReadTokens":0,"totalCost":2.5,"firstActivity":"2026-10-03T12:00:00Z","lastActivity":"2026-10-03T13:00:00Z"}]}`),
	})

	// workflow-model-merged-and-sorted
	tests = append(tests, &testCase{
		_id: "workflow-model-merged-and-sorted",
		_expected: &CcusageSide{
			CostUsd:       3,
			FirstActivity: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
			LastActivity:  time.Date(2026, 10, 3, 14, 0, 0, 0, time.UTC),
			Models: []ModelUsage{
				{Model: "claude-haiku-4-5-20251001", Usage: TokenClasses{Input: 3, Output: 4}},
				{Model: "claude-sonnet-5-5", Usage: TokenClasses{CacheRead: 100 + 50, Input: 10 + 1, Output: 20 + 2}},
			},
			Usage:        TokenClasses{CacheRead: 150, Input: 14, Output: 26},
			WorkflowRows: 1,
		},
		_shouldPass: true,
		_sides:      1,

		id:     "session-1",
		output: []byte(`{"sessions":[{"sessionId":"session-1","projectPath":"-src-repo","inputTokens":10,"outputTokens":20,"cacheCreationTokens":0,"cacheReadTokens":100,"totalCost":2,"firstActivity":"2026-10-03T12:00:00Z","lastActivity":"2026-10-03T13:00:00Z","modelBreakdowns":[{"modelName":"claude-sonnet-5-5","inputTokens":10,"outputTokens":20,"cacheCreationTokens":0,"cacheReadTokens":100}]},{"sessionId":"wf_1","projectPath":"-src-repo/session-1/subagents/workflows","inputTokens":4,"outputTokens":6,"cacheCreationTokens":0,"cacheReadTokens":50,"totalCost":1,"firstActivity":"2026-10-03T12:30:00Z","lastActivity":"2026-10-03T14:00:00Z","modelBreakdowns":[{"modelName":"claude-sonnet-5-5","inputTokens":1,"outputTokens":2,"cacheCreationTokens":0,"cacheReadTokens":50},{"modelName":"claude-haiku-4-5-20251001","inputTokens":3,"outputTokens":4,"cacheCreationTokens":0,"cacheReadTokens":0}]}]}`),
	})

	// no-model-breakdowns-nil-models
	tests = append(tests, &testCase{
		_id: "no-model-breakdowns-nil-models",
		_expected: &CcusageSide{
			CostUsd:       1.5,
			FirstActivity: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
			LastActivity:  time.Date(2026, 10, 3, 13, 0, 0, 0, time.UTC),
			Models:        nil,
			Usage:         TokenClasses{Input: 10, Output: 20},
		},
		_shouldPass: true,
		_sides:      1,

		id:     "session-1",
		output: []byte(`{"sessions":[{"sessionId":"session-1","projectPath":"-src-repo","inputTokens":10,"outputTokens":20,"cacheCreationTokens":0,"cacheReadTokens":0,"totalCost":1.5,"firstActivity":"2026-10-03T12:00:00Z","lastActivity":"2026-10-03T13:00:00Z"}]}`),
	})

	// unpriced-marked
	tests = append(tests, &testCase{
		_id: "unpriced-marked",
		_expected: &CcusageSide{
			FirstActivity: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
			IsUnpriced:    true,
			LastActivity:  time.Date(2026, 10, 3, 13, 0, 0, 0, time.UTC),
			Usage:         TokenClasses{Input: 10, Output: 20},
		},
		_shouldPass: true,
		_sides:      1,

		id:     "session-1",
		output: []byte(`{"sessions":[{"sessionId":"session-1","projectPath":"-src-repo","inputTokens":10,"outputTokens":20,"cacheCreationTokens":0,"cacheReadTokens":0,"totalCost":0,"firstActivity":"2026-10-03T12:00:00Z","lastActivity":"2026-10-03T13:00:00Z"}]}`),
	})

	// malformed-json
	tests = append(tests, &testCase{
		_id:         "malformed-json",
		_expected:   nil,
		_shouldPass: false,
		_sides:      0,

		id:     "session-1",
		output: []byte(`{"sessions":[`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			sides, err := decodeClaudeSessions(test.output)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Len(t, sides, test._sides)
			assert.Equal(t, test._expected, sides[test.id])
		})
	}
}

func TestDecodeCodexSessions(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *CcusageSide
		_shouldPass bool
		_sides      int

		id     string
		output []byte
	}

	report, err := os.ReadFile("testdata/ccusage-codex.json")
	require.NoError(t, err)
	tests := make([]*testCase, 0)

	// codex-uuid-from-rollout
	tests = append(tests, &testCase{
		_id: "codex-uuid-from-rollout",
		_expected: &CcusageSide{
			CostUsd:      0.6951980000000001,
			LastActivity: time.Date(2026, 9, 30, 21, 57, 3, 869000000, time.UTC),
			Models:       nil,
			Usage:        TokenClasses{CacheRead: 132608, Input: 52404, Output: 771},
		},
		_shouldPass: true,
		_sides:      2,

		id:     "01a0f451-b1d9-7a30-a996-08dae8ffed07",
		output: report,
	})

	// short-session-file-keeps-session-id
	tests = append(tests, &testCase{
		_id: "short-session-file-keeps-session-id",
		_expected: &CcusageSide{
			IsUnpriced:   true,
			LastActivity: time.Date(2026, 10, 3, 13, 0, 0, 0, time.UTC),
			Usage:        TokenClasses{Input: 4},
		},
		_shouldPass: true,
		_sides:      1,

		id:     "2026/10/03/rollout-x",
		output: []byte(`{"sessions":[{"sessionId":"2026/10/03/rollout-x","sessionFile":"rollout-x","inputTokens":4,"outputTokens":0,"cacheCreationTokens":0,"cacheReadTokens":0,"costUSD":0,"lastActivity":"2026-10-03T13:00:00Z"}]}`),
	})

	// malformed-json
	tests = append(tests, &testCase{
		_id:         "malformed-json",
		_expected:   nil,
		_shouldPass: false,
		_sides:      0,

		id:     "01a0f451-b1d9-7a30-a996-08dae8ffed07",
		output: []byte(`[`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			sides, err := decodeCodexSessions(test.output)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Len(t, sides, test._sides)
			assert.Equal(t, test._expected, sides[test.id])
		})
	}
}

func TestParseCcusageVersion(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedIsOlder bool
		_expectedOk      bool

		output string
	}

	tests := make([]*testCase, 0)

	// 20.0.20
	tests = append(tests, &testCase{
		_id:              "20.0.20",
		_expectedIsOlder: false,
		_expectedOk:      true,

		output: "ccusage 20.0.20\n",
	})

	// 20.0.19-older
	tests = append(tests, &testCase{
		_id:              "20.0.19-older",
		_expectedIsOlder: true,
		_expectedOk:      true,

		output: "ccusage 20.0.19\n",
	})

	// 21.0.0
	tests = append(tests, &testCase{
		_id:              "21.0.0",
		_expectedIsOlder: false,
		_expectedOk:      true,

		output: "21.0.0",
	})

	// garbage
	tests = append(tests, &testCase{
		_id:              "garbage",
		_expectedIsOlder: true,
		_expectedOk:      false,

		output: "ccusage v-next",
	})

	// empty
	tests = append(tests, &testCase{
		_id:              "empty",
		_expectedIsOlder: true,
		_expectedOk:      false,

		output: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			version, ok := parseCcusageVersion(test.output)
			assert.Equal(t, test._expectedOk, ok)
			assert.Equal(t, test._expectedIsOlder, isOlderVersion(version, ccusageMinVersion))
		})
	}
}

func TestSessionArgs(t *testing.T) {
	since := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, []string{"claude", "session", "--json", "--since", "20000101"}, sessionArgs("claude", since))
	assert.Equal(t, []string{"codex", "session", "--json"}, sessionArgs("codex", time.Time{}))
}
