changelog: [{
	date:    "2026-10-17"
	version: "3.7.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  2981
}, {
	date:    "2026-10-09"
	version: "3.6.0"
	text:    "Runtime placement declared: root-only (Workflow stages), owns acceptEdits; a plan-mode session switches to acceptEdits through the Desktop session tool before the Workflow gate — the gate still stops headless and restricted sessions."
	tokens:  3156
}, {
	date:    "2026-10-04"
	version: "3.5.0"
	text:    "Requires context: deploy skips the skill without them."
	tokens:  3086
}, {
	date:    "2026-10-04"
	version: "3.4.2"
	text:    "Dropped the serena arg — Serena support removed repo-wide, goland stays the one IDE opt-in; the standing bootstrap is named routine bootstrap."
	tokens:  2991
}, {
	date:    "2026-09-29"
	version: "3.4.1"
	text:    "workflow spawns now name a model through the generated GEN:models vocabulary; absent model ⇒ default subagent model"
	tokens:  3086
}, {
	date:    "2026-09-28"
	version: "3.4.0"
	text:    "repo: reads registered repos through Read(@repos/**) — no global additional directories; unregistered paths stop with a Repos-page steer."
}, {
	date:    "2026-09-28"
	version: "3.3.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-27"
	version: "3.3.2"
	text:    "Declared Delegation: never — Workflow fan-out, root session only."
}, {
	date:    "2026-09-27"
	version: "3.3.1"
	text:    "Bracket and arm tokens use <model>:<effort>"
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; the workflow takes ide[] and puts the opt-in into every miner and verifier prompt; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-24"
	version: "3.2.0"
	text:    "repo arg: foreign-checkout path passed into the workflow and prefixed onto verifier prompts — first foreign run (worktrunk, 6 chunks x 3 lenses, opus-high, 42 cells, 4.79M subagent tokens, 56 min) had verifiers reading the cwd and refuting 40 of 44 candidates as 'no Rust code in this repo'."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind clauses (context/phase), Definitions/Global Rules split out of Procedure, pred/post clauses on rule/stop/step/spawn entries, param-register bullets, sibling atoms.json (8 bool atoms) and two queries (gate-blocks-run unsat, full-path-witness sat), caveman compression pass."
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
	text:    "Format-v2 reformat: When to use folded into Routing (Purpose/Disambiguation/Preconditions/Pipeline), Args promoted to a 4-column table, argument-hint added, allowed-tools order normalized, caveman prose pass."
}, {
	date:    "2026-09-05"
	version: "1.1"
	text:    "Entries conformed to the titled grammar — **Title** — statement headlines, 300-rune cap with detail moved to Why/Applies bullets"
}, {
	date:    "2026-09-05"
	version: "1.0"
	text:    "Initial: experimental whole-repo miner — dimensions x chunks Workflow fan-out (history dimension via history: n), merge + verify, candidate-entry report; apply via context-create."
}]
enableDelegation: "never"
enforcePermissions: ["acceptEdits"]
frontmatter: [{
	key:   "name"
	value: " context-bootstrap"
}, {
	key:   "description"
	value: " EXPERIMENTAL whole-repo context miner — fan out dimension-by-dimension over repo chunks and report candidate ACDSL context entries with band and evidence. Trigger on /context-bootstrap only — expensive, never self-invoked. Args — dimensions: lens override; chunks: mode[maxFiles:maxLines]; paths: pathspec; lanes: n[model:effort]; history: n commits; repo: foreign checkout path."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.7.0"
}, {
	key:   "title-code"
	value: " CtxBoot"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Read, Read(@repos/**), TaskOutput, ToolSearch, Workflow, Write, Bash(git log*), Bash(git ls-files*), Bash(git rev-parse*), Bash(jq *)"
}, {
	key:   "argument-hint"
	value: " 'dimensions: facts pitfalls chunks: layers[30:6000] paths: internal/ cmd/ lanes: 2[opus:medium] history: 500 repo: ~/GolandProjects/peek-mcp'"
}]
preamble: """
	# Context Bootstrap

	Mines a whole repo — code, not session transcripts — into candidate ACDSL context entries.

	Experimental. Works like railroad-review reviewing a diff: independent dimensions (lenses), fanned over repo chunks, merged and
	verified. Output is a report of candidates in full entry grammar; nothing gets authored — accepted candidates go through
	/context-create one by one. Contrast: the standing bootstrap (`routine bootstrap`) seeds a machine from transcripts; this
	seeds a repo's context from the repo itself.
	"""
requires: ["context"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "from"
					skill:    "skillroutine-estimate"
					optional: true
				}, {
					kind:  "to"
					skill: "context-create"
				}]
				note: "its report candidates are context-create inputs"
			}
			preconditions: ["interactive non-plan-mode session with the Workflow tool (hard gate)", "clean-enough tree — `git ls-files` reflects the repo", "invoker accepts the cost (estimate gate)", "`repo` (when given) is a registered repo other than this one — the skill's `@repos` grant admits registered repos only"]
			purpose: "mines a repo's own code into candidate context entries — independent lenses fanned over chunks, merged and verified."
			routes: [{
				for: "findings from real usage"
				use: "the smine pipeline"
			}, {
				for: "authoring one known entry"
				use: "/context-create"
			}, {
				for: "reviewing a change"
				use: "/railroad-review"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "dimensions"
				default: "all three"
				effect:  "overrides the lens set — see [Dimensions](#dimensions)"
				form:    "lens set, e.g. `facts pitfalls`"
			}, {
				name:    "chunks"
				default: "`layers[30:6000]`"
				effect:  "partition mode: layers/languages/pathspec groups — [Chunk derivation](#skill-contextbootstrap-chunk-001a)"
				form:    "`<mode>[maxFiles:maxLines]`"
			}, {
				name:    "paths"
				default: "whole repo"
				effect:  "narrows the mined set"
				form:    "pathspec"
			}, {
				name:    "lanes"
				default: "`1`"
				effect:  "miner lanes per dimension x chunk; bracket also governs mergers/verifiers"
				form:    "`<n>[<model>:<effort>]`"
			}, {
				name:    "history"
				default: "absent, dimension off"
				effect:  "mines the last n commits as an extra dimension, 40/chunk — [Chunk derivation](#skill-contextbootstrap-chunk-001a)"
				form:    "`<n>` commits"
			}, {
				name:    "repo"
				default: "cwd"
				effect:  "registered foreign checkout to mine, live tree; chunk paths and verifier evidence resolve there — [Run workflow](#skill-contextbootstrap-run-001)"
				form:    "absolute path"
			}]
		}
	}]
}, {
	title: "Dimensions"
	blocks: [{
		prose: """
			| Dimension | Lens | Produces |
			|---|---|---|
			| facts | architecture and world facts an agent can't cheaply re-derive: entry points, data flow, deployment, invariants, cross-system contracts | FACT candidates |
			| rules | recurring artifact-shape conventions followed in the code: naming, error shape, test skeletons, file layout | RULE candidates, band-classified |
			| pitfalls | integrity-critical spots and repo-specific traps: validation/transaction patterns, ordering constraints, things a naive edit breaks | ACTION candidates, band-classified |
			| history (arg-activated) | repeated fixes, reverted approaches, conventions established over time in the last n commits — mined from `git log --stat`/`-p` per commit-range chunk | FACT/ACTION candidates, evidenced by commit hashes + file:line |
			"""
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id:        "SKILL-CONTEXTBOOTSTRAP-EST-001a"
			role:      "def"
			statement: "Cells = sum over dimensions of (chunks x lanes) miners + 1 merger/dimension + 1 cross-merger + ceil(candidates/4) verifiers (verifier count estimated at ~8 candidates/dimension)."
			title:     "Cell math"
		}
	}]
	kind: "context"
}, {
	title: "Global Rules"
	blocks: [{
		entry: {
			id:        "SKILL-CONTEXTBOOTSTRAP-REPORT-002a"
			pred:      "not context_edited"
			role:      "rule"
			statement: "This skill MUST NOT edit any context doc, rules.acdsl, or registry — report only; candidates NEVER bypass authoring."
			title:     "No edits"
		}
	}, {
		entry: {
			id:        "SKILL-CONTEXTBOOTSTRAP-REPORT-002b"
			role:      "rule"
			statement: "Application MUST run per-candidate /context-create, in the repo that owns the context surface (reach-tagged for foreign repos)."
			title:     "Application path"
		}
	}]
	kind: "context"
}, {
	title: "Procedure"
	blocks: [{
		entry: {
			id: "SKILL-CONTEXTBOOTSTRAP-GATE-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the call: `mcp__ccd_session_mgmt__set_session_permission_mode` with `session_id: \"self\"`, `mode: \"acceptEdits\"`; continue once it answers."
			}]
			post: []
			pred:      "plan_mode and mode_switch_tool"
			role:      "step"
			statement: "SWITCH a plan-mode session to acceptEdits through the Desktop session tool — the Workflow tool is denied in plan mode."
			title:     "Plan-mode switch"
		}
	}, {
		entry: {
			id:        "SKILL-CONTEXTBOOTSTRAP-GATE-001"
			pred:      "not workflow_tool_available"
			role:      "stop"
			statement: "No Workflow tool (headless, restricted, plan mode with no Desktop tool) → STOP: \"context-bootstrap needs the Workflow tool\". Never fall back to sequential in-session mining, never to the Agent tool."
			title:     "Workflow gate"
		}
	}, {
		entry: {
			id:        "SKILL-CONTEXTBOOTSTRAP-CHUNK-000"
			pred:      "repo_given and repo_denied"
			role:      "stop"
			statement: "If `git -C <repo> rev-parse --show-toplevel` is denied or fails ⇒ STOP: `repo` must be a registered repo other than this one; register it on the Repos page, or run from a session inside it."
			title:     "Unregistered repo"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTBOOTSTRAP-CHUNK-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "with `history: <n>`: also derive commit-range chunks from `git log --format=%H -n <n>`, 40 commits per chunk — history dimension only."
			}]
			post: ["chunks_derived"]
			role:      "step"
			statement: "DERIVE chunks before any fan-out: mined set = `git ls-files` minus vendor/generated/lockfiles/binary, narrowed by `paths`; partition per the chunks mode under the bounds."
			title:     "Chunk derivation"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTBOOTSTRAP-CHUNK-001b"
			post: ["chunk_list_stated"]
			pred:      "chunks_derived"
			role:      "step"
			statement: "STATE the chunk list (id, file/commit count, line count) in chat."
			title:     "State chunk list"
		}
	}, {
		entry: {
			id:        "SKILL-CONTEXTBOOTSTRAP-EST-001"
			pred:      "not go_received"
			role:      "stop"
			statement: "If no explicit go received ⇒ STOP: PRESENT cell count, cost range, 5-hour window fit, and wait; NEVER spawn same-turn."
			title:     "Estimate gate"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTBOOTSTRAP-RUN-001"
			items: [{
				fence: {
					body: """
						dimensions: [{key, prompt, chunks: [{id, spec}]}]
						laneModel?
						laneEffort?
						lanes
						repo?

						"""
					info:   "text"
					marker: "```"
				}
			}, {
				label:  "param"
				marker: "*"
				text:   "`repo` (foreign checkout) is passed through so verifiers resolve evidence there; dimension prompts name the same path."
			}, {
				label:  "param"
				marker: "*"
				text:   "every dimension carries its own chunk list — code dimensions share the file chunks, history its commit-range chunks."
			}, {
				label:  "param"
				marker: "*"
				text:   "each chunk's `spec` is the mining instruction text: file list, or `git log -p <range>` commands for a commit range."
			}, {
				label:  "param"
				marker: "*"
				text:   "dimension prompts carry the lens text from the Dimensions table."
			}, {
				label:  "param"
				marker: "*"
				text:   "candidates: entry-grammar statements with band, applies/location and >=2 evidence points (file:line; history adds commit hash)."
			}, {
				label:  "param"
				marker: "*"
				text:   "prompts are read-only; never propose entries restating what the code trivially shows."
			}]
			post: ["workflow_spawned"]
			pred:      "go_received"
			role:      "spawn"
			statement: "RUN the bundled workflow (`workflows/context-bootstrap.js` in this skill's deployed dir) with the args object:"
			title:     "Run workflow"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTBOOTSTRAP-REPORT-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "per candidate: full entry-grammar block (ID left as `<KIND>-<SCOPE>-???`, numbering happens at create time), band, evidence file:line list."
			}, {
				label:  "param"
				marker: "*"
				text:   "per F/A candidate: verifier sketch + anchor, plus the verify agent's verdict."
			}]
			post: ["report_written"]
			pred:      "workflow_spawned"
			role:      "step"
			statement: "WRITE verified candidates to `<scratchpad>/context-bootstrap-report-<date>.md`, grouped by dimension."
			title:     "Write report"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTBOOTSTRAP-REPORT-001b"
			post: ["report_delivered"]
			pred:      "report_written"
			role:      "step"
			statement: "DELIVER in chat with per-candidate apply lines (`/context-create rule: ... gate: ...`); refuted candidates listed with the refutation, never dropped silently."
			title:     "Deliver report"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "lens quality decides everything; weak miners produce noise that costs more to triage than to regenerate."
			tier:   "frontier"
			notes: ["Experimental — not yet measured; record real cells and cost in changelog entries as runs happen."]
		}
	}]
}]
atoms: {
	plan_mode: {
		sort:   "state"
		source: "session"
	}
}
smoke: {
	skip: "interactive session required and an explicit go at the estimate gate precedes every spawn"
}
