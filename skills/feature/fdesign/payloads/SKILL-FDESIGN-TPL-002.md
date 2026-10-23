# <feature> — Handoff Rendering

target: `<runner tier>` — source: <link to the approved plan> — frozen at: <HEAD short hash>

## Binding rules
<the source plan's Decisions verbatim; [USER] rows first>

## Build sequence
<numbered units in dependency order; per unit: files, gate command, expected result>

## Units
<per unit: complete final code; modifications carry replaces: `path:lines`; the constraints binding this unit restated inline>

## Known-red ledger
<table: Expected failure | Trigger | Sanctioned fix — e.g. generate-check red → run the generator; anything not listed here escalates>

## Forbidden fixes
<never weaken a test, gate, or validation to green; never touch a file outside the planned-files table; never substitute a mechanism>

## Escalation contract
<enumerated blocked-triggers; each is a report line with the observed state, never a halt — a plan claim not matching the tree, a red not in the ledger, a fix requiring an out-of-inventory file, a constraint that cannot hold>

## Post-run
<instruction to the delegating session: run a cheap contract sweep of the runner's diff against Binding rules before commit sign-off>

## Runner verification
<the source plan's checklist verbatim>
