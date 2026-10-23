changelog: [{
	date:    "2026-10-17"
	version: "3.10.0"
	text:    "Dropped the caveman arg and its two entries — no skill takes caveman as an arg."
	tokens:  7019
}, {
	date:    "2026-10-17"
	version: "3.9.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  7241
}, {
	date:    "2026-10-15"
	version: "3.8.0"
	text:    "The suite-green verify runs the repo's test command one Bash call per line."
	tokens:  7329
}, {
	date:    "2026-10-14"
	version: "3.7.0"
	text:    "Interfaces: one variable per capability — TEST"
	tokens:  7321
}, {
	date:    "2026-10-13"
	version: "3.6.0"
	text:    "The suite command is the optional env variable $SKILL_COVERAGEINCREASE_TEST, a param of the suite-green verify instead of the project's test target."
	tokens:  7321
}, {
	date:    "2026-10-09"
	version: "3.5.0"
	text:    "Runtime placement declared: owns plan mode — the brief is delivered through ExitPlanMode."
	tokens:  7296
}, {
	date:    "2026-10-05"
	version: "3.4.5"
	text:    "Added the Adds column to the Decisions table of the plan template"
	tokens:  7296
}, {
	date:    "2026-10-04"
	version: "3.4.4"
	text:    "Dropped the serena arg — Serena support removed repo-wide; goland stays the one IDE opt-in."
	tokens:  7292
}, {
	date:    "2026-10-03"
	version: "3.4.3"
	text:    "Template follows the plan catalog: Decisions directly behind Scope, reader cells first in the Decisions header."
	tokens:  7373
}, {
	date:    "2026-10-02"
	version: "3.4.2"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  7373
}, {
	date:    "2026-09-30"
	version: "3.4.1"
	text:    "Template re-cut onto the human-first plan catalog: Context folded into the Coverage baseline override, Decisions in the Brief / Decision (brief) shape, Verification as the ID | Check | Command | Expected | Surface table."
	tokens:  7377
}, {
	date:    "2026-09-29"
	version: "3.4.0"
	text:    "Brief drops the Deferred findings group: needs-refactor and integration-only paths are Out rows with their reason."
}, {
	date:    "2026-09-28"
	version: "3.3.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-28"
	version: "3.3.0"
	text:    "Decisions table follows the plan format's new shape: ID | Problem | Facts | Decision | Rejected | Why | Consequences."
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-25"
	version: "3.1.2"
	text:    "Red baseline no longer stops the run: pre-existing failures and flakes in the target packages get fixed at root first, then tests are written."
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind lines on every topic section (phase/context), a new Context wiring section replacing the static-constraints prose paragraph with one [rule] entry, freeform bullets converted to * param: register, pred/post formal clauses on 26 entries (3 invariants, 23 guarded transitions) using new atoms.json (19 bool, 2 int) plus the global mode enum, three committed queries (nightly-blocks-exitplanmode, nightly-no-gate, nightly-scope-cap-witness), caveman-compressed."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), Args table, argument-hint full example invocation, internal entry cross-refs as title links, caveman-compressed prose — content identical"
}, {
	date:    "2026-09-05"
	version: "1.21"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.20"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-04"
	version: "1.19"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not coverage-work input"
}, {
	date:    "2026-09-03"
	version: "1.18"
	text:    "Coverage brief re-cut onto the plan catalog (TPL-001 declared via plan-template): Targets → Scope, Goal and decisions → Decisions/Open questions, coverage table as an override section, goal as the Verification pass condition; RULE-PLAN-* injected"
}, {
	date:    "2026-09-01"
	version: "1.17"
	text:    "Context-entry ids removed from the body; hot-class set added to the frontmatter acdsl-context declaration (context/rules/skills.md)"
}, {
	date:    "2026-08-23"
	version: "1.16"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.15"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-23"
	version: "1.14"
	text:    "acdsl-context: key rename — the ACDSL declaration key moved off Claude Code's native context: frontmatter field; prose references updated"
}, {
	date:    "2026-08-18"
	version: "1.13"
	text:    "reference rename — fchange → fdesign change (fchange retired into the fdesign change route)"
}, {
	date:    "2026-08-12"
	version: "1.12"
	text:    "migrated to the context: declaration — implementing/review-test doctrine, Go test rules, and facts injected at invocation; stale style/ paths corrected to rules/ and actions/"
}, {
	date:    "2026-08-09"
	version: "1.11"
	text:    "language-agnosticism sweep — \"governed files\" wording, generated-code de-rank per repo convention"
}, {
	date:    "2026-08-09"
	version: "1.10"
	text:    "ACDSL is the referenced context system — SessionStart delivery wording, gates authoritative"
}, {
	date:    "2026-07-31"
	version: "1.9"
	text:    "risk list reads the hot-class gates from rules/concepting.md"
}, {
	date:    "2026-07-31"
	version: "1.8"
	text:    "activity-scoped context — test style in style/go.md TESTS section, risk list from ALWAYS-HOT entries"
}, {
	date:    "2026-07-30"
	version: "1.7"
	text:    "context redesign — risk list via ../fdesign/assets/hot-items.md + rules/ overlay; stop conditions via ../fdesign/assets/"
}, {
	date:    "2026-07-30"
	version: "1.6"
	text:    "moved under skills/quality/ group; reference rename per-package-commit → package-commit"
}, {
	date:    "2026-07-26"
	version: "1.5"
	text:    "Args section"
}, {
	date:    "2026-07-24"
	version: "1.4"
	text:    "reference merge: feature-refactor → feature-change; effort token normalized"
}, {
	date:    "2026-07-19"
	version: "1.3"
	text:    "reference rename: refactor → feature-refactor"
}, {
	date:    "2026-07-19"
	version: "1.2"
	text:    "--nightly unattended mode (dedicated branch, no ExitPlanMode gate, brief + morning-review report on the branch, top-N scope cap, non-interactive skips) + n cap arg"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-11"
	version: "1.0"
	text:    "initial version"
}]
enforcePermissions: ["planMode"]
env: {
	TEST: {
		interface: "test@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " coverage-increase"
}, {
	key:   "description"
	value: " Raise test coverage on existing code — gap brief for approval, then the approved tests. Trigger on /coverage-increase or \"add test coverage\" or \"backfill tests\". Args — target: package(s)/paths to raise coverage on; goal: optional coverage target; --nightly: unattended worktree-branch mode, no approval gates; n: nightly scope cap, top-N gaps (default 5)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.10.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " 'target: internal/server goal: 80% --nightly n: 5'"
}, {
	key:   "acdsl-context"
	value: " ACTION-CONCEPT-*, ACTION-IMPL-*, ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-TEST-*, RULE-GOLANG-TEST-*, RULE-PLAN-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "plan-template"
	value: " SKILL-COVERAGEINCREASE-TPL-001"
}]
preamble: """
	# Coverage Increase

	This skill runs ground → gate → write in one session: measure coverage, present the coverage brief for approval, then write the
	approved tests.

	Nothing is written before the brief is approved, and product code is never modified — tests only.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "to"
					skill: "package-commit"
				}]
				standalone: true
			}
			preconditions: ["target package(s) build and pass — a red baseline gets fixed first ([Red baseline](#skill-coverageincrease-stop-006))"]
			purpose: "raises test coverage on existing, already-shipped code with a gap brief for approval, then writes the approved tests."
			routes: [{
				for: "tests as part of a newly designed feature"
				use: "the Tests section of /fdesign"
			}, {
				for: "code that needs restructuring for a seam"
				use: "the brief routes those items to /fdesign"
			}, {
				for: "full-stack e2e verification against a running system"
				use: "/dev-stack"
			}, {
				for: "reviewing existing tests for quality"
				use: "/railroad-review or /code-review"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "target"
				default: "none — asks if absent in a multi-package repo"
				effect:  "packages/paths to raise coverage on — [Target arg](#skill-coverageincrease-intake-001a)"
				form:    "path/package list"
			}, {
				name:    "goal"
				default: "brief proposes one from measurement"
				effect:  "coverage goal; never 100% by default — [Goal arg](#skill-coverageincrease-intake-001b)"
				form:    "coverage number or scope"
			}, {
				name:    "--nightly"
				default: "absent"
				effect:  "unattended mode on a dedicated worktree branch, no approval gate — [Nightly no gate](#skill-coverageincrease-brief-002)"
				form:    "flag"
			}, {
				name:    "n"
				default: "`5`"
				effect:  "nightly scope cap: top-N ranked gaps written — [Nightly scope cap](#skill-coverageincrease-write-008a)"
				form:    "integer"
			}]
		}
	}]
}, {
	title: "Context wiring"
	blocks: [{
		entry: {
			id: "SKILL-COVERAGEINCREASE-CTX-001"
			custom: ["hook:skill-context"]
			items: [{
				label:  "param"
				marker: "*"
				text:   "ACDSL gates authoritative over prose wherever gate covers constraint."
			}, {
				label:  "param"
				marker: "*"
				text:   "missing context doc stated once; run continues on built-in baselines."
			}]
			pred:      "context_docs_missing => fallback_stated"
			role:      "rule"
			statement: "`AGENTS.md`, per-language style guides, and `actions/` doctrine entries MUST govern every test written — arrive via this skill's frontmatter `acdsl-context:` declaration."
			title:     "Static constraints govern"
		}
	}]
	kind: "context"
}, {
	title: "Phase 0 — Intake"
	blocks: [{
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-004"
			role:      "rule"
			statement: "Measurement MUST run on the current working tree."
			title:     "Measurement scope"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-001a"
			role:      "step"
			statement: "RESOLVE `target` — the package(s)/path(s) to raise coverage on, from the invocation; if absent in a multi-package repo, ask one line."
			title:     "Target arg"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-001d"
			pred:      "not target_present"
			role:      "stop"
			statement: "If `target` is absent ⇒ STOP: never default to repo-wide."
			title:     "No repo-wide default"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-001b"
			role:      "step"
			statement: "RESOLVE `goal` — optional coverage number or scope; when absent, propose one grounded in the measurement."
			title:     "Goal arg"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-002a"
			role:      "verify"
			statement: "VERIFY the language style guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` and the `actions/` chapters exist (or arrived via context declaration)."
			title:     "Context-doc check"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-002b"
			pred:      "context_docs_missing"
			role:      "verify"
			statement: "If missing, state once and continue with built-in baselines — never stall."
			title:     "Context-doc fallback"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-INTAKE-003"
			role:      "step"
			statement: "RESOLVE the target's language(s) to the applicable test style guide; if no guide exists, the nearest sibling test file is the only shape authority — say so in the brief."
			title:     "Style authority"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 1 — Ground"
	blocks: [{
		entry: {
			id: "SKILL-COVERAGEINCREASE-GROUND-001"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "The run MUST produce evidence, not opinions."
			title:     "Evidence only"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-GROUND-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "record exact command in brief; report's delta must come from same command."
			}, {
				label:  "param"
				marker: "*"
				text:   "numbers come from real profile run, never estimated."
			}]
			role:      "step"
			statement: "RESOLVE the project's own coverage tooling first: a Makefile coverage target, else the stack default (`go test -coverprofile`, `pytest --cov`, `--coverage`)."
			title:     "Measure"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-GROUND-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "de-rank trivial accessors, `main()` wiring, generated code (per repo convention, e.g. `*.gen.go`)."
			}]
			role:      "step"
			statement: "RANK exported/public behavior first, then risk (hot-class entries of `$AGENT_CONTEXT_DIR_DEFAULT/actions/concepting.md`, baseline + overlay), then branch complexity; recent churn breaks ties."
			title:     "Rank"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-GROUND-004"
			role:      "step"
			statement: "NAME the nearest sibling test file per target, same package first; if no test file exists anywhere, flag \"first test in this project\" — the style guide alone defines shape."
			title:     "Exemplar"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-GROUND-005a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "no hard-wired globals; route lands in the brief's Not tested section."
			}]
			role:      "verify"
			statement: "VERIFY the code can be tested without restructuring (a seam exists for clock/network/fs); if not, mark needs-refactor and route to /fdesign."
			title:     "Testability triage"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-GROUND-005b"
			pred:      "not seam_present"
			role:      "stop"
			statement: "If a seam is missing ⇒ STOP: never restructure product code inline to create it."
			title:     "No inline restructure"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 2 — Decide"
	blocks: [{
		entry: {
			id:        "SKILL-COVERAGEINCREASE-DECIDE-001a"
			role:      "step"
			statement: "PRESENT the full gap menu ranked by value with a recommended cut; the user cuts scope at the gate."
			title:     "Full menu"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-DECIDE-001b"
			pred:      "scope_cut_needed"
			role:      "stop"
			statement: "If scope needs cutting ⇒ STOP: never pre-shrink the menu yourself — the user cuts at the gate."
			title:     "No pre-shrink"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-DECIDE-002"
			role:      "step"
			statement: "GROUND the goal in the measurement — per target or overall, qualitative where a percentage misleads; 100% is never the default."
			title:     "Grounded goal"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-DECIDE-003a"
			role:      "step"
			statement: "NAME the approach and goal per target in the brief (e.g. table-driven over the parser's error branches); cases emerge while writing."
			title:     "Strategy names only"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-DECIDE-003b"
			pred:      "case_names_in_brief"
			role:      "stop"
			statement: "If drafting the brief ⇒ STOP before enumerating test-case names — strategies and goals only."
			title:     "No case names"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 3 — Brief & gate"
	blocks: [{
		entry: {
			id:        "SKILL-COVERAGEINCREASE-BRIEF-001c"
			pred:      "(not brief_approved => not tests_written) and not product_code_modified"
			role:      "rule"
			statement: "Phases 0–3 MUST stay read-only; the write phase starts only on approval."
			title:     "Read-only phases"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-BRIEF-001a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "the plan-interface delivery rule in `$AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md`; a session-only plan carries no Changelog."
			}]
			post: ["gate_pauses"]
			pred:      "mode = interactive"
			role:      "step"
			statement: "DELIVER the brief through `ExitPlanMode` — it persists under `~/.claude/plans` and displays in the desktop UI."
			title:     "Deliver via ExitPlanMode"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-BRIEF-001b"
			pred:      "plans_file_written"
			role:      "stop"
			statement: "If delivering the brief ⇒ STOP before writing a plans/ file in the target repo — use ExitPlanMode instead."
			title:     "No plans file"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-BRIEF-001d"
			role:      "verify"
			statement: "VERIFY whether the user cut targets or moved the goal; if so, update the brief and re-present — the approved brief is binding."
			title:     "Re-present on changes"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-BRIEF-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "approval judgment deferred, not deleted — moves wholly to morning review of branch; nothing merges without it."
			}, {
				label:  "param"
				marker: "*"
				text:   "phases 0–2 stay read-only as usual; only change: `coverage-brief.md` on branch replaces ExitPlanMode call."
			}, {
				label:  "param"
				marker: "*"
				text:   "invoked headless (`claude -p`) or via coverage-increaser routine, always on dedicated worktree branch, e.g. `coverage/nightly-YYYY-MM-DD`."
			}]
			post: ["brief_approved"]
			pred:      "mode = nightly"
			role:      "step"
			statement: "SKIP approval for `--nightly` — write the brief to the branch as `coverage-brief.md` and proceed to the write phase immediately."
			title:     "Nightly no gate"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-TPL-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:      "payload"
			statement: "The coverage brief, on the plan catalog."
			title:     "Coverage brief"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 4 — Write the tests"
	blocks: [{
		entry: {
			id: "SKILL-COVERAGEINCREASE-WRITE-001"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Targets MUST be processed per approved target, in brief order (highest value first)."
			title:     "Order"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-WRITE-002"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Tests MUST mirror the exemplar's exact structure — file layout, naming, helper shape, test skeleton; shape rules come from the language's test style guide, never from memory."
			title:     "Mirror exemplar"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-WRITE-003"
			custom: ["todo:unmeasured"]
			role:      "quality"
			statement: "Tests MUST implement the approved strategy and assert behavior; a test that only executes lines to move the number is a coverage lie."
			title:     "Assert behavior"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-WRITE-004"
			role:      "step"
			statement: "RUN targeted tests first (`go test ./path -run TestName` analog per stack), then the package suite — green before the next target."
			title:     "Targeted then full"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-WRITE-005"
			role:      "step"
			statement: "RUN the formatter/imports pass per stack (gofmt/goimports analog) before leaving the target — each package stays independently committable."
			title:     "Formatter pass"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-WRITE-006"
			role:      "step"
			statement: "RE-MEASURE the package; note the delta."
			title:     "Re-measure"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-WRITE-007a"
			pred:      "bug_found"
			role:      "verify"
			statement: "VERIFY intent once (docs, callers, sibling tests) when a test fails against actual behavior; if genuinely wrong, drop the test, record the bug as a finding, continue with the next target."
			title:     "Failing test protocol"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-WRITE-007b"
			pred:      "bug_pinned"
			role:      "stop"
			statement: "If a bug is found ⇒ STOP before pinning it with an assertion or fixing product code inline — record it as a finding instead."
			title:     "No bug pinned"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-WRITE-008a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "product-code-never-modified invariant unchanged."
			}]
			pred:      "mode = nightly and gaps_written <= n_cap"
			role:      "step"
			statement: "WRITE tests for at most the top-N ranked gaps (`n`, default 5), in rank order, when there is no approved cut — so the run terminates."
			title:     "Nightly scope cap"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-WRITE-008b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "every written test must pass before commit; gap whose tests can't be made green reported skipped with failure, never committed red."
			}]
			pred:      "not test_green"
			role:      "stop"
			statement: "If a test can't be made green ⇒ STOP before committing it — report skipped with the failure instead."
			title:     "No red commits"
		}
	}]
	kind: "phase"
}, {
	title: "Phase 5 — Report"
	blocks: [{
		entry: {
			id:        "SKILL-COVERAGEINCREASE-REPORT-001"
			role:      "step"
			statement: "PRODUCE the delta table `Target | Before | After | Goal` with the Baseline command."
			title:     "Delta table"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-REPORT-002"
			role:      "step"
			statement: "LIST tests added: file → behavior covered, one strategy-level line each."
			title:     "Tests added"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-REPORT-003"
			role:      "step"
			statement: "REPORT findings: bugs → /diagnose-debug, needs-refactor → /fdesign, flakes observed."
			title:     "Findings"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-REPORT-004"
			role:      "step"
			statement: "REPORT remains uncovered: the updated Not-tested list."
			title:     "Remains uncovered"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-REPORT-005"
			role:      "step"
			statement: "HAND OFF committing to /package-commit — not this skill's job."
			title:     "Handoff to package-commit"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-REPORT-006a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "carries brief, per-gap outcome (written / skipped + reason), coverage before/after, test-run result."
			}, {
				label:  "param"
				marker: "*"
				text:   "here committing *is* run's job — commit on branch following /package-commit conventions, then terminate."
			}]
			pred:      "mode = nightly"
			role:      "step"
			statement: "WRITE the report as the morning-review artifact, to the branch."
			title:     "Nightly report"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-REPORT-006b"
			pred:      "mode = nightly and gate_pauses"
			role:      "stop"
			statement: "If an action would prompt interactively ⇒ STOP: record it as skipped instead — never park."
			title:     "No prompt parking"
		}
	}]
	kind: "phase"
}, {
	title: "Self-check gate"
	blocks: [{
		prose: "Before presenting the brief:"
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-001"
			role:      "verify"
			statement: "VERIFY every Baseline number comes from an actual profile run and the exact command is recorded in the brief."
			title:     "Baseline sourced"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-002"
			role:      "verify"
			statement: "VERIFY every target row has a strategy and a named exemplar (or a flagged \"first test in this project\")."
			title:     "Strategy and exemplar"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-003"
			role:      "verify"
			statement: "VERIFY no test-case names appear anywhere in the brief — strategies and goals only."
			title:     "No case names"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-004"
			role:      "verify"
			statement: "VERIFY every exclusion states its because and every needs-refactor item is routed, not dropped."
			title:     "Exclusions explained"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-005"
			role:      "verify"
			statement: "VERIFY integration-shaped targets carry their infra need as a decision."
			title:     "Integration infra decided"
		}
	}, {
		prose: "Before reporting done:"
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-006"
			role:      "verify"
			statement: "VERIFY `git status` shows only test files and fixtures — zero product-code edits, zero unapproved dependency changes."
			title:     "Only test files"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-CHECK-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "suite: `$SKILL_COVERAGEINCREASE_TEST`, one Bash call per line."
			}]
			role:      "verify"
			statement: "VERIFY the full suite is green and the formatter/imports pass ran."
			title:     "Suite green"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-008"
			role:      "verify"
			statement: "VERIFY every written test asserts behavior; none exists only to move the number."
			title:     "Behavior asserted"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-009"
			role:      "verify"
			statement: "VERIFY coverage is re-measured with the Baseline command and the delta reported against the goal, shortfall explained."
			title:     "Delta reported"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-010"
			role:      "verify"
			statement: "VERIFY no suspected bug is pinned by an assertion or fixed inline; each is a finding."
			title:     "No bug pinned"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-011a"
			pred:      "mode = nightly"
			role:      "verify"
			statement: "VERIFY, under `--nightly`: brief written to the branch as `coverage-brief.md` (no ExitPlanMode call); at most top-N gaps written."
			title:     "Nightly checklist"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-CHECK-011b"
			pred:      "mode = nightly"
			role:      "verify"
			statement: "VERIFY, under `--nightly`: morning-review report and commit land on the branch; no interactive prompt hit."
			title:     "Nightly report checklist"
		}
	}]
	kind: "context"
}, {
	title: "Stop conditions"
	blocks: [{
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-001a"
			role:      "rule"
			statement: "The gate entries of the implementing chapter (`$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md`, injected via this skill's context declaration) MUST apply."
			title:     "Implementing gates apply"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-STOP-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "in `--nightly` mode, every stop that would ask user instead recorded in morning-review report and skipped — never parks on prompt."
			}]
			pred:      "gate_pauses => mode = interactive"
			role:      "rule"
			statement: "Missing prerequisite → ask; never start infrastructure yourself."
			title:     "Ask before infrastructure"
		}
	}, {
		prose: "Coverage-specific:"
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-003a"
			pred:      "not seam_present"
			role:      "stop"
			statement: "If a target cannot be tested without modifying product code (seam, export, signature) ⇒ STOP: no seam disqualifies inline restructuring."
			title:     "No seam"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-003b"
			role:      "step"
			statement: "ROUTE the exclusion to /fdesign at the gate; if discovered mid-write, record a finding and skip."
			title:     "Route exclusion"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-004a"
			role:      "verify"
			statement: "VERIFY the test framework and coverage tooling are present; if missing, write a Decisions-needed row at the gate."
			title:     "Tooling missing"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-004b"
			pred:      "not tooling_present"
			role:      "stop"
			statement: "If tooling is missing ⇒ STOP before installing it silently."
			title:     "No silent install"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-005a"
			pred:      "not integration_infra_up"
			role:      "stop"
			statement: "If the goal needs integration infra that is not running ⇒ STOP."
			title:     "Integration infra down"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-005b"
			role:      "step"
			statement: "REPORT the shortfall and point at /dev-stack."
			title:     "Report shortfall"
		}
	}, {
		entry: {
			id:        "SKILL-COVERAGEINCREASE-STOP-005c"
			pred:      "not integration_infra_up"
			role:      "stop"
			statement: "If integration infra is unavailable ⇒ STOP before faking it with assertion-free mocks."
			title:     "No fake mocks"
		}
	}, {
		entry: {
			id: "SKILL-COVERAGEINCREASE-STOP-006"
			post: ["baseline_green"]
			pred:      "not baseline_green"
			role:      "verify"
			statement: "Before writing tests: VERIFY target packages build and pass without flakes; FIX pre-existing failures and flakes at root first — delta needs green baseline."
			title:     "Red baseline"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "gap analysis + writing style-conformant tests"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {
		check: ["go", "test", "./internal/smokecov"]
	}
	fixture: {
		patch: "change.patch"
	}
	invoke: "target: internal/smokecov goal: 100% --nightly n: 1"
	limits: {
		minutes: 25
		usd:     5.0
	}
}
