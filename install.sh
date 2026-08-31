#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

# Per-install values: install.env carries the previous install's ports and
# home overrides, environment variables override the file, defaults fill the
# rest — and the resolved values are written back, so a plain re-run keeps
# this profile's ports instead of resetting to the colliding defaults (D9).
ENV_CONFIGSERVER_PORT="${CONFIGSERVER_PORT:-}"
ENV_PEEK_PORT="${PEEK_PORT:-}"
ENV_PEEK_CONTROL_PORT="${PEEK_CONTROL_PORT:-}"
ENV_PEEK_CLAUDE_HOME="${PEEK_CLAUDE_HOME:-}"
ENV_PEEK_CODEX_HOME="${PEEK_CODEX_HOME:-}"
if [ -f "$REPO_DIR/install.env" ]; then
  # shellcheck source=/dev/null
  . "$REPO_DIR/install.env"
fi
ADDR_PORT="${ENV_CONFIGSERVER_PORT:-${CONFIGSERVER_PORT:-6001}}"
PEEK_PORT="${ENV_PEEK_PORT:-${PEEK_PORT:-4242}}"
PEEK_CONTROL_PORT="${ENV_PEEK_CONTROL_PORT:-${PEEK_CONTROL_PORT:-42442}}"
PEEK_CLAUDE_HOME="${ENV_PEEK_CLAUDE_HOME:-${PEEK_CLAUDE_HOME:-}}"
PEEK_CODEX_HOME="${ENV_PEEK_CODEX_HOME:-${PEEK_CODEX_HOME:-}}"

{
  echo "CONFIGSERVER_PORT=$ADDR_PORT"
  echo "PEEK_PORT=$PEEK_PORT"
  echo "PEEK_CONTROL_PORT=$PEEK_CONTROL_PORT"
  if [ -n "$PEEK_CLAUDE_HOME" ]; then
    echo "PEEK_CLAUDE_HOME=$PEEK_CLAUDE_HOME"
  fi
  if [ -n "$PEEK_CODEX_HOME" ]; then
    echo "PEEK_CODEX_HOME=$PEEK_CODEX_HOME"
  fi
} > "$REPO_DIR/install.env"

INSTALL_PEEK=1
INSTALL_SERENA=0
RUN_SYNC=1
INIT_WELCOME=false
for arg in "$@"; do
  case "$arg" in
    --no-peek)      INSTALL_PEEK=0 ;;
    --serena)       INSTALL_SERENA=1 ;;
    --no-sync)      RUN_SYNC=0 ;;
    --init-welcome) INIT_WELCOME=true ;;
    *)
      echo "usage: $0 [--no-peek] [--serena] [--no-sync] [--init-welcome]" >&2
      exit 1
      ;;
  esac
done

bash "$REPO_DIR/cmd/sync/ensure_git_repo.sh" "$REPO_DIR"

# Warn-only dependency check — the acdsl gates are the enforcement, this is
# the early warning (a missing shellcheck reddens every acdsl check).
for dep in jq shellcheck; do
  command -v "$dep" >/dev/null 2>&1 || echo "WARN: $dep not found — install it (brew install $dep) before the first mining run"
done

if [ "$INSTALL_PEEK" = 1 ]; then
  echo "-> Installing peek-mcp ..."
  go install github.com/kevinhorst/peek-mcp@v1.2.2
else
  echo "skip: peek-mcp (--no-peek)"
fi

if [ "$INSTALL_SERENA" = 1 ]; then
  echo "-> Installing serena ..."
  uv tool install -p 3.13 --upgrade serena-agent
else
  echo "skip: serena (opt in with --serena)"
fi

echo "-> Building configserver ..."
make -C "$REPO_DIR" build

# Deploy the repo-managed home state alongside the binaries: settings, hooks,
# and skills. Context packs are deliberately excluded — they deploy per target
# repo via sync_context.sh / the config server's Context tab.
if [ "$RUN_SYNC" = 1 ]; then
  echo "-> Syncing settings ..."
  if [ "$INSTALL_SERENA" = 1 ]; then
    bash "$REPO_DIR/cmd/sync/sync_settings.sh" --serena
  else
    bash "$REPO_DIR/cmd/sync/sync_settings.sh"
  fi
  echo "-> Syncing hooks ..."
  bash "$REPO_DIR/cmd/sync/sync_hooks.sh"
  echo "-> Syncing skills ..."
  bash "$REPO_DIR/cmd/sync/sync_skills.sh"
else
  echo "skip: settings/hooks/skills sync (--no-sync)"
fi

# Per-install presentation profile: opt-in copy of a repo template
# (settings/claude_code/presentation-profile.<id>.md) to the machine-global
# path every consumer reads (global-context hook, config server, nightly).
if [ -n "${SMINE_PRESENTATION_PROFILE:-}" ]; then
  echo "-> Installing presentation profile ($SMINE_PRESENTATION_PROFILE) ..."
  mkdir -p "$HOME/.claude/context/global"
  cp "$REPO_DIR/settings/claude_code/presentation-profile.$SMINE_PRESENTATION_PROFILE.md" \
     "$HOME/.claude/context/global/presentation-profile.md"
fi

echo "-> Materializing routine plists ..."
for template in "$REPO_DIR"/routines/*/*.plist.template; do
  [ -e "$template" ] || continue
  target="${template%.template}"
  if [ -e "$target" ]; then
    echo "skip: $(basename "$target") exists (edit via the config server)"
    continue
  fi
  sed -e "s|__REPO_DIR__|$REPO_DIR|g" \
      -e "s|__HOME__|$HOME|g" \
    "$template" > "$target"
  echo "   $(basename "$target")"
done

LABEL="com.smine.configserver"
AGENT_PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"

PEEK_HOME_ARGS=""
if [ -n "$PEEK_CLAUDE_HOME" ]; then
  PEEK_HOME_ARGS="<string>-claude-home</string><string>$PEEK_CLAUDE_HOME</string>"
fi
if [ -n "$PEEK_CODEX_HOME" ]; then
  PEEK_HOME_ARGS="$PEEK_HOME_ARGS<string>-codex-home</string><string>$PEEK_CODEX_HOME</string>"
fi

echo "-> Installing LaunchAgent $LABEL ..."
mkdir -p "$HOME/Library/LaunchAgents"
sed -e "s|__REPO_DIR__|$REPO_DIR|g" \
    -e "s|__PORT__|$ADDR_PORT|g" \
    -e "s|__PEEK_PORT__|$PEEK_PORT|g" \
    -e "s|__PEEK_CONTROL_PORT__|$PEEK_CONTROL_PORT|g" \
    -e "s|__PEEK_HOME_ARGS__|$PEEK_HOME_ARGS|g" \
    -e "s|__HOME__|$HOME|g" \
    -e "s|__PATH__|$PATH|g" \
    -e "s|__INIT_WELCOME__|$INIT_WELCOME|g" \
  "$REPO_DIR/cmd/configserver/$LABEL.plist.template" > "$AGENT_PLIST"

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true

# Legacy agents under other labels KeepAlive-respawn a stale binary and win
# the port race against $LABEL (observed: com.kevinhorst.configserver) —
# bootout any foreign LaunchAgent whose plist runs a configserver binary.
for legacy_plist in "$HOME/Library/LaunchAgents"/*.plist; do
  [ -e "$legacy_plist" ] || continue
  legacy_label="$(basename "$legacy_plist" .plist)"
  [ "$legacy_label" = "$LABEL" ] && continue
  if grep -q "bin/configserver" "$legacy_plist"; then
    echo "-> Removing legacy configserver agent $legacy_label ..."
    launchctl bootout "gui/$(id -u)/$legacy_label" 2>/dev/null || true
    rm -f "$legacy_plist"
  fi
done

# Legacy pre-LaunchAgent instances (nohup) hold the port; kill by pid.
pid="$(lsof -tiTCP:"$ADDR_PORT" -sTCP:LISTEN || true)"
if [ -n "$pid" ]; then
  echo "-> Stopping process on :$ADDR_PORT (pid $pid) ..."
  kill "$pid" 2>/dev/null || true
  while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
fi

# A running peek survives configserver restarts by design, so install is the
# peek update vector: kill this profile's stale peek so the fresh configserver
# respawns it from the just-installed binary with current flags. lsof without
# sudo lists only this user's processes — another profile's peek on the same
# port is never killed here; the configserver's identity check reports it.
for peek_port in "$PEEK_PORT" "$PEEK_CONTROL_PORT"; do
  pid="$(lsof -tiTCP:"$peek_port" -sTCP:LISTEN || true)"
  if [ -n "$pid" ]; then
    echo "-> Stopping process on :$peek_port (pid $pid) ..."
    kill "$pid" 2>/dev/null || true
    while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
  fi
done

echo "-> Bootstrapping $LABEL ..."
launchctl bootstrap "gui/$(id -u)" "$AGENT_PLIST"

# Verify the port is served by THIS repo's binary — a foreign holder means a
# stale server answers and every "old state" symptom follows; fail loudly.
expected="$REPO_DIR/bin/configserver"
for _ in 1 2 3 4 5 6 7 8 9 10; do
  sleep 0.5
  pid="$(lsof -tiTCP:"$ADDR_PORT" -sTCP:LISTEN || true)"
  [ -n "$pid" ] || continue
  serving="$(ps -o comm= -p "$pid" 2>/dev/null || true)"
  if [ "$serving" = "$expected" ]; then
    echo "configserver LaunchAgent serving :$ADDR_PORT from $expected"
    echo "(logs: ~/Library/Logs/smine-configserver.*.log)"
    exit 0
  fi
done
echo "ERROR: :$ADDR_PORT is not served by $expected (holder: pid ${pid:-none}, ${serving:-nothing bound})" >&2
echo "       check ~/Library/Logs/smine-configserver.err.log and 'launchctl list | grep configserver'" >&2
exit 1
