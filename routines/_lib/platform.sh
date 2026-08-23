#!/usr/bin/env bash
# Platform indirections for routine wrappers. Sourced, never executed.
# darwin: flock + coreutils timeout + caffeinate, exactly as before.
# msys (Git Bash under routinewrap.exe): locks are already held by the
# launcher (ROUTINE_LOCKS_HELD=1); per-stage ceilings use a bash watchdog.

routine_platform() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    MINGW*|MSYS*) echo msys ;;
    *) echo other ;;
  esac
}

# routine_self_lock — fail-fast overlap guard on fd 9 at $routine_dir/.lock.
# Callers set routine_dir before sourcing (existing wrapper contract).
routine_self_lock() {
  [ "${ROUTINE_LOCKS_HELD:-0}" = "1" ] && return 0
  exec 9>"$routine_dir/.lock"
  flock -n 9
}

# routine_run_claude <timeout_s> <cmd...> — per-stage ceiling that kills only
# the claude child; the wrapper survives to log, publish and drain (plan D4).
# msys exit status is 143 (TERM) instead of coreutils timeout's 124.
routine_run_claude() {
  local timeout_s="$1"; shift
  if [ "$(routine_platform)" = "darwin" ]; then
    timeout "$timeout_s" caffeinate -is "$@"
    return
  fi
  "$@" &
  local cmd_pid=$!
  # exec >/dev/null: the watchdog (and its sleep child, which survives the
  # kill below as an orphan) must not inherit the caller's stdout — under
  # command substitution an inherited write end keeps $() reading until the
  # orphaned sleep dies, blocking the caller for the full ceiling (CI hang).
  ( exec >/dev/null 2>&1; sleep "$timeout_s"; kill "$cmd_pid" 2>/dev/null ) &
  local watchdog_pid=$!
  local status=0
  wait "$cmd_pid" || status=$?
  kill "$watchdog_pid" 2>/dev/null
  wait "$watchdog_pid" 2>/dev/null
  return "$status"
}
