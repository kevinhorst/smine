---
name: fdesign
description: Produce the implementation plan for a non-trivial feature or fix — a plan, not code — and revise not-yet-implemented plans via the refine route. Trigger on /fdesign before multi-file or architecture-touching work, or /fdesign refine <plan file> to revise an approved plan. Args — refine <plan file>: revise a not-yet-implemented plan; mode: unfamiliar|familiar|owned explanation depth; caveman: compress plan prose, requires caveman skill.
author: Kevin Horst
version: 2.5
---

# fdesign

This skill produces one artifact: the implementation plan. The plan is what roots the implementation — every constraint worth enforcing must end up **inside the plan**, including the stop conditions that travel with it into implementation.

Static constraints are not restated here: `AGENTS.md`, the style guides under `$AGENT_CONTEXT_DIR_DEFAULT/style/` (incl. `plan.md`), and the doctrine entries under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (concepting — hot-class gates; implementing — stops, integrity; reviewing — Definition of Done) govern every edit. They are usually injected by the review-context hook — look for the `===== REVIEW CONTEXT (injected by hook) =====` header before re-reading them.

## When to use

**Use when:** designing a non-trivial feature or fix — multi-file or architecture-touching work — before any implementation. Invoked explicitly via /fdesign.
**Don't use when:** changing an existing feature — adjustments, behavioral tweaks, contract changes, restructuring of any size — use /fchange instead; fdesign is the initial concept → design → implementation pipeline, never the escalation target for adjustments. The solution space is still open or contested — run /fexplore first. The what/why (product shape, user stories) is unsettled — that is /concept, then /clarify. Format-only migration of a plan — /fmt plan.
**Preconditions:** a stable requirement source — a clarified concept (`plans/{slug}/concept/`), a design doc, or a precise request. If `plans/{slug}/design/exploration.md` exists, its chosen option is binding input. Refine route: a plan file produced by /fdesign (or /fchange) and at least one named driver.
**Workflow position:** clarify / fexplore → **fdesign** (refine route for pre-implementation revisions) → implementation (see `docs/skill-map.md`, smine repo).

## Args

- `refine <plan file>`: revise a not-yet-implemented plan — routes to the Refine route below; absent → the standard Phases 0–5.
- `mode`: `unfamiliar | familiar | owned` (default `familiar`) — explanation depth only, never rigor or code; `unfamiliar` adds a Flow trace, `owned` cuts Context to ≤3 bullets.
- `caveman`: compress all plan prose in the caveman style, any mode; requires the caveman skill installed, else STOP.

## Phase 0 — Intake

- **Context-doc check.** Verify `style/plan.md` and the `rules/` chapters (concepting, implementing, reviewing) exist under `$AGENT_CONTEXT_DIR_DEFAULT/` (or arrive via the review-context hook). If a repo doc is missing: state it once, continue with the skill's built-in baseline (generic stop conditions inline, baseline hot classes), and suggest seeding the repo via the smine `sync_context.sh`. Never stall on missing docs. (Origin: missing docs interrupted runs in two repos.)
- **Familiarity mode.** Intake arg `mode: unfamiliar | familiar | owned`, default `familiar`. Modes change explanation depth, never rigor and never code — facts verification, decisions, hot items, stop conditions, and the code/diff blocks in Changes are identical in all modes (code is mode-invariant, style/plan.md).
  - `unfamiliar`: the plan gains a mandatory `Flow trace` subsection under Context — per-component end-to-end trace with the user's real sample values, break point marked — plus inline explanations of domain terms. Preference questions are deferred until after the trace is presented: teach first, then ask. (Origin: "I need to be able to follow the data flow completely and see where it breaks.")
  - `familiar`: the default behavior described in this skill.
  - `owned`: Context ≤3 bullets, Baseline holds only pivotal facts, no diagrams, no explanatory prose. Changes keep their full code and diffs — only the surrounding prose is cut.
- **Caveman arg.** `caveman` compresses all plan prose in the style of the `caveman` skill (technical content byte-perfect), any mode. Requires the skill: if `~/.claude/skills/caveman` is missing, STOP with "caveman requested but skill not installed" — never approximate the style.
- **Exploration input.** If `plans/{slug}/design/exploration.md` exists (from /fexplore), its chosen option is a binding input — recorded as a `[USER]` decision citing the exploration doc; rejected options are not re-explored. If the solution space is genuinely open and no exploration exists, flag it and suggest /fexplore instead of silently locking one design.
- **Design doc input.** A user-written design doc or spec does not skip the phases — it fixes the decisions. Phase 1 still verifies the doc's assumptions against the repo (a false assumption is a finding to report, not a license to redesign). Phase 2 asks only about gaps the doc leaves open. The plan elaborates the doc into file-level precision; it never alters the doc's architecture. Disagree at most once, clearly labeled, then execute the decision. (Origin: re-architected design docs were rejected wholesale; a verified-false doc assumption was the most-valued finding of a session.)
- "Without X" excludes X and nothing else — the rest keeps full scope. If the exclusion could be read as shrinking the core task, ask one sentence instead of guessing. (Origin: "without the admin stuff" was read as dropping the entire new data model; it meant skip the admin UI.)
- "Replace" means the old thing is gone at the end: no parallel old/new mechanisms, and no code that still reads or accepts the old format "just in case". After an authorized clean break, backward-compatibility code is a bug, not a courtesy. (Origin: fallbacks parsing the old Redis key format were added despite "Redis will be wiped" — a full session was spent deleting them.)

## Phase 1 — Ground

Produce evidence, not opinions. Each step yields lines in the plan's Baseline / Exemplar sections:

1. **Baseline** — read the named base branch; check every assumption a design doc makes against actual repo state and record the deltas in the plan's Assumptions section. An assumed-but-missing refactor is a headline finding, not a footnote.
2. **Exemplar** — for every new artifact (service, model, test file), name the nearest sibling it will mirror. Mirroring means copying the sibling's exact structure — file layout, naming, helper shape, test skeleton — not just using the same general technique. ("Table-driven tests" that don't match the repo's actual test skeleton have been rejected as off-pattern.)
3. **Reuse inventory** — existing models/generics, helpers, key functions, and the generators/Makefile targets that produce or validate the artifacts involved. Never hand-write a generated format.
4. **External facts** — API capabilities, config paths, exact field names: verified in docs or source with file:line citation, never from memory.
5. **Real data** — when designing on a data shape, inspect a real artifact (JSONL record, live response, DB row) first. A field name is a hypothesis until seen.

## Phase 2 — Decide

- Enumerate the genuine design decisions the request leaves open. These are asked **in the plan, not in popups**: each becomes an `OPEN` row in the Decisions table at the point it binds (`D<n> | problem | facts | OPEN — options: a) … b) … | why it matters`), per style/plan.md. The plan is presented with its OPEN rows; the user answers against full context. Never route design decisions through AskUserQuestion — the user lacks the plan context in a popup. Everything else is pre-grounded by Phase 1, not asked.
- Local implementation questions (nil-guards, hashing, allocation, error wrapping) are decided inline and recorded in the plan's Decisions section — never asked. (Origin: the winning parallel run decided token hashing and nil-guards inline; the losing runs asked or hedged. "Q&A is bullshit for feature design; I do not have enough context to answer.")
- An answered question is binding. If the chosen option turns out much bigger than presented, ask again — never silently demote it to "deferred".
- Competing options are weighed on three axes — controllable (the user can steer it), debuggable (a failure can be located), reliable (it degrades predictably) — and the Decision's Why cites the axis that decided. State-bearing designs additionally pass the data-integrity entries (`NEVER-INTEG-*` / `ALWAYS-INTEG-*`) in `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md`.
- After a rejection, the next proposal must be structurally different; diff it against the rejected one before presenting.

## Phase 3 — Write the plan

Use the template below. Every section is required; an empty section is a finding, not a formality. A section that genuinely does not apply gets exactly `N/A — <reason>` — one line, consciously dismissed, never filler. Works as a plan-mode plan file or a standalone `.md`.

**Persistence.** When the feature has (or gets) a `plans/{slug}/` directory, the approved plan is persisted to `plans/{slug}/design/raw.md` — the immutable original; the refine route writes its revisions to `design/refined.md` next to it. Implementation follows `refined.md` when present, else `raw.md`.

Format per `$AGENT_CONTEXT_DIR_DEFAULT/style/plan.md`: bullets not blobs, code only in fenced blocks, complete units or anchored deltas, never fragments.

```markdown
# <feature> — Implementation Plan

<args line when non-default: mode: `owned`, style: `caveman` — per style/plan.md>

## TLDR
<half page max, bullets only — what is being done, why, what the result is; no tables, no diagrams, no F/D references>

## Context
<max 5 bullets — problem, cause (path:line), the design being implemented (link to the user's doc — binding), constraints; mode `unfamiliar` adds a `Flow trace` subsection: per-component end-to-end trace with real sample values, break point marked>

## Scope
<In / Out (explicit non-goals) / Not changed / Deferred findings as parent bullets;
one sub-bullet per item with a bold label naming it — never semicolon enumerations;
Deferred findings: out-of-scope discoveries, listed — never silently fixed, never dropped>

## Assumptions
<table: Assumption | Reality | Location — source doc/sketch/user premise vs. repo reality;
`N/A — <reason>` when the plan rests on no external assumptions>

## Decisions
<table: ID | Problem | Facts | Decision | Why — one row per decision, `D<n>` IDs with anchors; "Facts" cites the `F<n>` IDs the decision rests on; "Why" gives the reasoning that makes the decision follow from those facts — without it the decision is unjustified and does not stand on its own; user-made decisions marked [USER]; deliberate deviations from exemplar/doc, flagged; referenced docs linked>

## Baseline (verified)
<base branch, then the facts table: ID | Fact | Needed for | Location — one fact per row, `F<n>` IDs with anchors (pivotal facts marked `F<n>!` per style/plan.md, sorted first; anchor-only facts live in their Changes entry instead), "Needed for" links to the dependent decision/Changes entry, rows ordered by target in document order (decisions, then changes, then hot items); real data inspected>

## Exemplar & reuse
<Reuses table only: Existing | Used for — cross-cutting infrastructure. Mirrors live on each Changes entry as its mirrors: line. One bullet names any change WITHOUT an exemplar — the risk signal>

## Changes
<per file, dependency order; exact signatures; full final code for small units>

## Hot items
<every implementation in a high-risk class: example implementation written out here>

## Tests
<unit tests table: Location.Method | Cases | Comment — one case per line in the Cases cell; integration tests with the setup each requires; "not tested: X, because Y">

## Test runbook
<smoke scenarios as complete request files in the project's smoke-test tool format (discovered, never assumed): per scenario a location: line under plans/<feature>/runbooks/ + a fenced tool-native file, usable out of the box; host/auth via the tool's env mechanism only; closing run line; `N/A — <reason>` when no callable surface>

## Contracts & sweeps
<table: Contract | Sides | Sweep — every cross-boundary contract touched, sweeps across all languages/tests/fixtures/docs>

## Verification
<`- [ ]` checklist of checkable action points — verb-first, observable pass condition in the item; consumer-accepts checks; build/test via Makefile targets>

## Stop conditions
<table: ID | Condition | Action — the 6 generic ones + plan-specific>

## Open questions
<index of pointers to OPEN decision rows (`Q1 → D7`) — must be empty at approval, or the plan is presented as questions>

## Changelog
<table: Date | Trigger | What changed — created with `| — | initial | plan created |`; every post-approval edit appends a row (see style/plan.md)>
```

Section notes:

- **Changes** — plans are reviewed line-by-line, so precision per line matters. Each entry: plain-language heading + (new | modified), a `location:` line (paths as inline code), a `mirrors:` line naming its exemplar, then a fenced block — the complete final unit for new units, a `diff` block (green/red, enclosing signature + `// ...` context) for edits inside existing code. No prose-described code; no code formatting in headings; config/TOML/SQL/JSON shown as the final block content, pretty-printed.
- **Hot items** — the high-risk classes are the `ALWAYS-HOT-*` gate entries in `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md` (baseline 001–006 plus the repo's overlay entries 100+). Every planned implementation in a hot class gets its example implementation written into the plan for explicit approval **before any code is written**, citing the entry ID.
- **Tests** — coverage and structure are decided now, not improvised later. Per component: unit tests (in the project's established test style — the test style guide and the nearest sibling test file define it) vs integration tests; the setup each integration test needs (running DB/Redis, auth sessions); and an explicit "not tested: X, because Y" list the user approves. Never discover mid-implementation that a test needs infrastructure.
- **Test runbook** — scenarios come from the DoD's End-to-End Verification items and reuse the plan's real data. The tool and file format are the project's: discovery order existing collection or `plans/*/runbooks/` → Makefile smoke target → CLAUDE.md/AGENTS.md note; nothing discoverable → an OPEN decision row asking the user, never an invented default. Files persist to `plans/<feature>/runbooks/` at implementation. Usable out of the box is the bar: a request that needs editing before it runs is a finding, not a runbook entry.
- **Contracts & sweeps** — any touched JSON tag, route, query param, HTTP method, or key format lists every consumer language plus tests, fixtures, and docs. A rename or removal is finished when a repo-wide search for the old name finds nothing — a passing build only proves the compiled language was swept; the other languages and the fixtures are where leftovers hide. 
- **Verification** — observable outcomes only: the consumer accepts the artifact, the running app exercises the real end-to-end path, build/test through Makefile targets.

## Phase 4 — Self-check gate

Before presenting the plan, verify:

- [ ] For every new type, file, interface, goroutine, dependency, and endpoint in the plan, the requirement that demands it is named in the plan, next to the item. No requirement to point at = invention: remove it or ask. (The classic rejects: job stores, TTL-cleanup goroutines, unasked persistence.)
- [ ] Every helper has ≥2 callers; a single-caller helper is inlined at its call site. (One-line wrappers around existing functions get deleted in review every time.)
- [ ] For every mechanism the plan adds, the plan states which layer owns it. The recurring misplacements: persistence/IO written into a model package instead of the layer that already owns persistence; configuration injected by the caller (e.g. a CI workflow env var) instead of set in the file that owns it (e.g. the Makefile); fallback or policy behavior baked into a shared helper instead of left to the caller.
- [ ] No validation, transaction, or guard is weakened, bypassed, or deleted — unless the plan explicitly states it and gives the reasoning why that is the better approach.
- [ ] Not isomorphic to any previously rejected proposal.
- [ ] The plan passes style/plan.md: no prose blob over 3 lines; no semicolon-chained bullets; every modification a diff block with its enclosing signature; no code in headings; no inline span with 3+ code items or embedded backticks; JSON/TOML pretty-printed; canonical tables throughout; every F/D/§/doc reference an internal or file link; verification a checklist of action points; ASCII + Mermaid for any windowing/pipeline/state mechanism.
- [ ] Test runbook entries are complete tool-native request files usable out of the box (or the section is a justified N/A) — tool discovered from the project, no prose-described requests, no placeholders beyond the tool's env mechanism.
- [ ] Section order matches style/plan.md (TLDR → Context → Scope → Assumptions → Decisions → Baseline → Exemplar & reuse → Changes → …); args line present when non-default args were used.
- [ ] No semicolon-chained enumerations anywhere — Scope items are sub-bullets; multi-clause table cells stack via `<br>`, code identifier first, discriminator bold; Cases cells one case per line.
- [ ] Code is mode-invariant: every added function/method has its full code block, every modification its diff — regardless of mode/style; only named-exemplar boilerplate is described.
- [ ] Pivotal markers consistent: every fact cited in a Decision's Facts column carries `!`; no anchor-only fact sits in Baseline (they live in their Changes entry).
- [ ] Changelog section present with its initial row.
- [ ] Open questions is empty — or the plan is presented as questions, not for approval (OPEN rows in place).

## Phase 5 — Approval handoff

The plan is this skill's deliverable — approval of the plan is **not** an instruction to implement it, even though the harness's plan-mode default says to implement an approved plan. On approval:

1. Persist the plan per Phase 3 (`plans/{slug}/design/raw.md` when the feature has a plans directory).
2. If the user already stated what happens after approval (at intake, in the request, or in the approval message itself), follow that — no question.
3. Otherwise ask via AskUserQuestion how to proceed: **a)** implement now, ad hoc in this session; **b)** implement via the /fimplement skill (if installed — omit this option when it is not); **c)** plan only — persist the design file and stop.

This is the one sanctioned AskUserQuestion in this skill: it is process routing, not a design decision — the Phase-2 ban on popup design questions stands.

## Refine route

`/fdesign refine <plan file>` revises a not-yet-implemented plan. The artifact is the revised plan; the contract is consistency — a changed decision that leaves any dependent section stale makes the plan worse than before the refinement. The plan keeps this skill's template and must re-pass the full Phase 4 gate. Persistence follows Phase 3: revisions land in `plans/{slug}/design/refined.md`, `raw.md` stays untouched; repeated refinements update `refined.md` in place. Scope boundary: refinement happens before implementation starts — architecture-level rethinking is a fresh /fdesign run; mid-implementation failures are handled by the plan's stop conditions; an implemented feature needing a change is /fchange.

### R0 — Intake

- Name the plan file and the **drivers**: each piece of feedback, rejection, new fact, or failed assumption motivating the refinement. Number them — the refinement log reports against these.
- Classify each driver: decision change (targets a D<n>), fact correction (targets an F<n>), scope change (In/Out moves), or presentation-only (format, clarity).
- **Clarify the driver's premise before rejecting it.** When evaluating a user-proposed mechanism, state the assumption any rejection rests on. A rejection resting on "the current artifact doesn't do X" gets one clarifying question first when the user's team owns X — the premise may be that X will be changed. (Origin: "This is what this feature-refine should have clarified.")
- The doc rule carries over: disagree with a driver at most once, clearly labeled, then execute the decision.
- Decisions not named by any driver stay binding. Refinement is not a license to re-litigate the rest of the plan.
- `mode`/`caveman` args pass through: the refined plan keeps the original plan's mode unless overridden at invocation. The plan's args line (per style/plan.md) is kept current — an override updates it.
- **Format migration.** A plan predating the current plan-format (wrong section order, semicolon-chained cells, missing Assumptions/args line) is migrated as one implicit presentation-only driver: structure moves, content stays byte-identical, and the Changelog gets one row (`refine: format migration`). The migration mechanics are the /fmt plan route's; for format-only fixes without content drivers, use /fmt plan directly.

### R1 — Impact map

For every targeted D<n>/F<n>, walk the plan's own links before changing anything:

1. Trace through Baseline "Needed for" links, dependent Decisions, Changes entries, Hot items, Tests, Test runbook entries, Contracts & sweeps, Verification items, and Stop conditions.
2. Produce the map as a table: Driver | Targets (D/F) | Dependent sections | Action (revise / delete / unchanged).
3. Anything a revised decision newly requires (file, API, pattern, data shape) gets Phase 1 treatment — baseline checked, exemplar named, reuse inventory, real data inspected. A refinement is never designed from memory.

### R2 — Redesign

- Apply the revisions per the impact map, nothing outside it. Sections not on the map stay content-identical — only the R0 format migration may move or restructure them.
- After a rejection, the replacement must be structurally different — diff it against the rejected version before presenting. Cosmetic variation of a rejected design is a repeat rejection.
- A deleted Change leaves no orphans: its tests, contract rows, verification items, and hot-item examples go with it.
- A revised Change keeps plan precision: exact signatures, diff blocks with enclosing signatures, its `mirrors:` line updated if the exemplar changed.
- Revised decisions are updated in place in the Decisions table under their D<n>; history lives in the refinement log and the plan's Changelog, not the table.

### R3 — Rewrite & refinement log

- Update the plan in place; the template is unchanged — no extra sections invented.
- The refinement delta lands in **three places**, so it is findable in the plan and visible in chat:
  1. **Chat message**: table Driver | D/F touched | What changed — so review can be scoped to the delta instead of re-reading the whole plan. After any plan-mode round-trip, REPEAT this table in chat — the plan-mode rewrite swallows it otherwise. (Origin: "Exited Plan Mode. Tell me what you changed here in chat, not in the plan.")
  2. **Plan Changelog**: one row per driver (`refine: driver <n>`), per style/plan.md.
  3. **Rev-markers**: every revised D/F row gets `⟲` after its ID (`D3 ⟲`) so additions are findable when scrolling the plan body. Markers from earlier refinements are replaced, not stacked.

### R4 — Refine gate

Re-run the **full Phase 4 gate** on the revised plan, then the refine-specific checks:

- [ ] No dangling reference to a removed D<n>, F<n>, or Changes entry anywhere in the plan.
- [ ] No section contradicts a revised decision — checked by walking the impact map, not from memory.
- [ ] Every driver from R0 is either applied or explicitly declined with the one labeled disagreement.
- [ ] The revised plan is not isomorphic to any previously rejected version.
- [ ] Sections outside the impact map are content-unchanged (format migration excepted).
- [ ] Open questions is empty — or the plan is presented as questions, not for approval.

Refine-specific stop conditions (added to the generic set):

1. The impact map shows a driver invalidates the plan's architecture (most Changes entries, or the design named in Context) → stop; that is a fresh /fdesign run, not a refinement.
2. A fact correction cascades into more Decisions than it leaves standing → stop and report the cascade before rewriting anything.
3. A driver contradicts a [USER] decision not named by any driver → surface the conflict; never silently override a user decision.

## Stop conditions

Copy the `ALWAYS-EXEC-*` entries from `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md` into every plan's Stop conditions table (citing the entry IDs), then add plan-specific ones.

Plan-specific stop conditions name the obstacles this particular design could hit (an import cycle between named packages, a dependency's changed semantics, a data-migration edge) and the required action: stop and report.

## Model

- Suggested: frontier / high
- Reason: judgment-heavy plan design, self-check gate, hot-item code
- Tested unviable: gpt-5.5 (all sizes)

## Changelog

- v2.5 (2026-07-31): hot-class gates read from rules/concepting.md
- v2.4 (2026-07-31): activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries
- v2.3 (2026-07-30): context redesign — plan-format.md, stop-conditions.md, hot-items.md relocated into this skill's assets/; data-integrity doctrine at $AGENT_CONTEXT_DIR_DEFAULT/rules/integrity.md
- v2.2 (2026-07-30): absorbs design-refine v1.6 as the refine route (/fdesign refine <plan file>) — driver classification, impact map, redesign discipline, three-place delta reporting, refine gate + stops; design-refine retired
- v2.1 (2026-07-26): Args section
- v2.0 (2026-07-24): renamed feature-design → fdesign (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.9 (2026-07-24): Test runbook section — smoke scenarios as out-of-the-box request files in the project's tool, persisted to plans/<feature>/runbooks/; gate item; plan-format/DoD wired
- v1.8 (2026-07-24): reference merge: feature-refine + feature-refactor → feature-change; effort token normalized
- v1.7 (2026-07-19): reference rename: refactor → feature-refactor; moved under skills/feature/
- v1.6 (2026-07-19): Phase 5 approval handoff — approval ≠ implement; AskUserQuestion routes ad hoc vs /feature-implement vs plan-only unless pre-clarified
- v1.5 (2026-07-16): plan persists to plans/{slug}/design/raw.md; exploration input from design/exploration.md; TLDR section
- v1.4 (2026-07-15): feature-refine renamed to design-refine; post-implementation adjustments route to the new /feature-refine
- v1.3 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.2 (2026-07-11): data-integrity.md wired into preamble and Phase-0 check; decision axes controllable/debuggable/reliable
- v1.1 (2026-07-10): template reordered per plan-format v2; Assumptions section; Phase-0 doc-check/modes/exploration input; extended gate checks
- v1.0 (2026-07-03): initial version
