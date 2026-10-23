changelog: [{
	date:    "2026-10-20"
	version: "1.3.1"
	text:    "Interpretation: bullets only, at most 8 of 160 runes, gated like the eval's."
	tokens:  4281
}, {
	date:    "2026-10-13"
	version: "1.3.0"
	text:    "Requires the mining component: the skill drives this repo's own make targets and never deploys outside the smine checkout."
	tokens:  4270
}, {
	date:    "2026-10-09"
	version: "1.2.0"
	text:    "Runtime placement declared: owns acceptEdits (non-plan-mode runs)."
	tokens:  4270
}, {
	date:    "2026-10-05"
	version: "1.1.0"
	text:    "A/B sides are golden test runs: ab-init stops with exit 74 when the skill's golden is not minted; the golden judge runs only when run.json says golden kind test."
	tokens:  4270
}, {
	date:    "2026-10-02"
	version: "1.0.0"
	text:    "Initial: A/B layer over run records — one-factor variant descriptor, side-by-side eval runs for runner and overlay variants, stored eval or smoke records for version variants, blind compare judge and cmd/evalstats moved from skillroutine-eval."
	tokens:  4172
}]
enforcePermissions: ["acceptEdits"]
frontmatter: [{
	key:   "name"
	value: " skillroutine-ab"
}, {
	key:   "description"
	value: " Measure what one change does to a skill — model, effort, MCP server, overlay patch or version — as baseline vs variant run records. Trigger on /skillroutine-ab <skill> or \"what did this change do\" or \"compare models on a skill\". Args — skill: skill under test; layer: eval|smoke; variant: one changed factor; replicas: runs per side; token: stored token label."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 1.3.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"fimplement layer: eval variant: model:sonnet replicas: 3 token: <label>\""
}, {
	key:   "branch-prefix"
	value: " claude-review/run-"
}, {
	key:   "acdsl-context"
	value: " FACT-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Agent, Glob, Grep, Read, Read(~/.cache/claude-routine/worktrees/run-*/**), TaskOutput, Write, Bash(git diff *), Bash(go run ./cmd/evalstats *), Bash(go run ./cmd/skilleval *)"
}]
preamble: """
	# Skillroutine A/B (one change, two record sets)

	Compares a baseline and a variant that differ in exactly one factor, on the run records of a lower layer: eval runs
	side by side for a runner or overlay change, stored eval or smoke records for a skill-version change. Reports per
	measure both sides' mean and spread with Welch's t, plus a blind compare judge over side-by-side outputs. Output:
	`evals/<skill>/ab-<date>-<hash6>/` with `ab.json` and `report.md`.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "from"
					skill:    "skillroutine-eval"
					optional: true
				}]
				standalone: true
			}
			preconditions: ["session cwd is the claude-configs checkout — `go run ./cmd/skilleval` and `evals/` resolve there", "non-plan-mode session; peek-mcp at `http://127.0.0.1:4242/mcp` (config server up), else telemetry cells stay null", "a runner or overlay variant: the skill bundles `eval/definition.json`", "a version variant: records of both versions exist — eval runs under `evals/<skill>/`, or one `make skill-smoke` run per version"]
			purpose: "answers \"what did this one change do\" — baseline vs variant run records per measure with spread, plus a blind comparison of side-by-side outputs."
			routes: [{
				for: "measuring one configuration"
				use: "/skillroutine-eval"
			}, {
				for: "does the skill run at all on a dummy input"
				use: "`make skill-smoke SKILL=<leaf>`"
			}, {
				for: "predicting a run's cost before spending it"
				use: "/skillroutine-estimate"
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
				effect:  "skill under test; the eval layer runs its `eval/definition.json` as baseline — [Variant descriptor](#skill-skillroutineab-var-001)"
				form:    "free text, first"
			}, {
				name:    "layer"
				default: "`eval`"
				effect:  "lower layer whose run records are compared; smoke compares `version:` variants only — [Init](#skill-skillroutineab-init-001)"
				form:    "`layer: eval` or `layer: smoke`"
			}, {
				name:    "variant"
				default: "none, required"
				effect:  "the one changed factor — [Variant descriptor](#skill-skillroutineab-var-001)"
				form:    "`variant: <descriptor>`"
			}, {
				name:    "replicas"
				default: "definition `replicas`, else `1`"
				effect:  "runs per side, equal on both — [Init](#skill-skillroutineab-init-001)"
				form:    "`replicas: <n>`"
			}, {
				name:    "token"
				default: "none; required when the sides run"
				effect:  "stored OAuth token the runs bill, by label only — [Unknown token](#skill-skillroutineab-init-005)"
				form:    "`token: <label>`"
			}]
		}
	}]
}, {
	title: "Variant"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEAB-VAR-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`model`, `effort`, `no-` change the runner; `overlay` adds a patch on top of the definition's overlay — both sides run side by side."
			}, {
				label:  "param"
				marker: "*"
				text:   "`version` runs nothing: stored records of skill versions `<a>` and `<b>`, selected by `skillVersion`."
			}, {
				label:  "param"
				marker: "*"
				text:   "smoke layer: `version` only — smoke keeps one record per deployed version."
			}]
			role:      "def"
			statement: "Exactly one of `model:<name>`, `effort:<level>`, `no-<mcp server>`, `overlay:<repo-relative patch>`, `version:<a>..<b>`; the baseline is the definition's configuration."
			title:     "Variant descriptor"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-VAR-002"
			role:      "rule"
			statement: "A descriptor MUST change exactly one factor against the baseline — two changes or none is no A/B."
			title:     "One factor"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEAB-RULES-001"
			role:      "rule"
			statement: "The skill under test MUST NEVER be modified for a run; a change that seems necessary is a design-premise violation — stop and report."
			title:     "Skill untouched"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-RULES-002"
			role:      "rule"
			statement: "Fan-out worker models MUST come from the task's own pins; a spawn without a model is stopped by heartbeat — say which pins in `interpretation.md`."
			title:     "Worker pins"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-RULES-003"
			role:      "rule"
			statement: "A token value MUST NEVER enter `run.json`, chat or any artifact — only its store label; `go run ./cmd/skilleval arm` reads the value itself."
			title:     "Label only"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-RULES-004"
			role:      "rule"
			statement: "Unverifiable values MUST be null with a note, never estimated."
			title:     "Nothing invented"
		}
	}]
	kind: "context"
}, {
	title: "1. Init"
	blocks: [{
		entry: {
			id: "SKILL-SKILLROUTINEAB-INIT-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`-layer`, `-replicas` and `-token` only when given; flags before the skill."
			}, {
				label:  "param"
				marker: "*"
				text:   "sides run ⇒ `run.json` holds a baseline key and a variant key with equal replicas under shuffled labels."
			}, {
				label:  "param"
				marker: "*"
				text:   "`version` variant ⇒ no `run.json`, no token needed, no side runs."
			}]
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval ab-init -layer <layer> -replicas <n> -token <label> -variant <descriptor> <skill>` — prints the A/B dir; writes `ab.json`, and `run.json` when the sides run."
			title:     "Init"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-002"
			role:      "stop"
			statement: "If ab-init exits 72 ⇒ STOP: print its reason; ask for one [Variant descriptor](#skill-skillroutineab-var-001)."
			title:     "Bad variant"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-003"
			role:      "stop"
			statement: "If ab-init exits 73 ⇒ STOP: the smoke layer compares versions only; offer `layer: eval` or a `version:` descriptor."
			title:     "Not runnable"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-004"
			role:      "stop"
			statement: "If ab-init exits 69 ⇒ STOP: the skill has no `eval/definition.json`; name the skill — a definition is authored in its skill dir."
			title:     "No definition"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-005"
			role:      "stop"
			statement: "If ab-init exits 70 ⇒ STOP: list the labels it printed, ask for one; never repeat a value; tokens are saved in the config server's token store."
			title:     "Unknown token"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-006"
			role:      "stop"
			statement: "If ab-init exits 71 ⇒ STOP: print its math — both sides count; suggest fewer replicas or env `SKILLEVAL_WINDOW_BUDGET_USD`."
			title:     "Over window"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-008"
			role:      "stop"
			statement: "If ab-init exits 74 ⇒ STOP: the skill's golden is not minted; A/B sides are test runs and need it."
			title:     "No golden"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-INIT-007"
			role:      "stop"
			statement: "If ab-init exits any other non-zero ⇒ STOP before any run: report its stderr."
			title:     "Init failed"
		}
	}]
}, {
	title:   "2. Run sides"
	between: "INIT-007..COLLECT-001"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-001"
			role:      "rule"
			statement: "Judges MUST get only the brief `skilleval brief` prints — never side keys, models, efforts, the descriptor, telemetry or another judge's output."
			title:     "Blind judges"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-002"
			role:      "rule"
			statement: "Every judge MUST run on the `run.json` `judge` model, the same for both sides — NEVER inherit a run's model."
			title:     "Pinned judge model"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-003"
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval provision <ab dir>` — resolves repo and refs, creates one worktree per label with its side's overlay."
			title:     "Provision"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-004"
			role:      "stop"
			statement: "If provision exits non-zero ⇒ STOP before any run: 67 names an unregistered repo, else report its stderr."
			title:     "Provision failed"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-005"
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval arm <ab dir> <label>` once per `run.json` label, each its own background Bash call, all in one message; WAIT for every completion notification."
			title:     "Launch runs"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-006"
			role:      "verify"
			statement: "After all runs: VERIFY each `runs/<label>/result.json` exists; rerun a missing label once, then report it as a harness failure."
			title:     "Run results"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-007"
			role:      "spawn"
			statement: "SPAWN one judge per label with the brief `go run ./cmd/skilleval brief adherence <ab dir> <label>` prints; it writes `runs/<label>/adherence.json`."
			title:     "Adherence judges"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEAB-SIDES-008"
			items: [{
				label:  "param"
				marker: "*"
				text:   "only when `jq -r .golden.kind <ab dir>/run.json` prints `test` — a run without a golden has no golden judge."
			}]
			role:      "spawn"
			statement: "SPAWN one judge per label with the brief `go run ./cmd/skilleval brief golden <ab dir> <label>` prints; it writes `runs/<label>/judge.json`."
			title:     "Golden judges"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-009"
			role:      "spawn"
			statement: "SPAWN one judge with the brief `go run ./cmd/skilleval brief compare <ab dir>` prints; it writes `compare.json`."
			title:     "Compare judge"
		}
	}, {
		entry: {
			id: "SKILL-SKILLROUTINEAB-SIDES-010"
			items: [{
				label:  "param"
				marker: "*"
				text:   "never forced, never deleting bytes; a `removed: false` row is reported, never retried."
			}]
			role:      "step"
			statement: "After judges finish, RUN `go run ./cmd/skilleval cleanup <ab dir> <label>` per label."
			title:     "Run cleanup"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-011"
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval fold <ab dir>` — writes `eval.json` with every run's measures."
			title:     "Fold"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-SIDES-012"
			role:      "stop"
			statement: "If fold exits non-zero ⇒ STOP: report its error and `eval.rejected.json`; never hand-edit eval.json into shape."
			title:     "Fold rejected"
		}
	}]
	kind: "route"
	pred: "sides_run"
}, {
	title: "3. Collect"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEAB-COLLECT-001"
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval ab-collect <ab dir>` — fills both record sets of `ab.json`: the A/B dir's runs, or the stored records of each version."
			title:     "Collect"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-COLLECT-002"
			role:      "stop"
			statement: "If ab-collect exits non-zero ⇒ STOP: report its error — a side without records names what to run first."
			title:     "Empty side"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-COLLECT-003"
			role:      "step"
			statement: "RUN `go run ./cmd/evalstats -ab <ab dir>/ab.json -out <ab dir>/stats.json`."
			title:     "Statistics"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-COLLECT-004"
			role:      "step"
			statement: "RUN `go run ./cmd/skilleval ab-report <ab dir>` — writes the per-measure differences into `ab.json` and renders `report.md`."
			title:     "Report render"
		}
	}]
}, {
	title: "4. Report"
	blocks: [{
		entry: {
			id:        "SKILL-SKILLROUTINEAB-REPORT-001"
			role:      "rule"
			statement: "`interpretation.md` MUST stay human-curatable: in-place edits expected, git history is the log."
			title:     "Interpretation curation"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-REPORT-002"
			role:      "step"
			statement: "WRITE `<ab dir>/interpretation.md`: bullets only, at most 8, each at most 160 runes, on what the change did, from the difference cells; call a difference significant only where the report does."
			title:     "Interpretation"
		}
	}, {
		entry: {
			id:        "SKILL-SKILLROUTINEAB-REPORT-003"
			role:      "step"
			statement: "PRINT the difference table of `report.md` in chat, then STOP for user review."
			title:     "Report"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "runs fixed Go steps and spawns judges on the pinned judge model; one short interpretation"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	skip: "runs two eval-layer configurations or needs stored records of two versions — no dummy input fits a smoke budget"
}
