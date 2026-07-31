---
name: dev-stack
description: Create or use a project-local e2e dev stack (docker-compose + Makefile + seeded data). Trigger on /dev-stack or "local end-to-end testing" or "docker-compose setup".
author: Kevin Horst
version: 1.2
---

# Dev Stack

Goal: the agent can start the system locally and verify its changes end-to-end inside its own environment. The stack itself is always project-specific — this skill defines only how to build and use one. All project specifics (compose file, Makefile, exclusions, seed shape) live in the project, never in this skill.

## When to use

**Use when:** a change needs e2e verification in a running system (docker-compose + Makefile + seeded data), or when full-stack verification is blocked because services are not running.
**Don't use when:** unit/package-level coverage gaps — /coverage-increase. Diagnosing a bug that does not require running the stack — /diagnose-debug. Planning the feature itself — /fdesign.
**Preconditions:** Docker available on the host; known services and datastores.
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo).

## 1. Reuse before building

- Look for existing entry points: `docker-compose*.yml`, top-level `Makefile` (`up`/`seed` targets), a project skill, a CLAUDE.md section.
- If one exists: use it (`make up` → verify → done). Never build a second stack next to an existing one.

## 2. Inventory (only when authoring a new stack)

- List all services + datastores. For each: can it run on this host? Platform-locked components are excluded with a documented reason, not emulated.
- Hunt for the blocker that kills the naive approach: interactive auth flows, endpoints locked by a mode/flag, licensed components. Present blockers as explicit decision points BEFORE writing anything — typical options: mount existing session/credential dirs, remove the dead mode entirely, exclude the component.
- Third-party credentials: confirm whether prod credentials are acceptable for local testing. Do not invent mock services unprompted.

## 3. Author

- `docker-compose.yml` with the runnable subset. Verify `env_file` and volume paths resolve relative to the compose file's directory.
- Top-level `Makefile`: `up`, `down`, `build`, `logs`, `seed`, `clean`, `status`.
- Config per environment: dedicated config file (e.g. `config.docker.json`); never mutate prod/dev configs.
- Seed data: realistic values (real symbols, plausible prices, UUIDs, timestamps from the last days), ALL entity types, edge states included (valid/invalid, open/closed/failed). Every UI view must have something to display — empty views hide bugs.
- `.env.example` documenting required variables.

## 4. Verify the stack itself

- `make up` from a clean state, then `make status`.
- curl health + one list endpoint per service; confirm the frontend renders seeded data.
- `make down && make up` must be idempotent.

## 5. Persist knowledge in the project

- Document in the project CLAUDE.md (or a project skill): how to start, seed, verify — and what is excluded and why.
- Gitignore runtime artifacts (volumes, session dirs).

## Rules

- Blockers are surfaced as decisions, never silently designed around.
- Fresh worktrees: install dependencies first (e.g. `npm install`) before any build or dev server.
- E2E verification of a change means exercising the real frontend↔backend path in this stack — a passing unit build is not verification.

## Model

- Suggested: mid-tier / medium
- Reason: compose/Makefile scaffolding follows known patterns
- Tested unviable: — (none yet)

## Changelog

- v1.2 (2026-07-30): moved under skills/quality/ group; name and behavior unchanged
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-03): initial version
