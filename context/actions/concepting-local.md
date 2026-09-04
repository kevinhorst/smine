<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# Concepting — local additions

Unpublished hot-class gates layered on top of `concepting.md`. Numbering continues the baseline
range — before adding a new baseline gate in either file, check both for ID collisions
(`make audit` catches duplicates). `*-local.md` files are excluded from the public smine repo
(see docs/publishing.md).

## Hot classes

**ACTION-CONCEPT-HOT-003** `[gate]` — Present an approved example implementation in the plan before introducing a new interface or generic type.

* Applies: planned implementation adds an interface or type parameter.

**ACTION-CONCEPT-HOT-004** `[gate]` — Write the source the generator consumes; never freehand generated output — the example in the plan is the generator input.

* Applies: planned implementation touches migrations or generated formats.

**ACTION-CONCEPT-HOT-005** `[gate]` — Present an approved example implementation in the plan before adding, weakening, or removing validation, transaction, or guard logic.

* Applies: planned implementation touches validation, transactions, or guards.

**ACTION-CONCEPT-HOT-006** `[gate]` — Present an approved example in the plan before using an anonymous (inline) struct type.

* Applies: planned implementation contains an anonymous struct.

**ACTION-CONCEPT-HOT-007** `[gate]` — Present a screenshot of the actual UI under design — captured from the running app — in the plan before changing user-facing UI, referenced from every Changes entry that alters it (RULE-PLAN-069/070); a rendered mockup only when the UI does not exist yet, prose widget descriptions never.

* Applies: planned implementation adds or changes user-facing UI (templates, views, widgets, styling).
