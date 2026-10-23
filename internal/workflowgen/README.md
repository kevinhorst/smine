# workflowgen

Regenerates the generated blocks of a skill's stage workflow files (`<skill>/workflows/*.js`) and enforces the workflow-layer shape rules. Driven by `rules workflow-gen` (`make workflow-gen`, and `make workflow-gen-check` in the audit).

## How it works

**Generated blocks**
- The Workflow runtime has no imports and no filesystem access, so anything single-sourced — the schemas, the shared common prelude, the subagent model vocabulary, the skill's prompt payloads — is injected between markers:

```
// GEN:schemas <name>[,<name>...]   ... // GEN:end
// GEN:common                       ... // GEN:end
// GEN:models                       ... // GEN:end
// GEN:payload <entry-id>           ... // GEN:end
```

- `GEN:schemas` emits one JS const per named schema from `<skill>/schemas/<name>.json`, as indented JSON. The const is `<NAME>_SCHEMA`, except `finding` → `FINDING_PROPS`, `verdict` → `VERDICT_PROPS`, `severity-table` → `SEVERITY_TABLE`, `severity-fixtures` → `SEVERITY_EXAMPLES`.
- Shared property sets: the string tokens `"$FINDING_PROPS"` and `"$VERDICT_PROPS"` inside a schema stand for the content of `finding.json` and `verdict.json` beside it. The generator and the handoff schema compiler both resolve this list.
- `severity-fixtures` emits only the example rows of `<skill>/severity-fixtures.json`, each in the key order effect, exposure, severity, discriminator.
- `GEN:common` is `workflows/common/common.src.js`, empty when the skill has none.
- `GEN:models` is the models block the caller passes.
- `GEN:payload` emits `<skill>/payloads/<entry-id>.md` as the JS string const `PAYLOAD_<ENTRY_ID>` (dashes to underscores, upper case): the whole file, its trailing newline trimmed — a `Return:` line included; the script fills the payload's placeholders. A missing payload file fails the run.

**Run and check**
- A run regenerates every stage file of the dir in place; under check it only compares. Both return the findings: stale files and shape violations. A workflows dir without a stage file is an error.

**Shape rules** on every stage file
- At most 300 handwritten lines (lines outside the generated blocks, marker lines excluded).
- No schema literal (`*_SCHEMA`, `*_PROPS`, `*_TABLE`, `*_EXAMPLES` assigned an object or array) outside a generated block; the check keys on the const suffix alone.
- `make workflow-gen` and `make workflow-gen-check` cover only `skills/*/*/workflows` (`WORKFLOW_DIRS`); a skill at another depth is neither generated nor checked.
- A file that calls `agent(` carries the models block and names each model (`assertModel(` or `DEFAULT_SUBAGENT_MODEL`).
- No conduct or limit language (token conduct, "as few calls", file-size limits, ToolSearch bans).
- No copy-back language ("EXACTLY as given", "byte-for-byte", "output you return is that file").
