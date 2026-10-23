package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/kevinhorst/smine/internal/codex"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/migrate"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server"
	"github.com/kevinhorst/smine/internal/skills"
)

// defaultHeartbeatPort is the integer form of config.DefaultHeartbeatPort; restartGrace lets the spawning server finish its response before launchd stops it.
const (
	defaultHeartbeatPort = 6101
	restartGrace         = time.Second
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		log.Fatal(err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	storeRoot, err := config.StoreRoot(workDir)
	if err != nil {
		log.Fatal(err)
	}

	storeBacklogDir := filepath.Join(storeRoot, config.StoreBacklogDir)

	addr := flag.String("addr", "127.0.0.1:6001", "listen address")
	allowRemote := flag.Bool("allow-remote", false, "permit binding a routable (non-loopback) interface; required opt-in since the token then crosses the network in clear text")
	acdslVerdicts := flag.String("acdsl-verdicts", "", "acdsl verdict log (default: ~/.claude/acdsl/verdicts.jsonl)")
	backlogDir := flag.String("backlog", storeBacklogDir, "path to the backlog store directory (backlog.json); default: the plans-archive store of the checkout the server runs in")
	codexPath := flag.String("codex-config", codex.DefaultPath(), "path to codex config.toml")
	sessionsDir := flag.String("sessions", "sessions", "path to the sessions directory")
	settingsPath := flag.String("settings", config.DefaultPath(), "path to settings.json")
	claudeJsonPath := flag.String("claude-json", config.DefaultClaudeJsonPath(), "path to Claude Code's .claude.json (MCP server names)")
	claudeFragment := flag.String("claude-fragment", "settings/claude_code", "repo settings dir: base.json plus components/*.json, composed into the Claude settings template")
	codexFragment := flag.String("codex-fragment", "settings/codex/config.toml", "repo fragment for the live codex config")
	skillsHome := flag.String("skills-home", skills.DefaultHomePath(), "path to the home skills root")
	skillsRepo := flag.String("skills-repo", "skills", "path to the repo skills root")
	evalsDir := flag.String("evals", "evals", "path to the eval results root")
	examplesDir := flag.String("examples", "examples", "path to the skill examples root")
	peekBin := flag.String("peek-bin", "peek-mcp", "peek-mcp binary (resolved via PATH)")
	peekPort := flag.Int("peek-port", 4242, "peek-mcp HTTP port (0 disables peek entirely)")
	peekControlPort := flag.Int("peek-control-port", 42442, "peek-mcp control dashboard port (0 disables the dashboard)")
	heartbeatPort := flag.Int("heartbeat-port", defaultHeartbeatPort, "heartbeat daemon port, stored in the install record by -install")
	peekStart := flag.Bool("peek-start", true, "spawn peek-mcp when the port is not serving")
	peekVersionFile := flag.String("peek-version-file", "PEEK_MCP_VERSION", "file holding the pinned peek-mcp version")
	claudeHome := flag.String("claude-home", filepath.Join(homeDir, ".claude"), "claude home: passed to the spawned peek-mcp and heartbeat, holds session transcripts")
	codexHome := flag.String("codex-home", filepath.Join(homeDir, ".codex"), "codex home passed to the spawned peek-mcp")
	reposPath := flag.String("repos", filepath.Join(stateDir, config.StateRegistryFile), "path to the repo registry file")
	initWelcome := flag.Bool("init-welcome", false, "always show the Welcome nav entry and setup tile, even when all setup checks are green")
	presentationPath := flag.String("presentation", presentation.DefaultPath(), "path to the per-install presentation profile (missing file = English/developer defaults)")
	presentationTemplate := flag.String("presentation-template", "settings/claude_code/presentation-profile.de.md", "repo template seeding the German casual profile body on first activation")
	presentationProfile := flag.String("presentation-profile", "", "presentation profile template id to install when no profile exists (e.g. de); empty installs none")
	stylePath := flag.String("style", presentation.DefaultStylePath(), "path to the mined per-install style profile (missing file = no learned style yet)")
	isInstall := flag.Bool("install", false, "write the selection record from -components/-skills (else the stored record, Full when absent), apply it, start the server, then exit")
	componentsArg := flag.String("components", "", "with -install: component ids, comma-separated, or minimal | full")
	skillsArg := flag.String("skills", "", "with -install: all | none | chosen:<leaf>,<leaf>")
	skillsConfirmed := flag.Bool("skills-confirmed", false, "with -install: mark the skill selection confirmed so the terminal installer skips its skill question (=false clears the mark)")
	_ = flag.Bool("yes", false, "with -install: accepted for the installer's argument form; configserver never prompts")
	isPreview := flag.Bool("preview", false, "with -install: print the steps the apply would run, write nothing")
	isApply := flag.Bool("apply", false, "apply the stored selection record of this checkout, then exit")
	isRestartAgent := flag.Bool("restart-agent", false, "darwin: restart the config server LaunchAgent from its plist after a grace second, then exit — spawned detached by the Profile apply")
	logFile := flag.String("logfile", "", "append all output to this file (the Windows logon task passes it; the binary is windowsgui-subsystem there, so a console is never attached)")
	routinesDir := flag.String("routines", "routines", "path to the routines directory")
	refreshPlists := flag.Bool("refresh-routine-plists", false, "reconcile installed routine plists from their templates (env + schedule carried over), then exit")
	agentwtBin := flag.String("agentwt", filepath.Join("bin", "agentwt"), "path to the agentwt binary (worktree status and removal)")
	heartbeatBin := flag.String("heartbeat", filepath.Join("bin", "heartbeat"), "path to the heartbeat binary (fleet status document)")
	syncScripts := flag.String("sync-scripts", "cmd/sync", "path to the sync scripts")
	contextDir := flag.String("context", "context", "path to the context source root")
	credentialPath := flag.String("credential", filepath.Join(homeDir, ".claude", "configserver", "credential"), "file holding the SHA-256 digest of the access token (written by -new-credential)")
	devMode := flag.Bool("dev", false, "dev instance (make serve-dev): seed the settings file from the repo fragment, serve without a credential on a loopback address, peek off; refuses a home that holds an operator credential")
	migrateCheckout := flag.String("migrate-checkout", ".", "with -migrate-state: the main checkout the registry and the stores leave")
	migrateState := flag.Bool("migrate-state", false, "move repos.json into the state dir and place the store files under the store root — from the checkout, else from git history — each only when its target is missing; initialize the plans-archive repo when missing; then exit")
	newCredential := flag.Bool("new-credential", false, "generate a new access token, store its digest in -credential, print the token once, then exit")
	syncInto := flag.String("sync-into", "", "with -sync-skills -sync-variant: root the variant is rendered into (default: the claude skills root)")
	syncPrune := flag.Bool("sync-prune", false, "with -sync-skills: move marked installed skills the repo does not hold into the trash")
	syncSkills := flag.Bool("sync-skills", false, "deploy the repo skills of the checkout holding -sync-scripts into the skill homes, then exit")
	syncVariant := flag.String("sync-variant", "", "with -sync-skills: render one variant <leaf>=<name>:<disable> instead of the full sync")
	flag.Parse()

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

	if *refreshPlists {
		routinesAbs, err := filepath.Abs(*routinesDir)
		if err != nil {
			log.Fatal(err)
		}

		results, err := routines.RefreshPlists(homeDir, filepath.Dir(routinesAbs))
		for _, result := range results {
			if result.Changed {
				fmt.Printf("refreshed %s\n", result.Name)
			}
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if *newCredential {
		if err := runNewCredential(*credentialPath, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *migrateState {
		if err := runMigrateState(context.Background(), *migrateCheckout, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *syncSkills {
		repoDir, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		env, err := install.NewEnv(repoDir, io.Discard)
		if err != nil {
			log.Fatal(err)
		}

		output, err := runSyncSkills(env, *syncInto, *syncPrune, *syncScripts, *syncVariant)
		fmt.Print(output)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if *isRestartAgent {
		restartCtx, cancelRestart := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		code := runRestartAgent(restartCtx)
		cancelRestart()
		os.Exit(code)
	}

	if *isApply {
		applyCtx, cancelApply := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		code := runApply(applyCtx)
		cancelApply()
		os.Exit(code)
	}

	loopback, err := isLoopbackAddr(*addr)
	if err != nil {
		log.Fatalf("invalid -addr %q: %v", *addr, err)
	}
	if *devMode && !loopback {
		log.Fatalf("-addr %q binds a routable interface; a dev instance serves without a credential and binds loopback only", *addr)
	}
	if !loopback && !*allowRemote {
		log.Fatalf("-addr %q binds a routable interface; pass -allow-remote to opt in (the token then crosses the network in clear text)", *addr)
	}

	syncScriptsDir, err := filepath.Abs(*syncScripts)
	if err != nil {
		log.Fatal(err)
	}

	agentwtPath, err := filepath.Abs(*agentwtBin)
	if err != nil {
		log.Fatal(err)
	}

	heartbeatPath, err := filepath.Abs(*heartbeatBin)
	if err != nil {
		log.Fatal(err)
	}

	tokenDir, err := routines.TokenDir()
	if err != nil {
		log.Fatal(err)
	}

	if *isInstall {
		explicitFlags := map[string]bool{}
		flag.Visit(func(set *flag.Flag) { explicitFlags[set.Name] = true })
		request := &installRequest{
			addr:                  *addr,
			componentsArg:         *componentsArg,
			credentialPath:        *credentialPath,
			explicitFlags:         explicitFlags,
			heartbeatPort:         *heartbeatPort,
			initWelcome:           *initWelcome,
			isPreview:             *isPreview,
			isSkillsConfirmed:     *skillsConfirmed,
			peekControlPort:       *peekControlPort,
			peekPort:              *peekPort,
			presentationProfileId: *presentationProfile,
			skillsArg:             *skillsArg,
		}
		installCtx, cancelInstall := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		code := runInstall(installCtx, request)
		cancelInstall()
		os.Exit(code)
	}

	credential, err := startupCredential(*claudeFragment, *credentialPath, workDir, *devMode, *settingsPath)
	if err != nil {
		log.Fatal(err)
	}
	if *devMode {
		*peekPort = 0
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	listenAddr := listener.Addr().String()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	expectedPeek := ""
	if data, err := os.ReadFile(*peekVersionFile); err == nil {
		expectedPeek = strings.TrimSpace(string(data))
	} else {
		log.Printf("WARN: cannot read %s: %v — MCP version panel disabled", *peekVersionFile, err)
	}

	peekSpawn := peek.SpawnOptions{
		BackLink:    "http://" + listenAddr + "/",
		Bin:         *peekBin,
		ClaudeHome:  *claudeHome,
		CodexHome:   *codexHome,
		ControlPort: *peekControlPort,
		Port:        *peekPort,
	}

	endpoint := ""
	dashboardURL := ""
	if *peekPort != 0 {
		endpoint = fmt.Sprintf("http://127.0.0.1:%d/mcp", *peekPort)
		if *peekControlPort != 0 {
			dashboardURL = fmt.Sprintf("http://127.0.0.1:%d/", *peekControlPort)
		}
		if *peekStart && !peek.Start(ctx, peekSpawn) || !*peekStart && peek.Probe(peekSpawn) == peek.HolderForeign {
			endpoint = ""
			dashboardURL = ""
		}
	}

	options := &server.Options{
		AcdslVerdictsPath:        *acdslVerdicts,
		AgentwtPath:              agentwtPath,
		BacklogDir:               *backlogDir,
		ClaudeFragmentPath:       *claudeFragment,
		ClaudeHome:               *claudeHome,
		ClaudeJsonPath:           *claudeJsonPath,
		CodexFragmentPath:        *codexFragment,
		CodexPath:                *codexPath,
		ContextDir:               *contextDir,
		EvalsDir:                 *evalsDir,
		ExamplesDir:              *examplesDir,
		HeartbeatPath:            heartbeatPath,
		InitWelcome:              *initWelcome,
		IsDevInstance:            *devMode,
		PeekDashboardURL:         dashboardURL,
		PeekEndpoint:             endpoint,
		PeekExpectedVersion:      expectedPeek,
		PeekSpawn:                peekSpawn,
		PresentationPath:         *presentationPath,
		PresentationTemplatePath: *presentationTemplate,
		RepoDir:                  workDir,
		ReposPath:                *reposPath,
		RoutinesDir:              *routinesDir,
		SessionsDir:              *sessionsDir,
		SettingsPath:             *settingsPath,
		SkillsHome:               *skillsHome,
		SkillsRepo:               *skillsRepo,
		StateDir:                 stateDir,
		StylePath:                *stylePath,
		SyncScriptsDir:           syncScriptsDir,
		TokenDir:                 tokenDir,
		Version:                  version,
	}
	if err := contextdocs.Regenerate(*contextDir); err != nil {
		log.Printf("WARN: %v — context pages read the previous context file", err)
	}
	configServer, err := server.New(options)
	if err != nil {
		log.Fatal(err)
	}

	if isRoutinesSelected(workDir) {
		routines.SyncAll(ctx, *routinesDir)
	}
	// Deferred reloads apply only after the sync, when no freshly synced routine can be mid-bootout.
	configServer.ApplyPendingReloads(ctx)

	fmt.Printf("Config server listening on %s\n", listenAddr)
	fmt.Printf("Settings:  %s\n", *settingsPath)
	fmt.Printf("Disabled:  %s\n", config.DisabledPath(*settingsPath))
	fmt.Printf("Codex:     %s\n", *codexPath)
	fmt.Printf("Sessions:  %s\n", *sessionsDir)
	fmt.Printf("State:     %s\n", stateDir)
	fmt.Printf("Backlog:   %s\n", *backlogDir)
	fmt.Printf("Repos:     %s\n", *reposPath)
	fmt.Printf("Routines:  %s\n", *routinesDir)

	handler := configServer.Handler()
	if !*devMode {
		handler = credential.Require(handler)
	}
	httpServer := &http.Server{Handler: handler}
	go func() {
		<-ctx.Done()
		httpServer.Shutdown(context.Background())
	}()

	if err := httpServer.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

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

// An unreadable record keeps the boot sync.
func isRoutinesSelected(repoDir string) bool {
	env, err := install.NewEnv(repoDir, io.Discard)
	if err != nil {
		log.Printf("isRoutinesSelected: %v", err)
		return true
	}

	record, err := install.LoadCurrent(env)
	if err != nil {
		log.Printf("isRoutinesSelected: %v", err)
		return true
	}
	return slices.Contains(record.Components, components.Routines)
}

func prepareDev(claudeFragmentPath, credentialPath, repoDir, settingsPath string) error {
	_, err := os.Stat(credentialPath)
	if err == nil {
		return fmt.Errorf("prepareDev: %s holds an operator credential; -dev refuses this home", credentialPath)
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("prepareDev: %w", err)
	}

	if err := os.MkdirAll(os.TempDir(), 0o755); err != nil {
		return fmt.Errorf("prepareDev: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("prepareDev: %w", err)
	}

	return seedDevSettings(claudeFragmentPath, repoDir, settingsPath)
}

// An existing settings file is the dev instance's own state and stays.
func seedDevSettings(settingsDir, repoDir, settingsPath string) error {
	_, err := os.Stat(settingsPath)
	if err == nil {
		return nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("seedDevSettings: %w", err)
	}

	env, err := install.NewEnv(repoDir, io.Discard)
	if err != nil {
		return fmt.Errorf("seedDevSettings: %w", err)
	}

	markers := env.Values.Markers(env.Home, env.RepoDir)
	composed, err := config.LoadFragment(settingsDir, markers)
	if err != nil {
		return fmt.Errorf("seedDevSettings: %w", err)
	}

	if err := config.Save(settingsPath, composed, nil); err != nil {
		return fmt.Errorf("seedDevSettings: %w", err)
	}

	return nil
}

func startupCredential(claudeFragmentPath, credentialPath, repoDir string, isDev bool, settingsPath string) (*server.Credential, error) {
	if isDev {
		return nil, prepareDev(claudeFragmentPath, credentialPath, repoDir, settingsPath)
	}

	credential, err := server.LoadCredential(credentialPath)
	if err != nil {
		return nil, fmt.Errorf("%w — create one with: configserver -new-credential", err)
	}

	return credential, nil
}

func runMigrateState(ctx context.Context, checkoutDir string, output io.Writer) error {
	request, err := migrate.NewRequest(checkoutDir)
	if err != nil {
		return fmt.Errorf("runMigrateState: %w", err)
	}

	steps, runErr := request.Run(ctx)
	for _, step := range steps {
		if _, err := fmt.Fprintln(output, step.Line()); err != nil {
			return fmt.Errorf("runMigrateState: %w", err)
		}
	}

	if runErr != nil {
		return fmt.Errorf("runMigrateState: %w", runErr)
	}

	return nil
}

func runNewCredential(credentialPath string, output io.Writer) error {
	token, err := server.GenerateCredential(credentialPath)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(output, "Config server token — shown once, save it in your password manager (the browser asks for it as the password; any user name works):\n%s\n", token); err != nil {
		return fmt.Errorf("runNewCredential: %w", err)
	}

	return nil
}

// runRestartAgent waits for the spawning server to answer, then reloads its LaunchAgent from the plist on disk.
func runRestartAgent(ctx context.Context) int {
	repoDir, err := os.Getwd()
	if err != nil {
		log.Printf("runRestartAgent: %v", err)
		return 1
	}

	env, err := install.NewEnv(repoDir, os.Stdout)
	if err != nil {
		log.Printf("runRestartAgent: %v", err)
		return 1
	}

	time.Sleep(restartGrace)
	if err := install.RestartAgent(ctx, install.ServerAgentLabel, install.ServerAgentPlist(env)); err != nil {
		log.Printf("runRestartAgent: %v", err)
		return 1
	}
	return 0
}

func runSyncSkills(env install.Env, into string, isPrune bool, scriptsDir, variant string) (string, error) {
	options, err := skills.RepoSyncOptions(scriptsDir)
	if err != nil {
		return "", fmt.Errorf("runSyncSkills: %w", err)
	}

	record, err := install.LoadCurrent(env)
	if err != nil {
		return "", fmt.Errorf("runSyncSkills: %w", err)
	}

	contextDir, err := install.Resolve(record, components.Context, "context_dir")
	if err != nil {
		return "", fmt.Errorf("runSyncSkills: %w", err)
	}

	options.Components = install.ExpandedComponents(record, env.GOOS)
	options.ContextDir = contextDir.Text
	options.Into = into
	options.Prune = isPrune
	options.Selection = &skills.Selection{Chosen: record.Skills.Chosen, Mode: record.Skills.Mode}
	options.Rows = install.SkillRows(record)
	options.Variant = variant
	return skills.Sync(options)
}
