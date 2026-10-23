# rules

Validates the ACTION/RULE/FACT entry files and generates the untracked context file (`context/context.json`) from them and the tracked taxonomy `context/aspects.json`; also carries the deterministic tool steps of several skills — the railroad-review dispatcher (lanes, worktrees, ledgers, build and test), fdesign's plan checks and display copy, /merge-resolve's work branch, the heartbeat demo, the workflow generator and the store paths. Called through `make rules ARGS=…` (runs `go run ./cmd/rules` from source), `make generate`, `make workflow-gen`, the audit, and the skills' tool calls to the installed binary `~/go/bin/smine-rules` (not on Windows), built by the config-server component's installer step.

## Usage

```
usage: rules <interfaces|validate|generate|pack|entries|render-skill|walk|pairs|base|audit-crates|lanes|intake|provision|cleanup|reset-round|ledger|probe-replay|buildtest|prune|snapshot|present|demo-value|demo-close|workflow-gen|plan-sections|plan-claims|plan-display|capture-ui|merge-branch|merge-skillcue|store-check|store-root|settings|settings-defaults|fragment-check|task-check|title> [flags] | rules -h
```

`rules -h` (also `--help`, `help`) prints the command table with a one-line summary each and exits 0; `smine-rules -h`, `smine-rules settings list` and `smine-rules settings get` are in the gate's `read_only_commands` default (`gate.read_only_commands`), so any session may run them without a grant; `smine-rules settings set *` is an allow rule of the gate template. Per subcommand, as each prints its own usage (`rules <command> -h` for the flag-driven ones):

```
rules interfaces
rules validate [--deployed <dir>] [--dir <context-dir>]
rules generate [--dir <context-dir>] [--out <file>]
rules pack [--dir <context-dir>]
rules entries --marker <tag> [--deployed <dir>] [--dir <context-dir>]
rules render-skill [--strip-formal [--disable id,glob*] | --help-test] <skill dir>
rules walk --direction <d> --map <context-map.json> [--files a.go,b.go] [--ceiling 10] [--deployed <dir>] [--dir <context-dir>]
rules pairs --base <commit> --head <commit> [--exclude x,y]
rules base --head <commit> --mainline <branch> [--base <ref>]
rules audit-crates [-base <ref>] [-mainline <branch>] [-scope changed|all]   # the Rust crates whose files changed on this branch (base: the bracket, else the upstream fork, else the merge-base with the mainline), space-separated; all on -scope all, on no base or on an empty change set
rules lanes --round-dir <dir> --directions a,b --base <commit> --head <commit> [--map <context-map.json>] [--lanes 2] [--paths p,q] [--exclude x,y] [--chunks <file>] [--coverage <file>] [--known <file>] [--checklist <direction>=<path>]... [--ceiling 10] [--deployed <dir>] [--worktree-root <dir>]
rules intake --base <commit> --head <commit> [--paths p,q] [--exclude x,y] [--claude-home <dir>] [--gate]
rules provision --repo <repo> --root <dir> --head <commit> --branch <branch> [--overlay <patch>] [--prebuilt p,q] <slug>
rules cleanup --repo <repo> --prefix claude-review/<runNs>- --head <commit> --salvage <dir> [--run <run id>]… [--root <dir>] [--overlay <patch>]
rules reset-round --repo <repo> --prefix claude-review/<runNs>- --head <commit> --salvage <dir> --round-dir <dir> --round <r> [--run <run id>]… [--root <dir>] [--overlay <patch>]
rules ledger --known <file> (--round-dir <dir> --coverage <file> [--autofix-level minor] [--ignore-level none] [--done-after n] [--schema <file>] [--plan-dir <dir>] | --round n --fix-state applied|fixed --ids a,b) | rules ledger --reject <id> --reason <text> --rejected <file> --round-dir <dir> | rules ledger --round-dir <dir> --void
rules probe-replay --repo <repo> --head <commit> --root <dir> --branch <branch> --rounds <dir,dir> [--acdsl <program>]
rules buildtest --repo <repo> --head <commit> --root <dir> --branch <branch> --salvage <dir> [--overlay <patch>] [--skip]
rules prune --artifacts <dir> [--verbose]
rules snapshot --scope wip|all
rules present --review <review.json> [--ignore-level none] [--section <heading>]
rules demo-value --seed <s> [--salt <x>]
rules demo-close --dir <d>
rules workflow-gen [--check] [--root <repo>] <workflows dir>...
rules plan-sections <plan file>
rules plan-claims <plan file>
rules plan-display <plan file> <display file>
rules capture-ui <url|sim> <out.png> [--size WxH] [--wait-ms N]
rules merge-branch create <ours> <theirs> | cleanup <ours>
rules merge-skillcue <base> <ours> <theirs> <path> <ours label> <theirs label>   # git merge driver for skill model files (`<leaf>.cue`): both sides' changelog rows kept, the fresher commit's on top, exit 1 leaves git's text merge
rules store-check [-root <dir>]
rules store-root
rules settings [--claude-home <dir>] [--repo <dir>] list [--json] [<component>] | get <component>.<key> | set <component>.<key> <value>
rules settings-defaults [--check] <path>
rules task-check <task file>...
```

**Context**
- `validate` — parses a context tree (`actions/`, `rules/`, `facts/`) and prints one line per contract violation; with `--deployed`, baseline files are recognized by their synced header.
- `generate` — renders the source `context.json`: entries, tombstones and the aspect taxonomy read from `aspects.json`. The file is gitignored and regenerated on disk wherever it is consumed: `make audit` before the gates, worktree creation (the WorktreeCreate hook and the routine runner's group worktrees — a dir whose index carries a `deploy` record is left alone), config-server start and the config server's reach and aspect edits. Source indexes carry the repo-identity stamp the skill-context hook reach-filters on; a deployed index is cut from the source index by the context deploy engine (`internal/contextdocs`), reach-filtered, unstamped, with the `deploy` record first.
- `pack` — writes the always-reach entry blocks of the whole tree to `~/.claude/context/global/always-context.md`, which the install engine and every context deploy also write — only when the content changed, the previous pack kept in the trash; moves that file into the trash when no always entries exist; a file there without the smine pack header is kept and the run fails naming it.
- `entries` — lists the entries carrying a marker tag, one `id — statement` line each. It parses the `.md` tree directly, so it works in deployed context dirs where `context.json` was not synced; where the file exists, `jq '.entries[] | select(.markers // [] | index("DoD"))'` is the same query.
- `render-skill` — loads a skill dir's model and prints its repo SKILL.md; `--strip-formal` prints the deployed SKILL.md instead (formal clauses and `--disable`d entries — ids or trailing-`*` globs — dropped); `--help-test` prints the `help-test.md` body deploy writes from the smoke block and the skill's `eval/definition.json`, empty when the skill declares neither.

**Railroad review**
- `walk` — prints a lane's checklist (the walk object of its read-gate manifest) for one rule direction: the prose entries the skill's context-map tokens select, plus the language guides' rules for code-style, grouped by family and split at the ceiling. Reads the context dir's `context.json` — the only context a deployed repo or a review overlay carries — never the markdown sources. Exit 1 when nothing is selected, so the dispatcher stops rather than dispatching an empty lane.
- `pairs` — prints a review range's test pairing `{partners, tests}`: `tests` the changed files that are test files or fixtures, `partners` each changed file's tracked counterparts (`git ls-tree` at head; a test without a naming counterpart gets its package's files).
- `base` — prints the review base `{commit, source}`: the bracket, else head's fork point with its upstream, else with mainline. `pairs` and `base` run git in the current directory.
- `lanes` — writes one round's lane manifests next to their artifacts and prints the lane plan `{idFloor, knownPath, lanes, missingContext, skipped}`. Inputs: the review diff (pathspec-narrowed and excluded), its test pairing, the dispatcher's partition (`--chunks`, `[{name, files}]` — contracts lanes always get the whole diff, Go never computes chunks), the coverage ledger's done files, the judgment-direction group-first walks `--checklist <direction>=<path>` names (contracts, correctness; one per direction, read as given, never looked up in the round dir), and the context. `--lanes` sets the lane count, `--ceiling` the walk split size, `--deployed` the context dir whose `context.json` is read (as `walk`), `--map` defaults to the deployed railroad-review context-map.json, `--known <file>` merges the handed-in list with the run's rows into `<round-dir>/known.json` and reports its path and the per-direction id floor. A chunk with no relevant files left lands in `skipped`; directions with no walk entries exit 1, which the dispatcher handles as missing context; an empty diff and a round above 1 without `--coverage` exit 2. A contracts checklist whose `head` is not the review head exits 2 — the diff moved, re-extract it.
- `intake` — prints the review-config analysis of a range from `git diff --numstat` (names and counts, never hunks; renames count as a delete plus an add, and removed files skip the generated-marker read): the diff totals, exclusion candidates by class (`generated`, `lockfile`, `vendored`, `binary` default `excluded`; `prose`, `records` default `kept`) collapsed to directory pathspecs, the chunk count under the default bound (50 files / 4000 lines) with a balanced `chunks: layers[maxFiles:maxLines]` bracket when needed, the lanes and station brackets built on the Claude home's default subagent model, and one `Suggested: /railroad-review …` line whose `exclude:` names only the `excluded` rows; `--gate` appends the reply line the skill's config gate waits on. Runs git in the current directory; exit 2 on an empty diff.
- `provision` — creates one review worktree for a railroad-review worker, the worker's first action on the explicit-worktree route.
- `cleanup` — salvages and removes every worktree of one run (its branch under `--prefix`, its path under `--root`, or a harness worktree of a `--run`) and deletes the run's branches after saving their diffs. Foreign claude-review worktrees at the review head are reported, never touched.
- `reset-round` — resets one failed round before its re-dispatch: the round dir's files move into the salvage dir, then the round's worktrees and branches are salvaged and removed as `cleanup` does.
- `ledger` — updates the known and coverage ledgers from the round's `review.json` and prints the autofix split, the ignored ids and coverage gaps — a new row at or below `--ignore-level` is ledgered `ignored`, never fixed; `--fix-state` moves the named rows of one round to applied or fixed; `--reject` appends one human-rejected finding to the rejection ledger; `--void` records a round without a handoff and prints `{void: true, void_rounds: n}`, `n` the `round-*` siblings of the round dir, itself included, holding no `review.json`. Findings confirmed again after their fix landed are printed in the document's `oscillating` list and noted on stderr; the call still exits 0, so the railroad-review `ledger` anchor captures them.
- `probe-replay` — replays every persisted probe of the named rounds against the post-fix head in a fresh worktree built with `--build`; `--acdsl` names the acdsl program, else the tree's `bin/acdsl`; without a probe nothing is provisioned or built and `--build` is not needed; hits are listed in `hits`, exit 0; a missing `--build`, a failed build, a missing `bin/acdsl` or an unreadable round exits 2.
- `buildtest` — builds and tests one commit in a disposable worktree with `--build` and `--test` and prints `{branches, build, salvage, tests}`; `--skip` runs nothing and answers both steps `skipped`.

railroad-review passes `--build` and `--test` from its `SKILL_RAILROADREVIEW_BUILD` and `_TEST` variables, the repo's values on the config server's Environment page. Without one the command exits 2 with `No build command — the repo's build and test commands are set on the Environment page` (`test` likewise). A value is one command per line, each split by `internal/cmdline` (`SplitLines`) — spaces split words, double quotes group them, no shell operators — and run in order without a shell; the first red line is the step's outcome. Nothing is guessed from a Makefile or `go.mod`.
- `prune` — reduces a finished review's artifact dir to its keep list — `salvage/` always on it — by moving every other file into the trash, and prints `{kept, removed, stash}` (`stash` the first move's stamp); `--verbose` also keeps every round's lane reports and verdicts.
- `snapshot` — commits the working tree of the current repo on top of HEAD through a temporary index and prints `{snapshot, differs}`.
- `present` — prints the gate text, or with `--section` the plan's review section, rendered from `review.json` and the known ledger beside the round dir: linked locations, a State column, the ignored line for rows at or below `--ignore-level`, refused, unverified, probes and uncovered blocks.

**Plans**
- `plan-sections` — checks a plan file and prints one `file:line: message` per violation; a clean run prints `plan-sections: ok — <gate> gate`. See below.
- `plan-claims` — prints a plan's refute ledger as JSON: `{schema: 1, claims: [{claim_id, text, evidence, source}]}`, one claim per `A<n>` row of the Assumptions table — `text` the Reality cell, `evidence` the Location cell, `source` the Assumption cell plus `— decisions: <ids>`, the decisions whose Facts cell links the row; no table prints an empty `claims`. The fdesign refute workflow takes `claims` verbatim under a provenance check.
- `plan-display` — writes a plan's display copy (RULE-PLAN-092), the session plan file peek shows and the gate message links outside plan mode.
- `capture-ui` — writes a PNG of a running UI for a plan's visual evidence: headless Chrome for a URL, the booted iOS simulator for `sim`; prints the absolute path of the PNG. `--size WxH` becomes Chrome's window size.

**Other**
- `interfaces` — the provider handshake: prints `{"interfaces": [...]}`, the sorted `<id>@<version>` list of `skills/interfaces.cue` interfaces this build implements. The skills deploy runs it once per run and leaves a variable unset when the answer lacks its interface; a test pins the list to the catalog's `smine-rules` provider.
- `demo-value` — prints the heartbeat-demo producer value: the first 12 hex characters of `sha256(seed/salt)`, deterministic so provenance has a value to compare.
- `demo-close` — writes the heartbeat-demo closing marker `closed.json` the stop check requires into the run's dir; a denied stage never created the dir, and the close still lands.
- `workflow-gen` — regenerates the generated blocks of the stage workflow files in the given workflows dirs, or with `--check` fails on a stale file; both modes enforce the workflow-layer shape rules (`internal/workflowgen`).
- `merge-branch` — /merge-resolve's work-branch naming and cleanup: `create` derives `claude-merge/<theirs-slug>-<ours-slug>` (lowercased, alphanumeric runs joined by `-`), checks it out and prints its name; `cleanup` checks out `<ours>` and deletes the `claude-merge/*` branches already merged into it (`-d` only, never `-D`); unmerged branches are kept and reported.
- `store-check` — validates the plans-archive backlog store (`backlog.json`) of the repo it runs in against the schema of the code checkout; `-root` replaces the resolved store root.
- `store-root` — prints the store root of the repo it runs in, the one path skills name the stores by, so a session never spells the archive path itself.
- `title` — `title set <TOKEN>` confirms the session-title state a skill's gate step sets: prints `title: set <TOKEN>` when the token is `WIP`, `READY` or a state a deployed skill's `title.json` declares (`-skills` names the deployed skills dir, default `~/.claude/skills`), exit 2 otherwise. The sessiontitle hook records the confirmed call and renames the session at the turn end (`cmd/sessiontitle`); the skill model requires the gate entry bound to a state to run its set command.
- `task-check` — validates smine change task files (`.smine-run/changes/<kind>-<slug>.json`) against the task schema embedded in `internal/routines`: one `<file>:1: <message>` line per violation on stdout, `task-check: N file(s) valid` when clean; problem ≤ 320, fix ≤ 240, each of at most 3 quotes ≤ 160 characters; listed in the gate's `read_only_commands` default, so miners and the smine dispatcher run it without a grant.

**Settings**
- `settings` — the agent's path to this machine's hook and heartbeat settings ([docs/settings.html](../../docs/settings.html)); `--claude-home` names the Claude home whose record is read and written (default `~/.claude`), `--repo` the checkout whose fragments the apply renders from (default `.`).
  - `list [--json] [<component>]` prints one line per row — an asterisk marks an override — with kind, current value, default and help; `--json` prints the rows with their current values.
  - `get <component>.<key>` prints the effective value, then `default:`, `source: record|default`, `kind:`, `applies:` and `help:` lines.
  - `set <component>.<key> <value>` parses the value by the row's kind (a list kind takes a JSON string array), stores it as an override in the install record — a value equal to the default clears the override — applies that component through `install.ApplyComponent` (a component with hooks config rows applies Session hooks too), prints the apply summary, then `reload: live` or, for a `restart` row, `restart: the consumer was restarted`. A wrong key or value exits 2 with the row's help and valid form; a failed apply exits 1.
  - Clearing a hook-group override (`groups.<smineId>.timeout|matcher|async` back to its default) reverts the live group on that apply: the three fields are render-owned and copied onto an equal-version live group; the group's command is not — a changed command needs a `smineVersion` bump (`fragment-check`).
- `fragment-check [--base <ref>] [--mainline main] [--settings settings/claude_code]` — compares every component fragment's hook groups with their copy at the branch base (the upstream fork point, else the merge-base with the mainline, `--base` overriding) and fails on a group whose name or hooks — type, command, url; the render-owned timeout and async excluded — changed without a higher `smineVersion`: the apply keeps an equal-version live group's command, so such a change never reaches a machine. A fragment or group absent at the base is new and passes; `make fragment-check` and the `fragment-check` audit unit.
- `settings-defaults [--check] <path>` — writes the heartbeat crate's defaults fixture (`cmd/heartbeat/tests/fixtures/config-defaults.json`, every heartbeatConfig row's default in its rendered form), or checks it; `make settings-defaults`, compared by the crate's `defaults_match_the_catalog_fixture` test.

## How it works

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | clean |
| 1 | violations |
| 2 | usage or tool error |

`capture-ui` has its own: 64 usage, 69 no capture tool, 70 nothing written. `buildtest` exits 0 on red build or test results — they are data for the dispatcher, not a command failure. `cleanup` and `reset-round` exit 1 when a worktree or branch was skipped.

**plan-sections**
- Checks the Changes section (RULE-PLAN-088) and the driver coverage (RULE-PLAN-096); for plans under the order cutoff, the order of the Changes entries (RULE-PLAN-098).
- For plans dated on or after the cutoff: the Decisions table's Adds column (counts in the fixed vocabulary plus a Reuses clause), the phase table against the per-entry phase lines (on a mismatch it prints the table the phase lines imply), and at most four entries per phase.
- After the ok line it prints the Adds totals line, then one line per Drivers row once the Changes hold entries.

**plan-display**
- The source must be a wip plan dir's plan file, `plans/<feature folder>/<plan dir>/design/<slug>.md`.
- The display file must be a markdown file directly inside the Claude home's plans folder and, if it exists, a regular file; the path is checked where the write happens, since the caller names it.
- The copy is the plan's text without projection blocks, its image embeds under `ui/` pointing at the design folder, and — when the plan dir holds a rules file — the rules between the args line and the first section.

**capture-ui**
- Chrome is the app bundle under `/Applications`, then the user's Applications dir, then a Chrome or Chromium on PATH. Whether an image landed is checked after the capture.

**cleanup and reset-round**
- A scope selects worktrees on a branch under the prefix or under the root, the harness worktrees of the run ids (directly under a `.claude/worktrees` dir, named `<run id>-…`), and with a round only the prefix's branches naming `-r<round>-`.
- Flags must hold head, prefix (under `claude-review/`), repo and salvage, and every run id must be a workflow run id.
- Each selected worktree is salvaged into a fresh dest under the salvage dir (`dest`, else the first free `dest-<n>` from 2 — never an earlier one) and removed; a failed salvage is skipped with its error and the loop goes on. Then the worktrees are pruned and the selected branches deleted after their diffs are saved into a fresh branches dir. A run worktree's branch is its harness `worktree-wf_` branch or one under the prefix; a detached head deletes nothing.
- `reset-round` requires the existing `round-<n>` dir and a salvage dir outside it; the round dir's entries move into a fresh `<salvage>/round-<n>`, leaving the round dir empty.
- Output: the report as JSON (removed and skipped worktrees and branches).

**Stores**
- The store root is named after the main checkout (the parent of the common git dir), so every worktree of the repo resolves the same root; a confined session gets the root its runner set (`config.StoreRootEnv`).
- The backlog schema lives in the code checkout, the store in the plans-archive repo. The schema is compiled even without store files, so a missing schema never passes; a missing store root is an error, so a wrong repo name never passes as an empty store.
- Output: one `<file>:1: <message>` line per violation on stdout; a clean run prints `store-check: N file(s) valid under <root>`.

**Tasks**
- `task-check`'s schema is embedded in the binary, so it checks from any worktree without a checkout path. The runner validates against the same schema and skips an invalid task file in the run report.

## Files

- `main.go` — dispatch and the context, review and render-skill subcommands
- `interfaces.go` — `interfaces`
- `intake.go` — `intake`
- `cleanup.go` — `cleanup` and `reset-round`
- `captureui.go` — `capture-ui`
- `demo.go` — `demo-value` and `demo-close`
- `fragmentcheck.go` — `fragment-check`
- `mergebranch.go` — `merge-branch`
- `planclaims.go` — `plan-claims`
- `plandisplay.go` — `plan-display`
- `plansections.go` — `plan-sections`
- `settings.go` — `settings` and `settings-defaults`
- `storecheck.go` — `store-check` and `store-root`
- `taskcheck.go` — `task-check`
- `title.go` — `title`
- `workflowgen.go` — `workflow-gen`
