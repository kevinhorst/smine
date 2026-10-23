# <feature> — Implementation Plan

<args line: role: `master` | `derived`, master: <link to the master plan file, derived only>, mode: `unfamiliar` | `owned`, target: `<tier>`, refuted: `no (plan-mode run)` | `yes` | `yes (no premises)` — per rules/plan.md>

## TLDR
<at most five bullets, each a bold title of at most three words and one line — what is being done, why, what the result is; no tables, no diagrams, no F/D references>

## Scope
<In / Out / Not changed as parent bullets; every item a bold title on its own bullet with one or more sub-bullets naming the thing and linking its decision — never an explanation, never an enumeration on the group line; a discovery outside the request does not enter the plan>

## Decisions
<table: ID | Brief | Decision (brief) | Consequences | Adds | Why | Rejected | Problem [AGENT-ONLY] | Decision [AGENT-ONLY] | Facts [AGENT-ONLY] — one row per decision, `D<n>` IDs as plain text referenced via section-slug links (per rules/plan.md); Problem one coherent paragraph binding implementation, Decision the complete choice; Brief, Decision (brief), Consequences and Why each open with a bold title of at most three words, Brief and Decision (brief) one sentence each; Adds opens with `**Adds**` and counts in the plan rules' vocabulary or `nothing`, then `<br>**Reuses**` and the existing carrier or `nothing` with the reason; Rejected `<br>`-separated bullets `**title** — reason`; Facts links the `A<n>` / `F<n>` ids; every question decided here, `[USER]` only for choices made in the request or as gate feedback; deliberate deviations from exemplar/doc flagged; restructuring existing code: disposal of every old structure and every deliberate adaptation is its own row>

## Assumptions
<table: ID | Assumption | Reality (brief) | Reality [AGENT-ONLY] | Location [AGENT-ONLY] — `A<n>` ids; the premises the request or its named artifacts state vs. repo reality, never a premise the planner adds; this table is the refute ledger; `N/A — <reason>` when the plan rests on no external assumptions>

## Drivers
<table: ID | Origin | Observed | Wanted | Impact | Check — `R<n>` ids, one row per requirement; Origin quotes the user's own prompt words or names the artifact the user handed over (a failing test, a review finding, a plan, the design doc — the binding source), never a label or an agent's note; Observed carries the cause with its path:line; Impact ∈ behavior-preserving | behavioral | contract-touching | no-change; Check ∈ ok | added | no basis | rejected — <reason>, from the driver check, `—` on a derived plan>

## Open questions
<`N/A — fdesign decides every question`>

## Changes
<master plan: the split table Derived plan | Carries | Decisions | Depends on at the design gate, nothing else; every other plan, code gate only: opens with the planned-files table — File | Kind | Entry, every created/modified/deleted file (per rules/plan.md), then the phase table — Phase | Entries | Runs | Needs; every entry: a `location:` line (files only, one repo-relative path per code span, generated files included), then `mirrors:` when an exemplar exists, `ui:` on a UI entry when UI capture ran (the captured PNG and its `design/ui/` source), `hot: <class>` when the entry is in a hot class — its code block is the example implementation — then `phase: <n>`, then `drivers:` naming the `R<n>` rows it serves; one phase's entries touch disjoint files and number at most four, each one agent's work package; an entry changing a type, func signature or package API another entry compiles against stands a phase earlier, and entries in one package share a phase only when neither changes what the other compiles against; phases run in order, the app works after every phase; entries in document order hot-class entries, then other code, then prose entries headed ` [AGENT-ONLY]` (README, skill sources, docs and context), dependency order inside a group, the planned-files table in the same order; exact signatures, full final code for new units, diff blocks from THIS codebase for modified code, self-contained>

## Contracts
<table: Contract | Sides | Sweep — every cross-boundary contract touched, sweeps across all languages/tests/fixtures/docs; consumers from the consumer inventory, grep-to-zero criteria, per-survivor justification — contract-touching drivers land here by design>

## Tests
<unit tests table: Location.Method | Cases | Comment — one case per line in the Cases cell; existing tests as the safety net — which pin behavior, which get updated, what is added; integration tests with the setup each requires; "not tested: X, because Y">

## Verification
<table: ID | Check | Command | Expected | Surface — `V<n>` ids, one row per observable check, Command exactly as typed, Expected the observable pass condition, Surface ∈ local | dev stack | live; first-run recipe rows with the user's real values; degenerate cases their own rows; CI-first repos name the job and its result; no screenshot capture rows; per behavioral driver the changed behavior observed in the running app>

## Current state [AGENT-ONLY]
<filled when the plan restructures existing code: file/line inventory with duplication counts and responsibilities per file; mode `unfamiliar` adds a `Flow trace` subsection: per-component end-to-end trace with real sample values, break point marked; `N/A — <reason>` otherwise>

## Target state [AGENT-ONLY]
<filled when the plan restructures existing code: structure diagram; a Principle block per section: the engineering principle applied and the language/framework mechanism that implements it; `N/A — <reason>` otherwise; a UI-capture run adds a `UI` subsection either way: per changed page its PNG embed and capture command>

## Baseline (verified) [AGENT-ONLY]
<base branch, then the facts table: ID | Fact | Needed for | Location — one fact per row, `F<n>` IDs as plain text, referenced via section-slug links (pivotal facts marked `F<n>!` per rules/plan.md, sorted first; anchor-only facts live in their Changes entry instead), "Needed for" links to the dependent decision/Changes entry, rows ordered by target in document order (decisions, then changes); real data inspected>

## Exemplar & reuse [AGENT-ONLY]
<Reuses table only: Existing | Used for — cross-cutting infrastructure. Mirrors live on each Changes entry as its mirrors: line. One bullet names any change WITHOUT an exemplar — the risk signal>

## Test runbook [AGENT-ONLY]
<default: scenario index — one line per smoke scenario (name, tool/endpoint, data source), no request files; behavioral drivers list the requests they would add/adjust, behavior-preserving drivers name which existing runbook requests re-verify the Not changed items; with the `runbook` arg or an explicit user ask: complete request files in the project's smoke-test tool format (discovered, never assumed), per scenario a location: line under the plan dir's runbooks/ + a fenced tool-native file, usable out of the box, host/auth via the tool's env mechanism only, closing run line; `N/A — <reason>` when no callable surface>

## Changelog
<table: Date | Trigger | What changed — created as `| — | initial | plan created |`; every post-approval edit appends a row (per rules/plan.md)>
