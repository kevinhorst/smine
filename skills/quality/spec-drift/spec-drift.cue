changelog: [{
	date:    "2026-10-17"
	version: "4.6.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  5188
}, {
	date:    "2026-10-09"
	version: "4.5.1"
	text:    "acdsl-context narrowed from RULE-PLAN-* to the plan-dir naming and resolution entries — the scope step's only plan-rule touchpoint."
}, {
	date:    "2026-10-04"
	version: "4.5.0"
	text:    "mining steps and routes tagged with needs: partial installs and the bundle drop them."
	tokens:  5359
}, {
	date:    "2026-10-04"
	version: "4.4.3"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  5277
}, {
	date:    "2026-10-02"
	version: "4.4.2"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  5359
}, {
	date:    "2026-09-28"
	version: "4.4.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  5360
}, {
	date:    "2026-09-26"
	version: "4.4.0"
	text:    "Route sections declare their anchor: Doc mode and Contract mode between MODE-002..end (alternatives), Nightly route between DOCREPORT-001b..DOCREPORT-002 (the report file write); route lines stay in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "4.3.0"
	text:    "--nightly: report written to reports/spec-drift/<date>.md with its Summary as the routine commit body; the durable doc set never includes reports/; wrapper def names the weekly spec-drift routine"
}, {
	date:    "2026-09-26"
	version: "4.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-15"
	version: "4.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "4.1.0"
	text:    "Formal-schema reformat: section kinds (route on doc/contract mode, context on self-check gate and nightly wrapper); pred/post clauses on 11 entries backed by new atoms.json (run_mode enum plus 9 bool atoms); three queries; freeform enumeration bullets converted to * param: register; scope-default entry's doc-set enumeration replaced by a plain reference to the skill's own discovery logic; caveman-compressed."
}, {
	date:    "2026-09-14"
	version: "4.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "3.0.0"
	text:    "Format-v2 reformat: When to use migrated to Routing (Purpose/Disambiguation/Preconditions/Pipeline); Args migrated to the 4-column table with argument-hint; consumer-language enumeration fenced; content identical."
}, {
	date:    "2026-09-05"
	version: "2.13"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-01"
	version: "2.12"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "2.11"
	text:    "Doc-mode scope wording migrated to the dated plan-dir convention (RULE-PLAN-072)"
}, {
	date:    "2026-08-23"
	version: "2.10"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "2.9"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-18"
	version: "2.8"
	text:    "reference rename — fchange → fdesign change (fchange retired into the fdesign change route)"
}, {
	date:    "2026-08-11"
	version: "2.7"
	text:    "reference rename — smine-style → smine-context"
}, {
	date:    "2026-07-30"
	version: "2.6"
	text:    "reference rename — smine-rules → smine-style"
}, {
	date:    "2026-07-30"
	version: "2.5"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-30"
	version: "2.4"
	text:    "moved under skills/quality/ group; name and behavior unchanged"
}, {
	date:    "2026-07-27"
	version: "2.3"
	text:    "reference rename — analyze-rules → smine-rules"
}, {
	date:    "2026-07-26"
	version: "2.2"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "2.1"
	text:    "reference merge: feature-refactor → feature-change"
}, {
	date:    "2026-07-23"
	version: "2.0"
	text:    "absorbed contract-drift-audit as read-only contract mode (consumer enumeration + classification + fix-order report, no fixing); mode selection at intake; effort token normalized"
}, {
	date:    "2026-07-19"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " spec-drift"
}, {
	key:   "description"
	value: " Report drift between a source of truth and the code, read-only, with per-claim evidence. Trigger on /spec-drift [scope] or \"does the doc still match the code\" or \"audit this contract change\". Args — scope: doc-mode target, absent means the repo's durable doc set; contract: changed identifier to audit in contract mode; cap: per-run verified-claim cap override; --nightly: dated report file."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.6.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, RULE-PLAN-071, RULE-PLAN-072, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"plans/2026-08-01-foo/design cap: 50 --nightly\""
}, {
	key:   "allowed-tools"
	value: " Glob, Grep, Read, Bash(git log *), Bash(grep *)"
}]
preamble: """
	# Spec drift detector

	Produces one artifact: drift report, source of truth vs code, one checkable claim at a time, each verdict backed by code
	read.

	Read-only every mode — never edits docs or code. Two modes share that contract: **doc mode** diffs spec/design doc set vs
	current code; **contract mode** audits every consumer of one changed shared contract, reports which broken, with
	recommended fix order. Fixing always separate follow-up session, working from report.

	Verification grounded in real code reads (grep-to-zero discipline), never memory of codebase. Claim not "verified" til
	implementing code — or its absence — on screen with file:line.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "from"
					skill:    "diagnose-debug"
					optional: true
				}]
				note:       "every finding hands off to a follow-up fix session"
				standalone: true
			}
			preconditions: ["doc mode: doc set to audit (durable doc, `plans/` concept/design dir, or `$AGENT_CONTEXT_DIR_DEFAULT/*` doc) and code it describes, both readable in current worktree", "contract mode: changed contract (or rename/move rule) known, identifiable by searchable form"]
			purpose: "checks whether spec/design doc set still matches code (doc mode), or enumerates and classifies every consumer of one changed shared contract (contract mode); output always drift report, never fix."
			routes: [{
				for: "the failing contract not yet known"
				use: "/diagnose-debug first (hands off here once identified)"
			}, {
				for: "a restructuring plan"
				use: "/fdesign"
			}, {
				for: "mining session batch reports for rule/doc updates"
				use: "/smine-context"
				needs: ["mining"]
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "scope"
				default: "repo's durable doc set"
				effect:  "doc-mode target — doc, feature dir, or plan-dir tree — see [Scope resolution](#skill-specdrift-docintake-001a)"
				form:    "free text, doc mode positional"
			}, {
				name:    "contract"
				default: "mode picked by subject"
				effect:  "contract-mode subject — changed identifier to audit — see [Decide by subject](#skill-specdrift-mode-001)"
				form:    "free text, contract mode positional"
			}, {
				name:    "cap"
				default: "config default"
				effect:  "per-run verified-claim cap; overflow reported `unchecked` — see [Per-run cap](#skill-specdrift-docintake-003)"
				form:    "`cap: <n>`"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: report to `reports/spec-drift/<date>.md`, never chat-only — see [Nightly report](#skill-specdrift-nightly-004)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Mode selection"
	blocks: [{
		entry: {
			id:        "SKILL-SPECDRIFT-MODE-003"
			pred:      "not escalated_in_run"
			role:      "rule"
			statement: "Doc-mode finding that turns out contract change MUST become new contract-mode run, not in-run escalation."
			title:     "New run escalation"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-MODE-001"
			post: ["mode_decided"]
			role:      "step"
			statement: "DECIDE mode at intake by subject: doc set → doc mode; one changed contract → contract mode."
			title:     "Decide by subject"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-MODE-002"
			pred:      "run_mode = contract and multi_contract_scope"
			role:      "stop"
			statement: "If contract-mode scope covers more than one contract ⇒ STOP: split into separate runs — folding two audits into one loses negative grep's meaning."
			title:     "One contract"
		}
	}]
	kind: "phase"
}, {
	title:   "Doc mode"
	between: "MODE-002..end"
	blocks: [{
		heading: "0. Intake"
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-DOCINTAKE-001a"
			role:      "step"
			statement: "RESOLVE scope: arg is doc set — one doc, feature dir, or plan dir tree (plan-dir resolution rule)."
			title:     "Scope resolution"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-DOCINTAKE-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "never `reports/` — the skill's own output."
			}]
			role:      "step"
			statement: "No arg → repo's durable doc set, per skill's own discovery logic."
			title:     "Scope default"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-DOCINTAKE-002"
			role:      "step"
			statement: "FIX baseline: current worktree HEAD is code side; docs read as they sit now. State both so report reproducible."
			title:     "Fix the baseline"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-DOCINTAKE-003"
			role:      "step"
			statement: "APPLY per-run cap on verified claims (default from config, overridable by arg) so nightly run terminates. Report overflow as `unchecked`, never silently dropped."
			title:     "Per-run cap"
		}
	}, {
		heading: "1. Claim extraction"
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-EXTRACT-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "Named routes, methods, functions, symbols."
			}, {
				label:  "param"
				marker: "*"
				text:   "Model fields and types."
			}, {
				label:  "param"
				marker: "*"
				text:   "File paths and directory layout."
			}, {
				label:  "param"
				marker: "*"
				text:   "Config keys, env vars, feature flags."
			}, {
				label:  "param"
				marker: "*"
				text:   "Limits, thresholds, timeouts, caps."
			}, {
				label:  "param"
				marker: "*"
				text:   "Described behaviors naming concrete code location."
			}]
			role:      "step"
			statement: "PARSE each doc into checkable claims — claim is statement with code anchor that can pass or fail against code:"
			title:     "Checkable claim"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-EXTRACT-002"
			role:      "step"
			statement: "LIST prose claims with no code anchor once under **unverifiable** — never extract as checkable, never guess."
			title:     "Unverifiable prose"
		}
	}, {
		heading: "2. Verification"
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-VERIFY-002"
			role:      "def"
			statement: "**pass** — verdict when claim holds against code."
			title:     "Pass"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-VERIFY-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "**doc-stale** — code moved on deliberately; superseding decision exists. Check `decisions.md` / git history, cite it."
			}, {
				label:  "param"
				marker: "*"
				text:   "**code-wrong** — doc is agreed contract, code diverged from it."
			}, {
				label:  "param"
				marker: "*"
				text:   "**undecidable** — differential real but which side authoritative needs owner."
			}]
			role:      "def"
			statement: "**drift** — verdict when code differs from claim, classified:"
			title:     "Drift classification"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-VERIFY-004"
			role:      "def"
			statement: "**dead reference** — verdict when referenced code not found at all."
			title:     "Dead reference"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-VERIFY-005"
			role:      "def"
			statement: "**unchecked** — verdict when claim past per-run cap — extracted but not verified."
			title:     "Unchecked"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-VERIFY-001"
			role:      "step"
			statement: "LOCATE implementing code by grep / symbol search for each extracted claim, compare against claim. Every verdict carries code file:line (or MISSING) — no verdict from memory."
			title:     "Locate and compare"
		}
	}, {
		heading: "3. Report"
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-DOCREPORT-001a"
			post: ["report_written"]
			role:      "step"
			statement: "WRITE per-claim table, numbered so every item addressable in follow-up session."
			title:     "Per-claim table"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-DOCREPORT-001b"
			needs: ["mining"]
			role:      "step"
			statement: "REUSE `smine-context` evidence discipline — verbatim excerpts, real anchors, no reconstruction — rather than inventing second report shape."
			title:     "Reuse evidence discipline"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-DOCREPORT-002"
			pred:      "report_written"
			role:      "stop"
			statement: "If per-claim table written ⇒ STOP: report is deliverable, do not fix anything."
			title:     "Stop, no fix"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "The doc-mode drift report."
			title:     "Doc-mode report"
		}
	}]
	kind: "route"
	pred: "run_mode = doc"
}, {
	title:   "Contract mode"
	between: "MODE-002..end"
	blocks: [{
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-005b"
			pred:      "not fix_order_executed"
			role:      "rule"
			statement: "Order is plan for follow-up fix session — this run MUST NOT execute it."
			title:     "Order not executed"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-001a"
			role:      "step"
			statement: "INTAKE changed contract and its searchable identifier — JSON tag, route, HTTP method, query param, Redis key format, file path, config key, or method name."
			title:     "Intake"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-001b"
			role:      "step"
			statement: "Contract drifted (reactive, from breakage) or being changed (proactive rename/move rule)."
			title:     "Intake trigger"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-002a"
			role:      "step"
			statement: "FIND commit where contract drifted (`git log -S`/`-G` on identifier), read diff."
			title:     "Git archaeology (reactive)"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-002b"
			role:      "step"
			statement: "Commit diff isn't current state — re-read affected files from disk after, since later commits may have moved things again."
			title:     "Re-read after diff"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CONTRACT-003"
			items: [{
				fence: {
					body: """
						Go
						Python
						Svelte/TS
						Makefiles
						Dockerfiles
						fixtures
						seeds
						docs
						API collections

						"""
					info:   "text"
					marker: "```"
				}
			}, {
				label:  "param"
				marker: "*"
				text:   "Known miss class is relative-path trap — `cd ../x && ...` line without searched substring."
			}, {
				label:  "param"
				marker: "*"
				text:   "Search by consumer *kind* (every place that could build, encode or reference contract), never only by literal substring."
			}, {
				label:  "param"
				marker: "*"
				text:   "Cover derived identifier forms: snake_case / camelCase / kebab-case, URL-encoded forms, string-built keys."
			}, {
				label:  "param"
				marker: "*"
				text:   "Key assembled from fragments won't match whole literal."
			}]
			role:      "step"
			statement: "SEARCH repo-wide for contract across **all** languages and artifacts:"
			title:     "Consumer enumeration"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-004"
			role:      "step"
			statement: "DIFF each consumer's expectation against new reality, label it, each with file:line evidence: **broken** / **already-correct** / **intentionally-divergent** (with justification)."
			title:     "Classify"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CONTRACT-005a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "same numbering and evidence discipline as the doc-mode report."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "fix order is contract's source of truth first, then consumers in order, compiler may drive cascade."
			}]
			post: ["consumer_report_written"]
			role:      "step"
			statement: "WRITE per-consumer table plus **recommended fix order**."
			title:     "Report"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-006a"
			role:      "step"
			statement: "GREP repo-wide for old form, classify every surviving hit — named in report as broken or individually justified (\"stays because …\")."
			title:     "Close on form"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-006b"
			pred:      "unexplained_hit"
			role:      "stop"
			statement: "If unexplained hit remains ⇒ STOP: enumeration incomplete, report not done."
			title:     "Enumeration complete"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-CONTRACT-007"
			pred:      "consumer_report_written"
			role:      "stop"
			statement: "If per-consumer report written ⇒ STOP: report is deliverable, do not fix anything."
			title:     "Stop, no fix"
		}
	}, {
		heading: "Cross-system parity sub-mode"
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-PARITY-001a"
			role:      "step"
			statement: "ENUMERATE reference behaviors for port — Go port of Django behavior, say — classify each as match / divergence / acceptable-divergence."
			title:     "Enumerate reference behaviors"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-PARITY-001b"
			role:      "step"
			statement: "AUDIT construction and bootstrap paths, not just outputs: credential chains, env wiring, initialization order are where parity silently breaks. Report-only, like everything else here."
			title:     "Audit construction paths"
		}
	}]
	kind: "route"
	pred: "run_mode = contract"
}, {
	title: "Self-check gate"
	blocks: [{
		entry: {
			id:        "SKILL-SPECDRIFT-CHECK-006"
			pred:      "not file_edited"
			role:      "rule"
			statement: "Skill MUST NOT edit any doc or code file, in any mode."
			title:     "No edits"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CHECK-001"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Every non-unverifiable claim (doc mode) and every consumer (contract mode) MUST carry code file:line or explicit MISSING — no verdict rests on memory."
			title:     "File:line or MISSING"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CHECK-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Every drift MUST be classified doc-stale / code-wrong / undecidable; doc-stale MUST cite superseding decision (doc mode)."
			title:     "Drift classified"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CHECK-003"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Every consumer MUST be classified broken / already-correct / intentionally-divergent; every surviving grep hit for old form MUST be accounted for (contract mode)."
			title:     "Consumers classified"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CHECK-004"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Prose-only claims MUST sit under **unverifiable**, not silently dropped, not guessed at."
			title:     "Unverifiable not dropped"
		}
	}, {
		entry: {
			id: "SKILL-SPECDRIFT-CHECK-005"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Overflow past the cap MUST be reported as **unchecked**, not dropped."
			title:     "Overflow reported"
		}
	}]
	kind: "context"
}, {
	title: "Nightly routine wrapper"
	blocks: [{
		entry: {
			id:        "SKILL-SPECDRIFT-NIGHTLY-001a"
			role:      "def"
			statement: "Routine `spec-drift` wraps it weekly: `/spec-drift --nightly cap: 40` over the repo's durable doc set."
			title:     "Wrapper shape"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-NIGHTLY-001b"
			role:      "def"
			statement: "Report written to a dated file for morning review, no fixes applied."
			title:     "Wrapper output"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-NIGHTLY-002"
			role:      "def"
			statement: "Auth resolves via non-bare `claude -p` + `CLAUDE_CODE_OAUTH_TOKEN` (`claude setup-token`) under Pro subscription."
			title:     "Auth"
		}
	}, {
		entry: {
			id:        "SKILL-SPECDRIFT-NIGHTLY-003"
			pred:      "not schedule_created_by_skill"
			role:      "rule"
			statement: "Routine is separate build step — this skill NEVER creates schedule itself."
			title:     "Separate build step"
		}
	}]
	kind: "context"
}, {
	title:   "Nightly route"
	between: "DOCREPORT-001b..DOCREPORT-002"
	blocks: [{
		entry: {
			id: "SKILL-SPECDRIFT-NIGHTLY-004"
			post: ["report_written"]
			role:      "step"
			statement: "WRITE the report to `reports/spec-drift/<YYYY-MM-DD>.md` in the run worktree; its Summary goes to `.routine-commit-body`."
			title:     "Nightly report"
		}
	}]
	kind: "route"
	pred: "mode = nightly"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "claim extraction and code-vs-doc adjudication, plus exhaustive multi-language consumer enumeration and divergence classification"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {
		check: ["test", "-s", ".routine-commit-body"]
	}
	fixture: {
		patch: "change.patch"
	}
	invoke: "internal/smokedrift/README.md cap: 5 --nightly"
}
