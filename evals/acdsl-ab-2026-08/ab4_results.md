# ACDSL A/B round 4 — the affinity trap, three rules, one run (2026-08-06)

> Self-controlled design: three rules at different salience govern the same runs; within-run
> differences isolate the effect. Haiku 4.5, full-pack delivery (as round 3), projection off,
> write guard live. Task: JSON config loader + table-driven tests (two files — contested
> attention). n=15 (batches of 5; batch 4 cut at the user's call — signal already unambiguous).
> No new evalscripts this round [USER]: R2/R3 classified by hand against a hypothetical 100%.

## Results

| Rule | Salience design | Adherence | Notes |
|---|---|---|---|
| R1 new(expr) (RULE-POINTER-003) | task-affine, prose-only bullet in pack | **0/10 uncontaminated** (3/5 in the one contaminated batch) | every clean run wrote &Config{ — total collapse of the rule that went 20/20 in round 3's smaller task |
| R2 error prefix (RULE-ERR-001) | zero affinity, worked examples in pack, repo evidence aligned | **13/15** | all messages Identifier-prefixed (xp30.Load:, Config.Load:) except 2 runs in batch 2 (4 bare sites each) |
| R3 house test schema (RULE-TEST-001..005) | zero affinity, full code schema in pack, repo evidence MIXED | **13/15** | full _id/_shouldPass/append/t.Run(test._id) schema; the 2 misses read main-checkout common-style tests as reference and copied them |

Per-batch: B1 R1 0/5, R2 5/5, R3 5/5 · B2 (contaminated) R1 3/5, R2 3/5, R3 5/5 · B3 R1 0/5, R2 5/5, R3 3/5.

- **Retry loop:** batch 1's five R1 reds all converged in one mechanical retry (check -json generated prompts) — 5/5, consistent with round 2.
- **Contamination as data:** batch 2 probes read batch 1's post-retry files and picked up new(expr) from them — exemplars-on-disk beat the pack. Enforced full cleanup from batch 3 on restored the collapse.
- Zero worktree escapes; main checkout untouched; guard silent and sufficient across all weak-model runs.

## The finding — hypothesis inverted, something better found

The predicted gradient (affine holds, incidental drops) is **backwards**. What actually predicts survival under contested attention is not task affinity but **how the rule exists in the pack and the world**:

1. **R1 died (0/10)**: a prose-only bullet, one micro-decision inside a bigger task, zero corroborating evidence anywhere — the model's prior filled the gap the moment attention was contested. The identical rule scored 20/20 in round 3's single-function task: task size alone flipped it.
2. **R2 survived (13/15)**: the pack carries multiple worked examples (RULE-ERR-004's GOOD/BAD blocks), and every repo file the probes read corroborates the pattern. Procedural rules applied while writing the very construct stay locally salient.
3. **R3 survived (13/15) despite maximal anti-prior**: the pack contains the full code schema — and both misses correlate perfectly with reading common-style repo tests, i.e. the mixture decided, not the prior.

**Pack-design consequences, evidence-backed:** worked examples are load-bearing — a rule shown as code survives attention pressure that kills the same-strength rule stated as prose. In-repo evidence is a channel that can override the pack in both directions (rescued R1 in the contaminated batch, killed R3 twice). And affinity guarantees nothing: the rule closest to the task's subject died first because it competed head-on with the strongest prior at the highest-traffic decision point.

**Framework consequence:** this is precisely the variance the gate exists for. R1's 0/10 first-pass became 5/5 green after one generated retry each — the doctrine most likely to be silently dropped is exactly the doctrine the evalscript layer recovers at a fixed, small cost. Prompt-side delivery decides first-pass economics; only the gate decides outcomes.

## Follow-up hooks (not built, per the no-machinery decision)

- R1's collapse justifies its evalscript existing (it does — forbid-addr-lit carried the round).
- R2/R3 miss rates (~13%) are the case for their evalscripts (errorf-prefix; a test-schema check) if those rules are worth gating — now a measured cost/benefit, not a guess.
- Untested axes remaining: burial-in-time (long sessions), stronger models under contested attention.

## Cross-round picture (final for this session)

| Condition | R-affine (pointer) | R-incidental w/ examples | Gate recovery |
|---|---|---|---|
| no channel, small task (r2) | 0/9 | — | 1 retry |
| any channel, small task (r2/r3) | 26/26 | — | — |
| full pack, big task (r4) | 0/10 | 13/15, 13/15 | 5/5 in 1 retry |

Adherence is not a property of the rule or the channel alone — it is rule-form × task-pressure × ambient evidence. The only invariant across all four rounds: the gate plus a generated retry converts every violation it can see, in exactly one hop.
