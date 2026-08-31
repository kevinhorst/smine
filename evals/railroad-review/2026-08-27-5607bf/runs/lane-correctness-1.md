# Lane correctness-1 — findings

Range: a7abd04..5607bf4 · direction: correctness · context: ACTION-REVIEW-SPEC-001..005 + plan `plans/railroad-review-observability/design/raw.md`

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| correctness-MAJOR-1 | MAJOR | run.sh:167 (+ plist template:16) | Rewrite drops the routine's only aggregate budget cap: old code capped the session with `--max-budget-usd ${ROUTINE_MAX_BUDGET_USD:-15}`; new code caps each cell at `$cell_budget` (ROUTINE_CELL_BUDGET_USD, default 4) with no total ceiling and never reads ROUTINE_MAX_BUDGET_USD. The config server still renders the standard "Max budget (USD)" widget (routines.go:41) for this routine, so an operator setting it gets a silent no-op while worst-case spend is ~13 cells × $4 ≈ $52/run, unbounded. | Restore a total-cost ceiling that reads ROUTINE_MAX_BUDGET_USD, or add ROUTINE_CELL_BUDGET_USD/REFUTE_* to routines.go standardSettings so the new knob is exposed and the dead widget isn't the only visible budget control. |
| correctness-MINOR-1 | MINOR | run.sh:224 | Refute-candidate cap truncates by filename, not severity: `sort_by(-rank) | unique_by([.file,.line]) | .[0:$max]` — jq's unique_by re-sorts by [.file,.line], discarding the rank sort, so with the cap binding a BLOCKER in a late-sorting file (e.g. z.go) is dropped while lower-severity findings in a..f get refuters. The flatten also spans all directions, collapsing same-line findings from different directions. (Station still unions all lane findings, so impact is limited to refuter selection.) | Re-apply the severity sort after unique_by; add direction to the dedup key. |
| correctness-MINOR-2 | MINOR | run.sh:245 | Refuter verdicts are keyed only by finding_id, which is not unique: lanes number `$d-<SEV>-<n>` from 1 independently, so two lanes of one direction both emit `correctness-MAJOR-1` for different file:line. The refute JSON carries no file/line, so the station (which applies verdicts by finding_id) can attach a debunk to the wrong claim and drop a real finding. | Echo file+line in the refuter output and join on {finding_id,file,line}, or assign each deduped candidate a fresh unique id passed to both refuter and station. |

Confirmation pass: all three claims re-read in full context and survived. Cross-boundary claims (MAJOR-1) verified against internal/server/routines.go, internal/server/env.go, internal/routines/plist.go; MINOR-1 verified against live jq behavior.

Funnel: 3 produced / 3 confirmed by lane self-check / 0 dropped.
