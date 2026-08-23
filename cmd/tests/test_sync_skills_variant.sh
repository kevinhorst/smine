#!/usr/bin/env bash
# Regression coverage for sync_skills.sh --variant: a skill renders under a
# distinct <leaf>--<name> with the disabled entries stripped and its
# frontmatter name suffixed; bad names, unknown leaves and unknown ids fail;
# --prune removes variant leftovers without asking.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/sync-variant.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == sync-variant.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

# A sandbox repo copy: the real cmd/sync + cmd/rules + a tiny skills tree.
sandbox="$TMP/repo"
mkdir -p "$sandbox/cmd/sync" "$sandbox/skills/group/demo" "$TMP/home/.claude/hooks" "$TMP/root"
cp "$REPO_DIR/cmd/sync/sync_skills.sh" "$REPO_DIR/cmd/sync/smine_tool.sh" "$sandbox/cmd/sync/"
mkdir -p "$sandbox/cmd/worktrees/_lib"
printf "#!/usr/bin/env bash\n" > "$sandbox/cmd/worktrees/remove_agent_worktrees.sh"
printf "# lib\n" > "$sandbox/cmd/worktrees/_lib/verdict.sh"
mkdir -p "$sandbox/bin"
cat > "$sandbox/bin/rules" <<EOF
#!/usr/bin/env bash
exec go run -C "$REPO_DIR" ./cmd/rules "\$@"
EOF
chmod +x "$sandbox/bin/rules"
printf 'AGENT_CONTEXT_DIR_DEFAULT=ctx\n' > "$TMP/home/.claude/hooks/global-context.env"
cat > "$sandbox/skills/group/demo/SKILL.md" <<'EOF'
---
name: demo
description: demo. Trigger on /demo.
author: Kevin Horst
version: 1.0
---
# Demo

Read $AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md.

## Intake

**SKILL-DEMO-INTAKE-001** `[step]` — Ask for the feature name.

## Templates

**SKILL-DEMO-TPL-001** `[payload]` — Overview template.

```markdown
# Concept
```
EOF
printf 'sibling\n' > "$sandbox/skills/group/demo/notes.md"

SYNC="$sandbox/cmd/sync/sync_skills.sh"
run_sync() { rc=0; out=$(HOME="$TMP/home" bash "$SYNC" "$@" 2>&1) || rc=$?; }

test_variant_renders_under_suffixed_name() {
  run_sync --variant "demo=no-tpl:SKILL-DEMO-TPL-*" --into "$TMP/root"
  [ "$rc" -eq 0 ] || fail "variant render failed: $out"
  target="$TMP/root/demo--no-tpl/SKILL.md"
  [ -f "$target" ] || fail "variant SKILL.md missing"
  grep -q '^name: demo--no-tpl$' "$target" || fail "frontmatter name not suffixed"
  grep -q 'SKILL-DEMO-INTAKE-001' "$target" || fail "kept entry missing"
  ! grep -q 'SKILL-DEMO-TPL-001' "$target" || fail "disabled payload still present"
  ! grep -q '```markdown' "$target" || fail "payload fence still present"
  ! grep -q '## Templates' "$target" || fail "emptied section heading still present"
  grep -q 'Read ctx/rules/plan.md' "$target" || fail "context dir not substituted"
  [ -f "$TMP/root/demo--no-tpl/notes.md" ] || fail "sibling file not copied"
}

test_bad_variant_name() {
  run_sync --variant "demo=No_Tpl:SKILL-DEMO-TPL-*" --into "$TMP/root"
  [ "$rc" -eq 1 ] || fail "bad name accepted: $out"
}

test_unknown_leaf() {
  run_sync --variant "nope=x:SKILL-NOPE-A-001" --into "$TMP/root"
  [ "$rc" -eq 1 ] || fail "unknown leaf accepted: $out"
}

test_unknown_id_fails() {
  run_sync --variant "demo=bad:SKILL-DEMO-NOPE-001" --into "$TMP/root"
  [ "$rc" -ne 0 ] || fail "unknown id accepted: $out"
  case "$out" in *"matches no entry"*) : ;; *) fail "render error not surfaced: $out" ;; esac
}

test_unknown_id_leaves_no_variant_dir() {
  [ ! -d "$TMP/root/demo--bad" ] || fail "failed render left demo--bad behind"
}

test_list_entries_json() {
  listing=$("$sandbox/bin/rules" render-skill --list-entries "$sandbox/skills/group/demo/SKILL.md") || fail "list-entries exited non-zero"
  echo "$listing" | jq -e 'length == 2 and .[0].id == "SKILL-DEMO-INTAKE-001" and .[0].class == "step" and .[0].statement == "Ask for the feature name." and .[0].line == 13 and .[1].payload == true' >/dev/null || fail "list-entries JSON shape: $listing"
  broken="$TMP/broken.md"
  printf -- '---\nname: demo\n---\n# Demo\n\n**SKILL-DEMO-INTAKE-001** `[step]` — First.\n\n**SKILL-DEMO-INTAKE-002** `[bogus]` — Unknown class.\n' > "$broken"
  rc=0; err=$("$sandbox/bin/rules" render-skill --list-entries "$broken" 2>&1 >/dev/null) || rc=$?
  [ "$rc" -eq 1 ] || fail "violations must exit 1, got $rc: $err"
  [ -n "$err" ] || fail "violation not reported on stderr"
}

test_prune_removes_variant_leftovers() {
  mkdir -p "$TMP/home/.claude/skills/demo--old" "$TMP/home/.codex/skills"
  touch "$TMP/home/.claude/skills/demo--old/SKILL.md"
  run_sync --prune
  [ "$rc" -eq 0 ] || fail "prune run failed: $out"
  [ ! -d "$TMP/home/.claude/skills/demo--old" ] || fail "variant leftover not pruned"
  [ -f "$TMP/home/.claude/skills/demo/SKILL.md" ] || fail "real skill not installed"
}

test_variant_renders_under_suffixed_name
test_bad_variant_name
test_unknown_leaf
test_unknown_id_fails
test_unknown_id_leaves_no_variant_dir
test_list_entries_json
test_prune_removes_variant_leftovers

echo "PASS: test_sync_skills_variant.sh"
