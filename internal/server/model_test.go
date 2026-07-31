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

func TestModelSectionOverriddenBadge(t *testing.T) {
	model := `{"model": "opus"}`

	drifted := newSectionServer(t, model, `{"model": "sonnet"}`)
	assertSectionBadge(t, drifted, "/api/model", "model", true)

	clean := newSectionServer(t, model, model)
	assertSectionBadge(t, clean, "/api/model", "model", false)
}

func TestModel(t *testing.T) {
	t.Run("set-model", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		server := newTestServer(t, &Options{SettingsPath: settingsPath})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/model", url.Values{"model": {"opus"}}))
		require.Equal(t, 200, response.Code, response.Body.String())

		settings, err := config.Load(settingsPath)
		require.NoError(t, err)
		model, err := settings.Model()
		require.NoError(t, err)
		assert.Equal(t, "opus", model)
	})
}

func TestModelMutationTriggersConfigOp(t *testing.T) {
	server := newTestServer(t, &Options{SettingsPath: filepath.Join(t.TempDir(), "settings.json")})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/model", url.Values{"model": {"opus"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/model", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}
