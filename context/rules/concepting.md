# Concepting

Doctrine for shaping a change before code — the plan is where these gates bind. Every `[gate]`
entry blocks implementation until the plan carries what it demands. Cite entry IDs.

## Hot classes

Every class below requires an approved example implementation in the plan before any code is
written. Repos add their own classes as overlay entries (`ALWAYS-HOT-100+`).

**ALWAYS-HOT-001** `[gate]` — Present an approved example implementation in the plan before writing SQL with CTEs.

* Applies: planned implementation contains SQL with common table expressions.

**ALWAYS-HOT-002** `[gate]` — Present an approved example implementation in the plan before writing goroutines, channels, or locking.

* Applies: planned implementation adds or changes concurrency primitives.

**ALWAYS-HOT-003** `[gate]` — Present an approved example implementation in the plan before introducing a new interface or generic type.

* Applies: planned implementation adds an interface or type parameter.

**ALWAYS-HOT-004** `[gate]` — Write the source the generator consumes; never freehand generated output — the example in the plan is the generator input.

* Applies: planned implementation touches migrations or generated formats.

**ALWAYS-HOT-005** `[gate]` — Present an approved example implementation in the plan before adding, weakening, or removing validation, transaction, or guard logic.

* Applies: planned implementation touches validation, transactions, or guards.

**ALWAYS-HOT-006** `[gate]` — Present an approved example in the plan before using an anonymous (inline) struct type.

* Applies: planned implementation contains an anonymous struct.
