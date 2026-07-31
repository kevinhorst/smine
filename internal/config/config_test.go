package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestSettingsPermissionsView(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		s := NewSettings()
		perms := Permissions{Allow: []string{"Bash(jq *)"}, Ask: []string{"Bash(rm *)"}}
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
		require.NoError(t, Save(path, s))

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
		// Parent dir does not exist, so the tmp WriteFile fails.
		err := Save(filepath.Join(t.TempDir(), "no-such-dir", "settings.json"), s)
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
