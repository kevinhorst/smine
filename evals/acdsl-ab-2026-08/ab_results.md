# ACDSL A/B — context channels vs first-pass conformance (2026-08-06)

> Directional probe run, n=2 per arm, one repo, one task class, Fable-class probe agents.
> Question: does the agent need the projection hook (or plan-carried rules) to produce
> rule-conformant output, per path (edit-adjacent vs brand-new package)?

## Setup

- Task E: add `internal/evals/gitstatus.go` (existing governed package, no in-package exec exemplar) — function running `git status --short`.
- Task C: create `internal/probecN/runner.go` (brand-new package, no siblings) — function running `ls`.
- Arms: E-on / C-on (PreToolUse projection hook live) · E-off / C-off (kill switch) · C-plan (hook off, prompt carries `project -plan` output).
- Prompts neutral: no rule mention (except C-plan's rule block), repo tooling forbidden.
- Metrics: exec.Command (violation) vs shell.Run/CommandContext; first-pass `acdsl check`; files the probe reported reading.

## Results

| Run | Implementation | First-pass gate | Read shell.go |
|---|---|---|---|
| E-on-1 | shell.Run | green | yes |
| E-on-2 | shell.Run | green | yes |
| C-on-1 | shell.Run | green | yes |
| C-on-2 | shell.Run | green | yes |
| E-off-1 | shell.Run | green | yes — cited ACDSL-EXEC-001 unprompted |
| E-off-2 | shell.Run | green | yes |
| C-off-1 | shell.Run | green | yes |
| C-off-2 | shell.Run | green | yes |
| C-plan-1 | shell.Run | green | yes |
| C-plan-2 | shell.Run | green | yes — also read launchctl.go |

10/10 conformant, 10/10 first-pass green, zero variance across arms.

## Findings

1. **Ceiling effect: the channels stack, and the strongest one wasn't the hook.** Every probe in every arm navigated to `internal/shell/shell.go` — the natural home of the subprocess mechanism — and found both the helper and (in off arms) the committed rule marker itself. Conformance was delivered by repo structure + declaration-site visibility + convention-following before the projection could add anything measurable.
2. **The declaration site is a context channel of its own.** E-off-1 cited ACDSL-EXEC-001 by ID with the hook off — it read the marker line in shell.go. "Hook off" is not "rule-blind"; any experiment wanting a rule-blind arm must control for where markers are declared.
3. **The rule's why-text steers navigation.** C-plan-2, given the rule text naming `internal/shell.Run`, went and read exactly that file plus a call-site example. Plan-carried rules act as navigation hints, not just constraints — an argument for why-texts naming their remedy.
4. **Two design bugs found by running, not by reading:**
   - Projection blocks contain rule why-texts; STATE-001's why names `rules.json`, so projected files tripped the text-class literal-owner evalscript. Fix (shipped): text-class evalscripts clean their own input — strip the projection block in memory, never on disk; violation line numbers stay in disk coordinates. AST-class evalscripts were never affected (comments invisible).
   - The experiment itself needed the gate to be projection-aware before any arm could be measured — the interaction was invisible until real projected files met a real check run.

## Honest limits

- n=2 per arm; one repo; one task class; strong probe model; the task ("run a command") cues navigation toward the governed mechanism's home package. No differential signal is obtainable at a ceiling.
- The red-retry path was never exercised — no probe violated, so retry economics (the thing projection is hypothesized to buy) remains unmeasured.

## What would produce signal next

- A rule whose remedy has **no gravity well** — no obvious helper package the task naturally leads to (e.g. an ordering/structure rule, or a literal-owner rule about a path agents don't visit). Convention-following can't rescue those; channels separate.
- A weaker probe model — ceiling likely drops, differential appears.
- Marker declarations moved away from the mechanism's home (or a control arm with shell.go made unreadable) — isolates declaration-site leakage.
- A violating baseline task — measures the retry loop and generated-prompt quality, not just first-pass rate.

## Verdict for the flow design

The full flow held under live multi-agent load — projection on read, view == disk, gate green with projections present (after the clean-input fix), strip/restore byte-exact, plan-time resolution working as a prompt block. The hook's *marginal* value over this repo's existing channels is unproven at this task class — which is itself the pack-level-A/B methodology working: a channel whose removal changes nothing at the gate is, for that slice, dead weight. Keep the hook (its cost is near zero and the trace benefit stands), but the next evidence run should target a no-gravity-well rule before investing in finer-grained delivery.
