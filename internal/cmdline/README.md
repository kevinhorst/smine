# cmdline

One splitter for every command value a skill variable or a review flag carries.

## Grammar

- Spaces separate words; runs of spaces count as one.
- Double quotes group words; `\"` inside a group is a literal quote; `""` is an empty word.
- Refused: `&&`, `||`, `|`, `;`, `$(`, a backtick, a newline and an unbalanced quote — the error names the construct.
- Nothing else is interpreted: no variables, no globs, no single quotes, no escapes outside a group.
- `Split` takes one line; `SplitLines` takes a multi-line command value, skips blank lines, splits each other line with
  `Split` and refuses a value without a command line.

## Consumers

- `skillgraph.CheckValue` — the value check for `command` values (`SplitLines`) and `program` values (`Split`, one word).
- `skills.ValidateLocation` — resolves the first word of each command line on PATH, a relative word with a slash inside
  the repo it serves.
- `reviewscope` — `smine-rules buildtest` and `probe-replay` split `--build`/`--test` with `SplitLines`
  (`commandLines`) and exec each line's words without a shell, in order.
