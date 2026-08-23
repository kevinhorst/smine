---
name: coverage-increase
description: Raise test coverage on existing code — gap brief for approval, then the approved tests. Trigger on /coverage-increase or "add test coverage" or "backfill tests". Args — --nightly: unattended worktree-branch mode, no approval gates; n: nightly scope cap, top-N gaps (default 5).
author: Kevin Horst
version: 1.16
argument-hint: "[--nightly] [n]"
acdsl-context: ACTION-IMPL-*, ACTION-REVIEW-TEST-*, RULE-GOLANG-TEST-*, FACT-*
---

# Coverage Increase

This skill runs ground → gate → write in one session: measure coverage, present the coverage brief for approval, then write the approved tests. Nothing is written before the brief is approved, and product code is never modified — tests only.

Static constraints are not restated here: `AGENTS.md`, the per-language style guides (test rules live in each language's TESTS section, e.g. `$AGENT_CONTEXT_DIR_DEFAULT/rules/go.md`), and the doctrine entries under `$AGENT_CONTEXT_DIR_DEFAULT/actions/` govern every test written. They arrive via this skill's frontmatter `acdsl-context:` declaration, injected at invocation by the skill-context hook; non-Go language style guides are read from `$AGENT_CONTEXT_DIR_DEFAULT/rules/` on demand. Re-read entries from `$AGENT_CONTEXT_DIR_DEFAULT/` only when they are no longer in context. The checkable slice of these constraints is enforced by ACDSL gates (`go run ./cmd/acdsl check`, projection blocks on governed files) — for anything a gate covers, the gate is authoritative, not the prose.

## When to use

**Use when:** raising test coverage on existing, already-shipped code — "add test coverage", "backfill tests", "test this package".
**Don't use when:** tests are part of a newly designed feature — the Tests section of /fdesign handles that. Code cannot be tested without restructuring (no seam) — the brief routes those items to /fdesign change. Full-stack e2e verification against a running system — /dev-stack. Reviewing existing tests for quality — /railroad-review or /code-review.
**Preconditions:** the target package(s) build and their existing tests pass (green baseline — stop condition #5).
**Workflow position:** standalone — hands off to /package-commit for committing (see README.md § Skill map, smine repo).

## Args

`target`, `goal`, and `caveman` are covered in Phase 0. The nightly mode adds two:

- `--nightly`: unattended mode (headless `claude -p` or the coverage-increaser routine), always on a dedicated worktree branch (e.g. `coverage/nightly-YYYY-MM-DD`). Phases 0–2 run exactly as in the default mode. Phase 3 drops the ExitPlanMode gate and writes the brief to the branch as `coverage-brief.md`; Phase 4 writes the top-N gaps; Phase 5 writes the morning-review report and commits on the branch. Non-interactive: any situation that would ask the user (ambiguous target, needs-refactor code, missing tooling) is recorded in the report and skipped — the run never parks on a prompt. The default (interactive) mode is unchanged.
- `n`: nightly scope cap — the top-N ranked gaps written per run (default 5). A nightly run must terminate, not chase 100% coverage; the cap is what guarantees it. Ignored outside `--nightly`.

## Phase 0 — Intake

- **Args.** `target` — the package(s)/path(s) to raise coverage on, taken from the invocation. If absent in a multi-package repo, ask one line — never default to repo-wide. `goal` — optional coverage number or scope; when absent, the brief proposes one grounded in the measurement. `caveman` — compresses brief prose in the style of the `caveman` skill (fdesign semantics); if `~/.claude/skills/caveman` is missing, STOP — never approximate the style.
- **Context-doc check.** Verify the language style guides under `$AGENT_CONTEXT_DIR_DEFAULT/rules/` and the `actions/` chapters exist (or their entries already arrived via this skill's context declaration). If any is missing: state it once, continue with built-in baselines, never stall.
- **Style authority.** Resolve the target's language(s) to the applicable test style guide. If no guide exists for a language, the nearest sibling test file is the only shape authority — say so in the brief.
- Measurement runs on the current working tree.

## Phase 1 — Ground

Produce evidence, not opinions.

1. **Measure** — resolve the project's own coverage tooling first: a Makefile coverage target, else the stack default (`go test -coverprofile`, `pytest --cov`, `--coverage`). Record the exact command in the brief; the report's delta must come from the same command. Numbers come from a real profile run, never estimated.
2. **Rank** — exported/public behavior first, then risk (the `ACTION-CONCEPT-HOT-*` classes in `$AGENT_CONTEXT_DIR_DEFAULT/actions/concepting.md`, baseline + overlay, are the risk list), then branch complexity; recent churn breaks ties. De-rank trivial accessors, `main()` wiring, and generated code (per repo convention, e.g. `*.gen.go`).
3. **Exemplar** — name the nearest sibling test file per target, same package first. If no test file exists anywhere, flag "first test in this project" — the style guide alone defines shape.
4. **Testability triage** — code that cannot be tested without restructuring (hard-wired globals, no seam for clock/network/fs) is marked needs-refactor and routed to /fdesign change in the brief's Not tested section — never restructured inline.

## Phase 2 — Decide

- Present the full gap menu ranked by value with a recommended cut; the user cuts scope at the gate. Never pre-shrink the menu yourself.
- The goal is grounded in the measurement — per target or overall, qualitative where a percentage misleads. 100% is never the default.
- **Strategy, not case names.** The brief names the approach and goal per target (e.g. table-driven over the parser's error branches) — never enumerated test-case names. Cases emerge while writing. (Origin: an over-specified test plan enumerating per-case names died unimplemented; the thin successor won.)

## Phase 3 — Brief & gate

Deliver the brief through `ExitPlanMode` — it persists under `~/.claude/plans` and displays in the desktop UI. Never write a plans/ file in the target repo. Phases 0–3 are read-only; the write phase starts only on approval. If the user cuts targets or moves the goal, update the brief and re-present; the approved brief is binding.

**`--nightly`: no gate.** The brief is not presented for approval — it is written to the branch as `coverage-brief.md` and the write phase proceeds immediately. The approval judgment is deferred, not deleted: it moves wholly to the morning review of the branch, where nothing merges without it. Phases 0–2 stay read-only as usual; the only change is that `coverage-brief.md` on the branch replaces the ExitPlanMode call.

```markdown
# <target> — Coverage Brief

## Baseline
<exact coverage command; table: Target | Coverage — from the profile run, never estimated>

## Targets
<table: Target | Current | Why it ranks | Strategy | Exemplar — ranked, recommended cut applied;
one "Below the line:" bullet naming what was de-ranked (trivial accessors, generated code, main wiring)>

## Goal
<the number/scope the write phase is measured against — per target or overall>

## Not tested
<X, because Y — needs-refactor items routed to /fdesign change; integration-only paths with their infra need; generated code>

## Decisions needed
<one line each: missing framework/dependency, integration infra, goal trade-offs — or N/A>
```

## Phase 4 — Write the tests

Per approved target, in brief order (highest value first):

1. Read the exemplar and mirror its exact structure — file layout, naming, helper shape, test skeleton. Shape rules come from the language's test style guide, never from memory.
2. Write tests implementing the approved strategy. Every test asserts behavior — a test that only executes lines to move the number is a coverage lie.
3. Run targeted tests first (`go test ./path -run TestName` analog per stack), then the package suite — green before the next target.
4. Formatter/imports pass per stack (gofmt/goimports analog) before leaving the target — each package stays independently committable.
5. Re-measure the package; note the delta.

**A test that fails against actual behavior:** check intent once (docs, callers, sibling tests). Genuinely wrong → do not pin it with an assertion, do not fix product code — drop the test, record the bug as a finding, continue with the next target.

**`--nightly` scope & green rule:** there is no approved cut — write tests for at most the top-N ranked gaps (`n`, default 5), in rank order, so the run terminates. The product-code-never-modified invariant is unchanged. Every written test must pass before commit; a gap whose tests can't be made green is reported as skipped with the failure and never committed red.

## Phase 5 — Report

- Delta table `Target | Before | After | Goal`, produced with the Baseline command.
- Tests added: file → behavior covered, one strategy-level line each.
- Findings: bugs → /diagnose-debug, needs-refactor → /fdesign change, flakes observed.
- Remains uncovered: the updated Not-tested list.
- Committing is not this skill's job — hand off to /package-commit.

**`--nightly`:** the report is the morning-review artifact, written to the branch. It carries: the brief, per-gap outcome (written / skipped + reason), coverage before/after, and the test-run result. Here committing *is* the run's job — commit on the branch following /package-commit conventions, then terminate. The run must never park on an interactive prompt; anything that would prompt is recorded as skipped instead.

## Self-check gate

Before presenting the brief:

- [ ] Every Baseline number comes from an actual profile run; the exact command is recorded in the brief.
- [ ] Every target row has a strategy and a named exemplar (or a flagged "first test in this project").
- [ ] No test-case names anywhere in the brief — strategies and goals only.
- [ ] Every exclusion has its because; every needs-refactor item is routed, not dropped.
- [ ] Integration-shaped targets carry their infra need as a decision.

Before reporting done:

- [ ] `git status` shows only test files and fixtures — zero product-code edits, zero unapproved dependency changes.
- [ ] Full suite green via the project's test target; formatter/imports pass run.
- [ ] Every written test asserts behavior; none exists only to move the number.
- [ ] Coverage re-measured with the Baseline command; delta reported against the goal, shortfall explained.
- [ ] No suspected bug pinned by an assertion or fixed inline; each is a finding.
- [ ] `--nightly`: brief written to the branch as `coverage-brief.md` (no ExitPlanMode call); at most top-N gaps written; morning-review report + commit on the branch; no interactive prompt hit.

## Stop conditions

The `ACTION-IMPL-*` gate entries from `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md` apply (ACTION-IMPL-003: missing prerequisite → ask, never start infrastructure yourself). In `--nightly` mode every stop below that would ask the user (ambiguous target, needs-refactor code, missing tooling, integration infra) is instead recorded in the morning-review report and skipped — the run never parks on a prompt. Coverage-specific:

1. A test reveals product behavior that looks genuinely wrong → the bug protocol: a finding, never an assertion that pins it, never an inline fix.
2. A target cannot be tested without modifying product code (seam, export, signature) → exclusion routed to /fdesign change at the gate; discovered mid-write → finding + skip.
3. Test framework or coverage tooling missing → a Decisions-needed row at the gate; never installed silently.
4. The goal needs integration infra that is not running → stop, report the shortfall, point at /dev-stack; never fake it with mocks that assert nothing.
5. Pre-existing failures or flakes in the target packages → stop before writing anything; the baseline must be green or the delta is meaningless.

## Model

- Suggested: frontier / high
- Reason: gap analysis + writing style-conformant tests
- Tested unviable: — (none yet)
