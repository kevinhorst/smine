package routinerun

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/routines"
)

const sandboxIgnore = `routines/*/results.jsonl
routines/*/runs/
routines/*/.lock
routines/*/.style-stamp
routines/*/.cadence-stamp
routines/*/.run-now-prompt
proposals/votes*.jsonl
.cell-pids
/.smine-run/
`

const sandboxPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>Label</key><string>com.test.routine.NAME</string>
	<key>ProgramArguments</key><array><string>/repo/bin/routine</string><string>run</string><string>DIR</string></array>
	<key>EnvironmentVariables</key><dict><key>ROUTINE_TEST_PLIST_KEY</key><string>from-plist</string></dict>
</dict></plist>`

// sandbox is a repo carrying one routine plus an isolated HOME, token, state
// dir, worktree root and steer root, with the fake claude first on PATH.
type sandbox struct {
	home       string
	repo       string
	routineDir string
	stateDir   string
	steerDir   string
	store      string
	wtRoot     string
}

func newSandbox(t *testing.T, name, manifest string) *sandbox {
	t.Helper()
	installFakeClaude(t)
	repo := newTestRepo(t)
	routineDir := filepath.Join(repo, "routines", name)
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "proposals"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "proposals", ".keep"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(sandboxIgnore), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, routines.ManifestFileName), []byte(manifest), 0o644))
	plistContent := strings.NewReplacer("NAME", name, "DIR", routineDir).Replace(sandboxPlist)
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine."+name+".plist"), []byte(plistContent), 0o644))
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-qm", "routine")

	home := t.TempDir()
	installFakeAgentwt(t, home)
	tokenDir := filepath.Join(home, ".config", "claude-routine")
	require.NoError(t, os.MkdirAll(tokenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tokenDir, "token"), []byte("test-token\n"), 0o600))
	claudeJson := `{"mcpServers":{"peek-mcp":{"type":"stdio","command":"peek-mcp","args":["start","--transport=stdio"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claudeJson), 0o600))

	steerRoot := t.TempDir()
	wtRoot := t.TempDir()
	cleared := []string{"CLAUDE_CODE_OAUTH_TOKEN", "DISABLE_AUTOUPDATER", "ROUTINE_TEST_PLIST_KEY"}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if isInheritedRoutineKey(key) {
			cleared = append(cleared, key)
		}
	}
	for _, key := range cleared {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	t.Setenv("HOME", home)
	t.Setenv("ROUTINE_WT_ROOT", wtRoot)
	t.Setenv("ROUTINE_STEER_ROOT", steerRoot)
	t.Setenv("ROUTINE_CHAT_POLL_S", "1")
	t.Setenv("ROUTINE_CHAT_EXIT_GRACE_S", "5")
	t.Setenv("ROUTINE_MAX_OPEN_BRANCHES", "")
	t.Setenv("ROUTINE_CADENCE_DAYS", "")
	t.Setenv("ROUTINE_TIMEOUT_S", "")

	stateDir, err := settings.StateDir("")
	require.NoError(t, err)

	store, err := settings.StoreRoot(repo)
	require.NoError(t, err)
	newStoreArchive(t, store, sandboxStore)

	box := &sandbox{
		home:       home,
		repo:       repo,
		routineDir: routineDir,
		stateDir:   stateDir,
		steerDir:   filepath.Join(steerRoot, name),
		store:      store,
		wtRoot:     wtRoot,
	}
	return box
}

// isInheritedRoutineKey: routine and harness env a routine-launched test run
// inherits (ROUTINE_TOKEN from the installed plist) — blanked so the sandbox
// alone configures the runner.
func isInheritedRoutineKey(key string) bool {
	for _, prefix := range []string{"ROUTINE_", "SMINE_", "CLAUDE_CODE_"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func (s *sandbox) results(t *testing.T) []routines.RunResult {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.routineDir, routines.ResultsFileName))
	require.NoError(t, err)
	var rows []routines.RunResult
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var row routines.RunResult
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		rows = append(rows, row)
	}
	return rows
}

func (s *sandbox) run(t *testing.T) int {
	t.Helper()
	runner, err := NewRunner(s.routineDir)
	require.NoError(t, err)
	return runner.Run(context.Background())
}

// sandboxStore is a minimal valid backlog store carrying B-001.
const sandboxStore = `{
  "archive": {
    "done": [],
    "rejected": []
  },
  "changelog": [
    {
      "date": "2026-09-18",
      "delta": "Initial"
    }
  ],
  "sections": [
    {
      "items": [
        {
          "id": "B-001",
          "added": "2026-09-18",
          "deps": [],
          "description": "Item\nDone when: done.",
          "evidence": [],
          "link": "",
          "pri": "critical",
          "route": "/idea",
          "size": "L",
          "status": "shaped",
          "theme": "quality",
          "title": "Item"
        }
      ],
      "name": "Sprint 1"
    }
  ],
  "updated": "2026-09-18"
}
`

func TestRunnerRunSingle(t *testing.T) {
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/idea go"}`)
	t.Setenv(fakeMarkerEnv, filepath.Join(box.steerDir, routines.RunCompleteFileName))
	t.Setenv(fakeScriptEnv, "/idea=out.txt=done")
	require.NoError(t, os.WriteFile(filepath.Join(box.routineDir, routines.RunNowPromptFileName), []byte("B-001: item\n"), 0o644))

	status := box.run(t)

	require.Equal(t, exitOk, status)
	branch := "claude-routines/demo-" + today()
	rows := box.results(t)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].IsRunning())
	assert.Equal(t, "B-001", rows[0].Item)
	assert.Equal(t, branch, rows[0].Branch)
	assert.Contains(t, rows[1].Result, "/idea go B-001: item When the entire task is complete")
	assert.Equal(t, "session-1", rows[1].SessionId)
	assert.Equal(t, "B-001", rows[1].Item)
	assert.Equal(t, branch, rows[1].Branch)
	assert.Equal(t, "from-plist", os.Getenv("ROUTINE_TEST_PLIST_KEY"))
	assert.Equal(t, "test-token", os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"))
	assert.Equal(t, box.repo, os.Getenv("ROUTINE_PROTECTED_PATHS"))
	assert.NoFileExists(t, filepath.Join(box.routineDir, routines.RunNowPromptFileName))

	files := gitRun(t, box.repo, "show", "--name-only", "--format=", branch)
	assert.Contains(t, files, "out.txt")
	assert.NotContains(t, files, "backlog.json")

	archive := filepath.Dir(box.store)
	storeFile := filepath.Base(box.store) + "/backlog/backlog.json"
	store := gitRun(t, archive, "show", "HEAD:"+storeFile)
	assert.Contains(t, store, `"link": "`+branch+`"`)
	assert.Contains(t, store, `"status": "planning"`)
	assert.Equal(t, storeFile, gitRun(t, archive, "show", "--name-only", "--format=", "HEAD"))
	assert.Contains(t, gitRun(t, archive, "log", "-1", "--format=%s"), "store changes after the demo run")
	assert.Contains(t, gitRun(t, archive, "show", "HEAD~1:"+storeFile), `"status": "shaped"`)
}

func TestClaudeArgs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		cell   *stageCell
		runner *Runner
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeJson := `{"mcpServers":{"peek-mcp":{"type":"stdio","command":"peek-mcp","args":["start"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claudeJson), 0o600))
	registeredMcp := `{"mcpServers":{"peek-mcp":{"args":["start"],"command":"peek-mcp","type":"stdio"}}}`
	cell := &stageCell{skill: "demo"}
	hostManifest := &routines.Manifest{Kind: routines.KindSingle}
	tests := make([]*testCase, 0)

	// knobs-empty-omit-flags
	tests = append(tests, &testCase{
		_id:       "knobs-empty-omit-flags",
		_expected: nil,

		cell:   cell,
		runner: &Runner{config: &Config{}, manifest: hostManifest},
	})

	// knobs-set-pass-flags
	tests = append(tests, &testCase{
		_id:       "knobs-set-pass-flags",
		_expected: []string{"--model", "sonnet", "--permission-mode", "acceptEdits", "--max-budget-usd", "5"},

		cell:   cell,
		runner: &Runner{config: &Config{BudgetUsd: "5", Model: "sonnet", PermissionMode: "acceptEdits"}, manifest: hostManifest},
	})

	// mcp-config-strict
	tests = append(tests, &testCase{
		_id:       "mcp-config-strict",
		_expected: []string{"--mcp-config", `{"mcpServers":{}}`, "--strict-mcp-config"},

		cell:   &stageCell{mcpConfig: `{"mcpServers":{}}`, skill: "demo"},
		runner: &Runner{config: &Config{}, manifest: hostManifest},
	})

	// container-single-gets-registered-mcp
	tests = append(tests, &testCase{
		_id:       "container-single-gets-registered-mcp",
		_expected: []string{"--mcp-config", registeredMcp, "--strict-mcp-config"},

		cell:   cell,
		runner: &Runner{config: &Config{IsContainerActive: true}, manifest: hostManifest},
	})

	// container-nightly-keeps-own-mcp
	tests = append(tests, &testCase{
		_id:       "container-nightly-keeps-own-mcp",
		_expected: []string{"--mcp-config", `{"mcpServers":{"own":{}}}`, "--strict-mcp-config"},

		cell:   &stageCell{mcpConfig: `{"mcpServers":{"own":{}}}`, skill: "demo"},
		runner: &Runner{config: &Config{IsContainerActive: true}, manifest: &routines.Manifest{Kind: routines.KindSmineNightly}},
	})

	// host-single-no-mcp-flags
	tests = append(tests, &testCase{
		_id:       "host-single-no-mcp-flags",
		_expected: nil,

		cell:   cell,
		runner: &Runner{config: &Config{}, manifest: hostManifest},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.runner.claudeArgs(test.cell))
		})
	}
}

// newContainerSandbox is a sandbox whose routine runs in Docker: a deployed
// settings.json to mount, the pinned peek version for the build,
// the fake docker first on PATH; returns the docker argv log.
func newContainerSandbox(t *testing.T) (*sandbox, string) {
	t.Helper()
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo go"}`)
	settingsPath := filepath.Join(box.home, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"env":{"HEARTBEAT_ADDR":"127.0.0.1:6101"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(box.repo, peekVersionFile), []byte("v1.2.14\n"), 0o644))
	t.Setenv(fakeMarkerEnv, filepath.Join(box.steerDir, routines.RunCompleteFileName))
	return box, installFakeDocker(t)
}

// dockerVerbs is the subcommand of each logged docker call, in order.
func dockerVerbs(t *testing.T, logPath string) []string {
	t.Helper()
	var verbs []string
	for line := range strings.Lines(readDockerLog(t, logPath)) {
		verbs = append(verbs, strings.Fields(line)[0])
	}
	return verbs
}

// isSlotFree reports whether container slot 0 can be taken right now.
func isSlotFree(t *testing.T, box *sandbox) bool {
	t.Helper()
	path := filepath.Join(box.home, ".cache", "claude-routine", "container", "slots", "0.lock")
	slot, isHeld, err := acquireLock(path, 0)
	require.NoError(t, err)
	if isHeld {
		slot.Close()
	}
	return isHeld
}

// cancelWhenExists cancels once the hanging fake claude wrote path, so the cancel always lands inside a running cell.
func cancelWhenExists(ctx context.Context, cancel context.CancelFunc, path string) {
	for ctx.Err() == nil {
		if _, err := os.Stat(path); err == nil {
			cancel()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// runRoutine runs the sandbox's routine; a non-empty cancelOn cancels it once that file exists.
func runRoutine(t *testing.T, box *sandbox, cancelOn string) int {
	t.Helper()
	runner, err := NewRunner(box.routineDir)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if cancelOn != "" {
		go cancelWhenExists(ctx, cancel, cancelOn)
	}
	return runner.Run(ctx)
}

func TestRunnerRunContainer(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedIsStarted bool
		_expectedResult    string
		_expectedStatus    int
		_expectedVerbs     []string

		cancelOn string
		env      map[string]string
	}

	cellVerbs := []string{"version", "build", "rm", "run", "exec", "exec", "exec", "exec", "exec", "exec", "run", "rm"}
	hangMarker := filepath.Join(t.TempDir(), "hanging")
	tests := make([]*testCase, 0)

	// single-happy-path
	tests = append(tests, &testCase{
		_id:                "single-happy-path",
		_expectedIsStarted: true,
		_expectedResult:    "/demo go",
		_expectedStatus:    exitOk,
		_expectedVerbs:     cellVerbs,

		env: map[string]string{routines.EnvContainer: "1"},
	})

	// unregistered-mount
	tests = append(tests, &testCase{
		_id:                "unregistered-mount",
		_expectedIsStarted: false,
		_expectedResult:    "container: resolveRepoMounts: Repo winds is not registered",
		_expectedStatus:    exitConfig,
		_expectedVerbs:     nil,

		env: map[string]string{routines.EnvContainer: "1", routines.EnvContainerMounts: "winds:ro"},
	})

	// docker-down
	tests = append(tests, &testCase{
		_id:                "docker-down",
		_expectedIsStarted: true,
		_expectedResult:    "container: buildImage: Docker daemon unreachable",
		_expectedStatus:    exitUnavailable,
		_expectedVerbs:     []string{"version"},

		env: map[string]string{routines.EnvContainer: "1", fakeDockerFailEnv: "version"},
	})

	// build-fails
	tests = append(tests, &testCase{
		_id:                "build-fails",
		_expectedIsStarted: true,
		_expectedResult:    "container: buildImage: docker build failed",
		_expectedStatus:    exitUnavailable,
		_expectedVerbs:     []string{"version", "build"},

		env: map[string]string{routines.EnvContainer: "1", fakeDockerFailEnv: "build"},
	})

	// holder-fails
	tests = append(tests, &testCase{
		_id:                "holder-fails",
		_expectedIsStarted: true,
		_expectedResult:    "container: containerCell.startHolder",
		_expectedStatus:    exitUnavailable,
		_expectedVerbs:     []string{"version", "build", "rm", "run", "rm"},

		env: map[string]string{routines.EnvContainer: "1", fakeDockerFailEnv: "run"},
	})

	// cancelled-run
	tests = append(tests, &testCase{
		_id:                "cancelled-run",
		_expectedIsStarted: true,
		_expectedResult:    "",
		_expectedStatus:    exitCancelled,
		_expectedVerbs:     cellVerbs,

		cancelOn: hangMarker,
		env:      map[string]string{routines.EnvContainer: "1", fakeHangEnv: "1", fakeHangMarkerEnv: hangMarker},
	})

	// host-routine-never-calls-docker
	tests = append(tests, &testCase{
		_id:                "host-routine-never-calls-docker",
		_expectedIsStarted: true,
		_expectedResult:    "/demo go",
		_expectedStatus:    exitOk,
		_expectedVerbs:     nil,

		env: map[string]string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			box, dockerLog := newContainerSandbox(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			status := runRoutine(t, box, test.cancelOn)

			rows := box.results(t)
			assert.Equal(t, test._expectedStatus, status)
			assert.Equal(t, test._expectedStatus, rows[len(rows)-1].ExitStatus)
			assert.Contains(t, rows[len(rows)-1].Result, test._expectedResult)
			assert.Equal(t, test._expectedVerbs, dockerVerbs(t, dockerLog))
			assert.Equal(t, test._expectedIsStarted, rows[0].IsRunning())
			assert.Nil(t, registrationFiles(t, filepath.Join(box.stateDir, registrationsDir)))
			assert.True(t, isSlotFree(t, box))
		})
	}
}

func TestRunner_markSession(t *testing.T) {
	t.Run("before-start-writes-nothing", func(t *testing.T) {
		resultsPath := filepath.Join(t.TempDir(), routines.ResultsFileName)
		runner := &Runner{itemId: "B-007", resultsPath: resultsPath}
		runner.markSession("s-1")
		assert.NoFileExists(t, resultsPath)
	})

	t.Run("supersedes-the-started-row-with-the-session", func(t *testing.T) {
		resultsPath := filepath.Join(t.TempDir(), routines.ResultsFileName)
		runner := &Runner{itemId: "B-007", resultsPath: resultsPath}
		runner.markStarted("claude-routines/backlog-idea-2026-10-10")
		runner.markSession("s-1")

		history, err := routines.History(resultsPath, 10)
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.True(t, history[0].IsRunning())
		assert.Equal(t, "s-1", history[0].SessionId)
		assert.Equal(t, "B-007", history[0].Item)
		assert.Equal(t, "claude-routines/backlog-idea-2026-10-10", history[0].Branch)
	})
}

func TestRunner_prepareImage(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedRows  int
		_expectedVerbs []string
		_shouldPass    bool

		dockerFail string
	}

	tests := make([]*testCase, 0)

	// build-once
	tests = append(tests, &testCase{
		_id:            "build-once",
		_expectedRows:  0,
		_expectedVerbs: []string{"version", "build"},
		_shouldPass:    true,

		dockerFail: "",
	})

	// failure-not-retried
	tests = append(tests, &testCase{
		_id:            "failure-not-retried",
		_expectedRows:  1,
		_expectedVerbs: []string{"version", "build"},
		_shouldPass:    false,

		dockerFail: "build",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			box, dockerLog := newContainerSandbox(t)
			t.Setenv(fakeDockerFailEnv, test.dockerFail)
			runner, err := NewRunner(box.routineDir)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(runner.config.SteerDir, 0o755))

			first := runner.prepareImage(context.Background())
			second := runner.prepareImage(context.Background())
			rows, err := routines.History(runner.resultsPath, 10)

			assert.NoError(t, err)
			assert.Equal(t, test._shouldPass, first == exitOk)
			assert.Equal(t, first, second)
			assert.Equal(t, test._expectedVerbs, dockerVerbs(t, dockerLog))
			assert.Len(t, rows, test._expectedRows)
		})
	}
}

func TestRunner_NightlyRoster(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []routines.RunReportRepo

		runner *Runner
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	settings := `{"permissions":{"additionalDirectories":[]}}`
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(settings), 0o644))

	registered := t.TempDir()
	gitRepo := filepath.Join(registered, "checkout")
	plainDir := filepath.Join(registered, "plain")
	require.NoError(t, os.MkdirAll(filepath.Join(gitRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(plainDir, 0o755))
	registry := `{"repos":[{"name":"alpha","path":"` + gitRepo + `"},{"name":"plain","path":"` + plainDir + `"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(registered, "repos.json"), []byte(registry), 0o644))

	tests := make([]*testCase, 0)

	// git-and-plain-registered
	tests = append(tests, &testCase{
		_id:       "git-and-plain-registered",
		_expected: []routines.RunReportRepo{{Name: "alpha", Path: gitRepo}},

		runner: &Runner{config: &Config{StateDir: registered}},
	})

	// missing-registry
	tests = append(tests, &testCase{
		_id:       "missing-registry",
		_expected: []routines.RunReportRepo{},

		runner: &Runner{config: &Config{StateDir: t.TempDir()}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.runner.nightlyRoster())
			assert.FileExists(t, settingsPath)
		})
	}

	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, settings, string(content))
}

func TestRunnerRunSingleIgnoresInheritedEnv(t *testing.T) {
	t.Setenv("ROUTINE_TOKEN", "nightly-token")
	t.Setenv("SMINE_SUBAGENTS", "8")
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo go"}`)
	t.Setenv(fakeMarkerEnv, filepath.Join(box.steerDir, routines.RunCompleteFileName))

	status := box.run(t)

	assert.Equal(t, exitOk, status)
	assert.Equal(t, "test-token", os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"))
}

func TestRunStageFailureEnvelope(t *testing.T) {
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo"}`)
	t.Setenv(fakeMarkerEnv, filepath.Join(box.steerDir, routines.RunCompleteFileName))
	t.Setenv(fakeEnvelopeEnv, `{"type":"result","subtype":"success","is_error":true,"session_id":"session-1","num_turns":1,"total_cost_usd":0,"result":"API Error: 400"}`)

	status := box.run(t)

	assert.Equal(t, exitSoftware, status)
	rows := box.results(t)
	require.Len(t, rows, 2)
	assert.Equal(t, exitSoftware, rows[1].ExitStatus)
}

func TestRunnerRunMissingToken(t *testing.T) {
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo"}`)
	require.NoError(t, os.Remove(filepath.Join(box.home, ".config", "claude-routine", "token")))

	status := box.run(t)

	assert.Equal(t, exitConfig, status)
	rows := box.results(t)
	require.Len(t, rows, 1)
	assert.Equal(t, exitConfig, rows[0].ExitStatus)
	assert.Contains(t, rows[0].Result, "token file missing or empty")
}

func TestRunnerRunAlreadyRunning(t *testing.T) {
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo"}`)
	lock, isHeld, err := acquireLock(filepath.Join(box.routineDir, ".lock"), 0)
	require.NoError(t, err)
	require.True(t, isHeld)
	defer lock.Close()

	status := box.run(t)

	assert.Equal(t, exitOk, status)
	assert.NoFileExists(t, filepath.Join(box.routineDir, routines.ResultsFileName))
}

func TestRunnerRunCadenceSkip(t *testing.T) {
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo"}`)
	t.Setenv("ROUTINE_CADENCE_DAYS", "3")
	t.Setenv(fakeMarkerEnv, filepath.Join(box.steerDir, routines.RunCompleteFileName))
	require.Equal(t, exitOk, box.run(t))
	require.Len(t, box.results(t), 2)

	status := box.run(t)

	assert.Equal(t, exitOk, status)
	assert.Len(t, box.results(t), 2, "the second run inside the cadence window is skipped")
}

// newBudgetSandbox is a sandbox carrying a budget routine and an empty
// transcript dir, so the archive succeeds and peek (absent from the sandbox
// HOME) is the failing source.
func newBudgetSandbox(t *testing.T) *sandbox {
	t.Helper()
	box := newSandbox(t, "smine-budget", `{"kind":"budget"}`)
	require.NoError(t, os.MkdirAll(filepath.Join(box.home, ".claude", "projects"), 0o755))
	// A port nothing listens on: the run must never reach a live peek.
	env, err := install.NewEnv(box.repo, io.Discard)
	require.NoError(t, err)
	record := install.FullRecord(env.GOOS)
	peekPort, _ := components.LookupSetting(components.Peek, "port")
	value, err := components.ParseText(&peekPort, "1")
	require.NoError(t, err)
	require.NoError(t, install.SetOverride(&record, components.Peek, "port", value))
	require.NoError(t, install.Save(env, record))
	return box
}

func TestRunnerRunBudgetWithoutToken(t *testing.T) {
	box := newBudgetSandbox(t)
	require.NoError(t, os.Remove(filepath.Join(box.home, ".config", "claude-routine", "token")))

	status := box.run(t)

	assert.NotEqual(t, exitConfig, status)
	assert.Empty(t, os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"))
}

func TestRunnerRunBudgetAlreadyRunning(t *testing.T) {
	box := newBudgetSandbox(t)
	lock, isHeld, err := acquireLock(filepath.Join(box.routineDir, ".lock"), 0)
	require.NoError(t, err)
	require.True(t, isHeld)
	defer lock.Close()

	status := box.run(t)

	assert.Equal(t, exitOk, status)
	assert.NoFileExists(t, filepath.Join(box.routineDir, routines.ResultsFileName))
}

func TestRunnerRunBudgetRemovesRunNowPrompt(t *testing.T) {
	box := newBudgetSandbox(t)
	promptPath := filepath.Join(box.routineDir, routines.RunNowPromptFileName)
	require.NoError(t, os.WriteFile(promptPath, []byte("B-001: item\n"), 0o644))

	box.run(t)

	assert.NoFileExists(t, promptPath)
}

func TestRunnerRunBudgetSourceFailure(t *testing.T) {
	box := newBudgetSandbox(t)

	status := box.run(t)

	assert.Equal(t, exitSoftware, status)
	rows := box.results(t)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].IsError)
	assert.Equal(t, exitSoftware, rows[0].ExitStatus)
	assert.Contains(t, rows[0].Result, "Client.Connect")
	assert.FileExists(t, filepath.Join(box.stateDir, "usage", "state.json"))
	assert.True(t, strings.HasPrefix(box.stateDir, box.home))
}

func TestRunnerRunBackstop(t *testing.T) {
	box := newSandbox(t, "demo", `{"kind":"single","prompt":"/demo"}`)
	t.Setenv(fakeHangEnv, "1")
	t.Setenv("ROUTINE_TIMEOUT_S", "2")

	status := box.run(t)

	assert.Equal(t, exitDeadline, status)
	rows := box.results(t)
	last := rows[len(rows)-1]
	assert.Equal(t, exitDeadline, last.ExitStatus)
	assert.Contains(t, last.Result, "run backstop deadline hit")
}

// A launchd job starts with a PATH lacking ~/go/bin; the cells call the smine programs by bare name.
func TestPrepareProcessEnv_SmineBinDir(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "go", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("DISABLE_AUTOUPDATER", "")
	t.Setenv(settings.ProtectedPathsKey, "")

	prepareProcessEnv(t.TempDir())
	assert.Contains(t, filepath.SplitList(os.Getenv("PATH")), binDir, "existing-bin-dir-added")

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "/usr/bin:/bin")
	prepareProcessEnv(t.TempDir())
	assert.NotContains(t, os.Getenv("PATH"), "go/bin", "missing-bin-dir-skipped")
}
