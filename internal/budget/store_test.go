package budget

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Record(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Record
		_shouldPass bool

		id    string
		store *Store
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// missing-file-nil
	tests = append(tests, &testCase{
		_id:         "missing-file-nil",
		_expected:   nil,
		_shouldPass: true,

		id:    "session-1",
		store: NewStore(t.TempDir()),
	})

	// save-then-load
	saved := &Record{
		Id:             "session-1",
		Agent:          "claude",
		CapturedAt:     anchor,
		Ccusage:        &CcusageSide{CostUsd: 1.25, LastActivity: anchor, Usage: TokenClasses{Input: 5}},
		CcusageChecked: anchor,
		Peek:           &PeekSide{LastActive: anchor, Root: TokenClasses{Output: 7}},
		Repo:           "configs",
		Schema:         RecordSchema,
		Title:          "Usage overview",
	}
	savedStore := NewStore(t.TempDir())
	require.NoError(t, savedStore.SaveRecord(saved))
	tests = append(tests, &testCase{
		_id:         "save-then-load",
		_expected:   saved,
		_shouldPass: true,

		id:    "session-1",
		store: savedStore,
	})

	// wrong-schema-error
	wrongSchemaDir := t.TempDir()
	writeTestFile(t, filepath.Join(wrongSchemaDir, "usage", "records", "session-1.json"), `{"id":"session-1","schema":2}`)
	tests = append(tests, &testCase{
		_id:         "wrong-schema-error",
		_expected:   nil,
		_shouldPass: false,

		id:    "session-1",
		store: NewStore(wrongSchemaDir),
	})

	// malformed-json-error
	malformedDir := t.TempDir()
	writeTestFile(t, filepath.Join(malformedDir, "usage", "records", "session-1.json"), `{"id":`)
	tests = append(tests, &testCase{
		_id:         "malformed-json-error",
		_expected:   nil,
		_shouldPass: false,

		id:    "session-1",
		store: NewStore(malformedDir),
	})

	// invalid-id-refused-parent
	tests = append(tests, &testCase{
		_id:         "invalid-id-refused-parent",
		_expected:   nil,
		_shouldPass: false,

		id:    "../x",
		store: NewStore(t.TempDir()),
	})

	// invalid-id-refused-empty
	tests = append(tests, &testCase{
		_id:         "invalid-id-refused-empty",
		_expected:   nil,
		_shouldPass: false,

		id:    "",
		store: NewStore(t.TempDir()),
	})

	// invalid-id-refused-slash
	tests = append(tests, &testCase{
		_id:         "invalid-id-refused-slash",
		_expected:   nil,
		_shouldPass: false,

		id:    "a/b",
		store: NewStore(t.TempDir()),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := test.store.Record(test.id)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, record)
		})
	}
}

func TestStore_Records(t *testing.T) {
	store := NewStore(t.TempDir())
	records, err := store.Records()
	require.NoError(t, err)
	assert.Empty(t, records)

	record := &Record{Id: "session-1", Agent: "claude", Schema: RecordSchema}
	require.NoError(t, store.SaveRecord(record))
	writeTestFile(t, filepath.Join(store.dir, "records", "notes.txt"), "not a record")
	writeTestFile(t, filepath.Join(store.dir, "records", "session-2.json.tmp"), `{"id":`)

	records, err = store.Records()
	require.NoError(t, err)
	assert.Equal(t, []*Record{record}, records)
}

func TestStore_Breakdown(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir())

	missing, err := store.Breakdown("session-1")
	require.NoError(t, err)
	assert.Nil(t, missing)
	assert.True(t, store.BreakdownModTime("session-1").IsZero())

	breakdown := &Breakdown{Id: "session-1", ComputedAt: anchor, Error: "failed", LastActivity: anchor, Schema: RecordSchema}
	require.NoError(t, store.SaveBreakdown(breakdown))
	loaded, err := store.Breakdown("session-1")
	require.NoError(t, err)
	assert.Equal(t, breakdown, loaded)
	assert.False(t, store.BreakdownModTime("session-1").IsZero())

	writeTestFile(t, filepath.Join(store.dir, "breakdowns", "session-2.json"), `{"id":"session-2","schema":0}`)
	wrongSchema, err := store.Breakdown("session-2")
	require.Error(t, err)
	assert.Nil(t, wrongSchema)

	_, err = store.Breakdown("../x")
	require.Error(t, err)
	require.Error(t, store.SaveBreakdown(&Breakdown{Id: "a/b"}))
}

func TestStore_Settings(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Settings
		_shouldPass bool

		store *Store
	}

	tests := make([]*testCase, 0)

	// missing-file-default-plan
	tests = append(tests, &testCase{
		_id:         "missing-file-default-plan",
		_expected:   &Settings{Plan: DefaultPlan},
		_shouldPass: true,

		store: NewStore(t.TempDir()),
	})

	// saved-plan-read-back
	savedStore := NewStore(t.TempDir())
	savedSettings := &Settings{Plan: PlanMax20x}
	require.NoError(t, savedStore.SaveSettings(savedSettings))
	tests = append(tests, &testCase{
		_id:         "saved-plan-read-back",
		_expected:   &Settings{Plan: PlanMax20x},
		_shouldPass: true,

		store: savedStore,
	})

	// legacy-capacity-file-default-plan
	legacyDir := t.TempDir()
	writeTestFile(t, filepath.Join(legacyDir, "usage", "settings.json"), `{"window_capacity_usd":190}`)
	tests = append(tests, &testCase{
		_id:         "legacy-capacity-file-default-plan",
		_expected:   &Settings{Plan: DefaultPlan},
		_shouldPass: true,

		store: NewStore(legacyDir),
	})

	// unknown-plan-default-plan
	unknownDir := t.TempDir()
	writeTestFile(t, filepath.Join(unknownDir, "usage", "settings.json"), `{"plan":"nonsense"}`)
	tests = append(tests, &testCase{
		_id:         "unknown-plan-default-plan",
		_expected:   &Settings{Plan: DefaultPlan},
		_shouldPass: true,

		store: NewStore(unknownDir),
	})

	// broken-json-error
	brokenDir := t.TempDir()
	writeTestFile(t, filepath.Join(brokenDir, "usage", "settings.json"), `{"plan":`)
	tests = append(tests, &testCase{
		_id:         "broken-json-error",
		_expected:   nil,
		_shouldPass: false,

		store: NewStore(brokenDir),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			settings, err := test.store.Settings()
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, settings)
		})
	}
}

func TestStore_State(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir())

	state, err := store.State()
	require.NoError(t, err)
	assert.Nil(t, state)

	savedState := &State{Archived: 3, CcusageError: "ccusage down", FinishedAt: anchor, StartedAt: anchor, Taken: 2}
	require.NoError(t, store.SaveState(savedState))

	state, err = store.State()
	require.NoError(t, err)
	assert.Equal(t, savedState, state)
	assert.Equal(t, "taken 2 sessions, archived 3 files; failed: ccusage down", state.Summary())
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
