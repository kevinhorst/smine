# skillenv

The Skill environment component's hook: it puts each repo's skill values into the session and stops a skill whose required repo value is unset before it runs.

## Events

- **SessionStart, CwdChanged** — resolves the session's main checkout and appends one `export SKILL_…='…'` line per value to `CLAUDE_ENV_FILE`; the Bash tool sources it before every command, the skills' awk placeholders included.
- **UserPromptExpansion** (a typed `/<skill>`) and **PreToolUse** matching `Skill` (the agent's call) — reads the invoked skill's deployed `env.json`, the repo values its body reads:
  - a required one unset blocks the typed command (`decision: block`, the reason shown to the user) or denies the call (`permissionDecision: deny`, the reason shown to the agent); the body never expands, so its guard never runs;
  - otherwise it adds one context line per value: `skillenv: <NAME> = "<value>" (<source>)` — source `repo <registry name>`, `skill default` or `global default` — or `skillenv: <NAME> not set — optional, its steps are left out`.
- A skill without `env.json` — a plugin skill, a home skill, one reading no repo value — passes silently; so does an unreadable one, logged.

## Resolution

Per variable, the strongest set value wins: the repo's own value (registry row `env`, set on the skill page per repo) over the skill default (`skills.variables` of the install record) over the global default of its kind (`skills.defaults`, the Environment tab's card, keyed by interface id such as `test`). A global default reaches a variable through a deployed `env.json` that binds it to that id. The gate fills `allowed-tools` rules with the same resolution.

## Refusal

One line per unset required variable — name, the repo it is unset for, what it does, an example — then the skill page link for the repo (or an export line per variable when no config server is installed), the hint to start a new session after setting it, and `#disable inject:env` for a value exported in the shell, which the hook cannot see. A value saved mid-session reaches the next session; the running session's shell keeps the values it started with.

## Flags

- `-claude-home <dir>` — the Claude home the record, the deployed skills and the session switches are read from; default `~/.claude`.
- `-env-file <path>` — the export target; default `$CLAUDE_ENV_FILE`.
- `-repos <path>` — the repo registry; default `~/.claude/smine/repos.json`.

`#disable inject:env` switches both modes off for the session, and a smoke session (`SMINE_SKILLENV_OFF` set by the smoke runner) runs neither. The hook exits 0 whatever it finds; a read failure is logged and each one closes a refusal as a `Note:` line.
