# planmodecontext

The UserPromptSubmit and PostToolUse hook that delivers the plan context pack — the RULE-PLAN entries of the cwd's context index — once per plan-mode stretch of the root agent. Wired in the Claude Code settings template.

## Usage

```
Usage of planmodecontext:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
```

## How it works

**Plan mode**
- Read from the hook input: `permission_mode` `plan` (the user switched it on in the UI); the `EnterPlanMode` PostToolUse branch is dead while the tool-policy gate denies that tool.
- Only the root agent is served while `planmodecontext.root_only` holds (default); an event carrying an `agent_id` is then ignored.

**Delivery**
- The pack goes to the session's context-required dir as `planmode.md`, and a pointer is added as context; the read gate then denies every other tool call of the root agent until it has read the file in full.
- The pack: banner, heading, one line per entry whose id starts with `planmodecontext.entry_id_prefix` (default `RULE-PLAN-`) in index order, and a trailer pointing at `acdsl project -plan <path>`. No matching entry means no pack.
- The context index is `<cwd>/<context dir>/context.json`, the context dir being `context.context_dir` of the hooks config, else `<cwd>/context/context.json`.
- An empty `context.context_dir` delivers no pack; a plan-mode event then adds the one line `smine: no context dir configured — set context.context_dir …` as context.

**State**
- The injection file is the only state: present means delivered. It is created exclusively, so only the first event of a stretch delivers; a failed write removes it so a later event retries.
- The first root event outside plan mode removes it.

**Configuration**
- Reads `~/.claude/hooks/config.json` (`internal/hookconfig`): `context.context_dir`, the `planmodecontext` section (banner, heading, entry and pack formats, pointer format, trailer, entry id prefix, `root_only`) and `globalcontext.read_budget` for the pointer's Read chunks — the Context system's `planmode_*` settings in `docs/settings.html`. A missing or malformed file is a hook error naming the path.
- Disable: park its hook groups on the Hooks tab; the session's `#disable` switch for plan injection stays per session.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | context JSON on stdout, or nothing to add |
| 1 | any failure, usage included — a non-blocking hook error; never 2, which would block the user's prompt on UserPromptSubmit |
