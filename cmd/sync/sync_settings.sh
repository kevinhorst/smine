#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

SYNC_SERENA=0
for arg in "$@"; do
  case "$arg" in
    --serena) SYNC_SERENA=1 ;;
    *)
      echo "usage: $0 [--serena]" >&2
      exit 1
      ;;
  esac
done

CLAUDE_SRC="$REPO_DIR/settings/claude_code/settings.json"
CLAUDE_MD_SRC="$REPO_DIR/settings/claude_code/CLAUDE.md"
CODEX_SRC="$REPO_DIR/settings/codex/config.toml"

CLAUDE_DST="$HOME/.claude/settings.json"
CLAUDE_MD_DST="$HOME/.claude/CLAUDE.md"
CODEX_DST="$HOME/.codex/config.toml"

# Repo settings files are templates: {{HOME}} markers are expanded to the
# local $HOME at deploy time (same marker pattern as sync_context.sh).
sync_file() {
  local src="$1" dst="$2" tmp

  if [ ! -f "$src" ]; then
    echo "skip: $src not found"
    return
  fi

  tmp="$(mktemp)"
  sed "s|{{HOME}}|$HOME|g" "$src" > "$tmp"

  if [ -f "$dst" ] && diff -q "$tmp" "$dst" >/dev/null 2>&1; then
    echo "unchanged: $dst"
    rm -f "$tmp"
    return
  fi

  cp "$tmp" "$dst"
  rm -f "$tmp"
  echo "synced: $src -> $dst"
}

HOOKS_ENV="$HOME/.claude/hooks/review-context.env"
DOCS_DIR=""
if [ -f "$HOOKS_ENV" ]; then
  DOCS_DIR="$(grep '^AGENT_CONTEXT_DIR_DEFAULT=' "$HOOKS_ENV" | cut -d= -f2)"
fi

if [ -z "$DOCS_DIR" ]; then
  read -rp "Docs folder relative to project root [docs]: " DOCS_DIR
  DOCS_DIR="${DOCS_DIR:-docs}"
else
  echo "using context dir: $DOCS_DIR (from $HOOKS_ENV)"
fi

# Merge the repo-managed MCP servers into ~/.claude.json. The file carries live
# harness state (oauth, caches, machineID) — only .mcpServers keys from the
# fragment are touched, everything else is preserved, and the write is atomic.
merge_mcp_servers() {
  local frag="$1" dst="$2" tmp expanded

  if [ ! -f "$frag" ]; then
    echo "skip: $frag not found"
    return
  fi
  if [ ! -f "$dst" ]; then
    echo "skip: $dst not found (created by Claude Code, not by this script)"
    return
  fi

  expanded="$(mktemp)"
  sed "s|{{HOME}}|$HOME|g" "$frag" > "$expanded"

  tmp="$(mktemp)"
  jq --slurpfile frag "$expanded" \
    '.mcpServers = ((.mcpServers // {}) + $frag[0].mcpServers)' \
    "$dst" > "$tmp"
  jq empty "$tmp"
  rm -f "$expanded"

  if diff -q "$tmp" "$dst" >/dev/null 2>&1; then
    echo "unchanged: $dst (mcpServers)"
    rm -f "$tmp"
    return
  fi

  mv "$tmp" "$dst"
  echo "merged: $frag -> $dst (mcpServers)"
}

CLAUDE_MCP_SRC="$REPO_DIR/settings/claude_code/claude.json"
CLAUDE_MCP_DST="$HOME/.claude.json"

sync_file "$CLAUDE_SRC" "$CLAUDE_DST"
sync_file "$CLAUDE_MD_SRC" "$CLAUDE_MD_DST"
if [ "$SYNC_SERENA" = 1 ]; then
  CODEX_TMP="$(mktemp)"
  cat "$CODEX_SRC" "$REPO_DIR/settings/codex/config.serena.toml" > "$CODEX_TMP"
  sync_file "$CODEX_TMP" "$CODEX_DST"
  rm -f "$CODEX_TMP"
else
  sync_file "$CODEX_SRC" "$CODEX_DST"
fi
merge_mcp_servers "$CLAUDE_MCP_SRC" "$CLAUDE_MCP_DST"
if [ "$SYNC_SERENA" = 1 ]; then
  merge_mcp_servers "$REPO_DIR/settings/claude_code/claude.serena.json" "$CLAUDE_MCP_DST"
fi

HOOKS_SRC="$REPO_DIR/cmd/hooks"
HOOKS_DST="$HOME/.claude/hooks"

if [ -d "$HOOKS_SRC" ]; then
  mkdir -p "$HOOKS_DST"
  for hook_script in "$HOOKS_SRC"/*.sh; do
    [ -f "$hook_script" ] || continue
    dst_file="$HOOKS_DST/$(basename "$hook_script")"
    if [ -f "$dst_file" ] && diff -q "$hook_script" "$dst_file" >/dev/null 2>&1; then
      echo "unchanged: $dst_file"
    else
      cp "$hook_script" "$dst_file"
      chmod +x "$dst_file"
      echo "synced: $hook_script -> $dst_file"
    fi
  done

  # Regenerate the env file but keep the user's toggle state (see review-context-toggle.sh).
  ENABLED="1"
  if [ -f "$HOOKS_DST/review-context.env" ]; then
    val="$(grep '^REVIEW_CONTEXT_ENABLED=' "$HOOKS_DST/review-context.env" | cut -d= -f2 || true)"
    [ -n "$val" ] && ENABLED="$val"
  fi
  {
    echo "AGENT_CONTEXT_DIR_DEFAULT=$DOCS_DIR"
    echo "REVIEW_CONTEXT_ENABLED=$ENABLED"
  } > "$HOOKS_DST/review-context.env"
  echo "wrote: $HOOKS_DST/review-context.env (AGENT_CONTEXT_DIR_DEFAULT=$DOCS_DIR, REVIEW_CONTEXT_ENABLED=$ENABLED)"
fi
