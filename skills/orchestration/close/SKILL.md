---
name: close
description: Remove the done session's pool worktree and claude/* branch, safety-gated, killing the session. Trigger on /close or "this session is done, clean up the worktree".
author: Kevin Horst
version: 1.7
---

# Close

Explicit end-of-session cleanup: remove this session's `.claude/worktrees/<name>` pool worktree and delete its `claude/*` branch, safety-gated. Success kills the session — that is the point.

## When to use

**Use when:** the user declares the current session done and wants its worktree and branch removed — /close.
**Don't use when:** bulk cleanup of many agent worktrees, or any forced removal — run `~/.claude/agents/tools/remove_agent_worktrees.sh` manually. Routine (launchd) worktrees — `routines/_lib/worktree.sh` owns their lifecycle. Merging or cherry-picking the session's work first — do that before /close; this skill never moves commits.
**Preconditions:** session runs inside a `.claude/worktrees/<name>` pool worktree on a `claude/*` branch; the work already lives on a non-claude branch (the script's gate enforces this).
**Workflow position:** standalone, terminal — nothing hands off from it (see README.md § Skill map, smine repo).

## Procedure

/close takes no arguments. There is deliberately no force variant, and the skill never passes `--force` under any circumstances — forced removal is a manual `remove_agent_worktrees.sh --force` invocation outside this skill.

1. **Locate.** Resolve the session worktree and branch: `git rev-parse --show-toplevel` and `git rev-parse --abbrev-ref HEAD`. Refuse with a clear message if the toplevel is not under `.claude/worktrees/` or the branch is not `claude/*`. Resolve the main checkout as the first entry of `git worktree list --porcelain`.
2. **Pre-flight report.** State the worktree path, the branch, the main checkout, and whether uncommitted changes exist (`git status --porcelain`). No confirmation question — invoking /close is the confirmation.
3. **Remove.** One command, the deployed script, no `cd`, no compound — the script normalizes its own cwd to the main checkout, so it is safe to invoke from inside the dying worktree:

   ```
   ~/.claude/agents/tools/remove_agent_worktrees.sh --delete-branch claude/<branch>
   ```

4. **Report.** Relay the script output verbatim. On skip (modified/untracked files, or commits on no non-claude branch): report the reasons and the fix path — commit, cherry-pick/merge to a non-claude branch, re-run /close — and stop. On removal: state that worktree and branch are gone and the session's shell cwd no longer exists — the session is dead, as intended.

The toolset copy at `~/.claude/agents/tools/` is deployed from `cmd/worktrees/remove_agent_worktrees.sh` by `sync_skills.sh` and is the one agents and manual runs use; the repo copy is the configserver's script. The script owns all safety logic — do not duplicate it here: it lifts the SessionStart hook's `git worktree lock`, excludes infra entries (`.idea/`, `.claude/`, `.claude-worktree` sentinel, `.serena/`, `.DS_Store`) from the untracked gate, and only removes when every commit is reachable from a non-`claude/*` branch (ancestry or `git cherry` patch-id).

## Model

- Suggested: small / low
- Reason: single-script wrapper; all safety logic lives in remove_agent_worktrees.sh
- Tested unviable: — (none yet)
