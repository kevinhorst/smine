---
name: code-verdict
description: Verdict on a scoped piece of existing code — problem or fine — with alternative solutions only when a problem is confirmed. Trigger on /code-verdict or "analyze this code" or "is this code a problem, and what are the alternatives". Args — target: file/line anchor or pasted snippet; concern: suspected problem to test, optional.
author: Kevin Horst
version: 1.2
argument-hint: "[target] [concern]"
allowed-tools: Read, Grep, Glob, Write, Bash(grep *), Bash(git log *)
acdsl-context: ACTION-REVIEW-*, RULE-PLAN-*, FACT-*
---

# Code Verdict

This skill produces one artifact: `verdict.md` — a grounded verdict on whether a scoped piece of existing code has an actionable problem, followed by evaluated alternatives **only** when a problem is confirmed. A "no actionable problem" verdict is a complete, successful outcome — never padded with speculative alternatives. (Origin: the founding session asked for "analysis of a snippet, alternatives if there's a problem"; /fexplore was the wrong tool, and the verdict turned out to be "no problem — the snippet implements recorded decision D9 verbatim".)

Static constraints are not restated here: `AGENTS.md` and the style guides under `$AGENT_CONTEXT_DIR_DEFAULT/` govern the artifact. Missing context docs: state it once, continue with built-in baselines — never stall.

## When to use

**Use when:** an existing piece of code (function, snippet, few lines) needs a problem-or-fine verdict, with alternatives if a problem is confirmed — "analyze this code", "is this OK", "if this is wrong, what else could we do". No pending change, no external position, no observed failure.
**Don't use when:** evaluating a proposed or pending *change* — /fimpact. Adjudicating an external position against a closed question — /support-decision. Diagnosing an observed failure or symptom — /diagnose-debug. The solution space is feature-level and open (concept exists, space contested) — /fexplore. Reviewing a branch/PR — /railroad-review.
**Preconditions:** an identifiable code target — `path:line-range` in a reachable repo, or a pasted snippet plus the repo it lives in.
**Workflow position:** standalone, on demand; hands off to /fexplore when the alternatives space turns out feature-level, and to /fdesign or /fimplement when the user accepts a recommended alternative (see README.md § Skill map, smine repo).

## Args

- `target`: positional — the code under verdict: a `path:lines` anchor, or a pasted snippet plus the repo it lives in.
- `concern`: optional — the suspected problem to test (e.g. "TOCTOU race"). A hypothesis to verify, never a pre-accepted verdict.

## 1. Intake

**SKILL-CODEVERDICT-INTAKE-001** `[step]` — Resolve `target` to real lines on disk; a pasted snippet is matched to its file before anything else. No verdict on code that was not read from the repo.

**SKILL-CODEVERDICT-INTAKE-002** `[step]` — Enumerate the call sites and the contracts the code participates in (callers, exceptions raised, API/error-code contracts, DB writes it guards).

**SKILL-CODEVERDICT-INTAKE-003** `[step]` — Locate the owning feature plan tree (`plans/{slug}/`) when one exists — its design decisions and verification checklists are evidence for the verdict and the destination for the artifact.

**SKILL-CODEVERDICT-INTAKE-004** `[step]` — Treat the user's `concern` as a hypothesis to verify against evidence, never as a pre-accepted verdict.

## 2. Ground

**SKILL-CODEVERDICT-GROUND-001** `[step]` — Every claim gets a `path:line` anchor (or measurement); a claim without an anchor is an opinion and does not enter the verdict.

**SKILL-CODEVERDICT-GROUND-002** `[gate]` — Check recorded design decisions covering the target lines before declaring a problem. A snippet implementing a recorded decision verbatim is not a defect — the verdict cites the decision and evaluates only whether the decision's stated premise still holds.
* Why: the founding session's "suspicious" error-code literal and TOCTOU window were both recorded, deliberate decisions; flagging them as defects would have been noise.

**SKILL-CODEVERDICT-GROUND-003** `[step]` — Capability questions the repo cannot answer (e.g. "does the DB actually have that unique index") follow the fexplore measurement discipline: write the runnable query or probe, hand it to the user, and mark the point `unverified` — never claim it from adjacent artifacts like migration files.

## 3. Verdict

**SKILL-CODEVERDICT-VERDICT-001** `[gate]` — Each examined point gets exactly one classification: `problem confirmed` | `no actionable problem` | `unverifiable without measurement`, with its evidence. No hedged in-betweens.

**SKILL-CODEVERDICT-VERDICT-002** `[gate]` — No point classified `problem confirmed` → write `verdict.md` with the verdict and per-point reasoning and **stop**. The alternatives stage never runs on a clean verdict.

## 4. Alternatives (only on a confirmed problem)

**SKILL-CODEVERDICT-ALT-001** `[step]` — Follow fexplore Phase 2–4 discipline scoped to the snippet: hard constraints with anchors first, then the option families the constraints admit — always including "keep as is, accept the cost" — then the evaluation table `Option | Groundedness | Blast radius | Effort | Reversibility | Verdict`, then a recommendation. Two options within noise → both go to the recommendation as OPEN; never fake a winner.

**SKILL-CODEVERDICT-ALT-002** `[gate]` — If the space turns out feature-level or contested (needs concept-level decisions), stop and hand off to /fexplore explicitly instead of half-surveying it; the verdict and the handoff line are the deliverable.

## 5. Output

**SKILL-CODEVERDICT-OUT-001** `[step]` — Write to `plans/{slug}/verdict.md`, where `{slug}` is the owning feature's slug when the target belongs to an existing plan tree, else a new kebab slug named after the target (e.g. `plans/formsv1-pending-email-check/verdict.md`).

**SKILL-CODEVERDICT-OUT-002** `[gate]` — Read-only beyond that one file: a verdict and alternatives, never a fix. Edits start only on an explicit user go, routed per the Workflow position.

**SKILL-CODEVERDICT-OUT-003** `[payload]` — `verdict.md` template:

```markdown
# <target> — Verdict

## Context
<the target, the concern (if any), how the code was scoped — max 5 bullets>

## Evidence
<anchored facts: call sites, contracts, recorded decisions, measurements; unverified points marked>

## Verdict
<per examined point: classification + one paragraph of reasoning; then one overall line>

## Alternatives
<only when a problem was confirmed: constraints table, option families, evaluation table, recommendation — or the explicit /fexplore handoff>

## Handoff
<next step: none (clean verdict), pending measurement, /fexplore, or /fdesign–/fimplement on the recommended option>
```

## Model

- Suggested: frontier / high
- Reason: grounded independent judgment plus scoped solution-space evaluation
- Tested unviable: — (none yet)
