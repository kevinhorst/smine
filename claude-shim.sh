#!/usr/bin/env bash
# smine claude shim: resolve the newest Claude Desktop bundled claude-code CLI
# and exec it. Deployed to %LOCALAPPDATA%\smine\bin\claude by install.ps1 and
# put on PATH. Resolves the versioned dir at call time, so Claude Desktop
# updates need no re-install.
set -euo pipefail
pkgroot="$(cygpath "$LOCALAPPDATA")/Packages"
exe="$(ls -d "$pkgroot"/Claude_*/LocalCache/Roaming/Claude/claude-code/*/claude.exe 2>/dev/null | sort -V | tail -1)"
if [ -z "${exe:-}" ]; then
  echo "smine claude shim: Claude Desktop claude.exe not found under $pkgroot/Claude_*" >&2
  exit 127
fi
exec "$exe" "$@"
