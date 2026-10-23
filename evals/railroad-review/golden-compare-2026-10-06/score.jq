$s[0] as $S
| {BLOCKER: 4, MAJOR: 3, MINOR: 2, NIT: 1, INFO: 0} as $rank
| $rank[$S.triageLine] as $triage
| ([$outs[].pairs[]] | map(select($mode == "all" or .confidence == "high")) | unique_by([.old, .new])) as $pairs
| ($v9[0].findings | map({id: .golden_id, severity})) as $old
| ($g2[0] | map({id: .finding_id, severity})) as $new
| def sevOf(list): list | map({(.id): .severity}) | add;
  def score(golden; run; gk; rk):
    sevOf(run) as $rs
    | ($pairs | group_by(.[gk]) | map({(.[0][gk]): map(.[rk])}) | add) as $m
    | (golden | map(. as $g
        | $S.minutes[$g.severity] as $min
        | $S.escapeMultiplier[$g.severity] as $k
        | ($m[$g.id] // []) as $hits
        | if ($hits | length) == 0 then {sev: $g.severity, state: "missed", cost: ($min * $k)}
          else ($hits | map($rs[.]) | max_by($rank[.])) as $sr
            | if $rank[$g.severity] >= $triage and $rank[$sr] < $triage
              then {sev: $g.severity, state: "triage-escape", reported: $sr, cost: ($min * $k)}
              else {sev: $g.severity, state: (if $sr == $g.severity then "matched" else "misclassified" end), reported: $sr,
                    cost: ($min + (($S.minutes[$sr] - $min) | fabs))}
              end
          end)) as $rows
    | (golden | map($S.minutes[.severity]) | add) as $oracle
    | (golden | map($S.minutes[.severity] * $S.escapeMultiplier[.severity]) | add) as $null
    | ($rows | map(.cost) | add) as $runMin
    | ($pairs | map(.[rk]) | unique | length) as $runMatched
    | {
        golden: (golden | length),
        found: ($rows | map(select(.state != "missed")) | length),
        missed: ($rows | map(select(.state == "missed")) | length),
        recall: (($rows | map(select(.state != "missed")) | length) / (golden | length) * 100 | round / 100),
        misclassified: ($rows | map(select(.state == "misclassified")) | length),
        triageEscapes: ($rows | map(select(.state == "triage-escape")) | length),
        bySeverity: ($rows | group_by(.sev) | map({(.[0].sev): {total: length, found: (map(select(.state != "missed")) | length), triageEscape: (map(select(.state == "triage-escape")) | length)}}) | add),
        oracleMinutes: $oracle, nullMinutes: $null, runMinutes: $runMin,
        lostMinutes: ($runMin - $oracle),
        reviewScore: ((1 - ($runMin - $oracle) / ($null - $oracle)) * 1000 | round / 1000),
        runEntries: (run | length), runEntriesMatched: $runMatched, runEntriesUncataloged: ((run | length) - $runMatched)
      };
  {
    mode: $mode,
    pairs: ($pairs | length),
    newAsRunVsOldGolden: score($old; $new; "old"; "new"),
    oldAsRunVsNewGolden: score($new; $old; "new"; "old")
  }
