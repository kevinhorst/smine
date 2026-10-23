# railroad-review — eval 2026-10-05

- prompt: /railroad-review directions: code-style correctness tests security lanes: 2[opus:medium] scope: branch[b4981b88d52deaa69d6ed1d60eb51b57ead47ba0] exclude: sessions/ evals/ proposals/ plans/ spec: plans/v1-2-1/2026-08-24-language-style-settin…
- start commit: `1f725ced` in `/Users/devuser/GolandProjects/smine`
- auth: token Max-Private
- judge: `opus`

| Run | Smoke | Check pass | Wall | Main-loop active | Output tokens (all agents) | Cost | Permission denials (all agents) | Step misses | Major+ defects | Rank |
|---|---|---|---|---|---|---|---|---|---|---|
| opus-high-r1 | pass | — | 47m 01s | 9m 16s | 1.9M | $230.10 | 148 | **7%** (7) | — | — |

## Step misses

- `opus-high-r1` SKILL-RAILROADREVIEW-PROC-003 — Wrong-base heuristic (skipped): trace.md:15 — Diff is ~8000 lines over 178 files; no plausibility check of the base appears. Base happens to equal the merge-base, so no harm.
- `opus-high-r1` SKILL-RAILROADREVIEW-PROC-005 — Confirm scope (partial): trace.md:57 — Inclusions/exclusions first stated after the round launched (57) and in the final report (68), not up front.
- `opus-high-r1` SKILL-RAILROADREVIEW-MODES-005e — Filter bookkeeping (partial): trace.md:52 — Empty skipped list stated in chat (52, 68) but not in the review record (review-record.md).
- `opus-high-r1` SKILL-RAILROADREVIEW-PROBE-001 — Deterministic probes (partial): artifacts/round-1/review.json — 49 probes; several claims went manual, some because tool policy denied the validity/verdict runs.
- `opus-high-r1` SKILL-RAILROADREVIEW-PROBE-003 — Validity first (partial): artifacts/round-1/review.json — 4 probes (code-style NIT-190..193) carry no fixture.
- `opus-high-r1` SKILL-RAILROADREVIEW-DELIVER-001 — Plan-file record (partial): trace.md:66 — Record written to artifacts/review-record.md instead of a session plan file.
- `opus-high-r1` SKILL-RAILROADREVIEW-DELIVER-005 — Section structure (partial): artifacts/review-record.md:1 — Rendered via present --section, but the heading passed already began 'Review — ', giving a doubled 'Review — Review —' heading.

## Smoke and variant failures

None.
