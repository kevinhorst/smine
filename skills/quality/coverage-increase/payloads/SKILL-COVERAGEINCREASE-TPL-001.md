# <target> — Coverage Brief

## TLDR
<what gets tested, the goal, what stays untested — at most five titled one-line bullets>

## Coverage baseline
<override: the exact coverage command, the profile run's commit and the exemplar test files, then the measured coverage per target from the profile run, never estimated — table: Target | Coverage; measurements are not a facts table>

## Scope
<In: ranked targets, each a bold title on its own bullet with sub-bullets — current coverage, why it ranks, strategy, exemplar — the recommended cut applied; Out: below-the-line items (trivial accessors, generated code, main wiring), needs-refactor and integration-only paths, each with the reason it is out>

## Decisions
<table: ID | Brief | Decision (brief) | Consequences | Adds | Why | Rejected | Problem [AGENT-ONLY] | Decision [AGENT-ONLY] | Facts [AGENT-ONLY] (per rules/plan.md) — the goal per target or overall (100% never the default), strategy choices; OPEN rows for a missing framework or dependency, integration infra, goal trade-offs>

## Open questions
<index of pointers to OPEN decision rows — empty at approval>

## Verification
<table: ID | Check | Command | Expected | Surface — the coverage command re-run per target with the goal as Expected, every new test executed with the repo's test command, a row asserting no behavior-free line-mover tests; Surface local unless a target needs the dev stack>
