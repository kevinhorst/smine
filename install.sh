#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
ADDR_PORT="${CONFIGSERVER_PORT:-6001}"

INSTALL_PEEK=1
INSTALL_SERENA=0
for arg in "$@"; do
  case "$arg" in
    --no-peek)   INSTALL_PEEK=0 ;;
    --serena)    INSTALL_SERENA=1 ;;
    *)
      echo "usage: $0 [--no-peek] [--serena]" >&2
      exit 1
      ;;
  esac
done

if [ "$INSTALL_PEEK" = 1 ]; then
  echo "-> Installing peek-mcp ..."
  go install github.com/kevinhorst/peek-mcp@v1.0.7
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

# Legacy pre-LaunchAgent instances (nohup) hold the port; kill by pid.
pid="$(lsof -tiTCP:"$ADDR_PORT" -sTCP:LISTEN || true)"
if [ -n "$pid" ]; then
  echo "-> Stopping process on :$ADDR_PORT (pid $pid) ..."
  kill "$pid" 2>/dev/null || true
  while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
fi

echo "-> Bootstrapping $LABEL ..."
launchctl bootstrap "gui/$(id -u)" "$AGENT_PLIST"
echo "configserver LaunchAgent loaded (logs: ~/Library/Logs/smine-configserver.*.log)"
