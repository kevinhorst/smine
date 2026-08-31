# Station review — a7abd04..5607bf4 — code-style, correctness, critical

Reviewed tree = this worktree (HEAD 5607bf4). Changed files in range: `routines/skill-eval-railroad-review/run.sh`, `routines/skill-eval-railroad-review/com.smine.routine.skill-eval-railroad-review.plist.template`, `plans/railroad-review-observability/runbooks/trace.md`.

## Findings

| ID | Severity | Location | Finding | Proposed fix | Verdict |
|----|----------|----------|---------|--------------|---------|
| correctness-MAJOR-1 | MAJOR | run.sh:167 (+ plist 16-17) | Aggregate budget ceiling dropped; each cell capped independently at ROUTINE_CELL_BUDGET_USD (default $4), no total cap (~$52 worst-case, 13 cells). Config server still renders 'Max budget (USD)'→ROUTINE_MAX_BUDGET_USD (routines.go:41) which run.sh never reads — the only UI budget lever for this routine is dead. | Read ROUTINE_MAX_BUDGET_USD as a total ceiling in run.sh, or add ROUTINE_CELL_BUDGET_USD to routines.go standardSettings. | confirmed (station) |
| code-style-MINOR-1 | MINOR | run.sh:2-3 | The `[ACDSL-PROJECTION]` header is committed into HEAD despite its own text 'stripped before commit'; no sibling routine run.sh carries it. | Strip run.sh lines 2-3 from the committed file. | confirmed (station) |
| correctness-MINOR-1 | MINOR | run.sh:224 | Refute-candidate cap is severity-blind: `unique_by([.file,.line])` re-sorts, discarding `sort_by(-rank)`, so `.[0:$max]` truncates in lexical file order and can drop a BLOCKER. Flatten also spans all directions, collapsing cross-direction findings at the same line. Bounded to refuter coverage (station reads all lane JSONs). | Re-sort by severity after dedup; key dedup on `[.file,.line,.direction]`. | confirmed (station) |
| correctness-MINOR-2 | MINOR | run.sh:245 | Refuter verdicts keyed only by finding_id, but ids are numbered per-lane so they collide across lanes; the refuter JSON carries no file/line, so the station can attach a verdict to the wrong claim. | Refuter echoes file+line and station joins on `{finding_id,file,line}`, or assign fresh unique ids per candidate. | confirmed (station) |
| code-style-NIT-1 | NIT | run.sh:294 | `done < <(cat "$cells_file")` — useless use of cat; sibling matrix.sh uses `done < "$file"`. | `done < "$cells_file"`. | confirmed (station) |
| code-style-NIT-2 | NIT | run.sh:227 | `total_major` counts candidates at the configured `$refute_rank` (any level), not 'major'; name + message mislead when ROUTINE_REFUTE_LEVEL≠major. | Rename to `total_at_threshold` at both assignments and the echo. | confirmed (station) |

## Debunked (audit list — not in the review body)

| ID | Severity | Location | Claim | Why debunked |
|----|----------|----------|-------|--------------|
| critical-MAJOR-1 (merges critical-MINOR-1, critical lane 1) | MAJOR | run.sh:154 | Cell worktree registrations leak and permanently brick a cell id until a human runs `git worktree prune`. | run.sh:87/94 source and call `routine_worktree_create` before the lane loop; worktree.sh:133 runs a bare `git worktree prune` (expire=TIME_MAX) unconditionally every run, reclaiming any registration whose directory is missing. Both lanes grepped run.sh alone and missed the sourced worktree.sh. Refuter debunk re-verified against the tree. |

Funnel: claims produced 10 / after dedup 7 / debunked 1 / survivors 6
