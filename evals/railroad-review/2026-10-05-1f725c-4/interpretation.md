# Interpretation — railroad-review 24.1.0, opus / high, golden produce

- One run, completed without error: 42m 28s wall, 8m 23s main-loop active, 1.6M output tokens over all agents, $204.39 — against a 315-finding seed, the first run billed on the Work-Team-Premium token.
- Yield keeps falling as the seed grows: 37 candidates (`runs/A/result.json`), after 51 in run `2026-10-05-1f725c-3` and 54 in `2026-10-05-1f725c-2`. The run's closing text reports 48 claims, 38 confirmed, 6 duplicates, 4 disproved, recommendation CONDITIONAL. Nothing is scored against a golden — produce run; Major+ defects and Rank are empty by design.
- Step misses 7% (7): three skipped, four partial, none violated. The one with substance is the handoff check: the id-prefix jq failed because `ranked_plan` entries carry no `direction` field, the retry sampled only entry 0, and the run still claimed "ids canonical" — the check cannot run as the skill writes it.
- Relay gaps in the final report: the station's 8 permanent checks were never offered, and the cleanup result's one unattributed worktree (the run worktree itself) was never mentioned. Unlike run `-3`, no review record was written at all — a headless run has no session plan file, and this one did not fall back to an artifact file.
- 114 permission denials over all agents, down from 148. Adherence was graded from main-loop trace lines plus round artifacts; the trace holds no subagent lines, and the `probes/` directory `review.json` points to is missing from the eval copy.

Worker pins: the task pins lanes to `2[opus:medium]` and the station to `opus:medium`; the root ran opus at high effort; the adherence judge ran on `opus`, the `run.json` judge model.
