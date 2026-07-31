---
name: merge-resolve
description: Merge two diverged git branches by resolving all conflicts once at final-tree level, verified by build, tests and parent diffs. Trigger on /merge-resolve or "merge main into my branch" or "these branches conflict / cherry-picks keep failing". Args — ours: branch to merge into (default current); theirs: branch to merge in.
author: Kevin Horst
version: 1.1
---

# Merge Resolve

Merge two diverged branches by resolving all conflicts once at final-tree level. Any conflicted integration ask — failed cherry-pick chain, mid-rebase mess, replay-cache branch — is converted into this one flow, never resolved in place.

## When to use

**Use when:** two branches conflict and one must absorb the other; a cherry-pick/replay/rebase attempt at the same integration has already failed or stalled.
**Don't use when:** resolving a single conflicted file the user already staged (just resolve it); planning a restructure → `/fchange`; auditing what a contract change broke → `/spec-drift`.
**Preconditions:** both refs exist locally; working tree clean or only abortable in-flight state; project has a build/test command.
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo); output feeds a normal merge/PR into the target branch.

## Args

- `ours`: branch that absorbs the merge (default: current branch)
- `theirs`: branch being merged in

## 1. Normalize to a merge (abort in-flight state first)

- `git cherry-pick --abort` / `git rebase --abort` / `git merge --abort` as applicable; `git status` must be clean.
- If the user is mid-way through a replay/cache branch: abandon it, state why (partial replays resolve against half-adapted context and accumulate drift). Never continue a commit-by-commit resolution.

## 2. Scope before touching anything

- `git merge-tree --write-tree <ours> <theirs>` → list of conflicted files. Report the count.
- Establish which side has what: `git log --oneline <ours>..<theirs>` and reverse; note deliberate renames/exports (commit messages, design docs in `plans/`) — these are binding during resolution.

## 3. Merge on a work branch

- `git checkout -b merge/<theirs-slug>-<ours-slug> <ours>` (or merge directly on `ours` if the user says so), then `git merge <theirs>`.

## 4. Resolve per file — union of both intents

- Ground truth is each side's *final* tree (`git show <ref>:<path>`), never intermediate diffs.
- Classify each hunk: additive-union (keep both) | rename collision (keep the side whose rename has external consumers or a design-doc rationale) | mechanism collision (two implementations of the same behavior — pick the designed one, delete the stopgap; commit messages saying "so X passes" mark stopgaps) | contradictory tests (auto-merge can splice assertions from both sides into one impossible test — rewrite to the surviving mechanism).
- Sweep for what compiles but is wrong: same-typed parameter reorders (alphabetized string params), resurrected dead code (fields/channels one side deleted), constructor arity changes in files only the other side added.

## 5. Verify (abort = unresolved finding, not a skipped check)

- No conflict markers left (`grep -rl '<<<<<<<'` over tracked source).
- Project build + vet + full test suite + formatter — all green.
- `git diff <theirs> --stat` and `git diff <ours> --stat`: every delta vs each parent must be attributable to the other parent or to a named resolution decision.
- `git merge-tree --write-tree <final-target> <work-branch>`: the eventual real merge must be clean.

## 6. Commit and report

- Commit with the default `MERGE_MSG`. Report: files resolved, each semantic judgment call made (especially deleted mechanisms and behavior deltas), and the verification results.

## Rules

- Never resolve by replaying commits; always at final trees.
- Every judgment call (deleted mechanism, behavior delta) is named explicitly in the result — silent semantic choices are forbidden.
- A test rewritten during resolution must still cover the surviving behavior, not be deleted.
- If both sides' intent for the same behavior genuinely contradicts and no commit/design evidence decides it: stop and ask.

## Model

- Suggested: frontier / medium
- Reason: semantic conflict classification and stopgap-vs-design judgment; mechanical phases are cheap but misjudged collisions ship silent regressions
- Tested unviable: — (none yet)

## Changelog

- v1.1 (2026-07-27): moved under skills/git/ group; name and behavior unchanged
- v1.0 (2026-07-26): Initial version — final-tree merge method distilled from the peek-mcp deep-analysis merge session.
