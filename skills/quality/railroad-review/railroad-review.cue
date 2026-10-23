changelog: [{
	date:    "2026-10-17"
	version: "28.0.5"
	text:    "The expect-base-red and expect-head-red rows say the buildtest still runs as `buildtest --skip` for tracking, and the lanes output stays on stdout — smoke sessions read \"skips\" as skipping the call and redirected lanes to a file, so the provenance gate denied the round launch."
	tokens:  24433
}, {
	date:    "2026-10-17"
	version: "28.0.4"
	text:    "The smoke sets its own BUILD and TEST on a package present at both commits (`internal/cmdline`) — a smoke never reads the machine's values, and base health builds one package instead of the repo."
	tokens:  24345
}, {
	date:    "2026-10-17"
	version: "28.0.3"
	text:    "Worker prompts drop the projection-hook dirt clause: the Read hook no longer writes into governed files."
	tokens:  24345
}, {
	date:    "2026-10-17"
	version: "28.0.2"
	text:    "The smoke runs without expect-base-red: the base is built and tested, so the run exercises base health."
	tokens:  24345
}, {
	date:    "2026-10-17"
	version: "28.0.1"
	text:    "Base health runs before round 1 launches, `--skip` included, and the fan-out copies `baseHealth`, `lanePlan`, `knownPath` and `idFloor` verbatim — two smoke runs filled them from the round-args schema and hit the provenance gate."
	tokens:  24345
}, {
	date:    "2026-10-17"
	version: "28.0.0"
	text:    "BUILD and TEST are required: a review without the repo's build and test commands leaves the refuters judging by reading alone, so an invocation without them aborts."
	tokens:  24274
}, {
	date:    "2026-10-17"
	version: "27.1.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  24274
}, {
	date:    "2026-10-17"
	version: "27.0.1"
	text:    "PROC-009, ROUND-016, STATION-011b: the build, test and lint flags read optional repo-scoped variables, so they sit in their own item — an unset variable drops that item alone, never the required --repo/--root/--branch/--salvage flags."
	tokens:  24366
}, {
	date:    "2026-10-16"
	version: "27.0.0"
	changes: ["Format 3: 225 params into 9 typed fields — 40 became field values, 74 folded into statements, 111 deleted; the 11 Script and 6 Schema items are fields.script and fields.schema, six scripted substeps gained their script.", "Links by id: every skill-internal link is a [[SECTION-NNN]] token, the render derives title and anchor.", "Atom cut 65 → 14, no post: every outcome a stop reads is an enforce capture — new review_json anchor (recommendation), ledger captures autofix, oscillating, void_rounds.", "Queries: red-base-never-approve, fix-stage-needs-review, second-void-round-stops, nightly-skips-human-gate replace the three budget and lane queries."]
	tokens: 24350
}, {
	date:    "2026-10-16"
	version: "26.4.0"
	text:    "The probe-replay program is the repo's lint gate: LINT (lint@1) replaces ACDSL_CHECK."
	tokens:  30314
}, {
	date:    "2026-10-15"
	version: "26.3.0"
	text:    "The exit-2 stops point at the skill page, where the repo's build and test commands are set."
	tokens:  30321
}, {
	date:    "2026-10-15"
	version: "26.2.0"
	text:    "BUILD and TEST are optional variables again: base health and head buildtest pass `--build`/`--test`, the probe replay `--build`; exit 2 points at the Environment page."
	tokens:  30323
}, {
	date:    "2026-10-14"
	version: "26.1.0"
	text:    "Interfaces: one variable per capability — ACDSL_CHECK, BASE, BUILDTEST, CLEANUP, ENTRIES, INTAKE, LANES, LEDGER, PAIRS, PRESENT, PROBE_REPLAY, PROVISION, PRUNE, RESET_ROUND, SNAPSHOT, TITLE_SET; BUILD and TEST dropped — buildtest reads the repo registry."
	tokens:  30118
}, {
	date:    "2026-10-14"
	version: "26.0.1"
	text:    "Smoke judges only what its invocation reaches: the wip scope takes its base from HEAD, so the base anchor leaves the expected set; autofix-level none never applies a fix, so the three fix checks and no_hunks never pass and minChecks drops from 0.83 to 0.75 (12 of 16)."
	tokens:  30141
}, {
	date:    "2026-10-13"
	version: "26.0.0"
	changes: ["Env block: RULES (required, rules tool), BUILD, TEST and ACDSL (optional); every rules-tool call, grant, enforce pattern, reason and title set step runs through $SKILL_RAILROADREVIEW_RULES; Bash(make *) dropped — no step names a make target.", "Base health and head build-test take `--build`/`--test`, the probe replay `--build`/`--acdsl`, as params dropped when the install leaves them unset — the tool's discovery runs then.", "The rules tool resolves the context dir itself: the context-docs precondition and the lanes `--deployed` param are gone; a lanes exit 2 (empty diff or no context dir) stops the review with its stderr line (Lanes refused); the DoD walk reads entries through the tool only."]
	tokens: 30141
}, {
	date:    "2026-10-12"
	version: "25.0.0"
	changes: ["Blocking by severity: NIT and INFO never block and never make a round CONDITIONAL; the station returns ranked refs and station.js counts blocking (new, MINOR or worse, above ignore-level); no arg.", "Ignore level ledgered: the dedup tail exit is gone; every second-confirmed finding reaches the station and known.json, rows at or below ignore-level as fix_state ignored (ledger --ignore-level), hidden at the gate (present --ignore-level), never fixed; the funnel loses its tail bucket.", "Fix commits per package: the fixer commits through package-commit; fix-apply returns preFixHead, fixHead and fixCommits; fix check, probe replay and the patch read the range.", "Probe replay: no worktree and no build without probes; with probes the buildtest recipe builds; hits exit 0 with hits in the JSON.", "Contracts checklist carries its head: re-extracted only when the head moved, else the previous file passes as --checklist again; smine-rules lanes refuses a contracts checklist of another head.", "Report: linked locations, State column (new | carried: <fix_state>), route_reason, ignored line, refused, unverified, probes and uncovered blocks in the gate and the plan section.", "Default directions: every direction but critical, diff-pruned; critical only by directions.", "Code-style lanes receive the language rule families through the context map; a default slice keeps the dispatcher's injection as before.", "Enforce: the ledger anchors match any flag order; round_after_lanes requires a lanes call since the last cleanup."]
	tokens: 29211
}, {
	date:    "2026-10-12"
	version: "24.7.0"
	text:    "Correctness checklist once per run: new checklist arg passes a ready correctness checklist and skips its extraction, else checklist.js extracts it in round 1 only; the contracts checklist is still re-extracted every round over the changed diff; smine-rules lanes takes every checklist as --checklist <direction>=<path> each round instead of scanning the round dir. The no-hunks phase opens at smine-rules lanes, which runs every round, instead of the checklist stage, which a round without a judgment direction never launches."
}, {
	date:    "2026-10-12"
	version: "24.6.0"
	text:    "Title states are set explicitly: the config gate runs `smine-rules title set CONFIG-GATE`, the round gate `smine-rules title set HUMAN-GATE`, the lane manifests step `smine-rules title set WIP`; the title block drops the enter and leave signals — the sessiontitle hook renames at the turn end from the last set call alone; grant Bash(smine-rules title *)."
}, {
	date:    "2026-10-12"
	version: "24.5.0"
	text:    "Missing context degrades, never blocks: a lane whose injection lacks its direction's context notes MISSING CONTEXT and reviews from the direction definition, round.js returns those notes as contextGaps and the human gate relays them — no blocker finding, no halt. Explicit route: the overlay is applied uncommitted to the session checkout before round 1 (the skill-context hook reads the cwd index the explicit-route workers share), reversed around each fix-apply and each wip/all snapshot so it never enters a commit, reversed for good before the artifact prune."
}, {
	date:    "2026-10-09"
	version: "24.4.0"
	text:    "Runtime placement declared: root-only (Workflow stages), owns acceptEdits; a plan-mode session switches to acceptEdits through the Desktop session tool instead of stopping — the stop remains only without that tool."
	tokens:  27437
}, {
	date:    "2026-10-08"
	version: "24.3.0"
	text:    "Config gate: a bare invocation runs smine-rules intake --gate and stops after its report (chunk bound, exclusion candidates, model brackets, one suggested line) for go, defaults or an args line; every run runs the command once and takes its hard-class rows as the exclude list when the invocation names no exclude or paths; EXEC-002 names the gate as the second sanctioned pause; title state CONFIG-GATE; grant Bash(smine-rules intake *)."
	tokens:  27826
}, {
	date:    "2026-10-09"
	version: "24.2.4"
	text:    "Lane prompt states the lane total the lane plan holds; args.lanes and its fallback of 2 no longer read by lanes.js."
}, {
	date:    "2026-10-06"
	version: "24.2.3"
	text:    "Explicit-worktree workers: the Skill call stays the only first action — the provision command opens the pin action (second action for lanes, refuters and verifiers; first for the station) instead of running as the very first command."
}, {
	date:    "2026-10-06"
	version: "24.2.2"
	text:    "Probe verdict: run the provisioned worktree's bin/acdsl check directly in every repo; the make-target verdict form the tool policy denies is gone from the skill and the worker prompts."
}, {
	date:    "2026-10-06"
	version: "24.2.1"
	text:    "Relevance filter: the per-direction exclusion map leaves the statement — exclusions are a param pointing at smine-rules lanes (relevantFiles), the single source; the skill's copy had drifted (security keep-list missing)."
	tokens:  27257
}, {
	date:    "2026-10-04"
	version: "24.2.0"
	text:    "Requires config-server: deploy skips the skill without them."
	tokens:  27243
}, {
	date:    "2026-10-06"
	version: "24.1.2"
	text:    "Model: the sonnet class is tested unviable as lane and judgment-worker model too (golden test 2026-10-06-1f725c: recall 13.6%, review score 0, more window than opus lanes) — opus is the floor for every role."
	tokens:  27243
}, {
	date:    "2026-10-06"
	version: "24.1.1"
	text:    "allowed-tools grants Bash(git check-ignore *) — lanes and refuters probe ignore rules with it; the gate denied every call (3 in eval run 2026-10-05-1f725c-4) for want of a rule."
	tokens:  27050
}, {
	date:    "2026-10-05"
	version: "24.1.0"
	text:    "known arg: a caller hands in the known findings of an earlier run as a file; smine-rules lanes --known writes each round's known list and prints knownPath and idFloor, which the round takes as stage args under two heartbeat provenance checks; the run's own known.json holds only its own rows; finding numbers continue after the highest known number per direction; the ledger call no longer writes a round copy; a known.json found in the artifact dir is no longer taken as input."
	tokens:  27050
}, {
	date:    "2026-10-05"
	version: "24.0.1"
	text:    "Lane reports leave the artifact dir at review end; verbose keeps them beside the verdict files."
	tokens:  26960
}, {
	date:    "2026-10-05"
	version: "24.0.0"
	text:    "Stacks removed: stack detection, introduced_in attribution and down-stack routing gone, smine-rules stack replaced by smine-rules base ({commit, source}), git blame grant dropped; a red base no longer stops the review — reported, forces REJECT; verbose arg and smine-rules prune as the review's last action; dispositions carry the verdict's decisive line as evidence; a known.json present before round 1 is round 1's known ledger."
	tokens:  26950
}, {
	date:    "2026-10-04"
	version: "23.1.0"
	text:    "spec takes a doc list: every spec is copied each round as round-{r}/spec-<n>-<basename>, stages take specPaths (array) in place of specPath, the checklist stage builds one correctness checklist over all specs grouped per spec, and lanes, refuters, verifiers and the station see every spec copy."
	tokens:  27168
}, {
	date:    "2026-10-04"
	version: "23.0.1"
	text:    "Model-assignment table states the workers' real fallback: the lanes bracket, else opus, effort the agent default unless the bracket sets one; the fix-diff check row is now the fix check; the stage prelude names smine-rules as the policy home."
	tokens:  27090
}, {
	date:    "2026-10-04"
	version: "23.0.0"
	text:    "autofix-level replaces apply-level (default minor): the fix workflow fixes every new confirmed finding at or below it that is routed auto-fix, computed by smine-rules ledger; known-ledger rows carry fix_state open | applied | fixed, written at the round ledger call, after fix-apply and after the fix check (ledger --fix-state replaces --unfix); the fixer attempts every finding (failed replaces skipped); the fix check judges per finding whether the defect is gone and grants fixed; fix re-review entries removed — the next round reviews the fixes; a patch per round on every route; new args exclude and prebuilt; base health runs without the overlay."
	tokens:  27090
}, {
	date:    "2026-10-04"
	version: "22.4.5"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  27036
}, {
	date:    "2026-10-03"
	version: "22.4.4"
	text:    "Declared tier: strict — the semantic tier is the explicit top-level field, no longer implied by the atoms block."
	tokens:  27110
}, {
	date:    "2026-10-03"
	version: "22.4.3"
	text:    "Second confirmation without tooling work: acdsl check -registry overlays the repo registry, so a probe registry holds only its own entries and a reused verifier needs none; the verdict runs through make acdsl, never a build; the verdict file carries no candidate copy; lanes cite committed line numbers, never the projection block's."
	tokens:  27118
}, {
	date:    "2026-10-03"
	version: "22.4.2"
	text:    "Stage files name workflowsDir as round.js's own arg — the smoke's first round launch omitted it and threw before any agent spawned."
	tokens:  27068
}, {
	date:    "2026-10-02"
	version: "22.4.1"
	text:    "The nightly round cap is the declared `rounds` arg (default 2); the smoke runs one round."
	tokens:  27013
}, {
	date:    "2026-10-02"
	version: "22.4.0"
	text:    "expect-base-red skips the base build and test instead of tolerating a red result, new expect-head-red skips the head build and test: both run `smine-rules buildtest --skip`, which builds nothing and answers both steps skipped, so the anchors and provenance checks hold unchanged; round.js drops args.expectBaseRed and throws on every measured red base; ignore-level defaults to none (was minor); the smoke runs a correctness and a tests lane over a fixture with one defect each, both waivers set and ignore-level none."
	tokens:  26957
}, {
	date:    "2026-10-02"
	version: "22.3.0"
	text:    "Review-run defects: both buildtests run in the foreground as two Bash calls with the 600000 ms timeout, every smine-rules command in the foreground; TaskOutput and the four snapshot-recipe grants gone, smine-rules reset-round granted; the make rules forms gone; the base_health, head_buildtest, lanes and stack anchors exclude piped and compound commands; headCommit pinned once per round (Head pin), workers pin by checkout; a failed round is recorded by ledger --void or its ledger, reset by smine-rules reset-round, re-prepared at a new pin (head buildtest again when the pin moved) and retried clean (Round reset); cleanup and the ledger named in area D's final forms; the fix commit and its first parent bound every fix step; the spec is copied into the round dir; Primer hop payload for a launch after a typed message; stack, ledger and entries calls carry their required flags, --done-after passed, the ledger validates against the deployed schema by default."
	tokens:  26777
}, {
	date:    "2026-10-02"
	version: "22.2.1"
	text:    "Dropped the help-only Eval section — --help shows the smoke and eval declared beside the skill."
	tokens:  24816
}, {
	date:    "2026-10-02"
	version: "22.2.0"
	text:    "Title block: the session title reads HUMAN-GATE at the round gate, entered by the cleanup command and left when the next round's workflow starts."
	tokens:  24816
}, {
	date:    "2026-10-01"
	version: "22.1.1"
	text:    "Context gates entry says injected context file, not spill — the retired word."
}, {
	date:    "2026-09-30"
	version: "22.1.0"
	text:    "Atoms reduced to the 61 some pred reads; observers and the posts of unread atoms removed with batch conformance."
	tokens:  24816
}, {
	date:    "2026-09-29"
	version: "22.0.2"
	text:    "Fable no longer runs a subagent: station example opus:high, escalation suggestion raises lane model to opus; absent lanes/station model ⇒ default subagent model (opus); review render agent on opus; bracket model = subagent alias from the generated vocabulary."
}, {
	date:    "2026-09-29"
	version: "22.0.1"
	text:    "Merged main's 21.2.0 and 21.2.1: inner-wildcard git grants (git * apply/branch/checkout -B/diff/worktree add/worktree remove) replaced by the plain forms plus Bash(git apply *) — the gate matches git -C <dir> calls against plain rules; plan-file examples and lookup use design/<slug>.md (fdesign 5.8.0)."
}, {
	date:    "2026-09-29"
	version: "22.0.0"
	text:    "Runtime enforcement: enforce block (12 anchors; provenance of buildtest, base-health and lane-plan args into round.js; ledger before fixes and the next round; fix check and probe replay before any re-review; no dispatcher hunks; cleanup before ending; gate rules for workflow-only agent types, root-denied ledgers, checklist writers, plan mode); fix loop removed — fix-apply.js (railroad-fixer) applies the auto-fix set, routes human | auto-fix; fix check before re-review; patch range; checklist stage checklist.js; stack, snapshot, present and ledger --schema/--plan-dir/--reject replace hand procedures; finding ids unique per direction across lanes and severities; probe ids PROBE-<slug>; funnel conservation; background tasks collected before a turn ends"
}, {
	date:    "2026-09-28"
	version: "21.2.1"
	text:    "Stops trigger on positive failure atoms (base_unresolvable, snapshot_identical, head_mismatch, round_gate_pending) instead of on an outcome not yet granted, so no stop holds at run start; two_clean_rounds is a sourced round-return fact, not a coverage-ledger grant"
}, {
	date:    "2026-09-28"
	version: "21.2.0"
	text:    "Formalized every flow entry with an observer-backed post (84: 65 observable, 14 text-only, 5 unobservable), added applicability preds to the conditional flow entries, bound the lane-side premise stop, claim re-read and substeps to their spawns, and dropped the risk-map print duty"
}, {
	date:    "2026-09-28"
	version: "21.1.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "21.1.0"
	text:    "Default artifactsDir moved to ~/.cache/claude-routine/artifacts/railroad-<runNs>/artifacts — the routine-worktree container is no additional directory anymore."
}, {
	date:    "2026-09-27"
	version: "21.0.1"
	text:    "Declared Delegation: never — Workflow fan-out, root session only."
}, {
	date:    "2026-09-27"
	version: "21.0.0"
	text:    "Mechanics by program: smine-rules lanes writes lane manifests next to their artifacts (read-gate derives them, ~/.claude store gone), workers self-provision, smine-rules cleanup/ledger/probe-replay replace cleanup.js, both scripts and the hand-kept ledgers; no copy-back — station input inline, a summary return with an in-stage retry, no tail writer, evidence by reference; no hardcoded models, fix pipeline in session; base health and build-test by smine-rules buildtest; brackets read <model>:<effort>; one rr- run namespace, no eval wording in agent-facing text; prompts name the allowed form for cd, scripts, jq flags, globs and scratch paths."
}, {
	date:    "2026-09-27"
	version: "20.1.0"
	text:    "Granted every command the body, stage workflows and scripts prescribe — git -C mid-pattern forms (worktree add/remove, apply, branch, checkout -B), probe tooling (bin/acdsl check, go run, go -C build/test/vet, go tool cover), the snapshot and fix-pipeline commands, both scripts, smine-rules entries, manifest store writes and cleanup; added the eval-only overlay-patch arg keying explicit worktrees (Eval worktrees); named the probe binary form; worker prompts carry a shell-conduct block and probe scripts run through their interpreter in the registry argv."
}, {
	date:    "2026-09-27"
	version: "20.0.0"
	text:    "Nightly runs the workflow route headless — the route now replaces only the round gate, looping rounds to the cap without a per-round go, and a round that failed on an API error waits before its re-dispatch; the Agent-tool fan-out entries, brief files, dispatch ledger, batch waits, the batch-wait arg, scripts/station-input.jq and args.nightly in branchStep are gone. Tail findings at or below ignore-level join the verification pool and land in tail.json with their verdict (funnel tail counts the entries not debunked); scoped fix re-review runs correctness code-style contracts instead of critical; known-ledger rows carry effect, exposure, route and applied, and a re-review finding inside an applied fix flips its entry fixed: false; ranked_plan entries require effect and exposure."
}, {
	date:    "2026-09-26"
	version: "19.1.0"
	text:    "Nightly route declares its anchor: replaces ROUND-001..ROUND-031 — the stage-workflow spawns in Review from Fan-out through Remove run branches; the route line stays in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "19.0.0"
	text:    "Correctness lanes drop test files; tests lanes read each test with its production partners (rules pairs); refuters and verifiers get lane-parity context; stacked heads detected (rules stack), base derives to the nearest stack branch, findings carry introduced_in, down-stack fixes rejected and never auto-applied."
}, {
	date:    "2026-09-26"
	version: "18.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; stage arg ide carries the tokens, common ideBlock puts the opt-in into lane, refuter/verifier and station prompts; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-26"
	version: "18.1.4"
	text:    "Added the help-only ## Eval section: the /skillroutine-eval preset run with two dispatcher arms under the window budget; shown by --help, stripped on deploy."
}, {
	date:    "2026-09-26"
	version: "18.1.3"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "18.1.2"
	text:    "Cleanup calls ~/go/bin/smine-agentwt remove --delete-branch (the agentwt binary) instead of the retired agent-toolset remove_agent_worktrees.sh; the missing-tool hint points at make build-install."
}, {
	date:    "2026-09-25"
	version: "18.1.1"
	text:    "Make-only entry points: allowed-tools, the walk step and the worker prelude call make acdsl / make rules instead of go run ./cmd/*; the build grant narrows to the bare build target."
}, {
	date:    "2026-09-24"
	version: "18.1.0"
	text:    "Severity table v2: two axes — effect destructive|incorrect|degraded|debt|cosmetic, exposure always|often|rare|eventually — the affected cell removed from findings, refs, verdicts and prompts; missing folded into incorrect (a walk FAIL cites its item), worse split into degraded and debt; grades: destructive never below MAJOR, incorrect at often BLOCKER, degraded at often MAJOR, debt one notch below degraded; 11 rows, 20 cells, 8 fixtures"
}, {
	date:    "2026-09-24"
	version: "18.0.1"
	text:    "Re-attached EXEC-003b's dispatcher-reads param to EXEC-003b — the 18.0.0 insertion of EXEC-003d had landed between the statement and its bullet"
}, {
	date:    "2026-09-24"
	version: "18.0.0"
	text:    "Lane walk: the dispatcher writes a grouped checklist into each lane manifest (walk, built by rules walk or a checklist agent), the read-gate denies walk_complete while a pair is unanswered, lanes append findings and answers as they go and are continued by the workflow when lost or partial; every worker returns refs (id, cells, fix, paths) only — claim bodies and verdict evidence travel by file; findings cite their checklist item; lanes no longer see the risk map and the high-tier extra round is gone; new arg walk-group (default 10)"
}, {
	date:    "2026-09-24"
	version: "17.0.0"
	text:    "Severity as a vector: lanes answer effect/affected/exposure from evidence, severity is the row of schemas/severity-table.json (OUT-005/OUT-005b), recomputed at dedup entry and re-minted (ROUND-033), refuters attack the cells and may lower once (verdict.json cells), station carries cells verbatim; ignore-level default minor now exits at dedup output into round-{r}/tail.json (ROUND-034, funnel gains tail) — STATION-008b, the ranked_plan ignored flag and the station ignore block deleted; STATION-012/STATION-015 severity-check agent, its TPL-006 row and the severity-rulings.json ledger deleted (severity-fixtures.json + ACDSL-SKILL-015 replace them); the decomposed nightly path (NIGHTLY-009b) carries lane severities as returned, recorded as legacy."
}, {
	date:    "2026-09-23"
	version: "16.0.1"
	text:    "allowed-tools grants Skill(railroad-review) — lanes, refuters, verifiers and the station invoke the skill first under the tool-policy gate, which resolves the orchestrator's allowed-tools for its subagents; the lane prompt invokes the skill before the checkout verification so the skill-invoke gate and the prompt agree on the first action"
}, {
	date:    "2026-09-17"
	version: "16.0.0"
	text:    "One protocol on every launcher: apply-level is the sole auto-apply ceiling (default blocker), the fix loop runs on every route, the plan checkpoint never holds (NIGHTLY-003/003b deleted); known ledger records every confirmed finding with a fixed flag; refutation merged into verify.js (refuters + verification pool + build-test concurrent, lost-refuter retries after, refute.js deleted); station input travels as round-{r}/station-input.json (writer agent on the workflow route, scripts/station-input.jq on the nightly route); round.js orchestrator packs lanes-dedup-verify-station into one Workflow invocation per round via child workflows; salvage scripted as scripts/salvage-worktree.sh, fix-loop worktrees registered for cleanup, worker-scratch porcelain check; Model section records sonnet-5 unviable as dispatcher and station."
}, {
	date:    "2026-09-18"
	version: "15.2.3"
	text:    "Restored ACTION-CONCEPT-HOT-* and ACTION-IMPL-INTEG-* to the acdsl-context declaration — the 15.2.2 trim missed context-map.json, whose critical and data-integrity lanes select those sets (ACDSL-SKILL-006 subset gate)."
}, {
	date:    "2026-09-16"
	version: "15.2.2"
	text:    "Trimmed the acdsl-context declaration to review-relevant entries — dropped ACTION-CONCEPT-*, ACTION-IMPL-* and RULE-PLAN-* (planning/implementation doctrine, useless to the review dispatcher); the injected set is now ACTION-REVIEW-*, RULE-SKILL-012/013/014, FACT-REPO-*."
}, {
	date:    "2026-09-16"
	version: "15.2.1"
	text:    "moved the Empty diff stop after the snapshot steps — the check reads snapshot state and cannot precede the snapshot (surfaced by the new consumed-before-grant analysis)"
}, {
	date:    "2026-09-16"
	version: "15.2.0"
	text:    "workflow layer re-architected: the one-invocation-per-round monolith replaced by per-stage workflow files (lanes, dedup, refute, verify, station, cleanup) the dispatcher invokes at each spawn — schemas extracted to schemas/*.json and injected at gen time (make workflow-gen, freshness-gated), the token-conduct block and every tool-call/read limit deleted (never limit), the fix-apply policy extracted to scripts/apply-plan.sh called on both routes; script channel tags every stage-invocable spawn and substep with its workflow file, schema channels carry todo:validation-needed until output validation exists; base_bracketed renamed base_provided, base_red/base_resolvable/snapshot_differs granted by their producing checks instead of declared inputs"
}, {
	date:    "2026-09-15"
	version: "15.1.0"
	text:    "closed the condition system: producer-or-input checked atoms (sources declared for arg/session/environment inputs), renames (directions_provided, review_complete, fixes_applied), dispatch/round_started grants replaced by dispatch_prepared out of Dispatch, outcome grants on the detecting steps (second_round_failed, finding_reconfirmed, api_error_second, two_clean_rounds); nightly route thinned (granularity and rounds-loop entries deleted, round cap default 2 on Nightly routing); Groupers spawn adopts the dedup substeps and every spawn carries substeps; On-demand rendering folded into Handoff copy; quality entries speak MUST"
}, {
	date:    "2026-09-15"
	version: "15.0.0"
	text:    "section architecture re-cut along interfaces: Dispatch reduced to setup (invocation, bracket unpacking, lane set and filter bookkeeping moved out), one Review section from invocation through fan-out, station stages, handoff copy and human gate, a Fixes section gated on review.json, a Ledgers (between rounds) section carrying the rejection/known/coverage ledgers and the three end-of-round stops, and the delivery section renamed Review record; Route substitution def states the nightly route replaces the Workflow invocation alone; ref discipline applied — links point at defs, payloads and own-section flow only, cross-section sequencing left to the condition system (Route def inverted onto Fix application, Termination and Failed-round links removed)"
}, {
	date:    "2026-09-15"
	version: "14.4.0"
	text:    "post clauses over the causal chain (head_pinned, round_started, risk_map, dispatch, review_json, fix_landed, round_end), fix application gated on review_json with the fixes-need-review query; end-of-round stops moved from Global rules to the Station tail and retitled (Round gate, Budget stop); Stage order recast as the Round stages def; Nightly branch step deleted (runnability gate and Nightly routing already carry it); API-error abort wording names why Failed round catches it"
}, {
	date:    "2026-09-15"
	version: "14.3.0"
	changes: ["Moved Run namespace into Dispatch (it governs both routes) and unpacked the Fix plan loop into its stations — fix change plan, plan approval, fix implementation, fix check as flow entries"]
}, {
	date:    "2026-09-15"
	version: "14.2.0"
	changes: ["Re-roled the flow entries living in context sections as rules — lane context invocation (CTX-002a) and the probe procedure (PROBE-001/003/004/006) are standing conduct for the executing agents, not chain steps"]
}, {
	date:    "2026-09-14"
	version: "14.1.0"
	text:    "Run-namespace isolation: `runNs` is now minted once per run (nightly/eval: `eval-<run-id>`; interactive: `rr-<token>`) and required by the workflow. It scopes the manifest store to `~/.claude/read-gate-manifests/<runNs>/` (read-gate.sh now scans one subdirectory level in addition to the flat store, and prunes empty run subdirectories on its allow-path GC) and the branch prefix to `claude-review/<runNs>-` on every route, not just nightly/eval. Cleanup's unattributed check and branch deletion are scoped to this run's prefix only — a concurrent run's `claude-review/*` branches are never listed, reported, or touched — and cleanup's last step removes this run's own manifest subdirectory. Fixes concurrent railroad-review runs racing on the shared manifest store and cleanup tripping over other runs' branches."
}, {
	date:    "2026-09-14"
	version: "14.0.0"
	text:    "Format v2 completion: node-ontology/register/section-flow explainer moved to skill context (declared RULE-SKILL-012/013/014 in acdsl-context, body preamble removed); Context wiring section moved before Dispatch; new spawn role — 15 dispatcher/station spawn acts retagged (fan-out, refutation, verification pool, build-test, station barrier, cleanup, severity check, fix-diff check, on-demand rendering, nightly lane/warm-up/batched/retry spawns, workflow invocation); ROUND-004 retagged verify with Before-return anchor, MODES-003c def, OUT-008 rule, OUT-009 def; every statement rewritten into the role register (MUST/MUST NOT/NEVER rules, UPPERCASE-verb steps, If-STOP stops, anchored VERIFY, SHOULD quality) with capitalized starts."
}, {
	date:    "2026-09-14"
	version: "13.1.0"
	text:    "Split step into step (dispatcher's own transcript) and substep (a spawned subagent's own fanout transcript) — 25 of 74 step entries retagged substep: the whole Round section (lane/grouper/refuter/verifier/build-test/station work), STATION-011c/012/013 (single-purpose spawned agents), NIGHTLY-002 (the lane's own persist), PROBE-001/004/006 (the reviewing agent's own second-confirmation work), CTX-002a (the lane's own Skill invocation), OUT-008/009 (review.json content the station constructs). Everything the dispatcher itself does — scope resolution, workflow invocation, nightly orchestration, the human gate, fix application, plan-file bookkeeping — stays step. Enabler: internal/skills EntryRoles gains substep."
}, {
	date:    "2026-09-14"
	version: "13.0.0"
	text:    "Entry ontology rework — every entry is one typed node with one information point. Roles: def (vocabulary/data shape), rule (standing invariant, binary breach), step (one action), verify (test + repair, no halt), stop (condition false ⇒ halt), quality (statistical degradation), payload. The prose channel is gone from this skill; bullets are a single `* param:` register (one point each, parameterizes the parent step). Mechanism moves into the tag ([hook:read-gate], [hook:skill-context], [hook:skill-invoke-gate], [schema:review], [gate:skill-008]) — Hook:/Gate:/Schema:/Why:/Applies: pointer bullets deleted. Compound statements split into sibling entries (SCOPE-001→001+001a-c, DIR-001→001+001a-c, PROC-002/009 verify+stop, MODES-004/016b/016c/002 splits, NIGHTLY-003/006a/008a/008c/008d/008e splits, ROUND-009b/014 splits, STATION-002/004a splits). MODES-018 lost its escalate arg and opus-at/fable-at thresholds — now a no-arg, evidence-free suggestion to drop to fable on a considerable finding drop, thresholds moved to README as conjecture. Deleted: DIR-002 no-cap bullet, TPL-001 worked example, DIR-006, ROUND-013 (folded into ROUND-008), EXEC-001 and CTX-002c (duplicates) — rationale extracted to README design notes. Body re-sectioned as execution flow: definitions, global rules, dispatch, round, station & fix, nightly, probe, context, security, stacked, output, delivery. Enabler: internal/skills EntryRoles gains def/rule/verify and the custom-tag charset allows digits."
}, {
	date:    "2026-09-13"
	version: "12.0.0"
	text:    "Protocol register rework — grammar v2 with format: 2 strict caps (statement 200, bullet 160, no Why bullets), 36 Why bullets extracted to the new README.md design notes (3 constraints promoted first: dispatcher-only manifests, return-contract scope, severity twin discipline), every nightly-route entry re-topicked MODES → NIGHTLY (006a/b→001a/b, 007a→002, 008→003, 009→004, 010→005, 011a/b→006a/b, 012→007, 013a-h→008a-h, 015a/b→009a/b), the workflow script semaphore and workerCap arg deleted (the Workflow runtime queues at 16 by itself; the env cap is nightly-only), the salvage sequence extracted to a numbered procedure payload (TPL-007), DELIVER-004 compressed to its target form, and every entry necessity-pruned and caveman-compressed"
}, {
	date:    "2026-09-13"
	version: "11.1.0"
	text:    "Schema and channel truth — reference/review.schema.json with closed enums gated by ACDSL-JSON-004, the eval definition schema unstale-d (contextFreeze overlay, desktop ignore) and gated by ACDSL-JSON-005, runtime enum vocabularies in the workflow schemas, seven entries retagged to their real hook/gate enforcement channels via the new [hook] skill-entry channel, the dispositions schema todo closed, and the deliberate no-findings-cap decision documented on the falsifiability bar"
}, {
	date:    "2026-09-12"
	version: "11.0.0"
	text:    "v11 consolidation — model-assignment table (TPL-006) with sonnet-pinned mechanical agents (provision, cleanup, build-test), judgment-only station via script-pre-mapped draft dispositions and probe index, JSON-only station handoff with an escalations field and on-demand md rendering (STATION-013), build-test purpose with a red-build CONDITIONAL cap, verifier purpose statement, the fdesign-gated fix plan loop with a mandatory half-config scoped fix re-review (STATION-005c/005d), and the done-after file-clearing arg"
}, {
	date:    "2026-09-12"
	version: "10.14.0"
	text:    "Concurrency truth: the workflow clamps workerCap to the Workflow runtime's fixed 16-concurrent ceiling with a log line — CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS governs Agent-tool spawns only and stays merely the knob's source; ROUND-023 rewritten, knobs.json desktopBinding notes the clamp"
}, {
	date:    "2026-09-12"
	version: "10.13.0"
	text:    "Round-based lane-model escalation as a SUGGESTION: new escalate: <opus-at>[,<fable-at>] arg (defaults 20,10; absent = off) — after each round the dispatcher computes a monotone tier recommendation from new confirmed findings and surfaces it at the round gate; the model changes only on operator confirmation, never automatically; golden eval definitions leave the new escalateArg slot empty (MODES-018)"
}, {
	date:    "2026-09-12"
	version: "10.12.0"
	text:    "Hook-enforced lane coverage: the dispatcher writes a per-lane read-gate manifest {artifact, root, files} (MODES-003d, workflow arg manifestDir), the read-gate denies the lane's findings-artifact write until every manifest file is fully read, and its <artifact>.coverage.json sidecar becomes the measured coverage record — files_reviewed self-report removed from the lane schema, coverage_gaps narrowed to assignments of unfinished lanes, per_lane dropped; full-file coverage is the contract (no partial-read mode)"
}, {
	date:    "2026-09-12"
	version: "10.11.0"
	text:    "Judge defect equality: judgeGuide matches on same consequence + same triggering input + same violated assumption — file identity is evidence, never the criterion; golden entries and findings may carry a files array for multi-file defect chains"
}, {
	date:    "2026-09-12"
	version: "10.10.0"
	text:    "Severity anchoring: lane prompt and OUT-005 carry the full five consequence-class definitions verbatim with a tiebreak-downward rule; severity freezes at second confirmation — the station's reconcile-severities license removed; a post-station severity-check agent (STATION-012) re-derives each confirmed class and flags disagreements severity_disputed for human resolution, rulings accumulating as precedent"
}, {
	date:    "2026-09-12"
	version: "10.9.0"
	text:    "Station identity and mechanical funnel: finding ids and direction prefixes are immutable station inputs (prompt constraint + script id validation), the funnel is script-recomputed from the dispositions array overriding the station's counts, and STATION-010 gains the dispatcher-side funnel recount and id check — either mismatch re-runs the station merge"
}, {
	date:    "2026-09-09"
	version: "10.8.0"
	text:    "Entry-taxonomy pass: retagged/split entries under the two-axis role model (quality role added: DIR-002, MODES-014a/014c, ROUND-024a/b/c all quality with todo:unmeasured pending an eval-ablation consumer; ROUND-012 prose with todo:schema-needed); split MODES-013f into retry (step) and abort (stop, new 013h); cut ROUND-010's station-re-verify escape clause (contradicted ROUND-011b); reworded the code-style goal cell; extracted MODES-005d's repo-specific security-file fact to ACTION-REVIEW-SEC-001 and FACT-REPO-SEC-001, wired into the security lane"
}, {
	date:    "2026-09-11"
	version: "10.7.0"
	text:    "Interactive rounds are user-triggered: MODES-001 is now a gate — one workflow round per user go (round → human gate → fixes → fix check → report → STOP), the next round only on explicit user instruction, even after a clean round under the cap; only nightly loops rounds unattended to the cap. Termination counting unchanged. EXEC-002 and the flow diagram name the end-of-round stop"
}, {
	date:    "2026-09-11"
	version: "10.6.0"
	text:    "Fail-loud dispatch guards: the workflow throws on a laneModel/stationModel that is not a harness model token (the runtime otherwise silently falls back to the session model — every lane on the wrong model) and on an args.chunks that is present but not the computed [{name, files[]}] array (otherwise the fan-out silently collapses to one lane per direction); MODES-017 requires the dispatcher to resolve version-suffixed model aliases before dispatch and state the resolution"
}, {
	date:    "2026-09-11"
	version: "10.5.1"
	text:    "Caveman/shareability pass: session- and project-specific incident references stripped from every entry (dates, run names, review-round citations), the 10.5.0 entries tightened to register; wording only, no semantic change"
}, {
	date:    "2026-09-11"
	version: "10.5.0"
	text:    "Fix-check stage (STATION-011a/b/c): after a round's fixes land, probe replay (every persisted probe of rounds 1..r green at the post-fix head) plus a fix-diff check agent (fix diff vs ranked plan — each fix matches its Proposed fix, nothing unprescribed) run before the next round; hits route like confirmed findings interactively, append to the known ledger nightly. PROC-010 names the standalone fix re-review pattern (scope branch[<reviewed head>] + spec findings doc/change plan). Flow diagram gains the FC node"
}, {
	date:    "2026-09-11"
	version: "10.4.0"
	text:    "clear-level renamed to ignore-level (default none): sub-threshold findings are author-APPROVED — appended to the rejection ledger, never routed — instead of unrouted 'cleared' open items; new STATION-005b Full fix scope gate: every fix plan/handoff/delegate batch carries the complete confirmed set, all severities (origin: PR-6/PR-7 gates fixed MAJORs only); workflow args.clear → args.ignore with stale-arg throw, cleared flag/counters → ignored, applyPlan excludes ignored findings; eval definition moved to ignore-level: none (applyPlan output unchanged, clean counting stricter)"
}, {
	date:    "2026-09-08"
	version: "10.3.0"
	text:    "Caveman trim of over-cap entry statements (wording only, cap now 320 display runes)"
}, {
	date:    "2026-09-07"
	version: "10.2.0"
	text:    "Normalized allowed-tools order (plain tools first, grouped Bash entries sorted)"
}, {
	date:    "2026-09-07"
	version: "10.1.0"
	text:    "Explicit-worktree mode for the eval dispatcher: workflows/railroad-review.js accepts worktreeMode explicit with targetRepo, worktreeRoot and overlayPatch — a provisioning agent batch-creates detached worktrees of the target repo at the review head before each spawning stage (overlay applied uncommitted), workers spawn without harness isolation with their worktree path named in the prompt, and cleanup reverse-applies the overlay, removes the provisioned worktrees (never forced) and deletes the round claude-review/* branches. The default mode is byte-identical to before. eval-arm.js wires the full knob set through: overlay in setup, apply-level fix policy with known-ledger recording, seed knownPath, station model/effort, workerCap, per-round worker worktree roots."
}, {
	date:    "2026-09-07"
	version: "10.0.0"
	text:    "Format v2 (major): Args syntax-index table with title positional, Routing section (Purpose/Disambiguation/Preconditions/Pipeline), full example argument-hint, fenced workflow-args list, title links for every internal ref and context-doc links"
}, {
	date:    "2026-09-06"
	version: "9.13.0"
	text:    "Steering and rate-limit fault tolerance from the 2026-09-06 incident. Dispatch ledger (MODES-013e): dispatcher tracks every worker in artifactsDir/dispatch.json, state driven only by task-notifications — artifact-file existence never triggers a dispatch (duplicate dispatches into one pinned worktree overwrote a confirmed BLOCKER). API-error retries (MODES-013f): at most one retry per worker, exponential backoff on the batch wait; second failure aborts the lane. New batch-wait arg (MODES-013g, default 120s): time-based spacing between dispatch messages, wired through the eval definition as {batchWaitArg} and the routine knob ROUTINE_AB_BATCH_WAIT_S; MODES-013a/b and ROUND-023 reference it instead of back-to-back dispatch. Companion repo changes: the chat pump no longer precedes a steer with control_request interrupt (it aborted the whole in-flight turn including async lane subagents), plist subagent cap 64 -> 32."
}, {
	date:    "2026-09-06"
	version: "9.12.0"
	text:    "Process hardening from the 2026-09-06 eval runs. Fix policy: new apply-level arg (default minor) — nightly applies only confirmed findings at or below it (MODES-008); everything above accrues in the known ledger artifactsDir/known.json (STATION-004c), replayed by path to lanes/refuters/station, marked known on ranked-plan entries, never blocking (STATION-008a resets only on NEW blocking findings); applied fixes are committed per round and persisted as round-{r}/fixes.patch (STATION-005). Dispatch: ROUND-023 cap is now args.workerCap from CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS (plist raised to 64), refuters/verifiers dispatch per MODES-013a batches of 8 — never fixed completion waves; enforced by new ACDSL-SKILL-008 (forbid-phrase verifier). Contradiction fixes: run-id branch namespace implemented in the script (branchNs), MODES-005c relevance filter implemented (relevantFiles, security keep-list), nightly cleanup clause on ROUND-021 (git worktree remove, never --force, leftovers reported), MODES-013d brief root wording, MODES-007a placeholder, MODES-009 eval-override note. Coverage scoping v1 (MODES-016a/b/c): dispatcher-owned coverage.json shrinks per-direction lane lists after doneAfter clean reviews (code-style 1, tests 2, correctness 3, security 3; risk-high +1; findings/fixes reset). Named worker agent types: railroad-lane-<direction> ×8 plus railroad-grouper/-refuter/-verifier/-buildtest/-station under agents/, wired in script and eval prompt, preconditions warn when undeployed. Eval promptTemplate updated for all of the above; summary.json gains known_findings, leftover_worktrees and per-round fixes_patch"
}, {
	date:    "2026-09-06"
	version: "9.11.1"
	text:    "EXEC-003c no-dispatcher-hunks gate: the dispatcher derives diffFiles, partition bounds and the risk map from git diff --name-only/--numstat only and never writes or reads diff hunk content — hunks are exclusively lane-side (each lane diffs in its own worktree). Closes the remaining dispatcher-window leak from the 2026-09-05 overload run (~40 per-chunk hunk files read back during one round); eval promptTemplate carries the same clause."
}, {
	date:    "2026-09-06"
	version: "9.11.0"
	text:    "Context-overload fix from the 2026-09-05 21:43 eval run (main session died prompt-too-long after round 1): MODES-013d brief-file dispatch — every worker prompt body is written once to a brief file under the run scratch (briefs/round-{r}/<agent-slug>.md) and the Agent prompt carries only the invariant preamble (REQUIRE-SKILL marker, skill invocation with lane token, worktree path, brief path, return contract) plus the read-your-brief order; the dispatcher never pastes a brief body into an Agent prompt and never reads a brief back (bodies previously transited the dispatcher window 2-3x). MODES-006b reworded: verbatim binds content, delivery is the brief file — third sanctioned deviation. Eval promptTemplate carries the same brief-file dispatch clause; skilleval dispatcher cells re-enable auto-compaction via --settings (machine default stays off). Frontmatter/changelog version pair re-synced (was 9.9.2 vs 9.10.0)."
}, {
	date:    "2026-09-06"
	version: "9.10.0"
	text:    "Stage reorder: cross-direction dedup now runs BEFORE refutation (ROUND-022a) so no refuter is spent on a cross-direction duplicate — refutation becomes one global stage over the deduped union, ROUND-022b verdict carry-over and the double-refutation reconciliation are gone (verdict null now only means a lost refuter/verifier), MODES-007c/TPL-002/eval prompt updated to the new order. Worker branch step is mode-aware via branchStep()/args.nightly (MODES-011b): nightly detached worktrees get `git branch <name> HEAD` (create) instead of the interactive `git branch -m`, which fails on detached HEAD and left lane worktrees branchless and invisible to run-id cleanup"
}, {
	date:    "2026-09-05"
	version: "9.9.2"
	text:    "Declared branch-prefix: claude-review/ frontmatter (agent branch-prefix convention)"
}, {
	date:    "2026-09-05"
	version: "9.9.1"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-05"
	version: "9.9"
	text:    "Nightly worker returns slimmed to aborted flag + artifact paths + counts (second sanctioned prompt deviation beside the .md drop); dispatcher assembles all stage inputs from artifact files with jq — inline worker findings overflowed a 1M dispatcher context at the round-2 station on the 2026-09-05 eval run"
}, {
	date:    "2026-09-05"
	version: "9.8"
	text:    "Chunk defaults raised to 50 files / 4000 changed lines (fixed per-agent overhead dominates lane cost — the 2026-09-05 eval measured $2-5/lane regardless of chunk size); nightly lanes persist the JSON artifact only (the .md half dropped, station review.md stays); new nightly dispatch schedule (all lanes of a round in one message, no chunk waves) and budget conduct (budget pressure means fewer honest rounds, never thinned stages); direction relevance filter drops docs from tests/security lanes, never SKILL.md/hooks/settings"
}, {
	date:    "2026-09-05"
	version: "9.7"
	text:    "Eval prompt's pinned subagent-worktree path moved from /tmp to the persistent {wtRoot}/railroad-wt-{runId}/ — a cell's /tmp can be sandbox-isolated per tool call (observed: 64 lane worktrees vanished between the dispatcher's own Bash calls while a concurrent arm's /tmp persisted); the runner substitutes {wtRoot} and the stale sweep covers the new location"
}, {
	date:    "2026-09-05"
	version: "9.6"
	text:    "Eval prompt pins dispatcher-created subagent worktrees under /tmp/railroad-wt-{runId}/ at the head SHA and forbids the Agent tool's worktree isolation for every worker (the r2 lane-contamination fix); branches via git branch <name> HEAD per MODES-011"
}, {
	date:    "2026-09-04"
	version: "9.5"
	text:    "Eval prompt states the sandbox contract: the arm is a disposable worktree on its own claude-review/eval-* branch; never cd out of it or touch another checkout of the reviewed repository (pairs with the runner's attached-branch arm worktrees and ROUTINE_PROTECTED_PATHS guards)"
}, {
	date:    "2026-09-04"
	version: "9.4"
	text:    "Split MODES-006 into MODES-006..012 (one nightly concern each); nightly lane prompts are the Agent prompt itself, never a file pointer (the invoke gate reads REQUIRE-SKILL from the first user message); pinned detached worktrees create their branch with git branch <name> HEAD instead of the rename; ROUND-005 diffs files_reviewed against the lane's full Read-tool reads when the transcript is at hand"
}, {
	date:    "2026-09-04"
	version: "9.3"
	text:    "Token efficiency: ledger and risk map travel by path through artifactsDir (args ledgerPath/riskMapPath replace rejectedFindings/riskMap), every worker prompt carries a shared token-conduct block (batch tool calls, no mid-run tool loading, no oversized full reads), dispatcher conduct entry (preload tools, pass paths); acdsl-context narrowed FACT-* to FACT-REPO-*"
}, {
	date:    "2026-09-04"
	version: "9.2"
	text:    "MODES-006 states the nightly fan-out granularity and lane-artifact naming explicitly (one agent per direction × chunk × lane)"
}, {
	date:    "2026-09-04"
	version: "9.1"
	text:    "acdsl-context drops the commit-format entries — no review direction reviews commit messages"
}, {
	date:    "2026-09-03"
	version: "9.0"
	text:    "major: station split — cross-direction dedup moves to a grouper over the union (ROUND-022, refuter verdicts carry over, opposing verdicts re-verify), below-threshold second confirmation moves from the station to a capped pool of batched verifier agents using the refuter brief verbatim (ROUND-011), build+tests run concurrently in a dedicated build-test agent (ROUND-016), the station only consolidates; one shared semaphore caps refuter/verifier agents at REFUTE_CONCURRENCY=4 with VERIFY_BATCH=4 (ROUND-023) — script constants, not args. New clear-level intake arg (default minor): the maximum severity a confirmed finding may have and still leave the round clean — cleared findings are reported in a separate non-blocking table, routed but never fixed or counter-resetting (STATION-008, OUT-001/009); the workflow takes args.clear and returns clean/blocking/cleared computed from severities. Groupers, refuters, verifiers, and build-test now inherit the lane model (laneOpts) instead of the session model; the station and cleanup stay on the session model. Workflow return gains verifications, duplicates, buildTest, clean, blocking, cleared, clear; REFUTER_SCHEMA becomes {verdicts[]} with a script field; colliding lane finding ids get a lane suffix instead of shadowing each other in dedup"
}, {
	date:    "2026-09-03"
	version: "8.11"
	text:    "eval definition schema 1.1: severityWeights (BLOCKER 5 / MAJOR 3 / MINOR 2 / NIT 1 / INFO 1), weighted_recall metric, verdict thresholds moved to weighted_recall >= 0.6, run block (arm prompt template, tools, budget, timeout, roundCap var) and matrix defaults folded into the definition"
}, {
	date:    "2026-09-02"
	version: "8.10"
	text:    "DIR-006: naming a coverage/unread-file gap as the special-focus `focus` argument forces a round to cover those files — a user-directed coverage widening, distinct from the deterministic tests-direction coverage check"
}, {
	date:    "2026-09-02"
	version: "8.9"
	text:    "Nightly mode: dispatcher-pinned agent worktrees (never Agent-tool pool isolation, which forks from the primary checkout's branch) and a run-id arg giving each run a claude-review/eval-<run-id>- branch namespace with prefix-scoped cleanup"
}, {
	date:    "2026-09-02"
	version: "8.8"
	text:    "Station handoff-conformance gate: a round's review.json must carry a non-empty ranked_plan with direction-prefixed finding ids before acceptance"
}, {
	date:    "2026-09-01"
	version: "8.7"
	text:    "Context-entry ids removed from the body (direction-table sources now name chapters in plain language); entries declared via frontmatter acdsl-context (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "8.6"
	text:    "Spec source, review-round copies and rejection ledger addressed via the dated plan-dir convention (RULE-PLAN-072)"
}, {
	date:    "2026-09-01"
	version: "8.5"
	text:    "per-lane context injection: lanes now invoke the Skill tool with args=\"lane:<direction>\" instead of bare, and the skill-context hook injects only that direction's context slice from the new bundled context-map.json (SKILL_CONTEXT_MAP overrides the map path for A/B arms) — an unmapped direction or missing map still falls back to the full frontmatter declaration, so the change can only narrow context, never weaken it. The map is DIR-003's machine mirror (new DIR-005); read-gate now maps each lane's Skill call to its per-direction spill (railroad-review--<direction>.md). New nightly mode (nightly arg): with the Workflow tool absent but the Agent tool present, the dispatcher fans out via the Agent tool using the workflow's own prompt definitions, auto-accepts the human gate, applies fixes in its disposable worktree, and loops rounds to a cap — the headless A/B harness runs the real multi-round skill. CTX-002 and MODES-002 reworded; MODES-006 added"
}, {
	date:    "2026-08-30"
	version: "8.4"
	text:    "lane context model reversed: lanes now invoke the Skill tool BARE and receive the full doctrine injection (the v8.3 --no-context suppression is removed) — the intended model is every lane gets all the review context, delivered identically, with zero agent discretion. A new skill-invoke-gate PreToolUse hook denies every lane tool call until the skill is invoked (forcing the invocation that was previously prose-only and sometimes skipped) and denies direct reads of context-entry files after invocation (the change's language guide excepted); read-gate's blanket subagent exemption is narrowed so a subagent that invoked a skill must read that skill's injected spill in full. CTX-002 reworded to the enforced contract; the headless eval routine drops --no-context from all cells and retires the missing-required metric (nothing to miss once context arrives injected)"
}, {
	date:    "2026-08-26"
	version: "8.3"
	text:    "lane context leak closed: the lane prompt's Skill invocation now carries --no-context (the skill-context hook's per-invocation opt-out), so lanes no longer receive the dispatcher's full acdsl-context doctrine injection — CTX-002 reworded to name the real mechanism (lanes DO invoke the Skill tool; the old 'they never invoke a skill' premise was false); all skill-created branches (lanes, refuters, station) moved from claude/railroad-* to the claude-review/* namespace (lane {direction}[-{chunk}]-l{i}-r{r}-{hash6}, refute-{finding-id}-…, station-…), cleanup enumeration and namespace prose updated to match"
}, {
	date:    "2026-08-24"
	version: "8.2"
	text:    "probe artifact diet: review.json gains a required probes[] index (id, script, fixture, verdict, finding_id — refuter probes mapped in via the new REFUTER_SCHEMA fixture field) and review.md a Probes table before the funnel line (OUT-010); probes/ persists exactly scripts, fixtures/, registry.json, and the probes.acdsl mirror — verdicts.md dropped (content lives in probes[]) and the validity-check files lists are written to the system temp dir, never persisted (they were orphaned listNN.txt sidecars nothing re-read)"
}, {
	date:    "2026-08-24"
	version: "8.1"
	text:    "ROUND-017 DoD walk now driven by [DoD] entry markers instead of reviewing.md membership — the set is queried via rules entries --marker DoD (or jq over context.json .markers), covering DoD criteria that live outside reviewing.md (e.g. ACTION-IMPL-INTEG-001) and excluding non-criterion entries (ACTION-REVIEW-RISK-*)"
}, {
	date:    "2026-08-24"
	version: "8.0"
	text:    "major: intake arg surface collapsed to bracket syntax — scope absorbs base (scope: branch[develop]; explicit base supersedes derivation, unresolvable stops and reports, bracket-less falls back to derivation with an explicit chat warning); chunks absorbs maxFiles/maxLines as chunk-size bounds (chunks: layers[30:200]) and gains the languages partition mode; over-bounds now partitions automatically instead of stop-and-report — the total review cap is gone (PROC-008 gate to step); lanes absorbs laneModel/laneEffort (lanes: 2[opus-medium], bracket tokens resolved by effort vocabulary); refute renamed refute-level with the loglevel analogy spelled out; base-red: expected replaced by the boolean expect-base-red flag. Workflow interface unchanged — the dispatcher unpacks the brackets into the decomposed args (MODES-003)"
}, {
	date:    "2026-08-24"
	version: "7.4"
	text:    "directions intake arg — explicit set overrides MODES-004 dynamic selection; arg surfaces fixed: argument-hint rewritten to the real args, paths/directions added to the description, lanes count split from laneModel/laneEffort, base and spec declared in Args"
}, {
	date:    "2026-08-23"
	version: "7.3"
	text:    "evalscript renamed to verifier in prose and the workflow script"
}, {
	date:    "2026-08-23"
	version: "7.2"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-23"
	version: "7.1"
	text:    "final touches — declaration key renamed to acdsl-context (freed the native context: field); frontmatter gains allowed-tools manifest and argument-hint (author's native runtime fields context: fork/agent/background/effort consciously skipped — the dispatcher must run in-session, effort stays advisory); front/orchestrator unified to dispatcher across skill, workflow prompts, docs; bold title on every entry; When-to-use fields render as separate paragraphs; correctness-drop bolded"
}, {
	date:    "2026-08-23"
	version: "7.0"
	text:    "major: solo mode removed (never used; a single self-confirming context is the review shape the protocol replaces) — with it plan-mode/ExitPlanMode delivery, the solo reviewer-independence gate, auto-solo fallbacks, and the solo halves of intro/args/deliver; a single direction is now a valid railroad (workflow guard relaxed to >=1), plan-mode session or missing Workflow tool stops with a report (MODES-002). Full entry audit applied: cut duplicates (DIR DoD pointer, artifacts restatement, stacked-PR JSON attachment aspiration), merges (empty-snapshot rule into base derivation, front-range bridge into the invocation contract, ledger-ID footnote into the ledger gate, unverified display into the disposition entry), two calibrations moved to context (ACTION-REVIEW-TEST-005 test-mode identity constants, ACTION-REVIEW-QUALITY-008 expired temporal TODOs), remaining prose trimmed caveman-tight; 68 entries down to 54, gapless renumbering"
}, {
	date:    "2026-08-22"
	version: "6.1"
	text:    "dedup made semantic (user refutation of the file:line heuristic) — exact-match dedupeClaims replaced by a lightweight grouper agent per direction (text-only, no worktree, low effort) that groups same-defect claims; the script assembles deterministically: best-argued survivor, max group severity, merged_from provenance, unplaced claims kept; station dedup is cross-direction only; funnel produced/duplicates computed from claims_produced"
}, {
	date:    "2026-08-22"
	version: "6.0"
	text:    "major: skeleton restructure (fdesign change plan) — direction-merge agent deleted, script-side exact dedup replaces it, station is the single semantic consolidator (one serial hop and one agent per direction saved); refute becomes a loglevel severity threshold (blocker|major|minor|nit|info|none, default major); directions reduced to goal sentence + context requirement by entry family (checklists deleted, concept audit and generated-files checks moved to ACTION-REVIEW-SPEC-005 / QUALITY-007); Required Reading dissolved into a generic Context section; every wall entry split to one point per identifier (full renumbering, probe protocol its own topic); args gain worked examples; mermaid flow updated to the real gates and stages"
}, {
	date:    "2026-08-22"
	version: "5.2"
	text:    "second integration round from the author-skill critique — per-candidate refutation stage between direction merge and station (fresh refuter per BLOCKER/MAJOR candidate by default, refute arg all|blk-maj|station, verdicts binding at the station); experimental chunked fan-out (chunks arg: derived layers or explicit groups, chunk-scoped lanes, contracts chunk-exempt); size cap maxFiles 30 / maxLines 2000 over reviewed code only with chunked-run or layer-run split proposal (PROC-008); base-health gate before round 1, waivable via base-red: expected (PROC-009); risk-map taxonomy moved to ACTION-REVIEW-RISK-* context entries with in-skill fallback; falsifiability bar with worked example (TPL-005); no spec => correctness direction dropped instead of degraded; recommendation glosses"
}, {
	date:    "2026-08-21"
	version: "5.1"
	text:    "never-prompt gate (EXEC-005) — the review never calls AskUserQuestion; base derived autonomously (given, else tracked upstream, else mainline, stated), assumptions stated and carried, unresolvable inputs and empty diffs end with a report; the only pause is the station-protocol human gate"
}, {
	date:    "2026-08-21"
	version: "5.0"
	text:    "major: formal merge of the author's work-review-wip SKILL.md — dynamic scope arg (branch|wip|all + paths, dynamic default) via snapshot commits so uncommitted and untracked work reviews through the unchanged worktree machinery; 5-level consequence-based severity (BLOCKER/MAJOR/MINOR/NIT/INFO, WARNING retired) with <direction>-<SEV>-<n> ids; per-finding fix route (human|auto-fix|runner-fix) consumed by fix application; unverified disposition kept with concrete missing precondition; adversarial refutation framing with both-directions test checks on the manual path; risk map (PROC-007) driving attention and diff-proportional direction selection with auto-added security; wrong-base heuristic in the guard; station runs build+tests once, names permanent checks for recurring classes, escalates undemonstrated high-risk defects to the top, and ends review.md with the claim funnel; solo reviewer-independence gate (EXEC-004); no-spec degradation for correctness; When-to-use widened — THE review skill, /code-review boundary removed"
}, {
	date:    "2026-08-20"
	version: "4.2"
	text:    "description rewritten to repo format under the new brevity caps (sentence <=160, total <=450 chars)"
}, {
	date:    "2026-08-20"
	version: "4.1"
	text:    "claim semantics (author feedback) — lane findings are claims with a mandatory confirmation pass in a required evidence field (first confirmation, still in question); station re-verifies every claim regardless of severity (second confirmation), deterministically in ACDSL repos via per-defect task-lifetime probe rules (>=2 authoring attempts, fail-fixture validity check, check -rule verdict); only twice-confirmed claims enter the review body, refuted claims are debunked and removed; probe artifacts persist with the round"
}, {
	date:    "2026-08-19"
	version: "4.0"
	text:    "major: body migrated to entries via /fmt skill — 62 entries (DIR 6, MODES 4, ROUND 5, EXEC 3, READ 4, PROC 6, STATION 7, SEC 4, STACK 3, OUT 10, DELIVER 6, TPL 4 payloads), content unchanged; Model effort normalized large→high"
}, {
	date:    "2026-08-12"
	version: "3.11"
	text:    "migrated to the context: declaration — orchestrator doctrine injected at invocation; lanes read per-direction files (no SubagentStart doctrine injection anymore); stale rules/-vs-actions/ paths and ALWAYS-* entry names corrected"
}, {
	date:    "2026-08-11"
	version: "3.10"
	text:    "the station review persists to the repo — the front copies round-{r}/review.{json,md} from the scratchpad into plans/{slug}/reviews/round-{r}/ after every round (workflow script unchanged)"
}, {
	date:    "2026-08-10"
	version: "3.9"
	text:    "station protocol — fixes applied on the review branch by default (orchestrator inline or same-worktree runner), cross-branch merge-back handoff now opt-in; mermaid, Modes, fan-out, exec-requirements, description reworded"
}, {
	date:    "2026-08-10"
	version: "3.8"
	text:    "lane safety — reworded the pristine instruction (projection-hook dirt is expected) and forbade tree-mutating git commands on the lane, direction-merge, and station-merge prompts"
}, {
	date:    "2026-08-09"
	version: "3.7"
	text:    "language-agnosticism sweep — concurrency-primitive phrasing in critical fallback list and concept audit, generated-file excludes per repo convention"
}, {
	date:    "2026-08-09"
	version: "3.6"
	text:    "identity-grammar citation sweep (ACTION-IMPL-INTEG family)"
}, {
	date:    "2026-08-09"
	version: "3.5"
	text:    "ACDSL context wording — SessionStart delivery, gate output as review evidence; description gains Trigger on clause (ACDSL-SKILL-002 gate)"
}, {
	date:    "2026-07-31"
	version: "3.4"
	text:    "diff range self-resolved — baseCommit pinned to the merge-base fork point (never the base tip); lane premise aborts explicit (required aborted flag), aborted lanes excluded from merges, fully-aborted rounds return abortedLanes loudly; front re-resolves and re-dispatches once on a premise-failed round"
}, {
	date:    "2026-07-31"
	version: "3.3"
	text:    "critical direction reads the hot-class gates from rules/concepting.md"
}, {
	date:    "2026-07-31"
	version: "3.2"
	text:    "activity-scoped context — hot classes and integrity entries in rules/implementing.md, DoD walk via rules/reviewing.md, Go style at style/go.md"
}, {
	date:    "2026-07-30"
	version: "3.1"
	text:    "context redesign — hot-items via ../fdesign/assets/ + rules/ overlay; data-integrity direction reads rules/integrity.md and cites entry IDs"
}, {
	date:    "2026-07-30"
	version: "3.0"
	text:    "station protocol: tracks renamed to directions, n lanes per direction (default 2) with per-direction merge, deterministic diff-coverage self-verification per lane, every artifact a JSON+MD pair, human review gate with durable rejection ledger (plans/{slug}/reviews/rejected.json), mandatory fix handoff to a different agent/session, clean-slate rounds until two consecutive clean rounds; v2 hand-run protocol and approval styles deleted"
}, {
	date:    "2026-07-30"
	version: "2.7"
	text:    "moved under skills/quality/ group; reference rename per-package-commit → package-commit"
}, {
	date:    "2026-07-26"
	version: "2.6"
	text:    "findings render as a table with a mandatory Proposed-fix column (was a bare severity line); Stage A TRACK_SCHEMA gains a required fix per finding, and the merge stage carries/refines each track's fix into the ranked plan instead of inventing one"
}, {
	date:    "2026-07-24"
	version: "2.5"
	text:    "railroad default mode (solo via inline arg, auto-solo fallbacks); artifacts to args.artifactsDir so worktrees stay removable; Cleanup stage invokes the agent-toolset remove_agent_worktrees.sh per claude/railroad-* branch, safety-gated, never forced"
}, {
	date:    "2026-07-20"
	version: "2.4"
	text:    "merge agent runs in an isolated worktree too, branch renamed to claude/railroad-merge-{base} — same leftover visibility as the track agents"
}, {
	date:    "2026-07-20"
	version: "2.3"
	text:    "workflow guard fixed to the head tip (worktrees fork from the orchestrator checkout, not the base) — args gains headCommit, agents verify HEAD == headCommit + baseCommit ancestry; string args tolerated (harness delivers JSON strings on resume); optional trackModel/trackEffort passthrough"
}, {
	date:    "2026-07-19"
	version: "2.2"
	text:    "track agents rename their worktree branch to claude/railroad-{track}-{base} so leftovers are visible to claude/* tooling"
}, {
	date:    "2026-07-19"
	version: "2.1"
	text:    "railroad mode fronts the bundled workflow (workflows/railroad-review.js, invoke via scriptPath as analyze does) — Stage A track fan-out (review-only, isolated worktrees) + Stage B/C merge barrier with per-finding dispositions, current-tree re-verification, and ranked fix plan"
}, {
	date:    "2026-07-16"
	version: "2.0"
	text:    "tracks, solo/railroad modes, multi-agent merge protocol, hot-items critical track"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-06-22"
	version: "1.0"
	text:    "initial version"
}]
enableDelegation: "never"
enforcePermissions: ["acceptEdits"]
env: {
	BASE: {
		interface: "base@1"
		required:  true
	}
	BUILD: {
		interface: "build@1"
		required:  true
	}
	BUILDTEST: {
		interface: "buildtest@1"
		required:  true
	}
	CLEANUP: {
		interface: "cleanup@1"
		required:  true
	}
	ENTRIES: {
		interface: "entries@1"
		required:  true
	}
	INTAKE: {
		interface: "intake@1"
		required:  true
	}
	LANES: {
		interface: "lanes@1"
		required:  true
	}
	LEDGER: {
		interface: "ledger@1"
		required:  true
	}
	LINT: {
		interface: "lint@1"
	}
	PAIRS: {
		interface: "pairs@1"
		required:  true
	}
	PRESENT: {
		interface: "present@1"
		required:  true
	}
	PROBE_REPLAY: {
		interface: "probe-replay@1"
		required:  true
	}
	PROVISION: {
		interface: "provision@1"
		required:  true
	}
	PRUNE: {
		interface: "prune@1"
		required:  true
	}
	RESET_ROUND: {
		interface: "reset-round@1"
		required:  true
	}
	SNAPSHOT: {
		interface: "snapshot@1"
		required:  true
	}
	TEST: {
		interface: "test@1"
		required:  true
	}
	TITLE_SET: {
		interface: "title-set@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " railroad-review"
}, {
	key:   "description"
	value: " Review code changes against conventions, specs and DoD — parallel directions, one station review; a bare call analyzes the config first. Trigger on /railroad-review or \"review this branch/PR/my changes\". Args — title; directions; lanes; station; scope; paths; spec; checklist; focus; refute/ignore/autofix-level; done-after; chunks; walk-group; expect-base|head-red; nightly; rounds; run-id; overlay-patch; exclude; prebuilt; known; verbose."
}, {
	key:   "author"
	value: " Andreas Geßner"
}, {
	key:   "version"
	value: " 28.0.5"
}, {
	key:   "title-code"
	value: " RR"
}, {
	key:   "format"
	value: " 3"
}, {
	key:   "acdsl-context"
	value: " ACTION-CONCEPT-HOT-*, ACTION-IMPL-INTEG-*, ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014, RULE-GOLANG-*, RULE-PYTHON-*, RULE-RUST-*, RULE-SQL-*, FACT-REPO-*"
}, {
	key:   "allowed-tools"
	value: " Edit, Read, Skill(package-commit), Skill(railroad-review), ToolSearch, Workflow, Write, Bash($SKILL_RAILROADREVIEW_BASE base *), Bash($SKILL_RAILROADREVIEW_BUILDTEST buildtest *), Bash($SKILL_RAILROADREVIEW_CLEANUP cleanup *), Bash($SKILL_RAILROADREVIEW_ENTRIES entries *), Bash($SKILL_RAILROADREVIEW_INTAKE intake *), Bash($SKILL_RAILROADREVIEW_LANES lanes *), Bash($SKILL_RAILROADREVIEW_LEDGER ledger *), Bash($SKILL_RAILROADREVIEW_PAIRS pairs *), Bash($SKILL_RAILROADREVIEW_PRESENT present *), Bash($SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay *), Bash($SKILL_RAILROADREVIEW_PROVISION provision *), Bash($SKILL_RAILROADREVIEW_PRUNE prune *), Bash($SKILL_RAILROADREVIEW_RESET_ROUND reset-round *), Bash($SKILL_RAILROADREVIEW_SNAPSHOT snapshot *), Bash($SKILL_RAILROADREVIEW_TITLE_SET title *),Bash(*/bin/acdsl check *), Bash(*/bin/verifiers/*), Bash(./bin/acdsl check *), Bash(bin/acdsl check *), Bash(cp *), Bash(git add*), Bash(git apply *), Bash(git branch*), Bash(git check-ignore *), Bash(git checkout -B *), Bash(git diff*), Bash(git format-patch *), Bash(git init *), Bash(git log*), Bash(git merge-base*), Bash(git rev-list*), Bash(git rev-parse*), Bash(git show *), Bash(git status*), Bash(git worktree add*), Bash(git worktree list*), Bash(git worktree remove*), Bash(go * build *), Bash(go * test *), Bash(go * vet *), Bash(go run *), Bash(go tool cover *), Bash(jq *), Bash(mkdir *), Bash(mv *)"
}, {
	key:   "argument-hint"
	value: " 'scheduler rewrite review directions: tests security contracts lanes: 2[opus:medium] station: opus:high scope: branch[main] paths: internal/server cmd/*.go exclude: plans/ sessions/ spec: plans/2026-09-07-foo-wip-abc1234/design/foo.md checklist: plans/2026-09-07-foo-wip-abc1234/design/checklist-correctness.json focus: \"goroutine lifecycle in the scheduler\" refute-level: major ignore-level: none autofix-level: minor done-after: 2 chunks: layers[50:400] walk-group: 10 expect-base-red expect-head-red nightly rounds: 2 run-id: 2026-09-27-a34281-a overlay-patch: overlays/review-context.patch prebuilt: bin/acdsl bin/verifiers known: reviews/known.json verbose'"
}, {
	key:   "branch-prefix"
	value: " claude-review/"
}]
preamble: """
	# Railroad Review

	Structural review against project's standards, specs, and plans. Directions — independent review lenses — fan out to parallel
	lanes through station protocol. Session running this skill is **dispatcher**: schedules rounds, holds human gate, applies
	fixes; never reviews lane itself. Review standards live in project's context entries; design rationale lives in this skill's
	README.md.
	"""
requires: ["config-server"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "dod-report"
					optional: true
				}, {
					kind:  "to"
					skill: "package-commit"
				}]
				standalone: true
			}
			preconditions: ["non-plan-mode session with the Workflow tool, headless `claude -p` included ([[MODES-002]])", "branch/all scope: resolvable diff base", "correctness direction: spec, plan, request doc or `checklist` — **dropped** per [[DIR-004]] without one"]
			purpose: "reviews any code change — PRs, branches, commits, uncommitted work, untracked files — against project conventions, specs, style guides, definition-of-done."
			routes: [{
				for: "verdict on existing, unchanged code"
				use: "/code-verdict"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "title"
				default: "generic session name"
				effect:  "names the run — chat title and initial branch derive from it"
				form:    "free text, first"
			}, {
				name:    "directions"
				default: "all but critical, diff-pruned"
				effect:  "binding direction-set override — [[MODES-004]]"
				form:    "space list"
			}, {
				name:    "lanes"
				default: "`2`, default subagent model (`opus`)"
				effect:  "lanes per direction; bracket governs judgment workers — [[MODES-017]]"
				form:    "`<n>[<model>:<effort>]`"
			}, {
				name:    "station"
				default: "`opus`, high"
				effect:  "station-merge model override — [[MODES-017]]"
				form:    "`<model>[:<effort>]`"
			}, {
				name:    "scope"
				default: "`branch` clean tree, `all` dirty"
				effect:  "what to review — [[SCOPE-001]]"
				form:    "`branch[<base>]` \\| `wip` \\| `all[<base>]`"
			}, {
				name:    "paths"
				default: "whole diff"
				effect:  "narrows any scope to named files/dirs — [[PROC-004]]"
				form:    "pathspec list"
			}, {
				name:    "exclude"
				default: "absent"
				effect:  "pathspecs left out of every review diff — [[PROC-004]]"
				form:    "pathspec list"
			}, {
				name:    "spec"
				default: "[[DIR-003]] chain"
				effect:  "correctness spec docs, one checklist over all; none at all drops direction — [[DIR-004]]"
				form:    "doc path list"
			}, {
				name:    "checklist"
				default: "extracted from `spec` in round 1"
				effect:  "ready correctness checklist, reused every round; no extraction — [[WALK-004]]"
				form:    "group-first JSON path"
			}, {
				name:    "focus"
				default: "direction off"
				effect:  "special-focus subject — [[DIR-003]]"
				form:    "quoted subject"
			}, {
				name:    "refute-level"
				default: "`major`"
				effect:  "minimum severity getting a fresh refuter — [[ROUND-007]]"
				form:    "`blocker\\|major\\|minor\\|nit\\|info\\|none`"
			}, {
				name:    "ignore-level"
				default: "`none`"
				effect:  "severity at/below is ledgered `ignored` after the gate — hidden there, never fixed, never blocking — [[STATION-004c]]"
				form:    "same vocabulary"
			}, {
				name:    "autofix-level"
				default: "`minor`"
				effect:  "highest severity the fix workflow fixes; human-routed findings stay open — [[STATION-005b]]"
				form:    "same vocabulary"
			}, {
				name:    "done-after"
				default: "per-direction map"
				effect:  "uniform consecutive-clean count clearing a file for every direction — [[MODES-016b]]"
				form:    "`<n>`"
			}, {
				name:    "chunks"
				default: "auto over bounds"
				effect:  "partitioned fan-out — [[MODES-005a]]"
				form:    "`<mode>[maxFiles:maxLines]`"
			}, {
				name:    "walk-group"
				default: "`10`"
				effect:  "checklist chunk size — items per checklist group, passed as `--ceiling` — [[MODES-003b]]"
				form:    "`<n>`"
			}, {
				name:    "expect-base-red"
				default: "absent"
				effect:  "base health still runs, as `buildtest --skip` for tracking: nothing is built or tested — [[PROC-009]]"
				form:    "flag"
			}, {
				name:    "expect-head-red"
				default: "absent"
				effect:  "head buildtest still runs each round, as `buildtest --skip` for tracking: nothing built or tested; nothing caps the recommendation — [[ROUND-016]]"
				form:    "flag"
			}, {
				name:    "nightly"
				default: "absent"
				effect:  "unattended headless — rounds loop to the cap, no per-round go — [[NIGHTLY-001a]]"
				form:    "flag"
			}, {
				name:    "rounds"
				default: "`2`"
				effect:  "nightly round cap — [[NIGHTLY-001a]]"
				form:    "`<n>`"
			}, {
				name:    "run-id"
				default: "`rr-<6 hex>`"
				effect:  "seeds this run's `runNs` — [[NIGHTLY-007]]"
				form:    "token"
			}, {
				name:    "overlay-patch"
				default: "absent"
				effect:  "explicit worktrees with this review-context overlay applied — [[MODES-019]]"
				form:    "path"
			}, {
				name:    "prebuilt"
				default: "absent"
				effect:  "repo-relative built paths linked into every worker worktree — [[MODES-019]]"
				form:    "path list"
			}, {
				name:    "known"
				default: "absent"
				effect:  "known findings of an earlier run: never re-reported, never written — [[MODES-003d]]"
				form:    "path"
			}, {
				name:    "verbose"
				default: "absent"
				effect:  "keeps the lane reports and per-finding verdict files at review end — [[ROUND-036]]"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-001"
			role:      "def"
			statement: "One review lens."
			title:     "Direction"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-001a"
			role:      "def"
			statement: "One agent, one direction, one chunk."
			title:     "Lane"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-001b"
			role:      "def"
			statement: "Session running this skill; never reviews a lane."
			title:     "Dispatcher"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-001c"
			role:      "def"
			statement: "Every direction but critical and special-focus; each fans to `lanes` lanes."
			title:     "Default directions"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-SCOPE-001a"
			role:      "def"
			statement: "Committed range, fork point to the round's pinned head."
			title:     "Branch scope"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-SCOPE-001b"
			role:      "def"
			statement: "Staged + unstaged + untracked snapshot, base = HEAD."
			title:     "Wip scope"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-SCOPE-001c"
			role:      "def"
			statement: "Fork point to working tree incl. untracked, snapshot."
			title:     "All scope"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-008a"
			role:      "def"
			statement: "50 files / 4000 changed lines, counted after scope exclusions — the intake command's constants; `chunks` bracket overrides; no total review cap."
			title:     "Partition bounds"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-INTAKE-001"
			role:      "def"
			statement: "A `/railroad-review` prompt carrying no args, until the config gate's reply is folded."
			title:     "Bare invocation"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-017"
			role:      "def"
			statement: "`lanes`/`station` tokens: `<model>:<effort>`, either side optional; effort ∈ `low | medium | high | xhigh | max`; absent model ⇒ default subagent model (`opus`), absent effort ⇒ session."
			title:     "Bracket vocabulary"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-003"
			role:      "def"
			statement: "Each direction = goal + context requirement — standards themselves are context, never restated here:"
			title:     "Direction table"
		}
	}, {
		prose: """
			| Direction | Goal | Context |
			| :--- | :--- | :--- |
			| `code-style` | Naming, idioms, structure match surrounding code. | [reviewing chapter's quality entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/reviewing.md); repo's style rules for change's language |
			| `correctness` | Production code does exactly what spec requires — nothing missing, nothing nobody asked for; test files excluded. | spec/plan/request: invocation-named, else plan dir's plan file `design/<slug>.md` (resolved per [plan-dir resolution rule]($AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md)); [reviewing chapter's spec entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/reviewing.md) |
			| `critical` | No high-risk defect class ships. | [concepting chapter's hot-class entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/concepting.md) (fallback one-liner: concurrency misuse, nil derefs, weakened guards, resource leaks) |
			| `special-focus` | Whatever focus argument names. | `focus` arg |
			| `data-integrity` | State-bearing changes keep integrity doctrine. | [implementing chapter's data-integrity entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md) (fallback one-liner: append-only migrations, no timer-driven transitions, claim-before-send, exhaustive enums) |
			| `contracts` | Cross-boundary contract change lands on every side — no leftover old names anywhere. | repo-wide search, all languages; client backward-compatibility DoD entry |
			| `tests` | Changed code covered by on-pattern tests; spec-named tests present. | [reviewing chapter's test entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/reviewing.md); each test with its production partners; spec |
			| `security` | Only quantified, reproducible exploits. | [reviewing chapter's security entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/reviewing.md) |
			"""
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-005"
			role:      "def"
			statement: "`context-map.json` in this skill's directory is injector's source per direction; Directions table is its human twin — a change to one is a change to both."
			title:     "Machine mirror"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-WALK-001"
			role:      "def"
			statement: "A lane's checklist: the manifest's `walk` object — `shape` plus `groups` of at most `walk-group` items; the lane answers one group at a time and records each answer as a `walk` entry in its artifact."
			title:     "Walk"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-WALK-002"
			role:      "def"
			statement: "`file-first` (code-style, tests, security, data-integrity, critical): one answer per assigned file × group. `group-first` (correctness, contracts): one answer per group over the whole diff."
			title:     "Walk shape"
		}
	}]
	kind: "context"
}, {
	title: "Global rules"
	blocks: [{
		entry: {
			id: "SKILL-RAILROADREVIEW-EXEC-002"
			custom: ["todo:hook-needed"]
			role:      "rule"
			statement: "Review NEVER calls AskUserQuestion nor stops for input; resolve every input, state assumption, carry on; genuinely unresolvable ends the review with a report."
			title:     "Never prompt"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-EXEC-003a"
			role:      "rule"
			statement: "Dispatcher MUST load every tool railroad needs before round 1."
			title:     "Preloaded tools"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-EXEC-003b"
			role:      "rule"
			statement: "Station handoffs MUST be consumed via targeted jq over `review.json`; ledger and risk map MUST travel as files."
			title:     "Paths, never content"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-EXEC-003d"
			role:      "rule"
			statement: "Every worker MUST return ids, cells, counts and paths — claim bodies, verdict evidence and write-ups travel by file, read with jq."
			title:     "Workers return refs"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-EXEC-003c"
			role:      "rule"
			statement: "Dispatcher NEVER reads or writes diff hunk content; hunks stay exclusively lane-side."
			title:     "No dispatcher hunks"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-EXEC-003e"
			role:      "rule"
			statement: "Agents NEVER re-emit data another component holds — a file is written by the program that computes it or the agent that authors it; Workflow args carry refs and small lists only."
			title:     "No copy-back"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-EXEC-004"
			role:      "rule"
			statement: "Dispatcher MUST run every review-tool command in the foreground; a launched Workflow resumes the round through its completion notification."
			title:     "Foreground commands"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-014a"
			role:      "rule"
			statement: "NEVER skip or thin refutation, dedup, verification or station under budget pressure, nor widen chunk bounds past arg/defaults."
			title:     "Budget invariance"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-014c"
			role:      "rule"
			statement: "Worker prompt NEVER mentions remaining budget."
			title:     "No budget mentions"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-CTX-003"
			role:      "rule"
			statement: "Context a direction requires but cannot find MUST be reported, never silent; the direction runs degraded — never a finding, never a halt; lane notes open `MISSING CONTEXT:`, relayed at the gate."
			title:     "Missing context"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-CTX-004"
			role:      "rule"
			statement: "In ACDSL repos gate output is review evidence — directions MUST skip what gates enforce and cite rule IDs."
			title:     "Gate evidence"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DIR-004"
			role:      "rule"
			statement: "With no spec, plan, request doc, ticket text or checklist the correctness direction MUST be dropped, named as dropped."
			title:     "Spec required"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-DIR-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Every direction except code-style: a claim MUST name concrete input, state, or sequence producing wrong behaviour; opinions and unquantified robustness die at lane."
			title:     "Falsifiability bar"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-008"
			payload: {
				info:   "js"
				marker: "```"
			}
			role:      "payload"
			statement: "A stage launch after a typed user message: LAUNCH this script inline as `script`, END the turn, RELAUNCH the stage in the turn its completion notification opens:"
			title:     "Primer hop"
		}
	}]
	kind: "context"
}, {
	title: "Context wiring"
	blocks: [{
		entry: {
			id: "SKILL-RAILROADREVIEW-CTX-001"
			custom: ["hook:skill-context"]
			role:      "def"
			statement: "Dispatcher's doctrine arrives via this skill's frontmatter `acdsl-context:` declaration, injected at invocation."
			title:     "Dispatcher context"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-CTX-002b"
			custom: ["hook:skill-invoke-gate"]
			role:      "rule"
			statement: "Every other tool call but a Read of the required context file is denied until invocation; the injected per-direction context file MUST be read in full."
			title:     "Context gates"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-CTX-002a"
			custom: ["hook:skill-context"]
			role:      "rule"
			statement: "Every lane MUST invoke the Skill tool with `args=\"lane:<direction>\"`, receiving that direction's slice from `context-map.json`."
			title:     "Lane context"
		}
	}]
	kind: "context"
}, {
	title: "Dispatch (dispatcher readies the round)"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-003c"
			role:      "def"
			statement: "`artifactsDir` = the handshake dir the invocation names, else `~/.cache/claude-routine/artifacts/railroad-<runNs>/artifacts`; round files under `round-{r}/`."
			title:     "Artifact paths"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STACK-005"
			role:      "def"
			statement: "`<mainline>` = the branch `git rev-parse --abbrev-ref origin/HEAD` names, remote prefix stripped; `main` when that fails."
			title:     "Mainline"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-001"
			role:      "rule"
			statement: "Premise pin MUST be the fork point — `git merge-base <base> <head>`, never base tip."
			title:     "Guard"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-PROC-006"
			custom: ["hook:read-gate"]
			role:      "rule"
			statement: "Changed files MUST be read in full — whole file, not just hunks."
			title:     "Full-file reads"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-004c"
			role:      "rule"
			statement: "Risk-map high for auth/crypto/input/persisted data MUST add `security`."
			title:     "Security add"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-005b"
			role:      "rule"
			statement: "Lane MUST read only its chunk; `contracts` is chunk-exempt, cross-chunk suspicions go to notes."
			title:     "Chunk conduct"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-005c"
			role:      "rule"
			statement: "Lane file list MUST be its manifest's relevance-filtered `files` — never re-filtered; a `tests` lane adds each file's partners, read, never counted as reviewed."
			title:     "Relevance filter"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-016b"
			role:      "rule"
			statement: "A file is done for a direction after `doneAfter[direction]` consecutive clean reviews."
			title:     "Done rule"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-016d"
			role:      "rule"
			statement: "Any confirmed finding in a file, or applied fix touching it, MUST reset every direction's counter for that file."
			title:     "Counter reset"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-016f"
			role:      "rule"
			statement: "Round 1 MUST review everything."
			title:     "Round-1 full"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-007b"
			role:      "rule"
			statement: "The JSON-only lane artifact is the contract on every route — NEVER a per-route deviation."
			title:     "One artifact contract"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-SCOPE-001"
			role:      "step"
			statement: "RESOLVE scope: arg absent ⇒ `branch` on clean tree, `all` on dirty one, chosen scope stated."
			title:     "Scope first"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-SCOPE-004"
			role:      "step"
			statement: "RUN `git rev-parse HEAD` once per round, before `base`, `lanes` and the buildtest; `headCommit` = its output — every later command and prompt of the round names it."
			title:     "Head pin"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-SCOPE-002"
			fields: {
				mints: {
					id: {
						baseCommit: "<HEAD or fork point>"
						headCommit: "<snapshot>"
					}
				}
			}
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_SNAPSHOT snapshot --scope wip|all`, no branch/index/tree touched; `headCommit` = its `snapshot`, `baseCommit` = HEAD (`wip`) or the fork point (`all`)."
			title:     "Snapshot commit"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-SCOPE-003"
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_SNAPSHOT snapshot` anew each round; fixes land on dispatcher's working tree."
			title:     "Snapshot rounds"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-002a"
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_BASE base --base <bracket> --head <headCommit> --mainline <mainline>` (retrying `origin/<base>` counts); USE its `commit` as `baseCommit`."
			title:     "Explicit base"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-002c"
			role:      "stop"
			statement: "If `$SKILL_RAILROADREVIEW_BASE base --base` exits 2 (bracket base will not resolve) ⇒ STOP: report, never silently substitute."
			title:     "Base unresolvable"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-002b"
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_BASE base --head <headCommit> --mainline <mainline>` when no bracket; USE its `commit` — warning in chat naming it and its `source` (upstream, mainline)."
			title:     "Derived base"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-003"
			role:      "verify"
			statement: "VERIFY base plausibility when the diff runs to thousands of lines; settle a suspected wrong base before reviewing."
			title:     "Wrong-base heuristic"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-PROC-004"
			fields: {
				forwards: [{
					arg:  "exclude"
					flag: "--exclude <csv>"
				}]
			}
			role:      "step"
			statement: "RUN `git diff --numstat <baseCommit> <headCommit>`, narrowed by `paths`, minus `exclude` — names and line counts only, never hunks."
			title:     "Diff"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-011"
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_PAIRS pairs --base <baseCommit> --head <headCommit> [--exclude <csv>]` each round; READ `.tests | length` for direction selection only."
			title:     "Test pairs"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-005"
			role:      "step"
			statement: "STATE inclusions and exclusions up front; hard-class rows of [[INTAKE-002]] are pre-excluded unless `exclude` or `paths` is given."
			title:     "Confirm scope"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-002d"
			role:      "stop"
			statement: "If `$SKILL_RAILROADREVIEW_LANES lanes` exits 2 ⇒ STOP: report its stderr line — an empty diff between base and head, or no context dir (`context/` or the deployed dir holding `context.json`)."
			title:     "Lanes refused"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-INTAKE-002"
			fields: {
				forwards: [{
					arg:  "paths"
					flag: "--paths <csv>"
				}, {
					arg:  "exclude"
					flag: "--exclude <csv>"
				}]
			}
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_INTAKE intake --base <baseCommit> --head <headCommit>` once per run, `--gate` on a [[INTAKE-001]]; RELAY the report; no `exclude`/`paths` ⇒ `excluded` rows are `exclude`."
			title:     "Config analysis"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-INTAKE-003"
			fields: {
				ref: ["SCOPE-001", "TPL-008"]
			}
			pred:      "bare_invocation"
			role:      "stop"
			statement: "If bare invocation ⇒ STOP: RUN `$SKILL_RAILROADREVIEW_TITLE_SET title set CONFIG-GATE`, report; the next prompt — `go`, `defaults` or args — restarts intake with those args."
			title:     "Config gate"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-007a"
			role:      "step"
			statement: "TIER every changed file per [risk-tier entries]($AGENT_CONTEXT_DIR_DEFAULT/actions/reviewing.md)."
			title:     "Risk tiers"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-PROC-007b"
			fields: {
				mints: {
					file: "artifactsDir/round-{r}/risk-map.json"
				}
			}
			role:      "step"
			statement: "WRITE the round's risk map."
			title:     "Risk-map artifact"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROC-008b"
			role:      "step"
			statement: "PARTITION automatically over either bound (mode from `chunks`, `layers` when absent) until every chunk fits; an oversize file rides alone, a test file with its partner; state the partition."
			title:     "Auto-partition"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-004"
			role:      "step"
			statement: "BIND to an explicit `directions` arg — exactly that set, stated; `special-focus` in it still requires `focus`."
			title:     "Direction selection"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-004b"
			role:      "step"
			statement: "START from defaults without `directions`; DROP and NAME directions the diff starves: tests without pairs, correctness without spec, data-integrity without state, security without auth/input/secrets."
			title:     "Diff-proportional set"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-016c"
			role:      "step"
			statement: "PASS `artifactsDir/coverage.json` as `--coverage` from round 2 on (round 1 excludes nothing; `lanes` exits 2 without it) — done files leave their direction's lanes."
			title:     "Round shrink"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-002a"
			pred:      "plan_mode and mode_switch_tool"
			role:      "step"
			statement: "SWITCH a plan-mode session to acceptEdits through `mcp__ccd_session_mgmt__set_session_permission_mode` (`session_id` `self`) — the review runs Workflow stages plan mode denies."
			title:     "Plan-mode switch"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-002"
			pred:      "plan_mode and not mode_switch_tool"
			role:      "stop"
			statement: "If plan-mode session without the Desktop session tool, or no Workflow tool ⇒ STOP: ask the user to leave plan mode (Shift+Tab), then rerun."
			title:     "Runnability gate"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-PROC-009"
			fields: {
				flags: "--repo <dispatcher toplevel> --root <artifactsDir parent>/wt --branch claude-review/<runNs>-basehealth --salvage <artifactsDir>/salvage"
			}
			items: [{
				marker: "*"
				text:   "add `--build '$SKILL_RAILROADREVIEW_BUILD' --test '$SKILL_RAILROADREVIEW_TEST'`."
			}]
			role:      "verify"
			statement: "VERIFY base health before round 1: RUN `$SKILL_RAILROADREVIEW_BUILDTEST buildtest --head <baseCommit>` in the foreground, 600000 ms timeout, `--skip` on `expect-base-red`; READ `build` and `tests`."
			title:     "Base health"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-NIGHTLY-007"
			fields: {
				forwards: [{
					arg: "run-id"
				}]
				mints: {
					id: {
						runNs: "rr-<run-id or scratchpad hex6>"
					}
				}
			}
			role:      "step"
			statement: "MINT `runNs` once per run, before round 1, and PREFIX every branch the round creates with `claude-review/<runNs>-`."
			title:     "Run namespace"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-MODES-003d"
			custom: ["hook:read-gate"]
			fields: {
				flags: "--checklist <direction>=<checklist>"
				forwards: [{
					arg:  "overlay-patch"
					flag: "--worktree-root <worktreeRoot>"
				}, {
					arg:  "known"
					flag: "--known <known>"
				}]
				mints: {
					file: "round-{r}/known.json"
				}
			}
			items: [{
				marker: "*"
				text:   "the `lanes` output stays on stdout, never redirected to a file — heartbeat captures `idFloor` from stdout."
			}]
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_TITLE_SET title set WIP`, then `$SKILL_RAILROADREVIEW_LANES lanes` each round after the checklist agents; PASS `lanes` as `lanePlan`, `knownPath`, `idFloor` verbatim."
			title:     "Lane manifests"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-PROC-012"
			fields: {
				mints: {
					file: "round-{r}/spec-<n>-<basename>"
					id: {
						n: "<1-based position in the spec list>"
					}
				}
			}
			role:      "step"
			statement: "COPY each spec every round before the checklist stage: `cp <spec file> <artifactsDir>/round-{r}/spec-<n>-<basename>` (plan dir: its `design/<slug>.md`); PASS copies as `specPaths`, empty without spec."
			title:     "Spec copy"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-WALK-004"
			channel: "script"
			fields: {
				ref: ["TPL-008"]
				script: "workflows/checklist.js"
			}
			role:      "spawn"
			statement: "INVOKE `workflows/checklist.js` — one `railroad-checklist` agent per direction lacking a checklist: contracts on a moved head, over the diff; correctness in round 1 without checklist, over the spec."
			title:     "Checklist agents"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-WALK-005"
			parentSpawn: "SKILL-RAILROADREVIEW-WALK-004"
			role:        "substep"
			statement:   "EXTRACT the requirements (correctness; test-only ones omitted) or changed boundaries (contracts) as numbered `\"<direction>-<n>: <text>\"` items, grouped coherently."
			title:       "Extract checklist"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-016"
			fields: {
				flags: "--repo <dispatcher toplevel> --root <artifactsDir parent>/wt --branch claude-review/<runNs>-buildtest-r{r} --salvage <artifactsDir>/salvage"
				forwards: [{
					arg:  "expect-head-red"
					flag: "--skip"
				}]
				mints: {
					branch: "claude-review/<runNs>-buildtest-r{r}"
				}
				timeout: 600000
			}
			items: [{
				marker: "*"
				text:   "add `--build '$SKILL_RAILROADREVIEW_BUILD' --test '$SKILL_RAILROADREVIEW_TEST'`."
			}]
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_BUILDTEST buildtest --head <headCommit>` in the foreground each round; PASS its `{build, tests}` as `buildTest` — head health, never a finding source."
			title:     "Build and test"
		}
	}]
	kind: "phase"
}, {
	title: "Review (invocation to human gate)"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-007c"
			role:      "def"
			statement: "Round stages, in script order: fan-out, dedup per direction then cross-direction, second confirmation (refuters and the verification pool concurrent; a lost refuter's claim retried at once), station."
			title:     "Round stages"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-008"
			role:      "def"
			statement: "`confirmed` (survived, artifact in hand) | `debunked` (refuted) | `unverified` (survives but not demonstrable here); \"could not reproduce\" is a successful result."
			title:     "Refuter verdicts"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-009b"
			role:      "def"
			statement: "Deduped claim refs with every second-confirmation verdict and verdict path, head build/test result, `baseHealth`, unioned coverage gaps."
			title:     "Station inputs"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-012"
			custom: ["schema:review", "todo:validation-needed"]
			role:      "def"
			statement: "Closed set, exactly one per claim: `confirmed` | `unverified` (kept, flagged) | `rejected-with-reason` | `duplicate-of` | `debunked`."
			title:     "Dispositions"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-MODES-003a"
			items: [{
				fence: {
					body: """
						round.js    — Round orchestrator (runs the four stages below as child workflows)
						lanes.js    — Fan-out
						dedup.js    — Groupers (per-direction, then cross-direction)
						verify.js   — Second confirmation (refuters + verification pool, concurrent)
						station.js  — Station barrier

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "The dispatcher invokes `round.js` once per round (`scriptPath`) — the stage files below run as its child workflows in stage order, each stage's return handed to the next in memory:"
			title:     "Stage files"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-003"
			role:      "rule"
			statement: "Lanes MUST NOT edit, enter plan mode, or run tree-mutating git; the artifact is written under `round-{r}/` outside the worktree, then the lane ends."
			title:     "Review-only lanes"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-005a"
			custom: ["hook:read-gate"]
			role:      "rule"
			statement: "A finished lane is fully covered by construction — its artifact write MUST stay denied until every manifest file is fully read and every walk pair answered."
			title:     "Coverage check"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-005b"
			custom: ["hook:read-gate"]
			role:      "rule"
			statement: "`<artifact>.coverage.json` sidecar MUST be written by the enforcing hook from its own observation, never lane self-report."
			title:     "Measured coverage"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-WALK-006"
			custom: ["hook:read-gate"]
			role:      "rule"
			statement: "A lane's artifact write marking `walk_complete` MUST stay denied while any checklist pair (file × group, or group) has no `walk` entry in the artifact; the deny message lists the pairs."
			title:     "Walk gate"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-WALK-007"
			role:      "rule"
			statement: "A lane MUST write its artifact skeleton after its reads and append every finding and every walk answer the moment it exists; the station MUST build `review.json` the same way — never one final dump."
			title:     "Incremental artifact"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-WALK-010"
			role:      "rule"
			statement: "A finding answering a checklist item MUST carry the item id in `checklist`; a group-first FAIL is a finding with effect `incorrect` citing it; an item without a walk answer is NEVER a finding."
			title:     "Item citation"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-022b"
			role:      "rule"
			statement: "Each survivor MUST be second-confirmed exactly once — refuter at/above `refute-level`, else pool; a lost refuter's claim re-enters the pool."
			title:     "One verdict each"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-035"
			role:      "rule"
			statement: "A second-confirming worker MUST receive everything the producing lane received — `lane:<direction>` injection, lane manifest, diff, spec, focus, context refs, both ledgers."
			title:     "Refuter context parity"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-011b"
			role:      "rule"
			statement: "Station NEVER second-confirms a claim itself; only a lost verifier or opposing verdicts make it re-read, adversarially, stating so."
			title:     "No station self-verify"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-023"
			custom: ["gate:skill-008"]
			role:      "rule"
			statement: "Workflow runtime runs at most 16 agents concurrently and queues the rest — the workflow MUST spawn freely, no script throttle."
			title:     "Concurrency limits"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-009c"
			role:      "rule"
			statement: "Station NEVER dedups, probes, or builds itself; every id, direction, severity and its cells MUST carry verbatim."
			title:     "Station never re-derives"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-009d"
			role:      "rule"
			statement: "Station MUST echo the script's `draft_dispositions` and `draft_probes`, adjusting ONLY ledger/known matches, inconsistent verdicts and lost-verdict claims."
			title:     "Echo the drafts"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-009e"
			role:      "rule"
			statement: "The station input MUST travel inside the script-built station prompt — never a file, never a writer agent, never dispatcher-assembled."
			title:     "Station input inline"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-010"
			role:      "rule"
			statement: "Refuter verdicts are binding — they MUST map straight into dispositions with their reason, probe, and artifacts."
			title:     "Verdict intake"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-019"
			role:      "rule"
			statement: "Suspected but undemonstrated defects in migrations, auth, concurrency, money, public API, or persisted data MUST go into `review.json`'s `escalations` field."
			title:     "Escalate, don't bury"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-024a"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Every worker prompt MUST batch its independent tool calls into one message."
			title:     "Batch tool calls"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-024b"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "A worker prompt MUST NOT load a new tool mid-run."
			title:     "No mid-run loads"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-024c"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "A worker prompt MUST NOT fully read a file outside its assigned review set."
			title:     "Scoped reads only"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-MODES-005a"
			fields: {
				forwards: [{
					arg:  "chunks"
					flag: "--chunks round-{r}/chunks.json"
				}]
				mints: {
					file: "round-{r}/chunks.json"
				}
			}
			role:      "step"
			statement: "WRITE the partition as `round-{r}/chunks.json` (`[{name, files}]`) when chunking — modes `layers`, `languages` or pathspec groups, each chunk within `[maxFiles:maxLines]` — PASS it as `--chunks`."
			title:     "Lane set"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-MODES-005e"
			fields: {
				ref: ["ROUND-005a"]
			}
			role:      "step"
			statement: "STATE the lane plan's skipped list in the review record."
			title:     "Filter bookkeeping"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-MODES-003b"
			fields: {
				forwards: [{
					arg: "chunks"
				}, {
					arg:  "walk-group"
					flag: "--ceiling <walk-group>"
				}]
			}
			role:      "step"
			statement: "UNPACK brackets into the real JSON `args` object: `scope[base]` → `base`, `lanes[model:effort]` → `laneModel`/`laneEffort`; every other dash arg becomes its camelCase field."
			title:     "Bracket unpacking"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-MODES-019"
			fields: {
				forwards: [{
					arg: "overlay-patch"
				}, {
					arg: "prebuilt"
				}]
			}
			role:      "step"
			statement: "SET the explicit-worktree keys from `overlay-patch`: `worktreeMode` `explicit`, `targetRepo` session toplevel, `worktreeRoot` `<dirname(artifactsDir)>/wt`, `overlayPatch` the path — none without it."
			title:     "Explicit worktrees"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-019a"
			role:      "verify"
			statement: "Before round 1: VERIFY `git apply -R --check <overlayPatch>` exits 0 in the session checkout, else `git apply <overlayPatch>` uncommitted; REVERSE it around each fix-apply and snapshot, never staged."
			title:     "Cwd overlay"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-001"
			channel: "script"
			custom: ["todo:validation-needed"]
			fields: {
				forwards: [{
					arg:   "exclude"
					stage: "excludes"
				}, {
					arg:   "lanes"
					stage: "laneModel"
				}, {
					arg:   "overlay-patch"
					stage: "overlayPatch"
				}, {
					arg:   "prebuilt"
					stage: "prebuilt"
				}, {
					arg:   "spec"
					stage: "specPaths"
				}]
				mints: {
					id: {
						runId: "<the round.js launch's run id>"
					}
				}
				schema: "schemas/round-args.json"
				script: "workflows/round.js"
			}
			items: [{
				marker: "*"
				text:   "`baseHealth` is the base-health buildtest's `build`/`tests`; `lanePlan`, `knownPath`, `idFloor` the `lanes` output — verbatim, never from the schema."
			}]
			role:      "spawn"
			statement: "SPAWN via `round.js` (child `lanes.js`) `lanes` lanes per selected direction (× chunks), each in an isolated worktree at `headCommit`, blind to sibling lanes."
			title:     "Fan-out"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-032"
			channel: "script"
			fields: {
				script: "workflows/lanes.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-001"
			role:        "substep"
			statement:   "REVIEW the assigned chunk against the direction's context — full-file reads, claims with concrete evidence; RETURN claim refs and the artifact path."
			title:       "Lane review"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-WALK-008"
			channel: "script"
			fields: {
				script: "workflows/lanes.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-001"
			role:        "substep"
			statement:   "ANSWER one group at a time — file by file on `file-first`, group by group on `group-first` — until no pair is open, then mark `walk_complete`."
			title:       "Walk the checklist"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-ROUND-002"
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-001"
			role:        "stop"
			statement:   "If the pin checkout to `headCommit` fails or `baseCommit` is no ancestor of it ⇒ STOP: abort with commit facts."
			title:       "Lane premise"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-ROUND-004"
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-001"
			role:        "verify"
			statement:   "Before return: VERIFY each claim's cited code survives a lane re-read; DROP what does not, record the check in `evidence`."
			title:       "Claims output"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-WALK-009"
			channel: "script"
			fields: {
				mints: {
					branch: "claude-review/<runNs>-<lane branch>-a<n>"
				}
				script: "workflows/lanes.js"
			}
			role:      "spawn"
			statement: "SPAWN the same lane again over the same manifest when it returns nothing or returns with `walk_open > 0`; it continues from its artifact."
			title:     "Continuation lane"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-025"
			channel: "script"
			custom: ["todo:validation-needed"]
			fields: {
				schema: "schemas/grouper.json"
				script: "workflows/dedup.js"
			}
			role:      "spawn"
			statement: "SPAWN text-only grouper agents — no worktree, no code access."
			title:     "Groupers"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-033"
			channel: "script"
			fields: {
				script: "workflows/dedup.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-025"
			role:        "substep"
			statement:   "RECOMPUTE every lane-returned claim's severity from its cells at dedup entry; a mismatch corrects the severity, re-mints the id's SEV token and records `{from_id, to_id}` in the stage's corrections."
			title:       "Severity recompute"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-006"
			channel: "script"
			fields: {
				script: "workflows/dedup.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-025"
			role:        "substep"
			statement:   "GROUP same-defect claims across lanes; in doubt keep separate — a wrong merge drops a finding."
			title:       "Per-direction dedup"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-022a"
			channel: "script"
			fields: {
				script: "workflows/dedup.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-025"
			role:        "substep"
			statement:   "PASS the union of all claims through one grouper after per-direction dedups, before any refutation; absorbed cross-direction pairs reach the station as `duplicate-of`."
			title:       "Cross-direction dedup"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-007"
			channel: "script"
			custom: ["todo:validation-needed"]
			fields: {
				mints: {
					branch: "claude-review/<runNs>-refute-{finding-id}-r{r}-{hash6}"
				}
				ref: ["ROUND-035"]
				schema: "schemas/refuter.json"
				script: "workflows/verify.js"
			}
			role:      "spawn"
			statement: "SPAWN a fresh refuter that did not produce it per cross-deduped claim ≥ `refute-level` (`none` disables the stage) — one candidate, per Probe protocol."
			title:     "Refutation"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-026"
			channel: "script"
			fields: {
				script: "workflows/verify.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-007"
			role:        "substep"
			statement:   "PROBE the claim per the Probe protocol — one candidate, both directions."
			title:       "Probe claim"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-027"
			channel: "script"
			fields: {
				script: "workflows/verify.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-007"
			role:        "substep"
			statement:   "RETURN the verdict with artifacts — confirmed, debunked or unverified."
			title:       "Return verdict"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-011a"
			channel: "script"
			custom: ["todo:validation-needed"]
			fields: {
				mints: {
					branch: "claude-review/<runNs>-verify-b{k}-r{r}-{hash6}"
				}
				schema: "schemas/refuter.json"
				script: "workflows/verify.js"
			}
			role:      "spawn"
			statement: "SPAWN verifier agents over below-`refute-level` claims concurrently with refutation; lost-refuter claims re-enter the pool the moment their refuter is lost."
			title:     "Verification pool"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-028"
			channel: "script"
			fields: {
				script: "workflows/verify.js"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-011a"
			role:        "substep"
			statement:   "SECOND-CONFIRM each claim of the batch with the refuter brief and the Probe protocol."
			title:       "Confirm batch"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-ROUND-009a"
			channel: "script"
			custom: ["todo:validation-needed"]
			fields: {
				mints: {
					branch: "claude-review/<runNs>-station-r{r}-{hash6}"
				}
				schema: "schemas/station-return.json"
				script: "workflows/station.js"
			}
			role:      "spawn"
			statement: "SPAWN one consolidator agent: isolated worktree pinned at `headCommit`."
			title:     "Station barrier"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-ROUND-014"
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-009a"
			role:        "substep"
			statement:   "RESOLVE spec-vs-code conflicts against `decisions.md`, separating real bugs from intentional deviations."
			title:       "Reconciliation"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-ROUND-014b"
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-009a"
			role:        "substep"
			statement:   "ASSIGN every confirmed finding a `route` ([[OUT-006]])."
			title:       "Routing"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-ROUND-015"
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-009a"
			role:        "substep"
			statement:   "NAME a durable check catching the class per recurring-class confirmed finding; the dispatcher relays and offers, never adds unasked."
			title:       "Permanent checks"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-017"
			fields: {
				flags: "--marker DoD"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-009a"
			role:        "substep"
			statement:   "MARK every `[DoD]`-marked context entry PASS/FAIL/N/A, queried with `$SKILL_RAILROADREVIEW_ENTRIES entries`."
			title:       "DoD walk"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-018"
			custom: ["todo:validation-needed"]
			fields: {
				schema: "schemas/consolidation.json"
			}
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-009a"
			role:        "substep"
			statement:   "WRITE the consolidated review as `round-{r}/review.json` — the single handoff artifact."
			title:       "Handoff"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-ROUND-020"
			parentSpawn: "SKILL-RAILROADREVIEW-ROUND-009a"
			role:        "substep"
			statement:   "RECORD the funnel in `review.json` — claims produced / confirmed / unverified / duplicates / rejected / debunked."
			title:       "Funnel"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-021"
			fields: {
				ref: ["TPL-007"]
			}
			items: [{
				fence: {
					body: """
						$SKILL_RAILROADREVIEW_CLEANUP cleanup --repo <repo> --prefix claude-review/<runNs>- --head <headCommit> --salvage <artifactsDir>/salvage --run <runId> [--run <runId>]… [--root <worktreeRoot> --overlay <overlayPatch>]

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "step"
			statement: "RUN the round cleanup after the fix pipeline, one Bash call — one `--run` per kept run id, `--root`/`--overlay` on the explicit route only; RELAY `skipped` and `unattributed`:"
			title:     "Cleanup"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-001"
			role:      "step"
			statement: "PASS `--plan-dir <plan dir>` to `$SKILL_RAILROADREVIEW_LEDGER ledger` — it copies `review.json` and `probes/` to plan dir's `reviews/round-{r}/`; a `review.md` only on request, derived."
			title:     "Handoff copy"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-002"
			fields: {
				ref: ["TPL-008"]
			}
			role:      "verify"
			statement: "After a round: VERIFY `handoff` non-null, `abortedLanes` empty (`void: true` = all lanes aborted); a Workflow crash RESUMES via `resumeFromRunId`, else reset and re-dispatch once per [[TPL-009]]."
			title:     "Failed round"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-002b"
			pred:      "void_rounds >= 2"
			role:      "stop"
			statement: "If a second round is recorded void ⇒ STOP: report abort notes verbatim."
			title:     "Second failure"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-010"
			fields: {
				flags: "'{recommendation, confirmed: (.ranked_plan | length)}' <artifactsDir>/round-{r}/review.json"
			}
			role:      "verify"
			statement: "Before accepting a round: VERIFY the ledger validation held (exit 2 lists violations), else re-run the merge, never hand-accept; RUN `jq` on `review.json` for the recommendation."
			title:     "Handoff conformance"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-003"
			role:      "step"
			statement: "PRESENT the round with `$SKILL_RAILROADREVIEW_PRESENT present --review <review.json> --ignore-level <level>`; no comment ⇒ accepted, comments bind; the fix pipeline never waits."
			title:     "Human gate"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-007"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "Pinned-worktree removal is salvage-then-remove: bytes archived, never deleted, no force, no `git clean`:"
			title:     "Salvage sequence"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-006"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Every agent's model and effort, one table:"
			title:     "Model assignment"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-009"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "A failed round's retry starts from a clean state under the same run namespace, round number and branch names, one Bash call per command:"
			title:     "Round reset"
		}
	}]
	kind: "phase"
}, {
	title: "Fixes"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-005b"
			role:      "rule"
			statement: "The fix set MUST be every new confirmed ranked-plan entry at or below `autofix-level` routed `auto-fix`, minus human-struck; the rest stay `open` at the gate."
			title:     "Fix scope"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-STATION-005"
			channel: "script"
			fields: {
				mints: {
					id: {
						fixHead:    "<the fixer's last commit>"
						preFixHead: "<the fixer's base>"
					}
				}
				ref: ["OUT-006", "TPL-008"]
				returns: ["applied", "failed"]
				schema: "schemas/fix-return.json"
				script: "workflows/fix-apply.js"
			}
			pred:      "review_complete"
			role:      "spawn"
			statement: "INVOKE `workflows/fix-apply.js` with the ledger's `autofix` set, whole — one `railroad-fixer` commits per package through package-commit; the dispatcher NEVER edits a file."
			title:     "Fix application"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-016"
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_LEDGER ledger --known <artifactsDir>/known.json --round {r} --fix-state applied --ids <csv>` with the stage's `applied` ids."
			title:     "Record applied"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-006"
			role:      "verify"
			statement: "After fixes land: VERIFY `fixHead` non-null (snapshot scopes: the next snapshot differs); RUN `git format-patch -o <artifactsDir>/round-{r}/fixes <preFixHead>..<fixHead>`."
			title:     "Fix commit"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-011a"
			role:      "step"
			statement: "RUN probe replay plus fix check after the round's fixes land, before the next round; no applied fixes ⇒ skipped, stated."
			title:     "Fix check"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-011b"
			fields: {
				flags: "--repo <dispatcher toplevel> --head <fixHead> --root <artifactsDir parent>/wt --branch claude-review/<runNs>-replay-r{r} --rounds <artifactsDir>/round-1 … <artifactsDir>/round-{r}"
			}
			items: [{
				marker: "*"
				text:   "add `--acdsl '$SKILL_RAILROADREVIEW_LINT' --build '$SKILL_RAILROADREVIEW_BUILD'`."
			}]
			role:      "verify"
			statement: "After fixes: VERIFY every persisted probe of rounds 1..r with `$SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay` against `fixHead`; ANY non-green probe is a hit."
			title:     "Probe replay"
		}
	}, {
		entry: {
			id:      "SKILL-RAILROADREVIEW-STATION-011c"
			channel: "script"
			fields: {
				ref: ["TPL-008"]
				schema: "schemas/fix-check.json"
				script: "workflows/fix-check.js"
			}
			role:      "spawn"
			statement: "INVOKE `workflows/fix-check.js` over `preFixHead..fixHead` — one fresh `railroad-fix-checker` agent judges per applied finding whether its defect is gone."
			title:     "Fix check stage"
		}
	}, {
		entry: {
			id:          "SKILL-RAILROADREVIEW-STATION-014"
			parentSpawn: "SKILL-RAILROADREVIEW-STATION-011c"
			role:        "substep"
			statement:   "JUDGE each applied finding at the post-fix head: defect gone ⇒ `fixed`; FLAG everything else as a hit."
			title:       "Judge fixes"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-017"
			fields: {
				flags: "--known <artifactsDir>/known.json --round {r} --fix-state fixed"
			}
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_LEDGER ledger --ids <csv>` with the checker's `fixed` ids minus this round's red probes; an earlier round's red `fixed` row goes back to `applied`, reported, never reverted."
			title:     "Record fixed"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-009"
			pred:      "finding_reconfirmed"
			role:      "stop"
			statement: "If the known-ledger call prints `oscillating` ids (a finding confirmed again after its fix landed) ⇒ STOP: report; never a third automated attempt."
			title:     "Oscillation guard"
		}
	}]
	kind: "phase"
}, {
	title: "Ledgers (between rounds)"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-004e"
			role:      "rule"
			statement: "A rejected finding NEVER comes back."
			title:     "Rejections permanent"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-004d"
			role:      "rule"
			statement: "Ledger matching MUST go by file and claim substance, never by ID."
			title:     "Match by substance"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-007"
			role:      "rule"
			statement: "The next round MUST run fresh lanes with no memory of prior rounds, carrying only the rejection and known ledgers."
			title:     "Clean-slate rounds"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-004a"
			items: [{
				fence: {
					body: """
						finding_id, file, line
						claim, reason
						round, date

						"""
					info:   "text"
					marker: "```"
				}
			}]
			pred:      "review_complete"
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_LEDGER ledger --reject <id> --reason <text> --rejected <plan dir>/reviews/rejected.json --round-dir <round dir>` per human-rejected finding:"
			title:     "Append rejection"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-STATION-004b"
			role:      "step"
			statement: "COPY the ledger to `artifactsDir/round-{r}/rejected.json` each later round; PASS the path, never content."
			title:     "Ledger replay"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-004c"
			items: [{
				fence: {
					body: """
						finding_id, file, line
						severity, effect, exposure
						claim, fix, route
						round, fix_state

						"""
					info:   "text"
					marker: "```"
				}
			}, {
				fence: {
					body: """
						$SKILL_RAILROADREVIEW_LEDGER ledger --round-dir <artifactsDir>/round-{r} --known <artifactsDir>/known.json --coverage <artifactsDir>/coverage.json --autofix-level <level> --ignore-level <level> [--done-after <n>] [--plan-dir <plan dir>]

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "step"
			statement: "RUN the known-ledger call after the gate, one Bash call (`--plan-dir` in a plan run, `--done-after` only with the arg):"
			title:     "Known ledger"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-016a"
			role:      "step"
			statement: "MAINTAIN `artifactsDir/coverage.json` through `$SKILL_RAILROADREVIEW_LEDGER ledger` — per direction × file `{clean_reviews, last_round, done}`."
			title:     "Coverage ledger"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-018"
			role:      "step"
			statement: "REPORT a one-line suggestion to raise lane model to `opus` when new confirmed findings fall far below last round — the operator applies it, never automatic."
			title:     "Escalation suggestion"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-001"
			pred:      "mode = interactive and round_gate_pending"
			role:      "stop"
			statement: "If interactive ⇒ STOP after the round completes end-to-end (gate, fixes, fix check, patch, report): RUN `$SKILL_RAILROADREVIEW_TITLE_SET title set HUMAN-GATE` before the gate; next round on go."
			title:     "Round gate"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-MODES-014b"
			pred:      "budget_low"
			role:      "stop"
			statement: "If remainder can't fund full next round ⇒ STOP after this one: report count honestly."
			title:     "Budget stop"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-ROUND-037"
			role:      "step"
			statement: "RUN `git apply -R <overlayPatch>` in the session checkout right before [[ROUND-036]], one Bash call — only an overlay this run applied; a refused reverse is reported, never forced."
			title:     "Cwd overlay reverse"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-ROUND-036"
			fields: {
				forwards: [{
					arg:  "verbose"
					flag: "--verbose"
				}]
			}
			role:      "step"
			statement: "RUN `$SKILL_RAILROADREVIEW_PRUNE prune --artifacts <artifactsDir>` as the review's last action, never between rounds, one Bash call."
			title:     "Artifact prune"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-STATION-008a"
			fields: {
				ref: ["ROUND-036"]
			}
			role:      "stop"
			statement: "If two consecutive clean rounds (a handoff, no new blocking finding, no aborted lane, no lost claim) and the gate accepts ⇒ STOP: finished, APPROVED."
			title:     "Termination"
		}
	}]
	kind: "phase"
}, {
	title: "Nightly route"
	blocks: [{
		entry: {
			id: "SKILL-RAILROADREVIEW-NIGHTLY-001a"
			fields: {
				forwards: [{
					arg: "rounds"
				}]
			}
			role:      "step"
			statement: "LOOP rounds to the `rounds` cap without a per-round go — same workflow route, headless; hitting the cap is reported, never silent."
			title:     "Unattended rounds"
		}
	}]
	kind:     "route"
	pred:     "mode = nightly"
	replaces: "MODES-001"
}, {
	title: "Probe protocol (second confirmation)"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-002a"
			role:      "def"
			statement: "Three parts: an untracked `railroad-probes-r{r}.acdsl` rule line at the worktree root; a verifier (files list plus `key=value` argv; exit 0 clean, 1 with `file:line: message`); a registry."
			title:     "Probe anatomy"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-002b"
			role:      "def"
			statement: "Third part: the probe's own entries under `round-{r}/probes/` — `acdsl check -registry` lays them over the repo registry, never a copy of it; its rule id is `PROBE-<FINDING-SLUG>`."
			title:     "Probe registry"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-005"
			role:      "rule"
			statement: "Exactly the scripts, `fixtures/`, the per-finding registries and `<finding-slug>.acdsl` mirrors MUST persist under `round-{r}/probes/`, indexed in `review.json`'s `probes[]`, never sidecars."
			title:     "Probe persistence"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-007"
			role:      "rule"
			statement: "A test demonstrates a claim ONLY if it fails at `headCommit` for the stated reason and passes with the fix applied temporarily in a disposable worktree — both stated, never committed."
			title:     "Both directions"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-001"
			role:      "rule"
			statement: "Prober MUST author a deterministic debug probe per claim in ACDSL repos — second confirmation, two authoring attempts."
			title:     "Deterministic probes"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-003"
			role:      "rule"
			statement: "Before the real run, the probe MUST exit 1 on a minimal fail fixture under `round-{r}/probes/fixtures/`; a probe blind to it is a failed attempt, the fixture path recorded on the `probes[]` entry."
			title:     "Validity first"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-004"
			role:      "rule"
			statement: "The verdict MUST come from `<worktree>/bin/acdsl check -rule <probe-id> [-registry <probe registry>]` (the prebuilt binary, never a build) — red on the claimed file ⇒ `confirmed`; green ⇒ `debunked`."
			title:     "Probe verdict"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-PROBE-006"
			role:      "rule"
			statement: "Non-ACDSL or twice-unprobeable claims MUST settle via the cheapest artifact — failing test, command with actual+expected, or repro sequence — under `round-{r}/`, flagged `probe: \"none: <why>\"`."
			title:     "Manual path"
		}
	}]
	kind: "context"
}, {
	title: "Output Format"
	blocks: [{
		prose: "These rules govern gate presentation and any on-demand rendering; data contract is `review.json`."
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-002"
			role:      "def"
			statement: "`<direction>-<SEV>-<k>-r<r>`, SEV ∈ BLK · MAJ · MIN · NIT · INF; `k` numbered from 1 per direction and kept through re-mints, `r` the round."
			title:     "ID"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-005"
			role:      "def"
			statement: "Two cells per finding, from evidence: `effect` (what goes wrong), `exposure` (share of normal operation hit); severity is the table row for the cells, never a grade."
			title:     "Severity vector"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-005b"
			role:      "def"
			statement: "`schemas/severity-table.json` maps every cell pair to one severity, `severity-fixtures.json` holds the ratified cases; lane assigns, dedup raises and a refuter lowers at most once each, then frozen."
			title:     "Severity table"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-006"
			role:      "def"
			statement: "Per confirmed finding: `human` (judgment, surfaced at the gate) | `auto-fix` (mechanical, fix-apply stage), each with a one-sentence `route_reason`; a make-source touch routes `human`."
			title:     "Route"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-010"
			role:      "def"
			statement: "`review.json`'s `probes[]` is the probe index (id, finding, script, fixture, registry, tree verdict per probe)."
			title:     "Probes index"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-009"
			pred:      "((base_build_red or base_tests_red) => recommendation = REJECT) and ((head_build_red or head_tests_red) => recommendation != APPROVE)"
			role:      "rule"
			statement: "Recommendation MUST be APPROVE (nothing blocking) | CONDITIONAL (blocking fixes pending) | REJECT; blocking = new, ≥ MINOR, above `ignore-level`; red head caps at CONDITIONAL, red base forces REJECT."
			title:     "Recommendation"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-001"
			role:      "rule"
			statement: "One row per finding of the round, proposed fix in every row; State says `new` or `carried: <fix_state>`; rows at or below `ignore-level` stand in one Ignored line under the table."
			title:     "Findings table"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-003"
			role:      "rule"
			statement: "The proposed fix is NEVER empty: concrete edit, one sentence or short diff sketch."
			title:     "Proposed fix"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-004"
			role:      "rule"
			statement: "Every review MUST state files reviewed against the diff list; uncovered files listed explicitly, never silently dropped."
			title:     "Coverage"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-007"
			role:      "rule"
			statement: "Rows MUST order by severity; the finding id's direction prefix keeps each finding's direction visible."
			title:     "Row order"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-OUT-008"
			role:      "rule"
			statement: "Every review MUST end with the Definition-of-Done summary table."
			title:     "DoD table"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-003"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Findings table format:"
			title:     "Table template"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-004"
			payload: {
				marker: "```"
			}
			role:      "payload"
			statement: "Definition of Done summary table format:"
			title:     "DoD template"
		}
	}]
	kind: "context"
}, {
	title: "Review record"
	blocks: [{
		entry: {
			id:        "SKILL-RAILROADREVIEW-DELIVER-001"
			role:      "rule"
			statement: "Dispatcher MUST maintain the session plan file, one section per round, each referencing the round's handoff path — never a hand-written `plan.md` in the repo."
			title:     "Plan-file record"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DELIVER-002"
			role:      "rule"
			statement: "Reviews MUST accumulate in the session plan file — append new dated sections, preserve earlier ones."
			title:     "Merge, don't overwrite"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DELIVER-003"
			role:      "rule"
			statement: "A superseded finding MUST be marked resolved (strikethrough + pointer to resolving review), never deleted."
			title:     "Resolved findings"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DELIVER-004"
			role:      "step"
			statement: "SYNC worktree, CONFIRM spec current, REPORT deltas."
			title:     "Re-review deltas"
		}
	}, {
		entry: {
			id:        "SKILL-RAILROADREVIEW-DELIVER-005"
			role:      "step"
			statement: "RENDER each review section with `$SKILL_RAILROADREVIEW_PRESENT present --review <review.json> --ignore-level <level> --section <heading>` per [[TPL-005]]."
			title:     "Section structure"
		}
	}, {
		entry: {
			id: "SKILL-RAILROADREVIEW-TPL-005"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Review section structure:"
			title:     "Section template"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "multi-standard compliance review; railroad orchestration — opus or above in every role"
			tier:   "frontier"
			notes: ["opus is the floor for every role — dispatcher, lanes, judgment workers, station; the sonnet class is out of this skill"]
			unviable: ["sonnet-5 as dispatcher (interactive route) and as station — g2 run 2026-09-16: protocol mislabeling, hand-condensed station input, inadequate BLOCKER fix accepted", "sonnet-5 as lane and judgment-worker model — golden test 2026-10-06-1f725c at lanes 1[sonnet:medium], two rounds: recall 13.6% against the g2 v1 golden (1 of 8 BLOCKERs, 7 of 41 MAJORs), review score 0, 126% of a Team Premium window against 117% for 2[opus:medium] over two rounds"]
		}
	}]
}]
tier: "strict"
atoms: {
	bare_invocation: {
		sort:   "state"
		source: "session"
	}
	base_build_red: {
		sort:   "state"
		source: "runtime: base_health.base_build != ok"
	}
	base_tests_red: {
		sort:   "state"
		source: "runtime: base_health.base_tests != ok"
	}
	budget_low: {
		sort:   "state"
		source: "session"
	}
	finding_reconfirmed: {
		sort:   "state"
		source: "runtime: ledger.oscillating != []"
	}
	head_build_red: {
		sort:   "state"
		source: "runtime: head_buildtest.head_build != ok"
	}
	head_tests_red: {
		sort:   "state"
		source: "runtime: head_buildtest.head_tests != ok"
	}
	plan_mode: {
		sort:   "state"
		source: "session"
	}
	recommendation: {
		kind: "enum"
		members: ["APPROVE", "CONDITIONAL", "REJECT"]
		sort:   "state"
		source: "runtime: review_json.recommendation"
	}
	review_complete: {
		sort:   "state"
		source: "runtime: ledger.autofix"
	}
	round_gate_pending: {
		sort:   "state"
		source: "session"
	}
	void_rounds: {
		kind:   "int"
		sort:   "count"
		source: "runtime: ledger.void_rounds"
	}
}
enforce: {
	anchors: {
		base: {
			capture: [{
				name:    "base_commit"
				pointer: "/commit"
			}]
			exclude: [{
				field:   "command"
				pattern: "[|;&`]|\\$\\("
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_BASE base "
			}]
			tool: "^Bash$"
		}
		base_health: {
			capture: [{
				name:    "base_build"
				pointer: "/build"
			}, {
				name:    "base_tests"
				pointer: "/tests"
			}]
			exclude: [{
				field:   "command"
				pattern: "[|;&`]|\\$\\("
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_BUILDTEST buildtest .*--branch \\S+-basehealth(\\s|$)"
			}]
			tool: "^Bash$"
		}
		cleanup: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_CLEANUP cleanup "
			}]
			tool: "^Bash$"
		}
		fix_apply: {
			input: [{
				field:   "scriptPath"
				pattern: "/railroad-review/workflows/fix-apply\\.js$"
			}]
			tool: "^Workflow$"
		}
		fix_check: {
			input: [{
				field:   "scriptPath"
				pattern: "/railroad-review/workflows/fix-check\\.js$"
			}]
			tool: "^Workflow$"
		}
		fix_state_fixed: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_LEDGER ledger (\\S+ )*--fix-state fixed(\\s|$)"
			}]
			tool: "^Bash$"
		}
		head_buildtest: {
			capture: [{
				name:    "head_build"
				pointer: "/build"
			}, {
				name:    "head_tests"
				pointer: "/tests"
			}]
			exclude: [{
				field:   "command"
				pattern: "[|;&`]|\\$\\("
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_BUILDTEST buildtest .*--branch \\S+-buildtest-r[0-9]+(\\s|$)"
			}]
			tool: "^Bash$"
		}
		hunk_read: {
			exclude: [{
				field:   "command"
				pattern: "--(numstat|name-only|name-status|stat)\\b"
			}]
			input: [{
				field:   "command"
				pattern: "^git (-C \\S+ )?diff "
			}]
			tool: "^Bash$"
		}
		lanes: {
			capture: [{
				name:    "lanes"
				pointer: "/lanes"
			}, {
				name:    "id_floor"
				pointer: "/idFloor"
			}, {
				name:    "known_path"
				pointer: "/knownPath"
			}]
			exclude: [{
				field:   "command"
				pattern: "[|;&`]|\\$\\("
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_LANES lanes "
			}]
			tool: "^Bash$"
		}
		ledger: {
			capture: [{
				name:    "autofix"
				pointer: "/autofix"
			}, {
				name:    "oscillating"
				pointer: "/oscillating"
			}, {
				name:    "void_rounds"
				pointer: "/void_rounds"
			}]
			exclude: [{
				field:   "command"
				pattern: "--reject "
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_LEDGER ledger (\\S+ )*--round-dir "
			}]
			tool: "^Bash$"
		}
		probe_replay: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay "
			}]
			tool: "^Bash$"
		}
		review_json: {
			capture: [{
				name:    "recommendation"
				pointer: "/recommendation"
			}]
			exclude: [{
				field:   "command"
				pattern: "[;&`]|\\$\\("
			}]
			input: [{
				field:   "command"
				pattern: "^jq .*round-[0-9]+/review\\.json(\\s|$)"
			}]
			tool: "^Bash$"
		}
		round: {
			input: [{
				field:   "scriptPath"
				pattern: "/railroad-review/workflows/round\\.js$"
			}]
			tool: "^Workflow$"
		}
	}
	checks: {
		cleanup_before_end: {
			after:    "round"
			kind:     "stop"
			reason:   "railroad-review: run `$SKILL_RAILROADREVIEW_CLEANUP cleanup` for the round before ending the turn"
			recovery: "heartbeat runtime ({finding}): run the round's `$SKILL_RAILROADREVIEW_CLEANUP cleanup`, then end the turn"
			require:  "cleanup"
		}
		fix_after_ledger: {
			after:    "ledger"
			kind:     "requires"
			on:       "fix_apply"
			reason:   "railroad-review: run `$SKILL_RAILROADREVIEW_LEDGER ledger --round-dir …` after the round, before the fix-apply stage — its `autofix` set is the fix set"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_RAILROADREVIEW_LEDGER ledger` for this round, then pass its `autofix` set to fix-apply.js"
			since:    "round"
		}
		fixed_after_fix_check: {
			after:    "fix_check"
			kind:     "requires"
			on:       "fix_state_fixed"
			reason:   "railroad-review: a row becomes fixed only after fix-check.js judged this round's fixes"
			recovery: "heartbeat runtime ({finding}): invoke workflows/fix-check.js over the fix commit, then pass its `fixed` ids"
			since:    "fix_apply"
		}
		fixed_after_replay: {
			after:    "probe_replay"
			kind:     "requires"
			on:       "fix_state_fixed"
			reason:   "railroad-review: a row becomes fixed only after `$SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay` ran against the post-fix head"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay`, then leave its red finding ids out of `--ids`"
			since:    "fix_apply"
		}
		next_round_after_fix_check: {
			after:    "fix_check"
			kind:     "requires"
			on:       "round"
			reason:   "railroad-review: after fixes land, fix-check.js runs before the next round"
			recovery: "heartbeat runtime ({finding}): invoke workflows/fix-check.js over the fix commit, then start the next round"
			since:    "fix_apply"
		}
		next_round_after_ledger: {
			after:    "ledger"
			kind:     "requires"
			on:       "round"
			reason:   "railroad-review: a new round needs `$SKILL_RAILROADREVIEW_LEDGER ledger --round-dir …` over the previous round first"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_RAILROADREVIEW_LEDGER ledger` for the previous round, then start the next"
			since:    "round"
		}
		next_round_after_replay: {
			after:    "probe_replay"
			kind:     "requires"
			on:       "round"
			reason:   "railroad-review: after fixes land, `$SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay` runs before the next round — a hit goes to the gate"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_RAILROADREVIEW_PROBE_REPLAY probe-replay` against the post-fix head; hits go to the gate, then start the next round"
			since:    "fix_apply"
		}
		no_hunks: {
			deny:     "hunk_read"
			from:     "lanes"
			kind:     "phase"
			reason:   "railroad-review: the dispatcher never reads diff hunks — `git diff` content stays lane-side (`--numstat`, `--name-only` and `--stat` are fine)"
			recovery: "heartbeat runtime ({finding}): drop the hunk read; use `git diff --numstat` or read the lane artifacts"
			until:    "cleanup"
		}
		round_after_lanes: {
			after:    "lanes"
			kind:     "requires"
			on:       "round"
			reason:   "railroad-review: run `$SKILL_RAILROADREVIEW_LANES lanes` before the round — its `lanes` array is the round's lanePlan"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_RAILROADREVIEW_LANES lanes`, then pass its `lanes` array as lanePlan"
			since:    "cleanup"
		}
		round_base_health_build: {
			arg:      "/args/baseHealth/build"
			capture:  "base_build"
			from:     "base_health"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.baseHealth.build must be the `build` the base-health `$SKILL_RAILROADREVIEW_BUILDTEST buildtest` printed"
			recovery: "heartbeat runtime ({finding}): pass the base-health buildtest result verbatim: {evidence}"
		}
		round_base_health_tests: {
			arg:      "/args/baseHealth/tests"
			capture:  "base_tests"
			from:     "base_health"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.baseHealth.tests must be the `tests` the base-health `$SKILL_RAILROADREVIEW_BUILDTEST buildtest` printed"
			recovery: "heartbeat runtime ({finding}): pass the base-health buildtest result verbatim: {evidence}"
		}
		round_buildtest_build: {
			arg:      "/args/buildTest/build"
			capture:  "head_build"
			from:     "head_buildtest"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.buildTest.build must be the `build` the round's head `$SKILL_RAILROADREVIEW_BUILDTEST buildtest` printed — never a re-run without `--overlay`"
			recovery: "heartbeat runtime ({finding}): pass the round's head buildtest result verbatim: {evidence}"
		}
		round_buildtest_tests: {
			arg:      "/args/buildTest/tests"
			capture:  "head_tests"
			from:     "head_buildtest"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.buildTest.tests must be the `tests` the round's head `$SKILL_RAILROADREVIEW_BUILDTEST buildtest` printed — never a re-run without `--overlay`"
			recovery: "heartbeat runtime ({finding}): pass the round's head buildtest result verbatim: {evidence}"
		}
		round_id_floor: {
			arg:      "/args/idFloor"
			capture:  "id_floor"
			from:     "lanes"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.idFloor must be the `idFloor` the last `$SKILL_RAILROADREVIEW_LANES lanes` printed, unedited"
			recovery: "heartbeat runtime ({finding}): pass the `idFloor` of the last `$SKILL_RAILROADREVIEW_LANES lanes` verbatim"
		}
		round_known_path: {
			arg:      "/args/knownPath"
			capture:  "known_path"
			from:     "lanes"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.knownPath must be the `knownPath` the last `$SKILL_RAILROADREVIEW_LANES lanes` printed"
			recovery: "heartbeat runtime ({finding}): pass the `knownPath` of the last `$SKILL_RAILROADREVIEW_LANES lanes` verbatim"
		}
		round_lane_plan: {
			arg:      "/args/lanePlan"
			capture:  "lanes"
			from:     "lanes"
			kind:     "provenance"
			on:       "round"
			reason:   "railroad-review: args.lanePlan must be the `lanes` array the last `$SKILL_RAILROADREVIEW_LANES lanes` printed, unedited"
			recovery: "heartbeat runtime ({finding}): pass the `lanes` array of the last `$SKILL_RAILROADREVIEW_LANES lanes` verbatim"
		}
	}
	gate: {
		planModeDenied: ["Workflow"]
		rootDenied: ["*/known.json", "*/coverage.json", "*/reviews/rejected.json"]
		workflowOnly: ["railroad-lane-*", "railroad-grouper", "railroad-refuter", "railroad-verifier", "railroad-station", "railroad-checklist", "railroad-fixer", "railroad-fix-checker"]
		writers: [{
			agentTypes: ["railroad-checklist"]
			glob: "*/round-*/checklist-*.json"
		}]
	}
	judgment: {
		base_plausible: {
			anchor: "base"
			evidence: ["base_commit", "arg:/command"]
			question: "Is the resolved base the branch's real fork point — not a stale upstream?"
			send:     "heartbeat runtime ({finding}): the runtime agent questions the review base — re-check it before round 1: {evidence}"
		}
		direction_selection: {
			anchor: "lanes"
			evidence: ["arg:/command", "lanes"]
			question: "Do the selected directions fit the diff's risk (security for auth or input handling, data-integrity for persisted state)?"
			send:     "heartbeat runtime ({finding}): the runtime agent questions the direction selection for this diff: {evidence}"
		}
		recommendation_fits: {
			anchor: "review_json"
			evidence: ["recommendation", "arg:/command"]
			question: "Does the recommendation fit the round's findings and health — no APPROVE over a red base, no CONDITIONAL over an empty plan?"
			send:     "heartbeat runtime ({finding}): the runtime agent questions this round's recommendation: {evidence}"
		}
		tiering: {
			anchor: "lanes"
			evidence: ["arg:/command"]
			question: "Do the lane count and lane model fit the diff size and risk?"
			send:     "heartbeat runtime ({finding}): the runtime agent questions the lane tiering for this diff: {evidence}"
		}
	}
}
smoke: {
	env: {
		BUILD: "go build ./internal/cmdline/..."
		TEST:  "go test ./internal/cmdline/..."
	}
	expect: {
		anchors: ["base_health", "cleanup", "head_buildtest", "lanes", "ledger", "round"]
		minChecks: 0.75
	}
	fixture: {
		patch: "change.patch"
		tree:  "tree"
	}
	invoke: "smoke review directions: correctness tests lanes: 1[sonnet:low] station: opus:low scope: wip paths: internal/smokereview spec: smoke-rr-spec.md refute-level: none ignore-level: none autofix-level: none expect-head-red nightly rounds: 1"
	limits: {
		minutes: 60
		usd:     15.0
	}
	model: "opus"
}
title: {
	states: {
		"CONFIG-GATE": {
			entry:   "SKILL-RAILROADREVIEW-INTAKE-003"
			meaning: "review config analyzed — reply go, defaults, or an args line"
		}
		"HUMAN-GATE": {
			entry:   "SKILL-RAILROADREVIEW-MODES-001"
			meaning: "round complete — say go for the next round or accept the review"
		}
	}
}
