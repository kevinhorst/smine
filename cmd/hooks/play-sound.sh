#!/usr/bin/env bash
# Claude Code sound-hook dispatch: darwin plays the event sound; everything
# else is a silent no-op (Windows decision, plans/windows_support).
[ "$(uname -s)" = "Darwin" ] || exit 0
case "${1:-}" in
  notify) exec afplay /System/Library/Sounds/Funk.aiff ;;
  stop)   exec afplay /System/Library/Sounds/Hero.aiff ;;
esac
exit 0
