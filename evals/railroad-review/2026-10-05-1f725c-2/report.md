# railroad-review — eval 2026-10-05

- prompt: /railroad-review directions: code-style correctness tests security lanes: 2[opus:medium] scope: branch[b4981b88d52deaa69d6ed1d60eb51b57ead47ba0] exclude: sessions/ evals/ proposals/ plans/ spec: plans/v1-2-1/2026-08-24-language-style-settin…
- start commit: `1f725ced` in `/Users/devuser/GolandProjects/smine`
- auth: token Max-Private
- judge: `opus`

| Run | Smoke | Check pass | Wall | Main-loop active | Output tokens (all agents) | Cost | Permission denials (all agents) | Step misses | Major+ defects | Rank |
|---|---|---|---|---|---|---|---|---|---|---|
| opus-high-r1 | pass | — | 50m 25s | 9m 49s | 1.8M | $218.90 | 142 | **2%** (2) | — | — |

## Step misses

- `opus-high-r1` SKILL-RAILROADREVIEW-PROC-003 — Wrong-base heuristic (skipped): trace.md:17 — Diff was 7314+/749- lines over 178 files; no plausibility check of the base followed before reviewing.
- `opus-high-r1` SKILL-RAILROADREVIEW-PROC-005 — Confirm scope (partial): trace.md:69 — Inclusions/exclusions stated only in the final report, not up front (trace.md:10, trace.md:42 say nothing about scope).

## Smoke and variant failures

None.
