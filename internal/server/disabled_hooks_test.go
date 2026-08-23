package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeHookSettings(t *testing.T, hooks map[string][]config.HookGroup) string {
	t.Helper()
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(hooks))
	require.NoError(t, config.Save(settingsPath, main))
	return settingsPath
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

func TestToggleOffMovesGroupToSidecar(t *testing.T) {
	settingsPath := writeHookSettings(t, stopHook("say done"))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/hooks/Stop/0/toggle?enabled=true", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	assert.Empty(t, loadHooks(t, settingsPath))
	assert.Len(t, server.disabledHooks.Snapshot()["Stop"], 1)
	_, err := os.Stat(server.disabledHooks.path)
	assert.NoError(t, err, "toggle off left no sidecar")
}

func TestToggleOnWritesBackToSettings(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/hooks/Stop/0/toggle?enabled=false", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	hooks := loadHooks(t, settingsPath)
	require.Len(t, hooks["Stop"], 1)
	assert.Equal(t, "say done", hooks["Stop"][0].Hooks[0].Command)
	assert.Empty(t, server.disabledHooks.Snapshot())
	_, err := os.Stat(server.disabledHooks.path)
	assert.True(t, os.IsNotExist(err), "empty store left a sidecar behind")
}

func TestToggleMissingEnabledParamIs400(t *testing.T) {
	settingsPath := writeHookSettings(t, stopHook("say done"))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/hooks/Stop/0/toggle", nil)
	server.Handler().ServeHTTP(response, request)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestDeleteEnabledRowRemovesFromFile(t *testing.T) {
	settingsPath := writeHookSettings(t, stopHook("say done"))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/hooks/Stop/0?enabled=true", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	assert.Empty(t, loadHooks(t, settingsPath))
	assert.Empty(t, server.disabledHooks.Snapshot())
}

func TestDeleteDisabledRowRemovesFromStore(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/hooks/Stop/0?enabled=false", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	assert.Empty(t, server.disabledHooks.Snapshot())
}

func TestSidecarSurvivesRestart(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}))

	// No flush — a hard death loses nothing; the next boot loads the sidecar.
	restored := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.Len(t, restored.disabledHooks.Snapshot()["Stop"], 1)
}

func TestBootAbsorbsLegacyTmpFile(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	tmpPath := disabledHooksTmpPath(settingsPath)
	data, err := json.Marshal(stopHook("legacy hook"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.Len(t, server.disabledHooks.Snapshot()["Stop"], 1)

	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "legacy tmp file not consumed at boot")
	_, err = os.Stat(server.disabledHooks.path)
	assert.NoError(t, err, "absorbed legacy state not persisted to the sidecar")
}

func TestBootAbsorbsSidecarHooks(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	sidecar := config.NewSettings()
	require.NoError(t, sidecar.SetHooks(stopHook("ghost hook")))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), sidecar))

	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.Len(t, server.disabledHooks.Snapshot()["Stop"], 1)

	// The sidecar's hooks key is stripped — no ghost can resurface.
	assert.Empty(t, loadHooks(t, config.DisabledPath(settingsPath)))
}
