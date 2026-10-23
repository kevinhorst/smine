# fleet — design notes

Rationale the skill body does not carry, keyed by entry id. The P4 plan,
[raw-p4-consumer-ui.md](../../../plans/2026-09-27-heartbeat-mvp-wip-c2228c3/design/raw-p4-consumer-ui.md) (§ Decisions),
argues the loop; the member rework,
[change-fleet-members.md](../../../plans/2026-09-27-heartbeat-mvp-wip-c2228c3/design/change-fleet-members.md)
(§ Decisions), argues 2.0.0; the orchestrator branch,
[change-orchestrator-branch.md](../../../plans/2026-09-27-heartbeat-mvp-wip-c2228c3/design/change-orchestrator-branch.md)
(§ Decisions), argues 3.0.0; worker cells and the design playbook,
[fleet-worker-cells.md](../../../plans/2026-10-03-fleet-worker-cells-3a299f8/2026-10-03-fleet-worker-cells-wip-3a299f8/design/fleet-worker-cells.md)
(§ Decisions), argue 5.0.0; the landing plan,
[merge-strategy-landing.md](../../../plans/2026-10-01-merge-strategy-c627d20/2026-10-02-merge-strategy-landing-wip-d0c0b28/design/merge-strategy-landing.md)
(§ Decisions), argues 5.2.0; the landing targets plan,
[merge-strategy-targets.md](../../../plans/2026-10-01-merge-strategy-c627d20/2026-10-03-merge-strategy-targets-wip-cceeb45/design/merge-strategy-targets.md)
(§ Decisions), argues 5.3.0.

- **DEF-001, LOOP-001a:** every wake re-reads the status document; the digest only picks where to look first, so a
  missed, coalesced or re-ordered digest cannot drift the loop (P4 D2). The journal is the loop's memory across
  compactions (members D6).
- **DEF-003:** every member carries a heartbeat-assigned token `<fleet>-M<letter>`, the one name used in chat, messages
  and the journal (members D2). The orchestrator is the operator `<fleet>-O`, never a member: it takes no letter
  (branch D1).
- **DEF-004, DEF-005, DEF-012, LOOP-005a:** a member lands only by fast-forward, so the test is ancestry: the merge
  branch's tip is in its branch. `merge_ready` says a merge would be clean, which is no fast-forward. A member commits in
  pieces, so landing also needs its own `/done`; a worker counts once the orchestrator reported its result `done`
  (members D4, D8; landing D8; cells D5).
- **DEF-008, LOOP-008c:** heartbeat's `synced` is true from the moment the merge branch is cut, because it compares the
  merge branch with the target it was cut from. Synced therefore also requires `integrated`, which needs a delivered,
  merged worker (cells D10).
- **DEF-009, TPL-004:** a worker is a headless `claude -p` cell in its own `.claude/worktrees/<name>` on `claude/<name>`,
  forked from the merge branch. A subagent cannot run fdesign's Workflow stages, and a subagent implementer's own
  subagents were fenced to the orchestrator's worktree. The orchestrator registers the branch as a member, because a
  cell cannot stay one: an ended session leaves the fleet (cells D1, D2, D5, D6).
- **TPL-004:** the launcher takes the skill and its args apart from the slash, because the permission gate reads an
  argv word starting with `/` as a path outside the session roots.
- **DEF-007, START-002a:** the orchestrator never checks out a human branch. It works on its own agent-owned
  `claude-orch/<fleet>-<run uuid>`, which heartbeat tracks as the fleet's merge branch: members are judged against it,
  and a member merged into it reads merged (branch D2, D3, D4). The uuid keeps a reused fleet name off an old run's
  branch; it exists only once the subscribe started the run, so START-002a runs after START-003 and reads the name
  from `fleets[].merge_branch`.
- **START-003:** subscribing binds the fleet to this repo; the sessiontitle hook writes `<fleet>-O` into the title
  (branch D7).
- **RULES-005:** every act is recorded in heartbeat's journal, persisted and shown on the fleet page, because a decision
  that lives only in chat is lost on compaction and invisible to the operator (members D6).
- **RULES-006, LOOP-004:** the orchestrator never writes into a member branch; a member that is behind restacks its own
  branch on request, because the owner knows the intent of its changes. The ask clears the member's delivered mark, so it
  is sent once and the member delivers again (members D11; landing D8).
- **LOOP-005b, LOOP-005c:** a stopped worker has no session, so the orchestrator restacks its branch on a work branch,
  fast-forwards to it and deletes it at once: a work branch that only the merge branch holds would sit in the trains
  until the sync. The plain delete suffices because HEAD holds the work branch after the fast-forward (landing D8).
- **RULES-007:** the dry-run form of `smine-agentwt remove` would retitle this session CLOSED through the sessiontitle
  hook (P4 D6).
- **RULES-001:** the orchestrator files nothing for a human — no defect report, no backlog item; a skill or context
  contradiction it meets reaches smine through its transcript, and the fleet page's queue holds only heartbeat state.
- **LOOP-008e:** the target is written once, at `integrated`, by the done skill for every target. The merge branch was
  created from the target, so heartbeat resolves the target as its landing target; the orchestrator's session is no
  joined member, so its done holds that target's landing turn and lands the merge branch as its owner — a restack if
  the target moved, then the ancestry-checked fast-forward. One sync path means a fleet sync and a solo landing never
  move one ref at once. Git refuses the fast-forward while the target is checked out in another worktree; done ends
  its turn abandoned, the orchestrator journals `sync blocked`, and the fleet page's queue shows it until the human
  frees the branch (branch D5, D9; landing D9; targets D10).
- **LOOP-008f:** a fleet target that is no landing target (the default branch, an agent branch, an unresolved base)
  can never be synced by done; retrying each wake would loop forever, so the run stops and names the target.
- **LOOP-008c, LOOP-008d:** worker worktrees are removed only after the sync: before it their work lives only on the
  agent-owned merge branch, which agentwt's removal gate does not count. The orchestrator then unsubscribes, and the
  human closes the session with `/close`, which accepts `claude-*` branches (branch D6, D8).
- **FEATURE-001..003:** under the feature playbook designers are attended sessions that `/join`. The feature route is
  the only feature-specific part, so the core runs any 2–3-member fleet (members D7).
- **FEATURE-003:** a worker's worktree forks the orchestrator's HEAD, the merge branch, so the design member's plan is
  merged first and the worker finds it on its own branch (members D9).
- **FEATURE-004, FEATURE-005:** the implementation worker is a cell launched `-attended`: without the headless marker
  its `/fimplement` prompt applies the plan's frozen rules, and the orchestrator's operator is the human step. At most
  one runs, a stopped worker's question goes to the design member, and a plan gets at most two cells (cells D6, D9).
- **FEATURE-006:** a stopped cell has ended, but its session resumes: `claude -p --resume <session_id>` over the
  stream-json protocol continued a finished cell with its context (probe 2026-10-03). The launcher keeps a stopped
  implementation cell's worktree, so the resumed session finds its partial work; its `/fimplement` re-anchor restacks it
  onto the merge branch to get the refined plan. A worker cell runs `/fimplement` itself, so it re-anchors on its own and
  never comes back blocked on a branch behind; a delivered worker branch that is behind is restacked by the orchestrator
  (LOOP-005b), since a cell has no session to ask.
- **DESIGN-001..009:** the design playbook reads its rows from the master on the merge branch and proves a row done
  with the section check, which reports the code gate once the row's plan holds its Changes entries. The `designers` cap (default two) bounds the
  spend of one subscription window. A row gets `attempts` cells (default two), then it is held and recorded (cells D7,
  D8).
- **DEF-010, DEF-011:** model, effort, budget, the designer cap and the attempt cap are args since 5.1.0; their
  defaults are the values 5.0.0 fixed in the launch entries, so a call without them is unchanged. The implementer
  count stays a rule: the feature queue, hold and resume entries assume one running worker.
