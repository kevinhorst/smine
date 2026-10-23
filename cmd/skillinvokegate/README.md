# skillinvokegate

The PreToolUse skill gate of review lanes: a session whose prompt carries a `REQUIRE-SKILL:<name>` marker must invoke `Skill(<name>)` before any other tool call, and then reads context entries only through the injection. Wired in the Claude Code settings template; the markers are written by the review dispatchers into lane prompts.

## Usage

```
Usage of skillinvokegate:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
```

## How it works

**Prompt markers**
- The prompt is the user lines before the first assistant line of the transcript.
- `REQUIRE-SKILL:<name>` — the last marker of the first prompt line that carries one; the name may be empty.
- `LANE-MANIFEST:<absolute path>` — per prompt line, the last such marker followed by an absolute path.
- A prompt starting with `/<name>` counts as invoked (headless lanes), as does a Skill tool call for the skill.

**Transcript**
- Inside a subagent the gate reads the subagent's own transcript: `agent-<id>.jsonl` at any depth below the session's subagents dir (Agent tool: directly; Workflow: under `workflows/<run>/`). Otherwise the envelope's transcript.
- A user message's prompt text is its string body, or the text of every block joined by newlines.
- One line that is not JSON fails the read, so the gate fails open rather than judging half a session. A field of another type than expected still counts; only the type, the content and the Skill calls are read.

**Before the invocation**
- Only the Skill call and a Read under the session's `context-required` dir pass. A path with a `.` or `..` segment never counts, and the session id joins the dir as written, so it is never cleaned into another dir.

**After the invocation**
- Direct reads of context entries stay denied — they arrive injected. A direct read is a Read, or a plain `cat <path>` Bash command without shell syntax.
- Entry files are the `.md` files of the chapters `skillinvokegate.entry_chapters` names (default `actions`, `facts`, `rules`), denied in any context dir of that name; every other file only in the cwd's context dir. A context path is resolved below the cwd's context dir first, else below the last `/<context dir>/` segment of the path (a pinned lane worktree outside the cwd).
- An empty `context.context_dir` gates no entry read.
- Language guides the context index lists pass; an unreadable index lists none. The index is `SKILL_CONTEXT_INDEX` in the process env if set, else `<cwd>/<context dir>/context.json`.
- Via Read, the files a lane manifest lists pass: a literal absolute path equal to `<base>/<file>`, base the manifest's root, else the envelope cwd — the read gate's resolution. An unreadable manifest lists nothing; Bash reads name nothing.

**Fail-open**
- An unreadable or undecodable transcript, and an absent marker, allow every call.

**Configuration**
- A gated session reads `~/.claude/hooks/config.json` (`internal/hookconfig`): `context.context_dir` names the context dir, the `skillinvokegate` section carries `entry_chapters`, `shell_meta_chars` and the two `deny_texts` — the Permission gate's `invoke_*` settings in `docs/settings.html`. A missing or malformed file, or a `deny_texts` list not holding two texts, denies the call with the error.
- Disable: park the hook group on the Hooks tab; the session's `#disable` switch for the gate stays per session.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | deny as JSON on stdout, or a silent allow |
| 1 | failure |
| 2 | usage error |
