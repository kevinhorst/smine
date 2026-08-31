# Lane correctness-2 — findings

Range: a7abd04..HEAD (5607bf4). Direction: correctness ("does exactly what the spec requires — nothing missing, nothing nobody asked for").

Files reviewed (full read):
- plans/railroad-review-observability/runbooks/trace.md
- routines/skill-eval-railroad-review/com.smine.routine.skill-eval-railroad-review.plist.template
- routines/skill-eval-railroad-review/run.sh

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| correctness-MINOR-1 | MINOR | run.sh:220 | Refute-candidate cap does not keep the highest-severity candidates. `sort_by(-severity) \| unique_by([.file,.line]) \| .[0:$max]` — jq's `unique_by` re-sorts ascending by `[.file,.line]`, undoing the severity sort, so `.[0:$max]` truncates in file/line order. With `refute_max=6` and 7+ distinct-location candidates where the lone BLOCKER sorts high by path (`zzz.md`) and six MAJORs sort low (`aaa.md`..`fff.md`), the BLOCKER is dropped from refutation and the MAJORs kept. Bounded impact: the station (run.sh:249-260) still ingests every lane finding via `eval-in/lanes/*.json`, so no finding is lost — only the second-confirmation pass is misallocated. | Re-sort by severity after dedup: `... \| unique_by([.file,.line]) \| sort_by(-(severity-rank)) \| .[0:$max]`. |

Funnel: claims produced 1 / confirmed 1 / unverified 0 / duplicates 0 / rejected 0 / debunked 0.
