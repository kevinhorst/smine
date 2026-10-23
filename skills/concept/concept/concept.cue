changelog: [{
	date:    "2026-10-05"
	version: "4.4.4"
	text:    "Added the Adds column to the Decisions table of the plan template"
	tokens:  6808
}, {
	date:    "2026-10-03"
	version: "4.4.3"
	text:    "Template follows the plan catalog: Decisions directly behind Scope, reader cells first in the Decisions header."
	tokens:  6804
}, {
	date:    "2026-10-02"
	version: "4.4.2"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  6818
}, {
	date:    "2026-09-30"
	version: "4.4.1"
	text:    "Template re-cut onto the human-first plan catalog: Context folded into a Drivers table, Scope items titled with sub-bullets, Decisions in the Brief / Decision (brief) shape."
	tokens:  6779
}, {
	date:    "2026-09-29"
	version: "4.4.0"
	text:    "Scope drops the Deferred findings group: a discovery is folded into In with its decision, or dropped."
}, {
	date:    "2026-09-28"
	version: "4.3.0"
	text:    "Decisions table follows the plan format's new shape: ID | Problem | Facts | Decision | Rejected | Why | Consequences."
}, {
	date:    "2026-09-28"
	version: "4.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-26"
	version: "4.2.0"
	text:    "Route sections declare their anchor: Create Mode and Refine Mode between start..INTAKE-002 (alternatives), Nightly route replaces MODEA-001 + CLOSE-001; route lines stay in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "4.1.0"
	text:    "--nightly: Nightly route — dispatched text is the feature, no feature ask, escalations held OPEN with a nightly status note (status stays Draft), halting stops recorded in the concept and .routine-commit-body; directory slug kebab-case per the plan-dir naming rule"
}, {
	date:    "2026-09-24"
	version: "4.0.0"
	text:    "Absorbs /clarify: one skill, one file. Clarify phases (intake, triage, source diff, resolve, record, close) run inline after Create or Refine Mode, gated on not drafting_only; ids SKILL-CLARIFY-* become SKILL-CONCEPT-* keeping topic and number. Autonomy: every question is decided by the agent — researched answers recorded without confirmation, preference and business rules decided by recommendation and tagged [AGENT]; only redirecting or nuanced questions (TRIAGE-006a-c) reach the human, batched into one ask at close (CLOSE-001). Dropped: order confirmation (INTAKE-003), questions.md export and translated twin (TRIAGE-005a-d, clarify TPL-001), detailed design pages (TPL-002, MODEA-005), stories file (TPL-003, MODEA-004 — stories are a concept.md section), day estimates (WRITING-007), Closing offer entries. Model effort normalized to high."
}, {
	date:    "2026-09-18"
	version: "3.1.2"
	text:    "routing — /backlog added as optional pre-stage (disambiguation row + preconditions line)"
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kinds (Writing Guidelines/templates context, Modes split into Create/Refine route sections on refine_mode); pred/post clauses on Mode A/B steps and Closing rule/steps; atoms.json (refine_mode, specifics_provided, content_removed, removal_requested, restructure_requested, existing_read, open_questions_empty, drafting_only, clarify_offered, clarify_accepted, clarify_invoked, status_draft); queries/ (refine-blocks-create, no-clarify-when-empty, clarify-witness); caveman compression pass."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), intro trimmed to one-sentence lead"
}, {
	date:    "2026-09-05"
	version: "1.11"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-04"
	version: "1.10"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not concepting input"
}, {
	date:    "2026-09-03"
	version: "1.9"
	text:    "Overview template re-cut onto the plan catalog and declared via plan-template: TLDR/Context/Scope first, User Flows as an override section, open questions as OPEN Decisions rows indexed in Open questions, Changelog last; CLOSING-001/002 and WRITING-006 follow the new form"
}, {
	date:    "2026-09-01"
	version: "1.8"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.7"
	text:    "Plan paths migrated to the dated plan-dir convention (RULE-PLAN-071/072): creation as plans/<date>-<slug>-wip-<hash>/, resolution via glob"
}, {
	date:    "2026-08-23"
	version: "1.6"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-18"
	version: "1.5"
	text:    "body migrated to entries (23 entries, 3 of them payload templates) — content unchanged; pilot of ACDSL-SKILL-005 / rules render-skill / eval v2"
}, {
	date:    "2026-08-11"
	version: "1.4"
	text:    "declares its context set via the frontmatter context: line (ACTION-CONCEPT-*, FACT-*) — entries arrive injected at invocation"
}, {
	date:    "2026-07-19"
	version: "1.3"
	text:    "closing step chains into /concept-clarify in-session when Open Questions remain; moved under skills/concept/"
}, {
	date:    "2026-07-16"
	version: "1.2"
	text:    "concept artifacts move to plans/{slug}/concept/"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-06"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " concept"
}, {
	key:   "description"
	value: " Create, refine, and clarify a feature concept in plans/ as one concept.md, deciding open questions itself and asking only redirecting or nuanced calls. Trigger on /concept or \"draft a concept\" or \"clarify the concept\". Args — --nightly: unattended, escalations stay OPEN."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.4.4"
}, {
	key:   "title-code"
	value: " Con"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-CONCEPT-*, ACTION-IMPL-*, RULE-PLAN-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "plan-template"
	value: " SKILL-CONCEPT-TPL-001"
}, {
	key:   "argument-hint"
	value: " \"--nightly\""
}]
preamble: """
	# Concept Document Skill

	Creates, refines, and clarifies a feature concept — one file, open questions decided in the same run.

	The concept is one file, `concept/concept.md`, in the feature's plan dir — the dir sits inside a feature folder, is named
	`plans/<created_at>-<slug>-<hash>/<created_at>-<slug>-wip-<hash>/` and is resolved by glob, per the plan-dir rules in
	`context/rules/plan.md` (injected via this skill's context declaration). Drafting and question-draining are one run: the
	concept lands with its Open questions index empty, or holding only escalated rows the human has seen.

	The context entries this skill declares in its frontmatter (concepting doctrine incl. hot gates, implementing gates, repo
	facts) arrive injected at invocation — honor them without re-reading the context tree.

	All content MUST be in English.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "from"
					skill:    "backlog"
					optional: true
				}, {
					kind:     "from"
					skill:    "idea"
					optional: true
				}, {
					kind:     "from"
					skill:    "discuss"
					optional: true
				}, {
					kind:     "to"
					skill:    "fexplore"
					optional: true
				}, {
					kind:  "to"
					skill: "fdesign"
				}]
			}
			preconditions: ["none — entry point of the planning chain; backlog and idea are optional pre-stages."]
			purpose: "drafts or extends the what/why of a feature — goals, user flows, user stories, MVP/backlog cuts — and resolves its open questions into binding decisions."
			routes: [{
				for: "many unrelated ideas at once, need organizing and prioritizing"
				use: "/backlog"
			}, {
				for: "*how* to build — files, functions, contracts"
				use: "/fdesign"
			}, {
				for: "comparing solution approaches"
				use: "/fexplore"
			}, {
				for: "open questions surfacing after a plan is approved"
				use: "/fdesign <plan file> — draining here invalidates the plan"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "--nightly"
				effect: "unattended — dispatched text is the feature, escalations stay `OPEN` with a status note, never asked; see [Nightly substitution](#skill-concept-nightly-001)."
			}]
		}
	}]
}, {
	title: "Writing Guidelines"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-WRITING-006a"
			role:      "def"
			statement: "Open questions are OPEN rows in the Decisions table with lettered options and, once escalated, an owner tag."
			title:     "Open questions rows"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-006b"
			role:      "def"
			statement: "Open questions are indexed in Open questions (`Q1 → D3`) — the in-plan Q&A form of `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`."
			title:     "Open questions index"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-008"
			role:      "def"
			statement: "`[USER]` a human technical call, `[BUSINESS]` a human product call, `[AGENT]` a preference or business rule decided autonomously; researched decisions carry evidence instead of a tag."
			title:     "Decision tags"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-001"
			role:      "rule"
			statement: "Agent MUST write concise, direct prose, without filler or hedging."
			title:     "Concise prose"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-002"
			role:      "rule"
			statement: "Agent MUST use tables for structured data (thresholds, field definitions, limits)."
			title:     "Structured tables"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-003"
			role:      "rule"
			statement: "Agent MUST use numbered steps for sequential flows, indenting sub-steps for backend/system logic within a flow step."
			title:     "Numbered flows"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-004"
			role:      "rule"
			statement: "Agent MUST use concrete examples to clarify non-obvious rules or edge cases."
			title:     "Concrete examples"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-WRITING-005"
			role:      "rule"
			statement: "Agent MUST reference existing project services by name when relevant (consult `docs/context/service-map.md`)."
			title:     "Service names"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-STORIES-001"
			role:      "rule"
			statement: "Agent MUST group stories by user-facing theme, use horizontal-rule dividers between groups, keep stories independent and testable, and prefer small specific stories over broad ones."
			title:     "Story grouping"
		}
	}]
	kind: "context"
}, {
	title:   "Create Mode"
	between: "start..INTAKE-002"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-MODEA-001"
			pred:      "mode = interactive"
			role:      "step"
			statement: "ASK the user for the feature name and a description if not already provided."
			title:     "Feature intake"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-MODEA-002"
			role:      "step"
			statement: "DERIVE a directory slug (kebab-case, e.g. `login-intent`)."
			title:     "Directory slug"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-MODEA-003"
			role:      "step"
			statement: "CREATE `concept/concept.md` in the plan dir from the Overview Template; create an absent dir as `plans/<today>-{slug}-<short-HEAD>/<today>-{slug}-wip-<short-HEAD>/` (plan-dir naming rule)."
			title:     "Concept file"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-MODEA-006"
			post: ["status_draft"]
			role:      "step"
			statement: "SET status to \"Draft\"."
			title:     "Draft status"
		}
	}]
	kind: "route"
	pred: "not refine_mode"
}, {
	title:   "Refine Mode"
	between: "start..INTAKE-002"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-MODEB-003"
			pred:      "refine_mode => (not content_removed or removal_requested)"
			role:      "rule"
			statement: "Agent MUST NOT remove content the user did not ask to change."
			title:     "No removals"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-MODEB-004"
			custom: ["todo:unmeasured"]
			pred:      "refine_mode => not restructure_requested"
			role:      "quality"
			statement: "MUST preserve existing section structure unless restructuring is requested."
			title:     "Preserve structure"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-MODEB-001"
			post: ["existing_read"]
			role:      "step"
			statement: "READ the existing `concept.md` at the path the user indicates, or resolve the plan dir by glob per the plan-dir resolution rule."
			title:     "Read existing"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-MODEB-002"
			pred:      "existing_read"
			role:      "step"
			statement: "APPLY user feedback when given: expand sections, add detail, resolve open questions; a bare \"clarify\" carries none."
			title:     "Apply feedback"
		}
	}]
	kind: "route"
	pred: "refine_mode"
}, {
	title: "Clarify — Intake"
	blocks: [{
		entry: {
			id: "SKILL-CONCEPT-INTAKE-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "implicit questions are added as OPEN rows first — same treatment."
			}]
			post: ["question_list_built"]
			pred:      "not drafting_only"
			role:      "step"
			statement: "BUILD the question list: OPEN rows of the Decisions table, plus implicit ones — contradictions between sections, TBD markers, sections dodging a decision."
			title:     "Build question list"
		}
	}]
	kind: "phase"
}, {
	title: "Clarify — Triage"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-006a"
			role:      "def"
			statement: "Redirecting question: its answer changes Goals or TLDR, flips a flow in or out of scope, or overturns a prior `[USER]` decision."
			title:     "Redirecting question"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-006b"
			role:      "def"
			statement: "Nuanced question: after research no option has a defensible recommendation, or it sets a business rule with legal, money, or policy consequence the repo cannot ground."
			title:     "Nuanced question"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-006c"
			role:      "def"
			statement: "Escalation class = redirecting or nuanced questions; the only questions that reach the human."
			title:     "Escalation class"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-004a"
			pred:      "escalated => owner_tag_present"
			role:      "rule"
			statement: "Escalated questions MUST carry owner `[USER]` (technical) or `[BUSINESS]` (product rules: ranking, tiebreaks, precision, eligibility, defaults)."
			title:     "Owner tags"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-004b"
			pred:      "not escalated => agent_decides"
			role:      "rule"
			statement: "Agent MUST decide every non-escalated question itself — asking the human what research or a recommendation settles is a defect."
			title:     "Agent decides"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-TRIAGE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "classify each question — class determines how it is answered."
			}]
			pred:      "question_list_built"
			role:      "step"
			statement: "RESEARCH questions answerable from repo, project docs, or external source (API capability, library behavior, real data shape); never ask what is lookupable."
			title:     "Researchable class"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-002"
			pred:      "question_list_built"
			role:      "step"
			statement: "MARK as preference questions with no technical winner: scope, priorities, trade-offs, business rules — decided by recommendation, not asked."
			title:     "Preference class"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-TRIAGE-007"
			post: ["escalated"]
			pred:      "question_list_built"
			role:      "step"
			statement: "MARK as escalation the redirecting and nuanced questions per the definitions above; re-check the class after research — evidence often demotes a question to preference."
			title:     "Escalation class"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-TRIAGE-003"
			pred:      "question_list_built"
			role:      "step"
			statement: "MARK as obsolete questions overtaken by other decisions or dropped scope; REMOVE with a Changelog row stating the reason, never silently."
			title:     "Obsolete class"
		}
	}]
	kind: "phase"
}, {
	title: "Clarify — Source-requirements diff"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-SRCDIFF-002a"
			role:      "def"
			statement: "Proxy substitution: requirement not computable from available data, silently redefined (e.g. \"aktiv in der App\" became \"voted\")."
			title:     "Proxy substitutions"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-SRCDIFF-003"
			role:      "def"
			statement: "Requirement strengthening or weakening: concept gates or restricts more, or less, than the source says (e.g. \"werden angezeigt\" became view-gating)."
			title:     "Requirement strengthening/weakening"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-SRCDIFF-004"
			role:      "def"
			statement: "Unstated interpretation choice: place where the concept picked one reading of an ambiguous requirement without recording it."
			title:     "Unstated interpretation choices"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-SRCDIFF-002b"
			pred:      "not silent_substitution"
			role:      "rule"
			statement: "Agent MUST surface a substitution as a decision — never silently redefine a metric."
			title:     "Surface substitutions"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-SRCDIFF-001"
			role:      "step"
			statement: "When the concept derives from a source requirements document (often German): RE-READ the source line-by-line against the concept; flag drift as findings."
			title:     "Diff pass"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-SRCDIFF-005"
			pred:      "question_list_built"
			role:      "step"
			statement: "JOIN findings into the question list with the same triage treatment."
			title:     "Findings join list"
		}
	}]
	kind: "phase"
}, {
	title: "Clarify — Resolve"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-RESOLVE-002b"
			pred:      "answer_recorded => (evidence_present or owner_tag_present or agent_tag_present)"
			role:      "rule"
			statement: "A recorded answer MUST carry evidence (file:line, doc link, inspected data) or a decision tag with rationale; a proposal without either is opinion."
			title:     "Evidence required"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RESOLVE-001"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "List MUST be worked highest-leverage first — an early answer often kills later questions — batching questions with independent answers."
			title:     "Work order"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RESOLVE-003b"
			custom: ["todo:unmeasured"]
			pred:      "human_answered => human_answer_binding"
			role:      "quality"
			statement: "A human's answer MUST be treated as binding; if the chosen option balloons, ask again — never silently demote."
			title:     "Binding human answer"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RESOLVE-002a"
			post: ["evidence_present", "answer_recorded"]
			pred:      "question_list_built"
			role:      "step"
			statement: "For researchable questions: GATHER evidence with sources (file:line, doc link, inspected real data); RECORD the answer directly — no confirmation round."
			title:     "Researchable resolve"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RESOLVE-003a"
			post: ["agent_tag_present", "answer_recorded"]
			pred:      "question_list_built"
			role:      "step"
			statement: "For preference questions: DECIDE by the recommended option; RECORD it `[AGENT]`-tagged with the alternatives considered in Why."
			title:     "Preference resolve"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RESOLVE-005"
			post: ["escalation_pending"]
			pred:      "escalated"
			role:      "step"
			statement: "For escalation-class questions: HOLD the row OPEN with lettered options, a marked recommendation, and what the answer changes; never ask mid-run."
			title:     "Escalation hold"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RESOLVE-004a"
			post: ["verified_against_repo", "answer_recorded"]
			pred:      "unprompted_answer"
			role:      "verify"
			statement: "Before recording a human's unprompted answer: VERIFY it against repo reality; reconcile any mismatch before recording."
			title:     "Directly answered"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-RESOLVE-004b"
			pred:      "contradicts_prior_decision"
			role:      "stop"
			statement: "If a human's answer contradicts prior decisions ⇒ STOP: surface immediately, never silently record or correct."
			title:     "Contradicting answer"
		}
	}]
	kind: "phase"
}, {
	title: "Clarify — Record & ripple"
	blocks: [{
		entry: {
			id: "SKILL-CONCEPT-RECORD-001a"
			post: ["decision_recorded"]
			pred:      "answer_recorded"
			role:      "step"
			statement: "CONVERT each resolved question's OPEN row to a decision with rationale; tag `[AGENT]`, `[USER]`, or `[BUSINESS]` per the decision-tag definition."
			title:     "Convert row"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RECORD-001b"
			post: ["question_closed"]
			pred:      "decision_recorded"
			role:      "step"
			statement: "DROP its Open-questions pointer; append a Changelog row."
			title:     "Drop and log"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RECORD-002a"
			post: ["decision_recorded"]
			pred:      "chat_only_decision"
			role:      "step"
			statement: "WRITE every decision reached in chat into the concept the same session — even a single-field call never listed as an open question; chat-only decisions resurface as review findings."
			title:     "Write chat decisions"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RECORD-003"
			post: ["decision_rippled"]
			pred:      "decision_recorded"
			role:      "step"
			statement: "RIPPLE every decision into affected sections: user flows, user stories, scope. A decision recorded but not rippled leaves the concept self-contradictory — ripple IS the work."
			title:     "Ripple decisions"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-RECORD-005"
			post: ["status_note_added"]
			pred:      "escalation_pending"
			role:      "step"
			statement: "KEEP escalated rows OPEN under their number plus a status note: the recommendation and what the answer changes."
			title:     "Status note"
		}
	}]
	kind: "phase"
}, {
	title: "Clarify — Close"
	blocks: [{
		entry: {
			id: "SKILL-CONCEPT-CLOSE-001"
			post: ["escalation_asked"]
			pred:      "escalation_pending and mode = interactive"
			role:      "step"
			statement: "ASK all escalated questions in one message: per question the lettered options, the recommendation, what the answer changes; the human's answers go through the record phase as `[USER]` or `[BUSINESS]`."
			title:     "Ask escalations"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-CLOSE-002"
			post: ["status_in_review"]
			pred:      "open_index_empty"
			role:      "step"
			statement: "BUMP status Draft → In Review when the Open questions index is empty; append a Changelog row."
			title:     "Status bump"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-CLOSE-003"
			role:      "step"
			statement: "REPORT in chat: every `[AGENT]` decision taken (id plus one line), escalations pending, status — the human overrules an `[AGENT]` row through Refine Mode."
			title:     "Report"
		}
	}]
	kind: "phase"
}, {
	title: "Self-check gate"
	blocks: [{
		entry: {
			id: "SKILL-CONCEPT-GATE-001"
			custom: ["todo:unmeasured"]
			pred:      "decision_recorded => (evidence_present or owner_tag_present or agent_tag_present)"
			role:      "quality"
			statement: "Every new Decision MUST have rationale, plus an evidence source or a decision tag."
			title:     "Rationale + source"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-GATE-002"
			custom: ["todo:unmeasured"]
			pred:      "not contradiction_found"
			role:      "quality"
			statement: "No section MUST contradict a new decision — checked by re-reading the concept, not from memory."
			title:     "No contradiction"
		}
	}, {
		entry: {
			id: "SKILL-CONCEPT-GATE-004"
			custom: ["todo:unmeasured"]
			pred:      "open_index_empty or escalation_asked"
			role:      "quality"
			statement: "The Open questions index MUST be empty — or every surviving OPEN row MUST be escalation class, asked, with a status note."
			title:     "Index drained"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-GATE-005"
			pred:      "not open_index_empty"
			role:      "stop"
			statement: "If the Open questions index is not empty ⇒ STOP: do not bump status from Draft to In Review."
			title:     "Status bump gated"
		}
	}]
	kind: "context"
}, {
	title: "Stop conditions"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-STOP-001"
			role:      "rule"
			statement: "Gate entries of the implementing chapter (`$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md`, injected via this skill's context declaration) MUST apply."
			title:     "Implementing gates apply"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-STOP-002"
			pred:      "goal_invalidated"
			role:      "stop"
			statement: "If evidence invalidates the concept's Goals or a prior `[USER]` decision ⇒ STOP: report it immediately, not at close; re-scoping is the user's call, not ripple."
			title:     "Goal invalidation"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-STOP-003"
			pred:      "rejection_count >= 2"
			role:      "stop"
			statement: "If two proposals for the same question are rejected ⇒ STOP: proposing; present the trade-off with evidence for each side, let the user decide."
			title:     "Two rejections"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-STOP-004"
			pred:      "false_assumption_found"
			role:      "stop"
			statement: "If research shows a whole user flow built on a false assumption ⇒ STOP: that is a redesign, the user's call, not clarification."
			title:     "False assumption"
		}
	}]
	kind: "context"
}, {
	title: "Nightly route"
	blocks: [{
		entry: {
			id:        "SKILL-CONCEPT-NIGHTLY-001"
			role:      "def"
			statement: "`--nightly` replaces the human gates named below, nothing else; every other entry applies unchanged."
			title:     "Nightly substitution"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-NIGHTLY-002"
			role:      "rule"
			statement: "A nightly run MUST NOT ask or wait — [Feature intake](#skill-concept-modea-001) and [Ask escalations](#skill-concept-close-001) never fire."
			title:     "Never prompt"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-NIGHTLY-003"
			role:      "step"
			statement: "TAKE the text after the args — `B-NNN: <title>`, description — as feature name (the title) and description."
			title:     "Dispatched request"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-NIGHTLY-004"
			role:      "step"
			statement: "HOLD escalated rows `OPEN` per [Escalation hold](#skill-concept-resolve-005), status note `nightly — awaiting review`; status stays Draft."
			title:     "Held escalations"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-NIGHTLY-005"
			role:      "step"
			statement: "WRITE `.routine-commit-body`: concept path, `[AGENT]` decision count, escalated row pointers."
			title:     "Commit body"
		}
	}, {
		entry: {
			id:        "SKILL-CONCEPT-NIGHTLY-006"
			role:      "stop"
			statement: "If [Goal invalidation](#skill-concept-stop-002) or [False assumption](#skill-concept-stop-004) fires ⇒ STOP: record it under Open questions and in `.routine-commit-body`, end the run."
			title:     "Halt record"
		}
	}, {
		prose: "---"
	}]
	kind:     "route"
	pred:     "mode = nightly"
	replaces: "MODEA-001 + CLOSE-001"
}, {
	title: "Overview Template"
	blocks: [{
		entry: {
			id: "SKILL-CONCEPT-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "Overview Template — the structure of `concept.md`, the single concept file, on the plan catalog. The status blockquote stays above TLDR."
			title:     "Overview template"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "concept/user-story drafting, product judgment, turning open questions into binding decisions"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {
		check: ["test", "-s", ".routine-commit-body"]
	}
	invoke: "--nightly B-001: smoke-greet — a CLI flag --greet that prints hello on stdout; single user, no config, no persistence"
	limits: {
		minutes: 25
		usd:     5.0
	}
}
