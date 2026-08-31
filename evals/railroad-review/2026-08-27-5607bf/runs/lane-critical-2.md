# Lane critical-2 — findings

Direction: critical (high-risk defect classes: concurrency misuse, nil derefs, weakened guards, resource leaks). Range a7abd04..HEAD. Aborted: no.

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| critical-MAJOR-1 | MAJOR | run.sh:154 | Resource/registration leak: run_cell never prunes git worktrees. If the routine is killed after `git worktree add` (154) registers the cell but before removal (188), and the cell dir under `~/.cache/.../rr-eval-cells` (a purge-prone location) is later deleted while `.git/worktrees/<id>` survives, line 153's dir-existence guard is false, so the next `git worktree add` fails `already exists` and that cell id fails on every run until a manual `git worktree prune`. The cited `matrix.sh` pattern (header line 9) prunes in cleanup (matrix.sh:319); run.sh dropped it. | Add `git -C "$repo_root" worktree prune \|\| true` at the top of run_cell, or guard removal by registration instead of dir existence. |
| critical-MINOR-1 | MINOR | run.sh:224 | Severity-blind refute cap: `sort_by(-sev) \| unique_by([.file,.line]) \| .[0:$max]` — jq's `unique_by` re-sorts by [file,line], discarding the severity sort, so the `.[0:6]` cap selects candidates by lexical file:line order. With >6 at-threshold findings a BLOCKER in a lexically-late path can be dropped from refutation while MAJORs are refuted. Bounded: the station re-verifies all surviving claims, so no finding is lost — only refute prioritisation is wrong. | Re-sort by the severity map after `unique_by`, before the `.[0:$max]` slice. |

Funnel: 2 claims produced / 2 confirmed (self-confirmation pass) / 0 unverified / 0 duplicates / 0 rejected / 0 debunked.
