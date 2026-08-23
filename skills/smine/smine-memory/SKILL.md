---
name: smine-memory
description: Route memory-dimension findings from smine-batch reports into context proposals (proposals/context.json, kind context) — never auto-memory writes. Trigger on /smine-memory or "mine batch reports into context" or "drain auto-memory". Args — batch file: one batch; absent means all batches with ledger-missing sessions; migrate: drain an auto-memory dir into proposals instead of batches.
author: Kevin Horst
version: 2.1
argument-hint: "[batch file] [migrate [project-dir]]"
allowed-tools: Read, Write, Edit, Bash(go run ./cmd/acdsl *)
---

# smine: memory → context

Route memory candidates ("things the agent should have known") out of smine-batch reports into context proposals. This dimension never writes auto-memory: facts become votable `proposals/context.json` entries targeting context docs — visible, synced, reach-routed, and governed like every other mined change. Auto-memory itself keeps operating as the harness's staging surface; the `migrate` mode drains it into the same proposal flow.

## When to use

**Use when:** mining batch reports' memory dimension into context proposals, or draining an existing auto-memory dir (`migrate`). Invoked via /smine-memory or as part of the /smine fan-out.
**Don't use when:** the finding is an enforcement rule, doc drift, or review-DoD material — /smine-context owns those surfaces. Adding a single known fact — author the context entry directly (taxonomy ID, `context/facts/*`). Writing a batch report — /smine-batch. Editing context docs, CLAUDE.md, or memory files — this skill only proposes.
**Preconditions:** one or more completed batch reports under `sessions/` (batch mode), or an existing auto-memory dir (`migrate` mode).
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-memory** (after smine-context — shared `context.json`; see README.md § Skill map, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- migrate: `migrate [project-dir]` — input is an auto-memory dir (`~/.claude/projects/<munged-path>/memory/`: MEMORY.md + fact files) instead of batches; project-dir absent → the current project's dir. Same evaluate/route/propose machinery, no ledger involvement; evidence cites the memory file (memory files carry no session IDs). Repeatable drain — archive suppression and the covered-by filter make re-runs idempotent. Never deletes or edits memory files; deleting drained facts is a follow-up after their proposals are voted and applied.

## 0. Setup

- Input: batch reports `sessions/*/*batch-*.md` — every scope directory under `sessions/` except `proposals/` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`) plus the batch JSON (`sessions/<scope>/json/<batch>.json`, `findings[].dimension == "memory"`). `migrate` mode: the fact files listed in the target dir's MEMORY.md instead.
- Ledger (batch mode only): `sessions/<scope>/analyzed-memory.txt` (historical filename, predates the redesign) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `proposals/context.json` (kind `context`) — shared with /smine-context, cumulative, grouped by target surface, updated in place, conforming to `proposals/schema.json`. This skill's groups are the fact surfaces below; evidence objects carry `dimension: "memory"`.
- Old batches predate dedicated sections — mine the prose for memory candidates, not only "Memory candidates" headings.

## 1. Extract & cluster

- Collect memory candidates from the in-scope batches (or fact files in `migrate` mode), with their evidence (session IDs, quotes; `migrate`: the memory file path + its description line).
- Cluster duplicates across batches — the same fact recurs; recurrence count is evidence, keep it.

## 2. Evaluate (the filter)

Drop clusters that are:
- Already covered by the context surfaces: `context/context.json` + the fact/action/rule files, or global CLAUDE.md (`settings/claude_code/CLAUDE.md`). Existing auto-memory files are NOT a coverage source — a fact living only in auto-memory should become a context proposal; that is the migration ratchet.
- Derivable from the repo itself (code, git history, CLAUDE.md/AGENTS.md of the target project).
- Wrong surface: enforcement rules, doc drift, review-DoD material, style conventions — skip with a note; /smine-context extracts them from the same batches itself, no handoff.

## 3. Archive suppression

- Before writing any proposal, read `proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and is eligible again from day 15 on.
- Drop every candidate matching the suppression set and list each drop (`candidate → matching archived entry`) in the run report — suppression is auditable, never silent.

## 4. Route & propose

Group survivors by target surface and write each as a proposal (`groups[]` per surface, one `change` per proposal, `<slug>--<n>` split siblings, `proposed` date stamped once — the shared context.json conventions):

- `context/facts/<domain>.md` — durable repo/world facts as FACT entries: ID per the `KIND-SCOPE[-TOPIC]-NNN` grammar with scope/topic from the registered taxonomy (`context/aspects.json`; new scopes registered first — `context/actions/README.md`), `Location:` bullet required.
- Skill `acdsl-context:` declarations — when a fact exists to serve named skills, the proposal's single change covers both the entry and "declare <entry-ID> in skills/<path>/SKILL.md `acdsl-context:`" for each consuming skill.
- `settings/claude_code/CLAUDE.md` — user-level preferences and cross-project working facts (the old `user`/`feedback` memory types) become CLAUDE.md-edit proposals.
- External repos not on the sync roster — flagged with the exact target path in the run report, never edited and never proposed here.

Rules for the proposals:
- **No `gate` object** — facts are knowledge, not enforcement; `context/facts/*` sits outside the band-mandatory targets. A candidate that turns out to be a rule is a wrong-surface skip (§2), never a J-band fact.
- **Thresholds**: evidence from ≥2 distinct sessions, OR 1 session + the fact verified against current repo/world state during this run (name the verification in the evidence `note`). Sub-threshold candidates are NOT written: list each in the run notes (`candidate → below threshold (n sessions)`). `migrate` mode: a fact file counts as one occurrence; verify against the repo before proposing (stale memory facts are drops, not proposals).
- **Repo tags (reach routing)**: evidence concentrated in one or more repos → tag each `repo:<name>` (roster name = target dir basename); spread across most repos → no tag (global reach). smine-apply routes tags into the entry's `Reach:` bullet.
- Read the current target surface before proposing: near-misses become amendments referencing the existing entry ID; a new-entry proposal carries a one-line justification of why no existing entry absorbs it.

## 5. Finish

- Validate: run `go run ./cmd/acdsl check` from the repo root and fix any violation in files this run wrote. If Bash is unavailable (restricted headless run), note "schema check skipped — consolidate/CI covers it" in the run report.
- Report: proposed (per surface), dropped-covered, dropped-derivable, dropped-suppressed, skipped-wrong-surface, below-threshold — counts plus the survivor list.
- Batch mode: append the processed session IDs to the ledger and set its `Last analyzed batch:` first line (insert if absent). Then STOP for user review.

## Rules

- Evidence format (batch mode) — one `evidence[]` object per point (schema fields): `title` (the fact, one line); `sessions[]` (1–3, ranked, each `{id, note}`, full IDs never truncated); `dimension: "memory"`; optional `snippets[]` verbatim from the batch report. `migrate` mode: memory files carry no session IDs and the schema requires them on evidence — omit `evidence[]` entirely and record provenance as a `fields[]` entry (`label: "source"`, text = the memory file path + its description line).
- Evidence quotes verbatim; better to drop than to bloat — an unclear candidate is a drop, not a hedge-write.
- Proposals only — this skill never edits a context doc, CLAUDE.md, a memory file, or an external repo.
- Runs after /smine-context in the pipeline fan-out — the two skills share `proposals/context.json` and must not write it concurrently.

## Model

- Suggested: mid-tier / medium
- Reason: clustering candidates against context surfaces and diffing against existing entries
- Tested unviable: — (none yet)
