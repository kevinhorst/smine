# Plan Format

**For reviewers / agents:** cite the stable `RULE-PLAN-*` id when flagging a format violation.

Presentation rules for every plan the fdesign skill produces on any route. A plan the user cannot skim is worthless — every section must be checkable at reading speed.

## Language

**RULE-PLAN-001** `[review]` — All plan and doc artifacts are written in English, regardless of the chat or source-document language. Non-English requirements in, English plan out.

## Section order

**RULE-PLAN-002** `[review]` — Every plan, any route: TLDR → Context → Drivers → Scope → Assumptions → Current state → Target state → Behavior contract → Decisions → Open questions → Baseline (verified) → Exemplar & reuse → Changes → Hot items → Tests → Test runbook → Contracts & sweeps → Verification → Stop conditions → Changelog.
**RULE-PLAN-003** `[review]` — Route-owned sections — Drivers, Current state, Target state, Behavior contract (change route); Baseline (verified), Exemplar & reuse (new route) — read exactly `N/A — <other> route` on the other route; they are never omitted.
- Rationale: question → answer → evidence. Scope and decisions frame the plan before the evidence tables that back them. Open questions sits beside Decisions because it indexes the OPEN rows the user must answer before approval.

## TLDR

**RULE-PLAN-004** `[review]` — First section of every plan: half a page max, bullets only — what is being done, why, what the result is. Nothing more.
**RULE-PLAN-005** `[review]` — No tables, no diagrams, no F/D references — it must read standalone, before any other section.
**RULE-PLAN-006** `[review]` — the fdesign refine route keeps it current: a refinement that changes what/why/result updates the TLDR in the same pass.

## Args line

**RULE-PLAN-007** `[review]` — A plan built with non-default intake args records them directly under the title: mode: `owned`, style: `caveman`.
**RULE-PLAN-008** `[review]` — the fdesign refine route keeps the line current — an override at invocation updates it.

## Assumptions

**RULE-PLAN-009** `[review]` — Own section directly after Scope: what the source doc, sketch, or user premise assumed vs. repo reality. Table: `Assumption | Reality | Location`.
**RULE-PLAN-010** `[review]` — `N/A — <reason>` when the plan rests on no external assumptions.

## Text

**RULE-PLAN-011** `[review]` — Bullets, not paragraphs. No prose block over 3 lines — break it into bullets with a bold lead word. This applies inside Changes entries too, not just top-level sections.
**RULE-PLAN-012** `[review]` — Explanatory prose only where a hot item or an OPEN question needs it. Everything else is a bullet, a table row, or code.
**RULE-PLAN-013** `[review]` — One idea per bullet; detail goes into sub-bullets.
**RULE-PLAN-014** `[review]` — No semicolon-chained clauses in bullets: each clause is its own (sub-)bullet. A bullet with 3+ semicolons is a list pretending to be a sentence.
**RULE-PLAN-015** `[review]` — Per-file package descriptions: file name as bullet, responsibilities as sub-bullets — never one run-on line per file.
**RULE-PLAN-016** `[review]` — Scope bullets start with a bold label naming the item (e.g. **labeling-stage:**, **filter:**).
**RULE-PLAN-017** `[review]` — Scope groups (**In:**, **Out:**, **Not changed:**, **Deferred findings:**) are parent bullets; every item is its own sub-bullet — never a semicolon-joined enumeration on the group line.

## Links

**RULE-PLAN-018** `[review]` — IDs (F20, D6) are plain text — never raw-HTML anchors, which the Claude plan viewer renders literally. Every reference to a fact, decision, or section is a markdown link to the containing section's heading auto-slug (`[F20](#baseline-verified)`, `[D6](#decisions)`, `[§4](#changes)`): VS Code navigates it, the Claude viewer shows clean styled text. (`<br>` in table cells stays the one sanctioned HTML tag.)
**RULE-PLAN-019** `[review]` — Every referenced document (design doc, style guide) is a markdown link with its path — never "doc §5" without a target.
**RULE-PLAN-020** `[review]` — Locations are markdown links to path:line where possible so the viewer can open the file; plain text fallback if the renderer doesn't linkify.

## Headlines

**RULE-PLAN-021** `[review]` — No inline code or formatting in headings — plain-language title plus (new | modified).
**RULE-PLAN-022** `[review]` — The first line under a Changes heading is `location:` with each path formatted as inline code.
**RULE-PLAN-023** `[review]` — A Changes entry with an exemplar adds a `mirrors:` line under `location:` naming the sibling it copies.

## Code

**RULE-PLAN-024** `[review]` — Code appears only in fenced blocks with a language tag — never woven into sentences.
**RULE-PLAN-025** `[review]` — An inline code span holds exactly one identifier or path. A sentence needing 3+ spans becomes bullets or a code block.
**RULE-PLAN-026** `[review]` — Code containing backticks (Go struct tags) never goes in an inline span — fenced block only. Inline backtick nesting breaks the plan renderer and mangles the surrounding paragraph.
**RULE-PLAN-027** `[review]` — **Modified existing code = `diff` fenced block**: `+`/`-` lines with 2–3 unchanged context lines — renders green/red. Every diff inside a function includes the enclosing function signature line and a `// ...` marker for elided code; a hunk must be attributable to its method at a glance.
**RULE-PLAN-028** `[review]` — **New standalone units = language-tagged block with the complete final unit** (file / type / function). Never a floating fragment.
**RULE-PLAN-029** `[review]` — Config, TOML, SQL, and JSON changes are shown as the final block content — never described in prose.
**RULE-PLAN-030** `[review]` — JSON/TOML examples are always pretty-printed, one key per line — never multiple keys per line.
**RULE-PLAN-031** `[review]` — **Code is mode-invariant.** Every added function/method appears as its full code block and every modification as its diff block — in every familiarity mode and style. Only complete boilerplate covered by a named exemplar may stay a descriptive bullet; exhaustively: mirrored test skeletons, one-line doc/config edits. Modes compress prose, never code.

## Tables

**RULE-PLAN-032** `[review]` — Cells hold paths, identifiers, or short phrases. Explanations go to bullets under the table, referencing the row.
**RULE-PLAN-033** `[review]` — Multi-clause cells stack their clauses top-to-bottom with `<br>` — never semicolon chains.
**RULE-PLAN-034** `[review]` — A stacked clause leads with its code identifier(s) plus a colon, prose after; **bold** the discriminating words. Example cell:

```markdown
`auctionId` + `bidderRequestId` (1:1): shared by **all offers** of one request<br>`bidId`/`transactionId`: **per offer**
```

**RULE-PLAN-035** `[review]` — Cases cells list one test case per line (`<br>`-separated) — never a semicolon chain.
**RULE-PLAN-036** `[review]` — Never restate in prose what a table row already says.
**RULE-PLAN-037** `[review]` — Inline code is allowed on identifiers in Fact/Decision/Cases cells; Location cells stay plain or are links — never backticked.
**RULE-PLAN-038** `[review]` — Canonical structures:
  - Baseline facts: `ID | Fact | Needed for | Location` — "Needed for" links to the Changes entry or decision that depends on the fact. Rows are ordered by their Needed-for target in document order — decision-facts first (by D number), then change-facts (by § number), then hot-item facts — so the table can be checked side-by-side while scrolling the plan once. The primary target comes first in the cell; IDs stay stable, only row order follows the targets.
  - Assumptions: `Assumption | Reality | Location` (own section after Scope — see Assumptions)
  - Decisions: `ID | Problem | Facts | Decision | Why` — problem first, so each row reads question → answer; "Facts" cites the `F<n>` IDs the decision rests on; "Why" gives the reasoning. User-made decisions get the marker `[USER]` in the Decision cell.
  - Unit tests: `Location.Method | Cases | Comment` — location and method merged, receiver included when known.
  - Contracts & sweeps: `Contract | Sides | Sweep`
  - Stop conditions: `ID | Condition | Action`
  - Exemplar reuses: `Existing | Used for`
  - Changelog: `Date | Trigger | What changed` (see Changelog section)

## Facts relevance (EXPERIMENT — evaluate in next retrospective)

**RULE-PLAN-039** `[review]` — **Pivotal marker**: a fact ID gets `!` (e.g. `F3!`) when the fact (a) **falsifies** an input assumption (memo/concept said otherwise), (b) **decides** — is cited in a Decision's Facts column, or (c) **constrains** — binds a hot item or rules out a mechanism.
**RULE-PLAN-040** `[review]` — Discriminator: what would change if the fact were false? "A decision dies" = pivotal. "Nothing, it just locates code" = anchor.
**RULE-PLAN-041** `[review]` — Pivotal rows sort first within their target group; cross-checking reads only pivotal rows.
**RULE-PLAN-042** `[review]` — Anchor-only facts that exist solely to locate a Changes entry move INTO that Changes entry (its `location:` line) and leave Baseline entirely.

## Open questions & in-plan Q&A

**RULE-PLAN-043** `[review]` — Design questions live INSIDE the plan at the point they bind — never in AskUserQuestion popups. The user needs the plan context to answer; the question must sit where the context is.
**RULE-PLAN-044** `[review]` — Form: an `OPEN` row in the Decisions table — `D<n> | problem | facts | OPEN — options: a) … b) … | why it matters` — or an `> OPEN(Q<n>):` block inside the affected Changes entry.
**RULE-PLAN-045** `[review]` — The `Open questions` section is an index of pointers only (`Q1 → D7`), not a separate list.
**RULE-PLAN-046** `[review]` — An answer converts the row to a `[USER]` decision and appends a Changelog row.

## Changelog

**RULE-PLAN-047** `[review]` — Last section of every plan. Table: `Date | Trigger | What changed`.
**RULE-PLAN-048** `[review]` — One row per event: Q&A resolution (`Q: <question>`), fdesign refine pass (`refine: driver <n>`), post-implementation adjustment via the fdesign change route (`adjust: driver <n>`), local refinement without the full skill (`local: <ask>`).
**RULE-PLAN-049** `[review]` — Any post-approval plan edit appends a row. The plan body is updated in place; history lives only here.
**RULE-PLAN-050** `[review]` — Created empty at plan creation: `| — | initial | plan created |`.

## Caveman mode

**RULE-PLAN-051** `[review]` — Planning skills accept a `caveman` intake arg: plan prose is compressed in the style of the `caveman` skill (technical content byte-perfect).
**RULE-PLAN-052** `[review]` — The style is delegated to the installed skill at `~/.claude/skills/caveman` — a skill invoked with `caveman` stops with "caveman requested but skill not installed" if it is missing; it never approximates the style itself.
**RULE-PLAN-053** `[review]` — Compression removes words **inside** a bullet or cell, never structure: bullets, sub-bullets, and stacked cells are invariant. Merging bullets into semicolon chains is a compression bug, not a feature.

## Exemplar placement

**RULE-PLAN-054** `[review]` — Mirrors live on the Changes entries (`mirrors:` line), not in a standalone mapping — reviewable where they matter.
**RULE-PLAN-055** `[review]` — The Exemplar & reuse section holds only the Reuses table (cross-cutting infrastructure) plus one bullet naming any change WITHOUT an exemplar — that is the risk signal.

## Checklists

**RULE-PLAN-056** `[review]` — Verification is a `- [ ]` checklist, not a numbered list. Every item is a checkable action point: verb-first, with the observable pass condition in the item ("run X — expect Y"), so ticking it means something.
**RULE-PLAN-057** `[review]` — Tool/CLI deliverables include a first-run recipe as checklist items: the exact commands with the user's real sample values, never placeholders.
**RULE-PLAN-058** `[review]` — Degenerate cases (empty input, zero rows, missing config) are explicit verification items, not implied by the happy path.
**RULE-PLAN-059** `[review]` — When the repo has no local runtime, verification is CI-first: items drive a CI run and name the expected job result — never local commands that cannot run.

## Test runbook

**RULE-PLAN-060** `[review]` — Own section between Tests and Contracts & sweeps: the feature's smoke scenarios as complete request files in the project's smoke-test tool format — the tool is taken from the project (existing collection or runbooks, Makefile smoke target, CLAUDE.md/AGENTS.md), never assumed. No discoverable tool → an OPEN row in the plan, not a guess.
**RULE-PLAN-061** `[review]` — Scenarios mirror the DoD End-to-End Verification items: happy path, 1–2 edge cases, error cases — one request file per scenario.
**RULE-PLAN-062** `[review]` — Each entry: a `location:` line with the file's target path under `plans/<feature>/runbooks/`, then a fenced block holding the complete tool-native file — usable out of the box, never prose-described.
**RULE-PLAN-063** `[review]` — Real values from the plan's evidence sections; host and auth go through the tool's environment mechanism — no other placeholders.
**RULE-PLAN-064** `[review]` — The section closes with the run line as a fenced command using the project's tool.
**RULE-PLAN-065** `[review]` — `N/A — <reason>` when the feature has no externally callable surface.

## Diagrams

**RULE-PLAN-066** `[review]` — Sequential/spatial mechanisms (windowing, overlap, pipelines, state machines) get displayed using Mermaid; bullets only annotate the diagrams — never explain such a mechanism in prose alone.

## Length

**RULE-PLAN-067** `[review]` — Context: max 5 bullets — problem, cause (one path:line), the design being implemented, constraints.
**RULE-PLAN-068** `[review]` — Baseline: one fact per row; only facts the plan depends on.

## UI evidence

**RULE-PLAN-069** `[review]` — UI-touching plans embed a screenshot of the **actual UI under design**, captured from the running app (browser pane, simulator); files live under `plans/{slug}/design/ui/` and are embedded with a relative `![...]` image link in Hot items. A rendered mockup stands in only when the UI does not exist yet, and says so.
**RULE-PLAN-070** `[review]` — Every Changes entry that alters user-facing UI carries a `ui:` line (after `location:`/`mirrors:`) linking the screenshot it changes — the code delta and the picture are reviewed together.
