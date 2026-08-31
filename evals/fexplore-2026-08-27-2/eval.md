# fexplore — skill-run evaluation (2026-08-27)

Six ctx-on runs of one prose brief (**applied-probe safety**) across two models — `claude-fable-5` and `claude-opus-4-8[1m]` — at three efforts (medium / high / max). Manifest: `evals/fexplore-2026-08-27-2/manifest.json`.

**Axes:** `self` (15 synthesized `SKILL-FEXPLORE-NNN` rows — fexplore is a prose skill, `render-skill --list-entries` returned `[]`, so rows are anchored to directive lines in the loaded `skills/default.md`), `output` (9 `C<n>` quality rows judged against the exemplar `plans/archived/applied_probe_safety/design/exploration.md`). **`context` axis is N/A** — `context_record.sh` shows `injected.skill=[]` (fexplore declares no `context:` frontmatter); no context rows, no context totals.

Run id legend (all `default.ctx-on.applied-probe-safety.*`): **fm** fable-5.medium · **fh** fable-5.high · **fx** fable-5.max · **om** opus-4-8.medium · **oh** opus-4-8.high · **ox** opus-4-8.max.

## Self axis (`SKILL-FEXPLORE-NNN`)

| Rule | fm | fh | fx | om | oh | ox |
|------|----|----|----|----|----|----|
| 001 one artifact, no plan/code | +1 | +1 | +1 | +1 | +1 | +1 |
| 002 constraints first, C<n>+anchor | +1 | +1 | +1 | +1 | +1 | +1 |
| 003 facts carry file:line anchors | +1 | +1 | +1 | +1 | +1 | +1 |
| 004 gaps vs limits, owner intent | +1 | +1 | +1 | +1 | +1 | +1 |
| 005 live probe when blocked | +1 | **0** | +1 | **0** | **0** | **0** |
| 006 design by measurement | +1 | +1 | +1 | +1 | +1 | +1 |
| 007 exhaustive families | +1 | +1 | +1 | +1 | +1 | +1 |
| 008 killed families shown w/ constraint | +1 | +1 | +1 | +1 | +1 | +1 |
| 009 eval table, prescribed columns | +1 | +1 | +1 | +1 | +1 | +1 |
| 010 groundedness = rides existing code | +1 | +1 | +1 | +1 | +1 | +1 |
| 011 per-scenario verdicts allowed | +1 | +1 | +1 | +1 | +1 | +1 |
| 012 genuine tie → OPEN, never fake | +1 | +1 | +1 | +1 | +1 | +1 |
| 013 exploration.md template | +1 | +1 | +1 | +1 | +1 | +1 |
| 014 user candidate first-class | **0** | **0** | **0** | **0** | **0** | **0** |
| 015 mode stated, depth not rigor | +1 | +1 | +1 | +1 | +1 | +1 |
| **Self raw / 15** | **14** | **13** | **14** | **13** | **13** | **13** |
| **Self pct** | 93.3 | 86.7 | 93.3 | 86.7 | 86.7 | 86.7 |

## Output axis (`C<n>` quality)

| Rule | fm | fh | fx | om | oh | ox |
|------|----|----|----|----|----|----|
| C1 constraints anchored/measured | +1 | +1 | +1 | +1 | +1 | +1 |
| C2 exhaustive families, killed shown | +1 | +1 | +1 | +1 | +1 | +1 |
| C3 eval table + per-option verdict | +1 | +1 | +1 | +1 | +1 | +1 |
| C4 groundedness honest, real anchors | +1 | +1 | +1 | +1 | +1 | +1 |
| C5 design-by-measurement separation | +1 | +1 | +1 | +1 | +1 | +1 |
| C6 recommendation: what fdesign imports | +1 | +1 | +1 | +1 | +1 | +1 |
| C7 rejected register, one-line reasons | +1 | +1 | +1 | +1 | +1 | +1 |
| C8 surveys probe AND verdict-source | +1 | +1 | +1 | +1 | +1 | +1 |
| C9 non-obvious substrate alternative | +1 | +1 | +1 | **0** | +1 | +1 |
| **Output raw / 9** | **9** | **9** | **9** | **8** | **9** | **9** |
| **Output pct** | 100 | 100 | 100 | 88.9 | 100 | 100 |

## Metrics (ranked, never folded into rule totals)

| Metric | dir | fm | fh | fx | om | oh | ox |
|--------|-----|----|----|----|----|----|----|
| wall_s | lower | **178** | 281 | 543 | 224 | 346 | 565 |
| output_tokens | lower | **12500** | 20490 | 38215 | 14315 | 17651 | 39293 |
| cost_usd | lower | 0.582 | 1.031 | 1.629 | 0.891 | **0.781** | 1.526 |
| files_touched (want 1) | lower | 3 | 4 | 4 | **2** | 7 | 6 |
| constraints_enumerated | higher | 9 | 10 | **12** | 9 | 9 | 8 |
| options_surveyed | higher | 12 | 16 | 14 | 9 | 13 | **20** |
| entry_citations | higher | 0 | 0 | 0 | 0 | 0 | 0 |
| audit_pass | higher | n/a | n/a | n/a | n/a | n/a | n/a |
| diff_lines | lower | n/a | n/a | n/a | n/a | n/a | n/a |

`audit_pass` / `diff_lines` are null everywhere: both need cross-directory Bash in the cell worktree, which is approval-gated in an unattended run. `entry_citations` is 0 everywhere: a prose skill has no `SKILL-NNN` vocabulary — outputs cite `concept.md` / `verdict.sh` file:line anchors instead. `files_touched` surplus over 1 is projected read-files (ACDSL working-copy view), not authored output.

## Non-+1 justifications

### Self

| Rule | Run | Score | Justification |
|------|-----|-------|---------------|
| 005 | fh | 0 | Raises no blocked capability question and proposes no live experiment — A1-A9 all code-grounded, no version-gated merge-tree substrate surfaced. Phase-1 behavior not triggered; not demonstrable. |
| 005 | om | 0 | Surfaces no blocked capability question; Open Questions cover only B1-derive/emit and cache deferral. No substrate raised to probe. |
| 005 | oh | 0 | Surfaces merge-tree (A5) but resolves it by analysis ("subsumed by A4"), not by a live git-version probe. |
| 005 | ox | 0 | Raises merge-tree's version sensitivity (P4) but closes it by existing-standard reasoning ("the repo already standardizes on the cherry-pick probe"), not a proposed live experiment. |
| 014 | all six | 0 | Not demonstrable — the brief supplies no distinct user candidate; the concept defines the space. Uniform across runs, no ranking effect. |

### Output

| Rule | Run | Score | Justification |
|------|-----|-------|---------------|
| C9 | om | 0 | Re-derives only the shipped range-diff ladder (A1 patch-id → A2 re-pick → A3 -X theirs → A4 range-diff → A5 twin) + B1-B4 packaging; surfaces no worktreeless merge-tree / pickaxe / provenance-trailer substrate. The five other runs each surface at least one. |

## Ranking

**Self** (raw / 15): **fm 14 · fx 14** > fh 13 · om 13 · oh 13 · ox 13. The only self discriminators are 005 (live-probe-when-blocked) and the uniform 014=0. fm and fx earn 005 by proposing a live `git --version` check for the version-gated merge-tree substrate; fh/om/oh/ox either never surface a blocking capability question or resolve it by analysis.

**Output** (raw / 9): **fm · fh · fx · oh · ox all 9 (100%)** > om 8 (88.9%). Every run nails the template, constraint anchoring, evaluation table, honest groundedness disclosure, measured separation, fdesign-import block and rejected register. om alone loses C9 — it re-derives the shipped ladder without surfacing a substrate alternative.

**Overall read:** all six are high-quality, constraint-first surveys that converge on the shipped design (the in-tree implementation was visible during grounding and every run discloses it honestly as the incumbent/exemplar). Effort buys *breadth*, not correctness: `options_surveyed` climbs 9→20 and `constraints_enumerated` up to 12 at higher effort, while wall-time and tokens climb steeply (max costs ~2.8× the tokens of medium). fable-5.medium is the value pick — top self score, full output, lowest wall-time and tokens. opus-4-8.max is the breadth pick — 20 option families, test-file-anchored measurements, and the cleanest handling of rule 012 (it flags OQ1, twin-sweep acceptance strictness, as a genuine OPEN rather than faking a settled winner). opus-4-8.medium is the weakest cell: it drops both effort-sensitive discriminators (005 and C9).

**Caveat on 005:** the rule rewards proposing the cheapest live experiment when a capability question blocks. In this unattended run, live Bash was approval-gated, so the most a run could do was *name* the probe — which fm and fx did (`git --version` before pursuing merge-tree) and the others did not. The gap is real but narrow.
