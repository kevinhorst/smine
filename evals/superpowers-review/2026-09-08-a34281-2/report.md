# superpowers-review golden eval — 2026-09-08 (32-replica run)

Range: b4981b88d52deaa69d6ed1d60eb51b57ead47ba0..a342811b4cf7b8bb2b54359ced6c786d3b5f349b | overlay: smine-1.1.4-1.2.1.overlay.patch | golden findings: 252 (v6, scope full) | parallel: 32

Verdict: **fail** — 0 of 32 arms pass either threshold (precision >= 0.8, weighted_recall >= 0.35). Best arm: sonnet-5-high (r7, 5 matches).

## Setup

| Constraint | Value |
|---|---|
| Arms | claude-sonnet-5[1m] x high x r32 (parallel 32, single-pass fanout wf_846085a6-982) |
| Arm shape | single-pass by design: one reviewer subagent, vendored obra/superpowers code-reviewer.md (pin b36e082), round cap 1 |
| Route | Desktop fanout workflow, template mode (shared reviewerPromptTemplate/judgePromptTemplate, `__RUNID__` substitution — added this run to avoid a 256KB args blob; rendered prompts byte-identical to the inline form) |
| Budgets USD (arm / judge / window) | 60 / 60 / 150 — accepted-not-enforceable on this route |
| Context | frozen pack on: 163 entries, freeze sha verified; every reviewer read it in full (preamble) |
| Seeded known ledger | off |
| Golden | smine-1.1.4-1.2.1.v6.json (252 findings, 16 human-ruled false positives) |
| Judges | claude-sonnet-5[1m] / high, one per arm, in-pipeline in the live arm worktree |
| Self rows | none (prose wrapper SKILL.md, `--list-entries` = `[]`) |

## Aggregate

| Aggregate (n=32 arms) | Missed | Weighted recall | Precision | False positives | Uncataloged | Recall | Wall min |
|---|---|---|---|---|---|---|---|
| per-arm mean (adjudicated) | **98.8%** (mean 3.0 of 252) | 0.0175 (mean) | 0.6853 (pooled: 95 matched / 43 FP) | **43** total | 42 confirmed instances (30 distinct) | 0.0118 mean | 35.9 mean / 44.5 max |
| 32-arm union (this run, adjudicated) | **88.9%** (224 of 252) | — | — | 43 total | 42 instances | 0.1111 (28 of 252) | — |

- 32 arms produced 183 transcribed findings: 83 matched pairs onto only 27 unique golden ids, 35 false positives, 65 uncataloged rulings. Even pooling ALL 34 arms ever run misses 89.3% of the golden by count and 86.2% by weight — saturation, not sampling noise: doubling arms from 17 to 34 added single new ids only in the tail (frequency-1 ids).
- **Uncataloged re-verification (2026-09-09, uncataloged-verification.json):** the 65 rulings are per-arm counts with cross-arm duplication — they dedup to 45 distinct claims (uncataloged-groups.json). Three adversarial verifiers re-adjudicated every group against golden v6 and the code: **38 distinct claims confirmed genuinely new** (51 of the 65 instances); 3 groups (9 instances) actually match existing golden entries — judge errors (the CheckPolicy unresolvable-base family → GV3-108, a skill-invoke-gate variant → GV3-166, casual-lockout prose-only → GV5-237); 3 groups false (the dist.go "mode free" premise is wrong — smine's policy is gated; a type-placement style claim matches the file's own convention; the README no-plan-doc claim is process, not a defect); 1 group (2 instances) out of range (the D1-reversal claims rest on plans/, which the diff excludes). Never read the Σ tile as unique new findings.
- **Double verification + adjudication (2026-09-09, uncataloged-verification-r2.json, unclear-claims-analysis.md):** the 38 first-pass-confirmed groups were re-adjudicated by two fresh independent verifiers (four-leg protocol, no delegation), strict both-confirm fold, lower severity wins — 28 both-confirmed, 8 splits, 2 rejected by both. Each split then got a dedicated prosecutor/refuter agent pair and a ruling: 2 more confirmed (g33 repos name-field plan deviation MAJOR, g29 styleTestPrompt Changelog gap MINOR), 2 more golden matches (g14 → GV3-147, g21 → GV6-252), 4 claim-false. **Final: of 45 distinct claims — 30 confirmed (4 MAJOR, 12 MINOR, 8 NIT, 6 INFO; 42 instances, Σ weight 73), 6 golden matches, 7 false, 2 out of range.** Scores are corrected to this adjudicated state directly in `armruns.jsonl`'s judge blocks and top-level recall/false_positives/uncataloged/misclassified scalars (not just a metricValues patch) — this eval.json is a clean re-fold from that corrected source, the single source of truth for every downstream view (Result table, union bar, golden.normalized re-scoring): 12 instances re-credited as golden matches (recall 1.03%→**1.18%** mean, union 26→**28** goldens — the Tests-page union bar now matches), 8 instances became false positives (pooled precision 70.3%→**68.5%**), and **`weighted_recall_incl_new`** (per-run confirmed-instance credit against golden weight 449) means **2.26%** vs **1.75%** plain. `golden.normalized` was regenerated via `normalize.sh` off the corrected data, so mean efficiency also moved (0.29×→**0.32×**). Verdict thresholds still bind on plain weighted_recall — the run's `fail` stands.
- **Coverage-conditioned scoring (2026-09-09, touched_files mined from workflow transcripts):** golden_coverage (mean **40.5%** of golden severity weight — a few hot files carry disproportionate weight) and weighted_recall_reachable (mean **4.9%**, barely above plain weighted recall) factor recall into where the arm looked vs how well it reviewed what it looked at; identity coverage × reachable = plain weighted_recall holds exactly per run. See superpowers-railroad-review-comparison.md.
- Frequency of matched golden ids is heavily skewed: GV3-166 (skill-invoke-gate two-shape gap) 15/34 arms, GV3-001 (policy base_ref free-mode short-circuit) 12/34, GV3-198 (ACDSL allowlist removal) 8/34, GV3-065 (GitRevParse test-only export) 7/34; 14 of the 27 ids were found by exactly one arm.
- Judge repairs: r17 and r29 judges truncated their `missed` enumeration; repaired mechanically as golden-minus-matched (the headless merge's own formula); originals kept as judge.json.orig. All 32 partitions validate clean after repair.
- Worktree hygiene: r9 and r20 reviewers reverted the uncommitted context overlay in-tree mid-review (same read-only breach as yesterday's r2; pack context unaffected). Salvage found all 32 trees clean; all worktrees and `claude-review/eval-*` branches removed unforced. The leftover eval branches/worktrees visible in the smine repo belong to the concurrent railroad-review eval session (skillroutine-eval-railroad-078bb3) and were not touched.
- Telemetry: EXACT, from the workflow's per-agent transcripts (usage rows deduped by message.id, output as per-id max) — see telemetry-subagents.json. Fanout totals: 129 agents, 3,387 API calls, 13.58M cache-creation, 608.1M cache-read, 2.34M output tokens (reviewers 1.67M output over 2,416 calls, judges 571k over 740). Per-arm `output_tokens`/`cache_*`/`turns` in armruns/eval.json are the arm cells only (setup+review+transcribe, judges excluded), Σ 1.77M output; the earlier `budget.spent()` deltas were overlapping and are superseded (kept as armruns.jsonl.approx). `cost_usd` stays null — no reconciled per-arm price. Wall 50.9 min for the whole fanout.
- Normalized efficiency (schema 2.5): mean composite 0.290 vs golden-production pace, all three axes real (tokens axis now measured).

Per-run rows: armruns.jsonl / eval.json. Combined 34-arm review deliverable: combined-review.md.

## Timeline (UTC)

Dispatch 19:38:59 → preflight 25s → 32 arms parallel (harness-capped) → workflow complete 20:30 → fold/report 20:31. 129 agents, 3356 tool calls, 0 errors.
