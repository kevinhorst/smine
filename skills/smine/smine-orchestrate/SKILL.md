---
name: smine-orchestrate
description: Judge, merge, and deploy a nightly smine run — re-verify the stages' output, fix or reject, merge into main, sync deployed state and repos. Trigger on /smine-orchestrate <branch> as the smine-nightly orchestrate stage, or /smine-orchestrate bootstrap as the bootstrap wrapper's terminal stage. Args — branch: run branch to judge and merge; bootstrap: judge and commit the working tree (initial setup), optional (since: date) scope check.
author: Kevin Horst
version: 1.2
argument-hint: "<branch> | bootstrap [(since: YYYY-MM-DD)]"
allowed-tools: Read, Write, Edit, Bash(git *), Bash(jq *), Bash(mv *), Bash(make audit *), Bash(go run ./cmd/acdsl *), Bash(bash cmd/sync/*), ToolSearch
---

# smine: orchestrate a run

The gatekeeper of the automatic pipeline: everything the other stages produced is judged before it becomes the machine's state. Judgment lives here; the invoking wrapper (smine-nightly run.sh stage 3) verifies only mechanical postconditions — clean tree, branch merged-or-kept.

## When to use

**Use when:** a published nightly run branch needs judging, merging, and deploying (the smine-nightly orchestrate stage), or an initial-setup run needs finalizing (`bootstrap` mode, invoked as the last stage of `cmd/bootstrap/run.sh`).
**Don't use when:** producing or consolidating proposals — the dimension skills and /smine-consolidate. Applying votes — /smine-apply. A developer machine's manual review — merging locally IS the acceptance act there.
**Preconditions:** cwd is the main checkout on branch `main` with a clean tree; branch mode additionally needs the published run branch to exist.
**Workflow position:** smine pipeline: mine → consolidate → apply → publish → **smine-orchestrate** (merge, sync, reconcile).

## Args

- branch: positional — the dated run branch (`claude-routines/...`) to judge and merge.
- `bootstrap`: positional alternative — initial-setup mode: judge the working tree's uncommitted/committed pipeline output with initial-setup framing (lenient on seeding volume, strict on schema, audit, and style), commit directly on main, no branch and no merge; duties 3–7 below are identical. You are the initial setup agent: the machine is being seeded, large output volume is expected and acceptable.
- `(since: <YYYY-MM-DD>)`: bootstrap-mode suffix, passed by the wrapper when the run had a date floor — any mined session with `last_active` before the date is a finding (fix the batch or reject).

## Hard invariants

- A failed or hopeless run is **rejected, never half-merged**: no merge, branch kept, reason written to `.orchestrate-report` at the repo root (the wrapper surfaces and removes it).
- The checkout must be `main` and clean before merging; end every path with a clean tree.
- `additionalDirectories` is wrapper-owned: this skill verifies and reports, never writes the user settings file directly.
- Never push unless `git remote get-url origin` succeeds; never add a remote.

## 1. Judge

- Review `main..<branch>` (bootstrap: the working tree delta plus any commits this run made).
- Gates for acceptance:
  - sessions/proposals JSON schema-valid (`go run ./cmd/acdsl check`, `proposals/schema.json` via jq spot checks);
  - `make audit` green on the branch tree;
  - prose conforming to the installed presentation profile (`~/.claude/context/global/presentation-profile.md`): language and register uniform across every proposal; on a casual audience the user-visible fields (title's change-name, `change`, `fields[].label/text`, `evidence[].title`, `sessions[].note`) carry no file paths, no rule/FACT/ACDSL IDs, and no schema or taxonomy jargon — technical anchors belong in `target`/`anchor`/`code`/snippet fields;
  - **casual lockout** — when the profile audience is `casual`: no commit in the reviewed delta modifies a `skills/*/*/SKILL.md` (or files bundled with that skill) whose frontmatter lacks `origin: user`; a violation is a hard invariant breach — revert the commit on the branch (or reject the run) and set the driving proposal to `rejected` with the reason.
- Auto-applied changes are re-judged individually — you are the final arbiter: revert a bad one on the branch and set its proposal entry to `rejected` with the reason, so it never re-applies.
- Small defects: fix and commit on the branch. Hopeless run (structural garbage, unfixable audit red): **reject** — write the reason to `.orchestrate-report`, stop after the report step.

## 2. Merge

- Verify the checkout is `main` and clean; merge the branch (`git merge --no-edit`).
- Conflicts: resolve by the reconcile doctrine — cache the branch's delta, reset to the last cleanly-merging common state, reapply on top; never hand-resolve hunks in place.
- Run `make audit` on merged main; red audit → revert the merge and reject (step 1's report path).

## 3. Sync deployed state

- `bash cmd/sync/sync_settings.sh`, `bash cmd/sync/sync_hooks.sh`, `bash cmd/sync/sync_skills.sh` — failures are reported, never mask the merge result.

## 4. Sync context packs

- For each `repos.json` entry whose target carries `docs/context.json`: re-run `bash cmd/sync/sync_context.sh` with that file's recorded deploy options (context-dir, langs, role, symlink, prose/acdsl flags). Per-repo failures are reported and do not stop the loop.

## 5. Verify additionalDirectories reconciliation

- The invoking wrapper appends registry paths to `permissions.additionalDirectories` before this stage (headless agents cannot write `~/.claude/settings.json` — the CLI's sensitive-file guard blocks it). Verify: every `repos.json` path present in the settings permissions; report any still missing as a finding — never attempt the write yourself.

## 6. Push

- `git push origin main` only when a remote exists; report "no remote" otherwise.

## 7. Report

- One line per duty: accepted/rejected (+ reason), merged SHA, per-sync outcome, per-repo context outcomes, reconciliation delta, push result. The final message is the machine-read result; on rejection the same reason stands in `.orchestrate-report`.

## Model

- Suggested: frontier / medium
- Reason: adversarial judgment over other agents' output, conflict resolution
- Tested unviable: — (none yet)
