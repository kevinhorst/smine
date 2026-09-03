# Context-entry eviction screening

Produces a tombstone candidate list from traced real railroad-review runs — purely
observational (the causal ablation harness is deferred; see the plan). A candidate
here is input to a manual tombstone decision, never an automatic removal.

## Rule

As of railroad-review 8.4 lanes receive the full doctrine **injected** and are gate-blocked
from reading context-entry files directly, so the old "never read" signal is dead — every
entry is injected into every lane, read or not. Screening now rests on citation alone.

An entry is a candidate when BOTH hold across at least 3 traced runs (trace.md):

1. **Never cited** — the entry id appears in no finding of any `round-*/review.json`
   and no lane artifact.
2. **Not mechanically enforced** — `enforcement` is neither `gate` nor `lint`
   (those entries are gate-owned regardless of prompt-side use).

Anything failing a condition is "keep"; an entry whose citations vary run-to-run is
"inconclusive — keep". Citation is a weaker signal than the retired read+cite pair — an
entry that is genuinely load-bearing but rarely the subject of a finding will look like a
candidate, so treat the list as a prompt for review, not evidence of deadweight; the causal
ablation harness (deferred, see the plan) is what would settle a contested candidate.

## Recipes

Citation scan across a round's artifacts (run per traced round dir):

```bash
grep -rhoE "(ACTION|RULE|FACT)-[A-Z-]+-[0-9]{3}" <roundDir>/ | sort | uniq -c | sort -rn
```

Entries a direction covers but that no finding cited (feed the citation scan above as cited.txt):

```bash
jq -r --arg glob "ACTION-IMPL-INTEG-" '.entries[] | select(.id | startswith($glob)) | [.id, .source, .enforcement] | @tsv' context/context.json | while IFS=$'\t' read -r id src enf; do
  if [ "$enf" != "gate" ] && [ "$enf" != "lint" ] && ! grep -qx "$id" cited.txt; then echo "$id	$src	uncited"; fi
done
```

## Output

Save the candidate table as `evals/railroad-review-<date>/eviction-candidates.md`:

| Entry | Direction(s) | Runs observed | Cited? | Enforcement | Evidence |
|---|---|---|---|---|---|

Each row links the trace outputs and review.json paths it was derived from. Kevin
tombstones accepted candidates in the context source (with replacement pointers where
applicable) — the existing lifecycle mechanism; nothing is written onto entries.
