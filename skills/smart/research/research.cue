changelog: [{
	date:    "2026-10-17"
	version: "1.3.1"
	text:    "Model file is <leaf>.cue"
	tokens:  2648
}, {
	date:    "2026-10-10"
	version: "1.3.0"
	text:    "Confidence label derived from the count of cited primaries read in this run — one read source caps at medium; a verify before return drops every citation never read or fetched and recomputes the label."
}, {
	date:    "2026-10-01"
	version: "1.2.0"
	text:    "allowed-tools grants Skill(research) — the topic re-invocation is a declared grant, no longer a global one; pipeline adds spawnedBy /parallelize."
	tokens:  2550
}, {
	date:    "2026-09-28"
	version: "1.1.4"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2560
}, {
	date:    "2026-09-28"
	version: "1.1.3"
	text:    "Agent spawns name an explicit model — the gate denies inherited or Fable models; an absent bracket model means sonnet."
}, {
	date:    "2026-09-27"
	version: "1.1.2"
	text:    "Declared Delegation: degraded — Agent-tool fan-out, degrades below root."
}, {
	date:    "2026-09-27"
	version: "1.1.1"
	text:    "Bracket and arm tokens use <model>:<effort>"
}, {
	date:    "2026-09-24"
	version: "1.1.0"
	text:    "subagents: n[model-effort] arg — opt-in reader fan-out (spawn, two substeps, merge step, reader brief); single agent stays the default"
}, {
	date:    "2026-09-24"
	version: "1.0.0"
	text:    "Initial smartskill: single-agent primary-source research with citations and confidence, contested ⇒ /investigate, default slice + topic widening"
}]
frontmatter: [{
	key:   "name"
	value: " research"
}, {
	key:   "description"
	value: " Answer one question from primary sources — web and repo — with citations; single agent unless readers are requested. Trigger on /research or \"research this\" or \"look this up\" or \"what does the doc actually say\". Args — question: the question; sources: URLs or paths to read first; topic: context slice(s) to load; subagents: n[model:effort] reader fan-out, absent = single agent."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.3.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"question: does claude -p expose the Workflow tool on 2.1.271 sources: https://code.claude.com/docs/en/workflows topic:harness subagents: 2[sonnet:medium]\""
}, {
	key:   "allowed-tools"
	value: " Agent, Glob, Grep, Read, Skill(research), WebFetch, WebSearch"
}, {
	key:   "acdsl-context"
	value: " ACTION-*, FACT-*, RULE-*"
}]
preamble: """
	# Research

	One question, primary sources read in full, answer with citations and a stated confidence — the cheap, single-pass
	sibling of /investigate; one agent by default, n readers on request.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "escalates"
					skill: "investigate"
				}]
				standalone: true
			}
			preconditions: ["one question stated", "`WebFetch`/`WebSearch` reachable, or `sources:` naming local files"]
			purpose: "ground an answer in documents rather than recall — docs, changelogs, issues, repo files."
			routes: [{
				for: "contested question needing N independent runs and adversarial merge"
				use: "/investigate"
			}, {
				for: "a verified root-cause diagnosis of a defect"
				use: "/diagnose-debug"
			}, {
				for: "a quick answer without sources"
				use: "/qq"
			}, {
				for: "the harness-bundled multi-agent deep research"
				use: "never — token cost; use this skill or /investigate"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "question"
				default: "required"
				effect:  "the question answered — [Question](#skill-research-intake-001)"
				form:    "`question: <text>` or bare text"
			}, {
				name:    "sources"
				default: "none"
				effect:  "read first, before any search — [Sources](#skill-research-intake-002)"
				form:    "`sources: <url or path>, …`"
			}, {
				name:    "topic"
				default: "default slice"
				effect:  "context slice(s) injected; `--no-context` skips all — [Topic use](#skill-research-context-001)"
				form:    "`topic:<name>`, repeatable"
			}, {
				name:    "subagents"
				default: "absent, single agent"
				effect:  "n readers on the bracket model/effort; sources split among them — [Readers](#skill-research-intake-003)"
				form:    "`subagents: <n>[<model>:<effort>]`"
			}]
		}
	}]
}, {
	title: "Intake"
	blocks: [{
		entry: {
			id:        "SKILL-RESEARCH-INTAKE-001"
			role:      "def"
			statement: "One closed or bounded question; an open \"tell me about X\" narrows to the decision it serves, stated in the answer."
			title:     "Question"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-INTAKE-002"
			role:      "def"
			statement: "User-named URLs or paths, read in full before any search; they outrank search hits."
			title:     "Sources"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-INTAKE-003"
			role:      "def"
			statement: "`subagents: <n>[<model>:<effort>]` — n reader agents, each on the bracket model and effort (`2[sonnet:medium]`); absent bracket model ⇒ `sonnet`. Absent arg ⇒ no readers, the agent reads alone."
			title:     "Readers"
		}
	}]
	kind: "context"
}, {
	title: "Context"
	blocks: [{
		entry: {
			id:        "SKILL-RESEARCH-CONTEXT-001"
			role:      "def"
			statement: "Topic names and their slices per the smartskill rule in `context/rules/skills.md`; the default slice carries that rule and the entry ontology."
			title:     "Topic use"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-CONTEXT-002"
			role:      "rule"
			statement: "Every load-bearing claim MUST cite a primary source read in this run — URL or path:line — never recall, never a search snippet alone."
			title:     "Primary sources"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-CONTEXT-003"
			role:      "rule"
			statement: "Agent MUST NOT spawn agents or workflows unless `subagents` is given; then exactly n readers, never a nested fan-out — readers read and cite, the agent alone answers."
			title:     "Requested readers"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-CONTEXT-004"
			role:      "rule"
			statement: "Agent MUST NOT write files or edit code; the answer is the deliverable."
			title:     "Read-only"
		}
	}, {
		entry: {
			id: "SKILL-RESEARCH-CONTEXT-005"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Agent MUST stop reading once two independent primary sources agree on the answer; disagreement adds sources, agreement does not."
			title:     "Source budget"
		}
	}]
	kind: "context"
}, {
	title: "Run"
	blocks: [{
		entry: {
			id:        "SKILL-RESEARCH-RUN-001"
			role:      "step"
			statement: "INVOKE the Skill tool on `research` with `topic:<name>` plus the question when the question hits a topic not yet loaded."
			title:     "Widen"
		}
	}, {
		entry: {
			id: "SKILL-RESEARCH-RUN-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "with `subagents`, the named sources are split evenly among the readers instead — see [Reader spawn](#skill-research-run-006)."
			}]
			role:      "step"
			statement: "READ every user-named source in full; RECORD per source the claims it settles."
			title:     "Read sources"
		}
	}, {
		entry: {
			id: "SKILL-RESEARCH-RUN-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "with `subagents`, each reader searches its own share of the gaps; the agent merges their returns."
			}]
			role:      "step"
			statement: "SEARCH for the gaps the named sources leave; FETCH the primary page behind each hit, never the snippet; prefer official docs, changelogs, issues, source code."
			title:     "Search"
		}
	}, {
		entry: {
			id: "SKILL-RESEARCH-RUN-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "model and effort from the bracket; absent bracket model ⇒ `sonnet`, `model` never omitted; `subagent_type` general-purpose; readers never spawn."
			}]
			role:      "spawn"
			statement: "SPAWN n readers in one message via the Agent tool, each with the question, its source share, its gap share and the [Reader brief](#skill-research-tpl-001)."
			title:     "Reader spawn"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-RUN-007"
			role:      "substep"
			statement: "READ every assigned source in full, SEARCH the assigned gaps, FETCH primary pages behind hits."
			title:     "Reader read"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-RUN-008"
			role:      "substep"
			statement: "RETURN JSON lines `{claim, locator, date_or_version, settles}` — one per settled claim, no prose report."
			title:     "Reader return"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-RUN-009"
			role:      "step"
			statement: "MERGE reader returns into one claim list; RE-READ the locator of every load-bearing claim before it enters the answer."
			title:     "Merge returns"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-RUN-004"
			role:      "verify"
			statement: "Before citing a versioned or dated fact: VERIFY the source's date or version against the current one; stale ⇒ mark the claim dated."
			title:     "Currency"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-RUN-005"
			pred:      "sources_disagree"
			role:      "stop"
			statement: "If primary sources disagree and a second reading does not resolve it ⇒ STOP: report both positions with citations and name /investigate."
			title:     "Contested"
		}
	}, {
		entry: {
			id: "SKILL-RESEARCH-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "The reader's spawn prompt."
			title:     "Reader brief"
		}
	}]
}, {
	title: "Output"
	blocks: [{
		entry: {
			id:        "SKILL-RESEARCH-OUT-001"
			role:      "step"
			statement: "ANSWER first, in plain prose; DERIVE confidence from the count of cited primaries read in this run (two or more agreeing high, one medium, none low)."
			title:     "Answer"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-OUT-002"
			role:      "step"
			statement: "LIST the sources used, one line each: locator, what it settled, date or version."
			title:     "Citations"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-OUT-004"
			role:      "verify"
			statement: "Before return: VERIFY every cited locator was read or fetched in this run, by the agent or a reader; unread ⇒ drop the citation and recompute the label."
			title:     "Read citations"
		}
	}, {
		entry: {
			id:        "SKILL-RESEARCH-OUT-003"
			role:      "step"
			statement: "STATE what no source settled, one line, or omit when none."
			title:     "Gaps"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "reading and citation discipline; frontier adds nothing a source does not"
			tier:   "mid-tier"
			notes: ["Readers: bracket model/effort from `subagents`, mechanical reading — `sonnet:medium` is the sensible default"]
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "question: which make target regenerates a skill's SKILL.md from its <leaf>.cue sources: Makefile"
}
