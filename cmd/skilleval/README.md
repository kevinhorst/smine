# skilleval

The eval and A/B layer toolbox, and the smoke runner. The /skillroutine-eval and /skillroutine-ab skills drive it step by step: initialise a run or an A/B from a skill's `eval/definition.json`, provision the run worktrees, run one arm, render judge briefs, clean up after the judges, fold a run dir, collect and report an A/B, generate or check derived goldens. The smoke targets run one skill's smoke and judge an interactive smoke's session.

## Usage

```
usage: skilleval ab-collect <ab-dir> | ab-init [-layer eval|smoke] [-replicas <n>] [-token <label>] -variant <descriptor> <skill> | ab-report <ab-dir> | arm [-heartbeat <bin>] <run-dir> <label> | brief adherence|golden <run-dir> <label> | brief compare <run-dir> | cleanup <run-dir> <label> | compose [-golden produce|test] [-set <name>[=<value>]]… <skill> | fold [-peek <url>] [-schema <path>] <run-dir> | goldens gen <master> | goldens check | init [-golden produce|test] [-replicas <n>] [-set <name>[=<value>]]… -token <label> <skill> | promote <run-dir> [<label>] | provision <run-dir> | smoke [-heartbeat <bin>] [-token <label>] <leaf> | smoke-verdict [-heartbeat <bin>] <leaf> <session-id>
```

- `init` — creates a dated run dir under `evals/` from the definition and prints it. `-replicas` 0 means the definition's count, else 1; `-token` is the stored token label the runs bill; `-golden` picks produce or test (default test when the definition has a golden); `-set name=value` or a bare flag name sets an arg the definition opens, repeatable.
- `compose` — prints the prompt `init` would run, with the same `-golden` and `-set`.
- `provision <run-dir>` — creates the run's worktrees.
- `arm <run-dir> <label>` — runs one arm, sleep prevented, reading the session's enforced runs from heartbeat (`-heartbeat`, default `~/go/bin/smine-heartbeat`).
- `brief adherence|golden <run-dir> <label>` prints one label's judge brief; `brief compare <run-dir>` the whole run's.
- `cleanup <run-dir> <label>` — removes an arm's leftovers after the judges and prints the report as JSON; exit 1 unless everything was removed. Every worktree is salvaged into `<arm worktree>-out/salvage/` and branch diffs into its `branches/` before removal; once every worktree is gone the whole `-out` dir, salvage included, moves into the trash (`stash` in the report), else it stays.
- `fold <run-dir>` — folds a run dir into its eval record, reading sessions from peek-mcp (`-peek`, default `http://127.0.0.1:<peek.port>/mcp` from the install record) and validating against `-schema` (default `skills/skillroutine/skillroutine-eval/reference/schema.json`). An adherence verdict `not-observable` — an entry run inside a Workflow agent, whose work `trace.md` does not carry, with no trace line or artifact showing it — counts like `not-applicable` toward `step_miss_pct` and is counted per run in `step_unobserved` and the report's Unobservable entries section.
- `promote <run-dir> [<label>]` — promotes a run (or one label) into a seed file and prints its path.
- `ab-init <skill>` — creates an A/B dir and prints it. `-layer` eval or smoke; `-replicas` runs per side; `-variant` is one changed factor: `model:<name>`, `effort:<level>`, `no-<mcp>`, `overlay:<patch>`, `version:<a>..<b>`; `-token` is unused by a version variant.
- `ab-collect <ab-dir>` writes the A/B's `ab.json`; `ab-report <ab-dir>` renders its report.
- `goldens gen <master>` regenerates derived goldens from a master; `goldens check` fails on stale derived files.
- `smoke <leaf>` — runs one skill's smoke; `-token` is the stored token label a headless smoke bills.
- `smoke-verdict <leaf> <session-id>` — judges the session of an interactive smoke.

## How it works

**Environment**
- `arm` and `smoke` set `CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS=0` for the cells they start.
- The heartbeat and peek addresses come from the install record (`heartbeat.port`, `peek.port`); an unreadable record keeps the catalog default ports.
- SIGTERM and interrupt cancel the running command.

**Exit codes** — the eval and A/B skills stop on these

| Code | Meaning |
|---|---|
| 0 | done; for a smoke: pass, skip, or interactive instructions printed |
| 1 | failure; for a smoke: fail or harness error |
| 2 | usage error, or a bad run arg |
| 3 | smoke: the deployed copy differs from the checkout |
| 64 | `goldens` usage error |
| 65 | bad golden master |
| 66 | golden master missing |
| 67 | provision: repo not registered |
| 68 | provision: unresolvable ref |
| 69 | the skill has no eval definition |
| 70 | unknown token label |
| 71 | over the usage window |
| 72 | bad A/B variant |
| 73 | configuration not runnable |
| 74 | golden test without a golden |
| 75 | `-set` on an arg the definition keeps closed |
| 76 | the run manifest changed under the command |
