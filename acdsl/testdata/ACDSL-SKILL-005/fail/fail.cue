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
	value: " fail fixture. Trigger on /fail."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.0"
}]
preamble: "# Fail fixture"
sections: [{
	title: "Intake"
	blocks: [{
		entry: {
			id:        "SKILL-FAIL-INTAKE-002"
			channel:   "hook"
			role:      "step"
			statement: "Hook channel without a Hook bullet."
			title:     "Claimless hook"
		}
	}, {
		entry: {
			id:      "SKILL-FAIL-INTAKE-003"
			channel: "gate"
			items: [{
				label:  "Gate"
				marker: "*"
				text:   "ACDSL-NOPE-999"
			}]
			role:      "step"
			statement: "Gate bullet naming an undeclared rule."
			title:     "Unknown gate"
		}
	}, {
		entry: {
			id:        "SKILL-FAIL-INTAKE-004"
			role:      "step"
			statement: "Pick one of `a, b, c, d, e, f, g`."
			title:     "Long list"
		}
	}]
}]
