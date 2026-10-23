changelog: [{
	date:    "2026-09-27"
	version: "1.0"
	text:    "fixture"
}]
frontmatter: [{
	key:   "name"
	value: " causal"
}, {
	key:   "description"
	value: " Golden-query fixture skill with a causally dead step. Trigger on nothing."
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
	# causal

	Fixture body for the causal-reachability golden tests.
	"""
sections: [{
	title: "Run"
	blocks: [{
		entry: {
			id:        "SKILL-CAUSAL-PROC-001"
			pred:      "prepared"
			role:      "step"
			statement: "RUN the prepared step."
			title:     "Needs preparation"
		}
	}, {
		entry: {
			id:        "SKILL-CAUSAL-PROC-002"
			role:      "step"
			statement: "RUN the free step."
			title:     "Free step"
		}
	}]
}, {
	title: "Later"
	blocks: [{
		entry: {
			id: "SKILL-CAUSAL-PROC-003"
			post: ["prepared"]
			role:      "verify"
			statement: "After run: VERIFY preparation; prepare."
			title:     "Prepare late"
		}
	}]
}]
atoms: {
	prepared: {
		sort: "state"
	}
}
