#!/usr/bin/env bash
set -euo pipefail

# Enable/disable the review-context UserPromptSubmit hook without touching
# settings.json (UserPromptSubmit hooks ignore matchers, so review-context.sh
# checks REVIEW_CONTEXT_ENABLED instead).
#
# Usage: review-context-toggle.sh [on|off]   (no arg = toggle)

ENV_FILE="$HOME/.claude/hooks/review-context.env"

current="1"
if [ -f "$ENV_FILE" ]; then
  val="$(grep '^REVIEW_CONTEXT_ENABLED=' "$ENV_FILE" | cut -d= -f2 || true)"
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
if grep -q '^REVIEW_CONTEXT_ENABLED=' "$ENV_FILE"; then
  sed -i '' "s|^REVIEW_CONTEXT_ENABLED=.*|REVIEW_CONTEXT_ENABLED=$new|" "$ENV_FILE"
else
  echo "REVIEW_CONTEXT_ENABLED=$new" >> "$ENV_FILE"
fi

if [ "$new" = "1" ]; then
  echo "review-context hook: ON"
else
  echo "review-context hook: OFF"
fi
