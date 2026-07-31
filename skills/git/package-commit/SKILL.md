---
name: package-commit
description: Commit changed files grouped by package, validated first. Trigger on /package-commit or "commit per package" or when multiple packages need separate validated commits. Args — file: one commit per changed file; trust: skip validation, stated in the result.
author: Kevin Horst
version: 1.8
allowed-tools: Bash(go build *), Bash(go test *), Bash(git diff *), Bash(git add *), Bash(git commit -m *), Bash(git log *), Read
---

# Package Commit

Commit changed files grouped by Go package. Validate first.

## When to use

**Use when:** local changes span multiple packages (or files) and need to be committed separately with per-package validation (build + test before each commit).
**Don't use when:** committing a single coherent change to one package — a normal `git commit` suffices. The code does not build or tests fail — fix first, this skill aborts on failure.
**Preconditions:** `go build ./...` and `go test ./...` pass (for Go projects); changes staged or unstaged.
**Workflow position:** terminal step after implementation — /fdesign and /coverage-increase hand off here (see `docs/skill-map.md`, smine repo).

## Args

- `file`: each changed file is its own commit. Grouping step degenerates to one file per group; message prefix stays the dot-notation path of the file's directory. Validation, ordering, and rules unchanged.
- `trust`: skip step 1 entirely. For when validation already ran this session or changes have no build surface (docs-only). State "validation skipped (trust)" in the result — never skip silently.

## 1. Validate (abort on any fail, skip for non-go)
- `go build ./...` — fail: show error, stop.
- `go test ./...` — fail: show error, stop.
  No commit if either fails.

## 2. Identify changed files
- `git diff --name-only`
- `git diff --cached --name-only`
- Union both lists.

## 3. Group
- Go files: by package dir, dot-notation.
  `util/reporting/interactive/queries` → `util.reporting.interactive.queries`
  - In dependency order
- Non-Go files: by logical owner.
  - `sql/reporting/*` → `util.reporting`
  - `docs/runbooks/*` → `docs/runbooks`
  - root Makefile → `build`
- `*.gen.go`: skip (not version controlled).

## 4. Commit (per group, logical order)
- `git add <files>`
- Message: `<package.path>: <description>`
  Format: dot-notation path, colon, present-tense, concise, no trailing period, no rule IDs.

Examples:
  - docs.runbooks: added deployment.md

## 5. Show result
- `git log --oneline -10`

## Rules
- build/test fail → show error, no commit.
- Never combine unrelated packages.
- No push (local only).
- No "Co-authored by Claude".
- Skip `*.gen.go`.

## Model

- Suggested: mid-tier / low
- Reason: procedural grouping + build/test/commit loop
- Tested unviable: delegated run (sonnet runner, 2026-07-24) — runner round-trip far slower than inline; not delegatable

## Changelog

- v1.8 (2026-07-31): activity-scoped context — commit style guide moved to the pack (style/commits.md)
- v1.7 (2026-07-30): context redesign — commit style guide moved to this skill's assets/commits.md
- v1.6 (2026-07-30): allowed-tools permission manifest declared
- v1.5 (2026-07-30): renamed per-package-commit → package-commit; behavior unchanged
- v1.4 (2026-07-27): moved under skills/git/ group; name and behavior unchanged
- v1.3 (2026-07-24): delegation removed — not delegatable (tested: delegated run far slower than inline); step 0 and inline arg deleted
- v1.2 (2026-07-22): delegation intake (self-delegating to sonnet), inline arg, classification unattended-safe, effort low
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-06-22): initial version
