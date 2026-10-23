# Navigation & Scoping

Ambient doctrine for orienting in and extending a repo — every session, every task. Cite the entry
ID when flagging a violation.

**ACTION-NAV-001** `[prose]` — Never scan the entire repository.

* Why: full scans burn context on irrelevant files; the context docs and targeted search answer faster.
* Applies: any repo exploration.

**ACTION-NAV-002** `[prose]` — Always orient via the context docs (rules, facts, style guides) before exploring the code.

* Why: context facts are verified ground truth; re-deriving them wastes context and drifts.
* Applies: session start and any unfamiliar area.

**ACTION-NAV-003** `[prose]` — Always read the nearest sibling (service, model, test file) before writing new code — new code mirrors its architecture.

* Why: the sibling is the golden path; deviating creates a second pattern reviewers must reconcile.
* Applies: any new file, type, or endpoint.

**ACTION-NAV-004** `[prose]` — Always prefer existing implementations over new ones.

* Why: duplicate mechanisms double the maintenance surface and diverge.
* Applies: any new helper, utility, or abstraction.

**ACTION-NAV-005** `[prose]` — Always use `jq` for JSON/JSONL extraction — `python3 -c` / `python -c` is permission-denied by the harness (`Bash(python3 -c *)` and `Bash(python -c *)` in the settings `deny` list): the call fails outright, it is not promptable. `jq` is allowlisted (`Bash(jq *)`); the `jq` skill covers field extraction from large files.

* Why: jq is inspectable in one line and leaves no script litter; before the deny, ad-hoc `python3 -c "import json ..."` was the dominant permission-prompt source across agents — every use burned a manual grant.
* Applies: any JSON/JSONL extraction — repo JSON artifacts, tool outputs, eval run files, config files.
* Example: `python3 -c "import json; d=json.load(open('acdsl/registry.json')); print(json.dumps(d.get('gofmt'), indent=2))"` → `jq '.gofmt' acdsl/registry.json`.

**ACTION-NAV-006** `[prose]` — Treat generated, gitignored artifacts as part of the codebase: consult the repo's `FACT-REPO-GEN-*` facts for what is generated and how to produce it before concluding a file or package is missing.

* Why: generated models are invisible to git-based grounding; an agent that misses them re-implements them or misreads the build.
* Applies: session start and any missing-file or missing-package conclusion.

**ACTION-NAV-007** `[prose]` — Prune `vendor/`, `bin/`, `node_modules/`, `*.gen.go`, and other generated trees from sweeps; never grep the repo's own high-frequency domain keyword.

* Why: unpruned sweeps and a domain-keyword grep match everything and overflow the output; large-branch reviews drown in vendored/generated diffs. Operationalizes ACTION-NAV-001.
* How: pre-scope the vendored and generated trees before reading; grep for the specific symbol, never the repo's name or its central noun.
* Applies: any repo-wide sweep or large-branch review.

**ACTION-NAV-008** `[prose]` — A rename or group-move sweep drives to zero every human-readable path reference in docs and READMEs, not just code and slash-command names.

* Why: a doc path pointing at a pre-refactor location (`.claude/commit/SKILL.md` after a rename; `skills/util/peek` after a move) is drift; a slug-only sweep leaves it standing.
* How: enumerate path-shaped references as part of the grep-to-zero close, alongside code references and command names.
* Applies: any skill/asset rename or move between directories.

**ACTION-NAV-009** `[prose]` — An active (non-archived) plan left referencing a renamed or deleted asset is drift — record the staleness in that plan's Changelog.

* Why: an unflagged stale reference makes a stale-by-decision plan indistinguishable from an unfinished sweep.
* How: when a rename or deletion sweep consciously leaves an active plan pointing at the old name, add a Changelog note rather than leaving it silently wrong.
* Applies: any rename or deletion sweep that leaves an active plan untouched by decision.

**ACTION-NAV-010** `[prose]` — Never call an IDE MCP server's tools — `goland`, `pycharm` (`mcp__<server>__*`), database tools included — unless the user's own message names that server; otherwise use Read/Grep/Glob, Edit/Write and the CLI clients, even when the server is connected. No skill opts a run into an IDE server — it declares no IDE arg, and its prompts carry no IDE line.

* Why: IDE servers are opt-in arms, not ambient tooling — per-worktree index startup, language-server config drift, duplicate workspace-folder results and an IDE that must be open make them unreliable as a default; the opt-in is the user's per session, not a skill's, and an eval arm switches a server off itself (`/skillroutine-ab` `no-goland` variants), so IDE-vs-grep comparisons stay attributable.
* Applies: any symbol lookup, reference search, rename, symbol-level edit, IDE file operation or database query, in skill runs and ad-hoc sessions alike; one named server opts in that server only.
* A spawned subagent is opted in only when its spawning prompt says so in the user's words — the user's message never reaches it otherwise.
* Example: "use goland for the rename" → `mcp__goland__search_symbol` is allowed, `mcp__pycharm__*` is not; `/fimplement plans/2026-09-28-x-7ea30b5/2026-09-28-x-wip-7ea30b5/design/x.md` alone → `git grep -n "renameInstruction("`, then Read the hits.

**ACTION-NAV-011** `[prose]` — With an IDE server opted in, route symbol questions (references, implementations, declarations, renames, symbol-body edits) to it and keep text questions (string literals, paths, prose, config keys, files in languages it does not index) on grep.

* Why: the symbol index is exact where grep misses call sites, multi-file renames above all, but blind to text — skill markdown, context docs, JSON and shell references are grep's job, and the grep-to-zero close (ACTION-NAV-008) sweeps text by definition.
* Applies: runs where ACTION-NAV-010 admitted a server; an IDE tool that errors or returns empty for an indexed symbol falls back to grep, stated in the output.
* Example: "use goland" before renaming `renameInstruction` → `mcp__goland__rename_refactoring`, then `git grep -n "renameInstruction"` over `*.md`, `*.json`, `*.sh` for the mentions the index cannot see.

**ACTION-NAV-012** `[prose]` — Read what the read gate owes before the first gated call, each owed file alone, never inside a parallel batch: `global.md` and the injection files the hook pointers name, then the language guide of every file the next calls touch (the `**Files:**` globs of the context rules — `go.md` for `*.go`, `go_test.md` for `*_test.go`); follow a denial's Read lines exactly, then retry.

* Why: the gate judges one call at a time and a parallel batch is already committed — one miss denies every sibling through the pending guide, and the guide stays owed on every gated call until it is read in full; session e2513024 spent 10 of its 13 read-gate denials on batch siblings of 2 misses.
* Applies: every root and subagent, the first calls of a turn above all; Bash, Read, Edit, Write, Grep, Glob and NotebookEdit are gated, tool loading and the session, peek and ccd tools are not.
* Example: a diagnosis about to touch `cmd/agentwt/src/gitstate.rs` and `cmd/rules/mergeskillcue.go` → Read `rust.md`, then `go.md`, each alone, then the batch; wrong: the batch first, seven denials, then the guides.
* Reach: always

**ACTION-NAV-GROUND-001** `[prose]` — Ground through exactly one round of parallel Explore subagents, one per area, at most as many as the running skill allows; your own reads fill only the gaps that round leaves — never ground the areas yourself, never a second round.

* Why: serial grounding piles every read into one context, so the round returns conclusions instead of file dumps; an unbounded fan-out is the opposite failure — an fdesign run spent six agents in two rounds and 1.84M subagent tokens on grounding before its first decision.
* Applies: grounding phases of plan, concept, exploration and diagnosis skills, in root sessions and in subagents that still hold the Agent tool (depth ≤ 2, ACTION-IMPL-HARNESS-002); the size is the skill's own (fdesign: `subagents: <n>[<model>:<effort>]`, three agents without it).
* Applies: grounding agents run on an Opus model, never on any Sonnet model — Sonnet is unsuited to grounding; only a model the invocation names explicitly (fdesign `subagents: 3[sonnet:low]`) overrides this.
* Example: heartbeat P1 fdesign as a serial subagent — 44 minutes, 430k context, no plan text written; the root-session rerun fanned grounding out to Explore agents and had first plan text after 2 minutes.

**ACTION-NAV-GROUND-002** `[prose]` — Write the artifact skeleton — every catalog heading — as soon as scope and main decisions are known, then fill sections as grounding reports land.

* Why: a written skeleton turns grounding into gap-filling; grounding "until complete" before writing has no end condition.
* Applies: any artifact on the plan catalog (plans, concepts, explorations), grounded per ACTION-NAV-GROUND-001.
* Example: the P1 rerun wrote every heading with `<pending>` after the first reports, then replaced one section per Edit as later reports arrived; the serial subagent kept "checking details" and never wrote.

**ACTION-NAV-GROUND-003** `[prose]` — Fill a missing or unverified detail with your own read and fold it into the owning section — never a further agent, never a second grounding round.

* Why: a gap is a line range, a field name or one probe — a handful of reads; an agent per gap re-reads the area from zero, and gap rounds have no end condition.
* Applies: gaps found after the skeleton exists (ACTION-NAV-GROUND-002), with or without a granted grounding round before them.
* Example: a plan missing the free listen port → the planner greps the config server and peek ports itself and writes the port into the Names row; wrong: one more Explore agent "to check the ports".

