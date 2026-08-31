# fexplore — skill-run evaluation (2026-08-27)

Skill: `fexplore` (`skills/feature/fexplore/SKILL.md`) · manifest: `evals/fexplore-2026-08-27/manifest.json`
Brief: `routines/skill-eval-fexplore/briefs/fexplore/applied-probe-safety.md` — explore the layered applied-probe + unified verdict source, write `plans/applied-probe-safety/design/exploration.md`, do not read the archived design.
Runs: 4 ctx-on cells (fable-5 / opus-4-8 × medium / max), one replica each, `default` variant (no variants → no `sharedTotals`).

## Ranking

| Rank | Run | Self | Output | Cost | Wall | Bundle |
|---|---|---|---|---|---|---|
| 1 | **fable-5 · medium** | 93.3% (14/15) | **100%** (9/9) | **$0.81** | **195s** | 1 authored + 4 pre-existing |
| 2 | fable-5 · max | 93.3% (14/15) | 100% (9/9) | $2.70 | 822s | **1 (exploration only)** |
| 2 | opus-4-8 · max | 93.3% (14/15) | 100% (9/9) | $1.91 | 701s | 1 authored + 6 pre-existing |
| 4 | opus-4-8 · medium | 80.0% (12/15) | 77.8% (7/9) | $0.62 | 203s | 1 authored + 4 pre-existing |

Three runs tie on quality (23/24 rule points). **fable-5 · medium wins on value** — top-tier quality at the lowest cost and fastest wall time. **fable-5 · max** matches the quality with the cleanest single-artifact discipline (it grounded at the pre-implementation parent and never touched the shipped code) but at ~3.3× the cost. **opus-4-8 · max** matches quality at higher cost and the widest option survey. **opus-4-8 · medium** trails: it proposed no live measurement and mis-framed the merge-tree substrate.

Context axis is **N/A** — the injected context was the Go language pack (87 `RULE-GOLANG-*` ids, `injected.skill=[]`); the deliverable is a prose exploration with no Go, so code-doctrine is not demonstrable against it.

## Self axis (the skill's own directives)

| Rule | fable·med | fable·max | opus·med | opus·max |
|---|---|---|---|---|
| 001 one artifact, no code | +1 | +1 | +1 | +1 |
| 002 constraints first, C<n>+anchor | +1 | +1 | +1 | +1 |
| 003 facts carry anchors | +1 | +1 | +1 | +1 |
| 004 as-is vs fully-implemented | +1 | +1 | +1 | +1 |
| 005 live probe over archaeology | +1 | +1 | **0** | +1 |
| 006 design by measurement | +1 | +1 | **0** | +1 |
| 007 exhaustive families | +1 | +1 | +1 | +1 |
| 008 killed family + killing constraint | +1 | +1 | +1 | +1 |
| 009 evaluation table columns | +1 | +1 | +1 | +1 |
| 010 groundedness = rides existing code | +1 | +1 | +1 | +1 |
| 011 per-scenario verdicts | +1 | +1 | +1 | +1 |
| 012 no fake winner / OPEN | +1 | +1 | +1 | +1 |
| 013 template + what fdesign imports | +1 | +1 | +1 | +1 |
| 014 user candidate first-class | 0 | 0 | 0 | 0 |
| 015 mode = depth not rigor | +1 | +1 | +1 | +1 |
| **raw / 15** | **14** | **14** | **12** | **14** |

## Output axis (quality vs the exemplar exploration)

| Rule | fable·med | fable·max | opus·med | opus·max |
|---|---|---|---|---|
| C1 constraints anchored/measured | +1 | +1 | +1 | +1 |
| C2 exhaustive families + kill reason | +1 | +1 | +1 | +1 |
| C3 evaluation table + verdicts | +1 | +1 | +1 | +1 |
| C4 grounding honest & disclosed | +1 | +1 | +1 | +1 |
| C5 design-by-measurement backing | +1 | +1 | **0** | +1 |
| C6 what fdesign imports | +1 | +1 | +1 | +1 |
| C7 rejected register | +1 | +1 | +1 | +1 |
| C8 both axes surveyed | +1 | +1 | +1 | +1 |
| C9 surfaces merge-tree substrate | +1 | +1 | **0** | +1 |
| **raw / 9** | **9** | **9** | **7** | **9** |

## Metrics

| Metric | dir | fable·med | fable·max | opus·med | opus·max |
|---|---|---|---|---|---|
| wall_s | lower | **195** | 822 | 203 | 701 |
| output_tokens | lower | **13,359** | 58,798 | 13,787 | 47,984 |
| cost_usd | lower | 0.81 | 2.70 | **0.62** | 1.91 |
| files in bundle | lower | 5 | **1** | 5 | 7 |
| constraints enumerated | higher | 11 | **14** | 9 | 9 |
| option families surveyed | higher | 11 | 10 | 11 | **13** |
| entry_citations | higher | 0 | 0 | 0 | 0 |
| audit_pass | higher | null | null | null | null |
| diff_lines | lower | null | null | null | null |

`entry_citations` is uniformly 0 (fexplore defines no `SKILL` entry ids; the exploration anchors with `file:line` instead). `audit_pass` / `diff_lines` are null — cross-directory Bash in the cell worktrees is approval-gated and this was an unattended run.

## Non-+1 justifications

| Rule | Runs | Why |
|---|---|---|
| 014 user candidate | all four (0) | Not demonstrable — the brief supplies no distinct user candidate; the concept defines the space. Uniform, no ranking effect. |
| 005 live probe | opus·med (0) | Surfaces no blocked capability question and proposes no live experiment; Open Questions cover only A-WIDE scope and cache deferral. |
| 006 design by measurement | opus·med (0) | No runnable query written or handed over; the ground-truth separation is cited from the concept's prior decision-127 work, not measured here. |
| C5 measurement backing | opus·med (0) | The load-bearing separation is asserted via citation to the prior decision-127 measurement rather than a runnable-and-handed-over check. |
| C9 merge-tree substrate | opus·med (0) | Raises merge-tree only as A3 and kills it ("adds no disambiguation … loses the lazy-worktree reuse"), missing its role as a worktreeless layer-2 substrate — the OPEN the other three flagged. |

## Mechanical probes

- **context-injection-shape** — `injected.skill=[]`, one Go lang pack (87 ids) on the sampled transcripts; context axis N/A.
- **code-bundle-provenance** — three outputs bundle implementation files under `## file:` headers, byte-identical across two different models and matching the on-branch shipped files (incl. the `[ACDSL-PROJECTION] … stripped before commit` working-copy header). Not run-authored code — the harness surfaced the working-copy projection view of files the run read. `SKILL-FEXPLORE-001` not violated by any run. fable-5.max bundles only exploration.md.
- **archived-design-prohibition** — all four disclose non-consultation; option taxonomies differ from the golden's, consistent with independent derivation.
- **concept-vs-shipped-contradiction** — concept MVP says "FROM only" while shipped code does a candidate-wide twin sweep; all four surfaced the tension.
- **merge-tree-substrate-coverage** — only opus-4-8.medium fails to raise the worktreeless merge-tree substrate as a live option.

## Notes

The `qualityContext` exemplar (`plans/archived/applied_probe_safety/design/exploration.md`) is the downstream layer-3 exploration, used as a structure/rigor standard, **not** a content answer key (the brief covers the broader concept). Self rows are synthesized `SKILL-FEXPLORE-NNN` because fexplore is a prose skill with no formal entry ids (`render-skill --list-entries` returned `[]`); each anchors to a directive line in the loaded `skills/default.md`.
