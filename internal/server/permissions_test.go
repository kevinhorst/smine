package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

func postAddPermission(t *testing.T, server *Server, rule string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/add?rule="+url.QueryEscape(rule), nil))
	return response
}

// Fixture (writePermissionSettings): allow ls+cat, ask rm, disabled-allow jq.
func TestAddPermission(t *testing.T) {
	t.Run("new-rule-appended", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(rg *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">allowed</span>")

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Bash(rg *)"}, mainPerms.Allow)
	})

	t.Run("already-allowed-noop", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(ls *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">allowed</span>")

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
		assert.Equal(t, []string{"Bash(jq *)"}, disabledPerms.Allow)
	})

	t.Run("already-ask-noop", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(rm *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">ask</span>")

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(rm *)"}, mainPerms.Ask)
	})

	t.Run("parked-allow-moved-back", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(jq *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Bash(jq *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Allow)
	})

	t.Run("parked-ask-moved-to-allow", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, config.Save(settingsPath, config.NewSettings()))
		disabled := config.NewSettings()
		require.NoError(t, disabled.SetPermissions(config.Permissions{Ask: []string{"Bash(rg *)"}}))
		require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled))
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := postAddPermission(t, server, "Bash(rg *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(rg *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Ask)
	})

	t.Run("empty-rule-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/add", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("non-rule-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "rm -rf /")
		assert.Equal(t, http.StatusBadRequest, response.Code)
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

func TestPermissionRows(t *testing.T) {
	t.Run("marks-shared-local-and-repo-only", func(t *testing.T) {
		main := []string{"Bash(ls *)", "Bash(cat *)"} // ls shared, cat local
		disabled := []string{"Bash(jq *)"}            // jq local (parked)
		frag := []string{"Bash(ls *)", "Bash(rm *)"}  // rm repo-only
		rows := permissionRows(main, disabled, frag, listAllow, listDisabledAllow, true)

		bySource := map[string]permissionRow{}
		for _, row := range rows {
			bySource[row.Value] = row
		}
		assert.Equal(t, permSourceShared, bySource["Bash(ls *)"].Source)
		assert.Equal(t, permSourceLocal, bySource["Bash(cat *)"].Source)
		assert.Equal(t, permSourceLocal, bySource["Bash(jq *)"].Source)
		require.Contains(t, bySource, "Bash(rm *)")
		repoOnly := bySource["Bash(rm *)"]
		assert.Equal(t, permSourceRepoOnly, repoOnly.Source)
		assert.False(t, repoOnly.Enabled)
		assert.Empty(t, repoOnly.List)
	})

	t.Run("unknown-fragment-skips-marking", func(t *testing.T) {
		rows := permissionRows([]string{"Bash(ls *)"}, nil, nil, listAllow, listDisabledAllow, false)
		require.Len(t, rows, 1)
		assert.Empty(t, rows[0].Source)
	})
}

func TestPermissionsLocalAndRepoOnlyMarking(t *testing.T) {
	live := `{"permissions": {"allow": ["Bash(ls *)", "Bash(cat *)"]}}`    // ls shared, cat local
	fragment := `{"permissions": {"allow": ["Bash(ls *)", "Bash(rm *)"]}}` // rm repo-only
	server := newSectionServer(t, live, fragment)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()

	assert.Contains(t, body, "Bash(rm *)")        // repo-only rule surfaced as a row
	assert.Contains(t, body, ">local</span>")     // cat marked local (orange)
	assert.Contains(t, body, ">Repo Only</span>") // rm marked repo-only (grey)
}
