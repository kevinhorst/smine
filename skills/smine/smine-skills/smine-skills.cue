changelog: [{
	date:    "2026-10-17"
	version: "6.1.1"
	text:    "Model file is <leaf>.cue"
	tokens:  2230
}, {
	date:    "2026-10-13"
	version: "6.1.0"
	text:    "The caps item names the task check, not the rules program — no program literal left."
	tokens:  2227
}, {
	date:    "2026-10-09"
	version: "6.0.0"
	text:    "Task problem is one plain sentence ≤ 160 runes gated by task-check; evidence carries the record finding index; backlog id is its own field."
	tokens:  2234
}, {
	date:    "2026-10-09"
	version: "5.4.0"
	text:    "Runtime placement declared: spawned only, never at root."
	tokens:  2117
}, {
	date:    "2026-10-08"
	version: "5.3.0"
	text:    "Task text caps: problem ≤ 320 with one incident written out, fix ≤ 240, quotes ≤ 160 as verbatim excerpts of the problem's evidence notes"
	tokens:  2117
}, {
	date:    "2026-10-08"
	version: "5.2.0"
	text:    "Task carries problem, evidence and fix instead of finding."
	tokens:  2036
}, {
	date:    "2026-10-04"
	version: "5.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  1946
}, {
	date:    "2026-10-04"
	version: "5.0.0"
	text:    "The plan route returns {task, reason} — one skills change task for the implement stage or null — instead of an artifact plan; smine-apply references dropped; acdsl-context adds the intended-cost doctrine."
	tokens:  1946
}, {
	date:    "2026-09-28"
	version: "4.0.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1598
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
	text:    "Formal-schema reformat: section-kind lines on the two context sections (2b, Rules); pred/post clauses on the causal setup->extract->inventory->archive->rank->finish chain and 10 rule/quality invariants; sibling atoms.json (28 bool, 2 int); two committed queries (cap-holds-at-boundary, new-skill-needs-justification) — content unchanged."
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
	text:    "format v2 reformat — Routing section replaces When to use, Args switched to bullet form, allowed-tools normalized, context id linked, content identical"
}, {
	date:    "2026-09-05"
	version: "1.27"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-04"
	version: "1.26"
	text:    "acdsl-context added with FACT-CLAUDE-*: skill/workflow proposal mining now sees the Claude platform facts (prompt-cache mechanics, harness behavior, multi-agent cost law)"
}, {
	date:    "2026-08-27"
	version: "1.25"
	text:    "casual presentation rule — on audience: casual the user-visible prose fields carry no file paths, no rule/FACT/ACDSL IDs, and no schema or taxonomy jargon; technical anchors move to target/anchor/code/snippet fields"
}, {
	date:    "2026-08-27"
	version: "1.24"
	text:    "presentation-profile language contract for proposal prose; the three group titles flagged as verbatim machine-read keys"
}, {
	date:    "2026-08-27"
	version: "1.23"
	text:    "casual lockout at extraction — with audience: casual, edit proposals may target only skills carrying origin: user frontmatter (others dropped, listed in the run report); batch input glob now every sessions/ folder except archived/ (scopes retired for per-repo folders)"
}, {
	date:    "2026-08-23"
	version: "1.22"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.21"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-23"
	version: "1.20"
	text:    "acdsl-context: key rename — the ACDSL declaration key moved off Claude Code's native context: frontmatter field; prose references updated"
}, {
	date:    "2026-08-18"
	version: "1.19"
	text:    "absorbs smine-workflows — workflow candidates are a third kind, stored in the \"Workflows (skill-bundled scripts)\" group of skills.json with the owning skill as target"
}, {
	date:    "2026-08-13"
	version: "1.18"
	text:    "skill-edit-vs-context boundary (step 2b) — knowledge a skill needs reroutes to /smine-context as entry + context: declaration, cross-skill rules never split into sibling edits; Finish validates via go run ./cmd/acdsl check (Bash added to allowed-tools)"
}, {
	date:    "2026-08-12"
	version: "1.17"
	text:    "dynamic scope roster — input glob sessions/*/ (every scope dir except proposals/), personal/work no longer hardcoded"
}, {
	date:    "2026-08-10"
	version: "1.16"
	text:    "reconciliation guard — step 3 also sweeps existing proposed entries (drop those already shipped in done.md or present in the target skill; re-point renamed targets across id/title/target); title contract <target> — <distinct name>, split siblings never repeat a shared candidate heading"
}, {
	date:    "2026-08-09"
	version: "1.15"
	text:    "repo tags for routing — repo:<name> on repo-concentrated evidence; foreign-repo proposals disposition manual-external"
}, {
	date:    "2026-07-31"
	version: "1.14"
	text:    "session-scope rename to personal/work (input globs)"
}, {
	date:    "2026-07-30"
	version: "1.13"
	text:    "per-run production cap arg (smine pipeline --max-proposals-per-dimension)"
}, {
	date:    "2026-07-30"
	version: "1.12"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-27"
	version: "1.11"
	text:    "renamed analyze-skill → smine-skills; part of the /smine fan-out; skill authoring handoff → /skillroutine-create"
}, {
	date:    "2026-07-26"
	version: "1.10"
	text:    "JSON-only — skills.json is the sole authoritative artifact; the md form and the regenerate-from-md step are gone, sections/evidence re-anchored to schema fields"
}, {
	date:    "2026-07-26"
	version: "1.9"
	text:    "proposed-date stamp — new entries carry proposed: <date> (schema.json bump)"
}, {
	date:    "2026-07-24"
	version: "1.8"
	text:    "one proposal = one change = one vote — proposal-level change field, evidence provenance-only, --<n> split ids (schema.json bump)"
}, {
	date:    "2026-07-24"
	version: "1.7"
	text:    "mechanical Archive-suppression step (step 3) replaces the advisory archive line — build the suppression set from done/rejected/postponed and drop matching candidates, listing drops; Inventory step gains the Edits-beat-New justification requirement"
}, {
	date:    "2026-07-22"
	version: "1.6"
	text:    "archive doctrine incl. postponed 14-day suppression; status vocab completed"
}, {
	date:    "2026-07-19"
	version: "1.5"
	text:    "ledger carries a Last-analyzed-batch first line, updated on every append"
}, {
	date:    "2026-07-19"
	version: "1.4"
	text:    "per-session note bullets replace the evidence text blob; dimension must match batch findings"
}, {
	date:    "2026-07-19"
	version: "1.3"
	text:    "structured evidence contract (title + 1-3 ranked session ids + snippets); target + change mandatory for edit proposals"
}, {
	date:    "2026-07-16"
	version: "1.2"
	text:    "skill authoring delegates to /couchskill-create"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-11"
	version: "1.0"
	text:    "initial version"
}]
enableDelegation: "always"
frontmatter: [{
	key:   "name"
	value: " smine-skills"
}, {
	key:   "description"
	value: " Turn one solution direction of a mined problem into one skills change task — returns it, writes nothing. Trigger on `plan` from smine-problems. Args — plan: the problem JSON and direction index arrive in the prompt."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 6.1.1"
}, {
	key:   "argument-hint"
	value: " \"plan\""
}, {
	key:   "allowed-tools"
	value: " Read"
}, {
	key:   "acdsl-context"
	value: " ACTION-IMPL-ARCH-004, FACT-CLAUDE-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "format"
	value: " 2"
}]
preamble: """
	# smine: skill planner

	Turns one solution direction of a mined problem into one skills change task — a skill or workflow edit the implement
	stage commits — returns it, writes nothing.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "spawnedBy"
					skill: "smine-problems"
				}]
				note: "per solution direction; returns one change task or null to smine-problems, which writes it"
			}
			preconditions: ["a problem JSON and a direction index in the prompt"]
			purpose: "turns one solution direction of a mined problem into one skills change task — returns it, writes nothing. Spawned by smine-problems, one per direction."
			routes: [{
				for: "creating or editing an actual skill, or writing a workflow script"
				use: "/skillroutine-create"
			}, {
				for: "a routine for this direction"
				use: "the smine-routines planner"
			}, {
				for: "synthesizing problems from session records"
				use: "/smine-problems"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "plan"
				effect: "the problem JSON and direction index arrive in the prompt; return `{task, reason}`, write nothing."
			}]
		}
	}]
}, {
	title: "Skill knowledge"
	blocks: [{
		entry: {
			id: "SKILL-SMINESKILLS-EDITCTX-001a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "procedure-consumed knowledge = repo facts, style rules, review checklists, constraints from failure cases."
			}]
			role:      "def"
			statement: "The boundary test: a change to the skill's **procedure** (steps, args, routing, output contract) is a skill edit; knowledge it consumes is context."
			title:     "Boundary test"
		}
	}, {
		entry: {
			id: "SKILL-SMINESKILLS-EDITCTX-002a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "see [the acdsl-context declaration stays resolvable rule]($AGENT_CONTEXT_DIR_DEFAULT/rules/acdsl-skill.md)."
			}]
			role:      "def"
			statement: "Mechanism: a skill's frontmatter `acdsl-context:` line declares context-entry IDs/globs; the skill-context hook injects matching entries from `context/context.json` at invocation."
			title:     "Reroute mechanism"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-EDITCTX-002b"
			role:      "def"
			statement: "Context reaches the skill without touching its prose."
			title:     "Context untouched prose"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-EDITCTX-003b"
			role:      "def"
			statement: "That duplication is the reroute's strongest signal (precedent: the batch-29 rendered-mockup pair)."
			title:     "Duplication signal"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-INVENTORY-001"
			role:      "step"
			statement: "READ the live `skills/` inventory (this repo). A direction an existing skill already covers is an edit to that skill; otherwise a new skill."
			title:     "Read inventory"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-INVENTORY-002a"
			role:      "step"
			statement: "READ `~/.claude/context/global/presentation-profile.md` first. `audience: casual` → the `change` may target only a skill with `origin: user` frontmatter."
			title:     "Casual lockout read"
		}
	}]
	kind: "context"
}, {
	title: "Presentation"
	blocks: [{
		entry: {
			id:        "SKILL-SMINESKILLS-SETUP-005b"
			role:      "rule"
			statement: "`language:` set, not `en` → author `title`, `problem` and `fix` in that language, per the profile's register and glossary."
			title:     "Language of change"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-SETUP-005c"
			role:      "rule"
			statement: "Translation NEVER touches `id`, `kind`, `target`, `change`, `notes`, file paths, code, or ids; no profile = English, unchanged."
			title:     "Language field exclusions"
		}
	}, {
		entry: {
			id: "SKILL-SMINESKILLS-SETUP-006a"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Profile `audience:` = `casual` → `title`, `problem` and `fix` MUST carry no file paths, rule/FACT/ACDSL IDs, schema/taxonomy jargon."
			title:     "Casual presentation scope"
		}
	}, {
		entry: {
			id: "SKILL-SMINESKILLS-SETUP-006b"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Say what changes for the user; technical anchors go in `target`, `change` and `notes`."
			title:     "Casual presentation anchors"
		}
	}]
	kind: "context"
}, {
	title: "Plan"
	blocks: [{
		entry: {
			id: "SKILL-SMINESKILLS-PLAN-004"
			items: [{
				fence: {
					body: """
						{"id": "skills/<slug>", "kind": "skills", "title": "", "problem": "", "backlog": "", "evidence": [{"session": "", "quote": "", "finding": 0}], "fix": "", "sessions": [], "target": "", "change": "", "notes": ""}

						"""
					info:   "json"
					marker: "```"
				}
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`id` = `skills/<slug>`; `kind` = `skills`; `sessions` = the problem's evidence session ids; `target` = the `<leaf>.cue` or workflow file."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`problem` = one plain sentence ≤ 160 runes stating what is wrong for a reader who was not in the session."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "no ids, shas, paths, line numbers, code spans or parenthesised counts — the task check refuses them."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`backlog` = the matched `B-NNN`, omitted when the problem matched no backlog item."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`fix` = why this edit fixes it, ≤ 240 characters."
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
				text:   "a longer note cut at a clause boundary with a trailing `…`, never paraphrased."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`change` one imperative sentence."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`notes` = edit mechanics: `make skill-render SKILL=<leaf>`, version + newest changelog row bump, smoke impact, `make workflow-gen` for stage files."
			}]
			role:      "def"
			statement: "Change task: one JSON object keyed as the block below — smine-problems writes it to `.smine-run/changes/skills-<slug>.json`."
			title:     "Change task"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-PLAN-001"
			role:      "step"
			statement: "READ the problem and direction index from the prompt; READ the live target surface per the context entries above."
			title:     "Read problem"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-PLAN-002"
			role:      "step"
			statement: "COMPOSE the [change task](#skill-smineskills-plan-004) for the direction; a direction this kind cannot carry, or a knowledge change, yields `task: null` with the reason."
			title:     "Compose task"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESKILLS-PLAN-003"
			role:      "stop"
			statement: "RETURN `{task, reason}` as the final message ⇒ STOP: write nothing."
			title:     "Return"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "plan one artifact kind from a problem record against the live skill inventory"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "plan Problem: {\"id\":\"smoke-unrun-skill-edits\",\"title\":\"Skill edits ship without a run\",\"problem\":\"Skill edits in 6 of 10 sessions were deployed without running the skill once; two broke an enforce check on the next real run and cost a full retry.\",\"cost\":\"6 sessions, 1 repo, 30 turns, 1800 s\",\"backlog\":\"\",\"evidence\":[{\"session\":\"00000000-0000-0000-0000-000000000000\",\"note\":\"enforce check failed on the first real run\",\"finding\":0}],\"directions\":[{\"title\":\"Smoke before deploy\",\"direction\":\"Run make skill-smoke for every skill whose <leaf>.cue changed before the skill sync deploys it.\",\"feasibility\":\"make skill-smoke exists\",\"tasteFit\":\"mechanical verdict, no model judge\"}]}. Direction index: 0."
}
