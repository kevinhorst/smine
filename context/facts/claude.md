<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# Claude platform facts

Facts about Claude itself — API mechanics, prompt caching, harness behavior, cost model.
Sources: docs/cache-token.md (investigation wf_6d36f04f-246) and
docs/review-skill-token-debug.md in the smine repo. Harness facts predating this file
remain under FACT-REPO-ARCH-012/013/019 (claude-configs.md); a migration would rename
IDs and is deliberately deferred.

**FACT-CLAUDE-CACHE-001** — The Messages API is stateless: every call resends the whole conversation, and prompt caching is strict longest-prefix matching — the cache key hashes everything from byte 0 to a breakpoint (max 4 per request), so a single changed early byte invalidates everything after it. Entries are account-scoped, not session-scoped (byte-identical prefixes share across sessions and agents); minimum cacheable prompt 512 tokens; TTL 1h on subscription main conversations, 5m for subagents/workflows/compaction unless `subagentPromptCacheTtl` is set; use refreshes TTL free.

* Location: docs/cache-token.md §1–§2; https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching
* Reach: global

**FACT-CLAUDE-CACHE-002** — Usage-field semantics: `input_tokens` is only the remainder after the last breakpoint (≈2 in Claude Code — never "prompt size"); processed context = input + cache_creation + cache_read. Every conversation token is written to cache once and re-read on every later call: `cache_read(N+1) = cache_read(N) + cache_creation(N)`, so a session's cache-read total ≈ Σ per-call context — context size is a recurring per-call cost, not one-time. Pricing multipliers (× base input): read 0.1× (0.025× on 5.1-generation models), 5m write 1.25×, 1h write 2×, output 5× — never compare raw token counts across these categories.

* Location: docs/cache-token.md §3–§4, §6
* Reach: global

**FACT-CLAUDE-CACHE-003** — Cache invalidation hierarchy is tools → system → messages; a change invalidates its level and everything after. Triggers: mid-session tool-set changes (ToolSearch/MCP connects), hook or settings edits, CLAUDE.md reload, compaction (full rebuild), thinking-config or speed changes (messages-level), TTL expiry. A miss converts that turn's 0.1× read into up to a full-context 2× write — a ~20× one-turn spike; at a 500k context one ToolSearch costs ≈1M input-equivalents. Model switches are partial misses where measured (fable-5 ↔ opus-4-8: ~87–96% of the prefix survived); treat unmeasured pairs as full rebuilds.

* Location: docs/cache-token.md §5
* Reach: global

**FACT-CLAUDE-CACHE-004** — Cross-agent cache sharing exists only through byte-identical request prefixes; tool results are never shared between agents. Consequences for multi-agent design: keep worker prompt templates stable with volatile parts (round numbers, hashes, ids) as late as possible, spawn same-typed agents so tools+system match, and never expect one agent's file reads to be cached for a sibling. Per-agent grounding cost is design-invariant: review-type workers measured ~2M cache-read each across three differently-orchestrated runs.

* Location: docs/cache-token.md §2; docs/review-skill-token-debug.md recheck sections
* Reach: global

**FACT-CLAUDE-HARNESS-001** — A nested orchestrator subagent (a fork or depth-1 agent that spawns children) receives none of its children's task notifications — they escalate to the root session; an orchestrator that waits for child messages deadlocks into polling. Multi-agent skills must move results through files or agent return values, never notifications or inter-agent messages.

* Location: docs/review-skill-token-debug.md F17; cross-session-communication baseline
* Reach: global

**FACT-CLAUDE-HARNESS-002** — Multi-agent cost law: read volume ≈ Σ over calls of (context size at that call), so total cost is governed by agent count × per-agent calls × context size. The dominant controllable lever is per-agent call count — batching independent tool calls into one message halves calls and thus reads; the second is context ceiling (a long-lived coordinator re-reads its whole context every call — measured 548k peak / 54.9M reads for one dispatcher). Cost-weight before attributing: a measured large run split ~42% reads / 37% writes / 20% output at Opus multipliers.

* Location: docs/review-skill-token-debug.md; docs/cache-token.md §11
* Reach: global
