#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
ADDR_PORT="${CONFIGSERVER_PORT:-6001}"

INSTALL_PEEK=1
INSTALL_SERENA=0
RUN_SYNC=1
for arg in "$@"; do
  case "$arg" in
    --no-peek)   INSTALL_PEEK=0 ;;
    --serena)    INSTALL_SERENA=1 ;;
    --no-sync)   RUN_SYNC=0 ;;
    *)
      echo "usage: $0 [--no-peek] [--serena] [--no-sync]" >&2
      exit 1
      ;;
  esac
done

if [ "$INSTALL_PEEK" = 1 ]; then
  echo "-> Installing peek-mcp ..."
  go install github.com/kevinhorst/peek-mcp@v1.2.0
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

echo "-> Installing LaunchAgent $LABEL ..."
mkdir -p "$HOME/Library/LaunchAgents"
sed -e "s|__REPO_DIR__|$REPO_DIR|g" \
    -e "s|__PORT__|$ADDR_PORT|g" \
    -e "s|__HOME__|$HOME|g" \
    -e "s|__PATH__|$PATH|g" \
  "$REPO_DIR/cmd/configserver/$LABEL.plist.template" > "$AGENT_PLIST"

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true

# Legacy agents under other labels KeepAlive-respawn a stale binary and win
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
