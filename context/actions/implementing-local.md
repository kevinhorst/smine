<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# Implementing — local additions

Unpublished doctrine layered on top of `implementing.md`. Numbering continues the baseline
range — before adding a new baseline entry in either file, check both for ID collisions
(`make audit` catches duplicates). `*-local.md` files are excluded from the public smine repo
(see docs/publishing.md).

## Data integrity


**ACTION-IMPL-INTEG-001** `[review]` `[DoD]` — Never edit or delete an applied migration; corrections are new migrations.

* Why: migration history is append-only — editing an applied migration desyncs environments that already ran it.
* Applies: diff touches migration files or migration tooling.

**ACTION-IMPL-INTEG-002** `[review]` — No auto-complete or auto-rollback timers on stateful flows; state transitions fire on explicit events, never on clocks.

* Why: clock-driven transitions mask lost events and complete flows that never happened.
* Applies: diff touches a state machine, scheduled job, or timer wired to persisted state.

**ACTION-IMPL-INTEG-003** `[review]` — Identity resolution never uses fuzzy matching without explicit approval; lookups are exact.

* Why: a fuzzy match on identity silently merges or misattributes accounts — unrecoverable data damage.
* Applies: diff resolves users, accounts, or entities by identifier or search.

**ACTION-IMPL-INTEG-004** `[review]` — Non-idempotent side effects (sends, charges, external calls) claim before they fire: persist the claim in the same transaction, so a retry sees the claim and skips.

* Why: without a transactional claim, a retry after a crash fires the side effect twice.
* Applies: diff adds or changes a send, charge, or external call inside a stateful flow.

**ACTION-IMPL-INTEG-005** `[review]` — Every switch over a domain enum names all members; no `default` that silently swallows new ones.

* Why: a swallowing default turns an added enum member into silent misbehavior instead of a compile/test failure.
* Applies: diff adds a domain enum member or a switch over a domain enum.

**ACTION-IMPL-INTEG-006** `[review]` — Dev/test credentials are deterministic and documented — never generated per run.

* Why: per-run credentials break reproducibility and hide credential handling from review.
* Applies: diff touches dev-stack seeds, fixtures, or local credential setup.
