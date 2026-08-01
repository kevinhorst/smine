# Routines — Operations

Local launchd jobs wrapping headless `claude -p` runs. This README covers what
runs, what you can adjust and where, what must exist before anything works, and
how to observe and debug a run. The generic `claude -p` mechanics (flags, auth
options, stream-json) are in [docs/nightly-routines.md](../docs/nightly-routines.md);
authoring a new routine is `/skillroutine-create` (routine route).

## What runs

| Routine | Schedule (local) | Group / branch | Prompt | Budget |
|---|---|---|---|---|
| `smine-nightly` | 03:00 | `smine-nightly` → `routine/smine-nightly` | `/smine --nightly`, then `/smine-apply <votes file>` when votes are pending (two stages, one publish) | $15 per stage |

Each group owns one worktree at `~/.cache/claude-routine/worktrees/<group>` and
one branch `routine/<group>`. Chain members (routines sharing a `ROUTINE_GROUP`)
serialize on the group lock; different groups run concurrently and never touch
each other's worktrees. Output is
committed locally on the group branch — no push, no PR. **Locally merging the
group branch accepts a run; resetting it to main discards one.** A run that
failed (non-zero exit) is still committed but its subject is prefixed `[failed]`;
a branch whose every unmerged commit is `[failed]` is reset to main on the next
create instead of stacking new work on dead runs.

If a branch has diverged from main with a mix of merged, manual-merge, and real
commits (so the ancestor and all-failed resets never fire), reconcile it by hand
once — merge it into main (or cherry-pick what you want, discard the rest), then
`git branch -f routine/<group> main` — to return to the steady state the create
logic assumes.

## What you can adjust, and where

| Knob | Where | Takes effect |
|---|---|---|
| Schedule | `StartCalendarInterval` in `<routine>/com.smine.routine.<name>.plist` | after re-bootstrap (below) |
| Model, effort, budget, timeout | flags on the `claude -p` call in `<routine>/run.sh` (`--model`, `--effort`, `--max-budget-usd`, the `timeout 3600` wrapper) | next run |
| Prompt / wrapped skill | `PROMPT=` in `run.sh` (the smine-nightly apply stage builds its prompt inline) | next run |
| Group membership | `ROUTINE_GROUP=<group>` in `run.sh` before sourcing `_lib/worktree.sh`; default is the routine's own name | next run — creates a new `routine/<group>` branch |
| Worktree root / branch name | `ROUTINE_WT_ROOT` / `ROUTINE_BRANCH` env overrides (defaults in `_lib/worktree.sh`) | next run |
| Per-routine params | `EnvironmentVariables` in the routine's plist — editable from the config server's Routines page (Configure form: one input per plist-declared key), which rewrites the plist and re-bootstraps | after the re-bootstrap the form does |
| Run every N days | `ROUTINE_CADENCE_DAYS` in the plist's `EnvironmentVariables` (Configure form). launchd stays on its daily schedule; `_lib/cadence.sh` gates the run against a `<routine>/.cadence-stamp` file and skips until N days have passed. Unset or `1` = every scheduled run | next run |

Plist changes need a reload from the **main checkout** (never a worktree):

```bash
launchctl bootout gui/$(id -u)/com.smine.routine.<name>
```

```bash
launchctl bootstrap gui/$(id -u) <repo-root>/routines/<name>/com.smine.routine.<name>.plist
```

## Prerequisites (nothing runs without these)

1. **Token** — `~/.config/claude-routine/token`, non-empty, from `claude setup-token`
   (1-year OAuth; strip whitespace when pasting). Missing/empty → run exits 78
   before doing anything. Never cat it; check with `[[ -s ]]`.
2. **Tools** — `brew install flock coreutils` (macOS ships neither `flock` nor
   `timeout`); `jq`; `claude` on PATH (run.sh prepends `/opt/homebrew/bin`).
3. **Bootstrap** — handled by the configserver: at startup it bootstraps every
   routine that is not degraded, already loaded, or stopped via the UI (Stop
   persists through `launchctl disable`; Start re-enables). Since the
   configserver runs as a LaunchAgent, a fresh login re-loads all enabled
   routines automatically. Manual `launchctl bootstrap` (command above) is only
   needed for ad-hoc testing without the server. Verify by exit code, not
   output: `launchctl print gui/$(id -u)/<label>` — 0 loaded, 113 not loaded.

## Observing a run

- **Result line per run** — `<routine>/results.jsonl`: timestamp, exit_status,
  session_id, num_turns, total_cost_usd, first 300 chars of the agent's result.
  First stop for "did last night work".
- **Wrapper stdout/stderr** — `~/Library/Logs/claude-routine-<name>.out.log` /
  `.err.log` (read stderr first).
- **launchd state** — `launchctl print gui/$(id -u)/<label>`: classify by the
  `runs` and `last exit code` counters, never by the `state` string (a finished
  job always reports `state = not running`).
- **The work itself** — `git log main..routine/<group>` in the repo the group
  runs against.
- **Full transcript** — the session_id from results.jsonl, via peek-mcp.

## Debugging

- **Manual run**: execute `<routine>/run.sh` directly from the main checkout —
  same code path as launchd. `launchctl kickstart gui/$(id -u)/<label>` runs it
  with launchd's environment instead.
- **Exit codes**: 78 = precondition (token), 75 = group lock
  timeout (2 h), 70 = worktree create failed or worktree vanished mid-run,
  0 with `is_error: true` in results.jsonl = the agent itself reported failure.
- **"already running"** = the routine's own `.lock` (self-overlap);
  **group lock timeout** = a chain sibling held `routine/<group>` too long.
- **Leftover worktree** under `~/.cache/claude-routine/worktrees/<group>` after
  a failure is intentional (kept for inspection); the group's next create
  removes it. Only that group's own path is ever swept — sibling groups are
  never touched.
- **smine-apply vote safety** (smine-nightly apply stage): the live sidecar `sessions/proposals/votes.jsonl`
  is *copied* (never moved) into the worktree as `votes-processing-<ts>.jsonl`, so
  the sidecar is never emptied by the wrapper. The agent appends every terminally
  processed vote to `votes-archive.jsonl` (with its disposition); after a committed
  publish the wrapper drains the live sidecar by removing exactly those archived
  `(kind, id)` keys. A half-failed night still drains whatever the agent finished,
  so progress is monotonic; over-cap votes stay live for a future run; a failed
  publish leaves the sidecar fully intact. The wrapper also fails the run when the
  claude JSON envelope reports `is_error`/non-`success`, not just on a non-zero exit.
  `SMINE_APPLY_CAP` (default 3) overrides the per-run implementation cap.
- **Lib regression tests**: `bash cmd/tests/test_routine_worktree.sh`
  (branch reuse/reset, own-group sweep, sibling survival, commit-body handoff).
