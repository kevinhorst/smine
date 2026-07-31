package server

import (
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

// writeRegistryFixture returns a repos.json path with one repo "demo" at
// targetPath, so context tests can select it in the target dropdown.
func writeRegistryFixture(t *testing.T, targetPath string) string {
	t.Helper()
	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "demo", "path": "` + targetPath + `"}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))
	return reposPath
}

func writeContextFixture(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "style"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "rules"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "AGENTS.md"), []byte("<!-- template marker -->\n# Agents"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "style", "plan.md"), []byte("# Plan Format"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "style", "go.md"), []byte("# Go Style"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "style", "notes.txt"), []byte("plain"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "rules", "aspects.json"),
		[]byte(`[{"name": "INTEG", "scope": "Data integrity"}, {"name": "NAV", "scope": "Navigation"}]`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "rules", "navigation.md"),
		[]byte("**NEVER-NAV-001** `[review]` — Never scan.\n\n* Applies: everywhere.\n"), 0644))
	return src
}

func TestContextIndex(t *testing.T) {
	server := newTestServer(t, &Options{
		ContextDir: writeContextFixture(t),
		ReposPath:  writeRegistryFixture(t, "/tmp/target"),
	})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "<h2>rules</h2>")
	assert.Contains(t, body, "<h2>style</h2>")
	assert.Contains(t, body, "plan.md")
	assert.Contains(t, body, "template marker")
	assert.Contains(t, body, `name="name"`)
	assert.Contains(t, body, `<option value="demo">demo</option>`)
	assert.Contains(t, body, `name="context-dir"`)
	assert.NotContains(t, body, `name="lang"`)
	assert.Contains(t, body, `name="symlink"`)
	assert.Contains(t, body, "<h2>Aspects</h2>")
	assert.Contains(t, body, "Data integrity")
	assert.Contains(t, body, `hx-post="/context/aspects"`)
}

func TestContextAspects(t *testing.T) {
	readAspectsFile := func(t *testing.T, contextDir string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(contextDir, "rules", "aspects.json"))
		require.NoError(t, err)
		return string(data)
	}

	t.Run("add-persists-sorted", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		values := url.Values{"name": {"arch"}, "scope": {"Architecture"}}
		server.Handler().ServeHTTP(response, formPost("/context/aspects", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "ARCH")

		file := readAspectsFile(t, contextDir)
		assert.Contains(t, file, `"ARCH"`)
		assert.Less(t, strings.Index(file, `"ARCH"`), strings.Index(file, `"INTEG"`))
	})

	t.Run("add-duplicate-rejected", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		values := url.Values{"name": {"NAV"}, "scope": {"again"}}
		server.Handler().ServeHTTP(response, formPost("/context/aspects", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "already exists")
	})

	t.Run("add-invalid-name-rejected", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		values := url.Values{"name": {"n0pe!"}, "scope": {"x"}}
		server.Handler().ServeHTTP(response, formPost("/context/aspects", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "2-12 letters")
		assert.NotContains(t, readAspectsFile(t, contextDir), "N0PE")
	})

	t.Run("add-empty-scope-rejected", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/context/aspects", url.Values{"name": {"PERF"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "scope is required")
	})

	t.Run("delete-unused-removes", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/INTEG", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, readAspectsFile(t, contextDir), "INTEG")
	})

	t.Run("delete-in-use-blocked-with-ids", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/NAV", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "in use: NEVER-NAV-001")
		assert.Contains(t, readAspectsFile(t, contextDir), "NAV")
	})

	t.Run("delete-unknown-400", func(t *testing.T) {
		server := newTestServer(t, &Options{ContextDir: writeContextFixture(t)})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/GHOST", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestContextDoc(t *testing.T) {
	server := newTestServer(t, &Options{ContextDir: writeContextFixture(t)})

	t.Run("valid-doc-renders-markdown", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/style/go.md", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "<h1>Go Style</h1>")
		assert.Contains(t, response.Body.String(), "context/style/go.md")
	})

	t.Run("unknown-file-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/style/nope.md", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("on-disk-but-unscanned-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/style/notes.txt", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotContains(t, response.Body.String(), "plain")
	})
}

func TestContextSyncRendersStubOutput(t *testing.T) {
	syncScripts := t.TempDir()
	stub := "#!/bin/sh\necho \"context-sync $*\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(syncScripts, "sync_context.sh"), []byte(stub), 0o755))
	server := newTestServer(t, &Options{
		ContextDir:     writeContextFixture(t),
		ReposPath:      writeRegistryFixture(t, "/tmp/target"),
		SyncScriptsDir: syncScripts,
	})

	t.Run("resolves-name-and-syncs-all-languages", func(t *testing.T) {
		response := httptest.NewRecorder()
		values := url.Values{
			"context-dir": {"docs"},
			"symlink":     {"on"},
			"name":        {"demo"},
		}
		server.Handler().ServeHTTP(response, formPost("/context/sync", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "context-sync --context-dir docs --langs go --symlink /tmp/target")
	})

	t.Run("unknown-name-is-bad-request", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/context/sync", url.Values{"name": {"ghost"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}
