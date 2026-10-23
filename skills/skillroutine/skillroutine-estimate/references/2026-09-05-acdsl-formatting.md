# Measured run: "Skill ACDSL formatting" (2026-09-05)

First real invocation of /skillroutine-estimate, followed live via peek. Target: an /fdesign run ("Skill ACDSL formatting" — merge main via merge-resolve, fix the merge-resolve branch prefix, add branch-prefix context rules, /fmt-migrate ~35 skills) on Fable-medium, later switched to Fable-low for implementation. Team Premium plan.

## Estimate revisions

| Revision | Basis | Chain estimate | Wall-clock estimate |
|---|---|---|---|
| 1 | Priors only, Fable pricing assumed 1.5–4× opus, 44-skill roster guess | $18–180 | 2–5 h |
| 2 | Published Fable pricing + peeked target plan (sonnet lanes, 35-skill roster, 2-conflict merge) | $27–95 | 2.5–5 h |
| Actual | Peeked session_events, token arithmetic | ≈ $70 | 55 min |

## Measured numbers

- Plan phase (fable-medium, 20 min): $11.07 — 44k output, 160k 1h-cache writes, 4.74M cache reads, 2 sonnet Explore agents.
- Full chain (55 min wall, 2612 s active): main session 95k output / 531k cache writes / 31.2M cache reads ≈ $47; 11 sonnet migration lanes + 2 Explores ≈ $25 (73k output, ~54M cache reads at $0.30/M). Total ≈ $70 API-equivalent.
- 13 subagents total; lanes ran concurrently (69–925 s each) — wall-clock = critical path, not sum.
- Effort switch medium→low mid-run: no event recorded, no context reinjection (uncached input +12 tokens), one-time ~116k cache-prefix rewrite (~$2.30) consistent with thinking-parameter cache invalidation at message level.
- Telemetry caveat: peek `cost_usd` reset mid-run ($17.18 → $12.99) while token counters stayed monotonic — token arithmetic was authoritative.

## Pricing (published 2026-09)

Fable 5: $10/M input, $50/M output — 2× Opus 4.8; 1h cache write $20/M, cache read $1/M. Sonnet 5: $3/$15, cache read $0.30/M. Sources: finout.io, openrouter.ai/anthropic/claude-fable-5, pricepertoken.com, ayautomate.com (Fable 5 pricing pages).

## Window calibration (Team Premium)

Meter deltas: 13% → 24% for ≈$32; 13% → 40% for ≈$70 total ⇒ 5-hour window capacity ≈ $260–290 API-equivalent. Weekly: run consumed 5% all-models / 7% Fable. (Max 20x ceiling for comparison: ≈$190, measured 2026-09-04.)

## Scorecard

- Right: cost envelope ($70 in $27–95), dominant term (the skill sweep), merge triviality (2 small conflicts), sonnet-lane fan-out shape once the plan was peeked.
- Wrong: wall-clock 3–5× over — sequential-batch assumption vs parallel lanes, and no effort latency model (fable-low ran the whole 5-phase implementation in ~35 min).
- Missing prior: cache reads dominate long orchestration sessions ($31 of the $47 main session) — estimate avg-context × requests × read-rate as its own line.
