changelog: [{
	date:    "2026-10-17"
	version: "3.5.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  2952
}, {
	date:    "2026-10-15"
	version: "3.4.0"
	text:    "Verdict in chat by default: verdict.md is written into the plan dir and committed only with the persist flag (git add and git commit -m granted). Probes run where a permitted call can; the example plan path takes the feature-folder form."
	tokens:  3040
}, {
	date:    "2026-10-04"
	version: "3.3.2"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  2782
}, {
	date:    "2026-09-28"
	version: "3.3.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2863
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "Alternatives route declares its anchor: between VERDICT-002b..end (optional insertion); the route line stays in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; Args converted to the table form; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind lines (Global rules/Output as context, Alternatives as a route gated by problem_confirmed), pred/post clauses on rule/step/stop entries, sibling atoms.json, three queries. Intro's static-constraints boilerplate replaced by one Global-rules entry (injected acdsl-context, gates authoritative over prose, missing-doc-stated-once) carrying a pred/post clause."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation table/Preconditions/Pipeline), argument-hint to a full example invocation, allowed-tools normalized, caveman-compressed prose"
}, {
	date:    "2026-09-05"
	version: "1.7"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-04"
	version: "1.6"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not verdict input"
}, {
	date:    "2026-09-04"
	version: "1.5"
	text:    "acdsl-context narrowed: the RULE-PLAN-* glob replaced by the two exact plan-dir entries this skill depends on (naming + glob resolution) — the verdict reads plans, it does not author them"
}, {
	date:    "2026-09-01"
	version: "1.4"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.3"
	text:    "verdict.md destination migrated to the dated plan-dir convention (RULE-PLAN-071/072)"
}, {
	date:    "2026-08-23"
	version: "1.2"
	text:    "added argument-hint frontmatter; decision-support reference updated to /support-decision"
}, {
	date:    "2026-08-23"
	version: "1.1"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-21"
	version: "1.0"
	text:    "initial — verdict-first code analysis with conditional alternatives stage; closes the fexplore/fimpact gap from the code-analysis-alternatives session"
}]
frontmatter: [{
	key:   "name"
	value: " code-verdict"
}, {
	key:   "description"
	value: " Verdict on a scoped piece of existing code — problem or fine — with alternative solutions only when a problem is confirmed, presented in chat. Trigger on /code-verdict or \"analyze this code\" or \"is this code a problem, and what are the alternatives\". Args — target: file/line anchor or pasted snippet; concern: suspected problem to test, optional; persist: write verdict.md into the plan dir and commit it."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.5.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"target: internal/pricing/discount.go:120-145 concern: TOCTOU race persist\""
}, {
	key:   "allowed-tools"
	value: " Glob, Grep, Read, Write, Bash(git add *), Bash(git commit -m *), Bash(git log *), Bash(grep *)"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-*, RULE-PLAN-071, RULE-PLAN-072, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}]
preamble: """
	# Code Verdict

	Delivers one verdict in chat: grounded problem-or-fine verdict on scoped existing code, evaluated alternatives only when
	problem confirmed; `persist` writes it as `verdict.md` into the plan dir and commits it.

	A "no actionable problem" verdict: complete, successful outcome — never padded with speculative alternatives. (Origin:
	founding session asked "analysis of a snippet, alternatives if there's a problem"; /fexplore wrong tool; verdict turned out
	"no problem — snippet implements recorded decision D9 verbatim".)
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "fexplore"
					optional: true
				}, {
					kind:     "to"
					skill:    "fdesign"
					optional: true
				}, {
					kind:     "to"
					skill:    "fimplement"
					optional: true
				}]
				note:       "/fexplore when the alternatives space turns feature-level; /fdesign or /fimplement when the user accepts a recommended alternative"
				standalone: true
			}
			preconditions: ["identifiable code target — `path:line-range` in reachable repo, or pasted snippet plus its repo."]
			purpose: "problem-or-fine verdict on existing code (function, snippet, few lines); alternatives if problem confirmed."
			routes: [{
				for: "a proposed or pending change"
				use: "/fimpact"
			}, {
				for: "an external position against a closed question"
				use: "/support-decision"
			}, {
				for: "an observed failure or symptom"
				use: "/diagnose-debug"
			}, {
				for: "a feature-level, contested solution space"
				use: "/fexplore"
			}, {
				for: "a branch or PR"
				use: "/railroad-review"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "target"
				default: "required"
				effect:  "code under verdict: `path:lines` anchor, or pasted snippet plus its repo"
				form:    "free text, positional"
			}, {
				name:    "concern"
				default: "absent"
				effect:  "suspected problem to test (e.g. \"TOCTOU race\") — hypothesis to verify, never pre-accepted verdict"
				form:    "free text"
			}, {
				name:    "persist"
				default: "absent"
				effect:  "write `verdict.md` into the plan dir and commit it — [Persist verdict](#skill-codeverdict-out-004); absent: chat only"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Global rules"
	blocks: [{
		entry: {
			id: "SKILL-CODEVERDICT-CTX-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "missing context doc — stated once, run continues on built-in baselines."
			}]
			pred:      "context_doc_missing => baseline_stated"
			role:      "rule"
			statement: "Skill MUST source static constraints from the frontmatter `acdsl-context:` injection, never restate them; gates rule over prose."
			title:     "Injected constraints"
		}
	}]
	kind: "context"
}, {
	title: "1. Intake"
	blocks: [{
		entry: {
			id:        "SKILL-CODEVERDICT-INTAKE-004"
			pred:      "not concern_preaccepted"
			role:      "rule"
			statement: "Skill MUST treat user's `concern` as hypothesis to verify against evidence, never pre-accepted verdict."
			title:     "Concern hypothesis"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-INTAKE-001"
			post: ["target_resolved"]
			role:      "step"
			statement: "RESOLVE `target` to real lines on disk; pasted snippet matched to its file first. No verdict on code not read from repo."
			title:     "Target resolution"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-INTAKE-002"
			post: ["contracts_enumerated"]
			pred:      "target_resolved"
			role:      "step"
			statement: "ENUMERATE call sites and contracts code participates in (callers, exceptions raised, API/error-code contracts, DB writes guarded)."
			title:     "Contract enumeration"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-INTAKE-003"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "design decisions and verification checklists are evidence for verdict and destination for artifact."
			}]
			pred:      "target_resolved"
			role:      "step"
			statement: "LOCATE owning feature plan dir when one exists (glob per plan-dir resolution rule, rules/plan.md) — evidence source, and the `persist` destination."
			title:     "Plan dir"
		}
	}]
}, {
	title: "2. Ground"
	blocks: [{
		entry: {
			id:        "SKILL-CODEVERDICT-GROUND-001"
			pred:      "not unanchored_claim_in_verdict"
			role:      "rule"
			statement: "Claims MUST carry `path:line` anchor (or measurement); unanchored claim MUST NOT enter verdict — stays opinion."
			title:     "Anchored claims"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-GROUND-002"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "code implementing recorded decision verbatim not a defect — cite decision, judge only whether stated premise still holds."
			}]
			pred:      "problem_confirmed => decisions_checked"
			role:      "rule"
			statement: "Skill MUST check recorded design decisions covering target lines before calling problem."
			title:     "Recorded decisions"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-GROUND-003"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "e.g. \"does DB have that unique index\"."
			}, {
				marker: "*"
				text:   "Never claim from adjacent artifacts like migration files."
			}]
			post: ["probe_written"]
			pred:      "contracts_enumerated"
			role:      "step"
			statement: "RUN a probe for capability questions repo can't answer (fexplore measurement discipline); only a probe no permitted call can run goes to the user, point marked `unverified`."
			title:     "Measurement discipline"
		}
	}]
}, {
	title: "3. Verdict"
	blocks: [{
		entry: {
			id:        "SKILL-CODEVERDICT-VERDICT-001"
			role:      "rule"
			statement: "Each examined point MUST get exactly one classification: `problem confirmed` | `no actionable problem` | `unverifiable without measurement`, with evidence — never hedge in-betweens."
			title:     "Single classification"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-VERDICT-002a"
			post: ["verdict_written"]
			pred:      "not problem_confirmed"
			role:      "stop"
			statement: "If no point classified `problem confirmed` ⇒ STOP: present the verdict with per-point reasoning; `persist` writes it too."
			title:     "Clean stop write"
		}
	}, {
		entry: {
			id:        "SKILL-CODEVERDICT-VERDICT-002b"
			pred:      "not problem_confirmed"
			role:      "stop"
			statement: "If verdict clean ⇒ STOP: skip alternatives stage entirely."
			title:     "No alternatives"
		}
	}]
}, {
	title:   "4. Alternatives (only on a confirmed problem)"
	between: "VERDICT-002b..end"
	blocks: [{
		entry: {
			id:        "SKILL-CODEVERDICT-ALT-001c"
			role:      "rule"
			statement: "Options within noise of each other MUST both go to recommendation as OPEN."
			title:     "Noise tie list"
		}
	}, {
		entry: {
			id:        "SKILL-CODEVERDICT-ALT-001d"
			role:      "rule"
			statement: "Skill MUST NOT fake a winner."
			title:     "No fake winner"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-ALT-001a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "evaluation table `Option | Groundedness | Blast radius | Effort | Reversibility | Verdict`."
			}, {
				marker: "*"
				text:   "Option families are ones constraints admit; always include \"keep as is, accept cost\"."
			}]
			pred:      "problem_confirmed"
			role:      "step"
			statement: "APPLY Fexplore Phase 2–4 discipline scoped to snippet: hard constraints with anchors first, then option families, evaluation, recommendation."
			title:     "Alternatives discipline"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-ALT-002"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "needs concept-level decisions; never half-survey it."
			}]
			pred:      "feature_level"
			role:      "stop"
			statement: "If space turns feature-level or contested ⇒ STOP: hand off to /fexplore explicitly — verdict and handoff line are deliverable."
			title:     "Feature-level handoff"
		}
	}]
	kind: "route"
	pred: "problem_confirmed"
}, {
	title: "5. Output"
	blocks: [{
		entry: {
			id:        "SKILL-CODEVERDICT-OUT-002"
			pred:      "edits_started => user_go"
			role:      "rule"
			statement: "Skill MUST stay read-only — no file without `persist`, never a fix; edits start only on explicit user go, routed per Workflow position."
			title:     "Read-only"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-OUT-001"
			post: ["verdict_written"]
			role:      "step"
			statement: "PRESENT the verdict in chat in the [Verdict template](#skill-codeverdict-out-003)'s shape — one copy-ready block."
			title:     "Verdict in chat"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-OUT-004"
			items: [{
				marker: "*"
				text:   "e.g. `plans/2026-09-01-formsv1-8ca6ed6/2026-09-01-formsv1-pending-email-check-wip-8ca6ed6/verdict.md`"
			}, {
				label:  "param"
				marker: "*"
				text:   "`git add <path>` then `git commit -m \"plans.<feature folder>: Added the code verdict\"` — two calls."
			}]
			pred:      "persist_requested"
			role:      "step"
			statement: "With `persist` WRITE `verdict.md` into plan dir — owning feature's dir if target has a plan tree, else new dir per plan-dir naming rule, kebab-slugged after target — then add and commit it."
			title:     "Persist verdict"
		}
	}, {
		entry: {
			id: "SKILL-CODEVERDICT-OUT-003"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "`verdict.md` template:"
			title:     "Verdict template"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "grounded independent judgment, scoped solution-space evaluation"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	fixture: {
		patch: "change.patch"
	}
	invoke: "target: internal/smokeverdict/counter.go concern: concurrent map writes"
}
