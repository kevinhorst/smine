#!/usr/bin/env bash

routine_cadence_gate() {
  local days="${ROUTINE_CADENCE_DAYS:-1}"
  case "$days" in
    ''|*[!0-9]*) echo "invalid ROUTINE_CADENCE_DAYS: $days" >&2; days=1 ;;
  esac
  [[ "$days" -le 1 ]] && return 0

  local stamp="$routine_dir/.cadence-stamp"
  local now last age
  now="$(date +%s)"
  if [[ -f "$stamp" ]]; then
    last="$(cat "$stamp" 2>/dev/null || echo 0)"
    age=$(( now - last ))
    if (( age < days * 86400 )); then
      echo "cadence gate: last run $(( age / 86400 ))d ago, next after ${days}d — skipping"
      return 1
    fi
  fi
  printf '%s' "$now" > "$stamp"
  return 0
}
