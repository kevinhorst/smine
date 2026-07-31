---
name: fchange
description: Produce the plan for any change to an existing feature — a plan, not code. Trigger on /fchange with notes or drivers. Args — notes/drivers: observed-vs-wanted statements, never solutions; mode: unfamiliar|familiar|owned explanation depth; caveman: compress plan prose, requires caveman skill.
author: Kevin Horst
version: 1.8
---

# fchange

This skill produces one artifact: the change plan. Like the fdesign skill, everything worth enforcing must end up **inside the plan** — including the stop conditions that travel with it into implementation. This skill never implements: /fimplement (or an ad-hoc session) executes the approved plan. Impact classification is the skill's job, not the user's — each driver is classified behavior-preserving, behavioral, or contract-touching, and the classification is surfaced in the plan.

Static constraints are not restated here: `AGENTS.md`, the style guides under `$AGENT_CONTEXT_DIR_DEFAULT/style/` (incl. `plan.md`), and the doctrine entries under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (concepting — hot-class gates; implementing — stops, integrity; reviewing — Definition of Done) govern every edit. They are usually injected by the review-context hook — look for the `===== REVIEW CONTEXT (injected by hook) =====` header before re-reading them.

## When to use

**Use when:** an existing, implemented feature needs a change — a minor adjustment surfaced by testing or usage, a behavioral or display/calculation tweak, a contract change, or restructuring, extraction, consolidation, or migration of any size. Invoked via /fchange with notes or drivers.
**Don't use when:** a new feature from a concept — /fdesign (the initial concept → design → implementation pipeline; never the escalation target for adjustments). The plan exists but implementation has not started — /fdesign refine. The behavior deviates from the approved plan — that is a bug: /diagnose-debug first. Executing an approved plan — /fimplement. Restructuring only to make code testable as part of a coverage push — still this skill; /coverage-increase routes such items here and never restructures inline.
**Preconditions:** the feature exists in the repo, and at least one named driver or note. No pre-classification and no target-state-source requirement — a reference implementation, when given, feeds Phase 1 but is not required.
**Workflow position:** sibling of fdesign; downstream is the same: /fimplement → package-commit. Also the post-implementation loop target: implementation → verification → **fchange** (plan) → fimplement → package-commit (see `docs/skill-map.md`, smine repo).

## Args

- notes/drivers: positional — the adjustments to plan — each an observed-vs-wanted statement, never a solution.
- `mode`: `unfamiliar | familiar | owned` (default `familiar`) — explanation depth and prose compression, never rigor (fdesign semantics).
- `caveman`: compress all plan prose in the caveman style; requires the caveman skill installed, else STOP.

## Phase 0 — Intake

- **Context-doc check.** Verify `style/plan.md` and the `rules/` chapters (concepting, implementing, reviewing) exist under `$AGENT_CONTEXT_DIR_DEFAULT/` (or arrive via the review-context hook). If a repo doc is missing: state it once, continue with the built-in baselines, suggest seeding via the smine `sync_context.sh`. Never stall on missing docs.
- **Mode/caveman args.** `mode: unfamiliar | familiar | owned` and `caveman` carry fdesign semantics (explanation depth and prose compression, never rigor). `caveman` requires `~/.claude/skills/caveman` — STOP with "caveman requested but skill not installed" if missing.
- Name the **drivers**: each note or adjustment, numbered — the plan and the final report count against these. A driver states observed vs. wanted behavior (or current vs. target structure), never a solution.
- **Impact classification, internal.** Classify every driver: behavior-preserving | behavioral | contract-touching. The classification is a plan output — the Impact column of the Drivers table — never an intake question and never a re-route. Contract-touching drivers are in scope; they demand Contracts & sweeps rows, not a different skill.
- **Originating plan lookup.** If the plan that produced the feature is named or found under `plans/`, read its Decisions: a driver contradicting a `[USER]` decision is surfaced, never silently overridden.
- **Drivers bound the scope; the named layer bounds the diff.** "Refactor the frontend" does not license schema/seed/backend cleanup. Adjacent improvements and out-of-scope discoveries go to Deferred findings — never planned alongside. (Origin: a frontend fix that wandered into seed SQL was rejected.)
- **Behavior preservation is the default contract** for restructuring drivers. Every intentional behavior change is a Decision listed in the plan — never a silent side effect of restructuring.

## Phase 1 — Ground

Produce evidence, not opinions. Each step yields a plan section. Steps 3–6 scale with the drivers: for a small display tweak they collapse to `N/A — <reason>` lines, never skipped silently.

1. **Locate** — the exact implementation each driver targets (path:line) and the tests pinning the current behavior.
2. **Observe** — for behavioral and display/calculation drivers: the actual current behavior, from the running app or a real artifact, with real values; the wanted behavior stated against the same values. A delta designed from memory is a guess.
3. **Current-state inventory** — for restructuring drivers: the files being restructured with line counts, responsibilities, and duplication counts ("this fetch ceremony appears 10+ times"). This table is the argument for the change and the checklist for its completion.
4. **Target-state extraction** — when a reference implementation exists (a sibling project, an in-repo exemplar, code included in the request): name the patterns it demonstrates and re-express them as diff blocks in the target codebase. The plan must stand alone — no "like project X does" references. (Origin: a refactor plan was rejected until all references to the exemplar project were replaced with concrete code.)
5. **Consumer inventory** — everything that references what moves, gets renamed, or changes contract, in every form: other languages, tests, fixtures, docs, Makefile `cd`s, globs, Dockerfile contexts, key formats. Grep the component's name, not just the literal old path. (Origin: sweeps that only covered the compiled language missed Python, fixtures, and README three sessions running.)
6. **Behavior inventory** — the observable behaviors that must survive (endpoints, outputs, UI interactions) and the existing tests that pin them. This becomes the verification checklist; a behavior with no pinning test is a finding.

## Phase 2 — Decide

- Present the **full opportunity menu**, ranked by value; the user cuts scope. Record the cut in the plan. Never pre-shrink the menu yourself. (Origin: 7 proposed refactor sections, the user chose 5 — the cut took one message.)
- **Disposal decisions**: for every old structure, the plan states when it is deleted. No "kept for X" leftovers without an explicit decision — a consolidation that keeps the old type alive is not done.
- **Port, don't rewrite**: when consolidating duplicates, the best-designed existing logic is carried over verbatim; deliberate adaptations are flagged in the plan. (Origin: "the group is the most well designed part of this here.")
- Genuine design decisions are asked **in the plan, not in popups**: each becomes an `OPEN` row in the Decisions table at the point it binds, per style/plan.md. Never route design decisions through AskUserQuestion — the user lacks the plan context in a popup.
- An answered decision is binding. After a rejection, the next proposal must be structurally different; diff it against the rejected one before presenting.

## Phase 3 — Write the plan

Use the template below. Every section is required; an empty section is a finding, not a formality. A section that genuinely does not apply gets exactly `N/A — <reason>` — one line, consciously dismissed, never filler.

**Persistence.** When the feature has (or gets) a `plans/{slug}/` directory, the approved plan is persisted there under `design/`; otherwise it works as a plan-mode plan file or a standalone `.md` — same rule as fdesign.

Format per `$AGENT_CONTEXT_DIR_DEFAULT/style/plan.md`: bullets not blobs, code only in fenced blocks, complete units or anchored deltas, never fragments.

```markdown
# <feature> — Change Plan

<args line when non-default: mode: `owned`, style: `caveman` — per style/plan.md>

## TLDR
<half page max, bullets only — what is being done, why, what the result is; no tables, no diagrams, no F/D references>

## Context
<why; originating plan linked when found; reference implementation named when given>

## Drivers
<table: ID | Observed | Wanted | Impact | Origin — Impact ∈ behavior-preserving | behavioral | contract-touching; Origin names the test, usage session, review, or request that surfaced it>

## Scope
<opportunity menu with the user's cut recorded;
In / Out / Not changed / Deferred findings as parent bullets, one sub-bullet per item — never semicolon enumerations>

## Assumptions
<table: Assumption | Reality | Location — what the reference implementation, originating plan, or request assumed vs. this repo's reality;
`N/A — <reason>` when the plan rests on no external assumptions>

## Current state
<file/line inventory with duplication counts and responsibilities per file; `N/A — <reason>` for small non-structural changes>

## Target state
<structure diagram; a Principle block per section: the engineering principle applied
and the language/framework mechanism that implements it; `N/A — <reason>` for small non-structural changes>

## Behavior contract
<what must not change; intentional changes (flagged decisions, matching the behavioral/contract-touching drivers)>

## Decisions
<table: ID | Problem | Facts | Decision | Why — canonical per style/plan.md; "Facts" cites Current-state/Behavior-contract rows or sections;
answered decisions are binding; disposal of every old structure and every deliberate adaptation is its own row; user-made decisions marked [USER]>

## Changes
<per phase, each independently shippable — the app works after every phase;
per entry: plain-language heading, location: line, mirrors: line where placing something new,
diff blocks (green/red, 2–3 context lines) from THIS codebase with line references; self-contained, never prose-described>

## Hot items
<every implementation in a high-risk class (see hot-items.md): example implementation for approval; a small delta in a hot class is still hot>

## Tests
<existing tests as the safety net: which pin behavior, which get updated, what is added (table: Location.Method | Cases | Comment — one case per line in the Cases cell); setup needs; "not tested: X, because Y">

## Test runbook
<smoke scenarios as complete request files in the project's smoke-test tool format (discovered, never assumed): per scenario a location: line under plans/<feature>/runbooks/ + a fenced tool-native file, usable out of the box; behavioral drivers add/adjust requests, behavior-preserving drivers re-run the existing runbook; `N/A — <reason>` when no callable surface>

## Contracts & sweeps
<table: Contract | Sides | Sweep — every touched contract with all consumers (from the consumer inventory), grep-to-zero criteria, per-survivor justification for any hit that legitimately remains; contract-touching drivers land here by design>

## Verification
<per phase, as a `- [ ]` checklist of checkable action points — verb-first, observable pass condition; per driver, observe the changed behavior in the running app with the same real values from Phase 1; build/test via Makefile targets>

## Stop conditions
<table: ID | Condition | Action — the generic ones from stop-conditions.md + the 3 skill-specific + plan-specific>

## Open questions
<index of pointers to OPEN decision rows — must be empty at approval, or the plan is presented as questions>

## Changelog
<table: Date | Trigger | What changed — created with `| — | initial | plan created |`; every post-approval edit appends a row (see style/plan.md)>
```

Section notes:

- **Principle blocks** — each target-state section names the principle (e.g. single source of truth, facade, container/presentational split) and the concrete mechanism that implements it (e.g. `as const` token objects, generics, callback props). This is what makes the plan reviewable as design, not just as a diff. (Origin: requested verbatim in the reference refactor session.)
- **Hot items** — the high-risk classes are the `ALWAYS-HOT-*` gate entries in `$AGENT_CONTEXT_DIR_DEFAULT/rules/concepting.md` (baseline 001–006 plus repo overlay entries 100+); cite the entry ID at each example.
- **Changes** — phasing follows dependency order (primitives → domain pieces → shells → final sweep). For renames/moves in compiled code, plan a compiler-driven cascade (change the core, build, fix errors, repeat) instead of pre-enumerating every edit; intermediate commits need not build individually if the branch ends green. Anything newly placed mirrors the nearest sibling's exact structure — its `mirrors:` line names the exemplar.
- **Test runbook** — scenarios come from the DoD's End-to-End Verification items and reuse Phase 1's real values. The tool and file format are the project's: discovery order existing collection or `plans/*/runbooks/` → Makefile smoke target → CLAUDE.md/AGENTS.md note; nothing discoverable → an OPEN decision row asking the user, never an invented default. Files persist to `plans/<feature>/runbooks/` at implementation; usable out of the box is the bar — a request that needs editing before it runs is a finding, not a runbook entry. Behavior-preserving restructurings normally re-run the feature's existing runbook: the section then lists which existing requests re-verify the Behavior contract instead of minting new files.
- **Verification** — behavior preservation is verified against the Behavior inventory: every listed interaction exercised in the running app, per phase, not once at the end.

## Phase 4 — Self-check gate

Before presenting the plan, verify:

- [ ] Every driver is observed-vs-wanted (or current-vs-target), not a solution statement, and carries its impact classification.
- [ ] Every contract-touching driver has its Contracts & sweeps rows — consumers enumerated, sweep criteria stated.
- [ ] The plan is self-contained: no external-project references; every change shown as a diff block from the target codebase, with its enclosing signature; new units complete; nothing prose-described.
- [ ] No change outside a driver; adjacent finds sit in Deferred findings.
- [ ] Every old structure has a disposal line; at plan end nothing exists in parallel old/new form, and no backward-compatibility code survives an authorized clean break.
- [ ] No new abstraction enters as a by-product: an import cycle or visibility problem is solved by moving the component, never by minting an interface/DTO/wrapper.
- [ ] Consolidations port the best existing logic; any rewrite of working logic is flagged as a decision.
- [ ] Every phase leaves the app working.
- [ ] Anything newly placed has a mirrors: line.
- [ ] Not isomorphic to a previously rejected proposal.
- [ ] The plan passes style/plan.md: no prose blob over 3 lines; no semicolon-chained bullets; every modification a diff block with its enclosing signature; no code in headings; no inline span with 3+ code items or embedded backticks; JSON/TOML pretty-printed; canonical tables throughout; every F/D/§/doc reference an internal or file link; verification a checklist of action points; ASCII + Mermaid for any pipeline/state mechanism.
- [ ] Test runbook entries are complete tool-native request files usable out of the box (or the section is a justified N/A) — tool discovered from the project, no prose-described requests, no placeholders beyond the tool's env mechanism.
- [ ] Section order matches the template (TLDR → Context → Drivers → Scope → Assumptions → Current state → Target state → Behavior contract → Decisions → Changes → …); args line present when non-default args were used.
- [ ] No semicolon-chained enumerations — Scope items are sub-bullets; multi-clause table cells stack via `<br>`, code identifier first, discriminator bold; Cases cells one case per line.
- [ ] Code is mode-invariant: every added unit has its full code block, every modification its diff — regardless of mode/style; only named-exemplar boilerplate is described.
- [ ] Changelog section present with its initial row.
- [ ] Open questions is empty — or the plan is presented as questions, not for approval (OPEN rows in the Decisions section per style/plan.md, never AskUserQuestion popups).

## Phase 5 — Approval handoff

The plan is this skill's deliverable — approval of the plan is **not** an instruction to implement it, even though the harness's plan-mode default says to implement an approved plan. On approval:

1. Persist the plan per Phase 3.
2. If the user already stated what happens after approval (at intake, in the request, or in the approval message itself), follow that — no question.
3. Otherwise ask via AskUserQuestion how to proceed: **a)** implement now, ad hoc in this session; **b)** implement via the /fimplement skill (if installed — omit this option when it is not); **c)** plan only — persist the plan file and stop.

This is the one sanctioned AskUserQuestion in this skill: it is process routing, not a design decision — the Phase-2 ban on popup design questions stands.

## Stop conditions

Copy the `ALWAYS-EXEC-*` entries from `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md` into every plan's Stop conditions table (citing the entry IDs), then add these three skill-specific ones, then plan-specific ones:

7. A mechanical transform (squash, move, regenerate, format conversion) → diff the result against the source element-by-element before presenting; any fidelity loss → stop. (Origin: a migration squash silently dropped NOT NULL constraints.)
8. The old and new structure would have to coexist beyond the plan's phasing → stop and report; never leave a half-migration as "done".
9. A driver contradicts a `[USER]` decision in the originating plan → surface the conflict; never silently override.

## Model

- Suggested: frontier / high
- Reason: impact classification plus behavior-preserving restructuring plans and contract-sweep design
- Tested unviable: — (none yet)

## Changelog

- v1.8 (2026-07-31): hot-class gates read from rules/concepting.md
- v1.7 (2026-07-31): activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries
- v1.6 (2026-07-30): context redesign — planning assets referenced from ../fdesign/assets/; data-integrity doctrine at $AGENT_CONTEXT_DIR_DEFAULT/rules/integrity.md; stray changelog row removed from the plan template
- v1.5 (2026-07-30): reference rename — design-refine → the fdesign refine route
- v1.4 (2026-07-30): reference rename — per-package-commit → package-commit
- v1.3 (2026-07-26): Args section
- v1.2 (2026-07-24): renamed feature-change → fchange (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.1 (2026-07-24): Test runbook section mirrored from feature-design v1.9
- v1.0 (2026-07-24): initial version — merges feature-refine v1.1 and feature-refactor v1.4 into one plan-only skill; impact classified internally; implement phase removed (handoff to /feature-implement)
