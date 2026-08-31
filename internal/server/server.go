package server

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

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
	AcdslVerdictsPath  string
	AutoApplyRulesPath string
	ChecklistPath      string
	ClaudeFragmentPath string
	ClaudeJsonPath     string
	CodexFragmentPath  string
	CodexPath          string
	ContextDir         string
	EvalsDir           string
	ExamplesDir        string
	InitWelcome        bool
	PeekDashboardURL   string
	PeekEndpoint       string
	PresentationPath   string
	ProposalsDir       string
	ReposPath          string
	RoutinesDir        string
	SessionsDir        string
	SettingsPath       string
	SkillsHome         string
	SkillsRepo         string
	StylePath          string
	SyncScriptsDir     string
	TokenDir           string
	Version            string
	WorktreeScriptsDir string
}

type Server struct {
	acdslVerdictsPath  string
	autoApplyRulesPath string
	bootstrap          *bootstrapRun
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
	initWelcome        bool
	peekClient         *peek.Client
	presentation       *presentationStore
	proposalsDir       string
	repoLocks          *repos.Locks
	repoRegistry       *repos.Registry
	routinesDir        string
	sessions           *sessions.Store
	sessionsDir        string
	settingsPath       string
	skillsHome         string
	skillsRepo         string
	statusCache        *statusCache
	syncScripts        string
	tmpl               *template.Template
	tokenDir           string
	worktreeScripts    string
}

func New(opts *Options) (*Server, error) {
	presentation := newPresentationStore(opts.PresentationPath, opts.StylePath)
	funcs := template.FuncMap{
		// baseName shortens worktree paths to their directory name; the
		// detail table compares it against the branch slug to expose
		// pool-recycled dirs whose name no longer matches their branch.
		"baseName": filepath.Base,
		// branchSlug strips the agent-namespace prefix for the worktree-dir
		// display compare; URLs and forms carry the full branch name (D3).
		"branchSlug": func(branch string) string {
			return branch[strings.Index(branch, "/")+1:]
		},
		"hasPrefix": strings.HasPrefix,
		"domID":     domID,
		// enforcementTitle explains an enforcement token in plain language —
		// the tokens are contract vocabulary, the tooltip is where they get
		// defined.
		"enforcementTitle": enforcementTitle,
		// intOrEmpty renders optional interval fields ("" when nil).
		"intOrEmpty": func(p *int) any {
			if p == nil {
				return ""
			}
			return *p
		},
		// nameWrap inserts word-break opportunities after '_' and '-' so long
		// repo names wrap at token boundaries, never mid-word (pathWrap's
		// sibling for names).
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
		// truncate caps a display string at limit runes with an ellipsis —
		// run-history result summaries stay one line.
		"truncate": func(text string, limit int) string {
			runes := []rune(text)
			if len(runes) <= limit {
				return text
			}
			return string(runes[:limit]) + "…"
		},
		// pathWrap inserts word-break opportunities after each slash so long
		// absolute paths wrap at segment boundaries, never mid-word.
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
		// initWelcome gates the Welcome nav entry: hidden once setup is done
		// unless the install opted in (--init-welcome) for debugging.
		"initWelcome": func() bool {
			return opts.InitWelcome
		},
		// t overlays the profile language onto English source strings;
		// identity when no catalog entry exists or the language is English
		// (plan D5). Reads go through the store so a Profile-page save takes
		// effect without a restart.
		"t": func(text string) string {
			return translate(presentation.language(), text)
		},
		"langAttr": presentation.language,
		// isDeveloperAudience gates internals-exposing nav entries and
		// proposal-card parts (plan D6).
		"isDeveloperAudience": presentation.isDeveloperAudience,
		// appVersion feeds the nav brand; ldflags -X main.version stamps it.
		"appVersion": func() string {
			return opts.Version
		},
		"peekSessionURL": func(id string) string {
			if opts.PeekDashboardURL == "" || id == "" {
				return ""
			}
			return opts.PeekDashboardURL + "sessions/" + url.PathEscape(id)
		},
		"queryEscape": url.QueryEscape,
		// stampDate/stampTime split an RFC3339 run timestamp into its date and
		// time-of-day lines; an unparseable stamp renders verbatim as the date.
		"stampDate": func(stamp string) string {
			if t, err := time.Parse(time.RFC3339, stamp); err == nil {
				return t.Format("2006-01-02")
			}
			return stamp
		},
		"stampTime": func(stamp string) string {
			if t, err := time.Parse(time.RFC3339, stamp); err == nil {
				return t.Format("15:04:05Z")
			}
			return ""
		},
		// sourceDocHref maps an entry source path to its doc-view route.
		"sourceDocHref": sourceDocHref,
		// docHref extracts a key's code.claude.com deep link from its
		// explanation, falling back to the generic schema source.
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
		acdslVerdictsPath:  opts.AcdslVerdictsPath,
		autoApplyRulesPath: opts.AutoApplyRulesPath,
		bootstrap:          &bootstrapRun{},
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
		initWelcome:        opts.InitWelcome,
		peekClient:         peek.NewClient(opts.PeekEndpoint),
		presentation:       presentation,
		proposalsDir:       opts.ProposalsDir,
		repoLocks:          repos.NewLocks(),
		repoRegistry:       registry,
		routinesDir:        opts.RoutinesDir,
		sessions:           store,
		sessionsDir:        opts.SessionsDir,
		settingsPath:       opts.SettingsPath,
		skillsHome:         opts.SkillsHome,
		skillsRepo:         opts.SkillsRepo,
		statusCache:        newStatusCache(),
		syncScripts:        opts.SyncScriptsDir,
		tmpl:               tmpl,
		tokenDir:           opts.TokenDir,
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
	mux.HandleFunc("POST /api/permissions/add", s.handleAddPermission)
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
	mux.HandleFunc("GET /sessions/archive", s.handleSessionsArchiveTab)
	mux.HandleFunc("POST /sessions/{scope}/archive", s.handleSessionsScopeArchive)
	mux.HandleFunc("POST /sessions/archive/{name}/unarchive", s.handleSessionsScopeUnarchive)
	mux.HandleFunc("POST /sessions/archive/{name}/delete", s.handleSessionsScopeDelete)
	mux.HandleFunc("GET /proposals", s.handleProposals)
	mux.HandleFunc("POST /api/proposals/{kind}/{id}/vote", s.handleProposalVote)
	mux.HandleFunc("GET /config", s.handleConfigRedirect)
	mux.HandleFunc("GET /config/{target}", s.handleConfigPage)
	mux.HandleFunc("POST /api/config/{target}/{key}", s.handleConfigSet)
	mux.HandleFunc("DELETE /api/config/{target}/{key}", s.handleConfigUnset)
	mux.HandleFunc("POST /api/config/{target}/sync/apply", s.handleConfigApplyOverrides)
	mux.HandleFunc("POST /api/config/{target}/sync/revert", s.handleConfigRevert)
	mux.HandleFunc("POST /api/config/{target}/{key}/items", s.handleConfigItemAdd)
	mux.HandleFunc("POST /api/config/{target}/{key}/choose-folder", s.handleConfigItemChooseFolder)
	mux.HandleFunc("DELETE /api/config/{target}/{key}/items/{index}", s.handleConfigItemRemove)
	mux.HandleFunc("POST /api/config/codex/{key}/toggle", s.handleConfigCodexToggle)
	mux.HandleFunc("GET /docs/checklist", s.handleChecklistPage)
	mux.HandleFunc("GET /profile", s.handleProfilePage)
	mux.HandleFunc("POST /profile", s.handleProfileSave)
	mux.HandleFunc("POST /profile/style", s.handleProfileStyleSave)
	mux.HandleFunc("POST /profile/test", s.handleProfileTest)
	mux.HandleFunc("GET /welcome", s.handleWelcome)
	mux.HandleFunc("GET /welcome/checks", s.handleWelcomeChecks)
	mux.HandleFunc("POST /welcome/verify-token", s.handleWelcomeVerifyToken)
	mux.HandleFunc("POST /welcome/bootstrap", s.handleWelcomeBootstrap)
	mux.HandleFunc("GET /welcome/bootstrap/status", s.handleWelcomeBootstrapStatus)
	mux.HandleFunc("POST /api/checklist/{number}/status", s.handleChecklistStatus)
	mux.HandleFunc("GET /scripts/skills", s.handleSkillsIndex)
	mux.HandleFunc("POST /scripts/skills/sync", s.handleSkillsSync)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}", s.handleSkillDetail)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/file", s.handleSkillFile)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/tests", s.handleSkillTests)
	mux.HandleFunc("GET /scripts/skills/{origin}/{name}/tests/file", s.handleSkillTestsFile)
	mux.HandleFunc("GET /context", s.handleContextIndex)
	mux.HandleFunc("POST /context/acdsl/fixtures/run", s.handleAcdslFixturesRun)
	mux.HandleFunc("POST /context/reach/bulk", s.handleContextReachBulk)
	mux.HandleFunc("POST /context/rule/{id}/reach", s.handleContextRuleReach)
	mux.HandleFunc("POST /context/entry/{id}/reach", s.handleContextEntryReach)
	mux.HandleFunc("POST /context/sync", s.handleContextSync)
	mux.HandleFunc("POST /context/aspects", s.handleContextAspectAdd)
	mux.HandleFunc("DELETE /context/aspects/{name}", s.handleContextAspectDelete)
	mux.HandleFunc("GET /context/entry/{id}", s.handleContextEntry)
	mux.HandleFunc("GET /context/rule/{id}", s.handleContextRule)
	mux.HandleFunc("POST /context/rule/{id}/fixtures/run", s.handleContextRuleFixturesRun)
	mux.HandleFunc("GET /context/{dir}/{file}", s.handleContextDoc)
	mux.HandleFunc("DELETE /api/hooks/{event}/{index}", s.handleDeleteHook)
	mux.HandleFunc("GET /repos", s.handleReposIndex)
	mux.HandleFunc("POST /repos/add", s.handleReposAdd)
	mux.HandleFunc("POST /repos/delete", s.handleReposDelete)
	mux.HandleFunc("POST /repos/choose-folder", s.handleReposChooseFolder)
	mux.HandleFunc("GET /repos/{name}", s.handleRepoDetail)
	mux.HandleFunc("GET /repos/{name}/status", s.handleRepoWorktreeStatus)
	mux.HandleFunc("GET /repos/{name}/branches/{branch}/commits/{class}", s.handleRepoCommits)
	mux.HandleFunc("POST /repos/{name}/branches/{branch}/sync", s.handleRepoSync)
	mux.HandleFunc("POST /repos/{name}/branches/{branch}/merge", s.handleRepoMerge)
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
	mux.HandleFunc("POST /routines/tokens", s.handleRoutineTokenAdd)
	mux.HandleFunc("GET /tools", s.handleToolsIndex)
	mux.HandleFunc("POST /tools/prune-jetbrains", s.handleToolsPruneJetbrains)
	return sameOriginGuard(mux)
}

// sameOriginGuard rejects state-changing requests that a cross-site page could
// forge. Safe methods (GET/HEAD) pass untouched, so every read path is
// unaffected. For POST/DELETE/PUT/PATCH the guard trusts the browser's Fetch
// metadata (Sec-Fetch-Site: same-origin/none) and, absent it, an Origin whose
// host matches the request host. Non-browser callers (curl, tests) send
// neither header and are allowed — they are not the CSRF vector.
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

// sameOriginRequest reports whether a mutating request originates same-origin.
// Sec-Fetch-Site is authoritative when present; otherwise an Origin header
// must match the request host. Requests carrying neither header are treated as
// non-browser clients and allowed.
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
