package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMCPFixtures: claude.json registers alpha+beta; the live list disables
// beta plus the unregistered connector name "ghost"; the sidecar still holds
// a leftover shuttle key that toggles must clear (D5).
func writeMCPFixtures(t *testing.T) (string, *Server) {
	t.Helper()
	dir := t.TempDir()

	claudeJsonPath := filepath.Join(dir, ".claude.json")
	claudeJson := `{"mcpServers": {"alpha": {}, "beta": {}}}`
	require.NoError(t, os.WriteFile(claudeJsonPath, []byte(claudeJson), 0644))

	settingsPath := filepath.Join(dir, "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetDisabledMcpjsonServers([]string{"beta", "ghost"}))
	disabled := config.NewSettings()
	require.NoError(t, disabled.SetDisabledMcpjsonServers([]string{"leftover"}))
	require.NoError(t, config.Save(settingsPath, main))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled))

	server := newTestServer(t, &Options{ClaudeJsonPath: claudeJsonPath, SettingsPath: settingsPath})
	return settingsPath, server
}

func loadMCPLists(t *testing.T, settingsPath string) ([]string, []string) {
	t.Helper()
	main, err := config.Load(settingsPath)
	require.NoError(t, err)
	disabled, err := config.Load(config.DisabledPath(settingsPath))
	require.NoError(t, err)
	mainList, err := main.DisabledMcpjsonServers()
	require.NoError(t, err)
	disabledList, err := disabled.DisabledMcpjsonServers()
	require.NoError(t, err)
	return mainList, disabledList
}

func TestGetMCP(t *testing.T) {
	t.Run("claude-json-servers-listed-with-state", func(t *testing.T) {
		_, server := writeMCPFixtures(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
		require.Equal(t, http.StatusOK, response.Code)

		body := response.Body.String()
		assert.Contains(t, body, "alpha")
		assert.Contains(t, body, "beta")
		// Disabled-list name not registered in claude.json still shows.
		assert.Contains(t, body, "ghost")
		// The sidecar shuttle no longer feeds the list.
		assert.NotContains(t, body, "leftover")
		// Disabled rows render as normal cards, not dimmed.
		assert.NotContains(t, body, "card-disabled")
	})

	t.Run("unreadable-claude-json-degrades", func(t *testing.T) {
		settingsPath, _ := writeMCPFixtures(t)
		broken := filepath.Join(t.TempDir(), ".claude.json")
		require.NoError(t, os.WriteFile(broken, []byte("{not json"), 0644))
		server := newTestServer(t, &Options{ClaudeJsonPath: broken, SettingsPath: settingsPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
		require.Equal(t, http.StatusOK, response.Code)

		body := response.Body.String()
		assert.Contains(t, body, "McpServerNames")
		// Disabled-list names still render.
		assert.Contains(t, body, "ghost")
	})
}

func TestMCPSectionOverriddenBadge(t *testing.T) {
	list := `{"disabledMcpjsonServers": ["clickup"]}`

	drifted := newSectionServer(t, list, `{}`)
	assertSectionBadge(t, drifted, "/api/mcp", "mcp", true)

	clean := newSectionServer(t, list, list)
	assertSectionBadge(t, clean, "/api/mcp", "mcp", false)
}

func TestToggleMCP(t *testing.T) {
	t.Run("enabled-to-disabled", func(t *testing.T) {
		settingsPath, server := writeMCPFixtures(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/mcp/alpha/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainList, disabledList := loadMCPLists(t, settingsPath)
		assert.Equal(t, []string{"beta", "ghost", "alpha"}, mainList)
		// The leftover shuttle key is cleared on toggle.
		assert.Empty(t, disabledList)
	})

	t.Run("disabled-to-enabled", func(t *testing.T) {
		settingsPath, server := writeMCPFixtures(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/mcp/beta/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainList, disabledList := loadMCPLists(t, settingsPath)
		assert.Equal(t, []string{"ghost"}, mainList)
		assert.Empty(t, disabledList)
	})
}

func TestMCPMutationTriggersConfigOp(t *testing.T) {
	_, server := writeMCPFixtures(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/mcp/alpha/toggle", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
