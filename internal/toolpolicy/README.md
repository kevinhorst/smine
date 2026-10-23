# toolpolicy

The tool-policy definition both hook gates read: the `toolpolicy` section of `~/.claude/hooks/config.json`, decoded by `internal/hookconfig` and rendered by the install from the `gate.*` settings (catalog defaults plus the record's overrides, [docs/settings.html](../../docs/settings.html) § Permission gate). `Policy` is an alias of `hookconfig.Toolpolicy`; the class constants alias hookconfig's.

## Who reads it

- `cmd/toolpolicy` — every verdict: denied and ungated tools, the read fence, the tool class switch, read-only listings, protected and extra paths.
- `cmd/readgate` — the tool class only: it decides Bash, Read and Write class calls and lets every other tool pass.

## How it works

**Load**
- `hookconfig.Load` decodes the whole file with unknown keys refused, so a misspelled key fails every hook at load.
- It refuses an empty `read_only_commands`, an empty `ungated_tools`, a missing `bash`, `read` or `write` class, a tool listed in two classes, an empty listing and a head listed both bare and with subcommands.

**Tool classes**
- `tool_classes` holds four lists: `bash`, `read`, `write`, `rule_bearing`.
- `ClassOf` checks them in that order; an entry ending in `*` matches by prefix (`mcp__*`); a tool in no list is unknown.
- `ungated_tools` is checked before the class switch, so a tool listed in both (`Agent`) is allowed by the toolpolicy gate and ungated by the read gate.

**Read-only listings**
- Each `read_only_commands` entry is a program or a program plus subcommand words; `ReadOnlyListing` returns a head's `Listing` — `IsBare` admits every subcommand, else only `Subcommands`.
- The flag analysis over a listing stays in `cmd/toolpolicy/readonly.go`, since it needs the gate's git option parsing.

## Files

- `policy.go` — the `Policy` alias and the class constants; the model, its load checks, `ClassOf`, `Listing` and `ExpandHome` live in `internal/hookconfig/sections.go`
