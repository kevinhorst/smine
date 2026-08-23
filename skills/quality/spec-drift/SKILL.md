---
name: spec-drift
description: Report drift between a source of truth and the code, read-only, with per-claim evidence. Trigger on /spec-drift [scope] or "does the doc still match the code" or "audit this contract change". Args — scope: doc-mode target, absent means the repo's durable doc set; contract: changed identifier to audit in contract mode; cap: per-run verified-claim cap override.
author: Kevin Horst
version: 2.10
argument-hint: "[scope] [contract] [cap]"
allowed-tools: Read, Grep, Glob, Bash(grep *), Bash(git log *)
---

# Spec drift detector

This skill produces one artifact: a drift report, one checkable claim at a time, each verdict backed by a code read. It is read-only in every mode — it never edits docs or code. Two modes share that contract: **doc mode** diffs a spec/design doc set against the current code; **contract mode** audits every consumer of one changed shared contract and reports which are broken, with a recommended fix order. Fixing is always a separate follow-up session working from the report.

Verification is grounded in real code reads (grep-to-zero discipline), never in memory of the codebase. A claim is not "verified" until the implementing code — or its absence — is on screen with a file:line.

## When to use

**Use when:** checking whether a spec/design doc set still matches the code — "does the doc still match the code", "audit spec vs implementation", "find stale docs", the nightly routine wrapper, invoked via /spec-drift [scope] (doc mode). Or one shared contract changed — a JSON tag, route, HTTP method, query param, Redis key format, file path, config key, or method name — and its consumers must be enumerated and classified: "X broke after the refactor/rename", "audit this contract change", "what consumes X", post-cherry-pick breakage on a shared identifier (contract mode). The output is a drift report, never a fix.
**Don't use when:** the failing contract is not yet known — find the single bug's root cause with /diagnose-debug first (it hands off here once the contract is identified). A restructuring plan is wanted — /fdesign change. Mining *session batch reports* for rule/doc updates — /smine-context. Applying a fix — that is a follow-up session after the report is reviewed.
**Preconditions:** doc mode — a doc set to audit (a durable doc, a `plans/` concept/design dir, or a `$AGENT_CONTEXT_DIR_DEFAULT/*` doc) and the code it describes, both readable in the current worktree. Contract mode — the changed contract (or the rename/move rule) is known and identifiable by a searchable form.
**Workflow position:** standalone — /diagnose-debug may hand off into contract mode; every drift or broken-consumer finding hands off to a follow-up fix session (see README.md § Skill map, smine repo).

## Args

- scope: positional, doc mode — a single doc, a feature dir, or a `plans/{slug}/` tree; absent → the repo's durable doc set.
- contract: contract mode — the changed identifier to audit — mode is selected by subject at intake, not by a flag.
- cap: per-run verified-claim cap override; overflow reported as `unchecked`.

## Mode selection

Decided at intake by subject: a doc set → doc mode; one changed contract → contract mode. One contract per contract-mode run — multiple contracts mean multiple runs, never fold two audits into one, the negative grep loses its meaning. A doc-mode finding that turns out to be a contract change becomes a new contract-mode run, not an in-run escalation.

## Doc mode

### 0. Intake

- Scope: the arg is a doc set — a single doc, a feature dir, or a `plans/{slug}/` tree. No arg defaults to the repo's durable doc set: `spec.md`, `decisions.md`, `release.md`, concept/design docs under `plans/`, and the `$AGENT_CONTEXT_DIR_DEFAULT/*` doc tree.
- Fix the baseline: the current worktree HEAD is the code side; the docs are read as they sit now. State both so the report is reproducible.
- Per-run cap on verified claims (default from config, overridable by arg) so a nightly run terminates. Overflow is reported as `unchecked`, never silently dropped.

### 1. Claim extraction

Parse each doc into checkable claims — a claim is a statement with a code anchor that can pass or fail against the code:

- Named routes, methods, functions, symbols.
- Model fields and their types.
- File paths and directory layout.
- Config keys, env vars, feature flags.
- Limits, thresholds, timeouts, caps.
- Described behaviors that name a concrete code location.

Prose claims with no code anchor are not extracted as checkable — they are listed once under **unverifiable**, never guessed at.

### 2. Verification

For each extracted claim, locate the implementing code by grep / symbol search and compare against the claim. Every verdict carries a code file:line (or MISSING) — no verdict from memory.

- **pass** — claim holds against the code.
- **drift** — code differs from the claim. Classify:
  - **doc-stale** — the code moved on deliberately; a superseding decision exists. Check `decisions.md` / git history and cite it.
  - **code-wrong** — the doc is the agreed contract and the code diverged from it.
  - **undecidable** — the differential is real but which side is authoritative needs the owner.
- **dead reference** — the referenced code is not found at all.
- **unchecked** — past the per-run cap; extracted but not verified.

### 3. Report

Write a per-claim table, numbered so every item is addressable in a follow-up session. Reuse the `smine-context` evidence discipline — verbatim excerpts, real anchors, no reconstruction — rather than inventing a second report shape.

```markdown
# Spec drift — <scope> (HEAD <short-sha>)

## Summary
<n claims: p pass / d drift / r dead / u unverifiable / c unchecked; caps and scope stated>

## Drift
| # | Claim | Doc (file:line) | Code (file:line or MISSING) | Verdict | Classification | Evidence |
| - | ----- | --------------- | --------------------------- | ------- | -------------- | -------- |
| 1 | <the claim, one line> | spec.md:42 | server/routes.go:88 | drift | code-wrong | <verbatim doc excerpt vs code excerpt> |

## Dead references
<claims whose code was not found at all, with the anchor searched>

## Unverifiable
<prose claims with no code anchor — listed, not judged>

## Unchecked
<claims past the per-run cap>

## Hand-offs
<drift findings that are contract changes → new contract-mode run; doc-stale items → doc-fix session>
```

Then STOP. The report is the deliverable; do not fix anything.

## Contract mode

1. **Intake** — the changed contract and its searchable identifier: which JSON tag / route / HTTP method / query param / Redis key format / file path / config key / method name drifted (reactive, from a breakage) or is being changed (proactive, a rename/move rule).
2. **Git archaeology** (reactive) — find the commit where the contract drifted (`git log -S`/`-G` on the identifier), read the actual diff. A commit diff is not the current state — re-read the affected files from disk afterward, because later commits may have moved things again.
3. **Consumer enumeration** — repo-wide search for the contract across **all** languages and artifacts: Go, Python, Svelte/TS, Makefiles, Dockerfiles, fixtures, seeds, docs, API collections. The known miss class is the relative-path trap — a `cd ../x && ...` whose command line does not contain the searched substring — so search by consumer *kind* (every place that could build, encode, or reference this contract), not only by literal substring. Cover derived forms of the identifier: snake_case / camelCase / kebab-case variants, URL-encoded forms, and string-built keys (a key assembled from fragments will not match the whole literal).
4. **Classify** — diff each consumer's expectation against the new reality and label it, each with file:line evidence: **broken** / **already-correct** / **intentionally-divergent** (with the justification).
5. **Report** — per-consumer table (same numbering and evidence discipline as the doc-mode report), plus a **recommended fix order**: the contract's source of truth first, then consumers in dependency order, noting where a compiler can drive the cascade. The order is a plan for the follow-up fix session — nothing is executed here.
6. **Close** — a repo-wide grep for the old form: every surviving hit is named in the report as broken or individually justified ("stays because …"). An unexplained hit means the enumeration is incomplete — the report is not done while one exists.

Then STOP. The report is the deliverable; do not fix anything.

### Cross-system parity sub-mode

For a port — a Go port of a Django behavior, say — enumerate the reference behaviors, classify each as match / divergence / acceptable-divergence, and **audit the construction and bootstrap paths, not just the outputs**: credential chains, env wiring, and initialization order are where parity silently breaks. Report-only, like everything else here.

## Self-check gate

- [ ] Every non-unverifiable claim (doc mode) and every consumer (contract mode) has a code file:line or an explicit MISSING — no verdict rests on memory.
- [ ] Every drift is classified doc-stale / code-wrong / undecidable, and doc-stale cites the superseding decision (doc mode).
- [ ] Every consumer is classified broken / already-correct / intentionally-divergent, and every surviving grep hit for the old form is accounted for (contract mode).
- [ ] Prose-only claims are under **unverifiable**, not silently dropped and not guessed at.
- [ ] Overflow past the cap is reported as **unchecked**, not dropped.
- [ ] No doc or code file was edited — in any mode.

## Nightly routine wrapper

After the skill works, a routine wraps it (see `proposals/routines.json` #5): a local `claude -p` run over the configured repos' doc sets, report written to a dated file for morning review, no fixes applied. Auth resolves via non-bare `claude -p` + `CLAUDE_CODE_OAUTH_TOKEN` (`claude setup-token`) under the Pro subscription. The routine is a separate build step — this skill never creates the schedule itself.

## Model

- Suggested: frontier / high
- Reason: claim extraction and code-vs-doc adjudication plus exhaustive multi-language consumer enumeration and divergence classification
- Tested unviable: — (none yet)
