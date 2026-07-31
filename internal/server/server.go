package server

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/sessions"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets
var assetsFS embed.FS

const (
	pageOverview = "overview"
	tmplOverview = "overview.html"
)

type Options struct {
	AutoApplyRulesPath string
	ChecklistPath      string
	ClaudeFragmentPath string
	ClaudeJsonPath     string
	CodexFragmentPath  string
	CodexPath          string
	ContextDir         string
	EvalsDir           string
	ExamplesDir        string
	PeekDashboardURL   string
	PeekEndpoint       string
	ReposPath          string
	RoutinesDir        string
	SessionsDir        string
	SettingsPath       string
	SkillsHome         string
	SkillsRepo         string
	SyncScriptsDir     string
	WorktreeScriptsDir string
}

type Server struct {
	autoApplyRulesPath string
	catalog            []catalog.Entry
	checklistPath      string
	claudeFragmentPath string
	claudeJsonPath     string
	codexFragmentPath  string
	codexPath          string
	contextDir         string
	disabledHooks      *disabledHookStore
	evalsDir           string
	examplesDir        string
	peekClient         *peek.Client
	repoLocks          *repos.Locks
	repoRegistry       *repos.Registry
	routinesDir        string
	sessions           *sessions.Store
	sessionsDir        string
	settingsPath       string
	skillsHome         string
	skillsRepo         string
	syncScripts        string
	tmpl               *template.Template
	worktreeScripts    string
}

func New(opts *Options) (*Server, error) {
	funcs := template.FuncMap{
		// baseName shortens worktree paths to their directory name; the
		// detail table compares it against the branch slug to expose
		// pool-recycled dirs whose name no longer matches their branch.
		"baseName": filepath.Base,
		// branchParam strips the claude/ prefix for URL segments (D16).
		"branchParam": func(branch string) string {
			return strings.TrimPrefix(branch, agentBranchPrefix)
		},
		"domID": domID,
		// intOrEmpty renders optional interval fields ("" when nil).
		"intOrEmpty": func(p *int) any {
			if p == nil {
				return ""
			}
			return *p
		},
		"pathEscape": url.PathEscape,
		"peekDashboardURL": func() string {
			return opts.PeekDashboardURL
		},
		"peekSessionURL": func(id string) string {
			if opts.PeekDashboardURL == "" || id == "" {
				return ""
			}
			return opts.PeekDashboardURL + "sessions/" + url.PathEscape(id)
		},
		"queryEscape": url.QueryEscape,
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	entries, err := catalog.Load()
	if err != nil {
		return nil, err
	}

	store := sessions.NewStore(opts.SessionsDir)
	if err := store.Reload(); err != nil {
		log.Printf("sessions: initial load failed, serving empty: %v", err)
	}

	registry := repos.NewRegistry(opts.ReposPath)
	if err := registry.Reload(); err != nil {
		log.Printf("repos: initial load failed, serving empty: %v", err)
	}

	disabledHooks, err := restoreDisabledHooks(opts.SettingsPath)
	if err != nil {
		log.Printf("hooks: disabled-store restore failed, starting empty: %v", err)
	}

	claudeFragment := opts.ClaudeFragmentPath
	if claudeFragment == "" {
		claudeFragment = "settings/claude_code/settings.json"
	}
	codexFragment := opts.CodexFragmentPath
	if codexFragment == "" {
		codexFragment = "settings/codex/config.toml"
	}

	configServer := &Server{
		autoApplyRulesPath: opts.AutoApplyRulesPath,
		catalog:            entries,
		checklistPath:      opts.ChecklistPath,
		claudeFragmentPath: claudeFragment,
		claudeJsonPath:     opts.ClaudeJsonPath,
		codexFragmentPath:  codexFragment,
		codexPath:          opts.CodexPath,
		contextDir:         opts.ContextDir,
		disabledHooks:      disabledHooks,
		evalsDir:           opts.EvalsDir,
		examplesDir:        opts.ExamplesDir,
		peekClient:         peek.NewClient(opts.PeekEndpoint),
		repoLocks:          repos.NewLocks(),
		repoRegistry:       registry,
		routinesDir:        opts.RoutinesDir,
		sessions:           store,
		sessionsDir:        opts.SessionsDir,
		settingsPath:       opts.SettingsPath,
		skillsHome:         opts.SkillsHome,
		skillsRepo:         opts.SkillsRepo,
		syncScripts:        opts.SyncScriptsDir,
		tmpl:               tmpl,
		worktreeScripts:    opts.WorktreeScriptsDir,
	}
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

func (s *Server) saveBoth(main, disabled *config.Settings) error {
	if err := config.Save(s.settingsPath, main); err != nil {
		return err
	}

	return config.Save(config.DisabledPath(s.settingsPath), disabled)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.Handle("GET /assets/", s.assetsHandler())
	mux.HandleFunc("GET /api/hooks", s.handleGetHooks)
	mux.HandleFunc("POST /api/hooks/{event}/{index}/toggle", s.handleToggleHook)
	mux.HandleFunc("GET /api/permissions", s.handleGetPermissions)
	mux.HandleFunc("POST /api/permissions/{list}/{index}/toggle", s.handleTogglePermission)
	mux.HandleFunc("GET /api/mcp", s.handleGetMCP)
	mux.HandleFunc("POST /api/mcp/{server}/toggle", s.handleToggleMCP)
	mux.HandleFunc("GET /api/env", s.handleGetEnv)
	mux.HandleFunc("POST /api/env", s.handleSetEnv)
	mux.HandleFunc("DELETE /api/env/{key}", s.handleDeleteEnv)
	mux.HandleFunc("GET /api/model", s.handleGetModel)
	mux.HandleFunc("POST /api/model", s.handleSetModel)
	mux.HandleFunc("GET /sessions", s.handleSessionsIndex)
	mux.HandleFunc("GET /sessions/{scope}", s.handleSessionsScope)
	mux.HandleFunc("GET /sessions/{scope}/{batch}", s.handleSessionsBatch)
	mux.HandleFunc("POST /sessions/reload", s.handleSessionsReload)
	mux.HandleFunc("GET /proposals", s.handleProposals)
	mux.HandleFunc("POST /api/proposals/{kind}/{id}/vote", s.handleProposalVote)
	mux.HandleFunc("GET /config", s.handleConfigRedirect)
	mux.HandleFunc("GET /config/{target}", s.handleConfigPage)
	mux.HandleFunc("GET /config/{target}/docs", s.handleConfigDocs)
	mux.HandleFunc("POST /api/config/{target}/{key}", s.handleConfigSet)
	mux.HandleFunc("DELETE /api/config/{target}/{key}", s.handleConfigUnset)
	mux.HandleFunc("POST /api/config/{target}/sync/apply", s.handleConfigApplyOverrides)
	mux.HandleFunc("POST /api/config/{target}/sync/revert", s.handleConfigRevert)
	mux.HandleFunc("POST /api/config/{target}/{key}/items", s.handleConfigItemAdd)
	mux.HandleFunc("DELETE /api/config/{target}/{key}/items/{index}", s.handleConfigItemRemove)
	mux.HandleFunc("POST /api/config/codex/{key}/toggle", s.handleConfigCodexToggle)
	mux.HandleFunc("GET /docs/checklist", s.handleChecklistPage)
	mux.HandleFunc("POST /api/checklist/{number}/status", s.handleChecklistStatus)
	mux.HandleFunc("POST /api/auto-apply-rules", s.handleAutoApplyRulesSave)
	mux.HandleFunc("GET /scripts/skills", s.handleSkillsIndex)
	mux.HandleFunc("POST /scripts/skills/sync", s.handleSkillsSync)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}", s.handleSkillDetail)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/file", s.handleSkillFile)
	mux.HandleFunc("GET /context", s.handleContextIndex)
	mux.HandleFunc("POST /context/sync", s.handleContextSync)
	mux.HandleFunc("POST /context/aspects", s.handleContextAspectAdd)
	mux.HandleFunc("DELETE /context/aspects/{name}", s.handleContextAspectDelete)
	mux.HandleFunc("GET /context/{dir}/{file}", s.handleContextDoc)
	mux.HandleFunc("DELETE /api/hooks/{event}/{index}", s.handleDeleteHook)
	mux.HandleFunc("GET /repos", s.handleReposIndex)
	mux.HandleFunc("POST /repos/add", s.handleReposAdd)
	mux.HandleFunc("POST /repos/delete", s.handleReposDelete)
	mux.HandleFunc("POST /repos/choose-folder", s.handleReposChooseFolder)
	mux.HandleFunc("GET /repos/{name}", s.handleRepoDetail)
	mux.HandleFunc("GET /repos/{name}/branches/{branch}/commits/{class}", s.handleRepoCommits)
	mux.HandleFunc("POST /repos/{name}/branches/{branch}/sync", s.handleRepoSync)
	mux.HandleFunc("POST /repos/{name}/branches/{branch}/merge", s.handleRepoMerge)
	mux.HandleFunc("POST /repos/{name}/branches/{branch}/remove", s.handleRepoRemove)
	mux.HandleFunc("POST /repos/{name}/remove-selected", s.handleRepoRemoveSelected)
	mux.HandleFunc("POST /repos/{name}/cherry-pick", s.handleRepoCherryPick)
	mux.HandleFunc("GET /routines", s.handleRoutinesIndex)
	mux.HandleFunc("GET /routines/{name}", s.handleRoutineDetail)
	mux.HandleFunc("GET /routines/{name}/row", s.handleRoutineRow)
	mux.HandleFunc("POST /routines/{name}/start", s.handleRoutineStart)
	mux.HandleFunc("POST /routines/{name}/stop", s.handleRoutineStop)
	mux.HandleFunc("POST /routines/{name}/run", s.handleRoutineRun)
	mux.HandleFunc("POST /routines/{name}/reschedule", s.handleRoutineReschedule)
	mux.HandleFunc("GET /routines/{name}/configure", s.handleRoutineConfigure)
	mux.HandleFunc("POST /routines/{name}/params", s.handleRoutineParams)
	mux.HandleFunc("POST /routines/{name}/duplicate", s.handleRoutineDuplicate)
	mux.HandleFunc("GET /tools", s.handleToolsIndex)
	mux.HandleFunc("POST /tools/prune-jetbrains", s.handleToolsPruneJetbrains)
	mux.HandleFunc("POST /tools/secretscan", s.handleToolsSecretScan)
	return mux
}
