# skillgraph

The skill model: loads the model file `<leaf>.cue` (`ModelFile`, `ModelPath`, `IsModelFile`; a dir still holding the retired `skill.cue` is refused) and its payloads, checks the model, renders the canonical CUE and the deployed SKILL.md, `help-test.md`, `title.json`, `enforce.json`, `env.json` and `answer.md`. The authoring contract is `docs/skillcue.html`; this file covers the format-3 fields and links, the interface catalog, the env variables and the literal check.

## Fields and links

- `format: 3` (`entryFormatFields`) is the ratchet every check below gates on, except `checkSources` and the capture count, which run on every skill.
- `fields.go` — `checkLabels` refuses a labelled item; `checkFields` holds the role set and the value shape of each of the nine keys (`fieldRoles`, `channelFieldViolations`, `mintsViolations`); a `flags` placeholder resolves against `fieldVocabulary` (Args rows, env keys, def titles and their backticked identifiers, `reads`/`returns`/minted id names). `checkFieldFiles(dir, skill)` runs in `Load`: `script`/`schema` exist under `workflows/`/`schemas/`, and `reads`/`returns`/`forwards.stage` are top-level properties of the entry's schema. `fieldTexts` is the one field-bullet text source: `renderFields` renders it with derived links, `FieldTexts` gives the diagram the raw texts.
- `refs.go` — `[[SECTION-NNN]]` / `[[SKILL-X-SECTION-NNN]]` tokens (`refTokenPattern`, `tokenId`); `RenderRef` derives `[Title · SECTION-NNN](#anchor)`; `displayText` counts a token as its target's title for the rune caps; `checkRefs` (resolution plus the ref discipline; Args and Routing tokens only resolve), `checkTitles` (case-insensitive uniqueness), `UnresolvedCrossRefs` (the root pass for foreign tokens). `Entry.Refs(skillName)` reads markdown links, tokens and `fields.ref`.
- `sources.go` — `ParseSource` is the one source parser (`arg`, `session`, `environment`, `runtime`); `checkSources` binds `arg` parts to Args rows and enum members and `runtime` parts to the enforce block's anchors and captures by atom kind; `checkVocabularySources` refuses a shared `runtime` source; `RuntimeReaders` feeds the capture count of `checkEnforce`.
- `readers.go` — `AtomReaders` indexes every pred and query read as `Reader{Entry, Query, Role, Section}`; `UnreadAtoms` needs a checkable reader (stop, rule, quality, route section, query) for a format-3 skill's locals and every shared atom; `checkClauseAtoms` refuses an identifier that is no atom or enum member on format 3.
- `queries.go` — `Query` and `LoadQueries`; `Load` sets `Skill.Queries` and `Skill.Dir`.

## Interface catalog

- `skills/interfaces.cue` (`FileCatalog`) holds the `Catalog`: `Interfaces` (`Interface{Id, Version, Kind, Scope, Summary, Example, Invocation, Result, Validate}`, kind `command`, `program`, `path` or `skill`, scope `install` or `repo`) and `Providers` (`Provider{Id, Program, Binary, Component, Platforms, Implements}`). `FindCatalog(dir)` loads it from the skills root beside `vocabulary.cue`; `Load` attaches it as `Skill.Catalog`, and a root without the file is the empty catalog.
- `LoadCatalog` decodes against `#Catalog` and runs `checkCatalog`: `<id>@<version>` unique; command and program interfaces validate with `resolves: true`, path interfaces with `exists: dir|file`, skill interfaces with neither; a program interface's args open with the subcommand word; `result.exit` holds `"0"` and `"other"`; `invocation.timeout` is a Go duration or `none`; provider ids unique, `binary` only with a `program`, every claim names a record, at most one provider per interface version.
- `ProviderFor(ref)` returns the provider claiming a version; `Provider.Executable` is `Binary` when set, else `Program`; `IsProvidedOn` needs a program and a matching platform.
- `CheckValue(iface, value)` holds the value rules: one line, no backtick, no `$SKILL_`, no parenthesis; a command value splits cleanly with `internal/cmdline`, a program value is exactly one word, a path value opens with `~/` or `/`, a skill value is one leaf name (lowercase letters, digits, hyphens).

## Env variables

- `Skill.Env` holds the `env` block: key → `Variable{Interface, Required}`. `VariableName` renders `SKILL_<leaf uppercased, hyphens removed>_<key>`; `VariableNames` maps every rendered name to its declaration.
- A reference is `$SKILL_<SKILL>_<NAME>` (`variableRefPattern`). `texts` is every string the render emits — frontmatter values, preamble, prose, statements, items and fences, payloads, Args cells, routing and model text, enforce patterns, check reasons and recoveries, judgment questions and sends.
- `checkEnv` (part of `Check`, so every `Load` and `Render`) refuses a reference no declaration covers, a declaration no text reads, an optional variable read in fixed text, a binding whose interface is not in the catalog, and a key other than `VariableKey(id)` (the id uppercased, hyphens as underscores).
- Reference shape: a program reference is followed by the interface's args head word (`ArgsHead`), which itself ends in a space, backtick, quote, `(`, `)` or a line end; a bare program reference directly closed by a backtick, quote or `)` passes as the program value. A command reference is followed only by the end, a backtick, a quote, `)`, `>`, `2>` or `|`. Path references are unchecked.
- `VariableReads(skill, name)` is the typed list of what reads a name — `VariableRead{Id, Kind}`, kind entry, param, flags (a format-3 entry's `flags` field), grant, anchor or check — and what an unset name drops; `VariableReaders` its strings: `entry <id>`, `param of <id>`, `field flags of <id>`, `grant <rule>`, `anchor <id>`, `check <id>`. The env widget and the graph's env tiles both read it.
- `UnconfiguredVariables(skill, values)` lists the required names `values` lacks; the deploy skips such a skill.

## Render with values

`RenderOptions.Values` (rendered name → value) and `Deployable` (leaf → deployable) drive `RenderDeployed`; nil for either is the canonical render — nothing dropped, nothing filled. `Deferred` (names filled at invocation) and `DeferredHint` (the abort sentence's tail) drive the placeholders; nil `Deferred` renders none.

- `placeholder.go`: an entry or item reading a deferred name renders as one ```` ```! ```` awk block reading `ENVIRON["<NAME>"]`; a required name unset prints `<NAME>: not set - <summary>, e.g. <example> - <hint>` and exits 2, an optional one exits 0. Bytes outside letters, digits and ` ,.:/_=+@~-` are octal escapes. The deferral records every name a block read.

- `variableDisabled`: an entry whose statement or payload references an unset variable is disabled like a component drop; `itemView` drops an item (text or fence) reading one and keeps the entry.
- `allowedToolsView` drops a rule reading an unset variable and a `Skill(<leaf>)` rule whose leaf is known undeployable; `routingView` drops edges to undeployable skills and routes or edges whose `needs` the set misses.
- `enforceView` drops anchors whose tool, input or exclude pattern reads an unset variable, then every check and judgment naming a dropped anchor or reading an unset variable in its reason, recovery, question or send, and fills the rest with `FillPattern` (values `regexp.QuoteMeta`-quoted). The rendered enforce.json is filled once more with JSON-escaped values for reasons and recovery texts.
- `FillVariables` replaces set references in the rendered SKILL.md; unset ones are left for the caller (the bundle scan flags any survivor).
- `renderDeclarations` writes enforce.json only with gate and heartbeat, title.json only with session-helpers and a set `title-set@1` variable (any under nil values), env.json only with skill-environment and a rendered placeholder — every deferred name a block read, sorted, with its interface id, scope, summary and example (`skillenv.Document`) — and answer.md for `answeredBy`; `help-test.md` renders only with session-helpers.

## Literal check

`checkLiterals(dir, skill)` runs in `Load` only (it reads the skill dir) and refuses `LiteralNeedles` — `smine-rules`, `smine-agentwt`, `smine-heartbeat`, `smine-routine\b`, `toolpolicy` — in every model text and in every file below `agents/` and `workflows/` (walked recursively, `path:line`). Those sibling files are filled at deploy but never trimmed, so they may reference declared required variables only. `LiteralNeedles` is the base of the bundle scan in `internal/skills`.

## Title states

`checkTitle` binds each state to a step or stop whose statement or param runs `$SKILL_<SKILL>_TITLE_SET title set <STATE>` through a variable bound to `title-set@1` (`titleSetRefs`); a skill with states and no such variable is refused.
