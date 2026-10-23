changelog: [{
	date:    "2026-10-15"
	version: "2.6.0"
	text:    "The audit verify runs the repo's check command one Bash call per line; the archive remove binds its own variable, AGENTWT_REMOVE."
	tokens:  3695
}, {
	date:    "2026-10-15"
	version: "2.5.0"
	text:    "Remove archive step: after the adopting renames the -orig archive goes through the safety-gated remove ($SKILL_RESTACK_AGENTWT remove --delete-branch, granted); a skipped line keeps it. The no-delete rule now ends at adoption."
	tokens:  3659
}, {
	date:    "2026-10-14"
	version: "2.4.0"
	text:    "Interfaces: one variable per capability — AGENTWT_STATUS, AGENTWT_UNPLACED, CHECK, HEARTBEAT_STATUS"
	tokens:  3428
}, {
	date:    "2026-10-13"
	version: "2.3.0"
	changes: ["Programs and commands are env variables: $SKILL_RESTACK_AGENTWT (required; unplaced query, status rows, grant), $SKILL_RESTACK_HEARTBEAT (optional; status row source), $SKILL_RESTACK_CHECK (required; the audit verify).", "Version hunk (REPLAY-003b) deleted: a skill.cue merge is the merge driver's and the repo check's business."]
	tokens: 3406
}, {
	date:    "2026-10-10"
	version: "2.2.0"
	text:    "Version hunk (REPLAY-003b): a skill.cue version conflict keeps both sides' rows, the fresher commit's newest with the higher version, then make skill-render."
	tokens:  3504
}, {
	date:    "2026-10-03"
	version: "2.1.0"
	text:    "Requires agentwt: deploy skips the skill without them."
	tokens:  3396
}, {
	date:    "2026-10-04"
	version: "2.0.0"
	text:    "Restack decides every conflict itself, contradicting intents included, and names each call in its report: the Contradiction stop, the Unresolvable pick stop and the Unattended run: arg are gone. The Verification failed stop stays."
	tokens:  3396
}, {
	date:    "2026-10-03"
	version: "1.1.0"
	text:    "Landing target: the heartbeat row's target, else the base from smine-agentwt status when it is a local branch neither agent-owned nor the default branch; no target and no onto arg stops the run. Guard: a restack onto another branch than the base stops when that branch lacks commits of the base the branch carries."
	tokens:  3562
}, {
	date:    "2026-10-02"
	version: "1.0.0"
	text:    "initial version: replays a branch's unplaced commits (smine-agentwt unplaced) onto its target's tip on a claude-restack work branch, one cherry-pick per commit, verifies ancestry, audit and diff, then adopts by two renames; a stop keeps the branch untouched and the work branch for a resumed rerun."
	tokens:  3129
}]
env: {
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
	CHECK: {
		interface: "check@1"
		required:  true
	}
	HEARTBEAT_STATUS: {
		interface: "heartbeat-status@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " restack"
}, {
	key:   "description"
	value: " Replay a branch's unlanded commits onto its target's tip on a work branch and adopt it after a green audit — never a merge commit. Trigger on /restack or \"restack my branch\" or \"bring my branch onto its target\". Args — branch: branch to restack (default current); onto: target branch (default landing target)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 2.6.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"branch: claude/my-feature onto: feature/heartbeat\""
}, {
	key:   "branch-prefix"
	value: " claude-restack/"
}, {
	key:   "allowed-tools"
	value: " Bash($SKILL_RESTACK_AGENTWT_REMOVE remove --delete-branch claude-restack/*), Bash($SKILL_RESTACK_AGENTWT_UNPLACED unplaced *), Bash(git branch -m *), Bash(git cherry-pick *), Bash(git commit --no-edit)"
}]
preamble: """
	# Restack

	Replays a branch's unlanded commits onto its target's tip — the landing strategy's one conflict operation, never a merge commit.

	Branch keeps its name: replay runs on a work branch, adoption is two renames after a green audit, then the archive goes
	through the safety-gated remove; a stop leaves the branch untouched.
	"""
requires: ["agentwt"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				note:       "called by /fimplement on a branch behind its landing target; output is a branch at the target's tip"
				standalone: true
			}
			preconditions: ["`branch` and `onto` are local branches", "working tree clean, no git operation in progress", "repo has an audit command"]
			purpose: "replay a branch's unlanded commits onto its target's tip so landing it is a fast-forward."
			routes: [{
				for: "merging two long-lived branches, or a repo with a merge-based flow"
				use: "`/merge-resolve`"
			}, {
				for: "file overlap with other sessions before implementing"
				use: "`/overlap`"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "branch"
				default: "checked-out branch"
				effect:  "branch whose unlanded commits are replayed; adopted only when it is the start branch — [Adopt](#skill-restack-adopt-001a)"
				form:    "`branch: <local branch>`"
			}, {
				name:    "onto"
				default: "landing target"
				effect:  "branch whose tip the commits are replayed onto — [Landing target](#skill-restack-def-001)"
				form:    "`onto: <local branch>`"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id: "SKILL-RESTACK-DEF-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "row source: `branches[]` of `$SKILL_RESTACK_HEARTBEAT_STATUS status --format json`."
			}, {
				label:  "param"
				marker: "*"
				text:   "row of the checked-out branch: `worktree` equals `git rev-parse --show-toplevel`; of a named branch: `branch` equals it."
			}, {
				label:  "param"
				marker: "*"
				text:   "agentwt row: the `rows[]` entry whose `branch` equals it in `$SKILL_RESTACK_AGENTWT_STATUS status --format json`."
			}, {
				label:  "param"
				marker: "*"
				text:   "local branch: `git rev-parse --verify refs/heads/<name>` exits 0."
			}, {
				label:  "param"
				marker: "*"
				text:   "agent-owned: the name has its own `rows[]` entry in the same document."
			}, {
				label:  "param"
				marker: "*"
				text:   "default branch: `repo.default_branch` of `$SKILL_RESTACK_AGENTWT_STATUS status --format json`."
			}]
			role:      "def"
			statement: "Landing target of a branch: `target` of its heartbeat status row; no answer or no row — `from.name` of its agentwt row when a local branch neither agent-owned nor the default branch; else none."
			title:     "Landing target"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-DEF-002"
			role:      "def"
			statement: "Slug of a branch: its name lower-cased, every run of characters outside a-z and 0-9 one dash — `claude/x-1ef1c4` gives `claude-x-1ef1c4`."
			title:     "Slug"
		}
	}, {
		entry: {
			id: "SKILL-RESTACK-DEF-003"
			items: [{
				marker: "*"
				text:   "`<archive>` is `<branch>` renamed: it keeps `<branch>`'s reflog, creation line included."
			}]
			role:      "def"
			statement: "Work branch `claude-restack/<slug>-<short>`, archive `claude-restack/<slug>-<short>-orig`; `<short>` is `onto_short_sha` of the first query, 8 hex."
			title:     "Branch names"
		}
	}]
	kind: "context"
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-RESTACK-RULES-001"
			role:      "rule"
			statement: "Skill MUST NOT create a merge commit, force or delete a ref, or change `<branch>` before adoption; after it only `<archive>` goes, through the safety-gated remove."
			title:     "Non-destructive replay"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-RULES-002"
			role:      "rule"
			statement: "Each commit MUST be replayed by its own `git cherry-pick <sha>` call — never with `-x`, never as a range."
			title:     "Single-commit picks"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-RULES-003"
			role:      "rule"
			statement: "Every judgment call — conflict resolution, skipped pick, audit repair — MUST be named in the report."
			title:     "Judgment calls named"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-RULES-004"
			role:      "rule"
			statement: "Skill MUST decide every conflict itself, contradicting intents included — best call from commit and design evidence; NEVER ask the user, NEVER abort a pick as unresolvable."
			title:     "Own judgment"
		}
	}]
	kind: "context"
}, {
	title: "1. Intake"
	blocks: [{
		entry: {
			id:        "SKILL-RESTACK-INTAKE-001"
			role:      "step"
			statement: "RESOLVE `branch` (default `git rev-parse --abbrev-ref HEAD`) and `onto` (default [Landing target](#skill-restack-def-001)); note the start branch."
			title:     "Resolve args"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-INTAKE-006"
			role:      "stop"
			statement: "If `onto` is absent and `<branch>` has no [Landing target](#skill-restack-def-001) ⇒ STOP: report the branch has no landing target; nothing touched."
			title:     "No target"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-INTAKE-002"
			role:      "stop"
			statement: "If `git status --porcelain` lists a tracked change, or a merge, rebase or pick is in progress ⇒ STOP: report it; nothing touched."
			title:     "Unclean tree"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-INTAKE-003"
			role:      "stop"
			statement: "If `git merge-base --is-ancestor <onto> <branch>` exits 0 ⇒ STOP: report `<branch>` already holds `<onto>`'s tip; nothing changed."
			title:     "Already on target"
		}
	}, {
		entry: {
			id: "SKILL-RESTACK-INTAKE-007"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<base>`: `from.name` of the agentwt row of `<branch>` per [Landing target](#skill-restack-def-001); no row or `null` ⇒ no check."
			}, {
				label:  "param"
				marker: "*"
				text:   "`<fork>`: RUN `git merge-base <base> <branch>`."
			}, {
				label:  "param"
				marker: "*"
				text:   "not in `<onto>`: `git merge-base --is-ancestor <fork> <onto>` exits 1."
			}]
			role:      "stop"
			statement: "If base `<base>` is known and not `<onto>`, and `<fork>` is not in `<onto>` ⇒ STOP: report `<onto>` lacks commits of `<base>` the branch carries — bring `<onto>` up to `<base>` first."
			title:     "Foreign base commits"
		}
	}, {
		entry: {
			id: "SKILL-RESTACK-INTAKE-004"
			items: [{
				fence: {
					body: """
						$SKILL_RESTACK_AGENTWT_UNPLACED unplaced --onto <onto> <branch> --format json

						"""
					marker: "```"
				}
			}]
			role:      "step"
			statement: "RUN the placement query; keep `onto_short_sha` and `commits` — the replay list, oldest first:"
			title:     "Query"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-INTAKE-005"
			role:      "stop"
			statement: "If the query exits 3 ⇒ STOP: relay stderr verbatim; never substitute another branch."
			title:     "Unknown branch"
		}
	}]
	kind: "phase"
}, {
	title: "2. Replay on a work branch"
	blocks: [{
		entry: {
			id: "SKILL-RESTACK-REPLAY-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<work>` and `<archive>` per [Branch names](#skill-restack-def-003), slug per [Slug](#skill-restack-def-002)."
			}, {
				label:  "param"
				marker: "*"
				text:   "an existing `<work>` is a stopped run's — resumed, never deleted or recut."
			}]
			role:      "step"
			statement: "RUN `git switch -c <work> <onto>`; when `<work>` exists, `git switch <work>` and repeat the query with `--onto <work>` — its `commits` are what is left to replay."
			title:     "Work branch"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-REPLAY-002"
			role:      "step"
			statement: "RUN `git cherry-pick <sha>` for each listed commit, in list order."
			title:     "Pick"
		}
	}, {
		entry: {
			id: "SKILL-RESTACK-REPLAY-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "ground truth: `git show <onto>:<path>` and `git show <sha>:<path>`, never an intermediate diff."
			}, {
				label:  "param"
				marker: "*"
				text:   "additive hunks keep both sides; of two mechanisms for one behavior keep the designed one."
			}, {
				label:  "param"
				marker: "*"
				text:   "contradicting intents, no evidence deciding: keep `<onto>`'s behavior, fit the commit's change to it, name the call."
			}]
			role:      "step"
			statement: "On a conflict RESOLVE each file by both sides' intent, `git add <paths>`, finish with `git commit --no-edit` — author, date and message survive."
			title:     "Resolve conflict"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-REPLAY-004"
			role:      "step"
			statement: "On a pick that turns out empty RUN `git cherry-pick --skip`."
			title:     "Empty pick"
		}
	}]
	kind: "phase"
}, {
	title: "3. Verify before adoption"
	blocks: [{
		entry: {
			id:        "SKILL-RESTACK-VERIFY-001"
			role:      "verify"
			statement: "VERIFY `git merge-base --is-ancestor <onto> HEAD` exits 0 and `git status --porcelain` is empty."
			title:     "Target held"
		}
	}, {
		entry: {
			id: "SKILL-RESTACK-VERIFY-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "runs after zero picks too."
			}]
			role:      "verify"
			statement: "VERIFY the repo's check green — `$SKILL_RESTACK_CHECK`, one Bash call per line; red: repair by a commit on `<work>`, rerun."
			title:     "Audit green"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-VERIFY-003"
			role:      "verify"
			statement: "VERIFY every file of `git diff --stat <onto> HEAD` traces to a replayed commit, a named resolution or an audit repair."
			title:     "Diff attributed"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-VERIFY-004"
			role:      "stop"
			statement: "If the audit stays red or a file is unattributable ⇒ STOP: `git switch <start branch>`, report; `<work>` kept, `<branch>` untouched."
			title:     "Verification failed"
		}
	}]
	kind: "phase"
}, {
	title: "4. Adopt and report"
	blocks: [{
		entry: {
			id:        "SKILL-RESTACK-ADOPT-001a"
			role:      "step"
			statement: "When `<branch>` was the start branch RUN `git branch -m <branch> <archive>`, then `git branch -m <work> <branch>` — one command per call; HEAD ends on `<branch>`."
			title:     "Adopt"
		}
	}, {
		entry: {
			id: "SKILL-RESTACK-ADOPT-001c"
			items: [{
				fence: {
					body: """
						$SKILL_RESTACK_AGENTWT_REMOVE remove --delete-branch <archive>

						"""
					marker: "```"
				}
			}, {
				label:  "param"
				marker: "*"
				text:   "a `skipped` line is relayed verbatim, `<archive>` kept, the run goes on — the gate decides, never the skill."
			}]
			role:      "step"
			statement: "After the renames RUN the safety-gated remove on `<archive>` — its commits live replayed on `<branch>`:"
			title:     "Remove archive"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-ADOPT-001b"
			role:      "step"
			statement: "For any other `<branch>` RENAME nothing: HEAD stays on `<work>`; the caller moves refs, switches back and deletes `<work>`."
			title:     "Other branch"
		}
	}, {
		entry: {
			id:        "SKILL-RESTACK-ADOPT-002"
			role:      "step"
			statement: "STATE target tip, commits replayed, picks skipped, every judgment call, audit result, and whether `<archive>` was removed or kept (its skipped line), or the work branch name."
			title:     "Report"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "conflict resolution by intent; a misjudged pick ships a silent regression, and a subagent's Skill call activates no grants"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "needs a branch behind its target with commits to replay, which a fixture tree or patch cannot create"
}
