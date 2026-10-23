# skillhelp

The UserPromptSubmit help hook: a prompt of the form `/<skill> --help` is answered with the skill's help and blocked, so the user reads it without the skill running. Wired as a UserPromptSubmit hook in `settings/claude_code/components/session-helpers.json`, beside sessiontitle.

## Usage

```
Usage of skillhelp:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
  -skills string
    	deployed skills directory (default <claude home>/skills)
```

The help pattern and the feature-catalog lead come from the `skillhelp` section of `<claude home>/hooks/config.json` (the `session-helpers.help_*` settings).

## How it works

**Matching**
- `/<skill>` followed by `--help`, `-help`, `-h`, `--h`, or the same with an en or em dash.
- Only the first line of the prompt is matched: the Desktop app merges queued messages into one multi-line prompt, and a help call stays a help call whatever follows it.
- Any other prompt passes through silently.
- A `/<leaf>` prompt of a skill whose deployed frontmatter carries `answered-by: hook` is answered on every invocation: `answer.md` (the skill's payload bodies) plus the feature catalog of `internal/sessionstate`: a `## Features` heading, the lead line, then one ``- `<feature>` — <what it does while on> (<hook>)`` entry each. The `--help` form of such a skill still renders the help.

**Help text**
- Read from the deployed skill dir: `SKILL.md` (frontmatter, Routing, Args), `title.json` (title states), `help-test.md` (the Tests lines), `context-map.json` (topics).
- A skill that is not installed there (a plugin or unknown skill) yields nothing, and the prompt proceeds.
- The answer is a `block` decision, `{"decision":"block","reason":<text>}`; Claude Code shows the reason and does not run the prompt. How a block interacts with the other UserPromptSubmit hooks of the same event is not documented in this repository.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | block JSON on stdout, or silent pass-through |
| 1 | failure — a non-blocking hook error, the prompt proceeds; a missing binary behaves the same |
| 2 | usage error |
