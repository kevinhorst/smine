# configserver

The local config server: a web UI and HTTP API over the operator's Claude Code and Codex settings, skills, routines, sessions, repos and context. It also runs the one-shot entries the installers use: `-install` applies a selection record through the install engine (`internal/install`) on every OS. Started by the macOS LaunchAgent, the Windows logon task, or `make serve-dev` for a dev instance.

## Usage

Defaults shown as `<…>` are resolved at start from the home dir and the internal packages.

```
Usage of configserver:
  -acdsl-verdicts string
    	acdsl verdict log (default: ~/.claude/acdsl/verdicts.jsonl)
  -addr string
    	listen address (default "127.0.0.1:6001")
  -agentwt string
    	path to the agentwt binary (worktree status and removal) (default "bin/agentwt")
  -allow-remote
    	permit binding a routable (non-loopback) interface; required opt-in since the token then crosses the network in clear text
  -apply
    	apply the stored selection record of this checkout, then exit
  -backlog string
    	path to the backlog store directory (backlog.json); default: the plans-archive store of the checkout the server runs in
  -claude-fragment string
    	repo settings dir: base.json plus components/*.json, composed into the Claude settings template (default "settings/claude_code")
  -claude-home string
    	claude home: passed to the spawned peek-mcp and heartbeat, holds session transcripts (default <~/.claude>)
  -claude-json string
    	path to Claude Code's .claude.json (MCP server names) (default <config.DefaultClaudeJsonPath>)
  -codex-config string
    	path to codex config.toml (default <codex.DefaultPath>)
  -components string
    	with -install: component ids, comma-separated, or minimal | full
  -codex-fragment string
    	repo fragment for the live codex config (default "settings/codex/config.toml")
  -codex-home string
    	codex home passed to the spawned peek-mcp (default <~/.codex>)
  -context string
    	path to the context source root (default "context")
  -credential string
    	file holding the SHA-256 digest of the access token (written by -new-credential) (default <~/.claude/configserver/credential>)
  -dev
    	dev instance (make serve-dev): seed the settings file from the repo fragment, serve without a credential on a loopback address, peek off; refuses a home that holds an operator credential
  -evals string
    	path to the eval results root (default "evals")
  -examples string
    	path to the skill examples root (default "examples")
  -heartbeat string
    	path to the heartbeat binary (fleet status document) (default "bin/heartbeat")
  -init-welcome
    	always show the Welcome nav entry and setup tile, even when all setup checks are green
  -install
    	write the selection record from -components/-skills (else the stored record, Full when absent), apply it, start the server, then exit
  -logfile string
    	append all output to this file (the Windows logon task passes it; the binary is windowsgui-subsystem there, so a console is never attached)
  -heartbeat-port int
    	heartbeat daemon port, stored in the install record by -install (default 6101)
  -migrate-checkout string
    	with -migrate-state: the main checkout the registry and the stores leave (default ".")
  -migrate-state
    	move repos.json into the state dir and place the store files under the store root — from the checkout, else from git history — each only when its target is missing; initialize the plans-archive repo when missing; then exit
  -new-credential
    	generate a new access token, store its digest in -credential, print the token once, then exit
  -peek-bin string
    	peek-mcp binary (resolved via PATH) (default "peek-mcp")
  -peek-control-port int
    	peek-mcp control dashboard port (0 disables the dashboard) (default 42442)
  -peek-port int
    	peek-mcp HTTP port (0 disables peek entirely) (default 4242)
  -peek-start
    	spawn peek-mcp when the port is not serving (default true)
  -peek-version-file string
    	file holding the pinned peek-mcp version (default "PEEK_MCP_VERSION")
  -presentation string
    	path to the per-install presentation profile (missing file = English/developer defaults) (default <presentation.DefaultPath>)
  -presentation-profile string
    	presentation profile template id to install when no profile exists (e.g. de); empty installs none
  -presentation-template string
    	repo template seeding the German casual profile body on first activation (default "settings/claude_code/presentation-profile.de.md")
  -preview
    	with -install: print the steps the apply would run, write nothing
  -refresh-routine-plists
    	reconcile installed routine plists from their templates (env + schedule carried over), then exit
  -repos string
    	path to the repo registry file (default <state dir>/<config.StateRegistryFile>)
  -restart-agent
    	darwin: restart the config server LaunchAgent from its plist after a grace second, then exit — spawned detached by the Profile apply
  -routines string
    	path to the routines directory (default "routines")
  -sessions string
    	path to the sessions directory (default "sessions")
  -settings string
    	path to settings.json (default <config.DefaultPath>)
  -skills string
    	with -install: all | none | chosen:<leaf>,<leaf>
  -skills-confirmed
    	with -install: mark the skill selection confirmed so the terminal installer skips its skill question (=false clears the mark)
  -skills-home string
    	path to the home skills root (default <skills.DefaultHomePath>)
  -skills-repo string
    	path to the repo skills root (default "skills")
  -style string
    	path to the mined per-install style profile (missing file = no learned style yet) (default <presentation.DefaultStylePath>)
  -sync-into string
    	with -sync-skills -sync-variant: root the variant is rendered into (default: the claude skills root)
  -sync-prune
    	with -sync-skills: move marked installed skills the repo does not hold into the trash
  -sync-scripts string
    	path to the sync scripts (default "cmd/sync")
  -sync-skills
    	deploy the repo skills of the checkout holding -sync-scripts into the skill homes, then exit
  -sync-variant string
    	with -sync-skills: render one variant <leaf>=<name>:<disable> instead of the full sync
  -yes
    	with -install: accepted for the installer's argument form; configserver never prompts
```

- One-shot modes, each exiting after its work, checked in this order: `-refresh-routine-plists`, `-new-credential`, `-migrate-state` (one line per file, also for files moved before an error), `-sync-skills` (the entry the smine-orchestrate skill uses), `-restart-agent`, `-apply` (the stored record applied as is — the entry that replaced the retired sync scripts), `-install`.
- Without a one-shot flag the binary serves.

## How it works

**Start-up**
- `-logfile` redirects before the first output: fmt prints, log and child stderr all land in one append-only file — the launchd `StandardOutPath` analog for the windowsgui logon task.
- Machine state lives under the Claude home of `$HOME`, the dir every binary resolves; a dev instance's sandbox `HOME` holds its own.
- Binding a routable interface exposes every mutating and exec endpoint to LAN peers, so anything but a loopback bind needs `-allow-remote`. An empty host binds every interface and counts as routable; `localhost` and loopback IPs are loopback; an unresolved hostname counts as routable so the gate errs safe. A dev instance never binds a routable address.
- The listener binds before any other start-up work: a busy port fails at once, and port 0 gets its kernel-assigned number for the back link and the listening line.
- peek-mcp is spawned when its port is not serving; with `-peek-start=false` an externally started peek is reused only when it belongs to this profile. A missing version file disables the MCP version panel.
- With Routines selected in the stored record (an unreadable record counts as selected): on darwin a new login session starts with an empty gui domain, so the routines that should be running are reloaded; on Windows the task store is reconciled with the routines directory instead. Deferred routine reloads left from the previous server lifetime (a config save during a run that outlived the server) apply right after.
- launchd stops agents with SIGTERM; Ctrl-C stays for manual runs. Windows never delivers SIGTERM, and Task Scheduler's End is a hard kill.

**Credential**
- Every request must carry the access token whose SHA-256 digest is stored in `-credential`; `-new-credential` replaces the digest and prints the token once — the only moment the token exists outside the operator's hands. A credential loaded from a file follows that file: the running server re-reads the digest when its modification time moves, and an unreadable or invalid file keeps the last valid digest.
- A dev instance has no credential: its scratch home is readied instead (the composed settings template — base plus every fragment, markers expanded — is written in once; an existing settings file is the dev instance's own state) and every request is served. It refuses a home that holds an operator credential, so `-dev` can never serve an operator's instance without one.

**Install (`-install`)**
- The one install flow of every OS; `install.sh`, `install.ps1` and `smine-setup.exe` reach it through the Go installer (`cmd/installer`). The record is the stored one (Full when absent) with `-components` and `-skills` applied; `-preview` prints the engine's plan and writes nothing.
- Steps: creates the access credential on first install; runs the platform preparation; moves the state and stores out of the checkout (`-migrate-state`); writes the explicitly passed ports and `-init-welcome` into the install record's settings (`config-server.port`, `peek.port`, `peek.control_port`, `heartbeat.port`, `config-server.init_welcome`; `config-server.server_path` stays an override the user sets, empty meaning the applying process's `PATH`); prints `orphan row kept: <key>` for every skill row no checkout skill or provider declares (nothing is removed); applies the record through the install engine; installs the selected presentation profile template only when `~/.claude/context/global/presentation-profile.md` does not exist (`kept: <path> (exists)` otherwise; empty id installs none); runs the platform finish.
- macOS and Linux: the credential token is printed once. The finish always restarts the LaunchAgent on darwin — the installer rebuilt `bin/configserver`, and a loaded agent keeps serving the binary it started with; Linux has no autostart and prints the start command.
- Windows preparation: relaunches itself elevated when the token is not (ShellExecuteW verb `runas`, original args plus `-logfile %TEMP%\smine-install.log`), waits for exit and replays the child's log, a declined UAC prompt being fatal; locates Git Bash and runs the git bootstrap; appends the shim dir to `HKCU\Environment` Path (case-insensitive segment match, value type preserved), broadcasts `WM_SETTINGCHANGE` and prepends it to the process PATH; makes `claude` callable for routines — a CLI visible from Git Bash wins, otherwise Claude Desktop's bundled `claude.exe` gets the repo's bash shim, neither found being a warning; with peek selected, refreshes `peek-mcp.exe` in the shim dir from `SMINE_PEEK_BIN`, a sibling peek-mcp checkout, or an installer-bundled copy. The credential token shows in a message box — the binary is windowsgui, and the install log must never hold the token.
- Windows finish: takes over the port with a try-bind (a held port gets one stop of the smine server task and a retry, a still-held port is fatal); registers `\smine\configserver` — logon trigger, bounded restart-on-crash, no time limit, single instance, port flags in the arguments, peek port 0 without peek — bound to the current user with `-User`, then fires the first run; waits for the port to answer HTTP within a generous window; with Routines or Mining selected, prints the routine-token guidance when the token is missing. Routine sync stays the server task's boot sync: a second concurrent sync races it on `Register-ScheduledTask`.
- `-apply` applies the stored record as is and, building nothing, restarts the server only when its plist changed or it is not loaded; `-restart-agent` waits a grace second for the spawning server, then reloads its LaunchAgent from the plist on disk.

## Files

- `main.go` — flags, one-shot modes, start-up and serving
- `install.go` — the record, preview, apply and install flow shared by every OS
- `install_windows.go` — the Windows preparation and finish
- `install_other.go` — the macOS and Linux credential, preparation and finish
- `version.go` — the build version string
