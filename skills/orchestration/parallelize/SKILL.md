---
name: parallelize
description: Fan one skill across a {model} x {effort} x {arg-variants} x replicas matrix, then consolidate into a grounded base. Trigger on /parallelize or "bake-off" or "matrix run". Args — invocation: skill name, args, shared context doc; models[]: e.g. [opus, fable]; efforts[]: shorthand l/m/h/xh/max; argVariants[]: invocation flavors of the task; replicas: identical runs per cell; base commit: common start commit, clean tree.
author: Kevin Horst
version: 1.10
argument-hint: "[invocation] [models] [efforts] [argVariants] [replicas] [base commit]"
---

# Parallelize (matrix bake-off)

Fan one skill invocation across a dynamic matrix of cells, each in its own isolated worktree from a common base commit, then consolidate. Skill-fronts-Workflow: this skill resolves the invocation and matrix spec, the `parallelize` workflow (`workflows/parallelize.js` in this skill's directory) does the deterministic fan-out and consolidation.

## When to use

**Use when:** fanning one skill invocation — any skill: fdesign (any route), viability verdicts, investigations — across a `{model} × {effort} × {arg-variants} × replicas` matrix from a common base commit, to compare outputs and pick a grounded base. Invoked via /parallelize.
**Don't use when:** a single run is wanted — invoke the skill directly. Reviewing a diff — /railroad-review. Scoring or comparing already-finished runs — /skillroutine-eval. A fresh matrix whose deliverable is per-rule eval scores rather than a merged base — /skillroutine-eval matrix mode (its pipe workflow nests this skill's workflow).
**Preconditions:** clean working tree at the base commit; the invocation has an unattended form (interactive gates bypassed per cell, e.g. `--nightly`) — a cell parked on an approval gate kills the matrix.
**Workflow position:** standalone (see README.md § Skill map, smine repo).

## Args

- invocation: skill name + args + one shared context doc/prompt.
- `models[]`: e.g. `[opus, fable]` (default: session model).
- `efforts[]`: shorthand `l/m/h/xh/max` (default: inherit session effort).
- `argVariants[]`: different invocation flavors of the same task (default: one, the args as given).
- `replicas`: identical runs per cell for variance probing (default 1).
- base commit: common start commit (default current HEAD; tree must be clean). Hard cap: 16 cells.

## 1. Intake

- The invocation under test: skill name + args + one shared context doc/prompt. Any skill, not just planning.
- The matrix spec — a cartesian product of up to four dimensions, any of which may be a singleton:
  - `models[]` — e.g. `[opus, fable]`; default: the session model.
  - `efforts[]` — shorthand l/m/h/xh/max maps to `low`/`medium`/`high`/`xhigh`/`max`; default: inherit session effort.
  - `argVariants[]` — first-class input fuzzing: each variant is a different invocation flavor of the same task (e.g. `[ad-hoc, serena]`); default: one variant, the args as given.
  - `replicas` — N identical runs per cell (variance probing); default 1.
- Base commit: default current HEAD. The working tree must be clean and on it — every cell starts from the identical premise.
- **Hard cap: 16 cells.** Refuse a larger matrix and say which dimension to shrink — never silently truncate.
- Verify the invocation is unattended-safe; if the skill has an interactive gate, require its unattended mode in the args before running.

## 2. Run

- Call the Workflow tool: `{scriptPath: '<this skill's base directory>/workflows/parallelize.js', args: {skill, skillArgs, contextDoc, models, efforts, argVariants, replicas, baseCommit}}` — the base directory is stated when this skill loads; args is a real JSON object, never a string.
- Stage A runs one agent per cell in its own reset worktree with the cell's model/effort applied and the arg-variant substituted; Stage B disqualifies contaminated or baseline-mismatched cells; Stage C is a single synthesizer producing the decision-point matrix, shared blind spots, grounded base, and cross-graft plan.

## 3. Report & record

- Relay the consolidation: decision-point comparison matrix, divergences, shared blind spots (claims all runs make that none verified), the grounded base and why it won, the cross-graft plan (base + losers' genuinely-good deltas as a small patch), disqualified and failed cells.
- Append the verdict — matrix spec, winning cell, why — to `sessions/parallelize/verdicts.md` so bake-off outcomes accumulate as data on model/effort/arg-variant fit per skill. Main session writes this, never a workflow agent.

## Rules

- A cell that read a sibling worktree or branch is contaminated: disqualified, never averaged in.
- Stage C actively ingests every surviving artifact — sibling outputs are never assumed to have cross-pollinated.
- Judgment/viability cells must not route through plan mode/ExitPlanMode — the findings message is the deliverable.
- One invocation per workflow run.

## Model

- Suggested: mid-tier / medium
- Reason: intake + deterministic workflow fan-out; the heavy lifting happens in the cells and the synthesizer
- Tested unviable: — (none yet)
