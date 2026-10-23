#!/usr/bin/env bash
# install.sh — macOS and Linux launcher: the installer is Go (cmd/installer);
# this script finds go and forwards every argument (master D9).
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: go not found — install Go (macOS: brew install go), then re-run" >&2
  exit 1
fi

cd "$REPO_DIR"
exec go run ./cmd/installer "$@"
