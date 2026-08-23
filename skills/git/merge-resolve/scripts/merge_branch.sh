#!/usr/bin/env bash
# merge_branch.sh — deterministic work-branch naming and safety-gated cleanup
# for /merge-resolve. create: derive merge/<theirs-slug>-<ours-slug> from <ours>
# and check it out. cleanup: delete merge/* branches already merged into <ours>
# (-d only, never -D); unmerged branches are kept and reported.
set -euo pipefail

slug() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//'; }

cmd="${1:-}"
case "$cmd" in
  create)
    ours="${2:?usage: merge_branch.sh create <ours> <theirs>}"
    theirs="${3:?usage: merge_branch.sh create <ours> <theirs>}"
    branch="merge/$(slug "$theirs")-$(slug "$ours")"
    git checkout -b "$branch" "$ours"
    echo "$branch"
    ;;
  cleanup)
    ours="${2:?usage: merge_branch.sh cleanup <ours>}"
    git checkout "$ours"
    git for-each-ref --format='%(refname:short)' 'refs/heads/merge/*' | while IFS= read -r b; do
      if git merge-base --is-ancestor "$b" "$ours"; then
        git branch -d "$b"
        echo "deleted: $b"
      else
        echo "kept (not merged into $ours): $b"
      fi
    done
    ;;
  *)
    echo "usage: merge_branch.sh create <ours> <theirs> | cleanup <ours>" >&2
    exit 2
    ;;
esac
