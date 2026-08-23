---
name: smine-context
description: Extract context-surface rules and doc drift from smine-batch reports into proposals/context.json (kind context). Trigger on /smine-context or "extract context rules or doc fixes from batches". Args — batch file: one batch; absent means all batches with ledger-missing sessions; production cap: honored when the invocation states one.
author: Kevin Horst
version: 1.31
argument-hint: "[batch file] [production cap]"
allowed-tools: Read, Write, Edit, mcp__Peek_MCP__session_plan, mcp__Peek_MCP__session_diff, Bash(go run ./cmd/acdsl *), ToolSearch
---

# smine: context proposals

Turn repo-surface rules and doc-drift findings from batch reports into per-surface edit lists. The ad-hoc precedent is TODO.md (memory-batch evaluation: 5 of 24 clusters were "right content, wrong surface").

## When to use

**Use when:** extracting context-surface rules and doc drift from batch reports — style rules, review checklist items, process rules, and contradictions against checked-in docs. Invoked via /smine-context or as part of the /smine fan-out.
**Don't use when:** the finding is a process rule belonging inside a specific skill — reroute to /smine-skills. Editing context docs or AGENTS.md directly — this skill only proposes.
**Preconditions:** one or more completed batch reports under `sessions/`.
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-context** (see README.md § Skill map, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- production cap: when the invocation states "Production cap: add at most N new proposals", write at most N new proposals this run — keep the best-ranked candidates, list every dropped candidate in the run report (never silent). Evidence appends to existing proposals do not count against the cap.

## 0. Setup

- Input: batch reports `sessions/*/*batch-*.md` — every scope directory under `sessions/` except `proposals/` (both naming schemes: `sessions-batch-NN.md`, `session-analysis-batch-NN.md`) plus the batch JSON (`sessions/<scope>/json/<batch>.json`) for records and context-use findings.
- Ledger: `sessions/<scope>/analyzed-rules.txt` (historical filename, predates the smine-style rename) — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `proposals/context.json` (kind `context`) — the single authoritative artifact: cumulative, cross-scope, grouped by surface, updated in place, conforming to `proposals/schema.json`. Shared with /smine-memory (its groups are the fact surfaces), which the fan-out runs after this skill — never concurrently with it. Non-context findings (project-local tooling, external-repo flags, reroutes) live there as their own groups. There is no md form and no style.json (kind `style` retired 2026-08-09).
- Old batches predate dedicated sections — mine the prose: corrections phrased as conventions ("never X", "always Y", test-struct rules) are rule material wherever they appear.

## 1. Extract

- **Context-use findings** (batch JSON `findings[].dimension == "context-use"`, with `sessions[].context` records) — three sub-kinds, each routed in step 4b.
- **Rules**: corrections that belong in enforcement surfaces — style/design conventions, review checklist items, process rules. Plain facts/knowledge are /smine-memory's side of the seam (it proposes them into this same context.json as fact-surface groups) — skip with a note, no handoff.
- **Doc drift**: session evidence contradicting checked-in docs — quote the contradiction and name the doc.

## 2. Group by target surface

- `context/actions/*` — ACTION activity chapters (implementing, navigating) + reviewing.md (Definition of Done).
- `context/rules/*` — artifact style guides and RULE-* candidates (plan.md, commits.md, per-language files incl. go.md's TESTS/GOROUTINES sections).
- `context/AGENTS.md` — template-level conventions.
- Skills — only changes to a skill's **procedure** (steps, args, routing, output contract) go to /smine-skills's report card; **knowledge a skill needs** stays here as an entry + declaration.
- Skill `acdsl-context:` declarations — when an entry exists to serve named skills, the proposal's single change covers both the entry and "declare <entry-ID> in skills/<path>/SKILL.md `acdsl-context:`" for each consuming skill; this replaces per-skill prose edits (one rule, one vote). Mechanism: the frontmatter `acdsl-context:` line lists entry IDs/globs; the skill-context hook injects the matching entries from the generated `context/context.json` at invocation (gate ACDSL-SKILL-004 keeps declarations resolvable).
- External repos (their AGENTS.md/CLAUDE.md/docs) — flagged with the exact target path, never edited from here.

## 3. Archive suppression & reconciliation

- Before writing any proposal, read `proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and is eligible again from day 15 on.
- Drop every candidate matching the suppression set and list each drop (`candidate → matching archived entry`) in the run report — suppression is auditable, never silent.
- Reconcile the entries **already in `context.json`**, not just incoming candidates: drop any `proposed` entry whose rule/change now matches a `done`/`rejected` archive entry, or is already present on its target surface — an entry written before its rule was applied (or migrated in from an older form) is a stale duplicate. This touches `proposed` rows only, never a user-set status. List each `drop <id>` in the run report.

## 4. Diff & write

- Read the current target surface before proposing: drop rules already covered; near-misses become amendments referencing the existing rule ID. Amendments beat new rules: a **new-rule** proposal carries a one-line justification of why no existing rule or context doc absorbs it.
- **Band classification (context targets)** — every proposal targeting `context/actions/*`, `context/rules/*`, or `context/AGENTS.md` carries a `gate` object (schema): `band` F/A/D/J per the ACDSL band taxonomy. F/A/D additionally name `verifier` (an existing `acdsl/registry.json` entry when one fits, else a one-line sketch) and an `anchor` regex. J-band (judgment prose) carries no verifier — instead the proposal's `fields[]` MUST include a worked example (measured: example-form rules held under contested attention where prose-form collapsed) and the tag `ab-required`. A proposal whose `gate.verifier` is a sketch (not an existing `acdsl/registry.json` entry) MUST carry ≥1 `violation` and ≥1 `context` (clean) snippet in its evidence — they seed the generated rule's fail/pass fixtures (smine-apply generation route); without both, the proposal is not gate-complete and stays J-form prose.
- **Occurrence thresholds (context targets)** — a context proposal needs evidence from ≥2 distinct sessions; a J-band proposal needs ≥3. Sub-threshold candidates are NOT written as proposals: list each in the run notes (`candidate → below threshold (n sessions)`) so recurrence is findable on the next run.
- **Repo tags (reach routing)** — evidence concentrated in one or more repos' sessions → tag each `repo:<name>` (roster name = target dir basename, from the batch's `[Repo]` attribution); evidence spread across most repos → no repo tag (global reach). smine-apply routes on these tags into the entry's `Reach:` bullet / rule's `reach=` field.
- `context.json` groups proposals by target surface (`groups[]`), one proposal object per item, evidence (session IDs + quotes) attached. Each proposal's `status` (`proposed | applied | rejected | postponed`) is the user's; re-runs add evidence, never change a user-set status or delete a proposal.
- **Title.** A proposal's `title` is its own distinct rule name (one line, = its evidence `title`). Split siblings (`<slug>--<n>`) never repeat a shared candidate heading verbatim across every sibling — that renders the cards indistinguishable in the UI; the `--<n>` id already links them, and the group header already names the target surface.
- **One proposal = one change = one vote.** Every votable proposal carries exactly one `change` field — the imperative edit. A candidate demanding N distinct edits becomes N sibling proposals with ids `<slug>--1` … `<slug>--N`, assigned once and never renumbered on re-runs (votes bind to the id). New proposals carry a `proposed` field (`<YYYY-MM-DD>`, the analyze-run date) — stamped once at first write, never rewritten.

## 4b. Context-use routes

- `violated-despite-present` (≥2 sessions; ≥3 for J): the step-4 gate route — band F/A/D with verifier sketch + anchor and the finding's snippets, or J worked-example rewrite tagged `ab-required`. The delivering channel goes on each evidence `note` (`via skill-context /fimplement`, `via read-gate go`).
- `needed-not-present` (≥2 sessions): rule exists + a skill was invoked → `acdsl-context:` declaration amendment for that skill (step 2 mechanism, one proposal covers entry + declaration); rule exists, no skill scope, should bind on read → ACDSL marker line via the gate object; `RULE-<LANG>-*` in a session whose record shows the language delivery landed (`injected.lang` non-null) → hook defect line in the run report, no proposal; rule missing → new-rule proposal (amendments-beat-new justification).
- `present-irrelevant`: aggregate per ID across sessions; ≥5 sessions with zero relevance → eviction proposal — `change` = `set projected="false" on <ID>` (ACDSL rule) or `remove <ID> from <skill> context:` / drop from guide (prose entry). ACDSL eviction additionally requires the verdict bar below.
- **Verdict join**: `go run ./cmd/acdsl verdicts -since 720h`; an ACDSL eviction proposal is written only when verifier + fixtures are green and ≥300 clean projected runs are logged (acdsl/README.md); rules below the bar are listed in the run report as `<ID>: n/300 clean projected runs`.
- **Channel stats**: one run-report table — per repo tag, sessions with each channel present, pack reads vs pushes, `honored` totals; informational, never lowers a threshold.

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
- Proposals only — this skill never edits a context doc, AGENTS.md, or external repo itself.
- Single-session evidence never becomes a context proposal (thresholds in step 4); for non-context targets one occurrence still proposes tentatively, recurrence hardens.
- Archived items (`archive/done.md`, `archive/rejected.md`, `archive/postponed.md`) are anti-re-proposal memory — the mechanical suppression in step 3 is the enforcement.

## Model

- Suggested: mid-tier / medium
- Reason: diff rule candidates against current docs
- Tested unviable: — (none yet)
