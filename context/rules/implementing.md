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