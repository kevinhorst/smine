# Lane code-style-2 — Findings

Direction: code-style · Lane 2 · Aborted: no · Range: 72f50e5..523b69

Context walked: `ACTION-REVIEW-QUALITY-*` (context/actions/reviewing.md), repo JS-workflow house pattern (established from existing `skills/*/*/workflows/*.js`), `context/facts/claude-configs.md`.

Reviewed code surface: the four workflow JS files (three new: `subsystem-grounding.js`, `config-ui-fidelity-gate.js`, `foreign-toolchain-pretag.js`; one modified: `session-mine.js`) and `cmd/worktrees/_lib/verdict.sh` (change = an injected ACDSL-projection header only, no logic change). The remaining diff (session-analysis `.md`/`.json`/`.txt`, `evals/fexplore-*`, `proposals/*.json`, `SKILL.md`/`changelog.json`, `docs/workflows.md`) is generated / mined / doc artifacts — low risk tier, read only for scope creep; none found.

| ID | Severity | Route | Location | Finding | Proposed fix |
|----|----------|-------|----------|---------|--------------|
| code-style-NIT-1 | NIT | auto-fix | skills/feature/fimplement/workflows/config-ui-fidelity-gate.js:73 | `phase('Gate')` sits inside the pipeline transform callback after an early return; every other workflow (incl. sibling foreign-toolchain-pretag.js:58, same Build/Gate shape) declares each `phase()` once at top level. Render-failed templates skip the Gate marker and the phase flip-flops per concurrent item. | Move `phase('Gate')` to top level after the pipeline, or split Render/Gate into two sequential passes. |
| code-style-NIT-2 | NIT | auto-fix | skills/git/package-commit/workflows/foreign-toolchain-pretag.js:66 | JS-side const / return key `blocking_errors` is snake_case, breaking the repo's camelCase convention for JS identifiers (snake_case is reserved for agent-output schema fields). Direct sibling config-ui-fidelity-gate.js returns the same concept as `gate.violations`, so the two gate workflows diverge. | Rename to camelCase and align with the sibling: `gate: { pass, violations }` (or `blockingErrors`). |

Funnel: 2 claims produced / 2 survived confirmation / 0 unverified / 0 duplicates / 0 rejected / 0 debunked.
