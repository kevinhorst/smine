# Smoke request — Shout

Add a new Go package `internal/smokeshout` that nothing imports.

- `Shout(text string) string` returns `strings.ToUpper(text) + "!"`; an empty `text` returns `"!"`.
- `TestShout` is table-driven and covers an empty string, a lower-case word and a mixed-case sentence.
- No other file changes; no command, flag, config key or doc is added.
