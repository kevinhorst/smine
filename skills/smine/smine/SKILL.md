---
name: smine
description: Run the full session-mining pipeline — mine transcripts into batch reports, then fan each batch to all six dimension skills. Trigger on /smine or "analyze sessions" or "run a session retrospective". Args — --nightly: unattended, all pending batches; --no-batch: route existing unrouted batches only; --no-summary/--no-memory/--no-routines/--no-style/--no-skills/--no-workflows: skip that dimension; --max-proposals-per-dimension n / --max-proposals-mined n: production caps.
author: Kevin Horst
version: 1.2
enable-model-invocation: true
---

# smine (pipeline)

Mine past session transcripts, then route every batch to its dimension skills — one entrypoint for the whole retrospective. Skill-fronts-Workflow: this skill resolves the flags and (in `--no-batch`) the target batches; the `session-mine` workflow (`workflows/session-mine.js` in this skill's directory) does the deterministic mine-then-route.

## When to use

**Use when:** mining past sessions end-to-end (mine + route), or routing already-mined batches that were never fanned out (`--no-batch`). Invoked via /smine.
**Don't use when:** a quick look at what a session did — /peek. Only one dimension on one batch — invoke that dimension skill directly (e.g. /smine-memory, /smine-style). Just the raw transcript miner without routing — /smine-batch. Evaluating a skill's runs — /skillroutine-eval.
**Preconditions:** peek-mcp available for the mine stage (not needed with `--no-batch`); no batch currently being written.
**Workflow position:** **smine** = smine-batch → batch report → dimension fan-out (smine-memory, smine-skills, smine-workflows, smine-routines, smine-style, smine-summary) → proposals / memory / JSON (see `docs/skill-map.md`, smine repo).

## Args

- `--nightly`: unattended mode — mine all pending batches (headless), route each, no stop for review. For the launchd routine.
- `--no-batch`: skip the mine stage; route the existing unrouted batches resolved from the ledger cursor (§1). For retro-analysis of already-mined batches.
- `--no-summary` / `--no-memory` / `--no-routines` / `--no-style` / `--no-skills` / `--no-workflows`: skip that one dimension in the fan-out. Flag → dimension skill:
  - `--no-memory` → smine-memory
  - `--no-skills` → smine-skills
  - `--no-workflows` → smine-workflows
  - `--no-routines` → smine-routines
  - `--no-style` → smine-style
  - `--no-summary` → smine-summary
- `--max-proposals-per-dimension <n>`: cap each proposal-producing dimension (style, routines, skills, workflows) at n new proposals this run; passed into each dimension invocation.
- `--max-proposals-mined <n>`: global cap on new proposals this run; enforced by a deterministic trim stage after the fan-out (drop from the kind with the most new proposals first, within a kind lowest-ranked last-in-array first).

## 1. Resolve

- Parse the flags into `{nightly, noBatch, skip[], batches[], maxMinedPerDimension, maxMinedTotal}`. `skip[]` collects each `--no-<dimension>` as its dimension skill name (mapping above); the two cap flags parse as positive integers (absent → no cap).
- `batches[]` is empty unless `--no-batch` is set. With `--no-batch`, resolve the unrouted batches from the ledgers' first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>` (the six ledgers, historical filenames unchanged: `analyzed-memory.txt`, `analyzed-skill.txt`, `analyzed-workflow.txt`, `analyzed-routines.txt`, `analyzed-rules.txt`, `analyzed-summarize.txt`). Per scope: take the lowest-numbered such line across the six; the target is the next existing batch after it. Both scopes (`sessions/{personal,work}/`) considered — if both have a target, the lower batch number wins. The line is authoritative for progress: sessions a batch marked skipped or unretrievable stay out of the ledgers deliberately and never re-qualify a batch. Only a ledger missing the line falls back to an oldest-first scan for session IDs absent from it. Interactive `--no-batch`: pass just the single oldest unrouted batch; nightly `--no-batch`: pass every unrouted batch. If no scope has a batch beyond its cursor, report "all batches routed" and stop.

## 2. Run

- Call the Workflow tool: `{scriptPath: '<this skill's base directory>/workflows/session-mine.js', args: {nightly, noBatch, skip, batches, maxMinedPerDimension, maxMinedTotal}}` — `args` is a real JSON object, never a string; omit a cap key when its flag is absent. The base directory is stated when this skill loads.
- The workflow mines first (one smine-batch agent, skipped with `--no-batch`), then routes each batch through the non-skipped dimension agents in one parallel barrier per batch, batches sequential (the per-dimension ledgers are shared across batches). Writes are disjoint (per-dimension ledgers, per-dimension proposal JSON under `sessions/proposals/`, memory) — no coordination beyond the sequential-batch ordering.
- Headless fallback: if the Workflow tool is unavailable (headless `claude -p` sessions do not expose it), do not ask and do not stop — replicate the pipeline yourself: unless `--no-batch`, first invoke the Skill tool with skill="smine-batch" and args="--nightly" when set (else no args), following it exactly and treating its STOP-for-review as finish-and-return; then per batch launch the non-skipped dimension agents (general-purpose) in one message, batches sequential, each prompted to invoke the Skill tool with skill="<dimension>" and args="<batch path>", follow the loaded skill exactly scoped to that batch, work from the repo root, treat STOP-for-review as finish-and-return, and return counts, every file written (including the ledger), and skips/reroutes. With `--max-proposals-per-dimension`, add the cap sentence ("Production cap: add at most N new proposals this run — keep the best-ranked, list every dropped candidate in notes.") to each proposal-dimension prompt (smine-style, smine-routines, smine-skills, smine-workflows); with `--max-proposals-mined`, run the trim yourself after the fan-out per the flag's rule above.

## 3. Report

- Relay per batch, per dimension: extracted vs applied/proposed counts, files written, skips/reroutes, failed dimensions. Note the mined batch count and any skipped dimensions.
- Remind: proposal JSON under `sessions/proposals/` and memory changes await review; nothing is committed.
- Interactive mode: STOP after reporting for review. `--nightly`: no stop — loop is unattended.

## Rules

- Never run against a batch smine-batch is still writing.
- A failed dimension is reported, not retried silently; its ledger stays unappended so a re-run picks it up.
- Never ask a confirmation question mid-run — in a headless session a question terminates the run with nothing done; pick the codified path and proceed.

## Model

- Suggested: frontier / high
- Reason: batch resolution + deterministic pipeline fan-out; the mine stage inherits the miner's demands and nightly runs the whole pipeline under this skill
- Tested unviable: — (none yet)

## Changelog

- v1.2 (2026-07-31): session-scope rename to personal/work (sessions/{personal,work} globs)
- v1.1 (2026-07-30): smine-rules → smine-style (--no-rules → --no-style); production caps --max-proposals-per-dimension / --max-proposals-mined threaded into the workflow plus a deterministic trim stage
- v1.0 (2026-07-27): merges the analyze v1.6 pipeline front and the /smine entrypoint — /smine now runs mine (smine-batch) + six-dimension fan-out via the session-mine workflow; adds --no-batch and per-dimension --no-* flags; analyze skill and analyze.js retired
