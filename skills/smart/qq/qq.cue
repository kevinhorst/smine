changelog: [{
	date:    "2026-10-02"
	version: "1.0.4"
	text:    "Dropped the help-only Eval section — --help shows the smoke and eval declared beside the skill."
	tokens:  809
}, {
	date:    "2026-09-28"
	version: "1.0.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  809
}, {
	date:    "2026-09-27"
	version: "1.0.2"
	text:    "Bracket and arm tokens use <model>:<effort>"
}, {
	date:    "2026-09-26"
	version: "1.0.1"
	text:    "Added the help-only ## Eval section: a /skillroutine-eval smoke invocation; shown by --help, stripped on deploy."
}, {
	date:    "2026-09-24"
	version: "1.0.0"
	text:    "Initial smartskill: one question, ≤5-sentence answer, one lookup, one widening, default slice = smartskill rule only"
}]
frontmatter: [{
	key:   "name"
	value: " qq"
}, {
	key:   "description"
	value: " One quick question, one short answer — minimal context, no tools unless the answer needs a lookup. Trigger on /qq or \"quick question\". Args — topic: context slice(s) to load."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.0.4"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"topic:harness how many Agent-tool subagents run concurrently\""
}, {
	key:   "allowed-tools"
	value: " Glob, Grep, Read"
}, {
	key:   "acdsl-context"
	value: " ACTION-*, FACT-*, RULE-*"
}]
preamble: """
	# Quick Question

	One question in, one short answer out — plain prose, no headers, no lists unless the answer is a list.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["one question in the invocation"]
			purpose: "answer a single question fast and cheaply."
			routes: [{
				for: "a subject to talk through"
				use: "/discuss"
			}, {
				for: "an answer that needs sources and citations"
				use: "/research"
			}, {
				for: "a verdict on a piece of code"
				use: "/code-verdict"
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
				effect:  "context slice(s) injected; `--no-context` skips all — [Topic use](#skill-qq-context-001)"
				form:    "`topic:<name>`, repeatable"
			}]
		}
	}]
}, {
	title: "Context"
	blocks: [{
		entry: {
			id:        "SKILL-QQ-CONTEXT-001"
			role:      "def"
			statement: "Topic names and their slices per the smartskill rule in `context/rules/skills.md`; the default slice carries that rule only."
			title:     "Topic use"
		}
	}, {
		entry: {
			id:        "SKILL-QQ-CONTEXT-002"
			role:      "rule"
			statement: "Answer MUST be at most five sentences of plain prose; a list only when the answer is a list; no headers, no preamble."
			title:     "Short answer"
		}
	}, {
		entry: {
			id:        "SKILL-QQ-CONTEXT-003"
			role:      "rule"
			statement: "Agent MUST NOT write files, edit code, or spawn agents."
			title:     "No artifact"
		}
	}]
	kind: "context"
}, {
	title: "Answer"
	blocks: [{
		entry: {
			id:        "SKILL-QQ-ANSWER-001"
			role:      "step"
			statement: "INVOKE the Skill tool on `qq` with `topic:<name>` plus the question when the answer hinges on a topic not yet loaded — at most one widening."
			title:     "Widen"
		}
	}, {
		entry: {
			id:        "SKILL-QQ-ANSWER-002"
			role:      "step"
			statement: "READ the one file or grep the one pattern that settles the question when loaded context does not; skip when it does."
			title:     "Look up"
		}
	}, {
		entry: {
			id:        "SKILL-QQ-ANSWER-003"
			role:      "step"
			statement: "ANSWER directly, uncertainty stated in the same sentence as the claim."
			title:     "Answer"
		}
	}, {
		entry: {
			id:        "SKILL-QQ-ANSWER-004"
			pred:      "question_not_quick"
			role:      "stop"
			statement: "If the answer needs more than one lookup or one widening ⇒ STOP: answer what is known and name /research or /discuss."
			title:     "Not quick"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "single lookup, short answer"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "how many skill groups sit under skills/"
}
