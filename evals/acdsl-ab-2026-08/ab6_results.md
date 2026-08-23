# ACDSL A/B round 6 — task contracts under probe fire (E1) (2026-08-07)

> First probe round vs `lifetime="task"` entries ([design/ab_program.md](design/ab_program.md) E1).
> Contract: symbol-exists `func Load(path string) (*Config, error)`, symbol-coverage min=80,
> test-schema. Arms: **blind** (contract on disk only) vs **carried** (entries in prompt).
> Haiku 4.5 n=5 each, serial, reset per run. Fable carried n=3 (ran before "No Fable" stop; arm
> retired). Projection hook off, write guard live, no pack. Score: `check -lifetime all` per
> probe into round sink. Retry: same agent, mechanical prompt from diagnostics.

## Results — first pass

Cell = failure rate first pass (probes failed). Arm size in row.

| Arm | Signature | Coverage | Test schema |
|---|---|---|---|
| blind, Haiku (5) | **100%** (5) | **100%** (5) | **40%** (2) ¹ |
| carried, Haiku (5) | **40%** (2) ² | **40%** (2) ² | **40%** (2) ² |
| carried, Fable (3) | **0%** | **0%** | **0%** |

¹ other 3 passed vacuously — no `t.Run` for checker to judge.
² same 2 probes: copied contract lines into source, malformed markers, gate aborted before any
check. Other 3 passed everything.

All 5 blind probes named loader `LoadConfig`, not planned `Load`.

Retries: 4 chains green in 1 hop (one with all 3 entries red at once). 1 chain 2 hops — hop 1
left param `filePath` vs planned `path`. 2 chains operator-cut before last hop, residuals match
completed chains. Zero probe non-convergence.

Doctrine, no channel, 13 probes: ERR-001 failed 62% (8, incl. all 3 Fable). FMT-001 failed 31%
(4). TEST-001 failed 15% (2). PTR-001 failed 0%. Sink `~/.claude/acdsl/ab/ab6.jsonl`. Real sink
untouched.

## Findings

1. **Contract channel = full differential on signature.** Blind: all 5 wrote `LoadConfig`.
   Carried: none missed where gate ran. Round-2 shape, now plan-level contract.
2. **Leak-copy DoSes gate.** 2 carried Haiku probes pasted `//acdsl:` lines into Go files. Prompt's
   elided form = no `anchor=` = malformed marker = **whole gate aborts at authoring** — task AND
   doctrine ungated. Full-text copy would abort as duplicate ID same way. Fable: zero copies. Round 2 saw
   this with projection blocks; channel changed, behavior not. Fix candidates: contract in prompt
   as non-marker text; discovery warns instead of parses task markers outside `.acdsl`. Design
   call, not made here.
3. **test-schema vacuous.** Passes file with zero `t.Run`. Contract use needs presence check.
4. **Contract text beats prompt bans.** 3 probes read repo test files hunting "house testCase
   schema" despite read ban. Gate held; ban did not.
5. **Retry no longer strictly 1 hop.** Param name (`filePath` vs `path`) survived 2 of 7 chains;
   got/planned diagnostic closed it in hop 2. Multi-entry (3 rules, 6 sites) still 1 hop.
6. **Fable misses convention too.** All 3 wrote unprefixed errors (ERR-001 ×2 each) while hitting
   contract exactly. Unnamed = model prior, any model.

## Rig lessons

- Retry wait must key on agent completion notification. Output-file check races (file exists from
  first run). One retry scored mid-flight (purged, remeasured); 2 chains cut by early reset.
- Serial too slow: ~3–4 min per probe, 13 probes. [USER] binding for rounds 7–9: parallel probes,
  per-probe worktree isolation, no keep-prefix stash, no Fable.

## Limits

- n=5 per Haiku arm. Schema differential rests on 2 genuine blind reds. Fable n=3, retired.
- Elided prompt markers shaped the malformed copies; non-marker rendering not run as control.
- 2 retry chains cut before hop 2; hop-2 conversion measured once.

## Cross-round

| Condition | Round | Result |
|---|---|---|
| no channel, doctrine anti-prior rule | r2 | 100% failed (9) |
| no channel, planned signature | r6 blind | 100% failed (5) |
| contract in prompt | r6 carried | 0% failed (8 checked) |
| gate + generated retry | r2/r4/r5/r6 | every visible red converted; r6 adds 2-hop param tail |
| leak-copy | r2 projection, r6 prompt | weak model copies whatever carries rules; r6: copies can kill gate |

§3c model held first live fire: declare, red while absent, carried text moves model onto planned
symbol, gate + retry converge rest. New costs: marker text in prompt = gate-DoS vector; weak
primitive (schema presence) = vacuous pass.
