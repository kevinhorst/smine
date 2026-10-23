changelog: [{
	date:    "2026-09-27"
	version: "1.0"
	text:    "fixture"
}]
frontmatter: [{
	key:   "name"
	value: " fail"
}, {
	key:   "description"
	value: " Fixture skill for the skill-model gate. Trigger on /fixture-skill or \"fixture\"."
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
	# fixture-skill

	Fixture body.
	"""
sections: [{
	title: "Dispatch"
	blocks: [{
		entry: {
			id:        "SKILL-FAIL-EXEC-001"
			pred:      "warp_drive and"
			role:      "rule"
			statement: "Dispatcher MUST NOT engage the warp drive."
			title:     "Broken pred"
		}
	}, {
		entry: {
			id: "SKILL-FAIL-PROC-001"
			post: ["unregistered_atom"]
			role:      "step"
			statement: "RESOLVE the base."
			title:     "Unresolved post"
		}
	}]
}]
atoms: {
	broken_level: {
		kind: "enum"
		members: ["broken_level"]
		sort: "level"
	}
	free_text: {
		sort:   "state"
		source: "check: the bracket base does not resolve"
	}
}
tier: "strict"
