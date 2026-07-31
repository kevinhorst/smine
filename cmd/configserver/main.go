package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/kevinhorst/smine/internal/codex"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server"
	"github.com/kevinhorst/smine/internal/skills"
)

func main() {
	addr := flag.String("addr", ":6001", "listen address")
	checklistPath := flag.String("checklist", "docs/checklist.md", "path to the workflow-improvements checklist")
	autoApplyRules := flag.String("auto-apply-rules", "docs/smine-auto-apply-rules.md", "path to the smine auto-apply decide rules")
	codexPath := flag.String("codex-config", codex.DefaultPath(), "path to codex config.toml")
	sessionsDir := flag.String("sessions", "sessions", "path to the sessions directory")
	settingsPath := flag.String("settings", config.DefaultPath(), "path to settings.json")
	claudeJsonPath := flag.String("claude-json", config.DefaultClaudeJsonPath(), "path to Claude Code's .claude.json (MCP server names)")
	claudeFragment := flag.String("claude-fragment", "settings/claude_code/settings.json", "repo fragment for the live Claude settings")
	codexFragment := flag.String("codex-fragment", "settings/codex/config.toml", "repo fragment for the live codex config")
	skillsHome := flag.String("skills-home", skills.DefaultHomePath(), "path to the home skills root")
	skillsRepo := flag.String("skills-repo", "skills", "path to the repo skills root")
	evalsDir := flag.String("evals", "evals", "path to the eval results root")
	examplesDir := flag.String("examples", "examples", "path to the skill examples root")
	peekBin := flag.String("peek-bin", "peek-mcp", "peek-mcp binary (resolved via PATH)")
	peekPort := flag.Int("peek-port", 4242, "peek-mcp HTTP port (0 disables peek entirely)")
	peekControlPort := flag.Int("peek-control-port", 42422, "peek-mcp control dashboard port (0 disables the dashboard)")
	peekStart := flag.Bool("peek-start", true, "spawn peek-mcp when the port is not serving")
	reposPath := flag.String("repos", "repos.json", "path to the repo registry file")
	routinesDir := flag.String("routines", "routines", "path to the routines directory")
	worktreeScripts := flag.String("worktree-scripts", "cmd/worktrees", "path to the worktree scripts")
	syncScripts := flag.String("sync-scripts", "cmd/sync", "path to the sync scripts")
	contextDir := flag.String("context", "context", "path to the context source root")
	flag.Parse()

	scriptsDir, err := filepath.Abs(*worktreeScripts)
	if err != nil {
		log.Fatal(err)
	}

	syncScriptsDir, err := filepath.Abs(*syncScripts)
	if err != nil {
		log.Fatal(err)
	}

	// launchd stops agents with SIGTERM (D4); Ctrl-C stays for manual runs.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	endpoint := ""
	dashboardURL := ""
	if *peekPort != 0 {
		endpoint = fmt.Sprintf("http://127.0.0.1:%d/mcp", *peekPort)
		if *peekControlPort != 0 {
			dashboardURL = fmt.Sprintf("http://127.0.0.1:%d/", *peekControlPort)
		}
		if *peekStart {
			startPeek(ctx, *peekBin, *peekPort, *peekControlPort)
		}
	}

	options := &server.Options{
		AutoApplyRulesPath: *autoApplyRules,
		ChecklistPath:      *checklistPath,
		ClaudeFragmentPath: *claudeFragment,
		ClaudeJsonPath:     *claudeJsonPath,
		CodexFragmentPath:  *codexFragment,
		CodexPath:          *codexPath,
		ContextDir:         *contextDir,
		EvalsDir:           *evalsDir,
		ExamplesDir:        *examplesDir,
		PeekDashboardURL:   dashboardURL,
		PeekEndpoint:       endpoint,
		ReposPath:          *reposPath,
		RoutinesDir:        *routinesDir,
		SessionsDir:        *sessionsDir,
		SettingsPath:       *settingsPath,
		SkillsHome:         *skillsHome,
		SkillsRepo:         *skillsRepo,
		SyncScriptsDir:     syncScriptsDir,
		WorktreeScriptsDir: scriptsDir,
	}
	configServer, err := server.New(options)
	if err != nil {
		log.Fatal(err)
	}

	// A new login session starts with an empty gui domain; reload the
	// routines that should be running (D1).
	routines.BootstrapAll(ctx, *routinesDir)

	fmt.Printf("Config server listening on %s\n", *addr)
	fmt.Printf("Settings:  %s\n", *settingsPath)
	fmt.Printf("Disabled:  %s\n", config.DisabledPath(*settingsPath))
	fmt.Printf("Codex:     %s\n", *codexPath)
	fmt.Printf("Sessions:  %s\n", *sessionsDir)
	fmt.Printf("Checklist: %s\n", *checklistPath)
	fmt.Printf("Repos:     %s\n", *reposPath)
	fmt.Printf("Routines:  %s\n", *routinesDir)

	httpServer := &http.Server{Addr: *addr, Handler: configServer.Handler()}
	go func() {
		<-ctx.Done()
		httpServer.Shutdown(context.Background())
	}()

	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	// Graceful shutdown: park the in-memory disabled hooks for the next boot
	// (D33); the spawned peek child dies with ctx.
	if err := configServer.FlushDisabledHooks(); err != nil {
		log.Printf("hooks: disabled-store flush failed: %v", err)
	}
}

// startPeek spawns the peek-mcp binary unless something already listens on
// the port (an externally started peek keeps working, D32). The child shares
// ctx — it dies with the server. No restart loop.
func startPeek(ctx context.Context, bin string, port, controlPort int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		conn.Close()
		log.Printf("peek-mcp already serving on %s, not spawning", addr)
		return
	}

	args := []string{"start", "--transport", "http", "--port", strconv.Itoa(port)}
	if controlPort != 0 {
		args = append(args, "--control-port", strconv.Itoa(controlPort))
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("peek-mcp spawn failed (session column degraded): %v", err)
		return
	}

	log.Printf("peek-mcp spawned (pid %d) on http://%s/mcp", cmd.Process.Pid, addr)
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("peek-mcp exited: %v", err)
		}
	}()
}
