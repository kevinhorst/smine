<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# Concepting

Doctrine for shaping a change before code — the plan is where these gates bind. Every `[gate]`
entry blocks implementation until the plan carries what it demands. Cite entry IDs.

## Hot classes

Every class below requires an approved example implementation in the plan before any code is
written. Repos add their own classes as overlay entries (`ACTION-CONCEPT-HOT-100+`).

**ACTION-CONCEPT-HOT-001** `[gate]` — Present an approved example implementation in the plan before writing SQL with CTEs.

* Applies: planned implementation contains SQL with common table expressions.

**ACTION-CONCEPT-HOT-002** `[gate]` — Present an approved example implementation in the plan before writing concurrency primitives (goroutines/channels, threads, locks, async tasks).

* Applies: planned implementation adds or changes concurrency primitives.