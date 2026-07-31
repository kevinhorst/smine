#!/usr/bin/env bash
# End-to-end coverage for secretscan: exit codes, baseline flow, history scan
# on fixture repos with planted, documented fake credentials.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/secretscan.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == secretscan.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

BIN=$TMP/secretscan
go build -o "$BIN" "$REPO_DIR/cmd/secretscan"

# AWS's documented example access key id — deterministic fake, never live.
FAKE_AWS_KEY="AKIAIOSFODNN7EXAMPLE"

run_scan() {
  rc=0
  out=$("$BIN" "$@" 2>&1) || rc=$?
}

init_repo() {
  local repo=$1
  mkdir -p "$repo"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
}

test_not_a_repo_exits_2() {
  mkdir -p "$TMP/plaindir"
  run_scan "$TMP/plaindir"
  [ "$rc" -eq 2 ] || fail "expected exit 2 on non-repo, got $rc"
  [[ "$out" == *"Not a git repository"* ]] || fail "guard message missing: $out"
}

test_clean_repo_exits_0() {
  local repo=$TMP/clean
  init_repo "$repo"
  printf 'package main\n' > "$repo/main.go"
  run_scan "$repo"
  [ "$rc" -eq 0 ] || fail "expected exit 0 on clean repo, got $rc: $out"
  [[ "$out" == *"0 new"* ]] || fail "summary missing: $out"
}

test_findings_exit_1() {
  local repo=$TMP/dirty
  init_repo "$repo"
  printf 'key = "%s"\n' "$FAKE_AWS_KEY" > "$repo/config.py"
  touch "$repo/.env"
  run_scan "$repo"
  [ "$rc" -eq 1 ] || fail "expected exit 1 on findings, got $rc: $out"
  [[ "$out" == *"aws-access-key-id"* ]] || fail "aws detector missing: $out"
  [[ "$out" == *"deny-file"* ]] || fail "deny-file finding missing: $out"
}

test_json_output_parses() {
  local repo=$TMP/dirty
  run_scan -json "$repo"
  [ "$rc" -eq 1 ] || fail "expected exit 1, got $rc"
  count=$(printf '%s' "$out" | jq '.newFindings | length') || fail "JSON did not parse"
  [ "$count" -ge 2 ] || fail "expected >=2 new findings in JSON, got $count"
}

test_update_baseline_then_clean() {
  local repo=$TMP/dirty
  run_scan -update-baseline "$repo"
  [ "$rc" -eq 0 ] || fail "expected exit 0 after baseline update, got $rc: $out"
  [ -f "$repo/.secretscan-baseline" ] || fail "baseline file not written"
  run_scan "$repo"
  [ "$rc" -eq 0 ] || fail "expected exit 0 on baselined repo, got $rc: $out"
  mv "$repo/config.py" "$repo/renamed.py"
  run_scan "$repo"
  [ "$rc" -eq 1 ] || fail "rename must resurface the finding, got $rc: $out"
}

test_history_finding_detected() {
  local repo=$TMP/history
  init_repo "$repo"
  printf 'key = "%s"\n' "$FAKE_AWS_KEY" > "$repo/secret.py"
  git -C "$repo" add secret.py
  git -C "$repo" commit -qm "add secret"
  git -C "$repo" rm -q secret.py
  git -C "$repo" commit -qm "remove secret"
  run_scan "$repo"
  [ "$rc" -eq 0 ] || fail "tree must be clean after removal, got $rc: $out"
  run_scan -history "$repo"
  [ "$rc" -eq 1 ] || fail "history scan must find the removed secret, got $rc: $out"
  [[ "$out" == *"secret.py"* ]] || fail "history path missing: $out"
}

test_determinism() {
  local repo=$TMP/dirty
  run_scan "$repo"; first=$out
  run_scan "$repo"; second=$out
  [ "$first" = "$second" ] || fail "two scans differ"
}

test_not_a_repo_exits_2
test_clean_repo_exits_0
test_findings_exit_1
test_json_output_parses
test_update_baseline_then_clean
test_history_finding_detected
test_determinism

echo "PASS: secretscan"
