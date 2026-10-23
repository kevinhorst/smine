# readgate

The read gate. On PreToolUse it denies an agent's tool call until the agent has read, in full, every file it owes; on PostToolUse, UserPromptSubmit and SessionStart it keeps the ledger of what was read. Wired on all four events in the Claude Code settings template.

## Usage

```
Usage of readgate:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
```

## How it works

**Gated tools**
- Only tools in the Bash, Read and Write classes of the `toolpolicy` section of `~/.claude/hooks/config.json` (`tool_classes`, `internal/hookconfig`) are decided; every other tool — ToolSearch, Skill, Agent, MCP tools — passes without a ledger read.
- A missing or malformed hooks config is a hook error naming the path (exit 1, the call proceeds); the install renders the file.
- An empty `context.context_dir` allows every call; SessionStart then prints `smine: no context dir configured — set context.context_dir …` to stderr once.

**What a call owes**
- The language guides governing a path the call touches — the guides of the context index, each declaring its file globs; every guide whose glob matches the absolute or tree-relative path, if its file exists (a `*_test.go` file owes `go.md` and `go_test.md`).
- The guides a denied touch left pending: from the first guide denial on, every later call of that agent owes them too, so the denial cannot be sidestepped with calls that name no governed path.
- The agent's required reading under `context-required/<session>/`: `global.md`, the injection files the skill-context hook wrote for the skills this agent itself invoked (root and subagent alike, never a file written for another agent), and for the root alone the plan-mode injection `planmode.md` of a plan-mode session.
- A review lane's manifest files, before the lane's findings artifact lands.

**Paths a call touches**
- The file of a Read, Edit or Write; the path-like words of a Bash command.
- Bash commands are split conservatively: shell metacharacters separate, surrounding quotes are stripped, flags and dotless words are dropped. Quoted paths with spaces, `$VAR` expansions and heredocs stay unseen and fail open.
- The context dir governing a path is that of its nearest ancestor carrying a context index (a pinned lane worktree brings its own), else the cwd's; the context dir name is `context.context_dir` of the hooks config.

**Always allowed**
- A Read of any owed file, whatever else is still unread, so following a denial's Read instructions in any order never deadlocks.
- A Read of a language guide of the target's own tree.

**Deny reason**
- Lists each owed file's unread ranges as Read instructions with offset and limit, chunked to at most `globalcontext.read_budget` bytes each (default 55000) so no instruction exceeds the harness Read cap.

**Ledger**
- Per agent (`internal/readledger`), filled from what the harness returned: PostToolUse on Read records the line range actually returned (the token cap can make it fewer than asked); PostToolUse on Skill and a typed `/<skill>` prompt record the invocation.
- SessionStart after a compaction or clear drops the root's read records, because what was read left the context; its invocations stay owed.

**Skill injection files**
- Per Skill record, the gate looks for the file the skill-context hook wrote for that call: the sliced file when it exists, else the unsliced one.
- The slice suffix is named the same way as in `cmd/skillcontext` (change both together): no `lane:`/`topic:` token is the smartskill default `readgate.default_slice_suffix` (default `topic-default`); else the first sorted token's kind, a dash, and the sorted names joined by `+`.

**Review lane manifest**
- A Write or Edit of a findings artifact `<artifact>` with a sibling `<artifact>.lane-manifest.json` owes the manifest's files, resolved against its root, else the cwd. A missing or unreadable manifest owes nothing (fail open).
- A manifest pinning a head the lane worktree is not at denies the write: the findings would review the wrong code. An unresolvable head fails open.
- A write that marks the artifact `walk_complete` is denied while checklist pairs stay unanswered: every file × group on file-first, every group alone on group-first. Answers are the walk entries on disk plus a Write's content; an Edit marks by its new string carrying `"walk_complete": true`.
- An allowed artifact write leaves `<artifact>.coverage.json`: the manifest files present under its root, as the manifest names them (`[]`, never null).

**Configuration**
- The `readgate` section of `~/.claude/hooks/config.json` carries the ledger retention, the default slice suffix and the deny texts; the rows are the Context system's `readgate_*` settings in `docs/settings.html`.
- `gate_subagents` off (`readgate_gate_subagents`) gates the root agent alone: a subagent's call is allowed without a ledger read, its ledger still filled. `disabled_guides` (`readgate_disabled_guides`) names guide files by their file name (`go_test.md`) the gate never owes, touched or pending; the other guides and the injection files stay owed.
- Disable: park its hook groups on the Hooks tab; a session's `#disable` switches stay per session.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | deny as JSON on stdout, or a silent allow |
| 1 | failure — a non-blocking hook error, so the call proceeds |
| 2 | usage error |

## Files

- `main.go` — envelope, event dispatch, hooks config load, ledger recording and the deny output
- `gate.go` — the PreToolUse decision: owed files, guides, injection files, Bash path words
- `manifest.go` — review lane manifests: owed files, head pin, walk completeness, coverage record
