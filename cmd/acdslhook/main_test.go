package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const (
	contextFile     = "data.json"
	contextOutput   = `{"hookSpecificOutput":{"additionalContext":"ACDSL rules governing data.json:\n- [ACDSL-JSON-003] One\n- [ACDSL-JSON-004] Two","hookEventName":"PostToolUse"}}` + "\n"
	goContextOutput = `{"hookSpecificOutput":{"additionalContext":"ACDSL rules governing file.go:\n- [ACDSL-JSON-003] One\n- [ACDSL-JSON-004] Two","hookEventName":"PostToolUse"}}` + "\n"
	governedFile    = "file.go"
	stubExitKey     = "ACDSLHOOK_STUB_EXIT"
	stubLogFile     = "stub.log"
	stubLogKey      = "ACDSLHOOK_STUB_LOG"
	stubOutKey      = "ACDSLHOOK_STUB_OUT"
	stubRules       = "- [ACDSL-JSON-003] One\n- [ACDSL-JSON-004] Two\n"
	stubSleepKey    = "ACDSLHOOK_STUB_SLEEP"
	testSessionId   = "session-1"
	unixAcdsl       = "acdsl"
	windowsAcdsl    = "acdsl.exe"
)

// TestMain doubles the test binary as the stub acdsl (and go): with the stub
// log key set it records its argv in the cwd-relative log, prints the stub
// output and exits with the stub status instead of running tests.
func TestMain(m *testing.M) {
	if os.Getenv(stubLogKey) != "" {
		os.Exit(runStub())
	}
	os.Exit(m.Run())
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedLog      string
		_expectedRunError string
		_expectedStderr   string
		_expectedStdout   string

		args   []string
		cwd    string
		stdin  *strings.Reader
		stdout *bytes.Buffer
	}

	hookConfig := provideHookConfig(t)
	tests := make([]*testCase, 0)

	// repo-binary-used
	repo := provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "repo-binary-used",
		_expectedCode:   0,
		_expectedLog:    "acdsl project -context file.go\n",
		_expectedStdout: goContextOutput,

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// windows-named-binary-used
	repo = provideRepo(t, windowsAcdsl)
	tests = append(tests, &testCase{
		_id:             "windows-named-binary-used",
		_expectedCode:   0,
		_expectedLog:    "acdsl.exe project -context file.go\n",
		_expectedStdout: goContextOutput,

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// go-run-fallback
	repo = provideRepo(t, "")
	tests = append(tests, &testCase{
		_id:             "go-run-fallback",
		_expectedCode:   0,
		_expectedLog:    "go run ./cmd/acdsl project -context file.go\n",
		_expectedStdout: goContextOutput,

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// go-run-fallback-off-no-call
	noFallbackConfig := provideHookConfig(t)
	noFallbackConfig.Acdslhook.GoRunFallback = false
	repo = provideRepo(t, "")
	tests = append(tests, &testCase{
		_id:             "go-run-fallback-off-no-call",
		_expectedCode:   0,
		_expectedLog:    "",
		_expectedStdout: "",

		args:   provideArgs(provideClaudeHome(t, noFallbackConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// binaries-from-config
	windowsOnlyConfig := provideHookConfig(t)
	windowsOnlyConfig.Acdslhook.Binaries = []string{windowsAcdsl}
	repo = provideRepo(t, unixAcdsl)
	provideStub(t, filepath.Join(repo, "bin", windowsAcdsl))
	tests = append(tests, &testCase{
		_id:             "binaries-from-config",
		_expectedCode:   0,
		_expectedLog:    "acdsl.exe project -context file.go\n",
		_expectedStdout: goContextOutput,

		args:   provideArgs(provideClaudeHome(t, windowsOnlyConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// missing-config-errors
	claudeHome := t.TempDir()
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:               "missing-config-errors",
		_expectedCode:     1,
		_expectedLog:      "",
		_expectedRunError: hookconfig.Path(claudeHome),
		_expectedStdout:   "",

		args:   provideArgs(claudeHome),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// no-registry-no-call
	repo = provideRepo(t, unixAcdsl)
	require.NoError(t, os.Remove(filepath.Join(repo, registryPath)))
	tests = append(tests, &testCase{
		_id:             "no-registry-no-call",
		_expectedCode:   0,
		_expectedLog:    "",
		_expectedStdout: "",

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// file-outside-cwd-no-call
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "file-outside-cwd-no-call",
		_expectedCode:   0,
		_expectedLog:    "",
		_expectedStdout: "",

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(t.TempDir(), governedFile)),
		stdout: &bytes.Buffer{},
	})

	// empty-file-path-no-call
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "empty-file-path-no-call",
		_expectedCode:   0,
		_expectedLog:    "",
		_expectedStdout: "",

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, ""),
		stdout: &bytes.Buffer{},
	})

	// malformed-input-exits-1
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "malformed-input-exits-1",
		_expectedCode:   1,
		_expectedLog:    "",
		_expectedStdout: "",

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  strings.NewReader("{"),
		stdout: &bytes.Buffer{},
	})

	// json-rules-printed
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "json-rules-printed",
		_expectedCode:   0,
		_expectedLog:    "acdsl project -context data.json\n",
		_expectedStdout: contextOutput,

		args:   provideArgs(provideClaudeHome(t, hookConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, contextFile)),
		stdout: &bytes.Buffer{},
	})

	// text-from-config
	textConfig := provideHookConfig(t)
	textConfig.Acdslhook.ContextText = "rules of %s:\n%s"
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "text-from-config",
		_expectedCode:   0,
		_expectedLog:    "acdsl project -context data.json\n",
		_expectedStdout: `{"hookSpecificOutput":{"additionalContext":"rules of data.json:\n- [ACDSL-JSON-003] One\n- [ACDSL-JSON-004] Two","hookEventName":"PostToolUse"}}` + "\n",

		args:   provideArgs(provideClaudeHome(t, textConfig)),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, contextFile)),
		stdout: &bytes.Buffer{},
	})

	// switch-off-injects-nothing
	claudeHome = provideClaudeHome(t, hookConfig)
	provideSwitchesOff(t, claudeHome, sessionstate.InjectAcdsl)
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "switch-off-injects-nothing",
		_expectedCode:   0,
		_expectedLog:    "",
		_expectedStderr: "",
		_expectedStdout: "",

		args:   provideArgs(claudeHome),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	// record-garbage-acts-and-logs
	claudeHome = provideClaudeHome(t, hookConfig)
	garbage := provideGarbageRecord(t, claudeHome)
	repo = provideRepo(t, unixAcdsl)
	tests = append(tests, &testCase{
		_id:             "record-garbage-acts-and-logs",
		_expectedCode:   0,
		_expectedLog:    "acdsl project -context file.go\n",
		_expectedStderr: fmt.Sprintf("acdslhook: isOff: IsOff: Load: Failed to decode %s: unexpected end of JSON input\n", garbage),
		_expectedStdout: goContextOutput,

		args:   provideArgs(claudeHome),
		cwd:    repo,
		stdin:  provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout: &bytes.Buffer{},
	})

	provideStubEnv(t, "0", stubRules)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stderr := provideStderr(t)
			runStderr := &bytes.Buffer{}
			exitCode := run(
				t.Context(),
				test.args,
				runStderr,
				test.stdin,
				test.stdout,
			)

			assert.Equal(t, test._expectedCode, exitCode)
			assert.Contains(t, runStderr.String(), test._expectedRunError)
			assert.Equal(t, test._expectedLog, readStubLog(t, test.cwd))
			assert.Equal(t, test._expectedStderr, readStderr(t, stderr))
			assert.Equal(t, test._expectedStdout, test.stdout.String())
		})
	}
}

func TestRun_EmptyOutput(t *testing.T) {
	hookConfig := provideHookConfig(t)
	repo := provideRepo(t, unixAcdsl)
	stdout := &bytes.Buffer{}
	provideStubEnv(t, "0", "")

	exitCode := run(
		t.Context(),
		provideArgs(provideClaudeHome(t, hookConfig)),
		io.Discard,
		provideEnvelope(t, repo, filepath.Join(repo, governedFile)),
		stdout,
	)

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "acdsl project -context file.go\n", readStubLog(t, repo))
	assert.Empty(t, stdout.String())
}

func TestRun_FailingAcdsl(t *testing.T) {
	hookConfig := provideHookConfig(t)
	repo := provideRepo(t, unixAcdsl)
	stdout := &bytes.Buffer{}
	provideStubEnv(t, "1", stubRules)

	exitCode := run(
		t.Context(),
		provideArgs(provideClaudeHome(t, hookConfig)),
		io.Discard,
		provideEnvelope(t, repo, filepath.Join(repo, contextFile)),
		stdout,
	)

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "acdsl project -context data.json\n", readStubLog(t, repo))
	assert.Empty(t, stdout.String())
}

func TestRun_TimeoutFromConfig(t *testing.T) {
	hookConfig := provideHookConfig(t)
	hookConfig.Acdslhook.TimeoutMs = hookconfig.DurationMs(time.Millisecond)
	repo := provideRepo(t, unixAcdsl)
	stdout := &bytes.Buffer{}
	provideStubEnv(t, "0", stubRules)
	t.Setenv(stubSleepKey, "1m")

	started := time.Now()
	exitCode := run(
		t.Context(),
		provideArgs(provideClaudeHome(t, hookConfig)),
		io.Discard,
		provideEnvelope(t, repo, filepath.Join(repo, contextFile)),
		stdout,
	)

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stdout.String())
	assert.Less(t, time.Since(started), 30*time.Second)
}

func provideArgs(claudeHome string) []string {
	return []string{"-claude-home", claudeHome}
}

// provideClaudeHome is a Claude home holding the rendered hooks config.
func provideClaudeHome(t *testing.T, hookConfig *hookconfig.Config) string {
	t.Helper()

	claudeHome := t.TempDir()
	writeHookConfig(t, claudeHome, hookConfig)
	return claudeHome
}

func provideEnvelope(t *testing.T, cwd string, filePath string) *strings.Reader {
	t.Helper()

	input := &HookInput{
		Cwd:       cwd,
		SessionId: testSessionId,
		ToolInput: ToolInput{FilePath: filePath},
	}
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return strings.NewReader(string(data))
}

// provideGarbageRecord writes an undecodable switch record for the test session and returns its path.
func provideGarbageRecord(t *testing.T, claudeHome string) string {
	t.Helper()

	path := sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o644))
	return path
}

// provideHookConfig is the catalog's default hooks config with the context dir set to docs.
func provideHookConfig(t *testing.T) *hookconfig.Config {
	t.Helper()

	hookConfig := &hookconfig.Config{}
	hookComponents := []components.ID{components.Context, components.Gate, components.Worktrees, components.SessionHelpers}
	for _, id := range hookComponents {
		rendered, err := install.RenderDefaults(id, components.SinkHooksConfig)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(rendered, hookConfig))
	}

	hookConfig.Context.ContextDir = "docs"
	return hookConfig
}

// provideRepo builds an ACDSL repo fixture: a verifier registry, one Go and
// one JSON governed file, and the stub as bin/<binaryName> unless empty.
func provideRepo(t *testing.T, binaryName string) string {
	t.Helper()

	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "acdsl"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, registryPath), []byte("{}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo, governedFile), []byte("package x\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo, contextFile), []byte("{}\n"), 0o644))
	if binaryName != "" {
		provideStub(t, filepath.Join(repo, "bin", binaryName))
	}
	return repo
}

// provideStderr points os.Stderr, where isOff logs, at a file for the test's duration.
func provideStderr(t *testing.T) *os.File {
	t.Helper()

	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	require.NoError(t, err)

	original := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() {
		os.Stderr = original
		stderr.Close()
	})
	return stderr
}

// provideStub copies the running test binary to path as an executable.
func provideStub(t *testing.T, path string) {
	t.Helper()

	executable, err := os.Executable()
	require.NoError(t, err)

	data, err := os.ReadFile(executable)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o755))
}

// provideStubEnv pins every env input of the stub: output and status, and a
// PATH whose first entry holds the stub as go for the go-run fallback.
func provideStubEnv(t *testing.T, exitCode string, rulesOutput string) {
	t.Helper()

	goStubDir := t.TempDir()
	provideStub(t, filepath.Join(goStubDir, "go"))
	if runtime.GOOS == windowsOs {
		provideStub(t, filepath.Join(goStubDir, "go.exe"))
	}

	t.Setenv("PATH", goStubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(stubExitKey, exitCode)
	t.Setenv(stubLogKey, stubLogFile)
	t.Setenv(stubOutKey, rulesOutput)
}

// provideSwitchesOff writes the test session's switch record with the given features off.
func provideSwitchesOff(t *testing.T, claudeHome string, features ...sessionstate.Feature) {
	t.Helper()

	state := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: features}}
	require.NoError(t, sessionstate.Save(sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId), state))
}

func readStderr(t *testing.T, stderr *os.File) string {
	t.Helper()

	data, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	return string(data)
}

// readStubLog returns the argv lines the stub recorded in cwd, empty when it
// never ran.
func readStubLog(t *testing.T, cwd string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(cwd, stubLogFile))
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}

	require.NoError(t, err)
	return string(data)
}

// runStub is the stub acdsl: argv line appended to the log, stub output on
// stdout, stub status as exit code.
func runStub() int {
	line := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ") + "\n"
	logFile, err := os.OpenFile(os.Getenv(stubLogKey), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 1
	}
	defer logFile.Close()

	if _, err := logFile.WriteString(line); err != nil {
		return 1
	}

	if sleep, err := time.ParseDuration(os.Getenv(stubSleepKey)); err == nil {
		time.Sleep(sleep)
	}

	fmt.Print(os.Getenv(stubOutKey))
	exitCode, err := strconv.Atoi(cmp.Or(os.Getenv(stubExitKey), "0"))
	if err != nil {
		return 1
	}
	return exitCode
}

// writeHookConfig writes a complete hooks config where the install renders it.
func writeHookConfig(t *testing.T, claudeHome string, hookConfig *hookconfig.Config) {
	t.Helper()

	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)

	path := hookconfig.Path(claudeHome)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, encoded, 0o644))
}
