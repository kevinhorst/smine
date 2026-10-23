## Design notes

- **SKILL-FDESIGN-INTAKE-001**: missing docs interrupted runs in two repos.
- **SKILL-FDESIGN-INTAKE-005**: re-architected design docs were rejected wholesale; a verified-false doc assumption was the
  most-valued finding of a session.
- **SKILL-FDESIGN-INTAKE-007**: "without the admin stuff" was read as dropping the entire new data model; it meant skip the
  admin UI.
- **SKILL-FDESIGN-INTAKE-008**: fallbacks parsing the old Redis key format were added despite "Redis will be wiped" — a full
  session was spent deleting them.
- **SKILL-FDESIGN-INTAKE-009**: driver-list ambiguity — not skill misuse — drives revision-heavy authoring; surfacing it up
  front — the planner resolves each driver's reading itself before a draft — avoids round-tripping through a plan
  rejection. Ambiguous multi-driver
  punch-lists took 13–22 plan revisions and 2 rejections before approval, while an unambiguous driver list flowed through in 1
  revision, 0 rejections.
- **SKILL-FDESIGN-GROUND-001b**: the plan carries its own gate context.
- **SKILL-FDESIGN-GROUND-002**: "table-driven tests" that don't match the repo's actual test skeleton have been rejected as
  off-pattern.
- **SKILL-FDESIGN-GROUND-007a/b**: permission rules judged by eye were wrong both ways — one plan wrote N/A and its
  session was then denied `Bash(z3 *)` 7× and `go run` 4×; others missed lsof, shellcheck and golangci-lint. Path-fence and
  roots denies take no rule, so only a "no allow rule matches" answer becomes a `permissions.json` rule.
- **SKILL-FDESIGN-CODE-006/007/008**: the freeze follows the code-gate approval because a session has one freeze per
  design prompt the user typed — freezing an unapproved draft would spend it. The rules are shown in the session plan
  file, rendered from the persisted plan: an in-repo view left the plan modified at turn end, and peek shows the
  session plan file, which nothing updated after the design gate. The Plan UI redraws that file only on plan-mode
  entry, so the code-gate message links it and the file viewer loads it from disk.
- **SKILL-FDESIGN-CODE-005, INTAKE-011**: the first approval ends plan mode, so the code gate always runs outside it; a
  run's intensity is fixed at its start so that mode change is never read as a new one. The notice once ended "Leave
  plan mode and rerun to refute", and every plan-mode run announced "I'll exit plan mode and rerun to actually refute" —
  a rerun that the fixed intensity forbids. The plan-mode line now says refutation is off for the run and a later mode
  change changes nothing.
- **SKILL-FDESIGN-DECIDE-002**: the winning parallel run decided token hashing and nil-guards inline; the losing runs asked or
  hedged. "Q&A is bullshit for feature design; I do not have enough context to answer."
- **SKILL-FDESIGN-DECIDE-006**: an "a and b" answer to a narrow question was inflated into a skill-deploy toggle UI, a
  machine-local opt-in store, a re-sync path and a repo `primary:true` field with validation — all unrequested, all torn out.
- **SKILL-FDESIGN-WRITE-002**: the approved design is committed on its own, so the code gate diffs against an approved state.
- **SKILL-FDESIGN-INTAKE-015**: an implemented plan stays the record of the repo state it was written against, so later
  work gets a sibling.
- **SKILL-FDESIGN-WRITE-009**: the planner cannot verify its own reading of the request, and the Workflow rounds do not
  run in plan mode.
- **SKILL-FDESIGN-WRITE-010**: a suggestion the planner drops must stay visible at the design gate.
- **SKILL-FDESIGN-GROUND-010**: design runs kept finding wrong and missing statements in docs and context while grounding
  and nothing carried them back; the agents that already read the area report them, and the planner writes them down.
- **SKILL-FDESIGN-HANDOFF-001**: sessions repeatedly stopped after approval; the stall went unnoticed and wasted wall-clock
  time. The chain's explicit plan path keeps the /fimplement hook's plan resolution unambiguous on any branch, fleet
  branches carrying several plans included. The preamble once placed the chain "right after approval (Phase 4)" while
  the rule binds to the code gate; both now name the code-gate approval.
- **SKILL-FDESIGN-TRUST-001/002**: the code gate is a second review turn; where the design already determines the
  code the user waives it per invocation. The flag is the authorization for the freeze — the rules stay inside the
  gate's self-service shape check and one chat line names them. Under `--nightly` the flag does nothing: a routine run
  never chains.
- **SKILL-FDESIGN-INTAKE-018, INTAKE-011 (request line), WRITE-009**: a `/fdesign` typed at the end of a discussion took the
  quoted paragraph after it as the request and planned one topic of six; the driver check got "the user's messages of this
  run" and filed an earlier message as "before the invocation". The user: "when I type /fdesign after a discussion, it means
  everything of this discussion" — and, against a whole-conversation rule: "it just has to look at the last n turns and
  decide by itself what makes sense to be in scope". No other skill hands anything over and no fixed window binds; the
  user's stated scope binds, the planner calls only what is ambiguous, the mode notice names the call before the grounding round spends its agents so a wrong call is
  interruptible for free, it lands as the plan's Scope In and Out, and the design gate's feedback settles it.
- **SKILL-FDESIGN-INTAKE-012**: a chained invocation's args are written by the invoking agent; one run turned the agent's
  own note ("graceful-shutdown drain is untested") into a driver, two decisions and five test scenarios, then spent two
  refute rounds and three user questions on them. The request boundary closes that at the reader.
- **SKILL-FDESIGN-DECIDE-001**: OPEN rows cost the user a chat turn each; the budget-seconds question was answered
  "its fine", the drain-bound question was asked three times about a mechanism outside the request. The user's position:
  misalignments surface at the gate or in a discarded implementation, both cheaper than the question.
- **SKILL-FDESIGN-REFUTE-001/002/004**: the run refuted 32 claims (facts, decision cells) in two rounds plus four ad-hoc
  agents — 12 of 14 debunks corrected a word like "only"; the never-settle rule plus the round cap left the planner
  claims it could neither settle nor resend, so it spawned. Now: Assumptions rows only, one round, the planner reads the
  debunk's evidence itself, two attempts then cut.
- **CODE-003 (removed 6.3.0)**: a diff that does not apply is found by /fimplement at implementation cost; a refute round
  over Changes entries cost a planning session's tokens for the same answer.
- **SKILL-FDESIGN-MASTER-001/002/005/006, DERIVED-001..004, WRITE-007/008**: codified from the merge-strategy run — one
  code gate would have run to 4,000–5,000 plan lines over two Rust crates, Go, six skills and docs. The master held
  decisions and eight contracts; four derived plans held the code, and each derived design gate corrected the master
  first. One derived plan added two cross-plan contracts locally, which is what the contract rule closes.
- **SKILL-FDESIGN-DECIDE-007, MASTER-003/004**: the planner over-splits. A later run cut one feature into seven derived
  plans, one of them a single additive field and one docs alone, after the user wrote that everything could fit one
  plan. Every row costs a session, a master correction and coordination, while agents design and implement a full plan
  quickly. So one plan is the default, the `master` arg only permits a split, and rows are plan-sized. By these figures
  the merge-strategy split would have had three rows, the docs row folded into landing.
- **SKILL-FDESIGN-INTAKE-013/014, GROUND-008/009**: the derived prompts repeated the split row and pointed at one plan's
  decision; later designers typed which siblings were implemented. The row and the branch answer both.
- **SKILL-FDESIGN-HANDOFF-003**: three of four fleet designers typed `/done` at the code gate and the agent took it as
  approval. A session knows it is a fleet member from its `/join` and its title token.
- **SKILL-FDESIGN-MASTER-007**: the orchestrator treats any member branch with a plan dir as a design member and spawns a
  worker for it, and a master has no code to implement.
- **SKILL-FDESIGN-TPL-003**: the freeze counts one plan per `/fdesign` prompt the user typed as a prompt's first line — a
  derived run started inside another message was refused its freeze.
- **SKILL-FDESIGN-DECIDE-001 (Adds cell)**: fourteen one-line decisions were approved with no size, and their sum — files,
  flags, exit codes — showed only after implementation; counting per row and summing in `plan-sections` shows it at the gate.
- **SKILL-FDESIGN-GATE-001 (waiver)**: a chat waiver of the code gate left the plan without entries and the implementer ran
  as one agent for half an hour; read as `--trust`, the code sections are still written.
- **SKILL-FDESIGN-GROUND-010 (in the plan)**: corrections kept in a commit apart from the plan were never reviewed at a gate;
  the user wants them in the plan, as prose entries.
- **SKILL-FDESIGN-GATE-003 (blank rejection)**: the widget withheld the send for a rejection without a comment, and the
  error sat far above the button; a rejection needs no reason.
- **SKILL-FDESIGN-GATE-006/007, GATE-001/003 (one gate)**: every fold re-presented the widget and, in plan mode, a second
  approval dialog followed — minutes and a click per run for a design the user had already marked up. The widget shows
  once and the feedback folds as the approval; an item the fold cannot settle is one chat question, a waiver is the
  planner's call. The user's words: "we dont need the plan -> accept roundtrip at all".
- **SKILL-FDESIGN-GATE-001/002/008/012 (leaving plan mode)**: ExitPlanMode right after the widget opened the plan dialog,
  which presented the plan and kept the widget's Send from posting. The run leaves plan mode by switching the session to
  acceptEdits; fdesign never calls ExitPlanMode. The enforce check that wanted a clean `plan-sections` before ExitPlanMode
  guards the switch instead. The 9.4.0 rewrite kept the dialog's role and renamed it "card": the switch was the gate's last
  call, "its card the only click", and the feedback was to follow. No such prompt exists — the tool returns at once — so
  session 8c771968 rendered the widget, switched, wrote "Design approved via the switch" and persisted the plan with no
  user message in between. Now the switch runs after the plan write and a clean `plan-sections`, directly before the
  widget, as mechanics; the gate turn ends after the gate message; the next user message folds and the standing verdict
  is the approval; persistence has `design_approved` as its pred and follows the verdict in flow order. The code gate
  ends its turn the same way: approval is a typed `/fimplement` or an explicit approving reply, "a chat reply" no longer
  counts, and the append after it is its own step (CODE-009).
- **SKILL-FDESIGN-GATE-002 (widget)**: a gate reopened a day later had no what and why on screen — the widget now opens
  with the plan's TLDR; marked decisions use solid fills, the faint stops were hard to tell apart; the `smine.`
  wordmark and the OK mark use the config server's neon green, the wordmark sits top left because the host's menu
  occupies the top right, and the repo has no logo file.
- **SKILL-FDESIGN-GATE-008**: the run never enters plan mode on its own; the tool-policy gate denies `EnterPlanMode` in every
  session (`denied_tools`, since b26dba270), so the prose rule is gate-backed.
- **SKILL-FDESIGN-INTAKE-019**: ui-capture in plan mode switches the session to acceptEdits through the Desktop session
  tool before the mode notice; INTAKE-017 stops only where that tool is absent — no hook output can set a permission mode.
- **SKILL-FDESIGN-GATE-006/009/010/011 (verdict)**: the heartbeat-state-handling gate came back with two adjustments and a
  rejection out of nine decisions and the fold went straight to the code gate. The user: "It went okay this time", but
  substantial feedback must bring the gate back without being told, and no number fits — "one scope change or adjusted
  decision could turn the whole design around, 10 adjustments could all just be minor"; the planner "must tell us if it
  thinks it can code now or needs another round". A named-versus-unnamed-change test was rejected as "way too mechanical,
  endless design gates", a "never re-gate on amendments" clause for its never. What stayed: three questions the planner
  answers itself — approach similar, scope similar, intent similar — short and sharp by the user's ask, and a verdict after every fold that
  shows each answer with its reason before the decision; moved presents the same gate again, the one-gate rule holds for
  standing folds.
- **SKILL-FDESIGN-GATE-013, GATE-009, CODE-002 (delta gate)**: after an all-OK approval, main rewrote the title hook under the
  heartbeat plan; the planner withdrew four decisions, added one, rated it "approach moved" and re-presented the whole
  gate. The user: "I dont want to read what I already approved", "Main moving is irrelevant here, minor changes". A
  re-gate now carries only the changed decisions; a post-approval revision goes through the same verdict, and a decision
  main made moot does not move the approach.
