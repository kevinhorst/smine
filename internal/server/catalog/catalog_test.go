package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAndValidate(t *testing.T) {
	t.Run("embedded-catalog-parses", func(t *testing.T) {
		entries, err := Load()
		require.NoError(t, err)
		require.NotEmpty(t, entries)

		validTypes := []string{"string", "bool", "number", "enum", "array", "object", "table"}
		for _, e := range entries {
			assert.NotEmpty(t, e.Key, "entry without key")
			assert.Contains(t, []string{"claude", "codex"}, e.Target, "key %s", e.Key)
			assert.Contains(t, validTypes, e.Type, "key %s", e.Key)
			assert.NotEmpty(t, e.Explanation, "key %s", e.Key)
			if e.Type == "enum" {
				assert.NotEmpty(t, e.Values, "enum key %s without values", e.Key)
			}
		}

		assert.NotEmpty(t, ForTarget(entries, "claude"))
		assert.NotEmpty(t, ForTarget(entries, "codex"))
	})

	t.Run("bool-enum-number-validation", func(t *testing.T) {
		assert.NoError(t, Validate(&Entry{Key: "k", Type: "bool"}, "true"))
		assert.Error(t, Validate(&Entry{Key: "k", Type: "bool"}, "yes"))
		assert.NoError(t, Validate(&Entry{Key: "k", Type: "number"}, "12.5"))
		assert.Error(t, Validate(&Entry{Key: "k", Type: "number"}, "twelve"))
		assert.NoError(t, Validate(&Entry{Key: "k", Type: "enum", Values: []string{"low", "high"}}, "low"))
		assert.NoError(t, Validate(&Entry{Key: "k", Type: "string"}, "anything"))
	})

	t.Run("unknown-enum-value-rejected", func(t *testing.T) {
		err := Validate(&Entry{Key: "k", Type: "enum", Values: []string{"low", "high"}}, "medium")
		assert.Error(t, err)
	})
}
