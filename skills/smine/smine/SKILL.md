---
name: smine
description: Run the full session-mining pipeline — mine transcripts into batch reports, then fan each to five dimension skills. Trigger on /smine or "run a session retrospective". Args — --nightly: all pending batches; --no-batch: route existing; --no-<dimension>: skip one; --subagents: mine subagent transcripts; --max-proposals-*: caps; --repos/--agents: roster filters; --since/--last: date floor / newest-n; --dev: include smine/routine sessions.
author: Kevin Horst
version: 1.16
argument-hint: "[--nightly] [--no-batch] [--no-<dimension>] [--subagents] [--max-proposals-per-dimension n] [--max-proposals-mined n] [--repos name=path,...] [--agents claude,codex] [--since YYYY-MM-DD] [--last n] [--dev]"
allowed-tools: Skill(smine-batch), Skill(smine-context), Skill(smine-memory), Skill(smine-permissions), Skill(smine-routines), Skill(smine-skills), Task, ToolSearch, Read, Write, Edit, mcp__peek-mcp__session_list, mcp__peek-mcp__session_events, mcp__peek-mcp__session_get, Bash(jq *), Bash(go run ./cmd/acdsl *), Bash(make audit *), Bash(git diff *), Bash(git log *), Bash(git status *), Bash(ls *), Bash(cat *), Bash(head *), Bash(wc *), Bash(grep *)
enable-model-invocation: true
---

# smine (pipeline)

Mine past session transcripts, then route every batch to its dimension skills — one entrypoint for the whole retrospective. This skill resolves the flags and (in `--no-batch`) the target batches, then runs the mine-then-route pipeline directly; the `session-mine` workflow (`workflows/session-mine.js` in this skill's directory) is the manual-run alternative for interactive sessions.

## When to use

**Use when:** mining past sessions end-to-end (mine + route), or routing already-mined batches that were never fanned out (`--no-batch`). Invoked via /smine.
**Don't use when:** a quick look at what a session did — /peek. Only one dimension on one batch — invoke that dimension skill directly (e.g. /smine-memory, /smine-context). Just the raw transcript miner without routing — /smine-batch. Evaluating a skill's runs — /skillroutine-eval.
**Preconditions:** peek-mcp available for the mine stage (not needed with `--no-batch`); no batch currently being written.
**Workflow position:** **smine** = smine-batch (report + JSON) → dimension fan-out (smine-memory, smine-skills, smine-routines, smine-context) → proposals (see README.md § Skill map, smine repo).

## Args

- `--nightly`: unattended mode — mine all pending batches (headless), route each, no stop for review. For the launchd routine.
- `--no-batch`: skip the mine stage; route the existing unrouted batches resolved from the ledger cursor (§1). For retro-analysis of already-mined batches.
- `--no-memory` / `--no-routines` / `--no-context` / `--no-skills` / `--no-permissions`: skip that one dimension in the fan-out. Flag → dimension skill:
  - `--no-memory` → smine-memory
  - `--no-skills` → smine-skills
  - `--no-routines` → smine-routines
  - `--no-context` → smine-context
  - `--no-permissions` → smine-permissions
- `--subagents`: passed through verbatim to smine-batch — mine each session's per-subagent transcripts too (expensive; the nightly wrapper sets it from `SMINE_SUBAGENTS=1`). Absent → smine-batch mines top-level transcripts only.
- `--max-proposals-per-dimension <n>`: cap each proposal-producing dimension (context, memory, routines, skills) at n new proposals this run; passed into each dimension invocation.
- `--agents <claude,codex>`: which agents' sessions to mine; absent → both. Passed through verbatim to smine-batch (the nightly wrapper assembles it from `SMINE_AGENTS`).
- `--max-proposals-mined <n>`: global cap on new proposals this run; enforced by a deterministic trim stage after the fan-out (drop from the kind with the most new proposals first, within a kind lowest-ranked last-in-array first).
- `--repos <name=path,…>`: the working-repo roster (from the permission config's additionalDirectories; the nightly wrapper assembles it). Passed through verbatim to smine-batch for mechanical repo attribution — and thereby folder routing. Absent → smine-batch keeps transcript inference.
- `--since <YYYY-MM-DD>`: mine only sessions last active on or after this date. Passed through verbatim to smine-batch. Absent → no date floor.
- `--last <n>`: mine only the machine's n most recent qualifying sessions. Passed through verbatim to smine-batch; mutually exclusive with `--since` — when both are given, `--since` wins and `--last` is dropped.
- `--dev`: include the pipeline's own sessions (smine repo, routine branches) in mining. Passed through verbatim to smine-batch; absent → smine-batch excludes them (its default-deny rule).

## 1. Resolve

- Parse the flags into `{nightly, noBatch, skip[], batches[], subagents, maxMinedPerDimension, maxMinedTotal, since, last, dev, agents}`. `skip[]` collects each `--no-<dimension>` as its dimension skill name (mapping above); the two cap flags parse as positive integers (absent → no cap); `subagents` is a bool from `--subagents`.
- `batches[]` is empty unless `--no-batch` is set. With `--no-batch`, resolve the unrouted batches from the ledgers' first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>` (the five dimension ledgers, historical filenames unchanged: `analyzed-memory.txt`, `analyzed-skill.txt`, `analyzed-routines.txt`, `analyzed-rules.txt`, `analyzed-permissions.txt`). Per scope: take the lowest-numbered such line across the five; the target is the next existing batch after it. Every folder under `sessions/` except `archived/` considered — if several have a target, the lowest batch number wins. The line is authoritative for progress: sessions a batch marked skipped or unretrievable stay out of the ledgers deliberately and never re-qualify a batch. Only a ledger missing the line falls back to an oldest-first scan for session IDs absent from it. Interactive `--no-batch`: pass just the single oldest unrouted batch; nightly `--no-batch`: pass every unrouted batch. If no scope has a batch beyond its cursor, report "all batches routed" and stop.

## 2. Run

- Primary path (all `--nightly` / headless runs, and the default whenever in doubt): run the pipeline directly — do not attempt to load the Workflow tool (headless `claude -p` sessions do not expose it), do not ask and do not stop. Unless `--no-batch`, first invoke the Skill tool with skill="smine-batch" and args="--nightly" when set plus the `--repos`/`--since`/`--last`/`--dev`/`--subagents`/`--agents` values when given (else no args), following it exactly and treating its STOP-for-review as finish-and-return; then per batch launch the non-skipped dimension agents (general-purpose) in one message — except smine-memory, launched only after smine-context returns (shared `proposals/context.json`); smine-permissions runs with the parallel set (its own `proposals/permissions.json`) — batches sequential, each prompted to invoke the Skill tool with skill="<dimension>" and args="<batch path>", follow the loaded skill exactly scoped to that batch, work from the repo root, treat STOP-for-review as finish-and-return, and return counts, every file written (including the ledger), and skips/reroutes. With `--max-proposals-per-dimension`, add the cap sentence ("Production cap: add at most N new proposals this run — keep the best-ranked, list every dropped candidate in notes.") to each proposal-dimension prompt (smine-context, smine-memory, smine-permissions, smine-routines, smine-skills); with `--max-proposals-mined`, run the trim yourself after the fan-out per the flag's rule above.
- Manual-run alternative (interactive sessions only, when the Workflow tool is available): call the Workflow tool with `{scriptPath: '<this skill's base directory>/workflows/session-mine.js', args: {nightly, noBatch, skip, batches, maxMinedPerDimension, maxMinedTotal, since, last, dev, subagents, agents}}` — `args` is a real JSON object, never a string; omit a cap key when its flag is absent. The base directory is stated when this skill loads. The workflow encodes the same pipeline: mine first (one smine-batch agent, skipped with `--no-batch`), then per batch the non-skipped dimension agents — skills/routines/context in one parallel barrier, smine-memory after it (shared `proposals/context.json`) — batches sequential (the per-dimension ledgers are shared across batches). All other writes are disjoint (per-dimension ledgers, per-dimension proposal JSON under `proposals/`).

## 3. Report

- Relay per batch, per dimension: extracted vs applied/proposed counts, files written, skips/reroutes, failed dimensions. Note the mined batch count and any skipped dimensions.
- Remind: proposal JSON under `proposals/` awaits review; nothing is committed.
- Interactive mode: STOP after reporting for review. `--nightly`: no stop — loop is unattended.

## Rules

- Never run against a batch smine-batch is still writing.
- A failed dimension is reported, not retried silently; its ledger stays unappended so a re-run picks it up.
- Never ask a confirmation question mid-run — in a headless session a question terminates the run with nothing done; pick the codified path and proceed.

## Model

- Suggested: frontier / high
- Reason: batch resolution + deterministic pipeline fan-out; the mine stage inherits the miner's demands and nightly runs the whole pipeline under this skill
- Tested unviable: — (none yet)
