changelog: [{
	date:    "2026-10-21"
	version: "6.2.2"
	text:    "The fixtures contract points at docs/acdsl.html."
	tokens:  3797
}, {
	date:    "2026-10-17"
	version: "6.2.1"
	text:    "Model file is <leaf>.cue"
	tokens:  3796
}, {
	date:    "2026-10-14"
	version: "6.2.0"
	text:    "Interfaces: one variable per capability — TASK_CHECK"
	tokens:  3795
}, {
	date:    "2026-10-13"
	version: "6.1.0"
	text:    "The task check is the env variable $SKILL_SMINEACDSL_RULES (required) in the write verify and the caps item."
	tokens:  3785
}, {
	date:    "2026-10-09"
	version: "6.0.0"
	text:    "Task problem is one plain sentence ≤ 160 runes gated by task-check; evidence carries the record finding index."
	tokens:  3757
}, {
	date:    "2026-10-08"
	version: "5.3.0"
	text:    "Task text caps: problem ≤ 320, fix ≤ 240, quotes ≤ 160 as verbatim excerpts; smine-rules task-check replaces the jq validate"
	tokens:  3672
}, {
	date:    "2026-10-08"
	version: "5.2.0"
	text:    "Task carries problem, evidence and fix instead of finding."
	tokens:  3573
}, {
	date:    "2026-10-04"
	version: "5.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  3483
}, {
	date:    "2026-10-04"
	version: "5.0.1"
	text:    "Session records are read as the JSON files make sessions lists; the markdown record is gone."
	tokens:  3483
}, {
	date:    "2026-10-04"
	version: "5.0.0"
	text:    "Miner, not planner: the plan route is gone; route mine reads the unsynthesized records for prose rules violated despite being present and repeated review findings, and writes ACDSL rule change tasks whose notes carry rule line, verifier, fixtures and validation commands; capped by --cap."
	tokens:  3484
}, {
	date:    "2026-09-28"
	version: "4.0.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1788
}, {
	date:    "2026-09-26"
	version: "4.0.2"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "4.0.1"
	text:    "Make-only entry points: allowed-tools and prose call make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "4.0.0"
	text:    "Planner contract: mining from batches retired; plan route returns one artifact plan for a problem direction and writes nothing; per-kind knowledge kept."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kind lines (0/2/3/4/5/Model-Rules as phase, 1/Rules as context), pred/post clauses on 5 rules and 10 flow entries, new atoms.json (17 bool, 1 int, 1 enum), 3 queries (band-j-forbidden, eviction-bar-satisfiable, finish-reachable), Applies bullets renamed to param register, caveman-compressed."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-08"
	version: "2.1.0"
	text:    "Caveman trim of over-cap entry statements (wording only, cap now 320 display runes)"
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "format-v2 reformat — When to use replaced with Routing (Purpose/Disambiguation/Preconditions/Pipeline), allowed-tools normalized, two over-length entry statements compressed; content identical"
}, {
	date:    "2026-09-05"
	version: "1.1"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.0"
	text:    "initial version — dedicated gate-proposal dimension: mines batch reports for machine-checkable recurring violations into proposals/acdsl.json (kind acdsl), route-classified per the creation ladder (tool config, existing verifier, generate-with-justification), mandatory violation+clean snippets, verdict-join bar on evictions"
}]
env: {
	TASK_CHECK: {
		interface: "task-check@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-acdsl"
}, {
	key:   "description"
	value: " Mine prose rules sessions keep violating into ACDSL gate change tasks — rule line, verifier, fixtures. Trigger on /smine-acdsl mine or the smine acdsl stage. Args — mine: read the unsynthesized records, write tasks; --nightly: unattended; --cap: max tasks this run (default 5)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 6.2.2"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-IMPL-ARCH-004, RULE-*"
}, {
	key:   "allowed-tools"
	value: " Read, Write, Bash(jq *), Bash(make sessions *)"
}, {
	key:   "argument-hint"
	value: " \"mine --nightly --cap 3\""
}]
preamble: """
	# smine: acdsl miner

	Reads the unsynthesized session records for prose rules violated again and again, turns each machine-checkable one into
	an ACDSL gate change task — rule line, verifier, fixtures — under `.smine-run/changes/`; never edits the gate surface.
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
				note: "acdsl stage of the nightly; the implement stage turns each task into one commit"
			}
			preconditions: ["at least one session record without `synthesized`", "`acdsl/registry.json` present (a gate surface exists); absent: write nothing, report \"no acdsl surface on this install\""]
			purpose: "the acdsl miner — turns repeated violations of prose rules into gate change tasks; never edits the gate surface."
			routes: [{
				for: "mining transcripts into session records"
				use: "/smine-batch"
			}, {
				for: "a judgment rule no verifier can prove"
				use: "/smine-context mine"
			}, {
				for: "authoring a context entry or gate directly"
				use: "/context-create"
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
				effect:  "the only route: read the unsynthesized records, write ACDSL rule tasks — [Resolve records](#skill-smineacdsl-mine-001)"
				form:    "route"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: no review stop, no question — [Nightly continues](#skill-smineacdsl-mine-000)"
				form:    "flag"
			}, {
				name:    "--cap"
				default: "`5`"
				effect:  "max task files this run, strongest first; drops listed in the report — [Cap and write](#skill-smineacdsl-mine-006)"
				form:    "`<n>`"
			}]
		}
	}]
}, {
	title: "Inputs"
	blocks: [{
		entry: {
			id: "SKILL-SMINEACDSL-INPUT-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "signals — `contextUse` kind `violated-despite-present` with its `ruleId`; the same review finding repeated across review sessions."
			}]
			role:      "def"
			statement: "Record input: the records `make sessions ARGS=\"unsynthesized\"` lists — the current batch run's records included — read as JSON; never a transcript."
			title:     "Record input"
		}
	}, {
		entry: {
			id: "SKILL-SMINEACDSL-INPUT-002"
			items: [{
				fence: {
					body: """
						acdsl/rules.acdsl           rule lines (+ -local sibling)
						acdsl/registry.json         registered verifiers
						docs/acdsl.html             bands, generation bounds, fixtures contract
						acdsl/evalgen.json          generation bounds
						acdsl/testdata/<id>/        pass|fail fixtures per rule
						skills/context/context-create/context-create.cue   gate route: existing verifier or generated one

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "Gate surface: the files below, read before any task is composed."
			title:     "Gate surface"
		}
	}, {
		entry: {
			id: "SKILL-SMINEACDSL-TASK-001"
			items: [{
				fence: {
					body: """
						{"id": "acdsl/<slug>", "kind": "acdsl", "title": "", "problem": "", "evidence": [], "fix": "", "sessions": [], "target": "acdsl/rules.acdsl", "change": "", "notes": ""}

						"""
					info:   "json"
					marker: "```"
				}
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`id` = `acdsl/<slug>`; `kind` = `acdsl`; `target` = `acdsl/rules.acdsl` or the tool config; `sessions` the violating sessions."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`problem` = one plain sentence ≤ 160 runes stating what is wrong for a reader who was not in the session."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "no ids, shas, paths, line numbers, code spans or parenthesised counts — `$SKILL_SMINEACDSL_TASK_CHECK task-check` refuses them."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`fix` = why a gate stops it, ≤ 240 characters."
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
				text:   "`change` one imperative sentence."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`notes` = [Task notes](#skill-smineacdsl-task-002)."
			}]
			role:      "def"
			statement: "Change task: one JSON file `.smine-run/changes/acdsl-<slug>.json` in cwd, keyed as the block below; the Write tool creates the dir."
			title:     "Change task"
		}
	}, {
		entry: {
			id: "SKILL-SMINEACDSL-TASK-002"
			items: [{
				fence: {
					body: """
						rule line     the //acdsl:<ID> line to append, anchor regex, band, why=
						verifier      registry name (existing) or the new verifier's name + routeJustification
						fixtures      acdsl/testdata/<ID>/pass and /fail from the records' violating and clean snippets
						entry         the context entry whose channel becomes [gate] with a Gate: <ID> bullet
						validate      make acdsl ARGS="fixtures"; make acdsl ARGS="check -rule <ID>" — every hit a real violation

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "Task notes carry the implementer's mechanics, one line each:"
			title:     "Task notes"
		}
	}]
	kind: "context"
}, {
	title: "Ladder"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEACDSL-ROUTE-004a"
			role:      "rule"
			statement: "Band MUST be F/A/D per taxonomy; J never a gate."
			title:     "Band, J excluded"
		}
	}, {
		entry: {
			id: "SKILL-SMINEACDSL-ROUTE-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`.golangci.yml` linter/setting, shellcheck directive, or JSON-schema tightening."
			}]
			role:      "step"
			statement: "CLASSIFY as route `1-tool-config` when established tool repo gates already covers it: name tool, config delta in `change`."
			title:     "Route: tool config"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-ROUTE-001b"
			role:      "step"
			statement: "NAME existing wrapper (e.g. `golangci`, `shellcheck`, `jsonschema`) as the verifier in `notes`."
			title:     "Tool config verifier"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-ROUTE-002"
			role:      "step"
			statement: "CLASSIFY as route `2-existing-verifier` when a registered verifier fits with new params/anchors: `notes` names the registry entry, the anchor path regex and the line to append."
			title:     "Route: existing verifier"
		}
	}, {
		entry: {
			id: "SKILL-SMINEACDSL-ROUTE-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the implementer follows the context-create generation contract (evalgen bounds); a task without routeJustification is never written."
			}]
			role:      "step"
			statement: "CLASSIFY as route `3-generate` when neither route fits: sketch the verifier, name in `notes` the routeJustification why routes 1, 2 fail; band F/A only."
			title:     "Route: generate"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-ROUTE-004b"
			role:      "verify"
			statement: "VERIFY candidate's band not J; if J, write no task — list it `rerouted to smine-context (J)` in the report."
			title:     "Reroute J-band"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-ROUTE-005"
			role:      "step"
			statement: "CHECK the [gate surface](#skill-smineacdsl-input-002) per candidate: an already-gated rule is a drop `covered`; a near-miss becomes an amendment of the existing rule id."
			title:     "Check gate surface"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-ARCHIVE-004a"
			role:      "stop"
			statement: "If an ACDSL-rule eviction (`set projected=\"false\" on <ID>`) lacks green verifier+fixtures or <300 clean runs (`make acdsl ARGS=\"verdicts -since 720h\"`) ⇒ STOP: write no eviction task."
			title:     "Eviction verdict bar"
		}
	}]
	kind: "context"
}, {
	title: "Presentation"
	blocks: [{
		entry: {
			id: "SKILL-SMINEACDSL-SETUP-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "profile path `~/.claude/context/global/presentation-profile.md`; a casual audience gets no ids/paths/jargon in prose."
			}]
			role:      "step"
			statement: "FOLLOW the presentation profile when writing `title`, `problem` and `fix`; keep ids, paths, code, dates, tags and schema keys untranslated."
			title:     "Language/casual presentation"
		}
	}]
	kind: "context"
}, {
	title: "Mine"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-000"
			role:      "rule"
			statement: "`--nightly` MUST finish without a stop or a question."
			title:     "Nightly continues"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-007"
			role:      "rule"
			statement: "A candidate MUST rest on ≥2 sessions violating the same rule and on a violating plus a clean snippet for the fixtures; weaker evidence is a drop `below-threshold`."
			title:     "Evidence threshold"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-008"
			role:      "rule"
			statement: "This skill MUST NOT edit the gate surface, a context doc or a store; its only writes are task files under `.smine-run/changes/`."
			title:     "Tasks only"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-001"
			role:      "step"
			statement: "RUN `make sessions ARGS=\"unsynthesized\"`; READ every listed record JSON and the [gate surface](#skill-smineacdsl-input-002)."
			title:     "Resolve records"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-002"
			role:      "stop"
			statement: "If the listing is empty ⇒ STOP: report \"no unsynthesized records\", write nothing."
			title:     "Nothing to mine"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-003"
			role:      "step"
			statement: "COLLECT violated-despite-present signals and repeated review findings; GROUP them per rule id, counting distinct sessions."
			title:     "Collect candidates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-004"
			role:      "step"
			statement: "DROP a candidate whose cost is the stated purpose of a mechanism as `intended-cost` — see [mining drops intended cost]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md)."
			title:     "Intended cost"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-005"
			role:      "step"
			statement: "ROUTE each survivor through the ladder above; compose its task with [Task notes](#skill-smineacdsl-task-002) filled."
			title:     "Route survivors"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-006"
			role:      "step"
			statement: "RANK survivors by distinct sessions and cost; WRITE at most `--cap` [change tasks](#skill-smineacdsl-task-001); LIST every drop as `dropped → <reason>`."
			title:     "Cap and write"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-009"
			role:      "verify"
			statement: "After writing: VERIFY `$SKILL_SMINEACDSL_TASK_CHECK task-check .smine-run/changes/acdsl-*.json` exits 0; a violation is repaired in the same run by shortening the text, never by cutting a sentence."
			title:     "Validate tasks"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-010"
			role:      "step"
			statement: "REPORT: tasks written (id, route, verifier), drops with reasons, J-band reroutes."
			title:     "Stage report"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEACDSL-MINE-011"
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
			effort: "medium"
			reason: "judge which repeated violations a verifier can prove and route them through the creation ladder"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "mine --cap 1"
}
