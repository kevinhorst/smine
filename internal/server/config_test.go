package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/codex"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigSetClaude(t *testing.T) {
	t.Run("set-documented-key", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/model", url.Values{"value": {"opus"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		var file map[string]any
		require.NoError(t, json.Unmarshal(data, &file))
		assert.Equal(t, "opus", file["model"])
	})

	t.Run("unknown-keys-survive-save", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, os.WriteFile(settingsPath,
			[]byte(`{"zzUnknownKey": {"nested": true}, "model": "sonnet"}`), 0644))
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/model", url.Values{"value": {"opus"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		var file map[string]any
		require.NoError(t, json.Unmarshal(data, &file))
		assert.Equal(t, map[string]any{"nested": true}, file["zzUnknownKey"])
		assert.Equal(t, "opus", file["model"])
		// Key order preserved: the unknown key stays first.
		assert.Less(t, strings.Index(string(data), "zzUnknownKey"), strings.Index(string(data), "model"))
	})

	t.Run("mutations-trigger-config-op", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/model", url.Values{"value": {"opus"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/config/claude/model", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

		// The page GET must not carry the trigger — it would loop the re-pull.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Empty(t, response.Header().Get("HX-Trigger"))
	})

	t.Run("invalid-enum-400", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/editorMode", url.Values{"value": {"bogus"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("undocumented-key-strict-json", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/zzCustom", url.Values{"value": {"not json"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/zzCustom", url.Values{"value": {`{"a": 1}`}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		var file map[string]any
		require.NoError(t, json.Unmarshal(data, &file))
		assert.Equal(t, map[string]any{"a": float64(1)}, file["zzCustom"])
	})
}

func TestConfigPages(t *testing.T) {
	server := newTestServer(t, &Options{})

	t.Run("config-redirects-to-claude", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config", nil))
		require.Equal(t, http.StatusFound, response.Code)
		assert.Equal(t, "/config/claude", response.Header().Get("Location"))
	})

	t.Run("unknown-target-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/bogus", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestConfigActiveAvailableSplit(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"model": "opus"}`), 0644))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()

	assert.Contains(t, body, "Show available (")
	// The set key renders before the available section, unset keys after it.
	availableAt := strings.Index(body, "Show available (")
	modelAt := strings.Index(body, `id="row-claude-model"`)
	unsetAt := strings.Index(body, `id="row-claude-disabledMcpjsonServers"`)
	require.Greater(t, modelAt, 0)
	require.Greater(t, unsetAt, 0)
	assert.Less(t, modelAt, availableAt)
	assert.Greater(t, unsetAt, availableAt)
}

func TestConfigOpenSection(t *testing.T) {
	server := newTestServer(t, &Options{})

	t.Run("open-param-expands-section", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude?open=model", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `<details id="model" class="section" open>`)
	})

	t.Run("no-param-leaves-sections-collapsed", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<details id="model" class="section">`)
		assert.NotContains(t, body, `<details id="model" class="section" open>`)
	})
}

func TestConfigTabs(t *testing.T) {
	server := newTestServer(t, &Options{})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `href="/config/codex"`)
	assert.NotContains(t, body, "/docs\"")
}

const codexFixture = `sandbox_mode = "workspace-write"
model = "gpt-5.5"

[mcp_servers.peek-mcp]
command = "/Users/example/go/bin/peek-mcp"

[mcp_servers.peek-mcp.tools.session_full]
approval_mode = "approve"

[mcp_servers.peek-mcp.tools.session_latest]
approval_mode = "approve"

[features]
js_repl = false
`

func TestConfigCodexToggle(t *testing.T) {
	t.Run("disable-mcp-table-with-subtables", func(t *testing.T) {
		codexPath := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(codexPath, []byte(codexFixture), 0644))
		server := newTestServer(t, &Options{CodexPath: codexPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/mcp_servers.peek-mcp/toggle", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		mainData, err := os.ReadFile(codexPath)
		require.NoError(t, err)
		assert.NotContains(t, string(mainData), "peek-mcp")
		assert.Contains(t, string(mainData), "[features]")

		disabledData, err := os.ReadFile(codex.DisabledPath(codexPath))
		require.NoError(t, err)
		assert.Contains(t, string(disabledData), "[mcp_servers.peek-mcp]")
		assert.Contains(t, string(disabledData), "[mcp_servers.peek-mcp.tools.session_full]")
		assert.Contains(t, string(disabledData), "[mcp_servers.peek-mcp.tools.session_latest]")
	})

	t.Run("unknown-404", func(t *testing.T) {
		codexPath := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(codexPath, []byte(codexFixture), 0644))
		server := newTestServer(t, &Options{CodexPath: codexPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/mcp_servers.nope/toggle", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestClaudeOverridden(t *testing.T) {
	doc := func(t *testing.T, content string) *config.Document {
		t.Helper()
		path := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		settings, err := config.Load(path)
		require.NoError(t, err)
		return settings.Doc()
	}

	t.Run("equal-values-different-key-order-false", func(t *testing.T) {
		live := doc(t, `{"env": {"A": "1", "B": "2"}}`)
		frag := doc(t, `{"env": {"B": "2", "A": "1"}}`)
		assert.False(t, claudeOverridden(live, frag, []string{"env"}))
	})

	t.Run("both-missing-false", func(t *testing.T) {
		live := doc(t, `{}`)
		frag := doc(t, `{}`)
		assert.False(t, claudeOverridden(live, frag, []string{"model"}))
	})

	t.Run("live-only-true", func(t *testing.T) {
		live := doc(t, `{"model": "opus"}`)
		frag := doc(t, `{}`)
		assert.True(t, claudeOverridden(live, frag, []string{"model"}))
	})

	t.Run("fragment-only-true", func(t *testing.T) {
		live := doc(t, `{}`)
		frag := doc(t, `{"model": "opus"}`)
		assert.True(t, claudeOverridden(live, frag, []string{"model"}))
	})

	t.Run("array-order-difference-false", func(t *testing.T) {
		// Toggles re-append on enable; order is presentation noise.
		live := doc(t, `{"disabledMcpjsonServers": ["a", "b"]}`)
		frag := doc(t, `{"disabledMcpjsonServers": ["b", "a"]}`)
		assert.False(t, claudeOverridden(live, frag, []string{"disabledMcpjsonServers"}))
	})

	t.Run("nested-array-order-difference-false", func(t *testing.T) {
		live := doc(t, `{"permissions": {"allow": ["Bash(ls *)", "WebSearch"]}}`)
		frag := doc(t, `{"permissions": {"allow": ["WebSearch", "Bash(ls *)"]}}`)
		assert.False(t, claudeOverridden(live, frag, []string{"permissions"}))
	})

	t.Run("array-content-difference-true", func(t *testing.T) {
		live := doc(t, `{"disabledMcpjsonServers": ["a", "a"]}`)
		frag := doc(t, `{"disabledMcpjsonServers": ["a"]}`)
		assert.True(t, claudeOverridden(live, frag, []string{"disabledMcpjsonServers"}))
	})
}

func TestConfigOverriddenPage(t *testing.T) {
	newServer := func(t *testing.T, live, fragment string) *Server {
		t.Helper()
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")
		require.NoError(t, os.WriteFile(settingsPath, []byte(live), 0644))
		fragmentPath := filepath.Join(dir, "fragment.json")
		if fragment != "" {
			require.NoError(t, os.WriteFile(fragmentPath, []byte(fragment), 0644))
		}
		return newTestServer(t, &Options{ClaudeFragmentPath: fragmentPath, SettingsPath: settingsPath})
	}

	getConfig := func(t *testing.T, server *Server) string {
		t.Helper()
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		return response.Body.String()
	}

	t.Run("drifted-key-marked-overridden", func(t *testing.T) {
		server := newServer(t, `{"model": "sonnet", "theme": "dark"}`, `{"model": "opus", "theme": "dark"}`)
		body := getConfig(t, server)

		modelRow := body[strings.Index(body, `id="row-claude-model"`):]
		modelRow = modelRow[:strings.Index(modelRow, "</div>")]
		assert.Contains(t, modelRow, "overridden")

		themeRow := body[strings.Index(body, `id="row-claude-theme"`):]
		themeRow = themeRow[:strings.Index(themeRow, "</div>")]
		assert.NotContains(t, themeRow, "overridden")
	})

	t.Run("fragment-only-undocumented-key-listed", func(t *testing.T) {
		server := newServer(t, `{}`, `{"customThing": true}`)
		body := getConfig(t, server)
		row := body[strings.Index(body, `id="row-claude-customThing"`):]
		row = row[:strings.Index(row, "</div>")]
		// A fragment-only key is flagged Repo Only (the precise form of the
		// old generic "overridden") and shows the repo value (D1b).
		assert.Contains(t, row, "Repo Only")
		assert.Contains(t, row, "repo: true")
		assert.Contains(t, row, "undocumented")
	})

	t.Run("group-summary-counts-overridden", func(t *testing.T) {
		server := newServer(t, `{"model": "sonnet"}`, `{"model": "opus"}`)
		body := getConfig(t, server)
		// model is category "model & reasoning"; its summary carries the count.
		summaryAt := strings.Index(body, "1 overridden")
		require.Greater(t, summaryAt, 0)
		assert.Less(t, summaryAt, strings.Index(body, `id="row-claude-model"`))
	})

	t.Run("available-summary-counts-overridden", func(t *testing.T) {
		server := newServer(t, `{}`, `{"model": "opus"}`)
		body := getConfig(t, server)
		availableAt := strings.Index(body, "Show available (")
		require.Greater(t, availableAt, 0)
		summary := body[availableAt:]
		summary = summary[:strings.Index(summary, "</summary>")]
		assert.Contains(t, summary, "1 overridden")
	})

	t.Run("overridden-key-shows-repo-value", func(t *testing.T) {
		server := newServer(t, `{"model": "sonnet"}`, `{"model": "opus"}`)
		body := getConfig(t, server)
		modelRow := body[strings.Index(body, `id="row-claude-model"`):]
		modelRow = modelRow[:strings.Index(modelRow, "</div>")]
		assert.Contains(t, modelRow, "repo: opus")
	})

	t.Run("fragment-only-key-marked-repo-only", func(t *testing.T) {
		server := newServer(t, `{}`, `{"model": "opus"}`)
		body := getConfig(t, server)
		modelRow := body[strings.Index(body, `id="row-claude-model"`):]
		modelRow = modelRow[:strings.Index(modelRow, "</div>")]
		assert.Contains(t, modelRow, "Repo Only")
		assert.Contains(t, modelRow, "repo: opus")
	})

	t.Run("missing-fragment-disables-sync-buttons", func(t *testing.T) {
		server := newServer(t, `{}`, "")
		body := getConfig(t, server)
		assert.Contains(t, body, "repo fragment missing")
		assert.Contains(t, body, "disabled>Sync")
		assert.Contains(t, body, "disabled>Persist overrides")
	})
}

func TestDocHref(t *testing.T) {
	cases := []struct {
		name        string
		explanation string
		source      string
		want        string
	}{
		{"deep-link-with-anchor",
			"Enterprise allowlist. See https://code.claude.com/docs/en/mcp#restriction-options",
			"https://www.schemastore.org/claude-code-settings.json",
			"https://code.claude.com/docs/en/mcp#restriction-options"},
		{"trailing-period-trimmed",
			"See https://code.claude.com/docs/en/statusline.",
			"src",
			"https://code.claude.com/docs/en/statusline"},
		{"no-url-falls-back",
			"A plain description with no link.",
			"https://www.schemastore.org/claude-code-settings.json",
			"https://www.schemastore.org/claude-code-settings.json"},
		{"empty-explanation-falls-back",
			"",
			"https://developers.openai.com/codex/config-reference",
			"https://developers.openai.com/codex/config-reference"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, docHref(tc.explanation, tc.source))
		})
	}
}
