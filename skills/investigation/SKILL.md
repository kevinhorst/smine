---
name: investigation
description: Fan out N independent investigations of one question, then merge adversarially with primary-source re-verification. Trigger on /investigation or "investigate this from multiple angles" or "does this analysis hold up". Args — question: the one shared open question; inputs: fetchable primary-source inputs; artifactsDir: absolute session scratchpad path for investigator artifacts (required); priorArtifact: prior analysis entering as investigation #1; investigators: fan-out width (default 3, cap 8); resumeFromRunId: fold measurement-gate results back in.
author: Kevin Horst
version: 1.4
---

# Investigation Adversarial Merge

Fan out N independent investigations of one open analysis/data question, then merge adversarially: later analysis re-verifies every prior load-bearing claim against the **actual primary source** — public files, spreadsheets, live and archived URLs, prod query results — refutes what doesn't hold, and emits one baseline with a refuted-hypotheses register so nobody re-investigates dead ends. Skill-fronts-Workflow: this skill resolves the question, inputs, and fan-out width; the `investigation` workflow (`workflows/investigation.js` in this skill's directory) runs the deterministic fan-out, re-verification barrier, and merge.

## When to use

**Use when:** one shared open question over a dataset — a metric regression, a funnel drop, a root-cause hunt spanning external systems — warrants several independent takes plus adversarial re-verification against primary sources. Also when an existing analysis artifact (a report/PDF) is handed in for a second opinion — it enters as investigation #1 and its load-bearing claims get re-verified. Invoked via /investigation.
**Don't use when:** reviewing a code diff against conventions/specs — /railroad-review (same fan-out→verify→merge shape, but claims verified against source code, not external artifacts). A single verified root-cause diagnosis — /diagnose-debug. Fanning an arbitrary skill across a `{model} × {effort} × {arg-variants} × replicas` matrix — /parallelize.
**Preconditions:** the question is stated and the shared inputs are reachable (files, URLs, or query targets the investigators and the verifier can actually fetch — verification is primary-source-only).
**Workflow position:** standalone (see `docs/skill-map.md`, smine repo).

## Args

- question: the one shared open question to investigate.
- inputs: the fetchable primary-source inputs (paths, live/archived URLs, prod-query targets).
- `artifactsDir`: absolute path to the session scratchpad, outside every worktree — where investigator artifacts are written; required, the skill passes the session scratchpad directory (this skill is read-only toward the repo).
- `priorArtifact`: a prior analysis artifact entering as investigation #1; its load-bearing claims are re-verified.
- `investigators`: fan-out width (default 3, cap 8).
- `resumeFromRunId`: fold measurement-gate results back in, replaying the unchanged prefix from cache.

## 1. Intake

- The one shared question and the shared inputs — the dataset plus every source ref an investigator would need. Verification is primary-source-only, so inputs must be fetchable (paths, live/archived URLs, prod-query targets), not prose descriptions.
- Optionally, a prior analysis artifact entering as investigation #1 — its ranked hypotheses and load-bearing claims are extracted and re-verified alongside the fresh runs.
- Fan-out width `investigators` (default 3, cap 8) — how many independent runs attack the question. Wider only when the hypothesis space is genuinely open.

## 2. Run

- Call the Workflow tool: `{scriptPath: '<this skill's base directory>/workflows/investigation.js', args: {question, inputs, artifactsDir, priorArtifact, investigators}}` — the base directory is stated when this skill loads; args is a real JSON object, never a string. `question` and `inputs` are required strings; `artifactsDir` is the required absolute session scratchpad path (outside every worktree — the workflow rejects a missing or non-absolute value so no artifact lands in the repo); `priorArtifact` and `investigators` are optional.
- Stage A (Fan-out) runs `investigators` independent agents — each reads the inputs first, builds ranked hypotheses, and names the primary source for every load-bearing claim; siblings are never read (anti-anchoring). Stage B (Re-verify) is the barrier: claims are deduped, then parallel verifier shards fetch each claim's actual primary artifact — never a prior run's summary — and rule `holds | refuted | needs-measurement`, watching the full funnel. Stage C (Merge) emits the single baseline with fact-vs-hypothesis labeling, the refuted-hypotheses register, credited net-new findings, and a discrimination plan.

## 3. Report & the measurement gate

- Relay the merged baseline (every statement labeled fact vs hypothesis), the refuted-hypotheses register R1..Rn with refuting evidence, net-new findings per run, and the discrimination/experiment plan (one variable per cell, control arm, pre-registered failure criteria).
- **Measurement gate:** `needs-measurement` claims carry concrete queries the user runs in prod. A subagent cannot pause mid-run for that, so the workflow returns them as `open_measurements`. Surface the queries, collect the user's results as **binding facts**, then fold them in — re-invoke with `resumeFromRunId` so the unchanged prefix replays from cache and only the merge re-runs.

## Rules

- Primary sources only for verification: a claim whose primary artifact is unreachable stays `needs-measurement`, never silently `holds`.
- Investigators are independent — no reading sibling runs, no anchoring on any prior conclusion (including investigation #1's).
- External fetches (public files, Wayback) are part of the contract — investigators and the verifier get web access.
- Read-only toward the repo; the deliverable is the merged baseline document, not code changes.
- Stage C actively ingests every surviving artifact — sibling outputs are never assumed to have cross-pollinated.

## Model

- Suggested: mid-tier / medium
- Reason: intake + deterministic workflow fan-out; the hypothesis-building, primary-source re-verification, and merge happen in the cells and the high-effort synthesizers
- Tested unviable: — (none yet)

## Changelog

- v1.4 (2026-07-26): artifacts moved out of the repo — workflow arg `outputDir` (defaulted to `sessions/investigations`) replaced by required absolute `artifactsDir`, rejected if missing/non-absolute; makes "read-only toward the repo" enforced
- v1.3 (2026-07-26): Args section
- v1.2 (2026-07-24): Stage B re-verification parallelized — claim dedup + sharded verifiers (batch ≤8, effort medium); serial single-verifier removed
- v1.1 (2026-07-19): workflow tolerates args delivered as a JSON-encoded string (harness stringification)
- v1.0 (2026-07-19): initial version
