changelog: [{
	date:    "2026-10-20"
	version: "13.5.3"
	text:    "Interpretation: at most 8 bullets of 160 runes, gated; the Tests tab shows all 8."
	tokens:  4086
}, {
	date:    "2026-10-17"
	version: "13.5.2"
	text:    "Run cleanup names the salvage layout <worktree>-out/salvage/ and the trash: the out dir moves there, never deleted."
	tokens:  4086
}, {
	date:    "2026-10-17"
	version: "13.5.1"
	text:    "The smoke route names make skill-smoke SKILL=<leaf> TOKEN=<label>; the target refuses an empty token."
	tokens:  4080
}, {
	date:    "2026-10-14"
	version: "13.5.0"
	text:    "Schema 2.13: runs[].claudeCodeVersion from the stream init record; golden.perRun[].severities[].outcomes puts every golden entry in exactly one outcome (atPrice, wrongSeverity, fixed, fixUnverified, fixFailed, dismissed, missed) with its count and minutes. The Tests tab's config widget shows environment and CLI version, the breakdown counts entries by where their minutes landed and notes every outcome not priced at count × price."
	tokens:  4071
}, {
	date:    "2026-10-13"
	version: "13.4.0"
	text:    "Requires the mining component: the skill drives this repo's own make targets and never deploys outside the smine checkout."
	tokens:  4071
}, {
	date:    "2026-10-12"
	version: "13.3.2"
	text:    "Schema 2.12: golden.perRun[].severities — the scorer's per-severity entries, found, missed, false positives and the found, escaped and false-positive minutes; the Tests tab's breakdown table reads it."
	tokens:  4071
}, {
	date:    "2026-10-12"
	version: "13.3.1"
	text:    "Interpretation: bullets only, at most 5 of 160 runes, the worker pins as one bullet; the Tests tab clamps to the same bound."
	tokens:  4071
}, {
	date:    "2026-10-09"
	version: "13.3.0"
	text:    "Runtime placement declared: owns acceptEdits (non-plan-mode runs)."
	tokens:  4050
}, {
	date:    "2026-10-06"
	version: "13.2.0"
	text:    "The adherence judge grades an entry run inside a Workflow agent that no trace line or artifact shows as not-observable, never followed; the fold excludes it from step_miss_pct like not-applicable, counts it per run as step_unobserved, and report.md lists it under Unobservable entries."
}, {
	date:    "2026-10-06"
	version: "13.1.0"
	text:    "skilleval arm -resume <session id> resumes a run's stopped cell in its worktree after a usage-window stop: the stopped artifacts move back, the cell gets a continuation prompt, its records append behind a resume marker in stream.jsonl, and result.json keeps the first start and sums both processes' cost and output tokens."
	tokens:  4050
}, {
	date:    "2026-10-05"
	version: "13.0.0"
	text:    "Golden runs are produce or test (arg golden); the definition lists the skill's args as data (schema 4.0) and init composes the prompt, with set for the args the definition opens; a run's config is frozen at init — run.json with a digest, seed or golden snapshots in the run dir, provision.json for worktrees; skilleval promote appends a produce run's findings to the seed file; the golden judge runs on test runs only; report.md lists every step miss."
	tokens:  4023
}, {
	date:    "2026-10-05"
	version: "12.1.0"
	text:    "Cleanup salvages worktrees into the run's <worktree>-out dir instead of runs/<label>/salvage, then removes the out dir; a removed: false row keeps it. The prebuild log lives there too, never in the record. The runner places goldens[0].seedFile as the run's known.json before the cell; seed rows (round 0) are never candidates."
	tokens:  3529
}, {
	date:    "2026-10-02"
	version: "12.0.0"
	text:    "Thin eval layer: args skill, replicas, token only; go run ./cmd/skilleval init builds run.json from eval/definition.json (3.1: check, reference, scorer, model, effort, replicas) with prerequisite, token and window checks; judge briefs from skilleval brief; scorer metrics land in result.json scores and eval.json 2.11; arms, compare judge and statistics moved to /skillroutine-ab; smoke flag retired."
	tokens:  3523
}, {
	date:    "2026-10-02"
	version: "11.2.1"
	text:    "The repo registry moved out of the main checkout to `~/.claude/smine/repos.json`; the definition schema's targetRepo description names it."
	tokens:  8242
}, {
	date:    "2026-09-30"
	version: "11.2.0"
	text:    "Schema 2.10: every arm captures heartbeat's enforced runs of its session into runtime.json (skilleval arm -heartbeat <bin>); the fold reports check_failures and runs_killed, source heartbeat."
	tokens:  8242
}, {
	date:    "2026-09-29"
	version: "11.1.1"
	text:    "Worker pins: a fan-out spawn without a model is stopped by heartbeat instead of inheriting the arm model; workflow spawns now name a model through the generated GEN:models vocabulary."
}, {
	date:    "2026-09-29"
	version: "11.1.0"
	text:    "Dropped the inner-wildcard Bash(git * diff *) grant — git -C <dir> diff matches Bash(git diff *) through the gate's -C normalization."
}, {
	date:    "2026-09-29"
	version: "11.0.0"
	text:    "Made token mandatory — intake stops without a listed label and the arm runner refuses an empty one; arms never fall back to an env token, an API key or the CLI login."
}, {
	date:    "2026-09-28"
	version: "10.0.0"
	text:    "Cross-repo git moved into go run ./cmd/skilleval provision <run dir>: repo lookup, ref resolution, sweep, arm worktrees and overlay, reference.patch; run.json gains repoName; judges read arm worktrees via Read(~/.cache/claude-routine/worktrees/run-*/**)."
}, {
	date:    "2026-09-28"
	version: "9.0.2"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-27"
	version: "9.0.1"
	text:    "Declared Delegation: degraded — Agent-tool fan-out, degrades below root."
}, {
	date:    "2026-09-27"
	version: "9.0.0"
	text:    "Arm cleanup and stale sweep salvage by program (skilleval cleanup, gitwt); arm artifacts live outside the run dir during the arm; output tokens, cost and denials cover all agents from the result envelopes (eval.json 2.9); a wrong-model arm stops at its init line; arms run under a gate read fence with neutral run-<id> names and a staged overlay; arm tokens read <model>[:<effort>], full CLI ids accepted."
}, {
	date:    "2026-09-27"
	version: "8.2.0"
	text:    "Run dir by program: go run ./cmd/skilleval init creates runs/, copies the deployed SKILL.md and the preset overlay into the run dir (no Read/Write streaming); run.json carries overlayPatch, the arm runner renders {overlayPatch} into the prompt and arm worktrees apply that copy; grants add git -C branch/diff forms and printenv for the window-budget env read."
}, {
	date:    "2026-09-27"
	version: "8.1.0"
	text:    "Repos resolve by repo-registry name, never by path: the repo arg and a preset's targetRepo name a repos.json entry at the main checkout, an unregistered name stops before any arm; the SKILLEVAL_TARGET_REPO fallback is gone, so a golden preset needs only its arms — /skillroutine-eval railroad-review arms: opus-high, sonnet-high."
}, {
	date:    "2026-09-27"
	version: "8.0.0"
	text:    "Golden scoring prices a run in minutes: scoring.json carries minutes, escapeMultiplier, triageLine and per-route fixFactor instead of severityWeights; new metrics review_score (1 - lost / (null - oracle)), lost_minutes, escaped_minutes and false_positive_minutes replace weighted_recall and precision in eval.json 2.8, report.md and best-arm selection; a finding truly at or above the triage line but reported below it counts as escaped, false positives (duplicates included) cost their claimed minutes plus the fix review when a machine applied one; candidates carry route, effect and exposure plus applied/fixed joined from the definition's findings.ledger; judge.json gains fixEscapes; golden and candidate severities the price list lacks fail the fold; scores are not comparable with pre-8.0.0 evals."
}, {
	date:    "2026-09-26"
	version: "7.2.0"
	text:    "Smoke no longer fails on permission denials — closed-world hook denials hit every arm; new per-arm metric permission_denials (count, lower better) from peek session_events permissions.requests rejects, null with a note when peek serves no permission block, shown as its own report.md column and compared by evalstats; the envelope permission_denials capture and result.json permissionDenials removed; smoke_pass is not comparable with pre-7.2.0 evals."
}, {
	date:    "2026-09-26"
	version: "7.1.1"
	text:    "Cross-repo git runs headless: allowed-tools grants the git -C forms (git * apply|checkout|worktree *); arm provisioning and cleanup pin git -C <path>, never cd or a chain. Unblocks the skill-eval-railroad-review routine against the smine target repo."
}, {
	date:    "2026-09-26"
	version: "7.1.0"
	text:    "token: <label> arg: arms bill a stored OAuth token from the routine token store, by label only — intake checks the label against the store listing and never repeats a value; run.json carries the label, the arm process exports the value; report and eval.json name the auth used. Usage guide in docs/skilleval.md."
}, {
	date:    "2026-09-26"
	version: "7.0.0"
	text:    "Comparison-first rewrite: arms are headless roots differing in model, effort or disabled MCP servers; go run ./cmd/skilleval arm|fold|goldens|sweep replaces every cmd/skilleval shell and jq file; mechanical smoke verdict, per-arm adherence judge, blind compare judge; the golden route stays, scored from the golden family's scoring.json (definition schema 3.0) with normalize lineage, efficiency and rounds telemetry dropped; eval.json 2.7 validated before write; route/extra args and the dead workflows/*.js removed."
}, {
	date:    "2026-09-17"
	version: "6.1.0"
	text:    "Scoped worker root rule: a fan-out arm explicit worktreeRoot is <worktreeRoot>/railroad-eval-<runId>/round-<r>, never the shared pool root, cleanup included; Model section records sonnet-5 unviable as the arm dispatcher (g2 eval 2026-09-16)."
}, {
	date:    "2026-09-16"
	version: "6.0.0"
	text:    "Generic contract-runner: schema 2.0 contract (invocation/environment/artifacts/budgets; scoring keys stay top-level for fold.jq/normalize.sh), one route in two containers — interactive session-as-dispatcher and a headless claude -p root cell (lib/runner.sh, allowed-tools + Workflow/Skill, budgets enforced); promptTemplate, knobs/env registry, desktop wrapper workflows and the skill-specific harness checks retired from the body; extraction is the contract-declared jq over the skill's natural artifacts with a loud zero-findings warning."
}, {
	date:    "2026-09-16"
	version: "5.4.0"
	text:    "Production-bootstrap mode: goldens[0].bootstrap=true with an absent golden file runs unjudged (PRE-004c) — golden total 0, confirmed findings are promotion candidates, range from goldens[0].range; schema gains bootstrap and range on goldens items; headless twins in load_golden (stub golden), arm judge skips, and a normalize skip for unminted bootstrap goldens."
}, {
	date:    "2026-09-16"
	version: "5.3.2"
	text:    "Reflowed ARMS-004c (rounds telemetry fold) to the format-2 rune caps — statement shortened, detail split into five param bullets; content unchanged."
}, {
	date:    "2026-09-16"
	version: "5.3.1"
	text:    "Seed knob gains a definition default: ARMS-011a resolves env SKILLEVAL_SEED_KNOWN else run.seedKnown (headless resolve_seed identical); definition schema run block adds peekVersion and seedKnown so strict validation accepts pinned, self-sufficient golden-run definitions."
}, {
	date:    "2026-09-16"
	version: "5.3.0"
	text:    "Desktop-route parity with the peek-authority telemetry pipeline: PRE-008a/b pin the peek version against /api/healthz when the definition sets run.peekVersion (mirrors the headless verify_peek_version, exit 78); ARMS-004c folds the peek per-subagent breakdown into each arm row as active_seconds plus rounds_telemetry with review-scope filtering, review.json-mtime round attribution and dispositions-sourced confirmed_ids — the same shape the headless arm_rounds_telemetry writes."
}, {
	date:    "2026-09-14"
	version: "5.2.0"
	text:    "Full formal-schema reformat: Rules section marked {section: context}; freeform Applies:/indented sub-bullets normalized to param: registers; 18 of 44 coverable entries gained pred/post clauses (mode reused from the global vocabulary; local atoms.json declares definition_validated/goldens_verified/desktop_dispatch/single_pass_mode/parallel and related route/stop guards); three queries added under queries/ (a stop blocking Arm plan, the desktop dispatcher gated to the interactive route, a single-pass-fanout witness) — all skillfmt/skillmodel check/consistency/query checks green."
}, {
	date:    "2026-09-14"
	version: "5.1.0"
	text:    "Schema 2.6 reapplied on the v2-format base (the 4.6.0 commit was reverted for the format-v2 migration): round-indexed efficiency block — perRound rows vs the golden roundLadder at matching saturation depth with split marginal/discovery scores (token + active-time axes), compositeMarginal/compositeDiscovery composites, cumulative token axis as fallback"
}, {
	date:    "2026-09-14"
	version: "5.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-13"
	version: "4.5.1"
	text:    "Register sweep — over-long bullets split to one claim each; workerCap dropped from the desktop dispatcher args and eval-arm.js after the railroad workflow lost its script throttle (the Workflow runtime queues at 16 itself; the subagent env var caps nightly Agent-tool dispatch only)"
}, {
	date:    "2026-09-09"
	version: "4.5.0"
	text:    "Single-pass route generalized beyond superpowers: run.desktop.render (fenced|body) keys the reviewerPrompt rendering, run.desktop.transcribe carries the product transcription instruction into eval-arm-singlepass.js args.transcribePrompt (fanout passes it to every arm); built-in superpowers Issues-section transcription stays the default — first consumer aqms-review-wip"
}, {
	date:    "2026-09-08"
	version: "4.4.1"
	text:    "Judge section: pin the scoring agent to a fixed named model/effort independent of the arm under test — an arm-inherited judge confounds judge quality with the measured thing and corrupts the A/B metric (JUDGE-004)"
}, {
	date:    "2026-09-08"
	version: "4.4.0"
	text:    "Env-free frozen-pack delivery on the single-pass fanout: the preflight extracts the frozen index from the golden overlay in a scratch worktree, verifies the freeze sha, renders the declared pack via the skill-context hook to a run-dir pack file, and every reviewer reads that file via the singlepass contextPack preamble — replaces the forced-Skill-call/contextSkill mechanism, which resolved the live session index and could not honor the freeze from a desktop UI session; headless keeps live hook injection via the cell-exported frozen index."
}, {
	date:    "2026-09-08"
	version: "4.3.1"
	text:    "ARMS-006d sessionIndex clarified: it is the index the hook actually resolves — SKILL_CONTEXT_INDEX from the session env when set, else the session checkout index; frozen-pack runs after context drift launch the eval session with SKILL_CONTEXT_INDEX at the extracted frozen index instead of re-freezing."
}, {
	date:    "2026-09-08"
	version: "4.3.0"
	text:    "Single-pass fanout: new workflows/eval-arm-singlepass-fanout.js — mode single-pass with parallel > 1 runs ALL arms in one arm-to-judge pipeline invocation (new ARMS-006d, ARMS-006a exception, JUDGE-003 in-pipeline judging); freeze preflight (session index sha + synthetic-render id count vs contextFreeze); eval-arm-singlepass gains contextSkill forced-Skill-call preamble and worktree-add lock retry; ARMS-006c overlayPatch resolves contextFreeze.overlay first and adds contextSkill."
}, {
	date:    "2026-09-08"
	version: "4.2.0"
	text:    "Route-parity update: ARMS-008 summary assembly gains the headless-parity fields (funnel_per_round with fixes_patch, known_findings from the ledger, leftover_worktrees expected empty), ARMS-010 arm cleanup becomes salvage-then-remove (never forced, never deleting bytes), new ARMS-013 stale-run sweep via cmd/skilleval/sweep_stale.sh shared with the headless route; finding ids referenced as run-unique with the -r{r} round suffix"
}, {
	date:    "2026-09-08"
	version: "4.1.0"
	text:    "Caveman trim of over-cap entry statements (wording only, cap now 320 display runes)"
}, {
	date:    "2026-09-07"
	version: "4.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), Args bullets -> table, real argument-hint, normalized allowed-tools order, bare entry-id refs turned into title links, caveman pass."
}, {
	date:    "2026-09-07"
	version: "3.8.0"
	text:    "Routine parity for the desktop route. New env: arg — paste a routine plist EnvironmentVariables dict, KEY=VALUE lines, or JSON; keys map through the shared registry cmd/skilleval/knobs.json (PRE-007) with an applied / accepted-not-enforceable / ignored report. Explicit-worktree dispatch (ARMS-006b): the wrapped workflow runs worktreeMode explicit — dispatcher-provisioned detached target-repo worktrees with the context overlay applied, retiring the cwd gate (ARMS-007 is now a target-repo resolve). Full knob wiring: apply level, seed ledger (ARMS-011), station model/effort, workerCap, judge model/effort (JUDGE-003), worktree root. Telemetry from the peek per-subagent breakdown with budget-delta fallback (ARMS-004/009), desktop manifest for eval.json .setup (ARMS-012). Definition schema 1.3: goldens[0].contextlessFile — the context knob selects the per-environment golden (PRE-004); normalize.sh runs as the terminal step of every run and is scope-aware (FOLD-005)."
}, {
	date:    "2026-09-07"
	version: "3.7.0"
	text:    "Schema 2.5: golden.normalized.perRun gains efficiency — strict normalized recall over the run's share of the golden-production budget (rounds / output tokens / wall vs the golden's production.totals), geometric-mean composite; 1.0 = golden-production pace. Written by cmd/skilleval/normalize.sh; golden masters carry rounds per production contribution."
}, {
	date:    "2026-09-07"
	version: "3.6.0"
	text:    "Schema 2.4: eval.notesBullets (<=5 bullets, <=120 chars; headless route synthesizes them post-fold), golden.goldenVersion required (preconditions gate, fold default dropped), golden.normalized lineage re-scoring via new cmd/skilleval/normalize.sh; cell/backfill cache-token envelope fallbacks; metrics direction enum gains neutral empty string. Per-environment goldens: goldens[] gains optional scope (full | contextless) as a declarative label; the environment is selected by pointing goldens[0].file at a generated golden — cmd/skilleval/gen_goldens.sh derives flag-free full/contextless files plus the seed from a hand-maintained *.master.json carrying the contextOnly classification, and make audit golden-check rejects stale derived files. The runner refuses goldens carrying classification keys. Schema also adds seedFile (previously undeclared under additionalProperties: false)."
}, {
	date:    "2026-09-07"
	version: "3.5.0"
	text:    "Desktop route learns single-pass arms: run.desktop.mode keys the per-arm workflow (rounds -> eval-arm.js unchanged, single-pass -> new eval-arm-singlepass.js with worktree+overlay setup, one reviewer agent, report transcription); definition schema gains desktop.mode and makes wraps rounds-only. First consumer: superpowers-review."
}, {
	date:    "2026-09-05"
	version: "3.4.2"
	text:    "Declared branch-prefix: claude-review/eval- frontmatter (agent branch-prefix convention)"
}, {
	date:    "2026-09-05"
	version: "3.4.1"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-05"
	version: "3.4"
	text:    "desktop-fidelity arm route: definitions may declare run.desktop {wraps, lanes, refute, clear} (schema 1.2, which also legalizes run.windowBudgetUsd); ARMS-006..010 run one workflows/eval-arm.js Workflow invocation per arm — a setup agent pins the arm worktree/branch, a deterministic round loop nests the evaluated skill's own workflow (run.desktop.wraps) once per round, a fix agent applies confirmed blocking findings between rounds, termination on two consecutive clean rounds or the vars.roundCap. The arm is blind: summary.json is assembled mechanically by the skill from round-*/review.json; judges run via the Agent tool in the arm worktree before cleanup. Session cwd must be inside the target repo; rounds is a real measurement, output_tokens approximated via budget.spent() deltas, cost/duration/turns null"
}, {
	date:    "2026-09-04"
	version: "3.3"
	text:    "PRE-006: documented self-row derivation and its prose-skill fallback — self rows (axis:self) derive from rules render-skill --list-entries <skillMd>; when that returns [] (prose SKILL.md, no entry-id format) reuse the prior eval hand-synthesized self rows only if the loaded SKILL.md is byte-identical, else hand-derive fresh, recording which path was taken in the run report"
}, {
	date:    "2026-09-04"
	version: "3.2"
	text:    "judge adjudicates unmatched findings instead of auto-scoring them false positives: the judge cell runs inside the arm's target worktree with read tools, receives the golden's human-ruled falsePositives as binding wrong-claim rulings plus the reviewed range, and classifies every unmatched run finding as falsePositive (untrue / known-wrong / out of range) or the new neutral uncataloged bucket (genuine, in range, absent from the golden — informational count, no threshold). Output schema bumped to v2.2 (additive golden.perRun[].uncataloged + uncataloged metric). Golden range blocks gain optional checkoutRef (arm worktrees check it out; default headSha) and the runner honors SKILLEVAL_BASE_REF/HEAD_REF/CHECKOUT_REF env overrides"
}, {
	date:    "2026-09-03"
	version: "3.1"
	text:    "body migrated to addressable entries (SKILL-SKILLROUTINEEVAL-{PRE,ARMS,JUDGE,FOLD,RULES}-NNN) per the repo skill format — content unchanged"
}, {
	date:    "2026-09-03"
	version: "3.0"
	text:    "cut to the golden pipeline: axis rubric / mechanical probes / ternary rule scoring / manifest mode / matrix mode deleted; the eval is fully driven by the evaluated skill's eval/definition.json (schema 1.1: severityWeights, run block, defaults, judge overrides); weighted_recall (severity-weighted golden recall) is the threshold and best-arm metric with plain recall still reported; interactive route fronts workflows/skilleval.js (arms as sequential-by-default agents, no cost telemetry on that route), headless route is cmd/skilleval/run.sh with the shared cmd/skilleval/fold.jq; manifest.schema.json and workflows/parallel-eval.js removed"
}, {
	date:    "2026-09-02"
	version: "2.8"
	text:    "verdict block carries the applied thresholds as data (metricId -> bound); reason is the plain best-arm rule sentence — the config server renders the verdict from these fields instead of parsing prose"
}, {
	date:    "2026-09-01"
	version: "2.7"
	text:    "golden-driven skill tests: a skill may bundle eval/definition.json (new reference/definition.schema.json) declaring prerequisites, golden pointers, skill-specific metrics, verdict thresholds and a judgeGuide. New section 0.5 checks the prerequisites, judges each run's findings against the golden (matched/missed/false-positive/misclassified), joins the golden metrics, and computes a finished pass/fail verdict with a best arm — nothing left for the reader. Output schema bumped to v2.1 (additive golden + verdict blocks; schemaVersion enum 2.0|2.1); the emit step now cross-checks golden/verdict references"
}, {
	date:    "2026-08-26"
	version: "2.6"
	text:    "matrix mode: documented the telemetry asymmetry — only CLI cells (skill-eval routine / matrix.sh) carry model.telemetry; time/token-bearing model bake-offs route via the CLI runner, scored in manifest mode"
}, {
	date:    "2026-08-23"
	version: "2.5"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "2.4"
	text:    "matrix-mode resultsDir default moved to evals/<skill>-<base6>/ (durable results live under evals/, consumed by the config server's Tests tab)"
}, {
	date:    "2026-08-23"
	version: "2.3"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-18"
	version: "2.2"
	text:    "self rows: --list-entries line documented as 1-based (rules render-skill now emits file line numbers)"
}, {
	date:    "2026-08-18"
	version: "2.1"
	text:    "allowed-tools permission manifest for headless runs; skill-eval routine named as the headless A/B runner (matrix mode stays interactive)"
}, {
	date:    "2026-08-18"
	version: "2.0"
	text:    "three axes (self/context/output) with entry ids as rubric ids, metrics table, variants in matrix mode, schema 2.0 + manifest 2.0; exemplar path corrected"
}, {
	date:    "2026-08-12"
	version: "1.8"
	text:    "contextFiles example corrected to rules/plan.md (style/ layout retired)"
}, {
	date:    "2026-07-31"
	version: "1.7"
	text:    "contextFiles example updated to style/plan.md"
}, {
	date:    "2026-07-27"
	version: "1.6"
	text:    "renamed couchskill-eval → skillroutine-eval; workflow couchskill-parallel-eval → skillroutine-parallel-eval; matrix resultsDir default sessions/skillroutine-eval/; sibling handoffs → skillroutine-create, smine-skills; effort token normalized (large → high)"
}, {
	date:    "2026-07-26"
	version: "1.5"
	text:    "Args section"
}, {
	date:    "2026-07-19"
	version: "1.4"
	text:    "matrix mode — couchskill-parallel-eval pipe workflow nests parallelize fan-out then evals the surviving runs"
}, {
	date:    "2026-07-19"
	version: "1.3"
	text:    "renamed eval-skill → couchskill-eval; moved under skills/couchskill/"
}, {
	date:    "2026-07-16"
	version: "1.2"
	text:    "skill authoring delegates to /couchskill-create"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-13"
	version: "1.0"
	text:    "initial version"
}]
enforcePermissions: ["acceptEdits"]
frontmatter: [{
	key:   "name"
	value: " skillroutine-eval"
}, {
	key:   "description"
	value: " Measure one declared configuration of a skill on its eval/definition.json — time, tokens, cost, step misses, check, scorer or golden scores. Trigger on /skillroutine-eval <skill> or \"evaluate a skill\". Args — skill: skill with eval/definition.json; replicas: runs; token: stored token label, required; golden: produce|test; set: open arg."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 13.5.3"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"railroad-review replicas: 1 token: <label> golden: produce set: rounds=2\""
}, {
	key:   "branch-prefix"
	value: " claude-review/run-"
}, {
	key:   "acdsl-context"
	value: " FACT-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Agent, Glob, Grep, Read, Read(~/.cache/claude-routine/worktrees/run-*/**), TaskOutput, Write, Bash(git diff *), Bash(go run ./cmd/skilleval *)"
}]
preamble: """
	# Skillroutine eval (one declared configuration)

	Runs a skill's `eval/definition.json` as headless replicas of one configuration and reports per run: mechanical
	verdict, check, wall and active time, output tokens, cost, permission denials, step misses against the skill's own
	entries, plus scorer metrics or golden scores as the definition declares. Output: run dir
	`evals/<skill>/<date>-<hash6>/` with `report.md`, `eval.json`
	([reference/schema.json](reference/schema.json), 2.13, validated) and per-run records under `runs/<label>/`.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "skillroutine-ab"
					optional: true
				}]
				standalone: true
			}
			preconditions: ["session cwd is the claude-configs checkout — `go run ./cmd/skilleval` and `evals/` resolve there", "non-plan-mode session; peek-mcp at `http://127.0.0.1:4242/mcp` (config server up), else telemetry cells stay null", "the skill bundles `eval/definition.json`", "the worktree root is the default one — judges read run worktrees through the skill's read grant"]
			purpose: "answers \"how good is one configuration of this skill on its declared task\" — time, tokens, cost, step adherence, check, scorer or golden scores."
			routes: [{
				for: "what one change does — model, effort, MCP server, overlay, skill version"
				use: "/skillroutine-ab"
			}, {
				for: "does the skill run at all on a dummy input"
				use: "`make skill-smoke SKILL=<leaf> TOKEN=<label>`"
			}, {
				for: "creating or editing a skill"
				use: "/skillroutine-create"
			}, {
				for: "predicting a run's cost before spending it"
				use: "/skillroutine-estimate"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "skill"
				default: "none, required"
				effect:  "evaluated skill; its `eval/definition.json` is the run, its deployed SKILL.md the adherence rubric — [Init](#skill-skillroutineeval-intake-008)"
				form:    "free text, first"
			}, {
				name:    "replicas"
				default: "definition `replicas`, else `1`"
				effect:  "runs of the one configuration — [Init](#skill-skillroutineeval-intake-008)"
				form:    "`replicas: <n>`"
			}, {
				name:    "token"
				default: "none, required"
				effect:  "stored OAuth token the runs bill, by label only — [Unknown token](#skill-skillroutineeval-intake-012)"
				form:    "`token: <label>`"
			}, {
				name:    "golden"
				default: "`test` when the definition has a golden"
				effect:  "produce a golden from the seed, or test against the minted golden — [Init](#skill-skillroutineeval-intake-008)"
				form:    "`golden: produce\\|test`"
			}, {
				name:    "set"
				default: "absent"
				effect:  "sets an arg the definition opens, for this run — [Init](#skill-skillroutineeval-intake-008)"
				form:    "`set: <name>=<value>`, repeatable"
			}]
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-RULES-006"
			role:      "rule"
			statement: "The evaluated skill MUST NEVER be modified for a run; a change that seems necessary is a design-premise violation — stop and report."
			title:     "Skill untouched"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-RULES-009"
			role:      "rule"
			statement: "Fan-out worker models MUST come from the task's own pins; a spawn without a model is stopped by heartbeat — say which pins in `interpretation.md`."
			title:     "Worker pins"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-RULES-011"
			role:      "rule"
			statement: "A token value MUST NEVER enter `run.json`, chat or any artifact — only its store label; `go run ./cmd/skilleval arm` reads the value itself."
			title:     "Label only"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-RULES-003"
			role:      "rule"
			statement: "Unverifiable values MUST be null with a note, never estimated."
			title:     "Nothing invented"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-RULES-012"
			role:      "rule"
			statement: "A run's `run.json` MUST NEVER be edited after init — every stage refuses a changed one with exit 76; another config is a new run."
			title:     "Frozen run"
		}
	}]
	kind: "context"
}, {
	title: "1. Init"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEEVAL-INTAKE-008"
			items: [{
				label:  "param"
				marker: "*"
				text:   "flags, each only when its arg is given: `-replicas <n>`, `-golden <kind>`, `-set <name>=<value>`; flags before the skill."
			}, {
				label:  "param"
				marker: "*"
				text:   "init checks prerequisites, the golden kind's file, the token label and runs × arm budget against the window."
			}, {
				label:  "param"
				marker: "*"
				text:   "never write or edit a prompt — `go run ./cmd/skilleval compose` with the same flags prints the one a run gets."
			}, {
				label:  "param"
				marker: "*"
				text:   "never stream SKILL.md through Read/Write, never `cp` — the deployed skills dir is a gated path."
			}]
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval init <flags> -token <label> <skill>` — prints the run dir; composes the prompt and freezes the run's config there."
			title:     "Init"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-INTAKE-005b"
			role:      "stop"
			statement: "If init exits 69 ⇒ STOP: the skill has no `eval/definition.json`; name the skill — a definition is authored in its skill dir."
			title:     "No definition"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-INTAKE-012"
			role:      "stop"
			statement: "If init exits 70 ⇒ STOP: list the labels it printed, ask for one; never repeat a value; tokens are saved in the config server's token store."
			title:     "Unknown token"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-INTAKE-006b"
			role:      "stop"
			statement: "If init exits 71 ⇒ STOP: print its math; suggest fewer replicas or env `SKILLEVAL_WINDOW_BUDGET_USD`."
			title:     "Over window"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-INTAKE-014"
			role:      "stop"
			statement: "If init exits 74 ⇒ STOP: the golden is not minted, a test run needs it; offer `golden: produce`."
			title:     "No golden"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-INTAKE-015"
			role:      "stop"
			statement: "If init exits 75 ⇒ STOP: name the arg it printed; only the args in the definition's `invocation.open` take `set:`."
			title:     "Closed arg"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-INTAKE-013"
			role:      "stop"
			statement: "If init exits any other non-zero ⇒ STOP before any run: report its stderr."
			title:     "Init failed"
		}
	}]
}, {
	title: "2. Provision"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEEVAL-ARM-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "worktree root `~/.cache/claude-routine/worktrees`; env `SKILLEVAL_WT_ROOT` overrides — judges then lack the read grant."
			}, {
				label:  "param"
				marker: "*"
				text:   "never `git -C <repo>`, never `cd` — provision owns every git call against `repo`."
			}]
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval provision <run dir>` — resolves `repoName` and refs, creates run worktrees with overlay, writes `reference.patch` and `provision.json`."
			title:     "Provision"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-ARM-008a"
			role:      "stop"
			statement: "If provision exits 67 ⇒ STOP before any run: name the repo; the clone is registered on the config server's Repos page."
			title:     "Unregistered repo"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-ARM-008b"
			role:      "stop"
			statement: "If provision exits 68 or any other non-zero ⇒ STOP before any run: report its stderr."
			title:     "Provision failed"
		}
	}]
}, {
	title: "3. Run"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEEVAL-RUN-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "runner writes every per-run file — stream, trace, logs, candidates, diff, `result.json` — to `runs/<label>/`."
			}, {
				label:  "param"
				marker: "*"
				text:   "run artifacts live in `<worktree>-out/artifacts` while it runs, then move to `runs/<label>/artifacts` — never in the run dir mid-run."
			}]
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval arm <run dir> <label>` once per `run.json` label, each its own background Bash call, all in one message; WAIT for every completion notification."
			title:     "Launch runs"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-RUN-002"
			role:      "verify"
			statement: "After all runs: VERIFY each `runs/<label>/result.json` exists and holds no window stop; rerun a missing label once, resume a window-stopped cell after the reset: `arm -resume <sessionId>`."
			title:     "Run results"
		}
	}]
}, {
	title: "4. Judge"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-JUDGE-003a"
			role:      "rule"
			statement: "Judges MUST get only the brief `skilleval brief` prints — never run ids, models, efforts, keys, telemetry or another judge's output."
			title:     "Blind judges"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-JUDGE-004a"
			role:      "rule"
			statement: "Every judge MUST run on the `run.json` `judge` model, the same for all runs — NEVER inherit a run's model."
			title:     "Pinned judge model"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-JUDGE-001"
			role:      "spawn"
			statement: "SPAWN one judge per label with the brief `go run ./cmd/skilleval brief adherence <run dir> <label>` prints; it writes `runs/<label>/adherence.json`."
			title:     "Adherence judges"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEEVAL-GOLD-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "only when `jq -r .golden.kind <run dir>/run.json` prints `test` — a produce run and a run without a golden have no golden judge."
			}]
			role:      "spawn"
			statement: "SPAWN one judge per label with the brief `go run ./cmd/skilleval brief golden <run dir> <label>` prints; it writes `runs/<label>/judge.json`."
			title:     "Golden judges"
		}
	}]
}, {
	title: "5. Cleanup"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEEVAL-ARM-009"
			items: [{
				label:  "param"
				marker: "*"
				text:   "never forced; a `removed: false` row is reported, never retried, and keeps the out dir with its salvage."
			}]
			role:      "step"
			statement: "After judges finish, RUN `go run ./cmd/skilleval cleanup <run dir> <label>` per label — salvage (worktrees, branch diffs) into `<worktree>-out/salvage/`, unforced removal, out dir moved to trash."
			title:     "Run cleanup"
		}
	}]
}, {
	title: "6. Fold & report"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-FOLD-006b"
			role:      "rule"
			statement: "`interpretation.md` MUST stay human-curatable: in-place edits expected, git history is the log."
			title:     "Interpretation curation"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-FOLD-001"
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval fold <run dir>`; it validates and writes `eval.json`, and writes `report.md`."
			title:     "Fold"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-FOLD-004"
			role:      "stop"
			statement: "If fold exits non-zero ⇒ STOP: report its error and `eval.rejected.json`; never hand-edit eval.json into shape."
			title:     "Fold rejected"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEEVAL-FOLD-006a"
			role:      "step"
			statement: "WRITE `<run dir>/interpretation.md`: bullets only, at most 8, each at most 160 runes, on how this configuration did from the report's cells, worker-pin note as one of them; no heading, no paragraph."
			title:     "Interpretation"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEEVAL-FOLD-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "produce run: name `go run ./cmd/skilleval promote <run dir>` as the way to add its findings to the seed; never run it unasked."
			}]
			role:      "step"
			statement: "PRINT the result table of `report.md` in chat, then STOP for user review."
			title:     "Report"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "runs fixed Go steps and spawns judges on the pinned judge model; one short interpretation"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "qq replicas: 1 token: {token}"
	limits: {
		minutes: 25
		usd:     5.0
	}
}
