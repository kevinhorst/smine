package codex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture mirrors the repo settings/codex/config.toml: global keys, a table
// with subtables, quoted dotted table names.
const fixture = `sandbox_mode = "workspace-write"
model = "gpt-5.5"
model_reasoning_effort = "xhigh"

[sandbox_workspace_write]
network_access = false

[plugins."documents@openai-primary-runtime"]
enabled = true

[mcp_servers.peek-mcp]
command = "/Users/example/go/bin/peek-mcp"
enabled = true

[mcp_servers.peek-mcp.tools.session_full]
approval_mode = "approve"

[mcp_servers.peek-mcp.tools.session_latest]
approval_mode = "approve"

[features]
js_repl = false
`

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(fixture), 0644))
	return path
}

func TestConfigSet(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
		check   func(t *testing.T, content string)
	}{
		{
			name:  "replace-existing-scalar",
			key:   "model_reasoning_effort",
			value: `"high"`,
			check: func(t *testing.T, content string) {
				assert.Contains(t, content, `model_reasoning_effort = "high"`)
				assert.NotContains(t, content, "xhigh")
			},
		},
		{
			name:  "new-global-key",
			key:   "personality",
			value: `"pragmatic"`,
			check: func(t *testing.T, content string) {
				assert.Contains(t, content, `personality = "pragmatic"`)
			},
		},
		{
			name:  "new-key-in-existing-table",
			key:   "features.web_search",
			value: "true",
			check: func(t *testing.T, content string) {
				assert.Contains(t, content, "web_search = true")
			},
		},
		{
			name:  "new-key-creates-table",
			key:   "shell_environment_policy.inherit",
			value: `"all"`,
			check: func(t *testing.T, content string) {
				assert.Contains(t, content, "[shell_environment_policy]")
				assert.Contains(t, content, `inherit = "all"`)
			},
		},
		{
			name:    "invalid-toml-value",
			key:     "model",
			value:   `"unterminated`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFixture(t)
			cfg, err := Load(path)
			require.NoError(t, err)

			err = cfg.Set(tt.key, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NoError(t, Save(path, cfg))
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			tt.check(t, string(data))
		})
	}
}

func TestToggle(t *testing.T) {
	t.Run("table-with-subtables-moves-whole-unit", func(t *testing.T) {
		path := writeFixture(t)
		require.NoError(t, Toggle(path, "mcp_servers.peek-mcp"))

		main, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(main), "peek-mcp")

		disabled, err := os.ReadFile(DisabledPath(path))
		require.NoError(t, err)
		assert.Contains(t, string(disabled), "[mcp_servers.peek-mcp]")
		assert.Contains(t, string(disabled), "[mcp_servers.peek-mcp.tools.session_full]")
		assert.Contains(t, string(disabled), "[mcp_servers.peek-mcp.tools.session_latest]")
	})

	t.Run("global-key-moves", func(t *testing.T) {
		path := writeFixture(t)
		require.NoError(t, Toggle(path, "model"))

		main, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(main), `model = "gpt-5.5"`)

		disabled, err := os.ReadFile(DisabledPath(path))
		require.NoError(t, err)
		assert.Contains(t, string(disabled), `model = "gpt-5.5"`)
	})

	t.Run("re-enable-restores", func(t *testing.T) {
		path := writeFixture(t)
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		require.NoError(t, Toggle(path, "mcp_servers.peek-mcp"))
		require.NoError(t, Toggle(path, "mcp_servers.peek-mcp"))

		after, err := os.ReadFile(path)
		require.NoError(t, err)

		// Untouched lines byte-identical: every non-empty line of the
		// original file must be present verbatim after disable+re-enable.
		for _, line := range strings.Split(string(before), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			assert.Contains(t, string(after), line)
		}
	})

	t.Run("unknown-key-not-found", func(t *testing.T) {
		path := writeFixture(t)
		err := Toggle(path, "no_such_key")
		assert.True(t, errors.Is(err, ErrNotFound))
	})
}
