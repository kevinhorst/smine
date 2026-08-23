# ACDSL Stage-1 Corpus — wanted doctrine for this repo's Go tree

> **Status:** Draft v2 — awaiting veto (audit does not run on an unvetoed corpus)
> **Drafted:** agent, 2026-08-03, from the live source of `cmd/` + `internal/`
> **Rule:** per concept decision, fresh wanted-doctrine — NOT derived from `context/rules/*.md`

**Anchor requirement (from veto round 1, binding for fdesign):** an anchor is a deterministic, repeatable query — parser/regexp-class — that, when run against the tree, returns concrete matches. Prose scope descriptions below are sketches; their executable form is fdesign work.

## Veto history

- **v1 rejected (2026-08-03) — layer error.** v1 was import-topology and package-hygiene confinement ("net/http only in server", "no os.Getenv", "concurrency only in listed packages"). Verdict: checkable but irrelevant — nitpicking, obvious, overfitted to the current tree; a real codebase generates unbounded rules of that class. Finding recorded for the audit: **the value filter precedes checkability — the metric that matters is cared-about ∩ expressible**, not expressible alone. Also rejected specifically: confining goroutines by location — goroutines may live anywhere if they follow structure; structure, not placement, is the doctrine.

## Corpus v2 — damage rules

Each entry: anchor sketch, statement, the damage its violation causes, current state (only what was actually verified; otherwise "not measured").

---

**C-01** — anchor: writes to durable state files (JSON registries, proposals, votes, baselines, sessions)
Durable state is written atomically — temp file + rename in the same directory — never in-place truncation.
*Damage:* a torn write corrupts a registry every consumer trusts; recovery is manual.
*Current state:* temp+rename pattern present in 10 packages; no inverse sweep for in-place writers.

**C-02** — anchor: durable state files ↔ writing packages
Every durable file has exactly one owning package that writes it; all other packages mutate through the owner.
*Damage:* two writers race and clobber each other's updates without any error surfacing.
*Current state:* believed to hold (`rules.json` via `internal/contextdocs`, votes via `internal/proposals`); not swept.

**C-03** — anchor: child-process call sites
Every child process runs under a context deadline (`internal/shell.Run` or equivalent); no unbounded `Wait`.
*Damage:* one hung child (git, launchctl, a sync script) wedges the server or a routine forever.
*Current state:* `shell.Run` enforces a timeout; raw `os/exec` sites (`routines/launchctl.go`, `secretscan/history.go`, `cmd/configserver/main.go`) not verified for deadlines.

**C-04** — anchor: goroutine spawn sites
Every spawned goroutine has an owner that observes its completion or failure (WaitGroup/errgroup/join) before the surrounding operation reports success; no fire-and-forget.
*Damage:* partial work reported as success; goroutines leak under repeated requests until the process dies.
*Current state:* not measured.

**C-05** — anchor: switches over closed domain state sets (skill state, proposal status, session dimension, …)
A switch over a domain state set names every member; no `default` that silently swallows new members.
*Damage:* adding a member turns into silent misbehavior instead of a loud failure at the switch.
*Current state:* such switches exist (`internal/server/skills.go:217`, `internal/server/sessions.go:309`); exhaustiveness not measured.

**C-06** — anchor: durable state transitions
State transitions fire on explicit events, never on clocks — no timer- or elapsed-time-driven completion, rollback, or expiry of persisted state.
*Damage:* clock transitions mask lost events and complete flows that never happened.
*Current state:* not measured.

**C-07** — anchor: registry lookups by identity (session ids, proposal ids, repo names)
Identity lookups are exact-key matches; no fuzzy, prefix, or case-insensitive resolution of identities.
*Damage:* a near-match resolves to the wrong entity — writes land on the wrong record, unrecoverably.
*Current state:* believed to hold; short-id display exists but is display-only (computed, never used for lookup).

**C-08** — anchor: exported functions returning errors across package boundaries
Errors crossing a package boundary are wrapped with `%w` plus the failing operation and subject (path, id).
*Damage:* the edge logs a bare "no such file" with no path back to which operation on which object failed.
*Current state:* mixed; not measured.

**C-09** — anchor: shared in-memory registries (`internal/repos`, `internal/sessions`)
Shared registries are accessed only through the owning type's methods under its mutex; no state escapes to callers by reference.
*Damage:* data race — corrupt map or crash; `-race` in `make audit` fails intermittently, eroding gate trust.
*Current state:* mutexes exist in the owning types; escape discipline not swept.

**C-10** — anchor: accessor/getter functions on loaded state
Getters return already-loaded state; disk reads happen at ingestion or explicit refresh points — no lazy-load fallback chains, no hidden I/O in accessors.
*Damage:* render paths become failure paths; state mid-request is non-reproducible; double fallbacks mask missing data.
*Current state:* not measured.
