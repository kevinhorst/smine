#!/usr/bin/env bash
# Tests for worktree-write-guard.sh: worktree-anchored sessions may write only
# inside their worktree; everything outside the repo stays allowed; non-pool
# sessions are untouched.
set -euo pipefail
cd "$(dirname "$0")/../.."

GUARD=cmd/hooks/worktree-write-guard.sh
WT=/repo/.claude/worktrees/session-a

run_guard() {
	printf '{"cwd":"%s","tool_input":{"file_path":"%s"}}' "$1" "$2" | bash "$GUARD"
}

expect_allow() {
	out=$(run_guard "$1" "$2")
	if [ -n "$out" ]; then
		echo "FAIL: expected allow for $2, got: $out"
		exit 1
	fi
}

expect_deny() {
	out=$(run_guard "$1" "$2")
	if ! grep -q '"permissionDecision": *"deny"' <<<"$out"; then
		echo "FAIL: expected deny for $2, got: $out"
		exit 1
	fi
}

echo "write inside own worktree: allowed"
expect_allow "$WT" "$WT/internal/x.go"

echo "relative path (resolves under cwd): allowed"
expect_allow "$WT" "internal/x.go"

echo "write into main checkout: denied"
expect_deny "$WT" "/repo/internal/x.go"

echo "write into sibling worktree: denied"
expect_deny "$WT" "/repo/.claude/worktrees/session-b/internal/x.go"

echo "path with ../ escape: denied"
expect_deny "$WT" "$WT/../../../internal/x.go"

echo "write outside the repo (tmp): allowed"
expect_allow "$WT" "/tmp/scratch.txt"

echo "write to home config (plans/memory): allowed"
expect_allow "$WT" "$HOME/.claude/plans/plan.md"

echo "non-pool session: guard does not apply"
expect_allow "/somewhere/else" "/repo/internal/x.go"

echo "subdir cwd inside worktree: main-checkout write still denied"
expect_deny "$WT/internal/deep" "/repo/internal/x.go"

echo "PASS: worktree-write-guard.sh"
