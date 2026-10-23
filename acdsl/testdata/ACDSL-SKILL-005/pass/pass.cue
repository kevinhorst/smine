changelog: [{
	date:    "2026-09-27"
	version: "1.0"
	text:    "fixture"
}]
frontmatter: [{
	key:   "name"
	value: " pass"
}, {
	key:   "description"
	value: " pass fixture. Trigger on /pass."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.0"
}]
preamble: "# Pass fixture"
sections: [{
	title: "Intake"
	blocks: [{
		entry: {
			id: "SKILL-PASS-INTAKE-001"
			items: [{
				label:  "Why"
				marker: "*"
				text:   "nothing else is known yet."
			}]
			role:      "step"
			statement: "Ask for the feature name."
			title:     "Feature name"
		}
	}, {
		entry: {
			id:      "SKILL-PASS-INTAKE-002"
			channel: "hook"
			items: [{
				label:  "Hook"
				marker: "*"
				text:   "cmd/acdslhook/main.go"
			}]
			role:      "step"
			statement: "The projection hook writes the governing rules above the file."
			title:     "Projected rules"
		}
	}]
}, {
	title: "Templates"
	blocks: [{
		entry: {
			id: "SKILL-PASS-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Overview template."
			title:     "Overview template"
		}
	}]
}]
