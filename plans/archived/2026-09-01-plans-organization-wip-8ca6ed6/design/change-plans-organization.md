# Plans Organization & Session Reclassification — Change Plan

route: `change`, mode: `familiar`, repos: `claude-configs` (+ archive repo, + 4 code repos at migration)

## TLDR

- Plans get one lifecycle: born **in-repo** under `plans/` (reviewable next to the code until the work merges), then **archived after merge** into one central private archive repo — integrating the two options from the earlier concept run as consecutive stages, not alternatives.
- Plan folders get a self-describing name: `<created_at>-<slug>-<state>-<commit_hash>` (e.g. `2026-09-01-plans-organization-wip-8ca6ed6`). Date-first means `ls plans/` sorts by creation date; state is `wip` in code repos, `done` in the archive.
- One resolution rule replaces the ~13 hardcoded `plans/{slug}/` path conventions; a new `/plan-archive` skill performs the move; all 5 repos' existing plan trees migrate (live dirs renamed, `plans/archived/` leaves the code repos entirely).
- Session reclassification: every session in the retired `sessions/personal` + `sessions/work` batches is re-attributed to its repo (the union of both instances' rosters — claude-configs shared, peek-mcp, go, backend, feedad-backend, copy_trader, …) via a reviewable attribution table; batches are then split into per-repo files under `sessions/<repo>/`, with the untouched originals (ledgers, cross-session arcs) preserved under `sessions/archived/` so nothing re-mines and nothing is lost.
- The "one peek instance across both profiles" idea is recorded as an open direction question, not designed here — the approved two-instance plan (`peek_multiple_profiles`) stays the near-term fix.

## Context

- Plan trees (`plans/{slug}/concept|design|reviews|runbooks/`) are committed in 5 repos with 3 divergent archival conventions, no date/state encoding, and ~13 skills hardcoding the path shape ([SKILL.md:20](skills/feature/fdesign/SKILL.md) et al.).
- Kevin's directive (binding): keep in-repo plans while work is under review, archive them after merge; folder names carry `<created_at>-<slug>-<state[wip|done]>-<commit_hash_init|commit_hash_done>`.
- Second stream: the legacy personal/work session-scope folders predate per-repo routing ([smine-batch SKILL.md:34](skills/smine/smine-batch/SKILL.md)) and need reclassifying; the two-profile (personal/work Mac account) architecture question spans smine + peek-mcp.
- Existing overlap: `plans/peek_multiple_profiles/design/change-multi-profile-ports.md` is an approved, unimplemented change plan establishing two peek instances (one per profile) — this plan must not silently contradict it.
- Constraint: `coverage-increase` deliberately uses `~/.claude/plans` and plan-mode flat files also live there — both stay untouched.

## Drivers

| ID | Observed | Wanted | Impact | Origin |
|---|---|---|---|---|
| DR1 | Plan dirs carry no date/state/commit metadata; `ls plans/` is unordered; wip vs done is invisible | Names encode `<created_at>-<slug>-<state>-<hash>`; date-sorted listing | contract-touching | user request (this invocation) |
| DR2 | Merged features' plans linger in `plans/` or pile up in per-repo `archived/` dirs (50 dirs, ~4.7M in claude-configs alone); 3 divergent archival conventions across 5 repos | After merge, plans move to one central archive repo; code repos hold only wip plans | behavioral | user request + repo survey |
| DR3 | ~13 skills + rules + README hardcode `plans/{slug}/...` | One resolution rule; skills reference it — future moves are one edit, not thirteen | behavior-preserving | repo survey (skill grep) |
| DR4 | `sessions/personal` + `sessions/work` hold 79 batches whose sessions are not classified by repo; work-scope JSON is almost entirely unattributed (18/23 batches with zero `repo` fields), labels informal where present | Every legacy session broken out and reclassified into `sessions/<repo>/` over the canonical folder set from both rosters | behavioral | user request ("sessions need to be broken up and reclassified into their repos") + provided rosters |
| DR5 | Two profiles need two smine/peek stacks; Kevin floats one instance reading multiple `.claude` homes | A recorded direction decision — concept work scoped, approved plan not silently superseded | behavioral (future) | user request ("needs concept across both repos") |

## Scope

- **In**
  - **naming convention:** plan-dir name format, creation rule, resolution rule in `context/rules/plan.md`.
  - **archive repo:** creation, layout `<repo-name>/<dir>`, README.
  - **plan-archive skill:** new skill performing verify-merged → copy → commit archive → remove → commit code repo.
  - **skill sweep:** all path-referencing skills, `context/rules/runbooks.md`, `context/actions/reviewing.md`, `README.md`.
  - **context-doc promotion:** `routine_management.md` out of `plans/archived/` into `context/` (a live skill must not depend on an archived plan).
  - **migration:** all 5 repos — live dirs renamed to the new scheme, archived/stale trees moved to the archive repo.
  - **session reclassification:** per-session repo attribution table (reviewable), mechanical split of legacy batches into per-repo files under `sessions/<repo>/`, originals preserved under `sessions/archived/{personal,work}`.
- **Out (non-goals)**
  - **plan-mode flat files:** `~/.claude/plans/*.md` namespace stays separate (harness-owned).
  - **coverage-increase:** keeps its `~/.claude/plans` persistence unchanged.
  - **single-instance peek redesign:** recorded as [D10](#decisions) OPEN; any implementation is a separate concept.
  - **peek_multiple_profiles plan:** not modified here.
  - **history rewriting:** old `changelog.json` entries and eval golden files keep their historical `plans/` path strings.
- **Not changed**
  - **`sync_public.sh`:** `--exclude='/plans/'` stays ([sync_public.sh:48](cmd/sync/sync_public.sh)).
  - **ACDSL anchors:** `^plans/[^/]+/...` already matches the new single-segment names — no `rules.acdsl` edit.
  - **plan artifact substructure:** `concept/ design/ reviews/ runbooks/` inside the dir is unchanged.
- **Deferred findings**
  - **runbook extension mismatch:** ACDSL-RUNBOOK-001 anchors `.bru` while several runbooks are `.md` — pre-existing, out of scope, worth its own fix.
  - **slug-case inconsistency:** snake vs kebab slugs coexist; migration normalizes to kebab but no rule yet enforces it for future slugs beyond the naming rule added here.
  - **archival routine sweep:** automation that detects merged-but-unarchived plans — backlog, after the manual skill proves the flow.

## Assumptions

| Assumption | Reality | Location |
|---|---|---|
| ~12 skills hardcode plans paths (earlier concept run) | 13 skills + 2 rules docs + 1 actions doc + README; `merge-resolve` is a soft reference; `coverage-increase` is the deliberate exception | skill grep, [SKILL.md:54](skills/quality/coverage-increase/SKILL.md) |
| `plans/_template/runbooks/` + `make runbook-new/-zip` exist (FACT-REPO-TEST-001) | Neither exists in this repo — the fact describes another repo's setup; no Makefile target touches `plans/` | Makefile:23-134 |
| Archiving session folders breaks the mining skip-list | Skip-list is the union of `sessions/*/` **and** `sessions/archived/*/` ledgers — archived folders stay dedup-visible | [smine-batch SKILL.md:36](skills/smine/smine-batch/SKILL.md) |
| Per-repo session folders already exist on disk | Only `personal/` and `work/` exist in this worktree; per-repo folders are created on first use by future runs | sessions/ listing |
| "One peek instance" is a small config change | peek-mcp supports exactly one `--claude-home` per process ([cmd/start.go:113](../../GolandProjects/peek-mcp/cmd/start.go)); multi-home is a real feature spanning both repos | peek-mcp source |

## Current state

- **plans/ (claude-configs, 5.6M):** 9 live dirs (plain slugs, mixed case conventions, no metadata) + `plans/archived/` with 50 dirs; archival is manual `git mv`, no tooling. (F1)
- **Other repos:** peek-mcp (flat, 57 files), RollAndRecord (flat, stale), copy_trader_flask_server_llm (`active/`+`stale/`, stale), smine stub — 3 divergent conventions. (F2)
- **Path references:** concept, clarify, idea, fexplore, fdesign, fimplement, railroad-review, code-verdict, spec-drift, fmt, skillroutine-create, dod-report, merge-resolve SKILL.md files; `context/rules/plan.md:132,148`; `context/rules/runbooks.md:3,9`; `context/actions/reviewing.md:43`; `README.md:188,231-236`. (F3)
- **ACDSL:** ACDSL-PLAN-001 anchor `^plans/[^/]+/design/(raw|refined)\.md$`, ACDSL-RUNBOOK-001 `^plans/[^/]+/runbooks/.*\.bru$` — both single-segment, new names still match. (F4)
- **Live-skill dependency on archived plan:** [skillroutine-create SKILL.md:89,93](skills/skillroutine/skillroutine-create/SKILL.md) reads `plans/archived/feature_extension_v2/concept/routine_management.md`. (F5)
- **sessions/:** only `personal/` (56 md batches + 56 json) and `work/` (23 md + 23 json), each with the six `analyzed-*.txt` ledgers; retired as routing targets, still read as ledger input. (F6)
- **Attribution coverage:** personal json carries per-session `repo` in most batches (7 batches partial/zero); work json is nearly unattributed (18/23 batches zero), and attributed values are informal labels — Couch, FA, Peek, Refactor, Worker, copy_trader, RollAndRecord, pgroll — not canonical repo names; some sessions carry only `{id, title, findings}`. The md reports often state `Repo:` per session section. Batches also hold a top-level `arcs` array spanning sessions (not mechanically splittable per repo). (F6a)
- **Skip-list contract:** the mining dedup skip-list is the union of `sessions/*/analyzed-sessions.txt` and `sessions/archived/*/analyzed-sessions.txt` — archived folders stay dedup-visible ([smine-batch SKILL.md:36](skills/smine/smine-batch/SKILL.md)). (F6b)
- **Profiles:** two macOS accounts (`kevinpersonal`, `kevin_aqms_mac`), each its own `~/.claude` + smine install; approved-unimplemented plan gives each its own peek ports/homes. peek-mcp: one home per process, `session_list` already exposes `meta.cwd`/`git_branch`. (F7)
- **Rosters (user-provided):** personal — claude-configs, copy_trader_flask_server_llm (+`_backup`), gotests (+`-v2`), peek-mcp, pgroll (+`-forked`); work — go, backend, feedad-backend, claude-configs. claude-configs exists on **both** instances (different paths, same repo). RollAndRecord appears in legacy attributions but in neither roster. (F8)

## Target state

```
code repo (any of the 5)
  plans/
    2026-09-01-plans-organization-wip-8ca6ed6/     ← wip: created_at + slug + init commit
      concept/  design/  reviews/  runbooks/        ← substructure unchanged
                                                     merge to main
                                                          │  /plan-archive <slug>
archive repo  ~/GolandProjects/agent-plans (private) ▼
  claude-configs/
    2026-09-01-plans-organization-done-3fa9c21/    ← done: same dir, renamed with done commit
  peek-mcp/
    ...
sessions/ (claude-configs)
  archived/personal/  archived/work/               ← originals intact: md, json, arcs, ledgers (dedup union)
  claude-configs/  peek-mcp/  go/  backend/  feedad-backend/  copy_trader_flask_server_llm/ ...  ← D9c set
    legacy-personal-13.md + json/legacy-personal-13.json   ← split per-repo views of the legacy batches
    (future per-repo mining appends here — routing unchanged)
```

**Principles**: single source of truth (one naming rule + one archive root fact, referenced everywhere — RULE-level indirection instead of 13 path copies); state visible on disk (name carries date/state/commit — no metadata files, no hidden view); append-only history (archive repo accumulates; code-repo history stops growing plan weight).

## Behavior contract

- Must not change: plan artifact substructure and formats (raw.md immutability, review rounds, rejected.json); `sync_public.sh` privacy exclusion; ACDSL gate behavior on plan/runbook files; mining dedup (no legacy session ever re-mines); plan-mode and coverage-increase `~/.claude/plans` behavior; the approved peek_multiple_profiles plan.
- Intentional changes: dir names gain date/state/hash (DR1); post-merge plans leave code repos (DR2); skills resolve the plan dir via rule + glob instead of a literal path (DR3); legacy sessions appear split per-repo under `sessions/<repo>/` with originals relocated to `sessions/archived/` (DR4).

## Decisions

| ID | Problem | Facts | Decision | Why |
|---|---|---|---|---|
| D1 | In-repo vs central plans | F1, F2 | [USER] Both, staged: in-repo while wip (review value), central archive repo after merge | Explicit user directive integrating the concept run's Options A + in-repo |
| D2 | Name format | F1 | [USER] `<created_at>-<slug>-<state>-<hash>`, concretely `YYYY-MM-DD-<kebab-slug>-wip|done-<7-char-hash>` | User-specified fields; date-first gives lexicographic date ordering ("order plans by date") with zero tooling — debuggable in plain `ls` |
| D3 | Hash semantics | F1 | `wip` carries the short HEAD of the base branch at plan creation; `done` carries the short merge commit on the main branch (fallback: main HEAD at archive time) | The two hashes bracket the feature: where work started, where it landed; both are recoverable from `git log` if ever wrong |
| D4 | How skills find "the plan dir for {slug}" | F3, F4 | One resolution rule in `context/rules/plan.md`: create as `plans/<today>-<slug>-wip-<head>/`, resolve as glob `plans/*-<slug>-wip-*/` (unique match required); skills cite the rule, not a literal shape | Controllable single indirection point (DR3); a glob keeps ACDSL anchors and all substructure paths untouched (F4) |
| D5 | Archival trigger | F1 | Manual `/plan-archive <slug>` run after merge, from the code repo's main checkout; routine sweep deferred to backlog | Explicit event beats a clock (no timer-driven state transitions); merge detection is verifiable at run time |
| D6 | Archive repo layout | F2 | `<repo-name>/<archived-dir-name>/` — flat per repo, no year subdirs | Date is already in the dir name; extra hierarchy adds a concept without adding information |
| D7 | Migration scope | F1, F2, F5 | All 5 repos: live dirs renamed in place (wip, dates/hashes from `git log`); `plans/archived/` + stale repos' trees move to the archive as done; code repos end with zero non-wip plan dirs | Half-migrated state would leave two conventions alive — the situation this plan exists to end; F5 is unblocked by D8 first |
| D8 | skillroutine-create's archived-plan dependency | F5 | Promote `routine_management.md` to `context/routines.md` (synced context doc) before any archival move; skill reference updated | A live skill must not depend on an archived artifact — was backlog in the concept run, becomes a blocking prerequisite once `plans/archived/` leaves the repo |
| D9 | How legacy sessions get repo-classified | F6, F6a, F6b | [USER] Break batches up per session and reclassify into `sessions/<repo>/`. Mechanism: two stages — (1) an attribution table `session-id → repo` built from json `repo` → md `Repo:` line → peek `meta.cwd` (where the transcript still exists) → content inference, with a label→repo mapping over the canonical folder set from both rosters (F8) presented for review before anything moves; (2) a mechanical splitter consuming the approved table | User directive; identity-style resolution is never fuzzy-guessed silently — low-confidence rows go to the table for review, unresolvable ones land in `default`, not a guessed repo |
| D9c | Canonical folder names | F8 | Union of both rosters by name: claude-configs, copy_trader_flask_server_llm (+_backup), gotests (+-v2), peek-mcp, pgroll (+-forked), go, backend, feedad-backend; one shared `claude-configs` folder for both instances' sessions; smine-cwd sessions normalize to claude-configs (same codebase); RollAndRecord kept as its own folder (historical repo, rosterless) | Folder = repo identity, not profile — claude-configs work from either instance is one history; names come from the user's rosters, not invented |
| D9a | What happens to the original batch files | F6a, F6b | [USER] The batch **md report** moves whole to the repo folder holding the plurality of its sessions (`sessions/<majority>/legacy-<scope>-<NN>.md`); the original per-scope **json** is replaced by the per-repo split JSONs; the six per-scope **ledgers** move to `sessions/archived/{personal,work}/` (dedup union + routing-cursor history); `sessions/{personal,work}` are removed | User directive: keep each batch report next to where its work mostly lives, not in a dead archive; `arcs` are batch-wide so they ride the plurality split (the one co-located with the md); ties broken alphabetically |
| D9b | Split-file naming and shape | F6 | `sessions/<repo>/legacy-<scope>-<NN>.md` + `json/legacy-<scope>-<NN>.json`, each json schema-conformant with `batch.scope` = the repo folder and `batch.file` = the new md path; only that repo's session sections/objects included | Traceability to the origin batch stays in the name; schema conformance keeps the session-overview server working on the new files |
| D10 | Profile architecture: two stacks vs one peek reading both `.claude` homes | F7 | [USER] Implement the approved two-instance plan (`peek_multiple_profiles`) as-is; the single-instance multi-home concept is deferred — not scheduled by this plan | Unblocks the work-profile mining backlog with an already-approved design; multi-home is a real cross-repo feature (one home per peek process today), kept in concept space for later |
| D11 | Archive repo name/location/backing | — | [USER] Local-only plain git repo at `~/GolandProjects/agent-plans`, no remote | Versioning (raw.md immutability, review history) needs git, not GitHub; a remote can be added later without restructuring |

## Open questions

Empty — Q1 and Q2 answered by the user ([D10](#decisions): two-instance plan, concept deferred; [D11](#decisions): local-only repo).

## Baseline (verified)

N/A — change route (facts live in Current state).

## Exemplar & reuse

N/A — change route. Cross-cutting reuse: `skillroutine-create` authors the new skill under the repo skill format; `sync_skills.sh` deploys it; `/package-commit` handles all code-repo commits. No change lacks an exemplar except the archive repo itself (new, trivially structured).

## Changes

### Phase 1 — Convention + archive repo (shippable alone; nothing breaks while old names coexist)

**Naming and resolution rules (modified)**
location: `context/rules/plan.md`

- Add three rules (next free RULE-PLAN numbers):
  - **Naming:** a plan dir is `plans/<created_at>-<slug>-<state>-<hash>/` — `created_at` = `YYYY-MM-DD` at creation, slug kebab-case, state `wip` (code repo) or `done` (archive), hash = 7-char short commit per [D3](#decisions).
  - **Resolution:** skills resolve "the plan dir for `<slug>`" via glob `plans/*-<slug>-wip-*/`; zero or multiple matches is a stop-and-ask, never a guess. Creation uses today's date + `git rev-parse --short HEAD`.
  - **Archival:** after the feature's merge to the main branch, the dir moves to `<archive-root>/<repo-name>/`, renamed `wip-<init>` → `done-<merge-hash>`; code repos never hold `done` dirs.
- RULE-PLAN-062/069 path examples updated to the placeholder form ("the plan dir per the resolution rule").

**Archive-root fact (new)**
location: `context/` synced doc carrying repo-facts (same doc set `sync_context.sh` builds)

- One FACT entry: archive root path (per [D11](#decisions)), layout `<repo-name>/<dir>`, and "wip lives in the code repo, done lives here".

**Archive repo (new)**
location: archive root per [D11](#decisions)

- `git init` at `~/GolandProjects/agent-plans` (local-only, no remote — [D11](#decisions)), `README.md` stating layout + naming + "written only by /plan-archive and migrations".

### Phase 2 — Skill and doc sweep (behavior-preserving; each file a small edit)

location: `skills/concept/concept/SKILL.md`, `skills/concept/clarify/SKILL.md`, `skills/concept/idea/SKILL.md`, `skills/feature/fexplore/SKILL.md`, `skills/feature/fdesign/SKILL.md`, `skills/feature/fimplement/SKILL.md`, `skills/quality/railroad-review/SKILL.md`, `skills/quality/code-verdict/SKILL.md`, `skills/quality/spec-drift/SKILL.md`, `skills/quality/dod-report/SKILL.md`, `skills/fmt/fmt/SKILL.md`, `skills/skillroutine/skillroutine-create/SKILL.md`, `skills/git/merge-resolve/SKILL.md`

- **pattern (once, applied per file):** every literal `plans/{slug}/...` becomes "the plan dir for `{slug}` (resolution rule, `context/rules/plan.md`) + `/<subpath>`"; skills that *create* the dir (concept, idea, fexplore, fdesign) additionally state the creation naming with date + HEAD hash.
- **skillroutine-create:** sweep-exclude `plans/archived/**` updated to the archive-root path; `routine_management.md` references point at the promoted context doc ([D8](#decisions)).
- **spec-drift / fmt / railroad-review:** glob patterns (`plans/**`, excludePaths) unchanged — still match the new names.
- Also: `context/rules/runbooks.md:3,9`, `context/actions/reviewing.md:43` (ACTION-REVIEW-VERIFY-004), `README.md:188,231-236` — same placeholder-form update.
- Per-skill `changelog.json` entries appended; historical entries untouched.

### Phase 3 — /plan-archive skill (new)

location: `skills/git/plan-archive/SKILL.md` (authored via `/skillroutine-create` conventions)
mirrors: `skills/orchestration/close/SKILL.md` (safety-gated destructive skill shape)

- Steps the skill encodes (guard logic — example flow written out here per hot-item rule):
  1. **Resolve:** glob `plans/*-<slug>-wip-*/` on the code repo's main checkout, main branch, clean tree — any miss stops.
  2. **Verify merged:** the dir exists on `main` and `git log -1 -- <dir>` is reachable from `main`; the feature's merge commit is located via `git log --merges --oneline -- <dir>` (fallback: main HEAD, [D3](#decisions)).
  3. **Copy:** `cp -R` the dir to `<archive-root>/<repo-name>/<created_at>-<slug>-done-<merge-hash>/`; fail if the target exists.
  4. **Commit archive repo:** one commit, message `archive: <repo-name>/<slug>`.
  5. **Remove from code repo:** `git rm -r <dir>`; commit via `/package-commit` (deletion staged with its owning group explicitly).
  6. **Report:** both commit SHAs, old and new paths.
- Never runs from a worktree; never deletes before the archive commit exists (copy-commit-then-remove ordering is the integrity guard).

### Phase 4 — claude-configs migration (depends on Phases 1–3)

location: `context/routines.md` (new, promoted from `plans/archived/feature_extension_v2/concept/routine_management.md`), `plans/` tree

- **promotion first** ([D8](#decisions)): content moved into the synced context doc set; skillroutine-create references flipped (done in Phase 2, lands together).
- **live dirs (9):** `git mv` to `<first-commit-date>-<kebab-slug>-wip-<first-commit-short-hash>` — dates/hashes from `git log --diff-filter=A --follow -1 --format=%ad -- <dir>`; slug normalized to kebab.
- **archived dirs (50 + acdsl):** moved to `<archive-root>/claude-configs/` as `done` dirs (date from first commit, hash from last commit touching the dir); `plans/archived/` deleted from the repo.
- **this plan self-hosts:** persisted as `plans/2026-09-01-plans-organization-wip-<head>/design/change-plans-organization.md` — first exemplar of the new naming.

### Phase 5 — Other repos' migration (independent per repo)

location: `peek-mcp/plans/`, `RollAndRecord/plans/`, `copy_trader_flask_server_llm/plans/`, `smine` stub

- **live plans** (e.g. peek-mcp's current work, `peek_multiple_profiles` companion artifacts): renamed to wip form in place.
- **stale/complete trees** (RollAndRecord, copy_trader `active/`+`stale/`, flat historical peek-mcp plans): moved to `<archive-root>/<repo-name>/` as done; empty `plans/` stubs removed.
- Each repo's move is one commit pair (archive commit + repo removal commit), per repo's own commit conventions.

### Phase 6 — Session reclassification (independent of Phases 1–5)

location: `sessions/personal/`, `sessions/work/`, new `sessions/<repo>/` folders

**Stage 6a — attribution table (reviewable artifact, no moves yet)**

- One JSON table on disk (plan dir of this feature), one row per session across all 79 batches:

```json
{
  "session_id": "92d87619-983e-491b-a114-70bef5ba5f29",
  "origin": "personal/session-analysis-batch-13",
  "repo": "copy_trader_flask_server_llm",
  "source": "json-repo | md-repo-line | peek-cwd | content-inference | unresolved",
  "confidence": "exact | inferred | unresolved"
}
```

- **resolution order** ([D9](#decisions)): json `repo` field → md section `Repo:` line → peek `session_get` `meta.cwd` where the transcript still exists → content inference by an agent reading the md section; unresolved → `default`.
- **label normalization:** informal labels mapped to the canonical folder set ([D9c](#decisions)) — Peek→peek-mcp, copy_trader→copy_trader_flask_server_llm, Couch→go, FA→feedad-backend, smine→claude-configs, RollAndRecord/pgroll as-is, Refactor/Worker resolved per session content; the full mapping is part of the table's header and reviewed with it.
- **approval gate:** the mapping plus all `inferred`/`unresolved` rows are presented before Stage 6b runs (S10).

**Stage 6b — mechanical split (script consuming the approved table)**

- Split each md by its `## Session` headings; each session section goes verbatim to `sessions/<repo>/legacy-<scope>-<NN>.md` (batch preamble header retained, only that repo's sections included).
- Split each json's `.sessions[]` by the table; write `sessions/<repo>/json/legacy-<scope>-<NN>.json` with `batch.scope`/`batch.file` rewritten ([D9b](#decisions)), validated against `skills/smine/smine-batch/reference/schema.json`.
- Write `sessions/<repo>/analyzed-sessions.txt` from the table (append if the folder exists from live mining).
- `git mv sessions/personal sessions/archived/personal`, same for `work` — originals + `arcs` + all six ledgers intact ([D9a](#decisions)); dimension-ledger cursors in `archived/*` keep their historical filenames (batches there are fully routed — verified precondition).
- **consumer check:** confirm the session-overview server enumerates `sessions/<repo>/` folders and skips `archived/` (reserved name) — duplicate display of archived originals is a finding to fix server-side, not a reason to delete originals.
- **profile stream:** no implementation — per [D10](#decisions) the approved two-instance plan stands; the multi-home `/concept` is deferred, not scheduled here. Work-profile repos (go, backend, feedad) are folder names here — their future mining happens on the work profile after that plan ships.

## Hot items

- **/plan-archive guard logic** (verify-merged + copy-commit-then-remove ordering): example flow written out in Phase 3 — the only destructive path in the plan; approval of that flow gates implementation.
- No SQL, no concurrency, no new interfaces/generics, no generated formats, no UI — no other hot classes touched.

## Tests

| Location.Method | Cases | Comment |
|---|---|---|
| `make audit` (acdsl gates) | ACDSL-PLAN-001/RUNBOOK-001 anchors still match renamed dirs<br>rules validate green after `plan.md` edits | gate is authoritative for governed files |
| /plan-archive dry run on a scratch repo | wip dir resolves by glob<br>unmerged dir → stop<br>duplicate archive target → stop<br>copy-commit precedes removal | manual scenario, no unit harness for skills |
| split-script invariants (Stage 6b) | every session ID lands in exactly one per-repo file<br>sum of split sessions == original count per batch<br>every split json validates against schema.json<br>md heading count preserved across splits | scripted assertions run as part of 6b, not a persistent suite |
| not tested: other repos' migrations — verified by the per-repo checklist in Verification, no test harness exists there | | |

## Test runbook

- **archive round-trip** — scratch repo with a merged dummy plan dir; run `/plan-archive dummy`; assert archive commit + repo removal commit + name transform.
- **glob resolution** — after Phase 4 renames, run `/fdesign refine` (or any resolving skill) against a renamed slug; assert it finds the dir.
- **mining dedup** — after Phase 6, inspect the Select skip-list union: every legacy session ID present (via `archived/*` and per-repo ledgers) — nothing re-qualifies for mining.
- **overview server** — open the session-overview UI: per-repo folders listed with their legacy batches; `archived/` not double-displayed.

`N/A` for tool-native request files — no HTTP/callable surface; scenarios are CLI/skill runs.

## Contracts & sweeps

| Contract | Sides | Sweep |
|---|---|---|
| plan-dir shape | 13 SKILL.md files ↔ `context/rules/plan.md` ↔ ACDSL anchors | grep `plans/{slug}\|plans/<feature>\|plans/{feature_name}` to zero in `skills/ context/ README.md`; survivors only in `changelog.json` history + eval goldens (immutable records — justified) |
| `plans/archived` references | skillroutine-create ↔ promoted context doc | grep `plans/archived` to zero outside changelogs; sweep excludes `.claude/worktrees/ examples/ sessions/` per repo convention |
| archive-root path | fact doc ↔ /plan-archive skill ↔ migration commits | grep the archive path — exactly fact + skill + README |
| sessions scope names | smine-batch routing ↔ folder layout | grep `sessions/personal\|sessions/work` in `skills/` — only historical changelog/schema-derivation notes survive |
| batch json schema | split files ↔ `reference/schema.json` ↔ session-overview server | validate every `legacy-*.json`; server renders per-repo folders without error |

## Verification

- [ ] Run `make audit` after Phases 1–2 and after Phase 4 — expect green (rules validate + generate-check + gates).
- [ ] `ls plans/` in claude-configs after Phase 4 — expect only `YYYY-MM-DD-*-wip-*` dirs, date-sorted, no `archived/`.
- [ ] Archive repo after Phases 4–5 — expect `claude-configs/` with 50+ done dirs, one dir per migrated plan, `git log` showing per-repo archive commits.
- [ ] Run the archive round-trip scenario (Test runbook) — expect stop on unmerged, success on merged, removal only after archive commit.
- [ ] Grep sweeps from Contracts & sweeps — expect zero non-justified survivors.
- [ ] Attribution table after Stage 6a — every one of the ~360 legacy sessions has a row; mapping + inferred/unresolved rows reviewed and approved before 6b.
- [ ] `sessions/` after Phase 6 — per-repo folders hold the split legacy files; `archived/{personal,work}` hold the intact originals; split invariants (count preservation, schema validation) all green; dimension-ledger cursors verified routed before the move.
- [ ] Degenerate cases: `/plan-archive` on a nonexistent slug → stop with message; two dirs matching one slug → stop; empty `plans/` repo (smine stub) → stub removed, nothing archived.
- [ ] `context/routines.md` exists and `grep routine_management.md skills/` → only changelog history.

## Stop conditions

| ID | Condition | Action |
|---|---|---|
| S1 | An approved contract can't hold as planned | Stop and report — never improvise mid-edit |
| S2 | Second failed approach in a row | Stop, re-read actual state, redesign — no third band-aid |
| S3 | Missing prerequisite (e.g. archive repo not yet created when /plan-archive runs) | Run the producing step (Phase 1); if blocked, ask |
| S4 | Discovered work materially exceeds scope (e.g. a repo's plans tree diverges from the surveyed shape) | Ask before continuing |
| S5 | Same bug class twice: in own diff → fix all in diff; pre-existing outside → report and ask | Sweeps are the user's call |
| S6 | Structural obstacle tempts a new abstraction | Stop and report; relocate, don't wrap |
| S7 | Any dimension-ledger cursor behind its folder's last batch at Phase 6 | Stop — route the pending batches first, never move an unrouted scope |
| S10 | Stage 6b started without the attribution table (mapping + inferred/unresolved rows) approved | Stop — attribution is identity resolution; no split runs on unreviewed inference |
| S11 | Split invariants fail (session count mismatch, schema rejection) | Stop, no partial state — 6b is re-runnable from the originals |
| S8 | A skill's plans reference doesn't fit the placeholder pattern (semantics differ, not just path) | Stop and show the divergent skill before editing it |
| S9 | An archive target name collides during migration | Stop — never overwrite; report the colliding pair |

## Changelog

| Date | Trigger | What changed |
|---|---|---|
| — | initial | plan created |
| 2026-09-01 | Q: profile architecture (Q1) | D10 → [USER] two-instance plan stands, multi-home concept deferred |
| 2026-09-01 | Q: archive repo (Q2) | D11 → [USER] local-only repo, no remote |
| 2026-09-01 | refine: DR4 rework | wholesale archival rejected — sessions broken up and reclassified per repo (D9/D9a/D9b, Phase 6 two-stage attribution + split) |
| 2026-09-01 | local: rosters provided | D9c canonical folder set from both instances' rosters; claude-configs one shared folder; FA→feedad-backend, Couch→go, smine→claude-configs |
| 2026-09-01 | local: no context ids in skill bodies | Phase 2 sweep reworked — entry ids moved out of all skill bodies into frontmatter acdsl-context declarations; new context/rules/skills.md rule governs it |
| 2026-09-01 | local: batch to plurality repo | D9a override — md report moves to the plurality repo (not archived); JSON-only split confirmed; ledgers to archived, scope folders removed |
