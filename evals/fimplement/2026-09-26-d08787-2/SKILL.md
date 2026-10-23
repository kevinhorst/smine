---
name: fimplement
description: Carry a change to working, committed code — an /fdesign plan or findings doc binds as contract, else best-effort under the repo's context. Trigger on /fimplement [plan file or task]. Args — plan file: plan, findings doc, or task (absent = best-effort); errata: invocation feedback superseding the plan; caveman: compress prose, requires caveman skill; serena|goland: IDE MCP opt-in.
author: Kevin Horst
version: 3.4.0
title-code: FImpl
format: 2
argument-hint: 'plans/2026-09-07-foo-wip-abc1234/design/refined.md errata: "skip step 3, already applied" caveman serena goland'
acdsl-context: ACTION-IMPL-*, ACTION-NAV-010, ACTION-NAV-011, ACTION-REVIEW-VERIFY-*, RULE-COMMIT-*, FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014
allowed-tools: Edit, Read, Write, Bash(git add *), Bash(git commit -m *), Bash(git diff *), Bash(git log *), Bash(gofmt *), Bash(go build *), Bash(go test *), Bash(go vet *), Bash(jq *)
---

# fimplement

Carry change to working, committed code.

Marked entrypoint for "implementation started" — session minable later for what got built. Approved plan present: plan binds
as contract down to exact signatures, contracts, architecture — no redesign here. No plan: best-effort under repo context and
code-style. Autonomy default: missing contract not an obstacle, selects best-effort mode — proceed and adhere, don't halt.
Genuine obstacle — contract that can't hold, `[USER]` decision contradicted — still STOPs and reports, never a silent
deviation.

Static constraints not restated here: `AGENTS.md`, format guides under `context/rules/` (incl.
`commits.md`), doctrine chapters under `context/actions/` (implementing — stops, integrity; reviewing —
end-to-end verification) govern every edit. Arrive via this skill's frontmatter `acdsl-context:` declaration, injected at
invocation by skill-context hook; per-language style guides read from `context/rules/` on demand. Re-read
entries from `context/` only when no longer in context. Checkable slice enforced by ACDSL gates — in ACDSL
repos (`acdsl/registry.json` present), green `make acdsl ARGS="check"` part of done; plan shipped task contract, green
`check -lifetime task` too. Gate covers it, gate rules — not prose.

## Routing

**Purpose:** turns any change into working, committed code — binding-contract mode with a plan, best-effort under context +
code-style without one.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| reviewed design before any code | /fdesign (new or change route) |
| plan needs revising first | /fdesign refine (optional — fimplement otherwise proceeds best-effort) |

**Preconditions:**
- none required — a plan sharpens the target but is optional

**Pipeline:** fdesign (refine route optional) → fimplement → package-commit; commits via /package-commit (see README.md §
Skill map, smine repo).

## Args

| Arg | Form | Default | Effect |
| :-- | :--- | :------ | :----- |
| `plan file` | free text | best-effort mode | plan (fdesign new/change), findings doc, or task, pasted or by path; binds as contract when it defines one — see [Any input accepted](#skill-fimplement-intake-001) |
| `errata` | free text | absent | feedback/corrections at invocation, supersede the plan; clashes with a `[USER]` decision surface — see [Errata supersede plan](#skill-fimplement-intake-004) |
| `caveman` | flag | absent | compress prose caveman-style; requires the caveman skill else STOP — see [Caveman requires skill](#skill-fimplement-intake-005) |
| `serena` | flag | absent | opt this run into Serena MCP tools; absent = no Serena — see [IDE MCP opt-in](context/actions/navigating.md) |
| `goland` | flag | absent | opt this run into GoLand MCP tools, database tools included; absent = no GoLand — see [IDE MCP opt-in](context/actions/navigating.md) |

## Phase 0 — Intake

**SKILL-FIMPLEMENT-INTAKE-002b** `[rule]` **Runbook opt-in**

Test runbook MUST run only when opted in — plan args line shows `runbook`, or user asks at implementation.
* param: files persist to plan dir's `runbooks/` (per plan-dir resolution rule), run as part of verification.
* param: scenario-index-only section is informational — executes nothing.

**SKILL-FIMPLEMENT-INTAKE-002c** `[rule]` **Findings doc units**

A review-findings doc MUST give one unit per confirmed finding (file:line + proposed fix); rejected or ledgered findings stay
out of scope.

**SKILL-FIMPLEMENT-INTAKE-003** `[rule]` **Open rows OK**

Open questions or OPEN rows in a provided plan MUST NOT stop the run — implement what's settled, make the reasonable call on
the rest guided by context, note each call in the report.
* param: /fdesign refine stays available when you'd rather resolve them as design first.

**SKILL-FIMPLEMENT-INTAKE-004** `[rule]` **Errata supersede plan**

User errata MUST supersede the plan — colliding with a `[USER]` decision surfaces it
([User override](#skill-fimplement-stop-005)); else take the errata, note the call.

**SKILL-FIMPLEMENT-INTAKE-007** `[rule]` **Handoff rendering input**

A handoff rendering (fdesign handoff route) MUST bind tighter than a plan:
* param: units applied in Build-sequence order; modifications by final-state replacement of the replaces: span.
* param: red gate covered by Known-red ledger gets exactly the sanctioned fix.
* param: forbidden fixes are absolute.
* param: every Escalation-contract trigger is a stop — return blocked, never improvise (stop 11).
* param: best-effort latitude doesn't exist under a rendering — what stop 7 tolerates for plans escalates here.

**SKILL-FIMPLEMENT-INTAKE-001** `[step]` **Any input accepted**

TAKE whatever's given — an /fdesign plan (new or change route), a review-findings doc, or a bare task.
* param: no input isn't a stop — absent contract selects best-effort mode.
* param: bounded by context docs (AGENTS.md, `context/rules/`, style guides), nearest in-repo patterns.

**SKILL-FIMPLEMENT-INTAKE-002a** `[step]` **Read whole plan**

READ the whole plan first when present, not just Changes — these sections travel into implementation, bound it:

```text
Decisions (binding, `[USER]` rows especially)
Behavior contract
Hot items
Tests
Test runbook
Contracts & sweeps
Verification
Stop conditions
```

**SKILL-FIMPLEMENT-INTAKE-005** `[stop]` **Caveman requires skill**

If `caveman` requested and `~/.claude/skills/caveman` missing ⇒ STOP: "caveman requested but skill not installed"; style is
never approximated.

**SKILL-FIMPLEMENT-INTAKE-006a** `[verify]` **Target-tier proceed**

VERIFY session's model against input's target-tier line (rules/plan.md); at/above target proceed silently, one step below
warn once and proceed, no line means no check.

**SKILL-FIMPLEMENT-INTAKE-006b** `[stop]` **Target-tier stop**

If session's model two or more steps below target ⇒ STOP: run denied — report the mismatch, don't implement.

## Phase 1 — Ground

**SKILL-FIMPLEMENT-GROUND-005b** `[rule]` **Route around denials**

Denied call MUST be routed around with admitted means first (allowlisted form, make target, read-only form, file tool);
NEVER by defeating gate — script-hidden command, shell-written settings.

**SKILL-FIMPLEMENT-GROUND-001** `[step]` **Re-verify against tree**

RE-CHECK every claim against the real tree before writing anything — Explore and subagent findings re-verified against actual
files, never trusted as-is.

**SKILL-FIMPLEMENT-GROUND-002a** `[verify]` **Verify touch points**

VERIFY the touch points — with a plan, every file, symbol, prerequisite it names exists as described (path:line).

**SKILL-FIMPLEMENT-GROUND-002c** `[stop]` **Touch-point defect stop**

If a claim doesn't hold ⇒ STOP: report the plan defect, don't patch around it.

**SKILL-FIMPLEMENT-GROUND-002b** `[step]` **Ground w/o plan**

LOCATE the code the task targets, nearest sibling patterns directly when no plan exists; ground the change against those.

**SKILL-FIMPLEMENT-GROUND-003** `[verify]` **Prerequisites present**

VERIFY prerequisites present — generated code, migrations, running infra the plan assumes; missing producer step runs, down
infra gets asked about, never started.

**SKILL-FIMPLEMENT-GROUND-004** `[verify]` **Exemplar per unit**

VERIFY nearest sibling the plan points to still exists for each new unit; new code mirrors that sibling's architecture
(AGENTS.md rule).

**SKILL-FIMPLEMENT-GROUND-005a** `[step]` **Apply permissions once**

RUN `toolpolicy allow '<rule>' …` once with plan's `settings.local.json` Permissions rows before first unit; gate writes file,
returns APPLIED. Never edit the file.
* param: rule strings verbatim from plan, one quoted word each.
* param: `N/A` section or no `settings.local.json` rows — skip.

**SKILL-FIMPLEMENT-GROUND-005c** `[stop]` **Permission gap**

If no admitted path reaches the unit's goal ⇒ STOP: report the missing rule; user sends `#allow settings.local.json` or
edits the file.

## Phase 2 — Implement

**SKILL-FIMPLEMENT-IMPL-003b** `[rule]` **Test spec honored**

Plan's test spec (per-case fields as struct members, run loop call+assert only) MUST be honored; standing plan-review
checklist code rules MUST NOT be re-violated in the implementation.

**SKILL-FIMPLEMENT-IMPL-005b** `[rule]` **No pre-enumeration theater**

Pre-enumeration theater MUST NOT happen where the compiler already knows the touch list; intermediate commits need not each
build if the branch ends green.

**SKILL-FIMPLEMENT-IMPL-001** `[step]` **One unit only**

BUILD in dependency order, one logical unit at a time — never a godfile, "each endpoint, one at a time." Per unit:

**SKILL-FIMPLEMENT-IMPL-002** `[step]` **Write to contract**

WRITE the unit to the plan's exact signature and contract when a plan defines one, else to the shape surrounding code and
context dictate; new code mirrors its nearest sibling.

**SKILL-FIMPLEMENT-IMPL-003a** `[verify]` **Gate before advance**

VERIFY build plus tests green per coherent unit group before moving on — a unit that can't build alone joins its group;
config and doc units ride the final audit.

**SKILL-FIMPLEMENT-IMPL-004** `[step]` **Advance**

ADVANCE to the next unit in dependency order.

**SKILL-FIMPLEMENT-IMPL-005a** `[step]` **Compiler-driven cascades**

DRIVE refactor cascades and renames in compiled code with the compiler: change the core type/signature → build → fix each
reported error → test → green.

**SKILL-FIMPLEMENT-IMPL-006a** `[stop]` **Two-attempt ceiling**

If a logic unit reaches a second rejected attempt ⇒ STOP generating: state where code stands, which constraint's unsolved,
hand over.

**SKILL-FIMPLEMENT-IMPL-006b** `[step]` **Reverse-pass review**

REVIEW the human's version against the plan's Decisions and Stop conditions when they write one — that reverse pass has caught
real bugs.

## Phase 3 — Verify

**SKILL-FIMPLEMENT-VERIFY-001b** `[def]` **End-to-end verification**

Happy path plus the plan's edge and error cases, exercised against a running system with real values.

**SKILL-FIMPLEMENT-VERIFY-002b** `[rule]` **Capture replaces targets**

The capture MUST replace the plan's target images; no capture, no verified UI.

**SKILL-FIMPLEMENT-VERIFY-007b** `[rule]` **No proxy claim**

A stubbed dry run, proxy render check, or reasoned-not-run claim over the risk locus MUST NOT satisfy the Definition of Done
— report it unverified, not green.

**SKILL-FIMPLEMENT-VERIFY-001a** `[verify]` **Verify observable path**

VERIFY the observable path in the running app, not only unit tests — Definition of Done's End-to-End Verification.

**SKILL-FIMPLEMENT-VERIFY-002a** `[step]` **UI capture required**

END UI-touching changes with a capture — the verified page written with `~/.claude/skills/fdesign/scripts/capture-ui.sh` into
the plan dir's `design/ui/`.
* Applies: per visual-evidence rules in `context/rules/plan.md`.

**SKILL-FIMPLEMENT-VERIFY-003** `[step]` **Run running-app check**

RUN the running-app check directly through an admitted path; project needs a local e2e stack first, /dev-stack stands it up
— this skill never stands up infrastructure itself.

**SKILL-FIMPLEMENT-VERIFY-004** `[verify]` **Data-dir exclude**

Before reporting done: VERIFY a relocated data directory's new path still matched by a `sync_public.sh` `--exclude`.

**SKILL-FIMPLEMENT-VERIFY-005a** `[verify]` **Export must listen**

Before reporting done: VERIFY something listening on the `host:port` an env var exports to — exported telemetry/metrics
endpoint nothing binds sends its data nowhere.

**SKILL-FIMPLEMENT-VERIFY-005b** `[step]` **Probe the port**

PROBE the port (bind check / connection attempt) as part of running-app verification, not just that the var's set.

**SKILL-FIMPLEMENT-VERIFY-006a** `[verify]` **Report checklist gap**

VERIFY every behavior the plan's Verification checklist lists is exercised by a test or run; report any gap.

**SKILL-FIMPLEMENT-VERIFY-006b** `[stop]` **Gap not done**

If Verification checklist coverage has a gap ⇒ STOP: don't mark done.

**SKILL-FIMPLEMENT-VERIFY-007a** `[verify]` **Unreachable risk locus**

VERIFY an unreachable risk locus — a path the happy-path route can't exercise — by executing that exact path once with real
inputs before done.
* param: examples — a headless `claude -p` route, a background-reinvoke hinge, a scheduled/unattended path.

## Phase 4 — Commit

**SKILL-FIMPLEMENT-COMMIT-001** `[step]` **Stop background processes**

STOP every background process this session started — dev servers, port listeners, watchers — before offering to commit;
verify no orphaned listeners remain on the ports used.

**SKILL-FIMPLEMENT-COMMIT-002** `[step]` **Hand off commit**

HAND OFF to /package-commit — build, test, commit changed files grouped by package; this skill never hand-rolls commit
grouping.

**SKILL-FIMPLEMENT-COMMIT-003** `[step]` **Report against plan**

REPORT against the plan — each unit built, its gate result, the running-app verification outcome, any plan defect surfaced
and reported rather than worked around.

## Delegation

**SKILL-FIMPLEMENT-DELEGATE-001b** `[def]` **Delegated runner scope**

A delegated runner executes Phases 1–4 unattended.

**SKILL-FIMPLEMENT-DELEGATE-003** `[def]` **Gate entries verbatim**

The implementing chapter's gate entries (`context/actions/implementing.md`), verbatim, plus skill
conditions 8–11 below — the runner never sees the files otherwise.

**SKILL-FIMPLEMENT-DELEGATE-006** `[def]` **Plan plus errata**

The plan, findings doc, or task, and any user errata.

**SKILL-FIMPLEMENT-DELEGATE-008** `[def]` **Relay-class gates**

Relay-class gates are stop conditions 1, 2, 8, 9, 10, 11 — 8 and 9 fire only when a plan defines the contract or scope, 11
only under a handoff rendering.

**SKILL-FIMPLEMENT-DELEGATE-001a** `[rule]` **Never self-delegate**

This skill NEVER self-delegates — relays data for /delegate, whose mechanism (spawn, result contract, relay loop, respawn
fallback) lives in the delegate skill.

**SKILL-FIMPLEMENT-DELEGATE-002** `[rule]` **Spawn prompt order**

The spawn prompt MUST inline, in order:

**SKILL-FIMPLEMENT-DELEGATE-004a** `[rule]` **Discipline by mode**

Discipline MUST follow mode — with a plan, binding-contract rules (contract, exact signatures, no silent deviation); without
one, best-effort under the context docs and code-style.

**SKILL-FIMPLEMENT-DELEGATE-004b** `[rule]` **Ceiling either way**

Two-attempt ceiling per unit applies either way.

**SKILL-FIMPLEMENT-DELEGATE-005b** `[rule]` **Not a gate**

A missing or contract-light input MUST NOT be treated as a gate — proceed.

**SKILL-FIMPLEMENT-DELEGATE-007** `[rule]` **Chained skills direct**

The chained /package-commit MUST be invoked directly — no nested delegation, never via /delegate.

**SKILL-FIMPLEMENT-DELEGATE-009a** `[rule]` **Attempt-ceiling relay semantics**

The runner MUST self-count attempts; at the ceiling it returns `blocked` with both attempts' diffs/locations and the unsolved
constraint.

**SKILL-FIMPLEMENT-DELEGATE-009b** `[rule]` **Continue closes counter**

On "my version is in — continue," the runner MUST read the user's version, learn from it, proceed — user's version closes
that unit's counter, it's not attempt 3.

`{pred: (caveman_requested and not caveman_skill_installed) or tier_gap >= 2 or (plan_provided and contract_unhonorable) or
rework_required or (errata_present and user_decision_conflict) or (handoff_rendering and escalation_trigger_fired)}`\
**SKILL-FIMPLEMENT-DELEGATE-005a** `[stop]` **Gate instruction**

If any relay-class gate (stops 1, 2, 8, 9, 10, 11) fires ⇒ STOP work immediately: return the blocked-state result, don't
continue other units past it; returning `blocked` is success.

## Stop conditions

**SKILL-FIMPLEMENT-STOP-001b** `[def]` **Skill-specific stops follow**

Travel from the plan into this run; skill-specific stops follow.

**SKILL-FIMPLEMENT-STOP-001a** `[rule]` **External stop gates**

The implementing chapter's gate entries (`context/actions/implementing.md`, injected via this skill's
context declaration) MUST apply verbatim.

**SKILL-FIMPLEMENT-STOP-002** `[rule]` **Missing input**

A missing or contract-light input MUST NOT stop the run (stop 7) — implement best-effort under the context docs and
code-style, note the calls made.
  * Applies: a provided plan's open questions / OPEN rows are implemented best-effort too, not routed back. Only the genuine
    blockers below stop the run.

**SKILL-FIMPLEMENT-STOP-003b** `[rule]` **No substitution**

This skill NEVER substitutes a different mechanism, demotes a `[USER]` decision to "deferred," or invents architecture
mid-edit.

**SKILL-FIMPLEMENT-STOP-004b** `[rule]` **No-plan scope**

With no plan, the task's stated intent MUST be the scope.

**SKILL-FIMPLEMENT-STOP-003a** `[stop]` **Unhonorable contract**

If the plan's contract can't be honored as written — a signature, key format, or architecture the tree won't support (stop 8)
⇒ STOP: report the defect.

**SKILL-FIMPLEMENT-STOP-004a** `[verify]` **Scope creep**

VERIFY implementation stays within a provided plan's scope (stop 9); a self-contained necessary adjacent change is made and
noted.

**SKILL-FIMPLEMENT-STOP-004c** `[stop]` **Rework stops**

If a change would rework the designed architecture ⇒ STOP: report it.

**SKILL-FIMPLEMENT-STOP-005** `[stop]` **User override**

If a driver or discovery contradicts a `[USER]` decision in the plan (stop 10) ⇒ STOP: surface the conflict, never silently
override it.

**SKILL-FIMPLEMENT-STOP-006** `[stop]` **Escalation trigger**

If any Escalation-contract trigger fires under a handoff rendering (stop 11) ⇒ STOP: report the trigger and observed state —
in a delegated run this is the blocked result, returning it is success.

## Model

- Suggested: frontier / medium
- Delegation: gated
- The allowed-tools manifest covers mechanics only — target-repo payload commands still prompt; a full promptless run isn't
  promised.
- Runner: implement-runner
- Reason: implementation discipline — contract-honoring when a plan exists, context-and-style-guided when not — plus multi-unit
  builds and compiler-driven cascades
- Tested unviable: — (none yet)
