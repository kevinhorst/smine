package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsIndex(t *testing.T) {
	server := newTestServer(t, &Options{ReposPath: writeRegistryFixture(t, "/tmp/target")})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "Prune JetBrains recent projects")
	assert.Contains(t, body, `hx-post="/tools/prune-jetbrains"`)
	assert.NotContains(t, body, "secretscan")
	assert.Contains(t, body, `<a href="/tools" class="active">Tools</a>`)
}

func TestToolsPruneJetbrains(t *testing.T) {
	scriptsDir := t.TempDir()
	stub := "#!/bin/sh\necho \"prune $*\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "prune_jetbrains_recent_projects.sh"), []byte(stub), 0o755))
	server := newTestServer(t, &Options{WorktreeScriptsDir: scriptsDir})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/tools/prune-jetbrains", url.Values{"dry-run": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "prune ")
	assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
}
