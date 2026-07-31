package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSyncFixtures(t *testing.T) (livePath, fragmentPath string, server *Server) {
	t.Helper()
	dir := t.TempDir()
	livePath = filepath.Join(dir, "settings.json")
	fragmentPath = filepath.Join(dir, "fragment.json")
	require.NoError(t, os.WriteFile(livePath, []byte(`{"model": "live"}`), 0644))
	require.NoError(t, os.WriteFile(fragmentPath, []byte(`{"model": "fragment"}`), 0644))
	server = newTestServer(t, &Options{ClaudeFragmentPath: fragmentPath, SettingsPath: livePath})
	return livePath, fragmentPath, server
}

// newSectionServer builds a server whose live settings and repo fragment
// hold the given JSON — the fixture for the section overridden badges (D1).
func newSectionServer(t *testing.T, live, fragment string) *Server {
	t.Helper()
	dir := t.TempDir()
	livePath := filepath.Join(dir, "settings.json")
	fragmentPath := filepath.Join(dir, "fragment.json")
	require.NoError(t, os.WriteFile(livePath, []byte(live), 0644))
	require.NoError(t, os.WriteFile(fragmentPath, []byte(fragment), 0644))
	return newTestServer(t, &Options{ClaudeFragmentPath: fragmentPath, SettingsPath: livePath})
}

// assertSectionBadge asserts the fragment carries the OOB summary span and
// whether the overridden badge is inside it.
func assertSectionBadge(t *testing.T, server *Server, path, section string, overridden bool) {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
	require.Equal(t, 200, response.Code, response.Body.String())

	body := response.Body.String()
	spanAt := strings.Index(body, `id="ovr-`+section+`" hx-swap-oob="true"`)
	require.Greater(t, spanAt, -1, body)
	span := body[spanAt:]
	span = span[:strings.Index(span, "</span>")+len("</span>")]
	if overridden {
		assert.Contains(t, span, ">overridden<")
	} else {
		assert.NotContains(t, span, ">overridden<")
	}
}

func TestConfigSync(t *testing.T) {
	t.Run("apply-copies-live-to-fragment", func(t *testing.T) {
		livePath, fragmentPath, server := writeSyncFixtures(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/apply", nil))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), "synced:")

		fragment, err := os.ReadFile(fragmentPath)
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "live"}`, string(fragment))
		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "live"}`, string(live))
	})

	t.Run("revert-copies-fragment-to-live", func(t *testing.T) {
		livePath, _, server := writeSyncFixtures(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/revert", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "fragment"}`, string(live))
	})

	t.Run("revert-clears-parked-disabled-state", func(t *testing.T) {
		livePath, _, server := writeSyncFixtures(t)
		server.disabledHooks.Add("Stop", config.HookGroup{
			Hooks: []config.Hook{{Command: "afplay x.aiff", Type: "command"}},
		})
		sidecar := config.NewSettings()
		require.NoError(t, sidecar.SetDisabledMcpjsonServers([]string{"parked"}))
		require.NoError(t, config.Save(config.DisabledPath(livePath), sidecar))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/revert", nil))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "cleared parked disabled state")

		assert.Empty(t, server.disabledHooks.Snapshot())
		reloaded, err := config.Load(config.DisabledPath(livePath))
		require.NoError(t, err)
		parked, err := reloaded.DisabledMcpjsonServers()
		require.NoError(t, err)
		assert.Empty(t, parked)
	})

	t.Run("apply-keeps-parked-disabled-state", func(t *testing.T) {
		_, _, server := writeSyncFixtures(t)
		server.disabledHooks.Add("Stop", config.HookGroup{
			Hooks: []config.Hook{{Command: "afplay x.aiff", Type: "command"}},
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/apply", nil))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Len(t, server.disabledHooks.Snapshot(), 1)
	})

	t.Run("unknown-target-404", func(t *testing.T) {
		_, _, server := writeSyncFixtures(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/bogus/sync/apply", nil))
		assert.Equal(t, 404, response.Code)
	})

	t.Run("missing-source-errors-without-partial-write", func(t *testing.T) {
		livePath, fragmentPath, server := writeSyncFixtures(t)
		require.NoError(t, os.Remove(fragmentPath))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/revert", nil))
		// Op errors render inside the result block, not as HTTP errors.
		require.Equal(t, 200, response.Code)
		assert.Contains(t, response.Body.String(), "copyFileAtomic")

		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "live"}`, string(live))
	})
}
