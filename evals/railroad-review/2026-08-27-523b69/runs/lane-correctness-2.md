# Lane correctness-2 — findings

Diff: `72f50e5..523b699`. Direction: **correctness** (change does exactly what the spec requires — nothing missing, nothing extra). Spec source: the per-skill `changelog.json` entries + `docs/workflows.md` (the authoritative workflow contract). Data artifacts (`sessions/**`, `evals/**`, `proposals/**`) are non-code and out of scope for correctness; the review covers the code-bearing files (workflow `.js`, `SKILL.md`, changelogs).

The four new/changed workflow scripts themselves are internally correct and match `docs/workflows.md`: `pipeline(items, ...stages)` (2-arg form in foreign-toolchain is valid — single stage), `parallel(thunks)`, the `tier = {...(model && {model}), ...(effort && {effort})}` spread, and the drift stage all check out. The findings are wiring/documentation gaps between the shipped workflows and their fronting skills.

| ID | Severity | Location | Finding | Proposed fix |
|----|----------|----------|---------|--------------|
| correctness-MINOR-1 | MINOR | skills/feature/fdesign/SKILL.md:60 | `subsystem-grounding.js` (v3.6) is bundled but fdesign SKILL.md never invokes it — no Workflow-tool/scriptPath/manual-run reference; unlike session-mine (wired at smine SKILL.md:45). /fdesign change spanning 2+ subsystems never runs the promised deterministic grounding. | Add a Phase-1 manual-run bullet invoking `Workflow({scriptPath:'.../subsystem-grounding.js', args:{subsystems,drivers,planRef}})`. |
| correctness-MINOR-2 | MINOR | skills/feature/fimplement/SKILL.md:62 | `config-ui-fidelity-gate.js` (v1.23) is bundled but fimplement never invokes it — the Phase-3 diff added only the sync-exclude and host:port bullets, no UI-gate call. A config-server template/CSS change is committed without the promised render/single-Save-button gate ever firing. | Add a pre-first-commit bullet invoking `Workflow({scriptPath:'.../config-ui-fidelity-gate.js', args:{templates,cssPath,renderHint}})`, gating on `gate.pass`. |
| correctness-MINOR-3 | MINOR | skills/git/package-commit/SKILL.md:20 | `foreign-toolchain-pretag.js` (v3.3) is bundled but package-commit's diff only bumps the version — no body step invokes it. A release with an Inno Setup / GOOS artifact tags without the promised local pre-compile; CI on the pushed tag stays the first compile. | Add a release-tag step invoking `Workflow({scriptPath:'.../foreign-toolchain-pretag.js', args:{artifacts,tagRef}})`, refusing to tag on `gate.pass=false`. |
| correctness-INFO-1 | INFO | skills/smine/smine/SKILL.md:45 | `session-mine.js` v1.13 added optional `model`/`effort` args, but the manual-run arg list at SKILL.md:45 still omits them, so a hand-invoking user can't discover the tier override. No runtime defect (both optional). | Append `model, effort` to the args object listed at SKILL.md:45. |

## Definition of Done (correctness lens)

| Criterion | Status | Notes |
|-----------|--------|-------|
| New workflows match their documented contract (`docs/workflows.md`) | PASS | args validation, stages, returns all match. |
| New workflows are invoked by their fronting skill | FAIL | subsystem-grounding, config-ui-fidelity-gate, foreign-toolchain-pretag are inert (MINOR-1/2/3). |
| Changelog/doc surfaces match the shipped args | PARTIAL | smine SKILL.md:45 manual-run omits model/effort (INFO-1); docs/workflows.md is current. |
| No behavioral change nobody asked for | PASS | changes are additive; renumbered fimplement changelog stays version- and date-descending. |

### Recommendation
**CONDITIONAL** — the three bundled gate/grounding workflows are correct but unwired into their fronting skills, so the behavior their changelogs promise never fires; wire them (or confirm wiring is a separate follow-up) before relying on them.

Funnel: 4 produced / 0 confirmed (lane first-confirmation only) / 0 unverified / 0 duplicates / 0 rejected / 0 debunked.
