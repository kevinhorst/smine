package budget

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
)

func TestRecord_needsPeekAndCcusage(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedNeedsCcusage bool
		_expectedNeedsPeek    bool

		record  *Record
		session *peek.Session
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// missing-side
	tests = append(tests, &testCase{
		_id:                   "missing-side",
		_expectedNeedsCcusage: true,
		_expectedNeedsPeek:    true,

		record:  &Record{Id: "session-1", Schema: RecordSchema},
		session: &peek.Session{Id: "session-1", LastActive: anchor},
	})

	// session-moved
	tests = append(tests, &testCase{
		_id:                   "session-moved",
		_expectedNeedsCcusage: true,
		_expectedNeedsPeek:    true,

		record: &Record{
			Id:             "session-1",
			CcusageChecked: anchor,
			Peek:           &PeekSide{LastActive: anchor.Add(time.Hour)},
			Schema:         RecordSchema,
		},
		session: &peek.Session{Id: "session-1", LastActive: anchor.Add(2 * time.Hour)},
	})

	// unchanged
	tests = append(tests, &testCase{
		_id:                   "unchanged",
		_expectedNeedsCcusage: false,
		_expectedNeedsPeek:    false,

		record: &Record{
			Id:             "session-1",
			CcusageChecked: anchor.Add(time.Minute),
			Peek:           &PeekSide{LastActive: anchor},
			Schema:         RecordSchema,
		},
		session: &peek.Session{Id: "session-1", LastActive: anchor},
	})

	// ccusage-only-record
	tests = append(tests, &testCase{
		_id:                   "ccusage-only-record",
		_expectedNeedsCcusage: false,
		_expectedNeedsPeek:    true,

		record: &Record{
			Id:             "session-1",
			Ccusage:        &CcusageSide{LastActivity: anchor},
			CcusageChecked: anchor,
			Schema:         RecordSchema,
		},
		session: &peek.Session{Id: "session-1", LastActive: anchor},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedNeedsCcusage, test.record.needsCcusage())
			assert.Equal(t, test._expectedNeedsPeek, test.record.needsPeek(test.session))
		})
	}
}

func TestCapture_needsBreakdown(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		breakdown *Breakdown
		capture   *Capture
		session   *peek.Session
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	session := &peek.Session{Id: "session-1", LastActive: anchor}
	retryable := breakdownWithUncounted(contextrecord.UncountedRate)
	retryable.LastActivity = anchor
	tests := make([]*testCase, 0)

	// none-yet
	tests = append(tests, &testCase{
		_id:       "none-yet",
		_expected: true,

		breakdown: nil,
		capture:   &Capture{IsCounting: true},
		session:   session,
	})

	// older-than-session
	tests = append(tests, &testCase{
		_id:       "older-than-session",
		_expected: true,

		breakdown: &Breakdown{LastActivity: anchor.Add(-time.Minute)},
		capture:   &Capture{IsCounting: false},
		session:   session,
	})

	// retryable-while-counting
	tests = append(tests, &testCase{
		_id:       "retryable-while-counting",
		_expected: true,

		breakdown: retryable,
		capture:   &Capture{IsCounting: true},
		session:   session,
	})

	// retryable-counting-off
	tests = append(tests, &testCase{
		_id:       "retryable-counting-off",
		_expected: false,

		breakdown: retryable,
		capture:   &Capture{IsCounting: false},
		session:   session,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.capture.needsBreakdown(test.breakdown, test.session))
		})
	}
}

func TestCcusageSince(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedIsNeeded bool
		_expectedSince    time.Time

		records []*Record
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// no-records-all
	tests = append(tests, &testCase{
		_id:               "no-records-all",
		_expectedIsNeeded: true,
		_expectedSince:    time.Time{},

		records: nil,
	})

	// none-needed
	tests = append(tests, &testCase{
		_id:               "none-needed",
		_expectedIsNeeded: false,
		_expectedSince:    time.Time{},

		records: []*Record{
			{Id: "session-1", CcusageChecked: anchor.Add(time.Hour), Peek: &PeekSide{LastActive: anchor}},
			{Id: "session-2", Ccusage: &CcusageSide{LastActivity: anchor}, CcusageChecked: anchor},
		},
	})

	// earliest-minus-one-day
	tests = append(tests, &testCase{
		_id:               "earliest-minus-one-day",
		_expectedIsNeeded: true,
		_expectedSince:    anchor.AddDate(0, 0, 2).Local().AddDate(0, 0, -1),

		records: []*Record{
			{Id: "session-1", Peek: &PeekSide{LastActive: anchor.AddDate(0, 0, 5)}},
			{Id: "session-2", CcusageChecked: anchor, Peek: &PeekSide{LastActive: anchor.AddDate(0, 0, 2)}},
			{Id: "session-3", CcusageChecked: anchor.AddDate(0, 0, 9), Peek: &PeekSide{LastActive: anchor.AddDate(0, 0, 1)}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			since, isNeeded := ccusageSince(test.records)
			assert.Equal(t, test._expectedIsNeeded, isNeeded)
			assert.Equal(t, test._expectedSince, since)
		})
	}
}

func TestApplyCcusage(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	known := &Record{Id: "session-1", Agent: contextrecord.SessionClaude, Schema: RecordSchema}
	byId := map[string]*Record{"session-1": known}
	knownSide := &CcusageSide{CostUsd: 1, LastActivity: anchor}
	unknownSide := &CcusageSide{CostUsd: 2, LastActivity: anchor}
	sides := map[string]*CcusageSide{"session-1": knownSide, "session-2": unknownSide}

	applyCcusage(contextrecord.SessionCodex, byId, sides)

	assert.Same(t, knownSide, known.Ccusage)
	assert.Equal(t, contextrecord.SessionClaude, known.Agent)
	expectedNew := &Record{Id: "session-2", Agent: contextrecord.SessionCodex, Ccusage: unknownSide, Schema: RecordSchema}
	assert.Equal(t, expectedNew, byId["session-2"])
}

func TestCapture_foldBreakdown(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedHasContext bool
		_expectedIsFolded   bool
		_shouldPass         bool

		capture *Capture
		record  *Record
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// no-breakdown-file
	tests = append(tests, &testCase{
		_id:                 "no-breakdown-file",
		_expectedHasContext: false,
		_expectedIsFolded:   false,
		_shouldPass:         true,

		capture: &Capture{Store: NewStore(t.TempDir())},
		record:  &Record{Id: "session-1", Schema: RecordSchema},
	})

	// newer-breakdown-folded
	foldedStore := NewStore(t.TempDir())
	foldedBreakdown := breakdownWithUncounted("")
	foldedBreakdown.Id = "session-1"
	require.NoError(t, foldedStore.SaveBreakdown(foldedBreakdown))
	tests = append(tests, &testCase{
		_id:                 "newer-breakdown-folded",
		_expectedHasContext: true,
		_expectedIsFolded:   true,
		_shouldPass:         true,

		capture: &Capture{Store: foldedStore},
		record:  &Record{Id: "session-1", Context: &ContextSummary{Source: anchor}, Schema: RecordSchema},
	})

	// summary-current-not-folded
	currentStore := NewStore(t.TempDir())
	currentBreakdown := breakdownWithUncounted("")
	currentBreakdown.Id = "session-1"
	require.NoError(t, currentStore.SaveBreakdown(currentBreakdown))
	tests = append(tests, &testCase{
		_id:                 "summary-current-not-folded",
		_expectedHasContext: true,
		_expectedIsFolded:   false,
		_shouldPass:         true,

		capture: &Capture{Store: currentStore},
		record: &Record{
			Id:      "session-1",
			Context: &ContextSummary{Source: currentStore.BreakdownModTime("session-1")},
			Schema:  RecordSchema,
		},
	})

	// error-breakdown-not-folded
	errorStore := NewStore(t.TempDir())
	errorBreakdown := &Breakdown{Id: "session-1", Error: "ExtractRecord: No transcript on disk", LastActivity: anchor, Schema: RecordSchema}
	require.NoError(t, errorStore.SaveBreakdown(errorBreakdown))
	tests = append(tests, &testCase{
		_id:                 "error-breakdown-not-folded",
		_expectedHasContext: false,
		_expectedIsFolded:   false,
		_shouldPass:         true,

		capture: &Capture{Store: errorStore},
		record:  &Record{Id: "session-1", Schema: RecordSchema},
	})

	// wrong-schema-error
	wrongSchemaStore := NewStore(t.TempDir())
	writeTestFile(t, filepath.Join(wrongSchemaStore.dir, "breakdowns", "session-1.json"), `{"id":"session-1","schema":9}`)
	tests = append(tests, &testCase{
		_id:                 "wrong-schema-error",
		_expectedHasContext: false,
		_expectedIsFolded:   false,
		_shouldPass:         false,

		capture: &Capture{Store: wrongSchemaStore},
		record:  &Record{Id: "session-1", Schema: RecordSchema},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isFolded, err := test.capture.foldBreakdown(test.record)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedIsFolded, isFolded)
			assert.Equal(t, test._expectedHasContext, test.record.Context != nil)
		})
	}
}

func TestExtractRecord(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *contextrecord.Record
		_shouldPass bool

		events  *peek.SessionEvents
		session *peek.Session
		turns   []peek.Turn
	}

	claudeHome := t.TempDir()
	events := &peek.SessionEvents{TouchedFiles: []peek.TouchedFile{{Path: "/src/main.go", Reads: 2, Writes: 1}}}
	touches := []contextrecord.FileTouch{{Path: "/src/main.go", Reads: 2, Writes: 1}}
	turns := []peek.Turn{
		{Role: "user", Text: "first prompt"},
		{Role: "assistant", Text: "answer"},
		{Role: "user", Text: "second prompt"},
	}
	tests := make([]*testCase, 0)

	// claude-no-transcript-error
	tests = append(tests, &testCase{
		_id:         "claude-no-transcript-error",
		_expected:   nil,
		_shouldPass: false,

		events:  nil,
		session: &peek.Session{Id: "session-1", Agent: contextrecord.SessionClaude},
		turns:   nil,
	})

	// codex-from-turns
	tests = append(tests, &testCase{
		_id:         "codex-from-turns",
		_expected:   contextrecord.ExtractCodex("session-2", "gpt-6-astra", []string{"first prompt", "second prompt"}, touches),
		_shouldPass: true,

		events:  events,
		session: &peek.Session{Id: "session-2", Agent: contextrecord.SessionCodex, Model: "gpt-6-astra"},
		turns:   turns,
	})

	// unknown-agent-error
	tests = append(tests, &testCase{
		_id:         "unknown-agent-error",
		_expected:   nil,
		_shouldPass: false,

		events:  events,
		session: &peek.Session{Id: "session-3", Agent: "gemini"},
		turns:   turns,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := ExtractRecord(claudeHome, "", test.events, test.session, test.turns)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, record)
		})
	}
}
