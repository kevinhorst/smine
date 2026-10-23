package routinerun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/backlog"
	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

// Claude --effort levels the stages use.
const (
	effortHigh   = "high"
	effortLow    = "low"
	effortMedium = "medium"
)

const (
	day                = 24 * time.Hour
	groupLockWait      = 2 * time.Hour
	dockerDesktopBin   = "/usr/local/bin" // Docker Desktop's CLI symlinks; launchd's PATH lacks it
	homebrewBin        = "/opt/homebrew/bin"
	homebrewRustupBin  = "/opt/homebrew/opt/rustup/bin"
	mergeResolvePrompt = "/merge-resolve theirs main. Unattended routine run: never ask a question; decide every judgement call yourself. Merge directly on the current branch — do not create a work branch and skip the cleanup step."
)

// probedKnobs are the CLAUDE_CODE_* concurrency knobs an outdated claude silently ignores.
var probedKnobs = []string{"CLAUDE_CODE_MAX_TOOL_USE_CONCURRENCY", "CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS"}

type Runner struct {
	branch       string // the run's dated branch once created
	cellPids     string
	config       *Config
	dir          string
	imageStatus  int // exitOk until a build fails; the build runs once per run
	isImageBuilt bool
	itemId       string // B-NNN from the run-now prompt, "" when not a dispatch
	label        string
	manifest     *routines.Manifest
	name         string
	repoMounts   []*bindMount // resolved once per run, before any container cell
	repoRoot     string
	resultsPath  string
	runNow       string // the one-shot Run-Now prompt, read once at start
	storeRoot    string // plans-archive dir holding the backlog and proposal stores
	worktree     *groupWorktree
}

func NewRunner(routineDir string) (*Runner, error) {
	dir, err := filepath.Abs(routineDir)
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	manifest, err := routines.LoadManifest(dir)
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	label, err := applyPlistEnv(dir)
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	name := filepath.Base(dir)
	runConfig, err := loadConfig(name)
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	repoRoot := filepath.Dir(filepath.Dir(dir))
	storeRoot, err := settings.StoreRoot(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	stateDir, err := settings.StateDir("")
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	groupPath := filepath.Join(runConfig.WorktreeRoot, runConfig.Group)
	worktree := &groupWorktree{
		agentwtPath:     runConfig.AgentwtPath,
		branchPrefix:    runConfig.BranchPrefix,
		maxOpenBranches: runConfig.MaxOpenBranches,
		path:            groupPath,
		repoRoot:        repoRoot,
		routineName:     name,
		trash:           trash.New(stateDir, 0),
	}
	runner := &Runner{
		cellPids:    filepath.Join(groupPath, routines.CellPidsFileName),
		config:      runConfig,
		dir:         dir,
		label:       label,
		manifest:    manifest,
		name:        name,
		repoRoot:    repoRoot,
		resultsPath: filepath.Join(dir, routines.ResultsFileName),
		storeRoot:   storeRoot,
		worktree:    worktree,
	}
	return runner, nil
}

func (r *Runner) Run(ctx context.Context) int {
	closeLogs := redirectRunLogs(r.label)
	defer closeLogs()
	prepareProcessEnv(r.repoRoot)
	if r.manifest.Kind == routines.KindBudget {
		return r.runWithoutAgent(ctx)
	}
	if err := r.exportToken(); err != nil {
		log.Printf("Runner.Run: %v", err)
		r.appendFailure(exitConfig, err.Error())
		return exitConfig
	}

	selfLock, isHeld, err := acquireLock(filepath.Join(r.dir, ".lock"), 0)
	if err != nil {
		log.Printf("Runner.Run: %v", err)
	}
	if !isHeld {
		fmt.Fprintln(os.Stdout, "already running")
		return exitOk
	}
	defer selfLock.Close()

	if !r.claimCadenceSlot() {
		return exitOk
	}
	warnIgnoredKnobs()

	release := PreventSleep()
	defer release()
	releaseTree := containTree()
	defer releaseTree()

	runCtx, cancel := context.WithTimeout(ctx, r.config.RunTimeout)
	defer cancel()

	if err := r.resetSteerDir(); err != nil {
		log.Printf("Runner.Run: %v", err)
		r.appendFailure(exitSoftware, err.Error())
		return exitSoftware
	}

	groupLockPath := filepath.Join(r.config.WorktreeRoot, "."+r.config.Group+".lock")
	groupLock, isHeld, err := acquireLock(groupLockPath, groupLockWait)
	if err != nil {
		log.Printf("Runner.Run: %v", err)
	}
	if !isHeld {
		reason := "group lock timeout: " + r.config.Group
		log.Print(reason)
		r.appendFailure(exitLockTimeout, reason)
		return exitLockTimeout
	}
	defer groupLock.Close()

	_ = os.Remove(r.cellPids)
	r.runNow = r.takeRunNowPrompt()
	r.itemId = backlog.ItemIdOf(r.runNow)
	status := r.prepareContainer()
	if status == exitOk {
		status = r.runKind(runCtx)
	}
	return r.finalStatus(ctx, runCtx.Err(), status)
}

func (r *Runner) appendFailure(status int, reason string) {
	appendResult(r.resultsPath, failureRow(status, reason))
}

func (r *Runner) claimCadenceSlot() bool {
	if r.config.CadenceDays <= 1 {
		return true
	}

	stamp := filepath.Join(r.dir, ".cadence-stamp")
	now := time.Now()
	if data, err := os.ReadFile(stamp); err == nil {
		last, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		age := now.Sub(time.Unix(last, 0))
		if age < time.Duration(r.config.CadenceDays)*day {
			fmt.Fprintf(os.Stdout, "cadence gate: last run %dd ago, next after %dd — skipping\n", int(age/day), r.config.CadenceDays)
			return false
		}
	}

	if err := os.WriteFile(stamp, []byte(strconv.FormatInt(now.Unix(), 10)), 0o644); err != nil {
		log.Printf("Runner.claimCadenceSlot: %v", err)
	}
	return true
}

func (r *Runner) claudeArgs(cell *stageCell) []string {
	var args []string
	tools := skills.AllowedTools(filepath.Join(skills.DefaultHomePath(), cell.skill))
	switch {
	case tools == "":
		log.Printf("Runner.claudeArgs: No allowed-tools manifest for %s; running without --allowedTools", cell.skill)
	case cell.grantsWrite:
		args = append(args, "--allowedTools", tools+", Write")
	default:
		args = append(args, "--allowedTools", tools)
	}
	if r.config.Model != "" {
		args = append(args, "--model", r.config.Model)
	}
	if cell.effort != "" {
		args = append(args, "--effort", cell.effort)
	}
	if r.config.PermissionMode != "" {
		args = append(args, "--permission-mode", r.config.PermissionMode)
	}
	if r.config.BudgetUsd != "" {
		args = append(args, "--max-budget-usd", r.config.BudgetUsd)
	}
	if mcpConfig := r.mcpConfig(cell); mcpConfig != "" {
		args = append(args, "--mcp-config", mcpConfig, "--strict-mcp-config")
	}
	return args
}

func (r *Runner) composePrompt(base string) string {
	parts := []string{base}
	if extra := r.extraPrompt(); extra != "" {
		parts = append(parts, extra)
	}
	return strings.Join(parts, " ")
}

// containerize appends its own failure row.
func (r *Runner) containerize(ctx context.Context, container *containerCell, spec *ChatSpec) int {
	command, err := container.start(ctx)
	switch {
	case errors.Is(err, errSlotTimeout):
		log.Printf("Runner.containerize: %v", err)
		r.appendFailure(exitLockTimeout, errSlotTimeout.Error())
		return exitLockTimeout
	case err != nil:
		log.Printf("Runner.containerize: %v", err)
		r.appendFailure(exitUnavailable, "container: "+firstLine(err.Error()))
		return exitUnavailable
	}

	spec.Command = command
	spec.Registration = container.registration()
	return exitOk
}

func (r *Runner) createWorktree(ctx context.Context) (string, int) {
	branch, err := r.worktree.create(ctx, r.resolveConflicts)
	switch {
	case errors.Is(err, errOpenBranchLimit):
		log.Printf(
			"open-instance limit reached for %s (ROUTINE_MAX_OPEN_BRANCHES=%d); merge a %s-* branch before the next run",
			r.config.Group,
			r.config.MaxOpenBranches,
			r.config.BranchPrefix,
		)
		return "", exitOk
	case err != nil:
		log.Printf("Runner.createWorktree: %v", err)
		r.appendFailure(exitSoftware, "worktree create failed: "+firstLine(err.Error()))
		return "", exitSoftware
	}
	r.worktree.regenerateContext()
	r.markStarted(branch)
	return r.worktree.path, exitOk
}

func (r *Runner) exportToken() error {
	token, err := routines.ReadToken(r.config.TokenLabel)
	if err != nil {
		return fmt.Errorf("Runner.exportToken: %w", err)
	}
	if err := os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", token); err != nil {
		return fmt.Errorf("Runner.exportToken: %w", err)
	}
	return nil
}

// extraPrompt joins ROUTINE_EXTRA_PROMPT with the consumed Run-Now file.
func (r *Runner) extraPrompt() string {
	var parts []string
	if r.config.ExtraPrompt != "" {
		parts = append(parts, r.config.ExtraPrompt)
	}
	if r.runNow != "" {
		parts = append(parts, r.runNow)
	}
	return strings.Join(parts, " ")
}

// finalStatus writes no row for a signal exit (143): the server's Kill op writes it.
func (r *Runner) finalStatus(ctx context.Context, runErr error, status int) int {
	switch {
	case ctx.Err() != nil:
		terminateRegistered(r.cellPids)
		return exitCancelled
	case runErr != nil:
		terminateRegistered(r.cellPids)
		r.appendFailure(exitDeadline, fmt.Sprintf("run backstop deadline hit after %s", r.config.RunTimeout))
		return exitDeadline
	}
	return status
}

// markSession supersedes the started row with one naming the cell's session, so a live run links its session before it ends; before markStarted it writes nothing.
func (r *Runner) markSession(sessionId string) {
	if r.branch == "" {
		return
	}

	row := startedRow(r.itemId, r.branch)
	row.SessionId = sessionId
	appendResult(r.resultsPath, row)
}

// markStarted appends the started row the board and the routine row poll on.
func (r *Runner) markStarted(branch string) {
	r.branch = branch
	appendResult(r.resultsPath, startedRow(r.itemId, branch))
}

// mcpConfig gives a container cell the registered peek-mcp entry: it has no ~/.claude.json.
func (r *Runner) mcpConfig(cell *stageCell) string {
	needsRegistered := cell.mcpConfig == "" && r.config.IsContainerActive
	if !needsRegistered {
		return cell.mcpConfig
	}

	mcpConfig, err := registeredMcpConfig(settings.DefaultClaudeJsonPath())
	if err != nil {
		log.Printf("Runner.mcpConfig: %v", err)
		return ""
	}
	return mcpConfig
}

func (r *Runner) nightlyRoster() []routines.RunReportRepo {
	roster := []routines.RunReportRepo{}
	registryPath := filepath.Join(r.config.StateDir, settings.StateRegistryFile)
	if _, err := os.Stat(registryPath); err != nil {
		return roster
	}

	registry := repos.NewRegistry(registryPath)
	if err := registry.Reload(); err != nil {
		log.Printf("Runner.nightlyRoster: %v", err)
		return roster
	}

	for _, repo := range registry.Repos() {
		if isGitRepo(repo.Path) {
			roster = append(roster, routines.RunReportRepo{Name: repo.Name, Path: repo.Path})
		}
	}
	return roster
}

// prepareContainer checks the container config before the worktree exists; the image builds at the first container cell.
func (r *Runner) prepareContainer() int {
	if !r.config.IsContainerActive {
		return exitOk
	}
	if !routines.ContainerSupported {
		r.appendFailure(exitConfig, "container routines are not supported on windows")
		return exitConfig
	}

	registryPath := filepath.Join(r.config.StateDir, settings.StateRegistryFile)
	repoMounts, err := resolveRepoMounts(r.config.ContainerMounts, registryPath, r.repoRoot)
	if err != nil {
		log.Printf("Runner.prepareContainer: %v", err)
		r.appendFailure(exitConfig, "container: "+firstLine(err.Error()))
		return exitConfig
	}
	r.repoMounts = repoMounts
	return exitOk
}

// prepareImage builds the cell image once per run; a failed build fails every later container cell without a rebuild.
func (r *Runner) prepareImage(ctx context.Context) int {
	if r.isImageBuilt {
		return r.imageStatus
	}
	r.isImageBuilt = true

	logPath := filepath.Join(r.config.SteerDir, imageBuildLogName)
	if err := buildImage(ctx, logPath, r.repoRoot); err != nil {
		log.Printf("Runner.prepareImage: %v", err)
		r.appendFailure(exitUnavailable, "container: "+firstLine(err.Error()))
		r.imageStatus = exitUnavailable
	}
	return r.imageStatus
}

// recordItem relies on the run's store commit to commit the backlog write.
func (r *Runner) recordItem() {
	if r.itemId == "" {
		return
	}

	storePath := filepath.Join(r.storeRoot, settings.StoreBacklogDir, "backlog.json")
	stage := backlog.StageForSkill(r.manifest.Skill())
	if err := backlog.RecordRun(r.itemId, r.branch, storePath, stage); err != nil {
		log.Printf("Runner.recordItem: %v", err)
	}
}

func (r *Runner) resetSteerDir() error {
	if err := os.MkdirAll(r.config.SteerDir, 0o755); err != nil {
		return fmt.Errorf("Runner.resetSteerDir: %w", err)
	}
	// The receipt stays: the server matches it to its log entry by content.
	_ = os.Remove(filepath.Join(r.config.SteerDir, routines.SteerPromptFileName))
	return nil
}

func (r *Runner) resolveConflicts(ctx context.Context) error {
	cell := &stageCell{
		complete: CompleteResult,
		effort:   effortLow,
		prompt:   mergeResolvePrompt,
		skill:    "merge-resolve",
		stage:    "merge-resolve",
		timeout:  time.Hour,
	}
	if _, status := r.runStage(ctx, r.worktree.path, cell); status != exitOk {
		return fmt.Errorf("Runner.resolveConflicts: merge-resolve exited %d", status)
	}
	return nil
}

func (r *Runner) runKind(ctx context.Context) int {
	switch r.manifest.Kind {
	case routines.KindBudget:
		return r.runBudget(ctx)
	case routines.KindSingle:
		return r.runSingle(ctx)
	case routines.KindSmineNightly:
		return newNightlyRun(r).run(ctx)
	}
	return exitConfig
}

// runStage fails the stage on a failure envelope behind a clean exit.
func (r *Runner) runStage(ctx context.Context, dir string, cell *stageCell) (*Envelope, int) {
	spec := &ChatSpec{
		Args:       r.claudeArgs(cell),
		CellPids:   r.cellPids,
		Complete:   cell.complete,
		Dir:        dir,
		ExitGrace:  r.config.ExitGrace,
		MarkerPath: cell.markerPath,
		OnSession:  r.markSession,
		Poll:       r.config.Poll,
		Prompt:     cell.prompt,
		SteerDir:   r.config.SteerDir,
		Timeout:    cell.timeout,
	}
	if r.config.IsContainerActive {
		if status := r.prepareImage(ctx); status != exitOk {
			return &Envelope{}, status
		}

		container := newContainerCell(r, dir)
		defer container.stop()
		if status := r.containerize(ctx, container, spec); status != exitOk {
			return &Envelope{}, status
		}
	}

	outcome, err := RunChat(ctx, spec)
	if err != nil {
		log.Printf("Runner.runStage: %s: %v", cell.stage, err)
		outcome = &ChatOutcome{Envelope: &Envelope{}, Status: exitSoftware}
	}

	status := outcome.Status
	if outcome.Envelope.isFailure() && status == exitOk {
		log.Printf("Runner.runStage: %s: Claude reported failure (is_error=%t subtype=%s)", cell.stage, outcome.Envelope.IsError, outcome.Envelope.Subtype)
		status = exitSoftware
	}
	appendResult(r.resultsPath, resultRow(outcome.Envelope, r.itemId, r.branch, cell.stage, status))
	return outcome.Envelope, status
}

// runWithoutAgent keeps the self lock and the deadline.
func (r *Runner) runWithoutAgent(ctx context.Context) int {
	selfLock, isHeld, err := acquireLock(filepath.Join(r.dir, ".lock"), 0)
	if err != nil {
		log.Printf("Runner.runWithoutAgent: %v", err)
	}
	if !isHeld {
		fmt.Fprintln(os.Stdout, "already running")
		return exitOk
	}
	defer selfLock.Close()

	runCtx, cancel := context.WithTimeout(ctx, r.config.RunTimeout)
	defer cancel()

	// A Run-Now prompt has no reader here; taking it removes the file.
	_ = r.takeRunNowPrompt()
	status := r.runKind(runCtx)
	return r.finalStatus(ctx, runCtx.Err(), status)
}

func (r *Runner) takeRunNowPrompt() string {
	path := filepath.Join(r.dir, routines.RunNowPromptFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	_ = os.Remove(path)
	return strings.TrimRight(string(data), "\n")
}

// stageCell is one claude chat stage.
type stageCell struct {
	complete    CompleteMode
	effort      string // "" = claude default
	grantsWrite bool   // appends Write for the marker file
	markerPath  string // required for CompleteMarker
	mcpConfig   string // --mcp-config value, strict; "" = the registered servers
	prompt      string
	skill       string // allowed-tools source
	stage       string // results.jsonl stage; "" = none
	timeout     time.Duration
}

// applyPlistEnv serves Task Scheduler and manual runs; launchd injects the keys itself.
func applyPlistEnv(dir string) (string, error) {
	label, env, err := routines.PlistMeta(dir)
	if err != nil {
		return "", fmt.Errorf("applyPlistEnv: %w", err)
	}

	for key, value := range env {
		if _, isSet := os.LookupEnv(key); isSet {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return "", fmt.Errorf("applyPlistEnv: %s: %w", key, err)
		}
	}
	return label, nil
}

// prepareProcessEnv widens a launchd-minimal PATH and protects the main checkout from the cells' writes.
func prepareProcessEnv(mainCheckout string) {
	pathDirs := []string{smineBinDir(), dockerDesktopBin, homebrewRustupBin, homebrewBin}
	for _, dir := range pathDirs {
		prependPathDir(dir)
	}
	_ = os.Setenv("DISABLE_AUTOUPDATER", "1")
	_ = os.Setenv(settings.ProtectedPathsKey, mainCheckout)
}

func prependPathDir(dir string) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}

	path := os.Getenv("PATH")
	if slices.Contains(filepath.SplitList(path), dir) {
		return
	}
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}

// smineBinDir is where the install puts the smine PATH binaries; skills call them by bare name.
func smineBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return settings.SmineBinDir(runtime.GOOS, home)
}

// warnIgnoredKnobs only warns: concurrency affects wall-clock, never results.
func warnIgnoredKnobs() {
	path, err := exec.LookPath("claude")
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	binary, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, knob := range probedKnobs {
		value := os.Getenv(knob)
		isUnset := value == ""
		isImplemented := bytes.Contains(binary, []byte(knob))
		if isUnset || isImplemented {
			continue
		}
		log.Printf("WARNING: %s=%s is set but the installed claude (%s) does not implement it — knob ignored; brew upgrade claude-code", knob, value, path)
	}
}
