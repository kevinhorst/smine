changelog: [{
	date:    "2026-10-14"
	version: "1.4.0"
	text:    "Interfaces: one variable per capability — HEARTBEAT_JOIN"
	tokens:  533
}, {
	date:    "2026-10-13"
	version: "1.3.0"
	text:    "The fleet tool is the required env variable $SKILL_JOIN_HEARTBEAT in the join step and its grant."
	tokens:  528
}, {
	date:    "2026-10-03"
	version: "1.2.0"
	text:    "Requires heartbeat: deploy skips the skill without them."
	tokens:  524
}, {
	date:    "2026-10-02"
	version: "1.1.0"
	text:    "allowed-tools gains Skill(done): the settings template no longer admits the done call, so a member's agent reaches /done through this grant."
	tokens:  526
}, {
	date:    "2026-09-28"
	version: "1.0.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  526
}, {
	date:    "2026-09-28"
	version: "1.0.0"
	text:    "initial version: joins the session to a heartbeat fleet through smine-heartbeat join; heartbeat assigns the member letter, the sessiontitle hook writes the token into the title."
}]
env: {
	HEARTBEAT_JOIN: {
		interface: "heartbeat-join@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " join"
}, {
	key:   "description"
	value: " Join this session to a heartbeat fleet — heartbeat assigns the member letter, the title shows it. Trigger on /join or \"join fleet T1\". Args — fleet: FleetId."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.4.0"
}, {
	key:   "argument-hint"
	value: " 'fleet: T1'"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "title-code"
	value: " Join"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Skill(done), Bash($SKILL_JOIN_HEARTBEAT_JOIN join *)"
}]
preamble: """
	# Join

	Joins this session to a heartbeat fleet as a member; heartbeat records it and names it `<fleet>-M<letter>`.
	"""
requires: ["heartbeat"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "to"
					skill: "done"
				}]
				note:       "the orchestrator of the fleet merges the member once it runs /done"
				standalone: true
			}
			preconditions: ["heartbeat daemon serves this Claude home"]
			purpose: "makes this session a fleet member its orchestrator can see, message and merge."
			routes: [{
				for: "running a fleet as its orchestrator"
				use: "/fleet"
			}, {
				for: "telling the orchestrator this member finished"
				use: "/done"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "fleet"
				effect: "`fleet: <FleetId>`, required — the fleet to join, letters, digits and `-`"
			}]
		}
	}]
}, {
	title: "Join"
	blocks: [{
		entry: {
			id: "SKILL-JOIN-RUN-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a title rename to the token follows from the session-title hook — carry it out."
			}]
			role:      "step"
			statement: "RUN `$SKILL_JOIN_HEARTBEAT_JOIN join --fleet <fleet>`; stdout is the member token, e.g. `T1-MA`."
			title:     "Join"
		}
	}, {
		entry: {
			id:        "SKILL-JOIN-RUN-002"
			role:      "stop"
			statement: "If join exits non-zero ⇒ STOP: relay stderr verbatim; never retry with another fleet."
			title:     "Refused"
		}
	}, {
		entry: {
			id:        "SKILL-JOIN-RUN-003"
			role:      "step"
			statement: "STATE `joined <token>`; nothing else."
			title:     "Report"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "one command, one relay"
			tier:   "small"
		}
	}]
}]
smoke: {
	skip: "joins the session to a live heartbeat fleet"
}
