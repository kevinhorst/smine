# aqms-review-wip golden eval — 2026-09-09-a34281 (smoke round)

| Setup | |
|---|---|
| Route | Desktop per-arm dispatcher (`eval-arm-singlepass.js`), parallel 1, 1 arm — operator-requested smoke round |
| Arm | claude-sonnet-5[1m] / high (definition default; operator confirmed sonnet-5) |
| Golden | smine 1.1.4→1.2.1 v6, scope full, 252 findings, 16 ruled false positives |
| Context | knob ON — frozen pack rendered inline (preflight: overlay index sha `f0b43c85…` matches freeze, 163 ids), read by the reviewer via `contextPack` preamble |
| Reviewer prompt | `run.desktop.render: body` — wip-review.md body verbatim + 4 dispatch-note lines with real range SHAs; transcribe from `run.desktop.transcribe` |
| Seed | off (SKILLEVAL_SEED_KNOWN unset) |
| Env resolve | no env block pasted — nothing applied/ignored; budgets 45/30/150 accepted-not-enforceable on this route |
| Self rows | none (prose wrapper SKILL.md, `rules render-skill --list-entries` = `[]`) — same as the superpowers runs |
| Judges | Agent-tool judge, model alias `sonnet` (claude-sonnet-5); effort not settable on the Agent tool (session-inherited) — deviation from the fanout route's explicit judgeModel/judgeEffort, recorded here |
| Session model | claude-fable-5 (dispatcher/fold only — no review or judge work) |

## Result (missed % first; arm n=1)

| Arm (n=1) | Missed % | Weighted recall | Recall | Precision | FP | Misclassified | Uncataloged | Rounds |
|---|---|---|---|---|---|---|---|---|
| sonnet-5-high | **98.8%** (249 of 252) | 1.56% | 1.19% (3) | 1.00 | 0 | 1 | 1 | 1 |

Verdict: **fail** — weighted_recall 0.0156 < 0.35 threshold (precision 1.00 ≥ 0.8 passes). Expected for a 1-arm single-pass smoke against the full 252-finding golden; the run's purpose was wiring verification, not a competitive score.

## Judge outcome

- MAJ-1 (skill-invoke-gate.sh cat-shape bypass) matched **two** goldens: GV3-138 (relative-path) + GV3-166 (quoting/exact-shape) — verified with a 3-payload repro.
- MAJ-3 (sessions.go unconditional HX-Redirect) matched GV3-122; severity MAJOR vs golden INFO → 1 misclassified.
- MAJ-2 (acdsl-policy-guard.sh unconditionally denies `acdsl/registry.local.json` in gated/strict, contradicting README + `overlayViolations`) — **uncataloged**: genuine in-range defect the golden does not catalog; golden-enrichment candidate.
- 0 false positives. Judge partition validated: 3 matched + 249 missed = 252; every candidate in exactly one bucket.

## Telemetry

EXACT, from the workflow's three per-agent transcripts (usage deduped by message.id, output as per-id max): arm cells (setup+review+transcribe) 89 API calls, 76,950 output, 433,361 cache-creation, 17,744,462 cache-read tokens. Reviewer alone: 82 calls / 74,137 output. Wall 19.7 min (dispatch 09:43:03Z → return 10:02:42Z). `budget.spent()` delta was 82,244 (superseded, kept in the workflow return). Judge (excluded from arm telemetry per convention): 120,877 subagent tokens, 2.6 min. peek served no per-subagent rows for workflow agents — transcript accounting used instead, same method as the 32-arm superpowers run. cost_usd null (no reconciled per-arm price). Efficiency (normalize, vs golden-production baseline): composite **0.31×**.

## Smoke-round mechanics — all green

- Preconditions: definition schema 1.3 valid; all prerequisite files present; both range SHAs resolve; deployed SKILL.md byte-identical with `acdsl-context` declaration.
- Stale sweep clean; preflight sha + id-count exact; scratch worktree torn down.
- Arm valid: report persisted, transcription 3/3 findings (report's "MAJ-4" artifact filenames are the reviewer's own numbering slip for MAJ-3 — content consistent).
- Reviewer honored the dispatch notes: reviewed the committed range only, ruled out the overlay-induced `TestDistShipsReachCoveredSliceWithBinaries` failure as harness context, left the tree clean.
- Note: the reviewer ran its lanes inline rather than spawning lane/refuter subagents (1 transcript, 82 calls) — part of the skill under test, recorded, not corrected.
- Cleanup: overlay reversed, nothing to salvage, worktree + `claude-review/eval-sonnet-5-high-r1` removed unforced. leftover_worktrees empty.
- Fold validated (runIds + golden ids); evalstats + normalize (schema 2.5, efficiency block) clean.
