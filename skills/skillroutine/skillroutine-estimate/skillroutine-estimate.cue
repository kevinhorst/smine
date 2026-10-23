changelog: [{
	date:    "2026-10-13"
	version: "3.2.0"
	text:    "Requires the mining component: the skill drives this repo's own make targets and never deploys outside the smine checkout."
	tokens:  4426
}, {
	date:    "2026-09-30"
	version: "3.1.5"
	text:    "Template re-cut onto the human-first plan catalog: Context became the Inputs override, Assumptions in the new shape."
	tokens:  4426
}, {
	date:    "2026-09-28"
	version: "3.1.4"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.1.3"
	text:    "Peek calls moved to the live mcp__peek-mcp namespace: session_get (plan only) replaces session_plan."
}, {
	date:    "2026-09-27"
	version: "3.1.2"
	text:    "Bracket and arm tokens use <model>:<effort>"
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema pass: pred/post clauses on 7 flow entries (read/peek/cells/price/report chain), sibling atoms.json (9 local bool atoms), 3 committed queries (stop-blocks-report, no-session witness, undocumented-fanout witness)."
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
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), Args bullets -> table, added argument-hint, normalized allowed-tools order, caveman pass."
}, {
	date:    "2026-09-05"
	version: "1.3"
	text:    "Moved pricing/ceilings to Claude usage facts, added plan-usage output, full peek surface (touched files), branch-fidelity and execution-strategy entries, and the estimate plan template."
}, {
	date:    "2026-09-05"
	version: "1.2"
	text:    "Peek-mcp measurement entries (session plan + events, telemetry arithmetic), hidden-cell math, explicit wall-clock entry (critical path, effort latency), per-token pricing block with cache-read term, per-plan window ceilings, first measured reference run."
}, {
	date:    "2026-09-05"
	version: "1.1"
	text:    "Entries conformed to the titled grammar — **Title** — statement headlines, 300-rune cap with detail moved to Why/Applies bullets"
}, {
	date:    "2026-09-05"
	version: "1.0"
	text:    "Initial: analytic cell derivation + measured-telemetry pricing + window-fit verdict for skill/routine configurations."
}]
frontmatter: [{
	key:   "name"
	value: " skillroutine-estimate"
}, {
	key:   "description"
	value: " Predict cost and usage for a skill or routine configuration before running it — cells, dollars, tokens, wall-clock, window fit. Trigger on /skillroutine-estimate <skill> [config] or \"what would this run cost\". Args — skill: target; config: invocation args or definition overrides; window: budget USD."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.2.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " FACT-REPO-*, FACT-CLAUDE-*, RULE-PLAN-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "plan-template"
	value: " SKILL-SKILLROUTINEESTIMATE-TPL-001"
}, {
	key:   "argument-hint"
	value: " \"railroad-review config: 'lanes: 2[opus:high] chunks: layers' window: 150\""
}, {
	key:   "allowed-tools"
	value: " Read, Bash(git ls-files*), Bash(jq *), Bash(ls *), mcp__peek-mcp__session_events, mcp__peek-mcp__session_get"
}]
preamble: """
	# Skillroutine Estimate

	Predicts cost, cells, tokens, wall-clock, and window fit for a skill or routine configuration before it runs.

	Never executes. Read-only — companion to skillroutine-eval, which scores a completed run instead of predicting one.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "context-bootstrap"
					optional: true
				}]
				note:       "feeds context-bootstrap's estimate gate"
				standalone: true
			}
			preconditions: ["the target skill's SKILL.md readable (deployed dir or repo)", "measured telemetry optional", "target spawns no agents and no session exists → report briefly (one session plus harness cells), never a full report"]
			purpose: "sizes a fan-out invocation before spending it — railroad-review with a full arg set, a skillroutine-eval matrix, a context-bootstrap run, a routine's headless dispatch."
			routes: [{
				for: "scoring a finished run"
				use: "/skillroutine-eval"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "skill"
				default: "none, required"
				effect:  "target skill (leaf) or routine name — a routine resolves to its wrapped skill plus the wrapper's arm matrix"
				form:    "free text, first"
			}, {
				name:    "config"
				default: "target's documented defaults"
				effect:  "concrete configuration to price — see [Derive cells](#skill-skillroutineestimate-cells-001)"
				form:    "target's own arg grammar, and/or `models`/`efforts`/`replicas` overrides"
			}, {
				name:    "window"
				default: "`150`"
				effect:  "window budget for the fit verdict (measured account ceiling ≈190/5h) — see [Report shape](#skill-skillroutineestimate-report-001)"
				form:    "USD"
			}]
		}
	}]
}, {
	title: "Procedure"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-REPORT-001b"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Estimates MUST always report a range; a point estimate is a defect."
			title:     "Ranges never blend"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-REPORT-002a"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Estimates MUST NOT launder uncertainty: one whose dominant term is unknown (e.g. rounds until two clean) reports the per-unit cost separately."
			title:     "No blending"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-REPORT-002b"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "The unknown multiplier MUST NOT be blended into a single number."
			title:     "Multiplier stays separate"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-READ-001a"
			items: [{
				marker: "*"
				text:   "Sources, in order: the target's SKILL.md Args/fan-out entries; `eval/definition.json` when bundled; the routine wrapper's env mapping for routine targets."
			}]
			post: ["math_read"]
			role:      "step"
			statement: "READ the target's own math, never assume."
			title:     "Read the math"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-READ-001b"
			items: [{
				marker: "*"
				text:   "When a session exists, the peek entries below supply the shape instead."
			}]
			post: ["unbounded_reported"]
			pred:      "not fanout_documented and not session_exists"
			role:      "step"
			statement: "TREAT a target with no fan-out shape and no observable session as unbounded: report \"unbounded/unknown — the skill documents no fan-out math\", never a guessed formula."
			title:     "No fan-out fallback"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-PEEK-001a"
			items: [{
				marker: "*"
				text:   "Touched files ground the estimate: they show which phases ran and what remains — scale remaining cost from them, not the full plan."
			}, {
				label:  "Plan decisions win"
				marker: "*"
				text:   "a plan-declared fan-out (e.g. \"sonnet lanes per group\") replaces the target skill's documented math for that run."
			}, {
				label:  "Live runs"
				marker: "*"
				text:   "re-peek at phase boundaries and report estimate-vs-actual drift, not just the original range."
			}]
			post: ["session_peeked"]
			pred:      "session_exists and not peek_unavailable"
			role:      "step"
			statement: "PEEK the target invocation's session before pricing when one exists: `session_get` (plan only) and `session_events` (tokens, agents, wall-clock, counters, files). Name the session in the report."
			title:     "Peek the session"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEESTIMATE-PEEK-001b"
			pred:      "session_exists and peek_unavailable"
			role:      "stop"
			statement: "If a known-existing session cannot be served by peek-mcp ⇒ STOP: report it as the blocker; never silently price session-blind when session telemetry exists."
			title:     "Peek unavailable"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-PEEK-002a"
			items: [{
				marker: "*"
				text:   "The main session's `usage` block excludes subagents: sum their envelopes separately."
			}]
			role:      "step"
			statement: "NEVER TRUST `cost_usd` alone: the counter can reset mid-run (in-memory stats survive only via the persisted fallback)."
			title:     "Telemetry arithmetic"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEESTIMATE-PEEK-002b"
			role:      "step"
			statement: "RECOMPUTE cost from token counters — output × output rate + cache writes × write rate + cache reads × read rate."
			title:     "Recompute from tokens"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-STATE-001"
			items: [{
				marker: "*"
				text:   "When that is not feasible, estimate from the available state and record it as an accuracy-lowering row in the report's Assumptions."
			}]
			role:      "step"
			statement: "READ the target skill's math from the same branch and state the executing session runs on (its `git_branch` from peek; `git show <branch>:<path>` when the estimator sits elsewhere)."
			title:     "Branch fidelity"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-CELLS-001"
			items: [{
				label:  "Example"
				marker: "*"
				text:   "railroad-review — rounds x (directions x chunks x lanes + groupers + refuters + verifier batches + build-test + station) + cleanup."
			}, {
				label:  "Example"
				marker: "*"
				text:   "eval matrices — arms = models x efforts x replicas, each arm one dispatcher cell + judge cells."
			}]
			post: ["cells_derived"]
			pred:      "math_read"
			role:      "step"
			statement: "DERIVE cells analytically from the target's formula with the config's values substituted. Show the formula with numbers substituted — every count must trace to a term."
			title:     "Derive cells"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEESTIMATE-CELLS-002a"
			role:      "step"
			statement: "ADD the cells the target's math never mentions: harness plan-mode agents (1–3 Explore + 0–3 Plan per planning phase) and any plan-declared lanes."
			title:     "Hidden cells"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEESTIMATE-CELLS-002b"
			role:      "step"
			statement: "TREAT wall-clock as the critical path of concurrent lanes, never the sum of cells."
			title:     "Critical-path wall-clock"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-TIME-001a"
			items: [{
				marker: "*"
				text:   "Calibrate from peeked session timings (`session_events` gives per-skill/per-subagent seconds) whenever a comparable run exists."
			}, {
				label:  "Effort dominates latency"
				marker: "*"
				text:   "fable-low ran a 5-phase build with gating in ~35 min, several times faster than medium/high."
			}, {
				marker: "*"
				text:   "Never reuse medium-effort durations for a low-effort run."
			}, {
				marker: "*"
				text:   "Measured stage anchors (2026-09-05): plan phase 20 min (fable-medium), implementation 35 min (fable-low), migration lanes 1–15 min each, in parallel."
			}]
			role:      "step"
			statement: "TREAT wall-clock as its own estimate, never a byproduct of cost."
			title:     "Explicit wall-clock"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEESTIMATE-TIME-001b"
			role:      "step"
			statement: "LAY per-stage durations on the critical path (parallel lanes overlap, only the slowest lane counts), with the model×effort latency stated per stage."
			title:     "Critical-path layout"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-STRAT-001a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "taken from the peeked plan when one exists; the report always carries this, per stage."
			}]
			role:      "step"
			statement: "STATE the run's model × effort per stage and subagent strategy — lane count, lane model/effort, parallelism — from the peeked plan when one exists."
			title:     "Execution strategy statement"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEESTIMATE-STRAT-001b"
			role:      "step"
			statement: "RECOMMEND a strategy when the target fixes none, and show its cost and wall-clock delta against the default."
			title:     "Execution strategy recommendation"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-PRICE-001a"
			items: [{
				marker: "*"
				text:   "No telemetry ⇒ analytic fallback from the priors table below, labeled \"analytic — no measured runs\"."
			}, {
				marker: "*"
				text:   "Interactive runs are measurable too: peek `session_events` serves per-session token envelopes and persisted telemetry (peek v1.2.2+)."
			}, {
				marker: "*"
				text:   "Pricing ladder, always named in the report: headless armruns > peeked session measurements > the priors below."
			}]
			post: ["priced"]
			role:      "step"
			statement: "PRICE from measurement first."
			title:     "Measurement first"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-PRICE-001b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "filtered to the config's model/effort where possible."
			}]
			role:      "step"
			statement: "When `evals/<skill>/` exists under the cwd, DERIVE per-run cost/token/duration distributions from `armruns.jsonl` and `stats.json`, and name the runs used."
			title:     "Derive from evals"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-REPORT-001a"
			items: [{
				marker: "*"
				text:   "Cost & plan usage carries the API-equivalent USD range and plan-usage percent — user's plan when known, else the all-tiers table with unmeasured tiers labeled."
			}, {
				marker: "*"
				text:   "Wall-clock states model×effort per stage."
			}, {
				label:  "Calibration source"
				marker: "*"
				text:   "measured runs vs priors — always named."
			}, {
				label:  "Window fit"
				marker: "*"
				text:   "`floor(window / (run + judge))` runs fit, against the plan's measured ceiling (context/facts/claude.md)."
			}, {
				label:  "Assumptions and unknowns"
				marker: "*"
				text:   "round count, candidate count, retry/resume overhead."
			}]
			post: ["report_rendered"]
			pred:      "cells_derived and priced"
			role:      "step"
			statement: "RENDER the estimate as a plan artifact from the template below and deliver it per the plan-interface delivery rule (plan-mode plan file, else chat)."
			title:     "Report shape"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-PRICE-002"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:      "payload"
			statement: "Per-cell priors (fallback only; update from evals as measurements land):"
			title:     "Per-cell priors"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEESTIMATE-TPL-001"
			payload: {
				info:   "markdown"
				marker: "````"
			}
			role:      "payload"
			statement: "The estimate-report template."
			title:     "Estimate template"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "arithmetic over documented formulas and jq over telemetry files; no generation."
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "qq"
}
