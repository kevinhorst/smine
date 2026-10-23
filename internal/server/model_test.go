package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const availableModelsSettings = `{"availableModels": ["claude-opus-5-5[1m]", "claude-sonnet-5", "claude-sonnet-5[1m]", "claude-fable-5"]}`

// availableModelsSettingsPath writes a live settings file carrying the
// availableModels fixture and returns its path.
func availableModelsSettingsPath(t *testing.T) string {
	t.Helper()
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(availableModelsSettings), 0o644))
	return settingsPath
}

func TestModelSectionOverriddenBadge(t *testing.T) {
	model := `{"model": "opus"}`

	drifted := newSectionServer(t, model, `{"model": "sonnet"}`)
	assertSectionBadge(t, drifted, "/api/model", "model", true)

	clean := newSectionServer(t, model, model)
	assertSectionBadge(t, clean, "/api/model", "model", false)
}

func TestModel(t *testing.T) {
	t.Run("set-model", func(t *testing.T) {
		settingsPath := availableModelsSettingsPath(t)
		server := newTestServer(t, &Options{SettingsPath: settingsPath})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/model", url.Values{"model": {"claude-sonnet-5"}}))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), `<option value="claude-fable-5"`)
		assert.NotContains(t, response.Body.String(), `<option value="opus"`)

		settings, err := config.Load(settingsPath)
		require.NoError(t, err)
		model, err := settings.Model()
		require.NoError(t, err)
		assert.Equal(t, "claude-sonnet-5", model)
	})

	t.Run("alias-not-available", func(t *testing.T) {
		server := newTestServer(t, &Options{SettingsPath: availableModelsSettingsPath(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/model", url.Values{"model": {"haiku"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "does not resolve against availableModels")
	})

	t.Run("no-available-models-admits-any", func(t *testing.T) {
		server := newTestServer(t, &Options{SettingsPath: filepath.Join(t.TempDir(), "settings.json")})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/model", url.Values{"model": {"claude-sonnet-5"}}))
		assert.Equal(t, http.StatusOK, response.Code)
	})
}

func TestModelMutationAnswersWithFragment(t *testing.T) {
	server := newTestServer(t, &Options{SettingsPath: availableModelsSettingsPath(t)})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/model", url.Values{"model": {"claude-sonnet-5"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Empty(t, response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/model", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
