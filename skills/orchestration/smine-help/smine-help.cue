answeredBy: "hook"
changelog: [{
	date:    "2026-10-17"
	version: "1.2.3"
	text:    "Help text: the inject group lists env, which also stops a skill whose required repo value is unset."
	tokens:  2181
}, {
	date:    "2026-10-17"
	version: "1.2.2"
	text:    "Model file is <leaf>.cue: the read-gate switch examples name it."
	tokens:  2179
}, {
	date:    "2026-10-13"
	version: "1.2.1"
	text:    "Help text: switches are shown on the session context page; inject:global covers subagent start."
	tokens:  2173
}, {
	date:    "2026-10-13"
	version: "1.2.0"
	text:    "The #allow settings.local.json row names the gate's allow command instead of the gate program."
	tokens:  2138
}, {
	date:    "2026-10-13"
	version: "1.1.0"
	text:    "Switch grammar: grouped single-word features (rgate, skill, inject, session, hb), group words, member lists and per-guide read-gate members; the catalog says what each feature keeps on."
	tokens:  2145
}, {
	date:    "2026-10-12"
	version: "1.0.0"
	text:    "Initial skill: the typed session messages explained by the prompt hook, never by a model."
	tokens:  1299
}]
frontmatter: [{
	key:   "name"
	value: " smine-help"
}, {
	key:   "description"
	value: " Explains the typed session messages — permissions and switches — with examples; answered by the prompt hook, never by a model. Trigger on /smine-help."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.2.3"
}, {
	key:   "format"
	value: " 2"
}]
preamble: """
	# smine help

	The typed `#` messages of a session — permissions and switches — explained with examples. The skillhelp hook blocks the
	prompt and prints the text below plus the live feature catalog.
	"""
requires: ["gate", "session-helpers"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["the gate and the session-helpers hooks are deployed"]
			purpose: "shows how the typed session messages work, as a hook answer."
			routes: [{
				for: "a skill's own help"
				use: "/<skill> --help"
			}, {
				for: "a quick question about anything else"
				use: "/qq"
			}]
		}
	}]
}, {
	title: "Answer"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEHELP-ANSWER-001"
			role:      "rule"
			statement: "Agent MUST NOT answer this skill; the prompt hook blocks `/smine-help` and prints the payload below with the feature catalog, and the settings deny the Skill tool on it."
			title:     "Hook answers"
		}
	}, {
		entry: {
			id: "SKILL-SMINEHELP-TEXT-001"
			payload: {
				info:   "markdown"
				marker: "````"
			}
			role:      "payload"
			statement: "The text the hook prints; the feature catalog follows it, rendered from the gate's code."
			title:     "Help text"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "no model runs — the hook answers"
			tier:   "small"
		}
	}]
}]
smoke: {
	skip: "the prompt hook blocks the invocation; the smoke runner never sees a model turn (cmd/skillhelp tests cover the block)"
}
