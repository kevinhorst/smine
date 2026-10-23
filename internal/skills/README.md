# skills

The skills sync and bundle: renders repo skills through `internal/skillgraph` into the Claude and Codex skill homes, deploys agent definitions, and writes the standalone bundle.

## Value resolution

`Values(ctx, dir, rows, isDeferring, prober, set, skill)` resolves every declared env variable, sorted by name, as a `VariableValue{Cause, Interface, IsDeferred, Name, Provider, Required, Source, Value}`. A deferring resolution marks every repo-scoped variable `IsDeferred` with source `invocation` and no value — it is filled at invocation, never unset and never in `ValueMap`; every other variable resolves:

1. `environment` — the deploy process's `SKILL_*` variable of the same name
2. `record` — the `SKILL_*` row of `SyncOptions.Rows` (`Rows.Skills`, the install record's `skills.variables`)
3. `global default` — `GlobalDefault`: for a repo-scoped interface only, `Rows.Defaults[<interface id>]` (the record's `skills.defaults`)
4. `provider <id>` — the catalog provider claiming the bound interface version: its `PROVIDER_<ID>` environment variable, then its `PROVIDER_<ID>` row (`Rows.Providers`, the record's `skills.providers`), then its catalog program when its component is in `set` and its platforms match; a nil set (the canonical deploy) counts every component as installed; `acdsl` has no program and fills nothing without a row
5. `unset`

A found value is then validated, first failure wins: `skillgraph.CheckValue` (the value rules), then the interface's `validate` through `ValidateLocation(dir, …)` — `resolves` looks the first word of each command line up on PATH (`~/` expanded, a relative word with a slash joined onto `dir`: the checkout at deploy, the repo's path for a repo value set on the skill page), `exists` stats the path as a dir or file — then, for a value that came from the provider, the provider's handshake must claim the bound version. A failure makes the variable `unset` with that `Cause`, e.g. `smine-rules does not implement plan-sections@1 (answered: …)`.

The `Prober` answers the handshake: `deployRun.Claims` runs `<executable> interfaces` (`Binary` when set, else `Program`) under a 10 s timeout once per executable per run and caches the answer or the failure; a probe failure is the cause for every variable of that provider. `NewProber` gives the config server the same cache outside a deploy run. A nil prober skips the handshake; the bundle run passes none.

`ValueMap` is the name → value view the render takes; unset and deferred names are absent. `DeferredNames` is the set the render passes as `RenderOptions.Deferred`. `Unconfigured` renders each unset required variable with its cause for the skip line; a deferred one never counts.

`OrphanRows(repoDir, rows)` lists every `SKILL_` row no repo skill declares and every `PROVIDER_` row no catalog provider declares, sorted, and removes nothing — a checkout lacking a skill is no authority on the user's values; a checkout without `skills/` lists nothing. `install.SkillRows(record)` is the copy of the record's rows the callers pass, `install.SetSkillRows` stores one back.

## Deploy run

- Every writer path passes the install record's components and skill rows: `configserver -sync-skills`, the install engine, the config server, which reads the record for every sync, apply and skill deploy.
- Each home renders on its own: `~/.claude/skills` with deferral on — repo-scoped references become invocation placeholders ending in the hint "set it for this repo on the skill page, or export it" — and `~/.codex/skills` with deferral off, so there a repo-scoped variable fills from the deploy environment, the record row, the global default (`GlobalDefault`), then the provider, else drops or skips the skill: `skip <leaf> unconfigured (…) in <home>`. Variants and `DeployLeaf`'s refusal judge the Claude home.
- A deployed dir holds the rendered SKILL.md, help-test.md, title.json, enforce.json, env.json and answer.md, each only when its render condition holds (`internal/skillgraph`); alias dirs copy them unchanged.
- The record's skill rows have four writers, the skill page's Environment save, the Environment tab's provider save, its global defaults save (`skills.defaults`, keyed by interface id) and `configserver -install`; none removes a row, and `-install` prints `orphan row kept: <key>` per `OrphanRows` hit; the skill page's Environment widget lists the orphans, each with a Forget button (`POST /scripts/skills/environment/forget`, the key and value journaled). A `SKILL_` row of a repo-scoped variable is that variable's default; the per-repo values live in the registry row's `env` and reach a session through the `skillenv` hook, never through a deploy.
- Each deployed dir's `.smine-deployed` marker holds two lines: `source <sha256>`, the hash of the skill dir's files but `eval/` and `smoke/`, projection blocks stripped, and `rendered <sha256>`, the hash of the files the deploy wrote (path NUL content NUL in path order, marker excluded; an alias copy carries its own). `IsDeployedCurrent` compares the first line with the checkout, which is all the smoke checks before it runs.
- Before a deploy writes a dir it checks ownership: an unmarked dir under a deployed name, or a marked one whose files no longer hash to the `rendered` line (a marker without the line counts as modified), moves whole into the trash (`SyncOptions.Trash`) and the report says `<dir>: <reason>, previous copy stashed at <stash>`.
- Nothing in a skill home is deleted: a file the render no longer produces, a marked dir outside the selection under prune, a dir removed from the Skills page, an agent of an unselected skill and the two retired agent tool scripts move into the trash with a journal line.
- Agent files carry `# smine-deployed <sha256 of the unmarked render>` as line 2 inside their front matter (line 1 without front matter). An installed file without it is kept and reported `kept: no smine marker`, never written over or removed; one byte-equal to the unmarked render is adopted and rewritten with the marker.
- The sync never touches a user settings file: a casual profile gets the routine overlay `~/.config/claude-routine/skill-overrides.json` (every deployed leaf and alias `on`), a developer profile has it removed. `SetOverrides(leaves, path, value, keep)` writes `skillOverrides` into a settings file for the Skills page's visibility action, the previous bytes kept in the trash. The audience migration (`non-developer` → `casual`) keeps the previous profile too.
- `deployableLeaves` marks each repo leaf whose `requires` the set meets and whose required variables are set or deferred; the render drops `Skill(<leaf>)` grants and edges to the rest.
- A skill with an unset required install-scoped variable is skipped with `skip <leaf> unconfigured (SKILL_…: <cause>)`; `DeployLeaf` refuses it.
- `leafFiles` fills every UTF-8 sibling file (`workflows/`, `schemas/`, `context-map.json`, …) with `FillVariables`; `deployAgents` does the same for `agents/*.md`. Binary files ship as they are.

## Bundle and its check

- `Bundle(outDir, repoDir)` renders for the empty component set with no record rows — the smineless form — with deferral on and the hint "export it in your shell", skipping skills with `requires`, a license key or an unset required install-scoped variable, writes `dist/skills` plus its index `README.md` (`- <leaf>[: with the full install also <summary>, …]` per shipped skill, from its unset optional install-scoped variables; `- <leaf> (skipped): needs <what>` per skipped one — the components, a license, or `<ref> from the <component> component`), then scans it. A grant reading a repo-scoped variable drops, since the set holds no gate to fill it.
- `CheckBundle(repoDir)` runs the same render in memory and returns `leaf/path:line: term` hits; `skillmodel render -check` (`make skills-check`) adds them to its drift list.
- The scan pattern is `skillgraph.LiteralNeedles`, any surviving `$SKILL_` reference (a placeholder reads `ENVIRON["SKILL_…"]` and holds none), and `make <target>` for every `## <target>:` help line of the repo's Makefile.
- Values still come from the process environment, so a `SKILL_*` or `PROVIDER_*` variable exported in the shell fills the check render; the bundle render probes no provider.
