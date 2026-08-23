package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// smineNightlyFixture builds a routines dir holding a smine-nightly routine —
// the routine whose configure widget hosts the rules editor.
func smineNightlyFixture(t *testing.T) string {
	t.Helper()
	routinesDir := t.TempDir()
	routineDir := filepath.Join(routinesDir, "smine-nightly")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))
	return routinesDir
}

func TestAutoApplyRules(t *testing.T) {
	t.Run("save-then-get-roundtrip", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "rules.md")
		require.NoError(t, os.WriteFile(rulesPath, []byte("# Rules\n"), 0o644))
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath, RoutinesDir: smineNightlyFixture(t)})

		// The rules textarea rides the params form — one Save for the panel.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/routines/smine-nightly/params", url.Values{"auto_apply_content": {"# Rules\n- new rule\n"}}))
		require.Equal(t, http.StatusOK, response.Code)
		saved, err := os.ReadFile(rulesPath)
		require.NoError(t, err)
		assert.Equal(t, "# Rules\n- new rule\n", string(saved))

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/smine-nightly/configure", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "- new rule")
		assert.Contains(t, body, "<textarea")
	})

	t.Run("oversize-rejected", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "rules.md")
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath, RoutinesDir: smineNightlyFixture(t)})

		oversize := strings.Repeat("a", maxAutoApplyRulesBytes+1)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/routines/smine-nightly/params", url.Values{"auto_apply_content": {oversize}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		_, err := os.Stat(rulesPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("other-routine-field-ignored", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "rules.md")
		routinesDir := t.TempDir()
		routineDir := filepath.Join(routinesDir, "demo")
		require.NoError(t, os.MkdirAll(routineDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath, RoutinesDir: routinesDir})

		// A routine without the editor never writes the rules file, even if
		// the field is posted.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{"auto_apply_content": {"injected"}}))
		require.Equal(t, http.StatusOK, response.Code)
		_, err := os.Stat(rulesPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("missing-file-page-renders-error", func(t *testing.T) {
		rulesPath := filepath.Join(t.TempDir(), "absent.md")
		server := newTestServer(t, &Options{AutoApplyRulesPath: rulesPath, RoutinesDir: smineNightlyFixture(t)})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines/smine-nightly/configure", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "no such file")
		assert.Contains(t, body, "<textarea")
	})
}
