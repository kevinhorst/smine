changelog: [{
	date:    "2026-10-16"
	version: "3.8.0"
	text:    "branch: refuses when the install sets no staging target (heartbeat.staging_target); the setting replaces serve --staging-target"
	tokens:  4159
}, {
	date:    "2026-10-16"
	version: "3.7.0"
	text:    "--close: the chained /close runs without args again, never --terminal — the session stays open."
	tokens:  4027
}, {
	date:    "2026-10-15"
	version: "3.6.0"
	text:    "--close: the chained /close archives the session now that its --terminal flag is gone."
	tokens:  4023
}, {
	date:    "2026-10-14"
	version: "3.5.0"
	text:    "Interfaces: one variable per capability — AGENTWT_FORWARD, AGENTWT_REMOVE, AGENTWT_STATUS, AGENTWT_UNPLACED, HEARTBEAT_DONE, HEARTBEAT_LAND, HEARTBEAT_STATUS"
	tokens:  4027
}, {
	date:    "2026-10-13"
	version: "3.4.0"
	text:    "Programs are required env variables: $SKILL_DONE_AGENTWT (status, unplaced, forward, remove; staging-move template) and $SKILL_DONE_HEARTBEAT (done, land, status) in steps, stops, grants and the template. Archive plans (LAND-012) and the escalates edge to plan-archive need routines."
	tokens:  3957
}, {
	date:    "2026-10-04"
	version: "3.3.0"
	text:    "The fast-forward is smine-agentwt forward: a target checked out in another worktree moves there, index and files included, and is refused only when that worktree has tracked changes; the ancestry check is the command's. --close chains into /close after a landing. Unattended run: is gone — restack never asks. allowed-tools gains Skill(close) and the forward command."
	tokens:  3923
}, {
	date:    "2026-10-04"
	version: "3.2.0"
	text:    "branch: archives the feature's plans inside the landing — after the restack, before the fast-forward, every feature folder the landing range touches goes through /plan-archive on the tip, so the staging target never holds a wip plan; a subject that needs no restack is checked out detached for the removal commit. allowed-tools gains Skill(plan-archive); the pipeline gains its escalates edge."
	tokens:  3813
}, {
	date:    "2026-10-03"
	version: "3.1.0"
	text:    "Requires agentwt, heartbeat: deploy skips the skill without them."
	tokens:  3318
}, {
	date:    "2026-10-03"
	version: "3.0.0"
	text:    "Landing target: a done lands on the target heartbeat resolves for the branch — its base, or the fleet's merge branch — inside that target's turn; a branch without one answers no-target and lands nothing. branch: lands a feature branch on the staging target read from status (develop by default) and refuses the staging target, the default branch and agent branches. Create and catch-up run only for the staging target. allowed-tools: Bash(git branch *) replaces the three develop grants."
	tokens:  3318
}, {
	date:    "2026-10-02"
	version: "2.0.0"
	text:    "Landing flow: a done without a join is accepted for develop; on the grant the skill creates develop, catches it up with the default branch, restacks the branch, fast-forwards develop and ends the turn; every stop ends the turn abandoned. Args branch and Unattended run:. allowed-tools gains the develop branch commands, land, the placement query and Skill(restack); the pipeline is standalone and gains the escalates edge to restack."
	tokens:  2955
}, {
	date:    "2026-09-28"
	version: "1.0.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  494
}, {
	date:    "2026-09-28"
	version: "1.0.0"
	text:    "initial version: reports the joined fleet member delivered through smine-heartbeat done after a clean-tree check; the sessiontitle hook renames the title to DONE."
}]
env: {
	AGENTWT_FORWARD: {
		interface: "agentwt-forward@1"
		required:  true
	}
	AGENTWT_REMOVE: {
		interface: "agentwt-remove@1"
		required:  true
	}
	AGENTWT_STATUS: {
		interface: "agentwt-status@1"
		required:  true
	}
	AGENTWT_UNPLACED: {
		interface: "agentwt-unplaced@1"
		required:  true
	}
	HEARTBEAT_DONE: {
		interface: "heartbeat-done@1"
		required:  true
	}
	HEARTBEAT_LAND: {
		interface: "heartbeat-land@1"
		required:  true
	}
	HEARTBEAT_STATUS: {
		interface: "heartbeat-status@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " done"
}, {
	key:   "description"
	value: " Report this session's work as finished — a fleet member is delivered, any other session lands its branch on its landing target; no target lands nothing. Trigger on /done or \"I'm done, hand it to the orchestrator\" or \"land this branch\". Args — branch: land this feature branch on the staging target instead, its plans archived; --close: after the landing remove this session's worktree and branch through /close."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.8.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "title-code"
	value: " Done"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"branch: feature/heartbeat --close\""
}, {
	key:   "branch-prefix"
	value: " claude-restack/"
}, {
	key:   "allowed-tools"
	value: " Skill(close), Skill(plan-archive), Skill(restack), Bash($SKILL_DONE_AGENTWT_FORWARD forward *), Bash($SKILL_DONE_AGENTWT_UNPLACED unplaced *), Bash($SKILL_DONE_HEARTBEAT_DONE done), Bash($SKILL_DONE_HEARTBEAT_LAND land *), Bash(git branch *)"
}]
preamble: """
	# Done

	Reports this session's work finished. Fleet member: delivered, its orchestrator lands the branch. Any other session:
	accepted for its landing target, lands its own branch inside that target's turn — restack, fast-forward, never a merge
	commit. No target: nothing lands. `branch`: the feature's wip plans leave for the plans archive in the same landing.
	`--close`: the landing chains into /close.
	"""
requires: ["agentwt", "heartbeat"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "join"
				}, {
					kind:  "to"
					skill: "close"
				}, {
					kind:  "escalates"
					skill: "restack"
				}, {
					kind:  "escalates"
					skill: "plan-archive"
					needs: ["routines"]
				}]
				note:       "member: the orchestrator lands the branch, then /close; no fleet: lands on its landing target itself, the human releases the staging target (heartbeat.staging_target) to main"
				standalone: true
			}
			preconditions: ["heartbeat daemon serves this Claude home", "work committed on the session's branch", "outside a fleet: typed by the human — an agent's call is admitted only after /join or inside /fleet"]
			purpose: "marks finished work landable: a member's branch for its orchestrator, any other session's branch onto its landing target."
			routes: [{
				for: "joining a fleet first"
				use: "/join"
			}, {
				for: "replaying a branch onto a target without landing it"
				use: "/restack"
			}, {
				for: "removing this session's worktree after the landing"
				use: "/close"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "branch"
				default: "session's branch"
				effect:  "lands this branch on the staging target instead, its plans archived; refuses staging, default and agent branches — [Request turn](#skill-done-run-005)"
				form:    "`branch: <local branch>`"
			}, {
				name:    "--close"
				default: "absent"
				effect:  "after a landing removes this session's worktree and branch through /close; a member's delivery closes nothing — [Close](#skill-done-land-014)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id: "SKILL-DONE-DEF-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<staging>`: `staging_target` of `$SKILL_DONE_HEARTBEAT_STATUS status --format json`; absent or empty when the install sets none."
			}, {
				label:  "param"
				marker: "*"
				text:   "`<default>`: `repo.default_branch` of `$SKILL_DONE_AGENTWT_STATUS status --format json`."
			}]
			role:      "def"
			statement: "`<target>`: the branch the turn answer names — the landing target heartbeat resolved for `<subject>`, with `branch` the staging target `<staging>`."
			title:     "Landing target"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-DEF-002"
			role:      "def"
			statement: "`<subject>`: the `branch` arg, else the checked-out branch; `<start>`: the branch checked out at invocation (`git rev-parse --abbrev-ref HEAD`)."
			title:     "Subject"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-DEF-004"
			role:      "def"
			statement: "Stdout line: `delivered <token>` for a member; `accepted <target> granted`, `accepted <target> queued <n>` or `no-target <base>` from done; `granted <target>` or `queued <target> <n>` from land."
			title:     "Turn answer"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-DONE-RULES-001"
			role:      "rule"
			statement: "Agent MUST NOT invoke this skill on its own outside a fleet; there the human types it."
			title:     "Human accepts"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RULES-002"
			role:      "rule"
			statement: "Skill MUST end a granted turn on every exit of Land: `land --end landed` after the move, `land --end abandoned` on a stop."
			title:     "Turn always ends"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RULES-003"
			role:      "rule"
			statement: "Skill MUST NOT push, move the default branch, create a merge commit, or force any ref but `<staging>` in [Staging move](#skill-done-tpl-001)."
			title:     "Linear and local"
		}
	}]
	kind: "context"
}, {
	title: "1. Report"
	blocks: [{
		entry: {
			id:        "SKILL-DONE-RUN-001"
			role:      "verify"
			statement: "Before reporting: VERIFY `git status --porcelain` shows no tracked change; else commit it first."
			title:     "Committed"
		}
	}, {
		entry: {
			id: "SKILL-DONE-RUN-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a title rename to `[DONE]` follows from the session-title hook — carry it out."
			}]
			role:      "step"
			statement: "Without `branch` RUN `$SKILL_DONE_HEARTBEAT_DONE done`."
			title:     "Report done"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RUN-007"
			role:      "stop"
			statement: "If `branch` names `<staging>`, `<default>` or a branch with a `rows[]` entry in `$SKILL_DONE_AGENTWT_STATUS status --format json` ⇒ STOP: state `branch` takes a feature branch only; nothing requested."
			title:     "Subject refused"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RUN-009"
			role:      "stop"
			statement: "If `branch` is set and `<staging>` is absent or empty ⇒ STOP: state `no staging target configured — set heartbeat.staging_target`; nothing requested."
			title:     "No staging"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RUN-005"
			role:      "step"
			statement: "With `branch` RUN `$SKILL_DONE_HEARTBEAT_LAND land --target <staging>` instead — the session is neither delivered nor retitled."
			title:     "Request turn"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RUN-003"
			role:      "stop"
			statement: "If the command exits non-zero ⇒ STOP: relay stderr verbatim."
			title:     "Refused"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-RUN-008"
			role:      "stop"
			statement: "If the line is `no-target <base>` ⇒ STOP: state `<subject>` is based on `<base>`, so nothing lands; you pick its commits yourself."
			title:     "No target"
		}
	}, {
		entry: {
			id: "SKILL-DONE-RUN-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "with `--close`: state nothing was closed — the orchestrator lands the branch first, /close comes after."
			}]
			role:      "stop"
			statement: "If the line is `delivered <token>` ⇒ STOP: state it; the next prompt reopens the member."
			title:     "Delivered"
		}
	}, {
		entry: {
			id: "SKILL-DONE-RUN-006"
			items: [{
				label:  "param"
				marker: "*"
				text:   "grant message: `heartbeat: landing turn granted for <target> in <repo>. Continue the landing steps, …`."
			}, {
				label:  "param"
				marker: "*"
				text:   "skill text gone on resume ⇒ invoke this skill again with the same args, `--close` included — the repeated request answers granted."
			}]
			role:      "stop"
			statement: "If the line ends `queued <n>` ⇒ STOP: state the place, END turn; heartbeat's grant message resumes at Land."
			title:     "Queued"
		}
	}]
	kind: "phase"
}, {
	title: "2. Land"
	blocks: [{
		entry: {
			id:        "SKILL-DONE-LAND-001"
			role:      "step"
			statement: "On a granted line RESOLVE `<default>`; when `<staging>` is set, `<target>` is `<staging>` and `git rev-parse --verify <staging>` fails RUN `git branch <staging> <default>`."
			title:     "Create staging"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-002"
			role:      "step"
			statement: "When `<staging>` is set, `<target>` is `<staging>` and `git merge-base --is-ancestor <default> <staging>` fails CALL Skill `restack` (`branch: <staging> onto: <default>`), then RUN [Staging move](#skill-done-tpl-001)."
			title:     "Catch up"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-003"
			role:      "step"
			statement: "When `git merge-base --is-ancestor <target> <subject>` fails RUN `$SKILL_DONE_AGENTWT_UNPLACED unplaced --onto <target> <subject> --format json`; keep `commits`."
			title:     "Placed check"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-004"
			role:      "stop"
			statement: "If the query ran and `commits` is empty ⇒ STOP: RUN `$SKILL_DONE_HEARTBEAT_LAND land --end abandoned`; state `<subject>` already placed on `<target>`."
			title:     "Already placed"
		}
	}, {
		entry: {
			id: "SKILL-DONE-LAND-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "own branch: HEAD ends on it, name unchanged; `<tip>` is HEAD."
			}, {
				label:  "param"
				marker: "*"
				text:   "`branch` arg: HEAD ends on the work branch `<work>` (`git rev-parse --abbrev-ref HEAD`), `<subject>` untouched; `<tip>` is `<work>`."
			}, {
				label:  "param"
				marker: "*"
				text:   "no restack ran: `<tip>` is `<subject>`."
			}]
			role:      "step"
			statement: "When `commits` lists any, CALL Skill `restack` with `branch: <subject> onto: <target>`."
			title:     "Restack"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-006"
			role:      "stop"
			statement: "If a restack call stops ⇒ STOP: RUN `$SKILL_DONE_HEARTBEAT_LAND land --end abandoned`; relay its reason — HEAD is back on `<start>`."
			title:     "Restack stopped"
		}
	}, {
		entry: {
			id: "SKILL-DONE-LAND-011"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<folders>`: each `plans/<feature folder>` with a listed path under one of its `*-wip-*` dirs."
			}, {
				label:  "param"
				marker: "*"
				text:   "a `*-wip-*` dir directly under `plans/` has no feature folder: left in place, named in the end statement."
			}]
			role:      "step"
			statement: "With `branch` RUN `git diff --name-only --diff-filter=d <target> <tip> -- plans` — the plans this landing brings."
			title:     "Find plans"
		}
	}, {
		entry: {
			id: "SKILL-DONE-LAND-012"
			items: [{
				label:  "param"
				marker: "*"
				text:   "after a restack HEAD is on `<work>` already; the removal commit lands with the feature, `<tip>` is HEAD from here on."
			}, {
				label:  "param"
				marker: "*"
				text:   "no implemented check: typing `branch` declares the feature finished, a master's open rows included."
			}]
			needs: ["routines"]
			role:      "step"
			statement: "When `<folders>` lists any: without a restack RUN `git switch --detach <subject>`; then CALL Skill `plan-archive` with the folder names."
			title:     "Archive plans"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-013"
			role:      "stop"
			statement: "If plan-archive stops ⇒ STOP: `git switch <start>` if HEAD left it, RUN `$SKILL_DONE_HEARTBEAT_LAND land --end abandoned`, relay its reason; `<work>` kept."
			title:     "Archive stopped"
		}
	}, {
		entry: {
			id: "SKILL-DONE-LAND-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the command checks `<tip>` holds `<target>`'s tip, then moves the ref; never `git branch -f` here."
			}, {
				label:  "param"
				marker: "*"
				text:   "a `<target>` checked out in another worktree moves there, index and files included; tracked changes there refuse it."
			}]
			role:      "step"
			statement: "RUN `$SKILL_DONE_AGENTWT_FORWARD forward --onto <target> <tip>`."
			title:     "Fast-forward"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-008"
			role:      "step"
			statement: "With `branch`, when HEAD left `<start>`, RUN `git switch <start>`; after a restack then `$SKILL_DONE_AGENTWT_REMOVE remove --delete-branch <work>`."
			title:     "Leave work branch"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-009"
			role:      "stop"
			statement: "If git refuses the create or the staging move, or the forward exits non-zero ⇒ STOP: `git switch <start>` if HEAD left it, RUN `$SKILL_DONE_HEARTBEAT_LAND land --end abandoned`, relay the line."
			title:     "Target unmovable"
		}
	}, {
		entry: {
			id:        "SKILL-DONE-LAND-010"
			role:      "step"
			statement: "RUN `$SKILL_DONE_HEARTBEAT_LAND land --end landed`; STATE `landed <subject> on <target>`, the tip of `<target>`, each plan folder archived or left in place."
			title:     "End turn"
		}
	}, {
		entry: {
			id: "SKILL-DONE-LAND-014"
			items: [{
				label:  "param"
				marker: "*"
				text:   "runs only after `land --end landed`; a stop of Report or Land closes nothing."
			}]
			role:      "step"
			statement: "With `--close` CALL Skill `close` without args — never `--terminal`."
			title:     "Close"
		}
	}]
	kind: "phase"
}, {
	title: "Templates"
	blocks: [{
		entry: {
			id: "SKILL-DONE-TPL-001"
			payload: {
				info:   "text"
				marker: "```"
			}
			role:  "payload"
			title: "Staging move"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "the landing path resolves conflicts through restack and moves a shared branch; the member path is one command; a subagent's Skill call activates no grants"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "reports a member of a live heartbeat fleet as delivered"
}
