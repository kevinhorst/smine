# Golden methodology — what went wrong in v1, and the v2 rules

What a golden run's numbers mean, what went wrong with the v1 line (golden v1–v9), and the rules the g2 line follows. Only rules the Go runner or the skill carries out are listed; the rest is history.

## Reading the scores

`skilleval fold` writes these per arm into `eval.json`; nothing else is computed.

- Golden metrics, only once a golden is minted:
  - `review_score` — the minutes the review saved against no review, 1.0 = a perfect human-fixed review
  - `lost_minutes`, `escaped_minutes`, `false_positive_minutes` — the minute buckets behind it
  - `recall`, `false_positives`, `misclassified` — diagnostic counts, never part of the score
- Telemetry:
  - `wall_seconds`, `active_seconds` — peek, the arm session
  - `output_tokens`, `cost_usd`, `permission_denials` — the arm's result envelopes, all agents
- Pricing rules are in the eval README, prices in `golden/scoring.json`.

There is no efficiency score, no round ladder and no yield-per-token comparison. A run compares to another only within one golden version.

## What went wrong in v1

Each item produced a wrong number before it was found. Ordered by damage.

1. **Envelope tokens counted only the main session.** Headless cells reported the per-turn `usage` field, which covers the dispatcher loop only; the 48+ lane subagents were invisible. The result envelope's `modelUsage` map covers every agent and was never read. Golden v5's production row said 251,858 output tokens; the transcript truth was 3,180,942 (12.6×). The efficiency baseline summed these envelopes, so every run's token axis was inflated ~2.5×.
   - Today: `output_tokens` is the sum of `modelUsage[*].outputTokens` and `cost_usd` is `total_cost_usd` of the last result envelope, all agents.
2. **Three scope conventions in one table.** Recovered numbers mixed whole-session-tree, arm-workflow-only and arm-plus-judge rows, summed blind.
   - Today: every arm metric has one source — the arm cell's envelopes or peek's arm session. Judges run as subagents of the eval session, outside the arm, and never enter an arm number.
3. **A lumped multi-round baseline.** Efficiency divided single runs by the whole 52-round campaign's rounds and wall-clock; a parallel desktop hour was compared against a serial headless week.
   - Today: no mechanism — the runner has no efficiency baseline.
4. **Matched vs new semantics divergence.** Fresh arms' judge-matched re-finds were counted as yield against a baseline that held only new findings; a fresh opus round scored 8.3 against a saturated round.
   - Today: no mechanism — no yield score exists. The judge separates matched from uncataloged; only uncataloged findings are promotion candidates.
5. **Source and id format drift.** Promotion provenance was recorded five ways, from `run-<dir>-<fid>` to free prose; golden ids were renumbered twice (`GV3-` covers v1 to v3); prose-sourced promotions (v4–v7) cannot be attributed to a round.
   - Today: candidate ids carry their round (`r<round>-<finding id>`). The `source` format is a convention of the hand-written master; no gate checks it.
6. **Missing per-round records.** Some rounds had no station `review.json` (aborted round 2s), so their confirmed sets were empty and attribution fell back to id markers. The v1 campaign (6 rounds, ~470 agents) has no round markers at all.
   - Today: a round without a handoff is recorded with `smine-rules ledger --void`; candidates come from the cumulative `known.json`, whose rows carry their round.
7. **Depth inferred instead of recorded.** Alignment depth was derived from the golden version a run was scored against; a seeded arm of a two-arm day was aligned one round shallow. The seed's content was never recorded per arm.
   - Today: a produce run hands `goldens[0].produce.seedFile` to the skill as its `known:` list, snapshotted per run as `seed.json`; the run's own ledger holds only its own findings, and those are the candidates. No depth alignment.

## Reconstruction status (v1 archive)

Backfilled from the on-disk transcript trees (message.id-deduped, per-id max), verified against committed telemetry to the token where both exist: arm-only tokens, active seconds and per-round splits for all ten railroad run dirs; the ladder (rounds 7–17) with per-round `newWeight`, `refoundWeight` and budgets. Residual, permanently: the v1 aggregate block; prose-source promotions without round attribution; seeded rounds' `refoundWeight` being ledger leakage rather than measured re-finding. Old run dirs keep their efficiency blocks; no current run writes one.

## V2 rules

Record at production time; never reconstruct.

1. **One golden version per promotion.** Each promotion bumps `goldenVersion` in the hand-written master and is derived with `goldens gen`.
2. **Telemetry at run time.** Time comes from peek at fold, tokens and cost from the last result envelope. Nothing is parsed from transcripts by hand.
3. **One scope.** Arm numbers cover the arm cell — dispatcher and every agent it spawned. Judges are accounted separately and never summed into an arm.
4. **One source format.** Every promoted finding's `source` is `run-<dir>-<candidate id>`, verbatim; candidate ids carry their round; golden ids are never renumbered — a claim change is an amendment on the same id, a removal a `falsePositives` entry.
5. **A round is not a round without a station review.** A round that ends without `review.json` is recorded with `smine-rules ledger --void`, never left as dangling lane files.
6. **Fix outcomes are recorded, not presumed.** Every `known.json` row carries `fix_state`: `open` when ledgered, `applied` after the fixer landed it, `fixed` only after the fix check confirmed the defect gone.
7. **Prices are pinned.** A change to `minutes`, `escapeMultiplier` or `fixFactor` in `scoring.json` bumps the golden version.

## Telemetry scopes

Two scopes; a report names which one a number uses, and they are not interchangeable.

1. **peek session scope** — `wall_seconds` and `active_seconds` of the arm session; active time excludes pauses.
2. **envelope scope** — `output_tokens`, `cost_usd` and `permission_denials` (distinct `tool_use_id` across all envelopes); covers the dispatcher and every subagent and workflow agent.
