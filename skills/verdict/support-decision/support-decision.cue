changelog: [{
	date:    "2026-10-17"
	version: "3.3.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  1918
}, {
	date:    "2026-10-10"
	version: "3.2.3"
	text:    "Apply offer gated on the middle ground naming a concrete edit; without one the output ends at the verdict, no closing question."
}, {
	date:    "2026-10-04"
	version: "3.2.2"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  1994
}, {
	date:    "2026-09-28"
	version: "3.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2075
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: first Args section — opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-24"
	version: "3.1.2"
	text:    "/clarify absorbed into /concept: disambiguation row points at /concept"
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind lines (Intake/Rules/Limits as context), pred/post clauses on rule/stop/quality/step entries, sibling atoms.json (13 bool atoms), three queries, caveman-compressed prose."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "format-v2 reformat: When to use → Routing (Purpose/Disambiguation table/Preconditions/Pipeline); no Args section (skill has no invocation args)"
}, {
	date:    "2026-09-05"
	version: "1.3"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-08-23"
	version: "1.2"
	text:    "renamed decision-support to support-decision and moved into the verdict group"
}, {
	date:    "2026-08-23"
	version: "1.1"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-07-19"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " support-decision"
}, {
	key:   "description"
	value: " Adjudicate an external position against the actual code/spec — structured verdict, read-only, never a fix. Trigger on /support-decision or \"does he have a point\" or \"who's right\"."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.3.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}]
preamble: """
	# Decision Support

	External position — colleague's chat message (often German), curl transcript, review comment — plus one grounding line and
	closed verdict question: adjudicate claim against actual code/spec, return independent evidenced verdict.

	Not spec-conformance lookup: "what do you think" gets reasoning + call, never citation alone.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["external position pasted verbatim", "one line grounding context", "closed verdict question — open \"thoughts?\" narrowed first (or proceeds on stated assumption of implied question)"]
			purpose: "adjudicates pasted external position against closed verdict question — \"does he have a point\", \"who's right\", \"is he wrong or am I wrong\" — incl. recurring \"[Decision Support]\" sessions."
			routes: [{
				for: "evaluating a *change* against impact axes"
				use: "/fimpact"
			}, {
				for: "resolving a *concept's* open questions"
				use: "/concept"
			}, {
				for: "hunting defects or verifying a change"
				use: "/railroad-review"
			}]
		}
	}]
}, {
	title: "Intake"
	blocks: [{
		prose: "Three parts, all required:"
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-INTAKE-001"
			role:      "def"
			statement: "Position, verbatim — colleague's message, curl transcript, or review comment, pasted as-is. Immutable: quoted in output, never paraphrased into weaker form."
			title:     "Position Verbatim"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-INTAKE-002"
			role:      "def"
			statement: "One grounding line — what position concerns (\"our rate-limit middleware on X\")."
			title:     "Grounding Line"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-INTAKE-003"
			role:      "def"
			statement: "Closed verdict question — \"Does he have a point?\", \"Is he wrong or am I wrong?\"; open \"thoughts?\" handling per [Requires Closed Question](#skill-supportdecision-limits-001)."
			title:     "Closed Question"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-SUPPORTDECISION-RULES-001"
			pred:      "not citation_only"
			role:      "rule"
			statement: "Adjudication MUST use independent judgment with reasoning — spec-conformance citation alone never the answer."
			title:     "Reasoned Judgment"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-RULES-002"
			pred:      "not flattery_used"
			role:      "rule"
			statement: "Adjudication MUST NOT use flattery framing or hedged diplomacy — candor, evidence, verdict only."
			title:     "No Flattery"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-RULES-003"
			pred:      "position_quoted"
			role:      "rule"
			statement: "Skill MUST quote pasted position verbatim, never paraphrase into weaker form."
			title:     "Quote Verbatim"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-RULES-004"
			pred:      "edits_started => apply_requested"
			role:      "rule"
			statement: "Session MUST stay read-only until explicit apply."
			title:     "Read-only Until Apply"
		}
	}]
	kind: "context"
}, {
	title: "Limits"
	blocks: [{
		entry: {
			id:        "SKILL-SUPPORTDECISION-LIMITS-001"
			pred:      "question_open"
			role:      "stop"
			statement: "If question open (e.g. \"thoughts?\") ⇒ STOP: request narrowing, or proceed on stated assumption of implied question."
			title:     "Requires Closed Question"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-LIMITS-002"
			pred:      "apply_out_of_scope"
			role:      "stop"
			statement: "If requested apply exceeds what adjudication proposed ⇒ STOP: route to planning chain (/fdesign)."
			title:     "Apply Scope"
		}
	}]
	kind: "context"
}, {
	title: "Process"
	blocks: [{
		entry: {
			id: "SKILL-SUPPORTDECISION-PROCESS-002b"
			custom: ["todo:unmeasured"]
			pred:      "not diplomacy_used"
			role:      "quality"
			statement: "Adjudication MUST use candor, not hedged diplomacy — no \"honest truth\" framing."
			title:     "Candor not diplomacy"
		}
	}, {
		entry: {
			id: "SKILL-SUPPORTDECISION-PROCESS-001"
			post: ["grounded"]
			role:      "step"
			statement: "Ground — read actual code/spec position concerns before judging. No adjudication from memory."
			title:     "Ground"
		}
	}, {
		entry: {
			id: "SKILL-SUPPORTDECISION-PROCESS-002a"
			post: ["adjudicated"]
			pred:      "grounded"
			role:      "step"
			statement: "Adjudicate — form independent cross-domain judgment with reasoning across whatever domain question targets: API semantics, git topology, architecture calls."
			title:     "Adjudicate"
		}
	}, {
		entry: {
			id: "SKILL-SUPPORTDECISION-PROCESS-003"
			post: ["answered"]
			pred:      "adjudicated"
			role:      "step"
			statement: "Answer — resolve closed question directly."
			title:     "Answer"
		}
	}]
}, {
	title: "Output"
	blocks: [{
		prose: "One structured adjudication, read-only:"
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-OUTPUT-001"
			role:      "step"
			statement: "LIST where he's right — points that hold, each with evidence (path:line or spec section)."
			title:     "Where He's Right"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-OUTPUT-002"
			role:      "step"
			statement: "LIST where I'd disagree — points that don't, each with evidence."
			title:     "Where I'd Disagree"
		}
	}, {
		entry: {
			id: "SKILL-SUPPORTDECISION-OUTPUT-003"
			post: ["edit_proposed"]
			role:      "step"
			statement: "STATE practical middle ground — the actionable resolution; name the concrete edit (path, change) when one exists."
			title:     "Middle Ground"
		}
	}, {
		entry: {
			id: "SKILL-SUPPORTDECISION-OUTPUT-004"
			post: ["verdict_stated"]
			pred:      "answered"
			role:      "step"
			statement: "STATE verdict — direct answer to closed question."
			title:     "Verdict"
		}
	}, {
		entry: {
			id: "SKILL-SUPPORTDECISION-OUTPUT-005a"
			post: ["apply_offered"]
			pred:      "verdict_stated and edit_proposed"
			role:      "step"
			statement: "OFFER once, only when middle ground names a concrete edit: \"Want me to apply?\"; else END at verdict — no question, no substitute offer."
			title:     "Offer Apply Once"
		}
	}, {
		entry: {
			id:        "SKILL-SUPPORTDECISION-OUTPUT-005b"
			pred:      "not apply_requested"
			role:      "stop"
			statement: "If no explicit \"apply\" / \"make the adjustments\" ⇒ STOP: no edits start."
			title:     "No Pre-Apply Edits"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "independent cross-domain judgment on grounded evidence, resisting framing of pasted position"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "position: \"a skill's smoke fixture belongs in plans/, not beside the skill\" — context: this repo's skills/ and plans/ trees — question: is the position right?"
}
