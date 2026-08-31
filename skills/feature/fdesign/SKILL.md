---
name: fdesign
description: Produce the plan for feature work — a plan, not code — via routes new, change, and refine. Trigger on /fdesign before multi-file or architecture-touching work, /fdesign change <drivers> for changes to existing features, or /fdesign refine <plan file>. Args — change <drivers>: observed-vs-wanted statements; refine <plan file>: revise a not-yet-implemented plan; mode: unfamiliar|familiar|owned; caveman: compress prose; runbook: full request files.
author: Kevin Horst
version: 3.7
argument-hint: "[change <drivers> | refine <plan file>] [mode] [caveman] [runbook]"
acdsl-context: ACTION-CONCEPT-*, ACTION-IMPL-*, ACTION-REVIEW-*, RULE-PLAN-*, FACT-*
---

# fdesign

This skill produces one artifact: the plan — the implementation plan on the default (new) route, the change plan on the change route, the revised plan on the refine route. The plan is what roots the implementation — every constraint worth enforcing must end up **inside the plan**, including the stop conditions that travel with it into implementation. This skill never implements: /fimplement executes the approved plan — and it does so by default right after approval (Phase 4), unless the user states otherwise.

Static constraints are not restated here: `AGENTS.md`, the format guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (incl. `plan.md`), and the doctrine chapters under `$AGENT_CONTEXT_DIR_DEFAULT/actions/` (concepting — hot-class gates; implementing — stops, integrity; reviewing — Definition of Done) govern every edit. They arrive via this skill's frontmatter `acdsl-context:` declaration, injected at invocation by the skill-context hook; per-language style guides are read from `$AGENT_CONTEXT_DIR_DEFAULT/rules/` on demand. Re-read entries from `$AGENT_CONTEXT_DIR_DEFAULT/` only when they are no longer in context. The checkable slice of these constraints is enforced by ACDSL gates (`go run ./cmd/acdsl check`, projection blocks on governed files, `project -plan` for planned paths) — for anything a gate covers, the gate is authoritative, not the prose.

## When to use

**Use when:** planning any feature work before implementation — new route: a non-trivial feature, multi-file or architecture-touching; change route: an existing, implemented feature needs a change — an adjustment surfaced by testing or usage, a behavioral or display/calculation tweak, a contract change, or restructuring, extraction, consolidation, or migration of any size (a coverage push that needs restructuring routes here too); refine route: an approved, not-yet-implemented plan has drivers against it.
**Don't use when:** the solution space is still open or contested — run /fexplore first. The what/why (product shape, user stories) is unsettled — that is /concept, then /clarify. The behavior deviates from the approved plan — that is a bug: /diagnose-debug first. Executing an approved plan — /fimplement. Format-only migration of a plan — /fmt plan.
**Preconditions:** new route: a stable requirement source — a clarified concept (`plans/{slug}/concept/`), a design doc, or a precise request; if `plans/{slug}/design/exploration.md` exists, its chosen option is binding input. Change route: the feature exists in the repo and at least one named driver or note; no pre-classification, no target-state source required. Refine route: a plan file produced by this skill and at least one named driver.
**Workflow position:** clarify / fexplore → **fdesign** (change route as the sibling entry for existing features, refine route for pre-implementation revisions) → implementation. Also the post-implementation loop target: implementation → verification → **fdesign change** (plan) → fimplement → package-commit (see README.md § Skill map, smine repo).

## Args

- `change <drivers>`: plan a change to an existing feature — routes to the Change route below; the drivers are positional observed-vs-wanted statements (or current-vs-target structure), never solutions.
- `refine <plan file>`: revise a not-yet-implemented plan — routes to the Refine route below; absent → the standard Phases 0–5.
- `mode`: `unfamiliar | familiar | owned` (default `familiar`) — explanation depth only, never rigor or code; `unfamiliar` adds a Flow trace, `owned` cuts Context to ≤3 bullets.
- `caveman`: compress all plan prose in the caveman style, any mode; requires the caveman skill installed, else STOP.
- `runbook`: generate full tool-native runbook request files in the plan; absent (default) the Test runbook section is a scenario index only — see the section note.

## Phase 0 — Intake

**SKILL-FDESIGN-INTAKE-001** `[step]` — **Context-doc check.** Verify `rules/plan.md` and the `actions/` chapters (concepting, implementing, reviewing) exist under `$AGENT_CONTEXT_DIR_DEFAULT/` (or their entries already arrived via this skill's context declaration). If a repo doc is missing: state it once, continue with the skill's built-in baseline (generic stop conditions inline, baseline hot classes), and suggest seeding the repo via the smine `sync_context.sh`. Never stall on missing docs. (Origin: missing docs interrupted runs in two repos.)

**SKILL-FDESIGN-INTAKE-002** `[step]` — **Familiarity mode.** Intake arg `mode: unfamiliar | familiar | owned`, default `familiar`. Modes change explanation depth, never rigor and never code — facts verification, decisions, hot items, stop conditions, and the code/diff blocks in Changes are identical in all modes (code is mode-invariant, rules/plan.md).

  - `unfamiliar`: the plan gains a mandatory `Flow trace` subsection under Context — per-component end-to-end trace with the user's real sample values, break point marked — plus inline explanations of domain terms. Preference questions are deferred until after the trace is presented: teach first, then ask. (Origin: "I need to be able to follow the data flow completely and see where it breaks.")
  - `familiar`: the default behavior described in this skill.
  - `owned`: Context ≤3 bullets, Baseline holds only pivotal facts, no diagrams, no explanatory prose. Changes keep their full code and diffs — only the surrounding prose is cut.

**SKILL-FDESIGN-INTAKE-003** `[gate]` — **Caveman arg.** `caveman` compresses all plan prose in the style of the `caveman` skill (technical content byte-perfect), any mode. Requires the skill: if `~/.claude/skills/caveman` is missing, STOP with "caveman requested but skill not installed" — never approximate the style.

**SKILL-FDESIGN-INTAKE-004** `[step]` — **Exploration input.** If `plans/{slug}/design/exploration.md` exists (from /fexplore), its chosen option is a binding input — recorded as a `[USER]` decision citing the exploration doc; rejected options are not re-explored. If the solution space is genuinely open and no exploration exists, flag it and suggest /fexplore instead of silently locking one design.

**SKILL-FDESIGN-INTAKE-005** `[review]` — **Design doc input.** A user-written design doc or spec does not skip the phases — it fixes the decisions. Phase 1 still verifies the doc's assumptions against the repo (a false assumption is a finding to report, not a license to redesign). Phase 2 asks only about gaps the doc leaves open. The plan elaborates the doc into file-level precision; it never alters the doc's architecture. Disagree at most once, clearly labeled, then execute the decision. (Origin: re-architected design docs were rejected wholesale; a verified-false doc assumption was the most-valued finding of a session.)

**SKILL-FDESIGN-INTAKE-006** `[review]` — **Errata supersede the spec.** A feedback or errata section accompanying a spec supersedes the spec text it corrects. Where the two contradict, that is a clarifying question, never a silent judgment call.

**SKILL-FDESIGN-INTAKE-007** `[review]` — "Without X" excludes X and nothing else — the rest keeps full scope. If the exclusion could be read as shrinking the core task, ask one sentence instead of guessing. (Origin: "without the admin stuff" was read as dropping the entire new data model; it meant skip the admin UI.)

**SKILL-FDESIGN-INTAKE-008** `[review]` — "Replace" means the old thing is gone at the end: no parallel old/new mechanisms, and no code that still reads or accepts the old format "just in case". After an authorized clean break, backward-compatibility code is a bug, not a courtesy. (Origin: fallbacks parsing the old Redis key format were added despite "Redis will be wiped" — a full session was spent deleting them.)

## Phase 1 — Ground

Produce evidence, not opinions. Each step yields lines in the plan's Baseline / Exemplar sections:

**SKILL-FDESIGN-GROUND-001** `[step]` — **Baseline** — read the named base branch; check every assumption a design doc makes against actual repo state and record the deltas in the plan's Assumptions section. An assumed-but-missing refactor is a headline finding, not a footnote. In ACDSL repos (`acdsl/registry.json` present), run `go run ./cmd/acdsl project -plan <path>` (use `./bin/acdsl` when the repo vendors the binary) for every file the plan creates or modifies and record the governing rules in that file's Changes entry — the plan carries its gate context.

**SKILL-FDESIGN-GROUND-002** `[step]` — **Exemplar** — for every new artifact (service, model, test file), name the nearest sibling it will mirror. Mirroring means copying the sibling's exact structure — file layout, naming, helper shape, test skeleton — not just using the same general technique. ("Table-driven tests" that don't match the repo's actual test skeleton have been rejected as off-pattern.)

**SKILL-FDESIGN-GROUND-003** `[step]` — **Reuse inventory** — existing models/generics, helpers, key functions, and the generators/Makefile targets that produce or validate the artifacts involved. Never hand-write a generated format.

**SKILL-FDESIGN-GROUND-004** `[step]` — **External facts** — API capabilities, config paths, exact field names: verified in docs or source with file:line citation, never from memory.

**SKILL-FDESIGN-GROUND-005** `[step]` — **Real data** — when designing on a data shape, inspect a real artifact (JSONL record, live response, DB row) first. A field name is a hypothesis until seen.

**SKILL-FDESIGN-GROUND-006** `[step]` — **Premise validation** — validate the design's premise against real data before building: do the throughput math before proposing batch limits, profile before designing performance tiers. Measurements the user ran in prod are binding design facts, not hypotheses to re-derive.

## Phase 2 — Decide

**SKILL-FDESIGN-DECIDE-001** `[step]` — Enumerate the genuine design decisions the request leaves open. These are asked **in the plan, not in popups**: each becomes an `OPEN` row in the Decisions table at the point it binds (`D<n> | problem | facts | OPEN — options: a) … b) … | why it matters`), per rules/plan.md. The plan is presented with its OPEN rows; the user answers against full context. Never route design decisions through AskUserQuestion — the user lacks the plan context in a popup. Everything else is pre-grounded by Phase 1, not asked.

**SKILL-FDESIGN-DECIDE-002** `[review]` — Local implementation questions (nil-guards, hashing, allocation, error wrapping) are decided inline and recorded in the plan's Decisions section — never asked. (Origin: the winning parallel run decided token hashing and nil-guards inline; the losing runs asked or hedged. "Q&A is bullshit for feature design; I do not have enough context to answer.")

**SKILL-FDESIGN-DECIDE-003** `[review]` — An answered question is binding. If the chosen option turns out much bigger than presented, ask again — never silently demote it to "deferred".

**SKILL-FDESIGN-DECIDE-004** `[review]` — Competing options are weighed on three axes — controllable (the user can steer it), debuggable (a failure can be located), reliable (it degrades predictably) — and the Decision's Why cites the axis that decided. State-bearing designs additionally pass the data-integrity entries (`ACTION-IMPL-INTEG-*`) in `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md`.

**SKILL-FDESIGN-DECIDE-005** `[review]` — After a rejection, the next proposal must be structurally different; diff it against the rejected one before presenting.

## Phase 3 — Write the plan

**SKILL-FDESIGN-WRITE-001** `[step]` — Use the template below. Every section is required; an empty section is a finding, not a formality. A section that genuinely does not apply gets exactly `N/A — <reason>` — one line, consciously dismissed, never filler. Works as a plan-mode plan file or a standalone `.md`.

**SKILL-FDESIGN-WRITE-002** `[step]` — **Persistence.** When the feature has (or gets) a `plans/{slug}/` directory, the approved plan is persisted to `plans/{slug}/design/raw.md` — the immutable original; the refine route writes its revisions to `design/refined.md` next to it; the change route writes `design/change-<topic>.md` and never touches raw/refined. Implementation follows `refined.md` when present, else `raw.md`; a change plan is implemented on its own.

**SKILL-FDESIGN-WRITE-003** `[review]` — **Routes and sections.** One template for every route. Sections tagged `[new]` belong to the new route, `[change]` to the change route; on the other route the section is exactly `N/A — <other> route`. The title follows the route: `— Implementation Plan` (new) or `— Change Plan` (change); the args line carries `route: change` on the change route.

**SKILL-FDESIGN-WRITE-004** `[review]` — Format per `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`: bullets not blobs, code only in fenced blocks, complete units or anchored deltas, never fragments.

**SKILL-FDESIGN-TPL-001** `[payload]` — The plan template — one template for every route.

````markdown
# <feature> — Implementation Plan | Change Plan

<args line when non-default: route: `change`, mode: `owned`, style: `caveman` — per rules/plan.md>

## TLDR
<half page max, bullets only — what is being done, why, what the result is; no tables, no diagrams, no F/D references>

## Context
<max 5 bullets — problem, cause (path:line), the design being implemented (link to the user's doc — binding), constraints; change route: originating plan linked when found, reference implementation named when given; mode `unfamiliar` adds a `Flow trace` subsection: per-component end-to-end trace with real sample values, break point marked>

## Drivers
<[change] table: ID | Observed | Wanted | Impact | Origin — Impact ∈ behavior-preserving | behavioral | contract-touching; Origin names the test, usage session, review, or request that surfaced it>

## Scope
<change route: opportunity menu with the user's cut recorded first;
In / Out (explicit non-goals) / Not changed / Deferred findings as parent bullets;
one sub-bullet per item with a bold label naming it — never semicolon enumerations;
Deferred findings: out-of-scope discoveries, listed — never silently fixed, never dropped>

## Assumptions
<table: Assumption | Reality | Location — source doc/sketch/user premise (change route: reference implementation, originating plan, or request) vs. repo reality;
`N/A — <reason>` when the plan rests on no external assumptions>

## Current state
<[change] file/line inventory with duplication counts and responsibilities per file; `N/A — <reason>` for small non-structural changes>

## Target state
<[change] structure diagram; a Principle block per section: the engineering principle applied and the language/framework mechanism that implements it; `N/A — <reason>` for small non-structural changes>

## Behavior contract
<[change] what must not change; intentional changes (flagged decisions, matching the behavioral/contract-touching drivers)>

## Decisions
<table: ID | Problem | Facts | Decision | Why — one row per decision, `D<n>` IDs as plain text, referenced via section-slug links (rules/plan.md RULE-PLAN-018); "Facts" cites the `F<n>` IDs (change route: Current-state / Behavior-contract rows) the decision rests on; "Why" gives the reasoning that makes the decision follow from those facts — without it the decision is unjustified and does not stand on its own; user-made decisions marked [USER]; deliberate deviations from exemplar/doc, flagged; change route: disposal of every old structure and every deliberate adaptation is its own row; referenced docs linked>

## Open questions
<index of pointers to OPEN decision rows (`Q1 → D7`) — must be empty at approval, or the plan is presented as questions>

## Baseline (verified)
<[new] base branch, then the facts table: ID | Fact | Needed for | Location — one fact per row, `F<n>` IDs as plain text, referenced via section-slug links (pivotal facts marked `F<n>!` per rules/plan.md, sorted first; anchor-only facts live in their Changes entry instead), "Needed for" links to the dependent decision/Changes entry, rows ordered by target in document order (decisions, then changes, then hot items); real data inspected>

## Exemplar & reuse
<[new] Reuses table only: Existing | Used for — cross-cutting infrastructure. Mirrors live on each Changes entry as its mirrors: line. One bullet names any change WITHOUT an exemplar — the risk signal>

## Changes
<new route: per file, dependency order; exact signatures; full final code for small units. Change route: per phase, each independently shippable — the app works after every phase; diff blocks from THIS codebase with line references, self-contained; UI-touching entries carry a ui: line linking their screenshot (RULE-PLAN-070)>

## Hot items
<every implementation in a high-risk class: example implementation written out here; a small delta in a hot class is still hot; UI-touching changes embed the screenshot of the actual UI under design here — captured from the running app, stored under plans/{slug}/design/ui/ (ACTION-CONCEPT-HOT-007, RULE-PLAN-069) — never a prose widget description>

## Tests
<unit tests table: Location.Method | Cases | Comment — one case per line in the Cases cell; change route: existing tests as the safety net — which pin behavior, which get updated, what is added; integration tests with the setup each requires; "not tested: X, because Y">

## Test runbook
<default: scenario index — one line per smoke scenario (name, tool/endpoint, data source), no request files; change route: behavioral drivers list the requests they would add/adjust, behavior-preserving drivers name which existing runbook requests re-verify the Behavior contract; with the `runbook` arg or an explicit user ask: complete request files in the project's smoke-test tool format (discovered, never assumed), per scenario a location: line under plans/<feature>/runbooks/ + a fenced tool-native file, usable out of the box, host/auth via the tool's env mechanism only, closing run line; `N/A — <reason>` when no callable surface>

## Contracts & sweeps
<table: Contract | Sides | Sweep — every cross-boundary contract touched, sweeps across all languages/tests/fixtures/docs; change route: consumers from the consumer inventory, grep-to-zero criteria, per-survivor justification — contract-touching drivers land here by design>

## Verification
<`- [ ]` checklist of checkable action points — verb-first, observable pass condition in the item; consumer-accepts checks; change route: per phase, per driver the changed behavior observed in the running app with Phase C1's real values; build/test via Makefile targets>

## Stop conditions
<table: ID | Condition | Action — the 6 generic ones + route-specific + plan-specific>
````

## Phase 4 — Approval handoff

**SKILL-FDESIGN-HANDOFF-001** `[gate]` — **Approval means implement.** On plan approval, the default continuation is /fimplement on the approved plan — immediately, in the same session, without asking. Never end the turn at approval waiting for a go-ahead. Only an explicit contrary statement ("only plan", "don't implement", a stated handoff to another session) makes approval terminal. (Origin: sessions repeatedly stopped after approval; the stall went unnoticed and wasted wall-clock time.)
