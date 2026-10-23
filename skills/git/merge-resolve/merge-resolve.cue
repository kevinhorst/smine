changelog: [{
	date:    "2026-10-17"
	version: "3.11.0"
	text:    "Dropped the goland arg — an IDE MCP server is opted into by the user's message, not a skill arg."
	tokens:  4361
}, {
	date:    "2026-10-16"
	version: "3.10.0"
	text:    "$SKILL_MERGERESOLVE_CHECK is optional: a repo without a check command runs the skill without the green-suite verify instead of aborting the invocation."
	tokens:  4449
}, {
	date:    "2026-10-15"
	version: "3.9.0"
	text:    "The green-suite verify runs the repo's check command one Bash call per line."
	tokens:  4449
}, {
	date:    "2026-10-14"
	version: "3.8.0"
	text:    "Interfaces: one variable per capability — CHECK, MERGE_BRANCH"
	tokens:  4441
}, {
	date:    "2026-10-13"
	version: "3.7.0"
	changes: ["Programs and commands are required env variables: $SKILL_MERGERESOLVE_RULES (merge-branch create and cleanup, grant), $SKILL_MERGERESOLVE_CHECK (the full green suite verify).", "Version hunk (RESOLVE-002g) deleted: a skill.cue merge is the merge driver's and the repo check's business."]
	tokens: 4427
}, {
	date:    "2026-10-10"
	version: "3.6.0"
	text:    "Version hunk (RESOLVE-002g) as in restack; Commit default message (COMMIT-001a) names git commit --no-edit, the one form the gate admits; Worktree commit fallback (COMMIT-001b) removed."
	tokens:  4489
}, {
	date:    "2026-10-07"
	version: "3.5.0"
	text:    "New steps Whole-file take (RESOLVE-002e): a file one side wins whole is taken with `git checkout --ours|--theirs -- <path>` then `git add <path>`; Generated-file regen (RESOLVE-002f): a generated file is taken from either side and regenerated with its make target, never hand-edited or written through a `git show` redirect. allowed-tools gains Bash(git checkout --ours -- *) and Bash(git checkout --theirs -- *)."
}, {
	date:    "2026-10-06"
	version: "3.4.0"
	text:    "New verify Rename pairs checked (VERIFY-005c): per side, `git diff --find-renames --name-status <base> <side>` lists rename pairs; edits the other side made to a renamed source must reach the live path, missing hunks ported. Merge-tree clean (VERIFY-006) now chains from renames_checked instead of deltas_attributed."
}, {
	date:    "2026-10-04"
	version: "3.3.2"
	text:    "Work-branch create and cleanup run smine-rules merge-branch instead of the bundled script; dropped the serena arg — Serena support removed repo-wide, goland stays the one IDE opt-in."
	tokens:  4105
}, {
	date:    "2026-10-02"
	version: "3.3.1"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  4221
}, {
	date:    "2026-09-29"
	version: "3.3.0"
	text:    "origin flag arg: only with it does the skill fetch origin and merge origin/<theirs>; default is local refs only. New rule Local refs default, steps Fetch origin and Check refs, stop Missing ref — no silent origin fallback, missing ref stops before scoping; Conflict scope chains from the ref check; base-ref checkpoint param and stale-ref stop re-worded for the arg; allowed-tools gains Bash(git fetch origin *)."
	tokens:  4222
}, {
	date:    "2026-09-28"
	version: "3.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "serena and goland args: opt the run into that IDE MCP server per the navigating IDE MCP opt-in doctrine; Args converted to the table form; acdsl-context injects ACTION-NAV-010 and ACTION-NAV-011."
}, {
	date:    "2026-09-25"
	version: "3.1.1"
	text:    "Prose: dropped the '(already allowlisted)' aside on the --no-edit fallback and stated on the merge step that the frontmatter grants every git verb the body issues — a session hand-built the merge commit via write-tree/commit-tree believing git merge was denied (ledger shows no denial). Disambiguation row for /overlap (renamed merge-risk). No grant change."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: `{section: phase|context}` clause on every topic section (Rules → context, 1-7 → phase); Applies bullets recast as param; pred/post clauses added to 26 of 33 entries — 7 rule invariants plus a causal chain from in-flight abort through scope, work-branch/nightly branching, the whole verify checkpoint, commit, and cleanup, gated on the shared `mode` atom for unattended (routine) runs; sibling atoms.json declares the 27 local atoms; three queries added — stale-ref-blocks-commit (stop preempts the commit step, unsat), nightly-skips-workbranch (route gated by mode, unsat), interactive-workbranch-witness (sat). No behavior change."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-08"
	version: "2.0.1"
	text:    "Commit step: retry once with git commit --no-edit when the default-MERGE_MSG commit fails on a session/pool worktree gitdir before treating the commit as failed (COMMIT-001a/001b)"
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Reformatted to skill format v2 — When-to-use recast as Routing (Purpose/Disambiguation table/Preconditions/Pipeline), argument-hint made a complete example, allowed-tools reordered; entry content and IDs unchanged"
}, {
	date:    "2026-09-05"
	version: "1.8"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.7"
	text:    "work branches carry the agent prefix — claude-merge/<theirs-slug>-<ours-slug> for create and cleanup (was merge/*); branch-prefix frontmatter declares the namespace; covered by cmd/tests/test_merge_branch.sh"
}, {
	date:    "2026-08-27"
	version: "1.6"
	text:    "allowed-tools manifest now covers the skill's own git mechanics (merge, merge-tree, cherry-pick/rebase aborts, commit --no-edit) so unattended --allowedTools runs can execute the merge; unattended invocations merge directly on ours (no work branch, no cleanup)"
}, {
	date:    "2026-08-23"
	version: "1.5"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.4"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-20"
	version: "1.3.1"
	text:    "step 5 hardened into a mandatory checkpoint that runs even on a clean auto-merge — asserts the merged base ref is the exact ref the user named (rejects stale origin/<x> for local <x>) and attributes every parent-diff delta before accepting (released as 1.4; the entry was lost when 1.4 was reused)"
}, {
	date:    "2026-08-18"
	version: "1.3"
	text:    "reference rename — fchange → fdesign change (fchange retired into the fdesign change route)"
}, {
	date:    "2026-08-11"
	version: "1.2"
	text:    "work-branch naming and cleanup via bundled scripts/merge_branch.sh (deterministic slug, safety-gated -d cleanup); allowed-tools manifest added"
}, {
	date:    "2026-07-27"
	version: "1.1"
	text:    "moved under skills/git/ group; name and behavior unchanged"
}, {
	date:    "2026-07-26"
	version: "1.0"
	text:    "Initial version — final-tree merge method distilled from the peek-mcp deep-analysis merge session."
}]
env: {
	CHECK: {
		interface: "check@1"
	}
	MERGE_BRANCH: {
		interface: "merge-branch@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " merge-resolve"
}, {
	key:   "description"
	value: " Merge two diverged git branches by resolving all conflicts once at final-tree level, verified by build, tests and parent diffs. Trigger on /merge-resolve or \"merge main into my branch\" or \"these branches conflict / cherry-picks keep failing\". Args — ours: branch to merge into (default current); theirs: branch to merge in; origin: fetch and merge origin/<theirs>, default local only."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.11.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " ACTION-NAV-010, ACTION-NAV-011, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"ours: my-feature theirs: main origin\""
}, {
	key:   "branch-prefix"
	value: " claude-merge/"
}, {
	key:   "allowed-tools"
	value: " Bash($SKILL_MERGERESOLVE_MERGE_BRANCH merge-branch *), Bash(git checkout --ours -- *), Bash(git checkout --theirs -- *), Bash(git cherry-pick --abort), Bash(git commit --no-edit), Bash(git fetch origin *), Bash(git merge *), Bash(git merge-tree *), Bash(git rebase --abort)"
}]
preamble: """
	# Merge Resolve

	Merge two diverged branches, resolve all conflicts once at final-tree level.

	Any conflicted integration ask — failed cherry-pick chain, mid-rebase mess, replay-cache branch — becomes this one flow, never
	resolved in place.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				note:       "output feeds a normal merge or PR into the target branch"
				standalone: true
			}
			preconditions: ["`ours` is a local branch; `theirs` a local branch, or on `origin` under `origin` arg", "working tree clean, or only abortable in-flight state", "project has a build/test command"]
			purpose: "merge two diverged branches when one must absorb the other, resolving every conflict once at final-tree level."
			routes: [{
				for: "a single conflicted file already staged"
				use: "just resolve it"
			}, {
				for: "file overlap with other sessions before implementing"
				use: "`/overlap`"
			}, {
				for: "planning a restructure"
				use: "`/fdesign`"
			}, {
				for: "auditing what a contract change broke"
				use: "`/spec-drift`"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "ours"
				default: "current branch"
				effect:  "branch absorbing the merge"
				form:    "branch"
			}, {
				name:    "theirs"
				default: "required"
				effect:  "branch merged in"
				form:    "branch"
			}, {
				name:    "origin"
				default: "absent"
				effect:  "fetch `origin`, merge `origin/<theirs>`; absent = local refs only, no fetch — [Fetch origin](#skill-mergeresolve-normalize-003)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-MERGERESOLVE-RULES-001"
			pred:      "not commits_replayed"
			role:      "rule"
			statement: "Resolution MUST NEVER replay commits — ground truth is each side's final tree, per [Final-tree ground truth](#skill-mergeresolve-resolve-001)."
			title:     "No replay resolution"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RULES-002"
			pred:      "judgment_call => judgment_named"
			role:      "rule"
			statement: "Every judgment call (deleted mechanism, behavior delta) MUST be named explicit in result — silent semantic choices forbidden."
			title:     "Judgment calls named"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RULES-003"
			pred:      "test_rewritten => coverage_preserved"
			role:      "rule"
			statement: "Test rewritten during resolution MUST still cover surviving behavior, never be deleted."
			title:     "Test coverage preserved"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RULES-005"
			pred:      "not origin_arg => not origin_ref_used"
			role:      "rule"
			statement: "Refs MUST resolve to local branches — no `git fetch`, no `origin/<x>` — unless `origin` arg set; `ours` always local."
			title:     "Local refs default"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RULES-004"
			pred:      "contradiction_unresolved"
			role:      "stop"
			statement: "If both sides' intent for same behavior genuinely contradicts, no commit/design evidence decides ⇒ STOP: ask user decide."
			title:     "Contradiction stop"
		}
	}]
	kind: "context"
}, {
	title: "1. Normalize to a merge (abort in-flight state first)"
	blocks: [{
		entry: {
			id:        "SKILL-MERGERESOLVE-NORMALIZE-002b"
			pred:      "not commit_by_commit"
			role:      "rule"
			statement: "Resolution MUST NOT continue commit-by-commit."
			title:     "Never commit-by-commit"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-NORMALIZE-001"
			post: ["inflight_aborted"]
			role:      "step"
			statement: "ABORT in-flight state with `git cherry-pick --abort` / `git rebase --abort` / `git merge --abort` as applicable; confirm `git status` clean."
			title:     "Abort in-flight"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-NORMALIZE-002a"
			post: ["replay_abandoned"]
			pred:      "replay_in_progress"
			role:      "step"
			statement: "If user mid-way through replay/cache branch, ABANDON it, state why: partial replays resolve against half-adapted context, accumulate drift."
			title:     "Abandon replay"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-NORMALIZE-003"
			post: ["origin_fetched"]
			pred:      "origin_arg and inflight_aborted"
			role:      "step"
			statement: "RUN `git fetch origin <theirs>`; every later `<theirs>` means `origin/<theirs>`."
			title:     "Fetch origin"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-NORMALIZE-004a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`ours`: `refs/heads/<ours>`; `theirs`: `refs/heads/<theirs>`, or `refs/remotes/origin/<theirs>` under `origin` arg."
			}]
			post: ["refs_checked"]
			pred:      "inflight_aborted and (origin_arg => origin_fetched)"
			role:      "step"
			statement: "RESOLVE both refs with `git rev-parse --verify <ref>`."
			title:     "Check refs"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-NORMALIZE-004b"
			pred:      "ref_missing"
			role:      "stop"
			statement: "If a ref fails to resolve ⇒ STOP: report missing ref, never substitute another ref."
			title:     "Missing ref"
		}
	}]
	kind: "phase"
}, {
	title: "2. Scope before touching anything"
	blocks: [{
		entry: {
			id: "SKILL-MERGERESOLVE-SCOPE-001"
			post: ["conflict_scope_known"]
			pred:      "refs_checked"
			role:      "step"
			statement: "RUN `git merge-tree --write-tree <ours> <theirs>` to list conflicted files, report count."
			title:     "Conflict scope"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-SCOPE-002"
			post: ["side_inventory_known"]
			pred:      "conflict_scope_known"
			role:      "step"
			statement: "ESTABLISH which side has what via `git log --oneline <ours>..<theirs>` and reverse; treat deliberate renames/exports noted in commit messages or `plans/` design docs as binding."
			title:     "Side inventory"
		}
	}]
	kind: "phase"
}, {
	title: "3. Merge on a work branch"
	blocks: [{
		entry: {
			id:        "SKILL-MERGERESOLVE-WORKBR-002"
			pred:      "mode = nightly => not work_branch_created"
			role:      "rule"
			statement: "Unattended invocations (routine runs) MUST merge directly on `ours`, no work branch, skip step 7 cleanup; HEAD MUST end on branch run started on."
			title:     "Unattended direct merge"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-WORKBR-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "deterministic work branch name `claude-merge/<theirs-slug>-<ours-slug>`."
			}]
			post: ["work_branch_created"]
			pred:      "mode != nightly"
			role:      "step"
			statement: "RUN `$SKILL_MERGERESOLVE_MERGE_BRANCH merge-branch create <ours> <theirs>` to create, check out work branch (or merge directly on `ours` if user says so)."
			title:     "Create work branch"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-WORKBR-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "every git verb this skill issues is granted by its frontmatter while it runs; never substitute `write-tree`/`commit-tree` for `git merge`."
			}]
			post: ["merge_started"]
			pred:      "work_branch_created or mode = nightly"
			role:      "step"
			statement: "RUN `git merge <theirs>` on the work branch created above."
			title:     "Merge theirs in"
		}
	}]
	kind: "phase"
}, {
	title: "4. Resolve per file — union of both intents"
	blocks: [{
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-001"
			pred:      "not intermediate_diff_used"
			role:      "rule"
			statement: "Ground truth for each side MUST be its final tree (`git show <ref>:<path>`), never intermediate diffs."
			title:     "Final-tree ground truth"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-002a"
			role:      "step"
			statement: "CLASSIFY each hunk as additive-union: keep both sides."
			title:     "Additive union"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-002b"
			role:      "step"
			statement: "CLASSIFY rename collision: keep side whose rename has external consumers or design-doc rationale."
			title:     "Rename collision"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-002c"
			role:      "step"
			statement: "CLASSIFY mechanism collision — two implementations same behavior — pick designed one, delete stopgap; commit messages saying \"so X passes\" mark stopgaps."
			title:     "Mechanism collision"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-002d"
			role:      "step"
			statement: "CLASSIFY contradictory tests: auto-merge can splice assertions from both sides into one impossible test — rewrite to surviving mechanism."
			title:     "Contradictory tests"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-002e"
			role:      "step"
			statement: "TAKE a file one side wins whole with `git checkout --ours|--theirs -- <path>`, then `git add <path>`."
			title:     "Whole-file take"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-002f"
			role:      "step"
			statement: "REGENERATE a generated file: take either side whole, rerun its make target, `git add <path>` — never hand-edit its conflict markers, never write it through a `git show <ref>:<path>` redirect."
			title:     "Generated-file regen"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-003a"
			role:      "step"
			statement: "SWEEP for what compiles but wrong: same-typed parameter reorders (alphabetized string params), resurrected dead code (fields/channels one side deleted)."
			title:     "Wrong-but-compiling sweep"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-RESOLVE-003b"
			role:      "step"
			statement: "SWEEP constructor arity changes in files only the other side added."
			title:     "Constructor arity sweep"
		}
	}]
	kind: "phase"
}, {
	title: "5. Verify — mandatory checkpoint (abort = unresolved finding, not a skipped check)"
	blocks: [{
		entry: {
			id:        "SKILL-MERGERESOLVE-VERIFY-001"
			pred:      "merge_started => not verification_skipped"
			role:      "rule"
			statement: "Verification MUST run in full even when merge auto-resolves zero conflicts — clean auto-merge against wrong base ref silently drops work; no step below skippable."
			title:     "Checkpoint always runs"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-002a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "bare `<ours>` means local branch always; `<theirs>` local unless `origin` arg, then `origin/<theirs>`."
			}]
			post: ["base_ref_verified"]
			pred:      "merge_started"
			role:      "verify"
			statement: "VERIFY merge base ref is ref user named; resolve ref actually merged, confirm match."
			title:     "Assert base ref"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-002c"
			post: ["sha_compared"]
			pred:      "base_ref_verified"
			role:      "step"
			statement: "COMPARE SHAs (`git rev-parse <named-ref>` against merged ref), produce ref-attribution finding."
			title:     "Compare SHAs"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-VERIFY-002d"
			pred:      "stale_ref_used"
			role:      "stop"
			statement: "If `origin/<x>` used in place of named local `<x>` without `origin` arg ⇒ STOP: abort, re-merge against named ref."
			title:     "Stale ref aborts"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-003"
			post: ["no_conflict_markers"]
			pred:      "sha_compared"
			role:      "verify"
			statement: "VERIFY no conflict markers remain: `grep -rl '<<<<<<<'` over tracked source."
			title:     "No conflict markers"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-004"
			post: ["suite_green"]
			pred:      "no_conflict_markers"
			role:      "verify"
			statement: "VERIFY `$SKILL_MERGERESOLVE_CHECK` green, one Bash call per line — the repo's build, vet, tests and formatter."
			title:     "Full green suite"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-005a"
			post: ["deltas_attributed"]
			pred:      "suite_green"
			role:      "verify"
			statement: "VERIFY every parent-diff delta attributed before accepting merge; `git diff <theirs> --stat` and `git diff <ours> --stat`, each delta traced to other parent or named resolution decision."
			title:     "Attribute deltas"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-005c"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`<base>` = `git merge-base <ours> <theirs>`; a split or moved file paired with an unrelated copy counts as a pair."
			}]
			post: ["renames_checked"]
			pred:      "deltas_attributed"
			role:      "verify"
			statement: "VERIFY per side `git diff --find-renames --name-status <base> <side>`: each R pair whose source other side edited has those edits on live path, not only moved copy; port missing hunks."
			title:     "Rename pairs checked"
		}
	}, {
		entry: {
			id:        "SKILL-MERGERESOLVE-VERIFY-005b"
			pred:      "delta_unattributable"
			role:      "stop"
			statement: "If delta unattributable ⇒ STOP: treat as dropped or spurious work, not pass."
			title:     "Unattributable fails"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-VERIFY-006"
			post: ["mergetree_clean"]
			pred:      "renames_checked"
			role:      "verify"
			statement: "VERIFY `git merge-tree --write-tree <final-target> <work-branch>` clean before treating real merge as ready."
			title:     "Merge-tree clean"
		}
	}]
	kind: "phase"
}, {
	title: "6. Commit and report"
	blocks: [{
		entry: {
			id: "SKILL-MERGERESOLVE-COMMIT-001a"
			post: ["commit_made"]
			pred:      "mergetree_clean"
			role:      "step"
			statement: "COMMIT with `git commit --no-edit`, the default `MERGE_MSG` kept."
			title:     "Commit default message"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-COMMIT-002"
			post: ["report_written"]
			pred:      "commit_made"
			role:      "step"
			statement: "REPORT files resolved, each semantic judgment call made (especially deleted mechanisms, behavior deltas), verification results."
			title:     "Report contents"
		}
	}]
	kind: "phase"
}, {
	title: "7. Cleanup"
	blocks: [{
		entry: {
			id: "SKILL-MERGERESOLVE-CLEANUP-001a"
			post: ["landed_on_ours"]
			pred:      "report_written"
			role:      "step"
			statement: "CONFIRM merge result landed on `<ours>` (fast-forward or real merge), check with user when landing theirs to do."
			title:     "Landed on ours"
		}
	}, {
		entry: {
			id: "SKILL-MERGERESOLVE-CLEANUP-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "deletes only merged `claude-merge/*` branches (`git branch -d`, never `-D`)."
			}, {
				label:  "param"
				marker: "*"
				text:   "anything unmerged kept, reported, never forced."
			}]
			pred:      "landed_on_ours and mode != nightly"
			role:      "step"
			statement: "RUN `$SKILL_MERGERESOLVE_MERGE_BRANCH merge-branch cleanup <ours>` to delete merged `claude-merge/*` branches."
			title:     "Run branch cleanup"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "semantic conflict classification, stopgap-vs-design judgment; mechanical phases cheap but misjudged collisions ship silent regressions"
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "needs two diverged local branches, which a fixture tree or patch cannot create"
}
