changelog: [{
	date:    "2026-10-14"
	version: "5.7.0"
	text:    "Interfaces: one variable per capability — AGENTWT_REMOVE, HEARTBEAT_RECORD, HEARTBEAT_REMOVE, HEARTBEAT_STATUS, HEARTBEAT_SUBSCRIBE, HEARTBEAT_UNSUBSCRIBE, HEARTBEAT_WORKER, PLAN_SECTIONS, ROUTINE_WORKER"
	tokens:  8263
}, {
	date:    "2026-10-13"
	version: "5.6.0"
	text:    "Programs are env variables: $SKILL_FLEET_HEARTBEAT, $SKILL_FLEET_AGENTWT and $SKILL_FLEET_ROUTINE (required) in steps, rules, the worker template and grants; $SKILL_FLEET_RULES (optional) is the Row done section-check param, Read rows runs that check."
	tokens:  8171
}, {
	date:    "2026-10-03"
	version: "5.5.0"
	text:    "Requires agentwt, heartbeat, routines: deploy skips the skill without them."
	tokens:  8099
}, {
	date:    "2026-10-06"
	version: "5.4.0"
	text:    "The orchestrator files no defect reports: Report defect and the Defect report template are gone, a held design row is only recorded, and the Write grant left with them; what it hits reaches smine through its transcript."
	tokens:  8099
}, {
	date:    "2026-10-04"
	version: "5.3.3"
	text:    "Restack and done calls carry no Unattended run: words — restack decides every conflict itself and the arg is gone from both skills."
	tokens:  8380
}, {
	date:    "2026-10-03"
	version: "5.3.2"
	text:    "The row-done check is written ~/go/bin/smine-rules plan-sections like every other command of this skill, in both design entries and in allowed-tools: an operator typed it with the prefix and was denied."
	tokens:  8435
}, {
	date:    "2026-10-03"
	version: "5.3.1"
	text:    "Worker prompt no longer forbids /delegate, and the delegate and parallelize routes are gone — both skills were deleted."
}, {
	date:    "2026-10-03"
	version: "5.3.0"
	text:    "Sync: the orchestrator lands its merge branch on every target through /done, inside that target's landing turn; a no-target answer stops the run. The own Sync and Fast-forward entries and the git branch -f grant are gone."
	tokens:  8477
}, {
	date:    "2026-10-03"
	version: "5.2.0"
	text:    "Linear landing: a member lands by git merge --ff-only once the merge branch's tip is in its branch; a member behind is asked to restack and run /done again; a delivered worker's branch that is behind is restacked on a work branch and fast-forwarded. Grants and the escalates edge move from merge-resolve to restack and done; the merge, abort and rebase grants are gone."
	tokens:  6466
}, {
	date:    "2026-10-03"
	version: "5.1.0"
	text:    "Worker model, effort, budget, the designer cap and the cells-per-row cap are args (model, effort, budget, designers, attempts) with the former values as defaults; one Worker flags definition feeds both playbooks."
	tokens:  8145
}, {
	date:    "2026-10-03"
	version: "5.0.1"
	text:    "Start no longer yields under playbook: design: with no member to send a digest, the first wake never came and no designer was launched."
	tokens:  7663
}, {
	date:    "2026-10-03"
	version: "5.0.0"
	text:    "Workers are headless cells launched with smine-routine worker and registered with smine-heartbeat worker; the Agent-tool worker and Skill(fimplement) are gone. New playbook: design with master: — one /fdesign derived --nightly cell per split row, no implementation. Synced requires an integrated fleet, so Finish no longer fires at start. A stopped implementation cell is resumed with the refined plan."
	tokens:  7624
}, {
	date:    "2026-10-03"
	version: "4.0.0"
	text:    "Renamed the skill from orchestrate to fleet: /fleet, entry ids SKILL-FLEET-*, title-code Fleet; the defect-report path ~/.claude/orchestrate/reports/ is unchanged."
	tokens:  5567
}, {
	date:    "2026-10-02"
	version: "3.3.1"
	text:    "Dropped the help-only Eval section — --help shows the smoke and eval declared beside the skill."
	tokens:  5846
}, {
	date:    "2026-10-01"
	version: "3.3.0"
	text:    "allowed-tools grants Skill(fimplement) for the feature playbook's implement-runner workers; pipeline declares spawns /fimplement."
	tokens:  5846
}, {
	date:    "2026-09-28"
	version: "3.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  5837
}, {
	date:    "2026-09-29"
	version: "3.2.0"
	text:    "The merge branch carries the run's uuid (claude-orch/<fleet>-<uuid>), so a reused fleet name never meets an old run's branch; Start subscribes first, then reads fleets[].merge_branch and switches to it."
}, {
	date:    "2026-09-28"
	version: "3.1.0"
	text:    "Sync waits for the feature queue (FEATURE-007); collisions: fleet|all maps to subscribe --outside-collisions; Finish runs unsubscribe --finish so the run dissolves; heartbeat now reads integrated only when every member delivered."
}, {
	date:    "2026-09-28"
	version: "3.0.2"
	text:    "Agent spawns name an explicit model — the gate denies inherited or Fable models; the implement-runner spawn passes model opus."
}, {
	date:    "2026-09-28"
	version: "3.0.1"
	text:    "claude-orch/<fleet> forks from the target, not from the session's HEAD (main), so workers fork from the target and the sync never carries unrelated commits."
}, {
	date:    "2026-09-28"
	version: "3.0.0"
	text:    "The orchestrator is the fleet's operator T1-O, never a member: no join. It works on its own claude-orch/<fleet> branch and never checks out the target; members merge there, and once at integrated the target is synced (rebase if it moved, then an ancestry-checked fast-forward with git branch -f <target> HEAD, which git refuses while the target is checked out elsewhere). Worker worktrees are removed after the sync; the human closes the session with /close."
}, {
	date:    "2026-09-28"
	version: "2.0.0"
	text:    "Generic core loop plus a feature playbook: joins its own fleet, merges only delivered and merge-ready members, journals every act with smine-heartbeat record, asks drifted members to merge the target, unsubscribes at integrated; design workers and the work: arg removed; the feature playbook merges a design member's plan first and runs at most one implementation worker."
}, {
	date:    "2026-09-28"
	version: "1.0.0"
	text:    "initial version: heartbeat fleet loop — nightly design and implement-runner workers, plan-collision handling, merges into the integration branch, direct worker removal, defect reports"
}]
env: {
	AGENTWT_REMOVE: {
		interface: "agentwt-remove@1"
		required:  true
	}
	HEARTBEAT_RECORD: {
		interface: "heartbeat-record@1"
		required:  true
	}
	HEARTBEAT_REMOVE: {
		interface: "heartbeat-remove@1"
		required:  true
	}
	HEARTBEAT_STATUS: {
		interface: "heartbeat-status@1"
		required:  true
	}
	HEARTBEAT_SUBSCRIBE: {
		interface: "heartbeat-subscribe@1"
		required:  true
	}
	HEARTBEAT_UNSUBSCRIBE: {
		interface: "heartbeat-unsubscribe@1"
		required:  true
	}
	HEARTBEAT_WORKER: {
		interface: "heartbeat-worker@1"
		required:  true
	}
	PLAN_SECTIONS: {
		interface: "plan-sections@1"
	}
	ROUTINE_WORKER: {
		interface: "routine-worker@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " fleet"
}, {
	key:   "description"
	value: " Run one heartbeat fleet from its own claude-orch branch — judge collisions, land delivered members by fast-forward, sync the target once at the end. Trigger on /fleet or \"orchestrate fleet T1\". Args — fleet: FleetId; target: synced branch; playbook: feature|design; master: master plan (design); collisions: fleet|all; model: worker model; effort: worker effort; budget: worker USD cap; designers: parallel designers; attempts: cells per row."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 5.7.0"
}, {
	key:   "argument-hint"
	value: " 'fleet: T1 target: feature/install playbook: design master: plans/2026-10-02-install-7ea30b5/2026-10-02-install-wip-7ea30b5/design/install.md collisions: all model: sonnet effort: medium budget: 15 designers: 3 attempts: 3'"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "title-code"
	value: " Fleet"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " ListAgents, Read, SendMessage, Skill(done), Skill(restack), Bash($SKILL_FLEET_AGENTWT_REMOVE remove --delete-branch claude/*), Bash($SKILL_FLEET_HEARTBEAT_RECORD record *), Bash($SKILL_FLEET_HEARTBEAT_REMOVE remove *), Bash($SKILL_FLEET_HEARTBEAT_STATUS status *), Bash($SKILL_FLEET_HEARTBEAT_SUBSCRIBE subscribe *), Bash($SKILL_FLEET_HEARTBEAT_UNSUBSCRIBE unsubscribe *), Bash($SKILL_FLEET_HEARTBEAT_WORKER worker *), Bash($SKILL_FLEET_PLAN_SECTIONS plan-sections *), Bash($SKILL_FLEET_ROUTINE_WORKER worker *),Bash(git branch -d claude-restack/*), Bash(git merge-base --is-ancestor *), Bash(git switch -c claude-orch/*)"
}]
preamble: """
	# Fleet

	Runs one heartbeat fleet as its operator `<fleet>-O` from its own `claude-orch/<fleet>-<uuid>` branch: members work,
	orchestrator judges collisions, lands finished work there by fast-forward, journals, syncs the target once.
	"""
requires: ["agentwt", "heartbeat", "routines"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "to"
					skill: "close"
				}, {
					kind:  "spawns"
					skill: "fdesign"
				}, {
					kind:  "spawns"
					skill: "fimplement"
				}, {
					kind:  "escalates"
					skill: "restack"
				}, {
					kind:  "escalates"
					skill: "done"
				}]
				note:       "loop over members that /join and /done; feature playbook launches /fimplement worker cells, design playbook /fdesign designer cells"
				standalone: true
			}
			preconditions: ["heartbeat daemon serves this Claude home (`$SKILL_FLEET_HEARTBEAT_STATUS status` exits 0)", "`target` branch exists locally (it may be checked out elsewhere until the sync)", "invoked in own session worktree (`claude/*` branch) of registered repo fleet binds to, operator attending chat"]
			purpose: "runs fleet named by FleetId on its own merge branch until the human-named target holds all its work."
			routes: [{
				for: "joining a fleet as a member"
				use: "/join"
			}, {
				for: "cleanup of own session worktree"
				use: "/close"
			}, {
				for: "attended merge of two diverged branches"
				use: "/merge-resolve"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "fleet"
				default: "required"
				effect:  "FleetId run subscribes to, prefixes its merge branch, filters status by — [Subscribe](#skill-fleet-start-003)"
				form:    "`fleet: <FleetId>`"
			}, {
				name:    "target"
				default: "required"
				effect:  "branch synced from the merge branch once at the end, never main — [Sync](#skill-fleet-loop-008e)"
				form:    "`target: <branch>`"
			}, {
				name:    "playbook"
				default: "absent → core loop only"
				effect:  "feature: merged plans feed one implementation worker; design: one designer cell per master row — [Row](#skill-fleet-design-001)"
				form:    "`playbook: feature\\|design`"
			}, {
				name:    "master"
				default: "required with `playbook: design`"
				effect:  "master plan file whose split rows the design playbook designs — [Row](#skill-fleet-design-001)"
				form:    "`master: <plan file>`"
			}, {
				name:    "collisions"
				default: "`fleet`"
				effect:  "all: also hear member vs non-fleet branch pairs — [Subscribe](#skill-fleet-start-003)"
				form:    "`collisions: fleet\\|all`"
			}, {
				name:    "model"
				default: "`opus`"
				effect:  "model of every worker cell — [Worker flags](#skill-fleet-def-010)"
				form:    "`model: <token>`"
			}, {
				name:    "effort"
				default: "design `high`, feature `none`"
				effect:  "effort of every worker cell; none drops the flag — [Worker flags](#skill-fleet-def-010)"
				form:    "`effort: <level>\\|none`"
			}, {
				name:    "budget"
				default: "design `25`, feature `none`"
				effect:  "USD cap per worker cell; none drops the flag — [Worker flags](#skill-fleet-def-010)"
				form:    "`budget: <usd>\\|none`"
			}, {
				name:    "designers"
				default: "`2`"
				effect:  "designer cells running at once under playbook: design — [Designer cap](#skill-fleet-design-004)"
				form:    "`designers: <n>`"
			}, {
				name:    "attempts"
				default: "`2`"
				effect:  "cells one row or plan gets before it is held — [Attempt](#skill-fleet-def-011)"
				form:    "`attempts: <n>`"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id:        "SKILL-FLEET-DEF-001"
			role:      "def"
			statement: "New turn from heartbeat digest, a worker launch's exit or skill start. Carries no facts — status document and the launch's result line do."
			title:     "Wake"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-002"
			role:      "def"
			statement: "Output of `$SKILL_FLEET_HEARTBEAT_STATUS status --fleet <fleet> --format json`; sole source of fleet facts per wake."
			title:     "Status document"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-003"
			role:      "def"
			statement: "`members[]` row of the fleet; named by its `token` (`T1-MA`) everywhere — chat, messages, journal."
			title:     "Member"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-004"
			role:      "def"
			statement: "Member row `delivered` true: a joined member ran /done, or the orchestrator reported a [Worker](#skill-fleet-def-009) delivered."
			title:     "Delivered"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-005"
			role:      "def"
			statement: "Member delivered AND `git merge-base --is-ancestor HEAD <branch>` exits 0: the merge branch's tip is in its branch. Nothing else lands."
			title:     "Landable"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-006"
			role:      "def"
			statement: "`collisions` row with a member branch on one side and no `decided` journal entry naming both members since it appeared."
			title:     "Unhandled collision"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-007"
			role:      "def"
			statement: "Fleet row `merge_branch` (`claude-orch/<fleet>-<run uuid>`), this session's own branch; every member lands here, the target only by [Sync](#skill-fleet-loop-008e)."
			title:     "Merge branch"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-008"
			role:      "def"
			statement: "Fleet row `synced` true while `target_state` is `integrated`: the target holds every member's merged work."
			title:     "Synced"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-009"
			role:      "def"
			statement: "Headless cell started per [Worker launch](#skill-fleet-tpl-004); member row `via` `worker` on branch `claude/<name>`, no session of its own."
			title:     "Worker"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-010"
			role:      "def"
			statement: "`-model <model> -effort <effort> -budget-usd <budget>` from the args, on every worker launch and resume; a flag whose value is `none` is dropped."
			title:     "Worker flags"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-011"
			role:      "def"
			statement: "One launch or resume of a worker cell for one row or plan."
			title:     "Attempt"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DEF-012"
			role:      "def"
			statement: "Member delivered, not [Landable](#skill-fleet-def-005), branch state neither `merged` nor `removable`: it holds work the merge branch lacks and restacks first."
			title:     "Behind"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-FLEET-RULES-001"
			role:      "rule"
			statement: "Orchestrator MUST NOT edit permissions, skills, context or settings."
			title:     "Foundation hands-off"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-RULES-002"
			role:      "rule"
			statement: "Orchestrator MUST NOT merge into, switch to or push default branch; landing stays human."
			title:     "Never main"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-RULES-003"
			role:      "rule"
			statement: "Orchestrator MUST land with `git merge --ff-only`; NEVER a merge commit, rebase, reset or `branch -f`; the target moves only by [Sync](#skill-fleet-loop-008e)."
			title:     "Fast-forward only"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-RULES-004"
			role:      "rule"
			statement: "Orchestrator MUST NOT edit files in its worktree; fast-forwards, restack with its conflict commits, the sync and removals only."
			title:     "Landing only"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-RULES-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "RECORD means `$SKILL_FLEET_HEARTBEAT_RECORD record --kind <kind> --subject <one line> [--member <letter>]… [--detail <why>]`."
			}, {
				label:  "param"
				marker: "*"
				text:   "kinds `assigned`, `decided`, `merged`, `spawned`, `note`."
			}]
			role:      "rule"
			statement: "Orchestrator MUST record each merge, decision, assignment and spawn with `$SKILL_FLEET_HEARTBEAT_RECORD record` right after it."
			title:     "Record every act"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-RULES-006"
			role:      "rule"
			statement: "Orchestrator MUST NOT write into a member branch or a worker's worktree; a member behind restacks itself, a worker's branch is restacked on a work branch."
			title:     "Owner restacks"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-RULES-007"
			role:      "rule"
			statement: "Orchestrator MUST NOT run `$SKILL_FLEET_AGENTWT_REMOVE remove --dry-run` — its sessiontitle hook retitles this session CLOSED."
			title:     "No dry-run"
		}
	}]
	kind: "context"
}, {
	title: "Start"
	blocks: [{
		entry: {
			id:        "SKILL-FLEET-START-001"
			role:      "step"
			statement: "READ every Args-table arg from invocation; an absent arg takes its Default cell."
			title:     "Parse args"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-START-006"
			role:      "stop"
			statement: "If `playbook: design` and `master` is absent or its args line lacks `role: master` ⇒ STOP: ask for the master plan file."
			title:     "Master missing"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-START-003"
			role:      "step"
			statement: "RUN `$SKILL_FLEET_HEARTBEAT_SUBSCRIBE subscribe --fleet <fleet> --target <target>`, plus `--outside-collisions` when `collisions: all`; the title becomes `<fleet>-O`, never a member."
			title:     "Subscribe"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-START-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "exit 1 refusal (no registered repo, fleet bound elsewhere), 2 usage, 3 unreachable, 4 foreign home."
			}]
			role:      "stop"
			statement: "If subscribe or first status exits non-zero ⇒ STOP: relay stderr verbatim; never retry with other fleet or target."
			title:     "Subscribe failed"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-START-002a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the name carries the run's uuid (`claude-orch/<fleet>-<uuid>`), known only once subscribed — never guessed."
			}]
			role:      "step"
			statement: "READ fleet row `merge_branch` from the first [Status document](#skill-fleet-def-002); RUN `git switch -c <merge_branch> <target>` in own worktree; every merge lands on it, the target is never checked out."
			title:     "Merge branch"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-START-002b"
			role:      "stop"
			statement: "If the switch fails (branch exists, worktree dirty) ⇒ STOP: relay git error; never reuse or delete the branch."
			title:     "Branch unusable"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-START-005"
			role:      "step"
			statement: "END turn; next wake enters Loop. Under `playbook: design` ENTER Loop in this turn instead — no member exists yet whose digest would wake it."
			title:     "Yield"
		}
	}]
	kind: "phase"
}, {
	title: "Loop"
	blocks: [{
		entry: {
			id:        "SKILL-FLEET-LOOP-001a"
			role:      "step"
			statement: "RUN `$SKILL_FLEET_HEARTBEAT_STATUS status --fleet <fleet> --format json` on every wake; decide only from it and the journal."
			title:     "Read status"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-001b"
			role:      "stop"
			statement: "If fleet `target_state` is `missing` ⇒ STOP: relay `target <target> missing in repo scan`; no merge."
			title:     "Target missing"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-LOOP-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "combine — both sides share enclosing symbols, one change reads cleaner than two."
			}, {
				label:  "param"
				marker: "*"
				text:   "sequence — second applies on top of first; second waits for first merge."
			}, {
				label:  "param"
				marker: "*"
				text:   "hold — sides contradict; held until one is refined."
			}]
			role:      "step"
			statement: "READ both sides' work of each [Unhandled collision](#skill-fleet-def-006); DECIDE combine, sequence or hold; RECORD `decided` naming both members with the reason."
			title:     "Judge collisions"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-003"
			role:      "step"
			statement: "SEND each attended member a decision that binds it, `to` its title; one `ListAgents` lookup on send error."
			title:     "Tell members"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-004"
			role:      "step"
			statement: "SEND each attended [Behind](#skill-fleet-def-012) member the [Restack ask](#skill-fleet-tpl-005); RECORD `decided`; it lands once it is delivered again."
			title:     "Drift"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-005a"
			role:      "step"
			statement: "RUN `git merge --ff-only <branch>` per [Landable](#skill-fleet-def-005) member not held or sequenced behind an unlanded one; RECORD `merged`."
			title:     "Merge"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-005b"
			role:      "step"
			statement: "For a delivered [Worker](#skill-fleet-def-009)'s [Behind](#skill-fleet-def-012) branch CALL Skill `restack` with [Restack args](#skill-fleet-tpl-001), then RUN [Worker landing](#skill-fleet-tpl-006); RECORD `merged`."
			title:     "Land worker"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-005c"
			role:      "stop"
			statement: "If restack stops ⇒ STOP: relay its reason; HEAD is back on `<merge_branch>`, the work branch kept; no further landing this wake."
			title:     "Restack stopped"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-LOOP-008e"
			items: [{
				label:  "param"
				marker: "*"
				text:   "done states a queue place ⇒ END turn; heartbeat's grant message wakes — call done again."
			}, {
				label:  "param"
				marker: "*"
				text:   "done ends its turn abandoned ⇒ RECORD `note` subject `sync blocked: <reason>`; END turn, retry next wake."
			}]
			role:      "step"
			statement: "When `target_state` is `integrated` and not [Synced](#skill-fleet-def-008): CALL Skill `done` — it lands the merge branch on the target inside the target's landing turn."
			title:     "Sync"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-008f"
			role:      "stop"
			statement: "If done states `no-target <base>` ⇒ STOP: relay the line; state the fleet target is no landing target, so nothing syncs; no retry."
			title:     "Target not landable"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-LOOP-008c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "unsubscribe: `$SKILL_FLEET_HEARTBEAT_UNSUBSCRIBE unsubscribe --finish` releases joined members, never attended ones; relay `skipped` lines verbatim."
			}]
			role:      "step"
			statement: "When [Synced](#skill-fleet-def-008): RUN `$SKILL_FLEET_AGENTWT_REMOVE remove --delete-branch <branch>` per delivered worker and own original `claude/<name>` branch; RECORD `note` with merged members; RUN unsubscribe."
			title:     "Finish"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-008d"
			role:      "stop"
			statement: "If [Finish](#skill-fleet-loop-008c) ran ⇒ STOP: state the target synced, tell the human to run `/close` here."
			title:     "Done"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-LOOP-009"
			role:      "step"
			statement: "END turn; next wake re-enters Loop."
			title:     "Yield"
		}
	}]
	kind: "phase"
}, {
	title:   "Feature playbook"
	between: "LOOP-005c..LOOP-008e"
	blocks: [{
		entry: {
			id:        "SKILL-FLEET-FEATURE-001"
			role:      "def"
			statement: "Attended member whose branch carries a `work_items` plan dir; delivered means its plan is final."
			title:     "Design member"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-FEATURE-002"
			role:      "rule"
			statement: "Orchestrator MUST NOT run a second implementation worker while one runs; queue further plans, one at a time."
			title:     "One implementer"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-FEATURE-003"
			role:      "rule"
			statement: "Orchestrator MUST land a design member's plan on the merge branch before launching the worker that implements it."
			title:     "Plan first"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-FEATURE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "[Worker flags](#skill-fleet-def-010) plus `-keep-stopped`, name `<fleet>-impl-<plan slug>` lower case."
			}, {
				label:  "param"
				marker: "*"
				text:   "`-attended` starts the cell without the headless marker, so its `/fimplement` prompt applies the plan's frozen rules."
			}, {
				label:  "param"
				marker: "*"
				text:   "combined plans — first plan as plan file, rest via errata; `OPEN` rows decided by orchestrator, passed as errata."
			}]
			role:      "step"
			statement: "RUN [Worker launch](#skill-fleet-tpl-004) for the oldest merged, unimplemented plan: skill `fimplement`, args [Impl args](#skill-fleet-tpl-002), `-attended`; RECORD `spawned` and `assigned`."
			title:     "Implement"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-FEATURE-005a"
			role:      "step"
			statement: "On a worker result `done`: RUN the delivered call of [Worker launch](#skill-fleet-tpl-004); its branch then lands by [Merge](#skill-fleet-loop-005a), or by [Land worker](#skill-fleet-loop-005b) when behind."
			title:     "Worker done"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-FEATURE-005b"
			role:      "step"
			statement: "On a worker result `stopped`: SEND the design member the result's question verbatim, RECORD `decided` hold with the result's `session_id`; the worker stays registered."
			title:     "Worker blocked"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-FEATURE-005c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "plan at `attempts` [Attempt](#skill-fleet-def-011)s — hold it and RECORD why; never one more cell."
			}]
			role:      "step"
			statement: "On a worker result `failed`: RUN the remove call of [Worker launch](#skill-fleet-tpl-004); RECORD `note`; launch a fresh worker while attempts remain."
			title:     "Worker failed"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-FEATURE-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "resume args — `merge <merge_branch> into your branch, re-read <plan file>, continue`; same flags as the launch."
			}]
			role:      "step"
			statement: "When a held plan's design member is delivered again with new commits, merge it and RUN the resume call of [Worker launch](#skill-fleet-tpl-004) for the stopped worker, or launch per [Implement](#skill-fleet-feature-004) if none is stopped."
			title:     "Refined plan"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-FEATURE-007"
			role:      "stop"
			statement: "If fleet `target_state` is `integrated` and a merged plan has no merged implementation or a plan is held ⇒ STOP: no Sync this wake; RECORD `note` subject `sync held: <plan> queued`, once per plan."
			title:     "Queue before sync"
		}
	}]
	kind: "route"
	pred: "playbook = feature"
}, {
	title:   "Design playbook"
	between: "LOOP-005c..LOOP-008e"
	blocks: [{
		entry: {
			id:        "SKILL-FLEET-DESIGN-001"
			role:      "def"
			statement: "Split-table row of the `master` plan on the merge branch: slug and Depends on. Its plan file is `design/<slug>.md` in the row's plan dir of the master's feature folder."
			title:     "Row"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-DESIGN-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "section check: `$SKILL_FLEET_PLAN_SECTIONS plan-sections <plan file>`."
			}]
			role:      "def"
			statement: "Row's plan file is on the merge branch and its section check prints `ok — code gate`."
			title:     "Row done"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DESIGN-003"
			role:      "def"
			statement: "Row with no worker launched for it and every Depends-on row [Row done](#skill-fleet-design-002)."
			title:     "Row ready"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DESIGN-004"
			role:      "rule"
			statement: "Orchestrator MUST NOT run more than `designers` designer workers at once."
			title:     "Designer cap"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DESIGN-005"
			role:      "rule"
			statement: "Orchestrator MUST NOT launch an implementation worker under this playbook; the run ends with every plan through its code gate."
			title:     "No implementation"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DESIGN-006"
			role:      "step"
			statement: "READ the master's split table; RUN the [Row done](#skill-fleet-design-002) section check per merged row plan not yet recorded done; RECORD `note` subject `row done: <slug>` once per row."
			title:     "Read rows"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-DESIGN-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "[Worker flags](#skill-fleet-def-010), name `<fleet>-<slug>` lower case."
			}, {
				label:  "param"
				marker: "*"
				text:   "never `-attended` — the headless marker gives the cell its one freeze."
			}]
			role:      "step"
			statement: "RUN [Worker launch](#skill-fleet-tpl-004) per [Row ready](#skill-fleet-design-003): skill `fdesign`, args `derived <master> <slug> --nightly`; RECORD `spawned` and `assigned`."
			title:     "Design"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DESIGN-008a"
			role:      "step"
			statement: "On a worker result `done`: RUN the delivered call of [Worker launch](#skill-fleet-tpl-004); its branch then lands as any [Landable](#skill-fleet-def-005) member."
			title:     "Designer done"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-DESIGN-008b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "unmerged worker — the remove call of Worker launch, then the original args under name `<fleet>-<slug>-r<k>`, k the attempt number."
			}, {
				label:  "param"
				marker: "*"
				text:   "merged, unfinished plan — args `<plan file> --nightly finish the code gate`, same retry name."
			}, {
				label:  "param"
				marker: "*"
				text:   "row at `attempts` [Attempt](#skill-fleet-def-011)s — hold it and RECORD why with the result line; never one more cell."
			}]
			role:      "step"
			statement: "On a worker result `failed` or `stopped`, or a merged plan not [Row done](#skill-fleet-design-002): RECORD `note`, RUN [Worker launch](#skill-fleet-tpl-004) again for the row while attempts remain."
			title:     "Designer failed"
		}
	}, {
		entry: {
			id:        "SKILL-FLEET-DESIGN-009"
			role:      "stop"
			statement: "If fleet `target_state` is `integrated` and a row is held or not [Row done](#skill-fleet-design-002) ⇒ STOP: no Sync this wake; RECORD `note` subject `sync held: <slug>`, once per row."
			title:     "Rows before sync"
		}
	}]
	kind: "route"
	pred: "playbook = design"
}, {
	title: "Templates"
	blocks: [{
		entry: {
			id: "SKILL-FLEET-TPL-001"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Restack args"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-TPL-002"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Impl args"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-TPL-004"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Worker launch"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-TPL-005"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Restack ask"
		}
	}, {
		entry: {
			id: "SKILL-FLEET-TPL-006"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Worker landing"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "high"
			reason: "collision judgment, merge and gate-relay decisions across members; high-blast work per [model suitability]($AGENT_CONTEXT_DIR_DEFAULT/facts/claude-configs.md)"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "drives a live heartbeat fleet and syncs its target branch"
}
