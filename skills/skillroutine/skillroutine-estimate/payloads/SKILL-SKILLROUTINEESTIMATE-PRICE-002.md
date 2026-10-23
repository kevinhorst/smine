model tier x effort   | per-cell USD | per-cell output tokens | per-cell wall-clock
frontier high (sonnet)| 0.30 - 1.20  | 1.5k - 6k              | 2 - 8 min
frontier high (opus)  | 0.10 - 0.40  | 0.8k - 2.5k            | 1 - 4 min
mid-tier medium       | 0.03 - 0.15  | 0.5k - 2k              | 1 - 3 min
Anchors: railroad-review full runs measured at $5.33 mean (opus-high) /
$20.13 mean, $123 max (sonnet-high); ~60 worker cells per round; workers
average 21 tool calls each. Sub-agent cost rolls up into the dispatcher
session's single total_cost_usd.

Anchor (measured, interactive, 2026-09-05 — see references/2026-09-05-acdsl-formatting.md):
"Skill ACDSL formatting" full chain ~ $70 API-equivalent vs $27-95 estimate;
plan phase $11; 13 agents (11 sonnet lanes); 55 min wall-clock (parallel).
Per-token pricing and plan window ceilings arrive via the Claude usage facts
(context/facts/claude.md).
