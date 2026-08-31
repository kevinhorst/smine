---
name: smine-batch
description: Mine past session transcripts into a batch report plus its schema-conformant JSON — the raw mining stage of the /smine pipeline. Trigger on /smine-batch or "mine session transcripts". Args — --nightly: all pending batches; --repos name=path,…: repo attribution/routing; --since/--last: date floor / newest-n cap; --dev: include smine/routine sessions; --subagents: also mine subagent transcripts (expensive).
author: Kevin Horst
version: 1.28
argument-hint: "[--nightly] [--agents claude,codex] [--repos name=path,...] [--since YYYY-MM-DD] [--last n] [--dev] [--subagents]"
allowed-tools: mcp__Peek_MCP__session_list, mcp__Peek_MCP__session_events, mcp__Peek_MCP__session_full, mcp__Peek_MCP__session_get, Task, ToolSearch, Read, Write, Bash(jq *), Bash(cmd/context/context_record.sh *)
---

# smine-batch

Sweep stored session transcripts and distill actionable items into a batch report **and its schema-conformant JSON** — stage 1 of the /smine pipeline. The consumer of the report is usually another agent (the /smine fan-out) — artifacts must be self-contained and injectable as-is; the JSON's consumer is the session-overview server.

## When to use

**Use when:** mining past sessions for actionable items — skill candidates, workflow patterns, memory updates, rule drift, harness friction — into a batch report. Trigger on /smine-batch, "mine session transcripts", "review transcripts", or "extract lessons".
**Don't use when:** running the whole retrospective (mine + route) — /smine. Quick-peeking the latest session — /peek. Routing an existing batch report into dimension proposals — /smine (or a single dimension skill directly). Evaluating a specific skill's runs — /skillroutine-eval.
**Preconditions:** peek-mcp available (session_list, session_full tools).
**Workflow position:** **smine-batch** → batch report + batch JSON → /smine fan-out → dimension skills (see README.md § Skill map, smine repo).

## Args

- `--agents <claude,codex>`: agents whose sessions are mined; absent → both. Anything outside `{claude,codex}` is an arg error — state it and stop.
- `--nightly`: unattended mode (headless `claude -p` routine) — process **all** pending batches sequentially; after each batch write the report and append the ledger as usual, but do not stop for review; end when no unanalyzed sessions remain.
- `--repos <name=path,…>`: the working-repo roster (permission-config additionalDirectories; assembled by the nightly wrapper). Enables mechanical repo attribution: resolve each session's cwd by longest-prefix match against the paths — the `[Repo, Type]` title tag carries the matching name; no match → `[external]` (external sessions are still mined, never skipped). Absent → keep transcript inference.
- `--since <YYYY-MM-DD>`: only sessions whose `last_active` (from `session_list`) is on or after this date qualify; older sessions are excluded before batching — never listed as skipped.
- `--last <n>`: keep only the n newest qualifying sessions — applied at Select after the merge and every exclusion; the rest are dropped silently, never listed as skipped.
- `--dev`: include the pipeline's own sessions — smine-repo and routine sessions — in mining; without it they are excluded at Select (default-deny, see §1).
- `--subagents`: also mine each session's per-subagent transcripts (§2, Subagent mining). Off by default — it costs one extra paginated `session_get`/`session_events` pull per subagent. Passed through from `/smine --subagents` (nightly: `SMINE_SUBAGENTS=1`).

## 0. Setup

- Data source: peek-mcp — `mcp__Peek_MCP__session_list`, `mcp__Peek_MCP__session_events`, `mcp__Peek_MCP__session_full` (paginate via `request_id` until `has_more=false`). Subagents must load these tools via ToolSearch first. Used-context record: `cmd/context/context_record.sh <transcript.jsonl> [ctx-dir]` (repo root), transcript path = `~/.claude/projects/<slug>/<id>.jsonl` where `<slug>` is the session cwd (from `session_list` meta) with every `/` and `.` replaced by `-`; Claude sessions only.
- Output routing: one folder per session, derived from `--repos` attribution — the matched roster name, or `default` when no roster entry matches (or no roster is given). Folder = `sessions/<name>/`, created on first use; `archived` and `default` are reserved names (`archived/` is never a mining target). Per-batch JSON to `sessions/<name>/json/<batch-stem>.json` (§4), against `reference/schema.json`; `.batch.scope` carries the folder name.
- **Language.** Read `~/.claude/context/global/presentation-profile.md` before writing output; when its `language:` is set and not `en`, author the batch JSON's user-visible prose fields — batch title, theme, arc summaries, per-session titles/summaries/notes, finding titles and summaries — in that language, following the profile body's register and glossary. Never translate: ids, dates, tags, file paths, code, quotes (verbatim evidence), schema keys and dimension names. The `.md` batch report stays English (operator artifact). Absent profile = English, unchanged.
- Ledger: `sessions/<name>/analyzed-sessions.txt`, one full session ID per line, appended after each batch. The skip-list consulted at Select is the union of every `sessions/*/analyzed-sessions.txt` and `sessions/archived/*/analyzed-sessions.txt` — a session mined into any folder, live or archived, never re-mines.
- Gate verdicts: `~/.claude/acdsl/verdicts.jsonl` (JSONL, best-effort — absent file means no gate data, never a stop). Join key is `branch` == the session's git branch from `session_list` meta (`session` is unpopulated); extract with `jq`.

## 1. Select

- `session_list` once per agent in `--agents` (`agent: claude` / `agent: codex`). Exclude the current session. Merge newest → oldest across agents.
- **Self-mining exclusion (default-deny).** Without `--dev`: drop every session whose cwd (from `session_list` meta) resolves inside the run's repo root — including `.claude/worktrees/` — and every session whose `git_branch` starts with `claude-routines/`. These are the pipeline's own runs; mining them is self-referential waste. Dropped sessions are never listed per-ID — the report states one line, `excluded N smine/routine sessions (dev mode off)`. With `--dev` they qualify normally. The flag's absence excludes, so a wrapper that forgets it cannot self-mine.
- With `--since`, drop sessions whose `last_active` is before the date, then order newest → oldest.
- With `--last <n>`, keep only the first n of the merged newest → oldest order (after the exclusions above); drop the rest silently.
- Group the qualifying sessions by target folder (attribution above); a batch never spans folders. Batch size: 10 per folder (or user-given); batch numbers continue per folder (a fresh folder starts at 01).

## 2. Analyze (parallel subagents, 2–3 sessions each)

Each subagent first calls `session_events` for its sessions (with the session's `agent`) — the typed event stream (permission denials/grants, permission-mode changes, plan rejections/revisions, skill invocations, subagent results, user answers) plus counters, usage, and a telemetry-derived `permissions` block (auto-allowed vs prompted-once vs prompted-always vs rejected, with the prompted commands) is the cheap structured signal that steers the transcript read: high denial/rejection counts flag friction before a single turn is read. Then, for Claude sessions, run the used-context record script on the session's transcript JSONL and keep the record in hand — every rule violation seen while reading is checked against it: was the rule present, through which channel. Codex sessions have no record (`context: null`). Then read the FULL transcript via `session_full`. For oversized sessions, spawn a dedicated subagent. Extract per session:

- **Skill candidates**: recurring procedure whose value is judgment and convention — phases, review lenses, question protocols — one agent conversing with the user. Validated dispatch/prompt templates count (frozen-plan implement prompt, task-chip format with repro + test list, consolidation merge instruction). Check the skill inventory first: an instance of an existing skill is report-card input, not a candidate.
- **Workflow candidates**: recurring orchestration whose value is deterministic control flow — fan-out over an enumerable item list, staged find→verify→consolidate pipelines, loop-until-dry — runnable unattended. Multi-session fan-outs the user drove by hand (parallel worktree reviews, bake-offs, consolidation passes) count as evidence.
- **Existing-skill report card**: when a session invokes a known skill, grade the invocation — stale paths, phase gaps, protocol misses → skill-improvement candidates.
- **Feature-design signal**: what the agent built wrong vs. what the user corrected it to. Reconstruct the before/after concretely enough to generalize.
- **Memory candidates**: things the agent should have known. Generalize; skip repo trivia that CLAUDE.md already covers.
- **Repo-surface rules + doc drift**: corrections that belong in enforcement surfaces (context/rules `RULE-*` guides incl. plan.md, context/actions `ACTION` activity chapters incl. reviewing.md, AGENTS.md) — name the target surface. Session evidence contradicting checked-in docs — quote the contradiction.
- **Harness/config friction**: recurring permission prompts → allowlist candidates; hook opportunities; worktree/infra defects; MCP capability gaps. Output is settings/hook/tooling edits, not memory.
- **Permissions** (always captured, feeds the JSON `permissions` block for smine-permissions): from the `session_events` stream and telemetry `permissions` view already in hand, record per session `{tool, command, decision}` for each **prompted-and-granted** tool/command (`prompted_once`/`prompted_always` from the telemetry view, else `granted_event` from a `permission_granted` event) and `{tool, command}` for each **denied** one. The md report gets a `#### Permissions` subsection listing the granted pairs and denial count; omit the subsection when the session had no permission activity. Denials are negative evidence only — never a deny/ask candidate. This is capture, not proposal-authoring (smine-permissions ranks them).
- **Nightly routine candidates**: recurring maintenance the user triggers manually on a cadence with no decisions in the loop (analysis batches, worktree cleanup, settings hygiene, ledger upkeep) → scheduled local routine. Time-driven, not on-demand — a routine typically wraps a Workflow or skill in a schedule.
- **Exemplar validation**: friction-free sessions that confirm a pattern or skill works — flag as validation, so working patterns don't read as absence of signal.
- **Frustration index**: cursing and corrections are a 100% interest flag. Quote verbatim, name the trigger.
- **Positive index**: explicit praise or expressed satisfaction — quote verbatim, name the trigger. The green counterpart of the frustration index; session-level user sentiment, distinct from exemplar validation (pattern-level).
- **Context used** (Claude sessions): one report line per session — `Context used: skill <name>(<n>)… · global <n> · acdsl <files>/<rules> · plan <n> · pack reads <n or none> · subagent <n> · lang n/a` — counts from the record; IDs go to the JSON. Row `harness prose: not persisted` is stated once in the batch header, not per session.
- **Context-use findings**: `violated-despite-present` (violation evidence + rule present via channel N), `needed-not-present` (violation evidence + rule absent from every channel; name the skill/language that should have carried it), `present-irrelevant` (entry present, no touchpoint in the session's task — judgment, quoted). Each names channel, rule/entry ID, evidence quotes. A rule present and followed where it applied is `honored` (counter, no finding).
- **Signals line**: one line per session in the report — event counters worth keeping (permission denials, plan rejections, plan revisions, subagent failures) from `session_events`; omit zeros.
  Codex sessions append `DATA GAP: <peek unsupported list>` (touched files, skill breakdown, hook context) — a gap is stated, never a skip.
- **Gate verdicts**: filter the verdict log to the session's branch (`jq 'select(.branch == "<branch>")'` over `~/.claude/acdsl/verdicts.jsonl`); report red runs, the violated rule IDs, and whether reds converged to a clean run (retries-to-green evidence). Sessions on unlogged branches simply have no line — never reconstruct.

### Subagent mining (opt-in, `--subagents`)

- Off by default: without the flag, only the top-level transcript and the subagent spawn/result events + counts are mined (current behavior).
- With `--subagents`: after the top-level pass, iterate the advertised subagent ids (always present in the `session_events` / `session_full` responses) and pull each one with the `subagent: <id>` parameter on `session_get` (turns) and `session_events` (events). Findings surfaced from a subagent scope are attributed `origin: subagent <id>` in the report; permission grants/denials found in a subagent scope merge into the same session's permissions block.
- Cost: one extra paginated pull per subagent — this is why it is opt-in. Only run it when explicitly enabled.

## Skill vs Workflow

- Deterministic steps over an enumerable item list, unattended, N agents → Workflow (skill-bundled `skills/<skill>/workflows/<name>.js` script, see `skills/README.md`).
- Judgment/conventions, adaptive flow, user decisions mid-flow → Skill (`SKILL.md`).
- Both is legal: a Skill fronts the conversation, a Workflow does the mechanical fan-out — this skill is the Skill, its step 2 is Workflow-shaped.

Workflow candidates are reported as a spec, never a full script:

- name + one-line purpose
- trigger (when it would be invoked)
- item source (files, sessions, findings, PRs)
- stage graph: phases, pipeline vs barrier, where verification/consolidation sits
- per-stage structured output (field names, not JSON Schema)
- evidence: session IDs where the pattern ran manually + what it cost by hand

## 3. Report per batch

- One `sessions-batch-NN.md` per batch.
- The header states the producing skill version — a line `Analyzer: smine-batch v<version>` (this skill's frontmatter version) next to the date; §4 extracts it into the batch JSON's `analyzerVersion`.
- Full queryable session IDs, always.
- Per session, one report line — `Skills invoked: <comma-separated skill names, or "none">` — feeding the batch JSON's `invoked_skills` field (§4).
- Curate: drop no-signal sessions (ghosts, trivial); list them as skipped with one word why.
- **Cross-session arcs**: link related sessions into arcs (design→implement→verify→refine); evaluate handoff quality and skill ordering across the arc — batch-level synthesis, not per-session extraction.
- Skill and Workflow candidates get their own headed section; Workflow candidates in the spec format above.
- Generalize lessons; specifics appear only as evidence quotes.
- **Explicit evidence, per point.** Every extracted finding spells out its evidence: 1–3 full session ids ranked strongest occurrence first, plus the verbatim quotes it rests on. Batch-level sections (arcs, candidate sections) attribute per evidence point, never only per section.
- **Verbatim code snippets.** When the transcript contains the offending code and/or the corrected code (pasted code, diffs, plan excerpts), include both verbatim in fenced blocks annotated `(violation)` / `(fix)` / `(context)` with a source note (`transcript` | `plan` | `diff`). Diffs of pruned worktrees are gone — best effort via the plan text; never reconstruct code the transcript does not show.
- Emit the batch JSON (§4) before finishing — the md and its JSON are produced by the same run, never split across stages.
- Append IDs to the ledger, then STOP for user review / context management (in `--nightly` mode: skip the STOP, continue with the next batch).
- Downstream: the `/smine` pipeline routes a finished batch to the dimension skills (memory, skills, routines, context) — not this skill's job. The JSON summary is no longer a fan-out dimension; it is produced here.

## 4. Emit batch JSON

After the md is written, produce `sessions/<name>/json/<batch-stem>.json` (1:1 with the md, e.g. `sessions/claude-configs/json/sessions-batch-19.json`) conforming to `reference/schema.json`. The schema is the contract with the session-overview server — change it only deliberately, with the consumer in mind, never as a side effect of one awkward batch.

Parse from the batch report just written (the content is already in hand — no re-read of transcripts):

- Batch metadata from the header prose: scope, batch number, analyzed date, covered date range, theme, source note.
- `analyzerVersion` ← the header's `Analyzer: smine-batch v<version>` line (version string only).
- `title` — a 5–9 word human title naming the batch's centerpiece; derived from the theme/header, never a fragment copy.
- Every session in the covered/analyzed table gets an entry — including skipped ones (`skipped: true` + reason) and DATA GAP flags. Session fields from the table + title tags: `[Repo, Type]` → `repo`/`type`; signal/verdict column → `signal`.
- Findings mapped to a canonical dimension (`skill-candidate`, `workflow-candidate`, `routine-candidate`, `skill-report-card`, `feature-design`, `memory`, `rule`, `doc-drift`, `harness-friction`, `exemplar`, `context-use`, `other`) with summary + verbatim quotes; a batch-level finding not attributable to one session attaches to every session it cites.
- `frustration[]` (quote + trigger), `positive[]` (same shape), top-level `arcs[]` from cross-session arcs; `invoked_skills` ← the per-session "Skills invoked:" line split on commas. An absent section/line → field omitted (nothing invented).
- Carry fenced code blocks into `findings[].snippets` verbatim — `{kind: violation|fix|context, lang, code, source}` from the report's annotations; never invent or trim code.
- `sessions[].agent` ← `claude | codex`; Codex `unsupported` signals appended to `dataGaps`.
- `sessions[].context` ← the record verbatim (`injected`, `acdsl_rules`, `plan_rules`, `pack_reads`, `subagent_context`) plus `touched_files{reads,writes}` from `session_events` and `honored`; absent for Codex sessions.
- `sessions[].permissions` ← the granted/denied pairs from the Permissions extraction (`{granted:[{tool, command, decision}], denied:[{tool, command}], telemetry_present}`); omit the field entirely when the session had no permission activity. Forward-only: this field is the sole input smine-permissions consumes — a batch produced before this field existed simply carries no permissions data.
- `context-use` findings carry `dimension: "context-use"` and `contextUse: {kind, channel, ruleId}`.

Validate with jq before finishing: parses; `.batch.scope` and `.batch.file` present; every `.sessions[].id` matches the UUID pattern; every finding has `dimension` + `summary`. A validation failure is fixed in the same run — never leave the batch md-only. Lossy is fine, wrong is not: omit what doesn't map cleanly and count it in the run report. Only `sessions[].id` is required per session.

## Rules

- Full session IDs, never truncated.
- Artifacts are injected elsewhere as-is: self-contained, no session-local shorthand or invented codenames.
- Write outputs to the analysis dir, never only to tmp/scratchpad — tmp gets cleaned.
- Wait for background-subagent completion notifications before treating their output as missing.
- User cursing is signal, not noise — always dig in.

## Model

- Suggested: frontier / high
- Reason: mining long transcripts for non-obvious lessons
- Tested unviable: — (none yet)
