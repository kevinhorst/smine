package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gateGroupJson = `{"smineId": "gate-pretooluse-toolpolicy", "smineName": "Tool policy gate", "smineVersion": "1.0.0", "hooks": [{"command": "toolpolicy gate", "type": "command"}], "matcher": "Bash"}`

func TestResolveScript(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedPath    string

		command string
	}

	dir := t.TempDir()
	writeFile := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0o755))
		return path
	}
	scriptBody := "#!/bin/sh\necho hook\n"
	scriptPath := writeFile("script.sh", []byte(scriptBody))
	emptyPath := writeFile("empty.sh", nil)
	binaryPath := writeFile("binary", []byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01})
	nulPath := writeFile("nul.txt", []byte("text\x00text"))
	atCapBody := strings.Repeat("a", maxHookScriptBytes)
	atCapPath := writeFile("at-cap.sh", []byte(atCapBody))
	overCapPath := writeFile("over-cap.sh", []byte(strings.Repeat("a", maxHookScriptBytes+1)))

	tests := make([]*testCase, 0)

	// text-script
	tests = append(tests, &testCase{
		_id:              "text-script",
		_expectedContent: scriptBody,
		_expectedPath:    scriptPath,

		command: scriptPath,
	})

	// text-script-with-args
	tests = append(tests, &testCase{
		_id:              "text-script-with-args",
		_expectedContent: scriptBody,
		_expectedPath:    scriptPath,

		command: scriptPath + " --flag value",
	})

	// at-size-cap
	tests = append(tests, &testCase{
		_id:              "at-size-cap",
		_expectedContent: atCapBody,
		_expectedPath:    atCapPath,

		command: atCapPath,
	})

	// over-size-cap
	tests = append(tests, &testCase{
		_id:              "over-size-cap",
		_expectedContent: "",
		_expectedPath:    "",

		command: overCapPath,
	})

	// compiled-binary
	tests = append(tests, &testCase{
		_id:              "compiled-binary",
		_expectedContent: "",
		_expectedPath:    "",

		command: binaryPath + " hook",
	})

	// nul-byte
	tests = append(tests, &testCase{
		_id:              "nul-byte",
		_expectedContent: "",
		_expectedPath:    "",

		command: nulPath,
	})

	// empty-file
	tests = append(tests, &testCase{
		_id:              "empty-file",
		_expectedContent: "",
		_expectedPath:    emptyPath,

		command: emptyPath,
	})

	// directory
	tests = append(tests, &testCase{
		_id:              "directory",
		_expectedContent: "",
		_expectedPath:    "",

		command: dir,
	})

	// missing-file
	tests = append(tests, &testCase{
		_id:              "missing-file",
		_expectedContent: "",
		_expectedPath:    "",

		command: filepath.Join(dir, "absent.sh"),
	})

	// relative-command
	tests = append(tests, &testCase{
		_id:              "relative-command",
		_expectedContent: "",
		_expectedPath:    "",

		command: "echo inline",
	})

	// empty-command
	tests = append(tests, &testCase{
		_id:              "empty-command",
		_expectedContent: "",
		_expectedPath:    "",

		command: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path, content := resolveScript(test.command)

			assert.Equal(t, test._expectedPath, path)
			assert.Equal(t, test._expectedContent, content)
		})
	}
}

// writeComponentFragment writes one component fragment into the template dir.
func writeComponentFragment(t *testing.T, templateDir, id, hooks string) {
	t.Helper()
	fragment := `{"hooks": ` + hooks + `}`
	path := filepath.Join(templateDir, config.ComponentFragmentDir, id+".json")
	require.NoError(t, os.WriteFile(path, []byte(fragment), 0644))
}

// writeGateFragment writes a Claude settings template whose gate component
// fragment declares one PreToolUse group and returns the template dir.
func writeGateFragment(t *testing.T) string {
	t.Helper()
	templateDir := writeClaudeTemplate(t, t.TempDir(), `{}`)
	writeComponentFragment(t, templateDir, "gate", `{"PreToolUse": [`+gateGroupJson+`]}`)
	return templateDir
}

// newHooksServer serves a Claude home holding record and a checkout whose
// gate fragment declares one group, live in settings.json.
func newHooksServer(t *testing.T, record install.Record) *Server {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	templateDir := writeGateFragment(t)
	claudeHome := t.TempDir()
	settingsPath := filepath.Join(claudeHome, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"hooks": {"PreToolUse": [`+gateGroupJson+`]}}`), 0644))
	require.NoError(t, install.Save(install.Env{ClaudeHome: claudeHome, GOOS: runtime.GOOS}, record))
	options := &Options{
		ClaudeFragmentPath: templateDir,
		RepoDir:            filepath.Dir(filepath.Dir(templateDir)),
		SettingsPath:       settingsPath,
	}
	return newTestServer(t, options)
}

// provideHooksRecord selects Config server and Skills only — a Save on any
// other card stores the record and the apply refuses — with context overrides.
func provideHooksRecord(t *testing.T, overrides map[string]string) install.Record {
	t.Helper()
	record := install.FullRecord(runtime.GOOS)
	record.Components = components.Close([]components.ID{components.ConfigServer, components.Skills})
	record.Preset = install.PresetCustom
	for key, text := range overrides {
		setting, ok := components.LookupSetting(components.Context, key)
		require.True(t, ok)
		value, err := components.ParseText(&setting, text)
		require.NoError(t, err)
		require.NoError(t, install.SetOverride(&record, components.Context, key, value))
	}
	return record
}

// provideGateRecord is provideHooksRecord with the Permission gate selected, so its groups' switches are live.
func provideGateRecord(t *testing.T) install.Record {
	t.Helper()
	record := provideHooksRecord(t, nil)
	record.Components = components.Close(append(record.Components, components.Gate))
	return record
}

func getHooks(t *testing.T, server *Server, target string) string {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	return response.Body.String()
}

func TestHooksPage(t *testing.T) {
	server := newHooksServer(t, provideGateRecord(t))

	body := getHooks(t, server, "/config/hooks")

	assert.Contains(t, body, `<h1>Config</h1>`)
	assert.Contains(t, body, `id="hooks-card-gate" open`)
	assert.NotContains(t, body, `id="hooks-card-heartbeat"`, "a component without live groups gets no card")
	assert.NotContains(t, body, `<code>kill_at</code>`, "settings live on the Smine tab")
	assert.Contains(t, body, `<strong>Tool policy gate</strong>`)
	assert.Contains(t, body, `hx-post="/config/hooks/gate/groups/PreToolUse/0/toggle?enabled=true"`)
	assert.Contains(t, body, `hx-post="/config/hooks/gate/groups/toggle?enabled=true"`, "the card header toggles every group")
	assert.Contains(t, body, `<a href="/config" class="active">Config</a>`)
	assert.Contains(t, body, `<a class="badge badge-ok" href="/config/hooks?state=all&q=">all</a>`)
	assert.NotContains(t, body, `id="hooks-card-other"`, "every group belongs to a component")
}

func TestHooksPageOffComponentNotSelectable(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	body := getHooks(t, server, "/config/hooks")
	unpark := httptest.NewRecorder()
	server.Handler().ServeHTTP(unpark, httptest.NewRequest(http.MethodPost, "/config/hooks/gate/groups/toggle?enabled=false", nil))

	assert.Contains(t, body, `title="not installed — install it on the Profile page and its groups come back">off</span>`)
	assert.NotContains(t, body, `hx-post="/config/hooks/gate/groups/toggle`, "an off component has no header switch")
	assert.Contains(t, body, "checked disabled", "its group switch is disabled")
	assert.Equal(t, http.StatusConflict, unpark.Code)
	assert.Contains(t, unpark.Body.String(), "install Permission gate first")
}

func TestPermissionsOffWithoutGate(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	smine := getHooks(t, server, "/config/smine")
	page := getHooks(t, server, "/config/permissions")
	add := httptest.NewRecorder()
	server.Handler().ServeHTTP(add, formPost("/api/permissions/add", url.Values{"list": {"allow"}, "rule": {"Bash(ls *)"}}))

	assert.NotContains(t, smine, `href="/config/permissions"`, "the tab is hidden")
	assert.Contains(t, page, "Permission gate", "a typed URL names the missing component")
	assert.Equal(t, http.StatusConflict, add.Code)
}

func TestHooksCardsCollapsedButFirst(t *testing.T) {
	settingsPath := writeHookSettings(t, map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: "say done", Type: "command"}}, Name: "Say done"}},
	})
	main := config.NewSettings()
	hooks := map[string][]config.HookGroup{
		"PreToolUse": {{Id: "gate-pretooluse-toolpolicy", Hooks: []config.Hook{{Command: "toolpolicy gate", Type: "command"}}, Name: "Tool policy gate", Version: "1.0.0"}},
		"Stop":       {{Hooks: []config.Hook{{Command: "say done", Type: "command"}}, Name: "Say done"}},
	}
	require.NoError(t, main.SetHooks(hooks))
	require.NoError(t, config.Save(settingsPath, main, nil))
	server := newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t), SettingsPath: settingsPath})

	body := getHooks(t, server, "/config/hooks")

	assert.Contains(t, body, `id="hooks-card-gate" open`)
	assert.Contains(t, body, `id="hooks-card-other">`, "every card but the first is collapsed")

	// a filter opens every card left
	filtered := getHooks(t, server, "/config/hooks?q=say")
	assert.NotContains(t, filtered, `id="hooks-card-gate"`)
	assert.Contains(t, filtered, `id="hooks-card-other" open`)
}

func TestConfigSmineTabFirstAndDefault(t *testing.T) {
	server := newHooksServer(t, provideGateRecord(t))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config", nil))
	require.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "/config/smine", response.Header().Get("Location"))

	body := getHooks(t, server, "/config/claude")
	smineTab := strings.Index(body, `<a href="/config/smine" >Smine</a>`)
	hooksTab := strings.Index(body, `<a href="/config/hooks" >Hooks</a>`)
	permissionsTab := strings.Index(body, `<a href="/config/permissions" >Permissions</a>`)
	claudeTab := strings.Index(body, `<a href="/config/claude" class="active">Claude</a>`)
	codexTab := strings.Index(body, `<a href="/config/codex" >Codex</a>`)
	assert.Positive(t, smineTab)
	assert.Less(t, smineTab, hooksTab)
	assert.Less(t, hooksTab, permissionsTab)
	assert.Less(t, permissionsTab, claudeTab)
	assert.Less(t, claudeTab, codexTab)
	assert.NotContains(t, body, `id="permissions"`, "the Claude tab lost its Permissions section")

	smineBody := getHooks(t, server, "/config/smine")
	assert.Contains(t, smineBody, `<a href="/config/smine" class="active">Smine</a>`)
	assert.Contains(t, smineBody, `<a href="/config" class="active">Config</a>`)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/hooks", nil))
	assert.Equal(t, http.StatusNotFound, response.Code, "the standalone page is gone")
}

func TestSminePage(t *testing.T) {
	// Heartbeat is darwin-only: the record refuses it elsewhere, so its tile exists only there.
	hasHeartbeat := components.Available(components.Heartbeat, runtime.GOOS)
	record := provideHooksRecord(t, map[string]string{"global_inline_cap": "1000"})
	record.Components = components.Close(append(record.Components, components.Context, components.Gate))
	if hasHeartbeat {
		record.Components = components.Close(append(record.Components, components.Heartbeat))
	}
	server := newHooksServer(t, record)

	// first-component-active
	body := getHooks(t, server, "/config/smine")
	assert.Contains(t, body, `href="/config/smine?component=config-server"`)
	assert.Equal(t, hasHeartbeat, strings.Contains(body, `href="/config/smine?component=heartbeat"`))

	// off-component-not-selectable
	assert.NotContains(t, body, `href="/config/smine?component=routines"`)
	assert.Contains(t, body, `<div class="tile card-disabled" title="off — install it on the Profile page to change its settings">
    <span class="tile-label">Routines</span>`)
	offBody := getHooks(t, server, "/config/smine?component=routines")
	assert.Contains(t, offBody, `tile-active" href="/config/smine?component=config-server"`, "a typed off component falls back to the first on tile")
	assert.NotContains(t, body, `href="/config/smine?component=ccusage"`, "a component without settings gets no tile")
	configServerTile := strings.Index(body, `tile-active" href="/config/smine?component=config-server"`)
	assert.Positive(t, configServerTile)
	assert.Less(t, strings.Index(body, `id="smine-body"`), configServerTile, "the tiles sit inside the swap region, so a tile click moves the highlight")
	assert.Equal(t, 1, strings.Count(body, "tile-active"))
	assert.Contains(t, body, `<h2 class="smine-panel-title">Config server`)
	assert.Contains(t, body, `name="port" value="6001"`)
	assert.NotContains(t, body, `>restart</span>`, "no restart badge; the Save report names the restart")
	assert.NotContains(t, body, `<code>kill_at</code>`, "one component's settings only")
	assert.NotContains(t, body, `hooks-card`, "no hook groups on the Smine tab")

	// selected-component
	body = getHooks(t, server, "/config/smine?component=context")
	assert.Contains(t, body, `tile-active" href="/config/smine?component=context"`)
	assert.Contains(t, body, `name="global_inline_cap" value="1000"`)
	assert.Contains(t, body, `1 overridden</span>`)

	// list-rows-as-items
	body = getHooks(t, server, "/config/smine?component=gate")
	assert.Contains(t, body, `name="groups.gate-pretooluse-toolpolicy.timeout"`, "the fragment's group timeout is a derived row")
	assert.NotContains(t, body, `groups.gate-pretooluse-toolpolicy.matcher`, "the matcher row stays off the page")
	assert.NotContains(t, body, `groups.gate-pretooluse-toolpolicy.async`, "the async row stays off the page")
	assert.NotContains(t, body, `<code>cause_cd_redirect</code>`, "the gate's text rows stay off the page")
	assert.Contains(t, body, `<input type="hidden" name="read_only_commands" value="[`, "the list posts with the panel form as JSON")
	assert.Contains(t, body, `onclick="smineListRemove(this)">×</button>`)
	assert.Contains(t, body, `<input type="text" name="read_only_commands.item" placeholder="new item" class="grow"`)
	assert.NotContains(t, body, `hx-post="/config/smine/gate/read_only_commands`, "no item endpoint: nothing persists before Save")
	assert.Contains(t, body, `<span class="badge badge-action smine-pending" hidden>pending</span>`, "the pending badge renders hidden until the form differs")
	assert.NotContains(t, body, `<textarea`, "no JSON textarea")

	// unknown-component-falls-back-to-first
	body = getHooks(t, server, "/config/smine?component=nope")
	assert.Contains(t, body, `tile-active" href="/config/smine?component=config-server"`)
}

func TestSmineListItems(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	defaults, err := install.Resolve(record, components.Gate, "read_only_commands")
	require.NoError(t, err)
	withTool := components.Value{Kind: components.KindStrings, List: append(slices.Clone(defaults.List), "my-tool")}

	// save-posted-list — the row's hidden JSON field is the list the browser edited
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/gate", url.Values{"read_only_commands": {withTool.String()}, "read_only_commands.item": {""}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `id="smine-panel"`)
	assert.Contains(t, response.Body.String(), `<code>my-tool</code>`)
	record, _, err = install.Load(server.machine)
	require.NoError(t, err)
	value, err := install.Resolve(record, components.Gate, "read_only_commands")
	require.NoError(t, err)
	assert.Contains(t, value.List, "my-tool")
	assert.Contains(t, value.List, "cat", "the default items stay")

	// save-default-list-clears — the default posted back stores no override
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/gate", url.Values{"read_only_commands": {defaults.String()}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	record, _, err = install.Load(server.machine)
	require.NoError(t, err)
	assert.False(t, install.IsOverridden(record, components.Gate, "read_only_commands"), "the default list stores no override")

	// save-empty-list — an emptied list is an override, not a reset
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/gate", url.Values{"read_only_commands": {"[]"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	record, _, err = install.Load(server.machine)
	require.NoError(t, err)
	value, err = install.Resolve(record, components.Gate, "read_only_commands")
	require.NoError(t, err)
	assert.Empty(t, value.List)
	assert.True(t, install.IsOverridden(record, components.Gate, "read_only_commands"))

	// save-folds-typed-item-in — a Save with text left in the add field appends it to the posted list
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/gate", url.Values{"read_only_commands": {defaults.String()}, "read_only_commands.item": {" typed-tool "}, "ungated_tools.item": {""}, "max_loop_items": {"7"}}))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `<code>typed-tool</code>`)
	assert.Contains(t, response.Body.String(), `data-close="#smine-report"`, "the report closes from its own dot")
	record, _, err = install.Load(server.machine)
	require.NoError(t, err)
	value, err = install.Resolve(record, components.Gate, "read_only_commands")
	require.NoError(t, err)
	assert.Contains(t, value.List, "typed-tool")
	assert.Contains(t, value.List, "cat", "the default items stay")
	assert.False(t, install.IsOverridden(record, components.Gate, "ungated_tools"), "an empty add field changes nothing")
	loops, err := install.Resolve(record, components.Gate, "max_loop_items")
	require.NoError(t, err)
	assert.Equal(t, 7, loops.Int, "the scalar rows of the same Save still store")

	// bad-list-refused
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/gate", url.Values{"read_only_commands": {"not json"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPanelSummary(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		results []install.Result
	}

	tests := make([]*testCase, 0)
	restart := install.Step{Action: install.ActionInstall, Component: components.Heartbeat, Detail: "~/Library/LaunchAgents/com.smine.heartbeat.plist"}
	configWrite := install.Step{Action: install.ActionUpdate, Component: components.Heartbeat, Detail: "~/.claude/heartbeat/config.json"}

	// daemon-restarted — the config changed and the LaunchAgent was bootstrapped again
	tests = append(tests, &testCase{
		_id:       "daemon-restarted",
		_expected: "heartbeat: changes applied: daemon restarted\nsession-hooks: changes applied\n",

		results: []install.Result{{Action: install.ActionInstall, Component: components.Heartbeat, Steps: []install.Step{configWrite, restart}}, {Action: install.ActionInstall, Component: components.SessionHooks}},
	})

	// nothing-to-apply — a Save whose values the deployed config already held
	tests = append(tests, &testCase{
		_id:       "nothing-to-apply",
		_expected: "heartbeat: saved, nothing to apply — every value was already in effect\n",

		results: []install.Result{{Action: install.ActionUnchanged, Component: components.Heartbeat, Steps: []install.Step{{Action: install.ActionKept, Component: components.Heartbeat, Detail: "x"}}}},
	})

	// restart-required — the config server's own restart is announced by the line that follows the summary
	tests = append(tests, &testCase{
		_id:       "restart-required",
		_expected: "config-server: changes applied\n",

		results: []install.Result{{Action: install.ActionRestartRequired, Component: components.ConfigServer}},
	})

	// failed — the error, whatever the action
	tests = append(tests, &testCase{
		_id:       "failed",
		_expected: "heartbeat: FAILED: launchctl denied\n",

		results: []install.Result{{Action: install.ActionUnchanged, Component: components.Heartbeat, Err: errors.New("launchctl denied")}},
	})

	// no-results
	tests = append(tests, &testCase{
		_id:       "no-results",
		_expected: "",

		results: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, panelSummary(test.results))
		})
	}
}

func TestToggleComponentHooks(t *testing.T) {
	server := newHooksServer(t, provideGateRecord(t))
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}, Name: "Say done"}))

	// all-off
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/gate/groups/toggle?enabled=true", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `all off</span>`)
	assert.Contains(t, response.Body.String(), `hx-post="/config/hooks/gate/groups/toggle?enabled=false"`)
	assert.Empty(t, loadHooks(t, server.settingsPath)["PreToolUse"])
	assert.Len(t, parkedHooks(t, server)["PreToolUse"], 1)
	assert.Len(t, parkedHooks(t, server)["Stop"], 1, "another component's parked group is untouched")

	// all-on
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/gate/groups/toggle?enabled=false", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.NotContains(t, response.Body.String(), `all off</span>`)
	assert.Contains(t, response.Body.String(), `hx-post="/config/hooks/gate/groups/toggle?enabled=true"`)
	assert.Len(t, loadHooks(t, server.settingsPath)["PreToolUse"], 1)
	assert.Empty(t, parkedHooks(t, server)["PreToolUse"])
	assert.Len(t, parkedHooks(t, server)["Stop"], 1)

	// unknown-component
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/nope/groups/toggle?enabled=true", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)

	t.Run("unpark-settings-write-failure-keeps-parked", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/gate/groups/toggle?enabled=true", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		lockSettingsDir(t, server.settingsPath)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/gate/groups/toggle?enabled=false", nil))
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.Empty(t, loadHooks(t, server.settingsPath)["PreToolUse"])
		assert.Len(t, parkedHooks(t, server)["PreToolUse"], 1, "the group stays parked when settings.json is not written")
	})
}

func TestPermissionsPage(t *testing.T) {
	server := newHooksServer(t, provideGateRecord(t))
	main, err := config.Load(server.settingsPath)
	require.NoError(t, err)
	require.NoError(t, main.SetPermissions(config.Permissions{Allow: []string{"Bash(git log*)", "Edit(docs/*)"}, Deny: []string{"Bash(rm *)"}}))
	require.NoError(t, config.Save(server.settingsPath, main, nil))

	body := getHooks(t, server, "/config/permissions")

	assert.Contains(t, body, `<a href="/config/permissions" class="active">Permissions</a>`)
	assert.Contains(t, body, `<a href="/config" class="active">Config</a>`)
	assert.Contains(t, body, `hx-post="/api/config/claude/sync/apply"`, "the sync widget persists the live lists")
	assert.Contains(t, body, `hx-post="/api/config/claude/sync/force/preview"`)
	assert.Contains(t, body, `id="permissions-body" hx-get="/config/permissions" hx-trigger="config-op from:body"`)
	assert.NotContains(t, body, "Codex has no rule lists")
	assert.Equal(t, 1, strings.Count(body, `id="ovr-permissions"`), "the overridden badge sits in the sync card alone, not inline above the lists")
	assert.Less(t, strings.Index(body, `id="ovr-permissions" class="push-right"`), strings.Index(body, `id="op-result"`))
	assert.Contains(t, body, `<span class="summary-tags"><span class="badge badge-dim">1</span>`, "the head's badges sit in the right-aligned group")
	allowAt := strings.Index(body, `>Allow <span`)
	askAt := strings.Index(body, `>Ask <span`)
	denyAt := strings.Index(body, `>Deny <span`)
	assert.Positive(t, allowAt)
	assert.Less(t, allowAt, askAt)
	assert.Less(t, askAt, denyAt)
	assert.Contains(t, body, `id="perms-allow-Bash" open`, "the first head is open")
	assert.Contains(t, body, `id="perms-allow-Edit">`)
	assert.Contains(t, body, `id="perms-deny-Bash">`)
	assert.Contains(t, body, `<code>Bash(git log*)</code>`)
}

// Live: an undeclared-id Stop group and an idless Notification group; neither belongs to the gate fragment.
func TestOtherHooksCard(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedNotification int
		_expectedParked       int
		_expectedStop         int

		request *http.Request
		server  *Server
	}

	newOtherServer := func() *Server {
		settingsPath := writeHookSettings(t, map[string][]config.HookGroup{
			"Notification": {{Hooks: []config.Hook{{Command: "afplay funk.aiff", Name: "Funk sound", Type: "command"}}}},
			"Stop":         {{Id: "mine-stop-say", Hooks: []config.Hook{{Command: "say done", Type: "command"}}, Name: "Say done", Version: "1.0.0"}},
		})
		return newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t), SettingsPath: settingsPath})
	}
	tests := make([]*testCase, 0)

	// toggle-idless-off
	tests = append(tests, &testCase{
		_id:                   "toggle-idless-off",
		_expectedNotification: 0,
		_expectedParked:       1,
		_expectedStop:         1,

		request: httptest.NewRequest(http.MethodPost, "/config/hooks/other/groups/Notification/0/toggle?enabled=true", nil),
		server:  newOtherServer(),
	})

	// toggle-undeclared-off
	tests = append(tests, &testCase{
		_id:                   "toggle-undeclared-off",
		_expectedNotification: 1,
		_expectedParked:       1,
		_expectedStop:         0,

		request: httptest.NewRequest(http.MethodPost, "/config/hooks/other/groups/Stop/0/toggle?enabled=true", nil),
		server:  newOtherServer(),
	})

	// remove-idless
	tests = append(tests, &testCase{
		_id:                   "remove-idless",
		_expectedNotification: 0,
		_expectedParked:       0,
		_expectedStop:         1,

		request: httptest.NewRequest(http.MethodDelete, "/config/hooks/other/groups/Notification/0?enabled=true", nil),
		server:  newOtherServer(),
	})

	// remove-undeclared
	tests = append(tests, &testCase{
		_id:                   "remove-undeclared",
		_expectedNotification: 1,
		_expectedParked:       0,
		_expectedStop:         0,

		request: httptest.NewRequest(http.MethodDelete, "/config/hooks/other/groups/Stop/0?enabled=true", nil),
		server:  newOtherServer(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			page := getHooks(t, test.server, "/config/hooks")
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, test.request)
			hooks := loadHooks(t, test.server.settingsPath)

			assert.Contains(t, page, `id="hooks-card-other"`)
			assert.Contains(t, page, `<strong>Funk sound</strong>`)
			assert.Contains(t, page, `<strong>Say done</strong>`)
			assert.Contains(t, page, `hx-post="/config/hooks/other/groups/Notification/0/toggle?enabled=true"`)
			assert.Less(t, strings.Index(page, `id="hooks-card-gate"`), strings.Index(page, `id="hooks-card-other"`), "the Other hooks card renders last")
			assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), `id="hooks-card-other"`)
			assert.Len(t, hooks["Notification"], test._expectedNotification)
			assert.Len(t, hooks["Stop"], test._expectedStop)
			assert.Len(t, slices.Concat(parkedHooks(t, test.server)["Notification"], parkedHooks(t, test.server)["Stop"]), test._expectedParked)
		})
	}
}

func TestOtherHooksCardRefusesSave(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/other", url.Values{"x": {"1"}}))

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestHooksPageFilter(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	// query-matches-name
	body := getHooks(t, server, "/config/hooks?q=policy")
	assert.Contains(t, body, `<strong>Tool policy gate</strong>`)

	// query-matches-event
	body = getHooks(t, server, "/config/hooks?q=pretooluse")
	assert.Contains(t, body, `<strong>Tool policy gate</strong>`)

	// query-misses
	body = getHooks(t, server, "/config/hooks?q=nothing-here")
	assert.NotContains(t, body, `<strong>Tool policy gate</strong>`)
	assert.Contains(t, body, "No hook group matches the filter")
}

func TestHooksPageStateFilter(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}, Name: "Say done"}))

	// disabled
	body := getHooks(t, server, "/config/hooks?state=disabled")
	assert.Contains(t, body, `<strong>Say done</strong>`)
	assert.NotContains(t, body, `<strong>Tool policy gate</strong>`)
	assert.Contains(t, body, `<a class="badge badge-ok" href="/config/hooks?state=disabled&q=">disabled</a>`)

	// local
	body = getHooks(t, server, "/config/hooks?state=local")
	assert.Contains(t, body, `<strong>Say done</strong>`)
	assert.NotContains(t, body, `<strong>Tool policy gate</strong>`)

	// changed
	body = getHooks(t, server, "/config/hooks?state=changed")
	assert.NotContains(t, body, `<strong>Say done</strong>`)
	assert.NotContains(t, body, `<strong>Tool policy gate</strong>`)

	// unknown-state-is-all
	body = getHooks(t, server, "/config/hooks?state=nope")
	assert.Contains(t, body, `<strong>Say done</strong>`)
	assert.Contains(t, body, `<strong>Tool policy gate</strong>`)
}

func TestHooksSave(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	form := url.Values{"global_inline_cap": {"12000"}, "global_read_budget": {"55000"}}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/context", form))

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()
	assert.Contains(t, body, `id="smine-panel"`)
	assert.Contains(t, body, `name="global_inline_cap" value="12000"`)
	assert.Contains(t, body, `title="the record overrides the default">overridden</span>`)
	assert.Contains(t, body, "is not selected", "the apply report sits in the card")
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	assert.True(t, install.IsOverridden(record, components.Context, "global_inline_cap"))
	assert.False(t, install.IsOverridden(record, components.Context, "global_read_budget"), "a posted default stores no override")
}

func TestHooksSaveReset(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, map[string]string{"global_inline_cap": "1000"}))

	form := url.Values{"global_inline_cap": {""}}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/context", form))

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `name="global_inline_cap" value="9000"`)
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	assert.False(t, install.IsOverridden(record, components.Context, "global_inline_cap"))
}

func TestHooksSaveBadValue(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	form := url.Values{"global_inline_cap": {"x"}}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/context", form))

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "Bytes of global context delivered inline")
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	assert.False(t, install.IsOverridden(record, components.Context, "global_inline_cap"))
}

func TestHooksSaveGroupDefaultClears(t *testing.T) {
	server := newHooksServer(t, provideHooksRecord(t, nil))

	form := url.Values{"groups.gate-pretooluse-toolpolicy.matcher": {"Bash"}}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/config/smine/gate", form))

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	assert.False(t, install.IsOverridden(record, components.Gate, "groups.gate-pretooluse-toolpolicy.matcher"))
}

func TestHooksCardOmitsBinaryHook(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "hookbin")
	binary := append([]byte{0xcf, 0xfa, 0xed, 0xfe}, []byte(strings.Repeat("BINARYPAYLOAD\x00", 4096))...)
	require.NoError(t, os.WriteFile(binaryPath, binary, 0o755))
	settingsPath := filepath.Join(dir, "settings.json")
	hooks := map[string][]config.HookGroup{
		"PreToolUse": {{Id: "gate-pretooluse-toolpolicy", Hooks: []config.Hook{{Command: binaryPath + " gate", Type: "command"}}}},
	}
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(hooks))
	require.NoError(t, config.Save(settingsPath, main, nil))
	server := newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t), SettingsPath: settingsPath})

	body := getHooks(t, server, "/config/hooks")

	assert.Contains(t, body, binaryPath+" gate")
	assert.NotContains(t, body, "BINARYPAYLOAD")
	assert.Equal(t, 1, strings.Count(body, `<pre class="file-view">`), "only the command block, no script view")
}

func TestHookGroupInfoLabels(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCommand string
		_expectedName    string

		group config.HookGroup
	}

	tests := make([]*testCase, 0)

	// named-group-shows-smine-name
	tests = append(tests, &testCase{
		_id:              "named-group-shows-smine-name",
		_expectedCommand: "afplay /tmp/funk.aiff",
		_expectedName:    "Play sound",

		group: config.HookGroup{Id: "session-helpers-stop-play-sound", Hooks: []config.Hook{{Command: "afplay /tmp/funk.aiff", Name: "Funk sound", Type: "command"}}, Name: "Play sound", Version: "1.0.1"},
	})

	// idless-named-hook-shows-hook-name
	tests = append(tests, &testCase{
		_id:              "idless-named-hook-shows-hook-name",
		_expectedCommand: "afplay /tmp/funk.aiff",
		_expectedName:    "Funk sound",

		group: config.HookGroup{Hooks: []config.Hook{{Command: "afplay /tmp/funk.aiff", Name: "Funk sound", Type: "command"}}},
	})

	// unnamed-hook-falls-back-to-cut-command
	tests = append(tests, &testCase{
		_id:              "unnamed-hook-falls-back-to-cut-command",
		_expectedCommand: "echo " + strings.Repeat("x", 28),
		_expectedName:    "echo " + strings.Repeat("x", 26) + "…",

		group: config.HookGroup{Hooks: []config.Hook{{Command: "echo " + strings.Repeat("x", 28), Type: "command"}}},
	})

	// http-hook-shows-url
	tests = append(tests, &testCase{
		_id:              "http-hook-shows-url",
		_expectedCommand: "http://localhost:6101/hook",
		_expectedName:    "http://localhost:6101/hook",

		group: config.HookGroup{Hooks: []config.Hook{{Type: "http", Url: "http://localhost:6101/hook"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			info := newHookGroupInfo(true, "Stop", 0, &test.group, nil)

			assert.Equal(t, test._expectedName, info.Name)
			assert.Equal(t, test._expectedCommand, info.Command)
		})
	}
}

func TestToggleRoundTripKeepsNameIdVersion(t *testing.T) {
	settingsPath := writeHookSettings(t, map[string][]config.HookGroup{
		"Notification": {{Hooks: []config.Hook{{Command: "afplay /tmp/funk.aiff", Name: "Funk sound", Type: "command"}}}},
		"Stop":         {{Id: "session-helpers-stop-play-sound", Hooks: []config.Hook{{Command: "say done", Timeout: 5, Type: "command"}}, Name: "Play sound", Version: "1.0.0"}},
	})
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	for _, event := range []string{"Notification", "Stop"} {
		for _, enabled := range []string{"true", "false"} {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/config/hooks/session-helpers/groups/"+event+"/0/toggle?enabled="+enabled, nil)
			server.Handler().ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		}
	}

	// Off then on: each group is back in settings.json with its name, id and version.
	hooks := loadHooks(t, settingsPath)
	require.Len(t, hooks["Notification"], 1)
	assert.Equal(t, "Funk sound", hooks["Notification"][0].Hooks[0].Name)
	require.Len(t, hooks["Stop"], 1)
	assert.Equal(t, "session-helpers-stop-play-sound", hooks["Stop"][0].Id)
	assert.Equal(t, "Play sound", hooks["Stop"][0].Name)
	assert.Equal(t, config.HookVersion("1.0.0"), hooks["Stop"][0].Version)
	assert.Equal(t, 5, hooks["Stop"][0].Hooks[0].Timeout)
}

func TestHookOwners(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedOwner string

		disabledHooks map[string][]config.HookGroup
		mainHooks     map[string][]config.HookGroup
		server        *Server
	}

	matcher := "Bash"
	gateGroup := config.HookGroup{Id: "gate-pretooluse-toolpolicy", Hooks: []config.Hook{{Command: "toolpolicy gate", Type: "command"}}, Matcher: &matcher, Version: "1.0.0"}
	editedGroup := config.HookGroup{Id: "gate-pretooluse-toolpolicy", Hooks: []config.Hook{{Command: "toolpolicy gate --edited", Type: "command"}}, Matcher: &matcher, Version: "1.0.0"}
	idlessGroup := config.HookGroup{Hooks: []config.Hook{{Command: "toolpolicy gate", Type: "command"}}, Matcher: &matcher}
	tests := make([]*testCase, 0)

	// fragment-group-owned
	tests = append(tests, &testCase{
		_id:            "fragment-group-owned",
		_expectedOwner: "Permission gate",

		disabledHooks: nil,
		mainHooks:     map[string][]config.HookGroup{"PreToolUse": {gateGroup}},
		server:        newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t)}),
	})

	// edited-group-owned-by-id
	tests = append(tests, &testCase{
		_id:            "edited-group-owned-by-id",
		_expectedOwner: "Permission gate",

		disabledHooks: nil,
		mainHooks:     map[string][]config.HookGroup{"PreToolUse": {editedGroup}},
		server:        newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t)}),
	})

	// idless-group-no-owner
	tests = append(tests, &testCase{
		_id:            "idless-group-no-owner",
		_expectedOwner: "",

		disabledHooks: nil,
		mainHooks:     map[string][]config.HookGroup{"PreToolUse": {idlessGroup}},
		server:        newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t)}),
	})

	// parked-group-attributed
	tests = append(tests, &testCase{
		_id:            "parked-group-attributed",
		_expectedOwner: "Permission gate",

		disabledHooks: map[string][]config.HookGroup{"PreToolUse": {gateGroup}},
		mainHooks:     nil,
		server:        newTestServer(t, &Options{ClaudeFragmentPath: writeGateFragment(t)}),
	})

	// missing-fragment-dir-no-owners
	tests = append(tests, &testCase{
		_id:            "missing-fragment-dir-no-owners",
		_expectedOwner: "",

		disabledHooks: nil,
		mainHooks:     map[string][]config.HookGroup{"PreToolUse": {gateGroup}},
		server:        newTestServer(t, &Options{ClaudeFragmentPath: filepath.Join(t.TempDir(), "missing")}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rows := hookRows(test.mainHooks, test.disabledHooks, newHookFragment(nil, false), test.server.hookOwners())
			require.Len(t, rows, 1)
			assert.Equal(t, test._expectedOwner, rows[0].Owner)
		})
	}
}

// Fragment: gate group (id) and an idless Stop group; live: the gate group edited, a local Stop group, the parked idless group.
func TestHookRowsSources(t *testing.T) {
	matcher := "Bash"
	gateGroup := config.HookGroup{Id: "gate-pretooluse-toolpolicy", Hooks: []config.Hook{{Command: "toolpolicy gate", Type: "command"}}, Matcher: &matcher, Version: "1.0.0"}
	editedGroup := config.HookGroup{Id: "gate-pretooluse-toolpolicy", Hooks: []config.Hook{{Command: "toolpolicy gate --edited", Type: "command"}}, Matcher: &matcher, Version: "1.0.0"}
	idlessGroup := config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}
	localGroup := config.HookGroup{Hooks: []config.Hook{{Command: "afplay funk.aiff", Type: "command"}}}
	repoOnlyGroup := config.HookGroup{Id: "session-helpers-stop-title", Hooks: []config.Hook{{Command: "sessiontitle", Type: "command"}}}
	fragment := newHookFragment(map[string][]config.HookGroup{
		"PreToolUse": {gateGroup},
		"Stop":       {idlessGroup, repoOnlyGroup},
	}, true)

	rows := hookRows(
		map[string][]config.HookGroup{"PreToolUse": {editedGroup}, "Stop": {localGroup}},
		map[string][]config.HookGroup{"Stop": {idlessGroup}},
		fragment,
		nil,
	)

	bySource := map[string]hookGroupInfo{}
	for _, row := range rows {
		bySource[row.Command] = row
	}
	require.Len(t, rows, 4)
	assert.Equal(t, sourceChanged, bySource["toolpolicy gate --edited"].Source)
	assert.NotEmpty(t, bySource["toolpolicy gate --edited"].Diff)
	assert.Equal(t, sourceLocal, bySource["afplay funk.aiff"].Source)
	assert.Equal(t, sourceShared, bySource["say done"].Source)
	assert.False(t, bySource["say done"].Enabled)
	assert.Equal(t, sourceRepoOnly, bySource["sessiontitle"].Source)
	assert.False(t, bySource["sessiontitle"].Enabled)
}

func TestToggleDisabledHookWithOverlappingIndex(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetHooks(map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: "enabled", Type: "command"}}}},
	}))
	disabled := config.NewSettings()
	require.NoError(t, disabled.SetHooks(map[string][]config.HookGroup{
		"Stop": {{Hooks: []config.Hook{{Command: "disabled", Type: "command"}}}},
	}))
	require.NoError(t, config.Save(settingsPath, main, nil))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled, nil))

	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	request := httptest.NewRequest(http.MethodPost, "/config/hooks/session-helpers/groups/Stop/0/toggle?enabled=false", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	mainHooks := loadHooks(t, settingsPath)
	assert.Equal(t, "enabled", mainHooks["Stop"][0].Hooks[0].Command)
	assert.Equal(t, "disabled", mainHooks["Stop"][1].Hooks[0].Command)
	assert.Empty(t, loadHooks(t, config.DisabledPath(settingsPath))["Stop"])
}

// Rows sort by (event, name) regardless of array order (D7).
func TestHookRowsSorted(t *testing.T) {
	rows := hookRows(map[string][]config.HookGroup{
		"Stop": {
			{Hooks: []config.Hook{{Command: "cmd", Name: "b-hook", Type: "command"}}},
			{Hooks: []config.Hook{{Command: "cmd", Name: "a-hook", Type: "command"}}},
		},
		"Notification": {
			{Hooks: []config.Hook{{Command: "cmd", Name: "z-hook", Type: "command"}}},
		},
	}, nil, newHookFragment(nil, false), nil)

	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	assert.Equal(t, []string{"z-hook", "a-hook", "b-hook"}, names)
}

func TestHookMutationAnswersWithFragment(t *testing.T) {
	settingsPath := writeHookSettings(t, nil)
	server := newTestServer(t, &Options{SettingsPath: settingsPath})
	require.NoError(t, server.disabledHooks.Add("Stop", config.HookGroup{Hooks: []config.Hook{{Command: "say done", Type: "command"}}}))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/session-helpers/groups/Stop/0/toggle?enabled=false", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Empty(t, response.Header().Get("HX-Trigger"))

	// GETs must never trigger the body re-pull — that would loop the page.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config/hooks", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}

func TestHookRoutesRefuseUnknownComponent(t *testing.T) {
	server := newTestServer(t, &Options{SettingsPath: writeHookSettings(t, stopHook("say done"))})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config/hooks/nope/groups/Stop/0/toggle?enabled=true", nil))

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Len(t, loadHooks(t, server.settingsPath)["Stop"], 1)
}
