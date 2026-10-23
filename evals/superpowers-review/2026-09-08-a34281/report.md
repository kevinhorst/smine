# superpowers-review golden eval — 2026-09-08

Range: b4981b88d52deaa69d6ed1d60eb51b57ead47ba0..a342811b4cf7b8bb2b54359ced6c786d3b5f349b | overlay: smine-1.1.4-1.2.1.overlay.patch | golden findings: 252 (v6, scope full) | parallel: 2

Verdict: **fail** — best arm sonnet-5-high (r1)

## Setup

| Constraint | Value |
|---|---|
| Arms | claude-sonnet-5[1m] x high x r2 (parallel 2, single-pass fanout) |
| Arm shape | single-pass by design: one reviewer subagent, vendored obra/superpowers code-reviewer.md (pin b36e082), no lanes, no refutation, round cap 1 |
| Route | Desktop fanout workflow (eval-arm-singlepass-fanout.js nesting eval-arm-singlepass.js, run wf_444c5bf7-6cd), judges in-pipeline |
| Budgets USD (arm / judge / window) | 60 / 60 / 150 — accepted-not-enforceable on this route (no cost telemetry) |
| Context | frozen pack on: 163 entries rendered once from the overlay index (freeze sha f0b43c85… verified), read in full by both reviewers via the contextPack preamble |
| Seeded known ledger | off |
| Golden | ../railroad-review/eval/golden/smine-1.1.4-1.2.1.v6.json v6 (252 findings, 16 human-ruled false positives) |
| Judges | claude-sonnet-5[1m] / high (env unset, definition judge block has no model → arm's own model per the headless resolution) |
| Self rows | none — `rules render-skill --list-entries` returns `[]` and the SKILL.md body is an eval-only wrapper with no procedure; hand-derived fresh (nothing to derive), not reused |

## Interpretation

- Verdict is **fail** against thresholds: precision >= 0.8, weighted_recall >= 0.35. Both arms fail both thresholds.
- Scored against golden v6 (252 findings) — recall numbers are NOT comparable to the 2026-09-07 report (scored against v5, 245 findings), though the shape is identical: single-digit matches out of 250+.
- Replica agreement: both replicas independently found the same two golden defects via different wordings — GV6-252/GV3-137-adjacent config-server and skill-frontmatter issues do not overlap; the only shared match family is "small incidental defects". r1 matched GV6-252 (saveProfileSelection CWD contract), GV3-012 (shell.RunSync near-duplicate), GV3-166 (verdict.sh coarse command heuristic); r2 matched GV3-137 (smine-orchestrate unrestricted `Bash(git *)` grant), GV3-024 (build-install doc-comment copy-paste). Zero overlap between replicas — at this recall level the prompt samples the golden space essentially at random.
- False positives: r1 SP-001 (plans/ dir non-conformance — out of range, plans/ is excluded by the diff pathspec). r2 SP-002 (handleReposAdd permissions grant — out of range) and SP-006 (matches a known human-ruled false positive on .gitignore). Precision 0.75 / 0.50.
- Uncataloged (golden-enrichment candidates, neutral): r1 SP-002 (install.sh PEEK_PORT kill-loop edge case), r2 SP-001 (cmd/hooks/skill-invoke-gate.sh bug, new in range), r2 SP-005 (styleTestTarget in internal/server/profilesettings).
- **r2 worktree mutation:** the r2 reviewer violated its read-only instruction mid-review — it ran `git checkout --` on the overlay's tracked modifications and `rm -f` on the overlay's untracked additions (treating the uncommitted overlay as contamination while stabilizing its test runs), and created/removed a temporary base worktree at /tmp/review-base. The frozen context pack had already been read in full, so context delivery was unaffected; the in-tree overlay files (AGENTS.md et al.) were absent for the later part of its review. Cleanup found a clean tree; the overlay reverse step no-opped with errors. r1 was clean.
- Both reviewers read the frozen context pack in full (verified in transcripts) — this is the first superpowers run with the railroad-identical context pack, and recall did not move: 3 and 2 matches vs 1 in the 2026-09-07 no-context run. The A/B signal is that the vendored single-pass prompt, not context, is the binding constraint.
- Telemetry: EXACT, from the workflow's per-agent transcripts (telemetry-subagents.json; usage deduped by message.id). Arm cells (setup+review+transcribe, judges excluded): r1 40,332 output / 68 calls, r2 63,956 output / 85 calls; fanout totals 9 agents, 203 calls, 876k cache-creation, 30.7M cache-read, 131k output tokens. The earlier budget-delta figures are superseded (armruns.jsonl.approx). cost_usd stays null. Session model: fable-5 (orchestration agents); reviewers and judges claude-sonnet-5[1m].
- Leftovers: /private/tmp/review-head (detached smine worktree at the head SHA) predates this run — no agent in wf_444c5bf7-6cd touched it; left in place, flagged for manual removal.

| Arm (n runs) | Missed | Weighted recall | Precision | False positives | Uncataloged | Misclassified | Rounds | Recall | Efficiency | Output tokens | Wall min |
|---|---|---|---|---|---|---|---|---|---|---|---|
| sonnet-5-high r1 (n=1) | **98.8%** (249 of 252) | 0.0156 | 0.75 | **1** | 1 | 1 | 1 | 0.0119 | 0.477 | 40,332 | 10.4 |
| sonnet-5-high r2 (n=1) | **99.2%** (250 of 252) | 0.0089 | 0.50 | **2** | 2 | 1 | 1 | 0.0079 | 0.198 | 63,956 | 17.2 |

## Timeline (UTC)

Dispatch 00:28:15 → preflight (pack render, 28s) → arms 00:28:37 (parallel) → r1 review done 00:38:33, judged 00:39:04–00:41:27 → r2 review done 00:45:32, judged 00:45:51–00:49:12 → fold/report. Fanout wall 21.0 min, 9 agents, 201 tool calls, 1.058M output-token budget delta total.

Statistics: see stats.json.
