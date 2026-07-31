---
name: fimplement
description: Execute an approved implementation plan to completion as a binding contract — working, committed code. Trigger on /fimplement <plan file>. Args — plan file: approved /fdesign or /fchange plan, absent means STOP; errata: invocation feedback superseding the plan; caveman: requires caveman skill, else STOP.
author: Kevin Horst
version: 1.13
allowed-tools: Bash(go build *), Bash(go test *), Bash(go vet *), Bash(gofmt *), Bash(make build *), Bash(make test *), Bash(make generate *), Bash(git add *), Bash(git commit -m *), Bash(git diff *), Bash(git log *), Bash(jq *), Read, Write, Edit
---

# fimplement

This skill consumes one artifact — an approved implementation plan — and produces working, committed code. The plan is a binding contract down to exact signatures, contracts, and architecture; this skill does not redesign it. The single most important rule: any obstacle triggers STOP and report, never a silent deviation. Executing near-flawlessly and reporting the plan's one defect beats improvising around it.

Static constraints are not restated here: `AGENTS.md`, the style guides under `$AGENT_CONTEXT_DIR_DEFAULT/style/` (incl. `commits.md`), and the doctrine entries under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (concepting — hot-class gates; implementing — stops, integrity; reviewing — Definition of Done) govern every edit. They are usually injected by the review-context hook — look for the `===== REVIEW CONTEXT (injected by hook) =====` header before re-reading them.

## When to use

**Use when:** an approved /fdesign or /fchange plan is ready to build — run it to completion as a binding contract, unit by unit, gated and committed.
**Don't use when:** there is no approved plan — route to /fdesign (new feature) or /fchange (change to an existing feature) first; never start from a vague description. The plan needs revising before implementation — /fdesign refine.
**Preconditions:** an approved plan (pasted or by path) whose Decisions and Changes are settled — Open questions empty.
**Workflow position:** fdesign (refine route optional) → **fimplement** → package-commit; delegates verification to /verify and commits via /package-commit (see `docs/skill-map.md`, smine repo).

## Args

- plan file: positional — the approved /fdesign or /fchange plan, pasted or by path; absent → STOP and route to /fdesign or /fchange.
- errata: feedback or corrections attached at invocation — they supersede the written plan; contradictions become clarifying questions.
- `caveman`: requires the caveman skill installed, else STOP — the style is never approximated.

## Phase 0 — Intake

- Name the **plan file** — the approved output of /fdesign or /fchange, pasted or by path. Without one, STOP and route to /fdesign or /fchange (stop condition 7).
- **Refuse a moving target.** A plan with a non-empty Open questions section, or unratified OPEN decision rows, is not approved — STOP and route to /fdesign refine.
- **Read the whole plan first**, not just the Changes section: Decisions (binding, `[USER]` rows especially), Behavior contract, Hot items, Tests, Test runbook, Contracts & sweeps, Verification, Stop conditions. These travel into implementation and bound it; the Test runbook's files are persisted to `plans/<feature>/runbooks/` and executed as part of verification.
- **User errata supersede the plan.** Feedback or corrections the user attaches at invocation override the written plan; a contradiction between them becomes a clarifying question, never a silent judgment call.
- `caveman` requires `~/.claude/skills/caveman` — STOP with "caveman requested but skill not installed" if missing; the style is never approximated.

## Phase 1 — Ground

Re-check the plan's factual claims against the real tree before writing anything — Explore and subagent findings are re-verified against actual files, never trusted as-is.

1. **Verify the touch points** — every file, symbol, and prerequisite the plan names exists as described (path:line). A claim that does not hold is a plan defect: STOP and report (ALWAYS-EXEC-001), do not patch around it.
2. **Prerequisites present** — generated code, migrations, running infra the plan assumes. A missing producer step is run; down infrastructure is asked about, never started (ALWAYS-EXEC-003).
3. **Exemplar per unit** — for each new unit, confirm the nearest sibling the plan points to still exists; new code mirrors that sibling's architecture (AGENTS.md rule).

## Phase 2 — Implement

Build in dependency order, **one logical unit at a time** — never a godfile, "each endpoint, one at a time." Per unit:

1. **Write** the unit to the plan's exact signature and contract; new code mirrors its named sibling.
2. **Gate** — build plus the unit's tests green before moving on. The plan's test spec (per-case fields as struct members, run loop call+assert only) is honored; standing code rules the plan-review checklist enumerates are not re-violated in the implementation.
3. **Advance** — next unit in dependency order.

For refactor cascades and renames in compiled code, **drive with the compiler**: change the core type or signature → build → fix each reported error → test → green. No pre-enumeration theater where the compiler already knows the touch list; intermediate commits need not each build if the branch ends green.

**Two-attempt ceiling per unit.** Max two attempts at any one logic unit. After the second rejection: STOP generating, state where the code stands and which constraint is unsolved, hand over (ALWAYS-EXEC-002). When the human writes their version, review it against the plan's Decisions and Stop conditions — that reverse pass has caught real bugs.

## Phase 3 — Verify

- **Verify the observable path in the running app**, not only unit tests — the Definition of Done's End-to-End Verification: happy path plus the plan's edge and error cases, exercised against a running system with real values.
- Delegate the running-app check to /verify; when the project needs a local e2e stack first, /dev-stack stands it up. This skill does not stand up infrastructure itself (ALWAYS-EXEC-003).
- A behavior the plan's Verification checklist lists but no test or run exercises is a gap — report it, do not mark done.

## Phase 4 — Commit

- Hand off to /package-commit — build, test, and commit changed files grouped by package. This skill does not hand-roll commit grouping.
- Report against the plan: each unit built, its gate result, the running-app verification outcome, and any plan defect surfaced and reported rather than worked around.

## Delegation

Relay data for /delegate — the mechanism (spawn, result contract, relay loop, respawn fallback) lives in the delegate skill; this skill never self-delegates. A delegated runner executes Phases 1–4 unattended.

- **Spawn prompt inlines, in order:**
  1. the `ALWAYS-EXEC-*` entries from `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md` verbatim, plus skill conditions 7–10 below — the runner never sees the files otherwise;
  2. the binding-contract rules: the plan is a contract, exact signatures, no silent deviation, two-attempt ceiling per unit;
  3. the gate instruction: on any relay-class gate (stops 1, 2, 8, 9, 10) stop work immediately and return the blocked-state result — do not continue other units past a contract-level gate; returning `blocked` is success;
  4. the plan file path and any user errata;
  5. chained skills (/verify, /package-commit) are invoked directly — no nested delegation, never via /delegate.
- **Relay-class gates:** stop conditions 1, 2, 8, 9, 10.
- **Attempt-ceiling relay semantics:** the runner self-counts attempts; at the ceiling it returns `blocked` with both attempts' diffs/locations and the unsolved constraint. On "my version is in — continue", the ruling tells the runner to read the user's version, learn from it, and proceed — the user's version closes that unit's counter, it is not attempt 3.

## Stop conditions

The `ALWAYS-EXEC-*` entries from `$AGENT_CONTEXT_DIR_DEFAULT/rules/implementing.md` apply verbatim — they travel from the plan into this run. Skill-specific:

7. No approved plan on intake, or the plan still has open questions / OPEN rows → STOP; route to /fdesign or /fdesign refine. Never implement from a vague description or an unratified plan.
8. The plan's contract can't be honored as written — a signature, key format, or architecture the tree won't support → STOP and report the defect. Never substitute a different mechanism, demote a `[USER]` decision to "deferred", or invent architecture mid-edit.
9. Implementation reveals work materially beyond the plan's scope → STOP and ask before continuing; every new concept must trace to the plan. "Keep it simple" scopes the implementation, never the designed architecture.
10. A driver or discovery contradicts a `[USER]` decision in the plan → surface the conflict; never silently override a user decision.

## Model

- Suggested: frontier / medium
- Delegation: gated
- The allowed-tools manifest covers mechanics only — target-repo payload commands still prompt; a full promptless run is not promised.
- Runner: implement-runner
- Reason: binding-contract discipline plus multi-unit implementation and compiler-driven cascades
- Tested unviable: — (none yet)

## Changelog

- v1.13 (2026-07-31): hot-class gates named under rules/concepting.md
- v1.12 (2026-07-31): activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries
- v1.11 (2026-07-30): context redesign — stop-conditions/hot-items via ../fdesign/assets/, commits via ../package-commit/assets/, doctrine entries under rules/
- v1.10 (2026-07-30): allowed-tools mechanics manifest declared; Command surface line retired
- v1.9 (2026-07-30): reference rename — design-refine → the fdesign refine route
- v1.8 (2026-07-30): reference rename — per-package-commit → package-commit
- v1.7 (2026-07-26): Args section
- v1.6 (2026-07-24): renamed feature-implement → fimplement (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.5 (2026-07-24): delegation explicit-only via /delegate — auto-intake check removed, relay procedure moved to the delegate skill, relay data kept in ## Delegation
- v1.4 (2026-07-24): Test runbook joins intake reading; runbook files persisted to plans/<feature>/runbooks/ and executed in verification
- v1.3 (2026-07-24): reference merge: feature-refine + feature-refactor → feature-change
- v1.2 (2026-07-22): gated delegation via implement-runner: intake check, gate relay + blocked-state handling, Model classification, effort medium
- v1.1 (2026-07-19): reference rename: refactor → feature-refactor; moved under skills/feature/
- v1.0 (2026-07-19): initial version
