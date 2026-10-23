# acdsl

Runs the agentic context DSL: one rule source (`acdsl/*.acdsl` plus `//acdsl:` lines), two projections — a gate that executes each rule's verifier, and the governing rules delivered to the agent at read time (`project -context`, the acdslhook hook's data source) or written as a comment block above a file's content (`project -file`, for consumers outside Claude). Called by `make acdsl` and `make audit`, by the acdslhook Read hook, and by the pre-commit guard.

## Usage

```
usage: acdsl <check|guard|project|fixtures> [flags]
usage: acdsl dist -target <name> -dest <dir> [-root <dir>] [-task]
```

- `check [-json] [-lifetime doctrine|task|all] [-rule <id>] [-report <file>] [-registry <file>] [-root <dir>]` — resolve every rule's anchor and run its registered verifier; one line per violation citing the rule id, or JSON records with `-json`. `-rule` gates one rule of any lifetime (the generation dry-run); `-report` also writes the grouped violations report to a file; `-registry` overlays a registry on the root's `acdsl/registry.json`, entries winning by name.
- `guard [-root <dir>]` — the fast pre-commit slice of `check`: only the staged-projection leak scan.
- `project (-file <path> | -strip | -plan <path> | -context <path>) [-root <dir>]` — exactly one mode:
  - `-file` syncs one file's on-disk projection block in place;
  - `-strip` removes every block before committing;
  - `-plan` prints the rules that would govern a path that need not exist yet (plan-time resolution);
  - `-context` prints the delivered rules of an existing governed file as plain lines, without touching it; empty when nothing delivered governs it.
- `fixtures [-lifetime doctrine|task|all] [-registry <file>] [-root <dir>]` — prove each rule's verifier against its committed pass/fail examples under `acdsl/testdata/<rule-id>/`.
- `verdicts [-path <file>] [-since <duration>]` — per-rule table of runs, red runs, violations and last red run from the verdict log.
- `dist -target <name> -dest <dir> [-root <dir>] [-task]` — distribute a target repo's gate slice: the reach-covered rules, their registry subset and the prebuilt runner and verifier binaries; `-task` also ships task-lifetime rules that reach the target.

## How it works

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | clean |
| 1 | violations (authoring, staged projection, policy or verifier findings; fixture failures) |
| 2 | tool error or bad usage |

**check**
- Refuses staged projection blocks: the working tree carries them, the index never may. Committed blocks are refused the same way.
- Enforces the `acdsl/policy.json` self-management mode: strict and gated repos may not change rules or verifiers off the base branch.
- Verdicts are cached per checkout under `bin/acdsl-cache/`: a rule replays its stored pass or diagnostics while the verifier binary, the rule, its params and its inputs are unchanged. The registry entry declares the inputs — `cache: files` (the matched files, `needs`, param-named files and the `reads` directory prefixes), `cache: tree` (every file of the universe, the default) or `cache: none`. Verifier errors are never cached; fixtures run uncached.
- `-lifetime` defaults to `doctrine`: the audit gates doctrine on every run. A task's contract (task-lifetime entries in a `*.acdsl` file anywhere in the repo) is legitimately red mid-implementation and is gated explicitly with `-lifetime task`.

**guard**
- Syntax-aware: a file quoting another syntax's projection marker in prose is a reference, not a leak.

**project**
- The block is the governing rules as a comment directly above the content.
- `-context` is the read-time delivery channel for every governed file; it is the acdslhook hook's data source. `-file` writes the same rules as an on-disk block for consumers that read bytes, never for Claude sessions; `make audit`, the routine runner and the pre-commit guard strip or refuse those blocks.

**Verdict log**
- Every `check` run appends the gated rule set and its diagnostics to the verdict sink (`$ACDSL_VERDICTS_PATH`, default `~/.claude/acdsl/verdicts.jsonl`), with repo root, branch and session id.
- Best-effort by contract: the exit code never depends on logging. `ACDSL_VERDICTS_ENABLED=0` turns it off.

## Verifiers

Every verifier under `cmd/acdsl/verifiers` is a registered binary with the same contract unless its section says otherwise:

- args: `<files-list path> [key=value ...]` — the files list holds the anchored files, whitespace-separated
- output: one violation per stdout line as `file:line: message`
- exit: 0 pass, 1 violations, 2 tool error

A tool error is never a silent pass: a missing binary, config or vocabulary exits 2.

### bash32compat

- Fails when a shell script uses a bash-4+ construct. Hooks and routines run under macOS system bash 3.2 (launchd), where these fail at parse or run time.
- Pattern scan, deliberately tight to stay free of false positives.
- Usage: `bash32compat: usage: <files-list>`. No params.

### commentcap

- Fails when a comment block in a listed Go file runs longer than `max` lines.
- Counts each comment of a group; bare `//` lines and directives (`//go:`, `//nolint`, `//export `, `//line `) do not count; an ACDSL projection block counts zero.
- Params: `max=<n>` (default 1, a positive integer).
- Usage: `commentcap: usage: <files-list> [max=<n>]`.

### errorfprefix

- Fails when a `fmt.Errorf` or `errors.New` message literal lacks the `Component.Method:` prefix (RULE-GOLANG-ERR-001): the segment before the first `": "` must be one identifier or a dotted identifier, so `"failed to read config: %w"` and unprefixed messages are violations.
- Non-literal first arguments are skipped. Dot and blank imports are out of scope.
- Usage: `errorfprefix: usage: <files-list>`. No params.

### evalgenbounds

- Fails when a verifier `main.go` leaves the `acdsl/evalgen.json` bounds: line count over `maxLines`, or an import outside the policy — Go stdlib (first path segment has no dot), the module's `internal/...` packages, or a module already in `go.mod`'s require set (single-line and block form, read line-wise).
- Applies to handwritten and generated verifiers alike: generation may never add a dependency (Band D stays a user call), and the gate enforces it, not the prompt.
- Missing config is a tool error.
- Usage: `evalgenbounds: usage: <files-list>`. No params.

### exhaustivewrap

- Fails when a switch over a typed-constant enum misses members (ACTION-IMPL-INTEG-005), by running the `exhaustive` analyzer (a `go.mod` tool dependency).
- Analyzers load packages, not loose files, so the wrapper runs over the unique package dirs of the listed files.
- Contract-shaped output lines mean violations (exit 1) whatever the analyzer's own exit code; a failure without contract lines is a tool error (exit 2). A distributed target without the tool dependency gets an actionable violation, not a tool error.
- Output shape: `file:line:col: message`. Usage: `exhaustivewrap: usage: <files-list>`. No params.

### filebaseline

- Fails when a listed file is absent from the committed baseline list, freezing the anchored file set so no new file of that class can be added.
- Params: `baseline=<path>` (required; one path per line, `#` comments).
- Usage: `filebaseline: usage: <files-list> baseline=<path>`.

### forbidaddrlit

- Fails when a file takes the address of a composite literal (`&T{...}`), the pre-Go-1.26 pointer-initialization idiom that `new(expr)` replaces.
- `&v` of a variable is out of scope.
- Usage: `forbidaddrlit: usage: <files-list>`. No params.

### forbidcall

- Fails when a file calls `<pkg>.<func>`, resolving import aliases per file. Dot and blank imports cannot be matched syntactically and are out of scope.
- Params: `pkg=<import path>`, `func=<name>`.
- Usage: `forbidcall: usage: <files-list> pkg=<path> func=<name>`.

### forbidphrase

- Fails when a file matches the forbidden `pattern`, or when `require` is set and no listed file contains it.
- Params: `pattern=<RE2>` (required), `require=<RE2>` (optional).
- Usage: `forbidphrase: usage: <files-list> pattern=<RE2> [require=<RE2>]`.

### funcsig

- Fails when a function declaration violates RULE-GOLANG-FUNC-001: a `context.Context` parameter is first and named `ctx`, an error result is last, at most 3 results.
- Scope: function declarations only; function literals and interface method sets are out of scope. Unnamed context parameters are not flagged for naming.
- Usage: `funcsig: usage: <files-list>`. No params.

### gofmtwrap

- Fails when an anchored file is not gofmt-formatted. Wraps `gofmt -l`, which exits 0 even when it finds unformatted files, and maps each listed filename to one `file:line:` violation with exit 1.
- Replaces the raw `gofmt -l cmd internal` audit line.
- Usage: `gofmtwrap: usage: <files-list>`. No params.

### golangciwrap

- Fails when golangci-lint reports findings on anchored Go files. Dep-gated (Band D, accepted): golangci-lint is a brew binary; a missing binary exits 2.
- golangci-lint resolves packages itself, so the run covers the packages of anchored Go files changed against the merge-base with `main` (the same base the config's `new-from-merge-base` filters on) and the output is filtered to the anchored files; no changed Go file means no lint run, no resolvable base means `./...`; a per-file invocation breaks its package loader.
- Fixture mode: when every anchored file lives under `acdsl/testdata/`, the repo config — whose differential gate would suppress committed examples — is replaced by a config-free run of one pinned linter (`mnd`) over the fixture dirs, proving the wrapper's plumbing.
- Cache and start-up lock are isolated per checkout. The cache keys on package content and fixture packages are byte-identical in every worktree, so a shared cache serves results whose relative paths point into other checkouts; the lock lives in `os.TempDir()` (per user on macOS), so a shared one fails concurrent runs of other checkouts with "parallel golangci-lint is running". Both modes wait for the checkout's own lock.
- Output shape: `file:line:col: message`. Usage: `golangciwrap: usage: <files-list>`. No params.

### goldendiff

- Runs an observation anchor: GET the recipe URL, apply the normalization spec, write the observation to `observed=`, and diff it line-wise against the golden file the rule anchors. The report shows observed against expected, not just FAIL; up to a fixed number of divergences are reported, anchored to golden line numbers.
- Normalization spec: one line per substitution, `s<delim>re2<delim>repl<delim>`, applied to the whole body — timestamps, generated ids and machine paths become stable tokens.
- The files list must hold exactly the golden file.
- An unreachable target is a red contract (exit 1), not a tool error: exit 2 would abort doctrine gating in the same `-lifetime all` run.
- Params: `url=<url>`, `observed=<path>`, optional `normalize=<spec>`.
- Usage: `goldendiff: usage: <files-list> url=<url> observed=<path> [normalize=<spec>]`.

### idgrammar

- Fails when a doctrine declaration's id leaves the shared identity grammar `KIND-SCOPE[-TOPIC]-NNN`, with SCOPE and TOPIC registered in the taxonomy (the generated context file's aspects of class `scope` and `topic`).
- Caps a rule's `why=` text at a fixed rune count, the same terseness as skill-entry statements; overflow is compressed or the rule split.
- Task-lifetime entries are exempt: contracts are ephemeral and free-form.
- A missing taxonomy file or an empty vocabulary is a tool error.
- Params: `taxonomy=<generated context file path>`.
- Usage: `idgrammar: usage: <files-list> taxonomy=<path>`.

### jsonschemacheck

- Fails when an anchored JSON file does not conform to the schema named by `schema=` (repo-relative path, draft-07). One generic entry serves every schema-backed JSON rule.
- Usage: `jsonschemacheck: usage: <files-list> schema=<path>`.

### linttags

- Fails when a style guide's linter-coverage claims disagree with the repo's golangci config. A `* Covered by \`<linter>\`` bullet claims CI proves that slice and a `[lint]` tag tells reviewers to skip the rule, so a claim naming a linter that is not enabled leaves the rule checked by nobody.
- A coverage bullet is cut to its claim sentence (up to the first `". "` or `" — "`), so trailing prose may name other backticked words.
- The config (default `.golangci.yml`) is found per guide by walking up from the guide's directory; a guide with no config above it passes — the claims are only checkable where the named tooling runs. The config is read line-based, mirroring the retired `check-tags.sh`.
- Params: `config=<filename>` (optional).
- Usage: `linttags: usage: <files-list> [config=<filename>]`.

### literalowner

- Fails when a file outside the allowed path pattern contains the literal.
- A top-of-file projection block is removed in memory before scanning, and line numbers still refer to the file on disk.
- Params: `literal=<string>`, `allowed=<RE2 over repo-relative paths>`.
- Usage: `literalowner: usage: <files-list> literal=<s> allowed=<re>`.

### modeltoken

- Fails when a model token a listed file selects does not resolve against the repo templates' `availableModels`; with `role=subagent` the token must also be able to run a subagent (`internal/models`).
- Line mode: tokens come from `<key>: <value>` pairs (`keys=a,b`) or bare `claude-<family>-<n>` ids (`ids=true`), on lines matching `line=<RE2>` (default every line).
- JSON mode: a `.json` file (without `ids=true`) is decoded; every object whose `name` is one of the keys is checked by its `value`, plus string members under those keys. The violation line is where the value first occurs.
- A value is its first field plus comma-continued fields, ending at the next `key:` field. Each item is cut at its `:effort` and `+no-<mcp>` suffix and reduced to its literal model token; non-literal items (identifiers, placeholders, expressions) are skipped.
- Params: `role=session|subagent`, `keys=a,b` or `ids=true` (one required), `line=<RE2>`.
- Usage: `modeltoken: usage: <files-list> [role=session|subagent] [keys=a,b] [ids=true] [line=<RE2>]`.

### planformat

- The lintable subset of `rules/plan.md` over persisted plans under a plan dir's `design/`:
  - the plan dir sits in a feature folder, never directly under `plans/` (RULE-PLAN-071);
  - the first `## ` section is TLDR;
  - a `## Changelog` section exists;
  - a `## ` heading carries `[AGENT-ONLY]` exactly when it names an agent-tier catalog section; an override section declares its own tier (RULE-PLAN-090);
  - catalog sections (`internal/plans.Sections`, RULE-PLAN-002) appear in catalog order and Changes entries in attention order (RULE-PLAN-098), both only in plans created on or after `plans.OrderCutoff`;
  - a plan carries no Permissions heading at any level (RULE-PLAN-082);
  - a Changes section that declares phases keeps one phase's entries on disjoint files (RULE-PLAN-088).
- Unknown headings are ignored; no prose heuristics.
- Usage: `planformat: usage: <files-list>`. No params.

### rulescheck

- Wraps `cmd/rules`: `mode=validate` runs `go run ./cmd/rules validate`, which prints one violation per line and exits 0/1; the wrapper normalizes tool failures.
- The anchored files only decide when the rule fires; the wrapped command always checks the whole pack.
- Replaces the raw Makefile validate line. `context/context.json` has no currency gate: it is untracked and `make audit` regenerates it before the gates run.
- Usage: `rulescheck: usage: <files-list> mode=<mode>`.

### runbookformat

- The lintable subset of `rules/runbooks.md` over Bruno runbook collections, per `.bru` file: request files need a non-empty `docs` block and an `assert` block; `collection.bru` needs a non-empty `docs` block; `folder.bru` and environment files are skipped.
- Usage: `runbookformat: usage: <files-list>`. No params.

### severitytable

- Checks a skill's `severity-fixtures.json` against its sibling `schemas/severity-table.json`:
  - axes — every axis has at least two values; every row value is an axis member;
  - complete — every effect × exposure combination matches exactly one row (an absent row axis matches every value);
  - fixtures — every fixture's vector resolves to its expected severity; ids are unique.
- Output shape: `file: message`. Usage: `severitytable: usage: <files-list>`.

### shellcheckwrap

- Fails when shellcheck reports findings on an anchored script. Dep-gated (Band D, accepted): a missing binary exits 2.
- Runs `-f gcc` for the contract line shape; contract-shaped lines mean violations whatever shellcheck's own exit code.
- Output shape: `file:line:col: message`. Usage: `shellcheckwrap: usage: <files-list>`. No params.

### skillcontext

- Validates a SKILL.md's optional `acdsl-context:` frontmatter declaration, the context entries a skill wants injected at invocation:
  - token — an entry id (`KIND-SCOPE[-TOPIC]-NNN`) or a trailing-`*` glob whose prefix ends in `-` (e.g. `ACTION-CONCEPT-*`);
  - exact id — must exist in the context index named by `index=`;
  - glob — must match at least one index entry.
- Skills without the line pass; the declaration is opt-in.
- A legacy `context:` key whose value looks like a declaration is a violation: that key is Claude Code's native frontmatter field (values like `fork`), and a declaration under it silently injects nothing.
- Only same-line frontmatter values are read; a block scalar reads as empty.
- Usage: `skillcontext: usage: <files-list> index=<index path>`.

### skillentries

- Per listed model file (`<leaf>.cue`, named after its dir), loads the model through `skillgraph.Load` — the CUE schema and `skillgraph.Check` own the entry grammar, unique ids, role order, route anchors, payload files and version agreement — and reports every Load violation, one per line.
- A loading model gets the checks Load cannot make:
  - NAME — every entry id's NAME segment is the leaf directory name, upper-cased without hyphens;
  - claims — a `[gate]`/`[hook]`/`[schema]` channel's `Gate:`/`Hook:`/`Schema:` bullet resolves against `acdsl/*.acdsl`, an existing path under `cmd/` (the hook's source — `cmd/<hook>/`, heartbeat's `cmd/heartbeat/src/enforce.rs`) or the repo tree; a missing `acdsl/` tree makes every Gate claim fail;
  - spans — no inline code span of the rendered SKILL.md enumerates more than 6 comma-separated items.
- Findings point at `<leaf>.cue:1` for model findings and `SKILL.md:<line>` for spans (the rendered file equals the committed one under the freshness gate).
- Run from the repo root: claim pointers resolve relative to the working directory.
- Usage: `skillentries: usage: <files-list>`.

### skillmap

- Validates `context-map.json` files, the per-skill lane maps the skill-context hook slices injections by:
  - schema — valid JSON, `schemaVersion` present, `skill` equals the map's parent directory name;
  - tokens — every lane token is an entry id or trailing-`*` glob and resolves in the context index;
  - subset — the entry set each lane selects is a subset of what the sibling skill's `acdsl-context:` declaration selects (the render of its `<leaf>.cue`, or a fixture's SKILL.md).
- Output shape: `file: message`. Usage: `skillmap: usage: <files-list> index=<index path>`.

### skillmeta

- Checks skill models and SKILL.md fixtures by `mode=`:
  - `version-sync` — over `<leaf>.cue`: the model loads; `skillgraph.Check` owns frontmatter `version` == `changelog[0].version`. A failed load is one pointer line; ACDSL-SKILL-005 lists its violations.
  - `frontmatter` — name, description, author and version present, single-line, non-empty; description carries `Trigger on`; a `## Args` section requires `Args —` in the description.
  - `allowed-tools` — comma-separated permission rules in the settings allow-list dialect; Bash entries never use colon wildcards; with `sort=required` the line is normalization-checked. Hyphens are allowed in MCP tool names whose server name carries one.
  - `argument-hint` — arg-bearing skills carry an args-only example invocation naming every `## Args` arg, never leading with `/<leaf>`.
  - `args-table` — `## Args` is the 4-column syntax-index table (bullet fallback for small selector sets).
  - `routing` — over `<leaf>.cue`: the model carries the typed routing block.
  - `context-links` — no bare context-entry id in body prose.
- Every mode except the model modes reads the SKILL.md text behind a listed file: the in-memory render of a `<leaf>.cue`, or a SKILL.md fixture as it is.
- Usage: `skillmeta: usage: <files-list> mode=<mode>`.

### skillmodelcheck

- Per skill, loads the model (`skillgraph.Load` merges `skills/vocabulary.cue`), derives the semantic model and validates every formal clause: pred expressions parse, every atom resolves, section references hold — strict when the derived model is, i.e. the skill declares atoms of its own.
- Solver-free: the z3 battery runs in `skillmodel check`.
- A listed `skills/vocabulary.cue` re-checks every skill under `skills/`.
- Findings point at the skill's `<leaf>.cue`. Run from the repo root.
- Usage: `skillmodelcheck: usage: <files-list>`.

### skillplantemplate

- The plan-interface gate of `rules/plan.md` over `<leaf>.cue`. A skill whose frontmatter carries `plan-template: <entry id>` names the `[payload]` entry holding its plan template; the template's `## ` headings must open with TLDR, keep catalog sections (`internal/plans.Sections`) in catalog order, carry `[AGENT-ONLY]` exactly on agent-tier catalog sections (RULE-PLAN-090), and mark every non-catalog section as an override — its first non-blank line starts with `<override:`. An override heading declares its own tier.
- Skills without the key pass.
- Findings point at `<leaf>.cue:1` for declaration findings and `payloads/<entry-id>.md:<line>` for heading findings.
- Usage: `skillplantemplate: usage: <files-list>`. No params.

### symbolcoverage

- A task-contract primitive: the named function's test coverage in the anchored package is at least `min` percent.
- Runs `go test -coverprofile` on the one package the anchored files live in and parses `go tool cover -func` output (`path:line: name pct%`), comparing the worst match against `min`. A multi-package anchor is an authoring error.
- Params: `symbol=<func>`, `min=<percent>`.
- Usage: `symbolcoverage: usage: <files-list> symbol=<func> min=<percent>`.

### symbolexists

- A task-contract primitive: the named func, method, type, var or const exists in the anchored files, optionally with the planned signature.
- The signature compare is whitespace-normalized text (plan-level precision, no type resolution); go/printer artifacts of a multi-line declaration are normalized away, so the on-disk layout never fails a matching planned signature.
- Params: `symbol=<name|Recv.Method>`, optional `signature=<text>`.
- Usage: `symbolexists: usage: <files-list> symbol=<name> [signature=<text>]`.

### testschema

- Fails when a test file executes table cases with anything other than the house `t.Run(test._id, ...)` — the sharpest deterministic discriminator between the house table-test schema (RULE-GOLANG-TEST-001..005) and the common `tt.name`/`tc.name` style.
- Files without `t.Run` calls pass.
- Usage: `testschema: usage: <files-list>`. No params.
