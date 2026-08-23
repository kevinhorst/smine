# concept — skill-run evaluation (2026-08-18)

Skill `concept` (`skills/concept/concept/SKILL.md`), 8 runs on the `login_intent` brief: 2 variants (`default`, `no-closing` = `SKILL-CONCEPT-CLOSING-*` stripped) × 2 context modes (ctx-on / ctx-off) × 2 replicas, all `claude-opus-4-8[1m]` / high.

Three axes are scored separately (+1 followed / 0 not-followed-or-not-demonstrable / −1 violated). **Self** = the skill's own non-payload entries (20 for `default`, 18 for `no-closing`). **Context** = the 10 entries the run received by injection. **Output-quality** is N/A — `qualityContext` was empty, so there are no `C` rules; numeric metrics stand in.

## Headline

- **Self axis is flat-to-noisy across the matrix.** 11 of the 18 shared self rules are +1 for every run. All variation comes from two review guidelines — tables (WRITING-002) and per-block totals (WRITING-007) — plus the `default`-only closing gate.
- **Context axis is non-discriminating: 0/10 for all 8 runs.** The concept skill's declared context set is code-writing HOT gates (SQL CTEs, concurrency, generics, guard logic) plus `claude-configs` repo-mechanics facts. None of it has an application surface in a login-redirect product concept, so ctx-on runs had no occasion to honor it and ctx-off runs never received it. Context injection made no observable difference on this task.
- **The `no-closing` variant is indistinguishable from `default` on shared rows** (the −4.17 pp shared-self gap is an artifact of WRITING-007 landing in 3/4 `default` runs vs 1/4 `no-closing`, not of the CLOSING removal). On full totals, `default` runs pick up a clean +1 each from correctly skipping the `/clarify` offer.

## Self axis — full totals (per variant, different denominators)

| Run | raw | max | pct |
| --- | --- | --- | --- |
| default.ctx-on.r2 | 14 | 20 | 70.00 |
| default.ctx-off.r1 | 13 | 20 | 65.00 |
| default.ctx-off.r2 | 13 | 20 | 65.00 |
| default.ctx-on.r1 | 12 | 20 | 60.00 |
| no-closing.ctx-off.r2 | 12 | 18 | 66.67 |
| no-closing.ctx-on.r1 | 11 | 18 | 61.11 |
| no-closing.ctx-on.r2 | 11 | 18 | 61.11 |
| no-closing.ctx-off.r1 | 11 | 18 | 61.11 |

## Self axis — shared rows only (18 rows, cross-variant comparable)

| Rank | Run | raw/18 | pct |
| --- | --- | --- | --- |
| 1 | default.ctx-on.r2 | 13 | 72.22 |
| 2 | default.ctx-off.r1 | 12 | 66.67 |
| 2 | default.ctx-off.r2 | 12 | 66.67 |
| 2 | no-closing.ctx-off.r2 | 12 | 66.67 |
| 5 | default.ctx-on.r1 | 11 | 61.11 |
| 5 | no-closing.ctx-on.r1 | 11 | 61.11 |
| 5 | no-closing.ctx-on.r2 | 11 | 61.11 |
| 5 | no-closing.ctx-off.r1 | 11 | 61.11 |

**Variant delta (no-closing − default), shared-self mean:** 62.50 − 66.67 = **−4.17 pp**. Attribution: WRITING-007 (block totals) was +1 in 3/4 `default` runs vs 1/4 `no-closing` runs — a replica-level coincidence at n=4, not an effect of removing the closing gate (CLOSING rows are excluded from the shared set).

## Variant-only rows (`default` only; stripped in `no-closing`)

| Rule | default runs | Score | Note |
| --- | --- | --- | --- |
| SKILL-CONCEPT-CLOSING-001 (offer `/clarify` when Open Questions remain) | all 4 | 0 | Not demonstrable — headless run has no interactive user to receive the offer; each run explicitly acknowledged the gate. |
| SKILL-CONCEPT-CLOSING-002 (skip the offer when scoped to drafting-only) | all 4 | +1 | Each run correctly skipped, citing the unattended/drafting-only scope verbatim. |

## Context axis — all runs

| Run | raw | max | pct |
| --- | --- | --- | --- |
| all 8 runs | 0 | 10 | 0.00 |

Every context cell is 0. ctx-on runs (all four) received the identical 10-entry set `[ACTION-CONCEPT-HOT-001..006, FACT-REPO-ARCH-001..003, FACT-REPO-STACK-001]` (per `context_record.sh`); ctx-off runs received none. Neither honored-demonstrably nor violated — the set is inapplicable to a product concept.

## Metrics (never folded into rule totals)

| Run | wall_s ↓ | output_tokens ↓ | files ↑ | citations ↑ | audit_pass | diff_lines |
| --- | --- | --- | --- | --- | --- | --- |
| default.ctx-on.r1 | 102 | 7120 | 3 | 0 | n/a | n/a |
| default.ctx-on.r2 | 113 | 7319 | 3 | 0 | n/a | n/a |
| no-closing.ctx-on.r1 | 93 | 6270 | 3 | 0 | n/a | n/a |
| no-closing.ctx-on.r2 | 97 | 6529 | 3 | 0 | n/a | n/a |
| default.ctx-off.r1 | 115 | 7314 | 3 | 0 | n/a | n/a |
| default.ctx-off.r2 | 117 | 7229 | 3 | 0 | n/a | n/a |
| no-closing.ctx-off.r1 | 103 | 7007 | 3 | 0 | n/a | n/a |
| no-closing.ctx-off.r2 | 109 | 7424 | 3 | 0 | n/a | n/a |

- **wall_s / output_tokens** (lower better): fastest and leanest is `no-closing.ctx-on.r1` (93 s, 6270 tok); slowest is `default.ctx-off.r2` (117 s). The two `no-closing.ctx-on` cells are the cheapest pair, consistent with a shorter loaded skill, but the spread is small.
- **files_touched** = 3 for all (concept.md + user_stories.md + one detailed-design page), counted from each artifact's `## file:` sections.
- **entry_citations** = 0 for all: concept output is product prose and cites no `SKILL-…-NNN` ids.
- **audit_pass / diff_lines** = null: the cell worktrees are outside accessible roots and a concept run makes no code change, so `make audit` / `git numstat` were not collectible.

## Non-+1 justification summary

Grouped by rule (identical justification across the listed runs). Full per-cell justification, evidence, and source live in `eval.json`.

| Rule | Score | Runs | Why |
| --- | --- | --- | --- |
| SKILL-CONCEPT-MODEB-001..004 | 0 | all 8 | Mode B (refine an existing concept) not exercised — every run did a Mode A fresh create, so the refine-path rules have no occasion to be demonstrated. |
| SKILL-CONCEPT-WRITING-005 | 0 | all 8 | No `service-map.md` applies to this greenfield login feature; services are referenced only generically. |
| SKILL-CONCEPT-WRITING-002 | 0 | 7 (all but default.ctx-on.r2) | Limits / Model fields / validation rendered as bullet lists, not tables. Payload template SKILL-CONCEPT-TPL-002 itself prescribes bullets for these sections — a skill-internal template-vs-guideline tension, not solely a run choice. |
| SKILL-CONCEPT-WRITING-007 | 0 | 4 (default.ctx-on.r1, no-closing.ctx-on.r1/r2, no-closing.ctx-off.r1) | Per-item day estimates present but no per-block total stated; the rule requires both. |
| SKILL-CONCEPT-CLOSING-001 | 0 | default×4 | Headless — no interactive user to receive the `/clarify` offer. |
| ACTION-CONCEPT-HOT-* / FACT-REPO-* (context) | 0 | all 8 | Inapplicable to a product concept (ctx-on) or never injected (ctx-off). |

## Mechanical probes

- **context-injection** — all 4 ctx-on runs received the identical 10-entry set; all 4 ctx-off runs received zero.
- **context-applicability** — the injected set is code-writing doctrine + repo-mechanics facts with no purchase on a login-redirect concept; context axis is non-discriminating (0/10 everywhere).
- **tables-vs-bullets** — 1/8 runs rendered a markdown table; TPL-002 prescribes bullets, in tension with WRITING-002.
- **block-totals** — per-item estimates in all 8; explicit block totals in 4 (default.ctx-on.r2, default.ctx-off.r1, default.ctx-off.r2, no-closing.ctx-off.r2).
- **closing-gate** — all 4 `default` runs correctly skipped the offer citing unattended scope (CLOSING-002 +1); none could extend an interactive offer (CLOSING-001 not demonstrable).
- **self-report-vs-artifact** — `default.ctx-off.r2`'s closing summary claims a "validation rules and limits table" its artifact does not contain (bullets only); scored on the artifact.
- **missing-shared-input** — the manifest input `routines/skill-eval/briefs/concept/login_intent.md` was absent from the worktree and reachable roots; brief content was inferred from verbatim references in the outputs, so the contradiction-audit and assumptions-novelty probes could not be run mechanically.

## Reader's caveats

- n = 2 replicas per cell: single-rule swings (WRITING-002, WRITING-007) move a run by ~5 pp and dominate the ranking. Treat cross-cell differences under ~1 rule as noise.
- The self axis rewards template-shaped completeness, not concept quality; all 8 outputs are substantively strong, near-equivalent login-intent concepts. The scored gaps are stylistic (tables, totals) plus the structural closing gate.
- Output-quality axis (content correctness vs a style/DoD guide) was not evaluated — no `qualityContext` was supplied.
