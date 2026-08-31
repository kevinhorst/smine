# Lane critical-1 — findings

Range: a7abd04..5607bf4 · direction: critical (high-risk defect classes: concurrency misuse, nil derefs, weakened guards, resource leaks) · context read: context/actions/concepting.md + concepting-local.md (ACTION-CONCEPT-HOT-*).

Files reviewed (all 3 in the diff, in full):
- plans/railroad-review-observability/runbooks/trace.md
- routines/skill-eval-railroad-review/com.smine.routine.skill-eval-railroad-review.plist.template
- routines/skill-eval-railroad-review/run.sh

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| critical-MINOR-1 | MINOR | run.sh:188 | Cell git worktrees leak into the main repo's worktree registry. run_cell removes a cell only at its own tail (line 188) or via a same-id guard at the next add (line 153); there is no EXIT trap, no bulk `$RR_CELLS/*` sweep, and no `git worktree prune` (grep confirms). Sequence: a 6-candidate run creates refute-1..6; run.sh is hard-killed (launchd unload, sleep, OOM, SIGKILL) after `git worktree add .../refute-6` and before line 188; the next run has ≤5 candidates, never re-adds refute-6, and its registered worktree (path + `.git/worktrees/refute-6`) persists forever. Nothing ever prunes, so orphans accumulate. The matrix.sh pattern it copies does the missing sweep (`routine_matrix_cleanup`, matrix.sh:311-319, ends in `git worktree prune`). | Add an EXIT trap that force-removes every `$RR_CELLS/*/` worktree and runs `git -C "$repo_root" worktree prune` (mirror `routine_matrix_cleanup`); at minimum prune at startup. |

Notes on the other two files: trace.md is a jq/bash runbook (documentation) — no high-risk defect class. The plist template is static launchd config — no defect class. bash 3.2 compatibility (ACDSL-SHELL-002, projected on run.sh) holds: all empty-array expansions (`lane_jsons`) are guarded behind `[[ ${#...[@]} -gt 0 ]]`, no `declare -A`/`mapfile`/parameter transformation, `$'\n'`/process-substitution/here-strings are all 3.2-supported. Cells run sequentially (no real concurrency), so no race/lock defect. Empty-envelope paths use `input? // {}` and `${var:-}` guards throughout — no unbound-var or nil-equivalent deref.

Funnel: 1 produced / 1 survived confirmation pass / 0 dropped.
