---
name: delegate
description: Run an eligible skill on a cheaper subagent runner — explicit invocation only, never a default. Trigger only on /delegate <skill> [args]. Args — skill: the eligible target skill; args: the target skill's own args, passed through unchanged.
author: Kevin Horst
version: 1.3
---

# Delegate

Run one eligible skill unattended on a subagent runner — same worktree, same branch — while the main session stays the interactive front. Skills declare eligibility (the `Delegation:` line in their `## Model` section); this skill owns every mechanism detail. Delegation never happens implicitly: no skill self-delegates at intake.

## When to use

**Use when:** explicitly invoked as /delegate <skill> [args...] to run an eligible skill on a cheaper runner.
**Don't use when:** the target skill has no `Delegation:` line or is small-tier — not delegatable, run it in-session. Fan-out across a model/effort matrix — /parallelize. The Agent tool is unavailable (e.g. Codex) — run in-session.
**Preconditions:** target skill installed; its `## Model` section declares `Delegation: unattended-safe | gated`.
**Workflow position:** standalone wrapper around any eligible skill (see `docs/skill-map.md`, smine repo).

## Args

- skill: positional — the eligible target skill to run on the cheaper runner.
- args: positional, rest — the target skill's own args, passed through unchanged.

## Eligibility (refuse, don't improvise)

Read the target skill's `## Model` section; on any failed check refuse with the reason and offer an in-session run:

- No `Delegation:` line → interactive-only.
- `Suggested` tier is `small` → never delegated: runner overhead exceeds the work.
- A `Tested unviable:` entry naming the mapped target model → vetoed, no silent fallback tier.
- No `allowed-tools` frontmatter on a delegatable skill → refuse as undeclared surface.
- An `allowed-tools` rule not covered by the permissions allowlist (`settings/claude_code/settings.json`) → a runner stalling on a permission prompt has nobody watching; the refusal names the missing rules.

## Tier → model mapping

| Tier | Runner model |
| :-- | :-- |
| mid-tier | sonnet |
| frontier | opus |

- `small` never maps — see Eligibility.
- Mapped IDs must exist in `availableModels` (settings/claude_code/settings.json, smine repo).
- The mapping lives only here — never restated in a skill.

## Spawn contract

Spawn ONE runner via the Agent tool and wait for it:

- `subagent_type`: the skill's `Runner:` (default `general-purpose`)
- `model`: the mapped target — only when the runner is `general-purpose`; a custom runner (`agents/<name>.md`) carries model + effort in its own frontmatter
- no `isolation` key — the runner inherits the session worktree and branch
- `run_in_background: false`

Prompt template:

1. Read `AGENTS.md` at the repository root first. Then: <constraints inlined verbatim — the target skill's Rules and Stop-conditions sections; a gated skill's `## Delegation` section names exactly what to inline>.
2. Invoke the Skill tool: `skill="<name>"`, args = the original args unchanged. Follow the loaded skill exactly, unattended: never wait for user approval, never enter plan mode, never invoke /delegate.
3. Abort-and-report per the skill's own STOP rules. Returning a `failed` or `blocked` result is success, not failure — never improvise past a gate.
4. Your final message is exactly the structured result below — nothing else.

## Result contract

- `status`: `done | failed | blocked` (`blocked` only for gated skills)
- `commits`: `hash — subject` per commit made (empty on `failed`)
- `failure`: failing command + trimmed output (only on `failed`)
- `notes`: deviations or skipped groups, if any

`blocked` (gated skills) adds:

- `gate`: stop-condition id
- `unit`: unit in flight + per-unit attempt counts for all touched units
- `question`: the decision needed, phrased for the user with the runner's findings — never a recommendation disguised as a question when the plan already decided
- `progress`: units completed (with commits), unit in flight, units remaining
- `worktree_state`: clean/dirty + last commit hash

Relay the result verbatim — never re-summarize from scratch. Runner null/error → report the delegation failure; check `git log` before offering an in-session rerun (the runner may have died mid-commit). Never rerun silently.

## Gate relay (gated skills)

- Before spawning, read the gated skill's `## Delegation` section — it carries the skill's relay data: relay-class gate ids, spawn-prompt inlines, attempt-ceiling semantics.
- On `status: blocked`: sanity-check `worktree_state` against `git log`, then surface the runner's `question` verbatim plus its `progress` digest.
- Send the user's ruling to the same runner via SendMessage — context and attempt counters intact — and wait again. One round-trip per gate; gates are never batched.
- Runner death (SendMessage fails / runner gone): respawn-with-digest fallback — a fresh runner primed with the last blocked-state result, `git log`/`git status` of the worktree, and the per-unit completion state. Attempt counters transfer via the digest (the blocked state always records them).
- On `status: done`, relay the runner's report.

## Model

- Suggested: mid-tier / low
- Reason: eligibility lookup, one spawn, verbatim relay — the judgment lives in the runner and the gated skill's own relay data
- Tested unviable: — (none yet)

## Changelog

- v1.3 (2026-07-30): eligibility reads allowed-tools frontmatter instead of the retired Command surface line; refusals name the missing rules
- v1.2 (2026-07-30): moved from skills/util/ to skills/orchestration/ group; name and behavior unchanged
- v1.1 (2026-07-26): Args section
- v1.0 (2026-07-24): initial version — explicit-only delegation; mechanism folded in from context/general/delegation.md (deleted); auto-intake removed from all skills
