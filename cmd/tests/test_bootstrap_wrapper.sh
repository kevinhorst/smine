#!/usr/bin/env bash
# Coverage for cmd/bootstrap/run.sh (one-shot bootstrap orchestrator):
# dry-run stage assembly — --last default vs --since (incl. the orchestrate
# (since:) suffix), dev-mode threading from the presentation profile with the
# casual force-off, the consolidate language arg, and the clean-tree/main
# preflight gate. Uses BOOTSTRAP_DRY_RUN=1 and a fake HOME; git + jq only.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/bootstrap-wrapper.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == bootstrap-wrapper.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

# ---- sandbox repo: the wrapper + the _lib files it sources ----
sandbox="$TMP/repo"
mkdir -p "$sandbox/cmd/bootstrap" "$sandbox/routines/_lib" "$sandbox/proposals"
cp "$REPO_DIR/cmd/bootstrap/run.sh" "$sandbox/cmd/bootstrap/"
cp "$REPO_DIR/routines/_lib/platform.sh" "$REPO_DIR/routines/_lib/skill.sh" "$sandbox/routines/_lib/"

git -C "$sandbox" init -q
git -C "$sandbox" config user.email test@example.com
git -C "$sandbox" config user.name test
git -C "$sandbox" checkout -qb main
git -C "$sandbox" add -A
git -C "$sandbox" commit -qm init

fake_home="$TMP/home"
mkdir -p "$fake_home/.claude/context/global"

run_wrapper() {
  HOME="$fake_home" BOOTSTRAP_DRY_RUN=1 bash "$sandbox/cmd/bootstrap/run.sh" "$@"
}

# ---- case: defaults — --last 30, no --dev, 5 stages ----
out="$(run_wrapper)"
[ "$(printf '%s\n' "$out" | grep -c '^DRY:')" = 5 ] || fail "defaults: expected 5 DRY stage lines, got: $out"
printf '%s' "$out" | grep -q -- '--last 30' || fail "defaults: mine stage missing --last 30"
printf '%s' "$out" | grep -q -- '--dev' && fail "defaults: unexpected --dev"
printf '%s' "$out" | grep -q '"/smine-orchestrate bootstrap"' || fail "defaults: orchestrate prompt wrong"

# ---- case: since wins over last, orchestrate carries (since:) ----
out="$(BOOTSTRAP_SINCE=2026-08-01 BOOTSTRAP_N=7 run_wrapper)"
printf '%s' "$out" | grep -q -- '--since 2026-08-01' || fail "since: mine stage missing --since"
printf '%s' "$out" | grep -q -- '--last' && fail "since: --last must be dropped"
printf '%s' "$out" | grep -q '(since: 2026-08-01)' || fail "since: orchestrate missing (since:) suffix"

# ---- case: dev-mode profile (developer) → --dev; language de → consolidate language ----
cat > "$fake_home/.claude/context/global/presentation-profile.md" <<'EOF'
---
language: de
audience:
dev-mode: true
---
EOF
out="$(run_wrapper)"
printf '%s' "$out" | grep -q -- '--dev' || fail "dev-mode: mine stage missing --dev"
printf '%s' "$out" | grep -q '/smine-consolidate proposals language de' || fail "dev-mode: consolidate missing language de"

# ---- case: casual audience forces dev-mode off ----
cat > "$fake_home/.claude/context/global/presentation-profile.md" <<'EOF'
---
language: de
audience: casual
dev-mode: true
---
EOF
out="$(run_wrapper)"
printf '%s' "$out" | grep -q -- '--dev' && fail "casual: --dev must be forced off"

# ---- case: env override wins without profile ----
rm "$fake_home/.claude/context/global/presentation-profile.md"
out="$(SMINE_DEV_MODE=1 run_wrapper)"
printf '%s' "$out" | grep -q -- '--dev' || fail "env: SMINE_DEV_MODE=1 must thread --dev"

# ---- case: installed manifest → tools echoed on the stage line ----
mkdir -p "$fake_home/.claude/skills/smine"
cat > "$fake_home/.claude/skills/smine/SKILL.md" <<'EOF'
---
name: smine
allowed-tools: Read, Bash(jq *)
---
EOF
out="$(run_wrapper)"
printf '%s' "$out" | grep -q 'tools=Read, Bash(jq \*)' || fail "manifest: mine stage must echo the allowed-tools manifest"
rm -rf "$fake_home/.claude/skills"

# ---- case: dirty tree → exit 64 ----
echo dirty > "$sandbox/untracked.txt"
status=0
run_wrapper >/dev/null 2>&1 || status=$?
[ "$status" = 64 ] || fail "dirty tree: expected exit 64, got $status"
rm "$sandbox/untracked.txt"

# ---- case: non-main branch → exit 64 ----
git -C "$sandbox" checkout -qb feature
status=0
run_wrapper >/dev/null 2>&1 || status=$?
[ "$status" = 64 ] || fail "non-main: expected exit 64, got $status"

echo "PASS: test_bootstrap_wrapper.sh"
