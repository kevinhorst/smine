# ACDSL A/B round 5 — prose vs gate on the baseline rules, verdict-sink measured (2026-08-07)

> First §3b-instrumented experiment: measurement is `acdsl check` into per-arm verdict sinks,
> readout is `acdsl verdicts -path` — no hand-classification. Rules under test: the §1 baseline
> (ACDSL-FMT-001 gofmt, ACDSL-FUNC-001 signatures, ACDSL-ENUM-001 exhaustive switches).
> Haiku 4.5, 20 probes per arm (40 total), batches of 5, projection hook off, write guard live.
> Probe dirs `internal/ab5pNN` — outside `^internal/xp`, so the legacy experiment rules stay out.

## Setup

- **Arm P (prose):** probes read a scratchpad rules doc first — verbatim pack text (RULE-FUNC-001 bullets, ALWAYS-INTEG-002 with why, the gofmt mandate), no worked examples added. First pass measured, no retry.
- **Arm C (checks):** identical prompt minus the rules doc. First pass measured, then the mechanical retry loop from `check -json` diagnostics (cap 2).
- Task (both arms): JSON config loader package — typed 3-member `Mode` enum with per-mode `Describe` switch, `Load(path) (*Config, error)`, `Apply(ctx, cfg) error`, table-driven tests. No rule named in the prompt.
- Rig self-test before probes: golden `ab5p00` checked green; seeded missing-enum-member mutation checked red citing ACDSL-ENUM-001.
- Measurement: one check run per probe (siblings stashed), `ACDSL_VERDICTS_PATH` per phase — sinks `armP.jsonl` (20 records), `armC_first.jsonl` (20), `armC_retry.jsonl` (18). Invalid outputs (check exit 2): **0/40**.

## Results — the three `verdicts` tables (verbatim)

Arm P (prose, first pass):

```
rule               projected  runs   red violations  last-red
ACDSL-ENUM-001     true         20     0          0
ACDSL-FMT-001      true         20     0          0
ACDSL-FUNC-001     true         20     0          0
```

Arm C (no prose, first pass):

```
rule               projected  runs   red violations  last-red
ACDSL-ENUM-001     true         20     0          0
ACDSL-FMT-001      true         20    18         28  2026-08-07T12:46:09Z
ACDSL-FUNC-001     true         20     0          0
```

Arm C (after retry):

```
rule               projected  runs   red violations  last-red
ACDSL-ENUM-001     true         18     0          0
ACDSL-FMT-001      true         18     0          0
ACDSL-FUNC-001     true         18     0          0
```

(Module-wide controls EXEC/STATE and the xp-anchored legacy rules: 0 red everywhere, as expected. Full 8-rule tables in the sinks.)

## Findings

1. **FMT-001 is the round's entire differential — and it is huge.** Without prose, 18/20 probes (90%) produced non-gofmt files (28 unformatted files; the dominant miss is struct-tag alignment). With one prose line ("gofmt; every Go file MUST be gofmt-formatted"), 0/20 — arm-P probes demonstrably ran gofmt before finishing (their reports say so). A single delivered sentence flipped a 90% failure rate to zero: prose as *behavioral instruction* (run the tool), not just constraint knowledge.
2. **Retry economics: 18/18 converted in exactly one mechanical hop.** The cross-round invariant holds at its largest n yet (rounds 2–5 cumulative: every gate-visible violation converted in one generated retry). But each retry cost a full Haiku invocation (~28k tokens) to do what `gofmt -w` does for free — **the strongest evidence yet for roadmap §2's autofix ladder**: with a `fix` argv on the gofmt registry entry, arm C's 18 reds would have cost zero model retries. check → autofix → recheck removes this entire failure class from the model economy.
3. **FUNC-001 and ENUM-001 hit a ceiling in both arms — no differential obtainable.** Two honest reasons stated up front: the task prompt itself pinned `Apply(ctx context.Context, cfg *Config) error` (a delivery channel for the ctx-first trap — unavoidable when the task must force a context-taking function without naming the rule), and error-last/≤3-returns plus fully-enumerated 3-member switches are strong Haiku priors at this package scale. These rules' prose value is **untested**, not proven zero; a differential would need tasks where the signature is not spec-pinned and the enum is larger/pre-existing.
4. **§3b eviction reading** (the input this data exists for): FMT-001's prose earns its tokens today (90-point first-pass swing) — but only until the fixer lands; with check→autofix, FMT-001 becomes the first `projected="false"` candidate (gate+fixer converts at zero model cost, prose buys nothing the fixer doesn't). FUNC/ENUM: no eviction case either way — both arms clean, so neither prose benefit nor gate-only sufficiency is measured yet.
5. **Rig lesson (new contamination class):** retry probes ran `acdsl check` themselves without the sink override — 6 records leaked into the real home sink (scrubbed post-run; window-verified). Future rounds: export `ACDSL_VERDICTS_ENABLED=0` into probe environments, or the sink accumulates probe noise as real-work data. Same family as round 4's exemplars-on-disk lesson: probes interact with every ambient channel they can reach.
6. **Verdict-sink measurement worked as designed.** Per-probe check runs made `verdicts` denominators probe counts; the three tables above are raw tool output, zero custom analysis. Cost: ~10s per measurement run; the isolation loop (stash siblings → check → restore) held with no cross-probe attribution errors.

## Limits

- FUNC/ENUM ceiling (finding 3): task-pinned signatures and small fresh enums; switch-avoidance (validating via if-chain instead of switch) was not classified per probe.
- Single-turn probes, single task shape, one weak model — as in all prior rounds.
- Arm P's prose doc contained exactly the three rules — no burial, no competing pack content (round 3 measured burial; this round isolates presence/absence).

## Cross-round picture

| Condition | Round | Result |
|---|---|---|
| no channel, small task | r2 | 0/9 first-pass (pointer rule) |
| any channel, small task | r2/r3 | 26/26 |
| full pack, big task — prose-form rule | r4 | 0/10 (R1 collapse) |
| full pack, big task — example-form rules | r4 | 13/15, 13/15 |
| three-rule prose doc, medium task | **r5 P** | **60/60 rule-observations green (20 probes × 3 rules)** |
| no channel, medium task | **r5 C** | **FMT 2/20 · FUNC 20/20 · ENUM 20/20 first-pass** |
| gate + generated retry | r2/r4/**r5** | every visible red converted in exactly 1 hop (r5: 18/18) |

The invariant sharpens: **which rules need delivery is a per-rule empirical fact** — FMT-001 collapses without a channel while FUNC/ENUM ride priors — and the delivery decision now has its measuring instrument (the verdict sink) and, for tool-fixable rules, its endgame (the autofix ladder, not prose).

## Follow-ups fed back

- Roadmap §2: gofmt `fix` argv promoted from candidate to evidence-backed first autofix entry (18 model retries bought nothing a fixer wouldn't).
- Roadmap §3b: FMT-001 flagged as the first `projected="false"` candidate — contingent on the fixer landing.
- Rig hygiene: probe env must carry `ACDSL_VERDICTS_ENABLED=0` (this round's sink-pollution lesson).
