changelog: [{
	date:    "2026-10-09"
	version: "3.2.0"
	text:    "Added the Report consistency verify before the append: summary counts match the table per status, every queried DoD family has a row or a stated N/A, any FAIL row leads State"
}, {
	date:    "2026-09-28"
	version: "3.1.4"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2932
}, {
	date:    "2026-09-26"
	version: "3.1.3"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "3.1.2"
	text:    "Make-only entry points: allowed-tools grants make rules instead of go run ./cmd/rules."
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind clause on all four numbered sections (phase); pred/post clauses on 16/20 coverable entries (1 rule invariant, 7 guarded step/stop transitions, 2 soft quality preds); new sibling atoms.json (15 skill-local bool atoms — none proposed for global vocabulary); three queries under queries/ (no-dir-blocks-output: the No-dir stop blocks Dated-section reach, unsat; self-gather-witness and reviewed-mode-witness: both report routes reach Dated section, sat); further caveman compression of entry-statement prose. skillfmt/skillmodel check/consistency/query checks all green."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation table/Preconditions/Pipeline); caveman-compressed entry statements and intro; argument-hint set to a full example invocation; allowed-tools reordered and respaced to the space-wildcard dialect; bare EVID-001 reference converted to a title anchor link"
}, {
	date:    "2026-09-05"
	version: "1.6"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-04"
	version: "1.5"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not report input"
}, {
	date:    "2026-09-04"
	version: "1.4"
	text:    "acdsl-context narrowed: the RULE-PLAN-* glob replaced by the two exact plan-dir entries this skill depends on (naming + glob resolution) — the report reads plans, it does not author them"
}, {
	date:    "2026-09-01"
	version: "1.3"
	text:    "Context-entry ids removed from body instructions (kept only as data in the output-format example); plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.2"
	text:    "Plan-source lookup migrated to the dated plan-dir convention (RULE-PLAN-072)"
}, {
	date:    "2026-08-27"
	version: "1.1"
	text:    "refined report shape from the first real run: Blockers & gaps becomes a State section (FAILs/gaps first, then deployment status, required manual steps with acceptance refs, accepted risks with where recorded); How validated + How tested merge into Validation & testing with finding dispositions summarized by location class and rejected findings carrying their accepted-risk disposition; reviewed-mode DoD is a count summary + deltas since the prior round + pointer to the review's table (self-gather keeps the full table); header carries the review round trajectory and head-vs-approved-snapshot identity; date dropped from the title (the dated plan-file section carries it)"
}, {
	date:    "2026-08-24"
	version: "1.0"
	text:    "initial: reviewer-handoff DoD report — four fixed questions (why this solution, how validated, how tested, DoD per point), DoD set queried via the [DoD] entry marker (rules entries --marker DoD), consumes a railroad-review station output when present (reconcile, never duplicate), self-gather mode stated otherwise, FAILs/gaps escalated to a top Blockers section, delivered as a dated session-plan section in English"
}]
frontmatter: [{
	key:   "name"
	value: " dod-report"
}, {
	key:   "description"
	value: " Compile the reviewer-handoff Definition-of-Done report — why this solution, how validated, how tested, every [DoD] point confirmed with evidence. Trigger on /dod-report or \"prepare the handoff report/presentation\". Args — review: station review doc (default: latest in the session plan file)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.2.0"
}, {
	key:   "argument-hint"
	value: " \"review: plans/2026-09-01-foo/review.md\""
}, {
	key:   "allowed-tools"
	value: " Glob, Grep, Read, Write, Bash(git diff *), Bash(git log *), Bash(git status *), Bash(grep *), Bash(jq *)"
}, {
	key:   "acdsl-context"
	value: " ACTION-REVIEW-*, RULE-PLAN-071, RULE-PLAN-072, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "format"
	value: " 2"
}]
preamble: """
	# DoD Report

	Compiles the reviewer-handoff report: an English markdown doc, sized for a ~10-minute read, answering four fixed questions — why
	this solution, how validated, how tested, DoD per point confirmed.

	It's evidence compilation by the session that did the work, **not** a review: no fresh-eyes findings, no fixes — claims cite
	artifacts that already exist, or get reported as gaps.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "railroad-review"
				}]
				note:       "after railroad-review's human gate, before the human reviewer"
				standalone: true
			}
			preconditions: ["the change exists as a diff (branch or uncommitted) in a reachable repo with a context directory (deployed `$AGENT_CONTEXT_DIR_DEFAULT/` or `context/` in smine)"]
			purpose: "hands finished work to a human reviewer with a report — after an APPROVE/CONDITIONAL railroad-review verdict, or straight to a human with no review at all."
			routes: [{
				for: "the code still needs reviewing (adversarial, fresh eyes)"
				use: "/railroad-review"
			}, {
				for: "a verdict on a snippet"
				use: "/code-verdict"
			}, {
				for: "evaluating a change's consequences"
				use: "/fimpact"
			}, {
				for: "committing"
				use: "/package-commit"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "review"
				effect: "optional — path or pointer to a railroad-review station output (`review.md`/`review.json` handoff pair or the plan-file review section) to consume. Default: latest dated review section in the session plan file; none found means self-gather mode, stated in the report header."
			}]
		}
	}]
}, {
	title: "1. Intake"
	blocks: [{
		entry: {
			id: "SKILL-DODREPORT-INTAKE-001"
			post: ["scope_resolved"]
			role:      "step"
			statement: "RESOLVE change scope: branch diff against its base, or uncommitted snapshot — same scope reviewer reads. State resolved scope (base, head, file count) in report header."
			title:     "Change scope"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-INTAKE-002"
			post: ["review_resolved"]
			role:      "step"
			statement: "RESOLVE `review`: given doc, else latest dated railroad-review section in plan file. Multi-round: full trajectory across rounds. State head-vs-approved identity; none found: self-gather, in header."
			title:     "Review resolution"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-INTAKE-003"
			post: ["sources_located"]
			role:      "step"
			statement: "LOCATE why-this-solution sources: approved plan (session plan file or plan dir per plan-dir resolution rule), concept, and recorded decisions covering change."
			title:     "Decision sources"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-INTAKE-004a"
			post: ["dod_queried"]
			role:      "step"
			statement: "QUERY DoD set: `rules entries --marker DoD` (deployed: `--deployed <dir>`), fallback `jq '.entries[] | select(.markers // [] | index(\"DoD\"))' <context-dir>/context.json`."
			title:     "DoD query"
		}
	}, {
		entry: {
			id:        "SKILL-DODREPORT-INTAKE-004b"
			pred:      "not context_dir_present or dod_set_empty"
			role:      "stop"
			statement: "If no dir or empty set ⇒ STOP: report."
			title:     "No dir stop"
		}
	}]
	kind: "phase"
}, {
	title: "2. Evidence"
	blocks: [{
		entry: {
			id: "SKILL-DODREPORT-EVID-001"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Every claim MUST carry evidence: `path:line` anchor, command output, review-finding id, or plan-decision reference. Without evidence: reported as gap, never asserted."
			title:     "Evidence anchors"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-EVID-002"
			post: ["rationale_stated"]
			pred:      "sources_located"
			role:      "step"
			statement: "STATE why this solution: decisions made and alternatives rejected, from plan/concept — delta-only, no spec restatement; reviewer shares spec."
			title:     "Solution rationale"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-EVID-003a"
			post: ["validation_compiled"]
			pred:      "review_present"
			role:      "step"
			statement: "COMPILE validated evidence (feeds `## Validation & testing`): with review, dispositions by location class (docs/runbook/config vs prod code) + rejected findings' risk dispositions."
			title:     "Validation evidence"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-EVID-003b"
			post: ["validation_compiled"]
			pred:      "not review_present"
			role:      "step"
			statement: "Without review: session's record, labeled self-reported."
			title:     "Self-reported validation"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-EVID-004a"
			post: ["tests_compiled"]
			role:      "step"
			statement: "COMPILE tested evidence (feeds `## Validation & testing`): manual (happy/edge/error cases) and automated (test/build runs), each with command or scenario run and outcome."
			title:     "Test record"
		}
	}, {
		entry: {
			id:        "SKILL-DODREPORT-EVID-004b"
			role:      "step"
			statement: "Not-run tests listed as not run, never implied green."
			title:     "Not-run tests"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-EVID-005a"
			post: ["state_compiled"]
			role:      "step"
			statement: "COMPILE state evidence: deployment status (where, or not yet + blocker), manual steps each with acceptance ref, accepted risks each with where recorded (design doc, ledger, review)."
			title:     "State evidence"
		}
	}, {
		entry: {
			id:        "SKILL-DODREPORT-EVID-005b"
			role:      "step"
			statement: "Risks reported with evidence, never re-litigated."
			title:     "Risk evidence"
		}
	}]
	kind: "phase"
}, {
	title: "3. DoD walk"
	blocks: [{
		entry: {
			id: "SKILL-DODREPORT-DOD-002"
			custom: ["todo:unmeasured"]
			pred:      "review_present"
			role:      "quality"
			statement: "A review's DoD table MUST be reconciled, not duplicated: matching statuses cite review; discrepancies (stale PASS, fixed FAIL) called out with both statuses shown."
			title:     "Table reconciliation"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-DOD-003"
			custom: ["todo:unmeasured"]
			pred:      "dod_fail_present"
			role:      "quality"
			statement: "FAIL rows and evidence gaps MUST lead `## State`, before deployment status and accepted risks; handoff burying FAIL in row 30 defeats 10-minute format."
			title:     "FAIL escalation"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-DOD-001a"
			post: ["dod_summary_written"]
			pred:      "review_present"
			role:      "step"
			statement: "WRITE reviewed-mode summary: summary line (`N criteria: X PASS, Y N/A, Z FAIL`, source round), status deltas with one-line reasons, and pointer to review's table and evidence."
			title:     "Reviewed summary"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-DOD-001b"
			post: ["dod_summary_written"]
			pred:      "not review_present"
			role:      "step"
			statement: "WRITE self-gather-mode table: one row per `[DoD]` entry from intake query — `| Entry | Status | Evidence |`, status PASS/FAIL/N/A, N/A with reason, PASS with evidence per [Evidence anchors](#skill-dodreport-evid-001)."
			title:     "Self-gather table"
		}
	}]
	kind: "phase"
}, {
	title: "4. Output"
	blocks: [{
		entry: {
			id:        "SKILL-DODREPORT-OUT-002"
			pred:      "not session_fixes_gaps"
			role:      "rule"
			statement: "Session MUST NOT fix gaps or FAILs in same run — read-only beyond report; fixes route back through normal implementation flow."
			title:     "Read-only"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-OUT-003"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Report MUST be sized for 10 minutes: prose sections ~1 page together; DoD section is summary in reviewed mode, full table in self-gather mode, N/A rows grouped one line per reason."
			title:     "Ten-minute size"
		}
	}, {
		entry: {
			id:        "SKILL-DODREPORT-OUT-005"
			role:      "verify"
			statement: "Before append: VERIFY summary counts equal the table's rows per status, every queried DoD family has a row or a stated N/A, any FAIL row leads State; repair the report text, never a status."
			title:     "Report consistency"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-OUT-001"
			post: ["report_appended"]
			pred:      "rationale_stated and validation_compiled and tests_compiled and state_compiled and dod_summary_written"
			role:      "step"
			statement: "APPEND report to session plan file as dated section (railroad-review delivery convention); written in English regardless of working language."
			title:     "Dated section"
		}
	}, {
		entry: {
			id: "SKILL-DODREPORT-OUT-004"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Report template (the dated section heading from OUT-001 carries the date, the title does not):"
			title:     "Report template"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "evidence-vs-claim judgment across plan, diff, and review artifacts; compilation, not adversarial review"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	fixture: {
		tree: "tree"
	}
	invoke: ""
}
