package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sessionStartGlobalGroup  = `{"smineId": "context-sessionstart-globalcontext", "smineName": "Global context", "smineVersion": "1.0.0", "hooks": [{"command": "globalcontext", "type": "command"}]}`
	sessionStartReadgate     = `{"smineId": "context-sessionstart-readgate", "smineName": "Read gate", "smineVersion": "1.0.0", "hooks": [{"command": "readgate", "type": "command"}]}`
	subagentStartGlobalGroup = `{"smineId": "context-subagentstart-globalcontext", "smineName": "Global context", "smineVersion": "1.0.0", "hooks": [{"command": "globalcontext", "type": "command"}]}`
)

// globalContextServer builds a server whose presentation profile lives under a
// temp ~/.claude/context/global tree and whose context fragment declares the
// two global-context groups plus a read-gate group, all live in settings.json.
func globalContextServer(t *testing.T) *Server {
	t.Helper()
	home := t.TempDir()
	globalDir := filepath.Join(home, "context", "global")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	hooks := `{"SessionStart": [` + sessionStartGlobalGroup + `, ` + sessionStartReadgate + `], "SubagentStart": [` + subagentStartGlobalGroup + `]}`
	templateDir := writeClaudeTemplate(t, t.TempDir(), `{}`)
	writeComponentFragment(t, templateDir, "context", hooks)
	settingsPath := filepath.Join(home, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"hooks": `+hooks+`}`), 0o644))
	return newTestServer(t, &Options{
		ClaudeFragmentPath: templateDir,
		PresentationPath:   filepath.Join(globalDir, "presentation-profile.md"),
		SettingsPath:       settingsPath,
		StylePath:          filepath.Join(globalDir, "style-profile.md"),
	})
}

func TestGlobalContextToggle(t *testing.T) {
	server := globalContextServer(t)

	// every group live → enabled
	assert.True(t, server.globalContextEnabled())

	// toggle-parks-groups: both global-context groups move to the parked store, the read gate stays
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/context/global/toggle", nil))
	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.False(t, server.globalContextEnabled())
	live := loadHooks(t, server.settingsPath)
	require.Len(t, live["SessionStart"], 1)
	assert.Equal(t, "context-sessionstart-readgate", live["SessionStart"][0].Id)
	assert.Empty(t, live["SubagentStart"])
	parked := parkedHooks(t, server)
	require.Len(t, parked["SessionStart"], 1)
	assert.Equal(t, "context-sessionstart-globalcontext", parked["SessionStart"][0].Id)
	require.Len(t, parked["SubagentStart"], 1)
	assert.Equal(t, "context-subagentstart-globalcontext", parked["SubagentStart"][0].Id)

	// toggle-unparks-groups: both come back live, the store holds none
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/context/global/toggle", nil))
	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.True(t, server.globalContextEnabled())
	live = loadHooks(t, server.settingsPath)
	assert.Len(t, live["SessionStart"], 2)
	require.Len(t, live["SubagentStart"], 1)
	assert.Equal(t, "context-subagentstart-globalcontext", live["SubagentStart"][0].Id)
	assert.Empty(t, parkedHooks(t, server))

	t.Run("unpark-settings-write-failure-keeps-parked", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, formPost("/context/global/toggle", nil))
		require.Equal(t, http.StatusSeeOther, recorder.Code)
		lockSettingsDir(t, server.settingsPath)

		recorder = httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, formPost("/context/global/toggle", nil))
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Empty(t, loadHooks(t, server.settingsPath)["SubagentStart"])
		parked := parkedHooks(t, server)
		assert.Len(t, parked["SessionStart"], 1, "the groups stay parked when settings.json is not written")
		assert.Len(t, parked["SubagentStart"], 1)
	})
}

func TestGlobalContextFilesLists(t *testing.T) {
	server := globalContextServer(t)
	require.NoError(t, os.WriteFile(server.presentation.path, []byte("---\n---\nbody\n"), 0o644))
	require.NoError(t, os.WriteFile(server.presentation.stylePath, []byte("- terse\n"), 0o644))

	files, err := server.globalContextFiles()
	require.NoError(t, err)
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	assert.Contains(t, names, "presentation-profile.md")
	assert.Contains(t, names, "style-profile.md")
}
