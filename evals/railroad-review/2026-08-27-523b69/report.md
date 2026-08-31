# Railroad-review measurement — 2026-08-27, arm 8.3

Range: 72f50e5f7193ff60248a06f0664a67e446ce3014..523b69946405f5e7f0a68e6ac3e3489225f4489e | directions: code-style correctness critical | lanes/direction: 2 | refute: major (cap 6, 0 spawned)

| Stage | Cells | Failed | Cost USD | Output tokens | Wall ms |
|---|---|---|---|---|---|
| lanes | 6 | 0 | 4.7588934 | 81320 | 1197676 |
| station | 1 | 0 | 0.7711682999999999 | 14965 | 196159 |

| Lane | Injected | Spill reads | Ctx reads | Missing required | Ctx at end | Output tokens | Wall ms |
|---|---|---|---|---|---|---|---|
| lane-code-style-1 | 0 | 0 | 0 | 3 | 107718 | 14518 | 216940 |
| lane-code-style-2 | 0 | 0 | 2 | 2 | 96810 | 12269 | 181745 |
| lane-correctness-1 | 0 | 0 | 0 | 1 | 98778 | 14761 | 202678 |
| lane-correctness-2 | 0 | 0 | 0 | 1 | 113492 | 22120 | 321153 |
| lane-critical-1 | 0 | 0 | 0 | 2 | 94203 | 11349 | 167725 |
| lane-critical-2 | 0 | 0 | 0 | 2 | 87708 | 6303 | 107435 |

Funnel: {"claims_produced":7,"after_dedup":6,"debunked":0,"survivors":6}
