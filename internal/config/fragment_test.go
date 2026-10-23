package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSettingsDir writes files below a temp settings dir and returns it.
func writeSettingsDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	return dir
}

func TestLoadFragment(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		dir string
	}

	env := Markers{Home: "/home/a", PeekControlPort: "42542", PeekMcp: "/home/a/go/bin/peek-mcp", RepoDir: "/home/a/src"}
	tests := make([]*testCase, 0)

	// base-plus-two-fragments-composed
	tests = append(tests, &testCase{
		_id:         "base-plus-two-fragments-composed",
		_expected:   `{"model":"m","env":{"A":"1","OTEL":"http://127.0.0.1:42542/otlp"},"permissions":{"defaultMode":"acceptEdits","allow":["Bash(jq *)","Bash(ls *)"]},"hooks":{"Stop":[{"smineId":"b-stop-x","smineName":"Stop x","smineVersion":"1.0.0","hooks":[{"type":"command","command":"/home/a/src/x"}]}]}}`,
		_shouldPass: true,

		dir: writeSettingsDir(t, map[string]string{
			"base.json":         `{"model": "m", "env": {"A": "1"}, "permissions": {"defaultMode": "acceptEdits"}}`,
			"components/a.json": `{"permissions": {"allow": ["Bash(jq *)"]}, "env": {"OTEL": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}}`,
			"components/b.json": `{"permissions": {"allow": ["Bash(ls *)"]}, "hooks": {"Stop": [{"smineId": "b-stop-x", "smineName": "Stop x", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "{{REPO_DIR}}/x"}]}]}}`,
		}),
	})

	// missing-dir-empty-settings
	tests = append(tests, &testCase{
		_id:         "missing-dir-empty-settings",
		_expected:   `{}`,
		_shouldPass: true,

		dir: filepath.Join(t.TempDir(), "absent"),
	})

	// malformed-fragment-errors
	tests = append(tests, &testCase{
		_id:         "malformed-fragment-errors",
		_expected:   `null`,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/a.json": `{not json`}),
	})

	// mcp-servers-left-out
	tests = append(tests, &testCase{
		_id:         "mcp-servers-left-out",
		_expected:   `{}`,
		_shouldPass: true,

		dir: writeSettingsDir(t, map[string]string{"components/peek.json": `{"mcpServers": {"peek-mcp": {"command": "{{PEEK_MCP}}"}}}`}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			settings, err := LoadFragment(test.dir, env)
			encoded, _ := json.Marshal(settingsDoc(settings))

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.JSONEq(t, test._expected, string(encoded))
		})
	}
}

func TestLoadComponentFragment(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Fragment
		_shouldPass bool

		dir string
	}

	env := Markers{Home: "/home/a", HeartbeatPort: "6102"}
	tests := make([]*testCase, 0)

	// file-present-markers-expanded
	tests = append(tests, &testCase{
		_id:         "file-present-markers-expanded",
		_expected:   &Fragment{Env: map[string]string{"HEARTBEAT_ADDR": "127.0.0.1:6102"}},
		_shouldPass: true,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"env": {"HEARTBEAT_ADDR": "127.0.0.1:{{HEARTBEAT_PORT}}"}}`}),
	})

	// file-missing-empty-fragment
	tests = append(tests, &testCase{
		_id:         "file-missing-empty-fragment",
		_expected:   &Fragment{},
		_shouldPass: true,

		dir: t.TempDir(),
	})

	// unknown-top-level-key-errors
	tests = append(tests, &testCase{
		_id:         "unknown-top-level-key-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"envs": {}}`}),
	})

	// group-with-id-and-version-passes
	stampedGroup := `{"smineId":"heartbeat-stop","smineName":"Heartbeat stop","smineVersion":"1.0.0","hooks":[{"type":"http","url":"http://127.0.0.1:{{HEARTBEAT_PORT}}/hook"}]}`
	expandedGroup := json.RawMessage(`{"smineId":"heartbeat-stop","smineName":"Heartbeat stop","smineVersion":"1.0.0","hooks":[{"type":"http","url":"http://127.0.0.1:6102/hook"}]}`)
	tests = append(tests, &testCase{
		_id:         "group-with-id-and-version-passes",
		_expected:   &Fragment{Hooks: map[string][]json.RawMessage{"Stop": {expandedGroup}}},
		_shouldPass: true,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [` + stampedGroup + `]}}`}),
	})

	// group-without-id-errors
	tests = append(tests, &testCase{
		_id:         "group-without-id-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineName": "Heartbeat stop", "smineVersion": "1.0.0", "hooks": []}]}}`}),
	})

	// group-foreign-prefix-errors
	tests = append(tests, &testCase{
		_id:         "group-foreign-prefix-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "gate-stop", "smineName": "Gate stop", "smineVersion": "1.0.0", "hooks": []}]}}`}),
	})

	// group-version-zero-errors
	tests = append(tests, &testCase{
		_id:         "group-version-zero-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "heartbeat-stop", "smineName": "Heartbeat stop", "smineVersion": 0, "hooks": []}]}}`}),
	})

	// group-version-missing-errors
	tests = append(tests, &testCase{
		_id:         "group-version-missing-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "heartbeat-stop", "smineName": "Heartbeat stop", "hooks": []}]}}`}),
	})

	// group-version-two-part-errors
	tests = append(tests, &testCase{
		_id:         "group-version-two-part-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "heartbeat-stop", "smineName": "Heartbeat stop", "smineVersion": "1.0", "hooks": []}]}}`}),
	})

	// group-version-zero-string-errors
	tests = append(tests, &testCase{
		_id:         "group-version-zero-string-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "heartbeat-stop", "smineName": "Heartbeat stop", "smineVersion": "0.0.0", "hooks": []}]}}`}),
	})

	// group-without-name-errors
	tests = append(tests, &testCase{
		_id:         "group-without-name-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "heartbeat-stop", "smineVersion": "1.0.0", "hooks": []}]}}`}),
	})

	// group-name-over-cap-errors
	overCapName := strings.Repeat("n", MaxHookNameRunes+1)
	tests = append(tests, &testCase{
		_id:         "group-name-over-cap-errors",
		_expected:   nil,
		_shouldPass: false,

		dir: writeSettingsDir(t, map[string]string{"components/heartbeat.json": `{"hooks": {"Stop": [{"smineId": "heartbeat-stop", "smineName": "` + overCapName + `", "smineVersion": "1.0.0", "hooks": []}]}}`}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			fragment, err := LoadComponentFragment(test.dir, "heartbeat", env)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, fragment)
		})
	}
}

func TestFragment_HookIds(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		fragment *Fragment
	}

	tests := make([]*testCase, 0)

	// two-events-in-event-order
	tests = append(tests, &testCase{
		_id:         "two-events-in-event-order",
		_expected:   []string{"a-pretooluse-x", "a-stop-y", "a-stop-z"},
		_shouldPass: true,

		fragment: &Fragment{Hooks: map[string][]json.RawMessage{
			"PreToolUse": {json.RawMessage(`{"smineId": "a-pretooluse-x", "smineName": "Pre tool use x", "smineVersion": "1.0.0", "hooks": []}`)},
			"Stop": {
				json.RawMessage(`{"smineId": "a-stop-y", "smineName": "Stop y", "smineVersion": "1.0.0", "hooks": []}`),
				json.RawMessage(`{"smineId": "a-stop-z", "smineName": "Stop z", "smineVersion": "1.0.1", "hooks": []}`),
			},
		}},
	})

	// no-hooks-empty
	tests = append(tests, &testCase{
		_id:         "no-hooks-empty",
		_expected:   nil,
		_shouldPass: true,

		fragment: &Fragment{},
	})

	// non-object-group-errors
	tests = append(tests, &testCase{
		_id:         "non-object-group-errors",
		_expected:   nil,
		_shouldPass: false,

		fragment: &Fragment{Hooks: map[string][]json.RawMessage{"Stop": {json.RawMessage(`"not-a-group"`)}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			ids, err := test.fragment.HookIds()

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, ids)
		})
	}
}

// settingsDoc is the document of settings, nil for nil settings.
func settingsDoc(settings *Settings) *Document {
	if settings == nil {
		return nil
	}
	return settings.Doc()
}
