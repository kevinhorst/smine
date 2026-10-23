changelog: [{
	date:    "2026-10-12"
	version: "1.1.0"
	text:    "Shape over stance: five-sentence turn cap, one lookup and one widening per turn, answer-what-was-said step; dropped the per-turn position and push-back mandates; effort low"
	tokens:  1177
}, {
	date:    "2026-09-28"
	version: "1.0.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1085
}, {
	date:    "2026-09-24"
	version: "1.0.0"
	text:    "Initial smartskill: dialogue-only discussion, default slice + topic widening via self re-invocation, no artifact"
}]
frontmatter: [{
	key:   "name"
	value: " discuss"
}, {
	key:   "description"
	value: " Talk a subject through in short grounded turns — no deliverable, no edits, context pulled in by topic on demand. Trigger on /discuss or \"let's discuss\" or \"talk me through\" or \"thoughts on\". Args — topic: context slice(s) to load."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.1.0"
}, {
	key:   "title-code"
	value: " Dis"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"topic:harness topic:repo why does the nightly routine lose its Workflow tool\""
}, {
	key:   "allowed-tools"
	value: " Glob, Grep, Read"
}, {
	key:   "acdsl-context"
	value: " ACTION-*, FACT-*, RULE-*"
}]
preamble: """
	# Discuss

	Conversation on one subject in short grounded turns — one answer per turn, one lookup at most, ends when the user ends
	it, produces nothing on disk.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "idea"
					optional: true
				}, {
					kind:     "to"
					skill:    "concept"
					optional: true
				}]
				note:       "when the user wants it written"
				standalone: true
			}
			preconditions: ["a subject stated in the invocation or the prior turn"]
			purpose: "think a subject through in dialogue — architecture, harness behavior, process — when no artifact is wanted yet."
			routes: [{
				for: "stress-testing one idea into a written verdict"
				use: "/idea"
			}, {
				for: "one question, one short answer"
				use: "/qq"
			}, {
				for: "a sourced factual answer with citations"
				use: "/research"
			}, {
				for: "adjudicating a pasted external position"
				use: "/support-decision"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "topic"
				default: "default slice"
				effect:  "context slice(s) injected; `--no-context` skips all — [Topic use](#skill-discuss-context-001)"
				form:    "`topic:<name>`, repeatable"
			}]
		}
	}]
}, {
	title: "Context"
	blocks: [{
		entry: {
			id:        "SKILL-DISCUSS-CONTEXT-001"
			role:      "def"
			statement: "Topic names and their slices per the smartskill rule in `context/rules/skills.md`; the default slice carries only that rule and the entry ontology."
			title:     "Topic use"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-CONTEXT-002"
			role:      "rule"
			statement: "Agent MUST widen context only to topics the subject needs — never the whole vocabulary, never a topic for a passing mention."
			title:     "Bounded widening"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-CONTEXT-003"
			role:      "rule"
			statement: "Agent MUST NOT write files, edit code, or spawn agents — dialogue only until the user asks for a handoff."
			title:     "No artifact"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-CONTEXT-004"
			role:      "rule"
			statement: "Turn MUST be at most five sentences of plain prose; a table or list only when the content is one; no headers, no preamble, no restating the user."
			title:     "Short turn"
		}
	}]
	kind: "context"
}, {
	title: "Conversation"
	blocks: [{
		entry: {
			id:        "SKILL-DISCUSS-TALK-001"
			role:      "step"
			statement: "INVOKE the Skill tool on `discuss` with `topic:<name>` plus the subject when the turn hinges on a topic not yet loaded — at most one widening per turn."
			title:     "Widen"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-TALK-006"
			role:      "step"
			statement: "READ the one file or grep the one pattern that settles the turn's claim when loaded context does not — at most one per turn; skip when it does."
			title:     "Look up"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-TALK-007"
			role:      "step"
			statement: "ANSWER what the user said — the question asked, the claim made, the decision stated — uncertainty in the same sentence as the claim."
			title:     "Answer"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-TALK-004"
			role:      "verify"
			statement: "Before a claim about repo state or harness behavior: VERIFY it against loaded context or a read file; unverifiable ⇒ say so in the sentence."
			title:     "Grounding"
		}
	}, {
		entry: {
			id:        "SKILL-DISCUSS-TALK-005"
			pred:      "handoff_wanted"
			role:      "stop"
			statement: "If the user wants the outcome written ⇒ STOP: name the target skill (/idea, /concept, /fdesign, /backlog) and end the discussion."
			title:     "Handoff"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "short grounded turns; one lookup, no fan-out, no artifact"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "topic:repo is one smoke per skill worth its upkeep"
}
