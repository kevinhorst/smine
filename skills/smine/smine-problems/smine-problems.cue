changelog: [{
	date:    "2026-10-17"
	version: "3.3.1"
	text:    "Model file is <leaf>.cue: the task template's target names it."
	tokens:  5920
}, {
	date:    "2026-10-14"
	version: "3.3.0"
	text:    "Interfaces: one variable per capability — TASK_CHECK"
	tokens:  5915
}, {
	date:    "2026-10-13"
	version: "3.2.0"
	text:    "The task check is the env variable $SKILL_SMINEPROBLEMS_RULES (required) in the write verify and the caps item."
	tokens:  5905
}, {
	date:    "2026-10-12"
	version: "3.1.0"
	text:    "Rejected backlog rows carry the user's reason and, for a proposal rejected on the Proposals page, its commit; a rejected drop names the reason."
	tokens:  5871
}, {
	date:    "2026-10-10"
	version: "3.0.1"
	text:    "Icebox recipe states the 120-rune title cap the store enforces; evidence names each note's finding as <session id>#<finding>; mined.by records the mining session."
	tokens:  5811
}, {
	date:    "2026-10-09"
	version: "3.0.0"
	text:    "Task problem is one plain sentence ≤ 160 runes gated by task-check; evidence carries the record finding index; backlog id is its own field."
	tokens:  5753
}, {
	date:    "2026-10-09"
	version: "2.5.0"
	text:    "Runtime placement declared: root-only (Workflow stages)."
	tokens:  5658
}, {
	date:    "2026-10-08"
	version: "2.4.0"
	text:    "Task text caps; smine-rules task-check validates every written task, the jq check keeps the backlog"
	tokens:  5658
}, {
	date:    "2026-10-08"
	version: "2.3.0"
	text:    "Task carries problem, evidence and fix instead of finding."
	tokens:  5569
}, {
	date:    "2026-10-06"
	version: "2.2.0"
	text:    "The backlog input carries the archive's rejected rows; a candidate matching a rejected row is dropped as rejected and never becomes a task or an Icebox row."
	tokens:  5450
}, {
	date:    "2026-10-04"
	version: "2.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  5339
}, {
	date:    "2026-10-04"
	version: "2.0.1"
	text:    "Session records are read as the JSON files make sessions lists; the markdown record is gone, and the smoke fixture record is JSON."
	tokens:  5339
}, {
	date:    "2026-10-04"
	version: "2.0.0"
	text:    "Internal synthesis stage without a store: problems.json, the proposal archive, route, blast radius, edge object and artifactPlan are gone; design intent (context index, docs, full backlog items, plan TLDRs) replaces the harness model; a mandatory intended-cost gate drops the stated cost of a mechanism; surviving problems go to the smine-skills and smine-routines planners, whose change tasks land in .smine-run/changes/; a problem no planner solves becomes one Icebox row with a mined marker; a backlog match enriches that item instead of duplicating it."
	tokens:  5349
}, {
	date:    "2026-10-02"
	version: "1.3.2"
	text:    "Store paths hang off `<store>`, the path `make rules ARGS=\"store-root\"` prints — a smoke or eval run works on its own store, never the live one; the smoke seeds its store from the fixture tree."
	tokens:  4662
}, {
	date:    "2026-10-02"
	version: "1.3.1"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  4751
}, {
	date:    "2026-10-02"
	version: "1.3.0"
	text:    "Problems store, archive and backlog read from the plans archive `~/GolandProjects/agent-plans/<repo-name>/`; Read and Edit grants there."
	tokens:  4749
}, {
	date:    "2026-10-01"
	version: "1.2.5"
	text:    "Pipeline declares from /smine — the dispatcher that invokes this stage."
	tokens:  4510
}, {
	date:    "2026-09-29"
	version: "1.2.4"
	text:    "workflow spawns now name a model through the generated GEN:models vocabulary; absent model ⇒ default subagent model"
	tokens:  4506
}, {
	date:    "2026-09-29"
	version: "1.2.3"
	text:    "Plan-file examples and lookup use design/<slug>.md (fdesign 5.8.0)."
}, {
	date:    "2026-09-28"
	version: "1.2.2"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-27"
	version: "1.2.1"
	text:    "Declared Delegation: never — Workflow fan-out, root session only."
}, {
	date:    "2026-09-27"
	version: "1.2.0"
	text:    "Input is the unsynthesized session records listed by make sessions; mark --synthesized replaces the analyzed-problems ledger; arcs replaced by cross-session patterns."
}, {
	date:    "2026-09-26"
	version: "1.1.1"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-26"
	version: "1.1.0"
	text:    "Planner fan-out runs as Workflow plan.js with named smine-planner agents."
}, {
	date:    "2026-09-25"
	version: "1.0.1"
	text:    "Make-only entry points: allowed-tools grants make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "1.0.0"
	text:    "Problem-first synthesis stage: batch JSON and arcs across scopes into proposals/problems.json with measured cost, edge gate, blast radius, route, directions and planner-returned artifact plans."
}]
enableDelegation: "never"
env: {
	TASK_CHECK: {
		interface: "task-check@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-problems"
}, {
	key:   "description"
	value: " Synthesize problems from unsynthesized session records into skill and routine change tasks, or Icebox rows for problems no planner solves. Trigger on /smine-problems or \"synthesize problems from sessions\". Args — --nightly: unattended, every unsynthesized record; --cap: max outputs this run (default 5)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.3.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-*, FACT-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"--nightly --cap 5\""
}, {
	key:   "allowed-tools"
	value: " Edit, Edit(~/GolandProjects/agent-plans/**), Read, Read(~/GolandProjects/agent-plans/**), Skill(smine-routines), Skill(smine-skills), ToolSearch, Workflow, Write, Bash(cat *), Bash(git log *), Bash(grep *), Bash(head *), Bash(jq *), Bash(ls *), Bash(wc *)"
}]
preamble: """
	# smine: problem synthesis

	The internal synthesis stage: reads every unsynthesized session record against the design intent — context index,
	docs, backlog, WIP plans — drops costs that are a mechanism's purpose, hands each surviving problem to the skill and
	routine planners and writes their change tasks to `.smine-run/changes/`; a problem no planner solves becomes one
	backlog Icebox row. Writes no problem store.
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
				}, {
					kind:  "from"
					skill: "smine-batch"
				}, {
					kind:     "from"
					skill:    "smine-style"
					optional: true
				}, {
					kind:  "to"
					skill: "smine-consolidate"
				}, {
					kind:  "spawns"
					skill: "smine-routines"
				}, {
					kind:  "spawns"
					skill: "smine-skills"
				}]
				note: "planners spawned per solution direction; their change tasks feed the implement stage"
			}
			preconditions: ["at least one session record without `synthesized`", "`<store>/backlog/backlog.json` present — `<store>` the path `make rules ARGS=\"store-root\"` prints"]
			purpose: "the internal synthesis stage — turns mined session records into skill and routine change tasks and Icebox rows; shown nowhere, writes no problem store."
			routes: [{
				for: "mining transcripts into session records"
				use: "/smine-batch"
			}, {
				for: "mine then synthesize in one go"
				use: "/smine"
			}, {
				for: "context, permission or gate changes"
				use: "/smine-context, /smine-permissions, /smine-acdsl (`mine`)"
			}, {
				for: "one change task for one direction"
				use: "the smine-skills or smine-routines `plan` route"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: every unsynthesized record, no review stop — [Nightly continues](#skill-smineproblems-finish-003)"
				form:    "flag"
			}, {
				name:    "--cap"
				default: "`5`"
				effect:  "max outputs (task files plus new Icebox rows) this run; drops listed in run notes — [Cap](#skill-smineproblems-write-004)"
				form:    "`<n>`"
			}]
		}
	}]
}, {
	title: "Inputs"
	blocks: [{
		entry: {
			id: "SKILL-SMINEPROBLEMS-INPUT-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "read — `cost`, `repo`, `week`; `findings`; `frustration`, `positive`, `permissions`."
			}]
			role:      "def"
			statement: "Session input: the record JSONs `make sessions ARGS=\"unsynthesized\"` lists; never a transcript."
			title:     "Record input"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-INPUT-003"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "context index `context/context.json` — facts and actions entries: what a mechanism is for; `docs/` — the explanations they point to."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "backlog `<store>/backlog/backlog.json` — every item in full (title, description): the user's known problems; `archive.rejected` rows: refused ones."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "rejected row `reason` = why the user refused it; a row with `commit` is a change commit rejected on the Proposals page."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "TLDR = the `## TLDR` section of `plans/*/*-wip-*/concept/concept.md` and the plan file `design/<slug>.md`; 30 days of `git log --format=%s`."
			}]
			role:      "def"
			statement: "Design intent: the context index, `docs/`, the backlog items, every WIP plan's TLDR and recent commit subjects; never a full plan body."
			title:     "Design intent"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-INPUT-004"
			role:      "def"
			statement: "`~/.claude/tool-policy/denials.jsonl` — one `{ts, session_id, tool, input, candidate_rule, cause}` per line; cost evidence only, never a problem on its own."
			title:     "Denial ledger"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-INPUT-005"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "style profile at `$HOME/.claude/context/global/style-profile.md`, injected by the global-context hook."
			}]
			role:      "def"
			statement: "Design taste: the `## Design taste` block of the style profile; absent = skip its check, note it in run notes."
			title:     "Design taste"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-INPUT-007"
			role:      "def"
			statement: "Total record input over 400000 characters ⇒ two-level synthesis: per-scope candidate lists first, then one merge pass over the scope outputs."
			title:     "Context budget"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-INPUT-008"
			role:      "def"
			statement: "`<store>` = the path `make rules ARGS=\"store-root\"` prints; this stage touches only the backlog `<store>/backlog/backlog.json`."
			title:     "Store root"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-TASK-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "`kind` `skills` or `routines`; `sessions` the evidence session ids, strongest first; `change` one imperative sentence."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`problem` = one plain sentence ≤ 160 runes stating what is wrong for a reader who was not in the session."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "no ids, shas, paths, line numbers, code spans or parenthesised counts — `$SKILL_SMINEPROBLEMS_TASK_CHECK task-check` refuses them."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`backlog` = the matched `B-NNN`, omitted when the problem matched no backlog item."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`evidence` = `[{session, quote, finding}]` from the problem's evidence notes, at most 3."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "each quote a verbatim excerpt ≤ 160 characters, `finding` the note's finding index."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "a longer note cut at a clause boundary with `…`."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`fix` = why the change fixes it, ≤ 240 characters."
			}]
			role:      "def"
			statement: "Change task: one JSON file `.smine-run/changes/<kind>-<slug>.json` in cwd, `id` = `<kind>/<slug>`, keyed as the [Task example](#skill-smineproblems-tpl-001)."
			title:     "Change task"
		}
	}]
	kind: "context"
}, {
	title: "0. Setup"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPROBLEMS-SETUP-001"
			role:      "step"
			statement: "RUN `make sessions ARGS=\"unsynthesized\"`; `--nightly` takes every listed record, interactive the oldest 10."
			title:     "Resolve records"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-SETUP-002"
			role:      "step"
			statement: "READ the [design intent](#skill-smineproblems-input-003) and the presentation profile at `~/.claude/context/global/presentation-profile.md`; the style profile is already in context."
			title:     "Read design intent"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-SETUP-003"
			role:      "stop"
			statement: "If the listing is empty ⇒ STOP: report \"all records synthesized\"."
			title:     "Nothing unsynthesized"
		}
	}]
	kind: "phase"
}, {
	title: "1. Synthesize"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPROBLEMS-SYNTH-001"
			role:      "rule"
			statement: "A candidate MUST be a problem — what hurt, how often, what it burned — NEVER an artifact type; the artifact kinds enter only through the planners."
			title:     "Problem, not artifact"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-SYNTH-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "never grouped by finding dimension."
			}]
			role:      "step"
			statement: "COLLECT candidates across the records — cross-session patterns first, then recurring findings, frustration entries and denial-ledger clusters — grouped by the underlying problem."
			title:     "Collect candidates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-SYNTH-003"
			role:      "step"
			statement: "Over budget per [Context budget](#skill-smineproblems-input-007): WRITE each scope's candidate list to run notes, then MERGE across scopes by meaning; one problem spanning scopes counts each scope."
			title:     "Two-level merge"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-SYNTH-004"
			role:      "step"
			statement: "MATCH every candidate against the backlog items, the archive's rejected rows and WIP plan TLDRs by meaning; a backlog match carries that item's `B-NNN` from here on."
			title:     "Intent match"
		}
	}]
	kind: "phase"
}, {
	title: "2. Gate"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPROBLEMS-GATE-002"
			role:      "rule"
			statement: "A candidate MUST need cross-session vantage or investigation time to survive; one the user fixes inline in the moment is dropped to run notes with `dropped → inline-fixable`."
			title:     "Edge gate"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-GATE-005"
			role:      "rule"
			statement: "No task, row or backlog edit MUST be written for a candidate before it has passed the [intended-cost check](#skill-smineproblems-gate-006)."
			title:     "Gate before writing"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-GATE-006"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "order: context index, then `docs/`, then plan TLDRs, then the backlog — the first stated purpose found decides."
			}]
			role:      "step"
			statement: "DROP a candidate whose cost is the stated purpose of a mechanism (gate, hook, required read, forbidden verb) as `dropped → intended-cost (<mechanism>)` — see [mining drops intended cost]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md)."
			title:     "Intended cost"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-GATE-007"
			role:      "step"
			statement: "DROP a candidate already fixed on main — a commit subject or the current code shows the fix — as `dropped → fixed (<commit or path>)`."
			title:     "Already fixed"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-GATE-008"
			role:      "step"
			statement: "DROP a candidate matching an archived rejected row as `dropped → rejected (<B-NNN>: <reason>)` — the user refused it on the board or the Proposals page."
			title:     "Rejected match"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-GATE-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "a record without cost counts 1 session."
			}]
			role:      "step"
			statement: "SUM per survivor over its evidence records' `cost` objects — sessions, distinct repos, turns, wallSeconds, denials — into the Problem JSON's `cost` line, used for ranking only."
			title:     "Measure cost"
		}
	}]
	kind: "phase"
}, {
	title: "3. Directions and plan"
	blocks: [{
		entry: {
			id: "SKILL-SMINEPROBLEMS-PLAN-005"
			items: [{
				fence: {
					body: """
						{"id": "", "title": "", "problem": "", "cost": "", "backlog": "", "evidence": [{"session": "", "note": "", "finding": 0}],
						 "directions": [{"title": "", "direction": "", "feasibility": "", "tasteFit": ""}]}

						"""
					info:   "json"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "Problem: the JSON a planner receives, keyed as below — `cost` the evidence line, `backlog` the matched `B-NNN` or empty, `finding` the index of the record finding each note rests on."
			title:     "Problem"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-PLAN-002"
			role:      "def"
			statement: "A planner returns `{task, reason}`: `task` one [change task](#skill-smineproblems-task-001) of its kind or `null`; it writes nothing."
			title:     "Planner return"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-PLAN-001"
			role:      "step"
			statement: "WRITE one to three `directions[]` per survivor, each `{title, direction, feasibility, tasteFit}`; `feasibility` names the mechanism it relies on, `tasteFit` the taste directive it follows."
			title:     "Directions"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-PLAN-003"
			items: [{
				label:  "Script"
				marker: "*"
				text:   "`workflows/plan.js`"
			}, {
				label:  "param"
				marker: "*"
				text:   "request = `{candidateId, directionIndex, planner, candidate}`, `candidate` the [Problem](#skill-smineproblems-plan-005) JSON."
			}, {
				label:  "param"
				marker: "*"
				text:   "planners — smine-skills, smine-routines; one request per direction and planner."
			}]
			role:      "spawn"
			statement: "SPAWN the Workflow tool, scriptPath `<base dir>/workflows/plan.js`, args `{requests[]}`; await its notification."
			title:     "Spawn planners"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-PLAN-004"
			role:      "step"
			statement: "KEEP per survivor the first non-null `task` in direction order, smine-skills before smine-routines; every return null ⇒ the survivor is unsolvable here."
			title:     "Pick task"
		}
	}]
	kind: "phase"
}, {
	title: "4. Write"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPROBLEMS-WRITE-001"
			role:      "rule"
			statement: "A task MUST carry every [change task](#skill-smineproblems-task-001) field; `sessions` holds at most 3 evidence ids, strongest first."
			title:     "Task shape"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-WRITE-002"
			role:      "rule"
			statement: "A survivor matching a backlog item MUST NOT become a new row: its task's `backlog` names the `B-NNN`, or, unsolvable, its sessions enrich that row's `mined` marker."
			title:     "Backlog pickup"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-WRITE-004"
			role:      "step"
			statement: "RANK survivors by cost, breadth and backlog match; WRITE at most `--cap` outputs; LIST every drop in run notes as `dropped → cap (<rank reason>)`."
			title:     "Cap"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-WRITE-003"
			role:      "step"
			statement: "WRITE each kept task to `.smine-run/changes/<kind>-<slug>.json`; the Write tool creates the dir."
			title:     "Write tasks"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-WRITE-007"
			role:      "step"
			statement: "APPEND each unsolvable survivor without a backlog match as ONE Icebox row per [Icebox row recipe](#skill-smineproblems-tpl-002); a matched row gets the recipe's marker step only."
			title:     "Icebox row"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-WRITE-006"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "a violation is repaired by shortening, never by cutting a sentence."
			}]
			role:      "verify"
			statement: "After writing: VERIFY `$SKILL_SMINEPROBLEMS_TASK_CHECK task-check .smine-run/changes/*.json` exits 0 and the backlog with `jq`: item keys per `skills/concept/backlog/schema.json`."
			title:     "Validate"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-WRITE-005"
			role:      "step"
			statement: "After validation: RUN `make sessions ARGS=\"mark --synthesized <today> <every id read>\"`; a non-ok line is a run note, never a retry."
			title:     "Mark synthesized"
		}
	}]
	kind: "phase"
}, {
	title: "5. Finish"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEPROBLEMS-FINISH-003"
			role:      "rule"
			statement: "`--nightly` MUST finish without a stop or a question."
			title:     "Nightly continues"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-FINISH-001"
			role:      "step"
			statement: "WRITE run notes to the return: tasks written, Icebox rows added or enriched, drops with reasons, scope candidate lists when two-level ran, missing inputs."
			title:     "Run notes"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEPROBLEMS-FINISH-002"
			role:      "stop"
			statement: "If mode is interactive ⇒ STOP: report for review; nothing is committed."
			title:     "Interactive stop"
		}
	}]
	kind: "phase"
}, {
	title: "Payloads"
	blocks: [{
		entry: {
			id: "SKILL-SMINEPROBLEMS-TPL-001"
			payload: {
				info:   "json"
				marker: "```"
			}
			role:  "payload"
			title: "Task example"
		}
	}, {
		entry: {
			id: "SKILL-SMINEPROBLEMS-TPL-002"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Icebox row recipe"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "cross-scope judgment over compressed evidence against the design intent; the one stage where effort buys quality"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	fixture: {
		tree: "tree"
	}
	invoke: "--nightly --cap 1"
}
