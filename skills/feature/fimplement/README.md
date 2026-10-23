## Design notes

- **SKILL-FIMPLEMENT-INTAKE-008/009**: in heartbeat sessions fdc550e4 and bd080288 the plan approval lived in another
  session, so the gate's allow command with the exact rows came back uncovered; the hook now fires on the /fimplement invocation
  itself, typed or Skill-chained, and applies the plan's frozen permission record (`permissions.json`) in any worktree;
  a missing report means the hook was not deployed.
- **SKILL-FIMPLEMENT-IMPL-009**: a plan's Permissions rows were edited after approval and the edited rows were applied on
  the next /fimplement invocation; the rules are now a frozen record that only a user-started design run changes.
- **SKILL-FIMPLEMENT-GROUND-005a**: one check call surfaces every permission gap before the first edit instead of mid-unit.
- **SKILL-FIMPLEMENT-GROUND-005d**: user directive 2026-10-01 — a run that halted on a permission gap delivered nothing; a run
  that skips the denied command delivers everything else, and the report's candidate rules let the user widen afterwards.
- **SKILL-FIMPLEMENT-DEVIATE-001**: user directive 2026-10-03 — every stop is gone (no-code-plan ask, section check, caveman,
  tier denial, touch-point defect, two-attempt ceiling, verification gap, unhonorable contract, rework, user override,
  escalation trigger, relay gates, re-anchor failure, behind-target blocked). Stops made runs halt on judgment calls that
  prose cannot encode; a run that halts delivers nothing, a run that makes the closest call and reports it delivers the rest.

- **SKILL-FIMPLEMENT-VERIFY-004**: moving a directory out from under an already-excluded parent silently drops it from the
  exclude set, can start publishing private data.
- **SKILL-FIMPLEMENT-COMMIT-001**: a left-running listener is part of done, not the user's cleanup.
