# Interpretation — railroad-review 24.1.0, opus / high, golden produce

- One run, completed without error: 47m 01s wall, 9m 16s main-loop active, 1.9M output tokens over all agents, $230.10 — well inside the 4 h timeout and the $1000 budget. The main loop is under a fifth of the wall time; the rest is the fan-out.
- Step misses 7% (7 of the graded entries): one skipped (wrong-base heuristic on an ~8000-line, 178-file diff — the base did equal the merge-base), six partial. None violated. Three of the partials are record-keeping (scope stated late, empty skipped list only in chat, doubled `Review — Review —` heading), one is the record landing in `artifacts/review-record.md` instead of a session plan file, which a headless run has none of.
- 148 permission denials over all agents is the cell worth acting on: the judge ties the two probe partials to it — several claims fell back to the manual path because tool policy denied the probe validity and verdict runs inside the probing agents, and 4 code-style probes carry no fixture.
- The run produced 51 candidates (`runs/A/result.json`); its own closing text reports 52 confirmed findings, one blocker and nine major, recommendation CONDITIONAL. Nothing is scored against a golden — this is the produce run; Major+ defects and Rank are empty by design.
- Adherence was graded from main-loop trace lines plus round artifacts: the trace holds no subagent lines, so the lane-conduct entries (incremental artifact writes, worker tool-call conduct) are not-applicable rather than observed.

Worker pins: the task pins lanes to `2[opus:medium]` and the station to `opus:medium`; the root ran `claude-opus-5-5` at high effort; the adherence judge ran on `opus`, the `run.json` judge model.
