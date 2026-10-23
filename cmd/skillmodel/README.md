# skillmodel

Works the typed skill model (`<leaf>.cue`, named after the skill dir) and the semantic model derived from it. Called by `make skill-render`, `make skills-check`, `make skill-export`, `make skill-import`, `make skill-bundle` and the `model-*` targets, always through `go run ./cmd/skillmodel`, and by hand for the solver commands.

## Usage

```
usage: skillmodel <atoms|bundle|check|diagram|export|import|model|query|render|renumber|smt|solve> [flags] <skill dir>…
```

Per command:

```
skillmodel atoms [--json] <skill dir>
skillmodel bundle <dir>
skillmodel check <skill dir>
skillmodel diagram [--json] [--phases] <skill dir>
skillmodel export <skill dir>
skillmodel import <skill dir | vocabulary.json>…
skillmodel model <skill dir>
skillmodel query --expr '<expr>' <skill dir>
skillmodel render [-check] <skill dir | skills root>…
skillmodel renumber <skill dir>…
skillmodel smt|solve (--consistency | --query <q.json> | --reach <id> [--bound n]) <skill dir>
```

- `atoms` — the atom census: one line per atom the skill declares or reads, sorted by id, `<id>  <kind>[ shared]  source=<source|->  preds=<ENTRY-ID(role),…|section:<title>|->  queries=<name,…|->`; `--json` prints `[{id, shared, kind, readers: [{entry?, query?, role, section}], source?}]`. Exit 0.
- `bundle` — writes the standalone skill bundle into an empty dir; exits 1 when a written file carries a smine reference.
- `check` — runs the check battery: clause validation, then on the strict tier the z3 checks (consistency, guards, halt, preservation, reach, completion). One line per finding plus coverage; only an error finding fails it.
- `diagram` — the whole-skill projection as Mermaid; `--json` prints the stable projection document instead, `--phases` renders the execution chain only, without context sections.
- `export` — the loaded model plus its merged atom view (the shared vocabulary overlaid with the skill-local atoms) under `allAtoms`, as JSON for jq; a format-3 entry carries its `fields` record.
- `import` — converts a markdown-era skill dir (SKILL.md, plus `atoms.json` and `changelog.json` when present) into `<leaf>.cue` plus payload files, or a `vocabulary.json` into its sibling `vocabulary.cue`. The shared vocabulary is attached so Check sees atom collisions. Never deletes a source file.
- `model` — dumps the derived semantic model as JSON for inspection.
- `query --expr` — asks whether the model's invariants imply the expression: unsat means it holds and the core names the proving invariants; sat comes with a counterexample scenario.
- `render` — regenerates skill dirs from their model (Load, then Save rewrites `<leaf>.cue` and the payload files canonically) and stores the deployed body's token count on the newest changelog row.
- `renumber` — rewrites a `<leaf>.cue` whose changelog versions are not strictly descending: bottom-up, a row not above the one below it is lifted by its own bump kind; the frontmatter version follows the top row. The one repair for the descending check; it never goes through Load.
- `smt` — emits the SMT-LIB2 document to stdout; `solve` pipes it through z3 and reports the verdict with the unsat core as entry ids and a witness scenario on sat.

## How it works

**Loading**
- Every command loads the skill through `skillgraph.Load`: `<leaf>.cue` (its frontmatter `name` must equal the dir), its payload files and the shared `vocabulary.cue`.
- The semantic model is derived mechanically from the skill's pred/post clauses; nothing is committed or drafted. It carries the solver vocabulary and the analysis tier (strict when `<leaf>.cue` sets `tier: "strict"`).

**render**
- Args: an arg holding `<leaf>.cue` (its own dir name) is a skill dir; an arg holding only SKILL.md is unmodeled; any other arg is a skills root expanded through `skillgraph.Discover`, which fails on an unmodeled dir below it and on a dir still holding `skill.cue`, naming the `git mv` that renames it.
- `-check` writes nothing and prints one drift line to stderr per non-canonical, missing or orphaned file, invalid model, leftover generated file, leftover markdown-era source, leftover `skill.cue`, or unmodeled dir arg.
- Per skills root it also runs the whole-root passes: pipeline edge agreement, Skill grants, the read-or-delete atom check (`UnreadAtoms`), cross-skill link tokens (`UnresolvedCrossRefs`), the smoke requirement and the smineless bundle scan (`skills.CheckBundle`). A skill that does not load is reported by its own dir pass and blocks them.
- The token count is the deployed SKILL.md body without its frontmatter, counted through the keychain-backed counter over the config server's token cache, so a render never re-bills a body the server already counted. A count that cannot run leaves the row unchanged with one warning; the render still succeeds.

**smt and solve**
- `--consistency` emits the whole-model consistency check; `--query <q.json>` emits one committed query file; `--reach <entry-id>` emits the causal reachability check for one transition.
- A query file carrying a `reach` field runs the reachability check instead of a static one; the file's own bound overrides `--bound`.
- A zero bound means the complete bound. Below it a warning is printed, and DEAD is claimed only when the bound reaches the complete bound.
- `solve`: an unknown verdict is an error; a query's `expect` field turns a mismatch into a nonzero exit.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | clean |
| 1 | findings: drift under `render -check`, an error finding of `check`, an `expect` mismatch |
| 2 | usage or tool error, including an unmodeled dir below a root arg |

## Files

- `bundle.go` — the bundle command
- `main.go` — dispatch and the check, diagram, model, query, smt and solve commands
- `render.go` — the render, export and import commands and the drift check
