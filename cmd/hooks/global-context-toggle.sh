#!/usr/bin/env bash
set -euo pipefail

# Enable/disable the global-context SessionStart/SubagentStart hook without
# touching settings.json (global-context.sh checks GLOBAL_CONTEXT_ENABLED
# instead).
#
# Usage: global-context-toggle.sh [on|off]   (no arg = toggle)

ENV_FILE="$HOME/.claude/hooks/global-context.env"

current="1"
if [ -f "$ENV_FILE" ]; then
  val="$(grep '^GLOBAL_CONTEXT_ENABLED=' "$ENV_FILE" | cut -d= -f2 || true)"
  [ -n "$val" ] && current="$val"
fi

case "${1:-}" in
  on) new="1" ;;
  off) new="0" ;;
  "") if [ "$current" = "0" ]; then new="1"; else new="0"; fi ;;
  *)
    echo "usage: $(basename "$0") [on|off]" >&2
    exit 1
    ;;
esac

touch "$ENV_FILE"
if grep -q '^GLOBAL_CONTEXT_ENABLED=' "$ENV_FILE"; then
  sed -i '' "s|^GLOBAL_CONTEXT_ENABLED=.*|GLOBAL_CONTEXT_ENABLED=$new|" "$ENV_FILE"
else
  echo "GLOBAL_CONTEXT_ENABLED=$new" >> "$ENV_FILE"
fi

if [ "$new" = "1" ]; then
  echo "global-context hook: ON"
else
  echo "global-context hook: OFF"
fi
