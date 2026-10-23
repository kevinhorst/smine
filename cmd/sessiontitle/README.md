# sessiontitle

Keeps the Desktop session title in the shape `<RepoId>,<SkillId> - <AI title> - <FleetId>[STATE]`. Wired on SessionStart, PreToolUse, PostToolUse and Stop in the Claude Code settings template; the PostToolUse and Stop entries start the binary only while the session's record exists.

## Usage

```
Usage of sessiontitle:
  -repos string
    	repo registry (default <state dir>/<registry file>)
  -skills string
    	deployed skills directory
```

The event name on stdin selects what the binary does.

## How it works

**When it acts**
- Only for an attended Desktop session: `CLAUDE_CODE_ENTRYPOINT=claude-desktop`, a Desktop `CLAUDE_CODE_HOST_SESSION_ID`, and `CLAUDE_CODE_SESSION_ATTENDED=1`. Events of a subagent (`agent_id` set) are ignored.
- SessionStart writes the session's record; without a record no later event does anything.

**The record** — one file per session, `~/.claude/sessiontitle/sessions/<session id>.json` (the id is checked so it never addresses a path outside its dir), written whole after every event:
- `named` — the first rename happened or was not needed
- `fleet` — the saved fleet token
- `turn` — the state the turn's last `title set` call asked for and whether the turn's first set already renamed at its call; Stop spends it. A `compact` SessionStart keeps it, every other start clears it.

**State changes are explicit** — a skill step runs its `title-set@1` program, `smine-rules title set <TOKEN>` by default (`cmd/rules`), which prints `title: set <TOKEN>` when the token is `WIP`, `READY` or a state a deployed skill declares in its `title.json`. The hook sees the Bash call on PostToolUse and records the token when the command holds any program word before `title set <TOKEN>` (at the start or after a separator, so an overridden program or a path counts) and the output carries the printed line — the printed line is the proof, not the program name. Nothing else moves a skill-declared state or READY: no tool-call pattern, no commit detection, no prompt rule.

**The turn's first set renames at the call** — the instruction goes out on that PostToolUse like the other call-time triggers below, so a state set at the start of a run (fimplement's WIP at intake) shows while the run works. Every later set of the turn is recorded only; the last one wins at the turn end.

**At the turn end** — Stop spends the recorded state, computes the title with it (TESTING, DONE and CLOSED stand; every other state yields) and blocks once with the rename it implies when the Desktop shows another title — the backstop for an ignored call-time instruction, and the delivery of a turn's later sets. The re-Stop after the rename finds nothing recorded and passes; a Stop that still shows the old title passes too — the hook blocks once, never twice.

**At the tool call** — the other triggers that may not wait for the turn end instruct at the call:
- First rename (WIP), once per session: while `named` is false every PostToolUse computes the title; the record stays unnamed while the Desktop's AI title (not its prompt echo) or the first prompt line is missing, so the next tool call retries.
- CLOSED after the `/close` removal check (`smine-agentwt remove --dry-run`, any install path), DONE after `smine-heartbeat done`, the fleet slot after `smine-heartbeat join` (the member token on stdout) or `subscribe` (`<fleet>-O` from `--fleet`).

**Ids**
- RepoId: the registered label of the session's main checkout, else its registered name, else the checkout's directory name.
- SkillId: the first deployed skill the session's human prompts invoked — its `title-code`, else its name. Both are set once and never replaced.
- The Desktop titles a session from the typed prompt before the CLI writes it to the transcript, so a missing transcript or one without a human line means "not yet", never an error.

**Instruction delivery**
- A hook cannot rename a Desktop session, so it instructs the main agent to call the Desktop's `set_session_title` tool; the title is JSON-quoted so the agent copies it byte for byte. On Stop, which takes no additional context, the instruction blocks the stop as its reason.
- PreToolUse pre-approves a `set_session_title` of this session whose title parses with a known state, so plan mode, which ignores the allow rule, never prompts; the read-gate lets the call through before any required reading.
- A title the user typed is managed like any other: its text is the wording, prefix and state are re-applied.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | JSON on stdout when there is something to say, else silent; SessionStart only writes the record |
| 1 | failure on stderr — non-blocking |

## Files

- `main.go` — events, dispatch, title transitions
- `record.go` — the session record
- `prompts.go` — command patterns of the Bash triggers and the skills the human prompts invoked
