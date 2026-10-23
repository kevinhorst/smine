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
}, {
	key:   "plan-template"
	value: " SKILL-FAIL-TPL-001"
}]
preamble: "# Fail fixture"
sections: [{
	title: "Templates"
	blocks: [{
		entry: {
			id: "SKILL-FAIL-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Plan template with three violations."
			title:     "Broken template"
		}
	}]
}]
