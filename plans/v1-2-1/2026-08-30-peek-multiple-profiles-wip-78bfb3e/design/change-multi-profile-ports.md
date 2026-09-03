# Peek Multiple Profiles — Change Plan

route: `change`, mode: `familiar`, repos: `peek-mcp` + `smine`

## TLDR

- Two macOS profiles on one Mac share the loopback port space. Both smine installs spawn peek on the same defaults (MCP 4242, control/OTLP 42442), so the first profile's peek wins and the second profile's configserver silently reuses it — a peek reading the *wrong* user's `~/.claude`/`~/.codex`. That is "peek is only started once", "can't change the paths", and the mining backlog in one mechanism.
- A second defect keeps it wrong forever: `startPeek` skips spawning whenever *anything* listens on the port, and reinstalls never kill the running peek — a stale peek from July (old flags, old binary) survives every reinstall.
- Fix in smine: per-profile peek ports (`PEEK_PORT`/`PEEK_CONTROL_PORT` envs, persisted in a gitignored `install.env`, baked into the LaunchAgent plist), explicit `--claude-home`/`--codex-home` on the spawned peek with configserver-level overrides, stale-peek kill at install, and an identity check before reusing a running peek.
- Fix in peek-mcp: a `GET /healthz` identity endpoint on the MCP HTTP listener, exact-bind (no port walk) when `--control-port` is explicitly set, and `peek-mcp setup` writing the OTLP endpoint from the configured control port instead of the hardcoded 42442. New release; smine bumps its pin.
- The Claude Code telemetry endpoint in the synced settings becomes per-profile via a `{{PEEK_CONTROL_PORT}}` marker expanded from `install.env`.

## Context

- Two profiles on this Mac: `kevinpersonal` (configserver :6001, peek :4242/:42442) and `kevin_aqms_mac` (configserver :6002, peek :4243 — a stale process running since 2026-07-23 with pre-1.2 flags, no `--claude-home`/`--codex-home`/`--control-port`).
- peek is spawned by the smine configserver, not by install.sh: [main.go:206](../../GolandProjects/smine/cmd/configserver/main.go) `startPeek` — dial check, then `peek-mcp start --transport http --port N --control-port M` with **no home flags**.
- install.sh parameterizes only the configserver port (`CONFIGSERVER_PORT`, [install.sh:5](../../GolandProjects/smine/install.sh)); peek ports are unreachable from the macOS install path (Windows `install.ps1` already has `-PeekPort`/`-PeekControlPort` params — the macOS path is the outlier).
- peek-mcp already supports `--claude-home`/`--codex-home`/`--port`/`--control-port` with `PEEK_*` env fallbacks ([cmd/start.go:271-283](cmd/start.go)); the control port walks 42442–42499 ([cmd/listen.go:11-32](cmd/listen.go)); `setup` hardcodes the OTLP export endpoint to base 42442 ([cmd/setup.go:218](cmd/setup.go)).
- The per-session stdio peeks (Claude/Codex MCP fragments) already carry `{{HOME}}`-expanded homes per profile and are NOT part of the problem — only the configserver-spawned HTTP peek and the OTLP settings are.

## Drivers

| ID | Observed | Wanted | Impact | Origin |
|---|---|---|---|---|
| DR1 | Profile B's configserver finds :4242 busy (profile A's peek) and reuses it — B's mining/UI reads A's sessions | Each profile runs its own peek on its own ports, serving its own homes | behavioral | user report + live `lsof`/`ps` on this Mac |
| DR2 | Spawned peek gets no `--claude-home`/`--codex-home`; no way to point it elsewhere | Homes passed explicitly, overridable per install | behavioral | user report ("can't change the codex and claude paths") |
| DR3 | Reinstall replaces binaries but never restarts a running peek — stale July process survives (`peek-mcp … --port 4243`, started 23 Jul) | Reinstall deterministically kills and respawns this profile's peek with current binary and flags | behavioral | live `ps` on `kevin_aqms_mac` |
| DR4 | `OTEL_EXPORTER_OTLP_ENDPOINT` is 42442 for every profile ([settings.json:6](../../GolandProjects/smine/settings/claude_code/settings.json)); whichever peek holds 42442 gets both profiles' telemetry | Per-profile control port in the synced settings | contract-touching | settings fragment + peek control-port walk |
| DR5 | Work-profile sessions unmined; backlog | Mining works on the work profile (consequence of DR1/DR2) | behavioral | user report |

## Scope

- **In**
  - **smine**: install.sh peek-port/home parameterization + `install.env` persistence, plist template args, `startPeek` identity check + home flags, stale-peek kill at install, `{{PEEK_CONTROL_PORT}}` marker in the settings fragment + expansion in `sync_settings.sh` and the config server's fragment compare/revert, peek pin bump, README/AGENTS docs.
  - **peek-mcp**: `GET /healthz` identity endpoint, exact-bind on explicitly set `--control-port`, `setup --control-port` flag driving the OTLP endpoint, docs, version bump + release tag.
- **Out (non-goals)**
  - **stdio per-session peeks**: already per-profile via `{{HOME}}`; untouched.
  - **Claude Desktop mcpb bundle**: untouched (hardcodes `${HOME}` defaults, correct per profile).
  - **Windows Inno/installer flow**: already port-parameterized; only `runInstall` gains the `install.env` write so sync scripts see the ports.
  - **Backlog mining itself**: operational — run smine on the work profile after the fix; not code.
- **Not changed**
  - **Default single-profile install**: no envs → 4242/42442/`$HOME` exactly as today.
  - **configserver `-peek-*` flag defaults and the `internal/peek` MCP client**.
- **Deferred findings**
  - **Welcome check gap**: welcome.go verifies the stdio fragment but not that the live OTLP endpoint matches this profile's peek control port — worth a setup check later.
  - **`Peek_MCP` vs `peek-mcp` tool-namespace split**: skills call `mcp__Peek_MCP__*` while the synced registration is `peek-mcp`; both are allowlisted, source of the capitalized registration not in either repo — pre-existing, untouched.
  - **docs/reference.md says `--poll-interval` default `1s`, code says `5s`** ([cmd/start.go:277](cmd/start.go)) — doc bug, unrelated.
  - **rsync deploy mirror (smine → claude-configs, `.claude/settings.local.json`)**: verify it does not delete untracked `install.env`/`repos.json` in the mirror target; if it uses `--delete`, add excludes.

## Assumptions

| Assumption | Reality | Location |
|---|---|---|
| "Peek is only started once" = a launchd/lock singleton | No lock exists; it is the dial-check in `startPeek` + shared loopback across profiles | [main.go:208-213](../../GolandProjects/smine/cmd/configserver/main.go) |
| "Reinstalling doesn't change it" = install bug | Correct: install.sh never touches a running peek, and the pin `@v1.2.0` also freezes the binary version | [install.sh:26-31](../../GolandProjects/smine/install.sh) |
| "Can't change claude/codex paths" = missing peek capability | peek has the flags/env since 1.2; smine never passes them to the HTTP peek | [cmd/start.go:275-276](cmd/start.go), [main.go:215-218](../../GolandProjects/smine/cmd/configserver/main.go) |
| Ports 6001/6002 are peek | They are the configserver; peek is 4242/4243 + 42442 | live `lsof` + plist |

## Current state

- [smine/cmd/configserver/main.go:44-47,203-233](../../GolandProjects/smine/cmd/configserver/main.go) — `-peek-bin/-peek-port/-peek-control-port/-peek-start` flags; `startPeek` dial-check + spawn without homes. (F1)
- [smine/install.sh:5,26-31,85-95](../../GolandProjects/smine/install.sh) — `CONFIGSERVER_PORT` only; pin `peek-mcp@v1.2.0`; plist sed with `__REPO_DIR__/__PORT__/__HOME__/__PATH__/__INIT_WELCOME__`. (F2)
- [smine/cmd/configserver/com.smine.configserver.plist.template](../../GolandProjects/smine/cmd/configserver/com.smine.configserver.plist.template) — ProgramArguments carry no peek args. (F3)
- [smine/cmd/sync/sync_settings.sh:12-20](../../GolandProjects/smine/cmd/sync/sync_settings.sh) — expands `{{PEEK_MCP}}`/`{{HOME}}`; syncs `settings/claude_code/settings.json` (with the OTLP env block, endpoint hardcoded `:42442`) to `~/.claude/settings.json`. (F4)
- [smine/internal/server/config_sync.go:37-80,97-109](../../GolandProjects/smine/internal/server/config_sync.go) — fragment↔live compare (`sectionOverridden`) and verbatim revert copy (`copyFileAtomic`); any marker left unexpanded here would show as permanent drift and clobber the live value on revert. (F5)
- [peek-mcp/cmd/start.go:244-262](cmd/start.go) — HTTP transport serves only the MCP handler; no identity/health route. (F6)
- [peek-mcp/cmd/listen.go:11-32](cmd/listen.go) + [cmd/start.go:162](cmd/start.go) — control port walks 42442–42499 unconditionally; a restarting peek can land on a neighbor profile's base and swallow its telemetry. (F7)
- [peek-mcp/cmd/setup.go:213-220](cmd/setup.go) — OTLP endpoint written from `controlPortBase`, never from configuration. (F8)
- [smine/cmd/configserver/install_windows.go:72,197-201](../../GolandProjects/smine/cmd/configserver/install_windows.go) — Windows already passes `-peek-port/-peek-control-port` into the scheduled task. (F9)
- Live: `kevin_aqms_mac` peek pid 17197 running since 23 Jul with pre-flags invocation; `kevinpersonal` peek 4242/42442 spawned by configserver. (F10)

## Target state

```
per profile (macOS user account)
  install.env (gitignored, REPO_DIR)        ← single source of per-install values
    CONFIGSERVER_PORT / PEEK_PORT / PEEK_CONTROL_PORT [/ CLAUDE_HOME / CODEX_HOME]
      ├─ install.sh → plist ProgramArguments (-addr, -peek-port, -peek-control-port [, -claude-home, -codex-home])
      ├─ sync_settings.sh → {{PEEK_CONTROL_PORT}} in settings fragment → ~/.claude/settings.json OTLP endpoint
      └─ configserver (config_sync) → expands the same marker for compare/revert
  configserver :600N
    └─ startPeek: GET /healthz on :PEEK_PORT
         no listener            → spawn peek with --port/--control-port/--claude-home/--codex-home
         identity match         → reuse
         mismatch / not peek    → loud error, peek endpoint disabled (degraded UI, never wrong data)
  peek :PEEK_PORT (/mcp + /healthz), control :PEEK_CONTROL_PORT (exact bind, no walk)
```

**Principles**: single source of truth (`install.env` feeds plist, sync, and server expansion — no value is derived twice); degrade visibly over serving foreign data (identity mismatch disables the peek endpoint instead of silently reading another user's home); platform-native mechanism (launchd plist stays the process manager; peek's own flags carry the config).

## Behavior contract

- Must not change: default install with no envs (ports 4242/42442, homes `$HOME/.claude`/`$HOME/.codex`); stdio MCP registrations for Claude/Codex; configserver HTTP API; peek MCP tool surface; peek stdio transport behavior.
- Intentional changes: `startPeek` no longer reuses an arbitrary listener (DR1/DR3); spawned peek gets explicit home flags (DR2); synced `~/.claude/settings.json` OTLP endpoint becomes per-profile (DR4); explicitly-set `--control-port` fails instead of walking (DR4); reinstall kills this profile's stale peek (DR3).

## Decisions

| ID | Problem | Facts | Decision | Why |
|---|---|---|---|---|
| D1 | How per-profile peek ports are chosen | F2, F9 | [USER] Explicit `PEEK_PORT`/`PEEK_CONTROL_PORT` envs on install.sh, persisted to `install.env` | Mirrors the existing Windows `install.ps1` parameters (nearest in-repo pattern) and stays debuggable — the ports are literal in `install.env` and the plist |
| D2 | What to do when the peek port is held by a foreign/unidentifiable process | F1, F10 | Log a loud error naming holder and expected identity; leave `PeekEndpoint` empty so the UI degrades to "session column degraded" | Serving another profile's sessions is silent data corruption; degraded-but-visible matches the existing spawn-failure path ([main.go:223](../../GolandProjects/smine/cmd/configserver/main.go)). Hard-failing the configserver would take the whole config UI down over a peek problem |
| D3 | Where the per-profile control port reaches the settings fragment | F4, F5 | `{{PEEK_CONTROL_PORT}}` marker in the fragment; expansion in `sync_settings.sh` AND in the config server's fragment load for compare/revert (shared helper reading `install.env`, default 42442) | The fragment is compared and revert-copied verbatim (F5); an unexpanded marker means permanent drift badges and revert clobbering the live port. Marker precedent exists (`{{PEEK_MCP}}`/`{{HOME}}` in the mcp fragment) |
| D4 | Control-port walking vs per-profile bases | F7 | peek: when `--control-port` is explicitly set (flag or `PEEK_CONTROL_PORT` — `flags.Changed` covers both), bind exactly or exit with error; default invocation keeps the walk | A walking peek that restarts can land on the neighbor profile's base and swallow its telemetry — undebuggable. Explicit config means the operator owns the port; failing loudly beats binding the wrong one |
| D5 | How the spawned peek learns its homes | F1 | configserver gains `-claude-home`/`-codex-home` flags (defaults `os.UserHomeDir()`-derived), always passed to the spawn as `--claude-home`/`--codex-home`; overridable via `install.env` → plist args | Explicit beats inherited defaults: the running process's flags become self-documenting in `ps`, and the override answers DR2 without peek-side changes |
| D6 | Stale peek across reinstalls | F2, F10 | install.sh kills same-user listeners on `PEEK_PORT` and `PEEK_CONTROL_PORT` before bootstrapping the agent (mirroring the existing :6001 kill at [install.sh:113-119](../../GolandProjects/smine/install.sh)); no restart API, no version negotiation in `startPeek` | Install is the only update vector (the pin changes there), so install-time kill fully covers freshness with zero new runtime machinery. A same-profile pre-healthz peek then reads as "mismatch" until reinstall — self-healing |
| D7 | Identity endpoint shape | F6 | `GET /healthz` on the MCP HTTP listener returning `{"version","claudeHome","codexHome","controlPort"}`, no auth (loopback-only, non-sensitive) | The MCP port is what `startPeek` dials; putting identity on the control port would leave the check racing the walk. Homes are the identity that matters (DR1) |
| D8 | peek version pin | F2 | Bump `install.sh` pin to the new release tag (v1.2.2) after tagging peek-mcp | Repo pattern is a pinned version; healthz is a hard dependency of the new `startPeek` |
| D9 | `install.env` semantics | F2 | install.sh sources `install.env` first (existing values are the defaults), env vars override, then writes the resolved values back; `runInstall` (Windows) writes the same file from its params | Re-running `./install.sh` with no envs must keep the profile's ports — otherwise every routine reinstall resets profile B to colliding defaults, recreating DR1 |
| D10 | `setup` OTLP endpoint | F8 | `peek-mcp setup` gains `--control-port` (default `controlPortBase`), used in the settings env block and the prompt text | Standalone peek users on a second profile hit the same hardcode; one flag closes it |

## Open questions

Empty — Q1 answered by the user (D1: explicit envs).

## Baseline (verified)

N/A — change route (facts live in Current state).

## Exemplar & reuse

N/A — change route. Mirrors are named per Changes entry; cross-cutting reuse: the existing `:6001` stale-kill loop ([install.sh:113-119](../../GolandProjects/smine/install.sh)) is the pattern for the peek-port kills; `listenLoopback` ([cmd/listen.go](cmd/listen.go)) already takes a range and needs no new mechanism for exact-bind.

## Changes

### Phase 1 — peek-mcp (shippable alone; release v1.2.2)

**[cmd/start.go](cmd/start.go)** — healthz + exact-bind. mirrors: existing control-server route wiring.

```go
// http case, replacing lines 244-252
httpSrv := server.NewStreamableHTTPServer(srv)

mux := http.NewServeMux()
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"version":     Version(),
		"claudeHome":  claudeHome,
		"codexHome":   codexHome,
		"controlPort": boundControlPort,
	})
})
mux.Handle("/", requestLogger(httpSrv))

addr := fmt.Sprintf("127.0.0.1:%d", port)
slog.Info("peek-mcp listening", "addr", fmt.Sprintf("http://%s/mcp", addr))

httpServer := &http.Server{Addr: addr, Handler: mux}
```

`boundControlPort` is the existing `boundPort` hoisted out of the `if controlPort > 0` block (0 when the control server is disabled). Exact-bind at line 162:

```go
walkEnd := controlPort + controlPortSpan - 1
if flags.Changed("control-port") {
	walkEnd = controlPort
}
controlLn, err := listenLoopback(controlPort, walkEnd)
```

(`applyEnvFallbacks` sets flags via `flags.Set`, so `Changed` covers `PEEK_CONTROL_PORT` too — [cmd/start.go:366-381](cmd/start.go).)

**[cmd/setup.go](cmd/setup.go)** — `--control-port` flag (default `controlPortBase`) registered next to `--control-server` (lines 51-57); [line 218](cmd/setup.go) and the prompt string at line 90 use it instead of `controlPortBase`.

**[docs/reference.md](docs/reference.md)** — document `/healthz`, exact-bind semantics, `setup --control-port`.

**[Makefile:5](Makefile)** — `VERSION = 1.2.2`; tag `v1.2.2` and push (release step, gated below).

### Phase 2 — smine (depends on the v1.2.2 tag for the pin; each file change is small)

Working copies: Phase 1 edits happen in this session's peek-mcp worktree. Phase 2 edits happen in `/Users/kevinpersonal/GolandProjects/claude-configs` — create a fresh git worktree + `claude/*` branch there before the first edit ([USER]); never edit its main checkout directly.

**[cmd/configserver/main.go](../../GolandProjects/smine/cmd/configserver/main.go)** — home flags + identity check. Hot item (guard logic) — full replacement of `startPeek` below:

```go
claudeHome := flag.String("claude-home", filepath.Join(homeDir, ".claude"), "claude home passed to the spawned peek-mcp")
codexHome := flag.String("codex-home", filepath.Join(homeDir, ".codex"), "codex home passed to the spawned peek-mcp")
```

(`homeDir` from `os.UserHomeDir()` at the top of `main`, fatal on error — same treatment as existing `filepath.Abs` fatals.) Call site: `startPeek(ctx, *peekBin, *peekPort, *peekControlPort, *claudeHome, *codexHome)` returning `ok bool`; when `!ok`, `endpoint` and `dashboardURL` are reset to `""` so the UI degrades (D2).

```go
// startPeek spawns peek-mcp with this profile's homes. A listener on the
// port is reused only when its /healthz identity matches — on this shared
// loopback another macOS profile's peek is reachable on the same port, and
// reusing it would serve that user's sessions (D2). Mismatch or an
// unidentifiable holder disables the peek integration for this run.
func startPeek(ctx context.Context, bin string, port, controlPort int, claudeHome, codexHome string) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if id, err := peekIdentity(addr); err == nil {
		if id.ClaudeHome == claudeHome && id.CodexHome == codexHome {
			log.Printf("peek-mcp %s already serving on %s for this profile, not spawning", id.Version, addr)
			return true
		}
		log.Printf("ERROR: peek-mcp on %s serves claude-home=%s codex-home=%s, want %s / %s — another profile's peek holds this port; set PEEK_PORT/PEEK_CONTROL_PORT and reinstall (session column disabled)", addr, id.ClaudeHome, id.CodexHome, claudeHome, codexHome)
		return false
	} else if conn, dialErr := net.DialTimeout("tcp", addr, time.Second); dialErr == nil {
		conn.Close()
		log.Printf("ERROR: %s is in use but not an identifiable peek-mcp (%v) — stale pre-1.2.2 peek or foreign process; reinstall to replace it (session column disabled)", addr, err)
		return false
	}

	args := []string{"start", "--transport", "http", "--port", strconv.Itoa(port),
		"--claude-home", claudeHome, "--codex-home", codexHome}
	if controlPort != 0 {
		args = append(args, "--control-port", strconv.Itoa(controlPort))
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stderr = os.Stderr
	shell.HideWindow(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("peek-mcp spawn failed (session column degraded): %v", err)
		return false
	}

	log.Printf("peek-mcp spawned (pid %d) on http://%s/mcp", cmd.Process.Pid, addr)
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("peek-mcp exited: %v", err)
		}
	}()
	return true
}

type peekID struct {
	Version     string `json:"version"`
	ClaudeHome  string `json:"claudeHome"`
	CodexHome   string `json:"codexHome"`
	ControlPort int    `json:"controlPort"`
}

func peekIdentity(addr string) (*peekID, error) {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("healthz status %d", resp.StatusCode)
	}
	var id peekID
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return nil, err
	}
	return &id, nil
}
```

**[install.sh](../../GolandProjects/smine/install.sh)** —
- After `REPO_DIR`: source `"$REPO_DIR/install.env"` if present, then `ADDR_PORT="${CONFIGSERVER_PORT:-${ADDR_PORT:-6001}}"`, `PEEK_PORT="${PEEK_PORT:-4242}"`, `PEEK_CONTROL_PORT="${PEEK_CONTROL_PORT:-42442}"` (env overrides file, file overrides default — D9), and write the resolved trio (plus `CLAUDE_HOME`/`CODEX_HOME` when set) back to `install.env`.
- Pin bump: `go install github.com/kevinhorst/peek-mcp@v1.2.2`.
- Plist sed gains `-e "s|__PEEK_PORT__|$PEEK_PORT|g" -e "s|__PEEK_CONTROL_PORT__|$PEEK_CONTROL_PORT|g"` (and home overrides when set — appended args, see plist note).
- Before `launchctl bootstrap`, kill stale same-user peek listeners on `$PEEK_PORT` and `$PEEK_CONTROL_PORT` — same loop as the existing `:6001` kill at lines 113-119 (`lsof -ti` only sees own-user processes, so a foreign profile's peek is never killed — it is reported by the D2 check instead).

**[cmd/configserver/com.smine.configserver.plist.template](../../GolandProjects/smine/cmd/configserver/com.smine.configserver.plist.template)** — ProgramArguments gain `<string>-peek-port</string><string>__PEEK_PORT__</string><string>-peek-control-port</string><string>__PEEK_CONTROL_PORT__</string>`. Home overrides: install.sh injects `-claude-home`/`-codex-home` arg pairs only when set in `install.env` (sed placeholder `__PEEK_HOME_ARGS__` expanded to the pairs or to nothing — keeps the default plist free of redundant args).

**[settings/claude_code/settings.json](../../GolandProjects/smine/settings/claude_code/settings.json)** — line 6: `"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"`.

**[cmd/sync/sync_settings.sh](../../GolandProjects/smine/cmd/sync/sync_settings.sh)** — source `install.env` (script-relative `../..`; default 42442 when absent) and add `{{PEEK_CONTROL_PORT}}` to the existing marker expansion where `settings.json` is synced. mirrors: the `{{PEEK_MCP}}`/`{{HOME}}` expansion in the same script.

**[internal/server/config_sync.go](../../GolandProjects/smine/internal/server/config_sync.go) + [internal/config](../../GolandProjects/smine/internal/config)** — marker expansion at the two fragment touchpoints (D3): a helper `config.ExpandInstallMarkers(data []byte) []byte` reading `install.env` from the working directory (parse `KEY=VALUE` lines; missing file → defaults), applied in `copyFileAtomic` when the source is the claude fragment (revert path) and in `sectionOverridden`/`handleConfig`'s fragment loads before compare. mirrors: `config.Load` usage at [config_sync.go:98](../../GolandProjects/smine/internal/server/config_sync.go).

**[cmd/configserver/install_windows.go](../../GolandProjects/smine/cmd/configserver/install_windows.go)** — `runInstall` writes `install.env` (same keys) into the repo dir so `sync_settings.sh` finds the ports on Windows too.

**[.gitignore](../../GolandProjects/smine/.gitignore)** — add `/install.env`.

**[README.md](../../GolandProjects/smine/README.md)** — multi-profile section: per-profile env example (`CONFIGSERVER_PORT=6002 PEEK_PORT=4243 PEEK_CONTROL_PORT=42542 ./install.sh`), the identity-check behavior, and the note that reinstall is the peek update vector.

## Hot items

`startPeek` (guard logic — weakened/replaced guard): full example implementation written out in Changes above for approval. No goroutines, interfaces, or generated formats are added; `peekID` is a named struct (no anonymous structs).

## Tests

| Location.Method | Cases | Comment |
|---|---|---|
| peek-mcp `cmd/start_test.go` (new, mirrors existing cmd tests if present, else a httptest-level test of the mux) | healthz returns version+homes+controlPort; unknown path still reaches MCP handler | |
| peek-mcp `cmd/listen_test.go` | explicit control-port with occupied port → error (no walk); default → walks to next free | extend existing listen tests if present |
| smine `cmd/configserver/main_test.go` (new, mirrors table-driven style of `internal/server` tests) | peekIdentity: healthy match → reuse; home mismatch → false; non-peek listener → false; no listener → spawn path (bin=`/bin/echo` stub) | table-driven over a httptest server serving canned /healthz |
| smine `internal/config/expand_test.go` (new) | marker expanded from install.env; missing file → default 42442; unknown markers untouched | |
| smine `internal/server/config_sync_test.go` | sectionOverridden with marker fragment + expanded live file → no drift; revert writes expanded value | extend existing tests at [config_sync_test.go:22-35](../../GolandProjects/smine/internal/server/config_sync_test.go) |
| not tested: install.sh env plumbing (shell) — covered by the runbook/verification on both profiles; launchd interaction untestable in unit scope | | |

## Test runbook

- **healthz identity** — `curl http://127.0.0.1:4242/healthz` (default profile) shows this profile's homes and version 1.2.2.
- **second-profile spawn** — on `kevin_aqms_mac`: reinstall with `CONFIGSERVER_PORT=6002 PEEK_PORT=4243 PEEK_CONTROL_PORT=42542`, then `curl :4243/healthz` shows that profile's homes; `session_list` via configserver UI shows that profile's sessions.
- **foreign-port refusal** — from profile B, point `-peek-port` at profile A's live port: configserver log carries the D2 ERROR, session column degraded.
- **telemetry routing** — each profile's `~/.claude/settings.json` OTLP endpoint carries its own control port; peek dashboard on each profile shows only its own sessions' telemetry.
- **reinstall freshness (DR3)** — with peek running, rerun install.sh: old pid gone, new pid with `--claude-home` flags visible in `ps`.

## Contracts & sweeps

| Contract | Sides | Sweep |
|---|---|---|
| `/healthz` JSON shape | peek-mcp start.go ↔ smine `peekIdentity` | field names identical both repos; grep `healthz` in both |
| `install.env` keys | install.sh ↔ sync_settings.sh ↔ `config.ExpandInstallMarkers` ↔ install_windows.go | grep `PEEK_CONTROL_PORT\|PEEK_PORT\|install.env` to confirm every reader/writer uses the same names |
| `{{PEEK_CONTROL_PORT}}` marker | settings fragment ↔ sync_settings.sh ↔ config_sync expansion | grep `{{PEEK_CONTROL_PORT}}` — exactly fragment + two expanders + docs |
| plist placeholders | plist.template ↔ install.sh sed | grep `__PEEK` in both files |
| pin `@v1.2.2` | peek-mcp tag ↔ install.sh ↔ README | grep `v1\.2\.0` to zero in smine |
| OTLP endpoint literal `42442` | settings fragment ↔ peek setup ↔ docs | grep `42442` both repos; survivors only as *defaults* (flag defaults, `controlPortBase`, docs describing defaults) — justified per hit |

## Verification

- [ ] Phase 1: `make -C peek-mcp test` green; `curl /healthz` against `make serve-http` returns the identity JSON; explicit `--control-port` on an occupied port exits with error while default walks.
- [ ] Tag v1.2.2 pushed; `go install github.com/kevinhorst/peek-mcp@v1.2.2` resolves (stop condition S8 until then).
- [ ] Phase 2: `make -C smine build && go test ./...` green in smine.
- [ ] Reinstall on `kevinpersonal` with no envs: ports unchanged (4242/42442), peek respawned with home flags visible in `ps`, `install.env` written with defaults.
- [ ] Reinstall on `kevin_aqms_mac` with `CONFIGSERVER_PORT=6002 PEEK_PORT=4243 PEEK_CONTROL_PORT=42542`: stale July peek replaced, healthz shows that profile's homes, configserver UI session column lists that profile's sessions (DR1/DR5 observed fixed in the running system).
- [ ] Both profiles' `~/.claude/settings.json` carry their own OTLP port; config server settings page shows no drift badge for the env section.
- [ ] `rerun ./install.sh` a second time on profile B with no envs → ports persist from `install.env` (D9 observed).
- [ ] Rsync deploy mirror run once → `install.env` in claude-configs intact (deferred-finding check).

## Stop conditions

| ID | Condition | Action |
|---|---|---|
| S1 | An approved signature/contract can't hold as planned | Stop and report — never improvise architecture mid-edit |
| S2 | Second failed fix on the same mechanism | Stop, research the actual cause, redesign — no third band-aid |
| S3 | Missing prerequisite (generated code, running infra) | Run the producing step; if infrastructure is down, ask |
| S4 | Discovered work materially exceeds approved scope | Ask before continuing |
| S5 | Same kind of bug found twice: in own diff → fix all in diff; pre-existing outside → report and ask | Sweeps are the user's call |
| S6 | Structural obstacle tempts a new abstraction | Stop and report; relocate, don't wrap |
| S7 | The config server's fragment compare/revert has more marker touchpoints than the two identified (F5) | Stop and report before spreading expansion further |
| S8 | v1.2.2 tag not yet published when Phase 2 starts | Implement Phase 2 against a locally-built peek (`go build`), leave the pin bump as the last commit, and report that the release must be pushed before install on other machines |
| S9 | The rsync mirror deletes untracked files in claude-configs | Stop and report — the exclude list edit is in user-local settings, not the repo |
