package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/trash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stashTestFile writes a file under a temp dir, moves it into the server's trash and returns its path, stamp and rel.
func stashTestFile(t *testing.T, server *Server, content string) (string, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	entry := &trash.Entry{Action: "test stash", Reason: "fixture"}
	_, err := server.trash.Move(path, entry)
	require.NoError(t, err)

	stashes, err := server.trash.List()
	require.NoError(t, err)
	require.Len(t, stashes, 1)
	require.Len(t, stashes[0].Entries, 1)
	return path, stashes[0].Stamp, stashes[0].Entries[0].Rel
}

func TestTrashPage(t *testing.T) {
	t.Run("empty-store-renders-empty-lines", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/trash", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "<h1>Trash</h1>")
		assert.Contains(t, body, "The trash is empty.")
		assert.Contains(t, body, "No journal lines.")
		assert.NotContains(t, body, "purge")
	})

	t.Run("stash-and-journal-render", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		_, stamp, rel := stashTestFile(t, server, "old notes")
		note := &trash.Entry{Action: "environment forget", Kind: trash.KindForgotten, Path: "install.json#SKILL_X", Reason: "value"}
		require.NoError(t, server.trash.Note(note))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/trash", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.NotContains(t, body, "The trash is empty.")
		assert.Contains(t, body, `hx-post="/trash/`+stamp+`/restore"`)
		assert.Contains(t, body, `<input type="hidden" name="rel" value="`+rel+`">`)
		assert.Contains(t, body, "9 B")
		assert.Contains(t, body, `name="replace" value="on"`)
		assert.Contains(t, body, "replace the current file (it is stashed first)")
		assert.Contains(t, body, "<td>test stash</td>")
		assert.Contains(t, body, "<td>removed</td>")
		assert.Contains(t, body, "<td>environment forget</td>")
		assert.Contains(t, body, "<td>forgotten</td>")
		assert.NotContains(t, body, "No journal lines.")
	})

	t.Run("overview-corner-links-trash-for-developers", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<a class="trash-corner" href="/trash"`)
		assert.NotContains(t, body, `>Trash</a>`, "the nav carries no Trash tab")
	})
}

func TestTrashRestore(t *testing.T) {
	t.Run("restores-stashed-file", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		path, stamp, rel := stashTestFile(t, server, "old notes")

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/trash/"+stamp+"/restore", url.Values{"rel": {rel}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "restored "+rel+" from "+stamp)
		assert.Equal(t, "trash-op", response.Header().Get("HX-Trigger"))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "old notes", string(data))

		journal, err := server.trash.Tail(journalTailLimit)
		require.NoError(t, err)
		require.NotEmpty(t, journal)
		assert.Equal(t, trash.KindRestored, journal[0].Kind)
		assert.Equal(t, path, journal[0].Path)
	})

	t.Run("refuses-present-target", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		path, stamp, rel := stashTestFile(t, server, "old notes")
		require.NoError(t, os.WriteFile(path, []byte("new notes"), 0o644))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/trash/"+stamp+"/restore", url.Values{"rel": {rel}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "restore target exists")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "new notes", string(data))
	})

	t.Run("replaces-with-tick-and-stashes-current", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		path, stamp, rel := stashTestFile(t, server, "old notes")
		require.NoError(t, os.WriteFile(path, []byte("new notes"), 0o644))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/trash/"+stamp+"/restore", url.Values{"rel": {rel}, "replace": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "Failed")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "old notes", string(data))

		stashes, err := server.trash.List()
		require.NoError(t, err)
		require.Len(t, stashes, 1)
		require.Len(t, stashes[0].Entries, 1)
		assert.NotEqual(t, stamp, stashes[0].Stamp)
		stashed, err := os.ReadFile(filepath.Join(filepath.Dir(server.settingsPath), "smine", trash.TrashDir, stashes[0].Stamp, filepath.FromSlash(stashes[0].Entries[0].Rel)))
		require.NoError(t, err)
		assert.Equal(t, "new notes", string(stashed))

		journal, err := server.trash.Tail(journalTailLimit)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(journal), 2)
		assert.Equal(t, trash.KindRestored, journal[0].Kind)
		assert.Equal(t, trash.KindRemoved, journal[1].Kind)
	})

	t.Run("missing-rel-is-bad-request", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/trash/2026-01-01T00-00-00.000000000Z/restore", url.Values{}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}
