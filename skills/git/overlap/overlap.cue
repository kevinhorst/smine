changelog: [{
	date:    "2026-10-09"
	version: "4.2.1"
	text:    "A candidate whose git_branch is a detached `HEAD` is skipped: it resolves to the caller's own HEAD and would report the caller's edits as the candidate's."
}, {
	date:    "2026-10-09"
	version: "4.2.0"
	text:    "Candidate file sets come from `git diff --name-only <diff_target>...<branch>` instead of session_get diffs (session_get grant dropped); a verdict over fewer than all in-window candidates reads `partial — K of N candidates checked`."
}, {
	date:    "2026-10-03"
	version: "4.1.0"
	text:    "Requires peek: deploy skips the skill without them."
	tokens:  1673
}, {
	date:    "2026-10-02"
	version: "4.0.3"
	text:    "fdesign routes and plan dir layout follow the plan lifecycle change."
	tokens:  1673
}, {
	date:    "2026-09-29"
	version: "4.0.2"
	text:    "Plan-file examples and lookup use design/<slug>.md (fdesign 5.8.0)."
	tokens:  1671
}, {
	date:    "2026-09-28"
	version: "4.0.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
}, {
	date:    "2026-09-25"
	version: "4.0.0"
	text:    "Renamed merge-risk → overlap (leaf dir, frontmatter, H1, entry ids SKILL-OVERLAP-*); too close to merge-resolve in invocation while sharing nothing but the word. Peek contract repaired: allowed-tools now mcp__peek-mcp__session_list and session_get (session_diff, session_plan, session_uncommitted_diff no longer exist, server renamed) — the skill was categorically denied under the closed world. Committed/uncommitted signals collapse into the session_get diff; planned signal derived from plan-dir files in that diff."
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: Rules and Method sections declare {section: context|phase}; pred/post clauses added on the read-only rule, both quality entries, and 7 Method transitions (own-file-set through report-verdict, incl. both stop guards); sibling atoms.json declares 14 local bool atoms; three queries added under queries/ (empty file set blocks candidate listing, peek unavailable blocks the verdict, the normal path reaches it); caveman compression pass over remaining prose."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "format-v2 reformat — Routing section (Purpose/Disambiguation/Preconditions/Pipeline), Args table, normalized argument-hint and allowed-tools; content identical"
}, {
	date:    "2026-09-05"
	version: "1.4"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-08-23"
	version: "1.3"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.2"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-18"
	version: "1.1"
	text:    "reference rename — fchange → fdesign change (fchange retired into the fdesign change route)"
}, {
	date:    "2026-08-11"
	version: "1.0"
	text:    "initial version — peek-mcp based cross-session file-overlap report before implementation"
}]
frontmatter: [{
	key:   "name"
	value: " overlap"
}, {
	key:   "description"
	value: " Report file overlap between the current plan and other recent sessions on the same repo — a pre-implementation risk report, never a fix. Trigger on /overlap [plan file] or \"will this collide with other sessions\". Args — plan file: plan whose Changes paths seed the check (default: this session's plan or uncommitted diff); window: recency cutoff in days (default 7); codex: also scan Codex sessions."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 4.2.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"plans/2026-09-07-foo-abc1234/2026-09-07-foo-wip-abc1234/design/foo.md window: 14 codex\""
}, {
	key:   "allowed-tools"
	value: " ToolSearch, Bash(git diff *), Bash(git diff --name-only *), mcp__peek-mcp__session_list"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}]
preamble: """
	# Overlap

	Reports file overlap between current plan and other recent sessions on same repo, before implement.

	Never merges, never fixes, never blocks — user decides what to do with overlap.
	"""
requires: ["peek"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "fdesign"
				}, {
					kind:  "to"
					skill: "fimplement"
				}]
				note:       "optional pre-implementation check"
				standalone: true
			}
			preconditions: ["peek-mcp available", "a file set to check (an approved plan's Changes paths, or the current worktree's diff)"]
			purpose: "before implementing, check whether any other recent session on same repo touches (or plans touch) same files; output is risk report."
			routes: [{
				for: "branches already conflict"
				use: "/merge-resolve"
			}, {
				for: "reviewing what another session did"
				use: "/peek"
			}, {
				for: "mining sessions"
				use: "/smine"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "plan file"
				default: "session plan, else worktree diff"
				effect:  "plan whose Changes `location:` paths seed the check; absent → this session's plan, else `git diff --name-only` vs base — [Own file set](#skill-overlap-method-001a)"
				form:    "free text (path), first"
			}, {
				name:    "window"
				default: "`7`"
				effect:  "recency cutoff in days for candidate sessions — [Candidate session drops](#skill-overlap-method-002b)"
				form:    "`<days>`"
			}, {
				name:    "codex"
				default: "Claude only"
				effect:  "also scan Codex sessions — [Candidate session list](#skill-overlap-method-002a)"
				form:    "flag"
			}]
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-OVERLAP-RULES-001"
			pred:      "not git_mutated and not messages_sent and not fixes_applied"
			role:      "rule"
			statement: "Overlap MUST stay read-only: no git mutations, no messages to other sessions, no fixes."
			title:     "Read-only"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-RULES-002"
			custom: ["todo:unmeasured"]
			pred:      "unattributable_exists => unattributable_listed"
			role:      "quality"
			statement: "Reports MUST NOT assert absence stronger than the data: sessions without `meta.cwd` are listed as \"unattributable\" when any exist, not silently dropped."
			title:     "No overclaiming absence"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-RULES-003"
			custom: ["todo:unmeasured"]
			pred:      "path_match_exact"
			role:      "quality"
			statement: "Path normalization MUST be exact-match on the repo root, never substring matching (nested checkouts would false-positive)."
			title:     "Exact-match normalization"
		}
	}]
	kind: "context"
}, {
	title: "Method"
	blocks: [{
		entry: {
			id: "SKILL-OVERLAP-METHOD-001a"
			post: ["own_files_collected"]
			role:      "step"
			statement: "COLLECT repo-relative paths from the plan's Changes `location:` lines (or the diff fallback)."
			title:     "Own file set"
		}
	}, {
		entry: {
			id:        "SKILL-OVERLAP-METHOD-001b"
			pred:      "file_set_empty"
			role:      "stop"
			statement: "If the file set is empty ⇒ STOP: report \"nothing to check\"."
			title:     "Empty set halts"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-METHOD-002a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "normalize by stripping trailing `/.claude/worktrees/<name>` from `meta.cwd`; apply same normalization to own cwd before comparing."
			}]
			post: ["candidates_listed"]
			pred:      "own_files_collected and not file_set_empty and not peek_unavailable"
			role:      "step"
			statement: "CALL `session_list` (agent `claude`; plus `codex` with the arg); keep items whose `meta.cwd` matches this repo's root."
			title:     "Candidate session list"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-METHOD-002b"
			post: ["candidates_filtered"]
			pred:      "candidates_listed"
			role:      "step"
			statement: "DROP items without `meta`; drop our own session (cwd equals this worktree path); drop `last_active` older than the window."
			title:     "Candidate session drops"
		}
	}, {
		entry: {
			id:        "SKILL-OVERLAP-METHOD-002c"
			pred:      "peek_unavailable"
			role:      "stop"
			statement: "If `session_list` does not answer (peek-mcp unavailable) ⇒ STOP: report it as the blocker — no other data source exists for session overlap."
			title:     "Peek unavailable"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-METHOD-003a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "three-dot form diffs from the merge base: commits shared with the base drop out; the name list is the authoritative `diff` signal."
			}, {
				label:  "Applies"
				marker: "*"
				text:   "a touched `plans/*/*/design/*.md` in the list marks the `location:` paths of `git diff <diff_target>...<branch> -- <path>` *planned* (weak signal)."
			}]
			post: ["their_files_collected"]
			pred:      "candidates_filtered"
			role:      "step"
			statement: "PER candidate, RUN `git diff --name-only <diff_target>...<meta.git_branch>` (both from `session_list`); the name list is its file set."
			title:     "Their file sets"
		}
	}, {
		entry: {
			id:        "SKILL-OVERLAP-METHOD-003b"
			role:      "verify"
			statement: "VERIFY the name list is non-empty and the branch resolves to a branch name, never a detached `HEAD`; else skip the candidate, counted as checked."
			title:     "Empty list skip"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-METHOD-004a"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "one line under the table notes uncommitted work in other worktrees is not scanned."
			}]
			post: ["report_table_emitted"]
			pred:      "their_files_collected"
			role:      "step"
			statement: "INTERSECT per candidate and emit one table: Session (title or id) | Branch (`meta.git_branch`) | Last active | Overlapping files | Signal (`diff` / `planned`)."
			title:     "Report table"
		}
	}, {
		entry: {
			id: "SKILL-OVERLAP-METHOD-004b"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "checked count below the in-window candidate count: verdict reads `partial — K of N candidates checked`, never `clear`."
			}]
			post: ["verdict_emitted"]
			pred:      "report_table_emitted"
			role:      "step"
			statement: "CLOSE with one-line verdict: `clear — no overlap` or `overlap — N sessions touch M shared files`. Flag *planned*-only overlaps as tentative."
			title:     "Report verdict"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "mechanical list-filter-intersect over structured tool output"
			tier:   "small"
		}
	}]
}]
smoke: {
	expect: {}
	fixture: {
		tree: "tree"
	}
	invoke: "plans/2026-10-02-smoke-overlap-0000000/2026-10-02-smoke-overlap-wip-0000000/design/smoke-overlap.md window: 1"
}
