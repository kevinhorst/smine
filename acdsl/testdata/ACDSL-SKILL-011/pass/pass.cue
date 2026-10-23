changelog: [{
	date:    "2026-09-28"
	version: "1.0"
	text:    "initial"
}]
frontmatter: [{
	key:   "name"
	value: " pass"
}, {
	key:   "description"
	value: " Fixture skill for the routing gate. Trigger on /fixture-skill or \"fixture\"."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.0"
}]
preamble: """
	# fixture-skill

	One-sentence fixture summary.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["a testable condition"]
			purpose: "one fixture sentence."
			routes: [{
				for: "something else"
				use: "/other-skill"
			}]
		}
	}]
}]
