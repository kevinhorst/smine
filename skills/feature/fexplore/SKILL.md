---
name: fexplore
description: Survey all sensible solutions for a feature — an evaluated solution space with a recommendation. Trigger on /fexplore or "explore solutions/options/approaches". Args — mode: unfamiliar|familiar|owned explanation depth.
author: Kevin Horst
version: 1.6
---

# fexplore

This skill produces one artifact: `plans/{slug}/design/exploration.md` — the evaluated solution space. It sits between clarify (questions drained, concept stable) and fdesign (one solution elaborated to file-level precision). fdesign locks a design in; this skill exists so the lock happens on a surveyed space, not the first workable idea. (Origin: identical parallel runs reached opposite verdicts on the same question; a bake-off was won by groundedness, not architecture; constraint-first analysis beat iterative variant-proposing)

Static constraints are not restated here: `AGENTS.md`, the style guides under `$AGENT_CONTEXT_DIR_DEFAULT/style/` (incl. `plan.md`), and the doctrine entries under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` govern every artifact. Missing context docs: state it once, continue with built-in baselines — never stall.

## When to use

**Use when:** the solution space is open or contested — a performance concern, a rejected first design, a new constraint, a user candidate to evaluate — or the user asks to "explore solutions/options/approaches".
**Don't use when:** the solution is already chosen or the space is narrow — go straight to /fdesign. The concept still has space-changing open questions — /clarify first (explore-specific stop condition #1). The deliverable is a plan or code — this skill produces neither.
**Preconditions:** a stable concept in `plans/{slug}/concept/` (questions drained).
**Workflow position:** clarify → **fexplore** → fdesign, which records the chosen option as a `[USER]` decision citing `exploration.md` (see `docs/skill-map.md`, smine repo).

## Args

- `mode`: `unfamiliar | familiar | owned` (default `familiar`) — explanation depth, never rigor.

## Phase 0 — Intake

- Locate the concept (`plans/{slug}/concept/`) and the drivers: what makes the solution space open (performance concern, rejected first design, new constraint, user's own candidate idea).
- Familiarity mode (`mode: unfamiliar | familiar | owned`, default familiar) carries the same semantics as fdesign: it changes explanation depth, never rigor.
- **The user's candidate is a first-class option.** Evaluate it under the user's premise — including changes to components the team owns. A rejection resting on "the current artifact doesn't do X" gets one clarifying question first.

## Phase 1 — Ground

fdesign Phase 1 discipline, scoped to what ALL candidate approaches need: verified facts with file:line anchors, real data inspected, exemplars named. Facts feed the constraint list and the evaluation — a claim without an anchor is an opinion.

- **Separate current gaps from architectural limits.** Evaluate each option both as-is and fully-implemented; the owner's implementation intent is an input, not something to infer from TODOs.
- **Live probe over document archaeology.** When a capability question blocks (vendor docs stale or unfetchable), propose the cheapest live experiment instead of continuing to search. (Origin: a 5-minute live URL-map import settled what hours of doc-hunting could not.)
- **Design by measurement.** When an option hinges on a data distribution, write the runnable query and hand it to the user; the result becomes a binding fact.

## Phase 2 — Constraints

Enumerate the hard constraints first — platform, language, data shapes, deploy topology, compatibility contracts. Each constraint gets an ID (`C<n>`) and an anchor or measurement. The constraints define the space; options are derived from them, not brainstormed freely.

## Phase 3 — Solution families

Derive the exhaustive set of families the constraints admit. Per family: name, mechanism in one paragraph, which constraints bind or kill it, what it assumes will be changed (and who owns that). A family that violates a hard constraint is listed with its killing constraint — visible, not silently dropped.

## Phase 4 — Evaluation

Table: `Option | Groundedness | Blast radius | Effort | Reversibility | Verdict`.

- Groundedness = how much existing code/pattern it rides (the criterion that decided the bake-off).
- Per-scenario verdicts are allowed: when a subset of requirements fits a "losing" option, say so — no winner-take-all. (Origin: "The sec scenarios are however brunoable.")
- Two options within noise of each other → both go to the recommendation as an OPEN decision. Never fake a winner.

## Phase 5 — Recommendation & handoff

Write `plans/{slug}/design/exploration.md`:

```markdown
# <feature> — Exploration

## Context
<the open question, drivers, mode — max 5 bullets>

## Constraints
<table: ID | Constraint | Source (anchor/measurement)>

## Options
<per family: mechanism, binding constraints, ownership assumptions>

## Evaluation
<the Phase 4 table + per-scenario notes>

## Recommendation
<chosen option (or OPEN with both finalists); what fdesign imports: the option, its binding constraints, its measurements>

## Rejected
<one line per rejected option with the killing reason — the refuted register, so nobody re-explores them>

## Changelog
<Date | Trigger | What changed — per style/plan.md>
```

The recommendation is input to /fdesign, recorded there as a `[USER]` decision citing this doc once the user confirms it.

## Self-check gate

- [ ] Every option's verdict traces to constraints or anchored facts — no vibes-based rankings.
- [ ] The user's candidate was evaluated under the user's premise, with the like-for-like comparison shown.
- [ ] Rejected register complete: every family that was considered appears, with its reason.
- [ ] No implementation planning leaked in — file-level precision belongs to fdesign.
- [ ] Artifact in English; passes style/plan.md presentation rules.

## Stop conditions

The `ALWAYS-EXEC-*` entries from `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md` apply. Explore-specific:

1. The concept still has open questions that change the solution space → stop; run /clarify first.
2. Two finalists within noise → stop ranking; present both as an OPEN decision with the trade-off.
3. An option requires re-scoping the concept's goals → stop and report; that is the user's call.

## Model

- Suggested: frontier / large
- Reason: open solution-space survey needs breadth + judgment
- Tested unviable: — (none yet)

## Changelog

- v1.6 (2026-07-31): activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries
- v1.5 (2026-07-30): context redesign — planning assets via ../fdesign/assets/, doctrine entries under rules/
- v1.4 (2026-07-26): Args section
- v1.3 (2026-07-24): renamed feature-explore → fexplore (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.2 (2026-07-16): exploration.md moves to plans/{slug}/design/
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-10): initial version
