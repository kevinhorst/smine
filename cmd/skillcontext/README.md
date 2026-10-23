# skillcontext

The skill-context hook. A typed `/<skill>` prompt (UserPromptSubmit) or a Skill call (PostToolUse) resolves the skill's `acdsl-context:` declaration against the repo's context index and injects the selected entries. Wired on both events in the Claude Code settings template.

## Usage

```
Usage of skillcontext:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
  -skills string
    	deployed skills directory (default <claude home>/skills)
```

## How it works

**Selection**
- The declaration is the first `acdsl-context:` line of the deployed SKILL.md frontmatter; its first `version:` line is stamped into the header, so the transcript records which skill version the session invoked.
- `lane:<name>` and `topic:<name>` tokens in the invocation select slices from the skill's `context-map.json`. Without a token, a `default` slice is the set (a smartskill). An unmapped name or a missing map keeps the full declaration and no slice name.
- Tokens resolve against the context index: `SKILL_CONTEXT_INDEX` in the process env if set, else the index file under `<cwd>/<context dir>` (`context.context_dir` of the hooks config); `SKILL_CONTEXT_MAP` in the process env overrides the context map the same way. Entries are unique by id (first occurrence wins), sorted by id, and reach-filtered when the index is a stamped source index.
- A typed prompt names the skill by the word after the slash — the prompt must begin with `/`, no leading whitespace, unlike `skillhelp` and the gate, which trim — and the whole prompt is the invocation text; a Skill call carries skill and args.

**Delivery**
- Per agent: bodies for the entries this agent has not received yet, ids for the rest; the delivered ids are recorded in the agent's read ledger. Without a session id there is no ledger and everything renders inline.
- Header: banner with the skill's deployed version (`?` when none), one bold id line per entry whose body follows, and the ids injected earlier in this agent's session; the context record harvests every id and the version from it.
- Bodies render as the context docs author them: bold id with enforcement tag and statement, tagged fields (`* Name: value`), bullets, notes, each example as a fenced block.
- Bodies within the inline cap (`skillcontext.inline_cap`, default 9000 runes) are injected directly; larger ones are appended to the agent's injection file with a pointer to the new lines, ending in one `Read <path> offset N limit M` line per `globalcontext.read_budget` chunk, and the read gate owes that file.
- Injection file: `<skill>[--<slice>]--<agent>.md` under `context-required/<session>/`. The slice name is the first sorted token's kind, a dash, and the sorted unique names joined by `+`; the read gate derives the same name (`cmd/readgate/gate.go`), so change both together.
- Output: plain stdout for a prompt, the `additionalContext` envelope for a Skill call.
- Every injection prunes `context-required` session dirs untouched for the ledger retention period, so a stretch without an injection file cannot let them outlive the window.

**Skipped**
- A bare `no-context` token with a `-`, `--`, en- or em-dash prefix, standing alone between blanks (punctuation-adjacent mentions do not count) or a help call, which the skillhelp hook answers; any line of the invocation counts for the help match.
- Nothing declared, deployed or selected.

**Configuration**
- Reads `~/.claude/hooks/config.json` (`internal/hookconfig`): `context.context_dir` and the `skillcontext` section (inline cap, default slice, help and no-context patterns, banner, repeat and pointer formats) — the Context system's `skill_*` settings in `docs/settings.html`. A missing or malformed file, or an invalid pattern, is a hook error naming it.
- An empty `context.context_dir` without `SKILL_CONTEXT_INDEX` injects no entries; an invocation of a declaring skill gets the one line `smine: no context dir configured — set context.context_dir …` instead.
- Disable: park its hook groups on the Hooks tab; the session's `#disable` switch for skill injection stays per session.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | injection on stdout, or a silent no-op |
| 1 | failure — a non-blocking hook error |
| 2 | usage error |

## Files

- `main.go` — envelope, invocation detection, skip rules, output
- `select.go` — declaration, context map slicing, index lookup, entry selection, session pruning
- `deliver.go` — per-agent delta against the ledger, inline cap, injection file
- `render.go` — header and entry body rendering
