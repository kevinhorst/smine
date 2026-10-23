# Railroad-review golden eval

Golden-based evaluation of the railroad-review skill: headless arms review a pinned commit range of the smine repo with the skill's own command, and a judge scores their findings against a human-confirmed golden set.

## Contract

- `definition.json`, definition schema 4.0 — the skill's args as data, environment, findings extraction, scoring file, budgets, prerequisites, goldens with their `produce` and `test` blocks. Everything eval-specific lives there; the skill is never modified for a run.
- Run: `/skillroutine-eval railroad-review token: <label> golden: produce|test` — the label names a stored OAuth token; the value never enters the run dir.
- Run dir: `evals/railroad-review/<date>-<hash6>[-<n>]`, hash = first six digits of the checkout commit (`range.headSha`), `-2`, `-3`, … when the dir exists.
- Scoring: severity minutes, escape multipliers, triage line, fix factors, thresholds and the judge guide live in `golden/scoring.json`.

## Stages

Each stage is one `go run ./cmd/skilleval` subcommand the eval skill runs in this order.

- `init` — takes the golden kind (`-golden produce|test`, default `test`) and the run's `-set` for the args `invocation.open` names. Loads the definition, checks prerequisites, the kind's file (the seed for produce, the minted golden for test), the token label and the window (runs × `armUsd` ≤ `windowUsd`, env `SKILLEVAL_WINDOW_BUDGET_USD` overrides). Composes the prompt from the args, copies SKILL.md, the overlay and the kind's files (`seed.json`, or `golden.json` and `scoring.json`) into the run dir and writes `run.json` with its digest `run.json.sha256`. Every later stage reads only the run dir and refuses a changed `run.json`. `compose` with the same flags prints the prompt and writes nothing.
- `provision` — one arm worktree per label at `range.headSha` in the registered smine clone, overlay applied, then the prebuild command run in it (log `<arm worktree>-out/prebuild.log`, 15-minute deadline); a failed command or a missing output fails the run.
- `arm` — on a produce run copies the run's `seed.json` to `<arm worktree>-out/inputs/seed.json`, then runs one headless `claude -p` cell per label with `{runId}`, `{overlayPatch}`, `{artifactsDir}` and `{seed}` filled; `SKILL_CONTEXT_INDEX` exported as `<arm worktree>/<contextIndex>`. Afterwards the artifact dir moves to `runs/<label>/artifacts`, `candidates.json` is extracted from the newest file matching `findings.glob`, and the arm's whole change is captured as a diff.
- judges — per label an adherence judge, and on a test run a golden judge, each a subagent fed only the brief `skilleval brief adherence|golden` prints. A produce run has no golden judge; `brief golden` refuses it.
- `cleanup` — salvages the arm worktree, its worker worktrees and its workflow worktrees into `<arm worktree>-out/salvage/`, removes them unforced, deletes the run branches, then removes `<arm worktree>-out`; a worktree git refuses to remove is reported `removed: false` and keeps the out dir.
- `fold` — writes `eval.json` (validated against its schema first) and `report.md`. Wall and active seconds come from peek for the arm session; output tokens, cost and permission denials from the arm's result envelopes, all agents.

## Environment

- Range `b4981b8..1f725ce`.
  - Base label `1.1.4`: smine release 1.1.4 (`b4981b8`).
  - Head label `1.2.1 + golden-input plans`: release 1.2.1 plus the plan docs committed as spec input on branch `evaluation/railroad-review-ab-test` (`1f725ce`). The definition pins the SHA, never the branch.
  - Labels are display-only (the skill page's Tests tab range line).
- `range.excludePaths` (`sessions/`, `evals/`, `proposals/`, `plans/`): the arm command gets them as `exclude:`, so they leave every diff the skill computes; the golden judge's diff carries the same `:(exclude)` pathspec.
- Spec: the command's `spec:` names the 14 design and concept docs of the 9 golden-input plan dirs under `plans/v1-2-1/` (runbooks and images left out). Each round copies them to `round-{r}/spec-<n>-<basename>`, and the checklist stage builds one correctness checklist over all of them, grouped per spec. `plans/` stays out of the reviewed diff — the docs are spec, not change.
- Overlay v2 (`golden/smine-1.1.4-1.2.1.g2.overlay.patch`): `AGENTS.md` and the context docs of the 2026-09-16 pack, the index at `context/review-index.json` — a name no gate or test of the reviewed tree knows. No gate file: the tree runs its own gates as committed. The arm hands the overlay on as `overlay-patch:`, so every worker worktree gets it too.
- Prebuild: `make build-audit` once per arm worktree; outputs `bin/acdsl` and `bin/verifiers` go to the skill as `prebuilt:` and are symlinked into every worker worktree — no worker builds a gate binary.
- Health, measured 2026-10-04: base green without the overlay; head green with overlay v2 (audit 24 rules OK, `make test` green). The skill checks the base without the overlay and each round's head with it.
- Arm: dispatcher `opus` at `high` (definition defaults), lanes `2[opus:medium]`, station `opus:medium`, $1000 and 4 hours per arm, $1000 window.

## Production flow of the g2 line

- One seeded run, one round in one arm (`rounds: 1`; `set: rounds=2` for more), `autofix-level: none`.
- Seed: `golden/smine-1.1.4-1.2.1.g2.seed.json` (`goldens[0].produce.seedFile`) holds 352 findings — the 151 round-1 findings of run `2026-10-04-1f725c`, the 59 that run `2026-10-05-1f725c` added, the 54 of run `2026-10-05-1f725c-2`, the 51 of run `2026-10-05-1f725c-3` and the 37 of run `2026-10-05-1f725c-4`. On a produce run the runner hands its run-dir copy to the skill as `known: <path>`; the skill never re-reports those findings, keeps only its own in `known.json` and numbers them after the seed's highest number per direction. The rows' `round` field is history and read by nothing.
- Per round:
  - review to the station gate;
  - the ledger appends every new confirmed finding to `known.json` as `open`;
  - no auto-fix set, no fix workflow, no patch — the next round reviews the same head with the grown ledger as known.
- Result: the findings the run adds to `known.json`; no patches.
- Produce run: unjudged against a golden and unscored; the arm writes its candidates, the run's own findings.
- Candidates: `runs/<label>/candidates.json`, one per `known.json` row, id `r<round>-<finding id>`, carrying `route`, `fix_state` and `round`.
- Record: `runs/<label>/artifacts/` holds the list the skill keeps when it ends — `coverage.json`, `known.json`, `review-record.md`, and per round `checklist-*.json`, `chunks.json`, `known.json`, `review.json`, `risk-map.json` plus the lane reports and `round-*/verdicts/`, since the definition's args pass `verbose`.
- Minting:
  - adjudicate the candidates by hand;
  - write `golden/smine-1.1.4-1.2.1.g2.v1.master.json` by hand: `G2-` ids, `goldenVersion: 1`, each finding's `source` as `run-<dir>-<candidate id>` and its `contextOnly` classification (generation refuses an unclassified finding);
  - derive with `go run ./cmd/skilleval goldens gen <master>` — writes the full, contextless and known siblings; `make golden-check` fails on drift;
  - `skilleval promote <run dir>` appends the run's own findings to the seed file once the run was read; it refuses when the seed changed since the run started. Once `goldens[0].file` is minted, test runs are judged against it. The runner reads no other golden file.

## Reading results

- **review_score** — `1 − (run − oracle) / (null − oracle)`, in minutes from `scoring.json`. Oracle: every golden entry found at its severity and fixed by a human. Null: no review — every golden entry missed and merged (minutes × escape multiplier). 1.0 = a perfect human-fixed review, 0 = no better than no review, below 0 = cost more than it saved, above 1.0 = confirmed machine fixes saved human minutes. Comparable only within one golden.
- **Run minutes** per golden entry, m = minutes of its true severity, k = its escape multiplier, factor = `fixFactor` of its severity:
  - missed, or the judge finds the golden defect still present after a `fixed` candidate (`fixEscapes`) — m × k;
  - `fixed` — m × factor, reviewing the fix;
  - `applied` — m × factor + m, that review plus a human redo;
  - `open`, truly at or above `triageLine` but reported below it — m × k, nobody looks and it merges;
  - `open` otherwise — m + |m(reported severity) − m|.
- **False positives** (duplicates of a matched defect included) — minutes of the claimed severity, plus that × the claimed severity's factor when the candidate is `applied` or `fixed`. Uncataloged: 0.
- **lost_minutes** — run − oracle; **escaped_minutes** — the escape bucket; **false_positive_minutes** — the false-positive bucket.
- **recall**, **false_positives**, **misclassified** — diagnostic counts, never part of the score. Genuine in-range defects the golden lacks are uncataloged and never count against the run; they are candidates for the next golden version.
- **Verdict** — per arm, pass when valid and inside every threshold (`review_score` ≥ 0.5, `false_positive_minutes` ≤ 60); run status `pass`/`mixed`/`fail` = all/some/no arms pass. An arm whose cell ran another model than requested is invalid and its golden metrics are null. A golden with no savable minutes nulls `review_score`; only the false-positive bound judges then.

## v1 line (archive)

The v1 line (golden v1–v9) is complete: never extended, never seeded from, never judged against in a g2 run. It reviewed head `a342811b` under the v1 overlay. The v9 files and `golden/smine-1.1.4-1.2.1.overlay.patch` stay for post-hoc comparison once g2 versions exist. `golden/smine-1.1.4-1.2.1.v9.reclass.json` is the hand reclassification of every v9 BLOCKER, MAJOR and MINOR against the g2 seed (2026-10-06): verdict, matching g2 ids, regraded severity, operator rulings — a comparison record; nothing from v9 is ever carried into a g2 golden. The mechanisms the history below names — context freeze, validity gates, the efficiency ladder — no longer exist in the runner; seeding returned in the g2 line as `goldens[0].produce.seedFile`.

### Golden provenance

**v8–v9** — the line ends at v9 (2026-09-13, 347 findings, 26 false positives): all 324 v8 findings carried, one claim amendment, 23 promotions (`GV9-325..GV9-347`) from the v8-seeded two-round run `2026-09-12-a34281`. Details in the v9 master's `provenance` block.

**v7 (294 findings, per-environment)** — created 2026-09-09 by carry-plus-promotion-plus-recheck: all 252 v6 findings carried byte-verbatim plus 42 promotions (`GV7-253..GV7-294`). All 63 post-v6 uncataloged claim units — 14 from `railroad-review/2026-09-08-a34281` (sonnet-5-medium), 45 pre-grouped from the 32-arm `superpowers-review/2026-09-08-a34281-2` (carrying that run's r2 double-verification verdicts as priors), 3 from `superpowers-review/2026-09-08-a34281` and 1 from `aqms-review-wip/2026-09-09-a34281` — were re-verified by 7 independent sonnet-5 verifier batches (≤10 units each, read-only, against the pinned review head a342811b + overlay): 45 confirmed (42 after cross-run dedup — the acdsl-policy-guard registry.local.json denial, the skill-invoke-gate prefix match and the install.sh port-kill pattern were each found by two runs), 10 refuted, 6 duplicate-golden, 2 out-of-range. Two prior sp-run confirmations were overturned with cited plan-decision evidence. The false-positive ledger grew 16 → 26 with the 10 recheck-refuted claims (sonnet-adjudicated, source-tagged). Each promoted entry was classified contextOnly at promotion time (6 of 42 flagged; contextless sibling holds 236). Recheck artifacts: `plans/2026-09-09-golden-v7-integration-wip-34debf1/verification/` in claude-configs.

**v6 (252 findings, per-environment)** — created 2026-09-07 by carry-plus-promotion-plus-context-classification: all 245 v5 findings carried byte-verbatim (modulo the added classification keys in the master) plus 7 promotions — `GV6-246..GV6-251` from the opus-4-8 judge of run `2026-09-07-a34281` (opus-4-8[1m]-high, the session-limit-interrupted arm completed by a Desktop dispatcher) and `GV6-252` (SP-003, the implicit CWD==repo-root path contract in saveProfileSelection/installenv) from the superpowers-review arm of the same date, uncataloged under both its judge passes. False positives carried unchanged at 16 (the railroad run's 4 new judge-ruled FPs are not human-adjudicated and were not added). All appended entries are judge-ruled, not yet human-re-adjudicated. First version with the contextOnly classification and generated per-environment files; the contextless sibling holds 200 findings.

**Context classification** — one-time pass 2026-09-07: 3 opus classifier cells over 84 findings each plus a mechanical rule-id pre-pass (advisory), criterion "contextOnly iff the defect is only identifiable via the ACDSL context pack; internal-inconsistency-visible-in-diff and generically-lintable claims stay in", human-skimmed and approved; 52 of 252 flagged (41 NIT, 7 MINOR, 4 INFO — no MAJOR/BLOCKER excluded).

**v5 (245 findings)** — created 2026-09-07 by carry-plus-promotion: all 218 v4 findings carried byte-verbatim plus 27 uncataloged candidates (`GV5-219..GV5-245`) promoted from the fable-5 judge of run `2026-09-06-a34281-182536` (sonnet-5[1m]-high seeded run, seed v4.known); the false-positive ledger carried unchanged at 16 (the run's 2 new judge-ruled FPs are not yet human-adjudicated and were not added). All appended entries carry `source: "fable-5 judge of sonnet-5-high-r1 2026-09-06-a34281-182536 (uncataloged, promoted)"` — judge-ruled, not yet human-re-adjudicated.

**v4 (218 findings)** — created 2026-09-07 by carry-plus-promotion: all 201 v3 findings carried byte-verbatim plus 17 uncataloged candidates (`GV4-202..GV4-218`) promoted from the fable-5 judge of run `2026-09-06-a34281-153853` (sonnet-5[1m]-high saturation run, judged from round-1 after the arm died unjudged); the false-positive ledger grew 11 → 16 with that judge's rulings. All appended entries carry `source: "fable-5 judge of sonnet-5-high-r1 2026-09-06"` — judge-ruled, not yet human-re-adjudicated.

**v3 (201 findings)** — created 2026-09-06 by carry-plus-promotion: all 168 v2 findings carried unchanged (source `GV2-*`) plus 33 uncataloged candidates promoted from run `2026-09-06-a34281` (sonnet-5[1m]-high, scored vs v2); the false-positive ledger grew 5 → 11 with that run's human-ruled rejections.

**v2 (168 findings)** — created 2026-09-05 by zero-state re-adjudication: all 136 v1 findings plus the 38 uncataloged candidates from run `2026-09-05-a34281-094349` were each verified against the exact tree eval arms review (`experiment/railroad-review-ab-test-with-context` @ `2a24fcb`) by 18 read-only fable-5-medium cells; the 6-drop / 38-promote lists were human-approved. Every v2 finding carries a `source` field (`v1-<id>` or `run-…`) and a zero-state-valid anchor.

**v1 (136 findings)** — distilled from the interactive review session "[Smine, RR, F.m→O.m] v1.2.1 Review" (2026-09-01→03): Desktop workflow mode, orchestrator claude-fable-5, lanes opus-4-8-medium, bypassPermissions, **no skill context injected (accidental)** — lanes ran from SKILL.md alone, which is part of why strong models still produced many nits. Rounds 1–6 ran with fixes applied between rounds, so later-round findings (`postFix`) describe a partially-fixed tree — the reason v1 recall was structurally capped and v2 exists.

Golden policy of the line: versions bumped only with a human-approved drop and promotion list; recall is never comparable across golden versions.

### Run history (scored vs golden v1 unless noted)

| Run | Setup | Outcome |
|---|---|---|
| 2026-08-27-523b69 / -5607bf | observability routine, per-lane cells | mechanical metrics only, no golden scoring |
| 2026-09-02-935dd9 | opus-4-8-high vs sonnet-5-high, r1, no context | **fail** — recall ~0, FPs up to 12: budget-driven self-triage without refutation |
| 2026-09-04-a34281-080018 | opus/sonnet ×3, no context | **fail** — recalls 0.02–0.04, several dead cells |
| 2026-09-04-a34281 | opus/sonnet r1–r5, no context | **fail** — most cells died (session-limit wall); best sonnet recall 0.10 |
| 2026-09-05-a34281 | sonnet-5-high r1 | **fail** — cell died (prompt too long) |
| 2026-09-05-a34281-094349 | sonnet-5[1m]-high, **with context** | **INVALID** (rounds-incomplete: 1M-context overflow at round-2 station, since fixed) — informational recall 0.40 / weighted 0.46, FP 6, uncataloged 38 (promoted into v2) |
| 2026-09-06-a34281 | sonnet-5[1m]-high, with context, **vs v2** | **fail** — VALID, recall 0.21, FP 1, rounds 1, uncataloged 38 (33 promoted into v3, 6 rulings into v3's FP ledger) |
| 2026-09-06-a34281-153853 | sonnet-5[1m]-high, saturation (seeded v3), **vs v3** | **fail** — INVALID (no-summary: dispatcher died to the pause/steer deadlock, since fixed); judged post-hoc from round-1 by a fable-5 judge (`judged_from: round-1`): matched 3, FP 5, uncataloged 17 (promoted into v4) |
| 2026-09-06-a34281-182536 | sonnet-5[1m]-high, saturation (seeded v4.known), **vs v4** | **fail** — VALID (dispatcher completed; judge stage re-run as fable salvage judge from summary); matched 6, FP 6, misclassified 3, uncataloged 27 (promoted into v5) |
| 2026-09-07-a34281 | opus-4-8[1m]-high, with context, **vs v5** | **fail** — VALID (headless arm hit the account session limit mid-verification; verify b3/b4/b5/b8, station and judge completed by a Desktop dispatcher on opus); missed 87% (213 of 245), weighted recall 0.13, precision 0.89, FP 4, misclassified 8, uncataloged 6 (promoted into v6) |
| 2026-09-07-a34281-175351 | sonnet-5[1m]-medium, with context, **vs v6** | **fail** — uncataloged 0 (nothing to promote) |
| 2026-09-08-a34281 | sonnet-5[1m]-medium (Desktop workflow route), with context, **vs v6** | **fail** — uncataloged 14 (all 14 recheck-confirmed, promoted into v7) |

Interpretation: the first runs were mechanical failures; the single with-context run was dramatically more productive despite dying early. That contrast motivated pinning the review context in an overlay.
