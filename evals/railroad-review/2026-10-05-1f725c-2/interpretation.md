# Interpretation — railroad-review, opus/high, golden produce (smine-1.1.4-1.2.1-g2)

- One run, completed: 50m 25s wall, 9m 49s main-loop active, 1.8M output tokens over all agents, $218.90 (`runs/A/result.json`, report table). No check and no golden score — a produce run has neither.
- The run produced 54 candidate findings (2 blockers, 9 majors, 2 minors, 41 nits by its own result text); 66 claims, 10 duplicates, 2 debunked, none lost.
- Step misses 2 of 101 applicable entries (2%): the wrong-base heuristic was skipped on a 7314+/749- diff over 178 files, and the scope was confirmed only in the final report instead of up front. 39 entries were not applicable (no fixes at `autofix-level: none`, no session plan file headless).
- 142 permission denials over all agents. The adherence judge saw 4 dispositions where `/bin/bash` and the worktree's `bin/acdsl` were denied and the claim took the manual probe path; the remaining denials are not attributed.
- Adherence for workflow-internal entries (lanes, refuters, station) is graded from `runs/A/artifacts/round-1/`, not from the trace, which holds main-loop lines only. The known-ledger call ran before the gate presentation; graded followed, strict reading would make it partial.

Worker pins: lanes `2[opus:medium]` and station `opus:medium`, both from the definition's invocation; the dispatcher ran on opus/high, the adherence judge on `opus`.
