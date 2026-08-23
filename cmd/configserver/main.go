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
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skills"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:6001", "listen address")
	allowRemote := flag.Bool("allow-remote", false, "permit binding a routable (non-loopback) interface; required opt-in since every endpoint is unauthenticated")
	acdslVerdicts := flag.String("acdsl-verdicts", "", "acdsl verdict log (default: ~/.claude/acdsl/verdicts.jsonl)")
	checklistPath := flag.String("checklist", "docs/checklist.md", "path to the workflow-improvements checklist")
	autoApplyRules := flag.String("auto-apply-rules", "skills/smine/smine-apply/assets/auto-apply-rules.md", "path to the smine auto-apply decide rules")
	codexPath := flag.String("codex-config", codex.DefaultPath(), "path to codex config.toml")
	sessionsDir := flag.String("sessions", "sessions", "path to the sessions directory")
	proposalsDir := flag.String("proposals", "proposals", "path to the proposals directory")
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
	peekControlPort := flag.Int("peek-control-port", 42442, "peek-mcp control dashboard port (0 disables the dashboard)")
	peekStart := flag.Bool("peek-start", true, "spawn peek-mcp when the port is not serving")
	reposPath := flag.String("repos", "repos.json", "path to the repo registry file")
	install := flag.Bool("install", false, "windows: register logon task + routines, then exit (macOS: use install.sh)")
	logFile := flag.String("logfile", "", "append all output to this file (the Windows logon task passes it; the binary is windowsgui-subsystem there, so a console is never attached)")
	routinesDir := flag.String("routines", "routines", "path to the routines directory")
	worktreeScripts := flag.String("worktree-scripts", "cmd/worktrees", "path to the worktree scripts")
	syncScripts := flag.String("sync-scripts", "cmd/sync", "path to the sync scripts")
	contextDir := flag.String("context", "context", "path to the context source root")
	flag.Parse()

	// Redirect before the first output: with -logfile everything (fmt prints,
	// log, child stderr) lands in one append-only file — the launchd
	// StandardOutPath analog for the windowsgui logon task.
	if *logFile != "" {
		if err := os.MkdirAll(filepath.Dir(*logFile), 0o755); err != nil {
			log.Fatal(err)
		}
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout, os.Stderr = f, f
		log.SetOutput(f)
	}

	// Binding a routable interface exposes every unauthenticated mutating/exec
	// endpoint to any LAN peer; require an explicit opt-in for anything but a
	// loopback bind.
	loopback, err := isLoopbackAddr(*addr)
	if err != nil {
		log.Fatalf("invalid -addr %q: %v", *addr, err)
	}
	if !loopback && !*allowRemote {
		log.Fatalf("-addr %q binds a routable interface; pass -allow-remote to opt in (every endpoint is unauthenticated)", *addr)
	}

	scriptsDir, err := filepath.Abs(*worktreeScripts)
	if err != nil {
		log.Fatal(err)
	}

	syncScriptsDir, err := filepath.Abs(*syncScripts)
	if err != nil {
		log.Fatal(err)
	}

	tokenDir, err := routines.TokenDir()
	if err != nil {
		log.Fatal(err)
	}

	// launchd stops agents with SIGTERM (D4); Ctrl-C stays for manual runs.
	// SIGTERM is never delivered on Windows; os.Interrupt covers Ctrl-C and
	// Task Scheduler's End is a hard kill — accepted (windows_support plan).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *install {
		os.Exit(runInstall(ctx, *addr, *peekPort, *peekControlPort))
	}

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
		AcdslVerdictsPath:  *acdslVerdicts,
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
		ProposalsDir:       *proposalsDir,
		ReposPath:          *reposPath,
		RoutinesDir:        *routinesDir,
		SessionsDir:        *sessionsDir,
		SettingsPath:       *settingsPath,
		SkillsHome:         *skillsHome,
		SkillsRepo:         *skillsRepo,
		SyncScriptsDir:     syncScriptsDir,
		TokenDir:           tokenDir,
		Version:            version,
		WorktreeScriptsDir: scriptsDir,
	}
	configServer, err := server.New(options)
	if err != nil {
		log.Fatal(err)
	}

	// darwin: a new login session starts with an empty gui domain; reload the
	// routines that should be running (D1). windows: reconcile the task store
	// with the routines directory instead.
	routines.SyncAll(ctx, *routinesDir)

	fmt.Printf("Config server listening on %s\n", *addr)
	fmt.Printf("Settings:  %s\n", *settingsPath)
	fmt.Printf("Disabled:  %s\n", config.DisabledPath(*settingsPath))
	fmt.Printf("Codex:     %s\n", *codexPath)
	fmt.Printf("Sessions:  %s\n", *sessionsDir)
	fmt.Printf("Proposals: %s\n", *proposalsDir)
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
}

// isLoopbackAddr reports whether a listen address binds only the loopback
// interface. An empty host binds every interface (routable); "localhost" and
// any loopback IP are loopback; an unresolved hostname is treated as routable
// so the opt-in gate errs safe.
func isLoopbackAddr(addr string) (bool, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}
	if host == "" {
		return false, nil
	}
	if host == "localhost" {
		return true, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false, nil
	}
	return ip.IsLoopback(), nil
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
	shell.HideWindow(cmd)
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
