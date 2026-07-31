#!/usr/bin/env bash
# Sync repo skills to the claude and codex home roots.
#
# Usage:
#   sync_skills.sh [--prune]
#
# Installed skills that no longer exist in the repo are confirmed one by one;
# with --prune they are removed without asking (the skip-list still applies).
set -euo pipefail

auto_prune=0
for arg in "$@"; do
  case "$arg" in
    --prune) auto_prune=1 ;;
    *) echo "usage: $(basename "$0") [--prune]"; exit 1 ;;
  esac
done

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SKILLS_SRC="$REPO_DIR/skills"

CLAUDE_SKILLS="$HOME/.claude/skills"
CODEX_SKILLS="$HOME/.codex/skills"

HOOKS_ENV="$HOME/.claude/hooks/review-context.env"

# Skills installed by other sources — never offered for pruning.
PRUNE_SKIP="codex-primary-runtime"

safe_prune_dir() {
  local target=$1 installed_dir=$2 installed_path expected_parent actual_parent name
  installed_path=${installed_dir%/}
  name=$(basename -- "$installed_path")

  if [ -L "$target" ] || [ -L "$installed_path" ] || [ ! -d "$installed_path" ]; then
    echo "error: refusing to prune unexpected path: $installed_path"
    return 1
  fi

  expected_parent=$(cd -- "$target" && pwd -P)
  actual_parent=$(cd -- "$(dirname -- "$installed_path")" && pwd -P)
  if [ "$actual_parent" != "$expected_parent" ] || [ -z "$name" ] || \
     [ "$name" = "." ] || [ "$name" = ".." ]; then
    echo "error: refusing to prune unsafe path: $installed_path"
    return 1
  fi
}

# skill_version reads the single-line frontmatter version: value between the
# --- fences; a missing file or field yields "".
skill_version() {
  [ -f "$1" ] || return 0
  awk '/^---$/{fence++; next} fence==1 && sub(/^version:[ \t]*/, "") {print; exit}' "$1"
}

if [ ! -d "$SKILLS_SRC" ]; then
  echo "error: skills directory not found at $SKILLS_SRC"
  exit 1
fi

context_dir=""
if [ -f "$HOOKS_ENV" ]; then
  context_dir="$(grep '^AGENT_CONTEXT_DIR_DEFAULT=' "$HOOKS_ENV" | cut -d= -f2)"
fi

if [ -z "$context_dir" ]; then
  printf "Context folder name (AGENT_CONTEXT_DIR_DEFAULT not found) [docs]: "
  # read fails at EOF under set -e when stdin is closed (server invocation) —
  # fall through to the default.
  read -r context_dir || true
  context_dir="${context_dir:-docs}"
fi

echo "using context dir: $context_dir"

# Resolve leaf skill dirs: a first-level dir containing SKILL.md is a skill;
# one without is a group folder whose children are skills. Targets stay flat —
# every leaf lands as $target/<leaf-name>.
skill_dirs=()
for entry in "$SKILLS_SRC"/*/; do
  if [ -f "${entry}SKILL.md" ]; then
    skill_dirs+=("$entry")
  else
    for sub in "$entry"*/; do
      if [ -f "${sub}SKILL.md" ]; then
        skill_dirs+=("$sub")
      fi
    done
  fi
done

# Empty array + set -u breaks "${skill_dirs[@]}" on macOS bash 3.2 — and an
# empty skills tree is a broken checkout anyway.
if [ "${#skill_dirs[@]}" -eq 0 ]; then
  echo "error: no */SKILL.md found under $SKILLS_SRC"
  exit 1
fi

repo_skill_names=" "
for skill_dir in "${skill_dirs[@]}"; do
  repo_skill_names+="$(basename "$skill_dir") "
done

for target in "$CLAUDE_SKILLS" "$CODEX_SKILLS"; do
  mkdir -p "$target"

  for skill_dir in "${skill_dirs[@]}"; do
    skill_name="$(basename "$skill_dir")"
    target_dir="$target/$skill_name"
    old_version="$(skill_version "$target_dir/SKILL.md")"
    new_version="$(skill_version "${skill_dir}SKILL.md")"
    mkdir -p "$target_dir"
    # Copy the whole skill (reference files etc.), then render SKILL.md with
    # the context dir substituted.
    cp -R "${skill_dir%/}/." "$target_dir/"
    sed "s|\\\$AGENT_CONTEXT_DIR_DEFAULT|$context_dir|g" "$skill_dir"SKILL.md > "$target_dir/SKILL.md"
    if [ -n "$new_version" ] && [ "$new_version" != "$old_version" ]; then
      if [ -n "$old_version" ]; then
        echo "  $skill_name -> $target_dir (bumped v$old_version -> v$new_version)"
      else
        echo "  $skill_name -> $target_dir (new v$new_version)"
      fi
    else
      echo "  $skill_name -> $target_dir"
    fi
  done

  # Prune installed skills that no longer exist in the repo (renamed or
  # removed). Confirm each one: targets may hold skills from other sources.
  for installed_dir in "$target"/*/; do
    [ -d "$installed_dir" ] || continue
    installed_name="$(basename "$installed_dir")"
    case " $PRUNE_SKIP " in
      *" $installed_name "*)
        echo "  skipped (prune skip-list): $installed_dir"
        continue
        ;;
    esac
    case "$repo_skill_names" in
      *" $installed_name "*) continue ;;
    esac
    if [ "$auto_prune" -eq 1 ]; then
      answer=y
    else
      printf "prune %s (not in repo)? [y/N]: " "$installed_dir"
      # EOF (no stdin) answers "n": never prune without an explicit yes.
      read -r answer || answer=n
    fi
    if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
      safe_prune_dir "$target" "$installed_dir" || exit 1
      rm -rf -- "${installed_dir%/}"
      echo "  pruned: $installed_dir"
    else
      echo "  kept: $installed_dir"
    fi
  done

  echo "synced to $target"
done

# --- Agent definitions (Claude only — Codex has no Agent-tool agents) ---
AGENTS_SRC="$REPO_DIR/agents"
CLAUDE_AGENTS="$HOME/.claude/agents"
if [ -d "$AGENTS_SRC" ]; then
  mkdir -p "$CLAUDE_AGENTS"
  for agent_file in "$AGENTS_SRC"/*.md; do
    [ -f "$agent_file" ] || continue
    cp "$agent_file" "$CLAUDE_AGENTS/$(basename "$agent_file")"
    echo "  agent $(basename "$agent_file") -> $CLAUDE_AGENTS"
  done
  echo "synced agents to $CLAUDE_AGENTS"
fi

# --- Agent toolset: scripts subagents invoke by stable path, independent of any checkout ---
TOOLS_DST="$HOME/.claude/agents/tools"
mkdir -p "$TOOLS_DST/_lib"
cp "$REPO_DIR/cmd/worktrees/remove_agent_worktrees.sh" "$TOOLS_DST/remove_agent_worktrees.sh"
chmod +x "$TOOLS_DST/remove_agent_worktrees.sh"
cp "$REPO_DIR/cmd/worktrees/_lib/verdict.sh" "$TOOLS_DST/_lib/verdict.sh"
echo "  toolset remove_agent_worktrees.sh (+ _lib/verdict.sh) -> $TOOLS_DST"
