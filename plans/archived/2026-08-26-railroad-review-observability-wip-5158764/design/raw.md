# Railroad-Review Observability & Fixes — Implementation Plan

## TLDR

- **Goal:** make railroad-review's subagent behavior visible and fix the verified context leak — without building an eval leviathan.
- **Headline finding:** CTX-002 ("lanes get no injection — they never invoke a skill") is false in code: the lane prompt orders every lane to invoke the Skill tool, the skill-context hook has no subagent guard, so each lane receives the full dispatcher doctrine (~45KB spill + read-in-full instruction). This is the suspected token/time waste.
- **Fix, not experiment:** the hook already honors a `--no-context` opt-out token — a one-line lane-prompt change restores the behavior CTX-002 already specifies. Before/after traces quantify the savings; a recall A/B is only warranted if review quality visibly drops afterwards.
- **Branch prefix:** all branches the railroad-review skill creates (lanes, refuters, station) move from `claude/railroad-*` to the `claude-review/*` namespace.
- **Visibility:** the trace runbook works entirely from on-disk workflow transcripts with jq — peek-mcp stays at v1.2.1 and nothing here depends on it. The verbose-mode contract (subagent events + main thinking, no subagent thinking) is recorded as the future peek-mcp work item, not a prerequisite.
- **Eviction:** observational screening over traced real runs (never-injected-needed / never-read / never-cited entries → tombstone candidate list); no confidence scores or labels on entries.
- **Deferred:** defect-seeded benchmark, context-pack ablation, scorer, cell driver — the causal harness is built only when a specific eviction decision demands causal proof.

## Context

- **Problem:** railroad-review is the most expensive skill and nobody can see what its lane subagents actually read or cost; one context mechanism (CTX-002) is provably violated by the skill's own lane prompt.
- **Cause:** lanes invoke the Skill tool ([railroad-review.js:203](skills/quality/railroad-review/workflows/railroad-review.js)) → PostToolUse skill-context hook fires with no subagent guard ([skill-context.sh:44-52](cmd/hooks/skill-context.sh)).
- **Design:** restore CTX-002 via the hook's existing `--no-context` token; rename the skill's branch namespace to `claude-review/*`; add a peek-mcp verbose mode as the durable inspection surface; run eviction screening observationally over real runs.
- **Constraint:** peek-mcp stays at v1.2.1 for this plan — the verbose mode is future separate-repo work spec'd here only as a contract; every runbook and verification step runs on on-disk transcripts with jq, no peek dependency.
- **Constraint:** subagent thinking transcripts stay excluded from verbose mode (volume).

## Drivers

N/A — new route.

## Scope

- **In:**
  - **lane-context fix:** `--no-context` on the lane's Skill invocation in railroad-review.js; CTX-002-adjacent prose alignment; changelog + version bump.
  - **branch namespace:** every skill-created branch (lane, refuter, station) renamed to the `claude-review/*` prefix, including cleanup expectations and prose.
  - **trace runbook:** jq recipes over on-disk workflow transcripts — per-lane context reads, injected volume, tokens, time, step adherence; before/after the fix. Self-sufficient at peek-mcp v1.2.1.
  - **peek verbose contract (spec only):** the tool-surface spec for subagent transcripts + main thinking, recorded for future peek-mcp work — nothing in this plan waits on it.
  - **eviction screening:** decision rule + recipe producing a tombstone candidate list from ≥3 traced real runs.
- **Out:**
  - **causal harness:** benchmark branch, answer key, ablation branches, scorer, cell driver — deferred until an eviction decision needs causal evidence.
  - **entry metadata:** no confidence scores, labels, or usage counters on context entries.
  - **subagent thinking:** excluded from verbose mode.
- **Not changed:**
  - **skill-context.sh / read-gate.sh:** the hooks stay as-is; the fix rides the existing opt-out token.
  - **skillroutine-eval / matrix.sh / config server:** untouched.
- **Deferred findings:**
  - **dead doc link:** `README.md:362` links `docs/telemetry.md`, which does not exist.
  - **missing verdict sink:** `sessions/parallelize/verdicts.md` is written to by two skills but the directory does not exist.

## Assumptions

| Assumption | Reality | Location |
|---|---|---|
| CTX-002: lanes get no injection because "they never invoke a skill" | Lanes are ordered to invoke the Skill tool; the hook fires for subagent tool calls with no agent guard — each lane gets the full doctrine injection (spill + read-in-full instruction when >9000 chars) | [railroad-review.js:203](skills/quality/railroad-review/workflows/railroad-review.js), [skill-context.sh:44-52,126-134](cmd/hooks/skill-context.sh) |
| Injection reaches lanes at runtime (statically verified only) | Confirmed empirically by the runbook's before-trace: the `===== SKILL CONTEXT` block / spill read appears in lane transcripts | Verification |
| `claude-review` prefix applies to the skill's own branches generally [USER] | All lane/refuter/station branches move to `claude-review/*`; other tooling that sweeps `claude/*` no longer touches them — cleanup relies on explicit branch names (`remove_agent_worktrees.sh --delete-branch`), so nothing else depends on the old namespace | [railroad-review.js:80-99,124-147](skills/quality/railroad-review/workflows/railroad-review.js) |
| peek-mcp availability | Pinned at v1.2.1 for this plan [USER] — no verbose mode exists or is assumed; the future implementation can reach subagent transcripts (`agent-<id>.jsonl` + `journal.jsonl` sit next to the session JSONL peek already reads) | [F13](#f13) |

## Current state

N/A — new route.

## Target state

N/A — new route.

## Behavior contract

N/A — new route.

## Decisions

| ID | Problem | Facts | Decision | Why |
|---|---|---|---|---|
| <a id="d1"></a>D1 | Fix the lane injection now or A/B it first | [F1!](#f1), [F2!](#f2), [F3!](#f3) | Apply `--no-context` directly as spec restoration; quantify with before/after traces; a recall A/B (the deferred harness) triggers only if review quality visibly drops | CTX-002 already specifies no lane injection — restoring documented behavior needs no experiment; the harness cost is only justified once the spec itself is in question |
| <a id="d2"></a>D2 | Verbose-mode shape — one giant blob vs filterable | [F13](#f13), [F6](#f6) | Events-level view per subagent (tool name, key args, usage, timestamps) as the default; full transcript text per single agent on demand; main-agent thinking included; subagent thinking excluded [USER] | A railroad round's subagent transcripts are megabytes — an unfilterable verbose mode relocates the overflow into the consumer's context; events answer the adherence/cost questions |
| <a id="d3"></a>D3 | Where eviction evidence lives | [F11!](#f11) | Nothing persisted on entries; screening reports live under `evals/railroad-review-<date>/`; eviction remains a manual tombstone citing the report | An observational verdict stored on the entry would masquerade as authority; tombstones are the existing single lifecycle mechanism |
| <a id="d4"></a>D4 | Eviction decision rule without causal evidence | D3 | Candidate = across ≥3 traced real runs the entry was (1) never read by any lane whose direction requires it, (2) never cited in any finding, (3) not `[gate]`-enforced. Candidates go to Kevin as a tombstone proposal list — accepted risk: observational only | Cheap, runs on real reviews (no benchmark), and the acceptance step keeps a human on the irreversible action; a candidate that later proves load-bearing is recoverable (tombstones record replacements) |
| <a id="d5"></a>D5 | Tracer as shipped script vs jq runbook | [F8](#f8), [F13](#f13) | Runbook of jq recipes (persisted under the plan's runbooks dir), not a shipped `cmd/` script | Fewest concepts: the recipes are 5–10 jq lines each against stable JSONL shapes; a shipped parser is maintenance surface for a diagnostic run a handful of times — promote to a script only if it becomes routine |
| <a id="d6"></a>D6 | Branch scheme under the new prefix | [F6](#f6) | `claude-review/{direction}[-{chunkSlug}]-l{i}-r{r}-{hash6}`, `claude-review/refute-{slug}-r{r}-{hash6}`, `claude-review/station-r{r}-{hash6}` — structure kept, redundant `railroad-` dropped (the prefix names the skill) | Cleanup enumerates explicit names, so only the name-builders and prose change; keeping the structural fields preserves debuggability of leftovers |
| D7 | Subject repo for any future benchmark | — | claude-configs [USER] | Recorded for the deferred harness; self-contained, real context pack |
| D8 | Spend | — | Trivial now (~2 real runs for before/after); the deferred harness's rung budget (~$60 cap, full matrix separately approved) is recorded for when it revives [USER] | The observational path removes the cost question |

## Open questions

None — all decisions closed ([D7](#d7)/D8 recorded as [USER] from review feedback).

## Baseline (verified)

Base branch: `main` (worktree `claude/railroad-review-testing-5b8432`).

| ID | Fact | Needed for | Location |
|---|---|---|---|
| <a id="f1"></a>F1! | Lane prompt orders `Invoke the Skill tool with skill="railroad-review"` — every lane triggers the skill-context hook | [D1](#d1), §1 | [railroad-review.js:203](skills/quality/railroad-review/workflows/railroad-review.js) |
| <a id="f2"></a>F2! | skill-context.sh PostToolUse(Skill) branch has no subagent guard; body >9000 chars spills to `~/.claude/context-required/<sid>/<skill>.md` with a read-in-full instruction | [D1](#d1) | [skill-context.sh:44-52,126-134](cmd/hooks/skill-context.sh) |
| <a id="f3"></a>F3! | The hook honors a bare `--no-context` token in `tool_input.args` as a per-invocation opt-out | [D1](#d1), §1 | [skill-context.sh:59-63](cmd/hooks/skill-context.sh) |
| <a id="f11"></a>F11! | Context entries carry no usage stats/confidence/labels; the only lifecycle mechanism is manual `tombstones` in context.json | [D3](#d3), [D4](#d4) | context/context.json (`tombstones` key) |
| <a id="f13"></a>F13! | Workflow journals each agent's return to `<transcriptDir>/journal.jsonl` with per-agent `agent-<id>.jsonl` transcripts; lane activity is absent from the top-level session JSONL | [D2](#d2), [D5](#d5), §3 | Workflow tool contract |
| <a id="f4"></a>F4 | read-gate.sh exempts subagents from the spill-file gate (`[ -z "$agent_id" ]`) — lanes are instructed but not forced to read the spill; the actual read rate is an empirical trace number | §4 runbook | [read-gate.sh:83](cmd/hooks/read-gate.sh) |
| <a id="f5"></a>F5 | railroad-review declares the full doctrine (`ACTION-CONCEPT-*, ACTION-IMPL-*, ACTION-REVIEW-*, RULE-PLAN-*, RULE-COMMIT-*, FACT-*`), ~45KB rendered in this repo | [D1](#d1) | [SKILL.md:6](skills/quality/railroad-review/SKILL.md) |
| <a id="f6"></a>F6 | Branch builders: lanes `claude/railroad-{direction}[-{chunkSlug}]-l{i}-r{r}-{hash6}` (js:80-82), refuters `claude/railroad-refute-{slug}-r{r}-{hash6}` (js:364), station `claude/railroad-station-r{r}-{hash6}` (js:130,532); cleanup enumerates these exact names + `-2` variants via `remove_agent_worktrees.sh --delete-branch` (js:124-147); lane prompt prose cites "the claude/* namespace" | [D6](#d6), §2 | [railroad-review.js](skills/quality/railroad-review/workflows/railroad-review.js) |
| <a id="f8"></a>F8 | context_record.sh already extracts injected blocks and Agent-prompt context from transcripts — the jq idioms the runbook recipes reuse | [D5](#d5), §4 | [context_record.sh:52-72](cmd/context/context_record.sh) |
| <a id="f7"></a>F7 | changelog.json `[0].version` must match frontmatter `version:` (currently 8.2) — any skill edit bumps both | §1, §2 | skills/quality/railroad-review/changelog.json |
| <a id="f9"></a>F9 | Entry `enforcement` field distinguishes `gate`/`lint` (mechanically enforced) from `review` — gate-enforced entries are excluded from eviction screening | [D4](#d4), §5 | context/context.json entries |

## Exemplar & reuse

| Existing | Used for |
|---|---|
| [context_record.sh:52-72](cmd/context/context_record.sh) jq idioms | injected-block and read extraction in the runbook recipes |
| skill-context.sh `--no-context` token ([F3!](#f3)) | the entire lane-context fix — no hook change |
| `plans/<feature>/runbooks/` convention | home for the trace runbook |
| tombstones mechanism in context.json | eviction endpoint |

- **Without exemplar:** the peek-mcp verbose-mode contract (§3) — no prior spec for exposing subagent transcripts exists; mitigated by it being a spec here, implemented in its own repo with its own review.

## Changes

### 1. Lane context fix (modified)

location: `skills/quality/railroad-review/workflows/railroad-review.js`

```diff
 function lanePrompt(direction, i, chunk) {
   // ...
-    `Invoke the Skill tool with skill="railroad-review", then follow the "${direction}" direction definition in its ` +
+    `Invoke the Skill tool with skill="railroad-review" and args="--no-context" (lanes get no doctrine injection — ` +
+    `CTX-002), then follow the "${direction}" direction definition in its ` +
     `SKILL.md exactly — read only the context that direction needs, walk only that direction's checklist. ` +
```

- **SKILL.md:** CTX-002's parenthetical corrected from "(they never invoke a skill)" to name the real mechanism — lanes invoke the skill with `--no-context`; the direction's named entries remain the lane's only context.
- **changelog.json + frontmatter:** version bump (8.2 → 8.3) with an entry naming both this fix and §2.

### 2. Branch namespace claude-review/* (modified)

location: `skills/quality/railroad-review/workflows/railroad-review.js`, `skills/quality/railroad-review/SKILL.md`

- Per [D6](#d6), the three branch builders change prefix; structure preserved:

```diff
 function laneBranchName(direction, chunk, i) {
-  return chunk ? `claude/railroad-${direction}-${chunkSlug(chunk.name)}-l${i}-r${r}-${hash6}`
-               : `claude/railroad-${direction}-l${i}-r${r}-${hash6}`
+  return chunk ? `claude-review/${direction}-${chunkSlug(chunk.name)}-l${i}-r${r}-${hash6}`
+               : `claude-review/${direction}-l${i}-r${r}-${hash6}`
 }
```

- Refuter (`js:364`) and station (`js:130,532`) builders get the same `claude/railroad-` → `claude-review/` substitution (exact diffs analogous; refuters drop to `claude-review/refute-…`, station to `claude-review/station-…`).
- **Prose sweep:** every "claude/* namespace" mention in lanePrompt / refuterPrompt / station prompt / cleanup notes becomes "claude-review/* namespace"; SKILL.md branch-name mentions (ROUND/PROC sections) updated to match — the cleanup enumeration in `runCleanup` (js:124-147) derives from the same builders, so it follows automatically.

### 3. peek-mcp verbose mode (future work — contract spec only, no dependency)

location: peek-mcp repo (github.com/kevinhorst/peek-mcp) — peek stays at v1.2.1 throughout this plan; nothing below is implemented or assumed available here. The contract is recorded so the future implementation matches what the runbook needs:

- **`session_agents` (new tool):** input `{agent: claude|codex, id}` → list of the session's subagent transcripts: `{agent_file, label, first_ts, last_ts, tokens_in, tokens_out, tool_call_count}` — discovered from `journal.jsonl` + `agent-*.jsonl` in the session's transcript directory.
- **`session_agent_events` (new tool):** input `{id, agent_file, events_only: true}` → per tool call: `{ts, tool, key_args (file paths, commands, skill names), usage}`; with `events_only: false` the full transcript text, paginated with the existing `has_more`/`request_id` mechanism.
- **`session_full` gains `include_thinking: bool`:** main-agent thinking blocks rendered inline; subagent thinking never included ([D2](#d2) [USER]).
- Consumers here: the §4 runbook is disk-jq only for this plan; migrating its recipes to these tools is part of the future peek-mcp work, not this one.

### 4. Trace runbook (new)

location: `plans/railroad-review-observability/runbooks/trace.md`

- jq recipes over the on-disk workflow transcript dir (peek-mcp v1.2.1 assumed — no peek tools used), one recipe per question, each a fenced command with real paths:
  - **lane roster:** journal.jsonl → agent file ↔ direction/lane join.
  - **context reads per lane:** Read/Bash tool calls in `agent-*.jsonl` targeting `context/` — compared against the direction's Context-requirement globs expanded via `context/context.json`.
  - **injection check:** presence of the `===== SKILL CONTEXT` block or a `context-required/<sid>/railroad-review.md` read in the lane transcript (the [F4](#f4) read-rate number).
  - **cost:** per-agent tokens_in/out from `usage` fields, wall time from timestamps; rollup per direction and per round.
  - **step adherence:** first tool call is `git rev-parse HEAD`; branch rename present; no Edit/Write in the worktree.
- Closing run line: the before/after procedure — one railroad round on a real branch at v8.2, one at v8.3, same scope args; table of per-lane injected tokens, total round tokens, wall time.

### 5. Eviction screening recipe (new)

location: `plans/railroad-review-observability/runbooks/eviction-screen.md`

- Input: ≥3 traced real runs (their trace outputs + `round-*/review.json`).
- Rule ([D4](#d4)): candidate = never read by a lane whose direction requires it ∧ never cited in any finding ∧ `enforcement != gate|lint`.
- Output: candidate table (entry, direction(s), runs observed, evidence pointers) saved under `evals/railroad-review-<date>/eviction-candidates.md` — input to a manual tombstone decision, never an automatic removal.

## Hot items

N/A — no hot-class code: no SQL, no concurrency, no interfaces/generics, no anonymous structs, no validation/guard changes, no UI. The only code edits are string-level changes in railroad-review.js, shown as diffs above.

## Tests

| Location.Method | Cases | Comment |
|---|---|---|
| existing skill checks (`make audit`: acdsl skill-entries, changelog/version sync) | version bump consistent<br>SKILL.md entry grammar intact after CTX-002 edit | no new test modules — the js is exercised by real runs, not unit tests (no js test infra exists for workflows) |
| not tested: workflow js behavior, because no workflow-js test harness exists in the repo | — | verification is the live before/after round (§4 runbook) |

## Test runbook

- **before-trace:** run `/railroad-review` (defaults) on a real branch at v8.2; run the §4 recipes — expect non-empty injection evidence per lane (confirms [F1](#f1)/[F2](#f2) empirically). Data source: workflow transcript dir.
- **after-trace:** same scope at v8.3 — expect zero injection evidence, `claude-review/*` branches created and cleaned, findings quality subjectively comparable.
- **eviction screen:** after ≥3 real v8.3 runs, run the §5 recipe — expect a candidate table (possibly empty).
- peek verbose scenarios live with the peek-mcp repo's own tests once implemented.

## Contracts & sweeps

| Contract | Sides | Sweep |
|---|---|---|
| `--no-context` token semantics | skill-context.sh:59-63 / lanePrompt §1 | after-trace proves zero injection; grep railroad-review.js for `no-context` = exactly the §1 site |
| branch names ↔ cleanup enumeration | branch builders / runCleanup (same file) | `grep -n "claude/railroad" skills/quality/railroad-review/` → zero after §2 (js + SKILL.md prose) |
| SKILL.md version ↔ changelog.json | frontmatter / changelog `[0].version` | `make audit` acdsl gate |
| peek verbose tool names/fields | this plan §3 / future peek-mcp implementation | dormant until that work starts — no side exists yet at v1.2.1; reconciled at peek-mcp review time |

## Verification

- [ ] Run `make audit` — expect green (skill-entry grammar, changelog/version sync).
- [ ] Run `grep -rn "claude/railroad" skills/quality/railroad-review/` — expect zero matches.
- [ ] Run the before-trace on a real branch — expect per-lane injection evidence present at v8.2 (empirical confirmation of the leak).
- [ ] Run the after-trace at v8.3 — expect zero lane injection, branches under `claude-review/*`, cleanup leaves no leftover branches (`git branch --list 'claude-review/*'` empty after the round).
- [ ] Compare before/after totals — expect a per-lane input-token drop roughly matching the rendered doctrine size (~45KB ≈ 11k tokens) plus the spill re-read where lanes obeyed it; report the actual numbers.
- [ ] Degenerate case: a lane that aborts (premise mismatch) appears in the trace as aborted, not as a recipe crash.
- [ ] Eviction screen over ≥3 runs produces a candidate table with evidence pointers, saved under `evals/`.

## Stop conditions

| ID | Condition | Action |
|---|---|---|
| S1 | An approved signature/contract can't hold as planned | Stop and report — never improvise architecture mid-edit |
| S2 | Second failed approach in a row on one unit | Stop, re-read actual state, write a plan — no third attempt |
| S3 | Missing prerequisite (generated code, infra) | Run the producing step; if infra is down, ask |
| S4 | Discovered work materially exceeds approved scope | Ask before continuing |
| S5 | Same bug class found twice | Fix all in-diff instances; pre-existing outside the diff: report and ask |
| S6 | Structural obstacle tempts a new abstraction | Stop and report; relocate, don't indirect |
| S7 | After-trace shows review quality visibly degraded (findings clearly poorer on the same branch) | Stop; the recall question is now live — revive the deferred causal harness as its own plan |
| S8 | Any tooling found to depend on the `claude/railroad-*` names beyond the skill's own cleanup | Stop and report before renaming |

## Changelog

| Date | Trigger | What changed |
|---|---|---|
| — | initial | plan created |
| 2026-08-26 | review feedback | leviathan (benchmark/ablation/scorer/cell driver) deferred; peek verbose mode added as separate-repo contract; `--no-context` fix promoted from experiment arm to direct spec restoration; `claude-review/*` prefix applied to the skill's own branches; D7/D8 answered [USER] |
| 2026-08-26 | review feedback | peek-mcp pinned at v1.2.1 — verbose mode demoted from parallel work item to future-work spec; runbook is disk-jq only, no peek dependency anywhere |
