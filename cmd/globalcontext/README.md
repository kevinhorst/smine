# globalcontext

The SessionStart and SubagentStart hook that injects the always-read global content into every session and subagent. Wired in the Claude Code settings template.

## Usage

```
Usage of globalcontext:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
```

## How it works

**Sources**
- Every `*.md` under `<claude home>/context/global`, then under `<cwd>/<context dir>/global`; dir order, then name order. A missing dir contributes nothing.
- Concatenated in the shape the agent reads: the banner once, then per file a path header and its content.

**Delivery**
- Content within the inline cap (`globalcontext.inline_cap`, default 9000 bytes) is added as `additionalContext` directly.
- Larger content is written to the session's context-required dir as `global.md`, and only a pointer is added; the read gate then denies every other tool call until the agent has read that file in full.
- Content that fits, or is empty, removes a `global.md` an earlier, larger content left behind, so the gate never owes stale global context.
- The pointer ends in one `Read <path> offset N limit M` line per `globalcontext.read_budget` chunk (default 55000 bytes, `internal/contextrequired.ReadInstructions`), the same lines the skill-context pointer and the read gate's deny print.
- The envelope must carry `session_id` (the injection file path hangs on it) and `cwd` (the repo-local dir).

**Configuration**
- Reads `~/.claude/hooks/config.json` (`internal/hookconfig`): `context.context_dir` names the repo-local context dir, the `globalcontext` section carries `inline_cap`, `read_budget`, `banner` and `pointer_format` — the Context system's `context_dir` and `global_*` settings in `docs/settings.html`. A missing or malformed file is a hook error naming the path.
- An empty `context.context_dir` drops only the repo-local dir: the Claude home's `context/global` still injects, followed by the one line `smine: no context dir configured — set context.context_dir …` (the line alone when the home dir holds nothing).
- Disable: park its SessionStart and SubagentStart hook groups on the Hooks tab, or turn off the Context page toggle, which parks the same groups; the session's `#disable` switch for global injection stays per session.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | context JSON on stdout, or nothing to inject |
| 1 | failure — a non-blocking hook error |
| 2 | usage error |
