# components

The installation component catalog: pure data naming every installable component, its hard requirements and platform restrictions. The installer, the apply engine, the skill model and the config server all read it.

## How it works

- The catalog is ordered for display; `Catalog`, `Full` and `inCatalogOrder` keep that order, and ids outside the catalog are dropped.
- A component with no `Platforms` is available everywhere; otherwise only on the listed GOOS values (`Available`).
- `Requires` are hard requirements. `Close` expands ids to everything they need, transitively; `Dependents` is the reverse view among a selection.
- `Fixed` marks the one component that is always selected, `ConfigServer`.
- `Minimal` is the preset of ConfigServer and Skills; `Full` is every component available on a GOOS.
- `Without` lists, one line each, what is off or degraded when the component is absent, and is shown on every surface.

## Providers

This package knows no tool. The provider records of `skills/interfaces.cue` name the component whose install ships each program (`smine-rules` → config-server, `smine-agentwt` → agentwt, `smine-heartbeat` → heartbeat, `smine-routine` → routines, `toolpolicy` → gate; `acdsl` has none); the skills deploy fills a capability from its provider when that component is in the render's set (`internal/skills.Values`). A `Without` line names the skills a missing component keeps from deploying; every other skill drops the steps that call its provider. The Welcome page joins `Without` lines with `; `, so no line carries its own semicolon.
