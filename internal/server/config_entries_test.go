package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entriesMux routes the entry endpoints until server.go wires them.
func entriesMux(server *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/config/{target}/{key}/entries", server.handleConfigEntrySet)
	mux.HandleFunc("DELETE /api/config/{target}/{key}/entries/{name}", server.handleConfigEntryRemove)
	return mux
}

// readSettingsCompact returns one top-level settings value compacted, keeping its key order.
func readSettingsCompact(t *testing.T, path, key string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var file map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &file))
	raw, ok := file[key]
	if !ok {
		return ""
	}
	var compacted bytes.Buffer
	require.NoError(t, json.Compact(&compacted, raw))
	return compacted.String()
}

func TestConfigEntries(t *testing.T) {
	newServer := func(t *testing.T, initial string) (string, *http.ServeMux) {
		t.Helper()
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		if initial != "" {
			require.NoError(t, os.WriteFile(settingsPath, []byte(initial), 0644))
		}
		return settingsPath, entriesMux(newTestServer(t, &Options{SettingsPath: settingsPath}))
	}

	t.Run("set-on-unset-key-creates-map", func(t *testing.T) {
		settingsPath, mux := newServer(t, "")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/claude/skillOverrides/entries", url.Values{"name": {"fdesign"}, "value": {"off"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.JSONEq(t, `{"fdesign": "off"}`, readSettingsCompact(t, settingsPath, "skillOverrides"))
	})

	t.Run("set-changes-one-entry-keeps-order", func(t *testing.T) {
		settingsPath, mux := newServer(t, `{"skillOverrides": {"b": "on", "a": "off"}}`)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/claude/skillOverrides/entries", url.Values{"name": {"a"}, "value": {"name-only"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		overrides := readSettingsCompact(t, settingsPath, "skillOverrides")
		assert.JSONEq(t, `{"b":"on","a":"name-only"}`, overrides)
		assert.Less(t, strings.Index(overrides, `"b"`), strings.Index(overrides, `"a"`))
	})

	t.Run("set-rejects-unknown-value-400", func(t *testing.T) {
		settingsPath, mux := newServer(t, `{"skillOverrides": {"a": "off"}}`)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/claude/skillOverrides/entries", url.Values{"name": {"a"}, "value": {"hidden"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.JSONEq(t, `{"a":"off"}`, readSettingsCompact(t, settingsPath, "skillOverrides"))
	})

	t.Run("set-rejects-empty-name-400", func(t *testing.T) {
		settingsPath, mux := newServer(t, "")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/claude/skillOverrides/entries", url.Values{"name": {"  "}, "value": {"off"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.NoFileExists(t, settingsPath)
	})

	t.Run("remove-entry", func(t *testing.T) {
		settingsPath, mux := newServer(t, `{"skillOverrides": {"b": "on", "a": "off"}}`)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/config/claude/skillOverrides/entries/b", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.JSONEq(t, `{"a":"off"}`, readSettingsCompact(t, settingsPath, "skillOverrides"))
	})

	t.Run("remove-missing-entry-404", func(t *testing.T) {
		_, mux := newServer(t, `{"skillOverrides": {"a": "off"}}`)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/config/claude/skillOverrides/entries/zz", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("codex-target-404", func(t *testing.T) {
		_, mux := newServer(t, "")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/codex/skillOverrides/entries", url.Values{"name": {"a"}, "value": {"off"}}))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("non-map-key-400", func(t *testing.T) {
		_, mux := newServer(t, "")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/claude/env/entries", url.Values{"name": {"A"}, "value": {"off"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("mutation-answers-with-fragment", func(t *testing.T) {
		_, mux := newServer(t, `{"skillOverrides": {"a": "off"}}`)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,
			formPost("/api/config/claude/skillOverrides/entries", url.Values{"name": {"b"}, "value": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))

		response = httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/config/claude/skillOverrides/entries/a", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))
	})
}
