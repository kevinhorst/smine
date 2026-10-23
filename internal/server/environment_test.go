package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The plan-skill fixture: one required repo-scoped project-plan program.
const (
	planSkillName        = "plan-skill"
	planSkillProgramName = "SKILL_PLANSKILL_PROJECT_PLAN"
)

// fakeProber answers every handshake with claims, or fails with err.
type fakeProber struct {
	claims []string
	err    error
}

func (p *fakeProber) Claims(ctx context.Context, provider *skillgraph.Provider) ([]string, error) {
	return p.claims, p.err
}

func TestEnvironmentPage(t *testing.T) {
	type testCase struct {
		_id               string
		_absentFragment   string
		_expectedFragment string

		path   string
		server *Server
	}

	tests := make([]*testCase, 0)
	tabPath := "/scripts/skills/environment"
	server := newEnvironmentServer(t, map[string]string{envSkillBuildName: "go build ./..."})

	// tool-state
	tests = append(tests, &testCase{
		_id:               "tool-state",
		_absentFragment:   `href="/environment`,
		_expectedFragment: `<span class="label label-key"><strong>smine-rules</strong></span>` + "\n" + `    <span class="card-tags"><span class="badge badge-ok">current</span></span>`,

		path:   tabPath,
		server: server,
	})

	// environment-tab-active
	tests = append(tests, &testCase{
		_id:               "environment-tab-active",
		_absentFragment:   `href="/environment`,
		_expectedFragment: `<a href="/scripts/skills/environment" class="active">Environment</a>`,

		path:   tabPath,
		server: server,
	})

	// override-form
	tests = append(tests, &testCase{
		_id:               "override-form",
		_absentFragment:   `name="` + planSkillProgramName + `"`,
		_expectedFragment: `<form hx-post="/scripts/skills/environment" hx-target="#op-result" hx-disabled-elt="find button" class="card-column configure-form">`,

		path:   tabPath,
		server: server,
	})

	// skill-select-first
	tests = append(tests, &testCase{
		_id:               "skill-select-first",
		_absentFragment:   `<option value="plan-skill" selected>`,
		_expectedFragment: `<select name="skill" aria-label="skill" hx-get="/scripts/skills/environment/widget" hx-trigger="change" hx-target="#skill-env" hx-swap="outerHTML">` + "\n" + `    <option value="env-skill" selected>/env-skill</option><option value="plan-skill" >/plan-skill</option>`,

		path:   tabPath,
		server: server,
	})

	// skill-widget-on-tab
	tests = append(tests, &testCase{
		_id:               "skill-widget-on-tab",
		_absentFragment:   planSkillProgramName,
		_expectedFragment: `<div class="on-environment-tab">`,

		path:   tabPath,
		server: server,
	})

	// skill-and-repo-query
	tests = append(tests, &testCase{
		_id:               "skill-and-repo-query",
		_absentFragment:   envSkillBuildName,
		_expectedFragment: `<form hx-post="/scripts/skills/repo/plan-skill/env/demo" hx-target="#skill-env" hx-swap="outerHTML" class="card-column configure-form">`,

		path:   tabPath + "?skill=" + planSkillName + "&repo=demo",
		server: server,
	})

	// global-defaults-card-lists-kinds
	tests = append(tests, &testCase{
		_id:               "global-defaults-card-lists-kinds",
		_absentFragment:   "No selected skill reads a repo value.",
		_expectedFragment: `<textarea name="build" rows="2" placeholder="go build ./..." aria-label="build"></textarea></span>` + "\n" + `      <span class="meta">used by /env-skill — a skill default or a repo value overrides it</span>` + "\n" + `    </div>` + "\n" + `  </div>` + "\n  \n" + `  <div class="card card-column card-config">` + "\n" + `    <div class="card-row card-head">` + "\n" + `      <span class="label label-key"><strong>project-plan</strong><br><span class="meta explanation">Projects the plan.</span></span>` + "\n" + `      <span class="card-tags"><span class="badge badge-dim">unset</span></span>`,

		path:   tabPath,
		server: server,
	})

	// no-tools
	checkout := t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	providePlanSkill(t, filepath.Join(checkout, "skills"))
	planOnly := install.Selection{Mode: skills.SelectionModeChosen, Chosen: []string{planSkillName}}
	noToolsServer, _ := newCheckoutServer(t, checkout, provideInstallRecord(install.PresetMinimal, planOnly))
	tests = append(tests, &testCase{
		_id:               "no-tools",
		_absentFragment:   `name="PROVIDER_`,
		_expectedFragment: `<div class="empty">No selected skill runs a tool.</div>`,

		path:   tabPath,
		server: noToolsServer,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedFragment)
			assert.NotContains(t, response.Body.String(), test._absentFragment)
		})
	}
}

func TestEnvironmentSave(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedBody      string
		_expectedCode      int
		_expectedProviders map[string]string

		checkout string
		form     url.Values
		server   *Server
	}

	tests := make([]*testCase, 0)
	rulesKey := "PROVIDER_SMINE_RULES"

	// override-saved-and-synced
	server, checkout := newEnvironmentCheckout(t, nil)
	tests = append(tests, &testCase{
		_id:                "override-saved-and-synced",
		_expectedBody:      "",
		_expectedCode:      http.StatusOK,
		_expectedProviders: map[string]string{rulesKey: "my-rules"},

		checkout: checkout,
		form:     url.Values{rulesKey: {" my-rules "}, "OTHER_KEY": {"ignored"}},
		server:   server,
	})

	// empty-clears-row
	server, checkout = newEnvironmentCheckout(t, map[string]string{rulesKey: "my-rules"})
	tests = append(tests, &testCase{
		_id:                "empty-clears-row",
		_expectedBody:      "",
		_expectedCode:      http.StatusOK,
		_expectedProviders: map[string]string{},

		checkout: checkout,
		form:     url.Values{rulesKey: {""}},
		server:   server,
	})

	// unknown-provider-400
	server, checkout = newEnvironmentCheckout(t, map[string]string{rulesKey: "my-rules"})
	tests = append(tests, &testCase{
		_id:                "unknown-provider-400",
		_expectedBody:      "unknown provider PROVIDER_NOPE",
		_expectedCode:      http.StatusBadRequest,
		_expectedProviders: map[string]string{rulesKey: "my-rules"},

		checkout: checkout,
		form:     url.Values{"PROVIDER_NOPE": {"x"}},
		server:   server,
	})

	// bad-program-400
	server, checkout = newEnvironmentCheckout(t, nil)
	tests = append(tests, &testCase{
		_id:                "bad-program-400",
		_expectedBody:      rulesKey + ": ",
		_expectedCode:      http.StatusBadRequest,
		_expectedProviders: map[string]string{},

		checkout: checkout,
		form:     url.Values{rulesKey: {"my rules"}},
		server:   server,
	})

	// stale-record-conflict — the cache serves a record another writer replaced under the same mtime; Save refuses it
	server, checkout = newEnvironmentCheckout(t, nil)
	rewriteUnderSameMtime(t, server, map[string]string{rulesKey: "other-rules"})
	tests = append(tests, &testCase{
		_id:                "stale-record-conflict",
		_expectedBody:      "install.json changed since it was read",
		_expectedCode:      http.StatusOK,
		_expectedProviders: map[string]string{rulesKey: "other-rules"},

		checkout: checkout,
		form:     url.Values{rulesKey: {"my-rules"}},
		server:   server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, formPost("/scripts/skills/environment", test.form))
			rows := storedSkillRows(t, test.server)
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedProviders, rows.Providers)
		})
	}
}

func TestEnvironmentForget(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedBody      string
		_expectedCode      int
		_expectedJournal   []string
		_expectedProviders map[string]string
		_expectedRows      map[string]string

		form   url.Values
		server *Server
	}

	tests := make([]*testCase, 0)
	rulesKey := "PROVIDER_SMINE_RULES"
	forgetPath := "/scripts/skills/environment/forget"

	// variable-row-forgotten
	server, _ := newEnvironmentCheckout(t, nil)
	tests = append(tests, &testCase{
		_id:                "variable-row-forgotten",
		_expectedBody:      "forgot " + envSkillBuildName,
		_expectedCode:      http.StatusOK,
		_expectedJournal:   []string{"environment forget forgotten install.json#" + envSkillBuildName + " my-build"},
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{},

		form:   url.Values{"key": {envSkillBuildName}, "skill": {envSkillName}},
		server: server,
	})

	// provider-row-forgotten
	server, _ = newEnvironmentCheckout(t, map[string]string{rulesKey: "my-rules"})
	tests = append(tests, &testCase{
		_id:                "provider-row-forgotten",
		_expectedBody:      "forgot " + rulesKey,
		_expectedCode:      http.StatusOK,
		_expectedJournal:   []string{"environment forget forgotten install.json#" + rulesKey + " my-rules"},
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{envSkillBuildName: "my-build"},

		form:   url.Values{"key": {rulesKey}, "skill": {envSkillName}},
		server: server,
	})

	// unknown-key-404
	server, _ = newEnvironmentCheckout(t, nil)
	tests = append(tests, &testCase{
		_id:                "unknown-key-404",
		_expectedBody:      "no record row SKILL_NOPE",
		_expectedCode:      http.StatusNotFound,
		_expectedJournal:   nil,
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{envSkillBuildName: "my-build"},

		form:   url.Values{"key": {"SKILL_NOPE"}, "skill": {envSkillName}},
		server: server,
	})

	// unknown-skill-404
	server, _ = newEnvironmentCheckout(t, nil)
	tests = append(tests, &testCase{
		_id:                "unknown-skill-404",
		_expectedBody:      "404 page not found",
		_expectedCode:      http.StatusNotFound,
		_expectedJournal:   nil,
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{envSkillBuildName: "my-build"},

		form:   url.Values{"key": {envSkillBuildName}, "skill": {"nope"}},
		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, formPost(forgetPath, test.form))
			rows := storedSkillRows(t, test.server)
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedProviders, rows.Providers)
			assert.Equal(t, test._expectedRows, rows.Skills)
			assert.Equal(t, test._expectedJournal, forgottenLines(t, test.server))
		})
	}
}

func TestGlobalDefaultsSave(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedBody     string
		_expectedCode     int
		_expectedDefaults map[string]string

		form   url.Values
		server *Server
	}

	tests := make([]*testCase, 0)
	defaultsPath := "/scripts/skills/environment/defaults"
	auditDefault := map[string]string{"build": "make audit"}

	// save-sets-default
	tests = append(tests, &testCase{
		_id:               "save-sets-default",
		_expectedBody:     `<span class="badge badge-ok">Ok</span> <span class="label">saved — the next invocation reads it</span>`,
		_expectedCode:     http.StatusOK,
		_expectedDefaults: auditDefault,

		form:   url.Values{"build": {" make audit "}, "project-plan": {""}},
		server: newGlobalDefaultsServer(t, nil),
	})

	// empty-clears-default
	tests = append(tests, &testCase{
		_id:               "empty-clears-default",
		_expectedBody:     `<span class="badge badge-dim">unset</span>`,
		_expectedCode:     http.StatusOK,
		_expectedDefaults: map[string]string{},

		form:   url.Values{"build": {""}},
		server: newGlobalDefaultsServer(t, auditDefault),
	})

	// unknown-kind-refused
	tests = append(tests, &testCase{
		_id:               "unknown-kind-refused",
		_expectedBody:     "lint: no selected skill reads this kind",
		_expectedCode:     http.StatusBadRequest,
		_expectedDefaults: auditDefault,

		form:   url.Values{"build": {""}, "lint": {"golangci-lint run"}},
		server: newGlobalDefaultsServer(t, auditDefault),
	})

	// invalid-value-refused
	tests = append(tests, &testCase{
		_id:               "invalid-value-refused",
		_expectedBody:     "project-plan: value: a program is one word, got 2",
		_expectedCode:     http.StatusBadRequest,
		_expectedDefaults: auditDefault,

		form:   url.Values{"project-plan": {"my rules"}},
		server: newGlobalDefaultsServer(t, auditDefault),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, formPost(defaultsPath, test.form))
			rows := storedSkillRows(t, test.server)
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedDefaults, rows.Defaults)
		})
	}
}

// forgottenLines renders the server's forgotten journal entries oldest first as "<action> <kind> <path> <reason>" lines; the fixture's own setup journals other kinds.
func forgottenLines(t *testing.T, server *Server) []string {
	t.Helper()
	entries, err := server.trash.Tail(100)
	require.NoError(t, err)

	var lines []string
	oldestFirst := slices.Backward(entries)
	for _, entry := range oldestFirst {
		if entry.Kind == trash.KindForgotten {
			lines = append(lines, entry.Action+" "+string(entry.Kind)+" "+entry.Path+" "+entry.Reason)
		}
	}
	return lines
}

// rewriteUnderSameMtime primes the server's record cache, then saves providers as another writer would and restores the file's mtime, so the cache serves the stale record.
func rewriteUnderSameMtime(t *testing.T, server *Server, providers map[string]string) {
	t.Helper()
	cached, err := server.records.load(server.machine)
	require.NoError(t, err)
	info, err := os.Stat(install.RecordPath(server.machine))
	require.NoError(t, err)

	rewritten := cached
	rewritten.Skills.Providers = providers
	require.NoError(t, install.Save(server.machine, rewritten))
	require.NoError(t, os.Chtimes(install.RecordPath(server.machine), info.ModTime(), info.ModTime()))
}

func TestEnvironmentWidget(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedFragment string

		path string
	}

	server := newEnvironmentServer(t, map[string]string{envSkillBuildName: "go build ./..."})
	tests := make([]*testCase, 0)

	// skill-on-repo
	tests = append(tests, &testCase{
		_id:               "skill-on-repo",
		_expectedCode:     http.StatusOK,
		_expectedFragment: `<form hx-post="/scripts/skills/repo/env-skill/env/demo" hx-target="#skill-env" hx-swap="outerHTML" class="card-column configure-form">`,

		path: "/scripts/skills/environment/widget?skill=" + envSkillName + "&repo=demo",
	})

	// skill-default
	tests = append(tests, &testCase{
		_id:               "skill-default",
		_expectedCode:     http.StatusOK,
		_expectedFragment: `<button type="submit">Save</button><span class="meta">· default</span>`,

		path: "/scripts/skills/environment/widget?skill=" + planSkillName,
	})

	// unknown-skill-404
	tests = append(tests, &testCase{
		_id:               "unknown-skill-404",
		_expectedCode:     http.StatusNotFound,
		_expectedFragment: "404 page not found",

		path: "/scripts/skills/environment/widget?skill=nope",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedFragment)
		})
	}
}

func TestSkillRepoEnv(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedBody string
		_expectedCode int
		_expectedEnv  map[string]string

		form   url.Values
		path   string
		server *Server
	}

	tests := make([]*testCase, 0)
	demoPath := "/scripts/skills/repo/" + envSkillName + "/env/demo"
	savedDemo := "saved for demo — its next session reads the new values"

	// writes-own-keys-keeps-others
	server := newEnvironmentServer(t, map[string]string{envSkillBuildName: "my-build", planSkillProgramName: "bin/acdsl"})
	tests = append(tests, &testCase{
		_id:           "writes-own-keys-keeps-others",
		_expectedBody: savedDemo,
		_expectedCode: http.StatusOK,
		_expectedEnv:  map[string]string{envSkillBuildName: "my-build --all", planSkillProgramName: "bin/acdsl"},

		form:   url.Values{envSkillBuildName: {" my-build --all "}},
		path:   demoPath,
		server: server,
	})

	// answers-with-repo-widget
	server = newEnvironmentServer(t, nil)
	tests = append(tests, &testCase{
		_id:           "answers-with-repo-widget",
		_expectedBody: `<span class="badge badge-ok">repo value</span>`,
		_expectedCode: http.StatusOK,
		_expectedEnv:  map[string]string{envSkillBuildName: "my-build --all"},

		form:   url.Values{envSkillBuildName: {"my-build --all"}},
		path:   demoPath,
		server: server,
	})

	// empty-clears-key
	server = newEnvironmentServer(t, map[string]string{envSkillBuildName: "my-build"})
	tests = append(tests, &testCase{
		_id:           "empty-clears-key",
		_expectedBody: savedDemo,
		_expectedCode: http.StatusOK,
		_expectedEnv:  nil,

		form:   url.Values{envSkillBuildName: {""}},
		path:   demoPath,
		server: server,
	})

	// bad-value-400
	server = newEnvironmentServer(t, map[string]string{envSkillBuildName: "my-build"})
	tests = append(tests, &testCase{
		_id:           "bad-value-400",
		_expectedBody: envSkillBuildName + ": ",
		_expectedCode: http.StatusBadRequest,
		_expectedEnv:  map[string]string{envSkillBuildName: "my-build"},

		form:   url.Values{envSkillBuildName: {"my-build && rm -rf x"}},
		path:   demoPath,
		server: server,
	})

	// other-skills-keys-ignored
	server = newEnvironmentServer(t, nil)
	tests = append(tests, &testCase{
		_id:           "other-skills-keys-ignored",
		_expectedBody: savedDemo,
		_expectedCode: http.StatusOK,
		_expectedEnv:  map[string]string{envSkillBuildName: "my-build"},

		form:   url.Values{envSkillBuildName: {"my-build"}, planSkillProgramName: {"bin/acdsl"}, "SKILL_OTHER_X": {"x"}, "PROVIDER_SMINE_RULES": {"y"}},
		path:   demoPath,
		server: server,
	})

	// relative-program-resolved-under-repo
	server = newEnvironmentServer(t, nil)
	tests = append(tests, &testCase{
		_id:           "relative-program-resolved-under-repo",
		_expectedBody: savedDemo,
		_expectedCode: http.StatusOK,
		_expectedEnv:  map[string]string{planSkillProgramName: "bin/acdsl"},

		form:   url.Values{planSkillProgramName: {"bin/acdsl"}},
		path:   "/scripts/skills/repo/" + planSkillName + "/env/demo",
		server: server,
	})

	// multi-line-command-saved
	server = newEnvironmentServer(t, nil)
	tests = append(tests, &testCase{
		_id:           "multi-line-command-saved",
		_expectedBody: savedDemo,
		_expectedCode: http.StatusOK,
		_expectedEnv:  map[string]string{envSkillBuildName: "my-build\nmy-build --test"},

		form:   url.Values{envSkillBuildName: {"my-build\r\nmy-build --test"}},
		path:   demoPath,
		server: server,
	})

	// unknown-repo-404
	server = newEnvironmentServer(t, nil)
	tests = append(tests, &testCase{
		_id:           "unknown-repo-404",
		_expectedBody: "404 page not found",
		_expectedCode: http.StatusNotFound,
		_expectedEnv:  nil,

		form:   url.Values{envSkillBuildName: {"my-build"}},
		path:   "/scripts/skills/repo/" + envSkillName + "/env/nope",
		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, formPost(test.path, test.form))
			repo, isFound := test.server.repoRegistry.Find("demo")
			require.True(t, isFound)
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedEnv, repo.Env)
		})
	}
}

func TestProviderStateOf(t *testing.T) {
	type testCase struct {
		_id       string
		_expected providerState

		bound       []string
		isInstalled bool
		prober      skills.Prober
		provider    *skillgraph.Provider
	}

	installRulesStub(t)
	title := components.Title(components.ConfigServer)
	rules := &skillgraph.Provider{Id: "smine-rules", Component: components.ConfigServer, Implements: []string{"lanes@1"}, Program: "smine-rules"}
	absent := &skillgraph.Provider{Id: "smine-absent", Component: components.ConfigServer, Implements: []string{"lanes@1"}, Program: "smine-absent"}
	tests := make([]*testCase, 0)

	// current
	tests = append(tests, &testCase{
		_id:       "current",
		_expected: providerState{Id: "smine-rules", Program: "smine-rules", State: stateCurrent},

		bound:       []string{"lanes@1"},
		isInstalled: true,
		prober:      &fakeProber{claims: []string{"lanes@1", "title-set@1"}},
		provider:    rules,
	})

	// stale-lacks-version
	tests = append(tests, &testCase{
		_id:       "stale-lacks-version",
		_expected: providerState{Id: "smine-rules", Fix: "lacks lanes@2, title-set@1 — rebuild and install the " + title + " component", Program: "smine-rules", State: stateStale},

		bound:       []string{"lanes@1", "lanes@2", "title-set@1"},
		isInstalled: true,
		prober:      &fakeProber{claims: []string{"lanes@1"}},
		provider:    rules,
	})

	// stale-no-answer
	tests = append(tests, &testCase{
		_id:       "stale-no-answer",
		_expected: providerState{Id: "smine-rules", Fix: "rebuild and install the " + title + " component — unknown subcommand interfaces", Program: "smine-rules", State: stateStale},

		bound:       []string{"lanes@1"},
		isInstalled: true,
		prober:      &fakeProber{err: errors.New("unknown subcommand interfaces")},
		provider:    rules,
	})

	// missing-not-installed
	tests = append(tests, &testCase{
		_id:       "missing-not-installed",
		_expected: providerState{Id: "smine-rules", Fix: "install the " + title + " component", Program: "smine-rules", State: stateMissing},

		bound:       []string{"lanes@1"},
		isInstalled: false,
		prober:      &fakeProber{claims: []string{"lanes@1"}},
		provider:    rules,
	})

	// missing-not-on-path
	tests = append(tests, &testCase{
		_id:       "missing-not-on-path",
		_expected: providerState{Id: "smine-absent", Fix: "install the " + title + " component — smine-absent is not on PATH", Program: "smine-absent", State: stateMissing},

		bound:       []string{"lanes@1"},
		isInstalled: true,
		prober:      &fakeProber{claims: []string{"lanes@1"}},
		provider:    absent,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			state := providerStateOf(t.Context(), test.bound, test.isInstalled, test.prober, test.provider)
			assert.Equal(t, test._expected, state)
		})
	}
}

// newEnvironmentCheckout serves a checkout holding env-skill and plan-skill
// with providers as the record's PROVIDER_ rows, and returns the checkout.
func newEnvironmentCheckout(t *testing.T, providers map[string]string) (*Server, string) {
	t.Helper()
	checkout := t.TempDir()
	skillsRepo := filepath.Join(checkout, "skills")
	provideEnvSkill(t, skillsRepo, false)
	providePlanSkill(t, skillsRepo)
	record := provideSkillRowsRecord(install.PresetFull, install.Selection{Mode: skills.SelectionModeAll}, providers, map[string]string{envSkillBuildName: "my-build"})
	server, _ := newCheckoutServer(t, checkout, record)
	return server, checkout
}

// newGlobalDefaultsServer serves a checkout holding env-skill and plan-skill
// with defaults as the record's global defaults.
func newGlobalDefaultsServer(t *testing.T, defaults map[string]string) *Server {
	t.Helper()
	checkout := t.TempDir()
	skillsRepo := filepath.Join(checkout, "skills")
	provideEnvSkill(t, skillsRepo, false)
	providePlanSkill(t, skillsRepo)
	record := provideInstallRecord(install.PresetFull, install.Selection{Mode: skills.SelectionModeAll})
	record.Skills.Defaults = defaults
	server, _ := newCheckoutServer(t, checkout, record)
	return server
}

// newEnvironmentServer serves a checkout holding env-skill and plan-skill,
// the record default SKILL_ENVSKILL_BUILD=my-build and one registered
// repo demo with env as its values and an executable bin/acdsl.
func newEnvironmentServer(t *testing.T, env map[string]string) *Server {
	t.Helper()
	server, _ := newEnvironmentCheckout(t, nil)

	repoPath := t.TempDir()
	name, script := "acdsl", "#!/bin/sh\n"
	if runtime.GOOS == "windows" {
		name, script = "acdsl.bat", "@exit /b 0\r\n"
	}
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "bin", name), []byte(script), 0o755))
	require.NoError(t, server.repoRegistry.Add(repos.Repo{Env: env, Name: "demo", Path: repoPath}))
	return server
}

// providePlanSkill writes the repo skill planSkillName into skillsRepo, one
// step running its required repo-scoped PROJECT_PLAN program.
func providePlanSkill(t *testing.T, skillsRepo string) {
	t.Helper()
	skillDir := filepath.Join(skillsRepo, planSkillName)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	manifest := "---\nname: " + planSkillName + "\ndescription: d\nversion: 1.0\nformat: 2\n---\n# Plan\n\n## Run\n\n" +
		"**SKILL-PLANSKILL-RUN-001** `[step]` **Run**\n\nRUN `$" + planSkillProgramName + " project-plan`.\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "changelog.json"), []byte(demoGraphChange), 0o644))
	skill, err := skillimport.Import(skillDir)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(skillDir, "changelog.json")))
	require.NoError(t, os.Remove(filepath.Join(skillDir, "SKILL.md")))

	program := &skillgraph.Variable{Interface: "project-plan@1", Required: true}
	skill.Catalog = provideSkillsCatalog(t, skillsRepo)
	skill.Env = map[string]*skillgraph.Variable{"PROJECT_PLAN": program}
	require.NoError(t, skillgraph.Save(skillDir, skill))
}
