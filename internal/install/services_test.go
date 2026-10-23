package install

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/routines"
)

func TestIsAlwaysPackDue(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	packFile, err := contextdocs.AlwaysPackFile()
	require.NoError(t, err)

	// missing-without-entries-not-due
	isDue, err := isAlwaysPackDue(ctx, env, true)
	require.NoError(t, err)
	assert.False(t, isDue)

	// missing-with-entries-due
	writeTestFile(t, env.RepoDir, "context/actions/implementing.md", "**ACTION-IMPL-002** `[prose]` — Always action.\n\n* Reach: always\n")
	isDue, err = isAlwaysPackDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)

	// header-bearing-equal-not-due
	packed, err := contextdocs.PackAlwaysDir(filepath.Join(env.RepoDir, "context"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(packFile), 0o755))
	require.NoError(t, os.WriteFile(packFile, []byte(contextdocs.AlwaysPackContent(packed)), 0o644))
	isDue, err = isAlwaysPackDue(ctx, env, true)
	require.NoError(t, err)
	assert.False(t, isDue)

	// differing-due
	require.NoError(t, os.WriteFile(packFile, []byte(contextdocs.AlwaysPackContent("older pack")), 0o644))
	isDue, err = isAlwaysPackDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)
}

func TestCodexConfigExtra(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	codex := codexConfigExtra()
	target := filepath.Join(env.Home, ".codex", "config.toml")

	// absent-written-and-recorded
	isDue, err := codex.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, codex.install(ctx, env))
	assert.Equal(t, "model = \"x\"\n", string(readTestFile(t, target)))
	isRecorded, err := env.Trash.IsUnchanged(target)
	require.NoError(t, err)
	assert.True(t, isRecorded)

	// recorded-and-stale-rewritten-with-stash
	writeTestFile(t, env.RepoDir, "settings/codex/config.toml", "model = \"y\"\n")
	isDue, err = codex.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, codex.install(ctx, env))
	assert.Equal(t, "model = \"y\"\n", string(readTestFile(t, target)))
	assert.Equal(t, "model = \"x\"\n", stashedBytes(t, env, target))

	// equal-unrecorded-due-and-recorded-without-stash
	require.NoError(t, env.Trash.ForgetOwned(target))
	journalBefore := len(journalLines(t, env.Trash))
	isDue, err = codex.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, codex.install(ctx, env))
	isRecorded, err = env.Trash.IsUnchanged(target)
	require.NoError(t, err)
	assert.True(t, isRecorded)
	assert.Len(t, journalLines(t, env.Trash), journalBefore)
	isDue, err = codex.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.False(t, isDue)

	// foreign-not-due
	writeTestFile(t, env.Home, ".codex/config.toml", "model = \"mine\"\n")
	isDue, err = codex.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.False(t, isDue)
}

func TestRemovePeekBinary(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	binary := peekBinaryPath(env)
	writeTestFile(t, env.Home, "go/bin/peek-mcp", "any peek build")

	require.NoError(t, removePeekBinary(ctx, env))

	assert.NoFileExists(t, binary)
	assert.Equal(t, "any peek build", stashedBytes(t, env, binary))
}

func TestPlistsRenderMainCheckout(t *testing.T) {
	env := newTestEnv(t)
	main := filepath.Join(t.TempDir(), "main")
	worktree := filepath.Join(main, ".claude", "worktrees", "work")
	require.NoError(t, os.MkdirAll(filepath.Join(main, ".git", "worktrees", "work"), 0o755))
	pointer := "gitdir: " + filepath.Join(main, ".git", "worktrees", "work") + "\n"
	writeTestFile(t, worktree, ".git", pointer)
	writeTestFile(t, worktree, "cmd/heartbeat/com.smine.heartbeat.plist.template", "<plist>__REPO_DIR__</plist>\n")
	writeTestFile(t, worktree, "cmd/configserver/com.smine.configserver.plist.template", "<plist>__REPO_DIR__</plist>\n")
	env.RepoDir, env.MainRepoDir = worktree, gitroot.MainRoot(worktree)

	heartbeat, err := heartbeatPlist(env)
	require.NoError(t, err)
	server, err := serverAgentPlist(env, false)
	require.NoError(t, err)

	assert.Equal(t, "<plist>"+main+"</plist>\n", string(heartbeat))
	assert.Equal(t, "<plist>"+main+"</plist>\n", string(server))
}

// provideRoutineJobs writes one valid routine name labelled label under the checkout's routines dir and swaps the scheduler calls for a fake over loaded.
func provideRoutineJobs(t *testing.T, env Env, name, label string, loaded map[string]bool) {
	t.Helper()
	routineDir := filepath.Join(routinesDir(env), name)
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>Label</key><string>` + label + `</string><key>ProgramArguments</key><array><string>/bin/true</string></array></dict></plist>
`
	writeTestFile(t, routineDir, routines.ManifestFileName, `{"kind":"single","prompt":"/demo"}`)
	writeTestFile(t, routineDir, label+".plist", plist)

	isLoaded, start, stop := isRoutineLoaded, startRoutine, stopRoutine
	t.Cleanup(func() { isRoutineLoaded, startRoutine, stopRoutine = isLoaded, start, stop })
	isRoutineLoaded = func(ctx context.Context, label string) (bool, error) { return loaded[label], nil }
	startRoutine = func(ctx context.Context, label, plistPath string) (string, error) {
		loaded[label] = true
		return "", nil
	}
	stopRoutine = func(ctx context.Context, label string) (string, error) {
		loaded[label] = false
		return "", nil
	}
}

func TestRoutineJobsExtra(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	label := "com.test.demo"
	loaded := map[string]bool{label: true}
	provideRoutineJobs(t, env, "demo", label, loaded)
	require.NoError(t, Save(env, customRecord(components.ConfigServer)))
	jobs := routineJobsExtra()

	// removal-suspends-the-loaded-routine
	isDue, err := jobs.isDue(ctx, env, false)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, jobs.remove(ctx, env))
	assert.False(t, loaded[label])
	record, err := LoadCurrent(env)
	require.NoError(t, err)
	assert.Equal(t, []string{label}, record.Suspended.Routines)

	// routines-off-keeps-it-suspended
	require.NoError(t, jobs.install(ctx, env))
	assert.False(t, loaded[label])
	record, err = LoadCurrent(env)
	require.NoError(t, err)
	assert.Equal(t, []string{label}, record.Suspended.Routines)

	// reinstall-starts-it-and-clears-the-list
	selectComponents(t, env, components.ConfigServer, components.Routines, components.Token)
	isDue, err = jobs.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, jobs.install(ctx, env))
	assert.True(t, loaded[label])
	record, err = LoadCurrent(env)
	require.NoError(t, err)
	assert.Empty(t, record.Suspended.Routines)

	// nothing-suspended-not-due
	isDue, err = jobs.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.False(t, isDue)
}

// TestNightlyJobExtra follows a full uninstall, Mining removed before Routines, and the reinstall.
func TestNightlyJobExtra(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	label := "com.smine.nightly"
	loaded := map[string]bool{label: true}
	provideRoutineJobs(t, env, nightlyRoutineName, label, loaded)
	require.NoError(t, Save(env, customRecord(components.ConfigServer)))
	nightly, jobs := nightlyJobExtra(), routineJobsExtra()

	// mining-removal-suspends-the-nightly
	isDue, err := nightly.isDue(ctx, env, false)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, nightly.remove(ctx, env))
	assert.False(t, loaded[label])
	isDue, err = jobs.isDue(ctx, env, false)
	require.NoError(t, err)
	assert.False(t, isDue, "the routines removal finds nothing left loaded")
	record, err := LoadCurrent(env)
	require.NoError(t, err)
	assert.Equal(t, []string{label}, record.Suspended.Routines)

	// routines-without-mining-keeps-it-suspended
	selectComponents(t, env, components.ConfigServer, components.Routines, components.Token)
	require.NoError(t, jobs.install(ctx, env))
	assert.False(t, loaded[label])

	// mining-back-starts-it
	selectComponents(t, env, components.ConfigServer, components.SessionHooks, components.Gate, components.Context, components.Peek, components.Routines, components.Token, components.Mining)
	isDue, err = nightly.isDue(ctx, env, true)
	require.NoError(t, err)
	assert.True(t, isDue)
	require.NoError(t, nightly.install(ctx, env))
	assert.True(t, loaded[label])
	record, err = LoadCurrent(env)
	require.NoError(t, err)
	assert.Empty(t, record.Suspended.Routines)
}

// selectComponents saves the stored record with ids as its components, as an apply's first save does.
func selectComponents(t *testing.T, env Env, ids ...components.ID) {
	t.Helper()
	record, err := LoadCurrent(env)
	require.NoError(t, err)
	record.Components = ids
	require.NoError(t, Save(env, record))
}
