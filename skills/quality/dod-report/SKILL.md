---
name: dod-report
description: Compile the reviewer-handoff Definition-of-Done report — why this solution, how validated, how tested, every [DoD] point confirmed with evidence. Trigger on /dod-report or "prepare the handoff report/presentation". Args — review: station review doc (default: latest in the session plan file).
author: Kevin Horst
version: 1.1
argument-hint: "[review]"
allowed-tools: Read, Grep, Glob, Write, Bash(git diff*), Bash(git log*), Bash(git status*), Bash(jq *), Bash(go run ./cmd/rules *), Bash(grep *)
acdsl-context: ACTION-REVIEW-*, RULE-PLAN-*, FACT-*
---

# DoD Report

This skill produces one artifact: the reviewer-handoff report — an English markdown document sized for a ~10-minute presentation that answers exactly four questions: Why this solution? How was it validated? How was it tested? Definition of Done — each point confirmed? It is evidence compilation by the session that did the work, **not** a review: no fresh-eyes findings, no fixes — claims are backed by artifacts that already exist or they are reported as gaps.

## When to use

**Use when:** finished work is handed to a human reviewer and the handoff brief is needed — "prepare the handoff report", "the DoD presentation", after an APPROVE/CONDITIONAL railroad-review station verdict, or on unreviewed work that goes straight to a human.
**Don't use when:** the code still needs reviewing — /railroad-review (adversarial, fresh eyes; this skill only reports). A verdict on a snippet — /code-verdict. Evaluating a change's consequences — /fimpact. Committing — /package-commit.
**Preconditions:** the change exists as a diff (branch or uncommitted) in a reachable repo with a context directory (deployed `$AGENT_CONTEXT_DIR_DEFAULT/` or `context/` in smine).
**Workflow position:** downstream of /railroad-review's human gate — `railroad-review → dod-report → human reviewer`; standalone-capable on unreviewed work (see README.md § Skill map, smine repo).

## Args

- `review`: optional — path or pointer to a railroad-review station output (`review.md`/`review.json` handoff pair or the plan-file review section) to consume. Default: the latest dated review section in the session plan file; none found ⇒ self-gather mode, stated in the report header.

## 1. Intake

**SKILL-DODREPORT-INTAKE-001** `[step]` — Resolve the change scope: the branch diff against its base, or the uncommitted snapshot — the same scope the reviewer will read. State the resolved scope (base, head, file count) in the report header.

**SKILL-DODREPORT-INTAKE-002** `[step]` — Resolve `review`: the given doc, else the latest dated railroad-review section in the session plan file. A multi-round review is resolved as its full trajectory — each round's verdict, finding counts, and dispositions — and head is diffed against the reviewed/approved snapshot: content-identical is stated as such, any divergence is named in the header. Nothing found ⇒ self-gather mode — the report says so in its header; a handoff that hides the absence of a review misleads the reviewer.

**SKILL-DODREPORT-INTAKE-003** `[step]` — Locate the why-this-solution sources: the approved plan (session plan file or `plans/{slug}/`), concept, and recorded decisions covering the change.

**SKILL-DODREPORT-INTAKE-004** `[gate]` — Query the DoD set from the context dir: `rules entries --marker DoD` (deployed dirs: `--deployed <dir>`), fallback `jq '.entries[] | select(.markers // [] | index("DoD"))' <context-dir>/context.json`. No resolvable context dir or an empty set is a **stop and report** — never substitute an improvised checklist.

## 2. Evidence

**SKILL-DODREPORT-EVID-001** `[gate]` — Every claim in the report carries evidence: a `path:line` anchor, a command with its actual output, a review-finding id, or a plan-decision reference. A claim without evidence is reported as a gap, never asserted.

**SKILL-DODREPORT-EVID-002** `[step]` — *Why this solution*: the decisions made and the alternatives rejected, from the plan/concept — delta-only, no spec restatement; the reviewer shares the spec.

**SKILL-DODREPORT-EVID-003** `[step]` — Validated (feeds `## Validation & testing`): spec-conformance evidence — with a review: the round trajectory's finding dispositions, summarized by location class (docs/runbook/config vs production code) plus each rejected finding with its accepted-risk disposition; without one: the session's own verification record, labeled self-reported.

**SKILL-DODREPORT-EVID-004** `[step]` — Tested (feeds `## Validation & testing`): manual (happy path, edge cases, error cases) and automated (test and build runs) — each with the command or scenario actually executed and its outcome. Tests that were not run are listed as not run, never implied green.

**SKILL-DODREPORT-EVID-005** `[step]` — *State*: the change's deployment/rollout status (deployed where, or not yet and what deployment waits on), required manual steps each with its acceptance reference, and accepted risks each with where it is recorded (design doc, ledger, review disposition). Accepted risks are reported with their backing evidence, never re-litigated.

## 3. DoD walk

**SKILL-DODREPORT-DOD-001** `[step]` — Reviewed mode: a summary line (`N criteria: X PASS, Y N/A, Z FAIL`, naming the source round), the status changes since the prior round each with a one-line reason, and a pointer to the review's full table and evidence. Self-gather mode: the full table, one row per `[DoD]` entry from the intake query — `| Entry | Status | Evidence |`, status PASS/FAIL/N/A, every N/A with a stated reason, every PASS with evidence per EVID-001.

**SKILL-DODREPORT-DOD-002** `[gate]` — A review's existing DoD table is reconciled, not duplicated: matching statuses cite the review; discrepancies (a PASS the evidence no longer supports, a FAIL since fixed) are called out as discrepancies with both statuses shown.

**SKILL-DODREPORT-DOD-003** `[gate]` — FAIL rows and evidence gaps are escalated to the top of `## State` — they lead the section before deployment status and accepted risks; a handoff that buries a FAIL in row 30 defeats the 10-minute format.

## 4. Output

**SKILL-DODREPORT-OUT-001** `[step]` — The report is appended to the session plan file as a dated section (the railroad-review delivery convention); written in English regardless of the working language.

**SKILL-DODREPORT-OUT-002** `[gate]` — Read-only beyond the report: gaps and FAILs are reported, never fixed in the same run — fixes route back through the normal implementation flow.

**SKILL-DODREPORT-OUT-003** `[step]` — Sized for 10 minutes: the prose sections at most ~1 page together; the DoD section is a summary in reviewed mode, and in self-gather mode the table runs as long as the marked set, N/A rows grouped to one line per reason.

**SKILL-DODREPORT-OUT-004** `[payload]` — Report template (the dated section heading from OUT-001 carries the date, the title does not):

```markdown
# DoD Report — <change>

Scope: `<base>..<head>` (<PR ref if any>, <n> files).
Review: <round trajectory: round N VERDICT → dispositions → round M VERDICT> (<head-vs-approved-snapshot identity note>) | none — self-gathered.

## State
<FAILs and unevidenced claims first — omitted when none; deployment status; required manual steps + acceptance ref; accepted risks + where recorded>

## Why this solution
<decisions + rejected alternatives, anchored>

## Validation & testing
<finding dispositions by location class + rejected findings' accepted-risk dispositions | self-reported verification record; manual scenarios and automated runs, each with command/outcome>

## Definition of Done
<reviewed: N criteria: **X PASS, Y N/A, Z FAIL** (source round); status deltas since the prior round with reasons; pointer to the review's full table and evidence>
<self-gather: full table>
| Entry | Status | Evidence |
|-------|--------|----------|
| ACTION-REVIEW-SPEC-001 | PASS | plan §2, api_test.go:88 |
| ACTION-REVIEW-MIGRATION-001 | N/A | no schema change |
```

## Model

- Suggested: frontier / medium
- Reason: evidence-vs-claim judgment across plan, diff, and review artifacts; compilation, not adversarial review
- Tested unviable: — (none yet)
