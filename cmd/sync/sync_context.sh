#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
CONTEXT_SRC="$REPO_DIR/context"

# --- Arguments ---
# usage: sync_context.sh [--context-dir NAME] [--langs a,b,c] [--role TEXT] [--symlink|--no-symlink] [TARGET]
# Precedence: flags > target's context-pack.json > prompt. With a pack file
# present the run is fully non-interactive.
CONTEXT_DIR=""
CONTEXT_DIR_SET=false
LANGS=""
LANGS_SET=false
ROLE=""
ROLE_SET=false
SYMLINK=""
TARGET=""
while [ $# -gt 0 ]; do
  case "$1" in
    --context-dir) CONTEXT_DIR="$2"; CONTEXT_DIR_SET=true; shift 2 ;;
    --langs) LANGS="$2"; LANGS_SET=true; shift 2 ;;
    --role) ROLE="$2"; ROLE_SET=true; shift 2 ;;
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

# --- Pack config (context-pack.json) ---
# The pack file is repo-owned; flags override it, prompts fill what neither
# provides, and the resolved values are written back on first sync.
read_pack_field() {
  jq -r "$2 // empty" "$1" 2>/dev/null || true
}

PACK_FILE=""
if $CONTEXT_DIR_SET; then
  PACK_FILE="$TARGET/$CONTEXT_DIR/context-pack.json"
elif [ -f "$TARGET/docs/context-pack.json" ]; then
  PACK_FILE="$TARGET/docs/context-pack.json"
fi

if [ -n "$PACK_FILE" ] && [ -f "$PACK_FILE" ]; then
  if ! command -v jq >/dev/null; then
    echo "error: jq is required to read $PACK_FILE"
    exit 1
  fi
  $CONTEXT_DIR_SET || { CONTEXT_DIR="$(read_pack_field "$PACK_FILE" '.contextDir')"; [ -n "$CONTEXT_DIR" ] && CONTEXT_DIR_SET=true; }
  $LANGS_SET || { LANGS="$(read_pack_field "$PACK_FILE" '.langs | join(",")')"; LANGS_SET=true; }
  $ROLE_SET || { ROLE="$(read_pack_field "$PACK_FILE" '.role')"; ROLE_SET=true; }
  if [ -z "$SYMLINK" ]; then
    # plain .symlink, not `// empty` — jq's // treats false as absent
    case "$(jq -r '.symlink' "$PACK_FILE" 2>/dev/null || true)" in
      true) SYMLINK=y ;;
      false) SYMLINK=n ;;
    esac
  fi
fi

# --- Context folder name ---
if ! $CONTEXT_DIR_SET; then
  printf "Context folder name [docs]: "
  read -r CONTEXT_DIR
fi
CONTEXT_DIR="${CONTEXT_DIR:-docs}"

# --- Language selection ---
# Languages are style/<lang>.md files; the artifact guides (plan, commits) are
# always synced and never selectable. `general` and `rules` stay accepted in
# --langs as no-ops for wrapper compatibility.
BASELINE_DIRS="general rules style"
ARTIFACT_STYLES="plan commits"
available_langs=()
for style_file in "$CONTEXT_SRC"/style/*.md; do
  [ -f "$style_file" ] || continue
  lang="$(basename "$style_file" .md)"
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
    for avail in "${available_langs[@]}"; do
      [ "$avail" = "$lang" ] && known=true
    done
    if ! $known; then
      echo "error: unknown context dir: $lang"
      exit 1
    fi
    selected_langs+=("$lang")
  done
else
  for lang in "${available_langs[@]}"; do
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
agents_content="$(echo "$agents_content" | sed "s|{{CONTEXT_DIR}}|$CONTEXT_DIR|g")"

# Strip unselected language blocks, unwrap selected ones
for lang in "${available_langs[@]}"; do
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

# --- Sync baseline: rules/ (ownership-headed) + style/ artifact guides ---
PACK_ROOT="$TARGET/$CONTEXT_DIR"
mkdir -p "$PACK_ROOT/rules" "$PACK_ROOT/style" "$PACK_ROOT/facts"

BASELINE_HEADER="<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->"
for f in "$CONTEXT_SRC/rules"/*.md; do
  [ -f "$f" ] || continue
  name="$(basename "$f")"
  dst="$PACK_ROOT/rules/$name"
  if [ -f "$dst" ] && ! head -1 "$dst" | grep -qF "synced from smine"; then
    echo "error: $dst is repo-owned but collides with a baseline file name"
    exit 1
  fi
  { echo "$BASELINE_HEADER"; echo ""; cat "$f"; } > "$dst"
  echo "  rules/$name -> $dst"
done

# Vocabulary is baseline-owned and always overwritten (JSON carries no header marker)
if [ -f "$CONTEXT_SRC/rules/aspects.json" ]; then
  cp "$CONTEXT_SRC/rules/aspects.json" "$PACK_ROOT/rules/aspects.json"
  echo "  rules/aspects.json -> $PACK_ROOT/rules/aspects.json"
fi

# --- Copy style guides: artifact guides always, languages as selected ---
for name in $ARTIFACT_STYLES; do
  if [ -f "$CONTEXT_SRC/style/$name.md" ]; then
    cp "$CONTEXT_SRC/style/$name.md" "$PACK_ROOT/style/$name.md"
    echo "  style/$name.md -> $PACK_ROOT/style/$name.md"
  fi
done
for lang in ${selected_langs[@]+"${selected_langs[@]}"}; do
  cp "$CONTEXT_SRC/style/$lang.md" "$PACK_ROOT/style/$lang.md"
  echo "  style/$lang.md -> $PACK_ROOT/style/$lang.md"
done

# --- Write context-pack.json (first sync only) ---
PACK_FILE="$PACK_ROOT/context-pack.json"
if [ ! -f "$PACK_FILE" ]; then
  if command -v jq >/dev/null; then
    langs_json="$(printf '%s\n' ${selected_langs[@]+"${selected_langs[@]}"} | jq -R . | jq -s 'map(select(length > 0))')"
    symlink_json=false
    [[ "$SYMLINK" =~ ^[Yy]$ ]] && symlink_json=true
    jq -n \
      --arg role "$ROLE" \
      --arg contextDir "$CONTEXT_DIR" \
      --argjson langs "$langs_json" \
      --argjson symlink "$symlink_json" \
      '{role: $role, contextDir: $contextDir, langs: $langs, symlink: $symlink}' > "$PACK_FILE"
    echo "  context-pack.json -> $PACK_FILE"
  else
    echo "  warning: jq not found — context-pack.json not written"
  fi
fi

# --- CLAUDE.md symlink ---
if [[ "$SYMLINK" =~ ^[Yy]$ ]]; then
  ln -sf AGENTS.md "$TARGET/CLAUDE.md"
  echo "  CLAUDE.md -> AGENTS.md (symlink)"
fi

# --- Validate the merged pack ---
if [ -d "$REPO_DIR/cmd/rules" ] && command -v go >/dev/null; then
  if ! (cd "$REPO_DIR" && go run ./cmd/rules validate --pack "$PACK_ROOT"); then
    echo "error: pack validation failed for $PACK_ROOT"
    exit 1
  fi
else
  echo "  warning: pack validation skipped (go or cmd/rules unavailable)"
fi

echo ""
echo "synced context pack to $TARGET"
