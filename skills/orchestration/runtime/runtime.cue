changelog: [{
	date:    "2026-10-14"
	version: "3.4.0"
	text:    "Interfaces: one variable per capability — HEARTBEAT_RUNTIME"
	tokens:  2210
}, {
	date:    "2026-10-13"
	version: "3.3.0"
	text:    "The fleet tool is the required env variable $SKILL_RUNTIME_HEARTBEAT in every runtime call, the precondition and the grant."
	tokens:  2168
}, {
	date:    "2026-10-03"
	version: "3.2.0"
	text:    "Requires heartbeat: deploy skips the skill without them."
	tokens:  2125
}, {
	date:    "2026-10-02"
	version: "3.1.2"
	text:    "heartbeat kills on a check's eighth failure, each failure before it raises a finding, and the prompt that lifts a kill resets the session's runs."
	tokens:  2127
}, {
	date:    "2026-09-29"
	version: "3.1.1"
	text:    "Act's send param names every hand-over point: next successful tool call, or a waiting root's Stop or next prompt."
	tokens:  2120
}, {
	date:    "2026-09-29"
	version: "3.1.0"
	text:    "when the watched session ends, heartbeat's last digest says `watch ended` and the watcher stops with its report instead of waiting."
}, {
	date:    "2026-09-29"
	version: "3.0.0"
	text:    "a watch is one-to-one: the required session arg names the one session to watch (id or exact title); only its findings reach this watcher and only they can be answered; a session has at most one watcher, and several watchers run side by side."
}, {
	date:    "2026-09-29"
	version: "2.0.0"
	text:    "findings are enforce check failures and judgment points from the skills' enforce blocks, no longer live-conformance verdicts; act mode is the recovery step before heartbeat escalates — a check's first failure frees its recovery template, a second failure kills the run in heartbeat."
}, {
	date:    "2026-09-29"
	version: "1.0.0"
	text:    "initial version: singleton heartbeat runtime subscriber; report mode lists findings of live-checked skill runs, act mode answers each finding once with heartbeat's rendered template (hook context on the observed agent's next successful tool call) or a stop, per the decision table."
}]
env: {
	HEARTBEAT_RUNTIME: {
		interface: "heartbeat-runtime@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " runtime"
}, {
	key:   "description"
	value: " Watch one session's enforced skill runs — report failed checks and judgment points, or answer each with its template or a stop before heartbeat escalates. Trigger on /runtime or \"watch skill runs live\". Args — session: the one session to watch, id or exact title (required); mode: report|act (default report)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.4.0"
}, {
	key:   "argument-hint"
	value: " session: \"CC,HD - Heartbeat demo recover - [WIP]\" mode: act"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "title-code"
	value: " Rt"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Read, Bash($SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime *)"
}]
preamble: """
	# Runtime

	Watches exactly one other session through heartbeat: its enforced skill runs raise findings — a check's failure
	short of the kill or a judgment point — that arrive as digests; act mode answers a finding once, through heartbeat
	only.
	"""
requires: ["heartbeat"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["heartbeat serving (`$SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime status` exits 0)", "the session to watch is live and has no other watcher"]
			purpose: "failed enforce checks and judgment points on one other session's skill runs, reported or answered with heartbeat's moves."
			routes: [{
				for: "running a heartbeat fleet"
				use: "/fleet"
			}, {
				for: "one session's liveness and progress on a cron"
				use: "/watch"
			}, {
				for: "offline skill checks across arms"
				use: "/skillroutine-eval"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "session"
				effect: "`session: <id or exact title>`, required — the one session to watch; nothing else reaches this watcher — [Subscribe](#skill-runtime-start-001)"
			}, {
				name:   "mode"
				effect: "`mode: report|act`, default `report` — act also answers findings; fixed at subscribe — [Subscribe](#skill-runtime-start-001)"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id:        "SKILL-RUNTIME-DEF-001"
			role:      "def"
			statement: "New turn from a heartbeat runtime digest or skill start. Carries no facts — runtime status does."
			title:     "Wake"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-DEF-002"
			role:      "def"
			statement: "Output of `$SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime status --format json`; sole source of runs and findings per wake."
			title:     "Runtime status"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-DEF-003"
			role:      "def"
			statement: "`findings[]` row: a `check_failed` or `judgment` with an `id`, its `evidence`, its allowed `moves` and its `acted` state."
			title:     "Finding"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-RUNTIME-RULES-001"
			role:      "rule"
			statement: "Runtime MUST NOT message or stop any session except through `$SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime send|stop <finding>`."
			title:     "Heartbeat moves only"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-RULES-002"
			role:      "rule"
			statement: "Runtime MUST NOT answer a finding twice or answer a finding of a `killed` run."
			title:     "Single move"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-RULES-003"
			role:      "rule"
			statement: "In report mode runtime MUST NOT run `send` or `stop`."
			title:     "Report mode passive"
		}
	}]
	kind: "context"
}, {
	title: "Start"
	blocks: [{
		entry: {
			id:        "SKILL-RUNTIME-START-001"
			role:      "step"
			statement: "RUN `$SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime subscribe --target <session> --mode <mode>`."
			title:     "Subscribe"
		}
	}, {
		entry: {
			id: "SKILL-RUNTIME-START-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "exit 1 refusal (no such session, already watched, a watcher itself, or itself), 2 usage, 3 unreachable, 4 foreign home."
			}]
			role:      "stop"
			statement: "If subscribe exits non-zero ⇒ STOP: relay stderr verbatim."
			title:     "Subscribe failed"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-START-003"
			role:      "step"
			statement: "END turn; next wake enters Loop."
			title:     "Yield"
		}
	}]
	kind: "phase"
}, {
	title: "Loop"
	blocks: [{
		entry: {
			id:        "SKILL-RUNTIME-LOOP-001"
			role:      "step"
			statement: "READ [Runtime status](#skill-runtime-def-002) on every wake."
			title:     "Read status"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-LOOP-006"
			role:      "stop"
			statement: "If the digest says `watch ended` ⇒ STOP: report its undelivered findings, runs watched and moves made; heartbeat already ended the watch, no unsubscribe."
			title:     "Watch ended"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-LOOP-002"
			role:      "step"
			statement: "REPORT each new finding of the watched session in one line: skill, kind, check or judgment id, reason, evidence."
			title:     "Report findings"
		}
	}, {
		entry: {
			id: "SKILL-RUNTIME-LOOP-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "send: heartbeat renders the finding's own template; the agent gets it on its next successful tool call, a waiting root at its Stop or next prompt."
			}, {
				label:  "param"
				marker: "*"
				text:   "stop: heartbeat ends the observed agent's tool calls until its next prompt (root) or its end (subagent)."
			}]
			pred:      "mode = act"
			role:      "step"
			statement: "DECIDE each open [Finding](#skill-runtime-def-003) per [Decision table](#skill-runtime-tpl-001); RUN `$SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime send|stop <finding>`."
			title:     "Act"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-LOOP-004"
			role:      "step"
			statement: "STATE every move and its outcome from the command's output."
			title:     "State moves"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-LOOP-005"
			role:      "step"
			statement: "END turn; next wake re-enters Loop."
			title:     "Yield"
		}
	}]
	kind: "phase"
}, {
	title: "Stop"
	blocks: [{
		entry: {
			id:        "SKILL-RUNTIME-STOP-001"
			pred:      "user_stop_requested"
			role:      "step"
			statement: "RUN `$SKILL_RUNTIME_HEARTBEAT_RUNTIME runtime unsubscribe`."
			title:     "Unsubscribe"
		}
	}, {
		entry: {
			id:        "SKILL-RUNTIME-STOP-002"
			role:      "stop"
			statement: "If unsubscribed ⇒ STOP: report runs watched and moves made."
			title:     "Done"
		}
	}]
	kind: "phase"
}, {
	title: "Templates"
	blocks: [{
		entry: {
			id: "SKILL-RUNTIME-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:  "payload"
			title: "Decision table"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "act decides steering moves on the watched session's runs from verdict evidence"
			tier:   "frontier"
			routes: [{
				effort: "medium"
				route:  "report"
				tier:   "mid-tier"
			}]
		}
	}]
}]
smoke: {
	skip: "watches another live session's enforced runs"
}
