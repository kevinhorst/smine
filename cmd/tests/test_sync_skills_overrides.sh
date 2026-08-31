#!/usr/bin/env bash
# Coverage for sync_skills.sh skill-visibility handling (presentation profile):
# a casual profile writes per-leaf skillOverrides "off" into the user
# settings and a routine overlay with the same leaves "on"; a legacy
# non-developer value is migrated in place to casual first; foreign settings
# keys survive; the run is idempotent; a developer profile (or none) drops the
# leaf overrides and deletes the overlay.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/sync-overrides.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == sync-overrides.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

# Sandbox repo copy: the real sync_skills + a tiny skills tree (variant-test shape).
sandbox="$TMP/repo"
mkdir -p "$sandbox/cmd/sync" "$sandbox/skills/group/demo" "$TMP/home/.claude/hooks" \
  "$TMP/home/.claude/context/global" "$TMP/home/.config/claude-routine"
cp "$REPO_DIR/cmd/sync/sync_skills.sh" "$REPO_DIR/cmd/sync/smine_tool.sh" "$sandbox/cmd/sync/"
mkdir -p "$sandbox/cmd/worktrees/_lib"
printf "#!/usr/bin/env bash\n" > "$sandbox/cmd/worktrees/remove_agent_worktrees.sh"
printf "# lib\n" > "$sandbox/cmd/worktrees/_lib/verdict.sh"
printf 'AGENT_CONTEXT_DIR_DEFAULT=ctx\n' > "$TMP/home/.claude/hooks/global-context.env"
cat > "$sandbox/skills/group/demo/SKILL.md" <<'EOF'
---
name: demo
description: demo. Trigger on /demo.
author: Kevin Horst
version: 1.0
---
# Demo
EOF

settings="$TMP/home/.claude/settings.json"
overlay="$TMP/home/.config/claude-routine/skill-overrides.json"
profile="$TMP/home/.claude/context/global/presentation-profile.md"
printf '{"model": "opus"}\n' > "$settings"

run_sync() { rc=0; out=$(HOME="$TMP/home" bash "$sandbox/cmd/sync/sync_skills.sh" --prune "$@" 2>&1) || rc=$?; }

test_casual_writes_overrides_and_overlay() {
  printf -- '---\nlanguage: de\naudience: casual\n---\n' > "$profile"
  run_sync
  [ "$rc" -eq 0 ] || fail "sync failed: $out"
  [ "$(jq -r '.skillOverrides.demo' "$settings")" = "off" ] || fail "demo not off in settings"
  [ "$(jq -r '.model' "$settings")" = "opus" ] || fail "foreign settings key lost"
  [ -f "$overlay" ] || fail "overlay missing"
  [ "$(jq -r '.skillOverrides.demo' "$overlay")" = "on" ] || fail "demo not on in overlay"
  [ "$(jq -r '.skillOverrides.demo' "$sandbox/.claude/settings.local.json")" = "on" ] || fail "demo not on in repo-local settings"
}

test_second_run_is_idempotent() {
  before=$(jq -S . "$settings")
  run_sync
  [ "$rc" -eq 0 ] || fail "second sync failed: $out"
  after=$(jq -S . "$settings")
  [ "$before" = "$after" ] || fail "settings changed on second run"
}

test_developer_drops_overrides_and_overlay() {
  printf -- '---\nlanguage: en\naudience: developer\n---\n' > "$profile"
  run_sync
  [ "$rc" -eq 0 ] || fail "developer sync failed: $out"
  [ "$(jq -r '.skillOverrides.demo // "absent"' "$settings")" = "absent" ] || fail "leaf override survived"
  [ "$(jq -r '.model' "$settings")" = "opus" ] || fail "foreign settings key lost on drop"
  [ ! -f "$overlay" ] || fail "overlay survived developer flip"
  [ "$(jq -r '.skillOverrides.demo // "absent"' "$sandbox/.claude/settings.local.json")" = "absent" ] || fail "repo-local override survived flip"
}

test_legacy_non_developer_migrated_to_casual() {
  printf -- '---\nlanguage: de\naudience: non-developer\n---\n' > "$profile"
  run_sync
  [ "$rc" -eq 0 ] || fail "migration sync failed: $out"
  grep -q '^audience: casual$' "$profile" || fail "legacy audience not migrated in place"
  [ "$(jq -r '.skillOverrides.demo' "$settings")" = "off" ] || fail "demo not off after migration"
}

test_casual_writes_overrides_and_overlay
test_second_run_is_idempotent
test_developer_drops_overrides_and_overlay
test_legacy_non_developer_migrated_to_casual
echo "PASS: sync_skills skill-visibility overrides"
