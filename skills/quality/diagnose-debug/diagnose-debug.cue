changelog: [{
	date:    "2026-10-17"
	version: "3.6.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  5374
}, {
	date:    "2026-10-07"
	version: "3.5.0"
	text:    "Phase 2 gained two verify entries before the diagnosis is written: Every occurrence lists each reported occurrence of a plural symptom and requires the cause to explain each; Prediction run executes the fix's cheapest observable prediction once and sends a refuted one back to hypotheses. Plan format (WRITE-001) is gated on both."
}, {
	date:    "2026-10-04"
	version: "3.4.0"
	text:    "Template re-ordered for the reader: TLDR (Symptom, Cause, Fix), Symptom, Root cause (classification as its last line), Changes; Reproduction, Ruled out, Other findings and Baseline move behind [AGENT-ONLY]."
	tokens:  5181
}, {
	date:    "2026-10-04"
	version: "3.3.3"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  5034
}, {
	date:    "2026-09-30"
	version: "3.3.2"
	text:    "Template re-cut onto the human-first plan catalog: Context became the Reproduction override, Baseline moves behind Changes as an [AGENT-ONLY] section."
	tokens:  5115
}, {
	date:    "2026-09-28"
	version: "3.3.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.3.0"
	text:    "An approved fix implemented ad-hoc in the same session is committed via /package-commit."
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: first Args section — opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kinds on Self-check gate and Stop conditions (context); pred/post clauses on 34/35 coverable entries; sibling atoms.json (40 bool, 1 int); three queries (contradicting-hypothesis-blocks-write, two-disproven-blocks-hypothesis, clean-run-reaches-diagnosis); caveman compression pass."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use replaced with Routing (Purpose/Disambiguation/Preconditions/Pipeline), intro cut to a one-sentence lead"
}, {
	date:    "2026-09-06"
	version: "1.12"
	text:    "Changes template placeholder now requires the planned-files table (File | Kind | Entry) at the top of Changes, per the new plan-format inventory rule"
}, {
	date:    "2026-09-05"
	version: "1.11"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.10"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-03"
	version: "1.9"
	text:    "Diagnosis re-cut onto the plan catalog (TPL-001 declared via plan-template): Evidence → Baseline (verified), Proposed fix → Changes, Root cause/Classification/Other findings/Ruled out as override sections; RULE-PLAN-* injected"
}, {
	date:    "2026-09-01"
	version: "1.8"
	text:    "Context-entry ids removed from the body; implementing-chapter entries declared via new frontmatter acdsl-context (context/rules/skills.md)"
}, {
	date:    "2026-08-23"
	version: "1.7"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-12"
	version: "1.6"
	text:    "stop-condition reference corrected — ACTION-IMPL-* gate entries in actions/implementing.md (retired ALWAYS-EXEC naming and rules/ path)"
}, {
	date:    "2026-07-31"
	version: "1.5"
	text:    "activity-scoped context — stop conditions as ALWAYS-EXEC entries in rules/implementing.md"
}, {
	date:    "2026-07-30"
	version: "1.4"
	text:    "context redesign — stop conditions via ../fdesign/assets/stop-conditions.md"
}, {
	date:    "2026-07-30"
	version: "1.3"
	text:    "moved under skills/quality/ group; name and behavior unchanged"
}, {
	date:    "2026-07-23"
	version: "1.2"
	text:    "Phase 1 evidence — payload-before-layers, disk-over-diff, config-surface proof, live-probe, gh issue-status verification; Phase 2 — never blame user env/stale binary, correction-as-rule grep-sweep; Phase 3 — 'not possible cleanly' acceptable, no DB writes on render"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-05"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " diagnose-debug"
}, {
	key:   "description"
	value: " Produce a verified root-cause diagnosis for approval — a diagnosis, not a fix. Trigger on /diagnose-debug or \"find out why X\" or pasted stack traces/logs."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.6.0"
}, {
	key:   "title-code"
	value: " DD"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-IMPL-*, ACTION-NAV-010, ACTION-NAV-011, RULE-PLAN-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "plan-template"
	value: " SKILL-DIAGNOSEDEBUG-TPL-001"
}]
preamble: """
	# Debug

	Produces one artifact: verified root-cause diagnosis, never a fix.

	Fix is a separate step, starts only after user approves diagnosis ("Go ahead"). Diagnose-then-ask wins; fix-while-guessing
	loses.

	Static constraints not restated here: `AGENTS.md` and style guides under `$AGENT_CONTEXT_DIR_DEFAULT/` govern any eventual
	fix. Investigation goes through project's Makefile targets like everything else.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "fdesign"
					optional: true
				}, {
					kind:     "to"
					skill:    "spec-drift"
					optional: true
					route:    "contract"
				}]
				note:       "an approved diagnosis hands off to direct implementation, or to /fdesign for non-trivial fixes"
				standalone: true
			}
			preconditions: ["observable symptom with evidence (logs, traces, repro steps, or environment comparison)"]
			purpose: "diagnoses bug, error, or unexplained behavior with unknown cause — pasted stack traces, Sentry errors, raw logs, any \"find out why X\" ask. Output is diagnosis, not fix."
			routes: [{
				for: "cause already known, fix trivial"
				use: "fix directly"
			}, {
				for: "a plan or design question"
				use: "/fdesign or /fexplore"
			}, {
				for: "writing tests for existing code"
				use: "/coverage-increase"
			}]
		}
	}]
}, {
	title: "Phase 0 — Intake"
	blocks: [{
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-INTAKE-001"
			post: ["facts_restated"]
			role:      "step"
			statement: "RESTATE symptom as observable facts: what happens, where, since when, what still works. User's statements are constraints — hypothesis contradicting one is dead on arrival."
			title:     "Restate as facts"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-INTAKE-002"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "\"Check develop, this is were the issue is. MAN\""
			}]
			post: ["env_mapped"]
			role:      "step"
			statement: "MAP environments to deployed branches before any diff — diff deployed branches, not local worktree (often pinned at main, making `main..HEAD` a no-op)."
			title:     "Environment mapping"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-INTAKE-003"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "\"I just introduced that just too see something. Bullshit wrong.\""
			}]
			post: ["asked_what_changed"]
			role:      "step"
			statement: "ASK what user changed or already ruled out. Freshly added anomalous code is usually their debug probe, not the regression — ask before building theory on it."
			title:     "Ask what changed"
		}
	}]
}, {
	title: "Phase 1 — Evidence"
	blocks: [{
		entry: {
			id:        "SKILL-DIAGNOSEDEBUG-EVIDENCE-011b"
			pred:      "not trusts_untrusted_source"
			role:      "rule"
			statement: "Diagnosis MUST NOT trust WebFetch, search results, or training knowledge for issue state."
			title:     "No untrusted sources"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-006b"
			custom: ["todo:unmeasured"]
			pred:      "not invents_limitation"
			role:      "quality"
			statement: "MUST NOT invent platform limitations, MUST NOT trust search result over evidence in front of you."
			title:     "No invented limitations"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-001"
			post: ["evidence_collected"]
			pred:      "facts_restated"
			role:      "step"
			statement: "COLLECT evidence before theorizing. Every fact needs a source (log line, file:line, command output)."
			title:     "Evidence before theory"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-002"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "an elaborate timeline was built on 208-day-old logs."
			}]
			post: ["evidence_fresh"]
			pred:      "evidence_collected"
			role:      "verify"
			statement: "Before analyzing: VERIFY evidence timestamps against now; discard stale logs instead of analyzing."
			title:     "Freshness"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-003"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "the root cause sat directly after \"Initialising schema...\" in a log the user had pasted twice."
			}]
			post: ["timeline_parsed"]
			pred:      "evidence_fresh"
			role:      "step"
			statement: "PARSE pasted logs positionally: what happened immediately before and after failure. Position of event in log IS the diagnosis."
			title:     "Timeline"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-004"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "two context deaths in one session from ~700KB of raw logs."
			}]
			post: ["paste_grepped"]
			pred:      "large_paste_present"
			role:      "step"
			statement: "GREP large pastes for relevant identifiers; never hold full dump in context."
			title:     "Large pastes"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-005"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "the \"newly introduced bug\" was a warm vs cold state DB; the code was identical."
			}]
			post: ["state_diffed"]
			pred:      "persisted_state_service"
			role:      "step"
			statement: "DIFF state between environments before diffing code, for services with persisted state (session files, state DBs, caches, cache keys) — even when user suspects code regression."
			title:     "State before code"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-006a"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "three invented explanations for a hook that the docs settled in one sentence; a GitHub issue \"confirmed\" a bug the user's own CI logs disproved."
			}]
			post: ["tool_behavior_sourced"]
			pred:      "tool_behavior_surprising"
			role:      "step"
			statement: "READ the tool's docs/source until you can say WHY it behaves that way, citing it."
			title:     "Surprising tool behavior"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-007"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a cherry-pick dropped `\"country_code\": \"DE\"`; \"Took me 30 seconds\"."
			}]
			post: ["payload_diffed"]
			pred:      "request_flow_failing"
			role:      "step"
			statement: "DUMP actual payload and diff field-by-field before theorizing in any downstream layer, when request flow fails."
			title:     "Payload before layers"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-008"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a phantom critical bug — \"You checked a stale implementation.\""
			}]
			post: ["files_reread_from_disk"]
			pred:      "recent_merge_or_hotfix"
			role:      "step"
			statement: "FORCE a re-read of affected files from disk after cherry-picks, merges, or user hotfixes before diagnosing; commit diffs are not current state."
			title:     "Disk over diff"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-009"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "an MCP env block configured the child process only."
			}]
			post: ["config_surface_verified"]
			pred:      "config_surface_claimed"
			role:      "verify"
			statement: "Before building on a config surface: VERIFY host process actually reads it, against deployed artifact, not checkout."
			title:     "Config surfaces"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-010"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a ~1.5h doc stall settled by one probe."
			}]
			post: ["live_probe_run"]
			pred:      "capability_question_blocking"
			role:      "step"
			statement: "RUN a five-minute test import instead of document archaeology, when capability question blocks progress."
			title:     "Live probe"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-EVIDENCE-011a"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "three issues cited as open+corroborating during a root-cause diagnosis were all closed."
			}]
			post: ["issue_status_verified"]
			pred:      "github_issue_cited"
			role:      "step"
			statement: "VERIFY GitHub issue/PR status with `gh issue view --json state`."
			title:     "Issue status (gh)"
		}
	}]
}, {
	title: "Phase 2 — Hypothesize"
	blocks: [{
		entry: {
			id:        "SKILL-DIAGNOSEDEBUG-HYPO-005"
			pred:      "treated_as_root_cause => reproduces_symptom_trajectory"
			role:      "rule"
			statement: "A found bug MUST NOT be treated as THE bug: a landmine found along the way is a separate finding — root cause only if it reproduces the reported symptom trajectory."
			title:     "Not THE bug"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-006"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "\"NEVER EVER EVER EVER TELL ME I RAN A STALE BINARY.\""
			}]
			pred:      "blames_environment => fresh_build_reproduced"
			role:      "rule"
			statement: "Diagnosis MUST NOT attribute a bug to the user's environment or a stale binary. Reproduce the exact failing invocation against a fresh build before presenting such a theory."
			title:     "Never blame environment"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-003"
			custom: ["todo:unmeasured"]
			pred:      "one_variable_per_round"
			role:      "quality"
			statement: "MUST change one variable per test round; otherwise \"it works now\" cannot be attributed to a cause."
			title:     "One variable"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-004"
			custom: ["todo:unmeasured"]
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "12 turns chasing test-listing output while every run printed PASS; \"Port in use you idiot\"."
			}]
			pred:      "ground_truth_trusted"
			role:      "quality"
			statement: "MUST trust ground-truth signals: PASS and exit codes beat listing or formatting anomalies; a bind error beats harness theory."
			title:     "Trust ground truth"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-001"
			post: ["hypothesis_stated"]
			pred:      "evidence_fresh"
			role:      "step"
			statement: "STATE hypotheses as hypotheses, never as \"Found the bug\". Promote to diagnosis only when it explains **every** observed fact — env A vs env B, every log line proving the process was alive."
			title:     "State as hypotheses"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-008"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a 'multiple occasions' symptom was explained from one screenshot episode; only a sweep of every occurrence found the causes."
			}]
			post: ["occurrences_enumerated"]
			pred:      "symptom_plural"
			role:      "verify"
			statement: "Before presenting: VERIFY every reported occurrence of a plural symptom is listed with its source and the cause explains each; else sweep the rest and re-hypothesize."
			title:     "Every occurrence"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-009"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a two-point timing extrapolation was approved as the mechanism; the raised limit failed twice before the real cause surfaced."
			}]
			post: ["prediction_tested"]
			pred:      "hypothesis_stated"
			role:      "verify"
			statement: "Before presenting: VERIFY the fix's cheapest observable prediction (rerun with changed value, one-variable probe) ran once, output quoted as pivotal Baseline fact; refuted ⇒ disproven, re-hypothesize."
			title:     "Prediction run"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-002"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a crash-loop theory contradicted the agent's own earlier observation that the handler was firing."
			}]
			pred:      "hypothesis_predicts_loud_symptoms and not loud_symptoms_observed"
			role:      "stop"
			statement: "If the theory predicts loud symptoms (crash loops, restart storms) the user did not observe ⇒ STOP: discard before presenting."
			title:     "Would-have-noticed gate"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-HYPO-007"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a blanket rule applied to only some siblings at ~165k tokens."
			}]
			post: ["sibling_sites_swept"]
			pred:      "correction_phrased_as_rule"
			role:      "step"
			statement: "SWEEP every sibling call site mechanically before claiming done, when a correction is phrased as rule."
			title:     "Rule triggers sweep"
		}
	}]
}, {
	title: "Phase 3 — Write the diagnosis"
	blocks: [{
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-WRITE-003"
			items: [{
				label:  "Origin"
				marker: "*"
				text:   "a `get_or_create` on page render — \"it returned 4!\""
			}]
			pred:      "not side_effect_write"
			role:      "rule"
			statement: "Rendering MUST NOT write DB rows as a side effect."
			title:     "No side-effect writes"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-WRITE-001"
			post: ["diagnosis_written"]
			pred:      "hypothesis_stated and not hypothesis_contradicts_user_fact and prediction_tested and (symptom_plural => occurrences_enumerated)"
			role:      "step"
			statement: "FILL diagnosis per plan format in `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`, delivered per its delivery rule (plan-mode plan file, else chat)."
			title:     "Plan format"
		}
	}, {
		entry: {
			id:        "SKILL-DIAGNOSEDEBUG-WRITE-002"
			pred:      "diagnosis_written"
			role:      "stop"
			statement: "If the diagnosis is written ⇒ STOP: ask before fixing. \"Not a bug\" is a valid outcome when backed by the evidence walk; \"not possible cleanly\" is acceptable too."
			title:     "Stop before fixing"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-WRITE-004"
			post: ["fix_committed"]
			pred:      "diagnosis_approved and fix_implemented_adhoc"
			role:      "step"
			statement: "COMMIT an approved fix implemented ad-hoc in this session (no /fdesign plan) via /package-commit through the Skill tool, after validation is green — never raw `git commit`, never left uncommitted."
			title:     "Commit ad-hoc fix"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Diagnosis, on the plan catalog."
			title:     "Diagnosis template"
		}
	}]
}, {
	title: "Self-check gate"
	blocks: [{
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-CHECK-001"
			custom: ["todo:unmeasured"]
			pred:      "root_cause_explains_all_facts"
			role:      "quality"
			statement: "Root cause MUST explain every fact in Evidence, including the A/B differential and the facts proving what still worked."
			title:     "Explains every fact"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-CHECK-002"
			custom: ["todo:unmeasured"]
			pred:      "hypothesis_predicts_loud_symptoms => loud_symptoms_observed"
			role:      "quality"
			statement: "It MUST pass the would-have-noticed gate."
			title:     "Passes noticed gate"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-CHECK-003"
			custom: ["todo:unmeasured"]
			pred:      "evidence_fresh"
			role:      "quality"
			statement: "Evidence MUST be fresh and the diff baseline MUST be the deployed branch."
			title:     "Fresh baseline"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-CHECK-004"
			custom: ["todo:unmeasured"]
			pred:      "not blames_environment or fresh_build_reproduced"
			role:      "quality"
			statement: "Nothing MUST blame the user's own instrumentation without asking first."
			title:     "No blind blame"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-CHECK-005"
			custom: ["todo:unmeasured"]
			pred:      "tool_behavior_surprising => tool_behavior_sourced"
			role:      "quality"
			statement: "Surprising tool behavior MUST be backed by docs/source, not by an invented limitation."
			title:     "Sourced tool claims"
		}
	}]
	kind: "context"
}, {
	title: "Stop conditions"
	blocks: [{
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-STOP-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "`$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md`, injected via this skill's context declaration."
			}]
			role:      "rule"
			statement: "The implementing chapter's gate entries MUST apply — notably: second failed fix stops, research, redesign."
			title:     "Implementing gates apply"
		}
	}, {
		prose: "Debug-specific:"
	}, {
		entry: {
			id:        "SKILL-DIAGNOSEDEBUG-STOP-002"
			pred:      "hypothesis_contradicts_user_fact"
			role:      "stop"
			statement: "If a hypothesis contradicts a user-stated fact ⇒ STOP: discard, do not present."
			title:     "Contradicts user fact"
		}
	}, {
		entry: {
			id:        "SKILL-DIAGNOSEDEBUG-STOP-003"
			pred:      "disproven_count >= 2"
			role:      "stop"
			statement: "If two hypotheses are disproven ⇒ STOP guessing: collect richer evidence instead — request logs, add temporary instrumentation (remove once question answered), or reproduce."
			title:     "Two disproven"
		}
	}, {
		entry: {
			id: "SKILL-DIAGNOSEDEBUG-STOP-004"
			post: ["pasted_evidence_mined"]
			pred:      "user_pasted_evidence"
			role:      "step"
			statement: "MINE pasted evidence before theorizing further, when the user pastes evidence. Shifting doubt onto the user (\"are you sure your log is first?\") comes only after their evidence is exhausted."
			title:     "Mine pasted evidence"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "root-cause reasoning over unfamiliar failures"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	fixture: {
		patch: "change.patch"
	}
	invoke: "go test ./internal/smokedebug fails: TestAverage expects 2.5 for [2 3] and gets 2 — find out why"
}
