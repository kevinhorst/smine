#!/usr/bin/env bash
# Regression coverage for sync_settings.sh --serena mode, focused on
# patch_serena_config: template discovery / seeding, in-place patching of
# web_dashboard_open_on_launch, and the idempotent + skip branches. The whole
# script is driven with a temp $HOME (matching test_sync_context.sh's style of
# running the real script under controlled dirs), so the merge_mcp_servers
# {}-seed path is exercised on the way through as well.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
SCRIPT="$REPO_DIR/cmd/sync/sync_settings.sh"
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/sync-settings.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == sync-settings.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_exists() {
  [ -e "$1" ] || fail "expected $1 to exist"
}

assert_missing() {
  [ ! -e "$1" ] || fail "expected $1 to be absent"
}

assert_contains() {
  grep -qF "$2" "$1" || fail "expected $1 to contain: $2"
}

assert_not_contains() {
  ! grep -qF "$2" "$1" || fail "expected $1 to not contain: $2"
}

# Build a fresh, isolated $HOME with the dirs sync_settings.sh writes into and
# a pre-seeded hooks env so the DOCS_DIR read prompt is never hit. Echoes the
# new HOME path.
#
# The empty serena-agent discovery dir is deliberate: with `set -o pipefail`,
# patch_serena_config's `find "$HOME/.local/.../serena-agent" ... | head -1`
# aborts the whole script if that dir is *missing* (find exits 1). Creating it
# empty lets the skip branch resolve to "no template found" via find's normal
# exit-0-empty result, which is the branch under test.
#
# Pass "no-serena-agent" as $1 to omit that discovery dir entirely, reproducing
# a fresh machine with serena never installed — the case the `|| true` guard on
# the find pipeline handles (find exits 1, pipefail would otherwise abort under
# set -e).
new_home() {
  local home make_serena_dir=1
  [ "${1:-}" = "no-serena-agent" ] && make_serena_dir=0
  home=$(mktemp -d "$TMP/home.XXXXXX")
  mkdir -p "$home/.claude/hooks" "$home/.codex"
  [ "$make_serena_dir" = 1 ] && mkdir -p "$home/.local/share/uv/tools/serena-agent"
  printf 'AGENT_CONTEXT_DIR_DEFAULT=docs\nGLOBAL_CONTEXT_ENABLED=1\n' \
    > "$home/.claude/hooks/global-context.env"
  echo "$home"
}

# Plant the packaged serena template at the discovery path under $HOME so the
# seed branch can find it via the script's `find ... serena-agent` lookup.
seed_template() {
  local home="$1" dir
  dir="$home/.local/share/uv/tools/serena-agent/lib/serena"
  mkdir -p "$dir"
  printf '%s\n' '# serena packaged template' \
    'web_dashboard_open_on_launch: true' \
    'other_key: keep_me' > "$dir/serena_config.template.yml"
}

# Plant a template whose web_dashboard_open_on_launch key is commented out, so
# the seed copies a cfg the targeted sed cannot rewrite (the line starts with
# '#'). The append-when-sed-cannot-rewrite branch is the only thing that makes
# the pin effective here.
seed_template_commented() {
  local home="$1" dir
  dir="$home/.local/share/uv/tools/serena-agent/lib/serena"
  mkdir -p "$dir"
  printf '%s\n' '# serena packaged template' \
    '# web_dashboard_open_on_launch: true' \
    'other_key: keep_me' > "$dir/serena_config.template.yml"
}

run_sync() {
  local home="$1" out="$2"
  HOME="$home" bash "$SCRIPT" --serena >"$out" 2>&1
}

# 1. skip branch: no cfg and no discoverable template.
test_serena_skip_no_cfg_no_template() {
  local home out
  home=$(new_home)
  out=$TMP/skip.out
  run_sync "$home" "$out"
  assert_contains "$out" "skip: serena config"
  assert_missing "$home/.serena/serena_config.yml"
}

# 2. seed-then-patch: template present, no cfg -> cfg created from template and
#    then pinned to false.
test_serena_seed_then_patch() {
  local home out cfg
  home=$(new_home)
  seed_template "$home"
  out=$TMP/seed.out
  run_sync "$home" "$out"
  cfg="$home/.serena/serena_config.yml"
  assert_exists "$cfg"
  assert_contains "$out" "created: $cfg (from packaged template)"
  assert_contains "$out" "patched: $cfg"
  assert_contains "$cfg" "web_dashboard_open_on_launch: false"
  assert_not_contains "$cfg" "web_dashboard_open_on_launch: true"
  # non-target keys from the template survive the seed+patch.
  assert_contains "$cfg" "other_key: keep_me"
}

# 3. patch existing: cfg already present with true -> rewritten to false
#    (exercises the mktemp + mv path).
test_serena_patch_existing() {
  local home out cfg
  home=$(new_home)
  mkdir -p "$home/.serena"
  cfg="$home/.serena/serena_config.yml"
  printf '%s\n' 'web_dashboard_open_on_launch: true' 'gui_log_window: true' > "$cfg"
  out=$TMP/patch.out
  run_sync "$home" "$out"
  assert_contains "$out" "patched: $cfg"
  assert_contains "$cfg" "web_dashboard_open_on_launch: false"
  assert_not_contains "$cfg" "web_dashboard_open_on_launch: true"
  # unrelated lines untouched by the targeted sed.
  assert_contains "$cfg" "gui_log_window: true"
}

# 4. idempotent: cfg already false -> reported unchanged and left byte-identical.
test_serena_idempotent() {
  local home out cfg
  home=$(new_home)
  mkdir -p "$home/.serena"
  cfg="$home/.serena/serena_config.yml"
  printf '%s\n' 'web_dashboard_open_on_launch: false' 'gui_log_window: true' > "$cfg"
  cp "$cfg" "$TMP/cfg.before"
  out=$TMP/idem.out
  run_sync "$home" "$out"
  assert_contains "$out" "unchanged: $cfg (web_dashboard_open_on_launch already false)"
  assert_not_contains "$out" "patched: $cfg"
  cmp -s "$cfg" "$TMP/cfg.before" || fail "idempotent run must leave $cfg byte-identical"
}

# 5. merge_mcp_servers {}-seed on a missing ~/.claude.json (exercised by the
#    full-script drive: fresh temp HOME has no .claude.json).
test_mcp_servers_seed_and_merge() {
  local home out dst
  home=$(new_home)
  dst="$home/.claude.json"
  assert_missing "$dst"
  out=$TMP/mcp.out
  run_sync "$home" "$out"
  assert_exists "$dst"
  assert_contains "$out" "created: $dst (empty seed"
  # the repo-managed servers landed under .mcpServers and the file is valid JSON.
  jq -e '.mcpServers | type == "object" and (length > 0)' "$dst" >/dev/null \
    || fail "expected merged .mcpServers object in $dst"
}

# 5b. merge_mcp_servers preserves foreign top-level keys when ~/.claude.json
#     already exists — the load-bearing invariant (oauth/caches/machineID must
#     survive the jq merge, only .mcpServers is touched).
test_mcp_servers_preserve_existing_keys() {
  local home out dst
  home=$(new_home)
  dst="$home/.claude.json"
  printf '%s\n' '{"oauth":"KEEP-ME"}' > "$dst"
  out=$TMP/mcp-preserve.out
  run_sync "$home" "$out"
  # the pre-existing key survives AND the repo servers were merged in.
  jq -e '.oauth == "KEEP-ME"' "$dst" >/dev/null \
    || fail "expected pre-existing top-level key to survive merge in $dst"
  jq -e '.mcpServers | type == "object" and (length > 0)' "$dst" >/dev/null \
    || fail "expected merged .mcpServers object in $dst"
}

# 6. dir-absent skip: serena-agent discovery dir does not exist at all.
#    Without the `|| true` guard on the find pipeline, pipefail + set -e abort
#    the whole script here (before the hooks sync) — this run must exit 0 and
#    skip.
test_serena_skip_dir_absent() {
  local home out
  home=$(new_home no-serena-agent)
  assert_missing "$home/.local/share/uv/tools/serena-agent"
  out=$TMP/skip-absent.out
  run_sync "$home" "$out"   # run_sync uses set -e; a nonzero script exit fails the test here
  assert_contains "$out" "skip: serena config"
  assert_missing "$home/.serena/serena_config.yml"
  # the script reached the end (past patch_serena_config) rather than aborting.
  assert_contains "$out" "wrote: $home/.claude/hooks/global-context.env"
}

# 8. env migration: a machine still on the retired review-context.env keeps
#    its context dir and toggle state; the old file is removed.
test_hooks_env_migration() {
  local home out env
  home=$(new_home)
  rm -f "$home/.claude/hooks/global-context.env"
  printf 'AGENT_CONTEXT_DIR_DEFAULT=docs\nREVIEW_CONTEXT_ENABLED=0\n' \
    > "$home/.claude/hooks/review-context.env"
  out=$TMP/migration.out
  run_sync "$home" "$out"
  env="$home/.claude/hooks/global-context.env"
  assert_exists "$env"
  grep -q '^GLOBAL_CONTEXT_ENABLED=0' "$env" || fail "toggle state not migrated in $env"
  grep -q '^AGENT_CONTEXT_DIR_DEFAULT=docs' "$env" || fail "context dir not migrated in $env"
  assert_missing "$home/.claude/hooks/review-context.env"
}

# 7. commented/absent-key patch: template ships the key commented, no cfg ->
#    the seeded cfg must still end up carrying an uncommented false pin.
test_serena_patch_commented_key() {
  local home out cfg
  home=$(new_home)
  seed_template_commented "$home"
  out=$TMP/commented.out
  run_sync "$home" "$out"
  cfg="$home/.serena/serena_config.yml"
  assert_exists "$cfg"
  assert_contains "$out" "patched: $cfg"
  # effective override present as an uncommented line, not just echoed.
  grep -q '^web_dashboard_open_on_launch: false' "$cfg" \
    || fail "expected an uncommented false pin in $cfg"
  # the original commented line is preserved (append, not rewrite).
  assert_contains "$cfg" "# web_dashboard_open_on_launch: true"
}

test_serena_skip_no_cfg_no_template
test_serena_seed_then_patch
test_serena_patch_existing
test_serena_idempotent
test_mcp_servers_seed_and_merge
test_mcp_servers_preserve_existing_keys
test_serena_skip_dir_absent
test_serena_patch_commented_key
test_hooks_env_migration

echo "test_sync_settings: OK"
