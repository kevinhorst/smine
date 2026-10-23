# sessions

Selects, checks and marks smine session records — the JSON files under `sessions/<repo>/<YYYY-Www>/<id>.json` the /smine pipeline mines into and reads from. Called through `make sessions` and `make context-record` by the smine-batch and smine-problems skills.

## Usage

```
usage: sessions <check|context-record|mark|pending|unmark|unsynthesized> [flags]
usage: sessions check [--context <transcript.jsonl>] <record.json>...
usage: sessions context-record <transcript.jsonl> [ctx-dir]
usage: sessions mark --synthesized <YYYY-MM-DD> <session-id>...
```

Full synopsis:

```
sessions context-record <transcript.jsonl> [ctx-dir]
sessions pending [--agents claude,codex] [--dev] [--limit n] [--peek-bin path] [--registry repos.json] [--root sessions] [--window-days n]
sessions check [--context <transcript.jsonl>] <record.json>...
sessions mark --synthesized <YYYY-MM-DD> [--root sessions] <session-id>...
sessions unmark [--root sessions]
sessions unsynthesized [--root sessions]
```

- `context-record` — prints the used-context record of one Claude transcript and its subagents as JSON. Tokens stay uncounted here; the config server counts them. The context dir is the second argument, else `AGENT_CONTEXT_DIR_DEFAULT`, else `docs`. An agent without injections carries `items: []`, never `null`.
- `pending` — lists the sessions to mine as JSON, newest first, each with its record path. Candidates come from peek-mcp over stdio, within `--window-days` (default the peek mining window) and a two-minute deadline; `--dev` includes the pipeline's own sessions; `--limit` caps the newest-first list (0 = all); the repo roster comes from the repo registry, the main checkout from the git common dir.
- `check` — normalizes each record, redacts its permission commands, validates it against `skills/smine/smine-batch/reference/session.schema.json` and rewrites it canonically. `--context <transcript.jsonl>` (exactly one record path; before or after it) first extracts that transcript's used-context record as `context-record` does — context dir from `AGENT_CONTEXT_DIR_DEFAULT`, else `docs` — and writes it into the record's `context` key, replacing any value there; smine-batch's miners merge it this way instead of pasting the record.
- `mark` — sets the synthesized date of the named records and re-checks them; an unknown id prints `<id>: no record`.
- `unmark` — clears every record's synthesized date, so the next run's mining stages read them all again.
- `unsynthesized` — lists the records smine-problems has not read yet, oldest first, as JSON (`id`, `lastActive`, `path`, `repo`, `week`); archived records are skipped, a record with violations is skipped with a note to run `check`.

## How it works

**Output**
- `check`, `mark` and `unmark` print `<path>: <violation>` per violation, else `<path>: ok`.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | clean |
| 1 | violations, or an unknown session id for `mark` |
| 2 | usage or tool error |
