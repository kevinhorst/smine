changelog: [{
	date:    "2026-10-17"
	version: "4.7.1"
	text:    "Model file is <leaf>.cue"
	tokens:  3646
}, {
	date:    "2026-10-14"
	version: "4.7.0"
	text:    "Interfaces: one variable per capability — TASK_CHECK"
	tokens:  3647
}, {
	date:    "2026-10-13"
	version: "4.6.0"
	text:    "The judge fix task's check is the env variable $SKILL_SMINEORCHESTRATE_RULES (required)."
	tokens:  3642
}, {
	date:    "2026-10-09"
	version: "4.5.1"
	text:    "Audit step runs `make audit` in one foreground call redirected into a scratchpad log, then Reads it; no pipe, no wakeup or sleep wait"
}, {
	date:    "2026-10-08"
	version: "4.5.0"
	text:    "Judge fix task carries all ten keys and passes smine-rules task-check before its commit"
	tokens:  3568
}, {
	date:    "2026-10-08"
	version: "4.4.0"
	text:    "Judge fix writes a task file and carries the fixed change's sessions; the content check reads the task's problem and fix"
	tokens:  3428
}, {
	date:    "2026-10-04"
	version: "4.3.0"
	text:    "Settings deploy named by the install engine"
	tokens:  3274
}, {
	date:    "2026-10-04"
	version: "4.2.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  3303
}, {
	date:    "2026-10-06"
	version: "4.1.0"
	text:    "Pick check removed from the run route: whether a change applies onto main is read live by the Proposals page; the per-change verdict judges quality only."
	tokens:  3303
}, {
	date:    "2026-10-04"
	version: "4.0.0"
	text:    "Run route judges each change commit in main..HEAD — trailer, pick-alone check, finding and target match, one acdsl check and audit for the branch — and returns per-change verdicts plus the run verdict; judge fixes are their own `judge/<slug>` change commits. Auto-applied re-judge, store delta, proposal-schema and presentation checks, store-check and the store grants removed; bootstrap no longer expects consolidate or apply outputs."
	tokens:  3536
}, {
	date:    "2026-10-04"
	version: "3.7.2"
	text:    "The bootstrap wrapper script is replaced by routine bootstrap; description, purpose and the bootstrap params name it."
	tokens:  3216
}, {
	date:    "2026-10-02"
	version: "3.7.1"
	text:    "Store paths hang off `<store>`, the path `make rules ARGS=\"store-root\"` prints — a smoke or eval run works on its own store, never the live one; the smoke seeds its store from a fixture tree."
	tokens:  3212
}, {
	date:    "2026-10-02"
	version: "3.7.0"
	text:    "Judge reviews the plans-archive store delta with `git -C` status and diff and validates the stores with `make rules ARGS=\"store-check\"`; run route may edit the store dirs under Read and Edit grants."
	tokens:  3287
}, {
	date:    "2026-10-02"
	version: "3.6.0"
	text:    "Skills deploy through `./bin/configserver -sync-skills`; the lockout no longer names SKILL.md, which no checkout holds."
	tokens:  3009
}, {
	date:    "2026-09-28"
	version: "3.5.2"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  3020
}, {
	date:    "2026-09-28"
	version: "3.5.1"
	text:    "Casual lockout covers a skill's skill.cue as well as its generated SKILL.md (JUDGE-002d)."
}, {
	date:    "2026-09-28"
	version: "3.5.0"
	text:    "Bootstrap context-pack sync moved to the wrapper; additionalDirectories reconcile and the wrapper-owns-dirs rule removed — repositories are never additional directories. The sync_context.sh grant is gone (now a forbidden fixture)."
}, {
	date:    "2026-09-28"
	version: "3.4.1"
	text:    "Pipeline line links the README Skill map."
}, {
	date:    "2026-09-28"
	version: "3.4.0"
	text:    "Manifest narrowed to the body's commands: mv dropped, the four sync scripts named, remote/push reduced to git remote get-url origin and git push origin main. Bootstrap records its start HEAD and runs no sync script or push when cmd/sync changed during the run."
}, {
	date:    "2026-09-26"
	version: "3.3.0"
	text:    "Route sections declare their anchor: 2. Sync deployed state between JUDGE-004b..REPORT-001, steps 3-5 chain each after the previous route's last entry; route lines stay in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "3.2.1"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "Branch mode replaced by the run route: judge the uncommitted run delta inside /smine, return the verdict; merge moved to the routine runner; sync, context sync, reconciliation and push are bootstrap-only; git rules narrowed."
}, {
	date:    "2026-09-25"
	version: "3.1.2"
	text:    "Named the Go routine runner as the owner of the orchestrate stage's mechanical postconditions"
}, {
	date:    "2026-09-25"
	version: "3.1.1"
	text:    "Make-only entry points: allowed-tools and prose call make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Full formal schema: section-kind clauses (context/phase) on every topic section; pred/post clauses on the invariant, mode, merge, lockout, audit and push entries; sibling atoms.json (20 bool atoms, one local enum run_mode: branch|bootstrap); three queries (merge-off blocks the merge step, bootstrap mode blocks merged, push reachable when a remote exists)."
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
	text:    "format-v2 reformat (/fmt skill v2) — Routing section, Args table, allowed-tools normalized, content unchanged"
}, {
	date:    "2026-09-05"
	version: "1.4"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-01"
	version: "1.3"
	text:    "branch-mode (merge: off) suffix (developer auto-merge: false) — judge, sync, reconcile and report but skip the merge duty, leaving the branch for a manual merge; cleanup and syncs still run and a kept branch is the expected terminal state, not a rejection"
}, {
	date:    "2026-08-27"
	version: "1.2"
	text:    "bootstrap mode is the terminal stage of the bootstrap wrapper (cmd/bootstrap/run.sh, /smine-bootstrap skill retired); optional (since: date) suffix — mined sessions older than the floor are findings; the presentation gate names the concrete user-visible fields and forbids file paths, rule/FACT/ACDSL IDs, and schema/taxonomy jargon on casual installs"
}, {
	date:    "2026-08-27"
	version: "1.1"
	text:    "casual lockout invariant — on a casual install the judge rejects any run-branch commit modifying a skill (SKILL.md or bundled files) lacking origin: user frontmatter; a violation fails the stage"
}, {
	date:    "2026-08-26"
	version: "1.0"
	text:    "initial version — nightly stage-3 gatekeeper: judge (incl. auto-applied re-judging), merge with reconcile doctrine, deployed-state and context-pack syncs, additionalDirectories reconciliation, push-if-remote; bootstrap mode for initial setup"
}]
env: {
	TASK_CHECK: {
		interface: "task-check@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-orchestrate"
}, {
	key:   "description"
	value: " Judge a smine run's change commits — one verdict per change plus the run verdict — or judge and commit an initial-setup run. Trigger on /smine-orchestrate run as the /smine judge stage, or /smine-orchestrate bootstrap after routine bootstrap. Args — run: judge each change commit in main..HEAD; bootstrap: commit what is left, optional (since: date) check."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.7.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"run | bootstrap (since: <YYYY-MM-DD>)\""
}, {
	key:   "allowed-tools"
	value: " Edit, Read, ToolSearch, Write, Bash(./bin/configserver -apply), Bash(./bin/configserver -sync-skills), Bash(git add *), Bash(git commit *), Bash(git diff *), Bash(git log *), Bash(git push origin main), Bash(git remote get-url origin), Bash(git show *), Bash(git status *), Bash(jq *)"
}]
preamble: """
	# smine: orchestrate a run

	Judge every change the other pipeline stages committed before it becomes the machine's state.

	Gatekeeper of automatic pipeline: one verdict per change commit plus the run verdict. Judgment lives here; the routine
	runner only publishes and lands mechanically — fast-forward, deploy, push.
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
					kind:  "from"
					skill: "smine-consolidate"
				}]
				note: "run route: verdicts, then the routine runner publishes and lands; bootstrap route: after the bootstrap stages (see README.md § Skill map, smine repo)"
			}
			preconditions: ["run: cwd = the run worktree, change commits in `main..HEAD`, ledger under `sessions/` uncommitted", "bootstrap: cwd main checkout, branch `main`, clean before its stages"]
			purpose: "judges a nightly run's change commits inside its run worktree and returns one verdict per change plus the run verdict (`run`, the /smine judge stage), or finalizes an initial-setup run (`bootstrap` mode, last stage of `routine bootstrap`)."
			routes: [{
				for: "producing change tasks"
				use: "the mining skills, /smine-problems"
			}, {
				for: "making change commits pick alone"
				use: "/smine-consolidate"
			}, {
				for: "developer's manual local review"
				use: "plain `git merge` — merging locally = acceptance act"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "run"
				default: "none"
				effect:  "judge every change commit in `main..HEAD`, return the verdicts — [Run route](#skill-smineorchestrate-judge-007)"
				form:    "positional"
			}, {
				name:    "bootstrap"
				default: "—"
				effect:  "initial-setup mode — [Bootstrap mode](#skill-smineorchestrate-judge-005)"
				form:    "positional alternative"
			}, {
				name:    "(since: <YYYY-MM-DD>)"
				default: "none"
				effect:  "date floor for mined sessions — [Since floor](#skill-smineorchestrate-judge-006)"
				form:    "bootstrap-mode suffix"
			}]
		}
	}]
}, {
	title: "Hard invariants"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-INVAR-006"
			role:      "def"
			statement: "Change commit = commit in `main..HEAD` with `Smine-Change: <kind>/<slug>` and `Smine-Sessions: <id>, <id>` trailers; its task file `.smine-run/changes/<kind>-<slug>.json` holds problem, evidence, fix."
			title:     "Change commit"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-INVAR-008"
			role:      "def"
			statement: "Verdicts = per change `{id, sha, verdict: ready|rejected, reason}`; run `{accepted, reason}`, accepted = audit green and no unexpected uncommitted change outside `sessions/`."
			title:     "Verdicts"
		}
	}, {
		entry: {
			id: "SKILL-SMINEORCHESTRATE-INVAR-009a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "`sessions` the fixed change's ids, `target` the fixed path, `change` one imperative sentence, `notes` empty."
			}]
			role:      "def"
			statement: "Judge fix task = `.smine-run/changes/judge-<slug>.json` with the ten task keys: `kind` `judge`, `problem` the defect ≤ 320 characters, `evidence` `[]`, `fix` why the fix holds ≤ 240 characters."
			title:     "Judge fix task"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-INVAR-009b"
			role:      "def"
			statement: "Judge fix commit = `git commit -q -m <subject> -m \"<defect> <fix>\" -m <trailers>`, trailer lines `Smine-Change: judge/<slug>`, `Smine-Sessions: <the fixed change's ids>`; judged like any change."
			title:     "Judge fix commit"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-INVAR-002"
			pred:      "run_mode = bootstrap => main_clean"
			role:      "rule"
			statement: "Bootstrap MUST start from clean `main` and end every path with a clean tree."
			title:     "Clean main required"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-INVAR-004"
			pred:      "not remote_added and (pushed => remote_exists)"
			role:      "rule"
			statement: "Orchestrator MUST NOT push unless `git remote get-url origin` succeeds; MUST NOT add remote."
			title:     "Push restrictions"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-INVAR-005"
			role:      "rule"
			statement: "Run route MUST NOT switch branches, merge, push or touch any checkout but cwd; its only commits are judge fix commits."
			title:     "Run route isolation"
		}
	}]
	kind: "context"
}, {
	title: "1. Judge"
	blocks: [{
		entry: {
			id: "SKILL-SMINEORCHESTRATE-JUDGE-005"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "lenient on seeding volume, strict on schema/audit/style — machine being seeded, large output volume expected, acceptable."
			}]
			pred:      "run_mode = bootstrap => not merged"
			role:      "rule"
			statement: "Orchestrator MUST treat `bootstrap` as setup mode: judge the stages' change commits, commit only what is left (ledger, style staging) on main, no branch or merge, then sections 2–4."
			title:     "Bootstrap mode"
		}
	}, {
		entry: {
			id: "SKILL-SMINEORCHESTRATE-JUDGE-006"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "bootstrap-mode suffix, passed by `routine bootstrap` when run had date floor."
			}]
			pred:      "since_floor_set and session_before_floor => finding_raised"
			role:      "rule"
			statement: "Orchestrator MUST treat `(since: <YYYY-MM-DD>)` as finding trigger: mined session with `last_active` before date = finding (fix batch or reject)."
			title:     "Since floor"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-007"
			role:      "rule"
			statement: "`run` MUST judge every change commit in `main..HEAD` one by one; the verdicts are the return value."
			title:     "Run route"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-001a"
			role:      "step"
			statement: "REVIEW — run: `git log --reverse --format=%H main..HEAD`, `git show` each, `git status --porcelain`; bootstrap: RECORD start HEAD (`git log -1 --format=%H`), then commits and tree."
			title:     "Review the commits"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-008a"
			role:      "verify"
			statement: "Per change commit: VERIFY its message carries `Smine-Change: <kind>/<slug>` and `Smine-Sessions:`; missing ⇒ verdict `rejected`."
			title:     "Trailer present"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-008c"
			role:      "verify"
			statement: "Per change commit: VERIFY the diff implements its task file's `problem` and `fix` and stays inside its target plus the tests and generated files it needs; fix or reject."
			title:     "Matches finding"
		}
	}, {
		entry: {
			id: "SKILL-SMINEORCHESTRATE-JUDGE-002a"
			post: ["schema_valid"]
			role:      "verify"
			statement: "VERIFY `make acdsl ARGS=\"check\"` green on the branch head; on failure fix or reject the causing change (duty 4)."
			title:     "ACDSL check"
		}
	}, {
		entry: {
			id: "SKILL-SMINEORCHESTRATE-JUDGE-002b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "never a pipe, never a ScheduleWakeup or sleep wait."
			}]
			post: ["audit_green"]
			role:      "verify"
			statement: "VERIFY `make audit > <scratchpad>/audit.log 2>&1` exits 0 once for the whole branch, one foreground call, then Read the log; on failure fix or reject the causing change (duty 4)."
			title:     "Audit green"
		}
	}, {
		entry: {
			id: "SKILL-SMINEORCHESTRATE-JUDGE-002d"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "covers every judged change commit."
			}]
			pred:      "audience_casual"
			role:      "verify"
			statement: "VERIFY on `casual` profile no change modifies a skill (`skills/*/*/*.cue` or bundled files) lacking `origin: user`; on violation apply lockout violation response."
			title:     "Casual lockout"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-002e"
			pred:      "lockout_violation"
			role:      "step"
			statement: "On lockout violation, SET the change's verdict `rejected` (run) or REVERT its commit (bootstrap)."
			title:     "Lockout violation response"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-009"
			role:      "verify"
			statement: "VERIFY `git status --porcelain` lists nothing outside `sessions/` and `.smine-run/`; any other path ⇒ run verdict `accepted: false`, paths in the reason."
			title:     "No stray changes"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-JUDGE-004a"
			role:      "step"
			statement: "FIX small defects — run: WRITE [Judge fix task](#skill-smineorchestrate-invar-009a), RUN `$SKILL_SMINEORCHESTRATE_TASK_CHECK task-check` on it (exit 0, else shorten), one judge fix commit and verdict row each; bootstrap: commit on main."
			title:     "Fix small defects"
		}
	}, {
		entry: {
			id: "SKILL-SMINEORCHESTRATE-JUDGE-004b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "bootstrap report written at repo root; `routine bootstrap` surfaces, removes it; stop after report step."
			}]
			post: ["rejected"]
			pred:      "run_hopeless"
			role:      "stop"
			statement: "If run hopeless ⇒ STOP: reject, never half-merge — run: run verdict `accepted: false` with reason; bootstrap: `.orchestrate-report`."
			title:     "Reject hopeless run"
		}
	}]
	kind: "phase"
}, {
	title:   "2. Sync deployed state"
	between: "JUDGE-004b..REPORT-001"
	blocks: [{
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-SYNCSTATE-002"
			pred:      "sync_scripts_changed"
			role:      "stop"
			statement: "If `git diff --name-only <start HEAD>..HEAD -- cmd/sync` or `git status --porcelain -- cmd/sync` lists a path ⇒ STOP: run no sync script, no push; report the paths as the sync duty's outcome."
			title:     "Sync scripts changed"
		}
	}, {
		entry: {
			id:        "SKILL-SMINEORCHESTRATE-SYNCSTATE-001"
			role:      "step"
			statement: "RUN `./bin/configserver -apply`, `./bin/configserver -sync-skills` — failures reported, never mask the commit result."
			title:     "Run sync scripts"
		}
	}]
	kind: "route"
	pred: "run_mode = bootstrap"
}, {
	title:   "3. Push"
	between: "SYNCSTATE-001..REPORT-001"
	blocks: [{
		entry: {
			id: "SKILL-SMINEORCHESTRATE-PUSH-001"
			post: ["pushed"]
			pred:      "remote_exists"
			role:      "step"
			statement: "RUN `git push origin main` only when remote exists; report \"no remote\" otherwise."
			title:     "Push if remote"
		}
	}]
	kind: "route"
	pred: "run_mode = bootstrap"
}, {
	title: "4. Report"
	blocks: [{
		entry: {
			id: "SKILL-SMINEORCHESTRATE-REPORT-001"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "bootstrap rejection reason stands in `.orchestrate-report`."
			}]
			role:      "step"
			statement: "REPORT one line per duty — run: verdicts JSON `{changes: [...], verdict: {accepted, reason}}` first, then per-check lines; bootstrap: per-duty outcomes."
			title:     "Per-duty report line"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "adversarial judgment over other agents' output"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "run"
	limits: {
		minutes: 30
		usd:     5.0
	}
}
