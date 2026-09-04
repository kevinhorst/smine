<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# SKILL FORMAT

**Files:** `skills/**/SKILL.md`

**For reviewers / agents:** the authoring grammar lives in the skillroutine-create skill; this guide holds the cross-cutting shape rules for skill bodies.

## Context references

**RULE-SKILL-001** `[review]` — A skill body never requests or references context-entry ids — the entries a skill relies on are declared in its frontmatter `acdsl-context:` line (injected at invocation by the skill-context hook), and the body refers to them in plain language plus the owning context file path.

* Why: an id in the body is a hidden dependency — it reads as authoritative while the entry may not be in context, and it breaks silently when entries are renumbered; the frontmatter declaration is the single machine-checked injection point.
* Body wording: "the plan-dir resolution rule in context/rules/plan.md", "the gate entries of the implementing chapter (actions/implementing.md)" — never the bare id.
* Sole exception: entry ids appearing **as data** in an output-format example, where the produced artifact itself carries ids (e.g. the DoD report table keyed per entry).
* ACDSL verifier/gate names (registry entries, not context entries) are operational identifiers and may be named where a command or gate is invoked.
