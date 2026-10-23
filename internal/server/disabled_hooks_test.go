package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/trash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeHookSettings(t *testing.T, hooks map[string][]config.HookGroup) string {
	t.Helper()
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(hooks))
	require.NoError(t, config.Save(settingsPath, main, nil))
	return settingsPath
}

// trashJournal reads the server's journal oldest first as "<kind> <path>" lines; nil when nothing was journaled.
func trashJournal(t *testing.T, server *Server) []string {
	t.Helper()
	entries, err := server.trash.Tail(100)
	require.NoError(t, err)

	var lines []string
	for _, entry := range slices.Backward(entries) {
		lines = append(lines, string(entry.Kind)+" "+entry.Path)
	}
	return lines
}

// lockSettingsDir makes the settings file's dir read-only until the test ends, so the next settings write fails; Windows ignores a dir's mode bits.
func lockSettingsDir(t *testing.T, settingsPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("a read-only dir mode does not block writes on Windows")
	}

	dir := filepath.Dir(settingsPath)
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// newestStash reads the bytes of the server's newest stash; "" when nothing was kept.
func newestStash(t *testing.T, server *Server) string {
	t.Helper()
	entries, err := server.trash.Tail(100)
	require.NoError(t, err)

	at := slices.IndexFunc(entries, func(entry *trash.Entry) bool { return entry.Stash != "" })
	if at < 0 {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(server.settingsPath), "smine", trash.TrashDir, filepath.FromSlash(entries[at].Stash)))
	require.NoError(t, err)
	return string(data)
}

func stopHook(command string) map[string][]config.HookGroup {
	return map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: command, Type: "command"}}}},
	}
}

func loadHooks(t *testing.T, path string) map[string][]config.HookGroup {
	t.Helper()
	settings, err := config.Load(path)
	require.NoError(t, err)
	hooks, err := settings.Hooks()
	require.NoError(t, err)
	return hooks
}

// parkedHooks reads the server's parked store from disk.
func parkedHooks(t *testing.T, server *Server) map[string][]config.HookGroup {
	t.Helper()
	parked, err := server.disabledHooks.Snapshot()
	require.NoError(t, err)
	return parked
}

func parkedStorePath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "hooks.disabled.json")
}

func TestToggleOffMovesGroupToSidecar(t *testing.T) {
	settingsPath := writeHookSettings(t, stopHook("say done"))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/config/hooks/session-helpers/groups/Stop/0/toggle?enabled=true", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	assert.Empty(t, loadHooks(t, settingsPath))
	assert.Len(t, parkedHooks(t, server)["Stop"], 1)
	_, err := os.Stat(parkedStorePath(settingsPath))
	assert.NoError(t, err, "toggle off left no sidecar")
}

func TestToggleOnWritesBackToSettings(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/config/hooks/session-helpers/groups/Stop/0/toggle?enabled=false", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	hooks := loadHooks(t, settingsPath)
	require.Len(t, hooks["Stop"], 1)
	assert.Equal(t, "say done", hooks["Stop"][0].Hooks[0].Command)
	assert.Empty(t, parkedHooks(t, server))
}

func TestToggleMissingEnabledParamIs400(t *testing.T) {
	settingsPath := writeHookSettings(t, stopHook("say done"))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/config/hooks/session-helpers/groups/Stop/0/toggle", nil)
	server.Handler().ServeHTTP(response, request)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRemoveHook(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCode    int
		_expectedEnabled int
		_expectedEvents  int
		_expectedJournal []string
		_expectedParked  int
		_expectedStashed string

		mux          *http.ServeMux
		request      *http.Request
		server       *Server
		settingsPath string
	}

	newRemoveCase := func(t *testing.T) (*Server, *http.ServeMux, string) {
		t.Helper()
		settingsPath := writeHookSettings(t, map[string][]config.HookGroup{
			"Notification": {{Hooks: []config.Hook{{Command: "notify hook", Type: "command"}}}},
			"Stop":         {{Hooks: []config.Hook{{Command: "enabled hook", Type: "command"}}}},
		})
		server := newTestServer(t, &Options{SettingsPath: settingsPath})
		require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "parked hook", Type: "command"}}}))
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /config/hooks/{component}/groups/{event}/{index}", server.handleRemoveHook)
		return server, mux, settingsPath
	}

	tests := make([]*testCase, 0)

	// enabled-row-removed-from-settings
	server, mux, settingsPath := newRemoveCase(t)
	tests = append(tests, &testCase{
		_id:              "enabled-row-removed-from-settings",
		_expectedCode:    http.StatusOK,
		_expectedEnabled: 0,
		_expectedEvents:  1,
		_expectedJournal: []string{"removed " + settingsPath + "#hooks/Stop/enabled hook", "replaced " + settingsPath},
		_expectedParked:  1,
		_expectedStashed: "enabled hook",

		mux:          mux,
		request:      httptest.NewRequest(http.MethodDelete, "/config/hooks/session-helpers/groups/Stop/0?enabled=true", nil),
		server:       server,
		settingsPath: settingsPath,
	})

	// disabled-row-removed-from-sidecar
	server, mux, settingsPath = newRemoveCase(t)
	tests = append(tests, &testCase{
		_id:              "disabled-row-removed-from-sidecar",
		_expectedCode:    http.StatusOK,
		_expectedEnabled: 1,
		_expectedEvents:  2,
		_expectedJournal: []string{"removed " + parkedStorePath(settingsPath) + "#Stop/parked hook", "replaced " + parkedStorePath(settingsPath)},
		_expectedParked:  0,
		_expectedStashed: "parked hook",

		mux:          mux,
		request:      httptest.NewRequest(http.MethodDelete, "/config/hooks/session-helpers/groups/Stop/0?enabled=false", nil),
		server:       server,
		settingsPath: settingsPath,
	})

	// index-out-of-range-404
	server, mux, settingsPath = newRemoveCase(t)
	tests = append(tests, &testCase{
		_id:              "index-out-of-range-404",
		_expectedCode:    http.StatusNotFound,
		_expectedEnabled: 1,
		_expectedEvents:  2,
		_expectedJournal: nil,
		_expectedParked:  1,
		_expectedStashed: "",

		mux:          mux,
		request:      httptest.NewRequest(http.MethodDelete, "/config/hooks/session-helpers/groups/Stop/5?enabled=true", nil),
		server:       server,
		settingsPath: settingsPath,
	})

	// missing-enabled-400
	server, mux, settingsPath = newRemoveCase(t)
	tests = append(tests, &testCase{
		_id:              "missing-enabled-400",
		_expectedCode:    http.StatusBadRequest,
		_expectedEnabled: 1,
		_expectedEvents:  2,
		_expectedJournal: nil,
		_expectedParked:  1,
		_expectedStashed: "",

		mux:          mux,
		request:      httptest.NewRequest(http.MethodDelete, "/config/hooks/session-helpers/groups/Stop/0", nil),
		server:       server,
		settingsPath: settingsPath,
	})

	// unknown-component-404
	server, mux, settingsPath = newRemoveCase(t)
	tests = append(tests, &testCase{
		_id:              "unknown-component-404",
		_expectedCode:    http.StatusNotFound,
		_expectedEnabled: 1,
		_expectedEvents:  2,
		_expectedJournal: nil,
		_expectedParked:  1,
		_expectedStashed: "",

		mux:          mux,
		request:      httptest.NewRequest(http.MethodDelete, "/config/hooks/nope/groups/Stop/0?enabled=true", nil),
		server:       server,
		settingsPath: settingsPath,
	})

	// mutation-answers-with-fragment
	server, mux, settingsPath = newRemoveCase(t)
	tests = append(tests, &testCase{
		_id:              "mutation-answers-with-fragment",
		_expectedCode:    http.StatusOK,
		_expectedEnabled: 1,
		_expectedEvents:  1,
		_expectedJournal: []string{"removed " + settingsPath + "#hooks/Notification/notify hook", "replaced " + settingsPath},
		_expectedParked:  1,
		_expectedStashed: "notify hook",

		mux:          mux,
		request:      httptest.NewRequest(http.MethodDelete, "/config/hooks/session-helpers/groups/Notification/0?enabled=true", nil),
		server:       server,
		settingsPath: settingsPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.mux.ServeHTTP(response, test.request)

			assert.Equal(t, test._expectedCode, response.Code, response.Body.String())
			assert.Empty(t, response.Header().Get("HX-Trigger"))
			assert.Len(t, loadHooks(t, test.settingsPath), test._expectedEvents)
			assert.Len(t, loadHooks(t, test.settingsPath)["Stop"], test._expectedEnabled)
			assert.Len(t, parkedHooks(t, test.server)["Stop"], test._expectedParked)
			assert.Equal(t, test._expectedJournal, trashJournal(t, test.server))
			assert.Contains(t, newestStash(t, test.server), test._expectedStashed)
		})
	}
}

func TestPeekDoesNotWrite(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "parked hook", Type: "command"}}}))
	before, err := os.ReadFile(parkedStorePath(settingsPath))
	require.NoError(t, err)

	group, ok, err := server.disabledHooks.Peek("Stop", 0)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "parked hook", group.Hooks[0].Command)

	_, isMissingOk, err := server.disabledHooks.Peek("Stop", 1)
	require.NoError(t, err)
	assert.False(t, isMissingOk)

	after, err := os.ReadFile(parkedStorePath(settingsPath))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
	assert.Nil(t, trashJournal(t, server))
}

func TestDropPersists(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "first", Type: "command"}}}))
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "second", Type: "command"}}}))

	require.NoError(t, server.disabledHooks.Drop("Stop", 0))
	require.NoError(t, server.disabledHooks.Drop("Stop", 5))

	restored := newTestServer(t, &Options{SettingsPath: settingsPath})
	parked := parkedHooks(t, restored)
	require.Len(t, parked["Stop"], 1)
	assert.Equal(t, "second", parked["Stop"][0].Hooks[0].Command)
	assert.Contains(t, newestStash(t, server), "first")
}

// The engine rewrites the sidecar between two store calls; the second call sees it, so a toggle never resurrects a stale map.
func TestStoreReadsDiskEveryCall(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "first", Type: "command"}}}))

	engine := install.Env{ClaudeHome: filepath.Dir(settingsPath)}
	rewritten := map[string][]config.HookGroup{"Notification": {{Hooks: []config.Hook{{Command: "engine", Type: "command"}}}}}
	require.NoError(t, install.SaveParked(engine, rewritten))

	assert.Equal(t, rewritten, parkedHooks(t, server))
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "second", Type: "command"}}}))
	parked := parkedHooks(t, server)
	assert.Len(t, parked["Notification"], 1, "the add kept the engine's write")
	require.Len(t, parked["Stop"], 1, "the add did not resurrect the overwritten group")
	assert.Equal(t, "second", parked["Stop"][0].Hooks[0].Command)
}

func TestSidecarSurvivesRestart(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}))

	// No flush — a hard death loses nothing; the next boot reads the sidecar.
	restored := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.Len(t, parkedHooks(t, restored)["Stop"], 1)
}

func TestBootAbsorbsLegacyTmpFile(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	tmpPath := disabledHooksTmpPath(settingsPath)
	data, err := json.Marshal(stopHook("legacy hook"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.Len(t, parkedHooks(t, server)["Stop"], 1)

	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "legacy tmp file not consumed at boot")
	_, err = os.Stat(parkedStorePath(settingsPath))
	assert.NoError(t, err, "absorbed legacy state not persisted to the sidecar")
}

func TestBootAbsorbsSidecarHooks(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	sidecar := config.NewSettings()
	require.NoError(t, sidecar.SetHooks(stopHook("ghost hook")))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), sidecar, nil))

	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.Len(t, parkedHooks(t, server)["Stop"], 1)

	// The sidecar's hooks key is stripped — no ghost can resurface.
	assert.Empty(t, loadHooks(t, config.DisabledPath(settingsPath)))
}
