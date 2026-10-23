changelog: [{
	date:    "2026-10-03"
	version: "3.4.0"
	text:    "Requires peek: deploy skips the skill without them. mining routes tagged with needs: partial installs and the bundle drop them."
	tokens:  1780
}, {
	date:    "2026-10-05"
	version: "3.3.0"
	text:    "Declares the alias pk: /pk runs peek exactly as /peek does"
	tokens:  1780
}, {
	date:    "2026-09-28"
	version: "3.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1780
}, {
	date:    "2026-09-28"
	version: "3.2.0"
	text:    "Rewired to the live peek-mcp tools: session_get with section flags replaces session_full/session_plan/session_diff, allowed-tools on the single mcp__peek-mcp namespace, title matching documented as exact-then-substring."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kinds (Routing phase, Pagination/Output format context), pred/post clauses on 11/12 coverable entries, sibling atoms.json (15 local bool atoms), three queries (id precedence, no-separate-pagination-calls, peek-unavailable stop-blocking), caveman compression pass."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline) placed above the existing routing table, no other content changed"
}, {
	date:    "2026-09-07"
	version: "1.11"
	text:    "Added the status mode (session_events — bounded counters/cost/time) and the monitoring-loop rule: recurring watchdog peeks use status, never full turns (prompt-too-long loop death, 2026-09-06)"
}, {
	date:    "2026-09-05"
	version: "1.10"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-08-23"
	version: "1.9"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.8"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-07-30"
	version: "1.7"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-30"
	version: "1.6"
	text:    "moved under skills/orchestration/ group; name and behavior unchanged"
}, {
	date:    "2026-07-27"
	version: "1.5"
	text:    "reference renames — /smine is now the pipeline (raw miner /smine-batch), /couchskill-eval → /skillroutine-eval"
}, {
	date:    "2026-07-26"
	version: "1.4"
	text:    "Args section"
}, {
	date:    "2026-07-22"
	version: "1.3"
	text:    "effort token normalized (small / low)"
}, {
	date:    "2026-07-19"
	version: "1.2"
	text:    "reference rename: eval-skill → couchskill-eval; moved under skills/util/"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-06-22"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " peek"
}, {
	key:   "aliases"
	value: " pk"
}, {
	key:   "description"
	value: " Show the latest Claude Code or Codex session — turns, plan, diff, status, or list. Trigger on /peek [n] or \"what Claude Code or Codex is doing\". Args — n: recent-turn count (default 5); list|plan|diff|status: mode token (status = bounded counters for monitoring loops); id or title: specific session, id wins; claude|codex: agent qualifier (default claude)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.4.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " '10 list\\|plan\\|diff\\|status my-session-title claude\\|codex'"
}, {
	key:   "allowed-tools"
	value: " ToolSearch, mcp__peek-mcp__session_events, mcp__peek-mcp__session_get, mcp__peek-mcp__session_list"
}]
preamble: """
	# Peek

	Shows latest Claude Code or Codex session — turns, plan, diff, status, or list.
	"""
requires: ["peek"]
sections: [{
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "n"
				default: "`5`"
				effect:  "number of recent turns shown"
				form:    "positional int"
			}, {
				name:    "list\\|plan\\|diff\\|status"
				default: "full-turn view"
				effect:  "session list / current plan / diff / bounded status — [Monitoring status](#skill-peek-route-005)"
				form:    "mode token"
			}, {
				name:    "id or title"
				default: "none"
				effect:  "targets a specific session; id wins over title — [ID precedence](#skill-peek-route-002)"
				form:    "positional, free text"
			}, {
				name:    "claude\\|codex"
				default: "`claude`"
				effect:  "agent qualifier — [Agent param](#skill-peek-route-004)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["peek-mcp available"]
			purpose: "quick-views latest Claude Code or Codex session — recent turns, plan, diff, or session list."
			routes: [{
				for: "systematically mining sessions for lessons"
				use: "/smine (pipeline; raw miner /smine-batch)"
				needs: ["mining"]
			}, {
				for: "reading session transcripts for memory/rule extraction"
				use: "/smine"
				needs: ["mining"]
			}, {
				for: "evaluating skill outputs"
				use: "/skillroutine-eval"
			}]
		}
	}, {
		prose: """
			| Input | Tool | Notes |
			|-------|------|-------|
			| `/peek [n]`, "what is Claude doing", "show session" | `session_get` `n` | n defaults to 5 |
			| `/peek list` | `session_list` | shows all sessions with plan/diff flags |
			| `/peek plan` | `session_get` `turns: false, events: false, diff: false` | current plan only |
			| `/peek diff` | `session_get` `turns: false, events: false, plan: false` | git diff only |
			| `/peek status [id|title]` | `session_events` | bounded status — counters, cost, wall/active time, no turns |
			| `/peek <id>` or `/peek <id> [n]` | `session_get` with `id` param | specific session by ID |
			| `/peek <title>` or `/peek <title> [n]` | `session_get` with `title` param | exact case-insensitive, else substring |
			"""
	}, {
		entry: {
			id:        "SKILL-PEEK-ROUTE-005"
			pred:      "recurring_check => status_mode"
			role:      "rule"
			statement: "Agent MUST use `status`, never full-turn peek, for recurring session check — full-turn payloads compound, once killed a monitoring loop via prompt-too-long (2026-09-06)."
			title:     "Monitoring status"
		}
	}, {
		entry: {
			id:        "SKILL-PEEK-ROUTE-002"
			pred:      "(id_provided and title_provided) => id_used"
			role:      "rule"
			statement: "`id` MUST take precedence over `title` when both are provided."
			title:     "ID precedence"
		}
	}, {
		entry: {
			id:        "SKILL-PEEK-ROUTE-003"
			role:      "rule"
			statement: "Title matching MUST follow server order: exact case-insensitive first, else substring; confirmation line NEVER omits matched title."
			title:     "Exact title match"
		}
	}, {
		entry: {
			id: "SKILL-PEEK-ROUTE-006"
			post: ["status_reported"]
			pred:      "status_mode"
			role:      "step"
			statement: "STATE one compact status report per call: session id, activity/turn counters, cost, wall/active time, last-activity timestamp."
			title:     "Status report"
		}
	}, {
		entry: {
			id:        "SKILL-PEEK-ROUTE-001"
			pred:      "id_provided or title_provided"
			role:      "step"
			statement: "PASS UUID-style input as `id`; else as `title`."
			title:     "ID/title dispatch"
		}
	}, {
		entry: {
			id: "SKILL-PEEK-ROUTE-004"
			post: ["agent_param_passed"]
			role:      "step"
			statement: "PASS required `agent` param (`\"claude\"` or `\"codex\"`) to every tool call — qualify when user names an agent (e.g. `/peek codex`), else default `claude`."
			title:     "Agent param"
		}
	}]
	kind: "phase"
}, {
	title: "Pagination"
	blocks: [{
		entry: {
			id:        "SKILL-PEEK-PAGE-001a"
			pred:      "(has_more and request_id_present) => refetched"
			role:      "rule"
			statement: "Agent MUST re-call `session_get` with returned `request_id` while `has_more` true, until false or `request_id` absent."
			title:     "Follow request_id"
		}
	}, {
		entry: {
			id:        "SKILL-PEEK-PAGE-002"
			pred:      "not separate_call_made"
			role:      "rule"
			statement: "Agent MUST make one `session_get` call per view, selecting turns/plan/diff by section flags — never one call per section."
			title:     "No separate calls"
		}
	}, {
		entry: {
			id:        "SKILL-PEEK-PAGE-001b"
			pred:      "peek_unavailable"
			role:      "stop"
			statement: "If `session_get` doesn't answer (peek-mcp unavailable) ⇒ STOP: report as blocker; never substitute direct transcript reads."
			title:     "Peek unavailable"
		}
	}]
	kind: "context"
}, {
	title: "Output format"
	blocks: [{
		entry: {
			id: "SKILL-PEEK-OUTPUT-001"
			custom: ["todo:unmeasured"]
			pred:      "not raw_reproduced"
			role:      "rule"
			statement: "Agent MUST NOT reproduce tool result — data already in context for LLM; re-formatting wastes time and tokens."
			title:     "No raw reproduction"
		}
	}, {
		entry: {
			id: "SKILL-PEEK-OUTPUT-003"
			post: ["confirmation_stated"]
			role:      "step"
			statement: "INCLUDE session title or ID, turn count, and which sections are present (plan/diff) — nothing else."
			title:     "Confirmation contents"
		}
	}, {
		entry: {
			id:        "SKILL-PEEK-OUTPUT-004"
			pred:      "list_mode"
			role:      "step"
			statement: "SHOW session table as-is for `/peek list` — already compact."
			title:     "List mode as-is"
		}
	}, {
		entry: {
			id: "SKILL-PEEK-OUTPUT-002"
			payload: {
				marker: "```"
			}
			role:      "payload"
			statement: "After calling tool, respond with only short confirmation line, e.g.:"
			title:     "Confirmation line"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "table-driven routing to MCP tools"
			tier:   "small"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "list"
}
