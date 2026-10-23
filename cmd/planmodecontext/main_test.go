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
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const (
	eventUserPromptSubmit = "UserPromptSubmit"
	indexNoPlanEntries    = `{"entries":[{"id":"RULE-SKILL-001","content":{"statement":"Other."}}]}`
	indexPlanEntries      = `{"entries":[{"id":"RULE-PLAN-001","content":{"statement":"First."}},{"id":"RULE-SKILL-001","content":{"statement":"Other."}},{"id":"RULE-PLAN-002","content":{"statement":"Second."}}]}`
	modeDefault           = "default"
	staleInjection        = "stale\n"
	testAgentId           = "agent-9"
	testSessionId         = "session-1"
	toolBash              = "Bash"
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
	tests := make([]*testCase, 0)
	packed := providePack(&hookConfig.Planmodecontext)

	// ui-plan-mode-prompt-writes-injection
	claudeHome := provideClaudeHome(t, hookConfig)
	repo := t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input := &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "ui-plan-mode-prompt-writes-injection",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStdout:    provideOutput(t, eventUserPromptSubmit, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// ui-plan-mode-tool-call-writes-injection
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: permissionModePlan, SessionId: testSessionId, ToolName: toolBash}
	tests = append(tests, &testCase{
		_id:                "ui-plan-mode-tool-call-writes-injection",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStdout:    provideOutput(t, eventPostToolUse, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// agent-enter-plan-mode-call-writes-injection
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: modeDefault, SessionId: testSessionId, ToolName: toolEnterPlanMode}
	tests = append(tests, &testCase{
		_id:                "agent-enter-plan-mode-call-writes-injection",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStdout:    provideOutput(t, eventPostToolUse, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// enter-plan-mode-name-on-prompt-event-ignored
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: modeDefault, SessionId: testSessionId, ToolName: toolEnterPlanMode}
	tests = append(tests, &testCase{
		_id:                "enter-plan-mode-name-on-prompt-event-ignored",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// already-delivered-silent
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	provideInjection(t, claudeHome)
	input = &HookInput{Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: permissionModePlan, SessionId: testSessionId, ToolName: toolBash}
	tests = append(tests, &testCase{
		_id:                "already-delivered-silent",
		_expectedCode:      0,
		_expectedInjection: staleInjection,
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// left-plan-mode-removes-injection
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	provideInjection(t, claudeHome)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: modeDefault, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "left-plan-mode-removes-injection",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// outside-plan-mode-silent
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: modeDefault, SessionId: testSessionId, ToolName: toolBash}
	tests = append(tests, &testCase{
		_id:                "outside-plan-mode-silent",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// missing-permission-mode-counts-as-outside
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	provideInjection(t, claudeHome)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "missing-permission-mode-counts-as-outside",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// subagent-in-plan-mode-ignored
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{AgentId: testAgentId, Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: permissionModePlan, SessionId: testSessionId, ToolName: toolBash}
	tests = append(tests, &testCase{
		_id:                "subagent-in-plan-mode-ignored",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// root-only-false-injects-into-subagent
	allAgentsConfig := provideHookConfig(t)
	allAgentsConfig.Planmodecontext.RootOnly = false
	claudeHome = provideClaudeHome(t, allAgentsConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{AgentId: testAgentId, Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: permissionModePlan, SessionId: testSessionId, ToolName: toolBash}
	tests = append(tests, &testCase{
		_id:                "root-only-false-injects-into-subagent",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStdout:    provideOutput(t, eventPostToolUse, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// subagent-outside-plan-mode-keeps-injection
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	provideInjection(t, claudeHome)
	input = &HookInput{AgentId: testAgentId, Cwd: repo, HookEventName: eventPostToolUse, PermissionMode: modeDefault, SessionId: testSessionId, ToolName: toolBash}
	tests = append(tests, &testCase{
		_id:                "subagent-outside-plan-mode-keeps-injection",
		_expectedCode:      0,
		_expectedInjection: staleInjection,
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// no-index-silent
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "no-index-silent",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// index-without-plan-entries-silent
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexNoPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "index-without-plan-entries-silent",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// index-in-fallback-context-dir
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "context"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "index-in-fallback-context-dir",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStdout:    provideOutput(t, eventUserPromptSubmit, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// custom-context-dir-from-config
	customDirConfig := provideHookConfig(t)
	customDirConfig.Context.ContextDir = "knowledge"
	claudeHome = provideClaudeHome(t, customDirConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "knowledge"), indexPlanEntries)
	provideIndex(t, filepath.Join(repo, "context"), indexNoPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "custom-context-dir-from-config",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStdout:    provideOutput(t, eventUserPromptSubmit, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-names-setting
	emptyDirConfig := provideHookConfig(t)
	emptyDirConfig.Context.ContextDir = ""
	claudeHome = provideClaudeHome(t, emptyDirConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "context"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "empty-context-dir-names-setting",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    provideOutput(t, eventUserPromptSubmit, noContextDirFormat),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-outside-plan-mode-silent
	claudeHome = provideClaudeHome(t, emptyDirConfig)
	repo = t.TempDir()
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: modeDefault, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "empty-context-dir-outside-plan-mode-silent",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// missing-config-errors
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "missing-config-errors",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedRunError:  hookconfig.Path(claudeHome),
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// malformed-index-exits-1
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), `{"entries": [`)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "malformed-index-exits-1",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
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
		stdin:      strings.NewReader(`{"cwd": "/repo", "hook_event_name": "UserPromptSubmit", "permission_mode": "plan"}`),
		stdout:     &bytes.Buffer{},
	})

	// missing-cwd-exits-1
	claudeHome = provideClaudeHome(t, hookConfig)
	tests = append(tests, &testCase{
		_id:                "missing-cwd-exits-1",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      strings.NewReader(`{"session_id": "session-1", "hook_event_name": "UserPromptSubmit", "permission_mode": "plan"}`),
		stdout:     &bytes.Buffer{},
	})

	// truncated-envelope-exits-1
	claudeHome = provideClaudeHome(t, hookConfig)
	tests = append(tests, &testCase{
		_id:                "truncated-envelope-exits-1",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      strings.NewReader(`{"session_id": "session-1", "cwd"`),
		stdout:     &bytes.Buffer{},
	})

	// empty-envelope-exits-1
	claudeHome = provideClaudeHome(t, hookConfig)
	tests = append(tests, &testCase{
		_id:                "empty-envelope-exits-1",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      strings.NewReader(""),
		stdout:     &bytes.Buffer{},
	})

	// unknown-flag-exits-1-never-2
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "unknown-flag-exits-1-never-2",
		_expectedCode:      1,
		_expectedInjection: "",
		_expectedStdout:    "",

		args:       []string{"-no-such-flag"},
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// switch-off-no-output
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	provideSwitchesOff(t, claudeHome, sessionstate.InjectPlan)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "switch-off-no-output",
		_expectedCode:      0,
		_expectedInjection: "",
		_expectedStderr:    "",
		_expectedStdout:    "",

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
		stdout:     &bytes.Buffer{},
	})

	// record-garbage-injects-and-logs
	claudeHome = provideClaudeHome(t, hookConfig)
	repo = t.TempDir()
	provideIndex(t, filepath.Join(repo, "docs"), indexPlanEntries)
	garbage := provideGarbageRecord(t, claudeHome)
	input = &HookInput{Cwd: repo, HookEventName: eventUserPromptSubmit, PermissionMode: permissionModePlan, SessionId: testSessionId}
	tests = append(tests, &testCase{
		_id:                "record-garbage-injects-and-logs",
		_expectedCode:      0,
		_expectedInjection: packed,
		_expectedStderr:    fmt.Sprintf("planmodecontext: isOff: IsOff: Load: Failed to decode %s: unexpected end of JSON input\n", garbage),
		_expectedStdout:    provideOutput(t, eventUserPromptSubmit, providePointer(claudeHome, packed, hookConfig)),

		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, input),
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

func TestClaim(t *testing.T) {
	injection := provideInjectionPath(t.TempDir())

	isFirstClaimed, firstErr := claim("first\n", injection)
	isSecondClaimed, secondErr := claim("second\n", injection)
	data, readErr := os.ReadFile(injection)

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, readErr)
	assert.True(t, isFirstClaimed)
	assert.False(t, isSecondClaimed)
	assert.Equal(t, "first\n", string(data))
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

func provideEnvelope(t *testing.T, input *HookInput) *strings.Reader {
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

// provideIndex writes a context index with the content into dir.
func provideIndex(t *testing.T, dir string, content string) {
	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, contextdocs.ContextFileName), []byte(content), 0o644)
	require.NoError(t, err)
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

// providePack is the pack of indexPlanEntries: the two RULE-PLAN lines in
// index order, the foreign entry left out.
func providePack(section *hookconfig.Planmodecontext) string {
	entries := "- **RULE-PLAN-001** — First.\n- **RULE-PLAN-002** — Second.\n"
	return fmt.Sprintf(section.PackFormat, section.Banner, section.Heading, entries, section.Trailer)
}

func providePointer(claudeHome string, packed string, hookConfig *hookconfig.Config) string {
	path := provideInjectionPath(claudeHome)
	lines := strings.Count(packed, "\n")
	section := hookConfig.Planmodecontext
	pointer := fmt.Sprintf(section.PointerFormat, section.Banner, path, len(packed), lines)
	return pointer + "\n" + strings.Join(contextrequired.ReadInstructions(lines, path, 1, hookConfig.Globalcontext.ReadBudget), "\n")
}

// provideInjection leaves a injection of an earlier delivery in the session's dir.
func provideInjection(t *testing.T, claudeHome string) {
	injection := provideInjectionPath(claudeHome)
	err := os.MkdirAll(filepath.Dir(injection), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(injection, []byte(staleInjection), 0o644)
	require.NoError(t, err)
}

func provideInjectionPath(claudeHome string) string {
	return filepath.Join(contextrequired.Dir(claudeHome, testSessionId), contextrequired.PlanModeFile)
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

// readInjection returns the session's plan injection, "" when there is none.
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
