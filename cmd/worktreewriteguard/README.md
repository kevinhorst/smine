# worktreewriteguard

The PreToolUse(Write|Edit|NotebookEdit) guard that keeps a worktree-anchored session's writes inside its own worktree. Wired in the Claude Code settings template.

## Usage

```
Usage of worktreewriteguard:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
```

It reads the hook envelope on stdin; the pool segment and the deny texts come from the `worktreewriteguard` section of `<claude home>/hooks/config.json` (the `worktrees.pool_segment` and `worktrees.guard_deny_texts` settings).

## How it works

**Rules**
- A session whose cwd lies in a `.claude/worktrees/<name>` worktree writes only inside that worktree: a write into the main checkout or a sibling worktree is denied, a write outside the repository (tmp, `~/.claude`) passes.
- A path holding `../` segments is denied in such a session; the agent writes with a normalized absolute path.
- Under `ROUTINE_PROTECTED_PATHS` no session writes into a listed checkout, whatever its cwd; routine cells write only inside their own worktree.
- The written path is `file_path` (Write, Edit) or `notebook_path` (NotebookEdit); a relative path is joined to the cwd.
- Paths are compared as text, the way the harness sends them. Bash is not matched: command strings are unparseable heuristics.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | deny as JSON on stdout, or a silent allow |
| 1 | failure (undecodable envelope, encode error) |
| 2 | usage error |
