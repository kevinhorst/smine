changelog: [{
	date:    "2026-10-14"
	version: "6.2.0"
	text:    "Interfaces: one variable per capability — TASK_CHECK"
	tokens:  3563
}, {
	date:    "2026-10-13"
	version: "6.1.0"
	text:    "Programs are env variables: $SKILL_SMINEPERMISSIONS_RULES (required) in the write verify and the caps item; the widening route names the gate's allow self-service without a program."
	tokens:  3553
}, {
	date:    "2026-10-13"
	version: "6.0.1"
	text:    "Widening route names the user's own #set and #add channel beside toolpolicy allow."
	tokens:  3519
}, {
	date:    "2026-10-09"
	version: "6.0.0"
	text:    "Task problem is one plain sentence ≤ 160 runes gated by task-check; evidence carries the record finding index."
	tokens:  3493
}, {
	date:    "2026-10-08"
	version: "5.4.0"
	text:    "Task text caps: problem ≤ 320, fix ≤ 240, quotes ≤ 160 as verbatim excerpts; smine-rules task-check replaces the jq validate"
	tokens:  3400
}, {
	date:    "2026-10-08"
	version: "5.3.0"
	text:    "Task carries problem, evidence and fix instead of finding."
	tokens:  3306
}, {
	date:    "2026-10-04"
	version: "5.2.0"
	text:    "Settings deploy named by the install engine: tasks target the gate fragment settings/claude_code/components/gate.json."
	tokens:  3205
}, {
	date:    "2026-10-04"
	version: "5.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  3205
}, {
	date:    "2026-10-04"
	version: "5.0.1"
	text:    "Session records are read as the JSON files make sessions lists; the markdown record is gone."
	tokens:  3205
}, {
	date:    "2026-10-04"
	version: "5.0.0"
	text:    "Miner, not planner: the plan route is gone; route mine reads the unsynthesized records' permission blocks plus the denial ledger and writes allow-rule change tasks for settings/claude_code/settings.json — gate candidates only, intended-cost denials dropped, capped by --cap."
	tokens:  3206
}, {
	date:    "2026-09-28"
	version: "4.0.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1827
}, {
	date:    "2026-09-26"
	version: "4.0.2"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "4.0.1"
	text:    "Make-only entry points: allowed-tools grants make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "4.0.0"
	text:    "Planner contract: mining from the denial ledger retired (smine-problems reads it); plan route returns one allowlist rule for a problem direction and writes nothing; dialect and suppression knowledge kept."
}, {
	date:    "2026-09-23"
	version: "2.3.1"
	text:    "ledger field list documents agent_id and agent_type — the gate stamps subagent denials so lane denials separate from dispatcher denials"
}, {
	date:    "2026-09-22"
	version: "2.3.0"
	text:    "input switched from smine-batch permission blocks to the tool-policy gate's denial ledger (~/.claude/tool-policy/denials.jsonl) — hook denials never reached the batch block; candidates key on the ledger's candidate_rule, compounds and deny-rule hits drop by cause, the batch-file arg became since"
}, {
	date:    "2026-09-17"
	version: "2.2.0"
	text:    "repurposed for closed-world permissions — proposals derive from commands denied in >=2 distinct sessions; grant mining and the negative-evidence policy retired; compound-command denials dropped as gate-by-design"
}, {
	date:    "2026-09-15"
	version: "2.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "2.1.0"
	text:    "Formal-schema reformat: section-kind clause lines (context/phase), pred/post clauses on 25 entries (12 invariants, 10 guarded transitions), sibling atoms.json (26 bool, 1 int, 1 enum), three queries under queries/, step/verify bullets reregistered to param:, caveman compression pass."
}, {
	date:    "2026-09-14"
	version: "2.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-08"
	version: "1.1.0"
	text:    "Caveman trim of over-cap entry statements (wording only, cap now 320 display runes)"
}, {
	date:    "2026-09-07"
	version: "1.0.0"
	text:    "Migrated to skill format v2 — Routing section (Purpose/Disambiguation/Preconditions/Pipeline) replaces When to use, one-sentence intro, bullet Args synced with argument-hint, allowed-tools reordered; body entries, IDs, and technical content unchanged"
}, {
	date:    "2026-09-05"
	version: "0.1.1"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-08-30"
	version: "0.1.0"
	text:    "initial version — mines smine-batch permissions blocks (peek-mcp v1.2.2 grant capture) into proposals/permissions.json; allow-rules only, narrowest dialect-conformant rule, ≥2-session recurrence gate, live-allowlist + archive suppression, denials as negative evidence"
}]
env: {
	TASK_CHECK: {
		interface: "task-check@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-permissions"
}, {
	key:   "description"
	value: " Mine the session records' permission blocks and the tool-policy denial ledger into allow-rule change tasks for the settings template. Trigger on /smine-permissions mine or the smine permissions stage. Args — mine: read the unsynthesized records, write tasks; --nightly: unattended; --cap: max tasks this run (default 5)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 6.2.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"mine --nightly --cap 3\""
}, {
	key:   "allowed-tools"
	value: " Read, Read(~/.claude/tool-policy/**), Write, Bash(jq *), Bash(make sessions *)"
}, {
	key:   "acdsl-context"
	value: " ACTION-IMPL-ARCH-004, ACTION-IMPL-EXEC-*, FACT-REPO-SEC-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}]
preamble: """
	# smine: permissions miner

	Reads the unsynthesized session records' permission blocks and the tool-policy gate's denial ledger, turns recurring
	denials of commands the closed world should admit into allow-rule change tasks under `.smine-run/changes/`; never edits
	a settings file or writes a store.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "smine"
				}]
				note: "permissions stage of the nightly; the implement stage turns each task into one commit"
			}
			preconditions: ["at least one session record without `synthesized`", "`settings/claude_code/components/gate.json` present — the live allowlist a candidate is checked against"]
			purpose: "the permissions miner — turns recurring gate denials in session records into allow-rule change tasks for `settings/claude_code/components/gate.json`; never edits settings."
			routes: [{
				for: "mining transcripts into session records"
				use: "/smine-batch"
			}, {
				for: "widening this session's permissions now"
				use: "the gate's `allow` self-service after the user's `#allow settings.local.json`, or the user's own `#set <rule>` / `#add <absolute dir>`"
			}, {
				for: "a plan's own rules"
				use: "/fdesign (the design run freezes `permissions.json`)"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "mine"
				default: "—"
				effect:  "the only route: read the unsynthesized records and the ledger, write allow-rule tasks — [Resolve records](#skill-sminepermissions-mine-001)"
				form:    "route"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: no review stop, no question — [Nightly continues](#skill-sminepermissions-mine-000)"
				form:    "flag"
			}, {
				name:    "--cap"
				default: "`5`"
				effect:  "max task files this run, strongest first; drops listed in the report — [Cap and write](#skill-sminepermissions-mine-006)"
				form:    "`<n>`"
			}]
		}
	}]
}, {
	title: "Inputs"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-INPUT-001"
			role:      "def"
			statement: "Record input: the records `make sessions ARGS=\"unsynthesized\"` lists — the current batch run's records included — read as JSON, their `permissions` blocks; never a transcript."
			title:     "Record input"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPERMISSIONS-INPUT-002"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "fields `{ts, session_id, tool, input, candidate_rule, cause}` plus agent stamps; only lines whose `session_id` is a read record count."
			}]
			role:      "def"
			statement: "Denial ledger: `~/.claude/tool-policy/denials.jsonl`, one gate denial per line."
			title:     "Denial ledger"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPERMISSIONS-TASK-001"
			items: [{
				fence: {
					body: """
						{"id": "permissions/<slug>", "kind": "permissions", "title": "", "problem": "", "evidence": [], "fix": "", "sessions": [], "target": "settings/claude_code/components/gate.json", "change": "", "notes": ""}

						"""
					info:   "json"
					marker: "```"
				}
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`id` = `permissions/<slug>`; `kind` = `permissions`; `target` = `settings/claude_code/components/gate.json`; `sessions` the denying sessions."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`problem` = one plain sentence ≤ 160 runes stating what is wrong for a reader who was not in the session."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "no ids, shas, paths, line numbers, code spans or parenthesised counts — `$SKILL_SMINEPERMISSIONS_TASK_CHECK task-check` refuses them."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`fix` = why the rule is safe, ≤ 240 characters."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`evidence` = `[{session, quote, finding}]`, at most 3, strongest first."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "each quote a verbatim excerpt ≤ 160 characters of the record's finding quote text (`quotes[].text`), `finding` the index of that finding in the record."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "A longer quote is cut at a clause boundary with a trailing `…`, never paraphrased."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`change` = \"Add `<rule>` to `permissions.allow`\"."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`notes` = the exact rule string, the ledger targets it admits, validate with `jq` that the settings file parses."
			}]
			role:      "def"
			statement: "Change task: one JSON file `.smine-run/changes/permissions-<slug>.json` in cwd, keyed as the block below; the Write tool creates the dir."
			title:     "Change task"
		}
	}]
	kind: "context"
}, {
	title: "Rule knowledge"
	blocks: [{
		entry: {
			id: "SKILL-SMINEPERMISSIONS-RULE-001a"
			items: [{
				fence: {
					body: """
						Dialect per tool kind:
						- Bash: Bash(<program> <subcommand> *) — never a bare Bash(*), never Bash(<program> *) when only one
						  subcommand was ever denied.
						- MCP: the exact mcp__server__tool string; a whole-server mcp__server only when every tool of that
						  server was denied across the evidence.
						- Web: WebFetch(domain:<host>); Skill: Skill(<name>).

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "The task's rule is the ledger's `candidate_rule` for the denied target — the narrowest dialect-conformant allow rule, in the repo's space-wildcard dialect."
			title:     "Rule form"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-RULES-001a"
			role:      "rule"
			statement: "A task MUST add an allow rule only — never a `deny` or `ask` change, never a deny-list edit, never broader than the gate's candidate."
			title:     "Allow only"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-RULES-001b"
			role:      "rule"
			statement: "The rule string MUST NOT be a bare `Bash(*)`, nor a whole-server `mcp__server` unless every tool of it was denied."
			title:     "No blanket rules"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-RULE-003"
			role:      "rule"
			statement: "A task MUST NOT allow a compound command or a wrapper (`bash <script>`, `sh -c`, `env …`, `xargs …`) — the closed world denies those by design."
			title:     "No compound rule"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-RULE-004"
			role:      "rule"
			statement: "A candidate MUST rest on a ledger `candidate_rule` denied in ≥2 distinct sessions; a denial without a candidate rule is never a task."
			title:     "Gate candidates only"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPERMISSIONS-SETUP-005c"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "absent profile means English, unchanged."
			}]
			role:      "rule"
			statement: "The task MUST NOT translate ids, targets, rule strings, file paths, dates, tags, schema keys, or status values."
			title:     "Language field exclusions"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPERMISSIONS-RULES-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Rule strings MUST be copied character-exact from the ledger's `candidate_rule` (space wildcards, never colon forms)."
			title:     "Exact rule strings"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPERMISSIONS-SETUP-006"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "When the profile's `audience:` is `casual`, `title`, `problem` and `fix` MUST carry no file paths and no schema/taxonomy jargon — say what becomes frictionless; the rule string stays in `change`."
			title:     "Casual presentation"
		}
	}]
	kind: "context"
}, {
	title: "Mine"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-000"
			role:      "rule"
			statement: "`--nightly` MUST finish without a stop or a question."
			title:     "Nightly continues"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-008"
			role:      "rule"
			statement: "This skill MUST NOT edit a settings file, a repo file or a store; its only writes are task files under `.smine-run/changes/`."
			title:     "Tasks only"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-001"
			role:      "step"
			statement: "RUN `make sessions ARGS=\"unsynthesized\"`; READ every listed record JSON and the ledger lines of those sessions."
			title:     "Resolve records"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-002"
			role:      "stop"
			statement: "If the listing is empty ⇒ STOP: report \"no unsynthesized records\", write nothing."
			title:     "Nothing to mine"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-003"
			role:      "step"
			statement: "GROUP denials by `candidate_rule`, counting distinct sessions; add the records' `permissions.denied` pairs as evidence of the same rule."
			title:     "Group denials"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPERMISSIONS-MINE-004"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "intended: a compound, a forbidden git verb, a path-fence or outside-root `cause`, a deny-rule hit, a wrapper."
			}]
			role:      "step"
			statement: "DROP a denial the doctrine states is intended as `intended-cost` — see [mining drops intended cost]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md) and [tool permissions]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md)."
			title:     "Intended cost"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-ARCHIVE-002a"
			role:      "verify"
			statement: "VERIFY the rule matches no `allow` or `deny` entry of `settings/claude_code/components/gate.json`; on a match, drop it `present`."
			title:     "Not already present"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-006"
			role:      "step"
			statement: "RANK survivors by distinct sessions and denial count; WRITE at most `--cap` [change tasks](#skill-sminepermissions-task-001); LIST every drop as `dropped → <reason>`."
			title:     "Cap and write"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-009"
			role:      "verify"
			statement: "After writing: VERIFY `$SKILL_SMINEPERMISSIONS_TASK_CHECK task-check .smine-run/changes/permissions-*.json` exits 0; repair a violation in the same run by shortening text, never by cutting a sentence."
			title:     "Validate tasks"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-010"
			role:      "step"
			statement: "REPORT: tasks written (id, rule), drops with reasons, denials without a candidate rule."
			title:     "Stage report"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPERMISSIONS-MINE-011"
			role:      "stop"
			statement: "If mode is interactive ⇒ STOP: hand the report over for review."
			title:     "Interactive stop"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "group ledger denials by candidate rule and drop intended ones against the doctrine — bounded, mechanical"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "mine --cap 1"
}
