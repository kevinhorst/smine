# routine

Runs local routines. `run <routine-dir>` is the launchd / Task Scheduler entry point; `worker` runs one fleet worker cell for an orchestrator session; `bootstrap` runs the one-shot smine bootstrap on the main checkout; `cell`, `allowed-tools`, `publish-style` and `smine-mcp-config` serve the operator.

## Usage

```
usage: routine run <routine-dir> | cell -timeout <s> -- <cmd...> | worker -name <name> -skill <leaf> (-base <ref> | -resume <session id>) [-args <text>] [-attended] [-keep-stopped] [-model <m>] [-effort <e>] [-budget-usd <n>] [-token <label>] [-timeout <s>] | bootstrap | allowed-tools <skill> | publish-style <staged-file> | smine-mcp-config
```

- `run <routine-dir>` — runs the routine the dir defines; the exit status is the runner's.
- `cell -timeout <s> -- <cmd...>` — runs one command as a cell under a deadline in seconds, sleep prevented; `ROUTINE_CELL_PIDS` in the env is handed to the cell; the exit status is the command's.
- `worker` — runs one fleet worker cell from the repository of the cwd (resolved through the git common dir, so the same from any worktree) and prints its result as one JSON line; the exit status is the cell's.
  - `-name` — worktree dir and `claude/<name>` branch;
  - `-skill` — skill leaf the cell's one prompt invokes; `-args` — text after `/<skill>`, or the whole prompt of a resumed cell;
  - `-base <ref>` — ref the worker branch forks from; or `-resume <session id>` — continues this worker's stopped cell in its kept worktree;
  - `-keep-stopped` — keep the worktree of a stopped cell for `-resume`;
  - `-attended` — start the cell without the headless marker;
  - `-model`, `-effort`, `-budget-usd` — the cell's model, effort and spend cap;
  - `-token <label>` — token label; empty uses the launching session's login;
  - `-timeout <s>` — deadline in active seconds, default 14400.
- `bootstrap` — the one-shot smine bootstrap on the checkout of the cwd.
- `allowed-tools <skill>` — prints the deployed skill's `allowed-tools` line.
- `publish-style <staged-file>` — publishes a staged style profile.
- `smine-mcp-config` — prints the MCP config the smine cells run with, from Claude Code's `.claude.json`.

## How it works

**Exit codes of the dispatcher**

| Code | Meaning |
|---|---|
| 2 | usage error |
| 78 | configuration error (routine dir unreadable, not in a git repository, config or publish failure) |
| 127 | a cell or worker could not be started |

Every other status comes from the routine run, the cell's command or the worker cell.

**Signals**
- SIGTERM and interrupt cancel the run's context, so a launchd or Task Scheduler stop reaches the running cell.
