# Auto-Apply Rules (decide mode)

Judged per proposal by the smine-nightly apply stage. A proposal is SAFE only when a Safe rule
below covers it AND no Unsafe rule matches; everything else is UNSAFE. Held proposals keep
status `proposed` and carry the reason in `autoApplyHeld`.

## Safe

- style: edits to existing style/context docs — rule wording, examples, amendments to existing RULE-* ids.
- workflows: edits to existing workflow scripts that add no network access and no process-launching steps.

## Unsafe

- Any new routine, any plist or launchd change.
- Any edit under settings/ or to permission surfaces (allowlists, hooks).
- Any new skill; skill edits beyond prose clarification.
- Any change that deletes or weakens tests, guards, or validation.
- Any target outside this repository.
