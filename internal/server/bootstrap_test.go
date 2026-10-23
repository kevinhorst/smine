package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBootstrapToken stores a legacy routine token file and returns the
// token dir the server reads — the bootstrap refuses to start without one.
func writeBootstrapToken(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "claude-routine")
	tokenDir := filepath.Join(base, "tokens")
	require.NoError(t, os.MkdirAll(tokenDir, 0o700))
	tokenPath := filepath.Join(base, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("sk-test-token"), 0o600))
	return tokenDir
}

func TestWelcomeBootstrapRefusals(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedStatus int

		form   url.Values
		server *Server
	}

	tests := make([]*testCase, 0)

	// fail-invalid-since
	tokenOptions := &Options{TokenDir: writeBootstrapToken(t)}
	invalidSinceForm := url.Values{"since": {"banana"}}
	tests = append(tests, &testCase{
		_id:             "fail-invalid-since",
		_expectedBody:   `Invalid field since: "banana"`,
		_expectedStatus: http.StatusBadRequest,

		form:   invalidSinceForm,
		server: newTestServer(t, tokenOptions),
	})

	// fail-missing-token
	noTokenOptions := &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")}
	validSinceForm := url.Values{"since": {"2026-08-01"}}
	tests = append(tests, &testCase{
		_id:             "fail-missing-token",
		_expectedBody:   "bootstrap needs the routine token: no token file to verify — save one first",
		_expectedStatus: http.StatusConflict,

		form:   validSinceForm,
		server: newTestServer(t, noTokenOptions),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/welcome/bootstrap", test.form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
			assert.Zero(t, test.server.bootstrap.pid)
			assert.False(t, test.server.bootstrap.running)
		})
	}
}

func TestWelcomeBootstrapStarts(t *testing.T) {
	// A failed exec of a missing path exits 126 or 127 depending on the bash
	// version.
	execFailedExitCodes := []int{126, 127}

	// An empty cwd has no bin/routine: the spawned bash's exec fails at once,
	// so the real bootstrap (a paid claude run) never starts.
	t.Chdir(t.TempDir())
	logDir := t.TempDir()
	t.Setenv("TMPDIR", logDir)
	t.Setenv("TMP", logDir)
	t.Setenv("TEMP", logDir)
	options := &Options{TokenDir: writeBootstrapToken(t)}
	server := newTestServer(t, options)
	form := url.Values{"since": {"2026-08-01"}}
	isExited := func() bool {
		server.bootstrap.mu.Lock()
		defer server.bootstrap.mu.Unlock()
		return server.bootstrap.exited
	}

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/welcome/bootstrap", form))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "bootstrap started (pid ")
	assert.Contains(t, recorder.Body.String(), `hx-get="/welcome/bootstrap/status"`)
	require.Eventually(t, isExited, 10*time.Second, 20*time.Millisecond)

	server.bootstrap.mu.Lock()
	defer server.bootstrap.mu.Unlock()
	assert.Positive(t, server.bootstrap.pid)
	assert.False(t, server.bootstrap.running)
	logContent, err := os.ReadFile(filepath.Join(logDir, "smine-bootstrap.log"))
	require.NoError(t, err)
	assert.Containsf(t, execFailedExitCodes, server.bootstrap.exitCode, "log: %s", logContent)
	assert.Contains(t, string(logContent), "bin/routine")
}

func TestWelcomeBootstrapStatus(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedDetail     string
		_expectedPolling    bool
		_expectedSessionRow bool

		server *Server
	}

	// resolveBootstrapSession matches peek sessions against the server's cwd.
	t.Chdir(t.TempDir())
	workDir, err := os.Getwd()
	require.NoError(t, err)
	startedAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	unreachablePeek := "http://127.0.0.1:1/mcp"
	peekWith := func(cwd string, lastActive time.Time) string {
		item := map[string]any{
			"id":          "boot-session",
			"last_active": lastActive.Format(time.RFC3339),
			"meta":        map[string]any{"cwd": cwd},
			"title":       "bootstrap",
		}
		return newStubPeek(t, []map[string]any{item}).URL
	}
	statusServer := func(exitCode int, isExited bool, isRunning bool, peekEndpoint string, pid int) *Server {
		options := &Options{PeekEndpoint: peekEndpoint}
		server := newTestServer(t, options)
		server.bootstrap.exitCode = exitCode
		server.bootstrap.exited = isExited
		server.bootstrap.pid = pid
		server.bootstrap.running = isRunning
		server.bootstrap.startedAt = startedAt
		return server
	}
	tests := make([]*testCase, 0)

	// no-run-yet
	tests = append(tests, &testCase{
		_id:                 "no-run-yet",
		_expectedDetail:     `<span class="label">no bootstrap run yet</span>`,
		_expectedPolling:    false,
		_expectedSessionRow: false,

		server: statusServer(0, false, false, unreachablePeek, 0),
	})

	// running-peek-unreachable
	tests = append(tests, &testCase{
		_id:                 "running-peek-unreachable",
		_expectedDetail:     `<span class="label">bootstrap running (pid 42) — session not indexed yet…</span>`,
		_expectedPolling:    true,
		_expectedSessionRow: false,

		server: statusServer(0, false, true, unreachablePeek, 42),
	})

	// running-fresh-session
	tests = append(tests, &testCase{
		_id:                 "running-fresh-session",
		_expectedDetail:     `<span class="label">bootstrap running (pid 42) — session</span>`,
		_expectedPolling:    true,
		_expectedSessionRow: true,

		server: statusServer(0, false, true, peekWith(workDir, startedAt.Add(time.Minute)), 42),
	})

	// running-session-within-tolerance — up to one minute before the start counts
	tests = append(tests, &testCase{
		_id:                 "running-session-within-tolerance",
		_expectedDetail:     `<span class="label">bootstrap running (pid 42) — session</span>`,
		_expectedPolling:    true,
		_expectedSessionRow: true,

		server: statusServer(0, false, true, peekWith(workDir, startedAt.Add(-30*time.Second)), 42),
	})

	// running-stale-session — an older session in the same folder is not the run
	tests = append(tests, &testCase{
		_id:                 "running-stale-session",
		_expectedDetail:     `<span class="label">bootstrap running (pid 42) — session not indexed yet…</span>`,
		_expectedPolling:    true,
		_expectedSessionRow: false,

		server: statusServer(0, false, true, peekWith(workDir, startedAt.Add(-2*time.Minute)), 42),
	})

	// running-other-cwd — a fresh session elsewhere is not the run
	tests = append(tests, &testCase{
		_id:                 "running-other-cwd",
		_expectedDetail:     `<span class="label">bootstrap running (pid 42) — session not indexed yet…</span>`,
		_expectedPolling:    true,
		_expectedSessionRow: false,

		server: statusServer(0, false, true, peekWith("/elsewhere", startedAt.Add(time.Minute)), 42),
	})

	// failed-exit-code
	tests = append(tests, &testCase{
		_id:                 "failed-exit-code",
		_expectedDetail:     `<span class="label">bootstrap failed (exit 3) — see the log `,
		_expectedPolling:    false,
		_expectedSessionRow: false,

		server: statusServer(3, true, false, unreachablePeek, 42),
	})

	// finished-with-session
	tests = append(tests, &testCase{
		_id:                 "finished-with-session",
		_expectedDetail:     `<span class="label">bootstrap finished — session</span>`,
		_expectedPolling:    false,
		_expectedSessionRow: true,

		server: statusServer(0, true, false, peekWith(workDir, startedAt.Add(time.Minute)), 42),
	})

	// finished-without-session
	tests = append(tests, &testCase{
		_id:                 "finished-without-session",
		_expectedDetail:     `<span class="label">bootstrap finished (pid 42) — see the peek dashboard or the log</span>`,
		_expectedPolling:    false,
		_expectedSessionRow: false,

		server: statusServer(0, true, false, unreachablePeek, 42),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/welcome/bootstrap/status", nil))
			body := recorder.Body.String()

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, body, test._expectedDetail)
			assert.Equal(t, test._expectedPolling, strings.Contains(body, `hx-trigger="every 5s"`))
			assert.Equal(t, test._expectedSessionRow, strings.Contains(body, "<code>boot-session</code>"))
		})
	}
}

func TestBootstrapEnv(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		extraPrompt string
		since       string
	}

	tests := make([]*testCase, 0)

	// empty-inputs
	tests = append(tests, &testCase{
		_id:         "empty-inputs",
		_expected:   []string{},
		_shouldPass: true,

		extraPrompt: "",
		since:       "",
	})

	// valid-since
	tests = append(tests, &testCase{
		_id:         "valid-since",
		_expected:   []string{"BOOTSTRAP_SINCE=2026-08-01"},
		_shouldPass: true,

		extraPrompt: "",
		since:       "2026-08-01",
	})

	// invalid-since
	tests = append(tests, &testCase{
		_id:         "invalid-since",
		_expected:   nil,
		_shouldPass: false,

		extraPrompt: "",
		since:       "banana",
	})

	// extra-prompt-appended
	tests = append(tests, &testCase{
		_id:         "extra-prompt-appended",
		_expected:   []string{"BOOTSTRAP_SINCE=2026-08-01", "BOOTSTRAP_EXTRA_PROMPT=skip dev sessions"},
		_shouldPass: true,

		extraPrompt: "skip dev sessions",
		since:       "2026-08-01",
	})

	// extra-prompt-only
	tests = append(tests, &testCase{
		_id:         "extra-prompt-only",
		_expected:   []string{"BOOTSTRAP_EXTRA_PROMPT=focus on tvde work"},
		_shouldPass: true,

		extraPrompt: "focus on tvde work",
		since:       "",
	})

	// quote-in-extra-prompt
	tests = append(tests, &testCase{
		_id:         "quote-in-extra-prompt",
		_expected:   nil,
		_shouldPass: false,

		extraPrompt: `say "hi"`,
		since:       "",
	})

	// newline-in-extra-prompt
	tests = append(tests, &testCase{
		_id:         "newline-in-extra-prompt",
		_expected:   nil,
		_shouldPass: false,

		extraPrompt: "line one\nline two",
		since:       "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			env, err := bootstrapEnv(test.since, test.extraPrompt)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, env)
		})
	}
}
