#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

# Marker expansion targets. The deployed files feed NATIVE processes (Claude
# Code/node, codex, the peek-mcp binary), so on Windows the markers expand to
# C:/-style paths (cygpath -m: forward slashes, JSON- and TOML-safe) and peek
# resolves to the smine bin copy that install.ps1 refreshes - never an MSYS
# /c/ path, which native spawns cannot open. Git Bash accepts C:/ paths too,
# so hook commands keep working from the same expansion.
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    HOME_NATIVE="$(cygpath -m "$HOME")"
    PEEK_MCP="$(cygpath -m "$LOCALAPPDATA")/smine/bin/peek-mcp.exe"
    ;;
  *)
    HOME_NATIVE="$HOME"
    PEEK_MCP="$HOME/go/bin/peek-mcp"
    ;;
esac

# Per-install values (peek ports) live in the gitignored install.env written
# by install.sh / the Windows installer; absent file = peek defaults.
if [ -f "$REPO_DIR/install.env" ]; then
  # shellcheck source=/dev/null
  . "$REPO_DIR/install.env"
fi
PEEK_CONTROL_PORT="${PEEK_CONTROL_PORT:-42442}"

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
  sed -e "s|{{HOME}}|$HOME_NATIVE|g" -e "s|{{PEEK_MCP}}|$PEEK_MCP|g" -e "s|{{PEEK_CONTROL_PORT}}|$PEEK_CONTROL_PORT|g" "$src" > "$tmp"

  if [ -f "$dst" ] && diff -q "$tmp" "$dst" >/dev/null 2>&1; then
    echo "unchanged: $dst"
    rm -f "$tmp"
    return
  fi

  cp "$tmp" "$dst"
  rm -f "$tmp"
  echo "synced: $src -> $dst"
}

HOOKS_ENV="$HOME/.claude/hooks/global-context.env"
DOCS_DIR=""
if [ -f "$HOOKS_ENV" ]; then
  DOCS_DIR="$(grep '^AGENT_CONTEXT_DIR_DEFAULT=' "$HOOKS_ENV" | cut -d= -f2)"
fi
# Migration: fall back to the retired review-context.env for the context dir.
if [ -z "$DOCS_DIR" ] && [ -f "$HOME/.claude/hooks/review-context.env" ]; then
  DOCS_DIR="$(grep '^AGENT_CONTEXT_DIR_DEFAULT=' "$HOME/.claude/hooks/review-context.env" | cut -d= -f2)"
fi

if [ -z "$DOCS_DIR" ]; then
  # First run with no stored value: ask when a terminal is attached; a
  # non-interactive run (piped install, CI) takes the default instead of
  # dying on read's EOF under set -e.
  if [ -t 0 ]; then
    read -rp "Docs folder relative to project root [docs]: " DOCS_DIR
  else
    echo "no terminal attached — using default docs dir"
  fi
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
    printf '{}\n' > "$dst"
    echo "created: $dst (empty seed — Claude Code adds its own state on first run)"
  fi

  expanded="$(mktemp)"
  sed -e "s|{{HOME}}|$HOME_NATIVE|g" -e "s|{{PEEK_MCP}}|$PEEK_MCP|g" -e "s|{{PEEK_CONTROL_PORT}}|$PEEK_CONTROL_PORT|g" "$frag" > "$expanded"

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

# Serena writes its own config on first launch from the packaged template; the
# template default auto-opens the web dashboard tab on every MCP session start.
# Pin web_dashboard_open_on_launch to false — patch in place when the config
# exists, else seed it from the packaged template.
patch_serena_config() {
  local cfg="$HOME/.serena/serena_config.yml" template tmp

  if [ ! -f "$cfg" ]; then
    template="$(find "$HOME/.local/share/uv/tools/serena-agent" \
      -name serena_config.template.yml 2>/dev/null | head -1 || true)"
    if [ -z "$template" ]; then
      echo "skip: serena config (no $cfg and no packaged template found — run serena once, then re-sync)"
      return
    fi
    mkdir -p "$HOME/.serena"
    cp "$template" "$cfg"
    echo "created: $cfg (from packaged template)"
  fi

  if grep -q '^web_dashboard_open_on_launch: [Ff]alse' "$cfg"; then
    echo "unchanged: $cfg (web_dashboard_open_on_launch already false)"
    return
  fi
  tmp="$(mktemp)"
  sed 's/^web_dashboard_open_on_launch: .*/web_dashboard_open_on_launch: false/' "$cfg" > "$tmp" && mv "$tmp" "$cfg"
  # sed only rewrites an uncommented key; if the template shipped it commented
  # or absent, the override never landed — append it so the pin is effective.
  if ! grep -q '^web_dashboard_open_on_launch: false' "$cfg"; then
    printf '%s\n' 'web_dashboard_open_on_launch: false' >> "$cfg"
  fi
  echo "patched: $cfg (web_dashboard_open_on_launch: false)"
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
  patch_serena_config
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

  # Regenerate the env file but keep the user's toggle state (see
  # global-context-toggle.sh); migrate from the retired review-context.env once.
  ENABLED="1"
  if [ -f "$HOOKS_DST/global-context.env" ]; then
    val="$(grep '^GLOBAL_CONTEXT_ENABLED=' "$HOOKS_DST/global-context.env" | cut -d= -f2 || true)"
    [ -n "$val" ] && ENABLED="$val"
  elif [ -f "$HOOKS_DST/review-context.env" ]; then
    val="$(grep '^REVIEW_CONTEXT_ENABLED=' "$HOOKS_DST/review-context.env" | cut -d= -f2 || true)"
    [ -n "$val" ] && ENABLED="$val"
  fi
  {
    echo "AGENT_CONTEXT_DIR_DEFAULT=$DOCS_DIR"
    echo "GLOBAL_CONTEXT_ENABLED=$ENABLED"
  } > "$HOOKS_DST/global-context.env"
  rm -f "$HOOKS_DST/review-context.env"
  echo "wrote: $HOOKS_DST/global-context.env (AGENT_CONTEXT_DIR_DEFAULT=$DOCS_DIR, GLOBAL_CONTEXT_ENABLED=$ENABLED)"
fi
