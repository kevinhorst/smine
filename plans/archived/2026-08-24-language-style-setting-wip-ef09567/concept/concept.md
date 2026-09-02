# Concept: Language + Style Setting (Non-Developer Install)

> **Status:** Draft
> **Author:** Kevin Horst
> **Date:** 2026-08-24

---

## Goals

- One global **presentation profile** per install: language (`de`), register (semi-casual / semi-professional), audience (non-developer — no developer jargon, no smine-engine internals).
- Every surface the end user reads honors the profile: config server UI, proposal cards, session/batch views, welcome/tutorial, and prose produced by smine skill runs.
- Enforcement against model drift back to English or developer register — a checked gate in the pipeline, not a polite instruction. Opus/Fable drift is the known failure mode.
- The install remains Kevin-operable: repo content, logs (`results.jsonl`), and operator artifacts stay English; a machine can be switched back to developer/English mode.

---

## User Flows

### Presentation Profile as Single Source of Truth

**Goals:**
- One place declares language + register + audience for the whole install; everything else derives from it.

**Options:**

**MVP** (~1–2d)
- Per-install profile file (not repo content — the claude-configs repo stays Kevin's), written by install/sync. (~0.5d)
- Profile materialized into `context/global/presentation-profile.md`, so the existing `cmd/hooks/global-context.sh` injection carries it into **every** session on that machine — smine skill runs and interactive Claude sessions alike. (~0.5d)
- Explicit supersede clause over the global CLAUDE.md rule "handoff/doc artifacts: always English" (`settings/claude_code/CLAUDE.md:16`) and the fmt English rule — the profile wins for user-visible text. (~0.5d)

**Backlog**
- Config-server settings page to view/edit the profile. (~1–2d)
- Install-time selection (installer asks for language/audience). (~1d)

**Challenges:**
- The English-only mandate is a hard, existing global rule; adding a German directive without superseding it produces contradictory instructions.

**Approach:**
- The profile doc names the rules it overrides and scopes the override to user-visible artifacts only; operator/repo artifacts explicitly stay English.

---

### German, Non-Developer LLM Prose

**Goals:**
- All user-visible prose written by the smine pipeline (proposal titles, change lines, detail fields, evidence notes, group titles, batch report views) is German, semi-casual/semi-professional, jargon-free.

**Options:**

**MVP** (~1–2d)
- Profile doc carries the language/register directive plus a **jargon glossary**: banned engine/developer terms with plain-German replacements (e.g. "worktree", "ACDSL gate", "transcript mining", "consolidate" never surface to the user). (~1d)
- Thread `language de` into the nightly consolidate stage (`routines/smine-nightly/run.sh:147`, currently `/smine-consolidate proposals` with no language arg — the skill's `language <lang>` contract already exists). Consolidate becomes the style-enforcement pass over the proposal store: it rewrites drifted English/jargon prose every night. (~0.5d)
- Exclusion list mirroring consolidate's mutable-prose set: `snippets[].code`, ids, dates, tags, gate fields are never translated or reworded. (~0.5d)

**Backlog**
- Dedicated style verifier/eval scoring pipeline outputs against the profile (skillroutine-eval style). (~2–3d)
- One-time migration pass rewriting the existing English proposal store on first profile activation (beyond what nightly consolidate catches). (~0.5d)

**Challenges:**
- Model drift: Opus/Fable fall back to English developer register mid-artifact, especially for technical content.
- Mixed-language store during transition.

**Approach:**
- Two layers: the directive reaches every session via global context (prevention), and the nightly consolidate language pass rewrites what slipped through (correction). Enforcement bar for the experiment is consolidate-as-gate only; a hard pre-render verifier is backlog.

---

### German Config Server UI

**Goals:**
- The web UI the user opens is German: navigation, welcome/tutorial, proposal card labels, page headings.

**Options:**

**MVP** (~3–5d)
- Minimal message catalog covering the pages the non-developer actually sees (aligned with the reduced navigation below): template strings plus Go-originated labels (`overview.go`, `proposals.go`, config-row copy). German overlay only; English remains the default and the fallback. (~3–4d)
- `lang` attribute driven by the profile (today `layout.html:3` hardcodes `lang="en"`). (~0.5d)

**Backlog**
- Full two-locale catalog across every page (Skills/Context/Repos/Tools/Checklist). (~2–3d)
- Further locales. (—)
- Localized install/console output (`install.sh`, sync scripts) — developer-facing today, lowest priority. (~1d)

**Challenges:**
- No i18n layer exists; every string is literal English in Go source and templates.

**Approach:**
- Scope the catalog to the reduced surface instead of a whole-app i18n project; the reduced navigation keeps the translated page set small.

---

### Non-Developer Surface Cut

**Goals:**
- "Abstain from exposing inner workings" is structural, not just wording: the Skills/Context/Repos/Tools tabs and proposal-card internals (gate band, tags, targets) ARE engine internals.

**Options:**

**MVP** (~1–2d)
- Reduced navigation mode tied to the profile's audience field: show Overview, Proposals, Sessions only. (~1d)
- Proposal cards hide gate/tag/target internals for the non-developer audience; code snippets render collapsed as clearly-marked "technical detail", untranslated. (~0.5–1d)

**Backlog**
- Fully separate "simple mode" welcome/tutorial rewritten for non-developers (the current `welcome.html` narrative assumes a developer). (~1–2d)
- Simplified proposal vocabulary/actions for the non-developer (what "approve" means in their world). (~1–2d)

**Challenges:**
- Hiding internals must not break the operator: Kevin still needs the full UI when he maintains the machine.

**Approach:**
- Audience gating is a render-time filter on the same data, not a fork; switching the profile back to developer/English restores the full surface.

---

## Decisions / Open Questions

**Decisions:**
- The profile is per-install state, never repo content; the claude-configs repo, logs, and `results.jsonl` stay English (operator surface).
- The profile persists as a file written by install/sync; a config-server edit page is backlog.
- UI i18n is a German overlay scoped to the reduced non-developer surface; a full two-locale catalog is backlog.
- The reduced non-developer navigation is MVP scope — jargon-free wording on top of an internals-exposing UI misses the point.
- The profile governs both interactive Claude sessions on the machine and smine-produced artifacts/UI, via the global-context hook.
- Enforcement is consolidate-as-gate for MVP; a hard verifier before proposals render is backlog.
- Code snippets, ids, dates, tags, and gate fields are never translated.
- The later fdesign plan for the UI flow must include rendered mockups (UI-mockup rule); this concept stays what/why.

**Open Questions:**

_None — all resolved at concept review._
