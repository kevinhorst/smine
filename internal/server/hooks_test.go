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

func TestHookNames(t *testing.T) {
	writeHooks := func(t *testing.T, hooks map[string][]config.HookGroup) (string, *Server) {
		t.Helper()
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		main := config.NewSettings()
		require.NoError(t, main.SetHooks(hooks))
		require.NoError(t, config.Save(settingsPath, main))
		return settingsPath, newTestServer(t, &Options{SettingsPath: settingsPath})
	}

	t.Run("named-hook-shows-name-command-demoted", func(t *testing.T) {
		_, server := writeHooks(t, map[string][]config.HookGroup{
			"Notification": {{Hooks: []config.Hook{{Command: "afplay /tmp/funk.aiff", Name: "Funk sound", Type: "command"}}}},
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "<strong>Funk sound</strong>")
		assert.Contains(t, body, `<div class="meta">afplay /tmp/funk.aiff</div>`)
	})

	t.Run("unnamed-hook-falls-back-to-command", func(t *testing.T) {
		_, server := writeHooks(t, map[string][]config.HookGroup{
			"Stop": {{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}},
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "<strong>say done</strong>")
		assert.NotContains(t, body, `<div class="meta">say done</div>`)
	})

	t.Run("toggle-round-trip-preserves-name", func(t *testing.T) {
		settingsPath, server := writeHooks(t, map[string][]config.HookGroup{
			"Notification": {{Hooks: []config.Hook{{Command: "afplay /tmp/funk.aiff", Name: "Funk sound", Type: "command"}}}},
		})

		for _, enabled := range []string{"true", "false"} {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/hooks/Notification/0/toggle?enabled="+enabled, nil)
			server.Handler().ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		}

		// Off then on: the group is back in settings.json with its name.
		updated, err := config.Load(settingsPath)
		require.NoError(t, err)
		hooks, err := updated.Hooks()
		require.NoError(t, err)
		require.Len(t, hooks["Notification"], 1)
		assert.Equal(t, "Funk sound", hooks["Notification"][0].Hooks[0].Name)
	})
}

func TestToggleDisabledHookWithOverlappingIndex(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: "enabled", Type: "command"}}}},
	}))
	disabled := config.NewSettings()
	require.NoError(t, disabled.SetHooks(map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: "disabled", Type: "command"}}}},
	}))
	require.NoError(t, config.Save(settingsPath, main))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled))

	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	request := httptest.NewRequest(http.MethodPost, "/api/hooks/Stop/0/toggle?enabled=false", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	updatedMain, err := config.Load(settingsPath)
	require.NoError(t, err)
	updatedDisabled, err := config.Load(config.DisabledPath(settingsPath))
	require.NoError(t, err)
	mainHooks, err := updatedMain.Hooks()
	require.NoError(t, err)
	disabledHooks, err := updatedDisabled.Hooks()
	require.NoError(t, err)
	assert.Equal(t, "enabled", mainHooks["Stop"][0].Hooks[0].Command)
	assert.Equal(t, "disabled", mainHooks["Stop"][1].Hooks[0].Command)
	assert.Empty(t, disabledHooks["Stop"])
}

func TestHooksSectionOverriddenBadge(t *testing.T) {
	hooks := `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]}}`

	drifted := newSectionServer(t, hooks, `{}`)
	assertSectionBadge(t, drifted, "/api/hooks", "hooks", true)

	clean := newSectionServer(t, hooks, hooks)
	assertSectionBadge(t, clean, "/api/hooks", "hooks", false)
}

// Rows sort by (event, name) regardless of array order (D7).
func TestHooksSorted(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(map[string][]config.HookGroup{
		"Stop": {
			{Hooks: []config.Hook{{Command: "cmd", Name: "b-hook", Type: "command"}}},
			{Hooks: []config.Hook{{Command: "cmd", Name: "a-hook", Type: "command"}}},
		},
		"Notification": {
			{Hooks: []config.Hook{{Command: "cmd", Name: "z-hook", Type: "command"}}},
		},
	}))
	require.NoError(t, config.Save(settingsPath, main))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	zAt := strings.Index(body, "z-hook")
	aAt := strings.Index(body, "a-hook")
	bAt := strings.Index(body, "b-hook")
	require.Greater(t, zAt, 0)
	require.Greater(t, aAt, 0)
	require.Greater(t, bAt, 0)
	// Notification event first, then Stop's groups alphabetically.
	assert.Less(t, zAt, aAt)
	assert.Less(t, aAt, bAt)
}

func TestHookMutationTriggersConfigOp(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}},
	}))
	require.NoError(t, config.Save(settingsPath, main))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/hooks/Stop/0/toggle?enabled=true", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	// GETs must never trigger the body re-pull — that would loop the page.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
