---
name: fimpact
description: Evaluate the consequences of a given change against qualitative axes — maintainability/understanding/complexity, security, business impact, and any axis named at intake. The output is a per-axis assessment with a verdict, never a fix, never a defect hunt. Trigger on "what are the consequences/impact of this change", "evaluate this change", or before committing to a contested change.
author: Mustafa Karademir
version: 1.2
---

# fimpact

Given a change — a diff, a plan, or a described change — evaluate it thoroughly against impact axes. This is not a breakage check: whether the change works is the review's job; this skill answers what the change does to the system and the product.

## When to use

**Use when:** weighing a proposed or pending change; a change is contested and needs a structured assessment; answering "what are the consequences of X".
**Don't use when:** hunting defects or verifying the change doesn't break anything — /railroad-review. Designing the change — /fdesign. Diagnosing an existing failure — /diagnose-debug.
**Preconditions:** an identifiable change: a diff, a plan, or a precise description of what would change.
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo).

## Args

- change: positional — the change to evaluate — a diff, a plan, or a precise description.
- axes: additional impact axes named at intake beyond the built-in maintainability / security / business (e.g. performance, operations, compliance).

## Axes

| Axis | Question | Evidence |
| :--- | :--- | :--- |
| `maintainability` | Does the system get easier or harder to understand and change? | New concepts introduced vs removed<br>ownership/layering shifts<br>parallel mechanisms left alive |
| `security` | Does the attack surface grow or shrink? | New inputs, endpoints, credentials, trust boundaries<br>weakened guards |
| `business` | What does the product gain, lose, or risk? | Affected user flows/stories<br>behavior changes users can observe<br>operational cost |
| custom (named) | The user names further axes at intake (performance, operations, compliance, …) | Stated at intake |

## Process

1. **Intake** — the change, plus any custom axes; read the diff/plan and the code it lands in.
2. **Evaluate per axis** — ground each judgment in specific evidence (path:line or plan section); no axis is skipped: an axis the change does not touch gets `neutral — <reason>`.
3. **Verdict** — per axis: better | worse | neutral | mixed, one paragraph of reasoning; then an overall verdict naming the axis that dominates.

## Output

One report: per-axis sections in table order, each with verdict + evidence; a final `Overall` line. Read-only — no fixes, no plan, no defect list.

## Model

- Suggested: frontier / large
- Reason: cross-axis judgment on repo-wide evidence
- Tested unviable: — (none yet)

## Changelog

- v1.2 (2026-07-26): Args section
- v1.1 (2026-07-24): renamed feature-impact → fimpact (leaf dir, frontmatter name, H1); live references swept repo-wide
- v1.0 (2026-07-16): initial version
