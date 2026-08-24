#!/usr/bin/env bash
# Makes the install dir a standalone git repository: fresh dirs get
# `git init` + an initial commit; an existing .git is never touched and no
# remote is ever configured (pushing is the user's own remote setup).
set -euo pipefail

REPO_DIR="${1:-$(pwd)}"

if [ -d "$REPO_DIR/.git" ]; then
  echo "skip: git repository exists in $REPO_DIR"
  exit 0
fi

echo "-> Initializing standalone git repository in $REPO_DIR ..."
# -b needs git >= 2.28; older gits get init + symbolic-ref.
if ! git -C "$REPO_DIR" init -b main 2>/dev/null; then
  git -C "$REPO_DIR" init
  git -C "$REPO_DIR" symbolic-ref HEAD refs/heads/main
fi

id_flags=()
if ! git -C "$REPO_DIR" config user.email >/dev/null; then
  id_flags=(-c user.name=smine -c user.email=smine@localhost)
fi

git -C "$REPO_DIR" add -A
git -C "$REPO_DIR" ${id_flags[@]+"${id_flags[@]}"} commit -m "smine initial install"
echo "   initialized on branch main (no remote — add your own to push)"
