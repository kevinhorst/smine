changelog: [{
	date:    "2026-10-14"
	version: "5.9.0"
	text:    "Interfaces: one variable per capability — AGENTWT_PROBE, TASK_CHECK"
	tokens:  4491
}, {
	date:    "2026-10-13"
	version: "5.8.0"
	text:    "Programs are env variables: $SKILL_SMINE_RULES (required, the implement stage's task check and its enforce anchor) and $SKILL_SMINE_AGENTWT (required, the consolidate stage's probe grant and precondition)."
	tokens:  4479
}, {
	date:    "2026-10-10"
	version: "5.7.1"
	text:    "allowed-tools: smine-agentwt probe replaces pick — the consolidate stage's pick check is one probe call; the pipeline never picks."
	tokens:  4467
}, {
	date:    "2026-10-09"
	version: "5.7.0"
	text:    "Carry stage removed from the pipeline and the run summary; --limit mines newest first."
	tokens:  4467
}, {
	date:    "2026-10-09"
	version: "5.6.0"
	text:    "Runtime placement declared: root-only — the dispatcher invokes Workflow-backed stage skills, never as a subagent."
	tokens:  4597
}, {
	date:    "2026-10-08"
	version: "5.5.0"
	text:    "Implement stage checks every task file with smine-rules task-check before the first spawn and skips an over-cap task; a heartbeat check denies the first implementer spawn without it"
	tokens:  4597
}, {
	date:    "2026-10-08"
	version: "5.4.0"
	text:    "Change task carries problem, evidence and fix instead of finding; the commit body is problem and fix as prose; the runner stores the task files in the run report."
	tokens:  4473
}, {
	date:    "2026-10-06"
	version: "5.3.0"
	text:    "Implement stage spawns one implementer subagent per change task — model opus, runs /fimplement with the task and errata, makes the one change commit; the dispatcher never implements a task itself; the fimplement edge is spawns."
	tokens:  4450
}, {
	date:    "2026-10-06"
	version: "5.2.0"
	text:    "Stage carry between problems and implement runs /smine-consolidate carry; the run summary carries the carried rows; the batch stage lists newest first."
	tokens:  3966
}, {
	date:    "2026-10-04"
	version: "5.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  3846
}, {
	date:    "2026-10-04"
	version: "5.0.0"
	text:    "Mining stage list: memory, context, permissions and acdsl run as stages, the implement stage commits each change task as one commit through /fimplement, consolidate makes every change commit pick alone, the judge returns per-change verdicts the summary carries as `changes`; harness and apply stages, --apply-args and the proposal store retired."
	tokens:  3846
}, {
	date:    "2026-10-02"
	version: "4.2.5"
	text:    "Precondition names the proposal store by `<store>`, the path `make rules ARGS=\"store-root\"` prints; the smoke seeds its store from a fixture tree."
	tokens:  2559
}, {
	date:    "2026-10-02"
	version: "4.2.4"
	text:    "Precondition names the plans-archive proposal store."
	tokens:  2571
}, {
	date:    "2026-10-01"
	version: "4.2.3"
	text:    "Pipeline declares every stage skill the dispatcher invokes, matching the Skill grants."
	tokens:  2524
}, {
	date:    "2026-09-28"
	version: "4.2.2"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2480
}, {
	date:    "2026-09-27"
	version: "4.2.1"
	text:    "argument-hint marker path moved to the group's steer dir, where the runner now reads the run summary."
}, {
	date:    "2026-09-27"
	version: "4.2.0"
	text:    "batch stage takes --limit (oldest first); --repos, --since and --last retired with the Go pending list."
}, {
	date:    "2026-09-26"
	version: "4.1.1"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-26"
	version: "4.1.0"
	text:    "Pipeline dispatcher: stages load inline via the Skill tool in one session, fan-out through the stage skills' workflows, --stages list, run-summary marker; session-mine workflow and --no-batch retired."
}, {
	date:    "2026-09-25"
	version: "4.0.1"
	text:    "Make-only entry points: allowed-tools grants make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "4.0.0"
	text:    "Problem-first pipeline: the dimension fan-out, per-dimension cap, dimension flags and six-ledger cursor retired; mine then spawn smine-problems (--cap); workflow stages Drift, Mine, Problems."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Full formal-schema reformat: section-kind lines on Rules (context), * param: register on the smine-batch passthrough bullets, Dimension flag mapping recast as a fenced block; pred/post formal clauses on 13 entries (6 invariants, 7 guarded transitions) over new atoms.json (11 local bool atoms; reuses global mode enum; asks_user flagged as a global-vocabulary candidate); three queries under queries/ (nightly-no-workflow, headless-blocks-workflow, batch-fanout-reachable)."
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
	text:    "format-v2 reformat: When to use → Routing (Purpose/Disambiguation table/Preconditions/Pipeline), Args bulleted list → syntax-index table with deep semantics moved into owning body entries (new RESOLVE-009, RUN-002/RUN-008 sub-bullets), argument-hint gains one full example invocation, allowed-tools reordered per the sort convention — content identical"
}, {
	date:    "2026-09-05"
	version: "1.18"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.17"
	text:    "smine-acdsl dimension — fan-out gains the acdsl gate-proposal dimension (own proposals/acdsl.json, parallel set), --no-acdsl flag, sixth ledger analyzed-acdsl.txt; workflow and cap plumbing updated"
}, {
	date:    "2026-08-30"
	version: "1.16"
	text:    "fifth dimension smine-permissions wired into the fan-out (--no-permissions skip, analyzed-permissions.txt ledger, proposals/permissions.json — parallel with the other dimensions); new --subagents flag passed through to smine-batch (per-subagent transcript mining) and threaded through the session-mine workflow args"
}, {
	date:    "2026-08-27"
	version: "1.15"
	text:    "added --last <n> (newest-n session cap, --since wins when both given) and --dev (include smine-repo and routine sessions, otherwise excluded by smine-batch's default-deny rule) — both passed through verbatim to smine-batch and threaded through the session-mine workflow args"
}, {
	date:    "2026-08-27"
	version: "1.14"
	text:    "added --since <YYYY-MM-DD> (date floor on session last_active, passed through to smine-batch and the session-mine workflow); retired --scopes — folder routing now derives from --repos attribution (one folder per repo, default for unmatched), batch resolution enumerates every sessions/ folder except archived/"
}, {
	date:    "2026-08-22"
	version: "1.13"
	text:    "session-mine workflow args gain optional model/effort — a tier override spread into every agent() call so the whole fan-out runs at a chosen model/effort tier without forking a per-tier workflow file; omitting either preserves session inheritance (tier orthogonal to model, never baked into a filename)"
}, {
	date:    "2026-08-21"
	version: "1.12"
	text:    "session-mine workflow gains a pre-run Drift stage — diffs each scope's per-dimension analyzed-*.txt ledgers and reports diverging session ids (held_by/missing_from/suspected_cause); informational, never gates, surfaces silent coverage drift especially on explicit-batch runs"
}, {
	date:    "2026-08-23"
	version: "1.11"
	text:    "allowed-tools manifest for headless runs (Skill/Task/peek-mcp/Bash union); Workflow demoted to the manual-run alternative — the direct pipeline is the primary path"
}, {
	date:    "2026-08-23"
	version: "1.10"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.9"
	text:    "smine-memory redesign follow-through: memory is now a proposal-producing dimension (context.json) — joins the per-dimension cap, fan-out sequences smine-memory after smine-context (shared proposals/context.json), report wording drops applied-memory"
}, {
	date:    "2026-08-23"
	version: "1.8"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-20"
	version: "1.7"
	text:    "description compressed under the brevity caps"
}, {
	date:    "2026-08-18"
	version: "1.6"
	text:    "smine-workflows folded into smine-skills — fan-out is four dimensions, --no-workflows dropped; --agents claude,codex passed through to smine-batch (default both)"
}, {
	date:    "2026-08-12"
	version: "1.5"
	text:    "dynamic scope roster — --scopes arg (restrict to named scope dirs, passed through to smine-batch), scope enumeration is every dir under sessions/ except proposals/, personal/work no longer hardcoded"
}, {
	date:    "2026-08-11"
	version: "1.4"
	text:    "smine-batch now emits the batch JSON — smine-summary retired, fan-out is five dimensions, --no-summary and the analyzed-summarize.txt cursor dropped; smine-style → smine-context (--no-style → --no-context)"
}, {
	date:    "2026-08-09"
	version: "1.3"
	text:    "--repos roster arg (permission-config additionalDirectories) passed through to smine-batch for mechanical repo attribution"
}, {
	date:    "2026-07-31"
	version: "1.2"
	text:    "session-scope rename to personal/work (sessions/{personal,work} globs)"
}, {
	date:    "2026-07-30"
	version: "1.1"
	text:    "smine-rules → smine-style (--no-rules → --no-style); production caps --max-proposals-per-dimension / --max-proposals-mined threaded into the workflow plus a deterministic trim stage"
}, {
	date:    "2026-07-27"
	version: "1.0"
	text:    "merges the analyze v1.6 pipeline front and the /smine entrypoint — /smine now runs mine (smine-batch) + six-dimension fan-out via the session-mine workflow; adds --no-batch and per-dimension --no-* flags; analyze skill and analyze.js retired"
}]
enableDelegation: "never"
env: {
	AGENTWT_PROBE: {
		interface: "agentwt-probe@1"
		required:  true
	}
	TASK_CHECK: {
		interface: "task-check@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine"
}, {
	key:   "description"
	value: " Run the smine pipeline in one session — stage skills in order via the Skill tool, one spawned implementer and one commit per change task, one run summary. Trigger on /smine or \"run a session retrospective\". Args — --nightly: unattended; --stages: stage list; --cap; --limit; --subagents; --agents; --dev; --language; --marker: summary path."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 5.9.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"--nightly --stages style,batch,memory,context,permissions,acdsl,problems,implement,consolidate,judge --cap 5 --limit 30 --subagents --agents claude --dev --language en --marker ~/.cache/claude-routine/steer/smine-nightly/run-complete.json\""
}, {
	key:   "allowed-tools"
	value: " Edit, Read, Skill(fimplement), Skill(smine-acdsl), Skill(smine-batch), Skill(smine-consolidate), Skill(smine-context), Skill(smine-memory), Skill(smine-orchestrate), Skill(smine-permissions), Skill(smine-problems), Skill(smine-style), ToolSearch, Workflow, Write, Bash($SKILL_SMINE_AGENTWT_PROBE probe *), Bash(cat *), Bash(git diff *), Bash(git log *), Bash(git restore --staged *), Bash(git show *), Bash(git status *), Bash(grep *), Bash(head *), Bash(jq *), Bash(ls *), mcp__peek-mcp__session_events, mcp__peek-mcp__session_get, mcp__peek-mcp__session_list, Bash(wc *)"
}, {
	key:   "enable-model-invocation"
	value: " true"
}]
preamble: """
	# smine (pipeline)

	Run the smine pipeline in this session: each stage skill loads here via the Skill tool, fan-out stages spawn their bundled
	workflows, every change task goes to one implementer subagent and becomes one commit, the run ends with one summary.
	"""
requires: ["mining"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "to"
					skill: "smine-style"
				}, {
					kind:  "to"
					skill: "smine-batch"
				}, {
					kind:  "to"
					skill: "smine-memory"
				}, {
					kind:  "to"
					skill: "smine-context"
				}, {
					kind:  "to"
					skill: "smine-permissions"
				}, {
					kind:  "to"
					skill: "smine-acdsl"
				}, {
					kind:  "to"
					skill: "smine-problems"
				}, {
					kind:  "to"
					skill: "smine-consolidate"
				}, {
					kind:  "to"
					skill: "smine-orchestrate"
					route: "run"
				}, {
					kind:  "spawns"
					skill: "fimplement"
				}]
				note: "runs the stages inline: style → batch → memory → context → permissions → acdsl → problems → implement (one implementer subagent per change task, running /fimplement) → consolidate → judge; the routine runner publishes and lands after the verdict"
			}
			preconditions: ["root session with the Workflow tool (never a subagent)", "cwd = the run worktree's repo root holding `sessions/`, on a branch started at `main`", "peek-mcp available when `batch` or `style` runs", "`$SKILL_SMINE_AGENTWT_PROBE probe` answers when `consolidate` runs"]
			purpose: "runs the requested pipeline stages in order in one root session — interactive and nightly run the same protocol, nightly drops the review stop."
			routes: [{
				for: "quick look at what a session did"
				use: "/peek"
			}, {
				for: "raw transcript mining only"
				use: "/smine-batch"
			}, {
				for: "problem synthesis only"
				use: "/smine-problems"
			}, {
				for: "implementing one change by hand"
				use: "/fimplement"
			}, {
				for: "judging a run's change commits only"
				use: "/smine-orchestrate"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "--nightly"
				default: "absent"
				effect:  "unattended: no review stop; the mining stages get `--nightly` — [Invoke stage](#skill-smine-run-004)"
				form:    "flag"
			}, {
				name:    "--stages"
				default: "mining list"
				effect:  "stages to run, map order; absent = the mining list — [Parse flags](#skill-smine-resolve-001)"
				form:    "`<name,…>`"
			}, {
				name:    "--cap"
				default: "`5`"
				effect:  "max new changes per miner and max new problems — [Stage map](#skill-smine-resolve-002)"
				form:    "`<n>`"
			}, {
				name:    "--subagents"
				default: "absent"
				effect:  "also mine subagent transcripts, passed to smine-batch — [Stage map](#skill-smine-resolve-002)"
				form:    "flag"
			}, {
				name:    "--agents"
				default: "both"
				effect:  "agents mined, passed to smine-batch — [Stage map](#skill-smine-resolve-002)"
				form:    "`<claude,codex>`"
			}, {
				name:    "--limit"
				default: "`5`"
				effect:  "sessions mined this run, newest first, passed to smine-batch — [Stage map](#skill-smine-resolve-002)"
				form:    "`<n>`"
			}, {
				name:    "--dev"
				default: "absent"
				effect:  "include the pipeline's own sessions, passed to smine-batch — [Stage map](#skill-smine-resolve-002)"
				form:    "flag"
			}, {
				name:    "--language"
				default: "profile language"
				effect:  "language of the run summary's stage summaries and reasons — [Run summary](#skill-smine-resolve-003)"
				form:    "`<lang>`"
			}, {
				name:    "--marker"
				default: "absent"
				effect:  "run summary JSON written here as final act — [Write summary](#skill-smine-finish-003)"
				form:    "`<path>`"
			}]
		}
	}]
}, {
	title: "1. Resolve"
	blocks: [{
		entry: {
			id: "SKILL-SMINE-RESOLVE-002"
			items: [{
				fence: {
					body: """
						style        smine-style        (none)
						batch        smine-batch        [--nightly] --agents --limit --dev --subagents (given ones)
						memory       smine-memory       [--nightly]
						context      smine-context      mine [--nightly] --cap <n>
						permissions  smine-permissions  mine [--nightly] --cap <n>
						acdsl        smine-acdsl        mine [--nightly] --cap <n>
						problems     smine-problems     [--nightly] --cap <n>
						implement    (this skill)       Implement stage section: one implementer spawn per task
						consolidate  smine-consolidate  run
						judge        smine-orchestrate  run

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "Stage name, skill, args — map order is run order:"
			title:     "Stage map"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RESOLVE-003"
			role:      "def"
			statement: "Run summary = [Run summary payload](#skill-smine-tpl-001): one `stages[]` row per requested stage; `changes` and `verdict` when `judge` ran; prose in `--language`."
			title:     "Run summary"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RESOLVE-001"
			role:      "step"
			statement: "PARSE the flags; `--stages` absent means the mining list: every [Stage map](#skill-smine-resolve-002) row from `batch` through `consolidate`."
			title:     "Parse flags"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RESOLVE-004"
			role:      "verify"
			statement: "VERIFY every requested name is in the [Stage map](#skill-smine-resolve-002); reorder to map order."
			title:     "Stage list"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RESOLVE-005"
			role:      "stop"
			statement: "If a stage name is not in the map ⇒ STOP: write the summary with that stage `failed` when `--marker` is set, report the arg error."
			title:     "Unknown stage"
		}
	}]
	kind: "phase"
}, {
	title: "2. Run"
	blocks: [{
		entry: {
			id:        "SKILL-SMINE-RUN-001"
			role:      "rule"
			statement: "Stage skills MUST load in this session via the Skill tool, NEVER in a subagent — Workflow is root-only; the implement stage's per-task implementers are the one spawn."
			title:     "Stages in root"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RUN-002"
			role:      "rule"
			statement: "The run MUST NOT ask a question or stop mid-run; pick the codified path."
			title:     "No questions"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RUN-003"
			role:      "rule"
			statement: "A failed stage MUST be recorded `failed` with its reason; the run MUST continue with the next stage."
			title:     "Failure continues"
		}
	}, {
		entry: {
			id: "SKILL-SMINE-RUN-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`--nightly` adds `--nightly` to every stage the map marks `[--nightly]`."
			}, {
				label:  "param"
				marker: "*"
				text:   "`implement` runs the Implement stage section below in place of a Skill call."
			}]
			role:      "step"
			statement: "For each stage in order: INVOKE the Skill tool with the mapped skill and args, FOLLOW it here; its review stop means return."
			title:     "Invoke stage"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RUN-005"
			role:      "verify"
			statement: "Before the next stage: VERIFY every Workflow the stage spawned sent its completion notification; wait, never move on early."
			title:     "Workflow returned"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RUN-006"
			role:      "step"
			statement: "RECORD the stage row: `ok`, `failed` or `skipped`, one-line summary with counts."
			title:     "Record outcome"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-RUN-007"
			role:      "step"
			statement: "After `judge`: RECORD its returned per-change verdicts `{id, sha, verdict, reason}` and run verdict `{accepted, reason}` verbatim."
			title:     "Record verdict"
		}
	}]
	kind: "phase"
}, {
	title: "Implement stage"
	blocks: [{
		entry: {
			id:        "SKILL-SMINE-IMPL-001"
			role:      "def"
			statement: "Change task = `.smine-run/changes/<kind>-<slug>.json` by a miner or problems: `{id, kind, title, problem, evidence, fix, sessions, target, change, notes}`; never committed, copied into the run report."
			title:     "Change task"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-002"
			role:      "def"
			statement: "Change commit = one commit per task: subject per [Commit subjects]($AGENT_CONTEXT_DIR_DEFAULT/rules/commits.md), body the task's problem and fix as prose, last paragraph the trailers `Smine-Change: <id>` and `Smine-Sessions: <id>, <id>`."
			title:     "Change commit"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-003"
			role:      "rule"
			statement: "The implement stage MUST NOT invoke /fdesign and MUST NOT edit settings or permission files except through a `permissions` task, whose change is one allow-rule line."
			title:     "Implement bounds"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-010"
			role:      "rule"
			statement: "The dispatcher MUST NOT edit or commit a change task itself — every edit and the one commit happen inside the task's implementer subagent; a task without a commit stays `skipped`."
			title:     "Dispatcher hands off"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-012"
			role:      "step"
			statement: "RUN `$SKILL_SMINE_TASK_CHECK task-check .smine-run/changes/*.json` once when task files exist; a file with a violation is `skipped: over cap — <first violation line>` and gets no implementer."
			title:     "Check tasks"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-004"
			role:      "step"
			statement: "For each task file in file-name order: RUN `make acdsl ARGS=\"project -strip\"`, RECORD HEAD (`git log -1 --format=%H`)."
			title:     "Prepare task"
		}
	}, {
		entry: {
			id: "SKILL-SMINE-IMPL-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "Agent tool, model `opus`, sequential: the next task's implementer starts after this one returned."
			}, {
				label:  "param"
				marker: "*"
				text:   "prompt: [Implementer prompt](#skill-smine-impl-011) with the task file's JSON and [Implement errata](#skill-smine-impl-009) substituted."
			}]
			role:      "spawn"
			statement: "SPAWN one implementer subagent for the task and WAIT for its return."
			title:     "Implement task"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-006"
			role:      "verify"
			statement: "After the implementer returns: VERIFY `git log --format=%B <recorded HEAD>..HEAD` holds one commit carrying `Smine-Change: <id>`; none ⇒ task `skipped`, any other deviation recorded for the judge."
			title:     "One commit"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-007"
			role:      "verify"
			statement: "After the implementer returns: VERIFY `git status --porcelain` lists nothing outside `sessions/` and `.smine-run/`; else `git restore --staged .` and record the leftover paths for the judge."
			title:     "Clean tree"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-IMPL-008"
			role:      "step"
			statement: "RECORD the `implement` stage row: commits made, tasks skipped with reason (over-cap files from [Check tasks](#skill-smine-impl-012) included), leftover paths."
			title:     "Record implement"
		}
	}, {
		entry: {
			id: "SKILL-SMINE-IMPL-009"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Implement errata"
		}
	}, {
		entry: {
			id: "SKILL-SMINE-IMPL-011"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Implementer prompt"
		}
	}]
	kind: "context"
}, {
	title: "3. Finish"
	blocks: [{
		entry: {
			id:        "SKILL-SMINE-FINISH-001"
			role:      "rule"
			statement: "The run summary MUST be written only after every stage returned — writing it ends the session."
			title:     "Summary last"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-FINISH-002"
			role:      "step"
			statement: "RELAY stage rows, failures, per-change verdicts and the run verdict; the change commits stay on the run branch for the routine runner."
			title:     "Relay"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-FINISH-003"
			role:      "step"
			statement: "With `--marker <path>`: WRITE the run summary JSON there, `changes` copied from the judge's per-change verdicts, VERIFY it with `jq empty`, then end the turn."
			title:     "Write summary"
		}
	}, {
		entry: {
			id:        "SKILL-SMINE-FINISH-004"
			role:      "stop"
			statement: "If mode is interactive ⇒ STOP: report for review."
			title:     "Interactive stop"
		}
	}]
	kind: "phase"
}, {
	title: "Payloads"
	blocks: [{
		entry: {
			id: "SKILL-SMINE-TPL-001"
			payload: {
				info:   "json"
				marker: "```"
			}
			role:  "payload"
			title: "Run summary payload"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "runs every stage's judgment inline and dispatches the fan-outs"
			tier:   "frontier"
		}
	}]
}]
enforce: {
	anchors: {
		implementer_spawn: {
			agent: "root"
			input: [{
				field:   "prompt"
				pattern: "Smine-Change"
			}]
			tool: "^Agent$"
		}
		task_check: {
			agent: "root"
			input: [{
				field:   "command"
				pattern: "^$SKILL_SMINE_TASK_CHECK task-check "
			}]
			tool: "^Bash$"
		}
	}
	checks: {
		tasks_checked: {
			after:    "task_check"
			kind:     "requires"
			on:       "implementer_spawn"
			reason:   "smine: run `$SKILL_SMINE_TASK_CHECK task-check .smine-run/changes/*.json` before the first implementer spawn"
			recovery: "heartbeat runtime ({finding}): run `$SKILL_SMINE_TASK_CHECK task-check .smine-run/changes/*.json`, skip every file it reports as `skipped: over cap — <line>`, then spawn the implementer"
		}
	}
}
smoke: {
	expect: {
		check: ["test", "-s", "smoke-run-complete.json"]
	}
	invoke: "--nightly --stages consolidate --marker smoke-run-complete.json"
}
