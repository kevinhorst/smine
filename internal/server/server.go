package server

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skilleval"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/tokencount"
	"github.com/kevinhorst/smine/internal/trash"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets
var assetsFS embed.FS

// assetVersion cache-busts the stylesheet: the 24h asset cache keys on the URL.
var assetVersion = computeAssetVersion()

func computeAssetVersion() string {
	data, err := assetsFS.ReadFile("assets/style.css")
	if err != nil {
		return "0"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

const (
	pageOverview     = "overview"
	tmplOverview     = "overview.html"
	tmplOverviewTile = "_overview_tile.html"
)

type Options struct {
	AcdslVerdictsPath        string
	AgentwtPath              string
	BacklogDir               string
	CcusagePath              string
	ClaudeFragmentPath       string
	ClaudeHome               string
	ClaudeJsonPath           string
	CodexFragmentPath        string
	CodexPath                string
	ContextDir               string
	EvalsDir                 string
	ExamplesDir              string
	HeartbeatPath            string
	InitWelcome              bool
	IsDevInstance            bool // the make serve-dev instance: Profile shows the installation read-only
	PeekDashboardURL         string
	PeekEndpoint             string
	PeekExpectedVersion      string
	PeekSpawn                peek.SpawnOptions
	PresentationPath         string
	PresentationTemplatePath string
	RepoDir                  string
	ReposPath                string
	RoutinesDir              string
	SessionsDir              string
	SettingsPath             string
	SkillsHome               string
	SkillsRepo               string
	SmokeDir                 string
	StateDir                 string
	StylePath                string
	SyncScriptsDir           string
	TokenDir                 string
	Version                  string
}

type Server struct {
	acdslVerdictsPath   string
	agentwt             string
	archiveDir          string
	backlogDir          string
	bootstrap           *bootstrapRun
	catalog             []catalog.Entry
	claude              *installedClaude
	claudeFragmentPath  string
	claudeHome          string
	claudeJsonPath      string
	codexFragmentPath   string
	codexPath           string
	contextDir          string
	countRuns           *countRuns
	disabledHooks       *disabledHookStore
	evalsDir            string
	examplesDir         string
	heartbeat           heartbeat.Cli
	initWelcome         bool
	isDevInstance       bool
	machine             install.Env // the checkout's install env, Claude home = the settings dir
	peekClient          *peek.Client
	peekDashboardURL    string
	peekExpectedVersion string
	peekSpawn           peek.SpawnOptions
	presentation        *presentationStore
	proposals           *proposalStore
	records             *recordCache
	repoDir             string
	repoLocks           *repos.Locks
	repoRegistry        *repos.Registry
	routinesDir         string
	sessions            *sessions.Store
	sessionsDir         string
	settingsPath        string
	skillsHome          string
	skillsRepo          string
	smokeDir            string
	stateDir            string
	statusCache         *statusCache
	syncScripts         string
	titles              *titleStore
	tmpl                *template.Template
	tokenDir            string
	tokens              *tokencount.Counter
	trash               *trash.Store
	usage               *budget.Store
	usageCcusagePath    string
}

func New(opts *Options) (*Server, error) {
	presentationTemplate := opts.PresentationTemplatePath
	if presentationTemplate == "" {
		presentationTemplate = filepath.Join("settings", "claude_code", "presentation-profile.de.md")
	}
	machine := newMachine(opts.RepoDir, opts.SettingsPath)
	presentation := newPresentationStore(opts.PresentationPath, opts.StylePath, presentationTemplate, machine.Trash)
	titles := newTitleStore(opts.SkillsHome)
	records := &recordCache{}
	var server *Server
	funcs := template.FuncMap{
		// baseName exposes pool-recycled dirs whose name no longer matches their branch slug.
		"baseName": filepath.Base,
		// branchSlug is display-only; URLs and forms carry the full branch name.
		"branchSlug": func(branch string) string {
			return branch[strings.Index(branch, "/")+1:]
		},
		// branchKind lets a new claude-<kind>/* namespace tag itself without a template change.
		"branchKind": repos.AgentBranchKind,
		// deref reads an optional bool; nil means not applicable.
		"deref": func(value *bool) bool {
			return value != nil && *value
		},
		"hasPrefix": strings.HasPrefix,
		"join":      strings.Join,
		"toLower":   strings.ToLower,
		// providerKey is a provider's record key without its PROVIDER_ prefix.
		"providerKey": func(providerId string) string {
			return strings.TrimPrefix(skills.ProviderKey(providerId), providerPrefix)
		},
		// has reports list membership — checkbox groups mark stored selections.
		"has": func(item string, list []string) bool {
			return slices.Contains(list, item)
		},
		"domID": domID,
		// enforcementTitle defines the contract tokens in the tooltip.
		"enforcementTitle": enforcementTitle,
		// roleTitle is the effect-role axis, orthogonal to enforcementTitle.
		"roleTitle": roleTitle,
		// intOrEmpty renders optional interval fields ("" when nil).
		"intOrEmpty": func(p *int) any {
			if p == nil {
				return ""
			}
			return *p
		},
		// nameWrap lets long repo names wrap after '_' and '-', never mid-word.
		"nameWrap": func(name string) template.HTML {
			var builder strings.Builder
			for _, character := range name {
				builder.WriteString(template.HTMLEscapeString(string(character)))
				if character == '_' || character == '-' {
					builder.WriteString("<wbr>")
				}
			}
			return template.HTML(builder.String())
		},
		"pathEscape": url.PathEscape,
		// truncate keeps run-history result summaries on one line.
		"truncate": func(text string, limit int) string {
			runes := []rune(text)
			if len(runes) <= limit {
				return text
			}
			return string(runes[:limit]) + "…"
		},
		// pathWrap lets long paths wrap after each slash, never mid-word.
		"pathWrap": func(path string) template.HTML {
			var builder strings.Builder
			for _, segment := range strings.SplitAfter(path, "/") {
				builder.WriteString(template.HTMLEscapeString(segment))
				builder.WriteString("<wbr>")
			}
			return template.HTML(builder.String())
		},
		"peekDashboardURL": func() string {
			return opts.PeekDashboardURL
		},
		// openProposalCount feeds the nav pill; cached, bounded, zero on error.
		"openProposalCount": func() openProposalView {
			return server.openProposals()
		},
		// initWelcome keeps the Welcome nav entry after setup only with --init-welcome.
		"initWelcome": func() bool {
			return opts.InitWelcome
		},
		// hasComponent fails the render on an unknown id instead of hiding a page (master C2).
		"hasComponent": func(id string) (bool, error) {
			if _, ok := components.Lookup(components.ID(id)); !ok {
				return false, fmt.Errorf("hasComponent: Unknown component %q", id)
			}
			return records.has(machine, components.ID(id)), nil
		},
		// hasProposals and hasUsage gate the pages whose data needs several components (pagegate.go).
		"hasProposals": func() bool { return server.hasProposals() },
		"hasUsage":     func() bool { return server.hasUsage() },
		// t reads through the store so a Profile-page save takes effect without a restart.
		"t": func(text string) string {
			return translate(presentation.language(), text)
		},
		"langAttr": presentation.language,
		// isDeveloperAudience gates internals-exposing nav entries and proposal-card parts.
		"isDeveloperAudience": presentation.isDeveloperAudience,
		// appVersion feeds the nav brand; ldflags -X main.version stamps it.
		"appVersion": func() string {
			return opts.Version
		},
		"assetVersion": func() string {
			return assetVersion
		},
		// sectionCells backs the session report's section tables.
		"sectionCells": sectionCells,
		"peekSessionURL": func(id string) string {
			return peekSessionURL(opts.PeekDashboardURL, id)
		},
		"queryEscape": url.QueryEscape,
		// pauseSupported gates the pause/resume ops off darwin.
		"pauseSupported": func() bool { return routines.PauseSupported },
		"relTime":        relTime,
		// priBadge maps a backlog priority to its badge class.
		"priBadge": priBadgeClass,
		"priLabel": priLabel,
		// stampDate renders an unparseable stamp verbatim.
		"stampDate": func(stamp string) string {
			if t, err := time.Parse(time.RFC3339, stamp); err == nil {
				return t.Format("2006-01-02")
			}
			return stamp
		},
		"stampTime": stampTime,
		// titleView splits a session title into its label and state badge.
		"titleView": titles.view,
		// anchorTrack folds a runtime run's anchor hits into pills.
		"anchorTrack": anchorTrack,
		// shortPath, headPaths and tailPaths fold collision paths past shownPaths.
		"shortPath": shortPath,
		"headPaths": headPaths,
		"tailPaths": tailPaths,
		// sourceDocHref maps an entry source path to its doc-view route.
		"sourceDocHref": sourceDocHref,
		// docHref falls back to the generic schema source.
		"docHref": docHref,
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	entries, err := catalog.Load()
	if err != nil {
		return nil, err
	}

	swept, err := machine.Trash.Sweep()
	if err != nil {
		log.Printf("New: Trash sweep failed: %v", err)
	}
	if swept > 0 {
		log.Printf("New: Trash sweep removed %d stash dir(s)", swept)
	}

	store := sessions.NewStore(opts.SessionsDir, machine.Trash)
	if err := store.Reload(); err != nil {
		log.Printf("sessions: initial load failed, serving empty: %v", err)
	}

	registry := repos.NewRegistry(opts.ReposPath)
	if err := registry.Reload(); err != nil {
		log.Printf("repos: initial load failed, serving empty: %v", err)
	}

	disabledHooks, err := restoreDisabledHooks(machine, opts.SettingsPath)
	if err != nil {
		log.Printf("hooks: disabled-store restore failed, starting empty: %v", err)
	}

	claudeFragment := opts.ClaudeFragmentPath
	if claudeFragment == "" {
		claudeFragment = "settings/claude_code"
	}
	codexFragment := opts.CodexFragmentPath
	if codexFragment == "" {
		codexFragment = "settings/codex/config.toml"
	}
	smokeDir := opts.SmokeDir
	if smokeDir == "" {
		smokeDir, err = skilleval.SmokeRoot()
		if err != nil {
			log.Printf("WARN: %v — skill pages show no smoke verdicts", err)
		}
	}

	heartbeatCli := heartbeat.Cli{
		Addr:       "127.0.0.1:" + machine.Values.HeartbeatPort,
		BinaryPath: opts.HeartbeatPath,
		ClaudeHome: opts.ClaudeHome,
	}
	tokenOptions := &tokencount.Options{
		CacheDir: filepath.Join(opts.ClaudeHome, "context-report"),
		Mode:     tokencount.Mode(machine.Values.CountTokens),
	}
	tokens, err := tokencount.New(tokenOptions)
	if err != nil {
		return nil, fmt.Errorf("New: %w", err)
	}

	configServer := &Server{
		acdslVerdictsPath:   opts.AcdslVerdictsPath,
		archiveDir:          filepath.Join(opts.ClaudeHome, "heartbeat", "archive"),
		backlogDir:          opts.BacklogDir,
		bootstrap:           &bootstrapRun{},
		catalog:             entries,
		claude:              &installedClaude{},
		claudeFragmentPath:  claudeFragment,
		claudeHome:          opts.ClaudeHome,
		claudeJsonPath:      opts.ClaudeJsonPath,
		codexFragmentPath:   codexFragment,
		codexPath:           opts.CodexPath,
		contextDir:          opts.ContextDir,
		countRuns:           newCountRuns(),
		disabledHooks:       disabledHooks,
		evalsDir:            opts.EvalsDir,
		examplesDir:         opts.ExamplesDir,
		heartbeat:           heartbeatCli,
		initWelcome:         opts.InitWelcome,
		isDevInstance:       opts.IsDevInstance,
		machine:             machine,
		peekClient:          peek.NewClient(opts.PeekEndpoint),
		peekDashboardURL:    opts.PeekDashboardURL,
		agentwt:             opts.AgentwtPath,
		peekExpectedVersion: opts.PeekExpectedVersion,
		peekSpawn:           opts.PeekSpawn,
		presentation:        presentation,
		proposals:           newProposalStore(),
		records:             records,
		repoDir:             opts.RepoDir,
		repoLocks:           repos.NewLocks(),
		repoRegistry:        registry,
		routinesDir:         opts.RoutinesDir,
		sessions:            store,
		sessionsDir:         opts.SessionsDir,
		settingsPath:        opts.SettingsPath,
		skillsHome:          opts.SkillsHome,
		skillsRepo:          opts.SkillsRepo,
		smokeDir:            smokeDir,
		stateDir:            opts.StateDir,
		statusCache:         newStatusCache(),
		syncScripts:         opts.SyncScriptsDir,
		titles:              titles,
		tmpl:                tmpl,
		tokenDir:            opts.TokenDir,
		tokens:              tokens,
		trash:               machine.Trash,
		usage:               budget.NewStore(opts.StateDir),
		usageCcusagePath:    opts.CcusagePath,
	}
	server = configServer
	return configServer, nil
}

func (s *Server) assetsHandler() http.Handler {
	fileServer := http.FileServerFS(assetsFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) loadBoth() (*config.Settings, *config.Settings, error) {
	main, err := config.Load(s.settingsPath)
	if err != nil {
		return nil, nil, err
	}

	disabled, err := config.Load(config.DisabledPath(s.settingsPath))
	if err != nil {
		return nil, nil, err
	}

	return main, disabled, nil
}

// markers is the fragments' marker table from the install values of the machine.
func (s *Server) markers() config.Markers {
	return s.machine.Values.Markers(s.machine.Home, s.machine.RepoDir)
}

func (s *Server) saveBoth(main, disabled *config.Settings) error {
	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		return err
	}

	return config.Save(config.DisabledPath(s.settingsPath), disabled, s.trash)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /overview/welcome-tile", s.handleOverviewWelcomeTile)
	mux.Handle("GET /assets/", s.assetsHandler())
	mux.HandleFunc("GET /config/smine", s.handleSminePage)
	mux.HandleFunc("POST /config/smine/{component}", s.handleSmineSave)
	mux.HandleFunc("GET /config/hooks", s.handleHooksPage)
	mux.HandleFunc("POST /config/hooks/{component}/groups/toggle", s.handleToggleComponentHooks)
	mux.HandleFunc("POST /config/hooks/{component}/groups/{event}/{index}/toggle", s.handleToggleHook)
	mux.HandleFunc("DELETE /config/hooks/{component}/groups/{event}/{index}", s.handleRemoveHook)
	mux.HandleFunc("GET /config/permissions", s.handlePermissionsPage)
	mux.HandleFunc("GET /api/permissions", s.withGate(s.handleGetPermissions))
	mux.HandleFunc("POST /api/permissions/add", s.withGate(s.handleAddPermission))
	mux.HandleFunc("POST /api/permissions/{list}/{index}/toggle", s.withGate(s.handleTogglePermission))
	mux.HandleFunc("POST /api/permissions/{list}/items", s.withGate(s.handleAddPermissionItem))
	mux.HandleFunc("DELETE /api/permissions/{list}/{index}", s.withGate(s.handleRemovePermission))
	mux.HandleFunc("GET /api/mcp", s.handleGetMCP)
	mux.HandleFunc("POST /api/mcp/{server}/toggle", s.handleToggleMCP)
	mux.HandleFunc("POST /api/mcp/peek-mcp/update", s.handleUpdatePeek)
	mux.HandleFunc("GET /api/env", s.handleGetEnv)
	mux.HandleFunc("POST /api/env", s.handleSetEnv)
	mux.HandleFunc("DELETE /api/env/{key}", s.handleDeleteEnv)
	mux.HandleFunc("GET /api/model", s.handleGetModel)
	mux.HandleFunc("POST /api/model", s.handleSetModel)
	mux.HandleFunc("GET /sessions", s.handleSessionsIndex)
	mux.HandleFunc("GET /sessions-report/{id}", s.handleSessionReport)
	mux.HandleFunc("GET /sessions-report/{id}/counting", s.handleSessionReportCounting)
	mux.HandleFunc("GET /sessions-report/{id}/md", s.handleSessionReportMarkdown)
	mux.HandleFunc("GET /sessions/{scope}", s.handleSessionsScope)
	mux.HandleFunc("GET /sessions/{scope}/{week}", s.handleSessionsWeek)
	// Outside /sessions/{scope}: a repo named "archive" would be shadowed by a literal route there.
	mux.HandleFunc("GET /sessions-archive", s.handleSessionsArchiveTab)
	mux.HandleFunc("GET /sessions-context", s.handleSessionsContext)
	mux.HandleFunc("GET /usage", s.handleUsage)
	mux.HandleFunc("GET /usage.csv", s.handleUsageCsv)
	mux.HandleFunc("POST /usage/plan", s.handleUsagePlan)
	mux.HandleFunc("POST /usage/recalculate", s.handleUsageRecalculate)
	mux.HandleFunc("POST /sessions-archive", s.handleSessionsScopeArchive)
	mux.HandleFunc("POST /sessions-archive/{name}/unarchive", s.handleSessionsScopeUnarchive)
	mux.HandleFunc("POST /sessions-archive/{name}/delete", s.handleSessionsScopeDelete)
	mux.HandleFunc("GET /trash", s.handleTrashPage)
	mux.HandleFunc("POST /trash/{stamp}/restore", s.handleTrashRestore)
	mux.HandleFunc("GET /proposals", s.handleProposals)
	mux.HandleFunc("POST /api/proposals/pick", s.handleProposalPick)
	mux.HandleFunc("POST /api/proposals/pick/{sha}", s.handleProposalPickOne)
	mux.HandleFunc("POST /api/proposals/reject", s.handleProposalReject)
	mux.HandleFunc("POST /api/proposals/reject/{sha}", s.handleProposalRejectOne)
	mux.HandleFunc("GET /config", s.handleConfigRedirect)
	mux.HandleFunc("GET /config/{target}", s.handleConfigPage)
	mux.HandleFunc("POST /api/config/{target}/{key}", s.handleConfigSet)
	mux.HandleFunc("DELETE /api/config/{target}/{key}", s.handleConfigUnset)
	mux.HandleFunc("POST /api/config/{target}/sync/apply", s.handleConfigApplyOverrides)
	mux.HandleFunc("POST /api/config/{target}/sync/force", s.handleConfigForce)
	mux.HandleFunc("POST /api/config/{target}/sync/force/preview", s.handleConfigForcePreview)
	mux.HandleFunc("POST /api/config/{target}/sync/revert", s.handleConfigRevert)
	mux.HandleFunc("POST /api/config/{target}/{key}/items", s.handleConfigItemAdd)
	mux.HandleFunc("POST /api/config/{target}/{key}/choose-folder", s.handleConfigItemChooseFolder)
	mux.HandleFunc("DELETE /api/config/{target}/{key}/items/{index}", s.handleConfigItemRemove)
	mux.HandleFunc("POST /api/config/{target}/{key}/entries", s.handleConfigEntrySet)
	mux.HandleFunc("DELETE /api/config/{target}/{key}/entries/{name}", s.handleConfigEntryRemove)
	mux.HandleFunc("POST /api/config/codex/{key}/toggle", s.handleConfigCodexToggle)
	mux.HandleFunc("GET /backlog", s.handleBacklogPage)
	mux.HandleFunc("GET /backlog/{id}", s.handleBacklogItem)
	mux.HandleFunc("GET /backlog/{id}/activity", s.handleBacklogActivity)
	mux.HandleFunc("GET /backlog/{id}/runs", s.handleBacklogRuns)
	mux.HandleFunc("GET /profile", s.handleProfilePage)
	mux.HandleFunc("POST /profile", s.handleProfileSave)
	mux.HandleFunc("POST /profile/presentation", s.handleProfilePresentationSave)
	mux.HandleFunc("POST /profile/style", s.handleProfileStyleSave)
	mux.HandleFunc("POST /profile/test", s.handleProfileTest)
	mux.HandleFunc("POST /profile/installation/preview", s.handleInstallationPreview)
	mux.HandleFunc("POST /profile/installation/apply", s.handleInstallationApply)
	mux.HandleFunc("GET /profile/record-history/{stamp}", s.handleRecordHistory)
	mux.HandleFunc("POST /profile/tokens", s.handleProfileTokenAdd)
	mux.HandleFunc("GET /context/global/{name}", s.handleContextGlobalDoc)
	mux.HandleFunc("POST /context/global/toggle", s.handleContextGlobalToggle)
	mux.HandleFunc("GET /welcome", s.handleWelcome)
	mux.HandleFunc("GET /welcome/checks", s.handleWelcomeChecks)
	mux.HandleFunc("POST /welcome/verify-token", s.handleWelcomeVerifyToken)
	mux.HandleFunc("POST /welcome/bootstrap", s.handleWelcomeBootstrap)
	mux.HandleFunc("GET /welcome/bootstrap/status", s.handleWelcomeBootstrapStatus)
	mux.HandleFunc("POST /api/backlog/{id}/status", s.handleBacklogStatus)
	mux.HandleFunc("POST /api/backlog/{id}/pri", s.handleBacklogPri)
	mux.HandleFunc("POST /api/backlog/{id}/route", s.handleBacklogRoute)
	mux.HandleFunc("POST /api/backlog/{id}/epic", s.handleBacklogEpicMove)
	mux.HandleFunc("POST /api/backlog/epics", s.handleBacklogEpicAdd)
	mux.HandleFunc("DELETE /api/backlog/epics/{name}", s.handleBacklogEpicRemove)
	mux.HandleFunc("POST /api/backlog/{id}/dispatch", s.handleBacklogDispatch)
	mux.HandleFunc("GET /scripts/skills", s.handleSkillsIndex)
	mux.HandleFunc("GET /scripts/skills/graph", s.handleSkillsGraph)
	mux.HandleFunc("GET /scripts/skills/environment", s.handleEnvironment)
	mux.HandleFunc("GET /scripts/skills/environment/widget", s.handleEnvironmentWidget)
	mux.HandleFunc("POST /scripts/skills/environment", s.handleEnvironmentSave)
	mux.HandleFunc("POST /scripts/skills/environment/defaults", s.handleGlobalDefaultsSave)
	mux.HandleFunc("POST /scripts/skills/environment/forget", s.handleEnvironmentForget)
	mux.HandleFunc("POST /scripts/skills/sync", s.handleSkillsSync)
	mux.HandleFunc("POST /scripts/skills/visibility", s.handleSkillVisibility)
	mux.HandleFunc("POST /scripts/skills/{name}/deploy", s.handleSkillDeploy)
	mux.HandleFunc("POST /scripts/skills/{name}/remove", s.handleSkillRemove)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/env", s.handleSkillEnvWidget)
	mux.HandleFunc("POST /scripts/skills/{origin}/{name}/env", s.handleSkillEnv)
	mux.HandleFunc("POST /scripts/skills/{origin}/{name}/env/{repo}", s.handleSkillRepoEnv)
	mux.HandleFunc("POST /sync", s.handleSync)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}", s.handleSkillDetail)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/file", s.handleSkillFile)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/tests", s.handleSkillTests)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/tests/file", s.handleSkillTestsFile)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/graph", s.handleSkillGraph)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/graph/diff", s.handleSkillGraphDiff)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/graph/findings", s.handleSkillGraphFindings)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/graph/scenario", s.handleSkillGraphScenario)
	mux.HandleFunc("GET /context", s.handleContextIndex)
	mux.HandleFunc("POST /context/reach/bulk", s.handleContextReachBulk)
	mux.HandleFunc("POST /context/rule/{id}/reach", s.handleContextRuleReach)
	mux.HandleFunc("POST /context/entry/{id}/reach", s.handleContextEntryReach)
	mux.HandleFunc("POST /context/sync", s.handleContextSync)
	mux.HandleFunc("POST /context/aspects", s.handleContextAspectAdd)
	mux.HandleFunc("DELETE /context/aspects/{name}", s.handleContextAspectDelete)
	mux.HandleFunc("GET /context/entry/{id}", s.handleContextEntry)
	mux.HandleFunc("GET /context/rule/{id}", s.handleContextRule)
	mux.HandleFunc("GET /context/{dir}/{file}", s.handleContextDoc)
	mux.HandleFunc("GET /fleet", s.handleFleet)
	mux.HandleFunc("GET /fleet/status", s.handleFleetStatus)
	mux.HandleFunc("GET /fleet/archive/{instance}", s.handleFleetArchive)
	mux.HandleFunc("POST /fleet/{fleet}/members/{member}/remove", s.handleFleetMemberRemove)
	mux.HandleFunc("GET /runtime", s.handleRuntime)
	mux.HandleFunc("GET /runtime/status", s.handleRuntimeStatus)
	mux.HandleFunc("GET /repos", s.handleReposIndex)
	mux.HandleFunc("POST /repos/add", s.handleReposAdd)
	mux.HandleFunc("POST /repos/delete", s.handleReposDelete)
	mux.HandleFunc("POST /repos/choose-folder", s.handleReposChooseFolder)
	mux.HandleFunc("POST /repos/prune-jetbrains", s.handleReposPruneJetbrains)
	mux.HandleFunc("GET /repos/{name}", s.handleRepoDetail)
	mux.HandleFunc("GET /repos/{name}/status", s.handleRepoWorktreeStatus)
	mux.HandleFunc("GET /repos/{name}/branches/{branch}/commits/{class}", s.handleRepoCommits)
	mux.HandleFunc("POST /repos/{name}/remove-selected", s.handleRepoRemoveSelected)
	mux.HandleFunc("GET /routines", s.handleRoutinesIndex)
	mux.HandleFunc("GET /routines/{name}", s.handleRoutineDetail)
	mux.HandleFunc("GET /routines/{name}/runs/{id}", s.handleRoutineRunReport)
	mux.HandleFunc("GET /routines/{name}/row", s.handleRoutineRow)
	mux.HandleFunc("GET /routines/{name}/sessions", s.handleRoutineSessions)
	mux.HandleFunc("POST /routines/{name}/start", s.handleRoutineStart)
	mux.HandleFunc("POST /routines/{name}/stop", s.handleRoutineStop)
	mux.HandleFunc("POST /routines/{name}/run", s.handleRoutineRun)
	mux.HandleFunc("POST /routines/{name}/reschedule", s.handleRoutineReschedule)
	mux.HandleFunc("POST /routines/{name}/kill", s.handleRoutineKill)
	mux.HandleFunc("POST /routines/{name}/steer", s.handleRoutineSteer)
	mux.HandleFunc("GET /routines/{name}/steer-panel", s.handleRoutineSteerPanel)
	mux.HandleFunc("POST /routines/{name}/pause", s.handleRoutinePause)
	mux.HandleFunc("POST /routines/{name}/resume", s.handleRoutineResume)
	mux.HandleFunc("GET /routines/{name}/configure", s.handleRoutineConfigure)
	mux.HandleFunc("POST /routines/{name}/params", s.handleRoutineParams)
	mux.HandleFunc("POST /routines/{name}/duplicate", s.handleRoutineDuplicate)
	mux.HandleFunc("POST /routines/{name}/delete", s.handleRoutineDelete)
	return sameOriginGuard(mux)
}

// sameOriginGuard allows non-browser callers without either header: they are not the CSRF vector.
func sameOriginGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutatingMethod(r.Method) && !sameOriginRequest(r) {
			http.Error(w, "cross-site request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isMutatingMethod reports whether the method changes server state.
func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodDelete, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

// The Claude home is the settings file's dir, so tests with a temp settings path never read the real install.json.
func newMachine(repoDir, settingsPath string) install.Env {
	machine, err := install.NewEnvAt(repoDir, filepath.Dir(settingsPath), io.Discard)
	if err != nil {
		log.Printf("newMachine: %v — the installation reads as Full", err)
		home, _ := config.NativeHome()
		machine = install.Env{
			GOOS:        runtime.GOOS,
			Home:        home,
			MainRepoDir: gitroot.MainRoot(repoDir),
			RepoDir:     repoDir,
			Stdout:      io.Discard,
			Trash:       trash.New(filepath.Join(filepath.Dir(settingsPath), "smine"), 0),
		}
		machine.Values, err = install.InstallValues(install.Record{})
		if err != nil {
			log.Printf("newMachine: %v", err)
		}
	}
	machine.ClaudeHome = filepath.Dir(settingsPath)
	return machine
}

// sameOriginRequest trusts Sec-Fetch-Site when present, else requires a matching Origin.
func sameOriginRequest(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "cross-site", "same-site":
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Host == r.Host
}

// peekSessionURL is the dashboard's session page; empty without a dashboard or an id.
func peekSessionURL(base, id string) string {
	if base == "" || id == "" {
		return ""
	}
	return base + "sessions/" + url.PathEscape(id)
}

func relTime(t time.Time) string {
	elapsed := time.Since(t)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

func stampTime(stamp string) string {
	if t, err := time.Parse(time.RFC3339, stamp); err == nil {
		return t.Format("15:04:05Z")
	}
	return ""
}
