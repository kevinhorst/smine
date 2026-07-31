package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentRoundTrip(t *testing.T) {
	fixture := `{
  "model": "opus",
  "zzUnknownKey": {"nested": true, "list": [1, 2]},
  "env": {"FOO": "bar"},
  "alwaysThinkingEnabled": false
}`

	t.Run("unknown-keys-preserved", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, os.WriteFile(path, []byte(fixture), 0644))
		s, err := Load(path)
		require.NoError(t, err)
		require.NoError(t, s.SetModel("sonnet"))
		require.NoError(t, Save(path, s))

		reloaded, err := Load(path)
		require.NoError(t, err)
		raw, ok := reloaded.Doc().Get([]string{"zzUnknownKey"})
		require.True(t, ok)
		assert.JSONEq(t, `{"nested": true, "list": [1, 2]}`, string(raw))
		raw, ok = reloaded.Doc().Get([]string{"alwaysThinkingEnabled"})
		require.True(t, ok)
		assert.Equal(t, "false", string(raw))
	})

	t.Run("key-order-preserved", func(t *testing.T) {
		doc := NewDocument()
		require.NoError(t, json.Unmarshal([]byte(fixture), doc))
		assert.Equal(t, []string{"model", "zzUnknownKey", "env", "alwaysThinkingEnabled"}, doc.Keys())

		out, err := json.Marshal(doc)
		require.NoError(t, err)
		reparsed := NewDocument()
		require.NoError(t, json.Unmarshal(out, reparsed))
		assert.Equal(t, doc.Keys(), reparsed.Keys())
	})

	t.Run("nested-set-creates-parent", func(t *testing.T) {
		doc := NewDocument()
		require.NoError(t, doc.Set([]string{"statusLine", "type"}, json.RawMessage(`"command"`)))
		raw, ok := doc.Get([]string{"statusLine", "type"})
		require.True(t, ok)
		assert.Equal(t, `"command"`, string(raw))
	})

	t.Run("unset-top-level", func(t *testing.T) {
		doc := NewDocument()
		require.NoError(t, json.Unmarshal([]byte(fixture), doc))
		assert.True(t, doc.Unset([]string{"model"}))
		_, ok := doc.Get([]string{"model"})
		assert.False(t, ok)
		assert.False(t, doc.Unset([]string{"model"}))
	})

	t.Run("unset-nested-keeps-empty-parent", func(t *testing.T) {
		doc := NewDocument()
		require.NoError(t, json.Unmarshal([]byte(fixture), doc))
		assert.True(t, doc.Unset([]string{"env", "FOO"}))
		raw, ok := doc.Get([]string{"env"})
		require.True(t, ok)
		assert.JSONEq(t, `{}`, string(raw))
	})

	t.Run("not-an-object", func(t *testing.T) {
		doc := NewDocument()
		assert.Error(t, json.Unmarshal([]byte(`[1, 2]`), doc))
		require.NoError(t, json.Unmarshal([]byte(fixture), doc))
		assert.Error(t, doc.Set([]string{"model", "nested"}, json.RawMessage(`1`)))
	})
}

func TestSettingsViews(t *testing.T) {
	t.Run("hooks-round-trip", func(t *testing.T) {
		s := NewSettings()
		hooks := map[string][]HookGroup{
			"Stop": {{Hooks: []Hook{{Command: "afplay done.aiff", Type: "command"}}}},
		}
		require.NoError(t, s.SetHooks(hooks))
		got, err := s.Hooks()
		require.NoError(t, err)
		assert.Equal(t, hooks, got)
	})

	t.Run("set-empty-unsets-key", func(t *testing.T) {
		s := NewSettings()
		require.NoError(t, s.SetHooks(map[string][]HookGroup{"Stop": {}}))
		require.NoError(t, s.SetHooks(nil))
		_, ok := s.Doc().Get([]string{"hooks"})
		assert.False(t, ok)
	})

	t.Run("model-string", func(t *testing.T) {
		s := NewSettings()
		require.NoError(t, s.SetModel("opus"))
		model, err := s.Model()
		require.NoError(t, err)
		assert.Equal(t, "opus", model)
		require.NoError(t, s.SetModel(""))
		_, ok := s.Doc().Get([]string{"model"})
		assert.False(t, ok)
	})

	t.Run("missing-key-zero-value", func(t *testing.T) {
		s := NewSettings()
		hooks, err := s.Hooks()
		require.NoError(t, err)
		assert.Nil(t, hooks)
		perms, err := s.Permissions()
		require.NoError(t, err)
		assert.Empty(t, perms.Allow)
		model, err := s.Model()
		require.NoError(t, err)
		assert.Empty(t, model)
	})
}
