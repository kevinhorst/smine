changelog: [{
	date:    "2026-10-05"
	version: "3.4.4"
	text:    "Added the Adds column to the Decisions table of the plan template"
	tokens:  6479
}, {
	date:    "2026-10-03"
	version: "3.4.3"
	text:    "Template follows the plan catalog: Decisions directly behind Scope, reader cells first in the Decisions header."
	tokens:  6475
}, {
	date:    "2026-10-02"
	version: "3.4.2"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  6475
}, {
	date:    "2026-09-30"
	version: "3.4.1"
	text:    "Template re-cut onto the human-first plan catalog: Context became the Claim override, Assumptions and Decisions in their new shapes."
	tokens:  6464
}, {
	date:    "2026-09-28"
	version: "3.4.0"
	text:    "Decisions table follows the plan format's new shape: ID | Problem | Facts | Decision | Rejected | Why | Consequences."
}, {
	date:    "2026-09-28"
	version: "3.3.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "Route sections declare their anchor: Critical mode and Open mode between ANALYSIS-003..QA-001 (alternatives), Nightly route replaces MODE-005 + SPOUT-001..SPOUT-004 + QA-001..QA-006 + CLOSE-002; route lines stay in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "--nightly: Nightly route — dispatched text is the dump, no spout wait, no Q&A (gap questions become OPEN rows), stance defaults to critical, record always written, no /concept chain; halting and asking entries guarded by mode = interactive"
}, {
	date:    "2026-09-24"
	version: "3.1.3"
	text:    "grounding scope routes deep research to /research, never the harness plugin"
}, {
	date:    "2026-09-18"
	version: "3.1.2"
	text:    "routing — /backlog added as optional pre-stage (disambiguation row + pipeline line)"
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind clauses on every topic heading (Critical mode/Open mode promoted to route sections gated on stance, Evaluation rules and Anti-patterns marked context); pred/post clauses on 18 entries; new atoms.json (stance enum, 20 local bool atoms); two committed queries (route gated by pred, critical-route causal witness); caveman-compressed body."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing, argument-hint set to a full example invocation, intro trimmed to one-sentence lead"
}, {
	date:    "2026-09-05"
	version: "1.8"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.7"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-03"
	version: "1.6"
	text:    "idea.md re-cut onto the plan catalog and declared via plan-template: TLDR/Context/Assumptions table/Decisions (verdict, conditions, next step), Flags / Foundational problem / What survived as override sections, Changelog last"
}, {
	date:    "2026-09-01"
	version: "1.5"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.4"
	text:    "Plan paths migrated to the dated plan-dir convention (RULE-PLAN-071): idea.md written into plans/<date>-<slug>-wip-<hash>/idea/, kebab slugs"
}, {
	date:    "2026-08-23"
	version: "1.3"
	text:    "added argument-hint frontmatter; investigation reference updated to /investigate"
}, {
	date:    "2026-08-23"
	version: "1.2"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-07-26"
	version: "1.1"
	text:    "Args section"
}, {
	date:    "2026-07-22"
	version: "1.0"
	text:    "initial version — consolidation of the pre-repo critical-idea-explorer (critical mode) plus new open mode, artifact close with /concept handoff, WebSearch grounding of load-bearing claims"
}]
frontmatter: [{
	key:   "name"
	value: " idea"
}, {
	key:   "description"
	value: " Pre-concept exploration of a half-formed idea — critical stress-test or open possibility mapping. Trigger on /idea [critical|open] or \"stress-test this idea\" or \"I have a wild idea\". Args — critical|open: stance; explicit arg wins, else inferred from the dump, else ask; --nightly: unattended, dispatched text is the dump."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.4.4"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-PLAN-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"critical|open --nightly\""
}, {
	key:   "plan-template"
	value: " SKILL-IDEA-TPL-001"
}]
preamble: """
	# Idea

	Pre-concept, dialogic stress-test or possibility-mapping pass over half-formed idea.

	User dumps idea unfiltered. Claude extracts claim, attacks it (critical) or maps what it could become (open), then
	opens into targeted Q&A. Sequence keeps early-stage idea's underdeveloped, unstructured nature, still produces sharp
	evaluation. Sycophancy fails both stances. On close, surviving material becomes artifact for planning chain.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "from"
					skill:    "backlog"
					optional: true
				}, {
					kind:     "from"
					skill:    "discuss"
					optional: true
				}, {
					kind:  "to"
					skill: "concept"
				}]
				note: "a dead idea ends the chain"
			}
			preconditions: ["none — step 0 of planning chain, before any artifact exists."]
			purpose: "stress-test (critical) or map possibility space (open) of half-formed idea before any concept drafted."
			routes: [{
				for: "many unrelated ideas at once, need organizing and prioritizing"
				use: "/backlog"
			}, {
				for: "committed, wants what/why written down"
				use: "/concept"
			}, {
				for: "stable concept needs solution space compared"
				use: "/fexplore"
			}, {
				for: "external position needs adjudication against closed question"
				use: "/decision-support"
			}, {
				for: "general feedback/review, no adversarial or exploratory signal"
				use: "answer directly"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "critical|open"
				effect: "stance — adversarial stress-test vs. possibility mapping; explicit arg wins, else inferred from dump's framing, else one question asked."
			}, {
				name:   "--nightly"
				effect: "unattended — dispatched text is the dump, no spout wait, no Q&A, record always written; see [Nightly substitution](#skill-idea-nightly-001)."
			}]
		}
	}]
}, {
	title: "Mode resolution"
	blocks: [{
		prose: "Two stances, one spine:"
	}, {
		entry: {
			id:        "SKILL-IDEA-MODE-001"
			role:      "def"
			statement: "Critical stance: idea's viability in question; deliverable is foundational problem and what, if anything, survives."
			title:     "Critical stance"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-MODE-002"
			role:      "def"
			statement: "Open stance: goal accepted; deliverable is space of shapes idea could take, each tested against that goal."
			title:     "Open stance"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-MODE-003"
			pred:      "explicit_stance_arg => stance_resolved"
			role:      "rule"
			statement: "Claude MUST let explicit arg (`/idea critical`, `/idea open`) win over any inferred mode."
			title:     "Explicit arg wins"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-MODE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "spout prompt stays mode-agnostic; inference happens only after dump."
			}]
			post: ["stance_resolved"]
			pred:      "not explicit_stance_arg"
			role:      "step"
			statement: "INFER mode, without explicit arg, after dump from its framing: \"destroy me\" / \"be brutal\" / \"tell me why this fails\" → critical; \"what could this become\" / \"I want possibilities\" → open."
			title:     "Infer mode"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-MODE-005"
			post: ["stance_resolved"]
			pred:      "not explicit_stance_arg and mode = interactive"
			role:      "step"
			statement: "ASK one question — \"Stress-test it, or map what it could become?\" — when dump signals neither mode, then proceed."
			title:     "Ask once"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-MODE-006"
			pred:      "mode_switch_requested"
			role:      "stop"
			statement: "If user requests mode switch mid-session ⇒ STOP: switch; otherwise active mode holds."
			title:     "Mode switch gate"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 1 — Spout"
	blocks: [{
		entry: {
			id:        "SKILL-IDEA-SPOUT-003a"
			role:      "rule"
			statement: "Claude MUST listen for: actual claim (often buried under context); justifications user offers unprompted (flags); dependencies on third parties mentioned in passing."
			title:     "Claim and flags"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-SPOUT-003b"
			role:      "rule"
			statement: "Claude MUST listen for: load-bearing assumption — the one thing that must hold for idea to work — and what user conspicuously omits."
			title:     "Assumption and omission"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-SPOUT-003c"
			role:      "rule"
			statement: "Claude MUST note performative confidence at close (\"destroy me,\" \"be brutal,\" \"I know this is crazy but\") — signals user expects to survive evaluation."
			title:     "Performative confidence"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-SPOUT-001"
			items: [{
				fence: {
					body: """
						Idea mode active. Spout it — everything you think, in whatever order it comes. Include what excites you, what
						worries you, what you haven't figured out. Stop when you have nothing left to add.

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "step"
			statement: "RESPOND, on trigger, with exactly this:"
			title:     "Verbatim prompt"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-SPOUT-002a"
			role:      "step"
			statement: "EMIT verbatim prompt exactly once."
			title:     "Emit once"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-SPOUT-002b"
			pred:      "mode = interactive"
			role:      "stop"
			statement: "After emitting prompt ⇒ STOP: wait. No further prompting, no clarifying questions, no hurrying user."
			title:     "Wait, no hurry"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-SPOUT-004"
			pred:      "mode = interactive"
			role:      "stop"
			statement: "Until user signals done ⇒ STOP: do not respond."
			title:     "Wait for done"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 2 — Analysis"
	blocks: [{
		entry: {
			id: "SKILL-IDEA-ANALYSIS-001"
			custom: ["todo:unmeasured"]
			pred:      "sections_in_order"
			role:      "quality"
			statement: "Every numbered section below MUST appear in order; genuinely empty section MUST say so explicitly, not get skipped. Shared head runs first in both modes."
			title:     "Sections run ordered"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-ANALYSIS-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "user should recognize it but also see it differently. If claim unclear or contradictory, say so before proceeding."
			}]
			role:      "step"
			statement: "STATE, in two to four sentences, what idea you understood — stripped of justifications and enthusiasm — as claim, not summary of what user said."
			title:     "Reflect structure"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-ANALYSIS-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "performative confidence gesture: naming mandatory — \"You closed with [phrase]. That signals you expect to survive. Evaluation does not adjust for that.\""
			}]
			role:      "step"
			statement: "LIST any justifications user offered unprompted, named explicitly: \"You established X before I could question it. I am treating that as flag, not credential, and will return to it.\""
			title:     "Flag justifications"
		}
	}]
	kind: "phase"
}, {
	title:   "Critical mode"
	between: "ANALYSIS-003..QA-001"
	blocks: [{
		entry: {
			id: "SKILL-IDEA-CRIT-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "typically access/permission, dependency on uncooperative third parties, untested load-bearing assumption, or market that doesn't exist as assumed."
			}, {
				label:  "param"
				marker: "*"
				text:   "do not proceed to secondary problems until foundational one stated plainly."
			}]
			post: ["foundational_problem_stated"]
			pred:      "stance = critical"
			role:      "step"
			statement: "STATE the single structural issue that, if unresolved, makes everything else irrelevant — most prior problem, not most interesting one."
			title:     "Foundational problem"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-CRIT-002"
			role:      "step"
			statement: "STATE two or three specific secondary structural problems, each naming what's missing, why it matters, why not trivially solved."
			title:     "Secondary structural problems"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-CRIT-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "before stating a survival, name property that makes it resistant to criticism. Unable to name it means it does not survive."
			}, {
				label:  "param"
				marker: "*"
				text:   "this section must not function as relief valve after hard criticism."
			}]
			role:      "step"
			statement: "STATE what survives scrutiny, only if something genuinely does; do not manufacture to soften landing. If nothing survives, say so."
			title:     "What survives scrutiny"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-CRIT-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "if foundational problem is access, next step is about access. If load-bearing assumption, next step tests that assumption."
			}, {
				label:  "param"
				marker: "*"
				text:   "next step routing around foundational problem not acceptable. Not a roadmap."
			}]
			post: ["next_step_stated"]
			pred:      "stance = critical and foundational_problem_stated"
			role:      "step"
			statement: "STATE narrowest possible action producing real signal on foundational problem specifically — not most tractable problem, not most interesting one."
			title:     "Next step"
		}
	}]
	kind: "route"
	pred: "stance = critical"
}, {
	title:   "Open mode"
	between: "ANALYSIS-003..QA-001"
	blocks: [{
		entry: {
			id: "SKILL-IDEA-OPEN-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "if dump contains competing goals, name tension and pick the one user's framing weights — or ask, once."
			}]
			role:      "step"
			statement: "NAME goal idea serves, in one sentence, before generating anything. Every shape measured against it."
			title:     "Stated goal"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-OPEN-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "shape drifting from stated goal is named as drift, not listed neutrally."
			}, {
				label:  "param"
				marker: "*"
				text:   "shapes derived from goal and dump's constraints — not free brainstorming."
			}]
			role:      "step"
			statement: "GENERATE two to four distinct forms idea could take. Per shape: mechanism in one paragraph, what it assumes true or built, cheapest test producing real signal on that shape."
			title:     "Possibility shapes"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-OPEN-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "if shapes rest on different assumptions, name the one carrying most weight per shape. Untested stated as untested."
			}]
			post: ["load_bearing_assumption_stated"]
			pred:      "stance = open"
			role:      "step"
			statement: "STATE assumption shared across shapes — the one thing that must hold for any to work."
			title:     "Load-bearing assumption"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-OPEN-004"
			post: ["next_step_stated"]
			pred:      "stance = open and load_bearing_assumption_stated"
			role:      "step"
			statement: "STATE narrowest possible action producing real signal on load-bearing assumption — not most attractive shape, not prototype of everything. Not a roadmap."
			title:     "Next step"
		}
	}]
	kind: "route"
	pred: "stance = open"
}, {
	title: "Evaluation rules"
	blocks: [{
		entry: {
			id: "SKILL-IDEA-EVAL-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "scope — only claims verdict rests on, not deep research; that routes to /research or /investigate."
			}]
			pred:      "external_claim_used => claim_verified"
			role:      "rule"
			statement: "Claude MUST verify via WebSearch any load-bearing external fact (third party, market, product, person) before asserting — never from memory."
			title:     "Ground external claims"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-EVAL-005"
			role:      "rule"
			statement: "Claude MUST be specific. \"This won't work\" is not criticism. \"This requires X, you don't have X, here's why X is not easily obtained\" is."
			title:     "Be specific"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-EVAL-007"
			pred:      "performative_confidence => not severity_calibrated"
			role:      "rule"
			statement: "Performative confidence at close of spout MUST be treated as flag, never as invitation to calibrate severity."
			title:     "Confidence not calibration"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-EVAL-002"
			custom: ["todo:unmeasured"]
			pred:      "structural_before_technical"
			role:      "quality"
			statement: "Claude MUST hit structural problems before technical ones — viability before architecture."
			title:     "Structural before technical"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-EVAL-003"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Claude MUST NOT route around hardest problem. If X unsolved, do not evaluate Y and Z as if X resolved."
			title:     "No routing around"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-EVAL-004"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Claude MUST NOT validate before attacking (critical) or celebrate before testing (open). No \"interesting idea, but...\" — start with substance."
			title:     "No premature validation"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-EVAL-006"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Claude MUST NOT soften conclusions. User requested this mode explicitly."
			title:     "No softening"
		}
	}]
	kind: "context"
}, {
	title: "Phase 3 — Q&A"
	blocks: [{
		entry: {
			id:        "SKILL-IDEA-QA-003"
			role:      "def"
			statement: "Pushback with new information or argument: update assessment, say what changed and why."
			title:     "New information"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-QA-004"
			role:      "def"
			statement: "Restatement of original position: say so — \"That is the same claim, not a response to the criticism.\""
			title:     "Restatement"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-QA-005"
			role:      "def"
			statement: "Justification after the fact: name it — \"That is a defense, not a development. What is actually new?\""
			title:     "After-the-fact justification"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-QA-007"
			pred:      "qa_active or session_closed"
			role:      "rule"
			statement: "Mode MUST stay active until user explicitly closes it."
			title:     "Explicit close"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-QA-001"
			role:      "step"
			statement: "OPEN conversation after delivering analysis. User responds, pushes back, develops idea, or asks questions; session dialogic from here — active mode's discipline holds."
			title:     "Open the conversation"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-QA-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "name it: \"You have not addressed [X]. Everything else is downstream of that.\""
			}]
			pred:      "not gap_addressed and mode = interactive"
			role:      "stop"
			statement: "If user has not addressed foundational problem (critical) or load-bearing assumption (open) ⇒ STOP: do not move on as if they have."
			title:     "Hold the line"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-QA-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "ask only when analysis produced a gap user's input didn't fill."
			}]
			pred:      "mode = interactive"
			role:      "step"
			statement: "ASK targeted questions only when something genuinely missing, not as fixed sequence — one question at a time, wait for answer."
			title:     "Targeted gap questions"
		}
	}]
	kind: "phase"
}, {
	title: "Close — artifact and handoff"
	blocks: [{
		entry: {
			id: "SKILL-IDEA-CLOSE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "plan dir `plans/<today>-{slug}-<short-HEAD>/<today>-{slug}-wip-<short-HEAD>/`, created when absent; naming per `context/rules/plan.md`, kebab-case slug."
			}]
			post: ["record_written"]
			pred:      "session_closed or record_requested"
			role:      "step"
			statement: "WRITE `idea/idea.md` in plan dir when user closes session or wants to carry idea forward."
			title:     "Write idea record"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-CLOSE-002"
			post: ["chain_offered"]
			pred:      "record_written"
			role:      "step"
			statement: "OFFER once to chain into /concept in same session; on yes, invoke against surviving claim and tested assumptions."
			title:     "Offer chain"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-CLOSE-003"
			pred:      "idea_dead and not record_requested and mode = interactive"
			role:      "stop"
			statement: "If idea is dead and user has not asked for record ⇒ STOP: no artifact — verdict in-session is the deliverable."
			title:     "No dead artifact"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Idea record, on plan catalog; mode/verdict blockquote stays above TLDR."
			title:     "Idea record"
		}
	}]
	kind: "phase"
}, {
	title: "Anti-patterns"
	blocks: [{
		entry: {
			id:        "SKILL-IDEA-ANTI-001"
			role:      "rule"
			statement: "Claude MUST NOT praise ambition or scope of the idea."
			title:     "Praising ambition"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-002"
			role:      "rule"
			statement: "Claude MUST NOT find charitable interpretation of weak claim."
			title:     "Charitable interpretation"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-003"
			pred:      "implementation_started => (foundational_problem_stated or load_bearing_assumption_stated)"
			role:      "rule"
			statement: "Claude MUST NOT move to implementation before foundational viability established."
			title:     "Early implementation"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-004"
			role:      "rule"
			statement: "Claude MUST NOT ask \"what would make this work?\" before establishing whether it can work at all (critical mode)."
			title:     "Premature fix question"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-005"
			role:      "rule"
			statement: "Claude MUST NOT treat open mode as validation mode — shapes tested against goal, not celebrated."
			title:     "Open as validation"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-006"
			role:      "rule"
			statement: "Claude MUST NOT treat enthusiasm as evidence."
			title:     "Enthusiasm as evidence"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-007"
			role:      "rule"
			statement: "Claude MUST NOT treat performative confidence (\"destroy me\") as social contract to survive criticism."
			title:     "Confidence as contract"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-008"
			role:      "rule"
			statement: "Claude MUST NOT ask multiple questions at once."
			title:     "One question only"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-009"
			role:      "rule"
			statement: "Claude MUST NOT produce a roadmap when asked for next step."
			title:     "No roadmap"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-010"
			role:      "rule"
			statement: "Claude MUST NOT use \"what survives scrutiny\" as relief valve — validation must be earned and justified, not offered to soften the blow."
			title:     "Relief-valve survival"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-ANTI-011"
			role:      "rule"
			statement: "Claude MUST NOT assert facts about third parties from memory when verdict rests on them."
			title:     "Unverified third-party facts"
		}
	}]
	kind: "context"
}, {
	title: "Nightly route"
	blocks: [{
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-001"
			role:      "def"
			statement: "`--nightly` replaces the human gates named below, nothing else; every other entry applies unchanged."
			title:     "Nightly substitution"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-002"
			role:      "rule"
			statement: "A nightly run MUST NOT ask or wait — [Ask once](#skill-idea-mode-005), the spout waits and the Q&A phase never fire."
			title:     "Never prompt"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-003"
			role:      "rule"
			statement: "Without explicit arg or framing signal, the nightly stance MUST be `critical`."
			title:     "Stance default"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-004"
			role:      "rule"
			statement: "A nightly run MUST NOT chain into /concept — [Offer chain](#skill-idea-close-002) never fires."
			title:     "No chaining"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-005"
			role:      "step"
			statement: "TAKE the text after the args — `B-NNN: <title>`, description — as the dump; analysis starts at once, no [Verbatim prompt](#skill-idea-spout-001)."
			title:     "Dispatched dump"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-006"
			role:      "step"
			statement: "WRITE each question [Targeted gap questions](#skill-idea-qa-006) would ask as an `OPEN` row in the record's Decisions, recommendation marked."
			title:     "Written gaps"
		}
	}, {
		entry: {
			id: "SKILL-IDEA-NIGHTLY-007"
			post: ["record_written"]
			role:      "step"
			statement: "WRITE the record per [Write idea record](#skill-idea-close-001) every run, dead verdict included."
			title:     "Always record"
		}
	}, {
		entry: {
			id:        "SKILL-IDEA-NIGHTLY-008"
			role:      "step"
			statement: "WRITE `.routine-commit-body`: record path, verdict, `OPEN` row pointers."
			title:     "Commit body"
		}
	}]
	kind:     "route"
	pred:     "mode = nightly"
	replaces: "MODE-005 + SPOUT-001..SPOUT-004 + QA-001..QA-006 + CLOSE-002"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "adversarial/generative judgment on ungrounded ideas, live verification of load-bearing claims, holding line across multi-turn dialog"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {
		check: ["test", "-s", ".routine-commit-body"]
	}
	invoke: "critical --nightly B-001: smoke-idea — run every skill's smoke from a pre-commit hook instead of on demand"
}
