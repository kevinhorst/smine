package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadEnv(t *testing.T, settingsPath string) map[string]string {
	t.Helper()
	settings, err := config.Load(settingsPath)
	require.NoError(t, err)
	env, err := settings.Env()
	require.NoError(t, err)
	return env
}

func TestEnv(t *testing.T) {
	t.Run("set-new-key", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {"FOO"}, "value": {"1"}}))
		require.Equal(t, 200, response.Code, response.Body.String())

		assert.Equal(t, map[string]string{"FOO": "1"}, loadEnv(t, settingsPath))
	})

	t.Run("overwrite-key", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {"FOO"}, "value": {"1"}}))
		require.Equal(t, 200, response.Code)
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {"FOO"}, "value": {"2"}}))
		require.Equal(t, 200, response.Code)

		assert.Equal(t, map[string]string{"FOO": "2"}, loadEnv(t, settingsPath))
	})

	t.Run("delete-key", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {"FOO"}, "value": {"1"}}))
		require.Equal(t, 200, response.Code)
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/env/FOO", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		assert.Empty(t, loadEnv(t, settingsPath))
	})

	t.Run("empty-key-400", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {""}, "value": {"1"}}))
		assert.Equal(t, 400, response.Code)
	})
}

func TestEnvSectionOverriddenBadge(t *testing.T) {
	env := `{"env": {"DEBUG": "1"}}`

	drifted := newSectionServer(t, env, `{}`)
	assertSectionBadge(t, drifted, "/api/env", "env", true)

	clean := newSectionServer(t, env, env)
	assertSectionBadge(t, clean, "/api/env", "env", false)
}

func TestEnvMutationTriggersConfigOp(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {"FOO"}, "value": {"1"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/env/FOO", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/env", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
