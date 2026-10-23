changelog: [{
	date:    "2026-10-17"
	version: "3.3.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  1002
}, {
	date:    "2026-10-04"
	version: "3.2.2"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  1090
}, {
	date:    "2026-09-28"
	version: "3.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  1171
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; Args converted to the table form; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kinds (Axes context, Process/Output default phase), pred/post clauses on Process/Output entries, sibling atoms.json (5 local bool atoms), two queries (no-fix-ever, report-needs-verdict)."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Migrated to format v2 — When to use folded into Routing (Purpose/Disambiguation/Preconditions/Pipeline); content identical"
}, {
	date:    "2026-09-05"
	version: "1.9"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-04"
	version: "1.8"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not impact-evaluation input"
}, {
	date:    "2026-09-04"
	version: "1.7"
	text:    "acdsl-context added: risk-tiering entries and repo facts — they bind on the impact verdict this skill produces"
}, {
	date:    "2026-08-23"
	version: "1.6"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.5"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-20"
	version: "1.4"
	text:    "description compressed under the brevity caps"
}, {
	date:    "2026-08-09"
	version: "1.3"
	text:    "description gains the Args — summary (ACDSL-SKILL-002 gate)"
}, {
	date:    "2026-07-26"
	version: "1.2"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "1.1"
	text:    "renamed feature-impact → fimpact (leaf dir, frontmatter name, H1); live references swept repo-wide"
}, {
	date:    "2026-07-16"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " fimpact"
}, {
	key:   "description"
	value: " Evaluate a change's consequences against qualitative axes — maintainability, security, business impact, plus any named at intake — a verdict, never a fix. Trigger on \"what are the consequences/impact of this change\" or \"evaluate this change\". Args — change: the diff, plan, or description to evaluate; axes: additional impact axes named at intake."
}, {
	key:   "author"
	value: " Mustafa Karademir"
}, {
	key:   "version"
	value: " 3.3.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-RISK-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"change: adding a redis cache in front of the pricing endpoint axes: performance operations\""
}]
preamble: """
	# fimpact

	Given change, evaluate against impact axes.

	Not breakage check: change working is review's job. This skill answers what change does to system, product.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["identifiable change: diff, plan, or precise description of what would change"]
			purpose: "weighs proposed or pending change against impact axes; returns verdict, never fix."
			routes: [{
				for: "hunting defects, verifying change doesn't break anything"
				use: "/railroad-review"
			}, {
				for: "designing change"
				use: "/fdesign"
			}, {
				for: "diagnosing existing failure"
				use: "/diagnose-debug"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "change"
				default: "required"
				effect:  "change to evaluate — diff, plan, or precise description"
				form:    "free text"
			}, {
				name:    "axes"
				default: "built-in axes only"
				effect:  "extra impact axes beyond maintainability/security/business (e.g. performance, operations, compliance)"
				form:    "axis list"
			}]
		}
	}]
}, {
	title: "Axes"
	blocks: [{
		prose: """
			| Axis | Question | Evidence |
			| :--- | :--- | :--- |
			| `maintainability` | Does system get easier or harder to understand, change? | New concepts introduced vs removed<br>ownership/layering shifts<br>parallel mechanisms left alive |
			| `security` | Does attack surface grow or shrink? | New inputs, endpoints, credentials, trust boundaries<br>weakened guards |
			| `business` | What does product gain, lose, or risk? | Affected user flows/stories<br>behavior changes users can observe<br>operational cost |
			| custom (named) | User names further axes at intake (performance, operations, compliance, …) | Stated at intake |
			"""
	}]
	kind: "context"
}, {
	title: "Process"
	blocks: [{
		entry: {
			id: "SKILL-FIMPACT-PROCESS-001"
			post: ["change_read"]
			role:      "step"
			statement: "TAKE change plus any custom axes; READ diff/plan and code it lands in."
			title:     "Intake"
		}
	}, {
		entry: {
			id: "SKILL-FIMPACT-PROCESS-002"
			post: ["axes_evaluated"]
			pred:      "change_read"
			role:      "step"
			statement: "GROUND each judgment in specific evidence (path:line or plan section); no axis skipped — axis change doesn't touch gets `neutral — <reason>`."
			title:     "Evaluate per axis"
		}
	}, {
		entry: {
			id: "SKILL-FIMPACT-PROCESS-003"
			post: ["verdict_produced"]
			pred:      "axes_evaluated"
			role:      "step"
			statement: "PRODUCE per axis: better | worse | neutral | mixed, with one paragraph reasoning, then overall verdict naming dominant axis."
			title:     "Verdict"
		}
	}]
}, {
	title: "Output"
	blocks: [{
		entry: {
			id:        "SKILL-FIMPACT-OUTPUT-001a"
			pred:      "not fix_produced"
			role:      "rule"
			statement: "Fimpact MUST NOT produce fix, plan, or defect list — output stays read-only."
			title:     "Read-only"
		}
	}, {
		entry: {
			id: "SKILL-FIMPACT-OUTPUT-001"
			post: ["report_produced"]
			pred:      "verdict_produced"
			role:      "step"
			statement: "PRODUCE one report: per-axis sections in table order (verdict + evidence), then final Overall line."
			title:     "One report"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "cross-axis judgment on repo-wide evidence"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "change: cache the parsed context/context.json in the config server process instead of re-reading it per request axes: operations"
}
