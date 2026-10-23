changelog: [{
	date:    "2026-10-14"
	version: "5.3.0"
	text:    "Interfaces: one variable per capability — AGENTWT_PROBE"
	tokens:  2098
}, {
	date:    "2026-10-13"
	version: "5.2.0"
	text:    "The pick check's worktree tool is the env variable $SKILL_SMINECONSOLIDATE_AGENTWT (required) in the probe definition, its grant and the precondition."
	tokens:  2088
}, {
	date:    "2026-10-10"
	version: "5.1.0"
	text:    "Pick check is one `smine-agentwt probe` call over every change commit; `pick --dry-run` is gone."
	tokens:  2072
}, {
	date:    "2026-10-09"
	version: "5.0.0"
	text:    "Carry route removed: a change lives on the night that made it and ages out by the runner's window; run is the only route."
	tokens:  2069
}, {
	date:    "2026-10-08"
	version: "4.5.0"
	text:    "Carry splits an older change over the proposal cap (7 files / 420 lines) into capped slices with their own task records; one with no seam fails and the runner parks it in the backlog"
	tokens:  3909
}, {
	date:    "2026-10-08"
	version: "4.4.0"
	text:    "Carry skips a cherry-pick copy of a rejected change, through every copy of a copy"
	tokens:  3089
}, {
	date:    "2026-10-08"
	version: "4.3.0"
	text:    "Carry skips changes a newer night already carries and drops paths main no longer tracks"
	tokens:  3003
}, {
	date:    "2026-10-06"
	version: "4.2.0"
	text:    "Carry route: cherry-picks every unsettled change of older proposals branches onto this night with git cherry-pick -x, resolves conflicts in place, reports carried rows; the verb rule binds the run route only."
	tokens:  2878
}, {
	date:    "2026-10-04"
	version: "4.1.0"
	text:    "Requires mining: deploy skips the skill without them."
	tokens:  2023
}, {
	date:    "2026-10-04"
	version: "4.0.0"
	text:    "Store role retired — mutable set, dedup, re-home, language sweep, votes and store-check gone. New single job, route `run`: make every change commit of the run pick cleanly onto main on its own, merging file-sharing changes into one commit with reset --soft and fresh commits, two rounds at most; caveman arg dropped."
	tokens:  2023
}, {
	date:    "2026-10-02"
	version: "3.3.1"
	text:    "Store paths hang off `<store>`, the path `make rules ARGS=\"store-root\"` prints — a smoke or eval run works on its own store, never the live one."
	tokens:  3581
}, {
	date:    "2026-10-02"
	version: "3.3.0"
	text:    "Store is `~/GolandProjects/agent-plans/<repo-name>/proposals/`; the validate pass runs `make rules ARGS=\"store-check\"` — the gate no longer sees the store; Read and Edit grants there; the mutable set reads the run's vote copy and the live vote file in the state dir."
	tokens:  3565
}, {
	date:    "2026-10-01"
	version: "3.2.4"
	text:    "Pipeline declares from /smine — the dispatcher that invokes this stage."
	tokens:  3429
}, {
	date:    "2026-09-28"
	version: "3.2.3"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  3425
}, {
	date:    "2026-09-26"
	version: "3.2.2"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "3.2.1"
	text:    "Make-only entry points: allowed-tools and prose call make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-24"
	version: "3.2.0"
	text:    "Problems kind: never re-homed, deduped by problem meaning with evidence, relevance and artifact plan unioned."
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind clauses (Hard invariants = context, numbered pipeline sections = phase), pred/post clauses on 15 entries, sibling atoms.json, two queries; freeform bullets normalized to Applies/param registers."
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
	text:    "Format v2 (major): Routing section (Purpose/Disambiguation/Preconditions/Pipeline) replaces When to use, Args table, full example argument-hint, title link for the language-sweep cross-reference — content identical"
}, {
	date:    "2026-09-05"
	version: "1.7"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-05"
	version: "1.6"
	text:    "acdsl kind routing — re-home test recognizes gate-bearing proposals as kind acdsl (proposals/acdsl.json)"
}, {
	date:    "2026-09-01"
	version: "1.5"
	text:    "vote-identity invariant — when proposals/votes-pending.jsonl is present the (kind,id) of every listed proposal is immutable this run (never re-homed/renumbered/dropped) and a near-duplicate merges INTO the voted proposal so a cast vote never goes stale; the presentation pass is renamed the style-application gate and now conforms all mutable user-visible prose to the style profile (style-profile.md, shared via --add-dir) every run, not only reworded entries"
}, {
	date:    "2026-08-27"
	version: "1.4"
	text:    "casual jargon sweep in the presentation pass — on audience: casual, mutable entries whose user-visible prose carries file paths, rule/FACT/ACDSL IDs, or schema/taxonomy jargon are reworded with the technical content moved to target/anchor/code/snippet fields"
}, {
	date:    "2026-08-27"
	version: "1.3"
	text:    "language arg widened to a full-store sweep — all mutable user-visible prose translated, fixed structural group titles verbatim"
}, {
	date:    "2026-08-23"
	version: "1.2"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "1.1"
	text:    "acdsl-context: key rename — the ACDSL declaration key moved off Claude Code's native context: frontmatter field; prose references updated"
}, {
	date:    "2026-08-13"
	version: "1.0"
	text:    "initial version — proposals mode (dedup, re-home, presentation, schema/audit gate); skills/context modes reserved"
}]
env: {
	AGENTWT_PROBE: {
		interface: "agentwt-probe@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " smine-consolidate"
}, {
	key:   "description"
	value: " Make every change commit of a smine run pick cleanly onto main on its own. Trigger on /smine-consolidate run, normally as the /smine consolidate stage. Args — run: rebuild the run's change commits until each picks alone."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 5.3.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"run\""
}, {
	key:   "allowed-tools"
	value: " Read, Bash($SKILL_SMINECONSOLIDATE_AGENTWT_PROBE probe *), Bash(git add *), Bash(git commit -q -m *), Bash(git log *), Bash(git reset --soft *), Bash(git restore --staged *), Bash(git rev-parse *), Bash(git show *)"
}]
preamble: """
	# smine: consolidate change commits

	Make every change commit of the run pick cleanly onto `main` on its own, so the routine runner can land each change
	independently.
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
					skill: "smine-problems"
				}, {
					kind:  "to"
					skill: "smine-orchestrate"
					route: "run"
				}]
				note: "runs after the /smine implement stage committed each change task; the Proposals page shows a commit whose probe onto main conflicts in its night's conflicts block, with the agentwt reason"
			}
			preconditions: ["cwd = the run worktree, on a branch started at `main`", "change commits in `main..HEAD`, ledger under `sessions/` uncommitted", "`$SKILL_SMINECONSOLIDATE_AGENTWT_PROBE probe` answers"]
			purpose: "rebuilds the run's change commits until each one picks onto `main` alone."
			routes: [{
				for: "producing change tasks"
				use: "the mining skills, /smine-problems"
			}, {
				for: "judging the change commits"
				use: "/smine-orchestrate run"
			}, {
				for: "merging two diverged branches"
				use: "/merge-resolve"
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
				effect:  "rebuild the run's change commits until each picks alone — [List change commits](#skill-smineconsolidate-check-001)"
				form:    "positional"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-DEF-001"
			role:      "def"
			statement: "Change commit = a commit in `main..HEAD` whose message carries a `Smine-Change: <kind>/<slug>` trailer, next to `Smine-Sessions: <id>, <id>`."
			title:     "Change commit"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-DEF-002"
			role:      "def"
			statement: "Probe `$SKILL_SMINECONSOLIDATE_AGENTWT_PROBE probe --onto main --format json <sha>…` → `{schema, onto, onto_sha, commits: [{commit, outcome, reason?}]}`, exit 0; outcome applies|conflict|skipped."
			title:     "Pick check"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-DEF-003"
			role:      "def"
			statement: "Rebuild range = from the oldest change commit sharing a file with the oldest failing commit F (F itself when none) up to HEAD."
			title:     "Rebuild range"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-DEF-004"
			role:      "def"
			statement: "Merge group = change commits of the rebuild range linked by shared files; a group of one keeps its commit and original message."
			title:     "Merge group"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-INVAR-001"
			role:      "rule"
			statement: "The run route MUST NOT use `git commit --amend`, rebase or cherry-pick — only `git reset --soft`, `git restore --staged`, `git add` and `git commit -q -m`."
			title:     "Allowlisted verbs"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-INVAR-002"
			role:      "rule"
			statement: "Consolidate MUST NOT stage or change uncommitted files — the `sessions/` ledger stays uncommitted for the runner."
			title:     "Uncommitted untouched"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-INVAR-003"
			role:      "rule"
			statement: "Rebuilt commits MUST carry every `Smine-Change:` and `Smine-Sessions:` trailer and every body paragraph of the commits they replace."
			title:     "Trailers preserved"
		}
	}]
	kind: "context"
}, {
	title: "1. Check"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-CHECK-001"
			role:      "step"
			statement: "LIST the change commits oldest first: `git log --reverse --format=%H --grep=^Smine-Change: main..HEAD`."
			title:     "List change commits"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-CHECK-002"
			role:      "step"
			statement: "RUN one pick check over every change commit; RECORD each outcome."
			title:     "Pick every commit"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-CHECK-003"
			role:      "stop"
			statement: "If every change commit applies, or none exists ⇒ STOP: report the stage `ok` with the commit count."
			title:     "All apply"
		}
	}]
	kind: "phase"
}, {
	title: "2. Rebuild"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-REBUILD-001"
			role:      "step"
			statement: "RECORD for every commit of the rebuild range its files (`git show --name-only --format= <sha>`) and full message (`git log -1 --format=%B <sha>`); FORM the merge groups."
			title:     "Record range"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-REBUILD-002"
			role:      "step"
			statement: "RESET with `git reset --soft <range start>^`, then `git restore --staged .`."
			title:     "Soft reset"
		}
	}, {
		entry: {
			id: "SKILL-SMINECONSOLIDATE-REBUILD-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "merged group: one subject per [Commit subjects]($AGENT_CONTEXT_DIR_DEFAULT/rules/commits.md), every body in change order, all trailer lines in one last `-m`."
			}]
			role:      "step"
			statement: "For each merge group in range order: `git add <group paths>`, `git commit -q -m <subject> -m <bodies> -m <trailers>`; CONFIRM it landed before the next; task files stay as they are."
			title:     "Re-commit groups"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-REBUILD-004"
			role:      "verify"
			statement: "After re-commit: VERIFY `git status --porcelain` shows only the changes uncommitted before the reset; re-add a missed group path."
			title:     "Nothing dropped"
		}
	}, {
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-REBUILD-005"
			role:      "step"
			statement: "RE-RUN the pick check over every change commit; REPEAT this section while a commit fails, at most two rounds."
			title:     "Re-check"
		}
	}]
	kind: "phase"
}, {
	title: "3. Report"
	blocks: [{
		entry: {
			id:        "SKILL-SMINECONSOLIDATE-REPORT-001"
			role:      "step"
			statement: "REPORT the stage summary: commits checked, groups merged with their ids, every commit still failing with its pick reason — the page shows those as conflicts."
			title:     "Stage summary"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "mechanical commit regrouping by file overlap against a deterministic pick check"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "run"
}
