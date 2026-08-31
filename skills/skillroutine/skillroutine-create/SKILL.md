---
name: skillroutine-create
description: Create or modify repo skills and launchd routines — route-selected by arg or intent, enforcing the repo skill format and the routine packaging contract. Trigger on /skillroutine-create [skill|routine] or "create a skill" or "create/schedule a routine". Args — skill|routine: route selector; inferred from intent when unambiguous.
author: Kevin Horst
version: 1.9
argument-hint: "[skill|routine]"
---

# Skillroutine Create

Author and modify skills, and scaffold launchd routines, in this repo. The deliverable is a format-compliant skill directory plus its registration, or a bootstrapped verified routine — never an unregistered orphan or an unloaded scaffold.

## When to use

**Use when:** creating or restructuring a repo skill, bumping a skill after content changes, creating a new scheduled routine, wrapping a skill as a nightly/periodic run, or bringing an ad-hoc launchd job under the routine contract.
**Don't use when:** evaluating skill runs — /skillroutine-eval. Ranking mined skill proposals — /smine-skills; mined routine candidates — /smine-routines. Plugin (non-repo) skills — the anthropic-skills plugin owns those. Managing existing routines (start/stop/reschedule/history) — the config server's `/routines` surface owns that.
**Preconditions:** a purpose statement — what the skill or routine does, when it triggers, what it must not swallow. Routine route additionally: the token file `~/.config/claude-routine/token` exists with non-zero size (verify with `[[ -s ]]` or `wc -c` only — **never read, cat, echo, or otherwise load its contents into context**, it is a live OAuth token); `flock` and coreutils `timeout` installed (`brew install flock coreutils`); a prompt or slash command that runs non-interactively.
**Workflow position:** standalone (see README.md § Skill map, smine repo); the routine route is often downstream of the skill route (skill first, wrapper second, same session) or of /smine-routines (accepted proposal in `proposals/routines.json`).

## Args

- skill|routine: route selector — `skill` authors/edits a repo skill, `routine` scaffolds a launchd routine. Absent ⇒ infer from the request; ambiguous ("a skill that runs nightly") ⇒ skill route first, then routine route, stated.

## Route

- The deliverable is a `SKILL.md` (new or edited) ⇒ **Skill route**.
- The deliverable is a `routines/<name>/` directory (`run.sh` + plist) ⇒ **Routine route**.
- Both are wanted (author a skill, then wrap it as a routine) ⇒ skill route first, routine route second — two deliverables, one session.

## Skill route

### Repo skill format

- Directory `skills/<group>/<name>/` for grouped skills (concept, feature, git, skillroutine, smine, util) or `skills/<name>/` top-level (e.g. `fmt`) — the **leaf** directory name is the skill name; group folders carry no SKILL.md. Deploy targets stay flat: sync copies every leaf to `~/.claude/skills/<name>/`.
- `SKILL.md` frontmatter: `name`, `description`, `author`, `version` — single-line values preferred; the config-server parser reads line-wise between the fences but tolerates `>`/`|` block-scalar descriptions (third-party skills). Own skills stay single-line.
- A skill created from a proposal (smine-apply, or this skill driven by an accepted `proposals/skills.json` entry) additionally carries `origin: user` (one line, after `author:`) — the provenance marker the casual lockout keys on. Never add it to shipped skills; never remove it.
- Required sections in order: title, `## When to use` (Use when / Don't use when / Preconditions / Workflow position linking the README Skill map), `## Args` (required when the skill takes intake args), the body, `## Model`. No `## Changelog` section — history lives in `changelog.json`.
- `## Args` declares every invocation-time input as `- <name>: <doc>` bullets (doc may wrap to indented continuation lines) — the verbose arg documentation for readers of the skill; the description carries the compressed form. A skill without intake args omits the section.
- `## Model` fields, in order:
  - `Suggested: <tier> / <effort>` — tier ∈ `small | mid-tier | frontier`; effort ∈ `low | medium | high | xhigh | max` (never size words like "large"; normalize when touching a skill that still carries one).
  - `Delegation:` (optional) ∈ `unattended-safe | gated` — absence means `interactive-only`. Eligibility declaration only, consumed by the /delegate skill: a skill never carries delegation intake or procedure, and never self-delegates. Never on a `small`-tier skill — runner overhead exceeds the work. A skill that grows an interactive gate is reclassified in the same change.
  - `allowed-tools` frontmatter required when `Delegation` is present — the skill's permission manifest (AGENTS.md skill-authoring rule); every rule must be covered by the permissions allowlist in `settings/claude_code/settings.json`.
  - `Runner:` (optional, gated skills with effort control) — names an existing `agents/<name>.md`; absence means `general-purpose` with session-effort inheritance.
  - `Reason:` and `Tested unviable:` as before. The tier→model mapping is never restated in a skill — it lives once in the delegate skill. Gated skills carry a data-only `## Delegation` section (relay-class gates, spawn-prompt inlines, attempt semantics) that /delegate reads.
- `changelog.json` next to SKILL.md: JSON array of `{version, date, text}`, newest first — the only changelog.
- Two version surfaces stay in sync (AGENTS.md rule): frontmatter version and `changelog.json[0].version`. Every content change bumps both; frontmatter-only metadata edits do not bump.
- Context-doc references use `$AGENT_CONTEXT_DIR_DEFAULT/...` — substituted at deploy time by `cmd/sync/sync_skills.sh`, never a hardcoded docs path.
- Body instructions are **entries**: `**SKILL-<NAME>-<TOPIC>-NNN** \`[class]\` — statement`, optional `* Why:` / `* Applies:` bullets; NAME = leaf dir upper-cased without hyphens (`package-commit` → `PACKAGECOMMIT`), TOPIC = the section's short tag (`INTAKE`, `MODEA`, `TPL`), NNN from 001 per topic; class ∈ hook | lint | gate | review | manual | step | payload. Templates and examples are `[payload]` entries followed by one fenced block (four-backtick fence when the block itself contains fences). Metadata sections (When to use, Args, Model) are not entries. Gate: ACDSL-SKILL-005; render/inspect: `rules render-skill [--disable …] [--list-entries] SKILL.md`. Migrate an existing skill with `/fmt skill <name>`.
- Description — the harness listing shows it verbatim and routes on it. Exact shape, one line: `<One short sentence what the skill does>. Trigger on /<name> or "<1-2 strongest trigger phrases>". Args — <name>: <short doc>; <name>: <short doc>.` The args clause compresses the `## Args` bullets and exists only for arg-bearing skills; routing boundaries live in `## When to use`, never in the description. Brevity caps, enforced by the repo test suite: first sentence ≤ 160 chars, whole description ≤ 450 chars — the harness tooltip renders the string verbatim, so an over-cap description is a broken display, not a style nit.

### Naming schema

Skill identity is the **leaf directory name** — the single source of truth (`internal/skills/skills.go` sets `Name` from `filepath.Base` and discards frontmatter `name`; frontmatter `name`, the H1, and every reference surface mirror it). Names follow this schema:

- **Single source of truth.** The leaf dir name is the identity; frontmatter `name`, the H1, and the README Skill map all mirror it. Renaming a skill is a dir rename plus a reference sweep to zero (excluding history: `sessions/**` batches/archive, `plans/archived/**`, and prior changelog entries), deployed with `sync --prune` so the old deployed leaf is removed.
- **No repeated family prefix.** A family sharing a long or collision-prone prefix collapses to a single-letter prefix rather than repeating the family word: `fdesign`, `fexplore`, `fimpact`, `fimplement` (not `feature-design` …).
- **Pipeline stages get stage names, unprefixed.** Sequential stages are named for the stage, not the family: `idea → concept → clarify` (not `concept-init` / `concept-clarify`).
- **A named pipeline prefixes its stage skills.** When a pipeline has a user-facing front skill, its stage and dimension skills carry the front's name as prefix so they group under one entrypoint and sort together: the smine pipeline's front is `smine`, its stages are `smine-batch` and the `smine-{memory,skills,workflows,routines,rules,summary}` dimensions. The prefix names the pipeline they belong to, not a redundant family word.
- **Variants over a target become one arg-routed skill.** When N skills differ only by their target, merge them into one skill routed by an argument rather than keeping N siblings: `fmt` (`/fmt plan`, `/fmt concept`) and this skill (`/skillroutine-create skill`, `/skillroutine-create routine`), not `reformat-plan` + `reformat-concept`.
- **Avoid harness collisions.** A raw name that collides with a built-in harness command is disqualified (e.g. `/design` is taken).

### Process

1. **Intake** — purpose, trigger phrases, routing boundaries; check the existing inventory for overlap (a new skill that shadows an existing one is a finding, not a deliverable). Pick the group folder (or top-level) and confirm it with the user.
2. **Draft** — SKILL.md per the format above; body follows the nearest sibling skill's structure.
3. **Format gate** — walk the checklist below; every miss is fixed before presenting.
4. **Register** — add the skill to README.md § Skill map in the smine repo (chain position or standalone list).
5. **Sync** — remind: `cmd/sync/sync_skills.sh` deploys to `~/.claude/skills/` (nested groups flattened to leaf names).

### Format gate

- [ ] Frontmatter has name, description, author, version — all single-line.
- [ ] When-to-use present with all four fields; Workflow position links the README Skill map.
- [ ] Intake args declared in a `## Args` section (`- <name>: <doc>` bullets); section absent only when the skill takes none.
- [ ] Model section present; no Changelog section; changelog.json exists and its first version matches the frontmatter.
- [ ] Suggested effort token from the real vocabulary (low | medium | high | xhigh | max); size words normalized on touch.
- [ ] Delegation absent or one of unattended-safe | gated — and absent on every small-tier skill; `allowed-tools` frontmatter present alongside it and fully allowlisted; Runner names an existing agents/<name>.md; no tier→model mapping restated; no delegation intake step in the body.
- [ ] Both version surfaces agree.
- [ ] Context docs referenced via $AGENT_CONTEXT_DIR_DEFAULT.
- [ ] README.md § Skill map updated.
- [ ] Description follows the shape: short sentence, trigger clause, args clause (arg-bearing skills only) — no routing boundaries.

### Evals

Delegated to /skillroutine-eval — this skill never runs benchmark loops. When a new skill needs an eval manifest, hand off with the skill name and the skillroutine-eval manifest stub.

## Routine route

Create a managed routine: a subdirectory of `routines/` containing a `run.sh` wrapper around `claude -p` and a launchd plist, conforming to the packaging contract in `plans/archived/feature_extension_v2/concept/routine_management.md`. The deliverable is a bootstrapped, verified routine — never an unloaded scaffold.

### Packaging contract

Defined in `plans/archived/feature_extension_v2/concept/routine_management.md`; templates checked in at `routines/_templates/` (leading underscore — skipped by server discovery). Do not restate either — copy and edit. Per routine `routines/<name>/`:

- `run.sh` — from the template, hardened like `routines/smine-nightly/run.sh` (the reference sibling): PATH export for homebrew, `DISABLE_AUTOUPDATER`/`DISABLE_TELEMETRY`, token `-s` check exiting 78 on failure, `flock -n` on a local `.lock`, worktree isolation via `routines/_lib/worktree.sh` (`routine_worktree_create` before `claude -p`, `routine_worktree_publish` after — the session cwd is the routine worktree, never the main checkout's working tree), `timeout` around `claude -p`, `--model`, `--permission-mode`, `--max-budget-usd`, `--output-format json`, result line appended to `results.jsonl` via `jq` even when the envelope is empty (timeout kill).
- `com.smine.routine.<name>.plist` — from the template: Label matches the filename, `ProgramArguments` → absolute `run.sh` path **in the main checkout, never a worktree**, `StartCalendarInterval` (the only supported trigger), `StandardOutPath`/`StandardErrorPath` under `~/Library/Logs/claude-routine-<name>.*.log`.
- `results.jsonl` — created by the first run, not scaffolded.

### Process (routine)

1. **Intake** — name, prompt/slash command, schedule, budget, permission mode; check `proposals/routines.json` for an accepted proposal and its caveats (e.g. non-interactive mode, loop constraints for observation routines: pin an explicit target ID, self-terminate after N no-change checks).
2. **Precondition check** — token file non-zero (`[[ -s ]]`), flock/timeout present, wrapped command runs headless. Any miss stops the scaffold.
3. **Scaffold** — copy both templates into `routines/<name>/`, edit per the contract above.
4. **Bootstrap + verify** — `launchctl bootstrap gui/$(id -u) <plist>`, then verify with `launchctl print gui/$(id -u)/<label>` branching on exit code: 0 loaded, 113 not loaded (never parse output for this).
5. **Smoke** — `launchctl kickstart gui/$(id -u)/<label>`, then read `runs` + `last exit code` from `launchctl print`. A finished job reports `state = not running` — classify it by the counters, never by the state string: `runs=1`/`exit=0` is a normal finish, `runs=1`/`exit=1` is a failure, `runs=0` never ran. Confirm a `results.jsonl` line with exit_status 0; on failure read the `~/Library/Logs` pair (stderr first), not the token file. (Origin: a nightly job exited 1 on first `launchctl kickstart` — embedded-newline token — and the `not running` state read as success.)

### Safety invariants

- The token file is verified by size only; its contents never enter a command substitution, log, commit, or context window outside `run.sh` itself.
- Routines share `~/.claude` and subscription auth via `CLAUDE_CODE_OAUTH_TOKEN` — never introduce `CLAUDE_CONFIG_DIR` isolation.
- Scaffold, bootstrap, and kickstart run against the main checkout, never a session worktree. The `claude -p` session itself runs in the routine group's own worktree (`~/.cache/claude-routine/worktrees/<group>`, a fresh dated branch `routine/<group>-<date>` per run) created and committed by `routines/_lib/worktree.sh` — routine output never lands in the main checkout's working tree. A run never reuses a branch: it bases the new dated branch on the newest un-merged one (the chain tip) else `main`, and `ROUTINE_MAX_OPEN_BRANCHES` caps how many un-merged branches may stack (empty = unlimited). A routine's group defaults to its own name; chain members set `ROUTINE_GROUP` to a shared group (one worktree + branch lineage, serialized by the group lock), and different groups run concurrently — nothing ever touches a sibling group's worktree or branches.
- Routines have no remote access: `routine_worktree_publish` commits on the run's dated branch locally — no push, no PR, no `gh`. Reviewing and locally merging the newest branch closes a run and prunes its merged ancestors; deleting a branch discards one.
- `coverage-increaser` additionally needs a target repo: `ROUTINE_TARGET_REPO` in the plist's `EnvironmentVariables` (editable from the config server's params form), falling back to the machine-local `~/.config/claude-routine/coverage-target` (absolute path, untracked like the token file) for manual non-launchd runs. Its worktree and `routine/coverage-increaser-<date>` branches live in that target repo while results and locks stay in smine.

### Format gate (routine)

- [ ] Directory under `routines/<name>/`, no leading underscore.
- [ ] run.sh matches the reference sibling: token `-s` check, flock, lib create/publish wiring, timeout, budget cap, JSON output, jq append tolerant of an empty envelope.
- [ ] Plist label = `com.smine.routine.<name>` = filename; absolute main-checkout path; `StartCalendarInterval` only; log paths set.
- [ ] Token contents never read at any step.
- [ ] Bootstrap verified by `launchctl print` exit code; smoke run recorded in `results.jsonl`.

## Model

- Suggested: frontier / high
- Reason: format enforcement plus routing judgment across the skill inventory, and security-sensitive routine scaffolding (live token adjacency) with launchd state verification
- Tested unviable: — (none yet)
