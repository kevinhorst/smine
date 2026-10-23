# Session messages

A line of your chat message whose first word is one of the seven `#` commands is read by the tool-policy gate. The rest of the message is yours to write, several command lines in one message apply in order, and one grammar serves all seven words.

- Counts: a typed prompt and a message typed while a turn ran (an interruption, with or without more text, one after another too).
- Never counts: pasted content — a Desktop paste wrapper is dropped with everything inside it, so a transcript you paste can never fire commands; the typed text around the paste still counts. Nor anything the harness writes as a user row — a hook's feedback (the session-title rename instruction above all), the `[Request interrupted by user]` marker, tool results, compaction summaries, `/model` and other local commands, messages from other sessions, task notifications, headless prompts.
- Each typed command answers with its own `tool-policy …` context line.

## Permissions

| Message | Effect | Example |
|---|---|---|
| `#allow Makefile` | unlocks every make source for this session; `#deny Makefile` re-locks | `#allow Makefile` |
| `#allow settings.local.json` | lets the agent self-serve rules through the gate's `allow` command; `#deny settings.local.json` revokes | `#allow -s` |
| `#add <absolute dir>` | makes an existing directory a read and write root (appended to `permissions.additionalDirectories`) | `#add /Users/devuser/GolandProjects/other-repo` |
| `#set <rule>` | appends allow rules in the settings dialect: the rest of the line, then one per following line up to a blank line or the next command | `#set Bash(mdfind *)` |
| `#reset` | clears the `#add` and `#set` lists and every switch; the earlier messages become inert, command lines after it apply afresh | `#reset` |

## Switches

Features are grouped: `rgate` (what the read gate makes you read), `skill` (the gates on skill runs), `inject` (what gets written into your context), `session` (the title), and `hb` (heartbeat). A group word names every member, `<group>:a,b` names members, `all` names everything; the catalog below says what each one keeps on while it is on.

| Message | Effect | Example |
|---|---|---|
| `#disable <word…>` | turns the named features off for this session, from the next hook event on | `#disable rgate:global,go hb` |
| `#enable <word…>` | turns them back on; per member the last word wins | `#enable rgate:go` |
| `#disable <group>` | every member of the group — for `rgate` every file extension the repo's language guides cover too | `#disable rgate` |
| `#disable all` / `#enable all` | every feature of the catalog below | `#disable all` |

- The language guides are switched by file extension or by guide name, `rgate:<ext|guide>`. An extension drops every guide covering such files: `rgate:go` drops the Go guide and the Go test guide alike, `rgate:cue` drops the skills guide for `<leaf>.cue` while `.md` payloads still owe it. A guide name drops that one guide for every file: `rgate:go_test` leaves plain `.go` files owing the Go guide, `rgate:rust`, `rgate:skills`. `rgate:go,rs,cue` and `rgate:go,rust,skills` are both valid lists. The extensions come from each guide's `Files:` patterns; a guide matching by path alone (no extension) is reachable by name only. `heartbeat` spells `hb`.
- A guide a denial already recorded is cleared by its name, or once every extension it covers is off: after a `<leaf>.cue` denial, `rgate:skills` or `rgate:cue,md` clears it, `rgate:cue` alone does not.
- A switch applies from the first hook event after the gate saw it: a typed message counts at the agent's first tool call, a mid-turn message one call later.
- Subagents follow the session: one record per session id.
- A word outside the grammar rejects that line and the answer shows the grammar: `tool-policy #disable: rejected — unknown feature "gate"; features: rgate[:global|invoked|plan|<ext|guide>], skill[:enforce|invokegate], inject[:global|skill|acdsl|env|plan], session[:title], hb, all`.
- The answer always names the full off set, a whole group by its word: `tool-policy #disable: off — rgate, hb (from the next hook event on)`, `tool-policy #enable: on — rgate:go; still off — hb`.
- Off never stops observation: reads are still recorded, the title record kept, heartbeat keeps tracking the session — only the hook's effect pauses.
- `inject:global` writes the global context file at session and subagent start; a session or subagent that already has it keeps owing it until `rgate:global` is off.

## What stays on

The gate itself and its message channel, the worktree write guard, the heart attack (forbidden subagent models), the `--help` hook, the worktree hook and the notification sound. `#disable gate` is rejected.

## Where to see it

The session's context page on the config server (`/sessions-report/<id>`) lists every feature under Session features, with each time it was turned off or on, read from `~/.claude/tool-policy/state/<session>.json`. The same record is what every hook reads.
