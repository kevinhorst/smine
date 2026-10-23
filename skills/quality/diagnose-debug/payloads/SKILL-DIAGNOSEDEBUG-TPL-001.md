# <symptom> — Diagnosis

## TLDR
<exactly three bullets titled **Symptom**, **Cause**, **Fix** — one plain-language line each, no fact links, no internal names the user has not used>

## Symptom
<override: what the user sees, where, since when, what still works — at most four one-line bullets; an observed symptom has no catalog section>

## Root cause
<override: the mechanism in plain language — at most five bullets in causal order, or one sequence diagram plus at most three bullets; a fact link only on the claims the fix rests on; last line the class: code bug | environment or state | not a bug (expected behavior), with the deciding fact — a causal chain has no catalog section>

## Changes
<the fix — opens with the planned-files table — File | Kind | Entry, every file the fix would create/modify/delete (per rules/plan.md); proposed, not applied — all affected locations, not just the one behind the visible symptom, as diff blocks; for environment or state causes: the operational runbook — what to delete or restart, in which order, and which log lines confirm recovery>

## Reproduction [AGENT-ONLY]
<override: environment → branch mapping, the reproduction command, the step-by-step walk against the user's own log lines and data — a repro recipe has no catalog section>

## Ruled out [AGENT-ONLY]
<override: hypotheses discarded and the fact that killed each one>

## Other findings [AGENT-ONLY]
<override: real defects found that are NOT the root cause, stated as such — never silently fixed>

## Baseline (verified) [AGENT-ONLY]
<facts table: ID | Fact | Needed for | Location — each fact with its source: log line, file:line, command output; freshness confirmed; "Needed for" links the Root cause claim or Ruled-out hypothesis it supports; pivotal facts marked `F<n>!`>
