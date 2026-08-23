#!/usr/bin/env bash
set -euo pipefail

items="a b c"
for item in $items; do
  printf '%s\n' "$item"
done
