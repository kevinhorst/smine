#!/usr/bin/env bash
# SessionStart + SubagentStart hook: inject always-read global content —
# things every session needs that belong neither in CLAUDE.md/AGENTS.md
# (harness-loaded project identity) nor in a skill's context: declaration
# (skill-scoped doctrine, injected by skill-context.sh at invocation).
# Content lives in <context-dir>/global/*.md (e.g. company facts).
# Silent when the directory is absent or empty.
set -euo pipefail

# shellcheck source=/dev/null
[ -f ~/.claude/hooks/global-context.env ] && source ~/.claude/hooks/global-context.env
DOCS_DIR="${AGENT_CONTEXT_DIR_DEFAULT:-docs}"

# Kill switch: SessionStart hooks have no matcher granularity worth using, so
# the only way to disable injection without deleting the settings.json entry
# is here. Flip with global-context-toggle.sh.
[ "${GLOBAL_CONTEXT_ENABLED:-1}" = "0" ] && exit 0

found=0
# Machine-global content ($HOME/.claude/context/global) first — the
# presentation profile lives there — then the repo-local docs dir.
for dir in "$HOME/.claude/context/global" "$DOCS_DIR/global"; do
  for f in "$dir"/*.md; do
    [ -f "$f" ] || continue
    if [ "$found" = 0 ]; then
      echo "===== GLOBAL CONTEXT (injected by hook) ====="
      echo ""
      found=1
    fi
    echo "===== $f ====="
    cat "$f"
    echo ""
  done
done
