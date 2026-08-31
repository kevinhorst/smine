# Lane code-style-1 — findings

Range: a7abd0401d2beadde42bb63ba4d395606c94f2dd..HEAD
Direction: code-style (context: ACTION-REVIEW-QUALITY-*, ACDSL-SHELL-002 bash 3.2 rule projected onto run.sh, RULE-RUNBOOK-* for trace.md)

Files reviewed (3/3, full read):
- routines/skill-eval-railroad-review/run.sh
- routines/skill-eval-railroad-review/com.smine.routine.skill-eval-railroad-review.plist.template
- plans/railroad-review-observability/runbooks/trace.md

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| code-style-MINOR-1 | MINOR | run.sh:2 | Committed `[ACDSL-PROJECTION]` header whose own text says "stripped before commit"; present in HEAD, and no sibling `routines/*/run.sh` carries one (they start shebang + plain comment). Does not read like the repo wrote it. | Strip the two header lines (2-3) from the committed file. |
| code-style-NIT-1 | NIT | run.sh:294 | `done < <(cat "$cells_file")` — useless use of cat; sibling `_lib/matrix.sh:226` uses the direct `done < "$plan"` idiom. | `done < "$cells_file"`. |
| code-style-NIT-2 | NIT | run.sh:227 | `total_major` counts candidates at the configured `$refute_rank` threshold (blocker\|major\|minor\|nit\|info), not "major"; name and the "(of $total_major pre-dedup)" message mislead when ROUTINE_REFUTE_LEVEL != major. | Rename to a threshold-generic name (e.g. `total_at_threshold`) at both assignments and the echo. |

Notes:
- Bash 3.2 (ACDSL-SHELL-002): no `declare -A`, `mapfile`/`readarray`, case-conversion/parameter-transformation expansions, or `|&` introduced; empty-array uses are guarded by `${#arr[@]} -gt 0`. Clean.
- plist template: env-var rename (`ROUTINE_MAX_BUDGET_USD`→`ROUTINE_CELL_BUDGET_USD`) and added `ROUTINE_REFUTE_LEVEL`/`ROUTINE_REFUTE_MAX` are consistent with run.sh. Clean.
- trace.md: the added "Routine runs" section is a doc/jq-recipe runbook (not a Bruno smoke-test collection), so RULE-RUNBOOK-001..008 do not apply; prose and code fences match the surrounding file style. Clean.

Funnel: 3 produced / 3 survived confirmation pass / 0 dropped.
