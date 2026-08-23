#!/usr/bin/env bash
# Pins the acdsl-project.sh binary resolution: prebuilt bin/acdsl (or .exe,
# the Windows installer payload) wins over the go-run fallback, in that order.
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TMP_BASE=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TMP=$(mktemp -d "$TMP_BASE/acdsl-project-hook.XXXXXX")

cleanup_tmp() {
  if [ -d "$TMP" ] && [ ! -L "$TMP" ] && [ "$(dirname "$TMP")" = "$TMP_BASE" ] && \
      [[ "$(basename "$TMP")" == acdsl-project-hook.* ]]; then
    rm -rf -- "$TMP"
  fi
}
trap cleanup_tmp EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

HOOK="$REPO_DIR/cmd/hooks/acdsl-project.sh"

# Fixture repo: registry present so the hook proceeds; stubbed jq and go on
# PATH so the run is hermetic. Every candidate binary records its invocation.
CWD="$TMP/repo"
LOG="$TMP/invocations"
mkdir -p "$CWD/acdsl" "$CWD/bin" "$TMP/stubs"
echo '{}' > "$CWD/acdsl/registry.json"
echo body > "$CWD/file.go"

cat > "$TMP/stubs/jq" <<EOF
#!/bin/sh
# hook asks for .tool_input.file_path then .cwd
case "\$*" in
  *file_path*) echo "$CWD/file.go" ;;
  *cwd*) echo "$CWD" ;;
esac
EOF
cat > "$TMP/stubs/go" <<EOF
#!/bin/sh
echo "go \$*" >> "$LOG"
EOF
chmod +x "$TMP/stubs/jq" "$TMP/stubs/go"

record_stub() {
  cat > "$1" <<EOF
#!/bin/sh
echo "$2 \$*" >> "$LOG"
EOF
  chmod +x "$1"
}

run_hook() {
  : > "$LOG"
  PATH="$TMP/stubs:/usr/bin:/bin" ACDSL_PROJECT_ENABLED=1 bash "$HOOK" <<'EOF'
{"tool_input": {"file_path": "ignored-by-stub"}, "cwd": "ignored-by-stub"}
EOF
}

# 1. bin/acdsl present -> used, no go run
record_stub "$CWD/bin/acdsl" "bin-acdsl"
run_hook
grep -q "^bin-acdsl project" "$LOG" || fail "bin/acdsl not used"
! grep -q "^go " "$LOG" || fail "go run must not fire with bin/acdsl present"
rm "$CWD/bin/acdsl"

# 2. bin/acdsl.exe present -> used
record_stub "$CWD/bin/acdsl.exe" "bin-acdsl-exe"
run_hook
grep -q "^bin-acdsl-exe project" "$LOG" || fail "bin/acdsl.exe not used"
rm "$CWD/bin/acdsl.exe"

# 3. neither -> go run fallback
run_hook
grep -q "^go run ./cmd/acdsl project" "$LOG" || fail "go run fallback missing"

echo "PASS: test_acdsl_project_hook.sh"
