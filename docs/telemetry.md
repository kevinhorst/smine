# Claude Code telemetry & usage analytics

Findings from the ccusage/peek/config-server investigation (2026-07-22, run `wf_296b23dd-776`; per-run artifacts and refuted-hypotheses register in the session scratchpad, decision summary below). Two distinct use cases emerged with two distinct answers.

**Source:** https://code.claude.com/docs/en/monitoring-usage

## Use case 1 — spend overview ("how much did I use")

**Answer: adopt the native ccusage binary. Do not build.**

The "npx/bunx-only" premise is dead: ccusage v20 is a full Rust rewrite distributed as a zero-dependency native binary — bottled homebrew-core formula (`brew install ccusage`), nix, and npm platform packages whose tarball is plain-curl-able (Windows: extract `bin/ccusage.exe` from the `@ccusage/ccusage-win32-x64` registry tarball, no Node needed).

It already implements the correctness-critical parts:

- Dedup on `(message.id, requestId)` with sidechain-replay collapse — load-bearing: local transcripts repeat identical usage lines up to 5× (streaming rewrites), so naive summation over-counts.
- 5m/1h cache-write pricing (1h = 2× input), >200k tiered pricing.
- Embedded LiteLLM + models.dev pricing snapshots; `--offline` and `--json` on every report.

Report granularity: daily / weekly / monthly / per-session / 5h billing blocks, per-model (`--breakdown`), per-project (`--instances`, `-p`). Finest grain is the **session** — no per-skill or per-tool dimension exists or can exist there (usage lines carry no skill/tool tags; ccusage never reads message content).

Rejected directions (with refuting evidence, do not re-investigate):

| Direction | Why rejected |
| :--- | :--- |
| Reimplement in config server (Go) | ccusage is Rust — nothing vendorable; 3–5 days; second JSONL parser next to peek-mcp over the same files |
| Extend peek-mcp with pricing/cost | 2–4 days + permanent pricing-maintenance treadmill; the missing 60% (dedup, cache tiers, pricing) is exactly what ccusage ships |
| OTel **metrics** as spend source | OTLP/Prometheus/console exporters only, forward-only, zero backfill of existing history; `cost_usd` is a client-side estimate on the same basis ccusage computes |
| Third-party trackers (ccusage_go, claude-usage, CodeBurn, …) | all less maintained than upstream's own native binary |

Caveats: no transcript contains `costUSD` (field absent, 0 hits in 276 files) — every dollar figure anywhere is computed API-equivalent pricing, notional under subscription billing. Config-server integration shape, if wanted: a `/usage` page that execs `ccusage daily --json --offline` (existing shell-out tool-action pattern); ccusage stays the single source of truth.

## Use case 2 — skill/tool efficacy ("is serena better than Grep") 

**Answer: Claude Code's native OTel *events* (logs), received by the config server.**

ccusage covers the odometer, not the diagnostics port. The comparison data lives in the OTel event stream (verified against the monitoring docs, 2026-07-22):

- `claude_code.tool_result` — `tool_name`, `success`, `duration_ms`, `error_type`, `tool_input_size_bytes`, `tool_result_size_bytes`, `mcp_server_scope`. Per-tool success rate, latency, and context-cost, with MCP tools distinguished.
- `claude_code.tool_decision` — accept/reject per tool with decision source (`config`/`hook`/`user_permanent`/`user_temporary`/`user_reject`). Permission-friction stats; unobtainable from transcripts, and it keeps human wait time out of `duration_ms`.
- `claude_code.user_prompt` — `command_name`: skill invocations as first-class events; joined via the standard `session.id` attribute, per-skill segmentation becomes a join instead of transcript slicing.
- `claude_code.api_request` — per-request tokens, `duration_ms`, `cost_usd`; inter-event timing yields thinking-time vs tool-time.

Constraints: exporters are OTLP/console only — no file export, forward-only (no backfill). Integration shape: a minimal OTLP HTTP log-receiver endpoint in the config server (already a standing local process) flattening events to SQLite/JSONL, plus `CLAUDE_CODE_ENABLE_TELEMETRY=1`, `OTEL_LOGS_EXPORTER=otlp`, endpoint → localhost in settings env. Roughly a day of Go for the receiver; reporting pages on top separate.

What transcripts (peek/smine) still own: historical backfill and **sequence analysis** — retry chains, tool fallbacks (serena fails → Grep), post-skill rework. Complementary, not competing. Prerequisite if peek ever reports token numbers: its `TotalUsage` currently sums duplicated streaming lines — dedup on `(message.id, requestId)` must land first (latent bug, exists today).

## Open verifications

1. `ccusage daily --json --offline` totals vs deduped jq ground truth (`group_by(message.id + "|" + requestId) | first`) — >0.1% divergence re-opens the decision.
2. `--offline` vs online pricing delta — >5% means the embedded snapshot lags; run online or pin a fresher binary.
3. Does `user_prompt.command_name` fire for Skill-tool invocations mid-conversation, or only typed slash commands? (`OTEL_LOGS_EXPORTER=console` smoke test.)
4. Do subagent/workflow tool events carry the parent `session.id`? (Same smoke test.)
5. Magnitude of peek-mcp `TotalUsage` over-count across the corpus (naive vs deduped sums).
