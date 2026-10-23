package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

// newTestEnv is an Env over temp homes and a temp checkout; HOME points at
// the temp home so every helper resolving the home stays inside it.
func newTestEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	repo := t.TempDir()
	t.Setenv("HOME", home)

	env := Env{
		ClaudeHome:  filepath.Join(home, ".claude"),
		GOOS:        goosDarwin,
		Home:        home,
		IsPayload:   true,
		MainRepoDir: repo,
		RepoDir:     repo,
		Stdout:      io.Discard,
		Trash:       trash.New(filepath.Join(home, ".claude", "smine"), 0),
		Values: Values{
			ConfigServerPort: "6001",
			CountTokens:      "off",
			HeartbeatPort:    "6101",
			InitWelcome:      false,
			PeekControlPort:  "42442",
			PeekPort:         "4242",
		},
	}
	return env
}

// writeTestFile writes content below root, creating its dirs.
func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// withStoredBase returns record carrying the stored record's updated stamp, the base Save compares against.
func withStoredBase(t *testing.T, env Env, record Record) Record {
	t.Helper()
	stored, _, err := Load(env)
	require.NoError(t, err)
	record.Updated = stored.Updated
	return record
}

func TestValidate(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		goos   string
		record Record
	}

	tests := make([]*testCase, 0)

	// pass-full-darwin
	tests = append(tests, &testCase{
		_id:         "pass-full-darwin",
		_shouldPass: true,

		goos:   goosDarwin,
		record: FullRecord(goosDarwin),
	})

	// fail-missing-config-server
	record := FullRecord(goosDarwin)
	record.Components = record.Components[1:]
	tests = append(tests, &testCase{
		_id:         "fail-missing-config-server",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-unknown-id
	record = FullRecord(goosDarwin)
	record.Components = append(record.Components, "nope")
	tests = append(tests, &testCase{
		_id:         "fail-unknown-id",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-heartbeat-on-linux
	record = FullRecord("linux")
	record.Components = append(record.Components, components.Heartbeat)
	tests = append(tests, &testCase{
		_id:         "fail-heartbeat-on-linux",
		_shouldPass: false,

		goos:   "linux",
		record: record,
	})

	// fail-context-without-session-hooks
	record = FullRecord(goosDarwin)
	record.Preset = PresetCustom
	record.Components = []components.ID{components.ConfigServer, components.Context}
	tests = append(tests, &testCase{
		_id:         "fail-context-without-session-hooks",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-duplicate-id
	record = FullRecord(goosDarwin)
	record.Components = append(record.Components, components.Skills)
	tests = append(tests, &testCase{
		_id:         "fail-duplicate-id",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-mode-chosen-empty
	record = FullRecord(goosDarwin)
	record.Skills = Selection{Mode: skills.SelectionModeChosen}
	tests = append(tests, &testCase{
		_id:         "fail-mode-chosen-empty",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-mode-all-with-chosen
	record = FullRecord(goosDarwin)
	record.Skills = Selection{Chosen: []string{"jq"}, Mode: skills.SelectionModeAll}
	tests = append(tests, &testCase{
		_id:         "fail-mode-all-with-chosen",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-unknown-preset
	record = FullRecord(goosDarwin)
	record.Preset = "huge"
	tests = append(tests, &testCase{
		_id:         "fail-unknown-preset",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// fail-schema-1
	record = FullRecord(goosDarwin)
	record.Schema = 1
	tests = append(tests, &testCase{
		_id:         "fail-schema-1",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// pass-schema-2-with-settings
	record = FullRecord(goosDarwin)
	record.Settings = Settings{components.ConfigServer: {"port": json.RawMessage(`6002`)}}
	record.Skills.Variables = map[string]string{"SKILL_FIMPLEMENT_CHECK": "make audit"}
	tests = append(tests, &testCase{
		_id:         "pass-schema-2-with-settings",
		_shouldPass: true,

		goos:   goosDarwin,
		record: record,
	})

	// pass-unselected-component-setting-kept
	record = FullRecord(goosDarwin)
	record.Preset = PresetMinimal
	record.Components = components.Minimal()
	record.Settings = Settings{components.Heartbeat: {"port": json.RawMessage(`6102`)}}
	tests = append(tests, &testCase{
		_id:         "pass-unselected-component-setting-kept",
		_shouldPass: true,

		goos:   goosDarwin,
		record: record,
	})

	// pass-unknown-setting-component-noted
	record = FullRecord(goosDarwin)
	record.Settings = Settings{"nope": {"port": json.RawMessage(`6002`)}}
	tests = append(tests, &testCase{
		_id:         "pass-unknown-setting-component-noted",
		_shouldPass: true,

		goos:   goosDarwin,
		record: record,
	})

	// pass-unknown-setting-key-noted
	record = FullRecord(goosDarwin)
	record.Settings = Settings{components.ConfigServer: {"nope": json.RawMessage(`6002`)}}
	tests = append(tests, &testCase{
		_id:         "pass-unknown-setting-key-noted",
		_shouldPass: true,

		goos:   goosDarwin,
		record: record,
	})

	// fail-setting-wrong-kind
	record = FullRecord(goosDarwin)
	record.Settings = Settings{components.ConfigServer: {"port": json.RawMessage(`"web"`)}}
	tests = append(tests, &testCase{
		_id:         "fail-setting-wrong-kind",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// pass-group-override
	record = FullRecord(goosDarwin)
	record.Settings = Settings{components.SessionHelpers: {"groups.session-helpers-stop-play-sound.timeout": json.RawMessage(`9`)}}
	tests = append(tests, &testCase{
		_id:         "pass-group-override",
		_shouldPass: true,

		goos:   goosDarwin,
		record: record,
	})

	// fail-group-override-wrong-kind
	record = FullRecord(goosDarwin)
	record.Settings = Settings{components.SessionHelpers: {"groups.session-helpers-stop-play-sound.async": json.RawMessage(`"yes"`)}}
	tests = append(tests, &testCase{
		_id:         "fail-group-override-wrong-kind",
		_shouldPass: false,

		goos:   goosDarwin,
		record: record,
	})

	// pass-group-override-unknown-field-noted
	record = FullRecord(goosDarwin)
	record.Settings = Settings{components.SessionHelpers: {"groups.session-helpers-stop-play-sound.command": json.RawMessage(`"x"`)}}
	tests = append(tests, &testCase{
		_id:         "pass-group-override-unknown-field-noted",
		_shouldPass: true,

		goos:   goosDarwin,
		record: record,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := Validate(test.record, test.goos)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestLoad(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   Record
		_isFound    bool
		_shouldPass bool

		content string
	}

	tests := make([]*testCase, 0)

	// missing-file-full-not-found
	tests = append(tests, &testCase{
		_id:         "missing-file-full-not-found",
		_expected:   FullRecord(goosDarwin),
		_isFound:    false,
		_shouldPass: true,

		content: "",
	})

	// valid-file-found
	minimal := Record{Components: components.Minimal(), Preset: PresetMinimal, Schema: 2, Skills: Selection{Mode: skills.SelectionModeNone}}
	tests = append(tests, &testCase{
		_id:         "valid-file-found",
		_expected:   minimal,
		_isFound:    true,
		_shouldPass: true,

		content: `{"schema": 2, "preset": "minimal", "components": ["config-server", "skills"], "skills": {"mode": "none"}, "updated": "0001-01-01T00:00:00Z"}`,
	})

	// retired-component-kept-in-retired
	retired := minimal
	retired.Retired = []components.ID{"ide-mcp"}
	tests = append(tests, &testCase{
		_id:         "retired-component-kept-in-retired",
		_expected:   retired,
		_isFound:    true,
		_shouldPass: true,

		content: `{"schema": 2, "preset": "minimal", "components": ["config-server", "skills", "ide-mcp"], "skills": {"mode": "none"}, "updated": "0001-01-01T00:00:00Z"}`,
	})

	// unknown-setting-key-kept-and-noted
	unknownKey := minimal
	unknownKey.Settings = Settings{components.ConfigServer: {"nope": json.RawMessage(`6002`)}}
	tests = append(tests, &testCase{
		_id:         "unknown-setting-key-kept-and-noted",
		_expected:   unknownKey,
		_isFound:    true,
		_shouldPass: true,

		content: `{"schema": 2, "preset": "minimal", "components": ["config-server", "skills"], "settings": {"config-server": {"nope": 6002}}, "skills": {"mode": "none"}, "updated": "0001-01-01T00:00:00Z"}`,
	})

	// schema-1-error-with-record
	legacy := minimal
	legacy.Schema = 1
	tests = append(tests, &testCase{
		_id:         "schema-1-error-with-record",
		_expected:   legacy,
		_isFound:    true,
		_shouldPass: false,

		content: `{"schema": 1, "preset": "minimal", "components": ["config-server", "skills"], "skills": {"mode": "none"}, "updated": "0001-01-01T00:00:00Z"}`,
	})

	// invalid-json-error
	tests = append(tests, &testCase{
		_id:         "invalid-json-error",
		_expected:   Record{},
		_isFound:    true,
		_shouldPass: false,

		content: `{"schema": `,
	})

	// invalid-record-error-with-record
	invalid := Record{Components: []components.ID{components.Skills}, Preset: PresetCustom, Schema: 2, Skills: Selection{Mode: skills.SelectionModeAll}}
	tests = append(tests, &testCase{
		_id:         "invalid-record-error-with-record",
		_expected:   invalid,
		_isFound:    true,
		_shouldPass: false,

		content: `{"schema": 2, "preset": "custom", "components": ["skills"], "skills": {"mode": "all"}, "updated": "0001-01-01T00:00:00Z"}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			env := newTestEnv(t)
			if test.content != "" {
				writeTestFile(t, env.ClaudeHome, "smine/install.json", test.content)
			}

			record, isFound, err := Load(env)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isFound, isFound)
			assert.Equal(t, test._expected, record)
		})
	}
}

func TestSave(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedComponents []components.ID
		_isWritten          bool
		_shouldPass         bool

		goos   string
		record Record
	}

	tests := make([]*testCase, 0)

	// minimal-expanded
	tests = append(tests, &testCase{
		_id:                 "minimal-expanded",
		_expectedComponents: components.Minimal(),
		_isWritten:          true,
		_shouldPass:         true,

		goos:   goosDarwin,
		record: Record{Preset: PresetMinimal, Schema: 2, Skills: Selection{Mode: skills.SelectionModeAll}},
	})

	// full-expanded-per-goos
	tests = append(tests, &testCase{
		_id:                 "full-expanded-per-goos",
		_expectedComponents: components.Full("linux"),
		_isWritten:          true,
		_shouldPass:         true,

		goos:   "linux",
		record: Record{Preset: PresetFull, Schema: 2, Skills: Selection{Mode: skills.SelectionModeAll}},
	})

	// invalid-refused-no-file
	tests = append(tests, &testCase{
		_id:                 "invalid-refused-no-file",
		_expectedComponents: components.Full(goosDarwin),
		_isWritten:          false,
		_shouldPass:         false,

		goos:   goosDarwin,
		record: Record{Components: []components.ID{components.Skills}, Preset: PresetCustom, Schema: 2, Skills: Selection{Mode: skills.SelectionModeAll}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			env := newTestEnv(t)
			env.GOOS = test.goos

			err := Save(env, test.record)
			_, statErr := os.Stat(RecordPath(env))
			saved, _, _ := Load(env)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isWritten, statErr == nil)
			assert.Equal(t, test._expectedComponents, saved.Components)
		})
	}
}

func TestSave_UpdatedStamped(t *testing.T) {
	env := newTestEnv(t)
	synctest.Test(t, func(t *testing.T) {
		require.NoError(t, Save(env, FullRecord(goosDarwin)))
		saved, isFound, err := Load(env)
		require.NoError(t, err)
		assert.True(t, isFound)
		assert.Equal(t, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), saved.Updated)
	})
}

func TestSave_History(t *testing.T) {
	env := newTestEnv(t)
	path := RecordPath(env)
	require.NoError(t, Save(env, FullRecord(goosDarwin)))
	firstBytes := readTestFile(t, path)

	// one-save-one-history-file
	require.NoError(t, Save(env, withStoredBase(t, env, FullRecord(goosDarwin))))
	history, err := History(path)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, firstBytes, readTestFile(t, history[0]))
	oldest := history[0]

	// fifty-two-saves-keep-fifty
	for range 49 {
		require.NoError(t, Save(env, withStoredBase(t, env, FullRecord(goosDarwin))))
	}
	beforeLast := readTestFile(t, path)
	require.NoError(t, Save(env, withStoredBase(t, env, FullRecord(goosDarwin))))
	history, err = History(path)
	require.NoError(t, err)
	assert.Len(t, history, historyKeep)
	assert.NotContains(t, history, oldest)
	assert.NoFileExists(t, oldest)

	// newest-first
	assert.Equal(t, beforeLast, readTestFile(t, history[0]))

	// pending-record-not-copied
	pending := withStoredBase(t, env, FullRecord(goosDarwin))
	pending.Applied = false
	require.NoError(t, Save(env, pending))
	historyBefore, err := History(path)
	require.NoError(t, err)
	require.NoError(t, Save(env, withStoredBase(t, env, FullRecord(goosDarwin))))
	history, err = History(path)
	require.NoError(t, err)
	assert.Equal(t, historyBefore[0], history[0])
}

func TestIsPendingApply(t *testing.T) {
	assert.True(t, isPendingApply([]byte(`{"applied": false}`)), "explicit-false")
	assert.False(t, isPendingApply([]byte(`{"applied": true}`)), "true")
	assert.False(t, isPendingApply([]byte(`{"schema": 2}`)), "key-absent-predates-the-flag")
	assert.False(t, isPendingApply([]byte(`not json`)), "unparsable")
}

func TestSave_CompareAndSwap(t *testing.T) {
	type testCase struct {
		_id              string
		_isRecordChanged bool
		_isUntouched     bool
		_shouldPass      bool

		before []byte
		env    Env
		record Record
	}

	tests := make([]*testCase, 0)

	// base-equals-file-saved
	env := newTestEnv(t)
	require.NoError(t, Save(env, FullRecord(goosDarwin)))
	tests = append(tests, &testCase{
		_id:              "base-equals-file-saved",
		_isRecordChanged: false,
		_isUntouched:     false,
		_shouldPass:      true,

		before: readTestFile(t, RecordPath(env)),
		env:    env,
		record: withStoredBase(t, env, FullRecord(goosDarwin)),
	})

	// stale-base-refused
	env = newTestEnv(t)
	require.NoError(t, Save(env, FullRecord(goosDarwin)))
	stale := withStoredBase(t, env, FullRecord(goosDarwin))
	require.NoError(t, Save(env, stale))
	tests = append(tests, &testCase{
		_id:              "stale-base-refused",
		_isRecordChanged: true,
		_isUntouched:     true,
		_shouldPass:      false,

		before: readTestFile(t, RecordPath(env)),
		env:    env,
		record: stale,
	})

	// zero-base-file-absent-saved
	env = newTestEnv(t)
	tests = append(tests, &testCase{
		_id:              "zero-base-file-absent-saved",
		_isRecordChanged: false,
		_isUntouched:     false,
		_shouldPass:      true,

		before: nil,
		env:    env,
		record: FullRecord(goosDarwin),
	})

	// zero-base-file-present-refused
	env = newTestEnv(t)
	require.NoError(t, Save(env, FullRecord(goosDarwin)))
	tests = append(tests, &testCase{
		_id:              "zero-base-file-present-refused",
		_isRecordChanged: true,
		_isUntouched:     true,
		_shouldPass:      false,

		before: readTestFile(t, RecordPath(env)),
		env:    env,
		record: FullRecord(goosDarwin),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := Save(test.env, test.record)
			after, _ := os.ReadFile(RecordPath(test.env))
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isRecordChanged, errors.Is(err, ErrRecordChanged))
			assert.Equal(t, test._isUntouched, bytes.Equal(test.before, after))
		})
	}
}

func TestNotes(t *testing.T) {
	// retired-and-unknown-listed
	record := FullRecord(goosDarwin)
	record.Retired = []components.ID{"ide-mcp"}
	record.Settings = Settings{
		"nope":                  {"port": json.RawMessage(`6002`)},
		components.ConfigServer: {"typo": json.RawMessage(`1`)},
	}
	expected := []string{
		"component ide-mcp: no longer in the catalog, kept",
		"settings nope: unknown component, kept",
		"settings config-server.typo: unknown key, kept",
	}
	assert.ElementsMatch(t, expected, Notes(record))

	// clean-record-none
	assert.Empty(t, Notes(FullRecord(goosDarwin)))
}

func TestSkillRows(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedDefaults map[string]string
		_expectedStored   map[string]string

		rows *skills.Rows
	}

	tests := make([]*testCase, 0)

	// defaults-round-trip
	defaults := map[string]string{"check": "make audit", "test": "go test ./..."}
	tests = append(tests, &testCase{
		_id:               "defaults-round-trip",
		_expectedDefaults: defaults,
		_expectedStored:   defaults,

		rows: &skills.Rows{Defaults: defaults},
	})

	// empty-defaults-dropped
	tests = append(tests, &testCase{
		_id:               "empty-defaults-dropped",
		_expectedDefaults: map[string]string{},
		_expectedStored:   nil,

		rows: &skills.Rows{Defaults: map[string]string{}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record := Record{}
			SetSkillRows(&record, test.rows)
			assert.Equal(t, test._expectedStored, record.Skills.Defaults)
			assert.Equal(t, test._expectedDefaults, SkillRows(record).Defaults)
		})
	}
}
