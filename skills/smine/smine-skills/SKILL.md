---
name: smine-skills
description: Extract and rank skill and workflow proposals from smine-batch reports into proposals/skills.json. Trigger on /smine-skills or "extract or rank skill or workflow proposals from batches". Args — batch file: one batch; absent means all batches with ledger-missing sessions; production cap: honored when the invocation states one.
author: Kevin Horst
version: 1.22
argument-hint: "[batch file] [production cap]"
allowed-tools: Read, Write, Edit, mcp__Peek_MCP__session_plan, mcp__Peek_MCP__session_diff, Bash(go run ./cmd/acdsl *), ToolSearch
---

# smine: skill proposals

Turn skill candidates and existing-skill report cards from batch reports into one ranked proposals doc.

## When to use

**Use when:** extracting and ranking skill proposals from batch reports — new skill candidates and existing-skill report cards. Invoked via /smine-skills or as part of the /smine fan-out.
**Don't use when:** creating or editing an actual skill or writing a workflow script — /skillroutine-create / author under `skills/<skill>/workflows/`. Extracting memory, routines, or rules — the other dimension skills. Writing a batch report — /smine-batch. Knowledge a skill's activity needs (facts, conventions, checklists, failure-case constraints) — that is context: reroute to /smine-context, which proposes the entry plus its `context:` declaration (see step 2b).
**Preconditions:** one or more completed batch reports under `sessions/`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-skills** (see README.md § Skill map, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- production cap: when the invocation states "Production cap: add at most N new proposals", write at most N new proposals this run — keep the best-ranked candidates, list every dropped candidate in the run report (never silent). Evidence appends to existing proposals do not count against the cap.

## 0. Setup

- Input: batch reports `sessions/*/*batch-*.md` — every scope directory under `sessions/` except `proposals/` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`).
- Ledger: `sessions/<scope>/analyzed-skill.txt` (historical filename, predates the smine-skills rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `proposals/skills.json` — the single authoritative artifact: cumulative, cross-scope, ranked, updated in place, conforming to `proposals/schema.json`. There is no md form.
- Repo tags: evidence concentrated in one or more repos' sessions → tag each `repo:<name>` (roster name from the batch's `[Repo]` attribution); cross-repo evidence → no repo tag. smine-apply dispositions foreign-repo proposals `manual-external` on these tags.
- Old batches predate dedicated sections — mine the prose for candidates, not only headings.

## 1. Extract & merge

- Collect three kinds per batch: **skill candidates** (new procedures, incl. validated dispatch/prompt templates), **existing-skill report-card entries** (defects/gaps in a named skill's invocation), and **workflow candidates** (deterministic orchestration in smine-batch's spec form: name, purpose, trigger, item source, stage graph, per-stage output, evidence). Apply the decision test (smine-batch, "Skill vs Workflow"): judgment/user-in-loop → skill; deterministic/unattended → workflow; time-driven → reroute note for /smine-routines.
- Merge duplicates across batches into one entry each; keep every batch's evidence (session IDs, quotes). Candidates recur — batch-01's `fimplement` is the precedent.

## 2. Inventory check

- Read the live `skills/` inventory (this repo). A candidate an existing skill already covers becomes an **edit** to that skill or is marked covered — never a duplicate proposal.
- A workflow candidate an existing `skills/<skill>/workflows/*.js` already covers becomes an edit to that script's owning skill or is marked covered. Edits beat new workflows: a **new-workflow** proposal carries a one-line justification of why no existing workflow or skill absorbs it.
- Edits beat new skills: a **new-skill** proposal requires an explicit one-line justification of why no existing skill can absorb it as an edit; absent that justification, default to an edit proposal against the closest existing skill.

## 2b. Skill edit vs context

- The boundary test: a change to the skill's **procedure** (steps, args, routing, output contract) is a skill edit; **knowledge the procedure consumes** (repo facts, style rules, review checklists, constraints from failure cases) is context — reroute it to /smine-context with a note naming every affected skill, and do not write a skill-edit proposal for it.
- Mechanism the reroute feeds: a skill's frontmatter `acdsl-context:` line declares context-entry IDs/globs; the skill-context hook injects the matching entries from the generated `context/context.json` at invocation (gate ACDSL-SKILL-004 keeps declarations resolvable). Context reaches the skill without touching its prose.
- One rule surfacing against ≥2 target skills is one finding, never N sibling edit proposals — that duplication is the reroute's strongest signal (precedent: the batch-29 rendered-mockup pair).

## 3. Archive suppression & reconciliation

- Before writing any proposal, read `proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and is eligible again from day 15 on.
- Drop every candidate matching the suppression set and list each drop (`candidate → matching archived entry`) in the run report — suppression is auditable, never silent.
- Reconcile the entries **already in `skills.json`**, not just incoming candidates: drop any `proposed` entry whose change now matches a `done`/`rejected` archive entry, or is already present in its target skill — an entry written before its cluster was applied (or migrated in from an older form) is a stale duplicate. This is the sole exception to the "never delete an entry" rule in step 4: it removes `proposed` rows only, never a user-set status.
- Every entry's `target` must resolve to a current `skills/*/<leaf>` dir. When a skill was renamed, re-point the entry in place — `target`, the `id` slug prefix (`<old>--<n>` → `<new>--<n>`), and the `title` prefix together — so the store never carries a dead skill name. List each `drop <id>` / `rename <old-target> → <new-target>` in the run report.

## 4. Rank & write

- Rank by impact, evidence-based ordering (no numeric formula): recurrence (# distinct sessions), frustration weight (attached quotes), cost of absence (burned cycles/sessions), breadth (repos/scopes affected).
- `skills.json` has three groups (`groups[]`): **New skills**, **Edits to existing skills**, **Workflows (skill-bundled scripts)**. Per proposal: title, one-line purpose, rank evidence, status. A Workflows entry names the owning skill as `target`, its `change` is "author `workflows/<name>.js` under `<target>` per spec", and the spec fields live in `fields[]` (never a full script).
- **One proposal = one change = one vote.** Every votable proposal carries exactly one `change` field — the imperative edit, quotable into the skill file. A candidate demanding N distinct changes becomes N sibling proposals with ids `<slug>--1` … `<slug>--N`, assigned once and never renumbered on re-runs (votes bind to the id). Edit proposals name their `target`; a proposal without a concrete change is not actionable and must not be written. New proposals carry a `proposed` field (`<YYYY-MM-DD>`, the analyze-run date) — stamped once at first write, never rewritten.
- **Title.** A proposal's `title` is `<target> — <distinct change name>` (Workflows group: `<target> — <script name>`) (its own evidence `title`). Split siblings (`<slug>--<n>`) carry only their distinct name — never a shared candidate heading repeated verbatim across every sibling (that renders the cards indistinguishable in the UI); the `--<n>` id already encodes the sibling link.
- Status is the user's column (`proposed | accepted | building | done | rejected | postponed`): new entries get `proposed`; re-runs may add evidence to any entry but never change a user-set status or delete an entry.

## 5. Finish

- Validate: run `go run ./cmd/acdsl check` from the repo root and fix any violation in files this run wrote. If Bash is unavailable (restricted headless run), note "schema check skipped — consolidate/CI covers it" in the run report.
- Append the processed session IDs to the ledger and set its `Last analyzed batch:` first line (insert if absent), then STOP for user review.

## Rules

- Evidence format — one `evidence[]` object per evidence point (schema fields):
  - `title` — the generalized pattern/rule name, one line, no prose blob.
  - `sessions[]` (1–3, ranked strongest occurrence first): each `{id, note}` — the full session id and one skimmable clause naming what that session evidences; the linked session carries the detail. At least one session is mandatory: a point with no attributable session id is not admissible evidence — recover the id from the batch report or fold the point into the proposal.
  - evidence is provenance only — the proposal's single `change` field lives at proposal level, never on an evidence object.
  - optional `dimension` — the deep link filters the batch page by dimension, so the effective dimension (kind default or this override) MUST match a finding dimension the cited sessions actually have in their batch JSON; verify and override, or the link lands on unrelated findings.
  - optional `snippets[]` annotated `kind` `(violation)` / `(fix)` / `(context)`, verbatim from the batch report or peek-mcp `session_plan`/`session_diff` — best effort, never reconstructed.
- Full session IDs, never truncated; evidence quotes verbatim.
- Proposals only — this skill never creates or edits a skill itself, nor writes a workflow script.
- Archived entries (`archive/done.md`, `archive/rejected.md`, `archive/postponed.md`) are anti-re-proposal memory — the mechanical suppression in step 3 is the enforcement.

## Model

- Suggested: mid-tier / medium
- Reason: merge/rank proposals against skill inventory
- Tested unviable: — (none yet)
