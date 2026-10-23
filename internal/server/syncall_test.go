package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
)

func TestHandleSync(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedContains string
		_expectedMissing  string

		server  *Server
		targets []string
	}

	tests := make([]*testCase, 0)

	// settings-only
	tests = append(tests, &testCase{
		_id:               "settings-only",
		_expectedCode:     http.StatusOK,
		_expectedContains: "== settings",
		_expectedMissing:  "== hooks",

		server:  newSyncServer(t, `{"model": "opus"}`),
		targets: []string{"settings"},
	})

	// install-order-whatever-the-tick-order
	tests = append(tests, &testCase{
		_id:               "install-order-whatever-the-tick-order",
		_expectedCode:     http.StatusOK,
		_expectedContains: "== hooks",
		_expectedMissing:  "== skills",

		server:  newSyncServer(t, `{"model": "opus"}`),
		targets: []string{"hooks", "settings"},
	})

	// failure-stops-the-run
	tests = append(tests, &testCase{
		_id:               "failure-stops-the-run",
		_expectedCode:     http.StatusOK,
		_expectedContains: "Failed to parse",
		_expectedMissing:  "== hooks",

		server:  newSyncServer(t, "{not json"),
		targets: []string{"settings", "hooks"},
	})

	// nothing-ticked-400
	tests = append(tests, &testCase{
		_id:               "nothing-ticked-400",
		_expectedCode:     http.StatusBadRequest,
		_expectedContains: "pick at least one sync target",
		_expectedMissing:  "== ",

		server:  newSyncServer(t, `{"model": "opus"}`),
		targets: nil,
	})

	// unknown-target-400
	tests = append(tests, &testCase{
		_id:               "unknown-target-400",
		_expectedCode:     http.StatusBadRequest,
		_expectedContains: `unknown sync target "context"`,
		_expectedMissing:  "== ",

		server:  newSyncServer(t, `{"model": "opus"}`),
		targets: []string{"settings", "context"},
	})

	// load-error-refuses
	brokenServer := newSyncServer(t, `{"model": "opus"}`)
	writeInstallFile(t, brokenServer.machine, []byte("{not json"))
	tests = append(tests, &testCase{
		_id:               "load-error-refuses",
		_expectedCode:     http.StatusInternalServerError,
		_expectedContains: "recordCache.load",
		_expectedMissing:  "== ",

		server:  brokenServer,
		targets: []string{"settings"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, formPost("/sync", url.Values{"target": test.targets}))
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedContains)
			assert.NotContains(t, response.Body.String(), test._expectedMissing)
		})
	}
}

func TestHandleSync_HooksTargetWritesHookBinaries(t *testing.T) {
	server := newSyncServer(t, `{"model": "opus"}`)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/sync", url.Values{"target": {"hooks"}}))

	require.Equal(t, http.StatusOK, response.Code)
	assert.FileExists(t, filepath.Join(server.machine.ClaudeHome, "hooks", "bin", "sessiontitle"))
	assert.NoFileExists(t, filepath.Join(server.machine.ClaudeHome, "settings.json"))
}

// newSyncServer serves a fixture checkout with baseTemplate as base.json and the session helpers' prebuilt hooks; the
// payload marker makes the engine take bin/<name> instead of building, and the stored record selects the session helpers.
func newSyncServer(t *testing.T, baseTemplate string) *Server {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	repoDir := t.TempDir()
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	files := map[string]string{
		"settings/claude_code/base.json": baseTemplate,
		"bin/" + install.PayloadMarker:   "0.0.0\n",
		"bin/playsound" + exe:            "playsound binary",
		"bin/sessiontitle" + exe:         "sessiontitle binary",
		"bin/skillenv" + exe:             "skillenv binary",
		"bin/skillhelp" + exe:            "skillhelp binary",
	}
	for path, content := range files {
		full := filepath.Join(repoDir, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	server := newTestServer(t, &Options{RepoDir: repoDir, SettingsPath: filepath.Join(t.TempDir(), "settings.json")})
	record := install.FullRecord(server.machine.GOOS)
	record.Components = components.Close([]components.ID{components.ConfigServer, components.SessionHelpers})
	record.Preset = install.PresetCustom
	require.NoError(t, install.Save(server.machine, record))
	return server
}
