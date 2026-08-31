---
name: smine-permissions
description: Extract allowlist-addition proposals from smine-batch permission blocks into proposals/permissions.json. Trigger on /smine-permissions or "mine permission grants into allowlist proposals". Args — batch file: one batch; absent means all batches with ledger-missing sessions; production cap: honored when the invocation states one.
author: Kevin Horst
version: 0.1.0
argument-hint: "[batch file] [production cap]"
allowed-tools: Read, Write, Edit, Bash(go run ./cmd/acdsl *)
---

# smine: permission proposals

Turn the prompted-and-granted tool/command pairs that smine-batch captured into one ranked doc of allowlist-addition proposals — the goal is sessions as safe and frictionless as possible: a rule that was repeatedly granted becomes a candidate for `permissions.allow`, nothing is widened automatically.

## When to use

**Use when:** extracting and ranking allowlist-addition proposals from batch reports — tool/command pairs that were prompted and granted across sessions. Invoked via /smine-permissions or as part of the /smine fan-out.
**Don't use when:** editing `settings.json` permissions directly — that is the config server's live permissions editor or /smine-apply (vote flow). Mining harness friction that is not a permission grant (hook opportunities, MCP gaps) — /smine-context. Writing a batch report — /smine-batch. Denials or deny/ask rules — out of scope: a denial is ambiguous and enters only as negative evidence on a candidate.
**Preconditions:** one or more completed batch reports under `sessions/` carrying the `permissions` block (batches produced by smine-batch ≥ 1.28).
**Workflow position:** smine pipeline: smine-batch → batch → fan-out → **smine-permissions** (see README.md § Skill map, smine repo).

## Args

- batch file: positional — one batch file; absent → every batch containing session IDs missing from the ledger.
- production cap: when the invocation states "Production cap: add at most N new proposals", write at most N new proposals this run — keep the best-ranked candidates, list every dropped candidate in the run report (never silent). Evidence appends to existing proposals do not count against the cap.

## 0. Setup

- Input: batch JSON sidecars `sessions/*/json/*batch-*.json` — every folder under `sessions/` except `archived/`; read the `sessions[].permissions` block, not the md prose. A batch without any `permissions` block is forward-only history (predates smine-batch 1.28): ledger its sessions without extraction.
- Ledger: `sessions/<scope>/analyzed-permissions.txt` — first line `Last analyzed batch: <batch filename> at <YYYY-MM-DD>`, then one full session ID per line. Create on first run. Sessions already in the ledger are skipped on re-runs.
- Scope: arg = one batch file. No arg = every batch containing session IDs missing from the ledger.
- Output: `proposals/permissions.json` — the single authoritative artifact: cumulative, cross-scope, ranked, updated in place, conforming to `proposals/schema.json` (kind `permissions`). There is no md form.
- **Language.** Read `~/.claude/context/global/presentation-profile.md` before writing output; when its `language:` is set and not `en`, author the user-visible prose fields — the title's change-name part after `<target> — `, `change`, `evidence[].title`, `sessions[].note` — in that language, following the profile body's register and glossary. Never translate: ids, targets, rule strings, file paths, dates, tags, schema keys, status values, and the fixed group title (machine-read key, translated in the UI). Absent profile = English, unchanged.
- **Casual presentation.** When the profile's `audience:` is `casual`, the prose fields carry no file paths and no schema/taxonomy jargon — say what becomes frictionless; the rule string itself stays in `change`.
- Repo tags: grants concentrated in one repo's sessions → tag each `repo:<name>` (roster name from the batch's scope); cross-repo grants → no repo tag. The apply target (`settings/claude_code/settings.json`) is this repo, so a `repo:<name>` tag is provenance, never a foreign-apply route — the rule always lands in the shared allowlist.

## 1. Extract & merge

- Collect every `sessions[].permissions.granted` entry across the in-scope batches: `{tool, command, decision}`. Skip `denied` entries here — they are negative evidence gathered in step 2, never a candidate on their own.
- Merge grants into **rule candidates** by tool + command family, not by exact command line: `Bash` commands sharing a program + subcommand (`gh pr view 123`, `gh pr view 456` → family `gh pr view`); an `mcp__server__tool` grant is its own family; a `WebFetch` grant families by domain; a `Skill(name)` grant families by skill. Count the distinct sessions each candidate appears in — recurrence is the rank signal.
- Keep every contributing session id + the concrete granted command as evidence.

## 2. Candidate → rule

- Derive the **narrowest** dialect-conformant rule covering the candidate's observed commands (the repo's space-wildcard allowlist dialect, copied verbatim from existing `permissions.allow` entries):
  - Bash: `Bash(<program> <subcommand> *)` — never a bare `Bash(*)`, never `Bash(<program> *)` when only one subcommand was ever granted.
  - MCP: the exact `mcp__server__tool` string; a whole-server `mcp__server` only when every tool of that server was granted across the evidence.
  - Web: `WebFetch(domain:<host>)`; Skill: `Skill(<name>)`.
- One proposal = one rule = one vote. `change` is the imperative edit sentence: "Add `Bash(gh pr view *)` to `permissions.allow` in settings/claude_code/settings.json". `target` is `settings/claude_code/settings.json`.
- Attach the denials of the same family as **negative evidence** in the proposal (a note naming the sessions that denied it); a candidate with denials in its family is capped in rank (step 4) and, when denials outweigh grants, dropped with a run-report line — a contested rule is not proposed.
- Evidence rows cite the granting sessions with `dimension: harness-friction` (the batch findings that carry permission signal); `note` names the concrete granted command.

## 3. Archive & allowlist suppression

- Before writing any proposal, read `proposals/archive/done.md`, `archive/rejected.md`, and `archive/postponed.md` and build the suppression set: `done` and `rejected` entries are permanent — never re-propose; a `postponed` entry is suppressed within 14 days of its dated status line and eligible again from day 15 on.
- Read `settings/claude_code/settings.json` and its `settings.disabled.json` sibling: drop any candidate whose rule is already present in `permissions.allow`, `deny`, or `ask`, or parked in `settings.disabled.json` (a rule Kevin deliberately turned off must not resurface). A candidate matching a `deny`/`ask` entry is dropped, never re-proposed as an allow.
- List every drop (`candidate → matching allowlist/archived entry`) in the run report — suppression is auditable, never silent.
- Reconcile the entries **already in `permissions.json`**: drop any `proposed` entry whose rule is now present in `permissions.allow` or matches a `done`/`rejected` archive entry — an entry written before its vote was applied is a stale duplicate. This removes `proposed` rows only, never a user-set status.

## 4. Rank & write

- Rank by distinct-session grant count (frequency of the prompt = friction removed), then by grant-vs-denial balance. A candidate needs **≥ 2 distinct sessions** granting it to become a proposal — a one-off grant is not recurring friction.
- Single group (`groups[]`): **Allowlist additions**. This title is a machine-read key — always written verbatim in English regardless of any presentation-profile language.
- **One proposal = one change = one vote.** Every votable proposal carries exactly one `change`. Ids are `<rule-slug>--<n>` only when one candidate genuinely needs N sibling rules; assigned once, never renumbered (votes bind to the id). New proposals carry a `proposed` field (`<YYYY-MM-DD>`, the analyze-run date) — stamped once, never rewritten.
- **Title.** `<target> — <rule>` (e.g. `settings.json — Bash(gh pr view *)`).
- Status is the user's column (`proposed | accepted | building | done | rejected | postponed`): new entries get `proposed`; re-runs may add evidence to any entry but never change a user-set status or delete an entry.

## 5. Finish

- Validate: run `go run ./cmd/acdsl check` from the repo root and fix any violation in files this run wrote. If Bash is unavailable (restricted headless run), note "schema check skipped — consolidate/CI covers it" in the run report.
- Append the processed session IDs to the ledger and set its `Last analyzed batch:` first line (insert if absent), then STOP for user review.

## Rules

- Allow-rules only — never propose a `deny` or `ask` rule, never a rule broader than what the evidence granted, never a bare `Bash(*)` or a whole-server `mcp__server` unless every tool of it was granted.
- Rule strings are copied character-exact into the store in the existing allowlist dialect (space wildcards, never colon forms).
- Evidence format — one `evidence[]` object per point (schema fields): `title` (the rule/pattern name), `sessions[]` (1–3, ranked strongest occurrence first, each `{id, note}` with the full session id and the granted command), optional `dimension: harness-friction`. At least one session is mandatory.
- Full session IDs, never truncated; granted commands verbatim.
- Proposals only — this skill never edits `settings.json` itself; smine-apply applies a `+` voted rule.
- Archived entries (`archive/done.md`, `archive/rejected.md`, `archive/postponed.md`) plus the live allowlist are anti-re-proposal memory — the mechanical suppression in step 3 is the enforcement.

## Model

- Suggested: mid-tier / medium
- Reason: merge/rank grant candidates against the live allowlist and archive
- Tested unviable: — (none yet)
