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

func TestWelcomeRendersAllChecks(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, &Options{
		ClaudeJsonPath: filepath.Join(dir, "claude.json"),
		RoutinesDir:    filepath.Join(dir, "routines"),
		SkillsHome:     filepath.Join(dir, "skills"),
		TokenDir:       filepath.Join(dir, "claude-routine", "tokens"),
	})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "routine token")
	assert.Contains(t, body, "peek config fragment")
	assert.Contains(t, body, "peek reachable")
	assert.Contains(t, body, "settings, hooks and skills synced")
	assert.Contains(t, body, "smine-nightly loaded")
	assert.Contains(t, body, "mining roster")
	assert.Contains(t, body, "badge-error")
	assert.Contains(t, body, "the nightly run exits 78 and does nothing")
	assert.Contains(t, body, "fix:")
}

func TestWelcomeChecksFragmentIsPartial(t *testing.T) {
	server := newTestServer(t, &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome/checks", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "setup checks:")
	assert.NotContains(t, recorder.Body.String(), "<nav>")
}

func TestWelcomeTutorialTab(t *testing.T) {
	server := newTestServer(t, &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome?tab=tutorial", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "Example proposal — reconcile by reapply")
	assert.Contains(t, body, "vote-disabled")
	assert.NotContains(t, body, "/api/proposals/context/welcome-demo-001/vote")
	assert.NotContains(t, body, "setup checks:")
}

func TestServer_CheckPeekFragment(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDetail string
		_expectedOk     bool

		server *Server
	}

	binaryPath := filepath.Join(t.TempDir(), "peek-mcp")
	require.NoError(t, os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755))
	fragment := func(t *testing.T, content string) *Server {
		path := filepath.Join(t.TempDir(), "claude.json")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return newTestServer(t, &Options{ClaudeJsonPath: path})
	}

	tests := make([]*testCase, 0)

	// pass-valid-fragment
	tests = append(tests, &testCase{
		_id:             "pass-valid-fragment",
		_expectedDetail: "--control-port=0",
		_expectedOk:     true,

		server: fragment(t, `{"mcpServers":{"peek-mcp":{"command":"`+binaryPath+`","args":["start","--control-port=0"]}}}`),
	})

	// fail-missing-server
	tests = append(tests, &testCase{
		_id:             "fail-missing-server",
		_expectedDetail: `No "peek-mcp" entry`,
		_expectedOk:     false,

		server: fragment(t, `{"mcpServers":{"other":{"command":"x"}}}`),
	})

	// fail-missing-control-port
	tests = append(tests, &testCase{
		_id:             "fail-missing-control-port",
		_expectedDetail: "--control-port=0",
		_expectedOk:     false,

		server: fragment(t, `{"mcpServers":{"peek-mcp":{"command":"`+binaryPath+`","args":["start"]}}}`),
	})

	// fail-command-not-found
	tests = append(tests, &testCase{
		_id:             "fail-command-not-found",
		_expectedDetail: "does not exist",
		_expectedOk:     false,

		server: fragment(t, `{"mcpServers":{"peek-mcp":{"command":"/absent/peek-mcp","args":["--control-port=0"]}}}`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			check := test.server.checkPeekFragment()
			assert.Equal(t, test._expectedOk, check.Ok)
			assert.Contains(t, check.Detail, test._expectedDetail)
		})
	}
}

func TestServer_CheckToken(t *testing.T) {
	type testCase struct {
		_id         string
		_expectedOk bool

		server *Server
	}

	tokenStore := func(t *testing.T, legacyValue string, labels map[string]string) *Server {
		base := filepath.Join(t.TempDir(), "claude-routine")
		tokenDir := filepath.Join(base, "tokens")
		require.NoError(t, os.MkdirAll(tokenDir, 0o700))
		if legacyValue != "" {
			require.NoError(t, os.WriteFile(filepath.Join(base, "token"), []byte(legacyValue), 0o600))
		}
		for label, value := range labels {
			require.NoError(t, os.WriteFile(filepath.Join(tokenDir, label), []byte(value), 0o600))
		}
		return newTestServer(t, &Options{TokenDir: tokenDir})
	}

	tests := make([]*testCase, 0)

	// pass-legacy-file
	tests = append(tests, &testCase{
		_id:         "pass-legacy-file",
		_expectedOk: true,

		server: tokenStore(t, "sk-secret-value", nil),
	})

	// pass-labeled-token
	tests = append(tests, &testCase{
		_id:         "pass-labeled-token",
		_expectedOk: true,

		server: tokenStore(t, "", map[string]string{"work": "sk-secret-value"}),
	})

	// fail-absent
	tests = append(tests, &testCase{
		_id:         "fail-absent",
		_expectedOk: false,

		server: tokenStore(t, "", nil),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			check := test.server.checkToken()
			assert.Equal(t, test._expectedOk, check.Ok)
			assert.NotContains(t, check.Detail, "sk-secret-value")
		})
	}
}

func TestServer_CheckTokenEmptyLegacyFile(t *testing.T) {
	base := filepath.Join(t.TempDir(), "claude-routine")
	require.NoError(t, os.MkdirAll(base, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(base, "token"), nil, 0o600))
	server := newTestServer(t, &Options{TokenDir: filepath.Join(base, "tokens")})

	check := server.checkToken()
	assert.False(t, check.Ok)
}

func TestOverviewWelcomeTile(t *testing.T) {
	server := newTestServer(t, &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `href="/welcome"`)
	assert.Regexp(t, `\d/\d checks`, recorder.Body.String())
}

func TestWelcomeNavGatedByInitWelcome(t *testing.T) {
	hidden := newTestServer(t, &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")})
	recorder := httptest.NewRecorder()
	hidden.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), `href="/welcome"`)

	shown := newTestServer(t, &Options{InitWelcome: true, TokenDir: filepath.Join(t.TempDir(), "tokens")})
	recorder = httptest.NewRecorder()
	shown.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `href="/welcome"`)
}

func TestServer_CheckRoster(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755))
	settingsPath := filepath.Join(dir, "settings.json")
	settings := `{"permissions":{"additionalDirectories":["` + repoDir + `","` + filepath.Join(dir, "not-a-repo") + `"]}}`
	require.NoError(t, os.WriteFile(settingsPath, []byte(settings), 0o644))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	check := server.checkRoster()
	assert.True(t, check.Ok)
	assert.Contains(t, check.Detail, "1 git repos")
}
