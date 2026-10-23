package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kevinhorst/smine/internal/routines"
	"github.com/stretchr/testify/require"
)

// The routines fixtures shared with the backlog, profile, sessions and welcome tests; routines_test.go itself stays out of the public sync.

type hangingPeek struct {
	calls atomic.Int32
	url   string
}

func newHangingPeek(t *testing.T) *hangingPeek {
	t.Helper()
	release := make(chan struct{})
	hanging := &hangingPeek{}
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hanging.calls.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(stub.Close)
	t.Cleanup(func() { close(release) })
	hanging.url = stub.URL
	return hanging
}

// routinesTestEnvWithTokens builds the routines fixture plus a token store
// seeded with label=value pairs; returns the server, routine dir, and store dir.
func routinesTestEnvWithTokens(t *testing.T, tokens map[string]string) (*Server, string, string) {
	t.Helper()
	routinesDir := t.TempDir()
	routineDir := filepath.Join(routinesDir, "demo")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "routine.json"), []byte(`{"kind":"single","prompt":"/demo"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), []byte(testPlist(routineDir)), 0o644))

	tokenDir := filepath.Join(t.TempDir(), "tokens")
	for label, value := range tokens {
		require.NoError(t, routines.SaveToken(label, value, tokenDir))
	}

	server := newTestServer(t, &Options{RoutinesDir: routinesDir, TokenDir: tokenDir})
	return server, routineDir, tokenDir
}

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
