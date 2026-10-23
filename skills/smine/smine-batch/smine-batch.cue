changelog: [{
	date:    "2026-10-21"
	version: "4.5.1"
	text:    "The Workflow criteria point at docs/skills.html."
	tokens:  10174
}, {
	date:    "2026-10-11"
	version: "4.5.0"
	text:    "Miners read the deployed SKILL.md at the skillDir the mine workflow receives and follow its Analyze substeps; the root-only Skill call and the miner arg are gone."
}, {
	date:    "2026-10-09"
	version: "4.4.0"
	text:    "Record format 2: one headline per finding, quotes as {text, turn}, summary ≤ 400 runes; make sessions ARGS=\"check <path>\" refuses a record without format 2 or with an unreadable headline."
	tokens:  10195
}, {
	date:    "2026-10-09"
	version: "4.3.0"
	text:    "Runtime placement declared: root-only (Workflow stages)."
	tokens:  9823
}, {
	date:    "2026-10-07"
	version: "4.2.0"
	text:    "The miner writes the record without context; make sessions ARGS=\"check <path> --context <transcript>\" merges the used-context record, so nothing is pasted and no scratch file is left."
}, {
	date:    "2026-10-04"
	version: "4.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  9771
}, {
	date:    "2026-10-06"
	version: "4.0.1"
	text:    "The pending list is newest first: a capped run mines the freshest sessions, older ones age out of the 14-day window."
	tokens:  9771
}, {
	date:    "2026-10-04"
	version: "4.0.0"
	text:    "The miner writes the session record as JSON, sessions/<repo>/<YYYY-Www>/<id>.json, and checks it with make sessions ARGS=\"check <path>\"; the markdown record and its render step are gone."
	tokens:  9774
}, {
	date:    "2026-10-04"
	version: "3.4.7"
	text:    "Pipeline drops the from-edge of the deleted smine-harness skill."
	tokens:  9552
}, {
	date:    "2026-10-01"
	version: "3.4.6"
	text:    "Pipeline declares from /smine — the dispatcher that invokes this stage."
	tokens:  9566
}, {
	date:    "2026-09-30"
	version: "3.4.5"
	text:    "make context-record runs the Go extractor (cmd/sessions context-record): per-agent agents[] with measured items (chars, lines, tokens) and totals beside the unchanged id fields; schema mirrors them as optional"
	tokens:  9569
}, {
	date:    "2026-09-29"
	version: "3.4.4"
	text:    "workflow spawns now name a model through the generated GEN:models vocabulary; absent model ⇒ default subagent model"
}, {
	date:    "2026-09-28"
	version: "3.4.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.4.2"
	text:    "Permission commands: render redacts secret spans to [REDACTED:<kind>] in the record md and JSON."
}, {
	date:    "2026-09-27"
	version: "3.4.1"
	text:    "Declared Delegation: never — Workflow fan-out, root session only."
}, {
	date:    "2026-09-27"
	version: "3.4.0"
	text:    "One markdown record per session at sessions/<repo>/<YYYY-Www>/<id>.md, JSON derived by make sessions render; pending list from Go, oldest first, --limit; ledgers, batches, arcs, writer route and the JSON section retired; unknown replaces default; record prose follows the profile language."
}, {
	date:    "2026-09-26"
	version: "3.3.1"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "Analyze runs as Workflow mine.js: one smine-miner per session writes its record, one smine-batch-writer per batch writes md + JSON, root appends ledgers; transcript presence checked at Select; peek tools on mcp__peek-mcp__ and session_get."
}, {
	date:    "2026-09-25"
	version: "3.2.1"
	text:    "Make-only entry points: allowed-tools and prose call make context-record instead of the context_record.sh script."
}, {
	date:    "2026-09-24"
	version: "3.2.0"
	text:    "Per-session cost block from peek session_events in the batch JSON (schema + emit step)."
}, {
	date:    "2026-09-24"
	version: "3.1.2"
	text:    "memory-candidate filter names AGENTS.md — CLAUDE.md retired"
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind lines on Skill vs Workflow and Rules (context), floating workflow-candidate-spec list folded into a payload entry, bare bullets promoted to param register, pred/post clauses on 14 flow entries plus mode=interactive on the review stop, new atoms.json (13 local bool atoms) and three queries under queries/."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-13"
	version: "2.0.1"
	text:    "Register sweep — the --repos bullet split to one claim per bullet"
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "format-v2 reformat: When to use -> Routing (Purpose/Disambiguation table/Preconditions/Pipeline), Args as a 4-column table with links into owning entries, argument-hint rewritten as one full invocation, allowed-tools reordered — content identical, deep semantics moved into SELECT-001/SETUP-003a/SELECT-005/REPORT-016/SUBAGENT-001 as sub-bullets"
}, {
	date:    "2026-09-05"
	version: "1.30"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.29"
	text:    "two blind-green guards — step 2 silent-empty/swallowed-result lens (a session that completed with an empty or swallowed result is a report-card/harness-friction finding, never dropped as no-signal); step 3 parser-health canary (a pass with zero findings/frustration/permissions across non-curated sessions is a suspect parse: PARSER HEALTH warning + ledger append withheld so the sessions re-mine)"
}, {
	date:    "2026-08-30"
	version: "1.28"
	text:    "permissions capture (peek-mcp v1.2.2): per-session permissions block in the batch JSON (granted/denied tool+command pairs from session_events + telemetry permissions view) feeding the new smine-permissions dimension, plus a #### Permissions report subsection; new --subagents flag mines per-subagent transcripts via the subagent parameter (off by default, one extra paginated pull per subagent)"
}, {
	date:    "2026-08-27"
	version: "1.27"
	text:    "self-mining exclusion (default-deny): without --dev, Select drops sessions whose cwd is inside the run's repo root (incl. .claude/worktrees/) and sessions on claude-routines/* branches, reported as one count line; new --last n keeps only the n newest qualifying sessions (silent drop) and --dev re-includes the pipeline's own sessions"
}, {
	date:    "2026-08-27"
	version: "1.26"
	text:    "presentation-profile language contract — batch JSON prose fields authored in the profile language, md report stays English"
}, {
	date:    "2026-08-27"
	version: "1.25"
	text:    "added --since <YYYY-MM-DD> (date floor on session last_active at Select); retired --scopes — output routing derives from --repos attribution: one folder per roster repo, sessions/default/ for unmatched, archived/+default reserved, per-folder batches and ledgers with a union skip-list across live and archived folders"
}, {
	date:    "2026-08-23"
	version: "1.24"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.23"
	text:    "Retargeted doc pointers to README.md § Skill map and skills/README.md (docs reorg)"
}, {
	date:    "2026-08-20"
	version: "1.22"
	text:    "description compressed under the brevity caps"
}, {
	date:    "2026-08-18"
	version: "1.21"
	text:    "Downstream note lists the four fan-out dimensions (workflows dimension retired)"
}, {
	date:    "2026-08-18"
	version: "1.20"
	text:    "--agents (Codex mined by default), used-context record via cmd/context/context_record.sh + \"Context used:\" report line, context-use finding dimension, batch schema bump (sessions[].agent, sessions[].context, findings[].contextUse)"
}, {
	date:    "2026-08-12"
	version: "1.19"
	text:    "dynamic scopes — --scopes arg (restrict to named scope dirs, default all under sessions/ except proposals/); schema: .batch.scope is the scope directory name (org-name values like aqms retired)"
}, {
	date:    "2026-08-11"
	version: "1.18"
	text:    "absorbs smine-summary — §4 emits the batch's schema-conformant JSON (reference/schema.json moved here) in the same run as the md, jq-validated; smine-summary retired, JSON is no longer a fan-out dimension"
}, {
	date:    "2026-08-09"
	version: "1.17"
	text:    "--repos roster arg — mechanical repo attribution by cwd prefix match, [external] tag outside the roster, external sessions still mined"
}, {
	date:    "2026-08-09"
	version: "1.16"
	text:    "identity-grammar wording (ACTION activity chapters)"
}, {
	date:    "2026-08-09"
	version: "1.15"
	text:    "session_events as pre-read signal (Signals line) + acdsl verdict-log join by branch (Gate verdicts line)"
}, {
	date:    "2026-07-31"
	version: "1.14"
	text:    "activity-scoped context — enforcement-surface list names style/ guides and rules/ activity chapters"
}, {
	date:    "2026-07-30"
	version: "1.13"
	text:    "context redesign — enforcement-surface list names context/rules entries and fdesign assets"
}, {
	date:    "2026-07-30"
	version: "1.12"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-27"
	version: "1.11"
	text:    "renamed smine → smine-batch; now stage 1 of the /smine pipeline (fronted by skills/smine/smine); Analyzer header line updated (Analyzer: smine-batch v<version>); enable-model-invocation moved to the front skill; triggers narrowed to avoid colliding with /smine; effort token normalized (large → high)"
}, {
	date:    "2026-07-26"
	version: "1.10"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "1.9"
	text:    "renamed session-analyze → smine (leaf dir, frontmatter name, H1); live references swept repo-wide"
}, {
	date:    "2026-07-19"
	version: "1.8"
	text:    "batch header carries the producing skill version (Analyzer: session-analyze v<version>)"
}, {
	date:    "2026-07-19"
	version: "1.7"
	text:    "reference rename: eval-skill → couchskill-eval; moved under skills/session/"
}, {
	date:    "2026-07-19"
	version: "1.6"
	text:    "workflow script location updated to skill-bundled convention (skills/<skill>/workflows/), .claude/workflows/ reference removed"
}, {
	date:    "2026-07-19"
	version: "1.5"
	text:    "per-point evidence attribution (1-3 ranked session ids) + verbatim violation/fix snippets in reports"
}, {
	date:    "2026-07-16"
	version: "1.4"
	text:    "--nightly unattended mode (all pending batches, no review STOPs) + enable-model-invocation for headless claude -p expansion"
}, {
	date:    "2026-07-15"
	version: "1.3"
	text:    "Positive index (green counterpart of the frustration index)"
}, {
	date:    "2026-07-13"
	version: "1.2"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-11"
	version: "1.1"
	text:    "explicit skill and workflow candidate extraction"
}, {
	date:    "2026-07-03"
	version: "1.0"
	text:    "initial version"
}]
enableDelegation: "never"
frontmatter: [{
	key:   "name"
	value: " smine-batch"
}, {
	key:   "description"
	value: " Mine idle sessions newest first into one JSON record each, sessions/<repo>/<YYYY-Www>/<id>.json, checked by Go — the mining stage of the /smine pipeline. Trigger on /smine-batch or \"mine session transcripts\". Args — --nightly: no review stop; --limit: sessions this run; --agents; --dev: include smine/routine sessions; --subagents: also mine subagent transcripts (expensive)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.5.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"--nightly --limit 30 --agents claude,codex --dev --subagents\""
}, {
	key:   "allowed-tools"
	value: " Edit, Read, ToolSearch, Workflow, Write, Bash(jq *), Bash(ls *), mcp__peek-mcp__session_events, mcp__peek-mcp__session_get"
}]
preamble: """
	# smine-batch

	Mine idle sessions, newest first, into one JSON record per session — stage 1 of the /smine pipeline.

	The record is the evidence: smine-problems and the session-overview server read it as-is, so it must be
	self-contained. `make sessions ARGS="check <path>"` validates it and rewrites it canonically.
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
					kind:  "to"
					skill: "smine-problems"
				}]
				note: "writes the session record JSON"
			}
			preconditions: ["peek-mcp installed (`make sessions ARGS=\"pending\"` spawns it; miners use `session_events`, `session_get`)", "root session with the Workflow tool (the miner route runs inside the mine workflow)"]
			purpose: "mines past sessions for actionable items — skill candidates, workflow patterns, memory updates, rule drift, harness friction — into one record per session."
			routes: [{
				for: "whole retrospective (mine + synthesize)"
				use: "/smine"
			}, {
				for: "quick-peek the latest session"
				use: "/peek"
			}, {
				for: "synthesizing problems from existing records"
				use: "/smine-problems"
			}, {
				for: "evaluating a specific skill's runs"
				use: "/skillroutine-eval"
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
				effect:  "no review stop — [Review stop](#skill-sminebatch-report-016a)"
				form:    "flag"
			}, {
				name:    "--limit"
				default: "`5`"
				effect:  "sessions mined this run, newest first — [Pending list](#skill-sminebatch-select-010)"
				form:    "`<n>`"
			}, {
				name:    "--agents"
				default: "both"
				effect:  "agents listed — [Pending list](#skill-sminebatch-select-010)"
				form:    "`<claude,codex>`"
			}, {
				name:    "--dev"
				default: "absent"
				effect:  "include the pipeline's own sessions — [Pending list](#skill-sminebatch-select-010)"
				form:    "flag"
			}, {
				name:    "--subagents"
				default: "absent (off)"
				effect:  "also mine per-subagent transcripts — [Default off](#skill-sminebatch-subagent-001)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "0. Setup"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-001c"
			role:      "def"
			statement: "Data source: peek-mcp — `mcp__peek-mcp__session_events`, `mcp__peek-mcp__session_get` (paginate via `request_id` until `has_more=false`); the session list comes from the pending list."
			title:     "Data source"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-002b"
			role:      "def"
			statement: "Transcript path = `~/.claude/projects/<slug>/<id>.jsonl` where `<slug>` = session cwd (pending entry) with `/` and `.` replaced by `-`."
			title:     "Used-context record path"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-010"
			role:      "def"
			statement: "Record: `sessions/<repo>/<YYYY-Www>/<session-id>.json`, repo a roster name or `unknown`, week the ISO week of the session's last activity; the only file per session."
			title:     "Record layout"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-009a"
			role:      "def"
			statement: "Gate verdicts: `~/.claude/acdsl/verdicts.jsonl` (JSONL, best-effort — absent file means no gate data, never a stop)."
			title:     "Gate verdicts source"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-009b"
			role:      "def"
			statement: "Join key is `branch` == the session's git branch from the pending entry (`session` is unpopulated); extract with `jq`."
			title:     "Gate verdicts key"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-006a"
			role:      "rule"
			statement: "Never translate: ids, dates, tags, file paths, code, quotes (verbatim evidence), JSON keys and enum values."
			title:     "Language exclusions"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-008b"
			role:      "rule"
			statement: "A session with a record in any folder, live or archived, never re-mines — the pending list already leaves it out."
			title:     "Mined once"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-005a"
			role:      "step"
			statement: "READ `~/.claude/context/global/presentation-profile.md` before writing records; when its `language:` is set and not `en`, author the record's user-visible prose —"
			title:     "Language profile read"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SETUP-005b"
			role:      "step"
			statement: "AUTHOR `title` and `signal`, finding summaries and quote triggers — in that language, following the profile body's register and glossary; absent profile = English."
			title:     "Language fields covered"
		}
	}]
}, {
	title: "1. Select"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEBATCH-SELECT-010"
			role:      "step"
			statement: "RUN `make sessions ARGS=\"pending --agents <agents> --limit <n>[ --dev]\"`; its JSON array is the run's sessions, newest first, each with `path`."
			title:     "Pending list"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SELECT-011"
			role:      "stop"
			statement: "If the list is empty ⇒ STOP: report \"nothing pending in peek's window\"."
			title:     "Nothing pending"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SELECT-012"
			role:      "stop"
			statement: "If `make sessions` exits non-zero ⇒ STOP: report its stderr as the stage failure."
			title:     "Selection failed"
		}
	}]
}, {
	title: "2. Analyze"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-002"
			role:      "def"
			statement: "Events are the cheap structured signal that steers the transcript read: high denial/rejection counts flag friction before a single turn is read."
			title:     "Events flag friction"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-003b"
			role:      "def"
			statement: "Every rule violation seen while reading is checked against the record: was the rule present, through which channel."
			title:     "Context record use"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-007b"
			role:      "def"
			statement: "Workflow candidates are loop-until-dry patterns — runnable unattended."
			title:     "Workflow candidate runnability"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-012b"
			role:      "def"
			statement: "Session evidence contradicting checked-in docs — quote the contradiction."
			title:     "Doc drift evidence"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-013b"
			role:      "def"
			statement: "Output is settings/hook/tooling edits, not memory."
			title:     "Harness friction output"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-017b"
			role:      "def"
			statement: "Examples: analysis batches, worktree cleanup, settings hygiene, ledger upkeep."
			title:     "Routine candidate examples"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-018"
			role:      "def"
			statement: "Routine candidate cadence: time-driven, not on-demand — a routine typically wraps a Workflow or skill in a schedule."
			title:     "Routine candidate cadence"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-020b"
			role:      "def"
			statement: "This is a report-card / harness-friction finding."
			title:     "Silent-empty finding class"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-021"
			role:      "def"
			statement: "A run that *completed* with an empty or swallowed result is first-class signal, not absence of signal."
			title:     "Completed empty signal"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-023b"
			role:      "def"
			statement: "The green counterpart of the frustration index; session-level user sentiment, distinct from exemplar validation (pattern-level)."
			title:     "Positive index nature"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-024b"
			role:      "def"
			statement: "Counts from the record's `context` object."
			title:     "Context line source"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-031"
			role:      "rule"
			statement: "Sessions on unlogged branches have no line — never reconstruct."
			title:     "Unlogged branches"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-034"
			role:      "rule"
			statement: "Miners MUST NOT spawn agents; an oversized transcript is paged, never delegated."
			title:     "No nested spawn"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-016a"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Denials MUST be treated as negative evidence only — never surfaced as a deny/ask candidate."
			title:     "Denials negative only"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-035"
			items: [{
				label:  "Script"
				marker: "*"
				text:   "`workflows/mine.js`"
			}, {
				label:  "param"
				marker: "*"
				text:   "`sessions` = the pending list verbatim."
			}, {
				label:  "param"
				marker: "*"
				text:   "`skillDir` = the skill's base dir; miners read their route from its SKILL.md."
			}]
			role:      "spawn"
			statement: "SPAWN the Workflow tool, scriptPath `<base dir>/workflows/mine.js`, args `{sessions, subagents, skillDir}`; await its notification."
			title:     "Spawn mine workflow"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-001a"
			role:      "substep"
			statement: "CALL `session_events` for the session (with `agent`) — typed stream: permission denials/grants, mode changes, plan rejections/revisions, skill invocations, subagent results, user answers"
			title:     "Events stream first"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-001b"
			role:      "substep"
			statement: "READ counters, usage, and a telemetry-derived `permissions` block (auto-allowed vs prompted-once vs prompted-always vs rejected, with the prompted commands)"
			title:     "Telemetry block"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-003a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "no transcript ⇒ no `context` key, `dataGaps` gains `context: transcript missing`."
			}, {
				label:  "param"
				marker: "*"
				text:   "Codex sessions have no record."
			}]
			role:      "substep"
			statement: "For a Claude session with a transcript: OMIT `context` from the written record; the check step merges it."
			title:     "Context record"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-004"
			pred:      "peek_loaded"
			role:      "substep"
			statement: "READ the FULL transcript via `session_get`, paginating to `has_more=false`. Extract per session:"
			title:     "Full transcript read"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-005"
			role:      "substep"
			statement: "EXTRACT skill candidates: recurring procedure whose value is judgment and convention — phases, review lenses, question protocols — one agent conversing with the user."
			title:     "Skill candidates"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "check the skill inventory first: an instance of an existing skill is report-card input, not a candidate."
			}]
			role:      "substep"
			statement: "COUNT validated dispatch/prompt templates (frozen-plan implement prompt, task-chip format with repro + test list, consolidation merge instruction)."
			title:     "Skill candidate templates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-007a"
			role:      "substep"
			statement: "EXTRACT workflow candidates: recurring orchestration whose value is deterministic control flow — fan-out over an enumerable item list, staged find→verify→consolidate pipelines."
			title:     "Workflow candidates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-008"
			role:      "substep"
			statement: "COUNT multi-session fan-outs the user drove by hand (parallel worktree reviews, bake-offs, consolidation passes) as evidence."
			title:     "Workflow evidence source"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-009"
			role:      "substep"
			statement: "GRADE the invocation when a session invokes a known skill: stale paths, phase gaps, protocol misses → skill-improvement candidates."
			title:     "Existing-skill report card"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-010"
			role:      "substep"
			statement: "EXTRACT feature-design signal: what the agent built wrong vs. what the user corrected it to. Reconstruct the before/after concretely enough to generalize."
			title:     "Feature-design signal"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-011"
			role:      "substep"
			statement: "EXTRACT memory candidates: things the agent should have known. Generalize; skip repo trivia that AGENTS.md already covers."
			title:     "Memory candidates"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-012a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "context/rules `RULE-*` guides incl. plan.md, context/actions `ACTION` activity chapters incl. reviewing.md, AGENTS.md."
			}]
			role:      "substep"
			statement: "EXTRACT repo-surface rules + doc drift: corrections that belong in enforcement surfaces; name the target surface."
			title:     "Doc drift"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-013a"
			role:      "substep"
			statement: "EXTRACT harness/config friction: recurring permission prompts → allowlist candidates; hook opportunities; worktree/infra defects; MCP capability gaps."
			title:     "Harness/config friction"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-014a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "always captured, feeds the record's `permissions` object for smine-permissions."
			}]
			role:      "substep"
			statement: "RECORD per session `{tool, command, decision}` from the `session_events` stream and telemetry `permissions` view."
			title:     "Permissions granted capture"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-014b"
			role:      "substep"
			statement: "for each **prompted-and-granted** tool/command (`prompted_once`/`prompted_always` from the telemetry view, else `granted_event` from a `permission_granted` event)"
			title:     "Granted decision source"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-015"
			role:      "substep"
			statement: "RECORD `{tool, command}` for each **denied** one."
			title:     "Permissions denied capture"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-016"
			items: [{
				label:  "param"
				marker: "*"
				text:   "this is capture, not proposal-authoring (smine-permissions ranks them)."
			}, {
				label:  "param"
				marker: "*"
				text:   "write commands as captured; check replaces secret spans with `[REDACTED:<kind>]` — never hand-redact."
			}]
			role:      "substep"
			statement: "WRITE the `permissions` object with the granted and denied pairs; omit it when the session had no permission activity."
			title:     "Permissions record object"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-017a"
			role:      "substep"
			statement: "EXTRACT nightly routine candidates: recurring maintenance the user triggers manually on a cadence with no decisions in the loop → scheduled local routine."
			title:     "Nightly routine candidates"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-019"
			role:      "substep"
			statement: "FLAG exemplar validation: friction-free sessions that confirm a pattern or skill works, so working patterns don't read as absence of signal."
			title:     "Exemplar validation"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-020a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "never dropped as a no-signal session."
			}]
			role:      "substep"
			statement: "FLAG silent-empty / swallowed-result: a session whose invoked skill or subagent fan-out returned empty, swallowed an error into a clean envelope, or produced no artifact despite normal counters."
			title:     "Silent-empty finding"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-022"
			role:      "substep"
			statement: "QUOTE frustration index verbatim, name the trigger: cursing and corrections are a 100% interest flag."
			title:     "Frustration index"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-023a"
			role:      "substep"
			statement: "QUOTE positive index verbatim, name the trigger: explicit praise or expressed satisfaction."
			title:     "Positive index"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-ANALYZE-026a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "`violated-despite-present` = violation + rule present via channel N."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "`needed-not-present` = violation + rule absent everywhere; name the skill/language that should carry it."
			}]
			role:      "substep"
			statement: "EXTRACT context-use findings: `violated-despite-present` or `needed-not-present`."
			title:     "Context-use kinds"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-026b"
			role:      "substep"
			statement: "`present-irrelevant` (entry present, no touchpoint in the session's task — judgment, quoted)."
			title:     "Context-use irrelevant kind"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-027"
			role:      "substep"
			statement: "NAME channel, rule/entry ID, evidence quotes for each finding. A rule present and followed where it applied is `honored` (counter, no finding)."
			title:     "Context-use honored counter"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-029"
			role:      "substep"
			statement: "APPEND `DATA GAP: <peek unsupported list>` to a Codex session's `dataGaps` (touched files, skill breakdown, hook context) — a gap is stated, never a skip."
			title:     "Codex data gap"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-030a"
			role:      "substep"
			statement: "FILTER the verdict log to the session's branch (`jq 'select(.branch == \"<branch>\")'` over `~/.claude/acdsl/verdicts.jsonl`)."
			title:     "Gate verdicts"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-030b"
			role:      "substep"
			statement: "REPORT red runs, the violated rule IDs, and reds-to-green convergence (retries-to-green evidence) as a finding."
			title:     "Gate verdicts report"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-014a"
			role:      "verify"
			statement: "Before writing: VERIFY a non-trivial session yields at least one finding; else RETURN `failed` with note `no signal — suspect parse`, write nothing, so it re-mines next run."
			title:     "Parser-health canary"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-037"
			role:      "substep"
			statement: "WRITE the session record JSON at `path` per [Session record payload](#skill-sminebatch-tpl-001), identity and cost from the pending entry and session_events."
			title:     "Write record"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-038"
			role:      "substep"
			statement: "RUN `make sessions ARGS=\"check <path> --context <transcript>\"`, `--context` only when [Used-context record path](#skill-sminebatch-setup-002b) exists; FIX each violation, re-run until `ok`."
			title:     "Check record"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-ANALYZE-039"
			role:      "substep"
			statement: "RETURN `{id, path, failed, skipped, dataGaps}`; `failed` when no checked record exists."
			title:     "Miner return"
		}
	}, {
		heading: "Subagent mining (opt-in, `--subagents`)"
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-SUBAGENT-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "passed through from `/smine --subagents` (nightly: `SMINE_SUBAGENTS=1`)."
			}]
			role:      "def"
			statement: "Off by default: without the flag, only the top-level transcript and the subagent spawn/result events + counts are mined (current behavior)."
			title:     "Default off"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SUBAGENT-004"
			role:      "def"
			statement: "Cost: one extra paginated pull per subagent — this is why it is opt-in. Only run it when explicitly enabled."
			title:     "Opt-in cost"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SUBAGENT-002a"
			pred:      "subagents_flag"
			role:      "substep"
			statement: "With `--subagents`: after the top-level pass, ITERATE the advertised subagent ids (always in the `session_events`/ `session_get` responses)."
			title:     "Per-subagent pull"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SUBAGENT-002b"
			role:      "substep"
			statement: "PULL each with `subagent: <id>` on `session_get` (turns) and `session_events` (events)."
			title:     "Per-subagent pull calls"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SUBAGENT-003"
			role:      "substep"
			statement: "ATTRIBUTE subagent-scope findings as `origin: subagent <id>` in the summary; their permission pairs merge into the record's `permissions`."
			title:     "Subagent attribution"
		}
	}]
}, {
	title: "Skill vs Workflow"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEBATCH-SKILLWF-001"
			role:      "def"
			statement: "Workflow criteria: deterministic steps over an enumerable item list, unattended, N agents → Workflow (skill-bundled `skills/<skill>/workflows/<name>.js` script, see `docs/skills.html`)."
			title:     "Workflow criteria"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SKILLWF-002"
			role:      "def"
			statement: "Skill criteria: judgment/conventions, adaptive flow, user decisions mid-flow → Skill (`SKILL.md`)."
			title:     "Skill criteria"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-SKILLWF-003"
			role:      "def"
			statement: "Both is legal: this skill fronts the conversation, its Analyze runs as Workflow `mine.js`, one miner per session."
			title:     "Both is legal"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-SKILLWF-004"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "Workflow candidates are reported as a spec, never a full script:"
			title:     "Workflow candidate spec"
		}
	}]
	kind: "context"
}, {
	title: "3. Record"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-017a"
			role:      "def"
			statement: "smine-problems reads records without `synthesized`; the config server reads the same JSON."
			title:     "Downstream routing"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-003"
			role:      "substep"
			statement: "USE full queryable session IDs, always."
			title:     "Full session IDs"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-004"
			role:      "substep"
			statement: "WRITE `invoked_skills` — the invoked skill names; omit it when none."
			title:     "Skills invoked line"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-005"
			role:      "substep"
			statement: "CURATE: a no-signal session (ghost, trivial) gets `skipped: true`, a one-word `skipReason` and empty `findings`."
			title:     "Curate no-signal"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-007"
			role:      "substep"
			statement: "WRITE Skill and Workflow candidates as their own findings, dimension `skill-candidate` / `workflow-candidate`; a Workflow candidate's summary in the spec format above."
			title:     "Candidate findings"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-008"
			role:      "substep"
			statement: "GENERALIZE lessons; specifics appear only as evidence quotes."
			title:     "Generalize lessons"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-009"
			role:      "substep"
			statement: "SPELL OUT explicit evidence per finding: the verbatim quotes it rests on, one `quotes` item each."
			title:     "Per-point evidence"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-REPORT-009a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "No ids, shas, paths, line numbers, code spans or parenthesised counts."
			}]
			role:      "substep"
			statement: "WRITE one `headline` per finding: one plain sentence ≤ 120 runes stating what went wrong or worked, for a reader not in the session."
			title:     "Finding headline"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-009b"
			role:      "substep"
			statement: "RECORD each quote as `{text, turn}`, `turn` = `turns_total − len(page) + i + 1` of the `session_get` page holding it, 1-based."
			title:     "Quote turn"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-011a"
			role:      "substep"
			statement: "INCLUDE, when the transcript has the offending and/or corrected code (pasted code, diffs, plan excerpts), both verbatim as `snippets`, `kind` `violation` / `fix` / `context`."
			title:     "Verbatim code snippets"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-011b"
			role:      "substep"
			statement: "SET each snippet's `source` (`transcript` | `plan` | `diff`)."
			title:     "Snippet source note"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-012a"
			role:      "substep"
			statement: "FALL BACK to the plan text, best effort, for diffs of pruned worktrees — they are gone."
			title:     "Pruned worktree fallback"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-012b"
			pred:      "code_reconstructed"
			role:      "stop"
			statement: "Never reconstruct code the transcript does not show."
			title:     "Never reconstruct"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-REPORT-019"
			role:      "step"
			statement: "REPORT mined, skipped and failed counts with ids, and every record that failed its check."
			title:     "Run report"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-REPORT-016a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`--nightly` never stops for review; the run ends when the workflow returned."
			}]
			pred:      "mode = interactive"
			role:      "stop"
			statement: "If not `--nightly` ⇒ STOP for user review after the run."
			title:     "Review stop"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-TPL-001"
			payload: {
				info:   "json"
				marker: "```"
			}
			role:      "payload"
			statement: "Keys per the session schema; key order free — check rewrites the file canonically."
			title:     "Session record payload"
		}
	}, {
		entry: {
			id: "SKILL-SMINEBATCH-TPL-002"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Record grammar"
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEBATCH-RULES-001"
			role:      "rule"
			statement: "Session IDs are NEVER truncated."
			title:     "Full session IDs"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-RULES-002"
			role:      "rule"
			statement: "Artifacts injected elsewhere as-is MUST be self-contained — no session-local shorthand or invented codenames."
			title:     "Self-contained artifacts"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-RULES-005"
			role:      "rule"
			statement: "User cursing MUST be treated as signal, not noise — always dig in."
			title:     "Cursing is signal"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-RULES-003"
			pred:      "output_scratch_only"
			role:      "stop"
			statement: "If output lands anywhere but the pending `path` ⇒ STOP and fix: never `/tmp`, the scratchpad or a second file beside the record."
			title:     "Record path only"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEBATCH-RULES-004"
			pred:      "subagent_pending"
			role:      "stop"
			statement: "If the mine workflow has not sent its completion notification ⇒ STOP: wait, never treat its output as missing."
			title:     "Wait for workflow"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "mining long transcripts for non-obvious lessons"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "mines the user's live session transcripts"
}
