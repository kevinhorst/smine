package routinerun

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	bootstrapBudgetDefault     = "15"
	bootstrapEffortLow         = "low"
	bootstrapEffortMedium      = "medium"
	bootstrapMineTimeout       = 3 * time.Hour
	bootstrapModelDefault      = "claude-opus-5-5[1m]"
	bootstrapPermissionDefault = "acceptEdits"
	bootstrapResultLimit       = 300
	bootstrapResultsFile       = "smine-bootstrap-results.jsonl"
	bootstrapSessionsDefault   = 30
	bootstrapStageTimeout      = time.Hour
	exitCellStart              = 127
	exitPreflight              = 64
	orchestrateReportFile      = ".orchestrate-report"
)

// bootstrapDeps are needed by the orchestrate stage's acdsl gates; a missing one is never fatal.
var bootstrapDeps = []string{"jq", "shellcheck"}

// Bootstrap never aborts on a failed stage: the orchestrate judge is the arbiter.
type Bootstrap struct {
	isDryRun  bool
	mcpConfig string
	repoRoot  string
	results   string
	status    int
	stderr    io.Writer
	stdout    io.Writer
}

// NewBootstrap binds a run to the checkout it starts in.
func NewBootstrap(repoRoot string, stderr io.Writer, stdout io.Writer) *Bootstrap {
	bootstrap := &Bootstrap{
		isDryRun: os.Getenv("BOOTSTRAP_DRY_RUN") == "1",
		repoRoot: repoRoot,
		results:  filepath.Join(os.TempDir(), bootstrapResultsFile),
		stderr:   stderr,
		stdout:   stdout,
	}
	return bootstrap
}

// exportToken puts the routine token into the cells' environment.
func (b *Bootstrap) exportToken() int {
	path := os.Getenv("BOOTSTRAP_TOKEN_FILE")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		fmt.Fprintf(b.stderr, "token file missing or empty: %s\n", cmp.Or(path, "<unset>"))
		return exitConfig
	}

	_ = os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", strings.Map(dropSpace, string(data)))
	return exitOk
}

// finish fails on a dirty tree: orchestrate must have committed everything.
func (b *Bootstrap) finish(ctx context.Context) int {
	reportPath := filepath.Join(b.repoRoot, orchestrateReportFile)
	status, err := git(ctx, b.repoRoot, "status", "--porcelain")
	if err != nil || status != "" {
		fmt.Fprintf(b.stderr, "bootstrap ended with a dirty tree — orchestrate did not reach a terminal commit; see %s and %s\n", b.results, orchestrateReportFile)
		report, _ := os.ReadFile(reportPath)
		_, _ = b.stderr.Write(report)
		return exitSoftware
	}

	_ = os.Remove(reportPath)
	b.syncContextPacks(ctx)
	fmt.Fprintf(b.stdout, "bootstrap complete (stages logged to %s)\n", b.results)
	return b.status
}

func (b *Bootstrap) preflight(ctx context.Context) int {
	branch, err := git(ctx, b.repoRoot, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		branch = ""
	}
	if branch != mainBranch {
		fmt.Fprintf(b.stderr, "bootstrap requires the main checkout on main (found: %s)\n", cmp.Or(branch, "detached"))
		return exitPreflight
	}

	status, err := git(ctx, b.repoRoot, "status", "--porcelain")
	if err != nil || status != "" {
		fmt.Fprintln(b.stderr, "bootstrap requires a clean tree — commit or clean first (orchestrate commits the whole tree)")
		return exitPreflight
	}

	for _, dep := range bootstrapDeps {
		if _, err := exec.LookPath(dep); err != nil {
			fmt.Fprintf(b.stderr, "missing dependency: %s (acdsl gates will report it; install it before the next run)\n", dep)
		}
	}

	if !b.isDryRun {
		if status := b.exportToken(); status != exitOk {
			return status
		}
	}

	mcpConfig, err := SmineMcpConfig(settings.DefaultClaudeJsonPath())
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		fmt.Fprintln(b.stderr, "no smine peek config — run bin/configserver -apply")
		return exitConfig
	}
	b.mcpConfig = mcpConfig
	return exitOk
}

func (b *Bootstrap) publishStyle() {
	if b.isDryRun {
		return
	}

	stateDir, err := settings.StateDir("")
	if err != nil {
		fmt.Fprintf(b.stderr, "Bootstrap.publishStyle: %v\n", err)
		fmt.Fprintln(b.stderr, "style profile not published (continuing — see the smine-style stage)")
		return
	}

	if err := PublishStyleProfile(StyleStagingPath(b.repoRoot), trash.New(stateDir, 0)); err != nil {
		fmt.Fprintln(b.stderr, err)
		fmt.Fprintln(b.stderr, "style profile not published (continuing — see the smine-style stage)")
	}
}

func (b *Bootstrap) recordResult(output []byte, stage string, status int) {
	row := &bootstrapResult{}
	_ = json.Unmarshal(output, row)
	row.ExitStatus = status
	row.Stage = stage
	row.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if result := []rune(row.Result); len(result) > bootstrapResultLimit {
		row.Result = string(result[:bootstrapResultLimit])
	}

	file, err := os.OpenFile(b.results, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(b.stderr, "Bootstrap.recordResult: %v\n", err)
		return
	}
	defer func() { _ = file.Close() }()

	if err := json.NewEncoder(file).Encode(row); err != nil {
		fmt.Fprintf(b.stderr, "Bootstrap.recordResult: %v\n", err)
	}
}

func (b *Bootstrap) runStage(ctx context.Context, effort string, prompt string, skill string, timeout time.Duration) {
	tools := skills.AllowedTools(filepath.Join(skills.DefaultHomePath(), skill))
	if tools == "" {
		fmt.Fprintf(b.stderr, "no allowed-tools manifest for %s; running without --allowedTools\n", skill)
	}

	if b.isDryRun {
		fmt.Fprintf(
			b.stdout,
			"DRY: claude -p \"%s\" [skill=%s timeout=%ds effort=%s tools=%s]\n",
			prompt,
			skill,
			int(timeout.Seconds()),
			effort,
			cmp.Or(tools, "none"),
		)
		return
	}

	args := []string{"claude", "-p", prompt}
	if tools != "" {
		args = append(args, "--allowedTools", tools)
	}
	args = append(
		args,
		"--mcp-config", b.mcpConfig,
		"--strict-mcp-config",
		"--model", envString("ROUTINE_MODEL", bootstrapModelDefault),
		"--effort", effort,
		"--permission-mode", envString("ROUTINE_PERMISSION_MODE", bootstrapPermissionDefault),
		"--max-budget-usd", envString("ROUTINE_MAX_BUDGET_USD", bootstrapBudgetDefault),
		"--output-format", "json",
	)

	output := &bytes.Buffer{}
	spec := &CellSpec{
		Args:     args,
		CellPids: os.Getenv("ROUTINE_CELL_PIDS"),
		Dir:      b.repoRoot,
		Stderr:   b.stderr,
		Stdout:   output,
		Timeout:  timeout,
	}
	status, err := RunCell(ctx, spec)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		status = exitCellStart
	}

	b.recordResult(output.Bytes(), skill, status)
	if status != exitOk {
		fmt.Fprintf(b.stderr, "stage %s exited %d (continuing — orchestrate judges the result)\n", skill, status)
	}
	b.status = firstFailure(b.status, status)
}

// syncContextPacks is operator code: no agent writes a registered repo; a repo without a build record was never synced and is skipped.
func (b *Bootstrap) syncContextPacks(ctx context.Context) {
	stateDir, err := settings.StateDir("")
	if err != nil {
		fmt.Fprintf(b.stderr, "Bootstrap.syncContextPacks: %v\n", err)
		return
	}
	if err := contextdocs.Regenerate(filepath.Join(b.repoRoot, contextdocs.SourceContextDir)); err != nil {
		fmt.Fprintf(b.stderr, "Bootstrap.syncContextPacks: %v\n", err)
		return
	}

	store := trash.New(stateDir, 0)
	registry := repos.NewRegistry(filepath.Join(stateDir, settings.StateRegistryFile))
	for _, repo := range registry.Repos() {
		record, err := contextdocs.LoadContextFile(contextdocs.DeployRecordPath(stateDir, repo.Name))
		if err != nil || record.Deploy == nil {
			continue
		}

		options := &contextdocs.DeployOptions{
			ContextDeploy: *record.Deploy,
			Name:          repo.Name,
			RepoPath:      repo.Path,
			SourceRoot:    b.repoRoot,
			StateDir:      stateDir,
			Trash:         store,
		}
		if _, err := contextdocs.Deploy(ctx, options); err != nil {
			fmt.Fprintf(b.stderr, "context sync failed for %s (continuing): %v\n", repo.Name, err)
		}
	}
}

func (b *Bootstrap) Run(ctx context.Context) int {
	prependPathDir(smineBinDir())
	prependPathDir(homebrewBin)
	_ = os.Setenv("DISABLE_AUTOUPDATER", "1")
	_ = os.Setenv("DISABLE_TELEMETRY", "1")
	_ = os.Setenv("CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS", "0")
	if status := b.preflight(ctx); status != exitOk {
		return status
	}

	profile := presentation.Load(presentation.DefaultPath())
	isDevMode := nightlyDevMode(profile, profile.Audience == presentation.AudienceCasual)
	sessions := envInt("BOOTSTRAP_N", bootstrapSessionsDefault)
	since := os.Getenv("BOOTSTRAP_SINCE")
	if !b.isDryRun {
		overlaySkillVisibility(b.repoRoot)
	}

	release := PreventSleep()
	defer release()
	b.runStage(
		ctx,
		bootstrapEffortMedium,
		fmt.Sprintf("/smine-style %d", sessions),
		"smine-style",
		bootstrapStageTimeout,
	)
	b.publishStyle()
	b.runStage(
		ctx,
		bootstrapEffortMedium,
		bootstrapMinePrompt(isDevMode, sessions),
		"smine",
		bootstrapMineTimeout,
	)
	b.runStage(
		ctx,
		bootstrapEffortMedium,
		bootstrapOrchestratePrompt(since),
		"smine-orchestrate",
		bootstrapStageTimeout,
	)
	if b.isDryRun {
		return exitOk
	}
	return b.finish(ctx)
}

type bootstrapResult struct {
	ExitStatus int    `json:"exit_status"`
	IsError    bool   `json:"is_error"`
	Result     string `json:"result"`
	SessionId  string `json:"session_id"`
	Stage      string `json:"stage"`
	Subtype    string `json:"subtype"`
	Timestamp  string `json:"timestamp"`
}

func bootstrapMinePrompt(isDevMode bool, sessions int) string {
	prompt := fmt.Sprintf("/smine --nightly --stages %s --limit %d", nightlyMiningStages, sessions)
	if isDevMode {
		prompt += " --dev"
	}
	prompt += " --agents " + envString("SMINE_AGENTS", "claude,codex")
	if extra := os.Getenv("BOOTSTRAP_EXTRA_PROMPT"); extra != "" {
		prompt += " " + extra
	}
	return prompt
}

func bootstrapOrchestratePrompt(since string) string {
	if since == "" {
		return "/smine-orchestrate bootstrap"
	}
	return fmt.Sprintf("/smine-orchestrate bootstrap (since: %s)", since)
}

// dropSpace removes every whitespace character from a token file's content.
func dropSpace(char rune) rune {
	if unicode.IsSpace(char) {
		return -1
	}
	return char
}
