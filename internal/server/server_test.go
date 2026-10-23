package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, opts *Options) *Server {
	t.Helper()
	if opts.SettingsPath == "" {
		opts.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	}
	if opts.StateDir == "" {
		opts.StateDir = t.TempDir()
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

// writeTestRecord saves an install.json with preset into a temp Claude home
// and returns the settings path whose dir the server reads it from.
func writeTestRecord(t *testing.T, preset string) string {
	t.Helper()
	claudeHome := t.TempDir()
	env := install.Env{ClaudeHome: claudeHome, GOOS: runtime.GOOS}
	record := install.FullRecord(env.GOOS)
	record.Preset = preset
	require.NoError(t, install.Save(env, record))
	return filepath.Join(claudeHome, "settings.json")
}

func TestNavHidesUnselectedComponents(t *testing.T) {
	type testCase struct {
		_id           string
		_hasBacklog   bool
		_hasContext   bool
		_hasFleet     bool
		_hasProposals bool
		_hasRoutines  bool
		_hasUsage     bool

		recorder *httptest.ResponseRecorder
		request  *http.Request
		server   *Server
	}

	// Full holds only the components this OS can run (heartbeat is darwin-only,
	// routines and mining darwin and windows); Proposals needs Mining.
	hasHeartbeat := components.Available(components.Heartbeat, runtime.GOOS)
	hasProposals := components.Available(components.Mining, runtime.GOOS)
	hasRoutines := components.Available(components.Routines, runtime.GOOS)
	tests := make([]*testCase, 0)

	// minimal-record
	minimalOptions := &Options{SettingsPath: writeTestRecord(t, install.PresetMinimal)}
	tests = append(tests, &testCase{
		_id:           "minimal-record",
		_hasBacklog:   false,
		_hasContext:   false,
		_hasFleet:     false,
		_hasProposals: false,
		_hasRoutines:  false,
		_hasUsage:     false,

		recorder: httptest.NewRecorder(),
		request:  httptest.NewRequest(http.MethodGet, "/", nil),
		server:   newTestServer(t, minimalOptions),
	})

	// full-record
	fullOptions := &Options{SettingsPath: writeTestRecord(t, install.PresetFull)}
	tests = append(tests, &testCase{
		_id:           "full-record",
		_hasBacklog:   hasRoutines,
		_hasContext:   true,
		_hasFleet:     hasHeartbeat,
		_hasProposals: hasProposals,
		_hasRoutines:  hasRoutines,
		_hasUsage:     hasRoutines,

		recorder: httptest.NewRecorder(),
		request:  httptest.NewRequest(http.MethodGet, "/", nil),
		server:   newTestServer(t, fullOptions),
	})

	// no-record
	noRecordOptions := &Options{}
	tests = append(tests, &testCase{
		_id:           "no-record",
		_hasBacklog:   hasRoutines,
		_hasContext:   true,
		_hasFleet:     hasHeartbeat,
		_hasProposals: hasProposals,
		_hasRoutines:  hasRoutines,
		_hasUsage:     hasRoutines,

		recorder: httptest.NewRecorder(),
		request:  httptest.NewRequest(http.MethodGet, "/", nil),
		server:   newTestServer(t, noRecordOptions),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.server.Handler().ServeHTTP(test.recorder, test.request)
			assert.Equal(t, http.StatusOK, test.recorder.Code)
			assert.Equal(t, test._hasBacklog, strings.Contains(test.recorder.Body.String(), `<a href="/backlog" `))
			assert.Equal(t, test._hasContext, strings.Contains(test.recorder.Body.String(), `<a href="/context" `))
			assert.Equal(t, test._hasFleet, strings.Contains(test.recorder.Body.String(), `<a href="/fleet" `))
			assert.Equal(t, test._hasProposals, strings.Contains(test.recorder.Body.String(), `<a href="/proposals" `))
			assert.Equal(t, test._hasRoutines, strings.Contains(test.recorder.Body.String(), `<a href="/routines" `))
			assert.Equal(t, test._hasUsage, strings.Contains(test.recorder.Body.String(), `<a href="/usage" `))
		})
	}
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

func TestNavCasual(t *testing.T) {
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
	hasRoutines := components.Available(components.Routines, runtime.GOOS)
	hasProposals := components.Available(components.Mining, runtime.GOOS)
	assert.Equal(t, hasRoutines, strings.Contains(body, `href="/usage"`))
	assert.Equal(t, hasRoutines, strings.Contains(body, ">Nutzung</a>"))
	assert.Equal(t, hasProposals, strings.Contains(body, ">Vorschläge</a>"))
	assert.Contains(t, body, ">Projekte</a>")
	assert.NotContains(t, body, `href="/context"`)
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
	assert.Equal(t, components.Available(components.Routines, runtime.GOOS), strings.Contains(body, ">Usage</a>"))
	assert.Contains(t, body, `href="/context"`)
	assert.Contains(t, body, `href="/repos"`)
	assert.NotContains(t, body, `href="/docs/checklist"`)
}

func TestRelTime(t *testing.T) {
	now := time.Now()
	assert.Equal(t, "just now", relTime(now.Add(-30*time.Second)))
	assert.Equal(t, "5m ago", relTime(now.Add(-5*time.Minute)))
	assert.Equal(t, "3h ago", relTime(now.Add(-3*time.Hour)))
	old := now.Add(-48 * time.Hour)
	assert.Equal(t, old.Format("2006-01-02 15:04"), relTime(old))
}
