# railroad-review — eval 2026-10-06

- prompt: /railroad-review directions: code-style correctness tests security lanes: 1[sonnet:medium] scope: branch[b4981b88d52deaa69d6ed1d60eb51b57ead47ba0] exclude: sessions/ evals/ proposals/ plans/ spec: plans/v1-2-1/2026-08-24-language-style-sett…
- start commit: `1f725ced` in `/Users/devuser/GolandProjects/smine`
- auth: token Work-Team-Premium
- judge: `opus`

| Run | Smoke | Check pass | Wall | Main-loop active | Output tokens (all agents) | Cost | Permission denials (all agents) | Step misses | Major+ defects | Rank | Review score | Lost minutes |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| opus-high-r1 | pass | — | 2h 13m | 23m 18s | 4.3M | $326.36 | 452 | **11%** (11) | — | — | -0% | 12576 min |

## Step misses

- `opus-high-r1` SKILL-RAILROADREVIEW-EXEC-003b — Paths, never content (partial): trace.md:77 — review.json read only through jq (trace.md:59, 61, 75) and ledger and risk map passed as paths. But the dispatcher also read probe files and a probe registry and hand-ran a probe (trace.md:77-81), reads outside gate presentation and fix rou…
- `opus-high-r1` SKILL-RAILROADREVIEW-PROC-003 — Wrong-base heuristic (skipped): trace.md:17 — The diff is ~8k changed lines over 178 files, yet no plausibility check of the base appears. The base was user-pinned, so the risk was low.
- `opus-high-r1` SKILL-RAILROADREVIEW-PROC-005 — Confirm scope (skipped): trace.md:10 — Inclusions and exclusions were never stated in chat; only passed as flags.
- `opus-high-r1` SKILL-RAILROADREVIEW-WALK-004 — Checklist agents (partial): trace.md:124 — Invoked both rounds with the right args, with a primer hop before round 1 (trace.md:47-49). After the harness resume (a new prompt), the checklist was relaunched without a primer hop.
- `opus-high-r1` SKILL-RAILROADREVIEW-ROUND-015 — Permanent checks (partial): trace.md:153 — The station named 11 permanent checks in review.json; the dispatcher's report never relayed or offered them.
- `opus-high-r1` SKILL-RAILROADREVIEW-STATION-002 — Failed round (partial): trace.md:132 — The checklist crash was resumed via resumeFromRunId (trace.md:124), and the failed launch's run id was kept. But the round-2 round.js failure was relaunched directly, without the Round reset sequence (void ledger record, reset-round, re-pin…
- `opus-high-r1` SKILL-RAILROADREVIEW-STATION-017 — Record fixed (skipped): trace.md:95 — Some applied ids had no probe: code-style-MIN-8/9-r1 in round 1, and code-style-MIN-21..24-r2, NIT-18/27-r2, tests-MIN-11/13/14-r2 and tests-NIT-12-r2 in round 2. So the checker's fixed set minus red probes was non-empty, yet the dispatcher…
- `opus-high-r1` SKILL-RAILROADREVIEW-PROBE-006 — Manual path (partial): artifacts/round-1/verdicts/code-style-min-9-r1.json — Manual path taken, but some verdicts (e.g. code-style-MIN-9-r1) carry no artifact (artifacts []).
- `opus-high-r1` SKILL-RAILROADREVIEW-OUT-004 — Coverage (partial): trace.md:153 — review.json carries coverage_gaps []. Neither the presentation nor the report states the files reviewed against the 178/183-file diff list.
- `opus-high-r1` SKILL-RAILROADREVIEW-DELIVER-001 — Plan-file record (skipped): trace.md:153 — No session plan file was written or updated in either round.
- `opus-high-r1` SKILL-RAILROADREVIEW-DELIVER-005 — Section structure (skipped): trace.md:137 — smine-rules present was run without --section; no review section was rendered.

## Unobservable entries

None.

## Smoke and variant failures

None.
