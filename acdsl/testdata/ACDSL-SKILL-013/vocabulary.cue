atoms: {
	mode: {
		aliases: ["headless", "unattended"]
		kind:    "enum"
		members: ["interactive", "nightly"]
		sort:    "mode"
		source:  "session | arg: nightly => nightly"
	}
}
