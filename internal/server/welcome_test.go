package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWelcomeServer builds a server whose install env lives in temp dirs —
// HOME, the Claude home and the checkout — so component health never reads
// the machine.
func newWelcomeServer(t *testing.T, isInitWelcome bool) *Server {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	options := &Options{
		InitWelcome:  isInitWelcome,
		RepoDir:      t.TempDir(),
		SettingsPath: filepath.Join(t.TempDir(), "settings.json"),
		TokenDir:     filepath.Join(t.TempDir(), "claude-routine", "tokens"),
	}
	return newTestServer(t, options)
}

// provideHealthyConfigServer writes what the config server's health checks:
// the rules binary in the smine bin dir (kept under the test home on
// Windows too) and the access credential.
func provideHealthyConfigServer(t *testing.T, server *Server) {
	t.Helper()
	t.Setenv("LOCALAPPDATA", filepath.Join(server.machine.Home, "AppData", "Local"))
	rules := "smine-rules"
	if runtime.GOOS == "windows" {
		rules += ".exe"
	}
	paths := []string{
		filepath.Join(config.SmineBinDir(runtime.GOOS, server.machine.Home), rules),
		filepath.Join(server.machine.ClaudeHome, "configserver", "credential"),
	}
	for _, path := range paths {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	}
}

// provideHealthySessionHooks writes what Session hooks' health checks: its
// installed target, the rendered hooks config.
func provideHealthySessionHooks(t *testing.T, server *Server) {
	t.Helper()
	path := filepath.Join(server.machine.ClaudeHome, "hooks", "config.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644))
}

// saveWelcomeRecord stores a custom record selecting ids, skills mode all.
func saveWelcomeRecord(t *testing.T, server *Server, ids ...components.ID) {
	t.Helper()
	record := install.FullRecord(server.machine.GOOS)
	record.Components = components.Close(ids)
	record.Preset = install.PresetCustom
	require.NoError(t, install.Save(server.machine, record))
}

func TestWelcomeRendersComponents(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedPresent bool

		server  *Server
		snippet string
	}

	tests := make([]*testCase, 0)

	// off-row-shows-profile-link
	offServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, offServer, components.ConfigServer, components.Skills)
	tests = append(tests, &testCase{
		_id:              "off-row-shows-profile-link",
		_expectedPresent: true,

		server:  offServer,
		snippet: `<a href="/profile#installation">turn it on</a>`,
	})

	// no-bootstrap-card-without-mining
	noMiningServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, noMiningServer, components.ConfigServer, components.Skills)
	tests = append(tests, &testCase{
		_id:              "no-bootstrap-card-without-mining",
		_expectedPresent: false,

		server:  noMiningServer,
		snippet: `hx-post="/welcome/bootstrap"`,
	})

	// bootstrap-card-with-mining — no record reads as Full, which holds
	// Mining wherever the platform offers it
	miningServer := newWelcomeServer(t, false)
	tests = append(tests, &testCase{
		_id:              "bootstrap-card-with-mining",
		_expectedPresent: components.Available(components.Mining, miningServer.machine.GOOS),

		server:  miningServer,
		snippet: `hx-post="/welcome/bootstrap"`,
	})

	// verify-button-only-on-selected-token-row — the button also needs bash
	tokenServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, tokenServer, components.ConfigServer, components.Skills, components.Token)
	tests = append(tests, &testCase{
		_id:              "verify-button-only-on-selected-token-row",
		_expectedPresent: verifyBashPath() != "",

		server:  tokenServer,
		snippet: `hx-post="/welcome/verify-token"`,
	})

	// no-verify-button-on-off-token-row
	tokenOffServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, tokenOffServer, components.ConfigServer, components.Skills)
	tests = append(tests, &testCase{
		_id:              "no-verify-button-on-off-token-row",
		_expectedPresent: false,

		server:  tokenOffServer,
		snippet: `hx-post="/welcome/verify-token"`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome", nil))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, test._expectedPresent, strings.Contains(recorder.Body.String(), test.snippet))
		})
	}
}

func TestWelcomeBootstrapCard(t *testing.T) {
	server := newWelcomeServer(t, false)
	if !components.Available(components.Mining, server.machine.GOOS) {
		t.Skip("the Mining component, and with it the bootstrap card, is not offered on this platform")
	}

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, `name="extra-prompt"`)
	assert.Contains(t, body, `id="bootstrap-result"`)
	assert.Regexp(t, `name="since" value="\d{4}-\d{2}-\d{2}" max="\d{4}-\d{2}-\d{2}"`, body)
	assert.Contains(t, body, `Bootstrap <span class="badge badge-action">experimental</span>`)
	assert.Contains(t, body, `<form class="card-column" hx-post="/welcome/bootstrap"`)
}

func TestWelcomeTile(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDetail string
		_expectedValue  string

		server *Server
	}

	tests := make([]*testCase, 0)

	// all-selected-healthy
	healthyServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, healthyServer, components.ConfigServer, components.Skills)
	provideHealthyConfigServer(t, healthyServer)
	tests = append(tests, &testCase{
		_id:             "all-selected-healthy",
		_expectedDetail: "all green",
		_expectedValue:  "2/2 components",

		server: healthyServer,
	})

	// one-down
	downServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, downServer, components.ConfigServer, components.Skills)
	tests = append(tests, &testCase{
		_id:             "one-down",
		_expectedDetail: "next: Config server",
		_expectedValue:  "1/2 components",

		server: downServer,
	})

	// off-components-not-counted — the token is off and absent; it neither
	// counts as selected nor reddens the tile; the skill environment's skillenv
	// binary is installed
	offServer := newWelcomeServer(t, false)
	saveWelcomeRecord(t, offServer, components.ConfigServer, components.Skills, components.SessionHooks, components.SkillEnvironment)
	provideHealthyConfigServer(t, offServer)
	provideHealthySessionHooks(t, offServer)
	skillenvBinary := filepath.Join(offServer.machine.ClaudeHome, "hooks", "bin", "skillenv")
	require.NoError(t, os.MkdirAll(filepath.Dir(skillenvBinary), 0o755))
	require.NoError(t, os.WriteFile(skillenvBinary, []byte("x"), 0o755))
	tests = append(tests, &testCase{
		_id:             "off-components-not-counted",
		_expectedDetail: "all green",
		_expectedValue:  "4/4 components",

		server: offServer,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			tile := test.server.welcomeTile(t.Context())
			assert.Equal(t, test._expectedValue, tile.Value)
			assert.Equal(t, test._expectedDetail, tile.Detail)
		})
	}
}

func TestBootstrapDates(t *testing.T) {
	syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	sinceDefault, today := bootstrapDates(syncNow)

	assert.Equal(t, "1999-12-02", sinceDefault)
	assert.Equal(t, "2000-01-01", today)
}

func TestWelcomeChecksFragmentIsPartial(t *testing.T) {
	server := newWelcomeServer(t, false)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome/checks", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `class="welcome-progress"`)
	assert.NotContains(t, recorder.Body.String(), "<nav>")
}

func TestWelcomeTutorialTab(t *testing.T) {
	server := newWelcomeServer(t, false)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome?tab=tutorial", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "context.actions: Added the reconcile-by-reapply rule (example)")
	assert.Contains(t, body, `<span class="proposal-title">Two sessions in one week re-broke a merge after resolving conflict hunks in place.</span>`)
	assert.Contains(t, body, "Pick a change onto main to accept it")
	assert.NotContains(t, body, "/api/proposals/pick")
	assert.NotContains(t, body, "/api/proposals/reject")
	assert.NotContains(t, body, `class="bulk-check"`)
	assert.NotContains(t, strings.ToLower(body), "vote")
	assert.NotContains(t, body, "/welcome/bootstrap")
	assert.NotContains(t, body, `class="welcome-progress"`)
}

func TestOverviewWelcomeTile(t *testing.T) {
	server := newWelcomeServer(t, false)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/overview/welcome-tile", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `href="/welcome"`)
	assert.Contains(t, recorder.Body.String(), `data-tile-id="welcome"`)
	assert.NotContains(t, recorder.Body.String(), "hx-get", "the loaded tile does not load itself again")
	assert.Regexp(t, `\d+/\d+ components`, recorder.Body.String())
}

func TestOverviewSkipsPeek(t *testing.T) {
	hanging := newHangingPeek(t)
	server := newTestServer(t, &Options{PeekEndpoint: hanging.url, TokenDir: filepath.Join(t.TempDir(), "tokens")})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Zero(t, hanging.calls.Load(), "the page never waits on peek")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `hx-get="/overview/welcome-tile"`)
	assert.NotRegexp(t, `\d+/\d+ components`, recorder.Body.String(), "component health runs in the fragment, not the page")
}

func TestWelcomeNavGatedByInitWelcome(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedShown bool

		server *Server
	}

	tests := make([]*testCase, 0)

	// hidden-without-init-welcome
	hiddenOptions := &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")}
	tests = append(tests, &testCase{
		_id:            "hidden-without-init-welcome",
		_expectedShown: false,

		server: newTestServer(t, hiddenOptions),
	})

	// shown-developer
	developerOptions := &Options{InitWelcome: true, TokenDir: filepath.Join(t.TempDir(), "tokens")}
	tests = append(tests, &testCase{
		_id:            "shown-developer",
		_expectedShown: true,

		server: newTestServer(t, developerOptions),
	})

	// shown-casual — Welcome is not an internals entry, casual users see it too
	casualOptions := &Options{
		InitWelcome:      true,
		PresentationPath: writeGermanProfile(t),
		TokenDir:         filepath.Join(t.TempDir(), "tokens"),
	}
	tests = append(tests, &testCase{
		_id:            "shown-casual",
		_expectedShown: true,

		server: newTestServer(t, casualOptions),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile", nil))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, test._expectedShown, strings.Contains(recorder.Body.String(), `href="/welcome"`))
		})
	}
}

func TestWelcomeBootstrapConcurrent(t *testing.T) {
	server := newTestServer(t, &Options{})
	server.bootstrap.running = true

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/welcome/bootstrap", url.Values{}))
	assert.Equal(t, http.StatusConflict, recorder.Code)
}
