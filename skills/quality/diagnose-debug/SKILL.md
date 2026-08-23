---
name: diagnose-debug
description: Produce a verified root-cause diagnosis for approval — a diagnosis, not a fix. Trigger on /diagnose-debug or "find out why X" or pasted stack traces/logs.
author: Kevin Horst
version: 1.7
---

# Debug

This skill produces one artifact: the verified diagnosis. The fix is a separate step that starts only after the user approves the diagnosis ("Go ahead"). Diagnose-then-ask is consistently rewarded; fix-while-guessing is consistently punished.

Static constraints are not restated here: `AGENTS.md` and the style guides under `$AGENT_CONTEXT_DIR_DEFAULT/` govern any eventual fix. Investigation goes through the project's Makefile targets like everything else.

## When to use

**Use when:** the cause of a bug, error, or unexplained behavior is unknown — pasted stack traces, Sentry errors, raw logs, any "find out why X" ask. The output is a diagnosis, not a fix.
**Don't use when:** the cause is already known and the fix is trivial — just fix it directly. The problem is a plan or design question — /fdesign or /fexplore. The task is writing tests for existing code — /coverage-increase.
**Preconditions:** an observable symptom with evidence (logs, traces, repro steps, or environment comparison).
**Workflow position:** standalone — the approved diagnosis hands off to direct implementation or a /fdesign plan for non-trivial fixes (see README.md § Skill map, smine repo).

## Phase 0 — Intake

- Restate the symptom as observable facts: what happens, where, since when, and what still works. The user's statements are constraints — a hypothesis that contradicts one is dead on arrival. (Origin: "That makes 0 sense. The same code is running on main, with the same group, where it works.")
- **Map environments to deployed branches before any diff.** "Works on staging, broken on develop" means diff the deployed branches — not the local worktree, which is often pinned at main and makes `main..HEAD` a no-op. (Origin: "Check develop, this is were the issue is. MAN")
- Ask what the user changed or already ruled out. Freshly added anomalous code is usually their debug probe, not the regression — ask before building a theory on it. (Origin: "I just introduced that just too see something. Bullshit wrong.")

## Phase 1 — Evidence

Collect before theorizing. Every fact gets a source (log line, file:line, command output).

1. **Freshness** — check evidence timestamps against now before analyzing anything. Stale logs are discarded, not analyzed. (Origin: an elaborate timeline was built on 208-day-old logs.)
2. **Timeline** — parse pasted logs positionally: what happened immediately before and after the failure. The position of an event in the log IS the diagnosis. (Origin: the root cause sat directly after "Initialising schema..." in a log the user had pasted twice.)
3. **Large pastes** — grep them for the relevant identifiers; never hold the full dump in context. (Origin: two context deaths in one session from ~700KB of raw logs.)
4. **State before code** — for services with persisted state (session files, state DBs, caches, cache keys), diff the state between environments before diffing code, even when the user suspects a code regression. (Origin: the "newly introduced bug" was a warm vs cold state DB; the code was identical.)
5. **Surprising tool behavior** — read the tool's docs or source until you can say WHY it behaves that way. Never invent platform limitations, and never trust a search result over the evidence in front of you. (Origin: three invented explanations for a hook that the docs settled in one sentence; a GitHub issue "confirmed" a bug the user's own CI logs disproved.)
6. **Payload before layers** — when a request flow fails, dump the actual payload and diff it field-by-field before theorizing in any downstream layer. (Origin: a cherry-pick dropped `"country_code": "DE"`; "Took me 30 seconds".)
7. **Disk over diff** — commit diffs are not current state. Force a re-read of the affected files from disk after cherry-picks, merges, or user hotfixes before diagnosing. (Origin: a phantom critical bug — "You checked a stale implementation.")
8. **Config surfaces** — prove the host process actually reads each config surface before building on it; verify against the deployed artifact, not the checkout. (Origin: an MCP env block configured the child process only.)
9. **Live probe over archaeology** — when a capability question blocks progress, run a five-minute test import instead of document archaeology. (Origin: a ~1.5h doc stall settled by one probe.)
10. **Issue/PR status via `gh`** — GitHub issue and PR status is verified with `gh issue view --json state` before citing. Never trust WebFetch, search results, or training knowledge for issue state. (Origin: three issues cited as open+corroborating during a root-cause diagnosis were all closed.)

## Phase 2 — Hypothesize

- State hypotheses as hypotheses, never as "Found the bug". A hypothesis is promoted to diagnosis only when it explains **every** observed fact — including why env A works and env B fails, and every log line proving the process was alive.
- **The would-have-noticed gate**: if the theory predicts loud symptoms (crash loops, restart storms) the user did not observe, discard it before presenting. (Origin: a crash-loop theory contradicted the agent's own earlier observation that the handler was firing.)
- Change one variable per test round; otherwise "it works now" cannot be attributed to a cause.
- Trust ground-truth signals: PASS and exit codes beat listing or formatting anomalies; a bind error beats harness theories. (Origin: 12 turns chasing test-listing output while every run printed PASS; "Port in use you idiot".)
- Finding A bug is not finding THE bug. A real landmine discovered along the way goes into the diagnosis as a separate finding — it is the root cause only if it reproduces the reported symptom trajectory.
- **Never attribute a bug to the user's environment or a stale binary.** Reproduce the exact failing invocation against a fresh build before presenting any such theory. (Origin: "NEVER EVER EVER EVER TELL ME I RAN A STALE BINARY.")
- A correction phrased as a rule demands a mechanical grep-sweep of every sibling call site before claiming done. (Origin: a blanket rule applied to only some siblings at ~165k tokens.)

## Phase 3 — Write the diagnosis

```markdown
# <symptom> — Diagnosis

## Symptom
<observable facts, restated; environment → branch mapping>

## Evidence
<each fact with its source: log line, file:line, command output; freshness confirmed>

## Root cause
<the mechanism, walked step by step against the user's own log lines / data>

## Classification
<code bug | environment or state | not a bug (expected behavior)>

## Other findings
<real defects found that are NOT the root cause, stated as such>

## Ruled out
<hypotheses discarded and the fact that killed each one>

## Proposed fix
<all affected locations, not just the one behind the visible symptom; for
environment/state causes: the operational runbook — what to delete/restart,
in which order, and which log lines confirm recovery>
```

Then STOP and ask before fixing. Classification matters: "not a bug" is a valid, acceptable outcome when backed by the evidence walk, and "not possible cleanly" is an acceptable answer. Never write DB rows as a side effect of rendering. (Origin: a `get_or_create` on page render — "it returned 4!")

## Self-check gate

- [ ] The root cause explains every fact in Evidence, including the A/B differential and the facts proving what still worked.
- [ ] It passes the would-have-noticed gate.
- [ ] The evidence is fresh and the diff baseline is the deployed branch.
- [ ] Nothing blames the user's own instrumentation without asking first.
- [ ] Surprising tool behavior is backed by docs/source, not by an invented limitation.

## Stop conditions

The `ACTION-IMPL-*` gate entries from `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md` apply (ACTION-IMPL-002: second failed fix → stop, research, redesign). Debug-specific:

1. A hypothesis contradicts a user-stated fact → discard it, do not present it.
2. Two hypotheses disproven → stop guessing. Collect richer evidence instead: request logs, add temporary instrumentation (and remove it once the question is answered), or reproduce.
3. The user pastes evidence → mine it before theorizing further. Shifting doubt onto the user ("are you sure your log is first?") comes only after their evidence is exhausted.

## Model

- Suggested: frontier / large
- Reason: root-cause reasoning over unfamiliar failures
- Tested unviable: — (none yet)
