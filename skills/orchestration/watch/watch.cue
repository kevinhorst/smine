changelog: [{
	date:    "2026-10-14"
	version: "1.3.0"
	text:    "Interfaces: one variable per capability — HEARTBEAT_STATUS"
	tokens:  1409
}, {
	date:    "2026-10-13"
	version: "1.2.0"
	text:    "The fleet tool is the optional env variable $SKILL_WATCH_HEARTBEAT: the schedule step fills the tick prompt's <heartbeat> placeholder from it, unset leaves every tick without heartbeat; the grant reads it."
	tokens:  1399
}, {
	date:    "2026-10-03"
	version: "1.1.0"
	text:    "Requires peek: deploy skips the skill without them."
	tokens:  1330
}, {
	date:    "2026-09-28"
	version: "1.0.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1332
}, {
	date:    "2026-09-28"
	version: "1.0.0"
	text:    "initial version: pins one session by id, ticks every n minutes (default 5) via a literal cron prompt; heartbeat decides liveness (waiting/blocked alerts, ended/died stop, quiet only with no running subagent and unchanged turns and tokens), session_get error stops, one all-sections-off session_get reports turns, tokens incl. subagents and plan revisions, session_events fetches only new plan revisions, self-stops after 3 quiet ticks; Codex targets use token deltas."
}]
env: {
	HEARTBEAT_STATUS: {
		interface: "heartbeat-status@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " watch"
}, {
	key:   "description"
	value: " Watch one session on a cron — heartbeat liveness, one delta line per tick, new plan revisions only, self-stops. Trigger on /watch <title|id> [n] or \"watch session X\". Args — target: session id or exact title, id wins; n: tick interval minutes (default 5)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.3.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " 'Login simplification 2'"
}, {
	key:   "allowed-tools"
	value: " CronCreate, CronDelete, CronList, Bash($SKILL_WATCH_HEARTBEAT_STATUS status *), mcp__peek-mcp__session_events, mcp__peek-mcp__session_get, mcp__peek-mcp__session_list"
}]
preamble: """
	# Watch

	Pins one session, ticks every `n` minutes: heartbeat for liveness, peek for progress, prints only change.
	"""
requires: ["peek"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["peek-mcp at the `PEEK_MCP_VERSION` pin", "heartbeat daemon up for Claude targets"]
			purpose: "recurring low-context observation of another session's liveness, progress and plan evolution."
			routes: [{
				for: "one-shot view of turns, plan, diff, status"
				use: "/peek"
			}, {
				for: "fleet-wide state and merges"
				use: "/fleet"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "target"
				default: "none"
				effect:  "session to watch; id wins over title — [Resolve target](#skill-watch-setup-001)"
				form:    "positional, free text"
			}, {
				name:    "n"
				default: "`5`"
				effect:  "tick interval minutes — [Schedule](#skill-watch-setup-003)"
				form:    "trailing positional int"
			}]
		}
	}]
}, {
	title: "Setup"
	blocks: [{
		entry: {
			id:        "SKILL-WATCH-SETUP-001"
			role:      "step"
			statement: "CALL `session_list` once; UUID-style target matches `id`, else exact case-insensitive `title`."
			title:     "Resolve target"
		}
	}, {
		entry: {
			id:        "SKILL-WATCH-SETUP-002"
			role:      "stop"
			statement: "If zero or several sessions match ⇒ STOP: list candidate titles and ids, schedule nothing."
			title:     "No unique match"
		}
	}, {
		entry: {
			id: "SKILL-WATCH-SETUP-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "fill `<heartbeat>` with the program of `$SKILL_WATCH_HEARTBEAT_STATUS status --session <id> --format json`; else leave it unfilled — no heartbeat."
			}]
			role:      "step"
			statement: "CALL `CronCreate` cron `*/<n> * * * *`, recurring, prompt = [Tick prompt](#skill-watch-tick-001) filled with pinned id and agent."
			title:     "Schedule"
		}
	}, {
		entry: {
			id:        "SKILL-WATCH-SETUP-004"
			role:      "step"
			statement: "RUN the tick once now; STATE job id, target title, interval, 7-day cron expiry."
			title:     "First tick"
		}
	}]
	kind: "phase"
}, {
	title: "Tick"
	blocks: [{
		entry: {
			id: "SKILL-WATCH-TICK-001"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "Literal cron prompt, never a slash command."
			title:     "Tick prompt"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "fixed tool routing, one line per tick"
			tier:   "small"
		}
	}]
}]
smoke: {
	skip: "watches another live session on a cron"
}
