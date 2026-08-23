#!/usr/bin/env bash
set -euo pipefail

declare -A lookup
lookup[key]=value
printf '%s\n' "${lookup[key]}"
