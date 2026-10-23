changelog: [{
	date:    "2026-10-21"
	version: "3.20.2"
	text:    "Preflight stop names the token and bin/routine precondition stop; flock and timeout are no routine precondition"
	tokens:  18809
}, {
	date:    "2026-10-17"
	version: "3.20.1"
	text:    "Model file is <leaf>.cue"
	tokens:  18810
}, {
	date:    "2026-10-13"
	version: "3.20.0"
	text:    "Requires the mining component: the skill drives this repo's own make targets and never deploys outside the smine checkout; the five soft mining tags it replaces are gone."
	tokens:  18720
}, {
	date:    "2026-10-09"
	version: "3.19.0"
	text:    "Authoring grammar names the runtime placement fields docker, enableDelegation and enforcePermissions and their gate ACDSL-SKILL-016."
	tokens:  18809
}, {
	date:    "2026-10-09"
	version: "3.18.1"
	text:    "Dropped the Sync step and the commit-body sync reminder — the global rules forbid telling the user to sync."
}, {
	date:    "2026-10-04"
	version: "3.18.0"
	text:    "mining steps, routes and edges tagged with needs: partial installs and the bundle drop them. References reworded to name no smine program."
	tokens:  18663
}, {
	date:    "2026-10-06"
	version: "3.17.5"
	text:    "New routine.json drops `container` — the Docker opt-in and its repo mounts are plist env keys set in the Configure form."
	tokens:  18667
}, {
	date:    "2026-10-06"
	version: "3.17.4"
	text:    "New routine.json carries `container` `false` — the Docker isolation opt-in, visible and off by default."
	tokens:  18666
}, {
	date:    "2026-10-05"
	version: "3.17.3"
	text:    "Added the Adds column to the Decisions table of the plan template"
	tokens:  18652
}, {
	date:    "2026-10-04"
	version: "3.17.2"
	text:    "Provenance rule names a skills-kind smine change instead of the deleted smine-apply skill."
	tokens:  18648
}, {
	date:    "2026-10-03"
	version: "3.17.1"
	text:    "Dropped the fmt and delegate references — both skills were deleted; the Delegation and Runner model fields are gone with their two format-gate checks."
	tokens:  18658
}, {
	date:    "2026-10-03"
	version: "3.17.0"
	text:    "Removed the approval-gate stop: a written plan proceeds to the deliverable; the skill and routine build steps no longer wait on plan approval."
	tokens:  19339
}, {
	date:    "2026-10-03"
	version: "3.16.3"
	text:    "Template follows the plan catalog: Decisions directly behind Scope, reader cells first in the Decisions header."
	tokens:  19482
}, {
	date:    "2026-10-03"
	version: "3.16.2"
	text:    "Permission-coverage face names what decides a headless call — settings layers and the skill's allowed-tools, read headless too; --allowedTools restates them and is no third source; Workflow needs a settings allow rule and the root session."
	tokens:  19482
}, {
	date:    "2026-10-02"
	version: "3.16.1"
	text:    "Routine intake names the proposal store by `<store>`, the path `make rules ARGS=\"store-root\"` prints — a smoke or eval run reads its own store, never the live one."
	tokens:  19438
}, {
	date:    "2026-10-02"
	version: "3.16.0"
	text:    "Routine intake reads accepted routine proposals from the plans-archive store under a Read grant; the origin-marker rule names the proposal kind, not a path."
	tokens:  19468
}, {
	date:    "2026-10-02"
	version: "3.15.0"
	text:    "No checkout holds SKILL.md: deploy renders it; skills-check gates the canonical model and generated leftovers."
	tokens:  19382
}, {
	date:    "2026-10-01"
	version: "3.14.0"
	text:    "Format gate: FMTGATE-019 — a skill body never calls EnterPlanMode; plan mode is the session's state at invocation; the user alone enters it."
	tokens:  19403
}, {
	date:    "2026-09-30"
	version: "3.13.1"
	text:    "Template re-cut onto the human-first plan catalog: Context became a Drivers table after Assumptions, Assumptions and Decisions in their new shapes, Verification as a table."
	tokens:  19302
}, {
	date:    "2026-09-29"
	version: "3.13.0"
	text:    "Scope drops the Deferred findings group."
}, {
	date:    "2026-09-29"
	version: "3.12.0"
	text:    "Plan template drops the Stop conditions section: its pre-checks (shadowed skill, headless preflight, token file, scaffold target) are settled in Assumptions before approval; the nightly halt record goes to the commit body and a plan Changelog row."
}, {
	date:    "2026-09-29"
	version: "3.11.2"
	text:    "Model-assignment table: judgment agents take the bracket model, else `opus` — never inherit the session model (heartbeat stops an inherited spawn)."
}, {
	date:    "2026-09-29"
	version: "3.11.1"
	text:    "Plan-file examples and lookup use design/<slug>.md (fdesign 5.8.0)."
}, {
	date:    "2026-09-28"
	version: "3.11.0"
	text:    "FORMAT-006: ## Model is the typed model block in skill.cue; tier and effort are schema-enforced, routes override per route"
}, {
	date:    "2026-09-28"
	version: "3.10.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.10.0"
	text:    "Model-first authoring: the skill route writes skill.cue plus payloads/<entry-id>.md, and make skill-render generates SKILL.md (FORMAT-012g, PROCESS-003); make skills-check gates hand edits (new FMTGATE-018). The plan carries skill.cue and payload files (PLAN-001, TPL-001, PROCESS-002). Version, changelog and atoms live in skill.cue (FORMAT-007/008, FMTGATE-005/008/016, LAYER-001, FORMAT-012h). Payload bodies live in payloads/ (FORMAT-012f), and renames re-id entries and payload files (NAMING-002). References to cmd/skillfmt, render-skill --list-entries and atoms.json are retired."
}, {
	date:    "2026-09-28"
	version: "3.9.0"
	text:    "Decisions table follows the plan format's new shape: ID | Problem | Facts | Decision | Rejected | Why | Consequences."
}, {
	date:    "2026-09-27"
	version: "3.8.0"
	text:    "Delegation vocabulary gained degraded (mandatory on Agent-tool [spawn] skills) and never (mandatory on Workflow stage-script skills); declared Delegation: degraded itself."
}, {
	date:    "2026-09-26"
	version: "3.7.0"
	text:    "Route anchors: route sections carry exactly one of replaces: <items> or between: <after>..<before> (FORMAT-012i, nightly template in LAYER-005a); Skill route and Routine route became phases whose flow entries carry skill_route_selected / routine_route_selected — skill, routine, or both are independent optional steps, not alternatives; Nightly route replaces PLAN-002 + PROCESS-013 + RPROC-006..RPROC-007b."
}, {
	date:    "2026-09-26"
	version: "3.6.0"
	text:    "Routine route: invocation-parity rule (PACKAGE-005) and gate check (RGATE-008) — the routine prompt is the exact interactive invocation, plist env carries runner knobs only; contract in context/routines.md."
}, {
	date:    "2026-09-26"
	version: "3.5.0"
	text:    "--nightly: Nightly route — plan to the plan dir, built without approval, disputes recorded as OPEN rows, bootstrap and smoke deferred past merge; Nightly layer (LAYER-005a-d) and nightly format check (FMTGATE-017); routine route scaffolds <label>.plist.template with __REPO_DIR__/__HOME__, dispatch-only routines allowed; headless preflight restated: plan mode, ExitPlanMode, AskUserQuestion absent, Workflow and Skill present, runner passes allowed-tools as --allowedTools"
}, {
	date:    "2026-09-25"
	version: "3.4.0"
	text:    "Routine route scaffolds routine.json + a plist that execs bin/routine run; run.sh, flock/timeout preconditions and the reference-sibling check retired with the shell runtime"
}, {
	date:    "2026-09-24"
	version: "3.3.1"
	text:    "Context-map shape names the default slice (smartskill) and the topic: token"
}, {
	date:    "2026-09-16"
	version: "3.3.0"
	changes: ["Adversarial classification pass: challenger spawn with probe battery, JSON verdict contract, fold step, dispute stop — between Draft and Format gate", "Advanced layers ladder: spawn flow, script channel, formal model, context-map, eval bundle, branch-prefix, allowed-tools — railroad-review as reference sibling", "Three new format-gate checks: branch-prefix, spawn wiring, opt-in model green", "Register sweep: imperative steps, If-STOP stops, MUST rules; definitional FORMAT/NAMING/PACKAGE entries retagged def/rule; sections re-sorted to schema order", "PLAN-001 retagged rule — a flow step sat in the context-kind Plan section"]
}, {
	date:    "2026-09-15"
	version: "3.2.0"
	changes: ["Reframed the formal-clause layer as the optional analysis tier — clauses annotations by default, atoms.json + queries the opt-in to the checked model run by audit-full"]
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal schema pass: section kinds on Plan (context) and both routes (route, pred: skill_route_selected / routine_route_selected); pred/post clauses on 20 route/plan/safety entries; sibling atoms.json (20 bool atoms); two solved queries (approval-clears-gate, context-injection-invariant)."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-13"
	version: "2.3.0"
	text:    "Entry grammar v2 — headline breaks after the title em-dash with the statement on following lines, 128-char line wrap outside fences and tables (cmd/skillfmt migrates), the protocol register (one element per entry, necessity test, caveman compression), format: 2 strict caps (statement 200, bullet 160, Why banned — rationale to the skill README.md) and the fixed file-layout skeleton, all per the new register rules in context/rules/skills.md"
}, {
	date:    "2026-09-13"
	version: "2.2.0"
	text:    "Entry grammar: channel axis gains hook — channel ∈ gate | hook | schema | prose, with the Hook: pointer bullet claim-checked against cmd/hooks/ like Gate:/Schema:"
}, {
	date:    "2026-09-08"
	version: "2.1.0"
	text:    "Caveman trim of over-cap entry statements (wording only, cap now 320 display runes)"
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat of the skill's own body: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), argument-hint now a real example, RGATE-003's enumeration fenced, caveman pass — FORMAT/FMTGATE entries' prescribed content unchanged."
}, {
	date:    "2026-09-07"
	version: "1.17.2"
	text:    "Routing section replaces When-to-use (Purpose/Disambiguation/Preconditions/Pipeline), one-sentence intro rule, argument-hint carries a leading run title"
}, {
	date:    "2026-09-07"
	version: "1.17.1"
	text:    "Args Effect cell ends with a title link to the owning entry (link-convention rule); cap measured on display runes"
}, {
	date:    "2026-09-07"
	version: "1.17.0"
	text:    "Skill format v2: Args as syntax-index table (semantics in owning entries), When-to-use with four technical fields incl. routing table, argument-hint as full example invocation (FORMAT-014), caveman-always register (FORMAT-015), 240-rune statement cap and enumeration rule; FMTGATE-003/004 updated, FMTGATE-012/013 added"
}, {
	date:    "2026-09-06"
	version: "1.16.4"
	text:    "Changes template placeholder now requires the planned-files table (File | Kind | Entry) at the top of Changes, per the new plan-format inventory rule"
}, {
	date:    "2026-09-05"
	version: "1.16.3"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.16.2"
	text:    "Corrected routine branch prose to the code's claude-routines/<group>-<date> literal (was routine/<group>-<date>)"
}, {
	date:    "2026-09-05"
	version: "1.16.1"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-04"
	version: "1.16"
	text:    "acdsl-context gains FACT-CLAUDE-*: skill authoring now sees the Claude platform facts (prompt-cache mechanics, harness behavior, multi-agent cost law)"
}, {
	date:    "2026-09-04"
	version: "1.15"
	text:    "Routine-route run.sh hardening contract and format gate now require a fan-out cell runner to retry a lane once on a transient is_error envelope (socket-closed / empty output / exit 1 with no artifacts) before recording it failed, and to write a distinguishing marker for a non-transient account session-limit kill rather than silently dropping the lane"
}, {
	date:    "2026-09-04"
	version: "1.14"
	text:    "Both routes write a plan before drafting (§ Plan: PLAN-001/002, TPL-001 declared via plan-template); acdsl-context RULE-PLAN-* added; format section and gate carry the plan-template rule; process steps renumbered around the plan step"
}, {
	date:    "2026-09-04"
	version: "1.13"
	text:    "acdsl-context added: the skill-format rules bind on the skills this skill authors"
}, {
	date:    "2026-09-02"
	version: "1.12"
	text:    "Routine route gains a headless-capability preflight (Process step 3 + format-gate item): every headless-driven skill's tools must exist under claude -p and every side-effecting command must be covered by allowed-tools ∪ settings allowlist ∪ --allowedTools, else the scaffold stops"
}, {
	date:    "2026-09-01"
	version: "1.11"
	text:    "Skill-format contract: context entries reach a skill only via frontmatter acdsl-context — a body never references a context-entry id (spec: context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.10"
	text:    "Routine packaging contract moved to context/routines.md (was an archived plan doc); rename-sweep excludes updated for the external plans-archive repo"
}, {
	date:    "2026-08-27"
	version: "1.9"
	text:    "skill route stamps origin: user frontmatter when creating a skill from a proposal — the provenance marker the casual lockout keys on; never added to shipped skills"
}, {
	date:    "2026-08-23"
	version: "1.8"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.7"
	text:    "Retargeted skill registration references to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-20"
	version: "1.6"
	text:    "description format rule gains brevity caps (sentence <=160, total <=450 chars), test-enforced"
}, {
	date:    "2026-08-18"
	version: "1.5"
	text:    "SKILL.md ## Changelog section retired — changelog.json is the only changelog; two version surfaces"
}, {
	date:    "2026-08-18"
	version: "1.4"
	text:    "entry grammar for skill bodies (SKILL-<NAME>-<TOPIC>-NNN, payload entries), gate ACDSL-SKILL-005, /fmt skill migration route"
}, {
	date:    "2026-08-18"
	version: "1.3"
	text:    "reference rename — fchange → fdesign change (fchange retired into the fdesign change route)"
}, {
	date:    "2026-07-31"
	version: "1.2"
	text:    "packaging contract label rename to com.smine.routine.*"
}, {
	date:    "2026-07-30"
	version: "1.1"
	text:    "Command surface retired in favor of allowed-tools frontmatter (permission-manifest MVP); Model contract and format gate updated"
}, {
	date:    "2026-07-27"
	version: "1.0"
	text:    "merges couchskill-create v1.7 and couchroutine-create v1.5 into one arg-routed skill (skill|routine routes); effort token normalized (large → high); eval handoff → /skillroutine-eval; naming schema documents the smine pipeline prefix"
}]
frontmatter: [{
	key:   "name"
	value: " skillroutine-create"
}, {
	key:   "description"
	value: " Create or modify repo skills and launchd routines — route-selected by arg or intent, enforcing the repo skill format and the routine packaging contract. Trigger on /skillroutine-create [skill|routine] or \"create a skill\" or \"create/schedule a routine\". Args — skill|routine: route selector; inferred from intent when unambiguous; --nightly: unattended, plan to plan dir, no approval."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.20.2"
}, {
	key:   "title-code"
	value: " SKCR"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"skill|routine --nightly\""
}, {
	key:   "allowed-tools"
	value: " Read(~/GolandProjects/agent-plans/**)"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-*, RULE-PLAN-*, FACT-CLAUDE-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "plan-template"
	value: " SKILL-SKILLROUTINECREATE-TPL-001"
}]
preamble: """
	# Skillroutine Create

	Authors and modifies repo skills, and scaffolds launchd routines.

	The deliverable is a format-compliant skill directory plus its registration, or a bootstrapped verified routine — never an
	unregistered orphan or an unloaded scaffold.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "from"
					skill:    "smine-routines"
					optional: true
				}]
				note:       "the routine route often follows the skill route in the same session, or an accepted routines proposal in the proposal store's `routines.json`"
				standalone: true
			}
			preconditions: ["a purpose statement — what the skill or routine does, when it triggers, what it must not swallow", "routine route additionally: the token file `~/.config/claude-routine/token` exists with non-zero size (verify with `[[ -s ]]` or `wc -c` only — **never read, cat, echo, or otherwise load its contents into context**, it is a live OAuth token)", "routine route: `bin/routine` built in the main checkout (`make build`)", "routine route: a prompt or slash command that runs non-interactively"]
			purpose: "creates or restructures a repo skill, bumps a skill after content changes, creates a new scheduled routine, wraps a skill as a nightly/periodic run, or brings an ad-hoc launchd job under the routine contract."
			routes: [{
				for: "evaluating skill runs"
				use: "/skillroutine-eval"
			}, {
				for: "ranking mined skill proposals"
				use: "/smine-skills"
			}, {
				for: "ranking mined routine candidates"
				use: "/smine-routines"
			}, {
				for: "plugin (non-repo) skills"
				use: "the anthropic-skills plugin"
			}, {
				for: "managing existing routines (start/stop/reschedule/history)"
				use: "the config server's `/routines` surface"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "skill|routine"
				effect: "route selector — `skill` authors/edits a repo skill, `routine` scaffolds a launchd routine. Absent → infer from the request; ambiguous (\"a skill that runs nightly\") → skill route first, then routine route, stated."
			}, {
				name:   "--nightly"
				effect: "unattended — plan written to the plan dir and built without approval, bootstrap deferred past merge; see [Nightly substitution](#skill-skillroutinecreate-nightly-001)."
			}]
		}
	}]
}, {
	title: "Route"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINECREATE-ROUTE-001"
			post: ["skill_route_selected"]
			role:      "step"
			statement: "TAKE the Skill route when the deliverable is a skill — `<leaf>.cue` plus payload files, new or edited; deploy renders SKILL.md from it."
			title:     "Skill deliverable"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-ROUTE-002"
			post: ["routine_route_selected"]
			role:      "step"
			statement: "TAKE the Routine route when the deliverable is a `routines/<name>/` directory (`routine.json` + plist)."
			title:     "Routine deliverable"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-ROUTE-003"
			pred:      "skill_route_selected and routine_route_selected"
			role:      "step"
			statement: "RUN the skill route first, then the routine route, when both are wanted — two deliverables, one session."
			title:     "Both wanted"
		}
	}]
}, {
	title: "Plan"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PLAN-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "Changes carries complete final `<leaf>.cue`, `payloads/*.md`, `routine.json`, plist as fenced blocks — steps copy them."
			}]
			role:      "rule"
			statement: "Both routes MUST write the plan (template below, per `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`) before any deliverable file, per its delivery rule (plan-mode file, else chat)."
			title:     "Plan first"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-TPL-001"
			payload: {
				info:   "markdown"
				marker: "````"
			}
			role:      "payload"
			statement: "The plan template, both routes."
			title:     "Plan template"
		}
	}]
	kind: "context"
}, {
	title: "Skill route"
	blocks: [{
		heading: "Repo skill format"
	}, {
		entry: {
			id:      "SKILL-SKILLROUTINECREATE-FORMAT-012a"
			channel: "gate"
			items: [{
				marker: "*"
				text:   "Lines outside fences and table rows break at 128 chars; the renderer wraps entry text, unwrapped in `<leaf>.cue` (line-shape rule, context/rules/skills.md)."
			}, {
				label:  "Gate"
				marker: "*"
				text:   "ACDSL-SKILL-005"
			}]
			role:      "def"
			statement: "Body instructions are **entries** (grammar v2): headline `**SKILL-<NAME>-<TOPIC>-NNN[a-z]?** \\`[role][channel]\\` **<Title>**`, no trailing dash; blank line before the statement."
			title:     "Entry headline"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012b"
			items: [{
				label:  "Title"
				marker: "*"
				text:   "bold, a 1–3-word noun-phrase name for the entry (e.g. **Chunked fan-out**, **Two-attempt cap**)."
			}]
			role:      "def"
			statement: "NAME = leaf dir upper-cased without hyphens (`package-commit` → `PACKAGECOMMIT`), TOPIC = the section's short tag (`INTAKE`, `MODEA`, `TPL`), NNN from 001 per topic."
			title:     "Headline tokens"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012c"
			items: [{
				marker: "*"
				text:   "role ∈ def | rule | step | spawn | substep | verify | stop | quality | payload."
			}, {
				marker: "*"
				text:   "channel ∈ gate | hook | schema | prose (`prose` implicit) — taxonomy in `context/actions/README.md`."
			}, {
				marker: "*"
				text:   "A freeform custom tag (`[key:value]`, e.g. `[todo:unmeasured]`) may follow role/channel — never validated, no closed set."
			}, {
				marker: "*"
				text:   "Every `[quality]` entry also carries `[todo:unmeasured]` until an eval-ablation consumer exists."
			}]
			role:      "def"
			statement: "Role and channel are the two mutually exclusive entry axes (node-ontology rule, context/rules/skills.md)."
			title:     "Role and channel"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012d"
			items: [{
				marker: "*"
				text:   "A new format-2 skill sets `format: 2` and declares the node-ontology, register and section-flow entries in its `acdsl-context:` line."
			}, {
				label:  "Register"
				marker: "*"
				text:   "one protocol element per entry, necessity-tested and caveman-compressed (protocol-register rule)."
			}, {
				marker: "*"
				text:   "Sequences over two ordered steps go to a numbered fenced procedure block in a `[payload]` entry."
			}]
			role:      "def"
			statement: "Statement register/capitalization follow the register rule in context/rules/skills.md; topic-section order and the no-format-meta-prose discipline follow the section-flow rule there."
			title:     "Register and flow"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012e"
			items: [{
				label:  "Over cap"
				marker: "*"
				text:   "split into letter-suffixed siblings sharing the number (`-014a`, `-014b`), each titled and under cap."
			}, {
				marker: "*"
				text:   "Enumerations over 6 comma-separated items never sit inline — fenced block one item per line, or a table (enumeration rule)."
			}]
			role:      "def"
			statement: "Caps: statement max 320 runes (legacy); under frontmatter `format: 2` max 200, bullets max 160 joined, `* Why:` bullets banned — rationale moves to the sibling README.md design notes (no-Why rule)."
			title:     "Caps and splitting"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012f"
			items: [{
				marker: "*"
				text:   "Metadata sections (Routing, Args, Model) are not entries — file skeleton is the layout rule."
			}, {
				marker: "*"
				text:   "A `[gate]`/`[hook]`/`[schema]` claim is checked: `Gate:`/`Hook:`/`Schema:` names only the id/script/schema path, nothing else on the line."
			}]
			role:      "def"
			statement: "Templates/examples are `[payload]` entries: body in `payloads/<entry-id>.md`, fence marker and info in `<leaf>.cue`; renders as one fenced block (four-backtick fence if the body has fences)."
			title:     "Payload entries"
		}
	}, {
		entry: {
			id:      "SKILL-SKILLROUTINECREATE-FORMAT-012g"
			channel: "gate"
			items: [{
				marker: "*"
				text:   "Edit `<leaf>.cue` as text, then render — the render re-saves it canonically; comments are rejected, notes are data."
			}, {
				label:  "Inspect"
				marker: "*"
				text:   "`make skill-export SKILL=<name>` prints the model as JSON."
			}, {
				marker: "*"
				text:   "Markdown-only SKILL.md (third-party): `make skill-import DIR=<dir>` once."
			}, {
				label:  "Drift"
				marker: "*"
				text:   "`make skills-check` (in `make audit`) fails on a non-canonical `<leaf>.cue` and on any generated file in a skill dir."
			}, {
				label:  "Gate"
				marker: "*"
				text:   "ACDSL-SKILL-005"
			}, {
				marker: "*"
				text:   "Runtime placement: `docker` ∈ `always | never`, `enableDelegation` ∈ `always | never`, `enforcePermissions` over `acceptEdits | planMode`; absent = agnostic."
			}, {
				marker: "*"
				text:   "`enforcePermissions: [\"bypass\"]` stands alone and needs `docker: \"always\"`; a Script: pointer needs `enableDelegation: \"never\"` — ACDSL-SKILL-016."
			}]
			role:      "def"
			statement: "Source is `<leaf>.cue` plus `payloads/<entry-id>.md`; no checkout holds SKILL.md, `title.json` or `enforce.json` — deploy renders them into the skill homes."
			title:     "Model source"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012h"
			items: [{
				marker: "*"
				text:   "The trailing `\\` is the markdown hard break — clause lines and title render as separate tight lines."
			}, {
				marker: "*"
				text:   "The semantic model derives from them mechanically; deploy strips them; zero runtime effect."
			}, {
				marker: "*"
				text:   "Clauses are annotations by default, parse-checked only; `atoms` in `<leaf>.cue` plus `queries/` opt into the checked model (audit-full runs the queries)."
			}, {
				marker: "*"
				text:   "Opt in only for interacting state — counters, modes, obligatory stops, fan-out; never on def/payload entries; a quality pred is soft; post is flow-roles only."
			}, {
				marker: "*"
				text:   "Entries without clauses are simply uncovered."
			}]
			role:      "def"
			statement: "Optional entry fields `pred`, `post` in `<leaf>.cue`, rendered as code lines above the headline: `{pred: <expr>}` first, `{post: <atoms>}` beneath (formal-clause rule, context/rules/skills.md)."
			title:     "Formal clauses"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-012i"
			items: [{
				marker: "*"
				text:   "A route section carries `, pred: <expr>` plus exactly one anchor: `, replaces: <items>` (substitution) or `, between: <after>..<before>` (optional insertion)."
			}, {
				marker: "*"
				text:   "The route line stays in the deployed skill."
			}, {
				marker: "*"
				text:   "Anchor items are short ids or `A..B` ranges joined with ` + `; `start`/`end` in between only."
			}, {
				marker: "*"
				text:   "Per section, entries keep the schema order: defs, rules, quality, flow entries in execution order, payloads."
			}, {
				marker: "*"
				text:   "Out-of-order blocks fail the model check under format 2; reorder them in `<leaf>.cue`."
			}]
			role:      "def"
			statement: "Sections declare their kind in `<leaf>.cue` section fields, rendered as `{section: phase|route|context}` under the heading; absent means phase (section-flow rule, context/rules/skills.md)."
			title:     "Section kinds"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-001"
			items: [{
				label:  "Groups"
				marker: "*"
				text:   "concept, feature, git, skillroutine, smine, util."
			}, {
				label:  "Deploy targets stay flat"
				marker: "*"
				text:   "sync copies every leaf to `~/.claude/skills/<name>/`."
			}]
			role:      "def"
			statement: "Directory `skills/<group>/<name>/` (grouped) or top-level `skills/<name>/` (e.g. `fmt`) — the **leaf** dir name is the skill name; group folders carry no SKILL.md."
			title:     "Skill directory"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-005"
			items: [{
				marker: "*"
				text:   "Deep semantics live in the owning body entry, never the cell. Effect cell ends with a title link to that entry (link convention in context/rules/skills.md)."
			}, {
				marker: "*"
				text:   "Bullet fallback `- <name>: <doc>` only for skills with ≤2 selector/flag args."
			}, {
				marker: "*"
				text:   "The description carries the compressed args clause; the set of Arg names matches it exactly."
			}, {
				marker: "*"
				text:   "A skill without intake args omits the section."
			}]
			role:      "def"
			statement: "`## Args` is a syntax-index table `| Arg | Form | Default | Effect |`: Arg backticked, Form the exact intake syntax, Default the absent-behavior, Effect one caveman line ≤160 display runes."
			title:     "Args section"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-006"
			items: [{
				marker: "*"
				text:   "`tier` ∈ `small | mid-tier | frontier`, `effort` ∈ `low | medium | high | xhigh | max` — schema-enforced; `routes` overrides per route."
			}, {
				marker: "*"
				text:   "Never size words in effort; normalize on touch."
			}, {
				marker: "*"
				text:   "`Reason:`, `Tested unviable:`, then notes."
			}]
			role:      "def"
			statement: "`## Model` is the typed `model` block in `<leaf>.cue`, rendered in this field order:"
			title:     "Model fields order"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FORMAT-007"
			role:      "def"
			statement: "`changelog` list in `<leaf>.cue`: rows `{version, date, text}`, newest first — the only changelog; SKILL.md never renders it."
			title:     "Changelog list"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-013"
			items: [{
				marker: "*"
				text:   "Args clause compresses the `## Args` table, arg-bearing skills only; routing boundaries live in `## Routing`, never here."
			}, {
				marker: "*"
				text:   "Brevity caps, repo-test-enforced: first sentence ≤160 chars, whole description ≤450 chars."
			}, {
				marker: "*"
				text:   "The tooltip renders the string verbatim, so over-cap is a broken display, not a style nit."
			}]
			role:      "def"
			statement: "Description shape, shown verbatim and used for routing: `<short sentence on what it does>. Trigger on /<name> or \"<top trigger phrases>\". Args — <name>: <short doc>.`"
			title:     "Description shape"
		}
	}, {
		entry: {
			id:      "SKILL-SKILLROUTINECREATE-FORMAT-010"
			channel: "gate"
			items: [{
				label:  "Gate"
				marker: "*"
				text:   "ACDSL-SKILL-012"
			}, {
				marker: "*"
				text:   "Instead it names the rule in plain language plus the owning file (`context/rules/skills.md`); sole exception: ids as data in an output-format example."
			}]
			pred:      "not context_id_referenced"
			role:      "rule"
			statement: "A skill body NEVER references a context-entry id. Context reaches it only through frontmatter `acdsl-context:` (the skill-context hook injects matching entries at invocation)."
			title:     "Context injection"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-002"
			items: [{
				marker: "*"
				text:   "The config-server parser reads line-wise between the fences but tolerates `>`/`|` block-scalar descriptions (third-party skills). Own skills stay single-line."
			}]
			role:      "rule"
			statement: "Frontmatter — `<leaf>.cue` `frontmatter` fields, rendered into SKILL.md — MUST carry single-line `name`, `description`, `author`, `version`."
			title:     "Frontmatter fields"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-003"
			items: [{
				marker: "*"
				text:   "Never add it to shipped skills; never remove it."
			}]
			role:      "rule"
			statement: "A skill created by a smine change (a skills-kind change commit) MUST carry `origin: user` (one line, after `author:`) — the provenance marker the casual lockout keys on."
			title:     "Origin marker"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-004"
			items: [{
				label:  "Intro"
				marker: "*"
				text:   "one sentence under the title, free prose after in own paragraphs; Routing: four fields, bullets below."
			}, {
				marker: "*"
				text:   "No `## Changelog` section — history lives in the `<leaf>.cue` `changelog` list."
			}, {
				label:  "Routing fields"
				marker: "*"
				text:   "**Purpose:** one sentence · **Disambiguation:** table `| For | Use |`, confusable siblings only."
			}, {
				marker: "*"
				text:   "**Preconditions:** testable bullets · **Pipeline:** one arrow line to the README Skill map."
			}]
			role:      "rule"
			statement: "Sections MUST follow the order: title, one-sentence intro, `## Routing`, `## Args` (arg-bearing only), the body, `## Model`."
			title:     "Required sections"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FORMAT-008"
			role:      "rule"
			statement: "Frontmatter `version` MUST equal the newest `changelog` row's version, both in `<leaf>.cue` (AGENTS.md rule); every content change bumps both, metadata-only edits do not."
			title:     "Version sync"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FORMAT-009"
			role:      "rule"
			statement: "Context-doc references MUST use `$AGENT_CONTEXT_DIR_DEFAULT/...` — substituted at deploy, never a hardcoded docs path."
			title:     "Context path token"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-011"
			items: [{
				marker: "*"
				text:   "Catalog sections from `context/rules/plan.md` in catalog order, TLDR first, non-catalog sections marked with an `<override:` placeholder. Gate: ACDSL-SKILL-007."
			}]
			role:      "rule"
			statement: "A plan-producing skill MUST declare `plan-template: <entry id>` in frontmatter, naming the `[payload]` entry holding its plan template."
			title:     "Plan template declaration"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-FORMAT-014"
			items: [{
				marker: "*"
				text:   "Never a bracketed arg-name list, never leads with `/<leaf>` — the harness UI already shows the skill name."
			}, {
				marker: "*"
				text:   "Skills without intake args omit the key."
			}]
			role:      "rule"
			statement: "Every arg-bearing skill MUST carry `argument-hint`: one args-only example — starts with the run title or first arg value, every optional arg set realistically, real intake syntax (`name: value`)."
			title:     "Argument hint"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FORMAT-015"
			pred:      "skill_route_selected"
			role:      "step"
			statement: "RUN an explicit caveman pass over all new prose as the last drafting action, per the prose-register rule in context/rules/skills.md — technical content byte-perfect."
			title:     "Caveman register"
		}
	}, {
		heading: "Advanced layers"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-001"
			role:      "def"
			statement: "Sophistication is opt-in layers over the plain body, each added only when its trigger holds; railroad-review carries every layer — the reference sibling."
			title:     "Complexity ladder"
		}
	}, {
		prose: """
			| Layer | Add when | Files |
			| :--- | :--- | :--- |
			| spawn flow | work fans out to subagents | `[spawn]`/`[substep]` entries + [Model table](#skill-skillroutinecreate-layer-002) |
			| script channel | spawn control must be deterministic | `workflows/<stage>.js`, `schemas/<name>.json` ([Script authoring](#skill-skillroutinecreate-layer-004)) |
			| formal model | interacting state — counters, modes, obligatory stops, fan-out | `<leaf>.cue` `atoms`, `queries/` ([Formal clauses](#skill-skillroutinecreate-format-012h)) |
			| context slices | lanes need differing injected context | `context-map.json` ([Context-map shape](#skill-skillroutinecreate-layer-003)) |
			| eval bundle | golden measurement wanted | `eval/definition.json` + `eval/golden/`, handed to /skillroutine-eval |
			| branch prefix | skill or its workflows mint branches | frontmatter `branch-prefix:` (branch-creation rule, context/rules/skills.md) |
			| allowed-tools | a headless command the settings allowlist misses | frontmatter `allowed-tools:`, fully allowlisted (Model section) |
			| nightly mode | a routine wraps the skill or the Backlog board dispatches to it | `--nightly` Args row + `## Nightly route` ([Nightly layer](#skill-skillroutinecreate-layer-005a)) |
			"""
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-002"
			role:      "def"
			statement: "Skill spawning 2+ agent kinds carries a Model-assignment `[payload]` table `| Agent | Model | Effort |`; mechanical workers pin `sonnet` low, judgment agents bracket model else `opus`, never inherit."
			title:     "Model table"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-003"
			role:      "def"
			statement: "`context-map.json` maps each slice (`lane:` or `topic:` name) to context-entry globs, a subset of the frontmatter declaration; a `default` slice makes a smartskill; ids here are data."
			title:     "Context-map shape"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-004"
			role:      "def"
			statement: "A stage file is written with the workflow-authoring skill loaded; `make workflow-gen` injects schema and common blocks, freshness-gated — generated output is never hand-edited."
			title:     "Script authoring"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-005a"
			role:      "def"
			statement: "`--nightly` flag plus a `## Nightly route` section `{section: route, pred: mode = nightly, replaces: <gate items>}` before `## Model`; halting or asking gate entries gain `{pred: mode = interactive}`."
			title:     "Nightly layer"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-005b"
			role:      "def"
			statement: "A nightly run never asks, waits, or enters plan mode; a would-be question is an `OPEN` row with marked recommendation; an approval-gated deliverable lands where approval would put it."
			title:     "Nightly contract"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-005c"
			role:      "def"
			statement: "A nightly run builds only its own skill's deliverable, never chains into another skill; a halting stop records its finding in the deliverable; `.routine-commit-body` names deliverable and `OPEN` rows."
			title:     "Nightly limits"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-LAYER-005d"
			role:      "def"
			statement: "Backlog dispatch appends route args, `B-NNN: <title>`, a blank line, the description; the route's intake step takes that text as the request."
			title:     "Nightly input"
		}
	}, {
		heading: "Naming schema"
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-NAMING-001"
			items: [{
				marker: "*"
				text:   "`internal/skills/skills.go` sets `Name` from `filepath.Base` and discards frontmatter `name`; frontmatter `name`, the H1, and every reference surface mirror it."
			}, {
				marker: "*"
				text:   "Authored in `<leaf>.cue`: the `name` frontmatter field and the H1 in `preamble`."
			}]
			role:      "def"
			statement: "Skill identity is the **leaf directory name** — the single source of truth. Names follow this schema:"
			title:     "Naming source"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NAMING-003"
			role:      "rule"
			statement: "A family sharing a long or collision-prone prefix MUST collapse to a single-letter prefix, never repeat the family word: `fdesign`, `fexplore`, `fimpact`, `fimplement` (not `feature-design` …)."
			title:     "No family prefix"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NAMING-004"
			role:      "rule"
			statement: "Sequential stages MUST be named for the stage, not the family: `idea → concept → clarify` (not `concept-init` / `concept-clarify`)."
			title:     "Pipeline stages unprefixed"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-NAMING-005"
			items: [{
				marker: "*"
				text:   "the smine pipeline's front is `smine`, its stages are `smine-batch` and the `smine-{memory,skills,workflows,routines,rules,summary}` dimensions."
			}, {
				marker: "*"
				text:   "The prefix names the pipeline they belong to, not a redundant family word."
			}]
			role:      "rule"
			statement: "Stage and dimension skills of a pipeline with a user-facing front MUST carry the front's name as prefix — one entrypoint, sorted together."
			title:     "Pipeline prefix rule"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NAMING-006"
			role:      "rule"
			statement: "N skills differing only by target MUST merge into one arg-routed skill: `fmt`, this skill (not `reformat-plan` + `reformat-concept`)."
			title:     "Arg-routed variants"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NAMING-007"
			role:      "rule"
			statement: "A raw name colliding with a built-in harness command MUST NOT be used (e.g. `/design` is taken)."
			title:     "Avoid harness collisions"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-NAMING-002"
			items: [{
				marker: "*"
				text:   "The leaf dir name is the identity; frontmatter `name`, the H1, and the README Skill map all mirror it."
			}, {
				marker: "*"
				text:   "In `<leaf>.cue`: edit `name` and the H1, rename entry ids and `payloads/<entry-id>.md` to the new NAME token; then `make skill-render`."
			}]
			pred:      "skill_route_selected"
			role:      "step"
			statement: "RENAME a skill by dir rename plus a reference sweep to zero (excluding `sessions/**`, prior changelog, plans-archive); deploy with `sync --prune` to drop the old leaf."
			title:     "Leaf name identity"
		}
	}, {
		heading: "Process"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-007"
			role:      "def"
			statement: "A role probe is one register-transform question against a drafted entry; its outcome is agree or dispute, a dispute carrying a worked counter-reading."
			title:     "Role probe"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PROCESS-008"
			items: [{
				marker: "*"
				text:   "Condition-met halt ⇒ stop; test-then-repair, no halt ⇒ verify; measured conduct ⇒ quality; firing-moment-free vocabulary ⇒ def; template only ⇒ payload."
			}, {
				label:  "Placement probes"
				marker: "*"
				text:   "flow entry in a `context` section, pred/post on def or payload, post on rule or quality, register mismatch — each a dispute."
			}]
			role:      "def"
			statement: "Battery order per entry: MUST-insertable unchanged ⇒ rule; one own-transcript action ⇒ step; spawned-transcript action ⇒ substep; the spawn event itself ⇒ spawn."
			title:     "Probe battery"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-001"
			pred:      "skill_route_selected"
			role:      "step"
			statement: "COLLECT purpose, trigger phrases, routing boundaries; CHECK the inventory for overlap (a shadowing new skill is a finding, not a deliverable); PICK the group folder, confirm with the user."
			title:     "Intake"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PROCESS-002"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "walk the [Complexity ladder](#skill-skillroutinecreate-layer-001); every triggered layer's files appear in the plan's Changes table."
			}]
			pred:      "skill_route_selected"
			role:      "step"
			statement: "WRITE the plan per § Plan with the complete `<leaf>.cue` and payload files in Changes; body follows the nearest sibling skill's structure."
			title:     "Plan"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PROCESS-003"
			post: ["draft_written"]
			pred:      "skill_route_selected"
			role:      "step"
			statement: "COPY the approved `<leaf>.cue` and payload files out of the plan; RUN `make skill-render SKILL=<name>` to canonicalize and check the model."
			title:     "Draft"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PROCESS-009"
			items: [{
				label:  "param"
				marker: "*"
				text:   "model `sonnet`, effort medium; over 40 entries ⇒ two challengers split by section, dispute lists unioned."
			}]
			post: ["challenge_returned"]
			pred:      "skill_route_selected and draft_written"
			role:      "spawn"
			statement: "SPAWN one challenger agent over the drafted body with the [Challenger brief](#skill-skillroutinecreate-tpl-002) — the body only, never the plan's reasoning."
			title:     "Challenger spawn"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-010"
			pred:      "skill_route_selected"
			role:      "substep"
			statement: "PROBE every entry with the battery; RECORD agree or dispute, the deciding probe, and a one-line worked example per dispute."
			title:     "Probe entries"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-011"
			pred:      "skill_route_selected"
			role:      "substep"
			statement: "RETURN JSON lines `{id, verdict, role_claimed, role_probed, probe, example}` — no prose report."
			title:     "Return verdicts"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PROCESS-012"
			post: ["classification_settled"]
			pred:      "skill_route_selected and challenge_returned"
			role:      "step"
			statement: "FOLD agreed retags into `<leaf>.cue`, re-sort each touched section to the schema order, re-render; a retag crossing the flow boundary re-checks the entry's clauses."
			title:     "Fold retags"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-013"
			pred:      "skill_route_selected and classification_disputed and mode = interactive"
			role:      "stop"
			statement: "If author and challenger hold defensible different roles for an entry ⇒ STOP: present both readings with probe results; the user's ruling folds in as binding."
			title:     "Disputed classification"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-004"
			pred:      "skill_route_selected and classification_settled"
			role:      "step"
			statement: "WALK the checklist below; FIX every miss in `<leaf>.cue` and re-render before presenting."
			title:     "Format gate"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PROCESS-005"
			pred:      "skill_route_selected"
			role:      "step"
			statement: "ADD the skill to README.md § Skill map in the smine repo (chain position or standalone list)."
			title:     "Register"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-TPL-002"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "The challenger's spawn prompt — probe battery plus worked misclassifications."
			title:     "Challenger brief"
		}
	}, {
		heading: "Format gate"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-001"
			role:      "rule"
			statement: "Frontmatter MUST have name, description, author, version — all single-line."
			title:     "Frontmatter check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-002"
			role:      "rule"
			statement: "A plan-producing skill MUST declare `plan-template:` and its template MUST pass the plan-interface gate."
			title:     "Plan-template check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-003"
			role:      "rule"
			statement: "`## Routing` MUST carry the four fields; Disambiguation MUST list only genuinely confusable siblings; each precondition MUST be individually testable; Pipeline MUST link the README Skill map."
			title:     "Routing check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-004a"
			role:      "rule"
			statement: "Intake args MUST be in the `## Args` table (bullet fallback only ≤2 selector args); Effect cells one line, semantics in owning entries."
			title:     "Args table check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-004b"
			role:      "rule"
			statement: "The arg-name set MUST match description and argument-hint; the section MUST be absent only when the skill takes none."
			title:     "Args name check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-005"
			role:      "rule"
			statement: "Model section MUST be present; no Changelog section MUST exist; `<leaf>.cue` `changelog` MUST be non-empty, its newest row's version equal to frontmatter `version`."
			title:     "Model/changelog check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-006"
			role:      "rule"
			statement: "Suggested effort token MUST come from the real vocabulary (low | medium | high | xhigh | max); size words are normalized on touch."
			title:     "Effort token check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-008"
			pred:      "version_synced"
			role:      "rule"
			statement: "Frontmatter `version` and the newest `changelog` row in `<leaf>.cue` MUST agree."
			title:     "Version sync check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-009"
			role:      "rule"
			statement: "Context docs MUST be referenced via $AGENT_CONTEXT_DIR_DEFAULT."
			title:     "Context path check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-010"
			role:      "rule"
			statement: "README.md § Skill map MUST be updated."
			title:     "Skill map check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-011"
			role:      "rule"
			statement: "Description MUST follow the shape: short sentence, trigger clause, args clause (arg-bearing skills only) — no routing boundaries."
			title:     "Description shape check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-012"
			role:      "rule"
			statement: "`argument-hint` MUST be an args-only example, never leading with `/<leaf>`, with every optional arg set to a realistic (non-placeholder) value in the skill's real syntax."
			title:     "Argument-hint check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-013"
			role:      "rule"
			statement: "Prose MUST pass the caveman register: no filler, no invented abbreviations, technical content byte-perfect; statements MUST stay under the format's rune cap (200 under `format: 2`, 320 legacy)."
			title:     "Caveman check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-014"
			role:      "rule"
			statement: "A branch-minting skill MUST declare frontmatter `branch-prefix:`; every branch it or its workflows mint MUST start with it."
			title:     "Branch-prefix check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-015"
			role:      "rule"
			statement: "Every `[spawn]` MUST own at least one `[substep]`; a `[script]` spawn MUST carry `Script:` and `Schema:` bullets naming existing files."
			title:     "Spawn wiring check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-016"
			role:      "rule"
			statement: "A skill declaring `atoms` in its `<leaf>.cue` MUST resolve every atom, declare sources for input atoms, and keep `queries/` green under `make audit-full`."
			title:     "Opt-in model check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-017"
			role:      "rule"
			statement: "A `--nightly` skill MUST name every human gate it replaces by link in its Nightly route; no ask, wait or approval entry MAY stay unreplaced."
			title:     "Nightly check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-018"
			role:      "rule"
			statement: "`<leaf>.cue` MUST be in canonical form — `make skills-check` green; it MUST carry no comment, no payload file MUST be orphaned and no generated file MUST sit in the skill dir."
			title:     "Render freshness check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-FMTGATE-019"
			role:      "rule"
			statement: "A skill body MUST NOT call `EnterPlanMode` — plan mode is the session's state at invocation, never a skill action; gate delivery branches on that state."
			title:     "Plan-mode entry check"
		}
	}, {
		heading: "Evals"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-EVALS-001"
			pred:      "skill_route_selected"
			role:      "step"
			statement: "HAND OFF eval needs to /skillroutine-eval with the skill name and its manifest stub — benchmark loops never run here."
			title:     "Delegate evals"
		}
	}]
	kind: "phase"
}, {
	title: "Routine route"
	blocks: [{
		prose: """
			Create a managed routine: a subdirectory of `routines/` with a `routine.json` manifest and a launchd plist that execs
			`bin/routine run`, conforming to the packaging contract in `context/routines.md`. The deliverable is a bootstrapped, verified
			routine — never an unloaded scaffold.
			"""
	}, {
		heading: "Packaging contract"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PACKAGE-001"
			role:      "def"
			statement: "The packaging contract lives in `context/routines.md`; templates at `routines/_templates/` (underscore skips server discovery) — copied and edited, never restated. Per routine `routines/<name>/`:"
			title:     "Contract source"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PACKAGE-002"
			items: [{
				marker: "*"
				text:   "Token, locks, worktree isolation, deadlines, results rows, publish: never re-authored per routine."
			}, {
				marker: "*"
				text:   "Flow `single` cannot express (stages, harvest/drain) = new kind in `internal/routinerun` via /fdesign, never a script."
			}, {
				marker: "*"
				text:   "Fan-out cells retry a lane once on transient `is_error`; non-transient loss (session-limit kill) gets its own marker."
			}]
			role:      "def"
			statement: "`routine.json` — from `routines/_templates/routine.json`: `kind` `single`, `prompt` (`/<skill> <args>`); Docker opt-in is plist env set in the Configure form; mechanics belong to the `routine` binary."
			title:     "Manifest contents"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PACKAGE-003a"
			items: [{
				marker: "*"
				text:   "`ProgramArguments` → `__REPO_DIR__/bin/routine`, `run`, `__REPO_DIR__/routines/<name>`."
			}, {
				marker: "*"
				text:   "`StartCalendarInterval` is the only supported trigger; absent = dispatch-only (Run Now, Backlog board)."
			}, {
				marker: "*"
				text:   "`StandardOutPath`/`StandardErrorPath` under `__HOME__/Library/Logs/claude-routine-<name>.*.log`."
			}]
			role:      "def"
			statement: "`com.smine.routine.<name>.plist.template` — from `routines/_templates/routine.plist`; Label `com.smine.routine.<name>`."
			title:     "Plist template"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PACKAGE-003b"
			role:      "def"
			statement: "`install.sh` materializes the gitignored `.plist` in the main checkout, substituting `__REPO_DIR__` and `__HOME__`."
			title:     "Plist materialization"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-PACKAGE-004"
			role:      "def"
			statement: "`results.jsonl` — created by the first run, not scaffolded."
			title:     "Results file"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-PACKAGE-005"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "invocation-parity section of `context/routines.md` — the closed set of permitted differences."
			}]
			role:      "rule"
			statement: "The routine prompt MUST be the exact interactive invocation — every skill input an arg; plist env MUST carry runner knobs only (`ROUTINE_*`, `SMINE_*`)."
			title:     "Invocation parity"
		}
	}, {
		heading: "Process (routine)"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RPROC-001a"
			pred:      "routine_route_selected"
			role:      "step"
			statement: "COLLECT name, prompt or slash command, schedule, budget, permission mode."
			title:     "Intake fields"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RPROC-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<store>` = the path `make rules ARGS=\"store-root\"` prints."
			}]
			pred:      "routine_route_selected"
			role:      "step"
			statement: "CHECK `<store>/proposals/routines.json` for accepted proposal and caveats (non-interactive mode; observation-loop: pin a target ID, self-terminate after N no-change)."
			title:     "Intake proposal check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RPROC-002"
			pred:      "routine_route_selected and (not token_present or not routine_binary_built)"
			role:      "stop"
			statement: "If the token file is empty (size check), or `bin/routine` is not built in the main checkout, or the wrapped command cannot run headless ⇒ STOP: no scaffold until fixed."
			title:     "Precondition check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RPROC-003a"
			pred:      "routine_route_selected"
			role:      "verify"
			statement: "Before scaffolding: VERIFY both faces for every skill the routine drives headless."
			title:     "Preflight scope"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RPROC-003b"
			pred:      "routine_route_selected"
			role:      "verify"
			statement: "VERIFY (a) the run path needs no plan mode, `ExitPlanMode` or `AskUserQuestion` — absent under `claude -p`; a skill hinging on them gets a [Nightly layer](#skill-skillroutinecreate-layer-005a) first."
			title:     "Tool-availability face"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RPROC-003c"
			items: [{
				marker: "*"
				text:   "The tool-policy gate judges a headless run like any session: settings layers ∪ the skill's `allowed-tools`; uncovered = **categorically denied**."
			}, {
				marker: "*"
				text:   "The routine mints its work, then silently fails the denied step."
			}, {
				marker: "*"
				text:   "The runner restates the installed skill's `allowed-tools` as `--allowedTools` (plus `Write`); headless reads the frontmatter line too."
			}, {
				marker: "*"
				text:   "`Workflow` runs only from the root session and only with a settings allow rule (`settings.json` or `settings.local.json`)."
			}]
			pred:      "routine_route_selected"
			role:      "verify"
			statement: "VERIFY (b) every side-effecting command the skill's body directs is covered by `allowed-tools` ∪ the settings allowlist."
			title:     "Permission-coverage face"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RPROC-003d"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a routine hinged cells on the Workflow tool before headless runs carried it."
			}, {
				label:  "Separately"
				marker: "*"
				text:   "merge-resolve issued bare `git merge`/`--abort`/`merge-tree`/`commit`, never allowlisted."
			}, {
				marker: "*"
				text:   "A nightly chain-sync minted the branch then was denied the merge."
			}]
			pred:      "routine_route_selected and (tool_gap_found or permission_gap_found)"
			role:      "stop"
			statement: "If the preflight finds a tool or permission gap ⇒ STOP: scaffold waits until the skill is fixed or the rule allowlisted — distinct from the token / bin/routine precondition stop."
			title:     "Gap and origin"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RPROC-004"
			pred:      "routine_route_selected"
			role:      "step"
			statement: "WRITE the plan per § Plan with the complete `routine.json` and plist in Changes and the preflight findings in Scope."
			title:     "Plan"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RPROC-005"
			pred:      "routine_route_selected"
			role:      "step"
			statement: "WRITE the approved `routine.json` and plist template from the plan into `routines/<name>/`."
			title:     "Scaffold"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RPROC-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "from the main checkout, once the scaffold is on `main` and `install.sh` materialized the plist."
			}]
			post: ["bootstrap_verified"]
			pred:      "routine_route_selected and mode = interactive"
			role:      "step"
			statement: "RUN `launchctl bootstrap gui/$(id -u) <plist>`, then VERIFY with `launchctl print gui/$(id -u)/<label>` by exit code: 0 loaded, 113 not loaded — never parse output for this."
			title:     "Bootstrap + verify"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RPROC-007a"
			items: [{
				marker: "*"
				text:   "`runs=1`/`exit=0` is a normal finish, `runs=1`/`exit=1` is a failure, `runs=0` never ran."
			}]
			pred:      "routine_route_selected and mode = interactive"
			role:      "step"
			statement: "RUN `launchctl kickstart gui/$(id -u)/<label>`, then READ `runs` + `last exit code` from `launchctl print` — classify by counters, never the `state = not running` string."
			title:     "Smoke kickstart"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RPROC-007b"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a nightly job exited 1 on first `launchctl kickstart` — embedded-newline token — and the `not running` state read as success."
			}]
			post: ["smoke_confirmed"]
			pred:      "routine_route_selected and bootstrap_verified"
			role:      "step"
			statement: "CONFIRM a `results.jsonl` line with exit_status 0; on failure READ the `~/Library/Logs` pair (stderr first), never the token file."
			title:     "Smoke confirmation"
		}
	}, {
		heading: "Safety invariants"
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-SAFETY-003b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "a fresh dated `claude-routines/<group>-<date>` branch per run, created/committed by the `routine` binary."
			}]
			pred:      "worktree_isolated"
			role:      "rule"
			statement: "The `claude -p` session MUST run in the routine group's own worktree, `~/.cache/claude-routine/worktrees/<group>`."
			title:     "Session worktree isolation"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-003c"
			role:      "rule"
			statement: "A run NEVER reuses a branch: bases the new dated branch on the newest un-merged one (chain tip) else `main`; `ROUTINE_MAX_OPEN_BRANCHES` caps stacked un-merged branches (empty = unlimited)."
			title:     "Branch chain rules"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-003d"
			role:      "rule"
			statement: "A routine's group defaults to its own name; chain members MUST set `ROUTINE_GROUP` to a shared group (one worktree + branch lineage, serialized by the group lock); different groups run concurrently."
			title:     "Group scoping default"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-004b"
			role:      "rule"
			statement: "A run MUST be closed by reviewing and locally merging its newest branch, pruning merged ancestors; deleting a branch discards that run."
			title:     "Review closes run"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-005b"
			role:      "rule"
			statement: "The coverage-increaser's worktree and `claude-routines/coverage-increaser-<date>` branches MUST live in the target repo; results and locks MUST stay in smine."
			title:     "Coverage-increaser worktree location"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-001a"
			pred:      "routine_route_selected"
			role:      "verify"
			statement: "VERIFY the token file by size only."
			title:     "Token size check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-001b"
			pred:      "routine_route_selected and token_read"
			role:      "stop"
			statement: "If a step would read the token contents ⇒ STOP: they never enter a command substitution, log, commit, or context window — only the `routine` binary reads the file."
			title:     "Token stays secret"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-002"
			pred:      "routine_route_selected and config_dir_isolated"
			role:      "stop"
			statement: "If a design introduces `CLAUDE_CONFIG_DIR` isolation ⇒ STOP: routines share `~/.claude` and subscription auth via `CLAUDE_CODE_OAUTH_TOKEN`."
			title:     "Shared auth isolation"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-003a"
			pred:      "routine_route_selected and not main_checkout_used"
			role:      "stop"
			statement: "If scaffold, bootstrap, or kickstart would run in a session worktree ⇒ STOP: they run against the main checkout only."
			title:     "Main-checkout scaffold"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-003e"
			pred:      "routine_route_selected and output_in_main_checkout"
			role:      "stop"
			statement: "If routine output would land in the main checkout's working tree ⇒ STOP: it stays in the run worktree."
			title:     "Output stays isolated"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-003f"
			pred:      "routine_route_selected and sibling_group_touched"
			role:      "stop"
			statement: "If a step would touch a sibling group's worktree or branches ⇒ STOP: nothing ever does."
			title:     "Sibling group isolation"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-SAFETY-004a"
			pred:      "routine_route_selected and remote_access_used"
			role:      "stop"
			statement: "If a routine step would push, open a PR, or call `gh` ⇒ STOP: routines have no remote access — the runner's publish commits locally on the dated branch."
			title:     "No remote access"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-SAFETY-005a"
			items: [{
				marker: "*"
				text:   "falling back to the machine-local `~/.config/claude-routine/coverage-target` (absolute path, untracked like the token file) for manual non-launchd runs."
			}]
			pred:      "routine_route_selected"
			role:      "step"
			statement: "SET `ROUTINE_TARGET_REPO` in the plist `EnvironmentVariables` when scaffolding coverage-increaser (editable in the config server's params form)."
			title:     "Coverage-increaser target repo"
		}
	}, {
		heading: "Format gate (routine)"
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-001"
			role:      "rule"
			statement: "Directory MUST be under `routines/<name>/`, no leading underscore."
			title:     "Directory check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-003"
			role:      "rule"
			statement: "`routine.json` MUST parse under `routines.LoadManifest` (kind known, required field present); no `.sh` file MAY exist in the routine dir."
			title:     "Manifest check"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-RGATE-004"
			items: [{
				marker: "*"
				text:   "A non-transient loss MUST be written as a distinguishing marker, never retried, never silently dropped."
			}]
			role:      "rule"
			statement: "A fan-out cell runner MUST retry a lane once on a transient `is_error` envelope (socket-closed/empty/exit 1, no artifacts) before recording it failed."
			title:     "Retry policy check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-005a"
			role:      "rule"
			statement: "Plist template label MUST equal `com.smine.routine.<name>`, file `<label>.plist.template`; paths MUST use `__REPO_DIR__` /`__HOME__`."
			title:     "Plist check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-005b"
			role:      "rule"
			statement: "`StartCalendarInterval`, when present, MUST be the only trigger; log paths MUST be set."
			title:     "Trigger and logs"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-006"
			role:      "rule"
			statement: "Token contents MUST NEVER be read at any step."
			title:     "Token secrecy check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-007"
			role:      "rule"
			statement: "Bootstrap MUST be verified by `launchctl print` exit code; smoke run MUST be recorded in `results.jsonl`."
			title:     "Bootstrap/smoke check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-008"
			role:      "rule"
			statement: "`routine.json` prompt MUST run unchanged when typed interactively; the plist MUST hold no skill input, no `CLAUDE_CODE_*` key."
			title:     "Parity check"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-RGATE-002"
			pred:      "routine_route_selected"
			role:      "verify"
			statement: "VERIFY headless-capability preflight: no plan mode, `ExitPlanMode` or `AskUserQuestion` on the run path; every side-effecting command covered by `allowed-tools` ∪ settings allowlist."
			title:     "Preflight check"
		}
	}]
	kind: "phase"
}, {
	title: "Nightly route"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-001"
			role:      "def"
			statement: "`--nightly` replaces the human gates named below, nothing else; every other entry applies unchanged."
			title:     "Nightly substitution"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-002"
			role:      "rule"
			statement: "A nightly run MUST NOT ask, wait, or enter plan mode; the group pick in [Intake](#skill-skillroutinecreate-process-001) is decided and recorded as a Decisions row."
			title:     "Never prompt"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-003"
			role:      "step"
			statement: "TAKE the text after the args — route selector, `B-NNN: <title>`, description — as the purpose statement."
			title:     "Dispatched request"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-004"
			role:      "step"
			statement: "WRITE the plan to the plan dir's plan file `design/<slug>.md` (plan-dir rules, `context/rules/plan.md`) and build from it."
			title:     "Plan-dir plan"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINECREATE-NIGHTLY-005"
			post: ["classification_settled"]
			role:      "step"
			statement: "KEEP the author's role on a [Disputed classification](#skill-skillroutinecreate-process-013); RECORD both readings as an `OPEN` row in the plan."
			title:     "Dispute record"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-006"
			role:      "step"
			statement: "SKIP [Bootstrap + verify](#skill-skillroutinecreate-rproc-006) and smoke; WRITE their exact commands into the plan's Verification as post-merge items."
			title:     "Deferred bootstrap"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-007"
			role:      "step"
			statement: "WRITE `.routine-commit-body`: plan path, deliverable paths, `OPEN` row pointers."
			title:     "Commit body"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINECREATE-NIGHTLY-008"
			role:      "stop"
			statement: "If a stop fires ⇒ STOP: record it in `.routine-commit-body` and a plan Changelog row, end the run — never park."
			title:     "Halt record"
		}
	}]
	kind:     "route"
	pred:     "mode = nightly"
	replaces: "PROCESS-013 + RPROC-006..RPROC-007b"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "format enforcement plus routing judgment across the skill inventory, and security-sensitive routine scaffolding (live token adjacency) with launchd state verification"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {
		check: ["test", "-s", ".routine-commit-body"]
	}
	invoke: "skill --nightly B-001: smoke-echo — a skill that repeats its argument back in one line; trigger on /smoke-echo; no args beyond the text"
	limits: {
		minutes: 30
		usd:     6.0
	}
}
