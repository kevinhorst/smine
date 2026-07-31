package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePermissionSettings(t *testing.T) (string, *Server) {
	t.Helper()
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetPermissions(config.Permissions{
		Allow: []string{"Bash(ls *)", "Bash(cat *)"},
		Ask:   []string{"Bash(rm *)"},
	}))
	disabled := config.NewSettings()
	require.NoError(t, disabled.SetPermissions(config.Permissions{
		Allow: []string{"Bash(jq *)"},
	}))
	require.NoError(t, config.Save(settingsPath, main))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled))
	return settingsPath, newTestServer(t, &Options{SettingsPath: settingsPath})
}

func loadPermissions(t *testing.T, settingsPath string) (config.Permissions, config.Permissions) {
	t.Helper()
	main, err := config.Load(settingsPath)
	require.NoError(t, err)
	disabled, err := config.Load(config.DisabledPath(settingsPath))
	require.NoError(t, err)
	mainPerms, err := main.Permissions()
	require.NoError(t, err)
	disabledPerms, err := disabled.Permissions()
	require.NoError(t, err)
	return mainPerms, disabledPerms
}

func TestTogglePermission(t *testing.T) {
	t.Run("allow-to-disabled", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(cat *)"}, mainPerms.Allow)
		assert.Equal(t, []string{"Bash(jq *)", "Bash(ls *)"}, disabledPerms.Allow)
	})

	t.Run("disabled-allow-to-main", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/disabledAllow/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Bash(jq *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Allow)
	})

	t.Run("ask-to-disabled", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/ask/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Empty(t, mainPerms.Ask)
		assert.Equal(t, []string{"Bash(rm *)"}, disabledPerms.Ask)
	})

	t.Run("index-out-of-range-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/99/toggle", nil))
		assert.Equal(t, 400, response.Code)
	})

	t.Run("invalid-list-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/bogus/0/toggle", nil))
		assert.Equal(t, 400, response.Code)
	})
}

func TestPermissionsSectionOverriddenBadge(t *testing.T) {
	perms := `{"permissions": {"allow": ["Bash(ls *)"]}}`

	drifted := newSectionServer(t, perms, `{"permissions": {"allow": []}}`)
	assertSectionBadge(t, drifted, "/api/permissions", "permissions", true)

	clean := newSectionServer(t, perms, perms)
	assertSectionBadge(t, clean, "/api/permissions", "permissions", false)
}

// Rows render alphabetically across main+disabled and stay in place after a
// toggle (D7); the fixture's file order is ls, cat, (disabled) jq.
func TestPermissionsSorted(t *testing.T) {
	assertSorted := func(t *testing.T, body string) {
		t.Helper()
		catAt := strings.Index(body, "Bash(cat *)")
		jqAt := strings.Index(body, "Bash(jq *)")
		lsAt := strings.Index(body, "Bash(ls *)")
		require.Greater(t, catAt, 0)
		require.Greater(t, jqAt, 0)
		require.Greater(t, lsAt, 0)
		assert.Less(t, catAt, jqAt)
		assert.Less(t, jqAt, lsAt)
	}

	_, server := writePermissionSettings(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, 200, response.Code)
	assertSorted(t, response.Body.String())

	// Toggling ls (main index 0) moves it between files but not in the list.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/0/toggle", nil))
	require.Equal(t, 200, response.Code, response.Body.String())
	assertSorted(t, response.Body.String())
}

func TestPermissionMutationTriggersConfigOp(t *testing.T) {
	_, server := writePermissionSettings(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/0/toggle", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
