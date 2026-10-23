changelog: [{
	date:    "2026-10-21"
	version: "4.15.0"
	text:    "The commit hand-off is the required repo-scoped variable $SKILL_FIMPLEMENT_COMMIT_SKILL (commit-skill@1, example package-commit): COMMIT-002 hands off to it, GROUND-005 preflights it, allowed-tools grants Skill() on it, the smoke sets package-commit; the pipeline edge to package-commit is gone — the skill names no commit skill."
	tokens:  7053
}, {
	date:    "2026-10-17"
	version: "4.14.1"
	text:    "The smoke sets its own CHECK (`go test ./internal/smoke/...`) — a smoke never reads the machine's values."
	tokens:  6995
}, {
	date:    "2026-10-17"
	version: "4.14.0"
	text:    "Dropped the post-implementation UI capture (VERIFY-002a, VERIFY-002b): UI evidence is a design-run artifact; the implemented UI is verified in the running app like any other observable path."
	tokens:  6995
}, {
	date:    "2026-10-17"
	version: "4.13.0"
	text:    "Dropped the caveman arg and its fallback rule — no skill takes caveman as an arg."
	tokens:  7214
}, {
	date:    "2026-10-17"
	version: "4.12.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  7374
}, {
	date:    "2026-10-15"
	version: "4.11.0"
	text:    "Repo-scoped commands run one Bash call per line (IMPL-003a build and tests, COMMIT-002 check); the preamble names the repo's check command instead of the variable."
	tokens:  7476
}, {
	date:    "2026-10-15"
	version: "4.10.0"
	text:    "Enforce anchors on the two title set calls (title_set_wip, title_set_ready) and a requires check ready_after_wip: READY is denied until the intake's WIP set ran, so a skipped intake set is caught instead of leaving the gate state on the title; the title hook renames the turn's first set at the call, so WIP shows while the run works."
	tokens:  7473
}, {
	date:    "2026-10-14"
	version: "4.9.0"
	text:    "Interfaces: one variable per capability — AGENTWT_STATUS, BUILD, CHECK, HEARTBEAT_STATUS, PERMISSION_CHECK, PERMISSION_FREEZE, TEST, TITLE_SET"
	tokens:  7473
}, {
	date:    "2026-10-13"
	version: "4.8.0"
	text:    "Programs and commands are env variables: $SKILL_FIMPLEMENT_CHECK (required, the repo's full gate before the package-commit handoff), _BUILD and _TEST (per-unit gate, replacing the acdsl check/make lint line), _AGENTWT and _HEARTBEAT (re-anchor; the step loses its agentwt tag), _TOOLPOLICY (check preflight, allow ban, freeze anchor), _RULES (title set) in steps, grants and enforce patterns; the sync_public.sh verify is deleted; a bare invocation's plan is the gate report's only when a report arrived."
	tokens:  7457
}, {
	date:    "2026-10-12"
	version: "4.7.0"
	text:    "Title states are set explicitly: intake runs `smine-rules title set WIP` so the approval gate's state leaves the session title at the turn end, the report step `smine-rules title set READY`; grant Bash(smine-rules title *)."
}, {
	date:    "2026-10-10"
	version: "4.6.0"
	text:    "The per-unit gate and the phase gate run the repo's lint check beside build and tests — `make acdsl ARGS=\"check\"` in ACDSL repos, else `make lint` — so lint findings in plan-copied code are fixed in the unit that wrote them, not first at the final audit (sessions 78d4af15, e59381f7, 70993e5d, 3ea92d5c)."
}, {
	date:    "2026-10-10"
	version: "4.5.0"
	text:    "The freeze ban is a heartbeat check: an enforce block denies every `toolpolicy freeze` call of the run (root or subagent) with a recovery pointing at the hook's approval and Permissions lines; the Frozen rules entry covers the whole run, intake and ground included, not only workers (sessions e59381f7, 6a6d6a88, 78d4af15 froze after a typed /fimplement). A Skill(fimplement) call from a subagent opens no heartbeat run, so there the ban stays prose."
}, {
	date:    "2026-10-06"
	version: "4.4.4"
	text:    "The re-anchor reads the landing target as `branches[0].target` of one `smine-heartbeat status --worktree . --format json` call, with no jq, substitution or show-toplevel step."
}, {
	date:    "2026-10-06"
	version: "4.4.3"
	text:    "The permission preflight names the check's answer shape: the refused call's `tool-policy: CHECK` reason with one verdict line per target is the answer, never retried."
}, {
	date:    "2026-10-06"
	version: "4.4.2"
	text:    "Pipeline declares spawnedBy smine — the smine implement stage runs one implementer subagent per change task through this skill."
	tokens:  7169
}, {
	date:    "2026-10-05"
	version: "4.4.1"
	text:    "The phase fan-out's plan_phased atom is declared as a session input."
	tokens:  7169
}, {
	date:    "2026-10-04"
	version: "4.4.0"
	text:    "A typed /fimplement at an open code gate approves it: the hook resolves the plan the session wrote last and freezes it; the report's approval line is that approval — implement, never ask for it again."
	tokens:  7169
}, {
	date:    "2026-10-04"
	version: "4.3.0"
	text:    "UI capture runs smine-rules capture-ui instead of fdesign's bundled script; dropped the serena arg — Serena support removed repo-wide, goland stays the one IDE opt-in."
	tokens:  7120
}, {
	date:    "2026-10-03"
	version: "4.2.0"
	text:    "gate and agentwt steps and the restack edge tagged with needs: partial installs and the bundle drop them. References reworded to name no smine program."
	tokens:  7221
}, {
	date:    "2026-10-03"
	version: "4.1.0"
	text:    "Delegation removed with the delegate skill and the implement-runner agent: the Delegation section, the gated class and runner, the delegated re-anchor param and the spawnedBy delegate edge are gone; Discipline by mode moves to Deviations."
	tokens:  7225
}, {
	date:    "2026-10-03"
	version: "4.0.0"
	text:    "Removed every stop: the run never halts, refuses or hands back — an obstacle gets the closest workable call and a report line (Never halt). Dropped the two-attempt ceiling, the tier denial, the section-check, caveman, touch-point and re-anchor stops, relay-class gates and every blocked result, the behind-target one included; Stop conditions section renamed Deviations."
	tokens:  7799
}, {
	date:    "2026-10-03"
	version: "3.16.0"
	text:    "Re-anchor target: the heartbeat row's target, else the base from smine-agentwt status when it is a local branch neither agent-owned nor the default branch; a branch without a target is not re-anchored and the run continues. The develop fallback is gone."
	tokens:  9155
}, {
	date:    "2026-10-03"
	version: "3.15.0"
	text:    "Plan stage removed: no stop for a missing code-stage plan — any given plan, or the one the hook names, is implemented; no plan and no task goes best-effort on the stated task. --adhoc, the Ad-hoc run rule and the delegated no-code-stage-plan block deleted."
	tokens:  9069
}, {
	date:    "2026-10-02"
	version: "3.14.0"
	text:    "Re-anchor: the ground phase first resolves the landing target and calls the restack skill when the branch is behind it; a stopped restack stops the run; a delegated run that is behind returns blocked `branch behind <target>`. allowed-tools gains Skill(restack) and the two status commands; the pipeline gains the escalates edge to restack."
	tokens:  9525
}, {
	date:    "2026-10-02"
	version: "3.13.1"
	text:    "Dropped the help-only Eval section — --help shows the smoke and eval declared beside the skill."
	tokens:  9057
}, {
	date:    "2026-10-02"
	version: "3.13.0"
	text:    "Pending-addenda rule removed: the plan's one Changes section is the work. Intake runs the section check on a plan-dir plan and stops on a violation. Stops and routing name `/fdesign <plan file>`; the edge to fdesign carries no route."
	tokens:  9057
}, {
	date:    "2026-10-01"
	version: "3.12.2"
	text:    "Pipeline declares spawnedBy /orchestrate — the feature playbook's workers."
	tokens:  9023
}, {
	date:    "2026-10-01"
	version: "3.12.1"
	text:    "Gaps never halt no longer claims to override the implementing chapter: the chapter itself now says continue with best effort."
	tokens:  9016
}, {
	date:    "2026-10-01"
	version: "3.12.0"
	text:    "Permission rules come from the plan's frozen record: the hook applies it in any worktree, a plan that is not frozen is noted in the report without halting the run, and permissions.json is never written outside a design run."
	tokens:  9014
}, {
	date:    "2026-10-01"
	version: "3.11.0"
	text:    "Permission gaps never halt a run: a missing hook report, a rejected row, a denied preflight target or a mid-unit denial each continue the run — the denied command is skipped, its candidate rule reported; never ask, never self-widen."
	tokens:  8918
}, {
	date:    "2026-09-30"
	version: "3.10.1"
	text:    "Plan read list follows the re-cut catalog (Contracts, hot: and ui: lines, Verification table); the verification-gap entries read the table."
	tokens:  8762
}, {
	date:    "2026-09-29"
	version: "3.10.0"
	text:    "Without a code-stage plan (no plan, or stage: design) fimplement stops and asks; --adhoc goes best-effort without asking and binds a design-stage plan's decisions; phased plans run one subagent per entry per phase; a delegated run without a code-stage plan returns blocked unless --adhoc."
}, {
	date:    "2026-09-29"
	version: "3.9.0"
	text:    "Plans carry no Stop conditions section: dropped from the plan read list and the reverse-pass review; new Minor adjustment rule — a deviation touching no Decision, contract or Scope item is made at once and logged as a local: Changelog row; contract and rework stops route to /fdesign refine."
}, {
	date:    "2026-09-29"
	version: "3.8.0"
	text:    "A plan with Addenda implements only the addenda the base branch lacks; argument-hint uses design/<slug>.md."
}, {
	date:    "2026-09-28"
	version: "3.7.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.7.0"
	text:    "Every /fimplement invocation, typed or Skill-chained, gets its plan's Permissions rows from the tool-policy hook; fimplement verifies the report and never runs toolpolicy allow for plan rows; toolpolicy check preflight before the first unit."
}, {
	date:    "2026-09-28"
	version: "3.6.3"
	text:    "Eval examples name the task by its repo-relative plan path — no main-checkout absolute path the fence would deny."
}, {
	date:    "2026-09-28"
	version: "3.6.2"
	text:    "Removed the bundled config-ui-fidelity-gate workflow: no step ever invoked it; UI evidence stays the plan-time screenshot and the running-app capture."
}, {
	date:    "2026-09-27"
	version: "3.6.1"
	text:    "Bracket and arm tokens use <model>:<effort>"
}, {
	date:    "2026-09-26"
	version: "3.6.0"
	text:    "Hand off commit: /package-commit now only commits — the green audit before it is the validation; COMMIT-002 no longer promises a build and test."
}, {
	date:    "2026-09-26"
	version: "3.5.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-26"
	version: "3.4.0"
	text:    "Added the help-only ## Eval section: /skillroutine-eval model-compare and serena-vs-grep invocations, shown by --help, stripped on deploy."
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "Dropped the stale /verify delegation (the skill is disabled): the running-app check runs directly through an admitted path. Gate before advance is per coherent unit group, config and doc units ride the final audit."
}, {
	date:    "2026-09-26"
	version: "3.2.3"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "3.2.2"
	text:    "Permissions are applied with one toolpolicy allow call that the gate writes itself; the agent never edits settings.local.json."
}, {
	date:    "2026-09-25"
	version: "3.2.1"
	text:    "Permission denials: route around with admitted means first (never by defeating the gate); the permission-gap stop fires only when no admitted path exists."
}, {
	date:    "2026-09-25"
	version: "3.2.0"
	text:    "Phase 1 applies the plan's Permissions rows to the session's settings.local.json in one self-service edit; a later permission gap is a stop."
}, {
	date:    "2026-09-25"
	version: "3.1.1"
	text:    "Make-only entry points: prose calls make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind lines (Delegation, Stop conditions marked context), freeform bullets folded into * param: register, pred/post clauses on 33 of 57 coverable entries (6 invariants, 23 guarded transitions) backed by new atoms.json (30 bool + 2 int atoms), three committed queries under queries/, caveman-compressed prose. skillfmt/skillmodel check/consistency/queries all green."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-08"
	version: "2.1.0"
	text:    "Added the target-tier check at intake (compares the running model against a plan's recorded target tier, warning one step below and denying two or more) and handoff-rendering intake (build-sequence order, final-state replacement, known-red ledger, forbidden fixes); added stop 11 (escalation trigger under a handoff rendering) and extended the relay-class gate set to include it"
}, {
	date:    "2026-09-08"
	version: "2.0.1"
	text:    "Phase 3 Verify: when the risk locus is a headless/background/scheduled path the happy-path check can't reach, execute that exact path once with real inputs before done — a stub, proxy render, or reasoned-not-run claim is reported unverified, not green (VERIFY-007a/007b)"
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format v2 reformat (/fmt skill v2) — Routing section, Args table, argument-hint example, allowed-tools normalized, enumeration fenced, caveman-compressed prose; content identical"
}, {
	date:    "2026-09-05"
	version: "1.32"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-04"
	version: "1.31"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not implementation input"
}, {
	date:    "2026-09-04"
	version: "1.30"
	text:    "Verification of UI-touching changes ends with a real capture via fdesign's capture-ui.sh into design/ui/ (visual-evidence rules in rules/plan.md)"
}, {
	date:    "2026-09-04"
	version: "1.29"
	text:    "acdsl-context narrowed: concept-chapter entries dropped (hot-class gates bind at design time) and the review chapter cut to the end-to-end verification entries this skill's own phases run; chapter list in the body updated to match"
}, {
	date:    "2026-09-01"
	version: "1.28"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.27"
	text:    "Runbook persistence path migrated to the dated plan-dir convention (RULE-PLAN-072)"
}, {
	date:    "2026-08-23"
	version: "1.26"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.25"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-23"
	version: "1.24"
	text:    "acdsl-context: key rename — the ACDSL declaration key moved off Claude Code's native context: frontmatter field; prose references updated"
}, {
	date:    "2026-08-21"
	version: "1.23"
	text:    "bundled workflow config-ui-fidelity-gate.js — render (throwaway harness) + screenshot + design-system diff + single-Save-button assertion, gating the first commit of any config-server template/CSS change"
}, {
	date:    "2026-08-21"
	version: "1.22"
	text:    "Phase 3 verify asserts a live listener on any host:port export target an env var points at before done — an exported telemetry endpoint nothing binds sends its data nowhere"
}, {
	date:    "2026-08-20"
	version: "1.21"
	text:    "Phase 3 verify asserts a relocated data directory is still matched by a sync_public.sh --exclude before done — guards against publishing private data when a dir moves out from under an excluded parent"
}, {
	date:    "2026-08-20"
	version: "1.20"
	text:    "description compressed under the brevity caps"
}, {
	date:    "2026-08-18"
	version: "1.19"
	text:    "reference rename — fchange → fdesign change (fchange retired into the fdesign change route)"
}, {
	date:    "2026-08-13"
	version: "1.18"
	text:    "Phase-4 done-gate — stop all session-started background processes (dev servers, listeners, watchers) and verify no orphaned port listeners remain before offering to commit"
}, {
	date:    "2026-08-12"
	version: "1.17"
	text:    "migrated to the context: declaration — doctrine, commit-format, and fact entries injected at invocation by the skill-context hook; SessionStart-injection wording retired"
}, {
	date:    "2026-08-11"
	version: "1.16"
	text:    "Test runbook execution is opt-in — runs only when the plan carries full request files (args line runbook) or the user asks; a scenario-index-only section executes nothing; context-path naming corrected (rules/commits.md, doctrine chapters under actions/)"
}, {
	date:    "2026-08-10"
	version: "1.15"
	text:    "reframed as the contract-optional implementation skill — a plan is honored as a binding contract when present, else best-effort under context + code-style; a missing/contract-light input no longer STOPs (stop 7 inverted, 9 softened); the invocation is the tracked start-of-implementation signal"
}, {
	date:    "2026-08-09"
	version: "1.14"
	text:    "ACDSL is the referenced context system — SessionStart delivery wording; green acdsl check (and -lifetime task for contracts) is part of done"
}, {
	date:    "2026-07-31"
	version: "1.13"
	text:    "hot-class gates named under rules/concepting.md"
}, {
	date:    "2026-07-31"
	version: "1.12"
	text:    "activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries"
}, {
	date:    "2026-07-30"
	version: "1.11"
	text:    "context redesign — stop-conditions/hot-items via ../fdesign/assets/, commits via ../package-commit/assets/, doctrine entries under rules/"
}, {
	date:    "2026-07-30"
	version: "1.10"
	text:    "allowed-tools mechanics manifest declared; Command surface line retired"
}, {
	date:    "2026-07-30"
	version: "1.9"
	text:    "reference rename — design-refine → the fdesign refine route"
}, {
	date:    "2026-07-30"
	version: "1.8"
	text:    "reference rename — per-package-commit → package-commit"
}, {
	date:    "2026-07-26"
	version: "1.7"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "1.6"
	text:    "renamed feature-implement → fimplement (leaf dir, frontmatter name, H1); live references swept repo-wide"
}, {
	date:    "2026-07-24"
	version: "1.5"
	text:    "delegation explicit-only via /delegate — auto-intake check removed, relay procedure moved to the delegate skill, relay data kept in ## Delegation"
}, {
	date:    "2026-07-24"
	version: "1.4"
	text:    "Test runbook joins intake reading; runbook files persisted to plans/<feature>/runbooks/ and executed in verification"
}, {
	date:    "2026-07-24"
	version: "1.3"
	text:    "reference merge: feature-refine + feature-refactor → feature-change"
}, {
	date:    "2026-07-22"
	version: "1.2"
	text:    "gated delegation via implement-runner: intake check, gate relay + blocked-state handling, Model classification, effort medium"
}, {
	date:    "2026-07-19"
	version: "1.1"
	text:    "reference rename: refactor → feature-refactor; moved under skills/feature/"
}, {
	date:    "2026-07-19"
	version: "1.0"
	text:    "initial version"
}]
env: {
	AGENTWT_STATUS: {
		interface: "agentwt-status@1"
	}
	BUILD: {
		interface: "build@1"
	}
	CHECK: {
		interface: "check@1"
		required:  true
	}
	COMMIT_SKILL: {
		interface: "commit-skill@1"
		required:  true
	}
	HEARTBEAT_STATUS: {
		interface: "heartbeat-status@1"
	}
	PERMISSION_CHECK: {
		interface: "permission-check@1"
	}
	PERMISSION_FREEZE: {
		interface: "permission-freeze@1"
	}
	TEST: {
		interface: "test@1"
	}
	TITLE_SET: {
		interface: "title-set@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " fimplement"
}, {
	key:   "description"
	value: " Carry a change to working, committed code — an /fdesign plan or findings doc binds as contract; without one it goes best-effort; never halts. Trigger on /fimplement [plan file or task]. Args — plan file: plan, findings doc, or task (absent = the hook-named plan); errata: invocation feedback superseding the plan."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.15.0"
}, {
	key:   "title-code"
	value: " FImpl"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " 'plans/2026-09-07-foo-abc1234/2026-09-07-foo-wip-abc1234/design/foo.md errata: \"skip step 3, already applied\"'"
}, {
	key:   "acdsl-context"
	value: " ACTION-IMPL-*, ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-VERIFY-*, RULE-COMMIT-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Edit, Read, Skill($SKILL_FIMPLEMENT_COMMIT_SKILL), Skill(restack), Write, Bash($SKILL_FIMPLEMENT_AGENTWT_STATUS status *), Bash($SKILL_FIMPLEMENT_HEARTBEAT_STATUS status *), Bash($SKILL_FIMPLEMENT_TITLE_SET title *), Bash(git add *), Bash(git commit -m *), Bash(git diff *), Bash(git log *), Bash(gofmt *), Bash(go build *), Bash(go test *), Bash(go vet *), Bash(jq *)"
}]
preamble: """
	# fimplement

	Carry change to working, committed code.

	Marked entrypoint for "implementation started" — session minable later for what got built. Approved plan present: plan binds
	as contract down to exact signatures, contracts, architecture — no redesign here. No plan: best-effort under repo context and
	code-style — proceed and adhere.
	Run never halts, refuses, or hands back — an obstacle gets the closest workable call and a report line, never a silent
	deviation.

	Static constraints not restated here: `AGENTS.md`, format guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (incl.
	`commits.md`), doctrine chapters under `$AGENT_CONTEXT_DIR_DEFAULT/actions/` (implementing — stops, integrity; reviewing —
	end-to-end verification) govern every edit. Arrive via this skill's frontmatter `acdsl-context:` declaration, injected at
	invocation by skill-context hook; per-language style guides read from `$AGENT_CONTEXT_DIR_DEFAULT/rules/` on demand. Re-read
	entries from `$AGENT_CONTEXT_DIR_DEFAULT/` only when no longer in context. Checkable slice enforced by the repo's check
	command — green before the handoff; its ACDSL gates included where the repo has them. Gate covers it, gate rules — not
	prose.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "fdesign"
				}, {
					kind:     "from"
					skill:    "overlap"
					optional: true
				}, {
					kind:     "from"
					skill:    "code-verdict"
					optional: true
				}, {
					kind:     "to"
					skill:    "fdesign"
					optional: true
				}, {
					kind:  "spawnedBy"
					skill: "fleet"
				}, {
					kind:  "spawnedBy"
					skill: "smine"
				}, {
					kind:  "escalates"
					skill: "restack"
					needs: ["agentwt"]
				}]
				note: "the commit hand-off goes to the skill the commit-skill variable names"
			}
			preconditions: ["none required — a plan sharpens the target but is optional"]
			purpose: "turns any change into working, committed code — binding-contract mode with a plan; without one best-effort; never halts."
			routes: [{
				for: "reviewed design before any code"
				use: "/fdesign"
			}, {
				for: "plan needs revising first"
				use: "/fdesign <plan file> (optional)"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "plan file"
				default: "the plan the tool-policy hook names, else the stated task"
				effect:  "plan (fdesign), findings doc, or task, pasted or by path; binds as contract when it defines one — see [Any input accepted](#skill-fimplement-intake-001)"
				form:    "free text"
			}, {
				name:    "errata"
				default: "absent"
				effect:  "feedback/corrections at invocation, supersede the plan, `[USER]` decisions included — see [Errata supersede plan](#skill-fimplement-intake-004)"
				form:    "free text"
			}]
		}
	}]
}, {
	title: "Phase 0 — Intake"
	blocks: [{
		entry: {
			id: "SKILL-FIMPLEMENT-INTAKE-002b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "files persist to plan dir's `runbooks/` (per plan-dir resolution rule), run as part of verification."
			}, {
				label:  "param"
				marker: "*"
				text:   "scenario-index-only section is informational — executes nothing."
			}]
			pred:      "runbook_ran => runbook_opted_in"
			role:      "rule"
			statement: "Test runbook MUST run only when opted in — plan args line shows `runbook`, or user asks at implementation."
			title:     "Runbook opt-in"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-INTAKE-002c"
			role:      "rule"
			statement: "A review-findings doc MUST give one unit per confirmed finding (file:line + proposed fix); rejected or ledgered findings stay out of scope."
			title:     "Findings doc units"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-INTAKE-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "/fdesign <plan file> stays available when you'd rather resolve them as design first."
			}]
			role:      "rule"
			statement: "Open questions or OPEN rows in a provided plan MUST NOT stop the run — implement what's settled, make the reasonable call on the rest guided by context, note each call in the report."
			title:     "Open rows OK"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-INTAKE-004"
			role:      "rule"
			statement: "User errata MUST supersede the plan, `[USER]` decisions included — take the errata, note each superseded decision in the report."
			title:     "Errata supersede plan"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-INTAKE-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "units applied in Build-sequence order; modifications by final-state replacement of the replaces: span."
			}, {
				label:  "param"
				marker: "*"
				text:   "red gate covered by Known-red ledger gets exactly the sanctioned fix."
			}, {
				label:  "param"
				marker: "*"
				text:   "forbidden fixes are absolute."
			}, {
				label:  "param"
				marker: "*"
				text:   "best-effort latitude doesn't exist under a rendering."
			}]
			role:      "rule"
			statement: "A handoff rendering (fdesign handoff route) MUST bind tighter than a plan:"
			title:     "Handoff rendering input"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-INTAKE-009"
			role:      "rule"
			statement: "A provided plan with no `/fimplement` hook report in context MUST NOT stop the run — implement anyway, note \"fimplement hook did not fire\" in the report."
			title:     "Hook report missing"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-INTAKE-010"
			role:      "rule"
			statement: "A hook report applying nothing (not frozen, rule rejected) or leaving the plan unresolved MUST NOT stop the run — implement the plan the args name, quote the hook line in the report."
			title:     "Rules not applied"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-INTAKE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "bare invocation — plan is the one the gate's `/fimplement` report names, when a report arrived; none — stated task, best-effort, calls noted in report."
			}, {
				label:  "param"
				marker: "*"
				text:   "hook report carries `approval:` — this typed prompt approved the plan's open code gate and the gate froze it; never ask for approval again."
			}, {
				label:  "param"
				marker: "*"
				text:   "bounded by context docs (AGENTS.md, `$AGENT_CONTEXT_DIR_DEFAULT/rules/`, style guides), nearest in-repo patterns."
			}, {
				label:  "param"
				marker: "*"
				text:   "RUN `$SKILL_FIMPLEMENT_TITLE_SET title set WIP` first — the session title leaves the approval gate state at the turn end."
			}]
			role:      "step"
			statement: "TAKE whatever's given — an /fdesign plan, a review-findings doc, or a bare task."
			title:     "Any input accepted"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-INTAKE-002a"
			items: [{
				fence: {
					body: """
						Decisions (binding, `[USER]` rows especially)
						the hook report's Permissions and checked: lines — the plan's frozen rules
						Changes entries' hot: and ui: lines
						Contracts
						Tests
						Verification (table — the dev stack and live rows)
						Test runbook

						"""
					info:   "text"
					marker: "```"
				}
			}]
			pred:      "plan_provided"
			role:      "step"
			statement: "READ the whole plan first when present, not just Changes — these sections travel into implementation, bound it:"
			title:     "Read whole plan"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-INTAKE-008"
			needs: ["gate"]
			pred:      "plan_provided"
			role:      "verify"
			statement: "VERIFY hook report's Permissions line names the plan's frozen rules; never widen permissions through the gate's allow command for them — the gate applies the record; a mismatch goes in the report."
			title:     "Hook applied record"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-INTAKE-006a"
			pred:      "target_tier_known and tier_gap >= 1"
			role:      "verify"
			statement: "VERIFY session's model against input's target-tier line (rules/plan.md); at/above target proceed silently, below target warn once and proceed, no line means no check."
			title:     "Target-tier check"
		}
	}]
}, {
	title: "Phase 1 — Ground"
	blocks: [{
		entry: {
			id:        "SKILL-FIMPLEMENT-GROUND-005b"
			role:      "rule"
			statement: "Denied call MUST be routed around with admitted means first (allowlisted form, make target, read-only form, file tool); NEVER by defeating gate — script-hidden command, shell-written settings."
			title:     "Route around denials"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-005d"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "every phase — preflight, units, gates, verification, commit; a denied check leaves its row unverified in the report, never halts."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "same line as the implementing chapter's continue-with-best-effort entry on missing permission rules."
			}]
			role:      "rule"
			statement: "A permission gap MUST NOT stop or pause the run, ask the user, or widen permissions — no permission-widening command, no `#allow` request; work continues until only denied commands remain."
			title:     "Gaps never halt"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-006b"
			needs: ["agentwt"]
			role:      "rule"
			statement: "A stopped restack MUST NOT stop the run — implement on the current base, relay restack's reason in the report."
			title:     "Re-anchor failed"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-006a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "target: `branches[0].target` of `$SKILL_FIMPLEMENT_HEARTBEAT_STATUS status --worktree . --format json`."
			}, {
				label:  "param"
				marker: "*"
				text:   "read the JSON in the tool result; never pipe it to jq."
			}, {
				label:  "param"
				marker: "*"
				text:   "no answer/row: `from.name` of HEAD's `rows[]` entry in `$SKILL_FIMPLEMENT_AGENTWT_STATUS status --format json`, when a valid base."
			}, {
				label:  "param"
				marker: "*"
				text:   "valid base: a local branch (`git rev-parse --verify refs/heads/<name>`) with no `rows[]` entry of its own, not `repo.default_branch`."
			}, {
				label:  "param"
				marker: "*"
				text:   "no target (row without `target`, or none by the fallback): no re-anchor, the run continues."
			}, {
				label:  "param"
				marker: "*"
				text:   "a heartbeat that does not answer never stops the run."
			}]
			role:      "step"
			statement: "RESOLVE the landing target first; when `git merge-base --is-ancestor <target> HEAD` fails, CALL Skill `restack` with `onto: <target>` before any other step reads the tree."
			title:     "Re-anchor"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-GROUND-001"
			role:      "step"
			statement: "RE-CHECK every claim against the real tree before writing anything — Explore and subagent findings re-verified against actual files, never trusted as-is."
			title:     "Re-verify against tree"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-002a"
			post: ["touch_points_verified"]
			pred:      "plan_provided"
			role:      "verify"
			statement: "VERIFY the touch points — with a plan, every file, symbol, prerequisite it names exists as described (path:line); a claim that doesn't hold is re-grounded against the tree, noted in the report."
			title:     "Verify touch points"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-002b"
			post: ["touch_points_verified"]
			pred:      "not plan_provided"
			role:      "step"
			statement: "LOCATE the code the task targets, nearest sibling patterns directly when no plan exists; ground the change against those."
			title:     "Ground w/o plan"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-003"
			post: ["prerequisites_verified"]
			role:      "verify"
			statement: "VERIFY prerequisites present — generated code, migrations, running infra the plan assumes; missing producer step runs; down infra is never started — its checks go unverified in the report."
			title:     "Prerequisites present"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-GROUND-004"
			role:      "verify"
			statement: "VERIFY nearest sibling the plan points to still exists for each new unit; new code mirrors that sibling's architecture (AGENTS.md rule)."
			title:     "Exemplar per unit"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-GROUND-005a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "targets — hook report's `checked:` line plus its rules plus any grounding adds; no plan — from grounded units."
			}, {
				label:  "param"
				marker: "*"
				text:   "answer: refused call's reason opening `tool-policy: CHECK`, one `N. <target> → admitted|denied: …` line per target; that is the answer, never retry."
			}]
			needs: ["gate"]
			post: ["permission_preflight_done"]
			role:      "step"
			statement: "RUN `$SKILL_FIMPLEMENT_PERMISSION_CHECK check --skill fimplement --skill $SKILL_FIMPLEMENT_COMMIT_SKILL '<target>' …` once over every command units and verification run, before first unit."
			title:     "Permission preflight"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-GROUND-005c"
			pred:      "preflight_denied"
			role:      "step"
			statement: "PROCEED past a denied target no admitted form reaches: implement every unit anyway, skip only that command, list its candidate rule in the final report."
			title:     "Permission gap"
		}
	}]
}, {
	title: "Phase 2 — Implement"
	blocks: [{
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-003b"
			pred:      "not checklist_reviolated"
			role:      "rule"
			statement: "Plan's test spec (per-case fields as struct members, run loop call+assert only) MUST be honored; standing plan-review checklist code rules MUST NOT be re-violated in the implementation."
			title:     "Test spec honored"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-005b"
			role:      "rule"
			statement: "Pre-enumeration theater MUST NOT happen where the compiler already knows the touch list; intermediate commits need not each build if the branch ends green."
			title:     "No pre-enumeration theater"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-007"
			role:      "rule"
			statement: "A mid-unit denial with no admitted form MUST NOT stop the unit — skip that command, finish the rest, record its candidate rule; NEVER retried, reshuffled, or self-widened."
			title:     "Mid-unit denial"
		}
	}, {
		entry: {
			id:      "SKILL-FIMPLEMENT-IMPL-009"
			channel: "hook"
			items: [{
				label:  "Hook"
				marker: "*"
				text:   "cmd/heartbeat/src/enforce.rs"
			}]
			needs: ["gate"]
			role:      "rule"
			statement: "The run MUST NOT write a plan dir's `permissions.json` or run `$SKILL_FIMPLEMENT_PERMISSION_FREEZE freeze`, intake and ground included — a plan's rules change only in a design run the user starts."
			title:     "Frozen rules"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-IMPL-008"
			items: [{
				label:  "param"
				marker: "*"
				text:   "subagent gets its entry and the plan file path; model `opus`; edits only its entry's files, never commits."
			}, {
				label:  "param"
				marker: "*"
				text:   "a one-entry phase runs inline; after each phase run the build/test/lint gate, then the next phase."
			}]
			pred:      "plan_phased"
			role:      "spawn"
			statement: "FOR each phase of the unit being implemented, in order: SPAWN one subagent per entry of the phase, wait for all of them, then gate the phase."
			title:     "Phase fan-out"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-001"
			pred:      "touch_points_verified and prerequisites_verified"
			role:      "step"
			statement: "BUILD in dependency order, one logical unit at a time — never a godfile, \"each endpoint, one at a time.\" Per unit:"
			title:     "One unit only"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-002"
			pred:      "touch_points_verified"
			role:      "step"
			statement: "WRITE the unit to the plan's exact signature and contract when a plan defines one, else to the shape surrounding code and context dictate; new code mirrors its nearest sibling."
			title:     "Write to contract"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-IMPL-003a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "build is `$SKILL_FIMPLEMENT_BUILD` — one Bash call per line; findings fixed in the unit that wrote them, before advancing."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "tests `$SKILL_FIMPLEMENT_TEST` — one Bash call per line; same rule."
			}]
			post: ["gate_green"]
			role:      "verify"
			statement: "VERIFY build and tests green per coherent unit group before moving on — a unit that can't build alone joins its group; config and doc units ride the final check."
			title:     "Gate before advance"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-004"
			pred:      "gate_green"
			role:      "step"
			statement: "ADVANCE to the next unit in dependency order."
			title:     "Advance"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-IMPL-005a"
			role:      "step"
			statement: "DRIVE refactor cascades and renames in compiled code with the compiler: change the core type/signature → build → fix each reported error → test → green."
			title:     "Compiler-driven cascades"
		}
	}]
}, {
	title: "Phase 3 — Verify"
	blocks: [{
		entry: {
			id:        "SKILL-FIMPLEMENT-VERIFY-001b"
			role:      "def"
			statement: "Happy path plus the plan's edge and error cases, exercised against a running system with real values."
			title:     "End-to-end verification"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-VERIFY-007b"
			pred:      "not proxy_claim_used"
			role:      "rule"
			statement: "A stubbed dry run, proxy render check, or reasoned-not-run claim over the risk locus MUST NOT satisfy the Definition of Done — report it unverified, not green."
			title:     "No proxy claim"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-VERIFY-001a"
			post: ["observable_path_verified"]
			role:      "verify"
			statement: "VERIFY the observable path in the running app, not only unit tests — Definition of Done's End-to-End Verification."
			title:     "Verify observable path"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-VERIFY-003"
			role:      "step"
			statement: "RUN the running-app check directly through an admitted path; project needs a local e2e stack first, /dev-stack stands it up — this skill never stands up infrastructure itself."
			title:     "Run running-app check"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-VERIFY-005a"
			pred:      "telemetry_var_exported"
			role:      "verify"
			statement: "Before reporting done: VERIFY something listening on the `host:port` an env var exports to — exported telemetry/metrics endpoint nothing binds sends its data nowhere."
			title:     "Export must listen"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-VERIFY-005b"
			post: ["port_listening_verified"]
			pred:      "telemetry_var_exported"
			role:      "step"
			statement: "PROBE the port (bind check / connection attempt) as part of running-app verification, not just that the var's set."
			title:     "Probe the port"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-VERIFY-006a"
			pred:      "observable_path_verified"
			role:      "verify"
			statement: "VERIFY every row of the plan's Verification table is exercised by a test or run; report any gap."
			title:     "Report verification gap"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-VERIFY-007a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "examples — a headless `claude -p` route, a background-reinvoke hinge, a scheduled/unattended path."
			}]
			post: ["risk_locus_verified"]
			pred:      "risk_locus_unreachable"
			role:      "verify"
			statement: "VERIFY an unreachable risk locus — a path the happy-path route can't exercise — by executing that exact path once with real inputs before done."
			title:     "Unreachable risk locus"
		}
	}]
}, {
	title: "Phase 4 — Commit"
	blocks: [{
		entry: {
			id:        "SKILL-FIMPLEMENT-COMMIT-001"
			role:      "step"
			statement: "STOP every background process this session started — dev servers, port listeners, watchers — before offering to commit; verify no orphaned listeners remain on the ports used."
			title:     "Stop background processes"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-COMMIT-002"
			role:      "step"
			statement: "HAND OFF the commit to /$SKILL_FIMPLEMENT_COMMIT_SKILL through the Skill tool after `$SKILL_FIMPLEMENT_CHECK` is green — one Bash call per line; this skill never hand-rolls a commit."
			title:     "Hand off commit"
		}
	}, {
		entry: {
			id: "SKILL-FIMPLEMENT-COMMIT-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "RUN `$SKILL_FIMPLEMENT_TITLE_SET title set READY` before the report — the session title shows the change awaits review at the turn end."
			}]
			role:      "step"
			statement: "REPORT against the plan — each unit built, its gate result, the running-app verification outcome, every deviation and unverified row with its reason."
			title:     "Report against plan"
		}
	}]
}, {
	title: "Deviations"
	blocks: [{
		entry: {
			id: "SKILL-FIMPLEMENT-DEVIATE-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "plan defect, unhonorable contract, architecture rework, scope overrun, failed attempts, verification gap."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "under a handoff rendering an Escalation-contract trigger is a report line with the observed state."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "the [implementing chapter]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md)'s stop-and-report and ask-first entries are report lines here."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "a deviation from a Decision, contract or Scope item is logged as a `local:` Changelog row."
			}]
			role:      "rule"
			statement: "The run NEVER halts, refuses, or hands back mid-work — an obstacle gets the closest workable call to the plan, remaining units are finished, each deviation is named in the report."
			title:     "Never halt"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-DEVIATE-002"
			pred:      "not contract_violated"
			role:      "rule"
			statement: "This skill NEVER substitutes a different mechanism, demotes a `[USER]` decision to \"deferred,\" or invents architecture mid-edit while the plan's contract can hold."
			title:     "No substitution"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-DEVIATE-003"
			role:      "rule"
			statement: "With no plan, the task's stated intent MUST be the scope."
			title:     "No-plan scope"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-DEVIATE-006"
			pred:      "(plan_provided => binding_contract_mode) and (not plan_provided => not binding_contract_mode)"
			role:      "rule"
			statement: "Discipline MUST follow mode — with a plan, binding-contract rules (contract, exact signatures, no silent deviation); without one, best-effort under the context docs and code-style."
			title:     "Discipline by mode"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-DEVIATE-004"
			role:      "rule"
			statement: "A deviation changing no Decision, no signature or contract in Changes, no Scope In/Out item MUST be made immediately and logged as a `local:` Changelog row — never asked."
			title:     "Minor adjustment"
		}
	}, {
		entry: {
			id:        "SKILL-FIMPLEMENT-DEVIATE-005"
			pred:      "plan_provided"
			role:      "verify"
			statement: "VERIFY implementation stays within a provided plan's scope; a self-contained necessary adjacent change is made and logged as a `local:` Changelog row."
			title:     "Scope check"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "implementation discipline — contract-honoring when a plan exists, context-and-style-guided when not — plus multi-unit builds and compiler-driven cascades"
			tier:   "frontier"
			notes: ["The allowed-tools manifest covers mechanics only — target-repo payload commands still prompt; a full promptless run isn't promised."]
		}
	}]
}]
atoms: {
	plan_phased: {
		sort:   "state"
		source: "session"
	}
}
enforce: {
	anchors: {
		freeze: {
			agent: "any"
			input: [{
				field:   "command"
				pattern: "(^|/)$SKILL_FIMPLEMENT_PERMISSION_FREEZE freeze( |$)"
			}]
			tool: "^Bash$"
		}
		title_set_ready: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_FIMPLEMENT_TITLE_SET title set READY$"
			}]
			tool: "^Bash$"
		}
		title_set_wip: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_FIMPLEMENT_TITLE_SET title set WIP$"
			}]
			tool: "^Bash$"
		}
	}
	checks: {
		no_freeze: {
			after:    "freeze"
			kind:     "requires"
			on:       "freeze"
			reason:   "fimplement: never run `$SKILL_FIMPLEMENT_PERMISSION_FREEZE freeze` — a typed /fimplement at the open code gate is the approval and the gate already froze the plan; a plan's rules change only in a design run"
			recovery: "heartbeat runtime ({finding}): do not freeze; read the `tool-policy /fimplement hook:` approval and Permissions lines, implement, and quote an unresolved or unfrozen plan line in the report"
		}
		ready_after_wip: {
			after:    "title_set_wip"
			kind:     "requires"
			on:       "title_set_ready"
			reason:   "fimplement: run `$SKILL_FIMPLEMENT_TITLE_SET title set WIP` at intake before setting READY — the session title leaves the gate state when the run starts, not when it ends"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_FIMPLEMENT_TITLE_SET title set WIP`, then `$SKILL_FIMPLEMENT_TITLE_SET title set READY` before the report"
		}
	}
}
smoke: {
	env: {
		CHECK:        "go test ./internal/smoke/..."
		COMMIT_SKILL: "package-commit"
	}
	expect: {
		check: ["go", "test", "internal/smoke/greeting.go", "internal/smoke/greeting_test.go", "internal/smoke/farewell.go", "internal/smoke/farewell_test.go", "internal/smoke/message.go", "internal/smoke/message_test.go"]
	}
	fixture: {
		tree: "tree"
	}
	invoke: "fimplement-smoke.md"
	limits: {
		minutes: 45
		usd:     8.0
	}
}
