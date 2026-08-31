---
name: merge-resolve
description: Merge two diverged git branches by resolving all conflicts once at final-tree level, verified by build, tests and parent diffs. Trigger on /merge-resolve or "merge main into my branch" or "these branches conflict / cherry-picks keep failing". Args — ours: branch to merge into (default current); theirs: branch to merge in.
author: Kevin Horst
version: 1.6
argument-hint: "[ours] [theirs]"
allowed-tools: Bash(~/.claude/skills/merge-resolve/scripts/merge_branch.sh *), Bash(git merge *), Bash(git merge-tree *), Bash(git cherry-pick --abort), Bash(git rebase --abort), Bash(git commit --no-edit)
---

# Merge Resolve

Merge two diverged branches by resolving all conflicts once at final-tree level. Any conflicted integration ask — failed cherry-pick chain, mid-rebase mess, replay-cache branch — is converted into this one flow, never resolved in place.

## When to use

**Use when:** two branches conflict and one must absorb the other; a cherry-pick/replay/rebase attempt at the same integration has already failed or stalled.
**Don't use when:** resolving a single conflicted file the user already staged (just resolve it); planning a restructure → `/fdesign change`; auditing what a contract change broke → `/spec-drift`.
**Preconditions:** both refs exist locally; working tree clean or only abortable in-flight state; project has a build/test command.
**Workflow position:** standalone (see README.md § Skill map, smine repo); output feeds a normal merge/PR into the target branch.

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

- `~/.claude/skills/merge-resolve/scripts/merge_branch.sh create <ours> <theirs>` — creates and checks out the deterministic work branch `merge/<theirs-slug>-<ours-slug>` (or merge directly on `ours` if the user says so), then `git merge <theirs>`.
- Unattended invocations (routine runs) merge directly on `ours` — no work branch, and step 7's cleanup is skipped; HEAD must end on the branch the run started on.

## 4. Resolve per file — union of both intents

- Ground truth is each side's *final* tree (`git show <ref>:<path>`), never intermediate diffs.
- Classify each hunk: additive-union (keep both) | rename collision (keep the side whose rename has external consumers or a design-doc rationale) | mechanism collision (two implementations of the same behavior — pick the designed one, delete the stopgap; commit messages saying "so X passes" mark stopgaps) | contradictory tests (auto-merge can splice assertions from both sides into one impossible test — rewrite to the surviving mechanism).
- Sweep for what compiles but is wrong: same-typed parameter reorders (alphabetized string params), resurrected dead code (fields/channels one side deleted), constructor arity changes in files only the other side added.

## 5. Verify — mandatory checkpoint (abort = unresolved finding, not a skipped check)

This checkpoint runs in full even when the merge auto-resolved with zero conflicts — a clean auto-merge against the wrong base ref silently drops work, so none of these steps are skippable.

- **Assert the merge base ref is the exact ref the user named.** Resolve the ref actually merged and confirm it is what the user said: a bare `<theirs>`/`<ours>` means the local branch, never `origin/<theirs>`/`origin/<ours>`. Compare SHAs (`git rev-parse <named-ref>` against the merged ref); a stale `origin/<x>` used in place of the named local `<x>` is an unresolved finding — abort and re-merge against the named ref.
- No conflict markers left (`grep -rl '<<<<<<<'` over tracked source).
- Project build + vet + full test suite + formatter — all green.
- **Attribute every parent-diff delta before accepting the merge.** `git diff <theirs> --stat` and `git diff <ours> --stat`: every delta vs each parent must be attributable to the other parent or to a named resolution decision — an unattributable delta is dropped or spurious work, not a pass.
- `git merge-tree --write-tree <final-target> <work-branch>`: the eventual real merge must be clean.

## 6. Commit and report

- Commit with the default `MERGE_MSG`. Report: files resolved, each semantic judgment call made (especially deleted mechanisms and behavior deltas), and the verification results.

## 7. Cleanup

- Once the merge result has landed on `<ours>` (fast-forward or real merge — confirm with the user when the landing is theirs to do): `~/.claude/skills/merge-resolve/scripts/merge_branch.sh cleanup <ours>` — deletes only `merge/*` branches already merged into `<ours>` (`git branch -d`, never `-D`); anything unmerged is kept and reported, never forced.

## Rules

- Never resolve by replaying commits; always at final trees.
- Every judgment call (deleted mechanism, behavior delta) is named explicitly in the result — silent semantic choices are forbidden.
- A test rewritten during resolution must still cover the surviving behavior, not be deleted.
- If both sides' intent for the same behavior genuinely contradicts and no commit/design evidence decides it: stop and ask.

## Model

- Suggested: frontier / medium
- Reason: semantic conflict classification and stopgap-vs-design judgment; mechanical phases are cheap but misjudged collisions ship silent regressions
- Tested unviable: — (none yet)
