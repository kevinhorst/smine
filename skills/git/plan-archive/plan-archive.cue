changelog: [{
	date:    "2026-10-15"
	version: "4.6.0"
	text:    "The plans-archive root is set on the skill page; the preamble and the archive-repo precondition say so."
	tokens:  2584
}, {
	date:    "2026-10-15"
	version: "4.5.0"
	text:    "The preamble and the archive-repo precondition name the plans-archive root the Environment page sets instead of the repo-scoped variable; entry statements keep it."
	tokens:  2586
}, {
	date:    "2026-10-14"
	version: "4.4.0"
	text:    "Interfaces: one variable per capability — PLANS_ARCHIVE"
	tokens:  2618
}, {
	date:    "2026-10-13"
	version: "4.3.0"
	text:    "The plans-archive root is the required env variable $SKILL_PLANARCHIVE_ARCHIVE in the preamble, precondition, archive folder, git -C calls and the Read and Edit grants."
	tokens:  2578
}, {
	date:    "2026-10-09"
	version: "4.2.0"
	text:    "The dirty-tree stop checks only tracked changes, so an untracked cache no longer fires it. A done dir left in plans/ is removed with the plans only when diff -r finds it equal to its archive copy, else the run stops."
}, {
	date:    "2026-10-04"
	version: "4.1.0"
	text:    "Requires routines: deploy skips the skill without them."
	tokens:  2108
}, {
	date:    "2026-10-04"
	version: "4.0.0"
	text:    "Archives inside a feature's landing instead of after a merge to main: the unit is the feature folder with every wip plan dir in it, the run works on the checked-out tip in any checkout, the done hash is that tip, and an earlier archived copy of the same plan is replaced. Called by /done branch: before its fast-forward."
	tokens:  2108
}, {
	date:    "2026-10-02"
	version: "3.3.0"
	text:    "Plan dirs are resolved inside their feature folder and archived under it."
	tokens:  1722
}, {
	date:    "2026-10-01"
	version: "3.2.1"
	text:    "Pipeline declares to /package-commit — the commit step this skill invokes."
	tokens:  1659
}, {
	date:    "2026-09-28"
	version: "3.2.0"
	text:    "allowed-tools grants the plans archive — no global additional directory anymore; archive commit via git -C, never cd."
	tokens:  1653
}, {
	date:    "2026-09-28"
	version: "3.1.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: pred/post clauses on every rule/step/stop/verify entry, sibling atoms.json (13 bool + 1 int atom, none global-worthy), two queries (happy-path reachability, bad-context stop blocking Copy) — both green; content and section order unchanged."
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
	text:    "Reformatted to skill format v2 — intro one-liner, When to use replaced by Routing (Purpose/Disambiguation/Preconditions/Pipeline), argument-hint added; content identical"
}, {
	date:    "2026-09-05"
	version: "1.2"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-01"
	version: "1.1"
	text:    "Context-entry ids removed from the body; entries declared via frontmatter acdsl-context, plain-language references (context/rules/skills.md)"
}, {
	date:    "2026-09-01"
	version: "1.0"
	text:    "Initial version: post-merge archival of one plan dir into the plans-archive repo (copy-commit-then-remove, wip → done rename per RULE-PLAN-071/073)"
}]
env: {
	PLANS_ARCHIVE: {
		interface: "plans-archive@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " plan-archive"
}, {
	key:   "description"
	value: " Move a landing feature's wip plan dirs from the code repo into the plans-archive repo, renamed wip → done. Trigger on /plan-archive <feature folder or slug>; /done branch: calls it before its fast-forward."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.6.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"2026-10-02-instance-lifecycle-c5fcaa4\""
}, {
	key:   "acdsl-context"
	value: " RULE-PLAN-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Edit($SKILL_PLANARCHIVE_PLANS_ARCHIVE/**), Read($SKILL_PLANARCHIVE_PLANS_ARCHIVE/**), Skill(package-commit), Bash(cp -R *), Bash(git -C * add *), Bash(git -C * commit *), Bash(git -C * rm *), Bash(git ls-files *), Bash(git rev-parse *), Bash(git rm -r *), Bash(git status *), Bash(mkdir -p *)"
}]
preamble: """
	# Plan Archive

	Archival of a landing feature's plan dirs from code repo into plans-archive repo.

	Copy every wip plan dir of a feature folder from code repo's `plans/<feature folder>/` into plans-archive repo
	(the plans-archive root set on the skill page, per plan-dir archival rule in `context/rules/plan.md`), each renamed from
	`<created_at>-<slug>-wip-<init-hash>` to `<created_at>-<slug>-done-<landing-hash>` under its unchanged feature folder, then
	remove them by a commit on the checked-out tip.
	Copy-commit-then-remove ordering is integrity guard — archive commit must exist before anything deleted.
	"""
requires: ["routines"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "package-commit"
				}, {
					kind:  "to"
					skill: "package-commit"
				}]
				note: "runs inside the feature's landing — /done `branch:` calls it after its restack, before its fast-forward"
			}
			preconditions: ["HEAD holds the tip about to land on the staging target — any checkout, worktree included", "no tracked changes", "named feature folders tracked at HEAD, each holding a `*-wip-*` or `*-done-*` plan dir", "archive repo exists at the plans-archive root set on the skill page"]
			purpose: "archive a landing feature's plans — copy every wip plan dir of its feature folder to plans-archive repo, rename wip → done, then remove them on the checked-out tip."
			routes: [{
				for: "bulk one-time plan migrations, plan dirs directly under `plans/`"
				use: "scripted one-off change, not this skill"
			}, {
				for: "landing the feature branch with its plans archived"
				use: "/done `branch: <feature branch>`"
			}]
		}
	}]
}, {
	title: "Procedure"
	blocks: [{
		entry: {
			id:        "SKILL-PLANARCHIVE-PROCEDURE-001"
			role:      "def"
			statement: "`/plan-archive` takes one or more feature folders, each by folder name or by the slug of one of its plans."
			title:     "Arguments"
		}
	}, {
		entry: {
			id:        "SKILL-PLANARCHIVE-PROCEDURE-009"
			role:      "def"
			statement: "`<hash>`: `git rev-parse --short HEAD`, read once before any change — the landing tip the plans were implemented at."
			title:     "Landing hash"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-011"
			items: [{
				label:  "param"
				marker: "*"
				text:   "repo-name: dir basename of the main checkout, the first entry of `git worktree list --porcelain` — never the worktree's."
			}, {
				label:  "param"
				marker: "*"
				text:   "an arg naming a `plans/*-done-*` dir has no feature folder: its archive folder is `$SKILL_PLANARCHIVE_PLANS_ARCHIVE/<repo-name>`."
			}]
			role:      "def"
			statement: "`<archive folder>`: `$SKILL_PLANARCHIVE_PLANS_ARCHIVE/<repo-name>/<feature folder>`."
			title:     "Archive folder"
		}
	}, {
		entry: {
			id:        "SKILL-PLANARCHIVE-PROCEDURE-008"
			pred:      "step_failed => run_halted"
			role:      "rule"
			statement: "Run MUST stop immediately on any step failure; a failure after the archive commit leaves plans in both places — a re-run replaces the archived copies."
			title:     "Failure ordering safe"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-002a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<dirs>`: every `*-wip-*` dir of the resolved folders — a master and its derived plans leave together."
			}, {
				label:  "param"
				marker: "*"
				text:   "`<orphans>`: every tracked `*-done-*` dir of the resolved folders, plus a `plans/*-done-*` dir an arg names directly."
			}]
			post: ["folders_resolved"]
			role:      "step"
			statement: "RESOLVE each arg to `plans/<arg>/`, else to the parent folder of the one `plans/*/*-<arg>-wip-*/` match."
			title:     "Resolve folders"
		}
	}, {
		entry: {
			id:        "SKILL-PLANARCHIVE-PROCEDURE-002c"
			pred:      "folder_unresolved"
			role:      "stop"
			statement: "If an arg names no folder or done dir, a slug matches several plan dirs, or a folder holds neither a `*-wip-*` nor a `*-done-*` dir ⇒ STOP: list candidates, never guess."
			title:     "Unresolved folder stop"
		}
	}, {
		entry: {
			id:        "SKILL-PLANARCHIVE-PROCEDURE-002b"
			pred:      "not tree_clean"
			role:      "stop"
			statement: "If `git status --porcelain --untracked-files=no` is non-empty ⇒ STOP: refuse — the removal commit holds nothing but the plans."
			title:     "Refuse tracked changes"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-003a"
			post: ["dirs_tracked"]
			pred:      "folders_resolved"
			role:      "verify"
			statement: "VERIFY every dir of `<dirs>` tracked at HEAD (`git ls-files <dir>` non-empty); an untracked one leaves `<dirs>` and is named in the report."
			title:     "Verify tracked"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-012a"
			post: ["orphans_verified"]
			pred:      "dirs_tracked"
			role:      "verify"
			statement: "VERIFY each dir of `<orphans>` equal to its copy: `diff -r <dir> <archive folder>/<dir name>` empty; an equal dir joins the removal, never the copy."
			title:     "Orphan done dir"
		}
	}, {
		entry: {
			id:        "SKILL-PLANARCHIVE-PROCEDURE-012b"
			pred:      "orphan_unmatched"
			role:      "stop"
			statement: "If an orphan done dir has no archive copy or `diff -r` is non-empty ⇒ STOP: name it, never delete."
			title:     "Unmatched orphan stop"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-010"
			post: ["earlier_copy_gone"]
			pred:      "dirs_tracked"
			role:      "step"
			statement: "When [Archive folder](#skill-planarchive-procedure-011) holds a `<created_at>-<slug>-done-*` dir of a plan in `<dirs>` RUN `git -C $SKILL_PLANARCHIVE_PLANS_ARCHIVE rm -r <that dir>`."
			title:     "Replace earlier copy"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "created_at and slug = the wip dir's own; `<hash>` per [Landing hash](#skill-planarchive-procedure-009)."
			}]
			post: ["archive_copied"]
			pred:      "dirs_tracked"
			role:      "step"
			statement: "COPY each dir of `<dirs>` to `<archive folder>/<created_at>-<slug>-done-<hash>/` via `mkdir -p` of the folder, then `cp -R`."
			title:     "Copy"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-005"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`git -C $SKILL_PLANARCHIVE_PLANS_ARCHIVE add <repo-name>/<feature folder>` per folder first."
			}, {
				label:  "param"
				marker: "*"
				text:   "then `git -C $SKILL_PLANARCHIVE_PLANS_ARCHIVE commit -m \"archive: <repo-name>/<feature folder>\"`; several folders: one commit, names comma-joined."
			}, {
				label:  "param"
				marker: "*"
				text:   "`<dirs>` empty, only `<orphans>` resolved: nothing copied, no archive commit."
			}]
			post: ["archive_committed"]
			pred:      "archive_copied"
			role:      "step"
			statement: "COMMIT the folders in archive repo via `git -C $SKILL_PLANARCHIVE_PLANS_ARCHIVE` — never `cd`, one command per call."
			title:     "Commit the archive"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-006"
			post: ["dirs_removed"]
			pred:      "archive_committed and orphans_verified"
			role:      "step"
			statement: "REMOVE `<dirs>` and verified `<orphans>` only after the archive commit: `git rm -r <dir>` each, then one commit via /package-commit, deletions staged under their owning group."
			title:     "Remove from repo"
		}
	}, {
		entry: {
			id: "SKILL-PLANARCHIVE-PROCEDURE-007"
			post: ["run_reported"]
			pred:      "dirs_removed"
			role:      "step"
			statement: "REPORT archive and removal commit SHAs, each old in-repo path with its new archive path, every replaced copy and removed orphan."
			title:     "Report"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "deterministic file moves with git bookkeeping; judgment lives in guards, which stop rather than decide"
			tier:   "small"
		}
	}]
}]
smoke: {
	skip: "writes the plans-archive repo and commits a removal on the checked-out branch"
}
