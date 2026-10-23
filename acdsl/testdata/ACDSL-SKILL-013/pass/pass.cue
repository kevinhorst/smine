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
			id:        "SKILL-PASS-EXEC-001"
			pred:      "mode != interactive"
			role:      "rule"
			statement: "Dispatcher MUST NOT run the fixture interactively."
			title:     "Never interactive"
		}
	}, {
		entry: {
			id: "SKILL-PASS-PROC-001"
			post: ["resolve_base_fixture"]
			role:      "step"
			statement: "RESOLVE the base."
			title:     "Resolve base"
		}
	}, {
		entry: {
			id:        "SKILL-PASS-PROC-002"
			pred:      "mode = nightly"
			role:      "stop"
			statement: "If nightly ⇒ STOP: report."
			title:     "Nightly stop"
		}
	}, {
		entry: {
			id:        "SKILL-PASS-PROC-003"
			pred:      "fixture_budget < 1 and fixture_level >= fixture_high"
			role:      "stop"
			statement: "If budget exhausted at high level ⇒ STOP: report."
			title:     "Budget stop"
		}
	}]
}]
atoms: {
	fixture_budget: {
		kind: "int"
		sort: "count"
	}
	fixture_level: {
		kind: "enum"
		members: ["fixture_low", "fixture_high"]
		sort: "level"
	}
	resolve_base_fixture: {
		sort: "event"
	}
}
