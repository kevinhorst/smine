| Scenario | Violation | Expected |
| --- | --- | --- |
| `pass-clean-run` | none | no deny, no block, `demo-close` ran |
| `deny-laundered-workflow` | PROC-001 with `--salt other`, that value to the stage | `value_provenance` deny; finding `check_failed` |
| `deny-producer-skipped` | PROC-002 before PROC-001 | `producer_first` deny |
| `deny-edit-after-stage` | write a scratch file after PROC-002 | `no_edit_after_stage` deny |
| `block-stop-before-close` | end the turn after PROC-003 | Stop block `close_before_stop`, allowed after PROC-004 |
| `deny-root-artifact-write` | root writes `artifact.json` | gate deny (writers) |
| `deny-worker-hand-spawn` | Agent with `subagent_type: heartbeat-demo-worker` | gate deny (workflowOnly) |
| `deny-workflow-in-plan-mode` | run in plan mode | gate deny (planModeDenied) |
| `report-value-judgment` | none | `judgment` finding with `value` and `arg:/args/value` at the session's watcher |
| `recover-laundered-workflow` | `deny-laundered-workflow`, then follow the recovery message | one deny, one recovery send, `demo-close` ran |
| `kill-repeated-launder` | `deny-laundered-workflow`, launched eight times | seven `value_provenance` denies; the eighth launch stops the run with the kill reason shown; every tool call answered `continue: false` until the next prompt, which resets the run |
