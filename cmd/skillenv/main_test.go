package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/skillenv"
)

const (
	switchOffSessionId = "switch-off"
	testSessionId      = "session"
)

// encodeOutput is the answer as the hook writes it, so the expected bytes carry the encoder's escaping.
func encodeOutput(t *testing.T, output *HookOutput) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, json.NewEncoder(&buffer).Encode(output))
	return buffer.String()
}

// provideCheckEnvelope encodes one check envelope as the harness sends it on stdin.
func provideCheckEnvelope(t *testing.T, input *HookInput) []byte {
	t.Helper()
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	return raw
}

// provideDocument writes the document as the deployed skill's env.json under the Claude home.
func provideDocument(t *testing.T, claudeHome string, document *skillenv.Document) {
	t.Helper()
	skillDir := filepath.Join(claudeHome, skillsDirName, document.Skill)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	raw, err := json.Marshal(document)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, skillenv.FileName), raw, envFileMode))
}

// provideEnvelope encodes one hook envelope as the harness sends it on stdin.
func provideEnvelope(t *testing.T, cwd string, event string, sessionId string) []byte {
	t.Helper()
	input := &HookInput{Cwd: cwd, HookEventName: event, SessionId: sessionId}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	return raw
}

// provideFile is a temp file holding content: a seeded env file or a broken registry.
func provideFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(path, []byte(content), envFileMode))
	return path
}

// provideRecord writes the Claude home's install record holding the global defaults and the skill rows.
func provideRecord(t *testing.T, claudeHome string, defaults map[string]string, skills map[string]string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(claudeHome, "smine"), 0o755))
	record := install.FullRecord(runtime.GOOS)
	record.Skills.Defaults = defaults
	record.Skills.Variables = skills
	require.NoError(t, install.Save(install.Env{ClaudeHome: claudeHome, GOOS: runtime.GOOS}, record))
}

// provideRegistry writes a registry file holding the rows and returns its path.
func provideRegistry(t *testing.T, rows ...repos.Repo) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.StateRegistryFile)
	registry := repos.NewRegistry(path)
	for _, row := range rows {
		require.NoError(t, registry.Add(row))
	}
	return path
}

// provideSwitchOff turns inject:env off for the session in the Claude home.
func provideSwitchOff(t *testing.T, claudeHome string, sessionId string) {
	t.Helper()
	switchOff := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: []sessionstate.Feature{sessionstate.InjectEnv}}}
	require.NoError(t, sessionstate.Save(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), switchOff))
}

// readEnvFile is the env file's content, empty when the hook never created it.
func readEnvFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	require.NoError(t, err)
	return string(raw)
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCode    int
		_expectedContent string

		args     []string
		envFile  string
		readPath string
		stdin    []byte
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	provideSwitchOff(t, filepath.Join(home, ".claude"), switchOffSessionId)

	base := t.TempDir()
	registered := filepath.Join(base, "registered")
	unregistered := filepath.Join(base, "unregistered")
	require.NoError(t, os.MkdirAll(filepath.Join(registered, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(unregistered, 0o755))

	defaults := map[string]string{
		"SKILL_FIMPLEMENT_CHECK":     "make audit",
		"SKILL_RAILROADREVIEW_BUILD": "go build ./...",
	}
	provideRecord(t, filepath.Join(home, ".claude"), nil, defaults)
	rowEnv := map[string]string{"SKILL_FIMPLEMENT_CHECK": "make test"}
	registryPath := provideRegistry(t, repos.Repo{Env: rowEnv, Name: "registered", Path: registered})
	unreadableRegistryPath := provideFile(t, "{not json")

	rowOverDefaults := "export SKILL_FIMPLEMENT_CHECK='make test'\nexport SKILL_RAILROADREVIEW_BUILD='go build ./...'\n"
	defaultsOnly := "export SKILL_FIMPLEMENT_CHECK='make audit'\nexport SKILL_RAILROADREVIEW_BUILD='go build ./...'\n"
	earlierLine := "export EARLIER='kept'\n"

	tests := make([]*testCase, 0)

	// sessionstart-writes-exports
	sessionStartPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "sessionstart-writes-exports",
		_expectedCode:    0,
		_expectedContent: rowOverDefaults,

		args:     []string{"-env-file", sessionStartPath, "-repos", registryPath},
		envFile:  "",
		readPath: sessionStartPath,
		stdin:    provideEnvelope(t, registered, eventSessionStart, testSessionId),
	})

	// cwdchanged-writes-exports
	cwdChangedPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "cwdchanged-writes-exports",
		_expectedCode:    0,
		_expectedContent: rowOverDefaults,

		args:     []string{"-env-file", cwdChangedPath, "-repos", registryPath},
		envFile:  "",
		readPath: cwdChangedPath,
		stdin:    provideEnvelope(t, registered, eventCwdChanged, testSessionId),
	})

	// other-event-writes-nothing
	otherEventPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "other-event-writes-nothing",
		_expectedCode:    0,
		_expectedContent: "",

		args:     []string{"-env-file", otherEventPath, "-repos", registryPath},
		envFile:  "",
		readPath: otherEventPath,
		stdin:    provideEnvelope(t, registered, "Stop", testSessionId),
	})

	// no-env-file-writes-nothing
	noEnvFilePath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "no-env-file-writes-nothing",
		_expectedCode:    0,
		_expectedContent: "",

		args:     []string{"-repos", registryPath},
		envFile:  "",
		readPath: noEnvFilePath,
		stdin:    provideEnvelope(t, registered, eventSessionStart, testSessionId),
	})

	// unregistered-cwd-defaults-only
	unregisteredPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "unregistered-cwd-defaults-only",
		_expectedCode:    0,
		_expectedContent: defaultsOnly,

		args:     []string{"-repos", registryPath},
		envFile:  unregisteredPath,
		readPath: unregisteredPath,
		stdin:    provideEnvelope(t, unregistered, eventSessionStart, testSessionId),
	})

	// unreadable-registry-exit-zero
	unreadableRegistryEnvPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "unreadable-registry-exit-zero",
		_expectedCode:    0,
		_expectedContent: defaultsOnly,

		args:     []string{"-env-file", unreadableRegistryEnvPath, "-repos", unreadableRegistryPath},
		envFile:  "",
		readPath: unreadableRegistryEnvPath,
		stdin:    provideEnvelope(t, registered, eventSessionStart, testSessionId),
	})

	// switch-off-writes-nothing
	switchOffEnvPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "switch-off-writes-nothing",
		_expectedCode:    0,
		_expectedContent: "",

		args:     []string{"-env-file", switchOffEnvPath, "-repos", registryPath},
		envFile:  "",
		readPath: switchOffEnvPath,
		stdin:    provideEnvelope(t, registered, eventSessionStart, switchOffSessionId),
	})

	// append-keeps-earlier-lines
	seededPath := provideFile(t, earlierLine)
	tests = append(tests, &testCase{
		_id:              "append-keeps-earlier-lines",
		_expectedCode:    0,
		_expectedContent: earlierLine + rowOverDefaults,

		args:     []string{"-env-file", seededPath, "-repos", registryPath},
		envFile:  "",
		readPath: seededPath,
		stdin:    provideEnvelope(t, registered, eventCwdChanged, testSessionId),
	})

	// sessionstart-binds-global-default
	globalHome := t.TempDir()
	globalDefaults := map[string]string{"check": "make check"}
	provideRecord(t, globalHome, globalDefaults, nil)
	globalVariable := &skillenv.Entry{Example: "make audit", Interface: "check", Name: "SKILL_FIMPLEMENT_CHECK", Required: true, Scope: "repo", Summary: "Runs the repo's check."}
	globalDocument := &skillenv.Document{Schema: skillenv.DocumentSchema, Skill: "fimplement", Variables: []*skillenv.Entry{globalVariable}, Version: "1.0.0"}
	provideDocument(t, globalHome, globalDocument)
	globalEnvPath := filepath.Join(t.TempDir(), "env.sh")
	tests = append(tests, &testCase{
		_id:              "sessionstart-binds-global-default",
		_expectedCode:    0,
		_expectedContent: "export SKILL_FIMPLEMENT_CHECK='make check'\n",

		args:     []string{"-claude-home", globalHome, "-env-file", globalEnvPath, "-repos", registryPath},
		envFile:  "",
		readPath: globalEnvPath,
		stdin:    provideEnvelope(t, unregistered, eventSessionStart, testSessionId),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCode, run(test.args, test.envFile, io.Discard, bytes.NewReader(test.stdin), io.Discard))
			assert.Equal(t, test._expectedContent, readEnvFile(t, test.readPath))
		})
	}
}

func TestRun_Check(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCode   int
		_expectedOutput string

		args   []string
		stdin  []byte
		stdout *bytes.Buffer
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	base := t.TempDir()
	registered := filepath.Join(base, "registered")
	unregistered := filepath.Join(base, "unregistered")
	require.NoError(t, os.MkdirAll(filepath.Join(registered, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(unregistered, 0o755))

	bareRegistryPath := provideRegistry(t, repos.Repo{Name: "registered", Path: registered})
	repoEnv := map[string]string{"SKILL_FIMPLEMENT_CHECK": "make test"}
	valueRegistryPath := provideRegistry(t, repos.Repo{Env: repoEnv, Name: "registered", Path: registered})

	checkVariable := &skillenv.Entry{Example: "make audit", Interface: "check", Name: "SKILL_FIMPLEMENT_CHECK", Required: true, Scope: "repo", Summary: "Runs the repo's check."}
	lintVariable := &skillenv.Entry{Example: "make lint", Interface: "lint", Name: "SKILL_FIMPLEMENT_LINT", Required: false, Scope: "repo", Summary: "Runs the repo's linter."}
	requiredDocument := &skillenv.Document{Schema: skillenv.DocumentSchema, Skill: "fimplement", Variables: []*skillenv.Entry{checkVariable}, Version: "1.0.0"}
	mixedDocument := &skillenv.Document{Schema: skillenv.DocumentSchema, Skill: "fimplement", Variables: []*skillenv.Entry{checkVariable, lintVariable}, Version: "1.0.0"}
	skillDefaults := map[string]string{"SKILL_FIMPLEMENT_CHECK": "make audit"}

	slashCommand := &HookInput{CommandName: "fimplement", Cwd: registered, ExpansionType: expansionSlashCommand, HookEventName: eventUserPromptExpansion, SessionId: testSessionId}
	skillCall := &HookInput{Cwd: registered, HookEventName: eventPreToolUse, SessionId: testSessionId, ToolInput: ToolInput{Skill: "fimplement"}}
	unregisteredCommand := &HookInput{CommandName: "fimplement", Cwd: unregistered, ExpansionType: expansionSlashCommand, HookEventName: eventUserPromptExpansion, SessionId: testSessionId}
	pluginCommand := &HookInput{CommandName: "plugin:fimplement", Cwd: registered, ExpansionType: expansionSlashCommand, HookEventName: eventUserPromptExpansion, SessionId: testSessionId}
	mcpPrompt := &HookInput{CommandName: "fimplement", Cwd: registered, ExpansionType: "mcp_prompt", HookEventName: eventUserPromptExpansion, SessionId: testSessionId}
	switchOffCommand := &HookInput{CommandName: "fimplement", Cwd: registered, ExpansionType: expansionSlashCommand, HookEventName: eventUserPromptExpansion, SessionId: switchOffSessionId}

	unsetRegistered := "SKILL_FIMPLEMENT_CHECK is not set for registered — runs the repo's check, e.g. make audit"
	linkRegistered := "Set it here, then start a new session: http://127.0.0.1:6001/scripts/skills/repo/fimplement?repo=registered"
	shellHint := "Exported in your shell already? Send #disable inject:env, then invoke /fimplement again."
	registeredReason := unsetRegistered + "\n" + linkRegistered + "\n" + shellHint

	tests := make([]*testCase, 0)

	// expansion-required-unset-blocks
	blockHome := t.TempDir()
	provideRecord(t, blockHome, nil, nil)
	provideDocument(t, blockHome, requiredDocument)
	blockOutput := &HookOutput{Decision: decisionBlock, Reason: registeredReason}
	tests = append(tests, &testCase{
		_id:             "expansion-required-unset-blocks",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, blockOutput),

		args:   []string{"-claude-home", blockHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, slashCommand),
		stdout: &bytes.Buffer{},
	})

	// pretooluse-required-unset-denies
	denyHome := t.TempDir()
	provideRecord(t, denyHome, nil, nil)
	provideDocument(t, denyHome, requiredDocument)
	denySpecific := &HookSpecificOutput{HookEventName: eventPreToolUse, PermissionDecision: decisionDeny, PermissionDecisionReason: registeredReason}
	denyOutput := &HookOutput{HookSpecificOutput: denySpecific}
	tests = append(tests, &testCase{
		_id:             "pretooluse-required-unset-denies",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, denyOutput),

		args:   []string{"-claude-home", denyHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, skillCall),
		stdout: &bytes.Buffer{},
	})

	// repo-value-adds-context
	repoHome := t.TempDir()
	provideRecord(t, repoHome, nil, skillDefaults)
	provideDocument(t, repoHome, requiredDocument)
	repoSpecific := &HookSpecificOutput{AdditionalContext: `skillenv: SKILL_FIMPLEMENT_CHECK = "make test" (repo registered)`, HookEventName: eventUserPromptExpansion}
	repoOutput := &HookOutput{HookSpecificOutput: repoSpecific}
	tests = append(tests, &testCase{
		_id:             "repo-value-adds-context",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, repoOutput),

		args:   []string{"-claude-home", repoHome, "-repos", valueRegistryPath},
		stdin:  provideCheckEnvelope(t, slashCommand),
		stdout: &bytes.Buffer{},
	})

	// skill-default-adds-context
	skillDefaultHome := t.TempDir()
	provideRecord(t, skillDefaultHome, nil, skillDefaults)
	provideDocument(t, skillDefaultHome, requiredDocument)
	skillDefaultSpecific := &HookSpecificOutput{AdditionalContext: `skillenv: SKILL_FIMPLEMENT_CHECK = "make audit" (skill default)`, HookEventName: eventPreToolUse}
	skillDefaultOutput := &HookOutput{HookSpecificOutput: skillDefaultSpecific}
	tests = append(tests, &testCase{
		_id:             "skill-default-adds-context",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, skillDefaultOutput),

		args:   []string{"-claude-home", skillDefaultHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, skillCall),
		stdout: &bytes.Buffer{},
	})

	// global-default-adds-context
	globalHome := t.TempDir()
	globalDefaults := map[string]string{"check": "make check"}
	provideRecord(t, globalHome, globalDefaults, nil)
	provideDocument(t, globalHome, requiredDocument)
	globalSpecific := &HookSpecificOutput{AdditionalContext: `skillenv: SKILL_FIMPLEMENT_CHECK = "make check" (global default)`, HookEventName: eventUserPromptExpansion}
	globalOutput := &HookOutput{HookSpecificOutput: globalSpecific}
	tests = append(tests, &testCase{
		_id:             "global-default-adds-context",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, globalOutput),

		args:   []string{"-claude-home", globalHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, slashCommand),
		stdout: &bytes.Buffer{},
	})

	// optional-unset-context-line
	optionalHome := t.TempDir()
	provideRecord(t, optionalHome, nil, skillDefaults)
	provideDocument(t, optionalHome, mixedDocument)
	optionalContext := "skillenv: SKILL_FIMPLEMENT_CHECK = \"make audit\" (skill default)\nskillenv: SKILL_FIMPLEMENT_LINT not set — optional, its steps are left out"
	optionalSpecific := &HookSpecificOutput{AdditionalContext: optionalContext, HookEventName: eventUserPromptExpansion}
	optionalOutput := &HookOutput{HookSpecificOutput: optionalSpecific}
	tests = append(tests, &testCase{
		_id:             "optional-unset-context-line",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, optionalOutput),

		args:   []string{"-claude-home", optionalHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, slashCommand),
		stdout: &bytes.Buffer{},
	})

	// unregistered-cwd-links-skill-page
	unregisteredHome := t.TempDir()
	provideRecord(t, unregisteredHome, nil, nil)
	provideDocument(t, unregisteredHome, requiredDocument)
	unregisteredReason := "SKILL_FIMPLEMENT_CHECK is not set (this repo is not registered) — runs the repo's check, e.g. make audit\n" +
		"Set it here, then start a new session: http://127.0.0.1:6001/scripts/skills/repo/fimplement\n" +
		shellHint
	unregisteredOutput := &HookOutput{Decision: decisionBlock, Reason: unregisteredReason}
	tests = append(tests, &testCase{
		_id:             "unregistered-cwd-links-skill-page",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, unregisteredOutput),

		args:   []string{"-claude-home", unregisteredHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, unregisteredCommand),
		stdout: &bytes.Buffer{},
	})

	// unreadable-record-export-hint-and-note
	unreadableHome := t.TempDir()
	unreadableRecordPath := install.RecordPath(install.Env{ClaudeHome: unreadableHome, GOOS: runtime.GOOS})
	require.NoError(t, os.MkdirAll(filepath.Dir(unreadableRecordPath), 0o755))
	require.NoError(t, os.WriteFile(unreadableRecordPath, []byte("{not json"), envFileMode))
	_, recordErr := install.LoadRecordAt(unreadableHome)
	require.Error(t, recordErr)
	provideDocument(t, unreadableHome, requiredDocument)
	unreadableReason := unsetRegistered + "\n" +
		"Export it before starting Claude Code: export SKILL_FIMPLEMENT_CHECK=…\n" +
		shellHint + "\n" +
		"Note: " + recordErr.Error()
	unreadableOutput := &HookOutput{Decision: decisionBlock, Reason: unreadableReason}
	tests = append(tests, &testCase{
		_id:             "unreadable-record-export-hint-and-note",
		_expectedCode:   0,
		_expectedOutput: encodeOutput(t, unreadableOutput),

		args:   []string{"-claude-home", unreadableHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, slashCommand),
		stdout: &bytes.Buffer{},
	})

	// no-sidecar-passes
	noSidecarHome := t.TempDir()
	provideRecord(t, noSidecarHome, nil, nil)
	tests = append(tests, &testCase{
		_id:             "no-sidecar-passes",
		_expectedCode:   0,
		_expectedOutput: "",

		args:   []string{"-claude-home", noSidecarHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, slashCommand),
		stdout: &bytes.Buffer{},
	})

	// plugin-name-passes
	pluginHome := t.TempDir()
	provideRecord(t, pluginHome, nil, nil)
	provideDocument(t, pluginHome, requiredDocument)
	tests = append(tests, &testCase{
		_id:             "plugin-name-passes",
		_expectedCode:   0,
		_expectedOutput: "",

		args:   []string{"-claude-home", pluginHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, pluginCommand),
		stdout: &bytes.Buffer{},
	})

	// mcp-prompt-passes
	mcpHome := t.TempDir()
	provideRecord(t, mcpHome, nil, nil)
	provideDocument(t, mcpHome, requiredDocument)
	tests = append(tests, &testCase{
		_id:             "mcp-prompt-passes",
		_expectedCode:   0,
		_expectedOutput: "",

		args:   []string{"-claude-home", mcpHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, mcpPrompt),
		stdout: &bytes.Buffer{},
	})

	// switch-off-passes
	switchOffHome := t.TempDir()
	provideRecord(t, switchOffHome, nil, nil)
	provideDocument(t, switchOffHome, requiredDocument)
	provideSwitchOff(t, switchOffHome, switchOffSessionId)
	tests = append(tests, &testCase{
		_id:             "switch-off-passes",
		_expectedCode:   0,
		_expectedOutput: "",

		args:   []string{"-claude-home", switchOffHome, "-repos", bareRegistryPath},
		stdin:  provideCheckEnvelope(t, switchOffCommand),
		stdout: &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCode, run(test.args, "", io.Discard, bytes.NewReader(test.stdin), test.stdout))
			assert.Equal(t, test._expectedOutput, test.stdout.String())
		})
	}
}
