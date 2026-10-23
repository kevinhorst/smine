atoms: {
	fixes_applied: {
		aliases: ["fixes land", "fix landed"]
		sort: "event"
	}
	mode: {
		aliases: ["headless", "unattended"]
		kind: "enum"
		members: ["interactive", "nightly"]
		sort:   "mode"
		source: "session | arg: nightly => nightly"
	}
	mode_switch_tool: {
		aliases: ["Desktop session tool", "set_session_permission_mode"]
		sort:    "state"
		source:  "session"
	}
}
