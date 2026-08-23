---
name: concept
description: Create, refine, or extend feature concept documents and user stories in plans/. Trigger on /concept or "draft a concept" or "write user stories".
author: Kevin Horst
version: 1.6
acdsl-context: ACTION-CONCEPT-*, FACT-*
---

# Concept Document Skill

Create and refine feature concept documents. Concepts live in `plans/{feature_name}/concept/` and consist of an overview page, user stories, and one or more detailed design pages per implementation block.

The context entries this skill declares in its frontmatter (concepting doctrine incl. hot gates, repo facts) arrive injected at invocation — honor them without re-reading the context tree.

All content MUST be in English.

## When to use

**Use when:** drafting or extending the what/why of a feature — goals, user flows, user stories, MVP/backlog cuts — or iterating on an existing concept in `plans/`.
**Don't use when:** the concept exists and only its open questions block progress — use /clarify. The question is *how* to build (files, functions, contracts) — /fdesign. Comparing solution approaches — /fexplore.
**Preconditions:** none — this is the entry point of the planning chain.
**Workflow position:** **concept** → clarify → fexplore → fdesign (see README.md § Skill map, smine repo).

## Modes

### Mode A: Create a new concept

**SKILL-CONCEPT-MODEA-001** `[step]` — Ask the user for the feature name and a description if not already provided.

**SKILL-CONCEPT-MODEA-002** `[step]` — Derive a directory slug (lowercase, underscores, e.g. `login_intent`).

**SKILL-CONCEPT-MODEA-003** `[step]` — Create `plans/{slug}/concept/concept.md` using the Overview Template.

**SKILL-CONCEPT-MODEA-004** `[step]` — Create `plans/{slug}/concept/user_stories.md` using the User Stories Template.

**SKILL-CONCEPT-MODEA-005** `[step]` — If the user provides enough specifics for an implementation block, create `plans/{slug}/concept/{block_slug}.md` using the Detailed Design Template.

**SKILL-CONCEPT-MODEA-006** `[step]` — Set status to "Draft".

### Mode B: Refine an existing concept

**SKILL-CONCEPT-MODEB-001** `[step]` — Read existing concept docs at the path the user indicates (or find them in `plans/`).

**SKILL-CONCEPT-MODEB-002** `[step]` — Apply user feedback: expand sections, add detail, add new detailed design pages, resolve open questions.

**SKILL-CONCEPT-MODEB-003** `[step]` — Do NOT remove content the user did not ask to change.

**SKILL-CONCEPT-MODEB-004** `[step]` — Preserve existing section structure unless restructuring is requested.

## Closing — chain into clarify

**SKILL-CONCEPT-CLOSING-001** `[step]` — When the concept lands (either mode) with a non-empty Open Questions section, offer to continue straight into /clarify in the same session; on yes, invoke it against the just-written concept.

**SKILL-CONCEPT-CLOSING-002** `[step]` — Skip the offer when Open Questions is empty or the user has already scoped the session to drafting only.

## Writing Guidelines

**SKILL-CONCEPT-WRITING-001** `[review]` — Write concise, direct prose. No filler or hedging.

**SKILL-CONCEPT-WRITING-002** `[review]` — Use tables for structured data (thresholds, field definitions, limits).

**SKILL-CONCEPT-WRITING-003** `[review]` — Use numbered steps for sequential flows. Indent sub-steps for backend/system logic within a flow step.

**SKILL-CONCEPT-WRITING-004** `[review]` — Use concrete examples to clarify non-obvious rules or edge cases.

**SKILL-CONCEPT-WRITING-005** `[review]` — Reference existing project services by name when relevant (consult `docs/context/service-map.md`).

**SKILL-CONCEPT-WRITING-006** `[review]` — Number open questions for easy reference in discussions.

**SKILL-CONCEPT-WRITING-007** `[review]` — Every MVP and Backlog item carries a rough day estimate (`~2d`); each block states its total. Estimates are ranges when uncertain (`~2–4d`), never omitted.

---

## Overview Template

**SKILL-CONCEPT-TPL-001** `[payload]` — Overview Template — use this structure for `concept.md`.

```markdown
# Concept: {Feature Name}

> **Status:** Draft | In Review | Approved | Superseded
> **Author:** {name}
> **Date:** {YYYY-MM-DD}

---

## Goals

- {Concrete outcome}
- {Concrete outcome}

---

## User Flows

### {Flow Name}

**Goals:**
- {What this flow achieves}

**Options:**

**MVP**
- {Flow or capability to implement now}
- {Flow or capability to implement now}

**Backlog**
- {Deferred flow or capability}

**Challenges:**
- {Known challenge or risk}

**Approach:**
- {How to address the challenge}

---

## Decisions / Open Questions

**Decisions:**
- {Resolved decision with rationale}

**Open Questions:**
1. {Actionable question}
2. {Actionable question}
```

---

## Detailed Design Template

**SKILL-CONCEPT-TPL-002** `[payload]` — Detailed Design Template — use this structure for `{implementation_block}.md`, one file per implementation block.

````markdown
# {Implementation Block Name}

---

## Flows

### {Scenario Name}

1. {Actor action}
2. {Actor action}
3. Backend
   1. {Backend step}
   2. {Backend step}
4. {Actor action}

### {Scenario Name}

1. ...

---

## Security Considerations

- {Threat and mitigation}
- {Threat and mitigation}

---

## Limits

- {Parameter}: {Value} ({rationale})
- {Parameter}: {Value} ({rationale})

---

## Models

### {Model Name}

**Public:**
- {field}: {description} ({type})

**Internal / Not Exported:**
- {field}: {description} ({type})

**Unique Index:**
- {field(s)}

---

## APIs

### {Method} {Path}

{Purpose}

**Notes:**
- {Behavioral note}

**Request fields:**
- {field}: {description} ({type})

**Request headers:**
- {header}

**Response fields:**
- {field}: {description} ({type})

**Rate Limits:**
{limit}

**Example ({status}):**

Request:
```json
{}
```

Response:
```json
{}
```

---

## Worker Tasks

- {Task name}
  - {Sub-task or detail}

---

## Infrastructure

- {Resource, config change, or deployment concern}

---

## Long-Tail Tasks

### {Area}

- {Task or R&D item}
- {Task or R&D item}
  - {Sub-item or open question}
````

---

## User Stories Template

**SKILL-CONCEPT-TPL-003** `[payload]` — User Stories Template — use this structure for `user_stories.md`.

```markdown
# User Stories: {Feature Name}

---

## {Theme / Category}

**As a {role}**, I want to {action}, so that {benefit}.

---

## {Theme / Category}

**As a {role}**, I want to {action}, so that {benefit}.
```

**SKILL-CONCEPT-STORIES-001** `[review]` — Group stories by user-facing theme; use horizontal-rule dividers between groups; keep stories independent and testable; prefer small, specific stories over broad ones.

## Model

- Suggested: frontier / large
- Reason: concept/user-story drafting, product judgment
- Tested unviable: — (none yet)
