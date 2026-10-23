## Design notes

- **SKILL-SKILLROUTINEAB-VAR-001**: model, effort and MCP set vary only through `claude -p` flags, which is why every
  run is a headless root; `no-<mcp>` maps to `--disallowedTools mcp__<server>`, and the arm runner checks the init
  record, because a server that silently fails to connect turns "goland vs grep" into grep vs grep. A version variant
  runs nothing: heartbeat and the gate read the one deployed copy, so two versions cannot run side by side — run,
  deploy, run, compare.
- **SKILL-SKILLROUTINEAB-VAR-002**: one factor per descriptor — a difference is attributable to one change only.
- **SKILL-SKILLROUTINEAB-SIDES-009**: output quality is judged comparatively and blind; the definition's reference
  commit anchors it when a known-good change exists.
- **SKILL-SKILLROUTINEAB-COLLECT-003**: the statistics method is the one the eval skill used before (sample variance,
  Welch's t, two-sided alpha 0.05 table); only its input moved from arms in `eval.json` to sides in `ab.json`.
