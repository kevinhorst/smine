#!/usr/bin/env bash
# Sourced by the sync scripts: run a repo tool prebuilt-first (bin/<name>, or
# bin/<name>.exe as shipped by the Windows installer) so target machines need
# no Go toolchain; dev machines fall back to `go run ./cmd/<name>`.
# RULES_REPO must be set by the caller before sourcing.
smine_tool() {
  local name="$1"; shift
  for candidate in "$RULES_REPO/bin/$name" "$RULES_REPO/bin/$name.exe"; do
    if [ -x "$candidate" ]; then "$candidate" "$@"; return; fi
  done
  (cd "$RULES_REPO" && go run "./cmd/$name" "$@")
}
