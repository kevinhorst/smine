#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
HOOKS_SRC="$REPO_DIR/cmd/hooks"
HOOKS_DST="$HOME/.claude/hooks"

if [ ! -d "$HOOKS_SRC" ]; then
  echo "error: hooks directory not found at $HOOKS_SRC"
  exit 1
fi

mkdir -p "$HOOKS_DST"

for src_file in "$HOOKS_SRC"/*.sh; do
  [ -f "$src_file" ] || continue
  name="$(basename "$src_file")"
  cp "$src_file" "$HOOKS_DST/$name"
  chmod +x "$HOOKS_DST/$name"
  echo "  $name -> $HOOKS_DST/$name"
done

# Prune hooks in ~/.claude/hooks/ that no longer exist in the repo.
# Only considers .sh files; skips non-script files (e.g. .env).
for installed in "$HOOKS_DST"/*.sh; do
  [ -f "$installed" ] || continue
  name="$(basename "$installed")"
  if [ ! -f "$HOOKS_SRC/$name" ]; then
    printf "prune %s (not in repo)? [y/N]: " "$installed"
    # EOF (no stdin) answers "n": never prune without an explicit yes.
    read -r answer || answer=n
    if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
      rm -f "$installed"
      echo "  pruned: $installed"
    else
      echo "  kept: $installed"
    fi
  fi
done

echo "synced hooks to $HOOKS_DST"
