changelog: [{
	date:    "2026-10-17"
	version: "9.20.0"
	text:    "No post-implementation UI capture: a UI entry without a design-run capture carries no `ui:` line and Verification no capture row; `ui: after implementation` is retired."
	tokens:  25628
}, {
	date:    "2026-10-17"
	version: "9.19.3"
	text:    "Preamble names read-hook rules instead of projection blocks on governed files."
	tokens:  25670
}, {
	date:    "2026-10-17"
	version: "9.19.2"
	text:    "The sections, persisted_sections and title-set anchors accept a trailing `2>&1` — a smoke session redirecting stderr ran every step yet hit no anchor, so the stop check blocked its turn end three times."
	tokens:  25669
}, {
	date:    "2026-10-17"
	version: "9.19.1"
	text:    "Nightly smoke reachable: CODE-READY title set is its own step (NIGHTLY-008), Persistence spells out the full plan dir path, persisted_sections accepts a quoted path, the smoke expects the persisted plan, its section check and CODE-READY at minChecks 0.4 — the nightly route reaches 3 of 7 checks."
	tokens:  25669
}, {
	date:    "2026-10-17"
	version: "9.19.0"
	text:    "Dropped the caveman and goland args — an IDE MCP server is opted into by the user's message, not a skill arg; the handoff, runbook and mode Effect cells link their entries."
	tokens:  25627
}, {
	date:    "2026-10-17"
	version: "9.18.1"
	text:    "Model file is <leaf>.cue"
	tokens:  26128
}, {
	date:    "2026-10-15"
	version: "9.18.0"
	text:    "Code gate state renamed CODE-REVIEWABLE to CODE-READY (title block, CODE-005 param, title_set_code anchor, code_ready_after_sections check); the Nightly route sets CODE-READY after the persisted plan's section check (NIGHTLY-005 param), so an unattended run ends on the code gate state instead of the last gate it replaced; the gate params no longer say the title shows at the turn end — the title hook renames the turn's first set at the call."
	tokens:  26125
}, {
	date:    "2026-10-14"
	version: "9.17.0"
	text:    "Interfaces: one variable per capability — CAPTURE_UI, PERMISSION_CHECK, PERMISSION_FREEZE, PLAN_CLAIMS, PLAN_DISPLAY, PLAN_SECTIONS, PROJECT_PLAN, TITLE_SET"
	tokens:  26082
}, {
	date:    "2026-10-13"
	version: "9.16.0"
	text:    "Binary-first gates under heartbeat: the two `$SKILL_FDESIGN_RULES title set` gate calls are anchored and a requires check denies each until `$SKILL_FDESIGN_RULES plan-sections` ran after the last plan edit (design_ready_after_sections, code_reviewable_after_sections); the claim ledger comes from `$SKILL_FDESIGN_RULES plan-claims` (REFUTE-002), anchored with a capture, and the refute launch is pinned to it (refute_claims, refute_after_claims); grant Bash($SKILL_FDESIGN_RULES plan-claims *)."
	tokens:  25992
}, {
	date:    "2026-10-13"
	version: "9.15.0"
	text:    "Programs are env variables: $SKILL_FDESIGN_RULES (plan-sections, plan-display, capture-ui, title set), $SKILL_FDESIGN_TOOLPOLICY (check, freeze), $SKILL_FDESIGN_PROJECT (acdsl project -plan, granted) in steps, grants and enforce patterns; the freeze-guarding stop is tagged gate, the code gate no longer writes permissions.json itself and its approval param names no hook; the plan-sections and plan-display steps drop their config-server tag — the variable carries the drop."
	tokens:  25921
}, {
	date:    "2026-10-13"
	version: "9.14.0"
	text:    "Re-gates show only what changed: a `moved` verdict re-presents the design gate as a delta gate (GATE-013) — widget `gate: delta` drops TLDR, Scope and totals, carries only added or changed decisions plus dropped ones as lines, the gate message likewise; decisions not shown stay approved (GATE-003). A revision after approval — base branch moved, code-gate gap — takes the same fold verdict against the approved design; a decision withdrawn because main removed its subject moves nothing (GATE-009, CODE-002). Session d5f2abc3 re-presented the whole gate after main rewrote the title hook, for one new decision."
	tokens:  25804
}, {
	date:    "2026-10-12"
	version: "9.13.0"
	text:    "Title states are set explicitly: the design gate and a re-gate run `smine-rules title set DESIGN-READY`, the code gate `smine-rules title set CODE-REVIEWABLE`, each before its gate message; the title block drops the enter and leave signals — the sessiontitle hook renames at the turn end from the last set call alone; grant Bash(smine-rules title *)."
}, {
	date:    "2026-10-12"
	version: "9.12.0"
	text:    "The context-doc check's miss advice names the config server's Context page sync (Write to Repo) — cmd/sync/sync_context.sh is gone, replaced by the Go context deploy engine (INTAKE-001)."
}, {
	date:    "2026-10-10"
	version: "9.11.0"
	text:    "Phases follow compile dependencies, not only file sets: an entry changing a type, func signature or package API another entry compiles against stands a phase earlier, and entries in one package share a phase only when neither changes what the other compiles against (CODE-002, the plan template's phase text, CODE-004's re-phase). Sessions 2b260689, 78d4af15, 6a6d6a88 and 4892c9ea had disjoint-file siblings break each other's builds and test gates."
}, {
	date:    "2026-10-10"
	version: "9.10.0"
	text:    "Grounding covers what the change reaches: the Ground brief and the ground round ask every area agent for each consumer of what the area changes — callers, libraries reading the same config, the install or deploy reconcile that writes it live, tests pinning it — and for defects on a path the change writes through (TPL-004, WGROUND-001); such a defect is inside the request and enters Scope In with a decision row (INTAKE-018). Sessions 3ea92d5c, 8f89348e and 78d4af15 met an installer-written config, a pinned settings test and a write-through defect only at implementation."
}, {
	date:    "2026-10-10"
	version: "9.9.0"
	text:    "One place for the designed plan: the enforce block blocks the turn end after any edit of the persisted plan until `smine-rules plan-sections` ran on that persisted path (persisted_sections_before_stop) — the session plan file no longer satisfies it once a persisted plan exists, nor a `make rules` detour; the fimplement hook resolves a bare /fimplement from that call alone (session 6a6d6a88 checked only the session plan file at its code gate, so the typed /fimplement resolved nothing). The section-check entry names the persisted path and the skill's own grant (CODE-004)."
	tokens:  25043
}, {
	date:    "2026-10-09"
	version: "9.8.0"
	text:    "Design approval is a user message, never a tool result: in plan mode the acceptEdits switch runs after the plan write and a clean plan-sections, directly before the widget (GATE-012); the gate turn ends after the gate message (GATE-001); the next user message folds (GATE-003) and the standing verdict grants design_approved (GATE-010); persistence (WRITE-002) follows the verdict. The code gate ends its turn the same way, approval is a typed /fimplement or an explicit approving reply, and the append after it is its own step (CODE-009). The switch-card bullets — a Desktop prompt that does not exist, carried over from the ExitPlanMode dialog in 9.4.0 — are gone; session 8c771968 read the switch's success as the approval and wrote code sections without feedback."
	tokens:  24948
}, {
	date:    "2026-10-09"
	version: "9.7.0"
	text:    "Runtime placement declared: root-only (Workflow stages), owns acceptEdits and plan mode; ui-capture in plan mode switches the session to acceptEdits through the Desktop session tool instead of stopping — the stop remains only without that tool."
	tokens:  24694
}, {
	date:    "2026-10-09"
	version: "9.6.0"
	text:    "The fold ends in a verdict the planner states in chat after every fold — three questions: same design, scope substantially more or less, intent changed — standing continues to the code gate, moved presents the design gate again; the planner's call, never a count."
	tokens:  24525
}, {
	date:    "2026-10-09"
	version: "9.5.0"
	text:    "The request is read from the args and the last turns of the conversation: scope the user stated binds, ambiguous scope is the planner's own call, never the args alone; the mode notice names the call before grounding, it lands as Scope In and Out and the design gate settles it; the driver check reads the recent user messages; the planner's own earlier positions stay context."
	tokens:  24059
}, {
	date:    "2026-10-08"
	version: "9.4.3"
	text:    "Widget: every scope move shows an arrow in its target column's color (Not changed a grey right arrow); commented scope items get the yellow star like adjusted decisions; Reset button restores the initial gate; errors spaced from their input; marked-card borders 1.5px with a 5px bar."
	tokens:  23701
}, {
	date:    "2026-10-08"
	version: "9.4.2"
	text:    "Widget scope markers: column headers carry a green check (In), red cross (Out), grey minus (Not changed); an item moved In to Out shows a red up-right arrow, Out to In a green down-left arrow; a decision marked Adjust shows a yellow star."
	tokens:  23701
}, {
	date:    "2026-10-08"
	version: "9.4.1"
	text:    "Widget after live review: `smine.` wordmark moved to the top left (the host's menu sits top right), raised above a lowered TLDR heading, in the config server's neon green `#00ebaa`; OK marks use that green, active Adjust/Reject buttons solid fills; Scope columns spaced from their heading."
	tokens:  23701
}, {
	date:    "2026-10-08"
	version: "9.4.0"
	text:    "Design gate leaves plan mode by switching the session to acceptEdits, never ExitPlanMode — the plan dialog presented the plan and blocked the widget's Send; the enforce check moves to the switch. Widget: TLDR section first (required `tldr` field), `smine.` wordmark top right, OK/Adjust/Reject marks in the saturated color stops."
	tokens:  23701
}, {
	date:    "2026-10-08"
	version: "9.3.0"
	text:    "Design gate shown once: the widget appears one time per run and ExitPlanMode follows it in the same turn, so the feedback arrives outside plan mode, folds and is the approval — the run continues; a feedback item the fold cannot settle earns one chat question, a waiver or no answer leaves the call to the planner; never a second widget, never a second approval wait, never EnterPlanMode."
	tokens:  23675
}, {
	date:    "2026-10-07"
	version: "9.2.1"
	text:    "Mode notice: the plan-mode line states that refutation is off for the run and that leaving plan mode later changes nothing; the exit-and-rerun invitation is gone, and the fixed-intensity param forbids offering one."
	tokens:  23244
}, {
	date:    "2026-10-06"
	version: "9.2.0"
	text:    "Plain gate lines: before the design gate every Scope item, Brief and Decision (brief) line is checked for labels coined during the run and rewritten in plain words, so plan file, widget and gate message match."
}, {
	date:    "2026-10-06"
	version: "9.1.1"
	text:    "The permission check's and freeze's refused-call reasons are read as answers: `tool-policy: CHECK` verdict lines are copied into permissions.json, a `tool-policy: denied` reason or no verdict line stops with no file written, and only `tool-policy: FROZEN` counts as frozen."
}, {
	date:    "2026-10-06"
	version: "9.1.0"
	text:    "ui-capture flag: before the design gate each changed page is implemented in a throwaway scratchpad worktree, run from the project's make target and shot by smine-rules capture-ui; PNGs embed in Target state and on ui: lines, UI sources land in design/ui/; worktree and app stay up across gate rounds and are torn down on design approval; plan mode stops at intake; allowed-tools grants Bash(kill *) for the server stop."
	tokens:  22841
}, {
	date:    "2026-10-05"
	version: "9.0.0"
	text:    "Decisions gain an Adds column (counts plus the reused carrier) and the gate widget shows the plan totals and one adds line per decision; a Changes entry is one agent's work package, at most four per phase, under a phase table; the plan is written heading-first, one section per call; a typed code-gate waiver runs the Trust route; a blank rejection is a rejection; fold docs corrects stale text inside the plan."
	tokens:  21186
}, {
	date:    "2026-10-04"
	version: "8.7.0"
	text:    "Design-gate widget: each scope item takes a comment, and decisions are marked OK, Adjust or Reject — Adjust keeps the option and amends the row per its required comment."
	tokens:  20709
}, {
	date:    "2026-10-04"
	version: "8.6.0"
	text:    "A typed /fimplement at the open code gate is its approval: the tool-policy hook freezes the plan on that prompt, the gate message names it as the way to approve, and the freeze step makes no call when the hook report shows the freeze."
	tokens:  20668
}, {
	date:    "2026-10-04"
	version: "8.5.0"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  20540
}, {
	date:    "2026-10-03"
	version: "8.4.0"
	text:    "Settings deploy named by the install engine"
	tokens:  20557
}, {
	date:    "2026-10-03"
	version: "8.3.0"
	text:    "gate and config-server steps tagged with needs: partial installs and the bundle drop them. References reworded to name no smine program."
	tokens:  20564
}, {
	date:    "2026-10-03"
	version: "8.2.0"
	text:    "Grounding always runs as one round of subagents, at most n (subagents arg, n[model:effort], default 3[opus]), with the same brief in plan mode (Explore agents) and outside (ground.js, size, model and effort from the arg); the planner then fills every gap with its own reads. A standing rule forbids any other spawn, gap agents and code-gate drafting agents included."
	tokens:  20642
}, {
	date:    "2026-10-03"
	version: "8.1.1"
	text:    "A handoff rendering is consumed by /fimplement directly and its escalation triggers are report lines, not blocked results; the fmt and delegate edges, the /fmt plan route and the delegation class are gone — both skills were deleted."
	tokens:  19700
}, {
	date:    "2026-10-03"
	version: "8.1.0"
	text:    "Removed the plan stage field: no args-line stage, nothing waits on it. A refused freeze no longer stops the run or the /fimplement chain; the handoff route no longer refuses an unapproved plan. A refused freeze tells the user to type /fdesign <plan file> to freeze again. Any Edit or Write returns the title to WIP, so implementation starting leaves a gate state."
	tokens:  19757
}, {
	date:    "2026-10-03"
	version: "8.0.0"
	text:    "Plans read decisions first: Decisions directly behind Scope, reader cells lead each row, Changes ordered hot code, other code, agent-only prose. Design gate shows a scope and decisions widget from widgets/design-gate.html; in plan mode it gets its own turn before the approval dialog. Chat messages: only what the operator needs at that gate and does not already know, as a title and short bullets, no bare plan ids."
	tokens:  19945
}, {
	date:    "2026-10-03"
	version: "7.3.0"
	text:    "Fold docs: grounding agents report statements in docs, context and skill text that the tree contradicts or that lack a fact; the planner writes the corrections into those files as its own commit, outside the plan."
	tokens:  19110
}, {
	date:    "2026-10-03"
	version: "7.2.0"
	text:    "delegation: never — grounding and refutation are Workflow stage scripts. An attended run entered through the Skill tool stops at intake (INTAKE-016): only a typed /fdesign prompt may freeze. The nightly persistence grants design_approved, so a nightly derived run corrects the master and writes its code sections. Pipeline: spawnedBy /fleet replaces spawnedBy /parallelize; the derived recipe names /fleet playbook: design."
	tokens:  18878
}, {
	date:    "2026-10-03"
	version: "7.1.0"
	text:    "Declared the input atoms — plan_mode, route, role, trust_arg, caveman_arg — with their arg and session sources; the graph view's invocation tiles set them."
	tokens:  18690
}, {
	date:    "2026-10-02"
	version: "7.0.1"
	text:    "Dropped the help-only Eval section — --help shows the smoke and eval declared beside the skill."
	tokens:  18690
}, {
	date:    "2026-10-02"
	version: "7.0.0"
	text:    "Routes change and refine removed: any request becomes a plan, and a request naming a plan file edits that plan in place while it is not implemented, else plans a sibling in its feature folder. Addenda removed. Plan dirs live in a feature folder. One driver-check subagent compares the request with the Drivers before the design gate; its verdict fills the Check column. Every Changes entry names its drivers. The approved design is committed."
	tokens:  18690
}, {
	date:    "2026-10-02"
	version: "6.9.0"
	text:    "Master and derived plans: `master` permits a coarse split of an oversized plan — a master holds decisions, contracts and the split table, never the code stage, and ends with the recipe for its derived runs; `derived <master plan file> <slug>` plans one split row, restates nothing and corrects the master first; one plan is the default and the planner never splits unasked; in a joined fleet session the code-gate approval is terminal and a typed /done approves it."
	tokens:  18349
}, {
	date:    "2026-10-02"
	version: "6.8.0"
	text:    "Title block: the session title reads DESIGN-READY at the design gate and CODE-REVIEWABLE at the code gate, entered by the section check's gate line and left by a plan write."
	tokens:  15161
}, {
	date:    "2026-10-02"
	version: "6.7.0"
	text:    "--trust: Trust route replaces the code gate — the design-gate approval covers the code sections, the run sets stage: code, freezes and chains into /fimplement without a second approval; preamble and chain rule name the code-gate approval as the default chain trigger."
	tokens:  15140
}, {
	date:    "2026-10-01"
	version: "6.6.0"
	text:    "allowed-tools grants Skill(fimplement) — the approval chain into /fimplement is a declared grant, no longer a global one; pipeline adds from /backlog and spawnedBy /parallelize."
	tokens:  14686
}, {
	date:    "2026-10-01"
	version: "6.5.1"
	text:    "The code-gate message links the session plan file: the Plan UI redraws only on plan-mode entry, so the display copy is opened from the link in the file viewer."
	tokens:  14665
}, {
	date:    "2026-10-01"
	version: "6.5.0"
	text:    "Display copy: smine-rules plan-display renders the persisted plan plus its rules into the session plan file after every plan write, replacing the in-repo rules view; the code gate has no plan-mode branch and its message opens with what approval does; the mode notice fixes the run's intensity."
	tokens:  14576
}, {
	date:    "2026-10-01"
	version: "6.4.0"
	text:    "Plans carry no Permissions section: the code gate writes the plan dir's permissions.json from the toolpolicy check answer and projects it into the plan's working copy; after approval, or at the end of a nightly run, toolpolicy freeze records it and the section check proves it."
	tokens:  14449
}, {
	date:    "2026-10-01"
	version: "6.3.2"
	text:    "ground.js runs its grounding agents on opus, never sonnet — sonnet is unsuited to grounding."
	tokens:  14231
}, {
	date:    "2026-10-01"
	version: "6.3.1"
	text:    "Code gate never re-enters plan mode: CODE-001 (CALL EnterPlanMode) removed — plan mode is the session's state at invocation, the code gate is delivered under whatever state the session is in (ExitPlanMode in plan mode, chat plus the plan file otherwise)."
}, {
	date:    "2026-09-30"
	version: "6.3.0"
	text:    "Refutation bounded to the Assumptions rows: one round of two blind refuters and a tie-break (refute.js without round and kind), the planner confirms each debunk from its cited evidence and folds once, two attempts per assumption, no agent spawned outside the stage scripts; no OPEN row or question in any route — every question a decision row; request boundary rule; CODE-003 deleted; plan template re-cut onto the human-first catalog with [AGENT-ONLY] markers, Brief and Decision (brief) columns, Verification table, hot: lines, changed-UI-from-dummy screenshots, refuted marker on the args line; enforce gate planModeDenied Workflow."
	tokens:  14296
}, {
	date:    "2026-09-30"
	version: "6.2.0"
	text:    "Refutation cut to one layer: refute.js judges only the plan's written claims — one blind refuter and one verifier over the ledger, a tie-break agent for the split claims; no implied-claim extraction, no until-clean loop — round 2 covers only the claims revised after round 1, then a still-debunked claim is an OPEN row. The code-gate refute checks one claim per Changes entry into its own verdict dir. schemas/claim.json deleted."
}, {
	date:    "2026-09-30"
	version: "6.1.2"
	text:    "refute.js batches at 50: one extract agent per 50 entries, then explicit and implied claims settled together by one refuter and one blind verifier per 50 claims and one tie-break agent per 50 split claims — a round is about four agents."
}, {
	date:    "2026-09-30"
	version: "6.1.1"
	text:    "refute.js batches: one refuter and one blind verifier per batch of 10 claims, one tie-break agent per batch of split claims, one extract agent per batch of 8 entries — agent count scales with batches, not claims; schemas/claim-verdicts.json replaces claim-verdict.json."
}, {
	date:    "2026-09-29"
	version: "6.1.0"
	text:    "Two gates: the design gate (TLDR through Baseline) is presented and approved before any Changes entry is written, then the code gate (Exemplar & reuse through Verification, with phase: lines checked by smine-rules plan-sections) before /fimplement; a mode notice opens every run; an enforce block denies ExitPlanMode and blocks turn end until plan-sections ran clean after the last plan edit; the template drops Deferred findings."
}, {
	date:    "2026-09-29"
	version: "6.0.0"
	text:    "major: Stop conditions section removed from the plan template; outside plan mode a Workflow grounding route (ground.js) replaces the Phase 1 reads and a Refute route (refute.js) runs implied-claim extraction plus a blind refuter and verifier per claim, a third agent on a split, until zero debunked and zero unverified; residue becomes OPEN rows; plan mode keeps Explore grounding and marks the plan unrefuted; allowed-tools grants Workflow; nightly halts recorded in the commit body and Changelog."
}, {
	date:    "2026-09-29"
	version: "5.8.0"
	text:    "One plan file per plan dir, design/<slug>.md: refine edits it in place; change edits its header in place and appends its code as an addendum under ## Addenda; Permissions stay in the one ## Permissions table."
}, {
	date:    "2026-09-28"
	version: "5.7.0"
	text:    "Permissions built from one toolpolicy check call with a checked: evidence line; plans with settings.local.json rows persist to a plan dir, and the auto-chain passes the plan file to /fimplement, whose hook applies the rows."
}, {
	date:    "2026-09-28"
	version: "5.6.1"
	text:    "Removed the bundled subsystem-grounding workflow: no step ever invoked it; the planner fans out its own Explore agents per ACTION-NAV-GROUND-001. IDE opt-in handoff names spawned agents only."
}, {
	date:    "2026-09-28"
	version: "5.6.0"
	text:    "Decisions table gains Rejected and Consequences columns and a standalone one-paragraph Problem; Drivers reordered to ID | Origin | Observed | Wanted | Impact with Origin quoted verbatim; OPEN row shape follows."
}, {
	date:    "2026-09-28"
	version: "5.5.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-27"
	version: "5.5.2"
	text:    "Declared Delegation: degraded — Agent-tool fan-out, degrades below root."
}, {
	date:    "2026-09-27"
	version: "5.5.1"
	text:    "Bracket and arm tokens use <model>:<effort>"
}, {
	date:    "2026-09-26"
	version: "5.5.0"
	text:    "Route sections declare their anchor: Handoff route replaces INTAKE-001..HANDOFF-002a (early exit over Phases 0-4), Nightly route replaces INTAKE-006 + INTAKE-007 + INTAKE-009a + HANDOFF-001; route lines stay in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "5.4.0"
	text:    "--nightly: Nightly route — no question, wait or plan mode (questions become OPEN rows), plan persisted to the plan dir without approval, no /fimplement chain, .routine-commit-body names plan and OPEN rows; restored the ## Model section lost in 4fe3d38a1"
}, {
	date:    "2026-09-26"
	version: "5.3.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; IDE opt-in handoff rule passes the tokens to spawned agents, subsystem-grounding.js takes ide[] and puts the opt-in into every Ground prompt; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-26"
	version: "5.2.3"
	text:    "Added the help-only ## Eval section: a /skillroutine-eval plan-quality compare, noting headless arms end at the written plan; shown by --help, stripped on deploy."
}, {
	date:    "2026-09-26"
	version: "5.2.2"
	text:    "Permission inventory: settings.local.json rows need no program exclusions; the gate authorizes the approved plan's rows."
}, {
	date:    "2026-09-25"
	version: "5.2.1"
	text:    "Permissions template: settings.local.json rows are applied by toolpolicy allow, never by editing the file."
}, {
	date:    "2026-09-25"
	version: "5.2.0"
	text:    "Plan template gains the Permissions section; Phase 1 inventories the allow rules the implementation needs and their layer."
}, {
	date:    "2026-09-25"
	version: "5.1.2"
	text:    "Make-only entry points: prose calls make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "5.1.1"
	text:    "/clarify absorbed into /concept: disambiguation row and handoff stop point at /concept"
}, {
	date:    "2026-09-14"
	version: "5.1.0"
	text:    "Formal-schema reformat: bullet register normalized to `* param:` throughout; formal pred/post clauses added to 18 entries (3 invariants, 12 guarded transitions incl. the handoff route's stop/spawn/step chain and the approval → auto-implement rule); Handoff route tagged `{section: route, pred: route = handoff}`; new sibling atoms.json (18 bool atoms, 1 enum `route`); three queries added under queries/ (small-plan-blocks-render, approval-forces-implement, handoff-render-witness); caveman-compressed body prose, technical content unchanged."
}, {
	date:    "2026-09-14"
	version: "5.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-08"
	version: "4.1.0"
	text:    "Added the handoff route — renders an approved plan into a runner-facing artifact a weaker model implements mechanically (full final code, dependency-ordered build sequence, known-red ledger, forbidden fixes, escalation contract); added the target-tier args-line entry (WRITE-005) so every plan records its intended runner tier per the new plan-format rule"
}, {
	date:    "2026-09-08"
	version: "4.0.1"
	text:    "Handoff: a plan whose Scope is entirely postponed/design-reference is a design-only artifact — approval is terminal without an explicit 'only plan', auto-implement fires only on an in-scope non-postponed change (HANDOFF-002a/002b)"
}, {
	date:    "2026-09-07"
	version: "4.0.0"
	text:    "major: reformatted to skill format v2 — When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), Args bulleted list -> 4-column table with title-links into owning entries, internal entry cross-reference (DECIDE-001) turned into a title link, argument-hint rewritten as a full example invocation; content and entry IDs unchanged"
}, {
	date:    "2026-09-06"
	version: "3.16"
	text:    "Changes template placeholder now requires the planned-files table (File | Kind | Entry) at the top of Changes, per the new plan-format inventory rule"
}, {
	date:    "2026-09-05"
	version: "3.15"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-05"
	version: "3.14"
	text:    "Phase 2 DECIDE-006 — a terse answer selects among the options presented and grants nothing beyond them; implied machinery (UI toggle, persisted concept, store, config field) is a fresh OPEN row, never built from the terse answer"
}, {
	date:    "2026-09-04"
	version: "3.13"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not feature-planning input"
}, {
	date:    "2026-09-04"
	version: "3.12"
	text:    "Plan interface: frontmatter plan-template names TPL-001 (gate ACDSL-SKILL-007); TPL-001 gains the Changelog section; Hot items and WRITE-001 point at the visual-evidence and delivery rules; scripts/capture-ui.sh bundled — headless Chrome / simulator PNG capture for plan evidence"
}, {
	date:    "2026-09-04"
	version: "3.11"
	text:    "acdsl-context narrowed: review-chapter entries dropped — the Definition of Done binds on reviewers, not on plan authoring; chapter list in the body updated to match"
}, {
	date:    "2026-09-02"
	version: "3.10"
	text:    "INTAKE-009: change-route driver-list triage — surface under-specified drivers as Open Questions in chat before the first plan draft, resolving ambiguity inline rather than after a plan rejection"
}, {
	date:    "2026-09-01"
	version: "3.9"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "3.8"
	text:    "Plan paths migrated to the dated plan-dir convention (RULE-PLAN-071/072): persistence, exploration input, UI screenshots and runbook locations addressed via the resolved plan dir"
}, {
	date:    "2026-08-22"
	version: "3.7"
	text:    "bundled workflow subsystem-grounding.js — fans one Explore agent per named subsystem over the live tree, collects each file:line report at a barrier, then folds every report verbatim into the fchange/fdesign plan; grounds a multi-subsystem plan deterministically instead of hand-deciding fan-out width each run"
}, {
	date:    "2026-08-27"
	version: "3.6"
	text:    "plan template: Open questions moved directly after Decisions; F/D IDs plain text with section-slug links instead of HTML anchors; Hot items require actual-UI screenshots under plans/{slug}/design/ui/ and Changes entries a ui: reference line"
}, {
	date:    "2026-08-23"
	version: "3.5"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "3.4"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-23"
	version: "3.3"
	text:    "acdsl-context: key rename — the ACDSL declaration key moved off Claude Code's native context: frontmatter field; prose references updated"
}, {
	date:    "2026-08-21"
	version: "3.2"
	text:    "Phase 4 — Approval handoff: on plan approval the default continuation is /fimplement in the same session, without asking; only an explicit contrary statement makes approval terminal"
}, {
	date:    "2026-08-20"
	version: "3.1"
	text:    "description rewritten under the brevity caps; Hot items template requires a rendered mockup/screenshot for UI-touching changes (ACTION-CONCEPT-HOT-007)"
}, {
	date:    "2026-08-19"
	version: "3.0"
	text:    "major: body migrated to entries via /fmt skill — 24 entries (INTAKE 8, GROUND 6, DECIDE 5, WRITE 4, TPL 1 payload), content unchanged; the plan-template fence, previously unclosed at EOF, is now a closed four-backtick payload fence"
}, {
	date:    "2026-08-18"
	version: "2.14"
	text:    "absorbs fchange v1.14 as the change route (/fdesign change <drivers>) — C0 intake with internal impact classification, C1 ground inventories, C2 decide extras, change-route stops, route-conditional gate items; one union plan template with [new]/[change] sections (N/A — <other> route), two titles, route: change on the args line; change plans persist to design/change-<topic>.md; fchange retired"
}, {
	date:    "2026-08-13"
	version: "2.13"
	text:    "plan-quality doctrine from mined sessions — Phase-4 gate items (simplicity = fewest concepts + SSOT not smallest diff, replacement-vs-supplement, pasted artifacts immutable, UI plans carry a rendered mockup), Phase-1 premise-validation step, Phase-0 errata-supersede-spec, and a Design-type doctrine section (diagnosis trace, viability, handoff docs, model bake-off)"
}, {
	date:    "2026-08-12"
	version: "2.12"
	text:    "migrated to the context: declaration — doctrine, plan-format, and fact entries injected at invocation by the skill-context hook; SessionStart-injection wording retired"
}, {
	date:    "2026-08-11"
	version: "2.11"
	text:    "Test runbook is opt-in — default plans carry a scenario index only, full tool-native request files require the runbook arg or a user ask (no popup); context-path naming corrected (rules/plan.md, doctrine chapters under actions/)"
}, {
	date:    "2026-08-10"
	version: "2.10"
	text:    "Phase-5 approval handoff no longer routes via AskUserQuestion — approval means implement (via /fimplement in-session), plan-only is explicit; the skill never calls AskUserQuestion"
}, {
	date:    "2026-08-09"
	version: "2.9"
	text:    "language-agnosticism sweep — \"governed files\" wording, vendored bin/acdsl note on project -plan, concurrency-primitive phrasing in the gate checklist"
}, {
	date:    "2026-08-09"
	version: "2.8"
	text:    "ACDSL is the referenced context system — SessionStart delivery wording, gates authoritative, project -plan per planned file in Ground"
}, {
	date:    "2026-08-07"
	version: "2.7"
	text:    "contract files are *.acdsl — path-agnostic, plans/{slug}/design/ is convention only; retire by renaming away from .acdsl"
}, {
	date:    "2026-08-07"
	version: "2.6"
	text:    "task-contract emission — plans in ACDSL repos persist design/contract.md; gate via check -lifetime task"
}, {
	date:    "2026-07-31"
	version: "2.5"
	text:    "hot-class gates read from rules/concepting.md"
}, {
	date:    "2026-07-31"
	version: "2.4"
	text:    "activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries"
}, {
	date:    "2026-07-30"
	version: "2.3"
	text:    "context redesign — plan-format.md, stop-conditions.md, hot-items.md relocated into this skill's assets/; data-integrity doctrine at $AGENT_CONTEXT_DIR_DEFAULT/rules/integrity.md"
}, {
	date:    "2026-07-30"
	version: "2.2"
	text:    "absorbs design-refine v1.6 as the refine route (/fdesign refine <plan file>) — driver classification, impact map, redesign discipline, three-place delta reporting, refine gate + stops; design-refine retired"
}, {
	date:    "2026-07-26"
	version: "2.1"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "2.0"
	text:    "renamed feature-design → fdesign (leaf dir, frontmatter name, H1); live references swept repo-wide"
}, {
	date:    "2026-07-24"
	version: "1.9"
	text:    "Test runbook section — smoke scenarios as out-of-the-box request files in the project's tool, persisted to plans/<feature>/runbooks/; gate item; plan-format/DoD wired"
}, {
	date:    "2026-07-24"
	version: "1.8"
	text:    "reference merge: feature-refine + feature-refactor → feature-change; effort token normalized"
}, {
	date:    "2026-07-19"
	version: "1.7"
	text:    "reference rename: refactor → feature-refactor; moved under skills/feature/"
}, {
	date:    "2026-07-19"
	version: "1.6"
	text:    "Phase 5 approval handoff — approval ≠ implement; AskUserQuestion routes ad hoc vs /feature-implement vs plan-only unless pre-clarified"
}, {
	date:    "2026-07-16"
	version: "1.5"
	text:    "plan persists to plans/{slug}/design/raw.md; exploration input from design/exploration.md; TLDR section"
}, {
	date:    "2026-07-15"
	version: "1.4"
	text:    "feature-refine renamed to design-refine; post-implementation adjustments route to the new /feature-refine"
}, {
	date:    "2026-07-13"
	version: "1.3"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-11"
	version: "1.2"
	text:    "data-integrity.md wired into preamble and Phase-0 check; decision axes controllable/debuggable/reliable"
}, {
	date:    "2026-07-10"
	version: "1.1"
	text:    "template reordered per plan-format v2; Assumptions section; Phase-0 doc-check/modes/exploration input; extended gate checks"
}, {
	date:    "2026-07-03"
	version: "1.0"
	text:    "initial version"
}]
enableDelegation: "never"
enforcePermissions: ["acceptEdits", "planMode"]
env: {
	CAPTURE_UI: {
		interface: "capture-ui@1"
	}
	PERMISSION_CHECK: {
		interface: "permission-check@1"
	}
	PERMISSION_FREEZE: {
		interface: "permission-freeze@1"
	}
	PLAN_CLAIMS: {
		interface: "plan-claims@1"
	}
	PLAN_DISPLAY: {
		interface: "plan-display@1"
	}
	PLAN_SECTIONS: {
		interface: "plan-sections@1"
	}
	PROJECT_PLAN: {
		interface: "project-plan@1"
	}
	TITLE_SET: {
		interface: "title-set@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " fdesign"
}, {
	key:   "description"
	value: " Produce the plan for feature work — a plan, not code. Trigger on /fdesign [<plan file>|handoff|master|derived]. Args — plan file: edit that plan; handoff: runner render; master: split if oversized; derived: one split row; mode: depth; runbook: request files; subagents: grounding round n[model:effort], default 3[opus]; ui-capture: real UI screenshots pre-gate; --trust: no code gate; --nightly: unattended."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 9.20.0"
}, {
	key:   "title-code"
	value: " FD"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " 'plans/2026-09-07-cart-abc1234/2026-09-07-cart-fix-wip-abc1234/design/cart-fix.md add the coupon edge case | handoff: plans/2026-09-07-cart-abc1234/2026-09-07-cart-fix-wip-abc1234/design/cart-fix.md mode: unfamiliar runbook subagents: 3[sonnet:medium] ui-capture --trust | master: plans/2026-09-07-cart-abc1234/2026-09-07-cart-wip-abc1234/design/cart.md | derived: plans/2026-09-07-cart-abc1234/2026-09-07-cart-wip-abc1234/design/cart.md cart-checkout | --nightly'"
}, {
	key:   "acdsl-context"
	value: " ACTION-CONCEPT-*, ACTION-IMPL-*, ACTION-NAV-010, ACTION-NAV-011, ACTION-NAV-GROUND-*, RULE-PLAN-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Skill(fimplement), Workflow, Bash($SKILL_FDESIGN_PLAN_CLAIMS plan-claims *), Bash($SKILL_FDESIGN_PLAN_DISPLAY plan-display *), Bash($SKILL_FDESIGN_PLAN_SECTIONS plan-sections *), Bash($SKILL_FDESIGN_PROJECT_PLAN project *), Bash($SKILL_FDESIGN_TITLE_SET title *), Bash(kill *)"
}, {
	key:   "plan-template"
	value: " SKILL-FDESIGN-TPL-001"
}]
preamble: """
	# fdesign

	Produces one artifact: plan.

	Plan roots implementation: premises refuted once by two blind agents, every question decided by the planner and recorded —
	never an OPEN row, never a design question (one chat question only for a gate feedback item the fold cannot settle);
	implementation executes, never discovers. Skill never implements — /fimplement executes
	approved plan, default right after code-gate approval (Phase 5), unless user says otherwise; design-gate approval alone
	starts it only under `--trust`.

	Static constraints stay unrepeated here: `AGENTS.md`, format guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (incl.
	`plan.md`), doctrine chapters under `$AGENT_CONTEXT_DIR_DEFAULT/actions/` (concepting — hot-class gates; implementing — stops,
	integrity) govern every edit. Arrive via this skill's frontmatter `acdsl-context:` declaration, injected at invocation by
	skill-context hook; per-language style guides read from `$AGENT_CONTEXT_DIR_DEFAULT/rules/` on demand. Re-read entries from
	`$AGENT_CONTEXT_DIR_DEFAULT/` only once dropped from context. Checkable slice enforced by ACDSL gates
	(the repo's check command, read-hook rules on governed files, `project -plan` for planned paths) — where gate covers it, gate
	rules, not prose.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "concept"
				}, {
					kind:  "from"
					skill: "fexplore"
				}, {
					kind:     "from"
					skill:    "backlog"
					optional: true
				}, {
					kind:     "from"
					skill:    "code-verdict"
					optional: true
				}, {
					kind:     "from"
					skill:    "diagnose-debug"
					optional: true
				}, {
					kind:     "from"
					skill:    "fimplement"
					optional: true
				}, {
					kind:  "to"
					skill: "fimplement"
				}, {
					kind:     "to"
					skill:    "overlap"
					optional: true
				}, {
					kind:  "spawnedBy"
					skill: "fleet"
				}]
				note: "a request naming a plan file: edits it, or plans a sibling once implemented; handoff route → /fimplement; master route → one /fdesign derived run per split row, typed or as /fleet playbook: design cells"
			}
			preconditions: ["stable requirement source — clarified concept (plan dir's `concept/`, resolved per plan-dir resolution rule in rules/plan.md), design doc, precise request, the conversation the invocation closes, or an existing plan file plus the ask; `design/exploration.md` present → chosen option binds as input", "handoff route: a plan this skill produced", "master route: `master` typed by the user — a request, or a design-approved plan this skill produced", "derived route: master plan file on the branch, `<slug>` a row of its split table"]
			purpose: "plans feature work — a request, or an edit of an existing plan named by its file; handoff route: rendering approved plan runner-mechanical for weaker model; master route: plan too large for one code gate — decisions, contracts and a coarse split into derived plans; derived route: one split row of a master as its own plan."
			routes: [{
				for: "solution space still open or contested"
				use: "/fexplore"
			}, {
				for: "product shape / user stories unsettled"
				use: "/concept"
			}, {
				for: "behavior deviates from the approved plan (a bug)"
				use: "/diagnose-debug"
			}, {
				for: "executing an approved plan"
				use: "/fimplement"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "plan file"
				default: "absent → new plan"
				effect:  "edits that plan in place while not implemented, else plans a sibling in its feature folder — [Existing plan](#skill-fdesign-intake-015)"
				form:    "`<plan file> <ask>`, free text, plan file first"
			}, {
				name:    "handoff"
				default: "absent"
				effect:  "routes to the Handoff route — renders the approved plan for a weaker runner — [Rendering deltas](#skill-fdesign-handoffroute-005)"
				form:    "`handoff <plan file>`"
			}, {
				name:    "master"
				default: "absent → one plan, never split"
				effect:  "permits a coarse split of an oversized plan into derived plans; unsplit it stays one plan — [Master plan](#skill-fdesign-master-001)"
				form:    "`master <request>` or `master <plan file>`"
			}, {
				name:    "derived"
				default: "absent"
				effect:  "plans one split row of a master in its own plan dir, restating nothing — [Derived plan](#skill-fdesign-derived-001)"
				form:    "`derived <master plan file> <slug>`"
			}, {
				name:    "mode"
				default: "`familiar`"
				effect:  "explanation depth only, never rigor or code — [Familiarity mode](#skill-fdesign-intake-002a)"
				form:    "`unfamiliar\\|familiar\\|owned`"
			}, {
				name:    "runbook"
				default: "absent"
				effect:  "full tool-native runbook request files in plan; default scenario index only — [Plan template](#skill-fdesign-tpl-001)"
				form:    "flag"
			}, {
				name:    "subagents"
				default: "`3[opus]`"
				effect:  "sizes the one grounding round: at most n agents on the bracket model and effort; gaps after it are the planner's reads — [Spawn bound](#skill-fdesign-ground-011)"
				form:    "`subagents: <n>[<model>:<effort>]`"
			}, {
				name:    "ui-capture"
				default: "absent → no `ui:` line"
				effect:  "implement each changed UI in a throwaway worktree, screenshot the running app, embed before the design gate — [UI capture](#skill-fdesign-uicap-001)"
				form:    "flag"
			}, {
				name:    "--trust"
				default: "absent → code gate waits"
				effect:  "design-gate approval covers the code: no code gate, code sections written, frozen, /fimplement chained — [Trust substitution](#skill-fdesign-trust-001)"
				form:    "flag"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: no question, no approval, plan persisted to plan dir, no /fimplement chain — [Nightly substitution](#skill-fdesign-nightly-001)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Phase 0 — Intake"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-INTAKE-002a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "facts, decisions, hot items, Changes code/diffs stay identical in every mode (rules/plan.md)."
			}]
			role:      "def"
			statement: "Intake arg `mode: unfamiliar | familiar | owned`, default `familiar`; explanation depth only, never rigor or code."
			title:     "Familiarity mode"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-002b"
			items: [{
				depth:  1
				marker: "-"
				text:   "`unfamiliar`: plan gains mandatory `Flow trace` subsection under Context — per-component end-to-end trace with user's real sample values, break point marked — plus inline explanations of domain terms. Preference questions wait until after trace: teach first, ask second."
			}, {
				depth:  1
				marker: "-"
				text:   "`familiar`: default behavior described in this skill."
			}, {
				depth:  1
				marker: "-"
				text:   "`owned`: Context ≤3 bullets, Baseline holds only pivotal facts, no diagrams, no explanatory prose. Changes keep full code and diffs — only surrounding prose cut."
			}]
			role:  "def"
			title: "Mode behaviors"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-008a"
			role:      "def"
			statement: "\"Replace\" means old thing gone at end: no parallel old/new mechanisms, no code still reading or accepting old format \"just in case\"."
			title:     "Replace semantics"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-018"
			items: [{
				label:  "param"
				marker: "*"
				text:   "ambiguous scope is the planner's call: a subject the recent turns settled — corrected or left standing — is in; one the user dropped or finished is out."
			}, {
				label:  "param"
				marker: "*"
				text:   "the call lands as the plan's Scope In and Out — a recent subject left out is an Out item — and the design gate's feedback settles it."
			}, {
				label:  "param"
				marker: "*"
				text:   "a defect on a path the planned change writes through is inside the request — it enters Scope In with a decision row."
			}, {
				label:  "param"
				marker: "*"
				text:   "the planner's own earlier turns are context: a correction decides; a position left standing is the planner's decision row, never `[USER]`."
			}, {
				label:  "param"
				marker: "*"
				text:   "no earlier turns — a first-message or dispatched invocation — make the args the request."
			}]
			role:      "def"
			statement: "The request is read from the args and the last turns: scope the user stated binds; where it is ambiguous the planner decides itself; the args alone are never the request."
			title:     "Request"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-015"
			items: [{
				label:  "param"
				marker: "*"
				text:   "implemented: the branch holds a commit newer than the plan's code-approval commit that touches a file of its planned-files table."
			}, {
				label:  "param"
				marker: "*"
				text:   "in-place edit: one Changelog row `fdesign: <ask>`, design gate only when a design-gate section changed."
			}, {
				label:  "param"
				marker: "*"
				text:   "sibling: the named plan binds as design doc per [Design doc input](#skill-fdesign-intake-005a) and is never edited."
			}]
			role:      "def"
			statement: "Args whose first word is a plan-dir plan file name an existing plan: not implemented ⇒ edited in place; implemented ⇒ new plan dir in its feature folder."
			title:     "Existing plan"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-005a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "Phase 1 still verifies assumptions against repo (false assumption = finding, not redesign license); Phase 2 asks only gaps it leaves open."
			}]
			role:      "rule"
			statement: "Design doc/spec MUST NOT skip phases — fixes decisions; plan elaborates it to file-level precision, never altering its architecture."
			title:     "Design doc input"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-005b"
			role:      "rule"
			statement: "Disagree with design doc at most once, clearly labeled, then execute decision."
			title:     "Design doc disagreement"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-006"
			role:      "rule"
			statement: "Feedback or errata accompanying spec supersedes spec text it corrects; a contradiction MUST be decided for the errata and recorded as a decision row — never asked."
			title:     "Errata supersede"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-007"
			role:      "rule"
			statement: "\"Without X\" excludes X and nothing else — rest keeps full scope. An exclusion that could read as shrinking core task MUST be decided literal and recorded — never asked."
			title:     "Exclusion scope"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-008b"
			role:      "rule"
			statement: "After authorized clean break, backward-compatibility code is bug, not courtesy."
			title:     "Post-replace code"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-012"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a discovery outside the request never enters the plan — no In fold, no Out mention, no decision row, no chat note."
			}, {
				label:  "param"
				marker: "*"
				text:   "Assumptions rows are premises the request or its named artifacts state, never premises the planner adds."
			}]
			role:      "rule"
			statement: "Drivers and Scope MUST come from the user's own words or artifacts the user named (failing test, review doc, plan file); text an invoking agent or skill added is context, never a driver."
			title:     "Request boundary"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-019"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the call: `mcp__ccd_session_mgmt__set_session_permission_mode` with `session_id: \"self\"`, `mode: \"acceptEdits\"`; the run is then a Workflow run."
			}]
			post: []
			pred:      "ui_capture_arg and plan_mode and mode_switch_tool"
			role:      "step"
			statement: "SWITCH the session to acceptEdits before the mode notice when `ui-capture` is set in plan mode — the capture needs a worktree, a build and a running app."
			title:     "Plan-mode capture switch"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-011"
			items: [{
				label:  "param"
				marker: "*"
				text:   "plan mode: \"Plan-mode run — no refutation this run, driver check runs; args line `refuted: no (plan-mode run)`. Leaving plan mode changes nothing.\""
			}, {
				label:  "param"
				marker: "*"
				text:   "outside: \"Workflow run — one refute round: two blind refuters and a tie-break over the Assumptions rows, three agents at most.\""
			}, {
				label:  "param"
				marker: "*"
				text:   "both: append \"Grounding: one round of at most <n> <model> agents, then own reads.\""
			}, {
				label:  "param"
				marker: "*"
				text:   "ui-capture: append \"UI capture: each changed page implemented in a throwaway worktree and screenshotted before the design gate.\""
			}, {
				label:  "param"
				marker: "*"
				text:   "intensity is fixed for the run — a later mode change starts no ground or refute round, leaves the `refuted:` marker; never offer an exit or rerun."
			}, {
				label:  "param"
				marker: "*"
				text:   "request: one line naming the subjects the plan covers and the recent subjects it leaves out — the scope call per [Request](#skill-fdesign-intake-018), before grounding spends."
			}]
			role:      "step"
			statement: "STATE the run's intensity as first output, before any grounding — which mode runs, what runs in it, what the plan will and will not carry."
			title:     "Mode notice"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-016"
			needs: ["gate"]
			pred:      "mode != nightly and not typed_start"
			role:      "stop"
			statement: "If this attended run was entered through the Skill tool, not a `/fdesign` prompt the user typed ⇒ STOP: ask the user to type `/fdesign <request>`; only a typed prompt can freeze the plan's rules."
			title:     "Untyped start"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "doc may arrive injected, not read directly; a miss means the repo's context was never deployed — config server's Context page sync, Write to Repo on."
			}, {
				label:  "param"
				marker: "*"
				text:   "baseline = baseline hot classes."
			}]
			role:      "step"
			statement: "VERIFY `rules/plan.md` and `actions/` chapters (concepting, implementing) under `$AGENT_CONTEXT_DIR_DEFAULT/`; missing doc: state once, continue on baseline — never stall."
			title:     "Context-doc check"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-017"
			pred:      "ui_capture_arg and plan_mode and not mode_switch_tool"
			role:      "stop"
			statement: "If `ui-capture` is set in plan mode and the Desktop session tool is absent ⇒ STOP: ask the user to leave plan mode (Shift+Tab), then rerun."
			title:     "Plan-mode capture"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "space still open, no exploration done — flag it, suggest /fexplore — never lock one design silently."
			}]
			post: ["option_chosen"]
			pred:      "exploration_present"
			role:      "verify"
			statement: "VERIFY plan dir holds `design/exploration.md` (from /fexplore, glob-resolved per plan-dir rule); chosen option binds as `[USER]` decision citing it, rejected options stay unre-explored."
			title:     "Exploration input"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-009a"
			post: ["ambiguity_resolved"]
			role:      "step"
			statement: "Before drafting from multi-driver punch-list, RESOLVE every under-specified driver by the planner's reading; each resolution is a decision row (per [Decide everything](#skill-fdesign-decide-001))."
			title:     "Driver resolution"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-INTAKE-013"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the master binds as design doc per [Design doc input](#skill-fdesign-intake-005a); request text beyond the row adds no scope."
			}]
			pred:      "role = derived"
			role:      "step"
			statement: "Derived route: READ the master's split row for `<slug>` — Carries is Scope In, Decisions the carried `D<n>`, Contracts rows naming the slug the carried `C<n>`, Depends on the siblings."
			title:     "Split row"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-INTAKE-014"
			pred:      "role = derived"
			role:      "stop"
			statement: "If derived route and the master lacks `role: master`, its split table lacks `<slug>`, or a wip plan dir for `<slug>` exists ⇒ STOP: name the miss; an existing plan is edited by naming its file."
			title:     "Row unusable"
		}
	}]
}, {
	title: "Phase 1 — Ground"
	blocks: [{
		prose: """
			Produce evidence, not opinions. Each step yields lines in plan's Baseline and Current state; a reuse answer lands in
			its decision's Adds cell.
			"""
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-002b"
			role:      "def"
			statement: "Mirroring means copying exact structure — file layout, naming, helper shape, test skeleton — not just same general technique."
			title:     "Mirroring definition"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-006b"
			role:      "rule"
			statement: "User-run prod measurements are binding facts, not hypotheses to re-derive."
			title:     "Prod measurements binding"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-011"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`subagents: <n>[<model>:<effort>]` sizes the round — n agents at most on the bracket model and effort; absent: 3 agents, `opus`."
			}, {
				label:  "param"
				marker: "*"
				text:   "after the round every gap is the planner's own read."
			}]
			role:      "rule"
			statement: "Planner MUST NOT spawn an agent or workflow beyond the one ground round, the refute round and the driver check — never for a gap, never for code sections."
			title:     "Spawn bound"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-012"
			role:      "rule"
			statement: "Planner MUST ground through the one ground round of subagents — its own reads fill only the gaps the round leaves."
			title:     "Round mandatory"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "assumed-but-missing refactor is headline finding, not footnote."
			}]
			role:      "step"
			statement: "READ named base branch; check every assumption design doc makes against actual repo state, record deltas in plan's Assumptions section."
			title:     "Baseline"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "record governing rules in that file's Changes entry."
			}]
			role:      "step"
			statement: "In ACDSL repos (`acdsl/registry.json` present), RUN `$SKILL_FDESIGN_PROJECT_PLAN project -plan <path>` for every file plan touches."
			title:     "ACDSL projection"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-002a"
			role:      "step"
			statement: "For every new artifact (service, model, test file), NAME nearest sibling it mirrors."
			title:     "Exemplar"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-003"
			role:      "step"
			statement: "LIST existing models/generics, helpers, key functions, generators/Makefile targets producing or validating artifacts involved. Never hand-write generated format."
			title:     "Reuse inventory"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-007a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`./bin/server serve`, never `bin/server *`; git without `-C`; one command per call; make with declared vars."
			}]
			role:      "step"
			statement: "LIST every command, URL, tool that implementation and verification will run — Changes, Tests, Verification, Test runbook, commit — in exact invocation form, as runner types it."
			title:     "Command inventory"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-007b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "admitted — into the file's `checked` list."
			}, {
				label:  "param"
				marker: "*"
				text:   "denied, self-serviceable — one `rules` object: `rule` byte-equal to reported candidate, `target` the checked target, `needed_for` its entry."
			}, {
				label:  "param"
				marker: "*"
				text:   "denied, reshape — split call, make target, stay in session roots; no admitted form — a decision row."
			}, {
				label:  "param"
				marker: "*"
				text:   "answer: refused call's reason opening `tool-policy: CHECK`, one `N. <target> → admitted|denied: …` line per target; that is the answer, never retry."
			}]
			needs: ["gate"]
			post: ["permission_checked"]
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_PERMISSION_CHECK check --skill fimplement --skill package-commit '<target>' …` once over whole inventory; build plan dir's `permissions.json` from its answer, never by eye."
			title:     "Permission check"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-007d"
			needs: ["gate"]
			role:      "verify"
			statement: "After check: VERIFY reason opens `tool-policy: CHECK`, each `N. <target> → …` verdict copied into permissions.json — admitted to `checked`, self-serviceable candidate to `rules`; repair: copy it."
			title:     "Verdicts copied"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-007c"
			needs: ["gate"]
			pred:      "not permission_check_available"
			role:      "stop"
			statement: "If `$SKILL_FDESIGN_PERMISSION_CHECK check` answers usage error, a `tool-policy: denied` reason or no verdict line ⇒ STOP: quote reason verbatim, write no permissions.json, never inventory by eye."
			title:     "Check unanswered"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-004"
			role:      "step"
			statement: "VERIFY API capabilities, config paths, exact field names in docs or source with file:line citation, never from memory."
			title:     "External facts"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-005"
			role:      "step"
			statement: "When designing on data shape, INSPECT real artifact (JSONL record, live response, DB row) first. Field name is hypothesis until seen."
			title:     "Real data"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-006a"
			role:      "step"
			statement: "VALIDATE design's premise against real data before building: throughput math before batch limits, profiling before performance tiers."
			title:     "Premise validation"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-008"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a fact taken from a sibling's plan is re-read from its files once they reach the branch, before the code gate."
			}]
			pred:      "role = derived"
			role:      "step"
			statement: "Derived route: RESOLVE each Depends-on sibling's source on the branch — implemented ⇒ its files, plan only ⇒ its Changes section; Baseline's base line names which."
			title:     "Sibling grounding"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GROUND-009"
			pred:      "role = derived"
			role:      "stop"
			statement: "If derived route and a Depends-on sibling has neither plan nor code on the branch ⇒ STOP: name it — merge the branch that holds it, or design that row first."
			title:     "Sibling missing"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GROUND-010"
			items: [{
				label:  "param"
				marker: "*"
				text:   "every grounding agent is asked for statements in docs, context or skill text its area's tree contradicts, and for facts those texts lack."
			}, {
				label:  "param"
				marker: "*"
				text:   "each comes as path:line, the statement quoted, the corrected text."
			}, {
				label:  "param"
				marker: "*"
				text:   "corrected in place by the plan's prose entries — never a commit apart from the plan."
			}, {
				label:  "param"
				marker: "*"
				text:   "a generated copy is regenerated, never hand-edited."
			}]
			role:      "step"
			statement: "WRITE what grounding found wrong or missing in docs and context into the plan: one Scope item, one decision row, prose entries at the code gate."
			title:     "Fold docs"
		}
	}]
}, {
	title: "Phase 2 — Decide"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-DECIDE-002"
			custom: ["todo:unmeasured"]
			role:      "rule"
			statement: "Local implementation questions (nil-guards, hashing, allocation, error wrapping) MUST be decided inline and recorded in plan's Decisions section — never asked."
			title:     "Inline decisions"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-DECIDE-003"
			custom: ["todo:unmeasured"]
			role:      "rule"
			statement: "Gate feedback is binding. An option that grows past what the gate showed MUST be recorded with its size in Consequences and stay decided — never demoted to \"deferred\", never re-asked."
			title:     "Binding feedback"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-DECIDE-004"
			custom: ["todo:unmeasured"]
			items: [{
				label:  "param"
				marker: "*"
				text:   "state-bearing designs also pass `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md`'s data-integrity entries."
			}]
			role:      "rule"
			statement: "Options MUST be weighed on three axes — controllable (user steers it), debuggable (failure locatable), reliable (degrades predictably); Decision's Why cites deciding axis."
			title:     "Decision axes"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-DECIDE-005"
			custom: ["todo:unmeasured"]
			role:      "rule"
			statement: "After rejection, next proposal MUST be structurally different; diff against rejected one before presenting."
			title:     "Structural difference"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-DECIDE-006"
			custom: ["todo:unmeasured"]
			items: [{
				label:  "param"
				marker: "*"
				text:   "machinery only gestured at — UI toggle, persisted concept, store, config field — becomes a decision row that excludes it, never built from terse answer."
			}]
			role:      "rule"
			statement: "Terse answer (\"a\", \"a and b\", one word) selects among options at that point, MUST NOT grant anything beyond them."
			title:     "Terse answers"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-DECIDE-007"
			role:      "rule"
			statement: "Planner MUST NOT split a design into several plans, or propose a split, without the `master` arg; size alone never stops a run."
			title:     "One plan default"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-DECIDE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "row shape: `D<n> | Brief | Decision (brief) | Consequences | Adds | Why | Rejected | Problem [AGENT-ONLY] | Decision [AGENT-ONLY] | Facts [AGENT-ONLY]`."
			}, {
				label:  "param"
				marker: "*"
				text:   "Adds: counts in the plan rules' vocabulary or `nothing`, then `Reuses` plus the existing call, check or helper — or `Reuses nothing` plus the reason."
			}, {
				label:  "param"
				marker: "*"
				text:   "`[USER]` marks only choices made in the request or as gate feedback; everything else is the planner's call, pre-grounded by Phase 1."
			}]
			post: ["questions_decided"]
			role:      "step"
			statement: "DECIDE every design question inline: each is a `D<n>` row in Decisions with the chosen option, the others in Rejected — never asked, never `OPEN`, never AskUserQuestion."
			title:     "Decide everything"
		}
	}]
}, {
	title: "Phase 3 — Design gate"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-WRITE-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "title `— Implementation Plan`."
			}, {
				label:  "param"
				marker: "*"
				text:   "args line carries `role: master`, or `role: derived` plus `master: <link>` — relative, no `#` fragment."
			}]
			role:      "def"
			statement: "One template serves every plan. Current state and Target state filled when the plan restructures existing code, else `N/A — <reason>`; Baseline and Exemplar & reuse always filled."
			title:     "Sections"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-009"
			items: [{
				label:  "param"
				marker: "*"
				text:   "Approach similar? Still the one presented at the gate, or a different one?"
			}, {
				label:  "param"
				marker: "*"
				text:   "Scope similar? Substantially more or less than before?"
			}, {
				label:  "param"
				marker: "*"
				text:   "Intent similar? Same purpose, same reason?"
			}, {
				label:  "param"
				marker: "*"
				text:   "after approval — main moved, code-gate gap — the same questions against the approved design."
			}, {
				label:  "param"
				marker: "*"
				text:   "a decision dropped because main removed its subject moves nothing."
			}]
			role:      "def"
			statement: "Verdict after a fold, the planner's answer to three questions — `standing` or `moved`; the answers decide, never a count of adjustments or rejections."
			title:     "Fold verdict"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-013"
			items: [{
				label:  "param"
				marker: "*"
				text:   "widget: `gate: delta`, `decisions` only the added or changed rows with `change`, `dropped` the withdrawn ones; no `tldr`, `scope`, `totals`."
			}]
			role:      "def"
			statement: "Re-gate form: widget and gate message holding only the decisions added or changed since the last gate, plus the dropped ones as one line each."
			title:     "Delta gate"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-WRITE-001b"
			role:      "rule"
			statement: "Delivered per rules/plan.md's delivery rule: plan-mode plan file, else standalone `.md`."
			title:     "Delivery format"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-WRITE-004"
			role:      "rule"
			statement: "Format MUST follow `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`: bullets not blobs, code only in fenced blocks, complete units or anchored deltas, never fragments."
			title:     "Plan format"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-WRITE-005"
			role:      "rule"
			statement: "Every plan's args line MUST record target: `<tier>` per target-tier rule in rules/plan.md — default planning session's own tier; handoff route overrides it with runner's tier."
			title:     "Target tier line"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CHAT-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "design gate: what was decided for them, what was left out and why; after the fold: the verdict line, then what the feedback changed."
			}, {
				label:  "param"
				marker: "*"
				text:   "re-gate: only the delta's decisions; never TLDR, Scope or an approved decision again."
			}, {
				label:  "param"
				marker: "*"
				text:   "code gate: which code to read, what changed since the design approval, what will surprise — never the approved design again."
			}, {
				label:  "param"
				marker: "*"
				text:   "UI capture ran: design gate message carries one bullet per captured page with its PNG link."
			}, {
				label:  "param"
				marker: "*"
				text:   "shape: title line with the plan link, then short bullets with a bold lead, one thing each; no fixed group names."
			}, {
				label:  "param"
				marker: "*"
				text:   "covers design gate, code gate, trust line and run end."
			}, {
				label:  "param"
				marker: "*"
				text:   "exempt: the mode notice, an answer to a question the user asked, the rule list of a run without a session plan file."
			}]
			role:      "rule"
			statement: "Planner MUST write every chat message for the operator at that gate: only what they need now and do not already know; a plan id only follows the words it stands for."
			title:     "Chat messages"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GATE-006"
			role:      "rule"
			statement: "The design gate MUST be presented again only on the planner's `moved` verdict and only as a [Delta gate](#skill-fdesign-gate-013); a `standing` verdict continues to the code gate."
			title:     "Gate on verdict"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-GATE-008"
			role:      "rule"
			statement: "Planner MUST NOT call EnterPlanMode or ExitPlanMode — plan mode is the session's state at invocation; the run leaves it by switching the session to acceptEdits before the widget."
			title:     "No plan-mode entry"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WRITE-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "genuinely inapplicable section gets `N/A — <reason>` — one line, consciously dismissed, never filler."
			}, {
				label:  "param"
				marker: "*"
				text:   "first write: title, args line and every heading over `<pending>`; then one edit per section — never two sections in one call."
			}]
			role:      "step"
			statement: "USE template below for the design gate — TLDR through Open questions plus Current state, Target state, Baseline (verified); no Changes entry in it. Every section required; empty = finding."
			title:     "Template use"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WRITE-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "Tests, Exemplar & reuse, Test runbook read `N/A — carried by the derived plans`."
			}, {
				label:  "param"
				marker: "*"
				text:   "the split is a decision row, `[USER]` when the `master` arg or gate feedback asked for it."
			}]
			pred:      "role = master"
			role:      "step"
			statement: "Master plan: WRITE into the design gate also Changes as the [Split table](#skill-fdesign-master-002), Contracts with each interface's exact text, Verification as end-to-end rows."
			title:     "Master sections"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-WRITE-006"
			role:      "step"
			statement: "ADD the args-line `refuted:` field the mode notice fixed: `no (plan-mode run)`, `yes` after the refute round, `yes (no premises)` when Assumptions read `N/A`."
			title:     "Refuted marker"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WRITE-009"
			items: [{
				label:  "param"
				marker: "*"
				text:   "prompt: the user's messages of the last turns verbatim ([Request](#skill-fdesign-intake-018)), the plan file path, the two questions below."
			}, {
				label:  "param"
				marker: "*"
				text:   "it first lists the requirements the messages state, then reads the Drivers table."
			}, {
				label:  "param"
				marker: "*"
				text:   "returns: requirements the table lacks, each with its quote; rows without a source in the messages."
			}, {
				label:  "param"
				marker: "*"
				text:   "once per run — Drivers changes from later gate feedback are the planner's, no second spawn."
			}]
			post: ["drivers_checked"]
			pred:      "role != derived"
			role:      "spawn"
			statement: "SPAWN one subagent through the Agent tool, model opus, in every mode, to compare the user's messages with the Drivers table."
			title:     "Driver check"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WRITE-010"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`ok`: found in the messages; `added`: suggested row taken; `no basis`: unsourced row kept."
			}, {
				label:  "param"
				marker: "*"
				text:   "`rejected — <reason>`: suggested row not taken — it stays as a row, the checker's quote as Origin."
			}, {
				label:  "param"
				marker: "*"
				text:   "derived plan: every Check cell `—`."
			}]
			role:      "step"
			statement: "MARK every Drivers row's Check cell from the driver check's return, per the Check column rule of [the plan rules]($AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md)."
			title:     "Fold check"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "coined label = a name, phrase or id first used in this run's chat or plan, not in the request, the repo or the docs."
			}, {
				label:  "param"
				marker: "*"
				text:   "applies to the plan cells themselves, so plan file, widget and gate message say the same thing."
			}]
			role:      "verify"
			statement: "Before the design gate: VERIFY every Scope item, Brief and Decision (brief) line names the user-visible change in plain words, no label coined this session; replace each with its meaning."
			title:     "Plain gate lines"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-012"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the call: `mcp__ccd_session_mgmt__set_session_permission_mode`, `session_id: \"self\"`, `mode: \"acceptEdits\"`; its result is no signal."
			}, {
				label:  "param"
				marker: "*"
				text:   "already outside plan mode, or ui-capture switched at intake: no call."
			}]
			pred:      "plan_mode and mode_switch_tool and not ui_capture_arg"
			role:      "step"
			statement: "SWITCH the session to acceptEdits after the plan is written and `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections` ran clean, directly before the widget — mechanics only, never an approval."
			title:     "Plan-mode switch"
		}
	}, {
		entry: {
			id:      "SKILL-FDESIGN-GATE-002"
			channel: "schema"
			items: [{
				label:  "Schema"
				marker: "*"
				text:   "skills/feature/fdesign/schemas/design-gate-widget.json"
			}, {
				label:  "param"
				marker: "*"
				text:   "scope item = bold title plus its first sub-bullet as `line`; decision = id plus the four cells as `{title, text}` and `adds` as `{adds, reuses}`."
			}, {
				label:  "param"
				marker: "*"
				text:   "`tldr` = each TLDR bullet: its bold title as `title`, its line as `line`."
			}, {
				label:  "param"
				marker: "*"
				text:   "`totals` = the `adds:` line `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections` prints."
			}, {
				label:  "param"
				marker: "*"
				text:   "plan mode: the switch precedes the widget; widget, gate message, turn end."
			}, {
				label:  "param"
				marker: "*"
				text:   "nightly run or no widget tool: skip; the gate is unchanged."
			}, {
				label:  "param"
				marker: "*"
				text:   "`gate`: `full` at the first gate, `delta` at a re-gate."
			}]
			role:      "step"
			statement: "SHOW the gate widget before the design gate when the session has the inline widget tool: `<base dir>/widgets/design-gate.html` with its `const DATA = {};` line filled, nothing else changed."
			title:     "Gate widget"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "nothing the run does is an approval — not the switch, not a tool result, not the widget's render; only the next user message, folded."
			}, {
				label:  "param"
				marker: "*"
				text:   "a rejection revises the design, never adds code."
			}, {
				label:  "param"
				marker: "*"
				text:   "a typed waiver of the code gate is `--trust` given late: the Trust route runs from the standing verdict."
			}, {
				label:  "param"
				marker: "*"
				text:   "UI capture ran: a revision touching a UI decision edits the live capture worktree and recaptures its pages before the plan is persisted."
			}, {
				label:  "param"
				marker: "*"
				text:   "RUN `$SKILL_FDESIGN_TITLE_SET title set DESIGN-READY` before the gate message — the session title shows the gate."
			}]
			role:      "step"
			statement: "PRESENT the design gate in chat after the widget, then END the turn; the approval is the user's next message folded to a `standing` verdict, nothing else."
			title:     "Design gate"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "waived — `you decide`, or the next message answers nothing — ⇒ the planner decides; the row stays its own call, Consequences names the comment."
			}, {
				label:  "param"
				marker: "*"
				text:   "an answer binds as `[USER]`; the fold then completes."
			}]
			role:      "stop"
			statement: "If a feedback item cannot be folded — unclear comment, rejection with no pickable replacement ⇒ STOP: one chat question per item, never a widget, never AskUserQuestion."
			title:     "Chat follow-up"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "OK and unmarked decisions stay as written."
			}, {
				label:  "param"
				marker: "*"
				text:   "adjusted decision: keep its option, amend the row as its comment says."
			}, {
				label:  "param"
				marker: "*"
				text:   "after the fold: `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>` clean, then the verdict line."
			}, {
				label:  "param"
				marker: "*"
				text:   "rejection with or without a reason — `Decision rejected: <title> (Dn)`, optionally ` — <reason>` — returns structurally different all the same."
			}, {
				label:  "param"
				marker: "*"
				text:   "delta gate feedback: decisions it did not show stay approved as before."
			}]
			role:      "step"
			statement: "FOLD the next user message — the widget's `Design gate feedback` or a typed reply, an approving reply as all OK: edits and comments bind as `[USER]`, a rejection returns structurally different."
			title:     "Widget feedback"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-010"
			items: [{
				label:  "param"
				marker: "*"
				text:   "standing: `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>` clean, then the run continues — this verdict is the approval, nothing further awaited."
			}]
			post: ["design_approved"]
			role:      "step"
			statement: "STATE the verdict first after every fold: each [Fold verdict](#skill-fdesign-gate-009) answer with its reason on one line, then `Design stands — code gate next` or `Design moved — gate again`."
			title:     "Verdict line"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-011"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a re-gate is widget, gate message and turn end; the session is already outside plan mode, no second switch."
			}, {
				label:  "param"
				marker: "*"
				text:   "RUN `$SKILL_FDESIGN_TITLE_SET title set DESIGN-READY` again before the gate message."
			}]
			role:      "stop"
			statement: "If the verdict is `moved` ⇒ STOP: SHOW the widget as a [Delta gate](#skill-fdesign-gate-013), PRESENT the gate naming only those decisions and wait; the next feedback folds the same way."
			title:     "Gate again"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-GATE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "stop: the project's stop target when it has one (e.g. `make -C <worktree> serve-dev-stop`), else `kill` the PID `lsof -ti tcp:<port>` prints."
			}, {
				label:  "param"
				marker: "*"
				text:   "run ended without approval: worktree and app stay; the next ui-capture run reuses the worktree."
			}]
			pred:      "ui_capture_arg and design_approved"
			role:      "step"
			statement: "STOP the capture app, then RUN `git worktree remove --force <scratchpad>/ui-capture`."
			title:     "Capture teardown"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WRITE-008"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the derived plan then cites the corrected text; an untouched master gets no row."
			}, {
				label:  "param"
				marker: "*"
				text:   "an interface another derived plan shares becomes a master Contracts row first, never a local contract."
			}]
			pred:      "role = derived and design_approved"
			role:      "step"
			statement: "Derived route, after design approval: EDIT the master first where a decision narrows or corrects it — cells in place, one Changelog row `derived: <slug> design gate`, its own commit."
			title:     "Master first"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WRITE-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "new plan: `plans/<today>-<slug>-<short HEAD>/<today>-<slug>-wip-<short HEAD>/`; sibling or derived plan: in that plan's folder."
			}, {
				label:  "param"
				marker: "*"
				text:   "existing plan, not implemented: edited in place — body, TLDR, args line; never a second file."
			}, {
				label:  "param"
				marker: "*"
				text:   "plan carries no permission rules — they live in plan dir's `permissions.json`, ignored by git."
			}, {
				label:  "param"
				marker: "*"
				text:   "every plan gets plan dir — rules are frozen per plan dir, an empty set included."
			}]
			post: ["plan_written"]
			pred:      "design_approved"
			role:      "step"
			statement: "After the standing verdict: WRITE the approved design to plan dir's one plan file `design/<slug>.md`, slug = plan dir's slug, and COMMIT it before any code section is written."
			title:     "Persistence"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-TPL-001"
			payload: {
				info:   "markdown"
				marker: "````"
			}
			role:      "payload"
			statement: "The plan template — one template for every route."
			title:     "Plan template"
		}
	}]
}, {
	title: "Phase 4 — Code gate"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-CODE-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "every entry: `location:` of files only, one repo-relative path per code span, then `phase: <n>`; one phase's entries touch disjoint files."
			}, {
				label:  "param"
				marker: "*"
				text:   "every entry: a `drivers:` line after `phase:` naming the Drivers rows it serves; a row needs an entry unless `no-change` or `rejected`."
			}, {
				label:  "param"
				marker: "*"
				text:   "an entry in a hot class carries `hot: <class>` after `location:`/`mirrors:`/`ui:`; its code block is the example implementation — no Hot items section."
			}, {
				label:  "param"
				marker: "*"
				text:   "document order: `hot:` code, other code, prose entries headed ` [AGENT-ONLY]` (README, skill sources, docs, context); planned-files table same order."
			}, {
				label:  "param"
				marker: "*"
				text:   "a prose entry names only `.md` files, `<leaf>.cue` and files generated from them, and carries no `hot:` line; code and prose never share an entry."
			}, {
				label:  "param"
				marker: "*"
				text:   "a UI entry shows no markup, style or script: source is a file in plan dir's `design/ui/`, linked on `ui:`, rendered at gate if widget tool exists."
			}, {
				label:  "param"
				marker: "*"
				text:   "a UI entry's `ui:` links its captured PNG and `design/ui/` source after a UI capture; without one, no `ui:` line and no capture row."
			}, {
				label:  "param"
				marker: "*"
				text:   "an entry is one agent's work package: grouped along the seam its files serve, work of minutes folded in, at most four per phase."
			}, {
				label:  "param"
				marker: "*"
				text:   "under the planned-files table the phase table `Phase | Entries | Runs | Needs`; an entry stands in the earliest phase its dependencies and the cap allow."
			}, {
				label:  "param"
				marker: "*"
				text:   "an entry changing a type, func signature or package API another entry's code compiles against stands in an earlier phase than that entry."
			}, {
				label:  "param"
				marker: "*"
				text:   "entries in one package share a phase only when neither changes what the other compiles against; disjoint files alone never make entries independent."
			}, {
				label:  "param"
				marker: "*"
				text:   "one Changes entry per write call."
			}, {
				label:  "param"
				marker: "*"
				text:   "a gap found here folds into the plan and takes a [Fold verdict](#skill-fdesign-gate-009); only `moved` re-gates."
			}]
			pred:      "design_approved"
			role:      "step"
			statement: "WRITE the code gate — Changes, Contracts, Tests, Verification, Exemplar & reuse, Test runbook — from the approved decisions only."
			title:     "Code sections"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CODE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "persisted plan path once it exists — the fimplement hook resolves a bare /fimplement from it; session plan file only before."
			}, {
				label:  "param"
				marker: "*"
				text:   "the direct form is this skill's own grant; a `$SKILL_FDESIGN_PERMISSION_CHECK check` deny of it binds the implementation run, not this one."
			}]
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>` after last plan edit; fix violations, re-phase or split till phase files are disjoint, none compiling on a sibling's change; rerun to 0."
			title:     "Section check"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CODE-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<session plan file>`: the path the plan-mode notice named; a run that never had one skips this step and lists the rules in the gate message."
			}, {
				label:  "param"
				marker: "*"
				text:   "run before a gate is presented and again after the freeze commit."
			}, {
				label:  "param"
				marker: "*"
				text:   "the Plan UI redraws only on plan-mode entry — it stays at the design gate; the linked file is the current plan."
			}]
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_PLAN_DISPLAY plan-display <plan file> <session plan file>` after every write to the persisted plan or `permissions.json` — the linked file and peek then show plan plus rules."
			title:     "Display copy"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CODE-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "chat plus the plan file, approved in chat — the code gate never runs through ExitPlanMode."
			}, {
				label:  "param"
				marker: "*"
				text:   "message links the session plan file as a clickable markdown link — the file viewer loads it from disk; no session plan file: link the persisted plan."
			}, {
				label:  "param"
				marker: "*"
				text:   "approval: a typed `/fimplement` or an explicit approving reply; any other reply is feedback — fold, present again."
			}, {
				label:  "param"
				marker: "*"
				text:   "message's last line states it: approve by typing `/fimplement`, or reply with changes."
			}, {
				label:  "param"
				marker: "*"
				text:   "nothing the run does is an approval — no tool result, no display copy; only the next user message."
			}, {
				label:  "param"
				marker: "*"
				text:   "RUN `$SKILL_FDESIGN_TITLE_SET title set CODE-READY` before the gate message — the session title shows the gate."
			}]
			role:      "step"
			statement: "PRESENT the code gate in chat, then END the turn; the approval is the user's next message and nothing else."
			title:     "Code gate"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CODE-009"
			post: ["approval_granted"]
			role:      "step"
			statement: "After the approving message: APPEND the code sections to the persisted plan — never before it."
			title:     "Approved append"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CODE-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "order: freeze, section check, commit; a nightly run leaves the commit to the runner."
			}, {
				label:  "param"
				marker: "*"
				text:   "plan needing no rule — freeze still runs; it records the empty set."
			}, {
				label:  "param"
				marker: "*"
				text:   "gate approved by a typed `/fimplement`: the hook report's `approval:` line shows the freeze — no freeze call."
			}, {
				label:  "param"
				marker: "*"
				text:   "answer: refused call's reason opening `tool-policy: FROZEN — <plan> holds N rule(s)` is success; never retry it."
			}]
			needs: ["gate"]
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_PERMISSION_FREEZE freeze <plan dir>` once the code gate is approved or, nightly or `--trust`, the code sections are written — never before."
			title:     "Freeze"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-CODE-008"
			needs: ["gate"]
			role:      "verify"
			statement: "After freeze: VERIFY reason opens `tool-policy: FROZEN`; on `tool-policy: denied …` quote it verbatim, report not frozen, tell user to type `/fdesign <plan file>` to refreeze; continue."
			title:     "Freeze refused"
		}
	}]
}, {
	title: "Phase 5 — Approval handoff"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-HANDOFF-001"
			custom: ["todo:unmeasured"]
			items: [{
				label:  "param"
				marker: "*"
				text:   "only explicit contrary statement (\"only plan\", \"don't implement\", stated handoff elsewhere) or [Fleet member approval](#skill-fdesign-handoff-003) makes approval terminal."
			}, {
				label:  "param"
				marker: "*"
				text:   "chain passes the persisted plan file as first args word — the /fimplement hook applies the frozen rules; never apply them yourself."
			}, {
				label:  "param"
				marker: "*"
				text:   "design-gate approval alone never chains — only code-gate approval, or `--trust` ([Trust substitution](#skill-fdesign-trust-001))."
			}]
			pred:      "(approval_granted and scope_in_scope) => auto_implement"
			role:      "rule"
			statement: "On code-gate approval default continuation MUST be /fimplement — immediately, same session, without asking. Never end turn waiting for go-ahead."
			title:     "Approval means implement"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-HANDOFF-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a typed `/done` at an open code gate is its approval: finish freeze and commit before the done call."
			}]
			pred:      "fleet_member"
			role:      "rule"
			statement: "In a joined fleet session code-gate approval MUST be terminal — freeze, commit, no /fimplement chain; the orchestrator's worker implements the plan."
			title:     "Fleet member approval"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-HANDOFF-002a"
			pred:      "approval_granted and not scope_in_scope"
			role:      "stop"
			statement: "If approved plan's Scope entirely postponed/design-reference ⇒ STOP: design-only, approval terminal even absent \"only plan\"; else auto-implement fires only on in-scope, non-postponed change."
			title:     "In-scope change required"
		}
	}]
}, {
	title: "Ground round route"
	blocks: [{
		entry: {
			id:      "SKILL-FDESIGN-WGROUND-001"
			channel: "script"
			items: [{
				label:  "Script"
				marker: "*"
				text:   "`workflows/ground.js`"
			}, {
				label:  "Schema"
				marker: "*"
				text:   "`schemas/fact.json`"
			}, {
				label:  "param"
				marker: "*"
				text:   "area = `{area, questions[]}`; questions carry every assumption, exemplar, reuse, external-fact, real-data, changed-thing consumer and deploy-path check."
			}, {
				label:  "param"
				marker: "*"
				text:   "`agents`, `model`, `effort` = n and the bracket of `subagents`, each omitted when absent."
			}, {
				label:  "param"
				marker: "*"
				text:   "the script sends each agent the [Ground brief](#skill-fdesign-tpl-004) and refuses more areas than `agents`."
			}]
			post: ["facts_returned"]
			pred:      "not plan_mode"
			role:      "spawn"
			statement: "SPAWN the Workflow tool, scriptPath `<base dir>/workflows/ground.js`, args `{base, areas[], agents, model, effort}`, at most n areas; await its notification."
			title:     "Ground round"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-WGROUND-003"
			items: [{
				label:  "SubagentType"
				marker: "*"
				text:   "Explore"
			}, {
				label:  "param"
				marker: "*"
				text:   "same areas and questions as the workflow round; returns the fact shape of `schemas/fact.json` as text."
			}]
			post: ["facts_returned"]
			pred:      "plan_mode"
			role:      "spawn"
			statement: "SPAWN at most n Explore agents in one message, model and effort per [Spawn bound](#skill-fdesign-ground-011), one area each, prompt the [Ground brief](#skill-fdesign-tpl-004); await every return."
			title:     "Plan-mode round"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-WGROUND-002"
			pred:      "facts_returned"
			role:      "step"
			statement: "WRITE returned facts into Baseline / Current state with their locations; FILL every gap with own reads — never a second round, never a gap agent."
			title:     "Fold facts"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-TPL-004"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "The brief every grounding agent gets, in plan mode and in ground.js alike."
			title:     "Ground brief"
		}
	}]
	kind:     "route"
	pred:     "route = new"
	replaces: "GROUND-001a + GROUND-002a + GROUND-003 + GROUND-004 + GROUND-005 + GROUND-006a"
}, {
	title:   "Refute route"
	between: "WRITE-001a..GATE-001"
	blocks: [{
		entry: {
			id:        "SKILL-FDESIGN-REFUTE-001"
			role:      "rule"
			statement: "Planner MUST confirm a debunk from its cited path:line itself before folding it."
			title:     "Bounded settlement"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-REFUTE-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "claim = `{claim_id, text, evidence, source}` — the long Reality cell, the Location cell, the Assumption cell plus the decisions whose Facts link it."
			}, {
				label:  "param"
				marker: "*"
				text:   "Assumptions `N/A` ⇒ an empty `claims` array ⇒ no refute round, args line `refuted: yes (no premises)`."
			}]
			post: ["ledger_written"]
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_PLAN_CLAIMS plan-claims <plan file>` — the claim ledger, one claim per Assumptions row, is the `claims` array of its printed document."
			title:     "Claim ledger"
		}
	}, {
		entry: {
			id:      "SKILL-FDESIGN-REFUTE-003"
			channel: "script"
			items: [{
				label:  "Script"
				marker: "*"
				text:   "`workflows/refute.js`"
			}, {
				label:  "Schema"
				marker: "*"
				text:   "`schemas/claim-verdicts.json`"
			}, {
				label:  "param"
				marker: "*"
				text:   "one round: refuter-a and refuter-b blind over the ledger, one tie-break agent over the split claims; nothing beyond the ledger is ever refuted."
			}, {
				label:  "param"
				marker: "*"
				text:   "`claims` = the printed array verbatim — a provenance check pins it; `verdictDir` = `<scratchpad>/refute`; stage procedure per [Refute procedure](#skill-fdesign-refute-006)."
			}]
			post: ["verdicts_returned"]
			pred:      "ledger_written"
			role:      "spawn"
			statement: "SPAWN the Workflow tool, scriptPath `<base dir>/workflows/refute.js`, args `{planPath, verdictDir, claims[]}`; await its notification."
			title:     "Refute round"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-REFUTE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`unverified`: settle it with a permitted probe, else redesign the dependent decision so nothing rests on it — recorded as a decision row."
			}, {
				label:  "param"
				marker: "*"
				text:   "no claim goes to an agent twice; an assumption whose rewrite the evidence does not support either is cut together with what rests on it."
			}]
			post: ["plan_refuted"]
			pred:      "verdicts_returned"
			role:      "step"
			statement: "FOLD verdicts in one pass: per `debunked` claim READ the verdict's path:line; evidence holds ⇒ rewrite both Reality cells, revise dependent decisions; else keep the row, verdict path in Location."
			title:     "Fold verdicts"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-REFUTE-006"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "The stage procedure refute.js runs per round."
			title:     "Refute procedure"
		}
	}]
	kind: "route"
	pred: "not plan_mode"
}, {
	title: "Handoff route"
	blocks: [{
		prose: """
			Renders approved plan into runner-facing artifact weaker model implements mechanically. Source plan stays untouched, stays
			review artifact; rendering derived, regenerable. Consumption path: /fimplement on rendering.
			"""
	}, {
		entry: {
			id: "SKILL-FDESIGN-HANDOFFROUTE-005"
			items: [{
				marker: "*"
				text:   "every unit's complete final code — mirrored-boilerplate bullets expanded, nothing descriptive"
			}, {
				marker: "*"
				text:   "modifications as complete final function plus replaces: path:lines line — never context-elided diffs"
			}, {
				label:  "build sequence"
				marker: "*"
				text:   "dependency-ordered units, per unit gate command and expected result"
			}, {
				marker: "*"
				text:   "naming, error strings, log wording explicit — no \"reasonable call\" surface"
			}, {
				marker: "*"
				text:   "constraints and [USER] decisions restated inside unit entry they bind"
			}]
			role:      "def"
			statement: "Content: superset of plan, judgment removed:"
			title:     "Rendering deltas"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-HANDOFFROUTE-006"
			role:      "rule"
			statement: "Rendering MUST write to plan dir's `design/handoff.md`; overwritten on re-render, never hand-edited; change to source plan invalidates it — regenerate, don't patch."
			title:     "Persistence"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-HANDOFFROUTE-007a"
			role:      "rule"
			statement: "Rendering MUST add mechanics only, never decisions: every decision cell, contract, test case of source plan survives verbatim."
			title:     "Never re-decide"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-HANDOFFROUTE-007b"
			pred:      "gap_needs_decision => not rendering_written"
			role:      "rule"
			statement: "Gap needing decision goes back to plan (an /fdesign run naming its file), not into rendering."
			title:     "Gap routing"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-HANDOFFROUTE-002a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "explicit \"handoff anyway\" overrides."
			}]
			pred:      "route = handoff and handoff_small"
			role:      "stop"
			statement: "If small (planned-files table < 3 files or < 2 buildable units; churn savings under handoff overhead) ⇒ STOP: refuse with reason, offer in-session /fimplement instead."
			title:     "Small-plan ineligibility"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-HANDOFFROUTE-002b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "weak runner fumbles these unrecoverably — model-routing fact, inverted."
			}]
			pred:      "route = handoff and handoff_high_blast"
			role:      "stop"
			statement: "If high-blast (multi-surface reductions, schema migrations, symbol renames, metric-semantics changes) ⇒ STOP: refuse with reason, offer in-session /fimplement instead."
			title:     "High-blast ineligibility"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-HANDOFFROUTE-003a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "one subagent invokes /fexplore scoped to hot items' approach, one invokes /fimpact on planned change."
			}]
			post: ["reconfirmation_done"]
			pred:      "route = handoff and hot_item_present"
			role:      "spawn"
			statement: "If any hot item present (every hot class of concepting chapter counts), before rendering SPAWN two subagents, model sonnet (default)."
			title:     "Hot-class reconfirmation"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-HANDOFFROUTE-003c"
			pred:      "reconfirmation_done"
			role:      "step"
			statement: "FOLD findings into plan (Changelog row per fold), then freeze."
			title:     "Fold and freeze"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-HANDOFFROUTE-003d"
			pred:      "findings_invalidate"
			role:      "stop"
			statement: "If findings invalidate decision ⇒ STOP: back to user."
			title:     "Invalidating findings"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-HANDOFFROUTE-004"
			post: ["rendering_written"]
			pred:      "route = handoff and not gap_needs_decision"
			role:      "step"
			statement: "WRITE rendering per handoff template below. Same-session-after-approval default flow — warm grounding; later invocation re-verifies every touch point against tree first."
			title:     "Render"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-TPL-002"
			payload: {
				info:   "markdown"
				marker: "````"
			}
			role:      "payload"
			statement: "The handoff rendering's structure."
			title:     "Handoff template"
		}
	}]
	kind:     "route"
	pred:     "route = handoff"
	replaces: "INTAKE-001..HANDOFF-002a"
}, {
	title: "Master route"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-MASTER-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`master` permits a split, never orders one — [Split threshold](#skill-fdesign-master-003) decides; unsplit, the run is an ordinary plan."
			}, {
				label:  "param"
				marker: "*"
				text:   "`master <plan file>` re-opens that design-approved plan: its design gate re-presents split, Contracts and Verification only."
			}]
			role:      "def"
			statement: "Plan with args line `role: master`: decisions, contracts between its derived plans, end-to-end checks, no code — shape per [master and derived plans]($AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md)."
			title:     "Master plan"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-002"
			role:      "def"
			statement: "Changes table `Derived plan | Carries | Decisions | Depends on` — one row per derived plan, slug `<master slug>-<part>`, file sets disjoint across rows."
			title:     "Split table"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-003"
			role:      "rule"
			statement: "A split MUST be decided only when one code gate would exceed about 4,000 plan lines, pattern-repeated edits counted once; below it the run continues as one ordinary plan."
			title:     "Split threshold"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-MASTER-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "docs, context entries, schema fields, wiring and per-skill boilerplate fold into the row whose code they serve; work of minutes is never a row."
			}, {
				label:  "param"
				marker: "*"
				text:   "the design gate shows estimated lines per row and for the unsplit plan; gate feedback on the split binds."
			}]
			role:      "rule"
			statement: "Rows MUST be the fewest that keep each code gate reviewable, each sized like a full plan of about 1,500–2,500 lines — never one row per layer, component or file kind."
			title:     "Coarse rows"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-005"
			role:      "rule"
			statement: "A master MUST have no code gate, no `permissions.json`, no freeze, no /fimplement chain; its code lives in the derived plans."
			title:     "Design only"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-006"
			role:      "rule"
			statement: "Every interface two derived plans share MUST be a `C<n>` Contracts row naming maker and readers by slug, its exact text in a fenced block under the table."
			title:     "Contract coverage"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-007"
			role:      "rule"
			statement: "A master MUST NOT be designed by a fleet member; it reaches the fleet's target branch before the fleet starts."
			title:     "Outside fleets"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-008"
			role:      "step"
			statement: "COMMIT the approved master after a clean section check and the display copy."
			title:     "Commit master"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-MASTER-009"
			role:      "step"
			statement: "PRINT the [Derived recipe](#skill-fdesign-tpl-003) filled per split row in Depends-on order, then END the run — approval is terminal."
			title:     "Print recipe"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-TPL-003"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "The next-steps block a master run ends with."
			title:     "Derived recipe"
		}
	}]
	kind:     "route"
	pred:     "role = master"
	replaces: "CODE-002 + CODE-005 + CODE-009 + CODE-007 + CODE-008 + HANDOFF-001 + HANDOFF-002a"
}, {
	title: "Derived plans"
	blocks: [{
		entry: {
			id:        "SKILL-FDESIGN-DERIVED-001"
			role:      "def"
			statement: "Plan with args line `role: derived` and `master: <link>` — one split row's code in its own plan dir in the master's feature folder, both gates; shape per [master and derived plans]($AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md)."
			title:     "Derived plan"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-DERIVED-002"
			role:      "rule"
			statement: "A derived plan MUST NOT restate the master: Assumptions read `N/A` naming the master rows, Drivers Origin links the carried `D<n>` and `C<n>`, Contracts name `C<n>` ids only."
			title:     "Restate nothing"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-DERIVED-003"
			role:      "rule"
			statement: "Decisions MUST hold only choices local to the plan's own files, opened by one line naming the carried master decisions; a master id is written `master D<n>`."
			title:     "Local decisions"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-DERIVED-004"
			role:      "rule"
			statement: "Code MUST meet the master's contract text byte for byte; a deviation is a master correction first, never a local variant."
			title:     "Contract fidelity"
		}
	}]
	kind: "context"
}, {
	title: "Trust route"
	blocks: [{
		entry: {
			id: "SKILL-FDESIGN-TRUST-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "under `--nightly` the flag has no effect — a nightly run never chains."
			}]
			role:      "def"
			statement: "`--trust` replaces the code gate's presentation and wait, nothing else; the design-gate approval is the run's one approval and covers the code sections."
			title:     "Trust substitution"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-TRUST-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "one chat line, no wait: the linked plan file plus the rules `permissions.json` carries — the user may interrupt."
			}, {
				label:  "param"
				marker: "*"
				text:   "freeze, commit and the /fimplement chain follow as after an approved code gate."
			}]
			post: ["approval_granted"]
			role:      "step"
			statement: "APPEND the code sections to the persisted plan — [Code gate](#skill-fdesign-code-005) is neither presented nor waited on."
			title:     "Ungated code"
		}
	}]
	kind:     "route"
	pred:     "trust_arg and mode != nightly"
	replaces: "CODE-005 + CODE-009"
}, {
	title:   "UI capture route"
	between: "WRITE-010..GATE-005"
	blocks: [{
		entry: {
			id:        "SKILL-FDESIGN-UICAP-001"
			role:      "def"
			statement: "A UI capture renders every UI-altering decision in the running app from a throwaway implementation, shot by `$SKILL_FDESIGN_CAPTURE_UI capture-ui`; never a mockup, never a bare source file."
			title:     "UI capture"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-UICAP-002"
			role:      "rule"
			statement: "Capture implementation MUST live only in a detached worktree under the session scratchpad, never committed, kept with its app until design approval; the session worktree gains only `design/ui/` files."
			title:     "Throwaway worktree"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-UICAP-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a worktree already at that path — earlier gate round or abandoned run: reuse it, no add."
			}]
			role:      "step"
			statement: "RUN `git worktree add --detach <scratchpad>/ui-capture HEAD`."
			title:     "Capture worktree"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-UICAP-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "data: the project's dev seed or fixtures, else live data read-only — never a write."
			}, {
				label:  "param"
				marker: "*"
				text:   "a gap found while implementing becomes a Decisions row before the gate, never an improvisation."
			}]
			role:      "step"
			statement: "IMPLEMENT every UI-altering decision in the capture worktree until the page renders it: markup, style, script, plus the handler and data plumbing the page reads."
			title:     "Implement UI"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-UICAP-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "run target: the one Command inventory grounded, e.g. `serve-dev`; none ⇒ the [Render check](#skill-fdesign-uicap-007) fallback."
			}, {
				label:  "param"
				marker: "*"
				text:   "app from an earlier round still answers on its port: skip the start; restart only when the edit needs a rebuild."
			}]
			role:      "step"
			statement: "START the app from the capture worktree in the background — `make -C <worktree> <run target>` — on a port `lsof -ti tcp:<port>` shows free."
			title:     "Start app"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-UICAP-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "one PNG per page and per state a decision changes — empty, filled, error — named `<page>[-<state>].png`."
			}]
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_CAPTURE_UI capture-ui <url|sim> <plan dir>/design/ui/<page>.png --size 1600x1200 --wait-ms 3000` per changed page."
			title:     "Capture pages"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-UICAP-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "two failed fix rounds or an app that never answers: cause as a Baseline fact, page gets no `ui:` line; continue."
			}]
			role:      "verify"
			statement: "After capture: VERIFY each PNG shows its decision's change by reading the image; fix the implementation and recapture until it does."
			title:     "Render check"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-UICAP-008"
			role:      "step"
			statement: "COPY the capture worktree's changed UI files — templates, styles, scripts — to the plan dir's `design/ui/`; handler and data plumbing are discarded."
			title:     "Keep sources"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-UICAP-009"
			role:      "step"
			statement: "EMBED each PNG in Target state under a `UI` bullet per page — `![<page>](ui/<page>.png)` plus its capture command — and COMMIT PNGs and sources with the plan."
			title:     "Embed capture"
		}
	}]
	kind: "route"
	pred: "ui_capture_arg"
}, {
	title: "Nightly route"
	blocks: [{
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-001"
			role:      "def"
			statement: "`--nightly` replaces the human gates named below, nothing else; every other entry applies unchanged."
			title:     "Nightly substitution"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-002"
			role:      "rule"
			statement: "A nightly run MUST NOT wait or enter plan mode; the morning review of the run branch is its gate."
			title:     "Never prompt"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-003"
			role:      "rule"
			statement: "A nightly run MUST NOT chain into /fimplement — [Approval means implement](#skill-fdesign-handoff-001) never fires."
			title:     "No chaining"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-004"
			role:      "step"
			statement: "TAKE the text after the args — route args, `B-NNN: <title>`, description — as the request; the plan title carries `B-NNN`."
			title:     "Dispatched request"
		}
	}, {
		entry: {
			id: "SKILL-FDESIGN-NIGHTLY-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "morning review of the run branch is the approval."
			}, {
				label:  "param"
				marker: "*"
				text:   "writes both gates in one pass; the morning review approves both."
			}]
			post: ["plan_written", "design_approved"]
			role:      "step"
			statement: "WRITE the plan per [Persistence](#skill-fdesign-write-002) without approval."
			title:     "Unapproved persistence"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-008"
			role:      "step"
			statement: "RUN `$SKILL_FDESIGN_TITLE_SET title set CODE-READY` once the persisted plan's section check exits 0 — the title shows the code gate the morning review finds."
			title:     "Code-ready title"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-006"
			role:      "step"
			statement: "WRITE `.routine-commit-body`: plan path, fired skill stops."
			title:     "Commit body"
		}
	}, {
		entry: {
			id:        "SKILL-FDESIGN-NIGHTLY-007"
			role:      "stop"
			statement: "If a stop fires ⇒ STOP: record it in `.routine-commit-body` and a plan Changelog row, persist the plan, end the run — never park."
			title:     "Halt record"
		}
	}]
	kind:     "route"
	pred:     "mode = nightly"
	replaces: "GATE-001 + CODE-005 + CODE-009 + HANDOFF-001"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "repo grounding, design decisions under hot-class gates, plan authoring at file-level precision"
			tier:   "frontier"
		}
	}]
}]
atoms: {
	plan_mode: {
		sort:   "state"
		source: "session"
	}
	role: {
		kind: "enum"
		members: ["single", "master", "derived"]
		sort:   "role"
		source: "arg: master => master | arg: derived => derived"
	}
	route: {
		kind: "enum"
		members: ["new", "handoff"]
		sort:   "route"
		source: "arg: handoff => handoff"
	}
	trust_arg: {
		sort:   "state"
		source: "arg: --trust"
	}
	typed_start: {
		sort:   "state"
		source: "session"
	}
	ui_capture_arg: {
		sort:   "state"
		source: "arg: ui-capture"
	}
}
enforce: {
	anchors: {
		claims: {
			capture: [{
				name:    "claims"
				pointer: "/claims"
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_FDESIGN_PLAN_CLAIMS plan-claims \\S+$"
			}]
			tool: "^Bash$"
		}
		mode_switch: {
			input: [{
				field:   "mode"
				pattern: "^acceptEdits$"
			}]
			tool: "^mcp__ccd_session_mgmt__set_session_permission_mode$"
		}
		persisted_sections: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_FDESIGN_PLAN_SECTIONS plan-sections \"?(\\S*/)?plans/[^/]+/[^/]+-wip-[^/]+/design/[^/]+\\.md\"?( 2>&1)?$"
			}]
			tool: "^Bash$"
		}
		persisted_write: {
			exclude: [{
				field:   "file_path"
				pattern: "/design/(exploration|handoff)\\.md$"
			}]
			input: [{
				field:   "file_path"
				pattern: "/plans/[^/]+/[^/]+-wip-[^/]+/design/[^/]+\\.md$"
			}]
			tool: "^(Edit|Write)$"
		}
		plan_write: {
			exclude: [{
				field:   "file_path"
				pattern: "/design/(exploration|handoff)\\.md$"
			}]
			input: [{
				field:   "file_path"
				pattern: "(/\\.claude/plans/[^/]+|/plans/[^/]+/[^/]+/design/[^/]+)\\.md$"
			}]
			tool: "^(Edit|Write)$"
		}
		refute: {
			input: [{
				field:   "scriptPath"
				pattern: "/fdesign/workflows/refute\\.js$"
			}]
			tool: "^Workflow$"
		}
		sections: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_FDESIGN_PLAN_SECTIONS plan-sections \\S+( 2>&1)?$"
			}]
			tool: "^Bash$"
		}
		title_set_code: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_FDESIGN_TITLE_SET title set CODE-READY( 2>&1)?$"
			}]
			tool: "^Bash$"
		}
		title_set_design: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_FDESIGN_TITLE_SET title set DESIGN-READY( 2>&1)?$"
			}]
			tool: "^Bash$"
		}
	}
	checks: {
		code_ready_after_sections: {
			after:    "persisted_sections"
			kind:     "requires"
			on:       "title_set_code"
			reason:   "fdesign: run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <persisted plan path>` after the last edit of the persisted plan before setting CODE-READY"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections plans/<feature folder>/<plan dir>/design/<plan>.md`, fix every violation, rerun to exit 0, then run `$SKILL_FDESIGN_TITLE_SET title set CODE-READY`"
			since:    "persisted_write"
		}
		design_ready_after_sections: {
			after:    "sections"
			kind:     "requires"
			on:       "title_set_design"
			reason:   "fdesign: run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>` after the last plan edit before setting DESIGN-READY"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>`, fix every violation, rerun to exit 0, then run `$SKILL_FDESIGN_TITLE_SET title set DESIGN-READY`"
			since:    "plan_write"
		}
		persisted_sections_before_stop: {
			after:    "persisted_write"
			kind:     "stop"
			reason:   "fdesign: run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <persisted plan path>` after the last edit of the persisted plan before ending the turn — the session plan file does not count"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections plans/<feature folder>/<plan dir>/design/<plan>.md`, fix every violation, rerun to exit 0, then end the turn"
			require:  "persisted_sections"
		}
		refute_after_claims: {
			after:    "claims"
			kind:     "requires"
			on:       "refute"
			reason:   "fdesign: run `$SKILL_FDESIGN_PLAN_CLAIMS plan-claims <plan file>` before the refute round — its `claims` array is the ledger"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FDESIGN_PLAN_CLAIMS plan-claims <plan file>`, then pass its `claims` array as args.claims"
		}
		refute_claims: {
			arg:      "/args/claims"
			capture:  "claims"
			from:     "claims"
			kind:     "provenance"
			on:       "refute"
			reason:   "fdesign: args.claims must be the `claims` array the last `$SKILL_FDESIGN_PLAN_CLAIMS plan-claims` printed, unedited"
			recovery: "heartbeat runtime ({finding}): pass the `claims` array of the last `$SKILL_FDESIGN_PLAN_CLAIMS plan-claims` verbatim: {evidence}"
		}
		sections_before_stop: {
			after:    "plan_write"
			kind:     "stop"
			reason:   "fdesign: run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>` after the last plan edit before ending the turn"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>`, fix every violation, rerun to exit 0, then end the turn"
			require:  "sections"
		}
		sections_before_switch: {
			after:    "sections"
			kind:     "requires"
			on:       "mode_switch"
			reason:   "fdesign: run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>` after the last plan edit before leaving plan mode"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FDESIGN_PLAN_SECTIONS plan-sections <plan file>`, fix every violation, rerun to exit 0, then switch the session to acceptEdits"
			since:    "plan_write"
		}
	}
	gate: {
		planModeDenied: ["Workflow"]
	}
}
smoke: {
	expect: {
		anchors: ["plan_write", "sections", "persisted_write", "persisted_sections", "title_set_code"]
		minChecks: 0.4
	}
	fixture: {
		tree: "tree"
	}
	invoke: "--nightly B-001: smoke-shout — plan the request in smoke-shout-request.md at the repository root"
	limits: {
		minutes: 40
		usd:     8.0
	}
}
title: {
	states: {
		"CODE-READY": {
			entry:   "SKILL-FDESIGN-CODE-005"
			meaning: "code gate presented — approve the Changes or request a change"
		}
		"DESIGN-READY": {
			entry:   "SKILL-FDESIGN-GATE-001"
			meaning: "design gate presented — approve or revise the design"
		}
	}
}
