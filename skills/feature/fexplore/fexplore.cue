changelog: [{
	date:    "2026-10-17"
	version: "3.6.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  3824
}, {
	date:    "2026-10-05"
	version: "3.5.4"
	text:    "Added the Adds column to the Decisions table of the plan template"
	tokens:  3912
}, {
	date:    "2026-10-04"
	version: "3.5.3"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  3908
}, {
	date:    "2026-10-03"
	version: "3.5.2"
	text:    "Template follows the plan catalog: Decisions directly behind Scope, reader cells first in the Decisions header."
	tokens:  3989
}, {
	date:    "2026-09-30"
	version: "3.5.1"
	text:    "Template re-cut onto the human-first plan catalog: Context became a Drivers table, Baseline carries the [AGENT-ONLY] marker, Decisions in the Brief / Decision (brief) shape."
	tokens:  3989
}, {
	date:    "2026-09-28"
	version: "3.5.0"
	text:    "Decisions table follows the plan format's new shape: ID | Problem | Facts | Decision | Rejected | Why | Consequences."
}, {
	date:    "2026-09-28"
	version: "3.4.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-26"
	version: "3.4.0"
	text:    "Nightly route declares its anchor: replaces INTAKE-003b + GROUND-003 — the gates it suppresses; the route line stays in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "--nightly: Nightly route — dispatched text is the drivers, clarifying questions and hand-over queries become OPEN rows, re-scoping halts with a record; restored the ## Model section lost in 4fe3d38a1"
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; Args converted to the table form; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-24"
	version: "3.1.2"
	text:    "/clarify absorbed into /concept: disambiguation row and precondition point at /concept"
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: added sibling atoms.json (13 bool atoms) and queries/ (2 queries); pred/post clauses on 9 of 16 coverable entries — the constraints-enumerated -> families-derived -> eval-table-built -> exploration-written causal chain (CONSTRAINTS-001, FAMILIES-001, EVAL-001, RECOMMEND-001), the user-candidate and gap-rejection rule invariants (INTAKE-003a/b), the per-scenario-verdict rule (EVAL-003) and its paired quality clause (EVAL-004b), and the ties-go-OPEN step (EVAL-004a). Floating Phase-1 preamble prose folded into new def entry GROUND-004. Origin bullet on EVAL-003 moved to the param register. All six topic sections stay default phase (no {section:} lines needed — none are context or route). Caveman-compressed."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), argument-hint rewritten to a full invocation, caveman compression pass over body prose — content unchanged"
}, {
	date:    "2026-09-05"
	version: "1.16"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.15"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-04"
	version: "1.14"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not exploration input"
}, {
	date:    "2026-09-04"
	version: "1.13"
	text:    "Exploration re-cut onto the plan catalog (TPL-001 declared via plan-template): TLDR first, Recommendation → Decisions (OPEN when finalists tie), Constraints → Baseline (verified), Options/Evaluation/Rejected as override sections, Changelog last (persisted plan)"
}, {
	date:    "2026-09-04"
	version: "1.12"
	text:    "acdsl-context gains FACT-* — grounded surveys need the repo facts; stale context paths in the body corrected (guides live under rules/, doctrine under actions/)"
}, {
	date:    "2026-09-01"
	version: "1.11"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.10"
	text:    "Plan paths migrated to the dated plan-dir convention (RULE-PLAN-071/072): exploration.md written into the resolved plan dir"
}, {
	date:    "2026-08-23"
	version: "1.9"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.8"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-12"
	version: "1.7"
	text:    "stale references corrected — rules/plan.md path, ACTION-IMPL-* gate entries in actions/implementing.md (retired style/ path and ALWAYS-EXEC naming)"
}, {
	date:    "2026-07-31"
	version: "1.6"
	text:    "activity-scoped context — style/ guides and rules/ activity chapters replace skill assets; stops and hot classes as ALWAYS-EXEC/HOT entries"
}, {
	date:    "2026-07-30"
	version: "1.5"
	text:    "context redesign — planning assets via ../fdesign/assets/, doctrine entries under rules/"
}, {
	date:    "2026-07-26"
	version: "1.4"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "1.3"
	text:    "renamed feature-explore → fexplore (leaf dir, frontmatter name, H1); live references swept repo-wide"
}, {
	date:    "2026-07-16"
	version: "1.2"
	text:    "exploration.md moves to plans/{slug}/design/"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-10"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " fexplore"
}, {
	key:   "description"
	value: " Survey all sensible solutions for a feature — an evaluated solution space with a recommendation. Trigger on /fexplore or \"explore solutions/options/approaches\". Args — mode: unfamiliar|familiar|owned explanation depth; --nightly: unattended, questions become OPEN rows."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.6.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, RULE-PLAN-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"mode: familiar --nightly\""
}, {
	key:   "plan-template"
	value: " SKILL-FEXPLORE-TPL-001"
}]
preamble: """
	# fexplore

	Surveys solution space for a feature; writes evaluated options plus recommendation.

	Produces one artifact: `design/exploration.md` in the feature's plan dir (naming and resolution per the plan-dir rules in
	`context/rules/plan.md`, injected via this skill's context declaration). Sits between clarify (questions drained, concept
	stable) and fdesign (one solution elaborated to file-level precision). fdesign locks a design in; this skill exists so the lock
	happens on a surveyed space, not the first workable idea. Origin: identical parallel runs reached opposite verdicts on the same
	question; a bake-off won by groundedness, not architecture; constraint-first analysis beat iterative variant-proposing.

	Static constraints not restated here: `AGENTS.md`, the format guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` (incl.
	`plan.md`), and the doctrine chapters under `$AGENT_CONTEXT_DIR_DEFAULT/actions/` govern every artifact. Missing context docs:
	state once, continue with built-in baselines — never stall.
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
					kind:     "from"
					skill:    "code-verdict"
					optional: true
				}, {
					kind:  "to"
					skill: "fdesign"
				}]
				note: "fdesign records the chosen option as a `[USER]` decision citing `exploration.md`"
			}
			preconditions: ["a stable concept in the plan dir's `concept/` (questions drained)", "no space-changing open questions remain (else /concept first)"]
			purpose: "surveys an open or contested solution space and hands fdesign an evaluated option set with a recommendation — produces neither a plan nor code."
			routes: [{
				for: "solution already chosen, or a narrow space"
				use: "/fdesign directly"
			}, {
				for: "space-changing open questions still open"
				use: "/concept first"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "mode"
				default: "`familiar`"
				effect:  "explanation depth, never rigor — [Familiarity mode](#skill-fexplore-intake-002)"
				form:    "`unfamiliar\\|familiar\\|owned`"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: clarifying questions and hand-over queries become `OPEN` rows — [Nightly substitution](#skill-fexplore-nightly-001)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Phase 0 — Intake"
	blocks: [{
		entry: {
			id:        "SKILL-FEXPLORE-INTAKE-002"
			role:      "def"
			statement: "`mode: unfamiliar | familiar | owned` (default familiar), same semantics as fdesign: changes explanation depth, never rigor."
			title:     "Familiarity mode"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-INTAKE-003a"
			pred:      "user_candidate_present => user_candidate_evaluated"
			role:      "rule"
			statement: "The user's candidate MUST be treated as a first-class option, evaluated under the user's premise — including changes to components the team owns."
			title:     "User candidate first-class"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-INTAKE-003b"
			pred:      "gap_rejection => clarifying_question_asked"
			role:      "rule"
			statement: "A rejection resting on \"the current artifact doesn't do X\" MUST get one clarifying question first."
			title:     "Rejection needs question"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-INTAKE-001a"
			role:      "step"
			statement: "FIND the concept: plan dir's `concept/`, resolved by glob per the plan-dir resolution rule."
			title:     "Locate concept"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-INTAKE-001b"
			role:      "step"
			statement: "FIND the drivers: what opens the solution space (performance concern, rejected first design, new constraint, user's own candidate idea)."
			title:     "Locate drivers"
		}
	}]
}, {
	title: "Phase 1 — Ground"
	blocks: [{
		entry: {
			id: "SKILL-FEXPLORE-GROUND-004"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "the facts feed the constraint list and the evaluation."
			}]
			role:      "def"
			statement: "fdesign Phase 1 discipline, scoped to what ALL candidate approaches need: verified facts with file:line anchors, real data inspected, exemplars named — claim without anchor is opinion."
			title:     "Ground scope"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-GROUND-001"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Current gaps MUST be separated from architectural limits. Each option MUST be evaluated both as-is and fully-implemented; the owner's implementation intent is an input, never inferred from TODOs."
			title:     "Gaps vs limits"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-GROUND-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Live probe MUST be preferred over document archaeology. A blocking capability question (vendor docs stale or unfetchable) MUST prompt the cheapest live experiment instead of more searching."
			title:     "Live probe"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-GROUND-003"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Design MUST proceed by measurement. An option hinging on a data distribution MUST get a runnable query handed to the user; the result becomes a binding fact."
			title:     "Design by measurement"
		}
	}]
}, {
	title: "Phase 2 — Constraints"
	blocks: [{
		entry: {
			id: "SKILL-FEXPLORE-CONSTRAINTS-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Constraints MUST define the space; options MUST derive from them, never brainstormed freely."
			title:     "Options derived"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-CONSTRAINTS-001"
			post: ["constraints_enumerated"]
			role:      "step"
			statement: "ENUMERATE hard constraints first — platform, language, data shapes, deploy topology, compatibility contracts. Each gets an ID (`C<n>`) and an anchor or measurement."
			title:     "Enumerate constraints"
		}
	}]
}, {
	title: "Phase 3 — Solution families"
	blocks: [{
		entry: {
			id: "SKILL-FEXPLORE-FAMILIES-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "A family violating a hard constraint MUST be listed with its killing constraint — visible, never silently dropped."
			title:     "Killed families visible"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-FAMILIES-001"
			post: ["families_derived"]
			pred:      "constraints_enumerated"
			role:      "step"
			statement: "DERIVE the exhaustive set of families the constraints admit. Per family: name, mechanism in one paragraph, which constraints bind or kill it, what it assumes will change (and who owns that)."
			title:     "Exhaustive families"
		}
	}]
}, {
	title: "Phase 4 — Evaluation"
	blocks: [{
		entry: {
			id:        "SKILL-FEXPLORE-EVAL-002"
			role:      "def"
			statement: "Groundedness = how much existing code/pattern it rides (the criterion that decided the bake-off)."
			title:     "Groundedness defined"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-EVAL-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "Origin — \"The sec scenarios are however brunoable.\""
			}]
			pred:      "partial_fit => per_scenario_verdict_given"
			role:      "rule"
			statement: "Per-scenario verdicts MUST be allowed: when a subset of requirements fits a \"losing\" option, MUST say so — never a winner-take-all verdict."
			title:     "Per-scenario verdicts allowed"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-EVAL-004b"
			custom: ["todo:unmeasured"]
			pred:      "options_tied => not single_winner_declared"
			role:      "quality"
			statement: "A false single winner MUST NOT be presented."
			title:     "Never fake winner"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-EVAL-001"
			post: ["eval_table_built"]
			pred:      "families_derived"
			role:      "step"
			statement: "BUILD the evaluation table: `Option | Groundedness | Blast radius | Effort | Reversibility | Verdict`."
			title:     "Evaluation table"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-EVAL-004a"
			post: ["open_decision_written"]
			pred:      "options_tied"
			role:      "step"
			statement: "WRITE ties within noise as both options under one OPEN decision in exploration.md: two options within noise of each other both go to the recommendation as an OPEN decision."
			title:     "Ties go OPEN"
		}
	}]
}, {
	title: "Phase 5 — Recommendation & handoff"
	blocks: [{
		entry: {
			id: "SKILL-FEXPLORE-RECOMMEND-001"
			post: ["exploration_written"]
			pred:      "eval_table_built"
			role:      "step"
			statement: "WRITE `design/exploration.md` in the plan dir (created per the plan-dir naming rule when absent), filled per the plan format in `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`:"
			title:     "Write exploration doc"
		}
	}, {
		entry: {
			id: "SKILL-FEXPLORE-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "The exploration, on the plan catalog."
			title:     "Exploration template"
		}
	}]
}, {
	title: "Nightly route"
	blocks: [{
		entry: {
			id:        "SKILL-FEXPLORE-NIGHTLY-001"
			role:      "def"
			statement: "`--nightly` replaces the human gates named below, nothing else; every other entry applies unchanged."
			title:     "Nightly substitution"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-NIGHTLY-002"
			role:      "rule"
			statement: "A nightly run MUST NOT ask or wait; the question [Rejection needs question](#skill-fexplore-intake-003b) demands and the query [Design by measurement](#skill-fexplore-ground-003) hands over become `OPEN` rows, recommendation marked."
			title:     "Never prompt"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-NIGHTLY-003"
			role:      "step"
			statement: "TAKE the text after the args — `B-NNN: <title>`, description — as the drivers; a matching plan-dir concept is read, else the description stands in."
			title:     "Dispatched request"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-NIGHTLY-004"
			role:      "step"
			statement: "WRITE `.routine-commit-body`: exploration path, recommendation, `OPEN` row pointers."
			title:     "Commit body"
		}
	}, {
		entry: {
			id:        "SKILL-FEXPLORE-NIGHTLY-005"
			role:      "stop"
			statement: "If the space needs re-scoping ⇒ STOP: record it in exploration.md's Open questions and `.routine-commit-body`, end the run."
			title:     "Halt record"
		}
	}]
	kind:     "route"
	pred:     "mode = nightly"
	replaces: "INTAKE-003b + GROUND-003"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "exhaustive solution-family enumeration, grounded evaluation, groundedness over architecture judgment"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {
		check: ["test", "-s", ".routine-commit-body"]
	}
	fixture: {
		tree: "tree"
	}
	invoke: "mode: owned --nightly B-001: smoke-explore — explore how the config server caches context/context.json per the concept"
	limits: {
		minutes: 25
		usd:     5.0
	}
}
