1. RECORD: no handoff ⇒ $SKILL_RAILROADREVIEW_LEDGER ledger --round-dir <artifactsDir>/round-{r} --void; a handoff ⇒ the round's known-ledger call.
2. RESET: $SKILL_RAILROADREVIEW_RESET_ROUND reset-round --repo <repo> --prefix claude-review/<runNs>- --head <headCommit> --salvage <artifactsDir>/salvage --round-dir <artifactsDir>/round-{r} --round <r> --run <runId> [--run <runId>]… [--root <worktreeRoot> --overlay <overlayPatch>]
   — <headCommit> and every kept run id of the failed attempt; --root/--overlay on the explicit route only; RELAY skipped and unattributed.
3. PIN: headCommit anew — git rev-parse HEAD on branch scope, $SKILL_RAILROADREVIEW_SNAPSHOT snapshot on wip and all.
4. PREPARE: $SKILL_RAILROADREVIEW_BASE base and $SKILL_RAILROADREVIEW_LANES lanes at the new pin, plus the head $SKILL_RAILROADREVIEW_BUILDTEST buildtest whenever the new pin differs from the failed attempt's head — the new baseCommit, lanePlan and buildTest.
5. RELAUNCH: round.js for the same round; keep the new launch's run id.
