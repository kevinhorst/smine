# Implementing

Doctrine for writing and changing code — any session, skill-driven or ad hoc. Planning skills
copy the `ALWAYS-EXEC-*` entries into every plan's Stop conditions table; every `[gate]` entry
blocks continuing until its condition is met. The hot-class gates live in `concepting.md` — they
bind at plan time. Cite entry IDs.

## Scoping

**ALWAYS-ARCH-001** `[review]` — Every new concept (type, file, interface, goroutine, dependency, endpoint) traces back to the request — otherwise remove it or stop and ask.

* Why: untraceable concepts are speculative scope — complexity nobody asked for.
* Applies: any diff introducing a new named concept.

## Stop conditions

**ALWAYS-EXEC-001** `[gate]` — Stop and report when an approved signature or contract can't hold as planned; never improvise architecture mid-edit.

* Applies: any plan-driven implementation.

**ALWAYS-EXEC-002** `[gate]` — Stop after the second failed fix on the same mechanism: research the actual cause and redesign — no third band-aid.

* Applies: any debugging or fixing loop.

**ALWAYS-EXEC-003** `[gate]` — Run the producing step for a missing prerequisite (generated code, running infra); if infrastructure is down, ask — never skip validation, never start infrastructure yourself.

* Applies: any build, generate, or integration step.

**ALWAYS-EXEC-004** `[gate]` — Ask before continuing when discovered work materially exceeds the approved scope.

* Applies: any approved plan or task.

**ALWAYS-EXEC-005** `[gate]` — On finding the same kind of bug a second time: inside your own diff, fix every instance in the diff now; pre-existing outside the diff, report and ask before searching further.

* Why: sweeps eat context and are the user's call.
* Applies: any implementation or review session.

**ALWAYS-EXEC-006** `[gate]` — Stop and report when a structural obstacle (import cycle, package visibility) tempts a new abstraction; the fix is relocating the component, not indirection.

* Applies: any implementation touching package structure.

## Data integrity

**NEVER-INTEG-001** `[review]` — Never edit or delete an applied migration; corrections are new migrations.

* Why: migration history is append-only — editing an applied migration desyncs environments that already ran it.
* Applies: diff touches migration files or migration tooling.

**NEVER-INTEG-002** `[review]` — No auto-complete or auto-rollback timers on stateful flows; state transitions fire on explicit events, never on clocks.

* Why: clock-driven transitions mask lost events and complete flows that never happened.
* Applies: diff touches a state machine, scheduled job, or timer wired to persisted state.

**NEVER-INTEG-003** `[review]` — Identity resolution never uses fuzzy matching without explicit approval; lookups are exact.

* Why: a fuzzy match on identity silently merges or misattributes accounts — unrecoverable data damage.
* Applies: diff resolves users, accounts, or entities by identifier or search.

**ALWAYS-INTEG-001** `[review]` — Non-idempotent side effects (sends, charges, external calls) claim before they fire: persist the claim in the same transaction, so a retry sees the claim and skips.

* Why: without a transactional claim, a retry after a crash fires the side effect twice.
* Applies: diff adds or changes a send, charge, or external call inside a stateful flow.

**ALWAYS-INTEG-002** `[review]` — Every switch over a domain enum names all members; no `default` that silently swallows new ones.

* Why: a swallowing default turns an added enum member into silent misbehavior instead of a compile/test failure.
* Applies: diff adds a domain enum member or a switch over a domain enum.

**ALWAYS-INTEG-003** `[review]` — Dev/test credentials are deterministic and documented — never generated per run.

* Why: per-run credentials break reproducibility and hide credential handling from review.
* Applies: diff touches dev-stack seeds, fixtures, or local credential setup.
