## Design notes

- **SKILL-SKILLROUTINEEVAL-RULES-006**: the eval's design premise — a skill is evaluated as shipped; any "necessary"
  change to it invalidates the measurement and is escalated, never made.
- **SKILL-SKILLROUTINEEVAL-INTAKE-008**: the skill is a thin wrapper — `skilleval init` builds every run input from the
  skill's own `eval/definition.json`; a skill changes its eval by editing that file, never this skill. Comparing
  configurations is `/skillroutine-ab`.
- **SKILL-SKILLROUTINEEVAL-JUDGE-001**: the skill body is its own rubric — adherence is judged per entry against a
  trace recorded at run time, so no golden is needed to ask "did it follow every step". The brief text lives in
  `internal/skilleval/briefs/` beside the fold that parses the judge's output, shared with `/skillroutine-ab`.
- **SKILL-SKILLROUTINEEVAL-GOLD-003**: the golden family owns how it is matched — judge guide, severity minutes, escape
  multipliers, triage line, fix factors and thresholds live once in `eval/golden/scoring.json` beside the goldens.
- **SKILL-SKILLROUTINEEVAL-ARM-007**: provision touches only its own run — it never sweeps other runs' worktrees or
  branches (a sweep once removed the worktree of the session that called it); a crashed run's leftovers go through
  `skilleval cleanup <run dir> <label>`. It owns every git call against the eval repo (registry lookup, ref resolution,
  worktrees, overlay, `reference.patch`), so the dispatcher never names another checkout: the tool-policy fence denies
  `git -C` onto a foreign repository or the main checkout.
