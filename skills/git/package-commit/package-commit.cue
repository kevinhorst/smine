changelog: [{
	date:    "2026-10-21"
	version: "6.6.0"
	text:    "Dropped the edge from fimplement: it hands its commit to the skill its commit-skill variable names, this skill by example, so the pipeline no longer binds the two."
	tokens:  2431
}, {
	date:    "2026-10-17"
	version: "6.5.1"
	text:    "Reworded the re-strip rule: the Read hook no longer writes projection blocks; the strip guards a tree projected with acdsl project -file."
	tokens:  2437
}, {
	date:    "2026-10-14"
	version: "6.5.0"
	text:    "Interfaces: one variable per capability — PROJECT_STRIP"
	tokens:  2426
}, {
	date:    "2026-10-13"
	version: "6.4.0"
	text:    "The projection strip runs the optional env variable $SKILL_PACKAGECOMMIT_STRIP (an acdsl program), granted; unset drops the strip step. The precondition names the caller's check command instead of make audit."
	tokens:  2420
}, {
	date:    "2026-10-12"
	version: "6.3.0"
	text:    "Railroad-review's fixer commits through this skill: edge from railroad-review; the subject rule says past tense and dot-notation, as the commit-subject guide does."
	tokens:  2436
}, {
	date:    "2026-10-09"
	version: "6.2.1"
	text:    "STRIP-003 names the real re-injectors: the acdsl Read hook on every Read of a governed file and test_acdsl.sh; make audit and acdsl check only strip."
	tokens:  2364
}, {
	date:    "2026-10-09"
	version: "6.2.0"
	text:    "Identify lists untracked files: IDENT-005 runs git ls-files --others --exclude-standard, IDENT-003 unions all three lists so new files are grouped and committed like modified ones; allowed-tools adds git ls-files."
}, {
	date:    "2026-10-06"
	version: "6.1.0"
	text:    "Every group starts from an empty index: IDENT-004 unstages the pre-staged paths (deletions join their package, renames the destination's), COMMIT-004 verifies the index equals the group before commit; allowed-tools adds git restore --staged."
}, {
	date:    "2026-10-01"
	version: "6.0.3"
	text:    "Pipeline declares from /plan-archive; the quiet commit form is a global rule now, the manifest keeps naming it."
	tokens:  2149
}, {
	date:    "2026-09-28"
	version: "6.0.2"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2142
}, {
	date:    "2026-09-28"
	version: "6.0.1"
	text:    "Removed the bundled foreign-toolchain-pretag workflow: no step ever invoked it and this skill never tags; release compiles stay with make installer-check before make git-release."
}, {
	date:    "2026-09-26"
	version: "6.0.0"
	text:    "Commits only (backlog B-004): the Validate section, the trust arg, the build/test precondition and the go build/go test grants are gone — callers validate before invoking; Strip is the one route, between start..IDENT-001."
}, {
	date:    "2026-09-26"
	version: "5.2.1"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "5.2.0"
	text:    "Commit step: COMMIT-003 prescribes the granted forms git commit -m \"<message>\" or git commit -q -m \"<message>\" as their own Bash call from the cwd (body as a second -m; no -a/-F/-C or other flags before -m, no chains or heredoc messages) and strictly sequential groups — agents had been inventing commit forms the tool-policy gate denies."
}, {
	date:    "2026-09-25"
	version: "5.1.2"
	text:    "allowed-tools admits quiet commits: Bash(git commit -q -m *)."
}, {
	date:    "2026-09-25"
	version: "5.1.1"
	text:    "Make-only entry points: allowed-tools and the strip step call make acdsl instead of go run ./cmd/acdsl."
}, {
	date:    "2026-09-14"
	version: "5.1.0"
	text:    "Formal schema pass: section kinds declared (Validate and Strip route-gated on must_validate/acdsl_repo, Rules context, phases default); 14 entries carry pred/post clauses (8 invariants, 6 guarded transitions incl. the red-build stop and the two staging/strip posts); floating Examples/Note prose folded into their owning entries (COMMIT-002, STRIP-002); sibling atoms.json declares 17 local bool atoms (must_validate replaces a plain trust check so the red-build reachability query stays monotonic-safe); three queries added under queries/ — red-blocks-commit (unsat: a red build with validation required blocks reaching Stage group), trust-bypasses-red (sat: trust skips the block), strip-route-acdsl-only (sat: the strip route fires under its acdsl_repo pred)."
}, {
	date:    "2026-09-14"
	version: "5.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-13"
	version: "4.0.1"
	text:    "Register sweep — the re-strip Why bullet split to one claim per bullet"
}, {
	date:    "2026-09-07"
	version: "4.0.0"
	text:    "Format v2 reformat: Routing section (Purpose/Disambiguation/Preconditions/Pipeline), full example argument-hint, normalized allowed-tools, caveman pass"
}, {
	date:    "2026-09-05"
	version: "3.4.1"
	text:    "Entry headlines: mandatory 3-word titles, 300-rune statement cap (compressed/split)"
}, {
	date:    "2026-09-05"
	version: "3.4"
	text:    "step 1b STRIP-003 — the ACDSL strip is not one-shot: make audit / acdsl check re-inject projection blocks, so re-strip immediately before each git add (after any audit/build) so a re-injected block is never committed"
}, {
	date:    "2026-08-22"
	version: "3.3"
	text:    "bundled workflow foreign-toolchain-pretag.js — for every release artifact built by a toolchain the repo does not run natively (Inno Setup .iss, GOOS cross-builds), compiles it locally (dockerized where needed), asserts exit 0, and blocks the tag on any failure so a CI run on a pushed tag is never the first compile"
}, {
	date:    "2026-08-23"
	version: "3.2"
	text:    "added argument-hint frontmatter"
}, {
	date:    "2026-08-23"
	version: "3.1"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-19"
	version: "3.0"
	text:    "major: body migrated to entries via /fmt skill — 21 entries (VALIDATE 2, STRIP 2, IDENT 3, GROUP 4, COMMIT 2, RESULT 1, RULES 7), content unchanged; context: RULE-COMMIT-* declaration added (commit-message doctrine injected at invocation)"
}, {
	date:    "2026-08-13"
	version: "2.1"
	text:    "explicit Rules from mined sessions — never amend unless asked, honor inline gate waivers, and message hygiene extended (\"no desc\"/\"no dsc\" = subject only; no Claude attribution of any kind, restating the existing no-rule-IDs / no-co-author trailer)"
}, {
	date:    "2026-08-09"
	version: "2.0"
	text:    "degoified — directory-based grouping, stack-resolved validation (Makefile targets first), generated-file skip per repo convention; make targets added to allowed-tools"
}, {
	date:    "2026-08-06"
	version: "1.9"
	text:    "step 1b — strip ACDSL projection blocks before staging in registry-bearing repos; acdsl added to allowed-tools"
}, {
	date:    "2026-07-31"
	version: "1.8"
	text:    "activity-scoped context — commit style guide moved to the pack (style/commits.md)"
}, {
	date:    "2026-07-30"
	version: "1.7"
	text:    "context redesign — commit style guide moved to this skill's assets/commits.md"
}, {
	date:    "2026-07-30"
	version: "1.6"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-30"
	version: "1.5"
	text:    "renamed per-package-commit → package-commit; behavior unchanged"
}, {
	date:    "2026-07-27"
	version: "1.4"
	text:    "moved under skills/git/ group; name and behavior unchanged"
}, {
	date:    "2026-07-24"
	version: "1.3"
	text:    "delegation removed — not delegatable (tested: delegated run far slower than inline); step 0 and inline arg deleted"
}, {
	date:    "2026-07-22"
	version: "1.2"
	text:    "delegation intake (self-delegating to sonnet), inline arg, classification unattended-safe, effort low"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-06-22"
	version: "1.0"
	text:    "initial version"
}]
env: {
	PROJECT_STRIP: {
		interface: "project-strip@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " package-commit"
}, {
	key:   "description"
	value: " Commit changed files grouped by package — commits only, callers validate first. Trigger on /package-commit or \"commit per package\" or when multiple packages need separate commits. Args — file: one commit per changed file."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 6.6.0"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "argument-hint"
	value: " \"file\""
}, {
	key:   "allowed-tools"
	value: " Read, Bash($SKILL_PACKAGECOMMIT_PROJECT_STRIP project -strip), Bash(git add *), Bash(git commit -m *), Bash(git commit -q -m *), Bash(git diff *), Bash(git log *), Bash(git ls-files *), Bash(git restore --staged *)"
}, {
	key:   "acdsl-context"
	value: " RULE-COMMIT-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}]
preamble: """
	# Package Commit

	Commit changed files grouped by package (directory).
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "coverage-increase"
				}, {
					kind:  "from"
					skill: "plan-archive"
				}, {
					kind:  "from"
					skill: "railroad-review"
				}, {
					kind:     "to"
					skill:    "plan-archive"
					optional: true
				}]
				note: "terminal step after implementation"
			}
			preconditions: ["callers validate before invoking — their check command is green; this skill never builds or tests", "changes present, staged or unstaged"]
			purpose: "commits local changes as separate per-package commits — committing only; validation is the caller's step."
			routes: [{
				for: "a single coherent change to one package"
				use: "plain `git commit`"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "file"
				effect: "each changed file its own commit. Grouping degenerates to one file per group; message prefix stays the file's dot-notation directory path. Ordering and rules unchanged."
			}]
		}
	}]
}, {
	title:   "1. Strip ACDSL projections (repos with acdsl/registry.json only)"
	between: "start..IDENT-001"
	blocks: [{
		entry: {
			id: "SKILL-PACKAGECOMMIT-STRIP-003"
			items: [{
				label:  "Observed"
				marker: "*"
				text:   "nightly runs committed projection blocks to HEAD while the Read hook wrote them; an `acdsl project -file` tree keeps its blocks until stripped."
			}]
			pred:      "acdsl_repo => stripped_before_add"
			role:      "rule"
			statement: "Worker MUST strip immediately before each `git add` — the Read hook writes nothing, but a tree projected with `acdsl project -file` keeps its blocks until stripped."
			title:     "Re-strip before add"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-STRIP-001"
			post: ["stripped"]
			role:      "step"
			statement: "RUN `$SKILL_PACKAGECOMMIT_PROJECT_STRIP project -strip` to remove working-copy projection blocks before staging anything."
			title:     "Projection strip"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-STRIP-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "stripping may empty a file's diff (the only change was the projection); such files drop out of step 2 naturally."
			}]
			role:      "step"
			statement: "SKIP silently when the repo has no `acdsl/registry.json`."
			title:     "No-registry skip"
		}
	}]
	kind: "route"
	pred: "acdsl_repo"
}, {
	title: "2. Identify changed files"
	blocks: [{
		entry: {
			id:        "SKILL-PACKAGECOMMIT-IDENT-001"
			role:      "step"
			statement: "RUN `git diff --name-only`."
			title:     "Unstaged diff"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-IDENT-002"
			role:      "step"
			statement: "RUN `git diff --cached --name-only`."
			title:     "Staged diff"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-IDENT-005"
			role:      "step"
			statement: "RUN `git ls-files --others --exclude-standard`."
			title:     "Untracked files"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-IDENT-003"
			post: ["changes_present"]
			role:      "step"
			statement: "UNION all three lists."
			title:     "Union lists"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-IDENT-004"
			items: [{
				label:  "param"
				marker: "*"
				text:   "pre-staged deletion joins its package's group (`git add <path>` restages it); pre-staged rename: old and new path join the destination's group."
			}]
			role:      "step"
			statement: "UNSTAGE every path of the staged list: `git restore --staged <paths>` — each group restages its own files."
			title:     "Empty index"
		}
	}]
}, {
	title: "3. Group"
	blocks: [{
		entry: {
			id:        "SKILL-PACKAGECOMMIT-GROUP-001"
			role:      "rule"
			statement: "Source files MUST be grouped by directory, dot-notation. `util/reporting/interactive/queries` → `util.reporting.interactive.queries`"
			title:     "Dot-notation groups"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-GROUP-002"
			role:      "rule"
			statement: "Groups MUST commit in dependency order where the stack defines one."
			title:     "Dependency order"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-GROUP-003"
			role:      "rule"
			statement: "Non-source files MUST be grouped by logical owner."
			title:     "Logical owner"
		}
	}, {
		prose: """
			  - `sql/reporting/*` → `util.reporting`
			  - `docs/runbooks/*` → `docs.runbooks`
			  - root Makefile → `build`
			"""
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-GROUP-004"
			pred:      "not generated_included"
			role:      "rule"
			statement: "Generated files MUST be skipped (per repo convention, e.g. `*.gen.go`)."
			title:     "Generated skipped"
		}
	}]
}, {
	title: "4. Commit (per group, logical order)"
	blocks: [{
		entry: {
			id: "SKILL-PACKAGECOMMIT-COMMIT-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "e.g. `docs.runbooks: Added deployment.md`."
			}]
			pred:      "message_format_valid"
			role:      "rule"
			statement: "Commit message MUST use format `<package.path>: <Description>` — dot-notation path, colon, capitalized past tense, concise, no trailing period, no rule IDs."
			title:     "Message format"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-COMMIT-001"
			post: ["group_staged"]
			role:      "step"
			statement: "STAGE the group: `git add <files>`."
			title:     "Stage group"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-COMMIT-004"
			role:      "verify"
			statement: "Before commit: VERIFY `git diff --cached --name-only` lists exactly the group's files; `git restore --staged <stray paths>`, then add any missing ones."
			title:     "Index equals group"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-COMMIT-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "only granted forms — no `-a`/`-F`/`-C <dir>` or other flag before `-m`, no `add && commit` chain, no `$(…)`/heredoc message."
			}, {
				label:  "param"
				marker: "*"
				text:   "groups strictly one at a time, never parallel calls — a denied commit leaves every group staged together."
			}]
			role:      "step"
			statement: "RUN `git commit -m \"<message>\"` (or `git commit -q -m`) as its own Bash call from the cwd, body as second `-m`; confirm, next group."
			title:     "Commit group"
		}
	}]
}, {
	title: "5. Show result"
	blocks: [{
		entry: {
			id:        "SKILL-PACKAGECOMMIT-RESULT-001"
			role:      "step"
			statement: "SHOW the commit log: `git log --oneline -10`."
			title:     "Show log"
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-PACKAGECOMMIT-RULES-002"
			pred:      "not packages_mixed"
			role:      "rule"
			statement: "Worker MUST NEVER combine unrelated packages in one commit."
			title:     "No mixing"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-RULES-003"
			pred:      "not pushed"
			role:      "rule"
			statement: "Worker MUST NOT push (local only)."
			title:     "No push"
		}
	}, {
		entry: {
			id:        "SKILL-PACKAGECOMMIT-RULES-004"
			pred:      "amended => amend_requested"
			role:      "rule"
			statement: "Worker MUST NEVER amend commits unless explicitly asked."
			title:     "Never amend"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-RULES-005"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "\"no desc\" / \"no dsc\" means the subject line only, no body."
			}]
			pred:      "commit_message_clean"
			role:      "rule"
			statement: "Commit messages MUST carry no review-rule IDs, no Claude attribution, and no co-author trailer — ever, dot-notation `package: message` across all languages."
			title:     "Clean messages"
		}
	}, {
		entry: {
			id: "SKILL-PACKAGECOMMIT-RULES-006"
			items: [{
				label:  "Applies"
				marker: "*"
				text:   "e.g. \"only one commit\" overrides grouping."
			}]
			pred:      "gate_waiver => waiver_honored"
			role:      "rule"
			statement: "Worker MUST honor inline gate waivers as legitimate overrides of this skill's gates and grouping."
			title:     "Gate waivers"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "procedural grouping + commit loop"
			tier:   "mid-tier"
			unviable: ["delegated run (sonnet runner, 2026-07-24) — runner round-trip far slower than inline; not delegatable"]
		}
	}]
}]
smoke: {
	expect: {
		check: ["git", "cat-file", "-e", "HEAD:smoke-pc-b/note.md"]
	}
	fixture: {
		tree: "tree"
	}
	invoke: ""
}
