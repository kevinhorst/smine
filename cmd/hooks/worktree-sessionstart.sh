#!/usr/bin/env bash
# SessionStart hook: harden the session worktree and keep the worktree pool
# free of hollow directories. Claude Desktop pools worktree dirs under
# <repo>/.claude/worktrees/ and has reused hollow ones (no .git entry) for
# new sessions, hijacking the main checkout via git's upward discovery.
#
# On every session start ($PWD = session directory):
#   in a valid pool worktree — write .idea/vcs.xml (JetBrains Git mapping),
#     drop the untracked .claude-worktree sentinel (never "clean", so the app
#     neither resets nor auto-removes it), git worktree lock it, then protect
#     the repo (decoy + sweep, see below).
#   in a hollow pool dir — warn only; the worktree-guard.sh prompt hook blocks
#     the session, and the decoy contains its git commands. Never repaired.
#   in a main checkout — protect the repo.
#
# Protecting a repo = install the pool decoy (.claude/worktrees/.git file with
# a dead gitdir pointer; kills upward discovery from hollow dirs) and sweep:
# delete pool dirs that are unregistered, have no .git entry, and contain
# nothing but .idea / .claude / .DS_Store / .claude-worktree. Anything else is
# reported and left alone.
#
# Usage:
#   worktree-sessionstart.sh                 hook mode ($PWD)
#   worktree-sessionstart.sh --sweep <repo> [<repo> ...]   manual decoy+sweep
#
# Hook mode always exits 0 (SessionStart must not disturb the session) and
# prints nothing to stdout (SessionStart stdout is injected into the model
# context). All reporting goes to stderr.

set -uo pipefail

DECOY_POINTER="gitdir: /claude-pool-guard-see-smine-cmd-worktrees"
SENTINEL=".claude-worktree"

log() { echo "worktree-sessionstart: $*" >&2; }

install_decoy() {
  local repo=$1 pool decoy
  pool="$repo/.claude/worktrees"
  decoy="$pool/.git"
  mkdir -p "$pool"
  if [ -e "$decoy" ] || [ -L "$decoy" ]; then
    if [ -f "$decoy" ] && [ ! -L "$decoy" ] && [ "$(<"$decoy")" = "$DECOY_POINTER" ]; then
      return 0
    fi
    log "unexpected $decoy exists — not replacing"
    return 1
  fi
  printf '%s\n' "$DECOY_POINTER" > "$decoy"
  log "installed pool decoy at $decoy"
}

# Delete hollow pool dirs; report everything that is not clearly a husk.
# $2 (optional): directory to leave alone (the session's own cwd).
sweep_pool() {
  local repo=$1 exclude=${2:-} pool registered d name entry husk line gitdir
  pool="$repo/.claude/worktrees"
  [ -d "$pool" ] || return 0
  registered=$(git -C "$repo" worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')

  for d in "$pool"/*/; do
    d="${d%/}"
    [ -d "$d" ] || continue
    name="${d##*/}"
    [ -n "$name" ] || continue
    [ -n "$exclude" ] && [ "$d" = "$exclude" ] && continue
    if [ -L "$d" ]; then
      log "skip symlink in pool: $d"
      continue
    fi

    if [ -e "$d/.git" ]; then
      if [ -f "$d/.git" ]; then
        IFS= read -r line < "$d/.git" || true
        gitdir="${line#gitdir: }"
        [[ "$gitdir" == /* ]] || gitdir="$d/$gitdir"
        [ -d "$gitdir" ] || log "$d has a dead gitdir pointer ($gitdir) — inspect manually"
      fi
      continue
    fi

    if printf '%s\n' "$registered" | grep -qxF "$d"; then
      log "$d is registered but has no .git entry — inspect manually"
      continue
    fi

    husk=1
    for entry in "$d"/* "$d"/.[!.]* "$d"/..?*; do
      [ -e "$entry" ] || [ -L "$entry" ] || continue
      case "${entry##*/}" in
        .idea | .claude | .DS_Store | "$SENTINEL") ;;
        *)
          husk=0
          break
          ;;
      esac
    done

    if [ "$husk" -eq 1 ]; then
      rm -rf -- "$d"
      log "removed hollow pool dir $d"
    else
      log "$d is not a worktree but holds unexpected files — left alone"
    fi
  done
}

# Make a valid pool worktree JetBrains-ready and non-"free" for the pool.
harden_worktree() {
  local repo=$1 wt=$2 branch
  if [ ! -f "$wt/.idea/vcs.xml" ]; then
    mkdir -p "$wt/.idea"
    cat > "$wt/.idea/vcs.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<project version="4">
  <component name="VcsDirectoryMappings">
    <mapping directory="$PROJECT_DIR$" vcs="Git" />
  </component>
</project>
EOF
    log "wrote $wt/.idea/vcs.xml"
  fi

  if [ ! -e "$wt/$SENTINEL" ]; then
    branch=$(git -C "$wt" branch --show-current 2>/dev/null)
    [ -n "$branch" ] || branch=$(git -C "$wt" rev-parse --short HEAD 2>/dev/null)
    {
      echo "created-at: $(date '+%Y-%m-%d %H:%M:%S %z')"
      echo "branch: $branch"
      echo "main-repo: $repo"
      echo "purpose: untracked sentinel — keeps the harness from classifying this worktree as free (reset/auto-remove); written by worktree-sessionstart.sh"
    } > "$wt/$SENTINEL"
    log "wrote sentinel $wt/$SENTINEL"
  fi

  git -C "$repo" worktree lock --reason "claude session (worktree-sessionstart.sh)" "$wt" 2>/dev/null \
    && log "locked $wt"
  return 0
}

# --- Manual mode -----------------------------------------------------------
if [ "${1:-}" = "--sweep" ]; then
  shift
  if [ $# -eq 0 ]; then
    echo "usage: $(basename "$0") --sweep <repo> [<repo> ...]" >&2
    exit 1
  fi
  failed=0
  for repo in "$@"; do
    repo=$(cd -- "$repo" 2>/dev/null && pwd -P) || {
      log "skip $repo: no such directory"
      failed=1
      continue
    }
    if [ ! -d "$repo/.git" ]; then
      log "skip $repo: not a main git checkout"
      failed=1
      continue
    fi
    install_decoy "$repo" || failed=1
    sweep_pool "$repo"
  done
  exit "$failed"
elif [ $# -gt 0 ]; then
  echo "usage: $(basename "$0") [--sweep <repo> ...]" >&2
  exit 1
fi

# --- Hook mode ($PWD, always exit 0) ----------------------------------------
REQUEST_DIR=$(cd -- "$PWD" && pwd -P)

MAIN_REPO=""
WORKTREE_DIR=""
check="$REQUEST_DIR"
while [[ "$check" == /*/* ]]; do
  parent="${check%/*}"
  grandparent="${parent%/*}"
  if [[ "${parent##*/}" == "worktrees" && "${grandparent##*/}" == ".claude" ]]; then
    MAIN_REPO="${grandparent%/*}"
    WORKTREE_DIR="$check"
    break
  fi
  check="$parent"
done

if [ -n "$WORKTREE_DIR" ]; then
  if [ ! -d "$MAIN_REPO/.git" ]; then
    log "pool dir $WORKTREE_DIR has no main checkout at $MAIN_REPO — nothing done"
    exit 0
  fi
  install_decoy "$MAIN_REPO" || true
  sweep_pool "$MAIN_REPO" "$WORKTREE_DIR"

  if [ -e "$WORKTREE_DIR/.git" ]; then
    valid=1
    if [ -f "$WORKTREE_DIR/.git" ]; then
      IFS= read -r line < "$WORKTREE_DIR/.git" || true
      gitdir="${line#gitdir: }"
      [[ "$gitdir" == /* ]] || gitdir="$WORKTREE_DIR/$gitdir"
      [ -d "$gitdir" ] || valid=0
    fi
    if [ "$valid" -eq 1 ]; then
      harden_worktree "$MAIN_REPO" "$WORKTREE_DIR"
    else
      log "$WORKTREE_DIR has a dead gitdir pointer — worktree-guard.sh will block prompts"
    fi
  else
    log "$WORKTREE_DIR is hollow (no .git entry) — worktree-guard.sh will block prompts"
  fi
  exit 0
fi

# Not in a pool dir: protect the repo the session runs in, if any.
common=$(git -C "$REQUEST_DIR" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) || exit 0
repo="${common%/.git}"
[ "$repo" != "$common" ] || exit 0
install_decoy "$repo" || true
sweep_pool "$repo"
exit 0
