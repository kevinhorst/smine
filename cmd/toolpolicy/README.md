# toolpolicy

The closed-world PreToolUse gate: it reads the hook envelope on stdin and decides every tool call — allowlisted calls reach the engine, everything else is denied with a steering reason and a ledger line. On a PermissionRequest it answers the engine's own ask with the same verdict; on UserPromptSubmit and PostToolUse it applies a plan's frozen rules for `/fimplement`; on every event it reconciles the user's `#add`/`#set` messages into the anchor's `settings.local.json`, and on DirectoryAdded the added directory. Wired directly as the hook command in the Claude Code settings template; a missing binary is a non-blocking hook error, so a broken deploy falls open to the engine instead of bricking a session. The policy and the mechanics are documented for users in `docs/permissions.html`.

## Usage

```
usage: toolpolicy [-ledger <file>] [-gate-repo <main checkout>] [-repos <repos.json>] < hook-input | toolpolicy [-gate-repo <main checkout>] [-repos <repos.json>] check [--skill <leaf>]… '<target>' …
```

- The policy is the `toolpolicy` section of `<claude home>/hooks/config.json` (`internal/hookconfig`), read at that fixed path on every call; the install renders it from the `gate.*` settings. A missing or invalid file exits 1 naming it.
- `-ledger` — the denial ledger (JSONL, appended); the frozen-rules ledger and the per-session state files live beside it.
- `-gate-repo` — absolute main checkout of the gate's own source repo: its linked worktrees may not run the policy's operator paths (empty: guard off).
- `-repos` — repo registry backing the `@repos` skill path grant (empty: no grant).
- `check …` — the terminal door: prints the same report as the in-session `toolpolicy check`, with the current directory as the worktree.

Inside a session the agent calls the gate's self-service commands as Bash; the gate answers them itself and refuses the call, so Claude Code neither runs nor prompts for them:

```
usage: toolpolicy allow '<rule>' ['<rule>' …] | toolpolicy check [--skill <leaf>]… '<command|url|tool>' … | toolpolicy freeze plans/<feature folder>/<plan dir> | toolpolicy revert — one literal command, no pipes or redirects
```

## How it works

**Verdict order** for one PreToolUse call
- `ExitPlanMode` is left to the engine's plan approval; every other call gets allow or deny.
- Denies bind before the ungated allow, in this order: a tool the policy denies; a settings deny rule; a Browser pane URL carrying credentials (the pane answers those with its own consent card); a skill's enforce gate rule; the relay guard; an Agent model that cannot run its subagent as named.
- Ungated tools are allowed. Then the session's read fence; then per tool class — the classes come from `tool_classes` of the `toolpolicy` section: Bash, read-family, write-family, rule-bearing tools (EnterWorktree, Skill, Agent, MCP, WebFetch, Workflow, …). A tool name outside the built-in set and the MCP namespace is unknown and denied.

**Deny reason**
- `tool-policy: denied <tool> — <cause>[: details]. candidate rule: <rule>. <steer>`, plus a fixed Bash steering sentence for Bash. The candidate is the narrowest allow rule covering the call: `Bash(<program> <subcommand> *)` from the program and its first subcommand-shaped token (letters, digits, `_`, `-`), else `Bash(<program> *)`.
- Details reach the reason only; the cause stays the fixed text the ledger groups by.
- An enforce or relay deny carries no candidate rule: no allow rule lifts it.

**Denial ledger**
- One JSON line per deny (`ts`, `session_id`, `agent_id`, `agent_type`, `tool`, `input`, `cause`, `candidate_rule`, `cwd`, `anchor`, `scratchpad_dir`), which smine-permissions reads by name. `input` is the one value the candidate rule was derived from, never an MCP argument object.
- Appended with `O_APPEND|O_CREATE`, dir mode 0700, file mode 0600; errors are swallowed after a stderr note — the ledger never changes a verdict.
- Self-service answers (APPLIED, CHECK, FROZEN) are not ledgered.

**Grants**
- The union of the settings layers the engine reads — the project layers at the session anchor, never at a drifted envelope cwd — and the `allowed-tools` of the skills the session invoked. Missing files are empty layers; a layer that does not parse contributes nothing and is reported on stderr.
- Skill activation replays the session's skill events in order. A typed prompt activates its skill (prompt-start slash, or the Desktop command-name tag). A Skill tool call activates it only when the user's newest message before it names the skill as a `/<skill>` word, or a settings allow rule or an active skill's `Skill(...)` rule admits it — so a denied call grants nothing. A later Skill call for an active skill adds no grant, so the gate admits it.
- Every active skill contributes its `allowed-tools` and its enforce gate rules; the rendered runtime placement lines (`docker`, `enable-delegation`, `enforce-permissions`) are declarative and the gate never reads them. Skill path rules resolve to roots: `~/x/**` names `~/x`, `//abs/**` names `/abs`, `@repos/**` names every registered repo but the session's own (a missing or unloadable registry grants nothing); a relative or non-recursive spec grants nothing. Settings-layer path rules never become roots.
- Folders the user added to the session in the Desktop app (read from the Desktop's session record) join the roots; a headless or terminal session has none.

**Transcript scan** — one pass over the session transcript; a missing or unreadable transcript grants nothing
- Skill events, the skills each user message names, the `#` command lines (approvals, and the `#add`/`#set`/`#disable`/`#enable` messages since the last `#reset`), the newest turn record's origin, the anchor moves, the first prompt's text, the argument text of every `/fdesign` prompt the user typed, and the plan file of every `smine-rules plan-sections` call.
- A turn record is a user row the harness stamped with a turn origin, or a message the user typed mid-turn (recorded as human); tool results, hook output, compaction rows and notification attachments are not turn records.
- The user's text is what `internal/transcript.Classify` calls human: a typed prompt row (every text block) or a mid-turn `queued_command` with a human origin and no meta mark; paste wrappers are dropped with their bodies (`transcript.StripPastedContents`), so pasted text carries no command. Hook feedback rows (`isMeta`, "Stop hook feedback:"), the interruption marker, tool results, compaction rows, local commands, peer and headless rows are other kinds and carry no command.
- A command is a line of that text whose first field is one of the seven `#` words (`hashtag.go`); prose around it is ignored and several commands per message apply in order. In a `/<skill>` prompt the text is the skill's args (`/fdesign #disable session:title`, or the `<command-args>` body of the transcript row), so the first args line counts as a line of its own. An approval line is exactly `#allow <target>` or `#deny <target>`; a third word disqualifies the line. Fixed short spellings resolve to `settings.local.json`; any other target is kept verbatim and unlocks only what it names.

**User commands** — the same lines, reconciled into `<anchor>/.claude/settings.local.json` on every event
- `#add <absolute existing directory>` (exactly two words) appends to `permissions.additionalDirectories`, cleaned, once; anything else is silently nothing. `#set <rule>` plus one rule per following line up to a blank line or the next command appends to `permissions.allow`: each rule must parse in the dialect and not be a path pattern; a rejected rule is skipped, the rest applies, and the new rules admit the call they are written on. `#reset` as a one-word line makes the earlier messages inert, so the switches they set come back on at the next event; typed as a prompt it also unsets both lists and clears the switches at once, and the command lines after it apply afresh.
- `#disable <word…>` / `#enable <word…>` take the grouped grammar of `internal/sessionstate` — `rgate:…`, `skill:…`, `inject:…`, `session:title`, `hb`, a bare group word, `<group>:a,b` lists and `all`. The parser (`switchMessage`) checks the word shape; `reconcileSettingsMessages` expands group words and `all` to members and validates every member against the catalog plus the anchor's context-index guides (`rgate:<ext|guide>` from the guides' `files` globs and `path` stems, the index found through the global-context hook's context dir), rejecting the message on an unknown one; the fold (last message per member wins) lands in `switches.off` of the session record `<ledger dir>/state/<session_id>.json`, written only on change. `Switches.Set` writes it and appends one `switches.history` event per feature turned off or on (`{action, at, feature}`, stamped at the write). The record is what the hook binaries and heartbeat read; the gate itself reads `skill:enforce` to skip the skill enforce rules. The report prints a whole group by its word (`sessionstate.Compact`).
- The file is the applied state; a message already reflected writes nothing. The submitted prompt's own outcome is printed to stdout (context), one line per command, approvals included; mid-turn messages, the transcript replay and the `DirectoryAdded` event (its `directory`, same check as `#add`) print nothing. Headless and wrapped prompts apply nothing from the prompt. The self-service state is untouched, so `revert` never removes these entries.

**Session anchor**
- The directory file access is rooted at: the first cwd the transcript records, moved by a successful EnterWorktree and returned by a successful ExitWorktree; the envelope cwd when there is no transcript or recorded cwd.
- A subagent's own isolation worktree overrides it: the first cwd the subagent's own transcript (`agent-<id>.jsonl` in the session's subagents dir or a `workflows/<run>` dir) records, when that lies directly under the `.claude/worktrees` of the parent anchor's repository. The envelope cwd never names it; an agent sitting in another agent's directory inherits nothing.
- An anchor whose directory is gone (a removed worktree) roots nothing, so a drifted envelope cwd never becomes a root.
- EnterWorktree with an explicit path must stay inside the session repository's own `.claude/worktrees`; a name or empty input is rule-matched.

**Roots**
- Read: never a fenced root; then the read roots — the anchor, `additionalDirectories` and the Desktop-added folders, the scratchpad, the session temp dir and the policy's extra read dirs — the system read dirs (outside `$HOME` and the temp dirs; `/etc` deliberately absent) and the invoked skills' read grants. A Glob pattern naming its own base directory is checked by its fixed prefix.
- Write: never a gate- or engine-protected path or a fenced root; then the write roots, the device files every shell writes, and the skills' write grants. Extra write dirs may carry `*` as one whole path segment (`~/.claude/projects/*/memory`). A `/tmp` target outside them is steered to the scratchpad.
- Paths resolve symlinks on both sides, through the longest existing prefix, so a path about to be created resolves through a symlinked parent (`/tmp` → `/private/tmp`).
- Session temp dir: `/tmp/claude-<uid>/<project dir>/<session id>` — the dir holding `scratchpad/`, `images/` and the task outputs `tasks/<id>.output` a background task or Workflow completion notification names. The root is the glob `/tmp/claude-<uid>/*/<session id>` built from the envelope's `session_id` and the gate process's uid, because the project dir varies (a cell's cwd, a Desktop session's launch dir or project) while the session id never does; a headless cell whose envelope carries no `scratchpad_dir` has it too, and the parent of `scratchpad_dir` is kept as a second statement of it. Read-only; absent only without a session id or where the uid is unavailable (Windows).
- Read fence: `config.ReadFenceEnv` (from the environment, never the hooks config) lists roots a confined session may not touch; a file tool's path inside it is denied, and so is a Bash command naming a fenced root as the fence spells it or home-relative (skills name the plans archive as `~/GolandProjects/agent-plans`).

**Protected paths** — never written by a direct tool call
- The settings layers (a session never authors its own allow rules), the policy's gate-input subtrees, the session transcript, every git dir (any `.git` segment, case-folded), the acdsl surface of a gated or strict repo, every plan dir's rules file the session may no longer freeze, and — until the user sends `#allow Makefile` — every make source (a default makefile name or an included `.mk`).
- Engine-protected names Claude Code never auto-approves writes under: a protected directory segment anywhere or a protected file name last; `.claude` is protected unless it holds worktrees, `.config` only as `.config/git`. The harness's own write dirs are exempt.
- The acdsl guard comes from the nearest `acdsl/policy.json` above the cwd: in strict mode a rule file is anything under `acdsl/`, or any `*.acdsl` while task contracts are off. A missing or unparsable policy guards nothing; `acdsl check` stays the authoritative gate.
- Operator paths: in a linked worktree of the gate's own source repo (`-gate-repo`), the policy's deploy entry points may not run; the main checkout runs them as the operator.

**Bash**
- The path fence runs on every call. It walks the command in execution order and checks every path argument against the roots: it tracks the cwd through `cd` (the target must be readable), literal variables through `VAR=`, fences command substitutions and `bash -c` scripts as their own commands in a subshell scope, runs loop bodies twice and enumerates a bounded number of loop items, and treats a conditional branch's cwd or variable changes as uncertain afterwards. A panic in the expander fails closed; process substitution and `~user` are rejected up front.
- Argument roles per head: generic (checked for reading when path-shaped — absolute, `~`, `./`, `../`, `.`, `..`, or containing a slash, never a URL), none (patterns, scripts, names, numbers), read, write, and read-when-`@`-prefixed (`curl -d @file`). An unresolved word denies in a path role, and in the generic role only when its raw text is path-shaped. With two or more positionals a spec may mark the last as written.
- Per-head handling: `awk` programs that write or execute are denied; `dd` reads `if=` and writes `of=`; `find` checks its search roots and file-valued predicates (exec and write flags are denied, steered to the pipe form); `sed` file operands are read, or written in place, and a `w` command or unresolved script word denies; `tar` checks `-f` and `-C` by mode (member names on extract are not paths); `rm`/`rmdir` deny critical targets — `/`, a top-level directory, `$HOME`, the anchor, the cwd or any parent of either (a glob counts only by such a prefix); `go` refuses the flags that run another program up to `-args`; any argument naming a protected path denies a call that is not read-only.
- git: `cd`+git is steered; `GIT_*` environment is refused except `GIT_INDEX_FILE` (fenced as a written path); global options are admitted only as `-C <dir>` pairs and value-less options that change neither config nor the executables git runs. Each `-C` target is checked by the subcommand's write class and moves the cwd cumulatively. Subcommands that run a command by design, and per-subcommand options that name a command or plant hooks and config, are refused; words after `--` are pathspecs. A single literal git call is rule-matched with its admitted leading global options cut out.
- Wrappers (`timeout`, `env`, `xargs`, `exec`, …) are unwrapped and the wrapped command judged; a bare wrapper executes nothing. `NAME=value` assignments qualify only with literal values.
- Read-only escape valve: a fully read-only command or batch is allowed. Read-only means a listed program, or a program listed with subcommands (`status`, `worktree list`, `config --get`) whose literal args start with one after its global flags, with no write flag: no `find` exec/write switch, no in-place `sed` or `w` command, no persisting `go env` flag, no ref-mutating `git branch` (a bare positional creates a branch unless a listing flag is present). Redirects must be input, fd duplication, or `>`/`>>` onto `/dev/null`; an unknown redirect fails closed as a write. Anything the walk does not positively recognize disqualifies allow.
- Compounds are denied with a steering reason, except one pipe `a | b` whose two literal segments are each allowed on their own: read-only, or matched by a settings allow rule or skill grant; a deny-rule match or a settings-layer argument vetoes. The make and protected-path checks apply to rule-admitted segments.
- acceptEdits: a single literal call of a filesystem command inside the working directories is allowed (paths already checked by the fence); `sed` is absent because the engine asks for every BSD in-place form — an empty `-i` suffix is read as the script. A flagged `cp` the gate would otherwise allow passes when its flags are short clusters of safe letters and every path lies inside the write roots.
- make: no code-loading flag (`-f`, `-E`/`--eval`, `-I`, `-e` and unambiguous long prefixes), only `NAME=value` words the targets' `## <target>:` help lines declare, with recipe-safe values; `-C` is applied the way make does. A target tagged `[operator]` in its help line is refused from a linked worktree of the gate repo.
- Otherwise a single command is rule-matched: a settings or skill-grant match allows; no match denies.

**Rule matching**
- `Bash(...)`: whole-command glob where `*` matches any text; a trailing ` *` also matches the bare prefix (`Bash(npm run *)` matches `npm run`); `:*` at the end is the legacy spelling of ` *`.
- `WebFetch(domain:...)`: case-insensitive; `*` matches everything, a leading `*.` matches any subdomain depth but not the apex, an inner `*` exactly one label.
- MCP: a bare server name admits every tool of the server, an exact tool string one tool, a glob after a literal `mcp__<server>__` prefix the tools it matches; the server segment never carries a glob.
- `Skill(name)` / `Agent(name)`; `name:*` also admits any value starting with `name`.
- Read/Edit path patterns and `Tool(param:value)` rules parse but never match; the loader reports them.

**Self-service**
- `allow` — validates each rule's shape (a Bash rule on a literal program that is neither a wrapper nor the gate's own binary, an exact WebFetch domain, or one exact MCP tool), refuses the call when the user's last `settings.local.json` message is `#deny`, requires a `#allow` for the rules not yet present, claims the cycle in the session state, then writes the anchor worktree's `settings.local.json` itself (atomic, every other key and the key order kept) and answers APPLIED; rules already present are APPLIED without an approval. A failed write keeps the claim.
- `revert` — removes exactly the last apply's rules — a `toolpolicy allow`, a `/fimplement` apply or a `freeze` apply — once; it needs no approval.
- `check` — answers a CHECK report: per target admitted; denied for want of an allow rule, its candidate marked self-serviceable or not; or denied by the fence, roots or shape, which no rule lifts. A URL target is a WebFetch, a single word naming a rule-bearing tool is that tool, anything else a Bash command. The probe grants drop everything the transcript authorizes (skill invocations, approvals, the human turn) and add the `--skill` leaves' `allowed-tools`.
- `freeze` — records a plan dir's rules file in the frozen ledger (beside the denial ledger) and answers FROZEN; it runs ahead of the human-turn requirement so a headless design run can freeze. A session freezes once per design prompt (the `/fdesign` prompts the user typed, or a headless session's first prompt invoking fdesign), and an already frozen plan only when a design prompt names its plan file. Every rule must pass the shape check and be the gate's own narrowest rule for its target; an absent rules file freezes the empty set. A failed ledger write is an error — an unrecorded freeze never answers FROZEN. The rules are then applied to the session's worktree unless the session is headless.
- Every failure denies. Session state lives at `<ledger dir>/state/<session id>.json` (the last allow's rules and whether it was reverted); without a ledger or session id self-service is denied.

**Frozen rules on /fimplement**
- A `/fimplement` invocation — a typed prompt holding the word at any position, a pasted block included, or a Skill call naming fimplement — resolves its plan: the plan file the arguments name (a path inside the checkout, or a bare name found in exactly one wip plan dir), else the plan file this session last ran `smine-rules plan-sections` on — the one place a design run records the plan it designed; fdesign's enforce block forces that call on the persisted plan path before the turn ends, however the file was written. Nothing else resolves it: not the frozen ledger, not the files the session wrote.
- The arguments are everything after the `/fimplement` word, line breaks included.
- A master plan expands to every derived plan whose `master:` link names it; each derived plan's record applies.
- A typed `/fimplement` at an open code gate is the gate's approval: a plan without a record that the session may still freeze is frozen now.
- Each frozen record's rules that the anchor worktree's `settings.local.json` lacks are appended in one write, validated like `allow`, all or nothing per record, and recorded as one apply for `revert`.
- The report is plain stdout for a prompt and `additionalContext` for a Skill call, one `Permissions` line per record. A prompt wrapped in a harness tag other than a pasted block (a cross-session message) is never the user's own `/fimplement`. Headless sessions pass silently.

**Relay guard**
- Only the relay primer — an inline Workflow script without agents whose completion notification opens a turn that relays nothing — may carry the relay marker; heartbeat skips one root turn end after a launch holding it. In a session with a typed turn a Workflow launches only from a turn a task notification opened, the primer aside.

**Enforce gate rules**
- A deployed skill's rendered enforce declaration beside its SKILL.md: who may write which file, which agent types only the skill's Workflow spawns, which files the dispatcher never writes, which tools plan mode denies. No file or no gate block is no rules; a file that does not parse denies every write, so the defect is visible.

**PermissionRequest**
- Answered with the gate's verdict for the call, so an admitted call never waits on a dialog: checks the engine runs after the PreToolUse answer — a write to a sensitive path, a tool's own consent — end here instead of at the user. ExitPlanMode stays the user's.

**Envelope**
- A tool's args decode as text whatever their shape (a Workflow stage launch passes an object); a Browser pane `actions` list that is not a list of objects decodes to none. Fields the gate never inspects stay undeclared.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | decision JSON or plain-text context on stdout (the `#add`/`#set`/`#reset`/`#disable`/`#enable` reports, the `/fimplement` report), or nothing to say |
| 1 | failure — a non-blocking hook error |
| 2 | usage error |

## Files

- `main.go` — flags, event dispatch, ledger append and output
- `decide.go` — the PreToolUse verdict, protected set, deny reasons, self-service dispatch
- `readonly.go` — the read-only command analysis over the policy's listings; the policy model itself lives in `internal/toolpolicy`
- `bash.go` — Bash parsing: classification, wrappers, redirects, pipelines, make and cd detection
- `fence.go` — the Bash path fence
- `fence_specs.go` — per-command argument roles of the fence
- `gitcall.go` — git global options, exec-capable subcommands and options, rule-match form
- `make.go` — make call parsing and the declared-parameter check
- `roots.go` — the session's read and write roots, engine-protected and critical paths
- `paths.go` — path containment with symlink and glob-segment resolution
- `anchor.go` — the session anchor from the transcript
- `grants.go` — settings layers, skill activation and the transcript scan
- `rules.go` — permission rule parsing, matching and the narrowest rule
- `selfservice.go` — the self-service shape check and the settings list writers; `allow` and `revert` dispatch in `decide.go`
- `check.go` — `check`, in session and from the terminal
- `freeze.go` — `freeze` and the frozen ledger
- `planhook.go` — frozen-rules apply on `/fimplement`
- `permission.go` — PermissionRequest answers
- `enforce.go` — skill enforce gate rules
- `manifest.go` — the `allowed-tools` of a deployed skill's frontmatter
- `relay.go` — the Workflow relay guard
- `login.go` — Browser pane URLs carrying credentials
- `acdsl.go` — the acdsl write guard
- `ledger.go` — the denial ledger
- `state.go` — the session record's path; the record itself is `internal/sessionstate`
- `envelope.go` — hook envelope and output types
- `sessiondirs.go` — folders added in the Desktop session
- `settingsmessage.go` — the `#add`, `#set`, `#reset`, `#disable` and `#enable` messages and the DirectoryAdded event
- `gitroot.go` — linked-worktree detection
