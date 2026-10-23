package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skills"
)

// provideApplyRepo fills the temp checkout with what the config server and
// global-instructions components read, and clears PATH so binaries come
// from the prebuilt bin/<name>.exe a Windows payload ships.
func provideApplyRepo(t *testing.T, env Env) {
	t.Helper()
	writeTestFile(t, env.RepoDir, "bin/"+PayloadMarker, "0.0.0\n")
	writeTestFile(t, env.RepoDir, "bin/rules", "rules binary")
	writeTestFile(t, env.RepoDir, "settings/codex/config.toml", "model = \"x\"\n")
	writeTestFile(t, env.RepoDir, "settings/claude_code/AGENTS.md", "# Global\n")
	writeTestFile(t, env.RepoDir, "cmd/configserver/com.smine.configserver.plist.template", "<plist>__PORT__ __HOME__</plist>\n")
}

// readTestFile reads one file the test expects to exist.
func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// customRecord is a custom record over ids with no skills.
func customRecord(ids ...components.ID) Record {
	record := Record{
		Components: ids,
		Preset:     PresetCustom,
		Schema:     recordSchema,
		Skills:     Selection{Mode: skills.SelectionModeNone},
	}
	return record
}

// setText stores a text-form value as an override; a value equal to the default stores nothing.
func setText(record *Record, id components.ID, key, text string) error {
	setting, ok := components.LookupSetting(id, key)
	if !ok {
		return fmt.Errorf("setText: Unknown setting %s.%s", id, key)
	}

	value, err := components.ParseText(&setting, text)
	if err != nil {
		return fmt.Errorf("setText: %w", err)
	}
	return SetOverride(record, id, key, value)
}

// resultActions maps each result to its action, failures to "failed".
func resultActions(results []Result) map[components.ID]string {
	actions := map[components.ID]string{}
	for _, result := range results {
		actions[result.Component] = result.Action
		if result.Err != nil {
			actions[result.Component] = "failed"
		}
	}
	return actions
}

func TestInstallers(t *testing.T) {
	var ids []components.ID
	for _, component := range installers(FullRecord(goosDarwin)) {
		ids = append(ids, component.ID())
	}

	var catalogIds []components.ID
	for _, component := range components.Catalog() {
		catalogIds = append(catalogIds, component.ID)
	}
	assert.Equal(t, catalogIds, ids, "one installer per catalog id, catalog order")
}

func TestApply(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	instructions := filepath.Join(env.ClaudeHome, "AGENTS.md")

	// file-only-component-installed-then-unchanged
	first, err := Apply(ctx, env, customRecord(components.ConfigServer, components.GlobalInstructions))
	require.NoError(t, err)
	assert.Equal(t, ActionRestartRequired, resultActions(first)[components.ConfigServer], "config-server plist change → restart-required")
	assert.Equal(t, ActionInstall, resultActions(first)[components.GlobalInstructions])
	assert.FileExists(t, instructions)
	assert.FileExists(t, filepath.Join(env.Home, "go", "bin", "smine-rules"))

	second, err := Apply(ctx, env, withStoredBase(t, env, customRecord(components.ConfigServer, components.GlobalInstructions)))
	require.NoError(t, err)
	assert.Equal(t, ActionUnchanged, resultActions(second)[components.ConfigServer])
	assert.Equal(t, ActionUnchanged, resultActions(second)[components.GlobalInstructions])
	assert.Len(t, second, len(components.Catalog()))

	// deselected-component-removed
	third, err := Apply(ctx, env, withStoredBase(t, env, customRecord(components.ConfigServer)))
	require.NoError(t, err)
	assert.Equal(t, ActionRemove, resultActions(third)[components.GlobalInstructions])
	assert.NoFileExists(t, instructions)
}

func TestApplyComponent(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	writeTestFile(t, env.RepoDir, "bin/toolpolicy", "toolpolicy binary")
	writeTestFile(t, env.RepoDir, "bin/skillinvokegate", "skillinvokegate binary")
	writeTestFile(t, env.RepoDir, "bin/skillenv", "skillenv binary")
	record := customRecord(components.ConfigServer, components.SessionHooks, components.Gate)
	require.NoError(t, setText(&record, components.Gate, "max_loop_items", "32"))

	// one-component-with-session-hooks
	results, err := ApplyComponent(ctx, env, record, components.Gate)
	require.NoError(t, err)
	assert.Equal(t, map[components.ID]string{components.Gate: ActionInstall, components.SessionHooks: ActionInstall}, resultActions(results))
	hooksConfig := decodeRendered(t, readTestFile(t, filepath.Join(env.ClaudeHome, "hooks", "config.json")))
	var toolpolicy map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(hooksConfig["toolpolicy"], &toolpolicy))
	assert.JSONEq(t, `32`, string(toolpolicy["max_loop_items"]))

	// one-component
	results, err = ApplyComponent(ctx, env, withStoredBase(t, env, record), components.ConfigServer)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// unselected-refused
	_, err = ApplyComponent(ctx, env, withStoredBase(t, env, record), components.Peek)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrRecordChanged)
}

func TestInstallOrder(t *testing.T) {
	all := installers(FullRecord(goosDarwin))

	ordered := installOrder(all)

	ids := make([]components.ID, 0, len(ordered))
	for _, component := range ordered {
		ids = append(ids, component.ID())
	}
	assert.Len(t, ids, len(all))
	assert.Equal(t, components.Skills, ids[len(ids)-1], "skills install after the providers their sync probes")
	assert.Less(t, slices.Index(ids, components.ConfigServer), slices.Index(ids, components.Agentwt), "the rest keeps catalog order")
}

func TestApply_AppliedFlag(t *testing.T) {
	ctx := context.Background()

	// applied-true-after-one-history-copy-per-apply
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	applied := customRecord(components.ConfigServer)
	applied.Applied = true
	require.NoError(t, Save(env, applied))
	before := readTestFile(t, RecordPath(env))
	stored, err := LoadCurrent(env)
	require.NoError(t, err)
	_, err = Apply(ctx, env, stored)
	require.NoError(t, err)
	saved, err := LoadCurrent(env)
	require.NoError(t, err)
	assert.True(t, saved.Applied)
	history, err := History(RecordPath(env))
	require.NoError(t, err)
	require.Len(t, history, 1, "the pending save's state is not copied a second time")
	assert.Equal(t, before, readTestFile(t, history[0]))

	// failed-component-still-applied
	env = newTestEnv(t)
	provideApplyRepo(t, env)
	results, err := Apply(ctx, env, customRecord(components.ConfigServer, components.Peek))
	require.NoError(t, err)
	assert.True(t, Failed(results))
	saved, err = LoadCurrent(env)
	require.NoError(t, err)
	assert.True(t, saved.Applied)
}

func TestApply_UserFileNeverRemoved(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "my own instructions\n")

	results, err := Apply(ctx, env, customRecord(components.ConfigServer))
	require.NoError(t, err)
	assert.Equal(t, ActionUnchanged, resultActions(results)[components.GlobalInstructions])
	assert.FileExists(t, filepath.Join(env.ClaudeHome, "AGENTS.md"))
}

func TestApply_FailingExtra(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)

	results, err := Apply(ctx, env, customRecord(components.ConfigServer, components.Peek))
	require.NoError(t, err)
	assert.Equal(t, "failed", resultActions(results)[components.Peek], "no PEEK_MCP_VERSION in the checkout fails peek only")
	assert.Equal(t, ActionRestartRequired, resultActions(results)[components.ConfigServer])
	assert.True(t, Failed(results))

	// record-saved-before-the-first-step
	saved, isFound, err := Load(env)
	require.NoError(t, err)
	assert.True(t, isFound)
	assert.Equal(t, []components.ID{components.ConfigServer, components.Peek}, saved.Components)
}

func TestStatus(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	env.GOOS = "linux"

	rows := Status(ctx, env, customRecord(components.ConfigServer, components.GlobalInstructions))
	byId := map[components.ID]ComponentStatus{}
	for _, row := range rows {
		byId[row.Component.ID] = row
	}

	// unselected-zero-health
	assert.Equal(t, Health{}, byId[components.Peek].Health)
	assert.False(t, byId[components.Peek].Selected)

	// unavailable-flagged
	assert.False(t, byId[components.Heartbeat].Available)

	// missing-target-not-ok-with-fix
	instructions := byId[components.GlobalInstructions]
	assert.True(t, instructions.Selected)
	assert.False(t, instructions.Health.Ok)
	assert.Equal(t, fixReapply, instructions.Health.Fix)
	assert.Contains(t, instructions.Health.Detail, filepath.Join(env.ClaudeHome, "AGENTS.md"))
	assert.Len(t, rows, len(components.Catalog()))
}

func TestSummary(t *testing.T) {
	results := []Result{
		{Action: ActionInstall, Component: components.Gate},
		{Action: ActionInstall, Component: components.Peek, Err: os.ErrNotExist},
	}
	assert.Equal(t, "gate: install\npeek: install — FAILED: file does not exist\n", Summary(results))
}
