package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, opts *Options) *Server {
	t.Helper()
	if opts.SettingsPath == "" {
		opts.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	}
	server, err := New(opts)
	require.NoError(t, err)
	return server
}

func formPost(path string, values url.Values) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestNavPeekLink(t *testing.T) {
	server := newTestServer(t, &Options{PeekDashboardURL: "http://127.0.0.1:4243/"})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `<a href="http://127.0.0.1:4243/">Peek</a>`)
}

func TestNavPeekLink_Disabled(t *testing.T) {
	server := newTestServer(t, &Options{})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), ">Peek</a>")
}

func TestNavNonDeveloper(t *testing.T) {
	server := newTestServer(t, &Options{
		PeekDashboardURL: "http://127.0.0.1:4243/",
		PresentationPath: writeGermanProfile(t),
	})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, `lang="de"`)
	assert.Contains(t, body, ">Übersicht</a>")
	assert.Contains(t, body, ">Sitzungen</a>")
	assert.Contains(t, body, ">Vorschläge</a>")
	assert.NotContains(t, body, `href="/context"`)
	assert.NotContains(t, body, `href="/repos"`)
	assert.NotContains(t, body, `href="/scripts/skills"`)
	assert.NotContains(t, body, `href="/config/claude"`)
	assert.NotContains(t, body, ">Peek</a>")
}

func TestNavDefaultProfile(t *testing.T) {
	server := newTestServer(t, &Options{})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, `lang="en"`)
	assert.Contains(t, body, ">Overview</a>")
	assert.Contains(t, body, `href="/context"`)
	assert.Contains(t, body, `href="/repos"`)
	assert.Contains(t, body, `href="/docs/checklist"`)
}
