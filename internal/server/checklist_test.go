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

const checklistFixture = "# Checklist\n" +
	"\n" +
	"**Status tags:** `[Untested]` `[Adopted]`\n" +
	"\n" +
	"| # | Item | Effort | Status |\n" +
	"|---|------|--------|--------|\n" +
	"| 8 | Foo | low | `[Open]` |\n" +
	"\n" +
	"## 8. Foo &nbsp; `[Open]`\n" +
	"\n" +
	"Body text.\n"

const checklistTabsFixture = "# Checklist\n" +
	"\n" +
	"**Status tags:** `[Done]` `[Open]`\n" +
	"\n" +
	"| # | Item | Effort | Status |\n" +
	"|---|------|--------|--------|\n" +
	"| 1 | Finished item | low | `[Done]` |\n" +
	"| 2 | Pending item | low | `[Open]` |\n" +
	"\n" +
	"## 1. Finished item &nbsp; `[Done]`\n" +
	"\n" +
	"Done body.\n" +
	"\n" +
	"## 2. Pending item &nbsp; `[Open]`\n" +
	"\n" +
	"Open body.\n"

func TestChecklistTabs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checklist.md")
	require.NoError(t, os.WriteFile(path, []byte(checklistTabsFixture), 0644))
	server := newTestServer(t, &Options{ChecklistPath: path})

	t.Run("default-open-excludes-done", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs/checklist", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Pending item")
		assert.NotContains(t, body, "Finished item")
		assert.Contains(t, body, "Not Done (1)")
		assert.Contains(t, body, "Done (1)")
	})

	t.Run("done-tab-shows-only-done", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs/checklist?tab=done", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Finished item")
		assert.NotContains(t, body, "Pending item")
	})
}

func TestChecklistStatus(t *testing.T) {
	writeChecklist := func(t *testing.T, content string) (string, *Server) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "checklist.md")
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		return path, newTestServer(t, &Options{ChecklistPath: path})
	}

	t.Run("status-changed", func(t *testing.T) {
		path, server := writeChecklist(t, checklistFixture)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/checklist/8/status", url.Values{"status": {"Adopted"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		// Full page reload instead of a fragment (D6).
		assert.Equal(t, "true", response.Header().Get("HX-Refresh"))
		assert.Empty(t, response.Body.String())

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		// Legend + rewritten heading + rewritten overview row.
		assert.Equal(t, 3, strings.Count(string(data), "`[Adopted]`"))
		assert.NotContains(t, string(data), "`[Open]`")
		assert.Contains(t, string(data), "Body text.")
	})

	t.Run("conflict-409-after-heading-rename", func(t *testing.T) {
		renamed := strings.Replace(checklistFixture, "## 8. Foo", "## 9. Foo", 1)
		_, server := writeChecklist(t, renamed)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/checklist/8/status", url.Values{"status": {"Adopted"}}))
		assert.Equal(t, http.StatusConflict, response.Code)
	})

	t.Run("invalid-tag-400", func(t *testing.T) {
		_, server := writeChecklist(t, checklistFixture)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/checklist/8/status", url.Values{"status": {"Bogus"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}
