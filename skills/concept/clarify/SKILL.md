---
name: clarify
description: Resolve a concept's open questions in plans/ into binding decisions, emptying the Open Questions section. Trigger on /clarify after a concept is checked in but questions remain.
author: Kevin Horst
version: 1.10
---

# clarify

This skill produces one artifact: the updated concept with its Open Questions drained into Decisions. Every resolved question becomes a decision with rationale, rippled into the affected concept pages. Resolution is joint work: some questions are answered by research, some only the user can answer — but every answer is verified before it is recorded.

Concept documents follow the structure of the `concept` skill: `plans/{slug}/concept/concept.md`, `user_stories.md`, and detailed design pages. This skill edits those files only. All content stays in English.

Ordering: clarify (and /fexplore) run BEFORE /fdesign. A clarify pass after an approved plan invalidates its decisions — the plan then needs /fdesign refine. (Origin: 4 approved plan decisions died two days after approval because clarification ran late.)

## When to use

**Use when:** a concept is checked in under `plans/` but open questions, TBDs, or contradictions remain — the deliverable is the concept with an empty Open Questions section.
**Don't use when:** no concept exists yet — /concept first. The open question is which *solution approach* to take — /fexplore. Open questions surface after a plan is approved — that is a /fdesign refine driver, running clarify then invalidates the plan.
**Preconditions:** concept documents in `plans/{slug}/concept/` following the /concept structure.
**Workflow position:** concept → **clarify** → fexplore / fdesign (see README.md § Skill map, smine repo).

## Phase 0 — Intake

- Locate the concept (user-named, or found in `plans/`) and read every page of it.
- Build the question list: the numbered Open Questions, plus implicit ones — contradictions between pages, TBD markers, sections that dodge a decision. Implicit questions are added to the numbered list first, so they get the same treatment.
- Confirm the list and its working order with the user before resolving anything. Highest-leverage questions first — an early answer often kills later questions.

## Phase 1 — Triage

Classify each question; the class determines who answers:

1. **Researchable** — the answer exists in the repo, the project docs, or an external source (API capability, library behavior, real data shape). Research it; never ask the user what can be looked up.
2. **Preference** — genuinely the user's call: product scope, priorities, trade-offs with no technical winner. Only these are asked.
3. **Obsolete** — overtaken by other decisions or dropped scope. Proposed for removal with the reason, never silently deleted.

Preference questions carry an **owner**: `[USER]` (technical calls) or `[BUSINESS]` (user-visible product rules: ranking, tiebreaks, display precision, eligibility definitions, default selections). The agent never invents a business rule — it proposes options and routes the question.

**Business handover**: `[BUSINESS]` questions are exported to `plans/{slug}/concept/questions.md` — each question self-contained (context a non-developer can read) with lettered options `a) b) c)` and a recommendation; answers are recorded inline with status "Answered". When the business side needs another language, a translated twin (`questions_de.md`) is kept in sync — English authoritative.

## Phase 1b — Source-requirements diff

When the concept derives from a source requirements document (often German): re-read the source line-by-line against the concept and flag as findings:

- **Proxy substitutions** — a requirement not computable from available data was silently redefined ("aktiv in der App" became "voted"). Surface the substitution as a decision, never silently redefine the metric.
- **Requirement strengthening/weakening** — the concept gates or restricts more (or less) than the source says ("werden angezeigt" became view-gating).
- **Unstated interpretation choices** — places where the concept picked one reading of an ambiguous requirement without recording the choice.

Findings join the numbered question list with the same triage treatment. (Origin: this pass caught 3 misreadings and 5 silent choices the question-drain missed.)

## Phase 2 — Resolve

Work the list in the confirmed order, batching questions whose answers are independent:

- **Researchable**: gather evidence with sources (file:line, doc link, inspected real data), then present the proposed answer WITH its evidence for confirmation. A proposal without a source is an opinion, not an answer.
- **Preference**: ask once, with concrete options and a recommendation. An answered question is binding; if the chosen option turns out much bigger than presented, ask again — never silently demote it.
- **Directly answered**: when the user answers a question unprompted, review the answer against repo reality before recording it. An answer that contradicts repo state or an existing decision is a finding to surface immediately — not silently recorded, not silently corrected.

## Phase 3 — Record & ripple

- Each resolved question becomes a Decision bullet with rationale; user-made calls marked [USER], business-made calls marked [BUSINESS]. Remove it from Open Questions (and mark it Answered in `questions.md`).
- A one-field design decision reached in chat is written into the plan/spec docs in the same session — even a single-field call that was never a listed Open Question. A decision that lives only in chat resurfaces later as a review finding. (Origin: a 5-digit-OTP / no-initial-delay decision made only in chat came back as a review finding.)
- Ripple every decision into the affected sections: user flows, detailed design pages, user stories. A decision recorded but not rippled leaves the concept self-contradictory — the ripple IS the work.
- The concept skill's editing rules hold: do not remove content the user did not ask to change; preserve section structure.
- Questions that stay open keep their number plus a status note: what blocks them and who owns the answer.

## Self-check gate

- [ ] Every new Decision has a rationale, and either an evidence source or a [USER] mark.
- [ ] No section of any concept page contradicts a new decision — checked by re-reading the pages, not from memory.
- [ ] Nothing was removed that the user did not ask to change.
- [ ] Open Questions is empty — or every survivor has a status note the user has seen.
- [ ] Status bumped (Draft → In Review) only when the list is empty.

## Stop conditions

The `ACTION-IMPL-*` gate entries from `$AGENT_CONTEXT_DIR_DEFAULT/actions/implementing.md` apply. Clarify-specific:

1. An answer invalidates the concept's Goals or a prior [USER] decision → stop and report; re-scoping the concept is the user's call, not a ripple.
2. Two rejected proposals for the same question → stop proposing; present the trade-off with the evidence for each side and let the user decide.
3. Research shows a whole detailed-design page is built on a false assumption → stop; that is a /concept refinement or redesign, not a clarification.

## Model

- Suggested: frontier / large
- Reason: turning open questions into binding decisions
- Tested unviable: — (none yet)
