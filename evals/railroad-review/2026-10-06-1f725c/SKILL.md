---
name: railroad-review
description: Review code changes against conventions, specs and definition-of-done — parallel directions, one station review. Trigger on /railroad-review or "review this branch/PR/my changes". Args — title; directions; lanes: n[model:effort]; station; scope: branch|wip|all; paths; spec; focus; refute/ignore/autofix-level; done-after; chunks; walk-group; expect-base|head-red; nightly; rounds; run-id; overlay-patch; exclude; prebuilt; known; verbose; goland.
author: Andreas Geßner
version: 24.1.0
title-code: RR
format: 2
acdsl-context: ACTION-CONCEPT-HOT-*, ACTION-IMPL-INTEG-*, ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014, FACT-REPO-*
allowed-tools: Edit, Read, Skill(railroad-review), ToolSearch, Workflow, Write, Bash(*/bin/acdsl check *), Bash(*/bin/verifiers/*), Bash(./bin/acdsl check *), Bash(bin/acdsl check *), Bash(cp *), Bash(git add*), Bash(git apply *), Bash(git branch*), Bash(git checkout -B *), Bash(git diff*), Bash(git format-patch *), Bash(git init *), Bash(git log*), Bash(git merge-base*), Bash(git rev-list*), Bash(git rev-parse*), Bash(git show *), Bash(git status*), Bash(git worktree add*), Bash(git worktree list*), Bash(git worktree remove*), Bash(go * build *), Bash(go * test *), Bash(go * vet *), Bash(go run *), Bash(go tool cover *), Bash(jq *), Bash(make *), Bash(mkdir *), Bash(mv *), Bash(smine-rules base *), Bash(smine-rules buildtest *), Bash(smine-rules cleanup *), Bash(smine-rules entries *), Bash(smine-rules lanes *), Bash(smine-rules ledger *), Bash(smine-rules pairs *), Bash(smine-rules present *), Bash(smine-rules probe-replay *), Bash(smine-rules provision *), Bash(smine-rules prune *), Bash(smine-rules reset-round *), Bash(smine-rules snapshot *), Bash(smine-rules walk *)
argument-hint: 'scheduler rewrite review directions: tests security contracts lanes: 2[opus:medium] station: opus:high scope: branch[main] paths: internal/server cmd/*.go exclude: plans/ sessions/ spec: plans/2026-09-07-foo-wip-abc1234/design/foo.md focus: "goroutine lifecycle in the scheduler" refute-level: major ignore-level: none autofix-level: minor done-after: 2 chunks: layers[50:400] walk-group: 10 expect-base-red expect-head-red nightly rounds: 2 run-id: 2026-09-27-a34281-a overlay-patch: overlays/review-context.patch prebuilt: bin/acdsl bin/verifiers known: reviews/known.json verbose goland'
branch-prefix: claude-review/
---

# Railroad Review

Structural review against project's standards, specs, and plans. Directions — independent review lenses — fan out to parallel
lanes through station protocol. Session running this skill is **dispatcher**: schedules rounds, holds human gate, applies
fixes; never reviews lane itself. Review standards live in project's context entries; design rationale lives in this skill's
README.md.

## Routing

**Purpose:** reviews any code change — PRs, branches, commits, uncommitted work, untracked files — against project conventions,
specs, style guides, definition-of-done.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| verdict on existing, unchanged code | /code-verdict |

**Preconditions:**
- non-plan-mode session with the Workflow tool, headless `claude -p` included ([Runnability
  gate](#skill-railroadreview-modes-002))
- project's context docs deployed
- branch/all scope: resolvable diff base
- correctness direction: spec, plan, or request doc — **dropped** per [Spec required](#skill-railroadreview-dir-004) without one

**Pipeline:** standalone; **railroad-review** → /dod-report *(optional)*

## Args

| Arg | Form | Default | Effect |
| :-- | :--- | :------ | :----- |
| `title` | free text, first | generic session name | names the run — chat title and initial branch derive from it |
| `directions` | space list | defaults + dynamic | binding direction-set override — [Direction selection](#skill-railroadreview-modes-004) |
| `lanes` | `<n>[<model>:<effort>]` | `2`, default subagent model (`opus`) | lanes per direction; bracket governs judgment workers — [Bracket vocabulary](#skill-railroadreview-modes-017) |
| `station` | `<model>[:<effort>]` | `opus`, high | station-merge model override — [Bracket vocabulary](#skill-railroadreview-modes-017) |
| `scope` | `branch[<base>]` \| `wip` \| `all[<base>]` | `branch` clean tree, `all` dirty | what to review — [Scope first](#skill-railroadreview-scope-001) |
| `paths` | pathspec list | whole diff | narrows any scope to named files/dirs — [Diff](#skill-railroadreview-proc-004) |
| `exclude` | pathspec list | absent | pathspecs left out of every review diff — [Diff](#skill-railroadreview-proc-004) |
| `spec` | doc path list | [Direction table](#skill-railroadreview-dir-003) chain | correctness spec docs, one checklist over all; none at all drops direction — [Spec required](#skill-railroadreview-dir-004) |
| `focus` | quoted subject | direction off | special-focus subject — [Direction table](#skill-railroadreview-dir-003) |
| `refute-level` | `blocker\|major\|minor\|nit\|info\|none` | `major` | minimum severity getting a fresh refuter — [Refutation](#skill-railroadreview-round-007) |
| `ignore-level` | same vocabulary | `none` | severity at/below exits at dedup into the round's tail — never refuted, staged or fixed — [Tail exit](#skill-railroadreview-round-034) |
| `autofix-level` | same vocabulary | `minor` | highest severity the fix workflow fixes; human-routed findings stay open — [Fix scope](#skill-railroadreview-station-005b) |
| `done-after` | `<n>` | per-direction map | uniform consecutive-clean count clearing a file for every direction — [Done rule](#skill-railroadreview-modes-016b) |
| `chunks` | `<mode>[maxFiles:maxLines]` | auto over bounds | partitioned fan-out — [Chunked fan-out](#skill-railroadreview-modes-005a) |
| `walk-group` | `<n>` | `10` | checklist group ceiling, passed as `--ceiling` — [Lane manifests](#skill-railroadreview-modes-003d) |
| `expect-base-red` | flag | absent | skips the base build and test — [Base health](#skill-railroadreview-proc-009) |
| `expect-head-red` | flag | absent | skips the head build and test each round; nothing caps the recommendation — [Build and test](#skill-railroadreview-round-016) |
| `nightly` | flag | absent | unattended headless — rounds loop to the cap, no per-round go — [Unattended rounds](#skill-railroadreview-nightly-001a) |
| `rounds` | `<n>` | `2` | nightly round cap — [Unattended rounds](#skill-railroadreview-nightly-001a) |
| `run-id` | token | `rr-<6 hex>` | seeds this run's `runNs` — [Run namespace](#skill-railroadreview-nightly-007) |
| `overlay-patch` | path | absent | explicit worktrees with this review-context overlay applied — [Explicit worktrees](#skill-railroadreview-modes-019) |
| `prebuilt` | path list | absent | repo-relative built paths linked into every worker worktree — [Explicit worktrees](#skill-railroadreview-modes-019) |
| `known` | path | absent | known findings of an earlier run: never re-reported, never written — [Lane manifests](#skill-railroadreview-modes-003d) |
| `verbose` | flag | absent | keeps the lane reports and per-finding verdict files at review end — [Artifact prune](#skill-railroadreview-round-036) |
| `goland` | flag | absent | opt dispatcher and every worker into GoLand MCP tools, via stage arg `ide` — [Stage files](#skill-railroadreview-modes-003a) |

## Definitions

**SKILL-RAILROADREVIEW-DIR-001** `[def]` **Direction**

One review lens.

**SKILL-RAILROADREVIEW-DIR-001a** `[def]` **Lane**

One agent, one direction, one chunk.

**SKILL-RAILROADREVIEW-DIR-001b** `[def]` **Dispatcher**

Session running this skill; never reviews a lane.

**SKILL-RAILROADREVIEW-DIR-001c** `[def]` **Default directions**

The set code-style, correctness, critical; each fans to `lanes` lanes.

**SKILL-RAILROADREVIEW-SCOPE-001a** `[def]` **Branch scope**

Committed range, fork point to the round's pinned head.

**SKILL-RAILROADREVIEW-SCOPE-001b** `[def]` **Wip scope**

Staged + unstaged + untracked snapshot, base = HEAD.

**SKILL-RAILROADREVIEW-SCOPE-001c** `[def]` **All scope**

Fork point to working tree incl. untracked, snapshot.

**SKILL-RAILROADREVIEW-PROC-008a** `[def]` **Partition bounds**

50 files / 4000 changed lines, counted after scope exclusions; `chunks` bracket overrides; no total review cap.

**SKILL-RAILROADREVIEW-MODES-017** `[def]` **Bracket vocabulary**

`lanes`/`station` tokens: `<model>:<effort>`, either side optional; effort ∈ `low | medium | high | xhigh | max`; absent model ⇒
default subagent model (`opus`), absent effort ⇒ session.
* param: model = subagent alias from generated vocabulary (`opus`, `sonnet` today); full CLI id reduces to family alias, stated
  — workflow agents: aliases only.
* param: `lanes` bracket governs judgment workers — lanes, groupers, refuters, verifiers; `station:` station alone; default
  station effort high.

**SKILL-RAILROADREVIEW-DIR-003** `[def]` **Direction table**

Each direction = goal + context requirement — standards themselves are context, never restated here:

| Direction | Goal | Context |
| :--- | :--- | :--- |
| `code-style` | Naming, idioms, structure match surrounding code. | [reviewing chapter's quality entries](context/actions/reviewing.md); repo's style rules for change's language |
| `correctness` | Production code does exactly what spec requires — nothing missing, nothing nobody asked for; test files excluded. | spec/plan/request: invocation-named, else plan dir's plan file `design/<slug>.md` (resolved per [plan-dir resolution rule](context/rules/plan.md)); [reviewing chapter's spec entries](context/actions/reviewing.md) |
| `critical` | No high-risk defect class ships. | [concepting chapter's hot-class entries](context/actions/concepting.md) (fallback one-liner: concurrency misuse, nil derefs, weakened guards, resource leaks) |
| `special-focus` | Whatever focus argument names. | `focus` arg |
| `data-integrity` | State-bearing changes keep integrity doctrine. | [implementing chapter's data-integrity entries](context/actions/implementing.md) (fallback one-liner: append-only migrations, no timer-driven transitions, claim-before-send, exhaustive enums) |
| `contracts` | Cross-boundary contract change lands on every side — no leftover old names anywhere. | repo-wide search, all languages; client backward-compatibility DoD entry |
| `tests` | Changed code covered by on-pattern tests; spec-named tests present. | [reviewing chapter's test entries](context/actions/reviewing.md); each test with its production partners; spec |
| `security` | Only quantified, reproducible exploits. | [reviewing chapter's security entries](context/actions/reviewing.md) |

**SKILL-RAILROADREVIEW-DIR-005** `[def]` **Machine mirror**

`context-map.json` in this skill's directory is injector's source per direction; Directions table is its human twin.
* param: change to one is change to both.

**SKILL-RAILROADREVIEW-WALK-001** `[def]` **Walk**

A lane's checklist: the manifest's `walk` object — `shape` plus `groups` of at most `walk-group` items; the lane answers one
group at a time and records each answer as a `walk` entry in its artifact.

**SKILL-RAILROADREVIEW-WALK-002** `[def]` **Walk shape**

`file-first` (code-style, tests, security, data-integrity, critical): one answer per assigned file × group. `group-first`
(correctness, contracts): one answer per group over the whole diff.
* param: file-first items are context-rule ids.
* param: group-first items are `"<direction>-<n>: <text>"` strings, requirements or boundaries, `n` stable for the run.

## Global rules

**SKILL-RAILROADREVIEW-EXEC-002** `[rule][todo:hook-needed]` **Never prompt**

Review NEVER calls AskUserQuestion nor stops for input; resolve every input, state assumption, carry on.
* param: genuinely unresolvable ends review with report.
* param: sole sanctioned pause — the interactive end-of-round STOP.

**SKILL-RAILROADREVIEW-EXEC-003a** `[rule]` **Preloaded tools**

Dispatcher MUST load every tool railroad needs before round 1.

**SKILL-RAILROADREVIEW-EXEC-003b** `[rule]` **Paths, never content**

Station handoffs MUST be consumed via targeted jq over `review.json`; ledger and risk map MUST travel as files.
* param: dispatcher reads stay limited to gate presentation and fix routing.

**SKILL-RAILROADREVIEW-EXEC-003d** `[rule]` **Workers return refs**

Every worker MUST return ids, cells, counts and paths — claim bodies, verdict evidence and write-ups travel by file, read with
jq.

**SKILL-RAILROADREVIEW-EXEC-003c** `[rule]` **No dispatcher hunks**

Dispatcher NEVER reads or writes diff hunk content; hunks stay exclusively lane-side.
* param: file list, partition bounds, risk map derive from `git diff --name-only` and `--numstat` only.

**SKILL-RAILROADREVIEW-EXEC-003e** `[rule]` **No copy-back**

Agents NEVER re-emit data another component holds — a file is written by the program that computes it or the agent that authors
it.
* param: the Workflow args are the one model-emitted channel into a workflow — refs and small lists only.

**SKILL-RAILROADREVIEW-EXEC-004** `[rule]` **Foreground commands**

Dispatcher MUST run every `smine-rules` command in the foreground; a launched Workflow resumes the round through its completion
notification.
* param: never piped or chained — a capture anchor skips a command holding `|`, `;`, `&`, a backtick or `$(`.

**SKILL-RAILROADREVIEW-MODES-014a** `[rule]` **Budget invariance**

NEVER skip or thin refutation, dedup, verification or station under budget pressure.
* param: never widen chunk bounds past arg/defaults to save money.

**SKILL-RAILROADREVIEW-MODES-014c** `[rule]` **No budget mentions**

Worker prompt NEVER mentions remaining budget.

**SKILL-RAILROADREVIEW-CTX-003** `[rule]` **Missing context**

Context source a direction requires but cannot find MUST become a blocker finding, never a silent skip.

**SKILL-RAILROADREVIEW-CTX-004** `[rule]` **Gate evidence**

In ACDSL repos gate output is review evidence — directions MUST skip what gates enforce and cite rule IDs.

**SKILL-RAILROADREVIEW-DIR-004** `[rule]` **Spec required**

With no spec, plan, or request doc the correctness direction MUST be dropped, named as dropped.
* param: ticket text or stated request counts as minimal spec.

**SKILL-RAILROADREVIEW-DIR-002** `[quality][todo:unmeasured]` **Falsifiability bar**

Every direction except code-style: a claim MUST name concrete input, state, or sequence producing wrong behaviour; opinions and
unquantified robustness die at lane.

**SKILL-RAILROADREVIEW-TPL-008** `[payload]` **Primer hop**

A stage launch after a typed user message: LAUNCH this script inline as `script`, END the turn, RELAUNCH the stage in the turn
its completion notification opens:

```js
export const meta = { name: 'relay-primer', description: 'SMINE-PRIMER — opens a notification turn' }
await new Promise(r => setTimeout(r, 10000))
return 'primed'
```

## Context wiring

**SKILL-RAILROADREVIEW-CTX-001** `[def][hook:skill-context]` **Dispatcher context**

Dispatcher's doctrine arrives via this skill's frontmatter `acdsl-context:` declaration, injected at invocation.

**SKILL-RAILROADREVIEW-CTX-002b** `[rule][hook:skill-invoke-gate]` **Context gates**

Every other tool call but a Read of the required context file is denied until invocation; the injected per-direction context
file MUST be read in full.
* param: lanes never read repo's context-entry files directly — exceptions: change's language guide and the files of their
  `LANE-MANIFEST:` manifests.

**SKILL-RAILROADREVIEW-CTX-002a** `[rule][hook:skill-context]` **Lane context**

Every lane MUST invoke the Skill tool with `args="lane:<direction>"`, receiving that direction's slice from `context-map.json`.
* param: `SKILL_CONTEXT_MAP` overrides map path; unmapped directions fall back to full declaration.

## Dispatch (dispatcher readies the round)

**SKILL-RAILROADREVIEW-MODES-003c** `[def]` **Artifact paths**

`artifactsDir` = the handshake dir the invocation names, else `~/.cache/claude-routine/artifacts/railroad-<runNs>/artifacts`;
round files under `round-{r}/`.
* param: `ledgerPath` = `artifactsDir/round-{r}/rejected.json` when rejections exist, else empty.
* param: never the session scratchpad, never `/tmp` — `dirname(artifactsDir)` also holds `wt/` and stays per run.

**SKILL-RAILROADREVIEW-STACK-005** `[def]` **Mainline**

`<mainline>` = the branch `git rev-parse --abbrev-ref origin/HEAD` names, remote prefix stripped; `main` when that fails.

**SKILL-RAILROADREVIEW-PROC-001** `[rule]` **Guard**

Premise pin MUST be the fork point — `git merge-base <base> <head>`, never base tip.

**SKILL-RAILROADREVIEW-PROC-006** `[rule][hook:read-gate]` **Full-file reads**

Changed files MUST be read in full — whole file, not just hunks.
* param: lane-side enforced via manifest gate; dispatcher-side reads stay conduct.

**SKILL-RAILROADREVIEW-MODES-004c** `[rule]` **Security add**

Risk-map high for auth/crypto/input/persisted data MUST add `security`.

**SKILL-RAILROADREVIEW-MODES-005b** `[rule]` **Chunk conduct**

Lane MUST read only its chunk.
* param: `contracts` is chunk-exempt; cross-chunk suspicions flagged in notes.

**SKILL-RAILROADREVIEW-MODES-005c** `[rule]` **Relevance filter**

Relevance MUST follow the fixed map — `code-style`: no filter; `correctness`: excludes test files; `tests`, `security`: exclude
`docs/**`, `README.md`, `*.txt`, `LICENSE*`.
* param: a `tests` lane's file list adds every assigned file's partners — never a test without its production file.
* param: partners are read, never counted as reviewed.

**SKILL-RAILROADREVIEW-MODES-016b** `[rule]` **Done rule**

A file is done for a direction after `doneAfter[direction]` consecutive clean reviews.
* param: defaults — code-style 1, tests 2, every other direction 3; `done-after: <n>` overrides every direction uniformly.

**SKILL-RAILROADREVIEW-MODES-016d** `[rule]` **Counter reset**

Any confirmed finding in a file, or applied fix touching it, MUST reset every direction's counter for that file.

**SKILL-RAILROADREVIEW-MODES-016f** `[rule]` **Round-1 full**

Round 1 MUST review everything.

**SKILL-RAILROADREVIEW-MODES-007b** `[rule]` **One artifact contract**

The JSON-only lane artifact is the contract on every route — NEVER a per-route deviation.
* param: hook-written `<artifact>.coverage.json` sidecar sits beside each finished artifact.

**SKILL-RAILROADREVIEW-SCOPE-001** `[step]` **Scope first**

RESOLVE scope: arg absent ⇒ `branch` on clean tree, `all` on dirty one, chosen scope stated.

**SKILL-RAILROADREVIEW-SCOPE-004** `[step]` **Head pin**

RUN `git rev-parse HEAD` once per round, before `base`, `lanes` and the buildtest; `headCommit` = its output — every later
command and prompt of the round names it.
* param: a commit landing on the branch mid-round waits for the next round's pin.

**SKILL-RAILROADREVIEW-SCOPE-002** `[step]` **Snapshot commit**

RUN `smine-rules snapshot --scope wip|all`; `headCommit` = its `snapshot` — temp index inside the command, no branch/index/tree
touched.
* param: `baseCommit` = HEAD (`wip`) or fork point (`all`).

**SKILL-RAILROADREVIEW-SCOPE-003** `[step]` **Snapshot rounds**

RUN `smine-rules snapshot` anew each round; fixes land on dispatcher's working tree.
* param: the snapshot is the round's pinned head — taken before `base`, `lanes` and the buildtest.

**SKILL-RAILROADREVIEW-PROC-002a** `[step]` **Explicit base**

RUN `smine-rules base --base <bracket> --head <headCommit> --mainline <mainline>` (retrying `origin/<base>` counts); USE its
`commit` as `baseCommit`.

**SKILL-RAILROADREVIEW-PROC-002c** `[stop]` **Base unresolvable**

If `smine-rules base --base` exits 2 (bracket base will not resolve) ⇒ STOP: report, never silently substitute.

**SKILL-RAILROADREVIEW-PROC-002b** `[step]` **Derived base**

RUN `smine-rules base --head <headCommit> --mainline <mainline>` when no bracket; USE its `commit` — warning in chat naming it
and its `source` (upstream, mainline).

**SKILL-RAILROADREVIEW-PROC-003** `[verify]` **Wrong-base heuristic**

VERIFY base plausibility when the diff runs to thousands of lines; settle a suspected wrong base before reviewing.

**SKILL-RAILROADREVIEW-PROC-004** `[step]` **Diff**

RUN `git diff --numstat <baseCommit> <headCommit>`, narrowed by `paths`, minus `exclude` — names and line counts only, never
hunks.
* param: `exclude` travels as `--exclude <csv>` to `smine-rules lanes` and `smine-rules pairs`, as stage arg `excludes` to every
  stage.

**SKILL-RAILROADREVIEW-PROC-011** `[step]` **Test pairs**

RUN `smine-rules pairs --base <baseCommit> --head <headCommit> [--exclude <csv>]` each round; READ `.tests | length` for
direction selection only.
* param: output `{tests, partners}` — test files, and per changed file its counterparts.

**SKILL-RAILROADREVIEW-PROC-005** `[step]` **Confirm scope**

STATE inclusions and exclusions up front.
* param: `vendor/` and generated files pre-excluded unless explicitly requested.

**SKILL-RAILROADREVIEW-PROC-002d** `[stop]` **Empty diff**

If `smine-rules lanes` exits 2 on an empty diff between base and head ⇒ STOP: report.

**SKILL-RAILROADREVIEW-PROC-007a** `[step]` **Risk tiers**

TIER every changed file per [risk-tier entries](context/actions/reviewing.md).
* param: fallback — high: migrations/auth/concurrency/money/API/persisted; medium: business logic/errors; low:
  tests/renames/format.

**SKILL-RAILROADREVIEW-PROC-007b** `[step]` **Risk-map artifact**

WRITE `artifactsDir/round-{r}/risk-map.json`.
* param: map drives [Direction selection](#skill-railroadreview-modes-004) only — never passed to a lane.

**SKILL-RAILROADREVIEW-PROC-008b** `[step]` **Auto-partition**

PARTITION automatically over either bound (mode from `chunks`, `layers` when absent) until every chunk fits.
* param: file exceeding `maxLines` rides alone; partition always stated.
* param: a test file rides in the chunk of its changed partner.

**SKILL-RAILROADREVIEW-MODES-004** `[step]` **Direction selection**

BIND to an explicit `directions` arg — exactly that set, stated.
* param: `special-focus` in set still requires `focus` arg.

**SKILL-RAILROADREVIEW-MODES-004b** `[step]` **Diff-proportional set**

SELECT diff-proportionally from the risk map when `directions` is absent; NAME every drop/add with reason.
* param: non-empty pairs `.tests` adds `tests`.

**SKILL-RAILROADREVIEW-MODES-016c** `[step]` **Round shrink**

PASS `artifactsDir/coverage.json` as `--coverage` from round 2 on — done files leave their direction's lanes.
* param: input is the previous round's coverage ledger — round 1 excludes nothing; `lanes` exits 2 without it from round 2.

**SKILL-RAILROADREVIEW-MODES-002** `[stop]` **Runnability gate**

If plan-mode session ⇒ STOP.
* param: Workflow tool missing ⇒ stop and report.

**SKILL-RAILROADREVIEW-PROC-009** `[verify]` **Base health**

VERIFY base health once per review: RUN `smine-rules buildtest --head <baseCommit>` in the foreground, 600000 ms timeout; READ
`build` and `tests` from its output.
* param: `--repo <repo> --root <dirname(artifactsDir)>/wt --branch claude-review/<runNs>-basehealth`.
* param: `--salvage <artifactsDir>/salvage`; never `--overlay` — the overlay is built against the head.
* param: `expect-base-red` ⇒ same call plus `--skip`: nothing is built, `build` and `tests` read `skipped: …` — pass them on as
  they are.
* param: `{build, tests}` travel as `baseHealth`; red ⇒ a report item that forces REJECT, never a stop — the review continues.

**SKILL-RAILROADREVIEW-NIGHTLY-007** `[step]` **Run namespace**

MINT `runNs` once per run, before round 1, and PREFIX every branch the round creates with `claude-review/<runNs>-`.
* param: `run-id` given ⇒ `runNs` = `rr-` + `run-id`, lowercased.
* param: else `runNs` = `rr-` + first 6 hex chars of the session scratchpad UUID.
* param: cleanup and reset take this prefix and the round launches' run ids; another prefix's `claude-review/` worktree at the
  head is only listed `unattributed`.

**SKILL-RAILROADREVIEW-MODES-003d** `[step][hook:read-gate]` **Lane manifests**

RUN `smine-rules lanes` once per round after the checklist agents; it writes every lane manifest next to its artifact — PASS its
`lanes` as `lanePlan`.
* param: flags per `smine-rules lanes -h`; `--deployed <context dir>` outside this repo.
* param: manifest `<artifact>.lane-manifest.json` = `{artifact, direction, files, partners, root, walk}`; agents never write it.
* param: exit 1 names directions whose walk selects nothing — handled as missing context, a blocker finding.
* param: explicit route passes `--worktree-root <worktreeRoot>`; `root` then names each lane's worktree.
* param: `known` travels as `--known <path>` every round; the call writes `round-{r}/known.json` — `known:` rows, then the run's
  own.
* param: PASS its `knownPath` and `idFloor` verbatim as stage args of those names, every round, empty values included.

**SKILL-RAILROADREVIEW-PROC-012** `[step]` **Spec copy**

COPY every spec each round before the checklist stage: `cp <spec file> <artifactsDir>/round-{r}/spec-<n>-<basename>`; PASS the
copies as `specPaths` — never a checkout path.
* param: spec files = every doc the `spec` list names, in order, a plan dir resolved to its `design/<slug>.md` first; no spec ⇒
  no copy, `specPaths` empty.
* param: n = the doc's 1-based position in the list — copies of same-named docs never collide.

**SKILL-RAILROADREVIEW-WALK-004** `[spawn][script]` **Checklist agents**

INVOKE `workflows/checklist.js` — one `railroad-checklist` agent per judgment direction, correctness over the spec, contracts
over the diff, writing `round-{r}/checklist-<direction>.json`.
* Script: workflows/checklist.js
* param: args: the round's commits and dirs, judgment `directions`, `specPaths` (round-dir copies), `walkGroup`, `laneModel`;
  stage returns path and counts only.
* param: correctness: one checklist over every spec, items grouped per spec, never merged across specs.
* param: launch after a typed user message ⇒ [Primer hop](#skill-railroadreview-tpl-008) first.

**SKILL-RAILROADREVIEW-WALK-005** `[substep]` **Extract checklist**

EXTRACT the requirements (correctness) or changed boundaries (contracts) as numbered `"<direction>-<n>: <text>"` items, grouped
coherently.
* param: correctness omits requirements naming only tests — `tests` lanes check those against the spec.

**SKILL-RAILROADREVIEW-ROUND-016** `[step]` **Build and test**

RUN `smine-rules buildtest --head <headCommit>` in the foreground each round, 600000 ms timeout; PASS its `{build, tests}` as
`buildTest` — head health, never a finding source.
* param: branch `claude-review/<runNs>-buildtest-r{r}`, flags as base health; station copies both verbatim.
* param: red caps the recommendation at CONDITIONAL, script-enforced; "skipped" never caps.
* param: `expect-head-red` ⇒ same call plus `--skip`: nothing is built, `build` and `tests` read `skipped: …` — pass them on as
  they are.
* param: round 1: this call and [Base health](#skill-railroadreview-proc-009) go out as two Bash calls in one message, each with
  the 600000 ms timeout.

## Review (invocation to human gate)

**SKILL-RAILROADREVIEW-MODES-007c** `[def]` **Round stages**

Round stages, in script order: fan-out, dedup per direction then cross-direction, second confirmation (refuters and the
verification pool concurrent; a lost refuter's claim retried at once), station.

**SKILL-RAILROADREVIEW-ROUND-008** `[def]` **Refuter verdicts**

`confirmed` (survived, artifact in hand) | `debunked` (refuted) | `unverified` (survives but not demonstrable here).
* param: "could not reproduce" is a successful result; evidence is never manufactured.
* param: unverified stays in review body, flagged with missing precondition, repeated at end as re-check.

**SKILL-RAILROADREVIEW-ROUND-009b** `[def]` **Station inputs**

Deduped claim refs with every second-confirmation verdict and verdict path, head build/test result, `baseHealth`, unioned
coverage gaps.

**SKILL-RAILROADREVIEW-ROUND-012** `[def][schema:review][todo:validation-needed]` **Dispositions**

Closed set, exactly one per claim: `confirmed` | `unverified` (kept, flagged) | `rejected-with-reason` | `duplicate-of` |
`debunked`.

**SKILL-RAILROADREVIEW-MODES-003a** `[def]` **Stage files**

The dispatcher invokes `round.js` once per round (`scriptPath`) — the stage files below run as its child workflows in stage
order, each stage's return handed to the next in memory:

```text
round.js    — Round orchestrator (runs the four stages below as child workflows)
lanes.js    — Fan-out
dedup.js    — Groupers (per-direction, then cross-direction)
verify.js   — Second confirmation (refuters + verification pool, concurrent)
station.js  — Station barrier
```

* param: each stage file gates its spawns and builds its prompts.
* param: handoff contract — stage args carry paths and config; the orchestrator passes prior stage returns in memory.
* param: bulk station input travels inside the station prompt.
* param: stage returns carry claim refs, artifact paths, branches and counts; hunks and write-ups stay on disk, agent-read only.
* param: universal stage args — `base`, `head`, `baseCommit`, `headCommit`, `artifactsDir`, `runNs`, `round`.
* param: `round.js` alone also takes `workflowsDir` — this skill's deployed `workflows/` dir, absolute; its child stages resolve
  against it.
* param: plus `buildTest` (head build and test), `baseHealth` (base build and test, to the station), `specPaths` (round-dir spec
  copies, else empty).
* param: the lanes stage takes `lanePlan` and `idFloor`; every stage takes `knownPath`.
* param: plus `laneModel`/`laneEffort` and the explicit-worktree keys (`worktreeMode`, `targetRepo`, `worktreeRoot`,
  `overlayPatch`, `prebuilt`).
* param: plus `excludes` — the `exclude` arg as an array, empty when absent.
* param: plus `ide` — invocation's `goland` token, empty when absent; every worker prompt carries exactly that opt-in.
* param: round.js returns the union of every stage's branches plus `clean`.
* param: `round.js` returns `void: true` and no `handoff` when every planned lane aborted, else `void: false`.
* param: keep the `runId` every `round.js` launch of the round answers — resumes and relaunches included — for the `--run` flags
  of reset and cleanup.
* param: launch after a typed user message ⇒ [Primer hop](#skill-railroadreview-tpl-008) first.
* param: `runNs` = this run's run namespace, minted in Dispatch.

**SKILL-RAILROADREVIEW-ROUND-003** `[rule]` **Review-only lanes**

Lanes MUST NOT edit, enter plan mode, or run tree-mutating git.
* param: artifact written under `round-{r}/` outside worktree; lane terminates after writing.

**SKILL-RAILROADREVIEW-ROUND-005a** `[rule][hook:read-gate]` **Coverage check**

A finished lane is fully covered by construction — its artifact write MUST stay denied until every manifest file is fully read
and every walk pair answered.
* param: `coverage_gaps` = assignments of lanes that did not finish, surfaced in station review, never dropped.

**SKILL-RAILROADREVIEW-ROUND-005b** `[rule][hook:read-gate]` **Measured coverage**

`<artifact>.coverage.json` sidecar MUST be written by the enforcing hook from its own observation, never lane self-report.

**SKILL-RAILROADREVIEW-WALK-006** `[rule][hook:read-gate]` **Walk gate**

A lane's artifact write marking `walk_complete` MUST stay denied while any checklist pair (file × group, or group) has no `walk`
entry in the artifact; the deny message lists the pairs.

**SKILL-RAILROADREVIEW-WALK-007** `[rule]` **Incremental artifact**

A lane MUST write its artifact skeleton after its reads and append every finding and every walk answer the moment it exists; the
station MUST build `review.json` the same way — never one final dump.

**SKILL-RAILROADREVIEW-WALK-010** `[rule]` **Item citation**

A finding answering a checklist item MUST carry the item id in `checklist`; a group-first FAIL is a finding with effect
`incorrect` citing it; an item without a walk answer is NEVER a finding.

**SKILL-RAILROADREVIEW-ROUND-022b** `[rule]` **One verdict each**

Each survivor MUST be second-confirmed exactly once — refuter at/above `refute-level`, else pool.
* param: lost refuter's claim re-enters verification, never second refuter.

**SKILL-RAILROADREVIEW-ROUND-035** `[rule]` **Refuter context parity**

A second-confirming worker MUST receive everything the producing lane received — `lane:<direction>` injection, lane manifest,
diff, spec, focus, context refs, both ledgers.
* param: never less context than first confirmation.

**SKILL-RAILROADREVIEW-ROUND-011b** `[rule]` **No station self-verify**

Station NEVER second-confirms a claim itself.
* param: only on a lost verifier or opposing absorbed verdicts does it re-read, adversarially, stating that it did.

**SKILL-RAILROADREVIEW-ROUND-023** `[rule][gate:skill-008]` **Concurrency limits**

Workflow runtime runs at most 16 agents concurrently and queues the rest — the workflow MUST spawn freely, no script throttle.

**SKILL-RAILROADREVIEW-ROUND-009c** `[rule]` **Station never re-derives**

Station NEVER dedups, probes, or builds itself; every id, direction, severity and its cells MUST carry verbatim.

**SKILL-RAILROADREVIEW-ROUND-009d** `[rule]` **Echo the drafts**

Station MUST echo the script's `draft_dispositions` and `draft_probes`, adjusting exceptions ONLY.
* param: exceptions — ledger/known matches, inconsistent verdicts, lost-verdict claims.

**SKILL-RAILROADREVIEW-ROUND-009e** `[rule]` **Station input inline**

The station input MUST travel inside the script-built station prompt — never a file, never a writer agent, never
dispatcher-assembled.

**SKILL-RAILROADREVIEW-ROUND-010** `[rule]` **Verdict intake**

Refuter verdicts are binding — they MUST map straight into dispositions with their reason, probe, and artifacts.
* param: each confirmed or debunked disposition carries `evidence` — the verdict's decisive line, script-copied.

**SKILL-RAILROADREVIEW-ROUND-019** `[rule]` **Escalate, don't bury**

Suspected but undemonstrated defects in migrations, auth, concurrency, money, public API, or persisted data MUST go into
`review.json`'s `escalations` field.
* param: every presentation leads with it.

**SKILL-RAILROADREVIEW-ROUND-024a** `[quality][todo:unmeasured]` **Batch tool calls**

Every worker prompt MUST batch its independent tool calls into one message.

**SKILL-RAILROADREVIEW-ROUND-024b** `[quality][todo:unmeasured]` **No mid-run loads**

A worker prompt MUST NOT load a new tool mid-run.

**SKILL-RAILROADREVIEW-ROUND-024c** `[quality][todo:unmeasured]` **Scoped reads only**

A worker prompt MUST NOT fully read a file outside its assigned review set.

**SKILL-RAILROADREVIEW-MODES-005a** `[step]` **Lane set**

WRITE the partition as `round-{r}/chunks.json` (`[{name, files}]`) when chunking, stated; PASS it as `--chunks` — the lanes
command multiplies directions × chunks × lanes.
* param: modes — `layers` (dependency-ordered), `languages`, or explicit pathspec groups.
* param: `[maxFiles:maxLines]` bracket bounds each chunk over reviewed code only.

**SKILL-RAILROADREVIEW-MODES-005e** `[step]` **Filter bookkeeping**

STATE the lane plan's skipped list in the review record.
* param: [Coverage check](#skill-railroadreview-round-005a) diffs against filtered list.

**SKILL-RAILROADREVIEW-MODES-003b** `[step]` **Bracket unpacking**

UNPACK brackets into the real JSON `args` object: `scope[base]` → `base`, `lanes[model:effort]` → `laneModel`/`laneEffort`.
* param: `chunks[maxFiles:maxLines]` → computed `chunks` groups; range re-resolved every round.
* param: `walk-group` → the checklist group ceiling passed to `smine-rules lanes --ceiling` and the checklist agents.
* param: every other dash arg is bare camelCase field — `refute-level` is `refuteLevel`, same rule as `run-id` → `runId`; no
  other translation, no legacy names.

**SKILL-RAILROADREVIEW-MODES-019** `[step]` **Explicit worktrees**

SET the explicit-worktree keys from `overlay-patch`: `worktreeMode` `explicit`, `targetRepo` session toplevel, `worktreeRoot`
`<dirname(artifactsDir)>/wt`, `overlayPatch` the path.
* param: absent `overlay-patch` ⇒ the four keys stay unset, workers get harness isolation.
* param: `prebuilt` arg ⇒ key `prebuilt`, its paths as an array; every provisioned worktree links them from `targetRepo`.

**SKILL-RAILROADREVIEW-ROUND-001** `[spawn][script][todo:validation-needed]` **Fan-out**

SPAWN `lanes` lanes per selected direction (× chunks), each in an isolated worktree at `headCommit`, blind to sibling lanes.
* Script: workflows/lanes.js
* Schema: schemas/lane.json

**SKILL-RAILROADREVIEW-ROUND-032** `[substep][script]` **Lane review**

REVIEW the assigned chunk against the direction's context — full-file reads, claims with concrete evidence.
* param: returns claim refs (id, file, line, severity, cells, fix, checklist) and the artifact path; claim and evidence stay in
  the artifact.

**SKILL-RAILROADREVIEW-WALK-008** `[substep][script]` **Walk the checklist**

ANSWER one group at a time — file by file on `file-first`, group by group on `group-first` — until no pair is open, then mark
`walk_complete`.
* Script: workflows/lanes.js

**SKILL-RAILROADREVIEW-ROUND-002** `[stop]` **Lane premise**

If the pin checkout to `headCommit` fails or `baseCommit` is no ancestor of it ⇒ STOP: abort with commit facts.
* param: each isolated worker runs `git branch -m <its branch>`, then `git checkout -B <its branch> <headCommit>` when HEAD
  differs — the checkout is the pin.
* param: lane manifests carry `head`; the read-gate hook denies a findings write from a worktree at another commit.

**SKILL-RAILROADREVIEW-ROUND-004** `[verify]` **Claims output**

Before return: VERIFY each claim's cited code survives a lane re-read; DROP what does not, record the check in `evidence`.
* param: this is first confirmation; claim stays in question until second-confirmed.

**SKILL-RAILROADREVIEW-WALK-009** `[spawn][script]` **Continuation lane**

SPAWN the same lane again over the same manifest when it returns nothing or returns with `walk_open > 0`; it continues from its
artifact.
* param: aborted after two consecutive lost runs, or a run that lowered nothing — surfaced as coverage gap.
* param: a re-spawn's branch ends `-a<n>`, `n` the attempt from 2 on; the round's branch list holds that name.
* Script: workflows/lanes.js

**SKILL-RAILROADREVIEW-ROUND-025** `[spawn][script][todo:validation-needed]` **Groupers**

SPAWN text-only grouper agents — no worktree, no code access.
* Script: workflows/dedup.js
* Schema: schemas/grouper.json

**SKILL-RAILROADREVIEW-ROUND-033** `[substep][script]` **Severity recompute**

RECOMPUTE every lane-returned claim's severity from its cells at dedup entry; a mismatch corrects the severity, re-mints the
id's SEV token and records `{from_id, to_id}` in the stage's corrections.
* param: recompute needs `effect`, `exposure` and `fix` on each claim object — carried on the lane's claim refs; INFO is `fix` =
  "none".
* Script: workflows/dedup.js

**SKILL-RAILROADREVIEW-ROUND-006** `[substep][script]` **Per-direction dedup**

GROUP same-defect claims across lanes.
* param: script assembles best-argued survivor; survivor takes the max-severity member's severity and cells, id re-minted;
  absorbed ids in `merged_from`.
* param: in doubt, keep separate — wrong merge drops finding.

**SKILL-RAILROADREVIEW-ROUND-034** `[substep][script]` **Tail exit**

EXIT every finding at or below `ignore-level` after per-direction dedup to the pool; its verdict file carries its candidate, the
round return lists it — never refuted, staged, fixed or known-ledgered.
* param: `none` keeps every severity in the pipeline; a refuter-lowered finding joins the tail late.
* param: the gate shows the count of tail entries not debunked and their ids.
* Script: workflows/dedup.js

**SKILL-RAILROADREVIEW-ROUND-022a** `[substep][script]` **Cross-direction dedup**

PASS the union of all claims through one grouper after per-direction dedups, before any refutation.
* param: absorbed cross-direction pairs go to station as `duplicate-of`.

**SKILL-RAILROADREVIEW-ROUND-007** `[spawn][script][todo:validation-needed]` **Refutation**

SPAWN a fresh refuter that did not produce it per cross-deduped claim ≥ `refute-level` — one candidate, per Probe protocol.
* param: branch `claude-review/<runNs>-refute-{finding-id}-r{r}-{hash6}`.
* param: `none` disables stage; every claim then goes to verification pool.
* param: refuters attack the cells as claims; a confirmed verdict may carry its own cells — severity only ever lowers.
* param: refuter context per [Refuter context parity](#skill-railroadreview-round-035).
* Script: workflows/verify.js
* Schema: schemas/refuter.json

**SKILL-RAILROADREVIEW-ROUND-026** `[substep][script]` **Probe claim**

PROBE the claim per the Probe protocol — one candidate, both directions.

**SKILL-RAILROADREVIEW-ROUND-027** `[substep][script]` **Return verdict**

RETURN the verdict with artifacts — confirmed, debunked or unverified.

**SKILL-RAILROADREVIEW-ROUND-011a** `[spawn][script][todo:validation-needed]` **Verification pool**

SPAWN verifier agents over below-`refute-level` claims concurrently with refutation; lost-refuter claims re-enter the pool the
moment their refuter is lost.
* param: the twice-confirmed invariant's completeness backstop.
* param: tail claims from [Tail exit](#skill-railroadreview-round-034) join the pool — never refuted, always second-confirmed.
* param: refuter brief and Probe protocol verbatim, `VERIFY_BATCH` (4) claims per agent, severity-descending.
* param: a batch never mixes directions.
* param: branch `claude-review/<runNs>-verify-b{k}-r{r}-{hash6}`.
* Script: workflows/verify.js
* Schema: schemas/refuter.json

**SKILL-RAILROADREVIEW-ROUND-028** `[substep][script]` **Confirm batch**

SECOND-CONFIRM each claim of the batch with the refuter brief and the Probe protocol.

**SKILL-RAILROADREVIEW-ROUND-009a** `[spawn][script][todo:validation-needed]` **Station barrier**

SPAWN one consolidator agent: isolated worktree pinned at `headCommit`, branch `claude-review/<runNs>-station-r{r}-{hash6}`.
* param: returns the station-return summary; a script-detected defect re-runs it once with a correction note, its branch
  suffixed `-a2`.
* Script: workflows/station.js
* Schema: schemas/station-return.json

**SKILL-RAILROADREVIEW-ROUND-014** `[substep]` **Reconciliation**

RESOLVE spec-vs-code conflicts against `decisions.md`, separating real bugs from intentional deviations.

**SKILL-RAILROADREVIEW-ROUND-014b** `[substep]` **Routing**

ASSIGN every confirmed finding a `route` ([Route](#skill-railroadreview-out-006)).

**SKILL-RAILROADREVIEW-ROUND-015** `[substep]` **Permanent checks**

NAME a durable check catching the class per recurring-class confirmed finding.
* param: dispatcher relays and offers — never adds unasked.

**SKILL-RAILROADREVIEW-ROUND-017** `[substep]` **DoD walk**

MARK every `[DoD]`-marked context entry PASS/FAIL/N/A, queried from the context dir.
* param: `smine-rules entries --marker DoD`, or jq over `context.json` — never assumed to equal one file's contents.

**SKILL-RAILROADREVIEW-ROUND-018** `[substep][todo:validation-needed]` **Handoff**

WRITE the consolidated review as `round-{r}/review.json` — the single handoff artifact.
* param: no `.md` by default; rendering is on-request only ([Handoff copy](#skill-railroadreview-station-001)).
* Schema: schemas/consolidation.json

**SKILL-RAILROADREVIEW-ROUND-020** `[substep]` **Funnel**

RECORD the funnel in `review.json` — claims produced / tail / confirmed / unverified / duplicates / rejected / debunked.
* param: `tail` = entries left at `ignore-level` and not debunked; severity corrections listed when any occurred.
* param: gate presentation states it as one line; `debunked` includes debunked tail entries.
* param: `round.js` returns `conservation {produced, accounted, lost}`; `lost` ≠ 0 marks the round unclean.

**SKILL-RAILROADREVIEW-ROUND-021** `[step]` **Cleanup**

RUN the round cleanup after the fix pipeline, one Bash call; RELAY `skipped` and `unattributed`:

```text
smine-rules cleanup --repo <repo> --prefix claude-review/<runNs>- --head <headCommit> --salvage <artifactsDir>/salvage --run <runId> [--run <runId>]… [--root <worktreeRoot> --overlay <overlayPatch>]
```

* param: one `--run` per kept run id of the review; `--root`/`--overlay` on the explicit route only.
* param: jurisdiction is the run prefix and the explicit root; any other branch is never touched.
* param: sequence per [Salvage sequence](#skill-railroadreview-tpl-007); runs from the dispatcher's checkout.

**SKILL-RAILROADREVIEW-STATION-001** `[step]` **Handoff copy**

PASS `--plan-dir <plan dir>` to `smine-rules ledger` — it copies `review.json` and `probes/` to plan dir's `reviews/round-{r}/`.
* param: plan-dir copy is durable; `artifactsDir` copy stays inter-agent handoff.
* param: on user request, SPAWN one agent (`opus`, low) rendering `review.md` from `review.json` — derived, regenerable, never
  contract.

**SKILL-RAILROADREVIEW-STATION-002** `[verify]` **Failed round**

After a round: VERIFY `handoff` non-null and `abortedLanes` empty; on a Workflow crash RESUME via `resumeFromRunId`, else reset
and re-dispatch once per [Round reset](#skill-railroadreview-tpl-009).
* param: after a resume, check the run's journal.jsonl that no lane or refuter re-ran; failed rounds never count as clean.
* param: `nightly` and the round ended on an API error ⇒ wait 120 s before the resume or re-dispatch — nobody resumes it.
* param: `void: true` — every lane aborted: no station, no `review.json`, `abortedLanes` with their notes.
* param: handoff with aborted lanes ⇒ the script caps the recommendation below APPROVE.
* param: a station defect was already retried inside its stage.
* param: launch after a typed user message ⇒ [Primer hop](#skill-railroadreview-tpl-008) first.

**SKILL-RAILROADREVIEW-STATION-002b** `[stop]` **Second failure**

If a second round fails ⇒ STOP: report abort notes verbatim.

**SKILL-RAILROADREVIEW-STATION-010** `[verify]` **Handoff conformance**

Before accepting a round: VERIFY the `smine-rules ledger` validation held — exit 2 names the violations; re-run the merge, never
hand-accept.
* param: `jq -e '.ranked_plan | length > 0'` when confirmed findings exist, each `finding_id` starting with its direction.
* param: `conservation.lost` is 0; the dispatcher flags ids the station re-minted — dedup-output and refuter-lowered ids are
  canonical.
* param: defects — summary-only review.json, re-minted ids, or funnel disagreeing with its dispositions.

**SKILL-RAILROADREVIEW-STATION-003** `[step]` **Human gate**

PRESENT the round with `smine-rules present --review <review.json>` — with path.
* param: order — escalations first, findings table, DoD, base and head health, funnel line, recommendation.
* param: no comment ⇒ accepted; comments are binding; presentation never blocks the round's fix pipeline.

**SKILL-RAILROADREVIEW-TPL-007** `[payload]` **Salvage sequence**

Pinned-worktree removal is salvage-then-remove: bytes archived, never deleted, no force, no `git clean`:

```text
smine-rules cleanup IS the sequence per worktree: reverse overlay, archive commits since head (commits.diff),
archive the tracked delta (tracked.diff), move untracked and ignored bytes to salvage, restore, git worktree remove
without --force; then each run branch's diff is saved before git branch -D. One JSON report; a failed salvage,
"removed": false and a skipped branch are skipped rows — reported defects, never retried, never a normal outcome;
the other worktrees go on. smine-rules reset-round runs the same sequence for one round.
```

**SKILL-RAILROADREVIEW-TPL-006** `[payload]` **Model assignment**

Every agent's model and effort, one table:

```markdown
| Agent | Model | Effort |
| :--- | :--- | :--- |
| lane | `lanes` bracket, else `opus` | `lanes` bracket, else agent default |
| grouper | `lanes` bracket, else `opus` | low, fixed |
| refuter | `lanes` bracket, else `opus` | `lanes` bracket, else agent default |
| verifier | `lanes` bracket, else `opus` | `lanes` bracket, else agent default |
| station | `station:`, else session | `station:` effort, else high |
| checklist (`checklist.js`) | `lanes` bracket, else `opus` | agent default |
| fixer (`fix-apply.js`) | `lanes` bracket, else `opus` | agent default |
| fix check (`fix-check.js`) | `lanes` bracket, else `opus` | agent default |
```

**SKILL-RAILROADREVIEW-TPL-009** `[payload]` **Round reset**

A failed round's retry starts from a clean state under the same run namespace, round number and branch names, one Bash call per
command:

```text
1. RECORD: no handoff ⇒ smine-rules ledger --round-dir <artifactsDir>/round-{r} --void; a handoff ⇒ the round's known-ledger call.
2. RESET: smine-rules reset-round --repo <repo> --prefix claude-review/<runNs>- --head <headCommit> --salvage <artifactsDir>/salvage --round-dir <artifactsDir>/round-{r} --round <r> --run <runId> [--run <runId>]… [--root <worktreeRoot> --overlay <overlayPatch>]
   — <headCommit> and every kept run id of the failed attempt; --root/--overlay on the explicit route only; RELAY skipped and unattributed.
3. PIN: headCommit anew — git rev-parse HEAD on branch scope, smine-rules snapshot on wip and all.
4. PREPARE: smine-rules base and smine-rules lanes at the new pin, plus the head smine-rules buildtest whenever the new pin differs from the failed attempt's head — the new baseCommit, lanePlan and buildTest.
5. RELAUNCH: round.js for the same round; keep the new launch's run id.
```

## Fixes

**SKILL-RAILROADREVIEW-STATION-005b** `[rule]` **Fix scope**

The fix set MUST be every new confirmed ranked-plan entry at or below `autofix-level` routed `auto-fix`, minus human-struck.
* param: entries above the level or routed `human` stay `open` and surface at the gate — never silently dropped.
* param: `autofix-level` = highest severity the fix workflow fixes, identical on every route.
* param: `minor` (default); `blocker` fixes every auto-fix entry, `none` fixes nothing.
* param: split from `smine-rules ledger` — `autofix` set fixed, `record` set surfaced.

**SKILL-RAILROADREVIEW-STATION-005** `[spawn][script]` **Fix application**

INVOKE `workflows/fix-apply.js` with the ledger's `autofix` set, whole.
* Script: workflows/fix-apply.js
* param: `route` values per [Route](#skill-railroadreview-out-006); gate comments can override routes or strike findings.
* param: one `railroad-fixer` agent commits the fixes as `railroad r{r} fixes`; the dispatcher NEVER edits a file.
* param: `human` findings stay in the known ledger and surface at the gate; no fixes ⇒ no commit, no patch.
* param: the fixer attempts every id; `failed` ids are relayed in the round report and stay `open`.
* param: the stage returns `applied`, `failed` and `fixCommit`; the first parent of `fixCommit` is the pre-fix head.
* param: launch after a typed user message ⇒ [Primer hop](#skill-railroadreview-tpl-008) first.

**SKILL-RAILROADREVIEW-STATION-016** `[step]` **Record applied**

RUN `smine-rules ledger --known <artifactsDir>/known.json --round {r} --fix-state applied --ids <csv>` with the stage's
`applied` ids.

**SKILL-RAILROADREVIEW-STATION-006** `[verify]` **Fix commit**

After fixes land: VERIFY `fixCommit` non-null (snapshot scopes: the next snapshot differs); the next round pins its head anew.
* param: patch, every route — `git format-patch -o <artifactsDir>/round-{r}/fixes <fixCommit>^..<fixCommit>`.

**SKILL-RAILROADREVIEW-STATION-011a** `[step]` **Fix check**

RUN probe replay plus fix check after the round's fixes land, before the next round.
* param: no applied fixes ⇒ skipped, stated.
* param: red probe of an earlier round's `fixed` row ⇒ `--round <its round> --fix-state applied --ids <id>`, reported at the
  gate, never reverted.

**SKILL-RAILROADREVIEW-STATION-011b** `[verify]` **Probe replay**

After fixes: VERIFY every persisted probe of rounds 1..r with `smine-rules probe-replay` against `fixCommit`; ANY non-green
probe is a hit.
* param: `--repo <repo> --head <fixCommit> --root <dirname(artifactsDir)>/wt --branch claude-review/<runNs>-replay-r{r} --rounds
  <round dirs>`.
* param: exit 1 lists hits; manual-path artifacts listed for re-check, never auto-run.

**SKILL-RAILROADREVIEW-STATION-011c** `[spawn][script]` **Fix check stage**

INVOKE `workflows/fix-check.js` — one fresh `railroad-fix-checker` agent judges per applied finding whether its defect is gone
at `fixCommit`.
* Script: workflows/fix-check.js
* param: fixed — defect gone, nothing the claim does not need.
* param: hit — defect still present, new mechanism/fallback, weakened guard/test, or file no entry accounts for.
* param: launch after a typed user message ⇒ [Primer hop](#skill-railroadreview-tpl-008) first.

**SKILL-RAILROADREVIEW-STATION-014** `[substep]` **Judge fixes**

JUDGE each applied finding at the post-fix head: defect gone ⇒ `fixed`; FLAG everything else as a hit.

**SKILL-RAILROADREVIEW-STATION-017** `[step]` **Record fixed**

RUN `smine-rules ledger --known <artifactsDir>/known.json --round {r} --fix-state fixed --ids <csv>` with the checker's `fixed`
ids minus this round's red probes.

**SKILL-RAILROADREVIEW-STATION-009** `[stop]` **Oscillation guard**

If `smine-rules ledger` exits 1 with `oscillating` ids (a finding confirmed again after its fix landed) ⇒ STOP: report; never a
third automated attempt.

## Ledgers (between rounds)

**SKILL-RAILROADREVIEW-STATION-004e** `[rule]` **Rejections permanent**

A rejected finding NEVER comes back.

**SKILL-RAILROADREVIEW-STATION-004d** `[rule]` **Match by substance**

Ledger matching MUST go by file and claim substance, never by ID.

**SKILL-RAILROADREVIEW-STATION-007** `[rule]` **Clean-slate rounds**

The next round MUST run fresh lanes with no memory of prior rounds, carrying only the rejection ledger.

**SKILL-RAILROADREVIEW-STATION-004a** `[step]` **Append rejection**

RUN `smine-rules ledger --reject <id> --reason <text> --rejected <plan dir>/reviews/rejected.json --round-dir <round dir>` per
human-rejected finding:

```text
finding_id, file, line
claim, reason
round, date
```

**SKILL-RAILROADREVIEW-STATION-004b** `[step]` **Ledger replay**

COPY the ledger to `artifactsDir/round-{r}/rejected.json` each later round; PASS the path, never content.

**SKILL-RAILROADREVIEW-STATION-004c** `[step]` **Known ledger**

RUN the known-ledger call after the gate, one Bash call:

```text
finding_id, file, line
severity, effect, exposure
claim, fix, route
round, fix_state
```

```text
smine-rules ledger --round-dir <artifactsDir>/round-{r} --known <artifactsDir>/known.json --coverage <artifactsDir>/coverage.json --autofix-level <level> [--done-after <n>] [--plan-dir <plan dir>]
```

* param: appends every new confirmed finding as `open` to the run's own `known.json`.
* param: prints `autofix`, `record` and `coverageGaps`.
* param: `--plan-dir` in a plan run — handoff copy, oscillation check; `--done-after` only with the arg; `review.json` validates
  against the deployed schema.
* param: lanes and refuters suppress matching claims at source; station marks matches `known`.

**SKILL-RAILROADREVIEW-MODES-016a** `[step]` **Coverage ledger**

MAINTAIN `artifactsDir/coverage.json` through `smine-rules ledger` — per direction × file `{clean_reviews, last_round, done}`.

**SKILL-RAILROADREVIEW-MODES-018** `[step]` **Escalation suggestion**

REPORT a one-line suggestion to raise lane model to `opus` when new confirmed findings fall considerably below last round.
* param: experimental, evidence-free beyond "fewer findings" — operator applies it, never automatic.

**SKILL-RAILROADREVIEW-MODES-001** `[stop]` **Round gate**

If interactive ⇒ STOP after the round completes end-to-end — gate presentation, fixes, fix check, patch, report — next round
only on explicit go.
* param: no launcher holds inside a round.

**SKILL-RAILROADREVIEW-MODES-014b** `[stop]` **Budget stop**

If remainder can't fund full next round ⇒ STOP after this one: report count honestly.

**SKILL-RAILROADREVIEW-ROUND-036** `[step]` **Artifact prune**

RUN `smine-rules prune --artifacts <artifactsDir>` as the review's last action, one Bash call.
* param: `verbose` ⇒ `--verbose`: the lane reports and per-finding verdict files stay.
* param: nightly route: after the final round's cleanup, however the run ends; interactive: at the termination stop.
* param: never between rounds — probe replay reads earlier rounds' probes.

**SKILL-RAILROADREVIEW-STATION-008a** `[stop]` **Termination**

If two consecutive clean rounds ⇒ STOP: finished — clean = workflow `clean: true` and gate accepts.
* param: before the halt, RUN [Artifact prune](#skill-railroadreview-round-036).
* param: `clean: true` = no NEW confirmed finding (the `ignore-level` tail left at dedup), no aborted lane.
* param: any new blocking finding resets counter; known-ledger carry-overs never reset it.
* param: then APPROVED → package-commit.

## Nightly route
`{section: route, pred: mode = nightly, replaces: MODES-001}`

**SKILL-RAILROADREVIEW-NIGHTLY-001a** `[step]` **Unattended rounds**

LOOP rounds to the round cap without a per-round go — same workflow route, headless.
* param: round cap = `rounds` arg, default 2; hitting cap reported, never silently truncated.
* param: after the final round's cleanup, however the run ends, RUN `smine-rules prune --artifacts <artifactsDir>` (`--verbose`
  with `verbose`) last.

## Probe protocol (second confirmation)

**SKILL-RAILROADREVIEW-PROBE-002a** `[def]` **Probe anatomy**

Three parts: task-lifetime rule line in untracked `railroad-probes-r{r}.acdsl` at worktree root; verifier; registry.
* param: verifier contract — files-list + `key=value` argv, exit 0 clean, exit 1 + `file:line: message` on hit.

**SKILL-RAILROADREVIEW-PROBE-002b** `[def]` **Probe registry**

Third part: the probe's own entries under `round-{r}/probes/` — `acdsl check -registry` lays them over the repo registry, never
a copy of it.
* param: a reused registry verifier needs none; a script entry's argv is `["/bin/bash", "<abs script>"]`, timeout ≤ 60s.

**SKILL-RAILROADREVIEW-PROBE-005** `[rule]` **Probe persistence**

Exactly the scripts, `fixtures/`, the per-finding registries and `<finding-slug>.acdsl` mirrors MUST persist under
`round-{r}/probes/`.
* param: index lives in `review.json`'s `probes[]`, each entry with its registry, never sidecars.

**SKILL-RAILROADREVIEW-PROBE-007** `[rule]` **Both directions**

A test demonstrates a claim ONLY if it fails at `headCommit` for the stated reason and passes with the fix — both stated.
* param: fix applies temporarily in disposable worktree, restored exactly, never committed.

**SKILL-RAILROADREVIEW-PROBE-001** `[rule]` **Deterministic probes**

Prober MUST author a deterministic debug probe per claim in ACDSL repos — second confirmation, two authoring attempts.

**SKILL-RAILROADREVIEW-PROBE-003** `[rule]` **Validity first**

Before the real run, the probe MUST exit 1 on a minimal fail fixture under `round-{r}/probes/fixtures/`.
* param: probe blind to its own fixture is failed attempt; fixture path recorded on `probes[]` entry.
* param: self-check's files list lives under `round-{r}/scratch/<finding-slug>/` — never under `probes/`, never `/tmp`.

**SKILL-RAILROADREVIEW-PROBE-004** `[rule]` **Probe verdict**

The verdict MUST come from `acdsl check -rule <probe-id> [-registry <probe registry>]` — red on claimed file ⇒ `confirmed`;
green ⇒ `debunked`.
* param: `make -C <worktree> acdsl ARGS="check -rule <probe-id> [-registry <file>]"` here; a repo shipping `bin/acdsl` runs it
  directly — never a build.
* param: unprobeable after two attempts ⇒ manual path, flagged `probe: "none: <why>"`.
* param: `<probe-id>` = `PROBE-` + the finding slug upper-cased — unique per finding.

**SKILL-RAILROADREVIEW-PROBE-006** `[rule]` **Manual path**

Non-ACDSL or unprobeable claims MUST settle via the cheapest artifact — failing test, command with actual+expected, or
reproduction sequence.
* param: written under `round-{r}/`, named for finding.

## Output Format

These rules govern gate presentation and any on-demand rendering; data contract is `review.json`.

**SKILL-RAILROADREVIEW-OUT-002** `[def]` **ID**

`<direction>-<SEV>-<k>-r<r>`, SEV ∈ BLK · MAJ · MIN · NIT · INF, `k` numbered from 1 per direction across lanes and severities,
`r` the round.
* param: lets author reply "fix critical-BLK-1, strike code-style-NIT-2".
* param: SEV token follows the severity through recompute and refuter lowering; `k` is kept, so no re-mint collides.

**SKILL-RAILROADREVIEW-OUT-005** `[def]` **Severity vector**

Two cells per finding, from evidence: `effect` (what goes wrong), `exposure` (share of normal operation hit); severity is the
table row for the cells, never a grade.
* param: effect — destructive | incorrect | degraded | debt | cosmetic; an absent required behavior is `incorrect`.
* param: exposure — always | often | rare | eventually.
* param: always/often/rare name the path, input or state in evidence; an absent behavior cites its checklist item.
* param: an unprovable cell takes the lower-severity value.
* param: INFO exactly when `fix` is "none"; every other finding takes its row.
* param: values twin `schemas/severity-table.json` axes; the lane prompt renders the rows and the example fixtures.

**SKILL-RAILROADREVIEW-OUT-005b** `[def]` **Severity table**

`schemas/severity-table.json` maps every cell combination to exactly one severity; `severity-fixtures.json` holds the ratified
cases; the severity-table gate proves completeness and every fixture.
* param: assigned by lane from the table, recomputed at dedup entry, raised at most once by dedup.
* param: lowered at most once by a refuter's cells, frozen at second confirmation.

**SKILL-RAILROADREVIEW-OUT-006** `[def]` **Route**

Per confirmed finding: `human` (needs judgment, surfaced at the gate) | `auto-fix` (mechanical, applied by the fix-apply stage).
* param: a fix touching `Makefile`, `makefile`, `GNUmakefile` or `*.mk` routes `human` — make sources are write-protected
  headless.

**SKILL-RAILROADREVIEW-OUT-009** `[def]` **Recommendation**

**APPROVE** (no blocking finding) | **CONDITIONAL** (mergeable once blocking fixes land) | **REJECT** (fundamental rework).
* param: red head build or tests cap it at CONDITIONAL; red base forces REJECT; an aborted lane bars APPROVE — all
  script-enforced; "skipped" never caps.

**SKILL-RAILROADREVIEW-OUT-010** `[def]` **Probes index**

`review.json`'s `probes[]` is the probe index (id, finding, script, fixture, registry, tree verdict per probe).
* param: presentations show it as table before funnel line.

**SKILL-RAILROADREVIEW-OUT-001** `[rule]` **Findings table**

One row per finding, proposed fix in every row; blocking findings MUST sit in the Findings table, approved in a separate table
below.

**SKILL-RAILROADREVIEW-OUT-003** `[rule]` **Proposed fix**

The proposed fix is NEVER empty: concrete edit, one sentence or short diff sketch.

**SKILL-RAILROADREVIEW-OUT-004** `[rule]` **Coverage**

Every review MUST state files reviewed against the diff list; uncovered files listed explicitly, never silently dropped.

**SKILL-RAILROADREVIEW-OUT-007** `[rule]` **Row order**

Rows MUST order by severity; the Direction column keeps each finding's direction visible.

**SKILL-RAILROADREVIEW-OUT-008** `[rule]` **DoD table**

Every review MUST end with the Definition-of-Done summary table.

**SKILL-RAILROADREVIEW-TPL-003** `[payload]` **Table template**

Findings table format:

```markdown
| ID | Severity | Route | Location | Finding | Proposed fix |
|----|----------|-------|----------|---------|--------------|
| code-style-NIT-1 | NIT | auto-fix | pkg/foo.go:42 | <the defect and the standard it violates> | <one-sentence fix or a short diff sketch> |
```

**SKILL-RAILROADREVIEW-TPL-004** `[payload]` **DoD template**

Definition of Done summary table format:

```
| Criterion | Status | Notes |
|-----------|--------|-------|
| Item 1    | PASS   |       |
| Item 2    | FAIL   | Missing X |
| Item 3    | N/A    | Not applicable |
```

## Review record

**SKILL-RAILROADREVIEW-DELIVER-001** `[rule]` **Plan-file record**

Dispatcher MUST maintain the session plan file, one section per round, each referencing the round's handoff path.
* param: never hand-write `plan.md` in repo.

**SKILL-RAILROADREVIEW-DELIVER-002** `[rule]` **Merge, don't overwrite**

Reviews MUST accumulate in the session plan file — append new dated sections, preserve earlier ones.

**SKILL-RAILROADREVIEW-DELIVER-003** `[rule]` **Resolved findings**

A superseded finding MUST be marked resolved (strikethrough + pointer to resolving review), never deleted.

**SKILL-RAILROADREVIEW-DELIVER-004** `[step]` **Re-review deltas**

SYNC worktree, CONFIRM spec current, REPORT deltas.

**SKILL-RAILROADREVIEW-DELIVER-005** `[step]` **Section structure**

RENDER each review section with `smine-rules present --review <review.json> --section <heading>` per [Section
template](#skill-railroadreview-tpl-005).

**SKILL-RAILROADREVIEW-TPL-005** `[payload]` **Section template**

Review section structure:

```markdown

## Review — <YYYY-MM-DD> — <base>...<head> — <directions, round>

<the findings table, per the Output Format>

### Definition of Done
<the DoD summary table>

### Recommendation
<APPROVE | CONDITIONAL | REJECT>
```

## Model

- Suggested: frontier / high
- Reason: multi-standard compliance review; railroad orchestration
- Tested unviable: sonnet-5 as dispatcher (interactive route) and as station — g2 run 2026-09-16: protocol mislabeling,
  hand-condensed station input, inadequate BLOCKER fix accepted
