---
name: qq
description: One quick question, one short answer — minimal context, no tools unless the answer needs a lookup. Trigger on /qq or "quick question". Args — topic: context slice(s) to load.
author: Kevin Horst
version: 1.0.0
format: 2
argument-hint: "topic:harness how many Agent-tool subagents run concurrently"
allowed-tools: Glob, Grep, Read
acdsl-context: ACTION-*, FACT-*, RULE-*
---

# Quick Question

One question in, one short answer out — plain prose, no headers, no lists unless the answer is a list.

## Routing

**Purpose:** answer a single question fast and cheaply.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| a subject to talk through | /discuss |
| an answer that needs sources and citations | /research |
| a verdict on a piece of code | /code-verdict |

**Preconditions:**
- one question in the invocation

**Pipeline:** standalone (see README.md § Skill map, smine repo).

## Args

| Arg | Form | Default | Effect |
| :--- | :--- | :--- | :--- |
| `topic` | `topic:<name>`, repeatable | default slice | context slice(s) injected; `--no-context` skips all — [Topic use](#skill-qq-context-001) |

## Context

**SKILL-QQ-CONTEXT-001** `[def]` **Topic use**

Topic names and their slices per the smartskill rule in `context/rules/skills.md`; the default slice carries that rule
only.

**SKILL-QQ-CONTEXT-002** `[rule]` **Short answer**

Answer MUST be at most five sentences of plain prose; a list only when the answer is a list; no headers, no preamble.

**SKILL-QQ-CONTEXT-003** `[rule]` **No artifact**

Agent MUST NOT write files, edit code, or spawn agents.

## Answer

**SKILL-QQ-ANSWER-001** `[step]` **Widen**

INVOKE the Skill tool on `qq` with `topic:<name>` plus the question when the answer hinges on a topic not yet loaded —
at most one widening.

**SKILL-QQ-ANSWER-002** `[step]` **Look up**

READ the one file or grep the one pattern that settles the question when loaded context does not; skip when it does.

**SKILL-QQ-ANSWER-003** `[step]` **Answer**

ANSWER directly, uncertainty stated in the same sentence as the claim.

**SKILL-QQ-ANSWER-004** `[stop]` **Not quick**

If the answer needs more than one lookup or one widening ⇒ STOP: answer what is known and name /research or /discuss.

## Model

- Suggested: mid-tier / low
- Reason: single lookup, short answer
- Tested unviable: — (none yet)
