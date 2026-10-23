changelog: [{
	date:    "2026-10-17"
	version: "7.2.1"
	text:    "Model file is <leaf>.cue"
	tokens:  4184
}, {
	date:    "2026-10-14"
	version: "7.2.0"
	text:    "Interfaces: one variable per capability — TASK_CHECK"
	tokens:  4175
}, {
	date:    "2026-10-13"
	version: "7.1.0"
	text:    "The task check is the env variable $SKILL_SMINEMEMORY_RULES (required) in the write verify and the caps item."
	tokens:  4165
}, {
	date:    "2026-10-09"
	version: "7.0.0"
	text:    "Task problem is one plain sentence ≤ 160 runes gated by task-check; memory provenance moves to notes."
	tokens:  3990
}, {
	date:    "2026-10-09"
	version: "6.4.1"
	text:    "Probe failed exempts a read-gate required-reading denial: read the named files, then retry the Read."
}, {
	date:    "2026-10-09"
	version: "6.4.0"
	text:    "No auto-memory fires only on a `File does not exist` Read answer; a denied or erroring probe ends the stage `failed` with its reason"
}, {
	date:    "2026-10-08"
	version: "6.3.0"
	text:    "Task text caps on problem and fix; evidence stays empty; smine-rules task-check replaces the jq validate"
	tokens:  3927
}, {
	date:    "2026-10-08"
	version: "6.2.0"
	text:    "Task carries problem, evidence and fix instead of finding."
	tokens:  3882
}, {
	date:    "2026-10-06"
	version: "6.1.1"
	text:    "Context-surface guard keys on context/aspects.json; context/context.json is generated and untracked."
	tokens:  3754
}, {
	date:    "2026-10-04"
	version: "6.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  3752
}, {
	date:    "2026-10-04"
	version: "6.0.0"
	text:    "Facts drain into memory change tasks under .smine-run/changes/ instead of the retired proposal store, interactive and nightly alike; --nightly runs as the smine memory stage, ends ok with `no auto-memory` when there is nothing to drain, and never stops for review; archive suppression gone; acdsl-context declares ACTION-*, FACT-*, RULE-*."
	tokens:  3752
}, {
	date:    "2026-10-02"
	version: "5.2.1"
	text:    "Store paths hang off `<store>`, the path `make rules ARGS=\"store-root\"` prints — a smoke or eval run works on its own store, never the live one; the smoke seeds its store from the fixture tree."
	tokens:  4043
}, {
	date:    "2026-10-02"
	version: "5.2.0"
	text:    "Output and archive suppression use the plans-archive store `~/GolandProjects/agent-plans/<repo-name>/proposals/`; Read and Edit grants there."
	tokens:  4039
}, {
	date:    "2026-09-28"
	version: "5.1.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  3963
}, {
	date:    "2026-09-28"
	version: "5.1.0"
	text:    "acdsl-context declarations target skills/<path>/skill.cue (ROUTE-003); SKILL.md is generated."
}, {
	date:    "2026-09-26"
	version: "5.0.2"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "5.0.1"
	text:    "Make-only entry points: allowed-tools and prose call make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "5.0.0"
	text:    "Batch route retired with the problem-first pipeline; migrate is the only route — drains an auto-memory dir into context proposals."
}, {
	date:    "2026-09-24"
	version: "4.1.2"
	text:    "global instructions surface is settings/claude_code/AGENTS.md — CLAUDE.md retired"
}, {
	date:    "2026-09-15"
	version: "4.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "4.1.0"
	text:    "Full formal schema: section kinds (context on Rules, phase on the six pipeline sections), pred/post clauses on 31/33 coverable entries incl. the run-mode gate and enforcement stop, sibling atoms.json (36 bool, 2 int, run_mode enum), three queries — enforcement blocks repo tagging, batch mode blocks the migrate threshold, single verified session clears the evidence threshold."
}, {
	date:    "2026-09-14"
	version: "4.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "3.0.0"
	text:    "format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), argument-hint as full example invocation, allowed-tools normalized, §2 cross-ref linked, prose compressed"
}, {
	date:    "2026-09-05"
	version: "2.7"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "2.6"
	text:    "gate wording — gate machinery is kind acdsl (smine-acdsl dimension); facts stay gate-free"
}, {
	date:    "2026-09-01"
	version: "2.5"
	text:    "absent-context-surface guard — when context/context.json is missing (an install without the private context surface) the skill produces nothing and reports 'no context surface on this install'; it never fabricates a taxonomy"
}, {
	date:    "2026-08-27"
	version: "2.4"
	text:    "casual presentation rule — on audience: casual the user-visible prose fields carry no file paths, no rule/FACT/ACDSL IDs, and no schema or taxonomy jargon; technical anchors move to target/anchor/code/snippet fields"
}, {
	date:    "2026-08-27"
	version: "2.3"
	text:    "presentation-profile language contract for proposal prose; fact-surface group titles and context-doc fact text stay English"
}, {
	date:    "2026-08-27"
	version: "2.2"
	text:    "batch input glob now every sessions/ folder except archived/ (work/personal scopes retired for per-repo folders)"
}, {
	date:    "2026-08-23"
	version: "2.1"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "2.0"
	text:    "redesign: memory → context — output is proposals/context.json (kind context, fact surfaces: context/facts/*, acdsl-context declarations, CLAUDE.md), never auto-memory writes; archive suppression + occurrence thresholds (≥2 sessions or 1 + repo verification); new migrate mode drains auto-memory dirs into the same proposal flow; consolidate-memory dependency dropped; runs after smine-context (shared context.json)"
}, {
	date:    "2026-08-23"
	version: "1.13"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-12"
	version: "1.12"
	text:    "dynamic scope roster — input glob sessions/*/ (every scope dir except proposals/), personal/work no longer hardcoded"
}, {
	date:    "2026-08-11"
	version: "1.11"
	text:    "reference rename — smine-style → smine-context; rules/plan.md path corrected"
}, {
	date:    "2026-08-09"
	version: "1.10"
	text:    "identity-grammar wording (ACTION/FACT entries)"
}, {
	date:    "2026-07-31"
	version: "1.9"
	text:    "session-scope rename to personal/work (input globs)"
}, {
	date:    "2026-07-30"
	version: "1.8"
	text:    "reference rename — smine-rules → smine-style"
}, {
	date:    "2026-07-31"
	version: "1.7"
	text:    "activity-scoped context — wrong-surface list names reviewing.md DoD and style/plan.md"
}, {
	date:    "2026-07-30"
	version: "1.6"
	text:    "context redesign — wrong-surface list includes FACT/NEVER/ALWAYS entries"
}, {
	date:    "2026-07-30"
	version: "1.5"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-27"
	version: "1.4"
	text:    "renamed analyze-memory → smine-memory; part of the /smine fan-out; sibling handoff → /smine-rules"
}, {
	date:    "2026-07-26"
	version: "1.3"
	text:    "Args section"
}, {
	date:    "2026-07-19"
	version: "1.2"
	text:    "ledger carries a Last-analyzed-batch first line, updated on every append"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-11"
	version: "1.0"
	text:    "initial version"
}]
env: {
	TASK_CHECK: {
		interface: "task-check@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-memory"
}, {
	key:   "description"
	value: " Drain an auto-memory dir into memory change tasks that move its facts into context docs. Trigger on /smine-memory migrate [project-dir] or the smine memory stage. Args — migrate: the auto-memory dir to drain (default: the current project's); --nightly: unattended, no auto-memory ends ok."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 7.2.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"migrate ~/.claude/projects/-Users-devuser-repo/memory --nightly\""
}, {
	key:   "allowed-tools"
	value: " Read, Read(~/.claude/projects/**), Write, Bash(jq *)"
}, {
	key:   "acdsl-context"
	value: " ACTION-*, FACT-*, RULE-*"
}]
preamble: """
	# smine: memory → context

	Drains an auto-memory dir (`MEMORY.md` + fact files) into memory change tasks under `.smine-run/changes/`, each moving
	one fact into the context doc it belongs in — visible, synced, reach-routed. Never writes or deletes auto-memory;
	interactive and nightly runs write the same task files.
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
				note: "memory stage of the nightly; the implement stage turns each task into one commit"
			}
			preconditions: ["`context/aspects.json` present — absent it: write nothing, report \"no context surface on this install\", never fabricate a taxonomy", "tasks only — never edits context docs, global instructions, or memory files itself"]
			purpose: "drains an auto-memory dir into memory change tasks targeting context docs; never auto-memory writes."
			routes: [{
				for: "context-use evidence in session records"
				use: "/smine-context mine"
			}, {
				for: "mining sessions into problems"
				use: "/smine-problems"
			}, {
				for: "a single known fact"
				use: "/context-create"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "migrate"
				default: "the current project's dir"
				effect:  "drain this auto-memory dir (`MEMORY.md` + fact files) — [Resolve dir](#skill-sminememory-setup-002)"
				form:    "`[dir]`"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: nothing to drain ends ok, no review stop — [No auto-memory](#skill-sminememory-setup-003)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id: "SKILL-SMINEMEMORY-RULES-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "`notes` names the memory file path + its description line and the recurrence count."
			}]
			role:      "def"
			statement: "Memory files carry no session IDs — a task's `sessions` and `evidence` are `[]`, provenance lives in `notes`."
			title:     "Evidence format"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-TASK-001"
			items: [{
				fence: {
					body: """
						{"id": "memory/<slug>", "kind": "memory", "title": "", "problem": "", "evidence": [], "fix": "", "sessions": [], "target": "context/facts/<domain>.md", "change": "", "notes": ""}

						"""
					info:   "json"
					marker: "```"
				}
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`id` = `memory/<slug>`; `kind` = `memory`; `target` = the context fact file the fact belongs in, or the global instructions."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`problem` = one plain sentence ≤ 160 runes stating what is wrong for a reader who was not in the session."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "no ids, shas, paths, line numbers, code spans or parenthesised counts — `$SKILL_SMINEMEMORY_TASK_CHECK task-check` refuses them."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`fix` = why the change fixes it, ≤ 240 characters."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`evidence` = `[]` and `sessions` = `[]` — memory files carry no session ids ([Evidence format](#skill-sminememory-rules-001))."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`change` one imperative sentence; `notes` = entry id grammar and next free number, `Location:`, `Reach:`, consuming skills' declarations."
			}]
			role:      "def"
			statement: "Change task: one JSON file `.smine-run/changes/memory-<slug>.json` in cwd, keyed as the block below; the Write tool creates the dir."
			title:     "Change task"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-RULES-003"
			role:      "rule"
			statement: "This skill NEVER edits a context doc, the global instructions, a memory file, or an external repo, and NEVER deletes a memory file; its only writes are task files."
			title:     "Tasks only"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-RULES-004"
			role:      "rule"
			statement: "`--nightly` MUST finish without a stop or a question."
			title:     "Nightly continues"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-RULES-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Evidence MUST quote verbatim; dropping a candidate MUST beat bloating it — an unclear candidate is a drop, not a hedge-write."
			title:     "Drop over hedge"
		}
	}]
	kind: "context"
}, {
	title: "0. Setup"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEMEMORY-SETUP-001"
			role:      "def"
			statement: "Input: the fact files listed in the target dir's `MEMORY.md`."
			title:     "Input"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-SETUP-005b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "each task's `title`, `problem` and `fix`."
			}]
			role:      "rule"
			statement: "When `language:` is set, not `en`: author user-visible prose fields per the profile's register and glossary."
			title:     "Language"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-SETUP-005c"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "ids, targets, file paths, code, context-doc fact text (repo docs stay English), dates, tags, schema keys."
			}]
			role:      "rule"
			statement: "NEVER translate identifiers, paths, code, or fact text destined for context docs; absent profile = English, unchanged."
			title:     "Never translate"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-SETUP-006a"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "When `audience:` is `casual`, `title`, `problem` and `fix` MUST carry no file paths, IDs, or schema/taxonomy jargon — say what changes for the user."
			title:     "Casual presentation"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-SETUP-006b"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Casual mode: technical anchors go in `target`, `change` and `notes` instead of prose."
			title:     "Casual anchors"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-SETUP-002"
			role:      "step"
			statement: "RESOLVE the dir: the `migrate` arg, else the current project's `~/.claude/projects/<cwd with every / and . as ->/memory/`."
			title:     "Resolve dir"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-SETUP-003"
			role:      "stop"
			statement: "If a Read of `<dir>/MEMORY.md` answers `File does not exist`, or a Read of every fact file it lists does ⇒ STOP: the stage ends ok with summary `no auto-memory`, nothing written."
			title:     "No auto-memory"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-SETUP-003b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "a read-gate required-reading denial is not the probe's answer: read the files it names, then retry the Read."
			}]
			role:      "stop"
			statement: "If that probe is denied or errors any other way ⇒ STOP: the stage ends `failed` with the denial or error quoted as reason, nothing written."
			title:     "Probe failed"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-SETUP-005a"
			role:      "step"
			statement: "READ `~/.claude/context/global/presentation-profile.md` before writing output."
			title:     "Read profile"
		}
	}]
	kind: "phase"
}, {
	title: "1. Extract & cluster"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEMEMORY-EXTRACT-001"
			role:      "step"
			statement: "COLLECT memory candidates from the target dir's fact files, with their provenance — the memory file path + its description line."
			title:     "Collect candidates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-EXTRACT-002"
			role:      "step"
			statement: "CLUSTER duplicates across fact files — the same fact recurs; recurrence count is evidence, keep it."
			title:     "Cluster duplicates"
		}
	}]
	kind: "phase"
}, {
	title: "2. Evaluate (the filter)"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEMEMORY-EVAL-001b"
			role:      "rule"
			statement: "Existing auto-memory files are NOT a coverage source — a fact living only in auto-memory MUST become a task; that is the migration ratchet."
			title:     "Not auto-memory"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-EVAL-001a"
			role:      "step"
			statement: "DROP clusters already covered by the context surfaces: `context/context.json` + the fact/action/rule files, or global instructions (`settings/claude_code/AGENTS.md`)."
			title:     "Already covered"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-EVAL-002"
			role:      "step"
			statement: "DROP clusters derivable from the repo itself (code, git history, AGENTS.md of the target project)."
			title:     "Derivable drop"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-EVAL-003"
			role:      "step"
			statement: "DROP clusters that are wrong surface: enforcement rules, doc drift, review-DoD material, style conventions — skip with a note; not this skill's surface."
			title:     "Wrong-surface drop"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-EVAL-004"
			role:      "step"
			statement: "DROP a feedback fact asking to remove a cost that is a mechanism's stated purpose as `intended-cost` — see [mining drops intended cost]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md)."
			title:     "Intended cost"
		}
	}]
	kind: "phase"
}, {
	title: "3. Route & write"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-002a"
			role:      "def"
			statement: "`context/facts/<domain>.md` — FACT entries, ID per `KIND-SCOPE[-TOPIC]-NNN` grammar; `Location:` required."
			title:     "Facts surface"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-002b"
			role:      "def"
			statement: "Scope/topic come from the registered taxonomy (`context/aspects.json`); new scopes register in `context/actions/README.md` first."
			title:     "Facts taxonomy"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-004"
			role:      "def"
			statement: "`settings/claude_code/AGENTS.md` — user-level preferences and cross-project working facts (the old `user`/`feedback` memory types) become global-instructions edit tasks."
			title:     "Global instructions surface"
		}
	}, {
		entry: {
			id: "SKILL-SMINEMEMORY-ROUTE-009"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Read the target surface before writing: near-misses MUST become edits of the existing entry; a new-entry task carries a one-line justification of why no entry absorbs it."
			title:     "Amendments beat new"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-006"
			role:      "stop"
			statement: "If a candidate is enforcement, not knowledge ⇒ STOP routing it as a fact: gate machinery is smine-acdsl's; drop it as a [wrong-surface drop](#skill-sminememory-eval-003)."
			title:     "No gate object"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-007"
			role:      "verify"
			statement: "VERIFY each fact against the repo before writing its task; a stale memory fact is a drop, not a task."
			title:     "Threshold"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-008a"
			role:      "step"
			statement: "SET `Reach:` in `notes`: evidence concentrated in some repos → their roster names (target dir basename); spread across most repos → none (global)."
			title:     "Reach"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-003"
			role:      "step"
			statement: "When a fact exists to serve named skills: the task's change covers both the entry and \"declare <entry-ID> in skills/<group>/<leaf>/<leaf>.cue `acdsl-context`\" for each consuming skill."
			title:     "Acdsl-context declarations"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-005"
			role:      "step"
			statement: "FLAG external repos not on the sync roster with the exact target path in the report; never a task for them."
			title:     "External repos flagged"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-ROUTE-001a"
			role:      "step"
			statement: "WRITE one [change task](#skill-sminememory-task-001) per surviving cluster."
			title:     "Write tasks"
		}
	}]
	kind: "phase"
}, {
	title: "4. Finish"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEMEMORY-FINISH-001a"
			role:      "verify"
			statement: "After writing: VERIFY `$SKILL_SMINEMEMORY_TASK_CHECK task-check .smine-run/changes/memory-*.json` exits 0; a violation is repaired in the same run by shortening the text, never by cutting a sentence."
			title:     "Validate"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-FINISH-002"
			role:      "step"
			statement: "REPORT: tasks written (per target), dropped-covered, dropped-derivable, dropped-intended-cost, skipped-wrong-surface, stale — counts plus the task list."
			title:     "Run report"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEMEMORY-FINISH-003"
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
			reason: "clustering candidates against context surfaces and diffing against existing entries"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	fixture: {
		tree: "tree"
	}
	invoke: "migrate smoke-memory"
}
