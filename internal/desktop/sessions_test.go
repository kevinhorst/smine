package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSession(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedDirectories []string
		_expectedTitle       string
		_expectedTitleSource string
		_shouldPass          bool

		hostSessionId string
		sessionsDir   string
	}

	sessionsDir := t.TempDir()
	writeRecord := func(account string, hostSessionId string, content string) {
		dir := filepath.Join(sessionsDir, account, "org")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, hostSessionId+".json"), []byte(content), 0o600))
	}
	writeRecord("account-a", "local_0001", `{
  "sessionId": "local_0001",
  "cliSessionId": "0000-1111",
  "title": "CC,FD - Design minor issues - [WIP]",
  "titleSource": "tool",
  "sessionPermissionUpdates": [
    {"type": "addDirectories", "directories": ["/Users/dev/Documents/Notes"], "destination": "session"},
    {"type": "addRules", "rules": [{"toolName": "Read", "ruleContent": "//tmp/**"}], "behavior": "allow", "destination": "session"},
    {"type": "addDirectories", "directories": ["/Users/dev/persisted"], "destination": "localSettings"},
    {"type": "addDirectories", "directories": ["/Users/dev/a", "/Users/dev/b"], "destination": "session"}
  ]
}`)
	writeRecord("account-a", "local_0002", `{"title": "My own title", "titleSource": "user", "sessionPermissionUpdates": []}`)
	writeRecord("account-a", "local_0003", `{"title": "No updates key", "titleSource": "auto"}`)
	writeRecord("account-a", "local_0004", "{")
	writeRecord("account-a", "local_0005", `{"title": "twice"}`)
	writeRecord("account-b", "local_0005", `{"title": "twice"}`)

	tests := make([]*testCase, 0)

	// pass-session-directories-in-record-order
	tests = append(tests, &testCase{
		_id:                  "pass-session-directories-in-record-order",
		_expectedDirectories: []string{"/Users/dev/Documents/Notes", "/Users/dev/a", "/Users/dev/b"},
		_expectedTitle:       "CC,FD - Design minor issues - [WIP]",
		_expectedTitleSource: "tool",
		_shouldPass:          true,

		hostSessionId: "local_0001",
		sessionsDir:   sessionsDir,
	})

	// pass-empty-updates
	tests = append(tests, &testCase{
		_id:                  "pass-empty-updates",
		_expectedDirectories: []string{},
		_expectedTitle:       "My own title",
		_expectedTitleSource: "user",
		_shouldPass:          true,

		hostSessionId: "local_0002",
		sessionsDir:   sessionsDir,
	})

	// pass-missing-updates-key
	tests = append(tests, &testCase{
		_id:                  "pass-missing-updates-key",
		_expectedDirectories: []string{},
		_expectedTitle:       "No updates key",
		_expectedTitleSource: "auto",
		_shouldPass:          true,

		hostSessionId: "local_0003",
		sessionsDir:   sessionsDir,
	})

	// fail-bad-json
	tests = append(tests, &testCase{
		_id:                  "fail-bad-json",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "local_0004",
		sessionsDir:   sessionsDir,
	})

	// fail-two-records-for-one-id
	tests = append(tests, &testCase{
		_id:                  "fail-two-records-for-one-id",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "local_0005",
		sessionsDir:   sessionsDir,
	})

	// fail-unknown-session
	tests = append(tests, &testCase{
		_id:                  "fail-unknown-session",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "local_9999",
		sessionsDir:   sessionsDir,
	})

	// fail-missing-store
	tests = append(tests, &testCase{
		_id:                  "fail-missing-store",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "local_0001",
		sessionsDir:   filepath.Join(sessionsDir, "absent"),
	})

	// fail-glob-shaped-id
	tests = append(tests, &testCase{
		_id:                  "fail-glob-shaped-id",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "local_*",
		sessionsDir:   sessionsDir,
	})

	// fail-path-shaped-id
	tests = append(tests, &testCase{
		_id:                  "fail-path-shaped-id",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "../account-a/org/local_0001",
		sessionsDir:   sessionsDir,
	})

	// fail-empty-id
	tests = append(tests, &testCase{
		_id:                  "fail-empty-id",
		_expectedDirectories: []string{},
		_expectedTitle:       "",
		_expectedTitleSource: "",
		_shouldPass:          false,

		hostSessionId: "",
		sessionsDir:   sessionsDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			session, err := ReadSession(test.hostSessionId, test.sessionsDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDirectories, provideDirectories(session))
			assert.Equal(t, test._expectedTitle, provideTitle(session))
			assert.Equal(t, test._expectedTitleSource, provideTitleSource(session))
		})
	}
}

func TestSessionStoreDir(t *testing.T) {
	assert.Equal(t, filepath.Join("/cfg", "Claude", "claude-code-sessions"), SessionStoreDir("/cfg"))
}

// provideDirectories reads the added directories of a session that may be
// nil, so a failing case asserts flat.
func provideDirectories(session *Session) []string {
	if session == nil {
		return []string{}
	}
	return session.AddedDirectories()
}

func provideTitle(session *Session) string {
	if session == nil {
		return ""
	}
	return session.Title
}

func provideTitleSource(session *Session) string {
	if session == nil {
		return ""
	}
	return session.TitleSource
}
