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

func writeStubSyncScripts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sync_skills.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	return dir
}

func TestServer_HandleProfilePage(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Profile")
	assert.Contains(t, recorder.Body.String(), "style_content")
}

func TestServer_HandleProfileSave(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedDevMode bool
		_expectedStatus  int
		_shouldPersist   bool

		form   url.Values
		server *Server
	}

	tests := make([]*testCase, 0)

	// pass-valid-developer
	dir := t.TempDir()
	server := newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
		SyncScriptsDir:   writeStubSyncScripts(t),
	})
	tests = append(tests, &testCase{
		_id:             "pass-valid-developer",
		_expectedStatus: http.StatusSeeOther,
		_shouldPersist:  false,

		form:   url.Values{"audience": {""}, "language": {"en"}},
		server: server,
	})

	// pass-valid-casual
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
		SyncScriptsDir:   writeStubSyncScripts(t),
	})
	tests = append(tests, &testCase{
		_id:             "pass-valid-casual",
		_expectedStatus: http.StatusSeeOther,
		_shouldPersist:  true,

		form:   url.Values{"audience": {"casual"}, "language": {"de"}},
		server: server,
	})

	// pass-developer-devmode
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
		SyncScriptsDir:   writeStubSyncScripts(t),
	})
	tests = append(tests, &testCase{
		_id:              "pass-developer-devmode",
		_expectedDevMode: true,
		_expectedStatus:  http.StatusSeeOther,
		_shouldPersist:   true,

		form:   url.Values{"audience": {""}, "language": {"en"}, "dev_mode": {"on"}},
		server: server,
	})

	// pass-casual-devmode-forced-off
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
		SyncScriptsDir:   writeStubSyncScripts(t),
	})
	tests = append(tests, &testCase{
		_id:              "pass-casual-devmode-forced-off",
		_expectedDevMode: false,
		_expectedStatus:  http.StatusSeeOther,
		_shouldPersist:   true,

		form:   url.Values{"audience": {"casual"}, "language": {"de"}, "dev_mode": {"on"}},
		server: server,
	})

	// fail-unknown-audience
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
		SyncScriptsDir:   writeStubSyncScripts(t),
	})
	tests = append(tests, &testCase{
		_id:             "fail-unknown-audience",
		_expectedStatus: http.StatusBadRequest,
		_shouldPersist:  false,

		form:   url.Values{"audience": {"manager"}, "language": {"en"}},
		server: server,
	})

	// fail-unsupported-language
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
		SyncScriptsDir:   writeStubSyncScripts(t),
	})
	tests = append(tests, &testCase{
		_id:             "fail-unsupported-language",
		_expectedStatus: http.StatusBadRequest,
		_shouldPersist:  false,

		form:   url.Values{"audience": {""}, "language": {"fr"}},
		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/profile", test.form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			_, statErr := os.Stat(test.server.presentation.path)
			assert.Equal(t, test._shouldPersist, statErr == nil)
			assert.Equal(t, test._expectedDevMode, test.server.presentation.isDevMode())
		})
	}
}

func TestServer_HandleProfileStyleSave(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedSaved  bool
		_expectedStatus int

		content string
		server  *Server
	}

	tests := make([]*testCase, 0)

	// saves-content
	dir := t.TempDir()
	server := newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})
	tests = append(tests, &testCase{
		_id:             "saves-content",
		_expectedSaved:  true,
		_expectedStatus: http.StatusSeeOther,

		content: "- Answer tersely.\n",
		server:  server,
	})

	// rejects-oversized
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})
	tests = append(tests, &testCase{
		_id:             "rejects-oversized",
		_expectedSaved:  false,
		_expectedStatus: http.StatusBadRequest,

		content: strings.Repeat("x", maxStyleProfileBytes+1),
		server:  server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			form := url.Values{"style_content": {test.content}}
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/profile/style", form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			saved, err := test.server.presentation.styleContent()
			require.NoError(t, err)
			assert.Equal(t, test._expectedSaved, saved == test.content)
		})
	}
}
