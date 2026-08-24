#!/usr/bin/env bash
# cmd/sync/ensure_git_repo.sh: fresh dir -> standalone repo on main with one
# commit and no remote; existing .git untouched; commits without an identity.
set -uo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

fail() { echo "FAIL: $*" >&2; exit 1; }

work="$(mktemp -d "${TMPDIR:-/tmp}/ensuregit.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# 1. Fresh dir: init on main, everything committed, no remote configured.
mkdir "$work/fresh"
echo hello > "$work/fresh/file.txt"
bash "$REPO_DIR/cmd/sync/ensure_git_repo.sh" "$work/fresh" || fail "fresh install failed"
[ "$(git -C "$work/fresh" symbolic-ref --short HEAD)" = "main" ] || fail "branch is not main"
[ "$(git -C "$work/fresh" rev-list --count HEAD)" = "1" ] || fail "expected exactly one commit"
git -C "$work/fresh" diff --quiet HEAD || fail "tree not fully committed"
[ -z "$(git -C "$work/fresh" remote)" ] || fail "a remote was configured"

# 2. Existing .git: untouched (same HEAD, no new commit).
before="$(git -C "$work/fresh" rev-parse HEAD)"
bash "$REPO_DIR/cmd/sync/ensure_git_repo.sh" "$work/fresh" || fail "rerun failed"
[ "$(git -C "$work/fresh" rev-parse HEAD)" = "$before" ] || fail "existing repo was touched"

# 3. No git identity anywhere: the fallback identity commits.
mkdir "$work/noident" "$work/home"
echo x > "$work/noident/f"
HOME="$work/home" GIT_CONFIG_GLOBAL="$work/home/gitconfig" GIT_CONFIG_SYSTEM=/dev/null \
  bash "$REPO_DIR/cmd/sync/ensure_git_repo.sh" "$work/noident" || fail "identity fallback failed"
[ "$(git -C "$work/noident" rev-list --count HEAD)" = "1" ] || fail "no commit without identity"

echo PASS
