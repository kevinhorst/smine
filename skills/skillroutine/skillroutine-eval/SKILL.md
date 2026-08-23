---
name: skillroutine-eval
description: Score skill runs against the skill's own rules, emitting schema-conformant JSON. Trigger on /skillroutine-eval [manifest|matrix] or "evaluate, score, or compare skill runs" or "eval a skill across models/efforts". Args — manifest: JSON eval manifest path, inline args for one quick run; matrix: generate a fresh sandboxed matrix, then score it.
author: Kevin Horst
version: 2.5
argument-hint: "[manifest] [matrix]"
allowed-tools: Read, Write, Edit, Glob, Grep, Skill(jq), Bash(jq *), Bash(go run ./cmd/rules *), Bash(bash cmd/context/context_record.sh *), Bash(git diff *), Bash(git log *), Bash(make audit *), Bash(wc *), Bash(grep *), Bash(ls *)
---

# Skillroutine eval (skill runs → scored JSON)

Score n runs of the same skill on three axes — **self** (the skill's own entry ids), **context** (the context entry ids the run received), **output** (quality rules from supplied context files plus numeric metrics). Output is JSON conforming to [reference/schema.json](reference/schema.json) (v2) — the schema is the contract; change it only deliberately. Read [reference/manifest.schema.json](reference/manifest.schema.json) when creating or checking a manifest. Invoke `/jq` to validate the manifest and output after writing them. Exemplar for md rendering: `examples/fdesign/correctness/feature-design-eval.md`.

## When to use

**Use when:** evaluating, scoring, or comparing one or more runs of a skill against its own rules — model bake-offs, skill regression checks, output-quality measurement. Invoked via /skillroutine-eval [manifest]. Matrix mode: the runs don't exist yet and a fresh sandboxed matrix should be generated and scored in one shot — /skillroutine-eval matrix.
**Don't use when:** extracting skill proposals from batch reports — /smine-skills. Creating or editing a skill — /skillroutine-create. Reviewing code changes — /railroad-review or /code-review. A bake-off whose deliverable is a merged grounded base rather than scores — /parallelize.
**Preconditions:** the skill's SKILL.md, one or more run output files, and (for structured eval) a manifest JSON. Matrix mode instead: a matrix spec, clean working tree on the base commit, and an unattended-safe invocation of the evaluated skill.
**Workflow position:** standalone (see README.md § Skill map, smine repo).

## Args

- manifest: positional — path to a JSON eval manifest; inline args accepted for a quick single-run eval, anything beyond one run + defaults needs the manifest.
- `matrix`: mode token — generate a fresh sandboxed `{models}×{efforts}×{argVariants}×replicas` matrix, then score it; intake then takes the parallelize matrix spec plus `contextFiles[]` and `qualityContext[]`.

## 0. Intake

- Arg: path to an eval manifest (JSON). Inline args accepted for a quick single-run eval; anything beyond one run + defaults needs the manifest.
- Manifest fields:
  - `skill` — name of the evaluated skill; `skillMd` — path to its source SKILL.md.
  - `runs[].skillMd` — the rendered SKILL.md the run loaded (a variant strips entries); `runs[].variant` — `{name, disable[]}` when not the full skill; `runs[].transcript` — session JSONL path or peek id; `runs[].worktree` — for probe metrics.
  - `metrics[]` — per-skill metrics added to the generic set (`audit_pass`, `diff_lines`, `files_touched`, `wall_s`, `output_tokens`, `entry_citations`).
  - `contextFiles[]` — format docs the skill binds to (e.g. rules/plan.md); rubric sources alongside SKILL.md.
  - `qualityContext[]` — style guides / definition-of-done judging the output's content (e.g. `context/go/*.md`). Empty or absent ⇒ quality axis is N/A, stated once, never stalled on.
  - `inputs[]` — the skill-input artifacts every run received (concept, exploration, args).
  - `runs[]` — per run: `id`, `output` path, `model { id, effort, mode, telemetry }`, optional `userFindings` (path or inline notes with the user's own read; integrated cells get source `user`).
  - `output` — where the JSON lands; default `eval.json` next to the run outputs. `md` — optional boolean to render markdown.
- Invoke `/jq` to parse the manifest before reading files; validate its required fields against `reference/manifest.schema.json`. Verify every listed file exists; a missing file is reported and its cells become 0 "not demonstrable" — never guessed around.

## 1. Derive rubric

- **self** rows: `go run ./cmd/rules render-skill --list-entries <runs[].skillMd>` (in claude-configs; `bin/rules` when vendored) — one row per non-payload entry, id verbatim, `rule` = statement (+ Why/Applies), `source` = `<skillMd>:<line>` (`line` is 1-based). Runs with different variants have different row sets; the shared set is their intersection.
- **context** rows: with `runs[].transcript`, `bash <claude-configs>/cmd/context/context_record.sh <transcript> <ctx-dir>` → `injected.skill[] | select(.skill == <skill or skill--variant>).ids`; without a transcript, resolve the skill's frontmatter `context:` line against the run's `<ctx-dir>/context.json` (the same jq as `skill-context.sh`) and state once that rows are declaration-derived. `rule` = the entry's `content.statement` from that index.
- **output** rows: `C<n>` from `qualityContext` as before; plus the metrics table (generic set + `metrics[]`).
- Present the counts per axis, then **freeze** — the same rows apply identically to every run; no rows added or reworded after the first score.

## 2. Mechanical probes

Deterministic checks run before any judgment, recorded as `probes[]` and cited as evidence in the cells they feed:

1. **Contradiction audit** — diff the binding inputs against each other; each real contradiction becomes a probe result; runs are scored on whether they surfaced it.
2. **Dependency reality check** — for every dependency a run's output pins or assumes, read the dep's own manifest (go.mod etc.); mismatch is a probe result.
3. **Assumptions novelty ratio** — per run: assumption rows novel vs recycled from the provided inputs.
4. **Diff-block count vs modified-file count** — per run: a plan modifying existing files with zero diff blocks is a mechanical red flag.
5. **Quality greps** — cheap probes over in-output code: `err.Error()` to client, per-request rescans, comment density. Extend the list from whatever the quality context flags as high-signal.
6. **Metrics** — per run: `audit_pass` (`make audit` exit in `runs[].worktree` when present, else null + note), `diff_lines` / `files_touched` (`git diff --numstat <base>..HEAD` in the worktree, else null), `wall_s` / `output_tokens` (from `model.telemetry` when it carries them, else null), `entry_citations` (count of `SKILL-[A-Z0-9-]+-[0-9]{3}` mentions in the output and transcript); manifest `metrics[]` with a `command` are run in the worktree, stdout parsed as number/bool. Values never estimated: null with a note.

## 3. Score

- Per run × rule: **+1** followed, **0** not followed or not demonstrable, **−1** actively violated.
- Metrics are recorded as `metricValues[]`, ranked per metric by `direction`, never folded into rule totals.
- Every non-+1 cell carries a justification with evidence — verbatim quote or file:line anchor — and a `source`: `agent` (own read), `probe` (mechanical), `user` (from `userFindings`).
- Not demonstrable = 0 and says so: unverifiable quality is itself a defect of the output, never a reason to skip the cell.

## 4. Emit

- Invoke `/jq` on the JSON output: parse it; verify every run × (its variant's) rubric rule has exactly one score cell; verify score references; require justification, evidence, and `source` for every non-+1 cell; recompute `totals` per axis from cells and, when variants differ, `sharedTotals` over the intersection. Fix and re-validate until clean.
- `md` set ⇒ render the markdown: one rule table per axis, the metrics table, non-+1 justification tables, a per-axis ranking, and — with variants — a delta row `variant − default` per axis over the shared rows and a separate table of variant-only rows.
- Report per-axis totals (raw + pct) and the ranking, then STOP for user review.

## Matrix mode (skillroutine-parallel-eval pipe)

Generates the runs, then scores them — a pipe workflow (`workflows/parallel-eval.js` in this skill's directory) that nests the parallelize workflow for the fan-out and hands the surviving artifacts to this skill's normal eval flow. Composition doctrine: README.md § Skill map in the smine repo (Composition section). Matrix mode needs the Workflow tool (interactive sessions only); headless A/B runs use the `skill-eval` routine (`routines/skill-eval/`, cells are real `claude -p` sessions via `routines/_lib/matrix.sh`), which writes the same v2 manifest and then invokes this skill in manifest mode.

1. **Intake** — the invocation under test (skill + args + optional shared context doc) and the matrix spec, exactly as in the parallelize skill's intake: `models[]`, `efforts[]` (l/m/h/xh/max shorthand), `argVariants[]`, `replicas`, base commit defaulting to HEAD, hard cap 16 cells, clean tree, unattended-safe invocation. Plus this skill's eval context: `contextFiles[]`, `qualityContext[]`. Plus `skillVariants[]`: `[{name, disable[]}]` — each variant is rendered as the skill `<leaf>--<name>` (home scope, `sync_skills.sh --variant`) before fan-out and removed after the eval; the full skill is the implicit `default`. Cells = models × efforts × argVariants × replicas × (1 + variants), hard cap 16.
2. **Resolve paths** (the pipe script has no filesystem access — paths always travel in args):
   - `parallelizeScript` — the parallelize skill's `workflows/parallelize.js`; the deployed skills root is this skill's base directory's parent, so the sibling `parallelize/workflows/parallelize.js`.
   - `skillMd` — the evaluated skill's SKILL.md, same resolution.
3. **Run** — call the Workflow tool: `{scriptPath: '<this skill's base directory>/workflows/parallel-eval.js', args: {parallelizeScript, syncSkillsScript, skill, skillArgs, contextDoc, models, efforts, argVariants, replicas, baseCommit, skillMd, contextFiles, qualityContext, skillVariants, resultsDir?}}` — args is a real JSON object, never a string. `syncSkillsScript` is `<claude-configs>/cmd/sync/sync_skills.sh`. `resultsDir` defaults to `evals/<skill>-<base6>/`. The pipe renders the variants, fans out via the nested parallelize workflow once per skill name (consolidation included — its grounded base/cross-graft plan is complementary to the scores), copies surviving artifacts and the rendered SKILL.md files out of the ephemeral cell worktrees, writes the v2 manifest, runs this skill unattended inside an agent, and removes the variant dirs.
4. **Report & record** — relay the ranking with per-axis totals, the consolidation grounded base + cross-graft plan, and disqualified/failed/missing cells; point at `eval.json` / `eval.md` under the results dir. Append the verdict — matrix spec, eval winner, why — to `sessions/parallelize/verdicts.md` (main session writes this, never a workflow agent — bake-off outcomes accumulate in one place).

## Rules

- Axes never merged: self, context, and output report separately; metrics never enter a rule total.
- Self rows come from the SKILL.md the run loaded, never from the source when a variant was run.
- Rubric frozen before the first score; a mid-scoring rubric change restarts scoring for all runs.
- Nothing invented: a cell without evidence is 0 "not demonstrable", never guessed; quotes verbatim.
- Totals are computed from the cells, never hand-summed.

## Model

- Suggested: frontier / high
- Reason: judgment-heavy rubric derivation and per-cell scoring
- Tested unviable: — (none yet)
