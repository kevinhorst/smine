changelog: [{
	date:    "2026-09-28"
	version: "3.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2168
}, {
	date:    "2026-09-26"
	version: "3.2.0"
	text:    "Inventory route declares its anchor: between REUSE-002b..AUTHOR-001 (optional insertion); the route line stays in the deployed skill."
}, {
	date:    "2026-09-24"
	version: "3.1.1"
	text:    "entry-point discovery and documentation target AGENTS.md — CLAUDE.md retired"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section-kind lines (Rules/Inventory), pred/post clauses on 13 flow entries + 2 invariants, sibling atoms.json (15 bool atoms), 2 queries, caveman compression pass."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format v2 reformat: Routing section, Makefile-target enumeration fenced, caveman pass"
}, {
	date:    "2026-09-05"
	version: "1.6"
	text:    "Body migrated to titled entries (/fmt skill) — content identical"
}, {
	date:    "2026-09-04"
	version: "1.5"
	text:    "acdsl-context narrowed FACT-* to FACT-REPO-*: the new Claude platform facts are not stack-setup input"
}, {
	date:    "2026-09-04"
	version: "1.4"
	text:    "acdsl-context added: repo facts (stack, DB, services) directly serve stack setup"
}, {
	date:    "2026-08-23"
	version: "1.3"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-07-30"
	version: "1.2"
	text:    "moved under skills/quality/ group; name and behavior unchanged"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-03"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " dev-stack"
}, {
	key:   "description"
	value: " Create or use a project-local e2e dev stack (docker-compose + Makefile + seeded data). Trigger on /dev-stack or \"local end-to-end testing\" or \"docker-compose setup\"."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.2.1"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}]
preamble: """
	# Dev Stack

	Build or reuse a project-local e2e dev stack, then verify changes against it.

	Goal: agent starts system locally, verifies changes end-to-end in own environment. Stack always project-specific — this skill
	defines only how to build and use one; all project specifics (compose file, Makefile, exclusions, seed shape) live in project,
	never here.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["Docker available on the host", "services and datastores known"]
			purpose: "create or reuse project-local e2e stack (docker-compose + Makefile + seeded data) so changes verify in running system."
			routes: [{
				for: "unit/package-level coverage gaps"
				use: "/coverage-increase"
			}, {
				for: "diagnosing a bug that needs no running stack"
				use: "/diagnose-debug"
			}, {
				for: "planning the feature itself"
				use: "/fdesign"
			}]
		}
	}]
}, {
	title: "Rules"
	blocks: [{
		entry: {
			id:        "SKILL-DEVSTACK-RULES-003"
			role:      "def"
			statement: "E2E verification of a change means exercising real frontend↔backend path in this stack; passing unit build alone is not verification."
			title:     "E2E verification defined"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-RULES-001"
			pred:      "blocker_found => blocker_surfaced"
			role:      "rule"
			statement: "Blockers MUST be surfaced as decisions, never silently designed around."
			title:     "Blockers are decisions"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-RULES-002"
			pred:      "build_started and fresh_worktree => deps_installed"
			role:      "rule"
			statement: "Fresh worktrees MUST install dependencies first (e.g. `npm install`) before any build or dev server."
			title:     "Fresh worktrees"
		}
	}]
	kind: "context"
}, {
	title: "1. Reuse before building"
	blocks: [{
		entry: {
			id:        "SKILL-DEVSTACK-REUSE-001"
			role:      "step"
			statement: "LOOK for existing entry points: `docker-compose*.yml`, top-level `Makefile` (`up`/`seed` targets), a project skill, an AGENTS.md section."
			title:     "Entry points"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-REUSE-002a"
			pred:      "stack_found"
			role:      "step"
			statement: "USE existing entry point if one exists: `make up`, verify, done."
			title:     "Use existing"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-REUSE-002b"
			pred:      "stack_found"
			role:      "stop"
			statement: "If existing stack found ⇒ STOP: never build second stack next to it."
			title:     "No second stack"
		}
	}]
}, {
	title:   "2. Inventory"
	between: "REUSE-002b..AUTHOR-001"
	blocks: [{
		entry: {
			id:        "SKILL-DEVSTACK-INVENTORY-001a"
			role:      "step"
			statement: "LIST all services + datastores; for each, check whether it can run on this host."
			title:     "List services"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-INVENTORY-001b"
			pred:      "platform_locked"
			role:      "stop"
			statement: "If a component is platform-locked ⇒ STOP: exclude it with a documented reason, never emulate it."
			title:     "No emulation"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-INVENTORY-002a"
			role:      "step"
			statement: "HUNT for the blocker that kills the naive approach: interactive auth flows, endpoints locked by a mode/flag, licensed components."
			title:     "Hunt for blockers"
		}
	}, {
		entry: {
			id: "SKILL-DEVSTACK-INVENTORY-002b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "typical options — mount existing session/credential dirs, remove the dead mode entirely, exclude the component."
			}]
			post: ["blocker_surfaced"]
			pred:      "blocker_found"
			role:      "step"
			statement: "PRESENT blockers as explicit decision points before writing anything."
			title:     "Surface as decisions"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-INVENTORY-003a"
			role:      "step"
			statement: "CONFIRM whether prod credentials are acceptable for local testing, for third-party credentials."
			title:     "Confirm credential use"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-INVENTORY-003b"
			pred:      "gap_tempts_workaround"
			role:      "stop"
			statement: "If a gap tempts a workaround ⇒ STOP: never invent mock services unprompted."
			title:     "No unprompted mocks"
		}
	}]
	kind: "route"
	pred: "not stack_found"
}, {
	title: "3. Author"
	blocks: [{
		entry: {
			id:        "SKILL-DEVSTACK-AUTHOR-004b"
			pred:      "views_seeded => views_populated"
			role:      "rule"
			statement: "Seeded UI views MUST show something — empty views hide bugs."
			title:     "Seeded views populated"
		}
	}, {
		entry: {
			id: "SKILL-DEVSTACK-AUTHOR-001"
			post: ["compose_written"]
			role:      "step"
			statement: "WRITE `docker-compose.yml` with the runnable subset; verify `env_file` and volume paths resolve relative to the compose file's directory."
			title:     "Compose file"
		}
	}, {
		entry: {
			id: "SKILL-DEVSTACK-AUTHOR-002"
			items: [{
				fence: {
					body: """
						up
						down
						build
						logs
						seed
						clean
						status

						"""
					info:   "text"
					marker: "```"
				}
			}]
			post: ["makefile_written"]
			role:      "step"
			statement: "WRITE a top-level `Makefile` with these targets:"
			title:     "Top-level Makefile"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-AUTHOR-003a"
			role:      "step"
			statement: "CREATE dedicated config file per environment (e.g. `config.docker.json`)."
			title:     "Config per environment"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-AUTHOR-003b"
			pred:      "config_mutation_needed"
			role:      "stop"
			statement: "If a config change is needed ⇒ STOP: never mutate prod/dev configs."
			title:     "No config mutation"
		}
	}, {
		entry: {
			id: "SKILL-DEVSTACK-AUTHOR-004a"
			post: ["views_seeded"]
			role:      "step"
			statement: "SEED data with realistic values: symbols, plausible prices, UUIDs, recent timestamps, ALL entity types, edge states included (valid/invalid, open/closed/failed)."
			title:     "Seed data values"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-AUTHOR-005"
			role:      "step"
			statement: "WRITE `.env.example` documenting required variables."
			title:     "Env example"
		}
	}]
}, {
	title: "4. Verify the stack itself"
	blocks: [{
		entry: {
			id: "SKILL-DEVSTACK-VERIFY-001"
			post: ["stack_up"]
			pred:      "compose_written and makefile_written"
			role:      "step"
			statement: "RUN `make up` from a clean state, then `make status`."
			title:     "Up and status"
		}
	}, {
		entry: {
			id: "SKILL-DEVSTACK-VERIFY-002"
			post: ["views_populated"]
			pred:      "stack_up"
			role:      "step"
			statement: "CURL health + one list endpoint per service; confirm frontend renders seeded data."
			title:     "Health and render"
		}
	}, {
		entry: {
			id: "SKILL-DEVSTACK-VERIFY-003"
			post: ["cycle_verified"]
			pred:      "stack_up"
			role:      "verify"
			statement: "VERIFY `make down && make up` is idempotent."
			title:     "Idempotent cycle"
		}
	}]
}, {
	title: "5. Persist knowledge in the project"
	blocks: [{
		entry: {
			id:        "SKILL-DEVSTACK-PERSIST-001"
			pred:      "cycle_verified"
			role:      "step"
			statement: "DOCUMENT in project AGENTS.md (or project skill): how to start, seed, verify — what excluded and why."
			title:     "Document in project"
		}
	}, {
		entry: {
			id:        "SKILL-DEVSTACK-PERSIST-002"
			role:      "step"
			statement: "GITIGNORE runtime artifacts (volumes, session dirs)."
			title:     "Gitignore artifacts"
		}
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "compose/Makefile scaffolding follows known patterns"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	skip: "needs Docker and a project's services running on the host"
}
