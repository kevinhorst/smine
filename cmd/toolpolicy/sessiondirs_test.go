package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/desktop"
)

func TestSessionDirectories(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		hostSessionId string
		sessionsDir   string
	}

	sessionsDir := t.TempDir()
	recordDir := filepath.Join(sessionsDir, "account", "org")
	require.NoError(t, os.MkdirAll(recordDir, 0o755))
	added := `{"sessionPermissionUpdates":[{"type":"addDirectories","directories":["/Users/dev/Notes"],"destination":"session"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(recordDir, "local_0001.json"), []byte(added), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(recordDir, "local_0002.json"), []byte("{"), 0o600))

	tests := make([]*testCase, 0)

	// added-folder-returned
	tests = append(tests, &testCase{
		_id:       "added-folder-returned",
		_expected: []string{"/Users/dev/Notes"},

		hostSessionId: "local_0001",
		sessionsDir:   sessionsDir,
	})

	// no-host-session-id
	tests = append(tests, &testCase{
		_id:       "no-host-session-id",
		_expected: nil,

		hostSessionId: "",
		sessionsDir:   sessionsDir,
	})

	// unknown-record
	tests = append(tests, &testCase{
		_id:       "unknown-record",
		_expected: nil,

		hostSessionId: "local_9999",
		sessionsDir:   sessionsDir,
	})

	// malformed-record
	tests = append(tests, &testCase{
		_id:       "malformed-record",
		_expected: nil,

		hostSessionId: "local_0002",
		sessionsDir:   sessionsDir,
	})

	// path-shaped-id
	tests = append(tests, &testCase{
		_id:       "path-shaped-id",
		_expected: nil,

		hostSessionId: "../../etc",
		sessionsDir:   sessionsDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sessionDirectories(test.hostSessionId, test.sessionsDir))
		})
	}
}

// TestRun_SessionDirectoryIsRoot runs the gate end to end: a write into a
// folder the Desktop record lists for the session is admitted, the same write
// without the record is denied.
func TestRun_SessionDirectoryIsRoot(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedStdout string

		hostSessionId string
	}

	base := fixtureBase(t)
	home := filepath.Join(base, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	writeHookConfig(t, filepath.Join(home, ".claude"), renderedHookConfig(t))
	configDir, err := os.UserConfigDir()
	require.NoError(t, err)

	added := filepath.Join(base, "added")
	cwd := filepath.Join(base, "work")
	recordDir := filepath.Join(desktop.SessionStoreDir(configDir), "account", "org")
	for _, dir := range []string{added, cwd, recordDir} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}

	update := desktop.PermissionUpdate{Destination: "session", Directories: []string{added}, Type: "addDirectories"}
	record, err := json.Marshal(&desktop.Session{PermissionUpdates: []desktop.PermissionUpdate{update}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(recordDir, "local_0001.json"), record, 0o600))

	target, err := json.Marshal(filepath.Join(added, "note.txt"))
	require.NoError(t, err)
	stdin := `{"session_id":"s1","cwd":"` + cwd + `","permission_mode":"default","tool_name":"Write","tool_input":{"file_path":` + string(target) + `}}`

	tests := make([]*testCase, 0)

	// listed-folder-write-allowed
	tests = append(tests, &testCase{
		_id:             "listed-folder-write-allowed",
		_expectedStdout: `"permissionDecision":"allow"`,

		hostSessionId: "local_0001",
	})

	// unlisted-session-write-denied
	tests = append(tests, &testCase{
		_id:             "unlisted-session-write-denied",
		_expectedStdout: `"permissionDecision":"deny"`,

		hostSessionId: "local_0002",
	})

	// no-desktop-session-write-denied
	tests = append(tests, &testCase{
		_id:             "no-desktop-session-write-denied",
		_expectedStdout: `"permissionDecision":"deny"`,

		hostSessionId: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(desktop.HostSessionEnv, test.hostSessionId)
			stdout := &strings.Builder{}
			exit := run([]string{}, strings.NewReader(stdin), stdout, &strings.Builder{})
			assert.Equal(t, 0, exit)
			assert.Contains(t, stdout.String(), test._expectedStdout)
		})
	}
}
