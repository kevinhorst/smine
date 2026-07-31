---
name: fmt
description: Reformat a plan or concept without changing its content — arg-routed by target. Trigger on /fmt plan <plan file> or /fmt concept <slug> <audience>. Args — plan|concept: route selector, inferred when unambiguous; plan file: plan to migrate to style/plan.md; mode: upward familiarity re-render only; caveman: compress prose, requires caveman skill; slug + audience: concept plus business|frontend-integration|custom.
author: Kevin Horst
version: 1.5
---

# fmt

One skill, two format-only routes over an existing artifact — never a content change. The contract shared by both routes is **content identity**: every decision, scope item, number, code block, and test case survives verbatim — words move, they never change. Only selection, ordering, language register, explanation depth, or structural conformance changes. The developer source stays the single source of truth; reformatted outputs are derived and regenerable.

## When to use

**Use when:** an existing plan must migrate to `style/plan.md` or up-convert its familiarity mode (`/fmt plan`), or an existing concept must be handed to a stakeholder who does not read the developer concept (`/fmt concept`).
**Don't use when:** the content must change — /concept (extend a concept), /clarify (drain concept questions), /fdesign refine (rethink a plan). The file is neither a plan nor a concept — stop.
**Preconditions:** the target artifact exists (a plan file for the plan route; `plans/{slug}/concept/` for the concept route). `style/plan.md` available for the plan route.
**Workflow position:** side-branch — `/fmt plan` off fdesign (incl. its refine route), `/fmt concept` off concept (see `docs/skill-map.md`, smine repo).

## Args

- `plan | concept`: route selector — explicit token always wins; otherwise inferred from the argument, ambiguous → ask.
- plan file: plan route, positional — the plan to migrate to style/plan.md.
- `mode`: plan route, upward only — re-render at a higher familiarity mode (`unfamiliar → familiar → owned`); a downward request STOPs.
- `caveman`: plan route — compress prose after structure migration; requires the caveman skill installed, else STOP.
- slug + audience: concept route, positional — concept slug plus audience `business | frontend-integration | custom` (custom names its own selection at intake).

## Routing

- `/fmt plan <plan file>` → the **Plan route**. A `plans/**` markdown file with a Decisions/Changes structure infers this route.
- `/fmt concept <slug> <audience>` → the **Concept route**. A slug plus a named audience infers this route.
- An explicit target token (`plan` / `concept`) always wins. When only an argument is given and it is unambiguous, infer as above; ambiguous → ask which route before doing anything.

mirrors: skillroutine-create's format rules — frontmatter, When-to-use routing, Model + Changelog sections, the three version surfaces.

---

## Plan route

Produces one artifact: the same plan, migrated to the current `style/plan.md`. Content identity: every fact, decision, code block, test case, and checklist item survives — words move, they never change. Rethinking content is /fdesign refine; this route is the standalone form of the fdesign refine route's format-migration driver.

`$AGENT_CONTEXT_DIR_DEFAULT/style/plan.md` is the spec, not a guideline: read it before touching the plan (it usually arrives via the review-context hook). If it is missing there AND not injected, STOP — there is nothing authoritative to reformat against; suggest seeding via the smine `sync_context.sh`.

### Phase 0 — Intake

- Name the plan file. Classify it: feature plan or refactor plan — that picks the section order from style/plan.md.
- **`caveman` arg (optional):** after the structure migration, compress the prose per the `caveman` skill (technical content byte-perfect). Requires `~/.claude/skills/caveman` — STOP with "caveman requested but skill not installed" if missing. Without the arg, the migration is fully content-identical.
- **`mode` arg (optional, up-conversion only):** re-render the plan at a higher familiarity mode — `unfamiliar → familiar → owned`, strictly upward. Up-conversion only deletes prose (flow traces, term explanations, explanatory bullets); code blocks and diffs are mode-invariant and stay byte-identical. A downward request STOPs — it would invent explanations on stale grounding; route to /fdesign refine.
  - The plan's args line is updated to the new mode; the Changelog row records `local: mode <from>→<to>`.
- Checked verification boxes, `⟲` rev-markers, `[USER]` markers, and OPEN rows are state, not format — they survive exactly as they are.

### Phase 1 — Format audit

Walk style/plan.md rule by rule against the plan and collect violations as a checklist. The usual suspects:

- section order wrong; Assumptions / Changelog / args line missing
- doc-assumption findings sitting inside Baseline instead of the Assumptions section
- semicolon-chained enumerations in Scope lines or bullets; prose blobs over 3 lines
- table cells with semicolon chains instead of `<br>`-stacked clauses (identifier first, **bold** discriminator); Cases cells not one-per-line
- F/D IDs without anchors; F/D/§ references that are plain text instead of internal links
- code woven into prose, fragments instead of complete units or anchored diffs, code in headings
- anchor-only facts in Baseline instead of on their Changes entry

### Phase 2 — Migrate

Apply the fixes mechanically, in place. With a `mode` arg, content identity applies to code, tables, decisions, and checklists — mode-specific prose is the one sanctioned deletion:

- **Move, never rewrite.** Sections reorder; the doc-assumption table moves to Assumptions; semicolon chains split into sub-bullets or `<br>` lines with the same words; anchors and links wrap existing IDs.
- **Create only empty-shape sections.** A missing Assumptions section with nothing to move into it gets `N/A — <reason>`. A missing Changelog is created with `| — | initial | plan created (pre-format) |`.
- **Never invent content.** A Changes entry that describes code in prose has no diff to restore — synthesizing one would be design work on stale grounding. Flag it in the report instead. Same for missing `mirrors:` lines, missing facts, missing stop conditions: reported gaps, not filled gaps.
- IDs stay stable; nothing is renumbered, re-grounded, or re-decided.
- With `caveman`: run the style pass last, compression inside bullets and cells only — structure and code stay byte-identical (per plan-format's caveman rules).

### Phase 3 — Report & changelog

- Append one Changelog row: `| <date> | local: reformat | migrated to current plan-format: <rules applied> |`.
- Chat report, two parts:
  1. table `Rule | Fixed where` — what the migration changed, so review can spot-check instead of re-reading.
  2. **Flagged gaps** — every place where conformance would need content (missing diffs, unanchored facts, empty required sections): each with a one-line pointer, recommended follow-up /fdesign refine.

### Plan-route self-check gate

- [ ] Content identity: fenced code blocks are byte-identical and their count is unchanged; every F/D ID, decision, test case, verification item, and stop condition from the original is present.
- [ ] The plan now passes style/plan.md's presentation checks: section order, args line (if non-default args are recorded), stacked cells, no semicolon chains, every F/D/§ reference an internal link, anchors on ID cells.
- [ ] Changelog row appended; no other Changelog rows touched.
- [ ] No invented content: every gap found in Phase 2 appears in the chat report, none was filled in.

### Plan-route stop conditions

1. `style/plan.md` unavailable (not in the repo pack, not injected) → stop; suggest `sync_context.sh`.
2. `caveman` requested but `~/.claude/skills/caveman` missing → stop.
3. A format fix cannot be applied without a content decision (ambiguous section for a stray paragraph, prose-only change with no code) → flag and continue; if the gaps dominate the plan, stop and recommend /fdesign refine instead.
4. The file is not a plan (no Decisions/Changes structure to migrate) → stop and report.

---

## Concept route

Produces one artifact: an audience rendering of an existing concept, written next to it. Same content-identity contract as the plan route: decisions, scope, limits, and estimates survive verbatim — only selection, ordering, language register, and explanation depth change. The developer concept stays the single source of truth; renderings are derived and regenerable.

Preconditions: a concept under `plans/{slug}/concept/`. Not for changing concept content — /concept (extend) or /clarify (drain questions); translating clarification questions is already covered by clarify's business handover.

### Audiences

| Audience | Selects | Register |
| :--- | :--- | :--- |
| `business` | Goals, user flows, MVP/backlog cuts with estimates, open business questions | No code, no schemas<br>domain abbreviations expanded on first use |
| `frontend-integration` | APIs, request/response fields, auth, limits, error semantics, models | Contract-first<br>endpoint and field definitions verbatim from the concept |
| custom (named) | The user names the selection at intake | Stated at intake |

### Process

1. **Intake** — slug, audience, and (custom only) the selection; read every page under `plans/{slug}/concept/`.
2. **Select** — pull the audience's sections; everything selected keeps its exact numbers, names, and decisions.
3. **Render** — write `plans/{slug}/concept/concept_{audience}.md`, headed by source pages + date. English, per the doc conventions; delta-only against material the audience already has.
4. **Report** — one chat line per gap where the concept lacks what the audience needs (e.g. no error semantics for frontend-integration) — reported, never invented.

### Concept-route rules

- Never invent content — a gap is a reported finding, not a filled one.
- Renderings are overwritten on re-run; they are never edited by hand and never feed back into the concept.
- A change to the developer concept invalidates the renderings — regenerate, don't patch.

## Model

- Suggested: frontier / large (concept route needs audience judgment); the mechanical plan route runs fine at mid-tier / medium
- Reason: audience judgment plus strict content identity (concept route); exhaustive format migration with no content decisions (plan route)
- Tested unviable: — (none yet)

## Changelog

- v1.5 (2026-07-31): activity-scoped context — plan spec at $AGENT_CONTEXT_DIR_DEFAULT/style/plan.md; missing-spec stop suggests sync_context.sh again
- v1.4 (2026-07-30): context redesign — plan-format.md read from ../fdesign/assets/ (ships with fdesign); missing-spec stop suggests sync_skills.sh
- v1.3 (2026-07-30): skills/fmt/ became a group dir (fmt + caveman); reference rename design-refine → the fdesign refine route
- v1.2 (2026-07-27): reference rename — couchskill-create → skillroutine-create
- v1.1 (2026-07-26): Args section
- v1.0 (2026-07-24): merges reformat-concept v1.0 + reformat-plan v1.5 into one arg-routed skill (`plan` | `concept` routes); behavior within each route unchanged
