# Plan Format

**For reviewers / agents:** cite the stable `RULE-FMT-*` id when flagging a format violation.

Presentation rules for every plan the fdesign and fchange skills produce. A plan the user cannot skim is worthless — every section must be checkable at reading speed.

## Language

- **RULE-FMT-001** — All plan and doc artifacts are written in English, regardless of the chat or source-document language. Non-English requirements in, English plan out.

## Section order

- **RULE-FMT-002** — Feature plans: TLDR → Context → Scope → Assumptions → Decisions → Baseline (verified) → Exemplar & reuse → Changes → Hot items → Tests → Test runbook → Contracts & sweeps → Verification → Stop conditions → Open questions → Changelog.
- **RULE-FMT-003** — Refactor plans: TLDR → Context → Scope → Assumptions → Current state → Target state → Behavior contract → Decisions → Changes → Hot items → Tests → Test runbook → Contracts & sweeps → Verification → Stop conditions → Open questions → Changelog.
- Rationale: question → answer → evidence. Scope and decisions frame the plan before the evidence tables that back them.

## TLDR

- **RULE-FMT-004** — First section of every plan: half a page max, bullets only — what is being done, why, what the result is. Nothing more.
- **RULE-FMT-005** — No tables, no diagrams, no F/D references — it must read standalone, before any other section.
- **RULE-FMT-006** — the fdesign refine route keeps it current: a refinement that changes what/why/result updates the TLDR in the same pass.

## Args line

- **RULE-FMT-007** — A plan built with non-default intake args records them directly under the title: mode: `owned`, style: `caveman`.
- **RULE-FMT-008** — the fdesign refine route keeps the line current — an override at invocation updates it.

## Assumptions

- **RULE-FMT-009** — Own section directly after Scope: what the source doc, sketch, or user premise assumed vs. repo reality. Table: `Assumption | Reality | Location`.
- **RULE-FMT-010** — `N/A — <reason>` when the plan rests on no external assumptions.

## Text

- **RULE-FMT-011** — Bullets, not paragraphs. No prose block over 3 lines — break it into bullets with a bold lead word. This applies inside Changes entries too, not just top-level sections.
- **RULE-FMT-012** — Explanatory prose only where a hot item or an OPEN question needs it. Everything else is a bullet, a table row, or code.
- **RULE-FMT-013** — One idea per bullet; detail goes into sub-bullets.
- **RULE-FMT-014** — No semicolon-chained clauses in bullets: each clause is its own (sub-)bullet. A bullet with 3+ semicolons is a list pretending to be a sentence.
- **RULE-FMT-015** — Per-file package descriptions: file name as bullet, responsibilities as sub-bullets — never one run-on line per file.
- **RULE-FMT-016** — Scope bullets start with a bold label naming the item (e.g. **labeling-stage:**, **filter:**).
- **RULE-FMT-017** — Scope groups (**In:**, **Out:**, **Not changed:**, **Deferred findings:**) are parent bullets; every item is its own sub-bullet — never a semicolon-joined enumeration on the group line.

## Links

- **RULE-FMT-018** — Every reference to a fact, decision, or section (F20, D6, §11) is an internal markdown link to an anchor; IDs carry an `<a id="f20"></a>`-style anchor in their table cell.
- **RULE-FMT-019** — Every referenced document (design doc, style guide) is a markdown link with its path — never "doc §5" without a target.
- **RULE-FMT-020** — Locations are markdown links to path:line where possible so the viewer can open the file; plain text fallback if the renderer doesn't linkify.

## Headlines

- **RULE-FMT-021** — No inline code or formatting in headings — plain-language title plus (new | modified).
- **RULE-FMT-022** — The first line under a Changes heading is `location:` with each path formatted as inline code.
- **RULE-FMT-023** — A Changes entry with an exemplar adds a `mirrors:` line under `location:` naming the sibling it copies.

## Code

- **RULE-FMT-024** — Code appears only in fenced blocks with a language tag — never woven into sentences.
- **RULE-FMT-025** — An inline code span holds exactly one identifier or path. A sentence needing 3+ spans becomes bullets or a code block.
- **RULE-FMT-026** — Code containing backticks (Go struct tags) never goes in an inline span — fenced block only. Inline backtick nesting breaks the plan renderer and mangles the surrounding paragraph.
- **RULE-FMT-027** — **Modified existing code = `diff` fenced block**: `+`/`-` lines with 2–3 unchanged context lines — renders green/red. Every diff inside a function includes the enclosing function signature line and a `// ...` marker for elided code; a hunk must be attributable to its method at a glance.
- **RULE-FMT-028** — **New standalone units = language-tagged block with the complete final unit** (file / type / function). Never a floating fragment.
- **RULE-FMT-029** — Config, TOML, SQL, and JSON changes are shown as the final block content — never described in prose.
- **RULE-FMT-030** — JSON/TOML examples are always pretty-printed, one key per line — never multiple keys per line.
- **RULE-FMT-031** — **Code is mode-invariant.** Every added function/method appears as its full code block and every modification as its diff block — in every familiarity mode and style. Only complete boilerplate covered by a named exemplar may stay a descriptive bullet; exhaustively: mirrored test skeletons, one-line doc/config edits. Modes compress prose, never code.

## Tables

- **RULE-FMT-032** — Cells hold paths, identifiers, or short phrases. Explanations go to bullets under the table, referencing the row.
- **RULE-FMT-033** — Multi-clause cells stack their clauses top-to-bottom with `<br>` — never semicolon chains.
- **RULE-FMT-034** — A stacked clause leads with its code identifier(s) plus a colon, prose after; **bold** the discriminating words. Example cell:

```markdown
`auctionId` + `bidderRequestId` (1:1): shared by **all offers** of one request<br>`bidId`/`transactionId`: **per offer**
```

- **RULE-FMT-035** — Cases cells list one test case per line (`<br>`-separated) — never a semicolon chain.
- **RULE-FMT-036** — Never restate in prose what a table row already says.
- **RULE-FMT-037** — Inline code is allowed on identifiers in Fact/Decision/Cases cells; Location cells stay plain or are links — never backticked.
- **RULE-FMT-038** — Canonical structures:
  - Baseline facts: `ID | Fact | Needed for | Location` — "Needed for" links to the Changes entry or decision that depends on the fact. Rows are ordered by their Needed-for target in document order — decision-facts first (by D number), then change-facts (by § number), then hot-item facts — so the table can be checked side-by-side while scrolling the plan once. The primary target comes first in the cell; IDs stay stable, only row order follows the targets.
  - Assumptions: `Assumption | Reality | Location` (own section after Scope — see Assumptions)
  - Decisions: `ID | Problem | Facts | Decision | Why` — problem first, so each row reads question → answer; "Facts" cites the `F<n>` IDs the decision rests on; "Why" gives the reasoning. User-made decisions get the marker `[USER]` in the Decision cell.
  - Unit tests: `Location.Method | Cases | Comment` — location and method merged, receiver included when known.
  - Contracts & sweeps: `Contract | Sides | Sweep`
  - Stop conditions: `ID | Condition | Action`
  - Exemplar reuses: `Existing | Used for`
  - Changelog: `Date | Trigger | What changed` (see Changelog section)

## Facts relevance (EXPERIMENT — evaluate in next retrospective)

- **RULE-FMT-039** — **Pivotal marker**: a fact ID gets `!` (e.g. `F3!`) when the fact (a) **falsifies** an input assumption (memo/concept said otherwise), (b) **decides** — is cited in a Decision's Facts column, or (c) **constrains** — binds a hot item or rules out a mechanism.
- **RULE-FMT-040** — Discriminator: what would change if the fact were false? "A decision dies" = pivotal. "Nothing, it just locates code" = anchor.
- **RULE-FMT-041** — Pivotal rows sort first within their target group; cross-checking reads only pivotal rows.
- **RULE-FMT-042** — Anchor-only facts that exist solely to locate a Changes entry move INTO that Changes entry (its `location:` line) and leave Baseline entirely.

## Open questions & in-plan Q&A

- **RULE-FMT-043** — Design questions live INSIDE the plan at the point they bind — never in AskUserQuestion popups. The user needs the plan context to answer; the question must sit where the context is.
- **RULE-FMT-044** — Form: an `OPEN` row in the Decisions table — `D<n> | problem | facts | OPEN — options: a) … b) … | why it matters` — or an `> OPEN(Q<n>):` block inside the affected Changes entry.
- **RULE-FMT-045** — The `Open questions` section is an index of pointers only (`Q1 → D7`), not a separate list.
- **RULE-FMT-046** — An answer converts the row to a `[USER]` decision and appends a Changelog row.

## Changelog

- **RULE-FMT-047** — Last section of every plan. Table: `Date | Trigger | What changed`.
- **RULE-FMT-048** — One row per event: Q&A resolution (`Q: <question>`), fdesign refine pass (`refine: driver <n>`), post-implementation adjustment via fchange (`adjust: driver <n>`), local refinement without the full skill (`local: <ask>`).
- **RULE-FMT-049** — Any post-approval plan edit appends a row. The plan body is updated in place; history lives only here.
- **RULE-FMT-050** — Created empty at plan creation: `| — | initial | plan created |`.

## Caveman mode

- **RULE-FMT-051** — Planning skills accept a `caveman` intake arg: plan prose is compressed in the style of the `caveman` skill (technical content byte-perfect).
- **RULE-FMT-052** — The style is delegated to the installed skill at `~/.claude/skills/caveman` — a skill invoked with `caveman` stops with "caveman requested but skill not installed" if it is missing; it never approximates the style itself.
- **RULE-FMT-053** — Compression removes words **inside** a bullet or cell, never structure: bullets, sub-bullets, and stacked cells are invariant. Merging bullets into semicolon chains is a compression bug, not a feature.

## Exemplar placement

- **RULE-FMT-054** — Mirrors live on the Changes entries (`mirrors:` line), not in a standalone mapping — reviewable where they matter.
- **RULE-FMT-055** — The Exemplar & reuse section holds only the Reuses table (cross-cutting infrastructure) plus one bullet naming any change WITHOUT an exemplar — that is the risk signal.

## Checklists

- **RULE-FMT-056** — Verification is a `- [ ]` checklist, not a numbered list. Every item is a checkable action point: verb-first, with the observable pass condition in the item ("run X — expect Y"), so ticking it means something.
- **RULE-FMT-057** — Tool/CLI deliverables include a first-run recipe as checklist items: the exact commands with the user's real sample values, never placeholders.
- **RULE-FMT-058** — Degenerate cases (empty input, zero rows, missing config) are explicit verification items, not implied by the happy path.
- **RULE-FMT-059** — When the repo has no local runtime, verification is CI-first: items drive a CI run and name the expected job result — never local commands that cannot run.

## Test runbook

- **RULE-FMT-060** — Own section between Tests and Contracts & sweeps: the feature's smoke scenarios as complete request files in the project's smoke-test tool format — the tool is taken from the project (existing collection or runbooks, Makefile smoke target, CLAUDE.md/AGENTS.md), never assumed. No discoverable tool → an OPEN row in the plan, not a guess.
- **RULE-FMT-061** — Scenarios mirror the DoD End-to-End Verification items: happy path, 1–2 edge cases, error cases — one request file per scenario.
- **RULE-FMT-062** — Each entry: a `location:` line with the file's target path under `plans/<feature>/runbooks/`, then a fenced block holding the complete tool-native file — usable out of the box, never prose-described.
- **RULE-FMT-063** — Real values from the plan's evidence sections; host and auth go through the tool's environment mechanism — no other placeholders.
- **RULE-FMT-064** — The section closes with the run line as a fenced command using the project's tool.
- **RULE-FMT-065** — `N/A — <reason>` when the feature has no externally callable surface.

## Diagrams

- **RULE-FMT-066** — Sequential/spatial mechanisms (windowing, overlap, pipelines, state machines) get displayed using Mermaid; bullets only annotate the diagrams — never explain such a mechanism in prose alone.

## Length

- **RULE-FMT-067** — Context: max 5 bullets — problem, cause (one path:line), the design being implemented, constraints.
- **RULE-FMT-068** — Baseline: one fact per row; only facts the plan depends on.
