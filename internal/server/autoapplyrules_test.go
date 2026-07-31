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

func TestAutoApplyRules(t *testing.T) {
	t.Run("save-then-get-roundtrip", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "rules.md")
		require.NoError(t, os.WriteFile(rulesPath, []byte("# Rules\n"), 0o644))
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/auto-apply-rules", url.Values{"content": {"# Rules\n- new rule\n"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "true", response.Header().Get("HX-Refresh"))
		saved, err := os.ReadFile(rulesPath)
		require.NoError(t, err)
		assert.Equal(t, "# Rules\n- new rule\n", string(saved))

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?tab=auto-apply", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "- new rule")
		assert.Contains(t, body, "<textarea")
	})

	t.Run("oversize-rejected", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "rules.md")
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath})

		oversize := strings.Repeat("a", maxAutoApplyRulesBytes+1)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/auto-apply-rules", url.Values{"content": {oversize}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		_, err := os.Stat(rulesPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("missing-file-page-renders-error", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "absent.md")
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?tab=auto-apply", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "no such file")
		assert.Contains(t, body, "<textarea")
	})
}
