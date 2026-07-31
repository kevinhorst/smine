---
name: concept
description: Create, refine, or extend feature concept documents and user stories in plans/. Trigger on /concept or "draft a concept" or "write user stories".
author: Kevin Horst
version: 1.3
---

# Concept Document Skill

Create and refine feature concept documents. Concepts live in `plans/{feature_name}/concept/` and consist of an overview page, user stories, and one or more detailed design pages per implementation block.

All content MUST be in English.

## When to use

**Use when:** drafting or extending the what/why of a feature — goals, user flows, user stories, MVP/backlog cuts — or iterating on an existing concept in `plans/`.
**Don't use when:** the concept exists and only its open questions block progress — use /clarify. The question is *how* to build (files, functions, contracts) — /fdesign. Comparing solution approaches — /fexplore.
**Preconditions:** none — this is the entry point of the planning chain.
**Workflow position:** **concept** → clarify → fexplore → fdesign (see `docs/skill-map.md`, smine repo).

## Modes

### Mode A: Create a new concept

1. Ask the user for the feature name and a description if not already provided
2. Derive a directory slug (lowercase, underscores, e.g. `login_intent`)
3. Create `plans/{slug}/concept/concept.md` using the Overview Template
4. Create `plans/{slug}/concept/user_stories.md` using the User Stories Template
5. If the user provides enough specifics for an implementation block, create `plans/{slug}/concept/{block_slug}.md` using the Detailed Design Template
6. Set status to "Draft"

### Mode B: Refine an existing concept

1. Read existing concept docs at the path the user indicates (or find them in `plans/`)
2. Apply user feedback: expand sections, add detail, add new detailed design pages, resolve open questions
3. Do NOT remove content the user did not ask to change
4. Preserve existing section structure unless restructuring is requested

## Closing — chain into clarify

When the concept lands (either mode) with a non-empty Open Questions section, offer to continue straight into /clarify in the same session; on yes, invoke it against the just-written concept. Skip the offer when Open Questions is empty or the user has already scoped the session to drafting only.

## Writing Guidelines

- Write concise, direct prose. No filler or hedging.
- Use tables for structured data (thresholds, field definitions, limits).
- Use numbered steps for sequential flows. Indent sub-steps for backend/system logic within a flow step.
- Use concrete examples to clarify non-obvious rules or edge cases.
- Reference existing project services by name when relevant (consult `docs/context/service-map.md`).
- Number open questions for easy reference in discussions.
- Every MVP and Backlog item carries a rough day estimate (`~2d`); each block states its total. Estimates are ranges when uncertain (`~2–4d`), never omitted.

---

## Overview Template

Use this structure for `concept.md`:

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

Use this structure for `{implementation_block}.md` — one file per implementation block:

```markdown
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
```

---

## User Stories Template

Use this structure for `user_stories.md`:

```markdown
# User Stories: {Feature Name}

---

## {Theme / Category}

**As a {role}**, I want to {action}, so that {benefit}.

---

## {Theme / Category}

**As a {role}**, I want to {action}, so that {benefit}.
```

Group stories by user-facing theme. Use horizontal-rule dividers between groups. Keep stories independent and testable. Prefer small, specific stories over broad ones.

## Model

- Suggested: frontier / large
- Reason: concept/user-story drafting, product judgment
- Tested unviable: — (none yet)

## Changelog

- v1.3 (2026-07-19): closing step chains into /concept-clarify in-session when Open Questions remain; moved under skills/concept/
- v1.2 (2026-07-16): concept artifacts move to plans/{slug}/concept/
- v1.1 (2026-07-13): When-to-use section (routing, preconditions, workflow position)
- v1.0 (2026-07-06): initial version
