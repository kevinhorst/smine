package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const (
	eventSessionStart  = "SessionStart"
	eventSubagentStart = "SubagentStart"
	testSessionId      = "session-1"
)

func TestRun(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedCode      int
		_expectedInjection string
		_expectedRunError  string
		_expectedStderr    string
		_expectedStdout    string

		args       []string
		claudeHome string
		stdin      *strings.Reader
		stdout     *bytes.Buffer
	}

	hookConfig := provideHookConfig(t)
	section := &hookConfig.Globalcontext
	tests := make([]*testCase, 0)
	large := strings.Repeat("rule\n", 2000)

	// no-files-silent
	claudeHome := provideClaudeHome(t, hookConfig)
	repo := t.TempDir()
	tests = append(tests, &testCase{
		_id:                "no-files-silent",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// small-content-inline
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	profile := provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	inline := provideGathered(section.Banner, profile, "language: en\n")
	tests = append(tests, &testCase{
		_id:                "small-content-inline",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventSessionStart, inline),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// banner-and-order-machine-then-repo
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	machine := provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "z-machine.md", "machine\n")
	repoFile := provideGlobalFile(t, filepath.Join(repo, "docs", "global"), "a-repo.md", "repo\n")
	ordered := fmt.Sprintf("%s\n\n===== %s =====\nmachine\n\n===== %s =====\nrepo\n\n", section.Banner, machine, repoFile)
	tests = append(tests, &testCase{
		_id:                "banner-and-order-machine-then-repo",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventSessionStart, ordered),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// large-content-writes-injection-with-pointer
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	always := provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "always-context.md", large)
	injected := provideGathered(section.Banner, always, large)
	tests = append(tests, &testCase{
		_id:                "large-content-writes-injection-with-pointer",
		_expectedCode:      0,
		_expectedInjection: injected,
		_expectedStdout:    provideOutput(t, eventSessionStart, providePointer(claudeHome, injected, section)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// shrunk-content-removes-stale-injection
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideStaleInjection(t, claudeHome)
	profile = provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	inline = provideGathered(section.Banner, profile, "language: en\n")
	tests = append(tests, &testCase{
		_id:                "shrunk-content-removes-stale-injection",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventSessionStart, inline),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// subagent-start-same-injection
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	always = provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "always-context.md", large)
	injected = provideGathered(section.Banner, always, large)
	tests = append(tests, &testCase{
		_id:                "subagent-start-same-injection",
		_expectedCode:      0,
		_expectedInjection: injected,
		_expectedStdout:    provideOutput(t, eventSubagentStart, providePointer(claudeHome, injected, section)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSubagentStart),
		stdout:     &bytes.Buffer{},
	})

	// missing-config-errors
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	tests = append(tests, &testCase{
		_id:                "missing-config-errors",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedRunError:  hookconfig.Path(claudeHome),
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// custom-context-dir-from-config
	customDirConfig := provideHookConfig(t)
	customDirConfig.Context.ContextDir = "context"
	claudeHome = provideClaudeHome(t, customDirConfig)
	repo = t.TempDir()
	provideGlobalFile(t, filepath.Join(repo, "docs", "global"), "ignored.md", "ignored\n")
	company := provideGlobalFile(t, filepath.Join(repo, "context", "global"), "company.md", "company\n")
	tests = append(tests, &testCase{
		_id:                "custom-context-dir-from-config",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventSessionStart, provideGathered(section.Banner, company, "company\n")),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-injects-home-and-names-setting
	emptyDirConfig := provideHookConfig(t)
	emptyDirConfig.Context.ContextDir = ""
	claudeHome = provideClaudeHome(t, emptyDirConfig)
	repo = t.TempDir()
	profile = provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	provideGlobalFile(t, filepath.Join(repo, "global"), "ignored.md", "ignored\n")
	homeOnly := provideGathered(section.Banner, profile, "language: en\n") + "\n" + noContextDirFormat
	tests = append(tests, &testCase{
		_id:                "empty-context-dir-injects-home-and-names-setting",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventSessionStart, homeOnly),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-no-files-names-setting
	claudeHome = provideClaudeHome(t, emptyDirConfig)
	repo = t.TempDir()
	tests = append(tests, &testCase{
		_id:                "empty-context-dir-no-files-names-setting",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventSessionStart, noContextDirFormat),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-large-content-pointer-then-setting
	claudeHome = provideClaudeHome(t, emptyDirConfig)
	repo = t.TempDir()
	always = provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "always-context.md", large)
	injected = provideGathered(section.Banner, always, large)
	tests = append(tests, &testCase{
		_id:                "empty-context-dir-large-content-pointer-then-setting",
		_expectedCode:      0,
		_expectedInjection: injected,
		_expectedStdout:    provideOutput(t, eventSubagentStart, providePointer(claudeHome, injected, section)+"\n"+noContextDirFormat),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSubagentStart),
		stdout:     &bytes.Buffer{},
	})

	// inline-cap-from-config
	smallCapConfig := provideHookConfig(t)
	smallCapConfig.Globalcontext.InlineCap = 10
	claudeHome = provideClaudeHome(t, smallCapConfig)
	repo = t.TempDir()
	profile = provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	injected = provideGathered(section.Banner, profile, "language: en\n")
	tests = append(tests, &testCase{
		_id:                "inline-cap-from-config",
		_expectedCode:      0,
		_expectedInjection: injected,
		_expectedStdout:    provideOutput(t, eventSessionStart, providePointer(claudeHome, injected, section)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// missing-session-id-exits-1
	claudeHome = provideClaudeHome(t, hookConfig)
	tests = append(tests, &testCase{
		_id:                "missing-session-id-exits-1",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      strings.NewReader(`{"cwd": "/repo", "hook_event_name": "SessionStart"}`),
		stdout:     &bytes.Buffer{},
	})

	// switch-off-no-output
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	provideSwitchesOff(t, claudeHome, sessionstate.InjectGlobal)
	tests = append(tests, &testCase{
		_id:                "switch-off-no-output",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStderr:    "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// switch-off-subagent-no-output
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "always-context.md", large)
	provideSwitchesOff(t, claudeHome, sessionstate.InjectGlobal)
	tests = append(tests, &testCase{
		_id:                "switch-off-subagent-no-output",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStderr:    "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSubagentStart),
		stdout:     &bytes.Buffer{},
	})

	// record-garbage-injects-and-logs
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	profile = provideGlobalFile(t, filepath.Join(claudeHome, "context", "global"), "profile.md", "language: en\n")
	garbage := provideGarbageRecord(t, claudeHome)
	tests = append(tests, &testCase{
		_id:                "record-garbage-injects-and-logs",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStderr:    fmt.Sprintf("globalcontext: isOff: IsOff: Load: Failed to decode %s: unexpected end of JSON input\n", garbage),
		_expectedStdout:    provideOutput(t, eventSessionStart, provideGathered(section.Banner, profile, "language: en\n")),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, repo, eventSessionStart),
		stdout:     &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stderr := provideStderr(t)
			runStderr := &bytes.Buffer{}
			exitCode := run(test.args, runStderr, test.stdin, test.stdout)

			assert.Equal(t, test._expectedCode, exitCode)
			assert.Contains(t, runStderr.String(), test._expectedRunError)
			assert.Equal(t, test._expectedStderr, readStderr(t, stderr))
			assert.Equal(t, test._expectedStdout, test.stdout.String())
			assert.Equal(t, test._expectedInjection, readInjection(t, test.claudeHome))
		})
	}
}

func provideArgs(claudeHome string) []string {
	return []string{"-claude-home", claudeHome}
}

// provideClaudeHome is a Claude home holding the rendered hooks config.
func provideClaudeHome(t *testing.T, hookConfig *hookconfig.Config) string {
	claudeHome := t.TempDir()
	writeHookConfig(t, claudeHome, hookConfig)
	return claudeHome
}

func provideEnvelope(t *testing.T, cwd string, event string) *strings.Reader {
	input := &HookInput{
		Cwd:           cwd,
		HookEventName: event,
		SessionId:     testSessionId,
	}
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return strings.NewReader(string(data))
}

// provideGarbageRecord writes an undecodable switch record for the test session and returns its path.
func provideGarbageRecord(t *testing.T, claudeHome string) string {
	path := sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte("{"), 0o644)
	require.NoError(t, err)
	return path
}

// provideGathered is the injected shape of one global file: banner, path
// header, content.
func provideGathered(banner string, path string, content string) string {
	return fmt.Sprintf("%s\n\n===== %s =====\n%s\n", banner, path, content)
}

// provideGlobalFile writes one global context file into dir and returns its
// path.
func provideGlobalFile(t *testing.T, dir string, name string, content string) string {
	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err)

	path := filepath.Join(dir, name)
	err = os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
	return path
}

// provideHookConfig is the catalog's default hooks config with the context dir set to docs.
func provideHookConfig(t *testing.T) *hookconfig.Config {
	hookConfig := &hookconfig.Config{}
	hookComponents := []components.ID{components.Context, components.Gate, components.Worktrees, components.SessionHelpers}
	for _, id := range hookComponents {
		rendered, err := install.RenderDefaults(id, components.SinkHooksConfig)
		require.NoError(t, err)

		err = json.Unmarshal(rendered, hookConfig)
		require.NoError(t, err)
	}

	hookConfig.Context.ContextDir = "docs"
	return hookConfig
}

func provideInjectionPath(claudeHome string) string {
	return filepath.Join(contextrequired.Dir(claudeHome, testSessionId), contextrequired.GlobalFile)
}

func provideOutput(t *testing.T, event string, addition string) string {
	output := &HookOutput{
		HookSpecificOutput: HookSpecificOutput{
			AdditionalContext: addition,
			HookEventName:     event,
		},
	}
	data, err := json.Marshal(output)
	require.NoError(t, err)
	return string(data) + "\n"
}

func providePointer(claudeHome string, injected string, section *hookconfig.Globalcontext) string {
	path := provideInjectionPath(claudeHome)
	lines := strings.Count(injected, "\n")
	pointer := fmt.Sprintf(section.PointerFormat, section.Banner, path, len(injected), lines)
	return pointer + "\n" + strings.Join(contextrequired.ReadInstructions(lines, path, 1, section.ReadBudget), "\n")
}

func provideStaleInjection(t *testing.T, claudeHome string) {
	injection := provideInjectionPath(claudeHome)
	err := os.MkdirAll(filepath.Dir(injection), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(injection, []byte("stale\n"), 0o644)
	require.NoError(t, err)
}

// provideStderr points os.Stderr, where isOff logs, at a file for the test's duration.
func provideStderr(t *testing.T) *os.File {
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

// provideSwitchesOff writes the test session's switch record with the given features off.
func provideSwitchesOff(t *testing.T, claudeHome string, features ...sessionstate.Feature) {
	state := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: features}}
	err := sessionstate.Save(sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId), state)
	require.NoError(t, err)
}

// readInjection returns the session's global injection file, "" when there is
// none.
func readInjection(t *testing.T, claudeHome string) string {
	data, err := os.ReadFile(provideInjectionPath(claudeHome))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}

	require.NoError(t, err)
	return string(data)
}

func readStderr(t *testing.T, stderr *os.File) string {
	data, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	return string(data)
}

// writeHookConfig writes a complete hooks config where the install renders it.
func writeHookConfig(t *testing.T, claudeHome string, hookConfig *hookconfig.Config) {
	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)

	path := hookconfig.Path(claudeHome)
	err = os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, encoded, 0o644)
	require.NoError(t, err)
}
