changelog: [{
	date:    "2026-09-27"
	version: "1.0"
	text:    "fixture"
}]
frontmatter: [{
	key:   "name"
	value: " seeded"
}, {
	key:   "description"
	value: " Golden-query fixture skill with a seeded contradiction and a dead transition. Trigger on nothing."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.0"
}, {
	key:   "format"
	value: " 2"
}]
preamble: """
	# seeded

	Fixture body for the solver golden tests.
	"""
sections: [{
	title: "Dispatch"
	blocks: [{
		entry: {
			id:        "SKILL-SEEDED-RULES-001"
			pred:      "mode = interactive => gate_pauses"
			role:      "rule"
			statement: "Interactive runs MUST pause at the gate."
			title:     "Gate pauses"
		}
	}, {
		entry: {
			id:        "SKILL-SEEDED-RULES-002"
			pred:      "mode = interactive => not gate_pauses"
			role:      "rule"
			statement: "Interactive runs MUST NOT pause at the gate."
			title:     "Gate never pauses"
		}
	}, {
		entry: {
			id:        "SKILL-SEEDED-PROC-001"
			pred:      "mode = interactive and mode = nightly"
			role:      "step"
			statement: "RUN the step both modes require at once."
			title:     "Impossible step"
		}
	}]
}]
atoms: {
	gate_pauses: {
		sort: "state"
	}
}
