# ACDSL Stage-2 Expressibility Audit — corpus v2

> Input: [corpus.md](corpus.md) v2 (10 damage rules, vetoed corpus). Bucketing per concept:
> expressible / evalscript too costly / judgment. Metric: cared-about ∩ expressible.

| Rule | Bucket | Evalscript / blocker | v0 |
|---|---|---|---|
| C-01 atomic temp+rename | expressible (rephrased) | forbid-call on os.WriteFile in owner packages; deferred — cmd/rules itself violates (main.go:164) | no |
| C-02 single writer per durable file | expressible | literal-owner (path-literal confinement approximates ownership) | YES — ACDSL-STATE-001 |
| C-03 child processes under deadline | expressible | forbid-call on exec.Command (context variant excluded syntactically) | YES — ACDSL-EXEC-001 |
| C-04 goroutine ownership | not expressible cheaply | needs flow/escape analysis — beyond deterministic-cheap checks | no |
| C-05 exhaustive domain switches | expressible, dep-gated | existing exhaustive linter as evalscript; blocked: no new deps in v0 | no |
| C-06 no clock-driven transitions | not expressible cheaply | "persisted state" is a semantic property; flow analysis territory | no |
| C-07 exact identity lookups | not expressible cheaply | "fuzzy matching on identity" is heuristic, violates determinism fence | no |
| C-08 boundary errors wrapped | expressible, dep-gated | wrapcheck as evalscript; blocked: no new deps in v0; noise risk | no |
| C-09 registry mutex escape | not expressible cheaply | escape analysis; go vet copylocks covers a sliver only | no |
| C-10 no hidden I/O in getters | not expressible cheaply | "getter" and "I/O reachability" are semantic; flow territory | no |

## Measurement

- Expressible: 5/10 (2 enforced in v0, 1 rephrased-deferred, 2 dep-gated).
- Not expressible under the determinism-and-cost fence: 5/10 — and these are the deepest-damage rules (goroutine ownership, clock transitions, hidden I/O).
- Reading: the mechanism works end-to-end, but the cared-about ∩ expressible fraction at v0 predicates is ~50%, concentrated in the shallower half. Raising it means either richer evalscripts (cost) or accepting the judgment pile keeps the deep rules (concept: quarantine, not eliminate). This ceiling is real and chosen — making the deep five expressible requires the analyzer-building explicitly refused by the anti-goals.
