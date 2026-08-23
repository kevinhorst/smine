#!/usr/bin/env bash
set -euo pipefail

target="${1:-}"
if [ -n "$target" ]; then
  printf 'target: %s\n' "$target"
fi
