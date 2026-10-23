package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
)

// provideTargetsRepo extends the apply checkout with a base template and the session hooks' and helpers' prebuilt hook binaries.
func provideTargetsRepo(t *testing.T, env Env) {
	t.Helper()
	provideApplyRepo(t, env)
	writeTestFile(t, env.RepoDir, "settings/claude_code/base.json", `{"model": "opus"}`)
	writeTestFile(t, env.RepoDir, "bin/skillenv", "skillenv binary")
	writeTestFile(t, env.RepoDir, "bin/sessiontitle", "sessiontitle binary")
	writeTestFile(t, env.RepoDir, "bin/skillhelp", "skillhelp binary")
	writeTestFile(t, env.RepoDir, "bin/playsound", "playsound binary")
}

// syncReport runs SyncTargets with a fresh report as the env's stdout.
func syncReport(env Env, r Record, targets ...string) (string, error) {
	var report strings.Builder
	env.Stdout = &report
	err := SyncTargets(context.Background(), env, r, targets)
	return report.String(), err
}

func TestApplyHooks_WritesOnlyHookArtifacts(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	steps, err := ApplyHooks(context.Background(), env, customRecord(components.ConfigServer, components.SessionHelpers))
	require.NoError(t, err)

	assert.Len(t, steps, 3, "three hook binaries")
	assert.FileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "sessiontitle"))
	assert.FileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "playsound"))
	assert.NoFileExists(t, filepath.Join(env.Home, "go", "bin", "smine-rules"), "PATH binaries stay with the full apply")
	assert.NoFileExists(t, filepath.Join(env.ClaudeHome, "settings.json"), "settings stay with the settings target")
}

func TestApplyHooks_SkillEnvironmentInstallsSkillenv(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	steps, err := ApplyHooks(context.Background(), env, customRecord(components.ConfigServer, components.SessionHooks, components.SkillEnvironment))
	require.NoError(t, err)

	assert.Len(t, steps, 2, "the rendered hooks config and the skillenv hook binary")
	assert.FileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "skillenv"))
	assert.FileExists(t, filepath.Join(env.ClaudeHome, "hooks", "config.json"))
	assert.NoFileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "sessiontitle"), "session helpers stay unselected")
}

func TestApplyHooks_SessionHooksAloneRendersOnlyTheHooksConfig(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	steps, err := ApplyHooks(context.Background(), env, customRecord(components.ConfigServer, components.SessionHooks))
	require.NoError(t, err)

	assert.Len(t, steps, 1, "the rendered hooks config")
	assert.FileExists(t, filepath.Join(env.ClaudeHome, "hooks", "config.json"))
	assert.NoFileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "skillenv"), "skillenv belongs to skill-environment")
}

func TestApplyHooks_UnselectedComponentUntouched(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	steps, err := ApplyHooks(context.Background(), env, customRecord(components.ConfigServer))
	require.NoError(t, err)

	assert.Empty(t, steps)
	assert.NoFileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "sessiontitle"))
}

func TestApplyHooks_SecondRunUnchanged(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)
	record := customRecord(components.ConfigServer, components.SessionHelpers)
	_, err := ApplyHooks(context.Background(), env, record)
	require.NoError(t, err)

	steps, err := ApplyHooks(context.Background(), env, record)
	require.NoError(t, err)

	assert.Empty(t, steps)
}

func TestSyncTargets_SettingsOnly(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	report, err := syncReport(env, customRecord(components.ConfigServer, components.SessionHelpers), SyncTargetSettings)
	require.NoError(t, err)

	assert.Contains(t, report, "== settings\n")
	assert.NotContains(t, report, "== hooks")
	content, err := os.ReadFile(filepath.Join(env.ClaudeHome, "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `"opus"`)
}

func TestSyncTargets_RunsInTargetOrder(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	report, err := syncReport(env, customRecord(components.ConfigServer, components.SessionHelpers), SyncTargetHooks, SyncTargetSettings)
	require.NoError(t, err)

	settingsAt, hooksAt := strings.Index(report, "== settings"), strings.Index(report, "== hooks")
	assert.GreaterOrEqual(t, settingsAt, 0)
	assert.Greater(t, hooksAt, settingsAt, "settings run before hooks whatever the tick order")
	assert.Contains(t, report, "install "+filepath.Join(env.ClaudeHome, "hooks", "bin", "sessiontitle"))
}

func TestSyncTargets_FailureStopsTheRun(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)
	writeTestFile(t, env.RepoDir, "settings/claude_code/base.json", "{not json")

	report, err := syncReport(env, customRecord(components.ConfigServer, components.SessionHelpers), SyncTargetSettings, SyncTargetHooks)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "SyncTargets: settings")
	assert.NotContains(t, report, "== hooks")
	assert.NoFileExists(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "sessiontitle"))
}

func TestSyncTargets_SkillsComponentNotSelected(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	report, err := syncReport(env, customRecord(components.ConfigServer), SyncTargetSkills)
	require.NoError(t, err)

	assert.Contains(t, report, "skills: component not selected — nothing deployed")
}

func TestSyncTargets_AllRunsApplyAlone(t *testing.T) {
	env := newTestEnv(t)
	provideTargetsRepo(t, env)

	report, err := syncReport(env, customRecord(components.ConfigServer, components.SessionHooks, components.SessionHelpers), SyncTargetAll, SyncTargetSettings)
	require.NoError(t, err)

	assert.Contains(t, report, "== all\n")
	assert.Contains(t, report, "session-helpers: install")
	assert.NotContains(t, report, "== settings")
	assert.FileExists(t, filepath.Join(env.Home, "go", "bin", "smine-rules"), "all is the full apply, PATH binaries included")
}

func TestIsSyncTarget(t *testing.T) {
	assert.True(t, IsSyncTarget(SyncTargetAll))
	assert.True(t, IsSyncTarget(SyncTargetHooks))
	assert.False(t, IsSyncTarget("context"))
}
