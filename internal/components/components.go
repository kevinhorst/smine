// Package components is the installation component catalog, pure data shared by installer, skill model and config server.
package components

import "slices"

// ID is a component's stable identifier, carried by the selection record, skill model requirement fields and installer arguments.
type ID string

const (
	ConfigServer       ID = "config-server"
	Skills             ID = "skills"
	SessionHooks       ID = "session-hooks"
	SkillEnvironment   ID = "skill-environment"
	Context            ID = "context"
	Gate               ID = "gate"
	Worktrees          ID = "worktrees"
	SessionHelpers     ID = "session-helpers"
	GlobalInstructions ID = "global-instructions"
	Agentwt            ID = "agentwt"
	Peek               ID = "peek"
	Ccusage            ID = "ccusage"
	Heartbeat          ID = "heartbeat"
	Routines           ID = "routines"
	Token              ID = "token"
	Mining             ID = "mining"
)

// GOOS values the catalog restricts components to.
const (
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"
)

// titleMining is the Mining pipeline's title, also named by the rows it degrades.
const titleMining = "Mining pipeline"

// catalog is every component in display order — the concept's catalog.
var catalog = []Component{
	{
		ID: ConfigServer,
		Brings: []string{
			"builds the config server and the rules tool from the checkout into bin/configserver and the smine bin dir (~/go/bin/smine-rules; %LOCALAPPDATA%\\smine\\bin\\smine-rules.exe on Windows)",
			"writes the access credential to ~/.claude/configserver/credential and prints its token once",
			"fills the base keys of ~/.claude/settings.json where they are absent — present values stay yours",
			"writes ~/.codex/config.toml from the checkout's settings/codex/config.toml",
			"macOS: writes the LaunchAgent com.smine.configserver and starts the server on the configured port",
			"ships smine-rules, the program behind the skill capabilities plan-sections, lanes, title-set and more — the skill deploy binds them to it",
		},
		Fixed:    true,
		Settings: configServerSettings,
		Summary:  "The web UI and the smine-rules tool, always on.",
		Title:    "Config server",
	},
	{
		ID: Skills,
		Brings: []string{
			"renders every selected skill into ~/.claude/skills/<leaf> and ~/.codex/skills/<leaf> (SKILL.md plus payloads), each folder marked .smine-deployed",
			"copies the agent definitions of the selected skills into ~/.claude/agents",
			"moves deselected smine skill folders to ~/.claude/smine/trash — folders without the marker are never touched",
		},
		Summary: "The repo's skills as slash commands in Claude Code and Codex.",
		Title:   "Skills",
		Without: []string{
			"no smine slash command in any session",
			"every folder marked .smine-deployed under ~/.claude/skills and ~/.codex/skills moves to the trash, the repo's agent definitions in ~/.claude/agents with them",
		},
	},
	{
		ID: SessionHooks,
		Brings: []string{
			"the hooks config ~/.claude/hooks/config.json, rendered from the settings of every selected hook component",
			"the master switch of the six hook components: Skill environment, Context system, Permission gate, Session worktrees, Session helpers and heartbeat",
		},
		Summary: "The master switch for everything that intercepts a session.",
		Title:   "Session hooks",
		Without: []string{
			"none of the six hook components can be selected",
			"Claude Code runs stock: no hook of this checkout touches a session",
		},
	},
	{
		ID: SkillEnvironment,
		Brings: []string{
			"the hook binary skillenv in ~/.claude/hooks/bin with its SessionStart, CwdChanged, UserPromptExpansion and PreToolUse groups",
			"each repo's skill values exported into its sessions — a skill whose required repo value is unset is stopped before it runs",
		},
		Requires: []ID{SessionHooks},
		Summary:  "Each repo's skill values exported into its sessions.",
		Title:    "Skill environment",
		Without: []string{
			"a skill reading a repo-scoped value aborts at invocation (<NAME>: not set) until you export the value in the shell that starts the session",
			"the Environment page still stores per-repo values — nothing carries them into a session",
		},
	},
	{
		ID: Context,
		Brings: []string{
			"the hook binaries readgate, globalcontext, skillcontext, planmodecontext and acdslhook in ~/.claude/hooks/bin",
			"their hook groups (SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, SubagentStart) in ~/.claude/settings.json",
			"the global context pack ~/.claude/context/global/always-context.md rendered from the checkout's context/",
		},
		Requires: []ID{SessionHooks, Gate},
		Settings: contextSettings,
		Summary:  "Context docs injected into sessions, skills and plan mode, plus the read gate.",
		Title:    "Context system",
		Without: []string{
			"no context reaches a session: the global pack and the per-skill context entries are not injected",
			"no plan-format rules on plan-mode entry and no ACDSL rules delivered with a governed file's Read",
			"the read gate no longer enforces the language guides before an edit",
			titleMining + " cannot be selected",
			"the context-create and context-bootstrap skills are not deployed",
		},
	},
	{
		ID: Gate,
		Brings: []string{
			"the hook binaries toolpolicy and skillinvokegate in ~/.claude/hooks/bin",
			"their hook groups (DirectoryAdded, PermissionRequest, PreToolUse, PostToolUse, UserPromptSubmit) and the permission template in ~/.claude/settings.json",
			"creates ~/.cache/claude-routine/artifacts, which the permission template adds to every session's directories for skill and eval artifacts",
		},
		Requires: []ID{SessionHooks},
		Settings: gateSettings,
		Summary:  "Closed-world permissions without prompts.",
		Title:    "Permission gate",
		Without: []string{
			"Claude Code prompts for every permission natively — your own permission settings stay as they are",
			"no frozen plan permissions and no toolpolicy allow self-service from inside a session",
			"any session may invoke any skill: no skill-invoke gate",
			"Context system, and with it " + titleMining + ", cannot be selected",
			"the heartbeat-demo and smine-help skills are not deployed — other skills drop their toolpolicy steps",
		},
	},
	{
		ID: Worktrees,
		Brings: []string{
			"the hook binaries worktreecreate and worktreewriteguard in ~/.claude/hooks/bin",
			"their hook groups (WorktreeCreate, SessionStart, PreToolUse) in ~/.claude/settings.json",
		},
		Requires: []ID{SessionHooks},
		Settings: worktreesSettings,
		Summary:  "Every session in its own worktree.",
		Title:    "Session worktrees",
		Without: []string{
			"no .claude/worktrees/<name> per session — every session edits the checkout it starts in",
			"no write guard: a session may write into the main checkout or another session's worktree",
		},
	},
	{
		ID: SessionHelpers,
		Brings: []string{
			"the hook binaries sessiontitle, skillhelp and playsound in ~/.claude/hooks/bin",
			"their hook groups (Notification, Stop, SessionStart, PreToolUse, PostToolUse, UserPromptSubmit) in ~/.claude/settings.json",
		},
		Requires: []ID{SessionHooks},
		Settings: sessionHelpersSettings,
		Summary:  "Session titles, skill help, sounds.",
		Title:    "Session helpers",
		Without: []string{
			"session titles stay whatever Claude Code generates",
			"/<skill> --help is not answered",
			"no sound on a notification or a finished turn",
			"the smine-help skill is not deployed",
		},
	},
	{
		ID: GlobalInstructions,
		Brings: []string{
			"writes ~/.claude/AGENTS.md from the checkout's settings/claude_code/AGENTS.md when the file is absent or unchanged since smine wrote it — your own file is never overwritten",
		},
		Summary: "The opinionated global instruction file.",
		Title:   "Global instructions",
		Without: []string{
			"your own ~/.claude/AGENTS.md stays as it is — only a copy this component wrote is removed",
		},
	},
	{
		ID: Agentwt,
		Brings: []string{
			"copies the prebuilt Rust binary bin/agentwt to the smine bin dir (~/go/bin/smine-agentwt; %LOCALAPPDATA%\\smine\\bin\\smine-agentwt.exe on Windows)",
		},
		Summary: "Worktree status, merge order, safe removal.",
		Title:   "agentwt",
		Without: []string{
			"the close, done, fleet, restack, smine and smine-consolidate skills are not deployed — other skills drop their smine-agentwt steps",
			"the Repos page shows no worktree table and offers no removal",
		},
	},
	{
		ID: Peek,
		Brings: []string{
			"installs peek-mcp at the pinned version (PEEK_MCP_VERSION) to ~/go/bin/peek-mcp",
			"registers the peek-mcp MCP server in ~/.claude.json",
			"sets the OTEL telemetry env keys in ~/.claude/settings.json so sessions export their telemetry to peek",
		},
		Settings: peekSettings,
		Summary:  "Session inspection and telemetry.",
		Title:    "peek",
		Without: []string{
			"the peek, overlap and watch skills are not deployed",
			titleMining + " cannot be selected",
			"no Peek link in the nav",
			"session columns show no session data",
			"eval runs record no telemetry",
			"the Usage page loses its peek figures — with ccusage off too, no Usage page and no Overview cost tile",
		},
	},
	{
		ID: Ccusage,
		Brings: []string{
			"nothing written — it checks that ccusage 20.0.20 or newer is on PATH (brew install ccusage, or npm install -g ccusage)",
			"the smine-budget routine records each session's ccusage cost beside peek's figures",
		},
		Summary: "Cost figures from ccusage.",
		Title:   "ccusage",
		Without: []string{
			"the Usage page loses its ccusage column and the recalculate ccusage button",
			"with peek off too, no Usage page and no Overview cost tile",
		},
	},
	{
		ID: Heartbeat,
		Brings: []string{
			"copies the prebuilt Rust binary bin/heartbeat to the smine bin dir (~/go/bin/smine-heartbeat; %LOCALAPPDATA%\\smine\\bin\\smine-heartbeat.exe on Windows)",
			"renders ~/.claude/heartbeat/config.json from the catalog defaults and your overrides",
			"writes the LaunchAgent com.smine.heartbeat (on Windows the logon task \\smine\\com.smine.heartbeat, logging to %LOCALAPPDATA%\\claude-routine\\logs\\heartbeat.log) and starts the daemon on the configured port",
			"hook groups for twelve events in ~/.claude/settings.json that feed the daemon over HTTP — SessionStart posts through the hook binary heartbeatfeed in ~/.claude/hooks/bin and ignores the answer — plus HEARTBEAT_ADDR",
		},
		Platforms: []string{goosDarwin, goosWindows},
		Requires:  []ID{SessionHooks},
		Settings:  heartbeatSettings,
		Summary:   "Fleet state, orchestration, runtime checks.",
		Title:     "heartbeat",
		Without: []string{
			"the join, done, runtime, fleet and heartbeat-demo skills are not deployed — other skills drop their smine-heartbeat steps",
			"no Fleet and Runtime tabs",
			"skill runs are not checked while they run",
			"eval runs record no heartbeat rows",
			"the watch skill falls back to peek alone",
		},
	},
	{
		ID: Routines,
		Brings: []string{
			"builds the routine runner from the checkout into the smine bin dir (~/go/bin/smine-routine; %LOCALAPPDATA%\\smine\\bin\\smine-routine.exe on Windows)",
			"renders routines/<name>/*.plist from their templates once — an existing plist stays yours; macOS loads them into launchd, Windows translates them into Task Scheduler jobs",
		},
		Platforms: []string{goosDarwin, goosWindows},
		Requires:  []ID{Token},
		Settings:  routinesSettings,
		Summary:   "Scheduled unattended runs.",
		Title:     "Routines",
		Without: []string{
			"no scheduled run: the nightly mining never fires",
			"every loaded routine job is suspended — turning Routines back on resumes it",
			"the plan-archive, fleet and smine-style skills are not deployed — other skills drop their smine-routine steps",
			"no Routines and Backlog pages",
			"no Usage page and no Overview cost tile: smine-budget, a routine, is the only writer of the usage store",
		},
	},
	{
		ID: Token,
		Brings: []string{
			"nothing written — it checks that a headless token exists under ~/.config/claude-routine/tokens/<label>",
		},
		Summary: "Unattended claude -p calls.",
		Title:   "Headless token",
		Without: []string{
			"Routines cannot be selected",
			"no unattended claude -p run: eval and bootstrap need the token",
			"no Verify token button on Welcome",
		},
	},
	{
		ID: Mining,
		Brings: []string{
			"the smine pipeline skills (every smine-* skill but smine-help) and the four skillroutine-* skills join the skill deploy",
			"the routine smine-nightly loads at config server start when Routines is on, unless you suspended it",
		},
		Platforms: []string{goosDarwin, goosWindows},
		Requires:  []ID{Peek, Context},
		Summary:   "Session retrospectives and proposals.",
		Title:     titleMining,
		Without: []string{
			"no session retrospectives, no proposals, no nightly change commits",
			"the smine pipeline and skillroutine-* skills are not deployed",
			"no Proposals page",
		},
	},
}

// Component is one catalog row.
type Component struct {
	ID        ID
	Title     string    // "Permission gate"
	Summary   string    // one line, shown on every surface
	Brings    []string  // what it writes to the machine, one line each
	Without   []string  // what is off or degraded without it, one line each
	Requires  []ID      // hard requirements (concept catalog)
	Platforms []string  // GOOS values; empty = all
	Settings  []Setting // the component's configurable values, in display order
	Fixed     bool      // true only for ConfigServer
}

// inCatalogOrder returns the catalog ids the set holds in display order, dropping ids outside the catalog.
func inCatalogOrder(set map[ID]bool) []ID {
	ids := make([]ID, 0, len(set))
	for _, component := range catalog {
		if set[component.ID] {
			ids = append(ids, component.ID)
		}
	}
	return ids
}

// Available reports whether id can be installed on goos.
func Available(id ID, goos string) bool {
	component, ok := Lookup(id)
	if !ok {
		return false
	}
	return len(component.Platforms) == 0 || slices.Contains(component.Platforms, goos)
}

// Catalog returns every component in display order.
func Catalog() []Component {
	return slices.Clone(catalog)
}

// Close returns ids plus every component they require, transitively.
func Close(ids []ID) []ID {
	closed := map[ID]bool{}
	pending := slices.Clone(ids)
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		if closed[id] {
			continue
		}

		closed[id] = true
		component, _ := Lookup(id)
		pending = append(pending, component.Requires...)
	}
	return inCatalogOrder(closed)
}

// Dependents returns the ids among selected that require id, transitively.
func Dependents(id ID, selected []ID) []ID {
	dependents := map[ID]bool{}
	for _, candidate := range selected {
		isRequiring := candidate != id && slices.Contains(Close([]ID{candidate}), id)
		if isRequiring {
			dependents[candidate] = true
		}
	}
	return inCatalogOrder(dependents)
}

// Full returns every component available on goos.
func Full(goos string) []ID {
	var ids []ID
	for _, component := range catalog {
		if Available(component.ID, goos) {
			ids = append(ids, component.ID)
		}
	}
	return ids
}

// Lookup returns the component with id and whether it exists.
func Lookup(id ID) (Component, bool) {
	for _, component := range catalog {
		if component.ID == id {
			return component, true
		}
	}
	return Component{}, false
}

// Minimal returns the Minimal preset: ConfigServer and Skills.
func Minimal() []ID {
	return []ID{ConfigServer, Skills}
}

// Title returns the catalog entry's title, "" when id is unknown.
func Title(id ID) string {
	component, _ := Lookup(id)
	return component.Title
}
