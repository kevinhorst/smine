# superpowers-review golden eval — 2026-09-07

Range: b4981b88d52deaa69d6ed1d60eb51b57ead47ba0..a342811b4cf7b8bb2b54359ced6c786d3b5f349b | overlay: smine-1.1.4-1.2.1.overlay.patch | golden findings: 245 | parallel: 1

Verdict: **fail** — best arm sonnet-5-high

## Setup

| Constraint | Value |
|---|---|
| Arms | claude-sonnet-5[1m] x high x r1 (parallel 1) |
| Arm shape | single-pass by design: one reviewer subagent, vendored obra/superpowers code-reviewer.md (pin b36e082), no lanes, no refutation, round cap 1 |
| Route | Desktop workflow (eval-arm-singlepass.js, run wf_951281dd-46f), session "Superpowers Skil-Eval" (d1c6793e) |
| Budgets USD (arm / judge / window) | 60 / 30 / 150 |
| Context | none by design (superpowers ships no ACDSL context); golden overlay applied to the arm worktree |
| Seeded known ledger | off |
| Golden | eval/golden/smine-1.1.4-1.2.1.v5.json v5 (245 findings, 16 human-ruled false positives) |
| Overlay | smine-1.1.4-1.2.1.overlay.patch |

## Interpretation

- Verdict is **fail** against thresholds: precision >= 0.8, weighted_recall >= 0.35. An arm passes only when VALID and inside every threshold.
- sonnet-5-high-r1: failed precision, weighted_recall.
- Scored against golden v5 (245 findings) — recall numbers are NOT comparable to reports scored against earlier golden versions.
- Golden re-pin mid-run: the definition still pinned v4 while railroad-review had moved to v5 (the README coupling this arm's docs warn about). The pin was fixed in the source and deployed definitions, and the arm was re-judged against v5; the v4 judgment is preserved as `runs/sonnet-5-high-r1/judge.v4.json`. All five dispositions are identical under v4 and v5 — only the denominator changed (218 → 245).
- **Reviewer coverage caveat:** the reviewer's report claims the full 178-file / 7,314-insertion scoped diff (verified against git with the golden excludes), but its actual reading was shallow — observed live at ~21 file reads, with the plan documents under `plans/v1-2-1/` only partially read. The report's own methodology note admits pass-based sampling ("deep-dive on the highest-risk units, lighter pass over the rest"). The 99.6% miss rate reflects that coverage, not just prompt quality.
- The single match, code-verified after judging: SP-002 ↔ GV3-065 (GitRevParse test-only export) is a real defect — `GitRevParse` is introduced in this range at internal/acdsl/git.go:78, exported, and its only caller is git_test.go:13; the acdsl-policy-modes plan (C2 "Git helpers") declares it but wires only its three siblings (GitMergeBase, GitChangedFiles, GitShowFile — all called from policy.go). Golden severity INFO (dead-export cruft, not a bug) stands; the arm's MAJOR is the misclassification.
- False positives: SP-001 (verdict.sh untracked-filter provenance — intentional, plan-consistent tooling filter), SP-004 (handleReposAdd git init side effect — C19 plan-backed, non-destructive), SP-005 (German i18n prose taste, no concrete defect). Uncataloged: SP-003 (implicit CWD==repo-root path contract in presentation.go/installenv.go) — golden-enrichment candidate.
- A/B note vs railroad-review same range: the railroad run at evals/railroad-review/2026-09-06-a34281-182536 was a seeded-ledger saturation run scored against v4 — not a valid counterpart for these numbers; compare via a fresh railroad v5 run.
- Metric definitions and validity gates: see skills/quality/superpowers-review/eval/README.md.

| Arm (n runs) | Missed | Weighted recall | Precision | False positives | Uncataloged | Misclassified | Rounds | Recall | Cost USD | Turns | Output tokens | Wall min |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| sonnet-5-high (n=1) | **99.6%** (244 of 245) | 0.0023 | 0.25 | **3** | 1 | 1 | 1 | 0.0041 | n/a | n/a | ~47043 (budget delta) | 13.2 |

## Session events & telemetry (peek, session d1c6793e "Superpowers Skil-Eval")

| Stage | Agent | Model | Wall | Output tokens (telemetry) |
|---|---|---|---|---|
| Arm setup (worktree + overlay) | a46904e0 | claude-opus-4-8 | 25s | 10 |
| Reviewer (vendored prompt) | ae3eebaa | claude-sonnet-5 | 740s | 6,441 |
| Transcriber (Issues → review.json) | af259647 | claude-opus-4-8 | 26s | 287 |
| v4 golden judge | a713faf1 | claude-opus-4-8 | 262s | 1,047 |
| v5 delta judge | af8bf17e | claude-fable-5 | 33s | 661 |

- Arm workflow total: 792s wall, 3 agents, 86 tool uses, 0 errors; `output_tokens` in eval.json (47,043) is the workflow `budget.spent()` delta — the telemetry-measured reviewer output was 6.4k tokens.
- Session totals at report time: wall 38.6 min (18.8 min active), 66.4k session output tokens, cache 398k created / 8.1M read; telemetry cost USD 24.78 for the whole session (eval orchestration + judges + this report — not attributable to the arm alone, so `cost_usd` stays null in eval.json).
- Events: 3 skill invocations (2x reload-skills, skillroutine-eval superpowers-review), permission mode → bypassPermissions at 00:36Z, session model opus-4-8 → fable-5 at 00:46Z (which is why the v4 judge ran on opus-4-8 and the v5 delta judge on fable-5), 0 permission denials, 171 auto-allowed permissions.
- Timeline (UTC): invoke 00:17:41 → arm workflow 00:23:20–00:36:32 → v4 judge 00:38:33–00:42:55 → golden re-pin + v5 delta judge 00:48:04–00:48:37 → fold/report.

Statistics: see stats.json (single cell — no comparisons).
