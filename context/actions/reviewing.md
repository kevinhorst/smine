# Reviewing — Definition of Done

**For reviewers / agents:** every entry below is marked PASS, FAIL, or N/A when reviewing —
cite the stable id, and every N/A carries a stated reason.

## Functionality & Spec

**ACTION-REVIEW-SPEC-001** `[review]` `[DoD]` — Feature matches the agreed spec.

* Applies: every reviewed change.

**ACTION-REVIEW-SPEC-002** `[review]` `[DoD]` — Inputs validated; auth checks enforced on new or changed endpoints.

* Applies: new or changed endpoints.

**ACTION-REVIEW-SPEC-003** `[review]` `[DoD]` — Outputs (response format, status codes, error cases) are correct.

* Applies: every reviewed change with an observable output.

**ACTION-REVIEW-SPEC-004** `[review]` `[DoD]` — No behavior regressions — existing flows re-checked where touched (backward compatibility).

* Applies: every reviewed change touching existing flows.

**ACTION-REVIEW-SPEC-005** `[review]` `[DoD]` — Concept audit: every new concept the change introduces (type, file, interface, concurrency primitive, dependency, endpoint) traces back to a requirement.

* An addition tracing to no requirement is behavior nobody asked for — a finding, not initiative.
* Applies: every reviewed change against a spec, plan, or request.

## End-to-End Verification

**ACTION-REVIEW-VERIFY-001** `[review]` `[DoD]` — Happy path verified end-to-end in a running system (realistic use case).

* Applies: every reviewed change.

**ACTION-REVIEW-VERIFY-002** `[review]` `[DoD]` — At least 1–2 relevant edge cases verified.

* Applies: every reviewed change.

**ACTION-REVIEW-VERIFY-003** `[review]` `[DoD]` — Error cases verified (e.g. invalid input, missing data).

* Applies: every reviewed change with error paths.

**ACTION-REVIEW-VERIFY-004** `[review]` `[DoD]` — Smoke-test runbook persisted under `plans/<feature>/runbooks/` in the project's smoke-test tool format, covering the verified scenarios.

* Applies: every feature or change with a callable surface.

**ACTION-REVIEW-VERIFY-005** `[review]` `[DoD]` — Runbook executed green against the running system.

* Applies: every change with a persisted runbook.

**ACTION-REVIEW-VERIFY-006** `[review]` `[DoD]` — A file consumed by a third-party tool is done only when that tool accepts it.

* For any tool-consumed artifact (CI YAML, MCP/client config, API-collection formats, generated formats): verify against the vendor doc or a working example before emitting, prove the host actually reads each config surface, and confirm the consuming tool accepts it (lint passes / client connects). "File written" is not done.
* Applies: every generated or config artifact read by an external tool.

**ACTION-REVIEW-VERIFY-007** `[review]` `[DoD]` — Reproduce against a freshly built artifact at the exact path/command the user invokes — never a stale binary.

* Before reporting a bug fixed — or attributing a failure to the environment — rebuild at the exact path/command the user invokes and reproduce the failing invocation against that fresh build. Never blame a stale binary or user error; the user always rebuilds.
* Applies: every reported fix and every environment-attributed failure.

## Automated Tests & Build

**ACTION-REVIEW-TEST-001** `[review]` `[DoD]` — Unit or integration tests added for core logic.

* Applies: every reviewed change with new logic.

**ACTION-REVIEW-TEST-002** `[review]` `[DoD]` — Existing tests updated when behavior changed.

* Applies: behavior-changing diffs.

**ACTION-REVIEW-TEST-003** `[review]` `[DoD]` — Tests pass.

* Applies: every reviewed change.

**ACTION-REVIEW-TEST-004** `[review]` `[DoD]` — Build, vet, and linters pass.

* Applies: every reviewed change.

**ACTION-REVIEW-TEST-005** `[review]` `[DoD]` — Deterministic test-mode constants used as uniqueness or identity keys are a BLOCKER, never a nit.

* Applies: every reviewed change with test-mode or fixture constants near identity/uniqueness logic.

## Database & Migrations

**ACTION-REVIEW-MIGRATION-001** `[review]` `[DoD]` — Schema migrations implemented correctly (including indexes, defaults, constraints).

* Applies: schema changes.

**ACTION-REVIEW-MIGRATION-002** `[review]` `[DoD]` — Data migrations created for initial data population.

* Applies: changes needing seeded data.

**ACTION-REVIEW-MIGRATION-003** `[review]` `[DoD]` — Migration tested (up + rollback considered).

* Applies: every migration.

**ACTION-REVIEW-MIGRATION-004** `[review]` `[DoD]` — Existing data / edge cases accounted for (null values, legacy data, etc.).

* Applies: every migration over existing data.

## Logging & Error Behavior

**ACTION-REVIEW-LOG-001** `[review]` `[DoD]` — Relevant errors are logged.

* Applies: every reviewed change with error paths.

**ACTION-REVIEW-LOG-002** `[review]` `[DoD]` — No debug logs left in code.

* Applies: every reviewed change.

**ACTION-REVIEW-LOG-003** `[review]` `[DoD]` — Errors carry enough context to locate the failure (no opaque errors).

* Applies: every reviewed change with error paths.

**ACTION-REVIEW-LOG-004** `[review]` `[DoD]` — No secrets in code, config, or logs.

* Applies: every reviewed change.

**ACTION-REVIEW-LOG-005** `[review]` `[DoD]` — Debug/temporary logging redacts sensitive payloads — no payment nonces, no full provider request/response bodies.

* Applies: every reviewed change logging external payloads.

## Deployment & Operations

**ACTION-REVIEW-DEPLOY-001** `[review]` `[DoD]` — Required steps documented (e.g. migration, config, env vars).

* Applies: every change needing deployment steps.

**ACTION-REVIEW-DEPLOY-002** `[review]` `[DoD]` — Changes are backward-compatible for clients.

* Applies: every contract-facing change.

## Code Quality

**ACTION-REVIEW-QUALITY-001** `[review]` `[DoD]` — Code follows project conventions (naming, structure).

* Applies: every reviewed change.

**ACTION-REVIEW-QUALITY-002** `[review]` `[DoD]` — Code follows the defined style guides.

* Applies: every reviewed change.

**ACTION-REVIEW-QUALITY-003** `[review]` `[DoD]` — TODOs introduced by this change resolved or ticketed.

* Applies: every reviewed change.

**ACTION-REVIEW-QUALITY-004** `[review]` `[DoD]` — No unnecessary complexity / dead code.

* Applies: every reviewed change.

**ACTION-REVIEW-QUALITY-005** `[review]` `[DoD]` — No unnecessary code duplication.

* Applies: every reviewed change.

**ACTION-REVIEW-QUALITY-006** `[review]` `[DoD]` — No superfluous comments — a comment exists only where intent or a non-obvious assumption cannot be expressed in code.

* Applies: every reviewed change.

**ACTION-REVIEW-QUALITY-007** `[review]` `[DoD]` — Generated files are regenerated from their source, never hand-edited.

* Applies: every reviewed change touching generated artifacts.

**ACTION-REVIEW-QUALITY-008** `[review]` `[DoD]` — Temporal TODOs ("remove after X", "until Y ships") are checked against the current date — expired ones are findings.

* Applies: every reviewed change containing or passing dated TODOs.

## Documentation & Agent Guidance

**ACTION-REVIEW-DOCS-001** `[review]` `[DoD]` — `AGENTS.md` updated if agent instructions or review expectations changed.

* Applies: changes to instructions, workflows, or expectations.

**ACTION-REVIEW-DOCS-002** `[review]` `[DoD]` — Relevant files in the context directory updated if workflows, context, or operational knowledge changed.

* Applies: changes to workflows or operational knowledge.

**ACTION-REVIEW-DOCS-003** `[review]` `[DoD]` — Status/progress docs re-derived from git log/diff — never updated from memory.

* Applies: every status or progress doc update.

**ACTION-REVIEW-DOCS-004** `[review]` `[DoD]` — A shipped feature's known-broken guards or dead dependencies are recorded as tracked open items, not buried as code comments.

* When a feature ships with a dependency that is known-broken or not-yet-available (an upstream API field missing, a guard that cannot function), the definition-of-done includes recording it as a tracked open item — not just a code comment. A guard that always evaluates one way is a documented gap, not "done".
* Applies: any feature shipping with a known-dead or not-yet-available dependency.

## Risk Tiering (review intake)

Consumed at review intake (e.g. the railroad-review risk map) — tier every changed file before any defect hunting; repo overlays may extend or re-tier.

**ACTION-REVIEW-RISK-001** `[review]` — **High tier**: migrations, auth, concurrency, money, public API contracts, persisted data.

* Applies: review-intake tiering of every changed file.

**ACTION-REVIEW-RISK-002** `[review]` — **Medium tier**: business logic, error handling.

* Applies: review-intake tiering of every changed file.

**ACTION-REVIEW-RISK-003** `[review]` — **Low tier**: tests, renames, formatting, generated files, boilerplate — read only for accidental scope creep.

* Applies: review-intake tiering of every changed file.

## Definition of "Done"

> Done means: every entry above is PASS or N/A — and each N/A has a stated reason.
