changelog: [{
	date:    "2026-10-22"
	version: "6.2.0"
	changes: ["--clean switch: Discard leftovers runs only with it; without it dirty(n) and untracked(n) still refuse the close and the fix path says /close --clean.", "CLEAN variable (clean@1, repo-scoped, optional): with --clean the repo's cleanup commands run after the discard, one Bash call per line in order, a red line stops before Check; granted as Bash($SKILL_CLOSE_CLEAN), one rule per line.", "Args are the table form now, three flags."]
	tokens: 4334
}, {
	date:    "2026-10-21"
	version: "6.1.0"
	changes: ["Discard leftovers step before Check: tracked modifications are restored (git restore --staged --worktree), untracked entries outside the binary's infra exclusions removed (git clean -fd, granted) — /close is the word that nothing uncommitted is work, so dirty(n) and untracked(n) no longer stop a close.", "Leftovers are junk rule: uncommitted state at /close is discarded, never committed or handed back as a fix path; the binary's unpicked gate still guards committed work.", "Projection strip step, its PROJECT_STRIP variable and grant are gone: the restore reverts projection blocks with every other edit.", "Report skip's fix path per reason: dirty or untracked re-runs /close, an operation is finished or aborted, unpicked commits land on a non-claude branch first."]
	tokens: 3985
}, {
	date:    "2026-10-16"
	version: "6.0.0"
	changes: ["--terminal is back as the only session-archive switch: without it the session stays open, also in a main checkout or on a non-agent branch, where nothing is removed.", "--archive: after a passing removal check, before Remove, the wip plan dirs this session's own commits touched are copied into the plans-archive repo ($SKILL_CLOSE_PLANS_ARCHIVE, optional repo-scoped) as done dirs and committed there; the code repo stays untouched."]
	tokens: 3734
}, {
	date:    "2026-10-15"
	version: "5.0.0"
	text:    "Archives always: --terminal is gone. After a removal, or at once when the session is in no pool worktree or on no agent branch, the Desktop session is archived; a refused or skipped removal still stops before it."
	tokens:  2693
}, {
	date:    "2026-10-14"
	version: "4.10.0"
	text:    "Interfaces: one variable per capability — AGENTWT_REMOVE, HEARTBEAT_LEAVE, PROJECT_STRIP"
	tokens:  2722
}, {
	date:    "2026-10-13"
	version: "4.9.0"
	text:    "Programs are env variables: $SKILL_CLOSE_AGENTWT (required; check, restack-branch and worktree removal, force route), $SKILL_CLOSE_HEARTBEAT (optional; leave fleet), $SKILL_CLOSE_STRIP (optional acdsl program; projection strip, granted). Binary provenance loses its build and install hint."
	tokens:  2672
}, {
	date:    "2026-10-05"
	version: "4.8.0"
	text:    "Projection strip step: before Pre-flight report and Check, ACDSL repos strip working-copy projection blocks so a read-only session no longer fails the removal check as dirty."
	tokens:  2707
}, {
	date:    "2026-10-04"
	version: "4.7.0"
	text:    "Requires agentwt: deploy skips the skill without them. heartbeat steps tagged with needs: partial installs and the bundle drop them."
	tokens:  2583
}, {
	date:    "2026-10-04"
	version: "4.6.1"
	text:    "Untracked-gate infra exclusions no longer list .serena/ (Serena support removed); the binary lifts a worktree lock only where one is left — no SessionStart hook locks worktrees any more."
	tokens:  2578
}, {
	date:    "2026-10-02"
	version: "4.6.0"
	text:    "Remove restack branches step: before Remove, the session's claude-restack/<slug>-<8 hex> work and -orig archive branches go through the same safety-gated remove; a skipped one is relayed and never stops the close."
	tokens:  2585
}, {
	date:    "2026-09-28"
	version: "4.5.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2340
}, {
	date:    "2026-09-28"
	version: "4.5.0"
	text:    "Leave fleet step: after a passing check the session leaves its heartbeat fleet (smine-heartbeat leave, failure ignored) before the removal, so a closed session is no member and an empty fleet dissolves."
}, {
	date:    "2026-09-28"
	version: "4.4.0"
	text:    "Accepts every agent-owned branch (claude/* or claude-*/*, agentwt's rule), so an orchestrator on claude-orch/<fleet> closes itself; the removal commands take the full branch name."
}, {
	date:    "2026-09-28"
	version: "4.3.0"
	text:    "A passing removal check renames the session to CLOSED; DONE now belongs to /done (a fleet member's delivered work)."
}, {
	date:    "2026-09-26"
	version: "4.2.0"
	text:    "Check step: remove --dry-run before removal; a refusal (exit 3) stops before anything changes; a passing check renames the session to DONE via the sessiontitle hook before Remove runs."
}, {
	date:    "2026-09-25"
	version: "4.1.2"
	text:    "Routine worktree lifecycle row points at the Go routine runner"
}, {
	date:    "2026-09-25"
	version: "4.1.1"
	text:    "Skip reasons name operation(<op>): the binary refuses removal while a rebase, bisect, cherry-pick, merge or revert is in progress in the worktree."
}, {
	date:    "2026-09-25"
	version: "4.1.0"
	text:    "--terminal flag: after removal archives the Desktop session; default keeps the session open and states that a continued session lands in the main checkout (Desktop fallback, branch gone)."
}, {
	date:    "2026-09-25"
	version: "4.0.1"
	text:    "Reachability wording: worktrunk dropped from the binary; containment is the gix ancestor test or applied/picked own commits, base branch first, default next, any other candidate last."
}, {
	date:    "2026-09-25"
	version: "4.0.0"
	text:    "remove_agent_worktrees.sh replaced by the smine-agentwt binary (remove --delete-branch); reachability now covers applied and picked commits (three-way merge simulation, patch-id twin); toolset copy under ~/.claude/agents/tools retired."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kinds (context/phase), pred/post clauses on 8 entries, sibling atoms.json (10 bool atoms), three queries under queries/, Definitions/Rules split out of Procedure."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use -> Routing (Purpose/Disambiguation/Preconditions/Pipeline), split intro to one-sentence lead"
}, {
	date:    "2026-09-05"
	version: "1.8"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-08-26"
	version: "1.7"
	text:    "Unified script references on the agent-toolset path ~/.claude/agents/tools/remove_agent_worktrees.sh (manual + invocation), tilde form matching the allowlist rule"
}, {
	date:    "2026-08-26"
	version: "1.6"
	text:    "documented .serena/ and .DS_Store in the untracked-gate infra exclusions (remove script change)"
}, {
	date:    "2026-08-23"
	version: "1.5"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-07-31"
	version: "1.4"
	text:    "genericized runner path to $HOME and persona reference (MIT release cleanup)"
}, {
	date:    "2026-07-30"
	version: "1.3"
	text:    "moved under skills/orchestration/ group; name and behavior unchanged"
}, {
	date:    "2026-07-26"
	version: "1.2"
	text:    "drop the cd — the script now normalizes its own cwd to the main checkout; invocation is a single plain command"
}, {
	date:    "2026-07-26"
	version: "1.1"
	text:    "invoke the deployed script at ~/.claude/agents/tools/ (no permission prompt) instead of the main-checkout path"
}, {
	date:    "2026-07-24"
	version: "1.0"
	text:    "initial version"
}]
env: {
	AGENTWT_REMOVE: {
		interface: "agentwt-remove@1"
		required:  true
	}
	CLEAN: {
		interface: "clean@1"
	}
	HEARTBEAT_LEAVE: {
		interface: "heartbeat-leave@1"
	}
	PLANS_ARCHIVE: {
		interface: "plans-archive@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " close"
}, {
	key:   "description"
	value: " Leave the fleet, remove the done session's worktree and claude/* branch, safety-gated; stays open unless terminal. Trigger on /close or \"this session is done, clean up the worktree\". Args — --clean: discard uncommitted leftovers and run the repo's cleanup commands first; --archive: also copy the session's plan dirs into the plans-archive repo; --terminal: also archive the Desktop session."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 6.2.0"
}, {
	key:   "argument-hint"
	value: " --clean --archive --terminal"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Read($SKILL_CLOSE_PLANS_ARCHIVE/**), Bash($SKILL_CLOSE_CLEAN), Bash(cp -R *), Bash(git -C * add *), Bash(git -C * commit *), Bash(git -C * rm *), Bash(git clean -fd *), Bash(mkdir -p *)"
}]
preamble: """
	# Close

	End-of-session cleanup: removes session's `.claude/worktrees/<name>` pool worktree, deletes its `claude/*` branch;
	`--clean` discards uncommitted leftovers first. Safety-gated on committed work living elsewhere.

	Session stays open by default; `--terminal` archives it, `--archive` copies its plan dirs into the plans-archive repo first.
	"""
requires: ["agentwt"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "done"
				}, {
					kind:  "from"
					skill: "fleet"
				}]
				note:       "terminal — nothing hands off"
				standalone: true
			}
			preconditions: ["session runs inside `.claude/worktrees/<name>` pool worktree on an agent-owned branch (`claude/*` or `claude-*/*`); any other checkout is left as is", "work already lives on non-claude branch (binary's gate enforces) — merge or cherry-pick first; skill never moves commits"]
			purpose: "ends session — removes pool worktree and `claude/*` branch once work lives elsewhere."
			routes: [{
				for: "bulk cleanup of many agent worktrees, or forced removal"
				use: "`$SKILL_CLOSE_AGENTWT_REMOVE remove --force` manually"
			}, {
				for: "routine (launchd) worktree lifecycle"
				use: "`internal/routinerun/worktree.go` (`bin/routine`)"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "--clean"
				default: "absent — a dirty tree stops the close"
				effect:  "discard uncommitted leftovers, then run the repo's cleanup commands, before the check — [Discard leftovers](#skill-close-proc-012)"
				form:    "flag"
			}, {
				name:    "--archive"
				default: "absent"
				effect:  "before removal copy the session's plan dirs into the plans-archive repo — [Find plans](#skill-close-proc-010a); needs the archive root set on the skill page"
				form:    "flag"
			}, {
				name:    "--terminal"
				default: "absent — session stays open"
				effect:  "after removal archive the Desktop session — [Archive session](#skill-close-proc-007)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id:        "SKILL-CLOSE-PROC-006a"
			role:      "def"
			statement: "`$SKILL_CLOSE_AGENTWT_REMOVE remove` is the worktree removal — the one agents, the config server and manual runs use."
			title:     "Binary provenance"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-006c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "applied/picked = three-way merge simulation of the commit, else patch-id twin on the candidate."
			}]
			role:      "def"
			statement: "Binary removes worktree only when a non-agent branch contains the work: tip ancestor, or every own commit applied/picked there — base branch first, default next, any other candidate last."
			title:     "Reachability check"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-CLOSE-PROC-001"
			pred:      "not force_used"
			role:      "rule"
			statement: "Close MUST NOT take arguments besides `--clean`, `--archive` and `--terminal`, NEVER passes `--force` — forced removal is manual `$SKILL_CLOSE_AGENTWT_REMOVE remove --force` invocation outside skill."
			title:     "No force"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-006b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "binary lifts a `git worktree lock` where a worktree still carries one."
			}, {
				label:  "param"
				marker: "*"
				text:   "binary excludes infra entries (`.idea/`, `.claude/`, `.claude-worktree` sentinel, `.DS_Store`, tool droppings) from untracked gate."
			}]
			pred:      "not safety_duplicated"
			role:      "rule"
			statement: "Binary MUST own all safety logic — skill NEVER duplicates it here."
			title:     "Owns safety logic"
		}
	}, {
		entry: {
			id:        "SKILL-CLOSE-PROC-011"
			role:      "rule"
			statement: "Without `--clean` uncommitted state MUST stay untouched — the check refuses it; with `--clean` it is discarded, never committed or handed back as a fix path."
			title:     "Leftovers switch"
		}
	}]
	kind: "context"
}, {
	title: "Procedure"
	blocks: [{
		entry: {
			id: "SKILL-CLOSE-PROC-002a"
			post: ["worktree_located"]
			role:      "step"
			statement: "RESOLVE session worktree and branch: `git rev-parse --show-toplevel`, `git rev-parse --abbrev-ref HEAD`."
			title:     "Locate"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-002c"
			post: ["nothing_to_remove"]
			pred:      "off_pattern"
			role:      "step"
			statement: "STATE nothing to remove when toplevel is not under `.claude/worktrees/` or branch's first segment is neither `claude` nor `claude-*`: checkout, branch and plans stay."
			title:     "Off-pattern"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-002b"
			post: ["main_checkout_resolved"]
			pred:      "not off_pattern"
			role:      "step"
			statement: "RESOLVE main checkout as first entry of `git worktree list --porcelain`."
			title:     "Main checkout"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-003"
			post: ["preflight_reported"]
			pred:      "main_checkout_resolved"
			role:      "step"
			statement: "STATE worktree path, branch, main checkout, whether uncommitted changes exist (`git status --porcelain`). No confirmation question — invoking /close is confirmation."
			title:     "Pre-flight report"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-012"
			items: [{
				label:  "param"
				marker: "*"
				text:   "infra exclusions: the untracked-gate list of [Owns safety logic](#skill-close-proc-006b) — those entries stay, the binary ignores them too."
			}, {
				label:  "param"
				marker: "*"
				text:   "one state-changing command per call; a projection block is a tracked modification and goes with the restore."
			}]
			post: ["leftovers_discarded"]
			pred:      "preflight_reported and clean_requested"
			role:      "step"
			statement: "With `--clean` RUN `git restore --staged --worktree -- .`, then `git clean -fd -- <path>…` over the pre-flight's untracked entries outside the infra exclusions."
			title:     "Discard leftovers"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-013"
			items: [{
				label:  "param"
				marker: "*"
				text:   "the repo's own cleanup, set on the skill page — build caches, sandbox homes, read-only trees git cannot delete; each line a separate Bash call."
			}]
			pred:      "leftovers_discarded"
			role:      "step"
			statement: "With `--clean` RUN `$SKILL_CLOSE_CLEAN`, one Bash call per line, in order."
			title:     "Repo cleanup"
		}
	}, {
		entry: {
			id:        "SKILL-CLOSE-PROC-014"
			pred:      "clean_requested and cleanup_failed"
			role:      "stop"
			statement: "If a `$SKILL_CLOSE_CLEAN` line exits non-zero ⇒ STOP before Check: relay its output; the lines after it do not run, worktree and branch stay."
			title:     "Cleanup failed"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-008a"
			items: [{
				fence: {
					body: """
						$SKILL_CLOSE_AGENTWT_REMOVE remove --dry-run --delete-branch <branch>

						"""
					marker: "```"
				}
			}, {
				label:  "param"
				marker: "*"
				text:   "rename instruction comes from the sessiontitle hook (`cmd/sessiontitle`), which matches the `remove --dry-run` command on PostToolUse."
			}, {
				label:  "param"
				marker: "*"
				text:   "`<branch>` is the full name from Locate, e.g. `claude/x` or `claude-orch/T1`."
			}]
			post: ["removal_checked"]
			pred:      "preflight_reported and (not clean_requested or leftovers_discarded)"
			role:      "step"
			statement: "RUN removal check, Remove's command plus `--dry-run`, one command; a passing check yields a session-title rename to CLOSED — carry it out before Remove:"
			title:     "Check"
		}
	}, {
		entry: {
			id:        "SKILL-CLOSE-PROC-008b"
			pred:      "removal_checked and check_refused"
			role:      "stop"
			statement: "If check exits 3 ⇒ STOP: relay stderr verbatim, reasons and fix path as in [Report skip](#skill-close-proc-005a); nothing removed, title unchanged."
			title:     "Report refusal"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-008c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "a closed session is no fleet member; its fleet dissolves once no member and no orchestrator remain."
			}]
			needs: ["heartbeat"]
			pred:      "removal_checked and not check_refused"
			role:      "step"
			statement: "RUN `$SKILL_CLOSE_HEARTBEAT_LEAVE leave`; any non-zero exit (not a joined session, daemon unreachable) is ignored — never blocks Remove."
			title:     "Leave fleet"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-010a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "lists only commits the session made on `<branch>` (its reflog); merged-in and restacked commits stay out."
			}, {
				label:  "param"
				marker: "*"
				text:   "archive root `$SKILL_CLOSE_PLANS_ARCHIVE`; `<repo-name>`: basename of the main checkout."
			}, {
				label:  "param"
				marker: "*"
				text:   "`<plans>` empty ⇒ state no plan to archive; copy and commit are skipped."
			}]
			post: ["plans_found"]
			pred:      "removal_checked and not check_refused and archive_requested"
			role:      "step"
			statement: "With `--archive` RUN `git log -g --grep-reflog=commit --no-merges --format= --name-only <branch> -- plans`; `<plans>`: each distinct `plans/<feature folder>/*-wip-*` dir it names, tracked at HEAD."
			title:     "Find plans"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-010b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<hash>`: `git rev-parse --short HEAD`; created_at and slug: the wip dir's own."
			}, {
				label:  "param"
				marker: "*"
				text:   "a `<created_at>-<slug>-done-*` dir already there goes first: `git -C $SKILL_CLOSE_PLANS_ARCHIVE rm -r <that dir>`."
			}]
			post: ["plans_copied"]
			pred:      "plans_found"
			role:      "step"
			statement: "COPY each dir of `<plans>` to `$SKILL_CLOSE_PLANS_ARCHIVE/<repo-name>/<feature folder>/<created_at>-<slug>-done-<hash>/` via `mkdir -p` of the folder, then `cp -R`."
			title:     "Copy plans"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-010c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "message `archive: <repo-name>/<feature folder>`; several folders comma-joined."
			}, {
				label:  "param"
				marker: "*"
				text:   "code repo stays untouched — the feature's landing archives its plans again and replaces this copy."
			}]
			post: ["plans_archived"]
			pred:      "plans_copied"
			role:      "step"
			statement: "COMMIT via `git -C $SKILL_CLOSE_PLANS_ARCHIVE`, never `cd`, one command per call: `add <repo-name>/<feature folder>` per folder, then one `commit -m`."
			title:     "Commit archive"
		}
	}, {
		entry: {
			id:        "SKILL-CLOSE-PROC-010d"
			pred:      "archive_requested and archive_failed"
			role:      "stop"
			statement: "If a copy or archive command fails ⇒ STOP before Remove: relay the error; worktree and branch stay, a re-run replaces the copies."
			title:     "Archive failed"
		}
	}, {
		entry: {
			id:        "SKILL-CLOSE-PROC-010e"
			pred:      "archive_requested and archive_root_unset"
			role:      "stop"
			statement: "If `--archive` is given and this page shows no Find plans step ⇒ STOP before Remove: the plans-archive root is unset — set it on the skill page."
			title:     "Archive root unset"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-009"
			items: [{
				fence: {
					body: """
						$SKILL_CLOSE_AGENTWT_REMOVE remove --delete-branch <restack branch>

						"""
					marker: "```"
				}
			}, {
				label:  "param"
				marker: "*"
				text:   "`<slug>`: session branch lower-cased, every run outside a-z and 0-9 one dash — `claude/x-1ef1c4` gives `claude-x-1ef1c4`."
			}, {
				label:  "param"
				marker: "*"
				text:   "one command per branch, before Remove — the worktree still exists; a skipped branch never stops close."
			}]
			pred:      "removal_checked and not check_refused"
			role:      "step"
			statement: "LIST `git branch --list 'claude-restack/<slug>-*'`; RUN the remove below for each name whose rest after `<slug>-` is 8 hex, optionally `-orig`; relay skipped lines, go on:"
			title:     "Remove restack branches"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-004"
			items: [{
				fence: {
					body: """
						$SKILL_CLOSE_AGENTWT_REMOVE remove --delete-branch <branch>

						"""
					marker: "```"
				}
			}]
			post: ["script_run"]
			pred:      "removal_checked and not check_refused"
			role:      "step"
			statement: "RUN deployed binary as one command, no `cd`, no compound — it resolves the repo from the worktree, safe to invoke from inside dying worktree:"
			title:     "Remove"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-005a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "skip reasons — `dirty(n)`, `untracked(n)`, `unpicked(n)` (commits on no non-claude branch), `work contained nowhere`."
			}, {
				label:  "param"
				marker: "*"
				text:   "skip reason `operation(<op>)` — rebase, bisect, cherry-pick, merge or revert in progress in the worktree."
			}, {
				label:  "param"
				marker: "*"
				text:   "fix path — `dirty`/`untracked`: re-run `/close --clean`, [Discard leftovers](#skill-close-proc-012) drops them; `operation`: finish or abort it."
			}, {
				label:  "param"
				marker: "*"
				text:   "fix path — `unpicked`: cherry-pick/merge to a non-claude branch, re-run /close."
			}]
			pred:      "script_run and script_skipped"
			role:      "stop"
			statement: "RELAY binary output verbatim. If binary reports skip ⇒ STOP: report reasons, fix path."
			title:     "Report skip"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-005b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "with `--archive` also name each plan dir with its archive path and the archive commit SHA."
			}]
			post: ["removal_reported"]
			pred:      "script_run and not script_skipped"
			role:      "step"
			statement: "STATE on removal worktree and branch gone; without `--terminal` session stays open — next message continues it in main checkout (Desktop fallback, branch gone)."
			title:     "Report removal"
		}
	}, {
		entry: {
			id: "SKILL-CLOSE-PROC-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "archiving stops the session's process and cleans up its worktree, so it never precedes a removal."
			}, {
				label:  "param"
				marker: "*"
				text:   "tool absent (CLI-only session) ⇒ state nothing to archive, end."
			}]
			post: ["session_archived"]
			pred:      "(removal_reported or nothing_to_remove) and terminal_requested"
			role:      "step"
			statement: "With `--terminal` CALL `mcp__ccd_session_mgmt__archive_session` with `session_id` `self`, reason `closed: worktree and branch removed` or `closed: nothing to remove` — last action."
			title:     "Archive session"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "single-command wrapper; all safety logic lives in the worktree tool"
			tier:   "small"
		}
	}]
}]
smoke: {
	skip: "removes the live session's worktree and claude branch"
}
