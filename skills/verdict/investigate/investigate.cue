changelog: [{
	date:    "2026-10-17"
	version: "3.3.1"
	text:    "Model file is <leaf>.cue"
	tokens:  3174
}, {
	date:    "2026-10-09"
	version: "3.3.0"
	text:    "Runtime placement declared: root-only (Workflow stages)."
	tokens:  3106
}, {
	date:    "2026-10-09"
	version: "3.2.1"
	text:    "the relay copies the refuted register and open measurements verbatim from the merge return; the docs/ baseline write runs on every baseline, one-fact answers included, never skipped or asked"
}, {
	date:    "2026-09-30"
	version: "3.2.0"
	text:    "the final baseline is written to docs/<topic>-investigation.md — the durable deliverable; scratchpad artifacts stay ephemeral"
	tokens:  3106
}, {
	date:    "2026-09-29"
	version: "3.1.5"
	text:    "workflow spawns now name a model through the generated GEN:models vocabulary; absent model ⇒ default subagent model"
}, {
	date:    "2026-09-28"
	version: "3.1.4"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.1.3"
	text:    "argument-hint example artifactsDir moved from /tmp to the non-repository artifacts root ~/.cache/claude-routine/artifacts — /tmp writes are denied with a scratchpad steer."
}, {
	date:    "2026-09-27"
	version: "3.1.2"
	text:    "Declared Delegation: never — Workflow fan-out, root session only."
}, {
	date:    "2026-09-24"
	version: "3.1.1"
	text:    "routing — /research named as the single-agent sibling"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: `## Rules` marked context section; pred/post clauses on the four global rules and ten flow entries (Workflow Call through Fold Results); sibling atoms.json (17 bool atoms); three queries (primary-source invariant, measurement-gate reach-block, clean-finish witness); caveman-compressed prose."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "format-v2 reformat: When to use → Routing (Purpose/Disambiguation table/Preconditions/Pipeline), Args as table with title-links to owning entries, argument-hint made a realistic example invocation"
}, {
	date:    "2026-09-05"
	version: "1.9"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.8"
	text:    "explicit model pinning — new optional model arg pins the fan-out, verifier and merge agents to a named tier (default frontier/opus) in workflows/investigate.js instead of inheriting the caller session model, so an investigate launched from a weaker session no longer silently degrades"
}, {
	date:    "2026-08-23"
	version: "1.7"
	text:    "renamed investigation to investigate, moved into the verdict group, workflow file renamed to investigate.js; added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.6"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-20"
	version: "1.5"
	text:    "artifactsDir optional — defaults to <session scratchpad>/investigation, created by the skill front; workflow validation unchanged"
}, {
	date:    "2026-07-26"
	version: "1.4"
	text:    "artifacts moved out of the repo — workflow arg outputDir (defaulted to sessions/investigations) replaced by required absolute artifactsDir, rejected if missing/non-absolute; makes \"read-only toward the repo\" enforced"
}, {
	date:    "2026-07-26"
	version: "1.3"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "1.2"
	text:    "Stage B re-verification parallelized — claim dedup + sharded verifiers (batch ≤8, effort medium); serial single-verifier removed"
}, {
	date:    "2026-07-19"
	version: "1.1"
	text:    "workflow tolerates args delivered as a JSON-encoded string (harness stringification)"
}, {
	date:    "2026-07-19"
	version: "1.0"
	text:    "initial version"
}]
enableDelegation: "never"
frontmatter: [{
	key:   "name"
	value: " investigate"
}, {
	key:   "description"
	value: " Fan out N independent investigations of one question, then merge adversarially with primary-source re-verification. Trigger on /investigate or \"does this analysis hold up\". Args — question: the shared question; inputs: fetchable primary sources; artifactsDir: session scratchpad default; priorArtifact: prior analysis as #1; investigators: width (default 3); model: run tier, frontier default; resumeFromRunId: fold measurement results back in."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.3.1"
}, {
	key:   "title-code"
	value: " Inv"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " 'question: \"why did signup conversion drop 12% in week 34\" inputs: \"s3://analytics/signup_funnel.csv, https://dashboard.internal/funnel\" artifactsDir: ~/.cache/claude-routine/artifacts/investigate-signup priorArtifact: plans/prior-analysis.pdf investigators: 4 model: opus resumeFromRunId: run-2026-09-01'"
}]
preamble: """
	# Investigation Adversarial Merge

	Fan out N independent investigations of one question, merge adversarially: later analysis re-verifies every prior
	load-bearing claim against **actual primary source** — files, spreadsheets, live/archived URLs, prod query results —
	refutes what fails, emits one baseline with refuted-hypotheses register. Nobody re-investigates dead ends.

	Skill-fronts-Workflow: skill resolves question, inputs, fan-out width; `investigate` workflow (`workflows/investigate.js`,
	this skill's dir) runs deterministic fan-out, re-verification barrier, merge.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["the question is stated", "shared inputs reachable (files, URLs, query targets both investigators and verifier can fetch) — verification primary-source-only"]
			purpose: "adjudicates one shared open question (or handed-in prior analysis, entering as investigation #1) via independent fan-out plus adversarial primary-source re-verification."
			routes: [{
				for: "diff review vs conventions/specs, verified against source code (same fan-out→verify→merge shape, different source)"
				use: "/railroad-review"
			}, {
				for: "single verified root-cause diagnosis"
				use: "/diagnose-debug"
			}, {
				for: "single-pass grounded lookup, one agent, citations"
				use: "/research"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "question"
				default: "required"
				effect:  "the one shared open question — [Question And Inputs](#skill-investigate-intake-001)"
				form:    "free text"
			}, {
				name:    "inputs"
				default: "required"
				effect:  "shared primary-source inputs, never prose descriptions — [Question And Inputs](#skill-investigate-intake-001)"
				form:    "fetchable paths/URLs/query targets"
			}, {
				name:    "artifactsDir"
				default: "`<scratchpad>/investigation`"
				effect:  "where investigator artifacts land, outside every worktree — [Artifacts Dir](#skill-investigate-run-002b)"
				form:    "absolute path"
			}, {
				name:    "priorArtifact"
				default: "none"
				effect:  "prior analysis entering as investigation #1, re-verified alongside fresh runs — [Prior Artifact](#skill-investigate-intake-002)"
				form:    "doc path"
			}, {
				name:    "investigators"
				default: "`3` (cap 8)"
				effect:  "fan-out width — [Fan-out Width](#skill-investigate-intake-003)"
				form:    "integer"
			}, {
				name:    "model"
				default: "`opus`"
				effect:  "run tier for fan-out/verify/merge agents, never inherited from caller — [Optional Args](#skill-investigate-run-002c)"
				form:    "model tier"
			}, {
				name:    "resumeFromRunId"
				default: "none"
				effect:  "folds measurement-gate results back in, replaying the unchanged prefix from cache — [Measurement Gate](#skill-investigate-report-002)"
				form:    "run id"
			}]
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-INVESTIGATE-RULES-001"
			pred:      "primary_source_unreachable => (measurement_needed and not claim_holds)"
			role:      "rule"
			statement: "Verification MUST use primary sources only: claim whose primary artifact unreachable stays `needs-measurement`, MUST NOT silently `holds`."
			title:     "Primary Sources Only"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-RULES-002"
			pred:      "not sibling_read"
			role:      "rule"
			statement: "Investigators MUST stay independent — no reading sibling runs, no anchoring on prior conclusion (incl. investigation #1's)."
			title:     "Independent Investigators"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-RULES-003"
			pred:      "web_access"
			role:      "rule"
			statement: "Investigators and verifier MUST have web access — external fetches (public files, Wayback) part of contract."
			title:     "External Fetches"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-RULES-004"
			pred:      "not repo_write"
			role:      "rule"
			statement: "Investigators and verifier MUST treat repo read-only; the only repo write is the root's baseline doc under `docs/`, never code changes."
			title:     "Read-only Repo"
		}
	}]
	kind: "context"
}, {
	title: "1. Intake"
	blocks: [{
		entry: {
			id: "SKILL-INVESTIGATE-INTAKE-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "verification primary-source-only — inputs must be fetchable (paths, live/archived URLs, prod-query targets), not prose."
			}]
			role:      "def"
			statement: "One shared question plus shared inputs — dataset plus every source ref investigator needs."
			title:     "Question And Inputs"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-INTAKE-002"
			role:      "def"
			statement: "Optional prior analysis artifact entering as investigation #1 — ranked hypotheses and load-bearing claims extracted, re-verified alongside fresh runs."
			title:     "Prior Artifact"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-INTAKE-003"
			role:      "def"
			statement: "Fan-out width `investigators` (default 3, cap 8) — independent runs attacking question. Wider only when hypothesis space genuinely open."
			title:     "Fan-out Width"
		}
	}]
}, {
	title: "2. Run"
	blocks: [{
		entry: {
			id:        "SKILL-INVESTIGATE-RUN-002a"
			role:      "def"
			statement: "`question` and `inputs` are required strings."
			title:     "Required Args"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-RUN-002c"
			role:      "def"
			statement: "`priorArtifact`, `investigators`, `model` optional (`model` defaults frontier tier, never caller session model)."
			title:     "Optional Args"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RUN-001"
			items: [{
				fence: {
					body: """
						{question, inputs, artifactsDir, priorArtifact, investigators, model}

						"""
					marker: "```"
				}
			}]
			post: ["workflow_spawned"]
			pred:      "question_given and inputs_given"
			role:      "spawn"
			statement: "SPAWN Workflow tool: `scriptPath: '<skill base dir>/workflows/investigate.js'`, args below — base dir stated at skill load; args real JSON object, never string."
			title:     "Workflow Call"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RUN-002b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "workflow rejects missing/non-absolute value — no artifact lands in repo."
			}]
			post: ["artifacts_dir_resolved"]
			pred:      "workflow_spawned"
			role:      "step"
			statement: "RESOLVE `artifactsDir` — absolute path outside every worktree; caller gives none ⇒ resolve session scratchpad dir, `mkdir -p <scratchpad>/investigation`, pass that path."
			title:     "Artifacts Dir"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RUN-003a"
			post: ["hypotheses_built"]
			pred:      "workflow_spawned and artifacts_dir_resolved"
			role:      "substep"
			statement: "RUN Stage A (Fan-out): `investigators` independent agents each read inputs first, build ranked hypotheses, name primary source for every load-bearing claim."
			title:     "Stage A Fan-out"
		}
	}, {
		entry: {
			id:        "SKILL-INVESTIGATE-RUN-003b"
			role:      "substep"
			statement: "NEVER read sibling runs during Stage A fan-out (anti-anchoring)."
			title:     "Stage A Anti-anchoring"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RUN-004a"
			post: ["claims_deduped"]
			pred:      "hypotheses_built"
			role:      "substep"
			statement: "RUN Stage B (Re-verify) as barrier: dedupe claims, parallel verifier shards fetch each claim's actual primary artifact — never prior run's summary."
			title:     "Stage B Re-verify"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RUN-004b"
			post: ["claims_ruled"]
			pred:      "claims_deduped"
			role:      "substep"
			statement: "RULE each claim `holds | refuted | needs-measurement` in Stage B, watching full funnel."
			title:     "Stage B Rulings"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RUN-005"
			post: ["baseline_emitted"]
			pred:      "claims_ruled"
			role:      "substep"
			statement: "RUN Stage C (Merge): emit single baseline — fact-vs-hypothesis labeling, refuted-hypotheses register, credited net-new findings, discrimination plan."
			title:     "Stage C Merge"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-RULES-005"
			post: ["artifacts_ingested"]
			pred:      "baseline_emitted"
			role:      "substep"
			statement: "INGEST every surviving artifact in Stage C — sibling outputs never assumed cross-pollinated."
			title:     "Stage C Ingestion"
		}
	}]
}, {
	title: "3. Report & the measurement gate"
	blocks: [{
		entry: {
			id: "SKILL-INVESTIGATE-REPORT-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "discrimination plan — one variable per cell, control arm, pre-registered failure criteria."
			}, {
				label:  "param"
				marker: "*"
				text:   "count and entries copied from the merge return's `refuted_register` and `open_measurements` fields, never restated from memory."
			}]
			post: ["baseline_relayed"]
			pred:      "baseline_emitted"
			role:      "step"
			statement: "RELAY merged baseline (each statement labeled fact vs hypothesis), net-new findings per run, discrimination plan, and the merge return's refuted register and open measurements verbatim."
			title:     "Relay Baseline"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-REPORT-002a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "results concrete queries user runs in prod."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "subagent can't pause mid-run for result — workflow returns these claims as `open_measurements`."
			}]
			pred:      "measurement_needed"
			role:      "stop"
			statement: "If claims are `needs-measurement` ⇒ STOP: halt the merge until the user supplies binding results."
			title:     "Measurement Gate"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-REPORT-002b"
			post: ["measurement_folded"]
			pred:      "measurement_needed"
			role:      "step"
			statement: "SURFACE queries, collect user's results as **binding facts**, FOLD in — re-invoke `resumeFromRunId` so unchanged prefix replays from cache, only merge re-runs."
			title:     "Fold Results"
		}
	}, {
		entry: {
			id: "SKILL-INVESTIGATE-REPORT-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "scratchpad artifacts are ephemeral — the doc is the durable deliverable; commit it via /package-commit."
			}, {
				label:  "param"
				marker: "*"
				text:   "unconditional — no skip, no ask the user."
			}]
			pred:      "baseline_emitted"
			role:      "step"
			statement: "WRITE every baseline, a one-fact answer included, to `docs/<topic>-investigation.md` — English, brief bullets, facts vs hypotheses, refuted register, open measurements; rewrite after each fold."
			title:     "Baseline Doc"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "intake + deterministic workflow fan-out; hypothesis-building, primary-source re-verification, merge happen in cells and high-effort synthesizers"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "question: \"which make target regenerates a skill's SKILL.md from its <leaf>.cue\" inputs: \"Makefile\" investigators: 2 model: sonnet"
	limits: {
		minutes: 20
		usd:     5.0
	}
}
