# Skill map

Single source of truth for skill ordering and hand-offs. Each skill's own `## When to use` section carries only its one-line position and links here.

## Planning chain

Ordering doctrine — violating it has invalidated approved plans:

```
idea (optional) → concept → clarify → fexplore → fdesign (+ refine route) → fimplement → package-commit
```

| Step | Skill | Artifact out | Feeds the next step as |
| :--- | :--- | :--- | :--- |
| 0 | `idea` *(optional — no commitment to a feature yet)* | `plans/{slug}/idea/idea.md` on close (dialogic session is the primary surface) | The surviving claim and tested assumptions the concept is drafted from; a dead idea terminates the chain |
| 1 | `concept` | `plans/{slug}/concept/concept.md`, `user_stories.md`, design pages | The concept the questions are drained from |
| 2 | `clarify` | Same files, Open Questions drained into Decisions | A stable concept — clarifying after plan approval invalidates decisions. `concept` chains straight into it in the same session when Open Questions remain |
| 3 | `fexplore` *(optional — solution space open/contested)* | `plans/{slug}/design/exploration.md` | Chosen option, recorded in the plan as a `[USER]` decision |
| 4 | `fdesign` | `plans/{slug}/design/raw.md` — the implementation plan; its `refine` route *(optional — drivers against the plan, pre-implementation)* updates `design/refined.md`, gate re-passed (refined supersedes raw) | The binding contract implementation follows |
| 5 | `fimplement` | Working, committed code | The approved plan run to completion as a binding contract |
| 6 | `package-commit` | Per-package validated commits | — |

`fchange` is the sibling of `fdesign` for any change to an existing feature — post-implementation adjustments through full restructurings, migrations, and consolidations. Impact is classified in the plan (behavior-preserving | behavioral | contract-touching), never pre-classified by the user; same rigor, same downstream (`fimplement` → package-commit — fimplement consumes change plans too), no concept/explore stages upstream. It is also the post-implementation loop target: implementation → verification → `fchange` (plan) → `fimplement` → `package-commit`. `fmt plan` is a side-branch off step 4 (either route): format migration plus familiarity-mode up-conversion, no content change. `fmt concept` is the same side-branch off step 1: audience renderings, no content change.

## Analyze chain

```
smine (pipeline) = smine-batch → batch report → fan-out → dimension skills → proposals / memory / JSON → votes (config server /proposals) → smine-apply (smine-nightly apply stage) → commits on routine/smine-nightly
```

| Step | Skill | Artifact out |
| :--- | :--- | :--- |
| 1 | `smine` | Runs the whole retrospective — mine then fan-out — via the `session-mine` workflow |
| 2 | `smine-batch` | `sessions/<scope>/*batch-NN.md` + ledger (stage 1: transcript mining) |
| 3a | `smine-memory` | Applied auto-memory updates + consolidation |
| 3b | `smine-skills` | `sessions/proposals/skills.json` |
| 3c | `smine-workflows` | `sessions/proposals/workflows.json` |
| 3d | `smine-routines` | `sessions/proposals/routines.json` |
| 3e | `smine-style` | `sessions/proposals/style.json` |
| 3f | `smine-summary` | `sessions/<scope>/json/<batch>.json` |
| 4 | `smine-apply` | consumes the votes sidecar; dispositions + implementations committed on `routine/smine-nightly` by the smine-nightly wrapper's apply stage |

`/smine` is the default route — it mines and fans out; `/smine --no-batch` routes already-mined batches, and `/smine --no-<dimension>` skips a dimension. A dimension skill runs standalone only when a single dimension on one batch is wanted; `/smine-batch` is the raw miner alone. Each dimension keeps its own `analyzed-*.txt` ledger (historical filenames, unchanged).

## Standalone skills

No fixed chain position; invoked on demand: `diagnose-debug` (diagnosis before any fix), `spec-drift` (read-only drift report — doc mode diffs a doc set against current code, contract mode enumerates and classifies every consumer of one changed contract with a recommended fix order; /diagnose-debug may hand off into contract mode; every finding hands off to a follow-up fix session), `railroad-review` (solo or multi-agent railroad — see the skill's Modes), `skillroutine-create` (repo skill authoring and launchd routine scaffolding + bootstrap — one arg-routed skill|routine; the routine route is often downstream of the skill route or /smine-routines), `fimpact` (per-axis change evaluation), `decision-support` (read-only adjudication of an external position against a closed verdict question), `coverage-increase` (hands off to package-commit), `parallelize` (matrix bake-off of one skill invocation across model/effort/arg-variant/replica cells; fronts the parallelize workflow), `skillroutine-eval` (score skill runs against a rubric derived from the skill's own SKILL.md; matrix mode fronts the skillroutine-parallel-eval pipe workflow, which nests the parallelize workflow), `investigation` (fan out N independent investigations of one open question, re-verify load-bearing claims against primary sources, merge into one baseline plus a refuted-hypotheses register; fronts the investigation workflow), `delegate` (explicit-only runner delegation: runs one eligible skill on a cheaper subagent; owns the whole mechanism — skills only declare eligibility in their Model section), `dev-stack`, `peek`, `jq`, `xlsx`, `caveman` (style modifier the planning skills delegate to), `close` (explicit end-of-session cleanup: removes the current session's pool worktree and claude/* branch via remove_agent_worktrees.sh, safety-gated, never forced; success kills the session), `merge-resolve` (merge two diverged branches by resolving all conflicts once at final-tree level — any conflicted integration ask, including failed cherry-pick chains and mid-rebase state, is normalized into this one flow; verified by build, tests and parent diffs).

## Composition (workflow piping)

How skills and workflows compose — three primitives, all harness-native:

1. **Skill fronts workflow** — the skill's SKILL.md resolves intake, the main session calls the Workflow tool with a `scriptPath` inside the skill's directory (`parallelize`, `investigation`, `smine`, `skillroutine-eval` matrix mode).
2. **Workflow nests workflow** — `workflow({scriptPath}, args)` inside a script runs another workflow inline, deterministically, sharing budget and concurrency. One nesting level only. This is the pipe primitive.
3. **Workflow runs skill** — an `agent()` stage (`agentType: 'general-purpose'`) whose prompt invokes the Skill tool and follows it unattended (parallelize cells run their target skill this way; the `session-mine` workflow runs smine-batch and the dimension skills this way; the skillroutine-parallel-eval pipe runs skillroutine-eval this way).

**Pipe doctrine:** piping skill A's output into skill B is codified as a thin workflow script — nest A's workflow via `workflow()`, adapt its structured return to B's input contract in pure JS, run B via an agent stage. The pipe lives in the **consumer** skill's `workflows/` dir (the adapter produces the consumer's input contract; the consumer's SKILL.md is the trigger surface; sync deploys the script with the skill for free). Paths a pipe needs (sibling scripts, SKILL.md locations) always travel in `args` — workflow scripts have no filesystem or env access. Because nesting is one level deep, pipes are never piped: a longer chain is one pipe script calling its children sequentially.

First instance: `skillroutine-parallel-eval` (`skills/skillroutine/skillroutine-eval/workflows/parallel-eval.js`) — nests the parallelize workflow for a sandboxed multi-model fan-out, copies surviving artifacts out of the ephemeral cell worktrees, writes the eval manifest, and scores every run with skillroutine-eval.
