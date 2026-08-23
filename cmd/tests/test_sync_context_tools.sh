#!/usr/bin/env bash
# Pins the sync scripts' smine_tool resolution (cmd/sync/smine_tool.sh): prebuilt bin/<name> first,
# bin/<name>.exe second, go run fallback last (Windows installer contract).
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/sync-context-tools.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == sync-context-tools.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# Extract smine_tool from the real script and drive it directly — the
# resolution order is the contract, the surrounding sync flow has its own test.
RULES_REPO="$TMP/rules-repo"
LOG="$TMP/invocations"
mkdir -p "$RULES_REPO/bin" "$RULES_REPO/cmd/rules" "$TMP/stubs"

cat > "$TMP/stubs/go" <<EOF
#!/bin/sh
echo "go \$*" >> "$LOG"
EOF
chmod +x "$TMP/stubs/go"

record_stub() {
  cat > "$1" <<EOF
#!/bin/sh
echo "$2 \$*" >> "$LOG"
EOF
  chmod +x "$1"
}

run_tool() {
  : > "$LOG"
  PATH="$TMP/stubs:/usr/bin:/bin" bash -c "
    set -euo pipefail
    RULES_REPO='$RULES_REPO'
    $(sed -n '/^smine_tool()/,/^}/p' "$REPO_DIR/cmd/sync/smine_tool.sh")
    smine_tool rules filter --target t
  "
}

# 1. bin/rules preferred over go run
record_stub "$RULES_REPO/bin/rules" "bin-rules"
run_tool
grep -q "^bin-rules filter --target t" "$LOG" || fail "bin/rules not used"
! grep -q "^go " "$LOG" || fail "go run must not fire with bin/rules present"
rm "$RULES_REPO/bin/rules"

# 2. bin/rules.exe fallback order (second candidate)
record_stub "$RULES_REPO/bin/rules.exe" "bin-rules-exe"
run_tool
grep -q "^bin-rules-exe filter --target t" "$LOG" || fail "bin/rules.exe not used"
rm "$RULES_REPO/bin/rules.exe"

# 3. neither -> go run fallback
run_tool
grep -q "^go run ./cmd/rules filter --target t" "$LOG" || fail "go run fallback missing"

echo "PASS: test_sync_context_tools.sh"
