---
name: decision-support
description: Adjudicate an external position against the actual code/spec — structured verdict, read-only, never a fix. Trigger on /decision-support or "does he have a point" or "who's right".
author: Kevin Horst
version: 1.0
---

# Decision Support

Given an external position — a colleague's chat message (often German), a raw curl transcript, a review comment — plus one line of grounding and a closed verdict question, adjudicate the claim against the actual code or spec it concerns and return an independent, evidenced verdict. This is not a spec-conformance lookup: "what do you think" is answered with reasoning and a call, never a citation alone.

## When to use

**Use when:** a pasted external position needs adjudication against a closed verdict question — "does he have a point", "who's right", "is he wrong or am I wrong"; recurring "[Decision Support]" sessions.
**Don't use when:** evaluating a *change* against impact axes — /fimpact. Resolving a *concept's* open questions — /clarify. Hunting defects or verifying a change — /railroad-review. There is no closed question, only an open "thoughts?" — narrow it first (or state the implied question).
**Preconditions:** the external position pasted verbatim, one line of grounding context, and a closed verdict question.
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo).

## Intake

Three parts, all required:

1. **The position, verbatim** — the colleague's message, curl transcript, or review comment, pasted as-is. Immutable: quoted in the output, never paraphrased into something easier to beat.
2. **One line of grounding** — what the position concerns ("our rate-limit middleware on X").
3. **A closed verdict question** — "Does he have a point?", "Is he wrong or am I wrong?". An open "thoughts?" is not enough: narrow it, or proceed on a stated assumption of the implied question.

## Process

1. **Ground** — read the actual code/spec the position concerns before judging. No adjudication from memory.
2. **Adjudicate** — independent judgment with reasoning across whatever domain the question targets: API semantics, git topology, architecture calls. Candor, not hedged diplomacy — no "honest truth" framing.
3. **Answer** — resolve the closed question directly.

## Output

One structured adjudication, read-only:

1. **Where he's right** — points that hold, each with evidence (path:line or spec section).
2. **Where I'd disagree** — points that don't, each with evidence.
3. **Practical middle ground** — the actionable resolution.
4. **Verdict** — a direct answer to the closed question.

Then offer once: "Want me to apply?" Edits start only on an explicit "apply" / "make the adjustments" — never before.

## Rules

- Independent judgment with reasoning — a spec-conformance citation alone is never the answer.
- No flattery framing, no hedged diplomacy; candor, evidence, verdict.
- The pasted position is quoted, never paraphrased into a weaker form.
- The session stays read-only until an explicit apply.

## Limits

- Requires a closed question; an open "thoughts?" gets a request to narrow (or a stated assumption of the implied question).
- The apply step, once triggered, is scoped to exactly what the adjudication proposed — anything larger routes to the planning chain (/fdesign).

## Model

- Suggested: frontier / large
- Reason: independent cross-domain judgment on grounded evidence, resisting the framing of the pasted position
- Tested unviable: — (none yet)

## Changelog

- v1.0 (2026-07-19): initial version
