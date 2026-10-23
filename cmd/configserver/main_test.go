package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	testBaseSettings      = `{"permissions":{"allow":["Read"]}}`
	testComponentFragment = `{"env":{"HOME_DIR":"{{HOME}}"},"permissions":{"allow":["Bash(x)"]}}`
	testSyncSkill         = "sync-skill"
)

type failingWriter struct{}

func (w *failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("failingWriter.Write: Closed")
}

// errorMessage is the error's text, empty for nil.
func errorMessage(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

// isPathPresent reports whether path exists.
func isPathPresent(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isPasswordAccepted reports whether credential admits a request carrying
// password as HTTP Basic; a nil credential admits nothing.
func isPasswordAccepted(credential *server.Credential, password string) bool {
	if credential == nil {
		return false
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetBasicAuth("operator", password)
	recorder := httptest.NewRecorder()
	credential.Require(next).ServeHTTP(recorder, request)
	return recorder.Code == http.StatusNoContent
}

// provideSyncSkill is a small model that passes Check: a routing block, one
// step and a model block.
func provideSyncSkill() *skillgraph.Skill {
	frontmatter := []skillgraph.Field{
		{Key: "name", Value: " " + testSyncSkill},
		{Key: "description", Value: " Sync fixture. Trigger on nothing."},
		{Key: "version", Value: " 1.0"},
		{Key: "format", Value: " 2"},
	}
	routing := &skillgraph.Routing{
		Pipeline:      skillgraph.Pipeline{Standalone: true},
		Preconditions: []string{"a demo request"},
		Purpose:       "demo.",
		Routes:        []skillgraph.Route{{For: "planning", Use: "/fdesign"}},
	}
	step := &skillgraph.Entry{Id: "SKILL-SYNCSKILL-FLOW-001", Role: skillgraph.RoleStep, Statement: "RUN the step.", Title: "Only step"}
	model := &skillgraph.Model{Effort: "high", Reason: "demo reasoning", Tier: "frontier"}
	sections := []*skillgraph.Section{
		{Title: "Routing", Blocks: []skillgraph.Block{{Routing: routing}}},
		{Title: "Flow", Blocks: []skillgraph.Block{{Entry: step}}},
		{Title: "Model", Blocks: []skillgraph.Block{{Model: model}}},
	}
	skill := &skillgraph.Skill{
		Changelog:   []skillgraph.ChangelogEntry{{Date: "2000-01-01", Text: "fixture", Version: "1.0"}},
		Frontmatter: frontmatter,
		Preamble:    "# " + testSyncSkill,
		Sections:    sections,
	}
	return skill
}

// readFileOrEmpty is the file's content, empty when it cannot be read.
func readFileOrEmpty(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(content)
}

// writeSettingsDir writes a settings dir into dir — base.json plus one
// component fragment carrying componentFragment — and returns its path.
func writeSettingsDir(t *testing.T, dir, componentFragment string) string {
	t.Helper()
	settingsDir := filepath.Join(dir, "settings")
	require.NoError(t, os.MkdirAll(filepath.Join(settingsDir, config.ComponentFragmentDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, config.BaseSettingsFile), []byte(testBaseSettings), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, config.ComponentFragmentDir, "x.json"), []byte(componentFragment), 0o644))
	return settingsDir
}

func TestRunMigrateState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.StoreRootEnv, "")
	checkout := filepath.Join(home, "GolandProjects", "x")
	require.NoError(t, os.MkdirAll(checkout, 0o755))
	output, err := shell.Run(context.Background(), checkout, "git", "init", "-q")
	require.NoError(t, err, output)
	output, err = shell.Run(
		context.Background(),
		checkout,
		"git",
		"-c",
		"user.name=t",
		"-c",
		"user.email=t@t",
		"commit",
		"-q",
		"--allow-empty",
		"-m",
		"init",
	)
	require.NoError(t, err, output)
	report := &strings.Builder{}

	err = runMigrateState(context.Background(), checkout, report)

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(report.String()), "\n")
	require.Len(t, lines, 12)
	archive := filepath.Join(home, "GolandProjects", "agent-plans")
	assert.Equal(t, "initialized "+archive+" <- git init", lines[1])
	outcomeCounts := make(map[string]int)
	for _, line := range lines {
		outcomeCounts[strings.Fields(line)[0]]++
	}
	expectedCounts := map[string]int{"absent": 11, "initialized": 1}
	assert.Equal(t, expectedCounts, outcomeCounts)
	assert.DirExists(t, filepath.Join(archive, ".git"))
}

func TestRunNewCredential(t *testing.T) {
	credentialPath := filepath.Join(t.TempDir(), "configserver", "credential")
	output := &strings.Builder{}

	err := runNewCredential(credentialPath, output)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	require.Len(t, lines, 2)
	token := lines[1]
	assert.Equal(t, 1, strings.Count(output.String(), token))

	credential, err := server.LoadCredential(credentialPath)
	require.NoError(t, err)
	assert.True(t, isPasswordAccepted(credential, token))

	// failing-writer
	err = runNewCredential(filepath.Join(t.TempDir(), "credential"), &failingWriter{})
	assert.Error(t, err)
}

func TestPrepareDev(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedSettings  string
		_isTempDirExpected bool
		_shouldPass        bool

		claudeFragmentPath string
		credentialPath     string
		repoDir            string
		settingsPath       string
	}

	// All cases share one TMPDIR that does not exist yet and run in order: the
	// cases failing before prepareDev creates it come first.
	tempDir := filepath.Join(t.TempDir(), "tmp")
	t.Setenv("TMPDIR", tempDir)
	markerHome := t.TempDir()
	t.Setenv("HOME", markerHome)
	tests := make([]*testCase, 0)

	// operator-credential-present
	home := t.TempDir()
	operatorCredentialPath := filepath.Join(home, "credential")
	_, err := server.GenerateCredential(operatorCredentialPath)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                "operator-credential-present",
		_expectedSettings:  "",
		_isTempDirExpected: false,
		_shouldPass:        false,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     operatorCredentialPath,
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
	})

	// credential-path-unreadable-parent
	home = t.TempDir()
	blockerPath := filepath.Join(home, "blocker")
	require.NoError(t, os.WriteFile(blockerPath, []byte("file"), 0o644))
	tests = append(tests, &testCase{
		_id:                "credential-path-unreadable-parent",
		_expectedSettings:  "",
		_isTempDirExpected: false,
		_shouldPass:        false,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     filepath.Join(blockerPath, "credential"),
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
	})

	// empty-home
	home = t.TempDir()
	composedSettings := "{\n  \"permissions\": {\n    \"allow\": [\n      \"Read\",\n      \"Bash(x)\"\n    ]\n  },\n  \"env\": {\n    \"HOME_DIR\": \"" + markerHome + "\"\n  }\n}\n"
	tests = append(tests, &testCase{
		_id:                "empty-home",
		_expectedSettings:  composedSettings,
		_isTempDirExpected: true,
		_shouldPass:        true,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     filepath.Join(home, ".claude", "configserver", "credential"),
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
	})

	// settings-exist
	home = t.TempDir()
	existingSettingsPath := filepath.Join(home, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(existingSettingsPath), 0o755))
	require.NoError(t, os.WriteFile(existingSettingsPath, []byte(`{"existing":true}`), 0o644))
	tests = append(tests, &testCase{
		_id:                "settings-exist",
		_expectedSettings:  `{"existing":true}`,
		_isTempDirExpected: true,
		_shouldPass:        true,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     filepath.Join(home, ".claude", "configserver", "credential"),
		repoDir:            home,
		settingsPath:       existingSettingsPath,
	})

	// fragment-malformed
	home = t.TempDir()
	tests = append(tests, &testCase{
		_id:                "fragment-malformed",
		_expectedSettings:  "",
		_isTempDirExpected: true,
		_shouldPass:        false,

		claudeFragmentPath: writeSettingsDir(t, home, `{"unknown":true}`),
		credentialPath:     filepath.Join(home, ".claude", "configserver", "credential"),
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := prepareDev(test.claudeFragmentPath, test.credentialPath, test.repoDir, test.settingsPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedSettings, readFileOrEmpty(test.settingsPath))
			assert.Equal(t, test._isTempDirExpected, isPathPresent(tempDir))
		})
	}
}

func TestPrepareDev_RecordInvalid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMPDIR", t.TempDir())
	invalidRecord := `{"schema":2,"preset":"full","components":[],"settings":{"heartbeat":{"port":"x"}},"skills":{"mode":"all"}}`
	recordPath := filepath.Join(home, ".claude", "smine", config.StateInstallFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(recordPath), 0o755))
	require.NoError(t, os.WriteFile(recordPath, []byte(invalidRecord), 0o644))
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	err := prepareDev(writeSettingsDir(t, home, testComponentFragment), filepath.Join(home, "credential"), home, settingsPath)

	assert.Error(t, err)
	assert.NoFileExists(t, settingsPath)
}

func TestStartupCredential(t *testing.T) {
	type testCase struct {
		_id                       string
		_expectedErrorText        string
		_isCredentialExpected     bool
		_isCredentialFileExpected bool
		_shouldPass               bool

		claudeFragmentPath string
		credentialPath     string
		isDev              bool
		repoDir            string
		settingsPath       string
		token              string
	}

	t.Setenv("TMPDIR", t.TempDir())
	tests := make([]*testCase, 0)

	// dev
	home := t.TempDir()
	tests = append(tests, &testCase{
		_id:                       "dev",
		_expectedErrorText:        "",
		_isCredentialExpected:     false,
		_isCredentialFileExpected: false,
		_shouldPass:               true,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     filepath.Join(home, ".claude", "configserver", "credential"),
		isDev:              true,
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
		token:              "dev",
	})

	// operator-with-file
	home = t.TempDir()
	operatorCredentialPath := filepath.Join(home, ".claude", "configserver", "credential")
	operatorToken, err := server.GenerateCredential(operatorCredentialPath)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                       "operator-with-file",
		_expectedErrorText:        "",
		_isCredentialExpected:     true,
		_isCredentialFileExpected: true,
		_shouldPass:               true,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     operatorCredentialPath,
		isDev:              false,
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
		token:              operatorToken,
	})

	// operator-without-file
	home = t.TempDir()
	tests = append(tests, &testCase{
		_id:                       "operator-without-file",
		_expectedErrorText:        "-new-credential",
		_isCredentialExpected:     false,
		_isCredentialFileExpected: false,
		_shouldPass:               false,

		claudeFragmentPath: writeSettingsDir(t, home, testComponentFragment),
		credentialPath:     filepath.Join(home, ".claude", "configserver", "credential"),
		isDev:              false,
		repoDir:            home,
		settingsPath:       filepath.Join(home, ".claude", "settings.json"),
		token:              "dev",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			credential, err := startupCredential(test.claudeFragmentPath, test.credentialPath, test.repoDir, test.isDev, test.settingsPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorMessage(err), test._expectedErrorText)
			assert.Equal(t, test._isCredentialExpected, credential != nil)
			assert.Equal(t, test._isCredentialExpected, isPasswordAccepted(credential, test.token))
			assert.Equal(t, test._isCredentialFileExpected, isPathPresent(test.credentialPath))
		})
	}
}

func TestRunSyncSkills(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedOutput string
		_shouldPass     bool

		deployedPath string
		into         string
		isPrune      bool
		variant      string
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	checkout := t.TempDir()
	require.NoError(t, skillgraph.Save(filepath.Join(checkout, "skills", "demo", testSyncSkill), provideSyncSkill()))
	scriptsDir := filepath.Join(checkout, "cmd", "sync")
	claudeSkills := filepath.Join(home, ".claude", "skills")
	env, err := install.NewEnv(checkout, io.Discard)
	require.NoError(t, err)
	tests := make([]*testCase, 0)

	// full-sync-into-temp-home
	tests = append(tests, &testCase{
		_id:             "full-sync-into-temp-home",
		_expectedOutput: "synced to " + claudeSkills,
		_shouldPass:     true,

		deployedPath: filepath.Join(claudeSkills, testSyncSkill, "SKILL.md"),
		into:         "",
		isPrune:      true,
		variant:      "",
	})

	// bad-variant-spec
	into := t.TempDir()
	tests = append(tests, &testCase{
		_id:             "bad-variant-spec",
		_expectedOutput: "using context dir",
		_shouldPass:     false,

		deployedPath: filepath.Join(into, testSyncSkill+"--fast", "SKILL.md"),
		into:         into,
		isPrune:      false,
		variant:      testSyncSkill + "=fast",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			output, err := runSyncSkills(env, test.into, test.isPrune, scriptsDir, test.variant)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, output, test._expectedOutput)
			assert.Equal(t, test._shouldPass, isPathPresent(test.deployedPath))
		})
	}
}

func TestParseComponents(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedComponents []components.ID
		_expectedPreset     string
		_shouldPass         bool

		goos  string
		value string
	}

	tests := make([]*testCase, 0)

	// minimal
	tests = append(tests, &testCase{
		_id:                 "minimal",
		_expectedComponents: components.Minimal(),
		_expectedPreset:     install.PresetMinimal,
		_shouldPass:         true,

		goos:  "linux",
		value: install.PresetMinimal,
	})

	// full
	tests = append(tests, &testCase{
		_id:                 "full",
		_expectedComponents: components.Full("darwin"),
		_expectedPreset:     install.PresetFull,
		_shouldPass:         true,

		goos:  "darwin",
		value: install.PresetFull,
	})

	// list-closed-over-requirements
	tests = append(tests, &testCase{
		_id:                 "list-closed-over-requirements",
		_expectedComponents: []components.ID{components.ConfigServer, components.SessionHooks, components.Context, components.Gate, components.Peek, components.Mining},
		_expectedPreset:     install.PresetCustom,
		_shouldPass:         true,

		goos:  "darwin",
		value: "mining",
	})

	// config-server-added
	tests = append(tests, &testCase{
		_id:                 "config-server-added",
		_expectedComponents: []components.ID{components.ConfigServer, components.Skills, components.Token},
		_expectedPreset:     install.PresetCustom,
		_shouldPass:         true,

		goos:  "linux",
		value: "skills, token",
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:                 "unknown-id",
		_expectedComponents: nil,
		_expectedPreset:     "",
		_shouldPass:         false,

		goos:  "linux",
		value: "skills,bogus",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			preset, ids, err := parseComponents(test.value, test.goos)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedPreset, preset)
			assert.Equal(t, test._expectedComponents, ids)
		})
	}
}

func TestParseSkills(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   install.Selection
		_shouldPass bool

		value string
	}

	tests := make([]*testCase, 0)

	// all
	tests = append(tests, &testCase{
		_id:         "all",
		_expected:   install.Selection{Mode: skills.SelectionModeAll},
		_shouldPass: true,

		value: "all",
	})

	// none
	tests = append(tests, &testCase{
		_id:         "none",
		_expected:   install.Selection{Mode: skills.SelectionModeNone},
		_shouldPass: true,

		value: "none",
	})

	// chosen-leaves
	tests = append(tests, &testCase{
		_id:         "chosen-leaves",
		_expected:   install.Selection{Chosen: []string{"a", "b"}, Mode: skills.SelectionModeChosen},
		_shouldPass: true,

		value: "chosen:a,b",
	})

	// chosen-without-leaves
	tests = append(tests, &testCase{
		_id:         "chosen-without-leaves",
		_expected:   install.Selection{Mode: skills.SelectionModeChosen},
		_shouldPass: true,

		value: "chosen",
	})

	// unknown-mode
	tests = append(tests, &testCase{
		_id:         "unknown-mode",
		_expected:   install.Selection{Mode: "some"},
		_shouldPass: false,

		value: "some",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			selection, err := parseSkills(test.value)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, selection)
		})
	}
}

func TestBuildRecord_KeepsSkillRows(t *testing.T) {
	env := install.Env{ClaudeHome: t.TempDir(), GOOS: runtime.GOOS}
	stored := install.FullRecord(env.GOOS)
	stored.Skills.IsConfirmed = true
	stored.Skills.Providers = map[string]string{"PROVIDER_SMINE_RULES": "my-rules"}
	stored.Skills.Variables = map[string]string{"SKILL_FIMPLEMENT_CHECK": "make audit"}
	require.NoError(t, install.Save(env, stored))
	request := &installRequest{explicitFlags: map[string]bool{}, skillsArg: "chosen:fimplement"}

	record, err := buildRecord(env, request)

	require.NoError(t, err)
	assert.Equal(t, skills.SelectionModeChosen, record.Skills.Mode)
	assert.Equal(t, []string{"fimplement"}, record.Skills.Chosen)
	assert.True(t, record.Skills.IsConfirmed, "-skills without -skills-confirmed keeps the confirmation")
	assert.Equal(t, stored.Skills.Providers, record.Skills.Providers)
	assert.Equal(t, stored.Skills.Variables, record.Skills.Variables, "-skills never drops the record's variable rows")
}

func TestSaveInstallValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	env, err := install.NewEnv(t.TempDir(), io.Discard)
	require.NoError(t, err)
	record := install.FullRecord(env.GOOS)
	heartbeatPort, _ := components.LookupSetting(components.Heartbeat, "port")
	storedPort, err := components.ParseText(&heartbeatPort, "6102")
	require.NoError(t, err)
	require.NoError(t, install.SetOverride(&record, components.Heartbeat, "port", storedPort))
	request := &installRequest{explicitFlags: map[string]bool{"peek-port": true}, heartbeatPort: 6101, peekControlPort: 42000, peekPort: 4300}

	saved, err := saveInstallValues(env, &record, request)

	require.NoError(t, err)
	assert.Equal(t, "6102", saved.Values.HeartbeatPort, "a flag not passed keeps the stored port")
	assert.Equal(t, "4300", saved.Values.PeekPort)
	assert.True(t, install.IsOverridden(record, components.Peek, "port"))
	assert.False(t, install.IsOverridden(record, components.Peek, "control_port"), "a flag not passed writes no override")
}
