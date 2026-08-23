# ACDSL A/B round 3 — rule buried in the full pack (2026-08-06)

> Question: does a weak model (Haiku 4.5) miss an anti-prior rule when it arrives buried
> among all the classic repo rules — doctrine chapters plus the Go, Python, and SQL style
> guides — instead of alone? n=20, one arm, per Kevin's sizing.

## Setup

- Context: the full pack concatenated (~1,930 lines ≈ 20k tokens): rules/concepting+implementing+navigating+reviewing (incl. local files), style/go.md, style/python.md, style/sql.md.
- The pointer rule did not exist in the pack; formulated natively as **RULE-POINTER-003** (new(expr) mandatory, &T{...} forbidden) and inserted mid-file inside go.md's existing POINTER rule family — realistic burial: adjacent to same-topic rules, not highlighted, ~55% into the file.
- Delivery: probes read the pack file from the scratchpad themselves (one full Read) before coding; the buried rule never appears in the task prompt.
- Task: round-2's create task (NewConfig() *Config, fresh xpN package). Projection hook OFF (isolates the pack channel). Worktree write guard LIVE, and prompts deliberately NOT path-hammered — the guard, not the prompt, carries worktree discipline this round.
- Runs: 20 (xp10–xp29), four batches of five, gate + classification + cleanup per batch.

## Results

- **Adherence: 20/20** — every probe wrote `return new(Config{...})`. First-pass gate green in all four batches. Zero `&Config{`, zero tmp-var workarounds, zero syntax-corrections.
- Exact binomial: 20/20 puts the 95% lower confidence bound at ~83% adherence.
- **Zero worktree escapes** across 20 weak-model runs with relaxed prompts (round 2 escaped 3/3 before the guard existed). Main checkout byte-identical to before the round. Whether probes never attempted an escape or the guard silently denied attempts is unobservable from here — which is the point: the guard makes the outcome certain regardless of model behavior.

## Reading

1. **Burial in volume costs nothing measurable here.** Against the 0/9 no-rule baseline (round 2) and 3/3 rule-alone, 20/20 buried says a ~20k-token pack does not dilute an anti-prior rule below Haiku's adherence threshold — under this round's conditions.
2. **Why it likely held — and where dilution risk actually lives.** Three favorable conditions stacked: the pack was read as one contiguous block immediately before generation (recency + salience); the rule sits in the POINTER family while the task is a pointer-initialization task (retrieval-by-relevance); the task itself is small, leaving attention uncontested. The untested dilution axes are distance and decay — pack injected at turn 1 with generation at turn 40, long multi-file sessions, or rules whose subject shares no keywords with the task. Burial-in-volume is now measured; burial-in-time is not.
3. **Channel economics, now quantifiable.** Full-pack delivery cost ~52k tokens per probe (the Read dominates); projection delivers the 2–4 governing lines per touched file for near-zero. Same adherence outcome in these tests — so projection is the economical channel, the full pack the robust-but-expensive one, and the gate stays the guarantee under either.

## Limits

- One rule, one task shape, task-rule keyword affinity is high (pointer task, POINTER rule family) — a rule with no lexical affinity to the task is the natural round 4.
- Single-turn probes: no attention decay over a long session.
- The rules file was the probe's only read — no competing repo exploration diluting the pack.

## Cross-round picture

| Channel | Adherence (Haiku 4.5) | Cost |
|---|---|---|
| none | 0/9 | — |
| rule alone in prompt | 3/3 | ~0.1k tok |
| rule buried in full pack (this round) | 20/20 | ~20k tok/run |
| projection on read | 3/3 | ~0.1k tok/file |
| gate + generated retry | converts any red in 1 retry | 1 extra loop |

Delivery channel barely matters for whether the rule lands (once any channel exists); it matters enormously for cost. The gate remains the only guarantee — and has needed exactly one retry per violation everywhere it fired.
