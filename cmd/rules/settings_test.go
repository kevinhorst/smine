package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/skills"
)

// captureStderr runs run with os.Stderr redirected into a temp file and returns its exit code and the captured text.
func captureStderr(t *testing.T, run func() int) (int, string) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)

	original := os.Stderr
	os.Stderr = file
	code := run()
	os.Stderr = original
	require.NoError(t, file.Close())

	captured, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	return code, string(captured)
}

// newSettingsEnv is a temp Claude home over a temp checkout marked as a release archive, whose gate binaries are prebuilt stubs.
func newSettingsEnv(t *testing.T) install.Env {
	t.Helper()
	home := t.TempDir()
	repo := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("TMP", t.TempDir())
	t.Setenv("TEMP", t.TempDir())
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "bin"), 0o755))
	require.NoError(t, os.WriteFile(install.PayloadMarkerPath(repo), []byte("0.0.0\n"), 0o644))
	for _, name := range gateBinaryStubs() {
		path := filepath.Join(repo, "bin", name+exe)
		require.NoError(t, os.WriteFile(path, []byte(name+" binary"), 0o644))
	}

	env, err := install.NewEnvAt(repo, filepath.Join(home, ".claude"), io.Discard)
	require.NoError(t, err)
	return env
}

// gateBinaryStubs are the hook binaries the gate component installs.
func gateBinaryStubs() []string {
	return []string{"toolpolicy", "skillinvokegate", "skillenv"}
}

// gateRecord selects the gate with the components it requires, no skills.
func gateRecord() install.Record {
	record := install.FullRecord(runtime.GOOS)
	record.Components = []components.ID{components.ConfigServer, components.SessionHooks, components.Gate}
	record.Preset = install.PresetCustom
	record.Skills = install.Selection{Mode: skills.SelectionModeNone}
	return record
}

// savedOverride is the compact JSON the saved record holds for gate.<key>, empty when it holds none.
func savedOverride(t *testing.T, env install.Env, key string) string {
	t.Helper()
	record, _, err := install.Load(env)
	require.NoError(t, err)

	raw, isSet := record.Settings[components.Gate][key]
	if !isSet {
		return ""
	}
	compact := &bytes.Buffer{}
	require.NoError(t, json.Compact(compact, raw))
	return compact.String()
}

// withOverride returns record with gate.<key> overridden by text.
func withOverride(t *testing.T, record install.Record, key, text string) install.Record {
	t.Helper()
	setting, ok := components.LookupSetting(components.Gate, key)
	require.True(t, ok)
	value, err := components.ParseText(&setting, text)
	require.NoError(t, err)
	require.NoError(t, install.SetOverride(&record, components.Gate, key, value))
	return record
}

func TestSettingsGet(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedCode int
		_expectedOut  string

		args   []string
		env    install.Env
		record install.Record
	}

	tests := make([]*testCase, 0)

	// default
	tests = append(tests, &testCase{
		_id:           "default",
		_expectedCode: exitClean,
		_expectedOut:  "16\ndefault: 16\nsource: default\nkind: int\napplies: live\n",

		args:   []string{"gate.max_loop_items"},
		env:    newSettingsEnv(t),
		record: gateRecord(),
	})

	// override
	tests = append(tests, &testCase{
		_id:           "override",
		_expectedCode: exitClean,
		_expectedOut:  "32\ndefault: 16\nsource: record\nkind: int\napplies: live\n",

		args:   []string{"gate.max_loop_items"},
		env:    newSettingsEnv(t),
		record: withOverride(t, gateRecord(), "max_loop_items", "32"),
	})

	// fail-unknown-key
	tests = append(tests, &testCase{
		_id:           "fail-unknown-key",
		_expectedCode: exitError,
		_expectedOut:  "",

		args:   []string{"gate.nope"},
		env:    newSettingsEnv(t),
		record: gateRecord(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			out := &bytes.Buffer{}
			code := settingsGet(test.record, test.args, test.env, out)
			assert.Equal(t, test._expectedCode, code)
			assert.Contains(t, out.String(), test._expectedOut)
		})
	}
}

func TestSettingsSet(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedError    string
		_expectedOut      string
		_expectedOverride string

		args   []string
		env    install.Env
		key    string
		record install.Record
	}

	tests := make([]*testCase, 0)

	// pass-int
	tests = append(tests, &testCase{
		_id:               "pass-int",
		_expectedCode:     exitClean,
		_expectedError:    "",
		_expectedOut:      "session-hooks: install\ngate: install\ngate.max_loop_items = 32\nreload: live\n",
		_expectedOverride: "32",

		args:   []string{"gate.max_loop_items", "32"},
		env:    newSettingsEnv(t),
		key:    "max_loop_items",
		record: gateRecord(),
	})

	// pass-list-json
	tests = append(tests, &testCase{
		_id:               "pass-list-json",
		_expectedCode:     exitClean,
		_expectedError:    "",
		_expectedOut:      "gate.denied_tools = [\"TaskStop\"]\nreload: live\n",
		_expectedOverride: `["TaskStop"]`,

		args:   []string{"gate.denied_tools", `["TaskStop"]`},
		env:    newSettingsEnv(t),
		key:    "denied_tools",
		record: gateRecord(),
	})

	// fail-unknown-key
	tests = append(tests, &testCase{
		_id:               "fail-unknown-key",
		_expectedCode:     exitError,
		_expectedError:    "unknown setting gate.nope",
		_expectedOut:      "",
		_expectedOverride: "",

		args:   []string{"gate.nope", "1"},
		env:    newSettingsEnv(t),
		key:    "nope",
		record: gateRecord(),
	})

	// fail-wrong-kind
	tests = append(tests, &testCase{
		_id:               "fail-wrong-kind",
		_expectedCode:     exitError,
		_expectedError:    "valid: a non-negative integer",
		_expectedOut:      "",
		_expectedOverride: "",

		args:   []string{"gate.max_loop_items", "x"},
		env:    newSettingsEnv(t),
		key:    "max_loop_items",
		record: gateRecord(),
	})

	// pass-default-clears-override
	tests = append(tests, &testCase{
		_id:               "pass-default-clears-override",
		_expectedCode:     exitClean,
		_expectedError:    "",
		_expectedOut:      "gate.max_loop_items = 16\nreload: live\n",
		_expectedOverride: "",

		args:   []string{"gate.max_loop_items", "16"},
		env:    newSettingsEnv(t),
		key:    "max_loop_items",
		record: withOverride(t, gateRecord(), "max_loop_items", "32"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			out := &bytes.Buffer{}
			code, stderr := captureStderr(t, func() int {
				return settingsSet(context.Background(), test.record, test.args, test.env, out)
			})
			assert.Equal(t, test._expectedCode, code)
			assert.Contains(t, stderr, test._expectedError)
			assert.Contains(t, out.String(), test._expectedOut)
			assert.Equal(t, test._expectedOverride, savedOverride(t, test.env, test.key))
		})
	}
}

func TestWriteOrCheck(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCode    int
		_expectedContent string

		isCheck  bool
		path     string
		rendered []byte
	}

	writeFixture := func(content string) string {
		path := filepath.Join(t.TempDir(), "config-defaults.json")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return path
	}

	tests := make([]*testCase, 0)

	// write
	tests = append(tests, &testCase{
		_id:              "write",
		_expectedCode:    exitClean,
		_expectedContent: "new\n",

		isCheck:  false,
		path:     filepath.Join(t.TempDir(), "config-defaults.json"),
		rendered: []byte("new\n"),
	})

	// current
	tests = append(tests, &testCase{
		_id:              "current",
		_expectedCode:    exitClean,
		_expectedContent: "same\n",

		isCheck:  true,
		path:     writeFixture("same\n"),
		rendered: []byte("same\n"),
	})

	// stale
	tests = append(tests, &testCase{
		_id:              "stale",
		_expectedCode:    exitViolations,
		_expectedContent: "old\n",

		isCheck:  true,
		path:     writeFixture("old\n"),
		rendered: []byte("new\n"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code := writeOrCheck(test.path, test.rendered, test.isCheck, "settings-defaults")
			content, err := os.ReadFile(test.path)
			assert.Equal(t, test._expectedCode, code)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedContent, string(content))
		})
	}
}
