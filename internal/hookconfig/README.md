# hookconfig

The one config file every Go hook but skillenv reads: `<claude home>/hooks/config.json`. The install renders it from the `hooksConfig` settings rows of the `internal/components` catalog plus the record's overrides ([docs/settings.html](../../docs/settings.html)); nothing else writes it, and a hand edit is replaced by the next apply.

## The file

One JSON object, one section per consumer:

| Section | Read by | Holds |
|---|---|---|
| `context` | globalcontext, readgate, skillcontext, planmodecontext, skillinvokegate | `context_dir`, the repo-relative context dir; empty means no repo context is injected (the Claude home's global context still is) |
| `globalcontext`, `skillcontext`, `planmodecontext`, `acdslhook` | the context hooks | caps, budgets, banners, pointer and entry formats, patterns, the acdsl timeout and binaries |
| `readgate` | readgate | deny texts, the ledger retention, the default-slice suffix |
| `toolpolicy` | toolpolicy, readgate (tool classes only) | the former `tool-policy.json` lists plus the gate's own literals; `internal/toolpolicy.Policy` aliases it |
| `skillinvokegate` | skillinvokegate | entry chapters, shell meta characters, deny texts |
| `worktreecreate`, `worktreewriteguard` | the worktree hooks | branch prefix, worktrees dir, reset retries, the pool segment, deny texts |
| `sessiontitle`, `skillhelp` | sessiontitle, skillhelp | title limits, trigger patterns, instruction texts, the help pattern |
| `sounds` | no binary reads it — the same values fill the `{{NOTIFICATION_SOUND}}`/`{{STOP_SOUND}}` markers, the argument of the `playsound` group commands | notification and stop sound paths |

Durations are `DurationMs` fields, integer milliseconds under a `_ms` key. Process-env facts stay outside the file: `SMINE_READ_FENCE` (`Toolpolicy.ReadFence`, never decoded), `SMINE_HEADLESS`, `ROUTINE_PROTECTED_PATHS`, `SKILL_CONTEXT_INDEX`, `SKILL_CONTEXT_MAP`, `CLAUDE_CODE_*`.

## Load

- `Load(claudeHome)` reads `Path(claudeHome)` and decodes with `DisallowUnknownFields`: an unknown key, a wrong type or a missing file is an error naming the path.
- The `toolpolicy` section then runs its listing checks — an empty `read_only_commands` or `ungated_tools`, a missing `bash`/`read`/`write` class, a tool in two classes, a head listed both bare and with subcommands — and builds the read-only lookup `ReadOnlyListing` serves.
- On a load error skillinvokegate answers a deny naming the error; every other binary, the tool-policy gate included, prints the error with the path to stderr and exits 1.

## Tests

- No package writes the file for another: each hook's tests build a `*Config` from `install.RenderDefaults(<component>, components.SinkHooksConfig)` for the components they need, change the fields under test, and write it with their own `writeHookConfig` helper (`json.Marshal` to `hookconfig.Path`).
- `config_test.go` covers the strict decode, the listing checks, the `DurationMs` round trip and `ClassOf`.

## Files

- `config.go` — `Config`, `DurationMs`, `Load`, `Path`
- `sections.go` — one struct per section, the tool classes, `Listing`, `ExpandHome`
