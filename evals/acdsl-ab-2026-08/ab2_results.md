# ACDSL A/B round 2 — anti-prior rule (new(expr)), no gravity well (2026-08-06)

> Probe: Haiku 4.5 (pre-new(expr) cutoff). Rule: ACDSL-PTR-001 — pointers to composites
> via Go 1.26 new(expr), &T{...} forbidden; evalscript forbid-addr-lit; anchor ^internal/xp
> (experiment tree, inert for real code). n=3 per arm, 12 primary runs + 3 retries.

## Results

| Arm | Channel | Implementation | First-pass gate | Retries to green |
|---|---|---|---|---|
| create-blind ×3 | none | 3× &Config{ | 3× RED | 1, 1, 1 — all green after one mechanical retry |
| create-plan ×3 | plan-carried rules | 3× new(Config{ | 3× green | — |
| edit-off ×3 | seed read, no projection | 3× &Config{ | 3× RED (not retried; loop already measured) | — |
| edit-on ×3 | projection on seed read | 3× new(Config{ | 3× green | — |

**Differential achieved: 0/6 first-pass conformance without a rule channel; 6/6 with one (plan-carried or projection). The channels work, and their absence is now measurable.**

## Findings

1. **The prior is real and uniform.** Every channel-less Haiku run produced `&T{...}` — including three discarded pilot runs (see limits). Nine of nine blind generations chose the pre-1.26 idiom.
2. **Projection flips the create-adjacent case completely.** Identical prompts, identical seed file; the only difference between edit-off (0/3) and edit-on (3/3) is the hook writing three comment lines into seed.go before the probe's read. This is the per-rule context-attribution measurement the framework doc wanted — on a closed model, no logprobs, via the kill switch.
3. **The retry loop works as designed, first time exercised.** The retry prompt was assembled mechanically from check -json fields (message + ID + why). All three reds went green in exactly one retry; no probe fought new(expr) as invalid syntax — the anticipated "model corrects the syntax away" failure mode did not occur, and all outputs compile under go 1.26.4.
4. **Why-text as remedy carries the fix.** Both the plan block and the retry prompt contained new(T{...}) verbatim in the why; probes applied it directly. Rules whose why names the remedy convert reds to greens in one hop.
5. **Leak behavior observed live, boundary held.** All three edit-on probes copied the projection block from seed.go into their newly created file — weak models treat the projected view as file content to imitate. The gate stayed green (worktree blocks are legal); the staged guard + strip is exactly where this gets caught. Corroborating signal: the harness's own security monitor flagged one probe's copied block as suspected instruction injection — a false positive against our own rig, but evidence that copied projection blocks are a real hygiene concern. Mitigation candidates: stronger do-not-copy phrasing in the block, or the per-symbol placement evolution reducing block salience.
6. **Weak-model worktree escape (operational, not ACDSL).** The first three probes ignored "work in <worktree path>" and wrote into the main checkout; prompts needed an explicit path-substring constraint. Any harness driving weak models against a worktree gate needs hard path fencing.

## Limits

- n=3 per arm, one rule, one task shape, one weak model.
- Cross-arm contamination in create-plan: probes could read earlier arms' xp outputs still on disk (xp5/xp6 read them; xp7 read none — its first-pass green is the clean plan-arm evidence). Blind and edit arms unaffected. Future runs: full xp cleanup between arms, always.
- Three discarded pilot runs (worktree escape) are counted only as prior-classification corroboration.
- Retry measured only from the blind arm; edit-off reds were not retried (loop already characterized).

## Verdict

Round 1's ceiling plus round 2's differential together give the full picture: when the repo's own structure carries the doctrine (gravity well, declaration site), channels are redundant; when it doesn't — anti-prior rules, novel idioms, no helper to imitate — the projection and plan channels are worth ~100 percentage points of first-pass conformance on a weak model, and the gate+retry loop converts the remainder at one retry per violation. The mechanism's value concentrates exactly where doctrine diverges from what models already believe — which is the only place doctrine is worth writing down.
