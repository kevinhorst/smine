#!/usr/bin/env bash
# routines/_lib/platform.sh: forced-msys watchdog kills a hung child and
# passes fast statuses through; ROUTINE_LOCKS_HELD short-circuits the locks.
set -uo pipefail

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

fail() { echo "FAIL: $*" >&2; exit 1; }

# Force the msys branch: platform.sh dispatches on uname -s.
uname() { echo MINGW64_NT; }

routine_dir="$(mktemp -d "${TMPDIR:-/tmp}/platformlib.XXXXXX")"
trap 'rm -rf "$routine_dir"' EXIT

source "$REPO_DIR/routines/_lib/platform.sh"

[ "$(routine_platform)" = "msys" ] || fail "uname override not honored"

# 1. The watchdog kills a hung child near the ceiling with TERM (143).
start="$(date +%s)"
status=0
routine_run_claude 1 sleep 30 || status=$?
elapsed=$(( $(date +%s) - start ))
[ "$status" -eq 143 ] || fail "expected status 143, got $status"
[ "$elapsed" -le 5 ] || fail "watchdog too slow: ${elapsed}s"

# 2. A fast child's status passes through untouched.
status=0
routine_run_claude 10 true || status=$?
[ "$status" -eq 0 ] || fail "true should pass 0, got $status"
status=0
routine_run_claude 10 false || status=$?
[ "$status" -eq 1 ] || fail "false should pass 1, got $status"

# 2b. Command substitution returns as soon as the child exits — the watchdog
# must not hold the $() pipe open until its sleep dies (the Linux CI hang:
# matrix.sh captures routine_run_claude output via $()).
start=$(date +%s)
captured=$(routine_run_claude 60 echo hi)
elapsed=$(( $(date +%s) - start ))
[ "$captured" = "hi" ] || fail "captured output wrong: $captured"
[ "$elapsed" -le 5 ] || fail "command substitution blocked on the watchdog: ${elapsed}s"

# 3. ROUTINE_LOCKS_HELD=1 short-circuits the self lock (no flock involved).
ROUTINE_LOCKS_HELD=1 routine_self_lock || fail "locks-held self lock should no-op"

echo "PASS: test_platform_lib.sh"
