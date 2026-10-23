package server

import (
	"encoding/json"
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
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skilleval"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routineSkillName is the repo skill fixture that requires Routines.
const routineSkillName = "routine-skill"

// The env-skill fixture: a lanes program its provider fills and a build command variable.
const (
	envSkillBuildName = "SKILL_ENVSKILL_BUILD"
	envSkillName      = "env-skill"
)

// testCatalogCue is the fixture skills root's interfaces.cue: the repo-scoped build command and project-plan program, and the lanes and title-set programs of smine-rules.
const testCatalogCue = `interfaces: [{id: "build", version: 1, kind: "command", scope: "repo", example: "go build ./...", summary: "Compiles the repo.", invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "20m"}, result: {exit: {"0": "pass", other: "fail"}, stdout: "diagnostic"}, validate: {resolves: true}}, {id: "lanes", version: 1, kind: "program", scope: "install", summary: "Writes the lanes.", invocation: {cwd: "repo-root", args: "lanes --round-dir <dir>", stdin: "none", timeout: "5m"}, result: {exit: {"0": "printed", other: "error"}, stdout: "the plan"}, validate: {resolves: true}}, {id: "project-plan", version: 1, kind: "program", scope: "repo", example: "bin/acdsl", summary: "Projects the plan.", invocation: {cwd: "repo-root", args: "project-plan <plan>", stdin: "none", timeout: "1m"}, result: {exit: {"0": "projected", other: "error"}, stdout: "the projection"}, validate: {resolves: true}}, {id: "title-set", version: 1, kind: "program", scope: "install", summary: "Sets the title state.", invocation: {cwd: "repo-root", args: "title set <TOKEN>", stdin: "none", timeout: "1m"}, result: {exit: {"0": "set", other: "error"}, stdout: "title: set <TOKEN>"}, validate: {resolves: true}}]
providers: [{id: "smine-rules", program: "smine-rules", component: "config-server", implements: ["lanes@1", "title-set@1"], probe: ["interfaces"]}]
`

// installRulesStub puts smine-rules and my-build ahead of PATH; smine-rules answers the handshake for lanes@1 and title-set@1.
func installRulesStub(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	answer := `{"interfaces":["lanes@1","title-set@1"]}`
	for _, program := range []string{"smine-rules", "my-build"} {
		name, script := program, "#!/bin/sh\necho '"+answer+"'\n"
		if runtime.GOOS == "windows" {
			name, script = program+".bat", "@echo "+answer+"\r\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSkillsStateFilter(t *testing.T) {
	writeHomeSkill := func(t *testing.T, root, name string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\nversion: 1.0\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillsHome := filepath.Join(dir, "home")
	writeRepoSkill(t, skillsRepo, "repo-skill", "1.0")
	writeRepoSkill(t, skillsRepo, "both-skill", "1.0")
	writeHomeSkill(t, skillsHome, "both-skill")
	writeHomeSkill(t, skillsHome, "home-skill")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	t.Run("all-states-with-counts", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "/repo-skill")
		assert.Contains(t, body, "/home-skill")
		assert.Contains(t, body, "/both-skill")
		assert.Contains(t, body, "All (3)")
		assert.Contains(t, body, "Synced (1)")
		assert.Contains(t, body, "Active only (1)")
		assert.Contains(t, body, "Uninstalled (1)")
		assert.Contains(t, body, "v1.0")
		assert.Contains(t, body, "0 invocations")
		assert.NotContains(t, body, "local")
		// The list re-pulls itself after a sync op (repo-op trigger).
		assert.Contains(t, body, `id="skills-list" hx-get="/scripts/skills"`)
		// The page's tabs: the list is active, the graph one click away.
		assert.Contains(t, body, `<a href="/scripts/skills" class="active">Skills</a>`)
		assert.Contains(t, body, `<a href="/scripts/skills/graph" >Graph</a>`)
		assert.Contains(t, body, `hx-trigger="repo-op from:body"`)
		// Synced cards link to the repo variant (D8).
		assert.Contains(t, body, "/scripts/skills/repo/both-skill")
		assert.NotContains(t, body, "/scripts/skills/home/both-skill")
	})

	t.Run("state-filter-selects-subset", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills?state=active", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "/home-skill")
		assert.NotContains(t, body, "/repo-skill")
		assert.NotContains(t, body, "/both-skill")
		// Counts stay global under a filter; the re-pull URL keeps the state.
		assert.Contains(t, body, "All (3)")
		assert.Contains(t, body, `id="skills-list" hx-get="/scripts/skills?state=active"`)
	})

	t.Run("invalid-state-falls-back-to-all", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills?state=bogus", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "/repo-skill")
		assert.Contains(t, body, "/home-skill")
		assert.Contains(t, body, "/both-skill")
	})
}

func TestMergeSkills(t *testing.T) {
	list := []skills.Skill{
		{Name: "both", Origin: skills.OriginHome, Path: "/home/both"},
		{Name: "both", Origin: skills.OriginRepo, Path: "/repo/both"},
		{Name: "home-only", Origin: skills.OriginHome},
		{Name: "repo-only", Origin: skills.OriginRepo},
	}

	cards := mergeSkills(list)
	require.Len(t, cards, 3)
	assert.Equal(t, "both", cards[0].Skill.Name)
	assert.Equal(t, skillStateSynced, cards[0].State)
	// The synced pair folds to the repo variant (D8).
	assert.Equal(t, skills.OriginRepo, cards[0].Skill.Origin)
	assert.Equal(t, "/repo/both", cards[0].Skill.Path)
	assert.Equal(t, skillStateActive, cards[1].State)
	assert.Equal(t, "home-only", cards[1].Skill.Name)
	assert.Equal(t, skillStateRepo, cards[2].State)
	assert.Equal(t, "repo-only", cards[2].Skill.Name)
	// equal versions carry no drift marker
	assert.Empty(t, cards[0].HomeVersion)

	drifted := mergeSkills([]skills.Skill{
		{Name: "bumped", Origin: skills.OriginHome, Version: "1.1"},
		{Name: "bumped", Origin: skills.OriginRepo, Version: "1.2"},
	})
	require.Len(t, drifted, 1)
	assert.Equal(t, skillStateSynced, drifted[0].State)
	assert.Equal(t, "1.1", drifted[0].HomeVersion)
	assert.Equal(t, "1.2", drifted[0].Skill.Version)

	assert.Empty(t, mergeSkills(nil))
}

func TestSkillsSync(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedOutput string
		_isDirKept      bool

		checkedDir string
		home       string
		response   *httptest.ResponseRecorder
		server     *Server
	}

	emptyForm := url.Values{}
	tests := make([]*testCase, 0)

	// foreign-dir-kept
	checkout := t.TempDir()
	providePlainSkill(t, filepath.Join(checkout, "skills", "demo"))
	server, home := newCheckoutServer(t, checkout, nil)
	tests = append(tests, &testCase{
		_id:             "foreign-dir-kept",
		_expectedOutput: "kept (not deployed by smine): " + filepath.Join(home, ".claude", "skills", "foreign-skill"),
		_isDirKept:      true,

		checkedDir: provideInstalledSkill(t, filepath.Join(home, ".claude", "skills"), "foreign-skill", false),
		home:       home,
		response:   httptest.NewRecorder(),
		server:     server,
	})

	// unselected-marked-dir-removed
	checkout = t.TempDir()
	providePlainSkill(t, filepath.Join(checkout, "skills", "demo"))
	writeRepoSkill(t, filepath.Join(checkout, "skills"), "unselected-skill", "1.0")
	chosen := install.Selection{Chosen: []string{demoPlainSkill}, Mode: skills.SelectionModeChosen}
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, chosen))
	tests = append(tests, &testCase{
		_id:             "unselected-marked-dir-removed",
		_expectedOutput: "pruned: " + filepath.Join(home, ".claude", "skills", "unselected-skill"),
		_isDirKept:      false,

		checkedDir: provideInstalledSkill(t, filepath.Join(home, ".claude", "skills"), "unselected-skill", true),
		home:       home,
		response:   httptest.NewRecorder(),
		server:     server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.server.Handler().ServeHTTP(test.response, formPost("/scripts/skills/sync", emptyForm))
			assert.Equal(t, http.StatusOK, test.response.Code)
			assert.Contains(t, test.response.Body.String(), test._expectedOutput)
			assert.FileExists(t, filepath.Join(test.home, ".claude", "skills", demoPlainSkill, "SKILL.md"))
			assert.FileExists(t, filepath.Join(test.home, ".codex", "skills", demoPlainSkill, "SKILL.md"))
			assert.Equal(t, test._isDirKept, isDirPresent(test.checkedDir))
		})
	}
}

func TestSkillVisibility(t *testing.T) {
	server, home := newCheckoutServer(t, t.TempDir(), nil)
	skillsHome := filepath.Join(home, ".claude", "skills")
	provideInstalledSkill(t, skillsHome, "demo", true)
	provideInstalledSkill(t, skillsHome, "demo-alias", true)
	provideInstalledSkill(t, skillsHome, "foreign-skill", false)
	require.NoError(t, os.WriteFile(server.settingsPath, []byte(`{"model": "x", "skillOverrides": {"user-own": "off"}}`), 0o644))
	storedOverrides := func() map[string]string {
		settings, err := config.Load(server.settingsPath)
		require.NoError(t, err)
		overrides, err := settings.SkillOverrides()
		require.NoError(t, err)
		return overrides
	}

	hidden := httptest.NewRecorder()
	server.Handler().ServeHTTP(hidden, formPost("/scripts/skills/visibility", url.Values{"visibility": {"off"}}))
	require.Equal(t, http.StatusOK, hidden.Code, hidden.Body.String())
	assert.Contains(t, hidden.Body.String(), "skillOverrides: 2 leaves off")
	assert.Equal(t, map[string]string{"demo": "off", "demo-alias": "off", "user-own": "off"}, storedOverrides())
	assert.Contains(t, stashedBases(t, server), filepath.Base(server.settingsPath))

	shown := httptest.NewRecorder()
	server.Handler().ServeHTTP(shown, formPost("/scripts/skills/visibility", url.Values{"visibility": {"on"}}))
	require.Equal(t, http.StatusOK, shown.Code, shown.Body.String())
	assert.Contains(t, shown.Body.String(), "skillOverrides: 2 leaves cleared")
	assert.Equal(t, map[string]string{"user-own": "off"}, storedOverrides())

	refused := httptest.NewRecorder()
	server.Handler().ServeHTTP(refused, formPost("/scripts/skills/visibility", url.Values{"visibility": {"maybe"}}))
	assert.Equal(t, http.StatusBadRequest, refused.Code)
}

func TestSelectionWith(t *testing.T) {
	type testCase struct {
		_id       string
		_expected install.Selection

		leaf      string
		selection install.Selection
	}

	tests := make([]*testCase, 0)

	// all-unchanged
	tests = append(tests, &testCase{
		_id:       "all-unchanged",
		_expected: install.Selection{Mode: skills.SelectionModeAll},

		leaf:      "alpha",
		selection: install.Selection{Mode: skills.SelectionModeAll},
	})

	// chosen-gains-leaf
	tests = append(tests, &testCase{
		_id:       "chosen-gains-leaf",
		_expected: install.Selection{Chosen: []string{"alpha", "beta"}, Mode: skills.SelectionModeChosen},

		leaf:      "beta",
		selection: install.Selection{Chosen: []string{"alpha"}, Mode: skills.SelectionModeChosen},
	})

	// chosen-with-leaf-unchanged
	tests = append(tests, &testCase{
		_id:       "chosen-with-leaf-unchanged",
		_expected: install.Selection{Chosen: []string{"alpha", "beta"}, Mode: skills.SelectionModeChosen},

		leaf:      "alpha",
		selection: install.Selection{Chosen: []string{"alpha", "beta"}, Mode: skills.SelectionModeChosen},
	})

	// none-becomes-chosen-leaf
	tests = append(tests, &testCase{
		_id:       "none-becomes-chosen-leaf",
		_expected: install.Selection{Chosen: []string{"alpha"}, Mode: skills.SelectionModeChosen},

		leaf:      "alpha",
		selection: install.Selection{Mode: skills.SelectionModeNone},
	})

	// rows-and-confirmation-kept
	tests = append(tests, &testCase{
		_id:       "rows-and-confirmation-kept",
		_expected: install.Selection{Chosen: []string{"alpha"}, IsConfirmed: true, Mode: skills.SelectionModeChosen, Providers: map[string]string{"PROVIDER_X": "x"}, Variables: map[string]string{"SKILL_A_CHECK": "make audit"}},

		leaf:      "alpha",
		selection: install.Selection{IsConfirmed: true, Mode: skills.SelectionModeNone, Providers: map[string]string{"PROVIDER_X": "x"}, Variables: map[string]string{"SKILL_A_CHECK": "make audit"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, selectionWith(test.selection, test.leaf))
		})
	}
}

func TestSelectionWithout(t *testing.T) {
	type testCase struct {
		_id       string
		_expected install.Selection

		available []string
		leaf      string
		selection install.Selection
	}

	available := []string{"alpha", "beta", "gamma"}
	tests := make([]*testCase, 0)

	// all-becomes-chosen-available-minus-leaf
	tests = append(tests, &testCase{
		_id:       "all-becomes-chosen-available-minus-leaf",
		_expected: install.Selection{Chosen: []string{"alpha", "gamma"}, Mode: skills.SelectionModeChosen},

		available: available,
		leaf:      "beta",
		selection: install.Selection{Mode: skills.SelectionModeAll},
	})

	// chosen-loses-leaf
	tests = append(tests, &testCase{
		_id:       "chosen-loses-leaf",
		_expected: install.Selection{Chosen: []string{"beta"}, Mode: skills.SelectionModeChosen},

		available: available,
		leaf:      "alpha",
		selection: install.Selection{Chosen: []string{"alpha", "beta"}, Mode: skills.SelectionModeChosen},
	})

	// last-leaf-becomes-none
	tests = append(tests, &testCase{
		_id:       "last-leaf-becomes-none",
		_expected: install.Selection{Mode: skills.SelectionModeNone},

		available: available,
		leaf:      "alpha",
		selection: install.Selection{Chosen: []string{"alpha"}, Mode: skills.SelectionModeChosen},
	})

	// none-stays-none
	tests = append(tests, &testCase{
		_id:       "none-stays-none",
		_expected: install.Selection{Mode: skills.SelectionModeNone},

		available: available,
		leaf:      "alpha",
		selection: install.Selection{Mode: skills.SelectionModeNone},
	})

	// rows-and-confirmation-kept
	tests = append(tests, &testCase{
		_id:       "rows-and-confirmation-kept",
		_expected: install.Selection{IsConfirmed: true, Mode: skills.SelectionModeNone, Providers: map[string]string{"PROVIDER_X": "x"}, Variables: map[string]string{"SKILL_A_CHECK": "make audit"}},

		available: available,
		leaf:      "alpha",
		selection: install.Selection{Chosen: []string{"alpha"}, IsConfirmed: true, Mode: skills.SelectionModeChosen, Providers: map[string]string{"PROVIDER_X": "x"}, Variables: map[string]string{"SKILL_A_CHECK": "make audit"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, selectionWithout(test.selection, test.leaf, test.available))
		})
	}
}

func TestSkillDeployRemove(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedBody      string
		_expectedCode      int
		_expectedSelection install.Selection
		_isDeployed        bool

		deployedDir string
		request     *http.Request
		response    *httptest.ResponseRecorder
		server      *Server
	}

	emptyForm := url.Values{}
	modeAll := install.Selection{Mode: skills.SelectionModeAll}
	tests := make([]*testCase, 0)

	// deploy-in-mode-none-saves-chosen-and-deploys
	checkout := t.TempDir()
	providePlainSkill(t, filepath.Join(checkout, "skills"))
	modeNone := install.Selection{Mode: skills.SelectionModeNone}
	server, home := newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeNone))
	deployedDir := filepath.Join(home, ".claude", "skills", demoPlainSkill)
	tests = append(tests, &testCase{
		_id:                "deploy-in-mode-none-saves-chosen-and-deploys",
		_expectedBody:      deployedDir,
		_expectedCode:      http.StatusOK,
		_expectedSelection: install.Selection{Chosen: []string{demoPlainSkill}, Mode: skills.SelectionModeChosen},
		_isDeployed:        true,

		deployedDir: deployedDir,
		request:     formPost("/scripts/skills/"+demoPlainSkill+"/deploy", emptyForm),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// deploy-unavailable-skill-op-error-record-unchanged
	checkout = t.TempDir()
	writeRoutineSkill(t, filepath.Join(checkout, "skills"))
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetMinimal, modeAll))
	tests = append(tests, &testCase{
		_id:                "deploy-unavailable-skill-op-error-record-unchanged",
		_expectedBody:      "unavailable (" + string(components.Routines) + ")",
		_expectedCode:      http.StatusOK,
		_expectedSelection: modeAll,
		_isDeployed:        false,

		deployedDir: filepath.Join(home, ".claude", "skills", routineSkillName),
		request:     formPost("/scripts/skills/"+routineSkillName+"/deploy", emptyForm),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// remove-in-mode-all-saves-chosen-without-it
	checkout = t.TempDir()
	providePlainSkill(t, filepath.Join(checkout, "skills"))
	writeRepoSkill(t, filepath.Join(checkout, "skills"), "other-skill", "1.0")
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	deployedDir = provideInstalledSkill(t, filepath.Join(home, ".claude", "skills"), demoPlainSkill, true)
	tests = append(tests, &testCase{
		_id:                "remove-in-mode-all-saves-chosen-without-it",
		_expectedBody:      "removed: " + deployedDir,
		_expectedCode:      http.StatusOK,
		_expectedSelection: install.Selection{Chosen: []string{"other-skill"}, Mode: skills.SelectionModeChosen},
		_isDeployed:        false,

		deployedDir: deployedDir,
		request:     formPost("/scripts/skills/"+demoPlainSkill+"/remove", emptyForm),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// unknown-name-404
	checkout = t.TempDir()
	providePlainSkill(t, filepath.Join(checkout, "skills"))
	server, home = newCheckoutServer(t, checkout, nil)
	tests = append(tests, &testCase{
		_id:                "unknown-name-404",
		_expectedBody:      "404 page not found",
		_expectedCode:      http.StatusNotFound,
		_expectedSelection: modeAll,
		_isDeployed:        false,

		deployedDir: filepath.Join(home, ".claude", "skills", "unknown-skill"),
		request:     formPost("/scripts/skills/unknown-skill/deploy", emptyForm),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.server.Handler().ServeHTTP(test.response, test.request)
			assert.Equal(t, test._expectedCode, test.response.Code)
			assert.Contains(t, test.response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedSelection, storedSelection(t, test.server))
			assert.Equal(t, test._isDeployed, isDirPresent(test.deployedDir))
		})
	}
}

func TestSkillDetailEnvironment(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedFragment string
		_isGateShown      bool

		path   string
		server *Server
	}

	hasGate := components.Available(components.Gate, runtime.GOOS)
	modeAll := install.Selection{Mode: skills.SelectionModeAll}
	buildRow := map[string]string{envSkillBuildName: "my-build"}
	detailPath := "/scripts/skills/repo/" + envSkillName
	tests := make([]*testCase, 0)

	// default-row-with-overrides
	checkout := t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, _ := newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetMinimal, modeAll, nil, buildRow))
	demoRepo := repos.Repo{Env: map[string]string{envSkillBuildName: "go build ./..."}, Name: "demo", Path: t.TempDir()}
	require.NoError(t, server.repoRegistry.Add(demoRepo))
	tests = append(tests, &testCase{
		_id:               "default-row-with-overrides",
		_expectedFragment: `<textarea name="SKILL_ENVSKILL_BUILD" rows="2" placeholder="go build ./..." aria-label="SKILL_ENVSKILL_BUILD">my-build</textarea></span>` + "\n" + `      <span class="meta"><a href="/scripts/skills/repo/env-skill?repo=demo" hx-get="/scripts/skills/repo/env-skill/env?repo=demo" hx-target="#skill-env" hx-swap="outerHTML">demo</a>: go build ./...</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// repo-selector-chips
	tests = append(tests, &testCase{
		_id:               "repo-selector-chips",
		_expectedFragment: `<a class="badge badge-dim" href="/scripts/skills/repo/env-skill" hx-get="/scripts/skills/repo/env-skill/env" hx-target="#skill-env" hx-swap="outerHTML">Default</a>` + "\n" + `    <a class="badge badge-ok" href="/scripts/skills/repo/env-skill?repo=demo" hx-get="/scripts/skills/repo/env-skill/env?repo=demo" hx-target="#skill-env" hx-swap="outerHTML">demo</a>`,
		_isGateShown:      false,

		path:   detailPath + "?repo=demo",
		server: server,
	})

	// repo-row-over-default
	tests = append(tests, &testCase{
		_id:               "repo-row-over-default",
		_expectedFragment: `<span class="badge badge-ok">repo value</span>` + "\n" + `      </span>` + "\n" + `    </div>` + "\n" + `    <div class="card-row card-foot">` + "\n" + `      <span class="card-control"><textarea name="SKILL_ENVSKILL_BUILD" rows="2" placeholder="my-build" aria-label="SKILL_ENVSKILL_BUILD">go build ./...</textarea></span>` + "\n" + `      <span class="meta">empty falls back to the default: my-build</span>`,
		_isGateShown:      false,

		path:   detailPath + "?repo=demo",
		server: server,
	})

	// repo-form-posts-to-repo
	tests = append(tests, &testCase{
		_id:               "repo-form-posts-to-repo",
		_expectedFragment: `<form hx-post="/scripts/skills/repo/env-skill/env/demo" hx-target="#skill-env" hx-swap="outerHTML" class="card-column configure-form">`,
		_isGateShown:      false,

		path:   detailPath + "?repo=demo",
		server: server,
	})

	// unknown-repo-shows-defaults
	tests = append(tests, &testCase{
		_id:               "unknown-repo-shows-defaults",
		_expectedFragment: `<a class="badge badge-ok" href="/scripts/skills/repo/env-skill" hx-get="/scripts/skills/repo/env-skill/env" hx-target="#skill-env" hx-swap="outerHTML">Default</a>`,
		_isGateShown:      false,

		path:   detailPath + "?repo=nope",
		server: server,
	})

	// tools-line
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, _ = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetMinimal, modeAll))
	tests = append(tests, &testCase{
		_id:               "tools-line",
		_expectedFragment: `<span class="label"><span class="meta">tools</span> <span class="badge badge-ok" title="">smine-rules current</span> </span>` + "\n" + `    <a class="meta env-tab-link" href="/scripts/skills/environment?skill=env-skill">Environment</a>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// advanced-holds-contract-rows
	tests = append(tests, &testCase{
		_id:               "advanced-holds-contract-rows",
		_expectedFragment: `<details class="advanced">` + "\n" + `    <summary>advanced: contracts and per-capability overrides</summary>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// provider-row-with-claims
	tests = append(tests, &testCase{
		_id:               "provider-row-with-claims",
		_expectedFragment: `<strong>smine-rules</strong> <code class="meta">PROVIDER_SMINE_RULES</code><br><span class="meta">provider · config-server · claims lanes@1, title-set@1</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// capability-row-beneath
	tests = append(tests, &testCase{
		_id:               "capability-row-beneath",
		_expectedFragment: `<span class="meta">provider smine-rules · lanes@1 — Writes the lanes.</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// deferred-contract-row-without-input
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), true)
	server, _ = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetMinimal, modeAll))
	tests = append(tests, &testCase{
		_id:               "deferred-contract-row-without-input",
		_expectedFragment: `<span class="meta">invocation · build@1 — Compiles the repo.</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// required-deferred-filled-at-invocation
	tests = append(tests, &testCase{
		_id:               "required-deferred-filled-at-invocation",
		_expectedFragment: `<br><span class="meta">repo-scoped — filled at invocation from the repo's value, the default above or the global default</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// required-unset-default
	tests = append(tests, &testCase{
		_id:               "required-unset-default",
		_expectedFragment: `<span class="badge badge-dim">required</span>` + "\n" + `        <span class="badge badge-dim">repo</span>` + "\n" + `        <span class="badge badge-warn">⚠ unset</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// set-install-row-under-advanced
	tests = append(tests, &testCase{
		_id:               "set-install-row-under-advanced",
		_expectedFragment: `<div class="env-capabilities">` + "\n" + `<div class="card configure-row env-program">`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// optional-default
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, _ = newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetMinimal, modeAll, nil, buildRow))
	tests = append(tests, &testCase{
		_id:               "optional-default",
		_expectedFragment: `<span class="badge badge-dim">optional</span>` + "\n" + `        <span class="badge badge-dim">repo</span>` + "\n" + `        <span class="badge badge-ok">default</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// global-default-shown
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), true)
	globalRecord := provideInstallRecord(install.PresetMinimal, modeAll)
	globalRecord.Skills.Defaults = map[string]string{"build": "make audit"}
	server, _ = newCheckoutServer(t, checkout, globalRecord)
	tests = append(tests, &testCase{
		_id:               "global-default-shown",
		_expectedFragment: `<span class="badge badge-dim">required</span>` + "\n" + `        <span class="badge badge-dim">repo</span>` + "\n" + `        <span class="badge badge-dim">global</span>` + "\n" + `      </span>` + "\n" + `    </div>` + "\n" + `    <div class="card-row card-foot">` + "\n" + `      <span class="card-control"><textarea name="SKILL_ENVSKILL_BUILD" rows="2" placeholder="make audit" aria-label="SKILL_ENVSKILL_BUILD"></textarea></span>` + "\n" + `      <span class="meta">empty uses the global default · no repo overrides it</span>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// head-unset-badge
	checkout = t.TempDir()
	provideRequiredLanesSkill(t, filepath.Join(checkout, "skills"))
	server, _ = newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetMinimal, modeAll, map[string]string{"PROVIDER_SMINE_RULES": "/nonexistent/smine-rules"}, buildRow))
	tests = append(tests, &testCase{
		_id:               "head-unset-badge",
		_expectedFragment: `<span class="badge badge-warn" title="required and unset: SKILL_ENVSKILL_LANES">⚠ 1 unset</span>` + "\n" + `</div>`,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// unset-required-install-in-visible-card
	tests = append(tests, &testCase{
		_id:               "unset-required-install-in-visible-card",
		_expectedFragment: `<span class="badge badge-warn">⚠ unset</span>` + "\n" + `      </span>` + "\n" + `    </div>` + "\n" + `    <div class="card-row card-foot">` + "\n" + `      <span class="meta">✗ `,
		_isGateShown:      false,

		path:   detailPath,
		server: server,
	})

	// gate-column-in-advanced
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home := newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	writeGateStub(t, home, "1. smine-rules lanes --round-dir x → denied: no allow rule matches; candidate rule Bash(smine-rules lanes *)")
	gateFragment := `exit 0 printed · exit other error · resolves ✓ · gate: denied: no allow rule matches; candidate rule Bash(smine-rules lanes *)</span>`
	if !hasGate {
		gateFragment = `exit 0 printed · exit other error · resolves ✓</span>`
	}
	tests = append(tests, &testCase{
		_id:               "gate-column-in-advanced",
		_expectedFragment: gateFragment,
		_isGateShown:      hasGate,

		path:   detailPath,
		server: server,
	})

	// gate-error-card
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, _ = newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetFull, modeAll, nil, buildRow))
	errorFragment := `<div class="card card-error"><span class="label">gate check unavailable: gateVerdicts:`
	if !hasGate {
		errorFragment = `<summary>Environment`
	}
	tests = append(tests, &testCase{
		_id:               "gate-error-card",
		_expectedFragment: errorFragment,
		_isGateShown:      hasGate,

		path:   detailPath,
		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			body := response.Body.String()
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, body, `<summary>Environment <span class="count-note">3</span></summary>`)
			assert.Contains(t, body, `<a href="/scripts/skills" class="tab-back">‹ Skills</a>`)
			assert.NotContains(t, body, `class="meta origin"`)
			assert.Contains(t, body, `hx-post="/scripts/skills/repo/`+envSkillName+`/env`)
			assert.Contains(t, body, test._expectedFragment)
			assert.Equal(t, test._isGateShown, strings.Contains(body, " · gate: "))
		})
	}
}

func TestSkillEnv(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedBody      string
		_expectedCode      int
		_expectedLocation  string
		_expectedProviders map[string]string
		_expectedRows      map[string]string
		_isDeployed        bool

		checkout    string
		deployedDir string
		request     *http.Request
		response    *httptest.ResponseRecorder
		server      *Server
	}

	modeAll := install.Selection{Mode: skills.SelectionModeAll}
	envPath := "/scripts/skills/repo/" + envSkillName + "/env"
	rulesProvider := "PROVIDER_SMINE_RULES"
	lanesName := "SKILL_ENVSKILL_LANES"
	savedDefault := "saved — the next invocation reads the new default"
	tests := make([]*testCase, 0)

	// default-save-no-deploy
	checkout := t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home := newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	form := url.Values{envSkillBuildName: {" go build ./...\r\ngo vet ./... "}, "OTHER_KEY": {"ignored"}}
	tests = append(tests, &testCase{
		_id:                "default-save-no-deploy",
		_expectedBody:      savedDefault,
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{envSkillBuildName: "go build ./...\ngo vet ./..."},
		_isDeployed:        false,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, form),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// install-value-save-deploys
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	form = url.Values{envSkillBuildName: {"go build ./..."}, lanesName: {"smine-rules"}}
	tests = append(tests, &testCase{
		_id:                "install-value-save-deploys",
		_expectedBody:      "saved and deployed",
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{envSkillBuildName: "go build ./...", lanesName: "smine-rules"},
		_isDeployed:        true,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, form),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// writes-row-and-redirects
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	form = url.Values{lanesName: {" smine-rules "}, "OTHER_KEY": {"ignored"}}
	tests = append(tests, &testCase{
		_id:                "writes-row-and-redirects",
		_expectedBody:      "saved and deployed",
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{lanesName: "smine-rules"},
		_isDeployed:        true,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, form),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// empty-field-removes-row
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetFull, modeAll, nil, map[string]string{envSkillBuildName: "make build"}))
	tests = append(tests, &testCase{
		_id:                "empty-field-removes-row",
		_expectedBody:      savedDefault,
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{},
		_isDeployed:        false,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{envSkillBuildName: {""}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// unknown-key-400
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	tests = append(tests, &testCase{
		_id:                "unknown-key-400",
		_expectedBody:      "unknown variable SKILL_ENVSKILL_NOPE",
		_expectedCode:      http.StatusBadRequest,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{},
		_isDeployed:        false,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{"SKILL_ENVSKILL_NOPE": {"x"}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// bad-value-400
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	tests = append(tests, &testCase{
		_id:                "bad-value-400",
		_expectedBody:      envSkillBuildName + ": value:",
		_expectedCode:      http.StatusBadRequest,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{},
		_isDeployed:        false,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{envSkillBuildName: {"make audit && rm"}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// provider-row-written
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	tests = append(tests, &testCase{
		_id:                "provider-row-written",
		_expectedBody:      "saved and deployed",
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{rulesProvider: "smine-rules"},
		_expectedRows:      map[string]string{},
		_isDeployed:        true,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{rulesProvider: {"smine-rules"}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// empty-removes-provider-row
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetFull, modeAll, map[string]string{rulesProvider: "smine-rules"}, nil))
	tests = append(tests, &testCase{
		_id:                "empty-removes-provider-row",
		_expectedBody:      "saved and deployed",
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{},
		_isDeployed:        true,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{rulesProvider: {""}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// unselected-skill-saved-not-deployed
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	modeNone := install.Selection{Mode: skills.SelectionModeNone}
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeNone))
	tests = append(tests, &testCase{
		_id:                "unselected-skill-saved-not-deployed",
		_expectedBody:      "saved — " + envSkillName + " is not selected, nothing deployed",
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{lanesName: "smine-rules"},
		_isDeployed:        false,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{lanesName: {"smine-rules"}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// orphan-rows-listed — kept in the record, listed by the widget for the Forget button
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetFull, modeAll, map[string]string{"PROVIDER_GHOST": "ghost"}, map[string]string{"SKILL_GHOST_X": "ghost"}))
	tests = append(tests, &testCase{
		_id:                "orphan-rows-listed",
		_expectedBody:      savedDefault,
		_expectedCode:      http.StatusOK,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{"PROVIDER_GHOST": "ghost"},
		_expectedRows:      map[string]string{envSkillBuildName: "my-build", "SKILL_GHOST_X": "ghost"},
		_isDeployed:        false,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost(envPath, url.Values{envSkillBuildName: {"my-build"}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// home-origin-404
	checkout = t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	server, home = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetFull, modeAll))
	provideInstalledSkill(t, filepath.Join(home, ".claude", "skills"), envSkillName, true)
	tests = append(tests, &testCase{
		_id:                "home-origin-404",
		_expectedBody:      "404 page not found",
		_expectedCode:      http.StatusNotFound,
		_expectedLocation:  "",
		_expectedProviders: map[string]string{},
		_expectedRows:      map[string]string{},
		_isDeployed:        true,

		checkout:    checkout,
		deployedDir: filepath.Join(home, ".claude", "skills", envSkillName),
		request:     formPost("/scripts/skills/home/"+envSkillName+"/env", url.Values{envSkillBuildName: {"x"}}),
		response:    httptest.NewRecorder(),
		server:      server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.server.Handler().ServeHTTP(test.response, test.request)
			rows := storedSkillRows(t, test.server)
			assert.Equal(t, test._expectedCode, test.response.Code)
			assert.Contains(t, test.response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedLocation, test.response.Header().Get("Location"))
			assert.Equal(t, test._expectedProviders, rows.Providers)
			assert.Equal(t, test._expectedRows, rows.Skills)
			assert.Equal(t, test._isDeployed, isDirPresent(test.deployedDir))
		})
	}
}

func TestSkillEnvOrphans(t *testing.T) {
	checkout := t.TempDir()
	provideEnvSkill(t, filepath.Join(checkout, "skills"), false)
	providers := map[string]string{"PROVIDER_GHOST": "ghost"}
	variables := map[string]string{envSkillBuildName: "my-build", "SKILL_GHOST_X": "ghost"}
	record := provideSkillRowsRecord(install.PresetFull, install.Selection{Mode: skills.SelectionModeAll}, providers, variables)
	server, _ := newCheckoutServer(t, checkout, record)
	skill, err := server.repoSkillNamed(envSkillName)
	require.NoError(t, err)

	view := server.skillEnv(t.Context(), "", skill)

	assert.Equal(t, []string{"PROVIDER_GHOST", "SKILL_GHOST_X"}, view.Orphans)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/"+envSkillName+"/env", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()
	assert.Contains(t, body, "Rows no skill declares")
	assert.Contains(t, body, `hx-post="/scripts/skills/environment/forget"`)
	assert.Contains(t, body, `<input type="hidden" name="key" value="SKILL_GHOST_X">`)
	assert.Contains(t, body, `<input type="hidden" name="skill" value="`+envSkillName+`">`)
}

func TestSkillCardUnavailable(t *testing.T) {
	checkout := t.TempDir()
	skillsRepo := filepath.Join(checkout, "skills")
	writeRoutineSkill(t, skillsRepo)
	providePlainSkill(t, skillsRepo)
	modeAll := install.Selection{Mode: skills.SelectionModeAll}
	server, _ := newCheckoutServer(t, checkout, provideInstallRecord(install.PresetMinimal, modeAll))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	t.Run("missing-component-badge", func(t *testing.T) {
		assert.Contains(t, body, "unavailable ("+string(components.Routines)+")")
	})

	t.Run("red-deploy-button-when-missing", func(t *testing.T) {
		blocked := `class="icon-button skill-deploy skill-blocked" hx-post="/scripts/skills/` + routineSkillName + `/deploy"`
		assert.Contains(t, body, blocked)
		assert.Contains(t, body, `title="Skill `+routineSkillName+` unavailable (`+string(components.Routines)+`)"`)
		// The available skill carries the plain button.
		assert.Contains(t, body, `class="icon-button skill-deploy" hx-post="/scripts/skills/`+demoPlainSkill+`/deploy"`)
	})
}

func TestSkillsGroupingAndBadges(t *testing.T) {
	writeHomeSkill := func(t *testing.T, root, name string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\nversion: 1.0\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillsHome := filepath.Join(dir, "home")
	writeRepoSkill(t, skillsRepo, "fam/fam-a", "1.0")
	writeRepoSkill(t, skillsRepo, "fam/fam-b", "1.0")
	writeRepoSkill(t, skillsRepo, "solo", "1.0")
	writeHomeSkill(t, skillsHome, "fam-a")
	writeHomeSkill(t, skillsHome, "extra")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	t.Run("groups-and-state-badges", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "<summary>fam (2)")
		assert.NotContains(t, body, "<summary>solo (1)")
		assert.Contains(t, body, "/solo")
		assert.Contains(t, body, ">Synced<")
		assert.Contains(t, body, ">Active only<")
		assert.Contains(t, body, ">Uninstalled<")
		// Family summary carries the invocation sum; no drift, no alert.
		assert.Contains(t, body, "0 invocations")
		assert.NotContains(t, body, "Update needed")
		// Matching versions grey the update icon; only the synced skill carries remove.
		assert.Contains(t, body, `disabled title="up to date"`)
		assert.NotContains(t, body, "update /fam-a")
		assert.Equal(t, 1, strings.Count(body, "skill-remove"))
	})
}

func TestSkillGroups(t *testing.T) {
	list := []skillCard{
		{Invocations: 4, Skill: skills.Skill{Group: "smine", Name: "smine"}},
		{HomeVersion: "1.1", Invocations: 2, Skill: skills.Skill{Group: "smine", Name: "smine-memory", Version: "1.2"}},
		{Skill: skills.Skill{Group: "smine", Name: "smine-context"}, Unset: []string{"SKILL_SMINECONTEXT_A", "SKILL_SMINECONTEXT_B"}},
		{Invocations: 7, Skill: skills.Skill{Name: "jq"}},
		{Skill: skills.Skill{Group: "skillroutine", Name: "skillroutine-create"}, State: skillStateRepo},
		{Skill: skills.Skill{Group: "skillroutine", Name: "skillroutine-eval"}, State: skillStateRepo},
		{Skill: skills.Skill{Group: "concept", Name: "clarify"}},
		{Skill: skills.Skill{Group: "concept", Name: "concept"}},
	}

	groups := skillGroups(list)
	require.Len(t, groups, 4)
	// Strictly alphabetical by family — single-member jq interleaves.
	assert.Equal(t, "concept", groups[0].Family)
	assert.Len(t, groups[0].Skills, 2)
	assert.Equal(t, "jq", groups[1].Family)
	assert.Len(t, groups[1].Skills, 1)
	assert.Equal(t, 7, groups[1].Invocations)
	assert.Equal(t, "skillroutine", groups[2].Family)
	assert.Len(t, groups[2].Skills, 2)
	// Aggregates: invocations summed, drifted members counted (D3).
	assert.Equal(t, "smine", groups[3].Family)
	assert.Len(t, groups[3].Skills, 3)
	assert.Equal(t, 6, groups[3].Invocations)
	assert.Equal(t, 1, groups[3].UpdatesNeeded)
	// A member's unset required variables surface on the group, so a collapsed group still shows them.
	assert.Equal(t, []string{"smine-context: SKILL_SMINECONTEXT_A, SKILL_SMINECONTEXT_B"}, groups[3].Unset)
	assert.Equal(t, 2, groups[3].UnsetCount)
	assert.Empty(t, groups[2].Unset)
	// Members only the repo holds count as uninstalled.
	assert.Equal(t, 2, groups[2].Uninstalled)
	assert.Zero(t, groups[3].Uninstalled)
	assert.Empty(t, skillGroups(nil))
}

func TestSkillsIndexVersionDriftBadge(t *testing.T) {
	writeVersioned := func(t *testing.T, root, name, version string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\nversion: " + version + "\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillsHome := filepath.Join(dir, "home")
	writeRepoSkill(t, skillsRepo, "bumped-skill", "1.2")
	writeVersioned(t, skillsHome, "bumped-skill", "1.1")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	// One card, still under the Synced tab; the version badge and the live update icon carry the drift (D7).
	assert.Contains(t, body, "Synced (1)")
	assert.Contains(t, body, `class="badge badge-action" title="installed v1.1, repo v1.2">v1.2</span>`)
	assert.Contains(t, body, `hx-post="/scripts/skills/bumped-skill/deploy" hx-target="#op-result" title="update v1.1 → v1.2"`)
}

func TestSkillsIndexUnsetBadge(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedFragment string
		_isBadgeShown     bool

		server *Server
	}

	modeAll := install.Selection{Mode: skills.SelectionModeAll}
	tests := make([]*testCase, 0)

	// unset-required-shows
	checkout := t.TempDir()
	provideRequiredLanesSkill(t, filepath.Join(checkout, "skills"))
	server, _ := newCheckoutServer(t, checkout, provideSkillRowsRecord(install.PresetMinimal, modeAll, map[string]string{"PROVIDER_SMINE_RULES": "/nonexistent/smine-rules"}, nil))
	tests = append(tests, &testCase{
		_id:               "unset-required-shows",
		_expectedFragment: `<span class="badge badge-warn" title="required and unset: SKILL_ENVSKILL_LANES">⚠ 1 unset</span>`,
		_isBadgeShown:     true,

		server: server,
	})

	// all-set-none
	checkout = t.TempDir()
	provideRequiredLanesSkill(t, filepath.Join(checkout, "skills"))
	server, _ = newCheckoutServer(t, checkout, provideInstallRecord(install.PresetMinimal, modeAll))
	tests = append(tests, &testCase{
		_id:               "all-set-none",
		_expectedFragment: `href="/scripts/skills/repo/` + envSkillName + `"`,
		_isBadgeShown:     false,

		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
			body := response.Body.String()
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, body, test._expectedFragment)
			assert.Equal(t, test._isBadgeShown, strings.Contains(body, "badge-warn"))
		})
	}
}

func TestSkillsSmokeBadge(t *testing.T) {
	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	smokeDir := filepath.Join(dir, "smoke")
	writeRepoSkill(t, skillsRepo, "smoked-skill", "1.2")
	writeRepoSkill(t, skillsRepo, "plain-skill", "1.0")
	writeRepoSkill(t, skillsRepo, "broken-skill", "1.0")
	writeRepoSkill(t, skillsRepo, "stale-skill", "1.2")
	coverage := skilleval.SmokeCoverage{
		AnchorsDeclared:    4,
		AnchorsExpected:    []string{"base", "lanes", "round", "cleanup"},
		AnchorsExpectedHit: 4,
		AnchorsHit:         4,
		ChecksDeclared:     4,
		ChecksPassed:       2,
	}
	endedAt := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	current := &skilleval.ArmResult{EndedAt: endedAt, SkillVersion: "1.2", Smoke: &skilleval.SmokeResult{Coverage: coverage, Verdict: statusPass}}
	writeSmokeRecord(t, smokeDir, "smoked-skill", "1.2", &skilleval.SmokeRecord{Result: current})
	stale := &skilleval.ArmResult{EndedAt: endedAt, SkillVersion: "1.1", Smoke: &skilleval.SmokeResult{Coverage: coverage, Verdict: statusPass}}
	writeSmokeRecord(t, smokeDir, "stale-skill", "1.1", &skilleval.SmokeRecord{Result: stale})
	// The malformed record's text is the input under test, so it is written raw.
	brokenDir := filepath.Join(smokeDir, "broken-skill", "1.0", "runs", "A")
	require.NoError(t, os.MkdirAll(brokenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(brokenDir, "result.json"), []byte("not json {"), 0o644))
	server := newTestServer(t, &Options{SkillsHome: filepath.Join(dir, "home"), SkillsRepo: skillsRepo, SmokeDir: smokeDir})
	badgeText := "smoke pass 2026-10-02 · anchors 4/4 (4), checks 2/4"
	badgeTitle := `title="smoke of v`

	t.Run("index-card-shows-badge", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		// The card shows the bare verdict; date and coverage move into the title.
		assert.Contains(t, body, `class="badge badge-ok" title="smoke of v1.2 2026-10-02 · anchors 4/4 (4), checks 2/4">pass</span>`)
		assert.NotContains(t, body, badgeText)
		// Only the current record carries a smoke-of title; the stale one names itself, the malformed record shows none.
		assert.Equal(t, 1, strings.Count(body, badgeTitle))
	})

	t.Run("detail-head-shows-badge", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/smoked-skill", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), badgeText)
	})

	t.Run("stale-record-dim", func(t *testing.T) {
		indexResponse := httptest.NewRecorder()
		server.Handler().ServeHTTP(indexResponse, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
		require.Equal(t, http.StatusOK, indexResponse.Code)
		assert.Contains(t, indexResponse.Body.String(), `class="badge badge-dim" title="stale — no smoke of v1.2 yet; v1.1 2026-10-02 · anchors 4/4 (4), checks 2/4">pass · stale</span>`)

		detailResponse := httptest.NewRecorder()
		server.Handler().ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/stale-skill", nil))
		require.Equal(t, http.StatusOK, detailResponse.Code)
		assert.Contains(t, detailResponse.Body.String(), `class="badge badge-dim" title="no smoke of v1.2 yet — v1.1 2026-10-02">smoke pass v1.1 · stale</span>`)
	})

	t.Run("no-record-no-badge", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/plain-skill", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), badgeTitle)
	})

	t.Run("malformed-record-no-badge", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/broken-skill", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), badgeTitle)
	})
}

func TestLoadChangelog(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []skillgraph.ChangelogEntry
		_shouldPass bool

		dir string
	}

	tests := make([]*testCase, 0)

	// no-model
	tests = append(tests, &testCase{
		_id:         "no-model",
		_expected:   nil,
		_shouldPass: true,

		dir: t.TempDir(),
	})

	// model
	modelDir := filepath.Join(t.TempDir(), "demo")
	writeSkillModel(t, modelDir, "---\nname: demo\ndescription: d\nversion: 1.1\n---\n\n# Demo\n", `[{"version": "1.1", "date": "2026-07-10", "text": "added X"}]`)
	tests = append(tests, &testCase{
		_id:         "model",
		_expected:   []skillgraph.ChangelogEntry{{Date: "2026-07-10", Text: "added X", Version: "1.1"}},
		_shouldPass: true,

		dir: modelDir,
	})

	// invalid-model
	invalidDir := t.TempDir()
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(invalidDir), []byte("not cue {"), 0o644))
	tests = append(tests, &testCase{
		_id:         "invalid-model",
		_expected:   nil,
		_shouldPass: false,

		dir: invalidDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			changelog, err := loadChangelog(test.dir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, changelog)
		})
	}
}

func TestSkillTitleStates(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []skillTitleState
		_shouldPass bool

		skill *skills.Skill
	}

	demoStates := []skillTitleState{{Entry: demoTitleEntry, Meaning: "base awaits the human", State: demoTitleState}}
	skillsRepo := filepath.Join(t.TempDir(), "repo")
	skillsHome := filepath.Join(t.TempDir(), "home")
	tests := make([]*testCase, 0)

	// repo-with-declaration
	demoDir := provideDemoGraphSkill(t, skillsRepo)
	tests = append(tests, &testCase{
		_id:         "repo-with-declaration",
		_expected:   demoStates,
		_shouldPass: true,

		skill: &skills.Skill{Origin: skills.OriginRepo, Path: demoDir},
	})

	// repo-without
	providePlainSkill(t, skillsRepo)
	tests = append(tests, &testCase{
		_id:         "repo-without",
		_expected:   nil,
		_shouldPass: true,

		skill: &skills.Skill{Origin: skills.OriginRepo, Path: filepath.Join(skillsRepo, demoPlainSkill)},
	})

	// home-with-title-json
	tests = append(tests, &testCase{
		_id:         "home-with-title-json",
		_expected:   demoStates,
		_shouldPass: true,

		skill: &skills.Skill{Origin: skills.OriginHome, Path: provideDeployedSkill(t, demoDir, skillsHome)},
	})

	// home-invalid-title-json
	invalidDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, sessiontitle.FileName), []byte(`{"schema": 2}`), 0o644))
	tests = append(tests, &testCase{
		_id:         "home-invalid-title-json",
		_expected:   nil,
		_shouldPass: false,

		skill: &skills.Skill{Origin: skills.OriginHome, Path: invalidDir},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			states, err := skillTitleStates(test.skill)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, states)
		})
	}
}

func TestSkillInvocations(t *testing.T) {
	type testCase struct {
		_id                   string
		_expected             []sessions.Invocation
		_expectedSourceLength int

		invocations map[string][]sessions.Invocation
		skill       *skills.Skill
	}

	ownInvocation := sessions.Invocation{LastActive: "2026-07-10T10:00:00Z", SessionId: "own"}
	aliasInvocation := sessions.Invocation{LastActive: "2026-07-12T09:00:00Z", SessionId: "alias"}
	aliasedSkill := &skills.Skill{Aliases: []string{"pk"}, Name: "peek"}
	tests := make([]*testCase, 0)

	// own-name-only
	tests = append(tests, &testCase{
		_id:                   "own-name-only",
		_expected:             []sessions.Invocation{ownInvocation},
		_expectedSourceLength: 1,

		invocations: map[string][]sessions.Invocation{"peek": {ownInvocation}},
		skill:       &skills.Skill{Name: "peek"},
	})

	// own-and-alias
	tests = append(tests, &testCase{
		_id:                   "own-and-alias",
		_expected:             []sessions.Invocation{ownInvocation, aliasInvocation},
		_expectedSourceLength: 1,

		invocations: map[string][]sessions.Invocation{"peek": {ownInvocation}, "pk": {aliasInvocation}},
		skill:       aliasedSkill,
	})

	// alias-only
	tests = append(tests, &testCase{
		_id:                   "alias-only",
		_expected:             []sessions.Invocation{aliasInvocation},
		_expectedSourceLength: 0,

		invocations: map[string][]sessions.Invocation{"pk": {aliasInvocation}},
		skill:       aliasedSkill,
	})

	// none
	tests = append(tests, &testCase{
		_id:                   "none",
		_expected:             nil,
		_expectedSourceLength: 0,

		invocations: map[string][]sessions.Invocation{},
		skill:       aliasedSkill,
	})

	// source-slice-untouched
	tests = append(tests, &testCase{
		_id:                   "source-slice-untouched",
		_expected:             []sessions.Invocation{ownInvocation, ownInvocation, aliasInvocation, aliasInvocation},
		_expectedSourceLength: 2,

		invocations: map[string][]sessions.Invocation{"peek": {ownInvocation, ownInvocation}, "pk": {aliasInvocation, aliasInvocation}},
		skill:       aliasedSkill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, skillInvocations(test.invocations, test.skill))
			assert.Len(t, test.invocations[test.skill.Name], test._expectedSourceLength)
		})
	}
}

// isDirPresent reports whether path is an existing directory.
func isDirPresent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// newCheckoutServer points a test server at checkout's skills under a fresh
// HOME and stores record as its install.json; a nil record stores none, so
// the server reads Full. Its repo registry is an empty temp file. It returns
// the server and the home.
func newCheckoutServer(t *testing.T, checkout string, record *install.Record) (*Server, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	options := &Options{
		RepoDir:    checkout,
		ReposPath:  filepath.Join(t.TempDir(), "repos.json"),
		SkillsHome: filepath.Join(home, ".claude", "skills"),
		SkillsRepo: filepath.Join(checkout, "skills"),
	}
	server := newTestServer(t, options)
	if record != nil {
		require.NoError(t, install.Save(server.machine, *record))
	}
	return server, home
}

// provideEnvSkill writes the repo skill envSkillName into skillsRepo, a skills
// root with an empty vocabulary and testCatalogCue: one step reading a BUILD
// command, required when isBuildRequired, and an optional LANES program.
func provideEnvSkill(t *testing.T, skillsRepo string, isBuildRequired bool) {
	t.Helper()
	installRulesStub(t)
	skillDir := filepath.Join(skillsRepo, envSkillName)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsRepo, skillgraph.FileVocabulary), []byte("atoms: {}\n"), 0o644))
	manifest := "---\nname: " + envSkillName + "\ndescription: d\nversion: 1.0\nformat: 2\n---\n# Env\n\n## Run\n\n" +
		"**SKILL-ENVSKILL-RUN-001** `[step]` **Run**\n\nRUN `$" + envSkillBuildName + "`, then `$SKILL_ENVSKILL_LANES lanes`.\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "changelog.json"), []byte(demoGraphChange), 0o644))
	skill, err := skillimport.Import(skillDir)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(skillDir, "changelog.json")))
	require.NoError(t, os.Remove(filepath.Join(skillDir, "SKILL.md")))

	build := &skillgraph.Variable{Interface: "build@1", Required: isBuildRequired}
	lanes := &skillgraph.Variable{Interface: "lanes@1"}
	skill.Catalog = provideSkillsCatalog(t, skillsRepo)
	skill.Env = map[string]*skillgraph.Variable{"BUILD": build, "LANES": lanes}
	require.NoError(t, skillgraph.Save(skillDir, skill))
}

// provideRequiredLanesSkill is provideEnvSkill with the install-scoped LANES variable required.
func provideRequiredLanesSkill(t *testing.T, skillsRepo string) {
	t.Helper()
	provideEnvSkill(t, skillsRepo, false)
	skillDir := filepath.Join(skillsRepo, envSkillName)
	skill, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	skill.Env["LANES"].Required = true
	require.NoError(t, skillgraph.Save(skillDir, skill))
}

// provideSkillRowsRecord is provideInstallRecord with rows as the record's
// SKILL_* rows and providers as its PROVIDER_* rows.
func provideSkillRowsRecord(preset string, selection install.Selection, providers, rows map[string]string) *install.Record {
	record := provideInstallRecord(preset, selection)
	record.Skills.Providers = providers
	record.Skills.Variables = rows
	return record
}

// storedSkillRows reads the server's record back as skill rows, maps never nil.
func storedSkillRows(t *testing.T, server *Server) *skills.Rows {
	t.Helper()
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	return install.SkillRows(record)
}

// provideInstallRecord is a record of preset with selection; Save expands
// the preset's components.
func provideInstallRecord(preset string, selection install.Selection) *install.Record {
	record := install.FullRecord(runtime.GOOS)
	record.Preset = preset
	record.Skills = selection
	return &record
}

// provideInstalledSkill writes an installed skill dir <skillsHome>/<name>,
// with the deploy marker when isMarked, and returns the dir.
func provideInstalledSkill(t *testing.T, skillsHome, name string, isMarked bool) string {
	t.Helper()
	dir := filepath.Join(skillsHome, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644))
	if isMarked {
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".smine-deployed"), []byte{}, 0o644))
	}
	return dir
}

// storedSelection reads the skill selection of the server's install.json.
func storedSelection(t *testing.T, server *Server) install.Selection {
	t.Helper()
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	return record.Skills
}

// writeRoutineSkill writes the repo skill routineSkillName into skillsRepo,
// requiring the Routines component.
func writeRoutineSkill(t *testing.T, skillsRepo string) {
	t.Helper()
	writeRepoSkill(t, skillsRepo, routineSkillName, "1.0")
	skillDir := filepath.Join(skillsRepo, routineSkillName)
	skill, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	skill.Requires = []components.ID{components.Routines}
	require.NoError(t, skillgraph.Save(skillDir, skill))
}

// writeSmokeRecord writes record as the smoke record of skill at version, in the cache layout
// <smokeDir>/<skill>/<version>/run.json and runs/A/{result,runtime}.json; a nil part stays unwritten.
func writeSmokeRecord(t *testing.T, smokeDir, skill, version string, record *skilleval.SmokeRecord) {
	t.Helper()
	versionDir := filepath.Join(smokeDir, skill, version)
	armDir := filepath.Join(versionDir, "runs", "A")
	require.NoError(t, os.MkdirAll(armDir, 0o755))
	if record.Result != nil {
		content, err := json.Marshal(record.Result)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(armDir, "result.json"), content, 0o644))
	}
	if record.Manifest != nil {
		content, err := json.Marshal(record.Manifest)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(versionDir, "run.json"), content, 0o644))
	}
	if record.Runtime != nil {
		content, err := json.Marshal(record.Runtime)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(armDir, "runtime.json"), content, 0o644))
	}
}

// writeSkillModel writes a markdown-era skill — SKILL.md and changelog.json
// — into dir and imports it into a saved model file, the way skillmodel
// import does; only the model stays behind.
func writeSkillModel(t *testing.T, dir, manifest, changelog string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(changelog), 0o644))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	require.NoError(t, os.Remove(filepath.Join(dir, "changelog.json")))
	require.NoError(t, os.Remove(filepath.Join(dir, "SKILL.md")))
}

// writeAliasedHomeSkill writes the deployed skill peek at version and the
// pk alias copy the deploy writes beside it into skillsHome.
func writeAliasedHomeSkill(t *testing.T, skillsHome, version string) {
	t.Helper()
	manifests := map[string]string{
		"peek": "---\nname: peek\ndescription: d\nversion: " + version + "\naliases: pk\n---\n",
		"pk":   "---\nname: pk\ntitle-code: peek\ndescription: d\nversion: " + version + "\naliases: pk\n---\n",
	}
	for name, manifest := range manifests {
		dir := filepath.Join(skillsHome, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644))
	}
}

// writeAliasedRepoSkill writes the repo skill peek, aliased pk, at version
// into skillsRepo, with changelog as its version history.
func writeAliasedRepoSkill(t *testing.T, skillsRepo, version, changelog string) {
	t.Helper()
	manifest := "---\nname: peek\ndescription: d\nversion: " + version + "\naliases: pk\n---\n\n# Peek\n"
	writeSkillModel(t, filepath.Join(skillsRepo, "peek"), manifest, changelog)
}

// writeRepoSkill writes a minimal repo skill <root>/<name> as a model whose
// version is the one changelog row's; the leaf of name is the skill name.
func writeRepoSkill(t *testing.T, root, name, version string) {
	t.Helper()
	leaf := filepath.Base(name)
	manifest := "---\nname: " + leaf + "\ndescription: d\nversion: " + version + "\n---\n\n# " + leaf + "\n"
	changelog := `[{"version": "` + version + `", "date": "2000-01-01", "text": "fixture"}]`
	writeSkillModel(t, filepath.Join(root, name), manifest, changelog)
}

// provideDeployedSkill writes the deployed render of the repo skill in
// skillDir into <skillsHome>/<leaf> and returns that dir.
func TestMarkStatusColumns(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedSmoke bool
		_expectedUnset bool

		cards []skillCard
	}

	tests := make([]*testCase, 0)

	// none-filled
	tests = append(tests, &testCase{
		_id:            "none-filled",
		_expectedSmoke: false,
		_expectedUnset: false,

		cards: []skillCard{{}, {}},
	})

	// one-unset-opens-every-card
	tests = append(tests, &testCase{
		_id:            "one-unset-opens-every-card",
		_expectedSmoke: false,
		_expectedUnset: true,

		cards: []skillCard{{}, {Unset: []string{"SKILL_DEMO_BUILD"}}},
	})

	// smoke-and-unset
	tests = append(tests, &testCase{
		_id:            "smoke-and-unset",
		_expectedSmoke: true,
		_expectedUnset: true,

		cards: []skillCard{{Smoke: &smokeBadge{Verdict: "pass"}}, {Unset: []string{"SKILL_DEMO_TEST"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			markStatusColumns(test.cards)
			for _, card := range test.cards {
				assert.Equal(t, test._expectedSmoke, card.HasSmokeColumn)
				assert.Equal(t, test._expectedUnset, card.HasUnsetColumn)
			}
		})
	}
}

func provideDeployedSkill(t *testing.T, skillDir, skillsHome string) string {
	t.Helper()
	skill, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	var canonical skillgraph.RenderOptions
	files, err := skillgraph.RenderDeployed(canonical, skill)
	require.NoError(t, err)

	deployedDir := filepath.Join(skillsHome, filepath.Base(skillDir))
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	for _, file := range files {
		require.NoError(t, os.WriteFile(filepath.Join(deployedDir, file.Path), file.Content, 0o644))
	}
	return deployedDir
}
