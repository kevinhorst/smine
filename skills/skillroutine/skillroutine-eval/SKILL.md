---
name: skillroutine-eval
description: Score skill runs against the skill's own rules, emitting schema-conformant JSON. Trigger on /skillroutine-eval [manifest|matrix] or "evaluate, score, or compare skill runs" or "eval a skill across models/efforts". Args — manifest: JSON eval manifest path, inline args for one quick run; matrix: generate a fresh sandboxed matrix, then score it.
author: Kevin Horst
version: 1.7
---

# Skillroutine eval (skill runs → scored JSON)

Score n runs of the same skill against a rubric derived from that skill's own SKILL.md, plus an output-quality axis from supplied context files. Output is JSON conforming to [reference/schema.json](reference/schema.json) — the schema is the contract; change it only deliberately. Read [reference/manifest.schema.json](reference/manifest.schema.json) when creating or checking a manifest. Invoke `/jq` to validate the manifest and output after writing them. Exemplar for md rendering: `examples/fdesign/models/eval.md`.

## When to use

**Use when:** evaluating, scoring, or comparing one or more runs of a skill against its own rules — model bake-offs, skill regression checks, output-quality measurement. Invoked via /skillroutine-eval [manifest]. Matrix mode: the runs don't exist yet and a fresh sandboxed matrix should be generated and scored in one shot — /skillroutine-eval matrix.
**Don't use when:** extracting skill proposals from batch reports — /smine-skills. Creating or editing a skill — /skillroutine-create. Reviewing code changes — /railroad-review or /code-review. A bake-off whose deliverable is a merged grounded base rather than scores — /parallelize.
**Preconditions:** the skill's SKILL.md, one or more run output files, and (for structured eval) a manifest JSON. Matrix mode instead: a matrix spec, clean working tree on the base commit, and an unattended-safe invocation of the evaluated skill.
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo).

## Args

- manifest: positional — path to a JSON eval manifest; inline args accepted for a quick single-run eval, anything beyond one run + defaults needs the manifest.
- `matrix`: mode token — generate a fresh sandboxed `{models}×{efforts}×{argVariants}×replicas` matrix, then score it; intake then takes the parallelize matrix spec plus `contextFiles[]` and `qualityContext[]`.

## 0. Intake

- Arg: path to an eval manifest (JSON). Inline args accepted for a quick single-run eval; anything beyond one run + defaults needs the manifest.
- Manifest fields:
  - `skill` — name of the evaluated skill; `skillMd` — path to its SKILL.md.
  - `contextFiles[]` — format docs the skill binds to (e.g. style/plan.md); rubric sources alongside SKILL.md.
  - `qualityContext[]` — style guides / definition-of-done judging the output's content (e.g. `context/go/*.md`). Empty or absent ⇒ quality axis is N/A, stated once, never stalled on.
  - `inputs[]` — the skill-input artifacts every run received (concept, exploration, args).
  - `runs[]` — per run: `id`, `output` path, `model { id, effort, mode, telemetry }`, optional `userFindings` (path or inline notes with the user's own read; integrated cells get source `user`).
  - `output` — where the JSON lands; default `eval.json` next to the run outputs. `md` — optional boolean to render markdown.
- Invoke `/jq` to parse the manifest before reading files; validate its required fields against `reference/manifest.schema.json`. Verify every listed file exists; a missing file is reported and its cells become 0 "not demonstrable" — never guessed around.

## 1. Derive rubric

- Read `skillMd` + `contextFiles`; extract the compliance rules: one testable behavior per row, phase-tagged, each citing the source line it derives from. Read `qualityContext` the same way for the quality rules.
- Rubric IDs: `S<n>` compliance, `C<n>` quality. Present the derived count per axis, then **freeze** — the same rubric applies identically to every run; no rows added or reworded after the first score.

## 2. Mechanical probes

Deterministic checks run before any judgment, recorded as `probes[]` and cited as evidence in the cells they feed:

1. **Contradiction audit** — diff the binding inputs against each other; each real contradiction becomes a probe result; runs are scored on whether they surfaced it.
2. **Dependency reality check** — for every dependency a run's output pins or assumes, read the dep's own manifest (go.mod etc.); mismatch is a probe result.
3. **Assumptions novelty ratio** — per run: assumption rows novel vs recycled from the provided inputs.
4. **Diff-block count vs modified-file count** — per run: a plan modifying existing files with zero diff blocks is a mechanical red flag.
5. **Quality greps** — cheap probes over in-output code: `err.Error()` to client, per-request rescans, comment density. Extend the list from whatever the quality context flags as high-signal.

## 3. Score

- Per run × rule: **+1** followed, **0** not followed or not demonstrable, **−1** actively violated.
- Every non-+1 cell carries a justification with evidence — verbatim quote or file:line anchor — and a `source`: `agent` (own read), `probe` (mechanical), `user` (from `userFindings`).
- Not demonstrable = 0 and says so: unverifiable quality is itself a defect of the output, never a reason to skip the cell.

## 4. Emit

- Invoke `/jq` on the JSON output: parse it; verify every run × rubric rule has exactly one score cell; verify score references; require justification, evidence, and `source` for every non-+1 cell; and recompute each compliance/quality total from cells. Fix and re-validate until clean.
- `md` set ⇒ additionally render the eval.md-shaped markdown from the same data (rule table per axis, non-+1 justification tables, combined ranking) next to the JSON. Markdown is derived from validated JSON; never maintain independent scores.
- Report per-axis totals (raw + pct) and the ranking, then STOP for user review.

## Matrix mode (skillroutine-parallel-eval pipe)

Generates the runs, then scores them — a pipe workflow (`workflows/parallel-eval.js` in this skill's directory) that nests the parallelize workflow for the fan-out and hands the surviving artifacts to this skill's normal eval flow. Composition doctrine: `docs/skill-map.md` in the smine repo (Composition section).

1. **Intake** — the invocation under test (skill + args + optional shared context doc) and the matrix spec, exactly as in the parallelize skill's intake: `models[]`, `efforts[]` (l/m/h/xh/max shorthand), `argVariants[]`, `replicas`, base commit defaulting to HEAD, hard cap 16 cells, clean tree, unattended-safe invocation. Plus this skill's eval context: `contextFiles[]`, `qualityContext[]`.
2. **Resolve paths** (the pipe script has no filesystem access — paths always travel in args):
   - `parallelizeScript` — the parallelize skill's `workflows/parallelize.js`; the deployed skills root is this skill's base directory's parent, so the sibling `parallelize/workflows/parallelize.js`.
   - `skillMd` — the evaluated skill's SKILL.md, same resolution.
3. **Run** — call the Workflow tool: `{scriptPath: '<this skill's base directory>/workflows/parallel-eval.js', args: {parallelizeScript, skill, skillArgs, contextDoc, models, efforts, argVariants, replicas, baseCommit, skillMd, contextFiles, qualityContext, resultsDir?}}` — args is a real JSON object, never a string. `resultsDir` defaults to `sessions/skillroutine-eval/<skill>-<base6>/`. The pipe fans out via the nested parallelize workflow (consolidation included — its grounded base/cross-graft plan is complementary to the scores), copies surviving artifacts out of the ephemeral cell worktrees, writes the manifest, and runs this skill unattended inside an agent.
4. **Report & record** — relay the ranking with per-axis totals, the consolidation grounded base + cross-graft plan, and disqualified/failed/missing cells; point at `eval.json` / `eval.md` under the results dir. Append the verdict — matrix spec, eval winner, why — to `sessions/parallelize/verdicts.md` (main session writes this, never a workflow agent — bake-off outcomes accumulate in one place).

## Rules

- Axes never merged: compliance and quality report separately; they anti-correlate at the extremes, a single headline number misleads.
- Rubric frozen before the first score; a mid-scoring rubric change restarts scoring for all runs.
- Nothing invented: a cell without evidence is 0 "not demonstrable", never guessed; quotes verbatim.
- Totals are computed from the cells, never hand-summed.

## Model

- Suggested: frontier / high
- Reason: judgment-heavy rubric derivation and per-cell scoring
- Tested unviable: — (none yet)

## Changelog

- v1.7 (2026-07-31): contextFiles example updated to style/plan.md
- v1.6 (2026-07-27): renamed couchskill-eval → skillroutine-eval; workflow couchskill-parallel-eval → skillroutine-parallel-eval; matrix resultsDir default sessions/skillroutine-eval/; sibling handoffs → skillroutine-create, smine-skills; effort token normalized (large → high)
- v1.5 (2026-07-26): Args section
- v1.4 (2026-07-19): matrix mode — couchskill-parallel-eval pipe workflow nests parallelize fan-out then evals the surviving runs
- v1.3 (2026-07-19): renamed eval-skill → couchskill-eval; moved under skills/couchskill/
- v1.2 (2026-07-16): skill authoring delegates to /couchskill-create
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-13): initial version
