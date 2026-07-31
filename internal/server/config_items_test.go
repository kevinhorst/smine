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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readSettingsArray(t *testing.T, path, key string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var file map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &file))
	raw, ok := file[key]
	if !ok {
		return nil
	}
	var items []string
	require.NoError(t, json.Unmarshal(raw, &items))
	return items
}

func TestConfigItems(t *testing.T) {
	newServer := func(t *testing.T, initial string) (string, *Server) {
		t.Helper()
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		if initial != "" {
			require.NoError(t, os.WriteFile(settingsPath, []byte(initial), 0644))
		}
		return settingsPath, newTestServer(t, &Options{SettingsPath: settingsPath})
	}

	t.Run("add-to-unset-key-creates-array", func(t *testing.T) {
		settingsPath, server := newServer(t, "")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response,
			formPost("/api/config/claude/disabledMcpjsonServers/items", url.Values{"value": {"x"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, []string{"x"}, readSettingsArray(t, settingsPath, "disabledMcpjsonServers"))
	})

	t.Run("add-appends-to-existing", func(t *testing.T) {
		settingsPath, server := newServer(t, `{"disabledMcpjsonServers": ["a"]}`)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response,
			formPost("/api/config/claude/disabledMcpjsonServers/items", url.Values{"value": {"b"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, []string{"a", "b"}, readSettingsArray(t, settingsPath, "disabledMcpjsonServers"))
	})

	t.Run("remove-by-index", func(t *testing.T) {
		settingsPath, server := newServer(t, `{"disabledMcpjsonServers": ["a", "b"]}`)
		request := httptest.NewRequest(http.MethodDelete, "/api/config/claude/disabledMcpjsonServers/items/0", nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, []string{"b"}, readSettingsArray(t, settingsPath, "disabledMcpjsonServers"))
	})

	t.Run("remove-out-of-range-404", func(t *testing.T) {
		_, server := newServer(t, `{"disabledMcpjsonServers": ["a"]}`)
		request := httptest.NewRequest(http.MethodDelete, "/api/config/claude/disabledMcpjsonServers/items/5", nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("undocumented-key-400", func(t *testing.T) {
		_, server := newServer(t, "")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response,
			formPost("/api/config/claude/zzCustom/items", url.Values{"value": {"x"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("non-array-documented-key-400", func(t *testing.T) {
		_, server := newServer(t, "")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response,
			formPost("/api/config/claude/model/items", url.Values{"value": {"x"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestConfigItemsCodex(t *testing.T) {
	newServer := func(t *testing.T, initial string) (string, *Server) {
		t.Helper()
		codexPath := filepath.Join(t.TempDir(), "config.toml")
		if initial != "" {
			require.NoError(t, os.WriteFile(codexPath, []byte(initial), 0644))
		}
		return codexPath, newTestServer(t, &Options{CodexPath: codexPath})
	}

	t.Run("add-to-unset-array", func(t *testing.T) {
		codexPath, server := newServer(t, "model = \"gpt-5.5\"\n")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response,
			formPost("/api/config/codex/project_root_markers/items", url.Values{"value": {".git"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		data, err := os.ReadFile(codexPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), `project_root_markers = [".git"]`)
	})

	t.Run("add-preserves-other-keys", func(t *testing.T) {
		codexPath, server := newServer(t, "model = \"gpt-5.5\"\nproject_root_markers = [\".git\"]\n")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response,
			formPost("/api/config/codex/project_root_markers/items", url.Values{"value": {"go.mod"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		data, err := os.ReadFile(codexPath)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, `model = "gpt-5.5"`)
		assert.Contains(t, content, `project_root_markers = [".git", "go.mod"]`)
	})

	t.Run("remove-by-index", func(t *testing.T) {
		codexPath, server := newServer(t, "project_root_markers = [\".git\", \"go.mod\"]\n")
		request := httptest.NewRequest(http.MethodDelete, "/api/config/codex/project_root_markers/items/0", nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		data, err := os.ReadFile(codexPath)
		require.NoError(t, err)
		line := ""
		for _, candidate := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(candidate, "project_root_markers") {
				line = candidate
			}
		}
		assert.Equal(t, `project_root_markers = ["go.mod"]`, line)
	})
}

// Items display alphabetically while delete URLs keep the storage index, so
// removing a sorted-first item deletes the right element (D7).
func TestConfigItemsSortedDisplay(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"disabledMcpjsonServers": ["b", "a"]}`), 0644))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/claude", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// "a" (storage index 1) renders before "b" (storage index 0).
	aDeleteAt := strings.Index(body, "disabledMcpjsonServers/items/1")
	bDeleteAt := strings.Index(body, "disabledMcpjsonServers/items/0")
	require.Greater(t, aDeleteAt, 0)
	require.Greater(t, bDeleteAt, 0)
	assert.Less(t, aDeleteAt, bDeleteAt)

	// Deleting via the sorted-first row's URL removes "a", not "b".
	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/config/claude/disabledMcpjsonServers/items/1", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, []string{"b"}, readSettingsArray(t, settingsPath, "disabledMcpjsonServers"))
}

func TestConfigItemMutationTriggersConfigOp(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"disabledMcpjsonServers": ["a"]}`), 0644))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response,
		formPost("/api/config/claude/disabledMcpjsonServers/items", url.Values{"value": {"x"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response,
		httptest.NewRequest(http.MethodDelete, "/api/config/claude/disabledMcpjsonServers/items/0", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))
}
