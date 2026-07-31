package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPlist mirrors a real routine plist: ProgramArguments points at the
// routine's own run.sh and the log paths carry the routine name, which is what
// Duplicate rewrites.
func testPlist(routineDir string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.test.routine-handler-test</string>
	<key>ProgramArguments</key>
	<array><string>/bin/bash</string><string>` + routineDir + `/run.sh</string></array>
	<key>StartCalendarInterval</key>
	<dict><key>Hour</key><integer>6</integer><key>Minute</key><integer>0</integer></dict>
	<key>StandardOutPath</key>
	<string>/tmp/claude-routine-demo.out.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/claude-routine-demo.err.log</string>
</dict>
</plist>
`
}

// testPlistWithEnv mirrors testPlist with a single EnvironmentVariables entry,
// used to seed a stored value the configure view reads back.
func testPlistWithEnv(routineDir, key, value string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.test.routine-handler-test</string>
	<key>ProgramArguments</key>
	<array><string>/bin/bash</string><string>` + routineDir + `/run.sh</string></array>
	<key>EnvironmentVariables</key>
	<dict><key>` + key + `</key><string>` + value + `</string></dict>
	<key>StartCalendarInterval</key>
	<dict><key>Hour</key><integer>6</integer><key>Minute</key><integer>0</integer></dict>
	<key>StandardOutPath</key>
	<string>/tmp/claude-routine-demo.out.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/claude-routine-demo.err.log</string>
</dict>
</plist>
`
}

func routinesTestEnv(t *testing.T) (*Server, string) {
	t.Helper()
	return routinesTestEnvWithRepos(t, "")
}

// routinesTestEnvWithRepos builds the routines fixture plus an optional repo
// registry (empty registryJSON → no repos configured).
func routinesTestEnvWithRepos(t *testing.T, registryJSON string) (*Server, string) {
	t.Helper()
	routinesDir := t.TempDir()
	routineDir := filepath.Join(routinesDir, "demo")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))

	opts := &Options{RoutinesDir: routinesDir}
	if registryJSON != "" {
		reposPath := filepath.Join(t.TempDir(), "repos.json")
		require.NoError(t, os.WriteFile(reposPath, []byte(registryJSON), 0o644))
		opts.ReposPath = reposPath
	}

	server := newTestServer(t, opts)
	return server, routineDir
}

func TestIsSettingKey(t *testing.T) {
	assert.True(t, isSettingKey("ROUTINE_MODEL", "demo"))
	assert.True(t, isSettingKey("SMINE_AUTO_APPLY", "smine-nightly"))
	assert.False(t, isSettingKey("SMINE_AUTO_APPLY", "demo"))
}

func TestValidateSmineSetting(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool
		key         string
		value       string
	}

	tests := make([]*testCase, 0)

	// valid-mode-decide
	tests = append(tests, &testCase{
		_id:         "valid-mode-decide",
		_shouldPass: true,
		key:         "SMINE_AUTO_APPLY",
		value:       "decide",
	})

	// invalid-mode
	tests = append(tests, &testCase{
		_id:         "invalid-mode",
		_shouldPass: false,
		key:         "SMINE_AUTO_APPLY",
		value:       "sometimes",
	})

	// valid-dimensions-csv-with-spaces
	tests = append(tests, &testCase{
		_id:         "valid-dimensions-csv-with-spaces",
		_shouldPass: true,
		key:         "SMINE_AUTO_APPLY_DIMENSIONS",
		value:       "style, workflows",
	})

	// invalid-dimension
	tests = append(tests, &testCase{
		_id:         "invalid-dimension",
		_shouldPass: false,
		key:         "SMINE_AUTO_APPLY_DIMENSIONS",
		value:       "style,rules",
	})

	// cap-positive
	tests = append(tests, &testCase{
		_id:         "cap-positive",
		_shouldPass: true,
		key:         "SMINE_APPLY_CAP",
		value:       "5",
	})

	// cap-zero
	tests = append(tests, &testCase{
		_id:         "cap-zero",
		_shouldPass: false,
		key:         "SMINE_MAX_PROPOSALS_MINED",
		value:       "0",
	})

	// cap-not-a-number
	tests = append(tests, &testCase{
		_id:         "cap-not-a-number",
		_shouldPass: false,
		key:         "SMINE_MAX_PROPOSALS_PER_DIMENSION",
		value:       "many",
	})

	// unknown-key-passes
	tests = append(tests, &testCase{
		_id:         "unknown-key-passes",
		_shouldPass: true,
		key:         "ROUTINE_CADENCE_DAYS",
		value:       "whatever",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := validateSmineSetting(test.key, test.value)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestSmineNightlyParams(t *testing.T) {
	routinesDir := t.TempDir()
	routineDir := filepath.Join(routinesDir, "smine-nightly")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))
	server := newTestServer(t, &Options{RoutinesDir: routinesDir})
	plistPath := filepath.Join(routineDir, "com.test.routine-handler-test.plist")

	// Configure lists the smine settings with the mode select
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/smine-nightly/configure", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "SMINE_AUTO_APPLY")
	assert.Contains(t, body, "Max proposals mined per dimension")
	assert.Contains(t, body, `<option value="decide" `)

	// Invalid mode rejected before any write
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/smine-nightly/params", url.Values{"key": {"SMINE_AUTO_APPLY"}, "value": {"sometimes"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code)

	// Valid save writes the plist env
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/smine-nightly/params", url.Values{"key": {"SMINE_AUTO_APPLY"}, "value": {"decide"}}))
	require.Equal(t, http.StatusOK, response.Code)
	plistData, err := os.ReadFile(plistPath)
	require.NoError(t, err)
	assert.Contains(t, string(plistData), "SMINE_AUTO_APPLY")

	// Empty save clears the key (absent = wrapper default)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/smine-nightly/params", url.Values{"key": {"SMINE_AUTO_APPLY"}, "value": {""}}))
	require.Equal(t, http.StatusOK, response.Code)
	plistData, err = os.ReadFile(plistPath)
	require.NoError(t, err)
	assert.NotContains(t, string(plistData), "EnvironmentVariables")
}

func TestRoutinesIndexDegradedRow(t *testing.T) {
	server, routineDir := routinesTestEnv(t)
	require.NoError(t, os.Remove(filepath.Join(routineDir, "run.sh")))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "missing run.sh")
	assert.NotContains(t, body, `<input type="radio"`)
}

func TestRoutineRescheduleRendersOkAndOOBRow(t *testing.T) {
	server, _ := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/reschedule", url.Values{"hour": {"7"}, "minute": {"30"}}))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, ">Ok</span>")
	assert.Contains(t, body, `hx-swap-oob="true"`)
}

func TestRoutineRowNewerResultStopsPolling(t *testing.T) {
	server, routineDir := routinesTestEnv(t)
	line := `{"timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `", "exit_status": 0, "session_id": "s", "num_turns": 1, "total_cost_usd": 0.1}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "results.jsonl"), []byte(line), 0o644))

	since := time.Now().Add(-time.Hour).Unix()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/routines/demo/row?since="+itoa(since), nil)
	server.Handler().ServeHTTP(response, request)
	assert.Equal(t, statusStopPolling, response.Code)
}

func TestRoutineRowWithoutNewerResultKeepsPolling(t *testing.T) {
	server, _ := routinesTestEnv(t)

	since := time.Now().Add(time.Hour).Unix()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/routines/demo/row?since="+itoa(since), nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "since="+itoa(since))
}

func TestRoutineDetailSessionLinks(t *testing.T) {
	routinesDir := t.TempDir()
	routineDir := filepath.Join(routinesDir, "demo")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))

	line := `{"timestamp": "2026-07-01T00:00:00Z", "exit_status": 0, "session_id": "abc123", "num_turns": 1, "total_cost_usd": 0.1}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "results.jsonl"), []byte(line), 0o644))

	t.Run("session-id-links-to-peek", func(t *testing.T) {
		server := newTestServer(t, &Options{
			PeekDashboardURL: "http://127.0.0.1:4243/",
			RoutinesDir:      routinesDir,
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/demo", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<a href="http://127.0.0.1:4243/sessions/abc123" title="open in peek"><code>abc123</code></a>`)
	})

	t.Run("no-dashboard-url-renders-plain-id", func(t *testing.T) {
		server := newTestServer(t, &Options{RoutinesDir: routinesDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/demo", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "<code>abc123</code>")
		assert.NotContains(t, body, "open in peek")
	})
}

func TestRescheduleFormValidation(t *testing.T) {
	server, _ := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/reschedule", url.Values{"hour": {"25"}, "minute": {"0"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRescheduleDayOfMonth(t *testing.T) {
	server, routineDir := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/reschedule", url.Values{"hour": {"4"}, "minute": {"0"}, "day": {"15"}}))
	require.Equal(t, http.StatusOK, response.Code)

	data, err := os.ReadFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "<key>Day</key>")
}

func TestRescheduleDayValidation(t *testing.T) {
	server, _ := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/reschedule", url.Values{"hour": {"4"}, "minute": {"0"}, "day": {"1"}, "weekday": {"1"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/reschedule", url.Values{"hour": {"4"}, "minute": {"0"}, "day": {"32"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoutineParams(t *testing.T) {
	server, routineDir := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{
		"key":   {"ROUTINE_TARGET_REPO", "ROUTINE_CADENCE_DAYS"},
		"value": {"/repo", "2"},
	}))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `hx-swap-oob="true"`)

	// The out-of-band row must render the just-written plist, not the state
	// findRoutine read before the write.
	assert.Contains(t, body, "ROUTINE_TARGET_REPO=/repo")
	assert.Contains(t, body, "ROUTINE_CADENCE_DAYS=2")

	data, err := os.ReadFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "ROUTINE_TARGET_REPO")
	assert.Contains(t, string(data), "<string>/repo</string>")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{"key": {"A", "B"}, "value": {"only-one"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code, "mismatched pairs rejected")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{"key": {""}, "value": {"x"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code, "empty key rejected")

	// A non-empty standard value is written; an empty standard value drops the
	// key so the wrapper default applies (D3). A declared key keeps its empty
	// string (the cadence declaration mechanism).
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{
		"key":   {"ROUTINE_MODEL", "ROUTINE_MAX_BUDGET_USD", "ROUTINE_CADENCE_DAYS"},
		"value": {"claude-sonnet-5", "", ""},
	}))
	require.Equal(t, http.StatusOK, response.Code)

	data, err = os.ReadFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"))
	require.NoError(t, err)
	plist := string(data)
	assert.Contains(t, plist, "ROUTINE_MODEL")
	assert.Contains(t, plist, "<string>claude-sonnet-5</string>")
	assert.NotContains(t, plist, "ROUTINE_MAX_BUDGET_USD", "empty standard key dropped")
	assert.Contains(t, plist, "ROUTINE_CADENCE_DAYS", "declared key keeps empty-string declaration")
}

func TestRoutinesIndexHidesStandardEnv(t *testing.T) {
	server, routineDir := routinesTestEnv(t)

	// Seed both a standard key and a declared key.
	server.Handler().ServeHTTP(httptest.NewRecorder(), formPost("/routines/demo/params", url.Values{
		"key":   {"ROUTINE_MODEL", "ROUTINE_CADENCE_DAYS"},
		"value": {"claude-sonnet-5", "2"},
	}))
	require.FileExists(t, filepath.Join(routineDir, "com.test.routine-handler-test.plist"))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.NotContains(t, body, "ROUTINE_MODEL=", "standard key hidden from the Env column (D4)")
	assert.Contains(t, body, "ROUTINE_CADENCE_DAYS=2", "declared key still shown")

	// Column order: State left of Name, Env last (DR3).
	assert.Contains(t, body,
		"<th></th><th>State</th><th>Name</th><th>Schedule</th><th>Last run</th><th>Next run</th><th>Env</th>")
}

func TestRoutineConfigure(t *testing.T) {
	server, routineDir := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{
		"key":   {"ROUTINE_CADENCE_DAYS"},
		"value": {"2"},
	}))
	require.Equal(t, http.StatusOK, response.Code)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/demo/configure", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "Configure demo")
	assert.Contains(t, body, `hx-post="/routines/demo/params"`)

	// The three standard settings always render, with the wrapper defaults as
	// dimmed placeholder examples (DR1, DR4).
	assert.Contains(t, body, `name="key" value="ROUTINE_MODEL"`)
	assert.Contains(t, body, `placeholder="claude-opus-4-8[1m]"`)
	assert.Contains(t, body, `name="key" value="ROUTINE_MAX_BUDGET_USD"`)
	assert.Contains(t, body, `name="key" value="ROUTINE_PERMISSION_MODE"`)
	assert.Contains(t, body, `<option value="">acceptEdits (default)</option>`)

	// A declared routine-specific key still renders as a text input with its
	// known example placeholder (D7).
	assert.Contains(t, body, `name="key" value="ROUTINE_CADENCE_DAYS"`)
	assert.Contains(t, body, `placeholder="1"`)

	// The "no settable params" branch is gone — every routine has settings now.
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/demo/configure", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.NotContains(t, body, "no settable params declared in the plist")
	assert.Contains(t, body, `name="key" value="ROUTINE_MODEL"`)
}

func TestRoutineConfigureRepoSelect(t *testing.T) {
	server, routineDir := routinesTestEnvWithRepos(t,
		`{"repos": [{"name": "alpha", "path": "/repos/alpha"}, {"name": "beta", "path": "/repos/beta"}]}`)

	// Stored value matching a registry path → that option is selected, labeled
	// by name (D1).
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"),
		[]byte(testPlistWithEnv(routineDir, "ROUTINE_TARGET_REPO", "/repos/beta")), 0o644))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/demo/configure", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `<option value="/repos/alpha" >alpha</option>`)
	assert.Contains(t, body, `<option value="/repos/beta" selected>beta</option>`)

	// Stored value not in the registry → an extra selected option preserves it
	// (D1, never silently dropped).
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"),
		[]byte(testPlistWithEnv(routineDir, "ROUTINE_TARGET_REPO", "/gone")), 0o644))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/demo/configure", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.Contains(t, body, `<option value="/repos/alpha" >alpha</option>`)
	assert.Contains(t, body, `<option value="/gone" selected>/gone</option>`)
}

func TestRoutineDuplicate(t *testing.T) {
	server, _ := routinesTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/duplicate", url.Values{"newname": {"demo-two"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))

	index := httptest.NewRecorder()
	server.Handler().ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/routines", nil))
	assert.Contains(t, index.Body.String(), "demo-two")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/duplicate", url.Values{"newname": {"demo-two"}}))
	assert.Equal(t, http.StatusBadRequest, response.Code, "existing name rejected")
}

func TestRoutineUnknownNameIs404(t *testing.T) {
	server, _ := routinesTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/nope", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
