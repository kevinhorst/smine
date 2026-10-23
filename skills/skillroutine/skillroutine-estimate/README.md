# Skillroutine Estimate

## Design notes

- **SKILL-SKILLROUTINEESTIMATE-PEEK-001a**: the plan's own decisions (lane model, batching, parallelism) override every
  analytic assumption — the first real run's estimate tightened from a 3.5× band to the landing zone only after the plan
  was peeked.
- **SKILL-SKILLROUTINEESTIMATE-PEEK-002**: in the first measured run the cost counter dropped from $17.18 to $12.99
  mid-run while token counters stayed monotonic; token arithmetic (≈$70) matched the user's meter delta, the counter did
  not.
- **SKILL-SKILLROUTINEESTIMATE-STATE-001**: skill bodies, rosters, and formulas differ between branches — the first
  measured run's roster was 44 on the estimator's branch and 35 on the executing branch.
- **SKILL-SKILLROUTINEESTIMATE-CELLS-002**: the first measured run accrued 13 agents on a "no fan-out" target, and
  parallel lanes cut a 2.5–5 h sequential estimate to 55 min actual.
- **SKILL-SKILLROUTINEESTIMATE-TIME-001**: the first measured run landed inside the cost envelope but missed wall-clock
  by 3–5× (estimate 2.5–5 h, actual 55 min) — sequential-batch assumptions plus no effort model.
- **SKILL-SKILLROUTINEESTIMATE-STRAT-001a**: on high-token skills the strategy is the largest lever — the measured run's
  sonnet lanes and fable-low switch moved cost and time more than any other variable.
