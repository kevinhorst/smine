# Railroad Review — critical direction — lane 1

Range: `72f5057..523b69` · Direction context: `ACTION-CONCEPT-HOT-*` (fallback: concurrency misuse, nil derefs, weakened guards, resource leaks)

## Findings

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| — | — | — | No critical defect found. | — |

## Scope of the critical surface

Of the 87 changed files, the executable surface is narrow — the rest are markdown/JSON session reports, eval artifacts, proposal stores, and changelogs with no high-risk runtime path.

Code files reviewed in full:

- **cmd/worktrees/_lib/verdict.sh** — the diff adds only a two-line `[ACDSL-PROJECTION]` comment header; the bash logic (candidate discovery, cherry sets, probe worktree lifecycle, twin sweep) is byte-for-byte unchanged. No shipping defect introduced. The lazy probe worktree is trap-cleaned (`verdict_cleanup` on EXIT), the set-`-u`-safe array iteration idiom `${arr[@]+"${arr[@]}"}` preserves space-containing elements, so no leak or mis-compare is introduced by this change.
- **skills/smine/smine/workflows/session-mine.js** — new Drift stage and a `tier` model/effort override. The drift result is null-guarded (`|| { … }`) and read defensively (`drift.diverging_ids && …`). `tier = { ...(model && { model }), ...(effort && { effort }) }` is safe: each of `model`/`effort` is either `''` (→ `{...''}` = `{}`) or a non-empty string (→ the object `{model}`/`{effort}`), never spreading string characters. The pre-existing route/`failed` alignment was not touched by this diff.
- **skills/feature/fdesign/workflows/subsystem-grounding.js** (new) — parallel Explore fan-out with a `reports.map((r,i) => r || fallback)` null guard before every field read; `parallel()` is the barrier, no shared mutable state across agents. No leak, nil deref, or weakened guard.
- **skills/feature/fimplement/workflows/config-ui-fidelity-gate.js** (new) — two-stage pipeline; the render-failed branch returns a fully-populated fallback object and the gate results are `map((r,i) => r || fallback)` guarded, so every object reaching the `flatMap(r => r.violations…)` carries `violations`. No defect.
- **skills/git/package-commit/workflows/foreign-toolchain-pretag.js** (new) — artifacts filtered to well-formed objects at intake; `results.map((r,i) => r || fallback)` indexes `artifacts[i]` safely (pipeline preserves length/order); failed builds default to `exit_code: 1` (fail-closed). No weakened guard.

## Confirmation pass

Re-read every cited hunk in full context: verdict.sh diff (comment-only, confirmed via `git diff`), session-mine.js `tier`/drift additions (lines 20-31, 92-106), and the three new workflow files end to end. No candidate survived to a falsifiable critical claim — every result-consumption site is null-guarded and no concurrency/resource-lifecycle path is introduced.

Funnel: produced 0 / confirmed 0 / unverified 0 / duplicates 0 / rejected 0 / debunked 0
