# worktreecreate

The WorktreeCreate hook: creates the session worktree itself as `<main>/.claude/worktrees/<name>` on branch `claude/<name>`, based on the repository's default branch, and prints its path. With `-base` it is the SessionStart hook of the same worktrees and moves a fresh session branch onto the base the user picked. Both are wired in the Claude Code settings template.

## Usage

```
Usage of worktreecreate:
  -base
    	SessionStart: move a fresh session branch onto the base the Desktop recorded for its worktree
  -claude-home string
    	Claude Code home directory (default ~/.claude)
  -store string
    	Desktop worktree store (default <user config dir>/Claude/git-worktrees.json)
```

Branch prefix, worktrees dir, name pattern, reset retries, the acdsl build deadline, the short-sha length and the default-branch fallback come from the `worktreecreate` section of `<claude home>/hooks/config.json` (the `worktrees.*` settings, [docs/settings.html](../../docs/settings.html)).

## How it works

**WorktreeCreate**
- A configured WorktreeCreate hook makes the Desktop app skip its worktree pool for new sessions, so directory and branch always share the name.
- The name must be non-empty, carry no path separator and not start with a dot or dash; the cwd must be absolute.
- Default branch by the Desktop's rule: the branch `origin/HEAD` names, else `main`, else `master`, as a local branch or an origin tracking branch.
- A transient `index.lock` (the Desktop polls git status in the new worktree) is retried a few times.

**SessionStart (`-base`)**
- The Desktop passes no base to the hook, but records the picked branch as `sourceBranch` in its worktree store before it spawns the CLI; session start recreates the still-untouched branch from that base by name.
- Acts only on a root startup event (`source` startup, no `agent_id`) in a `.claude/worktrees/<name>` dir on branch `claude/<name>` whose store entry is this worktree and hook-based.
- Base: `sourceBranch`, else the default branch; the start point handed to git (and named by the reflog) is the local branch, else `origin/<branch>`.
- The branch is moved only while it is still at its creation commit, its tracked tree is clean, and its reflog creation line names another start point. A dirty tracked tree is reported, not resolved.
- Moving recreates the branch: detach HEAD, delete the branch, create it again from the start point, so its reflog's one line names the base. When HEAD does not end at the base, the branch is put back at its creation commit and the failure returned.
- The move or the refusal to move is one `worktree-base:` line on stdout, which SessionStart injects into the session's context, so the agent reads the reflog's creation line as the picker's choice.

**Exit contract**

| Mode | Exit 0 | Exit 1 |
|---|---|---|
| WorktreeCreate | worktree path on stdout | reason on stderr; any non-zero exit aborts the creation |
| `-base` (SessionStart) | context line on stdout when the branch was moved or deliberately left; reason on stderr when the base could not be applied — the session starts in both cases | only on undecodable input or an unresolvable config dir |

A bad flag exits 1 in both modes.
