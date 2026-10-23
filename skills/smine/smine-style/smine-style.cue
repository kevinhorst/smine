changelog: [{
	date:    "2026-10-14"
	version: "4.5.0"
	text:    "Interfaces: one variable per capability — ROUTINE_PUBLISH_STYLE"
	tokens:  4081
}, {
	date:    "2026-10-13"
	version: "4.4.0"
	text:    "The operator's publish command is the env variable $SKILL_SMINESTYLE_ROUTINE (required) in the write rule and the report item."
	tokens:  4055
}, {
	date:    "2026-10-09"
	version: "4.3.0"
	text:    "Runtime placement declared: root-only (Workflow stages)."
	tokens:  4023
}, {
	date:    "2026-10-07"
	version: "4.2.0"
	text:    "Readers run as the bundled workflows/read.js: sonnet voice readers over batches of 10 selected session ids plus one taste reader, no nested spawns, schema-typed abstracts without verbatim text; selection drops routine, pipeline and single-turn sessions from one session_list call; the taste count is the summed sourcesActuallyRead. New args model and batch."
}, {
	date:    "2026-10-04"
	version: "4.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  3472
}, {
	date:    "2026-10-04"
	version: "4.0.6"
	text:    "Description names routine bootstrap, which replaces the bootstrap wrapper script."
	tokens:  3472
}, {
	date:    "2026-10-02"
	version: "4.0.5"
	text:    "The repo's store paths hang off `<store>`, the path `make rules ARGS=\"store-root\"` prints; the plans archive stays a literal path."
	tokens:  3472
}, {
	date:    "2026-10-02"
	version: "4.0.4"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  3513
}, {
	date:    "2026-10-02"
	version: "4.0.3"
	text:    "Rejection and backlog taste sources read from the plans-archive store."
	tokens:  3493
}, {
	date:    "2026-10-01"
	version: "4.0.2"
	text:    "Pipeline declares from /smine — the dispatcher that invokes this stage."
	tokens:  3383
}, {
	date:    "2026-09-28"
	version: "4.0.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  3379
}, {
	date:    "2026-09-28"
	version: "4.0.0"
	text:    "Profile staged at .smine-run/style-profile.md; the nightly runner, the bootstrap wrapper or the operator's smine-routine publish-style copies it — no agent writes injected context. The leak scan checks the staged file. Plans archive read via a skill grant."
}, {
	date:    "2026-09-28"
	version: "3.3.1"
	text:    "Pipeline line links the README Skill map."
}, {
	date:    "2026-09-28"
	version: "3.3.0"
	text:    "Leak scan: make style-check after every write, judged by its ': ok' / ': withheld' output line (make collapses exit codes); a secret/PII hit replaces the profile with the withheld stub, a scan failure makes the skill write it."
}, {
	date:    "2026-09-26"
	version: "3.2.1"
	text:    "Peek tools on the canonical mcp__peek-mcp__ namespace; session_full replaced by session_get."
}, {
	date:    "2026-09-24"
	version: "3.2.0"
	text:    "Design taste joins voice: plan Decisions tables, the plans archive, proposal rejections and backlog rejections are distilled into a ## Design taste block of the style profile; smine-problems reads it for solution directions; the nightly runs the skill when a taste source changed."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kind lines (Hard invariants=context, Steps=phase), pred/post clauses on every rule/verify/step/stop entry, sibling atoms.json (20 bool + 1 int atom), two queries (stub route blocks distillation, stub path still satisfies the always-writes invariant), Applies bullets under step/stop entries reclassified to param; content and registers unchanged."
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
	text:    "format-v2 reformat — When to use became Routing (Purpose/Disambiguation/Preconditions/Pipeline), argument-hint carries a full example invocation, allowed-tools normalized (plain tools then grouped Bash/mcp entries, sorted), version bumped to semver; content unchanged"
}, {
	date:    "2026-09-05"
	version: "1.3"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-01"
	version: "1.2"
	text:    "always writes style-profile.md — a stub (generated-by line plus a paragraph stating no style learned yet, the usable-session count, and that it refreshes automatically) is written when fewer than 2 usable sessions were sampled, so the UI can tell not-learned-yet from broken; an absent file is never a valid terminal state"
}, {
	date:    "2026-08-27"
	version: "1.1"
	text:    "directive lines are written in the presentation-profile language (fallback: dominant language of the sampled turns) and the file must exemplify its own register; a target-path write failure is a reported stage failure, never a silent staging file; trigger/workflow wording moved from the retired /smine-bootstrap skill to the bootstrap wrapper (cmd/bootstrap/run.sh)"
}, {
	date:    "2026-08-27"
	version: "1.0"
	text:    "initial version — sample the last n sessions' user messages via peek-mcp and write the machine-global style profile"
}]
enableDelegation: "never"
env: {
	ROUTINE_PUBLISH_STYLE: {
		interface: "routine-publish-style@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-style"
}, {
	key:   "description"
	value: " Learn the user's voice and design taste from session transcripts, plan decisions and rejection reasons into the style profile every session receives. Trigger on /smine-style [n], or as the style stage of routine bootstrap and the nightly run. Args — n: how many recent sessions to sample (default 30); model: model of every reader (default sonnet); batch: session ids per voice reader (default 10)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.5.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"40 model:sonnet batch:10\""
}, {
	key:   "allowed-tools"
	value: " Read, Read(~/GolandProjects/agent-plans/**), ToolSearch, Workflow, Write, Bash(grep *), Bash(jq *), Bash(ls *), mcp__peek-mcp__session_get, mcp__peek-mcp__session_list"
}]
preamble: """
	# smine: learn the user's style

	Samples how the user talks with agents and how the user decides designs, distills both into `.smine-run/style-profile.md`
	in cwd, which operator code publishes as `~/.claude/context/global/style-profile.md` — standing directive: every session
	answers in the user's own register and proposes in the user's design taste, not the model's defaults.
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
				note:       "install → bootstrap style stage → nightly style stage when a taste source changed (see README.md § Skill map, smine repo)"
				standalone: true
			}
			preconditions: ["peek-mcp available (`session_list`, `session_get`)", "taste sources optional: `plans/*/*-wip-*/` Decisions tables, the plans archive `~/GolandProjects/agent-plans/`, `<store>/proposals/archive/rejected.md`, the rejected rows of `<store>/backlog/backlog.json` (`<store>` the path `make rules ARGS=\"store-root\"` prints); fewer than 3 ⇒ the taste block is a stub paragraph"]
			purpose: "seeds/refreshes machine's style profile — voice and design taste — the standing directive every session answers in and designs by; smine-problems reads the taste block for solution directions."
			routes: [{
				for: "changing declared audience or UI language"
				use: "the presentation profile (Profile page / install)"
			}, {
				for: "mining actionable retrospective items"
				use: "/smine"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "n"
				default: "`30`"
				effect:  "how many of the machine's most recent usable sessions to sample — [Select sessions](#skill-sminestyle-steps-002a)"
				form:    "free text, positional"
			}, {
				name:    "model"
				default: "`sonnet`"
				effect:  "model of every voice and taste reader — [Spawn readers](#skill-sminestyle-steps-002d)"
				form:    "`model:<token>`"
			}, {
				name:    "batch"
				default: "`10`"
				effect:  "session ids per voice reader — [Spawn readers](#skill-sminestyle-steps-002d)"
				form:    "`batch:<int>`"
			}]
		}
	}]
}, {
	title: "Hard invariants"
	blocks: [{
		entry: {
			id:        "SKILL-SMINESTYLE-INVARIANT-001"
			pred:      "not verbatim_quoted and not pii_included"
			role:      "rule"
			statement: "Agent MUST NOT quote user messages, plan text or rejection reasons verbatim; MUST NOT include names, addresses, credentials, file contents, or PII from any source."
			title:     "No PII/verbatim"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-INVARIANT-002"
			pred:      "not hostility_mimicked"
			role:      "rule"
			statement: "Agent MUST NOT mimic profanity or hostility; describe register neutrally (e.g. \"very direct, no praise\") instead."
			title:     "No mimicked hostility"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-INVARIANT-003"
			pred:      "not extra_files_written"
			role:      "rule"
			statement: "Agent MUST write `style-profile.md` and nothing else."
			title:     "Output file only"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-INVARIANT-004"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "distilled profile, or stub ([Stub path content](#skill-sminestyle-steps-004f)) when too few sessions sampled; absent file looks broken in UI."
			}]
			pred:      "reported => profile_written"
			role:      "rule"
			statement: "Run MUST always end with `style-profile.md` written; MUST NOT end having written nothing."
			title:     "Always writes"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-INVARIANT-005b"
			pred:      "not silently_staged"
			role:      "rule"
			statement: "Agent MUST write only `.smine-run/style-profile.md` in cwd; MUST NEVER write `~/.claude/context/global/` nor run `$SKILL_SMINESTYLE_ROUTINE_PUBLISH_STYLE publish-style` — publishing is operator code."
			title:     "Stage only"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-INVARIANT-005a"
			post: ["write_verified"]
			pred:      "profile_written"
			role:      "verify"
			statement: "After writing target path: VERIFY write succeeded; on failure, report stage failure."
			title:     "Report write failure"
		}
	}]
	kind: "context"
}, {
	title: "Steps"
	blocks: [{
		entry: {
			id:        "SKILL-SMINESTYLE-STEPS-001b"
			pred:      "not declared_values_rewritten"
			role:      "rule"
			statement: "Declared values MUST NOT be contradicted or rewritten."
			title:     "Binding values"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-STEPS-003c"
			pred:      "not capability_changes_audience"
			role:      "rule"
			statement: "Capability signal (developer vs. casual vocabulary) MUST tune register depth only, never the audience value."
			title:     "Capability signal scope"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004d"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "English guide describing German usage = defect."
			}]
			pred:      "profile_written => directive_language_matches"
			role:      "rule"
			statement: "Directive lines MUST be written in presentation-profile language (absent profile: dominant language of sampled user turns); file MUST exemplify register it prescribes."
			title:     "Directive language"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-STEPS-004e"
			pred:      "presentation_conflict => conflicting_line_dropped"
			role:      "rule"
			statement: "Directives complement presentation profile; on conflict, presentation profile MUST win, conflicting line MUST drop."
			title:     "Presentation profile precedence"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-STEPS-004g"
			pred:      "stub_taken => stub_written"
			role:      "rule"
			statement: "UI MUST tell \"not learned yet\" from \"broken\"; absent file reads broken."
			title:     "Stub distinguishability"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-001a"
			post: ["profile_read"]
			role:      "step"
			statement: "READ `~/.claude/context/global/presentation-profile.md` for language and audience; absent means English/developer."
			title:     "Declared profile"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-002a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "routine and pipeline sessions — title starting `smine` or `routine`, or origin `sdk` or `scheduled`."
			}, {
				label:  "param"
				marker: "*"
				text:   "default n is 30; never widen `window_days`; read no transcript here — the readers do."
			}]
			post: ["peek_confirmed"]
			pred:      "profile_read"
			role:      "step"
			statement: "CALL `session_list` once with `agent: \"claude\"` as the visible peek check; DROP routine and pipeline sessions and sessions under 2 user turns; KEEP the newest n ids."
			title:     "Select sessions"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-002b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "report the degradation in the run output — never a bare halt."
			}, {
				label:  "param"
				marker: "*"
				text:   "usable sessions — ids [Select sessions](#skill-sminestyle-steps-002a) kept; after the read, the summed `sessionsRead` of the `voice[]` abstracts."
			}]
			post: ["stub_taken"]
			pred:      "usable_sessions < 2"
			role:      "stop"
			statement: "If peek-mcp doesn't answer, or fewer than 2 usable sessions remain ⇒ STOP: take **stub path** ([Stub path content](#skill-sminestyle-steps-004f)) instead of distilling."
			title:     "Peek unavailable"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-002c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "WIP plans — `plans/*/*-wip-*/concept/concept.md` and `design/*.md`; archive — `~/GolandProjects/agent-plans/*/`, flat `done` dirs and feature folders."
			}, {
				label:  "param"
				marker: "*"
				text:   "rejections — `<store>/proposals/archive/rejected.md`; a missing source is skipped."
			}, {
				label:  "param"
				marker: "*"
				text:   "backlog — `archive.rejected[]` of `<store>/backlog/backlog.json`; `<store>` = the path `make rules ARGS=\"store-root\"` prints."
			}]
			role:      "step"
			statement: "LIST the taste-source paths — WIP plan files, plans-archive plan dirs, the proposal rejections file, the backlog JSON — without reading them."
			title:     "List taste sources"
		}
	}, {
		entry: {
			id:      "SKILL-SMINESTYLE-STEPS-002d"
			channel: "script"
			items: [{
				label:  "Script"
				marker: "*"
				text:   "`workflows/read.js`"
			}, {
				label:  "param"
				marker: "*"
				text:   "`sessions` = the selected ids, `tasteSources` = the listed paths, `model` and `batch` from the Args."
			}, {
				label:  "param"
				marker: "*"
				text:   "returns `{voice[], taste}` — one schema-typed abstract per voice batch plus the taste abstract, labels only."
			}]
			post: ["sampled"]
			pred:      "peek_confirmed"
			role:      "spawn"
			statement: "SPAWN the Workflow tool, scriptPath `<base dir>/workflows/read.js`, args `{sessions[], tasteSources[], model, batch}`; await its notification."
			title:     "Spawn readers"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-003a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "language mix e.g. German with English technical terms; register spans terse vs. elaborate, direct vs. hedged."
			}]
			post: ["signals_derived"]
			pred:      "sampled and usable_sessions >= 2"
			role:      "step"
			statement: "DERIVE user's voice from the `voice[]` abstracts: `languageMix` and `register` weighted by `userTurnsRead`; abstracts with `sessionsRead` 0 carry no signal."
			title:     "Distill signals"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-003b"
			post: ["preferences_derived"]
			pred:      "signals_derived"
			role:      "step"
			statement: "DERIVE recurring phrasings and preferences from the abstracts' `patterns` labels that recur across batches (e.g. hates bullet walls, asks for examples)."
			title:     "Distill preferences"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-003d"
			items: [{
				label:  "param"
				marker: "*"
				text:   "preferred mechanisms, e.g. closed world, deterministic gate, single source of truth, disk-visible state."
			}, {
				label:  "param"
				marker: "*"
				text:   "rejected shapes, e.g. parallel mechanisms, user-drafted proposals, backfills."
			}]
			role:      "step"
			statement: "DERIVE the user's recurring design answers from the `taste` abstract: `chosen` mechanisms, `rejected` shapes and recurring rejection reasons, trade-off `tilts`."
			title:     "Distill taste"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004a"
			post: ["profile_written"]
			pred:      "preferences_derived"
			role:      "step"
			statement: "REWRITE `.smine-run/style-profile.md` in cwd wholesale:"
			title:     "Write wholesale"
		}
	}, {
		entry: {
			id:        "SKILL-SMINESTYLE-STEPS-004b"
			pred:      "profile_written"
			role:      "step"
			statement: "WRITE the first line as `<!-- generated by /smine-style — a mining re-run rewrites this file; edit freely until then (Profile page or any editor) -->`"
			title:     "Generated-by first line"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004c"
			post: ["directive_lines_written"]
			pred:      "profile_written"
			role:      "step"
			statement: "WRITE at most 30 directive lines (\"Answer in …\", \"Match …\", \"Avoid …\") an agent can follow with no other context."
			title:     "Directive line cap"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004h"
			items: [{
				label:  "param"
				marker: "*"
				text:   "directive shapes — \"Prefer …\", \"Reject …\", \"When trust is the problem, answer with …\"."
			}, {
				label:  "param"
				marker: "*"
				text:   "summed `sourcesActuallyRead` below 3 ⇒ one paragraph under the heading naming that count instead of directives."
			}]
			role:      "step"
			statement: "WRITE a `## Design taste` heading after the voice directives, then at most 20 directive lines an agent can apply with no other context."
			title:     "Design taste block"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004f"
			items: [{
				label:  "param"
				marker: "*"
				text:   "paragraph states usable-session count found, that it refreshes automatically as sessions accumulate."
			}]
			post: ["profile_written", "stub_written"]
			pred:      "stub_taken"
			role:      "step"
			statement: "WRITE file on **stub path** (fewer than 2 usable sessions): generated-by first line, then one paragraph stating no style learned yet."
			title:     "Stub path content"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004i"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`ARGS=\"--path .smine-run/style-profile.md\"` — scan the staged file, never the published one."
			}, {
				label:  "param"
				marker: "*"
				text:   "output names kind and line only; never read the matched line back into the report."
			}]
			post: ["profile_scanned"]
			pred:      "write_verified"
			role:      "step"
			statement: "RUN `make style-check` after every write, stub included; judge its output lines, never make's exit code: a `: withheld` line = stub in place — report hit kinds as stage degradation."
			title:     "Leak scan"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-004j"
			post: ["profile_written", "stub_written"]
			pred:      "scan_failed"
			role:      "verify"
			statement: "When output has neither a `: ok` nor a `: withheld` line: VERIFY profile is the stub; else WRITE [Stub path content](#skill-sminestyle-steps-004f) stating the profile was withheld, report stage failure."
			title:     "Scan failure"
		}
	}, {
		entry: {
			id: "SKILL-SMINESTYLE-STEPS-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "outside nightly and bootstrap runs, name the operator command `$SKILL_SMINESTYLE_ROUTINE_PUBLISH_STYLE publish-style <staged path>`."
			}]
			post: ["reported"]
			pred:      "write_verified and profile_scanned"
			role:      "step"
			statement: "REPORT summed `sessionsRead` and `userTurnsRead`, taste sources read as the summed `sourcesActuallyRead`, written file's line count, the staged path; then STOP."
			title:     "Report"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "reading many transcripts and decisions and abstracting voice and taste without copying content"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "reads live session transcripts and rewrites the global style profile outside the worktree"
}
