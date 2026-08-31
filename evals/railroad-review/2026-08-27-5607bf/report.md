# Railroad-review measurement — 2026-08-27, arm 8.3

Range: a7abd0401d2beadde42bb63ba4d395606c94f2dd..5607bf47c18971beae2436e6daa2f49a1ff28893 | directions: code-style correctness critical | lanes/direction: 2 | refute: major (cap 6, 2 spawned)

| Stage | Cells | Failed | Cost USD | Output tokens | Wall ms |
|---|---|---|---|---|---|
| lanes | 6 | 1 | 5.82747945 | 103788 | 1635302 |
| refute | 1 | 0 | 0.71003475 | 16342 | 219856 |
| station | 1 | 0 | 0.92818635 | 20995 | 278781 |

| Lane | Injected | Spill reads | Ctx reads | Missing required | Ctx at end | Output tokens | Wall ms |
|---|---|---|---|---|---|---|---|
| lane-code-style-1 | 3 | 0 | 1 | 3 | 100565 | 14039 | 212692 |
| lane-code-style-2 | 3 | 0 | 0 | 3 | 0 | 797 | 100224 |
| lane-correctness-1 | 3 | 0 | 0 | 1 | 133838 | 32802 | 490178 |
| lane-correctness-2 | 3 | 0 | 0 | 1 | 107060 | 20405 | 303518 |
| lane-critical-1 | 3 | 0 | 2 | 0 | 102371 | 19369 | 285257 |
| lane-critical-2 | 3 | 0 | 0 | 2 | 99436 | 16376 | 243433 |

Funnel: {"claims_produced":10,"after_dedup":7,"debunked":1,"survivors":6}

RUN FAILURES occurred — see cells.jsonl (exit_status != 0 rows).
