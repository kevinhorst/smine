# Implementing

Doctrine for writing and changing code — any session, skill-driven or ad hoc. Planning skills
copy the `ACTION-IMPL-*` entries into every plan's Stop conditions table; every `[gate]` entry
blocks continuing until its condition is met. The hot-class gates live in `concepting.md` — they
bind at plan time. Cite entry IDs.

## Scoping

**ACTION-IMPL-ARCH-001** `[review]` — Every new concept (type, file, interface, concurrency primitive, dependency, endpoint) traces back to the request — otherwise remove it or stop and ask.

* Why: untraceable concepts are speculative scope — complexity nobody asked for.
* Applies: any diff introducing a new named concept.

## Stop conditions

**ACTION-IMPL-001** `[gate]` — Stop and report when an approved signature or contract can't hold as planned; never improvise architecture mid-edit.

* Applies: any plan-driven implementation.

**ACTION-IMPL-002** `[gate]` — Stop after the second failed approach in a row: re-read the actual file state, research the cause, and write a plan — no third band-aid.

* Broadened beyond one mechanism: after the second failed *approach* in a row — exploratory flip-flopping through edits, not just repeated fixes to the same spot — stop editing, re-read the real state on disk, and write a plan before touching more code.
* This is the two-attempt rule's floor: after two rejected *designs* for one logic unit, stop generating versions. When two consecutive planning turns each reverse a prior design decision, that is requirement-discovery, not planning — hand back or ship a rough version; do not keep refining.
* Applies: any debugging, fixing, or design loop.

**ACTION-IMPL-003** `[gate]` — Run the producing step for a missing prerequisite (generated code, running infra); if infrastructure is down, ask — never skip validation, never start infrastructure yourself.

* Applies: any build, generate, or integration step.

**ACTION-IMPL-004** `[gate]` — Ask before continuing when discovered work materially exceeds the approved scope.

* Applies: any approved plan or task.

**ACTION-IMPL-005** `[gate]` — On finding the same kind of bug a second time: inside your own diff, fix every instance in the diff now; pre-existing outside the diff, report and ask before searching further.

* Why: sweeps eat context and are the user's call.
* Applies: any implementation or review session.

**ACTION-IMPL-006** `[gate]` — Stop and report when a structural obstacle (import cycle, package visibility) tempts a new abstraction; the fix is relocating the component, not indirection.

* Applies: any implementation touching package structure.

## Data integrity

**ACTION-IMPL-INTEG-007** `[review]` — Never weaken, bypass, or delete a validation, transaction, or guard to make an error go away.

* A legitimate new case that a check rejects → restructure the check to model it. Bad input → fix the input or its generator and file the upstream bug.
* Removing a safety mechanism is a semantics change that requires explicit sign-off.
* Applies: any change where a validation, constraint, or guard is failing.

**ACTION-IMPL-INTEG-008** `[review]` — Derive state from the source of truth over a trigger-maintained boolean flag.

* Don't add a derived boolean flag maintained by a trigger when the state is directly derivable (`x IS NOT NULL`, row presence). Add one only if the flag can genuinely diverge from its source; otherwise it is a desync risk and premature abstraction.
* Applies: any schema or model change introducing a status/flag column.

**ACTION-IMPL-INTEG-009** `[review]` — Never overload an existing field's semantics; add an explicit field.

* Never overload one field to carry a second meaning (a points column doubling as accuracy, one flag serving two states). Add an explicit field/column and model the distinct concept.
* Applies: any change that reuses an existing field for a new meaning.

**ACTION-IMPL-INTEG-010** `[review]` — All-time / historical aggregates never filter on mutable current-state flags.

* An all-time or historical aggregate MUST NOT be filtered by a mutable current-state flag (`is_active`, `is_enabled`) — the flag's present value silently rewrites history. Filter on immutable facts (row existence, timestamps).
* Applies: any query or metric describing a historical total.

**ACTION-IMPL-INTEG-011** `[review]` — Forging another system's persisted artifacts validates against the consuming deployment's live config, never framework defaults.

* When forging or emulating another system's persisted artifacts (sessions, tokens, cookies, signed payloads), enumerate config-dependent fields (backends, secrets, serializers, salts) as design inputs and validate against the *consumer's* live configuration.
* Diagnostic: if the consumer re-saves the injected data, it parsed and validated it — the failure is downstream of decoding.
* Applies: any cross-system artifact emulation. Overlaps the test-side RULE-GOLANG-TEST-010.

**ACTION-IMPL-INTEG-012** `[review]` — DB routine files are idempotent and self-contained; a routine is deleted when the column it maintains is dropped.

* Every `CREATE TRIGGER` is preceded by `DROP TRIGGER IF EXISTS <name> ON <table>;` (never a bare `DROP TRIGGER`), functions use `CREATE OR REPLACE FUNCTION`, and everything sits inside one `BEGIN; … COMMIT;` block.
* Why: plpgsql resolves columns at execution time — a routine whose target column was dropped applies cleanly and then breaks every write to the triggering table.
* Applies: diff touches `<app>/dbroutines/` files or drops a trigger-maintained column.
* Reach: backend

**ACTION-IMPL-INTEG-013** `[review]` — Advisory locks are transaction-scoped on the default connection, taken in sorted order when multiple, and never per-item over an unbounded set.

* Acquire inside `transaction.atomic(using = "default")` (autocommit would release immediately); reads backing the lock decision pin the master with `.using("default")`.
* Multiple locks in one transaction are acquired in stable `sorted()` order; a per-item lock over an unbounded set must be bounded or chunked (shared lock-table capacity).
* Keep the caller registry table in `docs/context/pg-advisory-locks.md` in sync when adding or changing a `pg_advisory_*` call.
* Applies: diff adds or changes a `pg_advisory_*` call.
* Reach: backend

**ACTION-IMPL-INTEG-014** `[review]` — A lock-guarded dedup decision recorded in Redis is written before COMMIT, while the lock is still held.

* Why: a blocked waiter re-checks Redis only after COMMIT releases the lock — deferring the write to an on-commit hook reopens the duplicate-insert race the lock exists to close.
* Applies: diff touches the comment-dedup flow or adds a lock-guarded Redis decision.
* Reach: backend

**ACTION-IMPL-MIGRATION-001** `[review]` — Migrations are minimal DDL, one concern per file; trigger/constraint logic guards the nullable-FK case.

* A migration file carries the minimal DDL for exactly one concern — no bundled grants or extras. Split multi-concern changes into separate numbered files, and order FK migrations after the table they reference.
* Trigger and constraint logic must handle the nullable-FK case explicitly.
* Applies: any schema migration.

## Execution fidelity

**ACTION-IMPL-EXEC-001** `[review]` — Execute an explicitly ordered sequence in order, with its stated boundaries; never reorder or merge separately-requested steps.

* When the user gives an ordered sequence ("commit, then extract", "do A, then B"), run it in that order with the stated boundaries — do not reorder for convenience or fold separately-requested steps into one. A checkpoint commit requested before a refactor is load-bearing.
* Applies: any multi-step instruction with explicit ordering.

**ACTION-IMPL-DEPLOY-001** `[review]` — A launchd / scheduled-job manifest references the main-checkout absolute path, never a worktree.

* A launchd plist's `ProgramArguments` and working directory MUST reference the main-checkout absolute path. A worktree path breaks once the worktree is destroyed, and worktrees lack untracked state files.
* Applies: authoring or editing any persisted job manifest.

**ACTION-IMPL-EXEC-002** `[review]` — Under `set -e`, write expected-failure assertions with an explicit `if`, and guard a `read` that can hit EOF.

* In shell scripts under `set -e`, write expected-failure assertions as `if cmd; then fail; fi`, never `cmd && fail` (the latter propagates the non-zero exit and aborts the script). Guard any `read` that can hit EOF with `|| true` or a default (`|| answer=n`).
* Applies: any shell script running under `set -e`.