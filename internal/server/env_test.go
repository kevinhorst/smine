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

// Live: DEBUG shared, LOCAL local, PORT changed; fragment adds REPO as repo-only.
func TestEnvRows(t *testing.T) {
	live := map[string]string{"DEBUG": "1", "LOCAL": "x", "PORT": "7"}
	fragment := map[string]string{"DEBUG": "1", "PORT": "8", "REPO": "y"}

	rows := envRows(live, fragment, true)

	expected := []envRow{
		{Key: "DEBUG", Source: sourceShared, Value: "1"},
		{Key: "LOCAL", Source: sourceLocal, Value: "x"},
		{Key: "PORT", RepoValue: "8", Source: sourceChanged, Value: "7"},
		{Key: "REPO", RepoValue: "y", Source: sourceRepoOnly},
	}
	assert.Equal(t, expected, rows)

	unknown := envRows(live, nil, false)
	require.Len(t, unknown, 3)
	assert.Empty(t, unknown[0].Source)
}

func TestEnvMutationAnswersWithFragment(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/env", url.Values{"key": {"FOO"}, "value": {"1"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Empty(t, response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/env/FOO", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/env", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
