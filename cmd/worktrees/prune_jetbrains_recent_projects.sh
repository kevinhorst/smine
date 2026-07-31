#!/usr/bin/env bash
# Prune Claude Code worktree entries from JetBrains IDEs' Recent Projects lists.
#
# JetBrains IDEs (2026.1) auto-register every git worktree of an open repo as a
# separate project. Claude Code sessions live in <repo>/.claude/worktrees/<name>,
# so the Welcome-screen Recent Projects list fills up with dozens of stale
# worktree entries. This strips every entry whose path contains
# /.claude/worktrees/ from each recentProjects.xml under:
#   ~/Library/Application Support/JetBrains/*/options/recentProjects.xml
#
# An IDE rewrites recentProjects.xml on exit, so it must be closed while its
# file is edited. A running IDE's file is skipped with a warning (other IDEs
# are still pruned) unless --force is given.
#
# Usage:
#   prune_jetbrains_recent_projects.sh [-n|--dry-run] [--force]
#
#   -n, --dry-run   report what would be removed, change nothing
#   --force         prune even files of running IDEs (edit may be lost)

set -euo pipefail

dry_run=0
force=0
for arg in "$@"; do
  case "$arg" in
    -n|--dry-run) dry_run=1 ;;
    --force) force=1 ;;
    -h|--help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "usage: $(basename "$0") [-n|--dry-run] [--force]" >&2; exit 1 ;;
  esac
done

jetbrains_dir="$HOME/Library/Application Support/JetBrains"
if [ ! -d "$jetbrains_dir" ]; then
  echo "no JetBrains config dir at $jetbrains_dir; nothing to do"
  exit 0
fi

# app_pattern maps a product config dir (version stripped) to the app-bundle
# name pgrep sees; camel-cased dirs with spaced app names need the map, the
# rest match their dir prefix verbatim.
app_pattern() {
  case "$1" in
    IntelliJIdea)  echo 'IntelliJ IDEA' ;;
    IdeaIC)        echo 'IntelliJ IDEA CE' ;;
    PyCharmCE)     echo 'PyCharm CE' ;;
    AndroidStudio) echo 'Android Studio' ;;
    *) echo "$1" ;;
  esac
}

# awk state machine: drop each <entry ...worktrees...> block through its </entry>.
prune_awk='
  /<entry key="[^"]*\/\.claude\/worktrees\// { skip = 1 }
  skip { if (/<\/entry>/) skip = 0; next }
  { print }
'

total_removed=0
files_changed=0
files_skipped=0

while IFS= read -r f; do
  [ -f "$f" ] || continue
  count=$(grep -c '<entry key="[^"]*/\.claude/worktrees/' "$f" || true)
  [ "$count" -gt 0 ] || continue

  rel=${f#"$jetbrains_dir"/}
  total_removed=$((total_removed + count))

  # Per-IDE guard (D4): an IDE rewrites recentProjects.xml on exit, so a
  # running IDE's file is skipped; other IDEs are still pruned.
  product=${rel%%/*}          # GoLand2026.1
  product=${product%%[0-9.]*} # GoLand
  if pgrep -f "$(app_pattern "$product")[^/]*\.app/Contents/MacOS" >/dev/null 2>&1; then
    if [ "$force" -eq 1 ]; then
      echo "warning: $product is running; --force given, pruning anyway (edit may be clobbered on exit)" >&2
    elif [ "$dry_run" -eq 0 ]; then
      echo "skipping $rel: $product is running (quit it or pass --force)" >&2
      total_removed=$((total_removed - count))
      files_skipped=$((files_skipped + 1))
      continue
    fi
  fi

  if [ "$dry_run" -eq 1 ]; then
    echo "would remove $count worktree entr$([ "$count" -eq 1 ] && echo y || echo ies): $rel"
    continue
  fi

  backup="$f.bak.$(date +%Y%m%d-%H%M%S)"
  cp -p "$f" "$backup"
  tmp=$(mktemp "${TMPDIR:-/tmp}/recentProjects.XXXXXX")
  awk "$prune_awk" "$f" > "$tmp"
  mv "$tmp" "$f"
  files_changed=$((files_changed + 1))
  echo "removed $count worktree entr$([ "$count" -eq 1 ] && echo y || echo ies): $rel (backup: ${backup##*/})"
done < <(find "$jetbrains_dir" -maxdepth 3 -path '*/options/recentProjects.xml' -type f 2>/dev/null)

skipped_note=""
[ "$files_skipped" -gt 0 ] && skipped_note=", skipped $files_skipped running IDE(s)"

if [ "$total_removed" -eq 0 ]; then
  echo "no worktree entries found; nothing to prune$skipped_note"
elif [ "$dry_run" -eq 1 ]; then
  echo "dry run: $total_removed worktree entr$([ "$total_removed" -eq 1 ] && echo y || echo ies) would be removed"
else
  echo "done: removed $total_removed worktree entr$([ "$total_removed" -eq 1 ] && echo y || echo ies) across $files_changed file(s)$skipped_note"
fi
