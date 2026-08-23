#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
CONTEXT_SRC="$REPO_DIR/context"
# Go module that provides cmd/rules (filter + generate). Overridable so the
# test fixture can run a script copy against the real module.
RULES_REPO="${SYNC_CONTEXT_RULES_REPO:-$REPO_DIR}"

# shellcheck source=smine_tool.sh
. "$(dirname "$0")/smine_tool.sh"

# --- Arguments ---
# usage: sync_context.sh [--context-dir NAME] [--langs a,b,c] [--role TEXT] [--no-prose] [--acdsl|--no-acdsl] [--symlink|--no-symlink] [TARGET]
# --no-prose skips the prose pack (actions chapters + rules guides); AGENTS.md
# and the target's context.json (deploy section) still deploy.
# --acdsl (default) ships the reach-covered gate slice via `acdsl dist`:
# rules, registry subset, prebuilt bin/acdsl + verifier binaries.
# Precedence: flags > target's context.json > prompt. With a settings file
# present the run is fully non-interactive.
CONTEXT_DIR=""
CONTEXT_DIR_SET=false
LANGS=""
LANGS_SET=false
ROLE=""
ROLE_SET=false
SYMLINK=""
PROSE=y
ACDSL=""
TASK=n
TARGET=""
while [ $# -gt 0 ]; do
  case "$1" in
    --context-dir) CONTEXT_DIR="$2"; CONTEXT_DIR_SET=true; shift 2 ;;
    --langs) LANGS="$2"; LANGS_SET=true; shift 2 ;;
    --role) ROLE="$2"; ROLE_SET=true; shift 2 ;;
    --no-prose) PROSE=n; shift ;;
    --acdsl) ACDSL=y; shift ;;
    --no-acdsl) ACDSL=n; shift ;;
    --task) TASK=y; shift ;;
    --symlink) SYMLINK=y; shift ;;
    --no-symlink) SYMLINK=n; shift ;;
    -*) echo "error: unknown flag: $1"; exit 1 ;;
    *)
      if [ -n "$TARGET" ]; then
        echo "error: multiple targets: $TARGET, $1"
        exit 1
      fi
      TARGET="$1"; shift ;;
  esac
done

if [ -z "$TARGET" ]; then
  printf "Target repository path: "
  read -e TARGET
fi

TARGET="$(cd "$TARGET" && pwd)"

if ! git -C "$TARGET" rev-parse --git-dir >/dev/null 2>&1; then
  echo "error: $TARGET is not a git repository"
  exit 1
fi

# --- Deploy settings (the "deploy" section of the target's context.json) ---
# The settings are target-owned; flags override them, prompts fill what
# neither provides, and the resolved values are written back on every sync.
# Reads tolerate the pre-restructure flat form once (.role at top level).
read_deploy_field() {
  jq -r "($2) // empty" "$1" 2>/dev/null || true
}

CTX_FILE=""
if $CONTEXT_DIR_SET; then
  CTX_FILE="$TARGET/$CONTEXT_DIR/context.json"
elif [ -f "$TARGET/docs/context.json" ]; then
  CTX_FILE="$TARGET/docs/context.json"
fi

if [ -n "$CTX_FILE" ] && [ -f "$CTX_FILE" ]; then
  if ! command -v jq >/dev/null; then
    echo "error: jq is required to read $CTX_FILE"
    exit 1
  fi
  $CONTEXT_DIR_SET || { CONTEXT_DIR="$(read_deploy_field "$CTX_FILE" '.deploy.contextDir // .contextDir')"; [ -n "$CONTEXT_DIR" ] && CONTEXT_DIR_SET=true; }
  $LANGS_SET || { LANGS="$(read_deploy_field "$CTX_FILE" '(.deploy.langs // .langs // []) | join(",")')"; LANGS_SET=true; }
  $ROLE_SET || { ROLE="$(read_deploy_field "$CTX_FILE" '.deploy.role // .role')"; ROLE_SET=true; }
  if [ -z "$SYMLINK" ]; then
    # explicit null check, not `//` — jq's // treats false as absent
    case "$(jq -r 'if .deploy.symlink != null then .deploy.symlink else .symlink end' "$CTX_FILE" 2>/dev/null || true)" in
      true) SYMLINK=y ;;
      false) SYMLINK=n ;;
    esac
  fi
  if [ -z "$ACDSL" ]; then
    case "$(jq -r 'if .deploy.acdsl != null then .deploy.acdsl else null end' "$CTX_FILE" 2>/dev/null || true)" in
      true) ACDSL=y ;;
      false) ACDSL=n ;;
    esac
  fi
fi
ACDSL="${ACDSL:-y}"

# --- Context folder name ---
if ! $CONTEXT_DIR_SET; then
  printf "Context folder name [docs]: "
  read -r CONTEXT_DIR
fi
CONTEXT_DIR="${CONTEXT_DIR:-docs}"

# --- Language selection ---
# Languages are rules/<lang>.md guides; the artifact guides (plan, commits)
# are always synced and never selectable. `general`, `rules`, `style`, and
# `actions` stay accepted in --langs as no-ops for wrapper compatibility.
BASELINE_DIRS="general rules style actions"
ARTIFACT_STYLES="plan commits"
available_langs=()
for guide_file in "$CONTEXT_SRC"/rules/*.md; do
  [ -f "$guide_file" ] || continue
  lang="$(basename "$guide_file" .md)"
  case " $ARTIFACT_STYLES " in *" $lang "*) continue ;; esac
  available_langs+=("$lang")
done

selected_langs=()
if $LANGS_SET; then
  IFS=',' read -r -a requested <<< "$LANGS"
  for lang in ${requested[@]+"${requested[@]}"}; do
    [ -z "$lang" ] && continue
    case " $BASELINE_DIRS " in *" $lang "*) continue ;; esac
    known=false
    for avail in ${available_langs[@]+"${available_langs[@]}"}; do
      [ "$avail" = "$lang" ] && known=true
    done
    if ! $known; then
      echo "error: unknown context dir: $lang"
      exit 1
    fi
    selected_langs+=("$lang")
  done
else
  for lang in ${available_langs[@]+"${available_langs[@]}"}; do
    printf "Include %s context? [Y/n]: " "$lang"
    read -r answer
    answer="${answer:-y}"
    if [[ "$answer" =~ ^[Yy]$ ]]; then
      selected_langs+=("$lang")
    fi
  done
fi

# --- Role line ---
if ! $ROLE_SET || [ -z "$ROLE" ]; then
  printf "Agent role line [You are a development agent for this repository.]: "
  read -r ROLE || true
fi
ROLE="${ROLE:-You are a development agent for this repository.}"

# --- CLAUDE.md symlink ---
if [ -z "$SYMLINK" ]; then
  printf "Create CLAUDE.md symlink to AGENTS.md? [Y/n]: "
  read -r SYMLINK
fi
SYMLINK="${SYMLINK:-y}"

# --- Build AGENTS.md from template ---
echo ""

agents_content="$(cat "$CONTEXT_SRC/AGENTS.md")"

# Strip the leading template-marker comment — it documents the source template,
# not the deployed copy (and substitution would mangle it anyway)
if [[ "$agents_content" == '<!--'* ]]; then
  agents_content="${agents_content#*-->}"
  agents_content="$(echo "$agents_content" | sed '/./,$!d')"
fi

# Replace placeholders
agents_content="${agents_content//\{\{ROLE\}\}/$ROLE}"
agents_content="${agents_content//\{\{CONTEXT_DIR\}\}/$CONTEXT_DIR}"

# Strip unselected language blocks, unwrap selected ones
# ${arr[@]+...} guards empty-array expansion under set -u on bash 3.2.
for lang in ${available_langs[@]+"${available_langs[@]}"}; do
  is_selected=false
  for sel in ${selected_langs[@]+"${selected_langs[@]}"}; do
    [ "$sel" = "$lang" ] && is_selected=true
  done

  if $is_selected; then
    agents_content="$(echo "$agents_content" | sed "/^{{LANG:${lang}}}$/d; /^{{\/LANG:${lang}}}$/d")"
  else
    agents_content="$(echo "$agents_content" | sed "/^{{LANG:${lang}}}$/,/^{{\/LANG:${lang}}}$/d")"
  fi
done

echo "$agents_content" > "$TARGET/AGENTS.md"
echo "  AGENTS.md -> $TARGET/AGENTS.md"

# --- Sync baseline: actions/ (ownership-headed) + rules/ guides ---
# Baseline copies pass through `cmd/rules filter` — entries whose reach does
# not cover this target and unselected-language entries never ship. Deploying
# unfiltered would leak repo-reach entries, so a rules tool is hard-required:
# a prebuilt bin/rules (Windows installer payload) or go + cmd/rules.
if [ ! -x "$RULES_REPO/bin/rules" ] && [ ! -x "$RULES_REPO/bin/rules.exe" ] \
  && { ! command -v go >/dev/null || [ ! -d "$RULES_REPO/cmd/rules" ]; }; then
  echo "error: bin/rules (prebuilt) or go and cmd/rules are required — baseline files deploy filtered (reach/lang)"
  exit 1
fi

CTX_ROOT="$TARGET/$CONTEXT_DIR"
mkdir -p "$CTX_ROOT/actions" "$CTX_ROOT/rules" "$CTX_ROOT/facts"

# Always-read global content (company facts etc.) — plain copy, unfiltered;
# injected per session by global-context.sh.
if [ "$PROSE" = y ] && [ -d "$CONTEXT_SRC/global" ]; then
  mkdir -p "$CTX_ROOT/global"
  cp "$CONTEXT_SRC/global"/*.md "$CTX_ROOT/global/" 2>/dev/null || true
fi

TARGET_NAME="$(basename "$TARGET")"
LANGS_CSV=""
for lang in ${selected_langs[@]+"${selected_langs[@]}"}; do
  LANGS_CSV="${LANGS_CSV:+$LANGS_CSV,}$lang"
done

BASELINE_HEADER="<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->"
if [ "$PROSE" = y ]; then
for f in "$CONTEXT_SRC/actions"/*.md; do
  [ -f "$f" ] || continue
  name="$(basename "$f")"
  dst="$CTX_ROOT/actions/$name"
  if [ -f "$dst" ] && ! head -1 "$dst" | grep -qF "synced from smine"; then
    echo "error: $dst is repo-owned but collides with a baseline file name"
    exit 1
  fi
  { echo "$BASELINE_HEADER"; echo ""; \
    smine_tool rules filter --target "$TARGET_NAME" --langs "$LANGS_CSV" "$f"; } > "$dst"
  echo "  actions/$name -> $dst"
done

# --- Copy rules guides: artifact guides always, languages as selected ---
# Baseline-owned and always overwritten; the header keeps origin detection
# marking them baseline in the target's generated context.json. Guides pass
# the same filter so a repo-reach RULE headline never ships.
copy_rules_guide() {
  { echo "$BASELINE_HEADER"; echo ""; \
    smine_tool rules filter --target "$TARGET_NAME" --langs "$LANGS_CSV" "$CONTEXT_SRC/rules/$1.md"; } > "$CTX_ROOT/rules/$1.md"
  echo "  rules/$1.md -> $CTX_ROOT/rules/$1.md"
}
for name in $ARTIFACT_STYLES; do
  if [ -f "$CONTEXT_SRC/rules/$name.md" ]; then
    copy_rules_guide "$name"
  fi
done
for lang in ${selected_langs[@]+"${selected_langs[@]}"}; do
  copy_rules_guide "$lang"
done

# --- Copy facts: reach-covered entries only ---
# Fact entries are repo-scoped via Reach; the filter drops non-covered ones.
# A file left with no entries after filtering is skipped entirely, so a
# target never receives another repo's (or an empty) facts file.
for f in "$CONTEXT_SRC/facts"/*.md; do
  [ -f "$f" ] || continue
  name="$(basename "$f")"
  dst="$CTX_ROOT/facts/$name"
  if [ -f "$dst" ] && ! head -1 "$dst" | grep -qF "synced from smine"; then
    echo "error: $dst is repo-owned but collides with a baseline file name"
    exit 1
  fi
  filtered="$(smine_tool rules filter --target "$TARGET_NAME" --langs "$LANGS_CSV" "$f")"
  if ! printf '%s\n' "$filtered" | grep -q '^\*\*FACT-'; then
    continue
  fi
  { echo "$BASELINE_HEADER"; echo ""; printf '%s\n' "$filtered"; } > "$dst"
  echo "  facts/$name -> $dst"
done
else
  echo "  prose pack skipped (--no-prose)"
fi

# --- Sync central facts: reach-filtered, independent of --no-prose ---
# Facts are entries like actions/rules, not prose pack; a file ships only when
# at least one FACT entry survives the reach filter for this target.
for f in "$CONTEXT_SRC/facts"/*.md; do
  [ -f "$f" ] || continue
  name="$(basename "$f")"
  filtered="$(smine_tool rules filter --target "$TARGET_NAME" --langs "$LANGS_CSV" "$f")"
  if ! grep -q '^\*\*FACT-' <<< "$filtered"; then
    continue
  fi
  dst="$CTX_ROOT/facts/$name"
  if [ -f "$dst" ] && ! head -1 "$dst" | grep -qF "synced from smine"; then
    echo "error: $dst is repo-owned but collides with a baseline file name"
    exit 1
  fi
  { echo "$BASELINE_HEADER"; echo ""; echo "$filtered"; } > "$dst"
  echo "  facts/$name -> $dst"
done

# --- Sync the gate slice: reach-covered rules + registry + binaries ---
# Binaries land under the target's bin/ — sync never touches the target's
# .gitignore; committing or ignoring them is the target's call.
if [ "$ACDSL" = y ]; then
  if [ "$TASK" = y ]; then
    smine_tool acdsl dist -task -target "$TARGET_NAME" -dest "$TARGET"
  else
    smine_tool acdsl dist -target "$TARGET_NAME" -dest "$TARGET"
  fi
else
  echo "  acdsl slice skipped (--no-acdsl)"
fi

# --- CLAUDE.md symlink ---
if [[ "$SYMLINK" =~ ^[Yy]$ ]]; then
  ln -sf AGENTS.md "$TARGET/CLAUDE.md"
  echo "  CLAUDE.md -> AGENTS.md (symlink)"
fi

# --- Write the target's context.json (deploy section + entries + aspects) ---
# Generation validates the merged tree first and refuses on violations, so
# this step is also the deployed-context validation. go is already
# hard-required above (baseline files deploy filtered).
CTX_FILE="$CTX_ROOT/context.json"
if ! command -v jq >/dev/null; then
  echo "  warning: jq not found — context.json not written"
else
  langs_json="$(printf '%s\n' ${selected_langs[@]+"${selected_langs[@]}"} | jq -R . | jq -s 'map(select(length > 0))')"
  symlink_json=false
  [[ "$SYMLINK" =~ ^[Yy]$ ]] && symlink_json=true
  acdsl_json=false
  [ "$ACDSL" = y ] && acdsl_json=true
  deploy_json="$(jq -n \
    --arg role "$ROLE" \
    --arg contextDir "$CONTEXT_DIR" \
    --argjson langs "$langs_json" \
    --argjson symlink "$symlink_json" \
    --argjson acdsl "$acdsl_json" \
    '{role: $role, contextDir: $contextDir, langs: $langs, symlink: $symlink, acdsl: $acdsl}')"
  generated="$(mktemp)"
  if ! smine_tool rules generate --deployed "$CTX_ROOT" -out "$generated"; then
    rm -f "$generated"
    echo "error: context validation failed for $CTX_ROOT"
    exit 1
  fi
  jq --argjson deploy "$deploy_json" '{deploy: $deploy} + .' "$generated" > "$CTX_FILE"
  rm -f "$generated"
  echo "  context.json -> $CTX_FILE"
fi

echo ""
echo "synced context to $TARGET"
