# Reviewing — Definition of Done

**For reviewers / agents:** cite the stable `RULE-DOD-*` id when marking an item PASS, FAIL, or N/A.

## 1. Functionality & Spec
- [ ] **RULE-DOD-101** — Feature matches the agreed spec
- [ ] **RULE-DOD-102** — Inputs validated; auth checks enforced on new or changed endpoints
- [ ] **RULE-DOD-103** — Outputs (response format, status codes, error cases) are correct
- [ ] **RULE-DOD-104** — No behavior regressions — existing flows re-checked where touched (backward compatibility)

## 2. End-to-End Verification
- [ ] **RULE-DOD-201** — Happy path verified end-to-end in a running system (realistic use case)
- [ ] **RULE-DOD-202** — At least 1–2 relevant edge cases verified
- [ ] **RULE-DOD-203** — Error cases verified (e.g. invalid input, missing data)
- [ ] **RULE-DOD-204** — Smoke-test runbook persisted under `plans/<feature>/runbooks/` in the project's smoke-test tool format, covering the verified scenarios
- [ ] **RULE-DOD-205** — Runbook executed green against the running system

## 3. Automated Tests & Build
- [ ] **RULE-DOD-301** — Unit or integration tests added for core logic
- [ ] **RULE-DOD-302** — Existing tests updated (if behavior changed)
- [ ] **RULE-DOD-303** — Tests pass
- [ ] **RULE-DOD-304** — Build, vet, and linters pass

## 4. Database & Migrations (if schema or data changed)
- [ ] **RULE-DOD-401** — Schema migrations implemented correctly (including indexes, defaults, constraints)
- [ ] **RULE-DOD-402** — Data migrations created for initial data population
- [ ] **RULE-DOD-403** — Migration tested (up + rollback considered)
- [ ] **RULE-DOD-404** — Existing data / edge cases accounted for (null values, legacy data, etc.)

## 5. Logging & Error Behavior
- [ ] **RULE-DOD-501** — Relevant errors are logged
- [ ] **RULE-DOD-502** — No debug logs left in code
- [ ] **RULE-DOD-503** — Errors carry enough context to locate the failure (no opaque errors)
- [ ] **RULE-DOD-504** — No secrets in code, config, or logs
- [ ] **RULE-DOD-505** — Debug/temporary logging redacts sensitive payloads — no payment nonces, no full provider request/response bodies

## 6. Deployment & Operations
- [ ] **RULE-DOD-601** — Required steps documented (e.g. migration, config, env vars)
- [ ] **RULE-DOD-602** — Changes are backward-compatible for clients

## 7. Code Quality
- [ ] **RULE-DOD-701** — Code follows project conventions (naming, structure)
- [ ] **RULE-DOD-702** — Code follows the defined style guides
- [ ] **RULE-DOD-703** — TODOs introduced by this change resolved or ticketed
- [ ] **RULE-DOD-704** — No unnecessary complexity / dead code
- [ ] **RULE-DOD-705** — No unnecessary code duplication
- [ ] **RULE-DOD-706** — No superfluous comments — a comment exists only where intent or a non-obvious assumption cannot be expressed in code

## 8. Documentation & Agent Guidance
- [ ] **RULE-DOD-801** — `AGENTS.md` updated if agent instructions or review expectations changed
- [ ] **RULE-DOD-802** — Relevant files in context directory updated if workflows, context, or operational knowledge changed
- [ ] **RULE-DOD-803** — Status/progress docs re-derived from git log/diff — never updated from memory

## Definition of "Done"
> Done means: every item above is PASS or N/A — and each N/A has a stated reason.
