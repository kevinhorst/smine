changelog: [{
	date:    "2026-10-17"
	version: "6.2.1"
	text:    "Model file is <leaf>.cue"
	tokens:  4452
}, {
	date:    "2026-10-14"
	version: "6.2.0"
	text:    "Interfaces: one variable per capability — TASK_CHECK"
	tokens:  4443
}, {
	date:    "2026-10-13"
	version: "6.1.0"
	text:    "The task check is the env variable $SKILL_SMINECONTEXT_RULES (required) in the write verify and the caps item."
	tokens:  4436
}, {
	date:    "2026-10-09"
	version: "6.0.0"
	text:    "Task problem is one plain sentence ≤ 160 runes gated by task-check; evidence carries the record finding index."
	tokens:  4406
}, {
	date:    "2026-10-08"
	version: "5.3.0"
	text:    "Task text caps: problem ≤ 320, fix ≤ 240, quotes ≤ 160 as verbatim excerpts; smine-rules task-check replaces the jq validate"
	tokens:  4310
}, {
	date:    "2026-10-08"
	version: "5.2.0"
	text:    "Task carries problem, evidence and fix instead of finding."
	tokens:  4223
}, {
	date:    "2026-10-06"
	version: "5.1.1"
	text:    "Context-surface guard keys on context/aspects.json; context/context.json is generated and untracked."
	tokens:  4126
}, {
	date:    "2026-10-04"
	version: "5.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  4124
}, {
	date:    "2026-10-04"
	version: "5.0.1"
	text:    "Session records are read as the JSON files make sessions lists; the markdown record is gone."
	tokens:  4124
}, {
	date:    "2026-10-04"
	version: "5.0.0"
	text:    "Miner, not planner: the plan route is gone; route mine reads the unsynthesized session records and writes context change tasks to .smine-run/changes/, capped by --cap, intended-cost candidates dropped; acdsl-context declares ACTION-*, FACT-*, RULE-*."
	tokens:  4125
}, {
	date:    "2026-09-28"
	version: "4.1.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2387
}, {
	date:    "2026-09-28"
	version: "4.1.0"
	text:    "acdsl-context declarations target skills/<path>/skill.cue (GROUP-005a); SKILL.md is generated."
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
	text:    "Planner contract: mining from batches retired; plan route returns one artifact plan for a problem direction and writes nothing; per-kind knowledge kept."
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "formal-schema reformat: section-kind clauses on every topic section, bullet registers normalized to Applies, pred/post clauses on 15 entries (profile-read gating, suppression-set ordering, gate/reroute/threshold rules, validate-then-stop), sibling atoms.json (12 bool, 1 int), three queries proving the single-session and J-band evidence thresholds plus the machine-checkable reroute invariant"
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format v2 (major): When to use replaced by Routing (Purpose/Disambiguation/Preconditions/Pipeline), bullet-form Args synced to the description, full example argument-hint, allowed-tools normalized (plain alpha, then Bash/mcp__ grouped and sorted), title links for internal step refs, content unchanged"
}, {
	date:    "2026-09-05"
	version: "1.37"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.36"
	text:    "prose-only de-scope — gate/band classification moved to the new smine-acdsl dimension: context proposals carry no gate object; machine-checkable candidates, ACDSL marker lines and ACDSL-rule evictions (with the verdict-join bar) reroute to smine-acdsl; judgment rules stay as worked-example prose tagged ab-required"
}, {
	date:    "2026-09-01"
	version: "1.35"
	text:    "absent-context-surface guard — when context/context.json is missing (an install without the private context surface) the skill produces nothing and reports 'no context surface on this install'; it never fabricates a taxonomy"
}, {
	date:    "2026-08-27"
	version: "1.34"
	text:    "casual presentation rule — on audience: casual the user-visible prose fields carry no file paths, no rule/FACT/ACDSL IDs, and no schema or taxonomy jargon; technical anchors move to target/anchor/code/snippet fields"
}, {
	date:    "2026-08-27"
	version: "1.33"
	text:    "presentation-profile language contract for proposal prose; surface-path group titles and enforcement rule text stay English"
}, {
	date:    "2026-08-27"
	version: "1.32"
	text:    "batch input glob now every sessions/ folder except archived/ (work/personal scopes retired for per-repo folders)"
}, {
	date:    "2026-08-23"
	version: "1.31"
	text:    "evalscript renamed to verifier across prose — gate.verifier field and sketched-verifier wording"
}, {
	date:    "2026-08-23"
	version: "1.30"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.29"
	text:    "seam update for the smine-memory redesign: context.json is shared with smine-memory (fact-surface groups, runs after this skill); fact/knowledge candidates skip to smine-memory instead of memory"
}, {
	date:    "2026-08-23"
	version: "1.28"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-23"
	version: "1.27"
	text:    "acdsl-context: key rename — the ACDSL declaration key moved off Claude Code's native context: frontmatter field; prose references updated"
}, {
	date:    "2026-08-18"
	version: "1.26"
	text:    "mines the used-context record — context-use findings routed (violated-despite-present → gate route, needed-not-present → declaration/marker/new-rule, present-irrelevant → eviction with the ≥300 clean-runs verdict bar), channel stats in the run report; legacy status-less context.json archived to rejected.md and reset"
}, {
	date:    "2026-08-13"
	version: "1.25"
	text:    "skill-vs-context boundary tightened — only procedure changes reroute to /smine-skills; entries serving named skills carry the context: declaration in their change (one rule, one vote); Finish validates via go run ./cmd/acdsl check (Bash added to allowed-tools)"
}, {
	date:    "2026-08-12"
	version: "1.24"
	text:    "dynamic scope roster — input glob sessions/*/ (every scope dir except proposals/), personal/work no longer hardcoded"
}, {
	date:    "2026-08-11"
	version: "1.23"
	text:    "renamed smine-style → smine-context (name/H1/trigger/workflow position); the kind was already context, this catches the skill name up (--no-style → --no-context in /smine)"
}, {
	date:    "2026-08-10"
	version: "1.22"
	text:    "reconciliation guard — step 3 also sweeps existing proposed entries already applied/archived; title contract — split siblings never repeat a shared candidate heading"
}, {
	date:    "2026-08-09"
	version: "1.21"
	text:    "repo tags for reach routing — repo:<name> on repo-concentrated evidence, untagged = global; smine-apply maps tags to Reach"
}, {
	date:    "2026-08-09"
	version: "1.20"
	text:    "output is sessions/proposals/context.json (kind context, single artifact — kind style retired); target paths on the facts/rules/actions layout"
}, {
	date:    "2026-08-09"
	version: "1.19"
	text:    "identity-grammar wording (ACTION activity chapters)"
}, {
	date:    "2026-08-09"
	version: "1.18"
	text:    "sketched-evalscript proposals must carry violation + clean snippets as fixture seeds, else they stay J-form prose"
}, {
	date:    "2026-08-09"
	version: "1.17"
	text:    "context proposals carry gate {band, evalscript, anchor}; thresholds tightened (2 sessions, 3 + worked example + ab-required for J-band)"
}, {
	date:    "2026-07-31"
	version: "1.16"
	text:    "session-scope rename to personal/work (input globs)"
}, {
	date:    "2026-07-30"
	version: "1.15"
	text:    "renamed smine-rules → smine-style (kind rename rules → style; output style.json); per-run production cap arg"
}, {
	date:    "2026-07-31"
	version: "1.14"
	text:    "navigating.md rename; stale context/go surface folded into context/style"
}, {
	date:    "2026-07-31"
	version: "1.13"
	text:    "activity-scoped context — target surfaces are rules/ activity chapters and style/ guides"
}, {
	date:    "2026-07-30"
	version: "1.12"
	text:    "context redesign — target surfaces: context/rules entries + relocated skill assets"
}, {
	date:    "2026-07-30"
	version: "1.11"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-27"
	version: "1.10"
	text:    "renamed analyze-rules → smine-rules; part of the /smine fan-out; sibling reroute → /smine-skills"
}, {
	date:    "2026-07-26"
	version: "1.9"
	text:    "JSON-only — rules.json is the sole authoritative artifact; the md form and the regenerate-from-md step are gone, evidence/format sections re-anchored to schema fields"
}, {
	date:    "2026-07-26"
	version: "1.8"
	text:    "proposed-date stamp — new items carry proposed: <date> (schema.json bump)"
}, {
	date:    "2026-07-24"
	version: "1.7"
	text:    "one proposal = one change = one vote — proposal-level change field, evidence provenance-only, --<n> split ids (schema.json bump)"
}, {
	date:    "2026-07-24"
	version: "1.6"
	text:    "mechanical Archive-suppression step (step 3) replaces the advisory archive line; Diff & write gains the Amendments-beat-New justification requirement"
}, {
	date:    "2026-07-22"
	version: "1.5"
	text:    "archive doctrine incl. postponed 14-day suppression; status vocab completed"
}, {
	date:    "2026-07-19"
	version: "1.4"
	text:    "ledger carries a Last-analyzed-batch first line, updated on every append"
}, {
	date:    "2026-07-19"
	version: "1.3"
	text:    "per-session note bullets replace the evidence text blob; dimension must match batch findings"
}, {
	date:    "2026-07-19"
	version: "1.2"
	text:    "structured evidence contract (title + 1-3 ranked session ids + optional change/snippets)"
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
	value: " smine-context"
}, {
	key:   "description"
	value: " Mine the session records' context-use signals into context change tasks — entry adds, edits and acdsl-context declarations. Trigger on /smine-context mine or the smine context stage. Args — mine: read the unsynthesized records, write tasks; --nightly: unattended; --cap: max tasks this run (default 5)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 6.2.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"mine --nightly --cap 3\""
}, {
	key:   "allowed-tools"
	value: " Read, Write, Bash(jq *), Bash(make sessions *)"
}, {
	key:   "acdsl-context"
	value: " ACTION-*, FACT-*, RULE-*"
}]
preamble: """
	# smine: context miner

	Reads the unsynthesized session records, turns what they show about context use — rules missing, ignored, stale or
	rediscovered — into context change tasks under `.smine-run/changes/`; never edits a doc or writes a store.
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
				note: "context stage of the nightly; the implement stage turns each task into one commit"
			}
			preconditions: ["at least one session record without `synthesized`", "`context/aspects.json` present — absent: write nothing, report \"no context surface on this install\", never fabricate a taxonomy"]
			purpose: "the context miner — turns context-use evidence in session records into change tasks on context docs and acdsl-context declarations; never edits a doc."
			routes: [{
				for: "mining transcripts into session records"
				use: "/smine-batch"
			}, {
				for: "a machine-checkable rule sessions keep violating"
				use: "/smine-acdsl mine"
			}, {
				for: "a change to a skill's procedure (steps, args, routing)"
				use: "/smine-problems (smine-skills planner)"
			}, {
				for: "authoring one known entry by hand"
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
				effect:  "the only route: read the unsynthesized records, write context change tasks — [Resolve records](#skill-sminecontext-mine-001)"
				form:    "route"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: no review stop, no question — [Nightly continues](#skill-sminecontext-mine-000)"
				form:    "flag"
			}, {
				name:    "--cap"
				default: "`5`"
				effect:  "max task files this run, strongest first; drops listed in the report — [Cap and write](#skill-sminecontext-mine-006)"
				form:    "`<n>`"
			}]
		}
	}]
}, {
	title: "Inputs"
	blocks: [{
		entry: {
			id: "SKILL-SMINECONTEXT-INPUT-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "read — findings with `contextUse {kind, channel, ruleId}`, quotes, the `context` object's injected block, `cost`."
			}]
			role:      "def"
			statement: "Record input: the records `make sessions ARGS=\"unsynthesized\"` lists — the current batch run's records included — read as JSON; never a transcript."
			title:     "Record input"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-INPUT-002"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "`docs/` holds the explanations a mechanism's entry points to; plan TLDRs and the backlog state intent not yet built."
			}]
			role:      "def"
			statement: "Live surface: the context index `context/context.json` (every entry, its statement and Applies) plus the context docs and `docs/` it points to."
			title:     "Live surface"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-TASK-001"
			items: [{
				fence: {
					body: """
						{"id": "context/<slug>", "kind": "context", "title": "", "problem": "", "evidence": [], "fix": "", "sessions": [], "target": "", "change": "", "notes": ""}

						"""
					info:   "json"
					marker: "```"
				}
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`id` = `context/<slug>`; `kind` = `context`; `title` one line; `sessions` the evidence session ids, strongest first."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`problem` = one plain sentence ≤ 160 runes stating what is wrong for a reader who was not in the session."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "no ids, shas, paths, line numbers, code spans or parenthesised counts — `$SKILL_SMINECONTEXT_TASK_CHECK task-check` refuses them."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`fix` = why the change fixes it, ≤ 240 characters."
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
				text:   "`change` one imperative sentence; `target` repo-relative path."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`notes` = implementer mechanics: entry id grammar and next free number, chapter, `Reach:`, consuming skills' declarations."
			}]
			role:      "def"
			statement: "Change task: one JSON file `.smine-run/changes/context-<slug>.json` in cwd, keyed as the block below; the Write tool creates the dir."
			title:     "Change task"
		}
	}]
	kind: "context"
}, {
	title: "Surfaces"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONTEXT-GROUP-001"
			role:      "def"
			statement: "`context/actions/*` — ACTION activity chapters (implementing, navigating) + reviewing.md (Definition of Done)."
			title:     "Actions surface"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-GROUP-002"
			role:      "def"
			statement: "`context/rules/*` — artifact style guides and RULE-* candidates (plan.md, commits.md, per-language files incl. go.md's TESTS/GOROUTINES sections)."
			title:     "Rules surface"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-GROUP-003"
			role:      "def"
			statement: "`context/AGENTS.md` — template-level conventions."
			title:     "AGENTS surface"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-GROUP-005b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "gate ACDSL-SKILL-004 keeps declarations resolvable."
			}]
			role:      "def"
			statement: "The frontmatter `acdsl-context:` line lists entry IDs/globs; the skill-context hook injects the matching entries from the generated `context/context.json` at invocation."
			title:     "Acdsl-context declarations, mechanism"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-GROUP-004"
			role:      "rule"
			statement: "Only changes to a skill's **procedure** (steps, args, routing, output contract) are the smine-skills planner's; **knowledge a skill needs** MUST stay context — an entry + declaration."
			title:     "Skills seam"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-GROUP-005a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "one rule, one declaration — no per-skill prose edit."
			}]
			role:      "rule"
			statement: "When an entry exists to serve named skills, the change MUST cover both the entry and \"declare <entry-ID> in skills/<group>/<leaf>/<leaf>.cue `acdsl-context`\" for each consuming skill."
			title:     "Acdsl-context declarations"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-GROUP-006"
			role:      "rule"
			statement: "External repos (their AGENTS.md/CLAUDE.md/docs) are NEVER edited from here — flag them with the exact target path."
			title:     "External repos flagged"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-SETUP-005b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "the `title`, `problem` and `fix` prose of each task."
			}]
			role:      "rule"
			statement: "Skill MUST author user-visible prose per the profile's register and glossary when `language:` is set and not `en`."
			title:     "Language"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-SETUP-005c"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "also rule text for enforcement surfaces (repo docs stay English); absent profile = English, unchanged."
			}]
			role:      "rule"
			statement: "Skill NEVER translates ids, targets, group titles, file paths, code, dates, tags, schema keys, or status values."
			title:     "Never translate"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-SETUP-006"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "technical anchors belong in `target`, `change` and `notes`."
			}]
			role:      "rule"
			statement: "When `audience:` is `casual`, the `title`, `problem` and `fix` prose MUST carry no file paths, no rule/FACT/ACDSL IDs, and no schema or taxonomy jargon — say what changes for the user."
			title:     "Casual presentation"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-DIFF-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "amendments beat new rules — a **new-rule** change carries a one-line justification of why no existing rule or context doc absorbs it."
			}]
			role:      "step"
			statement: "READ the current target surface before composing a task: a rule already covered means no task; a near-miss becomes an edit of the existing entry, never a duplicate."
			title:     "Read target first"
		}
	}]
	kind: "context"
}, {
	title: "Context-use routes"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONTEXT-ROUTES-001b"
			role:      "rule"
			statement: "A machine-checkable violation MUST be left to /smine-acdsl — no context task; the report names it."
			title:     "Reroute checkable"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-ROUTES-002a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "rule exists, no skill scope, should bind on read → reroute to /smine-acdsl (an ACDSL marker line is gate machinery)."
			}]
			role:      "rule"
			statement: "A rule exists but was not injected for a skill → the change is an [acdsl-context declaration](#skill-sminecontext-group-005a) amendment covering the entry + the declaration for that skill."
			title:     "Needed-not-present"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-ROUTES-002b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "rule missing → a new-rule change instead (amendments-beat-new justification)."
			}]
			role:      "rule"
			statement: "`RULE-<LANG>-*` delivery landed (`injected.lang` non-null) → a hook defect named in the report, no task."
			title:     "Needed-not-present, cont"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-ROUTES-003"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "ACDSL-rule evictions (`set projected=\"false\" on <ID>`) reroute to /smine-acdsl, which owns the verdict-join bar."
			}]
			role:      "rule"
			statement: "An ID present but irrelevant → a prose eviction task (`remove <ID> from <skill> acdsl-context` / drop from guide) for prose entries."
			title:     "Present-irrelevant"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-ROUTES-004"
			role:      "rule"
			statement: "An entry a record shows stale or contradicted by the code or a newer decision → an edit task on that entry, its `Location:` or statement corrected, never a second entry."
			title:     "Stale or contradicted"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONTEXT-ROUTES-005"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "target the `context/facts/<domain>.md` file of its domain; `Location:` names the file that proves it."
			}]
			role:      "rule"
			statement: "A fact sessions had to rediscover (the same probe, grep or question in ≥2 sessions) → a FACT entry add task."
			title:     "Rediscovered fact"
		}
	}]
	kind: "context"
}, {
	title: "Mine"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-000"
			role:      "rule"
			statement: "`--nightly` MUST finish without a stop or a question."
			title:     "Nightly continues"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-007"
			role:      "rule"
			statement: "A candidate MUST rest on ≥2 sessions, or on 1 session plus a repo check confirming it; weaker evidence is a drop `below-threshold`."
			title:     "Evidence threshold"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-008"
			role:      "rule"
			statement: "This skill MUST NOT edit a repo file or write a store; its only writes are task files under `.smine-run/changes/`."
			title:     "Tasks only"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-001"
			role:      "step"
			statement: "RUN `make sessions ARGS=\"unsynthesized\"`; READ every listed record JSON."
			title:     "Resolve records"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-002"
			role:      "stop"
			statement: "If the listing is empty ⇒ STOP: report \"no unsynthesized records\", write nothing."
			title:     "Nothing to mine"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-003"
			role:      "step"
			statement: "COLLECT candidates from `contextUse` findings and rediscovered facts; GROUP them per entry id or per fact, counting distinct sessions."
			title:     "Collect candidates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-004"
			role:      "step"
			statement: "DROP a candidate whose cost is the stated purpose of a mechanism as `intended-cost` — see [mining drops intended cost]($AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md)."
			title:     "Intended cost"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-005"
			role:      "verify"
			statement: "VERIFY each survivor against the live surface: covered ⇒ drop `covered`; an existing entry ⇒ the task edits it; then route it by the context-use routes above."
			title:     "Check live surface"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-006"
			role:      "step"
			statement: "RANK survivors by distinct sessions and cost; WRITE at most `--cap` [change tasks](#skill-sminecontext-task-001); LIST every drop as `dropped → <reason>`."
			title:     "Cap and write"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-009"
			role:      "verify"
			statement: "After writing: VERIFY `$SKILL_SMINECONTEXT_TASK_CHECK task-check .smine-run/changes/context-*.json` exits 0; repair a violation in the same run by shortening text, never by cutting a sentence."
			title:     "Validate tasks"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-010"
			role:      "step"
			statement: "REPORT: tasks written (id, target), drops with reasons, reroutes to /smine-acdsl, hook defects."
			title:     "Stage report"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONTEXT-MINE-011"
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
			reason: "judge context-use evidence across records against the live index — no fan-out"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "mine --cap 1"
}
