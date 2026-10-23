package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/trash"
)

func TestHookGroup_RoundTripKeepsHookKeys(t *testing.T) {
	stored := `{"smineId":"heartbeat-stop","hooks":[{"timeout":1,"type":"http","url":"http://127.0.0.1:6101/hook"},{"command":"say done","timeout":5,"type":"command"}],"smineName":"Heartbeat","smineVersion":"1.0.1"}`

	var group HookGroup
	require.NoError(t, json.Unmarshal([]byte(stored), &group))
	encoded, err := json.Marshal(&group)
	require.NoError(t, err)

	assert.JSONEq(t, stored, string(encoded))
}

func TestHookVersion_UnmarshalJSON(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   HookVersion
		_shouldPass bool

		data string
	}

	tests := make([]*testCase, 0)

	// string-kept
	tests = append(tests, &testCase{_id: "string-kept", _expected: "1.0.1", _shouldPass: true, data: `"1.0.1"`})

	// integer-becomes-major-release
	tests = append(tests, &testCase{_id: "integer-becomes-major-release", _expected: "1.0.0", _shouldPass: true, data: `1`})

	// bool-errors
	tests = append(tests, &testCase{_id: "bool-errors", _expected: "", _shouldPass: false, data: `true`})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			var version HookVersion
			err := json.Unmarshal([]byte(test.data), &version)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, version)
		})
	}
}

func TestHookVersion_Compare(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   int
		_shouldPass bool

		other   HookVersion
		version HookVersion
	}

	tests := make([]*testCase, 0)

	// lower-patch
	tests = append(tests, &testCase{_id: "lower-patch", _expected: -1, _shouldPass: true, other: "1.0.1", version: "1.0.0"})

	// higher-major
	tests = append(tests, &testCase{_id: "higher-major", _expected: 1, _shouldPass: true, other: "1.9.9", version: "2.0.0"})

	// malformed-errors
	tests = append(tests, &testCase{_id: "malformed-errors", _expected: 0, _shouldPass: false, other: "1.0.0", version: "x"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			order, err := test.version.Compare(test.other)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, order)
		})
	}
}

func TestSettingsEnvView(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		s := NewSettings()
		env := map[string]string{"FOO": "bar", "BAZ": "qux"}
		require.NoError(t, s.SetEnv(env))
		got, err := s.Env()
		require.NoError(t, err)
		assert.Equal(t, env, got)
	})

	t.Run("set-empty-unsets-key", func(t *testing.T) {
		s := NewSettings()
		require.NoError(t, s.SetEnv(map[string]string{"FOO": "bar"}))
		require.NoError(t, s.SetEnv(nil))
		_, ok := s.Doc().Get([]string{"env"})
		assert.False(t, ok)
	})

	t.Run("missing-key-nil", func(t *testing.T) {
		s := NewSettings()
		env, err := s.Env()
		require.NoError(t, err)
		assert.Nil(t, env)
	})
}

func TestSettings_SkillOverrides(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedKeys      []string
		_expectedOverrides map[string]string
		_shouldPass        bool

		settings *Settings
	}

	tests := make([]*testCase, 0)

	// absent-key
	tests = append(tests, &testCase{
		_id:                "absent-key",
		_expectedKeys:      []string{"model"},
		_expectedOverrides: nil,
		_shouldPass:        true,

		settings: loadSettings(t, `{"model":"opus"}`),
	})

	// set-and-read
	settings := NewSettings()
	require.NoError(t, settings.SetSkillOverrides(map[string]string{"demo": "off", "other": "on"}))
	tests = append(tests, &testCase{
		_id:                "set-and-read",
		_expectedKeys:      []string{"skillOverrides"},
		_expectedOverrides: map[string]string{"demo": "off", "other": "on"},
		_shouldPass:        true,

		settings: settings,
	})

	// empty-map-removes-key
	settings = loadSettings(t, `{"model":"opus","skillOverrides":{"demo":"off"}}`)
	require.NoError(t, settings.SetSkillOverrides(map[string]string{}))
	tests = append(tests, &testCase{
		_id:                "empty-map-removes-key",
		_expectedKeys:      []string{"model"},
		_expectedOverrides: nil,
		_shouldPass:        true,

		settings: settings,
	})

	// other-keys-keep-order
	settings = loadSettings(t, `{"zeta":1,"skillOverrides":{"demo":"off"},"alpha":2}`)
	require.NoError(t, settings.SetSkillOverrides(map[string]string{"demo": "on"}))
	tests = append(tests, &testCase{
		_id:                "other-keys-keep-order",
		_expectedKeys:      []string{"zeta", "skillOverrides", "alpha"},
		_expectedOverrides: map[string]string{"demo": "on"},
		_shouldPass:        true,

		settings: settings,
	})

	// wrong-shape
	tests = append(tests, &testCase{
		_id:                "wrong-shape",
		_expectedKeys:      []string{"skillOverrides"},
		_expectedOverrides: nil,
		_shouldPass:        false,

		settings: loadSettings(t, `{"skillOverrides":"nope"}`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			overrides, err := test.settings.SkillOverrides()

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedOverrides, overrides)
			assert.Equal(t, test._expectedKeys, test.settings.Doc().Keys())
		})
	}
}

// loadSettings parses body as a settings file.
func loadSettings(t *testing.T, body string) *Settings {
	t.Helper()
	settings, err := Load(writeSettingsFile(t, body))
	require.NoError(t, err)
	return settings
}

func TestSettingsPermissionsView(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		s := NewSettings()
		perms := Permissions{Allow: []string{"Bash(jq *)"}, Ask: []string{"Bash(rm *)"}, Deny: []string{"Bash(git stash*)"}}
		require.NoError(t, s.SetPermissions(perms))
		got, err := s.Permissions()
		require.NoError(t, err)
		assert.Equal(t, perms, got)
	})

	t.Run("set-empty-unsets-key", func(t *testing.T) {
		s := NewSettings()
		require.NoError(t, s.SetPermissions(Permissions{Allow: []string{"x"}}))
		require.NoError(t, s.SetPermissions(Permissions{}))
		_, ok := s.Doc().Get([]string{"permissions"})
		assert.False(t, ok)
	})
}

func TestSettingsDisabledMcpjsonServersView(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		s := NewSettings()
		servers := []string{"a", "b"}
		require.NoError(t, s.SetDisabledMcpjsonServers(servers))
		got, err := s.DisabledMcpjsonServers()
		require.NoError(t, err)
		assert.Equal(t, servers, got)
	})

	t.Run("set-empty-unsets-key", func(t *testing.T) {
		s := NewSettings()
		require.NoError(t, s.SetDisabledMcpjsonServers([]string{"a"}))
		require.NoError(t, s.SetDisabledMcpjsonServers(nil))
		_, ok := s.Doc().Get([]string{"disabledMcpjsonServers"})
		assert.False(t, ok)
	})

	t.Run("missing-key-nil", func(t *testing.T) {
		s := NewSettings()
		servers, err := s.DisabledMcpjsonServers()
		require.NoError(t, err)
		assert.Nil(t, servers)
	})
}

func TestMcpServerNames(t *testing.T) {
	t.Run("missing-file-nil", func(t *testing.T) {
		names, err := McpServerNames(filepath.Join(t.TempDir(), "absent.json"))
		require.NoError(t, err)
		assert.Nil(t, names)
	})

	t.Run("sorted-keys", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".claude.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{"zeta":{},"alpha":{},"mid":{}}}`), 0644))
		names, err := McpServerNames(path)
		require.NoError(t, err)
		assert.Equal(t, []string{"alpha", "mid", "zeta"}, names)
	})

	t.Run("no-servers-key-empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".claude.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"other":1}`), 0644))
		names, err := McpServerNames(path)
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("malformed-json-errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".claude.json")
		require.NoError(t, os.WriteFile(path, []byte(`{not json`), 0644))
		_, err := McpServerNames(path)
		assert.Error(t, err)
	})
}

func TestPathHelpers(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".claude.json"), DefaultClaudeJsonPath())
	assert.Equal(t, filepath.Join(home, ".claude", "settings.json"), DefaultPath())
	assert.Equal(t,
		filepath.Join(home, ".claude", "settings.disabled.json"),
		DisabledPath(filepath.Join(home, ".claude", "settings.json")),
	)
}

func TestLoadSave(t *testing.T) {
	t.Run("missing-file-yields-empty-settings", func(t *testing.T) {
		s, err := Load(filepath.Join(t.TempDir(), "absent.json"))
		require.NoError(t, err)
		_, ok := s.Doc().Get([]string{"model"})
		assert.False(t, ok)
	})

	t.Run("malformed-json-errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, os.WriteFile(path, []byte(`[1,2]`), 0644))
		_, err := Load(path)
		assert.Error(t, err)
	})

	t.Run("save-then-load-round-trip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		s := NewSettings()
		require.NoError(t, s.SetModel("opus"))
		require.NoError(t, s.SetEnv(map[string]string{"FOO": "bar"}))
		require.NoError(t, Save(path, s, nil))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, byte('\n'), data[len(data)-1])
		assert.NoFileExists(t, path+".tmp")

		reloaded, err := Load(path)
		require.NoError(t, err)
		model, err := reloaded.Model()
		require.NoError(t, err)
		assert.Equal(t, "opus", model)
	})
}

func TestSave_KeepsPrevious(t *testing.T) {
	stateDir := t.TempDir()
	store := trash.New(stateDir, 0)
	path := writeSettingsFile(t, `{"model": "old"}`)
	s := NewSettings()
	require.NoError(t, s.SetModel("new"))

	require.NoError(t, Save(path, s, store))

	live, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model": "new"}`, string(live))

	entries, err := store.Tail(10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, trash.KindReplaced, entries[0].Kind)
	assert.Equal(t, "settings save", entries[0].Action)
	assert.Equal(t, path, entries[0].Path)

	stashed, err := os.ReadFile(filepath.Join(stateDir, trash.TrashDir, filepath.FromSlash(entries[0].Stash)))
	require.NoError(t, err)
	assert.JSONEq(t, `{"model": "old"}`, string(stashed))
}

func TestSave_NilStore(t *testing.T) {
	path := writeSettingsFile(t, `{"model": "old"}`)
	s := NewSettings()
	require.NoError(t, s.SetModel("new"))

	require.NoError(t, Save(path, s, nil))

	live, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model": "new"}`, string(live))

	siblings, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, siblings, 1)
}

func writeSettingsFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestSettingsDecodeErrors(t *testing.T) {
	// A key present but shaped wrong for its typed view surfaces a decode error.
	load := func(t *testing.T, body string) *Settings {
		t.Helper()
		path := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, os.WriteFile(path, []byte(body), 0644))
		s, err := Load(path)
		require.NoError(t, err)
		return s
	}

	t.Run("hooks-wrong-shape", func(t *testing.T) {
		_, err := load(t, `{"hooks":"not-an-object"}`).Hooks()
		assert.Error(t, err)
	})
	t.Run("env-wrong-shape", func(t *testing.T) {
		_, err := load(t, `{"env":"not-an-object"}`).Env()
		assert.Error(t, err)
	})
	t.Run("model-wrong-shape", func(t *testing.T) {
		_, err := load(t, `{"model":123}`).Model()
		assert.Error(t, err)
	})
	t.Run("permissions-wrong-shape", func(t *testing.T) {
		_, err := load(t, `{"permissions":"nope"}`).Permissions()
		assert.Error(t, err)
	})
	t.Run("disabled-mcp-wrong-shape", func(t *testing.T) {
		_, err := load(t, `{"disabledMcpjsonServers":"nope"}`).DisabledMcpjsonServers()
		assert.Error(t, err)
	})
}

func TestFileIOErrors(t *testing.T) {
	t.Run("mcp-server-names-read-error", func(t *testing.T) {
		// A directory is not a missing file: the read fails with a non-IsNotExist error.
		_, err := McpServerNames(t.TempDir())
		assert.Error(t, err)
	})

	t.Run("load-read-error", func(t *testing.T) {
		_, err := Load(t.TempDir())
		assert.Error(t, err)
	})

	t.Run("save-write-error", func(t *testing.T) {
		s := NewSettings()
		require.NoError(t, s.SetModel("opus"))
		// The parent is a regular file, so the directory cannot be created.
		parent := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(parent, nil, 0o644))
		err := Save(filepath.Join(parent, "settings.json"), s, nil)
		assert.Error(t, err)
	})
}

func TestDocumentEdgeBranches(t *testing.T) {
	t.Run("get-empty-path", func(t *testing.T) {
		d := NewDocument()
		_, ok := d.Get(nil)
		assert.False(t, ok)
	})

	t.Run("get-nested-miss", func(t *testing.T) {
		d := NewDocument()
		require.NoError(t, d.Set([]string{"a"}, []byte(`{"b":1}`)))
		_, ok := d.Get([]string{"a", "missing"})
		assert.False(t, ok)
	})

	t.Run("get-nested-value-not-object", func(t *testing.T) {
		d := NewDocument()
		require.NoError(t, d.Set([]string{"a"}, []byte(`5`)))
		_, ok := d.Get([]string{"a", "b"})
		assert.False(t, ok)
	})

	t.Run("set-empty-path-errors", func(t *testing.T) {
		d := NewDocument()
		assert.Error(t, d.Set(nil, []byte(`1`)))
	})

	t.Run("unset-empty-path", func(t *testing.T) {
		d := NewDocument()
		assert.False(t, d.Unset(nil))
	})

	t.Run("unset-missing-key", func(t *testing.T) {
		d := NewDocument()
		assert.False(t, d.Unset([]string{"nope"}))
	})

	t.Run("unset-nested-key-not-object", func(t *testing.T) {
		d := NewDocument()
		require.NoError(t, d.Set([]string{"a"}, []byte(`5`)))
		assert.False(t, d.Unset([]string{"a", "b"}))
	})
}
