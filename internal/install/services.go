package install

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	ServerAgentLabel     = "com.smine.configserver"
	agentWaitAttempts    = 20
	agentWaitInterval    = 250 * time.Millisecond
	ccusageBinary        = "ccusage"
	goosDarwin           = "darwin"
	HeartbeatAgentLabel  = "com.smine.heartbeat"
	nightlyRoutineName   = "smine-nightly"
	heartbeatAgentDetail = "~/Library/LaunchAgents/com.smine.heartbeat.plist"
	heartbeatTaskDetail  = `logon task \smine\` + HeartbeatAgentLabel
	serverAgentDetail    = "~/Library/LaunchAgents/com.smine.configserver.plist"
	skillMarkerFile      = ".smine-deployed"
)

// ccusageDirs are the Homebrew bin dirs a launchd PATH usually lacks.
var ccusageDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

// The scheduler calls the routine jobs make, swapped by tests.
var (
	isRoutineLoaded = routines.IsLoaded
	startRoutine    = routines.Start
	stopRoutine     = routines.Stop
)

func agentPlist(env Env, label string) string {
	return filepath.Join(env.Home, "Library", "LaunchAgents", label+".plist")
}

func alwaysPackExtra() extra {
	return extra{detail: "~/.claude/context/global/always-context.md", install: installAlwaysPack, isDue: isAlwaysPackDue, remove: removeAlwaysPack}
}

func artifactsDirExtra() extra {
	return extra{detail: "~/.cache/claude-routine/artifacts", install: installArtifactsDir, isDue: isArtifactsDirDue}
}

func artifactsDir(env Env) string {
	return filepath.Join(env.Home, ".cache", "claude-routine", "artifacts")
}

func baseSettingsExtra() extra {
	return extra{detail: "~/.claude/settings.json base keys", install: installBaseSettings, isDue: isBaseSettingsDue}
}

// codexConfig is settings/codex/config.toml with its markers expanded.
func codexConfig(env Env) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(env.RepoDir, "settings", "codex", "config.toml"))
	if err != nil {
		return nil, fmt.Errorf("codexConfig: %w", err)
	}
	return env.markers().Expand(data), nil
}

func codexConfigExtra() extra {
	target := func(env Env) string { return filepath.Join(env.Home, ".codex", "config.toml") }
	return extra{
		detail: "~/.codex/config.toml",
		install: func(ctx context.Context, env Env) error {
			wanted, err := codexConfig(env)
			if err != nil {
				return err
			}
			current, err := os.ReadFile(target(env))
			if err == nil && bytes.Equal(current, wanted) {
				return env.Trash.RecordOwned(target(env), wanted)
			}
			entry := &trash.Entry{Action: "apply config-server", Reason: "codex config rewritten"}
			if _, err := env.Trash.Keep(target(env), entry); err != nil {
				return fmt.Errorf("codexConfigExtra: %w", err)
			}
			if err := fsx.WriteFileAtomic(wanted, target(env), 0o644); err != nil {
				return fmt.Errorf("codexConfigExtra: %w", err)
			}
			return env.Trash.RecordOwned(target(env), wanted)
		},
		isDue: func(ctx context.Context, env Env, isSelected bool) (bool, error) {
			if !isSelected {
				return false, nil
			}
			wanted, err := codexConfig(env)
			if err != nil {
				return false, err
			}
			return isGuardedStale(env, target(env), wanted)
		},
	}
}

// syncBase sets the base keys of settings.json to the template's values and reports whether any differed.
func syncBase(env Env, isWrite bool) (bool, error) {
	files, err := loadSettingsFiles(env)
	if err != nil {
		return false, err
	}

	base, err := config.LoadBaseSettings(settingsDir(env), env.markers())
	if err != nil {
		return false, fmt.Errorf("syncBase: %w", err)
	}

	changed, err := applyBase(files.settings.Doc(), base.Doc())
	if err != nil || len(changed) == 0 || !isWrite {
		return len(changed) > 0, err
	}
	files.isSettingsDirty = true
	return true, files.save(env)
}

func hasDeployedSkills(env Env) (bool, error) {
	markers, err := filepath.Glob(filepath.Join(env.ClaudeHome, "skills", "*", skillMarkerFile))
	if err != nil {
		return false, fmt.Errorf("hasDeployedSkills: %w", err)
	}
	return len(markers) > 0, nil
}

func heartbeatAgentExtra() extra {
	return extra{detail: heartbeatServiceDetail(), install: installHeartbeatService, isDue: isHeartbeatServiceDue, remove: removeHeartbeatService}
}

// heartbeatServiceDetail names the daemon's service as its step shows it: the LaunchAgent plist, on Windows the logon task.
func heartbeatServiceDetail() string {
	if runtime.GOOS == goosWindows {
		return heartbeatTaskDetail
	}
	return heartbeatAgentDetail
}

// IsDaemonRestart reports the step that restarted a daemon on the spot, whichever OS wrote it: the heartbeat service, rewritten and started again; the config server's step only schedules its restart.
func IsDaemonRestart(step Step) bool {
	return step.Action == ActionInstall && (step.Detail == heartbeatAgentDetail || step.Detail == heartbeatTaskDetail)
}

func heartbeatClaudeHome(env Env) string {
	if env.Values.PeekClaudeHome != "" {
		return env.Values.PeekClaudeHome
	}
	return env.ClaudeHome
}

// heartbeatLogPath is the daemon's log on windows, beside the routine logs Task Scheduler cannot capture itself.
func heartbeatLogPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-routine", "logs", "heartbeat.log")
}

func heartbeatPlist(env Env) ([]byte, error) {
	record, err := LoadCurrent(env)
	if err != nil {
		return nil, fmt.Errorf("heartbeatPlist: %w", err)
	}

	numberOfFiles, err := Resolve(record, components.Heartbeat, "number_of_files")
	if err != nil {
		return nil, fmt.Errorf("heartbeatPlist: %w", err)
	}
	return renderTemplate(
		filepath.Join(env.RepoDir, "cmd", "heartbeat", HeartbeatAgentLabel+".plist.template"),
		"__REPO_DIR__", env.MainRepoDir,
		"__HEARTBEAT_PORT__", env.Values.HeartbeatPort,
		"__NUMBER_OF_FILES__", strconv.Itoa(numberOfFiles.Int),
		"__CLAUDE_HOME__", heartbeatClaudeHome(env),
		"__HOME__", env.Home,
	)
}

// heartbeatTaskXML renders the logon task: the daemon from the main checkout's bin, the same flags the plist passes, plus the log file.
func heartbeatTaskXML(env Env) string {
	command := filepath.Join(env.MainRepoDir, "bin", "heartbeat.exe")
	claudeHome := heartbeatClaudeHome(env)
	arguments := fmt.Sprintf(`--addr 127.0.0.1:%s --claude-home "%s" --log-file "%s" serve --repos "%s" --config "%s"`,
		env.Values.HeartbeatPort,
		claudeHome,
		heartbeatLogPath(),
		filepath.Join(env.Home, ".claude", "smine", "repos.json"),
		filepath.Join(claudeHome, "heartbeat", "config.json"),
	)
	return routines.LogonTaskXML(arguments, command, env.MainRepoDir)
}

func installAlwaysPack(ctx context.Context, env Env) error {
	packFile, err := contextdocs.AlwaysPackFile()
	if err != nil {
		return fmt.Errorf("installAlwaysPack: %w", err)
	}
	if _, err := contextdocs.WriteAlwaysPack(filepath.Join(env.RepoDir, "context"), packFile, env.Trash); err != nil {
		return fmt.Errorf("installAlwaysPack: %w", err)
	}
	return nil
}

func installArtifactsDir(ctx context.Context, env Env) error {
	if err := os.MkdirAll(artifactsDir(env), 0o755); err != nil {
		return fmt.Errorf("installArtifactsDir: %w", err)
	}
	return nil
}

func installBaseSettings(ctx context.Context, env Env) error {
	_, err := syncBase(env, true)
	return err
}

func installHeartbeatAgent(ctx context.Context, env Env) error {
	plist, err := heartbeatPlist(env)
	if err != nil {
		return err
	}
	entry := &trash.Entry{Action: "apply heartbeat", Reason: "LaunchAgent rewritten"}
	if _, err := env.Trash.Keep(HeartbeatAgentPlist(env), entry); err != nil {
		return fmt.Errorf("installHeartbeatAgent: %w", err)
	}
	if err := fsx.WriteFileAtomic(plist, HeartbeatAgentPlist(env), 0o644); err != nil {
		return fmt.Errorf("installHeartbeatAgent: %w", err)
	}
	return RestartAgent(ctx, HeartbeatAgentLabel, HeartbeatAgentPlist(env))
}

func installHeartbeatService(ctx context.Context, env Env) error {
	if env.GOOS == goosWindows {
		return installHeartbeatTask(ctx, env)
	}
	return installHeartbeatAgent(ctx, env)
}

// installHeartbeatTask stops a running instance first: Register-ScheduledTask -Force replaces the definition, not the process.
func installHeartbeatTask(ctx context.Context, env Env) error {
	if _, err := routines.StopTask(ctx, HeartbeatAgentLabel); err != nil {
		return fmt.Errorf("installHeartbeatTask: %w", err)
	}
	if output, err := routines.RegisterLogonTask(ctx, HeartbeatAgentLabel, heartbeatTaskXML(env)); err != nil {
		return fmt.Errorf("installHeartbeatTask: %s: %w", output, err)
	}
	return nil
}

func installPeekBinary(ctx context.Context, env Env) error {
	version, err := peekVersion(env)
	if err != nil {
		return err
	}
	if err := peek.UpdateBinary(ctx, peekBinaryPath(env), version); err != nil {
		return fmt.Errorf("installPeekBinary: %w", err)
	}
	return nil
}

func isAlwaysPackDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	packFile, err := contextdocs.AlwaysPackFile()
	if err != nil {
		return false, fmt.Errorf("isAlwaysPackDue: %w", err)
	}

	current, readErr := os.ReadFile(packFile)
	isPresent := readErr == nil
	if !isSelected {
		return isPresent, nil
	}

	wanted, err := contextdocs.PackAlwaysDir(filepath.Join(env.RepoDir, "context"))
	if err != nil {
		return false, fmt.Errorf("isAlwaysPackDue: %w", err)
	}
	isStale := isPresent && string(current) != contextdocs.AlwaysPackContent(wanted)
	isMissing := !isPresent && wanted != ""
	return isStale || isMissing, nil
}

func isArtifactsDirDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	_, err := os.Stat(artifactsDir(env))
	return isSelected && err != nil, nil
}

func isBaseSettingsDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	if !isSelected {
		return false, nil
	}
	return syncBase(env, false)
}

// isFileStale reports a missing target or one whose bytes differ.
func isFileStale(path string, wanted []byte) bool {
	current, err := os.ReadFile(path)
	return err != nil || !bytes.Equal(current, wanted)
}

// isGuardedStale: absent → due; present and unchanged since smine wrote it and different → due; present and the user's → never due (master C5).
func isGuardedStale(env Env, path string, wanted []byte) (bool, error) {
	current, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("isGuardedStale: %w", err)
	}
	isUnchanged, err := env.Trash.IsUnchanged(path)
	if err != nil {
		return false, fmt.Errorf("isGuardedStale: %w", err)
	}
	if bytes.Equal(current, wanted) {
		return !isUnchanged, nil
	}
	return isUnchanged, nil
}

func isHeartbeatAgentDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	if env.GOOS != goosDarwin {
		return false, nil
	}

	path := HeartbeatAgentPlist(env)
	if !isSelected {
		_, err := os.Stat(path)
		return err == nil, nil
	}

	wanted, err := heartbeatPlist(env)
	if err != nil {
		return false, err
	}
	if isFileStale(path, wanted) {
		return true, nil
	}

	record, err := LoadCurrent(env)
	if err != nil {
		return false, fmt.Errorf("isHeartbeatAgentDue: %w", err)
	}
	rendered, err := RenderDocument(env, record, components.SinkHeartbeatConfig)
	if err != nil {
		return false, fmt.Errorf("isHeartbeatAgentDue: %w", err)
	}
	isRestartDue, err := restartKeysChanged(env, rendered)
	if err != nil || isRestartDue {
		return isRestartDue, err
	}

	isLoaded, err := routines.IsLoaded(ctx, HeartbeatAgentLabel)
	return !isLoaded, err
}

func isHeartbeatServiceDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	if env.GOOS == goosWindows {
		return isHeartbeatTaskDue(ctx, env, isSelected)
	}
	return isHeartbeatAgentDue(ctx, env, isSelected)
}

// isHeartbeatTaskDue mirrors the darwin rule: removed state due when the task exists; selected due when it is missing or a restart-class key changed.
func isHeartbeatTaskDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	isLoaded, err := routines.IsLoaded(ctx, HeartbeatAgentLabel)
	if err != nil {
		return false, fmt.Errorf("isHeartbeatTaskDue: %w", err)
	}
	if !isSelected {
		return isLoaded, nil
	}
	if !isLoaded {
		return true, nil
	}

	record, err := LoadCurrent(env)
	if err != nil {
		return false, fmt.Errorf("isHeartbeatTaskDue: %w", err)
	}
	rendered, err := RenderDocument(env, record, components.SinkHeartbeatConfig)
	if err != nil {
		return false, fmt.Errorf("isHeartbeatTaskDue: %w", err)
	}
	return restartKeysChanged(env, rendered)
}

// isPeekBinaryDue: Windows ships peek-mcp.exe in its payload (install_windows.go).
func isPeekBinaryDue(ctx context.Context, env Env, isSelected bool) (bool, error) {
	if env.GOOS == goosWindows {
		return false, nil
	}
	if !isSelected {
		_, err := os.Stat(peekBinaryPath(env))
		return err == nil, nil
	}

	version, err := peekVersion(env)
	if err != nil {
		return false, err
	}
	installed, err := peek.InstalledVersion(ctx, peekBinaryPath(env))
	return err != nil || !strings.Contains(installed, version), nil
}

func isServerAgentStep(step Step) bool {
	return step.Component == components.ConfigServer && step.Detail == serverAgentDetail
}

func loadedRoutines(ctx context.Context, env Env) ([]string, error) {
	list, err := routines.Scan(routinesDir(env))
	if err != nil {
		return nil, fmt.Errorf("loadedRoutines: %w", err)
	}

	var loaded []string
	for _, routine := range list {
		isLoaded, err := isRoutineLoaded(ctx, routine.Label)
		if err == nil && isLoaded {
			loaded = append(loaded, routine.Label)
		}
	}
	return loaded, nil
}

// nightlyJobExtra only stops the nightly when Mining is turned off; loading it is the config server's boot sync, which honours an operator's disable.
func nightlyJobExtra() extra {
	return extra{
		detail:  "routine smine-nightly",
		install: startSuspendedRoutines,
		isDue: func(ctx context.Context, env Env, isSelected bool) (bool, error) {
			routine, err := nightlyRoutine(env)
			if err != nil || routine == nil {
				return false, err
			}
			if isSelected {
				record, err := LoadCurrent(env)
				return slices.Contains(record.Suspended.Routines, routine.Label), err
			}

			return isRoutineLoaded(ctx, routine.Label)
		},
		remove: func(ctx context.Context, env Env) error {
			routine, err := nightlyRoutine(env)
			if err != nil {
				return err
			}
			if routine == nil {
				return nil
			}
			return suspendRoutines(ctx, env, []string{routine.Label})
		},
	}
}

// nightlyRoutine is the checkout's smine-nightly routine, nil when absent or broken.
func nightlyRoutine(env Env) (*routines.Routine, error) {
	list, err := routines.Scan(routinesDir(env))
	if err != nil {
		return nil, fmt.Errorf("nightlyRoutine: %w", err)
	}

	index := slices.IndexFunc(list, func(routine routines.Routine) bool {
		return routine.Name == nightlyRoutineName && routine.LoadError == ""
	})
	if index < 0 {
		return nil, nil
	}
	return &list[index], nil
}

func peekBinaryExtra() extra {
	return extra{detail: "~/go/bin/peek-mcp", install: installPeekBinary, isDue: isPeekBinaryDue, remove: removePeekBinary}
}

func peekBinaryPath(env Env) string {
	return filepath.Join(env.Home, "go", "bin", "peek-mcp")
}

func peekVersion(env Env) (string, error) {
	data, err := os.ReadFile(filepath.Join(env.RepoDir, "PEEK_MCP_VERSION"))
	if err != nil {
		return "", fmt.Errorf("peekVersion: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func pendingPlists(env Env) ([]string, error) {
	templates, err := filepath.Glob(filepath.Join(routinesDir(env), "*", "*.plist.template"))
	if err != nil {
		return nil, fmt.Errorf("pendingPlists: %w", err)
	}
	return slices.DeleteFunc(templates, func(template string) bool {
		_, err := os.Stat(strings.TrimSuffix(template, ".template"))
		return err == nil
	}), nil
}

// probeCcusage finds the binary on PATH or in the Homebrew dirs launchd lacks, then checks its version.
func probeCcusage(ctx context.Context, env Env) Health {
	fix := "brew install ccusage, or npm install -g ccusage, then reload"
	if env.GOOS == goosWindows {
		fix = "npm install -g ccusage, then reload"
	}
	path, err := exec.LookPath(ccusageBinary)
	if err != nil {
		path = ""
		for _, dir := range ccusageDirs {
			if _, statErr := os.Stat(filepath.Join(dir, ccusageBinary)); statErr == nil {
				path = filepath.Join(dir, ccusageBinary)
			}
		}
	}
	if path == "" {
		return Health{Detail: "ccusage not on PATH", Fix: fix}
	}

	ccusage := &budget.Ccusage{BinaryPath: path}
	if err := ccusage.CheckVersion(ctx); err != nil {
		return Health{Detail: err.Error(), Fix: fix}
	}
	return Health{Detail: path, Ok: true}
}

func probeCredential(ctx context.Context, env Env) Health {
	if _, err := os.Stat(filepath.Join(env.ClaudeHome, "configserver", "credential")); err != nil {
		return Health{Detail: "no access credential", Fix: "run bin/configserver -new-credential"}
	}
	return Health{Detail: "credential present", Ok: true}
}

func probeHeartbeat(ctx context.Context, env Env) Health {
	address := loopbackHost + ":" + env.Values.HeartbeatPort
	heartbeatBin := filepath.Join(env.RepoDir, "bin", "heartbeat")
	fix := "check ~/Library/Logs/smine-heartbeat.err.log, then run bin/configserver -apply"
	if env.GOOS == goosWindows {
		fix = `check %LOCALAPPDATA%\claude-routine\logs\heartbeat.log and the task \smine\` + HeartbeatAgentLabel + ", then run bin/configserver -apply"
	}
	if output, err := shell.Run(ctx, env.RepoDir, heartbeatBin, "--addr", address, "--claude-home", heartbeatClaudeHome(env), "status", "--format", "json"); err != nil {
		// the client's own line (connection refused, timeout, foreign home) says more than the exit status
		return Health{Detail: fmt.Sprintf("not serving %s: %s", address, cmp.Or(strings.TrimSpace(output), err.Error())), Fix: fix}
	}
	return Health{Detail: "serving " + address, Ok: true}
}

func probeNightly(ctx context.Context, env Env) Health {
	routine, err := nightlyRoutine(env)
	if err != nil || routine == nil {
		return Health{Detail: "no smine-nightly routine in the checkout", Fix: fixReapply}
	}

	isLoaded, err := isRoutineLoaded(ctx, routine.Label)
	if err != nil || !isLoaded {
		return Health{Detail: "smine-nightly not loaded", Fix: "start it on the Routines page, or turn on Routines and restart the config server"}
	}
	return Health{Detail: "smine-nightly loaded", Ok: true}
}

func probeToken(ctx context.Context, env Env) Health {
	fix := "run `claude setup-token`, then save the token on the Routines page or in ~/.config/claude-routine/token (0600)"
	tokenDir, err := routines.TokenDir()
	if err != nil {
		return Health{Detail: err.Error(), Fix: fix}
	}

	labels, err := routines.ListTokenLabels(tokenDir)
	if err == nil && len(labels) > 0 {
		return Health{Detail: fmt.Sprintf("%d token label(s)", len(labels)), Ok: true}
	}
	if fsx.HasContent(routines.LegacyTokenPath(tokenDir)) {
		return Health{Detail: "legacy token file", Ok: true}
	}
	return Health{Detail: "no headless token", Fix: fix}
}

func removeAlwaysPack(ctx context.Context, env Env) error {
	packFile, err := contextdocs.AlwaysPackFile()
	if err != nil {
		return fmt.Errorf("removeAlwaysPack: %w", err)
	}
	if err := contextdocs.RemoveAlwaysPack(packFile, env.Trash); err != nil {
		return fmt.Errorf("removeAlwaysPack: %w", err)
	}
	return nil
}

// removeHeartbeatAgent ignores Stop's error since a not-loaded agent is the goal state.
func removeHeartbeatAgent(ctx context.Context, env Env) error {
	_, _ = routines.Stop(ctx, HeartbeatAgentLabel)
	entry := &trash.Entry{Action: "retire heartbeat", Reason: reasonComponentOff}
	if _, err := env.Trash.Move(HeartbeatAgentPlist(env), entry); err != nil {
		return fmt.Errorf("removeHeartbeatAgent: %w", err)
	}
	return nil
}

func removeHeartbeatService(ctx context.Context, env Env) error {
	if env.GOOS != goosWindows {
		return removeHeartbeatAgent(ctx, env)
	}
	_, _ = routines.StopTask(ctx, HeartbeatAgentLabel)
	if _, err := routines.Unregister(ctx, HeartbeatAgentLabel); err != nil {
		return fmt.Errorf("removeHeartbeatService: %w", err)
	}
	return nil
}

// removePeekBinary moves the binary to the trash; peek-mcp is smine's by name, like every binary it installs.
func removePeekBinary(ctx context.Context, env Env) error {
	entry := &trash.Entry{Action: "retire peek", Reason: reasonComponentOff}
	if _, err := env.Trash.Move(peekBinaryPath(env), entry); err != nil {
		return fmt.Errorf("removePeekBinary: %w", err)
	}
	return nil
}

// renderTemplate replaces __KEY__ markers pairwise.
func renderTemplate(path string, pairs ...string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("renderTemplate: %w", err)
	}
	return []byte(strings.NewReplacer(pairs...).Replace(string(data))), nil
}

// routineJobsExtra stops loaded routines when Routines is turned off and records them as suspended, starting those again when it returns; loading the rest is the config server's boot sync.
func routineJobsExtra() extra {
	return extra{
		detail:  "routine jobs",
		install: startSuspendedRoutines,
		isDue: func(ctx context.Context, env Env, isSelected bool) (bool, error) {
			if isSelected {
				record, err := LoadCurrent(env)
				return len(record.Suspended.Routines) > 0, err
			}
			loaded, err := loadedRoutines(ctx, env)
			return len(loaded) > 0, err
		},
		remove: stopLoadedRoutines,
	}
}

// startSuspendedRoutines starts each suspended routine the record lets run — Routines selected, Mining too for the nightly; one that fails or may not run yet stays suspended, one that is gone is dropped.
func startSuspendedRoutines(ctx context.Context, env Env) error {
	list, err := routines.Scan(routinesDir(env))
	if err != nil {
		return fmt.Errorf("startSuspendedRoutines: %w", err)
	}

	record, err := LoadCurrent(env)
	if err != nil {
		return fmt.Errorf("startSuspendedRoutines: %w", err)
	}

	var remaining []string
	var errs []error
	for _, label := range record.Suspended.Routines {
		index := slices.IndexFunc(list, func(routine routines.Routine) bool {
			return routine.Label == label && routine.LoadError == ""
		})
		if index < 0 {
			continue
		}
		isMiningNeeded := list[index].Name == nightlyRoutineName && !slices.Contains(record.Components, components.Mining)
		if !slices.Contains(record.Components, components.Routines) || isMiningNeeded {
			remaining = append(remaining, label)
			continue
		}
		if output, err := startRoutine(ctx, label, list[index].PlistPath); err != nil {
			remaining = append(remaining, label)
			errs = append(errs, fmt.Errorf("%s: %w\n%s", label, err, output))
		}
	}

	record.Suspended.Routines = remaining
	if err := Save(env, record); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("startSuspendedRoutines: %w", err)
	}
	return nil
}

// stopLoadedRoutines suspends every loaded routine.
func stopLoadedRoutines(ctx context.Context, env Env) error {
	loaded, err := loadedRoutines(ctx, env)
	if err != nil {
		return fmt.Errorf("stopLoadedRoutines: %w", err)
	}
	return suspendRoutines(ctx, env, loaded)
}

// suspendRoutines stops the labels and records the stopped ones as suspended, also when a later stop fails.
func suspendRoutines(ctx context.Context, env Env, labels []string) error {
	var stopped []string
	var stopErr error
	for _, label := range labels {
		output, err := stopRoutine(ctx, label)
		if err != nil {
			stopErr = fmt.Errorf("suspendRoutines: %s: %w\n%s", label, err, output)
			break
		}
		stopped = append(stopped, label)
	}

	record, err := LoadCurrent(env)
	if err != nil {
		return errors.Join(stopErr, fmt.Errorf("suspendRoutines: %w", err))
	}
	for _, label := range stopped {
		if !slices.Contains(record.Suspended.Routines, label) {
			record.Suspended.Routines = append(record.Suspended.Routines, label)
		}
	}
	if err := Save(env, record); err != nil {
		return errors.Join(stopErr, fmt.Errorf("suspendRoutines: %w", err))
	}
	return stopErr
}

// routinePlistsExtra renders each routines plist from its template once, an existing plist being operator state.
func routinePlistsExtra() extra {
	return extra{
		detail: "routines/*/*.plist",
		install: func(ctx context.Context, env Env) error {
			pending, err := pendingPlists(env)
			if err != nil {
				return err
			}
			for _, template := range pending {
				plist, err := renderTemplate(template, "__REPO_DIR__", env.MainRepoDir, "__HOME__", env.Home)
				if err != nil {
					return err
				}
				if err := fsx.WriteFileAtomic(plist, strings.TrimSuffix(template, ".template"), 0o644); err != nil {
					return fmt.Errorf("routinePlistsExtra: %w", err)
				}
			}
			return nil
		},
		isDue: func(ctx context.Context, env Env, isSelected bool) (bool, error) {
			if !isSelected {
				return false, nil
			}
			pending, err := pendingPlists(env)
			return len(pending) > 0, err
		},
	}
}

func routinesDir(env Env) string {
	return filepath.Join(env.RepoDir, "routines")
}

// serverAgentExtra writes the config server plist on darwin and leaves the restart to the caller on restart-required (master C3).
func serverAgentExtra(isPeek bool) extra {
	return extra{
		detail: serverAgentDetail,
		install: func(ctx context.Context, env Env) error {
			plist, err := serverAgentPlist(env, isPeek)
			if err != nil {
				return err
			}
			entry := &trash.Entry{Action: "apply config-server", Reason: "LaunchAgent rewritten"}
			if _, err := env.Trash.Keep(ServerAgentPlist(env), entry); err != nil {
				return fmt.Errorf("serverAgentExtra: %w", err)
			}
			return fsx.WriteFileAtomic(plist, ServerAgentPlist(env), 0o644)
		},
		isDue: func(ctx context.Context, env Env, isSelected bool) (bool, error) {
			if !isSelected || env.GOOS != goosDarwin {
				return false, nil
			}
			plist, err := serverAgentPlist(env, isPeek)
			return isFileStale(ServerAgentPlist(env), plist), err
		},
	}
}

// serverAgentPlist renders the config server LaunchAgent, with -peek-port 0 when peek is off.
func serverAgentPlist(env Env, isPeek bool) ([]byte, error) {
	values := env.Values
	peekPort := values.PeekPort
	if !isPeek {
		peekPort = "0"
	}

	var peekHomeArgs strings.Builder
	if values.PeekClaudeHome != "" {
		fmt.Fprintf(&peekHomeArgs, "<string>-claude-home</string><string>%s</string>", values.PeekClaudeHome)
	}
	if values.PeekCodexHome != "" {
		fmt.Fprintf(&peekHomeArgs, "<string>-codex-home</string><string>%s</string>", values.PeekCodexHome)
	}

	return renderTemplate(
		filepath.Join(env.RepoDir, "cmd", "configserver", ServerAgentLabel+".plist.template"),
		"__REPO_DIR__", env.MainRepoDir,
		"__PORT__", values.ConfigServerPort,
		"__PEEK_PORT__", peekPort,
		"__PEEK_CONTROL_PORT__", values.PeekControlPort,
		"__PEEK_HOME_ARGS__", peekHomeArgs.String(),
		"__HOME__", env.Home,
		"__PATH__", values.ServerPath,
		"__INIT_WELCOME__", strconv.FormatBool(values.InitWelcome),
	)
}

// skillSyncExtra deploys the record's skill selection and prunes marked dirs outside it; mode none and removal prune all of them.
func skillSyncExtra(r Record) extra {
	return extra{
		detail: "skills sync (" + r.Skills.Mode + ")",
		install: func(ctx context.Context, env Env) error {
			selection := &skills.Selection{Chosen: r.Skills.Chosen, Mode: r.Skills.Mode}
			return syncSkills(env, selection, r.Components)
		},
		isDue: func(ctx context.Context, env Env, isSelected bool) (bool, error) {
			isDeploying := isSelected && r.Skills.Mode != skills.SelectionModeNone
			if isDeploying {
				return true, nil
			}
			return hasDeployedSkills(env)
		},
		remove: func(ctx context.Context, env Env) error {
			selection := &skills.Selection{Mode: skills.SelectionModeChosen}
			return syncSkills(env, selection, r.Components)
		},
	}
}

func syncSkills(env Env, selection *skills.Selection, componentSet []components.ID) error {
	record, err := LoadCurrent(env)
	if err != nil {
		return fmt.Errorf("syncSkills: %w", err)
	}

	contextDir, err := Resolve(record, components.Context, "context_dir")
	if err != nil {
		return fmt.Errorf("syncSkills: %w", err)
	}

	options := &skills.SyncOptions{
		Components: componentSet,
		ContextDir: contextDir.Text,
		Home:       env.Home,
		Prune:      true,
		RepoDir:    env.RepoDir,
		Selection:  selection,
		Rows:       SkillRows(record),
		Trash:      env.Trash,
	}
	report, err := skills.Sync(options)
	fmt.Fprint(env.Stdout, report)
	if err != nil {
		return fmt.Errorf("syncSkills: %w", err)
	}
	return nil
}

// RestartAgent boots a LaunchAgent out, waits until launchd dropped it and bootstraps it, ignoring Stop's error for a not-loaded agent.
func RestartAgent(ctx context.Context, label, plistPath string) error {
	_, _ = routines.Stop(ctx, label)
	for range agentWaitAttempts {
		isLoaded, err := routines.IsLoaded(ctx, label)
		if err != nil || !isLoaded {
			break
		}
		time.Sleep(agentWaitInterval)
	}

	if output, err := routines.Start(ctx, label, plistPath); err != nil {
		return fmt.Errorf("RestartAgent: %s: %w\n%s", label, err, output)
	}
	return nil
}

// RestartHeartbeatService is what -install runs after its apply: the binaries changed under a loaded daemon.
func RestartHeartbeatService(ctx context.Context, env Env) error {
	if env.GOOS == goosWindows {
		return installHeartbeatTask(ctx, env)
	}
	return RestartAgent(ctx, HeartbeatAgentLabel, HeartbeatAgentPlist(env))
}

func ServerAgentPlist(env Env) string {
	return agentPlist(env, ServerAgentLabel)
}

func HeartbeatAgentPlist(env Env) string {
	return agentPlist(env, HeartbeatAgentLabel)
}
