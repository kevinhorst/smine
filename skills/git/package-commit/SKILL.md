---
name: package-commit
description: Commit changed files grouped by package, validated first. Trigger on /package-commit or "commit per package" or when multiple packages need separate validated commits. Args — file: one commit per changed file; trust: skip validation, stated in the result.
author: Kevin Horst
version: 3.3
argument-hint: "[file] [trust]"
allowed-tools: Bash(make build *), Bash(make test *), Bash(go build *), Bash(go test *), Bash(go run ./cmd/acdsl *), Bash(git diff *), Bash(git add *), Bash(git commit -m *), Bash(git log *), Read
acdsl-context: RULE-COMMIT-*
---

# Package Commit

Commit changed files grouped by package (directory). Validate first.

## When to use

**Use when:** local changes span multiple packages (or files) and need to be committed separately with per-package validation (build + test before each commit).
**Don't use when:** committing a single coherent change to one package — a normal `git commit` suffices. The code does not build or tests fail — fix first, this skill aborts on failure.
**Preconditions:** the project's build and tests pass (when it has a build surface); changes staged or unstaged.
**Workflow position:** terminal step after implementation — /fdesign and /coverage-increase hand off here (see README.md § Skill map, smine repo).

## Args

- `file`: each changed file is its own commit. Grouping step degenerates to one file per group; message prefix stays the dot-notation path of the file's directory. Validation, ordering, and rules unchanged.
- `trust`: skip step 1 entirely. For when validation already ran this session or changes have no build surface (docs-only). State "validation skipped (trust)" in the result — never skip silently.

## 1. Validate (abort on any fail, skip when no build surface)

**SKILL-PACKAGECOMMIT-VALIDATE-001** `[step]` — Resolve the project's validation mechanism, in order: Makefile `build` / `test` targets → the stack's own commands (`go build ./...` + `go test ./...`, `npm run build` + `npm test`, `pytest`, …) → none (docs-only: state it, continue).

**SKILL-PACKAGECOMMIT-VALIDATE-002** `[gate]` — Run build, then tests — any failure: show the error, stop. No commit on red.

## 1b. Strip ACDSL projections (repos with acdsl/registry.json only)

**SKILL-PACKAGECOMMIT-STRIP-001** `[step]` — `go run ./cmd/acdsl project -strip` (use `./bin/acdsl` when the repo vendors the binary) — removes working-copy projection blocks before anything is staged.
* Why: the check gate refuses staged blocks, this step prevents the red instead of reacting to it.

**SKILL-PACKAGECOMMIT-STRIP-002** `[step]` — Skip silently when the repo has no `acdsl/registry.json`.

Note: stripping may empty a file's diff (the only change was the projection); such files drop out of step 2 naturally.

## 2. Identify changed files

**SKILL-PACKAGECOMMIT-IDENT-001** `[step]` — `git diff --name-only`

**SKILL-PACKAGECOMMIT-IDENT-002** `[step]` — `git diff --cached --name-only`

**SKILL-PACKAGECOMMIT-IDENT-003** `[step]` — Union both lists.

## 3. Group

**SKILL-PACKAGECOMMIT-GROUP-001** `[step]` — Source files: by directory, dot-notation. `util/reporting/interactive/queries` → `util.reporting.interactive.queries`

**SKILL-PACKAGECOMMIT-GROUP-002** `[step]` — In dependency order where the stack defines one.

**SKILL-PACKAGECOMMIT-GROUP-003** `[step]` — Non-source files: by logical owner.

  - `sql/reporting/*` → `util.reporting`
  - `docs/runbooks/*` → `docs/runbooks`
  - root Makefile → `build`

**SKILL-PACKAGECOMMIT-GROUP-004** `[step]` — Generated files: skip (per repo convention, e.g. `*.gen.go`).

## 4. Commit (per group, logical order)

**SKILL-PACKAGECOMMIT-COMMIT-001** `[step]` — `git add <files>`

**SKILL-PACKAGECOMMIT-COMMIT-002** `[step]` — Message: `<package.path>: <description>` — format: dot-notation path, colon, present-tense, concise, no trailing period, no rule IDs.

Examples:
  - docs.runbooks: added deployment.md

## 5. Show result

**SKILL-PACKAGECOMMIT-RESULT-001** `[step]` — `git log --oneline -10`

## Rules

**SKILL-PACKAGECOMMIT-RULES-001** `[gate]` — build/test fail → show error, no commit.

**SKILL-PACKAGECOMMIT-RULES-002** `[review]` — Never combine unrelated packages.

**SKILL-PACKAGECOMMIT-RULES-003** `[gate]` — No push (local only).

**SKILL-PACKAGECOMMIT-RULES-004** `[gate]` — **Never amend commits unless explicitly asked.**
* Why: rewritten SHAs break the user's continuous cherry-picking out of worktrees; one commit per logical change.

**SKILL-PACKAGECOMMIT-RULES-005** `[gate]` — Commit messages carry **no** review-rule IDs, **no** Claude attribution, and **no** co-author trailer — ever. Dot-notation `package: message` across all languages. "no desc" / "no dsc" means the subject line only, no body.

**SKILL-PACKAGECOMMIT-RULES-006** `[review]` — Honor inline gate waivers as legitimate overrides of this skill's gates and grouping: "Ignore the test failure. commit", "skip validation, already done", "only one commit" override the validation gate and the per-package grouping respectively.

**SKILL-PACKAGECOMMIT-RULES-007** `[review]` — Skip generated files (per repo convention, e.g. `*.gen.go`).

## Model

- Suggested: mid-tier / low
- Reason: procedural grouping + build/test/commit loop
- Tested unviable: delegated run (sonnet runner, 2026-07-24) — runner round-trip far slower than inline; not delegatable
