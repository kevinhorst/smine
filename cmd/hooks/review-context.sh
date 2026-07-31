#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
[ -f ~/.claude/hooks/review-context.env ] && source ~/.claude/hooks/review-context.env
DOCS_DIR="${AGENT_CONTEXT_DIR_DEFAULT:-docs}"

# Kill switch: UserPromptSubmit hooks ignore matchers, so the only way to
# disable injection without deleting the settings.json entry is here.
# Flip with review-context-toggle.sh.
[ "${REVIEW_CONTEXT_ENABLED:-1}" = "0" ] && exit 0

FILES=(
  "AGENTS.md"
  "go.mod"
  "requirements.txt"
  "$DOCS_DIR/style/commits.md"
  "$DOCS_DIR/style/go.md"
  "$DOCS_DIR/style/python.md"
  "$DOCS_DIR/style/sql.md"
)

# Doctrine chapters: baseline + repo overlays, whatever the pack holds
# (concepting incl. hot gates, implementing incl. stops/integrity, reviewing incl. DoD, navigating).
for f in "$DOCS_DIR"/rules/*.md; do
  [ -f "$f" ] && FILES+=("$f")
done

echo "===== REVIEW CONTEXT (injected by hook) ====="
echo ""

for f in "${FILES[@]}"; do
  if [ -f "$f" ]; then
    echo "===== $f ====="
    cat "$f"
    echo ""
  fi
done
