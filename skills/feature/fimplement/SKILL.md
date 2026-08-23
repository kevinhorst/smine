---
name: fimplement
description: Carry a change to working, committed code — an /fdesign plan or findings doc binds as contract, else best-effort under the repo's context. Trigger on /fimplement [plan file or task]. Args — plan file: plan, findings doc, or task (absent = best-effort); errata: invocation feedback superseding the plan; caveman: compress prose, requires caveman skill.
author: Kevin Horst
version: 1.23
argument-hint: "[plan file] [errata] [caveman]"
acdsl-context: ACTION-CONCEPT-*, ACTION-IMPL-*, ACTION-REVIEW-*, RULE-COMMIT-*, FACT-*
allowed-tools: Bash(go build *), Bash(go test *), Bash(go vet *), Bash(gofmt *), Bash(make build *), Bash(make test *), Bash(make generate *), Bash(git add *), Bash(git commit -m *), Bash(git diff *), Bash(git log *), Bash(jq *), Read, Write, Edit
---

# fimplement

This skill produces working, committed code — and is the marked entrypoint for "implementation started", so a session can later be mined for what got built. It runs from an approved plan when one exists — then the plan is a binding contract down to exact signatures, contracts, and architecture, and this skill does not redesign it — and best-effort under the repo's context and code-style when none does. Autonomy is the default: a missing contract is not an obstacle, it selects best-effort mode; proceed and adhere rather than halt. A genuine obstacle — a provided contract that can't hold, a [USER] decision contradicted — still triggers STOP and report, never a silent deviation.

Static constraints are not restated here: `AGENTS.md`, the format guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (incl. `commits.md`), and the doctrine chapters under `$AGENT_CONTEXT_DIR_DEFAULT/actions/` (concepting — hot-class gates; implementing — stops, integrity; reviewing — Definition of Done) govern every edit. They arrive via this skill's frontmatter `acdsl-context:` declaration, injected at invocation by the skill-context hook; per-language style guides are read from `$AGENT_CONTEXT_DIR_DEFAULT/rules/` on demand. Re-read entries from `$AGENT_CONTEXT_DIR_DEFAULT/` only when they are no longer in context. The checkable slice of these constraints is enforced by ACDSL gates — in ACDSL repos (`acdsl/registry.json` present), green `go run ./cmd/acdsl check` is part of done, and when the plan shipped a task contract, so is green `check -lifetime task`; for anything a gate covers, the gate is authoritative, not the prose.

## When to use

**Use when:** any change needs to become working, committed code — with a plan (/fdesign on the new or change route, or a review-findings doc) or without one. It runs the plan as a binding contract when given, and implements best-effort under context + code-style when not.
**Don't use when:** you want a reviewed design before any code — run /fdesign (new or change route) first; but fimplement no longer refuses a bare task. The plan needs revising first — /fdesign refine (optional; fimplement will otherwise proceed best-effort).
**Preconditions:** none required — a plan sharpens the target but is optional.
**Workflow position:** fdesign (refine route optional) → **fimplement** → package-commit; delegates verification to /verify and commits via /package-commit (see README.md § Skill map, smine repo).

## Args

- plan file (optional): positional — an /fdesign plan (new or change route), a review-findings doc, or a task description, pasted or by path. Honored as a binding contract when it defines one; absent → implement best-effort under context + code-style, never a STOP.
- errata: feedback or corrections attached at invocation — they supersede the written plan; contradictions become clarifying questions.
- `caveman`: requires the caveman skill installed, else STOP — the style is never approximated.

## Phase 0 — Intake

- **Take whatever input is given** — an /fdesign plan (new or change route), a review-findings doc, or a bare task. No input is not a stop: the absence of a contract selects best-effort mode, bounded by the context docs (AGENTS.md, `$AGENT_CONTEXT_DIR_DEFAULT/rules/`, style guides) and the nearest in-repo patterns.
- **When a plan is present, read the whole thing first**, not just the Changes section: Decisions (binding, `[USER]` rows especially), Behavior contract, Hot items, Tests, Test runbook, Contracts & sweeps, Verification, Stop conditions. These travel into implementation and bound it. The Test runbook is executed only when opted in: the plan carries full request files (its args line shows `runbook`) or the user asks at implementation — then the files are persisted to `plans/<feature>/runbooks/` and run as part of verification; a scenario-index-only section is informational and executes nothing. A review-findings doc gives one unit per confirmed finding (file:line + proposed fix); rejected/ledgered findings are out of scope.
- **A provided plan with open questions / OPEN rows is not a stop either** — implement what is settled and make the reasonable call on the rest, guided by context, noting each call in the report. (/fdesign refine remains available when you would rather resolve them as design first.)
- **User errata supersede the plan.** Feedback or corrections the user attaches at invocation override the written plan; where errata collide with a `[USER]` decision, surface it (stop 10), otherwise take the errata and note the call.
- `caveman` requires `~/.claude/skills/caveman` — STOP with "caveman requested but skill not installed" if missing; the style is never approximated.

## Phase 1 — Ground

Re-check every claim against the real tree before writing anything — Explore and subagent findings are re-verified against actual files, never trusted as-is.

1. **Verify the touch points.** With a plan: every file, symbol, and prerequisite it names exists as described (path:line); a claim that does not hold is a plan defect — STOP and report (ACTION-IMPL-001), do not patch around it. Without a plan: locate the code the task targets and the nearest sibling patterns directly, and ground the change against those.
2. **Prerequisites present** — generated code, migrations, running infra the plan assumes. A missing producer step is run; down infrastructure is asked about, never started (ACTION-IMPL-003).
3. **Exemplar per unit** — for each new unit, confirm the nearest sibling the plan points to still exists; new code mirrors that sibling's architecture (AGENTS.md rule).

## Phase 2 — Implement

Build in dependency order, **one logical unit at a time** — never a godfile, "each endpoint, one at a time." Per unit:

1. **Write** the unit to the plan's exact signature and contract when a plan defines one, else to the shape the surrounding code and context dictate; new code mirrors its nearest sibling.
2. **Gate** — build plus the unit's tests green before moving on. The plan's test spec (per-case fields as struct members, run loop call+assert only) is honored; standing code rules the plan-review checklist enumerates are not re-violated in the implementation.
3. **Advance** — next unit in dependency order.

For refactor cascades and renames in compiled code, **drive with the compiler**: change the core type or signature → build → fix each reported error → test → green. No pre-enumeration theater where the compiler already knows the touch list; intermediate commits need not each build if the branch ends green.

**Two-attempt ceiling per unit.** Max two attempts at any one logic unit. After the second rejection: STOP generating, state where the code stands and which constraint is unsolved, hand over (ACTION-IMPL-002). When the human writes their version, review it against the plan's Decisions and Stop conditions — that reverse pass has caught real bugs.

## Phase 3 — Verify

- **Verify the observable path in the running app**, not only unit tests — the Definition of Done's End-to-End Verification: happy path plus the plan's edge and error cases, exercised against a running system with real values.
- Delegate the running-app check to /verify; when the project needs a local e2e stack first, /dev-stack stands it up. This skill does not stand up infrastructure itself (ACTION-IMPL-003).
- A behavior the plan's Verification checklist lists but no test or run exercises is a gap — report it, do not mark done.

## Phase 4 — Commit

- **Stop every background process this session started** — dev servers, port listeners, watchers — before offering to commit or hand off, and verify no orphaned listeners remain on the ports used. A left-running listener is part of the definition of done, not the user's cleanup.
- Hand off to /package-commit — build, test, and commit changed files grouped by package. This skill does not hand-roll commit grouping.
- Report against the plan: each unit built, its gate result, the running-app verification outcome, and any plan defect surfaced and reported rather than worked around.

## Delegation

Relay data for /delegate — the mechanism (spawn, result contract, relay loop, respawn fallback) lives in the delegate skill; this skill never self-delegates. A delegated runner executes Phases 1–4 unattended.

- **Spawn prompt inlines, in order:**
  1. the `ACTION-IMPL-*` gate entries from `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md` verbatim, plus skill conditions 8–10 below — the runner never sees the files otherwise;
  2. the discipline: with a plan, the binding-contract rules (plan is a contract, exact signatures, no silent deviation); without one, best-effort under the context docs and code-style. Two-attempt ceiling per unit either way;
  3. the gate instruction: on any relay-class gate (stops 1, 2, 8, 9, 10) stop work immediately and return the blocked-state result — do not continue other units past such a gate; returning `blocked` is success. A missing/contract-light input is not a gate — proceed;
  4. the plan/findings-doc/task and any user errata;
  5. chained skills (/verify, /package-commit) are invoked directly — no nested delegation, never via /delegate.
- **Relay-class gates:** stop conditions 1, 2, 8, 9, 10 (8 and 9 fire only when a plan defines the contract/scope).
- **Attempt-ceiling relay semantics:** the runner self-counts attempts; at the ceiling it returns `blocked` with both attempts' diffs/locations and the unsolved constraint. On "my version is in — continue", the ruling tells the runner to read the user's version, learn from it, and proceed — the user's version closes that unit's counter, it is not attempt 3.

## Stop conditions

The `ACTION-IMPL-*` gate entries from `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md` apply verbatim — they travel from the plan into this run. Skill-specific:

7. A missing or contract-light input is **not** a stop — implement best-effort under the context docs and code-style, noting the calls made; a provided plan's open questions / OPEN rows are implemented best-effort too, not routed back. (Only the genuine blockers below stop the run.)
8. The plan's contract can't be honored as written — a signature, key format, or architecture the tree won't support → STOP and report the defect. Never substitute a different mechanism, demote a `[USER]` decision to "deferred", or invent architecture mid-edit.
9. Implementation reveals work materially beyond a provided plan's scope → stay in scope; a self-contained necessary adjacent change is made and noted, a change that would rework the designed architecture STOPs and reports. (No plan → the task's stated intent is the scope.)
10. A driver or discovery contradicts a `[USER]` decision in the plan → surface the conflict; never silently override a user decision.

## Model

- Suggested: frontier / medium
- Delegation: gated
- The allowed-tools manifest covers mechanics only — target-repo payload commands still prompt; a full promptless run is not promised.
- Runner: implement-runner
- Reason: implementation discipline — contract-honoring when a plan exists, context-and-style-guided when not — plus multi-unit builds and compiler-driven cascades
- Tested unviable: — (none yet)
