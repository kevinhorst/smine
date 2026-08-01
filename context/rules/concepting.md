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